package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// Fused `h[k] op= v` and `h[k] = (h[k] || d) op v` (decision 350): a hit updates the entry in place with one lookup; a miss keeps the unfused code for defaults, order and iteration errors.

// fuseOps on Integer and Float run no Ruby code but the operation itself.
var fuseOps = map[string]bool{"+": true, "-": true, "*": true, "/": true, "%": true}

// fuseHash: only the prelude's own Hash with an Integer, Float or String V, whose operators call no user code, so the slot cannot move under them.
func (f *fctx) fuseHash(t Type) (TClass, bool) {
	ht, ok := t.(TClass)
	if !ok || ht.C != f.c.classes["Hash"] || len(ht.Args) != 2 {
		return TClass{}, false
	}
	if v := ht.Args[1]; !isClass(v, "Integer") && !isClass(v, "Float") && !isClass(v, "String") {
		return TClass{}, false
	}
	for _, name := range []string{"[]", "[]="} {
		if e := ht.C.lookup(name); e == nil || e.M.Owner != ht.C || e.M.File == nil || !e.M.File.isPrelude() {
			return TClass{}, false
		}
	}
	return ht, true
}

// fusePure: n runs no Ruby code that could touch a Hash (an Integer's + with another class would call its coerce).
func (f *fctx) fusePure(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.IntegerNode, *parser.FloatNode, *parser.SymbolNode, *parser.StringNode, *parser.NilNode, *parser.TrueNode,
		*parser.FalseNode, *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.ConstantReadNode:
		return true
	case *parser.ParenthesesNode:
		s, ok := n.Body.(*parser.StatementsNode)
		return ok && len(s.Body) == 1 && f.fusePure(s.Body[0])
	case *parser.CallNode:
		args := callArgs(n)
		if n.Block != nil || n.Receiver == nil || !fuseOps[n.Name] || len(args) != 1 || n.IsSAFE_NAVIGATION() {
			return false
		}
		for _, o := range []parser.Node{n.Receiver, args[0]} {
			if !f.fusePure(o) {
				return false
			}
			var t Type
			f.probe(func() { t = f.genExpr(o, nil).typ })
			if !isClass(t, "Integer") && !isClass(t, "Float") {
				return false
			}
		}
		return true
	}
	return false
}

// fuseResult: `x op value` must be a V again (Integer + Float is a Float, which a V slot cannot hold).
func (f *fctx) fuseResult(n parser.Node, v Type, op string, value parser.Node) bool {
	if !fuseOps[op] || !f.fusePure(value) {
		return false
	}
	if isClass(v, "String") && op != "+" {
		return false
	}
	var rt Type
	f.probe(func() { rt = f.genOp(n, expr{code: "x", typ: v}, op, value).typ })
	return rt != nil && typeEq(rt, v)
}

// genFusedSlot: the frozen check follows the new value, as in Ruby, where []= runs after it is computed.
func (f *fctx) genFusedSlot(n parser.Node, recv expr, ht TClass, key expr, op string, value parser.Node, miss func() expr) expr {
	v := ht.Args[1]
	res, slot := f.newTmp(), f.newTmp()
	f.emit("var %s %s", res, f.c.goType(v))
	f.emit("if %s := rbHashSlot(%s, %s); %s != nil {", slot, recv.code, f.coerce(n, key, ht.Args[0]), slot)
	f.indent++
	saved := f.enterBlock()
	val := f.genOp(n, expr{code: "(*" + slot + ")", typ: v}, op, value)
	f.emit("%s = %s", res, f.coerce(n, val, v))
	f.emit("%s.rbCheckFrozen()", recv.code)
	f.emit("*%s = %s", slot, res)
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	m := miss()
	f.emit("%s = %s", res, f.coerce(n, m, v))
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s", res) // a statement's value is unused: wastedassign would flag the miss arm's store
	return expr{code: res, typ: v, done: true}
}

// genFusedElem is `a[i] op= v` of an Array of Integers or Floats at an Integer index within it: one bounds test, then
// the element read and written where it lies (a histogram's `counts[b] += 1`; decision 469). The read-then-write it
// replaces tests the index for the read, again for the write, and for a negative index each time. An index outside
// the Array (negative, or past the end: nil, or growth) takes that path, miss.
func (f *fctx) genFusedElem(n parser.Node, recv expr, elem Type, idx expr, op string, value parser.Node, miss func() expr) expr {
	res, i := f.newTmp(), f.newTmp()
	f.emit("var %s %s", res, f.c.goType(elem))
	if isOpt(idx.typ) { // an Integer? index (`--nil-check=runtime`, a gem's): nil takes the miss path, which raises as before
		f.emit("if %s := %s; %s.ok && uint64(%s.v) < uint64(len(%s.s)) {", i, idx.code, i, i, recv.code)
		i += ".v"
	} else {
		f.emit("if %s := %s; uint64(%s) < uint64(len(%s.s)) {", i, idx.code, i, recv.code)
	}
	f.indent++
	saved := f.enterBlock()
	val := f.genOp(n, expr{code: recv.code + ".s[" + i + "]", typ: elem}, op, value)
	f.emit("%s = %s", res, f.coerce(n, val, elem))
	f.emit("%s.rbCheckFrozen()", recv.code)
	f.emit("%s.s[%s] = %s", recv.code, i, res)
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	m := miss()
	f.emit("%s = %s", res, f.coerce(n, m, elem))
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s", res) // a statement's value is unused: wastedassign would flag the miss arm's store
	return expr{code: res, typ: elem, done: true}
}

// genFusedIndexAssign: k must be pure because Ruby evaluates it twice, here once.
func (f *fctx) genFusedIndexAssign(n *parser.CallNode) (expr, bool) {
	if n.Name != "[]=" || n.Block != nil || n.IsSAFE_NAVIGATION() || f.noFuse == n {
		return expr{}, false
	}
	args := callArgs(n)
	if len(args) != 2 || !isHashVarRead(n.Receiver) || !f.fusePure(args[0]) {
		return expr{}, false
	}
	val, ok := args[1].(*parser.CallNode)
	if !ok || val.Block != nil || val.Receiver == nil || len(callArgs(val)) != 1 {
		return expr{}, false
	}
	paren, ok := val.Receiver.(*parser.ParenthesesNode)
	if !ok {
		return expr{}, false
	}
	body, ok := paren.Body.(*parser.StatementsNode)
	if !ok || len(body.Body) != 1 {
		return expr{}, false
	}
	or, ok := body.Body[0].(*parser.OrNode)
	if !ok {
		return expr{}, false
	}
	read, ok := or.Left.(*parser.CallNode)
	if !ok || read.Name != "[]" || read.Block != nil || read.IsSAFE_NAVIGATION() || len(callArgs(read)) != 1 ||
		!isHashVarRead(read.Receiver) || f.f.text(read.Receiver.GetLocation()) != f.f.text(n.Receiver.GetLocation()) ||
		f.f.text(callArgs(read)[0].GetLocation()) != f.f.text(args[0].GetLocation()) {
		return expr{}, false
	}
	var rt Type
	f.probe(func() { rt = f.genExpr(n.Receiver, nil).typ })
	ht, ok := f.fuseHash(rt)
	if !ok || !f.fuseResult(val, ht.Args[1], val.Name, callArgs(val)[0]) {
		return expr{}, false
	}
	recv := f.valueOf(f.genExpr(n.Receiver, nil))
	recv.code = f.pinSimple(recv.code)
	key := f.valueOf(f.genExpr(args[0], nil))
	key.code = f.pinSimple(key.code)
	return f.genFusedSlot(val, recv, ht, key, val.Name, callArgs(val)[0], func() expr {
		f.noFuse = n
		defer func() { f.noFuse = nil }()
		return f.genExpr(n, nil)
	}), true
}

func isHashVarRead(n parser.Node) bool {
	switch n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode:
		return true
	}
	return false
}
