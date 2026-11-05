# rbs_inline: enabled

# A log entry stamped in UTC, so output is the same in every time zone.
class Entry
  attr_reader :at #: Time
  attr_reader :msg #: String

  #: (Time, String) -> void
  def initialize(at, msg)
    @at = at
    @msg = msg
  end

  #: () -> String
  def to_s = "[#{at.strftime("%Y-%m-%d %H:%M:%S")}] #{msg}"
end

#: (Float) -> String
def human(secs)
  mins = (secs / 60).to_i
  "#{mins}m#{(secs - mins * 60).to_i}s"
end

base = Time.utc(2024, 2, 29, 23, 59, 30)
log = [
  Entry.new(base, "start"),
  Entry.new(base + 45, "leap day rolls over"),
  Entry.new(base + 3.5, "fractional"),
  Entry.new(Time.at(1_700_000_000).utc, "epoch 1.7e9")
]
log.sort_by(&:at).each { |e| puts e }

puts base
puts base.inspect
puts (base + 0.25).inspect
puts base.iso8601, base.iso8601(3)
puts base.strftime("%a %-d %b %Y, %l:%M %p (%Z) day %j, week %V")
puts base.strftime("%A|%^B|%10A|%-m/%-d/%y|%s")
puts "#{base.year}-#{base.month}-#{base.day} wday=#{base.wday} yday=#{base.yday} thursday=#{base.thursday?}"

later = log.map(&:at).max || base
puts later.inspect
puts human(later - base)
puts base < later, base == Time.utc(2024, 2, 29, 23, 59, 30)
puts Time.at(0).utc, Time.at(0).utc.to_i

begin
  Time.utc(2024, 13, 1)
rescue ArgumentError => e
  puts "error: #{e.message}"
end

started = Time.now
puts (Time.now - started) >= 0
