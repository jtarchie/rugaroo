# rbs_inline: enabled

require "minitest/autorun"
require "fiddle"
require "fiddle/import"

# Fiddle over purego (decision 326): C calls, Pointer and CStruct bytes, callbacks, against MRI's Fiddle.
module FiddleTests
  module Lib
    extend Fiddle::Importer
    dlload Fiddle.dlopen(nil)

    typealias "word", "unsigned int"
    extern "double ldexp(double, int)"
    extern "float fabsf(float)"
    extern "int atoi(const char *)"
    extern "long labs(long)"
    extern "char *strchr(const char *, int)"
    extern "int strcmp(const char *, const char *)"
    extern "word abs(int)"
    extern "void *memcpy(void *, const void *, size_t)"
    extern "size_t strlen(const char *)"
    extern "double fabs(double)"
    extern "void free(void *)"
    extern "bool isatty(int)"
    extern "long time(void *)"
    bind("int add3(int, int, int)") { |a, b, c| a + b + c }
    bind("long mix9(int, int, int, int, int, int, double, double, double)") { |a, b, c, d, e, f, x, y, z| (a + b + c + d + e + f + x * y * z).to_i }
    extern "int getpid(void)"
    extern "int open(const char *, int)"
    extern "long pread(int, void *, size_t, long)"
    extern "int close(int)"

    Pair = struct ["short a", "long b", "unsigned char c[3]", "float f"]
    Bits = union ["unsigned int u", "unsigned char b[4]"]
  end

  # A Closure subclass: C calls #call.
  class ByLast < Fiddle::Closure
    #: (Fiddle::Pointer, Fiddle::Pointer) -> Integer
    def call(a, b) = a[1] <=> b[1]
  end

  class FiddleTest < Minitest::Test
    #: () -> Fiddle::Handle
    def libc = Fiddle.dlopen(nil)

    def test_importer_calls
      assert_equal [1280.0, 2.5, 42, 7, 0, 3], [Lib.ldexp(5, 8), Lib.fabsf(-2.5), Lib.atoi("42abc"), Lib.labs(-7), Lib.strcmp("a", "a"), Lib.abs(-3)]
      assert_equal "lo", Lib.strchr("hello", "l".ord).to_s[1, 2]
      assert_equal [true, Fiddle::Function, "atoi"], [Lib.strchr("abc", "z".ord).null?, Lib["atoi"].class, Lib["atoi"]&.name]
      assert_equal [4, 8, 8], [Lib.sizeof("int"), Lib.sizeof("long"), Lib.sizeof("char*")]
    end

    # Integer/Float arguments take the extern's typed twin; any other argument, the extern's own path (decision 442).
    def test_typed_twins
      x = -1.5
      hello = Fiddle::Pointer["hello"]
      assert_equal [7, 7, 1.5, 2.0, 2.5, 6], [Lib.labs(-7), Lib.labs(-7.9), Lib.fabs(x), Lib.fabs(-2), Lib.fabsf(-2.5), Lib.add3(1, 2, 3)]
      assert_equal [5, 5, 3], [Lib.strlen(hello.to_i), Lib.strlen(hello), Lib.strlen("abc")]
      assert_equal ["llo", "llo"], [Lib.strchr(hello.to_i, "l".ord).to_s, Lib.strchr(hello, "l".ord).to_s]
      assert_equal [nil, nil], [Lib.free(0), Lib.free(nil)]
      assert_equal [false, Errno::EBADF::Errno], [Lib.isatty(99), Fiddle.last_error]
      t = Lib.time(0)
      buf = Fiddle::Pointer.malloc(8, Fiddle::RUBY_FREE)
      assert_equal [true, true], [Lib.time(nil) - t <= 1, Lib.time(buf) == buf[0, 8].unpack1("q")]
      bad = begin
        Lib.labs("7")
      rescue TypeError => e
        e.class
      end
      assert_equal TypeError, bad
      fd = Lib.open("/dev/zero", 0)
      zeros = Fiddle::Pointer.malloc(4, Fiddle::RUBY_FREE)
      zeros[0, 4] = "abcd"
      assert_equal [true, 4, [0, 0, 0, 0], 0], [Lib.getpid == Process.pid, Lib.pread(fd, zeros.to_i, 4, 0), zeros[0, 4].bytes, Lib.close(fd)]
      assert_equal 27, Lib.mix9(1, 2, 3, 4, 5, 6, 1.5, 2.0, 2.0)
    end

    def test_function_by_address
      sqrt = Fiddle::Function.new(libc["sqrt"], [Fiddle::TYPE_DOUBLE], Fiddle::TYPE_DOUBLE)
      toupper = Fiddle::Function.new(libc["toupper"], [Fiddle::TYPE_INT], Fiddle::TYPE_INT)
      isdigit = Fiddle::Function.new(libc["isdigit"], [Fiddle::TYPE_INT], Fiddle::TYPE_INT)
      assert_equal [3.0, 65, true], [sqrt.call(9), toupper.call(97), isdigit.call(55) != 0]
      assert_equal [true, 1], [libc["sqrt"] == sqrt.to_i, sqrt.abi]
    end

    def test_last_error_is_errno
      open = Fiddle::Function.new(libc["open"], [Fiddle::TYPE_CONST_STRING, Fiddle::TYPE_INT], Fiddle::TYPE_INT)
      assert_equal [-1, Errno::ENOENT::Errno], [open.call("/nonexistent/rugaroo-fiddle", 0), Fiddle.last_error]
      Fiddle.last_error = 7 # a call sets it again, even with the errno the call before it stored
      open.call("/nonexistent/rugaroo-fiddle", 0)
      assert_equal [Errno::ENOENT::Errno, Errno::ENOENT::Errno], [Fiddle.last_error, Thread.current[:__FIDDLE_LAST_ERROR__]]
      Thread.current[:__FIDDLE_LAST_ERROR__] = nil
      open.call("/nonexistent/rugaroo-fiddle", 0)
      assert_equal Errno::ENOENT::Errno, Fiddle.last_error
      assert_equal [Errno::ENOSYS.new.errno, Errno::EOPNOTSUPP.new.errno], [Errno::ENOSYS::Errno, Errno::EOPNOTSUPP::Errno]
    end

    def test_pointer_bytes
      ptr = Fiddle::Pointer.malloc(8, Fiddle::RUBY_FREE)
      ptr[0, 8] = "abcdefgh"
      ptr[1] = 90
      assert_equal ["aZcdefgh", 90, "cd", 8, false], [ptr[0, 8], ptr[1], (ptr + 2)[0, 2], ptr.size, ptr.null?]
      ptr[2] = 200
      assert_equal RUBY_PLATFORM.match?(/\A(?:aarch64|arm64)-linux/) ? 200 : -56, ptr[2] # C's char is unsigned only on Linux arm64
      assert_equal [3, 1], [(ptr + 3).to_i - ptr.to_i, ptr.to_i - (ptr - 1).to_i]
      assert_equal [true, false], [ptr == Fiddle::Pointer.new(ptr.to_i), ptr == Fiddle::Pointer.new(0)]
      assert ptr.to_i == ptr.ref.ptr.to_i
      ptr.call_free
      assert ptr.freed?
    end

    def test_pointer_strings
      str = Fiddle::Pointer["hi there"]
      assert_equal ["hi there", "hi", "there"], [str.to_s, str.to_s(2), (str + 3).to_s]
      assert Fiddle::NULL.null?
      err = begin
        Fiddle::NULL[0]
        nil
      rescue Fiddle::DLError => e
        e.message
      end
      assert_equal "NULL pointer dereference", err
    end

    def test_struct_layout_and_members
      assert_equal [24, 8, %w[a b c f], [3, 5, [-2, 3], 7]], [Lib::Pair.size, Lib::Pair.alignment, Lib::Pair.members, Lib::Pair.types]
      assert_equal([0, 8, 16, 20, nil], %w[a b c f nope].map { |m| Lib::Pair.offsetof(m) })
      pair = Lib::Pair.malloc(Fiddle::RUBY_FREE)
      pair.a = -2
      pair.b = 1 << 40
      pair.c = [1, 255, 3]
      pair.f = 1.5
      assert_equal [-2, 1 << 40, [1, 255, 3], 1.5], [pair.a, pair.b, pair.c, pair.f]
      pair["a"] = 70_000
      assert_equal [4464, pair.to_ptr[0, 2].unpack1("s")], [pair["a"], pair.a]
      assert_equal({ "a" => 4464, "b" => 1 << 40, "c" => [1, 255, 3], "f" => 1.5 }, pair.to_h)
    end

    def test_union_shares_bytes
      bits = Lib::Bits.malloc(Fiddle::RUBY_FREE)
      bits.u = 0x01020304
      assert_equal [4, 4, [4, 3, 2, 1]], [Lib::Bits.size, bits.to_ptr.size, bits.b]
    end

    def test_struct_through_c
      src = Lib::Pair.malloc(Fiddle::RUBY_FREE)
      dst = Lib::Pair.malloc(Fiddle::RUBY_FREE)
      src.b = 99
      src.f = -0.25
      Lib.memcpy(dst, src, Lib::Pair.size)
      assert_equal [99, -0.25], [dst.b, dst.f]
    end

    def test_closures
      qsort = Fiddle::Function.new(libc["qsort"], [Fiddle::TYPE_VOIDP, Fiddle::TYPE_SIZE_T, Fiddle::TYPE_SIZE_T, Fiddle::TYPE_VOIDP], Fiddle::TYPE_VOID)
      buf = Fiddle::Pointer.malloc(8, Fiddle::RUBY_FREE)
      buf[0, 8] = "a4b3c2d1"
      qsort.call(buf, 4, 2, ByLast.new(Fiddle::TYPE_INT, [Fiddle::TYPE_VOIDP, Fiddle::TYPE_VOIDP]))
      assert_equal "d1c2b3a4", buf[0, 8]
      seen = [] #: Array[Integer]
      add = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_INT, Fiddle::TYPE_INT]) do |x, y|
        seen << x
        x + y
      end
      call = Fiddle::Function.new(add, [Fiddle::TYPE_INT, Fiddle::TYPE_INT], Fiddle::TYPE_INT)
      assert_equal [7, [3], [Fiddle::TYPE_INT, Fiddle::TYPE_INT], Fiddle::TYPE_INT], [call.call(3, 4), seen, add.args, add.ctype]
    end

    def test_closure_float_and_many_arguments
      got = [] #: Array[untyped]
      mixed = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_LONG, [Fiddle::TYPE_DOUBLE, Fiddle::TYPE_INT, Fiddle::TYPE_FLOAT, Fiddle::TYPE_LONG]) do |d, i, f, l|
        got << [d, i, f, l]
        (d * 4 + i + f * 2 + l).to_i
      end
      mixed_fn = Fiddle::Function.new(mixed, [Fiddle::TYPE_DOUBLE, Fiddle::TYPE_INT, Fiddle::TYPE_FLOAT, Fiddle::TYPE_LONG], Fiddle::TYPE_LONG)
      assert_equal [1003, [[1.5, -3, 0.25, 1000]]], [mixed_fn.call(1.5, -3, 0.25, 1000), got]
      seven = Fiddle::Closure::BlockCaller.new(Fiddle::TYPE_INT, [Fiddle::TYPE_INT] * 7) do |a, b, c, d, e, g, h|
        (((((a * 2 + b) * 2 + c) * 2 + d) * 2 + e) * 2 + g) * 2 + h
      end
      seven_fn = Fiddle::Function.new(seven, [Fiddle::TYPE_INT] * 7, Fiddle::TYPE_INT)
      assert_equal 0b1011001, seven_fn.call(1, 0, 1, 1, 0, 0, 1)
    end

    def test_handle_errors
      missing = begin
        libc["rugaroo_no_such_symbol"]
      rescue Fiddle::DLError => e
        e.message
      end
      assert_equal ["unknown symbol \"rugaroo_no_such_symbol\"", true, nil], [missing, libc.sym_defined?("strlen") == libc["strlen"], libc.sym_defined?("rugaroo_nope")]
      assert libc["strlen"] == Fiddle::Handle["strlen"]
    end
  end
end
