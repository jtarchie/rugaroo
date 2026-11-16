//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/big"
	"regexp"
	"strings"
)

func rbRat(n, d int64) *Rational {
	out := &Rational{}
	out.v.SetFrac64(n, d)
	return out
}

func rbRatOp(a, b *Rational, op func(z, x, y *big.Rat) *big.Rat) *Rational {
	out := &Rational{}
	op(&out.v, &a.v, &b.v)
	return out
}

// rbBigToInt raises where MRI would answer a Bignum (decision 35).
func rbBigToInt(b *big.Int) Integer {
	if !b.IsInt64() {
		panic(NewRangeError(Ref(String(b.String() + " overflows Integer (64-bit; no Bignum)"))))
	}
	return Integer(b.Int64())
}

func rbRatFloor(r *big.Rat) Integer {
	q, _ := new(big.Int).DivMod(r.Num(), r.Denom(), new(big.Int)) // Euclidean: the denominator is positive, so q is the floor
	return rbBigToInt(q)
}

var rbStrToRRe = regexp.MustCompile(`^\s*([-+]?(?:\d[\d_]*)?(?:\.\d[\d_]*)?(?:[eE][-+]?\d+)?)(?:/(\d[\d_]*))?`)

// rbStrToR is String#to_r: the leading rational ("1/3", "0.75", "-2.5e2", "1.5/2"), 0 when there is none; a zero
// denominator is MRI's ZeroDivisionError.
func rbStrToR(s string) *Rational {
	m := rbStrToRRe.FindStringSubmatch(s)
	out := &Rational{}
	num := strings.ReplaceAll(m[1], "_", "")
	if num == "" || num == "-" || num == "+" || strings.HasPrefix(strings.TrimLeft(num, "+-"), "e") {
		return out
	}
	if _, ok := out.v.SetString(num); !ok {
		return &Rational{}
	}
	if m[2] != "" {
		den, _ := new(big.Rat).SetString(strings.ReplaceAll(m[2], "_", ""))
		if den.Sign() == 0 {
			panic(NewZeroDivisionError(Ref(String("divided by 0"))))
		}
		out.v.Quo(&out.v, den)
	}
	return out
}

// rbFloatRationalize is MRI's Float#rationalize with no argument: the simplest rational within half a unit in the last
// place of f (rb_flt_rationalize).
func rbFloatRationalize(f float64) *Rational {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		panic(NewFloatDomainError(Ref(rbFloatToS(f))))
	}
	frac, exp := math.Frexp(f)
	m := new(big.Int)
	new(big.Float).SetFloat64(math.Ldexp(frac, 53)).Int(m)
	n := exp - 53
	out := &Rational{}
	if m.Sign() == 0 || n >= 0 {
		out.v.SetInt(new(big.Int).Lsh(m, uint(max(n, 0))))
		return out
	}
	den := new(big.Int).Lsh(big.NewInt(1), uint(1-n))
	twoM := new(big.Int).Mul(big.NewInt(2), m)
	a := new(big.Rat).SetFrac(new(big.Int).Sub(twoM, big.NewInt(1)), den)
	b := new(big.Rat).SetFrac(new(big.Int).Add(twoM, big.NewInt(1)), den)
	out.v.Set(rbSimplestRat(a, b))
	return out
}

// rbSimplestRat is nurat_rationalize_internal on exact rationals: the continued-fraction walk to the simplest p/q in (a, b).
func rbSimplestRat(a, b *big.Rat) *big.Rat {
	a, b = new(big.Rat).Set(a), new(big.Rat).Set(b)
	p0, p1, q0, q1 := big.NewInt(0), big.NewInt(1), big.NewInt(1), big.NewInt(0)
	one := big.NewRat(1, 1)
	ceil := func(x *big.Rat) *big.Int {
		q, r := new(big.Int).QuoRem(x.Num(), x.Denom(), new(big.Int))
		if r.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		}
		return q
	}
	for {
		c := ceil(a)
		if new(big.Rat).SetInt(c).Cmp(b) < 0 {
			p := new(big.Int).Add(new(big.Int).Mul(c, p1), p0)
			q := new(big.Int).Add(new(big.Int).Mul(c, q1), q0)
			return new(big.Rat).SetFrac(p, q)
		}
		k := new(big.Int).Sub(c, big.NewInt(1))
		kr := new(big.Rat).SetInt(k)
		p2 := new(big.Int).Add(new(big.Int).Mul(k, p1), p0)
		q2 := new(big.Int).Add(new(big.Int).Mul(k, q1), q0)
		t := new(big.Rat).Quo(one, new(big.Rat).Sub(b, kr))
		b = new(big.Rat).Quo(one, new(big.Rat).Sub(a, kr))
		a = t
		p0, q0, p1, q1 = p1, q1, p2, q2
	}
}

// rbFloatRationalizeEps is Float#rationalize(eps): the simplest rational in [f-|eps|, f+|eps|], walked on Floats as
// MRI's rb_flt_rationalize_with_prec does.
func rbFloatRationalizeEps(f, e float64) *Rational {
	e = math.Abs(e)
	a, b := f-e, f+e
	out := &Rational{}
	if a == b {
		out.v.SetFloat64(f)
		return out
	}
	var p0, p1, q0, q1 int64 = 0, 1, 1, 0
	for {
		c := math.Ceil(a)
		if c < b {
			out.v.SetFrac64(int64(c)*p1+p0, int64(c)*q1+q0)
			return out
		}
		k := c - 1
		p2, q2 := int64(k)*p1+p0, int64(k)*q1+q0
		t := 1 / (b - k)
		b = 1 / (a - k)
		a = t
		p0, q0, p1, q1 = p1, q1, p2, q2
	}
}

var rbRationalRe = regexp.MustCompile(`^\s*[-+]?(?:\d[\d_]*)(?:\.\d[\d_]*)?(?:[eE][-+]?\d+)?(?:/\d[\d_]*)?\s*$`)

// rbRationalOf is Kernel#Rational's one value: an Integer, Float or Rational as it is, a String that is a rational
// and nothing else ("2.5", "1/3"; String#to_r takes a prefix instead), ArgumentError or TypeError for the rest, as MRI.
func rbRationalOf(x any) *Rational {
	switch v := rbUnbox(x).(type) {
	case Integer:
		return rbRat(int64(v), 1)
	case Float:
		out := &Rational{}
		if out.v.SetFloat64(float64(v)) == nil {
			panic(NewFloatDomainError(Ref(rbFloatToS(float64(v)))))
		}
		return out
	case *Rational:
		return v
	case String:
		if !rbRationalRe.MatchString(string(v)) {
			panic(NewArgumentError(Ref(String("invalid value for convert(): " + string(v.Inspect())))))
		}
		return rbStrToR(string(v))
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into Rational"))))
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(x) + " into Rational"))))
}

// rbRational is Kernel#Rational(n, d): n over d, each converted as rbRationalOf.
func rbRational(n, d any) *Rational {
	num, den := rbRationalOf(n), rbRationalOf(d)
	if den.v.Sign() == 0 {
		panic(NewZeroDivisionError(Ref(String("divided by 0"))))
	}
	return rbRatOp(num, den, (*big.Rat).Quo)
}
