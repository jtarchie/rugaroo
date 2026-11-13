# rbs_inline: enabled

# Decision 140: an iter.Seq plus what inspect, size and StopIteration#result need; next/peek pull it with iter.Pull, no goroutine.

# @rbs generic E
# @go_type struct { seq func(func(E) bool); recv any; meth string; size func() *Integer; res *any; ext rbExt[E] }
class Enumerator < Object
  include Enumerable #[E]

  # The element type is what the block feeds `y` (the compiler probes it, decision 140) or the assignment's annotation.
  #: [X] () { (Enumerator::Yielder[X]) -> void } -> Enumerator[X]
  def self.new = %x{ return rbEnumNew[X](nil, blk) }

  #: [X] (Integer?) { (Enumerator::Yielder[X]) -> void } -> Enumerator[X]
  def self.__new_1(size) = %x{ return rbEnumNew[X](rbOptToPtr(size), blk) }

  #: () { (E) -> void } -> void
  def each = %x{ return func(yield func(E) bool) { self.seq(yield) } }

  # An Enumerator's own enumerator is itself (rubocop-ast's `each_child_node(:pair).to_enum`; decision 444).
  #: () -> Enumerator[E]
  def to_enum = self

  #: () -> Enumerator[E]
  def enum_for = self

  #: () -> self
  def __each_enum = self

  # Raises StopIteration at the end, with the iteration's result.
  #: () -> E
  def next = %x{ return self.ext.next(self.seq, func() any { return rbEnumResult(self.res, self.recv) }) }

  #: () -> E
  def peek = %x{ return self.ext.peek(self.seq, func() any { return rbEnumResult(self.res, self.recv) }) }

  #: () -> self
  def rewind = %x{
    self.ext.rewind()
    return self
  }

  # nil when it cannot be known without iterating, as MRI.
  #: () -> Integer?
  def size = %x{
    if self.size == nil {
      return rbOpt[Integer]{}
    }
    return rbOptFromPtr(self.size())
  }

  #: (?Integer) { (E, Integer) -> void } -> void
  def with_index(offset = 0) = %x{
    return func(yield func(E, Integer) bool) {
      i := offset
      for x := range self.seq {
        if !yield(x, i) {
          return
        }
        i++
      }
    }
  }

  #: [A] (A) { (E, A) -> void } -> A
  def with_object(memo, &block) = each_with_object(memo, &block)

  #: (?Integer) -> Enumerator[[E, Integer]]
  def __with_index_enum(offset = 0) = %x{
    meth := "with_index"
    if offset != 0 { // ponytail: an explicit with_index(0) still inspects bare; MRI shows "(0)"
      meth += "(" + string(rbInspect(offset)) + ")"
    }
    e := rbEnumOf(rbWithIndex(self.seq, offset), any(self), meth, self.size, nil)
    seq := e.seq
    e.seq = func(yield func(Tuple2[E, Integer]) bool) {
      seq(yield)
      r := rbEnumResult(self.res, self.recv) // read after the run: Enumerator.new sets its block's value then
      e.res = &r
    }
    return e
  }

  # @self Enumerator[[K, V]]
  #: [K, V] () -> Hash[K, V]
  def to_h = to_a.to_h

  # @self Enumerator[Array[U]]
  #: [U] () -> Hash[U, U]
  def __to_h_arrays = to_a.to_h

  #: () -> String
  def inspect = %x{ return rbEnumInspect(self.recv, self.meth) }

  #: () -> Enumerator[untyped]
  def _to_any = %x{ return rbEnumAny(self) }

  #: () -> String
  def to_s = inspect

  # @rbs generic E
  # @go_type struct { fn func(E) }
  class Yielder < Object
    #: (E) -> self
    def <<(x) = %x{
      self.fn(x)
      return self
    }

    #: (E) -> nil
    def yield(x) = %x{ self.fn(x) }

    #: () -> ^(E) -> void
    def to_proc = %x{
      fn := self.fn
      return &fn
    }

    #: () -> Yielder[untyped]
    def _to_any = %x{ return &Enumerator_Yielder[any]{fn: func(x any) { self.fn(x.(E)) }} }
  end

  # Blockless map/select/reject: their with_index maps or filters, as MRI's does.
  # @rbs generic E
  # @go_type struct { items *Array[E]; ext rbExt[E]; shown string }
  class Map < Object
    #: [X] (Array[X]) -> Map[X]
    def self.new(items) = %x{ return &Enumerator_Map[X]{items: items} }

    # map.with_index(off) without a block: an Enumerator of [element, index] pairs (#155).
    #: (?Integer) -> Enumerator[[E, Integer]]
    def __with_index_enum(offset = 0) = %x{
      meth := "with_index"
      if offset != 0 {
        meth += "(" + string(rbInspect(offset)) + ")"
      }
      return rbEnumOf(rbWithIndex(Array_Each(self.items), offset), any(self), meth, func() *Integer { n := Integer(len(self.items.s)); return &n }, nil)
    }

    # An Enumerable's blockless map: the elements, and the receiver's inspect, which MRI's shows (decision 450).
    #: [X] (Array[X], String) -> Map[X]
    def self.__of(items, shown) = %x{ return &Enumerator_Map[X]{items: items, shown: string(shown)} }

    #: () -> String
    def __shown = %x{ return String(self.shown) }

    #: [U] (?Integer) { (E, Integer) -> U } -> Array[U]
    def with_index(offset = 0) = %x{
      out := &Array[U]{}
      for i, x := range self.items.s {
        out.s = append(out.s, blk(x, Integer(i)+offset))
      }
      return out
    }

    #: [U] () { (E, Integer) -> U } -> Array[U]
    def each_with_index = %x{
      out := &Array[U]{}
      for i, x := range self.items.s {
        out.s = append(out.s, blk(x, Integer(i)))
      }
      return out
    }

    #: () -> E
    def next = %x{ return self.ext.next(Array_Each(self.items), func() any { return self.items }) }

    #: () -> E
    def peek = %x{ return self.ext.peek(Array_Each(self.items), func() any { return self.items }) }

    #: () -> self
    def rewind = %x{
      self.ext.rewind()
      return self
    }

    #: () -> Integer
    def size = %x{ return Integer(len(self.items.s)) }

    #: () -> Array[E]
    def to_a = %x{ self.items }

    #: () -> Map[untyped]
    def _to_any = %x{ return &Enumerator_Map[any]{items: self.items._ToAny()} }

    #: () -> String
    def inspect = "#<Enumerator: #{__shown.empty? ? to_a.inspect : __shown}:map>"

    #: () -> String
    def to_s = inspect
  end

  # @rbs generic E
  # @go_type struct { items *Array[E]; negate bool; name string; ext rbExt[E]; shown string }
  class Select < Object
    #: [X] (Array[X], bool, String) -> Select[X]
    def self.new(items, negate, name) = %x{ return &Enumerator_Select[X]{items: items, negate: bool(negate), name: string(name)} }

    #: [X] (Array[X], bool, String, String) -> Select[X]
    def self.__of(items, negate, name, shown) = %x{ return &Enumerator_Select[X]{items: items, negate: bool(negate), name: string(name), shown: string(shown)} }

    #: () -> String
    def __shown = %x{ return String(self.shown) }

    #: (?Integer) { (E, Integer) -> bool } -> Array[E]
    def with_index(offset = 0) = %x{
      out := &Array[E]{}
      for i, x := range self.items.s {
        if bool(blk(x, Integer(i)+offset)) != self.negate {
          out.s = append(out.s, x)
        }
      }
      return out
    }

    #: () { (E, Integer) -> bool } -> Array[E]
    def each_with_index = %x{
      out := &Array[E]{}
      for i, x := range self.items.s {
        if bool(blk(x, Integer(i))) != self.negate {
          out.s = append(out.s, x)
        }
      }
      return out
    }

    #: () -> E
    def next = %x{ return self.ext.next(Array_Each(self.items), func() any { return self.items }) }

    #: () -> E
    def peek = %x{ return self.ext.peek(Array_Each(self.items), func() any { return self.items }) }

    #: () -> self
    def rewind = %x{
      self.ext.rewind()
      return self
    }

    #: () -> Integer
    def size = %x{ return Integer(len(self.items.s)) }

    #: () -> Array[E]
    def to_a = %x{ self.items }

    #: () -> Select[untyped]
    def _to_any = %x{ return &Enumerator_Select[any]{items: self.items._ToAny(), negate: self.negate, name: self.name} }

    #: () -> String
    def inspect = "#<Enumerator: #{__shown.empty? ? to_a.inspect : __shown}:#{__name}>"

    #: () -> String
    def to_s = inspect

    #: () -> String
    def __name = %x{ String(self.name) }
  end
end

class Enumerator
  # Each step wraps the sequence before it, so nothing runs until first/to_a/each pulls (decision 140).
  # @rbs generic E
  # @go_type struct { seq func(func(E) bool); src any; meth string }
  class Lazy < Object
    include Enumerable #[E]

    #: () { (E) -> void } -> void
    def each = %x{ return func(yield func(E) bool) { self.seq(yield) } }

    #: [U] () { (E) -> U } -> Lazy[U]
    def map = %x{
      return rbLazy(func(yield func(U) bool) {
        for x := range self.seq {
          if !yield(blk(x)) {
            return
          }
        }
      }, any(self), "map")
    }

    #: [U] () { (E) -> U } -> Lazy[U]
    def collect = %x{ return rbLazy(Enumerator_Lazy_Map[E, U](self, blk).seq, any(self), "collect") }

    #: () { (E) -> bool } -> Lazy[E]
    def select = %x{ return rbLazyFilter(self, func(x E) bool { return bool(blk(x)) }, "select") }

    #: () { (E) -> bool } -> Lazy[E]
    def filter = %x{ return rbLazyFilter(self, func(x E) bool { return bool(blk(x)) }, "filter") }

    #: () { (E) -> bool } -> Lazy[E]
    def find_all = %x{ return rbLazyFilter(self, func(x E) bool { return bool(blk(x)) }, "find_all") }

    # grep/grep_v keep `select`'s source and name the step as MRI does (`grep(2..5)`).
    # @dynamic
    #: (untyped) -> Lazy[E]
    def grep(pattern) = select { |x| pattern === x }.__relabel(self, "grep(#{pattern.inspect})")

    # @dynamic
    #: (untyped) -> Lazy[E]
    def grep_v(pattern) = reject { |x| pattern === x }.__relabel(self, "grep_v(#{pattern.inspect})")

    # @dynamic
    #: [U] (untyped) { (E) -> U } -> Lazy[U]
    def __grep_block(pattern) = select { |x| pattern === x }.map { |x| yield(x) }.__relabel(self, "grep(#{pattern.inspect})")

    # @dynamic
    #: [U] (untyped) { (E) -> U } -> Lazy[U]
    def __grep_v_block(pattern) = reject { |x| pattern === x }.map { |x| yield(x) }.__relabel(self, "grep_v(#{pattern.inspect})")

    # The same sequence as a step of `src` called `meth`.
    #: (untyped, String) -> self
    def __relabel(src, meth) = %x{ return rbLazy(self.seq, src, string(meth)) }

    #: () { (E) -> bool } -> Lazy[E]
    def reject = %x{ return rbLazyFilter(self, func(x E) bool { return !bool(blk(x)) }, "reject") }

    #: [U] () { (E) -> U? } -> Lazy[U]
    def filter_map = %x{
      return rbLazy(func(yield func(U) bool) {
        for x := range self.seq {
          if v := blk(x); v != nil && !yield(*v) {
            return
          }
        }
      }, any(self), "filter_map")
    }

    #: [U] () { (E) -> Array[U] } -> Lazy[U]
    def flat_map = %x{
      return rbLazy(func(yield func(U) bool) {
        for x := range self.seq {
          for _, y := range blk(x).s {
            if !yield(y) {
              return
            }
          }
        }
      }, any(self), "flat_map")
    }

    #: [U] () { (E) -> Array[U] } -> Lazy[U]
    def collect_concat = %x{ return rbLazy(Enumerator_Lazy_FlatMap[E, U](self, blk).seq, any(self), "collect_concat") }

    # Lazy, so a chunk goes out as the next one starts and an infinite source works.
    #: () { (E, E) -> bool } -> Lazy[Array[E]]
    def chunk_while = %x{
      return rbLazy(func(yield func(*Array[E]) bool) {
        var cur *Array[E]
        var prev E
        for x := range self.seq {
          if cur != nil && bool(blk(prev, x)) {
            cur.s = append(cur.s, x)
          } else {
            if cur != nil && !yield(cur) {
              return
            }
            cur = &Array[E]{s: []E{x}}
          }
          prev = x
        }
        if cur != nil {
          yield(cur)
        }
      }, any(self), "chunk_while")
    }

    #: () { (E, E) -> bool } -> Lazy[Array[E]]
    def slice_when = %x{
      return rbLazy(func(yield func(*Array[E]) bool) {
        var cur *Array[E]
        var prev E
        for x := range self.seq {
          if cur != nil && !bool(blk(prev, x)) {
            cur.s = append(cur.s, x)
          } else {
            if cur != nil && !yield(cur) {
              return
            }
            cur = &Array[E]{s: []E{x}}
          }
          prev = x
        }
        if cur != nil {
          yield(cur)
        }
      }, any(self), "slice_when")
    }

    #: (Integer) -> Lazy[E]
    def take(n) = %x{
      if n < 0 {
        panic(NewArgumentError(Ref(String("attempt to take negative size"))))
      }
      return rbLazy(func(yield func(E) bool) {
        if n == 0 {
          return
        }
        i := Integer(0)
        for x := range self.seq {
          if !yield(x) {
            return
          }
          if i++; i >= n {
            return
          }
        }
      }, any(self), "take("+string(rbInspect(n))+")")
    }

    #: () { (E) -> bool } -> Lazy[E]
    def take_while = %x{
      return rbLazy(func(yield func(E) bool) {
        for x := range self.seq {
          if !bool(blk(x)) || !yield(x) {
            return
          }
        }
      }, any(self), "take_while")
    }

    #: (Integer) -> Lazy[E]
    def drop(n) = %x{
      if n < 0 {
        panic(NewArgumentError(Ref(String("attempt to drop negative size"))))
      }
      return rbLazy(func(yield func(E) bool) {
        i := Integer(0)
        for x := range self.seq {
          if i < n {
            i++
            continue
          }
          if !yield(x) {
            return
          }
        }
      }, any(self), "drop("+string(rbInspect(n))+")")
    }

    #: () { (E) -> bool } -> Lazy[E]
    def drop_while = %x{
      return rbLazy(func(yield func(E) bool) {
        dropping := true
        for x := range self.seq {
          if dropping && bool(blk(x)) {
            continue
          }
          dropping = false
          if !yield(x) {
            return
          }
        }
      }, any(self), "drop_while")
    }

    #: [U] (Array[U]) -> Lazy[[E, U?]]
    def zip(other) = %x{
      return rbLazy(func(yield func(Tuple2[E, *U]) bool) {
        i := 0
        for x := range self.seq {
          var y *U
          if i < len(other.s) {
            v := other.s[i] // a copy, not the slot (decision 397)
            y = &v
          }
          i++
          if !yield(Tuple2[E, *U]{x, y}) {
            return
          }
        }
      }, any(self), "zip")
    }

    #: (?Integer) -> Lazy[[E, Integer]]
    def with_index(offset = 0) = %x{ return rbLazy(rbWithIndex(self.seq, offset), any(self), "with_index") }

    # The elements pass on unchanged, as MRI's: the block only sees each with its index.
    #: (?Integer) { (E, Integer) -> void } -> Lazy[E]
    def __with_index_block(offset = 0) = %x{
      return rbLazy(func(yield func(E) bool) {
        i := offset
        for x := range self.seq {
          blk(x, i)
          i++
          if !yield(x) {
            return
          }
        }
      }, any(self), "with_index")
    }

    #: () -> Lazy[[E, Integer]]
    def __each_with_index_enum = %x{ return rbLazy(rbWithIndex(self.seq, 0), any(self), "each_with_index") }

    #: () -> Lazy[E]
    def uniq = %x{
      return rbLazy(func(yield func(E) bool) {
        seen := rbNewKeySet[E](nil)
        for x := range self.seq {
          if seen.has(x) {
            continue
          }
          seen.put(x)
          if !yield(x) {
            return
          }
        }
      }, any(self), "uniq")
    }

    # @self Lazy[U?]
    #: [U] () -> Lazy[U]
    def compact = %x{
      return rbLazy(func(yield func(U) bool) {
        for x := range self.seq {
          if x != nil && !yield(*x) {
            return
          }
        }
      }, any(self), "compact")
    }

    #: () -> Array[E]
    def force = to_a

    #: () -> Enumerator[E]
    def eager = %x{ return rbEnumOf(self.seq, any(self), "each", nil, nil) }

    #: () -> self
    def lazy = self

    #: () -> String
    def inspect = %x{
      if self.meth == "" {
        return "#<Enumerator::Lazy: " + rbInspect(self.src) + ">"
      }
      return "#<Enumerator::Lazy: " + rbInspect(self.src) + ":" + String(self.meth) + ">"
    }

    #: () -> String
    def to_s = inspect

    #: () -> Lazy[untyped]
    def _to_any = %x{ return rbLazy(rbSeqAny(self.seq), self.src, self.meth) }
  end
end

class Enumerator
  # Range#% / Range#step / Integer#step / Float#step without a block (decision 140); not an Enumerator subclass, since a @go_type class has no @go_type parent.
  # @rbs generic E
  # @go_type struct { b E; e E; by E; nth Integer; excl bool; endless bool; numeric bool; rng *Range[E]; insp string; ext rbExt[E] }
  class ArithmeticSequence < Object
    include Enumerable #[E]

    #: () { (E) -> void } -> void
    def each = %x{ return func(yield func(E) bool) { rbArithSeq(self)(yield) } }

    #: () -> E
    def next = %x{ return self.ext.next(rbArithSeq(self), func() any { return self }) }

    #: () -> E
    def peek = %x{ return self.ext.peek(rbArithSeq(self), func() any { return self }) }

    #: () -> self
    def rewind = %x{
      self.ext.rewind()
      return self
    }

    #: () -> E
    def begin = %x{ self.b }

    #: () -> E?
    def end = %x{
      if self.endless {
        return nil
      }
      return &self.e
    }

    #: () -> E
    def step = %x{ self.by }

    #: () -> bool
    def exclude_end? = %x{ Boolean(self.excl) }

    #: () -> Integer
    def size = %x{
      if self.endless {
        panic(NewRangeError(Ref(String("cannot get the size of an endless arithmetic sequence (rugaroo has no Infinity Integer)"))))
      }
      return *self.rbSize()
    }

    #: () -> Integer
    def count = size

    #: () -> E?
    def last = %x{
      all := rbArithAll(self)
      if len(all) == 0 {
        return nil
      }
      return &all[len(all)-1]
    }

    #: (Integer) -> Array[E]
    def __last_1(n) = %x{
      all := rbArithAll(self)
      if n < 0 {
        panic(NewArgumentError(Ref(String("negative array size"))))
      }
      return &Array[E]{s: slices.Clone(all[max(len(all)-int(n), 0):])}
    }

    #: (untyped) -> bool
    def ==(other) = %x{
      o, ok := other.(*Enumerator_ArithmeticSequence[E])
      return Boolean(ok && o.b == self.b && o.e == self.e && o.by == self.by && o.nth == self.nth && o.excl == self.excl && o.endless == self.endless && o.numeric == self.numeric)
    }

    #: () -> String
    def inspect = %x{ String(self.insp) }

    #: () -> ArithmeticSequence[untyped]
    def _to_any = %x{
      out := &Enumerator_ArithmeticSequence[any]{b: self.b, e: self.e, by: self.by, nth: self.nth, excl: self.excl, endless: self.endless, numeric: self.numeric, insp: self.insp}
      if self.rng != nil {
        out.rng = self.rng._ToAny()
      }
      return out
    }
  end
end
