# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/array_access.rb.
#: (Array[Integer]) -> void
def grow_by_size(list)
  list << list.size
end

#: (Integer) -> bool
def prime?(n) = ArrayTests::PRIMES.include?(n)

#: (?Array[Integer]) -> Integer
def count_all(xs = []) = xs.size

#: (Array[Integer]) -> Array[Integer]
def evens(nums)
  return [] if nums.empty?

  nums.select(&:even?)
end

#: (bool) -> Array[String]
def array_pick(b) = b ? ["x"] : []

# Helpers for the checks that were testdata/run/array_blocks.rb.
#: (Array[Integer], Integer) -> Integer?
def index_of(arr, target)
  arr.each_with_index { |x, i| return i if x == target }
  nil
end

#: (Array[String]) -> String
def first_long(words)
  words.each do |w|
    next if w.size < 4

    return w
  end
  "none"
end

#: (Integer) { (Integer, String) -> void } -> void
def pairs_upto(n)
  i = 0
  while i < n
    yield i, i.to_s * 2
    i += 1
  end
end

#: (Array[Integer]) { (Integer) -> void } -> void
def array_each_even(nums)
  nums.each { |n| yield n if n.even? }
end

#: (Array[Integer], Array[String]) { (Integer) -> void } -> void
def safe_each(nums, log)
  nums.each do |n|
    begin
      yield n
    rescue ZeroDivisionError => e
      log << "rescued #{e.message}"
    end
  end
end

#: (Array[Integer]) { (Integer) -> Integer } -> Integer
def sum_by(nums)
  total = 0
  nums.each { |n| total += yield(n) }
  total
end

#: (Array[Array[Integer]], Integer) -> Integer
def find_cell(grid, target)
  grid.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return r * 10 + c if v == target
    end
  end
  -1
end

#: (Array[String]) { (String) -> bool } -> Integer
def count_where(xs, &blk) = xs.select(&blk).size

#: (Array[Integer], Array[String]) -> void
def show_all(nums, log) = nums.each { |n| log << "all #{n}" }

#: (Array[Integer], Array[String]) -> void
def show_rev(nums, log)
  nums.reverse_each { |n| log << "rev #{n}" }
end

# Helpers for the checks that were testdata/run/array_bug_tuple_as_untyped.rb.
#: (untyped) -> bool
def arr?(x) = x.is_a?(Array)

#: (untyped) -> String
def arr_kind(x)
  case x
  when Array then "array #{x.size}"
  else "other"
  end
end

#: (Array[untyped]) -> Array[untyped]
def flat(xs)
  out = [] #: Array[untyped]
  xs.each do |x|
    if x.is_a?(Array)
      out.concat(flat(x))
    else
      out << x
    end
  end
  out
end

# Helpers for the checks that were testdata/run/array_bugs.rb.
#: (*String) -> Array[String]
def collect_rest(*parts) = parts

#: (*Integer) -> Integer
def total_elem(*ns) = ns.reduce(0) { |acc, n| acc + n }

#: (*Integer) -> Integer
def total_mixed(*ns) = ns.reduce(0) { |acc, n| acc + n }

#: (Array[Integer]) -> [Integer?, Integer?]
def min_max(nums) = [nums.min, nums.max]

#: (Array[untyped]) -> String
def show_list(xs) = xs.map { |x| x.inspect }.join(" ")

#: [T] (Array[T]) -> T?
def second_of(arr) = arr[1]

#: (Array[Array[Integer]], Integer) -> [Integer, Integer]?
def find_pos(grid, target)
  grid.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return [r, c] if v == target
    end
  end
  nil
end

#: (Array[Integer]?) -> Integer
def count_of(list) = list ? list.size : -1

#: (untyped) -> untyped
def pass_through(v) = v

# Helpers for the checks that were testdata/run/array_objects.rb.
class Array
  #: () -> E?
  def second = self[1]

  #: () -> Integer
  def twice_size = size * 2
end

# Helpers for the checks that were testdata/run/array_splat.rb.
#: (*Integer) -> Integer
def array_total(*ns) = ns.reduce(0) { |acc, n| acc + n }
def array_first_set(xs) = xs.reduce(nil) { |acc, x| acc || x }

#: (String, *Integer) -> String
def array_label(name, *ns) = "#{name}:#{ns.inspect}:#{ns.size}:#{ns.empty?}"

#: (*String) -> Array[String]
def collect(*parts) = parts

#: (*untyped) -> String
def kinds(*xs) = xs.map { |x| x.inspect }.join(",")

#: (?String, *Integer) -> String
def opt_then_rest(prefix = "p", *ns) = prefix + ns.map(&:to_s).join

#: (*[Integer, String]) -> String
def pairs(*ps) = ps.map { |n, s| s * n }.join("/")

#: (*Integer) -> Integer
def wrap(*xs) = array_total(*xs)

#: (*Integer) -> Array[Integer]
def array_grow(*xs)
  xs << 99
  xs
end

#: (Integer, *Integer) -> Integer
def first_plus(x, *rest) = x + rest.size

# Helpers for the checks that were testdata/run/array_tuples.rb.
#: (Integer, Integer) -> [Integer, Integer]
def array_divmod2(a, b) = [a / b, a % b]

#: (String) -> [String, Integer, bool]
def describe_str(s) = [s.upcase, s.size, s.empty?]

#: (Array[Integer]) -> [Integer?, Integer?]
def bounds(nums) = [nums.min, nums.max]

#: ([Integer, String]) -> String
def render(pair) = "#{pair[1]}=#{pair[0]}"

#: (untyped) -> String
def show_any(x) = x.inspect

#: () -> [Array[Integer], Integer]
def split_list = [[1, 2], 3]

# Helpers for the checks that were testdata/run/array_untyped.rb.
#: (untyped) -> String
def array_describe(x)
  if x.is_a?(Array)
    "array of #{x.size}: " + x.map { |e| array_describe(e) }.join(" ")
  elsif x.is_a?(Integer)
    "int(#{x})"
  elsif x.nil?
    "nil"
  else
    x.inspect
  end
end

#: (untyped) -> untyped
def array_ident(v) = v

#: (untyped) -> void
def push_two(x)
  x << 2
end

#: (untyped) -> String
def array_kind(x)
  case x
  when Array then "array #{x.size}"
  when Integer then "int"
  when nil then "nil"
  else "other"
  end
end

module Kernel
  #: () -> String
  def array_tag = "<#{self.class}>"
end

module ArrayTests
  PRIMES = [2, 3, 5, 7] #: Array[Integer]

  class Bag
    #: () -> void
    def initialize
      @items = [] #: Array[String]
    end

    #: (String) -> self
    def add(s)
      @items << s
      self
    end

    #: () { (String) -> void } -> void
    def each(&block) = @items.each(&block)

    #: () { (String) -> void } -> void
    def each_twice(&block)
      @items.each(&block)
      @items.reverse_each(&block)
    end

    #: () { (String) -> bool } -> Array[String]
    def keep(&block) = @items.select(&block)

    #: () { (String) -> Integer } -> Integer
    def score(&block)
      acc = 0
      @items.each { |s| acc += block.call(s) }
      acc
    end
  end

  class Util
    #: [T] (Array[T]) -> T?
    def self.second(arr) = arr[1]

    #: [T, U] (Array[T]) { (T) -> U } -> Array[U]
    def self.my_map(arr)
      out = [] #: Array[U]
      arr.each { |x| out << yield(x) }
      out
    end

    #: [T] (Array[T], T) -> Array[T]
    def self.without(arr, drop) = arr.reject { |x| x == drop }
  end

  # uniq matches arrays, Structs and Data by value
  Pair = Struct.new(:a, :b) #: [Integer, Integer]

  Val = Data.define(:v) #: [Integer]

  # Helpers for the checks that were testdata/run/array_custom_enumerable.rb.
  class Countdown
    include Enumerable #[Integer]

    #: (Integer) -> void
    def initialize(from)
      @from = from
    end

    #: () { (Integer) -> void } -> void
    def each
      n = @from
      while n > 0
        yield n
        n -= 1
      end
    end
  end

  class Shelf
    include Enumerable #[String]

    #: (*String) -> void
    def initialize(*titles)
      @titles = titles
    end

    #: () { (String) -> void } -> void
    def each(&block) = @titles.each(&block)
  end

  module Digits
    extend Enumerable #[Integer]

    #: () { (Integer) -> void } -> void
    def self.each(&block) = [3, 1, 2].each(&block)
  end

  class Version
    attr_reader :major #: Integer
    attr_reader :minor #: Integer

    #: (Integer, Integer) -> void
    def initialize(major, minor)
      @major = major
      @minor = minor
    end

    #: (Version) -> Integer
    def <=>(other) = major == other.major ? minor <=> other.minor : major <=> other.major

    #: () -> String
    def to_s = "v#{major}.#{minor}"

    #: () -> String
    def inspect = "#<V #{self}>"

    # a leaf class's C? from a filter_map block (Homebrew's `languages.filter_map { Locale.parse(it) }`)
    #: (String) -> Version?
    def self.parse(s)
      a, b = s.split(".")
      a && b ? new(a.to_i, b.to_i) : nil
    end
  end

  class Nums
    include Enumerable #[Integer]

    #: (*Integer) -> void
    def initialize(*ns)
      @ns = ns
    end

    #: () { (Integer) -> void } -> void
    def each(&block) = @ns.each(&block)

    #: () -> Integer
    def total = reduce(0) { |a, b| a + b }

    #: () -> Array[Integer]
    def evens = select(&:even?)
  end

  class MoreNums < Nums
    #: () -> Integer
    def biggest = max || 0

    #: () -> String
    def summary = "#{count}:#{sort.inspect}:#{include?(8)}:#{first(1).inspect}"
  end

  class Pairs
    include Enumerable #[[String, Integer]]

    #: () -> void
    def initialize
      @data = [["a", 1], ["b", 2]] #: Array[[String, Integer]]
    end

    #: () { ([String, Integer]) -> void } -> void
    def each(&block) = @data.each(&block)
  end

  class Naturals
    include Enumerable #[Integer]

    #: () { (Integer) -> void } -> void
    def each
      n = 0
      while true
        yield n
        n += 1
      end
    end
  end

  class Tolerant
    include Enumerable #[Integer]

    attr_reader :log #: Array[String]

    #: () -> void
    def initialize
      @log = []
    end

    # unannotated, and it rescues around yield: a closure, adapted for Enumerable
    def each
      n = 0
      while true
        n += 1
        begin
          yield n
        rescue ZeroDivisionError
          @log << "tolerant #{n}"
        ensure
          @log << "ensure #{n}" if n == 2
        end
        return if n == 3
      end
    end
  end

  # Helpers for the checks that were testdata/run/array_mid.rb.
  # a rescue inside a closure-taking each catches the block's own raise
  class Safe
    include Enumerable #[Integer]

    attr_reader :log #: Array[String]

    #: () -> void
    def initialize
      @log = []
    end

    #: () { (Integer) -> void } -> void
    def each
      [1, 2].each do |x|
        begin
          yield x
        rescue ZeroDivisionError
          @log << "rescued"
        end
      end
    end
  end

  # include? compares objects by identity and untyped elements by value
  class MidPt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end
  end

  class Shape
    #: () -> Float
    def area = 0.0

    #: () -> String
    def name = "shape"
  end

  class Circle < Shape
    #: (Float) -> void
    def initialize(r)
      @r = r
    end

    def area = 3.0 * @r * @r
    def name = "circle"
  end

  class Sq < Shape
    #: (Float) -> void
    def initialize(s)
      @s = s
    end

    def area = @s * @s
  end

  class Inbox
    attr_reader :items #: Array[String]

    #: () -> void
    def initialize
      @items = []
    end

    #: (String) -> self
    def add(s)
      @items << s
      self
    end
  end

  class Pt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Pt) && other.x == x

    #: () -> String
    def inspect = "P#{x}"
  end

  class Plain
    attr_accessor :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end
  end

  Box = Struct.new(:items) #: [Array[Integer]]

  class ArrayAccessTest < Minitest::Test
    def test_section_0
      nums = [10, 20, 30] #: Array[Integer]
      assert_equal 10, nums[0]
      assert_equal 30, nums[2]
      assert_nil nums[3]
      assert_nil nums[100]
      assert_equal 30, nums[-1]
      assert_equal 10, nums[-3]
      assert_nil nums[-4]
      assert_equal 30, nums.last
      assert_equal 3, nums.size
      assert_equal 3, nums.length
      assert_equal false, nums.empty?
      empty = [] #: Array[Integer]
      assert_nil empty[0]
      assert_nil empty[-1]
      assert_nil empty.last
      assert_equal 0, empty.size
      assert_equal true, empty.empty?
      nums[1] = 21
      nums[-1] = 31
      assert_equal [10, 21, 31], nums
      nums[3] = 40
      assert_equal [10, 21, 31, 40], nums
      assert_equal 4, nums.size
      assert_equal 11, (nums[0] = 11)
      assert_equal [11, 21, 31, 40], nums
      assert_equal 11, nums.fetch(0)
      assert_equal 40, nums.fetch(-1)
      assert_equal 11, nums.fetch(-4)
      assert_equal [99, 11], [nums.fetch(9, 99), nums.fetch(0, 99)]
      e = assert_raises(IndexError) { nums.fetch(4) }
      assert_equal "index 4 outside of array bounds: -4...4", e.message
      assert_equal IndexError, e.class
      e = assert_raises(IndexError) { nums.fetch(-5) }
      assert_equal "index -5 outside of array bounds: -4...4", e.message
      e = assert_raises(IndexError) { empty.fetch(0) }
      assert_equal "index 0 outside of array bounds: 0...0", e.message
      assert_raises(IndexError) { nums[-9] = 1 }
      assert_equal [11, 21, 31, 40, 50], nums.push(50)
      assert_equal [11, 21, 31, 40, 50, 60], (nums << 60)
      assert_equal [11, 21, 31, 40, 50, 60, 70], nums.append(70)
      assert_equal 70, nums.pop
      assert_equal 60, nums.pop
      assert_equal [11, 21, 31, 40, 50], nums
      assert_equal 11, nums.shift
      assert_equal [21, 31, 40, 50], nums
      assert_equal [5, 21, 31, 40, 50], nums.unshift(5)
      assert_nil empty.pop
      assert_nil empty.shift
      assert_equal [], empty
      assert_equal [5, 21, 31, 40, 50, 1, 2], (nums.concat([1, 2]))
      assert_equal [5, 21, 31, 40, 50, 1, 2], nums.concat([])
      assert_equal [5, 21, 31, 40, 50, 1, 2, 3], (nums + [3])
      assert_equal [5, 21, 31, 40, 50, 1, 2], nums
      assert_equal [], (empty + empty)
      assert_equal [2, 1, 50, 40, 31, 21, 5], nums.reverse
      assert_equal [5, 21, 31, 40, 50, 1, 2], nums
      assert_equal [], empty.reverse
      chain = [1] #: Array[Integer]
      chain << 2 << 3
      assert_equal [1, 2, 3], chain
      assert_equal true, chain.push(4).equal?(chain)
      assert_equal true, chain.append(5).equal?(chain)
      assert_equal true, chain.unshift(0).equal?(chain)
      assert_equal true, chain.concat([6]).equal?(chain)
      assert_equal false, (chain + []).equal?(chain)
      assert_equal false, chain.reverse.equal?(chain)
      copy = nums.dup
      copy << 99
      copy[0] = -1
      assert_equal [-1, 21, 31, 40, 50, 1, 2, 99], copy
      assert_equal [5, 21, 31, 40, 50, 1, 2], nums
      assert_equal false, copy.equal?(nums)
      alias_of = nums
      alias_of << 77
      assert_equal [5, 21, 31, 40, 50, 1, 2, 77], nums
      assert_equal true, alias_of.equal?(nums)
      grow_by_size(nums)
      grow_by_size(empty)
      assert_equal [5, 21, 31, 40, 50, 1, 2, 77, 8], nums
      assert_equal [0], empty
      assert_equal [3, 1, 2], ([3, 1, 3, 2, 1, 2].uniq)
      assert_equal ["b", "a"], (["b", "a", "b"].uniq)
      assert_equal [0], empty.uniq
      assert_equal ["apple", "bob"], %w[apple avocado bob banana].uniq { |w| w[0] }
      assert_equal ["apple", "bob"], %w[apple bob].uniq(&:size)
      extra = [2, 3] #: Array[Integer]
      maybe = (empty.size > 5 ? 9 : nil) #: Integer?
      assert_equal [1, 2, 3], [1, maybe, *extra].compact
      assert_equal [4, 5], ([4, 5].compact)
      assert_equal 21, nums.delete(21)
      assert_nil nums.delete(1000)
      assert_equal [5, 31, 40, 50, 1, 2, 77, 8], nums
      dup_vals = [1, 2, 1, 3, 1] #: Array[Integer]
      assert_equal 1, dup_vals.delete(1)
      assert_equal [2, 3], dup_vals
      words = ["x", "y", "x"] #: Array[String]
      assert_equal "x", words.delete("x")
      assert_equal ["y"], words
      assert_equal 5, nums.delete_at(0)
      assert_equal 8, nums.delete_at(-1)
      assert_nil nums.delete_at(100)
      assert_nil nums.delete_at(-100)
      assert_equal [31, 40, 50, 1, 2, 77], nums
      assert_equal [], nums.clear
      assert_equal true, nums.empty?
      assert_equal 0, nums.size
      assert_equal true, nums.clear.equal?(nums)
      queue = [1] #: Array[Integer]
      seen = [] #: Array[Integer]
      until queue.empty?
        x = queue.shift
        next unless x

        seen << x
        queue << x * 2 << x * 2 + 1 if x < 4
      end
      assert_equal [1, 2, 3, 4, 5, 6, 7], seen
      stack = [] #: Array[String]
      "a(b(c)d)e".each_char do |ch|
        if ch == "("
          stack.push(ch)
        elsif ch == ")"
          stack.pop
        end
      end
      assert_equal true, stack.empty?
      total = 0
      until seen.empty?
        top = seen.pop
        total += top if top
      end
      assert_equal 28, total
      assert_equal [], seen
      big = [9223372036854775807, -9223372036854775808, 0] #: Array[Integer]
      assert_equal [9223372036854775807, -9223372036854775808, 0], big
      assert_equal 9223372036854775807, big[0]
      assert_equal -9223372036854775808, big.fetch(1)
      grid = [[1], [2]] #: Array[Array[Integer]]
      gcopy = grid.dup
      gcopy[0]&.push(9)
      gcopy << [3]
      assert_equal [[1, 9], [2]], grid
      assert_equal [[1, 9], [2], [3]], gcopy
      assert_equal false, gcopy.equal?(grid)
      row = grid[0]
      row << 7 if row
      assert_equal [[1, 9, 7], [2]], grid
      assert_equal 1, grid[1]&.size
      assert_nil grid[5]&.size
      gap = []
      gap[2] = 1
      assert_equal [nil, nil, 1], gap
      assert_equal 3, gap.size
      ogap = [1] #: Array[Integer?]
      ogap[3] = 4
      assert_equal 4, ogap.size
      assert_equal true, big.is_a?(Array)
      assert_equal true, big.is_a?(Enumerable)
      assert_equal true, big.is_a?(Object)
      assert_equal false, big.is_a?(Integer)
      assert_equal true, big.kind_of?(Array)
      assert_equal true, big.respond_to?(:each)
      assert_equal false, big.respond_to?(:nope)
      assert_equal false, big.nil?
      assert_equal true, prime?(5)
      assert_equal false, prime?(4)
      assert_equal 4, PRIMES.size
      PRIMES << 11
      assert_equal true, prime?(11)
      assert_equal 11, PRIMES.last
      assert_equal 0, count_all
      assert_equal 2, (count_all([1, 2]))
      assert_equal [], evens([])
      assert_equal [2, 4], (evens([1, 2, 4]))
      assert_equal ["x"], array_pick(true)
      assert_equal [], array_pick(false)
      out = [] #: Array[Integer]
      out = evens([6]) if out.empty?
      assert_equal [6], out
      e = assert_raises(IndexError) { nums.fetch(3) }
      assert_equal "index 3 outside of array bounds: 0...0", e.message
    end
  end

  class ArrayBlocksTest < Minitest::Test
    NUMS = [4, 8, 15, 16, 23, 42] #: Array[Integer]

    def test_each_next_break
      nums = NUMS.dup
      ints = [] #: Array[Integer]
      nums.each { |n| ints << n if n.even? }
      assert_equal [4, 8, 16, 42], ints
      out = [] #: Array[String]
      nums.each do |n|
        next if n < 10
        break if n > 20

        out << "mid #{n}"
      end
      assert_equal ["mid 15", "mid 16"], out
      out = []
      nums.each_with_index { |n, i| out << "#{i}->#{n}" if i.odd? }
      assert_equal ["1->8", "3->16", "5->42"], out
      out = []
      nums.each_with_index do |n, i|
        break if i > 1

        out << "ewi #{n}"
      end
      assert_equal ["ewi 4", "ewi 8"], out
      ints = []
      nums.reverse_each { |n| ints << n }
      assert_equal [42, 23, 16, 15, 8, 4], ints
      out = []
      nums.reverse_each do |n|
        next if n.odd?
        break if n < 16

        out << "revbreak #{n}"
      end
      assert_equal ["revbreak 42", "revbreak 16"], out
      out = []
      nums.each_entry { |n| out << "entry #{n}" if n > 30 }
      assert_equal ["entry 42"], out
      out = []
      [].each { |x| out << "never #{x}" }
      assert_equal [], out
    end

    def test_return_from_block_in_method
      nums = NUMS.dup
      assert_equal 2, (index_of(nums, 15))
      assert_nil (index_of(nums, 99))
      assert_nil (index_of([], 1))
      assert_equal "cccc", (first_long(["a", "bb", "cccc", "ddddd"]))
      assert_equal "none", first_long([])
    end

    def test_yield_helpers
      nums = NUMS.dup
      ints = [] #: Array[Integer]
      array_each_even(nums) { |n| ints << n }
      assert_equal [4, 8, 16, 42], ints
      out = [] #: Array[String]
      array_each_even(nums) do |n|
        break if n > 10

        out << "even #{n}"
      end
      assert_equal ["even 4", "even 8"], out
      ints = []
      nums.each { ints << _1 * 100 if _1 > 40 }
      nums.each { ints << it - 1 if it < 5 }
      assert_equal [4200, 3], ints
      assert_equal [5, 9, 16, 17, 24, 43], (nums.map { _1 + 1 })
      assert_equal [15, 23], (nums.select { it.odd? })
      out = []
      nums.each_with_index { out << "#{_2}:#{_1}" if _2.zero? }
      assert_equal ["0:4"], out
      log = [] #: Array[String]
      safe_each([1, 0, 2], log) { |n| log << (10 / n).to_s }
      assert_equal ["10", "rescued divided by 0", "5"], log
      assert_equal 14, (sum_by([1, 2, 3]) { |n| n * n })
      assert_equal 0, (sum_by([]) { |n| n })
      out = []
      pairs_upto(3) { |i, s| out << "#{i}/#{s}" }
      assert_equal ["0/00", "1/11", "2/22"], out
      out = []
      pairs_upto(5) do |i, s|
        next if i == 1
        break if i == 3

        out << "#{i}|#{s}"
      end
      assert_equal ["0|00", "2|22"], out
      out = []
      pairs_upto(2) { |i| out << "only #{i}" }
      assert_equal ["only 0", "only 1"], out
      out = []
      pairs_upto(0) { |i, s| out << "never #{i}#{s}" }
      assert_equal [], out
    end

    def test_block_forwarding_class
      bag = Bag.new.add("x").add("yy").add("zzz")
      out = [] #: Array[String]
      bag.each { |s| out << s }
      assert_equal ["x", "yy", "zzz"], out
      out = []
      bag.each_twice { |s| out << s }
      assert_equal ["x", "yy", "zzz", "zzz", "yy", "x"], out
      out = []
      bag.each do |s|
        break if s.size > 1

        out << "bag first #{s}"
      end
      assert_equal ["bag first x"], out
      assert_equal ["yy", "zzz"], (bag.keep { |s| s.size > 1 })
      assert_equal 60, (bag.score { |s| s.size * 10 })
      assert_equal 2, (Util.second([1, 2, 3]))
      assert_nil Util.second(["a"])
      assert_equal [2, 3], (Util.second([[1], [2, 3]]))
      assert_equal ["1!", "2!"], (Util.my_map([1, 2]) { |n| n.to_s + "!" })
      assert_equal [1], (Util.my_map(["a"]) { |s| s.size })
      assert_equal [2, 3], (Util.without([1, 2, 1, 3], 1))
      assert_equal ["a", "b"], (Util.without(["a", "b"], "c"))
    end

    def test_closures_capture_locals
      nums = NUMS.dup
      total = 0
      [1, 2, 3].each { |n| total += n }
      assert_equal 6, total
      acc = [] #: Array[String]
      %w[a b c].each_with_index { |s, i| acc << s * (i + 1) }
      assert_equal ["a", "bb", "ccc"], acc
      counter = 0
      doubled = [1, 2, 3].map do |n|
        counter += 1
        n * counter
      end
      assert_equal [1, 4, 9], doubled
      assert_equal 3, counter
      labels = nums.map do |n|
        if n > 20
          "big"
        elsif n.odd?
          "odd"
        else
          "even"
        end
      end
      assert_equal ["even", "even", "odd", "even", "big", "big"], labels
    end

    def test_nested_blocks
      grid = [[1, 2], [3, 4], []] #: Array[Array[Integer]]
      ints = [] #: Array[Integer]
      grid.each { |row| row.each { |c| ints << c } }
      assert_equal [1, 2, 3, 4], ints
      out = [] #: Array[String]
      grid.each_with_index do |row, r|
        row.each_with_index { |c, col| out << "(#{r},#{col})=#{c}" if c.even? }
      end
      assert_equal ["(0,1)=2", "(1,1)=4"], out
      assert_equal [[1, 4], [9, 16], []], (grid.map { |row| row.map { |c| c * c } })
      assert_equal [1, 2, 3, 4], (grid.flat_map { |row| row })
      assert_equal [3, 7, 0], (grid.map { |row| row.reduce(0) { |a, b| a + b } })
      assert_equal 1, grid.select(&:empty?).size
      assert_equal [2, 2, 0], grid.map(&:size)
      assert_equal [0, 2, 2], grid.sort_by(&:size).map(&:size)
      assert_equal 3, ([1, 2, 3].then { |a| a.size })
      assert_equal [1, 3], ([3, 1].then { |a| a.sort })
      fns = [] #: Array[Integer]
      [1, 2, 3].each do |n|
        captured = n * 10
        fns << captured
      end
      assert_equal [10, 20, 30], fns
      sq = [[1, 2, 3], [4, 5, 6]] #: Array[Array[Integer]]
      assert_equal 11, (find_cell(sq, 5))
      assert_equal 0, (find_cell(sq, 1))
      assert_equal -1, (find_cell(sq, 9))
      out = []
      sq.each do |row|
        row.each do |v|
          next if v.even?
          break if v > 4

          out << v.to_s
        end
        out << "|"
      end
      assert_equal ["1", "3", "|", "|"], out
      assert_equal 6, (sum_by([-1, 2, -3], &:abs))
      assert_equal 1, (count_where(["a", "", "b"], &:empty?))
      assert_equal 2, (count_where(["a", "", "b"]) { |s| !s.empty? })
      small = [1, 2, 3] #: Array[Integer]
      sums = small.map do |n|
        r = 0
        small.each do |m|
          next if m == 2
          break if m > n

          r += m
        end
        r
      end
      assert_equal [1, 1, 4], sums
      found = small.select do |n|
        hit = false
        small.each_with_index { |m, i| hit = true if m == n && i.odd? }
        hit
      end
      assert_equal [2], found
      log = [] #: Array[String]
      show_all([1, 2], log)
      show_rev([3, 4], log)
      assert_equal ["all 1", "all 2", "rev 4", "rev 3"], log
      out = []
      array_each_even([2, 4, 12, 14]) do |n|
        pairs_upto(3) do |i, s|
          break if i > 1
          next if i.zero?

          out << "nest #{n} #{s}"
        end
        break if n > 10

        out << "after #{n}"
      end
      assert_equal ["nest 2 11", "after 2", "nest 4 11", "after 4", "nest 12 11"], out
    end
  end

  class ArrayBugFirstNegativeTest < Minitest::Test
    def test_first_and_take_reject_negative_counts
      nums = [1, 2] #: Array[Integer]
      e = assert_raises(ArgumentError) { nums.first(-1) }
      assert_equal "negative array size", e.message
      e = assert_raises(ArgumentError) { nums.take(-1) }
      assert_equal "attempt to take negative size", e.message
    end
  end

  class ArrayBugTupleAsUntypedTest < Minitest::Test
    def test_section_0
      pair = [1, "a"]
      assert_equal true, arr?(pair)
      assert_equal true, (arr?([1, 2]))
      assert_equal false, arr?(1)
      assert_equal "array 2", arr_kind(pair)
      assert_equal [1, "a"], pair
      u = [] #: Array[untyped]
      u << 1
      u << "a"
      assert_equal true, (pair == u)
      assert_equal true, (u == pair)
      assert_equal [1, 2, 3, 4, 5], (flat([1, [2, [3, [4]]], 5]))
    end
  end

  class ArrayBugsTest < Minitest::Test
    # a block on a Hash#[] result (Array?) receiver
    def test_a_block_on_a_hash
      hopt = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
      assert_equal [2, 4], (hopt["a"].map { |x| x * 2 })
      seen = [] #: Array[Integer]
      hopt["a"].each { |x| seen << x }
      assert_equal [1, 2], seen
    end

    # unused block params still bind positionally
    def test_unused_block_params_still_bind
      bp_nums = [1, 2] #: Array[Integer]
      seen = [] #: Array[Integer]
      bp_nums.each { |n| seen << n }
      bp_nums.each_with_index { |n, i| seen << i }
      assert_equal [1, 2, 0, 1], seen
      pairs = [[1, "a"]] #: Array[[Integer, String]]
      got = [] #: Array[untyped]
      pairs.each { |k, s| got << k << s }
      assert_equal [1, "a"], got
      assert_equal [1], (pairs.map { |k, s| k })
      # `_` in a param splatting an Array was emitted as `_ :=`
      rows = [["b", "2"], ["a", "1"]] #: Array[Array[String]]
      assert_equal ["2", "1"], (rows.map { |_, v| v })
      assert_equal ["a", "1"], (rows.min_by { |_, v| v.to_i })
      assert_equal ["b", "a"], (rows.map { |k, _, *| k })
    end

    # compact drops nils from Array[Integer?]
    def test_compact_drops_nils_from_array
      cmp_opt = [1, nil, 3, nil] #: Array[Integer?]
      assert_equal 2, cmp_opt.compact.size
      seen = [] #: Array[String]
      cmp_opt.compact.each { |x| seen << x.inspect }
      assert_equal ["1", "3"], seen
    end

    # compact! drops them in place, and answers nil when there were none (prime's sieve)
    def test_compact_bang_in_place
      xs = [1, nil, 2, nil] #: Array[Integer?]
      assert_equal [1, 2], xs.compact!
      assert_equal [1, 2], xs
      assert_nil xs.compact!
      assert_equal [1, 2], xs
    end

    # == across element types
    def test_across_element_types
      ints = [1, 2] #: Array[Integer]
      floats = [1.0, 2.0] #: Array[Float]
      eq_u = [1, 2] #: Array[untyped]
      assert_equal true, (ints == floats)
      assert_equal true, (ints == eq_u)
      assert_equal true, (eq_u == ints)
      nested = [[1, 2], [], [3]] #: Array[Array[Integer]]
      assert_equal true, (nested == [[1, 2], [], [3]])
    end

    # join flattens nested arrays
    def test_join_flattens_nested_arrays
      join_a = [] #: Array[untyped]
      join_a << 3
      join_a << [1, [2]]
      assert_equal "3,1,2", join_a.join(",")
      join_grid = [[1, 2], [3]] #: Array[Array[Integer]]
      assert_equal "1-2-3", join_grid.join("-")
    end

    # != compares by value, not identity
    def test_compares_by_value_not_identity
      ne_a = [1, 2] #: Array[Integer]
      ne_b = [1, 2] #: Array[Integer]
      assert_equal false, (ne_a != ne_b)
      assert_equal false, (ne_a != ne_a)
      assert_equal true, (ne_a != [3])
      assert_equal false, ([1] != [1])
    end

    # include?(nil) on Array[Integer?]
    def test_include_nil_on_array_integer
      inc_opt = [1, nil, 3] #: Array[Integer?]
      assert_equal true, inc_opt.include?(nil)
      assert_equal true, inc_opt.include?(3)
    end

    # nil elements inspect as nil
    def test_nil_elements_inspect_as_nil
      insp_opt = [1, nil, 3] #: Array[Integer?]
      assert_equal "[1, nil, 3]", insp_opt.inspect
      assert_equal "[1, nil, 3]", insp_opt.to_s
      assert_equal "interp [1, nil, 3]", ("interp #{insp_opt}")
      tail = [1, nil] #: Array[Integer?]
      assert_nil tail.last
      assert_nil tail.pop
      lit = [4, nil]
      assert_equal "[4, nil]", lit.inspect
    end

    # join and puts with nil elements
    def test_join_and_puts_with_nil
      jp_opt = [1, nil, 3] #: Array[Integer?]
      assert_equal "1--3", jp_opt.join("-")
    end

    # indexing Array[Integer?] yields a flat Integer?
    def test_indexing_array_integer_yields_a
      idx_opt = [1, nil, 3] #: Array[Integer?]
      assert_equal true, idx_opt[1].nil?
      assert_equal false, idx_opt[0].nil?
      assert_equal true, idx_opt[9].nil?
      branch = ""
      if (hole = idx_opt[1])
        branch = "truthy #{hole.inspect}"
      else
        branch = "falsy"
      end
      assert_equal "falsy", branch
      assert_nil idx_opt[1]
    end

    # sort/max_by with an optional (Hash#[]) key
    # sort! with a block (Homebrew's LinkageChecker#sort_by_formula_full_name!)
    def test_sort_bang_with_block
      a = [3, 1, 2]
      a.sort! { |x, y| y <=> x }
      assert_equal [3, 2, 1], a
    end

    def test_sort_max_by_with_an
      counts = { "a" => 3, "b" => 7, "c" => 5 } #: Hash[String, Integer]
      key_words = ["a", "b", "c"] #: Array[String]
      assert_equal "b", (key_words.max_by { |w| counts[w] })
      assert_equal "a", (key_words.min_by { |w| counts[w] })
      assert_equal ["a", "c", "b"], (key_words.sort_by { |w| [counts[w], w] })
      sort_opt = [3, 1, 2] #: Array[Integer?]
      assert_equal [1, 2, 3], sort_opt.sort
      assert_equal 3, sort_opt.max
    end

    # a splat rest param is a fresh array, not the caller's
    def test_splat_rest_param_is_fresh
      alias_words = ["p", "q"] #: Array[String]
      got = collect_rest(*alias_words)
      got[0] = "z"
      assert_equal ["p", "q"], alias_words
      assert_equal ["z", "q"], got
    end

    # splatting typed and untyped arrays into a rest param
    def test_splat_typed_and_untyped_into_rest
      elem_nums = [1, 2, 3] #: Array[Integer]
      assert_equal 6, total_elem(*elem_nums)
      elem_u = [] #: Array[untyped]
      elem_u << 4
      elem_u << 5
      assert_equal 9, total_elem(*elem_u)
    end

    # splats mixed with plain args
    def test_splats_mixed_with_plain_args
      mixed_nums = [1, 2, 3] #: Array[Integer]
      more = [4] #: Array[Integer]
      assert_equal 16, (total_mixed(10, *mixed_nums))
      assert_equal 11, (total_mixed(*mixed_nums, 5))
      assert_equal 10, (total_mixed(*mixed_nums, *more))
    end

    # tally/group_by key nested arrays by value
    def test_tally_group_by_key_nested
      tally_grid = [[1], [1], [2]] #: Array[Array[Integer]]
      assert_equal "{[1] => 2, [2] => 1}", (tally_grid.tally).inspect
      assert_equal 2, (tally_grid.group_by { |r| r }.size)
    end

    # to_a returns self, so << aliases
    def test_to_a_returns_self_so
      copy_a = [1, 2] #: Array[Integer]
      copy_b = copy_a.to_a
      copy_b << 3
      assert_equal [1, 2, 3], copy_a
      assert_equal true, copy_a.to_a.equal?(copy_a)
      assert_equal [nil, nil], min_max([])
      assert_equal [2, 2], min_max([2])
      show_nums = [1, 2] #: Array[Integer]
      assert_equal "1 2", show_list(show_nums)
      assert_equal [1, 2], ([] + show_nums)
      uniq_grid = [[1], [1], [2]] #: Array[Array[Integer]]
      assert_equal [[1], [2]], uniq_grid.uniq
      ps = [Pair.new(1, 2), Pair.new(1, 2)] #: Array[Pair]
      assert_equal 1, ps.uniq.size
      vs = [Val.new(1), Val.new(1), Val.new(2)] #: Array[Val]
      assert_equal 2, vs.uniq.size
    end

    # sort/min/max on an unannotated [] of integers
    def test_sort_min_max_on_an
      loose = []
      loose << 3
      loose << 1
      loose << 2
      assert_equal [1, 2, 3], loose.sort
      assert_equal 1, loose.min
      assert_equal 3, loose.max
      assert_equal [1, 2, 3], (loose.sort_by { |x| x })
    end

    # sort_by/max_by with an array key; sort of nested arrays
    def test_sort_by_max_by_with
      key_nums = [3, 1, 2] #: Array[Integer]
      assert_equal [2, 1, 3], (key_nums.sort_by { |n| [n % 2, n] })
      assert_equal 3, (key_nums.max_by { |n| [n, -n] })
      key_grid = [[2, 1], [1, 5]] #: Array[Array[Integer]]
      assert_equal [[1, 5], [2, 1]], key_grid.sort
      assert_equal [1, 5], key_grid.min
      assert_equal 2, (second_of([1, 2, 3]))
      assert_nil second_of(["a"])
    end

    # T? in a predicate block is truthiness; bare Array.new takes its annotation
    def test_t_in_a_predicate_block
      tb_h = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
      assert_equal "{\"a\" => 1}", ((tb_h.select { |_k, v| v })).inspect
      tb_h2 = { 1 => 2 } #: Hash[Integer, Integer]
      tb_nums = [1, 2] #: Array[Integer]
      assert_equal [1], (tb_nums.select { |n| tb_h2[n] })
      assert_equal ["a"], (["a", "b"].select { |s| s =~ /a/ })
      tb_a = Array.new #: Array[Integer]
      tb_a << 1
      assert_equal [1], tb_a
    end
  end

  class ArrayCustomEnumerableTest < Minitest::Test
    def test_section_0
      cd = Countdown.new(4)
      out = [] #: Array[String]
      cd.each { |n| out << "cd #{n}" }
      cd.each do |n|
        next if n == 3
        break if n == 1

        out << "cd2 #{n}"
      end
      assert_equal ["cd 4", "cd 3", "cd 2", "cd 1", "cd2 4", "cd2 2"], out
      assert_equal [16, 9, 4, 1], (cd.map { |n| n * n })
      assert_equal [4, 2], cd.select(&:even?)
      assert_equal [3, 1], cd.reject(&:even?)
      assert_equal [4, 3, 2, 1], cd.to_a
      assert_equal true, cd.include?(3)
      assert_equal false, cd.include?(9)
      assert_equal 4, cd.count
      assert_equal [1, 2, 3, 4], cd.sort
      assert_equal 1, cd.min
      assert_equal 4, cd.max
      assert_equal 24, (cd.reduce(1) { |a, n| a * n })
      assert_equal -10, (cd.inject(0) { |a, n| a - n })
      assert_equal [4, 3], cd.first(2)
      assert_equal [], cd.take(0)
      assert_equal [4, 3, 2, 1], cd.first(9)
      assert_equal "{4 => 1, 3 => 1, 2 => 1, 1 => 1}", (cd.tally).inspect
      assert_equal 2, (cd.find { |n| n < 3 })
      assert_nil (cd.detect { |n| n > 9 })
      assert_equal [2, 4, 1, 3], (cd.sort_by { |n| [n % 2, n.to_s] })
      assert_equal "{false => [4, 2], true => [3, 1]}", (cd.group_by(&:odd?)).inspect
      assert_equal [4, -4, 3, -3, 2, -2, 1, -1], (cd.flat_map { |n| [n, -n] })
      assert_equal 2, (cd.min_by { |n| (n - 2).abs })
      assert_equal 1, (cd.max_by { |n| -n })
      assert_equal true, (cd.any? { |n| n > 3 })
      assert_equal true, cd.all?(&:positive?)
      assert_equal false, (cd.none? { |n| n > 3 })
      out = []
      cd.each_with_index { |n, i| out << "#{i}:#{n}" }
      cd.each_with_index do |n, i|
        break if i == 2

        out << "ewi #{n}"
      end
      cd.each_entry { |n| out << "e#{n}" }
      assert_equal ["0:4", "1:3", "2:2", "3:1", "ewi 4", "ewi 3", "e4", "e3", "e2", "e1"], out
      zero = Countdown.new(0)
      assert_equal [], zero.to_a
      assert_nil zero.min
      assert_nil zero.max
      assert_equal 0, zero.count
      assert_equal [], zero.first(1)
      assert_equal true, (zero.all? { |n| n > 9 })
      assert_equal "{}", (zero.tally).inspect
      assert_equal [], zero.sort
      assert_equal [], (zero.map { |n| n })
      assert_nil (zero.min_by { |n| n })
      shelf = Shelf.new("Dune", "Emma", "Ulysses", "Beloved")
      assert_equal ["Beloved", "Dune", "Emma", "Ulysses"], shelf.sort
      assert_equal [4, 4, 7, 7], shelf.map(&:size)
      assert_equal "Ulysses", shelf.max_by(&:size)
      assert_equal true, shelf.include?("Emma")
      assert_equal ["Dune", "Ulysses", "Beloved"], (shelf.select { |t| t.include?("e") })
      assert_equal ["Dune", "Emma"], shelf.first(2)
      assert_equal 4, shelf.count
      assert_equal [], Shelf.new.to_a
      assert_nil Shelf.new.max
      assert_equal [1, 2, 3], Digits.sort
      assert_equal [6, 2, 4], (Digits.map { |d| d * 2 })
      assert_equal 3, Digits.max
      assert_equal 1, Digits.min
      assert_equal true, Digits.include?(2)
      assert_equal [3, 1, 2], Digits.to_a
      assert_equal 6, (Digits.reduce(0) { |a, d| a + d })
      assert_equal [3, 2, 1], (Digits.sort_by { |d| -d })
      assert_equal 3, Digits.count
      vs = [Version.new(2, 0), Version.new(1, 9), Version.new(1, 10)] #: Array[Version]
      assert_equal "[#<V v1.9>, #<V v1.10>, #<V v2.0>]", (vs.sort).inspect
      assert_equal "#<V v1.9>", (vs.min).inspect
      assert_equal "#<V v2.0>", (vs.max).inspect
      assert_equal ["v1.10", "v1.9", "v2.0"], (vs.sort_by { |v| -v.minor }.map(&:to_s))
      assert_equal "v2.0 < v1.9 < v1.10", (vs.map(&:to_s).join(" < "))
      assert_equal "#<V v2.0>", (vs.min_by(&:minor)).inspect
      assert_equal "#<V v2.0>", (vs.max_by(&:major)).inspect
      assert_equal "{2 => [#<V v2.0>], 1 => [#<V v1.9>, #<V v1.10>]}", (vs.group_by(&:major)).inspect
      assert_equal ["v2.0", "v1.9", "v1.10"], vs.map(&:to_s)
      mn = MoreNums.new(3, 8, 5)
      assert_equal 16, mn.total
      assert_equal [8], mn.evens
      assert_equal 8, mn.biggest
      assert_equal [3, 5, 8], mn.sort
      assert_equal [4, 9, 6], (mn.map { |x| x + 1 })
      assert_equal "3:[3, 5, 8]:true:[3]", mn.summary
      assert_equal [3, 8, 5], mn.to_a
      assert_equal true, mn.is_a?(Nums)
      assert_equal true, mn.is_a?(Enumerable)
      ints = [] #: Array[Integer]
      mn.each { |x| ints << x }
      assert_equal [3, 8, 5], ints
      assert_equal 0, MoreNums.new.biggest
      assert_equal "0:[]:false:[]", MoreNums.new.summary
      prs = Pairs.new
      out = []
      prs.each { |k, v| out << "#{k}=#{v}" }
      assert_equal ["a=1", "b=2"], out
      assert_equal ["a", "bb"], (prs.map { |k, v| k * v })
      assert_equal [["a", 1], ["b", 2]], prs.to_a
      assert_equal [["b", 2], ["a", 1]], (prs.sort_by { |_k, v| -v })
      assert_equal ["b", 2], (prs.find { |_k, v| v > 1 })
      assert_equal ["a", 1], (prs.min_by { |_k, v| v })
      assert_equal true, (prs.include?(["b", 2]))
      assert_equal [["a", 1], ["b", 2]], prs.sort
      assert_equal ["b", 2], prs.max
      assert_equal "{\"a\" => [[\"a\", 1]], \"b\" => [[\"b\", 2]]}", ((prs.group_by { |k, _v| k })).inspect
      assert_equal "{[\"a\", 1] => 1, [\"b\", 2] => 1}", (prs.tally).inspect
      out = []
      prs.each_with_index { |pr, i| out << "#{i}:#{pr.inspect}" }
      assert_equal ["0:[\"a\", 1]", "1:[\"b\", 2]"], out
      nat = Naturals.new
      assert_equal [0, 1, 2], nat.first(3)
      assert_equal [0, 1], nat.take(2)
      assert_equal 8, (nat.find { |n| n * n > 50 })
      assert_equal true, nat.include?(5)
      assert_equal true, (nat.any? { |n| n > 3 })
      assert_equal false, (nat.all? { |n| n < 3 })
      assert_equal false, (nat.none? { |n| n == 4 })
      assert_equal 1, nat.detect(&:positive?)
      ints = []
      nat.each do |n|
        break if n > 2

        ints << n
      end
      assert_equal [0, 1, 2], ints
      out = []
      nat.each_with_index do |n, i|
        break if i >= 2

        out << "#{i}:#{n}"
      end
      assert_equal ["0:0", "1:1"], out
      tol = Tolerant.new
      tol.each { |n| tol.log << (6 / (n - 2)).to_s }
      assert_equal ["-6", "tolerant 2", "ensure 2", "6"], tol.log
      tol.log.clear
      assert_equal 2, tol.find(&:even?)
      assert_equal ["ensure 2"], tol.log
      assert_equal [2, 4, 6], (tol.map { |n| n * 2 })
      assert_equal ["ensure 2", "ensure 2"], tol.log
      assert_equal true, tol.include?(2)
      assert_equal ["ensure 2", "ensure 2", "ensure 2"], tol.log
    end
  end

  class ArrayEnumerableTest < Minitest::Test
    def test_section_0
      words = ["pear", "fig", "apple", "fig", "kiwi", "Äpfel", "banana"] #: Array[String]
      nums = [5, -3, 0, 12, -3, 7] #: Array[Integer]
      none = [] #: Array[Integer]
      floats = [2.5, -1.0, 10.0, 0.1] #: Array[Float]
      assert_equal [10, -6, 0, 24, -6, 14], (nums.map { |n| n * 2 })
      assert_equal [], (none.map { |n| n * 2 })
      assert_equal [4, 3, 5, 3, 4, 5, 6], (words.map { |w| w.size })
      assert_equal ["PEAR", "FIG", "APPLE", "FIG", "KIWI", "ÄPFEL", "BANANA"], words.map(&:upcase)
      assert_equal ["5", "-3", "0", "12", "-3", "7"], nums.map(&:to_s)
      assert_equal [true, false, false, true, false, true], (nums.map { |n| n > 0 })
      assert_equal "[2.5, -1.5, 0.0, 6.0, -1.5, 3.5]", ((nums.map { |n| n.to_f / 2 })).inspect
      assert_equal [5, 12, 7], (nums.select { |n| n > 0 })
      assert_equal [-3, 0, -3], (nums.reject { |n| n > 0 })
      assert_equal [0], nums.select(&:zero?)
      assert_equal [], (none.select { |n| n > 0 })
      assert_equal [], (none.reject { |n| n > 0 })
      assert_equal [5, 0, 12, 7], nums.reject(&:negative?)
      assert_equal 12, (nums.find { |n| n > 6 })
      assert_nil (nums.find { |n| n > 100 })
      assert_equal -3, nums.detect(&:negative?)
      assert_nil (none.detect { |n| n > 0 })
      assert_equal true, (nums.any? { |n| n > 10 })
      assert_equal false, (nums.any? { |n| n > 100 })
      assert_equal false, (none.any? { |n| n > 0 })
      assert_equal true, (nums.all? { |n| n > -5 })
      assert_equal false, (nums.all? { |n| n > 0 })
      assert_equal true, (none.all? { |n| n > 0 })
      assert_equal true, (nums.none? { |n| n > 100 })
      assert_equal false, nums.none?(&:zero?)
      assert_equal true, (none.none? { |n| n > 0 })
      assert_equal 6, nums.count
      assert_equal 0, none.count
      assert_equal 7, words.count
      assert_equal 18, (nums.reduce(0) { |acc, n| acc + n })
      assert_equal 42, (none.reduce(42) { |acc, n| acc + n })
      assert_equal "5-3012-37", (nums.inject("") { |acc, n| acc + n.to_s })
      assert_equal 30, (words.inject(0) { |acc, w| acc + w.size })
      assert_equal 3780, (nums.reduce(1) { |acc, n| n.zero? ? acc : acc * n })
      assert_equal "11.6", ((floats.reduce(0.0) { |acc, f| acc + f })).inspect
      assert_equal true, nums.include?(12)
      assert_equal false, nums.include?(13)
      assert_equal false, none.include?(0)
      assert_equal true, words.include?("fig")
      assert_equal false, words.include?("Fig")
      assert_equal true, floats.include?(0.1)
      assert_equal false, ([0.1 + 0.2].include?(0.3))
      assert_equal [5, -3, 0, 12, -3, 7], nums.to_a
      assert_equal [], none.to_a
      assert_equal "{5 => 1, -3 => 2, 0 => 1, 12 => 1, 7 => 1}", (nums.tally).inspect
      assert_equal "{\"pear\" => 1, \"fig\" => 2, \"apple\" => 1, \"kiwi\" => 1, \"Äpfel\" => 1, \"banana\" => 1}", (words.tally).inspect
      assert_equal "{}", (none.tally).inspect
      assert_equal "{true => 2, false => 1}", (([true, false, true].tally)).inspect
      assert_equal [5, -3], nums.first(2)
      assert_equal [], nums.first(0)
      assert_equal [5, -3, 0, 12, -3, 7], nums.first(100)
      assert_equal [], none.first(1)
      assert_equal [5, -3, 0], nums.take(3)
      assert_equal ["pear"], words.take(1)
      seen = [] #: Array[Integer]
      nums.each_entry { |n| seen << n }
      assert_equal [5, -3, 0, 12, -3, 7], seen
      assert_equal [-3, -3, 0, 5, 7, 12], nums.sort
      assert_equal ["apple", "banana", "fig", "fig", "kiwi", "pear", "Äpfel"], words.sort
      assert_equal [], none.sort
      assert_equal "[-1.0, 0.1, 2.5, 10.0]", (floats.sort).inspect
      assert_equal [5, -3, 0, 12, -3, 7], nums
      assert_equal ["", "B", "Z", "a", "aa", "b", "é"], (["b", "B", "a", "é", "Z", "aa", ""].sort)
      assert_equal [:a, :b, :c], ([:b, :a, :c].sort)
      assert_equal ["fig", "fig", "kiwi", "pear", "apple", "Äpfel", "banana"], (words.sort_by { |w| [w.size, w] })
      assert_equal [12, 7, 5, 0, -3, -3], (nums.sort_by { |n| -n })
      assert_equal [], (none.sort_by { |n| n })
      assert_equal "[10.0, 2.5, 0.1, -1.0]", ((floats.sort_by { |f| -f })).inspect
      assert_equal ["apple", "banana", "fig", "fig", "kiwi", "pear", "Äpfel"], words.sort_by(&:downcase)
      assert_equal -3, nums.min
      assert_equal 12, nums.max
      assert_nil none.min
      assert_nil none.max
      assert_equal "apple", words.min
      assert_equal "Äpfel", words.max
      assert_equal "-1.0", (floats.min).inspect
      assert_equal "10.0", (floats.max).inspect
      assert_equal "fig", (words.min_by { |w| w.size })
      assert_equal "banana", (words.max_by { |w| w.size })
      assert_nil (none.min_by { |n| n })
      assert_nil (none.max_by { |n| n })
      assert_equal 0, (nums.min_by { |n| n.abs })
      assert_equal 12, nums.max_by(&:abs)
      assert_equal "{4 => [\"pear\", \"kiwi\"], 3 => [\"fig\", \"fig\"], 5 => [\"apple\", \"Äpfel\"], 6 => [\"banana\"]}", ((words.group_by { |w| w.size })).inspect
      assert_equal "{1 => [5, 12, 7], -1 => [-3, -3], 0 => [0]}", ((nums.group_by { |n| n <=> 0 })).inspect
      assert_equal "{}", (none.group_by(&:even?)).inspect
      assert_equal ["pear", "PEAR", "fig", "FIG", "apple", "APPLE", "fig", "FIG", "kiwi", "KIWI", "Äpfel", "ÄPFEL", "banana", "BANANA"], (words.flat_map { |w| [w, w.upcase] })
      assert_equal [5, -3, 0, 12, -3, 7], (nums.flat_map { |n| [n] })
      assert_equal [], (none.flat_map { |n| [n, n] })
      assert_equal [5, 5, 12, 12, 7, 7], (nums.flat_map { |n| n > 0 ? [n, n] : none })
      assert_equal 218, (nums.select(&:positive?).map { |n| n * n }.reduce(0) { |a, b| a + b })
      assert_equal ["apple", "banana", "fig"], words.map(&:downcase).uniq.sort.first(3)
      assert_equal [12, 7], nums.sort.reverse.take(2)
      assert_equal "{4 => 2, 3 => 2}", ((words.reject { |w| w.size > 4 }.map(&:size).tally)).inspect
      assert_equal ["4:2", "3:2", "5:2", "6:1"], (words.group_by(&:size).map { |k, v| "#{k}:#{v.size}" })
      assert_equal [5, -3, 0, 12, -3, 7], nums
      assert_equal ["pear", "fig", "apple", "fig", "kiwi", "Äpfel", "banana"], words
      acc0 = [] #: Array[Integer]
      assert_equal [10, -6, 0, 24, -6, 14], (nums.reduce(acc0) { |acc, n| acc + [n * 2] })
      assert_equal [], acc0
      assert_equal [5, -3, 0, 12, -3, 7], (nums.inject(acc0) { |acc, n| acc << n })
      assert_equal [5, -3, 0, 12, -3, 7], acc0
      h0 = {} #: Hash[Integer, String]
      assert_equal "{5 => \"55\", -3 => \"-3-3\", 0 => \"00\", 12 => \"1212\", 7 => \"77\"}", ((nums.reduce(h0) { |h, n| h[n] = n.to_s * 2; h })).inspect
      assert_equal "bb", (["bb", "aa", "c"].max_by(&:size))
      assert_equal "x", (["x", "yy", "z"].min_by(&:size))
      assert_equal "b", (["b", "a"].max_by { |_s| 0 })
      assert_equal "[0.0, 1.0]", (([0.0, -0.0, 1.0].uniq)).inspect
      assert_equal "{0.0 => 2}", (([0.0, -0.0].tally)).inspect
      assert_equal 3, ([3, 1, 3].max)
      odd = [4, 9, 2] #: Array[Integer]
      grid = [[1, 2], []] #: Array[Array[Integer]]
      assert_equal 10, odd.max&.succ
      assert_nil none.max&.succ
      assert_equal 3, (odd.min || 0) + 1
      assert_equal 1, (none.min || 0) + 1
      assert_equal "BANANA", (words.find { |w| w.size > 5 }&.upcase)
      assert_nil (words.find { |w| w.size > 9 }&.upcase)
      top = odd.max
      doubled = 0
      doubled = top * 2 if top
      assert_equal 18, doubled
      assert_equal 10, (odd[1] || -1) + 1
      assert_equal -1, (none[0] || -1)
      assert_equal 4, odd.first(1)[0]
      assert_equal 9, odd.sort.last
      assert_equal "4", (odd.map { |n| n.to_s }.max_by(&:size))
      assert_equal [2, 0], (grid.map { |r| r.max || 0 })
      assert_equal [2, 1], (grid.flat_map { |r| r }.sort.reverse.first(2))
      assert_equal [[9, "9"]], (odd.reject(&:even?).map { |n| [n, n.to_s] })
      assert_equal "2<4<9", odd.sort.map(&:to_s).join("<")
      bools = [true, false] #: Array[bool]
      syms = [:a, :b] #: Array[Symbol]
      flag = false
      assert_equal true, bools.include?(true)
      assert_equal true, bools.include?(false)
      assert_equal false, [true].include?(false)
      assert_equal true, syms.include?(:a)
      assert_equal false, syms.include?(:c)
      assert_equal true, bools.include?(flag)
      assert_equal "{true => 1, false => 1}", (bools.tally).inspect
      assert_equal [true, false], bools.uniq
      assert_equal 2, bools.count
      assert_equal [true], (bools.select { |b| b })
      assert_equal 1, (bools.reject { |b| b }.size)
      assert_equal [false, true], (bools.map { |b| !b })
      assert_equal false, (bools.all? { |b| b })
    end

    # grep / grep_v go through pattern === x.
    def test_grep_grep_v_go_through
      assert_equal [1], ([1, "a", :b, 2.5, nil].grep(Integer))
      assert_equal [2, 3, 4], (1..10).grep(2..4)
      assert_equal ["bb", "cb"], (%w[a bb cb].grep(/b/))
      assert_equal [2, 2], ([1, 2, 3, 2].grep(2))
      grep_mixed = [1, "a", 2.5] #: Array[untyped]
      assert_equal ["a"], (%w[a bb cb].grep_v(/b/))
      assert_equal [:ab], ([:ab, :cd].grep(/a/))
      assert_equal [1, "a"], grep_mixed.grep_v(Float)
    end
  end

  class ArrayFormatTest < Minitest::Test
    def test_section_0
      strs = ["é", "a\"b", "tab\there", "new\nline", "", "日本語", "\\", "\e", "#{1 + 1}"] #: Array[String]
      assert_equal "[\"é\", \"a\\\"b\", \"tab\\there\", \"new\\nline\", \"\", \"日本語\", \"\\\\\", \"\\e\", \"2\"]", strs.inspect
      assert_equal "[\"é\", \"a\\\"b\", \"tab\\there\", \"new\\nline\", \"\", \"日本語\", \"\\\\\", \"\\e\", \"2\"]", strs.to_s
      assert_equal "é|a\"b|tab\there|new\nline||日本語|\\|\e|2", strs.join("|")
      assert_equal "éa\"btab\therenew\nline日本語\\\e2", strs.join
      assert_equal "ab", (["a", "b"].join)
      assert_equal "a", (["a"].join(", "))
      assert_equal "ab", (["a", "b"].join(""))
      assert_equal "a→b", (["a", "b"].join("→"))
      nada = [] #: Array[String]
      assert_equal "", nada.join(",")
      assert_equal "[]", nada.inspect
      assert_equal "[]", nada.to_s
      floats = [1.0, 2.5, -0.0, 1e20, 1.0e-5, 100.0, 3.14159, 1e16, 123456789.123] #: Array[Float]
      assert_equal "[1.0, 2.5, -0.0, 1.0e+20, 1.0e-05, 100.0, 3.14159, 1.0e+16, 123456789.123]", (floats).inspect
      assert_equal "1.0 2.5 -0.0 1.0e+20 1.0e-05 100.0 3.14159 1.0e+16 123456789.123", (floats.join(" "))
      assert_equal "[0.30000000000000004, 0.3333333333333333]", (([0.1 + 0.2, 1.0 / 3])).inspect
      ints = [0, -1, 42, 1_000_000, 9223372036854775807, -9223372036854775808] #: Array[Integer]
      assert_equal "[0, -1, 42, 1000000, 9223372036854775807, -9223372036854775808]", ints.inspect
      assert_equal "0,-1,42,1000000,9223372036854775807,-9223372036854775808", ints.join(",")
      syms = [:a, :"b c", :c?, :d!, :e=, :+] #: Array[Symbol]
      assert_equal "[:a, :\"b c\", :c?, :d!, :e=, :+]", syms.inspect
      assert_equal "a-b c-c?-d!-e=-+", syms.join("-")
      bools = [true, false, true] #: Array[bool]
      assert_equal "[true, false, true]", bools.inspect
      assert_equal "true,false,true", bools.join(",")
      nested = [[1, 2], [], [3, [4, 5].size]] #: Array[Array[Integer]]
      assert_equal "[[1, 2], [], [3, 2]]", nested.inspect
      assert_equal "[[1, 2], [], [3, 2]]", nested.to_s
      assert_equal "[1, 2]", nested[0].inspect
      assert_equal "[]", nested[1].inspect
      assert_equal "nil", nested[5].inspect
      deep = [[["x"]], [[]]] #: Array[Array[Array[String]]]
      assert_equal "[[[\"x\"]], [[]]]", deep.inspect
      maps = [{ a: 1 }, { "b c" => [2] }, {}] #: Array[Hash[untyped, untyped]]
      assert_equal "[{a: 1}, {\"b c\" => [2]}, {}]", (maps).inspect
      holes = [nil, nil]
      assert_equal "[nil, nil]", holes.inspect
      assert_equal 2, holes.size
      assert_equal "[nil, nil]", holes.to_s
      assert_equal "interp [1, \"two\", :three, nil, 4.0]", ("interp #{[1, "two", :three, nil, 4.0]}")
      assert_equal "empty []", ("empty #{[]}")
      assert_equal true, ([1, 2] == [1, 2])
      assert_equal false, ([1, 2] == [2, 1])
      assert_equal false, ([1, 2] == [1, 2, 3])
      assert_equal false, ([1, 2, 3] == [1, 2])
      assert_equal true, (nested == nested.dup)
      assert_equal false, (nested == [[1, 2], [3, 2]])
      assert_equal false, (nested == [[1, 2], [7], [3, 2]])
      assert_equal true, ([[1], [2]] == [[1], [2]])
      assert_equal true, (ints == ints.dup)
      assert_equal true, (strs == strs.reverse.reverse)
      assert_equal false, (floats == [1.0])
      assert_equal false, ([1] == "1")
      assert_equal true, (["a"] == ["a"])
      assert_equal false, (["a"] == ["A"])
      multi = [
        1,
        2, # two
        3,
      ] #: Array[Integer]
      assert_equal "[1, 2, 3]", multi.inspect
      assert_equal "[1.5, -0.0]", ("#{[1.5, -0.0]}")
      w = %w[]
      assert_equal "[]", w.inspect
      assert_equal 0, w.size
      fl = [3.5, 1.25, 2.0] #: Array[Float]
      assert_equal "[1.25, 2.0, 3.5]", (fl.sort).inspect
      assert_equal "3.5", (fl.max).inspect
      assert_equal "3.5", ((fl.min_by { |f| -f })).inspect
      assert_equal "[3.5, 2.0, 1.25]", ((fl.sort_by { |f| f }.reverse)).inspect
      assert_equal true, fl.include?(2.0)
    end
  end

  class ArrayMidTest < Minitest::Test
    # each re-reads the length, so pushes, deletes and clear mid-loop match MRI
    def test_each_re_reads_the_length
      grow = [1, 2] #: Array[Integer]
      seen = [] #: Array[Integer]
      grow.each do |x|
        grow << x * 10 if x < 10
        seen << x
      end
      assert_equal [1, 2, 10, 20], seen
      assert_equal [1, 2, 10, 20], grow
      shrink = [1, 2, 3, 4] #: Array[Integer]
      seen = []
      shrink.each do |x|
        seen << x
        shrink.delete(x)
      end
      assert_equal [1, 3], seen
      assert_equal [2, 4], shrink
      cleared = [1, 2] #: Array[Integer]
      seen = []
      cleared.each do |x|
        cleared.clear
        seen << x
      end
      assert_equal [1], seen
      assert_equal [1, 2], Safe.new.to_a
      safe = Safe.new
      safe.each { |x| safe.log << (10 / (x - 1)).to_s }
      assert_equal ["rescued", "10"], safe.log
      pt1 = MidPt.new(1)
      pts = [pt1, MidPt.new(2)] #: Array[MidPt]
      assert_equal true, pts.include?(pt1)
      assert_equal false, pts.include?(MidPt.new(1))
      mixed_inc = []
      mixed_inc << 3
      mixed_inc << "x"
      assert_equal true, mixed_inc.include?(3)
      assert_equal true, mixed_inc.include?("x")
      assert_equal false, mixed_inc.include?("y")
      grid = [[1, 2, 3], [4, 5, 6]] #: Array[Array[Integer]]
      assert_equal [1, 1], (find_pos(grid, 5))
      assert_nil (find_pos(grid, 9))
      assert_equal 0, count_of([])
      assert_equal -1, count_of(nil)
      assert_equal 1, count_of([7])
      groups = { "odd" => [1] } #: Hash[String, Array[Integer]]
      assert_equal 0, (groups.fetch("even", []).size)
      assert_nil (pass_through(1) <=> "a")
      assert_nil (pass_through(:a) <=> 1)
      mixed = []
      mixed << 3
      mixed << "x"
      cmp_err = assert_raises(ArgumentError) { mixed.sort }
      assert_equal true, cmp_err.message.start_with?("comparison of ")
      cmp_err = assert_raises(ArgumentError) { mixed.max }
      assert_equal true, cmp_err.message.end_with?(" failed")
      idx_arr = %w[a b c b]
      assert_equal 1, idx_arr.index("b")
      assert_nil idx_arr.index("z")
      assert_equal 1, (idx_arr.index { |x| x > "a" })
      assert_equal 2, idx_arr.find_index("c")
      assert_nil (idx_arr.find_index { |x| x == "q" })
      assert_equal 3, idx_arr.rindex("b")
      assert_nil idx_arr.rindex("q")
      pairs = [["a", "1"], ["b"]] #: Array[Array[String]]
      got = [] #: Array[String]
      pairs.each { |k, v| got << [k, v].inspect }
      assert_equal ["[\"a\", \"1\"]", "[\"b\", nil]"], got
      opt = [["x", nil]] #: Array[Array[String?]]
      got = []
      opt.each { |k, v| got << k.inspect << v.inspect }
      assert_equal ["\"x\"", "nil"], got
      nums = [[1, 2, 3]] #: Array[Array[Integer]]
      assert_equal [3], (nums.map { |a, b| a.to_i + b.to_i })
      en_a = [5, 3, 8, 1, 9, 2]
      en_e = [] #: Array[Integer]
      assert_equal 4, (en_a.count { |x| x > 2 })
      assert_equal 1, en_a.count(3)
      assert_equal 28, (en_a.inject { |s, x| s + x })
      assert_equal 2160, (en_a.reduce { |s, x| s * x })
      assert_nil (en_e.inject { |s, x| s + x })
      assert_equal [10, 6, 16, 2, 18, 4], (en_a.each_with_object([]) { |x, acc| acc << x * 2 })
      assert_equal [50, 30, 10, 90], (en_a.filter_map { |x| x * 10 if x.odd? })
      assert_equal ["v1.2", "v3.4"], (%w[1.2 x 3.4].filter_map { |v| Version.parse(v) }.map(&:to_s))
      assert_equal [[8, 2], [5, 3, 1, 9]], en_a.partition(&:even?)
      assert_equal [1, 9], en_a.minmax
      assert_equal [nil, nil], en_e.minmax
      assert_equal [1, 2], en_a.min(2)
      assert_equal [9, 8], en_a.max(2)
      assert_equal [9, 8, 5, 3, 2, 1], (en_a.sort { |x, y| y <=> x })
      assert_equal [5, 3, 8], (en_a.take_while { |x| x > 2 })
      assert_equal [1, 9, 2], (en_a.drop_while { |x| x > 2 })
      assert_equal [9, 2], en_a.drop(4)
      slices = [] #: Array[Array[Integer]]
      en_a.each_slice(4) { |s| slices << s }
      assert_equal [[5, 3, 8, 1], [9, 2]], slices
      assert_equal [[5, 3, 8, 1], [9, 2]], en_a.each_slice(4).to_a
      assert_equal [[5, 3, 8, 1, 9], [3, 8, 1, 9, 2]], en_a.each_cons(5).to_a
      assert_equal [8, 9, 11], en_a.each_slice(2).map(&:sum)
      slices = []
      en_a.each_cons(5) { |c| slices << c }
      assert_equal [[5, 3, 8, 1, 9], [3, 8, 1, 9, 2]], slices
      assert_equal [[5, "a"], [3, "b"], [8, "c"], [1, nil], [9, nil], [2, nil]], (en_a.zip(%w[a b c]))
      assert_equal [[1, 2, 3], [5], [8, 9]], (en_a.sort.chunk_while { |x, y| y == x + 1 }.to_a)
      assert_equal [[5], [3, 8], [1, 9], [2]], (en_a.slice_when { |x, y| y < x }.to_a)
      assert_equal [0, 3, 16, 3, 36, 10], (en_a.each_with_index.map { |x, i| x * i })
      assert_equal [5, 3, 1, 9], en_a.find_all(&:odd?)
      assert_equal [1, 2], (en_a.filter { |x| x < 3 })
      en_h = { a: 1, b: 2, c: 3 }
      assert_equal [:a, :c], (en_h.filter_map { |k, v| k if v.odd? })
      assert_equal "{1 => :a, 2 => :b, 3 => :c}", ((en_h.each_with_object({}) { |(k, v), acc| acc[v] = k })).to_s
      assert_equal [[[:b, 2], [:c, 3]], [[:a, 1]]], (en_h.partition { |k, v| v > 1 })
      assert_equal [:c, 3], (en_h.min_by { |k, v| -v })
      assert_equal 2, (en_h.count { |k, v| v > 1 })
      assert_equal 6, (en_h.sum { |k, v| v })
      assert_equal [[[:a, 1], [:b, 2]], [[:c, 3]]], en_h.each_slice(2).to_a
      assert_equal [[:c, 3], [:b, 2], [:a, 1]], (en_h.sort_by { |k, v| -v })
      ar_a = [1, 2, 3, 4, 2]
      ar_b = [2, 4, 6]
      assert_equal [1, 3], (ar_a - ar_b)
      assert_equal [2, 4], (ar_a & ar_b)
      assert_equal [1, 2, 3, 4, 6], (ar_a | ar_b)
      assert_equal [2, 3, 4, 2, 1], ar_a.rotate
      assert_equal [2, 1, 2, 3, 4], ar_a.rotate(-1)
      assert_equal [3, 4, 2, 1, 2], ar_a.rotate(7)
      assert_equal [1, 3, nil], (ar_a.values_at(0, 2, 9))
      assert_equal true, ar_a.intersect?(ar_b)
      assert_equal [1, 2, 3, 4, 9], ar_a.union([9])
      assert_equal [[1, 2], [1, 2, 3]], [[1, 1, 2].union, [1].union([2, 1], [3])]
      assert_equal [2, 3, 4, 2], ar_a.difference([1])
      assert_equal [[1, 2], [1, 3], [2, 3]], ([1, 2, 3].combination(2).to_a)
      assert_equal 6, ([1, 2, 3].permutation(2).to_a.size)
      assert_equal [[1, 3], [1, 4], [2, 3], [2, 4]], ([1, 2].product([3, 4]))
      assert_equal [[1, 2, 3], [1, 3, 2]], ([1, 2, 3].permutation.first(2))
      assert_equal [[]], ([1, 2].combination(0).to_a)
      assert_equal 5, ([1, 3, 5, 7, 9].bsearch { |x| x >= 4 })
      assert_nil ([1, 3].bsearch { |x| x > 9 })
      assert_equal 2, ar_a.count(2)
      ar_c = [5, 1, 4]
      ar_c.sort!
      assert_equal [1, 4, 5], ar_c.dup
      ar_c.map! { |x| x * 2 }
      assert_equal [2, 8, 10], ar_c.dup
      assert_equal [8, 10], (ar_c.select! { |x| x > 2 })
      assert_nil (ar_c.select! { |x| x > 2 })
      assert_equal [8], (ar_c.reject! { |x| x > 9 })
      assert_nil (ar_c.reject! { |x| x > 99 })
      ar_c.delete_if { |x| x == 8 }
      assert_equal [], ar_c
      ar_c.keep_if(&:positive?)
      assert_equal [], ar_c
      ar_d = [3, 1, 3, 2]
      assert_equal [3, 1, 2], ar_d.uniq!
      assert_nil ar_d.uniq!
      ar_d.reverse!
      assert_equal [2, 1, 3], ar_d.dup
      assert_equal [2, 9, 8, 1, 3], (ar_d.insert(1, 9, 8).dup)
      assert_equal [2, 9, 8, 1, 7, 3], (ar_d.insert(-2, 7).dup)
      assert_equal [0, 1, 2, 3, 4, 5], ar_d.each_index.to_a
      assert_equal [0, 0, 0, 0, 0, 0], ar_d.fill(0)
      ar_e = [1, 2, 3]
      assert_equal [[1, 0], [2, 1], [3, 2]], ar_e.each_with_index.to_a
      assert_equal [[1, 1], [2, 2], [3, 3]], ar_e.each.with_index(1).to_a
      assert_equal [0, 2, 6], (ar_e.map.with_index { |x, i| x * i })
      assert_equal [[1, 1], [2, 2], [3, 3]], (ar_e.map.with_index(1) { |x, i| [i, x] })
      assert_equal [1, 3], (ar_e.select.with_index { |x, i| i.even? })
      assert_equal [2, 3], (ar_e.reject.with_index { |x, i| i.zero? })
      got = [] #: Array[String]
      ar_e.each.with_index(1) { |x, i| got << "#{x}#{i}" }
      ar_e.each_index { |i| got << i.to_s }
      assert_equal ["11", "22", "33", "0", "1", "2"], got
      ar_w = %w[b a c]
      ar_w.sort_by! { |s| s }
      assert_equal ["a", "b", "c"], ar_w
      me_nums = [1, 2] #: Array[Integer]
      assert_equal "#<Enumerator: [1, 2]:map>", (me_nums.map).inspect
      assert_equal "#<Enumerator: [1, 2]:select>", me_nums.select.inspect
      assert_equal [1, 2], me_nums.reject.to_a
    end
  end

  class ArrayObjectsTest < Minitest::Test
    # an unannotated literal of sibling classes joins to their superclass
    def test_an_unannotated_literal_of_sibling
      shapes = [Circle.new(1.0), Sq.new(2.0), Shape.new]
      assert_equal ["circle", "shape", "shape"], shapes.map(&:name)
      assert_equal "[3.0, 4.0, 0.0]", (shapes.map(&:area)).inspect
      assert_equal 3, shapes.size
      assert_equal "shape", shapes.max_by(&:area)&.name
      assert_equal ["shape", "circle", "shape"], shapes.sort_by(&:area).map(&:name)
      assert_equal ["circle", "shape"], (shapes.select { |s| s.area > 1.0 }.map(&:name))
      assert_equal "shape", shapes.min_by(&:area)&.name
      two = [Circle.new(2.0), Sq.new(1.0)]
      assert_equal ["circle", "shape"], two.map(&:name)
      assert_equal "[1.0, 12.0]", (two.reverse.map(&:area)).inspect
      shapes << Sq.new(3.0)
      assert_equal "9.0", (shapes.last&.area).inspect
      assert_equal 4, shapes.count
    end

    # an array ivar exposed by attr_reader is the same object
    def test_an_array_ivar_exposed_by
      box = Inbox.new
      box.add("a").add("c")
      box.items << "b"
      assert_equal ["a", "c", "b"], box.items
      assert_equal 3, box.items.size
      assert_equal ["a", "b", "c"], box.items.sort
      assert_equal true, box.items.equal?(box.items)
      snapshot = box.items.dup
      box.items.clear
      assert_equal ["a", "c", "b"], snapshot
      assert_equal [], box.items
    end

    # user-defined == drives Array#==, delete; inspect uses the user's inspect
    def test_user_defined_drives_array_delete
      pts = [Pt.new(1), Pt.new(2)] #: Array[Pt]
      assert_equal true, ([Pt.new(2)] == [Pt.new(2)])
      assert_equal false, ([Pt.new(2)] == [Pt.new(3)])
      assert_equal 2, ([Pt.new(1), Pt.new(1)].uniq.size)
      assert_equal "P1", (pts.delete(Pt.new(1))).inspect
      assert_equal "[P2]", (pts).inspect
      assert_equal "[P2]", pts.to_s
      assert_equal "[P2]", "#{pts}"
      assert_nil pts.delete(Pt.new(9))
    end

    # elements are references: mutating one through the array is seen outside
    def test_elements_are_references_mutating_one
      p1 = Plain.new(1)
      plains = [p1, Plain.new(2)] #: Array[Plain]
      plains.each { |pl| pl.n = pl.n + 10 }
      assert_equal 11, p1.n
      assert_equal [11, 12], plains.map(&:n)
      plains << p1
      assert_equal 11, plains.delete(p1)&.n
      assert_equal 1, plains.size
      plains.push(Plain.new(3))
      plains.unshift(Plain.new(0))
      assert_equal [0, 12, 3], plains.map(&:n)
    end

    # arrays stored in a Hash and a Struct are shared, not copied
    def test_arrays_in_hash_and_struct_are_shared
      groups = {} #: Hash[String, Array[Integer]]
      [1, 2, 3, 4, 5].each do |n|
        key = n.even? ? "even" : "odd"
        list = groups[key]
        if list
          list << n
        else
          groups[key] = [n]
        end
      end
      assert_equal "{\"odd\" => [1, 3, 5], \"even\" => [2, 4]}", (groups).inspect
      evens = groups["even"]
      evens << 100 if evens
      assert_equal "{\"odd\" => [1, 3, 5], \"even\" => [2, 4, 100]}", (groups).inspect
      assert_equal ["odd:3", "even:3"], (groups.map { |k, v| "#{k}:#{v.size}" })
      bx = Box.new([1])
      bx.items << 2
      held = bx.items
      held << 3
      assert_equal [1, 2, 3], bx.items
      assert_equal "#<struct ArrayTests::Box items=[1, 2, 3]>", (bx).inspect
    end

    # user methods on a reopened Array reach literals and joined-class arrays
    def test_reopened_array_methods
      shapes = [Circle.new(1.0), Sq.new(2.0), Shape.new]
      shapes << Sq.new(3.0)
      assert_equal 2, ([1, 2, 3].second)
      assert_nil ["a"].second
      assert_equal 0, [].twice_size
      assert_equal 8, shapes.twice_size
      assert_equal "shape", shapes.second&.name
    end
  end

  class ArraySplatTest < Minitest::Test
    def test_section_0
      nums = [1, 2, 3] #: Array[Integer]
      none = [] #: Array[Integer]
      assert_equal 0, array_total
      assert_equal 5, array_total(5)
      assert_equal 3, (array_total(1, 2))
      assert_equal 6, array_total(*nums)
      assert_equal 0, array_total(*none)
      assert_equal 0, (array_total(-5, 5))
      assert_equal "x:[]:0:true", array_label("x")
      assert_equal "y:[1]:1:false", (array_label("y", 1))
      assert_equal "z:[1, 2, 3]:3:false", (array_label("z", *nums))
      assert_equal "w:[]:0:true", (array_label("w", *none))
      assert_equal ["a", "b"], (collect("a", "b"))
      assert_equal [], collect
      assert_equal ["é"], collect("é")
      words = ["p", "q"] #: Array[String]
      assert_equal ["p", "q"], collect(*words)
      got = collect(*words)
      got << "r"
      assert_equal ["p", "q", "r"], got
      assert_equal ["p", "q"], words
      assert_equal "", kinds
      assert_equal "1,\"a\",:b,nil,2.5,[1],true", (kinds(1, "a", :b, nil, 2.5, [1], true))
      assert_equal "p", opt_then_rest
      assert_equal "q", opt_then_rest("q")
      assert_equal "r12", (opt_then_rest("r", 1, 2))
      assert_equal "", pairs
      assert_equal "aa/b", (pairs([2, "a"], [1, "b"]))
      pa = [[3, "x"], [0, "y"]] #: Array[[Integer, String]]
      assert_equal "xxx/", pairs(*pa)
      assert_equal 6, (wrap(1, 2, 3))
      assert_equal 0, wrap
      assert_equal 6, wrap(*nums)
      assert_equal [1, 99], array_grow(1)
      assert_equal [99], array_grow
      assert_equal [1, 2, 3, 99], array_grow(*nums)
      assert_equal [1, 2, 3], nums
      assert_equal 3, (first_plus(0, *nums))
      assert_equal 5, first_plus(5)
      assert_equal 2, (first_plus(1, 2))
    end
  end

  class ArrayTuplesTest < Minitest::Test
    def test_section_0
      pair = [1, "one"]
      assert_equal [1, "one"], pair
      assert_equal "[1, \"one\"]", pair.to_s
      assert_equal 1, pair[0]
      assert_equal "one", pair[1]
      assert_equal 1, pair.first
      assert_equal "one", pair.last
      assert_equal "interp [1, \"one\"]", ("interp #{pair}")
      trio = [2, "two", :b]
      assert_equal [2, "two", :b], trio
      assert_equal :b, trio[2]
      assert_equal :b, trio.last
      assert_equal 2, trio.first
      mixed = [1, 2.5]
      assert_equal "[1, 2.5]", (mixed).inspect
      nested = [[1, 2], "x"]
      assert_equal [[1, 2], "x"], nested
      assert_equal 2, nested.first.size
      assert_equal [1, 2], nested[0]
      assert_equal true, (pair == [1, "one"])
      assert_equal false, (pair == [2, "one"])
      assert_equal false, (pair != [1, "one"])
      assert_equal true, (pair != [1, "two"])
      assert_equal -1, ([1, "a"] <=> [1, "b"])
      assert_equal 1, ([2, "a"] <=> [1, "z"])
      assert_equal 0, ([1, "a"] <=> [1, "a"])
      assert_equal "seven=7", (render([7, "seven"]))
      assert_equal "one=1", render(pair)
      assert_equal "[1, \"a\"]", (show_any([1, "a"]))
      assert_equal "[[1, \"b\"], 2]", (show_any([[1, "b"], 2]))
      assert_equal "[1, \"one\"]", show_any(pair)
      q, r = array_divmod2(17, 5)
      assert_equal 3, q
      assert_equal 2, r
      q, r = array_divmod2(-17, 5)
      assert_equal -4, q
      assert_equal 3, r
      assert_equal [0, 0], (array_divmod2(0, 3))
      assert_equal [-3, -3], (array_divmod2(9, -4))
      up, len, emp = describe_str("héllo")
      assert_equal "HÉLLO", up
      assert_equal 5, len
      assert_equal false, emp
      assert_equal ["", 0, true], describe_str("")
      assert_equal ["日本", 2, false], describe_str("日本")
      lo, hi = bounds([3, 9, -2])
      assert_equal -2, lo
      assert_equal 9, hi
      lo, hi = bounds([])
      assert_nil lo
      assert_equal true, hi.nil?
      x, y = [3, "three"]
      assert_equal 3, x
      assert_equal "three", y
      a = 1
      b = 2
      a, b = b, a
      assert_equal 2, a
      assert_equal 1, b
      a, b = b, a + b
      assert_equal 1, a
      assert_equal 3, b
      first, second = [10, 20, 30] #: Array[Integer]
      assert_equal 10, first
      assert_equal 20, second
      m, n = [5] #: Array[Integer]
      assert_equal 5, m
      assert_nil n
      pairs = [[3, "c"], [1, "a"], [2, "b"]] #: Array[[Integer, String]]
      assert_equal [[3, "c"], [1, "a"], [2, "b"]], pairs
      assert_equal 3, pairs.size
      assert_equal [[1, "a"], [2, "b"], [3, "c"]], pairs.sort
      assert_equal [1, "a"], pairs.min
      assert_equal [3, "c"], pairs.max
      assert_equal [[2, "b"], [1, "a"], [3, "c"]], pairs.reverse
      out = [] #: Array[String]
      pairs.each { |num, s| out << "#{num}=#{s}" }
      assert_equal ["3=c", "1=a", "2=b"], out
      out = []
      pairs.each { |pr| out << pr.inspect }
      assert_equal ["[3, \"c\"]", "[1, \"a\"]", "[2, \"b\"]"], out
      out = []
      pairs.each_with_index { |pr, i| out << "#{i}:#{pr.inspect}:#{pr.last}" }
      assert_equal ["0:[3, \"c\"]:c", "1:[1, \"a\"]:a", "2:[2, \"b\"]:b"], out
      assert_equal ["ccc", "a", "bb"], (pairs.map { |num, s| s * num })
      assert_equal [[3, "c"], [1, "a"]], (pairs.select { |num, _s| num.odd? })
      assert_equal [[2, "b"]], (pairs.reject { |num, _s| num.odd? })
      assert_equal [[3, "c"], [2, "b"], [1, "a"]], (pairs.sort_by { |num, s| [-num, s] })
      assert_equal ["c", "a", "b"], (pairs.map { |pr| pr.last })
      assert_equal [3, 1, 2], pairs.map(&:first)
      assert_equal [["c", 3], ["a", 1], ["b", 2]], (pairs.map { |num, s| [s, num] })
      assert_equal true, (pairs.include?([1, "a"]))
      assert_equal false, (pairs.include?([1, "b"]))
      assert_equal [3, "c"], (pairs.find { |num, _s| num > 1 })
      assert_equal true, (pairs.any? { |_n, s| s == "b" })
      assert_equal "{true => [[3, \"c\"], [1, \"a\"]], false => [[2, \"b\"]]}", ((pairs.group_by { |num, _s| num.odd? })).inspect
      assert_equal [1, "a"], (pairs.min_by { |_n, s| s })
      assert_equal [3, "c"], (pairs.max_by { |num, _s| num })
      assert_equal 6, (pairs.reduce(0) { |acc, pr| acc + pr[0] })
      assert_equal ["c=3", "a=1", "b=2"], (pairs.map { |pr| render(pr) })
      pairs << [0, "z"]
      pairs.push([9, "i"])
      assert_equal [[3, "c"], [1, "a"], [2, "b"], [0, "z"], [9, "i"]], pairs
      assert_equal [9, "i"], pairs.last
      assert_equal [3, "c"], pairs[0]
      assert_nil pairs[99]
      dups = [[1, "a"], [1, "a"], [2, "a"]] #: Array[[Integer, String]]
      assert_equal [[1, "a"], [2, "a"]], dups.uniq
      assert_equal "{[1, \"a\"] => 2, [2, \"a\"] => 1}", (dups.tally).inspect
      assert_equal [1, "a"], (dups.delete([1, "a"]))
      assert_equal [[2, "a"]], dups
      assert_equal [["b", 98], ["a", 97], ["c", 99]], (["b", "a", "c"].map { |s| [s, s.ord] })
      assert_equal [[1, "a"], [2, "bb"], [3, "ccc"]], (["bb", "a", "ccc"].map { |s| [s.size, s] }.sort)
      ab = [1, 2, 3] #: Array[Integer]
      a1, b1 = ab
      assert_equal 1, a1
      assert_equal 2, b1
      w1, w2, w3, w4 = ab
      assert_equal 1, w1
      assert_equal 2, w2
      assert_equal 3, w3
      assert_nil w4
      list, cnt = split_list
      list << cnt
      assert_equal [1, 2, 3], list
      assert_equal [[1, 2], 3], split_list
      cells = {} #: Hash[[Integer, Integer], String]
      cells[[0, 1]] = "a"
      cells[[2, 3]] = "b"
      assert_equal "a", (cells[[0, 1]])
      assert_nil (cells[[9, 9]])
      assert_equal true, (cells.key?([2, 3]))
      assert_equal 2, cells.size
      cells[[0, 1]] = "c"
      assert_equal "{[0, 1] => \"c\", [2, 3] => \"b\"}", (cells).inspect
      coords = [[0, 1], [0, 1], [1, 1]] #: Array[[Integer, Integer]]
      assert_equal [[0, 1], [1, 1]], coords.uniq
      assert_equal "{[0, 1] => 2, [1, 1] => 1}", (coords.tally).inspect
      assert_equal true, (coords.include?([1, 1]))
      assert_equal [[0, 1], [0, 1], [1, 1]], coords.sort
      assert_equal [1, 1], coords.max
      assert_equal ["c", "c", "-"], (coords.map { |cx, cy| cells[[cx, cy]] || "-" })
      trios = [[1, "b", :x], [1, "a", :y], [0, "z", :z]] #: Array[[Integer, String, Symbol]]
      assert_equal [[0, "z", :z], [1, "a", :y], [1, "b", :x]], trios.sort
      assert_equal [0, "z", :z], trios.min
      assert_equal [0, "z", :z], (trios.max_by { |_n, s, _y| s })
      out = [] #: Array[String]
      trios.each { |tn, ts, ty| out << "#{tn}#{ts}#{ty}" }
      assert_equal ["1bx", "1ay", "0zz"], out
      assert_equal [[:x, 1], [:y, 1], [:z, 0]], (trios.map { |tn, _ts, ty| [ty, tn] })
      assert_equal [0, "z", :z], trios.last
      out = []
      pairs.each { out << _1.inspect }
      assert_equal ["[3, \"c\"]", "[1, \"a\"]", "[2, \"b\"]", "[0, \"z\"]", "[9, \"i\"]"], out
      out = []
      pairs.each { out << "#{_2}#{_1}" }
      assert_equal ["c3", "a1", "b2", "z0", "i9"], out
      out = []
      pairs.each { out << it.inspect }
      assert_equal ["[3, \"c\"]", "[1, \"a\"]", "[2, \"b\"]", "[0, \"z\"]", "[9, \"i\"]"], out
      assert_equal ["ccc", "a", "bb", "", "iiiiiiiii"], (pairs.map { _2 * _1 })
      assert_equal ["c", "a", "b", "z", "i"], (pairs.map { it.last })
      assert_equal [[3, "c"], [2, "b"], [9, "i"]], (pairs.select { _1.first > 1 })
    end
  end

  class ArrayUntypedTest < Minitest::Test
    def test_section_0
      bag = []
      bag << 3
      bag << "x"
      bag << nil
      bag << :sym
      bag << 2.5
      bag << [1, "y"]
      assert_equal "[3, \"x\", nil, :sym, 2.5, [1, \"y\"]]", (bag).inspect
      assert_equal "[3, \"x\", nil, :sym, 2.5, [1, \"y\"]]", bag.to_s
      assert_equal 6, bag.size
      assert_equal false, bag.empty?
      assert_equal 3, bag[0]
      assert_nil bag[2]
      assert_equal [1, "y"], bag[-1]
      assert_nil bag[99]
      assert_equal [1, "y"], bag.last
      assert_equal "3-x--sym-2.5", bag.first(5).join("-")
      assert_equal "[3, \"x\", :sym, 2.5, [1, \"y\"]]", (bag.compact).inspect
      assert_equal 5, bag.compact.size
      assert_equal true, bag.include?(nil)
      assert_equal ["3", "\"x\"", "nil", ":sym", "2.5", "[1, \"y\"]"], (bag.map { |e| e.inspect })
      assert_equal "3|x||sym|2.5|[1, \"y\"]", (bag.map { |e| e.to_s }.join("|"))
      assert_equal 1, (bag.select { |e| e.nil? }.size)
      assert_equal 5, (bag.reject { |e| e.nil? }.size)
      assert_equal 6, bag.count
      assert_equal "[[1, \"y\"], 2.5, :sym, nil, \"x\", 3]", (bag.reverse).inspect
      assert_equal [3, "x"], bag.first(2)
      assert_equal "array of 6: int(3) \"x\" nil :sym 2.5 array of 2: int(1) \"y\"", array_describe(bag)
      assert_nil bag.delete(nil)
      assert_equal "[3, \"x\", :sym, 2.5, [1, \"y\"]]", (bag).inspect
      assert_nil bag.delete("zzz")
      assert_equal "x", bag.delete("x")
      assert_equal "[3, :sym, 2.5, [1, \"y\"]]", (bag).inspect
      assert_equal 3, bag.delete_at(0)
      assert_equal [1, "y"], bag.pop
      assert_equal :sym, bag.shift
      assert_equal "[2.5]", (bag).inspect
      untyped_empty = []
      assert_equal [], untyped_empty
      assert_equal 0, untyped_empty.size
      assert_equal [], untyped_empty.first(1)
      assert_nil untyped_empty[0]
      untyped_empty.push("a")
      untyped_empty.unshift(1)
      untyped_empty.concat(["b", 2])
      assert_equal [1, "a", "b", 2], untyped_empty
      assert_equal [1, "a", "b", 2, nil], (untyped_empty + [nil])
      same = []
      same << 3
      same << 3
      same << "3"
      same << 3.0
      assert_equal "[3, \"3\", 3.0]", (same.uniq).inspect
      assert_equal "{3 => 2, \"3\" => 1, 3.0 => 1}", (same.tally).inspect
      assert_equal "3.0", (same.delete(3)).inspect
      assert_equal ["3"], same
      row = [1, "a", :b, 2.5]
      assert_equal "[1, \"a\", :b, 2.5]", (row).inspect
      assert_equal 4, row.size
      assert_equal "a", row[1]
      assert_equal "2.5", (row.last).inspect
      rows = [[1, "a"], [2, "b"]]
      assert_equal [[1, "a"], [2, "b"]], rows
      assert_equal 2, rows.size
      holder = [] #: Array[untyped]
      holder << [1, 2]
      holder << { k: "v" }
      assert_equal "[[1, 2], {k: \"v\"}]", (holder).inspect
      assert_equal "array of 2: array of 2: int(1) int(2) {k: \"v\"}", array_describe(holder)
      mixed_typed = [1, "a", nil, [2, [3]]] #: Array[untyped]
      assert_equal "array of 4: int(1) \"a\" nil array of 2: int(2) array of 1: int(3)", array_describe(mixed_typed)
      dyn = array_ident([3, 1, 2])
      assert_equal 3, dyn.size
      assert_equal 3, dyn[0]
      assert_equal 2, dyn.last
      assert_equal true, dyn.include?(1)
      assert_equal false, dyn.empty?
      assert_equal [3, 1, 2], dyn
      assert_equal 3, dyn.length
      dyn << 4
      assert_equal [3, 1, 2, 4], dyn
      assert_equal "3-1-2-4", dyn.join("-")
      assert_equal true, (dyn == [3, 1, 2, 4])
      assert_equal [4, 2, 1, 3], dyn.reverse
      assert_equal 4, dyn.pop
      assert_equal 3, dyn.shift
      assert_equal [1, 2], dyn
      assert_equal [1, 2], dyn.first(2)
      typed = [0] #: Array[Integer]
      push_two(typed)
      push_two(typed)
      assert_equal [0, 2, 2], typed
      assert_equal "array 3", array_kind(typed)
      assert_equal "array 0", array_kind([])
      assert_equal "int", array_kind(3)
      assert_equal "nil", array_kind(nil)
      assert_equal "other", array_kind("s")
      assert_equal "array 2", (array_kind([[1], [2]]))
      assert_equal "array 2", array_kind(holder)
      mix = []
      mix << 3
      mix << "x"
      mix << nil
      mix << 2.5
      assert_nil (mix.find { |e| e.nil? })
      assert_equal true, (mix.any? { |e| e.nil? })
      assert_equal false, (mix.all? { |e| e.nil? })
      assert_equal false, (mix.none? { |e| e.nil? })
      assert_equal "{false => [3, \"x\", 2.5], true => [nil]}", ((mix.group_by { |e| e.nil? })).inspect
      assert_nil (mix.min_by { |e| e.to_s })
      assert_equal "2.5", ((mix.max_by { |e| e.to_s.size })).inspect
      assert_equal 8, (mix.flat_map { |e| [e, e] }.size)
      assert_equal "[3, \"x\", 2.5]", ((mix.reject { |e| e.nil? })).inspect
      assert_equal 3, (mix.reduce(0) { |a, e| e.nil? ? a : a + 1 })
      out = [] #: Array[String]
      mix.each_with_index { |e, i| out << "#{i}=#{e.inspect}" }
      mix.reverse_each { |e| out << e.inspect }
      assert_equal ["0=3", "1=\"x\"", "2=nil", "3=2.5", "2.5", "nil", "\"x\"", "3"], out
      assert_equal ["3", "x", "", "2.5"], mix.map(&:to_s)
      assert_equal "3,\"x\",nil,2.5", mix.map(&:inspect).join(",")
      assert_equal 4, mix.tally.size
      assert_equal "[nil, 2.5, 3, \"x\"]", ((mix.sort_by { |e| e.to_s })).inspect
    end
  end

  # flatten / to_h / transpose, which need the element type to be an Array or pair (issue #5, decision 92).
  class ArrayNestedTest < Minitest::Test
    def test_flatten
      assert_equal [1, 2, 3], [[1, 2], [3]].flatten
      assert_equal [1, 2, 3], [[1, 2], [3]].flatten(1)
      assert_equal [1, 2, 3], [[[1, 2]], [[3]]].flatten
      assert_equal [[1, 2], [3]], [[[1, 2]], [[3]]].flatten(1)
      assert_equal %w[a b c], [%w[a b], %w[c]].flatten
      assert_equal [1, 2], [1, 2].flatten
      flat = [1, 2]
      refute_same flat, flat.flatten
      assert_equal [1, 2], flat.flatten
    end

    def test_flatten_untyped
      x = [] #: Array[untyped]
      x << 1
      x << [2, [3, [4]]]
      x << [:a, "b"]
      assert_equal [1, 2, 3, 4, :a, "b"], x.flatten
      assert_equal [1, 2, [3, [4]], :a, "b"], x.flatten(1)
      nested = [array_ident([1, [2]]), array_ident([3])] #: Array[Array[untyped]]
      assert_equal [1, 2, 3], nested.flatten
      assert_equal [1, [2], 3], nested.flatten(1)
    end

    def test_to_h
      assert_equal({ a: 1, b: 2 }, [[:a, 1], [:b, 2]].to_h)
      assert_equal({ a: 2 }, [[:a, 1], [:a, 2]].to_h)
      assert_equal({ 1 => 2, 3 => 4 }, [[1, 2], [3, 4]].to_h)
      assert_equal({ "a" => 1, "b" => 2 }, %w[a b].zip([1, 2]).to_h)
      assert_equal({ a: 1 }, { a: 1 }.to_a.to_h)
      assert_equal({ 1 => 1, 2 => 4 }, [1, 2].to_h { |v| [v, v * v] })
      assert_equal({ "1" => 1 }, [1].to_h { |v| [v.to_s, v] })
      e = assert_raises(ArgumentError) { [[1, 2], [3, 4, 5]].to_h }
      assert_equal "wrong array length at 1 (expected 2, was 3)", e.message
    end

    def test_transpose
      assert_equal [[1, 3], [2, 4]], [[1, 2], [3, 4]].transpose
      assert_equal %w[ac bd], [%w[a b], %w[c d]].transpose.map(&:join)
      none = [] #: Array[Array[Integer]]
      assert_equal [], none.transpose
      e = assert_raises(IndexError) { [[1, 2], [3]].transpose }
      assert_equal "element size differs (1 should be 2)", e.message
    end
  end

  # Array#freeze (decision 96).
  class ArrayFreezeTest < Minitest::Test
    FROZEN = [1, 2].freeze

    def test_freeze
      assert_equal true, FROZEN.frozen?
      assert_equal false, [1].frozen?
      assert_equal "can't modify frozen Array: [1, 2]", assert_raises(FrozenError) { FROZEN << 3 }.message
      assert_equal "can't modify frozen Array: [1, 2]", assert_raises(FrozenError) { FROZEN[0] = 9 }.message
      assert_equal "can't modify frozen Array: [1, 2]", assert_raises(FrozenError) { FROZEN.sort! }.message
      assert_equal "can't modify frozen Array: [1, 2]", assert_raises(FrozenError) { FROZEN.shift }.message
      copy = FROZEN.dup
      copy << 3
      assert_equal [[1, 2, 3], false], [copy, copy.frozen?]
      assert_equal [2, 4], FROZEN.map { |x| x * 2 }
      assert_same FROZEN, FROZEN.freeze
    end

    # The flag lives on the object: an equal Array, a copy or a slice stays mutable; an alias is the same object.
    def test_freeze_is_per_object
      a = [1, 2]
      b = [1, 2]
      alias_a = a
      a.freeze
      b << 3
      assert_equal [true, false, true], [a.frozen?, b.frozen?, alias_a.frozen?]
      assert_raises(FrozenError) { alias_a << 3 }
      assert_equal [false, false, false], [a.dup.frozen?, a[0, 1].frozen?, a.map { |x| x }.frozen?]
      h = { k: a }
      assert_equal true, h[:k].frozen?
      assert_equal [1, 2, 3], b
    end
  end

  # ruby/spec core/array and core/enumerable gaps (#49)
  class ArrayRubySpecTest < Minitest::Test
    def test_enumerable_queries
      assert_equal [true, false], [[1, 2, 3].one? { |x| x > 2 }, [1, 2, 3].one? { |x| x > 1 }]
      assert_equal [1, 1, nil], [(1..3).find_index(2), (1..3).find_index { |x| x > 1 }, (1..3).find_index(9)]
      assert_equal ["a", "dd"], %w[a b c dd ee].minmax_by(&:size)
      assert_equal [1, 1, 2, 2], [1, 2].collect_concat { |x| [x, x] }
      assert_equal [2, 4], [1, 2].each.with_object([]) { |x, acc| acc << x * 2 }
    end

    def test_enumerable_cycle
      out = [] #: Array[Integer]
      [1, 2].cycle do |x|
        out << x
        break if out.size > 4
      end
      [3].cycle(2) { |x| out << x }
      [].cycle { |x| out << x }
      assert_equal [1, 2, 1, 2, 1, 3, 3], out
    end

    def test_enumerable_slicing
      assert_equal [[1], [2, 3], [4]], [1, 2, 3, 4].slice_before(&:even?).to_a
      assert_equal [[1, 2], [3, 4]], [1, 2, 3, 4].slice_after(&:even?).to_a
      assert_equal [[true, [1]], [false, [2]], [true, [1, 3]]], [1, 2, 1, 3].chunk(&:odd?).to_a
      assert_equal [[true, [1]], [true, [3]]], [1, 2, 3].chunk { |x| x == 2 ? nil : x.odd? }.to_a
    end

    def test_repeated_combinations
      assert_equal [[1, 1], [1, 2], [2, 2]], [1, 2].repeated_combination(2).to_a
      assert_equal [[1, 1], [1, 2], [2, 1], [2, 2]], [1, 2].repeated_permutation(2).to_a
      assert_equal [[]], [1].repeated_combination(0).to_a
      assert_equal [], [1].repeated_permutation(-1).to_a
    end

    def test_array_access
      a = [1, 3, 5, 7]
      assert_equal [7, nil], [a.at(-1), a.at(9)]
      assert_equal [1, 5], a.fetch_values(0, 2)
      assert_raises(IndexError) { a.fetch_values(9) }
      assert_equal [1, nil], [a.bsearch_index { |x| x >= 3 }, a.bsearch_index { |x| x > 9 }]
      assert_equal [7, nil], [a.rfind(&:odd?), a.rfind(&:even?)]
      b = [1]
      assert_same b, b.replace([2, 3])
      assert_equal [2, 3], b
    end
  end

  # ruby/spec language gaps (#49): splat and nested multiple assignment, |a, *rest|
  class ArrayRubySpecDestructureTest < Minitest::Test
    #: () -> [Integer, String, Float]
    def array_triple = [1, "s", 2.5]

    def test_multiple_assignment
      a, *b = [1, 2, 3]
      *c, d = [1, 2, 3]
      e, *f, g = [1, 2, 3, 4]
      h, *i, j = [1]
      assert_equal [1, [2, 3], [1, 2], 3, 1, [2, 3], 4, 1, [], nil], [a, b, c, d, e, f, g, h, i, j]
      k, (l, m) = 1, [2, 3]
      n, = [7, 8]
      o, * = [5, 6]
      q, *r = array_triple
      s1, (s2, s3), s4 = 1, [2, 3], 4
      assert_equal [1, 2, 3, 7, 5, 1, ["s", 2.5], [1, 2, 3, 4]], [k, l, m, n, o, q, r, [s1, s2, s3, s4]]
    end

    def test_block_rest_params
      seen = [] #: Array[untyped]
      [[1, 2, 3], [4]].each { |a, *r| seen << [a, r] }
      tuples = [[1, "a", 2.5]] #: Array[[Integer, String, Float]]
      tuples.each { |a, *r| seen << [a, r] }
      { x: 1 }.each { |k, *v| seen << [k, v] }
      [1, 2].each_with_index { |*all| seen << all }
      [[1, 2]].each { |*all| seen << all }
      [[1, 2, 3]].each { |a, *| seen << a }
      assert_equal [[1, [2, 3]], [4, []], [1, ["a", 2.5]], [:x, [1]], [1, 0], [2, 1], [[1, 2]], 1], seen
      assert_equal [3, 5], [[1, 2, 3], [4, 5]].map { |first, *rest| rest.size + (first || 0) }
    end
  end

  # ruby/spec core/array gaps (#49): assoc and rassoc
  class ArrayRubySpecAssocTest < Minitest::Test
    def test_assoc
      pairs = [[1, "one"], [2, "two"]] #: Array[[Integer, String]]
      assert_equal [[2, "two"], [1, "one"]], [pairs.assoc(2), pairs.rassoc("one")]
      assert_nil pairs.assoc(9)
      mixed = [[:a, 1], [:b], [:c, 3, 4]] #: Array[Array[untyped]]
      assert_equal [[:c, 3, 4], [:a, 1]], [mixed.assoc(:c), mixed.rassoc(1)]
      assert_nil mixed.rassoc(nil)
    end
  end

  # ruby/spec language gaps (#49): splats inside array literals
  class ArrayRubySpecSplatLiteralTest < Minitest::Test
    def test_splat_literal
      a = [1, 2]
      b = [3]
      assert_equal [[1, 2, 3], [0, 1, 2, 9], [1, 2], [5]], [[*a, *b], [0, *a, 9], [*a], [*[], 5]]
      words = %w[x y]
      assert_equal ["a", "x", "y"], ["a", *words]
    end
  end

  # a tuple (divmod's [q, r]) had no Kernel methods: undefined method for tuple
  class ArrayBugTupleKernelTest < Minitest::Test
    def test_kernel_methods_on_tuple
      t = 7.divmod(2)
      assert_equal "<Array>", t.array_tag
      assert_equal false, t.frozen?
      assert_equal 4, t.then { |q, r| q + r }
    end
  end

  # an unannotated def whose [] literal is typed on a refinement round joined the untyped first run's return (#101)
  class ArrayRetEvidenceTest < Minitest::Test
    def ret_one
      r = []
      r << 1
      r
    end

    def ret_early(n)
      r = []
      r << "a"
      return r if n > 5

      r.push("b")
      r
    end

    def ret_hash(n)
      h = {}
      h[:a] = n
      return h if n > 1

      h
    end

    def test_refined_literal_return
      assert_equal [1], ret_one
      assert_equal [["a"], ["a", "b"], { a: 2 }], [ret_early(9), ret_early(1), ret_hash(2)]
    end
  end

  # tupleCall ignored the count and returned the scalar field (#101)
  class ArrayTupleFirstLastNTest < Minitest::Test
    def test_first_last_with_count
      t = [1, "a", :b]
      assert_equal [[1, "a"], ["a", :b], 1, :b, [], [1, "a", :b]], [t.first(2), t.last(2), t.first, t.last, t.first(0), t.last(5)]
      assert_equal [0, true], [t <=> [1, "a", :b], t == [1, "a", :b]]
    end
  end

  # Array#*: repeat by an Integer, join by a String (also through an untyped value)
  class ArrayMulTest < Minitest::Test
    def test_mul
      assert_equal [[1, 2, 1, 2], [], "1,2", []], [[1, 2] * 2, [] * 3, [1, 2] * ",", ["a"] * 0]
      e = assert_raises(ArgumentError) { [1] * -1 }
      assert_equal "negative argument", e.message
      x = [3] #: untyped
      assert_equal [[3, 3], "3"], [x * 2, x * "-"]
    end
  end

  class ArrayPushManyTest < Minitest::Test
    def test_push_and_append_take_many
      a = [1]
      a.push(2, 3)
      a.push(*[4, 5])
      a.append(6, *[7])
      a.push
      assert_equal [1, 2, 3, 4, 5, 6, 7], a
    end

    def test_flatten_pairs
      assert_equal [:a, 1, :b, 2], [[:a, 1], [:b, 2]].flatten
    end
  end

  class ArrayCompactOptTest < Minitest::Test
    #: (Array[Integer]) -> Integer
    def array_total(xs) = xs.sum

    def test_compact_drops_nil_from_the_type
      a = [1, nil, 3] #: Array[Integer?]
      assert_equal 4, array_total(a.compact)
      assert_equal [1, 2], [1, 2].compact
      u = [1, nil, "x"] #: Array[untyped]
      assert_equal [1, "x"], u.compact
    end

    def test_fill_spans_and_blocks
      assert_equal [[1, 0, 0], [0, 0, 0], [1, 0, 3], [1, 2, 0, 0, 0, 0], [0, 0, 3], [0, 0, 3], [1, 2, 3]],
                   [[1, 2, 3].fill(0, -2), [1, 2, 3].fill(0, -9), [1, 2, 3].fill(0, 1, 1), [1, 2, 3].fill(0, 2, 4),
                    [1, 2, 3].fill(0, nil, 2), [1, 2, 3].fill(0, 0...2), [1, 2, 3].fill(0, 2..0)]
      assert_equal [[0, 1, 4], [1, 10, 20], [-0, -1, 3], [1, 101, 3]],
                   [[1, 2, 3].fill { |i| i * i }, [1, 2, 3].fill(1) { |i| i * 10 }, [1, 2, 3].fill(0, 2) { |i| -i },
                    [1, 2, 3].fill(1..1) { |i| i + 100 }]
      e = assert_raises(RangeError) { [1, 2, 3].fill(0, -5..1) }
      assert_equal "-5..1 out of range", e.message
    end

    def test_core_methods_truncate_float_arguments
      assert_equal [[1], "abab", 2, "b"], [[1, 2, 3].first(1.9), "ab" * 2.5, [1, 2, 3][1.7], "abc"[1.2]]
      e = assert_raises(RangeError) { [1].first(Float::NAN) }
      assert_equal "float NaN out of range of integer", e.message
    end

    def test_literal_widened_by_later_writes
      ary = [1, 2, 3, 4, 5, 6]
      ary.fill("x", 2, 3)
      a = [1, 2]
      a << "s"
      b = [1, 2]
      b << 3
      assert_equal [[1, 2, "x", "x", "x", 6], [1, 2, "s"], 6], [ary, a, b.sum]
    end

    def test_index_assign_span
      a = [1, 2, 3, 4]
      a[1, 2] = [7, 8, 9]
      b = [1, 2, 3]
      b[1..] = []
      c = [1, 2, 3]
      c[0...1] = [5, 6]
      d = [1, 2, 3]
      d[-2..] = 0
      assert_equal [[1, 7, 8, 9, 4], [1], [5, 6, 2, 3], [1, 0]], [a, b, c, d]
      e = assert_raises(IndexError) { d[1, -1] = 1 }
      assert_equal "negative length (-1)", e.message
    end
  end

  # any?/all?/none?/one? without a block test the elements themselves, or `pattern === element` (decision 219).
  class ArrayBlocklessPredicateTest < Minitest::Test
    def test_truthiness
      xs = [1, nil]
      assert_equal [true, false, false, false, false, true, true], [xs.any?, [nil].any?, [].any?, xs.all?, xs.none?, xs.one?, [nil, false].none?]
    end

    def test_pattern
      xs = [1, nil]
      assert_equal [true, false, true, true, false], [xs.any?(Integer), xs.all?(Integer), xs.none?(String), xs.one?(Integer), [1, 2].one?(1..5)]
    end

    def test_hash
      h = { a: 1 }
      e = {} #: Hash[Symbol, Integer]
      assert_equal [true, false, true, false], [h.any?, e.any?, h.all?, h.none?]
    end
  end

  # flatten! and MatchData#to_a (decision 232).
  class ArrayFlattenBangTest < Minitest::Test
    def test_flatten_bang
      xs = [1, [2, [3]]] #: Array[untyped]
      assert_equal [[1, 2, 3], [1, 2, 3], nil], [xs.flatten!, xs, [1, 2].flatten!]
      assert_equal ["12", "1", "2"], "ab12".match(/(\d)(\d)/).to_a
    end

    def test_reduce_nil_accumulator_in_a_method
      none = [] #: Array[Integer]
      assert_equal [1, nil], [array_first_set([1, 2]), array_first_set(none)]
    end

    def test_unshift_and_prepend_take_many
      a = ["x"]
      a.unshift("a", "b")
      a.prepend("--t", "c")
      assert_equal ["--t", "c", "a", "b", "x"], a
    end
  end

  # Array#<=> is -1/0/1, never the length difference.
  class ArrayCmpSignTest < Minitest::Test
    def test_cmp_is_a_sign
      a = [] #: Array[Integer]
      b = [1] #: Array[Integer]
      c = [1, 2] #: Array[Integer]
      d = [1, 2, 3] #: Array[Integer]
      e = [2] #: Array[Integer]
      got = [a <=> a, a <=> b, b <=> a, d <=> b, b <=> d, d <=> a, a <=> d, c <=> d, d <=> c, c <=> e, e <=> d, d <=> d]
      assert_equal [0, -1, 1, 1, -1, 1, -1, -1, 1, -1, 1, 0], got
    end
  end

  # `Array.new(n, nil)` is typed by what is stored over its nils, as `[]` is (decision 182), for a local and an ivar.
  class ArrayNewNilSlots
    def initialize(n)
      @slots = ::Array.new(n, nil)
    end

    def put(i, s)
      @slots[i] = s
    end

    def lengths = @slots.map { |s| s&.length }
  end

  class ArrayNewNilTest < Minitest::Test
    def test_local_and_ivar_take_the_stored_type
      xs = Array.new(3, nil)
      xs[1] = "ab"
      slots = ArrayNewNilSlots.new(2)
      slots.put(1, "xyz")
      assert_equal [[nil, "ab", nil], [nil, 3]], [xs, slots.lengths]
    end
  end

  # #102: frozen checks and no-op mutators, as MRI's
  class ArrayRubySpec102Test < Minitest::Test
    def test_frozen_shuffle_and_noop_delete
      a = [1, 2].freeze
      got = begin; a.shuffle!; :none; rescue FrozenError => e; e.class; end
      assert_equal [FrozenError, nil, []], [got, a.delete(5), [].insert(10)]
    end

    def test_chunk_reserved_keys
      sep = [1, 2, 3].chunk { |x| x == 2 ? :_separator : true }.to_a
      alone = [1, 1, 2].chunk { |x| x == 1 ? :_alone : true }.to_a
      msg = begin; [1].chunk { :_foo }.to_a; rescue RuntimeError => e; e.message; end
      assert_equal [[[true, [1]], [true, [3]]], [[:_alone, [1]], [:_alone, [1]], [true, [2]]], "symbols beginning with an underscore are reserved"], [sep, alone, msg]
    end
  end
end

module ArrayTests
  class ArraySliceBangTest < Minitest::Test
    def test_slice_bang_range
      a = [1, 2, 3, 4, 5]
      assert_equal [2, 3], a.slice!(1...3)
      assert_equal [1, 4, 5], a
      assert_equal [4, 5], a.slice!(-2..)
      assert_equal [1], a
      assert_nil a.slice!(5..6)
      assert_equal [], a.slice!(0...0)
      assert_equal [1], a
    end
  end

  # Sizes a program controls are checked before anything is allocated (#109, decision 273): a Go out-of-memory is fatal and `rescue` cannot catch it. 2**62 is past MRI's ARY_MAX_SIZE, so MRI refuses up front too.
  class ArrayHugeTest < Minitest::Test
    HUGE = 2**62

    #: () { () -> untyped } -> Class?
    def self.raised_class
      yield
      nil
    rescue StandardError => e
      e.class
    end

    def test_index_set_far_is_index_error
      a = [1]
      e = assert_raises(IndexError) { a[HUGE] = 2 }
      assert_equal "index #{HUGE} too big", e.message
      assert_equal [1], a
    end

    def test_index_set_start_len_far_is_index_error
      a = [1]
      e = assert_raises(IndexError) { a[HUGE, 1] = 2 }
      assert_equal "index #{HUGE} too big", e.message
      assert_equal [1], a
    end

    def test_range_set_far_is_index_error
      a = [1]
      e = assert_raises(IndexError) { a[HUGE..(HUGE + 1)] = 2 }
      assert_equal "index #{HUGE} too big", e.message
      assert_equal [1], a
    end

    def test_insert_far_is_index_error
      a = [1]
      e = assert_raises(IndexError) { a.insert(HUGE, 2) }
      assert_equal "index #{HUGE} too big", e.message
      assert_equal [1], a
    end

    def test_padding_still_works
      a = ["a"]
      a[3] = "d"
      assert_equal 4, a.size
      b = ["a"]
      b.insert(2, "c")
      assert_equal 3, b.size
      c = ["a"]
      c[2, 1] = "e"
      assert_equal 3, c.size
    end

    def test_repeated_huge_count_is_argument_error
      assert_equal ArgumentError, ArrayHugeTest.raised_class { [1, 2].repeated_combination(HUGE).to_a } # message differs by design (decision 20)
      assert_equal ArgumentError, ArrayHugeTest.raised_class { [1, 2].repeated_permutation(HUGE).to_a }
      assert_equal [[1, 1], [1, 2], [2, 2]], [1, 2].repeated_combination(2).to_a
      assert_equal 4, [1, 2].repeated_permutation(2).to_a.size
    end

    def test_flatten_recursive_is_argument_error
      a = [1] #: Array[untyped]
      a << a
      e = assert_raises(ArgumentError) { a.flatten }
      assert_equal "tried to flatten recursive array", e.message
      assert_raises(ArgumentError) { a.flatten! }
      b = [a, 2] #: Array[untyped]
      assert_raises(ArgumentError) { b.flatten }
    end

    def test_flatten_shared_is_not_recursive
      shared = [1, [2]] #: Array[untyped]
      assert_equal [1, 2, 1, 2], [shared, shared].flatten
    end

    # MRI clamps to the shrunken length after each yield instead of indexing past it
    def test_reverse_each_survives_the_block_shrinking_the_array
      a = [1, 2, 3, 4, 5]
      seen = [] #: Array[Integer]
      a.reverse_each do |x|
        seen << x
        a.clear if x == 4
      end
      assert_equal [5, 4], seen
      b = [1, 2, 3, 4, 5]
      seen = []
      b.reverse_each do |x|
        seen << x
        b.delete_at(0) if x == 5
      end
      assert_equal [5, 5, 5, 5, 5], seen
      assert_empty b
    end
  end
end

module ArrayTests
  # `a[*xs]` and `a[*xs] = v`: a switch on xs's length over the one- and two-index forms (decision 363)
  class ArrayIndexSplatTest < Minitest::Test
    def test_index_splat
      a = ("a".."j").to_a
      args = [1, 3]
      a[*args] = "X"
      assert_equal ["a", "X", "e", "f", "g", "h", "i", "j"], a
      assert_equal ["X", "e", "f"], a[*args]
      one = [0]
      a[*one] = "Z"
      assert_equal "Z", a[*one]
      h = { 1 => 2 }
      k = [5]
      h[*k] = 6
      assert_equal({ 1 => 2, 5 => 6 }, h)
      begin
        a[*[1, 2, 3]] = "q"
        flunk "no error"
      rescue ArgumentError => e
        assert_kind_of ArgumentError, e
      end
    end
  end
end

module ArrayTests
  class ArrayPopShiftNTest < Minitest::Test
    def test_pop_and_shift_n
      a = [1, 2, 3]
      assert_equal [[2, 3], [1]], [a.pop(2), a]
      b = [1, 2, 3]
      assert_equal [[1, 2], [3]], [b.shift(2), b]
      assert_equal [[1], [], [], []], [[1].pop(5), [1].shift(0), [1].pop(0), [].shift(3)]
      assert_equal ArgumentError, (begin; [1].pop(-1); rescue ArgumentError => e; e.class; end)
      assert_equal FrozenError, (begin; [1].freeze.shift(1); rescue FrozenError => e; e.class; end)
      assert_equal FrozenError, (begin; [].freeze.pop(0); rescue FrozenError => e; e.class; end)
    end
  end
end

module ArrayTests
  # decision 366: a mixed literal used as an Array is an Array of the union; a tuple iterates as one
  class ArrayMixedLiteralTest < Minitest::Test
    def test_mixed_literal_widens
      a = [1, "two"]
      a << 3.0
      assert_equal [1, "two", 3.0], a
      got = [] #: Array[String]
      [1.0, 42].each { |n| got << n.inspect }
      assert_equal ["1.0", "42"], got
      c = [1, "two", 3.0]
      2.times { c << c }
      assert_equal 5, c.size
    end
  end

  class ArrayMinMaxByCountTest < Minitest::Test
    def test_max_by_and_min_by_with_a_count
      words = %w[aaaa b ccc dd]
      assert_equal ["aaaa", "ccc"], words.max_by(2, &:size)
      assert_equal ["b", "dd"], words.min_by(2) { it.size }
      assert_equal [5, 4, 3], (1..5).max_by(3) { it }
      assert_equal [3, 1], [3, 1].max_by(5) { it }
      assert_equal [], [1, 2].min_by(0) { it }
    end
  end

  # Array#[]= stores in place when the index is in bounds and the Array not frozen, else the full path (decision 387).
  class ArrayIndexStoreTest < Minitest::Test
    def test_store_paths
      a = [1, 2, 3]
      a[0] = 10
      a[-1] = 30
      a[5] = 6
      assert_equal [6, 10, 30, 6], [a.size, a[0].to_i, a[2].to_i, a[5].to_i]
      f = [1.5, 2.5]
      f[1] = 4
      assert_equal [1.5, 4.0], f
      o = [1, nil] #: Array[Integer?]
      o[0] = nil
      assert_nil o[0]
      frozen = [1, 2].freeze
      err = begin; frozen[0] = 5; ""; rescue FrozenError => e; e.message; end
      assert_equal "can't modify frozen Array: [1, 2]", err
      low = begin; a[-99] = 1; ""; rescue IndexError => e; e.message; end
      assert_equal "index -99 too small for array; minimum: -6", low
    end

    def test_optional_element_compare
      flags = [1, 0, 1]
      assert(flags[0] == 1)
      refute(flags[1] == 1)
      refute(flags[9] == 1)
      assert_equal 2, (0..3).count { |i| flags[i] == 1 }
      floats = [0.5]
      assert(floats[0] == 0.5)
      refute(floats[1] == 0.5)
    end

    def test_integer_array_equality
      assert([1, 2, 3] == [1, 2, 3])
      refute([1, 2, 3] == [1, 2, 4])
      refute([1, 2] == [1, 2, 3])
      assert([1, 2] == [1.0, 2.0])
      li = (1..1000).to_a
      assert(li == li.reverse.reverse)
    end
  end
end

# Arrays used as stacks of tuples (decision 384).
module ArrayTests
  class StackNode
    attr_reader :name, :kids

    def initialize(name, kids = [])
      @name = name
      @kids = kids
    end
  end

  class StackParser
    def initialize
      @stack = []
      @tree = StackNode.new("root")
      @depth = 0
    end

    def enter(el)
      @stack.push([@tree, @depth])
      @tree = el
      @depth += 1
    end

    def leave
      top = @stack.pop
      @tree, @depth = top if top
    end

    def path = @stack.map { |t, d| "#{t.name}:#{d}" } + ["#{@tree.name}:#{@depth}"]
  end

  class ArrayTupleStackTest < Minitest::Test
    def stack_tree
      StackNode.new("a", [StackNode.new("b", [StackNode.new("d")]), StackNode.new("c")])
    end

    def test_local_stack_of_mixed_tuples
      stack = []
      stack << [stack_tree, :open]
      out = []
      until stack.empty?
        node, state = stack.pop
        out << "#{state}:#{node.name}"
        stack << [node, :close] if state == :open && node.kids.empty? && node.name == "never"
        node.kids.reverse_each { |k| stack << [k, :open] }
      end
      assert_equal ["open:a", "open:b", "open:d", "open:c"], out
    end

    def test_seeded_stack_of_pairs
      stack = [[3, 0]]
      out = []
      until stack.empty?
        k, d = stack.pop
        out << "#{k}@#{d}"
        stack.push([k - 1, d + 1]) if k > 0
      end
      assert_equal ["3@0", "2@1", "1@2", "0@3"], out
    end

    def test_seeded_stack_of_triples
      stack = [[stack_tree, "", 0]]
      out = []
      while (top = stack.pop)
        node, path, depth = top
        out << "#{path}/#{node.name}:#{depth}"
        node.kids.each { |k| stack.push([k, path + "/" + node.name, depth + 1]) }
      end
      assert_equal ["/a:0", "/a/c:1", "/a/b:1", "/a/b/d:2"], out
      t, s = [[stack_tree, "x"]].last
      assert_equal ["a", 1], [t.name, s.size]
    end

    def test_ivar_stack_of_tuples
      pr = StackParser.new
      pr.enter(StackNode.new("a"))
      pr.enter(StackNode.new("b"))
      assert_equal ["root:0", "a:1", "b:2"], pr.path
      pr.leave
      assert_equal ["root:0", "a:1"], pr.path
      pr.leave
      pr.leave
      assert_equal ["root:0"], pr.path
    end
  end

  # A shift/pop an emptiness check guards is the element, not E? (decision 391).
  class ArraySureTakeTest < Minitest::Test
    def sure_drain(a, out)
      a.clear
      out
    end

    def test_guarded_takes_fill_a_typed_array
      src = (1..5).to_a
      dst = Array.new
      while (not src.empty?)
        dst.push(src.shift)
      end
      back = []
      until dst.empty?
        back << dst.pop
      end
      assert_equal [5, 4, 3, 2, 1], back
      assert_equal 15, back.sum
      words = %w[a b c]
      out = []
      out.push(words.pop) while !words.empty?
      assert_equal "cba", out.join
    end

    def test_if_unless_and_else_guards
      a = [1, 2, 3]
      out = []
      out << a.shift unless a.empty?
      if !a.empty?
        out << a.pop
      end
      if a.empty?
        out << 0
      else
        out << a.shift
      end
      if a.empty? then out << -1 else out << a.shift end
      assert_equal [1, 3, 2, -1], out
    end

    def test_destructured_pop
      stack = [[1, "a"], [2, "b"]]
      out = []
      until stack.empty?
        n, s = stack.pop
        out << "#{s}#{n + 1}"
      end
      assert_equal ["b3", "a2"], out
    end

    def test_frozen_receiver_raises
      a = [1, 2].freeze
      err = assert_raises(FrozenError) do
        until a.empty?
          a.shift
        end
      end
      assert_equal "can't modify frozen Array: [1, 2]", err.message
    end

    def test_second_take_in_one_iteration_may_be_nil
      a = [1, 2, 3]
      out = []
      until a.empty?
        x = a.shift
        y = a.shift
        out << [x, y]
      end
      assert_equal [[1, 2], [3, nil]], out
    end

    def test_redo_keeps_the_nil_check
      a = [1]
      out = []
      tries = 0
      while !a.empty?
        out << a.shift
        tries += 1
        redo if tries == 1
      end
      assert_equal [1, nil], out
    end

    def test_call_before_take_keeps_the_nil_check
      a = [1, 2]
      out = []
      until a.empty?
        sure_drain(a, out).push(a.shift)
      end
      assert_equal [nil], out
    end

    def test_optional_elements_stay_optional
      a = [1, nil, 3]
      out = []
      until a.empty?
        out << a.shift
      end
      assert_equal [1, nil, 3], out
    end
  end
end

module ArrayTests
  # #133: an element read out of an Array is a copy; a later write to the Array does not change it (decision 396).
  class ArrayElementCopyTest < Minitest::Test
    def test_read_survives_pop_and_push
      a = [1, 2, 3]
      x = a[-1]
      a.pop
      a.push(5)
      assert_equal 3, x
    end

    def test_hash_value_survives_store
      a = [1, 2, 3]
      h = {}
      h[:k] = a[0]
      a[0] = 9
      assert_equal 1, h[:k]
    end

    def test_last_survives_store
      a = [1, 2, 3]
      z = a.last
      a[-1] = 8
      assert_equal 3, z
    end

    def test_swap
      a = [1, 2, 3]
      t = a[0]
      a[0] = a[2]
      a[2] = t
      assert_equal [3, 2, 1], a
    end

    def test_float_read_survives_store
      a = [1.5, 2.5]
      f = a[1]
      a[1] = 0.0
      assert_equal 2.5, f
    end
  end

  # #138: an element read of a Symbol, Boolean or Array element is a copy too; Array#[] of a String keeps the slot
  # (decision 397, testdata/run/array_bug_string_slot.rb).
  class ArrayValueElementCopyTest < Minitest::Test
    def test_symbol_last_survives_store
      b = [:k]
      t = b.last
      b[0] = :j
      assert_equal :k, t
    end

    def test_boolean_read_survives_store
      c = [true, false]
      u = c[0]
      c[0] = false
      assert_equal true, u
    end

    def test_array_element_survives_store
      d = [[1], [2]]
      v = d[0]
      d[0] = [9]
      assert_equal [1], v
    end

    def test_hash_string_value_survives_store
      h = { a: "x" }
      w = h[:a]
      h[:a] = "y"
      assert_equal "x", w
    end

    def test_hash_default_survives_store
      h = Hash.new("dx") #: Hash[Symbol, String]
      d = h.default
      h.default = "dy"
      assert_equal "dx", d
    end

    def test_string_swap
      a = ["a", "b", "c"]
      t = a[0]
      a[0] = a[2]
      a[2] = t
      assert_equal ["c", "b", "a"], a
    end

    def test_symbol_swap
      a = [:a, :b]
      t = a[0]
      a[0] = a[1]
      a[1] = t
      assert_equal [:b, :a], a
    end

    def test_rfind_bsearch_assoc_survive_store
      f = ["a", "b", "c"]
      r = f.rfind { |q| q < "c" }
      b = f.bsearch { |q| q >= "b" }
      f[1] = "z"
      assert_equal "b", r
      assert_equal "b", b
      ys = [["k", "v"]]
      as = ys.assoc("k")
      ys[0] = ["k2", "v2"]
      assert_equal ["k", "v"], as
    end

    def test_multiple_assignment_survives_store
      m = [:p, :q]
      x1, x2 = m
      m[0] = :z
      m[1] = :z
      assert_equal :p, x1
      assert_equal :q, x2
    end

    def test_string_hash_key_survives_store
      a = ["x", "w"]
      h = {}
      k = a[0]
      h[k] = 1
      a[0] = "z"
      assert_equal({ "x" => 1 }, h)
      assert_equal 1, h["x"]
      assert_nil h["z"]
      h[a[1]] = 2
      h.delete("x")
      assert_equal({ "w" => 2 }, h)
    end
  end
end

module ArrayTests
  # count/sum/Array.new with a literal block run as index loops at the call (decision 402): same values, length re-read each step.
  class ArrayBlockLoopTest < Minitest::Test
    OFFSETS = [-1, 0, 1].freeze

    #: (Array[Integer], Integer, Integer) -> Integer
    def around(cells, x, y)
      OFFSETS.sum { |dy| OFFSETS.count { |dx| cells[(y + dy) % 3 * 3 + (x + dx) % 3] == 1 } }
    end

    def test_count
      none = [] #: Array[Integer]
      assert_equal 0, none.count { |x| x > 0 }
      assert_equal 2, [1, 2, 3, 4].count { |x| x.even? }
      odd = [1, 2, 3, 4].count do |x|
        next true if x.even?
        next if x == 3

        x > 0
      end
      assert_equal 3, odd
      assert_equal 2, [5, 6, 7].count { _1 > 5 }
      assert_equal 1, [5, 6, 7].count { it == 7 }
      total = [1, 2, 3].count { true }
      assert_equal 3, total
    end

    def test_count_mutating_receiver
      a = [1, 2, 3]
      popped = a.count do
        a.pop
        true
      end
      assert_equal [2, [1]], [popped, a]
      b = [1, 2]
      n = b.count do |x|
        b << x + 10 if x < 3
        true
      end
      assert_equal [4, [1, 2, 11, 12]], [n, b]
      c = [1, 2, 3]
      cleared = c.count do
        c.clear
        true
      end
      assert_equal 1, cleared
    end

    def test_sum_integer
      none = [] #: Array[Integer]
      assert_equal 0, none.sum { |x| x * 2 }
      assert_equal 12, [1, 2, 3].sum { |x| x * 2 }
      skipped = [1, 2, 3].sum do |x|
        next 10 if x == 2

        x
      end
      assert_equal 14, skipped
      a = [1, 2, 3]
      shrunk = a.sum do |x|
        a.pop
        x
      end
      assert_equal 3, shrunk
      assert_equal 5, [[1, 2], [3]].sum { |r| r.sum { |x| x + 0 } } - 1
    end

    def test_sum_float_compensated
      assert_equal 1.0, ([0.1] * 10).sum { |x| x }
      assert_equal 1.0, [1e100, 1.0, -1e100].sum { |x| x }
      assert_equal 0.6, [0.1, 0.2, 0.3].sum { |x| x + 0.0 }
      assert_equal [3.0, 1.5], [[1.0, 2.0].sum { |x| x }, [3].sum { |x| x / 2.0 }]
      assert_equal Float::INFINITY, [1.0, Float::INFINITY].sum { |x| x }
      assert_predicate [Float::INFINITY, -Float::INFINITY].sum { |x| x }, :nan?
      tenth = [0.1, 0.2].sum { |x| x }
      assert_equal 0.30000000000000004, tenth
    end

    def test_break_out_of_statement_call
      n = 0
      [1, 2, 3].count do |x|
        n += x
        break if x == 2

        true
      end
      assert_equal 3, n
      seen = [] #: Array[Integer]
      Array.new(4) do |i|
        seen << i
        break if i == 1

        i
      end
      assert_equal [0, 1], seen
    end

    def test_inner_loops_keep_their_own_jumps
      v = [3, 4].sum do |x|
        t = 0
        x.times do |i|
          next if i.odd?
          break if i > 2

          t += i
        end
        t
      end
      assert_equal 4, v
    end

    def test_array_new
      assert_equal [0, 1, 4, 9], Array.new(4) { |i| i * i }
      none = Array.new(0) { 1 }
      assert_equal [], none
      marked = Array.new(3) do |i|
        next -1 if i == 1

        i
      end
      assert_equal [0, -1, 2], marked
      assert_equal [[0, 1], [0, 1]], Array.new(2) { Array.new(2) { |j| j } }
      assert_equal ["a0", "a1"], Array.new(2) { |i| "a#{i}" }
      assert_equal [0.5, 1.5], Array.new(2) { |i| i + 0.5 }
      procs = Array.new(3) { |i| -> { i * 10 } }
      assert_equal [0, 10, 20], procs.map(&:call)
      e = assert_raises(ArgumentError) { Array.new(-1) { 0 } }
      assert_equal "negative array size", e.message
    end

    def test_rescue_in_block
      rescued = [1, 2].sum do |x|
        Integer("z#{x}")
      rescue ArgumentError
        x
      end
      assert_equal 3, rescued
    end

    def test_neighbours
      cells = [0, 1, 0, 1, 1, 0, 0, 0, 1]
      assert_equal [4, 4, 4], [0, 1, 2].map { |x| around(cells, x, 1) }
    end
  end
end

# A tuple where an Array is expected, and an untyped + of mixed classes (decision 416).
module ArrayTests
  class TupleArrayTest < Minitest::Test
    def test_tuple_into_array
      xs = [1] #: Array[Integer]
      assert_equal([1, 3, 1], xs + 7.divmod(2))
    end

    def test_untyped_plus_mixed
      ys = ["a"].map(&:to_s) #: untyped
      assert_equal(["a", 3, 1], ys + 7.divmod(2))
    end
  end
end
