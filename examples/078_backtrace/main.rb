# rbs_inline: enabled

# Exception#backtrace: the frames an exception was raised through, in
# MRI 4.0's format: "file:line:in 'Class#method'", blocks as
# "block in", core methods written in Ruby (Array#each, Kernel#Integer)
# at the line that called them.

class Parser
  #: (Array[String]) -> Array[Integer]
  def parse(fields)
    out = [] #: Array[Integer]
    fields.each do |f|
      [f].each { |s| out << Integer(s) }
    end
    out
  end

  #: (Array[String]) -> Array[Integer]
  def self.run(fields) = new.parse(fields)
end

#: (Array[String]) -> Array[Integer]
def parse_all(fields) = Parser.run(fields)

begin
  parse_all(["1", "x"])
rescue ArgumentError => e
  puts e.message
  puts e.backtrace
end

puts "-- a lambda"
check = ->(n) { raise "negative" if n < 0 } #: ^(Integer) -> void
begin
  check.call(-1)
rescue RuntimeError => e
  puts e.backtrace
end

puts "-- a re-raise keeps the original frames"
#: () -> void
def fails = raise(IOError, "disk")

#: () -> void
def retry_once
  fails
rescue IOError => e
  raise e
end

begin
  retry_once
rescue IOError => e
  puts e.backtrace
end

puts "-- an exception raised in a rescue clause starts there"
#: () -> void
def translate
  fails
rescue IOError => e
  raise ArgumentError, "wrapped #{e.message}"
end

begin
  translate
rescue ArgumentError => e
  puts e.backtrace
  puts "cause: #{e.cause.class}: #{e.cause.backtrace.first}"
end

puts "-- set_backtrace"
begin
  made = RuntimeError.new("made up")
  made.set_backtrace(["elsewhere.rb:1:in 'Object#nowhere'"])
  raise made
rescue RuntimeError => e
  puts e.backtrace
end

puts "-- never rescued with a binding: nil"
fresh = RuntimeError.new("fresh")
p fresh.backtrace
