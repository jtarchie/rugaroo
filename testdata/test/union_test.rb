# rbs_inline: enabled

require "minitest/autorun"

# Union types (decision 150): `A | B` values, calls on them as per-member
# type switches, narrowing by is_a?/case/nil checks, and the untyped boundary.

#: (Integer | String) -> String
def union_show(x) = "#{x}!"

#: (Integer | String) -> Integer
def union_width(x) = x.size

#: (bool) -> (Integer | String)
def union_pick(b) = b ? 7 : "seven"

#: (Integer | Float) -> (Integer | Float)
def union_bump(n) = n + 1

#: (untyped) -> untyped
def union_untyped(v) = v

#: (bool) -> (String | Regexp)
def union_sep(b) = b ? "," : /;\s*/

#: (Integer) -> String
def union_int_only(n) = "int #{n}"

#: (bool) -> (Integer | Float)
def union_num(b) = b ? 2 : 0.5

# no annotations: these join to unions (decision 150), where they were errors
def union_ident(x) = x
def union_half(x) = x / 2
# @rbs b: bool
def union_pick_inferred(b) = b ? 1 : "x"

#: (Integer | String | nil) -> String
def union_guard_nil(x)
  return "none" if x.nil?

  y = x #: Integer | String
  y.to_s
end

#: (Integer | String) -> String
def union_guard_not(x)
  return "int" if !x.is_a?(String)

  x.upcase
end

#: (Integer | String) -> String
def union_tag(v)
  case v
  when Integer then "int #{v}"
  else "str #{v}"
  end
end

# A generic value reaches a union parameter checked at run time, as untyped does.
#: [T] (T) -> String
def union_generic_tag(x) = union_tag(x)

#: (Array[untyped] | String | nil) -> String
def union_kind_opt(x)
  case x
  when Array then "array #{x.size}"
  when String then "string"
  else "nil"
  end
end

#: (Array[untyped] | String) -> String
def union_kind(x)
  case x
  when Array then "array #{x.size}"
  when String then "string #{x}"
  end
end

module UnionTests
  class Dog
    #: () -> String
    def speak = "woof"
  end

  class Robot
    #: () -> String
    def speak = "beep"
  end

  class Box
    attr_reader :v #: Integer | String

    #: (Integer | String) -> void
    def initialize(v)
      @v = v
    end

    #: () -> String
    def label
      case @v
      when Integer then "int #{@v + 1}"
      when String then "str #{@v.upcase}"
      end
    end
  end

  class Taker
    #: (Integer | String) -> String
    def take(x) = x.inspect
  end

  class UnionTest < Minitest::Test
    def test_calls_switch_on_member
      assert_equal "1!", union_show(1)
      assert_equal "a!", union_show("a")
      assert_equal 8, union_width(255)
      assert_equal 5, union_width("hello")
    end

    def test_union_result
      a = union_pick(true)
      b = union_pick(false)
      assert_equal 7, a
      assert_equal "seven", b
      assert_equal "7", a.to_s
      assert_equal "\"seven\"", b.inspect
      assert(a == 7)
      refute(b == 7)
    end

    #: (Integer | String | nil) -> String
    def nil_member(x) = x.inspect

    def test_nil_member
      assert_equal "nil", nil_member(nil)
      assert_equal "3", nil_member(3)
      assert_equal "\"s\"", nil_member("s")
    end

    #: (Integer | String | nil) -> String
    def truthy(x)
      if x
        x.to_s * 2
      else
        "none"
      end
    end

    def test_truthiness_drops_nil
      assert_equal "33", truthy(3)
      assert_equal "ss", truthy("s")
      assert_equal "none", truthy(nil)
    end

    #: (Integer | String) -> Integer
    def twice(x)
      if x.is_a?(Integer)
        x * 2
      else
        x.size * 2
      end
    end

    #: (Integer | String | Symbol) -> String
    def guard(x)
      return "int" if x.is_a?(Integer)
      return "str #{x.length}" unless x.is_a?(Symbol)

      "sym #{x.length}"
    end

    #: (Integer | String | Symbol) -> String
    def negated(x)
      if !x.is_a?(Integer)
        x.to_s
      else
        (x + 1).to_s
      end
    end

    #: (Integer | String | Symbol) -> Integer
    def either(x)
      if x.is_a?(String) || x.is_a?(Symbol)
        x.size
      else
        x
      end
    end

    #: (Integer | String | Symbol) -> String
    def unless_else(x)
      unless x.is_a?(Integer)
        x.to_s
      else
        (x * 3).to_s
      end
    end

    def test_narrowing
      assert_equal 8, twice(4)
      assert_equal 6, twice("abc")
      assert_equal "int", guard(1)
      assert_equal "str 3", guard("abc")
      assert_equal "sym 4", guard(:size)
      assert_equal "s", negated("s")
      assert_equal "2", negated(1)
      assert_equal 3, either("abc")
      assert_equal 2, either(:ab)
      assert_equal 9, either(9)
      assert_equal "z", unless_else(:z)
      assert_equal "6", unless_else(2)
    end

    #: (Integer | Float | String) -> String
    def classify(x)
      case x
      when Integer, Float then "number #{x * 2}"
      else "text #{x.upcase}"
      end
    end

    #: (Integer | Float | String | nil) -> String
    def classify_nil(x)
      case x
      when nil then "nothing"
      when String then x.reverse
      when Numeric then (x + 1).to_s
      end
    end

    def test_case_when
      assert_equal "number 4", classify(2)
      assert_equal "number 5.0", classify(2.5)
      assert_equal "text AB", classify("ab")
      assert_equal "nothing", classify_nil(nil)
      assert_equal "ba", classify_nil("ab")
      assert_equal "2", classify_nil(1)
      assert_equal "2.5", classify_nil(1.5)
    end

    def test_numeric_union
      assert_equal 2, union_bump(1)
      assert_in_delta 2.5, union_bump(1.5)
      assert_equal "Float", union_bump(0.5).class.name
    end

    def test_struct_members
      pets = [Dog.new, Robot.new] #: Array[Dog | Robot]
      assert_equal %w[woof beep], pets.map { |p| p.speak }
    end

    def test_containers
      items = [] #: Array[Integer | String]
      items << 1
      items << "two"
      items.push(3)
      assert_equal [1, "two", 3], items
      assert_equal %w[1 two 3], items.map { |i| i.to_s }
      assert_includes items, "two"
      h = {} #: Hash[Symbol, Integer | String]
      h[:a] = 1
      h[:b] = "b"
      assert_equal({ a: 1, b: "b" }, h)
      v = h.fetch(:a)
      assert_equal 2, twice(v) / 1
    end

    def test_ivar_and_reader
      assert_equal "int 42", Box.new(41).label
      assert_equal "str HI", Box.new("hi").label
      assert_equal "hi", Box.new("hi").v
    end

    #: (Array[Integer] | String) -> Integer
    def total(xs)
      return xs.size if xs.is_a?(String)

      sum = 0
      xs.each { |x| sum += x }
      sum
    end

    #: (Array[Integer] | Range[Integer]) -> Integer
    def iterate(xs)
      sum = 0
      xs.each { |x| sum += x }
      sum
    end

    def test_iterators
      assert_equal 6, total([1, 2, 3])
      assert_equal 5, total("hello")
      assert_equal 6, iterate([1, 2, 3])
      assert_equal 10, iterate(1..4)
    end

    def test_untyped_boundary
      assert_equal 14, twice(union_untyped(7))
      assert_equal 4, twice(union_untyped("ab"))
      raised = false
      begin
        twice(union_untyped(1.5)) # rugaroo: TypeError at the boundary (decision 20); MRI: NoMethodError inside
      rescue TypeError, NoMethodError
        raised = true
      end
      assert raised
    end

    def test_dynamic_call
      t = Taker.new #: untyped
      assert_equal "5", t.take(5)
      assert_equal "\"x\"", t.take("x")
    end

    #: (bool) -> ([Integer, String] | Integer)
    def pair_or_int(b) = b ? [1, "a"] : 2

    #: (bool) -> (Hash[Symbol, Integer] | String)
    def hash_or_str(b) = b ? { a: 1 } : "none"

    def test_literal_members
      v = pair_or_int(true)
      assert_equal [1, "a"], v
      assert_equal 2, pair_or_int(false)
      assert_equal "[1, \"a\"]", v.inspect
      assert v.is_a?(Array)
      both = [v, 3] #: Array[untyped]
      assert_equal [[1, "a"], 3], both
      assert_equal({ a: 1 }, hash_or_str(true))
      assert_equal "none", hash_or_str(false)
    end

    def test_joins_without_annotations
      x = 3
      a = case x
          when 1 then "one"
          else 0
          end
      assert_equal 0, a
      b = x > 1 ? 1 : "s"
      assert_equal 1, b
      c = x > 5 ? 1 : "s"
      assert_equal "s", c.to_s
      v = 1
      v = "s" if x > 1
      assert_equal "s", v
      w = if x.odd?
            "odd"
          else
            2
          end
      assert_equal "odd", w
      assert_equal 1, union_pick_inferred(true)
      assert_equal "x", union_pick_inferred(false)
      assert_equal 1, union_ident(1)
      assert_equal "a", union_ident("a")
      assert_equal 2, union_half(4)
      assert_in_delta 1.25, union_half(2.5)
      id = ->(z) { z }
      assert_equal 1, id.call(1)
      assert_equal "s", id.call("s")
      total = 0
      total += 1.5
      total += 2
      assert_in_delta 3.5, total
    end

    #: (Integer) -> (Integer | String | nil)
    def maybe(i) = i > 1 ? i : (i.zero? ? nil : "s")

    def test_or_and_on_unions
      y = maybe(0)
      y ||= "default"
      assert_equal "default", y
      z = maybe(5)
      z ||= "d"
      assert_equal 5, z
      assert_in_delta 1.5, maybe(0) || 1.5
      assert_equal :yes, maybe(1) && :yes
      assert_nil maybe(0) && :yes
      m = maybe(7)
      assert_equal "7", m && m.to_s
    end

    def test_union_arguments
      assert_equal ["a", "b;c"], "a,b;c".split(union_sep(true))
      assert_equal ["a,b", "c"], "a,b; c".split(union_sep(false))
      assert_equal "a!b", "a,b".sub(union_sep(true), "!")
      assert_equal 20, 10 * union_num(true)
      assert_in_delta 5.0, 10 * union_num(false)
      assert_in_delta 5.0, 10.0 / union_num(true)
      order = [] #: Array[String]
      pick = ->(s) { order << s; s }
      assert_equal "xy", pick.call("x") + pick.call("y")
      assert_equal %w[x y], order
    end

    #: (Integer | String) -> String
    def routed(x) = x.is_a?(Integer) ? union_int_only(x) : x

    def test_union_argument_needs_narrowing
      assert_equal "int 1", routed(1)
      assert_equal "s", routed("s")
    end

    #: (Integer) -> (Integer | String | Array[Integer] | nil)
    def varied(i)
      case i
      when 0 then nil
      when 1 then 5
      when 2 then "five"
      else [1, 2]
      end
    end

    #: (Integer | String | Array[Integer] | nil) -> String
    def matched(v)
      case v
      in Integer => n then "int #{n + 1}"
      in String => s then "str #{s.upcase}"
      in [a, b] then "pair #{a} #{b}"
      in nil then "nil"
      end
    end

    def test_patterns
      assert_equal ["nil", "int 6", "str FIVE", "pair 1 2"], (0..3).map { |i| matched(varied(i)) }
      assert varied(0).is_a?(NilClass)
      refute varied(1).is_a?(NilClass)
      assert varied(0).nil?
    end

    #: (Integer | bool) -> String
    def flag(x)
      x ? "yes #{x}" : "no"
    end

    def test_boolean_member
      assert_equal "yes 1", flag(1)
      assert_equal "yes true", flag(true)
      assert_equal "no", flag(false)
      b = true #: Integer | bool
      assert b.is_a?(TrueClass)
      refute b.is_a?(FalseClass)
    end

    def test_guard_narrowing
      assert_equal "none", union_guard_nil(nil)
      assert_equal "7", union_guard_nil(7)
      assert_equal "int", union_guard_not(1)
      assert_equal "AB", union_guard_not("ab")
    end

    def test_other_instantiation_argument
      a = [1, 2]
      assert_equal "array 2", union_kind(a)
      assert_equal "string s", union_kind("s")
    end
  end

  # Review fixes to decision 150's calls and conversions.
  class UnionReviewTest < Minitest::Test
    # Arguments of a call on a union are evaluated once, before the type switch.
    def test_argument_assigns_once
      x = [1, "s"].first #: Integer | String
      y = x * (n = 2)
      assert_equal [2, 2], [y, n]
    end

    # Nested arithmetic on a union compiles in time linear in its depth (it was members^depth).
    def test_deep_union_arithmetic
      x = [1, 2.5].first #: Integer | Float
      assert_equal 10, x + (x + (x + (x + (x + (x + (x + (x + (x + x))))))))
      assert_equal 1535, 1 + (1 + (1 + (1 + (1 + (1 + (1 + (1 + (1 + (x * 2) * 2) * 2) * 2) * 2) * 2) * 2) * 2) * 2) * 2
    end

    # A union holding Array[Integer] passed where Array[untyped] is a member is converted, not missed.
    def test_union_into_union_converts
      a = [[1, 2], "s"].first #: Array[Integer] | String
      assert_equal "array 2", union_kind(a)
      xs = [1, 2] #: Array[Integer]?
      assert_equal "array 2", union_kind_opt(xs)
      none = nil #: Array[Integer]?
      assert_equal "nil", union_kind_opt(none)
    end

    def test_generic_into_union
      assert_equal ["int 1", "str a"], [union_generic_tag(1), union_generic_tag("a")]
    end

    # an assignment narrows a union local to the value's class until its next write (decision 283)
    #: (String | Pathname) -> String
    def union_rebase(src)
      src = Pathname(src)
      src = src.dirname / src.basename if src.dirname.to_s != "."
      src.basename.to_s
    end

    #: (Integer | String) -> Integer
    def union_resize(x)
      x = x.to_s
      x.size
    end

    #: (Integer | String, bool) -> String
    def union_branch(x, flip)
      x = "s#{x}" if flip
      x.to_s
    end

    def test_assignment_narrows
      assert_equal ["b", "d", 2, 1], [union_rebase("a/b"), union_rebase(Pathname("d")), union_resize(12), union_resize("x")]
      assert_equal ["s1", "1"], [union_branch(1, true), union_branch(1, false)]
    end

    #: (String | Symbol | nil v) -> String?
    def union_safe_to_s(v) = v&.to_s

    class UnionPipeA
      #: (Array[String] log) -> void
      def initialize(log) = @log = log

      #: () -> nil
      def close
        @log << "a"
        nil
      end
    end

    class UnionPipeB
      #: (Array[String] log) -> void
      def initialize(log) = @log = log
      #: () -> void
      def close = @log << "b"
    end

    # `&.` on a union holding nil is nil for nil, not nil.to_s (Homebrew's Resource#url: `@url&.to_s`)
    def test_safe_navigation_on_nil_union
      assert_equal [nil, "a", "b"], [union_safe_to_s(nil), union_safe_to_s(:a), union_safe_to_s("b")]
      log = [] #: Array[String]
      [UnionPipeA.new(log), nil, UnionPipeB.new(log)].each do |v|
        w = v #: UnionPipeA | UnionPipeB | nil
        w&.close
      end
      assert_equal %w[a b], log
    end
  end
end

# Decision 385: decision 150's follow-ups (#123).
#: (Array[Numeric]) -> Numeric
def union_numeric_sum(xs) = xs.sum

#: (Hash[String, Numeric]) -> Numeric
def union_numeric_values(h) = h.values.sum

#: (Integer) -> ([Integer, Integer] | String | Hash[Symbol, Integer])
def union_shape(i)
  case i
  when 0 then [1, 2]
  when 1 then "s"
  else { k: 3 }
  end
end

#: (bool) -> (Array[Integer] | Integer)
def union_arr_or_int(b) = b ? [3, 4, 5] : 7

module UnionTests
  class UnionAnimal
    def initialize(n) = @n = n
    def say = "animal #{@n}"
  end

  class UnionDog < UnionAnimal
    def say = "dog #{@n}"
  end

  class UnionFollowUpTest < Minitest::Test
    #: (Array[UnionAnimal]) -> Array[String]
    def says(xs) = xs.map(&:say)

    def test_instantiation_into_wider_element_type
      ints = [1, 2, 3]
      assert_equal 6, union_numeric_sum(ints)
      assert_equal 3.5, union_numeric_sum([1.5, 2])
      assert_equal 3, union_numeric_values({ "a" => 1, "b" => 2 })
      assert_equal ["dog 1", "dog 2"], says([UnionDog.new(1), UnionDog.new(2)])
    end

    def test_patterns_on_a_union
      got = (0..2).map do |i|
        case union_shape(i)
        in [a, b] then "pair #{a + b}"
        in { k: Integer => k } then "hash #{k * 2}"
        in String => s then "str #{s}"
        end
      end
      assert_equal ["pair 3", "str s", "hash 6"], got
      found = case union_arr_or_int(true)
              in [*, 4, *post] then post.sum
              in Integer => n then n
              end
      assert_equal 5, found
      begin
        union_shape(1) => [q, r]
        flunk "matched #{q}, #{r}"
      rescue NoMatchingPatternError => e
        assert_equal %("s": "s" does not respond to #deconstruct), e.message
      end
    end

    def test_nested_union_arithmetic
      x = union_num(true)
      y = union_num(false)
      assert_equal(-82.5, 10 * (y + (x * (y - (x + (y * (x + (y * (x - (y + (x * (y - (x + (y * (x + 1)))))))))))))))
    end

    def test_mixed_literals
      h = { a: 1, b: "x" }
      h[:c] = 1.5
      assert_equal({ a: 1, b: "x", c: 1.5 }, h)
      t = [1, "a"]
      assert_equal [false, 2, false], [t.nil?, t.size, t.frozen?]
    end
  end
end

# #127: unions where an ivar or an inferred parameter meets one.
def union_render(v)
  return v if v.is_a?(String)
  return v.to_s if v.is_a?(Integer)

  "?"
end

def union_choose(v, c)
  return "s" if c

  v
end

def union_cell_text(v)
  return v if v.is_a?(String)
  return v.to_s if v.is_a?(Integer)
  return "[#{v.length} items]" if v.is_a?(Array)

  "?"
end

module UnionTests
  class UnionWriterBox
    # @rbs @v: Integer | String
    attr_writer :v

    def initialize = @v = 1
    def show = @v.to_s
  end

  # an unannotated ivar is the join of its writes (decision 15): spinel's bm_poly_cells
  class UnionCell
    def initialize = @value = ""
    def set(v) = @value = v
    def get = @value
  end

  class UnionNilCell
    attr_accessor :value # never called: its writer takes @value's type from set's writes (decision 445)

    def initialize = @value = nil
    def set(v) = @value = v
    def get = @value
  end

  class UnionLeaf
    attr_reader :n

    def initialize(n) = @n = n
  end

  class UnionBase
    def name = "base"
  end

  class UnionDerived < UnionBase
    def name = "derived"
  end

  class UnionHolder
    def initialize
      @leaf = UnionLeaf.new(1)
      @obj = UnionBase.new
      @flag = false
      @total = 0
    end

    def leaf=(x)
      @leaf = x
    end

    def obj=(x)
      @obj = x
    end

    def bump
      @flag = 1
      @total += 1.5
    end

    def leaf
      v = @leaf
      if v.is_a?(UnionLeaf)
        "leaf #{v.n}"
      elsif v.is_a?(String)
        "str #{v}"
      else
        "nil"
      end
    end

    def obj
      case @obj
      when UnionBase then @obj.name
      else @obj.to_s
      end
    end

    def flag = @flag ? "truthy #{@flag}" : "falsy"
    def total = @total
  end

  class UnionIvarTest < Minitest::Test
    def test_untyped_into_union_ivar
      b = UnionWriterBox.new
      assert_equal "1", b.show
      b.v = union_untyped("s")
      assert_equal "s", b.show
      b.v = union_untyped(5)
      assert_equal "5", b.show
    end

    def test_union_argument_types_an_inferred_param
      assert_equal "42", union_render(union_choose(42, false))
      assert_equal "s", union_render(union_choose(7, true))
    end

    def test_ivar_joins_its_writes
      a = UnionCell.new
      a.set("hello")
      b = UnionCell.new
      b.set(42)
      c = UnionCell.new
      c.set([1, 2, 3])
      assert_equal ["hello", "42", "[3 items]"], [a, b, c].map { |x| union_cell_text(x.get) }
      assert_equal "", UnionCell.new.get
      n = UnionNilCell.new
      assert_nil n.get
      n.set(7)
      assert_equal 7, n.get
      n.set("s")
      assert_equal "s", n.get
    end

    #: (Integer) -> UnionLeaf?
    def leaf_or_nil(i) = i > 1 ? UnionLeaf.new(i) : nil

    def test_ivar_join_members
      h = UnionHolder.new
      assert_equal ["leaf 1", "base", "falsy", 0], [h.leaf, h.obj, h.flag, h.total]
      h.leaf = "s"
      h.obj = "o"
      h.bump
      assert_equal ["str s", "o", "truthy 1", 1.5], [h.leaf, h.obj, h.flag, h.total]
      h.leaf = leaf_or_nil(0)
      h.obj = UnionDerived.new
      assert_equal ["nil", "derived"], [h.leaf, h.obj]
      h.leaf = leaf_or_nil(5)
      assert_equal "leaf 5", h.leaf
    end
  end
end
