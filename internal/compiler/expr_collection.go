package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ---- literals

// genSplatArray is a literal with `*xs` among its elements: the elements
// appended in order, each splatted Array's spread in its place.
func (f *fctx) genSplatArray(n *parser.ArrayNode, elemT, hint Type) expr {
	parts := make([]expr, len(n.Elements))
	var ts []Type
	for i, el := range n.Elements {
		mark := f.buf.Len()
		if sp, ok := el.(*parser.SplatNode); ok {
			var want Type
			if hint != nil {
				want = TClass{C: f.c.classes["Array"], Args: []Type{hint}}
			}
			parts[i] = f.genExpr(sp.Expression, want)
			ac, ok := parts[i].typ.(TClass)
			if !ok || ac.C.RubyName != "Array" {
				parts[i] = f.splatToArray(el, sp, parts[i])
				ac = parts[i].typ.(TClass)
			}
			ts = append(ts, ac.Args[0])
		} else {
			parts[i] = f.genExpr(el, hint)
			ts = append(ts, parts[i].typ)
		}
		f.pinExprs(mark, parts[:i])
	}
	if elemT == nil {
		elemT = joinOrAny(ts)
	}
	at := TClass{C: f.c.classes["Array"], Args: []Type{elemT}}
	out := f.newTmp()
	f.emit("%s := &Array[%s]{}", out, f.c.goType(elemT))
	for i, el := range n.Elements {
		if _, ok := el.(*parser.SplatNode); ok {
			if pt := parts[i].typ.(TClass).Args[0]; f.c.goType(pt) != f.c.goType(elemT) && !isAny(pt) {
				// `[f, parent, *siblings]`: Formula elements into an Array[Formula?] convert one by one, as a lone element would
				x := f.newTmp()
				f.emit("for _, %s := range (%s).s {", x, parts[i].code)
				f.emit("\t%s.s = append(%s.s, %s)", out, out, f.coerce(el, expr{code: x, typ: pt}, elemT))
				f.emit("}")
				continue
			}
			f.emit("%s.s = append(%s.s, (%s).s...)", out, out, f.coerce(el, parts[i], at))
			continue
		}
		f.emit("%s.s = append(%s.s, %s)", out, out, f.coerce(el, parts[i], elemT))
	}
	return expr{code: out, typ: at}
}

// splatToArray is `*x` of a non-Array inside an Array literal (decision 241): Ruby spreads x.to_a when x answers
// to_a (a tuple, MatchData, Hash, Range), nothing for nil, and x itself otherwise (`[*"a"]` is `["a"]`).
func (f *fctx) splatToArray(el parser.Node, sp *parser.SplatNode, e expr) expr {
	if isAny(e.typ) && f.lenient() { // `tags = [*tags]` of an untyped tags: spread as MRI would at run time
		return expr{code: "(&Array[any]{s: rbSplatObj(" + e.code + ")})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
	}
	spread := func(v expr) expr {
		if c, ok := v.typ.(TClass); ok && f.c.isDelegator(c.C) { // its object's to_a, at run time (Homebrew's LazyObject)
			return expr{code: "(&Array[any]{s: rbSplatObj(" + f.coerce(sp, v, TAny{}) + ")})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		}
		if c, ok := v.typ.(TClass); ok && c.C.lookup("to_a") == nil {
			return expr{code: "(&Array[" + f.c.goType(v.typ) + "]{s: []" + f.c.goType(v.typ) + "{" + v.code + "}})", typ: TClass{C: f.c.classes["Array"], Args: []Type{v.typ}}}
		}
		a := f.genMethodCall(el, v, "to_a", nil, nil)
		if !isClass(a.typ, "Array") && f.lenient() && !isVoid(a.typ) { // a gem to_a that answers an Array or a tuple (a union): spread at run time
			return expr{code: "(&Array[any]{s: rbSplatAny(" + f.coerce(sp, a, TAny{}) + ")})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		}
		if !isClass(a.typ, "Array") {
			f.errorf(el, "*%s inside an array literal: %s#to_a is %s, not an Array", f.f.text(sp.Expression.GetLocation()), v.typ, a.typ)
		}
		return a
	}
	if u, ok := e.typ.(TUnion); ok { // a type switch: each member spreads as itself would, nil as nothing
		member := func(m Type, code string) expr {
			if isClass(m, "Array") {
				return expr{code: code, typ: m}
			}
			return spread(expr{code: code, typ: m})
		}
		var et Type
		for _, m := range u.Members {
			if isNil(m) {
				continue
			}
			var a expr
			f.probe(func() { a = member(m, "x") })
			if et == nil {
				et = a.typ.(TClass).Args[0]
			} else {
				et = joinUnion(et, a.typ.(TClass).Args[0])
			}
		}
		at := TClass{C: f.c.classes["Array"], Args: []Type{et}}
		out, sw := f.newTmp(), f.newTmp()
		f.emit("%s := &Array[%s]{}", out, f.c.goType(et))
		f.emit("switch %s := %s.(type) {", sw, e.code)
		seen := map[string]bool{}
		for _, m := range u.Members {
			if isNil(m) || seen[f.c.goType(m)] {
				continue
			}
			seen[f.c.goType(m)] = true
			f.emit("case %s:", f.c.goType(m))
			f.indent++
			a := member(m, sw)
			f.emit("%s.s = append(%s.s, (%s).s...)", out, out, f.coerce(el, a, at))
			f.indent--
		}
		f.emit("}")
		return expr{code: out, typ: at}
	}
	o, ok := e.typ.(TOpt)
	if !ok {
		return spread(e)
	}
	rt := f.newTmp()
	inner := expr{code: derefOpt(rt, o), typ: o.Elem}
	var probe expr
	f.probe(func() { probe = spread(inner) })
	out := f.newTmp()
	f.emit("%s := &Array[%s]{}", out, f.c.goType(probe.typ.(TClass).Args[0]))
	f.emit("if %s := %s; %s {", rt, e.code, optNotNil(rt, o))
	f.indent++
	f.emit("%s = %s", out, spread(inner).code)
	f.indent--
	f.emit("}")
	return expr{code: out, typ: probe.typ}
}

func (f *fctx) genArray(n *parser.ArrayNode, expected Type) expr {
	// A literal is never nil, so an expected T? means T.
	expected = stripOpt(expected)
	if u, ok := expected.(TUnion); ok { // the member an Array literal can be: a tuple of its length, else an Array (decision 150)
		expected = f.literalMember(u, func(m Type) { f.genArray(n, m) }, func(m Type) bool { t, ok := m.(TTuple); return ok && len(t.Elems) == len(n.Elements) }, func(m Type) bool { return isClass(m, "Array") })
	}
	if tt, ok := expected.(TTuple); ok && len(tt.Elems) == len(n.Elements) {
		codes := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			mark := f.buf.Len()
			e := f.genExpr(el, tt.Elems[i])
			f.pinBefore(mark, codes[:i])
			codes[i] = f.coerce(el, e, tt.Elems[i])
		}
		return expr{code: f.c.goType(tt) + "{" + strings.Join(codes, ", ") + "}", typ: tt}
	}
	var elemT Type
	if ec, ok := expected.(TClass); ok && ec.C.Name == "Array" {
		elemT = ec.Args[0]
	}
	hint := elemT
	if isAny(expected) {
		hint = TAny{} // a nested literal is untyped-expected too: an Array, not a tuple
		if f.lenient() && len(n.Elements) > 0 {
			// a gem's literal into an untyped slot holds anything later (kramdown's `%w[template]` option default, which
			// its validator map!s to Symbols): Array[untyped], which rbConv still hands a typed reader (decision 330)
			elemT = TAny{}
		}
	}
	if slices.ContainsFunc(n.Elements, isSplat) {
		return f.genSplatArray(n, elemT, hint)
	}
	elems := make([]expr, len(n.Elements))
	for i, el := range n.Elements {
		mark := f.buf.Len()
		elems[i] = f.genExpr(el, hint)
		f.pinExprs(mark, elems[:i])
	}
	if elemT == nil {
		elemT = inferElemType(elems)
		// [Integer, String] with nothing expected is a tuple (a sort key, a
		// multiple-return); anywhere untyped is expected it is an Array.
		if isAny(elemT) && expected == nil && len(elems) > 0 {
			if t, ok := f.tupleLiteral(n, elems); ok {
				return t
			}
		}
	}
	codes := make([]string, len(elems))
	for i, e := range elems {
		codes[i] = f.coerce(n.Elements[i], e, elemT)
	}
	t := TClass{C: f.c.classes["Array"], Args: []Type{elemT}}
	return expr{code: "(" + arrayLit(f.c.goType(elemT), codes) + ")", typ: t}
}

// pinBefore keeps Ruby's left-to-right evaluation when generating an
// operand emitted statements (a hoisted temporary, `x&.y`) after earlier
// operands were generated as plain Go expressions: those earlier codes
// are evaluated into temporaries placed before the new statements.
// Constants need no pinning.
func (f *fctx) pinBefore(mark int, codes []string) {
	if !f.emittedSince(mark) {
		return
	}
	f.insertAt(mark, func() {
		for i, c := range codes {
			if pinnable(c) {
				t := f.newTmp()
				f.emit("%s := %s", t, c)
				codes[i] = t
			}
		}
	})
}

// pinExprs is pinBefore for operands not yet coerced.
func (f *fctx) pinExprs(mark int, es []expr) {
	if !f.emittedSince(mark) {
		return
	}
	f.insertAt(mark, func() {
		for i, e := range es {
			if !e.lit && !isVoid(e.typ) && pinnable(e.code) {
				t := f.newTmp()
				f.emit("%s := %s", t, e.code)
				es[i].code, es[i].view = t, ""
			}
		}
	})
}

// emittedSince reports whether statements (not just //line directives)
// were written to the buffer after mark.
func (f *fctx) emittedSince(mark int) bool {
	for l := range strings.SplitSeq(f.buf.String()[mark:], "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "//line ") {
			return true
		}
	}
	return false
}

// insertAt runs gen with its output placed at mark instead of the end.
func (f *fctx) insertAt(mark int, gen func()) {
	all := f.buf.String()
	var ins strings.Builder
	saved := f.buf
	f.buf = &ins
	gen()
	f.buf = saved
	f.buf.Reset()
	f.buf.WriteString(all[:mark])
	f.buf.WriteString(ins.String())
	f.buf.WriteString(all[mark:])
}

// inferElemType picks the element type of an unannotated array literal.
// Ruby's `[]` is untyped; so is ours.
func inferElemType(elems []expr) Type {
	ts := make([]Type, len(elems))
	for i, e := range elems {
		ts[i] = e.typ
	}
	return joinOrAny(ts)
}

func (f *fctx) genHash(n parser.Node, elements []parser.Node, expected Type) expr {
	if u, ok := stripOpt(expected).(TUnion); ok { // a Hash literal is the union's Hash member it compiles to (decision 150)
		expected = f.literalMember(u, func(m Type) { f.genHash(n, elements, m) }, func(m Type) bool { return isClass(m, "Hash") })
	}
	var kT, vT Type
	if ec, ok := stripOpt(expected).(TClass); ok && ec.C.RubyName == "Hash" {
		kT, vT = ec.Args[0], ec.Args[1]
	}
	type kv struct {
		k, v  expr
		splat parser.Node // `**h`: v is the Hash merged in
	}
	pairs := make([]kv, 0, len(elements))
	// Ruby runs the pairs left to right: when a later key or value emitted statements, the earlier ones (generated as
	// plain Go expressions) are pinned into temporaries first (decision 98), so they are tracked as a flat list.
	var done []expr
	ks, vs := make([]Type, 0, len(elements)), make([]Type, 0, len(elements))
	for _, el := range elements {
		if sp, ok := el.(*parser.AssocSplatNode); ok {
			var want Type
			if kT != nil {
				want = TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}
			}
			mark := f.buf.Len()
			h := f.genExpr(sp.Value, want)
			f.pinExprs(mark, done)
			if f.lenient() && isAny(h.typ) { // a gem's `new(**formula_hash)` of an untyped Hash: converted where it lands (decision 249)
				anyHash := TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}
				h = expr{code: f.coerce(sp, h, anyHash), typ: anyHash}
			}
			ht, ok := h.typ.(TClass)
			if !ok || ht.C.RubyName != "Hash" {
				f.errorf(sp, "**%s must be a Hash, not %s", f.f.text(sp.Value.GetLocation()), h.typ)
			}
			ks, vs = append(ks, ht.Args[0]), append(vs, ht.Args[1])
			pairs = append(pairs, kv{v: h, splat: sp})
			done = append(done, h)
			continue
		}
		a, ok := el.(*parser.AssocNode)
		if !ok {
			f.c.unsupported(f.f, el)
		}
		mark := f.buf.Len()
		k := f.genExpr(a.Key, kT)
		f.pinExprs(mark, done)
		done = append(done, k)
		mark = f.buf.Len()
		v := f.genExpr(a.Value, vT)
		f.pinExprs(mark, done)
		done = append(done, v)
		ks, vs = append(ks, k.typ), append(vs, v.typ)
		pairs = append(pairs, kv{k: k, v: v})
	}
	for i, d := 0, 0; i < len(pairs); i++ { // the pinned codes
		if pairs[i].splat != nil {
			pairs[i].v, d = done[d], d+1
		} else {
			pairs[i].k, pairs[i].v, d = done[d], done[d+1], d+2
		}
	}
	if kT == nil {
		kT, vT = joinOrAny(ks), joinOrAny(vs)
		if isAny(expected) && f.lenient() && len(elements) > 0 { // a gem's options bag into an untyped slot (kramdown's `opts = {indent: 0}`), as genArray's (decision 330)
			vT = TAny{}
		}
	}
	targs := "[" + f.c.goType(kT) + ", " + f.c.goType(vT) + "]"
	// Hash's methods are free funcs (decision 86): the last pair's call is outermost, so the openers go in reverse.
	var open, tail strings.Builder
	for i := len(pairs) - 1; i >= 0; i-- {
		if pairs[i].splat != nil {
			open.WriteString("rbHashSplat(")
		} else {
			open.WriteString("Hash___Set" + targs + "(")
		}
	}
	for _, p := range pairs {
		if p.splat != nil {
			tail.WriteString(", " + f.coerce(p.splat, p.v, TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}) + ")")
			continue
		}
		tail.WriteString(", " + f.coerce(n, p.k, kT) + ", " + f.coerce(n, p.v, vT) + ")")
	}
	code := open.String() + "NewHash" + targs + "()" + tail.String()
	return expr{code: code, typ: TClass{C: f.c.classes["Hash"], Args: []Type{kT, vT}}}
}

// joinOrAny is the element type of an unannotated literal: the join of its
// parts, or untyped when they have none in common (Ruby's literals are
// heterogeneous; RBS would say `untyped` too).
func joinOrAny(ts []Type) Type {
	var out Type
	for _, t := range ts {
		if out == nil {
			out = t
			continue
		}
		j, ok := join(out, t)
		if !ok || isUnion(j) && !isUnion(out) && !isUnion(t) { // ponytail: a mixed literal stays a tuple or untyped; Array[A | B] needs decision 13's fall-back-to-untyped when the typed program fails
			return TAny{}
		}
		out = j
	}
	if out == nil || isNil(out) || isVoid(out) {
		return TAny{}
	}
	return out
}

// arrayLit is a Go *Array literal of elems; Array is a struct holding its slice (decision 96).
func arrayLit(elemGo string, elems []string) string {
	if len(elems) == 0 {
		return "&Array[" + elemGo + "]{}"
	}
	return "&Array[" + elemGo + "]{s: []" + elemGo + "{" + strings.Join(elems, ", ") + "}}"
}
