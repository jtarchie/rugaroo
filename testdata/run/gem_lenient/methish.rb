# frozen_string_literal: true

# A Method object kept in an ivar and called with a block, as parser's TreeRewriter keeps `method(:enforce_policy)`
# and its actions call `@enforcer.call(event) do ... end` (decision 444).
module Methish
  class Rewriter
    def initialize(policy)
      @policy = policy
      @enforcer = method(:enforce)
      @seen = []
    end

    attr_reader :seen, :enforcer

    def enforce(event)
      return :accepted if @policy[event] == :accept
      return :nothing unless (values = yield)

      @seen << [event, values]
      values.size
    end
  end

  class Action
    def initialize(enforcer) = @enforcer = enforcer

    def merge(a, b)
      @enforcer.call(:different) do
        { range: a, other: b } if a != b
      end
    end

    def swallow = @enforcer.call(:swallowed) { raise "not reached" }
  end

  # rubocop's Team#investigate_fragments: a memo of accumulators and a slot, filled by another method
  class Collector
    def collect(items)
      offenses, errors, corrector = items.each_with_object([[], [], nil]) { |item, data| one(item, data) }
      [offenses, errors, corrector]
    end

    def one(item, data)
      offenses, errors, corrector = data
      offenses << item
      errors << item.to_s if item.odd?
      corrector ||= Rewriter.new({})
      data[2] = corrector
    end
  end

  def self.demo
    rw = Rewriter.new({ swallowed: :accept })
    action = Action.new(rw.enforcer)
    got = Collector.new.collect([1, 2, 3])
    [action.merge(1, 2), action.merge(3, 3), action.swallow, rw.seen, got[0], got[1], got[2].class.name]
  end
end
