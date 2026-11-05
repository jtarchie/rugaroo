# rbs_inline: enabled

puts "  units/temperature loaded"

class Temperature
  attr_reader :celsius #: Float

  #: (Float) -> void
  def initialize(celsius)
    @celsius = celsius
  end

  #: () -> Float
  def fahrenheit = (celsius * 9 / 5) + 32

  #: () -> String
  def to_s = format("%.1fC", celsius)
end
