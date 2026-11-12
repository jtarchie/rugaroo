module Srcish
  class Src
    attr_reader :lines

    def initialize(text) = @lines = text.lines.map(&:chomp)

    def [](*args)
      lines[*args]
    end
  end

  class Tok
    attr_reader :line, :text

    def initialize(line, text) = (@line = line; @text = text)
  end

  class Cop
    def prev_line(src, n)
      src[0..(n - 2)].reverse.find { |line| !line.start_with?("#") }
    end

    def slice(src, a, b) = src[(a - 1)..(b - 1)]

    def one(src, i) = src[i]

    def firsts(tokens)
      Set.new(tokens.uniq(&:line).select do |token|
        token.text != "="
      end)
    end

    DIRECTIVE = /rubocop:(\w+)/

    def directives(text)
      matches = text.to_enum(:scan, DIRECTIVE).map { Regexp.last_match }
      matches.map { |m| m[1] }
    end

    def compare(a, b)
      a <=> b
    end

    def sorted(list)
      list.sort { |x, y| (x <=> y) || 0 }
    end

    def sys?(error) = error && error.ancestors[1] == SystemCallError
  end

  def self.demo
    out = []
    c = Cop.new
    s = Src.new("a\n# c\nb\n# d\ne\n")
    out.push(c.prev_line(s, 5), c.slice(s, 2, 3), c.one(s, 1), s[1, 2])
    toks = [Tok.new(1, "x"), Tok.new(1, "="), Tok.new(2, "="), Tok.new(3, "y")]
    out.push(c.firsts(toks).map(&:text))
    out.push(c.directives("# rubocop:disable X # rubocop:enable Y"))
    out.push(c.compare(StandardError, Exception), c.compare(Exception, StandardError), c.compare(IOError, IOError), c.compare(IOError, ArgumentError))
    out.push(c.sorted([StandardError, IOError, Exception]).map(&:name))
    out.push(c.sys?(Errno::ENOENT), c.sys?(IOError), c.sys?(nil))
    out
  end
end
