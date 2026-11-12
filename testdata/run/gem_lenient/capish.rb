module Capish
  class Src
    attr_reader :lines

    def initialize(text) = @lines = text.lines.map(&:chomp)

    def [](*args)
      lines[*args]
    end
  end

  module Caps
    ANY = Object.new
    def ANY.==(_)
      true
    end
    private_constant :ANY

    def each_capture(named: ANY)
      return enum_for(__method__, named: named) unless block_given?

      tree&.each do |c|
        yield(c) if named == c.is_a?(Symbol)
      end

      self
    end
  end

  class Rx
    include Caps

    attr_reader :tree

    def initialize(tree) = @tree = tree
  end

  class Cop
    def initialize(src) = @src = src

    def end?(src, n) = src[n] == "end"
    def after(src, n) = src[n + 1]
    def prev(src, n) = src[0..(n - 2)].reverse.find { |l| l != "" }
    def span(a, b) = @src[(a - 1)..(b - 1)]
    def one(i) = @src[i]

    def mixed?(node)
      return false if node.each_capture(named: false).none?
      return false if node.each_capture(named: true).none?

      true
    end

    def all(node) = node.each_capture.to_a.size
  end

  def self.demo
    s = Src.new("a\n\nb\nend\n")
    c = Cop.new(s)
    h = { r: Rx.new(%w[1 a 2].map { |x| x == "a" ? :a : x.to_i }), q: Rx.new(%w[1 2].map(&:to_i)), z: Rx.new(nil) } #: Hash[Symbol, untyped]
    [c.end?(s, 3), c.after(s, 1), c.prev(s, 4), c.span(3, 4), c.one(0), c.mixed?(h[:r]), c.mixed?(h[:q]), c.all(h[:r]), c.all(h[:z])]
  end
end
