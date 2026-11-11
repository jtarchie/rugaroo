# rbs_inline: enabled

require "minitest/autorun"

# Enumerable's call forms beyond the plain one (#145), on Array and on a class that includes Enumerable.
module EnumerableTests
  class Bag
    include Enumerable #[Integer]

    #: (*Integer) -> void
    def initialize(*ns)
      @ns = ns
    end

    #: () { (Integer) -> void } -> void
    def each(&block) = @ns.each(&block)
  end

  class FormsTest < Minitest::Test
    def test_member
      assert [1, 2, 3].member?(2)
      refute [1, 2, 3].member?(5)
      assert Bag.new(1, 2, 3).member?(3)
      refute Bag.new(1, 2, 3).member?(0)
    end

    def test_sum_init
      assert_equal 16, [1, 2, 3].sum(10)
      assert_equal 6.0, [1, 2, 3].sum(0.0)
      assert_equal 7.0, [1.5, 2.5].sum(3)
      assert_equal 1.0, ([0.1] * 10).sum(0.0)
      assert_equal 13.0, [1, 2, 3].sum(1.0) { |x| x * 2 }
      assert_equal 22, [1, 2, 3].sum(10) { |x| x * 2 }
      assert_equal 3.5, [1, 2].sum(2) { |x| x * 0.5 }
      assert_equal Rational(7, 2), [1, 2].sum(Rational(1, 2))
      assert_equal 0, [].sum(0)
      assert_equal 16, Bag.new(1, 2, 3).sum(10)
      assert_equal 6.5, Bag.new(1, 2, 3).sum(0.5)
      assert_equal 12.0, Bag.new(1, 2, 3).sum(0.0) { |x| x * 2 }
      assert_equal 16, (1..3).sum(10)
    end

    def test_tally_into_hash
      h = { 1 => 5 }
      assert_equal({ 1 => 6, 2 => 1, 3 => 1 }, [1, 2, 3].tally(h))
      assert_equal({ 1 => 6, 2 => 1, 3 => 1 }, h)
      assert_equal({ 1 => 2, 2 => 1, 3 => 1 }, Bag.new(1, 2, 3).tally({ 1 => 1 }))
    end
  end
end
