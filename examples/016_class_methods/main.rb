# rbs_inline: enabled

# Class methods, class objects as values, `singleton(C)`, `klass.new`,
# `self.class`, and virtual dispatch of class methods.

class Shape
  attr_reader :size #: Integer

  #: (Integer) -> void
  def initialize(size)
    @size = size
  end

  #: () -> String
  def self.kind = "shape"

  #: (Integer) -> bool
  def self.fits?(n) = false

  # `new` on an implicit self dispatches to the receiving class.
  #: (Integer) -> Shape
  def self.build(n) = new(n * 10)

  #: () -> String
  def describe = "#{self.class.kind} #{self.class.name}(#{size})"
end

class Square < Shape
  def self.kind = "square"

  def self.fits?(n) = n.even?
end

class Circle < Shape
  def self.fits?(n) = n.odd?
end

SHAPES = [Square, Circle] #: Array[singleton(Shape)]

#: (Integer) -> singleton(Shape)
def pick(n) = SHAPES.find { |k| k.fits?(n) } || Shape

[1, 2, 0].each do |n|
  klass = pick(n)
  shape = klass.new(n)
  puts shape.describe, klass.name, klass == Square, klass
end
puts Shape.kind, Square.kind, Circle.kind
puts Square.build(4).describe, Circle.new(3).describe
puts Shape, Square.name
