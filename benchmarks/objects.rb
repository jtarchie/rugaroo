# rbs_inline: enabled

# Classes, inheritance, polymorphic calls, attr_accessor and Comparable: an n-body step plus a shape mix.

# A 2D vector is a value: rugaroo holds it as a Go struct, not a pointer (decision 441).
# @value
class Vec
  attr_reader :x #: Float
  attr_reader :y #: Float

  #: (Float x, Float y) -> void
  def initialize(x, y)
    @x = x
    @y = y
  end

  #: (Vec other) -> Vec
  def +(other) = Vec.new(@x + other.x, @y + other.y)

  #: (Vec other) -> Vec
  def -(other) = Vec.new(@x - other.x, @y - other.y)

  #: (Float k) -> Vec
  def *(k) = Vec.new(@x * k, @y * k)

  #: () -> Float
  def length = Math.sqrt(@x * @x + @y * @y)
end

class Body
  attr_accessor :pos #: Vec
  attr_accessor :vel #: Vec
  attr_reader :mass #: Float

  #: (Vec pos, Vec vel, Float mass) -> void
  def initialize(pos, vel, mass)
    @pos = pos
    @vel = vel
    @mass = mass
  end
end

class Shape
  include Comparable

  #: () -> Float
  def area = 0.0

  #: (Shape other) -> Integer
  def <=>(other) = (area <=> other.area) || 0
end

class Circle < Shape
  #: (Float r) -> void
  def initialize(r)
    @r = r
  end

  # @rbs override
  #: () -> Float
  def area = 3.14159 * @r * @r
end

class Rect < Shape
  #: (Float w, Float h) -> void
  def initialize(w, h)
    @w = w
    @h = h
  end

  # @rbs override
  #: () -> Float
  def area = @w * @h
end

class Square < Rect
  #: (Float s) -> void
  def initialize(s)
    super(s, s)
  end
end

bodies = (0...40).map { |i| Body.new(Vec.new(i.to_f, (i * 3 % 7).to_f), Vec.new(0.0, 0.0), 1.0 + (i % 3)) }
60.times do
  bodies.each do |a|
    acc = Vec.new(0.0, 0.0)
    bodies.each do |b|
      next if a.equal?(b)

      d = b.pos - a.pos
      dist = d.length + 0.01
      acc += d * (b.mass / (dist * dist * dist))
    end
    a.vel += acc * 0.001
  end
  bodies.each { |b| b.pos += b.vel }
end
puts bodies.sum { |b| b.pos.length }.round(6)

shapes = [] #: Array[Shape]
200_000.times do |i|
  shapes << case i % 3
            when 0 then Circle.new((i % 10).to_f)
            when 1 then Rect.new((i % 7).to_f, (i % 5).to_f)
            else Square.new((i % 4).to_f)
            end
end
puts shapes.sum(&:area).round(2)
puts shapes.max&.area
puts shapes.count { |s| s > shapes.fetch(1) }
