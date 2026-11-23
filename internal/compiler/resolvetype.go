package compiler

import (
	"slices"
	"strings"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

type typeScope struct {
	class     *Class
	lex       []*Class // lexical scope for constant names
	methodTPs []string
	srcPos
}

func (c *Compiler) resolveType(t rbs.Type, sc typeScope) Type {
	switch t := t.(type) {
	case rbs.Name:
		return c.resolveNameType(t, sc)
	case rbs.Bool:
		return TClass{C: c.classes["Boolean"]}
	case rbs.Singleton:
		cls, _ := c.lookupName(sc.lex, t.Name)
		if cls == nil {
			c.errorAt(sc.srcPos, "unknown type %s", t.Name)
		}
		if cls.IsModule && c.gemFile(sc.File) { // Sorbet's T::Class[Downloadable] is any class that includes the module: a class object, not the module's own
			return TClass{C: c.classes["Module"]}
		}
		if cls.meta == nil {
			c.errorAt(sc.srcPos, "%s has no class object type", t)
		}
		return TClass{C: cls.meta}
	case rbs.Optional:
		return optOf(c.resolveType(t.Elem, sc))
	case rbs.Tuple:
		elems := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			elems[i] = c.resolveType(e, sc)
		}
		if (len(elems) < 2 || len(elems) > maxTuple) && c.gemFile(sc.File) {
			return TClass{C: c.classes["Array"], Args: []Type{TAny{}}} // gem code: an Array of untyped
		}
		if len(elems) < 2 || len(elems) > maxTuple {
			c.errorAt(sc.srcPos, "only 2- to %d-tuples are supported", maxTuple)
		}
		return TTuple{Elems: elems}
	case rbs.Self:
		if sc.class == nil {
			c.errorAt(sc.srcPos, "`self` type outside a class")
		}
		return TVar{Name: "Self"}
	case rbs.Instance: // the defining class here; a class method's call on a subclass narrows its result to that class (instanceResult, decision 404)
		if sc.class == nil {
			c.errorAt(sc.srcPos, "`instance` type outside a class")
		}
		if sc.class.instanceParam {
			return TVar{Name: instanceTP}
		}
		if sc.class.metaOf != nil {
			return TClass{C: sc.class.metaOf}
		}
		return TClass{C: sc.class}
	case rbs.Void, rbs.Bot:
		return TVoid{}
	case rbs.Nil:
		return TNil{}
	case rbs.Untyped:
		return TAny{}
	case rbs.Union:
		return c.resolveUnion(t, sc)
	case rbs.Intersection:
		return c.resolveIntersection(t, sc)
	case rbs.Proc:
		return c.resolveProc(t, sc)
	case rbs.Literal:
		// A literal is a member of its base class (decision 12); the value
		// matters only to select an overload arm at the call site.
		return TClass{C: c.classes[string(t.Kind)]}
	case rbs.Record:
		return c.resolveRecord(t, sc)
	}
	c.errorAt(sc.srcPos, "unsupported type %s", t)
	return nil
}

// resolveUnion resolves `A | B` (decision 150). Members are what a Go type
// switch can test: classes, tuples, procs and nil.
func (c *Compiler) resolveUnion(t rbs.Union, sc typeScope) Type {
	ms := make([]Type, len(t.Elems))
	for i, e := range t.Elems {
		ms[i] = c.resolveType(e, sc)
		switch m := stripOpt(ms[i]).(type) {
		case TVar, TVoid:
			if c.gemFile(sc.File) {
				return TAny{} // gem code: the union is untyped (decision 172); a method's return keeps the members for its callers (retUnion, decision 321)
			}
			c.errorAt(sc.srcPos, "a union member must be a class, tuple, proc or nil, not %s: %s", m, t)
		case TClass:
			if m.C.IsModule && !m.C.universal { // ponytail: a module member needs an includer check in the type switch; use the module type or untyped until then
				if c.gemFile(sc.File) {
					return TAny{} // gem code: the union is untyped (decision 172)
				}
				c.errorAt(sc.srcPos, "a union member must be a class, not the module %s: %s", m, t)
			}
		case TAny, TFunc, TNil, TOpt, TTuple, TUnion:
		}
	}
	u := unionOf(ms...)
	if tu, ok := u.(TUnion); ok {
		seen := map[string]Type{}
		for _, m := range tu.Members {
			if isNil(m) {
				continue
			}
			g := c.goType(m)
			if o, dup := seen[g]; dup { // a Go type switch cannot tell them apart (Go rejects the duplicate case)
				if c.gemFile(sc.File) {
					return TAny{}
				}
				c.errorAt(sc.srcPos, "union members %s and %s are one Go type (%s): %s", o, m, g, t)
			}
			seen[g] = m
		}
	}
	return u
}

func (c *Compiler) resolveProc(t rbs.Proc, sc typeScope) Type {
	ps := make([]Type, len(t.Params))
	for i, p := range t.Params {
		ps[i] = c.resolveType(p, sc)
	}
	var self Type
	if t.Self != nil { // a Proc instance_eval rebinds (decision 164)
		self = c.resolveType(t.Self, sc)
	}
	return TFunc{Params: ps, Ret: c.resolveType(t.Ret, sc), Proc: true, Self: self}
}

// resolveIntersection: a value of `A & B` is each operand, so any one types it soundly; Go has no struct-and-interface intersection (decision 315).
func (c *Compiler) resolveIntersection(t rbs.Intersection, sc typeScope) Type {
	var pick Type
	for _, e := range t.Elems {
		if n, ok := e.(rbs.Name); ok && rbsInterface(n.Name) {
			continue
		}
		r := c.resolveType(e, sc)
		if k, ok := r.(TClass); ok && !k.C.IsModule {
			return r
		}
		if pick == nil || !isAny(r) && isAny(pick) {
			pick = r
		}
	}
	if pick != nil {
		return pick
	}
	return c.resolveType(t.Elems[0], sc)
}

// rbsInterface reports whether an RBS type name names an interface (`_Each`, `Foo::_Bar`).
func rbsInterface(name string) bool {
	return strings.HasPrefix(name[strings.LastIndex(name, "::")+1:], "_")
}

// scopeTypeVar is the method's or class's type variable t names, or its instance's argument for it, or nil.
func scopeTypeVar(t rbs.Name, sc typeScope) Type {
	if slices.Contains(sc.methodTPs, t.Name) {
		return TVar{Name: t.Name}
	}
	if sc.class == nil {
		return nil
	}
	if slices.Contains(sc.class.TypeParams, t.Name) {
		return TVar{Name: t.Name}
	}
	return instParam(sc.class, t)
}

// resolveNameType resolves a `Foo` / `Foo[A]` type: a type variable or a class.
func (c *Compiler) resolveNameType(t rbs.Name, sc typeScope) Type {
	if v := scopeTypeVar(t, sc); v != nil {
		return v
	}
	lex := sc.lex
	if t.Absolute { // `::Pathname` inside a module that has its own Pathname
		lex = nil
	}
	cls, k := c.lookupName(lex, t.Name)
	if cls == nil && k != nil && k.typeAlias != "" && len(t.Args) == 0 {
		at, err := rbs.ParseType(k.typeAlias)
		if err != nil {
			c.errorAt(k.srcPos, "%v", err)
		}
		return c.resolveType(at, typeScope{lex: k.Scope, srcPos: srcPos{k.File, k.Line}})
	}
	if cls == nil && len(t.Args) == 0 {
		if at, qualified := c.sigAlias(lex, t.Name); at != nil {
			return c.resolveSigAlias(qualified, at, sc)
		}
	}
	if cls == nil && c.gemFile(sc.File) {
		return TAny{} // a lazily-checked sig naming a class outside the closed world: nothing compiled can hold one
	}
	if cls == nil {
		c.errorAt(sc.srcPos, "unknown type %s", t.Name)
	}
	if len(t.Args) == 0 && (cls.RubyName == "Method" || cls.RubyName == "UnboundMethod") {
		return TClass{C: cls, Args: []Type{TAny{}}} // RBS's Method has no signature: called through dyn (decision 141)
	}
	if len(t.Args) > 0 && len(cls.TypeParams) == 0 && c.gemFile(sc.File) {
		t.Args = nil // a Sorbet generic (type_member), erased to untyped (decision 196): its args go too
	}
	if len(t.Args) == 0 && cls.helperOf != nil {
		return c.helperType(cls, sc)
	}
	if len(t.Args) == 0 && len(cls.TypeParams) > 0 && c.gemFile(sc.File) {
		return TClass{C: cls, Args: slices.Repeat([]Type{TAny{}}, len(cls.TypeParams))} // `Hash` with no args: untyped elements
	}
	if len(t.Args) != len(cls.TypeParams) {
		c.errorAt(sc.srcPos, "%s takes %d type args, got %d", cls.Name, len(cls.TypeParams), len(t.Args))
	}
	args := make([]Type, len(t.Args))
	for i, a := range t.Args {
		args[i] = c.resolveType(a, sc)
	}
	return c.classType(cls, args, sc.srcPos)
}

// instParam is an instance's argument for its template's parameter t names, or nil (decision 308).
func instParam(k *Class, t rbs.Name) Type {
	if k.template == nil || len(t.Args) > 0 {
		return nil
	}
	if i := slices.Index(k.template.TypeParams, t.Name); i >= 0 {
		return k.instArgs[i]
	}
	return nil
}

// helperType is a helper named bare in its outer class's code (decision 311): the instance at the enclosing
// instance's arguments, or in the template's own code the helper at its type variables.
func (c *Compiler) helperType(cls *Class, sc typeScope) Type {
	if args := helperArgs(cls, sc.class); args != nil {
		return c.classType(cls, args, sc.srcPos)
	}
	vars := make([]Type, len(cls.TypeParams))
	for i, p := range cls.TypeParams {
		vars[i] = TVar{Name: p}
	}
	return TClass{C: cls, Args: vars}
}

// classType is cls at args: a template's instance when args are closed (decision 308).
func (c *Compiler) classType(cls *Class, args []Type, at srcPos) TClass {
	if cls.isTemplate() {
		var vars []string
		for _, a := range args {
			freeVars(a, &vars)
		}
		if len(vars) == 0 {
			if inst := c.instOf(cls, args, at); inst != nil {
				return TClass{C: inst}
			}
		}
	}
	return TClass{C: cls, Args: args}
}

// resolveRecord resolves a record type to a Hash when every key and value is
// uniform, else Hash[untyped, untyped] (decision 160).
func (c *Compiler) resolveRecord(t rbs.Record, sc typeScope) Type {
	hash := c.classes["Hash"]
	anyHash := TClass{C: hash, Args: []Type{TAny{}, TAny{}}}
	if len(t.Fields) == 0 {
		return anyHash
	}
	var kt, vt Type
	for i, f := range t.Fields {
		k := TClass{C: c.classes["String"]}
		if f.Symbol {
			k = TClass{C: c.classes["Symbol"]}
		}
		v := c.resolveType(f.Value, sc)
		if i == 0 {
			kt, vt = k, v
			continue
		}
		if kt.String() != k.String() || vt.String() != v.String() {
			return anyHash
		}
	}
	return TClass{C: hash, Args: []Type{kt, vt}}
}

// sigAlias finds a `type X = T` a sig file declares, by its qualified name, in the lexical scope or beside an alias being resolved (decision 404).
func (c *Compiler) sigAlias(lex []*Class, name string) (rbs.Type, string) {
	owner, short := "", name
	if i := strings.LastIndex(name, "::"); i >= 0 {
		owner, short = name[:i], name[i+2:]
	}
	var scopes []string // innermost first, top level last
	if owner == "" {
		for i := len(c.resolvingAliases) - 1; i >= 0; i-- {
			if j := strings.LastIndex(c.resolvingAliases[i], "::"); j >= 0 {
				scopes = append(scopes, c.resolvingAliases[i][:j])
			}
		}
		for i := len(lex) - 1; i >= 0; i-- {
			scopes = append(scopes, lex[i].RubyName)
		}
	}
	scopes = append(scopes, owner)
	for _, o := range scopes {
		if d := c.sigs[o]; d != nil {
			for _, a := range d.Aliases {
				if a.Name == short {
					return a.Type, strings.TrimPrefix(o+"::"+short, "::")
				}
			}
		}
	}
	return nil, ""
}

// resolveSigAlias resolves an alias's type; a reference to an alias being resolved is untyped, as rugaroo has no recursive types.
func (c *Compiler) resolveSigAlias(qualified string, at rbs.Type, sc typeScope) Type {
	if slices.Contains(c.resolvingAliases, qualified) {
		return TAny{}
	}
	c.resolvingAliases = append(c.resolvingAliases, qualified)
	defer func() { c.resolvingAliases = c.resolvingAliases[:len(c.resolvingAliases)-1] }()
	return c.resolveType(at, sc)
}
