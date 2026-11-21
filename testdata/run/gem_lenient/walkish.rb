# rubocop's FileFinder#traverse_directories_upwards (decision 475): a gem's own iterator as a value, its block
# leaving with `break value`.
class Walkish
  def initialize(xs)
    @xs = xs
  end

  def upwards(from)
    i = 0
    while i < @xs.size
      yield(@xs[i]) if @xs[i] >= from
      i += 1
    end
  end
end

class WalkishUser
  def initialize(w)
    @w = w
  end

  def first_even(from)
    @found = @w.upwards(from) do |x|
      break x * 10 if x.even?
    end
  end
end
