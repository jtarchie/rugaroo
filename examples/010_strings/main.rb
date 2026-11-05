# rbs_inline: enabled

s = "Hello, World"
puts s.size, s.upcase, s.downcase, s.reverse
puts s.include?("World"), s.start_with?("Hell"), s.end_with?("x")
puts s.index("o").inspect, s.index("zz").inspect
puts s.sub("l", "L"), s.gsub("l", "L"), s.tr("lo", "01")
puts s[0].inspect, s[-1].inspect, s[99].inspect
puts s.chars.first(3).inspect
puts "  padded  ".strip + "|"
puts "abc" * 3
puts "a-b-c".split("-").inspect
puts "one two  three".split.inspect
puts "x".center(5) + "|", "x".ljust(3) + "|", "x".rjust(3) + "|"
puts "42abc".to_i + 1, "3.5".to_f * 2
puts "tab\there".inspect, "quote\"d".inspect, "new\nline".inspect
puts "%s" + "#{1 + 1}" + "#{true}" + "#{nil}" + "#{2.5}"
count = 0
"mississippi".each_char { |c| count += 1 if c == "s" }
puts count
puts "ab".ord, 97.chr
puts "Ruby".capitalize, "rUBY".capitalize
puts ["x", "y"].join(", "), [1, [2, 3]].inspect, [].inspect
puts "abc" <=> "abd", "b" > "a", "a".clamp("b", "c")
