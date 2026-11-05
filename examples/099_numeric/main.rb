# rbs_inline: enabled

# Numeric (#39): one method for every kind of number; mixed operands widen up Integer < Rational < Float.

#: (Numeric, Numeric) -> Numeric
def midpoint(a, b) = (a + b) / 2

#: (untyped) -> String
def number_kind(v)
  case v
  when Integer then "Integer #{v}, #{v.even? ? "even" : "odd"}"
  when Float then "Float #{v}, rounds to #{v.round}"
  when Numeric then "#{v.class} #{v}, real? #{v.real?}"
  else "not a number: #{v.inspect}"
  end
end

#: (Numeric) -> String
def sign(n)
  if n.zero? then "zero"
  elsif n.negative? then "negative"
  else "positive"
  end
end

# The result's class follows the operands: Integer division truncates, Rational stays exact.
puts "midpoint(3, 4)               = #{midpoint(3, 4).inspect}"
puts "midpoint(3, 4.0)             = #{midpoint(3, 4.0).inspect}"
puts "midpoint(Rational(1, 3), 1)  = #{midpoint(Rational(1, 3), 1).inspect}"
puts "midpoint(Complex(1, 2), 3)   = #{midpoint(Complex(1, 2), 3).inspect}"

values = [42, 2.5, Rational(3, 4), Complex(3, 4), "7", nil] #: Array[untyped]
values.each { |v| puts number_kind(v) }
puts "numbers among them: #{values.count { |v| v.is_a?(Numeric) }}"
signed = [-2, 0.0, Rational(1, 3)] #: Array[Numeric]
puts signed.map { |n| sign(n) }.inspect

# Mixed arithmetic and comparison between classes.
i = 3
f = 0.75
r = Rational(1, 3)
puts "#{i} + #{r} = #{(i + r).inspect}, #{r} + #{f} = #{(r + f).inspect}, #{i} * #{r} = #{(i * r).inspect}"
puts "#{r} < #{f}? #{r < f}; #{i} <=> #{r}: #{i <=> r}"

# Division family: div floors, divmod pairs it with the remainder, fdiv is always a Float.
puts "7.div(2.5) = #{7.div(2.5)}, 7.5.divmod(2) = #{7.5.divmod(2).inspect}"
puts "Rational(7, 2).divmod(Rational(1, 3)) = #{Rational(7, 2).divmod(Rational(1, 3)).inspect}"
puts "3.fdiv(Rational(1, 2)) = #{3.fdiv(Rational(1, 2))}, Rational(1, 2).fdiv(2) = #{Rational(1, 2).fdiv(2)}"

# step keeps the receiver's class; clamp answers the bound itself.
puts "1.step(2, 0.25):        #{1.step(2, 0.25).to_a.inspect}"
puts "Rational(1, 2).step(2): #{Rational(1, 2).step(2).to_a.inspect}"
thirds = [] #: Array[Rational]
Rational(0, 1).step(1, Rational(1, 3)) { |x| thirds << x }
puts "thirds: #{thirds.map(&:to_s).join(" ")}"
puts "clamp: #{Rational(5, 2).clamp(1, 2).inspect}, #{0.25.clamp(Rational(1, 2), 1).inspect}, #{7.clamp(1, 9.5).inspect}"

# A mixed Numeric array sorts and aggregates by value.
readings = [3, 1.5, Rational(1, 2), 2.25, Rational(7, 3), -1] #: Array[Numeric]
puts "sorted: #{readings.sort.inspect}"
puts "min #{readings.min.inspect}, max #{readings.max.inspect}"
puts "positives: #{readings.select(&:positive?).size}, absolute values: #{readings.map(&:abs).inspect}"
puts "between 1 and 2.5: #{readings.select { |n| n.between?(1, 2.5) }.inspect}"
