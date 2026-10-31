# frozen_string_literal: true

# A gem's module-body require_relative loads where it is (the parser gem's `module Parser; require_relative
# 'parser/meta'`): the body's next statement reads what it defined.
module Metaish
  require_relative "metaish/types"
  require "set"

  TYPE_COUNT = Types::NAMES.size
  TYPE_SET = Set.new(Types::NAMES)
end
