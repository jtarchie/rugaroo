module Sexpish
  class Node
    attr_reader :type, :children

    def initialize(type, children = []) = (@type = type; @children = children)

    def to_sexp(indent = 0)
      indented = "  " * indent
      sexp = "#{indented}(#{type}"
      children.each do |child|
        if child.is_a?(Node)
          sexp += "\n#{child.to_sexp(indent + 1)}"
        else
          sexp += " #{child.inspect}"
        end
      end
      sexp += ")"
      sexp
    end

    alias to_s to_sexp

    def inspect(indent = 0)
      "s(:#{type})"
    end
  end

  class Arg < Node
  end

  class Cop
    def name_of(arg)
      name_child = arg.children.first
      return if name_child.nil?

      full = name_child.to_s
      [full, full.match?(/[[:upper:]]/), "#{name_child}", name_child.inspect]
    end
  end

  def self.demo
    a = Arg.new(:arg, [:lhs])
    m = Node.new(:mlhs, [a, Arg.new(:arg, [:op])])
    h = { a: a, m: m } #: Hash[Symbol, untyped]
    c = Cop.new
    [c.name_of(h[:a]), c.name_of(h[:m]), a.to_s, "#{m}", [a].join(","), h[:m].to_s]
  end
end
