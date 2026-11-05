# rbs_inline: enabled

# Required by inventory.rb; requiring it back is a cycle Ruby ends by loading
# each file once.
require_relative "inventory"

puts "  item.rb loaded (the program: #{__FILE__ == $0}, in #{File.basename(__dir__)}/)"
count = 0 # not main.rb's count: top-level locals belong to their file
puts "  item.rb's count: #{count}"

class Item
  attr_reader :name #: String
  attr_reader :quantity #: Integer
  attr_reader :price #: Float

  #: (String, Integer, Float) -> void
  def initialize(name, quantity, price)
    @name = name
    @quantity = quantity
    @price = price
  end

  #: () -> Float
  def value = quantity * price
end
