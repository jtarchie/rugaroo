module Findish
  class Dot
    attr_reader :line, :column

    def initialize(l, c) = (@line = l; @column = c)
  end

  class Loc
    attr_reader :dot

    def initialize(dot) = @dot = dot
  end

  class Nd
    attr_reader :loc, :up

    def initialize(dot, up = nil) = (@loc = Loc.new(dot); @up = up)

    def loc?(name) = name == :dot && !@loc.dot.nil?

    def each_ancestor(&block)
      return to_enum(__method__) unless block

      n = up
      while n
        yield n
        n = n.up
      end
      self
    end
  end

  class Cop
    def above(node)
      node.each_ancestor.find do |a|
        dot = a.loc.dot if a.loc?(:dot)
        next unless dot

        dot.line == node.loc.dot.line - 1 && dot.column == node.loc.dot.column
      end
    end

    def rhs(node)
      r = node.each_ancestor.find do |ancestor|
        ancestor.loc?(:dot)
      end
      r && block_given? ? yield(r) : r
    end
  end

  def self.demo
    top = Nd.new(Dot.new(1, 4))
    mid = Nd.new(nil, top)
    low = Nd.new(Dot.new(2, 4), mid)
    h = { low: low, top: top } #: Hash[Symbol, untyped]
    c = Cop.new
    [c.above(h[:low]).equal?(top), c.above(h[:top]), c.rhs(h[:low]).equal?(top), c.rhs(h[:low]) { |r| r.loc.dot.line }]
  end
end
