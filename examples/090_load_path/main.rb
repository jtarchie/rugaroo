# rbs_inline: enabled

# load_path: lib

# Plain `require` of the program's own files, found on the load path given
# with -I (`ruby -I lib main.rb`, `rugaroo run -I lib main.rb`), next to a
# standard library require. Each file loads once, wherever it is first
# required.

require "json"
puts "requiring units"
require "units/report"
require "units/temperature" # loaded by report.rb already: a no-op

readings = [Temperature.new(21.5), Temperature.new(-3.0), Temperature.new(37.0)]
puts readings.map(&:to_s).join(", ")
puts Report.new(readings).summary
puts JSON.generate(readings.map(&:fahrenheit))
