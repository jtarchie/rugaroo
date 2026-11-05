# rbs_inline: enabled

require "monitor"

# Monitor and MonitorMixin: a reentrant lock, a condition variable over it,
# and a bounded buffer shared by a producer and a consumer thread.

class Buffer
  include MonitorMixin

  #: (Integer) -> void
  def initialize(max)
    @items = [] #: Array[Integer]
    @max = max
    @not_full = new_cond
    @not_empty = new_cond
  end

  #: (Integer) -> void
  def put(x)
    synchronize do
      @not_full.wait_while { @items.size >= @max }
      @items << x
      @not_empty.signal
    end
  end

  #: () -> Integer
  def take
    synchronize do
      @not_empty.wait_while { @items.empty? }
      x = @items.shift
      @not_full.signal
      x || 0
    end
  end
end

buf = Buffer.new(2)
producer = Thread.new do
  5.times { |i| buf.put(i * i) }
  nil
end
taken = (0...5).map { buf.take }
producer.join
puts "taken: #{taken.inspect}"

lock = Monitor.new
#: (Monitor, Integer) -> Integer
def nested(lock, depth)
  lock.synchronize do
    depth.zero? ? 0 : 1 + nested(lock, depth - 1)
  end
end
puts "re-entered #{nested(lock, 3)} times, owned now: #{lock.mon_owned?}, locked: #{lock.mon_locked?}"

lock.enter
puts "entered: owned #{lock.mon_owned?}, try_enter again #{lock.try_enter}"
lock.exit
lock.exit
puts "left twice: locked #{lock.mon_locked?}"
begin
  lock.exit
rescue ThreadError => e
  puts "exit without enter: #{e.message}"
end

t = Thread.new do
  Thread.current[:name] = "worker"
  Thread.current[:name]
end
puts "thread-local in the thread: #{t.value.inspect}, in main: #{Thread.current[:name].inspect}"
Thread.current[:count] = 2
Thread.current.thread_variable_set(:tv, "x")
puts "keys #{Thread.current.keys.inspect}, key? #{Thread.current.key?(:count)}, tv #{Thread.current.thread_variable_get(:tv).inspect}"
