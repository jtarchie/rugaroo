# parser's Context: accessors from a constant's Symbol list (decision 427).
class Ctx
  FLAGS = %i[in_def in_class].freeze

  attr_accessor(*FLAGS)

  def initialize
    @in_def = false
    @in_class = true
  end
end
