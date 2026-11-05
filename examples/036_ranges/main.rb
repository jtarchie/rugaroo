# rbs_inline: enabled

# Grades by score band: case/when uses Range#===.
#: (Integer) -> String
def grade(score)
  case score
  when 90..100 then "A"
  when 80...90 then "B"
  when 70...80 then "C"
  else "F"
  end
end

#: (Range[Integer]) -> Integer
def evens_in(r) = r.select(&:even?).size

[95, 85, 72, 10].each { |s| puts "#{s}: #{grade(s)}" }

r = 1..10
puts r
puts r.inspect
puts "sum=#{r.sum} size=#{r.size} evens=#{evens_in(r)}"
puts (1...10).to_a.inspect
puts r.map { |x| x * x }.first(3).inspect
puts r.include?(10), (1...10).include?(10)
puts r.first, r.last, r.min.inspect, (1...10).max.inspect
(1..10).step(4) { |i| print i, " " }
puts
(1..3).reverse_each { |i| print i }
puts

letters = "a".."e"
puts letters.to_a.join(",")
puts ("y".."ab").to_a.inspect

words = %w[zero one two three four five]
puts words[1..3].inspect
puts words[2..].inspect
puts words[-2..].inspect
puts words[1, 2].inspect
puts words.first.inspect, words.last(2).inspect

s = "hello, world"
puts s[0..4]
puts s[7..]
puts s[0...-7]
puts s[3, 2]
puts s[50..].inspect

(1..).each do |i|
  break if i > 3

  print i
end
puts
puts (1..).first(4).inspect
puts (1..3) == (1..3), (1..3) == (1...3)
