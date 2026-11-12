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

  class Bag
    def initialize(xs) = @xs = xs

    def find
      @xs.each { |x| return x if yield(x) }
      nil
    end
  end

  class Cop
    def assign_rhs(node)
      rhs_node = node.each_ancestor.find do |ancestor|
        break unless ancestor.loc?(:dot) || ancestor.up

        ancestor.loc?(:dot)
      end

      if rhs_node && block_given?
        yield rhs_node
      else
        rhs_node
      end
    end

    def bag_first(bag) = bag.find { |x| x > 1 }

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
    h = { low: low, top: top, bag: Bag.new([1, 2, 3]) } #: Hash[Symbol, untyped]
    c = Cop.new
    [c.above(h[:low]).equal?(top), c.above(h[:top]), c.rhs(h[:low]).equal?(top), c.rhs(h[:low]) { |r| r.loc.dot.line }, c.bag_first(h[:bag]), c.assign_rhs(h[:low]).equal?(top), c.assign_rhs(h[:top])]
  end
end
