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

  def self.demo
    rw = Rewriter.new({ swallowed: :accept })
    action = Action.new(rw.enforcer)
    [action.merge(1, 2), action.merge(3, 3), action.swallow, rw.seen]
  end
end
