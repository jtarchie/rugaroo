# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were testdata/run/hash_args.rb.
# Evaluation-order recorders (decisions 98, 241): each logs when it runs.
#: (Array[Symbol], Symbol, Integer) -> Integer
def hash_note(log, tag, v)
  log << tag
  v
end

#: (Array[Symbol], Symbol, String) -> String
def hash_snote(log, tag, v)
  log << tag
  v
end

#: (Array[Symbol], Symbol, ^(Integer) -> Integer) -> ^(Integer) -> Integer
def hash_proc_note(log, tag, pr)
  log << tag
  pr
end

#: (Hash[Symbol, Integer]) -> String
def hash_show(opts) = opts.inspect

#: (String, ?Hash[Symbol, String]) -> String
def tag(name, attrs = {})
  rendered = attrs.map { |key, value| " #{key}=\"#{value}\"" }.join
  "<#{name}#{rendered}>"
end

#: (Hash[String, Integer]) -> Integer
def hash_total(counts) = counts.values.reduce(0) { |sum, n| sum + n }

#: (Hash[Symbol, untyped]) -> String
def loose(o) = o.inspect

#: (Integer, Hash[Symbol, Integer]) -> Integer
def hash_scaled(n, opts) = n * opts.fetch(:by, 1)

#: (untyped) -> String
def any_arg(x) = x.inspect

#: (*untyped) -> String
def rest(*args) = args.inspect

#: (Integer, *untyped) -> String
def rest2(n, *args) = "#{n} #{args.inspect}"

#: (?Hash[Symbol, Integer]?) -> String
def opt_nil(o = nil) = o ? o.inspect : "none"

#: (Hash[untyped, Integer]) -> String
def mixed(o) = o.inspect

#: (Hash[Symbol, Integer]) { (Integer) -> Integer } -> Integer
def with_blk(o) = yield(o.fetch(:n, 0))

# Ruby fills optional positionals left to right, so a braceless hash lands in the first.
#: (Integer, ?Hash[Symbol, Integer], ?Hash[Symbol, Integer]) -> String
def two_opts(n, a = {}, b = {}) = "#{n} #{a.inspect} #{b.inspect}"

#: () { (Hash[Symbol, Integer]) -> void } -> void
def give = yield(a: 1)

#: () { (Hash[Symbol, Integer]) -> String } -> String
def give_v = yield(b: 2, c: 3)

# Helpers for the checks that were testdata/run/hash_basics.rb.
# User code can reopen Hash; K and V are in scope.
class Hash
  #: () -> Integer
  def double_size = size * 2

  #: () -> K?
  def second_key = keys[1]
end

#: (untyped) -> String
def hash_kind(x)
  case x
  when Hash then "hash"
  else "other"
  end
end

# Helpers for the checks that were testdata/run/hash_fetch_errors.rb.
#: (Hash[String, Integer], String) -> String
def hash_safe_fetch(h, k)
  h.fetch(k).to_s
rescue KeyError => e
  "rescued: #{e.message}"
end

# KeyError messages inspect the key, whatever its type.
#: (Hash[untyped, Integer], untyped) -> String
def try_fetch(h, k)
  h.fetch(k).inspect
rescue KeyError => e
  e.message
end

# Helpers for the checks that were testdata/run/hash_iteration.rb.
# A named block passed on to each/each_pair re-yields (decision 29).
#: (Hash[String, Integer]) { ([String, Integer]) -> void } -> void
def hash_walk(h, &blk)
  h.each(&blk)
end

#: (Hash[String, Integer]) { (String, Integer) -> void } -> void
def walk_pairs(h, &blk)
  h.each_pair(&blk)
end

#: (Hash[String, Integer]) { (String, Integer) -> void } -> void
def walk_yield(h)
  h.each { |k, v| yield k, v }
end

# return from inside each still runs ensure.
#: (Hash[String, Integer], Array[String]) -> String
def find_big(h, log)
  h.each { |k, v| return k if v > 1 }
  "none"
ensure
  log << "ensure"
end

#: (Hash[String, Integer], Integer) -> String?
def first_over(h, limit)
  h.each { |k, v| return k if v > limit }
  nil
end

#: (Hash[String, Integer]) -> Integer
def sum_until_negative(h)
  total = 0
  h.each_value do |v|
    break if v < 0

    total += v
  end
  total
end

#: (Hash[String, Integer]) -> Array[String]
def keys_via_each_key(h)
  out = [] #: Array[String]
  h.each_key { |k| out << k }
  out
end

# Helpers for the checks that were testdata/run/hash_mid.rb.
#: (?Hash[String, Integer]?) -> Integer
def count_or(o = nil) = o ? o.size : -1

#: (?Hash[Symbol, untyped]?) -> String
def opts_or(o = nil) = o ? o.inspect : "none"

#: (bool) -> Hash[String, Integer]?
def hash_maybe(b) = b ? {} : nil

#: (untyped) -> void
def add_key(x)
  if x.is_a?(Hash)
    x["new"] = 1
  end
end

#: (untyped) -> Integer
def hash_grow(x)
  case x
  when Hash
    x["grown"] = 2
    x.size
  else
    -1
  end
end

#: (Hash[Symbol, untyped]) -> Integer
def opts_size(h) = h.size

#: (Hash[untyped, untyped]) -> Integer
def hash_size(h) = h.size

#: (Hash[String, Integer]) -> Integer
def values_total(h) = h.values.reduce(0) { |a, b| a + b }

# Helpers for the checks that were testdata/run/hash_untyped.rb.
#: (untyped) -> untyped
def hash_ident(x) = x

#: (untyped) -> untyped
def lookup(x) = x["a"]

#: (untyped) -> bool
def has_a(x) = x.key?("a")

# Narrowing an untyped value to Hash for reads (writes: hash_bug_narrowed_copy).
#: (untyped) -> String
def hash_untyped_kind(x)
  case x
  when Hash then "hash #{x.size} #{x.keys.inspect}"
  when Array then "array #{x.size}"
  else "other #{x.inspect}"
  end
end

#: (untyped) -> Integer
def size_if_hash(x)
  if x.is_a?(Hash)
    x.size
  else
    -1
  end
end

#: (Integer) -> Integer
def inc(n) = n + 1

#: (String) -> String
def hash_up(s) = s.upcase

#: (untyped) -> String
def hash_describe(x) = x.inspect

#: (untyped) -> Integer
def dyn_size(x) = x.size

#: (Hash[untyped, untyped]) -> Integer
def any_size(h) = h.size

#: (String) -> Integer
def limit(name) = HashTests::LIMITS.fetch(name, 0)

# A default `{}` is a fresh hash on every call, as in Ruby.
#: (?Hash[String, Integer]) -> Integer
def hash_bump(h = {})
  h["x"] = (h["x"] || 0) + 1
  h.size * 10 + (h["x"] || 0)
end

#: (Array[String]) -> Hash[String, Integer]
def lengths(ws)
  out = {} #: Hash[String, Integer]
  ws.each { |w| out[w] = w.size }
  out
end

module HashTests
  class Base
    #: (Hash[Symbol, Integer]) -> void
    def initialize(opts)
      @opts = opts
    end

    #: () -> String
    def to_s = @opts.inspect
  end

  class Sub < Base
    #: () -> void
    def initialize
      super(a: 1, b: 2)
    end
  end

  class Box
    #: (Hash[Symbol, Integer]) -> String
    def show(o) = o.inspect

    #: () -> String
    def call_self = show(k: 1)
  end

  class Conn
    attr_reader :opts #: Hash[Symbol, untyped]

    #: (String, ?Hash[Symbol, untyped]) -> void
    def initialize(host, opts = {})
      @host = host
      @opts = opts
    end

    #: (Hash[Symbol, untyped]) -> Conn
    def with(extra) = Conn.new(@host, @opts.merge(extra))

    #: (Hash[String, Integer]) -> Integer
    def self.weigh(w) = w.values.reduce(0) { |a, b| a + b }

    #: () -> String
    def to_s = "#{@host} #{@opts.inspect}"
  end

  # A Hash subclass that transforms its keys, like Rack::Headers (issue #86).
  # `super` reaches the embedded Hash.
  # @rbs generic V
  # @rbs inherits Hash[untyped, V]
  class Downcaser < Hash
    def [](key)
      super(key.to_s.downcase)
    end

    def []=(k, v)
      super(k.to_s.downcase, v)
    end
  end

  # An alias of an inherited Hash method, like Rack::QueryParser::Params' to_params_hash (#82).
  class AliasParams < Hash
    alias_method :to_params_hash, :to_h
    alias plain_keys keys
  end

  # Helpers for the checks that were testdata/run/hash_bugs.rb.
  # Data and Struct keys match by value
  Point = Data.define(:x, :y) #: [Integer, Integer]

  Pair = Struct.new(:a, :b) #: [String, Integer]

  # instances of an empty class key by identity
  class Token
  end

  # Helpers for the checks that were testdata/run/hash_equality.rb.
  # Plain-object values compare by identity (their ==), as in MRI.
  # (Obj has an ivar: ivar-less instances are hash_bug_empty_class_keys.)
  class Obj
    #: () -> void
    def initialize
      @tag = 0
    end
  end

  # inspect of Hash values that may be nil, including struct-class and Array values
  class Foo
    #: () -> String
    def inspect = "#<Foo>"
  end

  # Helpers for the checks that were testdata/run/hash_values_keys.rb.
  class Animal
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def speak = "..."
  end

  class Dog < Animal
    #: () -> String
    def speak = "woof"
  end

  class Registry
    attr_reader :items #: Hash[String, Animal]

    #: () -> void
    def initialize
      @items = {}
    end

    #: (Animal) -> self
    def add(a)
      @items[a.name] = a
      self
    end

    #: () -> Array[String]
    def names = @items.keys

    # Values dispatch virtually: a Dog stored as an Animal still says woof.
    #: () -> String
    def voices = @items.map { |k, v| "#{k}:#{v.speak}" }.join(" ")
  end

  class Tag
    #: (String) -> void
    def initialize(s)
      @s = s
    end

    #: () -> String
    def inspect = "<#{@s}>"
  end

  LIMITS = { "low" => 1, "high" => 9 } #: Hash[String, Integer]

  # Hashes held by ivars, accessors, Struct members and optional locals.
  class Memo
    # @rbs @cache: Hash[Integer, Integer]

    #: () -> void
    def initialize
      @cache = {}
    end

    #: (Integer) -> Integer
    def fib(n)
      c = @cache[n]
      return c if c

      r = n < 2 ? n : fib(n - 1) + fib(n - 2)
      @cache[n] = r
    end

    #: () -> Integer
    def cached = @cache.size
  end

  class Settings
    attr_accessor :opts #: Hash[String, String]

    #: () -> void
    def initialize
      @opts = { "mode" => "fast" }
    end
  end

  Rec = Struct.new(:name, :tags) #: [String, Hash[String, Integer]]

  class HashArgsTest < Minitest::Test
    # a dynamic fetch whose default is not the Hash's value type answers it (concurrent-ruby's
    # `opts.fetch(:auto_terminate, true)` on a Hash[Symbol, Integer])
    def test_untyped_fetch_other_default
      h = { min: 1 } #: Hash[Symbol, Integer]
      o = h #: untyped
      assert_equal [true, 1, 1], [o.fetch(:auto, true), o.fetch(:min, true), o.fetch(:min, 5)]
    end

    def test_section_0
      assert_equal "{a: 1, b: 2}", (hash_show(a: 1, b: 2))
      assert_equal "{a: 3}", (hash_show a: 3)
      assert_equal "{c: 4}", (hash_show({ c: 4 }))
      assert_equal "{}", hash_show({})
      assert_equal "<br>", tag("br")
      assert_equal "<a href=\"/x\" id=\"y\">", (tag("a", href: "/x", id: "y"))
      assert_equal "<p>", (tag("p", {}))
      assert_equal "<i é=\"ü\">", (tag("i", "é": "ü"))
      assert_equal 3, (hash_total("a" => 1, "b" => 2))
      assert_equal -5, (hash_total("z" => -5))
      assert_equal 0, hash_total({})
      assert_equal "{port: 8080, host: \"h\", on: true, none: nil, list: [1, 2]}", (loose(port: 8080, host: "h", on: true, none: nil, list: [1, 2]))
      assert_equal 12, (hash_scaled(3, by: 4))
      assert_equal 3, (hash_scaled(3, {}))
      assert_equal 0, (hash_scaled(-2, by: 0))
      c = Conn.new("h", port: 1, tls: true)
      assert_equal "h {port: 1, tls: true}", c.to_s
      assert_equal "bare {}", Conn.new("bare").to_s
      assert_equal "h {port: 2, tls: true, retry: 3}", c.with(port: 2, retry: 3).to_s
      assert_equal "h {port: 1, tls: true}", c.with({}).to_s
      assert_equal "h {port: 1, tls: true}", c.to_s
      assert_equal 3, (Conn.weigh("x" => 1, "y" => 2))
      assert_equal 3, (Conn.weigh "z" => 3)
      assert_equal 1, c.opts[:port]
      assert_nil c.opts[:missing]
    end

    # Core methods take braceless hashes too.
    def test_core_methods_take_braceless_hashes
      assert_equal "[{a: 1}]", (([a: 1])).inspect
      assert_equal "[1, {b: 2}]", (([1, b: 2])).inspect
      k = { a: 1 } #: Hash[Symbol, Integer]
      assert_equal "{a: 1, b: 2}", ((k.merge(b: 2))).inspect
      assert_equal "{a: 9}", ((k.merge(a: 9))).inspect
      assert_equal "{a: 1}", (k).inspect
    end

    # Braceless hashes into untyped, rest, nilable, mixed-key and block-taking methods.
    #: (Hash[Symbol, String?] h) -> Array[String?]
    def hash_opt_values(h) = h.values

    # An untyped Hash of Strings and nil into a Hash[Symbol, String?] parameter: each value is boxed (Homebrew's
    # CaskStruct ruby_source_checksum from JSON data)
    def test_untyped_into_optional_values
      data = { "sha" => "abc" } #: Hash[String, untyped]
      h = { sha256: data["sha"], md5: data["none"] } #: untyped
      assert_equal ["abc", nil], hash_opt_values(h)
    end

    def test_braceless_hashes_into_untyped_rest
      assert_equal "{a: 1}", (any_arg(a: 1))
      assert_equal "{\"x\" => [1]}", (any_arg("x" => [1]))
      assert_equal "[{a: 1}]", (rest(a: 1))
      assert_equal "[1, {a: 2}]", (rest(1, a: 2))
      assert_equal "[]", rest
      assert_equal "1 [{b: 2}]", (rest2(1, b: 2))
      assert_equal "{z: 1}", (opt_nil(z: 1))
      assert_equal "none", opt_nil
      assert_equal "none", opt_nil(nil)
      assert_equal "{\"a\" => 1, b: 2}", (mixed("a" => 1, b: 2))
      assert_equal 10, (with_blk(n: 5) { |x| x * 2 })
      assert_equal 7, (with_blk(n: 6) do |x| x + 1 end)
      assert_equal "1 {} {}", two_opts(1)
      assert_equal "1 {x: 1} {}", (two_opts(1, x: 1))
      assert_equal "1 {x: 1} {y: 2}", (two_opts(1, { x: 1 }, y: 2))
      got = [] #: Array[String]
      give { |gh| got << gh.inspect }
      assert_equal ["{a: 1}"], got
      assert_equal "[:b, :c]", (give_v { |gh| gh.keys.inspect })
    end

    # ... into super, and into a call on self.
    def test_into_super_and_into_a
      assert_equal "{a: 1, b: 2}", Sub.new.to_s
      assert_equal "{k: 1}", Box.new.call_self
      shown = Box.new.show k2: 7
      assert_equal "{k2: 7}", shown
    end
  end

  class HashBasicsTest < Minitest::Test
    def test_section_0
      h = { "b" => 2, "a" => 1, "c" => 3 } #: Hash[String, Integer]
      assert_equal "{\"b\" => 2, \"a\" => 1, \"c\" => 3}", (h).inspect
      assert_equal ["b", "a", "c"], h.keys
      assert_equal [2, 1, 3], h.values
      h["a"] = 10
      h["d"] = 4
      assert_equal "{\"b\" => 2, \"a\" => 10, \"c\" => 3, \"d\" => 4}", (h).inspect
      assert_equal 2, h.delete("b")
      assert_nil h.delete("b")
      assert_nil h.delete("zz")
      assert_equal "{\"a\" => 10, \"c\" => 3, \"d\" => 4}", (h).inspect
      h["b"] = 20
      assert_equal "{\"a\" => 10, \"c\" => 3, \"d\" => 4, \"b\" => 20}", (h).inspect
      assert_equal 10, h["a"]
      assert_nil h["nope"]
      assert_nil h[""]
      assert_equal 4, h.size
      assert_equal 4, h.length
      assert_equal false, h.empty?
      assert_equal true, h.key?("a")
      assert_equal false, h.key?("A")
      assert_equal true, h.has_key?("b")
      assert_equal false, h.include?("x")
      assert_equal true, h.include?("d")
      assert_equal 3, h.fetch("c")
      assert_equal -1, (h.fetch("zz", -1))
      assert_equal 3, (h.fetch("c", 99))
      assert_equal 0, (h.fetch("zz", 0))
      r = (h["e"] = 5)
      assert_equal 5, r
      assert_equal 5, h.size
      e = {} #: Hash[String, Integer]
      assert_equal "{}", (e).inspect
      assert_equal 0, e.size
      assert_equal true, e.empty?
      assert_equal [], e.keys
      assert_equal [], e.values
      assert_nil e["x"]
      assert_nil e.delete("x")
      assert_equal false, e.key?("")
      c = h.clear
      assert_equal "{}", (c).inspect
      assert_equal true, c.equal?(h)
      assert_equal 0, h.size
      assert_equal true, h.empty?
      h["z"] = 26
      h["y"] = 25
      assert_equal "{\"z\" => 26, \"y\" => 25}", (h).inspect
      assert_equal ["z", "y"], h.keys
      ks = h.keys
      ks << "extra"
      vs = h.values
      vs << 0
      assert_equal ["z", "y", "extra"], ks
      assert_equal ["z", "y"], h.keys
      assert_equal [26, 25], h.values
      alias_h = h
      alias_h["x"] = 24
      assert_equal "{\"z\" => 26, \"y\" => 25, \"x\" => 24}", (h).inspect
      assert_equal 3, h.size
      m1 = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      m2 = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
      assert_equal "{\"a\" => 1, \"b\" => 20, \"c\" => 30}", (m1.merge(m2)).inspect
      assert_equal "{\"b\" => 2, \"c\" => 30, \"a\" => 1}", (m2.merge(m1)).inspect
      assert_equal "{\"a\" => 1, \"b\" => 2}", (m1).inspect
      assert_equal "{\"b\" => 20, \"c\" => 30}", (m2).inspect
      assert_equal "{\"a\" => 1, \"b\" => 2}", (m1.merge({})).inspect
      assert_equal "{\"a\" => 1, \"b\" => 2}", (e.merge(m1)).inspect
      assert_equal "{}", (e.merge(e)).inspect
      assert_equal "{\"q\" => 0, \"a\" => 1, \"b\" => 2}", (({ "q" => 0 }.merge(m1))).inspect
      ints = { 3 => "c", -1 => "neg", 0 => "zero", 4_611_686_018_427_387_904 => "big" } #: Hash[Integer, String]
      assert_equal "{3 => \"c\", -1 => \"neg\", 0 => \"zero\", 4611686018427387904 => \"big\"}", (ints).inspect
      assert_equal "neg", ints[-1]
      assert_equal "zero", ints[0]
      assert_equal "big", ints[4_611_686_018_427_387_904]
      assert_nil ints[1]
      ints[-1] = "minus one"
      assert_equal [3, -1, 0, 4611686018427387904], ints.keys
      assert_equal "minus one", ints[-1]
    end

    # Float keys: -0.0 and 0.0 are the same key, as in Ruby.
    def test_float_keys_0_0_and
      floats = { 0.5 => 1, -0.0 => 2, 1e20 => 3, 2.0 => 4 } #: Hash[Float, Integer]
      assert_equal "{0.5 => 1, -0.0 => 2, 1.0e+20 => 3, 2.0 => 4}", (floats).inspect
      assert_equal 2, floats[0.0]
      assert_equal 4, floats[2.0]
      assert_nil floats[0.25]
      bools = { true => "t", false => "f" } #: Hash[bool, String]
      assert_equal "f", bools[false]
      assert_equal "t", bools[true]
      assert_equal 2, bools.size
      sym = { alpha: 1, beta: 2 } #: Hash[Symbol, Integer]
      assert_equal 1, sym[:alpha]
      assert_nil sym[:gamma]
      assert_equal true, sym.key?(:beta)
      assert_equal [:alpha, :beta], sym.keys
      uni = { "héllo" => 1, "日本" => 2, "😀" => 3, "" => 4 } #: Hash[String, Integer]
      assert_equal 2, uni["日本"]
      assert_equal 3, uni["😀"]
      assert_equal 4, uni[""]
      assert_nil uni["hello"]
      assert_equal [5, 2, 1, 0], (uni.keys.map { |k| k.size })
    end

    # A String key and a Symbol key with the same text are different keys.
    def test_a_string_key_and_a
      sk = { a: 1, "a" => 2 } #: Hash[untyped, Integer]
      assert_equal 1, sk[:a]
      assert_equal 2, sk["a"]
      assert_equal 2, sk.size
    end

    # A hash may still be built explicitly without a default (decision 5).
    def test_no_default_value_hash_decision
      lists = {} #: Hash[String, Array[Integer]]
      [3, 1, 4, 1, 5].each do |n|
        key = n.odd? ? "odd" : "even"
        bucket = lists[key]
        if bucket
          bucket << n
        else
          lists[key] = [n]
        end
      end
      assert_equal "{\"odd\" => [3, 1, 1, 5], \"even\" => [4]}", (lists).inspect
    end

    # The decision-5 replacements for Hash.new(0) + `+= 1`.
    def test_the_decision_5_replacements_for
      counts = {} #: Hash[String, Integer]
      "the quick the lazy the end quick".split.each { |w| counts[w] = (counts[w] || 0) + 1 }
      assert_equal "{\"the\" => 3, \"quick\" => 2, \"lazy\" => 1, \"end\" => 1}", (counts).inspect
      counts2 = {} #: Hash[String, Integer]
      "b a b".split.each { |w| counts2[w] = counts2.fetch(w, 0) + 1 }
      assert_equal "{\"b\" => 2, \"a\" => 1}", (counts2).inspect
    end

    # Nested hashes: the inner hash is shared, not copied.
    def test_nested_hashes_the_inner_hash
      nested = { "x" => { "y" => 1 }, "e" => {} } #: Hash[String, Hash[String, Integer]]
      inner = nested["x"]
      if inner
        inner["z"] = 2
      end
      assert_equal "{\"x\" => {\"y\" => 1, \"z\" => 2}, \"e\" => {}}", (nested).inspect
      assert_equal 2, nested["x"]&.fetch("z")
      assert_nil nested["nope"]&.fetch("z")
      # Value-returning block calls through &. on a possibly-nil nested hash
      # (iterators through &. are dynamic_bug_safe_nav_iterator).
      assert_equal ["y1", "z2"], (nested["x"]&.map { |nk, nv| "#{nk}#{nv}" })
      assert_nil (nested["zz"]&.map { |nk, _nv| nk })
      assert_equal "{\"z\" => 2}", ((nested["x"]&.select { |_nk, nv| nv > 1 })).inspect
      assert_equal ["y", "z"], nested["x"]&.keys
    end

    # Reopened methods work on every instantiation.
    def test_reopened_methods_on_every_instantiation
      h = { "z" => 26, "y" => 25, "x" => 24 } #: Hash[String, Integer]
      e = {} #: Hash[String, Integer]
      sym = { alpha: 1, beta: 2 } #: Hash[Symbol, Integer]
      assert_equal 6, h.double_size
      assert_equal "y", h.second_key
      assert_nil e.second_key
      assert_equal :beta, sym.second_key
    end

    # Optional values: a stored nil is still a key.
    def test_optional_values_a_stored_nil
      ov = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
      assert_equal 2, ov.size
      assert_equal true, ov.key?("a")
      assert_equal ["a", "b"], ov.keys
      got = [] #: Array[String]
      ov.each { |k, v| got << "#{k} #{v.nil?} #{v.to_s}" }
      ov.each_value { |v| got << v.inspect }
      assert_equal ["a true ", "b false 2", "nil", "2"], got
      assert_nil ov.fetch("a")
      assert_equal 2, ov.fetch("b")
      assert_nil (ov.fetch("a", 9))
      ov["a"] = 5
      assert_equal ["a", "b"], (ov.select { |_k, v| !v.nil? }.keys)
      assert_equal ["a", "b"], (ov.map { |k, v| v ? k : "-" })
    end

    # Assignment in a condition narrows the lookup.
    def test_assignment_in_condition_narrows
      h = { "z" => 26, "y" => 25, "x" => 24 } #: Hash[String, Integer]
      got = 0
      if (hit = h["z"])
        got = hit + 1
      end
      assert_equal 27, got
      assert_equal 27, (h.delete("z") || 0) + 1
      assert_equal 1, (h.delete("z") || 0) + 1
    end

    # More bucket idioms without a default value.
    def test_more_bucket_idioms_without_a
      all = {} #: Hash[String, Array[Integer]]
      [1, 2, 3].each { |n| all["all"] = (all["all"] || []) + [n] }
      assert_equal "{\"all\" => [1, 2, 3]}", (all).inspect
      cnt = {} #: Hash[Symbol, Integer]
      %i[a b a c a].each { |s| cnt[s] = cnt.fetch(s, 0) + 1 }
      assert_equal "{a: 3, b: 1, c: 1}", (cnt).inspect
      assert_equal [:a, 3], (cnt.max_by { |_k, v| v })
    end
  end

  class HashBugsTest < Minitest::Test
    # adding a key during each raises, like MRI
    def test_adding_a_key_during_each
      add_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      visits = [] #: Array[String]
      e = assert_raises(RuntimeError) do
        add_h.each do |k, v|
          visits << "visit #{k}"
          add_h["new#{k}"] = v
        end
      end
      assert_equal ["visit a"], visits
      assert_equal RuntimeError, e.class
      assert_equal "can't add a new key into hash during iteration", e.message
      assert_equal "{\"a\" => 1, \"b\" => 2}", (add_h).inspect
      data_h = {} #: Hash[Point, String]
      data_h[Point.new(x: 1, y: 2)] = "p"
      assert_equal "p", (data_h[Point.new(x: 1, y: 2)])
      data_h[Point.new(x: 1, y: 2)] = "q"
      assert_equal 1, data_h.size
      data_s = {} #: Hash[Pair, Integer]
      data_s[Pair.new("k", 1)] = 1
      assert_equal true, (data_s.key?(Pair.new("k", 1)))
      assert_equal 1, ([Pair.new("k", 1), Pair.new("k", 1)].tally.size)
    end

    # each_pair with one block param yields the pair
    def test_each_pair_with_one_block
      pair_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
      got = [] #: Array[String]
      pair_h.each_pair { |x| got << x.inspect }
      assert_equal ["[\"a\", 1]", "[\"bb\", 2]"], got
      tok_a = Token.new
      tok_b = Token.new
      assert_equal false, tok_a.equal?(tok_b)
      assert_equal false, (tok_a == tok_b)
      seen = {} #: Hash[Token, Integer]
      seen[tok_a] = 1
      seen[tok_b] = 2
      assert_equal 2, seen.size
      assert_equal 1, seen[tok_a]
      assert_equal false, ({ "t" => tok_a } == { "t" => tok_b })
    end

    # == with optional values ignores order
    def test_with_optional_values_ignores_order
      opt_a = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
      opt_b = { "b" => nil, "a" => 1 } #: Hash[String, Integer?]
      assert_equal true, (opt_a == opt_b)
      opt_c = { "s" => "x" } #: Hash[String, String?]
      opt_d = { "s" => "x" } #: Hash[String, String?]
      assert_equal true, (opt_c == opt_d)
      u1 = { 1 => "x", "k" => nil }
      u2 = { "k" => nil, 1 => "x" }
      assert_equal true, (u1 == u2)
    end

    # == against untyped literals; MRI's 1 == 1.0 is true
    def test_against_untyped_literals_mri_s
      lit_e = {} #: Hash[String, Integer]
      assert_equal true, (lit_e == {})
      lit_t = { "a" => 1 } #: Hash[String, Integer]
      lit_u = {}
      lit_u["a"] = 1
      assert_equal true, (lit_t == lit_u)
      assert_equal true, (lit_u == lit_t)
      lit_s = { a: 1 } #: Hash[Symbol, Integer]
      assert_equal true, (lit_s == { a: 1, b: "x" }.reject { |k, _v| k == :b })
      assert_equal true, ({ 1 => 1 } == { 1 => 1.0 })
    end

    # an explicit nil fetch default is a value, not "no default"
    def test_an_explicit_nil_fetch_default
      fetch_h = { "a" => 1 } #: Hash[String, Integer]
      assert_equal 1, (fetch_h.fetch("a", nil))
      assert_nil (fetch_h.fetch("x", nil))
      cfg = { debug: false } #: Hash[Symbol, untyped]
      assert_nil (cfg.fetch(:nope, nil))
    end

    # a negative count to first is MRI's ArgumentError
    def test_a_negative_count_to_first
      first_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      e = assert_raises(ArgumentError) { first_h.first(-1) }
      assert_equal "attempt to take negative size", e.message
    end

    # hash literals are not frozen
    def test_hash_literals_are_not_frozen
      frozen_h = { "a" => 1 } #: Hash[String, Integer]
      assert_equal false, frozen_h.frozen?
      assert_equal false, {}.frozen?
    end

    # Hash.new with no default
    def test_hash_new_with_no_default
      new_h = Hash.new #: Hash[String, Integer]
      new_h["a"] = 1
      assert_equal "{\"a\" => 1}", (new_h).inspect
      assert_equal 1, new_h.size
    end

    # Hash.new(default) and Hash.new { |h, k| ... } (decision 5).
    def test_hash_default_value_and_block
      counts = Hash.new(0) #: Hash[String, Integer]
      counts["a"] += 1
      counts["a"] += 1
      counts["b"] += 1
      assert_equal "{\"a\" => 2, \"b\" => 1}", (counts).inspect
      assert_equal 0, counts["missing"]
      assert_equal [0], counts.values_at("missing")
      assert_equal 0, counts.default
      begin
        counts.fetch("missing")
        assert_equal "unreachable", "fetch used the default"
      rescue KeyError
        assert_equal 0, counts.default
      end
      counts.default = 7
      assert_equal 7, counts["nope"]
      assert_equal 7, counts.default

      made = Hash.new { |h, k| h[k] = k.to_s * 2 } #: Hash[String, String]
      assert_equal "aa", made["a"]
      assert_equal "bb", made["b"]
      assert_equal "{\"a\" => \"aa\", \"b\" => \"bb\"}", (made).inspect
      assert_nil made.default
      made.default = "z"
      assert_equal "z", made.default
    end

    # symbol keys print as labels only when MRI's would
    def test_symbol_keys_print_as_labels
      emo = { :"😀" => 2, :"a😀" => 3, :"😀?" => 23, :"٣" => 25 } #: Hash[Symbol, Integer]
      assert_equal "{😀: 2, a😀: 3, 😀?: 23, ٣: 25}", (emo).inspect
      mix = { é: 1, "@iv": 1, "$g": 1, "a=": 1, A: 1, a?: 1, "+": 1, "[]": 1, "1a": 1, "a b": 1, "`": 1 } #: Hash[Symbol, Integer]
      assert_equal "{é: 1, \"@iv\": 1, \"$g\": 1, \"a=\": 1, A: 1, a?: 1, \"+\": 1, \"[]\": 1, \"1a\": 1, \"a b\": 1, \"`\": 1}", (mix).inspect
    end

    # != compares by content, ignoring order
    def test_compares_by_content_ignoring_order
      ne_a = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
      ne_b = { "y" => 2, "x" => 1 } #: Hash[String, Integer]
      ne_c = { "x" => 1 } #: Hash[String, Integer]
      assert_equal false, (ne_a != ne_b)
      assert_equal true, (ne_a != ne_c)
      assert_equal false, (ne_a != ne_a)
    end

    # optional keys, and group_by/tally keyed by optional values
    def test_optional_keys_and_group_by
      nk = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
      assert_equal 1, nk[nil]
      assert_equal 2, nk["a"]
      assert_equal true, nk.key?("a")
      nk_s = "b"
      nk[nk_s] = 3
      assert_equal 3, nk["b"]
      assert_equal 3, nk.size
      nk["b"] = 4
      assert_equal 3, nk.size
      optv_h = { "a" => 1, "b" => 1, "c" => nil, "d" => nil } #: Hash[String, Integer?]
      assert_equal 2, (optv_h.group_by { |_k, v| v }.size)
      assert_equal 2, optv_h.values.tally.size
    end

    # self-referencing containers inspect as {...}/[...]
    def test_self_referencing_containers_inspect_as
      rec_h = {} #: Hash[String, untyped]
      rec_h["me"] = rec_h
      rec_h["n"] = 1
      assert_equal 2, rec_h.size
      assert_equal "{\"me\" => {...}, \"n\" => 1}", (rec_h).inspect
      rec_a = [1] #: Array[untyped]
      rec_a << rec_a
      assert_equal "[1, [...]]", (rec_a).inspect
      rec_g = {} #: Hash[Symbol, untyped]
      rec_g[:a] = [rec_g, rec_a]
      assert_equal "{a: [{...}, [1, [...]]]}", (rec_g).inspect
      assert_equal "[[1, [...]], [1, [...]]]", (([rec_a, rec_a])).inspect
    end

    # select/filter/reject with one block param yield the pair
    def test_select_filter_reject_with_one
      sel_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
      got = [] #: Array[String]
      sel_h.select { |x| got << x.inspect; true }
      sel_h.filter { |x| got << x.inspect; false }
      sel_h.reject { |x| got << x.inspect; false }
      assert_equal ["\"a\"", "\"bb\"", "\"a\"", "\"bb\"", "\"a\"", "\"bb\""], got
    end

    # _ block params
    def test_block_params
      us_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
      vals = [] #: Array[Integer]
      us_h.each { |_, v| vals << v }
      assert_equal [1, 2], vals
      assert_equal [3, 6], (us_h.map { |_, v| v * 3 })
      assert_equal "{\"bb\" => 2}", ((us_h.select { |k, _| k.size > 1 })).inspect
      us_n = 0
      us_h.each_key { |_| us_n += 1 }
      assert_equal 2, us_n
    end

    # sorting on untyped values and keys reaches rbCmp
    def test_sorting_on_untyped_values_and
      scores = { "b" => 2, "a" => 1, "c" => 3 } #: Hash[String, untyped]
      assert_equal ["c", 3], (scores.max_by { |_k, v| v })
      assert_equal ["a", 1], (scores.min_by { |_k, v| v })
      assert_equal [["a", 1], ["b", 2], ["c", 3]], (scores.sort_by { |_k, v| v })
      byk = { 2 => "b", 1 => "a" } #: Hash[untyped, String]
      assert_equal [[1, "a"], [2, "b"]], byk.sort
      assert_equal [1, "a"], byk.min
      people = [{ name: "a", age: 30 }, { name: "b", age: 20 }] #: Array[Hash[Symbol, untyped]]
      assert_equal "{name: \"b\", age: 20}", ((people.min_by { |p| p.fetch(:age) })).inspect
    end

    # unused block params
    def test_unused_block_params
      ub_h = { "a" => 1, "bb" => 2 } #: Hash[String, Integer]
      assert_equal "{\"bb\" => 2}", ((ub_h.select { |k, v| v > 1 })).inspect
      assert_equal ["a=1", "bb=2"], (ub_h.map { |k, v| "#{k}=#{v}" })
      assert_equal ["bb", 2], (ub_h.find { |k, v| v > 1 })
      assert_equal ["a", 1], (ub_h.min_by { |k, v| k.size })
      assert_equal "hash", (hash_kind({ "a" => 1 }))
      assert_equal "other", hash_kind(1)
    end
  end

  class HashEnumerableTest < Minitest::Test
    def test_section_0
      h = { "a" => 1, "bb" => 2, "ccc" => 3 } #: Hash[String, Integer]
      assert_equal "{\"a\" => 1, \"ccc\" => 3}", ((h.select { |_k, v| v.odd? })).inspect
      assert_equal "{\"bb\" => 2, \"ccc\" => 3}", ((h.filter { |k, _v| k.size > 1 })).inspect
      assert_equal "{\"a\" => 1, \"ccc\" => 3}", ((h.reject { |_k, v| v == 2 })).inspect
      assert_equal "{}", ((h.select { |_k, _v| false })).inspect
      assert_equal "{\"a\" => 1, \"bb\" => 2, \"ccc\" => 3}", ((h.reject { |_k, _v| false })).inspect
      sel = h.select { |_k, v| v > 0 }
      sel["new"] = 9
      assert_equal 3, h.size
      assert_equal 4, sel.size
      assert_equal ["a=1", "bb=2", "ccc=3"], (h.map { |k, v| "#{k}=#{v}" })
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], (h.map { |pair| pair })
      assert_equal [["A", 10], ["BB", 20], ["CCC", 30]], (h.map { |k, v| [k.upcase, v * 10] })
      assert_equal ["a", "1", "bb", "2", "ccc", "3"], (h.flat_map { |k, v| [k, v.to_s] })
      assert_equal ["bb", 2], (h.find { |_k, v| v > 1 })
      assert_nil (h.detect { |_k, v| v > 5 })
      assert_equal true, (h.any? { |_k, v| v > 2 })
      assert_equal false, (h.any? { |_k, v| v > 3 })
      assert_equal true, (h.all? { |_k, v| v > 0 })
      assert_equal false, (h.all? { |k, _v| k.size < 3 })
      assert_equal true, (h.none? { |k, _v| k.empty? })
      assert_equal false, (h.none? { |k, _v| k == "a" })
      assert_equal 3, h.count
      assert_equal 6, (h.reduce(0) { |sum, pair| sum + pair[1] })
      assert_equal "abbccc", (h.inject("") { |acc, pair| acc + pair[0] })
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], h.to_a
      assert_equal true, h.include?("a")
      assert_equal false, h.include?("z")
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], h.sort
      assert_equal ["a", 1], h.min
      assert_equal ["ccc", 3], h.max
      assert_equal [["ccc", 3], ["bb", 2], ["a", 1]], (h.sort_by { |_k, v| -v })
      assert_equal ["a", 1], (h.min_by { |_k, v| v })
      assert_equal ["ccc", 3], (h.max_by { |k, _v| k.size })
      assert_equal "{true => [[\"a\", 1], [\"ccc\", 3]], false => [[\"bb\", 2]]}", ((h.group_by { |_k, v| v.odd? })).inspect
      assert_equal "{[\"a\", 1] => 1, [\"bb\", 2] => 1, [\"ccc\", 3] => 1}", (h.tally).inspect
      assert_equal [["a", 1], ["bb", 2]], h.first(2)
      assert_equal [["a", 1]], h.take(1)
      assert_equal [], h.first(0)
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], h.first(10)
      ints = { 3 => "c", -1 => "neg", 0 => "zero" } #: Hash[Integer, String]
      assert_equal [[-1, "neg"], [0, "zero"], [3, "c"]], ints.sort
      assert_equal [-1, "neg"], ints.min
      assert_equal [3, "c"], ints.max
      assert_equal [3, "c"], (ints.min_by { |_k, v| v })
      assert_equal [[3, "c"], [-1, "neg"], [0, "zero"]], (ints.sort_by { |_k, v| v.size })
      sym = { b: 2, a: 1, c: 0 } #: Hash[Symbol, Integer]
      assert_equal [[:a, 1], [:b, 2], [:c, 0]], sym.sort
      assert_equal [[:c, 0], [:a, 1], [:b, 2]], (sym.sort_by { |_k, v| v })
      assert_equal [:a, 1], (sym.min_by { |k, _v| k })
      st = { "Zed" => "b", "alpha" => "a", "Élan" => "c", "beta" => "d" } #: Hash[String, String]
      assert_equal [["Zed", "b"], ["alpha", "a"], ["beta", "d"], ["Élan", "c"]], st.sort
      assert_equal ["alpha", "beta", "Zed", "Élan"], (st.sort_by { |k, _v| k.downcase }.map { |k, _v| k })
      fl = { "x" => 2.5, "y" => -1.5 } #: Hash[String, Float]
      assert_equal "[[\"y\", -1.5], [\"x\", 2.5]]", ((fl.sort_by { |_k, v| v })).inspect
      assert_equal "[\"x\", 2.5]", ((fl.max_by { |_k, v| v })).inspect
      arrv = { "b" => [2, 1], "a" => [1] } #: Hash[String, Array[Integer]]
      assert_equal [["a", [1]], ["b", [2, 1]]], arrv.sort
      assert_equal ["b", [2, 1]], (arrv.max_by { |_k, v| v.size })
      e = {} #: Hash[String, Integer]
      assert_nil e.min
      assert_nil e.max
      assert_nil (e.min_by { |_k, v| v })
      assert_equal [], e.sort
      assert_equal [], e.to_a
      assert_nil (e.find { |_k, _v| true })
      assert_equal false, (e.any? { |_k, _v| true })
      assert_equal true, (e.all? { |_k, _v| false })
      assert_equal 10, (e.reduce(10) { |s, _pair| s + 1 })
      assert_equal 0, e.count
      assert_equal [], e.first(3)
      assert_equal "{}", ((e.group_by { |k, _v| k })).inspect
      assert_equal "{}", (e.tally).inspect
      assert_equal [], (e.flat_map { |k, _v| [k] })
      assert_equal "{}", ((e.select { |_k, _v| true })).inspect
      assert_equal [], (e.map { |k, _v| k })
    end

    # Word counting as in example 05, sorted by count then word.
    def test_word_counting_as_in_example
      words = "the cat and the hat and the bat".split
      counts = words.tally
      assert_equal "{\"the\" => 3, \"cat\" => 1, \"and\" => 2, \"hat\" => 1, \"bat\" => 1}", (counts).inspect
      assert_equal [["the", 3], ["and", 2], ["bat", 1]], (counts.sort_by { |w, n| [-n, w] }.first(3))
      assert_equal "{3 => [\"the\", \"cat\", \"and\", \"the\", \"hat\", \"and\", \"the\", \"bat\"]}", ((words.group_by { |w| w.size })).inspect
      assert_equal "{\"é\" => 2, \"e\" => 1}", ((["é", "e", "é"].tally)).inspect
      assert_equal "{}", ([].tally).inspect
    end

    # Chains: Hash#select returns a Hash, so Hash methods keep working after it.
    def test_chains_after_hash_select
      h = { "a" => 1, "bb" => 2, "ccc" => 3 } #: Hash[String, Integer]
      assert_equal ["bb2", "ccc3"], (h.select { |_k, v| v > 1 }.map { |k, v| "#{k}#{v}" })
      assert_equal ["a"], (h.reject { |_k, v| v > 1 }.keys)
      assert_equal 3, (h.merge({ "d" => 4 }).select { |k, _v| k.size < 3 }.size)
      assert_equal ["bbbb"], (h.select { |_k, v| v > 1 }.reject { |k, _v| k.size > 2 }.map { |k, v| k * v }.sort.first(1))
      assert_equal ["ccc", "bb"], (h.sort_by { |_k, v| -v }.first(2).map { |k, _v| k })
      assert_equal [["ccc", 3], ["bb", 2], ["a", 1]], h.to_a.sort.reverse
      assert_equal [[1, "a"], [2, "bb"], [3, "ccc"]], (h.map { |k, v| [v, k] }.sort)
      assert_equal [3, "ccc"], (h.map { |k, v| [v, k] }.max)
      assert_equal ["A", "BB", "CCC"], h.keys.map(&:upcase)
      assert_equal [2, 4, 6], (h.values.map { |v| v * 2 })
      assert_equal "[{\"a\" => 1}, {\"bb\" => 2}, {\"ccc\" => 3}]", ((h.map { |k, v| { k => v } })).inspect
      got = [] #: Array[String]
      h.map { |k, v| [k, v] }.each { |k, v| got << "#{k}#{v}" }
      assert_equal ["a1", "bb2", "ccc3"], got
      assert_equal [3, 2, 1], h.values.sort.reverse
      assert_equal "ccc", h.keys.max
      assert_equal "ccc", (h.keys.min_by { |k| -k.size })
      assert_equal 6, (h.inject(0) { |s, pair| s + pair.last })
      pair = h.find { |_k, v| v == 2 }
      got = []
      if pair
        k, v = pair
        got << k << v.to_s
      end
      assert_equal ["bb", "2"], got
      assert_equal "a", (h.min_by { |_k, v| v }&.first)
      assert_equal 3, (h.max_by { |_k, v| v }&.last)
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], h.sort_by(&:last)
      assert_equal ["a", 1], h.min_by(&:last)
      assert_equal ["ccc", 3], h.max_by(&:first)
      assert_equal "{1 => [[\"a\", 1]], 2 => [[\"bb\", 2]], 3 => [[\"ccc\", 3]]}", (h.group_by(&:last)).inspect
      assert_equal ["[\"a\", 1]", "[\"bb\", 2]", "[\"ccc\", 3]"], h.map(&:inspect)
      assert_equal 3, (h.then { |x| x.size })
      assert_equal "Hash", (h.class).to_s
      assert_equal "Hash", (h.class).inspect
      assert_equal true, h.is_a?(Hash)
      assert_equal true, h.is_a?(Enumerable)
      assert_equal false, h.nil?
      assert_equal true, h.respond_to?(:each)
      assert_equal false, h.respond_to?(:nope)
      assert_equal true, (h.select { |_k, v| v > 5 }.empty?)
      assert_equal false, (!h)
      ss = { a: :z, b: :y, c: :x } #: Hash[Symbol, Symbol]
      assert_equal [[:c, :x], [:b, :y], [:a, :z]], (ss.sort_by { |_k, v| v })
      assert_equal [:a, :z], (ss.max_by { |_k, v| v })
      assert_equal [:a, :z], ss.min
      ff = { 2.5 => "x", -1.0 => "y", 0.0 => "z" } #: Hash[Float, String]
      assert_equal "[[-1.0, \"y\"], [0.0, \"z\"], [2.5, \"x\"]]", (ff.sort).inspect
      assert_equal "[2.5, \"x\"]", (ff.max).inspect
      assert_equal "[2.5, \"x\"]", ((ff.min_by { |_k, v| v })).inspect
      tt = { [2, "b"] => 1, [1, "z"] => 2, [2, "a"] => 3 } #: Hash[[Integer, String], Integer]
      assert_equal [[[1, "z"], 2], [[2, "a"], 3], [[2, "b"], 1]], tt.sort
      assert_equal [[1, "z"], 2], tt.min
      assert_equal [[1, "z"], [2, "a"], [2, "b"]], tt.keys.sort
      assert_equal [2, 3, 1], (tt.sort_by { |k, _v| k }.map(&:last))
    end

    # Building hashes with reduce, each_with_index and group_by.
    def test_building_hashes_with_reduce_each
      fruit = %w[pear apple fig apple pear pear]
      init = {} #: Hash[String, Integer]
      fc = fruit.reduce(init) do |acc, w|
        acc[w] = acc.fetch(w, 0) + 1
        acc
      end
      assert_equal "{\"pear\" => 3, \"apple\" => 2, \"fig\" => 1}", (fc).inspect
      assert_equal true, init.equal?(fc)
      first_at = {} #: Hash[String, Integer]
      fruit.each_with_index { |w, i| first_at[w] = i unless first_at.key?(w) }
      assert_equal "{\"pear\" => 0, \"apple\" => 1, \"fig\" => 2}", (first_at).inspect
      by_size = fruit.group_by { |w| w.size }
      assert_equal ["4:pear", "5:apple", "3:fig"], (by_size.map { |n, ws| "#{n}:#{ws.uniq.join("/")}" })
      assert_equal [3, 4, 5], by_size.keys.sort
      assert_equal 3, by_size.fetch(4).size
      assert_equal "{4 => [[\"pear\", 3]], 5 => [[\"apple\", 2]], 3 => [[\"fig\", 1]]}", ((fc.group_by { |w, _n| w.size })).inspect
      assert_equal "pear=3 apple=2 fig=1", (fc.sort_by { |w, n| [-n, w] }.map { |w, n| "#{w}=#{n}" }.join(" "))
      inv = {} #: Hash[Integer, Array[String]]
      fc.each do |w, n|
        list = inv[n]
        if list
          list << w
        else
          inv[n] = [w]
        end
      end
      assert_equal "{3 => [\"pear\"], 2 => [\"apple\"], 1 => [\"fig\"]}", (inv).inspect
    end
  end

  class HashEqualityTest < Minitest::Test
    def test_hash_equality
      a = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
      b = { "y" => 2, "x" => 1 } #: Hash[String, Integer]
      c = { "x" => 1 } #: Hash[String, Integer]
      d = { "x" => 1, "y" => 3 } #: Hash[String, Integer]
      f = { "x" => 1, "z" => 2 } #: Hash[String, Integer]
      assert_equal true, (a == b)
      assert_equal true, (b == a)
      assert_equal true, (a == a)
      assert_equal false, (a == c)
      assert_equal false, (c == a)
      assert_equal false, (a == d)
      assert_equal false, (a == f)
      assert_equal true, (c == { "x" => 1 })
      assert_equal false, (c == { "x" => 2 })
      assert_equal false, (a == 1)
      assert_equal false, (a == nil)
      assert_equal false, (a == [1])
      assert_equal false, (a == "x")
      assert_equal true, a.equal?(a)
      assert_equal false, a.equal?(b)
      alias_a = a
      assert_equal true, alias_a.equal?(a)
      e1 = {} #: Hash[String, Integer]
      e2 = {} #: Hash[String, Integer]
      assert_equal true, (e1 == e2)
      assert_equal false, (e1 == a)
      # Values compare with ==, so nested arrays and hashes compare by contents.
      n1 = { "k" => [1, 2], "j" => [] } #: Hash[String, Array[Integer]]
      n2 = { "j" => [], "k" => [1, 2] } #: Hash[String, Array[Integer]]
      n3 = { "k" => [2, 1], "j" => [] } #: Hash[String, Array[Integer]]
      assert_equal true, (n1 == n2)
      assert_equal false, (n1 == n3)
      hh1 = { "o" => { "i" => 1 } } #: Hash[String, Hash[String, Integer]]
      hh2 = { "o" => { "i" => 1 } } #: Hash[String, Hash[String, Integer]]
      hh3 = { "o" => { "i" => 2 } } #: Hash[String, Hash[String, Integer]]
      assert_equal true, (hh1 == hh2)
      assert_equal false, (hh1 == hh3)
      b["y"] = 3
      assert_equal false, (a == b)
      assert_equal true, (b == d)
      b.delete("y")
      assert_equal true, (b == c)
      sa = { a: 1, b: "two" } #: Hash[Symbol, untyped]
      sb = { b: "two", a: 1 } #: Hash[Symbol, untyped]
      sc = { a: 1, b: "TWO" } #: Hash[Symbol, untyped]
      assert_equal true, (sa == sb)
      assert_equal false, (sa == sc)
      u1 = { 1 => "x", "k" => :s }
      u2 = { "k" => :s, 1 => "x" }
      assert_equal true, (u1 == u2)
      assert_equal false, (u1 == { 1 => "x" })
      assert_equal true, ([{ "x" => 1 }] == [{ "x" => 1 }])
      assert_equal false, ([c] == [a])
      o1 = Obj.new
      o2 = Obj.new
      assert_equal true, ({ "o" => o1 } == { "o" => o1 })
      assert_equal false, ({ "o" => o1 } == { "o" => o2 })
      assert_equal true, ([c].include?({ "x" => 1 }))
      assert_equal false, ([c].include?({ "x" => 9 }))
      assert_equal false, c.equal?(c.merge({}))
      assert_equal true, (c == c.merge({}))
      assert_equal true, (a == a.select { |_k, _v| true })
      assert_equal false, (a.equal?(a.select { |_k, _v| true }))
    end
  end

  class HashFetchErrorsTest < Minitest::Test
    def test_section_0
      h = { "a" => 1 } #: Hash[String, Integer]
      assert_equal "1", (hash_safe_fetch(h, "a"))
      assert_equal "rescued: key not found: \"zz\"", (hash_safe_fetch(h, "zz"))
      assert_equal "rescued: key not found: \"\"", (hash_safe_fetch(h, ""))
      assert_equal "rescued: key not found: \"é\\n\"", (hash_safe_fetch(h, "é\n"))
      e = assert_raises(KeyError) { { a: 1 }.fetch(:nope) }
      assert_equal "key not found: :nope", e.message
      assert_equal KeyError, e.class
      ints = { 1 => "x" } #: Hash[Integer, String]
      se = assert_raises(StandardError) { ints.fetch(-7) }
      assert_equal "key not found: -7", se.message
      assert_equal KeyError, se.class
      assert_equal true, se.is_a?(IndexError)
      assert_equal true, se.is_a?(StandardError)
      assert_equal 2, h["a"].succ
      ne = assert_raises(NoMethodError) { h["zz"].succ }
      assert_equal NoMethodError, ne.class
      flags = { "on" => true, "off" => false } #: Hash[String, bool]
      assert_equal false, flags["off"]
      assert_nil flags["missing"]
      assert_equal false, flags.fetch("off")
      assert_equal true, (flags.fetch("on", false))
      assert_equal false, (flags.fetch("off", true))
      assert_equal true, (flags.fetch("missing", true))
      assert_equal false, (flags.fetch("missing", false))
      u = { "a" => 1 } #: Hash[untyped, Integer]
      assert_equal "key not found: nil", (try_fetch(u, nil))
      assert_equal "key not found: 1.5", (try_fetch(u, 1.5))
      assert_equal "key not found: :\"a b\"", (try_fetch(u, :"a b"))
      assert_equal "key not found: [1, \"x\"]", (try_fetch(u, [1, "x"]))
      assert_equal "1", (try_fetch(u, "a"))
      tk = { [1, "a"] => 1 } #: Hash[[Integer, String], Integer]
      e = assert_raises(KeyError) { tk.fetch([2, "b"]) }
      assert_equal "key not found: [2, \"b\"]", e.message
      sk = { a: 1 } #: Hash[Symbol, Integer]
      e = assert_raises(KeyError) { sk.fetch(:b?) }
      assert_equal "key not found: :b?", e.message
      ie = assert_raises(IndexError) { u.fetch(:zz) }
      assert_equal "KeyError: key not found: :zz", "#{ie.class}: #{ie.message}"
      x = u.fetch("zz") rescue -1
      assert_equal -1, x
      # formerly the uncaught KeyError that ended the file
      e = assert_raises(KeyError) { ints.fetch(42) }
      assert_equal "key not found: 42", e.message
    end
  end

  class HashInspectTest < Minitest::Test
    # Decision 34: symbol keys print as labels when they are identifiers, quoted otherwise.
    def test_decision_34_symbol_keys_print
      syms = { :+ => 1, :"é" => 4, :A => 5, :_ => 6, :"9a" => 7, :"a-b" => 8, :"" => 9, :"a!" => 10, :"A?" => 11, :@iv => 12, :$g => 13, :"a\nb" => 14, :[] => 15, :"日本" => 16, :"foo=" => 17, :"Foo=" => 18, :nil => 19, :if => 20, :__ => 21, :"a?b" => 22, :"ǅ" => 24, :"a٣" => 26, :"a b" => 27 } #: Hash[Symbol, Integer]
      assert_equal "{\"+\": 1, é: 4, A: 5, _: 6, \"9a\": 7, \"a-b\": 8, \"\": 9, a!: 10, A?: 11, \"@iv\": 12, \"$g\": 13, \"a\\nb\": 14, \"[]\": 15, 日本: 16, \"foo=\": 17, \"Foo=\": 18, nil: 19, if: 20, __: 21, \"a?b\": 22, ǅ: 24, a٣: 26, \"a b\": 27}", (syms).inspect
      assert_equal "{port: 1, \"a b\": 2, \"+\": 3, ok?: 4, \"s\" => 5, \"set=\": 6}", (({ port: 1, "a b": 2, :+ => 3, ok?: 4, "s" => 5, "set=": 6 })).inspect
    end

    # String keys and values go through String#inspect escaping.
    def test_string_keys_and_values_go
      strs = { "héllo" => "wörld", "日本" => "語", "😀" => "", "tab\there" => "q\"uote", "nl\n" => "back\\slash", "" => "\e", "#" => "\#{x}" } #: Hash[String, String]
      assert_equal "{\"héllo\" => \"wörld\", \"日本\" => \"語\", \"😀\" => \"\", \"tab\\there\" => \"q\\\"uote\", \"nl\\n\" => \"back\\\\slash\", \"\" => \"\\e\", \"#\" => \"\\\#{x}\"}", (strs).inspect
      assert_equal "{\"f\" => 1.5, \"g\" => 100.0, \"h\" => 1.0e-05, \"i\" => -0.0, \"j\" => 1.0e+20, \"k\" => 123456789.125}", (({ "f" => 1.5, "g" => 100.0, "h" => 1.0e-5, "i" => -0.0, "j" => 1e20, "k" => 123456789.125 })).inspect
      assert_equal "{3 => \"c\", -1 => \"neg\", 0 => \"zero\", 4611686018427387904 => \"big\"}", (({ 3 => "c", -1 => "neg", 0 => "zero", 4_611_686_018_427_387_904 => "big" })).inspect
      assert_equal "{true => \"t\", false => \"f\"}", (({ true => "t", false => "f" })).inspect
      assert_equal "{0.5 => :half, -2.0 => :neg}", (({ 0.5 => :half, -2.0 => :neg })).inspect
      e = {} #: Hash[String, Integer]
      assert_equal "{}", (e).inspect
      assert_equal "{}", e.to_s
      assert_equal "{\"x\" => {\"y\" => {\"z\" => {}}}, \"e\" => {}}", (({ "x" => { "y" => { "z" => {} } }, "e" => {} })).inspect
      assert_equal "{\"list\" => [1, [2, []]], \"empty\" => []}", (({ "list" => [1, [2, []]], "empty" => [] })).inspect
      assert_equal "[{\"a\" => 1}, {}, {b: :c}]", (([{ "a" => 1 }, {}, { b: :c }])).inspect
      mixed = { 1 => :a, 1.0 => :b, nil => 3, true => 4, [1, 2] => 5, "s" => 6, s: 7 }
      assert_equal "{1 => :a, 1.0 => :b, nil => 3, true => 4, [1, 2] => 5, \"s\" => 6, s: 7}", (mixed).inspect
      assert_equal 7, mixed.size
      assert_equal "{port: 8080, host: \"h\", on: true, none: nil, list: [1, 2], sub: {k: \"v\"}}", (({ port: 8080, host: "h", on: true, none: nil, list: [1, 2], sub: { k: "v" } })).inspect
      h = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
      assert_equal "{\"b\" => 20, \"c\" => 30}", h.to_s
      assert_equal "{\"b\" => 20, \"c\" => 30}", (h).to_s
      assert_equal "interp: {\"b\" => 20, \"c\" => 30} and {}", ("interp: #{h} and #{e}")
      assert_equal 22, h.inspect.size
      assert_equal true, (h.inspect == h.to_s)
    end

    # Values keep their own inspect: quoted symbols, nil, nested arrays.
    def test_values_keep_their_own_inspect
      h = { "b" => 20, "c" => 30 } #: Hash[String, Integer]
      assert_equal "{a: :\"b c\", b: :+, c: :\"9\", d: :ok?}", (({ a: :"b c", b: :+, c: :"9", d: :ok? })).inspect
      assert_equal "{\"a\\\"b\": 1, \"\\\\\": 2, \"a'b\": 3}", (({ :"a\"b" => 1, :"\\" => 2, :"a'b" => 3 })).inspect
      assert_equal "{\"x\" => nil}", (({ "x" => nil })).inspect
      assert_equal "{nil => nil}", (({ nil => nil })).inspect
      assert_equal "{\"t\" => [1, \"a\", :s, nil, 1.5, true]}", (({ "t" => [1, "a", :s, nil, 1.5, true] })).inspect
      assert_equal "[{\"b\" => 20, \"c\" => 30}]", [h].to_s
      assert_equal "[{\"b\" => 20, \"c\" => 30}] {x: [{\"b\" => 20, \"c\" => 30}]}", ("#{[h]} #{{ x: [h] }}")
    end
  end

  class HashIterationTest < Minitest::Test
    def test_each_forms
      h = { "a" => 1, "b" => 5, "c" => -2, "d" => 7 } #: Hash[String, Integer]
      out = [] #: Array[String]
      h.each { |k, v| out << "each #{k}=#{v}" }
      assert_equal ["each a=1", "each b=5", "each c=-2", "each d=7"], out
      out = []
      h.each { |pair| out << pair.inspect }
      assert_equal ["[\"a\", 1]", "[\"b\", 5]", "[\"c\", -2]", "[\"d\", 7]"], out
      out = []
      h.each_pair { |k, v| out << "pair #{k}=#{v}" }
      assert_equal ["pair a=1", "pair b=5", "pair c=-2", "pair d=7"], out
      out = []
      h.each_key { |k| out << k }
      assert_equal ["a", "b", "c", "d"], out
      ints = [] #: Array[Integer]
      h.each_value { |v| ints << v }
      assert_equal [1, 5, -2, 7], ints
      assert_equal "b", (first_over(h, 4))
      assert_nil (first_over(h, 100))
      assert_nil (first_over({}, 0))
      assert_equal 6, sum_until_negative(h)
      assert_equal 0, sum_until_negative({})
      assert_equal ["a", "b", "c", "d"], keys_via_each_key(h)
      out = []
      h.each_pair do |k, v|
        next if v.odd?

        out << "even #{k}"
      end
      h.each_key do |k|
        next unless k > "b"

        out << "late #{k}"
      end
      assert_equal ["even c", "late c", "late d"], out
      found = nil #: String?
      h.each do |k, v|
        if v == 7
          found = k
          break
        end
      end
      assert_equal "d", found
      acc = 0
      h.each_with_index do |pair, i|
        acc += i * 10 + pair[1]
        break if pair[1] < 0
      end
      assert_equal 34, acc
      out = []
      h.each_with_index { |pair, i| out << "#{i}:#{pair[0]}" }
      h.each_entry { |pair| out << pair[0] }
      assert_equal ["0:a", "1:b", "2:c", "3:d", "a", "b", "c", "d"], out
      out = []
      h.each do |pair|
        k, v = pair
        out << "#{pair.first}#{pair.last} #{k}#{v}"
      end
      assert_equal ["a1 a1", "b5 b5", "c-2 c-2", "d7 d7"], out
      assert_equal ["a", "b", "c", "d"], h.map(&:first)
      assert_equal [1, 5, -2, 7], h.map(&:last)
      e = {} #: Hash[String, Integer]
      out = []
      e.each { |k, _v| out << "never #{k}" }
      e.each_key { |k| out << "never #{k}" }
      e.each_value { |v| out << "never #{v}" }
      e.each_pair { |k, _v| out << "never #{k}" }
      assert_equal [], out
    end

    def test_order_after_delete_and_growth
      ord = {} #: Hash[Integer, String]
      [5, 3, 9, 1].each { |n| ord[n] = n.to_s }
      ord.delete(3)
      ord[3] = "three"
      ord[5] = "five"
      out = [] #: Array[String]
      ord.each { |k, v| out << "#{k}#{v}" }
      assert_equal ["5five", "99", "11", "3three"], out
      big = {} #: Hash[Integer, Integer]
      i = 0
      while i < 20_000
        big[(i * 7919) % 20_000] = i
        i += 1
      end
      assert_equal 20000, big.size
      assert_equal [0, 7919, 15838, 3757, 11676], big.keys.first(5)
      assert_equal [0, 1, 2, 3, 4], big.values.first(5)
      j = 0
      while j < 20_000
        big.delete(j) if j % 3 != 0
        j += 1
      end
      assert_equal 6667, big.size
      assert_equal [[0, 0], [11676, 4], [11271, 9], [10866, 14]], big.first(4)
      total = 0
      big.each_key { |k| total += k }
      assert_equal 66663333, total
      last = nil #: Integer?
      big.each_key { |k| last = k }
      assert_equal 12081, last
    end

    def test_named_blocks_and_mutation
      w = { "a" => 1, "bb" => 2, "ccc" => 3 } #: Hash[String, Integer]
      out = [] #: Array[String]
      hash_walk(w) { |k, v| out << "#{k}:#{v}" }
      hash_walk(w) { |pair| out << pair.inspect }
      walk_pairs(w) { |k, v| out << "#{k}-#{v}" }
      walk_yield(w) { |k, v| out << "#{k}+#{v}" }
      assert_equal ["a:1", "bb:2", "ccc:3", "[\"a\", 1]", "[\"bb\", 2]", "[\"ccc\", 3]", "a-1", "bb-2", "ccc-3", "a+1", "bb+2", "ccc+3"], out
      log = [] #: Array[String]
      assert_equal "bb", find_big(w, log)
      assert_equal "none", find_big({}, log)
      assert_equal ["ensure", "ensure"], log
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], (w.map { _1 })
      assert_equal ["a=1", "bb=2", "ccc=3"], (w.map { "#{_1}=#{_2}" })
      assert_equal [["a", 1], ["bb", 2], ["ccc", 3]], (w.map { it })
      assert_equal [["ccc", 3], ["bb", 2], ["a", 1]], (w.sort_by { -it[1] })
      out = []
      w.each { out << it.inspect }
      assert_equal ["[\"a\", 1]", "[\"bb\", 2]", "[\"ccc\", 3]"], out
      e = assert_raises(ArgumentError) do
        w.each { |k, v| raise ArgumentError, k if v > 1 }
      end
      assert_equal "bb", e.message
      out = []
      w.each do |k, v|
        begin
          raise "x#{k}" if v.odd?

          out << "ok #{k}"
        rescue => re
          out << re.message
        end
      end
      assert_equal ["xa", "ok bb", "xccc"], out
      w.each { |k, v| w[k] = v * 10 }
      assert_equal "{\"a\" => 10, \"bb\" => 20, \"ccc\" => 30}", (w).inspect
      w.each_key { |k| w[k] = (w[k] || 0) + 1 }
      assert_equal "{\"a\" => 11, \"bb\" => 21, \"ccc\" => 31}", (w).inspect
      w.keys.each { |k| w.delete(k) if k != "bb" }
      assert_equal "{\"bb\" => 21}", (w).inspect
      w.clear
      out = []
      w.each { |k, _v| out << "never #{k}" }
      w["z"] = 1
      w.each_pair { |k, v| out << "#{k}#{v}" }
      assert_equal ["z1"], out
      snap = w.map { |k, v| w[k] = v + 1; k }
      assert_equal ["z"], snap
      assert_equal "{\"z\" => 2}", (w).inspect
    end
  end

  class HashMidTest < Minitest::Test
    # Array and Hash keys match by value, not identity
    def test_array_and_hash_keys_match
      ak = {} #: Hash[Array[Integer], String]
      ak[[1, 2]] = "one-two"
      assert_equal "one-two", (ak[[1, 2]])
      assert_equal true, (ak.key?([1, 2]))
      ak[[1, 2]] = "again"
      assert_equal 1, ak.size
      assert_equal "{[1, 2] => \"again\"}", (ak).inspect
      assert_equal "{[1] => 2, [2] => 1}", (([[1], [1], [2]].tally)).inspect
      assert_equal "{[1] => [1, 3], [0] => [2, 4]}", (([1, 2, 3, 4].group_by { |gn| [gn % 2] })).inspect
      hk = {} #: Hash[Hash[String, Integer], Integer]
      hk[{ "a" => 1 }] = 1
      assert_equal 1, (hk[{ "a" => 1 }])
      assert_equal true, (hk.key?({ "a" => 1 }))
      ek_a = { "a" => 1 } #: Hash[String, Integer]
      ek_b = { "a" => 1 } #: Hash[String, Integer]
      assert_equal 1, ([ek_a, ek_b].uniq.size)
      assert_equal 1, ([ek_a, ek_b].tally.size)
      assert_equal 1, ([ek_a, ek_b].group_by { |x| x }.size)
    end

    # delete/clear inside each follows MRI's iteration order
    def test_delete_clear_inside_each_follows
      del_h = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
      out = [] #: Array[String]
      del_h.each do |k, v|
        out << "visit #{k}"
        del_h.delete(k) if v.even?
      end
      assert_equal ["visit a", "visit b", "visit c", "visit d"], out
      assert_equal "{\"a\" => 1, \"c\" => 3}", (del_h).inspect
      g = { "a" => 1, "b" => 2, "c" => 3, "d" => 4 } #: Hash[String, Integer]
      out = []
      g.each do |k, v|
        out << "g #{k} #{v}"
        g.delete("c") if k == "a"
      end
      assert_equal ["g a 1", "g b 2", "g d 4"], out
      assert_equal "{\"a\" => 1, \"b\" => 2, \"d\" => 4}", (g).inspect
      c = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      out = []
      c.each_value do |v|
        out << "c #{v}"
        c.clear
      end
      assert_equal ["c 1"], out
      assert_equal "{}", (c).inspect
      nest = { "a" => { "b" => 1 } } #: Hash[String, Hash[String, Integer]]
      assert_equal "{}", ((nest.fetch("x", {}))).inspect
      assert_equal "{\"b\" => 1}", ((nest.fetch("a", {}))).inspect
      assert_equal 0, count_or({})
      assert_equal 1, (count_or({ "a" => 1 }))
      assert_equal -1, count_or
      assert_equal "{a: 1}", (opts_or(a: 1))
      assert_equal "none", opts_or
      assert_equal "{}", (hash_maybe(true)).inspect
      assert_nil hash_maybe(false)
      m = {} #: Hash[String, Integer]?
      assert_equal "{}", (m).inspect
      nar_h = { "a" => 1 } #: Hash[String, Integer]
      add_key(nar_h)
      assert_equal "{\"a\" => 1, \"new\" => 1}", (nar_h).inspect
      assert_equal 3, hash_grow(nar_h)
      assert_equal 3, nar_h.size
      ins_inferred = { "a" => nil, "b" => 2 }
      assert_equal "{\"a\" => nil, \"b\" => 2}", (ins_inferred).inspect
      opt = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
      assert_equal "{\"a\" => nil, \"b\" => 2}", (opt).inspect
      assert_equal "{\"a\" => nil, \"b\" => 2}", opt.to_s
      nil_key = { nil => 1, "a" => 2 } #: Hash[String?, Integer]
      assert_equal "{nil => 1, \"a\" => 2}", (nil_key).inspect
      objs = { "f" => Foo.new } #: Hash[String, Foo?]
      assert_equal "{\"f\" => #<Foo>}", (objs).inspect
      lists = { "l" => [1] } #: Hash[String, Array[Integer]?]
      assert_equal "{\"l\" => [1]}", (lists).inspect
    end

    # a stored nil and a missing key both read as nil
    def test_a_stored_nil_and_a
      look_h = { "a" => nil, "b" => 2 } #: Hash[String, Integer?]
      look_v = look_h["a"]
      assert_equal true, look_v.nil?
      assert_equal true, look_h["a"].nil?
      assert_equal true, look_h["zz"].nil?
      assert_equal false, look_h["b"].nil?
      branch = ""
      if look_h["a"]
        branch = "truthy"
      else
        branch = "falsy"
      end
      assert_equal "falsy", branch
      assert_equal "7", (look_h["a"] || 7).to_s
      look_inferred = { "x" => nil, "y" => 1 }
      assert_equal true, look_inferred["x"].nil?
      d = look_h.delete("a")
      assert_equal true, d.nil?
      assert_equal 1, look_h.size
      assert_nil look_h["a"]
      look_h["c"] = nil
      assert_nil look_h.delete("c")
    end

    # collections built from untyped lookups keep working as values
    def test_collections_built_from_untyped_lookups
      people = [{ name: "a", age: 30 }, { name: "b", age: 20 }]
      names = people.map { |p| p[:name] }
      assert_equal 2, names.size
      assert_equal "a,b", names.join(",")
      assert_equal true, (names == ["a", "b"])
      assert_equal true, names.include?("a")
      cfg = { port: 8080, host: "h" } #: Hash[Symbol, untyped]
      pair = [cfg[:port], cfg[:host]]
      assert_equal 2, pair.size
      assert_equal [8080, "h"], pair
      assert_equal ["a", "b"], names
      picked = { "p" => cfg[:port], "h" => cfg[:host] }
      assert_equal 2, picked.size
      assert_equal "{\"p\" => 8080, \"h\" => \"h\"}", (picked).inspect
      ti = { a: 1 } #: Hash[Symbol, Integer]
      assert_equal 1, opts_size(ti)
      typed = { "n" => 1 } #: Hash[String, Integer]
      assert_equal 1, hash_size(typed)
      loose = {}
      loose["a"] = 2
      assert_equal 2, values_total(loose)
      groups = {} #: Hash[String, Array[Integer]]
      [["a", 1], ["b", 2], ["a", 3]].each { |k, v| (groups[k] ||= []) << v }
      assert_equal "{\"a\" => [1, 3], \"b\" => [2]}", (groups).to_s
      memo = {} #: Hash[Integer, String]
      assert_equal "one", (memo[1] ||= "one")
      assert_equal "one", (memo[1] ||= "uno")
      assert_equal "{1 => \"one\"}", (memo).to_s
      arr = [1, 2, 3]
      arr[0] += 10
      arr[-1] *= 2
      assert_equal 1, (arr[1] -= 1)
      assert_equal [11, 1, 6], arr
      counts = { "x" => 0 }
      counts["x"] += 5
      assert_equal "{\"x\" => 5}", (counts).to_s
      nested = {} #: Hash[Symbol, Hash[Symbol, Integer]]
      (nested[:a] ||= {})[:b] = 1
      assert_equal "{a: {b: 1}}", (nested).to_s
      flags = {} #: Hash[String, bool]
      flags["on"] ||= true
      assert_equal "{\"on\" => true}", (flags).to_s
      hs_h = { a: 1, b: 2, c: 3 }
      assert_equal "{a: 10, b: 20, c: 30}", ((hs_h.transform_values { |v| v * 10 })).to_s
      tv = { a: 1, b: 2 }
      assert_equal [{ a: 10, b: 20 }, true], [tv.transform_values! { |v| v * 10 }, tv.transform_values!(&:itself).equal?(tv)]
      assert_equal "{\"a\" => 1, \"b\" => 2, \"c\" => 3}", (hs_h.transform_keys(&:to_s)).to_s
      assert_equal "{1 => :a, 2 => :b, 3 => :c}", ((hs_h.to_h { |k, v| [v, k] })).to_s
      assert_equal "{a: 1, b: 2, c: 3}", (hs_h.to_h).to_s
      assert_equal "{1 => :a, 2 => :b, 3 => :c}", (hs_h.invert).to_s
      assert_equal :b, hs_h.key(2)
      assert_nil hs_h.key(9)
      assert_equal true, hs_h.value?(3)
      assert_equal false, hs_h.has_value?(0)
      assert_equal true, hs_h.member?(:a)
      assert_equal [1, nil], (hs_h.values_at(:a, :z))
      assert_equal [1, 2], (hs_h.fetch_values(:a, :b))
      assert_equal "{a: 1, c: 3}", ((hs_h.slice(:a, :c, :z))).to_s
      assert_equal "{a: 1, c: 3}", (hs_h.except(:b)).to_s
      assert_equal 4, (hs_h.store(:d, 4))
      assert_equal "{a: 1, b: 2, c: 3, d: 4}", (hs_h).to_s
      assert_equal "{a: 101, b: 2, c: 3, d: 4, e: 5}", ((hs_h.merge({ a: 100, e: 5 }) { |k, old, new| old + new })).to_s
      assert_equal 2, (hs_h.count { |k, v| v.odd? })
      assert_equal true, (hs_h.any? { |k, v| v > 3 })
      assert_equal true, (hs_h.all? { |k, v| v > 0 })
      assert_equal true, (hs_h.none? { |k, v| v > 9 })
      hs_h2 = hs_h.merge({})
      hs_h2.delete_if { |k, v| v.even? }
      assert_equal "{a: 1, c: 3}", (hs_h2).to_s
      assert_equal "{a: 1, b: 2}", ((hs_h.reject { |k, v| v > 2 })).to_s
      assert_equal "{c: 3, d: 4}", ((hs_h.select { |k, v| v > 2 })).to_s
      hs_h.update({ z: 26 })
      hs_h.keep_if { |k, v| v != 1 }
      assert_equal "{b: 2, c: 3, d: 4, z: 26}", (hs_h).to_s
      assert_equal [[:z, 26], [:d, 4]], (hs_h.sort_by { |k, v| -v }.first(2))
      assert_equal [:b, 2], (hs_h.min_by { |k, v| v })
      assert_equal 35, (hs_h.sum { |k, v| v })
      assert_equal "b=2&c=3&d=4&z=26", (hs_h.map { |k, v| "#{k}=#{v}" }.join("&"))
      assert_equal [:d, 4], (hs_h.find { |k, v| v > 3 })
      assert_equal [[:b, 2], [:c, 3], [:d, 4], [:z, 26]], hs_h.sort
      assert_equal [:z, 26], (hs_h.max_by { |k, v| v })
      assert_equal [:z, 26], hs_h.to_a.last
      assert_equal ["0:b", "1:c", "2:d", "3:z"], (hs_h.each_with_index.map { |(k, v), i| "#{i}:#{k}" })
    end
  end

  class HashUntypedTest < Minitest::Test
    # Decision 13: an unannotated `{}` is Hash[untyped, untyped].
    def test_decision_13_an_unannotated_is
      loose = {}
      loose["a"] = 1
      loose[:b] = "two"
      loose[3] = [3]
      loose[nil] = nil
      loose[2.5] = { x: 1 }
      assert_equal "{\"a\" => 1, b: \"two\", 3 => [3], nil => nil, 2.5 => {x: 1}}", (loose).inspect
      assert_equal 5, loose.size
      assert_equal 1, loose["a"]
      assert_equal "two", loose[:b]
      assert_equal [3], loose[3]
      assert_nil loose[:zz]
      assert_nil loose[nil]
      assert_equal true, loose.key?(nil)
      assert_equal false, loose.key?("b")
      loose.delete(:b)
      assert_equal "[\"a\", 3, nil, 2.5]", (loose.keys).inspect
      inferred = { "a" => 1, "b" => "x" }
      assert_equal "{\"a\" => 1, \"b\" => \"x\"}", (inferred).inspect
      assert_equal "x", inferred["b"]
      k_mixed = { 1 => "a", :b => "c" }
      assert_equal "{1 => \"a\", b: \"c\"}", (k_mixed).inspect
      assert_equal "c", k_mixed[:b]
      assert_equal "a", k_mixed[1]
      typed = { "n" => 1 } #: Hash[String, Integer]
      assert_equal "{\"n\" => 1}", hash_describe(typed)
      assert_equal 1, dyn_size(typed)
      assert_equal 0, any_size({})
      assert_equal 4, any_size(loose)
      assert_equal "{a: [1, {b: nil}]}", (hash_describe({ a: [1, { b: nil }] }))
      cfg = { port: 8080, host: "localhost", debug: false, ratio: 0.5 } #: Hash[Symbol, untyped]
      port = cfg[:port]
      assert_equal 8080, port
      assert_equal 8081, (cfg[:port] + 1)
      assert_equal "LOCALHOST", cfg[:host].upcase
      assert_equal false, cfg.fetch(:debug)
      assert_equal "dflt", (cfg.fetch(:nope, "dflt"))
      assert_equal [:port, :host, :ratio], (cfg.select { |_k, v| v }.keys)
      assert_equal "port:8080,host:localhost,debug:false,ratio:0.5", (cfg.map { |k, v| "#{k}:#{v}" }.join(","))
      assert_equal "{port: 8080, host: \"localhost\", debug: false}", ((cfg.reject { |k, _v| k == :ratio })).inspect
      cfg[:port] = "now a string"
      assert_equal "now a string", cfg[:port]
      assert_equal 4, cfg.size
    end

    # Dynamic calls on a typed hash held as untyped (decision 32).
    def test_dynamic_calls_on_a_typed
      th = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      u = hash_ident(th)
      assert_equal 2, u.size
      assert_equal "{\"a\" => 1, \"b\" => 2}", (u).inspect
      assert_equal "{\"a\" => 1, \"b\" => 2}", u.to_s
      assert_equal 2, u.length
      assert_equal 2, u.count
      assert_equal 1, lookup(th)
      assert_equal true, has_a(th)
      assert_equal false, has_a({})
      assert_equal 2, u.fetch("b")
      assert_equal 0, (u.fetch("zz", 0))
      assert_equal false, u.empty?
      assert_equal true, u.include?("b")
      assert_equal false, u.has_key?("zz")
      assert_nil u.delete("zz")
      assert_equal true, (u == th)
      assert_equal true, (th == u)
      assert_equal true, u.is_a?(Hash)
      assert_equal "hash 2 [\"a\", \"b\"]", hash_untyped_kind(th)
      assert_equal "array 1", hash_untyped_kind([1])
      assert_equal "other 1", hash_untyped_kind(1)
      assert_equal "hash 0 []", hash_untyped_kind({})
      assert_equal 2, size_if_hash(th)
      assert_equal -1, size_if_hash("s")
      u["c"] = 3
      assert_equal "{\"a\" => 1, \"b\" => 2, \"c\" => 3}", (th).inspect
    end

    # Untyped values flow into typed parameters.
    def test_untyped_values_flow_into_typed
      cfg2 = { port: 8080, host: "h", nested: { a: 1 } } #: Hash[Symbol, untyped]
      assert_equal 8081, inc(cfg2.fetch(:port))
      assert_equal "H", hash_up(cfg2.fetch(:host))
      pv = cfg2[:port]
      got = 0
      got = inc(pv) if pv
      assert_equal 8081, got
      nested = cfg2.fetch(:nested)
      assert_equal "{a: 1}", (nested).inspect
      assert_equal 1, nested.size
      assert_equal 1, nested[:a]
    end

    # Iterating an untyped hash: each key and value keeps its own type.
    def test_iterating_an_untyped_hash_each
      mixed = { "s" => 1, 2 => :two, nil => nil, [1] => 1.5 }
      out = [] #: Array[String]
      mixed.each { |k, v| out << "#{k.inspect}=#{v.inspect}" }
      assert_equal ["\"s\"=1", "2=:two", "nil=nil", "[1]=1.5"], out
      assert_equal "{\"s\" => 1}", ((mixed.select { |k, _v| k.is_a?(String) })).inspect
      assert_equal [false, false, true, false], (mixed.map { |k, _v| k.nil? })
      assert_equal 4, mixed.count
      assert_equal 4, mixed.keys.size
      inferred_nil = { "a" => 1, "b" => nil }
      out = []
      inferred_nil.each { |k, v| out << "#{k} #{v.nil?}" }
      assert_equal ["a false", "b true"], out
      assert_equal ["a", "b"], inferred_nil.keys
      assert_equal 2, inferred_nil.size
    end

    # Integer and Float values that are == compare equal inside Hash#==.
    def test_integer_and_float_values_that
      assert_equal true, ({ "n" => 1, "s" => "x" } == { "n" => 1.0, "s" => "x" })
      c1 = { "n" => 1 } #: Hash[String, untyped]
      c2 = { "n" => 1.0 } #: Hash[String, untyped]
      assert_equal true, (c1 == c2)
      assert_equal false, (c1 == { "n" => 2.0 })
    end

    # fetch on untyped values: stored false/nil come back, a false default is used.
    def test_fetch_on_untyped_values_stored
      flags = { debug: false, name: nil } #: Hash[Symbol, untyped]
      assert_equal false, flags.fetch(:debug)
      assert_nil flags.fetch(:name)
      assert_equal false, (flags.fetch(:nope, false))
    end
  end

  class HashValuesKeysTest < Minitest::Test
    def test_section_0
      r = Registry.new
      r.add(Animal.new("cat")).add(Dog.new("rex"))
      assert_equal ["cat", "rex"], r.names
      assert_equal "cat:... rex:woof", r.voices
      r.items["bob"] = Dog.new("bob")
      assert_equal 3, r.items.size
      assert_equal ["cat", "rex", "bob"], r.names
      assert_equal "woof", r.items["rex"]&.speak
      assert_nil r.items["zz"]&.speak
    end

    # Plain objects are keys by identity, in MRI too.
    def test_plain_objects_are_keys_by
      a1 = Animal.new("x")
      a2 = Animal.new("x")
      ids = {} #: Hash[Animal, Integer]
      ids[a1] = 1
      ids[a2] = 2
      ids[a1] = 3
      assert_equal 2, ids.size
      assert_equal 3, ids[a1]
      assert_equal 2, ids[a2]
      assert_equal false, ids.key?(Animal.new("x"))
      same = Point.new(x: 1, y: 1)
      by_id = { same => 1 } #: Hash[Point, Integer]
      assert_equal 1, by_id[same]
    end

    # Tuple keys compare by value.
    def test_tuple_keys_compare_by_value
      tk = {} #: Hash[[Integer, String], Integer]
      tk[[1, "a"]] = 1
      tk[[1, "a"]] = 2
      tk[[2, "a"]] = 3
      assert_equal 2, tk.size
      assert_equal 2, (tk[[1, "a"]])
      assert_nil (tk[[1, "b"]])
      assert_equal "{[1, \"a\"] => 2, [2, \"a\"] => 3}", (tk).inspect
      pairs = [[1, "a"], [1, "a"], [2, "b"]]
      assert_equal "{[1, \"a\"] => 2, [2, \"b\"] => 1}", (pairs.tally).inspect
      words = %w[apple avocado banana blueberry cherry]
      assert_equal "{[5, \"a\"] => [\"apple\"], [7, \"a\"] => [\"avocado\"], [6, \"b\"] => [\"banana\"], [9, \"b\"] => [\"blueberry\"], [6, \"c\"] => [\"cherry\"]}", ((words.group_by { |w| [w.size, w[0]] })).inspect
    end

    # Keys built at run time find literal keys.
    def test_keys_built_at_run_time
      sk = { "ab" => 1, ab: 2 } #: Hash[untyped, Integer]
      x = "a"
      assert_equal 1, (sk[x + "b"])
      assert_equal 1, sk["#{x}b"]
      assert_equal 2, (sk[(x + "b").to_sym])
      assert_nil sk[x]
    end

    # Values with their own inspect.
    def test_values_with_their_own_inspect
      pts = { "o" => Point.new(x: 0, y: 0), "p" => Point.new(x: 1, y: -2) } #: Hash[String, Point]
      assert_equal "{\"o\" => #<data HashTests::Point x=0, y=0>, \"p\" => #<data HashTests::Point x=1, y=-2>}", (pts).inspect
      assert_equal [0, 1], pts.values.map(&:x)
      assert_equal true, (pts == { "o" => Point.new(x: 0, y: 0), "p" => Point.new(x: 1, y: -2) })
      assert_equal false, (pts == { "o" => Point.new(x: 0, y: 1), "p" => Point.new(x: 1, y: -2) })
      pr = { k: Pair.new("s", 1) } #: Hash[Symbol, Pair]
      assert_equal "{k: #<struct HashTests::Pair a=\"s\", b=1>}", (pr).inspect
      assert_equal 1, pr[:k]&.b
      tg = { Tag.new("k") => Tag.new("v") } #: Hash[Tag, Tag]
      assert_equal "{<k> => <v>}", (tg).inspect
      mixed = { 1 => Tag.new("t"), 2 => Point.new(x: 3, y: 4) } #: Hash[Integer, untyped]
      assert_equal "{1 => <t>, 2 => #<data HashTests::Point x=3, y=4>}", (mixed).inspect
      assert_equal 9, limit("high")
      assert_equal 0, limit("mid")
      assert_equal 2, LIMITS.size
      assert_equal "{\"low\" => 1, \"high\" => 9}", (LIMITS).inspect
      assert_equal 11, hash_bump
      assert_equal 11, hash_bump
      shared = { "y" => 5 } #: Hash[String, Integer]
      assert_equal 21, hash_bump(shared)
      assert_equal 22, hash_bump(shared)
      assert_equal "{\"y\" => 5, \"x\" => 2}", (shared).inspect
    end

    # An array of hashes: hashes are shared, so each's writes stick.
    def test_an_array_of_hashes_hashes
      rows = [{ "n" => 3, "k" => 1 }, { "n" => 1, "k" => 2 }, { "n" => 2, "k" => 3 }] #: Array[Hash[String, Integer]]
      assert_equal [2, 3, 1], (rows.sort_by { |row| row.fetch("n") }.map { |row| row.fetch("k") })
      rows.each { |row| row["n"] = row.fetch("n") + 10 }
      assert_equal "[{\"n\" => 13, \"k\" => 1}, {\"n\" => 11, \"k\" => 2}, {\"n\" => 12, \"k\" => 3}]", (rows).inspect
      assert_equal 2, (rows.select { |row| row.fetch("k").odd? }.size)
      assert_equal "{\"n\" => 13, \"k\" => 1}", ((rows.max_by { |row| row.fetch("n") })).inspect
      people = [{ name: "a", age: 30 }, { name: "b", age: 20 }] #: Array[Hash[Symbol, untyped]]
      assert_equal ["a", "b"], (people.map { |p| p.fetch(:name) })
    end

    # A hash of hashes: buckets created on demand, then written through fetch.
    def test_a_hash_of_hashes_buckets
      reg = {} #: Hash[String, Hash[String, Integer]]
      %w[x y x].each_with_index do |name, i|
        inner = reg[name]
        if inner
          inner["n#{i}"] = i
        else
          reg[name] = { "n#{i}" => i }
        end
      end
      assert_equal "{\"x\" => {\"n0\" => 0, \"n2\" => 2}, \"y\" => {\"n1\" => 1}}", (reg).inspect
      reg.fetch("x")["z"] = 99
      assert_equal "{\"n0\" => 0, \"n2\" => 2, \"z\" => 99}", (reg["x"]).inspect
      assert_equal [["x", 3], ["y", 1]], (reg.map { |k, v| [k, v.size] })
      assert_equal [3, 1], reg.values.map(&:size)
    end

    # A hash returned from a method.
    def test_a_hash_returned_from_a
      out = [] #: Array[String]
      lengths(%w[aa b]).each { |k, v| out << "#{k}=#{v}" }
      assert_equal ["aa=2", "b=1"], out
      assert_equal true, lengths([]).empty?
      if (n = lengths(%w[q]).fetch("q", 0)) > 0
        out << "got #{n}"
      end
      assert_equal ["aa=2", "b=1", "got 1"], out
      zz = lengths(%w[zz y])["zz"]
      got = 0
      got = zz + 1 if zz
      assert_equal 3, got
      m = Memo.new
      assert_equal 102334155, m.fib(40)
      assert_equal 41, m.cached
      s = Settings.new
      s.opts["color"] = "red"
      assert_equal "{\"mode\" => \"fast\", \"color\" => \"red\"}", (s.opts).inspect
      before = s.opts
      s.opts = { "x" => "y" }
      assert_equal "{\"x\" => \"y\"}", (s.opts).inspect
      assert_equal "{\"mode\" => \"fast\", \"color\" => \"red\"}", (before).inspect
      rec = Rec.new("n", { "t" => 1 })
      rec.tags["u"] = 2
      assert_equal "#<struct HashTests::Rec name=\"n\", tags={\"t\" => 1, \"u\" => 2}>", (rec).inspect
      assert_equal 2, rec.tags.size
      lazy = nil #: Hash[String, Integer]?
      lazy ||= {}
      lazy["k"] = 1
      lazy ||= { "never" => 0 }
      assert_equal "{\"k\" => 1}", (lazy).inspect
      maybe = nil #: Hash[String, Integer]?
      assert_nil maybe&.size
      assert_nil maybe
      maybe = { "q" => 1 }
      assert_equal 1, maybe&.size
      assert_equal 1, maybe&.fetch("q")
      pair = [{ "in" => 1 }, 2] #: [Hash[String, Integer], Integer]
      pair[0]["in2"] = 3
      assert_equal "{\"in\" => 1, \"in2\" => 3}", (pair[0]).inspect
      assert_equal 2, pair[1]
    end
  end

  # Hash#freeze (decision 96).
  class HashFreezeTest < Minitest::Test
    FROZEN = { a: 1 }.freeze

    def test_freeze
      assert_equal true, FROZEN.frozen?
      assert_equal false, {}.frozen?
      assert_equal "can't modify frozen Hash: {a: 1}", assert_raises(FrozenError) { FROZEN[:b] = 2 }.message
      assert_equal "can't modify frozen Hash: {a: 1}", assert_raises(FrozenError) { FROZEN.delete(:a) }.message
      assert_equal "can't modify frozen Hash: {a: 1}", assert_raises(FrozenError) { FROZEN.merge!({ c: 3 }) }.message
      assert_equal({ a: 1, z: 0 }, FROZEN.merge({ z: 0 }))
      assert_equal 1, FROZEN.fetch(:a)
    end

    # The flag lives on the object: an equal Hash and a merged copy stay mutable.
    def test_freeze_is_per_object
      a = { x: 1 }
      b = { x: 1 }
      a.freeze
      b[:y] = 2
      assert_equal [true, false, false], [a.frozen?, b.frozen?, a.merge({}).frozen?]
      assert_equal({ x: 1, y: 2 }, b)
    end
  end

  # ruby/spec core/hash and core/set gaps (#49)
  class HashRubySpecTest < Minitest::Test
    def test_hash_shift_assoc_replace
      h = { a: 1, b: 2 }
      assert_equal [:a, 1], h.shift
      assert_equal({ b: 2 }, h)
      assert_equal [:b, 2], h.assoc(:b)
      assert_nil h.assoc(:z)
      e = {} #: Hash[Symbol, Integer]
      assert_nil e.shift
      assert_same h, h.replace({ c: 3 })
      assert_equal({ c: 3 }, h)
      assert_same h, h.to_hash
    end

    def test_hash_literal_sugar
      a = 1
      b = "x"
      assert_equal({ a: 1, b: "x" }, { a:, b: })
      h = { a: 1, b: 2 }
      assert_equal({ a: 1, b: 2, c: 3 }, { **h, c: 3 })
      assert_equal({ z: 0, a: 9, b: 2 }, { z: 0, **h, a: 9 })
      assert_equal({ color: "red", size: "l" }, { **{ color: "red", size: "m" }, **{ size: "l" } })
    end

    def test_set_mutators
      s = Set[1, 2, 3, 4]
      assert_same s, s.delete_if(&:odd?)
      assert_equal Set[2, 4], s
      t = Set[1, 2, 3]
      t.keep_if(&:odd?)
      assert_equal Set[1, 3], t
      u = Set[1, 2, 3]
      u.subtract(Set[1])
      u.subtract([2])
      assert_equal Set[3], u
      u.replace(Set[7, 8])
      assert_equal Set[7, 8], u
      u.replace([9])
      assert_equal Set[9], u
      assert_equal ["1-2", "12"], [Set[1, 2].join("-"), Set[1, 2].join]
    end
  end

  # ruby/spec core/hash gaps (#49): compact
  class HashRubySpecCompactTest < Minitest::Test
    def test_compact
      h = { a: 1, b: nil, c: 3 } #: Hash[Symbol, Integer?]
      assert_equal [{ a: 1, c: 3 }, { a: 1, b: nil, c: 3 }], [h.compact, h]
      assert_same h, h.compact!
      assert_equal({ a: 1, c: 3 }, h)
      assert_nil h.compact!
      assert_equal({ x: "y" }, { x: "y" }.compact)
    end
  end

  # ruby/spec core/array and core/hash gaps (#49): dig
  class HashRubySpecDigTest < Minitest::Test
    def test_dig
      assert_equal [2, 5, nil, nil, 2], [[[1, [2, 3]]].dig(0, 1, 0), { a: { b: [5] } }.dig(:a, :b, 0), { a: 1 }.dig(:z), [1].dig(5), [1, 2].dig(-1)]
      e = assert_raises(TypeError) { [1].dig(0, 1) }
      assert_equal "Integer does not have #dig method", e.message
      assert_nil({ "x" => { "y" => nil } }.dig("x", "y", "z"))
    end
  end

  # ruby/spec core/hash gaps (#49): flatten
  class HashRubySpecFlattenTest < Minitest::Test
    def test_flatten
      h = { a: 1, b: [2, [3]] }
      assert_equal [[:a, 1, :b, [2, [3]]], [:a, 1, :b, 2, [3]], [:a, 1, :b, 2, 3], []], [h.flatten, h.flatten(2), h.flatten(3), {}.flatten]
    end
  end

  # ruby/spec core/set gaps (#49): flatten
  class HashRubySpecSetFlattenTest < Minitest::Test
    def test_set_flatten
      assert_equal [Set[1, 2, 3], Set[1, 2]], [Set[Set[1, 2], Set[2, 3]].flatten, Set[1, 2].flatten]
    end
  end

  # #86: a Hash subclass rewrites keys, then reaches the embedded Hash with super
  class HashSubclassTest < Minitest::Test
    def test_downcased_keys
      d = Downcaser.new
      d["X-Foo"] = 1
      d["Bar"] = 2
      assert_equal 1, d["x-foo"]
      assert_equal 2, d["BAR"]
      assert_equal ["x-foo", "bar"], d.keys
      assert_equal 2, d.size
      assert_equal true, d.key?("x-foo")
      assert_equal false, d.key?("X-Foo")
      assert_equal 1, d.fetch("x-foo", 0)
    end

    def test_alias_of_inherited_hash_method
      pr = AliasParams.new
      pr["a"] = 1
      h = pr.to_params_hash
      assert_equal({ "a" => 1 }, h)
      assert_equal Hash, h.class
      assert_equal ["a"], pr.plain_keys
      assert pr.respond_to?(:to_params_hash)
      refute({}.respond_to?(:to_params_hash))
    end
  end

  # #56: a container holding itself compares and hashes like MRI's, not a Go stack overflow
  class HashRecursiveTest < Minitest::Test
    def test_recursive_equality
      h = {} #: Hash[Symbol, untyped]
      h[:self] = h
      g = {} #: Hash[Symbol, untyped]
      g[:self] = g
      assert_equal [true, true, true, true], [h == h, h.eql?(h), h == g, h.eql?(g)]
      assert h.hash == h.hash
      a = [] #: Array[untyped]
      a << a
      b = [] #: Array[untyped]
      b << b
      assert_equal [true, true, true, false], [a == a, a.eql?(b), a == b, a == [a, 1]]
      assert a.hash == a.hash
      assert_equal false, h == { self: 1 }
      assert_equal ["[[...]]", "{self: {...}}", "[{self: {...}}, [[[...]]]]"], [a.inspect, h.inspect, [h, [a]].inspect]
      assert_equal [true, false], [[1, [2, { k: [3] }]] == [1, [2, { k: [3] }]], [1, [2]] == [1, [3]]]
    end
  end

  class HashBracketTest < Minitest::Test
    def test_bracket_forms
      assert_equal({ 1 => true, 2 => true }, Hash[[[1, true], [2, true]]])
      assert_equal({ a: 1, b: 2 }, Hash[:a, 1, :b, 2])
      assert_equal({ "x" => 1 }, Hash[{ "x" => 1 }])
      assert_equal({ ok: 200 }, Hash[*[[:ok, 200]].flatten])
    end

    def test_odd_arguments
      Hash[1, 2, 3]
      flunk "no error"
    rescue ArgumentError => e
      assert_equal "odd number of arguments for Hash", e.message
    end

    def test_dup_keeps_default_and_is_independent
      h = Hash.new(0) #: Hash[String, Integer]
      h["x"] = 1
      g = h.dup
      g["y"] = 2
      assert_equal [{ "x" => 1 }, { "x" => 1, "y" => 2 }, 0], [h, g, g["zz"]]
    end
  end

  class HashSubclassNewTest < Minitest::Test
    class Params < Hash
    end

    def test_new_through_a_class_object
      klass = Params #: singleton(Params)
      h = klass.new
      h["a"] = 1
      assert_equal [{ "a" => 1 }, true], [h.to_h, h.is_a?(Params)]
    end

    def test_untyped_empty_containers
      h = Hash.new(5)
      h.clear
      q = Queue.new
      assert_equal [5, true], [h.default, q.empty?]
    end

    def test_empty_new_typed_from_use
      h = Hash.new
      h["a"] = 1
      q = Queue.new
      q << 5
      a = Array.new
      a << :s
      assert_equal [{ "a" => 1 }, 5, [:s]], [h, q.pop, a]
    end

    # Hash.new(default) and a memo literal are typed by what the code stores in them (decision 344)
    def test_open_default_and_memo_typed_from_use
      nums = [1, 2, 3, 4, 5]
      h = Hash.new(0)
      nums.each { |x| h[x % 2] += 1 }
      g = nums.each_with_object(Hash.new(0)) { |n, acc| acc[n % 3] += 1 }
      e = nums.each_with_object({}) { |n, acc| acc[n] ||= n.to_s }
      t = nums.inject(Hash.new(0)) { |acc, n| acc[n.even?] += 1; acc }
      assert_equal [{ 1 => 3, 0 => 2 }, { 1 => 2, 2 => 2, 0 => 1 }, 5, "3", { false => 3, true => 2 }, 7],
                   [h, g, e.size, e.fetch(3), t, h.fetch(1) + g.fetch(2) + 2]
    end

    # pop(timeout:) with nil waits (concurrent-ruby's idle worker), an Integer or Float bounds it; clock_gettime's
    # units (decision 254)
    def test_queue_pop_timeout_and_clock_units
      q = Queue.new #: Queue[Integer]
      q << 7
      t = nil #: Float?
      got = [q.pop(timeout: t), q.pop(timeout: 0), q.pop(timeout: 0.01)]
      ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)
      fs = Process.clock_gettime(Process::CLOCK_MONOTONIC, :float_second)
      assert_equal [[7, nil, nil], Integer, Float], [got, ns.class, fs.class]
    end
  end

  # fetch with a block: its value for a missing key or index.
  class HashFetchBlockTest < Minitest::Test
    def test_fetch_block
      h = { a: 1 }
      assert_equal [1, 1, 10, 2], [h.fetch(:b) { |k| k.to_s.size }, h.fetch(:a) { 0 }, [1, 2].fetch(5) { |i| i * 2 }, [1, 2].fetch(-1) { 0 }]
    end
  end

  # merge starts from a dup, so default/default_proc survive; to_h is self.
  class HashMergeDefaultTest < Minitest::Test
    def test_merge_keeps_default_value
      h = Hash.new(5) #: Hash[Symbol, Integer]
      h[:a] = 1
      m = h.merge({ b: 2 })
      assert_equal [5, 2, 1, 2], [m[:zz], m[:b], m[:a], m.size]
      assert_equal 5, m.default
    end

    def test_merge_keeps_default_proc
      h = Hash.new { |_, k| k.to_s.size } #: Hash[Symbol, Integer]
      m = h.merge({ a: 1 })
      assert_equal [3, 1], [m[:zzz], m[:a]]
    end

    def test_merge_with_block_keeps_default
      h = Hash.new(5) #: Hash[Symbol, Integer]
      h[:a] = 1
      m = h.merge({ a: 10 }) { |_, x, y| x + y }
      assert_equal [11, 5], [m[:a], m[:zz]]
    end

    def test_merge_untyped_keeps_default
      h = Hash.new(5) #: untyped
      h[:a] = 1
      assert_equal 5, h.merge({ b: 2 })[:zz]
    end

    def test_merge_result_is_unfrozen_and_distinct
      h = Hash.new(5) #: Hash[Symbol, Integer]
      h[:a] = 1
      h.freeze
      m = h.merge({})
      assert_equal [false, false, 5], [m.frozen?, m.equal?(h), m[:q]]
    end

    def test_update_keeps_default
      h = Hash.new(5) #: Hash[Symbol, Integer]
      h.update({ a: 1 })
      h.merge!({ b: 2 })
      assert_equal [5, 2], [h[:zz], h.size]
    end

    def test_to_h_is_self
      h = Hash.new(5) #: Hash[Symbol, Integer]
      h[:a] = 1
      assert_same h, h.to_h
      h.freeze
      assert_same h, h.to_h
      u = h #: untyped
      assert_same h, u.to_h
    end

    def test_subclass_to_h_is_a_plain_hash_copy
      s = HashToHSub.new
      s.default = 7
      s[:a] = 1
      t = s.to_h
      assert_equal [Hash, false, { a: 1 }], [t.class, t.equal?(s), t]
    end
  end

  # #102: the bang filters answer nil when nothing changed
  class HashBangFilterTest < Minitest::Test
    def test_reject_select_bang_nil
      h = { a: 1, b: 2 }
      assert_nil h.reject! { |_, v| v > 5 }
      assert_nil h.select! { true }
      assert_nil h.filter! { true }
      assert_equal({ b: 2 }, h.reject! { |_, v| v == 1 })
      assert_equal({}, h.select! { false })
    end
  end

  # #102: frozen checks, replace/compact keep the default, Hash[] element errors
  class HashRubySpec102Test < Minitest::Test
    def test_frozen_mutators_raise
      h = { a: 1 }
      h.freeze
      got = [-> { h.shift }, -> { h.default = 1 }, -> { h.merge!({}) }, -> { h.transform_values! { |v| v } }].map do |l|
        l.call
        :none
      rescue FrozenError => e
        e.class
      end
      assert_equal [FrozenError, FrozenError, FrozenError, FrozenError], got
    end

    def test_replace_and_compact_keep_default
      g = { b: 2 }
      g.default = 9
      r = { c: 1 }
      r.replace(g)
      c = { a: nil, b: 1 } #: Hash[Symbol, Integer?]
      c.default = 3
      assert_equal [9, 3], [r[:zz], c.compact[:q]]
    end

    def test_hash_brackets_element_errors
      m1 = begin; Hash[[:a]]; rescue ArgumentError => e; e.message; end
      m2 = begin; Hash[[[1, 2, 3]]]; rescue ArgumentError => e; e.message; end
      assert_equal ["wrong element type Symbol at 0 (expected array)", "invalid number of elements (3 for 1..2)"], [m1, m2]
      assert_equal({ a: nil, b: 2 }, Hash[[[:a], [:b, 2]]])
    end
  end

  # #102: a subclass's default(key) answers a missing key
  class HashDefaultOverrideTest < Minitest::Test
    def test_subclass_default_method
      h = HashDefaultSub.new
      h[:b] = 1
      assert_equal [100, 1, true, false], [h[:a], h[:b], h.key?(:b), h.key?(:a)]
    end
  end

  class HashDefaultSub < Hash
    def default(key)
      100
    end
  end

  class HashToHSub < Hash
  end
end

module HashTests
  class HashDefaultProcSetTest < Minitest::Test
    def test_default_proc_assign
      h = {} #: Hash[String, Integer]
      h.default_proc = proc { |hh, k| hh[k] = k.size }
      assert_equal 3, h["abc"]
      assert_equal({ "abc" => 3 }, h)
      h.default_proc = nil
      assert_nil h["zz"]
      assert_nil h.default_proc
    end
  end
end

module HashTests
  class HashEvalOrderTest < Minitest::Test
    def test_hash_literal_hoisting_key_sees_earlier_pair
      log = [] #: Array[Integer]
      h = { log.push(1).size => 1, (log.last || 0) + 1 => 5 }
      assert_equal({ 1 => 1, 2 => 5 }, h)
    end

    def test_hash_literal_splat_then_hoisting_value
      log = [] #: Array[Integer]
      h = { log.push(1).size => 1, **{ log.push(2).size => 2 }, 3 => (log.last || 0) + 1 }
      assert_equal({ 1 => 1, 2 => 2, 3 => 3 }, h)
      log.clear
      h = { log.push(1).size => 1, **{ (log.last || 0) + 1 => 2 } }
      assert_equal({ 1 => 1, 2 => 2 }, h)
    end

    def test_hash_literal_hoisting_value_sees_earlier_pair
      log = [] #: Array[Integer]
      h = { log.push(1).size => 1, log.push(2).size => log.last&.succ }
      assert_equal({ 1 => 1, 2 => 3 }, h)
      assert_equal [1, 2], log
    end

    def test_proc_compose_receiver_first
      log = [] #: Array[Symbol]
      f = proc { |x| x + 1 } #: ^(Integer) -> Integer
      g = proc { |x| x * 2 } #: ^(Integer) -> Integer
      left = hash_proc_note(log, :f, f) << hash_proc_note(log, :g, g)
      right = hash_proc_note(log, :f2, f) >> hash_proc_note(log, :g2, g)
      assert_equal [:f, :g, :f2, :g2], log
      assert_equal [7, 8], [left.call(3), right.call(3)]
    end

    def test_env_fetch_default_is_eager
      log = [] #: Array[Symbol]
      v = ENV.fetch("PATH", hash_snote(log, :d, "dflt"))
      assert_equal [:d], log
      assert_equal false, v == "dflt" # a boolean, so the trace oracle does not log PATH (go test puts GOROOT/bin on it)
      ENV.fetch(hash_snote(log, :k, "PATH"), hash_snote(log, :d2, "dflt"))
      assert_equal [:d, :k, :d2], log
    end
  end
end

module HashTests
  # Hash#inspect prints `k: v` for an identifier-shaped symbol and `"k": v` for the rest (Ruby 3.4).
  class HashLabelScanTest < Minitest::Test
    def test_symbol_keys_label_or_quoted
      syms = ["foo bar", "a?", "a!", "a=", "a?=", "a=?", "a!?", "[]=", "[]", "[]==", "9a", "a9", "+", "-", "a-b", "é", "éa?", "日本", "日本?", "$", "$a", "$-w", "$-", "$-é", "$-éé", "$1", "$10", "$01", "$0", "$~", "$~~", "@", "@@", "@@@a", "@a", "@@a", "@9", "@a?", "**", "<=>", "===", "!=", "+@", "-@", "*@", "`", "$`", "A", "_", "_9", "$_", "", "a b", "a\nb", "a\"b", "a#b", "#", "?", "=", "==", "=~", "!", "!~", "<<", ">=", "a.b", "nil", "self"] #: Array[String]
      assert_equal ["{\"foo bar\": 1}", "{a?: 1}", "{a!: 1}", "{\"a=\": 1}", "{\"a?=\": 1}", "{\"a=?\": 1}", "{\"a!?\": 1}", "{\"[]=\": 1}", "{\"[]\": 1}", "{\"[]==\": 1}", "{\"9a\": 1}", "{a9: 1}", "{\"+\": 1}", "{\"-\": 1}", "{\"a-b\": 1}", "{é: 1}", "{éa?: 1}", "{日本: 1}", "{日本?: 1}", "{\"$\": 1}", "{\"$a\": 1}", "{\"$-w\": 1}", "{\"$-\": 1}", "{\"$-é\": 1}", "{\"$-éé\": 1}", "{\"$1\": 1}", "{\"$10\": 1}", "{\"$01\": 1}", "{\"$0\": 1}", "{\"$~\": 1}", "{\"$~~\": 1}", "{\"@\": 1}", "{\"@@\": 1}", "{\"@@@a\": 1}", "{\"@a\": 1}", "{\"@@a\": 1}", "{\"@9\": 1}", "{\"@a?\": 1}", "{\"**\": 1}", "{\"<=>\": 1}", "{\"===\": 1}", "{\"!=\": 1}", "{\"+@\": 1}", "{\"-@\": 1}", "{\"*@\": 1}", "{\"`\": 1}", "{\"$`\": 1}", "{A: 1}", "{_: 1}", "{_9: 1}", "{\"$_\": 1}", "{\"\": 1}", "{\"a b\": 1}", "{\"a\\nb\": 1}", "{\"a\\\"b\": 1}", "{\"a#b\": 1}", "{\"#\": 1}", "{\"?\": 1}", "{\"=\": 1}", "{\"==\": 1}", "{\"=~\": 1}", "{\"!\": 1}", "{\"!~\": 1}", "{\"<<\": 1}", "{\">=\": 1}", "{\"a.b\": 1}", "{nil: 1}", "{self: 1}"], syms.map { |s| { s.to_sym => 1 }.inspect }
      assert_equal "{a: 1, \"b c\": 2, d?: 3, e!: 4, \"f=\": 5, \"g\" => 6}", { a: 1, "b c": 2, d?: 3, e!: 4, "f=": 5, "g" => 6 }.inspect
    end
  end
end

module HashTests
  # The entry-array layout (decision 350): tombstones, compaction, shift, and order across them.
  class HashLayoutTest < Minitest::Test
    def test_reinsert_after_delete_goes_last
      h = { a: 1, b: 2, c: 3 }
      h.delete(:a)
      h[:a] = 4
      assert_equal [:b, :c, :a], h.keys
      h[:b] = 20
      assert_equal [[:b, 20], [:c, 3], [:a, 4]], h.to_a
      assert_equal 3, h.size
    end

    def test_many_deletes_keep_order
      h = {} #: Hash[Integer, Integer]
      100.times { |i| h[i] = i * i }
      (0...100).step(2) { |i| h.delete(i) }
      assert_equal 50, h.size
      assert_equal (1...100).step(2).to_a, h.keys
      (0...90).each { |i| h.delete(i) }
      assert_equal [91, 93, 95, 97, 99], h.keys
      h[0] = 0
      h[93] = -1
      assert_equal [[91, 8281], [93, -1], [95, 9025], [97, 9409], [99, 9801], [0, 0]], h.to_a
      assert_equal 9801, h[99]
      assert_nil h[2]
      assert_equal({ 91 => 8281, 93 => -1, 95 => 9025, 97 => 9409, 99 => 9801, 0 => 0 }, h)
      [91, 93, 95, 97, 99, 0].each { |k| h.delete(k) }
      assert_empty h
      h[5] = 5
      assert_equal({ 5 => 5 }, h)
    end

    def test_delete_during_each
      h = {} #: Hash[Integer, String]
      40.times { |i| h[i] = i.to_s }
      seen = [] #: Array[Integer]
      h.each do |k, _v|
        seen << k
        h.delete(k + 1) if k.even?
        h.delete(k) if k % 3 == 0
      end
      assert_equal (0...40).step(2).to_a, seen
      assert_equal (0...40).step(2).reject { |k| k % 3 == 0 }, h.keys
      h[100] = "x"
      assert_equal "x", h[100]
      assert_equal 100, h.keys.last
    end

    def test_new_key_during_each_raises_existing_updates
      h = { a: 1, b: 2, c: 3 }
      h.each { |k, v| h[k] = v * 10 }
      assert_equal({ a: 10, b: 20, c: 30 }, h)
      raised = false
      begin
        h.each { |_k, _v| h[:z] = 0 }
      rescue RuntimeError => e
        raised = e.message == "can't add a new key into hash during iteration"
      end
      assert raised
      assert_equal [:a, :b, :c], h.keys
      h.each { |k, _v| h.delete(k) }
      assert_empty h
      h[:z] = 1
      assert_equal({ z: 1 }, h)
    end

    def test_clear_and_shift_during_each
      h = { a: 1, b: 2, c: 3 }
      seen = [] #: Array[Symbol]
      h.each do |k, _v|
        seen << k
        h.clear if k == :a
      end
      assert_equal [:a], seen
      assert_empty h
      h = { a: 1, b: 2, c: 3 }
      seen = []
      h.each do |k, _v|
        seen << k
        h.shift if k == :a
      end
      assert_equal [:a, :b, :c], seen
      assert_equal({ b: 2, c: 3 }, h)
    end

    def test_shift_as_queue
      h = Hash.new(-1) #: Hash[Integer, Integer]
      out = [] #: Array[Integer]
      1000.times do |i|
        h[i] = i
        h[i + 5000] = i
        pair = h.shift
        out << pair[0] if pair && i % 100 == 0
      end
      assert_equal 1000, h.size
      assert_equal [0, 50, 100, 150, 200, 250, 300, 350, 400, 450], out
      assert_equal [500, 5500], h.keys.first(2)
      h.clear
      assert_nil h.shift
      assert_equal(-1, h[3])
    end

    def test_delete_if_and_select_bang
      h = {} #: Hash[Integer, Integer]
      50.times { |i| h[i] = i }
      h.delete_if { |k, _v| k % 5 != 0 }
      assert_equal [0, 5, 10, 15, 20, 25, 30, 35, 40, 45], h.keys
      assert_nil h.select! { |_k, _v| true }
      h.keep_if { |k, _v| k > 20 }
      assert_equal [25, 30, 35, 40, 45], h.keys
      h.reject! { |k, _v| k == 30 }
      h[1] = 1
      assert_equal [25, 35, 40, 45, 1], h.keys
      raised = false
      begin
        h.delete_if { |_k, _v| h[99] = 9; false }
      rescue RuntimeError
        raised = true
      end
      assert raised
      assert_equal [25, 35, 40, 45, 1], h.keys
    end

    def test_frozen_mutators
      h = { a: 1 }.freeze
      [-> { h.delete(:a) }, -> { h.shift }, -> { h.clear }, -> { h[:b] = 2 }, -> { h.delete_if { true } }].each do |f|
        ok = false
        begin
          f.call
        rescue FrozenError
          ok = true
        end
        assert ok
      end
      assert_equal({ a: 1 }, h)
    end

    def test_value_keys_after_delete
      h = {} #: Hash[untyped, Integer]
      h[[1, 2]] = 1
      h[1] = 2
      h[1.0] = 3
      h["s"] = 4
      assert_equal 4, h.size
      h.delete([1, 2])
      assert_nil h[[1, 2]]
      h[[1, 2]] = 5
      assert_equal [1, 1.0, "s", [1, 2]], h.keys
      assert_equal 5, h[[1, 2]]
      h.delete(1)
      assert_equal 3, h[1.0]
      assert_nil h[1]
    end

    def test_dup_merge_equality_after_deletes
      h = {} #: Hash[String, Integer]
      30.times { |i| h["k#{i}"] = i }
      20.times { |i| h.delete("k#{i}") }
      d = h.dup
      d["new"] = 1
      refute_equal h, d
      d.delete("new")
      assert_equal h, d
      assert h.hash == d.hash
      assert_equal "{\"k20\" => 20, \"k21\" => 21}", h.first(2).to_h.inspect
      m = h.merge({ "k25" => 0, "z" => 9 }) { |_k, a, b| a + b }
      assert_equal 25, m["k25"]
      assert_equal "z", m.keys.last
      assert_equal 11, m.size
      assert_equal({ "k21" => 21 }, h.select { |k, _v| k == "k21" })
    end

    def test_update_in_place_and_defaults
      counts = Hash.new(0) #: Hash[String, Integer]
      %w[b a b c a b].each { |w| counts[w] += 1 }
      assert_equal [["b", 3], ["a", 2], ["c", 1]], counts.to_a
      memo = {} #: Hash[Symbol, Integer]
      memo[:x] ||= 1
      memo[:x] ||= 2
      memo[:y] = (memo[:y] || 10) + 5
      memo[:y] = (memo[:y] || 10) + 5
      assert_equal({ x: 1, y: 20 }, memo)
      big = Hash.new(0) #: Hash[Integer, Integer]
      200.times { |i| big[i % 7] += i }
      big.delete(3)
      big[3] += 1
      assert_equal [0, 1, 2, 4, 5, 6, 3], big.keys
      assert_equal 1, big[3]
    end
  end

  # The open-addressed index past 8 entries (decision 388): probes past deleted slots, slot reuse, rebuilds.
  class HashIndexTest < Minitest::Test
    def test_lookups_past_deleted_slots
      h = {} #: Hash[Integer, Integer]
      64.times { |i| h[i * 64] = i }
      (0...64).step(2) { |i| h.delete(i * 64) }
      assert_equal 32, h.size
      assert_equal (1...64).step(2).sum, (0...64).sum { |i| h[i * 64] || 0 }
      assert_nil h[0]
      h[0] = -1
      h[128] = -2
      assert_equal [-1, -2], h.values.last(2)
      assert_equal [64, 192], h.keys.first(2)
      h.delete(64)
      assert_equal 33, h.size
      assert_equal(-2, h[128])
      assert_equal 3, h[192]
    end

    def test_churn_at_constant_size
      h = {} #: Hash[Integer, Integer]
      20.times { |i| h[i] = i }
      5000.times do |i|
        h.delete(i)
        h[i + 20] = i
        h[i + 7] = -i if i.even?
      end
      assert_equal 20, h.size
      assert_equal (5000...5020).to_a, h.keys
      assert_equal 4993, h[5013]
      assert_equal(-4994, h[5001])
    end

    def test_clear_during_each_then_reuse
      h = {} #: Hash[String, Integer]
      12.times { |i| h["k#{i}"] = i }
      h.each { |k, _v| h.clear if k == "k3" }
      assert_empty h
      12.times { |i| h["k#{i}"] = i * 2 }
      h.delete("k5")
      h["k5"] = 0
      assert_equal 22, h["k11"]
      assert_equal "k5", h.keys.last
      assert_equal 12, h.size
    end

    def test_float_keys
      h = {} #: Hash[Float, Integer]
      10.times { |i| h[i.to_f + 0.5] = i }
      h[0.0] = 1
      h[-0.0] = 2
      assert_equal 11, h.size
      assert_equal 2, h[0.0]
      assert_equal 9, h[9.5]
      assert_nil h[10.5]
    end

    def test_value_keys_past_eight
      h = {} #: Hash[Array[Integer], Integer]
      20.times { |i| h[[i, i + 1]] = i }
      (0...20).step(3) { |i| h.delete([i, i + 1]) }
      assert_equal 13, h.size
      assert_equal 19, h[[19, 20]]
      assert_nil h[[3, 4]]
      h[[3, 4]] = 99
      assert_equal [3, 4], h.keys.last
      assert_equal 99, h[[3, 4]]
    end

    def test_symbol_and_string_keys_past_eight
      s = {} #: Hash[Symbol, Integer]
      words = %w[alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu]
      words.each_with_index { |w, i| s[w.to_sym] = i }
      assert_equal 11, s[:mu]
      t = {} #: Hash[String, Integer]
      words.each_with_index { |w, i| t[w] = i }
      t.delete("beta")
      assert_equal 7, t["theta"]
      assert_nil t["beta"]
      assert_equal words - ["beta"], t.keys
    end
  end

  # Fused `h[k] op= v` and `h[k] = (h[k] || d) op v` (decision 350): a hit updates in place, a miss is the unfused path.
  class HashFusedTest < Minitest::Test
    class Tally
      def initialize
        @counts = {} #: Hash[String, Integer]
      end

      #: (String) -> Integer
      def add(w) = @counts[w] = (@counts[w] || 0) + 1

      #: () -> Hash[String, Integer]
      def counts = @counts
    end

    def test_values_and_results
      h = { a: 1, b: 2 } #: Hash[Symbol, Integer]
      x = (h[:a] += 5)
      assert_equal 6, x
      y = h[:b] = (h[:b] || 0) * 10
      assert_equal 20, y
      h[:c] = (h[:c] || 7) - 1
      assert_equal({ a: 6, b: 20, c: 6 }, h)
      f = { a: 1.5 } #: Hash[Symbol, Float]
      f[:a] *= 2
      f[:a] /= 0
      assert_equal Float::INFINITY, f[:a]
      s = { a: "x" } #: Hash[Symbol, String]
      s[:a] += "y"
      s[:b] = (s[:b] || "<") + "z"
      assert_equal({ a: "xy", b: "<z" }, s)
      m = { a: 17 } #: Hash[Symbol, Integer]
      m[:a] %= 5
      m[:a] -= 10
      assert_equal(-8, m[:a])
      t = Tally.new
      %w[x y x z x].each { |w| t.add(w) }
      assert_equal({ "x" => 3, "y" => 1, "z" => 1 }, t.counts)
    end

    def test_defaults_and_order
      d = Hash.new(5) #: Hash[Symbol, Integer]
      d[:x] -= 1
      d[:y] += 1
      d[:x] -= 1
      assert_equal [[:x, 3], [:y, 6]], d.to_a
      calls = [] #: Array[Symbol]
      pr = Hash.new { |hash, k| calls << k; hash[k] = 10 } #: Hash[Symbol, Integer]
      pr[:a] += 1
      pr[:a] += 1
      pr[:b] += 2
      assert_equal({ a: 12, b: 12 }, pr)
      assert_equal [:a, :b], calls
    end

    def test_errors_leave_the_entry
      h = { a: 1 } #: Hash[Symbol, Integer]
      zero = 0
      raised = false
      begin
        h[:a] /= zero
      rescue ZeroDivisionError
        raised = true
      end
      assert raised
      assert_equal 1, h[:a]
      h.freeze
      frozen = false
      begin
        h[:a] += 1
      rescue FrozenError
        frozen = true
      end
      assert frozen
      frozen = false
      begin
        h[:a] = (h[:a] || 0) + 1
      rescue FrozenError
        frozen = true
      end
      assert frozen
      assert_equal({ a: 1 }, h)
    end

    def test_during_each
      h = Hash.new(0) #: Hash[Symbol, Integer]
      h[:a] = 1
      h[:b] = 2
      h.each { |k, _v| h[k] += 10 }
      assert_equal({ a: 11, b: 12 }, h)
      raised = false
      begin
        h.each { |_k, _v| h[:c] += 1 }
      rescue RuntimeError
        raised = true
      end
      assert raised
      assert_equal [:a, :b], h.keys
    end
  end
end

module HashTests
  # decision 364: a default block that takes nothing types V from its value, K untyped
  class HashParamlessDefaultTest < Minitest::Test
    def test_paramless_default_block
      h = Hash.new { 5 }
      assert_equal 0, h.size
      h[:a] = 1
      assert_equal [5, 1], [h[:b], h[:a]]
    end
  end
end

module HashTests
  # decision 366: a Hash literal local widens by later writes in user code too
  class HashMixedLiteralTest < Minitest::Test
    def test_hash_literal_widens
      g = { 1 => :a }
      g["s"] = 2
      assert_equal({ 1 => :a, "s" => 2 }, g)
    end
  end
end

# A converted copy keeps Hash.new's default (decision 416).
module HashTests
  class DefaultCopyTest < Minitest::Test
    #: (Hash[untyped, untyped]) -> untyped
    def hash_default_read(h) = h["zz"]

    def test_default_survives_conversion
      h = Hash.new(5) #: Hash[String, Integer]
      assert_equal(5, hash_default_read(h))
    end
  end

  # String? keys hash and compare by their bytes (decision 448): nil is one key, equal Strings from different reads are one
  class HashStringOptKeyTest < Minitest::Test
    def test_string_opt_keys
      words = "a b a c b a".split.map { |w| w + "" } #: Array[String?]
      words << nil
      words << nil
      freq = {} #: Hash[String?, Integer]
      words.each { |w| freq[w] = (freq.key?(w) ? (freq[w] || 0) + 1 : 1) }
      assert_equal({ "a" => 3, "b" => 2, "c" => 1, nil => 2 }, freq)
      k = "a".dup #: String?
      assert_equal 3, freq[k]
      assert_equal 2, freq[nil]
      assert_equal 3, freq.delete("a".dup)
      assert_nil freq["a"]
      (1..20).each { |i| freq["k#{i}"] = i }
      assert_equal 7, freq["k" + "7"]
      assert_equal 2, freq[nil]
      freq.delete(nil)
      assert_nil freq[nil]
      assert_equal 22, freq.size
    end
  end

  # A blockless map shows its receiver, as MRI's Enumerator (decision 450)
  class HashBlocklessMapTest < Minitest::Test
    def test_blockless_map
      h = { "a" => 1 } #: Hash[String, Integer]
      assert_equal '#<Enumerator: {"a" => 1}:map>', h.map.inspect
      assert_equal [["a", 1, 0]], h.map.with_index { |(k, v), i| [k, v, i] }
    end
  end
end
