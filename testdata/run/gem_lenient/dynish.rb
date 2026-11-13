require "forwardable"
module Dynish
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

  class CopD
    ANY = Object.new
    def ANY.==(_)
      true
    end
    private_constant :ANY

    def names(node)
      out = []
      node.arguments.each { |arg| out << arg.type }
      out
    end

    def any_block?(node) = node.each_ancestor(:block).any?
    def none_def?(node) = node.each_ancestor(:def).none?
    def first_def(node) = node.each_ancestor.find { |a| a.type == :def }&.type
    def firsts(tokens) = Set.new(tokens.select { |t| t.size > 1 })
    def slice(src, n) = src[0..(n - 2)].reverse.find { |l| l != "" }
    def set_any?(set) = set.any?
    def none_big?(list) = list.none? { |x| x.size > 5 }
    def any_big?(list) = list.any? { |x| x.size > 1 }
    def find_plain(list) = list.find { |x| x.size > 1 }
    def captures(node, named: ANY) = node.children.select { |c| named == c.is_a?(Node) }.size
    def sorted?(rows) = rows == rows.sort { |a, b| (a <=> b) || 0 }
  end

  class Def
    def initialize(args) = @args = args
    def arguments = @args
  end

  class Lines
    def initialize(l) = @l = l
    def [](*args) = @l[*args]
  end

  def self.demo
    out = []
    args = Node.new(:args, [Node.new(:arg, [:a]), Node.new(:optarg, [:b])])
    blk = Node.new(:block, [args, 1])
    root = Node.new(:def, [blk])
    h = { d: Def.new(args), n: args, s: Set[1, 2], e: Set[], l: Lines.new(["a", "", "b", "c"]), t: %w[ab c de], c: CopD.new, r: [[IOError, StandardError], [StandardError, IOError]] } #: Hash[Symbol, untyped]
    c = CopD.new
    out.push(c.names(h[:d]), c.any_block?(h[:n]), c.none_def?(h[:n]), c.first_def(h[:n]), c.firsts(h[:t]).to_a, c.slice(h[:l], 4))
    out.push(c.set_any?(h[:s]), c.set_any?(h[:e]), h[:c].captures(h[:n]), h[:c].captures(h[:n], named: true), h[:c].captures(h[:n], named: false), h[:c].captures(blk, named: false))
    out.push(c.sorted?(h[:r]), root.type, c.none_big?(h[:t]), c.any_big?(h[:t]), c.find_plain(h[:t]))
    out
  end
end
