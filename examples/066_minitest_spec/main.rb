# rbs_inline: enabled

# args: --seed 3

require "minitest/autorun"

class Cart
  #: () -> void
  def initialize
    @lines = {} #: Hash[String, Integer]
  end

  #: (String, ?Integer) -> void
  def add(item, qty = 1)
    raise ArgumentError, "qty must be positive" unless qty.positive?

    @lines[item] = (@lines[item] || 0) + qty
  end

  #: () -> Integer
  def count = @lines.values.sum

  #: () -> Array[String]
  def items = @lines.keys.sort

  #: () -> String
  def receipt = items.map { |i| "#{i} x#{@lines.fetch(i)}" }.join("\n")
end

describe Cart do
  let(:cart) { Cart.new }

  before do
    cart.add("apple", 2)
  end

  it "counts what was added" do
    _(cart.count).must_equal 2
  end

  it "rejects a zero quantity" do
    e = _ { cart.add("pear", 0) }.must_raise ArgumentError
    _(e.message).must_equal "qty must be positive"
  end

  describe "with several items" do
    before do
      cart.add("pear")
      cart.add("fig", 3)
    end

    it "sorts its items" do
      _(cart.items).must_equal %w[apple fig pear]
    end

    # Deliberately wrong: shows the diff for a multi-line value.
    it "prints a receipt" do
      _(cart.receipt).must_equal "apple x2\nfig x3\npear x2"
    end
  end

  it "applies discounts"
end

class CartSpec < Minitest::Spec
  subject { Cart.new }

  it "starts empty" do
    _(subject.items).must_be_empty
    _(subject.count).must_be :zero?
  end
end
