# rbs_inline: enabled

# args: --upcase alpha beta --repeat=2 gamma
# env: GREETING=hello
# env: RUGAROO_EMPTY=
# stderr: match

# A small command-line tool: flags and words from ARGV, settings from ENV,
# diagnostics on $stderr, and abort for a fatal error.

class Options
  attr_reader :upcase #: bool
  attr_reader :repeat #: Integer
  attr_reader :words #: Array[String]

  #: (Array[String]) -> void
  def initialize(argv)
    @upcase = false
    @repeat = 1
    @words = [] #: Array[String]
    argv.each do |arg|
      if arg == "--upcase"
        @upcase = true
      elsif arg.start_with?("--repeat=")
        @repeat = arg.delete_prefix("--repeat=").to_i
      elsif arg.start_with?("--")
        warn "unknown flag #{arg}"
      else
        @words << arg
      end
    end
  end
end

puts "program: #{$0}"
puts "same file: #{__FILE__ == $PROGRAM_NAME}"
puts "argc: #{ARGV.size}"
p ARGV

opts = Options.new(ARGV)
greeting = ENV.fetch("GREETING", "hi")
opts.words.each do |w|
  word = opts.upcase ? w.upcase : w
  opts.repeat.times { puts "#{greeting}, #{word}" }
end

puts "missing: #{ENV["RUGAROO_SURELY_UNSET"].inspect}"
puts "empty: #{ENV["RUGAROO_EMPTY"].inspect}"
puts "default: #{ENV.fetch("RUGAROO_SURELY_UNSET", "fallback")}"
puts "key?: #{ENV.key?("GREETING")}"
ENV["RUGAROO_SET"] = "set here"
puts "set: #{ENV["RUGAROO_SET"]}"
puts "deleted: #{ENV.delete("RUGAROO_SET").inspect}, now #{ENV["RUGAROO_SET"].inspect}"

begin
  ENV.fetch("RUGAROO_SURELY_UNSET")
rescue KeyError => e
  puts "KeyError: #{e.message}"
end

$stdout.puts "via $stdout"
STDOUT.print "via ", "STDOUT\n"
$stderr.puts "to stderr"
STDERR.print "also ", "stderr\n"
warn "warned", "twice"
$stdout << "chained" << " <<\n"
puts "tty? #{$stdout.tty?}"
puts "fileno #{$stderr.fileno}"

abort "fatal: done" if opts.words.size > 2
puts "not reached"
