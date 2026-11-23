//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import "math"

// rbNumTowerLevel is v's place in MRI's coercion order Integer < Rational < Float < BigDecimal < Complex (decision 142), -1 for a non-number.
func rbNumTowerLevel(v any) int {
	switch v.(type) { // one type per case, so a class the program never makes costs nothing (the pruner drops its case)
	case Integer:
		return 0
	case *Rational:
		return 1
	case Float:
		return 2
	case *BigDecimal:
		return 3
	case *Complex:
		return 4
	default: // gocritic rejects a one-case switch, which pruning can leave
		return -1
	}
}

// rbNumOther reports an argument that is a number of another class than self, which Comparable's methods then compare through rbNum.
func rbNumOther(self any, args []any) bool {
	l := rbNumTowerLevel(self)
	for _, a := range args {
		if k := rbNumTowerLevel(rbUnbox(a)); k >= 0 && k != l {
			return true
		}
	}
	return false
}

// rbNumCoerce converts two numbers of different classes to the higher one's, as MRI's coerce; false when they are not both numbers or a BigDecimal meets a Complex.
func rbNumCoerce(a, b any) (any, any, bool) {
	b = rbUnbox(b)
	la, lb := rbNumTowerLevel(a), rbNumTowerLevel(b)
	if la < 0 || lb < 0 || la == lb {
		return nil, nil, false
	}
	hi := a
	if lb > la {
		hi = b
	}
	switch h := hi.(type) {
	case *Rational:
		return rbNumTo(a, 1), rbNumTo(b, 1), true
	case Float:
		return Float(rbNumFloat(a)), Float(rbNumFloat(b)), true
	case *BigDecimal:
		prec := rbBDCoercePrec(h) // as rbBDRel: a fixed precision loses digits of a longer BigDecimal
		return rbBDArg(a, prec), rbBDArg(b, prec), true
	case *Complex:
		return rbNumComplex(a), rbNumComplex(b), la != 3 && lb != 3
	default: // gocritic rejects a one-case switch, which pruning can leave
		return nil, nil, false
	}
}

// rbNumComplex is a real number as a Complex, or a Complex itself.
func rbNumComplex(v any) any {
	switch v.(type) {
	case Integer, Float, *Rational:
		return &Complex{v, Integer(0)}
	}
	return v
}

func rbNumTo(a any, level int) any {
	switch level {
	case 1:
		if i, ok := a.(Integer); ok {
			return rbRat(int64(i), 1)
		}
	case 2:
		return Float(rbNumFloat(a))
	}
	return a
}

func rbNumFloat(a any) float64 {
	switch v := a.(type) {
	case Integer:
		return float64(v)
	case Float:
		return float64(v)
	case *Rational:
		return float64(v.ToF())
	}
	panic(rbConvError(a, "Float"))
}

func rbNumOp(op byte, a, b any) any {
	level := max(rbNumLevel(a), rbNumLevel(b))
	if op == '/' && level == 0 {
		level = 1 // Integer parts divide exactly, as MRI's quo
	}
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	switch level {
	case 0:
		x, y := a.(Integer), b.(Integer)
		switch op {
		case '+':
			return x.Op_plus(y)
		case '-':
			return x.Op_minus(y)
		}
		return x.Op_mul(y)
	case 1:
		x, y := a.(*Rational), b.(*Rational)
		switch op {
		case '+':
			return x.Op_plus(y)
		case '-':
			return x.Op_minus(y)
		case '*':
			return x.Op_mul(y)
		}
		return x.Op_div(y)
	}
	x, y := a.(Float), b.(Float)
	switch op {
	case '+':
		return x + y
	case '-':
		return x - y
	case '*':
		return x * y
	}
	return x / y
}

func rbNumEq(a, b any) bool {
	level := max(rbNumLevel(a), rbNumLevel(b))
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	if level == 1 {
		return a.(*Rational).v.Cmp(&b.(*Rational).v) == 0
	}
	return a == b
}

func rbNumAbs(a any) any {
	switch v := a.(type) {
	case Integer:
		return v.Abs()
	case Float:
		return Float(math.Abs(float64(v)))
	}
	return a.(*Rational).Abs()
}

// rbSummer is MRI's ary_sum/enum_sum: an exact Integer/Rational phase, then
// Kahan-Babuska compensated Float addition once a Float appears.
type rbSummer struct {
	exact    any // Integer or *Rational; nil until the first value
	floating bool
	f, c     float64
}

func (s *rbSummer) add(x any) {
	x = rbUnbox(x)
	if !s.floating {
		if _, isFloat := x.(Float); !isFloat {
			if s.exact == nil {
				s.exact = Integer(0)
			}
			s.exact = rbNumCanon(rbNumOp('+', s.exact, rbNumArg(x)))
			return
		}
		s.floating = true
		if s.exact != nil {
			s.f = rbNumFloat(s.exact)
		}
	}
	s.addFloat(rbNumFloat(rbNumArg(x)))
}

// addFloat is add for a Float, which a typed Float sum calls without boxing.
func (s *rbSummer) addFloat(v float64) {
	switch {
	case math.IsNaN(s.f):
	case math.IsNaN(v):
		s.f = v
	case math.IsInf(v, 0):
		if math.IsInf(s.f, 0) && math.Signbit(v) != math.Signbit(s.f) {
			s.f = math.NaN()
		} else {
			s.f = v
		}
	case math.IsInf(s.f, 0):
	default:
		t := s.f + v
		if math.Abs(s.f) >= math.Abs(v) {
			s.c += (s.f - t) + v
		} else {
			s.c += (v - t) + s.f
		}
		s.f = t
	}
}

func (s *rbSummer) result() any {
	if s.floating {
		return Float(s.f + s.c)
	}
	if s.exact == nil {
		return Integer(0)
	}
	return s.exact
}

// rbNumCmp orders two Complex parts in the tower; NaN compares to nothing.
func rbNumCmp(a, b any) *Integer {
	level := max(rbNumLevel(a), rbNumLevel(b))
	a, b = rbNumTo(a, level), rbNumTo(b, level)
	switch level {
	case 0:
		return Ref(a.(Integer).Op_cmp(b.(Integer)))
	case 1:
		return Ref(Integer(a.(*Rational).v.Cmp(&b.(*Rational).v)))
	}
	switch x, y := a.(Float), b.(Float); {
	case x < y:
		return Ref(Integer(-1))
	case x > y:
		return Ref(Integer(1))
	case x == y:
		return Ref(Integer(0))
	}
	return nil
}

// rbIntCmpAny is a gem's `i < x` (op '<', 'l' for <=, '>', 'g' for >=) of an Integer and an untyped x: a Float compares
// as a number, anything else by the implicit conversion to Integer as before (decision 496).
func rbIntCmpAny(op byte, a Integer, b any) Boolean {
	var c int
	if f, ok := b.(Float); ok {
		x, y := float64(a), float64(f)
		switch {
		case x < y:
			c = -1
		case x > y:
			c = 1
		case x != y: // NaN compares to nothing
			return false
		}
	} else {
		c = int(a.Op_cmp(rbAs[Integer](b, "Integer")))
	}
	switch op {
	case '<':
		return c < 0
	case 'l':
		return c <= 0
	case '>':
		return c > 0
	}
	return c >= 0
}
