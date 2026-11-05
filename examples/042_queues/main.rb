# rbs_inline: enabled

# A worker pool: jobs flow through a Queue, results are summed under a Mutex.
class Pool
  #: (Integer) -> void
  def initialize(size)
    @jobs = Queue.new #: Queue[Integer]
    @lock = Mutex.new
    @total = 0
    @done = 0
    @workers = (1..size).map { Thread.new { work } } #: Array[Thread]
  end

  #: (Integer) -> void
  def submit(n)
    @jobs << n
  end

  #: () -> [Integer, Integer]
  def finish
    @jobs.close
    @workers.each(&:join)
    [@total, @done]
  end

  private

  #: () -> void
  def work
    while (n = @jobs.pop)
      square = n * n
      @lock.synchronize do
        @total += square
        @done += 1
      end
    end
  end
end

pool = Pool.new(4)
(1..1000).each { |i| pool.submit(i) }
total, done = pool.finish
puts "#{done} jobs, sum of squares #{total}"

# Bounded hand-off: the producer blocks while the SizedQueue is full.
belt = SizedQueue.new(2) #: SizedQueue[String]
producer = Thread.new do
  %w[bolt nut gear spring].each { |part| belt << part }
  belt.close
end
received = [] #: Array[String]
while (part = belt.pop)
  received << part
end
producer.join
puts received.inspect, belt.closed?, belt.max

# A ConditionVariable gate: the waiter sleeps until signalled.
mutex = Mutex.new
gate = ConditionVariable.new
open = false
waiter = Thread.new do
  mutex.synchronize do
    gate.wait(mutex) until open
    puts "gate opened"
  end
end
mutex.synchronize do
  open = true
  gate.broadcast
end
waiter.join

puts mutex.locked?, mutex.try_lock, mutex.locked?
mutex.unlock
begin
  mutex.unlock
rescue ThreadError => e
  puts "error: #{e.message}"
end
begin
  belt << "late"
rescue ClosedQueueError => e
  puts "error: #{e.message}"
end
