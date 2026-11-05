# rbs_inline: enabled

require "optparse"

# OptionParser: each block's parameter is typed from its switch and
# coercion class (Integer here, Array[String] there, String? for [OPTIONAL]).

Options = Struct.new(:count, :tags, :format, :verbose, :out) #: [Integer, Array[String], String, bool, String?]

#: (Array[String]) -> void
def run(argv)
  opts = Options.new(1, [], "text", false, nil)
  parser = OptionParser.new do |o|
    o.banner = "Usage: report [options] FILE..."
    o.on("-c", "--count N", Integer, "Repeat N times") { |n| opts.count = n }
    o.on("-t", "--tags A,B", Array, "Only these tags") { |t| opts.tags = t }
    o.on("-f", "--format [FMT]", "text (default) or json") { |f| opts.format = f || "json" }
    o.on("-v", "--[no-]verbose", "Say more") { |v| opts.verbose = v }
    o.on("-o FILE", "Write to FILE") { |f| opts.out = f }
    o.separator ""
    o.on_tail("-h", "--help", "Show this help") { |_| puts o }
  end

  files = parser.parse(argv)
  puts "argv #{argv.inspect}"
  puts "  count=#{opts.count} tags=#{opts.tags.inspect} format=#{opts.format} verbose=#{opts.verbose} out=#{opts.out.inspect}"
  puts "  files #{files.inspect}"
rescue OptionParser::ParseError => e
  puts "argv #{argv.inspect}"
  puts "  error: #{e.message}"
end

run(%w[-c 3 report.csv --tags=q1,q2 -v])
run(%w[--format -o out.txt a.csv b.csv --no-verbose])
run(%w[--form json --cou 2])
run(%w[-c many])
run(%w[--colour])
run(%w[-h])
