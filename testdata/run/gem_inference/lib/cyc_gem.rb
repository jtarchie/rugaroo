# frozen_string_literal: true

# A scanner position that flows through helper methods and back, as parser's lexer passes `p`.
class CycStrings
  def initialize
    @herebody_s = nil
    @depth = 0
  end

  def on_newline(p)
    if @herebody_s
      p = @herebody_s
      @herebody_s = nil
    end
    p
  end

  def advance(p)
    @depth += 1
    @herebody_s = p + 2 if (@depth % 7).zero?
    [p + 1, (@depth % 3).zero? ? 5 : nil]
  end
end

class CycLexer
  def initialize
    @strings = CycStrings.new
    @p = 0
    @cs = 1
    @total = 0
    @te = nil
    @emit_plain = lambda { |chars, p| @total += chars.size; p }
    @emit_skip = lambda { |chars, p| @total += chars.size; p - 2 }
    @num_xfrm = nil
  end

  def on_newline(p)
    @strings.on_newline(p)
  end

  def advance(limit)
    pe = limit
    p, eof = @p, pe
    while p < eof
      if (p % 5).zero?
        p = on_newline(p)
      elsif (p % 11).zero?
        p, next_state = @strings.advance(p)
        @cs = next_state if next_state
      end
      if (p % 13).zero?
        @te = p + 3
        @num_xfrm = (p % 2).zero? ? @emit_plain : @emit_skip
        p = @num_xfrm.call("12", p)
        p = @te - 1
      end
      @total += @cs
      p += 1
    end
    @p = p
    @total
  end
end
