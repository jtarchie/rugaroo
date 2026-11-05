# rbs_inline: enabled

# DateTime (#45): a Date subclass with a time of day and a UTC offset. Flight times across zones, durations as exact Rationals, and parsing and formatting.
require "date"

departure = DateTime.new(2024, 2, 29, 22, 15, 0, "+09:00")
puts "departs:  #{departure}"
puts "  #{departure.year}-#{departure.month}-#{departure.day} #{departure.hour}:#{departure.minute} zone #{departure.zone} (#{departure.offset} of a day)"
puts "  a #{departure.strftime("%A")}, day #{departure.yday} of a leap year: #{departure.leap?}"

# Flight time is 11h40m; a fraction of a day added keeps the offset.
arrival = departure + Rational(11 * 60 + 40, 24 * 60)
puts "arrives:  #{arrival} (Tokyo time)"
local = arrival.new_offset("-08:00")
puts "          #{local} (San Francisco time)"
puts "          same instant: #{local == arrival}, earlier date: #{local.to_date < arrival.to_date}"

elapsed = arrival - departure
puts "elapsed:  #{elapsed} days = #{(elapsed * 24).to_f} hours"

# Day and month steps keep the time of day; >> clamps to the month's last day.
puts "next day:     #{departure + 1}"
puts "next month:   #{departure >> 1}"
puts "a year later: #{departure >> 12}"
puts "week ago:     #{(departure - 7).strftime("%Y-%m-%d %H:%M %z")}"

# Formats: ISO 8601 with fraction digits, RFC 2822, HTTP date (always GMT), strftime.
precise = DateTime.new(2024, 3, 1, 9, 30, Rational(61, 4), "+05:30")
puts "iso8601(3): #{precise.iso8601(3)}"
puts "rfc2822:    #{precise.rfc2822}"
puts "httpdate:   #{precise.httpdate}"
puts "strftime:   #{precise.strftime("%-d %b %Y, %I:%M:%S.%L %p %:z")}"

# Parsing several shapes; all are normalized to UTC for comparison.
inputs = [
  "2024-03-01T10:00:00+01:00",
  "Fri, 01 Mar 2024 04:00:00 -0500",
  "March 1, 2024 8:30am",
  "2024-03-01 09:00 Z"
]
parsed = inputs.map do |s|
  dt = DateTime.parse(s)
  puts "#{s.ljust(32)} => #{dt.new_offset(0)}"
  dt
end
puts "earliest: #{parsed.min&.new_offset(0)}"
puts "all on #{parsed.map(&:to_date).uniq.map(&:to_s).inspect}"

begin
  DateTime.parse("not a date")
rescue Date::Error => e
  puts "bad input: #{e.message} (#{e.class})"
end

# A Date is midnight UTC; mixing the two subtracts exactly.
puts "until midnight: #{Date.new(2024, 3, 2) - DateTime.new(2024, 3, 1, 18, 0, 0)} of a day"
