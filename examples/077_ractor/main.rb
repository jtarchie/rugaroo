# rbs_inline: enabled

#
# Ractors: isolated workers that talk through ports. A message or constructor
# argument is copied into the ractor, never shared, so no lock is needed.

# A pool: each worker takes jobs from its own inbox and reports on one shared port.
results = Ractor::Port.new
workers = 3.times.map do |i|
  Ractor.new(results, i) do |out, id|
    done = 0
    job = Ractor.receive #: Integer
    while job >= 0
      out << job * job
      done += 1
      job = Ractor.receive #: Integer
    end
    "worker #{id} did #{done}"
  end
end

jobs = (1..9).to_a
jobs.each_with_index { |job, i| workers[i % 3] << job }
workers.each { |w| w << -1 }

squares = [] #: Array[Integer]
jobs.size.times do
  sq = results.receive #: Integer
  squares << sq
end
puts squares.sort.inspect
workers.each { |w| puts w.value }

# Arguments are deep copies: the worker's changes never reach the caller.
list = [1, 2, 3]
grown = Ractor.new(list) do |xs|
  xs << 4
  xs.sum
end
puts grown.value, list.inspect

# A frozen object is shareable, so it is passed as is.
frozen = [1, 2].freeze
puts Ractor.new(frozen) { |xs| xs }.value.equal?(frozen)

# An exception inside a ractor reaches value/join as a RemoteError around it.
failing = Ractor.new { raise ArgumentError, "bad job" }
begin
  failing.value
rescue Ractor::RemoteError => err
  cause = err.cause
  puts "#{err.message} #{cause.class}: #{cause.message}" if cause
end

# Ractor.select waits on several ports at once; the main ractor has a port too.
fast = Ractor::Port.new
slow = Ractor::Port.new
Ractor.new(fast) { |pt| pt << :fast }
port, msg = Ractor.select(fast, slow)
puts msg, port.equal?(fast)
Ractor.new(Ractor.main) { |main| main << "hello from a worker" }
puts Ractor.receive
puts Ractor.main?, Ractor.new { Ractor.main? }.value
