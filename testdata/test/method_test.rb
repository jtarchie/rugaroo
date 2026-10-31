# rbs_inline: enabled

require "minitest/autorun"

#: (Integer) -> Integer
def method_twice(n) = n * 2

#: (Integer, Integer) -> Integer
def method_add(a, b) = a + b

#: (Integer, Integer) -> Integer
def method_sub(a, b) = a - b

#: (Integer) -> Integer
def method_neg(a) = -a

#: (Integer, ?Integer) -> Integer
def method_opt(a, b = 10) = a + b

#: (String, *String) -> String
def method_join(head, *rest) = ([head] + rest).join("-")

#: (Integer, ?k: Integer) -> Integer
def method_kw(a, k: 1) = a * k

#: () -> Integer
def method_seven = 7

#: (String) -> void
def method_log(s)
  MethodTests::LOG << s
end

module MethodTests
  LOG = [] #: Array[String]

  class Animal
    #: () -> String
    def speak = "..."

    #: (String) -> String
    def greet(who) = "#{speak} #{who}"

    #: (Integer) -> Integer
    def self.build(x) = x + 1
  end

  class Dog < Animal
    #: () -> String
    def speak = "woof"
  end

  class Counter
    attr_reader :total #: Integer

    #: () -> void
    def initialize
      @total = 0
    end

    #: (Integer) -> void
    def add(n)
      @total += n
    end

    #: (Array[Integer]) -> void
    def add_all(xs)
      xs.each(&method(:add))
    end

    #: () -> Method
    def adder = method(:add)
  end

  class Shape
    #: (String, ?String) -> String
    def label(s, suffix = "") = "shape " + s + suffix

    #: (Integer, ?k: Integer) -> Integer
    def scale(n, k: 2) = n * k
  end

  class Square < Shape
    #: (String, ?String) -> String
    def label(s, suffix = "") = "square " + s + suffix
  end

  class MethodTest < Minitest::Test
    def test_several_double_splats
      a = { print_stdout: false } #: Hash[Symbol, untyped]
      b = { timeout: 3, x: 1 } #: Hash[Symbol, untyped]
      c = { x: 2 } #: Hash[Symbol, untyped]
      assert_equal ["[\"u\"] false 3 {x: 2}", "[\"v\"] false nil {verbose: true, x: 2}"], [method_curl("u", **a, **b, **c), method_curl("v", **a, verbose: true, **c)]
    end

    def test_block_argument
      assert_equal [2, 4], [1, 2].map(&method(:method_twice))
      assert_equal [4, 5], [1, 2].map(&3.method(:+))
      assert_equal 6, [[1, 2], [3]].sum { |xs| xs.sum(&method(:method_neg)) } * -1
      assert_equal [11, 12], [1, 2].map(&method(:method_opt))
      d = Dog.new
      assert_equal ["woof a", "woof b"], ["a", "b"].map(&d.method(:greet))
    end

    def test_block_argument_iterator
      ["x", "y"].each(&method(:method_log))
      assert_equal ["x", "y"], LOG.last(2)
      c = Counter.new
      c.add_all([1, 2, 3])
      assert_equal 6, c.total
    end

    def test_call_forms
      m = method(:method_twice)
      assert_equal 10, m.call(5)
      assert_equal 8, m.(4)
      assert_equal 6, m[3]
      assert_equal 4, m === 2
      assert_equal 7, method(:method_seven).call
      assert_equal 3, method(:method_add).call(1, 2)
    end

    def test_optional_and_rest
      assert_equal 11, method(:method_opt).call(1)
      assert_equal 3, method(:method_opt).call(1, 2)
      assert_equal "a", method(:method_join).call("a")
      assert_equal "a-b-c", method(:method_join).call("a", "b", "c")
      assert_equal 5, method(:method_kw).call(5)
    end

    def test_reflection
      m = method(:method_twice)
      assert_equal 1, m.arity
      assert_equal [[:req, :n]], m.parameters
      assert_equal :method_twice, m.name
      assert_equal Object, m.owner
      assert m.receiver.equal?(self)
      assert_equal(-2, method(:method_opt).arity)
      assert_equal [[:req, :a], [:opt, :b]], method(:method_opt).parameters
      assert_equal(-2, method(:method_join).arity)
      assert_equal [[:req, :head], [:rest, :rest]], method(:method_join).parameters
      assert_equal(-2, method(:method_kw).arity)
      assert_equal [[:req, :a], [:key, :k]], method(:method_kw).parameters
      assert_equal 0, method(:method_seven).arity
      assert_equal [], method(:method_seven).parameters
    end

    def test_core_methods
      plus = 1.method(:+)
      assert_equal 1, plus.arity
      assert_equal [[:req]], plus.parameters
      assert_equal Integer, plus.owner
      assert_equal 1, plus.receiver
      assert_equal "#<Method: Integer#+(_)>", plus.inspect
      center = "x".method(:center)
      assert_equal(-1, center.arity)
      assert_equal [[:rest]], center.parameters
      assert_equal "#<Method: String#center(*)>", center.inspect
      assert_equal "#<UnboundMethod: String#length()>", String.instance_method(:length).inspect
      # a core method MRI writes in Ruby names its parameters: prism's unpack1 polyfill looks for offset (decision 427)
      assert_equal [[:req, :fmt], [:key, :offset]], String.instance_method(:unpack1).parameters
      assert_equal [[:key, :freeze]], Object.instance_method(:clone).parameters
      assert_equal(-1, Object.instance_method(:clone).arity)
      assert_equal "  x  ", center.call(5)
      assert_equal "**x**", center.call(5, "*")
    end

    def test_inspect
      file = __FILE__
      m = method(:method_twice)
      assert_equal "#<Method: MethodTests::MethodTest(Object)#method_twice(n) F:6>", m.inspect.sub(file, "F")
      assert_equal m.inspect, m.to_s
      assert_equal "#<Method: MethodTests::Dog(MethodTests::Animal)#greet(who) F:42>", Dog.new.method(:greet).inspect.sub(file, "F")
      assert_equal "#<Method: MethodTests::Dog#speak() F:50>", Dog.new.method(:speak).inspect.sub(file, "F")
      assert_equal "#<Method: MethodTests::Animal.build(x) F:45>", Animal.method(:build).inspect.sub(file, "F")
      assert_equal "#<UnboundMethod: Object#method_twice(n) F:6>", m.unbind.inspect.sub(file, "F")
      assert_equal "#<Method: MethodTests::MethodTest(Object)#method_opt(a, b=...) F:18>", method(:method_opt).inspect.sub(file, "F")
      assert_equal "#<Method: MethodTests::MethodTest(Object)#method_kw(a, k: ...) F:24>", method(:method_kw).inspect.sub(file, "F")
      assert_equal "#<Method: MethodTests::Counter#total() F:54>", Counter.new.method(:total).inspect.sub(file, "F")
    end

    def test_singleton
      b = Animal.method(:build)
      assert_equal 3, b.call(2)
      assert_equal "#<Class:MethodTests::Animal>", b.owner.inspect
      assert_equal :build, b.name
    end

    def test_to_proc_and_curry
      pr = method(:method_add).to_proc
      assert_equal 5, pr.call(2, 3)
      assert pr.lambda?
      assert_equal 2, pr.arity
      assert_equal 6, method(:method_twice).to_proc.(3)
      assert_equal 3, method(:method_add).curry[1][2]
      assert_equal 6, method(:method_twice).curry[3]
      inc = method(:method_add).curry(2)[1]
      assert_equal [2, 3], [1, 2].map(&inc)
    end

    def test_equality
      m = method(:method_twice)
      assert m == method(:method_twice)
      assert m.eql?(method(:method_twice))
      assert m.hash == method(:method_twice).hash
      refute m == method(:method_neg)
      refute 1.method(:+) == 2.method(:+)
      assert 1.method(:+) == 1.method(:+)
      assert String.instance_method(:upcase) == String.instance_method(:upcase)
      refute String.instance_method(:upcase) == String.instance_method(:downcase)
    end

    def test_unbound
      um = String.instance_method(:upcase)
      assert_equal "ABC", um.bind("abc").call
      assert_equal "XY", um.bind_call("xy")
      assert_equal :upcase, um.name
      assert_equal String, um.owner
      len = String.instance_method(:length)
      assert_equal 0, len.arity
      assert_equal [], len.parameters
      assert_equal 3, len.bind_call("abc")
      g = Animal.instance_method(:greet)
      assert_equal 1, g.arity
      assert_equal [[:req, :who]], g.parameters
      assert_equal "woof you", g.bind_call(Dog.new, "you")
      # MRI binds this definition, not the subclass's override
      assert_equal "...", Animal.instance_method(:speak).bind_call(Dog.new)
      assert_equal "woof", Dog.instance_method(:speak).bind(Dog.new).call
      assert_equal :center, "x".method(:center).unbind.name
      assert_equal 1, method(:method_twice).unbind.arity
    end

    def test_override_found_at_run_time
      a = Dog.new #: Animal
      m = a.method(:speak)
      assert_equal "woof", m.call
      assert_equal Dog, m.owner
      assert_equal "#<Method: MethodTests::Dog#speak() F:50>", m.inspect.sub(__FILE__, "F")
      g = a.method(:greet)
      assert_equal Animal, g.owner
    end

    def test_unbind_rebind
      um = Dog.new.method(:speak).unbind
      assert_equal MethodTests::Dog, um.owner
      d = Dog.new
      m = um.bind(d)
      assert m.receiver.equal?(d)
      assert_equal "woof", m.call
      begin
        um.bind(Animal.new)
        flunk "bind to a superclass instance"
      rescue TypeError => e
        assert_equal "bind argument must be an instance of MethodTests::Dog", e.message
      end
    end

    def test_dispatch_tables
      ops = { add: method(:method_add), sub: method(:method_sub) }
      assert_equal 5, ops[:add]&.call(2, 3)
      assert_equal [3, -1], ops.values.map { |m| m.call(1, 2) }
      list = [method(:method_twice), method(:method_neg)]
      assert_equal [6, -3], list.map { |m| m.call(3) }
      mixed = { twice: method(:method_twice), add: method(:method_add) } #: Hash[Symbol, Method]
      assert_equal 8, mixed.fetch(:twice).call(4)
      assert_equal 9, mixed.fetch(:add).call(4, 5)
      assert_equal [1, 2], mixed.values.map(&:arity)
      table = {} #: Hash[Symbol, Method]
      table[:seven] = method(:method_seven)
      assert_equal 7, table.fetch(:seven).call
    end

    def test_case_when
      even = 2.method(:==)
      got = case 2
            when even then "two"
            else "other"
            end
      assert_equal "two", got
    end

    def test_inside_a_class
      c = Counter.new
      m = c.adder
      m.call(4)
      m.(5)
      assert_equal 9, c.total
      assert m.receiver.equal?(c)
      assert_equal Counter, m.owner
    end

    def test_void_and_kernel
      log = method(:method_log)
      log.call("z")
      assert_equal "z", LOG.last
      pm = method(:puts)
      assert_equal Kernel, pm.owner
      assert_equal(-1, pm.arity)
      assert_equal [[:rest]], pm.parameters
      assert_equal :size, [1, 2].public_method(:size).name
      assert_equal 2, [1, 2].public_method(:size).call
    end

    def test_untyped_receiver
      x = 5 #: untyped
      m = x.method(:+)
      assert_equal 7, m.call(2)
      assert_equal :+, m.name
      assert_equal 5, m.receiver
    end

    def test_optional_args_bind_the_definition
      # through dyn too: MRI binds Shape's label, not Square's override
      assert_equal "shape a!", Shape.instance_method(:label).bind_call(Square.new, "a", "!")
      assert_equal "shape a!", Shape.instance_method(:label).bind(Square.new).call("a", "!")
    end

    def test_struct_target_with_keywords
      assert_equal 6, Shape.new.method(:scale).call(3)
      assert_equal 8, Shape.instance_method(:scale).bind_call(Square.new, 4)
    end

    def test_hash_key_survives_receiver_mutation
      a = [1] #: Array[Integer]
      h = { a.method(:size) => 1 } #: Hash[Method[^() -> Integer], Integer]
      a << 2
      assert_equal 1, h[a.method(:size)]
    end

    def test_class
      assert_equal Method, method(:method_twice).class
      assert_equal UnboundMethod, String.instance_method(:upcase).class
      assert_kind_of Method, method(:method_twice)
    end

    def test_instance_method_on_run_time_class
      assert_equal [ImBase, ImB, ImB], [ImA.new.brew_owner, ImB.new.brew_owner, ImC.new.brew_owner]
      assert_equal [1, 2, 2], [ImA.new.brew_bound, ImB.new.brew_bound, ImC.new.brew_bound]
    end
  end

  # instance_method on self.class: the class is the receiver's at run time
  class ImBase
    def brew = 1
    def brew_owner = self.class.instance_method(:brew).owner
    def brew_bound = self.class.instance_method(:brew).bind_call(self)
  end

  class ImA < ImBase; end

  class ImB < ImBase
    def brew = 2
  end

  class ImC < ImB; end
end

# several **splats (and a keyword between them) into named keywords and **rest: later keys win, in order (Homebrew's
# CurlDownloadStrategy#curl)
#: (*String, ?print_stdout: bool, ?timeout: Integer?, **untyped) -> String
def method_curl(*args, print_stdout: true, timeout: nil, **options) = "#{args} #{print_stdout} #{timeout.inspect} #{options}"

module MethodTests
  # `yield(*xs)`: the block takes the elements, `{ (*T) -> R }`, and a tail yield is the method's value (decision 363)
  class SplatYielder
    def m(*a)
      yield(*a)
    end

    def k(*a)
      yield(*a, 9)
    end

    def never(*a)
      yield(*a)
    end
  end

  class SplatYieldTest < Minitest::Test
    def test_yield_splat
      y = SplatYielder.new
      assert_equal 3, y.m(1, 2) { |x, z| x + z }
      assert_equal 1, y.m(1, 2) { |x| x }
      assert_equal [1, 2], y.m(1, 2) { |*r| r }
      assert_equal [4, 9], y.k(4) { |x, z| [x, z] }
    end
  end
end

module MethodTests
  # decision 364: a block to a method that never yields is ignored; a reassigned &block keeps yield on the block; a
  # proc literal for an untyped (*untyped) Proc takes any count; Proc literals answer lambda?/arity/parameters
  class NoYield
    def plain(x) = x + 1

    def ze(&block)
      block = proc { block }
      yield
    end
  end

  class ProcInfoTest < Minitest::Test
    def test_ignored_block
      assert_equal 2, NoYield.new.plain(1) { raise "never" }
    end

    def test_reassigned_block_param
      assert_equal 3, NoYield.new.ze { 3 }
    end

    def test_proc_literal_info
      assert_equal [[:opt, :x], [:opt, :y]], proc { |x, y| }.parameters
      assert_equal [[:req, :x], [:opt, :y]], ->(x, y = 1) {}.parameters
      assert_equal [[:keyreq, :k], [:key, :o], [:keyrest, :kr], [:block, :b]], proc { |k:, o: 1, **kr, &b| }.parameters
      assert_equal [], proc {}.parameters
      assert_equal [0, -2, 2, -1, 0], [proc { |x = 1| }.arity, ->(x, y = 1) {}.arity, proc { _1 + _2 }.arity, ->(x: 1) {}.arity, proc { |x: 0| }.arity]
      assert_equal [false, true, true], [proc {}.lambda?, -> {}.lambda?, lambda {}.lambda?]
    end
  end
end

module MethodTests
  # decision 365: a used tail yield is the method's value, a used core each its receiver, as MRI's
  class TailValues
    def m(a) = yield(a)

    def n(a) = yield(a)

    def last_each(xs)
      xs.each { |x| x }
    end

    def quiet(xs)
      xs.each { |x| x }
    end
  end

  class TailValueTest < Minitest::Test
    def test_tail_yield_and_each_values
      t = TailValues.new
      ary = [1, 2]
      assert_same ary, t.m(ary) { |a| a }
      assert_equal 3, t.n(1) { |a| a + 2 }
      assert_equal [7, 8], t.last_each([7, 8])
      r = [7].each_with_index { |x, i| x + i }
      assert_equal [7], r
      assert_equal [5], [5].send(:each) { |x| x }
      t.quiet([1])
    end
  end
end

module MethodTests
  # decision 367: MRI's split of a proc's arguments over leading, optional, rest and trailing names
  class ProcArgSplit
    def m(a) = yield(a)
  end

  class ProcArgSplitTest < Minitest::Test
    def test_proc_argument_split
      s = ProcArgSplit.new
      assert_equal [1, 5, 6, nil, nil], s.m([1]) { |a, b = 5, c = 6, d, e| [a, b, c, d, e] }
      assert_equal [1, 2, 6, 3, 4], s.m([1, 2, 3, 4]) { |a, b = 5, c = 6, d, e| [a, b, c, d, e] }
      assert_equal [1, 2, [3, 4, 5], 6, 7], s.m([1, 2, 3, 4, 5, 6, 7]) { |a, b = 5, *c, d, e| [a, b, c, d, e] }
      assert_equal [1, [], 2, nil], proc { |a, *b, c, d| [a, b, c, d] }.call(1, 2)
      assert_equal [[:opt, :a], [:rest, :b], [:opt, :c]], proc { |a, *b, c| }.parameters
    end
  end
end

module MethodTests
  # decision 369: nested destructuring parameters, keyword parameters a method yields none to, `|a = nil|`
  class NestedBlockParams
    def m(*a) = yield(*a)
    def one(a) = yield(a)
    def other(a) = yield(a)
  end

  class NestedBlockParamsTest < Minitest::Test
    def test_nested_and_keyword_block_params
      y = NestedBlockParams.new
      assert_equal [1, 2, 3, nil], y.m(1, [2, 3]) { |a, (b, (c, d))| [a, b, c, d] }
      assert_equal [1, 2, nil, 3], y.m(1, [2, 3]) { |a, ((b, c), d)| [a, b, c, d] }
      assert_equal [1, [2, 3], 4, 5, 6], proc { |(a, *b, c), d, e| [a, b, c, d, e] }.call([1, 2, 3, 4], 5, 6)
      assert_equal [[1, 2], :b, :c], y.one([1, 2]) { |a, b: :b, c: :c| [a, b, c] }
      assert_equal [[1, 2], {}], y.one([1, 2]) { |a, **k| [a, k] }
      assert_equal [[{ "a" => 10 }]], y.other(["a" => 10]) { |a = nil| [a] }
    end
  end
end

module MethodTests
  # Proc#parameters: one entry per parameter with its kind (lambda kinds, as rugaroo's lambda? answers; a Go func has no
  # names, so only the kinds and the count are MRI's here).
  class ProcParametersTest < Minitest::Test
    def test_parameter_kinds_and_count
      two = ->(a, b) { a + b }
      splat = ->(a, *rest) { [a, rest] }
      assert_equal 2, two.parameters.size
      assert_equal %i[req req], two.parameters.map(&:first)
      assert_equal %i[req rest], splat.parameters.map(&:first)
      assert_equal 2, two.parameters.map(&:last).uniq.size
    end
  end
end
