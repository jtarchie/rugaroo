# rbs_inline: enabled

# MRI's MT19937 with MRI's seeding, so `Random.new(42).rand(100)` matches; seeds are 64-bit.
# @go_type struct { mt rbMT; seed Integer }
class Random < Object
  #: (Integer) -> Random
  def self.new(seed) = %x{ return rbNewRandom(seed) }

  #: () -> Random
  def self.__new_0 = %x{ return rbNewRandom(rbNewSeed()) }

  #: () -> Random
  def self.__default = %x{ return rbDefaultRandom }

  #: () -> Integer
  def self.new_seed = %x{ rbNewSeed() }

  # On the default generator (decision 403: lock-free unless the program can seed); a negative bound is an error here, unlike Kernel#rand's.
  #: (Integer) -> Integer
  def self.rand(n) = %x{
    rbRandArg(n)
    return Integer(rbRandInt(int(n)))
  }

  #: () -> Float
  def self.__rand_0 = %x{ Float(rbRandFloat()) }

  # Kernel#srand on the class: returns the previous seed.
  #: (?Integer) -> Integer
  def self.srand(seed = 0) = %x{ rbSrand(seed) }

  #: () -> Integer
  def seed = %x{ self.seed }

  #: (Integer) -> Integer
  def rand(n) = %x{
    rbRandArg(n)
    return Integer(self.mt.upto(int(n)))
  }

  #: () -> Float
  def __rand_0 = %x{ Float(self.mt.real()) }

  #: (Float) -> Float
  def __rand_float(f) = %x{
    if f <= 0 {
      panic(NewArgumentError(Ref(String("invalid argument - " + string(f.ToS())))))
    }
    return Float(float64(f) * self.mt.real())
  }

  #: (Range[Integer]) -> Integer
  def __rand_range(r) = %x{
    b, e := r.b, r.e
    if !r.excl {
      e++
    }
    if e <= b {
      panic(NewArgumentError(Ref(String("invalid argument - " + string(r.Inspect())))))
    }
    return b + Integer(self.mt.upto(int(e-b)))
  }

  # @binary
  #: (Integer) -> String
  def bytes(n) = %x{
    out := make([]byte, 0, n+4)
    for len(out) < int(n) {
      self.mt.mu.Lock()
      w := self.mt.next()
      self.mt.mu.Unlock()
      out = binary.LittleEndian.AppendUint32(out, w)
    }
    return String(out[:n])
  }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Random)
    return Boolean(ok && o.seed == self.seed && o.mt.s == self.mt.s && o.mt.i == self.mt.i)
  }
end

module Kernel
  private

  # A negative bound counts as its absolute value, as MRI's Kernel#rand; 0 means a Float.
  #: (Integer) -> Integer
  def rand(n) = %x{
    if n < 0 {
      n = -n
    }
    rbRandArg(n)
    return Integer(rbRandInt(int(n)))
  }

  #: () -> Float
  def __rand_0 = %x{ Float(rbRandFloat()) }

  #: (Range[Integer]) -> Integer
  def __rand_range(r) = %x{
    b, e := r.b, r.e
    if !r.excl {
      e++
    }
    if e <= b {
      panic(NewArgumentError(Ref(String("invalid argument - " + string(r.Inspect())))))
    }
    return b + Integer(rbRandInt(int(e-b)))
  }

  # Returns the previous seed, as MRI.
  #: (?Integer) -> Integer
  def srand(seed = 0) = %x{ rbSrand(seed) }
end

class Array
  #: () -> Array[E]
  def __shuffle_0 = dup.shuffle!(random: Random.__default)

  #: (Hash[Symbol, Random]) -> Array[E]
  def shuffle(opts) = dup.shuffle!(opts)

  #: () -> Array[E]
  def __shuffle_bang_0 = shuffle!(random: Random.__default)

  #: (Hash[Symbol, Random]) -> Array[E]
  def shuffle!(opts) = %x{
    self.rbCheckFrozen()
    rng := rbDefaultRandom
    if r := Hash_Op_idx(opts, Symbol("random")); r != nil {
      rng = *r
    }
    rbShuffle(self.s, &rng.mt)
    return self
  }

  #: (Integer, Hash[Symbol, Random]) -> Array[E]
  def __sample_2(n, opts) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative sample number"))))
    }
    rng := rbDefaultRandom
    if r := Hash_Op_idx(opts, Symbol("random")); r != nil {
      rng = *r
    }
    idx := rbSampleIdx(int(n), len(self.s), &rng.mt)
    out := &Array[E]{s: make([]E, 0, len(idx))}
    for _, i := range idx {
      out.s = append(out.s, self.s[i])
    }
    return out
  }

  #: (Integer) -> Array[E]
  def sample(n) = __sample_2(n, { random: Random.__default })

  #: (Hash[Symbol, Random]) -> E?
  def __sample_hash(opts) = __sample_2(1, opts).first

  #: () -> E?
  def __sample_0 = __sample_hash({ random: Random.__default })
end
