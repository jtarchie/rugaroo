# rbs_inline: enabled

require "minitest/autorun"

module RangeTests
  # include?/member? iterate a String range; cover? and === compare.
  class RangeIncludeTest < Minitest::Test
    def test_string_include_iterates
      az = "a".."z"
      assert_equal [false, true, false, true], [az.include?("bb"), az.cover?("bb"), az.member?("bb"), az === "bb"]
      assert_equal [true, true, true], [az.include?("a"), az.include?("m"), az.include?("z")]
    end

    def test_string_include_multi_char_and_exclusive
      assert_equal [true, false, false], [("a".."zz").include?("bb"), ("aa".."zz").include?("c"), ("a"..."c").include?("c")]
      assert_equal [true, false, true], [("1".."10").include?("5"), ("1".."10").cover?("5"), ("1".."10").include?("10")]
      assert_equal [true, false], [("a".."e").include?("d"), ("a".."e").include?("ab")]
    end

    def test_string_include_endless_raises
      r = ("a"..) #: Range[String]
      cls = begin
        r.include?("b")
        nil
      rescue TypeError
        TypeError
      end
      assert_equal TypeError, cls
      assert_equal true, r.cover?("zzz")
    end

    def test_numeric_include_is_cover
      assert_equal [true, true, false], [(1.0..2.0).include?(1.5), (1..10).include?(5), (1.0..2.0).member?(2.5)]
      assert_equal [true, false], [(1..).include?(5), (1...3).member?(3)]
    end
  end

  class RangeEqTest < Minitest::Test
    def test_eq_compares_ends_with_eq
      one = 1..2
      flt = 1.0..2.0
      assert_equal [true, false, true, false], [one == flt, one.eql?(flt), one.eql?(1..2), one == (1...2)]
    end
  end

  # #102: Float steps, Float bsearch, beginless first, include? over succ
  class RangeFloatStepTest < Minitest::Test
    def test_step_floats
      a = [] #: Array[Float]
      (-5.0..5.0).step(2) { |x| a << x }
      (-1.0...1.0).step(0.5) { |x| a << x }
      (-2..2).step(1.5) { |x| a << x }
      (1.0...55.6).step(18.2) { |x| a << x }
      (-2.0..2.0).step { |x| a << x }
      assert_equal [-5.0, -3.0, -1.0, 1.0, 3.0, 5.0, -1.0, -0.5, 0.0, 0.5, -2.0, -0.5, 1.0, 1.0, 19.2, 37.4, 55.599999999999994, -2.0, -1.0, 0.0, 1.0, 2.0], a
      assert_equal [4, [1.0, 1.5, 2.0], [1.0, 1.5, 2.0]], [(1.0...55.6).step(18.2).size, (1.0..2.0).step(0.5).to_a, (1..2).step(0.5).to_a]
    end

    def test_step_negative_integer
      a = [] #: Array[Integer]
      (1..-1).step(-1) { |x| a << x }
      (-1..1).step(-1) { |x| a << x }
      assert_equal [1, 0, -1], a
    end

    def test_bsearch_floats
      assert_equal [3.5, nil, -1000.5, 3], [(0.0..10.0).bsearch { |x| x >= 3.5 }, (0.0..10.0).bsearch { |x| x >= 11 }, (-Float::INFINITY..0.0).bsearch { |x| x >= -1000.5 }, (..10).bsearch { |x| x >= 3 }]
    end

    def test_first_beginless
      assert_equal RangeError, (begin; (..1).first; rescue RangeError => e; e.class; end)
    end
  end

  class RangeSuccItem
    include Comparable

    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: (RangeSuccItem) -> Integer
    def <=>(other) = n <=> other.n

    #: () -> RangeSuccItem
    def succ = RangeSuccItem.new(n * 10)
  end

  class RangeIncludeSuccTest < Minitest::Test
    def test_include_iterates_succ
      r = RangeSuccItem.new(1)..RangeSuccItem.new(99)
      assert_equal [true, true, false, false], [r.include?(RangeSuccItem.new(1)), r.include?(RangeSuccItem.new(10)), r.include?(RangeSuccItem.new(2)), r.include?(RangeSuccItem.new(99))]
      assert_equal TypeError, (begin; (RangeSuccItem.new(0)..).include?(RangeSuccItem.new(5)); rescue TypeError => e; e.class; end)
    end
  end
end

module RangeTests
  class RangeLastNTest < Minitest::Test
    def test_last_n
      assert_equal [[], [1], [], [4, 5], [0, 1, 2, 3, 4], %w[d e]], [(0..2).last(0), (1..1).last(1), (0...0).last(2), (0..5).last(2), (0...5).last(10), ("a".."e").last(2)]
      assert_equal ArgumentError, (begin; (0..2).last(-1); rescue ArgumentError => e; e.class; end)
      assert_equal RangeError, (begin; (0..).last(1); rescue RangeError => e; e.class; end)
    end
  end
end

module RangeTests
  class RangeMinmaxTest < Minitest::Test
    def test_minmax_from_the_ends
      assert_equal [1, 3], (1..3).minmax
      assert_equal [1, 2], (1...3).minmax
      assert_equal [1, Float::INFINITY], (1..Float::INFINITY).minmax
      assert_equal [nil, nil], (3..1).minmax
    end
  end
end
