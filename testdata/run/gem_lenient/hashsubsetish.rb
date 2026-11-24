# frozen_string_literal: true

# rubocop's OptionsValidator: `@options > { cache: 'false' }` on its untyped options Hash, a superset test.
class HashSubsetish
  def initialize(options) = @options = options
  def invalid = @options > { cache: "false" }
end
