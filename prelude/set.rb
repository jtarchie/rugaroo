# rbs_inline: enabled

# Core in Ruby 4; insertion-ordered like the Hash it wraps, and keyed by eql?/hash the same way.
# @rbs generic E
# @go_type struct { h *Hash[E, Boolean] }
class Set < Object
  include Enumerable #[E]

  #: [X] (untyped) -> Set[X]
  def self.new(xs = nil) = %x{
    s := NewSet[X]()
    if xs == nil {
      return s
    }
    elems, ok := rbEnumElems(any(xs))
    if !ok {
      panic(NewTypeError(Ref(String("no implicit conversion into Set"))))
    }
    for _, e := range elems.s {
      Set_Add(s, e.(X))
    }
    return s
  }

  #: [X] (Array[X]) -> Set[X]
  def self.__new_array(xs) = %x{
    s := NewSet[X]()
    for _, x := range xs.s {
      Set_Add(s, x)
    }
    return s
  }

  #: [X] (Range[X]) -> Set[X]
  def self.__new_range(xs) = %x{
    s := NewSet[X]()
    for x := range xs.Each() {
      Set_Add(s, x)
    }
    return s
  }

  #: [X] (Set[X]) -> Set[X]
  def self.__new_set(xs) = %x{
    s := NewSet[X]()
    for x := range xs.Each() {
      Set_Add(s, x)
    }
    return s
  }

  #: [K, V] (Hash[K, V]) -> Set[[K, V]]
  def self.__new_hash(xs) = %x{
    s := NewSet[Tuple2[K, V]]()
    for t := range xs.Each() {
      Set_Add(s, t)
    }
    return s
  }

  # ponytail: block param is untyped (dynamic dispatch, see decision 32) instead of the source's real element type; add per-class block overloads (__new_array_block etc.) if this shows up hot.
  #: [X] (untyped) { (untyped) -> X } -> Set[X]
  def self.__new_block(xs) = %x{
    s := NewSet[X]()
    elems, ok := rbEnumElems(any(xs))
    if !ok {
      panic(NewTypeError(Ref(String("no implicit conversion into Set"))))
    }
    for _, e := range elems.s {
      Set_Add(s, blk(e))
    }
    return s
  }

  #: [X] (*X) -> Set[X]
  def self.[](*xs) = Set.new(xs)

  #: () { (E) -> void } -> void
  def each = %x{
    return func(yield func(E) bool) {
      for k := range Hash_EachKey(self.h) {
        if !yield(k) {
          return
        }
      }
    }
  }

  #: (E) -> self
  def add(x) = %x{
    Hash_Op_idxSet(self.h, x, true)
    return self
  }

  #: (E) -> self
  def <<(x) = add(x)

  #: () -> self
  def compare_by_identity = %x{
    Hash_CompareByIdentity(self.h)
    return self
  }

  #: () -> bool
  def compare_by_identity? = %x{ Hash_CompareByIdentityQ(self.h) }

  #: (E) -> Set[E]?
  def add?(x)
    return nil if include?(x)

    add(x)
  end

  #: (E) -> self
  def delete(x) = %x{
    Hash_Delete(self.h, x)
    return self
  }

  #: (E) -> Set[E]?
  def delete?(x)
    return nil unless include?(x)

    delete(x)
  end

  #: (E) -> bool
  def include?(x) = %x{ Hash_KeyQ(self.h, x) }

  #: (E) -> bool
  def member?(x) = include?(x)

  #: (E) -> bool
  def ===(x) = include?(x)

  #: () -> Integer
  def size = %x{ Hash_Size(self.h) }

  #: () -> Integer
  def length = size

  #: () -> Integer
  def count = size

  #: () -> bool
  def empty? = %x{ Hash_EmptyQ(self.h) }

  #: () -> self
  def clear = %x{
    Hash_Clear(self.h)
    return self
  }

  #: () -> Array[E]
  def to_a = %x{ Hash_Keys(self.h) }

  #: () -> Set[E]
  def dup = Set.new(to_a)

  #: () -> Set[E]
  def to_set = self

  #: () { (E) -> E } -> self
  def map!
    old = to_a
    clear
    old.each { |x| add(yield(x)) }
    self
  end

  #: () { (E) -> E } -> self
  def collect!(&block) = map!(&block)

  #: () { (E) -> bool } -> Set[E]?
  def select!
    n = size
    to_a.each { |x| delete(x) unless yield(x) }
    size == n ? nil : self
  end

  #: () { (E) -> bool } -> Set[E]?
  def filter!(&block) = select!(&block)

  #: () { (E) -> bool } -> Set[E]?
  def reject!
    n = size
    to_a.each { |x| delete(x) if yield(x) }
    size == n ? nil : self
  end

  #: () { (E) -> bool } -> self
  def delete_if
    to_a.each { |x| delete(x) if yield(x) }
    self
  end

  #: () { (E) -> bool } -> self
  def keep_if
    to_a.each { |x| delete(x) unless yield(x) }
    self
  end

  #: (Set[E]) -> self
  def subtract(xs)
    xs.each { |x| delete(x) }
    self
  end

  #: (Array[E]) -> self
  def __subtract_array(xs)
    xs.each { |x| delete(x) }
    self
  end

  #: (Set[E]) -> self
  def replace(xs)
    items = xs.to_a
    clear
    items.each { |x| add(x) }
    self
  end

  #: (Array[E]) -> self
  def __replace_array(xs)
    clear
    xs.each { |x| add(x) }
    self
  end

  #: (?String) -> String
  def join(sep = "") = to_a.join(sep)

  # A Set of plain elements is already flat; a Set of Sets goes to the @self form (decision 92).
  #: () -> Set[E]
  def flatten = Set.new(to_a)

  # @self Set[Set[U]]
  # @rbs [U] () -> Set[U]
  def __flatten_nested
    out = Set.new #: Set[U]
    each { |inner| inner.each { |x| out << x } }
    out
  end

  #: [U] () { (E) -> U } -> Hash[U, Set[E]]
  def classify
    h = {} #: Hash[U, Set[E]]
    each { |x| (h[yield(x)] ||= Set.new) << x }
    h
  end

  #: [U] () { (E) -> U } -> Set[Set[E]]
  def divide(&block) = Set.new(classify(&block).values)

  # one Set stays typed (decision 12: a union naming E blocks dynamic dispatch, growsTypeParams; liquid merges into an untyped Set, decision 331); other counts and classes are twins by count/class, and only __merge_any reaches rbEnumElems (it pulls Range#to_a), so programs that never merge many stay small
  #: (Set[E]) -> self
  def merge(xs)
    xs.each { |x| add(x) }
    self
  end

  #: (Array[E]) -> self
  def __merge_array(xs)
    xs.each { |x| add(x) }
    self
  end

  #: (Range[E]) -> self
  def __merge_range(xs)
    xs.each { |x| add(x) }
    self
  end

  #: () -> self
  def __merge_0 = self

  #: (untyped) -> self
  def __merge_any(en) = %x{
    elems, ok := rbEnumElems(any(en))
    if !ok {
      panic(NewArgumentError(Ref(String("value must be enumerable"))))
    }
    for _, e := range elems.s {
      Set_Add(self, e.(E))
    }
    return self
  }

  #: (untyped, untyped) -> self
  def __merge_2(a, b) = __merge_any(a).__merge_any(b)

  #: (untyped, untyped, untyped) -> self
  def __merge_3(a, b, c) = __merge_any(a).__merge_any(b).__merge_any(c)

  #: (untyped, untyped, untyped, untyped) -> self
  def __merge_4(a, b, c, d) = __merge_any(a).__merge_any(b).__merge_any(c).__merge_any(d)

  #: (Set[E] | Array[E]) -> Set[E]
  def |(o) = o.is_a?(Array) ? dup.__merge_array(o) : dup.merge(o)

  #: (Set[E] | Array[E]) -> Set[E]
  def union(o) = self | o

  #: (Set[E] | Array[E]) -> Set[E]
  def +(o) = self | o

  # MRI walks the argument unless it is a bigger Set: the smaller side's order wins.
  #: (Set[E] | Array[E]) -> Set[E]
  def &(o)
    out = Set.new #: Set[E]
    if o.is_a?(Array)
      o.each { |x| out << x if include?(x) }
    elsif o.size > size
      each { |x| out << x if o.include?(x) }
    else
      o.each { |x| out << x if include?(x) }
    end
    out
  end

  #: (Set[E] | Array[E]) -> Set[E]
  def intersection(o) = self & o

  #: (Set[E] | Array[E]) -> Set[E]
  def -(o) = Set.new(to_a.reject { |x| o.include?(x) })

  #: (Set[E] | Array[E]) -> Set[E]
  def difference(o) = self - o

  # Self-only elements in self's order, then the argument's own.
  #: (Set[E] | Array[E]) -> Set[E]
  def ^(o) = (self | o) - (self & o)

  #: (Set[E]) -> bool
  def subset?(o) = size <= o.size && all? { |x| o.include?(x) }

  #: (Set[E]) -> bool
  def <=(o) = subset?(o)

  #: (Set[E]) -> bool
  def superset?(o) = o.subset?(self)

  #: (Set[E]) -> bool
  def >=(o) = superset?(o)

  #: (Set[E]) -> bool
  def proper_subset?(o) = size < o.size && subset?(o)

  #: (Set[E]) -> bool
  def <(o) = proper_subset?(o)

  #: (untyped) -> Integer?
  def <=>(other) = %x{
    a, ok := other.(Set_Any)
    if !ok {
      return rbOpt[Integer]{}
    }
    s, o := self._ToAny(), a._ToAny()
    ns, no := Set_Size(s), Set_Size(o)
    switch {
    case ns == no && bool(Set_SubsetQ(s, o)):
      return rbSome[Integer](0)
    case ns < no && bool(Set_SubsetQ(s, o)):
      return rbSome[Integer](-1)
    case ns > no && bool(Set_SubsetQ(o, s)):
      return rbSome[Integer](1)
    }
    return rbOpt[Integer]{}
  }

  #: (Set[E]) -> bool
  def proper_superset?(o) = o.proper_subset?(self)

  #: (Set[E]) -> bool
  def >(o) = proper_superset?(o)

  #: (Set[E] | Array[E]) -> bool
  def disjoint?(o) = none? { |x| o.include?(x) }

  #: (Set[E] | Array[E]) -> bool
  def intersect?(o) = !disjoint?(o)

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Set[E])
    if !ok {
      if a, isSet := other.(Set_Any); isSet {
        return self._ToAny().Op_eq(a._ToAny())
      }
      return false
    }
    return Boolean(Set_Size(o) == Set_Size(self) && bool(Set_SubsetQ(self, o)))
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{
    var h Integer
    for k := range Hash_EachKey(self.h) {
      h += rbHash(k) // order-independent, as == is
    }
    return h
  }

  #: () -> String
  def inspect = "Set[#{to_a.map(&:inspect).join(", ")}]"

  #: () -> String
  def to_s = inspect

  #: () -> Set[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Set[any]); ok {
      return same
    }
    out := NewSet[any]()
    for k := range Hash_EachKey(self.h) {
      Set_Add(out, rbUnbox(k))
    }
    return out
  }
end
