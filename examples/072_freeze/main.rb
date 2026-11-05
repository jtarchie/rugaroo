# rbs_inline: enabled

# Frozen constants: Array#freeze and Hash#freeze make later mutation a FrozenError.

PRIMES = [2, 3, 5, 7].freeze
LIMITS = { small: 10, large: 100 }.freeze

puts "primes: #{PRIMES.inspect} (frozen: #{PRIMES.frozen?})"
puts "doubled: #{PRIMES.map { |n| n * 2 }.inspect}"

begin
  PRIMES << 11
rescue FrozenError => e
  puts "refused: #{e.message}"
end

more = PRIMES.dup
more << 11
puts "a copy can grow: #{more.inspect} (frozen: #{more.frozen?})"

begin
  LIMITS[:huge] = 1000
rescue FrozenError => e
  puts "refused: #{e.message}"
end
puts "merged copy: #{LIMITS.merge({ huge: 1000 }).inspect}"
