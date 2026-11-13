# prelude/hash.rb
# rbs_inline: enabled

#
# Hash: insertion-ordered, an entry array plus an open-addressed index of it (decisions 350, 388).

# iter is MRI's iter_lev: while non-zero, []= of a new key raises and nothing compacts (iterators skip tombstones; head is the first maybe-live entry, for shift).

# @rbs generic K
# @rbs generic V
# @go_type struct { ents []rbHashEnt[K, V]; slots []int32; gone []uint64; live int32; head int32; iter int; idx rbKeyIndex[K]; frozen atomic.Bool; defVal V; hasDef bool; defProc func(*Hash[K, V], K) V }
class Hash < Object
  include Enumerable #[[K, V]]

  # Hash.new(default) / Hash.new { |hash, key| ... } set a default for a
  # missing key (decision 5). A bare Hash.new is the zero value (genNew).
  # The default is returned by #[] and values_at/dig; fetch still reports the
  # miss.
  #: [K, V] (?V?) -> Hash[K, V]
  def self.new(d = nil) = %x{
    h := NewHash[K, V]()
    h.hasDef = true
    if d != nil {
      h.defVal = *d
    }
    return h
  }

  #: [K, V] () { (Hash[K, V], K) -> V } -> Hash[K, V]
  def self.__new_block = %x{
    h := NewHash[K, V]()
    h.defProc = func(owner *Hash[K, V], k K) V {
      return blk(owner, k)
    }
    return h
  }

  # Hash[hash], Hash[[[k, v], ...]] and Hash[k, v, ...], as one untyped form (RBS's three overloads).
  #: (*untyped) -> Hash[untyped, untyped]
  def self.[](*xs)
    h = {} #: Hash[untyped, untyped]
    if xs.size == 1
      x = xs[0]
      if x.is_a?(Hash)
        x.each { |k, v| h[k] = v }
        return h
      end
      if x.is_a?(Array)
        x.each_with_index do |pair, i|
          raise ArgumentError, "wrong element type #{pair.class} at #{i} (expected array)" unless pair.is_a?(Array)
          raise ArgumentError, "invalid number of elements (#{pair.size} for 1..2)" unless pair.size == 1 || pair.size == 2

          h[pair[0]] = pair[1]
        end
        return h
      end
    end
    raise ArgumentError, "odd number of arguments for Hash" if xs.size.odd?

    i = 0
    while i < xs.size
      h[xs[i]] = xs[i + 1]
      i += 2
    end
    h
  end

  #: () { ([K, V]) -> void } -> void
  def each = %x{
    return func(yield func(Tuple2[K, V]) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for i := int(self.head); i < len(self.ents); i++ {
        if !rbGone(self.gone, i) && !yield(Tuple2[K, V]{self.ents[i].k, self.ents[i].v}) {
          return
        }
      }
    }
  }

  # MRI's each_pair is each: an arity-1 block gets the [k, v] pair.
  alias each_pair each

  #: () { (K) -> void } -> void
  def each_key = %x{
    return func(yield func(K) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for i := int(self.head); i < len(self.ents); i++ {
        if !rbGone(self.gone, i) && !yield(self.ents[i].k) {
          return
        }
      }
    }
  }

  #: () { (V) -> void } -> void
  def each_value = %x{
    return func(yield func(V) bool) {
      self.iter++
      defer func() { self.iter-- }()
      for i := int(self.head); i < len(self.ents); i++ {
        if !rbGone(self.gone, i) && !yield(self.ents[i].v) {
          return
        }
      }
    }
  }

  # RBS core says `(K) -> V`; with a default that holds. Without one a miss
  # is nil, so V? (decision 5). A default proc is called (self, key), as MRI's.
  #: (K) -> V?
  # One out-of-line call, so this inlines and the V? it returns stays on the caller's stack.
  def [](k) = %x{
    if v, ok := rbHashLookup(self, k); ok {
      return &v
    }
    return nil
  }

  #: () -> V?
  def default = %x{
    if self.hasDef {
      v := self.defVal // a copy: a later `default=` must not show through (decision 397)
      return &v
    }
    return nil
  }

  #: (V?) -> V?
  def default=(d)
    %x{
    self.rbCheckFrozen()
    self.defProc = nil
    self.hasDef = true
    if d != nil {
      self.defVal = *d
    } else {
      var zero V
      self.defVal = zero
    }
    return d}
  end

  #: () -> untyped
  def default_proc = %x{
    if self.defProc == nil {
      return nil
    }
    return self.defProc
  }

  # A Proc held untyped (a gem's lambda) or another Hash's default_proc; clears the default value, as MRI does.
  #: (untyped) -> untyped
  def default_proc=(pr)
    %x{
    self.rbCheckFrozen()
    var zero V
    self.hasDef, self.defVal = false, zero
    if pr == nil {
      self.defProc = nil
      return nil
    }
    if fn, ok := pr.(func(*Hash[K, V], K) V); ok {
      self.defProc = fn
      return pr
    }
    if !rbIsProc(pr) {
      panic(NewTypeError(Ref(String("wrong default_proc type " + rbClassName(pr) + " (expected Proc)"))))
    }
    self.defProc = func(owner *Hash[K, V], k K) V {
      r, _ := rbDynProcCall(pr, []any{owner, k})
      return rbAs[V](r, "value")
    }
    return pr}
  end

  #: (K, V) -> V
  def []=(k, v)
    %x{
    self.rbCheckFrozen()
    var h uint64
    byValue := false
    if !self.idx.plain {
      k, h, byValue = self.idx.find(k)
    }
    var hv uint64
    if self.slots != nil {
      var i int
      if i, hv = rbHashFind(self, k); i >= 0 {
        self.ents[i].v = v
        return v
      }
    } else {
      for i := range self.ents {
        if (self.ents[i].k == k || self.idx.str && rbStrPEq(self.ents[i].k, k)) && (int(self.live) == len(self.ents) || !rbGone(self.gone, i)) {
          self.ents[i].v = v
          return v
        }
      }
    }
    if self.iter > 0 {
      panic(NewRuntimeError(Ref[String]("can't add a new key into hash during iteration")))
    }
    // a new key; tombstones are left to delete, so []= never compacts and the ninth entry builds slots
    self.ents = append(self.ents, rbHashEnt[K, V]{k, v})
    if self.slots != nil || len(self.ents) > rbSmallHash {
      rbHashIndex(self, hv)
    }
    self.live++
    if byValue {
      self.idx.add(k, h)
    }
    return v}
  end

  #: (K, V) -> self
  def __set(k, v)
    self[k] = v
    self
  end

  #: (K, ?V?) -> V
  def fetch(k, default = nil)
    v = __fetch_opt(k, nil)
    return v if v
    return default if default

    raise KeyError.__for("key not found: #{k.inspect}", self, k)
  end

  # fetch with a block: the block's value for a missing key.
  #: (K) { (K) -> V } -> V
  def __fetch_block(k) = key?(k) ? fetch(k) : yield(k)

  #: (K, V?) -> V?
  def __fetch_opt(k, fallback) = %x{
    if v, ok := self.rbGet(k); ok {
      return &v
    }
    return fallback
  }

  # Starts from dup so default/default_proc survive, as MRI's.
  #: (Hash[K, V]) -> Hash[K, V]
  def merge(other)
    out = dup
    other.each { |k, v| out[k] = v }
    out
  end

  # A new, unfrozen Hash with the same entries and default.
  #: () -> Hash[K, V]
  def dup = %x{
    out := NewHash[K, V]()
    out.defVal, out.hasDef, out.defProc = self.defVal, self.hasDef, self.defProc
    if int(self.live) == len(self.ents) && len(self.ents) > 0 { // a straight copy: no rehash
      out.ents = append([]rbHashEnt[K, V](nil), self.ents...)
      out.live = self.live
      out.slots = slices.Clone(self.slots)
      for h, b := range self.idx.byHash {
        if out.idx.byHash == nil {
          out.idx.byHash = make(map[uint64][]K, len(self.idx.byHash))
        }
        out.idx.byHash[h] = append([]K(nil), b...)
      }
      return out
    }
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      Hash_Op_idxSet[K, V](out, k, v)
    }
    return out
  }

  #: () -> self
  def clear = %x{
    self.rbCheckFrozen()
    rbHashClear(self)
    return self
  }

  # Keys match by Go ==: object identity for a class's objects. A String, Float or tuple key is a Go value with no
  # identity of its own and still matches by content (decision 444). ponytail: a copy (dup, select) starts by-value again.
  #: () -> self
  def compare_by_identity = %x{
    self.rbCheckFrozen()
    self.idx.plain, self.idx.ident, self.idx.str = true, true, false
    clear(self.idx.byHash)
    return self
  }

  #: () -> bool
  def compare_by_identity? = %x{ Boolean(self.idx.ident) }

  #: (K) -> bool
  def key?(k) = %x{
    _, ok := self.rbGet(k)
    return Boolean(ok)
  }

  #: (K) -> bool
  def include?(k) = key?(k)

  #: (K) -> bool
  def has_key?(k) = key?(k)

  #: (K) -> V?
  def delete(k) = %x{
    self.rbCheckFrozen()
    if !self.idx.plain {
      k, _, _ = self.idx.find(k)
    }
    i := -1
    if self.slots != nil {
      i, _ = rbHashFind(self, k)
    } else {
      for j := range self.ents {
        if (self.ents[j].k == k || self.idx.str && rbStrPEq(self.ents[j].k, k)) && (int(self.live) == len(self.ents) || !rbGone(self.gone, j)) {
          i = j
          break
        }
      }
    }
    if i < 0 {
      return nil
    }
    v := self.ents[i].v
    rbHashDropAt(self, i)
    return &v
  }

  # Unlike Enumerable#select, Ruby's Hash#select returns a Hash. MRI
  # yields (k, v), not the pair, so an arity-1 block gets the key.
  #: () { (K, V) -> bool } -> Hash[K, V]
  def select
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  alias filter select

  #: () { (K, V) -> bool } -> Hash[K, V]
  def reject
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless yield(k, v) }
    out
  end

  #: [U] () { (V) -> U } -> Hash[K, U]
  def transform_values
    out = {} #: Hash[K, U]
    each { |k, v| out[k] = yield(v) }
    out
  end

  # In place: the block keeps the value type, as a Hash's V is fixed.
  #: () { (V) -> V } -> self
  def transform_values!
    __check_frozen
    keys.each { |k| self[k] = yield(fetch(k)) }
    self
  end

  #: [U] () { (K) -> U } -> Hash[U, V]
  def transform_keys
    out = {} #: Hash[U, V]
    each { |k, v| out[yield(k)] = v }
    out
  end

  #: [A, B] () { (K, V) -> [A, B] } -> Hash[A, B]
  def __to_h_block
    out = {} #: Hash[A, B]
    each do |k, v|
      pair = yield(k, v)
      out[pair[0]] = pair[1]
    end
    out
  end

  # Self as MRI's; a subclass answers its embedded Hash (decision 165).
  #: () -> Hash[K, V]
  def to_h = self

  # A hash pattern's view of the Hash: itself, whatever keys it asks for, as MRI's (decision 143).
  #: (Array[Symbol]?) -> Hash[K, V]
  def deconstruct_keys(_keys) = self

  #: () -> Hash[V, K]
  def invert
    out = {} #: Hash[V, K]
    each { |k, v| out[v] = k }
    out
  end

  #: (V) -> K?
  def key(value)
    each { |k, v| return k if v == value }
    nil
  end

  #: (V) -> bool
  def value?(value)
    each { |_k, v| return true if v == value }
    false
  end

  #: (V) -> bool
  def has_value?(value) = value?(value)

  #: (K) -> bool
  def member?(k) = key?(k)

  #: (*K) -> Array[V?]
  def values_at(*ks) = ks.map { |k| self[k] }

  #: (*K) -> Array[V]
  def fetch_values(*ks) = ks.map { |k| fetch(k) }

  #: (*K) -> Hash[K, V]
  def slice(*ks) = %x{
    out := NewHash[K, V]()
    for _, k := range rest_ {
      if v, ok := self.rbGet(k); ok {
        Hash_Op_idxSet(out, k, v)
      }
    }
    return out
  }

  #: (*K) -> Hash[K, V]
  def except(*ks)
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless ks.include?(k) }
    out
  end

  #: (K, V) -> V
  def store(k, v) = self[k] = v

  #: (Hash[K, V]) -> self
  def update(other)
    __check_frozen
    other.each { |k, v| self[k] = v }
    self
  end

  #: (Hash[K, V]) -> self
  def merge!(other) = update(other)

  # merge!/update with a block, which resolves a key both hashes hold.
  #: (Hash[K, V]) { (K, V, V) -> V } -> self
  def __update_block(other)
    __check_frozen
    other.each do |k, v|
      self[k] = key?(k) ? yield(k, fetch(k), v) : v
    end
    self
  end

  #: (Hash[K, V]) { (K, V, V) -> V } -> self
  def __merge_bang_block(other, &blk) = __update_block(other, &blk)

  # The block resolves a key both hashes hold: (key, old, new) -> value.
  #: (Hash[K, V]) { (K, V, V) -> V } -> Hash[K, V]
  def __merge_block(other) = %x{
    out := Hash_Dup[K, V](self)
    other.iter++
    defer func() { other.iter-- }()
    for i := int(other.head); i < len(other.ents); i++ {
      if rbGone(other.gone, i) {
        continue
      }
      k, v := other.ents[i].k, other.ents[i].v
      if old, ok := out.rbGet(k); ok {
        v = blk(k, old, v)
      }
      Hash_Op_idxSet(out, k, v)
    }
    return out
  }

  #: () { (K, V) -> bool } -> self
  def delete_if = %x{
    self.rbCheckFrozen()
    rbHashDropWhere(self, func(k K, v V) bool { return bool(blk(k, v)) })
    return self
  }

  #: () { (K, V) -> bool } -> self
  def keep_if = %x{
    self.rbCheckFrozen()
    rbHashDropWhere(self, func(k K, v V) bool { return !bool(blk(k, v)) })
    return self
  }

  # nil when nothing was removed, as MRI's.
  #: () { (K, V) -> bool } -> Hash[K, V]?
  def reject!(&block)
    n = size
    delete_if(&block)
    size == n ? nil : self
  end

  #: () { (K, V) -> bool } -> Hash[K, V]?
  def select!(&block)
    n = size
    keep_if(&block)
    size == n ? nil : self
  end

  alias filter! select!

  #: () { (K, V) -> bool } -> Integer
  def __count_block
    n = 0
    each { |k, v| n += 1 if yield(k, v) }
    n
  end

  #: () { (K, V) -> bool } -> bool
  def any?
    each { |k, v| return true if yield(k, v) }
    false
  end

  #: () { (K, V) -> bool } -> bool
  def all?
    each { |k, v| return false unless yield(k, v) }
    true
  end

  #: () { (K, V) -> bool } -> bool
  def none?
    each { |k, v| return false if yield(k, v) }
    true
  end

  #: () -> [K, V]?
  def __first_pair
    each { |k, v| return [k, v] }
    nil
  end

  #: () -> Integer
  def size = %x{ Integer(int(self.live)) }

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = %x{ int(self.live) == 0 }

  #: () -> Array[K]
  def keys = %x{
    out := &Array[K]{s: make([]K, 0, self.live)}
    for i := int(self.head); i < len(self.ents); i++ {
      if !rbGone(self.gone, i) {
        out.s = append(out.s, self.ents[i].k)
      }
    }
    return out
  }

  #: () -> Array[V]
  def values = %x{
    out := &Array[V]{s: make([]V, 0, int(self.live))}
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      v := self.ents[i].v
      out.s = append(out.s, v)
    }
    return out
  }

  #: () -> String
  def inspect = _inspect_rec(nil)

  # inspect inside another container's: seen (an rbSeen) is passed down,
  # never shared between threads.
  #: (untyped) -> String
  def _inspect_rec(seen) = %x{
    s := rbSeenOf(seen)
    if !s.enter(self, nil) {
      return "{...}"
    }
    defer s.leave(self, nil)
    parts := make([]string, 0, int(self.live))
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      parts = append(parts, rbInspectPair(k, v, s))
    }
    return String("{" + strings.Join(parts, ", ") + "}")
  }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = _eq_rec(other, nil)

  # == with the pairs already being compared (seen, an rbSeen), made only
  # once a nested container is reached.
  #: (untyped, untyped) -> bool
  def _eq_rec(other, seen) = %x{
    o, ok := other.(*Hash[K, V])
    if !ok {
      // Another instantiation ({1 => 1} == {1 => 1.0}, typed vs untyped):
      // compare the untyped views, whose == takes the branch below.
      if h, ok := other.(Hash_Any); ok {
        return self._ToAny().Op_eq(h._ToAny())
      }
      return false
    }
    if int(o.live) != int(self.live) {
      return false
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true // a pair already being compared: equal, as MRI's recursive ==
      }
      defer s.leave(self, o)
    }
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      ov, ok := o.rbGet(k)
      if !ok {
        return false
      }
      if r, rec := any(v).(rbEqRec); rec {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqRec(ov, s) {
          return false
        }
      } else if !bool(rbEq(v, ov)) {
        return false
      }
    }
    return true
  }

  # Like ==, but values compare by eql? too ({1 => 1} is not eql? to
  # {1 => 1.0}).
  #: (untyped) -> bool
  def eql?(other) = _eql_rec(other, nil)

  #: (untyped, untyped) -> bool
  def _eql_rec(other, seen) = %x{
    o, ok := other.(*Hash[K, V])
    if !ok {
      if h, ok := other.(Hash_Any); ok {
        return self._ToAny().EqlQ(h._ToAny())
      }
      return false
    }
    if int(o.live) != int(self.live) {
      return false
    }
    s, _ := seen.(*rbSeen)
    if s != nil {
      if !s.enter(self, o) {
        return true
      }
      defer s.leave(self, o)
    }
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      ov, ok := o.rbGet(k)
      if !ok {
        return false
      }
      if r, rec := any(v).(rbEqlRec); rec {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, o)
        }
        if !r._EqlRec(ov, s) {
          return false
        }
      } else if !rbKeyEql(v, ov) {
        return false
      }
    }
    return true
  }

  # Order-independent, as eql? is.
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
    h := uint64(int(self.live)) //nolint:gosec // a length
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      var vh uint64
      if r, ok := any(v).(rbHashRec); ok {
        if s == nil {
          s = rbSeenOf(nil)
          s.enter(self, nil)
        }
        vh = uint64(r._HashRec(s)) //nolint:gosec // a hash, not arithmetic
      } else {
        vh = rbKeyHash(v)
      }
      h += rbKeyHash(k)*31 ^ vh
    }
    return Integer(h)
  }

  # The untyped view (rbHashView, prelude/go/view.go): per-instantiation accessors behind one compiled body (decision 236).
  #: () -> Integer
  def _len = %x{ return Integer(int(self.live)) }

  # The keys, in order, each as the untyped value it is ([]any).
  #: () -> untyped
  def _keys_any = %x{
    out := make([]any, 0, int(self.live))
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k := self.ents[i].k
      out = append(out, k)
    }
    return out
  }

  # The pairs, in order, as each yields them (decision 302): []any of Tuple2[K, V].
  #: () -> untyped
  def _pairs_any = %x{
    out := make([]any, 0, int(self.live))
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      out = append(out, Tuple2[K, V]{k, v})
    }
    return out
  }

  #: (untyped) -> bool
  def _has(k) = %x{
    _, ok := self.rbGet(rbAs[K](k, "key"))
    return Boolean(ok)
  }

  #: (untyped) -> untyped
  def _get(k) = %x{
    v, _ := self.rbGet(rbAs[K](k, "key"))
    return v
  }

  #: (untyped, untyped) -> void
  def _store(k, v) = %x{ Hash_Op_idxSet(self, rbAs[K](k, "key"), rbAs[V](v, "value")) }

  #: (untyped) -> void
  def _delete(k) = %x{ Hash_Delete(self, rbAs[K](k, "key")) }

  #: () -> untyped
  def _make = %x{ return NewHash[K, V]() }

  # dup with the default and default_proc, for a dynamic merge.
  #: () -> untyped
  def _dup = dup

  #: () -> Hash[untyped, untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Hash[any, any]); ok {
      return same
    }
    out := NewHash[any, any]()
    for i := int(self.head); i < len(self.ents); i++ {
      if rbGone(self.gone, i) {
        continue
      }
      k, v := self.ents[i].k, self.ents[i].v
      Hash_Op_idxSet(out, rbUnbox(k), rbUnbox(v))
    }
    return out
  }

  # The flag is on the object; every mutator above checks it first (decision 96).
  #: () -> self
  def freeze = %x{
    self.frozen.Store(true)
    return self
  }

  #: () -> bool
  def frozen? = %x{ Boolean(self.frozen.Load()) }

  #: () -> [K, V]?
  def shift = %x{
    self.rbCheckFrozen()
    i := rbHashFirst(self)
    if i < 0 {
      return nil
    }
    pair := Tuple2[K, V]{self.ents[i].k, self.ents[i].v}
    rbHashDropAt(self, i)
    return &pair
  }

  #: (K) -> [K, V]?
  def assoc(k) = key?(k) ? [k, fetch(k)] : nil

  #: (Hash[K, V]) -> self
  def replace(other)
    pairs = other.to_a
    clear
    pairs.each { |k, v| self[k] = v }
    __copy_default(other)
    self
  end

  # MRI's replace and compact carry the default value or proc over.
  #: (Hash[K, V]) -> void
  def __copy_default(other) = %x{
    self.defProc, self.hasDef, self.defVal = other.defProc, other.hasDef, other.defVal
  }

  #: () -> void
  def __check_frozen = %x{ self.rbCheckFrozen() }

  #: () -> self
  def to_hash = self

  #: (K, *untyped) -> untyped
  def dig(k, *rest) = %x{ return rbDig(self, append([]any{k}, rest_...)) }

  #: (?Integer) -> Array[untyped]
  def flatten(depth = 1)
    out = [] #: Array[untyped]
    each do |k, v|
      out << k
      out << v
    end
    depth > 1 ? out.flatten(depth - 1) : out
  end

  # Without the nil values. V keeps its type: a Hash[K, V?] result is still V?, though it holds no nil.
  #: () -> Hash[K, V]
  def compact
    out = {} #: Hash[K, V]
    each { |k, v| out[k] = v unless v.nil? }
    out.__copy_default(self)
    out
  end

  #: () -> Hash[K, V]?
  def compact!
    n = size
    delete_if { |_k, v| v.nil? }
    size == n ? nil : self
  end
end
