//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// rbStrictFloat is Kernel#Float(str): String#to_f's syntax (rbFloatPrefix),
// or a hex float, covering the whole string bar surrounding whitespace.
// Out-of-range values become ±Infinity or 0 as in MRI, minus its warning.
func rbStrictFloat(s String) Float {
	t := strings.Trim(string(s), " \t\n\v\f\r")
	body := strings.TrimLeft(t, "+-")
	var f float64
	var err error
	switch {
	case len(t)-len(body) > 1:
		err = strconv.ErrSyntax
	case len(body) > 2 && strings.EqualFold(body[:2], "0x") && body[2] == '_': // Go lets an underscore follow the prefix; MRI does not
		err = strconv.ErrSyntax
	case len(body) > 2 && strings.EqualFold(body[:2], "0x"):
		if f, err = strconv.ParseFloat(t, 64); err != nil && !strings.ContainsAny(body, "pP") {
			f, err = strconv.ParseFloat(t+"p0", 64)
		}
	default:
		if m := rbFloatPrefix.FindStringSubmatchIndex(t); m != nil && m[3] == len(t) {
			f, err = strconv.ParseFloat(strings.ReplaceAll(t, "_", ""), 64)
		} else {
			err = strconv.ErrSyntax
		}
	}
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		panic(NewArgumentError(Ref("invalid value for Float(): " + rbStringInspect(string(s)))))
	}
	return Float(f)
}

// rbFloatStep is MRI's ruby_float_step: n steps counted up front with its
// rounding allowance, each value beg + i*unit, the last clamped to end.
func rbFloatStep(beg, end, unit float64, yield func(Float) bool) {
	rbFloatStepX(beg, end, unit, false, yield)
}

// rbFloatStepX is rbFloatStep with MRI's exclusive-end count (ruby_float_step_size), so (1.0...55.6).step(18.2) ends at 55.599999999999994.
func rbFloatStepX(beg, end, unit float64, excl bool, yield func(Float) bool) {
	if unit == 0 {
		panic(NewArgumentError(Ref(String("step can't be 0"))))
	}
	if math.IsInf(unit, 0) {
		if unit > 0 && (beg < end || !excl && beg == end) || unit < 0 && (beg > end || !excl && beg == end) {
			yield(Float(beg))
		}
		return
	}
	n := (end - beg) / unit
	errAllow := (math.Abs(beg) + math.Abs(end) + math.Abs(end-beg)) / math.Abs(unit) * 2.220446049250313e-16
	if errAllow > 0.5 {
		errAllow = 0.5
	}
	if excl {
		if n <= 0 {
			return
		}
		if n < 1 {
			n = 0
		} else {
			n = math.Floor(n - errAllow)
		}
		if d := (n+1)*unit + beg; beg < end && d < end || beg > end && d > end {
			n++
		}
	} else {
		if n < 0 {
			return
		}
		n = math.Floor(n + errAllow)
	}
	for i := 0.0; i <= n; i++ {
		d := i*unit + beg
		if unit >= 0 && end < d || unit < 0 && end > d {
			d = end
		}
		if !yield(Float(d)) {
			return
		}
	}
}

// rbFloatDigits is MRI's Float#round (mode 3, half up), #floor (0) and #ceil (1) for digits nd > 0: a number already exact at nd
// digits is itself (float_round_overflow), one too small rounds to 0.0 (float_round_underflow), more than 14 digits go through
// an exact Rational because pow(10, nd) is no longer exact, and the rest scale by pow(10, nd) with a one-step correction.
func rbFloatDigits(x float64, nd int, mode int) float64 {
	if x == 0 || mode == 3 && (math.IsInf(x, 0) || math.IsNaN(x)) {
		return x
	}
	_, binexp := math.Frexp(x)
	over, under := 0, 0
	if binexp > 0 {
		over, under = binexp/4, binexp/3+1
	} else {
		over, under = binexp/3-1, binexp/4
	}
	if nd >= 17-over {
		return x
	}
	if nd < -under && (mode == 3 || mode == 0 && x > 0 || mode == 1 && x < 0) {
		return 0
	}
	if nd > 14 {
		if math.IsInf(x, 0) || math.IsNaN(x) {
			panic(NewFloatDomainError(Ref(rbFloatToS(x))))
		}
		scale := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(nd)), nil))
		v := new(big.Rat).SetFloat64(x)
		v.Mul(v, scale)
		q := new(big.Int)
		switch mode {
		case 0:
			q.Div(v.Num(), v.Denom()) // Euclidean: floor, as the denominator is positive
		case 1:
			q.Neg(q.Div(new(big.Int).Neg(v.Num()), v.Denom()))
		default: // half away from zero
			twice := new(big.Int).Lsh(new(big.Int).Abs(v.Num()), 1)
			twice.Add(twice, v.Denom())
			q.Div(twice, new(big.Int).Lsh(v.Denom(), 1))
			if v.Sign() < 0 {
				q.Neg(q)
			}
		}
		r := new(big.Rat).SetFrac(q, new(big.Int).Set(scale.Num()))
		const fix = 1 << 62 // MRI's Rational#to_f divides two fixnums as doubles (each rounded first), and only a Bignum part gets a correctly rounded quotient
		if n, d := r.Num(), r.Denom(); n.IsInt64() && d.IsInt64() && n.Int64() > -fix && n.Int64() < fix && d.Int64() < fix {
			return float64(n.Int64()) / float64(d.Int64())
		}
		f, _ := r.Float64()
		return f
	}
	f := math.Pow(10, float64(nd))
	switch mode {
	case 0:
		mul := math.Floor(x * f)
		if res := (mul + 1) / f; res <= x {
			return res
		}
		return mul / f
	case 1: // no step back, unlike floor: 0.000253.ceil(6) is 0.000254 as x*f is 253.00000000000003
		return math.Ceil(x*f) / f
	}
	r := math.Round(x * f)
	if x > 0 && (r+0.5)/f <= x {
		r++
	} else if x < 0 && (r-0.5)/f >= x {
		r--
	}
	return r / f
}
