# frozen_string_literal: true

# A dynamic call whose arguments include a splat (the parser gem's `@builder.loop(:while, val[0], *val[1], val[2])`):
# the parameters from the splat on take what nobody can count, so the calls without one do not type them.
class SplatBuilder
  def pair(kind, left, right)
    "#{kind}:#{left.inspect}:#{right.inspect}"
  end
end

class SplatParser
  def initialize(builder)
    @builder = builder
  end

  def plain(val)
    @builder.pair(:plain, val[0], val[0])
  end

  def spread(val)
    @builder.pair(:spread, *val[1])
  end
end

class SplatHost
  def self.builders
    [SplatBuilder.new, nil]
  end

  def self.run
    b = builders.compact.first
    pa = SplatParser.new(b)
    [pa.plain(stack), pa.spread(stack)]
  end

  # a value stack as racc keeps one: whatever the reductions pushed
  def self.stack
    vals = []
    vals.push("tok")
    vals.push([[1, 2], :sym])
    vals
  end
end
