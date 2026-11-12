package compiler

import (
	"fmt"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// reopenedMarker is the program's own definition of name on TrueClass, FalseClass or NilClass (`class FalseClass; def
// present? = false`), whose values are Boolean and nil: nil when only the prelude's markers or Object answer.
func (f *fctx) reopenedMarker(cls, name string) *entry {
	c := f.c.classes[cls]
	if c == nil {
		return nil
	}
	e, ok := c.Methods[name]
	if !ok || e.File == nil || e.File.isPrelude() {
		return nil
	}
	return &entry{M: e, Owner: c, Entry: c}
}

// markerCall is a call on true/false or nil that the program's reopened TrueClass/FalseClass/NilClass answers: MRI
// picks the method by the value's class, where Boolean would take Object's (decision 241).
func (f *fctx) markerCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if isNil(recv.typ) {
		if f.reopenedMarker("NilClass", name) == nil {
			return expr{}, false
		}
		return f.genMethodCall(n, expr{code: "(*NilClass)(nil)", typ: f.cls("NilClass")}, name, args, block), true
	}
	if f.inMarker || !isClass(recv.typ, "Boolean") || f.reopenedMarker("TrueClass", name) == nil && f.reopenedMarker("FalseClass", name) == nil {
		return expr{}, false
	}
	arm := func(cls string) expr {
		if f.reopenedMarker(cls, name) == nil { // Boolean's own answer, past this check
			f.inMarker = true
			defer func() { f.inMarker = false }()
			return f.genMethodCall(n, expr{code: recv.code, typ: recv.typ}, name, args, block)
		}
		return f.genMethodCall(n, expr{code: "(*" + cls + ")(nil)", typ: f.cls(cls)}, name, args, block)
	}
	rv := f.newTmp()
	f.emit("%s := %s", rv, recv.code)
	recv = expr{code: rv, typ: recv.typ}
	var te, fe expr
	f.probe(func() { te, fe = arm("TrueClass"), arm("FalseClass") })
	t, ok := join(te.typ, fe.typ)
	if !ok {
		t = TAny{}
	}
	out := f.newTmp()
	f.emit("var %s %s", out, f.c.goType(t))
	f.emit("if bool(%s) {", rv)
	f.indent++
	f.emit("%s = %s", out, f.coerce(n, arm("TrueClass"), t))
	f.indent--
	f.emit("} else {")
	f.indent++
	f.emit("%s = %s", out, f.coerce(n, arm("FalseClass"), t))
	f.indent--
	f.emit("}")
	return expr{code: out, typ: t}, true
}

// rubyShapedCall: calls whose MRI result needs another shape than the prelude method's typing gives (decision 241).
func (f *fctx) rubyShapedCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if e, ok := f.mixedArrayOp(n, recv, name, args, block); ok {
		return e, true
	}
	if e, ok := f.grepBlockLastMatch(n, recv, name, args, block); ok {
		return e, true
	}
	if e, ok := f.scanBlockLastMatch(n, recv, name, args, block); ok {
		return e, true
	}
	return f.subBlockLastMatch(n, recv, name, args, block)
}

// mixedArrayOp is `xs + ys` / `xs | ys` of Arrays, or `h.merge(g)` of Hashes, whose element (value) types differ,
// neither holding the other: MRI's result mixes them, so both sides become containers of the join first (decision 241).
func (f *fctx) mixedArrayOp(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	cls := "Array"
	if name == "merge" {
		cls = "Hash"
	} else if name != "+" && name != "|" {
		return expr{}, false
	}
	if len(args) != 1 || block != nil || !isClass(recv.typ, cls) {
		return expr{}, false
	}
	at := f.probeType(args[0], nil)
	if !isClass(at, cls) {
		return expr{}, false
	}
	ra, aa := recv.typ.(TClass).Args, at.(TClass).Args
	if cls == "Hash" && !typeEq(ra[0], aa[0]) {
		if !f.lenient() || isAny(ra[0]) && isAny(ra[1]) {
			return expr{}, false // `{a: 1}.merge(h)`: mixed keys stay merge's own conversion
		}
		// a gem's `options.merge(Sandbox::INHERITANCE_FD => fd)`, an Integer key into Symbol keys: MRI's result holds both,
		// so both sides are Hash[untyped, untyped] (decision 256)
		ra, aa = []Type{TAny{}, ra[1]}, []Type{TAny{}, aa[1]}
		if !typeEq(ra[1], aa[1]) {
			ra[1], aa[1] = TAny{}, TAny{}
		}
	}
	a, b := ra[len(ra)-1], aa[len(aa)-1]
	keysJoined := cls == "Hash" && isAny(ra[0]) && !isAny(recv.typ.(TClass).Args[0])
	switch {
	case keysJoined:
	case isAny(a) && isAny(b), typeEq(a, b):
		return expr{}, false
	case isAny(a) || isAny(b):
		if !f.lenient() {
			return expr{}, false // user code: the untyped side is converted to the typed one, as before
		}
	case fitsValue(expr{typ: b}, a) || fitsValue(expr{typ: a}, b):
		return expr{}, false
	}
	jt := joinUnion(a, b)
	if isAny(a) || isAny(b) { // a gem's `{build_env: true}.merge(options)` with options untyped: MRI's result holds both
		jt = TAny{}
	}
	j := TClass{C: f.c.classes[cls], Args: append(slices.Clone(ra[:len(ra)-1]), jt)}
	r := f.coerce(n, recv, TAny{})
	r = f.pinSimple(r)
	r = f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(j), r, j.String()))
	arg := f.genExpr(args[0], nil)
	ac := f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(j), f.coerce(args[0], arg, TAny{}), j.String()))
	return f.genMethodCall(n, expr{code: r, typ: j}, name, []parser.Node{&exprNode{e: expr{code: ac, typ: j}}}, nil), true
}
