# rbs_inline: enabled

# Thread.new / join. An exception in a thread re-raises on join.

results = []
worker = Thread.new { results << 42 }
worker.join
puts results.inspect

failing = Thread.new { raise ArgumentError, "boom" }
begin
  failing.join
rescue ArgumentError => e
  puts "joined: #{e.message}"
end
