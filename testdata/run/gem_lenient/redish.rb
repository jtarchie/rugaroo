module Redish
  class Rg
    attr_reader :src

    def initialize(src) = @src = src
    def join(other) = Rg.new(src + other.src)
  end

  class Nd
    def initialize(kids) = @kids = kids
    def children = @kids
    def source_range = Rg.new("n")
    def source = "s"
  end

  class Leaf
    def initialize(s) = @s = s
    def source_range = Rg.new(@s)
    def source = @s
    def children = []
  end

  class Al
    def add_offense(range, message:)
      r = "#{range.src}:#{message}"
      r += yield(range) if block_given?
      r
    end

    def for_args(node, &block)
      existing = node.children.map(&:source).join(' ')
      ranges = node.children.map(&:source_range)
      msg = format('use %<prefer>s not %<current>s', prefer: existing.upcase, current: existing)
      add_offense(ranges.reduce(&:join), message: msg, &block)
    end
  end
end
