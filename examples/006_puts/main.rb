# rbs_inline: enabled

class Point
  attr_reader :x #: Integer
  attr_reader :y #: Integer

  #: (Integer, Integer) -> void
  def initialize(x, y)
    @x = x
    @y = y
  end

  #: () -> String
  def to_s = "(#{x}, #{y})"
end

words = %w[a b]

puts "hello"
puts "no double newline\n"
puts
puts 42, 2.0
puts nil
puts words[5]
puts [[1, 2], [3]]
puts Point.new(1, 2)
puts "interp: #{Point.new(3, 4)}"
