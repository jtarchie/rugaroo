# frozen_string_literal: true

require "autoish"

# Reads the autoloaded constants in its class body, before the entry's own require of their files.
class AutoishUser
  DOUBLE = Autoish::LAZY_WIDTH * 2
  TITLE = Autoish::LAZY_NAME.upcase

  def self.show = [DOUBLE, TITLE]
end
