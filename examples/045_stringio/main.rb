# rbs_inline: enabled

require "stringio"

# A report renderer that writes to any StringIO, then a parser that reads it back.
class Report
  #: (Array[[String, Integer]]) -> void
  def initialize(rows)
    @rows = rows
  end

  #: (StringIO) -> Integer
  def render(out)
    out.puts "name,qty"
    @rows.each { |name, qty| out << name << "," << qty << "\n" }
    out.print "total,", @rows.sum { |_, q| q }, "\n"
    @rows.size
  end
end

buf = StringIO.new
count = Report.new([["apple", 3], ["pear", 5], ["fig", 12]]).render(buf)
puts "#{count} rows, #{buf.size} bytes"
puts buf.string

input = StringIO.new(buf.string)
header = input.gets || ""
puts "header: #{header.chomp}"
totals = Hash.new #: Hash[String, Integer]
input.each_line do |line|
  name, qty = line.chomp.split(",")
  totals[name || ""] = (qty || "0").to_i
end
puts totals.inspect, input.eof?, input.lineno

input.rewind
puts input.read(4).inspect, input.getc.inspect, input.readlines.size, input.read(1).inspect

patch = StringIO.new("hello world")
patch.write("J")
patch.pos = 6
patch.write("W")
puts patch.string
