# rbs_inline: enabled

# Exact fractions on math/big; numerator and denominator must fit an Integer when read.
# @go_type struct { v big.Rat }
class Rational < Object
  include Numeric

  #: () -> Integer
  def numerator = %x{ rbBigToInt(self.v.Num()) }

  #: () -> Integer
  def denominator = %x{ rbBigToInt(self.v.Denom()) }

  #: (Rational) -> Rational
  def +(o) = %x{ rbRatOp(self, o, (*big.Rat).Add) }

  #: (Rational) -> Rational
  def -(o) = %x{ rbRatOp(self, o, (*big.Rat).Sub) }

  #: (Rational) -> Rational
  def *(o) = %x{ rbRatOp(self, o, (*big.Rat).Mul) }

  #: (Rational) -> Rational
  def /(o) = %x{
    if o.v.Sign() == 0 {
      panic(NewZeroDivisionError(Ref(String("divided by 0"))))
    }
    return rbRatOp(self, o, (*big.Rat).Quo)
  }

  #: (Rational) -> Rational
  def quo(o) = self / o

  #: (Integer) -> Rational
  def __plus_integer(o) = self + o.to_r

  #: (Integer) -> Rational
  def __minus_integer(o) = self - o.to_r

  #: (Integer) -> Rational
  def __mul_integer(o) = self * o.to_r

  #: (Integer) -> Rational
  def __div_integer(o) = self / o.to_r

  #: (Float) -> Float
  def __plus_float(o) = to_f + o

  #: (Float) -> Float
  def __minus_float(o) = to_f - o

  #: (Float) -> Float
  def __mul_float(o) = to_f * o

  #: (Float) -> Float
  def __div_float(o) = to_f / o

  #: (Integer) -> Rational
  def **(n) = %x{
    e := int(n)
    if e < 0 {
      if self.v.Sign() == 0 {
        panic(NewZeroDivisionError(Ref(String("divided by 0"))))
      }
      e = -e
    }
    num := new(big.Int).Exp(self.v.Num(), big.NewInt(int64(e)), nil)
    den := new(big.Int).Exp(self.v.Denom(), big.NewInt(int64(e)), nil)
    out := &Rational{}
    if n < 0 {
      num, den = den, num
    }
    out.v.SetFrac(num, den)
    return out
  }

  # MRI's Numeric#+@: the number itself (matrix's `+a`)
  #: () -> Rational
  def +@ = self

  #: () -> Rational
  def -@ = %x{
    out := &Rational{}
    out.v.Neg(&self.v)
    return out
  }

  #: () -> Rational
  def abs = %x{
    out := &Rational{}
    out.v.Abs(&self.v)
    return out
  }

  #: (Rational) -> Integer
  def <=>(o) = %x{ Integer(self.v.Cmp(&o.v)) }

  #: (Integer) -> bool
  def __lt_integer(o) = self < o.to_r

  #: (Integer) -> bool
  def __le_integer(o) = self <= o.to_r

  #: (Integer) -> bool
  def __gt_integer(o) = self > o.to_r

  #: (Integer) -> bool
  def __ge_integer(o) = self >= o.to_r

  #: (Integer) -> Integer
  def __cmp_integer(o) = self <=> o.to_r

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := rbUnbox(other).(type) {
    case *Rational:
      return Boolean(self.v.Cmp(&o.v) == 0)
    case Integer:
      return Boolean(self.v.IsInt() && self.v.Num().IsInt64() && self.v.Num().Int64() == int64(o))
    case Float:
      return Boolean(float64(self.ToF()) == float64(o))
    }
    return false
  }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(*Rational)
    return Boolean(ok && self.v.Cmp(&o.v) == 0)
  }

  #: () -> Integer
  def hash = %x{ Integer(rbKeyHash(self.v.RatString())) }

  #: () -> bool
  def zero? = %x{ Boolean(self.v.Sign() == 0) }

  #: () -> bool
  def positive? = %x{ Boolean(self.v.Sign() > 0) }

  #: () -> bool
  def negative? = %x{ Boolean(self.v.Sign() < 0) }

  #: () -> bool
  def integer? = false

  #: () -> bool
  def frozen? = true

  #: () -> Integer
  def to_i = %x{ rbBigToInt(new(big.Int).Quo(self.v.Num(), self.v.Denom())) }

  #: () -> Integer
  def truncate = to_i

  #: () -> Integer
  def floor = %x{ rbRatFloor(&self.v) }

  #: () -> Integer
  def ceil = %x{
    var neg big.Rat
    return -rbRatFloor(neg.Neg(&self.v))
  }

  # Half away from zero, as MRI.
  #: () -> Integer
  def round = %x{
    var a, half big.Rat
    a.Abs(&self.v)
    half.SetFrac64(1, 2)
    r := rbRatFloor(a.Add(&a, &half))
    if self.v.Sign() < 0 {
      return -r
    }
    return r
  }

  #: () -> Float
  def to_f = %x{
    f, _ := self.v.Float64()
    return Float(f)
  }

  #: () -> Rational
  def to_r = self

  #: () -> String
  def to_s = %x{ String(self.v.Num().String() + "/" + self.v.Denom().String()) }

  #: () -> String
  def inspect = "(#{to_s})"

  #: () -> Rational?
  def nonzero? = zero? ? nil : self

  #: () -> bool
  def finite? = true

  #: () -> Integer?
  def infinite? = nil

  #: () -> bool
  def real? = true

  # MRI's Numeric#real, #imag, #conj, #rect for a real number (matrix's `e.real`)
  #: () -> Rational
  def real = self

  #: () -> Integer
  def imag = 0

  #: () -> Rational
  def conj = self

  #: () -> [Rational, Integer]
  def rect = [self, 0]
  alias imaginary imag
  alias conjugate conj
  alias rectangular rect

  #: () -> Rational
  def magnitude = abs

  # The exact quotient as a Float; a zero divisor gives a Float infinity or NaN, as MRI's nurat_fdiv.
  #: (Rational) -> Float
  def fdiv(o) = o.zero? ? to_f / 0.0 : (self / o).to_f

  #: (Float) -> Float
  def __fdiv_float(o) = to_f / o

  #: (Rational) -> Integer
  def div(o) = (self / o).floor

  #: (Rational) -> Rational
  def %(o) = self - o * div(o)

  #: (Rational) -> Rational
  def modulo(o) = self % o

  #: (Rational) -> [Integer, Rational]
  def divmod(o)
    q = div(o)
    [q, self - o * q]
  end

  #: (Rational) -> Rational
  def remainder(o) = self - o * (self / o).truncate

  #: (Rational, ?Rational) { (Rational) -> void } -> void
  def step(limit, by = 1r)
    raise ArgumentError, "step can't be 0" if by.zero?

    i = self
    while by.positive? ? i <= limit : i >= limit
      yield i
      i += by
    end
  end

  #: (Rational, ?Rational) -> Array[Rational]
  def __step_enum(limit, by = 1r)
    out = [] #: Array[Rational]
    step(limit, by) { |x| out << x }
    out
  end

  #: (Rational) -> [Rational, Rational]
  def coerce(o) = [o, self]

  #: (Integer) -> [Rational, Rational]
  def __coerce_integer(o) = [o.to_r, self]

  #: (Float) -> [Float, Float]
  def __coerce_float(o) = [o, to_f]

  #: (Complex) -> [Complex, Complex]
  def __coerce_complex(o) = [o, to_c]
end

class Integer
  #: () -> Rational
  def to_r = %x{ rbRat(int64(self), 1) }

  #: (Integer) -> Rational
  def quo(o) = to_r / o.to_r

  #: () -> Integer
  def numerator = self

  #: () -> Integer
  def denominator = 1

  #: (Rational) -> Rational
  def __plus_rational(o) = to_r + o

  #: (Rational) -> Rational
  def __minus_rational(o) = to_r - o

  #: (Rational) -> Rational
  def __mul_rational(o) = to_r * o

  #: (Rational) -> Rational
  def __div_rational(o) = to_r / o
end

class String
  # The leading rational, 0 when there is none, as MRI's (decision 191).
  #: () -> Rational
  def to_r = %x{ return rbStrToR(string(self)) }
end

class Float
  # Exact: every finite Float is a dyadic fraction.
  #: () -> Rational
  def to_r = %x{
    out := &Rational{}
    if out.v.SetFloat64(float64(self)) == nil {
      panic(NewFloatDomainError(Ref(String(rbFloatToS(float64(self))))))
    }
    return out
  }

  # MRI's rb_flt_rationalize: the simplest rational within half an ulp, or within |eps| (decision 367).
  #: () -> Rational
  def __rationalize_0 = %x{ return rbFloatRationalize(float64(self)) }

  #: (Numeric) -> Rational
  def rationalize(eps) = __rationalize_eps(eps.to_f)

  #: (Float) -> Rational
  def __rationalize_eps(e) = %x{ return rbFloatRationalizeEps(float64(self), float64(e)) }

  #: (Rational) -> Float
  def __plus_rational(o) = self + o.to_f

  #: (Rational) -> Float
  def __minus_rational(o) = self - o.to_f

  #: (Rational) -> Float
  def __mul_rational(o) = self * o.to_f

  #: (Rational) -> Float
  def __div_rational(o) = self / o.to_f
end

module Kernel
  private

  # An Integer, Float, Rational or a String that is one ("2.5", "1/3"), as MRI converts them: the parser gem's
  # `Rational(chars)` of a rational literal's digits.
  #: (untyped, ?untyped) -> Rational
  def Rational(n, d = 1) = %x{
    return rbRational(n, d)
  }
end
