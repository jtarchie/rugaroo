# rbs_inline: enabled

require "minitest/autorun"
require "set"

# Value classes (decision 441); no equal?/object_id/assert_same here: one such call keeps every Data class a pointer.
module ValueTests
  Point = Data.define(
    :x, #: Integer
    :y #: Integer
  ) do
    #: (Point o) -> Point
    def +(o) = with(x: x + o.x, y: y + o.y)

    #: () -> Integer
    def norm1 = x.abs + y.abs
  end

  Named = Data.define(:name, :age) #: [String, Integer]
  Flag = Data.define(:on, :weight) #: [bool, Float]

  class Holder
    attr_reader :pt #: Point?

    #: (Point? pt) -> void
    def initialize(pt)
      @pt = pt
    end
  end

  # @value
  class Vec
    include Comparable

    attr_reader :x #: Float
    attr_reader :y #: Float

    #: (Float x, Float y) -> void
    def initialize(x, y)
      @x = x
      @y = y
    end

    #: (Vec other) -> Vec
    def +(other) = Vec.new(@x + other.x, @y + other.y)

    #: (Float k) -> Vec
    def *(k) = Vec.new(@x * k, @y * k)

    #: (Vec other) -> Integer
    def <=>(other) = [x, y] <=> [other.x, other.y]

    # @rbs override
    #: () -> String
    def to_s = "(#{x}, #{y})"

    # @rbs override
    #: () -> String
    def inspect = "#<Vec #{x}, #{y}>" # Kernel#inspect's address: a value has none (decision 441)

    #: () -> Float
    def length = Math.sqrt(@x * @x + @y * @y)
  end

  class Body
    attr_accessor :pos #: Vec
    attr_accessor :last #: Vec?

    #: (Vec pos) -> void
    def initialize(pos)
      @pos = pos
      @last = nil
    end

    #: (Vec step) -> void
    def move(step)
      @last = @pos
      @pos += step
    end
  end

  class DataValueTest < Minitest::Test
    def test_equality_and_hash
      a = Point.new(x: 4, y: 6)
      b = Point.new(1, 2) + Point.new(3, 4)
      assert_equal a, b
      assert a.eql?(b)
      assert a.hash == b.hash
      refute_equal a, Point.new(x: 6, y: 4)
      assert a.frozen?
      assert_equal({ x: 4, y: 6 }, a.to_h)
      assert_equal [:x, :y], Point.members
    end

    def test_with
      a = Point.new(x: 1, y: 2)
      assert_equal Point.new(x: 1, y: 10), a.with(y: 10)
      assert_equal a, a.with
      assert_equal Point.new(x: 1, y: 2), a # with copied, the original is unchanged
      n = Named.new("bob", 3)
      assert_equal "#<data ValueTests::Named name=\"bob\", age=4>", n.with(age: 4).inspect
    end

    def test_inspect
      assert_equal "#<data ValueTests::Point x=1, y=2>", Point.new(x: 1, y: 2).inspect
      assert_equal "#<data ValueTests::Point x=1, y=2>", "#{Point.new(x: 1, y: 2)}"
      assert_equal "[#<data ValueTests::Flag on=true, weight=1.5>]", [Flag.new(on: true, weight: 1.5)].inspect
    end

    def test_arrays
      a = Point.new(x: 1, y: 2)
      arr = [a, Point.new(x: 3, y: 4), Point.new(x: 4, y: 6), Point.new(x: 1, y: 2)]
      assert_equal 3, arr.uniq.size
      assert arr.include?(Point.new(x: 3, y: 4))
      assert_equal 2, arr.index(Point.new(x: 4, y: 6))
      assert_equal [3, 3, 7, 10], arr.sort_by(&:norm1).map(&:norm1)
      assert_equal Point.new(x: 4, y: 6), arr.max_by(&:norm1)
      assert_equal({ Point.new(x: 1, y: 2) => 2, Point.new(x: 3, y: 4) => 1, Point.new(x: 4, y: 6) => 1 }, arr.tally)
      assert_equal [1, 3, 4], arr.group_by(&:x).keys
      sum = Array.new(3) { |i| Point.new(x: i, y: i) }.reduce(Point.new(x: 0, y: 0)) { |acc, pt| acc + pt }
      assert_equal Point.new(x: 3, y: 3), sum
      arr[0] = Point.new(x: 9, y: 9) # the element is replaced, a held copy is not
      assert_equal Point.new(x: 1, y: 2), a
    end

    def test_hash_keys
      h = {} #: Hash[Point, String]
      h[Point.new(x: 1, y: 2)] = "a"
      h[Point.new(x: 3, y: 4)] = "b"
      h[Point.new(x: 1, y: 2)] = "a2"
      assert_equal 2, h.size
      assert_equal "b", h[Point.new(x: 3, y: 4)]
      assert_equal "a2", h.fetch(Point.new(x: 1, y: 2))
      assert h.key?(Point.new(x: 3, y: 4))
      assert_equal 2, Set.new([Point.new(x: 1, y: 2), Point.new(x: 1, y: 2), Point.new(x: 0, y: 0)]).size
    end

    def test_patterns
      c = Point.new(x: 4, y: 6)
      got = case c
            in { x: 4, y: } then y
            else 0
            end
      assert_equal 6, got
      got = case Point.new(x: 3, y: 4)
            in [px, py] then px * py
            else 0
            end
      assert_equal 12, got
      assert_equal [4, 6], c.deconstruct
      assert_equal({ x: 4 }, c.deconstruct_keys([:x]))
      hit = Point.new(x: 1, y: 1) in Point(x: 1)
      assert hit
    end

    def test_optional
      empty = Holder.new(nil)
      assert_nil empty.pt
      assert_nil empty.pt&.norm1
      full = Holder.new(Point.new(x: 4, y: 6))
      assert_equal 10, full.pt&.norm1
      assert_equal 4, (full.pt || Point.new(x: 0, y: 0)).x
      assert_equal 0, (empty.pt || Point.new(x: 0, y: 0)).x
      opts = [Point.new(x: 1, y: 2), nil, Point.new(x: 3, y: 4)] #: Array[Point?]
      assert_equal [Point.new(x: 1, y: 2), Point.new(x: 3, y: 4)], opts.compact
      assert_equal 1, opts.count(&:nil?)
      q = opts[1]
      assert_nil q
      q = opts[2]
      assert_equal 3, q.x if q
    end

    def test_untyped_and_reflection
      a = Point.new(x: 1, y: 2)
      u = a #: untyped
      assert_equal 1, u.x
      assert_equal 3, u.norm1
      assert_equal a, u
      assert a.is_a?(Data)
      assert Point === a # rubocop:disable Style/CaseEquality
      assert_equal Point, a.class
      assert a.respond_to?(:x)
      assert_equal a, a.dup
      assert_equal [{ x: 1, y: 2 }], [a].map(&:to_h)
    end
  end

  # A `# @value` class compared through Comparable: MRI's == and the copies' agree.
  class AnnotatedValueTest < Minitest::Test
    def test_arithmetic
      v = Vec.new(1.0, 2.0) + Vec.new(3.0, 4.0) * 2.0
      assert_equal 7.0, v.x
      assert_equal 10.0, v.y
      assert_equal "(7.0, 10.0)", v.to_s
      assert_equal "(7.0, 10.0)", "#{v}"
      assert_in_delta 12.2066, v.length, 0.0001
      assert_equal Vec.new(7.0, 10.0), v
      assert Vec.new(1.0, 1.0) < v
      refute v.frozen?
    end

    def test_owner_holds_a_copy
      b = Body.new(Vec.new(0.0, 0.0))
      assert_nil b.last
      start = b.pos
      b.move(Vec.new(1.0, 2.0))
      b.move(Vec.new(1.0, 2.0))
      assert_equal Vec.new(2.0, 4.0), b.pos
      assert_equal Vec.new(1.0, 2.0), b.last
      assert_equal Vec.new(0.0, 0.0), start
      assert_equal 1.0, b.last.then { |l| l ? l.x : 0.0 }
      b.pos = Vec.new(9.0, 9.0)
      assert_equal 9.0, b.pos.x
    end

    def test_containers
      vs = [Vec.new(3.0, 1.0), Vec.new(1.0, 2.0), Vec.new(2.0, 0.0)]
      assert_equal [1.0, 2.0, 3.0], vs.sort.map(&:x)
      assert_equal Vec.new(3.0, 1.0), vs.max
      assert_equal Vec.new(6.0, 3.0), vs.reduce(Vec.new(0.0, 0.0)) { |acc, v| acc + v }
      by_name = { "a" => vs[0] } #: Hash[String, Vec]
      assert_equal 3.0, by_name.fetch("a").x
      opt = [vs[1], nil] #: Array[Vec?]
      assert_equal [Vec.new(1.0, 2.0)], opt.compact
      u = vs[2] #: untyped
      assert_equal 2.0, u.x
    end
  end
end
