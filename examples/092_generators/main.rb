# rbs_inline: enabled

# Generators, external iteration, lazy pipelines over infinite sources, and a Fiber pausing mid-block.

fib = Enumerator.new do |y|
  a, b = 0, 1
  loop do
    y << a
    a, b = b, a + b
  end
end

puts "first ten: #{fib.take(10).inspect}"
puts "even ones: #{fib.lazy.select(&:even?).first(5).inspect}"

primes = (2..Float::INFINITY).lazy.select { |n| (2..Integer.sqrt(n)).none? { |d| (n % d).zero? } }
puts "primes:    #{primes.first(8).inspect}"
puts "odd prime squares under 900: #{primes.reject(&:even?).map { |p| p * p }.take_while { |q| q < 900 }.to_a.inspect}"

words = %w[apple banana cherry]
cursor = words.each_with_index
loop do
  word, i = cursor.next
  puts "#{i + 1}. #{word} (next up: #{i + 1 < words.size ? cursor.peek.first : "nothing"})"
end

steps = (0..20) % 5
puts "#{steps.inspect} -> #{steps.to_a.inspect}"

ticker = Fiber.new do |start|
  n = start
  loop { n = Fiber.yield(n) + 1 }
end
puts "fiber: #{[ticker.resume(10), ticker.resume(20), ticker.resume(30)].inspect}"
