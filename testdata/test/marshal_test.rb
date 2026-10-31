# rbs_inline: enabled

require "minitest/autorun"
require "set"
require "date"
require "bigdecimal"
require "stringio"
require "tmpdir"
require "ostruct"
require "json"
require "socket"

# Marshal (decision 137): the bytes are rugaroo's own, so no assertion ever sees them; only the object graphs are MRI's.
module MarshalTests
  Point = Struct.new(:x, :y) #: [Integer, Integer]
  Pair = Data.define(:left, :right) #: [Integer, String]

  class Link
    attr_reader :nx #: untyped

    #: (untyped) -> void
    def initialize(nx)
      @nx = nx
    end
  end

  class Node
    attr_reader :name #: String
    attr_reader :children #: Array[Node]
    attr_accessor :parent #: Node?

    #: (String) -> void
    def initialize(name)
      @name = name
      @children = [] #: Array[Node]
      @parent = nil
    end

    #: (Node) -> Node
    def add(child)
      child.parent = self
      @children << child
      child
    end
  end

  class Account
    attr_reader :owner #: String
    attr_reader :balance #: Float
    attr_reader :tags #: Set[Symbol]
    attr_reader :note #: String?

    #: (String, Float, Set[Symbol], String?) -> void
    def initialize(owner, balance, tags, note)
      @owner = owner
      @balance = balance
      @tags = tags
      @note = note
    end
  end

  # marshal_dump/marshal_load replace the ivars (private, as MRI calls them regardless): the area is recomputed
  class Circle
    attr_reader :radius #: Integer
    attr_reader :area #: Integer

    #: (Integer) -> void
    def initialize(radius)
      @radius = radius
      @area = radius * radius * 3
    end

    private

    #: () -> Array[Integer]
    def marshal_dump = [@radius]

    #: (Array[Integer]) -> void
    def marshal_load(data)
      @radius = data.sum
      @area = @radius * @radius * 3
    end
  end

  class DumpOnly
    #: () -> Integer
    def marshal_dump = 1
  end

  class MarshalTest < Minitest::Test
    #: (untyped) -> untyped
    def round(x) = Marshal.load(Marshal.dump(x))

    def test_issue_example
      data = { name: "a", pts: [Point.new(1, 2), Point.new(3, 4)], when: Time.at(0).utc }
      copy = round(data)
      assert_equal true, copy == data
      copy[:pts][0].x = 99
      assert_equal 1, data[:pts][0].x
      assert_equal 99, copy[:pts][0].x
      assert_equal false, copy == data
      assert_equal "1970-01-01 00:00:00 UTC", copy[:when].to_s
    end

    def test_core_values
      assert_nil round(nil)
      assert_equal [true, false], round([true, false])
      assert_equal [0, -1, 4611686018427387904, -9223372036854775808], round([0, -1, 4611686018427387904, -9223372036854775808])
      assert_equal [1.5, -0.25, Float::INFINITY], round([1.5, -0.25, Float::INFINITY])
      assert_equal true, round(Float::NAN).nan?
      assert_equal "-0.0", round(-0.0).to_s
      assert_equal ["", "héllo", "a\nb"], round(["", "héllo", "a\nb"])
      assert_equal :sym, round(:sym)
      assert_equal [1, [2, ["three", nil]], :four], round([1, [2, ["three", nil]], :four])
      assert_equal({ "a" => 1, b: [2], 3 => nil }, round({ "a" => 1, b: [2], 3 => nil }))
      assert_equal [1, nil, 3], round([1, nil, 3])
      assert_equal Marshal::MAJOR_VERSION, 4
      assert_equal Marshal::MINOR_VERSION, 8
      assert_equal true, Marshal.dump(1).is_a?(String)
    end

    def test_ranges_sets_and_classes
      assert_equal 1..3, round(1..3)
      assert_equal "a"..."c", round("a"..."c")
      assert_equal Set[1, 2, 3], round(Set[1, 2, 3])
      assert_equal String, round(String)
      assert_equal Node, round(Node)
      assert_equal [Integer, Point], round([Integer, Point])
      assert_equal(/a+b/i, round(/a+b/i))
    end

    def test_numbers
      assert_equal Rational(1, 3), round(Rational(1, 3))
      assert_equal Complex(1, 2), round(Complex(1, 2))
      assert_equal Complex(Rational(1, 2), 1.5), round(Complex(Rational(1, 2), 1.5))
      assert_equal BigDecimal("123.456"), round(BigDecimal("123.456"))
      assert_equal "-0.1e-9", round(BigDecimal("-1e-10")).to_s
      assert_equal true, round(BigDecimal("NaN")).nan?
    end

    def test_times_and_dates
      t = Time.at(1_700_000_000).utc
      assert_equal t, round(t)
      assert_equal true, round(t).utc?
      local = Time.at(1_700_000_000)
      assert_equal local.to_s, round(local).to_s
      assert_equal false, round(local).utc?
      assert_equal Date.new(2024, 2, 29), round(Date.new(2024, 2, 29))
      dt = DateTime.new(2024, 2, 29, 13, 45, 30, "+09:00")
      assert_equal dt.to_s, round(dt).to_s
      assert_equal true, round(dt).instance_of?(DateTime)
    end

    def test_structs_data_and_objects
      p2 = round(Point.new(5, 6))
      assert_equal Point.new(5, 6), p2
      assert_equal Pair.new(left: 1, right: "r"), round(Pair.new(left: 1, right: "r"))
      acct = Account.new("ann", 10.5, Set[:a, :b], nil)
      copy = round(acct)
      assert_equal "ann", copy.owner
      assert_equal 10.5, copy.balance
      assert_equal Set[:a, :b], copy.tags
      assert_nil copy.note
      assert_equal true, copy.is_a?(Account)
      assert_equal false, copy.equal?(acct)
      assert_equal %i[@owner @balance @tags @note], copy.instance_variables
    end

    def test_copy_is_independent
      src = { list: [1, 2], inner: { k: "v" } }
      copy = round(src)
      copy[:list] << 3
      copy[:inner][:k] = "w"
      assert_equal({ list: [1, 2], inner: { k: "v" } }, src)
      assert_equal({ list: [1, 2, 3], inner: { k: "w" } }, copy)
      acct = Account.new("bo", 1.0, Set[:x], "n")
      c2 = round(acct)
      c2.tags << :y
      assert_equal Set[:x], acct.tags
    end

    def test_shared_references_and_cycles
      a = [] #: Array[untyped]
      a << a
      b = round(a)
      assert_equal true, b[0].equal?(b)
      shared = [1, 2]
      pair = round([shared, shared])
      assert_equal true, pair[0].equal?(pair[1])
      pair[0] << 3
      assert_equal [1, 2, 3], pair[1]
      h = {} #: Hash[Symbol, untyped]
      h[:self] = h
      h2 = round(h)
      assert_equal true, h2[:self].equal?(h2)
      root = Node.new("root")
      kid = root.add(Node.new("kid"))
      kid.add(Node.new("leaf"))
      r2 = round(root)
      k2 = r2.children[0]
      assert_equal "kid", k2.name
      assert_equal true, k2.parent.equal?(r2)
      assert_equal "leaf", k2.children[0].name
      assert_equal true, k2.children[0].parent.equal?(k2)
      pt = Point.new(1, 1)
      both = round({ a: pt, b: pt })
      assert_equal true, both[:a].equal?(both[:b])
    end

    def test_hooks
      c = round(Circle.new(4))
      assert_equal 4, c.radius
      assert_equal 48, c.area
      many = round([Circle.new(1), Circle.new(2)])
      assert_equal [3, 12], [many[0].area, many[1].area]
      bytes = Marshal.dump(DumpOnly.new)
      e = assert_raises(TypeError) { Marshal.load(bytes) }
      assert_equal "instance of MarshalTests::DumpOnly needs to have method 'marshal_load'", e.message
    end

    def test_io_forms
      io = StringIO.new
      assert_equal true, Marshal.dump([1, 2], io).equal?(io)
      Marshal.dump({ a: 1 }, io)
      io.rewind
      assert_equal [1, 2], Marshal.load(io)
      assert_equal({ a: 1 }, Marshal.load(io))
      e = assert_raises(EOFError) { Marshal.load(io) }
      assert_equal "end of file reached", e.message
      Dir.mktmpdir do |dir|
        path = File.join(dir, "cache.bin")
        File.binwrite(path, Marshal.dump({ name: "a", pts: [Point.new(1, 2)] }))
        back = Marshal.load(File.binread(path))
        assert_equal "a", back[:name]
        assert_equal [Point.new(1, 2)], back[:pts]
        File.open(path, "wb") do |f|
          Marshal.dump("first", f)
          Marshal.dump([:second], f)
        end
        got = [] #: Array[untyped]
        File.open(path, "rb") do |f|
          begin
            loop { got << Marshal.load(f) }
          rescue EOFError
            got << :eof
          end
          f.close
        end
        assert_equal ["first", [:second], :eof], got
      end
      a, b = UNIXSocket.pair
      Marshal.dump({ x: [1, 2] }, a)
      Marshal.dump("second", a)
      assert_equal [{ x: [1, 2] }, "second"], [Marshal.load(b), Marshal.load(b)]
      a.close
      assert_raises(EOFError) { Marshal.load(b) }
      b.close
      assert_equal [1], Marshal.restore(Marshal.dump([1]))
      [5, nil].each do |bad|
        e = assert_raises(TypeError) { Marshal.load(bad) }
        assert_equal "instance of IO needed", e.message
      end
      e = assert_raises(TypeError) { Marshal.dump(1, "x") }
      assert_equal "instance of IO needed", e.message
    end

    def test_library_values
      e = round(ArgumentError.new("boom"))
      assert_equal "boom", e.message
      assert_equal true, e.instance_of?(ArgumentError)
      o = round(OpenStruct.new(a: 1, b: "x"))
      assert_equal [1, "x"], [o.a, o.b]
      r = Random.new(3)
      r.rand(10)
      r2 = round(r)
      assert_equal r.rand(1000), r2.rand(1000)
      assert_equal({ "a" => [1, { "b" => nil }] }, round(JSON.parse('{"a": [1, {"b": null}]}')))
      assert_equal Comparable, round(Comparable)
      assert_equal ["1..", 1], [round(1..).inspect, round(1..).begin]
      words = %w[a b a]
      assert_equal({ "a" => 2, "b" => 1 }, round(words.tally))
      assert_equal [%w[a b], ["a"]], round(words.each_slice(2).to_a)
    end

    def test_depth_limit
      e = assert_raises(ArgumentError) { Marshal.dump([[1]], 1) }
      assert_equal "exceed depth limit", e.message
      assert_equal [[1]], Marshal.load(Marshal.dump([[1]], 3))
    end

    # dump and load bound nesting alike: a chain dump accepts, load takes back (MRI manages ~20000 levels)
    def test_deep_chain_round_trips
      chain = nil #: untyped
      15_000.times { chain = [chain] }
      back = Marshal.load(Marshal.dump(chain))
      depth = 0
      while back
        back = back[0]
        depth += 1
      end
      assert_equal 15_000, depth
      list = nil #: untyped
      15_000.times { list = Link.new(list) }
      back = Marshal.load(Marshal.dump(list))
      length = 0
      while back
        back = back.nx
        length += 1
      end
      assert_equal 15_000, length
    end

    def test_undumpable
      e = assert_raises(TypeError) { Marshal.dump(proc { 1 }) }
      assert_equal "no _dump_data is defined for class Proc", e.message
      e = assert_raises(TypeError) { Marshal.dump([1, $stdout]) }
      assert_equal "can't dump IO", e.message
      rd, wr = IO.pipe
      e = assert_raises(TypeError) { Marshal.dump(rd) }
      assert_equal "can't dump IO", e.message
      Marshal.dump([1, 2], wr)
      wr.close
      assert_equal [1, 2], Marshal.load(rd)
      rd.close
      e = assert_raises(TypeError) { Marshal.dump(1.method(:+)) }
      assert_equal "no _dump_data is defined for class Method", e.message
      e = assert_raises(TypeError) { Marshal.dump(Mutex.new) }
      assert_equal "no _dump_data is defined for class Thread::Mutex", e.message
      e = assert_raises(TypeError) { Marshal.dump(Thread.current) }
      assert_equal "no _dump_data is defined for class Thread", e.message
      q = Queue.new #: Queue[Integer]
      e = assert_raises(TypeError) { Marshal.dump(q) }
      assert_equal "can't dump Thread::Queue", e.message
      e = assert_raises(TypeError) { Marshal.dump(StringIO.new) }
      assert_equal "no _dump_data is defined for class StringIO", e.message
      Dir.mktmpdir do |dir|
        File.open(File.join(dir, "f"), "w") do |f|
          e = assert_raises(TypeError) { Marshal.dump(f) }
          assert_equal "can't dump File", e.message
        end
      end
    end

    def test_bad_input
      e = assert_raises(ArgumentError) { Marshal.load("") }
      assert_equal "marshal data too short", e.message
      e = assert_raises(ArgumentError) { Marshal.load(Marshal.dump("abcdef")[0, 12]) }
      assert_equal "marshal data too short", e.message
      bad = begin
        Marshal.load("not marshal data")
        nil
      rescue TypeError => err
        err.class
      end
      assert_equal TypeError, bad
    end
  end

  # rugaroo's own stream (decision 137), built by hand: MRI rejects its header (TypeError), rugaroo must reject the body (ArgumentError or TypeError), never panic or overflow the Go stack, which no rescue catches.
  class HostileInputTest < Minitest::Test
    #: (String) -> String
    def self.stream(body)
      size = body.bytesize
      "RUGAROO\x04\x08" + (0...8).map { |i| (size >> (8 * i)) & 255 }.pack("C*") + body
    end

    #: (String) -> Class?
    def self.load_error(body)
      Marshal.load(stream(body))
      nil
    rescue Exception => e # rubocop:disable Lint/RescueException
      e.class
    end

    def test_deep_nesting_is_rejected_not_a_stack_overflow
      assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error(("o\x0aArray[any]\x01" * 1_000_000) + "0"))
    end

    def test_random_state_index_is_checked
      state = "\x00" * 624
      assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error("o\x06Random\x06\x01" + state))
      assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error("o\x06Random\x06\xe2\x0a" + state))
    end

    def test_bigdecimal_state_is_checked
      assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error("o\x0aBigDecimal\x02zz\x00F\x00"))
      assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error("o\x0aBigDecimal\x0215\x01F\x12"))
    end

    #: (Integer) -> String
    def self.zigzag(n)
      z = n < 0 ? ((-n) << 1) - 1 : n << 1
      bytes = [] #: Array[Integer]
      while z >= 0x80
        bytes << ((z & 0x7f) | 0x80)
        z >>= 7
      end
      bytes << z
      bytes.pack("C*")
    end

    # A loaded exponent past what BigDecimal arithmetic keeps would reach 10**n on the first comparison (#109).
    def test_bigdecimal_exponent_is_checked
      [2**61 + 1, -(2**61) - 1].each do |exp|
        assert_equal true, [ArgumentError, TypeError].include?(HostileInputTest.load_error("o\x0aBigDecimal\x011" + HostileInputTest.zigzag(exp) + "F\x00"))
      end
    end
  end
  # MRI's own 4.8 stream loads when it holds only core values (decision 137, amended): a gem's data file
  # (unicode-display_width's and unicode-emoji's `*.marshal.gz`). The bytes are `Marshal.dump` of the value under MRI,
  # pasted: longs of every width, an object link, symbol links, encoding-wrapped strings, a hash with a default.
  class MarshalMRIStreamTest < Minitest::Test
    MRI_BYTES = "\x04\b{\b:\bsym;\x00:\barr[\x1C[\ai\x06i\a@\aI\"\x06s\x06:\x06ET\"\x06sf\b1.5f\a-00TFl+\t\x00\x00\x00\x00\x00\x00\x00@l-\t\x00\x00\x00\x00\x00\x00\x00@i\x00i\x7Fi\x01{i\x01\xFFi\x02\x00\x01i\x03\x00\x00\x01l+\a\x00\x00\x00@i\xFAi\x80i\xFF\x84i\xFF\x00I\"\a\xC3\xA9\x06;\aT:\x06h}\x06:\x06k;\x00i\f".b

    def test_core_values_of_an_mri_stream
      v = Marshal.load(MRI_BYTES)
      assert_equal [:sym, :arr, :h], v.keys
      assert_equal :sym, v[:sym]
      arr = v[:arr]
      assert_equal [1, 2], arr[0]
      assert_same arr[0], arr[1]
      assert_equal ["s", "s", 1.5, -0.0, nil, true, false], arr[2..8]
      assert_equal [2**62, -(2**62), 0, 122, 123, 255, 256, 65536, 2**30, -1, -123, -124, -256], arr[9..21]
      assert_equal "é", arr[22]
      assert_equal 23, arr.size
      assert_equal({ k: :sym }, v[:h])
      assert_equal 7, v[:h][:missing]
    end

    def test_mri_stream_from_an_io
      v = Marshal.load(StringIO.new(MRI_BYTES))
      assert_equal 1.5, v[:arr][4]
    end
  end

end
