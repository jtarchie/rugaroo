package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// parenBang is a gem's `(o = q.downcase).tr!('_', '-')`: the assignment, then the bang on o (decision 244).
func (f *fctx) parenBang(n *parser.CallNode) (expr, bool) {
	w := parenWrite(n.Receiver)
	if w == nil || !f.lenient() || !strings.HasSuffix(n.Name, "!") {
		return expr{}, false
	}
	f.emitExprStmt(w, f.genExpr(w, nil))
	bang := *n
	bang.Receiver = &parser.LocalVariableReadNode{Location: w.NameLoc, Name: w.Name, Depth: w.Depth}
	return f.genStringBang(&bang)
}

// parenWrite is `(x = v)`'s write, or nil.
func parenWrite(n parser.Node) *parser.LocalVariableWriteNode {
	pn, ok := n.(*parser.ParenthesesNode)
	if !ok {
		return nil
	}
	st, ok := pn.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return nil
	}
	w, _ := st.Body[0].(*parser.LocalVariableWriteNode)
	return w
}

// genStringBang compiles a String bang method (and <<) on a local in a gem file as a rebinding of that local
// (decision 172): Strings stay values (decision 136), and `x.sub!(a, b)` is `x = x.sub(a, b)`, nil when unchanged.
func (f *fctx) genStringBang(n *parser.CallNode) (expr, bool) {
	if e, ok := f.userAppend(n); ok {
		return e, true
	}
	if e, ok := f.userSetbyte(n); ok {
		return e, true
	}
	if e, ok := f.genOutBuffer(n); ok {
		return e, true
	}
	if e, ok := f.parenBang(n); ok {
		return e, true
	}
	if e, ok := f.freshAppend(n); ok {
		return e, true
	}
	if e, ok := f.ternaryAppend(n); ok {
		return e, true
	}
	if inner, ok := n.Receiver.(*parser.CallNode); ok && n.Name == "<<" && inner.Name == "<<" { // `x << a << b`: x << a, then x << b
		root := inner
		for r, ok := root.Receiver.(*parser.CallNode); ok && r.Name == "<<"; r, ok = r.Receiver.(*parser.CallNode) {
			root = r
		}
		lv := root.Receiver // a local, or (kramdown's `last.value << joiner << result`) a target read again after the write
		if _, local := lv.(*parser.LocalVariableReadNode); !local && (!f.rereadable(lv) || !f.bangTargetOK(root, lv)) {
			return expr{}, false
		}
		if _, ok := f.genStringBang(inner); !ok {
			return expr{}, false
		}
		outer := *n
		outer.Receiver = lv
		return f.genStringBang(&outer)
	}
	lv, write, ok := f.stringBangTarget(n, n.Receiver)
	if !ok || !f.lenient() || !strings.HasSuffix(n.Name, "!") && !strAppendNames[n.Name] && n.Name != "replace" && n.Name != "clear" && n.Name != "[]=" {
		return expr{}, false
	}
	if n.IsSAFE_NAVIGATION() { // kramdown's `second&.gsub!(re, s)`: nothing when the local is nil
		if _, local := lv.(*parser.LocalVariableReadNode); local {
			return f.safeStringBang(n, lv, write)
		}
		return expr{}, false
	}
	if it := f.heldBlockParam(lv); it != "" && strAppendNames[n.Name] && !f.noStrIndexSet { // noStrIndexSet: the non-String arm's own `<<`
		return f.heldParamAppend(n, lv, it, write)
	}
	return f.stringBangOn(n, lv, write)
}

// freshPieceIters yield Strings (or numbers) nothing else holds: a rebinding append on their block parameter loses nothing.
var freshPieceIters = setOf("each_char", "each_line", "each_byte", "each_grapheme_cluster", "each_codepoint", "times",
	"upto", "downto", "step", "loop")

// heldBlockParam is the iterator lv is a block parameter of, when the receiver still holds what it binds (an element, a memo).
func (f *fctx) heldBlockParam(lv parser.Node) string {
	r, ok := lv.(*parser.LocalVariableReadNode)
	if !ok || f.scopeRoot == nil {
		return ""
	}
	key := f.localKey(r.Name)
	if li := f.locals[key]; key.scope == methodScope || li == nil || !li.noHoist || li.param {
		return ""
	}
	var it string
	walkNode(f.scopeRoot, func(n parser.Node) (bool, bool) {
		if c, ok := n.(*parser.CallNode); ok && c.Block != nil && c.Block.GetLocation().StartOffset == key.scope {
			it = c.Name
			return true, false
		}
		return false, !opensScope(n)
	})
	if !discardBlocks[it] || freshPieceIters[it] {
		return ""
	}
	return it
}

// heldParamAppend raises NotImplementedError where a rebinding would silently leave the element unchanged (decision 405).
func (f *fctx) heldParamAppend(n *parser.CallNode, lv parser.Node, it string, write func(expr)) (expr, bool) {
	msg := fmt.Sprintf("rugaroo: %s:%d appends to a block parameter of %s, whose String the receiver still holds (an Array element, say), and Strings are values in rugaroo (decision 405)",
		f.f.Name, f.f.line(n.Location.StartOffset), it)
	t := stripOpt(f.probeType(lv, nil))
	if u, ok := t.(TUnion); !isAny(t) && !mayHoldStr(t) || ok && !slices.ContainsFunc(u.Members, func(m Type) bool { return isClass(m, "String") }) {
		return f.stringBangOn(n, lv, write) // `v << nil` on a v narrowed to an Array: no String edit
	}
	if !isAny(t) {
		f.errorf(n, "undefined method %s for String: `%s` is a block parameter of %s, whose String the receiver still holds, and Strings are values in rugaroo (decision 405)", n.Name, lv.(*parser.LocalVariableReadNode).Name, it)
	}
	return f.untypedStringEdit(n, lv, func(string) (expr, bool) {
		return expr{code: fmt.Sprintf("panic(NewNotImplementedError(Ref(String(%q))))", msg), typ: TVoid{}, stmt: true, noreturn: true}, true
	}), true
}

// ternaryAppend is a gem's `(c ? a : b) << x` as `c ? a << x : b << x`, so each branch's receiver gets its own rule:
// kramdown's `(cells.empty? ? cells : cells.last) << str` pushes onto the Array or appends to its last String (decision
// 334).
func (f *fctx) ternaryAppend(n *parser.CallNode) (expr, bool) {
	if n.Name != "<<" || !f.lenient() || n.Block != nil || len(callArgs(n)) != 1 {
		return expr{}, false
	}
	pn, ok := n.Receiver.(*parser.ParenthesesNode)
	if !ok {
		return expr{}, false
	}
	st, ok := pn.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return expr{}, false
	}
	ifn, ok := st.Body[0].(*parser.IfNode)
	if !ok || ifn.Statements == nil || len(ifn.Statements.Body) != 1 {
		return expr{}, false
	}
	els, ok := ifn.Subsequent.(*parser.ElseNode)
	if !ok || els.Statements == nil || len(els.Statements.Body) != 1 {
		return expr{}, false
	}
	push := func(recv parser.Node) *parser.StatementsNode {
		c := *n
		c.Receiver = recv
		return &parser.StatementsNode{Location: recv.GetLocation(), Body: []parser.Node{&c}}
	}
	e := *els
	e.Statements = push(els.Statements.Body[0])
	split := *ifn
	split.Statements = push(ifn.Statements.Body[0])
	split.Subsequent = &e
	return f.genExpr(&split, nil), true
}

// bangTargetOK reports, without emitting anything, whether a String edit on recv can be written back (stringBangTarget).
func (f *fctx) bangTargetOK(n parser.Node, recv parser.Node) bool {
	_, _, ok := f.stringBangTarget(n, recv)
	return ok
}

// safeStringBang is `x&.sub!(...)` on a local: the edit when x holds a value, nil otherwise.
func (f *fctx) safeStringBang(n *parser.CallNode, lv parser.Node, write func(expr)) (expr, bool) {
	plain := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: n.Name, MessageLoc: n.MessageLoc, Arguments: n.Arguments, Block: n.Block}
	x := f.genExpr(lv, nil)
	res := f.newTmp()
	f.emit("var %s any", res)
	f.emit("if rbUnbox(any(%s)) != nil {", x.code)
	f.indent++
	saved := f.enterBlock()
	if lr, ok := lv.(*parser.LocalVariableReadNode); ok && isOpt(x.typ) {
		if v := f.visibleLocal(lr.Name); v != nil {
			f.applyNarrow([]narrowInfo{{local: v, typ: stripOpt(x.typ)}}) // `if x`: non-nil inside
		}
	}
	e, ok := f.stringBangOn(plain, lv, write)
	if !ok {
		e = f.genExpr(plain, nil)
	}
	if e.stmt && !e.done || isVoid(e.typ) {
		f.emit("%s", e.code)
	} else { // a done edit's code is the String it left (net-ftp's `result&.concat(data)`, decision 416)
		f.emit("%s = %s", res, f.coerce(n, e, TAny{}))
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s", res)
	return expr{code: res, typ: TAny{}}, true
}

// stringBangOn is genStringBang on the local lv, whose String write rebinds.
func (f *fctx) stringBangOn(n *parser.CallNode, lv parser.Node, write func(expr)) (expr, bool) {
	t := f.probeType(lv, nil)
	str := f.cls("String")
	if _, local := lv.(*parser.LocalVariableReadNode); isAny(t) && !f.noStrIndexSet {
		switch {
		case n.Name == "[]=" && local && f.stringEditArgs(callArgs(n)):
			return f.untypedIndexSet(n, lv, write), true
		case (n.Name == "clear" || n.Name == "replace") && local: // net-protocol's rbuf_fill: `@rbuf << rv; rv.clear` (decision 416)
			return f.untypedStringEdit(n, lv, func(sv string) (expr, bool) {
				return f.stringReplaceBang(n, callArgs(n), func() string { return sv }, write)
			}), true
		case strAppendNames[n.Name] || strings.HasSuffix(n.Name, "!") && f.c.classes["String"].lookup(strings.TrimSuffix(n.Name, "!")) != nil:
			// kramdown's adapt_source: `source = source.encode('UTF-8'); source.gsub!(...); source << "\n"` (decision 330)
			return f.untypedStringEdit(n, lv, func(sv string) (expr, bool) {
				if strAppendNames[n.Name] { // its value is the local re-read after the write: lv itself
					return f.stringAppendBang(n, lv, callArgs(n), func() string { return sv }, write)
				}
				return f.stringBangOn(n, &exprNode{Node: lv, e: expr{code: sv, typ: str}}, write)
			}), true
		}
	}
	u, isUnion := stripOpt(t).(TUnion)
	if !typeEq(t, str) && !typeEq(t, TOpt{Elem: str}) && !(isUnion && slices.ContainsFunc(u.Members, func(m Type) bool { return typeEq(m, str) })) {
		return expr{}, false
	}
	cur := func() string { // the local's String; anything else raises NoMethodError, as the bang call would in MRI
		x := f.genExpr(lv, nil)
		switch {
		case typeEq(x.typ, str):
			return x.code
		case typeEq(x.typ, TOpt{Elem: str}):
			return fmt.Sprintf("rbStrRecv(%s, %q)", x.code, n.Name)
		}
		return fmt.Sprintf("rbStrRecvAny(%s, %q)", x.code, n.Name)
	}
	args := callArgs(n)
	switch n.Name {
	case "<<", "concat", "append_as_bytes":
		return f.stringAppendBang(n, lv, args, cur, write)
	case "replace", "clear": // rackup's Stream::Reader#read refills its buffer
		return f.stringReplaceBang(n, args, cur, write)
	case "slice!":
		return f.stringSliceBang(n, args, cur, write)
	case "[]=":
		return f.stringIndexSet(n, args, cur(), write)
	}
	plain := strings.TrimSuffix(n.Name, "!")
	if f.c.classes["String"].lookup(plain) == nil {
		return expr{}, false
	}
	before, after := f.newTmp(), f.newTmp()
	f.emit("%s := %s", before, cur())
	next := f.genExpr(&parser.CallNode{Location: n.Location, Receiver: &exprNode{Node: lv, e: expr{code: before, typ: str}},
		Name: plain, MessageLoc: n.MessageLoc, Arguments: n.Arguments, Block: n.Block}, str) // `gsub!(re) { }` keeps its block
	f.emit("%s := %s", after, f.coerce(n, next, str))
	write(expr{code: after, typ: str})
	return expr{code: fmt.Sprintf("rbStrChanged(%s, %s)", before, after), typ: TOpt{Elem: str}}, true
}

// stringReplaceBang is a gem's `s.replace(t)` / `s.clear`: s rebound to t, or to "".
func (f *fctx) stringReplaceBang(n *parser.CallNode, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) != map[string]int{"replace": 1, "clear": 0}[n.Name] {
		return expr{}, false
	}
	with := `""`
	if n.Name == "replace" {
		with = f.coerce(args[0], f.genExpr(args[0], str), str)
	}
	tmp := f.newTmp()
	f.emit("%s := rbStrReplace(%s, %s)", tmp, cur(), with) // cur: a non-String receiver raises as MRI's would
	write(expr{code: tmp, typ: str})
	return expr{code: tmp, typ: str}, true
}

// untypedIndexSet is a gem's `s[i] = v` on an untyped local: a String is rebound, anything else (a parsed JSON Hash)
// takes []= as before.
func (f *fctx) untypedIndexSet(n *parser.CallNode, lv parser.Node, write func(expr)) expr {
	return f.untypedStringEdit(n, lv, func(sv string) (expr, bool) { return f.stringIndexSet(n, callArgs(n), sv, write) })
}

// untypedStringEdit is a gem's String edit on an untyped local: onString (the edit rebinding the local) when it holds a
// String, the call as before on anything else.
func (f *fctx) untypedStringEdit(n *parser.CallNode, lv parser.Node, onString func(sv string) (expr, bool)) expr {
	x := f.genExpr(lv, nil)
	res, sv := f.newTmp(), f.newTmp()
	f.emit("var %s any", res)
	f.emit("if %s, ok := %s.(String); ok {", sv, x.code)
	f.indent++
	f.emit("_ = %s // an append chain's edit may read the local again instead", sv)
	saved := f.enterBlock()
	var r expr
	ok := false
	err := f.tryClean(func() { r, ok = onString(sv) })
	if err != nil { // `@tree.children << el`: no String takes an Element
		ok = false
	}
	switch {
	case ok && r.noreturn:
		f.emit("%s", r.code)
	case ok:
		f.emit("%s = %s", res, f.coerce(n, r, TAny{}))
	default:
		f.emit("panic(rbNoMethod(%q, %s, false))", n.Name, sv)
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	f.noStrIndexSet = true
	e := f.genExpr(n, nil)
	f.noStrIndexSet = false
	if e.stmt || isVoid(e.typ) {
		f.emit("%s", e.code)
	} else {
		f.emit("%s = %s", res, f.coerce(n, e, TAny{}))
	}
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	f.emit("_ = %s // `x << a << b`: the chain reads x again, not this", res)
	return expr{code: res, typ: TAny{}}
}

// stringEditArgs reports `[]=` arguments only a String edit takes: Integer or Range indexes and a String value (a
// Hash's `h[file] = libs` keeps its own path).
func (f *fctx) stringEditArgs(args []parser.Node) bool {
	if len(args) < 2 || len(args) > 3 {
		return false
	}
	for i, a := range args {
		t := f.probeType(a, nil)
		if i == len(args)-1 {
			return isClass(t, "String")
		}
		if !isClass(t, "Integer") && !isClass(t, "Range") {
			return false
		}
	}
	return false
}

// stringIndexSet is a gem's `s[range] = v` / `s[i, n] = v` / `s[i] = v` (patchelf's `new_dynstr[i..j] = "..."`):
// s rebound to the edited String, the value v (decision 324).
func (f *fctx) stringIndexSet(n *parser.CallNode, args []parser.Node, cur string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) < 2 || len(args) > 3 {
		return expr{}, false
	}
	var at expr
	f.probe(func() { at = f.genExpr(args[0], nil) })
	helper, idx := "__with_span", slices.Clone(args[:len(args)-1])
	switch {
	case isClass(at.typ, "Range") && len(args) == 2:
		helper = "__with_range"
	case len(args) == 2:
		idx = append(idx, &exprNode{e: expr{code: "Integer(1)", typ: f.cls("Integer"), lit: true}})
	}
	val := f.genExpr(args[len(args)-1], str)
	if !typeEq(val.typ, str) { // `h[k] = obj` of an untyped h that is a Hash at run time: a String only if it is one
		val = expr{code: f.coerce(args[len(args)-1], val, TAny{}), typ: TAny{}}
	}
	vt, s0 := f.newTmp(), f.newTmp()
	f.emit("%s := String(%s)", vt, f.coerce(args[len(args)-1], val, str))
	f.emit("%s := String(%s)", s0, cur)
	write(f.genMethodCall(n, expr{code: s0, typ: str}, helper, append(idx, &exprNode{e: expr{code: vt, typ: str}}), nil))
	return expr{code: vt, typ: str}, true
}

// stringAppendBang is a gem's `s << x` or `s.concat(a, b)` (decision 172, 233): s rebound to the joined String.
func (f *fctx) stringAppendBang(n *parser.CallNode, lv parser.Node, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) == 0 || n.Name == "<<" && len(args) != 1 {
		return expr{}, false
	}
	acc := cur()
	for _, arg := range args {
		a := f.genExpr(arg, nil)
		var code string
		switch {
		case n.Name == "append_as_bytes": // a String's bytes or an Integer's low byte
			code = "rbBytesArg(" + f.coerce(arg, a, TAny{}) + ")"
		case typeEq(a.typ, TOpt{Elem: str}): // `s << nil` raises in MRI too
			code = "rbMustStr(" + a.code + ")"
		default:
			code = f.coerce(arg, a, str)
		}
		acc = "String.Op_plus(" + acc + ", " + code + ")"
	}
	write(expr{code: acc, typ: str})
	v := f.genExpr(lv, nil)
	return expr{code: v.code, typ: v.typ, stmt: true, done: true}, true
}

// stringSliceBang is a gem's `s.slice!(i, n)` or `s.slice!(range)` (decision 172, 233): the part taken, s rebound
// to the rest.
func (f *fctx) stringSliceBang(n *parser.CallNode, args []parser.Node, cur func() string, write func(expr)) (expr, bool) {
	str := f.cls("String")
	if len(args) == 1 { // `buf.slice!(0..i)`: a Range's part, as String#[] takes it
		var at expr
		f.probe(func() { at = f.genExpr(args[0], nil) })
		if !isClass(at.typ, "Range") {
			return expr{}, false
		}
		s0, got := f.newTmp(), f.newTmp()
		f.emit("%s := %s", s0, cur())
		g := f.genMethodCall(n, expr{code: s0, typ: str}, "[]", args, nil)
		f.emit("%s := %s", got, g.code)
		rest := f.genMethodCall(n, expr{code: s0, typ: str}, "__rest_of_slice", args, nil)
		write(rest)
		return expr{code: got, typ: g.typ}, true
	}
	if len(args) != 2 {
		return expr{}, false
	}
	i := f.coerce(args[0], f.genExpr(args[0], f.cls("Integer")), f.cls("Integer"))
	k := f.coerce(args[1], f.genExpr(args[1], f.cls("Integer")), f.cls("Integer"))
	got, rest := f.newTmp(), f.newTmp()
	f.emit("%s, %s := rbStrSliceBang(%s, %s, %s)", got, rest, cur(), i, k)
	write(expr{code: rest, typ: str})
	return expr{code: got, typ: TOpt{Elem: str}}, true
}

// genOutBuffer is a gem's `io.read_nonblock(n, buf)` (readpartial, read, sysread): MRI fills buf in place; rugaroo's
// Strings do not change, so buf is rebound to what was read, which is also the call's value (decisions 172, 233).
func (f *fctx) genOutBuffer(n *parser.CallNode) (expr, bool) {
	args := callArgs(n)
	if !f.lenient() || len(args) != 2 || n.Receiver == nil || n.Block != nil {
		return expr{}, false
	}
	switch n.Name {
	case "read_nonblock", "readpartial", "read", "sysread":
	default:
		return expr{}, false
	}
	buf, write, ok := f.stringBangTarget(n, args[1])
	if !ok {
		return expr{}, false
	}
	if _, local := buf.(*parser.LocalVariableReadNode); local { // a parameter rebound before any read (staticcheck SA4009)
		f.emit("_ = %s", f.genExpr(buf, nil).code)
	}
	call := *n
	call.Arguments = &parser.ArgumentsNode{Location: n.Arguments.Location, Arguments: args[:1]}
	got := f.genExpr(&call, nil)
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, got.code)
	write(expr{code: tmp, typ: got.typ})
	if _, local := buf.(*parser.LocalVariableReadNode); local { // read after, so the rebinding counts as used (SA4006)
		f.emit("_ = %s", f.genExpr(buf, nil).code)
	}
	return expr{code: tmp, typ: got.typ}, true
}

// stringBangTarget is what a gem's String mutation rebinds (decision 172): a local, or (decision 233) an ivar
// (`@buffer.clear` in Homebrew's ReadlineNonblock), with the write that stores the new String.
func (f *fctx) stringBangTarget(n parser.Node, recv parser.Node) (parser.Node, func(expr), bool) {
	switch t := recv.(type) {
	case *parser.LocalVariableReadNode:
		return t, func(e expr) {
			f.assignLocal(n, t.Name, e, nil)
			f.narrowString(t.Name)
			if v := f.visibleLocal(t.Name); v != nil && f.lenient() { // `value.gsub!(re, ' ')` as a block's last use: MRI's edit is in place, ours a dead store
				f.emit("_ = %s", v.goName)
				if src := f.aliasSrc[f.localKey(t.Name)]; src != nil { // liquid's `previous_token = @nodelist.last; previous_token.rstrip!`
					if at, ok := f.elementTarget(src); ok {
						set := &parser.CallNode{Location: src.Location, Receiver: src.Receiver, Name: "[]=", Arguments: &parser.ArgumentsNode{
							Location: src.Location, Arguments: []parser.Node{at, &exprNode{Node: t, e: expr{code: v.goName, typ: v.typ}}}}}
						f.emitSlotWrite(src, set)
					}
				}
			}
		}, true
	case *parser.InstanceVariableReadNode:
		return t, func(e expr) {
			typ := e.typ
			if f.discover && f.lenient() && f.c.findIvar(f.owner, t.Name) == nil { // the edit stores back what the ivar held, untyped: it declares nothing (decision 392)
				typ = TAny{}
			}
			iv := f.ivar(t, t.Name, typ)
			f.emit("%s = %s", f.ivarCode(iv), f.coerce(n, e, iv.Type))
			f.markIvar(iv)
		}, true
	case *parser.CallNode:
		if f.freshString(t) { // `path.basename.to_s.delete_prefix!(x)`: nothing else sees the string, so only the value matters
			return t, func(expr) {}, true
		}
		if at, ok := f.elementTarget(t); ok { // kramdown's `item.value.last << result`: item.value[-1] = item.value[-1] + result
			return t, func(e expr) {
				set := &parser.CallNode{Location: t.Location, Receiver: t.Receiver, Name: "[]=", MessageLoc: t.MessageLoc,
					Arguments: &parser.ArgumentsNode{Location: t.Location, Arguments: []parser.Node{at, &exprNode{Node: t, e: e}}}}
				f.emitSlotWrite(t, set)
			}, true
		}
		if f.lenient() && f.attrTarget(t) { // kramdown's `item.value.sub!(re) { ... }`: item.value = item.value.sub(...) (decision 330)
			return t, func(e expr) {
				set := &parser.CallNode{Location: t.Location, Receiver: t.Receiver, Name: t.Name + "=", MessageLoc: t.MessageLoc,
					Arguments: &parser.ArgumentsNode{Location: t.Location, Arguments: []parser.Node{&exprNode{Node: t, e: e}}}}
				f.emitExprStmt(t, f.genExpr(set, nil))
			}, true
		}
	}
	return nil, nil, false
}

// emitSlotWrite emits set, src's `r[at] = edited`. An untyped r gets it only when it is an Array or a Hash at run time:
// those hold the String src returned, which MRI's edit changes; a StringScanner's or a String's `[i]` is a new String,
// which nothing else holds (kramdown's `name = @src[1]; name.downcase!`; decision 334).
func (f *fctx) emitSlotWrite(src *parser.CallNode, set *parser.CallNode) {
	if !isAny(stripOpt(f.probeType(src.Receiver, nil))) {
		f.emitExprStmt(src, f.genExpr(set, nil))
		return
	}
	r := f.genExpr(src.Receiver, nil)
	f.emit("if rbSlotShared(%s) {", r.code)
	f.emitExprStmt(src, f.genExpr(set, nil))
	f.emit("}")
}

// elementTarget is a gem's `r.last`, `r.first` or `r[i]` (i a literal or a local) on a re-readable r: an Array slot a
// String edit writes back, so the Array sees it as MRI's shared String would; another reference to that String does
// not (decision 136). at is the slot's index.
func (f *fctx) elementTarget(t *parser.CallNode) (at parser.Node, ok bool) {
	if !f.lenient() || t.Block != nil || !f.rereadable(t.Receiver) {
		return nil, false
	}
	args := callArgs(t)
	switch {
	case t.Name == "last" && len(args) == 0:
		at = &exprNode{Node: t, e: expr{code: "Integer(-1)", typ: f.cls("Integer"), lit: true}}
	case t.Name == "first" && len(args) == 0:
		at = &exprNode{Node: t, e: expr{code: "Integer(0)", typ: f.cls("Integer"), lit: true}}
	case t.Name == "[]" && len(args) == 1:
		switch args[0].(type) {
		case *parser.IntegerNode, *parser.LocalVariableReadNode:
			at = args[0]
		default:
			return nil, false
		}
	default:
		return nil, false
	}
	rt := stripOpt(f.probeType(t.Receiver, nil))
	return at, isAny(rt) || isClass(rt, "Array")
}

// rereadable is an expression evaluated again for the write-back without a side effect: a local, an ivar, self, or
// an attribute reader of one.
func (f *fctx) rereadable(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.SelfNode:
		return true
	case *parser.CallNode:
		if (n.Name == "first" || n.Name == "last") && n.Arguments == nil && n.Block == nil { // `kids.first.value << "\n"`
			return f.rereadable(n.Receiver)
		}
		return f.attrTarget(n)
	}
	return false
}

// attrTarget is `r.name` with r a local, ivar or self whose class has a `name=` writer: an attribute a String edit
// can rebind, reading r again (no side effect) to write it.
func (f *fctx) attrTarget(t *parser.CallNode) bool {
	if t.Block != nil || len(callArgs(t)) > 0 || t.Receiver == nil || !isIdentMethod(t.Name) {
		return false
	}
	if !f.rereadable(t.Receiver) {
		return false
	}
	rt := stripOpt(f.probeType(t.Receiver, nil))
	if isAny(rt) { // kramdown's `last.value << text` on an untyped tree's child: written back by a dynamic name= call
		return f.c.anyClassHasAccessor(t.Name)
	}
	cls := classOf(rt)
	return cls != nil && cls.lookup(t.Name+"=") != nil
}

// anyClassHasAccessor is whether some class has both `name` and `name=`. Memoized per name; classList only grows (a metaclass is made on demand, metaclass.go), so a new length drops the memo. The scan per call was 26% of a large gem compile's CPU.
func (c *Compiler) anyClassHasAccessor(name string) bool {
	if len(c.classList) != c.accessorAnyN {
		c.accessorAny, c.accessorAnyN = nil, len(c.classList)
	}
	if v, ok := c.accessorAny[name]; ok {
		return v
	}
	setter := name + "="
	v := slices.ContainsFunc(c.classList, func(k *Class) bool { return k.lookup(setter) != nil && k.lookup(name) != nil })
	if c.accessorAny == nil {
		c.accessorAny = map[string]bool{}
	}
	c.accessorAny[name] = v
	return v
}

// freshAppend is a gem's `' ' * indent << "<" << tag << ">"`: appends onto a String nothing else holds (an operator's
// or interpolation's result), so the chain's value is the concatenation (decision 330). Not for an Integer operand
// (`s << 65` appends a codepoint).
func (f *fctx) freshAppend(n *parser.CallNode) (expr, bool) {
	if n.Name != "<<" || !f.lenient() || n.Block != nil || len(callArgs(n)) != 1 {
		return expr{}, false
	}
	str := f.cls("String")
	var acc expr
	switch r := n.Receiver.(type) {
	case *parser.CallNode:
		if r.Name == "<<" {
			inner, ok := f.freshAppend(r)
			if !ok {
				return expr{}, false
			}
			acc = inner
			break
		}
		if r.Name != "*" && r.Name != "+" && r.Name != "%" && !f.freshString(r) || !isClass(f.probeType(r, nil), "String") {
			return expr{}, false
		}
		acc = f.genExpr(r, str)
	case *parser.InterpolatedStringNode:
		acc = f.genExpr(r, str)
	case *parser.NumberedReferenceReadNode: // `$1 << pad` in a sub! block: each read of $1 is a new String (decision 334)
		if !isClass(stripOpt(f.probeType(r, nil)), "String") {
			return expr{}, false
		}
		acc = expr{code: f.coerce(r, f.genExpr(r, nil), str), typ: str}
	default:
		return expr{}, false
	}
	arg := callArgs(n)[0]
	at := f.probeType(arg, nil)
	if !isClass(at, "String") && !isAny(at) {
		return expr{}, false
	}
	a := f.genExpr(arg, str)
	return expr{code: "String.Op_plus(" + acc.code + ", " + f.coerce(arg, a, str) + ")", typ: str}, true
}

// freshString: call returns a String no other reference holds, as MRI's do: `x.to_s` on a non-String, `s.dup`, `+s`.
func (f *fctx) freshString(call *parser.CallNode) bool {
	if call.Receiver == nil || call.Block != nil || len(callArgs(call)) > 0 {
		return false
	}
	switch call.Name {
	case "dup", "+@":
		return true
	case "to_s":
		rt := f.probeType(call.Receiver, nil)
		_, cls := rt.(TClass)
		return cls && !isClass(rt, "String")
	}
	return false
}
