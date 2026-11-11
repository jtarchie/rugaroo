package compiler

import (
	"maps"
	"slices"
	"strings"
)

// Type is a resolved Ruby type. It is a sealed sum type (decision 119): only the members below
// implement isType, and gochecksumtype makes every type switch on it list
// all of them, so adding a member means visiting every switch it names.
//
//sumtype:decl
type Type interface {
	String() string
	isType()
}

func (TClass) isType() {}
func (TOpt) isType()   {}
func (TTuple) isType() {}
func (TVar) isType()   {}
func (TFunc) isType()  {}
func (TAny) isType()   {}
func (TNil) isType()   {}
func (TVoid) isType()  {}
func (TUnion) isType() {}

type (
	// TClass is an instance of a class or module, possibly with type args.
	TClass struct {
		C    *Class
		Args []Type
	}
	// TOpt is `T?`.
	TOpt struct{ Elem Type }
	// TTuple is `[A, B]`.
	TTuple struct{ Elems []Type }
	// TVar is a type variable: a class/module type param, a method type
	// param, or Self.
	TVar struct{ Name string }
	// TFunc is the type of a block; with Proc, of a Proc value (RBS
	// `^(A) -> R`), which is a Go *func so that it is comparable.
	TFunc struct {
		Params      []Type
		Ret         Type
		Proc        bool
		Src         lambdaSrc  // a lambda typed from its calls: the key its arguments are recorded under (decision 146)
		Rest        bool       // the last param is a `*rest`'s element type; the Go func is variadic (decision 152)
		Self        Type       // non-nil: a Proc (or block) that escaped, taking `self` as a leading Go param (decision 164)
		SelfPending pendingKey // the inference key Self's type is recorded under (decision 164)
	}
	// TAny is `untyped`.
	TAny struct{}
	// TNil is the type of `nil`.
	TNil struct{}
	// TVoid is `void`: no value.
	TVoid struct{}
	// TUnion is `A | B` (decision 150): a Go `any` whose classes the
	// compiler knows. Built only by unionOf, so it is normalized: two or
	// more members, none a union, T? or untyped, nil (TNil) last if
	// present, the rest sorted by String().
	TUnion struct{ Members []Type }
)

func (t TClass) String() string {
	if t.C.metaOf != nil {
		return "singleton(" + t.C.metaOf.RubyName + ")"
	}
	if t.C.template != nil {
		return instKey(t.C.template, t.C.instArgs)
	}
	if len(t.Args) == 0 {
		return t.C.RubyName
	}
	return t.C.RubyName + "[" + joinTypes(t.Args) + "]"
}
func (t TOpt) String() string   { return t.Elem.String() + "?" }
func (t TTuple) String() string { return "[" + joinTypes(t.Elems) + "]" }
func (t TVar) String() string   { return t.Name }
func (t TFunc) String() string {
	ps := joinTypes(t.Params)
	if t.Self != nil {
		if ps != "" {
			ps = t.Self.String() + ", " + ps
		} else {
			ps = t.Self.String()
		}
	}
	if t.Rest {
		ps = joinTypes(t.Params[:len(t.Params)-1])
		if t.Self != nil {
			ps = t.Self.String() + ", " + ps
		}
		if len(t.Params) > 1 {
			ps += ", "
		}
		ps += "*" + t.Params[len(t.Params)-1].String()
	}
	return "^(" + ps + ") -> " + t.Ret.String()
}
func (TAny) String() string  { return "untyped" }
func (TNil) String() string  { return "nil" }
func (TVoid) String() string { return "void" }
func (t TUnion) String() string {
	parts := make([]string, len(t.Members))
	for i, m := range t.Members {
		parts[i] = m.String()
	}
	return strings.Join(parts, " | ")
}

func joinTypes(ts []Type) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}

func isVoid(t Type) bool {
	switch t.(type) {
	case TVoid, TNil, nil:
		return true
	case TAny, TClass, TFunc, TOpt, TTuple, TUnion, TVar: // values, not the absence of one
	}
	return false
}

func isAny(t Type) bool { _, ok := t.(TAny); return ok }

// holdsAny reports whether t is or contains untyped.
func holdsAny(t Type) bool {
	switch t := t.(type) {
	case TAny:
		return true
	case TOpt:
		return holdsAny(t.Elem)
	case TClass:
		return slices.ContainsFunc(t.Args, holdsAny)
	case TTuple:
		return slices.ContainsFunc(t.Elems, holdsAny)
	case TFunc:
		return slices.ContainsFunc(t.Params, holdsAny) || holdsAny(t.Ret) || holdsAny(t.Self)
	case TUnion:
		return slices.ContainsFunc(t.Members, holdsAny)
	case TNil, TVar, TVoid: // leaves with nothing untyped inside
	}
	return false
}
func isOpt(t Type) bool { _, ok := t.(TOpt); return ok }
func isNil(t Type) bool { _, ok := t.(TNil); return ok }

// isAbstract: Object and modules are Go `any` (goType), so their values' classes are known only at run time.
func isAbstract(t Type) bool {
	c, ok := t.(TClass)
	return ok && (c.C.universal || c.C.IsModule)
}

func isNumeric(t Type) bool { return isClass(t, "Integer") || isClass(t, "Float") }

func classOf(t Type) *Class {
	if c, ok := t.(TClass); ok {
		return c.C
	}
	return nil
}

func isClass(t Type, name string) bool {
	c := classOf(t)
	return c != nil && c.Name == name
}

// optOf is `t?`. Ruby has one nil, so T?? is T? and untyped? is untyped.
func optOf(t Type) Type {
	if isOpt(t) || isAny(t) {
		return t
	}
	if u, ok := t.(TUnion); ok {
		return unionOf(u, TNil{}) // a union holds nil as a member, not in a box
	}
	return TOpt{Elem: t}
}

// stripOpt returns the non-optional part of t.
func stripOpt(t Type) Type {
	if o, ok := t.(TOpt); ok {
		return o.Elem
	}
	return t
}

func typeEq(a, b Type) bool {
	switch a := a.(type) {
	case TClass:
		b, ok := b.(TClass)
		if !ok || a.C != b.C || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !typeEq(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case TOpt:
		b, ok := b.(TOpt)
		return ok && typeEq(a.Elem, b.Elem)
	case TTuple:
		b, ok := b.(TTuple)
		if !ok || len(a.Elems) != len(b.Elems) {
			return false
		}
		for i := range a.Elems {
			if !typeEq(a.Elems[i], b.Elems[i]) {
				return false
			}
		}
		return true
	case TVar:
		b, ok := b.(TVar)
		return ok && a.Name == b.Name
	case TFunc:
		b, ok := b.(TFunc)
		if !ok || a.Proc != b.Proc || a.Rest != b.Rest || len(a.Params) != len(b.Params) || !typeEq(a.Ret, b.Ret) || !sameSelf(a.Self, b.Self) {
			return false
		}
		for i := range a.Params {
			if !typeEq(a.Params[i], b.Params[i]) {
				return false
			}
		}
		return true
	case TAny:
		_, ok := b.(TAny)
		return ok
	case TNil:
		_, ok := b.(TNil)
		return ok
	case TVoid:
		_, ok := b.(TVoid)
		return ok
	case TUnion:
		b, ok := b.(TUnion)
		return ok && slices.EqualFunc(a.Members, b.Members, typeEq)
	}
	return false
}

// sameSelf reports whether two TFunc self types (both nil, or equal) match.
func sameSelf(a, b Type) bool {
	return (a == nil) == (b == nil) && (a == nil || typeEq(a, b))
}

// subst replaces type variables according to env.
func subst(t Type, env map[string]Type) Type {
	if len(env) == 0 {
		return t
	}
	switch t := t.(type) {
	case TVar:
		if r, ok := env[t.Name]; ok {
			return r
		}
		return t
	case TClass:
		if len(t.Args) == 0 {
			return t
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = subst(a, env)
		}
		return TClass{C: t.C, Args: args}
	case TOpt:
		return TOpt{Elem: subst(t.Elem, env)} // E? with E = A | B is a box (*any) in the generic Go code; flatOpt opens it at the call

	case TTuple:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = subst(e, env)
		}
		return TTuple{Elems: elems}
	case TFunc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = subst(p, env)
		}
		return TFunc{Params: ps, Ret: subst(t.Ret, env), Proc: t.Proc, Rest: t.Rest, Self: subst(t.Self, env), SelfPending: t.SelfPending, Src: t.Src}
	case TUnion:
		ms := make([]Type, len(t.Members))
		for i, m := range t.Members {
			ms[i] = subst(m, env)
		}
		return unionOf(ms...)
	case TAny, TNil, TVoid: // no type variables inside
	}
	return t
}

// unify binds type variables in pattern so that it matches actual.
// Returns false on a structural mismatch (never an error: Go decides).
func unify(pattern, actual Type, env map[string]Type) bool {
	switch p := pattern.(type) {
	case TVar:
		if bound, ok := env[p.Name]; ok {
			// Prefer the more specific binding: nil upgrades to T? later.
			if isNil(bound) && !isNil(actual) {
				env[p.Name] = actual
			}
			return true
		}
		if _, ok := actual.(TVoid); ok {
			return false
		}
		env[p.Name] = actual
		return true
	case TClass:
		return unifyClass(p, actual, env)
	case TOpt:
		switch a := actual.(type) {
		case TOpt:
			return unify(p.Elem, a.Elem, env)
		case TNil:
			return true
		default:
			return unify(p.Elem, actual, env)
		}
	case TTuple:
		a, ok := actual.(TTuple)
		if !ok {
			return false
		}
		return unifyAll(p.Elems, a.Elems, env)
	case TFunc:
		a, ok := actual.(TFunc)
		if !ok || a.Rest != p.Rest || !unifyAll(p.Params, a.Params, env) {
			return false
		}
		if p.Self != nil && a.Self != nil {
			if !unify(p.Self, a.Self, env) {
				return false
			}
		}
		return unify(p.Ret, a.Ret, env)
	case TUnion:
		// the first member actual unifies with binds its variables; a
		// member-for-member match of two unions binds nothing new
		if _, ok := actual.(TUnion); ok {
			return true
		}
		for _, m := range p.Members {
			try := maps.Clone(env)
			if try == nil {
				try = map[string]Type{}
			}
			if unify(m, actual, try) {
				maps.Copy(env, try)
				return true
			}
		}
		return true
	case TAny, TNil, TVoid: // match anything at this level; Go decides the rest
	}
	return true
}

func unifyClass(p TClass, actual Type, env map[string]Type) bool {
	a, ok := actual.(TClass)
	if !ok {
		return false
	}
	if a.C != p.C {
		// Allow a subclass to match its ancestor's pattern.
		return a.C.isSubclassOf(p.C)
	}
	return unifyAll(p.Args, a.Args, env)
}

func unifyAll(ps, as []Type, env map[string]Type) bool {
	if len(ps) != len(as) {
		return false
	}
	ok := true
	for i := range ps {
		ok = unify(ps[i], as[i], env) && ok
	}
	return ok
}

// freeVars lists the type variables mentioned in t, in order of appearance.
func freeVars(t Type, out *[]string) {
	switch t := t.(type) {
	case TVar:
		for _, n := range *out {
			if n == t.Name {
				return
			}
		}
		*out = append(*out, t.Name)
	case TClass:
		for _, a := range t.Args {
			freeVars(a, out)
		}
	case TOpt:
		freeVars(t.Elem, out)
	case TTuple:
		for _, e := range t.Elems {
			freeVars(e, out)
		}
	case TFunc:
		for _, p := range t.Params {
			freeVars(p, out)
		}
		freeVars(t.Self, out)
		freeVars(t.Ret, out)
	case TUnion:
		for _, m := range t.Members {
			freeVars(m, out)
		}
	case TAny, TNil, TVoid: // no type variables inside
	}
}

// join computes the type of `cond ? a : b`.
func join(a, b Type) (Type, bool) {
	switch {
	case isNil(a) && isNil(b):
		return TNil{}, true
	case isNil(a):
		return optOf(b), true
	case isNil(b):
		return optOf(a), true
	case typeEq(a, b):
		return a, true
	case isAny(a) || isAny(b):
		return TAny{}, true
	case isOpt(a) || isOpt(b):
		// T? with U: join T and U, then re-add the nil, so every order of
		// `x = nil`, `x = Derived.new`, `x = Base.new` gives Base?.
		j, ok := join(stripOpt(a), stripOpt(b))
		if !ok || isAny(j) {
			return j, ok
		}
		return optOf(j), true
	case isUnion(a) || isUnion(b):
		if isModuleType(a) || isModuleType(b) {
			return nil, false // a module member would be a `case any:` arm shadowing the rest; as for a class joined with a module
		}
		return unionOf(a, b), true // a member joins in; another member widens it (decision 150)
	case isClass(a, "Integer") && isClass(b, "Bignum"): // --int-overflow=promote: an Integer local that also holds a Bignum is one (decision 372)
		return b, true
	case isClass(a, "Bignum") && isClass(b, "Integer"):
		return a, true
	case isNumeric(a) && isNumeric(b):
		return unionOf(a, b), true // Integer | Float: each arithmetic call switches on the member (decision 150; was untyped, decision 12)
	}
	return joinClasses(a, b)
}

// joinClasses is join for two different non-nil, typed values: their
// nearest common class, else their union.
func joinClasses(a, b Type) (Type, bool) {
	if t, ok := joinTuples(a, b); ok {
		return t, true
	}
	// Methods of different signatures mix as Method[untyped] (decision 141).
	if ka, _, _ := methodFn(a); ka != "" && classOf(a) == classOf(b) {
		return TClass{C: classOf(a), Args: []Type{TAny{}}}, true
	}
	// Subclass / superclass: pick the ancestor.
	if ca, cb := classOf(a), classOf(b); ca != nil && cb != nil && ca != cb {
		if ca.isSubclassOf(cb) {
			return b, true
		}
		if cb.isSubclassOf(ca) {
			return a, true
		}
		// nearest common superclass, if it is more than Object
		if len(a.(TClass).Args) == 0 && len(b.(TClass).Args) == 0 {
			for k := ca.Super; k != nil && !k.universal; k = k.Super {
				if cb.isSubclassOf(k) {
					return TClass{C: k}, true
				}
			}
		}
	}
	// no common class but Object: the union of both (decision 150)
	if unionable(a) && unionable(b) && (classOf(a) == nil || classOf(a) != classOf(b)) {
		return unionOf(a, b), true
	}
	return nil, false
}

// joinTuples: a re-pushed popped `[node?, :close]` and `[node, :open]` are one `[Node?, Symbol]`, not two unions members of one Go type (decision 384).
func joinTuples(a, b Type) (Type, bool) {
	ta, ok1 := a.(TTuple)
	tb, ok2 := b.(TTuple)
	if !ok1 || !ok2 || len(ta.Elems) != len(tb.Elems) {
		return nil, false
	}
	out := make([]Type, len(ta.Elems))
	for i := range out {
		j, ok := join(ta.Elems[i], tb.Elems[i])
		if !ok || isAny(j) || isUnion(stripOpt(j)) && !isUnion(stripOpt(ta.Elems[i])) && !isUnion(stripOpt(tb.Elems[i])) {
			return nil, false
		}
		out[i] = j
	}
	return TTuple{Elems: out}, true
}

func isUnion(t Type) bool { _, ok := t.(TUnion); return ok }

// isModuleType reports a module type other than a universal one: no union member (decision 150).
func isModuleType(t Type) bool {
	c, ok := t.(TClass)
	return ok && c.C.IsModule && !c.C.universal
}

// unionable reports whether a join may put t in a union: a class other
// than a module or Object (the universal union already), a tuple or a proc.
func unionable(t Type) bool {
	switch t := t.(type) {
	case TClass:
		return !t.C.IsModule && !t.C.universal
	case TTuple:
		return true
	case TFunc:
		return t.Proc
	case TAny, TNil, TOpt, TUnion, TVar, TVoid:
	}
	return false
}

// fits reports whether a value of type t can stand where `to` is expected:
// the same class or a subclass, with type args that fit in turn. Only
// class-vs-class mismatches are rejected; type variables, untyped and
// shapes the codegen converts (tuples, blocks) are left to the caller.
func fits(t, to Type) bool {
	if tu, ok := t.(TUnion); ok {
		switch to.(type) {
		case TAny, TVar, TUnion:
		case TClass, TFunc, TNil, TOpt, TTuple, TVoid:
			return !slices.ContainsFunc(tu.Members, func(m Type) bool { return !memberFits(m, to) }) // every member, or a narrowing is needed
		}
	}
	switch to := to.(type) {
	case TClass:
		c, ok := t.(TClass)
		if !ok {
			return true
		}
		if c.C != to.C {
			return c.C.isSubclassOf(to.C)
		}
		for i := range min(len(c.Args), len(to.Args)) {
			if !fits(c.Args[i], to.Args[i]) {
				return false
			}
			if _, cls := to.Args[i].(TClass); cls && isOpt(c.Args[i]) { // Array[T?] is not an Array[T]: Go's type args are invariant
				return false
			}
		}
	case TOpt:
		return fits(stripOpt(t), to.Elem)
	case TTuple:
		tt, ok := t.(TTuple)
		if !ok || len(tt.Elems) != len(to.Elems) {
			return true
		}
		for i := range tt.Elems {
			if !fits(tt.Elems[i], to.Elems[i]) {
				return false
			}
		}
	case TUnion:
		switch t := t.(type) {
		case TUnion:
			return !slices.ContainsFunc(t.Members, func(m Type) bool { return !fits(m, to) })
		case TOpt:
			return fits(TNil{}, to) && fits(t.Elem, to)
		case TAny: // through the union's checker, as into a class (decision 150)
			return true
		case TClass, TFunc, TNil, TTuple, TVar, TVoid:
		}
		return slices.ContainsFunc(to.Members, func(m Type) bool { return memberFits(t, m) })
	case TAny, TFunc, TNil, TVar, TVoid: // not a class-vs-class check: left to the caller (doc above)
	}
	return true
}

// memberFits is fits for one side of a union, where the lenient answers
// fits gives across kinds (a class where a tuple is expected) would pick
// the wrong member: the kinds must agree.
func memberFits(t, m Type) bool {
	switch m := m.(type) {
	case TNil:
		return isNil(t)
	case TOpt:
		return isNil(t) || memberFits(stripOpt(t), m.Elem)
	case TClass:
		if m.C.universal {
			return true
		}
		_, ok := t.(TClass)
		return ok && fits(t, m)
	case TTuple:
		_, ok := t.(TTuple)
		return ok && fits(t, m)
	case TFunc:
		// the member's Go func type is a switch case: another signature would match none of them
		return typeEq(t, m)
	case TAny, TVar:
		return true
	case TUnion:
		return fits(t, m)
	case TVoid:
	}
	return false
}

// unionOf is the union of ts, normalized (decision 150): unions and T?
// flatten into members, untyped absorbs everything, Object every class,
// a subclass its superclass (and an includer its module), duplicates
// merge, and one member left is that member (or its T?). nil is a member
// of a union with two or more others, kept last.
func unionOf(ts ...Type) Type {
	var ms []Type
	hasNil, universal := false, Type(nil)
	var add func(t Type) bool
	add = func(t Type) bool {
		switch t := t.(type) {
		case TAny:
			return false
		case TNil, TVoid:
			hasNil = true
		case TOpt:
			hasNil = true
			return add(t.Elem)
		case TUnion:
			for _, m := range t.Members {
				if !add(m) {
					return false
				}
			}
		case TClass:
			if t.C.universal {
				universal = t
			}
			ms = append(ms, t)
		case TFunc, TTuple, TVar:
			ms = append(ms, t)
		}
		return true
	}
	for _, t := range ts {
		if !add(t) {
			return TAny{}
		}
	}
	if universal != nil {
		ms = []Type{universal}
	}
	var out []Type
	for i, m := range ms {
		drop := false
		for j, o := range ms {
			if i == j {
				continue
			}
			if typeEq(m, o) {
				drop = j < i // keep the first of equal members
			} else {
				drop = absorbs(o, m)
			}
			if drop {
				break
			}
		}
		if !drop {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b Type) int { return strings.Compare(a.String(), b.String()) })
	switch len(out) {
	case 0:
		return TNil{}
	case 1:
		if hasNil {
			return optOf(out[0])
		}
		return out[0]
	}
	if hasNil {
		out = append(out, TNil{})
	}
	return TUnion{Members: out}
}

// absorbs reports whether member o makes member m redundant: m's class is
// a strict subclass (or includer) of o's, and neither takes type args.
func absorbs(o, m Type) bool {
	oc, ok1 := o.(TClass)
	mc, ok2 := m.(TClass)
	return ok1 && ok2 && len(oc.Args) == 0 && len(mc.Args) == 0 && oc.C != mc.C && mc.C.isSubclassOf(oc.C)
}

func composeEnv(inner, outer map[string]Type) map[string]Type {
	out := map[string]Type{}
	for k, v := range inner {
		out[k] = subst(v, outer)
	}
	return out
}

func substAll(ts []Type, env map[string]Type) []Type {
	out := make([]Type, len(ts))
	for i, t := range ts {
		out[i] = subst(t, env)
	}
	return out
}

// holdsNil reports whether t admits nil: T?, untyped, or a union with nil.
func holdsNil(t Type) bool {
	if isOpt(t) || isAny(t) || isNil(t) {
		return true
	}
	if u, ok := t.(TUnion); ok {
		return slices.ContainsFunc(u.Members, holdsNil)
	}
	return false
}

// unionHas reports whether t is one of u's members (or u's T? element).
func unionHas(u, t Type) bool {
	if o, ok := u.(TOpt); ok {
		return typeEq(o.Elem, t) || unionHas(o.Elem, t)
	}
	if uu, ok := u.(TUnion); ok {
		return slices.ContainsFunc(uu.Members, func(m Type) bool { return typeEq(m, t) })
	}
	return false
}
