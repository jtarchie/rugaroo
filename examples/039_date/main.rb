# rbs_inline: enabled

require "date"

# A monthly billing schedule: month arithmetic clamps to the month's end.
class Subscription
  attr_reader :start #: Date
  attr_reader :plan #: String

  #: (Date, String) -> void
  def initialize(start, plan)
    @start = start
    @plan = plan
  end

  #: (Integer) -> Array[Date]
  def invoices(n) = (0...n).map { |i| start >> i }

  #: (Date) -> Integer
  def days_active(on) = (on - start).to_i
end

sub = Subscription.new(Date.new(2024, 1, 31), "pro")
puts sub.invoices(4).map(&:to_s).join(" ")
puts sub.days_active(Date.parse("2024-03-01"))

d = Date.parse("March 5, 2024")
puts d.inspect
puts "#{d} is a #{Date::DAYNAMES[d.wday]}, day #{d.yday}, ISO week #{d.cweek}"
puts d.strftime("%a %-d %b %Y"), d.leap?, (d + 30).to_s, (d << 12).to_s
puts Date.strptime("12/25/2023", "%m/%d/%Y"), Date.new(2024, -1, -1)

weekend = (Date.new(2024, 3, 1)..Date.new(2024, 3, 14)).select { |x| x.saturday? || x.sunday? }
puts weekend.map(&:day).inspect

Date.new(2024, 2, 27).upto(Date.new(2024, 3, 1)) { |x| print x.strftime("%m/%d"), " " }
puts

by_month = Hash.new #: Hash[Integer, Integer]
Date.new(2024, 1, 1).step(Date.new(2024, 12, 31), 7) do |x|
  by_month[x.month] = (by_month[x.month] || 0) + 1
end
puts by_month.inspect

puts [Date.new(2024, 5, 1), Date.new(2023, 5, 1)].max.to_s
puts Date.valid_date?(2023, 2, 29), Date.leap?(2000), Date.leap?(1900)

begin
  Date.new(2023, 2, 29)
rescue Date::Error => e
  puts "#{e.class}: #{e.message}"
end
