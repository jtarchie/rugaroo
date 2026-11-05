# rbs_inline: enabled

require "units/temperature"

puts "  units/report loaded"

class Report
  #: (Array[Temperature]) -> void
  def initialize(readings)
    @readings = readings
  end

  #: () -> String
  def summary
    coldest = @readings.min_by(&:celsius) || raise("no readings")
    warmest = @readings.max_by(&:celsius) || raise("no readings")
    "#{@readings.size} readings, coldest #{coldest}, warmest #{warmest}"
  end
end
