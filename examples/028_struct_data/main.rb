# rbs_inline: enabled

# Struct.new and Data.define classes. Member types come from a `#: T` after
# each member (rbs-inline's form), or from a `#: [A, B]` after the whole
# definition. Nilable Struct members may be left out of `new`; every Data
# member is required, and Data values are immutable (`with` copies).

Point = Struct.new(
  :x, #: Integer
  :y  #: Integer
)

Resource = Struct.new(:id, :label) do
  #: () -> String
  def describe = "resource #{id.inspect} (#{label})"

  #: (String?) -> Resource
  def self.build(id) = new(id, "built")
end #: [String?, String?]

class Labeled < Resource
  def describe = "labeled: #{super}"
end

Coord = Data.define(
  :lat, #: Float
  :lng  #: Float
) do
  #: () -> String
  def label = "#{lat},#{lng}"
end

origin = Point.new(0, 0)
point = Point.new(3, 4)
point.x = 5
puts point.x, point.y, point.inspect, point, point == Point.new(5, 4), point == origin
puts point.to_a.inspect, point.members.inspect, Point.members.inspect
puts Point.new(y: 7, x: 1).inspect, point.to_h.inspect

resource = Resource.new("1", "one")
puts resource.describe, Resource.build(nil).describe, Labeled.new("2", "two").describe
puts resource.inspect, Labeled.new(nil, "x").inspect, Resource.new.inspect
puts Resource.new(nil).label.inspect, Resource.new(label: "kw").inspect

here = Coord.new(lat: 1.5, lng: 2.0)
moved = here.with(lng: 3.25)
puts here.inspect, moved.inspect, here.label, moved.label
puts here == Coord.new(1.5, 2.0), here == moved, moved.to_h.inspect, Coord.members.inspect
