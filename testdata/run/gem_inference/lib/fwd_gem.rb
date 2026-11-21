# frozen_string_literal: true

# Arguments that reach a constructor through a `*args` forwarder (parser's LexerStrings#push_literal).
class FwdLit
  def initialize(owner, kind, start, heredoc_e = nil, indent = false)
    @owner = owner
    @kind = kind
    @start = start
    @heredoc_e = heredoc_e
    @indent = indent
  end
  attr_reader :heredoc_e, :start, :kind
end

class FwdStack
  def initialize
    @stack = []
  end

  def push(*args)
    lit = FwdLit.new(self, *args)
    @stack.push(lit)
    lit
  end

  def top
    @stack.last
  end

  def jump(p)
    cur = top
    if cur && cur.heredoc_e
      p = cur.heredoc_e - 1
    end
    p + cur.start
  end
end
