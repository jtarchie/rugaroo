package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// expr is a generated Go expression with its Ruby type.
type expr struct {
	code     string
	typ      Type
	classObj bool // exactly a class constant: its Go type is the concrete metaclass
	ctor     bool // a struct class's NewX(...): its Go type is *X, not XI
	lit      bool // untyped Go constant
	stmt     bool // already a complete statement (assignment, panic)
	noreturn bool // panic/exit: terminates the statement list
	done     bool // already emitted; code only names the value
	assert   bool // ends in a Go type assertion, which is not a statement
	// view is the untyped Go value that code converts, for an Array or Hash
	// seen as another instantiation (decision 21): a narrowed local, or a
	// call sent through it. Calls and untyped uses take the value itself.
	view    string
	nilable bool // untyped collapsed from untyped? (Hash#[]): calls warn like T?'s
	// provisional: an inference round's untyped result of a method whose parameters are not typed yet (decision 358)
	provisional bool
	// bin: a String the compiler knows is binary (ASCII-8BIT: `.b`, binread), so its calls take the byte-wise
	// `__bin_*` twins; binDeps are the locals it was read from, binary only while each of them is (decision 352).
	bin     bool
	binDeps []*localInfo
}

// genExpr notes each value's type for Marshal: an inferred local's container type is never rendered (decision 137).
func (f *fctx) genExpr(n parser.Node, expected Type) expr {
	if f.c.genCount != nil {
		*f.c.genCount++
		if f.c.genLimit != nil && *f.c.genCount > *f.c.genLimit {
			f.errorf(n, "generation budget exceeded")
		}
	}
	if (expected == nil || holdsAny(expected)) && f.openArgs != nil && isEmptyLit(n) {
		if t := f.openArgType(n); t != nil { // `each_with_object(Hash.new(0))` typed by its block (decision 344)
			expected = t
		}
	}
	e := f.genExpr0(n, expected)
	switch n.(type) {
	case *parser.YieldNode, *parser.SuperNode, *parser.ForwardingSuperNode: // the block or the parent may reassign ivars
		f.unnarrowIvars()
	}
	f.c.noteMarshal(e.typ)
	if fn, ok := e.typ.(TFunc); ok && fn.Proc {
		f.c.goType(fn) // a proc literal's Go type is written out inline, so rbIsProc (Marshal's "Proc" TypeError) would miss it
	}
	return e
}

func (f *fctx) genExpr0(n parser.Node, expected Type) expr {
	if e, ok := f.genLiteral(n); ok {
		return e
	}
	switch n := n.(type) {
	case *parser.LocalVariableReadNode:
		return f.genLocalRead(n)
	case *parser.ItLocalVariableReadNode:
		v := f.readLocal(&parser.LocalVariableReadNode{Name: "it", Location: n.Location})
		return expr{code: v.goName, typ: v.typ, view: v.view}
	case *parser.LocalVariableWriteNode:
		return f.genLocalWrite(n)
	case *parser.LocalVariableOperatorWriteNode:
		cur := f.genExpr(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location}, nil)
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		return f.assignLocal(n, n.Name, val, nil)
	case *parser.InstanceVariableReadNode, *parser.InstanceVariableWriteNode, *parser.InstanceVariableOperatorWriteNode,
		*parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOperatorWriteNode, *parser.ClassVariableOrWriteNode:
		return f.genIvarExpr(n)
	case *parser.CallNode:
		return f.genCallValue(n, expected)
	case *assignedArg:
		return f.genAssignedArg(n, expected)
	case *parser.ArrayNode:
		return f.genArray(n, expected)
	case *parser.HashNode:
		return f.genHash(n, n.Elements, expected)
	case *parser.RegularExpressionNode, *parser.InterpolatedRegularExpressionNode:
		return f.genRegexp(n)
	case *parser.MatchWriteNode:
		return f.genMatchWrite(n)
	case *parser.LocalVariableOrWriteNode, *parser.InstanceVariableOrWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode,
		*parser.InstanceVariableAndWriteNode, *parser.LocalVariableAndWriteNode:
		return f.genOrWrite(n)
	case *parser.CallOperatorWriteNode, *parser.IndexOperatorWriteNode:
		return f.genOpWrite(n)
	case *parser.MultiWriteNode:
		return f.genMultiWrite(n)
	case *parser.KeywordHashNode:
		// `f(a: 1)` on a method without keyword params passes a Hash.
		return f.genHash(n, n.Elements, expected)
	case *parser.ParenthesesNode:
		return f.genParens(n, expected)
	case *parser.IfNode, *parser.UnlessNode, *parser.CaseNode, *parser.CaseMatchNode, *parser.BeginNode,
		*parser.WhileNode, *parser.UntilNode: // a loop's value is nil, or what a `break v` passes (decision 364)
		return f.lift(n, expected, func(t tail) { f.genStmt(n, t) })
	case *parser.MatchRequiredNode, *parser.MatchPredicateNode:
		return f.genMatch(n)
	case *parser.OrNode:
		return f.genOr(n)
	case *parser.AndNode:
		return f.genAnd(n)
	case *parser.YieldNode:
		return f.genYield(n)
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		return f.genSuperNode(n)
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return f.genConstRead(n)
	case *parser.RescueModifierNode:
		return f.genRescueModifier(n, expected)
	case *parser.RangeNode:
		return f.genRange(n, expected)
	case *parser.LambdaNode:
		return f.genLambda(n, n, n.Parameters, expected)
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

func (f *fctx) genLocalWrite(n *parser.LocalVariableWriteNode) expr {
	var ann Type
	if t := f.f.trailingAnnotation(n); t != "" {
		ann = f.parseTypeAnn(n, t)
	}
	exp := ann
	if exp == nil {
		if v := f.visibleLocal(n.Name); v != nil {
			exp = v.typ
		}
	}
	exp = f.selfProcDecl(n, ann, exp)
	f.noteAliasSrc(n)
	tmpl := f.openTemplate(n.Value)
	empty := ann == nil && (isEmptyLit(n.Value) || tmpl != nil)
	if exp == nil && empty {
		exp = f.refined[f.localKey(n.Name)]
	}
	if exp == nil && empty && f.pass == 1 { // pass 1: untyped, so what goes in it can type it
		exp = f.openType(n.Value)
		if tmpl != nil {
			exp = TClass{C: tmpl, Args: slices.Repeat([]Type{TAny{}}, len(tmpl.TypeParams))}
		}
	}
	seed := f.widenSeed(n, ann, &exp)
	if r, ok := n.Value.(*parser.LocalVariableReadNode); ok && r.Name == n.Name && f.erbDepth > 0 { // `x = x` declares x for the template's parse: a no-op
		if v := f.visibleLocal(n.Name); v != nil {
			return expr{code: v.goName, typ: v.typ, view: v.view, done: true}
		}
		return expr{code: "", typ: TNil{}, done: true} // declared later in the def: not in scope here
	}
	val := f.localWriteValue(n, ann, exp)
	if val.noreturn { // `x = raise "e"`: x still needs a Go type for the (unreachable) code after
		t := exp
		if t == nil {
			t = TAny{}
		}
		val = expr{code: f.coerce(n, val, t), typ: t}
	}
	e := f.assignLocal(n, n.Name, val, ann)
	if info := f.localInfo(n.Name); seed != nil && info != nil { // what later writes put in it may widen it (decision 193)
		info.open, info.seeded = true, true
		if _, ok := f.filledArray(n.Value); ok {
			info.filled = true
		}
		if kv, ok := seed.(TTuple); ok && isClass(stripOpt(info.typ), "Hash") { // a Hash literal's key and value types
			info.elems[0], info.elems[1] = append(info.elems[0], kv.Elems[0]), append(info.elems[1], kv.Elems[1])
		} else {
			info.elems[0] = append(info.elems[0], seed)
		}
	}
	if info := f.localInfo(n.Name); empty && f.pass == 1 && info != nil {
		info.open = true
	}
	return e
}

// localWriteValue is the value a local write stores, expected to be exp.
func (f *fctx) localWriteValue(n *parser.LocalVariableWriteNode, ann, exp Type) expr {
	var val expr
	if f.lenient() && ann == nil && exp != nil { // a gem's local rebound to what its type does not hold (`data = { **data, ... }`)
		err := f.tryClean(func() { val = f.genExpr(n.Value, exp) })
		if err != nil {
			val = f.genExpr(n.Value, nil) // as itself; the local widens (decision 233)
		}
	} else {
		val = f.genExpr(n.Value, exp)
	}
	if _, fill := f.filledArray(n.Value); fill && exp != nil && !typeEq(val.typ, exp) && !isAny(exp) { // a fresh Array.new(n, 0) as the Array[untyped] its writes need (decision 414)
		val = expr{code: f.coerce(n, val, exp), typ: exp}
	}
	return val
}

func isRangeNode(n parser.Node) bool {
	_, ok := n.(*parser.RangeNode)
	return ok
}

func (f *fctx) cls(name string) Type { return TClass{C: f.c.classes[name]} }

func (f *fctx) parseTypeAnn(n parser.Node, s string) Type {
	t, err := rbs.ParseType(s)
	if err != nil {
		f.errorf(n, "%v", err)
	}
	var tps []string
	if f.m != nil {
		tps = f.m.TypeParams
	}
	return f.c.resolveType(t, typeScope{class: f.owner, lex: f.lex, methodTPs: tps, srcPos: srcPos{f.f, f.f.line(n.GetLocation().StartOffset)}})
}

func isSelfVar(t Type) bool { v, ok := t.(TVar); return ok && v.Name == "Self" }

// firstType is ts[0], or nil when ts is empty.
func firstType(ts []Type) Type {
	if len(ts) == 0 {
		return nil
	}
	return ts[0]
}

// literalName returns the text of a Symbol or String literal.
func literalName(n parser.Node) string {
	switch n := n.(type) {
	case *parser.SymbolNode:
		return n.Unescaped.Value
	case *parser.StringNode:
		return n.Unescaped.Value
	}
	return ""
}

// constPath turns "A::B" into a constant node lookupConst understands.
func constPath(path string) parser.Node {
	parts := strings.Split(strings.TrimPrefix(path, "::"), "::")
	var n parser.Node = &parser.ConstantReadNode{Name: parts[0]}
	for _, p := range parts[1:] {
		name := p
		n = &parser.ConstantPathNode{Parent: n, Name: &name}
	}
	return n
}

// exprNode carries an already generated expression where a node is
// expected (wrapper arguments).
type exprNode struct {
	parser.Node
	e        expr
	blockArg bool // a dynamic call's block value, appended to its arguments (universalBlockCall): not a parameter's use
	use      Type // what a pending parameter records instead of e.typ: a union argument's arm records the union (unionArgCall)
}

func (x *exprNode) GetLocation() parser.Location {
	if x.Node != nil {
		return x.Node.GetLocation() // the source node it was generated from
	}
	return parser.Location{}
}
func (x *exprNode) CompactChildNodes() []parser.Node { return nil } // a leaf: already generated
func (x *exprNode) ChildNodes() []parser.Node        { return nil }

func isSelf(n parser.Node) bool { _, ok := n.(*parser.SelfNode); return ok }

// genRescueModifier compiles `expr rescue fallback`: the fallback's value
// when expr raises a StandardError.
func (f *fctx) genRescueModifier(n *parser.RescueModifierNode, expected Type) expr {
	var e, r expr
	f.probe(func() {
		e = f.genExpr(n.Expression, expected)
		f.rescues++ // `x rescue $!` reads the handled exception (decision 414)
		r = f.genExpr(n.RescueExpression, expected)
		f.rescues--
	})
	typ, ok := join(e.typ, r.typ)
	switch {
	case r.noreturn: // `expr rescue raise(...)`
		typ = e.typ
	case !ok:
		typ = TAny{}
	}
	if isVoid(typ) && !isNil(typ) || isVoid(stripOpt(typ)) && !isNil(stripOpt(typ)) { // `require(x) rescue nil` (drb's auto_load): void or nil, untyped (decision 416)
		typ = TAny{}
	}
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(typ))
	f.emit("rbBegin(func() {")
	f.indent++
	saved := f.enterBlock()
	f.emit("defer func() {")
	f.emit("\tif r_ := recover(); r_ != nil {")
	if readsErrorInfo(&parser.StatementsNode{Body: []parser.Node{n.RescueExpression}}) {
		f.emit("\t\tr_ = rbCaptureBacktrace(r_)")
	}
	f.emit("\t\tr_ = rbWrapPanic(r_)")
	f.emit("\t\tif !rbIsA[StandardErrorI](r_) {")
	f.emit("\t\t\tpanic(r_)")
	f.emit("\t\t}")
	f.indent += 2
	f.rescues++
	r = f.genExpr(n.RescueExpression, typ)
	f.rescues--
	if r.noreturn {
		f.emit("%s", r.code)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, r, typ))
	}
	f.indent -= 2
	f.emit("\t}")
	f.emit("}()")
	e = f.genExpr(n.Expression, typ)
	if isVoid(e.typ) && !isNil(e.typ) { // `require(x) rescue nil`: the call runs, its value is nil
		f.emitExprStmt(n, e)
		f.emit("%s = nil", tmp)
	} else {
		f.emit("%s = %s", tmp, f.coerce(n, e, typ))
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("})")
	return expr{code: tmp, typ: typ}
}

// genRange builds a Range literal directly: generic classes have no class methods.
func (f *fctx) genRange(n *parser.RangeNode, expected Type) expr {
	if n.Left == nil && n.Right == nil {
		f.errorf(n, "a range needs a begin or an end")
	}
	var want Type
	if et, ok := expected.(TClass); ok && et.C.RubyName == "Range" && len(et.Args) == 1 {
		want = et.Args[0]
	}
	var parts []parser.Node
	for _, p := range []parser.Node{n.Left, n.Right} {
		if p != nil {
			parts = append(parts, p)
		}
	}
	var types []Type
	f.probe(func() {
		for _, p := range parts {
			types = append(types, f.genExpr(p, want).typ)
		}
	})
	elem := want
	inf := len(types) == 2 && isClass(types[0], "Integer") && isFloatInfinity(n.Right) && (want == nil || isClass(want, "Integer"))
	switch {
	case inf: // `1..Float::INFINITY` iterates Integers forever, as MRI: an endless Range[Integer] (decision 140)
		elem = types[0]
	case elem == nil:
		elem = f.joinAll(n, types)
	}
	if elem == nil || isVoid(elem) { // an endpoint that raises (decision 363) or has no value
		f.errorf(n, "cannot type this Range: an endpoint has no value")
	}
	t := TClass{C: f.c.classes["Range"], Args: []Type{elem}}
	code := "(&" + strings.TrimPrefix(f.c.goType(t), "*") + "{"
	if n.Left != nil {
		code += "b: " + f.coerce(n.Left, f.genExpr(n.Left, elem), elem)
	} else {
		code += "beginless: true"
	}
	switch {
	case inf:
		code += ", endless: true, inf: true"
	case n.Right != nil:
		code += ", e: " + f.coerce(n.Right, f.genExpr(n.Right, elem), elem)
	default:
		code += ", endless: true"
	}
	if n.IsEXCLUDE_END() {
		code += ", excl: true"
	}
	return expr{code: code + "})", typ: t}
}

// mentionsSelf reports whether t holds the Self type variable.
func mentionsSelf(t Type) bool {
	switch t := t.(type) {
	case TVar:
		return t.Name == "Self"
	case TOpt:
		return mentionsSelf(t.Elem)
	case TClass:
		return slices.ContainsFunc(t.Args, mentionsSelf)
	case TTuple:
		return slices.ContainsFunc(t.Elems, mentionsSelf)
	case TAny, TFunc, TNil, TUnion, TVoid:
	}
	return false
}

// probing reports a dry-run generation (probe), whose output is discarded.
func (f *fctx) probing() bool { return f.probeDepth > 0 }

// rangeHead opens a range over an iterator; one yielding no values binds none (`for range seq`).
func rangeHead(vars, call string) string {
	if strings.TrimSpace(vars) == "" {
		return "for range " + call + " {"
	}
	return "for " + vars + " := range " + call + " {"
}

// defaultsFile is where m's parameter defaults are written: a synthesized `new`'s are its initialize's.
func defaultsFile(m *Method) *File {
	if m.Kind == kindSynth && m.Name == "new" && m.Owner != nil && m.Owner.metaOf != nil {
		if init := m.Owner.metaOf.lookup("initialize"); init != nil && init.M.File != nil {
			return init.M.File
		}
	}
	return m.File
}

// isTypeVar is a type parameter other than Self (a Hash subclass's untyped V).
func isTypeVar(t Type) bool {
	v, ok := t.(TVar)
	return ok && v.Name != "Self"
}

// localVarRef is the name and scope depth of a node that reads or writes a local.
func localVarRef(n parser.Node) (name string, depth uint32, ok bool) {
	switch w := n.(type) {
	case *parser.LocalVariableReadNode:
		return w.Name, w.Depth, true
	case *parser.LocalVariableWriteNode:
		return w.Name, w.Depth, true
	case *parser.LocalVariableTargetNode:
		return w.Name, w.Depth, true
	case *parser.LocalVariableAndWriteNode:
		return w.Name, w.Depth, true
	case *parser.LocalVariableOrWriteNode:
		return w.Name, w.Depth, true
	case *parser.LocalVariableOperatorWriteNode:
		return w.Name, w.Depth, true
	}
	return "", 0, false
}

// genParens is `(x)`, `()` (nil) and `(a; b)` (a begin's statements, decision 364).
func (f *fctx) genParens(n *parser.ParenthesesNode, expected Type) expr {
	st, ok := n.Body.(*parser.StatementsNode)
	if n.Body == nil || ok && st == nil { // `()` is nil (decision 364)
		return expr{code: "nil", typ: TNil{}, lit: true}
	}
	if ok && len(st.Body) > 1 { // `(a; b)`: a begin's statements, its value the last
		return f.genExpr(f.c.rewrite(n, func() parser.Node { return &parser.BeginNode{Location: n.Location, Statements: st} }), expected)
	}
	if !ok || len(st.Body) != 1 {
		f.errorf(n, "parenthesised statement lists are not supported")
	}
	e := f.genExpr(st.Body[0], expected)
	e.code = "(" + e.code + ")"
	return e
}
