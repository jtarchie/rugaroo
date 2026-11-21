# frozen_string_literal: true

# A template method only the subclass defines (parser's `Base#initialize` calling `version`), whose answer types the
# ivar another class keeps it in.
class VerBase
  def initialize
    @lexer = VerLexer.new(version)
  end
  attr_reader :lexer
end

class VerRuby33 < VerBase
  def version
    33
  end
end

class VerLexer
  def initialize(version)
    @version = version
  end

  def new_syntax?
    @version >= 31 && @version < 40
  end
end
