package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// floatTruncators are core classes whose C methods take an Integer by NUM2LONG; numeric classes, Range, Kernel and
// Random take a Float as a Float instead (`2.pow(0.5)`, `rand(1.5)`).
var floatTruncators = map[string]bool{"String": true, "Array": true, "Symbol": true, "Hash": true, "Enumerable": true, "IO": true, "File": true, "StringIO": true, "MatchData": true, "Regexp": true, "Struct": true}

func truncatesFloats(m *Method) bool {
	return m.File != nil && m.File.isPrelude() && m.Owner != nil && floatTruncators[m.Owner.RubyName]
}

// argcOr is argc, or under an adapter passing its own arguments on the count its caller gave, not how many it names
// (decision 444).
func (f *fctx) argcOr(argc string) string {
	if f.argcPass != "" {
		return f.argcPass
	}
	return argc
}

// genArgs generates and coerces call arguments against m's parameters,
// binding type variables in env. exprs, if non-nil, are pre-generated.
// genArgs also returns where *rest's codes start (-1 if none), so a block arg can splice in before them, not after.
func (f *fctx) genArgs(n parser.Node, m *Method, env map[string]Type, args []parser.Node, exprs []expr) ([]string, int) {
	var codes []string
	restIdx := -1
	call := nilCall{m: m, recv: f.nilRecv}
	f.nilRecv = nil
	kw := &kwArgs{named: map[string]*parser.AssocNode{}, used: map[string]bool{}}
	if exprs == nil && m.hasKeywords() {
		args, kw = f.splitKeywordArgs(m, args)
	}
	nargs := len(args)
	if exprs != nil {
		nargs = len(exprs)
	}
	npos := m.positionalCount()
	hasRest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest })
	expected := npos // what the messages have always counted: positional params, *rest as one
	if hasRest {
		expected++
	}
	if exprs == nil && hasRest {
		args = f.splatOverRest(m, args)
		nargs = len(args)
	}
	splatTmp, splatAt, splatElem := f.splatArgs(m, args, exprs != nil || hasRest, npos)
	if splatTmp != "" {
		args, nargs = args[:splatAt], splatAt
	}
	if exprs == nil && !hasRest && nargs-1 == npos && f.emptySplat(args) {
		args, nargs = args[:len(args)-1], nargs-1
	}
	// Too many arguments is checked first: it is the error Ruby raises, and
	// the surplus would otherwise be coerced to the wrong parameter's type.
	if nargs > npos && !hasRest {
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, expected)
	}
	if exprs == nil {
		f.optJoin(m, env, args)
	}
	ai := 0
	npost := m.postCount() // post params (`def f(a, *r, z)`) take the last arguments
	posts := 0
	take := func(p Param, i int) {
		an, a := f.argAt(n, p, env, args, exprs, i, codes)
		f.noteUse(p, an, a.typ)
		f.noteWideningUse(m, p, an, a.typ)
		a = f.truncFloat(m, p, env, a)
		unify(p.Type, a.typ, env)
		f.bindGemUnbound(m, env, p.Type)
		prev := f.nilArg
		f.nilArg = nilCall{m: call.m, recv: call.recv, param: p.Type}
		defer func() { f.nilArg = prev }()
		codes = append(codes, f.coerceArg(an, a, p, env))
	}
	for _, p := range m.Params {
		if p.Post {
			i := nargs - npost + posts
			if i < ai {
				f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, expected)
			}
			take(p, i)
			posts++
			continue
		}
		if p.Keyword {
			mark := f.buf.Len()
			code := f.keywordArgBit(n, m, p, env, kw)
			f.pinBefore(mark, codes)
			codes = append(codes, code)
			continue
		}
		if p.Rest {
			restIdx = len(codes)
			mark := f.buf.Len()
			rest := f.genRestArgs(n, p, env, args, exprs, ai, max(nargs-npost, ai))
			f.pinBefore(mark, codes)
			codes = append(codes, rest...)
			ai = nargs
			continue
		}
		if ai < nargs-npost {
			take(p, ai)
			ai++
			continue
		}
		if splatTmp != "" {
			codes = append(codes, f.splatParam(n, m, p, env, splatTmp, ai-splatAt, splatElem))
			ai++
			continue
		}
		if p.Default != nil && m.calleeDefaults {
			codes = append(codes, "rbZero["+f.c.goType(subst(p.Type, env))+"]()")
			continue
		}
		if p.Default != nil {
			codes = append(codes, f.defaultArg(n, m, p, env))
			continue
		}
		f.errorf(n, "%s: wrong number of arguments (given %d, expected %d)", m, nargs, expected)
	}
	kw.checkUnknown(f, m)
	if m.kwMask() {
		codes = append([]string{strings.Join(append([]string{strconv.Itoa(kw.mask)}, kw.runtime...), " | ")}, codes...)
		if restIdx >= 0 {
			restIdx++
		}
	}
	if m.calleeDefaults { // optional params at Go index i ran their default when rbArgc <= i
		codes = append([]string{f.argcOr(argcCode(nargs, npos, npost, splatTmp, splatAt))}, codes...)
		if restIdx >= 0 {
			restIdx++
		}
	}
	return codes, restIdx
}

// truncFloat is a Float argument where a core method takes an Integer: MRI's NUM2LONG truncates it (decision 181). A
// param typed by the receiver's element (`[]=`'s E of an Array[Integer]) stores the Float, as MRI does (decision 414).
func (f *fctx) truncFloat(m *Method, p Param, _ map[string]Type, a expr) expr {
	if truncatesFloats(m) && isClass(a.typ, "Float") && isClass(p.Type, "Integer") {
		return expr{code: "rbNum2Long(" + a.code + ")", typ: f.cls("Integer")}
	}
	return a
}

// argcCode is the argument count a callee-defaults method sees: a splat's is its real length (decision 184).
func argcCode(nargs, npos, npost int, splatTmp string, splatAt int) string {
	if splatTmp != "" {
		return fmt.Sprintf("rbMinInt(%d+len(%s.s), %d)", splatAt, splatTmp, npos)
	}
	return strconv.Itoa(max(min(nargs, npos)-npost, 0))
}

// splatOverRest is a gem's trailing `*xs` that reaches positional parameters before a rest one (bindata's
// `DSLFieldParser.new(namespace, hints, *args)` into `initialize(namespace, hints, symbol, *args)`): xs's first elements
// fill those parameters and the rest of it splats into the rest, as MRI spreads it.
func (f *fctx) splatOverRest(m *Method, args []parser.Node) []parser.Node {
	if !f.lenient() || len(args) == 0 || m.postCount() != 0 {
		return args
	}
	sp, ok := args[len(args)-1].(*parser.SplatNode)
	if !ok || sp.Expression == nil || slices.ContainsFunc(args[:len(args)-1], isSplat) {
		return args
	}
	pre := 0 // positional parameters before the rest
	for _, p := range m.Params {
		if p.Rest {
			break
		}
		if !p.Keyword {
			pre++
		}
	}
	before := len(args) - 1
	if before >= pre {
		return args
	}
	a := f.genExpr(sp.Expression, nil)
	tc, ok := a.typ.(TClass)
	if !ok || tc.C.RubyName != "Array" || len(tc.Args) != 1 {
		a = expr{code: "&Array[any]{s: rbSplatObj(" + f.coerce(sp, a, TAny{}) + ")}", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		tc = a.typ.(TClass) //nolint:forcetypeassert // built just above
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, a.code)
	out := slices.Clone(args[:before])
	for i := range pre - before {
		out = append(out, &exprNode{Node: sp, e: expr{code: fmt.Sprintf("rbSplatElem(%s, %d)", tmp, i), typ: tc.Args[0]}})
	}
	rest := expr{code: fmt.Sprintf("rbMidSplat(%s, %d, 0)", tmp, pre-before), typ: tc}
	return append(out, &parser.SplatNode{Location: sp.Location, Expression: &exprNode{Node: sp.Expression, e: rest}})
}

// splatArgs is `f(a, *xs)` into positional parameters (decision 184): xs's length is known only at run time, so the
// arity is checked there (MRI's ArgumentError); it answers xs's temp, the splat's index and its element type, or "".
func (f *fctx) splatArgs(m *Method, args []parser.Node, skip bool, npos int) (string, int, Type) {
	if skip || len(args) == 0 || m.postCount() != 0 || m.hasKeywords() && !f.lenient() { // gem code: keywords were split off already
		return "", -1, nil
	}
	sp, ok := args[len(args)-1].(*parser.SplatNode)
	if !ok || sp.Expression == nil || slices.ContainsFunc(args[:len(args)-1], isSplat) {
		return "", -1, nil
	}
	a := f.genExpr(sp.Expression, nil)
	code := a.code
	var elem Type
	switch t := a.typ.(type) {
	case TAny: // MRI's splat: an Array spreads, nil is nothing, anything else is one argument (bindata's `*args` of a Symbol)
		code, elem = "&Array[any]{s: rbSplatObj("+a.code+")}", TAny{}
	case TClass:
		if t.C.RubyName != "Array" || len(t.Args) != 1 {
			f.errorf(sp, "splat of %s into a non-rest parameter: only an Array is supported", a.typ)
		}
		elem = t.Args[0]
	default:
		f.errorf(sp, "splat of %s into a non-rest parameter: only an Array is supported", a.typ)
	}
	at, tmp := len(args)-1, f.newTmp()
	f.emit("%s := %s", tmp, code)
	f.emit("rbSplatArity(%d+len(%s.s), %d, %d)", at, tmp, requiredArgs(m), npos)
	return tmp, at, elem
}

// splatParam is parameter p's argument from element j of a splat's temp: the element when the array has one, else
// (for an optional parameter whose default the caller evaluates) the default; a callee-side default sees the argc.
func (f *fctx) splatParam(n parser.Node, m *Method, p Param, env map[string]Type, tmp string, j int, elem Type) string {
	got := f.coerceArg(n, expr{code: fmt.Sprintf("rbSplatElem(%s, %d)", tmp, j), typ: elem}, p, env)
	if p.Default == nil || m.calleeDefaults {
		return got
	}
	def := f.coerceArg(n, f.defaultExpr(m, p, env), p, env)
	return fmt.Sprintf("func() %s {\n\tif len(%s.s) > %d {\n\t\treturn %s\n\t}\n\treturn %s\n}()", f.c.goType(closed(p.Type, env)), tmp, j, got, def)
}

// kwArgs is a call's `name: value` arguments, matched to keyword parameters by name.
type kwArgs struct {
	named  map[string]*parser.AssocNode
	order  []*parser.AssocNode
	splats []parser.Node // `**h`
	all    []parser.Node // both, in source order: `**opts` keeps it
	used   map[string]bool
	// `f(**h)` into named keywords: h's temp, and the Go bool saying the
	// keyword just rendered came from it (its rbKw bit is then run time)
	splatTmp, given string
	mask, bit       int      // rbKw: the optional keywords passed, by position
	runtime         []string // rbKw bits a `**h` decides at run time
}

// keywordArgBit is keywordArg plus p's rbKw bit.
func (f *fctx) keywordArgBit(n parser.Node, m *Method, p Param, env map[string]Type, kw *kwArgs) string {
	code, given := f.keywordArg(n, m, p, env, kw)
	if given {
		kw.mask |= 1 << kw.bit
	}
	if kw.given != "" { // from **h: known at run time
		kw.runtime = append(kw.runtime, fmt.Sprintf("rbKwBit(%s, %d)", kw.given, kw.bit))
		kw.given = ""
	}
	kw.bit++
	return code
}

// splitKeywordArgs takes a trailing `k: v, **h` off args when m has keyword parameters (a braced Hash stays positional, as in Ruby 3).
func (f *fctx) splitKeywordArgs(m *Method, args []parser.Node) ([]parser.Node, *kwArgs) {
	kw := &kwArgs{named: map[string]*parser.AssocNode{}, used: map[string]bool{}}
	if len(args) == 0 {
		return args, kw
	}
	kh, ok := args[len(args)-1].(*parser.KeywordHashNode)
	if !ok {
		return args, kw
	}
	kw.all = kh.Elements
	for _, el := range kh.Elements {
		switch el := el.(type) {
		case *parser.AssocNode:
			sym, ok := el.Key.(*parser.SymbolNode)
			if !ok {
				f.errorf(el.Key, "%s takes keyword arguments: keys must be symbols", m.Name)
			}
			name := sym.Unescaped.Value
			if kw.named[name] != nil {
				f.errorf(el, "duplicated keyword %s:", name)
			}
			kw.named[name] = el
			kw.order = append(kw.order, el)
		case *parser.AssocSplatNode:
			kw.splats = append(kw.splats, el)
		default:
			f.errorf(el, "unsupported keyword argument")
		}
	}
	if len(kw.splats) > 1 { // `**a, k: v, **b` is one `**{**a, k: v, **b}`: later keys win, in order, as MRI's (CurlDownloadStrategy#curl's super)
		at := slices.Index(kw.all, kw.splats[0])
		first := kw.splats[0].(*parser.AssocSplatNode)
		rest := slices.Clone(kw.all[at:])
		for _, el := range rest {
			if a, ok := el.(*parser.AssocNode); ok {
				delete(kw.named, a.Key.(*parser.SymbolNode).Unescaped.Value) //nolint:forcetypeassert // checked in the loop above
				kw.order = slices.DeleteFunc(kw.order, func(o *parser.AssocNode) bool { return o == a })
			}
		}
		merged := &parser.AssocSplatNode{Location: first.Location, Value: &parser.HashNode{Location: first.Location, Elements: rest}}
		kw.splats, kw.all = []parser.Node{merged}, append(slices.Clone(kw.all[:at]), merged)
	}
	return args[:len(args)-1], kw
}

// keywordArg renders keyword parameter p's argument: the call's value, a
// default (callee-side ones get a zero value and a clear bit in rbKw), or
// for `**opts` the call's other keywords as a Hash.
func (f *fctx) keywordArg(n parser.Node, m *Method, p Param, env map[string]Type, kw *kwArgs) (string, bool) {
	if p.KwRest {
		hadSplat := len(kw.splats) > 0
		var els []parser.Node
		for _, el := range kw.all {
			if sp, ok := el.(*parser.AssocSplatNode); ok && kw.splatTmp != "" {
				// already evaluated for the named keywords: rest is what they did not take
				var names []string
				for _, q := range m.Params {
					if q.Keyword && !q.KwRest {
						names = append(names, strconv.Quote(q.Name))
					}
				}
				code := kw.splatTmp
				if len(names) > 0 {
					code = "rbKwRestWithout(" + code + ", " + strings.Join(names, ", ") + ")"
				}
				el = &parser.AssocSplatNode{Value: &exprNode{e: expr{code: code, typ: f.splatHashType(kw)}}, Location: sp.Location}
			}
			if a, ok := el.(*parser.AssocNode); !ok || !kw.used[a.Key.(*parser.SymbolNode).Unescaped.Value] {
				els = append(els, el)
			}
		}
		kw.splats = nil
		for _, a := range kw.order {
			kw.used[a.Key.(*parser.SymbolNode).Unescaped.Value] = true
		}
		h := f.genHash(n, els, closed(p.Type, env))
		unify(p.Type, h.typ, env)
		code := f.coerceArg(n, h, p, env)
		if hadSplat { // a splatted key a named keyword took is not rest's
			code = kwRestWithout(code, m, kw)
		}
		return code, true
	}
	if a := kw.named[p.Name]; a != nil {
		kw.used[p.Name] = true
		v := f.genExpr(a.Value, f.pendingWant(p, closed(p.Type, env)))
		f.noteUse(p, a.Value, v.typ)
		unify(p.Type, v.typ, env)
		return f.coerceArg(a.Value, v, p, env), true
	}
	if len(kw.splats) > 0 {
		return f.keywordFromSplat(n, m, p, env, kw), false
	}
	switch {
	case p.Default != nil && m.calleeDefaults:
		return "rbZero[" + f.c.goType(subst(p.Type, env)) + "]()", false
	case p.Default != nil:
		return f.defaultArg(n, m, p, env), false
	}
	f.errorf(n, "%s: missing keyword: :%s", m, p.Name)
	return "", false
}

// kwRestWithout drops from a `**rest` built with splats the keys m's named
// keyword parameters took by name.
func kwRestWithout(code string, m *Method, kw *kwArgs) string {
	var names []string
	for _, q := range m.Params {
		if q.Keyword && !q.KwRest && kw.named[q.Name] != nil {
			names = append(names, strconv.Quote(q.Name))
		}
	}
	if len(names) == 0 {
		return code
	}
	return "rbKwRestWithout(" + code + ", " + strings.Join(names, ", ") + ")"
}

// keywordFromSplat takes keyword p from the call's `**h` at run time: h's
// value when it has the key, else p's default, else MRI's "missing keyword"
// ArgumentError. h is evaluated once, and a key no keyword names raises.
func (f *fctx) keywordFromSplat(n parser.Node, m *Method, p Param, env map[string]Type, kw *kwArgs) string {
	if len(kw.splats) > 1 {
		f.errorf(kw.splats[0], "**splat into %s's named keyword parameters works with one **hash; merge them first", m.Name)
	}
	hasRest := slices.ContainsFunc(m.Params, func(q Param) bool { return q.KwRest })
	sp := kw.splats[0].(*parser.AssocSplatNode)
	if kw.splatTmp == "" {
		h := f.genExpr(sp.Value, nil)
		ht, ok := stripOpt(h.typ).(TClass)
		if hashAny := ok && ht.C.RubyName == "Hash" && !isOpt(h.typ) && isAny(ht.Args[0]); f.lenient() && (isAny(h.typ) || hashAny) {
			// a gem's `Action.new(range, enforcer, **attributes)` of an untyped Hash (parser's TreeRewriter#combine): its
			// keys are checked to be Symbols when it runs, as MRI checks them (decision 444)
			want := TClass{C: f.c.classes["Hash"], Args: []Type{f.cls("Symbol"), TAny{}}}
			h = expr{code: fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(want), f.coerce(sp, h, TAny{}), want.String()), typ: want}
			ht, ok = want, true
		}
		if !ok || ht.C.RubyName != "Hash" || isOpt(h.typ) || !isClass(ht.Args[0], "Symbol") {
			f.errorf(sp, "**%s into %s's keywords must be a Hash[Symbol, T], not %s", f.f.text(sp.Value.GetLocation()), m.Name, h.typ)
		}
		names := []string{}
		for _, q := range m.Params {
			if q.Keyword {
				names = append(names, strconv.Quote(q.Name))
			}
		}
		kw.splatTmp = f.newTmp()
		if hasRest { // the other keys are **rest's (keywordArg)
			f.emit("%s := %s", kw.splatTmp, h.code)
		} else {
			f.emit("%s := rbKwSplat(%s, %s)", kw.splatTmp, h.code, strings.Join(names, ", "))
		}
		f.c.noteMarshal(h.typ)
	}
	ht := f.splatType(kw)
	want := subst(p.Type, env)
	out, got, v := f.newTmp(), f.newTmp(), f.newTmp()
	f.emit("var %s %s", out, f.c.goType(want))
	f.emit("%s, %s := %s.rbGet(Symbol(%q))", v, got, kw.splatTmp, p.Name)
	f.emit("if %s {", got)
	val := expr{code: v, typ: ht}
	switch {
	case fitsValue(val, closed(p.Type, env)) || isAny(ht):
		unify(p.Type, val.typ, env)
		f.noteUse(p, sp.Value, val.typ)
		f.emit("\t%s = %s", out, f.coerceArg(sp, val, p, env))
	case f.lenient(): // a gem's `**command_options` of mixed values: each converted where it lands (decision 255)
		f.emit("\t%s = %s", out, f.coerce(sp, expr{code: "any(" + v + ")", typ: TAny{}}, want))
	default: // `{ name: "a" }` holds Strings: an :age in it is a typed boundary's TypeError (decision 20)
		f.emit("\t_ = %s", v)
		f.emit("\tpanic(NewTypeError(Ref(String(%q))))", fmt.Sprintf("rugaroo: **%s holds %s values; keyword %s takes %s", f.f.text(sp.Value.GetLocation()), ht, p.Name, want))
	}
	f.emit("} else {")
	switch {
	case p.Default != nil && m.calleeDefaults:
		kw.given = got // the callee runs it when the bit is clear
	case p.Default != nil:
		f.indent++
		f.emit("%s = %s", out, f.coerceArg(n, f.defaultExpr(m, p, env), p, env))
		f.indent--
	default:
		f.emit("\trbKwMissing(%q)", p.Name)
	}
	f.emit("}")
	kw.used[p.Name] = true
	return out
}

// splatType is the value type of the call's `**h`.
func (f *fctx) splatType(kw *kwArgs) Type {
	return f.splatHashType(kw).Args[1]
}

func (f *fctx) splatHashType(kw *kwArgs) TClass {
	var h expr
	f.probe(func() { h = f.genExpr(kw.splats[0].(*parser.AssocSplatNode).Value, nil) })
	if tc, ok := stripOpt(h.typ).(TClass); ok && tc.C.RubyName == "Hash" && !isAny(tc.Args[0]) {
		return tc
	}
	// a gem's untyped **hash, converted to this where it was read (keywordFromSplat; decision 444)
	return TClass{C: f.c.classes["Hash"], Args: []Type{f.cls("Symbol"), TAny{}}}
}

// checkUnknown rejects keywords the method has no parameter for (MRI's ArgumentError, at compile time).
func (kw *kwArgs) checkUnknown(f *fctx, m *Method) {
	if kw == nil {
		return
	}
	for _, a := range kw.order {
		if name := a.Key.(*parser.SymbolNode).Unescaped.Value; !kw.used[name] {
			f.errorf(a, "%s: unknown keyword: :%s", m, name)
		}
	}
	if len(kw.splats) > 0 && kw.splatTmp == "" {
		f.errorf(kw.splats[0], "%s takes no **keywords", m)
	}
}

// optJoin binds a method type variable that several arguments share as
// X? when they are X and X? (or nil): `assert_equal 3, h[:a]` takes T as
// Integer?, where binding T from the first argument would reject the
// second (decision 93).
func (f *fctx) optJoin(m *Method, env map[string]Type, args []parser.Node) {
	if len(m.TypeParams) == 0 {
		return
	}
	at := map[string][]int{}
	for i, p := range m.Params {
		if p.Rest || i >= len(args) {
			break
		}
		if v, ok := p.Type.(TVar); ok && slices.Contains(m.TypeParams, v.Name) {
			if _, bound := env[v.Name]; !bound {
				at[v.Name] = append(at[v.Name], i)
			}
		}
	}
	for name, idx := range at {
		if len(idx) < 2 {
			continue
		}
		var j Type
		for k, i := range idx {
			if _, ok := args[i].(*parser.SplatNode); ok {
				j = nil
				break
			}
			var a expr
			f.probe(func() { a = f.genExpr(args[i], nil) })
			if k == 0 {
				j = a.typ
				continue
			}
			var ok bool
			if j, ok = join(j, a.typ); !ok {
				j = nil
				break
			}
		}
		if o, ok := j.(TOpt); ok && !isAny(o.Elem) {
			env[name] = o
		}
	}
}

// genRestArgs: with a splat, one fresh slice, since Go spreads only a lone slice and Ruby's rest param never aliases the caller's array.
func (f *fctx) genRestArgs(n parser.Node, p Param, env map[string]Type, args []parser.Node, exprs []expr, ai, nargs int) []string {
	var codes []string
	var splats map[int]bool
	for ; ai < nargs; ai++ {
		var a expr
		an := n
		if exprs != nil {
			a = exprs[ai]
		} else {
			an = args[ai]
			if sp, ok := an.(*parser.SplatNode); ok {
				a = f.lenientSplat(f.genExpr(sp.Expression, nil))
				ac, ok := a.typ.(TClass)
				if !ok || ac.C.Name != "Array" {
					f.errorf(an, "splat of non-array %s", a.typ)
				}
				f.noteUse(p, an, ac.Args[0])
				f.noteAnyRest(p, an, ac.Args[0])
				unify(p.Type, ac.Args[0], env)
				if splats == nil {
					splats = map[int]bool{}
				}
				splats[len(codes)] = true
				codes = append(codes, f.splatSlice(an, a.code, ac.Args[0], subst(p.Type, env)))
				continue
			}
			mark := f.buf.Len()
			a = f.genExpr(an, f.pendingWant(p, closed(p.Type, env)))
			f.pinBefore(mark, codes)
		}
		f.noteUse(p, an, a.typ)
		f.noteAnyRest(p, an, a.typ)
		unify(p.Type, a.typ, env)
		codes = append(codes, f.coerceArg(an, a, p, env))
	}
	if splats == nil {
		return codes
	}
	et := f.c.goType(subst(p.Type, env))
	var parts, lit []string
	flush := func() {
		if len(lit) > 0 {
			parts = append(parts, "[]"+et+"{"+strings.Join(lit, ", ")+"}")
			lit = nil
		}
	}
	for i, c := range codes {
		if splats[i] {
			flush()
			parts = append(parts, c)
			continue
		}
		lit = append(lit, c)
	}
	flush()
	return []string{"slices.Concat[[]" + et + "](" + strings.Join(parts, ", ") + ")..."}
}

// splatSlice converts elements only when the Go types differ ([]Integer is not []any).
func (f *fctx) splatSlice(n parser.Node, code string, from, to Type) string {
	ft, tt := f.c.goType(from), f.c.goType(to)
	if ft == tt {
		return "(" + code + ").s"
	}
	conv := f.coerce(n, expr{code: "x", typ: from}, to)
	return "rbSplat((" + code + ").s, func(x " + ft + ") " + tt + " { return " + conv + " })"
}

// overload stands in for RBS overloads (decision 12): a call whose argument
// count m cannot take goes to the receiver class's `__<name>_<count>`, and
// one whose first argument is of class C to `__<name>_<c>` when that takes
// the call's argument count (C snake-cased:
// `__idx_range`, `__minus_time`), if defined. Operators use their Go name
// minus `Op_`.
func (f *fctx) overload(e *entry, recvT Type, args []parser.Node) *entry {
	m := e.M
	owner := f.overloadOwner(m, recvT)
	if owner == nil || strings.HasPrefix(m.Name, "__") {
		return nil
	}
	name := "__" + overloadBase(m.Name) + "_"
	if r := f.literalOverload(m, owner, args); r != nil {
		return r
	}
	if r := f.selfOverload(m, owner, name, recvT, args); r != nil {
		return r
	}
	if f.curBlock != nil && m.Block == nil {
		if r := f.classBlockOverload(m, owner, name, args); r != nil {
			return r // `xs.sum(0.0) { |x| x }`
		}
		return blockOverload(owner, name, len(args))
	}
	if f.curBlock == nil && m.Block != nil {
		if r := f.enumClassOverload(owner, name, args); r != nil {
			return r
		}
		if r := blocklessOverload(m, owner, name, len(args)); r != nil {
			return r
		}
	}
	if r := owner.lookup(name + "same"); r != nil && len(args) >= 2 && f.sameArgs(r.M, args) {
		return r // `assert_equal 3, h[:a]`: both sides one static type
	}
	if r := f.classOverload(m, owner, name, args); r != nil {
		return r
	}
	rest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest })
	if len(args) >= requiredArgs(m) && (rest || len(args) <= len(m.Params)) {
		return nil
	}
	return owner.lookup(name + strconv.Itoa(len(args)))
}

// classOverload is `__<name>_<class>` for the first argument's class (secondArgOverload adds the second's).
func (f *fctx) classOverload(m *Method, owner *Class, name string, args []parser.Node) *entry {
	if !(len(args) >= 1 && !isSplat(args[0]) && classTwins(m.Name) && !optOn(owner, m) && slices.ContainsFunc(owner.methodSet(), func(x entry) bool { return strings.HasPrefix(x.M.Name, name) })) {
		return nil
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	if o, ok := a.typ.(TOpt); ok && !nilEqualOps[m.Name] && (f.lenient() || f.nilTwin(args[0], m, o)) { // a gem's T? is its T, nil raising where used (decisions 172, 393); `==` answers false (decision 404)
		a.typ = o.Elem
	}
	if c, ok := a.typ.(TClass); ok {
		if r := f.secondArgOverload(owner, name+snake(c.C.RubyName), args); r != nil {
			return r
		}
		if r := owner.lookup(name + snake(c.C.RubyName)); r != nil && len(args) >= requiredArgs(r.M) && (len(args) <= len(r.M.Params) || slices.ContainsFunc(r.M.Params, func(p Param) bool { return p.Rest })) {
			return r
		}
	}
	return nil
}

// classBlockOverload is `__<name>_<class>_block` for a call with a block by its one argument's class: sum(init)'s
// result class follows init's (#145).
func (f *fctx) classBlockOverload(m *Method, owner *Class, name string, args []parser.Node) *entry {
	if len(args) != 1 || isSplat(args[0]) || !classTwins(m.Name) || !slices.ContainsFunc(owner.methodSet(), func(x entry) bool {
		return strings.HasPrefix(x.M.Name, name) && strings.HasSuffix(x.M.Name, "_block") && x.M.Name != name+"block"
	}) {
		return nil
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	if c, ok := a.typ.(TClass); ok {
		return owner.lookup(name + snake(c.C.RubyName) + "_block")
	}
	return nil
}

// secondArgOverload is `__<name>_<first>_<second>` by the second argument's class too: `gsub(/re/, hash)` (decision 404).
func (f *fctx) secondArgOverload(owner *Class, prefix string, args []parser.Node) *entry {
	if len(args) != 2 || !slices.ContainsFunc(owner.methodSet(), func(x entry) bool { return strings.HasPrefix(x.M.Name, prefix+"_") }) {
		return nil
	}
	var b expr
	f.probe(func() { b = f.genExpr(args[1], nil) })
	if c, ok := b.typ.(TClass); ok {
		return owner.lookup(prefix + "_" + snake(c.C.RubyName))
	}
	return nil
}

// enumClassOverload is a blockless call's `__name_enum_<class>` for its one argument's class: `(1..2).step(0.5)` is a Float sequence (decision 299).
func (f *fctx) enumClassOverload(owner *Class, name string, args []parser.Node) *entry {
	if len(args) != 1 || !slices.ContainsFunc(owner.methodSet(), func(x entry) bool { return strings.HasPrefix(x.M.Name, name+"enum_") }) {
		return nil
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	if c, ok := a.typ.(TClass); ok {
		return owner.lookup(name + "enum_" + snake(c.C.RubyName))
	}
	return nil
}

// optOn: OptionParser's on-methods overload by switch kind, never by class: a runtime-decided switch keeps `on`, not `__on_string` (decision 245).
func optOn(owner *Class, m *Method) bool {
	return owner.RubyName == "OptionParser" && (m.Name == "on" || m.Name == "on_tail" || m.Name == "on_head")
}

// classTwins: a method named like an operator's overloads (`div` and `/`'s `__div_integer`) takes none by class, which are the operator's.
func classTwins(name string) bool {
	if _, op := opNames[name]; op {
		return true
	}
	return !opBases[overloadBase(name)]
}

// opBases are the operators' overload spellings (`div` for `/`).
var opBases = func() map[string]bool {
	out := map[string]bool{}
	for op := range opNames {
		out[overloadBase(op)] = true
	}
	return out
}()

// blockOverload is a call with a block to a method that takes none: `__name_block`, `__name_<count>_block`, or a
// `__name_<count>` that takes the block.
func blockOverload(owner *Class, name string, nargs int) *entry {
	if r := owner.lookup(name + "block"); r != nil && (nargs == 0 || len(r.M.Params) > 0) {
		return r // `xs.sum { |x| x.price }`; not `xs.sum(10) { }`, which is `__sum_1_block`
	}
	if r := owner.lookup(name + strconv.Itoa(nargs) + "_block"); r != nil {
		return r // `xs.fill(2) { |i| i }`: by count, with a block
	}
	if r := owner.lookup(name + strconv.Itoa(nargs)); r != nil && r.M.Block != nil {
		return r // `xs.fill { |i| i }`: the count's overload takes the block
	}
	return nil
}

// blocklessOverload is a call without a block to a method that takes one: a blockless `__name_<count>` (decision 219),
// or `__name_enum`.
func blocklessOverload(m *Method, owner *Class, name string, nargs int) *entry {
	if r := owner.lookup(name + strconv.Itoa(nargs)); r != nil && r.M.Block == nil && !m.Block.Optional {
		return r // `xs.any?`, `xs.any?(Integer)`: the blockless forms
	}
	if r := owner.lookup(name + "enum"); r != nil && (r.M.Owner == m.Owner || !r.M.Owner.IsModule) {
		return r // `xs.each_slice(2)` without a block: an Array standing in for the Enumerator; not a module's twin
		// of a method a class defines itself (Range#reverse_each over Enumerable's, decision 331)
	}
	return nil
}

var overloadSuffixes = strings.NewReplacer("?", "_q", "!", "_bang") // built once: per call it was 7.5 CPU-s of brew's gen

// overloadBase is name as its `__<base>_<suffix>` overloads spell it.
func overloadBase(name string) string {
	if op, ok := opNames[name]; ok {
		return strings.ToLower(strings.TrimPrefix(op, "Op_"))
	}
	return overloadSuffixes.Replace(name)
}

// literalOverload picks an overload from literal arguments: String#scan
// with a pattern that has groups yields each match's groups; OptionParser#on
// is typed by its switch strings (decision 101).
func (f *fctx) literalOverload(m *Method, owner *Class, args []parser.Node) *entry {
	switch {
	case owner.RubyName == "String" && len(args) >= 1:
		return f.stringLiteralOverload(m, owner, args)
	case owner.RubyName == "Array" && m.Name == "pack" && len(args) == 1: // a literal format without U packs bytes (decision 365)
		if s, ok := args[0].(*parser.StringNode); ok && !strings.ContainsAny(s.Unescaped.Value, "UmMu") {
			return owner.lookup("__pack_binary")
		}
	case owner.RubyName == "OptionParser" && (m.Name == "on" || m.Name == "on_tail" || m.Name == "on_head"):
		if k := f.optKind(args); k != "" {
			return owner.lookup("__" + m.Name + "_" + k)
		}
	case owner.RubyName == "CSV" && owner.metaOf != nil:
		return f.csvOverload(m, owner, args)
	case owner.RubyName == "Float" && (m.Name == "round" || m.Name == "floor" || m.Name == "ceil") && len(args) == 1:
		// x.round(n) is an Integer for n <= 0, as BigDecimal's: a literal n decides the type (decision 299)
		if lit, ok := args[0].(*parser.IntegerNode); ok {
			n, err := strconv.Atoi(f.f.text(lit.Location))
			if err == nil && n <= 0 {
				return owner.lookup("__" + m.Name + "_int")
			}
		}
	case owner.RubyName == "BigDecimal" && m.Name == "round" && len(args) == 1:
		// round(n) is a BigDecimal for n >= 1 and an Integer otherwise (decision 112): a literal n decides the type
		if lit, ok := args[0].(*parser.IntegerNode); ok {
			n, err := strconv.Atoi(f.f.text(lit.Location))
			if err == nil && n >= 1 {
				return owner.lookup("__round_digits")
			}
		}
	}
	return nil
}

// stringLiteralOverload: String#scan with a pattern that has groups yields each match's groups, and force_encoding of a
// literal BINARY and unpack1 of a literal a/A/Z/m/u/M first directive make a binary String (decisions 352, 439).
func (f *fctx) stringLiteralOverload(m *Method, owner *Class, args []parser.Node) *entry {
	arg := args[0]
	if m.Name == "unpack1" {
		if s, ok := arg.(*parser.StringNode); ok && s.Unescaped.Value != "" && strings.IndexByte("aAZmuM", s.Unescaped.Value[0]) >= 0 {
			return owner.lookup("__unpack1_binary")
		}
		return nil
	}
	if len(args) != 1 {
		return nil
	}
	switch m.Name {
	case "scan":
		g, ok := f.literalGroups(arg)
		switch {
		case ok && g > 0:
			return owner.lookup("__scan_groups")
		case !ok && f.lenient(): // a gem's pattern known only at run time: rows or strings, as MRI decides (decision 334)
			return owner.lookup("__scan_any")
		}
	case "force_encoding":
		if binaryEncodingArg(f.f, arg) {
			return owner.lookup("__force_encoding_binary")
		}
	}
	return nil
}

// binaryEncodingArg is a literal naming ASCII-8BIT: "BINARY"/"ASCII-8BIT" in any case, or Encoding::BINARY/ASCII_8BIT.
func binaryEncodingArg(f *File, n parser.Node) bool {
	switch a := n.(type) {
	case *parser.StringNode:
		s := strings.ToUpper(a.Unescaped.Value)
		return s == "BINARY" || s == "ASCII-8BIT"
	case *parser.ConstantPathNode:
		t := f.text(a.Location)
		return t == "Encoding::BINARY" || t == "Encoding::ASCII_8BIT" || t == "::Encoding::BINARY" || t == "::Encoding::ASCII_8BIT"
	}
	return false
}

// selfOverload picks, for a method whose own `@self` the receiver doesn't
// fit, the first `__<name>_*` sibling whose `@self` it does and that takes
// the call's arguments and block (`[[1], [2]].flatten` is
// `__flatten_nested`, decision 92).
func (f *fctx) selfOverload(m *Method, owner *Class, name string, recvT Type, args []parser.Node) *entry {
	if !owner.selfDefs {
		return nil
	}
	fits := func(x *Method) bool {
		f.c.resolveMethod(x)
		return x.SelfType != nil && (x.Block != nil) == (f.curBlock != nil) &&
			len(args) >= requiredArgs(x) && len(args) <= len(x.Params) &&
			unify(x.SelfType, recvT, map[string]Type{}) && optShape(x.SelfType, recvT) &&
			(!strings.HasSuffix(x.Name, "_untyped") || holdsAny(recvT)) // a `__name_untyped` twin is for an untyped receiver only: a typed one keeps its compile error (decision 444)
	}
	if m.SelfType != nil && fits(m) {
		return nil
	}
	for _, x := range owner.methodSet() {
		if strings.HasPrefix(x.M.Name, name) && fits(x.M) {
			return owner.lookup(x.M.Name)
		}
	}
	return nil
}

// optShape reports whether actual is optional wherever pattern is: unify lets `U?` take a plain `Integer`, but an
// `@self Array[U?]` form needs elements that really are optional (Go's *U).
func optShape(pattern, actual Type) bool {
	switch p := pattern.(type) {
	case TOpt:
		a, ok := actual.(TOpt)
		return ok && optShape(p.Elem, a.Elem)
	case TClass:
		a, ok := actual.(TClass)
		if !ok {
			return true
		}
		for i := range min(len(p.Args), len(a.Args)) {
			if !optShape(p.Args[i], a.Args[i]) {
				return false
			}
		}
	case TAny, TFunc, TNil, TTuple, TUnion, TVar, TVoid: // no optional slot a receiver must match
	}
	return true
}

// sameArgs reports whether a call's first two arguments have one static
// type (T and T? or nil counting as T?) that m's first parameter takes
// and m takes the argument count: then its `__<name>_same` overload
// applies (decision 93).
func (f *fctx) sameArgs(m *Method, args []parser.Node) bool {
	f.c.resolveMethod(m)
	if len(args) < requiredArgs(m) || len(args) > len(m.Params) {
		return false
	}
	var a, b expr
	f.probe(func() {
		a = f.genExpr(args[0], nil)
		b = f.genExpr(args[1], nil)
	})
	ea, eb := a.typ, b.typ
	if o, ok := ea.(TOpt); ok {
		ea = o.Elem
	}
	if o, ok := eb.(TOpt); ok {
		eb = o.Elem
	}
	switch {
	case isNil(ea) && isNil(eb), isAny(ea), isAny(eb):
		return false
	case !isNil(ea) && !isNil(eb) && !typeEq(ea, eb):
		return false
	}
	j, ok := join(a.typ, b.typ)
	return ok && unify(m.Params[0].Type, j, map[string]Type{})
}

// restArgs renders arguments for a Go variadic whose last parameter is a `*rest`'s element type: the leading ones
// one each, the rest (splats spliced in) as one slice passed with `...`.
func (f *fctx) restArgs(n parser.Node, args []parser.Node, params []Type) string {
	last := len(params) - 1
	if len(args) < last {
		f.errorf(n, "wrong number of arguments (given %d, expected %d+)", len(args), last)
	}
	codes := make([]string, 0, last+1)
	for i := range last {
		e := f.genExpr(args[i], params[i])
		codes = append(codes, f.coerce(args[i], e, params[i]))
	}
	elem := params[last]
	acc := "[]" + f.c.goType(elem) + "{}"
	for _, a := range args[last:] {
		if sp, ok := a.(*parser.SplatNode); ok {
			e := f.genExpr(sp.Expression, nil)
			t, isArr := e.typ.(TClass)
			if isArr && t.C.RubyName == "Array" && f.blockSig != nil && len(params) == len(f.blockSig.Params) {
				f.noteYield(last, sp, t.Args[0]) // what `yield(*xs)` passes types the block's *rest (decision 363)
			}
			switch {
			case isArr && t.C.RubyName == "Array" && typeEq(t.Args[0], elem):
				acc = "append(" + acc + ", " + e.code + ".s...)"
			case isArr && t.C.RubyName == "Array" && isAny(elem):
				acc = "append(" + acc + ", rbAnySlice(" + e.code + ")...)"
			case isAny(e.typ) && isAny(elem): // MRI's splat of an untyped value: an Array spreads, nil is nothing, else one value
				acc = "append(" + acc + ", rbSplatObj(" + e.code + ")...)"
			default:
				f.errorf(sp, "cannot splat %s into a block's *rest of %s", e.typ, elem)
			}
			continue
		}
		if kw, ok := a.(*parser.KeywordHashNode); ok { // a block takes `k: v` as one Hash, as a proc does
			a = &parser.HashNode{Location: kw.Location, Elements: kw.Elements}
		}
		e := f.genExpr(a, elem)
		acc = "append(" + acc + ", " + f.coerce(a, e, elem) + ")"
	}
	return strings.Join(append(codes, acc+"..."), ", ")
}

// lastSplat is args' last argument when it is a `*x` splat.
func lastSplat(args []parser.Node) (*parser.SplatNode, bool) {
	if len(args) == 0 {
		return nil, false
	}
	sp, ok := args[len(args)-1].(*parser.SplatNode)
	return sp, ok && sp.Expression != nil
}

// blockArg is the Go value a call passes for m's block: the closure of the literal block, or nil.
func (f *fctx) blockArg(n parser.Node, m *Method, env map[string]Type, block parser.Node) string {
	switch {
	case block != nil:
		f.inferYielder(n, m, env, block)
		if f.lenient() { // a gem's `@h ||= Hash.new { |h, k| ... }` into an untyped ivar: nothing binds K, V
			bindUntyped(m, env, m.Block.Params...) // only what the block is passed: its result (map's U) is inferred from the body
		}
		if b, ok := block.(*parser.BlockNode); ok && f.lenient() && anyRestBlock(m.Block) && (f.selfClassObj || isAny(f.selfType)) {
			// a class-body DSL block that does not compile on its lexical self (`section(:a) { item 1 }`, the
			// gem instance_eval-ing it on another object): its self is the one it is called with (decision 324)
			err := f.try(func() { f.probe(func() { f.genClosure(n, block, m.Block, maps.Clone(env)) }) })
			if fn, ok := f.rebindableBlock(n, b, false); ok && err != nil {
				return fn
			}
		}
		if b, ok := block.(*parser.BlockNode); ok && f.lenient() && anyRestBlock(m.Block) && !f.selfClassObj && !isAny(f.selfType) && f.selfMissing() {
			// a DSL block whose calls only self's method_missing answers on its lexical self (a cask's `postflight do
			// system_command ... end`, instance_eval'd on a DSL::Postflight later): its self is the one it is called with
			// (decision 374)
			before := f.missingCalls
			err := f.try(func() { f.probe(func() { f.genClosure(n, block, m.Block, maps.Clone(env)) }) })
			if err == nil && f.missingCalls > before {
				if fn, ok := f.rebindableBlock(n, b, true); ok {
					return fn
				}
			}
		}
		f.binNext = m.binBlock // its String params are binary (decision 352)
		return f.genClosure(n, block, m.Block, env)
	case !m.Block.Optional:
		f.errorf(n, "%s requires a block", m.Name)
	}
	return "nil"
}

// noteWideningUse: a gem's untyped value into an overridden method's slot (bindata's `obj.assign(value)` on a Base) or
// a constructor (`RawIO.new(io)` of a File where another call passes a StringIO) makes the parameter untyped (decision
// 324).
func (f *fctx) noteWideningUse(m *Method, p Param, an parser.Node, t Type) {
	if _, untyped := t.(TAny); !untyped || !f.lenient() || p.Pending == "" || f.c.uses == nil || m.File == nil || !f.c.gemFile(m.File) || m.Owner == nil {
		return
	}
	if m.Name == "initialize" || m.Owner.descendantDefines(m.Name, true) || m.Owner.includerDefines(m.Name, map[*Class]bool{}) {
		f.c.uses[p.Pending] = append(f.c.uses[p.Pending], paramUse{typ: TAny{}, file: f.f, off: an.GetLocation().StartOffset, widen: true})
	}
}

// noteAnyRest: a gem's untyped value into a rest parameter: its elements are anything (bindata's
// parse_and_append_field(*args)).
func (f *fctx) noteAnyRest(p Param, an parser.Node, t Type) {
	if isAny(t) && f.lenient() && p.Pending != "" && f.c.uses != nil {
		f.c.uses[p.Pending] = append(f.c.uses[p.Pending], paramUse{typ: TAny{}, file: f.f, off: an.GetLocation().StartOffset, anyRest: true})
	}
}

// emptySplat checks at run time that a gem forwarder's `*args` after every positional parameter is filled is empty,
// as MRI does; false when args does not end in a splat or the file is no gem's.
func (f *fctx) emptySplat(args []parser.Node) bool {
	sp, ok := lastSplat(args)
	if !ok || !f.lenient() {
		return false
	}
	a := f.genExpr(sp.Expression, nil)
	f.emit("rbSplatArity(len(rbAs[*Array[any]](%s, \"Array\").s), 0, 0)", f.coerce(sp, a, TAny{}))
	return true
}

// defaultArg is a caller-evaluated default. The default is a node of the def's file (often the prelude): literal
// text and error positions must come from there.
func (f *fctx) defaultArg(n parser.Node, m *Method, p Param, env map[string]Type) string {
	d := f.defaultExpr(m, p, env)
	unify(p.Type, d.typ, env)
	return f.coerceArg(n, d, p, env)
}

// defaultExpr is p's default generated in the def's file.
func (f *fctx) defaultExpr(m *Method, p Param, env map[string]Type) expr {
	caller := f.f
	f.f = defaultsFile(m)
	d := f.genExpr(p.Default, closed(p.Type, env))
	f.f = caller
	return d
}

// argAt is positional argument i for parameter p, generated (or given in exprs) with its node.
func (f *fctx) argAt(n parser.Node, p Param, env map[string]Type, args []parser.Node, exprs []expr, i int, codes []string) (parser.Node, expr) {
	if exprs != nil {
		return n, exprs[i]
	}
	an := args[i]
	if _, ok := an.(*parser.SplatNode); ok {
		f.errorf(an, "splat into a non-rest parameter is not supported")
	}
	mark := f.buf.Len()
	a := f.genExpr(an, f.pendingWant(p, closed(p.Type, env)))
	f.pinBefore(mark, codes)
	return an, a
}

// overloadOwner is the class whose twins a call picks from: the receiver's, or for a self call in a module method the
// enclosing class's (`reverse_each.to_a` in an Enumerable includer: its twin with the includer's E, decision 331).
func (f *fctx) overloadOwner(m *Method, recvT Type) *Class {
	if rc, ok := recvT.(TClass); ok {
		return rc.C
	}
	if tv, ok := recvT.(TVar); ok && tv.Name == "Self" && f.owner != nil && !f.owner.IsModule && m.Owner != nil && m.Owner.IsModule {
		return f.owner
	}
	return m.Owner
}
