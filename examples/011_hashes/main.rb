# rbs_inline: enabled

ages = { "alice" => 30, "bob" => 25 }
ages["carol"] = 35
puts ages.size, ages.inspect
puts ages.key?("bob"), ages.key?("dave")
ages.each { |name, age| puts "#{name}: #{age}" }
ages.each_pair { |name, age| puts "#{name}=#{age}" }
puts ages.keys.inspect, ages.values.inspect
puts ages.fetch("alice"), ages.fetch("dave", 0)
puts ages.delete("bob").inspect, ages.delete("bob").inspect
puts ages.inspect
oldest = ages.max_by { |_name, age| age }
puts oldest.inspect
puts ages.sort_by { |name, _age| name }.map { |name, age| "#{name}#{age}" }.join(",")
puts ages.select { |_n, a| a > 30 }.inspect
puts ages.map { |n, a| [n.upcase, a + 1] }.inspect
puts ages.count, ages.empty?, {}.empty? #: Hash[String, Integer]
words = "b a c a b a".split
by_len = words.group_by { |w| w.size }
puts by_len.inspect
puts words.tally.inspect
puts words.uniq.sort.inspect, words.tally.sort_by { |w, n| [-n, w] }.inspect
nested = { "k" => [1, 2] }
puts nested.inspect, nested == { "k" => [1, 2] }
words.each_with_index { |w, i| print w, i }
puts
