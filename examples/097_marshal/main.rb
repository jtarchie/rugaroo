# rbs_inline: enabled

# Marshal (#47): deep copies, object graphs with shared parts and cycles, and a cache file. rugaroo writes its own bytes, so they are never printed.
require "tmpdir"

Item = Struct.new(:sku, :qty, :price) #: [String, Integer, Float]
Order = Struct.new(:id, :items, :tags) #: [Integer, Array[Item], Hash[String, bool]]

class Employee
  attr_reader :name #: String
  attr_reader :reports #: Array[Employee]
  attr_accessor :manager #: Employee?

  #: (String) -> void
  def initialize(name)
    @name = name
    @reports = [] #: Array[Employee]
    @manager = nil
  end

  #: (Employee) -> Employee
  def hire(e)
    e.manager = self
    @reports << e
    e
  end
end

#: (untyped) -> untyped
def deep_copy(obj) = Marshal.load(Marshal.dump(obj))

# The deep-copy idiom: unlike dup, every nested Struct, Array and Hash is copied.
order = Order.new(7, [Item.new("A-1", 2, 9.5), Item.new("B-2", 1, 20.0)], { "rush" => true })
copy = deep_copy(order) #: Order
puts "equal: #{copy == order}, same object: #{copy.equal?(order)}"
copy.items.fetch(0).qty = 10
copy.tags["gift"] = true
puts "original qty: #{order.items.fetch(0).qty}, copy qty: #{copy.items.fetch(0).qty}"
puts "original tags: #{order.tags.keys.inspect}, copy tags: #{copy.tags.keys.inspect}"
puts "copy items: #{copy.items.map(&:to_a).inspect}"

# Shared references stay shared and cycles survive: manager and reports point at each other.
ceo = Employee.new("Ada")
cto = ceo.hire(Employee.new("Grace"))
cto.hire(Employee.new("Linus"))
org = deep_copy(ceo) #: Employee
dev = org.reports.fetch(0).reports.fetch(0)
puts "#{dev.name} reports to #{dev.manager&.name}, who reports to #{dev.manager&.manager&.name}"
puts "cycle intact: #{org.reports.fetch(0).manager.equal?(org)}, new objects: #{!org.equal?(ceo)}"

loop_list = [] #: Array[untyped]
loop_list << 1 << loop_list
again = deep_copy(loop_list) #: Array[untyped]
puts "self-containing array: #{again[1].equal?(again)}, first #{again[0]}"

# A cache file: each dump knows its own length, so several can share one file.
Dir.mktmpdir do |dir|
  path = File.join(dir, "cache.bin")
  File.open(path, "wb") do |f|
    Marshal.dump(order, f)
    Marshal.dump(%w[alpha beta], f)
    Marshal.dump(42, f)
  end
  loaded = [] #: Array[untyped]
  File.open(path, "rb") do |f|
    loaded << Marshal.load(f) until f.eof?
  end
  puts "read #{loaded.size} values back"
  puts "first one equals the order: #{loaded[0] == order}"
  puts "then #{loaded[1].inspect} and #{loaded[2]}"
end

# Some values carry live state and cannot be dumped.
undumpable = [proc { 1 }, $stdout] #: Array[untyped]
undumpable.each do |v|
  Marshal.dump(v)
rescue TypeError => e
  puts "TypeError: #{e.message}"
end
