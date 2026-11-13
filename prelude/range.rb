# rbs_inline: enabled

# Iteration needs Integer or String (MRI's succ); other E only compare.

# @rbs generic E
# @go_type struct { b E; e E; excl bool; endless bool; beginless bool; inf bool }
class Range < Object
  # Range.new(b, e, exclude_end = false), as the literal b..e (b...e) is; a nil end is not taken here (ponytail: Range[E?]).
  #: [X] (X, X, ?bool) -> Range[X]
  def self.new(b, e, excl = false) = %x{ return &Range[X]{b: b, e: e, excl: bool(excl)} }

  include Enumerable #[E]

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) { rbRangeEach(self, yield) }
  }

  # Blockless: Enumerable's (Range has its own map, so Enumerable's twin does not reach it; #155).
  #: () -> Enumerator::Map[E]
  def __map_enum = Enumerator::Map.__of(to_a, inspect)

  # Enumerable's, with an Integer range's result sized up front, at most 2^20 (a block may break early; decision 342).
  #: [U] () { (E) -> U } -> Array[U]
  def map = %x{
    n := 0
    if b, ok := any(self.b).(Integer); ok && !self.endless && !self.beginless {
      e := any(self.e).(Integer)
      if self.excl {
        e-- // MinInt wraps to MaxInt: capped below like any huge range
      }
      if e >= b {
        n = int(min(uint64(e)-uint64(b), 1<<20-1)) + 1
      }
    }
    out := &Array[U]{s: make([]U, 0, n)}
    rbRangeEach(self, func(x E) bool {
      out.s = append(out.s, blk(x))
      return true
    })
    return out
  }

  #: () { (E) -> void } -> void
  def reverse_each = %x{
    return func(yield func(E) bool) {
      if self.beginless {
        // Ruby 3.3+: a beginless Integer range counts down forever.
        e, ok := any(self.e).(Integer)
        if !ok {
          rbRangeNoBegin(self)
        }
        if self.excl {
          if e == math.MinInt {
            rbIntOverflow(e, "-", 1)
          }
          e--
        }
        for i := e; ; i-- {
          if !yield(any(i).(E)) {
            return
          }
          if i == math.MinInt { // i-- would wrap into a Bignum
            rbIntOverflow(i, "-", 1)
          }
        }
      }
      if self.endless {
        panic(NewTypeError(Ref(String("can't iterate from NilClass"))))
      }
      b, bok := any(self.b).(Integer)
      e, eok := any(self.e).(Integer)
      if !bok || !eok {
        xs := []E{}
        rbRangeEach(self, func(x E) bool { xs = append(xs, x); return true })
        for i := len(xs) - 1; i >= 0; i-- {
          if !yield(xs[i]) {
            return
          }
        }
        return
      }
      if self.excl {
        if e == math.MinInt {
          return
        }
        e--
      }
      for i := e; i >= b; i-- {
        if !yield(any(i).(E)) || i == math.MinInt { // i-- would wrap
          return
        }
      }
    }
  }

  # A numeric range steps as its ArithmeticSequence would (MRI's float counting, backward for a negative step).
  #: (?Integer) { (E) -> void } -> void
  def step(n = 1) = %x{
    return func(yield func(E) bool) {
      _, num := any(self.e).(Integer)
      switch any(self.b).(type) {
      case Integer, Float:
        num = true
      }
      if self.beginless && !num {
        panic(NewArgumentError(Ref(String("#step iteration for beginless ranges is meaningless"))))
      }
      if n == 0 {
        if !num {
          return // MRI 3.4: a zero step over a bounded non-numeric range yields nothing
        }
        panic(NewArgumentError(Ref(String("step can't be 0"))))
      }
      if a := rbArithRange(self, n, "step"); a.numeric {
        rbArithSeq(a)(yield)
        return
      }
      i := 0
      rbRangeEach(self, func(x E) bool {
        ok := i%int(n) != 0 || yield(x)
        i++
        return ok
      })
    }
  }

  # A Float step yields Floats whatever the bounds' class, as MRI's.
  #: (Float) { (Float) -> void } -> void
  def __step_float(by) = %x{
    return func(yield func(Float) bool) {
      if by == 0 {
        panic(NewArgumentError(Ref(String("step can't be 0"))))
      }
      rbRangeNoBegin(self)
      b := rbNumFloat(any(self.b))
      if self.endless {
        for i := 0.0; yield(Float(b + i*float64(by))); i++ {
        }
        return
      }
      rbFloatStepX(b, rbNumFloat(any(self.e)), float64(by), self.excl, yield)
    }
  }

  #: (Integer) -> Enumerator::ArithmeticSequence[E]
  def %(n) = %x{ return rbArithRange(self, n, "%") }

  #: (?Integer) -> Enumerator::ArithmeticSequence[E]
  def __step_enum(n = 1) = %x{ return rbArithRange(self, n, "step") }

  # A Float step's sequence is Floats whatever the bounds' class, as MRI's.
  #: (Float) -> Enumerator::ArithmeticSequence[Float]
  def __step_enum_float(by) = %x{ return rbArithRangeFloat(self, by) }

  #: () -> E
  def begin = %x{ self.b }

  #: () -> E
  def __first_0 = %x{
    if self.beginless {
      panic(NewRangeError(Ref(String("cannot get the first element of beginless range"))))
    }
    return self.b
  }

  #: () -> E
  def end = %x{
    if self.inf {
      panic(NewRangeError(Ref(String("rugaroo: the end of an Integer range to Float::INFINITY is Infinity, which a Range[Integer] cannot hold (decision 140)"))))
    }
    return self.e
  }

  #: () -> E
  def last = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the last element of endless range"))))
    }
    return self.e
  }

  #: (Integer) -> Array[E]
  def __last_1(n)
    raise RangeError, "cannot get the last element of endless range" if __endless?

    to_a.last(n)
  end

  #: () -> bool
  def exclude_end? = %x{ Boolean(self.excl) }

  #: () -> bool
  def __endless? = %x{ Boolean(self.endless) }

  #: (E) -> bool
  def cover?(v) = %x{ Boolean(rbRangeCover(self, v)) }

  #: (E) -> bool
  def include?(v) = %x{ Boolean(rbRangeInclude(self, v)) }

  #: (E) -> bool
  def member?(v) = %x{ Boolean(rbRangeInclude(self, v)) }

  # case/when: an incomparable value (another class, nil) is simply not covered.
  #: (untyped) -> bool
  def ===(v) = %x{
    x, ok := rbUnbox(v).(E)
    return Boolean(ok && rbRangeCover(self, x))
  }

  #: () -> Integer
  def size = %x{
    rbRangeNoBegin(self)
    b, ok := rbUnbox(any(self.b)).(Integer)
    if !ok {
      panic(NewTypeError(Ref(String("can't iterate from " + rbClassName(self.b)))))
    }
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the size of endless range (rugaroo has no Infinity Integer)"))))
    }
    return rbRangeIntCount(b, any(self.e).(Integer), self.excl)
  }

  #: () -> Integer
  def count = %x{
    if b, ok := rbUnbox(any(self.b)).(Integer); ok && !self.endless && !self.beginless {
      return rbRangeIntCount(b, any(self.e).(Integer), self.excl)
    }
    n := Integer(0)
    rbRangeEach(self, func(E) bool { n++; return true })
    return n
  }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def sum = %x{
    b, bok := any(self.b).(Integer)
    e, eok := any(self.e).(Integer)
    if !bok || !eok || self.endless || self.beginless {
      panic(NewTypeError(Ref(String("rugaroo: Range#sum needs an Integer range"))))
    }
    if self.excl {
      e--
    }
    if e < b {
      return 0
    }
    return rbRangeIntSum(b, e)
  }

  #: () -> E?
  def min = %x{
    if self.beginless {
      panic(NewRangeError(Ref(String("cannot get the minimum of beginless range"))))
    }
    if self.endless {
      return &self.b
    }
    if c := rbCmp(self.b, self.e); c > 0 || (c == 0 && self.excl) {
      return nil
    }
    return &self.b
  }

  # Range's own minmax (Ruby 2.7): from the ends, never by iterating. An Integer range to Float::INFINITY (decision 140)
  # answers Infinity as its maximum, which a Range[Integer]'s `max` cannot hold, so the maximum is untyped (rubocop-ast's
  # NodePattern arity ranges: `(0..Float::INFINITY).minmax`); the block form is Enumerable's __minmax_block. The minimum is
  # untyped too, since `[E?, untyped]` renders E? as *E in the generic body but rbOpt[E] at an Integer call.
  #: () -> [untyped, untyped]
  def minmax
    return [min, Float::INFINITY] if __to_infinity?

    [min, max]
  end

  #: () -> bool
  def __to_infinity? = %x{ Boolean(self.inf) }

  #: () -> E?
  def max = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot get the maximum of endless range"))))
    }
    if c := rbCmp(self.b, self.e); !self.beginless && (c > 0 || (c == 0 && self.excl)) {
      return nil
    }
    if !self.excl {
      return &self.e
    }
    e, ok := any(self.e).(Integer)
    if !ok {
      panic(NewTypeError(Ref(String("cannot exclude non Integer end value"))))
    }
    x := any(e - 1).(E)
    return &x
  }

  # Enumerable's min(n)/max(n) sort every element, which an endless range
  # never finishes: min(n) takes the first n, max(n) raises, as MRI's.
  #: (Integer) -> Array[E]
  def __min_1(n)
    raise RangeError, "cannot get the minimum of beginless range" if __beginless?
    return super unless __endless?
    raise ArgumentError, "negative array size (or size too big)" if n < 0

    first(n)
  end

  #: (Integer) -> Array[E]
  def __max_1(n)
    raise RangeError, "cannot get the maximum of endless range" if __endless?
    return super unless __beginless?
    raise ArgumentError, "negative array size (or size too big)" if n < 0

    out = [] #: Array[E]
    return out if n == 0

    reverse_each do |x|
      out << x
      break if out.size == n
    end
    out
  end

  #: () -> bool
  def __beginless? = %x{ Boolean(self.beginless) }

  #: () -> Array[E]
  def to_a = %x{
    if self.endless {
      panic(NewRangeError(Ref(String("cannot convert endless range to an array"))))
    }
    out := &Array[E]{}
    rbRangeEach(self, func(x E) bool { out.s = append(out.s, x); return true })
    return out
  }

  #: () -> Array[E]
  def entries = to_a

  #: () -> String
  def to_s = %x{ rbRangeStr(self, rbToS) }

  #: () -> String
  def inspect = %x{ rbRangeStr(self, rbInspect) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Range[E])
    if !ok {
      // Another instantiation ((1..2) == (1.0..2.0)): compare the untyped views.
      if a, ok := other.(Range_Any); ok {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    return Boolean(o.excl == self.excl && o.endless == self.endless && o.beginless == self.beginless && o.inf == self.inf && bool(rbEq(o.b, self.b)) && bool(rbEq(o.e, self.e)))
  }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := other.(*Range[E])
    return Boolean(ok && o.excl == self.excl && o.endless == self.endless && o.beginless == self.beginless && o.inf == self.inf && rbKeyEql(o.b, self.b) && rbKeyEql(o.e, self.e))
  }

  #: () -> Integer
  def hash = %x{
    h := rbHash(self.b)*31 + rbHash(self.e)
    if self.excl {
      h++
    }
    return h
  }

  # Immediate-like: literal ranges are frozen in MRI.
  #: () -> bool
  def frozen? = true

  # [start, length] within size n; nil when start is out of bounds, as MRI.
  #: (Integer) -> [Integer, Integer]?
  def __slice(n) = %x{
    b, ok := rbUnbox(any(self.b)).(Integer)
    if self.beginless {
      b, ok = 0, true
    }
    if !ok {
      panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(self.b) + " into Integer"))))
    }
    e := n
    if !self.endless {
      e = rbUnbox(any(self.e)).(Integer)
      if e < 0 {
        e += n
      }
      if !self.excl {
        e++
      }
    }
    if b < 0 {
      b += n
    }
    if b < 0 || b > n {
      return nil
    }
    e = min(e, n)
    return &Tuple2[Integer, Integer]{b, max(e-b, 0)}
  }

  # [start, count] for writing the range into an array of size n (`[]=`, `fill`): a negative start counts from the end, nil when still negative (the caller raises RangeError, so its frame is the one MRI shows). Unlike __slice, a start or end past n stands: the array grows.
  #: (Integer) -> [Integer, Integer]?
  def __span(n) = %x{
    b, ok := rbUnbox(any(self.b)).(Integer)
    if self.beginless {
      b, ok = 0, true
    }
    if !ok {
      panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(self.b) + " into Integer"))))
    }
    if b < 0 {
      b += n
    }
    if b < 0 {
      return nil
    }
    e := n
    if !self.endless {
      e = rbUnbox(any(self.e)).(Integer)
      if e < 0 {
        e += n
      }
      if !self.excl {
        e++
      }
    }
    return &Tuple2[Integer, Integer]{b, max(e-b, 0)}
  }

  #: () -> Range[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Range[any]); ok {
      return same
    }
    return &Range[any]{b: rbUnbox(self.b), e: rbUnbox(self.e), excl: self.excl, endless: self.endless, beginless: self.beginless, inf: self.inf}
  }

  #: (Range[E]) -> bool
  def overlap?(other) = %x{ Boolean(rbRangeOverlap(self, other)) }

  # Integer ranges only: the smallest value the block is true for (find-minimum mode).
  #: () { (E) -> bool } -> E?
  def bsearch = %x{ return rbRangeBsearch(self, func(x E) bool { return bool(blk(x)) }) }
end
