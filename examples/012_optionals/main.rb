# rbs_inline: enabled

class Node
  attr_reader :value #: Integer
  attr_reader :next_node #: Node?

  #: (Integer, Node?) -> void
  def initialize(value, next_node)
    @value = value
    @next_node = next_node
  end

  #: () -> Integer
  def length
    n = next_node
    n ? 1 + n.length : 1
  end

  #: () -> Integer
  def sum
    total = value
    cur = next_node
    while cur
      total += cur.value
      cur = cur.next_node
    end
    total
  end
end

list = Node.new(1, Node.new(2, Node.new(3, nil)))
puts list.length, list.sum
puts list.next_node&.next_node&.value.inspect
puts list.next_node&.next_node&.next_node&.value.inspect

#: (Array[Integer], Integer) -> Integer?
def find_gt(nums, n) = nums.find { |x| x > n }

nums = [1, 5, 9]
found = find_gt(nums, 4)
puts found.inspect, find_gt(nums, 100).inspect
puts found.nil?, find_gt(nums, 100).nil?
puts(found ? found * 2 : -1)
v = find_gt(nums, 100) || 0
puts v + 1
name = nil
puts name.to_s + "|", name.inspect
name = "x" if nums.size > 2
puts name.inspect
puts "big" if found && found > 3
puts "none" unless find_gt(nums, 100)
puts nums.min.inspect, nums.max.inspect, [].min.inspect #: Array[Integer]
puts nums.pop.inspect, nums.inspect, nums.last.inspect
