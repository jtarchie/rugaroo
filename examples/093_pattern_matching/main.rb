# rbs_inline: enabled

# Pattern matching (#38): case/in with guards, `=>` and `in`, over untyped values, typed Arrays and Hashes, Data, and a class with its own deconstruct_keys.

Point = Data.define(:x, :y) #: [Integer, Integer]

class Temperature
  attr_reader :degrees #: Integer
  attr_reader :unit #: Symbol

  #: (Integer, Symbol) -> void
  def initialize(degrees, unit)
    @degrees = degrees
    @unit = unit
  end

  #: (Array[Symbol]?) -> Hash[Symbol, untyped]
  def deconstruct_keys(_keys) = { degrees: degrees, unit: unit }
end

#: (untyped) -> String
def classify(v)
  case v
  in Integer => n if n > 100 then "big #{n}"
  in Integer | Float => n then "number #{n}"
  in Point(x: 0, y:) then "on the y axis at #{y}"
  in Point(x:, y:) then "point #{x}/#{y}"
  in Temperature(unit: :c, degrees: ..0) then "freezing"
  in Temperature(degrees:, unit:) then "#{degrees}°#{unit.to_s.upcase}"
  in [] then "empty list"
  in [x, y] then "pair #{x},#{y}"
  in [Integer => first, *rest] then "list from #{first}, #{rest.size} more"
  in { name: String => name, age: Integer => age } if age >= 18 then "#{name} is an adult"
  in { name: String => name } then "#{name}, age unknown"
  in nil then "nothing"
  else "other: #{v.inspect}"
  end
end

[500, 7, 2.5, [], [1, 2], [3, 4, 5], { name: "Ann", age: 30 }, { name: "Bo" },
 Point.new(0, 5), Point.new(x: 1, y: 2), Temperature.new(-3, :c),
 Temperature.new(20, :c), nil, "text"].each { |v| puts classify(v) }

# typed values bind typed locals: a and b are Integers here
scores = [90, 72, 85] #: Array[Integer]
case scores
in [a, b, *]
  puts "first two sum to #{a + b}"
end
case scores
in [*, 72 => low, *after]
  puts "found #{low}, then #{after.inspect}"
end

# rightward assignment destructures, and raises when the shape is wrong
config = { host: "localhost", port: 8080 } #: Hash[Symbol, untyped]
config => { host: String => host, port: }
puts "#{host}:#{port}"
begin
  config => { user: }
rescue NoMatchingPatternKeyError => e
  puts "#{e.class}: #{e.message} (key #{e.key.inspect})"
end
begin
  [1, "x"] => [Integer, Integer]
rescue NoMatchingPatternError => e
  puts "#{e.class}: #{e.message}"
end

# `in` is a boolean test
puts(({ ok: true } in { ok: true }))
puts((Point.new(1, 2) in [_, 3]))
