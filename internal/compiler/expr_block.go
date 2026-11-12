package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ---- blocks

// blockParam is one parameter of a block or lambda as written: a plain name (numbered `_1` and `it` too), a `*rest`
// (Name "" when anonymous) or a `(k, v)` destructuring of one yielded value (Sub).
type blockParam struct {
	Name string
	Rest bool
	Sub  []blockParam // non-nil: a destructuring tuple of plain names
}

type blockParams []blockParam

// hasRest reports a trailing `*rest` (a bare `|a, |` counts, as an anonymous one).
func (ps blockParams) hasRest() bool { return len(ps) > 0 && ps[len(ps)-1].Rest }

// special reports a rest or destructuring parameter, which the untyped-receiver forms do not take.
func (ps blockParams) special() bool {
	return slices.ContainsFunc(ps, func(p blockParam) bool { return p.Rest || p.Sub != nil })
}

// hasTuple reports a destructuring parameter.
func (ps blockParams) hasTuple() bool {
	return slices.ContainsFunc(ps, func(p blockParam) bool { return p.Sub != nil })
}

// locals are the names prism puts in the block's own Ruby scope: the plain ones, not a rest's or a tuple's.
func (ps blockParams) locals() []string {
	var names []string
	for _, p := range ps {
		if !p.Rest && p.Sub == nil {
			names = append(names, p.Name)
		}
	}
	return names
}

func (f *fctx) blockParamList(b parser.Node) blockParams {
	switch p := b.(type) {
	case nil:
		return nil
	case *parser.BlockParametersNode:
		if p.Parameters == nil {
			return nil
		}
		ps := p.Parameters
		if len(ps.Optionals) > 0 || len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
			f.errorf(b, "unsupported block parameter form")
		}
		var names blockParams
		for _, r := range ps.Requireds {
			switch rp := r.(type) {
			case *parser.RequiredParameterNode:
				names = append(names, blockParam{Name: rp.Name})
			case *parser.MultiTargetNode:
				// `|(k, v), acc|`
				if rp.Rest != nil || len(rp.Rights) > 0 {
					f.errorf(r, "unsupported block parameter form")
				}
				sub := []blockParam{}
				for _, l := range rp.Lefts {
					lp, ok := l.(*parser.RequiredParameterNode)
					if !ok {
						f.errorf(l, "unsupported block parameter form")
					}
					sub = append(sub, blockParam{Name: lp.Name})
				}
				names = append(names, blockParam{Sub: sub})
			default:
				f.errorf(r, "unsupported block parameter form")
			}
		}
		switch r := ps.Rest.(type) {
		case nil:
		case *parser.ImplicitRestNode: // `|a, |` splats a yielded Array as `|a, *|` does; `|a|` takes it whole
			names = append(names, blockParam{Rest: true})
		case *parser.RestParameterNode:
			name := ""
			if r.Name != nil {
				name = *r.Name
			}
			names = append(names, blockParam{Name: name, Rest: true}) // no name: an anonymous rest
		default:
			f.errorf(r, "unsupported block parameter form")
		}
		return names
	case *parser.NumberedParametersNode:
		var names blockParams
		for i := 1; i <= int(p.Maximum); i++ {
			names = append(names, blockParam{Name: fmt.Sprintf("_%d", i)})
		}
		return names
	case *parser.ItParametersNode:
		return blockParams{{Name: "it"}}
	}
	f.c.unsupported(f.f, b)
	return nil
}

// bindLoneSub binds `|(x)|`, a destructuring of one target: x is the value's first element, where `|x|` would take the
// whole value (rubocop's `scan(RE).map { |(digits)| digits.to_i }`; decision 444).
func (f *fctx) bindLoneSub(sub []blockParam, t Type, src string) bool {
	if len(sub) != 1 || sub[0].Sub != nil || sub[0].Rest {
		return false
	}
	var first expr
	switch tt := t.(type) {
	case TTuple:
		if len(tt.Elems) == 0 {
			return false
		}
		first = expr{code: src + ".F0", typ: tt.Elems[0]}
	case TClass:
		if tt.C.RubyName != "Array" || len(tt.Args) != 1 {
			return false
		}
		first = genericOpt(expr{code: fmt.Sprintf("Array_Op_idx[%s](%s, 0)", f.c.goType(tt.Args[0]), src), typ: TOpt{Elem: tt.Args[0]}})
	case TAny:
		first = expr{code: fmt.Sprintf("Opt(Array_Op_idx[any](rbToAry(%s), 0))", src), typ: TAny{}}
	default:
		return false
	}
	v := f.blockParam(sub[0].Name, first.typ)
	if v.goName != "_" {
		f.emit("%s := %s", v.goName, first.code)
		f.noteUnused(v)
	}
	return true
}

// bindRestParams binds `|a, *rest|`: one yielded tuple or Array is split
// across the params when there are leading ones (Ruby's auto-splat);
// otherwise rest collects the yielded values past the leading params.
func (f *fctx) bindRestParams(n parser.Node, names blockParams, yields []Type) ([]string, func()) {
	lead, rest := names[:len(names)-1], names[len(names)-1].Name
	if ac, ok := firstType(yields).(TClass); rest == procArgs && len(lead) == 0 && len(yields) == 1 && ok && ac.C.RubyName == "Array" {
		p := f.newTmp() // a lowered block of several names splats a lone yielded Array, as MRI's proc does (decision 367)
		return []string{p}, func() { f.bindBlockParam(rest, expr{code: p, typ: ac}) }
	}
	if _, untyped := firstType(yields).(TAny); rest == procArgs && len(lead) == 0 && len(yields) == 1 && untyped {
		p := f.newTmp() // an untyped lone value splats when it is an Array (rbAutoSplat)
		return []string{p}, func() {
			f.bindBlockParam(rest, expr{code: "(&Array[any]{s: rbAutoSplat([]any{" + p + "})})", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}})
		}
	}
	if len(yields) == 1 && len(lead) > 0 {
		if ps, pro := f.bindSplatParams(n, yields[0], lead, rest); pro != nil {
			return ps, pro
		}
	}
	if len(lead) > len(yields) {
		f.errorf(n, "block takes %d params but only %d values are yielded", len(lead), len(yields))
	}
	tmps := make([]string, len(yields))
	for i := range tmps {
		tmps[i] = f.newTmp()
	}
	return tmps, func() {
		parts := make([]expr, len(yields))
		for i, y := range yields {
			parts[i] = expr{code: tmps[i], typ: y}
		}
		for i, nm := range lead {
			f.bindBlockParam(nm.Name, parts[i])
		}
		f.bindBlockParam(rest, f.blockArrayOf(n, parts[len(lead):]))
		for i := len(lead); i < len(tmps) && rest == ""; i++ {
			f.emit("_ = %s", tmps[i])
		}
	}
}

// bindSplatParams destructures the single yielded value of a block that also
// has a rest param (a tuple, an Array, or to_ary); nil means the value does
// not splat and the caller takes the plain path.
func (f *fctx) bindSplatParams(n parser.Node, y Type, lead blockParams, rest string) ([]string, func()) {
	p := f.newTmp()
	switch t := y.(type) {
	case TTuple:
		if len(t.Elems) < len(lead) {
			f.errorf(n, "block takes %d params but the tuple has %d elements", len(lead), len(t.Elems))
		}
		return []string{p}, func() {
			parts := make([]expr, len(t.Elems))
			for i, et := range t.Elems {
				parts[i] = expr{code: fmt.Sprintf("%s.F%d", p, i), typ: et}
			}
			for i, nm := range lead {
				f.bindBlockParam(nm.Name, parts[i])
			}
			f.bindBlockParam(rest, f.blockArrayOf(n, parts[len(lead):]))
		}
	case TClass:
		if t.C.RubyName == "Array" {
			return []string{p}, func() {
				for i, nm := range lead {
					f.bindBlockParam(nm.Name, genericOpt(expr{code: fmt.Sprintf("rbSplatAt(%s, %d)", p, i), typ: TOpt{Elem: t.Args[0]}}))
				}
				f.bindBlockParam(rest, expr{code: fmt.Sprintf("rbMidSplat(%s, %d, 0)", p, len(lead)), typ: t})
			}
		}
	case TAny: // MRI's to_ary: an Array splits, anything else is the first param
		return []string{p}, func() {
			a := f.newTmp()
			f.emit("%s := rbToAry(%s)", a, p)
			for i, nm := range lead {
				f.bindBlockParam(nm.Name, flatOpt(expr{code: fmt.Sprintf("rbSplatAt(%s, %d)", a, i), typ: TOpt{Elem: TAny{}}}))
			}
			f.bindBlockParam(rest, expr{code: fmt.Sprintf("rbMidSplat(%s, %d, 0)", a, len(lead)), typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}})
		}
	case TFunc, TNil, TOpt, TUnion, TVar, TVoid: // a lone value: no splat
	}
	return nil, nil
}

// bindBlockParam declares a block parameter local and emits its binding; an
// empty name is a blank destructuring slot.
func (f *fctx) bindBlockParam(name string, e expr) {
	if name == "" {
		return
	}
	v := f.blockParam(name, e.typ)
	switch {
	case v.goName == "_": // `_ := x` declares nothing: Go rejects it
	case e.code == "nil": // a slot past a pair's end (liquid's `|output, (name, args, kwargs)|` over a Hash's pairs, decision 331)
		f.emit("var %s %s", v.goName, f.c.goType(v.typ))
	default:
		f.emit("%s := %s", v.goName, e.code)
	}
	f.noteUnused(v)
}

// blockArrayOf packs the tail of a splat into an Array of the parts' joined type.
func (f *fctx) blockArrayOf(n parser.Node, parts []expr) expr {
	ts := make([]Type, len(parts))
	for i, p := range parts {
		ts[i] = p.typ
	}
	et := joinOrAny(ts)
	codes := make([]string, len(parts))
	for i, p := range parts {
		codes[i] = f.coerce(n, p, et)
	}
	return expr{code: arrayLit(f.c.goType(et), codes), typ: TClass{C: f.c.classes["Array"], Args: []Type{et}}}
}

// bindTupleParams destructures one yielded tuple across several block params.
func (f *fctx) bindTupleParams(n parser.Node, names blockParams, tt TTuple) ([]string, func()) {
	if len(names) != len(tt.Elems) && f.lenient() { // a gem's proc: extra fields dropped, missing params nil (decision 233)
		return f.bindLenientTuple(n, names, tt)
	}
	if len(names) != len(tt.Elems) {
		f.errorf(n, "block takes %d params but the tuple has %d elements", len(names), len(tt.Elems))
	}
	p := f.newTmp()
	return []string{p}, func() {
		lhs := make([]string, 0, len(names))
		rhs := make([]string, 0, len(names))
		vars := make([]*local, 0, len(names))
		var nested []func()
		for i, nm := range names {
			if nm.Sub != nil { // `|(k, v), i|`
				sub, t, src := nm.Sub, tt.Elems[i], fmt.Sprintf("%s.F%d", p, i)
				nested = append(nested, func() {
					if f.bindLoneSub(sub, t, src) {
						return
					}
					gp, pro := f.bindBlockParams(n, sub, []Type{t})
					f.emit("%s := %s", gp[0], src)
					pro()
				})
				continue
			}
			v := f.blockParam(nm.Name, tt.Elems[i])
			vars = append(vars, v)
			if v.goName == "_" { // `|_, (token, _)|`: Go's `_ := x` declares nothing and does not compile
				continue
			}
			lhs = append(lhs, v.goName)
			rhs = append(rhs, fmt.Sprintf("%s.F%d", p, i))
		}
		if len(lhs) > 0 {
			f.emit("%s := %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
		}
		for _, d := range nested {
			d()
		}
		for _, v := range vars {
			f.noteUnused(v)
		}
	}
}

// bindLenientTuple binds a tuple to a different count of plain block params, as a Ruby proc's arity does: the first
// fields, then nil.
func (f *fctx) bindLenientTuple(n parser.Node, names blockParams, tt TTuple) ([]string, func()) {
	if names.hasTuple() {
		f.errorf(n, "block takes %d params but the tuple has %d elements", len(names), len(tt.Elems))
	}
	p := f.newTmp()
	return []string{p}, func() {
		for i, nm := range names {
			e := expr{code: "nil", typ: TNil{}}
			if i < len(tt.Elems) {
				e = expr{code: fmt.Sprintf("%s.F%d", p, i), typ: tt.Elems[i]}
			}
			f.bindBlockParam(nm.Name, e)
		}
		f.emit("_ = %s", p)
	}
}

// bindArraySplat splats a yielded Array[elem] across several block params;
// an untyped one (elem TAny) goes through rbToAry first, as MRI's to_ary.
func (f *fctx) bindArraySplat(n parser.Node, names blockParams, elem Type, untyped bool) ([]string, func()) {
	p := f.newTmp()
	splat := "rbSplatAt"
	if f.viewSplat && !untyped {
		splat = "rbSplatVal"
	}
	return []string{p}, func() {
		a := p
		if untyped {
			a = f.newTmp()
			f.emit("%s := rbToAry(%s)", a, p)
		}
		for i, nm := range names {
			e := genericOpt(expr{code: fmt.Sprintf("%s(%s, %d)", splat, a, i), typ: TOpt{Elem: elem}})
			if nm.Sub != nil { // `|(k, v), i|`
				st, code := e.typ, e.code
				if o, ok := st.(TOpt); ok && strings.HasPrefix(f.c.goType(o.Elem), "*Array[") && f.c.goType(st) == "*"+f.c.goType(o.Elem) {
					// `|(a, b), (c, d)|` over pairs of pairs: a missing pair's targets are nil, as the splat of a nil Array's are
					st, code = o.Elem, fmt.Sprintf("rbDerefOrNil(%s)", code)
				}
				gp, pro := f.bindBlockParams(n, nm.Sub, []Type{st})
				f.emit("%s := %s", gp[0], code)
				pro()
				continue
			}
			f.bindBlockParam(nm.Name, e)
		}
	}
}

// bindBlockParams declares block params for the yielded types, returning
// the Go loop/closure parameter names and a destructuring prologue.
func (f *fctx) bindBlockParams(n parser.Node, names blockParams, yields []Type) (goParams []string, prologue func()) {
	if f.c.env.UntypedWhy {
		f.blockWhy = f.blockParamWhy(n)
		defer func() { f.blockWhy = "" }()
	}
	if names.hasRest() {
		return f.bindRestParams(n, names, yields)
	}
	if tt, ok := firstType(yields).(TTuple); ok && len(yields) == 1 && len(names) > 1 {
		return f.bindTupleParams(n, names, tt)
	}
	// Ruby splats a yielded Array across several block params; each gets
	// its element, or nil past the end.
	if ac, ok := firstType(yields).(TClass); len(yields) == 1 && len(names) > 1 && ok && ac.C.RubyName == "Array" && len(ac.Args) == 1 {
		return f.bindArraySplat(n, names, ac.Args[0], false)
	}
	if _, ok := firstType(yields).(TAny); ok && len(yields) == 1 && len(names) > 1 {
		return f.bindArraySplat(n, names, TAny{}, true)
	}
	var extra blockParams // a gem block's params past what is yielded: nil, as a proc's are (decision 241)
	if len(names) > len(yields) {
		if !f.lenient() || names[len(yields):].hasTuple() {
			f.errorf(n, "block takes %d params but only %d values are yielded", len(names), len(yields))
		}
		extra = names[len(yields):]
	}
	var destructure []func()
	for _, nm := range extra {
		destructure = append(destructure, func() {
			v := f.blockParam(nm.Name, TAny{})
			f.emit("var %s any", v.goName)
			f.noteUnused(v)
		})
	}
	for i := range yields {
		switch {
		case i < len(names) && names[i].Sub != nil:
			p := f.newTmp()
			goParams = append(goParams, p)
			sub, t := names[i].Sub, yields[i]
			destructure = append(destructure, func() {
				if f.bindLoneSub(sub, t, p) {
					return
				}
				gp, pro := f.bindBlockParams(n, sub, []Type{t})
				f.emit("%s := %s", gp[0], p)
				pro()
			})
		case i < len(names):
			v := f.blockParam(names[i].Name, yields[i])
			goParams = append(goParams, v.goName)
		default:
			goParams = append(goParams, "_")
		}
	}
	return goParams, func() {
		for _, d := range destructure {
			d()
		}
		for i := range yields {
			if i < len(names) && names[i].Sub == nil {
				if v := f.scope.lookup(names[i].Name); v != nil {
					f.noteUnused(v)
				}
			}
		}
	}
}

// symbolBlockNames are a `&:name` block's params: x_ the receiver, the rest its arguments (`reduce(&:merge)`, decision 241).
func (f *fctx) symbolBlockNames(b parser.Node, name string, n int) blockParams {
	if n == 0 {
		f.errorf(b, "&:%s needs a block that takes a value", name)
	}
	names := blockParams{{Name: "x_"}}
	for i := 1; i < n; i++ {
		names = append(names, blockParam{Name: fmt.Sprintf("a%d_", i)})
	}
	return names
}

// symbolBlockCall is a `&:name` block's body: name called on its first value, the others its arguments.
func (f *fctx) symbolBlockCall(n, block parser.Node, name string, names blockParams) expr {
	v := f.scope.lookup("x_")
	args := make([]parser.Node, 0, len(names)-1)
	for _, nm := range names[1:] {
		args = append(args, &parser.LocalVariableReadNode{Name: nm.Name, Location: block.GetLocation()})
	}
	return f.genMethodCall(n, expr{code: v.goName, typ: v.typ}, name, args, nil)
}

// checkPairResult: a `&:name` block whose call answers no pair where one is wanted (`pairs.reduce(&:join)`, the Hash arm
// of an untyped receiver) is a compile error the arm's caller turns into NoMethodError at run time (decision 444).
func (f *fctx) checkPairResult(n parser.Node, name string, e expr, t tail) {
	if _, want := t.typ.(TTuple); !want || f.heldAsAny(e.typ) {
		return
	}
	if _, got := stripOpt(e.typ).(TTuple); !got {
		f.errorf(n, "&:%s answers %s where the block must answer %s", name, e.typ, t.typ)
	}
}

// withClosureSelf binds self to an escaped block's self param (decision 164)
// for the duration of gen, restoring the enclosing self after.
func (f *fctx) withClosureSelf(self Type, gen func()) {
	if self == nil {
		gen()
		return
	}
	st, sc := f.selfType, f.selfCode
	f.selfType, f.selfCode = self, "self"
	gen()
	f.selfType, f.selfCode = st, sc
}

// funcBlockArg is a dynBlockArg block func passed as a block: itself when the block is that shape (nil when the call
// gave none), else boxed as the Proc genClosure adapts (optparse's on_tail rewritten to __on_tail_flag's (bool)).
func funcBlockArg(b *parser.BlockArgumentNode, want TFunc, self Type) (string, *parser.BlockArgumentNode) {
	en, ok := b.Expression.(*exprNode)
	if !ok {
		return "", b
	}
	ft, isFn := en.e.typ.(TFunc)
	switch {
	case !isFn || ft.Proc || self != nil:
		return "", b
	case typeEq(ft, want):
		return en.e.code, b
	case ft.Rest:
		return "", b
	}
	ft.Proc = true
	return "", &parser.BlockArgumentNode{Location: b.Location, Expression: &exprNode{Node: en.Node, e: expr{code: "(&" + en.e.code + ")", typ: ft}}}
}

// closureHead opens a closure's func literal; wrapped when a generic `{ (E) -> U? }` wants Go's *U around the block's
// own U?: a leaf C's **C (decision 360), an Integer/Float rbOpt's *T (decision 396).
func (f *fctx) closureHead(sig *BlockSig, env map[string]Type, ps []string, ret Type, retS string) bool {
	if boxed := boxedLeafOpt(sig.Ret, env); boxed != nil {
		f.emit("func(%s) *%s {", strings.Join(ps, ", "), f.c.goType(ret))
		f.indent++
		f.emit("return rbBoxNil[%s](func()%s {", boxed.objType(), retS)
		return true
	}
	if boxedValOpt(sig.Ret, env) {
		f.emit("func(%s) *%s {", strings.Join(ps, ", "), f.c.goType(stripOpt(ret)))
		f.indent++
		f.emit("return rbOptToPtr(func()%s {", retS)
		return true
	}
	f.emit("func(%s)%s {", strings.Join(ps, ", "), retS)
	return false
}

// genClosure renders a block as a Go func literal.
func (f *fctx) genClosure(n parser.Node, block parser.Node, sig *BlockSig, env map[string]Type) string {
	bin := f.binNext
	f.binNext = false
	params := substAll(sig.Params, env)
	self := subst(sig.Self, env) // an escaped block's self, a leading Go param (decision 164)
	var body parser.Node
	var names blockParams
	var symbolCall string
	switch b := block.(type) {
	case *parser.BlockNode:
		names = f.blockParamList(b.Parameters)
		body = b.Body
	case *parser.LambdaNode:
		names = f.blockParamList(b.Parameters)
		body = b.Body
	case *parser.BlockArgumentNode:
		if f.isBlockParam(b.Expression) {
			return f.forwardClosure(b, sig, env)
		}
		if code, ok := restBlockFunc(b, sig); ok {
			return code
		}
		var code string
		if code, b = funcBlockArg(b, TFunc{Params: params, Ret: subst(sig.Ret, env), Rest: sig.Rest}, self); code != "" {
			return code
		}
		sym, ok := b.Expression.(*parser.SymbolNode)
		if !ok {
			if code, ok := f.selfProcArg(b, self); ok {
				return code
			}
			return f.genClosure(n, f.blockArgBlock(b, len(params)), sig, env)
		}
		if untypedRest(params, sig.Rest) && self == nil { // `super(&:<=)` into a gem's untyped &block
			return f.symbolProcAny(sym.Unescaped.Value)
		}
		symbolCall = sym.Unescaped.Value
		names = f.symbolBlockNames(b, symbolCall, len(params))
	default:
		f.c.unsupported(f.f, block)
	}
	bindParams := func() ([]string, func()) {
		f.binParams, f.viewSplat = bin, f.windowSplat(block)
		defer func() { f.binParams, f.viewSplat = false, false }()
		return f.bindClosureParams(n, names, params, sig.Rest)
	}
	gen := func(t tail) {
		if symbolCall != "" {
			e := f.symbolBlockCall(n, block, symbolCall, names)
			f.checkPairResult(n, symbolCall, e, t)
			f.applyTail(n, e, t)
			return
		}
		saved := f.bodyStmts
		f.bodyStmts = body
		f.genStmts(body, t)
		f.bodyStmts = saved
	}
	// A block is its own Go function: a begin around the call (or in the
	// method) must not route the block's value into the enclosing result.
	// A begin in the block keeps the value in the closure's own ret_.
	savedBegins, savedRetVar, savedWrap := f.begins, f.retVar, f.wrap
	f.begins, f.retVar, f.wrap = 0, "ret_", nil
	defer func() { f.begins, f.retVar, f.wrap = savedBegins, savedRetVar, savedWrap }()
	// probe the body's type if the block return has unbound vars
	ret := closed(sig.Ret, env)
	nilAcc := "" // `reduce(nil) { |acc, x| ... }`: A bound by a nil argument alone is what the block returns, or nil
	if tv, ok := sig.Ret.(TVar); ok && ret != nil && isNil(ret) {
		nilAcc, ret = tv.Name, nil
	}
	if ret == nil {
		var types []Type
		var lits []bool
		f.probe(func() {
			saved, savedRuby := f.enterRubyBlock(block, names.locals())
			f.closures++
			f.pushLoop(loopClosure)
			f.withClosureSelf(self, func() {
				_, pro := bindParams()
				pro()
				if c, ok := n.(*parser.CallNode); ok {
					f.openMemo(c, names)
				}
				f.redoLabel(body)
				f.withNextTail(tail{kind: tailReturn, types: &types, lits: &lits}, gen)
			})
			f.popLoop()
			f.closures--
			f.leaveRubyBlock(saved, savedRuby)
		})
		if f.bindBlockRet(n, sig, env, f.joinAllFrom(n, types, lits), nilAcc) {
			params = substAll(sig.Params, env) // the accumulator param is that type too
		}
		ret = subst(sig.Ret, env)
	}
	var b strings.Builder
	savedBuf := f.buf
	f.buf = &b
	saved, savedRuby := f.enterRubyBlock(block, names.locals())
	f.closures++
	f.pushLoop(loopClosure)
	savedSelfType, savedSelfCode := f.selfType, f.selfCode
	if self != nil {
		f.selfType, f.selfCode = self, "self"
	}
	ps, pro := bindParams()
	if self != nil {
		ps = append([]string{"self " + f.c.goType(self)}, ps...)
	}
	retS := ""
	if !isVoid(ret) {
		retS = " " + f.c.goType(ret)
		if containsRescue(body) {
			retS = " (ret_" + retS + ")"
		}
	}
	wrapped := f.closureHead(sig, env, ps, ret, retS)
	f.indent++
	pro()
	if c, ok := n.(*parser.CallNode); ok {
		f.openMemo(c, names)
	}
	f.hoistLocals(f.rbFrames[len(f.rbFrames)-1].key)
	f.redoLabel(body) // redo reruns the block with the same arguments: a closure is its own Go function (#52)
	savedRows := f.untypedRows
	if f.untypedRowBlock(n, ret) {
		f.untypedRows = f.closures
	}
	if isVoid(ret) {
		f.withNextTail(tail{}, gen)
	} else {
		f.withNextTail(tail{kind: tailReturn, typ: ret}, gen)
	}
	f.untypedRows = savedRows
	f.indent--
	if strings.Contains(b.String(), "\n//line ") {
		// the block's statements moved the line mapping; what follows it on the Go side is the call's line again. Only then: a block without its own directives (&:sym) would look to linters like it ends in blank lines.
		f.lineOf(n)
	}
	if wrapped {
		f.emit("}())")
		f.indent--
	}
	f.emit("}")
	f.selfType, f.selfCode = savedSelfType, savedSelfCode
	f.popLoop()
	f.closures--
	f.leaveRubyBlock(saved, savedRuby)
	f.buf = savedBuf
	code := strings.TrimSpace(b.String())
	return code
}

// bindClosureParams binds a closure's params as bindBlockParams does,
// returning each Go param with its type. A variadic lambda's (rest) last
// param is a Go `...E`, which its `*name` holds as an Array (decision 152).
func (f *fctx) bindClosureParams(n parser.Node, names blockParams, params []Type, rest bool) ([]string, func()) {
	if rest && len(params) == 1 && !names.hasRest() {
		return f.bindFromRest(names, params[0])
	}
	k, lead, extra := len(params), names, blockParams(nil)
	restName := ""
	if rest {
		// Ruby fills `|a, b|` over `(Integer, *String)` positionally: b is the rest's first element, not the rest Array (only a `*name` is). A block with fewer names than leading params binds the ones it has.
		k = len(params) - 1
		if names.hasRest() {
			restName, lead = names[len(names)-1].Name, names[:len(names)-1]
		}
		if len(lead) > k {
			lead, extra = lead[:k], lead[k:]
		}
		if extra.hasTuple() {
			f.errorf(n, "unsupported block parameter form: a destructuring parameter over a variadic block's rest")
		}
	}
	goParams, pro := f.bindBlockParams(n, lead, params[:k])
	ps := make([]string, 0, len(goParams)+1)
	for i, gp := range goParams {
		ps = append(ps, gp+" "+f.c.goType(params[i]))
	}
	if !rest {
		return ps, pro
	}
	rp, et := f.newTmp(), f.c.goType(params[k])
	ps = append(ps, rp+" ..."+et)
	return ps, func() {
		pro()
		for i, nm := range extra {
			f.bindBlockParam(nm.Name, expr{code: fmt.Sprintf("rbArgAt(%s, %d)", rp, i), typ: params[k]})
		}
		if restName == "" {
			f.emit("_ = %s", rp)
			return
		}
		v := f.blockParam(restName, TClass{C: f.c.classes["Array"], Args: []Type{params[k]}})
		if v.goName != "_" {
			f.emit("%s := &Array[%s]{s: %s[min(%d, len(%s)):]}", v.goName, et, rp, len(extra), rp)
		} else {
			f.emit("_ = %s", rp)
		}
		f.noteUnused(v)
	}
}

// bindFromRest binds `|a, b|` for a block the method calls as `{ (*T) -> R }`: each name is that argument, or nil
// (T's zero value) past the ones passed.
func (f *fctx) bindFromRest(names blockParams, et Type) ([]string, func()) {
	rp := f.newTmp()
	return []string{rp + " ..." + f.c.goType(et)}, func() {
		if _, untyped := et.(TAny); untyped && len(names) > 1 {
			f.emit("%s = rbAutoSplat(%s)", rp, rp) // a proc of two or more params splats a lone Array, as MRI's (decision 302)
		}
		for i, nm := range names {
			f.bindBlockParam(nm.Name, expr{code: fmt.Sprintf("rbArgAt(%s, %d)", rp, i), typ: et})
		}
		if len(names) == 0 {
			f.emit("_ = %s", rp)
		}
	}
}

// withNextTail runs gen(t) with t as the tail a bare `next` returns through.
func (f *fctx) withNextTail(t tail, gen func(tail)) {
	saved := f.nextTail
	f.nextTail = t
	gen(t)
	f.nextTail = saved
}

// iterClassTwin is an iterator call's twin for its one argument's class, as other calls take: (1.0..2.0).step(0.5) is __step_float (decision 299).
func (f *fctx) iterClassTwin(n *parser.CallNode, e *entry, recvT Type) *entry {
	args := callArgs(n)
	if len(args) != 1 || isSplat(args[0]) { // `each(*args, &block)`: the splat's class is not an argument's
		return nil
	}
	f.curBlock = n.Block
	o := f.overload(e, recvT, args)
	f.curBlock = nil
	if o != nil && o.M.Iterator && o.M.Name != "__"+overloadBase(e.M.Name)+"_1" {
		return o
	}
	return nil
}

// genTupleIterCall is `[1.0, 42].each { }`: it iterates a copy of the fields as an Array of their union (decision 366).
func (f *fctx) genTupleIterCall(n *parser.CallNode, tt TTuple, t tail) bool {
	et := joinAllUnion(tt.Elems)
	tmp := f.newTmp()
	recv := f.genExpr(n.Receiver, nil)
	f.emit("%s := %s", tmp, f.materialize(recv))
	parts := make([]string, len(tt.Elems))
	for i, ft := range tt.Elems {
		parts[i] = f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", tmp, i), typ: ft}, et)
	}
	at := TClass{C: f.c.classes["Array"], Args: []Type{et}}
	pinned := *n
	pinned.Receiver = &exprNode{Node: n.Receiver, e: expr{code: arrayLit(f.c.goType(et), parts), typ: at}}
	return f.genIterCall(&pinned, t)
}

// coreEachWanted is a user-code core each whose value is wanted: a typed slot, or a def's tail when a caller uses the
// def's value (decision 365).
func (f *fctx) coreEachWanted(m *Method, t tail) bool {
	if !coreEach(m) {
		return false
	}
	if t.typ != nil {
		return !isVoid(t.typ)
	}
	return t.kind == tailReturn && f.m != nil && f.c.callValueUsed(f.m.Name, false)
}

// genIterCall emits `for ... range recv.Each(...) { body }` for iterator
// methods called with a block. Returns false if the call is not one.
// A T? receiver is guarded first: `&.` skips the loop on nil, `.` raises.
func (f *fctx) genIterCall(n *parser.CallNode, t tail) bool {
	n = f.zipEachBlock(n, t)
	if cls := f.classRef(n.Receiver); cls != nil && cls.meta == nil {
		return false
	}
	var recvT Type
	var probed expr
	if n.Receiver == nil {
		recvT = f.selfType
	} else {
		f.probe(func() { probed = f.genExpr(n.Receiver, nil); recvT = probed.typ })
	}
	if u, ok := recvT.(TUnion); ok {
		if f.splitBlockForward(n, t) {
			return true
		}
		return f.genUnionIterCall(n, u, t)
	}
	if tt, ok := recvT.(TTuple); ok && t.kind == tailNone && f.c.classes["Array"].lookup(n.Name) != nil {
		return f.genTupleIterCall(n, tt, t)
	}
	opt, isOptRecv := recvT.(TOpt)
	if isOptRecv {
		recvT = opt.Elem
	}
	e := f.resolveIter(n, recvT)
	if e == nil || !e.M.Iterator {
		return false
	}
	e = f.iterTwin(n, e, recvT, probed)
	// a gem's `xs.each(&blk)` as a value is xs, as MRI's each returns its receiver (decision 172)
	selfValue := t.kind != tailNone && eachReturnsSelf[n.Name] && !isOptRecv && (f.lenient() || f.coreEachWanted(e.M, t))
	if !selfValue && t.kind != tailNone && t.typ != nil && !isVoid(t.typ) && (!isAny(t.typ) || n.Name != "loop" || !kernelRecv(n)) && !f.lenient() { // loopRescue's begin: a loop that ends without StopIteration was broken out of, nil
		f.errorf(n, "the value of an iterator call (%s) cannot be used", n.Name) // gem code: nil, a broken-out loop's (decision 233)
	}
	var recv expr
	if n.Receiver == nil {
		recv = expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}
	} else {
		recv = f.genExpr(n.Receiver, nil)
	}
	if selfValue {
		f.genSelfValueIter(n, e, recv, t)
		return true
	}
	safe := isOptRecv && n.IsSAFE_NAVIGATION()
	switch {
	case safe:
		rt := f.newTmp()
		f.emit("if %s := %s; %s {", rt, recv.code, optNotNil(rt, opt))
		f.indent++
		recv = expr{code: derefOpt(rt, opt), typ: opt.Elem}
	case isOptRecv:
		recv = f.nilGuard(n, recv, n.Name)
	}
	valued := f.kernelLoopValue(n, t)
	f.iterValue = valued
	f.genIterLoop(n, e, recv)
	if safe {
		f.indent--
		f.emit("}")
	}
	f.iterEmptyTail(n, t, valued)
	return true
}

// genSelfValueIter is an iterator call whose value is its receiver, as MRI's each returns it (decision 172).
func (f *fctx) genSelfValueIter(n *parser.CallNode, e *entry, recv expr, t tail) {
	tmp := f.newTmp()
	f.emit("var %s %s = %s", tmp, f.c.goType(recv.typ), recv.code) // typed: a literal receiver (`4.times`) is an Integer, not Go's int
	recv.code = tmp
	if b, ok := n.Block.(*parser.BlockNode); f.lenient() && (f.iterBreakValue || ok && b.Body != nil && t.kind != tailNone && blockBreaksValue(b.Body)) {
		f.iterBreakValue = false
		f.genBrokenIterLoop(n, e, recv, t)
		return
	}
	f.genIterLoop(n, e, recv)
	f.applyTail(n, recv, t)
}

// genBrokenIterLoop is a gem's `r.times do ... break ... end` as a value (prime's miller_rabin_test): the receiver when
// the loop runs to its end, nil when a break leaves it, as MRI's; the break is a goto past the receiver's assignment.
func (f *fctx) genBrokenIterLoop(n *parser.CallNode, e *entry, recv expr, t tail) {
	res := f.newTmp()
	f.emit("var %s any", res) // above the loop: the goto may not jump over a declaration
	bt := &brokeTarget{label: "broke" + res, res: res}
	f.iterBroke = bt
	f.genIterLoop(n, e, recv)
	f.iterBroke = nil
	f.emit("%s = %s", res, f.coerce(n, recv, TAny{}))
	if bt.used {
		f.emit("%s:", bt.label)
	}
	f.applyTail(n, expr{code: res, typ: TAny{}}, t)
}

// iterTwin is the twin an iterator call takes: literal options (CSV.foreach(path, headers: true), decision 117), its
// argument's class, or a binary receiver's (`bin.each_char { }`, decision 352).
func (f *fctx) iterTwin(n *parser.CallNode, e *entry, recvT Type, probed expr) *entry {
	if rc, ok := recvT.(TClass); ok {
		if o := f.literalOverload(e.M, rc.C, callArgs(n)); o != nil && o.M.Iterator {
			e = o
		} else if o := f.iterClassTwin(n, e, recvT); o != nil {
			e = o
		}
	}
	if probed.bin {
		if o := f.binTwin(e.M, probed, callArgs(n)); o != nil && o.M.Iterator {
			e = o
		}
	}
	if o := f.windowViewIter(n, e, recvT); o != nil && o.M.Iterator {
		e = o
	}
	return e
}

// resolveIter is the method an iterator call names: its zero-argument twin for a call with none, Kernel's for a gem
// module's own `loop do ... end`.
func (f *fctx) resolveIter(n *parser.CallNode, recvT Type) *entry {
	e := f.resolve(recvT, n.Name)
	if e != nil && len(callArgs(n)) == 0 && slices.ContainsFunc(e.M.Params, func(p Param) bool { return p.Default == nil && !p.Rest && !p.Keyword && !p.KwRest }) {
		if z := f.resolve(recvT, "__"+n.Name+"_0"); z != nil && z.M.Iterator { // decision 12's arity twin (`0.step do ... end`)
			return z
		}
	}
	if e == nil && n.Receiver == nil && f.owner != nil && f.owner.IsModule && f.lenient() {
		e = f.c.classes["Object"].lookup(n.Name)
	}
	if e != nil && e.M.Block == nil {
		if b := f.resolve(recvT, "__"+overloadBase(n.Name)+"_block"); b != nil && b.M.Iterator { // `xs.zip(ys) { }` (decision 389)
			return b
		}
	}
	return e
}

// untypedRowBlock is a gem's `Array.new(n) { ... }` whose elements are untyped (matrix's `@v = Array.new(@size) {
// Array.new(@size, 0) }`, later written with Floats): each row the block builds is an Array[untyped] (untypedRow).
func (f *fctx) untypedRowBlock(n parser.Node, ret Type) bool {
	c, ok := n.(*parser.CallNode)
	if !ok || !f.lenient() || !isAny(ret) || c.Name != "new" {
		return false
	}
	r, ok := c.Receiver.(*parser.ConstantReadNode)
	return ok && r.Name == "Array"
}

// untypedRow converts a fresh Array the untypedRowBlock returns into an Array[untyped], as MRI's row holds any value:
// an Array[Integer] row took `row[j] = 0.82` through the untyped view as a TypeError (decision 420). Fresh, so the copy
// aliases nothing.
func (f *fctx) untypedRow(n parser.Node, e expr, t tail) expr {
	if f.untypedRows == 0 || f.untypedRows != f.closures || !isAny(t.typ) {
		return e
	}
	at, ok := e.typ.(TClass)
	if !ok || at.C != f.c.classes["Array"] || len(at.Args) != 1 || isAny(at.Args[0]) {
		return e
	}
	switch x := n.(type) {
	case *parser.ArrayNode:
	case *parser.CallNode:
		if r, ok := x.Receiver.(*parser.ConstantReadNode); !ok || r.Name != "Array" || x.Name != "new" {
			return e
		}
	default:
		return e
	}
	return expr{code: fmt.Sprintf("rbAs[*Array[any]](any(%s), \"Array[untyped]\")", e.code), typ: TClass{C: at.C, Args: []Type{TAny{}}}}
}

// splitBlockForward is a gem's `generator.each(&block)` as its method's value, on a union, passing on the method's own
// optional block (prime's Prime#each): without one the call takes each member's block-less form (a `__each_enum` twin,
// decision 418), with one the loop runs as before; a union's iterator call has no value of its own.
func (f *fctx) splitBlockForward(n *parser.CallNode, t tail) bool {
	ba, ok := n.Block.(*parser.BlockArgumentNode)
	if !ok || !f.lenient() || t.kind == tailNone || f.splittingForward || f.m == nil || f.m.Block == nil || !f.m.Block.Optional && !f.m.Block.Untyped {
		return false
	}
	lv, ok := ba.Expression.(*parser.LocalVariableReadNode)
	if !ok || f.m.Node == nil || f.m.Node.Parameters == nil || f.m.Node.Parameters.Block == nil || f.m.Node.Parameters.Block.Name == nil || *f.m.Node.Parameters.Block.Name != lv.Name {
		return false
	}
	blk := f.genExpr(lv, nil)
	f.emit("if %s == nil {", blk.code)
	f.indent++
	without := *n
	without.Block = nil
	f.genStmt(&without, t)
	f.indent--
	f.emit("} else {")
	f.indent++
	f.splittingForward = true
	f.genStmt(n, tail{})
	f.splittingForward = false
	f.emptyTail(n, t) // a gem iterator call's value: nil (decision 233)
	f.indent--
	f.emit("}")
	return true
}

// genUnionIterCall is genIterCall on a union (decision 150): a type switch
// whose arms loop over the members whose method is an iterator and call
// it on the others, as unionCall does. False when no member's is one.
func (f *fctx) genUnionIterCall(n *parser.CallNode, u TUnion, t tail) bool {
	entries := make([]*entry, len(u.Members))
	found := false
	for i, m := range u.Members {
		if e := f.resolve(m, n.Name); e != nil && e.M.Iterator && !isNil(m) {
			entries[i], found = e, true
		}
	}
	if !found {
		return false
	}
	if t.kind != tailNone && t.typ != nil && !isVoid(t.typ) {
		f.errorf(n, "the value of an iterator call (%s) cannot be used", n.Name)
	}
	recv := f.genExpr(n.Receiver, nil)
	sw := f.newTmp()
	f.emit("switch %s := %s.(type) {", sw, recv.code)
	f.switches++
	for i, m := range u.Members {
		goT := "nil"
		if !isNil(m) {
			goT = f.c.goType(m)
		}
		f.emit("case %s:", goT)
		saved := f.enterBlock()
		f.indent++
		switch {
		case isNil(m):
			f.warn(n, "%s called on %s, which may be nil (raises NoMethodError on nil)", n.Name, u)
			f.emit("panic(rbNoMethod(%q, nil, false))", n.Name)
		case entries[i] != nil:
			f.genIterLoop(n, entries[i], expr{code: sw, typ: m})
		case f.lenient() && f.resolve(m, n.Name) == nil && f.c.isDelegator(classOf(m)): // a Delegator forwards the loop to its object (Homebrew's Requirements)
			obj := f.genMethodCall(n, expr{code: sw, typ: m}, "__getobj__", nil, nil)
			f.genStmt(&parser.CallNode{Location: n.Location, Receiver: &exprNode{Node: n, e: obj}, Name: n.Name, Arguments: n.Arguments, Block: n.Block}, tail{})
		case f.lenient() && f.resolve(m, n.Name) == nil: // a gem's member without the method: NoMethodError if taken, as MRI's
			f.emit("panic(rbNoMethod(%q, %s, false))", n.Name, sw)
		default:
			f.emitExprStmt(n, f.genMethodCall(n, expr{code: sw, typ: m}, n.Name, callArgs(n), n.Block))
		}
		f.indent--
		f.leaveBlock(saved)
	}
	f.switches--
	f.emit("}")
	if t.kind != tailNone {
		f.emptyTail(n, t)
	}
	return true
}

// genFor is `for x in coll`: coll.each as a range loop whose variables and
// body locals stay in the enclosing scope, since Ruby's for opens no block.
func (f *fctx) genFor(n *parser.ForNode, t tail) {
	recvT := f.probeType(n.Collection, nil)
	e := f.resolve(recvT, "each")
	if e == nil || !e.M.Iterator {
		f.errorf(n.Collection, "for needs a collection whose each is an iterator (Array, Range, Hash, Set, ...), not %s", recvT)
	}
	recv := f.genExpr(n.Collection, nil)
	env := receiverEnv(e, recv)
	yields := substAll(e.M.Block.Params, env)
	call := f.callCode(e, recv, nil, env)
	tmps := make([]string, len(yields))
	for i := range tmps {
		tmps[i] = f.newTmp()
	}
	f.pushLoop(loopIter)
	saved := f.enterBlock()
	if len(tmps) == 0 {
		f.emit("for range %s {", call)
	} else {
		f.emit("%s", rangeHead(strings.Join(tmps, ", "), call))
	}
	f.indent++
	vals := make([]expr, len(yields))
	for i, y := range yields {
		vals[i] = expr{code: tmps[i], typ: y}
	}
	switch ix := n.Index.(type) {
	case *parser.LocalVariableTargetNode:
		if len(vals) != 1 {
			f.errorf(ix, "for with one variable over a %d-value each", len(vals))
		}
		f.assignLocal(ix, ix.Name, vals[0], nil)
	case *parser.MultiTargetNode:
		if len(vals) == 1 {
			f.genMultiWrite(&parser.MultiWriteNode{Location: ix.Location, Lefts: ix.Lefts, Rest: ix.Rest, Rights: ix.Rights, Value: &exprNode{Node: ix, e: vals[0]}})
			break
		}
		if ix.Rest != nil || len(ix.Rights) > 0 || len(ix.Lefts) != len(vals) {
			f.errorf(ix, "for needs %d variables here", len(vals))
		}
		for i, l := range ix.Lefts {
			lt, ok := l.(*parser.LocalVariableTargetNode)
			if !ok {
				f.errorf(l, "unsupported for variable %s", nodeType(l))
			}
			f.assignLocal(lt, lt.Name, vals[i], nil)
		}
	default:
		f.errorf(n.Index, "unsupported for variable %s", nodeType(n.Index))
	}
	f.redoLabel(n.Statements)
	f.genStmts(n.Statements, tail{})
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.popLoop()
	f.emptyTail(n, t)
}

// iterEmptyTail is an iterator call's value when the loop ends: nil, unless a valued loop's assignment already started it so.
func (f *fctx) iterEmptyTail(n parser.Node, t tail, valued *tail) {
	if t.kind != tailNone && (valued == nil || t.kind != tailAssign) {
		f.emptyTail(n, t)
	}
}

// kernelLoopValue is the tail a gem's `loop do ... break v ... end` in value position puts v in, or nil (prime's each,
// decision 410), as a valued while's (decision 364).
func (f *fctx) kernelLoopValue(n *parser.CallNode, t tail) *tail {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok || n.Name != "loop" || !kernelRecv(n) || !f.lenient() {
		return nil
	}
	st, ok := blk.Body.(*parser.StatementsNode)
	if !ok {
		return nil
	}
	return f.loopValue(st, t)
}

// genIterLoop emits the range loop of iterator entry e on recv.
func (f *fctx) genIterLoop(n *parser.CallNode, e *entry, recv expr) {
	e, recv, args := f.numericIterArgs(e, recv, callArgs(n))
	m := e.M
	env := receiverEnv(e, recv)
	codes, _ := f.genArgs(n, m, env, args, nil)
	yields := substAll(m.Block.Params, env)
	call := f.callCode(e, recv, codes, env)
	zipVal := false
	if len(m.Block.Params) == 1 { // zip's `{ ([E, U?]) -> void }`: generic Go yields Tuple2[E, *U] (decision 396)
		if p, ok := m.Block.Params[0].(TTuple); ok && len(p.Elems) == 2 && !isOpt(p.Elems[0]) && boxedValOpt(p.Elems[1], env) {
			call = "rbZipValSeq(" + call + ")"
			zipVal = true
		}
	}
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		f.forwardIter(n, call, yields)
		return
	}
	names := f.blockParamList(blk.Parameters)
	saved, savedRuby := f.enterRubyBlock(blk, names.locals())
	f.viewSplat = f.windowSplat(blk)
	goParams, pro := f.bindBlockParams(n, names, yields)
	f.viewSplat = false
	allBlank := true
	for _, gp := range goParams {
		if gp != "_" {
			allBlank = false
		}
	}
	f.zipVal = zipVal
	head := f.countedLoopHead(n, e, recv, codes, names, goParams) // its temps go above the label pushLoop places
	f.zipVal = false
	f.pushLoop(loopIter)
	f.loops[len(f.loops)-1].value, f.iterValue = f.iterValue, nil
	f.loops[len(f.loops)-1].broke, f.iterBroke = f.iterBroke, nil
	switch {
	case head != "":
		f.emit("%s", head)
	case allBlank:
		f.emit("for range %s {", call)
	default:
		f.emit("%s", rangeHead(strings.Join(goParams, ", "), call))
	}
	f.indent++
	pro()
	f.hoistLocals(f.rbFrames[len(f.rbFrames)-1].key)
	f.redoLabel(blk.Body)
	f.genStmts(blk.Body, tail{})
	f.indent--
	f.leaveRubyBlock(saved, savedRuby)
	f.emit("}")
	f.popLoop()
}

// genYield compiles `yield`: a direct call of the block, or a Proc call when the method's block is optional.
func (f *fctx) genYield(n *parser.YieldNode) expr {
	args := []parser.Node{}
	if n.Arguments != nil {
		args = n.Arguments.Arguments
	}
	if name := f.m.optionalBlockLocal(); name != "" {
		if f.c.round && f.blockSig != nil { // an optional block's yield is a Proc call: record its values here
			for i, a := range args {
				f.probe(func() { _ = catchCompileError(func() { f.noteYield(i, a, f.genExpr(a, nil).typ) }) })
			}
		}
		if v := f.visibleLocal(name); v != nil && isOpt(v.typ) {
			return f.yieldMaybeMissing(n, v, args)
		}
		call := f.c.rewrite(n, func() parser.Node {
			return &parser.CallNode{Location: n.Location, Receiver: &parser.LocalVariableReadNode{Name: name, Location: n.Location}, Name: "call", Arguments: n.Arguments}
		})
		return f.genExpr(call, nil)
	}
	return f.yieldValues(n, args)
}

// genBlockCall is `blk.call(args)` on the method's own &block: a direct
// call of a required block; an optional one (a Proc? local) must be present.
func (f *fctx) genBlockCall(n *parser.CallNode) (expr, bool) {
	if n.Name != "call" || n.Block != nil {
		return expr{}, false
	}
	if name := f.m.optionalBlockLocal(); name != "" {
		if lv, ok := n.Receiver.(*parser.LocalVariableReadNode); ok && lv.Name == name && !n.IsSAFE_NAVIGATION() {
			if v := f.visibleLocal(name); v != nil && isOpt(v.typ) && f.m.Block != nil && f.m.Block.Untyped {
				// an unannotated def's block: missing, `blk.call` is MRI's NoMethodError on nil; past the check it is present
				f.emit("if %s == nil {", v.goName)
				f.emit("	panic(rbNoMethod(\"call\", nil, false))")
				f.emit("}")
				f.push()
				defer f.pop()
				f.applyNarrow([]narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}})
				return f.genExpr(n, nil), true
			}
			f.checkBlockPresent(n, name)
		}
		return expr{}, false
	}
	if f.isBlockParam(n.Receiver) {
		return f.yieldValues(n, callArgs(n)), true
	}
	return expr{}, false
}

// yieldMaybeMissing is `yield args` where the optional block may be
// missing: MRI evaluates the arguments, then raises LocalJumpError (decision
// 132); past the check the block is narrowed to present.
func (f *fctx) yieldMaybeMissing(n *parser.YieldNode, v *local, args []parser.Node) expr {
	params := v.typ.(TOpt).Elem.(TFunc).Params
	nodes := make([]parser.Node, len(args))
	for i, a := range args {
		nodes[i] = a
		if _, splat := a.(*parser.SplatNode); splat || i >= len(params) {
			continue
		}
		e := f.genExpr(a, params[i])
		if !e.lit && !isSimpleGo(e.code) {
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, e.code)
			e.code = tmp
		}
		nodes[i] = &exprNode{Node: a, e: e}
	}
	const msg = "no block given (yield)"
	f.c.strLits[msg] = true
	exc := f.withCause(f.genNew(n, f.c.classes["LocalJumpError"], nil, []expr{{code: strconv.Quote(msg), typ: f.cls("String"), lit: true}}, nil, nil).code)
	f.emit("if %s == nil {", v.goName)
	f.emit("\tpanic(%s)", exc)
	f.emit("}")
	f.push()
	defer f.pop()
	f.applyNarrow([]narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}})
	var argsNode *parser.ArgumentsNode
	if n.Arguments != nil {
		argsNode = &parser.ArgumentsNode{Location: n.Arguments.Location, Arguments: nodes}
	}
	return f.genExpr(&parser.CallNode{Location: n.Location, Receiver: &parser.LocalVariableReadNode{Name: v.name, Location: n.Location}, Name: "call", Arguments: argsNode}, nil)
}

// checkBlockPresent rejects `blk.call` on an optional block (`?{ }`) where it
// may be missing: MRI's NoMethodError on nil, at compile time.
func (f *fctx) checkBlockPresent(n parser.Node, name string) {
	if v := f.visibleLocal(name); v != nil && isOpt(v.typ) {
		f.errorf(n, "the block is optional (?{ ... }) and may be missing here: check block_given? first, or call it with &.call")
	}
}

// yieldValues is `yield args` and `block.call(args)`.
func (f *fctx) yieldValues(n parser.Node, args []parser.Node) expr {
	if f.blockSig == nil && f.m != nil && f.m.yieldDisagree != "" {
		f.errorf(f.m.yieldDisagreeAt, "%s", f.m.yieldDisagree)
	}
	if f.blockSig == nil {
		f.errorf(n, "yield in a method whose signature has no block")
	}
	if f.blockSig.Rest && !f.iterator {
		return expr{code: "blk(" + f.restArgs(n, args, f.blockSig.Params) + ")", typ: f.blockSig.Ret}
	}
	codes := make([]string, 0, len(args))
	if spread, ok := f.anySpread(args); ok && f.lenient() { // a gem's `yield(*args)`: each block parameter its position, nil past the end
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, spread)
		for i, want := range f.blockSig.Params {
			codes = append(codes, f.coerce(n, expr{code: fmt.Sprintf("rbArgAt(%s, %d)", tmp, i), typ: TAny{}}, want))
		}
		args = nil
	} else if len(args) != len(f.blockSig.Params) {
		f.errorf(n, "yield passes %d values but the block takes %d", len(args), len(f.blockSig.Params))
	}
	for i, a := range args {
		want := f.blockSig.Params[i]
		if f.c.round && isAny(want) && i < len(f.blockSig.Pending) && f.blockSig.Pending[i] != "" {
			want = nil
		}
		e := f.genExpr(a, want)
		f.noteYield(i, a, e.typ)
		codes = append(codes, f.coerce(a, e, f.blockSig.Params[i]))
	}
	if f.iterator && f.closures > 0 { // inside a block the method passes on (`@lock.synchronize { yield x }`): it runs during the
		// iteration, so Go's yield may be called, but a consumer's break cannot return through it (decision 226)
		f.emit("if !yield(%s) {", strings.Join(codes, ", "))
		f.emit("\tpanic(rbIterBreak{})")
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	if f.iterator {
		f.emit("if !yield(%s) {", strings.Join(codes, ", "))
		f.indent++
		f.emitReturn()
		f.indent--
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	if f.blockSig.Self != nil { // an escaped block: its self is rebound by instance_eval, so a lexical call is a compile error (decision 164)
		if f.blockSig.SelfPending != "" {
			f.noteUse(Param{Pending: f.blockSig.SelfPending}, n, f.concreteSelf(f.selfType))
		}
		f.errorf(n, "a block stored from &%s cannot also be called here (yield/blk.call): its self is rebound by instance_eval", f.m.BlockParam)
	}
	return expr{code: "blk(" + strings.Join(codes, ", ") + ")", typ: f.blockSig.Ret}
}

// blockParam declares a block parameter: a fresh local that Go syntax
// declares, so it is never hoisted.
func (f *fctx) blockParam(name string, typ Type) *local {
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{}
		f.locals[key] = info
	}
	if isAny(typ) && f.blockWhy != "" {
		f.setAnyOrigin(name, f.blockWhy)
	}
	info.noHoist = true
	f.declBin = f.binParams && isClass(typ, "String")
	v := f.declareLocal(name, typ)
	if f.declBin && f.pass < 2 {
		info.bin = true
	}
	f.declBin = false
	v.declared = true
	return v
}

// bindBlockRet binds sig's return variables from got, the probed block's type; true when a nil-bound accumulator
// (nilAcc, `reduce(nil)`) took it, so the block's params change too (decision 257).
func (f *fctx) bindBlockRet(n parser.Node, sig *BlockSig, env map[string]Type, got Type, nilAcc string) bool {
	if got == nil || isNil(got) {
		got = TNil{}
		if _, ok := sig.Ret.(TVar); ok {
			got = TAny{} // Go has no nil type
		}
	}
	// the probe typed `[k, v]` without the tuple it is wanted as: a
	// same-typed pair reads as Array[X], which the real pass builds as [X, X]
	if tt, ok := sig.Ret.(TTuple); ok {
		if ga, ok := got.(TClass); ok && ga.C.RubyName == "Array" && len(ga.Args) == 1 {
			elems := make([]Type, len(tt.Elems))
			for i := range elems {
				elems[i] = ga.Args[0]
			}
			got = TTuple{Elems: elems}
		}
	}
	if sa, ok := sig.Ret.(TClass); ok && sa.C.RubyName == "Array" && len(sa.Args) == 1 {
		// the other way: a pair where an Array is wanted (`flat_map { |a| [a, [v]] }`, rubocop's SpecialGlobalVars) is an
		// Array of what its elements join to, untyped when they are of different classes (decisions 22, 444)
		if gt, ok := got.(TTuple); ok {
			got = TClass{C: sa.C, Args: []Type{joinOrAny(gt.Elems)}}
		}
	}
	if nilAcc != "" {
		if j, ok := join(TNil{}, got); ok && !isNil(j) {
			env[nilAcc] = j
			return true
		}
	} else if !unify(sig.Ret, got, env) && !f.lenientBlockRet(sig.Ret, got, env) {
		f.errorf(n, "block returns %s, expected %s", got, subst(sig.Ret, env))
	}
	return false
}
