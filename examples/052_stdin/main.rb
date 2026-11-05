# rbs_inline: enabled

# stdin: "3\n"
# stdin: "alice 30\nbob 25\n"
# stdin: "carol 35\n"
# stdin: "--\nthe quick brown fox\njumps over\nthe lazy dog"

# Reads a count, that many records, then a free-text tail: the shapes a
# filter program takes on stdin.

Person = Struct.new(:name, :age) #: [String, Integer]

#: () -> Integer
def read_count
  line = gets
  return 0 unless line

  line.chomp.to_i
end

n = read_count
people = [] #: Array[Person]
n.times do
  line = $stdin.gets
  break unless line

  name, age = line.split
  people << Person.new(name || "?", (age || "0").to_i)
end

people.sort_by(&:age).each { |pr| puts "#{pr.name}: #{pr.age}" }
puts "oldest: #{people.max_by(&:age)&.name}"

separator = STDIN.gets&.chomp
puts "separator: #{separator.inspect}"

counts = {} #: Hash[String, Integer]
lines = 0
$stdin.each_line do |l|
  lines += 1
  l.split.each { |w| counts[w] = (counts[w] || 0) + 1 }
end
puts "lines: #{lines}"
counts.sort_by { |w, c| [-c, w] }.first(3).each { |w, c| puts "#{w} #{c}" }

puts "eof? #{$stdin.eof?}"
p $stdin.gets
p STDIN.read
p gets
