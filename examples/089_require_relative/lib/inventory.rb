# rbs_inline: enabled

require_relative "item"

puts "  inventory.rb loaded"
CURRENCY = "USD"

class Inventory
  #: () -> void
  def initialize
    @items = [] #: Array[Item]
  end

  #: (Item) -> void
  def add(item)
    @items << item
  end

  #: () -> Float
  def value = @items.map(&:value).sum

  #: () -> void
  def report
    @items.each { |i| puts "#{i.name.ljust(8)} #{i.quantity.to_s.rjust(3)}" }
  end
end
