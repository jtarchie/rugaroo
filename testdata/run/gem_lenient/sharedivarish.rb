# frozen_string_literal: true

# rubocop's Alignment and MultilineElementIndentation, both included by FirstHashElementIndentation, both set
# @column_delta: one ivar per object, as in MRI (decision 497).
module SharedIvarAligny
  attr_reader :column_delta

  def each_bad(items)
    items.each do |i|
      @column_delta = 0 - i
      yield i if @column_delta.nonzero?
    end
  end
end

module SharedIvarElementy
  def check_first(expected, actual)
    @column_delta = expected - actual
    return if @column_delta.zero?

    yield
  end
end

class SharedIvarHashCop
  include SharedIvarAligny
  include SharedIvarElementy

  def initialize = @out = []
  attr_reader :out

  def on_hash(e, a)
    check_first(e, a) { autocorrect }
  end

  def autocorrect = @out << @column_delta
end

class SharedIvarArrayCop
  include SharedIvarAligny

  def run(xs)
    out = []
    each_bad(xs) { out << @column_delta }
    out
  end
end
