package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genOrAssign renders `x ||= value` for a target whose current value is cur.
// The value is evaluated only when x is nil (or falsy, for untyped/bool).
func (f *fctx) genOrAssign(n parser.Node, cur expr, value parser.Node, written ...func(expr)) expr {
	elem := stripOpt(cur.typ)
	var cond string
	var want Type
	switch {
	case isAny(elem):
		cond, want = "!"+f.truthy(n, cur), TAny{}
	case isOpt(cur.typ) && isClass(elem, "Boolean"):
		cond, want = "!"+optTruthy(cur.code, cur.typ), cur.typ
	case isOpt(cur.typ):
		cond, want = optIsNil(cur.code, cur.typ), cur.typ
	case isClass(cur.typ, "Boolean"):
		cond, want = "!"+cur.code, cur.typ
	case unionFalsy(cur.typ): // a union holding nil or false (decision 150)
		cond, want = "!rbTruthy("+cur.code+")", cur.typ
	default:
		// never nil or false: Ruby leaves it alone and skips the value
		return expr{code: cur.code, typ: cur.typ, done: true}
	}
	f.emit("if %s {", cond)
	f.indent++
	saved := f.enterBlock()
	v := f.genExpr(value, elem)
	if v.noreturn { // `x ||= raise(...)`
		f.emit("%s", v.code)
	} else {
		f.emit("%s = %s", cur.code, f.coerce(value, v, want))
		for _, w := range written {
			w(expr{code: cur.code, typ: want})
		}
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	if isOpt(cur.typ) && !isAny(elem) && !isOpt(v.typ) && !isNil(v.typ) {
		return expr{code: derefOpt(cur.code, cur.typ), typ: elem, done: true}
	}
	if u, ok := cur.typ.(TUnion); ok && !v.noreturn && !isOpt(v.typ) && !isNil(v.typ) && !unionFalsy(v.typ) && slices.ContainsFunc(u.Members, isNil) {
		// set or already truthy: its nil member is gone (decision 241)
		rest := unionOf(slices.DeleteFunc(slices.Clone(u.Members), isNil)...)
		if isUnion(rest) {
			return expr{code: cur.code, typ: rest, done: true}
		}
		if _, ok := rest.(TClass); ok && !slices.ContainsFunc(u.Members, func(m Type) bool { return isClass(m, "Boolean") }) {
			return expr{code: cur.code + ".(" + f.c.goType(rest) + ")", typ: rest, done: true}
		}
	}
	return expr{code: cur.code, typ: cur.typ, done: true}
}

func (f *fctx) genOrAssignLocal(n *parser.LocalVariableOrWriteNode) expr {
	if f.visibleLocal(n.Name) == nil && f.sameScopeLocal(n.Name) == nil { // one assigned in a branch before is not new (`a, b = x if c`, decision 248)
		// a new local starts out nil, so this is a plain assignment
		v := f.genExpr(n.Value, nil)
		if f.c.env.UntypedWhy && isAny(v.typ) {
			f.setAnyOrigin(n.Name, "||= from: "+f.untypedWhy(n.Value, 1))
		}
		return f.assignLocal(n, n.Name, v, nil)
	}
	v := f.readLocal(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location})
	if v.base != nil {
		return expr{code: v.goName, typ: v.typ, done: true} // narrowed: already set
	}
	if isNil(v.typ) { // only ever nil so far (`x = nil; x ||= v`): an assignment, which widens x to T?
		return f.assignLocal(n, n.Name, f.genExpr(n.Value, nil), nil)
	}
	e := f.genOrAssign(n, expr{code: v.goName, typ: v.typ}, n.Value)
	if info := f.localInfo(n.Name); info != nil {
		info.writes++
		if !isAncestorBlock(info.declBlock, f.block) {
			info.hoist = true
		}
	}
	if isOpt(v.typ) && !isOpt(e.typ) {
		f.applyNarrow([]narrowInfo{{local: v, typ: e.typ}}) // non-nil from here on
	}
	if nw, ok := f.narrowUnion(v, func(m Type) bool { return !isNil(m) }); ok && !isNil(e.typ) {
		f.applyNarrow([]narrowInfo{nw}) // a union's nil is gone from here on (decision 150)
		return expr{code: nw.code, typ: nw.typ, done: true}
	}
	return e
}

func (f *fctx) genOrAssignIvar(n *parser.InstanceVariableOrWriteNode) expr {
	iv := f.c.findIvar(f.owner, n.Name)
	if iv == nil && f.discover {
		// first seen through ||=: it starts out nil
		var t Type
		if a := f.f.trailingAnnotation(n); a != "" {
			t = f.parseTypeAnn(n, a) // `@x ||= [] #: Array[T]` declares the ivar's type
		} else {
			var v expr
			f.probe(func() { v = f.genExpr(n.Value, nil) })
			t = v.typ
		}
		if !isOpt(t) && !isAny(t) && !isClass(t, "Boolean") && !isNil(t) {
			t = TOpt{Elem: t}
		}
		iv = f.ivar(n, n.Name, t)
	}
	if iv == nil {
		iv = f.ivar(n, n.Name, nil)
	}
	f.unnarrow(ivarNarrowKey(n.Name))
	if k, g := classOf(iv.Type), f.c.goType(iv.Type); k != nil && f.lenient() && !f.discover && !isOpt(iv.Type) && (g == k.ifaceName() || strings.HasPrefix(g, "*")) {
		// a gem's ivar typed by its only assignment is still nil until then (bindata's `@factory ||= @obj_class.new`)
		code := f.ivarCode(iv)
		f.emit("if %s == nil {", code)
		f.indent++
		saved := f.enterBlock()
		v := f.genExpr(n.Value, iv.Type)
		f.emit("%s = %s", code, f.coerce(n.Value, v, iv.Type))
		f.leaveBlock(saved)
		f.indent--
		f.emit("}")
		f.markIvar(iv)
		return expr{code: code, typ: iv.Type, done: true}
	}
	e := f.genOrAssign(n, expr{code: f.ivarCode(iv), typ: iv.Type}, n.Value)
	f.markIvar(iv) // set or already truthy, so assigned either way
	if isOpt(iv.Type) && !isAny(stripOpt(iv.Type)) && !isOpt(e.typ) {
		f.narrowIvar(iv) // non-nil from here on (decision 148)
	}
	return e
}

func (f *fctx) genOrWrite(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.LocalVariableOrWriteNode:
		return f.genOrAssignLocal(n)
	case *parser.InstanceVariableOrWriteNode:
		return f.genOrAssignIvar(n)
	case *parser.IndexOrWriteNode:
		return f.genIndexOrWrite(n)
	case *parser.InstanceVariableAndWriteNode: // `@x &&= v` is `@x && (@x = v)`
		w := &parser.InstanceVariableWriteNode{Name: n.Name, NameLoc: n.NameLoc, Value: n.Value, Location: n.Location}
		return f.genExpr(&parser.AndNode{Left: &parser.InstanceVariableReadNode{Name: n.Name, Location: n.NameLoc}, Right: w, Location: n.Location}, nil)
	case *parser.LocalVariableAndWriteNode: // `x &&= v` is `x && (x = v)`
		w := &parser.LocalVariableWriteNode{Name: n.Name, Depth: n.Depth, NameLoc: n.NameLoc, Value: n.Value, Location: n.Location}
		return f.genExpr(&parser.AndNode{Left: &parser.LocalVariableReadNode{Name: n.Name, Depth: n.Depth, Location: n.NameLoc}, Right: w, Location: n.Location}, nil)
	}
	return f.genOrAssignAttr(n.(*parser.CallOrWriteNode))
}

func (f *fctx) genOpWrite(n parser.Node) expr {
	if n, ok := n.(*parser.IndexOperatorWriteNode); ok {
		return f.genIndexOpWrite(n)
	}
	return f.genOpAssignAttr(n.(*parser.CallOperatorWriteNode))
}

// genOpAssignAttr: `recv.x += v` is recv.x=(recv.x + v), and its value is the new one, not the writer's result.
func (f *fctx) genOpAssignAttr(n *parser.CallOperatorWriteNode) expr {
	recv := f.attrRecv(n, n.Receiver, n.IsSAFE_NAVIGATION(), n.ReadName)
	val := f.genOp(n, f.genMethodCall(n, recv, n.ReadName, nil, nil), n.BinaryOperator, n.Value)
	val.code = f.pinSimple(val.code)
	if f.nilGuarded[n] && isOpt(recv.typ) { // the reader's guard already raised on nil: the writer's receiver is not nil
		recv = expr{code: derefOpt(recv.code, recv.typ), typ: stripOpt(recv.typ)}
	}
	f.callWriter(n, recv, n.WriteName, val)
	return expr{code: val.code, typ: val.typ, lit: val.lit, done: true}
}

// genOrAssignAttr: Ruby's `recv.x || recv.x = v`, so the writer runs only when the reader is nil/false.
func (f *fctx) genOrAssignAttr(n *parser.CallOrWriteNode) expr {
	recv := f.attrRecv(n, n.Receiver, n.IsSAFE_NAVIGATION(), n.ReadName)
	cur := f.genMethodCall(n, recv, n.ReadName, nil, nil)
	if !isAny(cur.typ) && !isOpt(cur.typ) && !isClass(cur.typ, "Boolean") {
		return cur // never nil or false: the writer never runs
	}
	if f.nilGuarded[n] && isOpt(recv.typ) { // as in genOpAssignAttr
		recv = expr{code: derefOpt(recv.code, recv.typ), typ: stripOpt(recv.typ)}
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, cur.code)
	return f.genOrAssign(n, expr{code: tmp, typ: cur.typ}, n.Value, func(v expr) { f.callWriter(n, recv, n.WriteName, v) })
}

// indexOperands evaluates an index assignment's receiver and arguments
// once, for both its `[]` read and its `[]=` write.
func (f *fctx) indexOperands(n, rn parser.Node, args *parser.ArgumentsNode, block *parser.BlockArgumentNode, safe bool) (expr, []parser.Node) {
	if block != nil {
		f.errorf(n, "a block argument in an index assignment is not supported")
	}
	recv := f.attrRecv(n, rn, safe, "[]")
	var as []parser.Node
	if args != nil {
		for _, a := range args.Arguments {
			e := f.valueOf(f.genExpr(a, nil))
			if !isSimpleGo(e.code) {
				tmp := f.newTmp()
				f.emit("%s := %s", tmp, f.materialize(e))
				e.code = tmp
			}
			as = append(as, &exprNode{Node: a, e: e})
		}
	}
	return recv, as
}

// genIndexOrWrite: `h[k] ||= v` is `h[k] || h[k] = v`, with h and k evaluated once.
func (f *fctx) genIndexOrWrite(n *parser.IndexOrWriteNode) expr {
	f.noteIndexWrite(n.Receiver, n.Arguments, n.Value)
	recv, as := f.indexOperands(n, n.Receiver, n.Arguments, n.Block, n.IsSAFE_NAVIGATION())
	cur := f.genMethodCall(n, recv, "[]", as, nil)
	if !isAny(cur.typ) && !isOpt(cur.typ) && !isClass(cur.typ, "Boolean") {
		return cur // never nil or false: the writer never runs
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, cur.code)
	return f.genOrAssign(n, expr{code: tmp, typ: cur.typ}, n.Value, func(v expr) {
		if isOpt(v.typ) && !isAny(stripOpt(v.typ)) {
			v = expr{code: derefOpt(v.code, v.typ), typ: stripOpt(v.typ)} // just assigned: not nil
		}
		f.emitExprStmt(n, f.genMethodCall(n, recv, "[]=", append(slices.Clone(as), &exprNode{Node: n.Value, e: v}), nil))
	})
}

// genIndexOpWrite: `a[i] += v` is `a[i] = a[i] + v`, with a and i evaluated once; its value is the new one.
func (f *fctx) genIndexOpWrite(n *parser.IndexOperatorWriteNode) expr {
	f.noteIndexWrite(n.Receiver, n.Arguments, nil)
	recv, as := f.indexOperands(n, n.Receiver, n.Arguments, n.Block, n.IsSAFE_NAVIGATION())
	unfused := func() expr {
		val := f.genOp(n, f.genMethodCall(n, recv, "[]", as, nil), n.BinaryOperator, n.Value)
		val.code = f.pinSimple(val.code)
		f.emitExprStmt(n, f.genMethodCall(n, recv, "[]=", append(slices.Clone(as), &exprNode{Node: n.Value, e: val}), nil))
		return expr{code: val.code, typ: val.typ, lit: val.lit, done: true}
	}
	if ht, ok := f.fuseHash(recv.typ); ok && len(as) == 1 && f.fuseResult(n, ht.Args[1], n.BinaryOperator, n.Value) {
		return f.genFusedSlot(n, recv, ht, as[0].(*exprNode).e, n.BinaryOperator, n.Value, unfused) // the receiver and key evaluate once either way
	}
	return unfused()
}

// attrRecv: the reader and the writer share one evaluation of the receiver.
func (f *fctx) attrRecv(n, rn parser.Node, safe bool, name string) expr {
	if safe {
		f.errorf(n, "&. with an operator assignment is not supported")
	}
	if isSelf(rn) {
		f.unnarrow(attrNarrowKey(name))
	}
	recv := f.valueOf(f.genExpr(rn, nil))
	recv.code = f.pinSimple(recv.code)
	return recv
}

func (f *fctx) callWriter(n parser.Node, recv expr, name string, val expr) {
	f.emitExprStmt(n, f.genMethodCall(n, recv, name, []parser.Node{&exprNode{e: val}}, nil))
}

// genMultiWrite renders `a, b = x, y`, `a, b = tuple`, and with splat or
// nested targets `a, *rest, z = arr` and `a, (b, c) = x`.
func (f *fctx) genMultiWrite(n *parser.MultiWriteNode) expr {
	if arr, ok := n.Value.(*parser.ArrayNode); ok && n.Rest == nil && len(n.Rights) == 0 && len(arr.Elements) == len(n.Lefts) && !slices.ContainsFunc(arr.Elements, isSplat) {
		for i, v := range f.multiLiteral(arr, n.Lefts) {
			f.assignTarget(n.Lefts[i], v)
		}
		return expr{code: "", typ: TVoid{}, stmt: true, done: true}
	}
	v := f.genExpr(n.Value, nil)
	f.noteAnyTargets(n.Value, v, n.Lefts)
	if isNil(v.typ) { // `q, a = nil`: no value to hold (Go has no `x := nil`)
		f.destructureInto(n, v, n.Lefts, n.Rest, n.Rights)
		return expr{code: "", typ: TVoid{}, stmt: true, done: true}
	}
	tmp := f.newTmp()
	if v.lit { // typed: a literal (`a, b = 1`) is an Integer, not Go's int
		f.emit("var %s %s = %s", tmp, f.c.goType(v.typ), v.code)
	} else {
		f.emit("%s := %s", tmp, v.code)
	}
	saved := f.destructuring
	f.destructuring = f.discover && f.lenient() && f.storageDestructure(n.Value)
	f.destructureInto(n, expr{code: tmp, typ: v.typ}, n.Lefts, n.Rest, n.Rights)
	f.destructuring = saved
	if len(n.Lefts) == 0 && len(n.Rights) == 0 && isBareRest(n.Rest) { // `(*) = x` reads nothing of x
		f.emit("_ = %s", tmp)
	}
	return expr{code: tmp, typ: v.typ, stmt: true, done: true} // the value of `a, = x` is x, as a condition (`(a, = x) && ...`) reads it
}

// destructureOpt is `a, b = pair_or_nil` (decision 222).
func (f *fctx) destructureOpt(n parser.Node, v expr, t TOpt, lefts []parser.Node, rest parser.Node, trail int) {
	lead := len(lefts)
	tt, ok := t.Elem.(TTuple)
	if !ok || rest != nil && !isBareRest(rest) || trail > 0 || lead > len(tt.Elems) && !f.lenient() || rest == nil && lead < len(tt.Elems) {
		f.errorf(n, "cannot destructure %s", v.typ)
	}
	// `tag, digest = h.find { }`: nil destructures into nils, as Ruby's to_ary-less nil does (decision 222); `a, = pair_or_nil`
	// keeps the leading fields (decision 241)
	vars := make([]string, lead)
	for i := range lead {
		vars[i] = f.newTmp()
		if i < len(tt.Elems) {
			f.emit("var %s %s", vars[i], f.c.goType(optOf(tt.Elems[i])))
		}
	}
	f.emit("if %s != nil {", v.code)
	for i := range min(lead, len(tt.Elems)) {
		et := tt.Elems[i]
		f.emit("\t%s = %s", vars[i], f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", v.code, i), typ: et}, optOf(et)))
	}
	f.emit("}")
	for i, l := range lefts {
		if i >= len(tt.Elems) {
			f.assignTarget(l, expr{code: "nil", typ: TNil{}})
			continue
		}
		f.assignTarget(l, flatOpt(expr{code: vars[i], typ: optOf(tt.Elems[i])}))
	}
}

// destructureLenient assigns a tuple's first fields to the targets, nil past its end, as Ruby's multiple assignment.
func (f *fctx) destructureLenient(v expr, t TTuple, lefts []parser.Node) {
	for i, l := range lefts {
		if i < len(t.Elems) {
			f.assignTarget(l, expr{code: fmt.Sprintf("%s.F%d", v.code, i), typ: t.Elems[i]})
		} else {
			f.assignTarget(l, expr{code: "nil", typ: TNil{}})
		}
	}
}

// destructureInto assigns tuple or Array v (held in a temporary) to the
// leading targets, a `*rest` target and the trailing ones, as Ruby: an
// Array gives each target its element or nil, rest the middle.
func (f *fctx) destructureInto(n parser.Node, v expr, lefts []parser.Node, rest parser.Node, rights []parser.Node) {
	lead, trail := len(lefts), len(rights)
	var restT parser.Node // the rest's target; nil for a bare `*`
	switch r := rest.(type) {
	case nil, *parser.ImplicitRestNode: // `a, = xs` drops the rest
	case *parser.SplatNode:
		restT = r.Expression
	default:
		f.errorf(rest, "unsupported rest target %s", nodeType(rest))
	}
	switch t := v.typ.(type) {
	case TTuple:
		m := len(t.Elems)
		if rest == nil && trail == 0 && m != lead && f.lenient() { // `a, b, c = four`: the first fields, then nil (decision 233)
			f.destructureLenient(v, t, lefts)
			return
		}
		if rest == nil && m != lead || rest != nil && m < lead+trail {
			f.errorf(n, "%d targets for a %d-tuple", lead+trail, m)
		}
		for i, l := range lefts {
			f.assignTarget(l, expr{code: fmt.Sprintf("%s.F%d", v.code, i), typ: t.Elems[i]})
		}
		if restT != nil {
			midTs := t.Elems[lead : m-trail]
			et := joinOrAny(midTs)
			parts := make([]string, len(midTs))
			for i, mt := range midTs {
				parts[i] = f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", v.code, lead+i), typ: mt}, et)
			}
			f.assignTarget(restT, expr{code: arrayLit(f.c.goType(et), parts), typ: TClass{C: f.c.classes["Array"], Args: []Type{et}}})
		}
		for j, r := range rights {
			f.assignTarget(r, expr{code: fmt.Sprintf("%s.F%d", v.code, m-trail+j), typ: t.Elems[m-trail+j]})
		}
	case TOpt:
		if a, ok := f.optArrayOrEmpty(v); ok {
			f.destructureInto(n, a, lefts, rest, rights)
			return
		}
		f.destructureOpt(n, v, t, lefts, rest, trail)
	case TClass:
		if t.C.RubyName != "Array" {
			f.destructureOne(n, v, t, lefts, rest, rights)
			return
		}
		et := f.c.goType(t.Args[0])
		for i, l := range lefts {
			f.assignTarget(l, genericOpt(expr{code: fmt.Sprintf("Array_Op_idx[%s](%s, %d)", et, v.code, i), typ: TOpt{Elem: t.Args[0]}}))
		}
		if restT != nil {
			f.assignTarget(restT, expr{code: fmt.Sprintf("rbMidSplat(%s, %d, %d)", v.code, lead, trail), typ: v.typ})
		}
		for j, r := range rights {
			f.assignTarget(r, genericOpt(expr{code: fmt.Sprintf("rbTrailIdx(%s, %d, %d, %d)", v.code, lead, trail, j), typ: TOpt{Elem: t.Args[0]}}))
		}
	case TAny: // MRI's implicit to_ary: an Array splits, anything else is the first target
		f.destructureInto(n, f.toAry(v), lefts, rest, rights)
	case TUnion:
		f.destructureUnion(n, v, t, lefts, rest, trail)
	case TNil: // `q, a = nil`: every target nil, a rest an empty Array (decision 244)
		for _, l := range append(slices.Clone(lefts), rights...) {
			f.assignTarget(l, expr{code: "nil", typ: TNil{}})
		}
		if restT != nil {
			f.assignTarget(restT, expr{code: "(&Array[any]{})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}})
		}
	case TFunc, TVar, TVoid: // only tuples and Arrays split
		f.errorf(n, "cannot destructure %s", v.typ)
	}
}

// optArrayOrEmpty is an Array? as the Array it destructures as: nil is an empty one, every target nil (decision 369).
func (f *fctx) optArrayOrEmpty(v expr) (expr, bool) {
	c, ok := stripOpt(v.typ).(TClass)
	if !ok || !isOpt(v.typ) || c.C.RubyName != "Array" {
		return expr{}, false
	}
	tmp := f.newTmp()
	f.emit("%s := %s{}", tmp, strings.Replace(f.c.goType(c), "*", "&", 1))
	f.emit("if p := %s; p != nil {", f.materialize(v))
	f.emit("\t%s = %s", tmp, derefOpt("p", v.typ))
	f.emit("}")
	return expr{code: tmp, typ: c}, true
}

// destructureOne is `a, b = x` of an object with no to_ary: x is the first target alone, as a one-element Array (decision 364).
// One with to_ary splits what it answers (matrix's `v, d, v_inv = eigensystem`, decision 414).
func (f *fctx) destructureOne(n parser.Node, v expr, t TClass, lefts []parser.Node, rest parser.Node, rights []parser.Node) {
	if t.C.lookup("to_ary") != nil {
		a := f.valueOf(f.genMethodCall(n, v, "to_ary", nil, nil))
		if c, ok := stripOpt(a.typ).(TClass); ok && c.C.RubyName != "Array" || isVoid(a.typ) {
			f.errorf(n, "cannot destructure %s: its to_ary answers %s", v.typ, a.typ)
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, a.code)
		if ac, ok := a.typ.(TClass); ok && f.lenient() && rest == nil && len(rights) == 0 && len(ac.Args) == 1 && isInterfaceObj(ac.Args[0]) {
			// a gem's `v, d, v_inv = eigensystem` of Matrix values: each target its element, nil past the end, as a gem's T? is
			// its T (decision 172); a *MatrixI would not be the receiver later calls take
			for i, l := range lefts {
				f.assignTarget(l, expr{code: fmt.Sprintf("rbArrAt[%s](%s, %d).v", f.c.goType(ac.Args[0]), tmp, i), typ: ac.Args[0]})
			}
			return
		}
		f.destructureInto(n, expr{code: tmp, typ: a.typ}, lefts, rest, rights)
		return
	}
	gt := f.c.goType(t)
	one := TClass{C: f.c.classes["Array"], Args: []Type{t}}
	f.destructureInto(n, expr{code: "(&Array[" + gt + "]{s: []" + gt + "{" + v.code + "}})", typ: one}, lefts, rest, rights)
}

// isInterfaceObj is a class whose objects are a Go interface (nil holds nil): not a value class, leaf or module.
func isInterfaceObj(t Type) bool {
	c, ok := t.(TClass)
	return ok && c.C.GoType == "" && !c.C.universal && !c.C.IsModule && strings.HasSuffix(c.C.objType(), "I")
}

// destructureUnion is `a, b = x` of a union (decision 241): a type switch gives each target what x's member gives it
// (a tuple's field, an Array's element, a non-Array the first target alone, nil the rest), typed as their join.
func (f *fctx) destructureUnion(n parser.Node, v expr, u TUnion, lefts []parser.Node, rest parser.Node, trail int) {
	if trail > 0 || rest != nil && !isBareRest(rest) {
		f.errorf(n, "cannot destructure %s with a rest target", v.typ)
	}
	field := func(m Type, i int, code string) expr {
		switch mt := m.(type) {
		case TNil:
		case TTuple:
			if i < len(mt.Elems) {
				return expr{code: fmt.Sprintf("%s.F%d", code, i), typ: mt.Elems[i]}
			}
		case TClass:
			if mt.C.RubyName == "Array" {
				return genericOpt(expr{code: fmt.Sprintf("Array_Op_idx[%s](%s, %d)", f.c.goType(mt.Args[0]), code, i), typ: TOpt{Elem: mt.Args[0]}})
			}
			if i == 0 {
				return expr{code: code, typ: m}
			}
		default:
			f.errorf(n, "cannot destructure %s (member %s)", u, m)
		}
		return expr{code: "nil", typ: TNil{}}
	}
	ts := make([]Type, len(lefts))
	for i := range lefts {
		for _, m := range u.Members {
			t := field(m, i, "x").typ
			if ts[i] == nil {
				ts[i] = t
			} else {
				ts[i] = joinUnion(ts[i], t)
			}
		}
		if isNil(ts[i]) {
			ts[i] = TAny{}
		}
	}
	vars := make([]string, len(lefts))
	for i := range lefts {
		vars[i] = f.newTmp()
		f.emit("var %s %s", vars[i], f.c.goType(ts[i]))
	}
	sw := f.newTmp()
	f.emit("switch %s := %s.(type) {", sw, v.code)
	seen := map[string]bool{} // members of one Go type (two tuples of untyped fields) are one case
	for _, m := range u.Members {
		goT := "nil"
		if !isNil(m) {
			goT = f.c.goType(m)
		}
		if seen[goT] {
			continue
		}
		seen[goT] = true
		f.emit("case %s:", goT)
		f.emit("	_ = %s", sw)
		for i := range lefts {
			if e := field(m, i, sw); !isNil(e.typ) {
				f.emit("	%s = %s", vars[i], f.coerce(n, e, ts[i]))
			}
		}
	}
	f.emit("}")
	for i, l := range lefts {
		f.assignTarget(l, flatOpt(expr{code: vars[i], typ: ts[i]}))
	}
}

// isBareRest is a multiple assignment's `a, = x` or `a, * = x`: a rest that keeps nothing.
func isBareRest(rest parser.Node) bool {
	switch r := rest.(type) {
	case *parser.ImplicitRestNode:
		return true
	case *parser.SplatNode:
		return r.Expression == nil
	}
	return false
}

// toAry wraps an untyped value as the Array[untyped] it destructures as.
func (f *fctx) toAry(v expr) expr {
	return expr{code: fmt.Sprintf("rbToAry(%s)", v.code), typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
}

// assignTarget writes v to one target of a multiple assignment.
func (f *fctx) assignTarget(target parser.Node, v expr) {
	switch t := target.(type) {
	case *parser.LocalVariableTargetNode:
		f.assignLocal(t, t.Name, v, nil)
	case *parser.InstanceVariableTargetNode:
		f.storageWrite = f.destructuring
		iv := f.ivar(t, t.Name, v.typ)
		f.storageWrite = false
		f.emit("%s = %s", f.ivarCode(iv), f.coerce(t, v, iv.Type))
		f.markIvar(iv)
	case *parser.MultiTargetNode:
		if c, ok := stripOpt(v.typ).(TClass); ok && isOpt(v.typ) && c.C.RubyName != "Array" && c.C.lookup("to_ary") == nil {
			// `(a, b) = x` of an Integer?: x (nil or not) is the first target alone, as a one-element Array (decision 364)
			gt := f.c.goType(v.typ)
			one := expr{code: "(&Array[" + gt + "]{s: []" + gt + "{" + f.materialize(v) + "}})", typ: TClass{C: f.c.classes["Array"], Args: []Type{v.typ}}}
			f.destructureInto(t, one, t.Lefts, t.Rest, t.Rights)
			return
		}
		if a, ok := f.optArrayOrEmpty(v); ok {
			f.destructureInto(t, a, t.Lefts, t.Rest, t.Rights)
			return
		}
		if isOpt(v.typ) && f.nilAt(t) {
			v = f.nilUnwrap(t, v, stripOpt(v.typ))
		}
		if isOpt(v.typ) {
			f.nilErrorf(t, "nested destructuring of a possibly-nil %s is not supported", v.typ)
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(v))
		f.destructureInto(t, expr{code: tmp, typ: v.typ}, t.Lefts, t.Rest, t.Rights)
	case *parser.IndexTargetNode: // `h[k], o.x = ...` write through []= and x=
		var args []parser.Node
		if t.Arguments != nil {
			args = slices.Clone(t.Arguments.Arguments)
		}
		f.assignCall(t, t.Receiver, "[]=", args, v)
	case *parser.CallTargetNode:
		f.assignCall(t, t.Receiver, t.Name, nil, v)
	case *parser.GlobalVariableTargetNode:
		if t.Name != "$VERBOSE" {
			f.errorf(target, "global variable %s cannot be assigned in a multiple assignment", t.Name)
		}
		f.emit("rbVerbose = %s", f.coerce(t, v, TOpt{Elem: f.cls("Boolean")}))
	default:
		f.errorf(target, "unsupported assignment target %s", nodeType(target))
	}
}

// assignCall writes v through a setter call (recv.name(args..., v)), as a multiple assignment's call targets do.
// ponytail: MRI evaluates the receiver and index before the right-hand sides; here they run after them.
func (f *fctx) assignCall(t parser.Node, recv parser.Node, name string, args []parser.Node, v expr) {
	loc := t.GetLocation()
	val := &exprNode{Node: t, e: expr{code: f.materialize(v), typ: v.typ}}
	f.genStmt(&parser.CallNode{Location: loc, Receiver: recv, Name: name, MessageLoc: &loc,
		Arguments: &parser.ArgumentsNode{Location: loc, Arguments: append(args, val)}}, tail{})
}

// multiLiteral evaluates every right-hand side into a temporary before any
// target is written, so `a, b = b, a` swaps.
func (f *fctx) multiLiteral(arr *parser.ArrayNode, lefts []parser.Node) []expr {
	vals := make([]expr, 0, len(arr.Elements))
	for i, el := range arr.Elements {
		if _, lit := el.(*parser.NilNode); lit && f.lenient() && f.targetType(lefts[i]) == nil {
			vals = append(vals, expr{code: "nil", typ: TNil{}}) // `name, arg = nil, ""` (getoptlong): the target is written nil, as `name = nil` is (decision 410)
			continue
		}
		e := f.genExpr(el, f.targetType(lefts[i]))
		if isNil(e.typ) {
			t := f.targetType(lefts[i])
			if t == nil {
				f.errorf(el, "cannot infer a type from nil; annotate the target first")
			}
			e.typ = t
		}
		tmp := f.newTmp()
		code := f.materialize(e)
		if isNil(e.typ) || code == "nil" {
			f.emit("var %s %s", tmp, f.c.goType(e.typ))
		} else {
			f.emit("%s := %s", tmp, code)
		}
		vals = append(vals, expr{code: tmp, typ: e.typ})
	}
	return vals
}

// targetType is the current type of an assignment target, if it has one.
func (f *fctx) targetType(n parser.Node) Type {
	switch t := n.(type) {
	case *parser.LocalVariableTargetNode:
		if v := f.visibleLocal(t.Name); v != nil {
			if v.base != nil {
				return v.base.typ
			}
			return v.typ
		}
	case *parser.InstanceVariableTargetNode:
		if iv := f.c.findIvar(f.owner, t.Name); iv != nil {
			return iv.Type
		}
	case *parser.GlobalVariableTargetNode:
		return TOpt{Elem: f.cls("Boolean")} // $VERBOSE, the one assignable there
	}
	return nil
}
