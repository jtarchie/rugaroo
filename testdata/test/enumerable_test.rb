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
  end
end
