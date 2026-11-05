# frozen_string_literal: true
# rbs_inline: enabled

#: (bool, String) -> void
def assert(value, msg)
  raise msg unless value
end

s = "hello, world"
assert(s.upcase == "HELLO, WORLD", "upcase")               # String
assert(s < "world", "<")                                   # Comparable
assert(s.clamp("a", "c") == "c", "clamp")                  # Comparable
assert(s.then { |x| x + x } == s + s, "then")              # Kernel
assert(s.equal?(s), "equal?")                              # BasicObject
assert(!s.equal?(s.dup), "equal? on dup")
