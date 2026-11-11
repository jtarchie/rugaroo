# rbs_inline: enabled

# Small immutable objects as Go value structs, no heap allocation per `new` (decision 441): Data.define automatically, `# @value` by assertion.

Cell = Data.define(
  :row, #: Integer
  :col  #: Integer
) do
  #: (Cell other) -> Cell
  def +(other) = with(row: row + other.row, col: col + other.col)

  #: () -> Array[Cell]
  def neighbors = [Cell.new(-1, 0), Cell.new(1, 0), Cell.new(0, -1), Cell.new(0, 1)].map { |d| self + d }
end

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

  #: (Float k) -> Vec
  def *(k) = Vec.new(@x * k, @y * k)

  #: () -> Float
  def length = Math.sqrt(@x * @x + @y * @y)

  # @rbs override
  #: () -> String
  def to_s = format("(%.3f, %.3f)", @x, @y)
end

# Breadth-first fill of a grid: Cells are Hash keys and Array elements, compared by their members.
walls = [Cell.new(1, 1), Cell.new(1, 2), Cell.new(2, 1)]
dist = { Cell.new(0, 0) => 0 } #: Hash[Cell, Integer]
queue = [Cell.new(0, 0)]
while (here = queue.shift)
  here.neighbors.each do |n|
    next if n.row.negative? || n.col.negative? || n.row > 4 || n.col > 4
    next if walls.include?(n) || dist.key?(n)

    dist[n] = dist.fetch(here) + 1
    queue << n
  end
end
puts "reached #{dist.size} cells; (4, 4) is #{dist[Cell.new(4, 4)]} steps away"
p dist.max_by { |_, d| d }&.first
p Cell.new(2, 3).with(col: 0), Cell.new(2, 3) == Cell.new(2, 3)

# A projectile under gravity: every step builds new Vecs, none of them on the heap.
pos = Vec.new(0.0, 0.0)
vel = Vec.new(3.0, 12.0)
gravity = Vec.new(0.0, -9.81)
dt = 0.001
steps = 0
while pos.y >= 0.0
  vel += gravity * dt
  pos += vel * dt
  steps += 1
end
puts "landed after #{steps} steps at #{pos}, speed #{format('%.3f', vel.length)}"
