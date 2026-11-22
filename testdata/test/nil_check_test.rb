# rbs_inline: enabled

# nil_check: runtime

require "minitest/autorun"

# Decision 393: MRI's own error at a core operator; elsewhere rugaroo raises at the boundary where MRI fails later, so only "raised" is checked.

module NilCheckTests
  class Leaf
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end
  end

  class Base
    #: () -> String
    def name = "base"
  end

  class Kid < Base
    #: () -> String
    def name = "kid"
  end

  class OperatorTest < Minitest::Test
    #: (Integer) -> String?
    def str(i) = i > 0 ? "s#{i}" : nil

    #: (Integer) -> Integer?
    def int(i) = i > 0 ? i : nil

    #: (Integer) -> Float?
    def flt(i) = i > 0 ? i.to_f : nil

    #: () { () -> untyped } -> [untyped, String]?
    def error_of
      yield
      nil
    rescue TypeError, ArgumentError => e
      [e.class, e.message]
    end

    def test_present_values_pass
      assert_equal "xs1", "x" + str(1)
      assert_equal 4, 3 + int(1)
      assert_equal 2.5, 1.5 + flt(1)
      assert_equal 2.0, 1 + flt(1)
      assert_equal 2, [1, 2, 3][int(1)]
      assert_equal true, 0 < int(1)
    end

    def test_string
      assert_equal [TypeError, "no implicit conversion of nil into String"], error_of { "x" + str(0) }
      assert_equal [ArgumentError, "comparison of String with nil failed"], error_of { "a" < str(0) }
      assert_equal [TypeError, "no implicit conversion from nil to integer"], error_of { "ab" * int(0) }
      assert_equal [TypeError, "no implicit conversion from nil to integer"], error_of { "abc"[int(0)] }
    end

    # decision 382: `s = s + a + b` appends into s's buffer only after every argument is checked (`s.dup`: passing s is an alias)
    def test_string_plus_append_chain
      s = "a"
      i = 0
      while i < 3
        s = s + "<" + str(i + 1) + ">"
        i += 1
      end
      assert_equal "a<s1><s2><s3>", s.dup
      msg = nil
      begin
        s = s + "x" + str(0)
      rescue TypeError => e
        msg = e.message
      end
      assert_equal "a<s1><s2><s3>", s.dup
      assert_equal "no implicit conversion of nil into String", msg
      begin
        s += str(0)
      rescue TypeError => e
        msg = e.message
      end
      assert_equal "a<s1><s2><s3>", s.dup
      s += str(4)
      assert_equal "a<s1><s2><s3>s4", s
    end

    def test_integer
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 1 + int(0) }
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 1 - int(0) }
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 3 * int(0) }
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 3 / int(0) }
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 3 % int(0) }
      assert_equal [TypeError, "nil can't be coerced into Integer"], error_of { 1 + flt(0) }
      assert_equal [ArgumentError, "comparison of Integer with nil failed"], error_of { 3 < int(0) }
      assert_equal [ArgumentError, "comparison of Integer with nil failed"], error_of { 3 >= int(0) }
      assert_equal [ArgumentError, "comparison of Integer with nil failed"], error_of { 5.between?(1, int(0)) }
      assert_equal [TypeError, "no implicit conversion of nil into Integer"], error_of { 1 << int(0) }
    end

    def test_float
      assert_equal [TypeError, "nil can't be coerced into Float"], error_of { 1.5 + flt(0) }
      assert_equal [TypeError, "nil can't be coerced into Float"], error_of { 1.5 * flt(0) }
      assert_equal [TypeError, "nil can't be coerced into Float"], error_of { 1.5 + int(0) }
      assert_equal [ArgumentError, "comparison of Float with nil failed"], error_of { 1.5 < flt(0) }
    end

    def test_index
      assert_equal [TypeError, "no implicit conversion from nil to integer"], error_of { [1, 2, 3][int(0)] }
    end

    def test_equality_answers
      assert_equal false, 1 == int(0)
      assert_equal true, 1 != int(0)
      assert_nil 1 <=> int(0)
      assert_nil 1.0 <=> flt(0)
      assert_nil "a" <=> str(0)
      assert_equal false, "a" == str(0)
    end
  end

  class BoundaryTest < Minitest::Test
    #: (Integer) -> Integer?
    def int(i) = i > 0 ? i : nil

    #: (Integer) -> Leaf?
    def leaf(i) = i > 0 ? Leaf.new(i) : nil

    #: (Integer) -> Base?
    def base(i) = i > 0 ? Kid.new : nil

    #: (Integer) -> Integer
    def twice(x) = x * 2

    #: (Leaf) -> Integer
    def leaf_value(l) = l.v

    #: (Base) -> String
    def base_name(b) = b.name

    #: () { () -> untyped } -> bool
    def raised?
      yield
      false
    rescue StandardError
      true
    end

    def test_present_values_pass
      assert_equal 2, twice(int(1))
      assert_equal 1, leaf_value(leaf(1))
      assert_equal "kid", base_name(base(1))
    end

    def test_nil_raises
      assert raised? { twice(int(0)) }
      assert raised? { leaf_value(leaf(0)) }
      assert raised? { base_name(base(0)) }
    end
  end

  # `a[i] op= b[j]`: a nilable element operand still updates in place, and raises MRI's error when it is nil (decision 489).
  class ElemOpAssignNilableTest < Minitest::Test
    def test_in_place
      sums = [0.0, 1.0]
      vals = [0.5, 2.0]
      counts = [10, 20]
      ints = [1, 2]
      bins = [1, 1, 0]
      bins.each_with_index do |b, j|
        sums[b] += vals[j % 2]
        counts[b] += ints[j % 2]
      end
      assert_equal [0.5, 3.5], sums
      assert_equal [11, 23], counts
      assert_equal 7.5, (sums[1] += vals[1] * 2)
      sums[0] -= vals[0]
      counts[1] *= ints[1]
      assert_equal [[0.0, 7.5], [11, 46]], [sums, counts]
    end

    def test_nil_operand
      sums = [0.0, 1.0]
      vals = [0.5]
      counts = [10]
      ints = [3]
      k = 5
      begin
        sums[1] += vals[k]
      rescue TypeError => e
        assert_equal [TypeError, "nil can't be coerced into Float", [0.0, 1.0]], [e.class, e.message, sums]
      end
      begin
        counts[0] += ints[-k]
      rescue TypeError => e
        assert_equal [TypeError, "nil can't be coerced into Integer", [10]], [e.class, e.message, counts]
      end
      sums.freeze
      begin
        sums[0] += vals[k]
      rescue TypeError => e
        assert_equal "nil can't be coerced into Float", e.message
      end
      begin
        sums[0] += vals[0]
      rescue FrozenError => e
        assert_equal FrozenError, e.class
      end
    end
  end
end
