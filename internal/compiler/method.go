package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

// methodFn reports whether t is a Method or UnboundMethod (kind), and its Proc type F when it still has one (decision 141).
func methodFn(t Type) (kind string, fn TFunc, typed bool) {
	tc, ok := t.(TClass)
	if !ok || len(tc.Args) != 1 || tc.C.RubyName != "Method" && tc.C.RubyName != "UnboundMethod" {
		return "", TFunc{}, false
	}
	fn, typed = tc.Args[0].(TFunc)
	return tc.C.RubyName, fn, typed
}

// genMethodObject is Kernel#method/public_method and Module#instance_method/public_instance_method with a literal name.
func (f *fctx) genMethodObject(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	unbound := name == "instance_method" || name == "public_instance_method"
	if !unbound && name != "method" && name != "public_method" || len(args) != 1 || block != nil {
		return expr{}, false
	}
	if f.resolve(recv.typ, name) != nil {
		return expr{}, false // the receiver's own `method` (Net::HTTP's requests)
	}
	lit := literalName(args[0])
	if lit == "" {
		f.errorf(n, "%s needs a literal name, a Symbol or String: the method is found at compile time (decision 141)", name)
	}
	public := strings.HasPrefix(name, "public_")
	if unbound {
		return f.genUnboundMethod(n, recv, lit, public), true
	}
	return f.genBoundMethod(n, recv, lit, public), true
}

// methodTarget checks that e can stand behind a Method value.
func (f *fctx) methodTarget(n parser.Node, e *entry, name string, public bool) {
	m := e.M
	switch {
	case public && (m.Private || m.Protected):
		f.errorf(n, "undefined public method '%s': it is private", name)
	case m.generic():
		f.errorf(n, "%s is generic: a Method needs one signature (decision 141)", name)
	case needsBlock(m) && !f.lenient():
		f.errorf(n, "%s takes a block, which Method#call cannot pass (decision 141)", name)
	case slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword && !p.KwRest && p.Default == nil }):
		f.errorf(n, "%s has required keywords, which Method#call cannot pass (decision 141)", name)
	}
}

// needsBlock reports a method whose block is required: a Method of it (gem code only, Homebrew's
// `instance_method(:brew).owner`) is built for its reflection, and calling it raises LocalJumpError without running
// it, as the method would at its first yield (decision 281). ponytail: statements before that yield do not run.
func needsBlock(m *Method) bool { return m.Block != nil && !m.Block.Optional }

const noBlockDyn = `func(any, ...any) any { panic(NewLocalJumpError(Ref(String("no block given (yield)")))) }`

// noBlockCall is a needsBlock Method's call: its arguments unused.
func (f *fctx) noBlockCall(args []parser.Node) expr {
	for _, a := range args {
		f.emit("_ = %s", a.(*exprNode).e.code)
	}
	return expr{code: `panic(NewLocalJumpError(Ref(String("no block given (yield)"))))`, noreturn: true}
}

// methodFnType is F: the target's required positional parameters and its result, bound by env.
func (f *fctx) methodFnType(n parser.Node, e *entry, env map[string]Type, self Type) TFunc {
	var ps []Type
	if self != nil {
		ps = append(ps, self)
	}
	for _, p := range e.M.Params {
		if !p.Rest && !p.Keyword && p.Default == nil {
			ps = append(ps, subst(p.Type, env))
		}
	}
	ret := subst(e.M.Ret, env)
	if ret == nil {
		ret = TVoid{}
	}
	ft := TFunc{Params: ps, Ret: ret, Proc: true}
	var vars []string
	freeVars(ft, &vars)
	if slices.ContainsFunc(vars, func(v string) bool { return v != "Self" }) {
		f.errorf(n, "%s's signature %s depends on its receiver's type arguments, which a Method cannot carry (decision 141)", e.M.Name, ft)
	}
	return ft
}

// methodRecv holds a receiver in a temp: Go closures capture variables, and Ruby's Method keeps the object it was taken from.
func (f *fctx) methodRecv(recv expr) expr {
	if recv.code == f.selfCode || recv.lit || recv.classObj {
		return recv
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, recv.code)
	recv.code, recv.view = tmp, ""
	return recv
}

// fieldOf selects a field of the struct code points to; a composite literal `&T{}` needs parentheses first.
func fieldOf(code, field string) string {
	if strings.HasPrefix(code, "&") || strings.HasPrefix(code, "*") {
		return "(" + code + ")." + field
	}
	return code + "." + field
}

// funcClosure is a Go func literal over params whose body is call's result.
func (f *fctx) funcClosure(n parser.Node, params []Type, ret Type, call func(args []parser.Node) expr) string {
	var b strings.Builder
	savedBuf, savedIndent := f.buf, f.indent
	savedBegins, savedRetVar, savedWrap := f.begins, f.retVar, f.wrap
	f.buf, f.indent = &b, 1
	f.begins, f.retVar, f.wrap = 0, "ret_", nil
	f.closures++
	ps := make([]string, len(params))
	nodes := make([]parser.Node, len(params))
	for i, t := range params {
		v := f.newTmp()
		ps[i] = v + " " + f.c.goType(t)
		nodes[i] = &exprNode{Node: n, e: expr{code: v, typ: t}}
	}
	res := call(nodes)
	switch {
	case isVoid(ret):
		if res.code != "" && res.code != "nil" {
			f.emit("%s", res.code)
		}
	case res.noreturn:
		f.emit("%s", res.code)
	default:
		f.emit("return %s", f.coerce(n, res, ret))
	}
	body := b.String()
	f.closures--
	f.buf, f.indent = savedBuf, savedIndent
	f.begins, f.retVar, f.wrap = savedBegins, savedRetVar, savedWrap
	retS := ""
	if !isVoid(ret) {
		retS = " " + f.c.goType(ret)
	}
	return "func(" + strings.Join(ps, ", ") + ")" + retS + " {\n" + body + "}"
}

// dynClosure is a Method's dyn: the target called from an untyped argument list on receiver o, as decision 32's wrappers call it; static calls e's own definition, not an override (an UnboundMethod's).
func (f *fctx) dynClosure(e *entry, recvT Type, env map[string]Type, private, static bool) string {
	if o := e.M.Owner; o != nil && o.IsModule && !o.universal && f.c.goType(recvT) == "any" {
		// a module's free func needs its includer's Go type, known only at run time: the dispatcher finds it
		f.c.noteDyn(e.M.Name)
		how := "rbCall"
		if private {
			how = "rbFCall"
		}
		return fmt.Sprintf("func(o_ any, args ...any) any { return rbDyn%s(%s, o_, args...) }", goMethodName(e.M.Name), how)
	}
	var b strings.Builder
	savedBuf, savedIndent, savedImplicit, savedStatic := f.buf, f.indent, f.implicitCall, f.staticDef
	f.buf, f.indent, f.implicitCall = &b, 1, private
	if static {
		f.staticDef = e.M
	}
	f.closures++
	r := expr{code: "o_", typ: recvT}
	switch {
	case e.M.Owner == nil: // a top-level def ignores its receiver
		r = expr{code: f.selfCode, typ: f.selfType}
		f.emit("_ = o_")
	case e.M.postCount() > 0: // emitDynCall only panics: r_ would be unused
	case f.c.goType(recvT) != "any":
		r.code = "r_"
		f.emit("r_ := rbAs[%s](o_, %q)", f.c.goType(recvT), recvT.String())
	}
	f.c.emitDynCall(f, e, r, env, false)
	f.closures--
	body := b.String()
	f.buf, f.indent, f.implicitCall, f.staticDef = savedBuf, savedIndent, savedImplicit, savedStatic
	return "func(o_ any, args ...any) any {\n" + body + "}"
}

// genBoundMethod is recv.method(:name): a Method[F] holding a closure over recv.
func (f *fctx) genBoundMethod(n parser.Node, recv expr, name string, public bool) expr {
	mc := f.c.classes["Method"]
	if isOpt(recv.typ) && !f.lenient() {
		f.errorf(n, "possibly-nil %s has no method %s; check it first", recv.typ, name)
	}
	self := recv.code == f.selfCode
	if !self && (isAny(recv.typ) || isNil(recv.typ) || isAbstract(recv.typ) || (isUnion(recv.typ) || isOpt(recv.typ)) && f.lenient()) {
		// a gem's union (`pattern.method(:convert)` after `respond_to?(:convert)`) is untyped here too (decision 244)
		return f.genDynMethod(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, name, public)
	}
	e := f.resolve(recv.typ, name)
	if e == nil {
		f.errorf(n, "undefined method '%s' for %s", name, recv.typ)
	}
	if public && kernelModuleFunction(e.M, recv.typ) { // Kernel.public_method(:puts): public on Kernel itself (decision 139)
		public = false
	}
	env := f.callEnv(n, e, recv)
	fe := e // F's source: an overload by count (`first`, `__first_0`) answers the fewest arguments
	if o := minOverload(e); o != nil {
		fe = o
	}
	f.methodTarget(n, fe, name, public)
	ft := f.methodFnType(n, fe, f.callEnv(n, fe, recv), nil)
	r := f.methodRecv(recv)
	fn := f.funcClosure(n, ft.Params, ft.Ret, func(args []parser.Node) expr {
		if needsBlock(fe.M) {
			return f.noBlockCall(args)
		}
		saved := f.implicitCall
		f.implicitCall = !public
		defer func() { f.implicitCall = saved }()
		return f.genMethodCall(n, r, name, args, nil)
	})
	dyn := noBlockDyn
	if !needsBlock(e.M) {
		dyn = f.dynClosure(e, recv.typ, env, !public, false)
	}
	t := TClass{C: mc, Args: []Type{ft}}
	code := fmt.Sprintf("&%s{fn: Ref(%s), dyn: %s, recv: %s, info: %s}",
		strings.TrimPrefix(f.c.goType(t), "*"), fn, dyn, f.coerce(n, r, TAny{}), f.boundInfo(n, e, r, name))
	return expr{code: code, typ: t}
}

// genDynMethod is method(:name) on an untyped receiver: called through the dispatcher, its signature unknown.
func (f *fctx) genDynMethod(n parser.Node, recv expr, name string, public bool) expr {
	if f.m == nil || !f.m.quietDynamic {
		f.warn(n, "dynamic call: method(:%s) on %s", name, recv.typ)
	}
	f.c.noteDyn(name)
	how := "rbFCall"
	if public {
		how = "rbCall"
	}
	r := f.methodRecv(recv)
	dyn := fmt.Sprintf("func(o_ any, args ...any) any { return rbDyn%s(%s, o_, args...) }", goMethodName(name), how)
	t := TClass{C: f.c.classes["Method"], Args: []Type{TAny{}}}
	if fn := f.c.noteAliasKey(name); fn != "" { // `p.method(:succ) == p.method(:next)` on an untyped p (prime's alias, decision 418)
		code := fmt.Sprintf("func(r_ any) *Method[any] { return &Method[any]{dyn: %s, recv: r_, info: &rbMethodInfo{name: %q, key: %s(r_)}} }(%s)", dyn, name, fn, f.coerce(n, r, TAny{}))
		return expr{code: code, typ: t}
	}
	code := fmt.Sprintf("&Method[any]{dyn: %s, recv: %s, info: &rbMethodInfo{name: %q, key: %q}}", dyn, f.coerce(n, r, TAny{}), name, "?#"+name)
	return expr{code: code, typ: t}
}

// aliasKeyClasses are the struct classes where name shares its def with another name (`alias next succ`): there a
// dynamic method(:name)'s key is the def's, as methodInfo's is, so the aliases compare equal.
func (c *Compiler) aliasKeyClasses(name string) []*Class {
	var out []*Class
	for _, cls := range c.classList {
		if !cls.isStruct() || cls.IsModule || cls.metaOf != nil || len(cls.TypeParams) > 0 {
			continue
		}
		e := cls.lookup(name)
		if e == nil || e.M.Node == nil || e.M.File == nil || e.M.File.isPrelude() || e.M.Kind != kindDef {
			continue
		}
		for _, o := range cls.methodSet() {
			if o.M != e.M && o.M.Node == e.M.Node {
				out = append(out, cls)
				break
			}
		}
	}
	return out
}

// noteAliasKey names rbMethodKey_X for name, or "" when no class aliases it.
func (c *Compiler) noteAliasKey(name string) string {
	if len(c.aliasKeyClasses(name)) == 0 {
		return ""
	}
	if c.aliasKeys == nil {
		c.aliasKeys = map[string]bool{}
	}
	c.aliasKeys[name] = true
	return "rbMethodKey_" + goMethodName(name)
}

// emitAliasKeys emits each noted rbMethodKey_X: a type switch over aliasKeyClasses, "?#name" for any other receiver.
func (c *Compiler) emitAliasKeys() {
	for _, name := range slices.Sorted(maps.Keys(c.aliasKeys)) {
		c.w("func rbMethodKey_%s(o any) string {\n\tswitch o.(type) {\n", goMethodName(name))
		for _, cls := range c.aliasKeyClasses(name) {
			m := cls.lookup(name).M
			owner := m.Owner.RubyName
			c.w("\tcase %s:\n\t\treturn %q\n", cls.concreteType(), owner+"#@"+m.File.Name+":"+strconv.Itoa(m.Node.Location.StartOffset))
		}
		c.w("\tdefault: // gocritic rejects the one-case switch pruning can leave\n\t}\n\treturn %q\n}\n\n", "?#"+name)
	}
}

// genUnboundMethod is Klass.instance_method(:name): UnboundMethod[^(Klass, ...) -> R], a closure taking the receiver first.
func (f *fctx) genUnboundMethod(n parser.Node, recv expr, name string, public bool) expr {
	meta := f.metaOfType(recv.typ)
	if meta == nil {
		f.errorf(n, "instance_method needs a class or module (decision 141)")
	}
	// `self.class.instance_method(:m)` (Homebrew's Formula#initialize): the class is known only at run time, so the
	// method dispatches through the receiver it is bound to and owner comes from the class object (decision 281)
	runTime := !recv.classObj && recv.code != f.selfCode
	cls := meta.metaOf
	if len(cls.TypeParams) > 0 {
		f.errorf(n, "%s.instance_method: %s is generic, and an UnboundMethod cannot carry its type arguments; use obj.method(:%s) (decision 141)", cls.RubyName, cls.RubyName, name)
	}
	self := cls.instance()
	e := f.resolve(self, name)
	if e == nil {
		f.errorf(n, "undefined method '%s' for class '%s'", name, cls.RubyName)
	}
	f.methodTarget(n, e, name, public)
	var recvTmp string
	if runTime {
		recvTmp = f.newTmp()
		f.emit("%s := %s", recvTmp, recv.code)
		f.emit("_ = %s", recvTmp) // no subclass may override name
	} else if !recv.classObj {
		f.discard(recv)
	}
	env := f.callEnv(n, e, expr{typ: self})
	ft := f.methodFnType(n, e, env, self)
	m := e.M
	static := !runTime && !m.File.isPrelude() && m.Kind == kindDef && m.Owner != nil && m.Owner.isStruct()
	fn := f.funcClosure(n, ft.Params, ft.Ret, func(args []parser.Node) expr {
		r := args[0].(*exprNode).e
		if needsBlock(m) {
			return f.noBlockCall(args)
		}
		if static {
			// MRI binds this definition, not a subclass's override
			codes, restIdx := f.genArgs(n, m, env, args[1:], nil)
			if m.Block != nil && !m.Iterator { // an optional block's Go param, before a rest's (matrix's `collect(which = :all, &block)`, decision 414)
				if restIdx < 0 {
					restIdx = len(codes)
				}
				codes = slices.Insert(codes, restIdx, "nil")
			}
			return expr{code: staticCallCode(m, "", r.code, strings.Join(codes, ", ")), typ: ft.Ret}
		}
		saved := f.implicitCall
		f.implicitCall = !public
		defer func() { f.implicitCall = saved }()
		return f.genMethodCall(n, r, name, args[1:], nil)
	})
	dyn := noBlockDyn
	if !needsBlock(m) {
		dyn = f.dynClosure(e, self, env, !public, static)
	}
	t := TClass{C: f.c.classes["UnboundMethod"], Args: []Type{ft}}
	info := f.methodInfo(n, e)
	if runTime {
		info = f.switchInfo(n, e, recvTmp, cls, name, func(sub *Class) string {
			if sub.meta == nil {
				return ""
			}
			return "*" + sub.meta.Name
		})
	}
	code := fmt.Sprintf("&%s{fn: Ref(%s), dyn: %s, info: %s}", strings.TrimPrefix(f.c.goType(t), "*"), fn, dyn, info)
	return expr{code: code, typ: t}
}

// boundInfo is methodInfo for the definition r's class answers: a subclass overriding name is found by a type switch over the closed world's subclasses.
func (f *fctx) boundInfo(n parser.Node, e *entry, r expr, name string) string {
	var cls *Class
	switch t := r.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no subclasses to switch over
	}
	return f.switchInfo(n, e, r.code, cls, name, func(sub *Class) string { return sub.concreteType() })
}

// switchInfo is methodInfo for the definition the run-time class answers: a type switch on code over cls's
// subclasses that override name, each case typed by goName (the instance's type, or its class object's).
func (f *fctx) switchInfo(n parser.Node, e *entry, code string, cls *Class, name string, goName func(*Class) string) string {
	base := f.methodInfo(n, e)
	if cls == nil || !cls.isStruct() || cls.universal || cls.metaOf != nil || len(cls.TypeParams) > 0 {
		return base
	}
	var cases, first []string
	var walk func(k *Class)
	walk = func(k *Class) {
		for _, sub := range k.Subclasses {
			if sub.metaOf != nil || len(sub.TypeParams) > 0 {
				continue
			}
			if se := sub.lookup(name); se != nil && se.M != e.M && goName(sub) != "" {
				info := f.methodInfo(n, se)
				cases = append(cases, fmt.Sprintf("\tcase %s:\n\t\treturn %s\n", goName(sub), info))
				if first == nil {
					first = []string{goName(sub), info}
				}
			}
			walk(sub)
		}
	}
	walk(cls)
	switch len(cases) {
	case 0:
		return base
	case 1: // a one-case type switch fails gocritic's singleCaseSwitch
		return "func() *rbMethodInfo {\n\tif _, ok := any(" + code + ").(" + first[0] + "); ok {\n\t\treturn " + first[1] + "\n\t}\n\treturn " + base + "\n}()"
	}
	return "func() *rbMethodInfo {\n\tswitch any(" + code + ").(type) {\n" + strings.Join(cases, "") + "\t}\n\treturn " + base + "\n}()"
}

// overloads are a prelude method's twins by argument count (decision 12): `__first_0` for `first`.
func overloads(m *Method) []entry {
	if m.Owner == nil || strings.HasPrefix(m.Name, "__") {
		return nil
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	var out []entry
	for _, x := range m.Owner.methodSet() {
		n, ok := strings.CutPrefix(x.M.Name, prefix)
		if ok && n != "" && strings.Trim(n, "0123456789") == "" {
			out = append(out, x)
		}
	}
	return out
}

// minOverload is the twin of e taking fewer required arguments than e, if any.
func minOverload(e *entry) *entry {
	var best *entry
	least := requiredArgs(e.M)
	for _, o := range overloads(e.M) {
		if r := requiredArgs(o.M); r < least {
			best, least = &o, r
		}
	}
	return best
}

// methodInfo is the *rbMethodInfo literal for e: what MRI's Method reflection reads.
func (f *fctx) methodInfo(n parser.Node, e *entry) string {
	m := e.M
	owner := m.Owner
	if owner == nil {
		owner = f.c.classes["Object"]
	}
	ownerCode, key, single := classVar(owner), owner.RubyName+"#", false
	if owner.metaOf != nil {
		ownerCode, key, single = "&rbSingletonClass{of: "+classVar(owner.metaOf)+"}", owner.metaOf.RubyName+".", true
	} else if owner.meta == nil {
		f.errorf(n, "%s has no class object to be %s's owner", owner.RubyName, m.Name)
	}
	if m.Node != nil && m.File != nil && (m.Kind == kindDef || m.Kind == kindPrimitive) { // an alias shares its def, and MRI's == compares the def (Matrix.method(:I) == method(:identity), decision 414)
		key += "@" + m.File.Name + ":" + strconv.Itoa(m.Node.Location.StartOffset)
	} else {
		key += m.Name
	}
	params, arity := methodParams(m, len(overloads(m)) > 0)
	ps := make([]string, len(params))
	for i, p := range params {
		ps[i] = fmt.Sprintf("{%q, %q}", p[0], p[1])
	}
	loc := ""
	if m.File != nil && !m.File.isPrelude() && m.Kind != kindSynth {
		loc = " " + m.File.Name + ":" + strconv.Itoa(m.Line)
	}
	return fmt.Sprintf("&rbMethodInfo{name: %q, key: %q, owner: %s, params: []rbParamInfo{%s}, arity: %d, loc: %q, single: %t, known: true}",
		m.Name, key, ownerCode, strings.Join(ps, ", "), arity, loc, single)
}

// cStyleParams: a prelude method stands for a C one, anonymous, unless it takes keywords: then MRI writes it in Ruby
// (clone, unpack1, warn) and names its parameters, which prism's unpack1 polyfill reads for `offset` (decision 427).
func cStyleParams(m *Method) bool {
	return m.Node == nil || m.File == nil || m.File.isPrelude() && (m.Node.Parameters == nil || len(m.Node.Parameters.Keywords) == 0)
}

// methodParams is MRI's #parameters and #arity; prelude methods stand for C methods, so they get C's anonymous `(_, _)` or `(*)`.
func methodParams(m *Method, overloaded bool) (ps [][2]string, arity int) {
	switch {
	case m.Kind == kindAttrReader:
		return nil, 0
	case m.Kind == kindAttrWriter:
		return [][2]string{{"req", ""}}, 1
	case m.Kind == kindSynth && m.Name == "new":
		return [][2]string{{"rest", ""}}, -1
	case cStyleParams(m):
		for _, p := range m.Params {
			if overloaded || p.Rest || p.Keyword || p.Default != nil {
				return [][2]string{{"rest", ""}}, -1
			}
			ps = append(ps, [2]string{"req", ""})
		}
		return ps, len(ps)
	}
	pn := m.Node.Parameters
	if pn == nil {
		return nil, 0
	}
	name := func(s *string, anon string) string {
		if s == nil {
			return anon
		}
		return *s
	}
	req, opt := 0, false
	positional := func(nodes []parser.Node) {
		for _, p := range nodes {
			req++
			if rp, ok := p.(*parser.RequiredParameterNode); ok {
				ps = append(ps, [2]string{"req", rp.Name})
			} else {
				ps = append(ps, [2]string{"req", ""}) // a destructuring (a, b)
			}
		}
	}
	positional(pn.Requireds)
	for _, p := range pn.Optionals {
		opt = true
		if op, ok := p.(*parser.OptionalParameterNode); ok {
			ps = append(ps, [2]string{"opt", op.Name})
		}
	}
	if rp, ok := pn.Rest.(*parser.RestParameterNode); ok {
		opt = true
		ps = append(ps, [2]string{"rest", name(rp.Name, "*")})
	}
	positional(pn.Posts)
	keyreq, key := false, false
	for _, k := range pn.Keywords {
		switch k := k.(type) {
		case *parser.RequiredKeywordParameterNode:
			keyreq = true
			ps = append(ps, [2]string{"keyreq", k.Name})
		case *parser.OptionalKeywordParameterNode:
			key = true
			ps = append(ps, [2]string{"key", k.Name})
		}
	}
	switch k := pn.KeywordRest.(type) {
	case *parser.KeywordRestParameterNode:
		key = true
		ps = append(ps, [2]string{"keyrest", name(k.Name, "**")})
	case *parser.ForwardingParameterNode:
		opt = true
		ps = append(ps, [2]string{"rest", "*"}, [2]string{"keyrest", "**"}, [2]string{"block", "&"})
	case *parser.NoKeywordsParameterNode:
		ps = append(ps, [2]string{"nokey", ""})
	}
	if pn.Block != nil {
		ps = append(ps, [2]string{"block", name(pn.Block.Name, "&")})
	}
	// MRI counts the keywords as one more argument, required when any keyword is
	if keyreq {
		req++
	} else if key {
		opt = true
	}
	if opt {
		return ps, -req - 1
	}
	return ps, req
}

// methodValueCall compiles what a typed Method or UnboundMethod answers from F: call, to_proc, curry, bind, bind_call.
func (f *fctx) methodValueCall(n parser.Node, recv expr, t TClass, name string, args []parser.Node, block parser.Node) (expr, bool) {
	kind, ft, typed := methodFn(t)
	if kind == "" {
		return expr{}, false
	}
	op := kind + "#" + name
	switch op {
	case "Method#call", "Method#[]", "Method#===", "Method#to_proc", "Method#curry", "UnboundMethod#bind", "UnboundMethod#bind_call":
	default:
		return expr{}, false
	}
	if block != nil {
		f.errorf(n, "%s does not take a block", op)
	}
	if !typed {
		if name == "to_proc" && f.lenient() && len(args) == 0 { // a gem's untyped Method as a Proc taking anything, called dynamically (decision 244)
			f.c.noteDyn("call")
			code := "func(xs_ ...any) any { return rbDynCall(rbCall, " + f.coerce(n, recv, TAny{}) + ", xs_...) }"
			return expr{code: code, typ: TFunc{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}, Proc: true}}, true
		}
		if name == "to_proc" || name == "curry" {
			f.errorf(n, "%s needs the method's signature, which a %s lost (decision 141)", op, t)
		}
		if f.m == nil || !f.m.quietDynamic {
			f.warn(n, "dynamic call: %s on %s", op, t)
		}
		return expr{}, false
	}
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			f.errorf(a, "a splat argument to %s is not supported", op)
		}
	}
	switch op {
	case "Method#to_proc":
		return expr{code: fieldOf(recv.code, "fn"), typ: ft}, true
	case "Method#curry":
		return f.methodCurry(n, recv, ft, args), true
	case "UnboundMethod#bind":
		return f.unboundBind(n, recv, ft, args), true
	case "UnboundMethod#bind_call":
		if len(args) == 0 {
			f.errorf(n, "wrong number of arguments (given 0, expected 1+)")
		}
	}
	return f.methodCallTyped(n, recv, kind, ft, args), true
}

// methodCallTyped calls F's closure: with F's arguments, typed; with more (optional or rest parameters), through dyn.
func (f *fctx) methodCallTyped(n parser.Node, recv expr, kind string, ft TFunc, args []parser.Node) expr {
	if len(args) < len(ft.Params) {
		f.errorf(n, "wrong number of arguments (given %d, expected %d)", len(args), len(ft.Params))
	}
	if len(args) == len(ft.Params) {
		codes := make([]string, len(args))
		for i, a := range args {
			codes[i] = f.coerce(a, f.genExpr(a, ft.Params[i]), ft.Params[i])
		}
		return expr{code: "(*" + fieldOf(recv.code, "fn") + ")(" + strings.Join(codes, ", ") + ")", typ: ft.Ret}
	}
	if f.m == nil || !f.m.quietDynamic {
		op := kind + "#call"
		if kind == "UnboundMethod" {
			op = "UnboundMethod#bind_call"
		}
		f.warn(n, "dynamic call: %s with more than its %d required arguments", op, len(ft.Params))
	}
	m := f.methodRecv(recv)
	o := m.code + ".recv"
	if kind == "UnboundMethod" {
		o = f.coerce(args[0], f.genExpr(args[0], ft.Params[0]), ft.Params[0])
		o = f.coerce(n, expr{code: o, typ: ft.Params[0]}, TAny{})
		args = args[1:]
	}
	call := expr{code: fmt.Sprintf("rbMethodCallDyn(%s.dyn, %s, %s.info, []any{%s})", m.code, o, m.code, strings.Join(f.anyArgs(args), ", ")), typ: TAny{}}
	if isVoid(ft.Ret) {
		return expr{code: "_ = " + call.code, typ: TVoid{}, stmt: true}
	}
	return expr{code: f.coerce(n, call, ft.Ret), typ: ft.Ret}
}

// unboundBind is UnboundMethod#bind on a typed one: a Method over F's other parameters, the receiver checked at compile time.
func (f *fctx) unboundBind(n parser.Node, recv expr, ft TFunc, args []parser.Node) expr {
	if len(args) != 1 {
		f.errorf(n, "wrong number of arguments (given %d, expected 1)", len(args))
	}
	u := f.methodRecv(recv)
	a := f.genExpr(args[0], ft.Params[0])
	obj := expr{code: f.coerce(args[0], a, ft.Params[0]), typ: ft.Params[0]}
	obj.lit = a.lit && obj.code == a.code
	obj = f.methodRecv(obj)
	rest := ft.Params[1:]
	ps, as := make([]string, len(rest)), make([]string, 0, 1+len(rest))
	as = append(as, obj.code)
	for i, p := range rest {
		v := fmt.Sprintf("a%d", i)
		ps[i], as = v+" "+f.c.goType(p), append(as, v)
	}
	body := "(*" + u.code + ".fn)(" + strings.Join(as, ", ") + ")"
	retS := ""
	if !isVoid(ft.Ret) {
		body, retS = "return "+body, " "+f.c.goType(ft.Ret)
	}
	mt := TClass{C: f.c.classes["Method"], Args: []Type{TFunc{Params: rest, Ret: ft.Ret, Proc: true}}}
	code := fmt.Sprintf("&%s{fn: Ref(func(%s)%s { %s }), dyn: %s.dyn, recv: %s, info: %s.info}",
		strings.TrimPrefix(f.c.goType(mt), "*"), strings.Join(ps, ", "), retS, body, u.code, f.coerce(n, obj, TAny{}), u.code)
	return expr{code: code, typ: mt}
}

// methodCurry is Method#curry: a Proc per parameter, the last calling F.
func (f *fctx) methodCurry(n parser.Node, recv expr, ft TFunc, args []parser.Node) expr {
	if len(args) > 1 {
		f.errorf(n, "wrong number of arguments (given %d, expected 0..1)", len(args))
	}
	if len(args) == 1 {
		if lit, ok := args[0].(*parser.IntegerNode); !ok || f.f.text(lit.Location) != strconv.Itoa(len(ft.Params)) {
			f.errorf(n, "curry takes the method's own arity, %d, as a literal (decision 141)", len(ft.Params))
		}
	}
	if len(ft.Params) <= 1 {
		return expr{code: fieldOf(recv.code, "fn"), typ: ft}
	}
	m := f.methodRecv(recv)
	names := make([]string, len(ft.Params))
	for i := range names {
		names[i] = fmt.Sprintf("a%d", i)
	}
	types := make([]TFunc, len(ft.Params)) // types[i] takes parameter i
	ret := ft.Ret
	for i := len(ft.Params) - 1; i >= 0; i-- {
		types[i] = TFunc{Params: []Type{ft.Params[i]}, Ret: ret, Proc: true}
		ret = types[i]
	}
	code := "(*" + m.code + ".fn)(" + strings.Join(names, ", ") + ")"
	if !isVoid(ft.Ret) {
		code = "return " + code
	}
	for i := len(ft.Params) - 1; i >= 0; i-- {
		retS := ""
		if !isVoid(types[i].Ret) {
			retS = " " + f.c.goType(types[i].Ret)
		}
		code = fmt.Sprintf("Ref(func(%s %s)%s { %s })", names[i], f.c.goType(ft.Params[i]), retS, code)
		if i > 0 {
			code = "return " + code
		}
	}
	return expr{code: code, typ: types[0]}
}

// methodRefBlock desugars `&method(:name)` to a block of the yielded arity, so the target's optional parameters take what is yielded.
func (f *fctx) methodRefBlock(ba *parser.BlockArgumentNode, nparams int) *parser.BlockNode {
	call, ok := ba.Expression.(*parser.CallNode)
	if !ok || call.Name != "method" || call.Block != nil || len(callArgs(call)) != 1 || literalName(callArgs(call)[0]) == "" || nparams < 0 {
		return nil
	}
	recvT := f.selfType
	if call.Receiver != nil {
		f.probe(func() { recvT = f.genExpr(call.Receiver, nil).typ })
	}
	if f.resolve(recvT, "method") != nil {
		return nil // the receiver's own `method`; checked before the receiver's temp is emitted
	}
	var recv parser.Node
	if call.Receiver != nil {
		switch call.Receiver.(type) {
		case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.SelfNode, *parser.ConstantReadNode, *parser.ConstantPathNode,
			*parser.IntegerNode, *parser.FloatNode, *parser.SymbolNode:
			recv = call.Receiver
		default: // evaluated once, as MRI takes the Method before the call
			e := f.genExpr(call.Receiver, nil)
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, e.code)
			recv = &exprNode{Node: call.Receiver, e: expr{code: tmp, typ: e.typ, classObj: e.classObj}}
		}
	}
	loc := ba.Location
	var locals []string
	args := []parser.Node{&parser.SymbolNode{Location: loc, Unescaped: parser.RubyString{Value: literalName(callArgs(call)[0])}}}
	var params []parser.Node
	for i := range nparams {
		name := "x_" + strconv.Itoa(i)
		locals = append(locals, name)
		params = append(params, &parser.RequiredParameterNode{Location: loc, Name: name})
		args = append(args, &parser.LocalVariableReadNode{Location: loc, Name: name})
	}
	body := &parser.CallNode{Location: loc, Receiver: recv, Name: "__send__", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: args}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     locals,
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: params}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{body}},
	}
}

// blockArity is how many values the method n calls yields to its block, or -1 when n's target is unknown.
func (f *fctx) blockArity(n *parser.CallNode) int {
	var recvT Type
	switch {
	case n.Receiver == nil:
		recvT = f.selfType
	case f.classRef(n.Receiver) != nil:
		cls := f.classRef(n.Receiver)
		if cls.meta == nil {
			return -1
		}
		recvT = TClass{C: cls.meta}
	default:
		f.probe(func() { recvT = f.genExpr(n.Receiver, nil).typ })
	}
	if e := f.resolve(stripOpt(recvT), n.Name); e != nil && e.M.Block != nil {
		return len(e.M.Block.Params)
	}
	return -1
}
