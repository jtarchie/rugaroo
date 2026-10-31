# rbs_inline: enabled

require "minitest/autorun"
require "set"

# Ractor (decision 103): isolation, copying, ports and errors, against MRI.
module RactorTests
  class Point
    attr_accessor :tags #: Array[String]

    #: (Array[String]) -> void
    def initialize(tags)
      @tags = tags
    end
  end

  Pair = Struct.new(:a, :b) #: [Array[Integer], String]
  Coord = Data.define(:xs) #: [Array[Integer]]

  class RactorTest < Minitest::Test
    def test_value_and_join
      r = Ractor.new { 6 * 7 }
      assert_equal 42, r.value
      assert_equal 42, r.value
      assert_same r, r.join
      assert_nil Ractor.new { ractor_noop }.value
      # the block's own locals are fine, from nested blocks too
      summed = Ractor.new do
        sum = 0
        [1, 2].each { |v| sum += v }
        sum
      end
      assert_equal 3, summed.value
    end

    def test_args_are_copied
      list = [1, [2]] #: Array[untyped]
      r = Ractor.new(list) do |xs|
        xs << 3
        xs.size
      end
      assert_equal 3, r.value
      assert_equal [1, [2]], list
      pt = Point.new(["a"])
      got = Ractor.new(pt) do |o|
        o.tags << "b"
        o.tags
      end.value
      assert_equal %w[a b], got
      assert_equal ["a"], pt.tags
      s = Pair.new([1], "x")
      assert_equal [[1, 9], "x"], Ractor.new(s) { |v|
        v.a << 9
        [v.a, v.b]
      }.value
      assert_equal [1], s.a
      c = Coord.new(xs: [1])
      assert_equal [1, 2], Ractor.new(c) { |v|
        v.xs << 2
        v.xs
      }.value
      assert_equal [1], c.xs
      h = { "k" => [1] }
      assert_equal({ "k" => [1, 2] }, Ractor.new(h) do |x|
        x["k"] << 2
        x
      end.value)
      assert_equal({ "k" => [1] }, h)
    end

    def test_copy_keeps_identity_and_shares_frozen
      shared = [1]
      pair = [shared, shared]
      assert_equal true, Ractor.new(pair) { |x| x[0].equal?(x[1]) }.value
      cyc = [1] #: Array[untyped]
      cyc << cyc
      assert_equal true, Ractor.new(cyc) { |x| x[1].equal?(x) }.value
      frozen = [1, 2].freeze
      assert_equal true, Ractor.new(frozen) { |x| x }.value.equal?(frozen)
      assert_equal true, Ractor.new(Ractor.main) { |m| m }.value.equal?(Ractor.main)
    end

    def test_three_args_and_name
      assert_equal "a1true", Ractor.new("a", 1, true) { |s, n, b| "#{s}#{n}#{b}" }.value
      assert_equal "w", Ractor.new(name: "w") { 1 }.name
      assert_nil Ractor.new { 1 }.name
    end

    def test_send_and_receive
      echo = Ractor.new { Ractor.receive }
      echo.send("hi")
      assert_equal "hi", echo.value
      summer = Ractor.new do
        a = Ractor.receive #: Integer
        b = Ractor.receive #: Integer
        a + b
      end
      summer << 1 << 2
      assert_equal 3, summer.value
      str = "abc"
      mover = Ractor.new { Ractor.receive }
      mover.send(str, move: true)
      assert_equal "abc", mover.value
    end

    def test_message_copied
      list = [1]
      r = Ractor.new do
        xs = Ractor.receive #: Array[Integer]
        xs << 2
        xs
      end
      r << list
      assert_equal [1, 2], r.value
      assert_equal [1], list
    end

    def test_ports
      port = Ractor::Port.new
      3.times { |i| Ractor.new(port, i) { |pt, n| pt << n * n } }
      got = [] #: Array[Integer]
      3.times do
        v = port.receive #: Integer
        got << v
      end
      assert_equal [0, 1, 4], got.sort
      assert_equal false, port.closed?
      port.close
      assert_equal true, port.closed?
      e = assert_raises(Ractor::ClosedError) { port.receive }
      assert_equal "The port was already closed", e.message
      e = assert_raises(Ractor::ClosedError) { port << 1 }
      assert_equal "The port was already closed", e.message
      r = Ractor.new { Ractor.receive }
      r << 1
      r.join
      e = assert_raises(Ractor::ClosedError) { r.send(2) }
      assert_equal "The port was already closed", e.message
      assert_equal Ractor::Port, r.default_port.class
    end

    def test_select
      a = Ractor::Port.new
      b = Ractor::Port.new
      Ractor.new(b) { |pt| pt << :b }
      port, msg = Ractor.select(a, b)
      assert_equal :b, msg
      assert_same b, port
      a.close
      b.close
      e = assert_raises(Ractor::ClosedError) { Ractor.select(a, b) }
      assert_equal "The port was already closed", e.message
    end

    def test_main
      assert_equal true, Ractor.main?
      assert_equal false, Ractor.new { Ractor.main? }.value
      assert_same Ractor.main, Ractor.current
      me = Ractor.new { Ractor.current }
      assert_same me, me.value
      Ractor.new(Ractor.main) { |m| m << :hello }
      assert_equal :hello, Ractor.receive
    end

    # Ractor.receive is resolved at run time (decision 104): from a helper method, and from a thread the ractor started.
    def test_receive_in_method_and_thread
      doubler = Ractor.new { ractor_helper_receive * 2 }
      doubler << 21
      assert_equal 42, doubler.value
      via_thread = Ractor.new { Thread.new { Ractor.receive }.value }
      via_thread << :via_thread
      assert_equal :via_thread, via_thread.value
      assert_equal true, Ractor.new { Thread.current.equal?(Thread.main) }.value
    end

    def test_port_owner
      port = Ractor::Port.new
      e = assert_raises(Ractor::RemoteError) { Ractor.new(port) { |pt| pt.receive }.value }
      cause = e.cause
      refute_nil cause
      assert_equal Ractor::Error, cause.class
      assert_equal "only allowed from the creator Ractor of this port", cause.message if cause
      e = assert_raises(Ractor::RemoteError) { Ractor.new(port) { |pt| Ractor.select(pt) }.value }
      cause = e.cause
      refute_nil cause
      assert_equal Ractor::ClosedError, cause.class
      assert_equal "The port was already closed", cause.message if cause
    end

    def test_remote_error
      failing = Ractor.new { raise ArgumentError, "bad" }
      e = assert_raises(Ractor::RemoteError) { failing.value }
      assert_equal "thrown by remote Ractor.", e.message
      assert_same failing, e.ractor
      cause = e.cause
      refute_nil cause
      assert_equal ArgumentError, cause.class
      assert_equal "bad", cause.message if cause
      e2 = assert_raises(Ractor::RemoteError) { failing.join }
      assert_equal "thrown by remote Ractor.", e2.message
    end

    def test_uncopyable
      e = assert_raises(Ractor::Error) { Ractor.new(Set[[1]]) { |s| s }.value }
      assert_equal "can not copy Set object.", e.message
      assert_equal Set[1, 2], Ractor.new(Set[1, 2]) { |s| s }.value
      e2 = assert_raises(NoMethodError) { Ractor.new(Queue.new([1])) { |q| q }.value }
      assert_equal "undefined method 'initialize_copy' for an instance of Thread::Queue", e2.message
      e3 = assert_raises(TypeError) { Ractor.new(Thread.new { 1 }) { |t| t }.value }
      assert_equal "allocator undefined for Thread", e3.message
    end

    def test_error_classes
      e = assert_raises(RuntimeError) { raise Ractor::Error, "base" }
      assert_equal "base", e.message
      e = assert_raises(Ractor::Error) { raise Ractor::IsolationError, "iso" }
      assert_equal "iso", e.message
      e = assert_raises(StopIteration) { raise Ractor::ClosedError, "closed" }
      assert_equal "closed", e.message
    end
  end
end

#: () -> void
def ractor_noop; end

#: () -> Integer
def ractor_helper_receive
  v = Ractor.receive #: Integer
  v
end

module RactorTests
  class RactorShareableTest < Minitest::Test
    def test_make_shareable_deep_freezes_and_answers_the_object
      a = [1, { k: "v" }] #: untyped
      b = Ractor.make_shareable(a)
      assert_equal a, b
      assert a.frozen?
      assert a[1].frozen?
      assert a[1][:k].frozen?
    end
  end
end
