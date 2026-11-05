# rbs_inline: enabled

require "weakref"

# WeakRef: a reference the garbage collector may clear. Alive while the
# object is held elsewhere; nil, and RefError on access, once it is gone.

class Session
  attr_reader :id #: Integer

  #: (Integer) -> void
  def initialize(id)
    @id = id
  end
end

cache = {} #: Hash[Integer, WeakRef[Session]]

#: (Hash[Integer, WeakRef[Session]]) -> Session
def open_session(cache)
  s = Session.new(7)
  cache[s.id] = WeakRef.new(s)
  s
end

#: (Hash[Integer, WeakRef[Session]]) -> void
def open_and_drop(cache)
  s = Session.new(8)
  cache[s.id] = WeakRef.new(s)
end

held = open_session(cache)
open_and_drop(cache)
puts "held: alive #{cache.fetch(7).weakref_alive?.inspect}, id #{cache.fetch(7).__getobj__.id}"
3.times { GC.start }
puts "dropped: alive #{cache.fetch(8).weakref_alive?.inspect}"
begin
  cache.fetch(8).__getobj__
rescue WeakRef::RefError => e
  puts "RefError: #{e.message}"
end
puts "still held: #{cache.fetch(7).__getobj__.id == held.id}"

num = WeakRef.new(42)
GC.start
puts "a value is always alive: #{num.weakref_alive?} #{num.__getobj__ + 1}"
