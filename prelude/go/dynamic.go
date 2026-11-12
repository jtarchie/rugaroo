//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"strconv"
)

// How a dynamic call was made, as MRI tells them apart: with a receiver
// (public methods only), without one or through send (private methods
// too), or as a bare name (NameError when missing).
const (
	rbCall = iota
	rbFCall
	rbVCall
)

// rbArity raises MRI's ArgumentError for a wrong argument count.
func rbArity(given, min, max int) {
	if given < min || max >= 0 && given > max {
		rbArityFail(given, min, max) // out of line, so the check inlines into every Dyn wrapper
	}
}

func rbArityFail(given, min, max int) {
	expected := strconv.Itoa(min)
	switch {
	case max < 0:
		expected += "+"
	case max != min:
		expected += ".." + strconv.Itoa(max)
	}
	panic(NewArgumentError(Ref(String(fmt.Sprintf("wrong number of arguments (given %d, expected %s)", given, expected)))))
}

// rbNumMixed reports an Integer or Float argument of the other class than
// self (an Integer or a Float).
func rbNumMixed(self any, args []any) bool {
	_, selfInt := self.(Integer)
	for _, a := range args {
		switch a.(type) {
		case Integer:
			if !selfInt {
				return true
			}
		case Float:
			if selfInt {
				return true
			}
		}
	}
	return false
}

// rbNum is an Integer or a Float as one Go type: Comparable's methods run
// on it when given both, so clamp returns the winning argument itself.
type rbNum struct{ v any }

func (a rbNum) Op_cmp(b rbNum) Integer { return rbCmp(a.v, b.v) }

func (a rbNum) Op_lt(b rbNum) Boolean { return rbCmp(a.v, b.v) < 0 }

// rbConv is v as a T: v itself, nil for untyped, an Array or Hash of
// another instantiation converted (rbFrom), or an Array as a tuple
// (_FromAny, decision 22). Go instantiations are invariant, so
// Array[Integer] where Array[untyped] is expected, or back, is a copy
// whose elements are converted in turn. An Integer where a Float is
// expected widens (MRI's coerce: Float's operators take Integers).
// ponytail: a T? element (*T) is not converted from its value; box it via OptOf when needed.
func rbConv[T any](v any) (T, bool) {
	if t, ok := v.(T); ok {
		return t, true
	}
	if n, ok := v.(Integer); ok {
		if t, ok := any(Float(n)).(T); ok {
			return t, true
		}
	}
	var zero T
	if t, ok := rbOptFrom[T](zero, v); ok { // a scalar's T?: nil, or the value boxed
		return t, true
	}
	if v == nil {
		_, untyped := any(&zero).(*any)
		return zero, untyped
	}
	if c, ok := any(zero).(interface{ rbFrom(v any) (T, bool) }); ok {
		return c.rbFrom(v)
	}

	if tup, ok := any(zero).(interface{ _FromAny(any) (T, bool) }); ok {
		return tup._FromAny(v)
	}
	return zero, false
}

// rbOptFrom converts an untyped value to a scalar's T? where generic code has only the pointer type: a converted
// collection's String? values (Homebrew's `ruby_source_checksum: Hash[Symbol, String?]` from JSON data).
func rbOptFrom[T any](zero T, v any) (T, bool) {
	var out any
	var ok bool
	switch any(zero).(type) {
	case *String:
		out, ok = rbOptScalar[String](v)
	case *Integer:
		out, ok = rbOptScalar[Integer](v)
	case *Float:
		out, ok = rbOptScalar[Float](v)
	case rbOpt[Integer]:
		out, ok = rbOptScalarV[Integer](v)
	case rbOpt[Float]:
		out, ok = rbOptScalarV[Float](v)
	case *Symbol:
		out, ok = rbOptScalar[Symbol](v)
	case *Boolean:
		out, ok = rbOptScalar[Boolean](v)
	default:
		out, ok = rbOptClassFrom(any(zero), v)
	}
	if !ok {
		return zero, false
	}
	t, _ := out.(T)
	return t, true
}

// rbOptScalarV is rbOptScalar for an rbOpt (decision 396).
func rbOptScalarV[T any](v any) (rbOpt[T], bool) {
	if v == nil {
		return rbOpt[T]{}, true
	}
	x, ok := rbConv[T](v)
	return rbOpt[T]{x, ok}, ok
}

func rbOptScalar[T any](v any) (*T, bool) {
	if p, ok := v.(*T); ok {
		return p, true
	}
	if v == nil {
		return nil, true
	}
	x, ok := rbConv[T](v)
	if !ok {
		return nil, false
	}
	return &x, true
}

func rbRest[T any](args []any, from int, want string) []T {
	out := make([]T, 0, len(args))
	for i := from; i < len(args); i++ {
		out = append(out, rbAs[T](args[i], want))
	}
	return out
}

// rbDescribe names a value the way MRI's messages do.
func rbDescribe(v any) string {
	switch r := v.(type) {
	case nil:
		return "nil"
	case Boolean:
		return strconv.FormatBool(bool(r))
	case rbModule:
		return r._Kind() + " " + string(r.Name())
	}
	return "an instance of " + rbClassName(v)
}

// rbNoMethod is MRI's error for a missing method; a bare `name` (a
// "vcall": no receiver, arguments or parentheses) is a NameError.
func rbNoMethod(name string, recv any, vcall bool) any {
	if vcall {
		e := NewNameError(Ref(String("undefined local variable or method '" + name + "' for " + rbDescribe(recv))))
		e.__SetCall(Symbol(name), recv)
		return e
	}
	e := NewNoMethodError(Ref(String("undefined method '" + name + "' for " + rbDescribe(recv))))
	e.__SetCall(Symbol(name), recv)
	return e
}

// rbPrivateMethod is MRI's error for a private method called with a receiver.
func rbPrivateMethod(name string, recv any) any {
	return NewNoMethodError(Ref(String("private method '" + name + "' called for " + rbDescribe(recv))))
}

// rbDynKw takes a call's keywords off an untyped call's arguments for a
// keyword method: a trailing Hash whose keys are all Symbols, as Ruby 2
// read one (the call site cannot tell keywords from a last positional Hash),
// unless the method's required positional parameters need it.
func rbDynKw(args []any, req int) (*Hash[Symbol, any], []any) {
	out := NewHash[Symbol, any]()
	if len(args) <= req {
		return out, args
	}
	h, ok := rbUnbox(args[len(args)-1]).(interface{ _ToAny() *Hash[any, any] })
	if !ok {
		return out, args
	}
	all := h._ToAny()
	for k, hv := range rbHashPairs(all) {
		s, ok := rbUnbox(k).(Symbol)
		if !ok {
			return NewHash[Symbol, any](), args
		}
		Hash___Set(out, s, hv)
	}
	return out, args[:len(args)-1]
}

// rbAt is xs[i], or nil past the end: a loose proc call's missing argument (decision 172).
// rbYieldOne is a splatted yield's values for a block of one Go parameter: the value when there is one, nil for none,
// else an Array, which a block of several parameters spreads (decision 444).
func rbYieldOne(xs []any) any {
	switch len(xs) {
	case 0:
		return nil
	case 1:
		return xs[0]
	}
	return &Array[any]{s: xs}
}

func rbAt(xs []any, i int) any {
	if i < len(xs) {
		return xs[i]
	}
	return nil
}
