module Fwdish
  module SimpleFwd
    def def_delegators(accessor, *methods)
      methods.each do |method|
        class_eval(<<~RUBY, __FILE__, __LINE__ + 1)
          def #{method}(...)
            #{accessor}.#{method}(...)
          end
        RUBY
      end
    end
  end

  module Coll
    extend SimpleFwd

    def to_a = children

    def_delegators :to_a, :each, :size, :map, :find, :first, :none?, :any?
  end

  class Node
    attr_reader :type, :children

    def initialize(type, children = [])
      @type = type
      @children = children
    end
  end

  class Up
    def initialize(chain) = @chain = chain

    def each_up(*types, &block)
      return to_enum(__method__, *types) unless block

      @chain.each { |x| yield x if types.empty? || types.include?(x) }
      self
    end
  end

  class Args < Node
    include Coll
  end

  class Arr2 < Node
    include Coll
  end

  class Plain
  end

  class Cop
    def each_bad(items, base)
      out = []
      prev = -1
      items.each do |current|
        out << current.type if base > prev
      end
      out
    end

    def check(node) = node.size < 2 ? [] : each_bad(node, 1)
    def types(node) = node.map(&:type)
    def top?(n) = n.each_up(:block, :def).none?
    def blocky?(n) = n.each_up(:block).any?
    def first_big(n) = n.each_up.find { |a| a.size > 3 }
    def no_opt?(node) = node.none? { |a| a.type == :optarg }
    def opt(node) = node.find { |a| a.type == :optarg }&.type
  end

  def self.demo
    args = Args.new(:args, [Node.new(:arg, [:a]), Node.new(:optarg, [:b])])
    h = { a: args, l: [Node.new(:x)], p: Plain.new, u: Up.new(%i[send block]), v: Up.new(%i[send]) } #: Hash[Symbol, untyped]
    c = Cop.new
    r = [c.check(h[:a]), c.each_bad(h[:l], 1), c.types(h[:a]), c.opt(h[:a]), c.each_bad(Arr2.new(:array, [Node.new(:int)]), 1)]
    r.push(c.top?(h[:u]), c.top?(h[:v]), c.blocky?(h[:u]), c.first_big(h[:u]), c.no_opt?(h[:a]))
    begin
      c.each_bad(h[:p], 1)
    rescue NoMethodError => e
      r << e.class
    end
    r
  end
end
