# rbs_inline: enabled

# Everyday Ruby semantics that untyped-looking code relies on: `rescue`
# modifiers, `return` inside `ensure`, NoMethodError on nil, `&&`/`||`
# returning values (and short-circuiting), locals first set in a branch,
# narrowing attribute reads, subclass-only methods, truthiness of untyped.

values = [1, 2]
missing_value = values.fetch(5) rescue -1
present_value = values.fetch(1) rescue -1
puts missing_value, present_value

#: () -> String
def swallow
  raise ArgumentError, "lost"
ensure
  return "ensure wins"
end

#: (bool) -> String
def maybe(flag)
  begin
    raise ArgumentError, "kept" if flag

    "body"
  ensure
    return "early" unless flag
  end
end

puts swallow, maybe(false)
begin
  maybe(true)
rescue ArgumentError => e
  puts "propagated #{e.message}"
end

match = "abc".match(/z/)
begin
  match[0]
rescue NoMethodError => e
  puts e.message
end

log = []
none = nil #: Integer?
some = 5 #: Integer?
puts (none && none + 1).inspect, (some && some + 1).inspect
skipped = "a" == "b" && log.push("never").size > 0
puts skipped, log.size
missing = nil #: String?
puts (missing || 42).inspect, (true && "yes").inspect, (false || "fallback").inspect

if values.size > 1
  amount = "many"
else
  amount = "few"
end
puts amount

class Shape
  attr_reader :label #: String?

  #: (String?) -> void
  def initialize(label)
    @label = label
  end

  #: () -> String
  def describe
    return "unlabeled" unless label

    "shape #{label.upcase} (#{area})"
  end
end

class Square < Shape
  #: () -> Integer
  def area = 4
end

puts Square.new("sq").describe, Shape.new(nil).describe
begin
  Shape.new("bare").describe
rescue NameError => e
  puts "#{e.class}: #{e.message}"
end

flag = nil #: untyped
puts(flag ? "yes" : "no")
mixed = [1, nil, "x", false]
puts mixed.select { |v| v }.inspect
