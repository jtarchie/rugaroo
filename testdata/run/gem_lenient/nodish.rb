require "forwardable"
module Nodish
  # A: a module overrides a private method of the module it includes
  module Styled
    def style
      @style ||= config[style_parameter_name].to_sym
    end

    private

    def style_parameter_name
      'EnforcedStyle'
    end
  end

  module EndAlign
    include Styled

    def check(x)
      "#{style}:#{x}"
    end

    private

    def style_parameter_name
      'EnforcedStyleAlignWith'
    end
  end

  class CopA
    include EndAlign

    def config = { 'EnforcedStyle' => 'plain', 'EnforcedStyleAlignWith' => 'keyword' }
  end

  class CopB
    include Styled

    def config = { 'EnforcedStyle' => 'plain' }
  end

  # B: String#match with a position, \G
  class Tok
    def initialize(src, pos) = (@src = src; @pos = pos)

    def space_after?
      @src.match(/\G\s/, @pos)
    end
  end

  # C/E/F/H: a node with delegated Array methods and enumerators
  class Node
    extend Forwardable
    include Enumerable

    attr_reader :type, :children, :parent

    def initialize(type, children = [])
      @type = type
      @children = children
      @parent = nil
      children.each { |c| c.parent = self if c.is_a?(Node) }
    end

    def parent=(p)
      @parent = p
    end

    def to_a = children

    def_delegators :to_a, :each, :first, :size, :empty?, :map

    def child_nodes = children.grep(Node)

    def each_child_node(*types)
      return to_enum(__method__, *types) unless block_given?

      children.each do |c|
        next unless c.is_a?(Node)

        yield c if types.empty? || types.include?(c.type)
      end
      self
    end

    def each_ancestor(*types, &block)
      return to_enum(__method__, *types) unless block

      visit_ancestors(types, &block)
      self
    end

    def visit_ancestors(types)
      last = self
      while (cur = last.parent)
        yield cur if types.empty? || types.include?(cur.type)
        last = cur
      end
    end

    def each_pair
      return each_child_node(:pair).to_enum unless block_given?

      each_child_node(:pair) do |pair|
        yield(*pair)
      end
      self
    end

    def pairs = each_pair.to_a

    def splat? = type == :splat
    def mlhs? = type == :mlhs

    def assignments
      child_nodes.flat_map do |node|
        if node.splat?
          node.child_nodes.first || node
        elsif node.mlhs?
          node.assignments
        else
          node
        end
      end
    end
  end

  class CopC
    def names(node)
      out = []
      node.each { |arg| out << arg.type }
      out
    end

    def in_block?(node) = node.each_ancestor(:block).any?
    def top?(node) = node.each_ancestor.none?

    def pair_types(hash)
      out = []
      r = hash.each_pair { |k, v| out << [k.type, v.type] }
      [out, r.type]
    end

    def newline?(s) = s.match?(/\R/)

    def first_def(node) = node.each_ancestor.find { |a| a.type == :def }&.type
  end
  def self.demo
    out = []
    out.push(CopA.new.check(1), CopB.new.style)
    t = Tok.new("ab cd", 2)
    out.push(t.space_after?.to_s, Tok.new("ab cd", 1).space_after?)
    n_ = nil
    leaf = Node.new(:lvasgn, [:x])
    args = Node.new(:args, [Node.new(:arg, [:a]), Node.new(:optarg, [:b])])
    pair = Node.new(:pair, [Node.new(:sym, [:k]), Node.new(:int, [1])])
    hash = Node.new(:hash, [pair, Node.new(:kwsplat, [])])
    mlhs = Node.new(:mlhs, [leaf, Node.new(:splat, [Node.new(:ivasgn, [:@y])]), Node.new(:mlhs, [Node.new(:gvasgn, [:$z])]), Node.new(:splat, [])])
    root = Node.new(:def, [args, Node.new(:block, [hash, mlhs])])
    c = CopC.new
    out.push(c.names(args), c.in_block?(hash), c.in_block?(args), c.top?(root), c.top?(args))
    out.push(hash.pairs.map(&:type), c.pair_types(hash))
    out.push(mlhs.assignments.map(&:type))
    out.push(c.newline?("a\r\nb"), c.newline?("ab"), c.first_def(pair), c.first_def(root))
    out
  end
end
