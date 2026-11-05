# rbs_inline: enabled

# Syntax and core methods from ruby/spec that rugaroo now compiles (#49):
# a subject-less case, begin/else, begin/end while, alias, hash shorthand,
# **splat in a hash literal, loop, and a few Integer/Symbol/Enumerable methods.

class Thermostat
  attr_reader :reading #: Integer

  #: (Integer) -> void
  def initialize(reading)
    @reading = reading
  end

  #: () -> String
  def describe
    case
    when reading > 30, reading < -10 then "extreme"
    when reading > 20 then "warm"
    else "mild"
    end
  end
  alias label describe
end

#: (String) -> Integer?
def parse_reading(text)
  value = Integer(text)
rescue ArgumentError
  nil
else
  value * 10
end

readings = %w[3 25 oops 40].map { |t| parse_reading(t) }
puts readings.inspect
puts readings.compact.map { |r| Thermostat.new(r / 10).label }.inspect

steps = 0
begin
  steps += 1
end while steps < 0
puts "do-while ran #{steps} time"

name = "rugaroo"
version = 87
defaults = { name: "?", debug: false }
puts({ **defaults, name:, version: }.inspect)

total = 0
loop do
  total += 7
  break if total > 20
end
puts "loop stopped at #{total}"

puts [0b1100 & 0b1010, 0b1100 | 1, 1 << 5, 1234.round(-2), 7.ceildiv(2)].inspect
puts %i[ready set go].map(&:upcase).inspect
puts [1, 2, 4, 5, 7].slice_when { |a, b| b != a + 1 }.to_a.inspect
puts [3, 1, 2].minmax_by { |x| -x }.inspect
puts "seen at line #{__LINE__} in #{__method__.inspect}"
