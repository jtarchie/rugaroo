# rbs_inline: enabled

# An event bus: handlers are Procs stored in a Hash of Arrays.
class Bus
  #: () -> void
  def initialize
    @handlers = {} #: Hash[Symbol, Array[^(String) -> void]]
  end

  #: (Symbol, ^(String) -> void) -> void
  def on(event, handler)
    list = @handlers[event] || []
    list << handler
    @handlers[event] = list
  end

  #: (Symbol, String) -> Integer
  def emit(event, payload)
    list = @handlers[event] || []
    list.each { |h| h.call(payload) }
    list.size
  end
end

log = [] #: Array[String]
bus = Bus.new
bus.on(:save, ->(p) { log << "saved #{p}" })
bus.on(:save, ->(p) { log << "audit #{p.upcase}" })
puts bus.emit(:save, "doc1"), bus.emit(:delete, "doc2")
puts log.inspect

# Closures capture locals; lambdas compose.
#: (Integer) -> ^(Integer) -> Integer
def multiplier(n) = ->(x) { x * n }

triple = multiplier(3)
succ = ->(x) { x + 1 } #: ^(Integer) -> Integer
puts triple.call(5), triple.(6), triple[7], (triple >> succ).call(2), (triple << succ).call(2)
puts [1, 2, 3].map(&triple).inspect

count = 0
tick = -> { count += 1 }
5.times { tick.call }
puts count

clamp = lambda do |x|
  return 0 if x < 0
  next 100 if x > 100

  x
end #: ^(Integer) -> Integer
puts [-5, 50, 500].map(&clamp).inspect

shout = proc { |s| puts s.upcase + "!" } #: ^(String) -> void
%w[hey you].each(&shout)

pipeline = [succ, triple, succ] #: Array[^(Integer) -> Integer]
puts pipeline.reduce(4) { |acc, f| f.call(acc) }
puts triple.arity, triple.lambda?, triple.class, triple.is_a?(Proc)
