# rbs_inline: enabled

#: (Integer) -> Integer
def collatz_steps(n)
  steps = 0
  while n != 1
    n = n.even? ? n / 2 : 3 * n + 1
    steps += 1
  end
  steps
end

#: (Integer) -> String
def fizzbuzz(i)
  case i % 15
  when 0 then "FizzBuzz"
  when 3, 6, 9, 12 then "Fizz"
  when 5, 10 then "Buzz"
  else i.to_s
  end
end

puts collatz_steps(27)
1.upto(15) { |i| print fizzbuzz(i), " " }
puts

total = 0
10.times do |i|
  next if i.odd?
  break if i > 6

  total += i
end
puts total

i = 0
i += 1 until i * i > 50
puts i

unless total.zero?
  puts "non-zero"
else
  puts "zero"
end

x = 7
label = if x > 5 && x < 10
          "mid"
        elsif x >= 10
          "high"
        else
          "low"
        end
puts label
puts(-7 / 2, -7 % 3, 2**10)
