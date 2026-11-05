# rbs_inline: enabled

require "singleton"

# A process-wide metrics registry that parses "name=value" samples and renders a table.
class Metrics
  include Singleton

  #: () -> void
  def initialize
    @samples = {} #: Hash[String, Array[Float]]
  end

  #: (String) -> Integer
  def ingest(text)
    pairs = text.scan(/(\w+)=(-?\d+(?:\.\d+)?)/)
    pairs.each do |name, value|
      (@samples[name || ""] ||= []) << (value || "0").to_f
    end
    pairs.size
  end

  #: () -> String
  def report
    lines = [format("%-10s %5s %8s %8s", "metric", "n", "mean", "max")]
    @samples.keys.sort.each do |name|
      vals = @samples.fetch(name)
      lines << format("%-10s %5d %8.2f %8.2f", name, vals.size, vals.sum / vals.size, vals.max || 0.0)
    end
    lines.join("\n")
  end
end

puts Metrics.instance.ingest("cpu=0.5 mem=512 cpu=0.75 latency_ms=12.5")
puts Metrics.instance.ingest("mem=640, latency_ms=-3; junk cpu=1")
puts Metrics.instance.report
puts Metrics.instance.equal?(Metrics.instance)

slug = "  Hello, World! Ruby->Go  ".strip.downcase.gsub(/[^a-z0-9]+/, "-").gsub(/\A-|-\z/, "")
puts slug
puts "2024-03-05".sub(/(\d+)-(\d+)-(\d+)/, "\\3/\\2/\\1")
puts "snake_case_name".gsub(/_(\w)/) { |m| m[1].to_s.upcase }
puts "The year 1999 and 2024".scan(/\d{4}/).inspect
puts "a, b;c  d".split(/[,;\s]+/).inspect
puts "%s scored %05.1f%% (%+d)" % ["ann", 93.456, 7]
puts format("%<name>s is %<age>d", name: "Bo", age: 3), format("%x %o %b %e", 255, 8, 5, 1234.5)
printf("%3d|%-3d|\n", 7, 7)
