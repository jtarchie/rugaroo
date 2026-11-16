# frozen_string_literal: true

# The shapes of parser's lexer and racc's runtime (decisions 465-467): a parameter reassigned from a method that takes
# it, code points unpacked from the source, a scanner's `case` over action numbers, and a value stack cut by
# `[-n, n]`.
module Ragelish
  class Buf
    def self.recognize(string)
      return if string.empty?

      string.start_with?("#") ? Encoding::UTF_8 : nil
    end

    def self.reencode(input)
      original = input.encoding
      detected = recognize(input.force_encoding(Encoding::BINARY))
      if detected.nil?
        input.force_encoding(original)
      else
        input.force_encoding(detected).encode(Encoding::UTF_8)
      end
    end

    def initialize(source)
      @source = nil
      self.source = source
    end

    def source = @source

    def source=(input)
      input = input.dup if input.frozen?
      input = self.class.reencode(input)
      raise EncodingError, "invalid byte sequence in #{input.encoding.name}" unless input.valid_encoding?

      @source = input.gsub("\r\n", "\n").freeze
    end
  end

  class Scanner
    class << self
      attr_accessor :_actions
    end
    self._actions = [0, 14, 15, 45, 49, 7, 14, 99]

    def initialize(buf)
      @pts = buf.source.unpack("U*")
      @count = 0
      @seen = []
    end

    def run
      actions = self.class._actions
      p = 0
      while p < @pts.size
        case actions[@pts[p] % 9]
        when 14 then @count += 1
        when 15 then @count += 10
        when 45, 49 then @count += 100
        when 7 then @seen << p
        when -1 then @count -= 1
        else @count += 1000
        end
        p += 1
      end
      [@count, @seen.size, @pts.last]
    end
  end

  # racc's _racc_do_reduce: the stacks come out of one mixed Array, so each is untyped here
  class Stack
    def initialize
      @arg = [[1, 2, 3, 4, 5, 6], %w[a b c d e f], :reduce, true]
    end

    def reduce(len)
      vstack, tstack, name, flag, * = @arg
      tmp_v = vstack[-len, len]
      tmp_t = tstack[-len, len]
      vstack[-len, len] = []
      tstack[-len, len] = []
      vstack.push(tmp_v.sum) if flag
      tstack[1, 1] = %w[x y]
      [tmp_v, tmp_t, vstack, tstack, name]
    end
  end

  def self.demo
    buf = Buf.new("# café\r\nx = 1\n")
    [buf.source, buf.source.size, Scanner.new(buf).run, Stack.new.reduce(2), Stack.new.reduce(0), Stack.new.reduce(5)]
  end
end
