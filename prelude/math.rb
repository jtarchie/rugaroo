# rbs_inline: enabled

module Math
  class DomainError < ArgumentError; end

  PI = 3.141592653589793 #: Float
  E = 2.718281828459045 #: Float

  #: (Float) -> Float
  def self.sqrt(x) = %x{
    rbMathDomain(x < 0, "sqrt")
    return Float(math.Sqrt(float64(x)))
  }

  #: (Float) -> Float
  def self.cbrt(x) = %x{ Float(math.Cbrt(float64(x))) }

  #: (Float) -> Float
  def self.sin(x) = %x{ Float(math.Sin(float64(x))) }

  #: (Float) -> Float
  def self.cos(x) = %x{ Float(math.Cos(float64(x))) }

  #: (Float) -> Float
  def self.tan(x) = %x{ Float(math.Tan(float64(x))) }

  #: (Float) -> Float
  def self.asin(x) = %x{
    rbMathDomain(x < -1 || x > 1, "asin")
    return Float(math.Asin(float64(x)))
  }

  #: (Float) -> Float
  def self.acos(x) = %x{
    rbMathDomain(x < -1 || x > 1, "acos")
    return Float(math.Acos(float64(x)))
  }

  #: (Float) -> Float
  def self.atan(x) = %x{ Float(math.Atan(float64(x))) }

  #: (Float, Float) -> Float
  def self.atan2(y, x) = %x{ Float(math.Atan2(float64(y), float64(x))) }

  #: (Float) -> Float
  def self.sinh(x) = %x{ Float(math.Sinh(float64(x))) }

  #: (Float) -> Float
  def self.cosh(x) = %x{ Float(math.Cosh(float64(x))) }

  #: (Float) -> Float
  def self.tanh(x) = %x{ Float(math.Tanh(float64(x))) }

  #: (Float) -> Float
  def self.exp(x) = %x{ Float(math.Exp(float64(x))) }

  #: (Float) -> Float
  def self.log(x) = %x{
    rbMathDomain(x < 0, "log")
    return Float(math.Log(float64(x)))
  }

  #: (Float, Float) -> Float
  def self.__log_2(x, base) = %x{
    rbMathDomain(x < 0 || base < 0, "log")
    return Float(math.Log(float64(x)) / math.Log(float64(base)))
  }

  #: (Float) -> Float
  def self.log2(x) = %x{
    rbMathDomain(x < 0, "log2")
    return Float(math.Log2(float64(x)))
  }

  #: (Float) -> Float
  def self.log10(x) = %x{
    rbMathDomain(x < 0, "log10")
    return Float(math.Log10(float64(x)))
  }

  #: (Float, Float) -> Float
  def self.hypot(x, y) = %x{ Float(math.Hypot(float64(x), float64(y))) }

  #: (Float) -> Float
  def self.log1p(x) = %x{
    rbMathDomain(x < -1, "log1p")
    return Float(math.Log1p(float64(x)))
  }

  #: (Float) -> Float
  def self.expm1(x) = %x{ Float(math.Expm1(float64(x))) }

  #: (Float) -> Float
  def self.asinh(x) = %x{ Float(math.Asinh(float64(x))) }

  #: (Float) -> Float
  def self.acosh(x) = %x{
    rbMathDomain(x < 1, "acosh")
    return Float(math.Acosh(float64(x)))
  }

  #: (Float) -> Float
  def self.atanh(x) = %x{
    rbMathDomain(x < -1 || x > 1, "atanh")
    return Float(math.Atanh(float64(x)))
  }

  #: (Float) -> [Float, Integer]
  def self.frexp(x) = %x{
    f, e := math.Frexp(float64(x))
    return Tuple2[Float, Integer]{Float(f), Integer(e)}
  }

  #: (Float, Integer) -> Float
  def self.ldexp(x, e) = %x{ Float(math.Ldexp(float64(x), int(e))) }

  #: (Float) -> Float
  def self.erf(x) = %x{ Float(math.Erf(float64(x))) }

  #: (Float) -> Float
  def self.erfc(x) = %x{ Float(math.Erfc(float64(x))) }

  #: (Float) -> Float
  def self.gamma(x) = %x{
    xf := float64(x)
    switch {
    case math.IsNaN(xf) || math.IsInf(xf, 1):
      return Float(xf)
    case xf == 0:
      return Float(math.Copysign(math.Inf(1), xf))
    case (xf == math.Trunc(xf) && xf < 0) || math.IsInf(xf, -1):
      rbMathDomain(true, "gamma")
    case xf > 171.7:
      return Float(math.Inf(1))
    }
    return Float(math.Gamma(xf))
  }

  #: (Float) -> [Float, Integer]
  def self.lgamma(x) = %x{
    xf := float64(x)
    switch {
    case math.IsInf(xf, -1):
      rbMathDomain(true, "lgamma")
    case math.IsNaN(xf) || math.IsInf(xf, 1):
      return Tuple2[Float, Integer]{Float(xf), Integer(1)}
    case xf == 0:
      if math.Signbit(xf) {
        return Tuple2[Float, Integer]{Float(math.Inf(1)), Integer(-1)}
      }
      return Tuple2[Float, Integer]{Float(math.Inf(1)), Integer(1)}
    case xf == math.Trunc(xf) && xf < 0:
      return Tuple2[Float, Integer]{Float(math.Inf(1)), Integer(1)}
    case xf == 1 || xf == 2:
      return Tuple2[Float, Integer]{Float(0), Integer(1)}
    }
    v, sign := math.Lgamma(xf)
    return Tuple2[Float, Integer]{Float(v), Integer(sign)}
  }
end

class Float
  INFINITY = 1.0 / 0 #: Float
  NAN = 0.0 / 0 #: Float
  EPSILON = 2.220446049250313e-16 #: Float
  MAX = 1.7976931348623157e+308 #: Float
  MIN = 2.2250738585072014e-308 #: Float
  DIG = 15 #: Integer
  MANT_DIG = 53 #: Integer
  MAX_EXP = 1024 #: Integer
  MIN_EXP = -1021 #: Integer
  MAX_10_EXP = 308 #: Integer
  MIN_10_EXP = -307 #: Integer
  RADIX = 2 #: Integer
end
