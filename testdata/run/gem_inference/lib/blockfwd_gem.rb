# frozen_string_literal: true

# A block passed on to a method that yields (kramdown's `parse_raw_html(el, &block)` handing its block to
# `handle_html_start_tag(line, &block)`): the block literal the first caller wrote takes what the last method yields,
# so the element it hands on is typed (decision 500).
class BfEl
  attr_reader :name, :kids

  def initialize(name)
    @name = name
    @kids = []
  end
end

class BfParser
  def initialize
    @tree = BfEl.new("root")
    @out = []
  end

  def start_tag(line)
    el = BfEl.new("tag#{line}")
    yield(el, line > 1)
  end

  def raw(el, &block)
    @tree = el
    start_tag(1, &block)
    start_tag(2, &block)
  end

  def handle(el, closed)
    @out << "#{el.name}:#{closed}:#{parse_blocks(el)}"
  end

  def parse_blocks(el)
    @tree = el
    @tree.kids << BfEl.new("k")
    @tree.kids.size
  end

  # A stack of two tuple shapes, read by a block that names only the first field (kramdown's
  # `@stack.any? { |el, _| el.type == element }`): each name takes its position from whichever tuple it is.
  def stacked(name)
    @stack = []
    @stack.push([@tree, :text])
    @stack.push([BfEl.new("deep"), "src", 3])
    found = @stack.any? { |el, _| el.name == name }
    kinds = @stack.map { |_, kind| kind.to_s }
    @stack.pop
    "#{found}:#{kinds.join(",")}:#{@stack.size}"
  end

  def run
    parse_blocks(BfEl.new("top"))
    raw(BfEl.new("r")) { |iel, ic| handle(iel, ic) }
    start_tag(3) { |iel, ic| handle(iel, ic) }
    @out << stacked("deep") << stacked("none")
    @out.join(" ")
  end
end

# A block passed on to core methods that want other results of it (rubocop-ast's `find_comment(&block)` over
# `comments.find(&block)`, CollectionNode's `flat_map`, `partition`, `chunk_while`): the forwarded block answers
# untyped, and each callee takes it through an adapter, a condition by truthiness.
class BfList
  def initialize(items)
    @items = items
  end

  def pick(&block)
    @items.find(&block)
  end

  def spread(&block)
    @items.flat_map(&block)
  end

  def split(&block)
    @items.partition(&block)
  end

  def runs(&block)
    @items.chunk_while(&block).to_a
  end
end
