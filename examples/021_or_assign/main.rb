# rbs_inline: enabled

# `||=` on locals and instance variables (memoization).

class Cache
  #: () -> void
  def initialize
    @calls = 0
  end

  #: () -> Integer
  def calls = @calls

  #: () -> String
  def value
    @value ||= compute
  end

  private

  #: () -> String
  def compute
    @calls += 1
    "computed"
  end
end

cache = Cache.new
puts cache.value, cache.value, cache.calls

name = nil
name ||= "default"
puts name
count = nil
count ||= 1
count ||= 2
puts count
