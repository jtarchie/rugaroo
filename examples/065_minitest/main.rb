# rbs_inline: enabled

# args: --seed 1

require "minitest/autorun"

class Inventory
  #: () -> void
  def initialize
    @items = {} #: Hash[String, Integer]
  end

  #: (String, Integer) -> void
  def add(name, count)
    raise ArgumentError, "count must be positive" unless count.positive?

    @items[name] = (@items[name] || 0) + count
  end

  #: (String) -> Integer
  def count(name) = @items.fetch(name)

  #: () -> String
  def report = @items.sort.map { |name, n| "#{name}: #{n}" }.join("\n")
end

class InventoryTest < Minitest::Test
  #: () -> void
  def setup
    @inv = Inventory.new
    @inv.add("apple", 3)
    @inv.add("pear", 1)
  end

  #: () -> void
  def test_add_accumulates
    @inv.add("apple", 2)
    assert_equal 5, @inv.count("apple")
  end

  #: () -> void
  def test_rejects_zero
    e = assert_raises(ArgumentError) { @inv.add("fig", 0) }
    assert_equal "count must be positive", e.message
  end

  #: () -> void
  def test_missing_item
    assert_raises(KeyError) { @inv.count("kiwi") }
  end

  #: () -> void
  def test_report_lines
    assert_includes @inv.report.lines.map(&:chomp), "pear: 1"
    assert_match(/apple: \d/, @inv.report)
  end

  # Deliberately wrong: shows minitest's diff output for multi-line strings.
  #: () -> void
  def test_report_diff
    assert_equal "apple: 3\npear: 2", @inv.report
  end

  # Deliberately wrong: a short value fails with Expected/Actual.
  #: () -> void
  def test_count_short
    assert_equal 4, @inv.count("apple"), "apples after setup"
  end

  #: () -> void
  def test_later
    skip "not written yet"
  end
end
