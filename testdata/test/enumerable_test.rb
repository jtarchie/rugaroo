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
      assert_equal "abc", %w[b c].sum("a")
      assert_equal [1, 2, 3], [[2], [3]].sum([1])
      assert_equal [1, 2], [[1], [2]].sum([])
    end

    def test_inject_symbol
      assert_equal 6, [1, 2, 3].inject(:+)
      assert_equal 6, [1, 2, 3].reduce(:*)
      assert_equal 6, [1, 2, 3].reduce(1, :*)
      assert_equal 16, [1, 2, 3].inject(10, :+)
      assert_nil [].inject(:+)
      assert_equal 3, [5, 3, 9].inject(:gcd).then { |g| g + 2 }
      assert_equal "abc", %w[a b c].inject(:+)
      assert_equal 6.5, [1, 2, 3].inject(0.5, :+)
      assert_equal 6, Bag.new(1, 2, 3).inject(:+)
      assert_equal 60, Bag.new(3, 4, 5).reduce(1, :*)
      assert_equal 10, (1..4).inject(:+)
    end

    def test_min_max_with_block
      assert_equal 3, [1, 2, 3].min { |a, b| b <=> a }
      assert_equal 1, [1, 2, 3].max { |a, b| b <=> a }
      assert_equal "ccc", %w[a ccc bb].max { |a, b| a.size <=> b.size }
      assert_equal [3, 1], [1, 2, 3].minmax { |a, b| b <=> a }
      assert_equal [3, 2], [1, 2, 3].min(2) { |a, b| b <=> a }
      assert_equal [1, 2], [1, 2, 3].max(2) { |a, b| b <=> a }
      assert_nil [].min { |a, b| a <=> b }
      assert_equal [nil, nil], [].minmax { |a, b| a <=> b }
      assert_equal "a", %w[a b c].minmax { |a, b| a.size <=> b.size }.first
      assert_equal 3, Bag.new(1, 2, 3).min { |a, b| b <=> a }
      assert_equal 1, Bag.new(1, 2, 3).max { |a, b| b <=> a }
      assert_equal [3, 1], Bag.new(1, 2, 3).minmax { |a, b| b <=> a }
      assert_equal [1, 2], Bag.new(3, 1, 2).max(2) { |a, b| b <=> a }
      assert_equal 4, (1..4).min { |a, b| b <=> a }
      assert_equal [4, 1], (1..4).minmax { |a, b| b <=> a }
    end

    def test_tally_into_hash
      h = { 1 => 5 }
      assert_equal({ 1 => 6, 2 => 1, 3 => 1 }, [1, 2, 3].tally(h))
      assert_equal({ 1 => 6, 2 => 1, 3 => 1 }, h)
      assert_equal({ 1 => 2, 2 => 1, 3 => 1 }, Bag.new(1, 2, 3).tally({ 1 => 1 }))
      assert_equal({ 1 => 1, 2 => 1 }, [1, 2].tally(Hash.new(100)))
      assert_same h, [4].tally(h)
      frozen = { 1 => 1 }.freeze
      begin
        Bag.new.tally(frozen)
        flunk "tally into a frozen Hash"
      rescue FrozenError => e
        assert_kind_of FrozenError, e
      end
    end
  end

  class OptBag
    include Enumerable #[Integer?]

    #: (*Integer?) -> void
    def initialize(*ns)
      @ns = ns
    end

    #: () { (Integer?) -> void } -> void
    def each(&block) = @ns.each(&block)
  end

  class StrBag
    include Enumerable #[String]

    #: (*String) -> void
    def initialize(*ss)
      @ss = ss
    end

    #: () { (String) -> void } -> void
    def each(&block) = @ss.each(&block)
  end

  # The forms decision 450 adds (#145)
  class MoreFormsTest < Minitest::Test
    def test_to_h_and_uniq_blocks
      assert_equal({ 3 => 6, 1 => 2, 2 => 4 }, Bag.new(3, 1, 2).to_h { |x| [x, x * 2] })
      assert_equal [3, 2], Bag.new(3, 1, 2).uniq { |x| x % 2 }
      assert_equal({ 1 => 1, 2 => 4 }, (1..2).to_h { |x| [x, x * x] })
    end

    def test_find_ifnone_and_compact
      assert_equal 0, Bag.new(3, 1, 2).find(-> { 0 }) { |x| x > 5 }
      assert_equal 3, Bag.new(3, 1, 2).find(-> { 0 }) { |x| x > 2 }
      assert_equal 0, (1..4).find(-> { 0 }) { |x| x > 9 }
      assert_equal [3, 1], OptBag.new(3, nil, 1, nil).compact
    end

    def test_slice_by_pattern
      assert_equal [["a", "b"], ["x", "c"]], StrBag.new("a", "b", "x", "c").slice_before(/x/).to_a
      assert_equal [["a", "b"], ["x", "c"]], StrBag.new("a", "b", "x", "c").slice_after(/b/).to_a
    end

    def test_sum_with_string_or_array_init_and_block
      assert_equal "312", Bag.new(3, 1, 2).sum("") { |x| x.to_s }
      assert_equal [3, 1], Bag.new(3, 1).sum([]) { |x| [x] }
      assert_equal "123", (1..3).sum("") { |x| x.to_s }
      assert_equal "ab", %w[a b].sum("") { |x| x }
    end

    def test_chain
      assert_equal [3, 1, 7, 8], Bag.new(3, 1).chain([7], [8]).to_a
      assert_equal [1, 2, 3], (1..2).chain([3]).to_a
    end

    def test_zip_block
      out = [] #: Array[[Integer, Integer?]]
      Bag.new(3, 1).zip([4]) { |pr| out << pr }
      assert_equal [[3, 4], [1, nil]], out
    end

    def test_blockless_with_index
      assert_equal [2, 1, 3], Bag.new(3, 1, 2).sort_by.with_index { |_x, i| -i }
      assert_equal [2, 1, 3], [3, 1, 2].sort_by.each_with_index { |_x, i| -i }
      assert_equal 2, [3, 1, 2].max_by.with_index { |_x, i| i }
      assert_equal 3, [3, 1, 2].min_by.with_index(1) { |_x, i| i }
      assert_equal({ true => [3, 2], false => [1] }, [3, 1, 2].group_by.with_index(1) { |_x, i| i.odd? })
      assert_equal [[1, 3], [2, 4]], (1..4).partition.with_index { |_x, i| i.even? }
    end
  end
end
