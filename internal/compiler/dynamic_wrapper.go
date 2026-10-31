package compiler

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// nilKernelBody is the wrapper body for nil held untyped calling a public Kernel/Object method (`nil.should`): nil has no Go methods to assert.
func (c *Compiler) nilKernelBody(name string) string {
	obj := c.classes["Object"]
	e := obj.lookup(name)
	if e == nil || e.M.Private || e.M.generic() || e.M.Block != nil || name == "initialize" {
		return ""
	}
	return c.dynWrapperBodyOrWarn(obj, e, name)
}

// dynWrapper is one class's wrapper for a name, before emission.
type dynWrapper struct {
	cls     *Class
	e       *entry
	private bool
	body    string
}

// dynArm is a wrapper body shared by the classes in ids: self is recv
// asserted to iface, or recv itself when iface is empty.
type dynArm struct {
	ids     []string
	iface   string
	body    string
	private bool
	members []dynWrapper
}

// seqNames are the whole-array methods a dynamic call answers through the untyped view (decision 236), by the helper
// that answers them: one body for every element type. The block-taking ones (decision 302) were per-shape wrappers,
// each naming its Enumerable chain once per element type (brew: 393 Array shapes).
var seqNames = viewHelpers(map[string][]string{
	"rbViewCall": {"sort", "sort!", "reverse", "reverse!", "compact", "include?", "index", "find_index", "rindex",
		"count", "max", "min", "join", "uniq", "uniq!", "+", "-", "difference", "&", "intersection", "intersect?", "|", "union", "*",
		"delete", "delete_at", "concat", "replace", "fill", "insert", "rotate", "take", "drop", "assoc", "rassoc", "flatten", "flatten!", "transpose"},
	"rbViewBlockCall": {"each", "each_entry", "each_with_index", "each_index", "reverse_each", "each_slice", "each_cons", "cycle",
		"first", "select", "filter", "find_all", "reject", "select!", "filter!", "keep_if", "reject!", "delete_if", "map!", "collect!",
		"find", "detect", "rfind", "inject", "reduce", "each_with_object", "sort_by", "sort_by!", "max_by", "min_by",
		"take_while", "drop_while", "bsearch", "bsearch_index"},
	"rbViewPredicate": {"all?", "any?", "none?", "one?"},
	"rbViewSum":       {"sum"},
	"rbViewCore": {"length", "size", "empty?", "frozen?", "to_a", "entries", "deconstruct", "itself", "dup", "clear", "[]",
		"slice", "at", "fetch", "fetch_values", "dig", "last", "[]=", "<<", "push", "append", "unshift", "prepend", "pop",
		"shift", "shuffle", "shuffle!", "sample"},
	"rbViewGrep": {"grep", "grep_v"},
})

func viewHelpers(by map[string][]string) map[string]string {
	out := map[string]string{}
	for fn, names := range by {
		for _, n := range names {
			out[n] = fn
		}
	}
	return out
}

// hashViewNames are the whole-hash methods a dynamic call answers through the Hash view (decisions 236, 302), by helper.
var hashViewNames = viewHelpers(map[string][]string{
	"rbHashViewCall": {"merge", "merge!", "update", "except", "slice", "to_h", "compact", "compact!", "flatten", "value?",
		"has_value?", "key", "replace"},
	"rbHashViewBlockCall": {"each", "each_pair", "each_entry", "each_key", "each_value", "each_with_index", "each_slice",
		"each_cons", "cycle", "select", "filter", "reject", "select!", "filter!", "keep_if", "reject!", "delete_if", "count",
		"find", "detect", "inject", "reduce", "each_with_object", "transform_values!"},
	"rbHashViewPredicate": {"all?", "any?", "none?", "one?"},
})

// dynPinned names wrappers Go code asserts for by method (rbCmp's DynOp_cmp,
// the dispatchers' fallbacks): each class keeps its own. (rbToJson asserts
// DynToJson only after ToJson(...any), the signature of every prelude to_json.)
var dynPinned = map[string]bool{"<=>": true, "method_missing": true, "respond_to_missing?": true}

// dynWrappers splits name's wrappers into those emitted per class and the
// arms shared by two or more classes. Growing with definitions, not with
// classes × names, is what keeps a program of many classes buildable
// (decision 85).
// includedEntry is name's method from one of cls's included modules, the last include first, with the include's type
// arguments bound, or nil.
func (c *Compiler) includedEntry(cls *Class, name string) *entry {
	for i := len(cls.Includes) - 1; i >= 0; i-- {
		inc := cls.Includes[i]
		if inc.Mod == nil {
			continue
		}
		if e := inc.Mod.lookup(name); e != nil {
			env := map[string]Type{}
			for j, p := range inc.Mod.TypeParams {
				if j < len(inc.Args) {
					env[p] = inc.Args[j]
				}
			}
			return &entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: cls}
		}
	}
	return nil
}

func (c *Compiler) dynWrappers(name string) ([]dynWrapper, []*dynArm) {
	var own []dynWrapper
	arms := map[string]*dynArm{}
	var keys []string
	for _, cls := range c.classList {
		e, private := c.dynEntry(cls, name)
		if e == nil && !cls.IsModule && !cls.universal && cls.lookup(name) != nil {
			// the own method cannot be called dynamically (Array#to_h is generic over its pairs' types): an included
			// module's answers, as MRI's would for the elements it gets (Enumerable#to_h, decision 194)
			if alt := c.includedEntry(cls, name); alt != nil && !alt.M.generic() && (alt.M.Block == nil || c.dynBlockable(name, alt.M)) && !growsTypeParams(cls, alt) {
				e, private = alt, alt.M.Private && !rubyPrivate[name]
			}
		}
		if e == nil {
			continue
		}
		w := dynWrapper{cls: cls, e: e, private: private, body: c.dynWrapperBodyOrWarn(cls, e, name)}
		if w.body == "" {
			continue
		}
		w.body = c.freeCall(cls, e, w.body)
		iface, body, ok := c.dynShareable(w, name)
		if !ok {
			own = append(own, w)
			continue
		}
		key := fmt.Sprintf("%t\x00%s\x00%s", private, iface, body)
		a := arms[key]
		if a == nil {
			a = &dynArm{iface: iface, body: body, private: private}
			arms[key] = a
			keys = append(keys, key)
		}
		a.ids = append(a.ids, strconv.Itoa(c.classID(cls)))
		a.members = append(a.members, w)
	}
	var shared []*dynArm
	for _, k := range keys {
		if a := arms[k]; len(a.members) > 1 {
			shared = append(shared, a)
		} else {
			own = append(own, a.members[0])
		}
	}
	return own, shared
}

// freeCall rewrites a wrapper's forwarder call of a Kernel method (`self.Sleep(`) into the free func over
// `any` (`Kernel_Sleep[any](self, `): one instantiation for every class, and no selector or interface
// literal naming the method, which would keep the forwarder on every reached class (decision 121). Only a
// universal owner: a named constraint's free func (`Module_Name[ModuleI]`) is not the method a metaclass's
// synth `Name()` overrides in Go, and a module constraint (`Comparable_Self[Self]`) needs the concrete class.
func (c *Compiler) freeCall(cls *Class, e *entry, body string) string {
	call := "self." + e.M.GoName + "("
	if !strings.Contains(body, call) || c.isDirectMethod(e.M) {
		return body
	}
	targ := "any"
	if !e.Owner.universal {
		targ = c.ownerIface(cls, e)
		if targ == "" {
			return body
		}
	}
	if c.plainSelf(e.M) {
		targ = ""
	} else {
		targ = "[" + targ + "]"
	}
	body = strings.ReplaceAll(body, call, freeFuncName(e.M)+targ+"(self, ")
	return strings.ReplaceAll(body, "(self, )", "(self)")
}

// ownerIface is the interface a struct class's wrapper instantiates an inherited class body with, as its forwarder
// does (decision 209): the owner's, which every class below it implements, so no forwarder is selected. "" for a
// module (its constraint may name Self), a generic owner, a wrapper on a primitive, or a class object: a metaclass
// answers Module's and Class's methods through Go-level overrides (synth Name, decision 121) that a promoted wrapper
// of its parent's would bypass.
func (c *Compiler) ownerIface(cls *Class, e *entry) string {
	if !cls.isStruct() || len(cls.TypeParams) > 0 || cls.metaOf != nil || e.Owner.IsModule || !e.Owner.isStruct() || len(e.Owner.TypeParams) > 0 || e.Owner.isSubclassOf(c.classes["Module"]) {
		return ""
	}
	return e.Owner.objType()
}

// dynGenerated: e comes from the prelude, or is a metaclass's generated
// new/name/to_s/inspect, not a method a user file defines.
func (c *Compiler) dynGenerated(e *entry) bool {
	return e.M.Kind == kindSynth || e.M.File != nil && e.M.File.isPrelude()
}

var selfIdent = regexp.MustCompile(`\bself\b`)

// re compiles pattern once per compile: dynShareable's per-owner and per-class patterns repeat for every wrapper.
func (c *Compiler) re(pattern string) *regexp.Regexp {
	if r, ok := c.reCache[pattern]; ok {
		return r
	}
	if c.reCache == nil {
		c.reCache = map[string]*regexp.Regexp{}
	}
	r := regexp.MustCompile(pattern)
	c.reCache[pattern] = r
	return r
}

// dynShareable is w's body with nothing particular to its class, and the
// interface its self must satisfy: a struct class (it has a _ClassID) reaching
// a prelude or generated method whose body only calls that method on self, or passes self
// to a universal owner's free func, whose Self is any.
func (c *Compiler) dynShareable(w dynWrapper, name string) (iface, body string, ok bool) {
	cls, e := w.cls, w.e
	if !c.shareableClass(w, name) {
		return "", "", false
	}
	body = w.body
	if e.Owner.universal {
		body = unboxLeafSelf(cls, body)
		for _, t := range []string{c.goType(TClass{C: cls}), c.recvType(cls)} {
			body = strings.ReplaceAll(body, "["+t+"](self", "[any](self")
		}
	}
	rest := strings.ReplaceAll(body, "[any](self", "")
	// a plain free func of the owner (its overloads too: Kernel___Sleep0) takes self as any or the owner's interface (decision 210)
	plainLit := e.Owner.Name + "_"
	plainMatch := func(s string) bool { _, n := cutPlainSelf(s, plainLit, false); return n > 0 }
	plainCut := func(s string) string { out, _ := cutPlainSelf(s, plainLit, true); return out }
	if regexp.QuoteMeta(e.Owner.Name) != e.Owner.Name { // never for a Ruby constant; keeps the regexp's meaning if a name ever carries metacharacters
		plain := c.re(`\b` + e.Owner.Name + `_\w+\(self\b`)
		plainMatch = plain.MatchString
		plainCut = func(s string) string { return plain.ReplaceAllString(s, "") }
	}
	if e.Owner.universal {
		rest = plainCut(rest)
	}
	if oi := c.ownerIface(cls, e); oi != "" && (strings.Contains(rest, "["+oi+"](self") || plainMatch(rest)) {
		iface, rest = oi, plainCut(strings.ReplaceAll(rest, "["+oi+"](self", "")) // self is recv asserted to the owner's interface
	} else if oi == "" && !e.Owner.universal && !e.Owner.IsModule && e.Owner.isStruct() && len(e.Owner.TypeParams) == 0 && c.plainSelf(e.M) && plainMatch(rest) {
		// a class object's body already calls the owner's plain func (Module_ConstGet(self, ...)): the same text serves every metaclass
		iface, rest = e.Owner.objType(), plainCut(rest)
	}
	if mod := e.Owner; iface == "" && mod.IsModule && !mod.universal && len(mod.TypeParams) == 0 && c.dispatchable(mod, e.M) {
		// a dispatched module method's body over the constraint itself (decision 208), the same for every includer
		// (decision 234): self asserted to the constraint
		inst := c.re(`\b` + regexp.QuoteMeta(freeFuncName(e.M)) + `\[[^\]]*\]\(self\b`)
		if inst.MatchString(body) {
			cons := mod.selfName() + "[any]"
			body = inst.ReplaceAllString(body, freeFuncName(e.M)+"["+cons+"](self")
			iface, rest = cons, inst.ReplaceAllString(rest, "")
		}
	}
	call := "self." + e.M.GoName + "("
	if !w.private && iface == "" && strings.Contains(rest, call) {
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(*e, cls)
		ps, ret := c.sig(e.M, env)
		iface = fmt.Sprintf("interface{ %s(%s) %s }", e.M.GoName, ps, ret)
		rest = strings.ReplaceAll(rest, call, "")
	}
	names := c.re(`\b` + regexp.QuoteMeta(cls.Name) + `(I|_Meta|_class)?\b`) // the class's own Go names, not a substring (rbArity holds an A)
	if strings.Contains(rest, "self") && selfIdent.MatchString(rest) || strings.Contains(body+iface, cls.Name) && names.MatchString(body+iface) {
		return "", "", false
	}
	return iface, body, true
}

// cutPlainSelf is `\b<lit>\w+\(self\b` removed from s (build) and counted, without the regexp engine: the bodies are scanned once per wrapper.
func cutPlainSelf(s, lit string, build bool) (string, int) {
	var out strings.Builder
	n, last := 0, 0
	for from := 0; ; {
		k := strings.Index(s[from:], lit)
		if k < 0 {
			break
		}
		k += from
		from = k + 1
		if k > 0 && isASCIIWord(s[k-1]) {
			continue // \b before the owner's name
		}
		j := k + len(lit)
		for j < len(s) && isASCIIWord(s[j]) {
			j++
		}
		if j == k+len(lit) || !strings.HasPrefix(s[j:], "(self") {
			continue
		}
		e := j + len("(self")
		if e < len(s) && isASCIIWord(s[e]) {
			continue // \b after self
		}
		if build {
			out.WriteString(s[last:k])
		}
		n++
		last, from = e, e
	}
	if !build || n == 0 {
		return s, n
	}
	out.WriteString(s[last:])
	return out.String(), n
}

// unboxLeafSelf: self is never nil, so a leaf's `-> self` needs no rbPtrAny and shares one [any] arm (decision 398).
func unboxLeafSelf(cls *Class, body string) string {
	box := "return rbPtrAny[" + cls.Name + "]("
	if i := strings.Index(body, box); cls.leaf() && i >= 0 && strings.HasSuffix(body, "[*"+cls.Name+"](self))\n") && strings.Count(body[i:], "\n") == 1 {
		return body[:i] + "return " + body[i+len(box):len(body)-2] + "\n"
	}
	return body
}

// shareableClass: a struct class, or a generic class whose wrapper body is a universal owner's naming none of its type
// parameters (the same text for every instantiation, so one arm instead of a case per element type in _DynGenN, #96).
func (c *Compiler) shareableClass(w dynWrapper, name string) bool {
	cls := w.cls
	if dynPinned[name] {
		return false
	}
	if len(cls.TypeParams) > 0 {
		return w.e.Owner.universal && !mentionsIdent(w.body, cls.TypeParams)
	}
	return cls.isStruct()
}

// emitDynArms emits the shared public (or private) arms: a switch on the
// receiver's class ID, so only the classes that reach the method take it.
func (c *Compiler) emitDynArms(arms []*dynArm, private bool) {
	var cases []*dynArm
	for _, a := range arms {
		if a.private == private {
			cases = append(cases, a)
		}
	}
	if len(cases) == 0 {
		return
	}
	cond := ""
	if private {
		cond = " && how != rbCall"
	}
	c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok%s {\n", cond)
	arm := func(a *dynArm) {
		switch {
		case a.iface != "":
			c.w("\tif self, ok := recv.(%s); ok {\n%s}\n", a.iface, a.body)
		case selfIdent.MatchString(a.body):
			c.w("\tself := recv\n%s", a.body)
		default:
			c.w("%s", a.body)
		}
	}
	c.w("\tswitch id._ClassID() {\n")
	for _, a := range cases {
		c.w("\tcase %s:\n", strings.Join(a.ids, ", "))
		arm(a)
	}
	c.w("\tdefault:\n\t}\n\t}\n") // default: gocritic rejects a one-case switch; Go compiles the cases to a binary search
}

// dynWrapperBodyOrWarn is cls's wrapper body for e, or "" with a warning
// when its types cannot cross `any`: calling it dynamically raises
// NoMethodError.
func (c *Compiler) dynWrapperBodyOrWarn(cls *Class, e *entry, name string) (body string) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			c.warn(nil, nil, "%s#%s cannot be called dynamically: %s", cls.RubyName, name, ce.Error())
			body = ""
		}
	}()
	return c.dynWrapperBody(cls, e)
}

// emitDynWrapper emits cls's DynName (_DynName for a private method).
func (c *Compiler) emitDynWrapper(cls *Class, e *entry, name string, private bool, body string) {
	prefix := "Dyn"
	if private {
		prefix = "_Dyn"
	}
	if cls.RubyName == "Boolean" {
		body = c.boolMarkerBody(name, prefix, body)
	}
	c.w("func (self %s) %s%s(args ...any) any {\n%s}\n\n", c.recvType(cls), prefix, goMethodName(name), body)
	c.dynWrapped[cls] = append(c.dynWrapped[cls], dynWrapped{name: name, goName: prefix + goMethodName(name), own: !e.M.File.isPrelude()})
}

// boolMarkerBody is Boolean's wrapper body for name when the program reopens TrueClass or FalseClass with it: true and
// false answer their own class's method, as a static call does (markerCall, decision 241), not Object's (Homebrew's
// `false.present?` through an untyped value was true; decision 291).
func (c *Compiler) boolMarkerBody(name, prefix, body string) string {
	var b strings.Builder
	for _, side := range []struct {
		cls, cond string
	}{{"TrueClass", "self"}, {"FalseClass", "!self"}} {
		k := c.classes[side.cls]
		if k == nil {
			continue
		}
		if m := k.Methods[name]; m != nil && m.File != nil && !m.File.isPrelude() {
			fmt.Fprintf(&b, "\tif %s {\n\t\treturn (*%s)(nil).%s%s(args...)\n\t}\n", side.cond, k.Name, prefix, goMethodName(name))
		}
	}
	return b.String() + body
}

func (c *Compiler) dynWrapperBody(cls *Class, e *entry) string {
	m := e.M
	file := cls.File
	if m.File != nil {
		file = m.File // default arguments are read from the method's source
	}
	f := c.newFctx(file, cls, nil)
	f.lex = m.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	f.indent = 1
	f.implicitCall = true // the method may be private: method_missing, or one send reaches
	f.plainCalls = cls.isStruct()
	recv := expr{code: "self", typ: cls.instance()}
	if cls.metaOf != nil || cls.isStruct() {
		recv.typ = TClass{C: cls}
		if e.Owner.IsModule && !e.Owner.universal && e.Entry != nil && e.Entry != cls && e.Entry.isStruct() && c.constraintNamesSelf(e.Owner) {
			// the module body is instantiated at its includer, as cls's forwarder is (selfTypeFor): the includer's
			// methods return its interface, which a subclass's Self would not satisfy (decision 217)
			recv.typ = TClass{C: e.Entry}
		}
	}
	env := composeEnv(e.Env, nil)
	for _, p := range cls.TypeParams {
		env[p] = TVar{Name: p}
	}
	for _, p := range m.TypeParams { // a block-taking generic (`Mutex#synchronize`'s T) called dynamically: untyped (decision 243)
		env[p] = TAny{}
	}
	env["Self"] = recv.typ
	cls.containerEnv(env)
	c.emitDynCall(f, e, recv, env, true)
	return f.buf.String()
}

// emitDynCall emits a call to e from an `args ...any` list: MRI's arity check, each argument converted (TypeError), then the typed call, one arm per count of optional arguments; mix adds the numeric coercion arms, which name the receiver `self`.
func (c *Compiler) emitDynCall(f *fctx, e *entry, recv expr, env map[string]Type, mix bool) {
	m := e.M
	if m.postCount() > 0 {
		if c.emitDynPostCall(f, e, recv, env) {
			return
		}
		f.emit("panic(NewArgumentError(Ref(String(%q))))", "rugaroo: "+m.String()+" takes post parameters, which a call on an untyped value cannot pass (decision 23)")
		return
	}
	var req, opt int
	var rest *Param
	for i := range m.Params {
		p := &m.Params[i]
		switch {
		case p.Keyword:
		case p.Rest:
			rest = p
		case p.Default != nil:
			opt++
		default:
			req++
		}
	}
	c.dynBlockTwin(f, e, recv, env)
	blk := c.dynCallBlock(f, m, env)
	var kwh *exprNode // the call's keywords, passed on as **kwh (#51)
	if m.hasKeywords() {
		f.emit("kwh, args := rbDynKw(args, %d)", req)
		kwh = &exprNode{e: expr{code: "kwh", typ: TClass{C: c.classes["Hash"], Args: []Type{TClass{C: c.classes["Symbol"]}, TAny{}}}}}
	}
	maxArgs := req + opt
	if rest != nil {
		maxArgs = -1
	}
	c.dynWideClassOverloads(f, e, recv, req, maxArgs)
	c.dynArityOverloads(f, e, recv, env, req, maxArgs)
	f.emit("rbArity(len(args), %d, %d)", req, maxArgs)
	if mix && rest == nil && opt == 0 {
		c.dynTwinsAndNumbers(f, e, recv, env)
	}
	if rest == nil {
		c.dynClassOverloads(f, e, recv, req, opt)
	}
	if m.Name == "<=>" && len(m.Params) == 1 && req == 1 {
		if t, ok := subst(m.Params[0].Type, env).(TClass); ok { // MRI's <=> answers nil for an incomparable argument
			f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(t))
			f.emit("\treturn nil")
			f.emit("}")
		}
	}
	c.emitHashKeyMiss(f, m, env, req)
	c.emitFetchDefault(f, m, env)
	call := func(k int, withRest bool) {
		nodes := make([]parser.Node, 0, k+1)
		for i := range k {
			t := subst(m.Params[i].Type, env)
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(t, i), typ: t}})
		}
		if withRest {
			t := subst(rest.Type, env)
			code := fmt.Sprintf("rbRest[%s](args, %d, %q)...", c.goType(t), k, t.String())
			if isAny(t) {
				code = fmt.Sprintf("args[%d:]...", k) // untyped takes anything, nil included
			}
			nodes = append(nodes, &exprNode{e: expr{code: code, typ: t}})
		}
		if kwh != nil {
			nodes = append(nodes, &parser.KeywordHashNode{Elements: []parser.Node{&parser.AssocSplatNode{Value: kwh}}})
		}
		if m.Iterator && blk != nil {
			c.dynIterCall(f, e, recv, env, nodes)
			return
		}
		f.emitDynReturn(f.callEntry(&parser.NilNode{}, e, recv, nodes, blk))
	}
	if opt == 0 {
		call(req, rest != nil)
		return
	}
	f.emit("switch len(args) {")
	for k := req; k < req+opt; k++ {
		f.emit("case %d:", k)
		f.indent++
		call(k, false)
		f.indent--
	}
	f.emit("default:")
	f.indent++
	call(req+opt, rest != nil)
	f.indent--
	f.emit("}")
}

// dynArityOverloads routes an argument count the method does not take to its decision-12 `__<name>_<count>`, as a typed call does.
func (c *Compiler) dynArityOverloads(f *fctx, e *entry, recv expr, env map[string]Type, req, maxArgs int) {
	m, cls := e.M, classOf(recv.typ)
	prefix := "__" + overloadBase(m.Name) + "_"
	if strings.HasPrefix(m.Name, "__") && strings.HasSuffix(m.Name, "_0") {
		prefix = strings.TrimSuffix(m.Name, "0") // dynEntry's block-less twin: `all?(Integer)` still takes `__all_q_1`
	} else if cls == nil || strings.HasPrefix(m.Name, "__") {
		return
	}
	if cls == nil {
		return
	}
	for _, x := range cls.methodSet() {
		k, ok := strings.CutPrefix(x.M.Name, prefix)
		if !ok {
			continue // before Atoi: its failure allocates an error for every other method of the class
		}
		n, err := strconv.Atoi(k)
		tw := x.M
		if err != nil || n >= req && (maxArgs < 0 || n <= maxArgs) || tw.generic() || tw.Block != nil || len(tw.Params) != n || tw.hasKeywords() {
			continue
		}
		c.resolveMethod(tw)
		if slices.ContainsFunc(tw.Params, func(p Param) bool { return p.Rest || p.Default != nil }) {
			continue
		}
		envT := maps.Clone(env) // the overload's owner params (Enumerable's E) in the class's terms
		for k, v := range x.Env {
			envT[k] = subst(v, env)
		}
		nodes := make([]parser.Node, n)
		for i := range n {
			t := subst(tw.Params[i].Type, envT)
			nodes[i] = &exprNode{e: expr{code: c.dynArg(t, i), typ: t}}
		}
		f.emit("if len(args) == %d {", n)
		f.indent++
		f.emitDynReturn(f.callEntry(&parser.NilNode{}, cls.lookup(tw.Name), recv, nodes, nil))
		f.indent--
		f.emit("}")
	}
}

// emitDynReturn ends a Dyn wrapper arm with res boxed as untyped; a void call runs, then returns nil unless it never returns.
func (f *fctx) emitDynReturn(res expr) {
	if !isVoid(res.typ) {
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
		return
	}
	if res.code != "" {
		f.emit("%s", res.code)
	}
	if !res.noreturn {
		f.emit("return nil")
	}
}

// rbDynFn is the Go name of the dispatcher for Ruby method name.
func rbDynFn(name string) string { return "rbDyn" + goMethodName(name) }

// dynClassOverloads picks a decision-12 overload by its first argument's run-time class, as a typed call picks it by
// the static one: `x.gsub(/re/, s)` on an untyped String is `__gsub_regexp`, not gsub's (String, String).
// With optional params (Time.utc's), each argument count calls the twin with that many, so its defaults run (decision 416).
func (c *Compiler) dynClassOverloads(f *fctx, e *entry, recv expr, req, opt int) {
	m, cls := e.M, classOf(recv.typ)
	if cls == nil || len(m.Params) == 0 || strings.HasPrefix(m.Name, "__") {
		return
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	for _, x := range cls.methodSet() {
		tw := x.M
		if !strings.HasPrefix(tw.Name, prefix) || tw.generic() || tw.Block != nil || len(tw.Params) != len(m.Params) {
			continue
		}
		c.resolveMethod(tw)
		env := composeEnv(x.Env, nil)
		cls.containerEnv(env) // a Set subclass's E is untyped
		t, ok := subst(tw.Params[0].Type, env).(TClass)
		if !ok || typeEq(t, subst(m.Params[0].Type, env)) {
			continue
		}
		te := cls.lookup(tw.Name)
		f.emit("if x, ok := %s; ok {", c.dynClassTest(t))
		f.indent++
		nodes := []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}
		for i := 1; i < len(tw.Params); i++ {
			pt := tw.Params[i].Type
			nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(pt, i), typ: pt}})
		}
		if opt == 0 {
			f.emitDynReturn(f.callEntry(&parser.NilNode{}, te, recv, nodes, nil))
		} else {
			for k := max(req, 1); k <= req+opt; k++ {
				f.emit("if len(args) == %d {", k)
				f.indent++
				f.emitDynReturn(f.callEntry(&parser.NilNode{}, te, recv, nodes[:k], nil))
				f.indent--
				f.emit("}")
			}
		}
		f.indent--
		f.emit("}")
	}
}

// dynClassTest is the Go test that args[0] is a t, for an overload picked by class: a generic instantiation converts
// (a gem's Range of untyped bounds is String#slice's Range[Integer]: ruby-macho's LCStr, decision 325).
func (c *Compiler) dynClassTest(t TClass) string {
	if len(t.Args) > 0 {
		return "rbConv[" + c.goType(t) + "](args[0])"
	}
	return "args[0].(" + c.goType(t) + ")"
}

// dynTwinsAndNumbers picks a twin by the argument's class (`xs * ","` is
// Array#*'s String twin) and, on a number, mixes the numeric classes.
func (c *Compiler) dynTwinsAndNumbers(f *fctx, e *entry, recv expr, env map[string]Type) {
	cls := classOf(recv.typ)
	if cls == nil {
		return
	}
	if numLevel(cls) < 0 {
		c.dynNumericTwins(f, cls, e, recv)
		return
	}
	c.dynNumericTwins(f, cls, e, expr{code: "self", typ: TClass{C: cls}})
	c.dynNumericMix(f, e, env)
	c.dynNumericTower(f, cls, e, env)
}

// dynNumericMix emits a number wrapper's answer to an argument of another
// number class, as numericMix compiles typed calls: Comparable's methods
// run on rbNum, and an Integer's own operator with a Float is redone by
// its Float, whose wrappers widen Integer arguments (rbAs).
func (c *Compiler) dynNumericMix(f *fctx, e *entry, env map[string]Type) {
	m := e.M
	if len(m.Params) == 0 {
		return
	}
	for _, p := range m.Params {
		if numLevel(classOf(subst(p.Type, env))) < 0 {
			return
		}
	}
	cond := "rbNumMixed(self, args)"
	var ret string
	switch {
	case m.Owner == c.classes["Comparable"]:
		args := make([]string, len(m.Params))
		for i := range args {
			args[i] = c.dynArg(TAny{}, i)
		}
		cond, ret = "rbNumOther(self, args)", f.rbNumCall(nil, m, "self", args).code
	case isClass(env["Self"], "Integer") && !slices.ContainsFunc(m.Params, func(p Param) bool { return !isNumeric(subst(p.Type, env)) }):
		if fe, private := c.dynEntry(c.classes["Float"], m.Name); fe == nil || private {
			return
		}
		ret = "Float(self).Dyn" + goMethodName(m.Name) + "(args...)"
	default:
		return
	}
	f.emit("if %s {", cond)
	f.emit("\treturn %s", ret)
	f.emit("}")
}

// dynNumericTwins is decision 12's choice of a `__<op>_<class>` twin by the argument's class, made at run time for a number's one-argument method.
func (c *Compiler) dynNumericTwins(f *fctx, cls *Class, e *entry, recv expr) {
	m := e.M
	if len(m.Params) != 1 || !classTwins(m.Name) {
		return
	}
	open := false
	for _, name := range append(slices.Clone(numTower), "String") {
		k := c.classes[name]
		if k == nil {
			continue
		}
		tw := cls.lookup("__" + overloadBase(m.Name) + "_" + snake(k.RubyName))
		if tw == nil || tw.M.generic() || tw.M.Block != nil || len(tw.M.Params) != 1 {
			continue
		}
		if !open {
			f.emit("switch x := args[0].(type) {") // one type per case: the pruner keeps a case only when its class is kept (no Rational code without a Rational)
			open = true
		}
		t := TClass{C: k}
		f.emit("case %s:", c.goType(t))
		f.indent++
		f.emitDynReturn(f.callEntry(&parser.NilNode{}, tw, recv, []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}, nil))
		f.indent--
	}
	if open {
		f.emit("default:") // gocritic rejects a one-case switch
		f.emit("}")
	}
}

// dynNumericTower is numericTower at run time: an argument of another number class the parameter does not take is coerced with self up the tower (rbNumCoerce), and the method sent again.
func (c *Compiler) dynNumericTower(f *fctx, cls *Class, e *entry, env map[string]Type) {
	m := e.M
	if len(m.Params) != 1 || !towerOps[m.Name] || !typeEq(subst(m.Params[0].Type, env), TClass{C: cls}) {
		return
	}
	f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(TClass{C: cls}))
	gn := "Dyn" + c.dynGoName(m.Name)
	f.emit("\tif x, y, ok := rbNumCoerce(self, args[0]); ok {")
	// x's own wrapper, not the rbDyn dispatcher, which a pinned wrapper (DynOp_cmp) would keep in every program
	f.emit("\t\tif r, ok := x.(interface{ %s(...any) any }); ok {", gn)
	f.emit("\t\t\treturn r.%s(y)", gn)
	f.emit("\t\t}")
	f.emit("\t\tpanic(rbNoMethod(%q, x, false))", m.Name)
	f.emit("\t}")
	if c.ownCoerce() { // an object's coerce (bindata's primitives answer it through method_missing), as MRI's
		c.noteDyn("coerce")
		c.noteRespond("coerce") // method_missing asks respond_to?(:coerce) of what it delegates to
		f.emit("\tif _, ok := args[0].(interface{ DynCoerce(...any) any }); ok || rbHasMissing(args[0]) {")
		f.emit("\t\tif p := rbToAry(rbDynCoerce(rbCall, args[0], self)).s; len(p) == 2 {")
		f.emit("\t\t\tif r, ok := p[0].(interface{ %s(...any) any }); ok {", gn)
		f.emit("\t\t\t\treturn r.%s(p[1])", gn)
		f.emit("\t\t\t}")
		f.emit("\t\t}")
		f.emit("\t}")
	}
	f.emit("}")
}

// emitHashKeyMiss: a dynamic Hash lookup whose key cannot be one of its keys misses (bindata's `choices[nil]`).
func (c *Compiler) emitHashKeyMiss(f *fctx, m *Method, env map[string]Type, req int) {
	miss, ok := hashKeyMiss[m.Name]
	if !ok || m.Owner != c.classes["Hash"] || req < 1 {
		return
	}
	t := subst(m.Params[0].Type, env)
	if isAny(t) {
		return
	}
	if m.Name == "[]" {
		miss = "rbUnbox(Opt(Hash_Default[" + c.goType(env["K"]) + ", " + c.goType(env["V"]) + "](self)))"
	}
	f.emit("if _, ok := rbConv[%s](args[0]); !ok {", c.goType(t))
	f.emit("\treturn %s", miss)
	f.emit("}")
}

// emitFetchDefault: a dynamic `h.fetch(k, default)` whose default is not the Hash's V (concurrent-ruby's
// `opts.fetch(:auto_terminate, true)` on a Hash[Symbol, Integer]) answers the value or that default, as MRI's does
// (decision 374).
func (c *Compiler) emitFetchDefault(f *fctx, m *Method, env map[string]Type) {
	if m.Name != "fetch" || m.Owner != c.classes["Hash"] || len(m.Params) != 2 || isAny(env["V"]) || isAny(env["K"]) {
		return
	}
	f.emit("if len(args) == 2 {")
	f.emit("\tif _, ok := rbConv[%s](rbArg(args, 1)); !ok && rbArg(args, 1) != nil {", c.goType(env["V"]))
	f.emit("\t\tif k, ok := rbConv[%s](args[0]); ok {", c.goType(env["K"])) // a key of another type misses: the default, as MRI's
	f.emit("\t\t\tif v, ok := self.rbGet(k); ok {\n\t\t\t\treturn v\n\t\t\t}\n\t\t}")
	f.emit("\t\treturn rbArg(args, 1)")
	f.emit("\t}")
	f.emit("}")
}

// ownCoerce reports a non-prelude class defining coerce or method_missing: only its objects can answer an Integer
// operator's coerce call (decision 324), so other programs keep no coerce dispatcher.
func (c *Compiler) ownCoerce() bool {
	if c.coerceOwned == nil {
		v := slices.ContainsFunc(c.classList, func(k *Class) bool {
			return k.File != nil && !k.File.isPrelude() && (k.Methods["coerce"] != nil || k.Methods["method_missing"] != nil)
		})
		c.coerceOwned = &v
	}
	return *c.coerceOwned
}

// hashKeyMiss is what a Hash lookup answers on a dynamic call whose key cannot be one of its keys.
var hashKeyMiss = map[string]string{"[]": "nil", "key?": "Boolean(false)", "include?": "Boolean(false)", "has_key?": "Boolean(false)",
	"member?": "Boolean(false)", "delete": "nil", "dig": "nil"}

// dynArg converts argument i to type t. Past the first it is read through
// rbArg: gosec's G602 tracks the argument count of each call site, and a
// dispatcher branch for more arguments than one site passes (Logger's
// one-argument `write` against REXML's two) reads past it, though rbArity
// has already ruled that branch out.
func (c *Compiler) dynArg(t Type, i int) string {
	arg := fmt.Sprintf("args[%d]", i)
	if i > 0 {
		arg = fmt.Sprintf("rbArg(args, %d)", i)
	}
	switch t := t.(type) {
	case TAny:
		return arg
	case TOpt:
		return c.optOf(t.Elem, arg)
	case TUnion:
		return c.unionFn(t) + "(" + arg + ")"
	case TClass, TFunc, TNil, TTuple, TVar, TVoid: // converted and checked by rbAs below
	}
	return fmt.Sprintf("rbAs[%s](%s, %q)", c.goType(t), arg, t.String())
}

// emitDynPostCall is a Dyn wrapper's call of a method with post parameters (concurrent-ruby's `evaluate_to(*args,
// block)`, decision 254): the leading parameters from the front of args, the posts from the back, the rest between.
// Only required, rest and post parameters, no keywords or block; false for any other shape.
func (c *Compiler) emitDynPostCall(f *fctx, e *entry, recv expr, env map[string]Type) bool {
	m := e.M
	if m.Block != nil || m.hasKeywords() || m.calleeDefaults {
		return false
	}
	var lead, post []Param
	var rest *Param
	for i := range m.Params {
		p := &m.Params[i]
		switch {
		case p.Keyword || p.KwRest || p.Default != nil:
			return false
		case p.Rest:
			rest = p
		case p.Post:
			post = append(post, *p)
		default:
			lead = append(lead, *p)
		}
	}
	maxArgs := len(lead) + len(post)
	if rest != nil {
		maxArgs = -1
	}
	f.emit("rbArity(len(args), %d, %d)", len(lead)+len(post), maxArgs)
	arg := func(t Type, idx string) parser.Node {
		code := "args[" + idx + "]"
		if !isAny(t) {
			code = fmt.Sprintf("rbAs[%s](%s, %q)", c.goType(t), code, t.String())
		}
		return &exprNode{e: expr{code: code, typ: t}}
	}
	nodes := make([]parser.Node, 0, len(m.Params))
	for i, p := range lead {
		nodes = append(nodes, arg(subst(p.Type, env), strconv.Itoa(i)))
	}
	if rest != nil {
		t := subst(rest.Type, env)
		code := fmt.Sprintf("rbRest[%s](args[:len(args)-%d], %d, %q)", c.goType(t), len(post), len(lead), t.String())
		if isAny(t) {
			code = fmt.Sprintf("args[%d:len(args)-%d]", len(lead), len(post))
		}
		code = fmt.Sprintf("(&Array[%s]{s: %s})", c.goType(t), code)
		nodes = append(nodes, &parser.SplatNode{Expression: &exprNode{e: expr{code: code, typ: TClass{C: c.classes["Array"], Args: []Type{t}}}}})
	}
	for j, p := range post {
		nodes = append(nodes, arg(subst(p.Type, env), fmt.Sprintf("len(args)-%d", len(post)-j)))
	}
	f.emitDynReturn(f.callEntry(&parser.NilNode{}, e, recv, nodes, nil))
	return true
}

// dynWideClassOverloads is dynClassOverloads for a twin taking another count of arguments than its method
// (String#[]'s `__idx_regexp(re, capture = 0)` beside `[](i)`, decision 254): chosen by the first argument's class
// and the count, before the method's own arity check would reject the call.
func (c *Compiler) dynWideClassOverloads(f *fctx, e *entry, recv expr, req, maxArgs int) {
	m, cls := e.M, classOf(recv.typ)
	if cls == nil || len(m.Params) == 0 || strings.HasPrefix(m.Name, "__") || maxArgs < 0 { // no rest: each arm is guarded by count
		return // (an optional parameter too: kramdown's untyped `value.split(/re/, -1)` is __split_regexp)
	}
	if slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword }) {
		return
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	for _, x := range cls.methodSet() {
		tw := x.M
		if !strings.HasPrefix(tw.Name, prefix) || tw.generic() || tw.Block != nil || len(tw.Params) == len(m.Params) || tw.hasKeywords() {
			continue
		}
		c.resolveMethod(tw)
		treq, tmax := arity(tw)
		if tmax < 0 || treq < 1 || treq == req && tmax == req {
			continue
		}
		env := composeEnv(x.Env, nil)
		cls.containerEnv(env)
		t, ok := subst(tw.Params[0].Type, env).(TClass)
		if !ok || typeEq(t, subst(m.Params[0].Type, env)) {
			continue
		}
		te := cls.lookup(tw.Name)
		f.emit("if len(args) >= %d && len(args) <= %d {", treq, tmax)
		f.emit("\tif x, ok := %s; ok {", c.dynClassTest(t))
		f.indent += 2
		for k := treq; k <= tmax; k++ { // one `if` per count: gocritic rejects a one-case switch
			nodes := []parser.Node{&exprNode{e: expr{code: "x", typ: t}}}
			for i := 1; i < k; i++ {
				pt := subst(tw.Params[i].Type, env)
				nodes = append(nodes, &exprNode{e: expr{code: c.dynArg(pt, i), typ: pt}})
			}
			f.emit("if len(args) == %d {", k)
			f.indent++
			f.emitDynReturn(f.callEntry(&parser.NilNode{}, te, recv, nodes, nil))
			f.indent--
			f.emit("}")
		}
		f.indent -= 2
		f.emit("\t}")
		f.emit("}")
	}
}
