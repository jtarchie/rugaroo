# frozen_string_literal: true

# regexp_parser's Subexpression shape: an Enumerable whose each forwards its rest and block to a container.
class Seqish
  include Enumerable

  def initialize(items)
    @items = items
  end

  def each(*args, &block)
    @items.each(*args, &block)
  end

  def to_h
    { size: @items.size }
  end
end

class SubSeqish < Seqish
  def first_two = take(2)
end
