# rbs_inline: enabled

class Shape
  #: () -> String
  def name = "Shape"

  #: () -> Float
  def area = raise(NotImplementedError)

  #: () -> String
  def describe = "#{name}: #{area}"
end

class Rect < Shape
  attr_reader :w #: Float
  attr_reader :h #: Float

  #: (Float, Float) -> void
  def initialize(w, h)
    @w = w
    @h = h
  end

  def name = "Rect"
  def area = w * h
end

class Square < Rect
  #: (Float) -> void
  def initialize(side) = super(side, side)

  def name = "Square"
end

shapes = [Rect.new(2.0, 3.0), Square.new(2.0)] #: Array[Shape]
shapes.each { |s| puts s.describe }
