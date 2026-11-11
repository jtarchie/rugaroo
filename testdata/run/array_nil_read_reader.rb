# Decision 440: the reader's read below is a nil error no dry run saw, so the whole program recompiles without 440.
class NilReadBox
  attr_reader :xs

  def initialize
    @xs = []
    @xs << 1.5 << 2.5
  end

  def above?(x, i) = x > @xs[i]
end

b = NilReadBox.new
p b.above?(2.0, 0)
p 1.0 + b.xs[1]
