package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// coerce converts e to the representation of type `to`.
func (f *fctx) coerce(n parser.Node, e expr, to Type) string {
	if f.lenient() && isVoid(e.typ) && !isNil(e.typ) && to != nil && !isVoid(to) && e.code != "" && !e.noreturn && !e.stmt {
		// a gem's value of a void call (`yield self` where the block was typed void): the call runs, its value is nil (decision 244)
		f.emitExprStmt(n, e)
		e = expr{code: "nil", typ: TNil{}}
	}
	if isClass(e.typ, "NilClass") && isAny(to) && !e.lit && e.code != "nil" {
		// a reopened NilClass's `self` (liquid's `def to_liquid = self`) is a typed nil pointer: boxed, it would not be nil
		return "rbNilAny(" + e.code + ")"
	}
	if e.code == "nil" && valOpt(to) { // a nil relabeled as Integer? (`@x = nil #: Integer?`): rbOpt's zero (decision 396)
		return f.c.goType(to) + "{}"
	}
	code := f.coerce0(n, e, to)
	if f.lenient() && code == e.code && f.pass == 2 {
		return f.goTypeGuard(n, e, to)
	}
	return code
}

// goTypeGuard converts what gem code passes unconverted although its Go type is not the wanted one, where rugaroo's
// lenient typing let it through (decision 172): checked at run time, as MRI would fail there.
func (f *fctx) goTypeGuard(n parser.Node, e expr, to Type) string {
	if to == nil || isVoid(to) || isVoid(e.typ) || e.noreturn || e.code == "nil" || e.lit {
		return e.code
	}
	from, want := f.c.goType(e.typ), f.c.goType(to)
	if from == want || want == "any" || from == "" || want == "" {
		return e.code
	}
	switch t := to.(type) {
	case TVar:
		if st, ok := f.selfType.(TVar); t.Name == "Self" && ok && st.Name == "Self" { // only inside a generic method body
			return "any(" + e.code + ").(Self)"
		}
		if from == "any" && t.Name != "Self" {
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", want, e.code, t.Name))
		}
		if t.Name != "Self" && want == t.Name { // a concrete value where a method type parameter is wanted (a Hash arm's pair as Item): checked at run time
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](any(%s), %q)", want, e.code, t.Name))
		}
	case TOpt:
		src := e.code
		if isOpt(e.typ) {
			src = f.c.optAny(e.code, e.typ)
		} else if from != "any" {
			return e.code
		}
		return f.noteConv(n, f.c.optOf(t.Elem, src))
	case TClass:
		if from == "any" {
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", want, e.code, t.String()))
		}
		if _, ok := e.typ.(TTuple); ok && t.C.RubyName != "Array" {
			// a Hash arm's `|pair|` where a String is wanted (Homebrew's `depends_on.formula.map { |f| Dependency.new f }` on
			// an untyped receiver): an Array there, raising if that arm runs (decision 258)
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", want, f.coerceToAny(e), t.String()))
		}
	case TAny, TFunc, TNil, TTuple, TUnion, TVoid:
		if from == "any" && !isUnion(to) {
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", want, e.code, to.String()))
		}
	}
	return e.code
}

func (f *fctx) coerce0(n parser.Node, e expr, to Type) string {
	if to == nil || isVoid(to) && !isNil(to) {
		return e.code
	}
	if e.noreturn { // `f(raise "x")`: a func literal ending in panic is a value of any type to Go
		return "func() " + f.c.goType(to) + " {\n" + e.code + "\n}()"
	}
	e = f.valueOf(e)
	if typeEq(e.typ, to) {
		return e.code
	}
	f.checkProcShape(n, e, to)
	if e.view != "" && isAny(to) {
		return e.view // the value itself, not a converted copy
	}
	// `untyped?` is just untyped: its nil is Ruby's nil.
	if o, ok := e.typ.(TOpt); ok && isAny(o.Elem) {
		e = expr{code: "Opt(" + e.code + ")", typ: TAny{}}
		if isAny(to) {
			return e.code
		}
	}
	if isClass(e.typ, "Bignum") && takesInteger(to) { // `io.gets(2**128)`: MRI's RangeError, raised where it converts (decision 372)
		e = expr{code: "rbBignumToInt(" + e.code + ")", typ: f.cls("Integer")}
	}
	if u, ok := e.typ.(TUnion); ok {
		if code, done := f.coerceFromUnion(n, e, u, to); done {
			return code
		}
	}
	switch to := to.(type) {
	case TUnion:
		return f.coerceToUnion(n, e, to)
	case TAny:
		return f.coerceToAny(e)
	case TOpt:
		return f.coerceToOpt(n, e, to)
	case TClass:
		return f.coerceClass(n, e, to)
	case TVar:
		return f.coerceToVar(n, e, to)
	case TTuple:
		if isAny(e.typ) || isClass(e.typ, "Array") { // `T.cast(pair, [A, B])`: an Array (or untyped holding one) as a tuple (rbAs, decision 22)
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), f.coerceToAny(e), to.String()))
		}
		if from, ok := e.typ.(TTuple); ok && len(from.Elems) == len(to.Elems) && f.c.goType(from) != f.c.goType(to) {
			// `T.cast(pair, [T::Array[Formula], ...])` of a `[Array[Formula | Keg], ...]`: each element converts
			t := f.newTmp()
			f.emit("%s := %s", t, e.code)
			codes := make([]string, len(to.Elems))
			for i := range to.Elems {
				el := expr{code: fmt.Sprintf("%s.F%d", t, i), typ: from.Elems[i]}
				if f.c.goType(from.Elems[i]) != f.c.goType(to.Elems[i]) { // a cast may narrow: checked at run time, as from untyped
					el = expr{code: f.coerceToAny(el), typ: TAny{}}
				}
				codes[i] = f.coerce(n, el, to.Elems[i])
			}
			return f.c.goType(to) + "{" + strings.Join(codes, ", ") + "}"
		}
	case TFunc, TNil, TVoid: // converted by the general rules below
	}
	return e.code
}

// coerceToAny boxes e as untyped code sees it.
func (f *fctx) coerceToAny(e expr) string {
	switch {
	case isOpt(e.typ) && e.code == "nil": // `T.let(nil, T.nilable(X))`: Opt(nil) could not infer its T
		return "nil"
	case isOpt(e.typ):
		switch e.typ.(TOpt).Elem.(type) {
		case TVar, TOpt: // E? with E = T?: Opt leaves the inner box
			return "rbUnbox(Opt(" + e.code + "))"
		case TAny, TClass, TFunc, TNil, TTuple, TUnion, TVoid: // a plain T?: Opt boxes it below
		}
		return f.c.optAny(e.code, e.typ)
	case e.lit:
		return f.c.goType(e.typ) + "(" + e.code + ")"
	}
	if _, ok := e.typ.(TTuple); ok {
		return e.code + "._ToAny()" // untyped code sees an Array (decision 22)
	}
	if leafRecv(e.typ) && e.code != "self" && !e.typ.(TClass).C.byValue { // self is never nil; a value is never nil (decision 441)
		return "rbPtrAny[" + e.typ.(TClass).C.Name + "](" + e.code + ")"
	}
	return e.code
}

// coerceToVar is e as a type variable's value: as it is, or asserted in gem code.
func (f *fctx) coerceToVar(n parser.Node, e expr, to TVar) string {
	if ec, ok := e.typ.(TClass); ok && to.Name == "Self" && f.lenient() && ec.C.isStruct() { // a gem's `self.class.new(...)` as self's type
		return "any(" + e.code + ").(Self)"
	}
	if f.c.goType(e.typ) == "any" && to.Name != "Self" && f.lenient() && f.c.goType(to) == to.Name { // a gem's untyped (or Object) value for a type parameter (Hash's V)
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", to.Name, e.code, to.Name))
	}
	return e.code
}

// coerceToUnion is coerce into a union (decision 150): a member's value is
// already one (boxed as untyped code sees it, but a tuple keeps its Go type
// so a member switch finds it), an untyped value is checked at run time,
// and anything else is a compile error.
func (f *fctx) coerceToUnion(n parser.Node, e expr, to TUnion) string {
	switch t := e.typ.(type) {
	case TAny:
		return f.noteConv(n, f.c.unionFn(to)+"("+e.code+")")
	case TUnion:
		if fits(t, to) {
			if f.sameGoMembers(t, to) {
				return e.code
			}
			// a member of another instantiation (Array[Integer] into Array[untyped]):
			// the target's checker converts it, or its type switch misses it
			return f.c.unionFn(to) + "(" + e.code + ")"
		}
		if f.lenient() { // a gem's wider union: checked on arrival, as its other unions are (decisions 233, 241)
			return f.noteConv(n, f.c.unionFn(to)+"("+e.code+")")
		}
	case TNil:
		if fits(t, to) || f.lenient() { // a gem's nil where its sig says non-nil: passed on, as MRI without Sorbet's run-time checks (decision 241)
			return "nil"
		}
	case TTuple:
		if fits(t, to) {
			return "any(" + e.code + ")"
		}
	case TVar:
		return f.varToUnion(n, e, t, to)
	case TClass, TFunc, TOpt, TVoid:
		if fitsValue(e, to) || e.lit && slices.ContainsFunc(to.Members, func(m Type) bool { return fitsValue(e, m) }) {
			if e.lit && isClass(e.typ, "Integer") && !slices.ContainsFunc(to.Members, func(m Type) bool { return isClass(m, "Integer") }) {
				e.typ = f.cls("Float") // 1 where Float | String is expected is 1.0
			}
			if m := f.instantiationMember(e.typ, to); m != nil {
				return f.coerce(n, e, m)
			}
			if o, ok := e.typ.(TOpt); ok && f.instantiationMember(o.Elem, to) != nil {
				return f.c.unionFn(to) + "(" + f.coerce(n, e, TAny{}) + ")" // nil, or the element converted
			}
			return f.coerce(n, e, TAny{})
		}
		for _, m := range to.Members { // through the first member e converts to (decision 179)
			if mc, ok := m.(TClass); ok {
				if code, ok := f.implicitConv(n, e, mc); ok {
					return f.coerce(n, expr{code: code, typ: mc}, to)
				}
			}
		}
		if f.lenient() { // a gem's other union, or a T? where nil is not a member: checked on arrival (decisions 225, 233)
			return f.c.unionFn(to) + "(" + f.coerce(n, e, TAny{}) + ")"
		}
	}
	return f.unionMismatch(n, e, to)
}

// sameGoMembers reports whether every member of from is held in to as the
// same Go type, so a value of from is already one of to's switch cases.
func (f *fctx) sameGoMembers(from, to TUnion) bool {
	goT := func(m Type) string {
		if isNil(m) {
			return "nil"
		}
		return f.c.goType(m)
	}
	return !slices.ContainsFunc(from.Members, func(m Type) bool {
		g := goT(m)
		return !slices.ContainsFunc(to.Members, func(t Type) bool { return goT(t) == g })
	})
}

// instantiationMember is the member of u a generic value of another
// instantiation fits (Array[Integer] for Array[untyped]): it must be
// converted to that member's Go type, or the member switch misses it. nil
// when a member already has t's Go type, or none is t's class.
func (f *fctx) instantiationMember(t Type, u TUnion) Type {
	tc, ok := t.(TClass)
	if !ok || len(tc.Args) == 0 {
		return nil
	}
	g := f.c.goType(t)
	if slices.ContainsFunc(u.Members, func(m Type) bool { return f.c.goType(m) == g }) {
		return nil
	}
	for _, m := range u.Members {
		if mc, ok := m.(TClass); ok && mc.C == tc.C && memberFits(t, m) {
			return m
		}
	}
	return nil
}

// coerceFromUnion is coerce out of a union (decision 150). Untyped code
// gets the value as it is; a type every member fits gets it asserted; a
// Boolean position tests truthiness, as for untyped. Anything else needs
// the union narrowed first, as T? does (decision 20). done is false when
// the target is a union, which coerceToUnion handles.
func (f *fctx) coerceFromUnion(n parser.Node, e expr, u TUnion, to Type) (string, bool) {
	switch to := to.(type) {
	case TUnion:
		return "", false
	case TAny, TVar, TNil: // nil is an any too: a local first typed nil takes the value as it is, as for T?
		if slices.ContainsFunc(u.Members, func(m Type) bool { _, ok := m.(TTuple); return ok }) {
			return f.c.unionFn(u) + "Out(" + e.code + ")", true
		}
		return e.code, true
	case TClass:
		if to.C.RubyName == "Boolean" {
			return "Boolean(rbTruthy(" + e.code + "))", true
		}
		if fits(u, to) {
			if f.c.goType(to) == "any" {
				return e.code, true
			}
			return e.code + ".(" + f.c.goType(to) + ")", true
		}
	case TOpt:
		if fits(u, to) {
			return f.noteConv(n, f.c.optOf(to.Elem, e.code)), true
		}
	case TFunc, TTuple, TVoid:
	}
	if f.lenient() { // a gem's union where one member is wanted: checked at run time (decision 172)
		if c, ok := to.(TClass); ok {
			return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(c), e.code, c.String())), true
		}
		if o, ok := to.(TOpt); ok {
			return f.noteConv(n, f.c.optOf(o.Elem, e.code)), true
		}
	}
	f.errorf(n, "%s where %s is expected; narrow it first (`is_a?`, `case`/`when`, `if x`)", u, to)
	return "", true
}

// truthiness is a value where a Boolean is expected that is not one: Ruby tests truthiness.
func (f *fctx) truthiness(e expr) (string, bool) {
	switch {
	case isAny(e.typ):
		return "Boolean(rbTruthy(" + e.code + "))", true // Ruby conditions test truthiness
	case isOpt(e.typ):
		return "Boolean(" + optTruthy(e.code, e.typ) + ")", true // a predicate block's `seen.add?(x)`
	case f.lenient() && !isClass(e.typ, "Boolean") && !isVoid(e.typ):
		// gem code: a predicate block's value is its truthiness (Homebrew's `select { |var| ENV.fetch(var) }`)
		return "Boolean(rbTruthy(" + f.coerceToAny(e) + "))", true
	}
	return "", false
}

// coerceIntoClass is the untyped value converted (rbAs also converts an Array/Hash of another instantiation, rbConv), or a
// tuple as an Array.
func (f *fctx) coerceIntoClass(n parser.Node, e expr, to TClass) (string, bool) {
	if isAny(e.typ) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), e.code, to.String())), true
	}
	if tt, ok := e.typ.(TTuple); ok && to.C.RubyName == "Array" && len(to.Args) == 1 {
		return f.tupleIntoArray(n, e, tt, to), true
	}
	return "", false
}

// checkProcShape: a Proc of another shape is another Go func type, with no conversion between them (decision 416).
func (f *fctx) checkProcShape(n parser.Node, e expr, to Type) {
	ef, ok := stripOpt(e.typ).(TFunc)
	if !ok {
		return
	}
	if tf, ok := stripOpt(to).(TFunc); ok && f.c.goType(ef) != f.c.goType(tf) {
		f.errorf(n, "a Proc %s where a Proc %s is expected; their parameters or result differ", ef, tf)
	}
}

// tupleIntoArray is a tuple where an Array is expected: an Array of its fields when each fits the element type, else a
// compile error (net-ftp's `host.split(".") + port.divmod(256)`, an [Integer, Integer] for Array[String]: gem code then
// calls + at run time, decision 416). Go would reject the tuple as it is.
func (f *fctx) tupleIntoArray(n parser.Node, e expr, tt TTuple, to TClass) string {
	et := to.Args[0]
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, e.code)
	parts := make([]string, len(tt.Elems))
	for i, el := range tt.Elems {
		if !isAny(et) && !fitsValue(expr{typ: el}, et) {
			f.errorf(n, "%s where %s is expected", e.typ, to)
		}
		parts[i] = f.coerce(n, expr{code: fmt.Sprintf("%s.F%d", tmp, i), typ: el}, et)
	}
	return arrayLit(f.c.goType(et), parts)
}

// coerceClass is coerce to a class type.
func (f *fctx) coerceClass(n parser.Node, e expr, to TClass) string {
	if to.C.universal && e.lit {
		return f.coerce(n, e, TAny{}) // Object is Go any: "a" must box as String, not string
	}
	if to.C.RubyName == "Boolean" {
		if code, ok := f.truthiness(e); ok {
			return code
		}
	}
	if code, ok := f.coerceIntoClass(n, e, to); ok {
		return code
	}
	if code, ok := bignumConv(e, to); ok {
		return code
	}
	// Go instantiations are invariant: Array[Integer] where
	// Array[untyped] (or back, or Array[Numeric]: decision 385) is expected is a converted copy.
	if converts(to) && (sameButUntyped(e.typ, to) || widerInstance(e.typ, to)) && f.c.goType(e.typ) != f.c.goType(to) {
		return f.noteConv(n, fmt.Sprintf("rbAs[%s](%s, %q)", f.c.goType(to), e.code, to.String()))
	}
	if code, ok := f.lenientClass(n, e, to); ok {
		return code
	}
	if isOpt(e.typ) {
		return f.optWhereClass(n, e, to)
	}
	if isNil(e.typ) && f.nilAsZero() {
		return "*new(" + f.c.goType(to) + ")"
	}
	if isNil(e.typ) {
		f.errorf(n, "nil where %s is expected", to)
	}
	if code, ok := f.selfConv(n, e, to); ok {
		return code
	}
	if l := f.leafSelf(e, to.C); l.code != e.code {
		return l.code
	}
	if isClass(to, "Integer") && isClass(e.typ, "Float") {
		if f.pass == 2 { // earlier passes may still widen the target
			f.errorf(n, "Float where Integer is expected; convert it (to_i, round, floor)")
		}
	} else if !fitsValue(e, to) {
		if code, ok := f.implicitConv(n, e, to); ok {
			return code
		}
		f.errorf(n, "%s where %s is expected", e.typ, to)
	}
	if isAbstract(to) { // Go any: literals need wrapping, as for untyped
		return f.coerce(n, e, TAny{})
	}
	if ec, ok := e.typ.(TClass); ok && ec.C.hashBase && ec.C.Super == to.C { // a @go_type subclass where its base is wanted: the embedded base
		if f.lenient() && slices.ContainsFunc(to.Args, func(a Type) bool { return !isAny(a) }) { // Array[any] inside, Array[String] wanted (Homebrew's NamedArgs < Array with a fixed Elem): converted at run time
			return fmt.Sprintf("rbAs[%s](any(%s), %q)", f.c.goType(to), embeddedSuper(ec.C, e.code), to.String())
		}
		return embeddedSuper(ec.C, e.code)
	}
	return e.code
}

// coerceArg coerces a call argument to its parameter. A `T | untyped`
// parameter holds typed arguments to T and passes untyped ones as they are.
func (f *fctx) coerceArg(n parser.Node, a expr, p Param, env map[string]Type) string {
	if p.Pending != "" {
		return f.coercePending(p, func() string { q := p; q.Pending = ""; return f.coerceArg(n, a, q, env) })
	}
	if p.Want != nil && !isAny(a.typ) {
		f.probe(func() { f.coerce(n, a, subst(p.Want, env)) }) // for its compile errors only: its statements (a tuple's temporary) would run the argument twice
	}
	t := subst(p.Type, env)
	code := f.coerce(n, a, t)
	if c := boxedLeafOpt(p.Type, env); c != nil && code != "nil" {
		return "rbBoxNil[" + c.objType() + "](" + code + ")" // a generic V? with V = *C (or CI) is Go **C (or *CI) (decisions 360, 378)
	}
	if boxedValOpt(p.Type, env) {
		if strings.HasSuffix(code, "]{}") {
			return "nil"
		}
		return "rbOptToPtr(" + code + ")" // a generic V? with V = Integer is Go *Integer (decision 396)
	}
	// A type variable bound to untyped gets an explicit any(...), or Go infers
	// it from the argument's own type (E = String for Array[untyped]#include?("a")).
	if _, ok := p.Type.(TVar); ok && isAny(t) && !isOpt(a.typ) && !isVoid(a.typ) && f.c.goType(a.typ) != "any" {
		return "any(" + code + ")"
	}
	// A literal for a type variable is an untyped Go constant, from which Go
	// would infer string or int, not String or Integer: name its type.
	if _, ok := p.Type.(TVar); ok && a.lit && typeEq(a.typ, t) && code == a.code {
		return f.c.goType(t) + "(" + code + ")"
	}
	return code
}

func widerInstance(t Type, to TClass) bool {
	c, ok := t.(TClass)
	return ok && c.C == to.C && fits(t, to)
}

// converts reports whether values of t convert between instantiations
// (rbAs): Array and Hash, which every instantiation of implements
// _to_any and rbFrom.
func converts(t TClass) bool {
	return len(t.Args) > 0 && t.C.lookup("_to_any") != nil
}

// nilAsZero reports where a nil gives T's zero value instead of failing. Ivar discovery reads an ivar no write typed
// yet as nil (decision 148), and `@root = @root ? meld(@root, n) : n` passes it to meld's typed parameter: failing would
// lose the write that types @root (decision 307); discovery's output is discarded, and the emitting pass still reports
// a real nil. ponytail: a gem override typed by its parent (concurrent-ruby's CachedThreadPool#ns_initialize ends in an
// `if` that is nil on MRI) gives T's zero value where it gives nil; make the slot T? with an adapter to fix (decision 251).
func (f *fctx) nilAsZero() bool {
	return f.discover || f.lenient() && f.m != nil && f.m.inherited != nil && f.closures == 0
}

// sameButUntyped reports whether a and b differ only where one of them is
// untyped (Hash[Symbol, Integer] and Hash[Symbol, untyped]).
func sameButUntyped(a, b Type) bool {
	if isAny(a) || isAny(b) || typeEq(a, b) {
		return true
	}
	switch a := a.(type) {
	case TClass:
		b, ok := b.(TClass)
		if !ok || a.C != b.C || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !sameButUntyped(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case TOpt:
		b, ok := b.(TOpt)
		return ok && sameButUntyped(a.Elem, b.Elem)
	case TAny, TFunc, TNil, TTuple, TUnion, TVar, TVoid: // the same only when typeEq, checked above
	}
	return false
}

// implicitConvs are MRI's implicit conversion protocols: a String
// parameter takes any object answering to_str, and so on (rb_convert_type).
var implicitConvs = map[string]string{"String": "to_str", "Integer": "to_int", "Array": "to_ary", "Hash": "to_hash"}

// selfConv is self converted where its class does not fit, as implicitConv does for a class: `File.basename(self)`
// in a Pathname takes its to_path (decision 289). Self is generic, so fits alone would pass it unconverted.
func (f *fctx) selfConv(n parser.Node, e expr, to TClass) (string, bool) {
	if !isSelfVar(e.typ) || f.owner == nil || fits(TClass{C: f.owner}, to) {
		return "", false
	}
	return f.implicitConv(n, e, to)
}

// implicitConv is e converted to `to` through its class's conversion
// method, chosen at compile time from e's static class (decision 179).
func (f *fctx) implicitConv(n parser.Node, e expr, to TClass) (string, bool) {
	from, ok := e.typ.(TClass)
	if isSelfVar(e.typ) && f.owner != nil { // self in a struct class's method: its class's protocols (a Pathname's to_path)
		from, ok = TClass{C: f.owner}, true
	}
	conv := implicitConvs[to.C.RubyName]
	if !ok || conv == "" {
		return "", false
	}
	if conv == "to_str" && f.resolve(from, conv) == nil && f.resolve(from, "to_path") != nil {
		conv = "to_path" // a path object where a String is taken: MRI's path parameters call to_path first (rugaroo types paths as String)
	} else if from.C.GoType != "" && conv != "to_str" { // a struct class: a core value type's own rules stand (Float is not an Integer); a Pathname's to_path is a path's (decision 253), and a to_str it is given (Homebrew's `alias to_str to_s`) converts (decision 258)
		return "", false
	}
	if f.resolve(from, conv) == nil {
		return "", false
	}
	r := f.genMethodCall(n, e, conv, nil, nil)
	if !r.noreturn && !fitsValue(r, to) {
		return "", false
	}
	return f.coerce(n, r, to), true
}

// structValued: t's Go values are interfaces of struct classes (a class, a union of classes, a class object) or nil.
func structValued(t Type) bool {
	switch t := t.(type) {
	case TNil:
		return true
	case TClass:
		return t.C.isStruct() && !t.C.universal && len(t.C.TypeParams) == 0
	case TUnion:
		return !slices.ContainsFunc(t.Members, func(m Type) bool { return !structValued(m) })
	case TAny, TFunc, TOpt, TTuple, TVar, TVoid:
	}
	return false
}

// coerceToOpt is e as a T?: nil, a T? as it is, untyped converted, else T boxed (a leaf's *C is its C?, decision 360).
func (f *fctx) coerceToOpt(n parser.Node, e expr, to TOpt) string {
	switch {
	case isNil(e.typ):
		if valOpt(to) {
			return f.c.goType(to) + "{}"
		}
		return "nil"
	case isOpt(e.typ):
		if valOpt(to) && !valOpt(e.typ) { // ponytail: a *T box of another class where Integer?/Float? is wanted
			return "rbOptFromPtr(" + e.code + ")"
		}
		if from, ok := e.typ.(TOpt); ok && leafOpt(from) && leafRecv(from.Elem) && leafOpt(to) && !leafRecv(to.Elem) {
			return "rbIfaceOf[" + f.c.goType(to.Elem) + "](" + e.code + ")" // a nil *C into an interface would be a typed nil, not nil
		}
		return e.code
	case isAny(e.typ):
		return f.noteConv(n, f.c.optOf(to.Elem, e.code))
	}
	if leafOpt(to) {
		return f.coerce(n, e, to.Elem)
	}
	if valOpt(to) {
		return "rbSome[" + f.c.goType(to.Elem) + "](" + f.coerce(n, e, to.Elem) + ")"
	}
	return "Ref[" + f.c.goType(to.Elem) + "](" + f.coerce(n, e, to.Elem) + ")"
}
