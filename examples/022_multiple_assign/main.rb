# rbs_inline: enabled

# Multiple assignment from tuples and array literals.

#: (Integer, Integer) -> [Integer, Integer]
def divmod2(a, b) = [a / b, a % b]

q, r = divmod2(17, 5)
puts q, r
status, headers, body = [200, { "A" => "b" }, "ok"]
puts status, headers.inspect, body
x = 1
y = 2
x, y = y, x
puts x, y
