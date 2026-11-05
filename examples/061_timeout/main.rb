# rbs_inline: enabled

require "timeout"

# Timeout.timeout races a block against a wall-clock deadline. Go has no
# thread preemption, so a stuck block cannot actually be interrupted: this
# can only notice the deadline passed and raise from the caller's side,
# leaving the block's own goroutine to keep running in the background.

#: (Float) -> void
def busy_wait(secs)
  started = Time.now
  while Time.now - started < secs
  end
end

class RetryBudgetExceeded < StandardError; end

#: () -> Integer
def slow_computation
  busy_wait(2.0)
  42
end

#: () -> Integer
def fast_computation
  1 + 1
end

begin
  Timeout.timeout(0.1) { slow_computation }
rescue Timeout::Error => e
  puts "#{e.class}: #{e.message}"
end

result = Timeout.timeout(5.0) { fast_computation }
puts "fast_computation: #{result}"

begin
  Timeout.timeout(0.1, RetryBudgetExceeded, "retry budget exceeded") { slow_computation }
rescue RetryBudgetExceeded => e
  puts "#{e.class}: #{e.message}"
end

result2 = Timeout.timeout(nil) { fast_computation }
puts "no deadline: #{result2}"
