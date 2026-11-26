# A program's own top-level def named as an Enumerable fold (sum, inject, reduce): its calls are the program's method,
# and the core ones on an Array still fold. `def sum(xs)` crashed the compiler (a nil owner in injectSymbol).
def sum(xs)
  t = 0
  xs.each { |x| t += x }
  t
end

def inject(a, b) = a + b

def reduce(a) = a * 2

p sum([1, 2]), sum([3, 4])
p inject(1, 2), reduce(4)
p [1, 2, 3].sum, [1, 2, 3].inject(:+), [1, 2, 3].reduce(1, :*)
