//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type rbConst struct {
	name      string
	value     any
	inherited bool // from a superclass; `inherit = false` skips it
}

type rbModule interface {
	_Kind() string // "class" or "module", for messages
	Name() String
}

// rbConstTable is a class object's generated constant table. Only constant reflection asks for
// it, and no generated interface declares it, so the pruner keeps the tables (and every class
// they name) only in programs that reflect (decision 49).
type rbConstTable interface{ _Consts(name string) []rbConst }

func rbConstName(name any) string {
	switch n := rbUnbox(name).(type) { // a String? held untyped arrives boxed (liquid's `send(method, *args)` with a filter name)
	case Symbol:
		return string(n)
	case String:
		return string(n)
	}
	panic(NewTypeError(Ref(rbInspect(name) + " is not a symbol nor a string")))
}

func rbConstFind(m rbModule, name string, inherit bool) (any, bool) {
	for _, c := range m.(rbConstTable)._Consts(name) {
		if c.name == name && (inherit || !c.inherited) {
			if p, ok := c.value.(rbEvalConst); ok { // a class a string eval defines: there once that eval has run (decision 324)
				if !*p.defined {
					return nil, false
				}
				return p.value, true
			}
			return c.value, true
		}
	}
	return nil, false
}

// rbEvalConst is a constant table's entry for a class a string eval defines at run time (bindata's Int and BitField
// classes): MRI has no such constant until the eval runs, so const_defined? is false and const_get reaches const_missing.
type rbEvalConst struct {
	defined *bool
	value   any
}

// rbConstMissingHook calls a module's const_missing (bindata's IntFactory); set when the program defines one.
var rbConstMissingHook func(m rbModule, name string) (any, bool)

// rbConstRead reads a constant that code may reach before main assigns
// it; MRI raises NameError there.
func rbConstRead[T any](set bool, v T, name string) T {
	if !set {
		panic(NewNameError(Ref(String("uninitialized constant " + name))))
	}
	return v
}

// MRI's rule: uppercase first letter, then identifier (or any non-ASCII) characters.
func rbIsConstName(s string) bool {
	for i, r := range s {
		switch {
		case i == 0:
			if !unicode.IsUpper(r) && !unicode.IsTitle(r) {
				return false
			}
		case r < utf8.RuneSelf && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return s != ""
}

// rbConstResolve walks an "A::B" path from m; the first segment also
// falls back to the top level, as Module#const_get does. Errors are
// raised segment by segment, in MRI's order. On a miss it returns the
// NameError message.
func rbConstResolve(m rbModule, name any, inherit bool) (any, string) {
	return rbConstLookup(m, name, inherit, true)
}

// rbConstLookup is rbConstResolve; missing says whether an absent name reaches const_missing (const_get) or not
// (const_defined?, as MRI's).
func rbConstLookup(m rbModule, name any, inherit, missing bool) (any, string) {
	var parts []string
	var path string
	switch n := name.(type) {
	case Symbol:
		path = string(n)
		parts = []string{path}
	case String:
		path = string(n)
		rest := path
		if len(rest) > 2 && strings.HasPrefix(rest, "::") {
			rest, m = rest[2:], Object_class
		}
		parts = strings.Split(rest, "::")
	default:
		what := string(rbInspect(name))
		if _, ok := name.(Boolean); !ok && name != nil {
			what = rbClassName(name)
		}
		panic(NewTypeError(Ref(String("no implicit conversion of " + what + " into String"))))
	}
	var val any
	for i, p := range parts {
		if p == "" || strings.Contains(p, ":") || (i == len(parts)-2 && parts[i+1] == "") {
			panic(NewNameError(Ref(String("wrong constant name " + path))))
		}
		if i > 0 {
			next, ok := val.(rbModule)
			if !ok {
				panic(NewTypeError(Ref(String(path + " does not refer to class/module"))))
			}
			m = next
		}
		if !rbIsConstName(p) {
			panic(NewNameError(Ref(String("wrong constant name " + p))))
		}
		v, ok := rbConstFind(m, p, inherit)
		if !ok && i == 0 && inherit {
			v, ok = rbConstFind(Object_class, p, true)
		}
		if !ok && missing && rbConstMissingHook != nil {
			v, ok = rbConstMissingHook(m, p)
		}
		if !ok {
			if owner := string(m.Name()); owner != "Object" {
				return nil, "uninitialized constant " + owner + "::" + p
			}
			return nil, "uninitialized constant " + p
		}
		val = v
	}
	return val, ""
}

// rbInstanceTest is a class object's generated _IsInstance (decision 76). It is asked for only
// here, not declared by ModuleI, so the pruner keeps the tests only in programs that use them.
type rbInstanceTest interface{ _IsInstance(v any) bool }

// rbIsInstanceOf is Module#===, and is_a? with a class value.
func rbIsInstanceOf(k, v any) bool {
	t, ok := rbUnbox(k).(rbInstanceTest)
	if !ok {
		panic(NewTypeError(Ref(String("class or module required"))))
	}
	return t._IsInstance(v)
}

// rbDescendants is a class object's generated descendant list, in definition order.
func rbDescendants(k any) []any {
	return k.(interface{ _Descendants() []any })._Descendants()
}

// rbClassVarRead is a class variable first assigned in a method, read
// before any assignment ran: MRI's NameError.
func rbClassVarRead[T any](set bool, v T, name, owner string) T {
	if !set {
		panic(NewNameError(Ref(String("uninitialized class variable " + name + " in " + owner))))
	}
	return v
}

// rbModuleCmp is a class or module's <, <=, > or >= (decision 255): self below other when other is among self's
// ancestors, above when self is among other's; true or false when related, nil when not, TypeError for a non-module.
// orEqual is <= and >=; above is > and >=.
func rbModuleCmp(self, other any, above, orEqual bool) *Boolean {
	if _, ok := other.(interface{ _Kind() string }); !ok {
		panic(NewTypeError(Ref(String("compared with non class/module"))))
	}
	if above {
		self, other = other, self
	}
	anc := func(m any) []any {
		if t, ok := m.(interface{ _Ancestors() []any }); ok {
			return t._Ancestors()
		}
		return nil
	}
	switch {
	case self == other:
		return Ref(Boolean(orEqual))
	case slices.Contains(anc(self), other):
		return Ref(Boolean(true))
	case slices.Contains(anc(other), self):
		return Ref(Boolean(false))
	}
	return nil
}

// rbActiveAncestors is a class object's ancestors without the latent includes (decision 331) its own include calls
// have not made yet. ponytail: a subclass object sees its parent's latent includes only once it includes them itself.
func rbActiveAncestors(self any, all []any) []any {
	latent := self.(interface{ _Latent() []any })._Latent()
	on, _ := rbIvarGet(self, Symbol("@__rugaroo_included")).(*Array[any])
	out := make([]any, 0, len(all))
	for _, k := range all {
		if slices.Contains(latent, k) && (on == nil || !slices.Contains(on.s, k)) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// rbIncludeAtRun is Module#include called at run time (decision 331): a latent include boot saw becomes visible to
// include? and ancestors on this class object; a module the class already has is MRI's no-op; any other is a module
// the closed world cannot add, so it raises.
func rbIncludeAtRun(self, mod any) {
	if t, ok := self.(interface{ _Latent() []any }); ok && slices.Contains(t._Latent(), mod) {
		on, _ := rbIvarGet(self, Symbol("@__rugaroo_included")).(*Array[any])
		if on == nil {
			on = &Array[any]{}
		}
		if !slices.Contains(on.s, mod) {
			on.s = append(on.s, mod)
		}
		rbIvarSet(self, Symbol("@__rugaroo_included"), on)
		return
	}
	if t, ok := self.(interface{ _Ancestors() []any }); ok && slices.Contains(t._Ancestors(), mod) {
		return
	}
	panic(NewNotImplementedError(Ref(String("rugaroo: " + rbClassName(mod) + " is included at run time where the traced run did not include it (decision 331)"))))
}

// rbNilAny is nil itself for a NilClass-typed value, which Go holds as a typed nil pointer (decision 331).
func rbNilAny[T any](_ T) any { return nil }

// rbPtrAny boxes a leaf class's *C (decision 359): a nil one (a gem's sig that did not hold) is Ruby's nil, not a typed nil pointer.
func rbPtrAny[T any](p *T) any {
	if p == nil {
		return nil
	}
	return p
}

// rbRescueMissing is a rescue clause naming a constant no loaded file defines: MRI reads it when an exception reaches
// the clause, and raises NameError (decision 444).
func rbRescueMissing(name string) bool {
	panic(NewNameError(Ref(String("uninitialized constant " + strings.TrimPrefix(name, "::")))))
}
