# frozen_string_literal: true

# A gem parent whose method is one literal (rubocop-ast's NodePattern::Node `def arity = 1`) is a placeholder its
# overrides fill with other kinds (Rest's `0..Float::INFINITY`): callers see the join, and `is_a?(Range)` tells them apart.
class Arityish
  INT_TO_RANGE = Hash.new { |h, k| h[k] = k..k }

  def arity
    1
  end

  def arity_range
    a = arity
    a.is_a?(Range) ? a : INT_TO_RANGE[a]
  end

  class Rest < Arityish
    ARITY = (0..Float::INFINITY).freeze

    def arity
      ARITY
    end
  end

  class Pair < Arityish
    def arity
      2
    end
  end
end
