# rbs_inline: enabled

# Nested modules and classes, `A::B` paths, constants, lexical lookup.

module Geometry
  VERSION = "1.2.0" #: String

  module Units
    SCALE = 100
  end

  class Error < StandardError; end

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

    # `Units` resolves lexically through Geometry.
    #: () -> Integer
    def scaled_x = x * Units::SCALE
  end

  # `Point` resolves lexically too.
  class Labeled < Point
    attr_reader :label #: String

    #: (Integer, Integer, String) -> void
    def initialize(x, y, label)
      super(x, y)
      @label = label
    end

    def to_s = "#{label}@#{super}"
  end
end

# Compact form: only Geometry::Circle is in the lexical scope here.
class Geometry::Circle < Geometry::Point
  attr_reader :r #: Integer

  #: (Integer, Integer, Integer) -> void
  def initialize(x, y, r)
    super(x, y)
    @r = r
  end

  def to_s = "circle #{super} r=#{r}"
end

module Geometry
  ORIGIN = Point.new(0, 0) #: Point
end

LIMIT = 3

labeled = Geometry::Labeled.new(1, 2, "a")
puts labeled, labeled.scaled_x
puts Geometry::VERSION, Geometry::Units::SCALE, Geometry::ORIGIN
puts Geometry::Circle.new(0, 0, 5)
puts LIMIT + 1

begin
  raise Geometry::Error
rescue Geometry::Error => e
  puts "rescued #{e.message}"
end
