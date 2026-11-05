# rbs_inline: enabled

# A seeded dice simulation: MRI's Mersenne Twister, so every run prints the same.
class Dice
  #: (Integer, Integer) -> void
  def initialize(sides, seed)
    @sides = sides
    @rng = Random.new(seed)
  end

  #: () -> Integer
  def roll = @rng.rand(1..@sides)

  #: (Integer) -> Array[Integer]
  def histogram(n)
    counts = Array.new(@sides, 0)
    n.times do
      i = roll - 1
      counts[i] = (counts[i] || 0) + 1
    end
    counts
  end
end

d = Dice.new(6, 2024)
puts (1..10).map { d.roll }.inspect
puts Dice.new(6, 7).histogram(600).inspect

rng = Random.new(42)
puts rng.rand(100), rng.rand.round(6), rng.rand(2.5).round(6), rng.rand(10**12), rng.seed

deck = %w[A K Q J 10 9 8 7]
puts deck.shuffle(random: Random.new(1)).join(" ")
puts deck.sample(random: Random.new(3)).inspect, deck.sample(3, random: Random.new(3)).inspect
hand = deck.dup
hand.shuffle!(random: Random.new(5))
puts hand.first(4).inspect

srand(99)
first = [rand(1000), rand(1000)]
srand(99)
puts first == [rand(1000), rand(1000)], rand.between?(0.0, 1.0), rand(3..4).between?(3, 4)

begin
  rng.rand(0)
rescue ArgumentError => e
  puts "error: #{e.message}"
end
