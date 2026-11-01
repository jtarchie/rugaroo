package compiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"slices"
	"strings"
)

// Dynamic dispatch on untyped values, generated from the closed world:
// no reflection. For every method name called on an untyped value there is
// a `rbDynName` dispatcher, and a `DynName(args ...any) any` wrapper on
// every concrete class whose public, non-generic, block-less method has
// that name (`_DynName` when the method is private: only send and
// receiver-less calls reach it). Wrappers check arity, convert arguments to
// the declared types and box the result, reusing the typed call path.

// emitDynamic emits the dispatchers that became needed (dynNeeded) and reports whether there were any: with the
// computed-send switches in play (dynAll) nearly every method is noted, and a program reaches few of them. A wrapper
// body notes what it calls (a default argument calling something dynamically), and a name noted that late is needed
// by what noted it, so this runs to a fixed point.
func (c *Compiler) emitDynamic(reached, selected func(string) bool) bool {
	if c.dynLazy == nil {
		c.noteDyn("<=>") // rbCmp falls back to DynOp_cmp, and which instantiations see untyped values is unknown here
		c.noteDyn("+")   // rbViewSum's fallback for elements that are not numbers
		c.noteDyn("===") // the views' grep/count/include-pattern fallbacks
		if c.dynAll {
			for _, name := range c.allMethodNames(!c.dynAllStrict) {
				c.noteDyn(name)
				c.noteRespond(name)
			}
			for _, name := range slices.Clone(c.dynNames) { // a name called on an untyped value may reach a method_missing asking respond_to? of it (bindata's BasePrimitive: `kind.to_i`)
				c.noteRespond(name)
			}
			for _, name := range []string{"to_int", "to_str"} { // rbImplicitConv asks an object's method_missing
				c.noteRespond(name)
			}
		}
		c.dynLazy = map[string]bool{}
		for _, n := range slices.Concat(c.dynNames, c.respondNames) {
			c.dynLazy[n] = true
		}
	}
	c.dynEvery = c.dynEvery || c.dynAll && reached("rbSendByName")
	c.respondEvery = c.respondEvery || c.dynEvery || c.dynAll && reached("rbRespondsByName") // decision 319: markers only, no wrappers; the eager retry sets both at once
	if !c.dynEvery && c.callable == nil && selected("_Call") {
		c.callable = c.callableNames()
	}
	var emitted []string
	for progress := true; progress; { // each pass in name order, not noting order, which discarded emissions shift (decision 432)
		progress = false
		var dyn, resp []string
		for _, n := range c.dynNames {
			if !c.dynOut[n] && c.dynNeeded(n, reached, selected) {
				dyn = append(dyn, n)
			}
		}
		for _, n := range c.respondNames {
			if !c.respondOut[n] && (c.respondEvery || c.dynNeeded(n, reached, selected)) {
				resp = append(resp, n)
			}
		}
		slices.Sort(dyn)
		slices.Sort(resp)
		for _, n := range dyn { // a wrapper body may note more names: the next pass takes them
			c.dynOut[n] = true
			c.emitDynName(n)
			emitted, progress = append(emitted, n), true
		}
		for _, n := range resp {
			c.respondOut[n] = true
			c.emitRespond(n)
			emitted, progress = append(emitted, n), true
		}
	}
	for _, name := range emitted {
		c.emitMarkers("_Private"+goMethodName(name), c.privateIn(name))
	}
	c.emitDynGen()
	return len(emitted) > 0
}

// dynPrivKey prefixes a private method's _DynGen key: NUL cannot start a Ruby name, where "_" also starts a public `_foo`.
const dynPrivKey = "\x00"

// dynGenCase is one wrapper body of a generic class, keyed by its Ruby name (dynPrivKey + name when private).
type dynGenCase struct{ key, body string }

// emitDynGen emits this batch's wrappers of each generic class as one method switching on the name. Go compiles
// every method of a generic type for each instantiation, so 120 DynX methods on Array[E] were 120 bodies per
// element type (64_observable: 10.1k of 51k compiled functions); the batch number keeps names unique across
// emitNext rounds, and the dispatchers this batch emitted name it. The pruner keeps a case only while its name's
// dispatcher is kept (visitSwitch), as a wrapper method was kept only while selected (decision 209).
func (c *Compiler) emitDynGen() {
	if len(c.dynGen) == 0 {
		return
	}
	gen := fmt.Sprintf("_DynGen%d", c.dynGenRound)
	c.dynGenRound++
	for _, cls := range c.classList {
		cases := c.dynGen[cls]
		if len(cases) == 0 {
			continue
		}
		c.w("func (self %s) %s(rbDynName string, args []any) (any, bool) {\n\tswitch rbDynName {\n", c.recvType(cls), gen)
		for _, k := range cases {
			c.w("\tcase %q:\n%s", k.key, okReturns(k.body))
		}
		c.w("\tdefault:\n\t}\n\treturn nil, false\n}\n\n") // default: gocritic rejects a one-case switch
	}
	c.dynGen = nil
}

// okReturns rewrites a wrapper body's `return v` (outside closures) to `return v, true`.
func okReturns(body string) string {
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", "package p\nfunc _() any {\n"+body+"}\n", 0)
	if err != nil {
		panic(fmt.Sprintf("rugaroo: wrapper body does not parse: %v\n%s", err, body))
	}
	fd, _ := f.Decls[0].(*ast.FuncDecl)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			n.Results = append(n.Results, ast.NewIdent("true"))
		}
		return true
	})
	var b strings.Builder
	for _, st := range fd.Body.List {
		b.WriteString("\t\t")
		_ = printer.Fprint(&b, fset, st)
		b.WriteString("\n")
	}
	return b.String()
}

// dynNeeded: name was noted by an emitted body, or kept code calls its dispatcher, asks respond_to? of it, or asserts its wrapper (rbCmp's DynOp_cmp, the json generator's DynToJson).
func (c *Compiler) dynNeeded(name string, reached, selected func(string) bool) bool {
	if c.dynEvery || !c.dynLazy[name] || c.callable[name] {
		return true
	}
	gn := goMethodName(name)
	return reached("rbDyn"+gn) || reached("rbResponds"+gn) || reached("rbHas_Private"+gn) || selected("Dyn"+gn) || selected("_Dyn"+gn)
}

// dynWrapped is one Dyn wrapper emitted on a class; own when a user file defines the method.
// goName is "" for a class a shared arm of the dispatcher serves.
type dynWrapped struct {
	name, goName string
	own          bool
}

// callByName: what minitest calls by name, tests and the predicates and operators of
// assert_predicate/assert_operator. Only these, because naming a common method
// (to_s, name) here would keep its Dyn wrapper on every class (decision 49).
func callByName(name string) bool {
	_, op := opNames[name]
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "?") || op
}

// callableNames are what the _Call tables switch over: the callByName methods user code defines on user classes, own or inherited. Only their dispatchers go out, where a computed send needs every one.
func (c *Compiler) callableNames() map[string]bool {
	out := map[string]bool{}
	for _, cls := range c.classList {
		if cls.File == nil || cls.File.isPrelude() || !cls.isStruct() || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		for _, e := range cls.methodSet() {
			if !e.M.File.isPrelude() && callByName(e.M.Name) {
				out[e.M.Name] = true
			}
		}
	}
	return out
}

// emitCallTables gives each user-defined class _Call(name, args...): send
// over the methods user code defined on it, own or inherited. It is the
// narrow alternative to rbSendByName, whose switch over every method name
// keeps the whole prelude in a build; minitest calls tests through it
// (decision 79). Only rbMtCall asks for _Call, so other programs prune it.
func (c *Compiler) emitCallTables() {
	for _, cls := range c.classList {
		if cls.File == nil || cls.File.isPrelude() || !cls.isStruct() || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		var cases []dynWrapped
		for _, w := range c.dynWrapped[cls] {
			if w.own && callByName(w.name) {
				cases = append(cases, w)
			}
		}
		c.w("func (self %s) _Call(name string, args ...any) (any, bool) {\n", c.recvType(cls))
		call := func(w dynWrapped) string {
			if w.goName == "" {
				return fmt.Sprintf("rbDyn%s(rbFCall, self, args...)", goMethodName(w.name))
			}
			return "self." + w.goName + "(args...)"
		}
		switch len(cases) {
		case 0:
		case 1: // gocritic rejects a one-case switch
			c.w("\tif name == %q {\n\t\treturn %s, true\n\t}\n", cases[0].name, call(cases[0]))
		default: // Go compiles a string switch to a search, not a chain of compares
			c.w("\tswitch name {\n")
			for _, w := range cases {
				c.w("\tcase %q:\n\t\treturn %s, true\n", w.name, call(w))
			}
			c.w("\t}\n")
		}
		c.w("\treturn nil, false\n}\n\n")
	}
}

// dynEntry is the method a dynamic call of name reaches on cls, if it can
// be called without a block and without type arguments, and whether it is
// private (the dispatchers call method_missing and respond_to_missing?
// whatever their visibility).
func (c *Compiler) dynEntry(cls *Class, name string) (*entry, bool) {
	if cls.IsModule || cls.universal || name == "initialize" {
		return nil, false
	}
	e := cls.lookup(name)
	if e != nil && e.M.Block != nil && !c.dynBlockable(name, e.M) && !c.dynBlockNames[name] && (e.M.Owner == nil || !e.M.Owner.universal) { // not Kernel's open on every class
		if b := blocklessTwin(cls, name); b != nil {
			e = b
		}
	}
	if e == nil || e.M.generic() && (e.M.Block == nil || !c.dynBlockable(name, e.M)) || e.M.Block != nil && !c.dynBlockable(name, e.M) {
		return nil, false
	}
	if len(cls.TypeParams) > 0 && growsTypeParams(cls, e) {
		return nil, false
	}
	if cls.hashBase && e.Owner != cls && e.Owner.IsModule && slices.Contains(cls.Super.ancestors(), e.Owner) {
		return nil, false // the embedded base's own module method (Pathname's Comparable): its Self is the base, not cls
	}
	return e, e.M.Private && !rubyPrivate[name] && name != "method_missing"
}

// blocklessTwin is the form a dynamic call without a block takes of a block method no dynamic call passes one: its
// `__name_enum` (net-ftp's `Socket.tcp(host, port)`, decision 416), else decision 12's zero-argument twin
// (`public_send(:all?)` on an Enumerable, ruby/spec's `x.should.all?`, decision 417).
func blocklessTwin(cls *Class, name string) *entry {
	if en := cls.lookup("__" + overloadBase(name) + "_enum"); en != nil && en.M.Block == nil && !en.M.generic() {
		return en
	}
	if z := cls.lookup("__" + overloadBase(name) + "_0"); z != nil && z.M.Block == nil && !z.M.generic() && len(z.M.Params) == 0 {
		return z
	}
	return nil
}

// privateIn lists the concrete classes on which name is a private method.
func (c *Compiler) privateIn(name string) []*Class {
	if len(c.classList) != c.privateInN { // asked per dynamic name, twice: 5 CPU-s of brew's gen unmemoized
		c.privateInMemo, c.privateInN = map[string][]*Class{}, len(c.classList)
	}
	if out, ok := c.privateInMemo[name]; ok {
		return out
	}
	var out []*Class
	defer func() { c.privateInMemo[name] = out }()
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		if e := cls.lookup(name); e != nil && e.M.Private {
			out = append(out, cls)
		}
	}
	return out
}

// growsTypeParams reports whether a generic class's method mentions its
// type parameters other than bare (E, E?), as the class itself
// (Array[E] on Array) or as an Array of one (Array[E] on Set). Wrapping such a method instantiates some other
// generic at a type built from E (Hash[E, Integer], Array[[E, Integer]]),
// whose own methods and wrappers can instantiate a bigger one still: a Go
// instantiation cycle.
func growsTypeParams(cls *Class, e *entry) bool {
	own := func(t TClass) bool {
		if t.C != cls || len(t.Args) != len(cls.TypeParams) {
			return false
		}
		for i, a := range t.Args {
			if v, ok := a.(TVar); !ok || v.Name != cls.TypeParams[i] {
				return false
			}
		}
		return true
	}
	// Set#to_a's Array[E]: an Array of a bare parameter, whose own wrappers already leave out what grows (another
	// class there, Hash[K, V]'s, closes a Go instantiation cycle through its pairs)
	bareArgs := func(t TClass) bool {
		if t.C.Name != "Array" || len(t.Args) != 1 {
			return false
		}
		v, ok := t.Args[0].(TVar)
		return ok && slices.Contains(cls.TypeParams, v.Name)
	}
	var grows func(t Type) bool
	grows = func(t Type) bool {
		switch t := t.(type) {
		case TOpt:
			return grows(t.Elem)
		case TTuple:
			return mentionsVar(t)
		case TClass:
			return !own(t) && !bareArgs(t) && mentionsVar(t)
		case TUnion: // its package-level rbUnion helper cannot name the class's parameters (Set#merge's Array[E] | Set[E])
			return slices.ContainsFunc(t.Members, grows) || mentionsVar(t)
		case TAny, TFunc, TNil, TVar, TVoid: // no class of its own whose wrappers could grow
		}
		return false
	}
	ts := make([]Type, 0, 1+len(e.M.Params))
	ts = append(ts, e.M.Ret)
	for _, p := range e.M.Params {
		ts = append(ts, p.Type)
	}
	for _, t := range ts {
		if grows(subst(t, e.Env)) {
			return true
		}
	}
	return false
}

// mentionsIdent reports whether Go code names any of idents as a whole word.
func mentionsIdent(code string, idents []string) bool {
	return slices.ContainsFunc(idents, func(id string) bool {
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`).MatchString(code)
	})
}

func mentionsVar(t Type) bool {
	var vars []string
	freeVars(t, &vars)
	return len(vars) > 0
}

// dynGoName is the Go name of name's dispatcher. Dispatchers are package-level, so a capital or digit that decision 3 merges (`foo_bar`/`fooBar`) collides across classes too
func (c *Compiler) dynGoName(name string) string {
	gn := goMethodName(name)
	if p, ok := c.dynGo[gn]; ok && p != name {
		c.errorf(nil, nil, "`%s` and `%s` both become Go %s, so dynamic calls cannot tell them apart; rename one", p, name, gn)
	}
	c.dynGo[gn] = name
	return gn
}

// emitDynFallback is a dispatcher's tail before NoMethodError: what a Delegator does not define its object answers, as
// its method_missing does (decisions 294, 295; Homebrew's `dependables.values.map(&:to_a)` of Requirements; a static
// call is decision 118's __getobj__ rewrite instead), then the receiver's own method_missing. Every object has
// Kernel#===, so method_missing is never reached for it.
func (c *Compiler) emitDynFallback(name, gn string) {
	if name == "method_missing" || name == "===" {
		return
	}
	if name != "__getobj__" {
		c.w("\tif o, ok := rbDelegateObj(recv); ok {\n\t\treturn rbDyn%s(how, o, args...)\n\t}\n", gn)
	}
	switch name { // Kernel's block-taking methods on a value whose class defines none of its own (a gem class defines `then`, an Array does not)
	case "then", "yield_self":
		c.w("\tif blk, rest := rbPopBlock(args); blk != nil && len(rest) == 0 {\n\t\treturn blk.fn(recv)\n\t}\n")
	case "tap":
		c.w("\tif blk, rest := rbPopBlock(args); blk != nil && len(rest) == 0 {\n\t\tblk.fn(recv)\n\t\treturn recv\n\t}\n")
	}
	delete(c.dynLazy, "method_missing") // the fallback below needs its wrappers
	c.w("\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(%q)}, args...)...)\n\t}\n", name)
}

// emitDynName emits rbDynName(how, recv, args...): the public wrapper, the
// private one unless the call had a receiver (how is rbCall), then
// method_missing, then NoMethodError. Wrappers whose bodies are the same
// for many classes (an inherited prelude method: `self.ToS()`) become one
// arm of the dispatcher, chosen by class ID, instead of a copy per class.
func (c *Compiler) emitDynName(name string) {
	gn := c.dynGoName(name)
	hidden := len(c.privateIn(name)) > 0
	own, shared := c.dynWrappers(name)
	var genPublic, genPrivate bool
	own = slices.DeleteFunc(own, func(w dynWrapper) bool {
		if seqNames[name] != "" && w.cls.Name == "Array" || hashViewNames[name] != "" && w.cls.Name == "Hash" {
			return true // the view answers it (rbViewCall / rbHashViewCall), once for every element type
		}
		if len(w.cls.TypeParams) == 0 || dynPinned[name] || name == "to_json" || strings.Contains(w.body, "//line") {
			return false
		}
		if c.dynGen == nil {
			c.dynGen = map[*Class][]dynGenCase{}
		}
		key := name
		if w.private {
			key, genPrivate = dynPrivKey+name, true
		} else {
			genPublic = true
		}
		c.dynGen[w.cls] = append(c.dynGen[w.cls], dynGenCase{key: key, body: w.body})
		return true
	})
	gen := fmt.Sprintf("_DynGen%d", c.dynGenRound)
	c.w("func rbDyn%s(how int, recv any, args ...any) any {\n", gn)
	// the arms go first: their class IDs are exact, and a class they serve
	// may still embed its parent's DynX, promoted by Go with the parent as self
	c.emitDynArms(shared, false)
	c.emitDynArms(shared, true)
	c.w("\tif r, ok := recv.(interface{ Dyn%s(...any) any }); ok {\n\t\treturn r.Dyn%s(args...)\n\t}\n", gn, gn)
	if fn := seqNames[name]; fn != "" {
		c.w("\tif s, ok := recv.(rbView); ok {\n\t\tif v, ok := %s(%q, s, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", fn, name)
	}
	if fn := hashViewNames[name]; fn != "" {
		c.w("\tif s, ok := recv.(rbHashView); ok {\n\t\tif v, ok := %s(%q, s, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", fn, name)
	}
	if genPublic {
		c.w("\tif r, ok := recv.(interface{ %s(string, []any) (any, bool) }); ok {\n\t\tif v, ok := r.%s(%q, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", gen, gen, name)
	}
	if procCallNames[name] { // Proc#call's aliases, as the typed path (genProcCall) has them
		c.w("\tif r, ok := rbDynProcCall(recv, args); ok {\n\t\treturn r\n\t}\n")
	}
	kernel := c.nilKernelBody(name) // built once: a body that cannot cross `any` warns
	if m := c.classes["NilClass"].Methods[name]; m != nil && m.File != nil && !m.File.isPrelude() {
		// a reopened NilClass answers nil before Object/Kernel, as markerCall does for a typed nil (decision 281)
		c.w("\tif recv == nil {\n\t\treturn (*NilClass)(nil).Dyn%s(args...)\n\t}\n", gn)
	}
	if v, ok := nilConversions[name]; ok {
		c.w("\tif recv == nil {\n\t\trbArity(len(args), 0, 0)\n\t\treturn %s\n\t}\n", v)
	} else if op, ok := nilOps[name]; ok {
		c.w("\tif recv == nil {\n\t\trbArity(len(args), %d, %d)\n\t\treturn %s\n\t}\n", op.min, op.max, op.code)
	} else if kernel != "" {
		c.w("\tif recv == nil {\n\t\tself := recv\n%s\t}\n", kernel)
	}
	if kernel != "" { // a bare Object (Object.new, main) has only Kernel's and Object's methods; so has a Proc, a Go func (decision 241)
		c.w("\tif _, ok := recv.(*Object); ok || rbIsProc(recv) {\n\t\tself := recv\n%s\t}\n", kernel)
	}
	if hidden {
		c.w("\tif r, ok := recv.(interface{ _Dyn%s(...any) any }); ok && how != rbCall {\n\t\treturn r._Dyn%s(args...)\n\t}\n", gn, gn)
	}
	if genPrivate {
		c.w("\tif r, ok := recv.(interface{ %s(string, []any) (any, bool) }); ok && how != rbCall {\n\t\tif v, ok := r.%s(%q, args); ok {\n\t\t\treturn v\n\t\t}\n\t}\n", gen, gen, dynPrivKey+name)
	}
	c.emitDynFallback(name, gn)
	if hidden {
		c.w("\tif rbHas_Private%s(recv) {\n\t\tpanic(rbPrivateMethod(%q, recv))\n\t}\n", gn, name)
	}
	if name == "===" {
		// Kernel#===: == unless its class defines one.
		// ponytail: a class object held untyped gets ==, not Module#=== (is_a?);
		// needs a generated per-metaclass instance check.
		c.w("\trbArity(len(args), 1, 1)\n\treturn Boolean(rbEq[any](recv, args[0]))\n}\n\n")
	} else {
		c.w("\tpanic(rbNoMethod(%q, recv, how == rbVCall))\n}\n\n", name)
	}
	for _, w := range own {
		c.emitDynWrapper(w.cls, w.e, name, w.private, w.body)
	}
	for _, a := range shared { // _Call reaches a shared arm through the dispatcher
		for _, w := range a.members {
			c.dynWrapped[w.cls] = append(c.dynWrapped[w.cls], dynWrapped{name: name, own: !c.dynGenerated(w.e)})
		}
	}
}

// procCallNames are Proc#call and its aliases: a dispatcher of one of them also calls a Proc held untyped.
var procCallNames = map[string]bool{"call": true, "[]": true, "yield": true, "===": true}
