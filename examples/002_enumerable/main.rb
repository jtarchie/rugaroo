# rbs_inline: enabled

nums = [1, 2, 3, 4]
evens = nums.select(&:even?)
total = nums.map { |n| n * 10 }.reduce(0) { |acc, n| acc + n }
puts evens.inspect, total
