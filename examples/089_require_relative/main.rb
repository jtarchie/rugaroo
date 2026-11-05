# rbs_inline: enabled

# A program split across files with require_relative: each file loads once,
# its top level runs where it is required, and its locals stay its own.

puts "loading the inventory library"
count = 3
require_relative "lib/inventory"
require_relative "lib/item" # already loaded by inventory.rb: a no-op

inventory = Inventory.new
inventory.add(Item.new("bolt", 12, 0.25))
inventory.add(Item.new("nut", 40, 0.1))
inventory.add(Item.new("washer", count, 0.05))
inventory.report
puts "total value: #{format("%.2f", inventory.value)} #{CURRENCY}"
puts "main.rb is the program: #{__FILE__ == $0}"
