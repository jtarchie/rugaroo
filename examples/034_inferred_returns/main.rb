# rbs_inline: enabled

class Rect
  attr_reader :w #: Integer
  attr_reader :h #: Integer

  #: (Integer, Integer) -> void
  def initialize(w, h)
    @w = w
    @h = h
  end

  def area = w * h

  def label
    return "square" if w == h

    "rect #{area}"
  end

  def big? = area > 10

  # @rbs by: Integer
  def grow(by)
    Rect.new(w + by, h + by)
  end

  # An early `return nil` joins with the Integer below: Integer?.
  def half_width
    return nil if w.odd?

    w / 2
  end

  def shout
    puts label.upcase
  end
end

class Square < Rect
  def label = "sq"
end

# @rbs xs: Array[Integer]
# @rbs scale: Integer
def widest(xs, scale = 2)
  xs.map { |x| x * scale }.max
end

# @rbs name: String
# @rbs return: String
def greet(name) = "hi #{name}"

def origin = Rect.new(0, 0)

r = Rect.new(3, 4)
puts r.area, r.label, r.big?, r.grow(1).area, r.half_width.inspect
r.shout
puts Rect.new(2, 2).label, Rect.new(2, 2).half_width.inspect
s = Square.new(1, 1) #: Rect
puts s.label
puts widest([1, 2, 3]).inspect, widest([], 5).inspect, greet("bo"), origin.area
