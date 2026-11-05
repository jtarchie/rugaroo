# rbs_inline: enabled

# Exact recipe scaling: fractions never drift the way Floats do.
class Ingredient
  attr_reader :name #: String
  attr_reader :qty #: Rational

  #: (String, Rational) -> void
  def initialize(name, qty)
    @name = name
    @qty = qty
  end

  #: (Rational) -> Ingredient
  def scale(by) = Ingredient.new(name, qty * by)

  #: () -> String
  def to_s
    whole = qty.to_i
    rest = qty - whole
    return "#{whole} #{name}" if rest.zero?
    return "#{rest} #{name}" if whole.zero?

    "#{whole} #{rest} #{name}"
  end
end

recipe = [
  Ingredient.new("cup flour", 3/4r),
  Ingredient.new("tsp salt", Rational(1, 8)),
  Ingredient.new("eggs", 2r)
]
[Rational(1, 2), 3r, Rational(4, 3)].each do |k|
  puts "x#{k.inspect}: #{recipe.map { |i| i.scale(k).to_s }.join(", ")}"
end

total = recipe.map(&:qty).reduce(0r) { |a, b| a + b }
puts total, total.inspect, total.to_f
puts (0.1r + 0.2r) == 0.3r, (0.1 + 0.2) == 0.3
puts 1.quo(3) + 1.quo(6), 2 - Rational(1, 3), Rational(5, 2) * 2
puts Rational(7, 2).round, Rational(-7, 2).round, Rational(-7, 2).floor, Rational(-7, 2).ceil
puts (Rational(2, 3)**-2).inspect, 0.75.to_r, Rational(6, 4) == Rational(3, 2)
puts [3/4r, 1/2r, 5/8r].sort.inspect, (1/3r).to_f.round(3)
puts Rational(1, 3) < 1, Rational(1, 3) + 0.5

begin
  Rational(1, 0)
rescue ZeroDivisionError => e
  puts "error: #{e.message}"
end
