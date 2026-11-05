# rbs_inline: enabled

require "time"
require "benchmark"

# Log lines stamped in assorted formats, normalised to UTC and bucketed by hour.
class LogLine
  attr_reader :at #: Time
  attr_reader :msg #: String

  #: (Time, String) -> void
  def initialize(at, msg)
    @at = at
    @msg = msg
  end

  #: (String) -> LogLine
  def self.parse(line)
    stamp, msg = line.split(" | ")
    new(Time.parse(stamp || ""), msg || "")
  end
end

lines = [
  "2024-03-05T12:34:56Z | boot",
  "2024-03-05 13:05:00 +0200 | login",
  "2024-03-05T09:15:30.250-05:00 | fetch",
  "Tue, 05 Mar 2024 14:00:00 GMT | cron"
]
logs = lines.map { |l| LogLine.parse(l) }
logs.each { |l| puts "#{l.at.utc.iso8601(3)} #{l.msg}" }
by_hour = logs.group_by { |l| l.at.utc.hour }
by_hour.keys.sort.each { |h| puts "#{h}: #{by_hour.fetch(h).map(&:msg).join(", ")}" }

t = Time.iso8601("2024-03-05T12:34:56+09:00")
puts t, t.utc_offset, t.iso8601, t.utc?
u = Time.xmlschema("1999-12-31T23:59:59.5Z")
puts u.inspect, u.utc?, u.usec
puts Time.utc(2024, 3, 5, 12, 0, 0).httpdate, Time.utc(2024, 3, 5, 12, 0, 0).rfc2822
puts Time.iso8601("2024-03-05T12:34:56+09:00").rfc2822, Time.iso8601("2024-03-05T12:34:56+09:00").httpdate
h = Time.httpdate("Sun, 06 Nov 1994 08:49:37 GMT")
puts h, h.utc?, h.wday
r = Time.rfc2822("Sun, 6 Nov 1994 08:49:37 -0800")
puts r, r.utc_offset
s = Time.strptime("05/03/2024 07:08:09 +0100", "%d/%m/%Y %H:%M:%S %z")
puts s, s.min
s2 = Time.strptime("2024-061 23:59", "%Y-%j %H:%M")
puts s2.month, s2.day, s2.hour
s3 = Time.strptime("March 7 2024 3:04 PM +0000", "%B %d %Y %I:%M %p %z")
puts s3.hour, s3.utc_offset
puts Time.parse("2024-03-05 01:02:03 UTC").utc?, Time.parse("2024-03-05 01:02:03 UTC")
begin
  Time.iso8601("yesterday")
rescue ArgumentError => e
  puts e.message
end
begin
  Time.strptime("nope", "%Y")
rescue ArgumentError => e
  puts e.message
end
begin
  Time.parse("")
rescue ArgumentError => e
  puts e.message
end

elapsed = Benchmark.realtime { (1..1000).sum }
puts elapsed.is_a?(Float), elapsed >= 0
