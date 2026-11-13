package compiler

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genCallRewrite is a call compiled as another expression: Regexp.last_match on the caller's $~ (decision 232), a
// frozen proc literal at its expected type (decision 226).
func (f *fctx) genCallRewrite(n *parser.CallNode, expected Type) (expr, bool) {
	if isRegexpConst(n.Receiver) && n.Name == "last_match" && n.Block == nil && len(callArgs(n)) <= 1 {
		// the caller's $~ (decision 171: it is the frame's), handed to the helper that indexes it
		gv := &parser.GlobalVariableReadNode{Location: n.Location, Name: "$~"}
		if len(callArgs(n)) == 0 {
			return f.genExpr(gv, expected), true
		}
		at := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "__last_match_at",
			Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{gv, callArgs(n)[0]}}}
		return f.genCall(at, expected), true
	}
	if r, ok := n.Receiver.(*parser.CallNode); ok && r.Name == "singleton_class" && r.Arguments == nil && n.Block == nil &&
		(n.Name == "<" || n.Name == "<=" || n.Name == "include?") && len(callArgs(n)) == 1 {
		// `x.singleton_class < M`: M is among x's singleton ancestors, which is x.is_a?(M) for a module M (decision 241)
		if cls := f.classRef(callArgs(n)[0]); cls != nil && cls.IsModule {
			self := r.Receiver
			if self == nil {
				self = &parser.SelfNode{Location: r.Location}
			}
			isA := &parser.CallNode{Location: n.Location, Receiver: self, Name: "is_a?", Arguments: n.Arguments}
			if n.Name == "include?" {
				return f.genCall(isA, expected), true
			}
			return f.genExpr(&parser.OrNode{Location: n.Location, Left: isA, Right: &parser.NilNode{Location: n.Location}}, expected), true // nil when unrelated
		}
	}
	if e, ok := f.genFoldOrLower(n, expected); ok {
		return e, true
	}
	if e, ok := f.symbolToProc(n, expected); ok {
		return e, true
	}
	if f.lenient() && f.undefsSingleton(n) {
		// `singleton_class.send(:undef_method, :inherited)` ahead of a define_singleton_method (bindata's register_subclasses):
		// the closed world has the one definition boot recorded, so removing the earlier one is a no-op (decision 301)
		return expr{code: "", typ: TVoid{}, stmt: true, done: true}, true
	}
	if e, ok := f.runtimeExtend(n); ok {
		return e, true
	}
	f.noteRuntimeRegexp(n)
	if e, ok := f.callSugar(n, expected); ok {
		return e, true
	}
	if procLiteral(n.Receiver) && (n.Name == "freeze" || n.Name == "itself") && n.Arguments == nil && n.Block == nil {
		if _, fn := expected.(TFunc); fn { // `T.let(-> {}.freeze, T.proc.void)`: the lambda at the expected type; freezing a Proc changes nothing it does
			return f.genExpr(n.Receiver, expected), true
		}
	}
	return expr{}, false
}

// runtimeExtend is `extend M` run by a class method (`safe_initialization!`): the closed world extends classes when
// they load (selfExtendCall), so at run time it is a no-op for a class that has M, and NotImplementedError for one that
// does not (decision 241).
func (f *fctx) runtimeExtend(n *parser.CallNode) (expr, bool) {
	if n.Name != "extend" || n.Receiver != nil || n.Block != nil || f.m == nil || f.owner == nil || f.owner.metaOf == nil || len(callArgs(n)) == 0 {
		return expr{}, false
	}
	self := expr{code: f.selfCode, typ: f.selfType}
	var checks []string
	for _, a := range callArgs(n) {
		if f.classRef(a) == nil {
			return expr{}, false
		}
		checks = append(checks, f.isACheck(n, self, a))
	}
	msg := fmt.Sprintf("rugaroo: extend at run time is not supported (%s:%d)", f.f.Name, f.f.line(n.Location.StartOffset))
	f.emit("if !(%s) {", strings.Join(checks, " && "))
	f.emit("\tpanic(NewNotImplementedError(Ref(String(%q))))", msg)
	f.emit("}")
	return expr{code: f.selfCode, typ: f.selfType}, true
}

// noteRuntimeRegexp: Regexp.new/compile/union of anything but string literals RE2 compiles builds a pattern only run
// time knows, so the program gets the regexp2 engine (decision 242).
func (f *fctx) noteRuntimeRegexp(n *parser.CallNode) {
	if !isRegexpConst(n.Receiver) || n.Name != "new" && n.Name != "compile" && n.Name != "union" || f.f.isPrelude() {
		return
	}
	for _, a := range callArgs(n) {
		s, ok := a.(*parser.StringNode)
		if !ok {
			f.emit("_ = rxAltReady") // this body may build what RE2 rejects: naming the engine keeps it
			return
		}
		p, err := rxPrepare(s.Unescaped.Value, false) // only decides whether the alternate engine is needed
		if err == nil && n.Name != "union" {
			_, err = regexp.Compile(p)
		}
		if err != nil {
			f.emit("_ = rxAltReady")
			return
		}
	}
}

// callSugar rewrites a call into the form rugaroo compiles (decision 241): `ENV.fetch(k, d)` is `ENV[k] || d` (an ENV
// value is a String, so this is fetch's answer whatever d is), and renamedCall's.
func (f *fctx) callSugar(n *parser.CallNode, expected Type) (expr, bool) {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && r.Name == "ENV" && n.Name == "fetch" && len(callArgs(n)) == 2 && n.Block == nil {
		key, def := callArgs(n)[0], callArgs(n)[1]
		if !sideEffectFree(def) { // MRI evaluates the default (after the key) even when the key exists
			ke := f.genExpr(key, nil)
			if pinnable(ke.code) && !ke.lit && !isVoid(ke.typ) {
				t := f.newTmp()
				f.emit("%s := %s", t, ke.code)
				ke.code, ke.view = t, ""
			}
			de := f.genExpr(def, expected)
			if !de.lit && !isVoid(de.typ) && de.code != "" && de.code != "nil" {
				t := f.newTmp()
				f.emit("%s := %s", t, de.code)
				de.code, de.view = t, ""
			}
			key, def = &exprNode{Node: key, e: ke}, &exprNode{Node: def, e: de}
		}
		idx := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "[]", Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{key}}}
		return f.genExpr(&parser.OrNode{Location: n.Location, Left: idx, Right: def}, expected), true
	}
	if c := f.injectSymbol(n); c != nil {
		return f.genCall(c, expected), true
	}
	if to := renamedCall(n); to != "" {
		c := *n
		c.Name = to
		return f.genCall(&c, expected), true
	}
	return expr{}, false
}

// injectSymbol is Enumerable's `xs.inject(:+)` and `xs.inject(init, :*)` as `xs.inject(&:+)`: the literal Symbol names
// the method at compile time, so the fold is the block form's (#145). A Symbol only run time knows is a compile error.
func (f *fctx) injectSymbol(n *parser.CallNode) *parser.CallNode {
	args := callArgs(n)
	_, blockLit := n.Block.(*parser.BlockNode)
	if n.Name != "inject" && n.Name != "reduce" && (n.Name != "sum" || len(args) != 1) || n.Block != nil && (n.Name != "sum" || !blockLit) || len(args) == 0 || len(args) > 2 {
		return nil
	}
	recvT := f.selfType
	if n.Receiver != nil {
		var recv expr
		f.probe(func() { recv = f.genExpr(n.Receiver, nil) })
		recvT = recv.typ
	}
	e := f.resolve(recvT, n.Name)
	if e == nil {
		return nil
	}
	if n.Name == "sum" && sumOwners[e.M.Owner.RubyName] { // Range's and Array's own sum add numbers only, as Enumerable's
		return f.sumPlus(n, args)
	}
	if e.M.Owner.RubyName != "Enumerable" {
		return nil
	}
	last := args[len(args)-1]
	sym, ok := last.(*parser.SymbolNode)
	if !ok {
		var a expr
		f.probe(func() { a = f.genExpr(last, nil) })
		if c, isSym := a.typ.(TClass); isSym && c.C.RubyName == "Symbol" {
			f.errorf(n, "%s(sym) needs a literal Symbol (`%s(:+)`) or a block", n.Name, n.Name)
		}
		return nil
	}
	c := *n
	c.Block = &parser.BlockArgumentNode{Location: sym.Location, Expression: sym}
	c.Arguments = nil
	if len(args) == 2 {
		c.Arguments = &parser.ArgumentsNode{Location: n.Arguments.Location, Arguments: args[:1]}
	}
	return &c
}

// sumOwners are the classes whose sum sumPlus may rewrite.
var sumOwners = map[string]bool{"Enumerable": true, "Range": true, "Array": true, "Set": true, "Hash": true}

// sumPlus is `xs.sum("")` and `xs.sum([])`: MRI adds with `+` past the numeric tower, so a String or Array init is
// `xs.inject(init, &:+)`, and with a block `xs.map { ... }.inject(init, &:+)` (#145).
func (f *fctx) sumPlus(n *parser.CallNode, args []parser.Node) *parser.CallNode {
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	if c, ok := a.typ.(TClass); !ok || c.C.RubyName != "String" && c.C.RubyName != "Array" {
		return nil
	}
	c := *n
	if n.Block != nil {
		c.Receiver = &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "map", Block: n.Block}
	}
	c.Name = "inject"
	c.Block = &parser.BlockArgumentNode{Location: n.Location, Expression: &parser.SymbolNode{Location: n.Location, Unescaped: parser.RubyString{Value: "+"}}}
	return &c
}

// renamedCall is the prelude name a call goes to (decision 241): Thread.fork is MRI's alias of Thread.start (defined as
// itself it would override Kernel#fork), and Kernel#exit! is __exit_bang, a name no dispatcher lists.
func renamedCall(n *parser.CallNode) string {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && r.Name == "Thread" && n.Name == "fork" {
		return "start"
	}
	if n.Name == "exit!" && n.Receiver == nil && n.Block == nil {
		return "__exit_bang"
	}
	return ""
}

// genCallForm is a call some form of genCall's handles before ordinary dispatch: a Sorbet cast, a boot-compiled
// define_method or eval (a no-op), to_enum, a Kernel intrinsic, a String bang, a block call, a bare name.
func (f *fctx) genCallForm(n *parser.CallNode, expected Type) (expr, bool) {
	if t, ok := f.f.sorbetCasts[n]; ok {
		return f.genSorbetCast(n, t), true
	}
	if units := f.evalUnitsAt(n); len(units) > 0 && f.m != nil {
		return f.genEvalSwitch(n, units), true
	}
	if rec := f.c.smSites[n]; rec != nil { // a run-time singleton method or extend: recorded on the object (decision 324)
		return f.genExpr(rec, expected), true
	}
	if n.Receiver != nil && f.c.dmSites[n] || f.isEvalSite(n) || f.c.blockEvalSites[n] { // boot made their methods defs (decisions 198, 204)
		return expr{code: "nil", typ: TNil{}, lit: true}, true
	}
	for _, form := range []func() (expr, bool){
		func() (expr, bool) { return f.genToEnum(n) },
		func() (expr, bool) { return f.genEachEdit(n, expected) },
		func() (expr, bool) { return f.genScanMatches(n, expected) },
		func() (expr, bool) { return f.genRecvToEnum(n, expected) },
		func() (expr, bool) { return f.genKernelCall(n, expected) },
		func() (expr, bool) { return f.genMatchSet(n) },
		func() (expr, bool) { return f.genStringBang(n) },
		func() (expr, bool) { return f.genBlockCall(n) },
		func() (expr, bool) { return f.genBareName(n, expected) },
		func() (expr, bool) { return f.genIndexSplat(n) },
		func() (expr, bool) { return f.genProcInfo(n) },
	} {
		if e, ok := form(); ok {
			return e, true
		}
	}
	return expr{}, false
}

// genIndexSplat is `a[*xs]` and `a[*xs] = v` (decision 363): `[]`'s arity picks its form, so it is a switch on
// xs's length over the one- and two-index forms, each the static call; a length no form compiles for raises ArgumentError.
func (f *fctx) genIndexSplat(n *parser.CallNode) (expr, bool) {
	args := callArgs(n)
	want := map[string]int{"[]": 1, "[]=": 2}[n.Name]
	if want == 0 || n.Receiver == nil || n.Block != nil || f.lenient() || len(args) != want || !isSplat(args[0]) {
		return expr{}, false
	}
	xs := f.genExpr(args[0].(*parser.SplatNode).Expression, nil) //nolint:forcetypeassert // isSplat
	at, ok := xs.typ.(TClass)
	if !ok || at.C.RubyName != "Array" || len(at.Args) != 1 {
		return expr{}, false
	}
	pin := func(e expr) *exprNode {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, e.code)
		return &exprNode{Node: n, e: expr{code: tmp, typ: e.typ}}
	}
	xn, recv := pin(xs), pin(f.valueOf(f.genExpr(n.Receiver, nil)))
	tail := args[1:]
	if len(tail) == 1 && !pureNode(tail[0]) { // a literal's Go type follows the parameter: left to each arm
		tail = []parser.Node{pin(f.genExpr(tail[0], nil))}
	}
	arm := func(k int) parser.Node {
		idx := make([]parser.Node, 0, k+len(tail))
		for i := range k {
			idx = append(idx, &exprNode{Node: n, e: expr{code: fmt.Sprintf("rbSplatElem(%s, %d)", xn.e.code, i), typ: at.Args[0]}})
		}
		return &parser.CallNode{Location: n.Location, Receiver: recv, Name: n.Name, Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: append(idx, tail...)}}
	}
	var types []Type
	var arities []int
	for k := 1; k <= 2; k++ {
		var e expr
		if f.attempt(func() { f.probe(func() { e = f.genExpr(arm(k), nil) }) }) == nil {
			types, arities = append(types, e.typ), append(arities, k)
		}
	}
	if len(types) == 0 {
		return expr{}, false
	}
	t := types[0]
	for _, u := range types[1:] {
		if j, ok := join(t, u); ok {
			t = j
		} else {
			t = TAny{}
		}
	}
	out := f.newTmp()
	f.emit("var %s %s", out, f.c.goType(t))
	f.emit("switch len(%s.s) {", xn.e.code)
	for _, k := range arities {
		f.emit("case %d:", k)
		f.indent++
		f.emit("%s = %s", out, f.coerce(n, f.genExpr(arm(k), nil), t))
		f.indent--
	}
	f.emit("default:")
	f.emit("\tpanic(NewArgumentError(Ref(String(fmt.Sprintf(\"wrong number of arguments (given %%d, expected %d..%d)\", len(%s.s)%s)))))", arities[0]+len(tail), arities[len(arities)-1]+len(tail), xn.e.code, map[bool]string{true: "+1", false: ""}[len(tail) == 1])
	f.emit("}")
	return expr{code: out, typ: t}, true
}

// undefsSingleton is `singleton_class.undef_method(:x)`, `.remove_method(:x)` or `.send(:undef_method, :x)` on the current class's singleton.
func (f *fctx) undefsSingleton(n *parser.CallNode) bool {
	r, ok := n.Receiver.(*parser.CallNode)
	if !ok || r.Name != "singleton_class" || r.Arguments != nil || r.Receiver != nil && !isSelf(r.Receiver) || n.Block != nil {
		return false
	}
	switch n.Name {
	case "undef_method", "remove_method":
		return true
	case "send", "public_send", "__send__":
		if args := callArgs(n); len(args) > 0 {
			if sym, ok := args[0].(*parser.SymbolNode); ok {
				return sym.Unescaped.Value == "undef_method" || sym.Unescaped.Value == "remove_method"
			}
		}
	}
	return false
}

// untypedRest is a gem's untyped Proc or block signature, `^(*untyped) -> untyped` (decision 244).
func untypedRest(params []Type, rest bool) bool { return rest && len(params) == 1 && isAny(params[0]) }

// symbolProcAny is `&:name` as an untyped Proc: its first argument receives name, the rest are passed on, dispatched at
// run time, as MRI's Symbol#to_proc.
func (f *fctx) symbolProcAny(name string) string {
	f.c.noteDyn(name)
	return "func(xs_ ...any) any { if len(xs_) == 0 { panic(NewArgumentError(Ref(String(\"no receiver given\")))) }; " +
		"return " + rbDynFn(name) + "(rbCall, xs_[0], xs_[1:]...) }"
}

// symbolToProc compiles `:<=.to_proc` where a Proc is expected (`@order = block || :<=.to_proc`): the lambda `&:<=` would
// pass, typed by it.
func (f *fctx) symbolToProc(n *parser.CallNode, expected Type) (expr, bool) {
	sym, ok := n.Receiver.(*parser.SymbolNode)
	if !ok || n.Name != "to_proc" || n.Arguments != nil || n.Block != nil {
		return expr{}, false
	}
	ft, ok := stripOpt(expected).(TFunc)
	switch {
	case ok && ft.Proc && untypedRest(ft.Params, ft.Rest):
		// a gem's untyped Proc (decision 244): the first argument receives the name, dispatched at run time
		return expr{code: "Ref(" + f.symbolProcAny(sym.Unescaped.Value) + ")", typ: ft}, true
	case ok && ft.Proc:
		return f.genLambda(n, &parser.BlockArgumentNode{Location: n.Location, Expression: sym}, nil, ft), true
	}
	return expr{}, false
}

// isRegexpConst is `Regexp` or `::Regexp` (Homebrew's `::Regexp.last_match(1)`).
func isRegexpConst(n parser.Node) bool { return isTopConst(n, "Regexp") }

// sideEffectFree is true for a literal node: evaluating it or not is unobservable.
func sideEffectFree(n parser.Node) bool {
	switch n.(type) {
	case *parser.StringNode, *parser.SymbolNode, *parser.IntegerNode, *parser.FloatNode, *parser.NilNode, *parser.TrueNode, *parser.FalseNode:
		return true
	}
	return false
}

// compactNil is `xs.compact` of an Array[C?] whose C holds nil itself: its elements are already C's Go type, where the
// prelude's `@self Array[U?]` form wants a box per element (decision 378).
func (f *fctx) compactNil(n *parser.CallNode) (expr, bool) {
	if n.Name != "compact" || n.Receiver == nil || n.Block != nil || n.Arguments != nil {
		return expr{}, false
	}
	t, ok := f.probeType(n.Receiver, nil).(TClass) // not an Array? receiver: that call is genSafeNav's or an error
	if ok && t.C.RubyName == "Enumerator::Lazy" && len(t.Args) == 1 && (valOpt(t.Args[0]) || leafOpt(t.Args[0])) {
		recv := f.genExpr(n.Receiver, nil)
		fn := "rbLazyCompactNil("
		if valOpt(t.Args[0]) {
			fn = "rbLazyCompactVal("
		}
		return expr{code: fn + recv.code + ")", typ: TClass{C: t.C, Args: []Type{stripOpt(t.Args[0])}}}, true
	}
	if !ok || t.C.RubyName != "Array" || len(t.Args) != 1 || !leafOpt(t.Args[0]) && !valOpt(t.Args[0]) {
		return expr{}, false
	}
	recv := f.genExpr(n.Receiver, nil)
	elem := t.Args[0].(TOpt).Elem //nolint:forcetypeassert // leafOpt and valOpt hold only for a TOpt
	if valOpt(t.Args[0]) {
		return expr{code: "rbCompactVal(" + recv.code + ")", typ: TClass{C: t.C, Args: []Type{elem}}}, true
	}
	return expr{code: "rbCompactNil(" + recv.code + ")", typ: TClass{C: t.C, Args: []Type{elem}}}, true
}

// genStringLowering is decisions 377-378's compile-time forms: a literal format string, a String + chain, and compact on
// an Array whose optional elements hold nil themselves.
func (f *fctx) genStringLowering(n *parser.CallNode) (expr, bool) {
	if e, ok := f.genLiteralFormat(n); ok {
		return e, true
	}
	if e, ok := f.compactNil(n); ok {
		return e, true
	}
	return f.genStrChain(n)
}

// genFoldOrLower is a Struct writer's __frozen_flag? as a constant, else a String lowering.
func (f *fctx) genFoldOrLower(n *parser.CallNode, expected Type) (expr, bool) {
	if c := f.withIndexChain(n); c != nil {
		return f.genCall(c, expected), true
	}
	if c := slicePattern(n); c != nil {
		return f.genCall(c, expected), true
	}
	if f.frozenFlagConst(n) {
		return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
	}
	return f.genStringLowering(n)
}

// frozenFlagConst: a Go constant when nothing can freeze (rbObjFrozenFlag is nil), so Go's inliner skips the writer's raise (decision 400).
func (f *fctx) frozenFlagConst(n *parser.CallNode) bool {
	return n.Name == "__frozen_flag?" && n.Receiver == nil && n.Arguments == nil && n.Block == nil && f.m != nil &&
		!f.c.objFreeze() && slices.Contains(f.m.Owner.allAncestors(), f.c.classes["Struct"])
}

// genScanMatches is `str.to_enum(:scan, re).map { Regexp.last_match }`, Ruby's idiom for every match's MatchData
// (rubocop's directive comments): String#__scan_matches, with no Enumerator or $~ between (decision 444).
func (f *fctx) genScanMatches(n *parser.CallNode, expected Type) (expr, bool) {
	b, ok := n.Block.(*parser.BlockNode)
	if !ok || n.Name != "map" && n.Name != "collect" || len(callArgs(n)) != 0 || b.Parameters != nil {
		return expr{}, false
	}
	st, ok := b.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return expr{}, false
	}
	last, ok := st.Body[0].(*parser.CallNode)
	if !ok || last.Name != "last_match" || len(callArgs(last)) != 0 || !isTopConst(last.Receiver, "Regexp") {
		return expr{}, false
	}
	en, ok := n.Receiver.(*parser.CallNode)
	if !ok || en.Name != "to_enum" && en.Name != "enum_for" || en.Receiver == nil || en.Block != nil {
		return expr{}, false
	}
	args := callArgs(en)
	if sym, ok := firstArg(args).(*parser.SymbolNode); !ok || len(args) != 2 || sym.Unescaped.Value != "scan" {
		return expr{}, false
	}
	return f.genExpr(&parser.CallNode{Location: n.Location, Receiver: en.Receiver, Name: "__scan_matches", MessageLoc: n.MessageLoc,
		Arguments: &parser.ArgumentsNode{Location: en.Arguments.Location, Arguments: args[1:]}}, expected), true
}

// genEachEdit is a gem's `lines.each { |line| line.chomp!("\n") }` on a local Array of Strings: MRI edits each element
// where it is, and a String is a value here, so the block would edit a copy and say nothing (parser's
// Buffer#source_lines kept every newline). It runs as `lines.each_index { |i| line = lines[i]; ...; lines[i] = line }`
// (decision 444, amends 405, whose `<<` on such a parameter raised). A body that leaves early (next, break, return)
// keeps the old rule.
func (f *fctx) genEachEdit(n *parser.CallNode, expected Type) (expr, bool) {
	if r := f.eachEdit(n); r != nil {
		return f.genExpr(r, expected), true
	}
	return expr{}, false
}

// eachEdit is genEachEdit's rewritten call, or nil.
func (f *fctx) eachEdit(n *parser.CallNode) *parser.CallNode {
	b, ok := n.Block.(*parser.BlockNode)
	recv, isLocal := n.Receiver.(*parser.LocalVariableReadNode)
	if !ok || !isLocal || n.Name != "each" || len(callArgs(n)) != 0 || !f.lenient() || b.Body == nil {
		return nil
	}
	ps, ok := b.Parameters.(*parser.BlockParametersNode)
	if !ok || ps.Parameters == nil || len(ps.Parameters.Requireds) != 1 || ps.Parameters.Rest != nil || len(ps.Parameters.Optionals) > 0 {
		return nil
	}
	p, ok := ps.Parameters.Requireds[0].(*parser.RequiredParameterNode)
	st, isStmts := b.Body.(*parser.StatementsNode)
	if !ok || !isStmts {
		return nil
	}
	if !editsParamInPlace(b.Body, p.Name) {
		return nil
	}
	if !f.stringElements(n) {
		return nil
	}
	loc, i := b.Location, "rb_i_"
	read := func(name string) parser.Node { return &parser.LocalVariableReadNode{Location: loc, Name: name} }
	at := &parser.CallNode{Location: loc, Receiver: read(recv.Name), Name: "[]", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{read(i)}}}
	set := &parser.CallNode{Location: loc, Receiver: read(recv.Name), Name: "[]=", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{read(i), read(p.Name)}}}
	body := append([]parser.Node{&parser.LocalVariableWriteNode{Location: loc, Name: p.Name, Value: at}}, st.Body...)
	body = append(body, set)
	blk := &parser.BlockNode{Location: loc, Locals: append([]string{i, p.Name}, b.Locals...),
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: i}}}},
		Body:       &parser.StatementsNode{Location: st.Location, Body: body}}
	return &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "each_index", MessageLoc: n.MessageLoc, Block: blk}
}

// editsParamInPlace reports a block body that edits its parameter name in place (`line.chomp!`, `s << x`) and never
// leaves early (next, break, return, redo).
func editsParamInPlace(body parser.Node, name string) bool {
	edits, leaves := false, false
	walkNode(body, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.NextNode, *parser.BreakNode, *parser.ReturnNode, *parser.RedoNode:
			leaves = true
		case *parser.CallNode:
			if r, ok := x.Receiver.(*parser.LocalVariableReadNode); ok && r.Name == name && x.Name != "!" && x.Name != "!=" &&
				(strings.HasSuffix(x.Name, "!") || strAppendNames[x.Name] || x.Name == "replace" || x.Name == "clear" || x.Name == "[]=") {
				edits = true
			}
		}
		return false, true
	})
	return edits && !leaves
}

// stringElements reports n's receiver an Array whose elements are Strings, or untyped ones, as far as the compiler knows.
func (f *fctx) stringElements(n *parser.CallNode) bool {
	recv := n.Receiver
	rt := stripOpt(f.probeType(recv, nil))
	if t, ok := rt.(TClass); !f.heldAsAny(rt) && (!ok || t.C.RubyName != "Array" || len(t.Args) != 1 || !isClass(t.Args[0], "String") && !isAny(t.Args[0])) {
		return false
	}
	var pt Type
	f.probe(func() {
		_ = catchCompileError(func() {
			pt = f.genExpr(&parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: "first"}, nil).typ
		})
	})
	return pt != nil && (f.heldAsAny(stripOpt(pt)) || isClass(stripOpt(pt), "String"))
}

// withIndexChains are the blockless Enumerable methods whose `.with_index { |x, i| }` feeds the block's value back.
var withIndexChains = map[string]bool{"sort_by": true, "min_by": true, "max_by": true, "group_by": true, "partition": true,
	"select": true, "filter": true, "reject": true} // these three only where the class has the twin: Hash's (#155)

// withIndexChain is `xs.sort_by.with_index(off) { |x, i| }` (or `.each_with_index`) as `xs.__sort_by_with_index(off) { |x, i| }`:
// the enumerator MRI builds is decided here, at compile time (#145).
func (f *fctx) withIndexChain(n *parser.CallNode) *parser.CallNode {
	inner, ok := n.Receiver.(*parser.CallNode)
	if !ok || n.Name != "with_index" && n.Name != "each_with_index" || !withIndexChains[inner.Name] || inner.Block != nil || inner.Arguments != nil || inner.Receiver == nil {
		return nil
	}
	if _, ok := n.Block.(*parser.BlockNode); !ok {
		return nil
	}
	args := callArgs(n)
	if n.Name == "each_with_index" && len(args) > 0 || len(args) > 1 {
		return nil
	}
	var recv expr
	f.probe(func() { recv = f.genExpr(inner.Receiver, nil) })
	name := "__" + inner.Name + "_with_index"
	if f.resolve(recv.typ, name) == nil {
		return nil
	}
	off := parser.Node(&parser.IntegerNode{Location: parser.Location{StartOffset: n.Location.StartOffset}, Value: 0}) // no source text: the literal is its Value
	if len(args) == 1 {
		off = args[0]
	}
	return &parser.CallNode{Location: n.Location, Receiver: inner.Receiver, Name: name, Block: n.Block,
		Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{off}}}
}

// slicePattern is `xs.slice_before(pat)` / `slice_after(pat)` as the block form `{ |x| pat === x }` (#145), for a pat
// that reads the same each time (a literal, a local, a constant): no `__slice_*` twin, whose name a `slice` call's
// overload scan would also match.
func slicePattern(n *parser.CallNode) *parser.CallNode {
	if n.Name != "slice_before" && n.Name != "slice_after" || n.Block != nil || len(callArgs(n)) != 1 {
		return nil
	}
	pat := callArgs(n)[0]
	switch pat.(type) {
	case *parser.RegularExpressionNode, *parser.StringNode, *parser.IntegerNode, *parser.SymbolNode, *parser.RangeNode,
		*parser.LocalVariableReadNode, *parser.ConstantReadNode, *parser.ConstantPathNode:
	default:
		return nil
	}
	loc := n.Location
	x := "slice_pattern_x_"
	body := &parser.CallNode{Location: loc, Receiver: pat, Name: "===", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{&parser.LocalVariableReadNode{Location: loc, Name: x}}}}
	blk := &parser.BlockNode{Location: loc, Locals: []string{x},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: x}}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{body}}}
	c := *n
	c.Arguments, c.Block = nil, blk
	return &c
}
