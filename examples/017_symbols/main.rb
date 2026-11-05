# rbs_inline: enabled

# Symbols, and braceless hash arguments (`f(a: 1)` passes a Hash).

config = { port: 8080, host: "localhost" }
puts config[:port], config[:host], config.keys.inspect
puts :abc.inspect, :abc.to_s, :a == :a, :a == :b

#: (String, ?Hash[Symbol, String]) -> String
def tag(name, attrs = {})
  rendered = attrs.map { |key, value| " #{key}=\"#{value}\"" }.join
  "<#{name}#{rendered}>"
end

#: (Hash[String, Integer]) -> Integer
def total(counts) = counts.values.reduce(0) { |sum, n| sum + n }

puts tag("br"), tag("a", href: "/x", id: "y")
puts total("a" => 1, "b" => 2)
puts({ port: 1, "a b": 2, :+ => 3, ok?: 4, "s" => 5, "set=": 6 }.inspect)
