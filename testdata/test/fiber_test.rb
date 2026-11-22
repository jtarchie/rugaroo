# rbs_inline: enabled

require "minitest/autorun"

# Fiber (decisions 140, 403): a coroutine per fiber (iter.Pull), against MRI.
module FiberTests
  class FiberTest < Minitest::Test
    def test_resume_and_yield
      f = Fiber.new do |x|
        y = Fiber.yield x * 2
        y + 1
      end
      assert_equal 10, f.resume(5)
      assert f.alive?
      assert_equal 11, f.resume(10)
      refute f.alive?
    end

    def test_generator
      gen = Fiber.new do
        a, b = 0, 1
        loop do
          Fiber.yield a
          a, b = b, a + b
        end
      end
      assert_equal [0, 1, 1, 2, 3, 5], 6.times.map { gen.resume }
      counter = Fiber.new do
        3.times { |i| Fiber.yield i }
        :done
      end
      assert_equal [0, 1, 2, :done], 4.times.map { counter.resume }
    end

    def test_values
      assert_equal [1, 2], Fiber.new { Fiber.yield 1, 2 }.resume
      assert_nil Fiber.new { Fiber.yield }.resume
      assert_equal "nil", Fiber.new { |v| v.inspect }.resume
      two = Fiber.new do
        got = Fiber.yield
        got
      end
      two.resume
      assert_equal [3, 4], two.resume(3, 4)
    end

    def test_current
      assert Fiber.current.alive?
      assert Fiber.new { Fiber.current.alive? }.resume
      outer = Fiber.current
      refute Fiber.new { Fiber.current == outer }.resume
    end

    def test_errors
      done = Fiber.new { 1 }
      done.resume
      err = assert_raises(FiberError) { done.resume }
      assert_equal "attempt to resume a terminated fiber", err.message
      err = assert_raises(FiberError) { Fiber.yield 1 }
      assert_equal "attempt to yield on a not resumed fiber", err.message
      me = Fiber.new { Fiber.current.resume }
      err = assert_raises(FiberError) { me.resume }
      assert_equal "attempt to resume the current fiber", err.message
      boom = Fiber.new { raise ArgumentError, "boom" }
      err = assert_raises(ArgumentError) { boom.resume }
      assert_equal "boom", err.message
      refute boom.alive?
    end

    def fiber_step(x)
      Fiber.yield x
    end

    # Fiber.yield from a method the body calls, a raise after a yield, a fiber resumed from inside a thread.
    def test_yield_deep_and_late_raise
      deep = Fiber.new do
        a = fiber_step(1)
        fiber_step(a + 1)
        :end
      end
      assert_equal [1, 11, :end], [deep.resume, deep.resume(10), deep.resume]

      late = Fiber.new do
        Fiber.yield :ok
        raise ArgumentError, "late"
      end
      assert_equal :ok, late.resume
      err = assert_raises(ArgumentError) { late.resume }
      assert_equal "late", err.message
      refute late.alive?

      in_thread = Thread.new do
        f = Fiber.new { Fiber.yield Fiber.current.alive?; :done }
        [f.resume, f.resume, f.alive?]
      end
      assert_equal [true, :done, false], in_thread.value
      many = Fiber.new { loop { Fiber.yield 1 } }
      assert_equal 1000, 1000.times.sum { many.resume }
    end

    def test_nested
      inner = Fiber.new do
        Fiber.yield :inner_a
        :inner_b
      end
      outer = Fiber.new do
        a = inner.resume
        Fiber.yield a
        inner.resume
      end
      assert_equal :inner_a, outer.resume
      assert_equal :inner_b, outer.resume
    end
  end
end

module FiberTests
  # The root fiber is made once per thread, then looked up (decision 488).
  class FiberRootReuseTest < Minitest::Test
    #: () -> Integer
    def fiber_sum_many
      f = Fiber.new do |x|
        total = 0
        loop { x = Fiber.yield(total += x) }
      end
      last = 0
      1000.times { |i| last = f.resume(i) }
      last
    end

    def test_main_thread
      assert_equal 499_500, fiber_sum_many
      assert_equal Fiber.current, Fiber.current
    end

    def test_threads
      sums = Array.new(4) { Thread.new { [fiber_sum_many, Fiber.current == Fiber.current] } }.map(&:value)
      assert_equal [[499_500, true]] * 4, sums
    end
  end
end
