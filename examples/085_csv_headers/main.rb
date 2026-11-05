# rbs_inline: enabled

require "csv"

# CSV with headers: rows read by column name, and numbers converted.

SALES = <<~CSV
  region,product,units,price
  north,widget,12,2.50
  south,widget,7,2.50
  north,gadget,3,19.99
  east,widget,,2.50
CSV

table = CSV.parse(SALES, headers: true, converters: :numeric)
puts "columns: #{table.headers.join(", ")}"
puts "rows: #{table.size}"

table.each do |row|
  units = row["units"] || 0
  puts format("%-6s %-7s %3d x %6.2f", row["region"], row["product"], units, row["price"])
end

total = 0.0
table.each { |row| total += (row["units"] || 0) * row["price"] }
puts format("revenue: %.2f", total)

north = table.select { |row| row["region"] == "north" }
puts "north products: #{north.map { |row| row["product"] }.join(", ")}"
puts "regions: #{table["region"].uniq.inspect}"
puts "first row: #{table[0].to_h}"
puts "missing units in: #{table.find { |row| row["units"].nil? }&.[]("region")}"

puts "--- without headers, converted"
CSV.parse(SALES, converters: :numeric).first(2).each { |r| p r }

puts "--- as text again"
print table.to_s
