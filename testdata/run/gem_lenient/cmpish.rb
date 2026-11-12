module Cmpish
  class Src
    attr_reader :lines

    def initialize(text) = @lines = text.lines.map(&:chomp)

    def [](*args)
      lines[*args]
    end
  end

  class Rx
    def initialize(caps) = @caps = caps

    def each_capture(named: nil)
      return enum_for(__method__, named: named) unless block_given?

      @caps.each { |c| yield(c) if named.nil? || named == c.is_a?(Symbol) }
      self
    end
  end

  class Cop
    def prev(src, n) = src[0..(n - 2)].reverse.find { |l| l != "" }
    def span(src, a, b) = src[(a - 1)..(b - 1)]

    def vars(list)
      variables = Set[]
      list.each { |x| variables << x if x.size > 1 }
      [variables.first, list.size] if variables.any?
    end

    def mixed?(rx) = !rx.each_capture(named: false).none? && !rx.each_capture(named: true).none?
    def count(rx) = rx.each_capture.to_a.size

    def sorted?(rows)
      rows == rows.sort { |a, b| (a <=> b) || 0 }
    end

    def pair_cmp(a, b) = a <=> b
  end

  def self.demo
    h = { s: Src.new("a\n\nb\nc\n"), r: Rx.new(%w[1 a 2].map { |x| x == 'a' ? :a : x.to_i }), q: Rx.new(%w[1 2].map(&:to_i)) } #: Hash[Symbol, untyped]
    c = Cop.new
    [c.prev(h[:s], 4), c.span(h[:s], 3, 4), c.vars(%w[a bb cc]), c.vars(%w[a]), c.mixed?(h[:r]), c.mixed?(h[:q]), c.count(h[:r]),
     c.sorted?([[IOError, 1], [StandardError, 2]]), c.pair_cmp([IOError], [StandardError]), c.pair_cmp([StandardError], [IOError]), c.pair_cmp([IOError], [ArgumentError])]
  end
end
