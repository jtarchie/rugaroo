# rbs_inline: enabled

require "minitest/autorun"
require "bigdecimal"

#: () { () -> Integer } -> String
def number_big
  yield.to_s
rescue RangeError
  "RangeError"
end

#: () { () -> void } -> String
def number_class_of
  yield
  "ok"
rescue FloatDomainError
  "FloatDomainError"
rescue RangeError
  "RangeError"
end

#: (Integer, ?Integer, ?Integer) -> Integer
def number_chain(a, b = a + 1, c = b + 1) = a + b + c

#: (Integer) -> Integer
def number_id(n) = n

# Helpers for the checks that were testdata/run/number_boolean.rb.
#: (bool) -> String
def number_yes_no(b) = b ? "yes" : "no"

#: (bool) -> String
def describe_bool(b)
  case b
  when true then "on"
  when false then "off"
  else "?"
  end
end

#: (Array[String], String, bool) -> bool
def noisy(log, tag, b)
  log << tag
  b
end

#: (untyped) -> untyped
def number_ident(v) = v

#: (untyped) -> String
def kind_eqq(v)
  case v
  when Integer then "int"
  when Float then "float"
  when true then "true"
  else "other"
  end
end

#: (Integer) -> String
def fizz(i)
  case i
  when NumberTests::Div.new(15) then "FizzBuzz"
  when NumberTests::Div.new(3) then "Fizz"
  when NumberTests::Div.new(5) then "Buzz"
  else i.to_s
  end
end

#: (Integer) -> String
def sized(n)
  case n
  when 0 then "zero"
  when Integer then "int"
  else "?"
  end
end

#: (untyped) -> untyped
def number_ident_cmp(v) = v

#: (untyped) -> untyped
def ident_hash(v) = v

# Helpers for the checks that were testdata/run/number_collections.rb.
#: (Array[Integer], Integer) -> Integer?
def find_over(xs, n)
  xs.each { |x| return x if x > n }
  nil
end

#: (Integer?) -> Integer
def double_or_zero(x)
  return 0 unless x

  x * 2
end

#: (Float?) -> Float
def number_half(x)
  return -1.0 if x.nil?

  x / 2.0
end

#: (Integer?) -> String
def number_describe(x)
  if x && x > 10
    "big #{x}"
  elsif x
    "small #{x.succ}"
  else
    "none"
  end
end

# Helpers for the checks that were testdata/run/number_comparable.rb.
# A method added to Comparable reaches Integer and Float, and reopened classes call mixed-in methods on self.
module Comparable
  #: (self) -> self
  def at_least(o) = self < o ? o : self
end

class Float
  #: () -> bool
  def unit? = between?(0.0, 1.0)
end

class Integer
  #: () -> Integer
  def digit = clamp(0, 9)
end

# Helpers for the checks that were testdata/run/number_iterators.rb.
#: (Integer) -> Integer
def number_first_square_over(n)
  1.upto(100) do |i|
    return i if i * i > n
  end
  -1
end

#: (Integer, Integer) -> Array[Integer]
def countdown(from, to)
  out = [] #: Array[Integer]
  from.downto(to) { |i| out << i }
  out
end

#: (Integer) -> Integer
def triangle(n)
  sum = 0
  n.times { |i| sum += i + 1 }
  sum
end

#: (Integer) { (Integer) -> void } -> void
def number_each_even(n)
  n.times { |i| yield i if i.even? }
end

#: (Integer) { (Integer) -> void } -> void
def number_rep(n, &blk)
  n.times(&blk)
end

#: (Integer, Integer) { (Integer, Integer) -> void } -> void
def grid_each(w, h)
  h.times { |y| w.times { |x| yield x, y } }
end

class Integer
  #: () { (Integer) -> void } -> void
  def each_digit
    s = to_s
    s.size.times { |i| yield s[i].to_i }
  end
end

#: (bool) -> bool?
def number_maybe(b) = b ? false : nil

# Helpers for the checks that were testdata/run/number_reopen.rb.
class Integer
  #: () -> Integer
  def double = self * 2

  #: (Integer) -> bool
  def divisible_by?(n) = (self % n).zero?

  #: () -> Integer
  def factorial
    acc = 1
    1.upto(self) { |i| acc *= i }
    acc
  end

  #: () -> String
  def ordinal
    return "#{self}th" if (self % 100).between?(11, 13)

    case self % 10
    when 1 then "#{self}st"
    when 2 then "#{self}nd"
    when 3 then "#{self}rd"
    else "#{self}th"
    end
  end
end

class Float
  #: () -> bool
  def above_zero? = self > 0.0

  #: () -> Float
  def half = self / 2.0
end

#: (?Integer, ?Float, ?bool) -> String
def defaults(n = 3, f = 1.5, b = true) = "#{n} #{f} #{b}"

# Helpers for the checks that were testdata/run/number_untyped.rb.
#: (untyped) -> String
def number_kind(v)
  case v
  when Integer then "Integer #{v.inspect}"
  when Float then "Float #{v.inspect}"
  when String then "String #{v.inspect}"
  when nil then "nil"
  else "other #{v.inspect}"
  end
end

#: (untyped) -> String
def truthy(v) = v ? "truthy" : "falsy"

# Numeric (#39): a parameter, a when arm and narrowing typed against it.
#: (Numeric) -> Numeric
def number_double(n) = n * 2

#: (untyped) -> String
def number_numeric_kind(v)
  case v
  when Integer then "int #{v + 1}"
  when Float then "float #{v.floor}"
  when Numeric then "number #{v.zero?}"
  else "other"
  end
end

#: (untyped) -> String
def number_only_numeric(v)
  case v
  when Numeric then "number #{v.abs}"
  else "other"
  end
end

#: (Numeric) -> String
def number_narrow(n)
  if n.is_a?(Integer)
    "even #{n.even?}"
  elsif n.is_a?(Float)
    "nan #{n.nan?}"
  else
    "other #{n.integer?}"
  end
end

#: [T] (T) -> String
def number_generic_kind(x)
  return "other" unless x.is_a?(Numeric)

  y = x
  y.is_a?(Integer) ? "int #{y + 1}" : "number #{x.zero?}"
end

#: (untyped) -> Numeric?
def number_succ(v)
  return nil unless v.is_a?(Numeric)

  v + 1
end

module NumberTests
  # Helpers for the checks that were testdata/run/number_bug_case_when_eqq.rb.
  class Div
    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: (Integer) -> bool
    def ===(x) = x % @n == 0
  end

  # Helpers for the checks that were testdata/run/number_bug_operator_name_collision.rb.
  class Meter
    #: (Integer) -> Integer
    def +(o) = o + 1
    #: (Integer) -> Integer
    def plus(o) = o + 2
    #: (Integer) -> Integer
    def [](i) = i * 10
    #: (Integer) -> Integer
    def idx(i) = i * 100
    #: () -> Integer
    def -@ = -1
    #: () -> Integer
    def neg = -2
    #: (Integer) -> Integer
    def /(o) = 100 / o
    #: (Integer) -> Integer
    def div(o) = 1000 / o
    #: (Integer) -> Integer
    def **(o) = o * o
    #: (Integer) -> Integer
    def pow(o) = o * o * o
    #: () -> bool
    def empty? = true
    #: () -> bool
    def empty_q = false
  end

  # Helpers for the checks that were testdata/run/number_bugs.rb.
  # Integer == falls back to the other side's == for a user object
  class One
    #: (untyped) -> bool
    def ==(o) = o == 1
  end

  # Helpers for the checks that were testdata/run/number_mid.rb.
  # != follows ==, across Integer/Float and a user-defined ==
  class Money
    attr_reader :cents #: Integer

    #: (Integer) -> void
    def initialize(cents)
      @cents = cents
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Money) && other.cents == cents
  end

  # += on a user class calls its own + (Num is defined with the number_operators.rb helpers below)

  # Helpers for the checks that were testdata/run/number_operators.rb.
  # Decision 3: every overloadable operator gets its own Go name, distinct from the camel-cased names of ordinary methods.
  class Num
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
      @last_set = nil #: Integer?
    end

    #: (Num) -> Num
    def +(o) = Num.new(v + o.v)
    #: (Num) -> Num
    def -(o) = Num.new(v - o.v)
    #: (Num) -> Num
    def *(o) = Num.new(v * o.v)
    #: (Num) -> Num
    def /(o) = Num.new(v / o.v)
    #: (Num) -> Num
    def %(o) = Num.new(v % o.v)
    #: (Integer) -> Num
    def **(e) = Num.new(v**e)
    #: () -> Num
    def -@ = Num.new(-v)
    #: () -> Num
    def +@ = Num.new(v.abs)
    #: () -> Num
    def ~ = Num.new(-v - 1)
    #: (Integer) -> Num
    def <<(k) = Num.new(v * (2**k))
    #: (Integer) -> Num
    def >>(k) = Num.new(v / (2**k))
    #: (Num) -> Num
    def &(o) = Num.new(v < o.v ? v : o.v)
    #: (Num) -> Num
    def |(o) = Num.new(v > o.v ? v : o.v)
    #: (Num) -> Num
    def ^(o) = Num.new((v - o.v).abs)
    #: (untyped) -> bool
    def ==(o) = o.is_a?(Num) && o.v == v
    #: (Num) -> bool
    def !=(o) = v != o.v
    #: (Num) -> Integer
    def <=>(o) = v <=> o.v
    #: (Num) -> bool
    def <(o) = v < o.v
    #: (Num) -> bool
    def <=(o) = v <= o.v
    #: (Num) -> bool
    def >(o) = v > o.v
    #: (Num) -> bool
    def >=(o) = v >= o.v
    #: (Integer) -> bool
    def ===(x) = x == v
    #: (Integer) -> bool
    def =~(x) = x % v == 0
    #: (Integer) -> bool
    def !~(x) = x % v != 0
    #: (Integer) -> Integer
    def [](i) = v * i

    #: (Integer, Integer) -> Integer
    def []=(i, x)
      @last_set = v + i + x
      x
    end

    #: () -> Integer?
    def last_set = @last_set
    #: () -> bool
    def ! = v == 0
    #: (Integer) -> Integer
    def index(i) = v * 1000 + i
    #: (Integer) -> bool
    def match(x) = x == v
    #: () -> String
    def to_s = "Num(#{v})"

    # Non-operator names: ? -> Q, ! -> Bang, = -> Set, leading underscores kept.
    #: (Integer) -> Integer
    def v=(x)
      @v = x
      x
    end

    #: () -> Num
    def reset!
      @v = 0
      self
    end

    #: () -> bool
    def big? = v > 10
    #: () -> Integer
    def __raw = v
    #: () -> Integer
    def _half = v / 2
  end

  # Helpers for the checks that were testdata/run/number_state.rb.
  LIMIT = 10 #: Integer

  RATE = 0.25

  DEBUG = false

  class Account
    attr_reader :balance #: Float
    attr_accessor :count #: Integer

    #: (?Float) -> void
    def initialize(balance = 0.0)
      @balance = balance
      @count = 0
      @open = true #: bool
    end

    #: (Float) -> Float
    def deposit(amt)
      @balance += amt
      @count += 1
      @balance
    end

    #: () -> bool
    def open? = @open

    #: () -> void
    def toggle
      @open = !@open
    end
  end

  class NumberBooleanTest < Minitest::Test
    # This file never seeds, so these draw from math/rand/v2 when it runs alone (decision 403).
    def test_unseeded_rand_ranges
      ints = 200.times.map { rand(10) }
      floats = 200.times.map { rand }
      rolls = 200.times.map { rand(1..6) }
      assert_equal [true, true, true], [ints.all? { |i| i >= 0 && i < 10 }, floats.all? { |f| f >= 0.0 && f < 1.0 }, rolls.all? { |r| r >= 1 && r <= 6 }]
      assert_equal true, Random.rand(3) < 3
    end

    def test_bool_operators_and_conversions
      t = true #: bool
      f = false #: bool

      # inspect and to_s (puts/print of bools stay in testdata/run/number_output.rb)
      assert_equal "true", t.inspect
      assert_equal "false", f.inspect
      assert_equal "true", t.to_s
      assert_equal "false", f.to_s
      assert_equal "true|false", "#{t}|#{f}"
      assert_equal "[true, false, true, false]", [t, f, true, false].inspect

      # ! and not
      assert_equal false, (!t)
      assert_equal true, (!f)
      assert_equal false, (not t)
      assert_equal true, (!!t)
      assert_equal false, (!!f)

      # & | ^ evaluate both sides
      assert_equal false, (t & f)
      assert_equal true, (t & t)
      assert_equal false, (f & f)
      assert_equal false, (f & t)
      assert_equal true, (t | f)
      assert_equal false, (f | f)
      assert_equal true, (t | t)
      assert_equal true, (f | t)
      assert_equal true, (t ^ f)
      assert_equal false, (t ^ t)
      assert_equal false, (f ^ f)
      assert_equal true, (f ^ t)

      # == and != take any object
      assert_equal true, (t == true)
      assert_equal false, (t == false)
      assert_equal true, (f == false)
      assert_equal false, (f == true)
      assert_equal false, (t == 1)
      assert_equal false, (f == 0)
      assert_equal false, (f == nil)
      assert_equal false, (t == "true")
      assert_equal false, (t == :true)
      assert_equal true, (t != f)
      assert_equal false, (t != true)
      assert_equal true, (f != nil)

      # && || and conditions
      assert_equal false, (t && f)
      assert_equal true, (t || f)
      assert_equal true, (f || t)
      assert_equal false, (f && t)
      assert_equal false, (f || f)
      assert_equal true, (t && t)
      assert_equal "yes", number_yes_no(t)
      assert_equal "no", number_yes_no(f)
      assert_equal "yes", (number_yes_no(1 < 2))
      assert_equal "no", (number_yes_no(2.5 < 1.0))
      assert_equal "yes", (number_yes_no(1 == 1.0))
      assert_equal "on", describe_bool(t)
      assert_equal "off", describe_bool(f)
      assert_equal "off", (describe_bool(1 > 2))
      assert_equal true, (1 < 2 && 2 < 3)
      assert_equal true, (1 > 2 || 3 > 2)
      assert_equal false, (!(1 < 2))
      flag = false
      3.times { |i| flag = !flag if i.even? }
      assert_equal false, flag
      count = 0
      count += 1 while count < 4 && !(count == 3)
      assert_equal 3, count

      # Kernel and BasicObject methods on bool
      assert_equal false, t.nil?
      assert_equal true, t.frozen?
      assert_equal true, t.equal?(true)
      assert_equal false, f.equal?(true)
      assert_equal false, (t.then { |b| !b })

      # & | ^ always evaluate the right side; && || short-circuit
      log = [] #: Array[String]
      assert_equal true, (t | noisy(log, "or", false))
      assert_equal false, (f & noisy(log, "and", true))
      assert_equal false, (t ^ noisy(log, "xor", true))
      assert_equal true, (t || noisy(log, "never", false))
      assert_equal false, (f && noisy(log, "never", true))
      assert_equal true, (f || noisy(log, "rhs", true))
      assert_equal ["or", "and", "xor", "rhs"], log

      # op-assign
      b = true
      b &= false
      assert_equal false, b
      b |= true
      assert_equal true, b
      b ^= true
      assert_equal false, b
      b ||= true
      assert_equal true, b
      c = nil #: bool?
      c ||= false
      assert_equal false, c

      # through untyped
      u = number_ident(true)
      w = number_ident(false)
      assert_equal false, (!u)
      assert_equal true, (!w)
      assert_equal false, (u & w)
      assert_equal true, (u | w)
      assert_equal true, (u ^ w)
      assert_equal "true", u.to_s
      assert_equal false, w
      assert_equal true, (u == true)
      assert_equal true, (w == false)
      assert_equal true, (u != w)
      assert_equal false, (w == nil)
      assert_equal false, (u == 1)
      assert_equal "yes", (u ? "yes" : "no")
      assert_equal "no", (w ? "yes" : "no")
      assert_equal "yes", number_yes_no(number_ident(true))
      assert_equal "no", number_yes_no(number_ident(nil))
      assert_equal "yes", number_yes_no(number_ident(0))
      assert_equal true, (t & number_ident(1))
      assert_equal false, (t & number_ident(nil))
      assert_equal true, (f | number_ident("s"))
      assert_equal true, (f ^ number_ident(0))
      assert_equal true, (t ^ number_ident(false))

      # in collections
      flags = [true, false, true] #: Array[bool]
      assert_equal 2, (flags.select { |x| x }.size)
      assert_equal true, flags.include?(false)
      assert_equal "{true => 2, false => 1}", (flags.tally).inspect
      assert_equal [false, true, false], flags.map(&:!)
      assert_equal false, (flags.all? { |x| x })
      assert_equal true, (flags.any? { |x| !x })
      assert_equal [false], (flags.reject { |x| x })
      bh = { "on" => true, "off" => false } #: Hash[String, bool]
      assert_equal true, bh["on"]
      assert_equal false, bh["off"]
      assert_nil bh["x"]
      assert_equal "{\"on\" => true, \"off\" => false}", (bh).inspect
      bk = { true => 1, false => 0 } #: Hash[bool, Integer]
      assert_equal 1, bk[true]
      assert_equal 0, (bk[1 > 2])
      assert_equal "{true => 1, false => 0}", (bk).inspect
    end

    # TrueClass/FalseClass/NilClass: markers over Boolean and nil
    def test_trueclass_falseclass_nilclass_markers_over
      assert_equal "TrueClass", (true.class).to_s
      assert_equal "FalseClass", (false.class).to_s
      assert_equal "NilClass", (nil.class).to_s
      bc_flag = 1 > 0
      assert_equal "TrueClass", (bc_flag.class).to_s
      assert_equal true, bc_flag.is_a?(TrueClass)
      assert_equal false, bc_flag.is_a?(FalseClass)
      bc_vals = [true, false, nil, 1] #: Array[untyped]
      bc_seen = bc_vals.map do |e|
        case e
        when TrueClass then "t"
        when FalseClass then "f"
        when NilClass then "n #{e.to_a.inspect} #{e.to_i}"
        else "other"
        end
      end
      assert_equal ["t", "f", "n [] 0", "other"], bc_seen
      assert_equal [], nil.to_a
      assert_equal 0, nil.to_i
      assert_equal "0.0", (nil.to_f).to_s
      bc_opt = nil #: Integer?
      assert_equal true, bc_opt.is_a?(NilClass)
      bc_opt = 3
      assert_equal false, bc_opt.is_a?(NilClass)
      bc_b = true #: bool?
      assert_equal true, bc_b.is_a?(TrueClass)
      assert_equal false, 1.is_a?(TrueClass)
      assert_equal true, (true.class == TrueClass)
      assert_equal "TrueClass", (TrueClass).to_s
      assert_equal "NilClass", (NilClass).to_s
    end
  end

  class NumberBugCaseWhenEqqTest < Minitest::Test
    def test_case_when_uses_eqq
      assert_equal "int", kind_eqq(1)
      assert_equal "float", kind_eqq(2.5)
      assert_equal "true", kind_eqq(true)
      assert_equal "other", kind_eqq("s")
      assert_equal ["1", "Fizz", "Buzz", "FizzBuzz"], ([1, 3, 5, 15].map { |i| fizz(i) })
      assert_equal "zero", sized(0)
      assert_equal "int", sized(7)
    end
  end

  class NumberBugFloatToINonfiniteTest < Minitest::Test
    def test_nonfinite_to_integer_raises
      z = 0.0 #: Float
      inf = 1.0 / z #: Float
      nan = z / z #: Float
      got = [] #: Array[String]
      [inf, -inf, nan].each do |f|
        got << "to_i #{assert_raises(RangeError) { f.to_i }.message}"
        got << "floor #{assert_raises(RangeError) { f.floor }.message}"
        got << "round #{assert_raises(RangeError) { f.round }.message}"
      end
      assert_equal ["to_i Infinity", "floor Infinity", "round Infinity",
                    "to_i -Infinity", "floor -Infinity", "round -Infinity",
                    "to_i NaN", "floor NaN", "round NaN"], got
      # was an uncaught error ending the old file
      e = assert_raises(FloatDomainError) { inf.ceil }
      assert_equal "Infinity", e.message
    end
  end

  class NumberBugIntegerPowZeroNegativeTest < Minitest::Test
    def test_zero_to_negative_power_raises
      z = 0 #: Integer
      e = assert_raises(ZeroDivisionError) { z**-1 }
      assert_equal "divided by 0", e.message
      # was an uncaught error ending the old file
      e = assert_raises(ZeroDivisionError) { 0**-2 }
      assert_equal "divided by 0", e.message
    end
  end

  class NumberBugOperatorNameCollisionTest < Minitest::Test
    def test_operator_and_named_methods_stay_distinct
      m = Meter.new
      assert_equal 2, (m + 1)
      assert_equal 3, m.plus(1)
      assert_equal 10, m[1]
      assert_equal 100, m.idx(1)
      assert_equal -1, (-m)
      assert_equal -2, m.neg
      assert_equal 10, (m / 10)
      assert_equal 100, m.div(10)
      assert_equal 9, (m**3)
      assert_equal 27, m.pow(3)
      assert_equal true, m.empty?
      assert_equal false, m.empty_q
    end
  end

  class NumberBugsTest < Minitest::Test
    # 0d decimal-prefix literals
    def test_0d_decimal_prefix_literals
      assert_equal 17, 0d17
      assert_equal 0, 0D0
    end

    # Integer#chr past 127 is one byte; out of range raises RangeError
    def test_integer_chr_past_127_is
      assert_equal 1, 200.chr.bytesize
      assert_equal 1, 255.chr.bytesize
      assert_equal 1, 128.chr.bytesize
      assert_equal 1, 127.chr.bytesize
      assert_equal 200, 200.chr.ord
      assert_equal 255, 255.chr.ord
      e = assert_raises(RangeError) { 256.chr }
      assert_equal "256 out of char range", e.message
      e = assert_raises(RangeError) { -1.chr }
      assert_equal "-1 out of char range", e.message
    end

    # clamp with min > max raises ArgumentError
    def test_clamp_with_min_max_raises
      e = assert_raises(ArgumentError) { 5.clamp(10, 1) }
      assert_equal "min argument must be less than or equal to max argument", e.message
      e = assert_raises(ArgumentError) { 2.5.clamp(3.0, 1.0) }
      assert_equal "min argument must be less than or equal to max argument", e.message
    end

    # Integer == falls back to the other side's == for a user object
    def test_integer_eq_falls_back_to_user_eq
      one = One.new
      assert_equal true, (1 == one)
      assert_equal true, (1.0 == one)
      assert_equal false, (2 == one)
      assert_equal true, (one == 1)
    end

    # <=> with NaN is nil
    def test_with_nan_is_nil
      nan_zero = 0.0 #: Float
      nan = nan_zero / nan_zero #: Float
      assert_nil (nan <=> 1.0)
      assert_nil (1.0 <=> nan)
      assert_nil (nan <=> nan)
    end

    # Float#** rounds like C pow
    def test_float_rounds_like_c_pow
      third = 1.0 / 3.0 #: Float
      assert_equal "2.0", ((8.0**third)).inspect
      assert_equal "3.0", ((27.0**third)).inspect
      assert_equal "9.999999999999998", ((1000.0**third)).inspect
      assert_equal "0.00010000000000000002", ((0.1**4.0)).inspect
      assert_equal "1.0303010000000001", ((1.01**3.0)).inspect
      assert_equal "9605.960100000002", ((9.9**4.0)).inspect
      assert_equal "5.656854249492381", ((2.0**2.5)).inspect
      assert_equal "316.22776601683796", ((10.0**2.5)).inspect
      pow_x = 1.1 #: Float
      assert_equal "2.5937424601000023", ((pow_x**10)).inspect
      neg = -1.1 #: Float
      assert_equal "-1.3310000000000004", ((neg**3)).inspect
      assert_equal "-0.064", (((neg - 1.4)**-3.0)).inspect
      assert_equal "9.849155306759329", ((0.5**-3.3)).inspect
      assert_equal "1.0e+305", ((1e10**30.5)).inspect
      assert_equal "2.7182816941320818", ((1.0000001**1e7)).inspect
      assert_equal "2.73552396956703e-07", ((7.5**-7.5)).inspect
    end

    # Float#** is exact wherever the power is a Float, on every path (decision 390)
    def test_float_pow_exact_powers
      bad = 0
      (2..40).each do |b|
        k = 1
        while b**k < 2**53
          bad += 1 if b.to_f**k.to_f != (b**k).to_f
          bad += 1 if (-b).to_f**k.to_f != ((-b)**k).to_f
          bad += 1 if b.to_f**-k.to_f != 1.0 / (b**k).to_f && b & (b - 1) == 0
          k += 1
        end
      end
      (1..1074).each { |k| bad += 1 if 2.0**-k.to_f != Math.ldexp(1.0, -k) }
      (1..1023).each { |k| bad += 1 if 2.0**k.to_f != Math.ldexp(1.0, k) }
      assert_equal 0, bad
    end

    # Float#** past the Float range, and large exponents in range
    def test_float_pow_large_exponents
      assert_equal "8.104774656527117e-177", ((2.0 / 3.0)**1000.0).inspect
      assert_equal "2.3365249867684727e-308", ((2.0 / 3.0)**1747.0).inspect
      assert_equal "1.9423975992963885e+130", (1.0000001**3.0e9).inspect
      assert_equal "1.43959991515206e+304", (0.999**-700000.0).inspect
      assert_equal "3.4336838202925124e+30", (3.0**64.0).inspect
      assert_equal "0.002243200793301555", (1.1**-64.0).inspect
      assert_equal "-405.26506222962956", ((-1.1)**63.0).inspect
      assert_equal "-0.0", ((-0.5)**2001.0).inspect
      assert_equal "0.0", ((-0.5)**2000.0).inspect
      assert_equal "-Infinity", ((-2.0)**2001.0).inspect
      assert_equal "Infinity", (2.0**2000.0).inspect
      assert_equal "-Infinity", ((-0.5)**-2001.0).inspect
      assert_equal "0.0", (1e-300**3.0).inspect
    end

    # Float#round rounds half away from zero
    def test_float_round_rounds_half_away
      assert_equal 3, 2.5.round
      assert_equal 1, 0.5.round
      assert_equal -3, -2.5.round
      assert_equal -1, -0.5.round
      assert_equal 2, 1.5.round
      assert_equal 4, 3.5.round
    end

    # Float#to_s switches to exponent form at 1e16, or integral values from 1e15
    def test_float_to_s_switches_to
      assert_equal "1.0e+15", (1e15).inspect
      assert_equal "1.5e+15", (1.5e15).inspect
      assert_equal "-1.2e+15", (-1.2e15).inspect
      assert_equal "1.234567890123456e+15", 1234567890123456.0.to_s
      assert_equal "9.999999999999998e+15", (9999999999999998.0).inspect
      assert_equal "9.007199254740992e+15", (9_007_199_254_740_993.to_f).inspect
      assert_equal "2.0e+15", "#{2e15}"
      assert_equal "1234567890123456.8", (1234567890123456.7).inspect
      assert_equal "999999999999999.9", (999999999999999.9).inspect
      assert_equal "100000000000000.0", (100000000000000.0).inspect
    end

    # Integer/Float mixed operators on typed locals, and an Integer accumulator fed Floats
    def test_integer_float_mixed_operators_on
      build_a = 7 #: Integer
      build_f = 1.5 #: Float
      assert_equal true, (1 < 1.5)
      assert_equal "1.4142135623730951", ((2**0.5)).inspect
      assert_equal "10.5", ((build_a * build_f)).inspect
      assert_equal "8.5", ((build_f + build_a)).inspect
      assert_equal 1, (build_a <=> build_f)
      assert_equal true, (1.between?(0.5, 2.5))
      assert_equal "2.5", ((build_a.clamp(0, 2.5))).inspect
      total = 0
      [1.5, 2.25].each { |add| total += add }
      assert_equal "3.75", (total).inspect
    end

    # Integer/Float mixed operators on untyped values
    def test_integer_float_mixed_operators_on_2
      mix_v = 5 #: untyped
      mix_w = 1.5 #: untyped
      assert_equal "6.5", ((mix_v + 1.5)).inspect
      assert_equal "7.5", ((mix_v * mix_w)).inspect
      assert_equal "6.5", ((mix_w + mix_v)).inspect
      assert_equal false, (mix_v < mix_w)
    end

    # Integer op Float literal, and Float#clamp with Integer bounds
    def test_integer_op_float_literal_and
      mixed_a = 7 #: Integer
      assert_equal "3.5", ((mixed_a / 2.0)).inspect
      assert_equal "3.0", ((1 + 2.0)).inspect
      assert_equal "7.0", ((mixed_a * 1.0)).inspect
      assert_equal "7.0", ((mixed_a - 0.0)).inspect
      mixed_x = 2.5 #: Float
      # inspect, so an Integer bound returned as a Float would fail
      assert_equal "2", mixed_x.clamp(1, 2).inspect
      assert_equal "1", 0.5.clamp(1, 2).inspect
    end

    # && on numbers yields the right side
    def test_on_numbers_yields_the_right
      and_a = 1 #: Integer
      and_v = and_a && 2
      assert_equal 2, and_v
      and_b = 2.5 #: Float
      and_w = and_b && 1.0
      assert_equal "1.0", (and_w).inspect
    end

    # 0 and 0.0 are truthy
    def test_0_and_0_0_are
      truth_x = 0 #: Integer
      truth_f = 0.0 #: Float
      truth_s = nil #: String?
      truth_s = "0 is truthy" if truth_x
      assert_equal "0 is truthy", truth_s
      assert_equal "0.0 is truthy", (truth_f ? "0.0 is truthy" : "0.0 is falsy")
      truth_n = 3
      while truth_n
        truth_n -= 1
        break if truth_n.zero?
      end
      assert_equal 0, truth_n
    end

    # a parenthesized literal as a receiver
    def test_a_parenthesized_literal_as_a
      assert_equal -8, ((-2)**3)
      assert_equal 3, (3)
      assert_equal "2.5", ((2.5)).inspect
      assert_equal 0, (-0)
      assert_equal false, (7).even?
      assert_equal 1, (1.5).floor
      paren_a = 1 #: Integer
      assert_equal 2, (paren_a && 2)
      assert_equal 1, paren_a
    end

    # <=> on untyped numbers
    def test_cmp_on_untyped_numbers
      cmp_a = number_ident_cmp(3)
      cmp_b = number_ident_cmp(4)
      assert_equal -1, (cmp_a <=> cmp_b)
      assert_equal 0, (number_ident_cmp(2.5) <=> number_ident_cmp(2.5))
      assert_equal 1, (cmp_a <=> 1)
      assert_equal -1, (number_ident_cmp(1.5) <=> 2.0)
    end

    # #hash on untyped values matches the typed hash
    def test_hash_on_untyped_matches_typed
      hash_v = ident_hash(5)
      hash_w = ident_hash("s")
      assert_equal true, (hash_v.hash == 5.hash)
      assert_equal false, (hash_v.hash == 6.hash)
      assert_equal true, (hash_w.hash == "s".hash)
    end

    # a local or a branch holding both Integer and Float is untyped, as MRI has it
    def test_a_local_or_a_branch
      join_x = 1
      join_x = 2.5
      assert_equal "2.5", (join_x).to_s
      join_c = true #: bool
      join_d = false #: bool
      assert_equal "1", (join_c ? 1 : 2.5).inspect
      assert_equal "2.5", (join_d ? 1 : 2.5).inspect
      join_y = join_d ? 2.5 : 1
      assert_equal "2", (join_y + 1).inspect
    end
  end

  class NumberCollectionsTest < Minitest::Test
    def test_optional_numbers_and_collections
      ints = [5, 3, 8, 1] #: Array[Integer]
      floats = [1.5, -2.25, 4.0] #: Array[Float]

      # Integer? from a method
      r = find_over(ints, 4)
      q = find_over(ints, 100)
      assert_equal 5, r
      assert_nil q
      assert_equal 5, (r || 0)
      assert_equal -1, (q || -1)
      assert_equal true, q.nil?
      assert_equal false, r.nil?
      assert_equal 6, r&.succ
      assert_nil q&.succ
      assert_equal 10, double_or_zero(r)
      assert_equal 0, double_or_zero(q)
      assert_equal "1.5", (number_half(3.0)).inspect
      assert_equal "-1.0", (number_half(nil)).inspect
      assert_equal "big 42", number_describe(42)
      assert_equal "small 4", number_describe(3)
      assert_equal "none", number_describe(nil)

      # Hash lookups are T?
      h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      fh = { "pi" => 3.14 } #: Hash[String, Float]
      bh = { "on" => true } #: Hash[String, bool]
      x = h["zz"]
      y = fh["zz"]
      z = bh["zz"]
      assert_equal 1, h["a"]
      assert_nil x
      assert_equal 0, (x || 0)
      assert_equal "3.14", (fh["pi"]).inspect
      assert_nil y
      assert_equal "2.72", ((y || 2.72)).inspect
      assert_nil z
      # puts/print of these nils stay in testdata/run/number_output.rb
      assert_equal "[][][][1][3.14][true]", "[#{x}][#{y}][#{z}][#{h["a"]}][#{fh["pi"]}][#{bh["on"]}]"
      assert_equal "", x.to_s
      assert_equal "", y.to_s
      assert_equal "", z.to_s
      assert_equal true, (!x)
      assert_equal true, (!!h["a"])
      assert_equal false, (!h["a"])
      assert_equal true, (x == nil)
      assert_equal true, (h["a"] == 1)
      assert_equal true, (h["a"] == 1.0)
      assert_equal false, (x == 0)
      assert_equal true, (fh["pi"] == 3.14)
      assert_equal true, (bh["on"] == true)
      w = h["b"]
      w_sum = nil #: Integer?
      w_sum = w + 10 if w
      assert_equal 12, w_sum
      assert_equal true, w&.even?
      c = h["q"]
      c ||= 7
      assert_equal 8, (c + 1)
      d = (h["a"] || 0) + 100
      assert_equal 101, d
      e = nil #: Float?
      e = 2.5 if h.size > 0
      assert_equal "5.0", ((e ? e * 2.0 : 0.0)).inspect
      counts = {} #: Hash[String, Integer]
      %w[a b a c a].each { |k| counts[k] = (counts[k] || 0) + 1 }
      assert_equal "{\"a\" => 3, \"b\" => 1, \"c\" => 1}", (counts).inspect

      # numbers as hash keys
      mk = { 1 => "int", 1.0 => "float", true => "bool" } #: Hash[untyped, String]
      assert_equal "{1 => \"int\", 1.0 => \"float\", true => \"bool\"}", (mk).inspect
      assert_equal 3, mk.size
      assert_equal "int", mk[1]
      assert_equal "float", mk[1.0]
      assert_equal "bool", mk[true]
      assert_nil mk[2]
      fk = { 0.5 => "half", -1.25 => "neg" } #: Hash[Float, String]
      assert_equal "half", fk[0.5]
      assert_equal "neg", fk[-1.25]
      assert_nil fk[2.0]
      assert_equal "[0.5, -1.25]", (fk.keys).inspect
      zk = { 0.0 => "zero" } #: Hash[Float, String]
      assert_equal "zero", zk[-0.0]
      ik = { 1 => 10, 2 => 20 } #: Hash[Integer, Integer]
      assert_equal [10, 40], (ik.map { |k, v| k * v })
      assert_equal "[1.0, 2.0]", (ik.keys.map(&:to_f)).inspect
      assert_equal 20, ik.values.max

      # Enumerable over numbers
      assert_equal 17, (ints.reduce(0) { |s, v| s + v })
      assert_equal "3.25", ((floats.reduce(0.0) { |s, v| s + v })).inspect
      assert_equal 120, (ints.inject(1) { |s, v| s * v })
      assert_equal "[2.5, 1.5, 4.0, 0.5]", ((ints.map { |v| v.to_f / 2.0 })).inspect
      assert_equal [2, -2, 4], (floats.map { |v| v.round })
      assert_equal [1, -3, 4], floats.map(&:floor)
      assert_equal [2, -2, 4], floats.map(&:ceil)
      assert_equal [8], ints.select(&:even?)
      assert_equal [8], ints.reject(&:odd?)
      assert_equal [6, 4, 9, 2], ints.map(&:succ)
      assert_equal [-5, -3, -8, -1], ints.map(&:-@)
      assert_equal true, ints.include?(8)
      assert_equal false, ints.include?(9)
      assert_equal true, floats.include?(4.0)
      assert_equal false, floats.include?(4.5)
      assert_equal "{5 => 1, 3 => 1, 8 => 1, 1 => 1}", (ints.tally).inspect
      assert_equal "{1 => 2, 2 => 1}", (([1, 1, 2].tally)).inspect
      assert_equal "[1.5, -0.5]", (([1.5, 1.5, -0.5].uniq)).inspect
      assert_equal [1, 2], ([1, 2, 1].uniq)
      assert_equal false, ints.any?(&:zero?)
      assert_equal true, ints.all?(&:positive?)
      assert_equal true, floats.none?(&:nan?)
      assert_equal "{true => [5, 3, 1], false => [8]}", (ints.group_by(&:odd?)).inspect
      assert_equal 1, ints.min_by(&:abs)
      assert_equal "[4.0, 1.5, -2.25]", ((floats.sort_by { |f| -f })).inspect
      assert_equal "Hi", ([72, 105].map(&:chr).join)
      assert_equal "5+3+8+1", ints.map(&:to_s).join("+")
      assert_equal [1, 3], ints.sort.first(2)
      assert_equal -3, floats.min.floor
      assert_equal 9, ints.max.succ
    end
  end

  class NumberComparableTest < Minitest::Test
    def test_comparable_on_numbers
      a = 7 #: Integer
      b = -7 #: Integer
      x = 2.5 #: Float

      # Comparable#between? is inclusive
      assert_equal true, (a.between?(1, 7))
      assert_equal true, (a.between?(7, 9))
      assert_equal false, (a.between?(8, 9))
      assert_equal false, (a.between?(1, 6))
      assert_equal true, (b.between?(-7, -7))
      assert_equal true, (b.between?(-8, 0))
      assert_equal true, (0.between?(b, a))
      assert_equal false, (a.between?(9, 1))
      assert_equal true, (x.between?(2.0, 3.0))
      assert_equal true, (x.between?(2.5, 2.5))
      assert_equal false, (x.between?(3.0, 4.0))
      assert_equal false, (x.between?(-1.0, 2.4))

      # Comparable#clamp
      assert_equal 5, (a.clamp(1, 5))
      assert_equal 8, (a.clamp(8, 10))
      assert_equal 7, (a.clamp(1, 10))
      assert_equal 7, (a.clamp(7, 7))
      assert_equal -3, (b.clamp(-3, 3))
      assert_equal "1.0", ((x.clamp(0.0, 1.0))).inspect
      assert_equal "3.0", ((x.clamp(3.0, 4.0))).inspect
      assert_equal "2.5", ((x.clamp(1.0, 4.0))).inspect
      assert_equal "-1.0", ((-x).clamp(-1.0, 1.0)).inspect

      # <=> drives sorting and min/max
      ints = [3, -1, 0, 10, -20, 3] #: Array[Integer]
      floats = [2.5, -0.5, 1.0, -3.25, 1e20, 0.0] #: Array[Float]
      assert_equal [-20, -1, 0, 3, 3, 10], ints.sort
      assert_equal "[-3.25, -0.5, 0.0, 1.0, 2.5, 1.0e+20]", (floats.sort).inspect
      assert_equal 10, ints.max
      assert_equal -20, ints.min
      assert_equal "1.0e+20", (floats.max).inspect
      assert_equal "-3.25", (floats.min).inspect
      assert_equal [10, 3, 3, 0, -1, -20], ints.sort.reverse
      assert_equal "[1.0e+20, 2.5, 1.0, 0.0, -0.5, -3.25]", (floats.sort.reverse).inspect
      assert_equal [10, 3, 3, 0, -1, -20], (ints.sort_by { |i| -i })
      assert_equal 0, (ints.min_by { |i| i.abs })
      assert_equal "1.0e+20", ((floats.max_by { |f| f.abs })).inspect
      assert_equal "[1.0, 2.0]", (([2.0, 1.0].sort)).inspect
      assert_equal [0, -1, -1, 1, -1, 0], (ints.map { |i| i <=> 3 })
      assert_equal [1, -1, 0, -1, 1, -1], (floats.map { |f| f <=> 1.0 })

      # mixed-in methods chained, in blocks, and on reopened classes
      assert_equal 6, (a.clamp(1, 5).succ)
      assert_equal 1, (x.clamp(0.0, 1.0).floor)
      assert_equal true, (a.clamp(1, 9).between?(6, 8))
      assert_equal [3, 0, 3], (ints.select { |i| i.between?(0, 5) })
      assert_equal "[1.0, -0.5, 1.0, -1.0, 1.0, 0.0]", ((floats.map { |f| f.clamp(-1.0, 1.0) })).inspect
      assert_equal 5, 3.at_least(5)
      assert_equal 7, 7.at_least(5)
      assert_equal "1.5", (1.5.at_least(0.5)).inspect
      assert_equal "2.5", (1.5.at_least(2.5)).inspect
      assert_equal true, 0.5.unit?
      assert_equal false, 1.5.unit?
      assert_equal 9, 12.digit
      assert_equal 0, -3.digit
      assert_equal 4, 4.digit
      lo = 2 #: Integer
      hi = 4 #: Integer
      assert_equal true, (3.between?(lo, hi))
      assert_equal 4, (9.clamp(lo, hi))
      assert_equal 6, (lo + hi).clamp(lo, hi * 2)

      # through untyped
      v = number_ident(5)
      w = number_ident(2.5)
      assert_equal true, (v.between?(1, 10))
      assert_equal "3", v.clamp(1, 3).inspect
      assert_equal false, (w.between?(3.0, 4.0))
      assert_equal "1.0", ((w.clamp(0.0, 1.0))).inspect
    end
  end

  class NumberFloatTest < Minitest::Test
    def test_float_behaviour
      x = 2.5 #: Float
      y = -1.25 #: Float
      z = 0.0 #: Float
      inf = 1.0 / z #: Float
      nan = z / z #: Float

      # arithmetic
      assert_equal "1.25", ((x + y)).inspect
      assert_equal "3.75", ((x - y)).inspect
      assert_equal "-3.125", ((x * y)).inspect
      assert_equal "-2.0", ((x / y)).inspect
      assert_equal "-0.5", ((y / x)).inspect
      assert_equal "6.25", ((x**2.0)).inspect
      assert_equal "1.4142135623730951", ((2.0**0.5)).inspect
      assert_equal "0.25", ((4.0**-1.0)).inspect
      assert_equal "1.0", ((x**0.0)).inspect
      assert_equal "3.0", ((9.0**0.5)).inspect
      assert_equal "0.30000000000000004", ((0.1 + 0.2)).inspect
      assert_equal "3.3000000000000003", ((1.1 + 2.2)).inspect
      assert_equal "0.30000000000000004", ((0.1 * 3.0)).inspect
      assert_equal "0.3333333333333333", ((1.0 / 3.0)).inspect
      assert_equal "0.6666666666666666", ((2.0 / 3.0)).inspect
      assert_equal "2.5", ((10.0 / 4.0)).inspect
      assert_equal "-2.5", ((-x)).inspect
      assert_equal "1.25", ((-y)).inspect
      assert_equal "-0.0", ((-z)).inspect
      assert_equal "-0.0", ((z * -1.0)).inspect
      assert_equal "2.5", (x.abs).inspect
      assert_equal "1.25", (y.abs).inspect
      assert_equal "0.0", ((-z).abs).inspect
      assert_equal "Infinity", ((-inf).abs).inspect

      # Integer literals where a Float is expected
      assert_equal "3.5", ((x + 1)).inspect
      assert_equal "5.0", ((x * 2)).inspect
      assert_equal "1.25", ((x / 2)).inspect
      assert_equal "6.25", ((x**2)).inspect
      assert_equal "-0.5", ((x - 3)).inspect

      # division by zero is IEEE, not an exception
      assert_equal "Infinity", (inf).inspect
      assert_equal "-Infinity", ((-1.0 / z)).inspect
      assert_equal "NaN", (nan).inspect
      assert_equal "-Infinity", ((1.0 / (-z))).inspect
      assert_equal "NaN", ((inf - inf)).inspect
      assert_equal "NaN", ((inf * z)).inspect
      assert_equal "Infinity", ((x / 0)).inspect
      assert_equal "-Infinity", ((y / 0)).inspect
      assert_equal "Infinity", ((inf + 1.0)).inspect
      assert_equal "0.0", ((1.0 / inf)).inspect

      # <=> and relational operators
      assert_equal -1, (x <=> 3.0)
      assert_equal 0, (x <=> 2.5)
      assert_equal 1, (x <=> y)
      assert_equal 0, (z <=> -z)
      assert_equal 1, (inf <=> x)
      assert_equal true, (x < 3.0)
      assert_equal false, (x < 2.5)
      assert_equal true, (x <= 2.5)
      assert_equal false, (x > 2.5)
      assert_equal true, (x > y)
      assert_equal false, (x >= 3.0)
      assert_equal true, (x >= 2.5)
      assert_equal false, (nan < 1.0)
      assert_equal false, (nan > 1.0)
      assert_equal false, (nan <= nan)
      assert_equal true, (-inf < y)

      # == and != across Float and Integer
      assert_equal true, (x == 2.5)
      assert_equal true, (2.0 == 2)
      assert_equal false, (x == 2)
      assert_equal true, (-z == z)
      assert_equal false, (x == "2.5")
      assert_equal false, (x == nil)
      assert_equal false, (x == true)
      assert_equal false, (nan == nan)
      assert_equal true, (inf == inf)
      assert_equal false, (x != 2.5)
      assert_equal true, (x != 3.0)
      assert_equal true, (nan != nan)
      w = 7 #: Integer
      assert_equal true, (7.0 == w)
      assert_equal true, (w.to_f == 7.0)
      assert_equal "3.5", ((w.to_f / 2.0)).inspect

      # to_i truncates, floor and ceil return Integer
      assert_equal 2, x.to_i
      assert_equal -1, y.to_i
      assert_equal 1, 1.99.to_i
      assert_equal -1, -1.99.to_i
      assert_equal 0, -0.5.to_i
      assert_equal 10000000000, 1e10.to_i
      assert_equal 2, x.floor
      assert_equal -2, y.floor
      assert_equal 3, 3.0.floor
      assert_equal -3, -3.0.floor
      assert_equal -1, -0.5.floor
      assert_equal 3, x.ceil
      assert_equal -1, y.ceil
      assert_equal 3, 3.0.ceil
      assert_equal -3, -3.0.ceil
      assert_equal 0, -0.5.ceil
      assert_equal 1, 0.1.ceil

      # round (halves are in number_bug_float_round_half)
      assert_equal 1, 1.4.round
      assert_equal 2, 1.6.round
      assert_equal -1, -1.4.round
      assert_equal -2, -1.6.round
      assert_equal 3, 3.0.round
      assert_equal 2, 2.49999.round
      assert_equal 0, 0.2.round
      assert_equal 0, 0.49999999999999994.round
      assert_equal 3, 2.5000000000000004.round
      assert_equal 0, -0.4.round
      assert_equal 4503599627370497, 4503599627370497.0.round

      # predicates and to_f
      assert_equal "2.5", (x.to_f).inspect
      assert_equal true, z.zero?
      assert_equal true, (-z).zero?
      assert_equal false, x.zero?
      assert_equal false, inf.zero?
      assert_equal false, x.nan?
      assert_equal true, nan.nan?
      assert_equal false, inf.nan?

      # to_s and inspect
      assert_equal "6.0", (6.0).inspect
      assert_equal "6.0", 6.0.to_s
      assert_equal "100.0", (100.0).inspect
      assert_equal "1.5", 1.5.to_s
      assert_equal "-2.75", (-2.75).inspect
      assert_equal "-0.0", (-0.0).inspect
      assert_equal "0.0", 0.0.to_s
      assert_equal "100.0", (1e2).inspect
      assert_equal "12.0", (12.0e0).inspect
      assert_equal "1000.5", (1_000.5).inspect
      assert_equal "1000.0", (1e3).inspect
      assert_equal "0.02", (2E-2).inspect
      assert_equal "3.14159", (3.14159).inspect
      assert_equal "123456789.12345679", (123456789.123456789).inspect
      assert_equal "123456789012345.6", (123456789012345.6).inspect
      assert_equal "999999999999999.0", (999999999999999.0).inspect
      assert_equal "100000000000000.5", (100000000000000.5).inspect
      assert_equal "1234567890123456.8", (1234567890123456.7).inspect
      assert_equal "-999999999999999.9", (-999999999999999.9).inspect
      assert_equal "1.0e+16", (1e16).inspect
      assert_equal "-1.0e+16", (-1e16).inspect
      assert_equal "1.0e+20", (1.0e20).inspect
      assert_equal "1.5e+300", (1.5e300).inspect
      assert_equal "1.7976931348623157e+308", (1.7976931348623157e308).inspect
      assert_equal "0.001", (0.001).inspect
      assert_equal "0.0001", (0.0001).inspect
      assert_equal "0.00011", (0.00011).inspect
      assert_equal "1.0e-05", (0.00001).inspect
      assert_equal "0.000123", (0.000123).inspect
      assert_equal "1.23e-05", (1.23e-5).inspect
      assert_equal "-1.23e-05", (-1.23e-5).inspect
      assert_equal "5.0e-324", (5e-324).inspect
      assert_equal "Infinity", inf.to_s
      assert_equal "-Infinity", (-inf).to_s
      assert_equal "NaN", nan.to_s
      assert_equal "2.5", (x).to_s
      assert_equal "-1.25", (y).to_s
      assert_equal "0.0", (z).to_s
      assert_equal "2.5|-1.25|-0.0|1.0e+20|Infinity", "#{x}|#{y}|#{-0.0}|#{1e20}|#{inf}"
      assert_equal "[2.5, -1.25, 0.0, 2.5e+20, -0.0, Infinity, NaN]", (([x, y, z, 2.5e20, -0.0, inf, nan])).inspect

      # op-assign
      f = 1.5
      f += 1.0
      f -= 0.25
      f *= 4.0
      f /= 3.0
      f **= 2.0
      assert_equal "9.0", (f).inspect

      # Kernel methods on Float
      assert_equal "5.0", ((x.then { |v| v * 2.0 })).inspect
      assert_equal true, x.frozen?
      assert_equal false, x.nil?
      assert_equal false, (!x)
    end
  end

  class NumberIntegerTest < Minitest::Test
    def test_integer_behaviour
      a = 7 #: Integer
      b = -7 #: Integer
      z = 0 #: Integer

      # + - * and unary minus
      assert_equal 10, (a + 3)
      assert_equal -3, (a - 10)
      assert_equal -21, (a * -3)
      assert_equal 0, (z * b)
      assert_equal -14, (b + b)
      assert_equal -7, (-a)
      assert_equal 7, (-b)
      assert_equal 0, (-z)
      assert_equal 7, (- -a)
      assert_equal -4, (-2**2)
      assert_equal 2, (-2.abs)

      # / floors toward negative infinity
      assert_equal 3, (a / 2)
      assert_equal -4, (b / 2)
      assert_equal -4, (a / -2)
      assert_equal 3, (b / -2)
      assert_equal 0, (z / 5)
      assert_equal -2, (-6 / 3)
      assert_equal -2, (6 / -3)
      assert_equal 0, (1 / 7)
      assert_equal -1, (-1 / 7)

      # % takes the divisor's sign
      assert_equal 1, (a % 3)
      assert_equal 2, (b % 3)
      assert_equal -2, (a % -3)
      assert_equal -1, (b % -3)
      assert_equal 0, (-6 % 3)
      assert_equal 0, (z % 7)
      assert_equal 6, (-1 % 7)
      assert_equal -6, (1 % -7)

      # ** with non-negative exponents
      assert_equal 1024, (2**10)
      assert_equal 1, (0**0)
      assert_equal 1, (5**0)
      assert_equal 0, (0**3)
      assert_equal 1, (1**100)
      assert_equal -343, (b**3)
      assert_equal 49, (b**2)
      assert_equal 4611686018427387904, (2**62)

      # abs, succ, pred
      assert_equal 7, a.abs
      assert_equal 7, b.abs
      assert_equal 0, z.abs
      assert_equal 8, a.succ
      assert_equal 6, a.pred
      assert_equal -6, b.succ
      assert_equal -1, z.pred
      assert_equal 0, -1.succ

      # <=> and relational operators
      assert_equal -1, (a <=> 8)
      assert_equal 0, (a <=> 7)
      assert_equal 1, (a <=> 6)
      assert_equal -1, (b <=> a)
      assert_equal 1, (a <=> b)
      assert_equal true, (a < 8)
      assert_equal false, (a < 7)
      assert_equal true, (a <= 7)
      assert_equal false, (a <= 6)
      assert_equal false, (a > 7)
      assert_equal true, (a > 6)
      assert_equal false, (a >= 8)
      assert_equal true, (a >= 7)
      assert_equal true, (b < z)

      # == and != (Integer#== takes any object)
      assert_equal true, (a == 7)
      assert_equal false, (a == 8)
      assert_equal true, (a == 7.0)
      assert_equal false, (a == 7.5)
      assert_equal false, (a == "7")
      assert_equal false, (a == nil)
      assert_equal false, (a == true)
      assert_equal true, (z == 0.0)
      assert_equal true, (z == -0.0)
      assert_equal false, (a != 7)
      assert_equal true, (a != 8)
      assert_equal true, (a != "7")

      # even? odd? zero? positive? negative?
      assert_equal true, 4.even?
      assert_equal false, 4.odd?
      assert_equal false, b.even?
      assert_equal true, b.odd?
      assert_equal true, z.even?
      assert_equal true, -4.even?
      assert_equal true, -3.odd?
      assert_equal true, z.zero?
      assert_equal false, a.zero?
      assert_equal false, b.zero?
      assert_equal true, a.positive?
      assert_equal false, b.positive?
      assert_equal false, z.positive?
      assert_equal false, a.negative?
      assert_equal true, b.negative?
      assert_equal false, z.negative?

      # to_i, to_f, to_s, inspect, hash
      assert_equal 7, a.to_i
      assert_equal -7, b.to_i
      assert_equal "7.0", (a.to_f).inspect
      assert_equal "-7.0", (b.to_f).inspect
      assert_equal "0.0", (z.to_f).inspect
      assert_equal "1000000.0", (1_000_000.to_f).inspect
      assert_equal "7", a.to_s
      assert_equal "-7", b.to_s
      assert_equal "0", z.to_s
      assert_equal "7", a.inspect
      assert_equal "-7", b.inspect
      assert_equal 7, a
      assert_equal -7, b
      assert_equal 0, z
      assert_equal "7|-7|0", "#{a}|#{b}|#{z}"
      assert_equal true, (a.hash == 7.hash)
      assert_equal false, (a.hash == b.hash)

      # chr in ASCII
      assert_equal "A", 65.chr
      assert_equal "a", 97.chr
      assert_equal "0", 48.chr
      assert_equal "~", 126.chr
      assert_equal " ", 32.chr
      assert_equal "\n", 10.chr
      assert_equal [0], 0.chr.bytes # MRI's chr is US-ASCII and inspects as "\x00"; rugaroo's strings are all UTF-8 ("\u0000")

      # literals
      assert_equal 1000000, 1_000_000
      assert_equal 255, 0xff
      assert_equal 255, 0XFF
      assert_equal 10, 0b1010
      assert_equal 15, 0o17
      assert_equal 15, 017
      assert_equal -16, -0x10
      assert_equal 0, 0
      assert_equal 9223372036854775807, 9_223_372_036_854_775_807
      assert_equal -9223372036854775807, -9_223_372_036_854_775_807
      assert_equal -9223372036854775808, (-9_223_372_036_854_775_807 - 1)

      # op-assign
      x = 10
      x += 5
      x -= 3
      x *= 2
      x /= 5
      x %= 3
      assert_equal 1, x
      x = -3
      x **= 3
      assert_equal -27, x
      x /= 2
      assert_equal -14, x

      # Kernel and BasicObject methods on Integer
      assert_equal 14, (a.then { |v| v * 2 })
      assert_equal true, a.equal?(7)
      assert_equal false, a.equal?(8)
      assert_equal true, a.frozen?
      assert_equal false, a.nil?
      assert_equal false, (!a)
      assert_equal false, (!z)

      # ZeroDivisionError
      e = assert_raises(ZeroDivisionError) { a / z }
      assert_equal "divided by 0", e.message
      assert_equal "ZeroDivisionError", e.class.to_s
      e = assert_raises(ZeroDivisionError) { a % z }
      assert_equal "divided by 0", e.message
      se = assert_raises(StandardError) { z / z }
      assert_equal "divided by 0", se.message
      # was an uncaught error ending the old file
      e = assert_raises(ZeroDivisionError) { b / z }
      assert_equal "divided by 0", e.message
    end
  end

  class NumberIteratorsTest < Minitest::Test
    def test_integer_iterators
      seen = [] #: Array[Integer]
      3.times { |i| seen << i }
      assert_equal [0, 1, 2], seen

      # zero and negative counts run nothing
      seen = []
      0.times { |i| seen << i }
      -2.times { |i| seen << i }
      5.upto(4) { |i| seen << i }
      0.downto(1) { |i| seen << i }
      assert_equal [], seen

      # next and break
      seen = []
      10.times do |i|
        next if i.odd?
        break if i > 6

        seen << i
      end
      assert_equal [0, 2, 4, 6], seen
      seen = []
      1.upto(10) do |i|
        break if i == 4

        seen << i
      end
      10.downto(1) do |i|
        next unless i % 4 == 0

        seen << i
      end
      assert_equal [1, 2, 3, 8, 4], seen

      # upto and downto bounds are inclusive
      seen = []
      1.upto(4) { |i| seen << i * i }
      -2.upto(-1) { |i| seen << i }
      3.upto(3) { |i| seen << i }
      assert_equal [1, 4, 9, 16, -2, -1, 3], seen
      assert_equal [3, 2, 1, 0, -1], (countdown(3, -1))
      assert_equal [], (countdown(0, 1))
      assert_equal [2], (countdown(2, 2))

      # return from inside an iterator
      assert_equal 8, number_first_square_over(50)
      assert_equal 1, number_first_square_over(0)
      assert_equal -1, number_first_square_over(99_999)
      assert_equal 0, triangle(0)
      assert_equal 1, triangle(1)
      assert_equal 5050, triangle(100)

      # nesting
      total = 0
      1.upto(3) do |i|
        i.downto(1) do |j|
          next if j == 2

          total += i * j
        end
      end
      assert_equal 15, total
      grid = [] #: Array[String]
      2.times { |r| 3.times { |c| grid << "#{r}#{c}" } }
      assert_equal ["00", "01", "02", "10", "11", "12"], grid
      acc = 0.0
      4.times { |i| acc += i.to_f / 2.0 }
      assert_equal "3.0", (acc).inspect

      # yield from inside an iterator, forwarded blocks, reopened Integer
      evens = [] #: Array[Integer]
      number_each_even(7) { |i| evens << i }
      assert_equal [0, 2, 4, 6], evens
      reps = [] #: Array[String]
      number_rep(2) { |i| reps << "rep #{i}" }
      assert_equal ["rep 0", "rep 1"], reps
      cells = [] #: Array[String]
      grid_each(2, 2) { |x, y| cells << "#{x},#{y}" }
      assert_equal ["0,0", "1,0", "0,1", "1,1"], cells
      digits = [] #: Array[Integer]
      1203.each_digit { |d| digits << d }
      assert_equal [1, 2, 0, 3], digits

      # iterators inside value blocks, with computed bounds
      r = [1, 2, 3].map do |k|
        s = 0
        10.times do |i|
          break if i > k
          next if i.zero?

          s += i
        end
        s
      end
      assert_equal [1, 3, 6], r
      n = 3
      down = [] #: Array[Integer]
      n.downto(n - 5) { |i| down << i }
      assert_equal [3, 2, 1, 0, -1, -2], down
      up = [] #: Array[Integer]
      2.upto(3.succ) { |i| up << i }
      assert_equal [2, 3, 4], up
    end
  end

  class NumberMidTest < Minitest::Test
    # NaN in between?/clamp/sort/max raises like MRI instead of comparing
    def test_nan_in_between_clamp_sort
      nz = 0.0 #: Float
      nan = nz / nz #: Float
      assert_raises(ArgumentError) { nan.between?(0.0, 1.0) }
      assert_raises(ArgumentError) { nan.clamp(0.0, 1.0) }
      # the failed comparison names (earlier, later) in a sort and (best so far, candidate) in min/max, as MRI's do
      e = assert_raises(ArgumentError) { [1.0, nan].sort }
      assert_equal "comparison of Float with NaN failed", e.message
      e = assert_raises(ArgumentError) { [nan, 2.0, 1.0].sort }
      assert_equal "comparison of Float with 2.0 failed", e.message
      # min/max name (best so far, candidate) as MRI's Float fast path does; with Float reopened (this file) MRI's
      # generic path names them the other way round, so the message is not compared here.
      err = begin
        [nan, 1.0].max
        nil
      rescue ArgumentError => e
        e
      end
      assert_equal false, err.nil?
    end

    # != follows ==, across Integer/Float and a user-defined ==
    def test_ne_follows_eq
      x = 1 #: Integer
      y = 1.0 #: Float
      assert_equal false, (x != 1.0)
      assert_equal false, (y != 1)
      assert_equal false, (x != y)
      assert_equal false, (y != x)
      assert_equal true, (Money.new(5) == Money.new(5))
      assert_equal false, (Money.new(5) != Money.new(5))
    end

    # a bool? holding false is as falsy as nil
    def test_optional_bool_false_is_falsy
      ob_c = number_maybe(true)
      assert_equal "fallback", (ob_c || "fallback")
      assert_equal "falsy", (ob_c ? "truthy" : "falsy")
      ob_s = nil #: String?
      ob_s = "not printed" if ob_c
      assert_nil ob_s
      ob_d = number_maybe(true)
      ob_d ||= true
      assert_equal true, ob_d
      ob_e = number_maybe(true)
      assert_equal false, (ob_e && "rhs")
      ob_h = { "off" => false } #: Hash[String, bool]
      assert_equal true, (ob_h["off"] || true)
      assert_equal "off", (ob_h["off"] ? "on" : "off")
      assert_equal "missing", (ob_h["missing"] ? "on" : "missing")
    end

    # += on a user class calls its own +
    def test_op_assign_calls_user_plus
      num_c = Num.new(1) #: Num
      num_c += Num.new(2)
      num_c += Num.new(3)
      assert_equal 6, num_c.v
      num_d = Num.new(10)
      num_d = num_d + Num.new(5)
      assert_equal 15, num_d.v
    end

    # results at Integer's 64-bit edges must not trip the overflow checks (decision 35)
    def test_results_at_integer_s_64
      m = 0x7fff_ffff_ffff_ffff #: Integer
      n = -9_223_372_036_854_775_808 #: Integer
      t = -2 #: Integer
      o = -1 #: Integer
      assert_equal 9223372036854775807, (m + 0)
      assert_equal -9223372036854775808, (n + 0)
      assert_equal 9223372036854775807, (m - 0)
      assert_equal -9223372036854775808, (n + 1) - 1
      assert_equal -1, (m + n)
      assert_equal -1, (n - -m)
      assert_equal -9223372036854775807, (0 - m)
      assert_equal 9223372030926249001, (3_037_000_499 * 3_037_000_499)
      assert_equal -9223372030926249001, (-3_037_000_499 * 3_037_000_499)
      assert_equal -9223372036854775808, (-4_611_686_018_427_387_904 * 2)
      assert_equal 9223372036854775806, (4_611_686_018_427_387_903 * 2)
      assert_equal -9223372036854775808, (n * 1)
      assert_equal -9223372036854775807, (m * -1)
      assert_equal -9223372036854775807, (-1 * m)
      assert_equal 4611686016279904256, (2_147_483_648 * 2_147_483_647)
      assert_equal 4611686018427387904, (-2_147_483_648 * -2_147_483_648)
      assert_equal 4611686018427387904, (1_099_511_627_776 * 4_194_304)
      assert_equal -9223372036854775808, (-1_099_511_627_776 * 8_388_608)
      assert_equal 4611686018427387904, (2**62)
      assert_equal -9223372036854775808, (t**63)
      assert_equal 4052555153018976267, (3**39)
      assert_equal 1000000000000000000, (10**18)
      assert_equal -27, (t - 1)**3
      assert_equal 1, (7**0)
      assert_equal 1, (1**1_000_000_000_000)
      assert_equal -1, (o**1_000_000_000_001)
      assert_equal 1, (o**-4)
      assert_equal -1, (o**-3)
      assert_equal 1, (1**-5)
      assert_equal 9223372036854775807, (-m).abs
      assert_equal 9223372036854775807, m.pred.succ
      assert_equal -9223372036854775808, (n / 1)
      assert_equal -4611686018427387904, (n / 2)
      assert_equal 0, (n % -1)
      assert_equal 9223372036854775807, (-m / -1)
      assert_equal 9200000000000000000, 9.2e18.to_i
      assert_equal -9200000000000000000, -9.2e18.floor
      assert_equal -9223372036854775808, "-9223372036854775808".to_i
      assert_equal 9223372036854775807, "9223372036854775807".to_i
    end

    # Integer and Float library methods
    def test_integer_and_float_methods
      assert_equal 2, 10.gcd(4)
      assert_equal 6, -12.gcd(18)
      assert_equal 12, 4.lcm(6)
      assert_equal 0, 0.lcm(5)
      assert_equal 6, (10.pow(3, 7))
      assert_equal 81, 3.pow(4)
      assert_equal -5, (2.pow(10, -7))
      assert_equal [4, 3, 2, 1], 1234.digits
      assert_equal [15, 15], 255.digits(16)
      assert_equal [0], 0.digits
      assert_equal "3.5", (7.fdiv(2)).to_s
      assert_equal "14.0", (7.fdiv(0.5)).to_s
      assert_equal [3, 1], 7.divmod(2)
      assert_equal [-4, 1], -7.divmod(2)
      assert_equal 3, 7.div(2)
      assert_equal -1, -7.remainder(2)
      assert_equal 1, -7.modulo(2)
      assert_equal 8, 255.bit_length
      assert_equal 0, -1.bit_length
      assert_equal 0, 0.bit_length
      assert_equal "11111111", 255.to_s(2)
      assert_equal "ff", 255.to_s(16)
      assert_equal "-73", -255.to_s(36)
      assert_equal 4, Integer.sqrt(17)
      assert_equal 1000000, Integer.sqrt(10**12)
      assert_equal true, 3.integer?
      assert_equal true, 3.finite?
      assert_equal [1, 4, 7, 10], (1.step(10, 3).to_a)
      assert_equal [10, 6, 2], (10.step(1, -4).to_a)
      assert_equal [0, 2, 4], (3.times.map { it * 2 })
      assert_equal [1, 2, 3], 1.upto(3).to_a
      assert_equal [3, 2, 1], (3.downto(1).map { |i| i })
      steps = [] #: Array[Integer]
      1.step(7, 2) { |i| steps << i }
      assert_equal [1, 3, 5, 7], steps
      assert_equal "3.14", (3.14159.floor(2)).to_s
      assert_equal "3.15", (3.14159.ceil(2)).to_s
      # TestFuzzDiff: 15+ digits go through an exact Rational, tiny numbers round to a positive 0.0, non-finite floor/ceil(15+) raise
      assert_equal "-4.959293e-09", (-4.959292644760041e-09).floor(15).to_s
      assert_equal "1.0e-17", 5.0e-324.ceil(17).to_s
      assert_equal "0.0", (-0.000339).round(1).to_s
      assert_equal "0.0", (-1.0e-300).ceil(3).to_s
      assert_equal "-0.001", (-1.0e-300).floor(3).to_s
      assert_equal "Infinity", Float::INFINITY.round(17).to_s
      cdigits = 6
      assert_equal "0.000254", 0.000253.ceil(cdigits).to_s # ceil does not step back, floor does: 253.00000000000003 rounds up
      assert_equal "0.0522000001", 0.0522.ceil(10).to_s
      assert_equal "-9.334282670881224", (-9.334282670881223).round(15).to_s # Rational#to_f divides two doubles, not one exact quotient
      fdigits = 17
      assert_equal FloatDomainError, begin; Float::INFINITY.floor(fdigits); rescue FloatDomainError => e; e.class; end
      assert_equal "-3.2", (-3.14159.floor(1)).to_s
      assert_equal 2, 2.5.truncate
      assert_equal -2, -2.5.truncate
      assert_equal "0.7000000000000002", ((3.7 % 1)).to_s
      assert_equal "0.2999999999999998", ((-3.7 % 1)).to_s
      assert_equal "-0.5", ((7.5 % -2.0)).to_s
      assert_equal "[3, 1.5]", (7.5.divmod(2.0)).to_s
      assert_equal "3.5", (7.0.fdiv(2.0)).to_s
      assert_equal true, 1.0.finite?
      assert_equal 1, (1.0 / 0).infinite?
      assert_equal -1, (-1.0 / 0).infinite?
      assert_nil 1.0.infinite?
      assert_equal false, 1.5.integer?
      assert_equal "0.1", (0.1.floor(1)).to_s
      assert_equal "1.1", (1.1.ceil(1)).to_s
      assert_equal "12.34", (12.34.floor(5)).to_s
    end
  end

  class NumberOperatorsTest < Minitest::Test
    def test_user_operators
      a = Num.new(7)
      b = Num.new(-2)
      assert_equal "Num(5)", (a + b).to_s
      assert_equal "Num(9)", (a - b).to_s
      assert_equal "Num(-14)", (a * b).to_s
      assert_equal "Num(-4)", (a / b).to_s
      assert_equal "Num(-1)", (a % b).to_s
      assert_equal "Num(49)", (a**2).to_s
      assert_equal "Num(-7)", (-a).to_s
      assert_equal "Num(2)", (+b).to_s
      assert_equal "Num(-8)", (~a).to_s
      assert_equal "Num(56)", (a << 3).to_s
      assert_equal "Num(3)", (a >> 1).to_s
      assert_equal "Num(-2)", (a & b).to_s
      assert_equal "Num(7)", (a | b).to_s
      assert_equal "Num(9)", (a ^ b).to_s
      assert_equal true, (a == Num.new(7))
      assert_equal false, (a == b)
      assert_equal false, (a == 7)
      assert_equal 1, (a <=> b)
      assert_equal -1, (b <=> a)
      assert_equal 0, (a <=> Num.new(7))
      assert_equal false, (a < b)
      assert_equal true, (a <= a)
      assert_equal true, (a > b)
      assert_equal false, (b >= a)
      assert_equal true, (a === 7)
      assert_equal false, (a === 8)
      assert_equal true, (a =~ 21)
      assert_equal false, (a =~ 22)
      assert_equal false, (a !~ 21)
      assert_equal true, (a !~ 22)
      assert_equal 21, a[3]
      assert_equal 7003, a.index(3)
      assert_equal true, a.match(7)
      assert_equal false, a.match(8)
      assert_equal 5, (a[1] = 5)
      assert_equal 13, a.last_set
      assert_equal false, (!a)
      assert_equal true, (!Num.new(0))
      assert_equal false, (a != Num.new(7))
      assert_equal true, (a != b)

      # names that are not operators
      c = Num.new(12)
      assert_equal true, c.big?
      assert_equal 12, c.__raw
      assert_equal 6, c._half
      c.v = 3
      assert_equal 3, c.v
      assert_equal false, c.big?
      assert_equal 0, c.reset!.v

      # operators called by name
      assert_equal "Num(5)", a.+(b).to_s
      assert_equal 3, 1.+(2)
      assert_equal 5, (7.send(:-, 2))
      assert_equal "5.0", ((2.5.public_send(:*, 2.0))).inspect
      assert_equal -1, (3.send(:<=>, 4))
      assert_equal 2, (6.send(:%, 4))
      assert_equal 32, (2.send(:**, 5))
      assert_equal true, (1.send(:==, 1.0))
      assert_equal false, (a.send(:<, b))
      assert_equal [-1, 2, -3], ([1, -2, 3].map(&:-@))
      assert_equal "[1.5, 2.5]", (([1.5, -2.5].map(&:abs))).inspect
      assert_equal ["3", "4"], ([3, 4].map(&:to_s))
    end
  end

  class NumberReopenTest < Minitest::Test
    def test_reopened_integer_and_float
      assert_equal 8, 4.double
      assert_equal -6, -3.double
      assert_equal true, 12.divisible_by?(4)
      assert_equal false, 12.divisible_by?(5)
      assert_equal 1, 0.factorial
      assert_equal 120, 5.factorial
      assert_equal 2432902008176640000, 20.factorial
      assert_equal ["1st", "2nd", "3rd", "4th", "11th", "12th", "13th", "21st", "22nd", "101st", "111th", "0th"], ([1, 2, 3, 4, 11, 12, 13, 21, 22, 101, 111, 0].map { |i| i.ordinal })
      assert_equal true, 4.5.above_zero?
      assert_equal false, -0.5.above_zero?
      assert_equal "1.5", (3.0.half).inspect
    end
  end

  class NumberStateTest < Minitest::Test
    def test_numbers_in_state
      # constants
      assert_equal 10, LIMIT
      assert_equal 20, (LIMIT * 2)
      assert_equal "0.25", (RATE).inspect
      assert_equal "1.0", ((RATE * 4.0)).inspect
      assert_equal false, DEBUG
      assert_equal true, (!DEBUG)

      # ivars
      acct = Account.new
      assert_equal "10.5", (acct.deposit(10.5)).inspect
      assert_equal "10.75", (acct.deposit(RATE)).inspect
      assert_equal 2, acct.count
      assert_equal "10.75", (acct.balance).inspect
      acct.count = 42
      assert_equal 42, acct.count
      assert_equal true, acct.open?
      acct.toggle
      assert_equal false, acct.open?
      acct.toggle
      assert_equal true, acct.open?
      assert_equal "2.0", (Account.new(2.0).balance).inspect

      # default arguments
      assert_equal "3 1.5 true", defaults
      assert_equal "4 1.5 true", defaults(4)
      assert_equal "5 0.5 true", (defaults(5, 0.5))
      assert_equal "6 -1.0 false", (defaults(6, -1.0, false))

      # multiple assignment
      a, b = 1, 2.5
      assert_equal 1, a
      assert_equal "2.5", (b).inspect
      i, j = 3, 4
      i, j = j, i
      assert_equal 4, i
      assert_equal 3, j
      lo, hi = [9, 2].sort
      assert_equal "2 9", ("#{lo} #{hi}")
      p1, p2, p3 = 1, true, 0.5
      assert_equal 1, p1
      assert_equal true, p2
      assert_equal "0.5", (p3).inspect

      # respond_to?, send, class
      assert_equal true, 1.respond_to?(:+)
      assert_equal false, 1.respond_to?(:upcase)
      assert_equal true, 1.5.respond_to?(:floor)
      assert_equal true, true.respond_to?(:&)
      assert_equal false, 1.5.respond_to?(:even?)
      assert_equal 3, (1.send(:+, 2))
      assert_equal 3, 3.public_send(:abs)
      assert_equal 2, 2.5.send(:floor)
      assert_equal 4, -4.send(:abs)
      assert_equal false, (true.send(:^, true))
      assert_equal "Integer", (1.class).inspect
      assert_equal "Float", (2.5.class).inspect
      assert_equal "Float", 2.5.class.name
      assert_equal "Integer", 1.class.name
      assert_equal "Integer", Integer.name
      assert_equal "Float", (Float).inspect
    end
  end

  class NumberUntypedTest < Minitest::Test
    def test_numbers_through_untyped
      # literals box to their Ruby class under untyped
      vals = [1, -2.5, true, false, nil, "s", 0, 0.0] #: Array[untyped]
      assert_equal ["Integer 1", "Float -2.5", "other true", "other false", "nil", "String \"s\"", "Integer 0", "Float 0.0"], vals.map { |v| number_kind(v) }
      assert_equal "[1, -2.5, true, false, nil, \"s\", 0, 0.0]", (vals).inspect
      assert_equal "Integer 3", number_kind(3)
      assert_equal "Float -0.0", number_kind(-0.0)
      assert_equal "other true", (number_kind(1 == 1))
      assert_equal "other false", (number_kind(1 > 2))
      assert_equal "Integer 3", (number_kind(7 / 2))
      assert_equal "Float 3.5", (number_kind(7.0 / 2.0))

      # only nil and false are falsy; 0 and 0.0 are truthy
      assert_equal ["truthy", "truthy", "truthy", "falsy", "falsy", "truthy", "truthy", "truthy"], vals.map { |v| truthy(v) }

      # values round-trip through untyped
      i = number_ident(42) #: Integer
      fl = number_ident(-1.5) #: Float
      bo = number_ident(false) #: bool
      assert_equal 43, (i + 1)
      assert_equal "-3.0", ((fl * 2.0)).inspect
      assert_equal true, (!bo)

      # == across the boundary
      one = 1.0 #: untyped
      assert_equal true, (1 == one)
      assert_equal true, (one == 1)
      assert_equal false, (2 == one)
      assert_equal true, (1.0 == number_ident(1))
      assert_equal true, (true == number_ident(true))
      assert_equal false, (false == number_ident(nil))

      # is_a? and class
      assert_equal true, 1.is_a?(Integer)
      assert_equal true, 1.is_a?(Comparable)
      assert_equal false, 1.is_a?(Float)
      assert_equal true, 1.kind_of?(Object)
      assert_equal true, 1.5.is_a?(Float)
      assert_equal true, 1.5.is_a?(Comparable)
      assert_equal false, 1.5.is_a?(Integer)
      assert_equal true, number_ident(1).is_a?(Integer)
      assert_equal false, number_ident(1.5).is_a?(Integer)
      assert_equal true, number_ident(1.5).is_a?(Float)
      assert_equal "Integer", (1.class).inspect
      assert_equal "Float", (1.5.class).inspect
      assert_equal "Integer", 1.class.name

      # dynamic calls on untyped numbers (values from ident are real Go `any`s; <=> is in number_bug_untyped_cmp)
      v = number_ident(5)
      w = number_ident(2.5)
      assert_equal "6", ((v + 1)).inspect
      assert_equal "-3", ((v - 8)).inspect
      assert_equal "15", ((v * 3)).inspect
      assert_equal "2", ((v / 2)).inspect
      assert_equal "2", ((v % 3)).inspect
      assert_equal "25", ((v**2)).inspect
      assert_equal "-5", ((-v)).inspect
      assert_equal "5.0", ((w * 2.0)).inspect
      assert_equal "2.0", ((w - 0.5)).inspect
      assert_equal "1.25", ((w / 2.0)).inspect
      assert_equal "-2.5", ((-w)).inspect
      assert_equal "5.0", ((w + w)).inspect
      assert_equal "25", ((v * v)).inspect
      assert_equal true, (v < 10)
      assert_equal true, (v >= 5)
      assert_equal false, (w > 3.0)
      assert_equal false, (v < v)
      assert_equal true, (w <= w)
      assert_equal false, v.even?
      assert_equal true, v.odd?
      assert_equal false, v.zero?
      assert_equal true, v.positive?
      assert_equal false, v.negative?
      assert_equal "6", (v.succ).inspect
      assert_equal "4", (v.pred).inspect
      assert_equal [5], v.chr.bytes # MRI's chr is US-ASCII and inspects as "\x05"; rugaroo's strings are all UTF-8 ("\u0005")
      assert_equal "5", (v.abs).inspect
      assert_equal "2", (w.floor).inspect
      assert_equal "3", (w.ceil).inspect
      assert_equal "2", (w.to_i).inspect
      assert_equal "5.0", (v.to_f).inspect
      assert_equal "2.5", (w.abs).inspect
      assert_equal false, w.nan?
      assert_equal false, w.zero?
      assert_equal "5", v.to_s
      assert_equal "2.5", w.to_s
      assert_equal "5", (v).inspect
      assert_equal "2.5", (w).inspect
      assert_equal "5|2.5|-0.0|1.0e+20", "#{v}|#{w}|#{number_ident(-0.0)}|#{number_ident(1e20)}"
      assert_equal true, (v == 5)
      assert_equal true, (w == 2.5)
      assert_equal false, (v == w)
      assert_equal true, (v == number_ident(5.0))
      assert_equal false, (number_ident(5.0) != v)
      total = 0
      [number_ident(1), number_ident(2), number_ident(3)].each { |x| total += x }
      assert_equal "6", (total).inspect
      h = { "n" => 1, "f" => 2.5, "b" => true } #: Hash[String, untyped]
      assert_equal "{\"n\" => 1, \"f\" => 2.5, \"b\" => true}", (h).inspect
      assert_equal "5.0", ((h["f"] * 2.0)).inspect
      assert_equal "[1.0e+20, -0.0, 100.0, 1.0e-05, 7, -3, true, nil]", (([1e20, -0.0, 100.0, 1e-5, 7, -3, true, nil])).inspect
    end
  end

  # Kernel#Integer / Kernel#Float: strict, whole-string conversion (issue #3).
  class NumberStrictConversionTest < Minitest::Test
    def test_integer_strings
      got = ["42", " 42\n", "-42", "+7", "1_000", "0x1f", "0X1F", "-0x1f", "0b101", "0o17", "017", "0_7", "0d19", "00"].map { |s| Integer(s) }
      assert_equal [42, 42, -42, 7, 1000, 31, 31, -31, 5, 15, 15, 7, 19, 0], got
    end

    def test_integer_bad_strings
      got = ["1__0", "_1", "1_", "08", "abc", "", " ", "4 2", "42abc", "0x", "- 42", "1e3", "12.5", "+-1"].map do |s|
        assert_raises(ArgumentError) { Integer(s) }.message
      end
      assert_equal ["invalid value for Integer(): \"1__0\"", "invalid value for Integer(): \"_1\"",
                    "invalid value for Integer(): \"1_\"", "invalid value for Integer(): \"08\"",
                    "invalid value for Integer(): \"abc\"", "invalid value for Integer(): \"\"",
                    "invalid value for Integer(): \" \"", "invalid value for Integer(): \"4 2\"",
                    "invalid value for Integer(): \"42abc\"", "invalid value for Integer(): \"0x\"",
                    "invalid value for Integer(): \"- 42\"", "invalid value for Integer(): \"1e3\"",
                    "invalid value for Integer(): \"12.5\"", "invalid value for Integer(): \"+-1\""], got
    end

    def test_integer_base
      assert_equal 255, Integer("ff", 16)
      assert_equal 255, Integer("0xff", 16)
      assert_equal 35, Integer("z", 36)
      assert_equal 2833, Integer("0b11", 16)
      assert_equal(-255, Integer(" -ff ", 16))
      assert_equal 8, Integer("010", 0)
      assert_equal "invalid value for Integer(): \"12\"", assert_raises(ArgumentError) { Integer("12", 2) }.message
      assert_equal "invalid value for Integer(): \"0xff\"", assert_raises(ArgumentError) { Integer("0xff", 10) }.message
      assert_equal "invalid radix 37", assert_raises(ArgumentError) { Integer("1", 37) }.message
    end

    def test_integer_numbers
      assert_equal 3, Integer(3.9)
      assert_equal(-3, Integer(-3.9))
      assert_equal 5, Integer(5)
      assert_equal "NaN", assert_raises(FloatDomainError) { Integer(Float::NAN) }.message
      assert_equal "-Infinity", assert_raises(FloatDomainError) { Integer(-Float::INFINITY) }.message
    end

    def test_nil_and_untyped
      h = { a: "12", f: "2.5" } #: Hash[Symbol, String]
      assert_equal 12, Integer(h[:a])
      assert_equal 2.5, Float(h[:f])
      assert_equal "can't convert nil into Integer", assert_raises(TypeError) { Integer(h[:b]) }.message
      assert_equal "can't convert nil into Float", assert_raises(TypeError) { Float(h[:b]) }.message
      v = number_ident("0x10") #: untyped
      assert_equal 16, Integer(v)
      assert_equal 16, Integer(v, 16)
      assert_equal 3, Integer(number_ident(3.5))
      assert_equal "base specified for non string value", assert_raises(ArgumentError) { Integer(number_ident(5), 2) }.message
    end

    def test_float_strings
      got = ["3.5", " 3.5 ", "1e5", "1E-2", "-1.5", "+1.5", "1_000.5", ".5", "5.", "1.e5", "0x1A", "0x1.8p1", "3", "1e5_0", "017", "1e+5", "0.1_2"].map { |s| Float(s) }
      assert_equal [3.5, 3.5, 100000.0, 0.01, -1.5, 1.5, 1000.5, 0.5, 5.0, 100000.0, 26.0, 3.0, 3.0, 1.0e+50, 17.0, 100000.0, 0.12], got
    end

    def test_float_bad_strings
      got = ["1__0.5", "Infinity", "NaN", "abc", "", "1.5abc", "1_e5", "0b11", "1e", "_1.5", "1.5_", "1._5", "0x", "- 1.5", "1e_5", "+-1"].map do |s|
        assert_raises(ArgumentError) { Float(s) }.message
      end
      assert_equal ["invalid value for Float(): \"1__0.5\"", "invalid value for Float(): \"Infinity\"",
                    "invalid value for Float(): \"NaN\"", "invalid value for Float(): \"abc\"",
                    "invalid value for Float(): \"\"", "invalid value for Float(): \"1.5abc\"",
                    "invalid value for Float(): \"1_e5\"", "invalid value for Float(): \"0b11\"",
                    "invalid value for Float(): \"1e\"", "invalid value for Float(): \"_1.5\"",
                    "invalid value for Float(): \"1.5_\"", "invalid value for Float(): \"1._5\"",
                    "invalid value for Float(): \"0x\"", "invalid value for Float(): \"- 1.5\"",
                    "invalid value for Float(): \"1e_5\"", "invalid value for Float(): \"+-1\""], got
    end

    def test_float_numbers
      assert_equal 3.0, Float(3)
      assert_equal 2.5, Float(2.5)
    end
  end

  # ruby/spec core/integer and core/float gaps (#49)
  class NumberRubySpecTest < Minitest::Test
    def test_integer_bits
      assert_equal [1, 7, 6, -6], [5 & 3, 5 | 3, 5 ^ 3, ~5]
      assert_equal [16, -4, 4, 0], [1 << 4, -16 >> 2, 1 >> -2, 1 >> 70]
      assert_equal [1, 0, 1, 0], [5[0], 5[1], -1[100], 5[-1]]
      assert_equal [true, true, true], [6.allbits?(2), 6.anybits?(3), 6.nobits?(1)]
    end

    def test_integer_rounding
      assert_equal [1200, 1300, -1300, 1200, -1300], [1234.round(-2), 1250.round(-2), -1250.round(-2), 1234.floor(-2), -1234.floor(-2)]
      assert_equal [1300, -1200, 7, 7, 7, 0], [1234.ceil(-2), -1234.truncate(-2), 7.round, 7.floor(1), 7.ceil(2), 7.truncate(-20)]
    end

    def test_integer_misc
      assert_equal [4, -3, 4, 3, 3], [7.ceildiv(2), -7.ceildiv(2), 3.next, -3.magnitude, 3.to_int]
      assert_equal [2, 12], 4.gcdlcm(6)
      assert_nil 0.nonzero?
      assert_equal [3, 9, 3, 0, true], [3.nonzero?, 3.abs2, 3.conj, 3.imag, 3.real?]
      assert_equal [3, 0], 3.rect
      assert_equal [true, false, 8], [1.eql?(1), 1.eql?(1.0), 3.size]
    end

    def test_float_misc
      assert_equal [1.0000000000000002, -5.0e-324], [1.0.next_float, 0.0.prev_float]
      assert_equal [true, true, false], [1.5.positive?, -1.5.negative?, -0.0.negative?]
      assert_equal [2.5, 2, true, false], [-2.5.magnitude, 2.5.to_int, 1.5.eql?(1.5), 1.0.eql?(1)]
      assert_nil 0.0.nonzero?
      assert_equal [1.5, 2.25, 1.5, 0, true], [1.5.nonzero?, 1.5.abs2, 1.5.conj, 1.5.imag, 1.5.real?]
      assert_equal [1.5, 0], 1.5.rect
    end
  end

  # ruby/spec core/math and core/range gaps (#49); last digits are rounded where Go's math and MRI's libm differ (decision 43)
  class NumberRubySpecMathTest < Minitest::Test
    def test_math
      assert_equal [0.88137358701954, 1.76274717403909, 0.54930614433405], [Math.asinh(1.0).round(14), Math.acosh(3.0).round(14), Math.atanh(0.5).round(14)]
      assert_equal [0.6931471805599453, 1.71828182845905, 9.999999999995e-13, 1.0000000000005e-12], [Math.log1p(1.0), Math.expm1(1.0).round(14), Math.log1p(1.0e-12), Math.expm1(1.0e-12)]
      assert_equal [-Float::INFINITY, Float::INFINITY, 0.0], [Math.log1p(-1.0), Math.atanh(1.0), Math.acosh(1.0)]
      assert_equal [[0.5, 4], [-0.6, -1], [0.0, 0]], [Math.frexp(8.0), Math.frexp(-0.3), Math.frexp(0.0)]
      assert_equal [8.0, 0.75], [Math.ldexp(0.5, 4), Math.ldexp(3.0, -2)]
      assert_raises(Math::DomainError) { Math.acosh(0.5) }
      assert_raises(Math::DomainError) { Math.atanh(2.0) }
      assert_raises(Math::DomainError) { Math.log1p(-2.0) }
    end

    def test_range
      assert_equal [true, false, false, true, false, false], [(1..3).overlap?(3..4), (1...3).overlap?(3..4), (3..1).overlap?(1..3), (1..).overlap?(5..6), (1...1).overlap?(1..1), (1..3).overlap?(4..)]
      assert_equal [3, nil, 100, nil], [(1..5).bsearch { |x| x >= 3 }, (1..5).bsearch { |x| x > 9 }, (0..).bsearch { |x| x >= 100 }, (1...3).bsearch { |x| x >= 3 }]
    end
  end

  # ruby/spec language gaps (#49): beginless ranges
  class NumberRubySpecBeginlessTest < Minitest::Test
    #: (Integer) -> String
    def number_grade(n)
      case n
      when ..59 then "F"
      when 60..79 then "C"
      else "A"
      end
    end

    def test_beginless_ranges
      a = [10, 20, 30, 40]
      assert_equal [[10, 20], [10, 20], [10, 20, 30], "hel"], [a[..1], a[...2], a[..-2], "hello"[..2]]
      r = ..5
      assert_equal ["..5", "...5", true, false, true, 5, 5], [r.inspect, (...5).inspect, r.cover?(3), r.cover?(6), r.include?(-100), r.end, r.max]
      assert_equal %w[F C A], [number_grade(10), number_grade(65), number_grade(99)]
      assert_equal [true, false, true, false], [(..3).overlap?(2..4), (..1).overlap?(2..4), (..5) == (..5), (..5) == (...5)]
      e = assert_raises(TypeError) { (..3).each { |x| x } }
      assert_equal "can't iterate from NilClass", e.message
    end
  end

  # ruby/spec core/float gaps (#49): step
  class NumberRubySpecFloatStepTest < Minitest::Test
    def test_float_step
      a = [] #: Array[Float]
      1.0.step(2.0, 0.1) { |x| a << x }
      assert_equal [1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9000000000000001, 2.0], a
      b = [] #: Array[Float]
      1.0.step(0.0, -0.25) { |x| b << x }
      c = [] #: Array[Float]
      1.0.step(2.0) { |x| c << x }
      d = [] #: Array[Float]
      2.0.step(1.0, 0.5) { |x| d << x }
      assert_equal [[1.0, 0.75, 0.5, 0.25, 0.0], [1.0, 2.0], []], [b, c, d]
    end
  end

  # ruby/spec core/math gaps (#49): erf, erfc, gamma, lgamma; last digits are rounded where Go and MRI differ (decision 43)
  class NumberRubySpecMathSpecialTest < Minitest::Test
    def test_erf
      assert_equal [0.5204998778130465, 0.9661051464753108, 0.999593047982555, -0.5204998778130465, 1.0, -1.0, 0.0],
                   [Math.erf(0.5), Math.erf(1.5), Math.erf(2.5), Math.erf(-0.5), Math.erf(10.0), Math.erf(-7.0), Math.erf(0.0)]
      assert_equal [0.4795001221869535, 0.033894853524689274, 1.5204998778130465, 1.9661051464753108, 0.0, 2.0],
                   [Math.erfc(0.5), Math.erfc(1.5), Math.erfc(-0.5), Math.erfc(-1.5), Math.erfc(30.0), Math.erfc(-10.0)]
    end

    def test_gamma
      assert_equal [1.77245385090552, 0.88622692545276, 1.32934038817914, 2.36327180120736, 24.0, 362880.0, 1.21645100408832e+17],
                   [Math.gamma(0.5).round(14), Math.gamma(1.5).round(14), Math.gamma(2.5).round(14), Math.gamma(-1.5).round(14), Math.gamma(5.0), Math.gamma(10.0), Math.gamma(20.0)]
      assert_equal [Float::INFINITY, -Float::INFINITY, Float::INFINITY], [Math.gamma(0.0), Math.gamma(-0.0), Math.gamma(172.0)]
      assert_raises(Math::DomainError) { Math.gamma(-1.0) }
    end

    def test_lgamma
      a, b, c, d, e, f = Math.lgamma(2.5), Math.lgamma(-0.5), Math.lgamma(-1.5), Math.lgamma(20.0), Math.lgamma(1.0), Math.lgamma(2.0)
      assert_equal [[0.28468287047292, 1], [1.26551212348465, -1], [0.86004701537648, 1], [39.3398841871995, 1], [0.0, 1], [0.0, 1]],
                   [[a[0].round(14), a[1]], [b[0].round(14), b[1]], [c[0].round(14), c[1]], [d[0].round(14), d[1]], [e[0].round(14), e[1]], [f[0].round(14), f[1]]]
      assert_equal [[Float::INFINITY, 1], [Float::INFINITY, -1], [Float::INFINITY, 1]], [Math.lgamma(0.0), Math.lgamma(-0.0), Math.lgamma(-1.0)]
      assert_raises(Math::DomainError) { Math.lgamma(-Float::INFINITY) }
    end
  end

  # ruby/spec core/numeric gaps (#49): polar
  class NumberRubySpecPolarTest < Minitest::Test
    def test_polar
      assert_equal [[3, 0], [3, Math::PI], [2.5, 0], [2.5, Math::PI], [0, 0]], [3.polar, -3.polar, 2.5.polar, -2.5.polar, 0.polar]
    end
  end

  # Numeric (#39, decision 142)
  class NumberNumericTest < Minitest::Test
    def test_numeric_parameter
      assert_equal [4, 5.0, Rational(2, 3), Complex(2, 4), BigDecimal("3")],
                   [number_double(2), number_double(2.5), number_double(Rational(1, 3)), number_double(Complex(1, 2)), number_double(BigDecimal("1.5"))]
    end

    def test_numeric_case_is_a_and_narrowing
      vs = [1, 1.5, Rational(1, 2), Complex(0, 0), BigDecimal("1"), "1", nil, :x]
      assert_equal ["int 2", "float 1", "number false", "number true", "number false", "other", "other", "other"], vs.map { |v| number_numeric_kind(v) }
      assert_equal [true, true, true, true, true, false, false, false], vs.map { |v| v.is_a?(Numeric) }
      assert_equal ["number 2", "number 1.5", "other"], [number_only_numeric(-2), number_only_numeric(1.5), number_only_numeric("x")]
      assert_equal [true, false, true, true, true], [Numeric === 2, Numeric === "2", 1.kind_of?(Numeric), Complex(1, 2).is_a?(Comparable), BigDecimal("1").is_a?(Comparable)]
      assert_equal ["even true", "nan false", "other false"], [number_narrow(2), number_narrow(1.5), number_narrow(Rational(1, 2))]
      assert_equal [2, 2.5, Rational(3, 2), nil], [number_succ(1), number_succ(1.5), number_succ(Rational(1, 2)), number_succ("x")]
      assert_equal ["int 2", "number false", "other"], [number_generic_kind(1), number_generic_kind(2.5), number_generic_kind("a")]
    end

    def test_numeric_mixed_typed
      i = 3
      f = 0.75
      r = Rational(1, 2)
      b = BigDecimal("2.5")
      c = Complex(1, 2)
      assert_equal [true, true, false, -1, 1, true], [r < f, f > r, i < r, r <=> f, b <=> r, r < b]
      assert_equal [Rational(7, 2), 1.25, BigDecimal("3"), Complex(Rational(3, 2), 2), BigDecimal("5.5")], [i + r, r + f, r + b, r + c, i + b]
      assert_equal [6, 3, [6, Rational(0, 1)], 6.0, BigDecimal("1.2")], [i.div(r), f.div(Rational(1, 4)), i.divmod(r), i.fdiv(r), i / b]
      assert_equal [0.25, Rational(1, 2), 0.5, BigDecimal("0.5")], [f % r, Rational(7, 2) % i, r.remainder(f), i.modulo(b)]
      assert_equal [nil, 0, 1, nil], [c <=> i, Complex(3, 0) <=> 3, Complex(3.0, 0) <=> r, i <=> c]
    end

    def test_numeric_mixed_through_numeric
      i = 3 #: Numeric
      f = 0.75 #: Numeric
      r = Rational(1, 2) #: Numeric
      b = BigDecimal("2.5") #: Numeric
      c = Complex(1, 2) #: Numeric
      assert_equal [Rational(7, 2), 1.25, BigDecimal("3"), Complex(Rational(3, 2), 2), BigDecimal("5.5"), Complex(4, 2)], [i + r, r + f, r + b, r + c, i + b, c + i]
      assert_equal [Rational(5, 2), 0.25, BigDecimal("-2"), Complex(Rational(-1, 2), -2), 2.25, Complex(0.75, 1.5)], [i - r, f - r, r - b, r - c, i * f, f * c]
      assert_equal [Rational(6, 1), 1.5, BigDecimal("0.2"), Complex(Rational(1, 10), Rational(-1, 5)), BigDecimal("1.2")], [i / r, f / r, r / b, r / c, i / b]
      assert_equal [true, true, false, -1, 1, nil, nil], [r < f, f > r, i < r, r <=> f, b <=> r, c <=> i, i <=> c]
      assert_equal [6, [6, Rational(0, 1)], 6.0, 0.25, BigDecimal("0.5")], [i.div(r), i.divmod(r), i.fdiv(r), f % r, i.modulo(b)]
    end

    def test_numeric_comparable
      xs = [3, 1.5, Rational(1, 2), BigDecimal("2.5")] #: Array[Numeric]
      assert_equal [Rational(1, 2), 1.5, BigDecimal("2.5"), 3], xs.sort
      assert_equal [Rational(1, 2), 3, [Rational(1, 2), 3]], [xs.min, xs.max, xs.minmax]
      n = 2 #: Numeric
      assert_equal [true, false, 2.5, Rational(5, 2), 2], [n.between?(1, 3), n.between?(Rational(5, 2), 3), n.clamp(2.5, 4), n.clamp(Rational(5, 2), 3), n.clamp(1, 3)]
      assert_equal [2, Rational(1, 2), true, BigDecimal("2"), 3], [Rational(5, 2).clamp(1, 2), 0.25.clamp(Rational(1, 2), 1), 1.5.between?(1, Rational(3, 2)), BigDecimal("1").clamp(BigDecimal("2"), 3), BigDecimal("4").clamp(1, 3)]
    end

    def test_numeric_predicates
      zs = [0, 0.0, Rational(0, 1), Complex(0, 0), BigDecimal("0")]
      ns = [-2, -1.5, Rational(-1, 2), BigDecimal("-2.5")]
      assert_equal [true, true, true, true, true], zs.map(&:zero?)
      assert_equal [nil, nil, nil, nil, nil], zs.map(&:nonzero?)
      assert_equal [-2, -1.5, Rational(-1, 2), BigDecimal("-2.5")], ns.map(&:nonzero?)
      assert_equal [[false, true], [false, true], [false, true], [false, true]], ns.map { |x| [x.positive?, x.negative?] }
      assert_equal [true, false, false, false, false], zs.map(&:integer?)
      assert_equal [true, true, true, true, true], zs.map(&:finite?)
      assert_equal [nil, 1, -1, nil, nil, 1, -1, nil], [3.infinite?, Float::INFINITY.infinite?, (-Float::INFINITY).infinite?, Rational(1, 2).infinite?, Complex(1, 2).infinite?, Complex(Float::INFINITY, 0).infinite?, BigDecimal("-Infinity").infinite?, BigDecimal("1").infinite?]
      assert_equal [true, true, true, false, true], zs.map(&:real?)
      assert_equal [2, 1.5, Rational(1, 2), BigDecimal("2.5")], ns.map(&:abs)
    end

    def test_numeric_division_family
      assert_equal [2, 3, 3, 0, 1], [7.div(2.5), 7.5.div(2), Rational(7, 2).div(1), Rational(1, 2).div(0.75), BigDecimal("7").div(4)]
      assert_equal [[10, Rational(1, 6)], [3, 1.5], [-4, Rational(1, 2)]], [Rational(7, 2).divmod(Rational(1, 3)), 7.5.divmod(2), Rational(-7, 2).divmod(1)]
      assert_equal [Rational(1, 2), 0.0, Rational(1, 1), 0.5], [Rational(7, 2) % 3, Rational(7, 2).modulo(0.5), 7 % Rational(2, 1), 7.5 % Rational(1)]
      assert_equal [-2.0, Rational(-1, 2), 1.5, -1.5], [-7.remainder(2.5), Rational(-7, 2).remainder(1), 5.5.remainder(2), -5.5.remainder(2)]
      assert_equal [6.0, 0.75, 0.25, 0.3333333333333333, 0.5], [3.fdiv(Rational(1, 2)), 1.5.fdiv(2), Rational(1, 2).fdiv(2), BigDecimal("1").fdiv(3), Rational(1, 4).fdiv(0.5)]
      assert_equal [Rational(3, 2), 0.5, Rational(1, 4)], [3.quo(2), 1.5.quo(3), Rational(1, 2).quo(2)]
      assert_raises(ZeroDivisionError) { 1.5.div(0) }
      assert_raises(ZeroDivisionError) { Rational(1, 2) % 0 }
    end

    def test_numeric_step
      assert_equal [[1.0, 1.5, 2.0], [Rational(1, 2), Rational(3, 2)], [1.0, 1.5, 2.0], [2, 6, 10]], [1.step(2, 0.5).to_a, Rational(1, 2).step(2).to_a, 1.0.step(2.0, 0.5).to_a, 2.step(10, 4).to_a]
      assert_equal [BigDecimal("1"), BigDecimal("1.5"), BigDecimal("2")], BigDecimal("1").step(2, BigDecimal("0.5")).to_a
      assert_equal [Rational(1, 1), Rational(2, 3), Rational(1, 3)], Rational(1, 1).step(Rational(1, 3), Rational(-1, 3)).to_a
      out = [] #: Array[Rational]
      Rational(1, 3).step(1, Rational(1, 3)) { |x| out << x }
      big = [] #: Array[String]
      BigDecimal("0.1").step(BigDecimal("0.3"), BigDecimal("0.1")) { |x| big << x.to_s }
      assert_equal [[Rational(1, 3), Rational(2, 3), Rational(1, 1)], %w[0.1e0 0.2e0 0.3e0]], [out, big]
      assert_raises(ArgumentError) { Rational(1, 2).step(2, 0r) { |x| out << x } }
    end

    def test_numeric_coerce_to_c_and_i
      assert_equal [[3, 3], [1.5, 3.0], [0.5, 3.0], [3.0, 1.5], [0.5, 1.5]], [3.coerce(3), 3.coerce(1.5), 3.coerce(Rational(1, 2)), 1.5.coerce(3), 1.5.coerce(Rational(1, 2))]
      assert_equal [[Rational(3, 1), Rational(1, 2)], [1.5, 0.5], [Complex(3, 0), Complex(1, 2)], [BigDecimal("2"), BigDecimal("1")]],
                   [Rational(1, 2).coerce(3), Rational(1, 2).coerce(1.5), Complex(1, 2).coerce(3), BigDecimal("1").coerce(2)]
      assert_equal [Complex(3, 0), Complex(1.5, 0), Complex(Rational(1, 2), 0), Complex(0, 3), Complex(0, 1.5), Complex(0, Rational(1, 2))],
                   [3.to_c, 1.5.to_c, Rational(1, 2).to_c, 3.i, 1.5.i, Rational(1, 2).i]
    end

    def test_numeric_methods_on_a_numeric
      xs = [-3, 2.5, Rational(-1, 2), Complex(3, 4), BigDecimal("-1.5")] #: Array[Numeric]
      assert_equal [3, 2.5, Rational(1, 2), 5.0, BigDecimal("1.5")], xs.map(&:abs)
      assert_equal [true, false, false, false, false], xs.map(&:integer?)
      assert_equal [false, false, false, false, false], xs.map(&:zero?)
      assert_equal [true, true, true, false, true], xs.map(&:real?)
      assert_equal [-3.0, 2.5, -0.5, -1.5], [xs[0].to_f, xs[1].to_f, xs[2].to_f, xs[4].to_f]
      n = 7 #: Numeric
      assert_equal [3.5, 3, [3, 1], 1, 1, Complex(7, 0), Complex(0, 7)], [n.fdiv(2), n.div(2), n.divmod(2), n.modulo(2), n.remainder(2), n.to_c, n.i]
      m = -7.5 #: Numeric
      assert_equal [-4, [-4, 0.5], 0.5, -1.5, Complex(-7.5, 0), -7.5, nil], [m.div(2), m.divmod(2), m % 2, m.remainder(2), m.to_c, m.nonzero?, m.infinite?]
    end
  end

  # Integer ops at the 64-bit edge (decision 35): MRI's exact answer where it fits, RangeError where MRI makes a Bignum.
  class NumberEdgeOverflowTest < Minitest::Test
    MX = 9_223_372_036_854_775_807
    MN = -MX - 1

    def big_or_range(want, got)
      assert_equal true, [want, "RangeError"].include?(got)
    end

    def test_range_each_stops_at_max
      n = 0
      (MX - 1..MX).each { |_i| n += 1; break if n > 5 }
      a = [] #: Array[Integer]
      (MX - 1...MX).each { |i| a << i }
      b = [] #: Array[Integer]
      (MN..MN + 1).each { |i| b << i }
      c = [] #: Array[Integer]
      (MX - 2..MX).each { |i| c << i }
      assert_equal [2, [MX - 1], [MN, MN + 1], [MX - 2, MX - 1, MX]], [n, a, b, c]
    end

    def test_endless_range_past_max
      a = [] #: Array[Integer]
      r = number_class_of { (MX - 1..).each { |i| a << i; break if a.size >= 3 } }
      assert_equal [MX - 1, MX], a.first(2)
      assert_equal true, %w[ok RangeError].include?(r)
      assert_equal true, a.size == 2 || a[2] > 0
    end

    def test_range_size_and_count
      assert_equal [MX, MX - 1, 0, 1], [(1..MX).size, (1...MX).size, (MX..MN).size, (MX..MX).count]
      big_or_range "18446744073709551616", number_big { (MN..MX).size }
      big_or_range "9223372036854775808", number_big { (0..MX).size }
      big_or_range "9223372036854775809", number_big { (-1..MX).count }
      big_or_range "9223372036854775808", number_big { (MN..-1).size }
      big_or_range "9223372036854775808", number_big { (MN...0).size }
      big_or_range "9223372036854775809", number_big { (MN..0).size }
      big_or_range "18446744073709551615", number_big { (-MX..MX).size }
    end

    def test_upto_downto_step_stop_at_the_bound
      n = 0
      (MX - 1).upto(MX) { |_i| n += 1; break if n > 5 }
      m = 0
      (MN + 1).downto(MN) { |_i| m += 1; break if m > 5 }
      k = 0
      (MX - 1).step(MX, 1) { |_i| k += 1; break if k > 5 }
      assert_equal [2, 2, 2], [n, m, k]
      a = [] #: Array[Integer]
      (MX - 2).upto(MX) { |i| a << i }
      b = [] #: Array[Integer]
      (MN + 2).downto(MN) { |i| b << i }
      c = [] #: Array[Integer]
      (MX - 3).step(MX, 2) { |i| c << i }
      d = [] #: Array[Integer]
      1.step(MX, MX) { |i| d << i }
      e = [] #: Array[Integer]
      (MN + 3).step(MN, -2) { |i| e << i }
      f = [] #: Array[Integer]
      0.step(MN, MN) { |i| f << i }
      g = [] #: Array[Integer]
      (-1).step(MN, MN) { |i| g << i }
      assert_equal [[MX - 2, MX - 1, MX], [MN + 2, MN + 1, MN], [MX - 3, MX - 1],
                    [1], [MN + 3, MN + 1], [0, MN], [-1]], [a, b, c, d, e, f, g]
    end

    def test_upto_downto_size_and_step_enumerators
      assert_equal [1, 1], [MX.upto(MX).size || 0, MN.downto(MN).size || 0]
      big_or_range "18446744073709551616", number_big { MN.upto(MX).size || 0 }
      big_or_range "9223372036854775808", number_big { 0.upto(MX).size || 0 }
      big_or_range "18446744073709551616", number_big { MX.downto(MN).size || 0 }
      big_or_range "18446744073709551616", number_big { MN.step(MX).size }
      assert_equal [MX - 3, MX - 1], (MX - 3).step(MX, 2).to_a
      assert_equal [1], 1.step(MX, MX).to_a
    end

    def test_gcd_lcm_at_min
      assert_equal [2, 2, 1, 4], [MN.gcd(6), 6.gcd(MN), MN.gcd(MX), MN.gcd(-4)]
      big_or_range "9223372036854775808", number_big { MN.gcd(0) }
      big_or_range "9223372036854775808", number_big { 0.gcd(MN) }
      big_or_range "9223372036854775808", number_big { MN.gcd(MN) }
      big_or_range "9223372036854775808", number_big { MN.lcm(1) }
      big_or_range "9223372036854775808", number_big { MN.lcm(MN) }
      big_or_range "27670116110564327424", number_big { MN.lcm(3) }
    end

    def test_shift_by_huge_counts
      assert_equal [0, 0, 0, -1, -1, 0, 0, -1, -1, -1, 0, 0, 0, 0],
                   [0 >> MN, 1 << MN, 0 << MN, -1 << MN, -5 << MN, 0 << MX, 1 >> MX, -5 >> MX, -5 >> 100, -5 << -MX, 5 << -100, 0 >> -MX, 0 << MX, 5 << MN]
    end

    def test_range_sum_reverse_each_bsearch_at_the_edge
      assert_equal [MX - 1, 0, 0], [(MX - 1...MX).sum, (MX...MX).sum, (-MX..MX).sum]
      big_or_range "18446744073709551613", number_big { (MX - 1..MX).sum }
      big_or_range "42535295865117307928310139910543638528", number_big { (0..MX).sum }
      a = [] #: Array[Integer]
      (MN..MN + 2).reverse_each { |i| a << i }
      b = [] #: Array[Integer]
      (MN...MN).reverse_each { |i| b << i }
      assert_equal [[MN + 2, MN + 1, MN], []], [a, b]
      assert_equal [5, nil, MX, MN], [(MN..MX).bsearch { |x| x >= 5 }, (MN...MN).bsearch { |_x| true }, (0..MX).bsearch { |x| x >= MX }, (MN..MX).bsearch { |_x| true }]
    end

    def test_shift_that_still_fits
      assert_equal [MN, MN, MX, -4_611_686_018_427_387_904], [-1 << 63, -2 >> -62, MX >> 0, -1 << 62]
      big_or_range "9223372036854775808", number_big { 1 << 63 }
      big_or_range "-18446744073709551616", number_big { -2 << 63 }
    end

    def test_format_d_of_a_float_past_int64
      z = 0.0 #: Float
      assert_equal ["FloatDomainError", "FloatDomainError", "FloatDomainError"],
                   [number_class_of { format("%d", z / z) }, number_class_of { format("%d", 1.0 / z) }, number_class_of { format("%x", -1.0 / z) }]
      assert_equal true, ["100000000000000000000", "RangeError"].include?(begin; format("%d", 1e20); rescue RangeError; "RangeError"; end)
      assert_equal true, ["56bc75e2d63100000", "RangeError"].include?(begin; format("%x", 1e20); rescue RangeError; "RangeError"; end)
      assert_equal "-9223372036854775808", format("%d", -9_223_372_036_854_775_808.0)
      assert_equal true, ["9300000000000000000", "RangeError"].include?(begin; format("%d", 9.3e18); rescue RangeError; "RangeError"; end)
    end
  end

  # ruby/spec core/range gaps (#49): % and step without a block
  class NumberRubySpecRangeStepTest < Minitest::Test
    def test_range_percent
      assert_equal [[1, 4, 7, 10], [1, 4, 7], [2, 10, 18], %w[a c e]], [((1..10) % 3).to_a, (1...10).step(3).to_a, ((1..10) % 4).map { |x| x * 2 }, ("a".."e").step(2).to_a]
    end
  end

  # #102: Complex from a String and from Complex parts
  class NumberComplexParseTest < Minitest::Test
    def test_kernel_complex_string
      assert_equal [Complex(1, 2), Complex(Rational(2, 3)), Complex(4, Rational(2, 3)), Complex(0, -1), Complex(0, 35), Complex(2e3, 4), Complex(79, 40), Complex(79, 4)],
                   [Complex("1+2i"), Complex("2/3"), Complex("4+2/3i"), Complex("-i"), Complex("35i"), Complex("2e3+4i"), Complex("7_9+4_0i"), Complex("  79+4j  ")]
      assert_equal Complex.polar(2, 0), Complex("2@0")
      e = begin; Complex("79+4iruby"); rescue ArgumentError => x; x.message; end
      assert_equal 'invalid value for convert(): "79+4iruby"', e
    end

    def test_kernel_complex_complex_parts
      assert_equal [Complex(1, 2), Complex(-3, 9), Complex(1, 0)], [Complex(Complex(1, 2)), Complex(Complex(3, 4), Complex(5, 6)), Complex(Complex(1, 0))]
    end

    def test_string_to_c
      assert_equal [Complex(79, 4), Complex(0, 0), Complex(3, 0)], ["79+4iruby".to_c, "ruby".to_c, "3".to_c]
    end

    def test_rectangular_complex_part
      assert_equal Complex(1.0, 2), Complex.rectangular(1.0 + 0i, 2 + 0.0i)
      assert_equal TypeError, (begin; Complex.rectangular(1.0 + 1i, 2); rescue TypeError => e; e.class; end)
    end
  end

  # #102: Float rounding to digits <= 0, zero modulus, digits past a C int
  class NumberFloatDigitsTest < Minitest::Test
    def test_float_round_nonpositive_digits
      assert_equal [12300, 12300, 12400, 3, -3, 1, 0], [12345.67.round(-2), 12345.67.floor(-2), 12345.67.ceil(-2), 2.5.round(0), -2.5.round(0), 1.5.floor(0), 4.2.round(-1)]
    end

    def test_zero_modulus
      zd = [-> { 1.0 % 0 }, -> { 1.0 % 0.0 }, -> { 10 % 0.0 }, -> { 1.0.divmod(0.0) }, -> { 10.divmod(0.0) }, -> { 1.0.modulo(0.0) }].map do |f|
        begin
          f.call
          nil
        rescue ZeroDivisionError => e
          e.class
        end
      end
      assert_equal [ZeroDivisionError] * 6, zd
    end

    # Go package and builtin names the emitters use (math.Copysign, os.Getpid, min) must survive same-named locals.
    def test_locals_named_like_go_names
      math = 1
      os = 2
      min = 3
      neg = -0.0
      pid = $$
      args = [1]
      assert_equal [1, 2, 3], [math, os, min]
      assert_equal "-0.0", neg.to_s
      assert_equal Integer, pid.class
      assert_equal 6, number_chain(*args)
    end

    def test_digits_out_of_int
      assert_equal RangeError, (begin; 42.round(1 << 31); rescue RangeError => e; e.class; end)
    end
  end
end

module NumberTests
  class FloatArgTest < Minitest::Test
    def test_arg
      assert_equal [0, 0, Math::PI, Math::PI, 0, Math::PI], [1.0.arg, 0.0.arg, -0.0.arg, -1.0.arg, 1.0.angle, -1.0.phase]
      assert_equal [0, Math::PI], [Float::INFINITY.arg, (-Float::INFINITY).arg]
      assert Float::NAN.arg.nan?
      assert_equal [0, Math::PI, 0, Math::PI], [1.arg, -1.arg, 0.angle, -5.phase]
    end
  end
end

module NumberTests
  # decision 365: a user class may subclass Numeric; it is an Object with Comparable here
  class NumSub < Numeric
    def initialize(value = 1.0)
      @value = value #: Float
    end

    def to_f = @value
  end

  class NumericSubclassTest < Minitest::Test
    def test_numeric_subclass
      assert_equal 2.0, NumSub.new(2.0).to_f
      assert_equal 1.0, NumSub.new.to_f
    end
  end
end

module NumberTests
  # decision 367: Float#rationalize, MRI's simplest rational
  class FloatRationalizeTest < Minitest::Test
    def test_rationalize
      assert_equal Rational(4806858197361, 1421), 3382729202.92822.rationalize
      assert_equal [Rational(1, 3), Rational(1, 3), Rational(3, 10), Rational(-3, 10)],
                   [0.3.rationalize(Rational(1, 10)), 0.3.rationalize(0.05), 0.3.rationalize(0.001), (-0.3).rationalize(0.001)]
      assert_equal Rational(333, 1000), 0.333.rationalize
    end
  end
end

module NumberTests
  # --int-overflow lanes (decision 372): each test runs only where testdata/int-overflow/<mode>-skip.txt leaves it in.
  class IntOverflowTest < Minitest::Test
    # Under wrap the low bits of an overflowing +, * and -@ are MRI's exact answer's; raise and promote leave it out.
    def test_wrap_low_bits
      a = number_id(0x1234_5678_9abc_def0)
      assert_equal 0xdeee_eeee_d1d0, (a * 0x0100_0000_01b3) & 0xffff_ffff_ffff
      max = number_id(0x7fff_ffff_ffff_ffff)
      assert_equal 0xfffe, (max + max) & 0xffff
      assert_equal 0, -(-max - 1) & 0xffff
    end

    # sum's block loop (decision 402) adds with the same checked `+` as the prelude's sum.
    def test_wrap_sum_low_bits
      max = number_id(0x7fff_ffff_ffff_ffff)
      xs = [max, 1, 2]
      low = xs.sum { |x| x } & 0xffff
      assert_equal 2, low
    end

    def test_promote_growth
      f = 1
      1.upto(25) { |i| f *= i }
      assert_equal "15511210043330985984000000", f.to_s
      a = 0
      b = 1
      100.times { a, b = b, a + b }
      assert_equal 354224848179261915075, a
      assert_equal 1267650600228229401496703205376, 2**100
      assert_equal [0, 1], [(2**64) % 2, (2**64 + 1) % 2]
      assert_equal true, a > 2**63
      assert_equal Integer, f.class
    end
  end
end
