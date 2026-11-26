# rbs_inline: enabled

require "minitest/autorun"
require "open3"

# Shared helpers (the old files each defined their own identical copies).

#: (untyped) -> untyped
def dynamic_ident(v) = v

#: (Integer) -> Integer?
def dynamic_maybe(n) = n.positive? ? n : nil

# tuples reaching untyped code through Array(x) answer as Arrays (Homebrew's ErrorDuringExecution#stderr)
#: (Array[[Symbol, String]]?) -> String
def dynamic_stderr(out) = Array(out).select { |type,| type == :stderr }.map(&:last).join

#: (Integer) -> String?
def dynamic_word(n) = n.positive? ? "w" * n : nil

#: (untyped?) -> String
def dynamic_show(v) = v.inspect

#: (DynamicTests::CondShape) -> String
def big(s)
  if s.is_a?(DynamicTests::CondCircle) && s.radius > 1
    "big circle"
  else
    "other"
  end
end

#: (untyped) -> String
def long(v)
  if v.is_a?(String) && v.size > 2
    "long string"
  else
    "other"
  end
end

#: (untyped, bool) -> String
def flagged(v, flag)
  v.is_a?(String) && flag ? "yes" : "no"
end

#: (untyped, bool) -> String
def either(v, flag)
  v.is_a?(Integer) || flag ? "yes" : "no"
end

# Helpers for the checks that were testdata/run/dynamic_bugs.rb.

#: (untyped) -> String
def f(v)
  case v
  when Object then "object"
  else "not an object"
  end
end

#: (String) -> Integer
def len(s) = s.size

#: (Integer) -> Integer?
def maybe_isa(n) = n.positive? ? n : nil

#: (Integer) -> Integer?
def maybe_elem(n) = n.positive? ? n : nil

#: (untyped) -> untyped
def ident_safe(v) = v

#: (untyped) -> untyped
def dynamic_ident_class(v) = v

#: (Integer) -> Integer?
def maybe_opt(n) = n.positive? ? n : nil

#: (untyped) -> String
def dynamic_classify(v)
  case v
  when nil then "nil"
  when Integer then "Integer #{v + 1}"
  when Float then "Float #{v * 2}"
  when String then "String #{v.upcase} (#{v.size})"
  when Symbol then "Symbol #{v.inspect}"
  when Array then "Array of #{v.size}: #{v.inspect}"
  when Hash then "Hash with keys #{v.keys.inspect}"
  when DynamicTests::Puppy then "Puppy: #{v.yip}"
  when DynamicTests::Dog then "Dog: #{v.bark}"
  when DynamicTests::Animal then "Animal #{v.name}"
  else "something else: #{v.inspect}"
  end
end

#: (untyped) -> String
def numeric_or_text(v)
  case v
  when Integer, Float then "number #{v.inspect}"
  when String, Symbol then "text #{v.inspect}"
  when nil then "nothing"
  else "other"
  end
end

#: (DynamicTests::Animal) -> String
def speak(a)
  case a
  when DynamicTests::Puppy then a.yip
  when DynamicTests::Dog then a.bark
  when DynamicTests::Cat then a.meow
  else "... from #{a.name}"
  end
end

#: (DynamicTests::Animal) -> String
def first_match_wins(a)
  case a
  when DynamicTests::Animal then "Animal first"
  when DynamicTests::Dog then "never Dog"
  else "never else"
  end
end

#: (String?) -> String
def opt_case(s)
  case s
  when nil then "nil string"
  when String then "string #{s.size}"
  else "unreachable"
  end
end

#: (untyped) -> String
def staff(v)
  case v
  when DynamicTests::Zoo::Vet then "vet #{v.name}"
  when DynamicTests::Zoo::Keeper then "keeper #{v.name}"
  else "visitor"
  end
end

#: (untyped) -> String
def dynamic_what(v)
  case v
  when Class then "class #{v.name}"
  when Module then "module #{v.name}"
  when Exception then "exception #{v.message}"
  else "value"
  end
end

#: (untyped) -> String
def shape_of(v)
  case v
  when Array
    v.select { |e| !e.nil? }.map { |e| e.to_s }.join(",")
  when Hash
    v.keys.map { |k| k.to_s }.sort.join("/")
  else
    "leaf"
  end
end

#: (DynamicTests::Animal?) -> String
def opt_animal(a)
  case a
  when DynamicTests::Dog then "dog #{a.bark}"
  when nil then "no animal"
  else "some animal"
  end
end

#: (Integer?) -> String
def opt_int(n)
  case n
  when Integer then "int #{n + 1}"
  else "nil"
  end
end

#: (DynamicTests::Vehicle) -> String
def checks(v)
  [v.is_a?(DynamicTests::Vehicle), v.is_a?(DynamicTests::Car), v.kind_of?(DynamicTests::Truck), v.is_a?(Object), v.is_a?(BasicObject), v.is_a?(Kernel)].inspect
end

#: (untyped) -> String
def untyped_checks(v)
  [v.is_a?(Integer), v.is_a?(Float), v.is_a?(String), v.is_a?(Symbol), v.is_a?(Array), v.is_a?(Hash), v.is_a?(DynamicTests::Vehicle), v.is_a?(DynamicTests::Car), v.is_a?(Object)].inspect
end

#: (untyped) -> String
def dynamic_walk(v)
  if v.is_a?(Array)
    v.map { |e| e.to_s }.join("+")
  elsif v.is_a?(Hash)
    out = [] #: Array[String]
    v.each { |k, x| out << "#{k}=#{x}" }
    out.join(",")
  elsif v.is_a?(DynamicTests::Geo::Pt)
    "pt #{v.x}"
  else
    "leaf"
  end
end

#: (DynamicTests::Base) -> String
def dynamic_probe(b) = "#{b.name} #{b.respond_to?(:name)} #{b.respond_to?(:extra)} #{b.respond_to?(:zzz)}"

# Helpers for the checks that were testdata/run/dynamic_mid.rb.
#: (Integer) -> String
def typed_int(n)
  case n
  when Float then "float"
  when Integer then "int #{n + 1}"
  else "?"
  end
end

#: (String) -> String
def typed_str(s)
  case s
  when Symbol then "symbol"
  when String then "string #{s.size}"
  else "?"
  end
end

#: (DynamicTests::MidAnimal?) -> String
def describe_animal(a)
  case a
  when DynamicTests::MidDog then "dog #{a.bark}"
  when nil then "none"
  else "other #{a.name}"
  end
end

#: (String?) -> String
def text(s)
  case s
  when nil then "nil"
  else "text #{s.size}"
  end
end

#: (untyped) -> untyped
def ident_dyn(v) = v

#: (untyped) -> untyped
def ident_var(v) = v

#: (untyped) -> void
def push_if_array(v)
  v << 99 if v.is_a?(Array)
end

#: (untyped) -> void
def push_case(v)
  case v
  when Array then v << 7
  end
end

#: (untyped) -> untyped
def ident_rest(v) = v

#: (untyped) -> untyped
def ident_opt(v) = v

#: (untyped) -> String
def nils(v)
  if v.is_a?(Array)
    v.map { |e| e.nil? }.inspect
  else
    "?"
  end
end

#: (untyped) -> String
def nils_case(v)
  case v
  when Array then v.select { |e| e.nil? }.size.to_s
  else "?"
  end
end

#: (untyped) -> untyped
def ident_priv(v) = v

# respond_to?(name, true) sees private methods, typed and untyped
#: (untyped) -> untyped
def ident_resp(v) = v

#: (untyped) -> untyped
def ident_send(v) = v

#: (untyped) -> untyped
def ident_type(v) = v

#: (String?) -> Integer
def len_if(s)
  if s
    s.size
  else
    -1
  end
end

#: (String?) -> Integer
def len_unless(s)
  return -1 unless s

  s.size
end

#: (String?) -> Integer
def len_nil_guard(s)
  return -2 if s.nil?

  s.size + 1000
end

#: (String?) -> Integer
def len_bang_guard(s)
  return -3 if !s

  s.size + 2000
end

#: (String?) -> Integer
def len_raise_guard(s)
  raise ArgumentError, "need a string" unless s

  s.size
end

#: (String?, String?) -> String
def both(a, b)
  if a && b
    a + b
  elsif a
    "only a=#{a.upcase}"
  elsif b
    "only b=#{b.upcase}"
  else
    "neither"
  end
end

#: (String?) -> String
def long_word(s)
  if s && s.size > 3
    "long #{s}"
  else
    "short or nil"
  end
end

#: (DynamicTests::NShape) -> String
def narrow_area_of(s)
  return "#{s.name}: #{s.radius * s.radius * 3}" if s.is_a?(DynamicTests::NCircle)
  return "#{s.name}: #{s.side * s.side}" if s.kind_of?(DynamicTests::NSquare)

  "#{s.name}: ?"
end

#: (DynamicTests::NShape) -> Integer
def diameter_guard(s)
  return -1 unless s.is_a?(DynamicTests::NCircle)

  s.diameter
end

#: (DynamicTests::NShape?) -> String
def opt_shape(s)
  if s.is_a?(DynamicTests::NCircle)
    "opt circle #{s.radius}"
  elsif s
    "opt #{s.name}"
  else
    "opt nil"
  end
end

#: (untyped) -> String
def untyped_narrow(v)
  if v.is_a?(String)
    "string of #{v.size}: #{v.upcase}"
  elsif v.is_a?(Integer)
    "integer doubled #{v * 2}"
  elsif v.is_a?(Array)
    "array of #{v.size}"
  elsif v.is_a?(Hash)
    "hash of #{v.size}"
  elsif v.is_a?(Float)
    "float #{v * 2}"
  elsif v.nil?
    "nil"
  else
    "other"
  end
end

#: (String) -> Integer
def strict_len(s) = s.size

#: (Array[String?]) -> Integer
def total_len(ws)
  sum = 0
  ws.each do |w|
    next if w.nil?

    sum += strict_len(w)
  end
  sum
end

#: (Array[String?]) -> Integer
def first_len(ws)
  ws.each do |w|
    return strict_len(w) if w
  end
  -1
end

#: (String?, String?) -> Integer
def both_or(a, b)
  return -1 unless a && b

  strict_len(a) + strict_len(b)
end

#: (String?) -> String
def dynamic_triple(q)
  if q && strict_len(q) > 2 && q.start_with?("w")
    "long w-word #{strict_len(q)}"
  else
    "no"
  end
end

#: (untyped) -> String
def unwrap(v)
  depth = 0
  while v.is_a?(Array)
    v = v[0]
    depth += 1
  end
  "#{depth}:#{v.inspect}"
end

#: (untyped) -> String
def retype(v)
  if v.is_a?(String)
    n = v.size
    v = n * 2
    return "was a string, now #{v.inspect} #{v.is_a?(Integer)}"
  end
  "not a string"
end

#: (Integer?) -> String
def describe_opt(n)
  return "nothing" if n.nil?

  "got #{n + 0}"
end

#: (Integer) -> DynamicTests::Pt?
def pt(n) = n.positive? ? DynamicTests::Pt.new(n) : nil

#: (Integer) -> String?
def only_even(n)
  "even" if n.even?
end

#: (DynamicTests::Shape) -> String
def area_of(s) = s.area.inspect

#: (DynamicTests::Shape) -> String
def sent_area(s) = s.send(:area).inspect

#: (Integer) -> DynamicTests::Shape?
def dynamic_pick(n) = n.zero? ? nil : (n.positive? ? DynamicTests::Sq.new("p") : DynamicTests::Blob.new("q"))

#: (untyped) -> String
def truth(v)
  if v
    "truthy"
  else
    "falsy"
  end
end

#: (untyped) -> String
def kind_of_value(v)
  case v
  when nil then "nil"
  when String then "String"
  else "other"
  end
end

#: (String) -> Integer
def strlen(s) = s.size

#: (untyped?) -> untyped?
def pass_on(v) = v

#: (bool) -> String
def dynamic_yes_no(b) = b ? "y" : "n"

#: (Integer?) -> String
def opt_desc(n) = n.nil? ? "none" : "n=#{n}"

#: (Array[Integer]) -> Integer
def total_of(a) = a.reduce(0) { |s, x| s + x }

#: (String) -> String
def dynamic_up(s) = s.upcase

module DynamicTests
  module Tools
  end

  class Plain
    #: () -> String
    def hello = "hi"

    #: (Integer) -> Integer
    def twice(n) = n * 2

    private

    #: () -> String
    def hidden = "hidden"
  end

  class Pt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Pt) && other.x == x
  end

  # Helpers for the checks that were testdata/run/dynamic_bug_default_arg_scope.rb.
  class ScopeFoo
    #: () -> void
    def initialize
      @x = 5
    end

    #: (String, ?Integer) -> Integer
    def f(a, b = a.size) = b

    #: (?Integer) -> Integer
    def get(a = @x) = a

    #: (?String) -> String
    def greet(g = greeting) = g + "!"

    #: () -> String
    def greeting = "hi from Foo"
  end

  class ScopeBar
    #: () -> void
    def initialize
      @x = 99
    end

    #: () -> String
    def greeting = "hi from Bar"

    #: (ScopeFoo) -> String
    def ask(foo) = "#{foo.get} #{foo.greet}"
  end

  class DynamicDefaultArgScopeTest < Minitest::Test
    def test_defaults_see_the_callee_self_and_earlier_params
      assert_equal "5 hi from Foo!", ScopeBar.new.ask(ScopeFoo.new)
      assert_equal 5, ScopeFoo.new.f("typed")
      assert_equal 3, dynamic_ident(ScopeFoo.new).f("abc")
      assert_equal 7, dynamic_ident(ScopeFoo.new).f("abc", 7)
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_bug_isa_and_cond.rb.
  class CondShape
  end

  class CondCircle < CondShape
    #: () -> Integer
    def radius = 3
  end

  class DynamicIsaAndCondTest < Minitest::Test
    def test_is_a_narrows_the_right_side_of_and
      assert_equal "big circle", big(CondCircle.new)
      assert_equal "other", big(CondShape.new)
      assert_equal "long string", long("abc")
      assert_equal "other", long("a")
      assert_equal "other", long(1)
    end

    def test_is_a_inside_a_ternary_condition
      assert_equal "yes", flagged("s", true)
      assert_equal "no", flagged("s", false)
      assert_equal "no", flagged(1, true)
      assert_equal "yes", either(1, false)
      assert_equal "yes", either("s", true)
      assert_equal "no", either("s", false)
    end
  end

  class BugVehicle
  end

  class BugCar < BugVehicle
  end

  # a public method_missing is still respond_to?-visible
  class MmConfig
    #: (Symbol, *untyped) -> String
    def method_missing(name, *args) = "mm #{name}"

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = false
  end

  class Foo
  end

  class DynamicBugsTest < Minitest::Test
    def test_tuple_through_untyped
      assert_equal ["ac", ""], [dynamic_stderr([[:stderr, "a"], [:stdout, "b"], [:stderr, "c"]]), dynamic_stderr(nil)]
    end

    def test_case_when_object_matches_anything_but_nil
      assert_equal "object", f(1)
      assert_equal "object", f(nil)
      assert_equal "object", f(false)
    end

    def test_is_a_across_a_user_subclass_and_builtins
      assert_equal true, :s.is_a?(Symbol)
      assert_equal true, [1].is_a?(Array)
      assert_equal true, { "a" => 1 }.is_a?(Hash)
      assert_equal true, BugCar.new.is_a?(BugVehicle)
      assert_equal false, BugCar.new.kind_of?(String)
    end

    def test_is_a_string_narrows_a_nilable_before_a_typed_call
      s = dynamic_word(2)
      got = -1
      got = len(s) if s.is_a?(String)
      assert_equal 2, got
      t = dynamic_word(0)
      assert_equal(-1, t.is_a?(String) ? len(t) : -1)
    end

    def test_is_a_object_on_nil
      none = maybe_isa(-1)
      assert_equal true, none.is_a?(Object)
      assert_equal true, none.kind_of?(BasicObject)
      assert_equal true, none.is_a?(Kernel)
      assert_equal false, none.is_a?(Integer)
      assert_equal true, maybe_isa(2).is_a?(Object)
    end

    def test_public_method_missing_is_respond_to_visible
      c = MmConfig.new
      assert_equal true, c.respond_to?(:method_missing)
      assert_equal false, c.respond_to?(:respond_to_missing?)
    end

    def test_nilable_element_equality_and_include
      elems = [maybe_elem(1), maybe_elem(2)] #: Array[Integer?]
      assert_equal true, elems == [maybe_elem(1), maybe_elem(2)]
      assert_equal true, elems.include?(maybe_elem(2))
      assert_equal true, elems.include?(2)
    end

    def test_respond_to_on_a_typed_method
      p1 = Plain.new
      assert_equal true, p1.respond_to?(:hello)
      assert_equal false, p1.respond_to?(:nope)
    end

    def test_safe_navigation_on_untyped_values
      u = ident_safe(nil)
      assert_nil u&.size
      v = ident_safe("abc")
      assert_equal 3, v&.size
      h = { "n" => nil, "s" => "str" } #: Hash[String, untyped]
      assert_equal 3, h["s"]&.size
      assert_nil h["zz"]&.size
      assert_nil h["n"]&.size
      a = [nil, "x"] #: Array[untyped]
      assert_nil a[0]&.size
    end

    def test_class_and_class_name_on_untyped_values
      assert_equal "Integer", dynamic_ident_class(1).class.to_s
      assert_equal "String", dynamic_ident_class("s").class.name
      assert_equal "DynamicTests::Foo", dynamic_ident_class(Foo.new).class.to_s
      assert_equal "Array", dynamic_ident_class([1]).class.to_s
      assert_equal "NilClass", dynamic_ident_class(nil).class.to_s
      assert_equal "Float", "#{dynamic_ident_class(2.5).class}"
    end

    def test_untyped_nilable_parameter
      assert_equal "nil", dynamic_show(maybe_opt(-1))
      assert_equal "4", dynamic_show(maybe_opt(4))
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_case.rb.
  class Animal
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end
  end

  class Dog < Animal
    #: () -> String
    def bark = "woof from #{name}"
  end

  class Puppy < Dog
    #: () -> String
    def yip = "yip from #{name}"
  end

  class Cat < Animal
    #: () -> String
    def meow = "meow from #{name}"
  end

  module Zoo
    class Keeper
      #: () -> String
      def name = "keeper"
    end

    class Vet < Keeper
    end
  end

  class DynamicCaseTest < Minitest::Test
    VALS = [nil, 0, -7, 2.5, "héllo", "", :sym, [1, "a"], [], { "a" => 1, "b" => 2 }, {}, true, false] #: Array[untyped]

    def test_case_when_on_classes_over_untyped_values
      assert_equal ["nil", "Integer 1", "Integer -6", "Float 5.0", "String HÉLLO (5)", "String  (0)", "Symbol :sym",
                    "Array of 2: [1, \"a\"]", "Array of 0: []", "Hash with keys [\"a\", \"b\"]", "Hash with keys []",
                    "something else: true", "something else: false"], VALS.map { |v| dynamic_classify(v) }
      assert_equal "Puppy: yip from rex", dynamic_classify(Puppy.new("rex"))
      assert_equal "Dog: woof from fido", dynamic_classify(Dog.new("fido"))
      assert_equal "Animal tom", dynamic_classify(Cat.new("tom"))
      assert_equal "Animal gen", dynamic_classify(Animal.new("gen"))
    end

    def test_several_classes_in_one_when
      assert_equal ["nothing", "number 0", "number -7", "number 2.5", "text \"héllo\"", "text \"\"", "text :sym",
                    "other", "other", "other", "other", "other", "other"], VALS.map { |v| numeric_or_text(v) }
    end

    def test_case_on_a_typed_struct_subject_narrows_to_subclasses
      animals = [Puppy.new("p"), Dog.new("d"), Cat.new("c"), Animal.new("a")] #: Array[Animal]
      assert_equal ["yip from p", "woof from d", "meow from c", "... from a"], animals.map { |a| speak(a) }
      assert_equal ["Animal first", "Animal first", "Animal first", "Animal first"], animals.map { |a| first_match_wins(a) }
    end

    def test_case_on_a_nilable
      assert_equal "nil string", opt_case(nil)
      assert_equal "string 3", opt_case("abc")
      assert_equal "string 0", opt_case("")
    end

    def test_case_on_an_expression_as_a_value
      kind = case dynamic_ident(42)
             when String then "s"
             when Integer then "i"
             else "?"
             end
      assert_equal "i", kind
      label = case dynamic_ident(nil)
              when nil then "was nil"
              else "not nil"
              end
      assert_equal "was nil", label
      out = case dynamic_ident([3, 4])
            when Array then "array"
            else "no"
            end
      assert_equal "array", out
    end

    def test_case_without_a_matching_branch_and_no_else_yields_nil
      res = case dynamic_ident(1.5)
            when String then "s"
            when Integer then "i"
            end
      assert_equal "nil", res.inspect
    end

    def test_case_when_on_exception_objects
      errs = [ArgumentError.new("bad arg"), KeyError.new("no key"), RuntimeError.new("boom")].map do |err|
        case err
        when ArgumentError then "arg: #{err.message}"
        when KeyError then "key: #{err.message}"
        when StandardError then "std: #{err.message}"
        else "?"
        end
      end
      assert_equal ["arg: bad arg", "key: no key", "std: boom"], errs
    end

    def test_case_when_on_values
      words = [1, 2, 5, 10].map do |n|
        case n
        when 1 then "one"
        when 2, 3 then "two or three"
        else "many"
        end
      end
      assert_equal ["one", "two or three", "many", "many"], words
      kinds = ["apple", "Banana", "cherry", ""].map do |s|
        case s
        when /^a/ then "starts with a"
        when /^[A-Z]/ then "capitalized"
        when "" then "empty"
        else "other"
        end
      end
      assert_equal ["starts with a", "capitalized", "other", "empty"], kinds
    end

    def test_namespaced_classes_class_objects_and_exceptions_in_when
      assert_equal "vet keeper", staff(Zoo::Vet.new)
      assert_equal "keeper keeper", staff(Zoo::Keeper.new)
      assert_equal "visitor", staff("x")
      assert_equal "class DynamicTests::Dog", dynamic_what(Dog)
      assert_equal "module DynamicTests::Tools", dynamic_what(Tools)
      assert_equal "class String", dynamic_what(String)
      assert_equal "module Comparable", dynamic_what(Comparable)
      assert_equal "exception kk", dynamic_what(KeyError.new("kk"))
      assert_equal "value", dynamic_what(3)
      assert_equal "value", dynamic_what(nil)
    end

    def test_narrowed_arms_take_blocks
      assert_equal "1,b,2.5", shape_of([1, nil, "b", 2.5])
      assert_equal "a/b", shape_of({ "b" => 1, :a => 2 })
      assert_equal "leaf", shape_of("s")
    end

    def test_class_whens_on_a_nilable_subject_narrow_it
      assert_equal "dog woof from pp", opt_animal(Puppy.new("pp"))
      assert_equal "no animal", opt_animal(nil)
      assert_equal "some animal", opt_animal(Cat.new("cc"))
      assert_equal "int 5", opt_int(4)
      assert_equal "nil", opt_int(nil)
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_is_a.rb.
  module Named
    #: () -> String
    def label = "named"
  end

  class Vehicle
    #: () -> String
    def kind = "vehicle"

    #: () -> bool
    def car? = is_a?(Car)

    #: () -> bool
    def self_truck? = self.kind_of?(Truck)
  end

  class Car < Vehicle
    include Named

    #: () -> String
    def kind = "car"
  end

  class Truck < Vehicle
  end

  module Geo
    class Pt
      attr_reader :x #: Integer

      #: (Integer) -> void
      def initialize(x)
        @x = x
      end
    end

    class Pt3 < Pt
    end
  end

  class DynamicIsATest < Minitest::Test
    def test_static_types_decide_is_a_on_primitives
      assert_equal [true, false, true, true, true, true], [1.is_a?(Integer), 1.is_a?(Float), 1.is_a?(Comparable), 1.is_a?(Object), 1.kind_of?(BasicObject), 1.is_a?(Kernel)]
      assert_equal [true, false, true], [1.5.is_a?(Float), 1.5.is_a?(Integer), 1.5.is_a?(Comparable)]
      assert_equal [true, true, false, true], ["s".is_a?(String), "s".is_a?(Comparable), "s".is_a?(Symbol), "".kind_of?(Object)]
      sym = :s
      arr = [1]
      hsh = { "a" => 1 }
      assert_equal [true, false, true], [sym.is_a?(Symbol), sym.is_a?(String), sym.is_a?(Comparable)]
      assert_equal :s, sym
      assert_equal [true, true, false, true], [arr.is_a?(Array), arr.is_a?(Enumerable), arr.is_a?(Hash), arr.kind_of?(Object)]
      assert_equal [1], arr
      assert_equal [true, true, false], [hsh.is_a?(Hash), hsh.is_a?(Enumerable), hsh.is_a?(Array)]
      assert_equal "{\"a\" => 1}", hsh.inspect
      assert_equal [true, true, false], [true.is_a?(Object), false.kind_of?(Object), nil.is_a?(Integer)]
      pair = [1, "a"]
      assert_equal [true, true, false], [pair.is_a?(Array), pair.is_a?(Object), pair.is_a?(Hash)]
      assert_equal [1, "a"], pair
    end

    def test_struct_classes_constant_when_static_else_runtime_check
      assert_equal "[true, false, false, true, true, true]", checks(Vehicle.new)
      assert_equal "[true, true, false, true, true, true]", checks(Car.new)
      assert_equal "[true, false, true, true, true, true]", checks(Truck.new)
      car = Car.new
      truck = Truck.new
      assert_equal [true, false, true, false], [car.is_a?(Vehicle), car.is_a?(Truck), car.is_a?(Named), truck.kind_of?(Car)]
      assert_equal "car", car.kind
      assert_equal "vehicle", truck.kind
    end

    def test_untyped_values_use_a_type_assertion
      vals = [1, -2.5, "str", :sym, [1], { 1 => 2 }, Vehicle.new, Car.new, Truck.new, nil, true, false] #: Array[untyped]
      assert_equal ["[true, false, false, false, false, false, false, false, true]",
                    "[false, true, false, false, false, false, false, false, true]",
                    "[false, false, true, false, false, false, false, false, true]",
                    "[false, false, false, true, false, false, false, false, true]",
                    "[false, false, false, false, true, false, false, false, true]",
                    "[false, false, false, false, false, true, false, false, true]",
                    "[false, false, false, false, false, false, true, false, true]",
                    "[false, false, false, false, false, false, true, true, true]",
                    "[false, false, false, false, false, false, true, false, true]",
                    "[false, false, false, false, false, false, false, false, true]",
                    "[false, false, false, false, false, false, false, false, true]",
                    "[false, false, false, false, false, false, false, false, true]"], vals.map { |v| untyped_checks(v) }
      assert_equal true, dynamic_ident(Car.new).is_a?(Vehicle)
      assert_equal false, dynamic_ident(Truck.new).is_a?(Car)
      assert_equal true, dynamic_ident([]).kind_of?(Array)
      assert_equal true, dynamic_ident({}).kind_of?(Hash)
    end

    def test_exceptions
      errs = [ArgumentError.new("a"), KeyError.new("k"), RuntimeError.new("r"), StandardError.new("s")] #: Array[StandardError]
      got = errs.map do |e|
        [e.is_a?(StandardError), e.is_a?(ArgumentError), e.is_a?(KeyError), e.is_a?(IndexError), e.is_a?(RuntimeError), e.is_a?(Exception)]
      end
      assert_equal [[true, true, false, false, false, true],
                    [true, false, true, true, false, true],
                    [true, false, false, false, true, true],
                    [true, false, false, false, false, true]], got
      e = assert_raises(StandardError) { raise KeyError, "missing" }
      assert_equal [true, true, false], [e.is_a?(KeyError), e.is_a?(IndexError), e.kind_of?(ArgumentError)]
      assert_equal "missing", e.message
    end

    def test_class_objects
      assert_equal [true, true, true, false, true], [Car.is_a?(Class), Car.is_a?(Module), Named.is_a?(Module), Named.is_a?(Class), Car.is_a?(Object)]
      assert_equal [true, true, true, false], [dynamic_ident(Car).is_a?(Class), dynamic_ident(Named).is_a?(Module), dynamic_ident(Car).is_a?(Module), dynamic_ident(1).is_a?(Class)]
    end

    def test_namespaced_classes
      pt3 = Geo::Pt3.new(2)
      assert_equal true, dynamic_ident(Geo::Pt3.new(1)).is_a?(Geo::Pt)
      assert_equal false, dynamic_ident(Geo::Pt.new(1)).is_a?(Geo::Pt3)
      assert_equal true, pt3.kind_of?(Geo::Pt)
      assert_equal 2, pt3.x
    end

    def test_narrowed_untyped_values_take_blocks
      assert_equal "1+a++2.5", dynamic_walk([1, "a", nil, 2.5])
      assert_equal "k=1,s=[2]", dynamic_walk({ "k" => 1, :s => [2] })
      assert_equal "pt 7", dynamic_walk(Geo::Pt3.new(7))
      assert_equal "leaf", dynamic_walk(3)
      assert_equal "", dynamic_walk([])
    end

    def test_untyped_exceptions
      ex = dynamic_ident(KeyError.new("k"))
      assert_equal true, ex.is_a?(IndexError)
      assert_equal true, ex.is_a?(StandardError)
      assert_equal false, ex.is_a?(ArgumentError)
      assert_equal true, ex.is_a?(Exception)
      assert_equal true, ex.kind_of?(KeyError)
    end

    def test_is_a_on_self_in_a_method_body_is_a_runtime_check
      assert_equal false, Vehicle.new.car?
      assert_equal true, Car.new.car?
      assert_equal false, Truck.new.car?
      assert_equal true, Truck.new.self_truck?
      assert_equal false, Car.new.self_truck?
    end

    # Class values (decision 76).
    def test_module_case_equality_and_class_values
      cv_k = Car #: singleton(Vehicle)
      assert_equal [true, false, true, true, false], [cv_k === Car.new, cv_k === Truck.new, Vehicle === Car.new, Named === Car.new, Named === Truck.new]
      assert_equal [true, true, false, true, true, false], [Integer === 3, Comparable === "s", String === :s, NilClass === nil, TrueClass === true, FalseClass === true]
      cv_e = KeyError.new("x")
      assert_equal [true, true, true, false], [Exception === cv_e, StandardError === cv_e, IndexError === cv_e, ArgumentError === cv_e]
      cv_kinds = [KeyError, ArgumentError, IndexError] #: Array[singleton(StandardError)]
      assert_equal [true, false, true], cv_kinds.map { |c| cv_e.is_a?(c) }
      assert_equal [true, false, true], cv_kinds.map { |c| c === cv_e }
      cv_m = Named #: Module
      assert_equal [true, false], [Car.new.is_a?(cv_m), 3.kind_of?(cv_m)]
      assert_equal [false, true, true, false], [Car.new.instance_of?(Vehicle), Car.new.instance_of?(Car), 3.instance_of?(Integer), cv_e.instance_of?(IndexError)]
      assert_equal [true, false, true], cv_kinds.map { |c| dynamic_ident(c) === cv_e }
      assert_equal true, cv_e.is_a?(dynamic_ident(IndexError))
      cv_vals = [1, "a", nil, :x, 2.0, [1], { a: 1 }] #: Array[untyped]
      got = [Integer, String, NilClass, Symbol, Float, Array, Hash].map do |c|
        "#{c}: #{cv_vals.map { |v| c === v }.inspect}"
      end
      assert_equal ["Integer: [true, false, false, false, false, false, false]",
                    "String: [false, true, false, false, false, false, false]",
                    "NilClass: [false, false, true, false, false, false, false]",
                    "Symbol: [false, false, false, true, false, false, false]",
                    "Float: [false, false, false, false, true, false, false]",
                    "Array: [false, false, false, false, false, true, false]",
                    "Hash: [false, false, false, false, false, false, true]"], got
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_method_missing.rb.
  class Config
    #: () -> void
    def initialize
      @values = { "color" => "blue", "size" => "L", "empty" => "" } #: Hash[String, String]
    end

    #: () -> String
    def to_s = "config"

    #: () -> String
    def color_twice = color + color

    #: (Symbol, *untyped) -> String
    def method_missing(name, *args)
      value = @values[name.to_s]
      return value if value

      "(no #{name}: #{args.inspect})"
    end

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = @values.key?(name.to_s)

    private

    #: () -> String
    def secret = "s3cret"
  end

  class StrictConfig < Config
    #: () -> String
    def size = "overridden"
  end

  class Recorder
    #: () -> void
    def initialize
      @calls = [] #: Array[String]
    end

    #: () -> Array[String]
    def calls = @calls

    #: (Symbol, *untyped) -> Integer
    def method_missing(name, *args)
      @calls << "#{name}(#{args.map { |a| a.inspect }.join(", ")})"
      @calls.size
    end
  end

  class SelfCaller
    #: () -> String
    def hi = "hi"

    #: () -> String
    def call_hi = self.send(:hi)

    #: () -> bool
    def can_hi = respond_to?(:hi)

    #: () -> bool
    def can_bye = self.respond_to?(:bye)

    #: (String) -> String
    def call_named(n) = self.send(n)
  end

  class Base
    #: () -> String
    def name = "base"

    #: () -> bool
    def can_extra? = respond_to?(:extra)
  end

  class Derived < Base
    #: () -> String
    def extra = "extra"
  end

  class Fallback
    #: (Symbol, *untyped) -> String
    def method_missing(name, *args) = "fallback #{name} #{args.size}"

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = name.to_s.end_with?("_x")

    #: () -> String
    def real = "real"
  end

  class FallbackChild < Fallback
    #: () -> String
    def own = "own #{zzz_x}"
  end

  module Chatty
    #: () -> String
    def chat = "chat #{unknown_thing}"
  end

  class Talker
    include Chatty

    #: (Symbol, *untyped) -> String
    def method_missing(name, *args) = "talker #{name}"
  end

  class Shy
    #: () -> String
    def hi = "hi"

    private

    #: (Symbol, *untyped) -> String
    def method_missing(name, *args) = "shy #{name} #{args.inspect}"

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = name == :maybe
  end

  class Proxy
    #: (untyped) -> void
    def initialize(target)
      @target = target #: untyped
    end

    #: (Symbol, *untyped) -> untyped
    def method_missing(name, *args) = @target.send(name)

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = @target.respond_to?(name)
  end

  class DynamicMethodMissingTest < Minitest::Test
    def test_unknown_methods_go_to_method_missing_typed_by
      c = Config.new
      assert_equal "blue", c.color
      assert_equal "L", c.size
      assert_equal "", c.empty
      assert_equal "(no weight: [])", c.weight
      assert_equal "(no weight: [1, \"two\", nil])", (c.weight(1, "two", nil))
      assert_equal "BLUE", c.color.upcase
      assert_equal "blueblue", c.color_twice
      assert_equal "config", c.to_s
      assert_equal "(no missing?: [])", c.missing?
      assert_equal "(no set!: [])", c.set!
      s = StrictConfig.new
      assert_equal "overridden", s.size
      assert_equal "blue", s.color
      assert_equal "(no nothing: [:x])", s.nothing(:x)

      # method_missing returning a counter
      r = Recorder.new
      assert_equal 1, r.first
      assert_equal 2, r.second(1)
      assert_equal 3, (r.third("a", [2], nil, 1.5, :sym))
      assert_equal ["first()", "second(1)", "third(\"a\", [2], nil, 1.5, :sym)"], r.calls
      total = r.one + r.two
      assert_equal 9, total

      # respond_to? folds to a constant or asks respond_to_missing?
      assert_equal true, c.respond_to?(:color)
      assert_equal false, c.respond_to?(:weight)
      assert_equal true, c.respond_to?(:to_s)
      assert_equal true, c.respond_to?(:color_twice)
      assert_equal true, c.respond_to?("size")
      assert_equal false, c.respond_to?("nope")
      assert_equal true, c.respond_to?(:empty)
      assert_equal false, c.respond_to?(:secret)
      assert_equal false, c.respond_to?(:initialize)
      assert_equal false, c.respond_to?(:respond_to_missing?)
      assert_equal true, s.respond_to?(:size)
      assert_equal false, s.respond_to?(:weight)
      assert_equal true, s.respond_to?(:inspect)
      assert_equal false, r.respond_to?(:anything)
      assert_equal true, r.respond_to?(:calls)
      p1 = Plain.new
      assert_equal true, p1.respond_to?(:hello)
      assert_equal true, p1.respond_to?(:twice)
      assert_equal false, p1.respond_to?(:bye)
      assert_equal false, p1.respond_to?(:hidden)
      assert_equal true, p1.respond_to?(:nil?)
      assert_equal true, 5.respond_to?(:even?)
      assert_equal false, 5.respond_to?(:upcase)
      assert_equal true, "s".respond_to?(:upcase)
      assert_equal false, "s".respond_to?(:even?)
      assert_equal true, [1].respond_to?(:each)
      assert_equal false, [1].respond_to?(:nope)
      assert_equal true, 1.5.respond_to?(:floor)
      assert_equal false, :sym.respond_to?(:to_proc_nope)

      # send and public_send with literal names are ordinary calls
      assert_equal "hi", p1.send(:hello)
      assert_equal "hi", p1.public_send(:hello)
      assert_equal 42, (p1.send(:twice, 21))
      assert_equal -8, (p1.public_send(:twice, -4))
      assert_equal "hi", p1.__send__(:hello)
      assert_equal "hidden", p1.send(:hidden)
      assert_equal "s3cret", c.send(:secret)
      assert_equal "blue", c.send(:color)
      assert_equal "L", c.public_send(:size)
      assert_equal "(no weight: [3])", (c.send(:weight, 3))
      assert_equal "hi", p1.send("hello")
      assert_equal 10, (p1.send("twice", 5))
      assert_equal 8, (5.send(:+, 3))
      assert_equal "ABC", "abc".send(:upcase)
      assert_equal [1, 2, 3], ([3, 1, 2].send(:sort))
      assert_equal -10, (10.public_send(:-, 20))
    end

    def test_send_and_respond_to_on_self
      sc = SelfCaller.new
      assert_equal "hi", sc.call_hi
      assert_equal true, sc.can_hi
      assert_equal false, sc.can_bye
      assert_equal "hi", sc.call_named("hi")
    end

    def test_respond_to_on_a_base_typed_value_holding
      assert_equal "base true false false", dynamic_probe(Base.new)
      assert_equal "base true true false", dynamic_probe(Derived.new)
    end

    def test_inherited_method_missing_and_respond_to_missing
      fc = FallbackChild.new
      assert_equal "own fallback zzz_x 0", fc.own
      assert_equal "fallback foo_x 0", fc.foo_x
      assert_equal "fallback bar 2", (fc.bar(1, 2))
      assert_equal "real", fc.real
      assert_equal true, fc.respond_to?(:foo_x)
      assert_equal false, fc.respond_to?(:foo)
      assert_equal true, fc.respond_to?(:own)
      assert_equal true, fc.respond_to?(:real)
      fu = dynamic_ident(fc)
      assert_equal "fallback anything 1", fu.anything(1)
      assert_equal "own fallback zzz_x 0", fu.own
      assert_equal true, fu.respond_to?(:q_x)
      assert_equal false, fu.respond_to?(:q)
      assert_equal "fallback dyn_x 1", (fc.send(:dyn_x, 3))
      assert_equal "fallback dyn_y 0", fc.public_send(:dyn_y)
      assert_equal "fallback direct 1", (fc.method_missing(:direct, 1))
    end

    def test_a_module_method_calling_an_unknown_name_reaches
      assert_equal "chat talker unknown_thing", Talker.new.chat
    end

    def test_a_private_method_missing_still_handles_typed_and
      shy = Shy.new
      assert_equal "shy whatever []", shy.whatever
      assert_equal "shy other [1]", shy.other(1)
      assert_equal true, shy.respond_to?(:maybe)
      assert_equal false, shy.respond_to?(:nope)
      assert_equal true, shy.respond_to?(:hi)
      shu = dynamic_ident(shy)
      assert_equal "shy whatever []", shu.whatever
      assert_equal "shy other [2]", shu.other(2)
      assert_equal true, shu.respond_to?(:maybe)
      assert_equal false, shu.respond_to?(:nope)
      assert_equal "hi", shu.hi
      assert_equal "shy dyn [3]", (shu.send(["dyn"].first(1)[0] || "", 3))
      assert_equal false, shy.respond_to?(:method_missing)
      assert_equal false, shy.respond_to?(:respond_to_missing?)
    end

    def test_method_missing_forwarding_to_an_untyped_target
      px = Proxy.new("héllo")
      assert_equal "HÉLLO", px.upcase
      assert_equal 5, px.size
      assert_equal "olléh", px.reverse
      assert_equal true, px.respond_to?(:upcase)
      assert_equal false, px.respond_to?(:fly)
      e = assert_raises(NoMethodError) { px.fly }
      assert_equal "undefined method 'fly' for an instance of String", e.message
    end

    def test_respond_to_on_self_in_a_base_class
      assert_equal false, Base.new.can_extra?
      assert_equal true, Derived.new.can_extra?
    end
  end

  # case on a T? with `when nil` narrows the else branch to T
  class MidAnimal
    #: () -> String
    def name = "animal"
  end

  class MidDog < MidAnimal
    #: () -> String
    def bark = "woof"
  end

  # user methods named like generated dispatchers (plus/neg) must not collide
  class Vec
    #: (Integer) -> Integer
    def plus(n) = n + 100

    #: () -> Integer
    def neg = -1
  end

  # an untyped receiver passes Array[Integer]/Array[untyped] across generic params
  class MidFoo
    #: (Array[untyped]) -> Integer
    def count_all(a) = a.size

    #: (Hash[Symbol, untyped]) -> Integer
    def opts(h) = h.size

    #: (Array[Integer]) -> Integer
    def total(a) = a.reduce(0) { |s, x| s + x }
  end

  # is_a?/kind_of? on self inside a module method
  module MidNamed
    #: () -> String
    def tag = is_a?(MidCar) ? "car-named" : "named"

    #: () -> bool
    def vehicle? = kind_of?(MidVehicle)
  end

  class MidVehicle
  end

  class MidCar < MidVehicle
    include MidNamed
  end

  class Boat
    include MidNamed
  end

  # method_missing/respond_to_missing? falling through to super
  class Picky
    #: (Symbol, *untyped) -> String
    def method_missing(name, *args)
      return "ok #{name}" if name.to_s.start_with?("get_")

      super
    end

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("get_") || super
  end

  # nil inside a rest arg reaches method_missing and a splat method intact
  class MidGhost
    #: (Symbol, *untyped) -> untyped
    def method_missing(name, *args) = "ghost #{name}(#{args.inspect})"
  end

  class LogStub
    #: (*untyped) -> String
    def log(*parts) = parts.inspect
  end

  # calling a private method on an untyped value raises MRI's message
  class Vault
    #: () -> String
    def open = "open"

    private

    #: () -> String
    def secret = "shh"
  end

  # respond_to? on self inside a module method depends on the includer
  module NamedDrive
    #: () -> bool
    def can_drive? = respond_to?(:drive)

    #: () -> String
    def maybe_drive = respond_to?(:drive) ? "can drive" : "cannot"
  end

  class CarDrive
    include NamedDrive

    #: () -> String
    def drive = "vroom"
  end

  class BoatDrive
    include NamedDrive
  end

  # &. before an iterator call skips the block on nil
  class Sides
    #: () { (Integer) -> void } -> void
    def each_side
      yield 1
      yield 2
    end
  end

  # unlike public_send, send may call private methods on typed and untyped receivers
  class VaultSend
    #: () -> String
    def open = "open"

    private

    #: () -> String
    def secret = "shh"
  end

  # a typed String param reached through an untyped receiver raises TypeError
  class MidGreeter
    #: (String) -> String
    def hi(other) = "hi " + other
  end

  # != dispatches through a user == and a computed send of :!=
  class VecNe
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (untyped) -> bool
    def ==(o) = o.is_a?(VecNe) && x == o.x
  end

  class DynamicMidTest < Minitest::Test
    def test_typed_case_on_a_primitive_still_checks_other_classes
      assert_equal "int 5", typed_int(4)
      assert_equal "string 3", typed_str("abc")
    end

    def test_case_on_nilable_with_when_nil_narrows_else
      assert_equal "dog woof", describe_animal(MidDog.new)
      assert_equal "none", describe_animal(nil)
      assert_equal "other animal", describe_animal(MidAnimal.new)
      assert_equal "text 3", text("abc")
      assert_equal "nil", text(nil)
    end

    def test_user_methods_named_like_generated_dispatchers
      assert_equal 3, ident_dyn(1) + 2
      assert_equal 101, ident_dyn(Vec.new).plus(1)
      assert_equal(-1, Vec.new.send(["neg"].first(1)[0] || ""))
    end

    def test_untyped_receiver_passes_arrays_across_generic_params
      foo_o = ident_var(MidFoo.new)
      var_nums = [1, 2] #: Array[Integer]
      var_mixed = [1, 2] #: Array[untyped]
      assert_equal 3, foo_o.total(var_nums)
      assert_equal 2, foo_o.count_all(var_mixed)
      assert_equal 2, foo_o.count_all(var_nums)
      assert_equal 1, foo_o.opts({ k: 1 })
      assert_equal 3, foo_o.total(var_mixed)
      assert_equal 0, foo_o.total([])
    end

    def test_is_a_array_narrowing_mutates_the_callers_array
      narrowed_a = [1, 2] #: Array[Integer]
      push_if_array(narrowed_a)
      push_case(narrowed_a)
      assert_equal [1, 2, 99, 7], narrowed_a
    end

    def test_is_a_on_self_inside_a_module_method
      assert_equal "car-named", MidCar.new.tag
      assert_equal "named", Boat.new.tag
      assert_equal true, MidCar.new.vehicle?
      assert_equal false, Boat.new.vehicle?
    end

    def test_method_missing_falling_through_to_super
      pk = Picky.new
      assert_equal "ok get_x", pk.get_x
      assert_equal true, pk.respond_to?(:get_y)
      assert_equal false, pk.respond_to?(:other)
      e = assert_raises(NoMethodError) { pk.other }
      assert_equal "undefined method 'other' for an instance of DynamicTests::Picky", e.message
    end

    def test_nil_inside_a_rest_arg_reaches_method_missing
      assert_equal "ghost fly([1, nil])", ident_rest(MidGhost.new).fly(1, nil)
      assert_equal "[nil, 2]", ident_rest(LogStub.new).log(nil, 2)
    end

    def test_nil_elements_survive_is_a_and_case_narrowing
      assert_equal "[false, true, false]", nils([1, nil, 3])
      assert_equal "1", nils_case(["a", nil])
      assert_equal true, ident_opt([1, nil])[1].nil?
    end

    def test_private_method_on_an_untyped_value_raises_mri_message
      vault_v = ident_priv(Vault.new)
      assert_equal "open", vault_v.open
      e = assert_raises(NoMethodError) { vault_v.secret }
      assert_equal "private method 'secret' called for an instance of DynamicTests::Vault", e.message
      e = assert_raises(NoMethodError) { vault_v.public_send(["secret"].first(1)[0] || "") }
      assert_equal "private method 'secret' called for an instance of DynamicTests::Vault", e.message
    end

    def test_respond_to_with_include_all_sees_private_methods
      plain_p = Plain.new
      assert_equal true, plain_p.respond_to?(:hidden, true)
      assert_equal true, plain_p.respond_to?(:hello, true)
      assert_equal false, plain_p.respond_to?(:nope, true)
      assert_equal "hi", plain_p.hello
      assert_equal true, ident_resp(plain_p).respond_to?(:hidden, true)
      assert_equal false, ident_resp(plain_p).respond_to?(:hidden)
    end

    def test_respond_to_on_self_in_a_module_depends_on_the_includer
      assert_equal true, CarDrive.new.can_drive?
      assert_equal false, BoatDrive.new.can_drive?
      assert_equal "can drive", CarDrive.new.maybe_drive
      assert_equal "cannot", BoatDrive.new.maybe_drive
    end

    def test_safe_navigation_before_an_iterator_skips_the_block_on_nil
      seen = [] #: Array[String]
      sides_h = { "a" => [1, 2] } #: Hash[String, Array[Integer]]
      sides_h["a"]&.each { |x| seen << x.to_s }
      sides_h["b"]&.each { |x| seen << x.to_s }
      sides = nil #: Sides?
      sides&.each_side { |s| seen << "side #{s}" }
      sides = Sides.new
      sides&.each_side { |s| seen << "side #{s}" }
      assert_equal ["1", "2", "side 1", "side 2"], seen
    end

    def test_send_may_call_private_methods_unlike_public_send
      send_v = VaultSend.new
      assert_equal ["open", "shh"], ["open", "secret"].map { |meth| send_v.send(meth) }
      assert_equal "shh", ident_send(send_v).send(:secret)
    end

    def test_typed_string_param_through_untyped_receiver_raises_type_error
      greeter_g = ident_type(MidGreeter.new)
      msgs = [5, 1.5, :sym, [1]].map do |bad|
        assert_raises(TypeError) { greeter_g.hi(bad) }.message
      end
      assert_equal ["no implicit conversion of Integer into String", "no implicit conversion of Float into String",
                    "no implicit conversion of Symbol into String", "no implicit conversion of Array into String"], msgs
    end

    def test_not_equal_dispatches_through_a_user_equals_and_computed_send
      ne_m = [:!=, :==][0] #: Symbol?
      ne_a = [1, 2] #: Array[Integer]
      ne_v = VecNe.new(1) #: untyped
      assert_equal false, ne_a.send(ne_m || :x, [1, 2])
      assert_equal false, ne_v.send(ne_m || :x, VecNe.new(1))
      assert_equal false, ne_v != VecNe.new(1)
      ne_w = VecNe.new(1)
      assert_equal false, ne_w != ne_v
      ne_pair = [1, "a"] #: [Integer, String]
      assert_equal false, ne_pair != [1, "a"]
      ne_o = nil #: Integer?
      ne_o = 3 if ne_a.size > 1
      assert_equal false, ne_o != 3
      assert_equal true, ne_o != nil
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_narrowing.rb.
  class NShape
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end
  end

  class NCircle < NShape
    attr_reader :radius #: Integer

    #: (Integer) -> void
    def initialize(radius)
      super("circle")
      @radius = radius
    end

    #: () -> Integer
    def diameter = radius * 2
  end

  class NSquare < NShape
    attr_reader :side #: Integer

    #: (Integer) -> void
    def initialize(side)
      super("square")
      @side = side
    end
  end

  class Node
    attr_reader :value #: Integer
    attr_reader :next_node #: Node?
    attr_reader :shape #: NShape

    #: (Integer, Node?, NShape) -> void
    def initialize(value, next_node, shape)
      @value = value
      @next_node = next_node
      @shape = shape
    end

    #: () -> String
    def tail_value
      if next_node
        "next=#{next_node.value}"
      else
        "last"
      end
    end

    #: () -> String
    def guarded
      return "no next" unless next_node

      "guarded=#{next_node.value + 100}"
    end

    #: () -> String
    def shape_info
      if shape.is_a?(NCircle)
        "circle d=#{shape.diameter}"
      elsif shape.is_a?(NSquare)
        "square s=#{shape.side}"
      else
        "shape #{shape.name}"
      end
    end
  end

  class DynamicNarrowingTest < Minitest::Test
    def test_if_unless_and_guards_on_nilable
      got = [dynamic_word(3), dynamic_word(0), dynamic_word(1)].map do |w|
        [len_if(w), len_unless(w), len_nil_guard(w), len_bang_guard(w)]
      end
      assert_equal [[3, 3, 1003, 2003], [-1, -1, -2, -3], [1, 1, 1001, 2001]], got
      assert_equal 0, len_if("")
      assert_equal 0, len_unless("")
      assert_equal 1000, len_nil_guard("")
      assert_equal 2000, len_bang_guard("")
      assert_equal 3, len_raise_guard("abc")
      e = assert_raises(ArgumentError) { len_raise_guard(nil) }
      assert_equal "need a string", e.message
    end

    def test_and_and_elsif_chains
      assert_equal "xy", both("x", "y")
      assert_equal "only a=X", both("x", nil)
      assert_equal "only b=Y", both(nil, "y")
      assert_equal "neither", both(nil, nil)
      assert_equal "", both("", "")
      assert_equal "long abcd", long_word("abcd")
      assert_equal "short or nil", long_word("abc")
      assert_equal "short or nil", long_word(nil)
      assert_equal "long 日本語です", long_word("日本語です")
    end

    def test_is_a_and_kind_of_narrow_struct_classes
      shapes = [NCircle.new(2), NSquare.new(3), NShape.new("blob")] #: Array[NShape]
      assert_equal ["circle: 12", "square: 9", "blob: ?"], shapes.map { |s| narrow_area_of(s) }
      assert_equal [4, -1, -1], shapes.map { |s| diameter_guard(s) }
      assert_equal "opt circle 5", opt_shape(NCircle.new(5))
      assert_equal "opt square", opt_shape(NSquare.new(1))
      assert_equal "opt nil", opt_shape(nil)
    end

    def test_attribute_reads_on_self_narrow
      list = Node.new(1, Node.new(2, nil, NSquare.new(4)), NCircle.new(7))
      assert_equal "next=2", list.tail_value
      assert_equal "\"last\"", list.next_node&.tail_value.inspect
      assert_equal "guarded=102", list.guarded
      assert_equal "\"no next\"", list.next_node&.guarded.inspect
      assert_equal "circle d=14", list.shape_info
      assert_equal "\"square s=4\"", list.next_node&.shape_info.inspect
      assert_equal "shape x", Node.new(0, nil, NShape.new("x")).shape_info
    end

    def test_is_a_narrows_untyped
      vals = ["héllo", 21, [1, 2, 3], { "k" => 1 }, 2.5, nil, :sym, true] #: Array[untyped]
      assert_equal ["string of 5: HÉLLO", "integer doubled 42", "array of 3", "hash of 1", "float 5.0", "nil", "other", "other"],
                   vals.map { |v| untyped_narrow(v) }
    end

    def test_while_narrows_the_loop_variable
      list = Node.new(1, Node.new(2, nil, NSquare.new(4)), NCircle.new(7))
      cur = list #: Node?
      total = 0
      while cur
        total += cur.value
        cur = cur.next_node
      end
      assert_equal 3, total
      assert_equal "nil", cur.inspect
    end

    def test_reassigning_drops_the_narrowing
      got = [] #: Array[String]
      s = dynamic_word(2)
      if s
        got << s.size.to_s
        s = dynamic_word(0)
        got << s.inspect
        s = dynamic_word(4)
        got << s.inspect
      end
      t = dynamic_word(1)
      if t
        t = nil
        got << t.inspect
      end
      assert_equal ["2", "nil", "\"wwww\"", "nil"], got
    end

    def test_or_assign_makes_the_value_usable
      m = dynamic_word(0)
      m ||= "fallback"
      assert_equal 8, m.size
      assert_equal "fallback", m
    end

    def test_ternary_and_modifier_forms
      got = [] #: Array[String]
      [dynamic_word(2), nil].each do |w|
        got << (w ? w.size : 0).to_s
        got << w.size.to_s if w
        got << "none" unless w
      end
      assert_equal ["2", "2", "0", "none"], got
    end

    def test_narrowing_holds_inside_a_block_in_the_branch
      got = [] #: Array[Integer]
      x = dynamic_word(2)
      if x
        [1, 2].each { |i| got << x.size + i }
      end
      assert_equal [3, 4], got
    end

    def test_next_unless_in_a_loop
      got = [] #: Array[String]
      [dynamic_word(1), nil, dynamic_word(3)].each do |w|
        next unless w

        got << w.upcase
      end
      assert_equal ["W", "WWW"], got
    end

    def test_guards_inside_blocks_and_and_chains
      assert_equal 4, total_len([dynamic_word(1), nil, dynamic_word(3)])
      assert_equal 0, total_len([])
      assert_equal 2, first_len([nil, dynamic_word(2)])
      assert_equal(-1, first_len([nil]))
      assert_equal 3, both_or("a", "bc")
      assert_equal(-1, both_or(nil, "x"))
      assert_equal(-1, both_or("x", nil))
      assert_equal(-1, both_or(nil, nil))
      assert_equal "long w-word 4", dynamic_triple(dynamic_word(4))
      assert_equal "no", dynamic_triple(dynamic_word(2))
      assert_equal "no", dynamic_triple(nil)
      assert_equal "no", dynamic_triple("abc")
      y = dynamic_word(3)
      assert_equal 3, y ? strict_len(y) : 0
    end

    def test_is_a_in_while_and_reassigning_an_untyped_local
      assert_equal "3:1", unwrap([[[1]]])
      assert_equal "0:5", unwrap(5)
      assert_equal "2:nil", unwrap([[nil]])
      assert_equal "1:nil", unwrap([])
      assert_equal "was a string, now 6 true", retype("abc")
      assert_equal "not a string", retype(1)
    end

    def test_block_parameters_narrow_like_locals
      got = [] #: Array[String]
      opt_words = [dynamic_word(1), nil, dynamic_word(3)] #: Array[String?]
      opt_words.each { |ow| got << strict_len(ow).to_s if ow }
      assert_equal [1, 0, 3], opt_words.map { |ow| ow ? strict_len(ow) : 0 }
      pairs = [["a", dynamic_word(2)], ["b", nil]] #: Array[[String, String?]]
      pairs.each do |k, pv|
        next unless pv

        got << "#{k}=#{strict_len(pv)}"
      end
      assert_equal ["1", "3", "a=2"], got
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_nil.rb.
  SHOUTS = [] #: Array[String]

  class Box
    attr_reader :label #: String
    attr_reader :inner #: Box?

    #: (String, Box?) -> void
    def initialize(label, inner)
      @label = label
      @inner = inner
    end

    #: () -> String
    def to_s = "Box(#{label})"

    #: () -> Integer?
    def size_or_nil = label.empty? ? nil : label.size

    #: () -> String?
    def inner_label = inner&.label

    #: () -> Integer
    def depth = (inner&.depth || -1) + 1

    #: () -> void
    def shout
      SHOUTS << "#{label.upcase}!"
    end
  end

  class Lazy
    # @rbs @cache: String?
    # @rbs @count: Integer

    #: () -> void
    def initialize
      @cache = nil
      @count = 0
    end

    #: () -> String
    def value
      @cache ||= compute
    end

    #: () -> String
    def compute
      @count += 1
      "computed #{@count}"
    end

    #: () -> Integer?
    def cache_size = @cache&.size

    #: () -> String
    def peek = @cache || "empty"

    #: () -> void
    def reset
      @cache = nil
    end
  end

  class DynamicNilTest < Minitest::Test
    def test_methods_on_nil_and_on_a_present_nilable
      none = dynamic_maybe(-1)
      some = dynamic_maybe(3)
      assert_equal true, none.nil?
      assert_equal false, some.nil?
      assert_equal "nil", none.inspect
      assert_equal "3", some.inspect
      assert_equal "", none.to_s
      assert_equal "3", some.to_s
      assert_equal true, none == nil
      assert_equal false, some == nil
      assert_equal false, none != nil
      assert_equal false, some != 3
      assert_equal true, some == 3
      assert_equal false, none == 3
      assert_equal true, some == 3.0
      assert_equal true, !none
      assert_equal false, !some
      assert_equal "[] [3]", "[#{none}] [#{some}]"
      assert_equal "nothing", describe_opt(none)
      assert_equal "got 3", describe_opt(some)
      assert_equal "got 0", describe_opt(0)
      assert_equal "nothing", describe_opt(dynamic_maybe(0))
      assert_equal "nil", nil.inspect
      assert_equal "", nil.to_s
      assert_equal true, nil.nil?
      assert_equal true, nil == nil
      assert_equal "|", "#{nil}|"
    end

    def test_safe_navigation_on_nil_and_present_values
      SHOUTS.clear
      b = Box.new("outer", Box.new("mid", Box.new("", nil)))
      assert_equal "mid", b.inner&.label
      assert_equal "", b.inner&.inner&.label
      assert_nil b.inner&.inner&.inner&.label
      assert_nil b.inner&.inner&.inner&.inner&.label
      assert_equal 3, b.inner&.size_or_nil
      assert_nil b.inner&.inner&.size_or_nil
      b.inner&.shout
      b.inner&.inner&.inner&.shout
      assert_equal "Box(mid)", b.inner&.to_s
      nobox = nil #: Box?
      assert_nil nobox&.label
      assert_nil nobox&.inner&.label
      assert_nil nobox&.size_or_nil
      nobox&.shout
      assert_equal ["MID!"], SHOUTS
      str = nil #: String?
      assert_nil str&.upcase
      assert_nil str&.size
      assert_nil str&.empty?
      str = "héllo"
      assert_equal "HÉLLO", str&.upcase
      assert_equal 5, str&.size
      assert_equal false, str&.empty?
      assert_equal "mid", b.inner_label
      assert_nil b.inner&.inner&.inner_label
      assert_equal 2, b.depth
      assert_equal 0, Box.new("x", nil).depth
      assert_equal "has inner", b.inner&.label ? "has inner" : "no"
      assert_equal "no inner-inner-inner", b.inner&.inner&.inner ? "yes" : "no inner-inner-inner"
      assert_equal "  héllo  ", str&.center(9)
      assert_nil nobox&.label&.center(9)
    end

    def test_equality_between_nilable_values_compares_the_values
      assert_equal true, dynamic_maybe(1) == dynamic_maybe(1)
      assert_equal true, dynamic_maybe(-1) == dynamic_maybe(-2)
      assert_equal false, dynamic_maybe(1) == dynamic_maybe(-1)
      assert_equal true, dynamic_maybe(2) != dynamic_maybe(3)
      assert_equal false, dynamic_maybe(-1) != nil
      assert_equal true, nil == dynamic_maybe(-1)
      assert_equal true, dynamic_maybe(7) == 7
      assert_equal false, dynamic_maybe(7) != 7.0
    end

    def test_or_picks_the_first_non_nil
      none = dynamic_maybe(-1)
      some = dynamic_maybe(3)
      b = Box.new("outer", Box.new("mid", Box.new("", nil)))
      nobox = nil #: Box?
      h = { "a" => 1, "z" => 0 } #: Hash[String, Integer]
      assert_equal 1, h["a"] || -1
      assert_equal(-1, h["b"] || -1)
      assert_equal 0, h["z"] || -1
      assert_equal 3, none || some || 99
      assert_equal 99, none || dynamic_maybe(-5) || 99
      str = "héllo" #: String?
      assert_equal "héllo", str || "default"
      empty = nil #: String?
      assert_equal "", empty || ""
      assert_equal 1, (empty || "d").size
      assert_equal "none", b.inner&.inner&.inner&.label || "none"
      assert_equal "none", nobox&.label || "none"
    end

    def test_or_assign_assigns_only_when_nil
      cache = nil #: Integer?
      cache ||= 10
      assert_equal 10, cache
      cache ||= 20
      assert_equal 10, cache
      label = nil #: String?
      label ||= "x" * 3
      label ||= "never"
      assert_equal "xxx", label
      zero = 0 #: Integer?
      zero ||= 5
      assert_equal 0, zero
    end

    def test_and_yields_the_right_side_or_nil
      none = dynamic_maybe(-1)
      some = dynamic_maybe(3)
      v = some && some * 2
      assert_equal 6, v
      w = none && none * 2
      assert_nil w
      flag = some && some > 2
      assert_equal true, flag
    end

    def test_nil_in_collections
      h = { "a" => 1, "z" => 0 } #: Hash[String, Integer]
      nums = [3, 1, 2] #: Array[Integer]
      found = nums.find { |x| x > 2 }
      missing = nums.find { |x| x > 5 }
      assert_equal 3, found
      assert_nil missing
      assert_equal 3, (found || 0) + (missing || 0)
      assert_equal 1, h["a"]
      assert_nil h["missing"]
      assert_equal 7, h.fetch("missing", 7)
      none_list = [] #: Array[Integer]
      assert_equal [], none_list.first(1)
      assert_nil none_list.min
      assert_nil none_list.max
      assert_nil none_list.pop
      assert_nil(nums.detect { |x| x == 9 })
      maybes = [dynamic_maybe(1), dynamic_maybe(-1), dynamic_maybe(2)] #: Array[Integer?]
      assert_equal 3, maybes.size
      assert_equal 1, maybes.select { |m| m.nil? }.size
      assert_equal [false, true, false], maybes.map { |m| m.nil? }
      assert_equal ["1", "nil", "2"], maybes.map { |m| m.inspect }
      assert_equal ["1", "", "2"], maybes.map { |m| m.to_s }
      assert_equal [10, 0, 20], maybes.map { |m| (m || 0) * 10 }
    end

    def test_ternary_on_nilable
      none = dynamic_maybe(-1)
      some = dynamic_maybe(3)
      assert_equal 4, some ? some + 1 : 0
      assert_equal 0, none ? none + 1 : 0
    end

    def test_safe_navigation_on_method_results_with_arguments_into_or
      fruit = { "a" => "apple", "n" => "" } #: Hash[String, String]
      assert_equal "APPLE", fruit["a"]&.upcase
      assert_nil fruit["zz"]&.upcase
      assert_equal true, fruit["n"]&.empty?
      assert_equal "5", fruit["a"]&.size&.to_s
      assert_nil fruit["zz"]&.size&.to_s
      assert_equal 1, (fruit["zz"]&.size || 0) + 1
      assert_equal "  apple  ", fruit["a"]&.center(9)
      assert_nil fruit["zz"]&.center(9)
      lists = { "a" => [1, 2, 3] } #: Hash[String, Array[Integer]]
      assert_equal [2, 4, 6], lists["a"]&.map { |x| x * 2 }
      assert_nil(lists["b"]&.map { |x| x * 2 })
      assert_equal 2, lists["a"]&.select { |x| x.odd? }&.size
      got = [] #: Array[String]
      w2 = nil #: Integer?
      [1, nil, 3].each do |x|
        w2 = x
        got << w2&.+(1).inspect
      end
      assert_equal ["2", "nil", "4"], got
    end

    def test_nilable_struct_equality_uses_the_class_equals
      assert_equal true, pt(1) == pt(1)
      assert_equal false, pt(1) == pt(2)
      assert_equal true, pt(0) == pt(-1)
      assert_equal false, pt(1) == nil
      assert_equal true, pt(0) == nil
      assert_equal 3, dynamic_ident(pt(3)).x
      assert_equal "nil", dynamic_ident(pt(0)).inspect
      assert_equal 4, pt(4)&.x || 0
      assert_nil pt(0)&.x
      op = pt(0)
      assert_equal(-1, op ? op.x : -1)
    end

    def test_an_if_without_else_yields_nil
      assert_equal "even", only_even(2)
      assert_nil only_even(3)
      assert_equal "odd", only_even(5) || "odd"
    end

    def test_respond_to_on_a_nilable_asks_the_value_or_nil
      none = dynamic_maybe(-1)
      present = dynamic_maybe(4)
      assert_equal true, present.respond_to?(:abs)
      assert_equal false, none.respond_to?(:abs)
      assert_equal true, none.respond_to?(:nil?)
      assert_equal true, none.respond_to?(:inspect)
    end

    def test_nilable_ivars_memoize_with_or_assign
      lz = Lazy.new
      assert_nil lz.cache_size
      assert_equal "empty", lz.peek
      assert_equal "computed 1", lz.value
      assert_equal "computed 1", lz.value
      assert_equal 10, lz.cache_size
      assert_equal "computed 1", lz.peek
      lz.reset
      assert_equal "computed 2", lz.value
      assert_equal "computed 2", lz.peek
    end

    def test_calling_a_method_on_nil_raises_no_method_error
      none = dynamic_maybe(-1)
      some = dynamic_maybe(3)
      empty = nil #: String?
      e = assert_raises(NoMethodError) { none.abs }
      assert_equal "undefined method 'abs' for nil", e.message
      assert_equal 3, some.abs
      e = assert_raises(NoMethodError) { empty.upcase }
      assert_equal "undefined method 'upcase' for nil", e.message
      e = assert_raises(NoMethodError) { empty.size }
      assert_equal "undefined method 'size' for nil", e.message
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_send.rb.
  ANNOUNCED = [] #: Array[String]

  class Greeter
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: (?String) -> String
    def greet(greeting = "hello") = "#{greeting}, #{name}"

    #: (String, ?String, ?String) -> String
    def wrap(body, left = "[", right = "]") = left + body + right

    #: (*String) -> String
    def shout(*words) = words.map(&:upcase).join(" ")

    #: (Integer, *Integer) -> Integer
    def sum(first, *rest) = rest.reduce(first) { |acc, x| acc + x }

    #: (String?) -> String
    def maybe_name(s) = s || "anon"

    #: (String) -> String
    def hi(other) = "hi " + other

    #: () -> Integer?
    def lucky = nil

    #: () -> Integer?
    def unlucky = 13

    #: () -> void
    def announce
      ANNOUNCED << "announcing #{name}"
      nil
    end

    #: () -> Greeter
    def twin = Greeter.new(name + "2")

    #: () -> Array[String]
    def letters = name.chars

    #: ([Integer, String]) -> String
    def pair_text(t) = "#{t[0]}/#{t[1]}"

    #: (singleton(Greeter)) -> String
    def class_text(k) = k.name

    #: () -> self
    def itself_again = self

    #: (bool) -> bool
    def flip(x) = !x

    #: (Float, Symbol) -> String
    def mixed(f, s) = "#{f}:#{s}"

    #: (Array[String]) -> Integer
    def count_words(words) = words.size

    private

    #: () -> String
    def secret = "shh"
  end

  class Ghost
    #: (Symbol, *untyped) -> untyped
    def method_missing(name, *args) = "ghost #{name}(#{args.map { |a| a.inspect }.join(", ")})"

    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("g")
  end

  class Robot
    #: () -> String
    def greet = "BEEP"
  end

  module Labeled
    #: () -> String
    def label = "labeled #{kind}"

    #: () -> String
    def kind = "?"
  end

  class SendVehicle
    #: () -> String
    def wheels = "4 wheels"

    #: () -> String
    def self.fleet = "fleet"
  end

  class SendCar < SendVehicle
    include Labeled

    #: () -> String
    def kind = "car"
  end

  class Counter
    attr_accessor :count #: Integer
    attr_accessor :note #: String?

    #: () -> void
    def initialize
      @count = 0
      @note = nil
    end

    #: (Integer?) -> String
    def opt(n) = n.nil? ? "none" : "n=#{n}"

    #: (bool) -> String
    def yn(b) = b ? "yes" : "no"

    #: (Float) -> Float
    def half(f) = f / 2
  end

  class DynamicSendTest < Minitest::Test
    def test_the_same_call_reaches_each_receivers_own_method
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      ghost = dynamic_ident(Ghost.new) #: untyped
      got = [g, ghost, dynamic_ident(Robot.new)].map do |thing|
        [thing.greet, thing.respond_to?(:greet), thing.respond_to?(:zap), thing.respond_to?(:gone), thing.respond_to?(:to_s)]
      end
      assert_equal [["hello, ada", true, false, false, true],
                    ["ghost greet()", true, false, true, true],
                    ["BEEP", true, false, false, true]], got
    end

    def test_optional_rest_and_nilable_parameters
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      assert_equal "hello, ada", g.greet
      assert_equal "hey, ada", g.greet("hey")
      assert_equal ", ada", g.greet("")
      assert_equal "[x]", g.wrap("x")
      assert_equal "<x]", g.wrap("x", "<")
      assert_equal "<x>", g.wrap("x", "<", ">")
      assert_equal "[]", g.wrap("")
      assert_equal "\"\"", g.shout.inspect
      assert_equal "A", g.shout("a")
      assert_equal "A B ÜNÏ", g.shout("a", "b", "ünï")
      assert_equal 1, g.sum(1)
      assert_equal 6, g.sum(1, 2, 3)
      assert_equal 0, g.sum(-5, 5)
      assert_equal "anon", g.maybe_name(nil)
      assert_equal "bob", g.maybe_name("bob")
    end

    def test_parameter_types_checked_at_the_boundary
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      assert_equal "7/seven", g.pair_text([7, "seven"])
      assert_equal "DynamicTests::Greeter", g.class_text(Greeter)
      assert_equal "ada", g.itself_again.name
      assert_equal "false", g.flip(true).inspect
      assert_equal "true", g.flip(false).inspect
      assert_equal "-0.5:sym", g.mixed(-0.5, :sym)
      assert_equal "1.0e+20:a b", g.mixed(1e20, :"a b")
      assert_equal 3, g.count_words(["a", "b", "c"])
      assert_equal 1, g.count_words(["ü"])
    end

    def test_results_cross_back_as_untyped_values
      ANNOUNCED.clear
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      assert_equal "nil", g.lucky.inspect
      assert_equal "13", g.unlucky.inspect
      assert_equal "true", g.lucky.nil?.inspect
      assert_equal "14", (g.unlucky + 1).inspect
      assert_equal "nil", g.announce.inspect
      assert_equal ["announcing ada"], ANNOUNCED
      assert_equal "ada2", g.twin.name
      assert_equal "yo, ada22", g.twin.twin.greet("yo")
      assert_equal "[\"a\", \"d\", \"a\"]", g.letters.inspect
      assert_equal 3, g.letters.size
      assert_equal "ADA", g.name.upcase
      assert_equal 3, g.name.size
      assert_equal "\"ada!\"", (g.name + "!").inspect
    end

    def test_dynamic_calls_on_untyped_primitives
      n = dynamic_ident(7) #: untyped
      s = dynamic_ident("héllo") #: untyped
      a = dynamic_ident([3, 1, 2]) #: untyped
      h = dynamic_ident({ "k" => 1 }) #: untyped
      assert_equal "8", (n + 1).inspect
      assert_equal "49", (n * n).inspect
      assert_equal "true", (n > 3).inspect
      assert_equal "false", n.even?.inspect
      assert_equal "\"7\"", n.to_s.inspect
      assert_equal "3", (n - 10).abs.inspect
      assert_equal "HÉLLO", s.upcase
      assert_equal 5, s.size
      assert_equal "olléh", s.reverse
      assert_equal "true", s.include?("é").inspect
      assert_equal "true", s.start_with?("hé").inspect
      assert_equal "\"h\"", s[0].inspect
      assert_equal 3, a.size
      assert_equal "[1, 2, 3]", a.sort.inspect
      assert_equal "[3, 1]", a.first(2).inspect
      assert_equal "true", a.include?(2).inspect
      assert_equal "2", a.last.inspect
      assert_equal "false", a.empty?.inspect
      assert_equal "1", a[1].inspect
      assert_equal "1", h["k"].inspect
      assert_equal "nil", h["zz"].inspect
      assert_equal 1, h.size
      assert_equal "true", h.key?("k").inspect
      assert_equal "false", h.key?("q").inspect
    end

    def test_send_and_public_send_with_literal_and_computed_names
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      ghost = dynamic_ident(Ghost.new) #: untyped
      assert_equal "yo, ada", g.send(:greet, "yo")
      assert_equal "ada", g.public_send(:name)
      assert_equal "(m)", g.send(:wrap, "m", "(", ")")
      assert_equal ["\"hello, ada\"", "\"\"", "\"ada\"", "nil"], ["greet", "shout", "name", "lucky"].map { |m| g.send(m).inspect }
      assert_equal ["\"hello, ada\"", "\"ada\""], [:greet, :name].map { |m| g.public_send(m).inspect }
      assert_equal "3", g.send("sum", 1, 2).inspect
      assert_equal "\"ghost glide(1)\"", ghost.send("glide", 1).inspect
      assert_equal "\"ghost anything()\"", ghost.send(:anything).inspect
      assert_equal [true, false, true, false, true, false], ["name", "fly", :greet, "secret", "to_s", "gone"].map { |m| g.respond_to?(m) }
      assert_equal [true, false], ["glow", "fly"].map { |m| ghost.respond_to?(m) }
    end

    def test_method_missing_receives_unknown_names_and_their_arguments
      ghost = dynamic_ident(Ghost.new) #: untyped
      assert_equal "ghost walk()", ghost.walk
      assert_equal "ghost fly(1, \"two\", :three, [4], 5.0)", ghost.fly(1, "two", :three, [4], 5.0)
      assert_equal "ghost greet(\"x\")", ghost.greet("x")
    end

    def test_class_objects
      klass = Object.const_get("DynamicTests::Greeter")
      assert_equal "hey, bob", klass.new("bob").greet("hey")
      assert_equal "DynamicTests::Greeter", klass.name
      assert_equal "DynamicTests::Greeter", klass.inspect
      assert_equal ["DynamicTests::Greeter", "DynamicTests::Robot"], [Greeter, Robot].map { |k| k.name }
    end

    def test_arity_errors_use_mri_messages
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      e = assert_raises(ArgumentError) { g.greet("a", "b", "c") }
      assert_equal "wrong number of arguments (given 3, expected 0..1)", e.message
      e = assert_raises(ArgumentError) { g.wrap }
      assert_equal "wrong number of arguments (given 0, expected 1..3)", e.message
      e = assert_raises(ArgumentError) { g.sum }
      assert_equal "wrong number of arguments (given 0, expected 1+)", e.message
      e = assert_raises(ArgumentError) { g.name(1) }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
      e = assert_raises(ArgumentError) { g.hi }
      assert_equal "wrong number of arguments (given 0, expected 1)", e.message
    end

    def test_argument_type_errors
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      assert_raises(TypeError) { g.hi(5) }
      assert_raises(TypeError) { g.wrap(1) }
    end

    def test_missing_methods_use_mri_messages
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      e = assert_raises(NoMethodError) { g.fly }
      assert_equal "undefined method 'fly' for an instance of DynamicTests::Greeter", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(nil).greet }
      assert_equal "undefined method 'greet' for nil", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(true).greet }
      assert_equal "undefined method 'greet' for true", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(false).greet(1) }
      assert_equal "undefined method 'greet' for false", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(5).greet }
      assert_equal "undefined method 'greet' for an instance of Integer", e.message
      e = assert_raises(NoMethodError) { dynamic_ident("str").greet }
      assert_equal "undefined method 'greet' for an instance of String", e.message
      e = assert_raises(NoMethodError) { dynamic_ident([1]).greet }
      assert_equal "undefined method 'greet' for an instance of Array", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(:sym).greet }
      assert_equal "undefined method 'greet' for an instance of Symbol", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(Robot).greet }
      assert_equal "undefined method 'greet' for class DynamicTests::Robot", e.message
      e = assert_raises(NoMethodError) { dynamic_ident(Tools).greet }
      assert_equal "undefined method 'greet' for module DynamicTests::Tools", e.message
      e = assert_raises(NoMethodError) { g.send("fly", 1) }
      assert_equal "undefined method 'fly' for an instance of DynamicTests::Greeter", e.message
      assert_raises(NoMethodError) { g.secret }
    end

    def test_a_computed_name_must_be_a_symbol_or_string
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      msgs = [] #: Array[String]
      [1, 2.5, nil, [1]].each do |bad|
        msgs << assert_raises(TypeError) { g.send(bad) }.message
        msgs << assert_raises(TypeError) { g.respond_to?(bad) }.message
      end
      assert_equal ["1 is not a symbol nor a string", "1 is not a symbol nor a string",
                    "2.5 is not a symbol nor a string", "2.5 is not a symbol nor a string",
                    "nil is not a symbol nor a string", "nil is not a symbol nor a string",
                    "[1] is not a symbol nor a string", "[1] is not a symbol nor a string"], msgs
    end

    def test_respond_to_on_untyped_nil_primitives_and_class_objects
      assert_equal false, dynamic_ident(nil).respond_to?(:greet)
      assert_equal true, dynamic_ident(nil).respond_to?(:inspect)
      assert_equal false, dynamic_ident(5).respond_to?(:greet)
      assert_equal true, dynamic_ident(5).respond_to?(:even?)
      assert_equal true, dynamic_ident(nil).respond_to?(:nil?)
      assert_equal false, dynamic_ident(Greeter).respond_to?(:greet)
      assert_equal true, dynamic_ident(Greeter).respond_to?(:name)
      assert_equal true, dynamic_ident(Greeter).respond_to?(:new)
      assert_equal true, dynamic_ident("s").respond_to?(:upcase)
      assert_equal true, dynamic_ident([]).respond_to?(:each)
      assert_equal true, dynamic_ident({}).respond_to?(:key?)
      assert_equal true, dynamic_ident(:s).respond_to?(:to_sym)
      # generic classes answer by class ID (decision 87); a tuple is an Array
      pair = [1, "a"] #: [Integer, String]
      assert_equal [true, true, false], [dynamic_ident(pair).respond_to?(:size), dynamic_ident(pair).respond_to?(:first), dynamic_ident(pair).respond_to?(:keys)]
      assert_equal [true, false], [dynamic_ident([1, 2]).respond_to?(:push), dynamic_ident([1, 2]).respond_to?(:keys)]
      assert_equal [true, false], [dynamic_ident({ a: 1 }).respond_to?(:keys), dynamic_ident({ a: 1 }).respond_to?(:push)]
      assert_equal [true, false], [dynamic_ident(1..3).respond_to?(:each), dynamic_ident(1..3).respond_to?(:push)]
    end

    def test_inherited_included_and_comparable_methods_reached_dynamically
      car = dynamic_ident(SendCar.new)
      assert_equal "labeled car", car.label
      assert_equal "4 wheels", car.wheels
      assert_equal "car", car.kind
      assert_equal true, car.respond_to?(:label)
      assert_equal true, car.respond_to?(:wheels)
      assert_equal false, car.respond_to?(:fleet)
      assert_equal true, dynamic_ident(3).between?(1, 5)
      assert_equal "b", dynamic_ident("b").clamp("a", "c")
      assert_equal 1, dynamic_ident([3, 1, 2]).min
      assert_equal 3, dynamic_ident([3, 1, 2]).max
      assert_equal 2, dynamic_ident(1.5).round
      assert_equal 3, dynamic_ident(-3).abs
      assert_equal "[\"a\", \"b\"]", dynamic_ident("a,b").split(",").inspect
    end

    def test_class_objects_through_untyped_new_and_class_methods
      kv = dynamic_ident(SendVehicle)
      assert_equal "4 wheels", kv.new.wheels
      assert_equal "fleet", kv.fleet
      assert_equal "DynamicTests::SendVehicle", kv.name
      assert_equal "fleet", dynamic_ident(SendCar).fleet
      assert_equal true, dynamic_ident(SendCar).new.is_a?(SendVehicle)
      e = assert_raises(ArgumentError) { dynamic_ident(Greeter).new }
      assert_equal "wrong number of arguments (given 0, expected 1)", e.message
    end

    def test_attribute_writers_index_assign_and_typed_params_through_dynamic_calls
      cu = dynamic_ident(Counter.new)
      cu.count = 5
      assert_equal 5, cu.count
      cu.note = "memo"
      assert_equal "\"memo\"", cu.note.inspect
      cu.note = nil
      assert_equal "nil", cu.note.inspect
      assert_equal "none", cu.opt(nil)
      assert_equal "n=3", cu.opt(3)
      assert_equal "yes", cu.yn(true)
      assert_equal "no", cu.yn(false)
      assert_equal "1.5", cu.half(3.0).to_s
      arr = dynamic_ident([1, 2, 3])
      arr[0] = 10
      assert_equal "[10, 2, 3]", arr.inspect
      hh = dynamic_ident({ "k" => 1 })
      hh["z"] = 2
      assert_equal "{\"k\" => 1, \"z\" => 2}", hh.inspect
    end

    def test_nil_true_and_false_arguments_use_mri_type_error_wording
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      msgs = [nil, true, false].map do |bad|
        assert_raises(TypeError) { g.hi(bad) }.message
      end
      assert_equal ["no implicit conversion of nil into String", "no implicit conversion of true into String",
                    "no implicit conversion of false into String"], msgs
    end

    def test_equals_on_untyped_uses_the_receivers_own_equals
      pu = dynamic_ident(Pt.new(1))
      assert_equal true, pu == Pt.new(1)
      assert_equal false, pu == Pt.new(2)
      assert_equal false, pu != Pt.new(1)
      assert_equal false, pu == 1
      assert_equal true, Pt.new(3) == dynamic_ident(Pt.new(3))
    end

    def test_send_with_literal_names_on_untyped_receivers
      g = dynamic_ident(Greeter.new("ada")) #: untyped
      assert_equal "yo, ada", g.send(:greet, "yo")
      assert_equal "[p]", g.public_send(:wrap, "p")
      assert_equal "ada", g.__send__(:name)
      assert_equal 6, dynamic_ident(5).send(:+, 1)
      assert_equal "AB", dynamic_ident("ab").public_send(:upcase)
      assert_equal "[1, 2]", dynamic_ident([2, 1]).send(:sort).inspect
      assert_equal true, g.respond_to?("greet")
      assert_equal false, g.respond_to?(:secret)
      assert_equal true, dynamic_ident("s").respond_to?("upcase")
      assert_raises(NoMethodError) { g.public_send(:secret) }
    end

    def test_respond_to_with_a_computed_name_on_a_typed_receiver
      robot = Robot.new
      names = ["greet", "fly", :greet] #: Array[untyped]
      assert_equal [true, false, true], names.map { |m| robot.respond_to?(m) }
    end

    def test_an_unknown_method_on_an_untyped_value_raises
      e = assert_raises(NoMethodError) { dynamic_ident(Robot.new).fly }
      assert_equal "undefined method 'fly' for an instance of DynamicTests::Robot", e.message
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_subclass.rb.
  class Shape
    attr_reader :tag #: String

    #: (String) -> void
    def initialize(tag)
      @tag = tag
    end

    #: () -> String
    def describe = "#{tag}: area #{area}"

    #: () -> String
    def self.kind = "shape"
  end

  class Sq < Shape
    #: () -> Integer
    def area = 4

    #: (Integer) -> Integer
    def scaled(k) = area * k

    #: () -> String
    def self.corners = "four"
  end

  class Tri < Shape
    #: () -> Float
    def area = 1.5

    #: (Integer, ?Integer) -> Float
    def scaled(k, extra = 0) = area * k.to_f + extra.to_f

    #: () -> String
    def self.corners = "three"
  end

  class Blob < Shape
  end

  class DynamicSubclassTest < Minitest::Test
    def test_a_method_only_subclasses_define_dispatches_at_run_time
      assert_equal "sq: area 4", Sq.new("sq").describe
      assert_equal "tri: area 1.5", Tri.new("tri").describe
      assert_equal "4", area_of(Sq.new("a"))
      assert_equal "1.5", area_of(Tri.new("b"))
      assert_equal "4", sent_area(Sq.new("c"))
      assert_equal "1.5", sent_area(Tri.new("d"))
    end

    def test_subclass_methods_through_a_base_typed_value
      sq = Sq.new("s") #: Shape
      tri = Tri.new("t") #: Shape
      blob = Blob.new("b") #: Shape
      assert_equal [true, true, true], [sq.respond_to?(:area), sq.respond_to?(:scaled), sq.respond_to?(:tag)]
      assert_equal "8", sq.scaled(2).inspect
      e = assert_raises(ArgumentError) { sq.scaled(2, 1) }
      assert_equal "wrong number of arguments (given 2, expected 1)", e.message
      assert_equal [true, true, true], [tri.respond_to?(:area), tri.respond_to?(:scaled), tri.respond_to?(:tag)]
      assert_equal "3.0", tri.scaled(2).inspect
      assert_equal "4.0", tri.scaled(2, 1).inspect
      assert_equal [false, false, true], [blob.respond_to?(:area), blob.respond_to?(:scaled), blob.respond_to?(:tag)]
      e = assert_raises(NoMethodError) { blob.scaled(2) }
      assert_equal "undefined method 'scaled' for an instance of DynamicTests::Blob", e.message
    end

    def test_a_base_instance_raises_no_method_error_or_name_error_for_a_bare_name
      e = assert_raises(NoMethodError) { area_of(Shape.new("plain")) }
      assert_equal "undefined method 'area' for an instance of DynamicTests::Shape", e.message
      ne = assert_raises(NameError) { Blob.new("blob").describe }
      assert_equal "NameError", ne.class.to_s
      assert_equal "undefined local variable or method 'area' for an instance of DynamicTests::Blob", ne.message
    end

    def test_class_objects_typed_singleton_base_and_module
      classes = [Sq, Tri, Blob] #: Array[singleton(Shape)]
      got = classes.map { |k| [k.new("x").tag, k.name, k.kind, k.to_s, k.inspect] }
      assert_equal [["x", "DynamicTests::Sq", "shape", "DynamicTests::Sq", "DynamicTests::Sq"], ["x", "DynamicTests::Tri", "shape", "DynamicTests::Tri", "DynamicTests::Tri"], ["x", "DynamicTests::Blob", "shape", "DynamicTests::Blob", "DynamicTests::Blob"]], got
      msq = Sq #: Module
      mtri = Tri #: Module
      mshape = Shape #: Module
      assert_equal "DynamicTests::Sq", msq.name
      assert_equal "four", msq.corners
      assert_equal "DynamicTests::Tri", mtri.name
      assert_equal "three", mtri.corners
      assert_equal "DynamicTests::Shape", mshape.name
      e = assert_raises(NoMethodError) { mshape.corners }
      assert_equal "undefined method 'corners' for class DynamicTests::Shape", e.message
    end

    def test_subclass_only_methods_through_singleton_base_and_nilable_base
      ksq = Sq #: singleton(Shape)
      kblob = Blob #: singleton(Shape)
      assert_equal [true, true], [ksq.respond_to?(:corners), ksq.respond_to?(:kind)]
      assert_equal "four", ksq.corners
      assert_equal [false, true], [kblob.respond_to?(:corners), kblob.respond_to?(:kind)]
      e = assert_raises(NoMethodError) { kblob.corners }
      assert_equal "undefined method 'corners' for class DynamicTests::Blob", e.message
      assert_equal "4", dynamic_pick(1)&.area.inspect
      assert_equal "nil", dynamic_pick(0)&.area.inspect
      assert_equal true, dynamic_pick(1).respond_to?(:area)
      assert_equal false, dynamic_pick(-1).respond_to?(:area)
      e = assert_raises(NoMethodError) { dynamic_pick(-1)&.area }
      assert_equal "undefined method 'area' for an instance of DynamicTests::Blob", e.message
    end
  end

  # Helpers for the checks that were testdata/run/dynamic_untyped.rb.
  class Temp
    attr_reader :deg #: Integer

    #: (Integer) -> void
    def initialize(deg)
      @deg = deg
    end

    #: () -> String
    def to_s = "#{deg}°"

    #: () -> String
    def inspect = "#<Temp #{deg}>"
  end

  class Holder
    attr_reader :value #: untyped

    #: (untyped) -> void
    def initialize(value)
      @value = value
    end

    #: () -> String
    def describe
      if value.is_a?(String)
        "str #{value.upcase}"
      elsif value.is_a?(Integer)
        "int #{value + 1}"
      elsif value.nil?
        "nil"
      else
        "other #{value.inspect}"
      end
    end

    #: () -> String
    def via_case
      case value
      when String then "S#{value.size}"
      when Array then "A#{value.size}"
      else "?"
      end
    end

    #: () -> String
    def ivar_dyn = "#{@value.to_s}!"
  end

  class DynamicUntypedTest < Minitest::Test
    VALS = [nil, false, true, 0, 0.0, -1, "", "0", :a, [], {}, Temp.new(-4)] #: Array[untyped]

    def test_truthiness_only_nil_and_false_are_falsy
      got = VALS.map { |v| "#{v.inspect} #{truth(v)} #{(!v).inspect} #{v ? 1 : 2}" }
      assert_equal ["nil falsy true 2", "false falsy true 2", "true truthy false 1", "0 truthy false 1",
                    "0.0 truthy false 1", "-1 truthy false 1", "\"\" truthy false 1", "\"0\" truthy false 1",
                    ":a truthy false 1", "[] truthy false 1", "{} truthy false 1", "#<Temp -4> truthy false 1"], got
      assert_equal 10, VALS.select { |v| v }.size
      assert_equal 2, VALS.reject { |v| v }.size
    end

    def test_to_s_inspect_and_interpolation_of_untyped_values
      more = [2**40, -0.5, 1e20, 0.1 + 0.2, "tab\there", "quote\"s", "uni ✓", "", :"with space", ["n", [1, [2]]], { "k" => "v", :s => 1.0 }, Temp.new(21)] #: Array[untyped]
      expected = [
        ["1099511627776", "1099511627776", "<1099511627776>"],
        ["-0.5", "-0.5", "<-0.5>"],
        ["1.0e+20", "1.0e+20", "<1.0e+20>"],
        ["0.30000000000000004", "0.30000000000000004", "<0.30000000000000004>"],
        ["\"tab\\there\"", "tab\there", "<tab\there>"],
        ["\"quote\\\"s\"", "quote\"s", "<quote\"s>"],
        ["\"uni ✓\"", "uni ✓", "<uni ✓>"],
        ["\"\"", "", "<>"],
        [":\"with space\"", "with space", "<with space>"],
        ["[\"n\", [1, [2]]]", "[\"n\", [1, [2]]]", "<[\"n\", [1, [2]]]>"],
        ["{\"k\" => \"v\", s: 1.0}", "{\"k\" => \"v\", s: 1.0}", "<{\"k\" => \"v\", s: 1.0}>"],
        ["#<Temp 21>", "21°", "<21°>"],
      ] #: Array[Array[String]]
      assert_equal expected, more.map { |v| [v.inspect, v.to_s, "<#{v}>"] }
      assert_equal "[1099511627776, -0.5, 1.0e+20, 0.30000000000000004, \"tab\\there\", \"quote\\\"s\", \"uni ✓\", \"\", :\"with space\", [\"n\", [1, [2]]], {\"k\" => \"v\", s: 1.0}, #<Temp 21>]", more.inspect
      assert_equal "[nil, false, true, 0, 0.0, -1, \"\", \"0\", :a, [], {}, #<Temp -4>]", VALS.inspect
    end

    def test_or_and_and_on_untyped
      assert_equal "\"left nil\"", (dynamic_ident(nil) || "left nil").inspect
      assert_equal "0", (dynamic_ident(false) || 0).inspect
      assert_equal "0", (dynamic_ident(0) || 1).inspect
      assert_equal "\"\"", (dynamic_ident("") || "x").inspect
      assert_equal "nil", (dynamic_ident(nil) && 1).inspect
      assert_equal "false", (dynamic_ident(false) && 1).inspect
      assert_equal "\"right\"", (dynamic_ident(0) && "right").inspect
      assert_equal "nil", (dynamic_ident([]) && dynamic_ident(nil)).inspect
      x = nil #: untyped
      x ||= 5
      assert_equal "5", x.inspect
      y = dynamic_ident(false)
      y ||= "was false"
      assert_equal "\"was false\"", y.inspect
      z = dynamic_ident(0)
      z ||= 99
      assert_equal "0", z.inspect
    end

    def test_equality_and_nil_on_untyped
      assert_equal true, dynamic_ident(1) == 1
      assert_equal true, dynamic_ident(1) == 1.0
      assert_equal true, dynamic_ident("a") == "a"
      assert_equal false, dynamic_ident(:a) == "a"
      assert_equal true, dynamic_ident(nil) == nil
      assert_equal false, dynamic_ident(false) == nil
      assert_equal true, dynamic_ident([1, 2]) == [1, 2]
      assert_equal true, dynamic_ident(1) != 2
      assert_equal true, dynamic_ident(nil).nil?
      assert_equal false, dynamic_ident(false).nil?
      assert_equal false, dynamic_ident(0).nil?
      assert_equal true, 1 == dynamic_ident(1)
      assert_equal false, "b" == dynamic_ident("a")
      assert_equal true, dynamic_ident(:a).equal?(:a)
      assert_equal true, dynamic_ident(1).equal?(1)
      assert_equal true, dynamic_ident(nil).equal?(nil)
      assert_equal false, dynamic_ident(1).equal?(2)
    end

    def test_nilable_crosses_into_untyped_as_nil_or_the_value
      assert_equal "nil", kind_of_value(dynamic_maybe(-1))
      assert_equal "other", kind_of_value(dynamic_maybe(2))
      assert_equal "nil", kind_of_value(dynamic_ident(dynamic_maybe(-3)))
      assert_equal "5", dynamic_ident(dynamic_maybe(5)).inspect
      assert_equal "nil", dynamic_ident(dynamic_maybe(-5)).inspect
      assert_equal "falsy", truth(dynamic_maybe(-1))
      assert_equal "truthy", truth(dynamic_maybe(1))
    end

    def test_untyped_passes_into_typed_parameters
      assert_equal 5, strlen(dynamic_ident("héllo"))
      assert_equal 0, strlen(dynamic_ident(""))
      n = dynamic_ident(41) #: Integer
      assert_equal 42, n + 1
      str = dynamic_ident("s") #: String
      assert_equal "sss", str * 3
      opt = dynamic_ident(nil) #: String?
      assert_equal "nil", opt.inspect
      assert_equal "\"d\"", (opt || "d").inspect
    end

    def test_untyped_nilable_is_untyped
      assert_equal "nil", dynamic_show(nil)
      assert_equal "3", dynamic_show(3)
      assert_equal "\"héllo\"", dynamic_show("héllo")
      assert_equal "nil", pass_on(nil).inspect
      assert_equal "\"x\"", pass_on("x").inspect
      assert_equal "falsy", truth(pass_on(false))
    end

    def test_untyped_collections
      arr = [1, "two", :three, 4.0, nil] #: Array[untyped]
      assert_equal 5, arr.size
      assert_equal "[1, \"two\"]", arr.first(2).inspect
      assert_equal "nil", arr.last.inspect
      assert_equal ["1", "two", "three", "4.0", ""], arr.map { |e| e.to_s }
      assert_equal 4, arr.compact.size
      arr << [5]
      assert_equal "[1, \"two\", :three, 4.0, nil, [5]]", arr.inspect
      h = { "a" => 1, "b" => "bee", "c" => nil } #: Hash[String, untyped]
      assert_equal "{\"a\" => 1, \"b\" => \"bee\", \"c\" => nil}", h.inspect
      assert_equal "1", h["a"].inspect
      assert_equal "\"bee\"", h["b"].inspect
      assert_equal "nil", h["c"].inspect
      assert_equal "nil", h["zz"].inspect
      assert_equal "\"bee\"", h.fetch("b", 0).inspect
      assert_equal "0", h.fetch("zz", 0).inspect
      assert_equal ["a", "b", "c"], h.keys
      got = [] #: Array[String]
      h.each { |k, v| got << "#{k}=#{v.inspect} #{truth(v)}" }
      assert_equal ["a=1 truthy", "b=\"bee\" truthy", "c=nil falsy"], got
      empty = [] #: Array[untyped]
      assert_equal "[]", empty.inspect
      assert_equal "[]", empty.first(1).inspect
      assert_equal "nil", empty.last.inspect
    end

    def test_untyped_attributes_narrow_like_locals
      holders = [Holder.new("abc"), Holder.new(41), Holder.new(nil), Holder.new([1, 2]), Holder.new(:s)]
      assert_equal [["str ABC", "S3", "abc!"], ["int 42", "?", "41!"], ["nil", "?", "!"], ["other [1, 2]", "A2", "[1, 2]!"], ["other :s", "?", "s!"]],
                   holders.map { |hd| [hd.describe, hd.via_case, hd.ivar_dyn] }
    end

    def test_symbol_keyed_untyped_hash_as_configuration
      cfg = { port: 8080, host: "localhost", tags: ["a", "b"] } #: Hash[Symbol, untyped]
      assert_equal 8081, (cfg[:port] || 80) + 1
      assert_equal "LOCALHOST", cfg[:host].upcase
      assert_equal 2, cfg[:tags].size
      assert_equal "dflt", cfg[:missing] || "dflt"
      assert_equal "has host", cfg[:host] ? "has host" : "no host"
      assert_equal "no missing", cfg[:missing] ? "missing" : "no missing"
    end

    def test_untyped_into_bool_nilable_and_generic_parameters
      assert_equal "n", dynamic_yes_no(dynamic_ident(nil))
      assert_equal "n", dynamic_yes_no(dynamic_ident(false))
      assert_equal "y", dynamic_yes_no(dynamic_ident(0))
      assert_equal "y", dynamic_yes_no(dynamic_ident(""))
      assert_equal "y", dynamic_yes_no(dynamic_ident(true))
      assert_equal "none", opt_desc(dynamic_ident(nil))
      assert_equal "n=4", opt_desc(dynamic_ident(4))
      assert_equal 6, total_of(dynamic_ident([1, 2, 3]))
      # rugaroo checks the untyped argument at the call (TypeError, decision 20) where MRI fails inside (NoMethodError), so the message is not compared.
      err = begin
        dynamic_up(dynamic_ident(5))
        nil
      rescue StandardError => e
        e
      end
      assert_equal false, err.nil?
    end
  end

  # #51: keywords through a call on an untyped value, as a trailing Symbol-keyed Hash
  class DynKwGreeter
    #: (String, name: String, ?n: Integer) -> String
    def greet(hi, name:, n: 1) = "#{hi} #{name}" * n

    #: (?Integer, **Integer rest) -> Array[untyped]
    def rest(a = 0, **rest) = [a, rest]

    #: (Hash[Symbol, Integer], ?k: Integer) -> Array[untyped]
    def first_hash(h, k: 1) = [h, k]
  end

  class DynamicKeywordTest < Minitest::Test
    def test_keywords_on_untyped
      x = DynKwGreeter.new #: untyped
      assert_equal ["hi ann", "yo boyo bo"], [x.greet("hi", name: "ann"), x.greet("yo", name: "bo", n: 2)]
      assert_equal [[5, { k: 1 }], [0, {}]], [x.rest(5, k: 1), x.rest]
      assert_equal [[{ a: 1 }, 1], [{ a: 1 }, 3]], [x.first_hash({ a: 1 }), x.first_hash({ a: 1 }, k: 3)]
    end

    def test_keyword_errors_on_untyped
      x = DynKwGreeter.new #: untyped
      e = assert_raises(ArgumentError) { x.greet("hi") }
      assert_equal "missing keyword: :name", e.message
      e = assert_raises(ArgumentError) { x.greet("hi", name: "a", zz: 1) }
      assert_equal "unknown keyword: :zz", e.message
    end
  end

  class DynamicOverloadTest < Minitest::Test
    def test_gsub_regexp_on_untyped
      m = "users/:id" #: untyped
      assert_equal "users/X", m.gsub(/:\w+/, "X")
      assert_equal "users/Y", m.sub(/:\w+/, "Y")
    end

    def test_universal_block_on_untyped
      x = [1, 2] #: untyped
      seen = [] #: Array[untyped]
      assert_same x, x.tap { |v| seen << v }
      assert_equal [[1, 2]], seen
    end
  end

  # rack's `env["rack.input"].read(n)` and Cuba's `matcher.call` on what it holds untyped (decision 175).
  class DynamicOverloadAndProcTest < Minitest::Test
    def test_arity_overload_on_untyped
      io = StringIO.new("abcdef") #: untyped
      assert_equal ["ab", "cdef"], [io.read(2), io.read]
    end

    def test_proc_call_on_untyped
      add = ->(x) { x + 1 } #: ^(Integer) -> Integer
      m = add #: untyped
      say = -> { "hi" } #: ^() -> String
      s = say #: untyped
      assert_equal [3, "hi"], [m.call(2), s.call]
    end
  end

  # An inherited method a subclass answers through the dispatcher's shared arm: self stays the subclass,
  # though Go promotes the parent's own wrapper into it (decision 216).
  class SharedBase
    def hi = "base:#{who}"
    def who = "base"
    def sub? = who == "sub"
  end

  class SharedSub < SharedBase
    def who = "sub"
  end

  class SharedLeaf < SharedSub
  end

  class DynamicSharedArmTest < Minitest::Test
    def test_inherited_method_keeps_self
      xs = [SharedBase.new, SharedSub.new, SharedLeaf.new] #: Array[untyped]
      assert_equal ["base:base", "base:sub", "base:sub"], xs.map(&:hi)
      assert_equal [false, true, true], xs.map(&:sub?)
      assert_predicate SharedLeaf.new, :sub?
    end
  end

  class SplatTarget
    #: (Integer, Integer, ?Integer) -> Integer
    def joined(a, b, c = 0) = a + b + c
  end

  # Splats into a dynamic call or a computed send: one argument list (decision 227).
  class DynamicSplatTest < Minitest::Test
    def test_two_splats_and_send
      x = SplatTarget.new #: untyped
      a = [1] #: Array[Integer]
      b = [2, 3] #: Array[Integer]
      m = :joined #: Symbol
      assert_equal [6, 3, 6], [x.joined(*a, *b), x.joined(*a, 2), SplatTarget.new.send(m, 1, *b)]
    end
  end

  # keys/values/to_a on an untyped Hash or Array: the [untyped] instantiation's method (decision 232).
  class DynamicContainerTest < Minitest::Test
    def test_untyped_container_methods
      h = { "a" => 1, "b" => 2 } #: untyped
      assert_equal [["a", "b"], [1, 2], [["a", 1], ["b", 2]]], [h.keys, h.values, h.to_a]
    end

    # Kernel's methods on typed containers held untyped: one class-ID arm for every element type (#96).
    def test_kernel_methods_on_typed_containers
      xs = [[1, 2], { "a" => 1 }, Set[:a], (1..3)] #: Array[untyped]
      assert_equal [false, false, false, true], xs.map(&:frozen?)
      assert_equal [true, true, true, true], xs.map { |x| x.itself.equal?(x) }
      assert_equal [true, true, true, true], xs.map { |x| x.object_id == x.object_id }
      assert_equal [nil, nil, nil, nil], xs.map { |x| x.instance_variable_get(:@nope) }
      assert_equal ["[1, 2]", "{\"a\" => 1}"], [xs[0].send(:format, "%s", xs[0]), xs[1].send(:format, "%s", xs[1])]
      assert_equal [true, true], [xs[0].nil? == false, xs[2].instance_of?(Set)]
    end

    # Whole-array methods on a typed Array held untyped run through the view (decision 236): results keep the
    # element type, sort! is in place, frozen is checked, join flattens nested arrays.
    def test_view_methods_on_typed_arrays
      a = [3, 1, 2] #: Array[Integer]
      x = a #: untyped
      assert_equal [[1, 2, 3], [2, 1, 3], 3, 1, 2, true, false, "3-1-2"],
                   [x.sort, x.reverse, x.max, x.min, x.index(2), x.include?(1), x.include?(9), x.join("-")]
      assert_equal [3, 1, 2], a
      assert_same x, x.sort!
      assert_equal [1, 2, 3], a
      w = ["b", nil, "a"] #: Array[String?]
      y = w #: untyped
      assert_equal [["b", "a"], "ba"], [y.compact, y.compact.join]
      n = [[1, [2]], 3] #: Array[untyped]
      assert_equal "1,2,3", n.join(",")
      e = [] #: Array[Integer]
      z = e #: untyped
      assert_equal [nil, nil, nil, [], ""], [z.max, z.min, z.index(1), z.sort, z.join]
      f = [2, 1].freeze #: Array[Integer]
      g = f #: untyped
      assert_raises(FrozenError) { g.sort! }
      assert_equal [1, 2], g.sort
    end

    # NUM2INT: one RangeError helper words both ends (rbNum2Int; "too big" for a negative was wrong)
    def test_num2int_range_error_wording
      File.open(__FILE__) do |f|
        e = assert_raises(RangeError) { f.lineno = -(2**40) }
        assert_equal "integer -1099511627776 too small to convert to 'int'", e.message
        e = assert_raises(RangeError) { f.lineno = 2**40 }
        assert_equal "integer 1099511627776 too big to convert to 'int'", e.message
      end
    end

    # sleep past what a Duration holds (~292 years) must still block, not wrap to a negative duration and return at once
    def test_sleep_huge_blocks
      q = Queue.new
      t = Thread.new do
        q << :started
        sleep(1e10)
        q << :woke
      end
      assert_equal :started, q.pop
      100.times { Thread.pass }
      assert_equal true, t.alive?
      assert_equal true, q.empty?
    end

    # Encoding.default_* are process-wide and the build runs under -race when threads exist (decision 88)
    def test_encoding_defaults_across_threads
      writer = Thread.new do
        200.times { Encoding.default_external = Encoding::UTF_8 }
        :done
      end
      seen = 0
      200.times { seen += 1 if Encoding.default_external == Encoding::UTF_8 }
      writer.join
      assert_equal 200, seen
      assert_nil Encoding.default_internal
    end

    # Process::Waiter#value memoizes its status; two threads asking at once must not race (-race builds, decision 88)
    def test_open3_wait_value_from_two_threads
      codes = [] #: Array[untyped]
      Open3.popen3("true") do |_in, _out, _err, wt|
        ts = Array.new(2) { Thread.new { wt.value } }
        ts.each(&:join)
        codes = ts.map { |t| t.value.exitstatus }
      end
      assert_equal [0, 0], codes
    end

    def test_view_set_and_edit_methods
      a = [3, 1, 2, 1] #: Array[Integer]
      x = a #: untyped
      b = [1, 9] #: Array[Integer]
      y = b #: untyped
      assert_equal [[3, 1, 2, 1, 1, 9], [3, 2], [1], [3, 1, 2, 9], [3, 1, 2], true, false, [3, 1, 2, 1, 3, 1, 2, 1]],
                   [x + y, x - y, x & y, x | y, x.uniq, x.intersect?(y), x.intersect?([7]), x * 2]
      assert_equal ["3,1,2,1", 2, 3, [1, 2, 1, 3], [2, 1], [3, 1]], [x * ",", x.count(1), x.rindex(1), x.rotate, x.drop(2), x.take(2)]
      assert_same x, x.reverse!
      assert_equal [1, 2, 1, 3], a
      assert_equal [[1, 2, 3], nil], [x.uniq!, x.uniq!]
      assert_equal 2, x.delete(2)
      assert_equal [nil, 1, [3], [3, 7, 8], [3, 7, 8, 1, 2], [0, 0, 0, 0, 0]],
                   [x.delete(9), x.delete_at(0), a.dup, x.insert(1, 7, 8).dup, x.concat([1, 2]).dup, x.fill(0)]
      assert_same x, x.replace([5, 6])
      assert_equal [5, 6], a
      pairs = [[1, "a"], [2, "b"]] #: Array[[Integer, String]]
      q = pairs #: untyped
      assert_equal [[2, "b"], [1, "a"], nil], [q.assoc(2), q.rassoc("a"), q.assoc(3)]
      n = [[1, [2]], 3] #: Array[untyped]
      assert_equal [[1, 2, 3], [1, 2, 3], nil], [n.flatten, n.flatten!, n.flatten!]
      t = [[1], [2]] #: Array[Array[Integer]]
      u = t #: untyped
      assert_equal [1, 2], u.flatten
      e = [] #: Array[Integer]
      z = e #: untyped
      assert_equal [[], [], nil, [], 0], [z.rotate, z.uniq, z.uniq!, z - [1], z.count]
      assert_raises(ArgumentError) { z.take(-1) }
    end

    def test_view_hash_methods
      h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      x = h #: untyped
      o = { "c" => 3 } #: Hash[String, Integer]
      y = o #: untyped
      assert_equal [{ "a" => 1, "b" => 2, "c" => 3 }, { "b" => 2 }, { "a" => 1 }, ["a", 1, "b", 2], true, false, "b", nil],
                   [x.merge(y), x.except("a"), x.slice("a", "z"), x.flatten, x.value?(2), x.has_value?(7), x.key(2), x.key(7)]
      assert_equal({ "a" => 1, "b" => 2 }, h)
      c = x.to_h # self, as MRI's
      c.store("d", 4)
      assert_equal [3, 3], [h.size, c.size]
      h.delete("d")
      assert_same x, x.merge!(y)
      assert_equal({ "a" => 1, "b" => 2, "c" => 3 }, h)
      u = { "a" => 5 } #: Hash[String, Integer]
      assert_same x, x.update(u)
      z = { "z" => 0 } #: Hash[String, Integer]
      assert_same x, x.replace(z)
      assert_equal({ "z" => 0 }, h)
      n = { "a" => 1, "b" => nil } #: Hash[String, Integer?]
      w = n #: untyped
      assert_equal [{ "a" => 1 }, true, "b", { "a" => 1, "b" => nil }], [w.compact, w.value?(nil), w.key(nil), w.merge(w)]
      assert_same w, w.compact!
      assert_equal [{ "a" => 1 }, nil], [n, w.compact!]
      k = { 1 => [[2, 3]] } #: Hash[Integer, Array[Array[Integer]]]
      assert_equal [[1, [[2, 3]]], [1, [2, 3]]], [k.flatten, k.flatten(2)]
      fz = { "q" => 1 }.freeze #: Hash[String, Integer]
      g = fz #: untyped
      r2 = { "r" => 2 } #: Hash[String, Integer]
      assert_raises(FrozenError) { g.merge!(r2) }
      assert_equal({ "q" => 1, "r" => 2 }, g.merge(r2))
    end
  end

  # _DynGen keyed a private method "_name": a public `_name` shared the key (duplicate case, or its case pruned) (#101)
  # decision 316: a computed send/respond_to? on a receiver of known class switches over that class's names only
  class SendOnAnimal
    #: (String) -> void
    def initialize(name) = @name = name
    #: () -> String
    def speak = "#{@name} speaks"
    #: (Integer) -> Integer
    def twice(n) = n * 2

    private

    #: () -> String
    def secret = "shh"
  end

  class SendOnDog < SendOnAnimal
    #: () -> String
    def fetch = "fetched"
    #: (Symbol, *untyped) -> untyped
    def method_missing(name, *args) = "dog mm #{name}(#{args.size})"
    #: (Symbol, ?bool) -> bool
    def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("x")
  end

  class SendOnTypedReceiverTest < Minitest::Test
    def test_typed_receiver
      a = SendOnAnimal.new("cat")
      assert_equal ["cat speaks", "shh"], %w[speak secret].map { |m| a.send(m) }
      assert_equal 8, a.send(%w[twice].first, 4)
      assert_raises(NoMethodError) { a.public_send(%w[secret].first) }
      assert_raises(NoMethodError) { a.send(%w[nope].first) }
      assert_equal false, a.respond_to?(%w[secret].first)
      assert_equal true, a.respond_to?(%w[secret].first, true)
      assert_equal false, a.respond_to?(%w[fetch].first)
    end

    def test_subclass_behind_parent_type
      pet = SendOnDog.new("rex") #: SendOnAnimal
      assert_equal "fetched", pet.send(%w[fetch].first)
      assert_equal "dog mm xyz(2)", pet.send(%w[xyz].first, 1, 2)
      assert_equal true, pet.respond_to?(%w[fetch].first)
      assert_equal true, pet.respond_to?(%w[xa].first)
      assert_equal false, pet.respond_to?(%w[ya].first)
    end

    def test_union_receiver
      u = [SendOnAnimal.new("owl"), SendOnDog.new("pug")].fetch(1) #: SendOnAnimal | SendOnDog
      assert_equal "pug speaks", u.send(%w[speak].first)
      assert_equal true, u.respond_to?(%w[fetch].first)
    end

    # decision 319: a computed respond_to? on an untyped value answers over every name without any dispatcher
    def test_computed_respond_to_on_untyped
      u = dynamic_ident(SendOnDog.new("rex"))
      assert_equal [true, true, false, true], %w[speak fetch nope xq].map { |m| u.respond_to?(m) }
      assert_equal true, u.respond_to?(%w[secret].first, true)
      assert_equal false, u.respond_to?(%w[secret].first)
    end

    # a class object, a module's self (some includer) and a core class scope too
    def test_class_object_module_and_core_receivers
      k = SendOnDog #: singleton(SendOnAnimal)
      assert_equal "kennel", k.send(%w[kennel].first)
      assert_equal "DynamicTests::SendOnDog", k.public_send(%w[name].first)
      assert_equal true, k.respond_to?(%w[kennel].first)
      assert_equal false, SendOnAnimal.respond_to?(%w[kennel].first)
      assert_equal "tagged pug", SendOnTagged.new.tagged(%w[tag].fetch(0))
      assert_equal "ABC", "abc".send(%w[upcase].first)
      assert_equal 3, "abc".public_send(%w[size].first)
      assert_equal false, "abc".respond_to?(%w[speak].first)
      s = ["x", 1].fetch(0) #: String | Integer
      assert_equal "X", s.send(%w[upcase].first)
    end
  end

  class SendOnAnimal
    #: () -> String
    def self.shelter = "none"
  end

  # decision 353: the interpolated name's literal prefix types render_* params from this call alone
  class SendPatternNode
    attr_reader :kind #: Symbol
    attr_reader :text #: String

    #: (Symbol kind, String text) -> void
    def initialize(kind, text)
      @kind = kind
      @text = text
    end
  end

  class SendPatternRenderer
    #: (SendPatternNode node, Integer depth) -> String
    def render(node, depth)
      send("render_#{node.kind}", node, depth)
    end

    def render_title(node, depth) = "#" * (depth + 1) + " " + node.text.upcase

    def render_para(node, depth) = ("  " * depth) + node.text.strip

    private

    def render_secret(node, _depth) = node.text.reverse
  end

  class SendPatternLoud < SendPatternRenderer
    def render_shout(node, depth) = node.text.upcase + "!" * depth
  end

  # decision 354: method_missing keeps the dynamic default; arms of different types join
  class SendPatternGhost
    #: (String kind, Integer n) -> (String | Integer)
    def pick(kind, n)
      send(:"#{kind}_value", n)
    end

    def word_value(n) = "w#{n}"

    def num_value(n) = n * 2

    def method_missing(name, *args)
      name.to_s.end_with?("_value") ? "ghost #{name} #{args.size}" : super
    end

    def respond_to_missing?(name, priv = false) = name.to_s.end_with?("_value") || super
  end

  class SendPatternTest < Minitest::Test
    def test_method_missing_and_mixed_arms
      g = SendPatternGhost.new
      assert_equal "w3", g.pick("word", 3)
      assert_equal 6, g.pick("num", 3)
      assert_equal "ghost other_value 1", g.pick("other", 3)
    end

    def test_interpolated_name_reaches_matching_methods
      r = SendPatternRenderer.new
      assert_equal "## HI", r.render(SendPatternNode.new(:title, "hi"), 1)
      assert_equal "    x", r.render(SendPatternNode.new(:para, " x "), 2)
      assert_equal "cba", r.render(SendPatternNode.new(:secret, "abc"), 0)
      assert_raises(NoMethodError) { r.render(SendPatternNode.new(:nope, "x"), 0) }
    end

    def test_subclass_method_behind_parent_type
      r = SendPatternLoud.new #: SendPatternRenderer
      assert_equal "HEY!!", r.render(SendPatternNode.new(:shout, "hey"), 2)
      assert_equal "# A", r.render(SendPatternNode.new(:title, "a"), 0)
    end
  end

  class SendOnDog
    #: () -> String
    def self.kennel = "kennel"
  end

  module SendOnTagging
    #: (String) -> String
    def tagged(m) = "tagged #{send(m)}"
  end

  class SendOnTagged
    include SendOnTagging
    #: () -> String
    def tag = "pug"
  end

  class DynGenPrivateKeyTest < Minitest::Test
    def test_public_underscore_and_private_plain_name_on_generic_receiver
      x = [1] #: untyped
      assert_equal ["pub", "both"], [x._dyngen_pub, x._dyngen_both]
      assert_equal "priv", x.send(:dyngen_both)
      assert_equal "priv2", x.send(:dyngen_priv)
      ok = [x.dyngen_both, x.dyngen_priv].map { "no" } rescue :nomethod
      assert_equal :nomethod, ok
    end
  end
end

class Array
  #: () -> String
  def _dyngen_pub = "pub"

  #: () -> String
  def _dyngen_both = "both"

  private

  #: () -> String
  def dyngen_both = "priv"

  #: () -> String
  def dyngen_priv = "priv2"
end

module DynamicTests
  class DynProcAndSetTest < Minitest::Test
    def test_proc_responds_to_call
      h = { a: ->(a, b) { a + b }, b: :< } #: Hash[Symbol, untyped]
      assert h[:a].respond_to?(:call)
      refute h[:b].respond_to?(:call)
      assert_equal 3, h[:a].call(1, 2)
    end

    def test_set_merge_untyped
      h = { s: Set.new([1]) } #: Hash[Symbol, untyped]
      s = h[:s]
      s.merge([2, 3])
      assert_equal 3, s.size
      t = Set[1, 2]
      t.merge(Set[4])
      assert_equal [1, 2, 4], (t | [5]).to_a.first(3)
      assert_equal 4, (t | [5]).size
    end
  end

  # An Array behind an untyped receiver takes the same allocation checks as a typed one (#109, decision 273).
  class DynHugeAllocTest < Minitest::Test
    HUGE = 2**62

    def test_untyped_array_repeat_too_big
      v = dynamic_ident([1, 2])
      e = assert_raises(ArgumentError) { v * HUGE }
      assert_equal "argument too big", e.message
      e = assert_raises(ArgumentError) { v * -1 }
      assert_equal "negative argument", e.message
      assert_equal [1, 2, 1, 2], v * 2
      assert_equal [], dynamic_ident([]) * HUGE
    end

    def test_untyped_array_insert_far
      v = dynamic_ident([1, 2])
      e = assert_raises(IndexError) { v.insert(HUGE, 3) }
      assert_equal "index #{HUGE} too big", e.message
      assert_equal [1, 2], v
      v.insert(2, 9)
      assert_equal [1, 2, 9], v
    end
  end
end

module DynamicTests
  class DynEnumBag
    include Enumerable

    def initialize(*list) = @list = list

    def each(&block)
      @list.each(&block)
      self
    end
  end

  # A computed public_send of an Enumerable predicate without a block takes its block-less form (decision 417).
  class DynEnumPredicateSendTest < Minitest::Test
    def dyn_enum_ask(obj, name, *args) = obj.public_send(name, *args) ? true : false

    def test_blockless_predicates_by_name
      assert dyn_enum_ask(DynEnumBag.new(1, 2), :all?)
      refute dyn_enum_ask(DynEnumBag.new(nil, 1), :all?)
      assert dyn_enum_ask(DynEnumBag.new(nil, 1), :one?)
      refute dyn_enum_ask(DynEnumBag.new(1, 2), :none?)
      assert dyn_enum_ask(DynEnumBag.new(1, "a"), :any?, String)
      refute dyn_enum_ask(DynEnumBag.new(1, "a"), :all?, Integer)
    end
  end
end

module DynamicTests
  # A wholly computed send passing a String to a handler whose parameter only its Symbol default types (net-ftp's fixture
  # server, decision 423): the String reaches it, as MRI's send passes it.
  class DynCommandServer
    def handle(line)
      command, argument = line.split(" ", 2)
      argument.nil? ? send(command.downcase.to_sym) : send(command.downcase.to_sym, argument)
    end

    def help(param = :default) = param == :default ? "help" : "help #{param}"

    def stat(param = :default) = param == :default ? "stat" : "stat #{param}"
  end

  class DynCommandSendTest < Minitest::Test
    def test_computed_send_types_a_defaulted_param
      s = DynCommandServer.new
      assert_equal %w[help stat], [s.handle("HELP"), s.handle("STAT")]
      assert_equal ["help retr", "stat x y"], [s.handle("HELP retr"), s.handle("STAT x y")]
    end
  end
end

module DynamicTests
  # Private defs, attr accessors and a module's self calls of its includers' methods dispatch by type switch
  # (decision 486): the override runs, send reaches a private one, a plain call does not, respond_to? sees it with true.
  # Its methods are annotated, so they stay the module's rather than copies in each includer (decision 307).
  module DispMixin
    #: () -> String
    def disp_label = "#{disp_name}:#{disp_size}:#{disp_tag}"

    #: () -> Integer
    def disp_bump
      self.disp_count = disp_count + 1
      disp_count
    end

    def disp_mixed = "m:#{disp_hidden}"

    private

    def disp_hidden = "mod-hidden"
  end

  class DispBase
    attr_accessor :disp_count #: Integer

    def initialize
      @disp_count = 0
    end

    def disp_name = "base"

    def disp_size = 1

    def disp_tag = "t#{disp_secret}"

    def disp_reveal = disp_secret

    private

    def disp_secret = "base-secret"
  end

  class DispKid < DispBase
    include DispMixin

    def disp_name = "kid"

    private

    def disp_secret = "kid-secret"

    def disp_hidden = "kid-hidden"
  end

  class DispGrandKid < DispKid
    def disp_size = 3
  end

  class DispOther < DispBase
    include DispMixin

    attr_reader :disp_extra #: String

    def initialize
      super
      @disp_extra = "x"
    end

    def disp_tag = "other-#{disp_extra}"
  end

  # Annotated module methods stay shared (decision 307 copies only unannotated ones): a self call of a name a
  # sibling module answers for every includer is that module's method (decision 498).
  module DispStore
    # @rbs @disp_env: Hash[String, String]

    #: (String) -> String?
    def disp_get(name)
      @disp_env[name]
    end

    #: (String, String) -> String
    def disp_set(name, value)
      @disp_env[name] = value
    end

    private

    # a private one keeps its direct call: instantiated at this module's constraint, whichever class includes both
    #: (String) -> String
    def disp_key(name)
      "key:#{name}"
    end
  end

  module DispHelpers
    #: () -> String
    def disp_host
      disp_get("HOST").to_s
    end

    #: (String) -> String
    def disp_host=(value)
      disp_set("HOST", value)
    end

    #: () -> String
    def disp_host_key
      disp_key("HOST")
    end
  end

  class DispReq
    include DispStore
    include DispHelpers

    #: (Hash[String, String]) -> void
    def initialize(env)
      @disp_env = env
    end
  end

  class DispSubReq < DispReq
  end

  class DispOtherReq
    include DispStore
    include DispHelpers

    #: () -> void
    def initialize
      @disp_env = { "HOST" => "other" }
    end

    #: () -> Integer
    def disp_extra = 1
  end

  module DispLoud
    #: () -> String
    def disp_shout
      disp_word.upcase
    end
  end

  module DispWords
    #: () -> String
    def disp_word = "word"
  end

  class DispPlain
    include DispWords
    include DispLoud
  end

  class DispOwn
    include DispWords
    include DispLoud

    #: () -> String
    def disp_word = "own"
  end

  class DispSiblingModuleTest < Minitest::Test
    def test_sibling_module_method_through_self
      req = DispReq.new({ "HOST" => "a" })
      sub = DispSubReq.new({ "HOST" => "b" })
      assert_equal %w[a b], [req.disp_host, sub.disp_host]
      req.disp_host = "c"
      assert_equal ["c", ""], [req.disp_host, DispReq.new({}).disp_host]
    end

    def test_sibling_modules_private_method_with_several_includers
      assert_equal %w[key:HOST key:HOST key:HOST other],
                   [DispReq.new({}).disp_host_key, DispSubReq.new({}).disp_host_key, DispOtherReq.new.disp_host_key,
                    DispOtherReq.new.disp_host]
    end

    def test_an_includers_own_method_still_wins
      assert_equal %w[WORD OWN], [DispPlain.new.disp_shout, DispOwn.new.disp_shout]
    end
  end

  class DispatchPrivateTest < Minitest::Test
    def test_module_self_call_runs_the_override
      assert_equal ["kid:1:tkid-secret", "kid:3:tkid-secret", "base:1:other-x"],
                   [DispKid.new, DispGrandKid.new, DispOther.new].map(&:disp_label)
    end

    def test_private_override_runs_from_the_base_body
      assert_equal %w[base-secret kid-secret kid-secret], [DispBase.new, DispKid.new, DispGrandKid.new].map(&:disp_reveal)
      assert_equal %w[m:kid-hidden m:kid-hidden m:mod-hidden], [DispKid.new, DispGrandKid.new, DispOther.new].map(&:disp_mixed)
    end

    def test_send_reaches_private_and_plain_call_does_not
      kid = DispGrandKid.new
      assert_equal "kid-secret", kid.send(:disp_secret)
      assert_equal "kid-hidden", kid.send(:disp_hidden)
      assert_equal "mod-hidden", DispOther.new.send(:disp_hidden)
      [%i[disp_secret], %i[disp_hidden]].each do |(name)|
        raised = begin
          dynamic_ident(kid).public_send(name)
          nil
        rescue NoMethodError => e
          e.class
        end
        assert_equal NoMethodError, raised
      end
      raised = begin
        dynamic_ident(DispBase.new).disp_secret
        nil
      rescue NoMethodError => e
        e.class
      end
      assert_equal NoMethodError, raised
    end

    def test_respond_to_private
      kid = DispKid.new
      assert_equal [false, true, false, true], [kid.respond_to?(:disp_secret), kid.respond_to?(:disp_secret, true),
                                                kid.respond_to?(:disp_hidden), kid.respond_to?(:disp_hidden, true)]
      assert_equal [true, true], [kid.respond_to?(:disp_reveal), kid.respond_to?(:disp_count=)]
    end

    def test_attr_accessor_through_the_module
      kid = DispKid.new
      assert_equal [1, 2], [kid.disp_bump, kid.disp_bump]
      kid.disp_count = 10
      assert_equal [11, 11], [kid.disp_bump, kid.disp_count]
      assert_equal [1, "x"], [DispOther.new.disp_bump, DispOther.new.disp_extra]
    end
  end
end
