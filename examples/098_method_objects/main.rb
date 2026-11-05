# rbs_inline: enabled

# Method and UnboundMethod (#40): methods as values for blocks, dispatch tables, reflection, currying and rebinding.

#: (String) -> String
def slugify(s) = s.downcase.strip.gsub(/[^a-z0-9]+/, "-")

#: (Integer, Integer) -> Integer
def add(a, b) = a + b

#: (Integer, Integer) -> Integer
def mul(a, b) = a * b

#: (Integer, Integer) -> Integer
def power(a, b) = a**b

#: (Integer, ?Integer) -> Integer
def scale(n, factor = 10) = n * factor

class Thermostat
  attr_reader :log #: Array[String]

  #: () -> void
  def initialize
    @log = [] #: Array[String]
  end

  #: (Integer) -> void
  def record(reading)
    @log << (reading > 25 ? "#{reading} hot" : "#{reading} ok")
  end

  #: () -> String
  def status = "#{@log.size} readings"
end

class SmartThermostat < Thermostat
  #: () -> String
  def status = "smart: #{super}"
end

# &method(:name) passes a method where a block is expected.
puts ["Hello World", "  Ruby & Go  "].map(&method(:slugify)).inspect
t = Thermostat.new
[21, 30, 18].each(&t.method(:record))
puts t.log.inspect

# A dispatch table: operator name to Method.
ops = { "+" => method(:add), "*" => method(:mul), "^" => method(:power) }
[["+", 2, 3], ["*", 4, 5], ["^", 2, 10], ["%", 7, 2]].each do |op, a, b|
  m = ops[op]
  puts m ? "#{a} #{op} #{b} = #{m.call(a, b)}" : "#{a} #{op} #{b}: unknown operator"
end

# Reflection comes from the def.
[method(:add), method(:scale), method(:slugify), t.method(:status)].each do |m|
  puts "#{m.name}: arity #{m.arity}, parameters #{m.parameters.inspect}, owner #{m.owner}"
end
puts "scale(4) = #{method(:scale).call(4)}"

# to_proc gives a lambda; curry fixes arguments one at a time.
adder = method(:add).to_proc
puts "lambda? #{adder.lambda?}, arity #{adder.arity}, 2 + 40 = #{adder.call(2, 40)}"
plus_ten = method(:add).curry[10]
times_three = method(:mul).curry[3]
puts [1, 2, 3].map(&plus_ten).map(&times_three).inspect

# instance_method gives an UnboundMethod; binding picks the receiver, and the definition stays the one asked for.
base_status = Thermostat.instance_method(:status)
smart = SmartThermostat.new
smart.record(27)
puts "own:  #{smart.status}"
puts "base: #{base_status.bind_call(smart)}"
upcase = String.instance_method(:upcase)
puts %w[ruby go].map { |s| upcase.bind_call(s) }.inspect
puts "same method? #{method(:add) == method(:add)}, #{method(:add) == method(:mul)}"
