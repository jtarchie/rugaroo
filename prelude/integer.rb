# prelude/integer.rb
# rbs_inline: enabled

#
# Integer as a named Go int. Where MRI would promote to a Bignum or a
# Rational, it raises RangeError instead (decision 35).

# @go_type int
class Integer < Object
  include Numeric

  #: (Integer) -> Integer
  def <=>(other) = %x{ Integer(cmp.Compare(self, other)) }

  #: (Integer) -> bool
  def <(other) = %x{ Boolean(self < other) }

  #: (Integer) -> bool
  def <=(other) = %x{ Boolean(self <= other) }

  #: (Integer) -> bool
  def >(other) = %x{ Boolean(self > other) }

  #: (Integer) -> bool
  def >=(other) = %x{ Boolean(self >= other) }

  # == on two Integers (decision 12's class overload): a plain Go compare,
  # inlined, with no boxing of the argument into untyped.
  #: (Integer) -> bool
  def __eq_integer(other) = %x{ Boolean(self == other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    switch o := other.(type) {
    case Integer:
      return Boolean(self == o)
    case Float:
      return Boolean(Float(self) == o)
    case *Integer: // a T? box that reached untyped unconverted; nil must not reach rbEq
      return self.Op_eq(Opt(o))
    case *Float:
      return self.Op_eq(Opt(o))
    case rbOpt[Integer]:
      return self.Op_eq(o.rbAny())
    case rbOpt[Float]:
      return self.Op_eq(o.rbAny())
    }
    return rbEq[any](other, self) // MRI asks other == self
  }

  #: (Integer) -> Integer
  def +(other) = %x{
    if rbIntWrap {
      return self + other
    }
    r := self + other
    if (r > self) != (other > 0) {
      panic(rbIntOverflowed{self, other, "+"})
    }
    return r
  }

  #: (Integer) -> Integer
  def -(other) = %x{
    if rbIntWrap {
      return self - other
    }
    r := self - other
    if (r < self) != (other > 0) {
      panic(rbIntOverflowed{self, other, "-"})
    }
    return r
  }

  # Operands within 32 bits cannot overflow; only larger ones pay for the
  # check: the 128-bit product's signed high word must be the low word's sign
  # extension. bits.Mul64 is an instruction, and the failure is a panic, so
  # no call rejoins the hot path (decision 400).
  #: (Integer) -> Integer
  def *(other) = %x{
    if rbIntWrap {
      return self * other
    }
    r := self * other
    if (uint64(self)+1<<31)|(uint64(other)+1<<31) >= 1<<32 {
      hi, _ := bits.Mul64(uint64(self), uint64(other))
      if Integer(hi)-(self>>63&other)-(other>>63&self) != r>>63 {
        panic(rbIntOverflowed{self, other, "*"})
      }
    }
    return r
  }

  # Ruby floors; Go truncates. A zero divisor is Go's own divide panic,
  # which rbWrapPanic raises as ZeroDivisionError: an explicit check would
  # push the body past Go's inlining budget.
  #: (Integer) -> Integer
  def /(other) = %x{
    if other == -1 && self == math.MinInt {
      panic(rbIntOverflowed{self, other, "/"})
    }
    q := self / other
    if q*other != self && (self^other) < 0 {
      q--
    }
    return q
  }

  # A zero divisor panics in Go and rbWrapPanic makes it ZeroDivisionError, as for /.
  #: (Integer) -> Integer
  def %(other) = %x{
    m := self % other
    if m != 0 && (m^other) < 0 {
      m += other
    }
    return m
  }

  # A negative exponent is a Rational in MRI unless the base is 0 or ±1.
  #: (Integer) -> Integer
  def **(other) = %x{
    if other < 0 {
      switch self {
      case 0:
        panic(NewZeroDivisionError(Ref[String]("divided by 0")))
      case 1:
        return 1
      case -1:
        return 1 - 2*(other&1)
      }
      panic(NewRangeError(Ref(String(fmt.Sprintf("%d ** %d is a Rational (Integer#** cannot return one)", self, other)))))
    }
    // Square-and-multiply; the base is squared only while bits remain, so
    // an overflowing square means the result overflows too.
    result, base, ok := Integer(1), self, true
    for e := other; ; {
      if e&1 != 0 {
        if result, ok = rbIntMulOk(result, base); !ok {
          break
        }
      }
      if e >>= 1; e == 0 {
        break
      }
      if base, ok = rbIntMulOk(base, base); !ok {
        break
      }
    }
    if !ok {
      rbIntOverflow(self, "**", other)
    }
    return result
  }

  # MRI's Numeric#+@: the number itself (matrix's `+a`)
  #: () -> Integer
  def +@ = self

  #: () -> Integer
  def -@ = %x{
    if rbIntWrap {
      return -self
    }
    if self == math.MinInt {
      panic(rbIntOverflowed{0, self, "-"})
    }
    return -self
  }

  #: () -> Integer
  def abs = %x{
    if self < 0 {
      return self.Op_neg()
    }
    return self
  }

  #: () -> bool
  def even? = %x{ self%2 == 0 }

  #: () -> bool
  def odd? = %x{ self%2 != 0 }

  #: () -> bool
  def zero? = %x{ self == 0 }

  #: () -> bool
  def positive? = %x{ self > 0 }

  #: () -> bool
  def negative? = %x{ self < 0 }

  #: () -> Integer
  def succ = self + 1

  #: () -> Integer
  def pred = self - 1

  #: () -> Integer
  def to_i = self

  #: () -> Float
  def to_f = %x{ Float(self) }

  #: () -> String
  def to_s = %x{ String(strconv.Itoa(int(self))) }

  #: () -> String
  def inspect = to_s

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true

  #: () -> Integer
  def hash = self

  # MRI's Integer#chr without an encoding: one byte, RangeError outside 0..255.
  #: () -> String
  def chr = %x{
    if self < 0 || self > 255 {
      panic(NewRangeError(Ref(String(strconv.Itoa(int(self)) + " out of char range"))))
    }
    return String([]byte{byte(self)})
  }

  #: () { (Integer) -> void } -> void
  def times = %x{
    return func(yield func(Integer) bool) {
      for i := Integer(0); i < self; i++ {
        if !yield(i) {
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def upto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i <= limit; i++ {
        if !yield(i) || i == math.MaxInt { // i++ would wrap
          return
        }
      }
    }
  }

  #: (Integer) { (Integer) -> void } -> void
  def downto(limit) = %x{
    return func(yield func(Integer) bool) {
      for i := self; i >= limit; i-- {
        if !yield(i) || i == math.MinInt {
          return
        }
      }
    }
  }

  #: (Integer) -> Integer
  def gcd(other) = %x{
    a, b := uint64(self), uint64(other)
    if self < 0 {
      a = -a // unsigned, so MinInt becomes 2**63
    }
    if other < 0 {
      b = -b
    }
    for b != 0 {
      a, b = b, a%b
    }
    if a > math.MaxInt {
      rbIntOverflow(self, "gcd", other)
    }
    return Integer(a)
  }

  #: (Integer) -> Integer
  def lcm(other)
    return 0 if zero? || other.zero?

    (self / gcd(other) * other).abs
  end

  # Modular exponentiation (square-and-multiply on big.Int, so no overflow).
  #: (Integer, Integer) -> Integer
  def __pow_2(exp, mod) = %x{
    if exp < 0 {
      panic(NewRangeError(Ref(String("Integer#pow() 2nd argument not allowed to be negative when 3rd argument specified"))))
    }
    if mod == 0 {
      panic(NewZeroDivisionError(Ref(String("divided by 0"))))
    }
    r := new(big.Int).Exp(big.NewInt(int64(self)), big.NewInt(int64(exp)), big.NewInt(int64(mod)))
    // big.Int.Exp's result is in [0, |mod|); Ruby's takes the modulus's sign
    if r.Sign() != 0 && mod < 0 {
      r.Add(r, big.NewInt(int64(mod)))
    }
    return Integer(r.Int64())
  }

  #: (Integer) -> Integer
  def pow(exp) = self**exp

  #: (?Integer) -> Array[Integer]
  def digits(base = 10) = %x{
    if self < 0 {
      panic(NewMath_DomainError(Ref(String("out of domain"))))
    }
    if base < 2 {
      panic(NewArgumentError(Ref(String("invalid radix " + strconv.FormatInt(int64(base), 10)))))
    }
    if self == 0 {
      return &Array[Integer]{s: []Integer{0}}
    }
    k := 0 // the result's length, counted first so it is built in one allocation
    for n := self; n > 0; n /= base {
      k++
    }
    out := &Array[Integer]{s: make([]Integer, 0, k)}
    for n := self; n > 0; n /= base {
      out.s = append(out.s, n%base)
    }
    return out
  }

  #: (Integer) -> Float
  def fdiv(other) = to_f / other

  #: (Float) -> Float
  def __fdiv_float(other) = to_f / other

  #: (Integer) -> [Integer, Integer]
  def divmod(other) = [self / other, self % other]

  #: (Integer) -> Integer
  def div(other) = self / other

  #: (Integer) -> Integer
  def modulo(other) = self % other

  #: (Integer) -> Integer
  def remainder(other) = %x{
    if other == 0 {
      panic(NewZeroDivisionError(Ref[String]("divided by 0")))
    }
    return self % other
  }

  #: () -> Integer
  def bit_length = %x{
    if self < 0 {
      return Integer(bits.Len64(uint64(^self)))
    }
    return Integer(bits.Len64(uint64(self)))
  }

  #: (Integer) -> String
  def __to_s_1(base) = %x{
    if base < 2 || base > 36 {
      panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(int(base))))))
    }
    return String(strconv.FormatInt(int64(self), int(base)))
  }

  #: () -> bool
  def integer? = true

  #: () -> bool
  def finite? = true

  #: () -> Integer
  def ord = self

  # Integer.sqrt: the floor of the square root, exact for any size.
  #: (Integer) -> Integer
  def self.sqrt(n) = %x{
    if n < 0 {
      panic(NewMath_DomainError(Ref(String(`Numerical argument is out of domain - "isqrt"`))))
    }
    return Integer(new(big.Int).Sqrt(big.NewInt(int64(n))).Int64())
  }

  #: (Integer, ?Integer) { (Integer) -> void } -> void
  def step(limit, by = 1) = %x{
    if by == 0 {
      panic(NewArgumentError(Ref(String("step can't be 0"))))
    }
    return func(yield func(Integer) bool) {
      for i := self; by > 0 && i <= limit || by < 0 && i >= limit; i += by {
        if !yield(i) || by > 0 && i > math.MaxInt-by || by < 0 && i < math.MinInt-by { // i += by would wrap, and the true next is past limit
          return
        }
      }
    }
  }

  # No limit: steps by 1 until the block breaks (elftools' `0.step do |i| ... break if ... end`).
  #: () { (Integer) -> void } -> void
  def __step_0 = %x{
    return func(yield func(Integer) bool) {
      for i := self; yield(i) && i < math.MaxInt; i++ {
      }
    }
  }

  #: (Integer, ?Integer) -> Enumerator::ArithmeticSequence[Integer]
  def __step_enum(limit, by = 1) = %x{ return rbArithNum(self, limit, by, by != 1) }

  #: () -> Enumerator[Integer]
  def __times_enum = %x{ return rbEnumOf(self.Times(), any(self), "times", rbSizeOf(max(self, 0)), nil) }

  #: (Integer) -> Enumerator[Integer]
  def __upto_enum(limit) = %x{ return rbEnumOf(self.Upto(limit), any(self), "upto("+string(rbInspect(limit))+")", func() *Integer { n := rbRangeIntCount(self, limit, false); return &n }, nil) }

  #: (Integer) -> Enumerator[Integer]
  def __downto_enum(limit) = %x{ return rbEnumOf(self.Downto(limit), any(self), "downto("+string(rbInspect(limit))+")", func() *Integer { n := rbRangeIntCount(limit, self, false); return &n }, nil) }

  #: (Integer) -> Integer
  def &(other) = %x{ self & other }

  #: (Integer) -> Integer
  def |(other) = %x{ self | other }

  #: (Integer) -> Integer
  def ^(other) = %x{ self ^ other }

  #: () -> Integer
  def ~ = %x{ ^self }

  # Shifts past 64 bits raise RangeError where MRI would promote to a Bignum (decision 35).
  #: (Integer) -> Integer
  def <<(other) = %x{ rbIntShl(self, other) }

  #: (Integer) -> Integer
  def >>(other) = %x{ rbIntShr(self, other) }

  #: (Integer) -> Integer
  def [](i) = %x{
    if i < 0 {
      return 0
    }
    return (self >> min(i, 63)) & 1
  }

  #: (Integer) -> bool
  def allbits?(mask) = %x{ self&mask == mask }

  #: (Integer) -> bool
  def anybits?(mask) = %x{ self&mask != 0 }

  #: (Integer) -> bool
  def nobits?(mask) = %x{ self&mask == 0 }

  #: (Integer) -> Integer
  def ceildiv(other) = -((-self) / other)

  #: () -> Integer
  def next = self + 1

  #: () -> Integer
  def magnitude = abs

  #: () -> Integer
  def to_int = self

  #: () -> Integer
  def size = 8

  #: (Integer) -> [Integer, Integer]
  def gcdlcm(other) = [gcd(other), lcm(other)]

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(Integer)
    return Boolean(ok && o == self)
  }

  #: () -> Integer
  def round = self

  #: () -> Integer
  def floor = self

  #: () -> Integer
  def ceil = self

  #: () -> Integer
  def truncate = self

  #: (Integer) -> Integer
  def __round_1(digits) = %x{ rbIntRoundTo(self, digits, 3) }

  #: (Integer) -> Integer
  def __floor_1(digits) = %x{ rbIntRoundTo(self, digits, 0) }

  #: (Integer) -> Integer
  def __ceil_1(digits) = %x{ rbIntRoundTo(self, digits, 1) }

  #: (Integer) -> Integer
  def __truncate_1(digits) = %x{ rbIntRoundTo(self, digits, 2) }

  #: () -> Integer?
  def nonzero? = zero? ? nil : self

  #: () -> Integer
  def abs2 = self * self

  #: () -> Integer
  def conj = self

  #: () -> Integer
  def imag = 0

  #: () -> bool
  def real? = true

  # MRI's Numeric#real and the long names of imag/conj/rect (matrix's `e.real`)
  #: () -> Integer
  def real = self

  #: () -> [Integer, Integer]
  def rect = [self, 0]
  alias imaginary imag
  alias conjugate conj
  alias rectangular rect

  # [abs, arg]: the angle is Integer 0 for a non-negative number and π for a negative one, as MRI's.
  #: () -> [Integer, untyped]
  def polar = negative? ? [abs, Math::PI] : [self, 0]

  # Integer 0, or π for a negative number, as MRI's.
  #: () -> untyped
  def arg = negative? ? Math::PI : 0

  #: () -> untyped
  def angle = arg

  #: () -> untyped
  def phase = arg

  #: () -> Integer?
  def infinite? = nil

  # Only an Integer stays one: anything else makes a Float pair, as MRI's int_coerce.
  #: (Integer) -> [Integer, Integer]
  def coerce(other) = [other, self]

  #: (Float) -> [Float, Float]
  def __coerce_float(other) = [other, to_f]

  #: (Rational) -> [Float, Float]
  def __coerce_rational(other) = [other.to_f, to_f]
end

# Kernel#Integer: strict conversion. A String argument must be a number
# entirely; `nil` raises TypeError. The argument's class picks the overload
# at compile time (decision 12), so only an untyped or nilable one is boxed.
module Kernel
  private

  # Kernel#String: to_s (nil is ""); a String is itself.
  #: (untyped) -> String
  def String(x) = %x{ return rbToS(x) }

  #: (untyped) -> Array[untyped]
  def Array(x) = %x{
    if x == nil { // to_ary/to_a are not consulted: an Array is itself, anything else is wrapped
      return NewArray[any]()
    }
    return rbToAry(x)
  }

  #: (untyped) -> Integer
  def Integer(x) = %x{
    switch v := rbUnbox(x).(type) {
    case Integer:
      return v
    case Float:
      return rbFloatToI(float64(v))
    case String:
      return rbStrictInt(v, 0)
    case nil:
      panic(NewTypeError(Ref(String("can't convert nil into Integer"))))
    }
    panic(NewTypeError(Ref(String("can't convert " + rbClassName(x) + " into Integer"))))
  }

  #: (untyped, Integer) -> Integer
  def __Integer_2(x, base) = %x{
    if v, ok := rbUnbox(x).(String); ok {
      return rbStrictInt(v, int(base))
    }
    panic(NewArgumentError(Ref(String("base specified for non string value"))))
  }

  #: (Integer) -> Integer
  def __Integer_integer(x) = x

  #: (Float) -> Integer
  def __Integer_float(x) = x.to_i

  #: (String, ?Integer) -> Integer
  def __Integer_string(x, base = 0) = %x{ rbStrictInt(x, int(base)) }
end
