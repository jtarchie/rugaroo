# rbs_inline: enabled

# Kernel#catch / #throw: leave several loops at once, carrying a value out.

MAZE = [
  "#########",
  "#S..#...#",
  "#.#.#.#.#",
  "#.#...#E#",
  "#########"
]

#: (String) -> [Integer, Integer]?
def locate(ch)
  catch(:found) do
    MAZE.each_with_index do |row, r|
      row.each_char.with_index do |c, col|
        throw :found, [r, col] if c == ch
      end
    end
    nil
  end
end

#: ([Integer, Integer]) -> Integer
def check_open(pos)
  r, c = pos
  throw :blocked, "wall at #{r},#{c}" if MAZE.fetch(r)[c] == "#"
  r * 10 + c
end

p locate("S")
p locate("E")
p locate("X")

cells = [[1, 2], [0, 0], [3, 7]] #: Array[[Integer, Integer]]
cells.each do |pos|
  outcome = catch(:blocked) { "cell #{check_open(pos)}" }
  puts outcome
end

steps = [] #: Array[String]
result = catch(:stop) do
  (1..3).each do |i|
    (1..3).each do |j|
      steps << "#{i}#{j}"
      throw :stop, i * j if i * j >= 4
    end
  end
  0
ensure
  steps << "done"
end
p result
puts steps.join(" ")

begin
  throw :nowhere
rescue UncaughtThrowError => e
  puts "#{e.class}: #{e.message}"
end
