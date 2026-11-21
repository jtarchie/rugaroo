# frozen_string_literal: true

# Ragel's state constants: class-level accessors set in the class body and read as `self.class.lex_en_x` by the
# instance, here and from a second class that holds the first (parser's Lexer and LexerStrings).
class LexEn
  class << self
    attr_accessor :lex_en_a
    attr_accessor :lex_en_b
  end
  self.lex_en_a = 229;
  self.lex_en_b = 508;

  def initialize
    @cs = self.class.lex_en_a
    @user = LexUser.new(self)
  end

  def pick(flag)
    if flag
      self.class.lex_en_b
    else
      self.class.lex_en_a
    end
  end

  def step(n)
    @cs = pick(n.odd?)
    p, state = @user.both(n)
    @cs + p + state
  end
end

class LexUser
  def initialize(lexer)
    @lexer = lexer
    @root = @lexer.class.lex_en_a
  end

  def both(p)
    [p, @root]
  end
end
