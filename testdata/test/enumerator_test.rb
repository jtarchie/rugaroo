# rbs_inline: enabled

require "minitest/autorun"

# Enumerator, external iteration, Enumerator::Lazy and ArithmeticSequence (decision 140), against MRI.
module EnumeratorTests
  class Basket
    include Enumerable #[String]

    #: (*String) -> void
    def initialize(*items)
      @items = items
    end

    #: () { (String) -> void } -> void
    def each
      @items.each { |x| yield x }
    end
  end

  class Drained
    include Enumerable #[Integer]

    #: (Array[Integer]) -> void
    def initialize(xs)
      @xs = xs
    end

    #: () { (Integer) -> void } -> void
    def each
      e = @xs.each
      loop { yield e.next }
    end
  end

  # an Enumerable includer of a leaf class, called through its parent (Homebrew's Options of Option): the forwarder opens
  # Enumerable's E? box to the leaf's *C (decision 360)
  class Flag
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name) = @name = name
  end

  class Flags
    include Enumerable #[Flag]

    #: (Array[Flag]) -> void
    def initialize(a) = @a = a

    #: () { (Flag) -> void } -> void
    def each(&) = @a.each(&)
  end

  class MoreFlags < Flags
  end

  class LeafIncluderTest < Minitest::Test
    #: (Flags) -> String?
    def first_name(fs) = fs.first&.name

    def test_first_through_parent
      assert_equal ["z", nil], [first_name(MoreFlags.new([Flag.new("z")])), first_name(Flags.new([]))]
    end
  end

  class ExternalTest < Minitest::Test
    def test_next_peek_rewind
      e = [10, 20].each
      assert_equal 10, e.next
      assert_equal 20, e.peek
      assert_equal 20, e.next
      err = assert_raises(StopIteration) { e.next }
      assert_equal [10, 20], err.result
      assert_raises(StopIteration) { e.peek }
      e.rewind
      assert_equal 10, e.next
    end

    def test_blockless_forms
      t = 3.times
      assert_equal [0, 1, 2], [t.next, t.next, t.next]
      assert_equal 3, t.size
      c = "héy".each_char
      assert_equal ["h", "é"], [c.next, c.next]
      assert_equal 3, c.size
      l = "a\nb\n".each_line
      assert_equal "a\n", l.next
      assert_nil l.size
      w = %w[a b].each_with_index
      assert_equal ["a", 0], w.next
      assert_equal ["b", 1], w.next
      s = [1, 2, 3].each_slice(2)
      assert_equal [1, 2], s.next
      assert_equal [[1, 2], [3]], s.to_a
      assert_equal 2, s.size
      k = [1, 2, 3].each_cons(2)
      assert_equal [1, 2], k.next
      assert_equal [2, 3], k.peek
      assert_equal 2, k.size
      m = [4, 5].map
      assert_equal 4, m.next
      assert_equal [8, 10], m.with_index { |x, i| x * 2 + i * 0 }
      u = 1.upto(3)
      assert_equal [1, 2], [u.next, u.next]
      assert_equal 3, u.size
      assert_equal [3, 2, 1], 3.downto(1).to_a
    end

    def test_prelude_iterators_without_a_block
      r = (1..3).each
      assert_equal [1, 2], [r.next, r.next]
      h = { a: 1, b: 2 }.each
      assert_equal [:a, 1], h.next
      assert_equal [[:a, 0]], { a: 1 }.each_with_index.map { |(k, _v), i| [k, i] }
      assert_equal 97, "abc".each_byte.next
      assert_equal [2, 1], [1, 2].reverse_each.to_a
    end

    def test_chains
      assert_equal [[1, 1], [2, 2]], [1, 2].each.with_index(1).to_a
      got = []
      [1, 2].each.with_index(1) { |x, i| got << x * i }
      assert_equal [1, 4], got
      assert_equal %w[a0 b1], %w[a b].each_with_index.map { |x, i| "#{x}#{i}" }
      assert_equal [3, 6], [1, 2].each.with_object([]) { |x, acc| acc << x * 3 }
      assert_equal [3, 3], [1, 2, 3].each_slice(2).map(&:sum)
      assert_equal ["h", "ee"], "he".each_char.with_index.map { |ch, i| ch * (i + 1) }
      assert_equal [[3, 1], ["x"]], [Basket.new("3", "1").each_slice(2).first.map(&:to_i), Basket.new("x").to_a]
      assert_equal [["a", 0], ["b", 1]], Basket.new("a", "b").each_with_index.to_a
    end

    def test_loop_rescues_stop_iteration
      e = [1, 2, 3].each
      seen = []
      result = loop do
        seen << e.next
      end
      assert_equal [1, 2, 3], seen
      assert_equal [1, 2, 3], result
      n = 0
      loop do
        n += 1
        raise StopIteration if n == 3
      end
      assert_equal 3, n
      assert_equal [5, 6], Drained.new([4, 5]).map { |x| x + 1 }
      src = [1, 2, 3].each
      doubled = Enumerator.new do |y|
        loop { y << src.next * 2 }
      end
      assert_equal [2, 4, 6], doubled.to_a
    end

    def test_inspect
      assert_equal "#<Enumerator: [1, 2]:each>", [1, 2].each.inspect
      assert_equal "#<Enumerator: 5:times>", 5.times.inspect
      assert_equal "#<Enumerator: [1, 2, 3]:each_slice(2)>", [1, 2, 3].each_slice(2).inspect
      assert_equal "#<Enumerator: \"ab\":each_char>", "ab".each_char.inspect
      assert_equal "#<Enumerator: 1..3:each>", (1..3).each.inspect
      assert_equal "#<Enumerator: #<Enumerator: [1]:each>:with_index(1)>", [1].each.with_index(1).inspect
    end

    def test_sizes
      assert_nil ("a".."e").each_slice(2).size
      assert_equal 3, (1..5).each_slice(2).size
      assert_equal 1, { a: 1 }.each_with_index.size
      assert_equal 2, ((1..10) % 3).each_slice(2).size
      assert_equal 3, ((1...10) % 3).size
      assert_equal 0, ((10..1) % 3).size
      assert_equal 4, 10.step(1, -3).size
      assert_equal 500_000_000_000, ((1..10**12) % 2).size
    end
  end

  class GeneratorTest < Minitest::Test
    #: () -> Enumerator[Integer]
    def fib
      Enumerator.new do |y|
        a, b = 0, 1
        loop do
          y << a
          a, b = b, a + b
        end
      end
    end

    def test_take_first_next
      f = fib
      assert_equal [0, 1, 1, 2, 3, 5, 8, 13], f.take(8)
      assert_equal 0, f.first
      assert_equal [0, 1, 1], f.first(3)
      assert_equal [0, 1, 1], [f.next, f.next, f.next]
      assert_equal 2, f.peek
      f.rewind
      assert_equal 0, f.next
      assert_nil f.size
      assert_equal [[0, 1], [1, 2]], f.each_slice(2).first(2)
      assert_equal [[0, 1], [1, 2]], f.with_index(1).take(2)
    end

    def test_finite_generator
      squares = Enumerator.new do |y|
        [1, 2, 3].each { |x| y.yield x * x }
      end
      assert_equal [1, 4, 9], squares.to_a
      assert_equal [2, 5, 10], squares.map { |x| x + 1 }
      assert squares.include?(4)
      assert_equal [4], squares.select(&:even?)
      assert_equal 14, squares.sum
      assert_equal [[1, 1], [4, 2], [9, 3]], squares.with_index(1).to_a
      s = squares.each
      assert_equal [1, 4, 9], [s.next, s.next, s.next]
      assert_raises(StopIteration) { s.next }
      w = [7, 8].each.with_index
      2.times { w.next }
      assert_equal [7, 8], assert_raises(StopIteration) { w.next }.result
    end

    def test_yielder_forms
      words = Enumerator.new { |y| %w[a b].each(&y) }
      assert_equal %w[a b], words.to_a
      procs = Enumerator.new { |y| y << 0; y.to_proc.call(5) }
      assert_equal [0, 5], procs.to_a
      calls = Enumerator.new { |y| y << "c" << "d" }
      assert_equal %w[c d], calls.to_a
      sized = Enumerator.new(3) { |y| y << "x" }
      assert_equal 3, sized.size
    end

    def test_generator_stops_where_the_consumer_does
      log = []
      gen = Enumerator.new do |y|
        y << 1
        log << :after_one
        y << 2
        log << :after_two
      end
      assert_equal [1], gen.first(1)
      assert_equal [], log
      assert_equal [1, 2], gen.take(2)
      assert_equal [:after_one], log
    end
  end

  class LazyTest < Minitest::Test
    def test_pipelines
      assert_equal [6, 12, 18], (1..).lazy.map { |x| x * 2 }.select { |x| x % 3 == 0 }.first(3)
      assert_equal [2, 4, 6], (1..20).lazy.reject(&:odd?).take(3).to_a
      assert_equal [4, 5], (1..).lazy.drop(3).drop_while { |x| x < 4 }.first(2)
      assert_equal [1, -1, 2, -2], (1..).lazy.flat_map { |x| [x, -x] }.first(4)
      assert_equal [[1, 10], [2, 20], [3, nil]], (1..3).lazy.zip([10, 20]).to_a
      assert_equal [1, 4, 9], (1..).lazy.with_index(1).map { |x, i| x * i }.first(3)
      assert_equal [[1, 0], [2, 1]], (1..).lazy.each_with_index.first(2)
      assert_equal [1, 2, 3], [1, 1, 2, 3, 3].lazy.uniq.force
      assert_equal [1, 2], [1, nil, 2].lazy.compact.to_a
      assert_equal [4, 8], (1..).lazy.filter_map { |x| x * 2 if x.even? }.first(2)
      assert_equal %w[1 2], (1..).lazy.map(&:to_s).take_while { |s| s.size < 2 }.first(2)
      assert_equal [2, 4, 6], (1..3).lazy.map { |x| x * 2 }.eager.to_a
      assert (1..).lazy.map { |x| x * x }.include?(49)
      assert_equal 20, (1..4).lazy.map { |x| x * 2 }.sum
      assert_equal 6, (1..).lazy.select(&:even?).map { |x| x * 3 }.first
    end

    def test_over_a_generator_and_infinity
      evens = Enumerator.new do |y|
        a, b = 0, 1
        loop do
          y << a
          a, b = b, a + b
        end
      end
      assert_equal [0, 2, 8], evens.lazy.select(&:even?).first(3)
      assert_equal [1, 2, 3], (1..Float::INFINITY).first(3)
      assert_equal [2, 4], (1..Float::INFINITY).lazy.map { |x| x * 2 }.first(2)
    end

    def test_with_index_block_and_each
      seen = []
      out = (1..3).lazy.with_index { |x, i| seen << x * 10 + i }.to_a
      assert_equal [1, 2, 3], out
      assert_equal [10, 21, 32], seen
      got = []
      (1..3).lazy.map { |x| x + 1 }.each { |x| got << x }
      assert_equal [2, 3, 4], got
    end

    def test_laziness
      pulled = []
      firsts = (1..).lazy.map { |x| pulled << x; x }.select(&:odd?).first(2)
      assert_equal [1, 3], firsts
      assert_equal [1, 2, 3], pulled
    end

    def test_inspect
      assert_equal "#<Enumerator::Lazy: 1..3>", (1..3).lazy.inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..3>:map>", (1..3).lazy.map { |x| x }.inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..>:take(2)>", (1..).lazy.take(2).inspect
      assert_equal "#<Enumerator::Lazy: #<Enumerator::Lazy: 1..Infinity>:select>", (1..Float::INFINITY).lazy.select(&:even?).inspect
    end
  end

  class Ticker
    include Enumerable #[Integer]

    attr_reader :runs #: Integer

    #: (Array[Integer]) -> void
    def initialize(xs)
      @xs = xs
      @runs = 0
    end

    #: () { (Integer) -> void } -> void
    def each
      @runs += 1
      @xs.each { |x| yield x }
    end
  end

  # Equal under <=>, distinct objects: shows which one min/max/minmax keep.
  class Tagged
    include Comparable

    attr_reader :k #: Integer
    attr_reader :tag #: String

    #: (Integer, String) -> void
    def initialize(k, tag)
      @k = k
      @tag = tag
    end

    #: (Tagged) -> Integer
    def <=>(other) = k <=> other.k
  end

  class StreamingTest < Minitest::Test
    def test_each_cons_block_streams
      n = 0
      (1..).each_cons(2) do |_|
        n += 1
        break
      end
      assert_equal 1, n
      seen = []
      (1..).each_cons(3) do |w|
        seen << w
        break if seen.size == 2
      end
      assert_equal [[1, 2, 3], [2, 3, 4]], seen
      gen = Enumerator.new do |y|
        i = 0
        loop { y << (i += 1) }
      end
      hit = nil #: Array[Integer]?
      gen.each_cons(2) do |w|
        hit = w
        break if w[0] == 2
      end
      assert_equal [2, 3], hit
      assert_equal [[1, 2], [2, 3]], gen.each_cons(2).first(2)
    end

    def test_each_slice_block_streams
      got = nil #: Array[Integer]?
      (1..).each_slice(2) do |w|
        got = w
        break
      end
      assert_equal [1, 2], got
    end

    def test_each_cons_blockless_and_edges
      t = Ticker.new([1, 2, 3])
      got = []
      t.each_cons(2) { |w| got << w }
      assert_equal [[1, 2], [2, 3]], got
      got = []
      t.each_cons(5) { |w| got << w }
      assert_equal [], got
      assert_nil t.each_cons(2).size
      assert_equal 2, [1, 2, 3].each_cons(2).size
    end

    def test_each_cons_windows_are_independent
      keep = []
      Ticker.new([1, 2, 3, 4]).each_cons(2) { |w| keep << w }
      assert_equal [[1, 2], [2, 3], [3, 4]], keep
    end

    def test_minmax_one_pass
      t = Ticker.new([3, 1, 4, 1, 5])
      assert_equal [1, 5], t.minmax
      assert_equal 1, t.runs
      assert_equal [nil, nil], Ticker.new([]).minmax
      assert_equal [7, 7], Ticker.new([7]).minmax
      assert_equal [1, 2], Ticker.new([2, 1]).minmax
      assert_equal [1, 3], Ticker.new([1, 2, 3]).minmax
    end

    def test_eager_grep_with_block
      t = Ticker.new([1, 2, 3, 4])
      assert_equal [20, 30], t.grep(2..3) { |x| x * 10 }
      assert_equal [10, 40], t.grep_v(2..3) { |x| x * 10 }
      assert_equal [20, 30], [1, 2, 3, 4].grep(2..3) { |x| x * 10 }
      assert_equal %w[AB], %w[ab cd].grep(/a/) { |s| s.upcase }
    end

    def test_minmax_ties
      a = Tagged.new(1, "a")
      b = Tagged.new(1, "b")
      c = Tagged.new(2, "c")
      d = Tagged.new(2, "d")
      e = Tagged.new(0, "e")
      f = Tagged.new(0, "f")
      mm = ->(xs) { TaggedList.new(xs).minmax.map(&:tag) }
      assert_equal %w[a a], mm.call([a, b])
      assert_equal %w[a c], mm.call([a, b, c, d])
      assert_equal %w[e c], mm.call([a, c, d, e, f])
      assert_equal %w[a c], mm.call([c, a, d, b])
      assert_equal %w[a a], mm.call([a, b, a])
    end

    def test_minmax_incomparable
      r = begin
        Mixed.new.minmax
        nil
      rescue ArgumentError
        :argument_error
      end
      assert_equal :argument_error, r
    end
  end

  class TaggedList
    include Enumerable #[Tagged]

    #: (Array[Tagged]) -> void
    def initialize(xs)
      @xs = xs
    end

    #: () { (Tagged) -> void } -> void
    def each
      @xs.each { |x| yield x }
    end
  end

  class Mixed
    include Enumerable #[untyped]

    #: () { (untyped) -> void } -> void
    def each
      yield 1
      yield "a"
    end
  end

  class LazyMoreTest < Minitest::Test
    def test_grep
      inf = (1..Float::INFINITY).lazy
      assert_equal [2, 3, 4], inf.grep(2..5).first(3)
      assert_equal [1, 6, 7], inf.grep_v(2..5).first(3)
      assert_equal [4, 6, 8], inf.grep(2..5) { |x| x * 2 }.first(3)
      assert_equal [1, 6], inf.grep_v(2..5) { |x| x }.first(2)
      assert_equal %w[ab ac], %w[ab xx ac].lazy.grep(/a/).to_a
      assert_equal [1, 3], [1, "a", 3, :b].lazy.grep(Integer).to_a
      assert_equal ["a", :b], [1, "a", 3, :b].lazy.grep_v(Integer).to_a
    end

    def test_grep_is_lazy_and_returns_lazy
      assert_kind_of Enumerator::Lazy, (1..3).lazy.grep(1)
      assert_kind_of Enumerator::Lazy, (1..3).lazy.grep_v(1)
      assert_kind_of Enumerator::Lazy, (1..3).lazy.find_all { |x| x > 1 }
      assert_kind_of Enumerator::Lazy, (1..3).lazy.collect_concat { |x| [x] }
      assert_kind_of Enumerator::Lazy, (1..3).lazy.collect { |x| x }
      pulled = []
      first = (1..).lazy.map { |x| pulled << x; x }.grep(Integer).first(2)
      assert_equal [1, 2], first
      assert_equal [1, 2], pulled
    end

    def test_find_all_and_collect_concat
      inf = (1..Float::INFINITY).lazy
      assert_equal [2, 4], inf.find_all(&:even?).first(2)
      assert_equal [1, -1, 2], inf.collect_concat { |x| [x, -x] }.first(3)
      assert_equal [1, -1, 2], inf.flat_map { |x| [x, -x] }.first(3)
    end

    def test_inspect_labels
      inf = (1..Float::INFINITY).lazy
      src = "#<Enumerator::Lazy: 1..Infinity>"
      assert_equal "#<Enumerator::Lazy: #{src}:grep(2..5)>", inf.grep(2..5).inspect
      assert_equal "#<Enumerator::Lazy: #{src}:grep_v(2..5)>", inf.grep_v(2..5).inspect
      assert_equal "#<Enumerator::Lazy: #{src}:find_all>", inf.find_all { |x| x > 1 }.inspect
      assert_equal "#<Enumerator::Lazy: #{src}:collect>", inf.collect { |x| x }.inspect
      assert_equal "#<Enumerator::Lazy: #{src}:collect_concat>", inf.collect_concat { |x| [x] }.inspect
      assert_equal "#<Enumerator::Lazy: #{src}:flat_map>", inf.flat_map { |x| [x] }.inspect
      assert_equal "#<Enumerator::Lazy: #{src}:map>", inf.map { |x| x }.inspect
    end

    def test_chunk_while_and_slice_when
      inf = (1..Float::INFINITY).lazy
      assert_equal [[1, 2], [3, 4, 5]], inf.slice_when { |a, b| b % 3 == 0 }.first(2)
      assert_equal [[1, 2, 3], [4, 5, 6]], inf.chunk_while { |a, b| b % 3 != 1 }.first(2)
      assert inf.chunk_while { |a, b| b == a + 1 }.is_a?(Enumerator::Lazy)
      assert_equal [[1, 2, 3], [5]], [1, 2, 3, 5].lazy.chunk_while { |a, b| b == a + 1 }.to_a
      assert_equal [[1, 2, 3], [5]], [1, 2, 3, 5].lazy.slice_when { |a, b| b != a + 1 }.to_a
    end
  end

  class ArithmeticSequenceTest < Minitest::Test
    def test_inspect_and_class
      assert_equal "((1..10).%(3))", ((1..10) % 3).inspect
      assert_equal "(1.step(10, 3))", 1.step(10, 3).inspect
      assert_equal "((1...10).step(3))", (1...10).step(3).inspect
      assert_equal "(1.step(10))", 1.step(10).inspect
      assert_equal "(1.0.step(2.0, 0.5))", 1.0.step(2.0, 0.5).inspect
      assert_equal "((1..).step(2))", (1..).step(2).inspect
      assert_equal "#<Enumerator: \"a\"..\"e\":step(2)>", ("a".."e").step(2).inspect
      assert_equal Enumerator::ArithmeticSequence, ((1..10) % 3).class
    end

    def test_values
      s = 1.step(10, 3)
      assert_equal [1, 10, 3], [s.begin, s.end, s.step]
      refute s.exclude_end?
      assert_equal 1, s.first
      assert_equal [1, 4], s.first(2)
      assert_equal 10, s.last
      assert_equal [7, 10], s.last(2)
      assert_equal 4, s.size
      assert_equal [1, 4, 7, 10], s.to_a
      assert_equal 1.step(10, 3), s
      assert_equal 7, (1...10).step(3).last
      assert_equal [10, 7, 4, 1], 10.step(1, -3).to_a
      assert_equal [2, 8, 14, 20], (1..10).step(3).map { |x| x * 2 }
      assert_equal [1, 3, 5], (1..).step(2).first(3)
      assert_equal [1.0, 1.5, 2.0], 1.0.step(2.0, 0.5).to_a
      assert_equal [10, 40], ((1..10) % 3).lazy.map { |x| x * 10 }.first(2)
      assert_equal [[1, 4], [7, 10]], 1.step(10, 3).each_slice(2).to_a
      assert_equal 22, (1..10).step(3).sum
      assert ((1..10) % 3).include?(4)
      assert_equal %w[a c e], ("a".."e").step(2).to_a
      assert_equal [], (5..1).step(2).to_a
      assert_nil (5..1).step(2).last
      e = (1..3).step(1)
      assert_equal [1, 2], [e.next, e.next]
    end

    def test_errors
      assert_raises(ArgumentError) { (1..10).step(0) }
      err = assert_raises(RangeError) { (1..).step(3).last }
      assert_equal "cannot get the last element of endless arithmetic sequence", err.message
    end
  end

  # #56: min(n)/max(n) on an endless range end, as MRI's do
  class EnumeratorEndlessMinMaxTest < Minitest::Test
    def test_endless_min_max_n
      assert_equal [[1, 2], [], [1, 2], [5, 4]], [(1..).min(2), (1..).min(0), (1..5).min(2), (1..5).max(2)]
      e = assert_raises(RangeError) { (1..).max(2) }
      assert_equal "cannot get the maximum of endless range", e.message
      e = assert_raises(RangeError) { (..1).min(2) }
      assert_equal "cannot get the minimum of beginless range", e.message
      e = assert_raises(ArgumentError) { (1..).min(-1) }
      assert_equal "negative array size (or size too big)", e.message
    end

    # #75: a beginless Integer range counts down from its end
    def test_beginless_max_n
      assert_equal [[5, 4], [4, 3], [], [5, 4, 3]], [(..5).max(2), (...5).max(2), (..5).max(0), (..5).reverse_each.first(3)]
      e = assert_raises(ArgumentError) { (..5).max(-1) }
      assert_equal "negative array size (or size too big)", e.message
      e = assert_raises(TypeError) { (.."c").max(2) }
      assert_equal "can't iterate from NilClass", e.message
    end
  end

  # decision 149: an each that never yields has nil elements, unobservably
  class EnumEmpty
    include Enumerable

    def each
      self
    end
  end

  class EnumeratorNeverYieldsTest < Minitest::Test
    def test_never_yields
      e = EnumEmpty.new
      assert_equal [[], [], 0, nil, false, []], [e.to_a, e.map { |x| x }, e.count, e.first, e.include?(1), e.sort]
    end
  end

  # decision 301: `def each(&block) = @list.each(&block)` under Enumerable (bindata's SanitizedFields)
  class Forwarded
    include Enumerable

    #: (Forwarded?) -> void
    def initialize(base = nil)
      @fields = base ? base.raw_fields : []
    end

    #: (String) -> void
    def add(name) = @fields << name

    #: () -> Array[String]
    def raw_fields = @fields.dup

    def each(&block)
      @fields.each(&block)
    end
  end

  class ForwardedTest < Minitest::Test
    def test_each_forwarding_its_block
      f = Forwarded.new
      f.add("a")
      f.add("b")
      assert_equal %w[a b], Forwarded.new(f).map { |s| s }
      assert_equal %w[b], f.select { |s| s > "a" }
    end
  end
end

module EnumeratorTests
  class EnumUntypedIncluder
    include Enumerable

    def initialize(xs)
      @xs = xs
    end

    def each
      @xs.each { |e| yield e }
    end

    def slices = each_slice(2).to_a
    def backwards = reverse_each.to_a
  end

  class EnumBlocklessTwinTest < Minitest::Test
    def test_blockless_twins_in_an_includer
      it = EnumUntypedIncluder.new([1, 2, 3])
      assert_equal [[1, 2], [3]], it.slices
      assert_equal [3, 2, 1], it.backwards
      out = [] #: Array[untyped]
      it.reverse_each { |x| out << x }
      assert_equal [3, 2, 1], out
    end
  end
end

module EnumeratorTests
  # each_slice, each_cons and zip carve their results from shared slabs (decision 348): each window is still its own
  # Array, so keeping, mutating or growing one leaves the others and the receiver as MRI leaves them.
  class EnumSlabTest < Minitest::Test
    def test_windows_are_independent
      src = (1..7).to_a
      cons = src.each_cons(3).to_a
      cons[0][0] = 99
      cons[1] << 42
      assert_equal [[99, 2, 3], [2, 3, 4, 42], [3, 4, 5], [4, 5, 6], [5, 6, 7]], cons
      assert_equal (1..7).to_a, src
      slices = src.each_slice(3).to_a
      slices[0] << 10
      slices[1][2] = 0
      assert_equal [[1, 2, 3, 10], [4, 5, 0], [7]], slices
      kept = [] #: Array[Array[Integer]]
      src.each_cons(2) { |w| kept << w }
      assert_equal 6, kept.map(&:object_id).uniq.size
      assert_equal [6, 7], kept.last
    end

    def test_large_and_small_sizes
      assert_equal [[1, 2]], [1, 2].each_slice(10**9).to_a
      assert_equal [], [1, 2].each_cons(5).to_a
      assert_equal 4_998, (1..5_000).each_cons(3).count { |w| w.sum.positive? }
      assert_equal 1_667, (1..5_000).each_slice(3).count
    end

    def test_zip_pairs
      a = [1, 2, 3]
      pairs = a.zip([10, 20])
      assert_equal [[1, 10], [2, 20], [3, nil]], pairs
      assert_equal 50, (1..2_000).to_a.zip((1..1_500).to_a).sum { |x, y| y ? 0 : 1 } - 450
    end
  end

  # A block that cannot keep its window gets one Array refilled per window (decision 386); one that can gets copies.
  class EnumWindowReuseTest < Minitest::Test
    def test_reading_blocks
      src = (1..10).to_a
      assert_equal 1, src.each_cons(3).count { |w| w.sum % 5 == 0 }
      assert_equal 15, src.each_slice(4).sum { |s| s.first.to_i }
      assert_equal [3, 5, 7, 9, 11, 13, 15, 17, 19], src.each_cons(2).map { |w| w.sum }
      assert_equal [10, 26, 19], src.each_slice(4).map { |s| s.sum }
      assert(src.each_cons(4).all? { |w| w.last.to_i - w.first.to_i == 3 })
      total = 0
      src.each_cons(3) { |w| total += w.max.to_i * w.size }
      assert_equal 156, total
      n = 0
      src.each_slice(3) { n += 1 }
      assert_equal 4, n
    end

    def test_destructured_params_are_copies
      kept = [] #: Array[Integer?]
      pairs = {} #: Hash[Integer, Integer?]
      (1..6).to_a.each_cons(2) do |a, b|
        kept << b
        pairs[a.to_i] = b
      end
      assert_equal [2, 3, 4, 5, 6], kept
      assert_equal({ 1 => 2, 2 => 3, 3 => 4, 4 => 5, 5 => 6 }, pairs)
      tails = [] #: Array[Integer?]
      (1..7).to_a.each_slice(3) { |a, b, c| tails << c }
      assert_equal [3, 6, nil], tails
      sums = (1..5).to_a.each_cons(3).map { |a, b, c| a.to_i + b.to_i + c.to_i }
      assert_equal [6, 9, 12], sums
    end

    def test_escaping_blocks_get_separate_windows
      src = (1..5).to_a
      res = [] #: Array[Array[Integer]]
      src.each_cons(2) { |w| res << w }
      assert_equal [[1, 2], [2, 3], [3, 4], [4, 5]], res
      assert_equal 4, res.map(&:object_id).uniq.size
      copies = [] #: Array[Array[Integer]]
      src.each_slice(2) { |s| copies << s.dup }
      assert_equal [[1, 2], [3, 4], [5]], copies
      firsts = [] #: Array[Integer?]
      src.each_cons(2) { |w| firsts << w.first }
      assert_equal [1, 2, 3, 4], firsts
      kept = src.each_slice(2).map { |s| s }
      assert_equal [[1, 2], [3, 4], [5]], kept
      heads = src.each_cons(3).map { |w| w.first }
      assert_equal [1, 2, 3], heads
      procs = [] #: Array[^() -> Integer]
      src.each_cons(2) { |w| procs << -> { w.sum } }
      assert_equal [3, 5, 7, 9], procs.map(&:call)
      grown = [] #: Array[Array[Integer]]
      src.each_cons(2) do |w|
        w << 0
        grown << w
      end
      assert_equal [[1, 2, 0], [2, 3, 0], [3, 4, 0], [4, 5, 0]], grown
    end

    def test_break_and_nesting
      seen = [] #: Array[Integer]
      (1..10).to_a.each_cons(2) do |a, b|
        break if a.to_i > 3

        seen << b.to_i
      end
      assert_equal [2, 3, 4], seen
      inner = 0
      (1..4).to_a.each_cons(2) do |w|
        w.each_slice(1) { |x| inner += x.sum }
      end
      assert_equal 0, inner - 15
    end
  end
end

module EnumeratorTests
  # zip with a block, or chained into one, builds no Array of pairs (decision 389): MRI's values all the same.
  class EnumZipFuseTest < Minitest::Test
    def test_block_form_reads_both_arrays
      a = [1, 2, 3]
      b = [4, 5]
      s = 0
      a.zip(b) { |x, y| s += x * (y || 0) }
      assert_equal 14, s
      seen = [] #: Array[Integer?]
      a.zip(b) do |x, y|
        b[1] = 7 if x == 1
        seen << y
      end
      assert_equal [4, 5, nil], seen
      grown = [] #: Array[Integer]
      c = [1, 2]
      c.zip([9]) do |x, _y|
        c << 3 if x == 1
        grown << x
      end
      assert_equal [1, 2, 3], grown
      shrunk = [] #: Array[Integer?]
      d = [1, 2, 3]
      d.zip([9, 8, 7]) do |x, y|
        d.pop if x == 1
        shrunk << y
      end
      assert_equal [9, 8], shrunk
    end

    def test_kept_pairs_and_values
      a = [1, 2, 3]
      b = [4, 5]
      out = [] #: Array[[Integer, Integer?]]
      a.zip(b) { |pair| out << pair }
      a.zip(b).each { |pair| out << pair }
      ys = [] #: Array[Integer?]
      a.zip(b) { |_x, y| ys << y }
      a.zip(b).each { |_x, y| ys << y }
      b[0] = 99
      assert_equal [[1, 4], [2, 5], [3, nil], [1, 4], [2, 5], [3, nil]], out
      assert_equal [4, 5, nil, 4, 5, nil], ys
      out[0] = [7, 7]
      assert_equal [2, 5], out[1]
    end

    def test_chained_consumers
      a = [1, 2, 3, 4]
      b = [10, 20, 30]
      assert_equal 140, a.zip(b).sum { |x, y| x * (y || 0) }
      assert_equal [11, 22, 33, 4], a.zip(b).map { |x, y| x + (y || 0) }
      assert_equal 1, a.zip(b).count { |_x, y| y.nil? }
      assert_equal [[3, 30], [4, nil]], a.zip(b).select { |x, _y| x > 2 }
      assert_equal [[1, 10]], a.zip(b).reject { |x, _y| x > 1 }
      assert(a.zip(b).any? { |x, y| y.nil? && x == 4 })
      assert(a.zip(b).all? { |x, _y| x.positive? })
      assert(a.zip(b).none? { |x, y| x == y })
      assert_equal [10, 40, 90], a.zip(b).filter_map { |x, y| y && x * y }
      assert_equal [1, 10, 2, 20, 3, 30, 4], a.zip(b).flat_map { |x, y| y ? [x, y] : [x] }
      words = %w[a b]
      assert_equal %w[a1 b2], words.zip([1, 2, 3]).map { |w, n| "#{w}#{n}" }
    end

    def test_chain_reads_arrays_before_the_block
      a = [1, 2, 3]
      b = [4, 5, 6]
      got = a.zip(b).map do |x, y|
        b[2] = 0
        a << 9 if a.size < 5
        x + (y || 0)
      end
      assert_equal [5, 7, 9], got
      assert_equal [4, 5, 0], b
      c = [1, 2, 3]
      d = [4, 5, 6]
      each_seen = [] #: Array[Integer]
      c.zip(d).each do |x, y|
        c[2] = 0
        d[2] = 0
        c << 9 if c.size < 5
        each_seen << x << (y || -1)
      end
      assert_equal [1, 4, 2, 5, 3, 6], each_seen
    end

    def test_break_and_next
      a = [1, 2, 3, 4]
      b = [5, 6, 7, 8]
      t = 0
      a.zip(b) do |x, y|
        next if x == 2
        break if x == 4

        t += x + y
      end
      assert_equal 16, t
      u = 0
      a.zip(b).each do |x, y|
        break if x == 3

        u += y - x
      end
      assert_equal 8, u
    end

    def test_three_arrays
      a = [1, 2, 3]
      b = [4, 5]
      c = [7, 8, 9]
      assert_equal [[1, 4, 7], [2, 5, 8], [3, nil, 9]], a.zip(b, c)
      assert_equal [12, 15, 12], a.zip(b, c).map { |x, y, z| x + (y || 0) + (z || 0) }
    end
  end
end

# Matrix-shaped classes (decision 412): an Enumerable override whose block takes another arity keeps its own block,
# `each(which = :all)` answers Enumerable's each(), `or` right operands hoist their locals, Self in a tuple, Numeric#+@.
module EnumeratorTests
  module Pairing
    def pair_with(n) = [self, n]
  end

  class Grid
    include Enumerable #[Integer]
    include Pairing

    #: (Array[Array[Integer]]) -> void
    def initialize(rows)
      @rows = rows
    end

    #: (?Symbol) { (Integer) -> void } -> self
    def each(which = :all)
      @rows.each_with_index do |row, i|
        row.each_with_index { |e, j| yield e if which == :all || (which == :diagonal && i == j) }
      end
      self
    end

    #: () { (Integer, Integer, Integer) -> void } -> void
    def each_with_index
      @rows.each_with_index do |row, i|
        row.each_with_index { |e, j| yield e, i, j }
      end
    end

    #: (Integer) -> Array[Integer]?
    def row_at(i) = @rows[i]

    #: (Integer) -> String
    def pick(i)
      rows = row_at(i) or found = "none"
      rows ? rows.sum.to_s : found.to_s
    end
  end

  class GridTest < Minitest::Test
    def test_each_default_and_enumerable
      g = Grid.new([[1, 2], [3, 4]])
      assert_equal([1, 2, 3, 4], g.to_a)
      assert_equal(10, g.sum)
      out = [] #: Array[Integer]
      g.each(:diagonal) { |e| out << e }
      assert_equal([1, 4], out)
    end

    def test_three_value_block
      out = [] #: Array[String]
      Grid.new([[5, 6], [7, 8]]).each_with_index { |e, i, j| out << "#{e}@#{i},#{j}" }
      assert_equal(["5@0,0", "6@0,1", "7@1,0", "8@1,1"], out)
    end

    def test_or_assignment_hoists
      g = Grid.new([[1, 2]])
      assert_equal("3", g.pick(0))
      assert_equal("none", g.pick(5))
    end

    def test_self_in_tuple
      g = Grid.new([[1]])
      pr = g.pair_with(2)
      assert(pr[0].equal?(g))
      assert_equal(2, pr[1])
    end

    def test_unary_plus
      assert_equal(3, +3)
      x = -2.5
      assert_in_delta(-2.5, +x)
      assert_equal(Rational(1, 2), +Rational(1, 2))
      assert_equal(Complex(1, 2), +Complex(1, 2))
    end
  end
end

# Matrix's other shapes (decision 414): `x.to_enum(:each, arg)`, a yield whose value Array.new's block takes, transpose of
# untyped rows, initialize_copy's super.
module EnumeratorTests
  class Sheet
    include Enumerable #[Integer]

    #: (Array[Array[Integer]]) -> void
    def initialize(rows)
      @rows = rows
    end

    #: (?Symbol) ?{ (Integer) -> void } -> untyped
    def each(which = :all)
      return to_enum(:each, which) unless block_given?

      @rows.each_with_index do |row, i|
        row.each_with_index { |e, j| yield e if which == :all || i == j }
      end
      self
    end

    #: (Integer) { (Integer, Integer) -> Integer } -> Array[Array[Integer]]
    def self.build(n)
      Array.new(n) { |i| Array.new(n) { |j| yield i, j } }
    end

    #: () -> Array[Array[Integer]]
    def rows = @rows

    private

    #: (Sheet) -> void
    def initialize_copy(other)
      super
      @rows = @rows.map(&:dup)
    end
  end

  class SheetTest < Minitest::Test
    def test_to_enum_on_receiver
      s = Sheet.new([[1, 2], [3, 4]])
      assert_equal([1, 4], s.to_enum(:each, :diagonal).to_a)
      assert_equal([1, 2, 3, 4], s.enum_for(:each).to_a)
    end

    def test_yield_value_in_array_new
      assert_equal([[0, 1], [10, 11]], Sheet.build(2) { |i, j| (i * 10) + j })
    end

    def test_transpose_untyped
      rows = [[1, "a"], [2, "b"]] #: Array[untyped]
      assert_equal([[1, 2], ["a", "b"]], rows.transpose)
    end

    def test_initialize_copy_super
      s = Sheet.new([[1, 2]])
      d = s.dup
      d.rows[0][0] = 9
      assert_equal(1, s.rows[0][0])
      assert_equal(9, d.rows[0][0])
    end
  end
end

# More of matrix's (decision 414): destructuring through to_ary, alias Methods equal, Numeric's long names.
module EnumeratorTests
  class Trio
    #: () -> [Integer, String, Symbol]
    def to_ary = [1, "b", :c]

    #: () -> Integer
    def det = 1
    alias determinant det

    #: (?Symbol) ?{ (Integer) -> Integer } -> Array[Integer]
    def collect(which = :all, &block)
      return [which.size] unless block

      [1, 2].map(&block)
    end
    alias map collect
  end

  class TrioTest < Minitest::Test
    def test_destructure_to_ary
      a, b, c = Trio.new
      assert_equal([1, "b", :c], [a, b, c])
      x, *ys = Trio.new
      assert_equal(1, x)
      assert_equal(["b", :c], ys)
    end

    def test_alias_methods_equal
      t = Trio.new
      assert(t.method(:det) == t.method(:determinant))
      assert(Trio.instance_method(:det) == Trio.instance_method(:determinant))
      refute(t.method(:det) == Trio.new.method(:det))
      assert(Trio.instance_method(:collect) == Trio.instance_method(:map))
    end

    def test_numeric_long_names
      assert_equal([3, 0, 3, [3, 0]], [3.real, 3.imaginary, 3.conjugate, 3.rectangular])
      assert_equal([2.5, 0, 2.5, [2.5, 0]], [2.5.real, 2.5.imaginary, 2.5.conjugate, 2.5.rectangular])
      r = Rational(1, 2)
      assert_equal([r, 0, r, [r, 0]], [r.real, r.imaginary, r.conjugate, r.rectangular])
    end
  end

  # An Enumerable includer whose each forwards its rest and block to a container (regexp_parser's Subexpression):
  # Enumerable's slot passes no arguments, so the adapter drops the rest.
  class RestForwarded
    include Enumerable #[Integer]

    #: (Array[Integer]) -> void
    def initialize(items)
      @items = items
    end

    #: (*untyped) { (Integer) -> void } -> void
    def each(*args, &block)
      @items.each(*args, &block)
    end

    #: () -> Hash[Symbol, Integer]
    def to_h = { size: @items.size }
  end

  class RestForwardedSub < RestForwarded
    #: () -> Array[Integer]
    def first_two = take(2)
  end

  class EachRestForwardTest < Minitest::Test
    def test_enumerable_through_rest_each
      s = RestForwardedSub.new([3, 1, 2])
      assert_equal([6, 2, 4], s.map { |x| x * 2 })
      assert_equal([1, 2, 3], s.sort)
      assert_equal({ size: 3 }, s.to_h)
      assert_equal([3, 1], s.first_two)
      assert_equal([[3, 0], [1, 1], [2, 2]], s.each_with_index.to_a)
      assert_equal([3, 1], s.select(&:odd?))
    end
  end
end
