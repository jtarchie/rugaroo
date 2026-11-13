# rbs_inline: enabled

require "minitest/autorun"

# Parameter types from use (#56, decision 146): no annotation on these
# defs, lambdas or blocks; each takes its types from the calls the
# program makes.

def infer_twice(x) = x * 2
def infer_bigger(e) = e.select { |x| x > 1 }

def infer_kwrest_size(**opts) = opts.size

# an Array of Hash literals whose values mix classes: the parameter is Array[Hash[Symbol, untyped]], so its block compiles (decision 447)
def infer_mixed_rows(items)
  out = []
  items.each_with_index { |x, i| out << "#{i}:#{x[:title]}:#{x[:price] * 2}" }
  out
end

def infer_mixed_opts(o)
  o.map { |k, v| "#{k}=#{v}" }
end

#: (Integer, ?Integer, ?Integer) -> Array[Integer]
def infer_three(a, b = 10, c = 20) = [a, b, c]

module InferTests
  class FallbackBlock
    def run(&blk) = blk.call(1, 2)
    def maybe(&blk) = blk ? blk.call(5) : :none
  end

  class FallbackBase
    include Enumerable
  end

  class FallbackKid < FallbackBase
    def each
      yield 1
      yield 2
    end
  end

  class FallbackFeed
    def each
      out = [] #: Array[untyped]
      out << yield
      out << yield
      out
    end
  end

  class FallbackPairs
    include Enumerable

    def each
      yield [:a, 1]
      yield [:b, 2]
    end
  end

  class FallbackCmp
    attr_reader :n

    def initialize(n) = @n = n
    def <=>(other) = n <=> other.n
  end
end

def infer_chain(y) = infer_twice(y) + 1
def infer_label(v, prefix = "n=") = "#{prefix}#{v}"
def infer_maybe(z) = z.nil? ? "none" : z.to_s
def infer_kw(a:, b: 10) = a + b

module InferTests
  class Numerous
    include Enumerable

    def initialize(*list)
      @list = list.empty? ? [2, 5, 3, 6, 1, 4] : list
    end

    def each
      @list.each { |i| yield i }
    end
  end

  class Pairs
    def initialize(names)
      @names = names
    end

    def each_pair
      @names.each_with_index { |n, i| yield i, n }
    end

    def first_or(fallback)
      return fallback unless block_given?

      yield @names.first
      fallback
    end
  end

  class Shape
    def initialize(sides) = (@sides = sides)
    def sides = @sides
  end

  class Square < Shape
    def initialize = super(4)
  end

  class Walker
    #: () { (Integer) -> void } -> void
    def walk
      yield 1
    end
  end

  class BlockWalker < Walker
    def walk(&blk) # an unannotated override with a &block takes Walker#walk's signature
      blk.call(2)
    end
  end

  class InferTest < Minitest::Test
    def test_method_params
      assert_equal [41, "n=3", "x:4", "none", "5"], [infer_chain(20), infer_label(3), infer_label(4, "x:"), infer_maybe(nil), infer_maybe(5)]
      assert_equal [11, 3], [infer_kw(a: 1), infer_kw(a: 1, b: 2)]
      assert_equal [3, 4], [Shape.new(3).sides, Square.new.sides]
      walked = [] #: Array[Integer]
      BlockWalker.new.walk { |i| walked << i }
      assert_equal [2], walked
    end

    def test_enumerable_from_each
      assert_equal [[1, 2, 3, 4, 5, 6], [30, 10], 1, 21], [Numerous.new.sort, Numerous.new(3, 1).map { |x| x * 10 }, Numerous.new.min, Numerous.new.sum]
    end

    def test_block_from_yield
      out = [] #: Array[String]
      Pairs.new(%w[a b]).each_pair { |i, n| out << "#{i}#{n}" }
      assert_equal %w[0a 1b], out
      seen = [] #: Array[String?]
      assert_equal [7, 8], [Pairs.new(["z"]).first_or(7), Pairs.new(["q"]).first_or(8) { |s| seen << s }]
      assert_equal ["q"], seen
    end

    def test_lambda_params
      add = ->(a, b) { a + b }
      shout = lambda { |s| s.upcase }
      double = proc { |x| x * 2 }
      assert_equal [3, 7, 11, "HI", [2, 4]], [add.call(1, 2), add.(3, 4), add[5, 6], shout.call("hi"), [1, 2].map(&double)]
    end

    # A *rest lambda is a Go variadic func: each call passes any count past
    # the leading params, and the rest's element joins them (decision 152).
    def test_variadic_lambda
      fmt = ->f, *args { f % args }
      assert_equal ["1", "1 2", "1.5 x"], [fmt.call("%d", 1), fmt.("%d %d", 1, 2), fmt["%.1f %s", 1.5, "x"]]
      assert_equal "3-a", ->f, *args { f % args }.call("%d-%s", 3, "a")
      total = ->(*xs) { xs.sum }
      assert_equal [6, 0, -1, -2], [total.call(1, 2, 3), total.call, total.arity, fmt.arity]
      first = ->(a, *) { a }
      assert_equal 1, first.call(1, 2, 3)
    end

    def test_block_on_a_parameter_its_call_types
      take = ->e { e.reject { |x| x > 1 } }
      assert_equal [[2, 3], [1]], [infer_bigger([1, 2, 3]), take.call([1, 2, 3])]
    end

    def test_untyped_fallbacks
      r = InferTests::FallbackBlock.new
      sorted = [3, 1, 2].map { |i| InferTests::FallbackCmp.send(:new, i) }.sort.map(&:n)
      assert_equal ["a", "a"], InferTests::FallbackFeed.new.each { "a" }
      assert_equal [3, 10, :none, [2, 4], [1, 2, 3]], [r.run { |a, b| a + b }, r.maybe { |x| x * 2 }, r.maybe, InferTests::FallbackKid.new.map { |x| x * 2 }, sorted]
    end

    def test_enumerable_to_h_and_unannotated_kwrest
      assert_equal [{ a: 1, b: 2 }, 2, 0], [InferTests::FallbackPairs.new.to_h, infer_kwrest_size(x: 1, y: 2), infer_kwrest_size]
    end

    def test_splat_into_positional_parameters
      xs = [1, 2]
      e = assert_raises(ArgumentError) { infer_three(*[]) }
      assert_equal [[1, 2, 20], [0, 5, 20], [7, 10, 20], "wrong number of arguments (given 0, expected 1..3)"],
                   [infer_three(*xs), infer_three(0, *[5]), infer_three(*[7]), e.message]
    end
  end

  # An unannotated initialize inherits its parent's signature only when its super passes its own parameters through
  # (decision 286): Imm's `name` is a String, not the parent's Ev.
  class InitEv
    #: (String) -> void
    def initialize(name)
      @name = name
    end

    def label = "ev #{@name}"
  end

  class InitBase
    #: (InitEv) -> void
    def initialize(ev)
      @ev = ev
    end

    def label = @ev.label
  end

  class InitImm < InitBase
    def initialize(name)
      super(InitEv.new(name))
    end
  end

  class InitPass < InitBase
    def initialize(ev)
      super(ev)
    end
  end

  class InitInheritTest < Minitest::Test
    def test_initialize_inherits_only_through_a_passing_super
      assert_equal ["ev x", "ev y"], [InitImm.new("x").label, InitPass.new(InitEv.new("y")).label]
    end
  end

  # Decision 307: nil-initialized links, a stored &block and an includer-only self call all type statically.
  module LinkMerge
    def link_merge_all(first)
      acc = first
      cur = first&.sibling
      while cur
        nxt = cur.sibling
        acc = meld(acc, cur)
        cur = nxt
      end
      acc
    end
  end

  class LinkNode
    attr_accessor :val, :child, :sibling

    def initialize(val)
      @val = val
      @child = nil
      @sibling = nil
    end
  end

  class LinkName
    attr_accessor :name, :child, :sibling

    def initialize(name)
      @name = name
      @child = nil
      @sibling = nil
    end
  end

  class LinkHeap
    include LinkMerge

    def initialize(&block)
      @root = nil
      @nodes = {}
      @order = block || :<=.to_proc
    end

    def push(v)
      n = LinkNode.new(v)
      @nodes[v] = n
      @root = @root ? meld(@root, n) : n
      self
    end

    def meld(a, b)
      if @order[a.val, b.val]
        b.sibling = a.child
        a.child = b
        a
      else
        a.sibling = b.child
        b.child = a
        b
      end
    end

    def node(v) = @nodes[v]

    def pop
      r = @root
      return nil unless r

      @nodes.delete(r.val)
      @root = link_merge_all(r.child)
      r.val
    end
  end

  class LinkNames
    include LinkMerge

    def initialize
      @root = nil
    end

    def push(s)
      n = LinkName.new(s)
      @root = @root ? meld(@root, n) : n
      self
    end

    def meld(a, b)
      if a.name.length <= b.name.length
        b.sibling = a.child
        a.child = b
        a
      else
        a.sibling = b.child
        b.child = a
        b
      end
    end

    def pop
      r = @root
      return nil unless r

      @root = link_merge_all(r.child)
      r.name
    end
  end

  class LinkTest < Minitest::Test
    def test_nil_links_typed_by_their_nodes
      h = LinkHeap.new
      [5, 3, 8, 1, 9, 2].each { |x| h.push(x) }
      assert_equal 8, h.node(8)&.val
      out = [] #: Array[Integer]
      while (x = h.pop)
        out << x
      end
      assert_equal [1, 2, 3, 5, 8, 9], out
      assert_nil h.node(8)
      d = LinkHeap.new { |a, b| a >= b }
      [5, 3, 8].each { |x| d.push(x) }
      assert_equal [8, 5], [d.pop, d.pop]
      n = LinkNames.new
      %w[ccc a bb].each { |w| n.push(w) }
      assert_equal ["a", "bb", "ccc", nil], [n.pop, n.pop, n.pop, n.pop]
    end
  end

  # Decisions 355-357: recursion, a nil-initialized tree typed in a branch, and call-survivor narrowing, all against MRI.
  class RecNode
    attr_accessor :key, :left, :right

    def initialize(key, left, right)
      @key = key
      @left = left
      @right = right
    end
  end

  class RecTree
    def initialize
      @root = nil
      @size = 0
    end

    def empty_p = @root.nil?

    def add(key)
      if empty_p
        @root = RecNode.new(key, nil, nil)
      else
        t = @root
        while true
          if key < t.key
            l = t.left
            if l.nil?
              t.left = RecNode.new(key, nil, nil)
              break
            end
            t = l
          else
            r = t.right
            if r.nil?
              t.right = RecNode.new(key, nil, nil)
              break
            end
            t = r
          end
        end
      end
      @size += 1
    end

    def leftmost
      t = @root
      return nil if t.nil?

      while true
        l = t.left
        break if l.nil?

        if l.key.even?
          t = l
          l = t.left
          break if l.nil?
        end
        t = l
      end
      t.key
    end

    def root_key = @root&.key
    def size = @size
  end

  class RecTest < Minitest::Test
    def test_recursive_returns
      assert_equal 120, infer_fact(5)
      assert_equal [true, false], [infer_even(10), infer_even(7)]
      assert_equal 2, infer_first_over([1, 2, 5, 3], 4)
      assert_equal 0, infer_first_over([1, 2], 4)
      assert_nil infer_first_over([], 4)
      t = infer_make_tree(3)
      assert_equal 15, infer_count_tree(t)
      assert_nil infer_make_tree(0).left
      assert_equal [nil, 5, nil, nil, 5], [infer_tri_a(0), infer_tri_a(1), infer_tri_a(2), infer_tri_a(3), infer_tri_a(4)]
    end

    def test_nil_initialized_tree_links
      t = RecTree.new
      assert_nil t.root_key
      [50, 30, 70, 20, 40, 10].each { |k| t.add(k) }
      assert_equal [50, 6], [t.root_key, t.size]
      assert_equal 10, t.leftmost
      a = RecLeaf.new(1)
      a.nxt = RecLeaf.new(2)
      a.nxt&.nxt = RecLeaf.new(3)
      assert_equal 6, a.sum
    end
  end

  class RecLeaf
    attr_accessor :val, :nxt

    def initialize(val)
      @val = val
      @nxt = nil
    end

    def sum
      s = @val
      t = @nxt
      while t
        s += t.val
        t = t.nxt
      end
      s
    end
  end
end

def infer_fact(n) = n <= 1 ? 1 : n * infer_fact(n - 1)

def infer_even(n)
  return true if n == 0

  infer_odd(n - 1)
end

def infer_odd(n)
  return false if n == 0

  infer_even(n - 1)
end

def infer_first_over(xs, i)
  return nil if xs.empty?
  return xs.size - 1 if xs.last > i

  infer_first_over(xs[0...-1], i) || 0
end

def infer_make_tree(depth)
  return InferTests::RecNode.new(depth, nil, nil) if depth == 0

  InferTests::RecNode.new(depth, infer_make_tree(depth - 1), infer_make_tree(depth - 1))
end

def infer_count_tree(node)
  return 1 if node.left.nil?

  1 + infer_count_tree(node.left) + infer_count_tree(node.right)
end

def infer_tri_a(n) = n <= 0 ? nil : infer_tri_b(n - 1)
def infer_tri_b(n) = n <= 0 ? 5 : infer_tri_c(n - 1)
def infer_tri_c(n) = n <= 0 ? nil : infer_tri_a(n - 1)

module InferTests
  class CycleBox
    def bump(x)
      (x ^ 1) + 1
    end
  end

  CyclePair = Struct.new(:a, :b)

  class CycleTest < Minitest::Test
    def test_param_typed_through_its_own_result
      box = CycleBox.new
      t = 0
      i = 0
      while i < 4
        t = box.bump(t + i)
        i += 1
      end
      assert_equal 10, t
      3.times { |k| t = box.bump(t + k) }
      assert_equal 15, t
    end

    def test_struct_members_from_new
      pr = CyclePair.new(1, "x")
      pr.a = pr.a + 2
      pr.b += "y"
      assert_equal [3, "xy"], [pr.a, pr.b]
      assert_equal "#<struct InferTests::CyclePair a=3, b=\"xy\">", pr.inspect
      assert_equal CyclePair.new(3, "xy"), pr
    end
  end

  # Decision 383: an ivar typed through a call whose parameter it types (`@root = meld(@root, n)`), and a class body's
  # unannotated `@levar = 1`.
  class IvCycNode
    attr_accessor :val, :child

    def initialize(val)
      @val = val
      @child = nil
    end
  end

  class IvCycHeap
    def initialize
      @root = nil
    end

    def push(v)
      @root = meld(@root, IvCycNode.new(v))
    end

    def meld(a, b)
      return b if a.nil?

      if b.val < a.val
        b.child = a
        b
      else
        a.child = b
        a
      end
    end

    def peek = @root&.val
  end

  class IvCycAcc
    def initialize
      @total = nil
    end

    def add(x) = @total = combine(@total, x)
    def combine(t, x) = t.nil? ? x : t + x
    def total = @total
  end

  class IvCycPing
    def initialize
      @a = nil
      @b = nil
    end

    def step(x)
      @a = left(@b, x)
      @b = right(@a)
    end

    def left(b, x) = b.nil? ? x : b + x
    def right(a) = a * 2
    def show = [@a, @b]
  end

  module IvCycLevar
    @levar = 3

    def self.get = @levar + 1
  end

  # liquid's ResourceLimits: counters `reset` sets from initialize, bumped through their attr writers (decision 394)
  class IvCycLimits
    attr_accessor :length, :score

    def initialize(max)
      @max = max
      reset
    end

    def reached? = @length > @max || @score > @max

    def reset
      @length = @score = 0
    end
  end

  class IvCycBody
    def initialize(nodes)
      @nodes = nodes
    end

    def render(limits, s)
      limits.score += @nodes.length
      limits.length += s.length
      limits.reached?
    end
  end

  class IvarCycleTest < Minitest::Test
    def test_ivar_typed_through_its_own_call
      h = IvCycHeap.new
      assert_nil h.peek
      [5, 3, 8].each { |x| h.push(x) }
      assert_equal 3, h.peek
      acc = IvCycAcc.new
      assert_nil acc.total
      [1, 2, 3].each { |x| acc.add(x) }
      assert_equal 6, acc.total
      pg = IvCycPing.new
      3.times { |i| pg.step(i) }
      assert_equal [4, 8], pg.show
    end

    def test_class_body_literal_ivar
      assert_equal 4, IvCycLevar.get
    end

    def test_ivar_typed_through_its_writer
      l = IvCycLimits.new(9)
      b = IvCycBody.new([1, 2])
      refute b.render(l, "abc")
      assert b.render(l, "defghij")
      assert_equal [10, 4], [l.length, l.score]
    end
  end
end

# An inferred `[]`/`{}` whose only veto is a possibly-nil read keeps its element type; the read is checked at run time
# (decision 440).
#: (Array[String], String) -> Integer
def infer_freq_plus(words, probe)
  freq = {}
  i = 0
  while i < words.length
    freq[words[i]] = i
    i += 1
  end
  100 + freq[probe]
end

module InferTests
  class NilReadBox
    def initialize
      @xs = []
      @xs << 1.5 << 2.5
    end

    #: (Float, Integer) -> bool
    def above?(x, i) = x > @xs[i]

    #: (Float, Integer) -> Float
    def plus(x, i) = x + @xs[i]

    #: (Integer) -> Float
    def bumped(i) = @xs[i] + 1.0
  end

  class NilReadTest < Minitest::Test
    def test_typed_ivar_reads
      b = NilReadBox.new
      assert b.above?(2.0, 0)
      refute b.above?(2.0, 1)
      assert_equal 4.0, b.plus(1.5, 1)
      assert_equal 2.5, b.bumped(0)
    end

    def test_typed_ivar_nil_read_raises
      b = NilReadBox.new
      begin
        b.above?(2.0, 7)
        flunk "no raise"
      rescue ArgumentError => e
        assert_equal ArgumentError, e.class
      end
      begin
        b.plus(2.0, 7)
        flunk "no raise"
      rescue TypeError => e
        assert_equal TypeError, e.class
      end
      begin
        b.bumped(7)
        flunk "no raise"
      rescue NoMethodError => e
        assert_equal NoMethodError, e.class
      end
    end

    def test_typed_local_nil_read
      assert_equal 101, infer_freq_plus(%w[a b c], "b")
      begin
        infer_freq_plus(%w[a b c], "z")
        flunk "no raise"
      rescue TypeError => e
        assert_equal TypeError, e.class
      end
    end
  end

  class MixedLiteralTest < Minitest::Test
    def test_array_of_mixed_hashes
      assert_equal ["0:a:2", "1:b:4"], infer_mixed_rows([{ title: "a", price: 1 }, { title: "b", price: 2 }])
      assert_equal ["a=1", "b=s"], infer_mixed_opts({ a: 1, b: "s" })
    end
  end
end
