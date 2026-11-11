# rbs_inline: enabled

require "minitest/autorun"

# Generic struct classes (#104, decision 308): one Go struct per instantiation, one class object.

module GenericClassTests
  # @rbs generic T
  class Box
    #: (T) -> void
    def initialize(v)
      @v = v
    end

    #: () -> T
    def get = @v

    #: (T) -> void
    def set(v)
      @v = v
    end

    #: () -> Array[T]
    def twice = [@v, @v]
  end

  # @rbs generic E
  class Stack
    #: (Array[E]) -> void
    def initialize(items)
      @items = items
    end

    #: (E) -> self
    def push(x)
      @items << x
      self
    end

    #: () -> E?
    def pop = @items.pop

    #: () -> Integer
    def size = @items.size

    #: () -> Box[E]?
    def top_box
      x = @items.last
      return nil if x.nil?

      Box.new(x)
    end
  end

  # @rbs generic E
  class Bag
    include Enumerable #[E]

    #: (Array[E]) -> void
    def initialize(items)
      @items = items
    end

    #: () { (E) -> void } -> void
    def each(&)
      @items.each { |x| yield x }
    end
  end

  # @rbs generic K
  # @rbs generic V
  class Pair
    attr_reader :k #: K
    attr_reader :v #: V

    #: (K, V) -> void
    def initialize(k, v)
      @k = k
      @v = v
    end

    #: () -> Pair[V, K]
    def swap = Pair.new(@v, @k)
  end

  # @rbs generic E
  # @rbs generic P
  class Heap
    #: () -> void
    def initialize
      @items = [] #: Array[[E, P]]
    end

    #: (E, P) -> self
    def push(e, pr)
      @items << [e, pr]
      self
    end

    #: () -> Integer
    def size = @items.size

    #: () -> E?
    def pop
      best = @items.min_by { |pair| pair[1] }
      return nil if best.nil?

      @items.delete(best)
      best[0]
    end
    alias dequeue pop
  end

  # @rbs generic E
  # @rbs generic P
  class MinQ < Heap #[E, P]
    alias_method :extract_min, :dequeue

    #: (E, P) -> self
    def push(e, pr)
      super
    end
  end

  # @rbs generic A
  # @rbs generic B
  class Flipped < Heap #[B, A]
  end

  class IntHeap < Heap #[Integer, Integer]
    #: () -> Integer
    def total = size * 10
  end

  # @rbs generic T
  class Chain
    class Link
      attr_accessor :val, :rest

      def initialize(val, rest)
        @val = val
        @rest = rest
      end
    end

    # @rbs @head: Link?

    #: () -> void
    def initialize
      @head = nil
    end

    #: (T) -> self
    def push(v)
      @head = Link.new(v, @head)
      self
    end

    #: () -> Array[T]
    def to_a
      out = [] #: Array[T]
      l = @head
      while l
        out << l.val
        l = l.rest
      end
      out
    end
  end

  # @rbs generic P < Comparable
  class Best
    #: (P) -> void
    def initialize(v)
      @v = v
    end

    #: (P) -> P
    def better(o) = o < @v ? o : @v
  end

  module Twice
    def twice(x) = combine(x, x)
  end

  # @rbs generic T
  class Doubler
    include Twice

    #: (T) -> void
    def initialize(v)
      @v = v
    end

    #: (T, T) -> T
    def combine(a, b) = a + b

    #: () -> T
    def doubled = twice(@v)
  end

  class Point
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end
  end

  class GenericClassTest < Minitest::Test
    def test_two_instantiations
      a = Box.new(1)
      s = Box.new("a")
      assert_equal 3, a.get + 2
      assert_equal "ab", s.get + "b"
      a.set(5)
      assert_equal [5, 5], a.twice
    end

    def test_annotated_instance
      b = Box.new(nil) #: Box[Integer?]
      assert_nil b.get
      b.set(3)
      assert_equal 3, b.get
    end

    def test_class_and_is_a
      a = Box.new(1)
      s = Box.new("a")
      assert_equal Box, a.class
      assert_same a.class, s.class
      assert a.is_a?(Box)
      assert_kind_of Box, s
      assert_equal "GenericClassTests::Box", Box.name
    end

    def test_inspect_and_ivars
      a = Box.new(7)
      assert_match(/\A#<GenericClassTests::Box:0x\h+ @v=7>\z/, a.inspect)
      assert_equal 7, a.instance_variable_get(:@v)
      assert_equal [:@v], a.instance_variables
    end

    def test_stack_instances
      ints = Stack.new([1, 2])
      strs = Stack.new(["x"])
      ints.push(3).push(4)
      assert_equal 4, ints.size
      assert_equal 4, ints.pop
      assert_equal "x", strs.pop
      assert_nil strs.pop
      assert_equal 3, ints.top_box&.get
    end

    def test_class_argument
      pts = Stack.new([Point.new(1)])
      pts.push(Point.new(2))
      assert_equal 2, pts.pop&.x
    end

    def test_enumerable_instances
      ints = Bag.new([3, 1, 2])
      strs = Bag.new(%w[b a])
      assert_equal [1, 2, 3], ints.sort
      assert_equal [6, 2, 4], ints.map { |x| x * 2 }
      assert_equal "a", strs.min
      assert strs.include?("a")
    end

    def test_two_parameters
      pr = Pair.new(1, "one")
      assert_equal "one", pr.swap.k
      assert_equal 1, pr.swap.v
      assert_equal "one", pr.swap.swap.v
      assert_instance_of Pair, pr.swap
      refute_instance_of Box, pr
    end

    def test_generic_superclass
      q = MinQ.new #: MinQ[String, Integer]
      q.push("b", 2).push("a", 1)
      assert_equal "a", q.extract_min
      assert_equal 1, q.size
      assert_kind_of Heap, q
      assert_equal Heap, MinQ.superclass
      assert_instance_of MinQ, q
    end

    def test_open_new_typed_by_later_calls
      q = MinQ.new
      q.push("b", 2)
      q.push("a", 1)
      assert_equal "a", q.extract_min
      h = Heap.new
      h.push(:x, 1.5)
      h.push(7, 0.5)
      assert_equal 7, h.pop
    end

    def test_nested_helper_takes_outer_params
      ints = Chain.new #: Chain[Integer]
      strs = Chain.new #: Chain[String]
      ints.push(1).push(2)
      strs.push("x").push("y")
      assert_equal [2, 1], ints.to_a
      assert_equal ["y", "x"], strs.to_a
      assert_equal 3, ints.to_a.sum
    end

    def test_bounded_parameter
      assert_equal 1, Best.new(3).better(1)
      assert_equal "b", Best.new("b").better("c")
    end

    def test_module_method_per_instance
      assert_equal 4, Doubler.new(2).doubled
      assert_equal "abab", Doubler.new("ab").doubled
    end

    def test_remapped_superclass_args
      f = Flipped.new #: Flipped[Integer, String]
      f.push("x", 3)
      assert_equal "x", f.pop
    end

    def test_non_generic_child
      h = IntHeap.new
      h.push(5, 5).push(3, 3)
      assert_equal 3, h.pop
      assert_equal 10, h.total
      assert_equal [IntHeap, Heap, Object], IntHeap.ancestors.take(3)
      held = h #: untyped
      assert Heap === held
    end

    def test_untyped_receiver
      items = [Box.new(1), Box.new("a")] #: Array[untyped]
      assert_equal [1, "a"], items.map(&:get)
      assert_equal [true, true], items.map { |x| x.is_a?(Box) }
      assert_equal 1, items[0].send(:get)
      assert_equal [Box, Box], items.map(&:class)
    end

    def test_case_when
      v = Box.new(1) #: untyped
      kind = case v
             when Stack then :stack
             when Box then :box
             else :other
             end
      assert_equal :box, kind
      assert Box === v
      refute Stack === v
    end
  end

  # A generic class's C? and a subclassed class's C? are their Go interface, nil Go's nil (decision 378).
  # @rbs generic T
  class NilNode
    attr_accessor :val #: T
    attr_accessor :next_node #: NilNode[T]?

    #: (T val) -> void
    def initialize(val)
      @val = val
      @next_node = nil
    end
  end

  class NilShape
    #: () -> String
    def name = "shape"
  end

  class NilSquare < NilShape
    #: () -> String
    def name = "square"
  end

  class NilLeaf < NilShape
    #: () -> String
    def name = "leaf"
  end

  class NilableObjectTest < Minitest::Test
    def test_generic_optional
      a = NilNode.new(1) #: NilNode[Integer]
      assert_nil a.next_node
      assert_nil a.next_node&.val
      a.next_node ||= NilNode.new(2)
      a.next_node ||= NilNode.new(3)
      assert_equal 2, a.next_node&.val
      n = a #: NilNode[Integer]?
      sum = 0
      while n
        sum += n.val
        n = n.next_node
      end
      assert_equal 3, sum
      a.next_node = nil
      assert a.next_node.nil?
      refute a.next_node
      x = a.next_node #: untyped
      assert_nil x
    end

    def test_subclassed_optional
      s = nil #: NilShape?
      assert_nil s&.name
      s ||= NilSquare.new
      assert_equal "square", s.name
      s = nil
      assert_equal "none", s ? s.name : "none"
      shapes = [NilSquare.new, nil, NilShape.new] #: Array[NilShape?]
      assert_equal ["square", "shape"], shapes.compact.map(&:name)
      assert_equal 1, shapes.count(&:nil?)
    end

    def test_nil_leaf_into_parent_optional
      leaf = nil #: NilLeaf?
      parent = leaf #: NilShape?
      assert_nil parent
      assert parent.nil?
      untyped = parent #: untyped
      assert_nil untyped
      leaf = NilLeaf.new
      parent = leaf
      assert_equal "leaf", parent&.name
      h = {} #: Hash[Symbol, NilShape]
      assert_nil h[:missing]
      h[:a] = NilSquare.new
      assert_equal "square", h[:a]&.name
    end

    # #130: Lazy#compact on C? for a non-leaf, a leaf and a generic class
    def test_lazy_compact
      shapes = [NilSquare.new, nil, NilShape.new] #: Array[NilShape?]
      assert_equal ["square", "shape"], shapes.lazy.compact.map(&:name).to_a
      leaves = [nil, NilLeaf.new, nil] #: Array[NilLeaf?]
      assert_equal ["leaf"], leaves.lazy.compact.map(&:name).first(5)
      nodes = [NilNode.new(1), nil, NilNode.new(2)] #: Array[NilNode[Integer]?]
      assert_equal [1, 2], nodes.lazy.compact.map(&:val).to_a
    end
  end
end
