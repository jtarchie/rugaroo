# prelude/array.rb
# rbs_inline: enabled

#
# Array as a Go struct holding its slice and its frozen flag (decision 96), always handled as a pointer.

# Array is mutable and aliased in Ruby, so it is always handled as a pointer.
# @rbs generic E
# @go_type struct { s []E; frozen atomic.Bool }
class Array < Object
  include Enumerable #[E]

  # An Array's elements as an untyped Array, else nil. Untyped, not Array[untyped]?: an optional Array type would add
  # arms to every program's unbox switches.
  #: (untyped) -> untyped
  def self.try_convert(obj) = %x{
    if a, ok := rbUnbox(obj).(Array_Any); ok {
      return a._ToAny()
    }
    return nil
  }

  # Reached only by an Array subclass's `super(items)` (decision 202): the subclass starts with items' elements.
  #: (?Array[E]?) -> void
  def initialize(items = nil) = %x{
    self.rbCheckFrozen()
    if items != nil {
      self.s = append(self.s[:0:0], (*items).s...)
    } else {
      self.s = self.s[:0:0]
    }
  }

  # Every slot holds the same v, as MRI's (mutating a shared Array element shows in all).
  #: [X] (Integer, X) -> Array[X]
  def self.new(n, v) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative array size"))))
    }
    if rbAllocTooBig(int(n), 8) {
      panic(NewArgumentError(Ref(String("array size too big"))))
    }
    out := &Array[X]{s: make([]X, n)}
    for i := range out.s {
      out.s[i] = v
    }
    return out
  }

  # `Array.new(n) { }` grows one element at a time, so the size is checked up front as `Array.new(n, v)` does.
  #: (Integer) -> void
  def self.__check_size(n) = %x{
    if rbAllocTooBig(int(n), 8) {
      panic(NewArgumentError(Ref(String("array size too big"))))
    }
  }

  # Without a block, n nils (elftools' LazyArray): X is what the caller holds them as.
  #: [X] (Integer) ?{ (Integer) -> X } -> Array[X]
  def self.__new_1(n)
    raise ArgumentError, "negative array size" if n < 0

    __check_size(n)
    out = [] #: Array[X]
    if block_given?
      n.times { |i| out << yield(i) }
    else
      __zeros(out, n)
    end
    out
  end

  #: [X] (Array[X], Integer) -> void
  def self.__zeros(out, n) = %x{ out.s = make([]X, n) }

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) {
      // An index loop re-reading the length, as MRI's: the block may
      // append, delete or clear. (Not `range len`: that reads it once.)
      for i := 0; ; i++ {
        if i >= len(self.s) || !yield(self.s[i]) {
          return
        }
      }
    }
  }

  # MRI reads self at each step but other once, before the first (its take_items), so a store into other is not seen (decision 389).
  #: [U] (Array[U]) { ([E, U?]) -> void } -> void
  def __zip_block(other) = %x{
    return func(yield func(Tuple2[E, *U]) bool) {
      ys := rbZipTake(self.s, other.s)
      for i := 0; i < len(self.s); i++ {
        if !yield(rbZipPair(self.s, ys, i)) {
          return
        }
      }
    }
  }

  # A statement `xs.zip(ys).each { }`: MRI's pairs are built before each runs, so both are copies taken now (decision 389).
  #: [U] (Array[U]) { ([E, U?]) -> void } -> void
  def __zip_each(other) = %x{
    return func(yield func(Tuple2[E, *U]) bool) {
      xs := slices.Clone(self.s)
      ys := rbZipTake(xs, other.s)
      for i := range xs {
        if !yield(rbZipPair(xs, ys, i)) {
          return
        }
      }
    }
  }

  # Copies taken now, as MRI's zip reads both Arrays before the consumer runs; no Array of pairs (decision 389).
  #: [U] (Array[U]) -> Enumerator[[E, U?]]
  def __zip_view_enum(other) = %x{
    xs, ys := slices.Clone(self.s), slices.Clone(other.s)
    return rbEnumOf(func(yield func(Tuple2[E, *U]) bool) {
      for i, x := range xs {
        var u *U
        if i < len(ys) {
          u = &ys[i]
        }
        if !yield(Tuple2[E, *U]{x, u}) {
          return
        }
      }
    }, any(self), "each", nil, nil)
  }

  #: (E) -> Integer?
  def index(v)
    each_with_index { |x, i| return i if x == v }
    nil
  end

  #: () { (E) -> bool } -> Integer?
  def __index_block
    each_with_index { |x, i| return i if yield(x) }
    nil
  end

  #: (E) -> Integer?
  def find_index(v) = index(v)

  alias __find_index_block __index_block

  #: (E) -> Integer?
  def rindex(v)
    i = size - 1
    while i >= 0
      return i if self[i] == v

      i -= 1
    end
    nil
  end

  #: () { (E, Integer) -> void } -> void
  def each_with_index = %x{
    return func(yield func(E, Integer) bool) {
      for i := 0; ; i++ {
        if i >= len(self.s) || !yield(self.s[i], Integer(i)) {
          return
        }
      }
    }
  }

  #: () { (E) -> void } -> void
  def reverse_each = %x{
    return func(yield func(E) bool) {
      for i := len(self.s) - 1; i >= 0; i-- {
        if !yield(self.s[i]) {
          return
        }
        i = min(i, len(self.s)) // the block may have shrunk the array: MRI clamps, it does not index past the end
      }
    }
  }

  #: (Integer) -> E?
  def [](i) = %x{
    if i < 0 {
      i += Integer(len(self.s))
    }
    if i < 0 || int(i) >= len(self.s) {
      return nil
    }
    x := self.s[i] // a copy: the slot's later writes must not show through (decision 397)
    return &x
  }

  #: (Range[Integer]) -> Array[E]?
  def __idx_range(r)
    s = r.__slice(size)
    return nil unless s

    __idx_2(s[0], s[1])
  end

  #: (Integer, Integer) -> Array[E]?
  def __idx_2(start, count) = %x{
    n := Integer(len(self.s))
    if start < 0 {
      start += n
    }
    if start < 0 || start > n || count < 0 {
      return nil
    }
    end := min(start+count, n)
    out := &Array[E]{s: make([]E, end-start)}
    copy(out.s, self.s[start:end])
    return Ref(out)
  }

  #: (Range[Integer]) -> Array[E]?
  def slice(r) = self[r]

  # The Range form of MRI's slice!: removes and returns self[r] (kramdown's `@warnings.slice!(0...n)`).
  # ponytail: slice!(i) and slice!(start, len) are not here; add them as overloads when a program needs them.
  #: (Range[Integer]) -> Array[E]?
  def slice!(r)
    s = r.__slice(size)
    return nil unless s

    out = __idx_2(s[0], s[1])
    __cut(s[0], s[1])
    out
  end

  #: (Integer, Integer) -> void
  def __cut(start, count) = %x{
    self.rbCheckFrozen()
    end := min(int(start+count), len(self.s))
    if start < 0 || int(start) >= end {
      return
    }
    self.s = append(self.s[:start], self.s[end:]...)
  }

  #: (Integer, E) -> E
  def []=(i, v)
    %x{
    if uint64(i) < uint64(len(self.s)) && !self.frozen.Load() { // in bounds: small enough for Go to inline (decision 387)
      self.s[i] = v
      return v
    }
    return rbArrayIdxSet(self, i, v)}
  end

  #: (Integer, ?E?) -> E
  def fetch(i, default = nil)
    v = self[i]
    return v if v
    return default if default

    raise IndexError, "index #{i} outside of array bounds: #{-size}...#{size}"
  end

  # fetch with a block: the block's value for an index outside the array.
  #: (Integer) { (Integer) -> E } -> E
  def __fetch_block(i) = i < size && i >= -size ? fetch(i) : yield(i)

  #: (E) -> self
  def <<(x) = %x{
    self.rbCheckFrozen()
    self.s = append(self.s, x)
    return self
  }

  #: (*E) -> self
  def push(*xs) = %x{
    self.rbCheckFrozen()
    self.s = append(self.s, rest_...)
    return self
  }

  #: (*E) -> self
  def append(*xs) = push(*xs)

  #: () -> E?
  def pop = %x{
    self.rbCheckFrozen()
    if len(self.s) == 0 {
      return nil
    }
    x := self.s[len(self.s)-1]
    self.s = self.s[:len(self.s)-1]
    return &x
  }

  #: () -> E?
  def shift = %x{
    self.rbCheckFrozen()
    if len(self.s) == 0 {
      return nil
    }
    x := self.s[0]
    self.s = self.s[1:]
    return &x
  }

  #: (Integer) -> Array[E]
  def __pop_1(n) = %x{
    self.rbCheckFrozen()
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative array size"))))
    }
    k := len(self.s) - min(int(n), len(self.s))
    out := &Array[E]{s: slices.Clone(self.s[k:])}
    self.s = self.s[:k]
    return out
  }

  #: (Integer) -> Array[E]
  def __shift_1(n) = %x{
    self.rbCheckFrozen()
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative array size"))))
    }
    k := min(int(n), len(self.s))
    out := &Array[E]{s: slices.Clone(self.s[:k])}
    self.s = self.s[k:]
    return out
  }

  #: (*E) -> self
  def unshift(*xs) = %x{
    self.rbCheckFrozen()
    self.s = append(slices.Clone(rest_), self.s...)
    return self
  }

  alias prepend unshift

  #: (Array[E]) -> self
  def concat(other) = %x{
    self.rbCheckFrozen()
    self.s = append(self.s, other.s...)
    return self
  }

  #: (Array[E]) -> Array[E]
  def +(other) = %x{
    out := &Array[E]{}
    out.s = append(append(out.s, self.s...), other.s...)
    return out
  }

  # Set operations match elements by eql?/hash, as MRI's.
  #: (Array[E]) -> Array[E]
  def -(other) = %x{
    drop := rbNewKeySet(other.s)
    out := &Array[E]{}
    for _, x := range self.s {
      if !drop.has(x) {
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: (Array[E]) -> Array[E]
  def difference(other) = self - other

  # pack with a literal format and no U directive: MRI's BINARY String (decision 365).
  # @binary
  #: (String) -> String
  def __pack_binary(format) = pack(format)

  # Every directive but the C pointers P and p (decision 138).
  #: (String) -> String
  def pack(format) = %x{
    items := make([]any, len(self.s))
    for i, x := range self.s {
      items[i] = x
    }
    return String(rbPack(items, string(format)))
  }

  #: (Array[E]) -> Array[E]
  def &(other) = %x{
    keep := rbNewKeySet(other.s)
    seen := rbNewKeySet[E](nil)
    out := &Array[E]{}
    for _, x := range self.s {
      if keep.has(x) && !seen.has(x) {
        seen.put(x)
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: (Array[E]) -> Array[E]
  def intersection(other) = self & other

  #: (Array[E]) -> bool
  def intersect?(other) = !(self & other).empty?

  #: (Array[E]) -> Array[E]
  def |(other) = (self + other).uniq

  #: (*Array[E]) -> Array[E]
  def union(*others) = others.reduce(uniq) { |acc, o| acc | o }

  #: (?Integer) -> Array[E]
  def rotate(n = 1)
    return [] if empty?

    k = n % size
    (self[k..] || []) + (self[0, k] || [])
  end

  #: [U] (Array[U]) -> Array[[E, U]]
  def product(other)
    out = [] #: Array[[E, U]]
    each { |x| other.each { |y| out << [x, y] } }
    out
  end

  # Find-minimum mode: the first element for which the block is true.
  #: () { (E) -> bool } -> E?
  def bsearch = %x{
    i := sort.Search(len(self.s), func(i int) bool { return bool(blk(self.s[i])) })
    if i == len(self.s) {
      return nil
    }
    x := self.s[i] // a copy: the slot's later writes must not show through (decision 397)
    return &x
  }

  #: () -> self
  def sort! = %x{
    self.rbCheckFrozen()
    rbSortStable(self.s, func(a, b E) int { return -int(rbCmp(b, a)) }) // (earlier, later), as MRI's failure names them
    return self
  }

  # sort! { |a, b| ... } (Homebrew's LinkageChecker#sort_by_formula_full_name!).
  #: () { (E, E) -> Integer } -> self
  def __sort_bang_block = %x{
    self.rbCheckFrozen()
    rbSortStable(self.s, func(a, b E) int { return int(blk(a, b)) })
    return self
  }

  #: [K] () { (E) -> K } -> self
  def sort_by! = %x{
    self.rbCheckFrozen()
    keys := make([]K, len(self.s))
    idx := make([]int, len(self.s))
    for i, x := range self.s {
      idx[i], keys[i] = i, blk(x)
    }
    cmp := rbCmpFunc[K]()
    rbSortStable(idx, func(a, b int) int { return -int(cmp(keys[b], keys[a])) })
    out := &Array[E]{s: make([]E, len(self.s))}
    for i, j := range idx {
      out.s[i] = self.s[j]
    }
    self.s = out.s
    return self
  }

  # Enumerable's, with the result sized up front (decision 342).
  #: [U] () { (E) -> U } -> Array[U]
  def map = %x{
    out := &Array[U]{s: make([]U, 0, len(self.s))}
    for i := 0; ; i++ { // re-reading the length, as each does: the block may change the receiver
      if i >= len(self.s) {
        break
      }
      out.s = append(out.s, blk(self.s[i]))
    }
    return out
  }

  #: () { (E) -> E } -> self
  def map! = %x{
    self.rbCheckFrozen()
    for i, x := range self.s {
      self.s[i] = blk(x)
    }
    return self
  }

  #: () { (E) -> E } -> self
  def collect!(&block) = map!(&block)

  #: () { (E) -> bool } -> self
  def keep_if = %x{
    self.rbCheckFrozen()
    out := self.s[:0]
    for _, x := range self.s {
      if bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear(self.s[len(out):])
    self.s = out
    return self
  }

  #: () { (E) -> bool } -> self
  def delete_if = %x{
    self.rbCheckFrozen()
    out := self.s[:0]
    for _, x := range self.s {
      if !bool(blk(x)) {
        out = append(out, x)
      }
    }
    clear(self.s[len(out):])
    self.s = out
    return self
  }

  # nil when nothing was removed, as MRI's.
  #: () { (E) -> bool } -> Array[E]?
  def select!(&block)
    n = size
    keep_if(&block)
    size == n ? nil : self
  end

  alias filter! select!

  #: () { (E) -> bool } -> Array[E]?
  def reject!(&block)
    n = size
    delete_if(&block)
    size == n ? nil : self
  end

  #: () -> Array[E]?
  def uniq! = %x{
    self.rbCheckFrozen()
    u := Array_Uniq(self)
    if len(u.s) == len(self.s) {
      return nil
    }
    self.s = u.s
    return &self
  }

  #: () -> self
  def reverse! = %x{
    self.rbCheckFrozen()
    slices.Reverse(self.s)
    return self
  }

  #: (Integer, *E) -> self
  def insert(i, *objs) = %x{
    self.rbCheckFrozen()
    if len(rest_) == 0 {
      return self
    }
    n := int(i)
    if n < 0 {
      n += len(self.s) + 1
    }
    if n < 0 {
      panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", int(i), len(self.s)+1)))))
    }
    if n > len(self.s) {
      rbCheckPad(n)
    }
    for len(self.s) < n {
      var zero E
      self.s = append(self.s, zero)
    }
    self.s = slices.Insert(self.s, n, rest_...)
    return self
  }

  #: (E) -> self
  def fill(v) = %x{
    self.rbCheckFrozen()
    for i := range self.s {
      self.s[i] = v
    }
    return self
  }

  # `a[start, len] = v`: v's elements replace the span when v is an Array, else v alone; past the end pads with nil (decision 7's zero)
  #: (Integer, Integer, untyped) -> untyped
  def __idxset_3(start, count, v) = %x{
    self.rbCheckFrozen()
    n := Integer(len(self.s))
    if start < 0 {
      if -start > n {
        panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", start, n)))))
      }
      start += n
    }
    if count < 0 {
      panic(NewIndexError(Ref(String(fmt.Sprintf("negative length (%d)", count)))))
    }
    var items []E
    if a, ok := v.(Array_Any); ok {
      for _, x := range a._ToAny().s {
        items = append(items, rbAs[E](x, "the array's element type"))
      }
    } else {
      items = []E{rbAs[E](v, "the array's element type")}
    }
    if Integer(len(self.s)) < start {
      rbCheckPad(int(start))
    }
    for Integer(len(self.s)) < start {
      var zero E
      self.s = append(self.s, zero)
    }
    end := start + min(count, Integer(len(self.s))-start)
    self.s = slices.Concat(self.s[:start:start], items, self.s[end:])
    return v
  }

  #: (Range[Integer], untyped) -> untyped
  def __idxset_range(r, v)
    span = r.__span(size)
    raise RangeError, "#{r.inspect} out of range" unless span

    __idxset_3(span[0], span[1], v)
  end

  #: (E, Integer | Range[Integer] | nil) -> self
  def __fill_2(v, at) = __fill_by(at, nil) { |_i| v }

  #: (E, Integer?, Integer?) -> self
  def __fill_3(v, start, len) = __fill_by(start, len) { |_i| v }

  #: () { (Integer) -> E } -> self
  def __fill_0(&blk) = __fill_by(nil, nil, &blk)

  #: (Integer | Range[Integer] | nil) { (Integer) -> E } -> self
  def __fill_1_block(at, &blk) = __fill_by(at, nil, &blk)

  #: (Integer?, Integer?) { (Integer) -> E } -> self
  def __fill_2_block(start, len, &blk) = __fill_by(start, len, &blk)

  # fill's span, as MRI clamps it: a start counted from the end, nil for 0 (or to the end), a range's ends; past the end grows the array
  #: (Integer | Range[Integer] | nil, Integer?) { (Integer) -> E } -> self
  def __fill_by(at, len)
    n = size
    if at.is_a?(Range)
      span = at.__span(n)
      raise RangeError, "#{at.inspect} out of range" unless span

      start, count = span
    else
      start = at || 0
      start += n if start < 0
      start = 0 if start < 0
      count = len || n - start
    end
    i = start
    while i < start + count
      self[i] = yield(i)
      i += 1
    end
    self
  end

  #: () { (Integer) -> void } -> void
  def each_index = %x{
    return func(yield func(Integer) bool) {
      for i := 0; ; i++ { // not range len: the block may grow the array
        if i >= len(self.s) || !yield(Integer(i)) {
          return
        }
      }
    }
  }

  #: () -> Enumerator[Integer]
  def __each_index_enum = %x{ return rbEnumOf(Array_EachIndex(self), any(self), "each_index", func() *Integer { n := Integer(len(self.s)); return &n }, nil) }

  #: () -> Enumerator[E]
  def __each_enum = %x{ return rbEnumOf(Array_Each(self), any(self), "each", func() *Integer { n := Integer(len(self.s)); return &n }, nil) }

  #: () -> Enumerator::Map[E]
  def __map_enum = Enumerator::Map.new(self)

  #: () -> Enumerator::Select[E]
  def __select_enum = Enumerator::Select.new(self, false, "select")

  #: () -> Enumerator::Select[E]
  def __filter_enum = Enumerator::Select.new(self, false, "filter")

  #: () -> Enumerator::Select[E]
  def __reject_enum = Enumerator::Select.new(self, true, "reject")

  # MRI's Array#first(-1) message; Enumerable#first raises it.
  #: () -> String
  def __negative_first = "negative array size"

  #: () -> Integer
  def size = %x{ Integer(len(self.s)) }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = %x{ len(self.s) == 0 }

  #: () -> E?
  def last = self[-1]

  # MRI's split of a proc's arguments (decision 367): how many optionals take a value once the leading and trailing
  # names have theirs, the rest, and trailing name i.
  #: (Integer, Integer, Integer) -> Integer
  def __bopts(pre, opt, post)
    n = size - pre - post
    return 0 if n < 0

    n < opt ? n : opt
  end

  #: (Integer, Integer, Integer) -> Array[E]
  def __brest(pre, opt, post)
    from = pre + __bopts(pre, opt, post)
    n = size - post - from
    n > 0 ? self[from, n] || [] : []
  end

  #: (Integer, Integer, Integer, Integer) -> E?
  def __bpost(i, pre, opt, post)
    from = pre + __bopts(pre, opt, post)
    self[(size - post > from ? size - post : from) + i]
  end

  #: (Integer) -> Array[E]
  def __last_1(n)
    raise ArgumentError, "negative array size" if n < 0

    self[[size - n, 0].max.to_i, n] || []
  end

  #: () -> Array[E]
  def reverse = %x{
    out := &Array[E]{}
    for i := len(self.s) - 1; i >= 0; i-- {
      out.s = append(out.s, self.s[i])
    }
    return out
  }

  #: () -> Array[E]
  def to_a = self

  # An array pattern's view of the Array: itself, as MRI's (decision 143).
  #: () -> Array[E]
  def deconstruct = self

  #: () -> Array[E]
  def dup = %x{
    out := &Array[E]{}
    out.s = append(out.s, self.s...)
    return out
  }

  # uniq with a block: the first element for each block value, by eql?/hash.
  #: [K] () { (E) -> K } -> Array[E]
  def __uniq_block
    seen = {} #: Hash[K, bool]
    out = [] #: Array[E]
    each do |x|
      k = yield(x)
      next if seen.key?(k)

      seen[k] = true
      out << x
    end
    out
  end

  # By eql?/hash, as Hash keys.
  #: () -> Array[E]
  def uniq = %x{
    seen := map[E]bool{}
    idx := rbKeyIndex[E]{plain: rbPlainKey[E]()}
    out := &Array[E]{}
    for _, x := range self.s {
      k, h, byValue := idx.find(x)
      if !seen[k] {
        seen[k] = true
        if byValue {
          idx.add(k, h)
        }
        out.s = append(out.s, x)
      }
    }
    return out
  }

  #: () -> Array[E]
  def compact = %x{
    out := &Array[E]{}
    for _, x := range self.s {
      if rbUnbox(any(x)) != nil {
        out.s = append(out.s, x)
      }
    }
    return out
  }

  # MRI's compact!: the nils removed in place; nil when there were none (prime's EratosthenesSieve).
  #: () -> Array[E]?
  def compact! = %x{
    self.rbCheckFrozen()
    out := self.s[:0]
    for _, x := range self.s {
      if rbUnbox(any(x)) != nil {
        out = append(out, x)
      }
    }
    if len(out) == len(self.s) {
      return nil
    }
    clear(self.s[len(out):]) // the tail no longer holds elements: let them be collected
    self.s = out
    return &self
  }

  #: () -> self
  def clear = %x{
    self.rbCheckFrozen()
    self.s = self.s[:0]
    return self
  }

  #: (E) -> E?
  def delete(v) = %x{
    if !slices.ContainsFunc(self.s, func(x E) bool { return bool(rbEq(x, v)) }) {
      return nil // MRI: no FrozenError when nothing is removed
    }
    self.rbCheckFrozen()
    var found *E
    out := self.s[:0]
    for _, x := range self.s {
      if rbEq(x, v) {
        x := x
        found = &x
        continue
      }
      out = append(out, x)
    }
    self.s = out
    return found
  }

  #: (Integer) -> E?
  def delete_at(i) = %x{
    self.rbCheckFrozen()
    if i < 0 {
      i += Integer(len(self.s))
    }
    if i < 0 || int(i) >= len(self.s) {
      return nil
    }
    x := self.s[i]
    self.s = append(self.s[:i], self.s[i+1:]...)
    return &x
  }

  # `xs * n` repeats; `xs * ","` is join (decision 12's twin by argument class).
  #: (Integer) -> Array[E]
  def *(n) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref(String("negative argument"))))
    }
    if len(self.s) > 0 && rbAllocTooBig(int(n), len(self.s)*8) {
      panic(NewArgumentError(Ref(String("argument too big"))))
    }
    out := &Array[E]{s: make([]E, 0, len(self.s)*int(n))}
    for i := 0; i < int(n) && len(self.s) > 0; i++ {
      out.s = append(out.s, self.s...)
    }
    return out
  }

  #: (String) -> String
  def __mul_string(sep) = join(sep)

  # The default is $, (nil unless the program set it, decision 414).
  #: (?String) -> String
  def join(sep = $, || "") = %x{
    parts := make([]string, len(self.s))
    for i, x := range self.s {
      // ponytail: a self-containing array overflows here, MRI raises ArgumentError; add a visited set.
      if s, ok := any(x).(String); ok { // MRI takes a String as is, without to_s; rbToS(x) would box it
        parts[i] = string(s)
        continue
      }
      if a, ok := any(x).(Array_Any); ok {
        parts[i] = string(Array_Join(a._ToAny(), sep))
        continue
      }
      parts[i] = string(rbToS(x))
    }
    if len(parts) == 1 { // Join would return the element itself
      return rbStrClone(String(parts[0]))
    }
    return String(strings.Join(parts, string(sep)))
  }

  #: () -> String
  def inspect = _inspect_rec(nil)

  # inspect inside another container's: seen (an rbSeen) is passed down,
  # never shared between threads.
  #: (untyped) -> String
  def _inspect_rec(seen) = %x{
    s := rbSeenOf(seen)
    if !s.enter(self, nil) {
      return "[...]"
    }
    defer s.leave(self, nil)
    parts := make([]string, len(self.s))
    for i, x := range self.s {
      parts[i] = string(rbInspectIn(x, s))
    }
    return String("[" + strings.Join(parts, ", ") + "]")
  }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = _eq_rec(other, nil)

  # == with the pairs already being compared (seen, an rbSeen), made only
  # once a nested container is reached: a flat Array allocates nothing.
  #: (untyped, untyped) -> bool
  def _eq_rec(other, seen) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      // Another instantiation ([1] == [1.0], typed vs untyped): compare
      // the untyped views, whose == takes the branch below.
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    if len(o.s) != len(self.s) {
      return false
    }
    if a, ok := any(self.s).([]Integer); ok && rbIntEqGo { // Integer#== is Go's: no boxing per element (decision 387)
      return Boolean(slices.Equal(a, any(o.s).([]Integer)))
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true // a pair already being compared: equal, as MRI's recursive ==
      }
      defer s.leave(self, o)
    }
    for i, x := range self.s {
      if r, ok := any(x).(rbEqRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqRec(o.s[i], s) {
          return false
        }
      } else if !rbEq(x, o.s[i]) {
        return false
      }
    }
    return true
  }

  #: (untyped) -> bool
  def eql?(other) = _eql_rec(other, nil)

  #: (untyped, untyped) -> bool
  def _eql_rec(other, seen) = %x{
    o, ok := other.(*Array[E])
    if !ok {
      if a, ok := other.(Array_Any); ok {
        return self._ToAny().EqlQ(a._ToAny())
      }
      return false
    }
    if len(o.s) != len(self.s) {
      return false
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true
      }
      defer s.leave(self, o)
    }
    for i, x := range self.s {
      if r, ok := any(x).(rbEqlRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqlRec(o.s[i], s) {
          return false
        }
      } else if !rbKeyEql(x, o.s[i]) {
        return false
      }
    }
    return true
  }

  #: () -> Integer
  def hash = _hash_rec(nil)

  #: (untyped) -> Integer
  def _hash_rec(seen) = %x{
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, nil) {
        return 0 // inside itself: a constant, so the hash is finite and stable
      }
      defer s.leave(self, nil)
    }
    h := uint64(len(self.s))
    for _, x := range self.s {
      if r, ok := any(x).(rbHashRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, nil)
        }
        h = h*31 + uint64(r._HashRec(s)) //nolint:gosec // a hash, not arithmetic
        continue
      }
      h = h*31 + rbKeyHash(x)
    }
    return Integer(h)
  }

  #: (Array[E]) -> Integer
  def <=>(other) = %x{
    n := min(len(self.s), len(other.s))
    for i := range n {
      if c := rbCmp(self.s[i], other.s[i]); c != 0 {
        return c
      }
    }
    return Integer(cmp.Compare(len(self.s), len(other.s)))
  }

  # The untyped view (rbView, prelude/go/view.go): a few one-line accessors per element type, so a dynamic call of a
  # whole-array method runs one body compiled once instead of one per element type (decision 236).
  #: () -> Integer
  def _len = %x{ return Integer(len(self.s)) }

  #: (Integer) -> untyped
  def _at(i) = %x{ return self.s[i] }

  #: (Integer, untyped) -> void
  def _set(i, v) = %x{ self.s[i] = rbAs[E](v, "element") }

  #: () -> untyped
  def _make = %x{ return &Array[E]{} }

  #: (untyped) -> void
  def _append(v) = %x{ self.s = append(self.s, rbAs[E](v, "element")) }

  # n elements: the first n, or padded with E's zero value as []= pads (decision 7).
  #: (Integer) -> void
  def _resize(n) = %x{
    if int(n) <= len(self.s) {
      self.s = self.s[:n]
      return
    }
    self.s = append(self.s, make([]E, int(n)-len(self.s))...)
  }

  # Drops the first n elements without copying, as shift does (decision 304).
  #: (Integer) -> void
  def _drop(n) = %x{ self.s = self.s[n:] }

  #: () -> Array[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Array[any]); ok {
      return same // already untyped: share it, so writes are seen
    }
    out := &Array[any]{}
    for _, x := range self.s {
      out.s = append(out.s, rbUnbox(x))
    }
    return out
  }

  # flatten on elements that are not Arrays: a copy, or, for untyped
  # elements, MRI's dynamic splice. Typed nesting goes to the @self forms
  # below (decision 92).
  #: (?Integer) -> Array[E]
  def flatten(depth = -1) = %x{
    if a, ok := any(self).(*Array[any]); ok {
      out := &Array[any]{}
      rbFlattenInto(out, a.s, int(depth))
      return any(out).(*Array[E])
    }
    return &Array[E]{s: slices.Clone(self.s)}
  }

  # flatten in place: self when anything was spliced, else nil (typed elements are never Arrays here).
  #: (?Integer) -> Array[E]?
  def flatten!(depth = -1) = %x{
    self.rbCheckFrozen()
    a, ok := any(self).(*Array[any])
    if !ok || !slices.ContainsFunc(a.s, func(x any) bool { _, isArr := x.(Array_Any); return isArr }) {
      return nil
    }
    out := &Array[any]{}
    rbFlattenInto(out, a.s, int(depth))
    a.s = out.s
    p := self
    return &p
  }

  # compact on optional elements drops the nils from the type too.
  # @self Array[U?]
  # @rbs [U] () -> Array[U]
  def __compact_opt = %x{
    out := &Array[U]{}
    for _, x := range self.s {
      if x != nil {
        out.s = append(out.s, *x)
      }
    }
    return out
  }

  # Pairs are Arrays to Ruby: [[:a, 1]].flatten splices them.
  # @self Array[[A, B]]
  # @rbs [A, B] (?Integer) -> Array[untyped]
  def __flatten_pairs(depth = -1) = %x{
    xs := make([]any, len(self.s))
    for i, t := range self.s {
      xs[i] = t
    }
    out := &Array[any]{}
    rbFlattenInto(out, xs, int(depth))
    return out
  }

  # @self Array[Array[Array[U]]]
  # @rbs [U] () -> Array[U]
  def __flatten_nested3 = %x{
    rows := make([]*Array[U], 0, len(self.s))
    for _, r := range self.s {
      rows = append(rows, r.s...)
    }
    return rbFlattenRows(rows, -1)
  }

  # @self Array[Array[U]]
  # @rbs [U] (?Integer) -> Array[U]
  def __flatten_nested(depth = -1) = %x{
    if depth == 0 { // the result would be Array[Array[U]], not this signature's Array[U]
      panic(NewNotImplementedError(Ref(String("rugaroo: flatten(0) of nested Arrays; use dup"))))
    }
    return rbFlattenRows(self.s, int(depth))
  }

  # @self Array[[K, V]]
  # @rbs [K, V] () -> Hash[K, V]
  def to_h
    out = {} #: Hash[K, V]
    each { |pair| out[pair[0]] = pair[1] }
    out
  end

  # Pairs as two-element Arrays: the length is checked as MRI does.
  # @self Array[Array[U]]
  # @rbs [U] () -> Hash[U, U]
  def __to_h_arrays
    out = {} #: Hash[U, U]
    each_with_index do |r, i|
      raise ArgumentError, "wrong array length at #{i} (expected 2, was #{r.size})" unless r.size == 2

      out[r.fetch(0)] = r.fetch(1)
    end
    out
  end

  # @rbs [K, V] () { (E) -> [K, V] } -> Hash[K, V]
  def __to_h_block
    out = {} #: Hash[K, V]
    each do |x|
      pair = yield(x)
      out[pair[0]] = pair[1]
    end
    out
  end

  # Rows of different lengths raise IndexError, as in MRI.
  # @self Array[Array[U]]
  # @rbs [U] () -> Array[Array[U]]
  def transpose = %x{
    out := &Array[*Array[U]]{}
    if len(self.s) == 0 {
      return out
    }
    n := len(self.s[0].s)
    for _, r := range self.s {
      if len(r.s) != n {
        panic(NewIndexError(Ref(String(fmt.Sprintf("element size differs (%d should be %d)", len(r.s), n)))))
      }
    }
    for j := range n {
      col := &Array[U]{s: make([]U, len(self.s))}
      for i, r := range self.s {
        col.s[i] = r.s[j]
      }
      out.s = append(out.s, col)
    }
    return out
  }

  # @self Array[X]
  # @rbs [X] () -> Array[Array[untyped]]
  def __transpose_any = %x{
    xs := make([]any, len(self.s))
    for i, x := range self.s {
      xs[i] = x
    }
    return rbTransposeAny(xs)
  }

  # The flag is on the object; every mutator above checks it first (decision 96).
  #: () -> self
  def freeze = %x{
    self.frozen.Store(true)
    return self
  }

  #: () -> bool
  def frozen? = %x{ Boolean(self.frozen.Load()) }

  #: (Integer) -> E?
  def at(i) = self[i]

  #: (Integer, *untyped) -> untyped
  def dig(i, *rest) = %x{ return rbDig(self, append([]any{i}, rest_...)) }

  # The first element that is an Array (or tuple) whose first item == key.
  #: (untyped) -> E?
  def assoc(key) = %x{ return rbAssoc(self, key, 0) }

  # The first element that is an Array (or tuple) whose second item == value.
  #: (untyped) -> E?
  def rassoc(value) = %x{ return rbAssoc(self, value, 1) }

  #: (*Integer) -> Array[E]
  def fetch_values(*idx) = idx.map { |i| fetch(i) }

  #: () { (E) -> bool } -> Integer?
  def bsearch_index = %x{
    i := sort.Search(len(self.s), func(i int) bool { return bool(blk(self.s[i])) })
    if i == len(self.s) {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(i))
  }

  #: () { (E) -> bool } -> E?
  def rfind = %x{
    for i := len(self.s) - 1; i >= 0; i-- {
      if bool(blk(self.s[i])) {
        x := self.s[i] // a copy (decision 397)
        return &x
      }
    }
    return nil
  }

  #: (Array[E]) -> self
  def replace(other) = %x{
    self.rbCheckFrozen()
    self.s = append([]E{}, other.s...)
    return self
  }
end
