# frozen_string_literal: true

# `fetch(key, default)` with a default of another class than the Hash's values (rubocop's
# `REVERSE_COMPARISON.fetch(operator.to_s, operator)`): the default is answered as it is.
module Fetchish
  REVERSE = { "<" => ">", "<=" => ">=" }.freeze

  def self.reverse(operator)
    REVERSE.fetch(operator.to_s, operator)
  end

  def self.sized(name)
    { "a" => 1 }.fetch(name, :none)
  end
end
