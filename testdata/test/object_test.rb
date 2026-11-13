# rbs_inline: enabled

require "minitest/autorun"
require "singleton"
require "forwardable"

# Helpers for the checks that were testdata/run/object_bug_class_name_generated_collision.rb.
class User
  #: () -> String
  def to_s = "user"
end

class NewUser
  #: () -> String
  def to_s = "new user"
end

#: (singleton(ObjectTests::NcSquare)) -> void
def make(k)
  k.new(1)
end

# Helpers for the checks that were testdata/run/object_bugs.rb.
# user classes named like runtime helpers (Opt, Ref) must not clash with them
class Opt
  #: () -> String
  def to_s = "opt"
end

class Ref
  #: () -> String
  def to_s = "ref"
end

#: () -> Integer
def lim = ObjectTests::BG_LIMIT

#: (String?) -> String
def bg_kind(s) = s.class.name

# Joins of class objects and of instances (ternary, case), class objects as Hash keys,
# and a subclass reopened without restating its superclass.
#: (bool) -> singleton(ObjectTests::CmShape)
def object_pick(flag) = flag ? ObjectTests::CmSquare : ObjectTests::Tri

#: (Integer) -> ObjectTests::CmShape
def by_num(n)
  case n
  when 0 then ObjectTests::CmShape.new(0)
  when 1 then ObjectTests::CmSquare.new(1)
  else ObjectTests::Tri.new(2)
  end
end

OBJECT_TOP = 99

OBJECT_NS_NAME = "top"

OBJECT_NS_LIMIT = 10

#: (String) -> Integer
def rank_of(name) = ObjectTests::Kinds.const_get(name).rank

# A computed name on CtChild joins its own and inherited constants (Integer + String: untyped);
# same-typed constants join to their type; Struct/Data classes are constants too.
#: (String) -> untyped
def child_const(n) = ObjectTests::CtChild.const_get(n)

#: (Integer, ?Integer) -> Integer
def top(a, b = a + ObjectTests::DA_LIMIT) = a * b

#: (ObjectTests::DaBase) -> Integer
def via_base(b) = b.calc

#: (ObjectTests::InAnimal) -> String
def in_kind(a)
  case a
  when ObjectTests::Puppy then "puppy #{a.bark}"
  when ObjectTests::InDog then "dog #{a.bark}"
  when ObjectTests::Cat then "cat"
  else "other"
  end
end

#: (Class) -> String
def cname(k) = k.name

#: (Module) -> String
def mname(k) = k.name

#: (Integer) -> ObjectTests::MtVec
def vec(n) = ObjectTests::MtVec.new(n)

#: (String) -> String
def object_trace(s)
  ObjectTests::NS_TRACE << "init #{s}"
  s
end

#: (Integer) -> ObjectTests::Coin
def coin(n) = ObjectTests::Coin.new(n)

#: (ObjectTests::OcBase) -> String
def object_show(b)
  "#{b.name} #{b.me.name} #{b.dup_me.name}"
end

#: (ObjectTests::ListNode?) -> Integer
def object_total(n)
  sum = 0
  while n
    sum += n.val
    n = n.nxt
  end
  sum
end

#: (ObjectTests::Tree) -> Integer
def tsum(t) = t.val + t.kids.reduce(0) { |a, k| a + tsum(k) }

# Helpers for the checks that were testdata/run/object_visibility.rb.
#: (Integer) -> String
def top_helper(n) = "top#{n}"

module Kernel
  #: () -> String
  def object_tag = "<#{self.class}>"
end

class NilClass
  def object_tests_kind = :nil
end

class Integer
  def object_tests_kind = :int
end

# Reopened TrueClass/FalseClass/NilClass answer for true, false and nil, ahead of Object's (decision 241).
class Object
  def object_test_present? = !object_test_blank?
  def object_test_blank? = respond_to?(:empty?) ? !!empty? : false
end

class NilClass
  def object_test_present? = false
end

# A nil T? receiver's Object method gets nil itself as self (decision 362), as mspec's `x.should == nil`.
class ObjectNilSelf
  #: (untyped) -> void
  def initialize(v)
    @v = v #: untyped
  end

  #: () -> bool
  def nil_v? = @v.nil? && @v == nil

  #: () -> String
  def v_inspect = @v.inspect
end

class Object
  def object_test_wrap = ObjectNilSelf.new(self)
end

class FalseClass
  def object_test_present? = false
end

module ObjectTests
  # Helpers for the checks that were testdata/run/object_attrs.rb.
  class Account
    attr_reader :owner #: String
    attr_accessor :balance, :limit #: Integer
    attr_writer :note #: String?
    attr_accessor :nick #: String?

    # @rbs @history: Array[Integer]

    # @rbs @cache: String?

    #: (String, ?Integer) -> void
    def initialize(owner, balance = 0)
      @owner = owner
      @balance = balance
      @limit = -100
      @note = nil
      @nick = nil
      @history = []
      @tags = {} #: Hash[Symbol, String]
      @ratio = 0.5
    end

    #: (Integer) -> self
    def deposit(n)
      self.balance = balance + n
      @history << n
      self
    end

    #: (Integer) -> bool
    def withdraw(n)
      return false if balance - n < limit

      self.balance = balance - n
      @history << -n
      true
    end

    #: () -> Array[Integer]
    def history = @history

    #: () -> String
    def note_text = @note || "(none)"

    #: () -> String
    def summary
      @cache ||= "#{owner}:#{balance}"
    end

    #: () -> void
    def reset_cache
      @cache = nil
    end

    #: (Symbol, String) -> void
    def tag(k, v)
      @tags[k] = v
    end

    #: () -> Hash[Symbol, String]
    def tags = @tags

    #: () -> Float
    def ratio = @ratio

    #: () -> String
    def display_name
      n = nick
      return n.upcase if n

      owner
    end
  end

  class Savings < Account
    attr_reader :rate #: Float

    #: (String, Float) -> void
    def initialize(owner, rate)
      super(owner, 100)
      @rate = rate
      @limit = 0
    end

    #: () -> Integer
    def interest = (balance.to_f * rate).floor

    #: () -> Integer
    def first_deposit = @history.first(1).fetch(0)
  end

  # Optional attrs narrowed by `unless`/`if`/ternary/`&.`; a setter drops the narrowing;
  # an ivar first assigned outside initialize; a lazy `@x ||=`; an ivar updated inside a block.
  class Profile
    attr_accessor :nick #: String?
    attr_reader :parent #: Profile?

    #: (String?, ?Profile?) -> void
    def initialize(nick, parent = nil)
      @nick = nick
      @parent = parent
    end

    #: () -> String
    def shout
      return "anon" unless nick

      nick.upcase
    end

    #: () -> String
    def lineage
      if parent
        "#{nick || "?"} < #{parent.lineage}"
      else
        nick || "root"
      end
    end

    #: () -> Integer
    def depth = parent ? parent.depth + 1 : 0

    #: () -> String
    def parent_nick = parent&.nick || "none"

    #: () -> String
    def reset
      return "none" unless nick

      first = nick.upcase
      self.nick = nil
      "#{first} then #{nick.inspect}"
    end
  end

  class Lazy
    #: () -> Array[Integer]
    def data
      @data ||= [1, 2]
    end

    #: () -> void
    def load
      @loaded = "yes"
      @sum = 0
      data.each { |x| @sum += x }
    end

    #: () -> String
    def loaded = "#{@loaded} #{@sum}"
  end

  # A def after attr_accessor replaces its reader; a hand-written writer replaces attr's;
  # a subclass restates an inherited attr_reader with the same type.
  class AtPerson
    attr_accessor :name #: String
    attr_reader :age #: Integer

    #: (String, Integer) -> void
    def initialize(name, age)
      @name = name
      @age = age
    end

    #: () -> String
    def name = @name.capitalize

    #: (Integer) -> void
    def age=(v)
      @age = v < 0 ? 0 : v
    end
  end

  class AtKid < AtPerson
    attr_reader :age #: Integer

    #: () -> String
    def info = "#{name}/#{age}"
  end

  # Helpers for the checks that were testdata/run/object_bug_attr_op_assign.rb.
  class AoCounter
    attr_accessor :n #: Integer
    attr_accessor :label #: String?

    #: () -> void
    def initialize
      @n = 0
      @label = nil
    end

    #: (Integer) -> void
    def drop(k)
      self.n -= k
    end
  end

  AoPoint = Struct.new(:x, :y) #: [Integer, Integer]

  class Shape
    #: () -> String
    def to_s = "shape"
  end

  class ShapeI
    #: () -> String
    def to_s = "shape i"
  end

  module Foo
    class Bar
      #: () -> String
      def to_s = "Foo::Bar"
    end
  end

  class Foo_Bar
    #: () -> String
    def to_s = "Foo_Bar"
  end

  # Helpers for the checks that were testdata/run/object_bug_comparable_struct.rb.
  class Version
    include Comparable

    attr_reader :major #: Integer
    attr_reader :minor #: Integer

    #: (Integer, Integer) -> void
    def initialize(major, minor)
      @major = major
      @minor = minor
    end

    def <=>(other)
      c = major <=> other.major
      return c unless c == 0

      minor <=> other.minor
    end

    #: () -> String
    def to_s = "v#{major}.#{minor}"
  end

  # Helpers for the checks that were testdata/run/object_bug_const_get_messages.rb.
  module CgM
  end

  # Helpers for the checks that were testdata/run/object_bug_hook_methods.rb.
  HK_LOG = [] #: Array[String]

  class Plugin
    #: (untyped) -> void
    def self.inherited(sub)
      HK_LOG << "inherited by #{sub}"
    end
  end

  class Alpha < Plugin
  end

  module Tracked
    #: (untyped) -> void
    def self.included(base)
      HK_LOG << "included in #{base}"
    end

    #: (untyped) -> void
    def self.extended(base)
      HK_LOG << "extended #{base}"
    end
  end

  class HkThing
    include Tracked
  end

  class HkOther
    extend Tracked
  end

  # Helpers for the checks that were testdata/run/object_bug_module_super.rb.
  module MsLoud
    #: () -> String
    def hi = "loud " + super
  end

  class MsA
    #: () -> String
    def hi = "a"
  end

  class MsB < MsA
    include MsLoud
  end

  # The target follows the module in each includer's ancestors: another
  # module, the includer's superclass, or nothing (NoMethodError).
  MS_LOG = [] #: Array[String]

  module Shout
    #: (Integer) -> Integer
    def calc(x) = super(x + 1) * 2

    #: (Integer) -> void
    def initialize(n)
      super
      MS_LOG << "shout init #{n}"
    end

    #: () -> String
    def hi = "shout " + super
  end

  module Quiet
    #: () -> String
    def hi = "quiet " + super
  end

  module MsBoth
    include Shout

    #: () -> String
    def hi = "both " + super
  end

  class P
    #: (Integer) -> void
    def initialize(n)
      @n = n
      MS_LOG << "p init #{n}"
    end

    #: () -> String
    def hi = "p#{@n}"

    #: (Integer) -> Integer
    def calc(x) = x + 100
  end

  class MsQ < P
    include Shout
  end

  class R < MsQ
    #: () -> String
    def hi = "r " + super
  end

  class S < P
    include Shout
    include Quiet
  end

  class T < P
    include MsBoth
  end

  class U
    include Quiet
  end

  # Helpers for the checks that were testdata/run/object_bug_narrowed_call_stmt.rb.
  class NcBuilder
    #: () -> void
    def initialize
      @parts = [] #: Array[String]
    end

    #: (String) -> self
    def add(part)
      @parts << part
      self
    end

    #: () -> String
    def build = @parts.join("-")
  end

  class NcHtmlBuilder < NcBuilder
  end

  module NcPlugins
    VERSION = "1.0"

    class Alpha
    end
  end

  NC_LOG = [] #: Array[String]

  class NcShape
    #: (Integer) -> void
    def initialize(n)
      NC_LOG << "made #{n}"
    end
  end

  class NcSquare < NcShape
  end

  # lim runs before BG_LIMIT is assigned
  BG_LIM_ERR = begin
    lim.to_s
  rescue NameError => e
    "NameError: #{e.message}"
  end

  BG_LIMIT = 5

  # const_get on an optional module constant keeps its nil
  module Settings
    DEFAULT = nil #: String?
  end

  # const_get through a non-module value raises TypeError
  module BgPlugins
    VERSION = "1.0"
  end

  # Data#with with no args returns self; with args, a new object
  Coord = Data.define(:lat, :lng) #: [Float, Float]

  # frozen? on a plain object, a Struct and a Data
  class BgPlain
  end

  BgPair = Struct.new(:a, :b) #: [Integer, Integer]

  BgVal = Data.define(:v) #: [Integer]

  # is_a? on a local used nowhere else
  class BgAnimal
  end

  class BgDog < BgAnimal
  end

  # constants/const_get/const_defined? see an included module's constants
  module BgConfig
    BG_LIMIT = 5
  end

  class BgUses
    include BgConfig

    OWN = 1
  end

  # optional constants at top level and in a module
  BG_NAME = "n" #: String?

  COUNT = 3 #: Integer?

  module Labels
    LABEL = "l" #: String?
  end

  # Struct/Data members named like keywords (begin, end, in, out)
  Span = Struct.new(:begin, :end) #: [Integer, Integer]

  Link = Data.define(:in, :out) #: [String, String]

  # a member named `other` does not shadow ==(other)
  Edge = Struct.new(:from, :other) #: [String, String]

  LinkOther = Data.define(:other) #: [Integer]

  class TapBox
    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end
  end

  SG_LOG = [] #: Array[String]

  class SgConfig
    include Singleton
    attr_accessor :level #: Integer

    #: () -> void
    def initialize
      @level = 1
      SG_LOG << "init"
    end
  end

  module SgApp
    class Registry
      include Singleton

      #: () -> void
      def initialize
        @names = [] #: Array[String]
      end

      #: (String) -> Array[String]
      def add(n) = @names << n
    end
  end

  # Helpers for the checks that were testdata/run/object_class_methods.rb.
  class CmShape
    attr_reader :size #: Integer

    #: (Integer) -> void
    def initialize(size)
      @size = size
    end

    #: () -> String
    def self.kind = "shape"

    #: () -> Integer
    def self.sides = 0

    #: () -> String
    def self.info = "#{kind}/#{sides}"

    #: (Integer) -> CmShape
    def self.build(n) = new(n + 1)

    #: (Integer) -> CmShape
    def self.build_explicit(n) = self.new(n * 2)

    #: () -> String
    def describe = "#{self.class.kind}:#{self.class.name}(#{size}) sides=#{self.class.sides}"

    #: () -> CmShape
    def grow = self.class.new(size + 100)
  end

  class CmSquare < CmShape
    def self.kind = "square"

    def self.sides = 4

    def self.info = "sq " + super
  end

  class Tri < CmShape
    def self.sides = 3
  end

  class Tiny < CmSquare
    #: () -> void
    def initialize = super(1)

    def self.kind = "tiny"
  end

  class CmRegistry
    #: () -> Integer
    def self.count
      @count ||= 0
    end

    #: () -> void
    def self.bump
      @count = count + 1
    end

    #: (String) -> String
    def self.fmt(s) = "<#{s}>"
  end

  class SubRegistry < CmRegistry
    def self.fmt(s) = super(s.upcase) + "!"
  end

  class Color
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    RED = Color.new("red")
    ALL = [RED, Color.new("blue")] #: Array[Color]

    #: () -> Color
    def self.default = RED

    #: (String) -> String
    def self.fmt(s) = "<#{s}>"
  end

  class Shade < Color
    #: (String, ?Integer) -> void
    def initialize(name, depth = 3)
      super(name)
      @depth = depth
    end

    def self.fmt(s) = super + "!"

    #: () -> String
    def self.plain = fmt("p")

    #: () -> Integer
    def depth = @depth
  end

  class CmDeep < Shade
    def initialize(name, depth = 9) = super
  end

  class Color
    #: () -> String
    def shout = name.upcase
  end

  class CmSquare
    #: () -> String
    def self.extra = "reopened #{kind}"

    #: () -> Integer
    def area = size * size
  end

  # A module's own class-level ivar; `self` in a class method is the (virtual) class object;
  # a class referenced before its definition; a constant holding class objects.
  module Tally
    #: () -> Integer
    def self.count
      @count ||= 0
    end

    #: () -> void
    def self.bump
      @count = count + 1
    end
  end

  class CmShape
    #: () -> singleton(CmShape)
    def self.me = self

    #: () -> Maker
    def maker = Maker.new(self)
  end

  class Maker
    #: (CmShape) -> void
    def initialize(s)
      @s = s
    end

    #: () -> String
    def made = "made #{@s.class.kind} #{@s.size}"
  end

  HANDLERS = [CmShape, CmSquare, Tiny]

  # Class methods taking value blocks (not iterators), with a default argument.
  class Conf
    attr_accessor :port #: Integer

    #: () -> void
    def initialize
      @port = 80
    end

    #: () { (Conf) -> void } -> Conf
    def self.build
      c = new
      yield c
      c
    end

    #: (?Integer) { (Integer) -> String } -> String
    def self.fmt(n = 1) = yield(n * 2)
  end

  # An explicit `< Object`; class-body constants built by a qualified class-method call and
  # from earlier constants, in source order.
  class CmTemp < Object
    attr_reader :deg #: Integer

    #: (Integer) -> void
    def initialize(deg)
      @deg = deg
    end

    #: (Integer) -> CmTemp
    def self.of(d) = new(d)

    FREEZING = CmTemp.of(0)
    BOILING = CmTemp.new(100)
    RANGE = BOILING.deg - FREEZING.deg
  end

  # A registry of class objects in a constant; an ivar holding a class object (a factory);
  # a module's to_s used by puts and interpolation.
  class Handler
    REGISTRY = {} #: Hash[String, singleton(Handler)]

    #: () -> String
    def self.tag = "base"

    #: (String, singleton(Handler)) -> void
    def self.register(name, k)
      REGISTRY[name] = k
    end

    #: (String) -> String
    def self.dispatch(name)
      k = REGISTRY[name]
      return "none" unless k

      k.new.run
    end

    #: () -> String
    def run = "run #{self.class.tag}"
  end

  class Posts < Handler
    def self.tag = "posts"
  end

  class Users < Handler
    def self.tag = "users"
  end

  class Factory
    #: (singleton(Handler)) -> void
    def initialize(klass)
      @klass = klass
    end

    #: () -> Handler
    def make = @klass.new

    #: () -> String
    def kind = @klass.tag
  end

  module Pretty
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def to_s = "pretty #{title}"
  end

  class CmDoc
    include Pretty

    #: () -> String
    def title = "d"
  end

  # respond_to? on class objects: class methods, subclass-only class methods through singleton(C), new, name.
  class Blob
    #: () -> String
    def self.kind = "blob"

    #: () -> String
    def area = "a"
  end

  class SubBlob < Blob
    #: () -> String
    def self.only_sub = "sub"
  end

  # A class's own name/to_s override: to_s and inspect do not call a custom name.
  class CmWidget
    #: () -> String
    def self.name = "CustomWidget"
  end

  class CmGadget
    #: () -> String
    def self.to_s = "GadgetClass"
  end

  # Helpers for the checks that were testdata/run/object_class_new_arity.rb.
  class NaBase
    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: () -> Integer
    def n = @n
  end

  class Two < NaBase
    #: (Integer, Integer) -> void
    def initialize(a, b)
      super(a + b)
    end
  end

  class Same < NaBase
  end

  # Helpers for the checks that were testdata/run/object_cmp_nil.rb.
  class Score
    attr_reader :v #: Float

    #: (Float) -> void
    def initialize(v)
      @v = v
    end

    #: (Score) -> Integer?
    def <=>(other) = v <=> other.v

    #: () -> String
    def to_s = "S#{v}"
  end

  class Bonus < Score
  end

  # Helpers for the checks that were testdata/run/object_constants.rb.
  module CtPlugins
    VERSION = "1.0"
    MAX = 3

    class Base
      #: () -> String
      def self.id = "base"

      #: () -> String
      def run = "base run"
    end

    class Alpha < Base
      def self.id = "alpha"

      def run = "alpha run"
    end

    class Beta < Base
      def self.id = "beta"
    end
  end

  module Kinds
    class Root
      #: () -> Integer
      def self.rank = 0
    end

    class One < Root
      def self.rank = 1
    end

    class Two < Root
      def self.rank = 2
    end
  end

  class CtParent
    COLOR = "red"
  end

  class CtChild < CtParent
    SIZE = 5
  end

  module Limits
    LOW = 1
    HIGH = 9
  end

  module CtGeo
    Coord = Data.define(:lat, :lng) #: [Float, Float]
    Tag = Struct.new(:text) #: [String]
    ORIGIN = Coord.new(0.0, 0.0)
  end

  # Helpers for the checks that were testdata/run/object_default_args_callee.rb.
  DA_LIMIT = 3

  module Greets
    #: (?String) -> String
    def hello(who = default_who) = "hello #{who}"

    #: () -> String
    def default_who = "module"
  end

  class DaBase
    include Greets

    #: () -> void
    def initialize
      @n = 10
    end

    #: (?Integer, ?Integer) -> Integer
    def calc(a = @n, b = a * 2) = a + b

    #: (Integer, ?Integer, *Integer) -> Integer
    def sum(first, second = first + 1, *rest) = first + second + rest.size

    #: (?Integer) { (Integer) -> void } -> void
    def times_up(n = DA_LIMIT)
      n.times { |i| yield i }
    end
  end

  class DaKid < DaBase
    #: (?Integer, ?Integer) -> Integer
    def calc(a = 1, b = 2) = super() * 100 + a + b

    #: () -> String
    def default_who = "kid"
  end

  class DaBox
    #: (Integer, ?Integer) -> void
    def initialize(w, h = w + 1)
      @w = w
      @h = h
    end

    #: () -> Integer
    def area = @w * @h
  end

  class DaShape
    #: (?String) -> String
    def self.make(kind = default_kind) = "made #{kind}"

    #: () -> String
    def self.default_kind = "shape"
  end

  class Circle < DaShape
    #: () -> String
    def self.default_kind = "circle"
  end

  class Log
    #: () -> void
    def initialize
      @lines = [] #: Array[String]
    end

    #: (String) -> String
    def note(s)
      @lines << s
      s
    end

    # the default is never read, but runs for its side effect
    #: (String, ?String) -> String
    def f(a, b = note("side")) = a

    #: () -> Integer
    def count = @lines.size
  end

  # Helpers for the checks that were testdata/run/object_extend_block.rb.
  module Describable
    #: (Integer) -> Integer
    def twice(n) = n * 2

    #: () -> String
    def describe = "I am #{name}"
  end

  class EbWidget
    extend Describable

    #: () -> String
    def self.kind = "widget"
  end

  class EbGadget < EbWidget
  end

  module Tools
    extend Describable
  end

  module EbColors
    extend Enumerable #[String]

    RED = "red"
    GREEN = "green"
    BLUE = "blue"

    #: () { (String) -> void } -> void
    def self.each(&block)
      [RED, GREEN, BLUE].each(&block)
    end
  end

  module Handlers
    extend Enumerable #[singleton(Handlers::Base)]

    class Base
      #: (String) -> bool
      def self.matches?(path) = false
    end

    class Posts < Base
      def self.matches?(path) = path.start_with?("/posts")
    end

    class Users < Base
      def self.matches?(path) = path.start_with?("/users")
    end

    #: () { (singleton(Base)) -> void } -> void
    def self.each(&block)
      constants.sort.map { |c| const_get(c) }.each(&block)
    end
  end

  class EbBag
    #: () -> void
    def initialize
      @items = [] #: Array[Integer]
    end

    #: (Integer) -> self
    def add(n)
      @items << n
      self
    end

    #: () { (Integer) -> void } -> void
    def each(&block) = @items.each(&block)

    #: () { (Integer) -> Integer } -> Array[Integer]
    def transform(&block) = @items.map(&block)

    #: () { (Integer) -> bool } -> Array[Integer]
    def keep(&block) = @items.select(&block)

    #: () { (Integer) -> String } -> String
    def first_as(&block) = block.call(@items.first(1).fetch(0))

    #: () { (Integer) -> void } -> void
    def each_twice(&block)
      each(&block)
      each(&block)
    end
  end

  # A class method overrides a method its class got from `extend`; the extended method
  # dispatches back to the override, per subclass, also through singleton(C).
  module Labelled
    #: () -> String
    def label = "#{name}:#{kind}"

    #: () -> String
    def kind = "base"
  end

  class Knob
    extend Labelled

    def self.kind = "knob"
  end

  class BigKnob < Knob
    def self.kind = "big"
  end

  class Lever
    extend Labelled
  end

  # Helpers for the checks that were testdata/run/object_inheritance.rb.
  class InBase
    attr_reader :tag #: String

    #: (String) -> void
    def initialize(tag)
      @tag = tag
    end

    #: () -> String
    def name = "InBase"

    #: () -> Integer
    def weight = 1

    #: () -> String
    def describe = "#{name}[#{tag}] w=#{weight}"

    #: (Integer) -> Integer
    def scale(n) = n * weight

    #: (String, ?String) -> String
    def greet(who, punct = "!") = "hi #{who}#{punct}"

    #: () -> String
    def to_s = "<#{name} #{tag}>"
  end

  class Middle < InBase
    def name = "Middle"

    def weight = super * 10

    def scale(n) = super(n) + 1

    def greet(who, punct = "?") = super.upcase
  end

  class InLeaf < Middle
    #: (String, Integer) -> void
    def initialize(tag, extra)
      super(tag)
      @extra = extra
    end

    def name = "InLeaf#{@extra}"

    def weight = super + @extra

    def describe = "leaf: #{super}"

    #: () -> Integer
    def extra = @extra
  end

  class Empty < InBase
  end

  class Fixed < InBase
    #: () -> void
    def initialize = super("fixed")
  end

  class Root
    #: () -> void
    def initialize
      super
      @n = 7 #: Integer
    end

    #: () -> Integer
    def n = @n
  end

  class Summer
    SCALE = 2

    #: (*Integer) -> void
    def initialize(*nums)
      @nums = nums
    end

    #: () -> Integer
    def total = @nums.reduce(0) { |a, b| a + b }

    #: () -> Integer
    def scaled = total * SCALE
  end

  class KidSummer < Summer
    SCALE = 5

    def initialize(*nums) = super

    #: () -> Integer
    def kid_scaled = total * SCALE
  end

  class InAnimal
    #: () -> String
    def name = "animal"
  end

  class InDog < InAnimal
    #: () -> String
    def bark = "woof"
  end

  class Puppy < InDog
  end

  class Cat < InAnimal
  end

  # zsuper passes the parameters' current values; super() passes none; super inside a block.
  class InCalc
    #: (Integer, ?Integer) -> Integer
    def compute(n, k = 10) = n * k

    #: () -> String
    def plain = "calc"

    #: () -> Array[String]
    def parts = ["a", "b"]

    #: (String) -> String
    def tag(s) = "<#{s}>"
  end

  class InKidCalc < InCalc
    def compute(n, k = 3)
      n += 1
      k *= 2
      super
    end

    def plain = super() + "!"

    def parts = super.map { |p| tag(p) + plain }

    def tag(s) = [1, 2].map { |i| super(s * i) }.join
  end

  class GrandCalc < InKidCalc
    def compute(n, k = 1) = super(n) + super(n, k)
  end

  # Three-level initialize chain with defaults; sibling subclasses with same-named ivars of different types.
  class InNode
    attr_reader :id #: Integer

    #: (?Integer) -> void
    def initialize(id = 1)
      @id = id
    end
  end

  class InNamed < InNode
    attr_reader :label #: String

    #: (String, ?Integer) -> void
    def initialize(label, id = 2)
      super(id)
      @label = label
    end
  end

  class Leafy < InNamed
    #: () -> void
    def initialize
      super("leafy")
    end
  end

  class IntBox < InNode
    #: () -> void
    def initialize
      super()
      @v = 41
    end

    #: () -> Integer
    def v = @v + 1
  end

  class StrBox < InNode
    #: () -> void
    def initialize
      super(9)
      @v = "s"
    end

    #: () -> String
    def v = @v * 2
  end

  # The abstract-method idiom: raise NotImplementedError in the base, which a bare rescue does not catch.
  class Figure
    #: () -> Integer
    def area = raise(NotImplementedError, "#{self.class.name}#area")

    #: () -> String
    def show = "area #{area}"
  end

  class Sq < Figure
    def area = 4
  end

  # An inherited `-> self` method keeps the subclass type through a chain, a local, and an array.
  class InBuilder
    #: () -> void
    def initialize
      @parts = [] #: Array[String]
    end

    #: (String) -> self
    def add(part)
      @parts << part
      self
    end

    #: () -> String
    def build = @parts.join("-")
  end

  class InHtmlBuilder < InBuilder
    #: () -> String
    def html = "<p>#{build}</p>"
  end

  # Helpers for the checks that were testdata/run/object_mid.rb.
  # an attr writer call's value is the assigned value, not the setter's return
  class MdBox
    attr_accessor :n #: Integer

    #: () -> void
    def initialize
      @n = 0
    end
  end

  # a class body can call its own class methods and new while defining constants
  class MdTemp
    attr_reader :deg #: Integer

    #: (Integer) -> void
    def initialize(deg)
      @deg = deg
    end

    #: (Integer) -> MdTemp
    def self.of(d) = new(d)

    FREEZING = of(0)
    BOILING = new(100)
  end

  # yielding class methods on a module and a class iterate from a call site
  module MdColors
    #: () { (String) -> void } -> void
    def self.each
      yield "red"
      yield "green"
    end
  end

  class MdCounter
    #: (Integer) { (Integer) -> void } -> void
    def self.upto(n)
      i = 0
      while i < n
        yield i
        i += 1
      end
    end
  end

  # inherit=false hides a superclass constant from constants/const_defined?/const_get
  class MdParent
    COLOR = "red"
  end

  class MdChild < MdParent
    SIZE = 5
  end

  # MdChild::X resolves constants and nested classes through the superclass
  class PathParent
    COLOR = "red"

    class Nested
      #: () -> String
      def hi = "nested"
    end
  end

  class PathChild < PathParent
    SIZE = 5
  end

  # a struct-class argument to generic then/reduce/include?, and singleton arrays
  class MdVec
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end

    #: (MdVec) -> MdVec
    def plus(o) = MdVec.new(v + o.v)
  end

  class Vec3 < MdVec
  end

  # user-defined self.name/to_s/inspect are inherited by subclasses
  class MdWidget
    #: () -> String
    def self.name = "CustomWidget"
  end

  class SubWidget < MdWidget
  end

  class MdGadget
    #: () -> String
    def self.to_s = "GadgetClass"

    #: () -> String
    def self.inspect = "GadgetInspect"
  end

  class SubGadget < MdGadget
  end

  # is_a?(Module) is false for a class that neither includes it nor has subclasses
  module Walker
  end

  class MdDoc
  end

  class Page
    include Walker
  end

  # an ivar assignment as a method's last expression is its return value
  class MdMemo
    # @rbs @last: Integer?

    #: () -> void
    def initialize
      @last = nil
    end

    #: (Integer) -> Integer
    def store(n)
      @last = n * 2
    end

    #: () -> Integer?
    def last = @last
  end

  # `@x ||= [] #: T` types the ivar from the trailing annotation
  class ItemBox
    #: () -> Array[String]
    def items
      @items ||= [] #: Array[String]
    end
  end

  class Reg
    #: () -> Hash[String, Integer]
    def self.table
      @table ||= {} #: Hash[String, Integer]
    end
  end

  # a local declared as the superclass can be reassigned a subclass instance or class
  class MdBase
    #: () -> String
    def name = "MdBase"
  end

  class MdLeaf < MdBase
    def name = "MdLeaf"
  end

  # a method named like its class (calc/MdCalc, node/MdNode) survives subclassing
  class MdCalc
    #: (Integer) -> Integer
    def calc(n) = n * 2
  end

  class MdKidCalc < MdCalc
    def calc(n) = super + 1
  end

  class MdNode
    attr_reader :node #: String

    #: () -> void
    def initialize
      @node = "node"
    end
  end

  class NodeLeaf < MdNode
  end

  # an included module's constant resolves unqualified in the class and its subclasses
  module MdConfig
    LIMIT = 5
  end

  class MdUses
    include MdConfig

    #: () -> Integer
    def lim = LIMIT
  end

  class MdSub < MdUses
    #: () -> Integer
    def twice = LIMIT * 2
  end

  # Helpers for the checks that were testdata/run/object_mid2.rb.
  # self.class inside a module method names the including object's class
  module Describe
    #: () -> String
    def kind = self.class.name

    #: () -> String
    def label = "#{self.class}!"
  end

  class MtFoo
    include Describe
  end

  class Bar < MtFoo
  end

  # a module as an element type dispatches to each includer's override
  module Printable
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def to_s = "P(#{title})"
  end

  class MtDoc
    include Printable

    #: () -> String
    def title = "doc"
  end

  class MtMemo
    include Printable

    #: () -> String
    def title = "memo"
  end

  # != uses a user-defined ==, and Struct's generated one
  class NeVec
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (NeVec) -> bool
    def ==(o) = x == o.x
  end

  NePoint = Struct.new(:x, :y) #: [Integer, Integer]

  # a subclass may override with a different arity and a narrower return type
  class MtBase
    #: (Integer) -> String
    def f(n) = "base #{n}"

    #: () -> MtBase
    def me = self

    #: () -> String
    def name = "base"
  end

  class MtSub < MtBase
    #: () -> String
    def f = "sub"

    #: () -> MtSub
    def me = self

    def name = "sub"

    #: () -> String
    def only = "only"
  end

  # private attr_reader/attr_accessor are callable on self, including self.x =
  class MtVault
    #: () -> void
    def initialize
      @pin = 1234
      @hits = 0
    end

    #: () -> String
    def open
      self.hits = hits + 1
      "pin has #{pin.to_s.size} digits, hit #{hits}"
    end

    private

    attr_reader :pin #: Integer
    attr_accessor :hits #: Integer
  end

  # a bare `private` does not reach `def self.x`
  class PrivCounter
    #: () -> String
    def self.shown = "shown"

    private

    #: () -> String
    def self.hidden = "still public"

    #: () -> String
    def inst = "private instance"
  end

  # reopening with `< ::MtA` or `class MtM::K < MtM::MtBase` is the same superclass
  class MtA
  end

  class MtC < MtA
    #: () -> String
    def one = "one"
  end

  class MtC < ::ObjectTests::MtA
    #: () -> String
    def two = "two"
  end

  module MtM
    class MtBase
    end

    class K < MtBase
    end
  end

  class MtM::K < MtM::MtBase
    #: () -> String
    def three = "three"
  end

  # a singleton(T) value converts to Class/Module parameters and Hash[Class, _] keys
  class MtShape
  end

  class MtSquare < MtShape
  end

  # a Struct.new block can override initialize/inspect and call super
  MtPoint = Struct.new(:x, :y) do
    #: (Integer, ?Integer) -> void
    def initialize(x, y = 0)
      super(x, y)
    end

    #: () -> Integer
    def sum = x + y
  end #: [Integer, Integer]

  MtPair = Struct.new(:a, :b) do
    #: () -> String
    def inspect = "P" + super
  end #: [Integer, Integer]

  # Struct/Data == is false across a subclass, true within one
  EqPoint = Struct.new(:x, :y) #: [Integer, Integer]

  class EqPoint3 < EqPoint
  end

  MtVal = Data.define(:v) #: [Integer]

  class SubVal < MtVal
  end

  # super inside an overridden attr reader/writer reaches the parent's attr
  class AttrBase
    attr_accessor :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end
  end

  class MtLoud < AttrBase
    def name = super.upcase

    def name=(v)
      super(v + "!")
    end
  end

  # include?/== on collections use a user-defined typed ==
  class MtVec
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (MtVec) -> bool
    def ==(o) = x == o.x
  end

  # zsuper from a no-arg initialize lets the parent apply its own default
  class MtNode
    attr_reader :id #: Integer

    #: (?Integer) -> void
    def initialize(id = 1)
      @id = id
    end
  end

  class MtBox < MtNode
    #: () -> void
    def initialize
      super
    end
  end

  # Method-name tables (decision 77): public_instance_methods/instance_methods
  # list public methods of the class and its ancestors, short of Object.
  module PimShared
    def test_shared = 1
  end

  class PimBase
    def test_base = 1
    def helper = 2

    private

    def test_private = 3
  end

  class PimChild < PimBase
    include PimShared

    def test_child = 4
    def ==(other) = true
  end

  # Helpers for the checks that were testdata/run/object_modules.rb.
  module MoNamed
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def label = "[#{title}]"

    #: () -> String
    def shout = label.upcase
  end

  module Tagged
    #: () -> String
    def label = "tag:#{title}"

    #: () -> Integer
    def tag_len = title.size
  end

  module Greeter
    include MoNamed

    #: (String) -> String
    def greet(other) = "#{label} greets #{other}"
  end

  class MoDoc
    include MoNamed

    attr_reader :title #: String

    #: (String) -> void
    def initialize(title)
      @title = title
    end
  end

  class MoBoth
    include MoNamed
    include Tagged

    #: () -> String
    def title = "both"
  end

  class Override
    include MoNamed

    #: () -> String
    def title = "ov"

    def label = "<#{super}>"
  end

  class MoPerson
    include Greeter

    #: () -> String
    def title = "ann"
  end

  module MoLoud
    #: () -> String
    def hi = "loud"
  end

  class MoA
    #: () -> String
    def hi = "a"

    #: () -> String
    def base_only = "base"
  end

  class MoB < MoA
    include MoLoud
  end

  class MoC < MoB
    def hi = "c/" + super
  end

  class MoQ < MoDoc
  end

  module MoRegistry
    extend Enumerable #[String]

    #: () { (String) -> void } -> void
    def self.each
      yield "alpha"
      yield "beta"
      yield "gamma"
    end
  end

  module Util
    #: (Integer) -> Integer
    def self.double(n) = n * 2

    #: (Integer) -> Integer
    def self.quad(n) = double(double(n))
  end

  # MoA module's own constants resolve lexically inside its methods; `-> self`, iterator,
  # &block-forwarding and private methods in a module, reached from an includer and its subclass.
  module Greeting
    PREFIX = "hi"

    #: () -> String
    def name = raise(NotImplementedError)

    #: () -> String
    def greet = "#{PREFIX} #{name}#{suffix}"

    #: () -> self
    def touch = self

    #: () { (Integer) -> void } -> void
    def each_two
      yield 1
      yield 2
    end

    #: (Integer) { (Integer) -> Integer } -> Array[Integer]
    def mapped(n, &blk) = [n, n + 1].map(&blk)

    private

    #: () -> String
    def suffix = "!"
  end

  class Member
    include Greeting

    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def loud = greet.upcase + suffix
  end

  class Junior < Member
    def name = "jr"
  end

  # Helpers for the checks that were testdata/run/object_namespaces.rb.
  # NS_TRACE records constant init order against the surrounding top-level code.
  NS_TRACE = ["start"] #: Array[String]

  FIRST = object_trace("first")

  module Outer
    OBJECT_NS_NAME = "outer"
    SECOND = object_trace("second")
    LIST = [1, 2, 3]
    TABLE = { "a" => 1 }
    EMPTY = [] #: Array[String]

    module Inner
      DEPTH = 2

      class Leaf
        #: () -> String
        def where = "#{OBJECT_NS_NAME} #{DEPTH} #{OBJECT_NS_LIMIT} #{SECOND}"

        #: () -> String
        def top = ::OBJECT_NS_NAME
      end
    end

    class Base
      SCALE = 3

      #: () -> Integer
      def scale = SCALE
    end

    class Derived < Base
      #: () -> Integer
      def doubled = SCALE * 2

      #: () -> Integer
      def list_sum = LIST.reduce(0) { |a, b| a + b }
    end
  end
end

ObjectTests::NS_TRACE << "middle"

module ObjectTests
  class Outer::Compact
    #: () -> String
    def name_seen = OBJECT_NS_NAME

    #: () -> String
    def qualified = Outer::OBJECT_NS_NAME
  end

  class NsOther < Outer::Base
    #: () -> Integer
    def inherited_scale = SCALE + 1
  end

  module Outer
    THIRD = object_trace("third")

    class Vec
      attr_reader :v #: Integer

      #: (Integer) -> void
      def initialize(v)
        @v = v
      end

      #: (Vec) -> Vec
      def plus(o) = Vec.new(v + o.v)
    end

    module Factory
      #: () -> Array[Vec]
      def self.make = [Vec.new(1), Vec.new(-2)]

      #: (singleton(Base)) -> Base
      def self.build(k) = k.new
    end

    ORIGIN = Inner::Leaf.new #: Inner::Leaf

    class Compact
      #: () -> String
      def reopened = OBJECT_NS_NAME
    end
  end

  # Classes nested in a class and inheriting from it; a nested constant shadowing the outer one;
  # relative paths (Config::PORT) resolved from a sibling module's lexical scope.
  class Expr
    PRECEDENCE = 0

    #: () -> String
    def show = "?"

    class Num < Expr
      attr_reader :v #: Integer

      #: (Integer) -> void
      def initialize(v)
        @v = v
      end

      def show = v.to_s
    end

    class Add < Expr
      PRECEDENCE = 1

      #: (Expr, Expr) -> void
      def initialize(l, r)
        @l = l
        @r = r
      end

      def show = "(#{@l.show} + #{@r.show})"

      #: () -> Integer
      def prec = PRECEDENCE

      #: (Integer, Integer) -> Add
      def self.of(a, b) = new(Num.new(a), Num.new(b))
    end

    #: () -> Integer
    def prec = PRECEDENCE
  end

  module App
    module Config
      PORT = 80
    end

    module Server
      #: () -> Integer
      def self.port = Config::PORT + 1

      class Handler
        #: () -> String
        def where = "#{Config::PORT}/#{Server.port}"
      end
    end
  end

  # RBS names in annotations resolve like constants: through the superclass (Item, OBJECT_NS_LIMIT from Holder)
  # and lexically outward (Vec is Outer::Vec inside Outer::Inner).
  class Holder
    class Item
      #: () -> String
      def hi = "item"
    end

    OBJECT_NS_LIMIT = 3
  end

  class Taker < Holder
    #: () -> Item
    def make = Item.new

    #: (Array[Item]) -> Integer
    def count(xs) = xs.size + OBJECT_NS_LIMIT
  end

  module Outer
    module Inner
      #: (Integer) -> Vec
      def self.vec(n) = Vec.new(n)
    end
  end

  # Helpers for the checks that were testdata/run/object_operators.rb.
  class OpVec
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (Integer, Integer) -> void
    def initialize(x, y)
      @x = x
      @y = y
    end

    #: (OpVec) -> OpVec
    def +(o) = OpVec.new(x + o.x, y + o.y)

    #: (OpVec) -> OpVec
    def -(o) = OpVec.new(x - o.x, y - o.y)

    #: (Integer) -> OpVec
    def *(k) = OpVec.new(x * k, y * k)

    #: () -> OpVec
    def -@ = OpVec.new(-x, -y)

    #: (OpVec) -> bool
    def ==(o) = x == o.x && y == o.y

    #: (Integer) -> Integer
    def [](i) = i == 0 ? x : y

    #: (Integer, Integer) -> void
    def []=(i, v)
      if i == 0
        @x = v
      else
        @y = v
      end
    end

    #: () -> String
    def to_s = "(#{x}, #{y})"

    #: () -> String
    def inspect = "#<OpVec #{x},#{y}>"

    #: () -> bool
    def zero? = x == 0 && y == 0

    #: () -> OpVec
    def negate! = self * -1

    #: (Integer) -> void
    def x=(v)
      @x = v
    end
  end

  # << returning self chains; unary +@ and ~; an ivar mutated inside a block.
  class OpBag
    #: () -> void
    def initialize
      @items = [] #: Array[Integer]
      @total = 0
    end

    #: (Integer) -> self
    def <<(x)
      @items << x
      self
    end

    #: () -> Integer
    def sum
      @total = 0
      @items.each { |i| @total += i }
      @total
    end

    #: () -> Integer
    def +@ = @items.size

    #: () -> Integer
    def ~ = -@items.size

    #: () -> Array[Integer]
    def evens = @items.select { |i| i.even? && i > @total / 100 }
  end

  # The default to_s/inspect name the class (MRI appends an address, so only the prefix is compared);
  # inspect does not call a user's to_s.
  class OpPlain
  end

  class Shown
    #: () -> String
    def to_s = "shown!"
  end

  module OpDeep
    class Thing
      #: () -> void
      def initialize
        @a = 1
      end
    end
  end

  # == taking untyped and checking the class first.
  class Coin
    attr_reader :cents #: Integer

    #: (Integer) -> void
    def initialize(cents)
      @cents = cents
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Coin) && cents == other.cents
  end

  # Helpers for the checks that were testdata/run/object_override_chain.rb.
  class OcBase
    #: (Integer) -> String
    def f(n) = "base #{n}"

    #: () -> OcBase
    def me = self

    #: () -> self
    def dup_me = self

    #: (self) -> bool
    def same?(o) = o.equal?(self)

    #: () -> String
    def name = "base"

    #: () -> String
    def call_f = f(7)

    #: (String) -> String
    def self.make(s) = "OcBase.make #{s}"
  end

  class OcSub < OcBase
    #: (?Integer) -> String
    def f(n = 3) = "sub #{n} " + super(n)

    #: () -> OcSub
    def me = self

    def dup_me = self

    def same?(o) = !o.equal?(self)

    def name = "sub"

    #: () -> String
    def only = "only"

    #: () -> String
    def self.make = "OcSub.make"
  end

  class SubSub < OcSub
    #: () -> String
    def f = "subsub"

    def name = "subsub"
  end

  class OcLeaf < SubSub
  end

  # Helpers for the checks that were testdata/run/object_struct_data.rb.
  SdPoint = Struct.new(
    :x, #: Integer
    :y  #: Integer
  )

  SdPair = Struct.new(:left, :right) #: [String, Float]

  Rec = Struct.new(:id, :name, :note) do
    #: () -> String
    def label = "#{id}:#{name || "-"}:#{note || "-"}"

    #: (Integer) -> Rec
    def self.numbered(n) = new(n)
  end #: [Integer, String?, String?]

  class SdNamed < Rec
    def label = "named " + super
  end

  module SdGeo
    Coord = Data.define(
      :lat, #: Float
      :lng  #: Float
    ) do
      #: () -> String
      def to_s = "(#{lat}, #{lng})"

      #: () -> Float
      def sum = lat + lng
    end

    Tag = Struct.new(:text) #: [String]
  end

  Money = Data.define(:cents, :currency) #: [Integer, String]

  Inner = Struct.new(:a) #: [Integer]

  Wrap = Struct.new(:inner, :list, :map) #: [Inner, Array[String], Hash[Symbol, Integer]]

  Kw = Struct.new(:type, :range, :func, :map, :go, :select) #: [String, Integer, String, String, bool, Integer]

  class SdThing
    attr_reader :type #: String
    attr_accessor :interface #: Integer

    #: () -> void
    def initialize
      @type = "t"
      @interface = 1
    end

    #: () -> String
    def len = "len"

    #: () -> String
    def string = "string"

    #: () -> String
    def default = "default"
  end

  SdVal = Data.define(:v) #: [Float]

  # Self-referential member types (an optional link, an array of children); a Struct subclass
  # adding its own attr and initialize that calls super.
  ListNode = Struct.new(:val, :nxt) #: [Integer, ListNode?]

  Tree = Struct.new(:val, :kids) #: [Integer, Array[Tree]]

  class Pt3 < SdPoint
    attr_reader :z #: Integer

    #: (Integer, Integer, Integer) -> void
    def initialize(x, y, z)
      super(x, y)
      @z = z
    end

    #: () -> Integer
    def sum = x + y + z
  end

  class ViVault
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
      @pin = 1234
    end

    #: () -> String
    def open = "#{name} opened with #{masked} #{self.check(4)}"

    #: () -> String
    def via_top = top_helper(pin % 10)

    private

    #: () -> Integer
    def pin = @pin

    #: () -> String
    def masked = "*" * pin.to_s.size

    #: (Integer) -> bool
    def check(n) = pin.to_s.size == n

    public

    #: () -> String
    def status = "ok #{check(3)}"

    #: () -> String
    private def secret = "s3cret"

    #: () -> String
    def reveal = secret.reverse
  end

  class BigVault < ViVault
    #: () -> String
    def deep = "#{masked}|#{pin}|#{secret}"

    def status = "big " + super
  end

  # A private method called from inside a block, and a private hand-written setter through self.
  class Dial
    #: () -> void
    def initialize
      @pin = 0
    end

    #: (Integer) -> Array[Integer]
    def scaled(n) = [1, 2].map { |x| helper(x) * n }

    #: (Integer) -> Integer
    def set(v)
      self.pin = v
      @pin
    end

    private

    #: (Integer) -> Integer
    def helper(x) = x + @pin

    #: (Integer) -> void
    def pin=(v)
      @pin = v
    end
  end

  class ObjectAttrsTest < Minitest::Test
    def test_section_0
      a = Account.new("ann")
      assert_equal "ann", a.owner
      assert_equal 0, a.balance
      assert_equal -100, a.limit
      a.deposit(50).deposit(25)
      assert_equal 75, a.balance
      assert_equal [50, 25], a.history
      assert_equal true, a.withdraw(100)
      assert_equal -25, a.balance
      assert_equal false, a.withdraw(80)
      assert_equal -25, a.balance
      assert_equal [50, 25, -100], a.history
      assert_equal "(none)", a.note_text
      a.note = "vip"
      assert_equal "vip", a.note_text
      a.note = nil
      assert_equal "(none)", a.note_text
      assert_nil a.nick
      assert_equal "ann", a.display_name
      a.nick = "ännie"
      assert_equal "ännie", a.nick
      assert_equal "ÄNNIE", a.display_name
      a.balance = 0
      a.limit = a.limit + 5
      assert_equal 0, a.balance
      assert_equal -95, a.limit
      assert_equal "ann:0", a.summary
      a.deposit(1)
      assert_equal "ann:0", a.summary
      a.reset_cache
      assert_equal "ann:1", a.summary
      a.tag(:b, "two")
      a.tag(:a, "one")
      a.tag(:b, "deux")
      assert_equal "{b: \"deux\", a: \"one\"}", (a.tags).inspect
      assert_equal "0.5", (a.ratio).inspect
      a.balance = 42
      assert_equal 42, a.balance
      s = Savings.new("sam", 0.25)
      assert_equal "sam", s.owner
      assert_equal 100, s.balance
      assert_equal 0, s.limit
      assert_equal "0.25", (s.rate).inspect
      s.deposit(20).history
      assert_equal 30, s.interest
      assert_equal 20, s.first_deposit
      assert_equal [20], s.history
      assert_equal false, s.withdraw(121)
      assert_equal true, s.withdraw(120)
      assert_equal 0, s.balance
      assert_equal "sam:0", s.summary
      accounts = [a, s] #: Array[Account]
      assert_equal [42, 0], accounts.map(&:balance)
      assert_equal [4, 2], (accounts.map { |x| x.history.size })
      accounts.each { |x| x.balance = x.balance * 2 }
      assert_equal [84, 0], accounts.map(&:balance)
      big = Account.new("big", 9_223_372_036_854_775_000)
      assert_equal 9223372036854775000, big.balance
      root = Profile.new(nil)
      kid = Profile.new("kid", root)
      baby = Profile.new("baby", kid)
      assert_equal "anon", root.shout
      assert_equal "KID", kid.shout
      assert_equal "baby < kid < root", baby.lineage
      assert_equal "root", root.lineage
      assert_equal 2, baby.depth
      assert_equal 0, root.depth
      assert_equal "kid", baby.parent_nick
      assert_equal "none", root.parent_nick
      assert_equal "none", kid.parent_nick
      baby.nick = nil
      assert_equal "anon", baby.shout
      assert_nil baby.nick
      kid.nick = "k2"
      assert_equal "? < k2 < root", baby.lineage
      assert_equal "K2 then nil", kid.reset
      assert_equal "none", kid.reset
      assert_nil kid.nick
      up = baby.parent
      assert_equal "? < root", (up.lineage if up)
      assert_nil (up&.parent&.nick)
      assert_nil (root.parent&.nick)
      lz = Lazy.new
      lz.data << 3
      lz.load
      assert_equal [1, 2, 3], lz.data
      assert_equal "yes 6", lz.loaded
      per = AtPerson.new("ann", 3)
      assert_equal "Ann", per.name
      per.name = "bob"
      per.age = -4
      assert_equal "Bob", per.name
      assert_equal 0, per.age
      kd = AtKid.new("cy", 2)
      kd.age = 7
      assert_equal "Cy/7", kd.info
    end
  end

  class ObjectBugAttrOpAssignTest < Minitest::Test
    def test_section_0
      c = AoCounter.new
      c.n += 3
      c.n *= 4
      c.drop(20)
      assert_equal -8, c.n
      c.label ||= "first"
      c.label ||= "second"
      assert_equal "first", c.label
      pt = AoPoint.new(1, 2)
      pt.x += 10
      pt.y -= 1
      assert_equal "#<struct ObjectTests::AoPoint x=11, y=1>", (pt).inspect
    end
  end

  class ObjectBugClassNameGeneratedCollisionTest < Minitest::Test
    def test_section_0
      assert_equal "user", User.new.to_s
      assert_equal "new user", NewUser.new.to_s
      assert_equal "shape", Shape.new.to_s
      assert_equal "shape i", ShapeI.new.to_s
      assert_equal "Foo::Bar", Foo::Bar.new.to_s
      assert_equal "Foo_Bar", Foo_Bar.new.to_s
      assert_equal "NewUser", NewUser.name
      assert_equal "ObjectTests::ShapeI", ShapeI.name
      assert_equal "ObjectTests::Foo_Bar", Foo_Bar.name
      assert_equal "ObjectTests::Foo::Bar", Foo::Bar.name
    end
  end

  class ObjectBugComparableStructTest < Minitest::Test
    def test_section_0
      v1 = Version.new(1, 2)
      v2 = Version.new(1, 10)
      v3 = Version.new(2, 0)
      assert_equal true, (v1 < v2)
      assert_equal false, (v2 < v1)
      assert_equal true, (v1 <= v1)
      assert_equal true, (v3 > v2)
      assert_equal false, (v1 >= v3)
      assert_equal -1, (v1 <=> v2)
      assert_equal 1, (v3 <=> v1)
      assert_equal 0, (v1 <=> Version.new(1, 2))
      assert_equal true, (v2.between?(v1, v3))
      assert_equal false, (v3.between?(v1, v2))
      assert_equal true, (v1.between?(v1, v1))
      assert_equal "v1.10", (v1.clamp(v2, v3)).to_s
      assert_equal "v1.10", (v3.clamp(v1, v2)).to_s
      assert_equal "v1.10", (v2.clamp(v1, v3)).to_s
      assert_equal ["v1.2", "v1.10", "v2.0"], ([v3, v1, v2].sort.map(&:to_s))
      assert_equal "v1.2", ([v3, v1, v2].min.to_s)
      assert_equal "v2.0", ([v3, v1, v2].max.to_s)
    end
  end

  class ObjectBugConstGetMessagesTest < Minitest::Test
    def test_section_0
      e = assert_raises(TypeError) { CgM.const_get(42) }
      assert_equal "no implicit conversion of Integer into String", e.message
      e = assert_raises(TypeError) { CgM.const_get(nil) }
      assert_equal "no implicit conversion of nil into String", e.message
      e = assert_raises(NameError) { CgM.const_get(:lower) }
      assert_equal "wrong constant name lower", e.message
      e = assert_raises(NameError) { CgM.const_get("") }
      assert_equal "wrong constant name ", e.message
      e = assert_raises(NameError) { CgM.const_defined?("lower") }
      assert_equal "wrong constant name lower", e.message
    end
  end

  class ObjectBugHookMethodsTest < Minitest::Test
    def test_section_0
      assert_equal ["inherited by ObjectTests::Alpha", "included in ObjectTests::HkThing", "extended ObjectTests::HkOther"], HK_LOG
    end
  end

  # #75: an unnamed Class.new(Base) runs Base.inherited each time it is evaluated
  ANON_HK_LOG = [] #: Array[String]

  class AnonHkBase
    #: (Class) -> void
    def self.inherited(sub)
      ANON_HK_LOG << "inherited by a #{sub.superclass}"
    end
  end

  class ObjectAnonInheritedTest < Minitest::Test
    #: () -> Class
    def make = Class.new(AnonHkBase)

    def test_unnamed_class_new_runs_inherited
      ANON_HK_LOG.clear
      make
      make
      assert_equal ["inherited by a ObjectTests::AnonHkBase", "inherited by a ObjectTests::AnonHkBase"], ANON_HK_LOG
    end
  end

  class ObjectBugModuleSuperTest < Minitest::Test
    def test_section_0
      assert_equal "loud a", MsB.new.hi
      MS_LOG.clear
      assert_equal "shout p1", MsQ.new(1).hi
      assert_equal 208, MsQ.new(2).calc(3)
      assert_equal "r shout p4", R.new(4).hi
      assert_equal "quiet shout p5", S.new(5).hi
      assert_equal "both shout p6", T.new(6).hi
      assert_equal ["p init 1", "shout init 1", "p init 2", "shout init 2", "p init 4", "shout init 4",
                    "p init 5", "shout init 5", "p init 6", "shout init 6"], MS_LOG
      e = assert_raises(NoMethodError) { U.new.hi }
      assert_equal "super: no superclass method 'hi' for an instance of ObjectTests::U", e.message
    end
  end

  class ObjectBugNarrowedCallStmtTest < Minitest::Test
    def test_section_0
      h = NcHtmlBuilder.new
      h.add("a")
      h.add("b").add("c")
      assert_equal "a-b-c", h.build
      NcPlugins.const_get(:VERSION)
      NcPlugins.const_get(:Alpha)
      make(NcSquare)
      assert_equal ["made 1"], NC_LOG
    end
  end

  class ObjectBugsTest < Minitest::Test
    def test_section_0
      assert_equal "opt", Opt.new.to_s
      assert_equal "ref", Ref.new.to_s
      assert_equal "NameError: uninitialized constant ObjectTests::BG_LIMIT", BG_LIM_ERR
      assert_equal 5, lim
      assert_nil Settings::DEFAULT
      assert_equal [:DEFAULT], Settings.constants
      assert_nil Settings.const_get(:DEFAULT)
      assert_equal true, Settings.const_get("DEFAULT").nil?
      type_err = assert_raises(TypeError) { Object.const_get("ObjectTests::BgPlugins::VERSION::X") }
      assert_equal "ObjectTests::BgPlugins::VERSION::X does not refer to class/module", type_err.message
      here = Coord.new(lat: 1.0, lng: 2.0)
      assert_equal true, here.with.equal?(here)
      assert_equal false, (here.with(lat: 1.0).equal?(here))
      assert_equal true, (here.with == here)
      assert_equal false, BgPlain.new.frozen?
      assert_equal false, (BgPair.new(1, 2).frozen?)
      assert_equal true, BgVal.new(1).frozen?
      d = BgDog.new
      assert_equal true, d.is_a?(BgAnimal)
      assert_equal [:BG_LIMIT, :OWN], BgUses.constants.sort
      assert_equal true, BgUses.const_defined?(:BG_LIMIT)
      assert_equal 5, BgUses.const_get(:BG_LIMIT)
      assert_equal "String", bg_kind("a")
      assert_equal "NilClass", bg_kind(nil)
      assert_equal "NilClass", (nil.class).to_s
      assert_equal "n", BG_NAME
      assert_equal 3, COUNT
      assert_equal "l", Labels::LABEL
      loud_name = BG_NAME.upcase if BG_NAME
      assert_equal "N", loud_name
      assert_equal 4, (COUNT || 0) + 1
      span = Span.new(1, 5)
      assert_equal 1, span.begin
      assert_equal 5, span.end
      assert_equal "#<struct ObjectTests::Span begin=1, end=5>", (span).inspect
      assert_equal [1, 5], span.to_a
      assert_equal "{begin: 1, end: 5}", (span.to_h).inspect
      span.end = 7
      assert_equal true, (span == Span.new(1, 7))
      l = Link.new(in: "a", out: "b")
      assert_equal "a", l.in
      assert_equal "#<data ObjectTests::Link in=\"a\", out=\"b\">", (l).inspect
      assert_equal "#<data ObjectTests::Link in=\"a\", out=\"c\">", ((l.with(out: "c"))).inspect
      assert_equal true, (Edge.new("a", "b") == Edge.new("a", "b"))
      assert_equal false, (Edge.new("a", "b") == Edge.new("a", "c"))
      assert_equal true, (LinkOther.new(1) == LinkOther.new(1))
      assert_equal false, (LinkOther.new(1) == LinkOther.new(2))
      seen = [] #: Array[Integer]
      tb = TapBox.new(4).tap { |b| seen << b.n }
      assert_equal 4, tb.n
      assert_equal 7, (7.tap { |x| seen << x + 1 })
      assert_equal [4, 8], seen
      ups = [] #: Array[String]
      assert_equal 1, ("s".tap { |s| ups << s.upcase }.size)
      assert_equal ["S"], ups
      assert_equal [1, 2, 3], ([1, 2].tap { |a| a << 3 })
      SgConfig.instance.level = 3
      assert_equal ["init"], SG_LOG
      assert_equal 3, SgConfig.instance.level
      assert_equal true, SgConfig.instance.equal?(SgConfig.instance)
      SgApp::Registry.instance.add("a")
      assert_equal ["a", "b"], SgApp::Registry.instance.add("b")
      assert_equal true, SgConfig.instance.is_a?(Singleton)
    end
  end

  class ObjectClassMethodsTest < Minitest::Test
    def test_section_0
      assert_equal "shape", CmShape.kind
      assert_equal "square", CmSquare.kind
      assert_equal "shape", Tri.kind
      assert_equal "tiny", Tiny.kind
      assert_equal "shape/0", CmShape.info
      assert_equal "sq square/4", CmSquare.info
      assert_equal "shape/3", Tri.info
      assert_equal "sq tiny/4", Tiny.info
      assert_equal "shape:ObjectTests::CmShape(2) sides=0", CmShape.build(1).describe
      assert_equal "square:ObjectTests::CmSquare(2) sides=4", CmSquare.build(1).describe
      assert_equal "shape:ObjectTests::Tri(10) sides=3", Tri.build_explicit(5).describe
      assert_equal "square:ObjectTests::CmSquare(102) sides=4", CmSquare.new(2).grow.describe
      assert_equal "shape:ObjectTests::Tri(100) sides=3", Tri.new(0).grow.describe
      assert_equal "tiny:ObjectTests::Tiny(1) sides=4", Tiny.new.describe
      assert_equal 1, Tiny.new.size
      klasses = [CmShape, CmSquare, Tri] #: Array[singleton(CmShape)]
      assert_equal ["shape:ObjectTests::CmShape(7) sides=0", "square:ObjectTests::CmSquare(7) sides=4", "shape:ObjectTests::Tri(7) sides=3"], klasses.map { |k| k.new(7).describe }
      assert_equal ["shape", "square", "shape"], klasses.map(&:kind)
      assert_equal [0, 4, 3], (klasses.map { |k| k.sides })
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri"], klasses.map(&:name)
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri"], klasses.map(&:to_s)
      assert_equal "[ObjectTests::CmShape, ObjectTests::CmSquare, ObjectTests::Tri]", (klasses).inspect
      assert_equal [1, 1, 1], (klasses.map { |k| k.build(0).size })
      k = klasses.fetch(1)
      assert_equal "square", k.kind
      assert_equal "ObjectTests::CmSquare", k.name
      assert_equal "square:ObjectTests::CmSquare(3) sides=4", k.new(3).describe
      assert_equal "sq square/4", k.info
      k = klasses.fetch(2)
      assert_equal "shape", k.kind
      assert_equal true, (k == Tri)
      assert_equal false, (k == CmSquare)
      assert_equal true, (k != CmSquare)
      by_name = { "sq" => CmSquare, "tri" => Tri } #: Hash[String, singleton(CmShape)]
      found = by_name["tri"]
      assert_equal "shape", (found.kind if found)
      assert_equal ["sq", "tri"], by_name.keys
      assert_equal "[ObjectTests::CmSquare, ObjectTests::Tri]", (by_name.values).inspect
      s = CmSquare.new(9)
      assert_equal "ObjectTests::CmSquare", (s.class).to_s
      assert_equal "ObjectTests::CmSquare", s.class.name
      assert_equal "square", s.class.kind
      assert_equal true, (s.class == CmSquare)
      assert_equal false, (s.class == CmShape)
      shapes = [CmShape.new(1), CmSquare.new(2), Tri.new(3), Tiny.new] #: Array[CmShape]
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri", "ObjectTests::Tiny"], (shapes.map { |x| x.class.name })
      assert_equal [0, 4, 3, 4], (shapes.map { |x| x.class.sides })
      assert_equal [false, true, false, false], (shapes.map { |x| x.class == CmSquare })
      assert_equal "Class", (CmShape.class).to_s
      assert_equal "Class", (CmSquare.class).to_s
      assert_equal "Class", (CmShape.class.class).to_s
      assert_equal "Module", (Comparable.class).to_s
      assert_equal "ObjectTests::CmShape", (CmShape).inspect
      assert_equal "ObjectTests::CmShape", CmShape.to_s
      assert_equal "ObjectTests::CmShape", CmShape.name
      assert_equal "Integer", (1.class).to_s
      assert_equal "String", ("s".class).to_s
      assert_equal "Symbol", (:sym.class).to_s
      assert_equal "Float", (1.5.class).to_s
      assert_equal "Array", ([1].class).to_s
      assert_equal "Hash", (({ a: 1 }.class)).to_s
      assert_equal "Class", (1.class.class).to_s
      assert_equal "Class", (Integer.class).to_s
      assert_equal "Integer", Integer.name
      assert_equal "String", (String).inspect
      assert_equal true, (1.class == Integer)
      assert_equal false, ("s".class == Symbol)
      CmRegistry.bump
      CmRegistry.bump
      SubRegistry.bump
      assert_equal 2, CmRegistry.count
      assert_equal 1, SubRegistry.count
      assert_equal "<a>", CmRegistry.fmt("a")
      assert_equal "<B>!", SubRegistry.fmt("b")
      assert_equal "<>!", SubRegistry.fmt("")
      mods = [CmShape, Comparable, Enumerable, CmRegistry] #: Array[Module]
      assert_equal ["ObjectTests::CmShape", "Comparable", "Enumerable", "ObjectTests::CmRegistry"], mods.map(&:name)
      assert_equal ["ObjectTests::CmShape", "Comparable", "Enumerable", "ObjectTests::CmRegistry"], mods.map(&:to_s)
      assert_equal "[ObjectTests::CmShape, Comparable, Enumerable, ObjectTests::CmRegistry]", (mods).inspect
      klass_list = [CmShape, CmRegistry, String] #: Array[Class]
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmRegistry", "String"], klass_list.map(&:name)
      assert_equal true, (klass_list.first(1).fetch(0) == CmShape)
      assert_equal "red", Color::RED.name
      assert_equal ["red", "blue"], Color::ALL.map(&:name)
      assert_equal "RED", Color.default.shout
      assert_equal "<x>!", Shade.fmt("x")
      assert_equal "<p>!", Shade.plain
      assert_equal 9, CmDeep.new("d").depth
      assert_equal 1, (CmDeep.new("e", 1).depth)
      assert_equal "S", Shade.new("s").shout
      assert_equal "square", object_pick(true).kind
      assert_equal "shape", object_pick(false).kind
      assert_equal "square:ObjectTests::CmSquare(4) sides=4", object_pick(true).new(4).describe
      assert_equal "square:ObjectTests::CmSquare(1) sides=4", by_num(1).describe
      assert_equal "ObjectTests::Tri", (by_num(2).class).to_s
      assert_equal "shape:ObjectTests::CmShape(0) sides=0", by_num(0).describe
      flag = CmShape.sides == 0
      kl = flag ? CmSquare : Tri
      assert_equal "square", kl.kind
      assert_equal "square:ObjectTests::CmSquare(5) sides=4", kl.new(5).describe
      assert_equal "ObjectTests::CmSquare", kl.name
      inst = flag ? Tri.new(1) : CmSquare.new(1)
      assert_equal "shape:ObjectTests::Tri(1) sides=3", inst.describe
      names = { CmSquare => "sq" } #: Hash[singleton(CmShape), String]
      names[Tri] = "tri"
      names[CmSquare] = "SQ"
      assert_equal "SQ", names[CmSquare]
      assert_equal "tri", names[Tri]
      assert_nil names[CmShape]
      assert_equal 2, names.size
      assert_equal "[ObjectTests::CmSquare, ObjectTests::Tri]", (names.keys).inspect
      tally = {} #: Hash[singleton(CmShape), Integer]
      [CmSquare, Tri, CmSquare, Tiny].each { |c| tally[c] = tally.fetch(c, 0) + 1 }
      assert_equal "{ObjectTests::CmSquare => 2, ObjectTests::Tri => 1, ObjectTests::Tiny => 1}", (tally).inspect
      assert_equal ["square", "shape", "tiny"], tally.keys.map(&:kind)
      assert_equal "reopened square", CmSquare.extra
      assert_equal "reopened tiny", Tiny.extra
      assert_equal 9, CmSquare.new(3).area
      assert_equal 1, Tiny.new.area
      Tally.bump
      Tally.bump
      assert_equal 2, Tally.count
      assert_equal "shape", CmShape.me.kind
      assert_equal "tiny", Tiny.me.kind
      assert_equal "ObjectTests::Tiny", Tiny.me.name
      assert_equal "made square 2", CmSquare.new(2).maker.made
      assert_equal "made tiny 1", Tiny.new.maker.made
      assert_equal ["shape", "square", "tiny"], HANDLERS.map(&:kind)
      assert_equal [0, 4, 4], (HANDLERS.map { |h| h.me.sides })
      assert_equal "[ObjectTests::CmShape, ObjectTests::CmSquare, ObjectTests::Tiny]", (HANDLERS).inspect
      conf = Conf.build { |x| x.port = 8080 }
      assert_equal 8080, conf.port
      assert_equal "n=2", (Conf.fmt { |n| "n=#{n}" })
      assert_equal "1010", (Conf.fmt(5) { |n| n.to_s * 2 })
      tmp = CmTemp.new(1)
      assert_equal 0, CmTemp::FREEZING.deg
      assert_equal 100, CmTemp::RANGE
      assert_equal true, tmp.is_a?(Object)
      assert_equal 1, tmp.deg
      Handler.register("p", Posts)
      Posts.register("u", Users)
      assert_equal "run posts", Handler.dispatch("p")
      assert_equal "run users", Users.dispatch("u")
      assert_equal "none", Handler.dispatch("x")
      assert_equal ["p", "u"], Handler::REGISTRY.keys
      assert_equal 2, Handler::REGISTRY.size
      fa = Factory.new(Users)
      assert_equal "run users", fa.make.run
      assert_equal "users", fa.kind
      assert_equal "run base", Factory.new(Handler).make.run
      assert_equal "pretty d", (CmDoc.new).to_s
      assert_equal "pretty d!", "#{CmDoc.new}!"
      assert_equal 8, CmDoc.new.to_s.size
      blobs = [Blob, SubBlob] #: Array[singleton(Blob)]
      assert_equal true, Blob.respond_to?(:kind)
      assert_equal false, Blob.respond_to?(:area)
      assert_equal true, SubBlob.respond_to?(:only_sub)
      assert_equal false, Blob.respond_to?(:only_sub)
      assert_equal [false, true], (blobs.map { |k| k.respond_to?(:only_sub) })
      assert_equal false, Blob.new.respond_to?(:kind)
      assert_equal true, Blob.respond_to?(:new)
      assert_equal true, Blob.respond_to?(:name)
      assert_equal "CustomWidget", CmWidget.name
      assert_equal "ObjectTests::CmWidget", (CmWidget).to_s
      assert_equal "ObjectTests::CmWidget", (CmWidget).inspect
      assert_equal "ObjectTests::CmWidget", "#{CmWidget}"
      assert_equal "[ObjectTests::CmWidget]", ([CmWidget]).inspect
      assert_equal "CustomWidget", CmWidget.new.class.name
      assert_equal "ObjectTests::CmGadget", CmGadget.name
      assert_equal "GadgetClass", (CmGadget).to_s
      assert_equal "ObjectTests::CmGadget", (CmGadget).inspect
      assert_equal "GadgetClass", "#{CmGadget}"
    end
  end

  class ObjectClassNewArityTest < Minitest::Test
    def test_section_0
      assert_equal 5, (Two.new(2, 3).n)
      # klass.new(1) through singleton(NaBase) on Two stays in testdata/run/object_class_new_arity.rb:
      # rugaroo fails there with a Go type-assertion error, not ArgumentError (decision 19).
      ks = [NaBase, Same] #: Array[singleton(NaBase)]
      assert_equal ["ObjectTests::NaBase: 1", "ObjectTests::Same: 1"], ks.map { |k| "#{k.name}: #{k.new(1).n}" }
    end
  end

  class ObjectCmpNilTest < Minitest::Test
    def test_section_0
      a = Score.new(1.5)
      b = Score.new(0.5)
      assert_equal 1, (a <=> b)
      assert_equal ["S0.5", "S1.5"], ([a, b].sort.map(&:to_s))
      assert_equal "S0.1", ([a, b, Bonus.new(0.1)].min.to_s)
      z = 0.0 #: Float
      n = Score.new(z / z)
      assert_nil (n <=> a)
      e = assert_raises(ArgumentError) { [a, n].sort }
      assert_equal "comparison of ObjectTests::Score with ObjectTests::Score failed", e.message
      e = assert_raises(ArgumentError) { [Bonus.new(1.0), Bonus.new(z / z)].max }
      assert_equal "comparison of ObjectTests::Bonus with ObjectTests::Bonus failed", e.message
      x = 1.5 #: Float
      assert_equal -1, (x <=> 2)
      assert_equal 1, (2 <=> x)
      assert_equal 0, ((x <=> 2.0) || 0) + 1
    end
  end

  class ObjectConstantsTest < Minitest::Test
    def test_section_0
      assert_equal [:Alpha, :Base, :Beta, :MAX, :VERSION], CtPlugins.constants.sort
      assert_equal [:One, :Root, :Two], Kinds.constants.sort
      assert_equal [:COLOR, :SIZE], CtChild.constants.sort
      assert_equal [:COLOR], CtParent.constants
      assert_equal [], CtPlugins::Alpha.constants
      assert_equal "1.0", CtPlugins.const_get(:VERSION)
      assert_equal 3, CtPlugins.const_get("MAX")
      assert_equal 4, (CtPlugins.const_get(:MAX) + 1)
      assert_equal "alpha", CtPlugins.const_get(:Alpha).id
      assert_equal "base run", CtPlugins.const_get("Beta").new.run
      assert_equal "alpha run", CtPlugins.const_get(:Alpha).new.run
      assert_equal 1, rank_of("One")
      assert_equal 2, rank_of("Two")
      assert_equal 0, rank_of("Root")
      assert_equal [1, 0, 2], (Kinds.constants.sort.map { |c| Kinds.const_get(c).rank })
      assert_equal "red", CtChild.const_get(:COLOR)
      assert_equal 5, CtChild.const_get(:SIZE)
      assert_equal 99, Object.const_get(:OBJECT_TOP)
      assert_equal "ObjectTests::CtPlugins::Alpha", (Object.const_get("ObjectTests::CtPlugins::Alpha")).to_s
      assert_equal "1.0", Object.const_get("::ObjectTests::CtPlugins::VERSION")
      assert_equal 99, CtPlugins.const_get(:OBJECT_TOP)
      assert_equal "String", (CtPlugins.const_get("String")).to_s
      assert_equal true, (ObjectTests.const_get(:CtPlugins) == CtPlugins)
      assert_equal true, (CtPlugins.const_get("Alpha") == CtPlugins::Alpha)
      assert_equal false, (CtPlugins.const_get(:Beta) == CtPlugins::Alpha)
      assert_equal true, CtPlugins.const_defined?(:VERSION)
      assert_equal true, CtPlugins.const_defined?("Alpha")
      assert_equal false, CtPlugins.const_defined?(:Gamma)
      assert_equal true, CtChild.const_defined?(:COLOR)
      assert_equal true, Object.const_defined?("ObjectTests::CtPlugins::Base")
      assert_equal false, Object.const_defined?("ObjectTests::CtPlugins::Nope")
      assert_equal true, CtPlugins.const_defined?(:OBJECT_TOP)
      assert_equal true, Object.const_defined?(:OBJECT_TOP)
      e = assert_raises(NameError) { CtPlugins.const_get(:Gamma) }
      assert_equal "uninitialized constant ObjectTests::CtPlugins::Gamma", e.message
      e = assert_raises(NameError) { Object.const_get("Nowhere") }
      assert_equal "uninitialized constant Nowhere", e.message
      e = assert_raises(NameError) { Object.const_get("ObjectTests::CtPlugins::Alpha::Deep") }
      assert_equal "uninitialized constant ObjectTests::CtPlugins::Alpha::Deep", e.message
      assert_equal 5, child_const("SIZE")
      assert_equal "red", child_const("COLOR")
      assert_equal 5, CtChild.const_get("SIZE".downcase.upcase)
      assert_equal [18, 2], (Limits.constants.sort.map { |c| Limits.const_get(c) * 2 })
      assert_equal [:Coord, :ORIGIN, :Tag], CtGeo.constants.sort
      assert_equal "#<struct ObjectTests::CtGeo::Tag text=\"t\">", (CtGeo.const_get(:Tag).new("t")).inspect
      assert_equal [:lat, :lng], CtGeo.const_get(:Coord).members
      assert_equal "0.0", (CtGeo.const_get(:ORIGIN).lat).inspect
      assert_equal [], CtGeo::Tag.constants
      assert_equal "ObjectTests::CtGeo::Coord", CtGeo.const_get("Coord").name
    end

    # was the file's uncaught error at exit
    def test_const_get_missing_raises
      e = assert_raises(NameError) { Kinds.const_get(:Three) }
      assert_equal "uninitialized constant ObjectTests::Kinds::Three", e.message
    end
  end

  class ObjectDefaultArgsCalleeTest < Minitest::Test
    def test_section_0
      b = DaBase.new
      assert_equal 30, b.calc
      assert_equal 3, b.calc(1)
      assert_equal 2, (b.calc(1, 1))
      assert_equal 3, b.sum(1)
      assert_equal 6, (b.sum(1, 5))
      assert_equal 8, (b.sum(1, 5, 7, 8))
      ups = [] #: Array[Integer]
      b.times_up { |i| ups << i }
      b.times_up(2) { |i| ups << i }
      assert_equal [0, 1, 2, 0, 1], ups
      assert_equal 3003, via_base(DaKid.new)
      assert_equal 3007, DaKid.new.calc(5)
      assert_equal "hello module", b.hello
      assert_equal "hello kid", DaKid.new.hello
      assert_equal "hello you", b.hello("you")
      assert_equal 12, DaBox.new(3).area
      assert_equal 9, (DaBox.new(3, 3).area)
      assert_equal "made shape", DaShape.make
      assert_equal "made circle", Circle.make
      assert_equal "made x", Circle.make("x")
      assert_equal 10, top(2)
      assert_equal 4, (top(2, 2))
      l = Log.new
      assert_equal "x", l.f("x")
      assert_equal "y", (l.f("y", "z"))
      assert_equal 1, l.count
    end
  end

  class ObjectExtendBlockTest < Minitest::Test
    def test_section_0
      assert_equal 8, EbWidget.twice(4)
      assert_equal -2, EbGadget.twice(-1)
      assert_equal 0, Tools.twice(0)
      assert_equal "I am ObjectTests::EbWidget", EbWidget.describe
      assert_equal "I am ObjectTests::EbGadget", EbGadget.describe
      assert_equal "I am ObjectTests::Tools", Tools.describe
      assert_equal "widget", EbGadget.kind
      assert_equal ["RED", "GREEN", "BLUE"], (EbColors.map { |c| c.upcase })
      assert_equal 3, EbColors.count
      assert_equal ["red", "green", "blue"], EbColors.to_a
      assert_equal ["red", "green", "blue"], (EbColors.select { |c| c.include?("e") })
      assert_equal "blue", (EbColors.find { |c| c.size == 4 })
      assert_equal true, EbColors.include?("green")
      assert_equal false, EbColors.include?("pink")
      assert_equal ["red"], EbColors.first(1)
      assert_equal ["red", "blue", "green"], (EbColors.sort_by { |c| c.size })
      assert_equal "blue", EbColors.min
      assert_equal "red", EbColors.max
      assert_equal "ObjectTests::Handlers::Users", ((Handlers.detect { |h| h.matches?("/users/1") })).inspect
      assert_equal ["ObjectTests::Handlers::Base", "ObjectTests::Handlers::Posts", "ObjectTests::Handlers::Users"], (Handlers.map { |h| h.name })
      assert_equal 3, Handlers.count
      assert_equal "[ObjectTests::Handlers::Posts]", ((Handlers.select { |h| h.matches?("/posts/9") })).inspect
      assert_nil (Handlers.find { |h| h.matches?("/x") })
      bag = EbBag.new.add(3).add(1).add(2)
      items = [] #: Array[Integer]
      bag.each { |n| items << n }
      assert_equal [3, 1, 2], items
      assert_equal [9, 1, 4], (bag.transform { |n| n * n })
      assert_equal [3, 1], (bag.keep { |n| n.odd? })
      assert_equal "first=3", (bag.first_as { |n| "first=#{n}" })
      total = 0
      bag.each_twice { |n| total += n }
      assert_equal 12, total
      saw = [] #: Array[String]
      bag.each do |n|
        next if n == 1

        saw << "saw #{n}"
      end
      assert_equal ["saw 3", "saw 2"], saw
      assert_equal "ObjectTests::Knob:knob", Knob.label
      assert_equal "ObjectTests::BigKnob:big", BigKnob.label
      assert_equal "ObjectTests::Lever:base", Lever.label
      assert_equal "base", Lever.kind
      knobs = [Knob, BigKnob] #: Array[singleton(Knob)]
      assert_equal ["ObjectTests::Knob:knob", "ObjectTests::BigKnob:big"], knobs.map(&:label)
      assert_equal ["knob", "big"], (knobs.map { |k| k.kind })
    end
  end

  class ObjectInheritanceTest < Minitest::Test
    def test_section_0
      b = InBase.new("b")
      m = Middle.new("m")
      l = InLeaf.new("l", 3)
      e = Empty.new("e")
      f = Fixed.new
      assert_equal "InBase[b] w=1", b.describe
      assert_equal "Middle[m] w=10", m.describe
      assert_equal "leaf: InLeaf3[l] w=13", l.describe
      assert_equal "InBase[e] w=1", e.describe
      assert_equal "InBase[fixed] w=1", f.describe
      assert_equal 1, b.weight
      assert_equal 10, m.weight
      assert_equal 13, l.weight
      assert_equal 1, e.weight
      assert_equal 2, b.scale(2)
      assert_equal 21, m.scale(2)
      assert_equal 27, l.scale(2)
      assert_equal 0, b.scale(0)
      assert_equal -39, m.scale(-4)
      assert_equal "hi a!", b.greet("a")
      assert_equal "HI A?", m.greet("a")
      assert_equal "HI A.", (m.greet("a", "."))
      assert_equal "HI Z", (l.greet("z", ""))
      assert_equal "<InBase b>", (b).to_s
      assert_equal "<Middle m>", (m).to_s
      assert_equal "<InLeaf3 l>", (l).to_s
      assert_equal "<InBase e>", (e).to_s
      assert_equal "<InBase fixed>", (f).to_s
      assert_equal "<InBase b>", b.to_s
      assert_equal "<InLeaf3 l>", l.to_s
      assert_equal 7, Root.new.n
      all = [b, m, l, e, f] #: Array[InBase]
      assert_equal ["InBase", "Middle", "InLeaf3", "InBase", "InBase"], all.map(&:name)
      assert_equal [1, 10, 13, 1, 1], (all.map { |x| x.weight })
      assert_equal ["b", "m", "l", "e", "fixed"], all.map(&:tag)
      assert_equal 28, (all.reduce(0) { |acc, x| acc + x.scale(1) })
      assert_equal true, l.is_a?(InBase)
      assert_equal true, l.is_a?(Middle)
      assert_equal true, l.is_a?(InLeaf)
      assert_equal false, m.is_a?(InLeaf)
      assert_equal false, b.kind_of?(Middle)
      assert_equal [false, true, true, false, false], (all.map { |x| x.is_a?(Middle) })
      assert_equal true, l.is_a?(Object)
      assert_equal true, l.is_a?(BasicObject)
      assert_equal true, l.is_a?(Kernel)
      assert_equal false, l.is_a?(String)
      assert_equal true, (b == b)
      assert_equal false, (b == InBase.new("b"))
      assert_equal true, (b != InBase.new("b"))
      assert_equal false, (b != b)
      assert_equal true, b.equal?(b)
      assert_equal false, b.equal?(m)
      assert_equal false, (!b)
      assert_equal false, b.nil?
      assert_equal 3, (Summer.new(1, 2).total)
      assert_equal 6, (KidSummer.new(1, 2, 3).total)
      assert_equal 0, KidSummer.new.total
      assert_equal 2, Summer.new(1).scaled
      assert_equal 2, KidSummer.new(1).scaled
      assert_equal -10, (KidSummer.new(1, -3).kid_scaled)
      narrowed = [] #: Array[String]
      all.each do |x|
        if x.is_a?(InLeaf)
          narrowed << "leaf extra #{x.extra}"
        elsif x.is_a?(Middle)
          narrowed << "middle #{x.tag}"
        end
      end
      assert_equal ["middle m", "leaf extra 3"], narrowed
      found = all.find { |x| x.is_a?(Fixed) }
      assert_equal "fixed", (found.tag if found)
      list = [InAnimal.new, InDog.new, Puppy.new, Cat.new] #: Array[InAnimal]
      assert_equal ["other", "dog woof", "puppy woof", "cat"], list.map { |a| in_kind(a) }
      kc = InKidCalc.new
      assert_equal 12, kc.compute(1)
      assert_equal 4, (kc.compute(1, 1))
      assert_equal 0, (kc.compute(-1, 0))
      assert_equal "calc!", kc.plain
      assert_equal ["<a><aa>calc!", "<b><bb>calc!"], kc.parts
      assert_equal "<x><xx>", kc.tag("x")
      assert_equal 16, GrandCalc.new.compute(1)
      assert_equal 48, (GrandCalc.new.compute(2, 5))
      assert_equal 20, InCalc.new.compute(2)
      assert_equal 1, InNode.new.id
      assert_equal 2, InNamed.new("n").id
      assert_equal "m", (InNamed.new("m", 7).label)
      assert_equal 2, Leafy.new.id
      assert_equal "leafy", Leafy.new.label
      assert_equal 42, IntBox.new.v
      assert_equal "ss", StrBox.new.v
      assert_equal 9, StrBox.new.id
      assert_equal 1, IntBox.new.id
      nodes = [InNode.new, Leafy.new, IntBox.new, StrBox.new] #: Array[InNode]
      assert_equal [1, 2, 1, 9], nodes.map(&:id)
      assert_equal ["ObjectTests::InNode", "ObjectTests::Leafy", "ObjectTests::IntBox", "ObjectTests::StrBox"], (nodes.map { |x| x.class.name })
      assert_equal 1, (nodes.select { |x| x.is_a?(InNamed) }.size)
      k1 = nodes.fetch(0)
      k2 = nodes.fetch(1)
      by_obj = { k1 => "one" } #: Hash[InNode, String]
      by_obj[k2] = "two"
      assert_equal "one", by_obj[k1]
      assert_equal "two", by_obj[k2]
      assert_nil by_obj[InNode.new]
      assert_equal 2, by_obj.size
      assert_equal true, nodes.include?(k1)
      assert_equal false, [k1].include?(k2)
      assert_equal 2, ([k1, k1, k2].uniq.size)
      assert_equal false, (k1 == k2)
      assert_equal "ObjectTests::StrBox", (nodes.max_by(&:id).class).to_s
      assert_equal 1, nodes.min_by(&:id).id
      assert_equal [9, 2, 1, 1], (nodes.sort_by { |x| -x.id }.map(&:id))
      assert_equal "[ObjectTests::InNode, ObjectTests::Leafy, ObjectTests::IntBox, ObjectTests::StrBox]", (nodes.group_by(&:class).keys).inspect
      assert_equal true, k1.equal?(nodes.fetch(0))
      assert_equal "area 4", Sq.new.show
      nie = assert_raises(NotImplementedError) { Figure.new.show }
      assert_equal "ObjectTests::Figure#area", nie.message
      # a bare rescue (StandardError) must not catch NotImplementedError (a ScriptError)
      caught = "none"
      begin
        begin
          Figure.new.area
        rescue => e
          caught = "bare rescue caught #{e.class}"
        end
      rescue NotImplementedError
        caught = "only NotImplementedError caught it"
      end
      assert_equal "only NotImplementedError caught it", caught
      hb = InHtmlBuilder.new
      assert_equal "<p>a-b</p>", hb.add("a").add("b").html
      assert_equal "a-b-c", hb.add("c").build
      hx = hb.add("d")
      assert_equal "<p>a-b-c-d</p>", hx.html
      assert_equal true, hx.equal?(hb)
      builders = [InBuilder.new, InHtmlBuilder.new] #: Array[InBuilder]
      assert_equal ["z", "z"], (builders.map { |x| x.add("z").build })
    end
  end

  class ObjectMidTest < Minitest::Test
    def test_section_0
      box_a = MdBox.new
      ret_a = (box_a.n = 42)
      assert_equal 42, ret_a
      assert_equal 42, box_a.n
      box_c = MdBox.new
      ret_x = box_c.n = 7
      assert_equal 7, ret_x
      assert_equal 0, MdTemp::FREEZING.deg
      assert_equal 100, MdTemp::BOILING.deg
      colors = [] #: Array[String]
      MdColors.each { |color| colors << color }
      assert_equal ["red", "green"], colors
      counts = [] #: Array[Integer]
      MdCounter.upto(3) { |i| counts << i }
      assert_equal [0, 1, 2], counts
      assert_equal [:SIZE], MdChild.constants(false)
      assert_equal false, (MdChild.const_defined?(:COLOR, false))
      name_err = assert_raises(NameError) { MdChild.const_get(:COLOR, false) }
      assert_equal "uninitialized constant ObjectTests::MdChild::COLOR", name_err.message
      assert_equal 5, PathChild::SIZE
      assert_equal "red", PathChild::COLOR
      assert_equal "nested", PathChild::Nested.new.hi
      gvs = [MdVec.new(1), MdVec.new(-2), MdVec.new(5)] #: Array[MdVec]
      assert_equal 8, (MdVec.new(4).then { |vb| vb.v * 2 })
      assert_equal "v=1", (MdVec.new(1).then { |vb| "v=#{vb.v}" })
      assert_equal 4, (gvs.reduce(MdVec.new(0)) { |acc, vb| acc.plus(vb) }.v)
      assert_equal false, gvs.include?(MdVec.new(5))
      klasses = [MdVec, Vec3] #: Array[singleton(MdVec)]
      assert_equal true, klasses.include?(Vec3)
      assert_equal "CustomWidget", MdWidget.name
      assert_equal "CustomWidget", SubWidget.name
      assert_equal "CustomWidget", SubWidget.new.class.name
      assert_equal "GadgetClass", (MdGadget).to_s
      assert_equal "GadgetClass", (SubGadget).to_s
      assert_equal "GadgetClass", "#{SubGadget}"
      assert_equal "GadgetInspect", (SubGadget).inspect
      assert_equal "[GadgetInspect]", ([SubGadget]).inspect
      d = MdDoc.new
      pg = Page.new
      assert_equal "ObjectTests::MdDoc", (d.class).to_s
      assert_equal "ObjectTests::Page", (pg.class).to_s
      assert_equal false, d.is_a?(Walker)
      assert_equal true, pg.is_a?(Walker)
      m = MdMemo.new
      assert_nil m.last
      assert_equal 42, m.store(21)
      assert_equal 42, m.last
      ibox = ItemBox.new
      ibox.items << "a"
      Reg.table["x"] = 1
      assert_equal ["a"], ibox.items
      assert_equal "{\"x\" => 1}", (Reg.table).inspect
      sub_a = MdBase.new #: MdBase
      sub_a = MdLeaf.new
      assert_equal "MdLeaf", sub_a.name
      sub_x = MdBase.new
      sub_x = MdLeaf.new
      assert_equal "MdLeaf", sub_x.name
      sub_y = MdLeaf.new #: MdBase
      assert_equal "MdLeaf", sub_y.name
      sub_y = MdBase.new
      assert_equal "MdBase", sub_y.name
      sub_k = MdLeaf #: singleton(MdBase)
      assert_equal "ObjectTests::MdLeaf", sub_k.name
      sub_k = MdBase
      assert_equal "ObjectTests::MdBase", sub_k.name
      assert_equal 2, MdCalc.new.calc(1)
      assert_equal 5, MdKidCalc.new.calc(2)
      assert_equal "node", NodeLeaf.new.node
      assert_equal 5, MdUses.new.lim
      assert_equal 10, MdSub.new.twice
      assert_equal 5, MdUses::LIMIT
      assert_equal 5, MdSub::LIMIT
    end
  end

  class ObjectMid2Test < Minitest::Test
    def test_section_0
      assert_equal "ObjectTests::MtFoo", MtFoo.new.kind
      assert_equal "ObjectTests::Bar", Bar.new.kind
      assert_equal "ObjectTests::Bar!", Bar.new.label
      items = [MtDoc.new, MtMemo.new] #: Array[Printable]
      assert_equal ["doc", "memo"], (items.map { |i| i.title })
      assert_equal ["P(doc)", "P(memo)"], items.map(&:to_s)
      a = NeVec.new(1)
      assert_equal true, (a == NeVec.new(1))
      assert_equal false, (a != NeVec.new(1))
      assert_equal true, (a != NeVec.new(2))
      assert_equal false, (NePoint.new(1, 2) != NePoint.new(1, 2))
      assert_equal true, (NePoint.new(1, 2) != NePoint.new(2, 1))
      assert_equal "base 1", MtBase.new.f(1)
      assert_equal "sub", MtSub.new.f
      assert_equal "base", MtBase.new.me.name
      assert_equal "sub", MtSub.new.me.name
      assert_equal "only", MtSub.new.me.only
      v = MtVault.new
      assert_equal "pin has 4 digits, hit 1", v.open
      assert_equal "pin has 4 digits, hit 2", v.open
      assert_equal "shown", PrivCounter.shown
      assert_equal "still public", PrivCounter.hidden
      assert_equal "one", MtC.new.one
      assert_equal "two", MtC.new.two
      assert_equal "three", MtM::K.new.three
      ks = [MtSquare, MtShape] #: Array[singleton(MtShape)]
      assert_equal "ObjectTests::MtSquare", cname(MtSquare)
      assert_equal "ObjectTests::MtSquare", mname(MtSquare)
      assert_equal "ObjectTests::MtSquare", cname(ks.fetch(0))
      assert_equal "ObjectTests::MtShape", mname(ks.fetch(1))
      counts = {} #: Hash[Class, Integer]
      [MtSquare, MtShape, MtSquare].each { |k| counts[k] = counts.fetch(k, 0) + 1 }
      assert_equal "{ObjectTests::MtSquare => 2, ObjectTests::MtShape => 1}", (counts).inspect
      assert_equal "#<struct ObjectTests::MtPoint x=1, y=0>", (MtPoint.new(1)).inspect
      assert_equal 3, (MtPoint.new(1, 2).sum)
      assert_equal "P#<struct ObjectTests::MtPair a=1, b=2>", ((MtPair.new(1, 2))).inspect
      assert_equal false, (EqPoint.new(1, 2) == EqPoint3.new(1, 2))
      assert_equal false, (EqPoint3.new(1, 2) == EqPoint.new(1, 2))
      assert_equal true, (EqPoint3.new(1, 2) == EqPoint3.new(1, 2))
      assert_equal false, (MtVal.new(1) == SubVal.new(1))
      assert_equal false, (SubVal.new(1) == MtVal.new(1))
      assert_equal "#<data ObjectTests::SubVal v=1>", (SubVal.new(1)).inspect
      assert_equal "#<data ObjectTests::SubVal v=3>", ((SubVal.new(2).with(v: 3))).inspect
      l = MtLoud.new("ann")
      assert_equal "ANN", l.name
      l.name = "bob"
      assert_equal "BOB!", l.name
      vs = [vec(1), vec(2)] #: Array[MtVec]
      assert_equal true, vs.include?(vec(1))
      assert_equal false, vs.include?(vec(3))
      assert_equal true, (vs == [vec(1), vec(2)])
      assert_equal false, (vs == [vec(2), vec(1)])
      h = { a: vec(1) } #: Hash[Symbol, MtVec]
      assert_equal true, (h == { a: vec(1) })
      assert_equal 1, MtBox.new.id
      assert_equal [:test_base, :test_child, :test_shared], PimChild.public_instance_methods(true).grep(/^test_/).sort
      assert_equal [:==, :test_child], PimChild.public_instance_methods(false).sort
      assert_equal [:==, :test_child], PimChild.instance_methods(false).sort
      assert_equal true, PimChild.method_defined?(:test_base)
      assert_equal false, PimChild.public_method_defined?("test_private")
      assert_equal false, (PimChild.method_defined?(:test_base, false))
      pim_k = PimChild #: singleton(PimBase)
      assert_equal ["test_base", "test_child", "test_shared"], pim_k.public_instance_methods.grep(/^test_/).map(&:to_s).sort
      assert_equal [:test_shared], PimShared.instance_methods.sort
    end
  end

  class ObjectModulesTest < Minitest::Test
    def test_section_0
      d = MoDoc.new("readme")
      assert_equal "[readme]", d.label
      assert_equal "[README]", d.shout
      assert_equal "tag:both", MoBoth.new.label
      assert_equal "TAG:BOTH", MoBoth.new.shout
      assert_equal 4, MoBoth.new.tag_len
      assert_equal "<[ov]>", Override.new.label
      assert_equal "<[OV]>", Override.new.shout
      assert_equal "[ann] greets bob", MoPerson.new.greet("bob")
      assert_equal "[ANN]", MoPerson.new.shout
      assert_equal "[]", MoDoc.new("").label
      assert_equal "[ÜNÏ]", MoDoc.new("ünï").shout
      assert_equal "a", MoA.new.hi
      assert_equal "loud", MoB.new.hi
      assert_equal "c/loud", MoC.new.hi
      assert_equal "base", MoC.new.base_only
      assert_equal "[Q]", MoQ.new("q").shout
      assert_equal "[q]", MoQ.new("q").label
      assert_equal ["ALPHA", "BETA", "GAMMA"], (MoRegistry.map { |s| s.upcase })
      assert_equal 3, (MoRegistry.select { |s| s.include?("a") }.size)
      assert_equal "beta", (MoRegistry.find { |s| s.start_with?("b") })
      assert_equal ["alpha", "beta", "gamma"], MoRegistry.to_a
      assert_equal 3, MoRegistry.count
      assert_equal true, MoRegistry.include?("beta")
      assert_equal ["alpha", "beta"], MoRegistry.first(2)
      assert_equal ["alpha", "gamma", "beta"], (MoRegistry.sort_by { |s| -s.size })
      assert_equal 42, Util.double(21)
      assert_equal -12, Util.quad(-3)
      assert_equal 0, Util.quad(0)
      assert_equal "ObjectTests::Util", (Util).to_s
      assert_equal "ObjectTests::Util", Util.name
      assert_equal "Module", (Util.class).to_s
      assert_equal "ObjectTests::MoRegistry", (MoRegistry).inspect
      pe = Member.new("ann")
      jr = Junior.new("kid")
      assert_equal "hi ann!", pe.greet
      assert_equal "ann", pe.touch.touch.name
      assert_equal "HI ANN!!", pe.loud
      assert_equal "hi jr!", jr.touch.greet
      assert_equal "HI JR!!", jr.loud
      twos = [] #: Array[String]
      pe.each_two { |i| twos << "two #{i}" }
      jr.each_two do |i|
        next if i == 1

        twos << "junior two #{i}"
      end
      assert_equal ["two 1", "two 2", "junior two 2"], twos
      assert_equal [50, 60], (pe.mapped(5) { |x| x * 10 })
      assert_equal [-2, -1], (jr.mapped(-1) { |x| x - 1 })
      assert_equal true, pe.is_a?(Greeting)
      assert_equal true, jr.is_a?(Greeting)
      assert_equal true, jr.is_a?(Member)
      assert_equal true, pe.respond_to?(:greet)
      assert_equal false, pe.respond_to?(:suffix)
      assert_equal "hi", Greeting::PREFIX
      assert_equal "ObjectTests::Greeting", Greeting.name
      assert_equal "ObjectTests::Greeting", (Greeting).inspect
      assert_equal "Module", (Greeting.class).to_s
      members = [pe, jr] #: Array[Member]
      assert_equal ["hi ann!", "hi jr!"], members.map(&:greet)
      assert_equal ["ann", "jr"], (members.map { |x| x.touch.name })
    end
  end

  class ObjectNamespacesTest < Minitest::Test
    def test_section_0
      assert_equal ["start", "init first", "init second", "middle", "init third"], NS_TRACE
      assert_equal "outer 2 10 second", Outer::Inner::Leaf.new.where
      assert_equal "top", Outer::Inner::Leaf.new.top
      assert_equal 3, Outer::Derived.new.scale
      assert_equal 6, Outer::Derived.new.doubled
      assert_equal 6, Outer::Derived.new.list_sum
      assert_equal "top", Outer::Compact.new.name_seen
      assert_equal "outer", Outer::Compact.new.qualified
      assert_equal "outer", Outer::Compact.new.reopened
      assert_equal 4, NsOther.new.inherited_scale
      assert_equal 3, NsOther.new.scale
      assert_equal "outer", Outer::OBJECT_NS_NAME
      assert_equal 2, Outer::Inner::DEPTH
      assert_equal 3, Outer::Base::SCALE
      assert_equal [1, 2, 3], Outer::LIST
      assert_equal "{\"a\" => 1}", (Outer::TABLE).inspect
      assert_equal [], Outer::EMPTY
      assert_equal "outer 2 10 second", Outer::ORIGIN.where
      assert_equal 10, ::OBJECT_NS_LIMIT
      assert_equal "first", FIRST
      assert_equal "third", Outer::THIRD
      assert_equal "ObjectTests::Outer", Outer.name
      assert_equal "ObjectTests::Outer::Inner", Outer::Inner.name
      assert_equal "ObjectTests::Outer::Inner::Leaf", Outer::Inner::Leaf.name
      assert_equal "ObjectTests::Outer::Compact", Outer::Compact.name
      assert_equal "ObjectTests::NsOther", NsOther.name
      assert_equal "ObjectTests::Outer::Inner::Leaf", (Outer::Inner::Leaf).to_s
      assert_equal "ObjectTests::Outer::Inner::Leaf", (Outer::Inner::Leaf).inspect
      assert_equal "ObjectTests::Outer::Derived", (Outer::Derived.new.class).to_s
      assert_equal "ObjectTests::Outer::Derived", Outer::Derived.new.class.name
      assert_equal "Module", (Outer.class).to_s
      assert_equal "Module", (Outer::Inner.class).to_s
      assert_equal "Class", (Outer::Base.class).to_s
      Outer::LIST << 4
      assert_equal [1, 2, 3, 4], Outer::LIST
      assert_equal 10, Outer::Derived.new.list_sum
      assert_equal [1, -2], Outer::Factory.make.map(&:v)
      assert_equal -1, Outer::Factory.make.fetch(0).plus(Outer::Factory.make.fetch(1)).v
      assert_equal 3, Outer::Factory.build(Outer::Derived).scale
      assert_equal "ObjectTests::Outer::Derived", (Outer::Factory.build(Outer::Derived).class).to_s
      ex = Expr::Add.new(Expr::Num.new(1), Expr::Add.of(2, 3))
      assert_equal "(1 + (2 + 3))", ex.show
      assert_equal 1, ex.prec
      assert_equal 0, Expr::Num.new(4).prec
      assert_equal 0, Expr.new.prec
      assert_equal "(5 + 6)", (Expr::Add.of(5, 6).show)
      assert_equal "ObjectTests::Expr::Add", Expr::Add.name
      assert_equal "ObjectTests::Expr::Num", (Expr::Num.new(1).class).to_s
      assert_equal 1, Expr::Add::PRECEDENCE
      assert_equal 0, Expr::PRECEDENCE
      assert_equal 81, App::Server.port
      assert_equal "80/81", App::Server::Handler.new.where
      assert_equal "ObjectTests::App::Server::Handler", App::Server::Handler.name
      assert_equal 80, App::Config::PORT
      exprs = [Expr::Num.new(7), ex] #: Array[Expr]
      assert_equal ["7", "(1 + (2 + 3))"], exprs.map(&:show)
      assert_equal [false, true], (exprs.map { |x| x.is_a?(Expr::Add) })
      assert_equal "[ObjectTests::Expr::Num, ObjectTests::Expr::Add]", (exprs.map(&:class)).inspect
      tk = Taker.new
      assert_equal "item", tk.make.hi
      assert_equal 5, (tk.count([tk.make, Holder::Item.new]))
      assert_equal 5, Outer::Inner.vec(4).plus(Outer::Inner.vec(1)).v
    end
  end

  class ObjectOperatorsTest < Minitest::Test
    def test_section_0
      a = OpVec.new(1, 2)
      b = OpVec.new(3, -4)
      assert_equal "(4, -2)", ((a + b)).to_s
      assert_equal "(-2, 6)", ((a - b)).to_s
      assert_equal "(3, 6)", ((a * 3)).to_s
      assert_equal "(-1, -2)", (-a).to_s
      assert_equal "(0, 0)", ((a * 0)).to_s
      assert_equal "(1, 2)", ((a + b - b)).to_s
      assert_equal true, (a == OpVec.new(1, 2))
      assert_equal false, (a == b)
      assert_equal 1, a[0]
      assert_equal 2, a[1]
      assert_equal false, a.zero?
      assert_equal true, (OpVec.new(0, 0).zero?)
      assert_equal "(-1, -2)", (a.negate!).to_s
      assert_equal "[#<OpVec 1,2>, #<OpVec 3,-4>]", (([a, b])).inspect
      assert_equal "#<OpVec 1,2>", (a).inspect
      assert_equal "(1, 2)", "#{a}"
      assert_equal "[[#<OpVec 1,2>]]", ([[a]]).inspect
      a.x = 10
      a[1] = -7
      assert_equal "(10, -7)", (a).to_s
      assert_equal "#<OpVec 10,-7>", (a).inspect
      list = [a, b] #: Array[OpVec]
      assert_equal "[#<OpVec -10,7>, #<OpVec -3,4>]", ((list.map { |v| -v })).inspect
      assert_equal "(23, -18)", ((list.reduce(list.fetch(0)) { |acc, v| acc + v })).to_s
      bag = OpBag.new
      bag << 1 << 2 << 3
      bag << 4
      assert_equal 10, bag.sum
      assert_equal 4, (+bag)
      assert_equal -4, (~bag)
      assert_equal [2, 4], bag.evens
      pl = OpPlain.new
      sh = Shown.new
      th = OpDeep::Thing.new
      assert_equal true, pl.inspect.start_with?("#<ObjectTests::OpPlain")
      assert_equal true, pl.to_s.start_with?("#<ObjectTests::OpPlain")
      assert_equal false, sh.inspect.include?("shown!")
      assert_equal true, sh.inspect.start_with?("#<ObjectTests::Shown")
      assert_equal true, th.to_s.start_with?("#<ObjectTests::OpDeep::Thing")
      assert_equal true, [pl].inspect.start_with?("[#<ObjectTests::OpPlain")
      assert_equal true, "#{pl}".start_with?("#<ObjectTests::OpPlain")
      assert_equal "shown!", "#{sh}"
      c1 = Coin.new(5)
      assert_equal true, (c1 == Coin.new(5))
      assert_equal false, (c1 == Coin.new(6))
      assert_equal false, (c1 == 5)
      assert_equal false, (c1 == "5")
      assert_equal false, (c1 == nil)
      coins = [c1, coin(1)] #: Array[Coin]
      assert_equal true, coins.include?(coin(1))
      assert_equal false, coins.include?(coin(2))
      assert_equal 2, coins.uniq.size
      assert_equal 2, ([c1, coin(5)].uniq.size)
      assert_equal 6, (list.fetch(1).then { |v| v.x * 2 })
      assert_equal "c=3", (coin(3).then { |c| "c=#{c.cents}" })
      assert_equal 6, (coins.reduce(coin(0)) { |a, c| coin(a.cents + c.cents) }.cents)
    end
  end

  class ObjectOverrideChainTest < Minitest::Test
    def test_section_0
      assert_equal "base 1", OcBase.new.f(1)
      assert_equal "sub 3 base 3", OcSub.new.f
      assert_equal "sub 5 base 5", OcSub.new.f(5)
      assert_equal "subsub", SubSub.new.f
      assert_equal "subsub", OcLeaf.new.f
      assert_equal "sub 7 base 7", OcSub.new.call_f
      shown = [OcBase.new, OcSub.new, SubSub.new, OcLeaf.new].map { |b| object_show(b) }
      assert_equal ["base base base", "sub sub sub", "subsub subsub subsub", "subsub subsub subsub"], shown
      s = OcSub.new
      assert_equal false, s.same?(s)
      assert_equal false, OcBase.new.same?(s)
      assert_equal "only", OcLeaf.new.me.only
      assert_equal "only", OcLeaf.new.dup_me.only
      x = OcSub.new #: untyped
      assert_equal "sub 2 base 2", x.f(2)
      assert_equal "sub 3 base 3", x.f
      assert_equal "OcBase.make x", OcBase.make("x")
      assert_equal "OcSub.make", OcSub.make
      k = OcSub #: singleton(OcBase)
      e = assert_raises(ArgumentError) { k.make("y") }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
      e = assert_raises(ArgumentError) { SubSub.new.call_f }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
    end
  end

  class ObjectStructDataTest < Minitest::Test
    def test_section_0
      origin = SdPoint.new(0, 0)
      pt = SdPoint.new(3, -4)
      assert_equal 3, pt.x
      assert_equal -4, pt.y
      assert_equal "#<struct ObjectTests::SdPoint x=3, y=-4>", (pt).inspect
      assert_equal "#<struct ObjectTests::SdPoint x=3, y=-4>", pt.to_s
      assert_equal "#<struct ObjectTests::SdPoint x=0, y=0>", (origin).inspect
      pt.x = 10
      pt.y = pt.y - 1
      assert_equal "#<struct ObjectTests::SdPoint x=10, y=-5>", (pt).inspect
      assert_equal [10, -5], pt.to_a
      assert_equal "{x: 10, y: -5}", (pt.to_h).inspect
      assert_equal [:x, :y], pt.members
      assert_equal [:x, :y], SdPoint.members
      assert_equal true, (pt == SdPoint.new(10, -5))
      assert_equal false, (pt == origin)
      assert_equal true, (pt != origin)
      assert_equal true, (pt == pt)
      assert_equal "#<struct ObjectTests::SdPoint x=1, y=2>", ((SdPoint.new(y: 2, x: 1))).inspect
      assert_equal [7, 8], (SdPoint.new(x: 7, y: 8).to_a)
      assert_equal "ObjectTests::SdPoint", SdPoint.name
      assert_equal "ObjectTests::SdPoint", ((SdPoint.new(1, 1).class)).to_s
      assert_equal "ObjectTests::SdPoint", (SdPoint).inspect
      pair = SdPair.new("ünï", 1.0)
      assert_equal "#<struct ObjectTests::SdPair left=\"ünï\", right=1.0>", (pair).inspect
      assert_equal "ünï", pair.left
      assert_equal "1.0", (pair.right).inspect
      assert_equal "#<struct ObjectTests::SdPair left=\"\", right=-0.5>", ((SdPair.new("", -0.5))).inspect
      pair.left = "quote\"d"
      assert_equal "#<struct ObjectTests::SdPair left=\"quote\\\"d\", right=1.0>", (pair).inspect
      assert_equal "{left: \"quote\\\"d\", right: 1.0}", (pair.to_h).inspect
      assert_equal "#<struct ObjectTests::Rec id=1, name=nil, note=nil>", (Rec.new(1)).inspect
      assert_equal "#<struct ObjectTests::Rec id=2, name=\"two\", note=nil>", ((Rec.new(2, "two"))).inspect
      assert_equal "#<struct ObjectTests::Rec id=3, name=\"three\", note=\"n\">", ((Rec.new(3, "three", "n"))).inspect
      assert_equal "1:-:-", Rec.new(1).label
      assert_equal "2:b:-", (Rec.new(2, "b").label)
      assert_equal "9:-:-", Rec.numbered(9).label
      assert_equal "named 4:d:-", (SdNamed.new(4, "d").label)
      assert_equal "#<struct ObjectTests::SdNamed id=5, name=nil, note=nil>", (SdNamed.new(5)).inspect
      assert_equal "#<struct ObjectTests::SdNamed id=6, name=nil, note=nil>", (SdNamed.numbered(6)).inspect
      assert_equal "#<struct ObjectTests::Rec id=7, name=nil, note=\"kw\">", ((Rec.new(id: 7, note: "kw"))).inspect
      assert_equal true, (SdNamed.new(1) == SdNamed.new(1))
      assert_equal false, (Rec.new(1, "a") == Rec.new(1, "b"))
      o = Rec.new(8)
      o.name = "late"
      assert_equal "8:late:-", o.label
      assert_equal [8, "late", nil], o.to_a
      assert_equal [:id, :name, :note], o.members
      here = SdGeo::Coord.new(lat: 1.5, lng: -2.0)
      there = here.with(lng: 3.25)
      same = here.with
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=-2.0>", (here).inspect
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=3.25>", (there).inspect
      assert_equal "(1.5, -2.0)", (here).to_s
      assert_equal "(1.5, 3.25)", (there).to_s
      assert_equal "-0.5", (here.sum).inspect
      assert_equal "4.75", (there.sum).inspect
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=-2.0>", (same).inspect
      assert_equal true, (same == here)
      assert_equal false, (here == there)
      assert_equal true, (here == SdGeo::Coord.new(1.5, -2.0))
      assert_equal "{lat: 1.5, lng: -2.0}", (here.to_h).inspect
      assert_equal [:lat, :lng], SdGeo::Coord.members
      assert_equal [:lat, :lng], here.members
      assert_equal "ObjectTests::SdGeo::Coord", SdGeo::Coord.name
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=0.0, lng=0.0>", ((here.with(lat: 0.0, lng: 0.0))).inspect
      assert_equal "#<struct ObjectTests::SdGeo::Tag text=\"t\">", (SdGeo::Tag.new("t")).inspect
      assert_equal ["t"], SdGeo::Tag.new("t").to_a
      m = Money.new(1999, "EUR")
      assert_equal "#<data ObjectTests::Money cents=1999, currency=\"EUR\">", (m).inspect
      assert_equal 1999, m.cents
      assert_equal "#<data ObjectTests::Money cents=-5, currency=\"USD\">", ((Money.new(currency: "USD", cents: -5))).inspect
      assert_equal "#<data ObjectTests::Money cents=0, currency=\"EUR\">", ((m.with(cents: 0))).inspect
      assert_equal true, (m == Money.new(1999, "EUR"))
      assert_equal false, (m == m.with(currency: "GBP"))
      assert_equal "{cents: 1999, currency: \"EUR\"}", (m.to_h).inspect
      assert_equal 9000000000000, (Money.new(9_000_000_000_000, "JPY").cents)
      points = [SdPoint.new(2, 1), SdPoint.new(1, 2), SdPoint.new(1, 1)] #: Array[SdPoint]
      assert_equal [[1, 1], [1, 2], [2, 1]], (points.sort_by { |q| (q.x * 10) + q.y }.map(&:to_a))
      assert_equal [2, 1, 1], points.map(&:x)
      assert_equal "[#<struct ObjectTests::SdPoint x=2, y=1>, #<struct ObjectTests::SdPoint x=1, y=1>]", ((points.select { |q| q.y == 1 })).inspect
      w = Wrap.new(Inner.new(1), ["a", "b"], { k: 1 })
      assert_equal "#<struct ObjectTests::Wrap inner=#<struct ObjectTests::Inner a=1>, list=[\"a\", \"b\"], map={k: 1}>", (w).inspect
      assert_equal "[#<struct ObjectTests::Inner a=1>, [\"a\", \"b\"], {k: 1}]", (w.to_a).inspect
      assert_equal "{inner: #<struct ObjectTests::Inner a=1>, list: [\"a\", \"b\"], map: {k: 1}}", (w.to_h).inspect
      k = Kw.new("x", 2, "f", "m", true, 0)
      assert_equal "#<struct ObjectTests::Kw type=\"x\", range=2, func=\"f\", map=\"m\", go=true, select=0>", (k).inspect
      assert_equal "x", k.type
      assert_equal 2, k.range
      assert_equal true, k.go
      k.type = "y"
      assert_equal "{type: \"y\", range: 2, func: \"f\", map: \"m\", go: true, select: 0}", (k.to_h).inspect
      t = SdThing.new
      t.interface = 5
      assert_equal "t", t.type
      assert_equal 5, t.interface
      assert_equal "len", t.len
      assert_equal "string", t.string
      assert_equal "default", t.default
      assert_equal "#<data ObjectTests::SdVal v=-0.0>", (SdVal.new(-0.0)).inspect
      assert_equal "#<data ObjectTests::SdVal v=1.0e+20>", (SdVal.new(1e20)).inspect
      assert_equal "#<data ObjectTests::SdVal v=0.3333333333333333>", ((SdVal.new(1.0 / 3))).inspect
      list = ListNode.new(1, ListNode.new(2, ListNode.new(3)))
      assert_equal 6, object_total(list)
      assert_equal "#<struct ObjectTests::ListNode val=2, nxt=#<struct ObjectTests::ListNode val=3, nxt=nil>>", (list.nxt).inspect
      assert_equal "#<struct ObjectTests::ListNode val=9, nxt=nil>", (ListNode.new(9)).inspect
      assert_equal 0, object_total(nil)
      list.nxt = nil
      assert_equal 1, object_total(list)
      assert_equal true, (list == ListNode.new(1))
      tr = Tree.new(1, [Tree.new(2, []), Tree.new(3, [Tree.new(4, [])])])
      assert_equal 10, tsum(tr)
      assert_equal 2, tr.kids.size
      assert_equal "#<struct ObjectTests::Tree val=2, kids=[]>", (tr.kids.fetch(0)).inspect
      p3 = Pt3.new(1, 2, 3)
      assert_equal 6, p3.sum
      assert_equal 1, p3.x
      assert_equal [1, 2], p3.to_a
      assert_equal "#<struct ObjectTests::Pt3 x=1, y=2>", (p3).inspect
      assert_equal [:x, :y], p3.members
      assert_equal 3, p3.z
      p3.x = 10
      assert_equal 15, p3.sum
      assert_equal "{x: 10, y: 2}", (p3.to_h).inspect
      assert_equal [:x, :y], Pt3.members
      assert_equal "ObjectTests::Pt3", Pt3.name
    end

    # Struct instances are references: a second name sees the mutation; equal? is identity, == is by value.
    def test_struct_instances_are_references_a
      pa = SdPoint.new(1, 2)
      pb = pa
      pb.x = 9
      assert_equal 9, pa.x
      assert_equal true, pa.equal?(pb)
      assert_equal false, (pa.equal?(SdPoint.new(9, 2)))
      assert_equal true, (pa == SdPoint.new(9, 2))
      assert_equal false, (pa == nil)
      assert_equal false, (pa == 9)
      assert_equal false, (pa == [9, 2])
      assert_equal false, (Money.new(1, "x") == SdPoint.new(1, 2))
      assert_equal true, (pa != nil)
    end
  end

  class ObjectVisibilityTest < Minitest::Test
    def test_section_0
      v = ViVault.new("v")
      assert_equal "v opened with **** true", v.open
      assert_equal "ok false", v.status
      assert_equal "terc3s", v.reveal
      assert_equal "top4", v.via_top
      b = BigVault.new("b")
      assert_equal "b opened with **** true", b.open
      assert_equal "big ok false", b.status
      assert_equal "****|1234|s3cret", b.deep
      assert_equal "terc3s", b.reveal
      assert_equal true, v.respond_to?(:open)
      assert_equal false, v.respond_to?(:pin)
      assert_equal true, v.respond_to?(:status)
      assert_equal "top-1", top_helper(-1)
      d = Dial.new
      assert_equal [1, 2], d.scaled(1)
      assert_equal 5, d.set(5)
      assert_equal [60, 70], d.scaled(10)
      assert_equal false, d.respond_to?(:pin=)
    end
  end

  # Forwardable (decision 99).
  class Deck
    extend Forwardable
    include Enumerable #[Integer]

    # @rbs @cards: Array[Integer]

    def_delegators :@cards, :size, :each, :<<, :first, :[]
    def_delegator :@cards, :join, :to_text
    def_delegator :tally, :fetch, :count_of
    delegate %i[keys] => :@marks

    #: () -> void
    def initialize
      @cards = [3, 1, 2]
      @marks = { "a" => 1 } #: Hash[String, Integer]
    end

    #: () -> Hash[Integer, Integer]
    def tally = @cards.tally
  end

  # A target whose block is optional (Homebrew's `delegate livecheck: :"self.class"`): the forwarder passes on the
  # block, or none
  class FwdSettings
    attr_accessor :level #: Integer

    #: () -> void
    def initialize = @level = 0

    #: () ?{ (FwdSettings) -> void } -> FwdSettings
    def configure
      yield self if block_given?
      self
    end

    #: () ?{ () [self: FwdSettings] -> void } -> FwdSettings
    def tune(&block)
      instance_eval(&block) if block
      self
    end
  end

  # A class-level `Settings?` configured by instance_eval of a self-bound block (Homebrew's Formula.livecheck)
  class FwdConfigured
    # @rbs self.@settings: FwdSettings?
    @settings = FwdSettings.new

    #: () ?{ () [self: FwdSettings] -> void } -> FwdSettings?
    def self.settings(&block)
      return @settings unless block

      @settings.instance_eval(&block)
      @settings
    end
  end

  class FwdApp
    extend Forwardable

    # @rbs @settings: FwdSettings

    def_delegators :@settings, :configure, :tune

    #: () -> void
    def initialize = @settings = FwdSettings.new
  end

  # decision 320: a delegation through a nilable accessor raises NoMethodError on nil, as MRI's forwarder does
  class FwdLazy
    extend Forwardable
    def_delegators :@cards, :size, :first
    def_delegator :@cards, :join, :text

    #: (bool) -> void
    def initialize(ready)
      @cards = ready ? [3, 1, 2] : nil #: Array[Integer]?
    end
  end

  class ForwardableTest < Minitest::Test
    def test_nilable_accessor
      assert_equal [3, 3, "3-1-2"], [FwdLazy.new(true).size, FwdLazy.new(true).first, FwdLazy.new(true).text("-")]
      e = assert_raises(NoMethodError) { FwdLazy.new(false).size }
      assert_includes e.message, "undefined method 'size' for nil"
    end

    def test_optional_block_delegator
      app = FwdApp.new
      assert_equal 0, app.configure.level
      app.configure { |s| s.level = 3 }
      assert_equal 3, app.configure.level
      app.tune { self.level = 7 }
      assert_equal 7, app.tune.level
      FwdConfigured.settings { self.level = 9 }
      assert_equal 9, FwdConfigured.settings&.level
    end

    def test_delegators
      d = Deck.new
      d << 5
      assert_equal [4, 3, 1, 5], [d.size, d.first, d[1], d[3]]
      assert_equal "3-1-2-5", d.to_text("-")
      assert_equal "3125", d.to_text
      assert_equal [1, 2, 3, 5], d.sort
      assert_equal 11, d.sum
      assert_equal 1, d.count_of(3)
      assert_equal 0, d.count_of(9, 0)
      assert_equal ["a"], d.keys
      assert_equal "key not found: 9", assert_raises(KeyError) { d.count_of(9) }.message
    end
  end

  # SingleForwardable (decision 132): the same forms define class methods.
  module FwdLog
    extend SingleForwardable
    def_delegator :$stderr, :puts, :say
  end

  module FwdRegistry
    extend SingleForwardable

    # @rbs self.@items: Array[String]

    def_delegators :@items, :size, :first
    def_single_delegator :@items, :push, :add
    single_delegate %i[last] => :@items
    def_delegator :names, :join
    def_single_delegators :"ObjectTests::FwdRegistry::LIMITS", :fetch, :key?
    def_delegator "Math", :sqrt

    LIMITS = { "a" => 1 } #: Hash[String, Integer]

    #: () -> void
    def self.reset
      @items = []
    end

    #: () -> Array[String]
    def self.names = @items.map(&:upcase)
  end

  class FwdBoth
    extend Forwardable
    extend SingleForwardable

    def_delegator :@count, :+, :plus
    def_instance_delegator :@inner, :size

    #: () -> void
    def self.setup
      @count = 10 #: Integer
    end

    #: () -> void
    def initialize
      @inner = [1, 2] #: Array[Integer]
    end
  end

  class FwdLater
    extend SingleForwardable
    extend Forwardable
    def_delegators :@v, :size

    #: () -> void
    def initialize
      @v = "abc" #: String
    end
  end

  class SingleForwardableTest < Minitest::Test
    def test_single_delegators
      _, err = capture_io { FwdLog.say "hi" }
      assert_equal "hi\n", err
      FwdRegistry.reset
      FwdRegistry.add("x")
      FwdRegistry.add("y")
      assert_equal [2, "x", ["x"], "y", "X-Y"], [FwdRegistry.size, FwdRegistry.first, FwdRegistry.first(1), FwdRegistry.last, FwdRegistry.join("-")]
      assert_equal [1, 2, true, 4.0], [FwdRegistry.fetch("a"), FwdRegistry.fetch("b", 2), FwdRegistry.key?("a"), FwdRegistry.sqrt(16)]
      FwdBoth.setup
      assert_equal [15, 2, 3], [FwdBoth.plus(5), FwdBoth.new.size, FwdLater.new.size]
    end
  end

  # Gaps the sealed Type sum types turned up (decision 119): a call on a void
  # method's nil, `.class` on a tuple, `is_a?` on a generic value.
  class SumTypeGapsTest < Minitest::Test
    #: () -> void
    def nothing
    end

    # @rbs [T] (T) -> bool
    def string?(x) = x.is_a?(String)

    def test_void_value_is_nil
      assert_equal true, nothing.nil?
      assert_equal "NilClass", nothing.class.name
      assert_equal true, nothing.is_a?(NilClass)
      assert_equal "nil", nothing.inspect
    end

    def test_tuple_class
      pair = [1, "a"] #: [Integer, String]
      assert_equal "Array", pair.class.name
    end

    def test_generic_is_a
      assert_equal true, string?("a")
      assert_equal false, string?(1)
      assert_equal false, string?(:s)
      assert_equal true, int?(1)
    end

    # @rbs [T] (T) -> bool
    def int?(x) = x.is_a?(Integer)
  end

  # A local assigned two sibling classes takes their common superclass (decision 116).
  class SiblingAssignTest < Minitest::Test
    def test_sibling_reassignment
      e = IOError.new("a")
      assert_equal "a", e.message
      e = ArgumentError.new("b")
      assert_equal "ArgumentError", e.class.name
      assert_equal "b", e.message
    end
  end

  # Exception#backtrace (decision 106): MRI 4.0's labels, first frame exact.
  class BacktraceTest < Minitest::Test
    #: (Exception) -> Array[String]
    def frames(e) = e.backtrace || []

    #: () -> void
    def deep
      [1].each { |x| [x].each { raise IOError, "deep" } }
    end

    #: () -> void
    def reraise
      deep
    rescue IOError => e
      raise e
    end

    #: () -> void
    def wrap
      deep
    rescue IOError
      raise ArgumentError, "wrapped"
    end

    def test_frames
      e = assert_raises(IOError) { deep }
      bt = frames(e)
      assert_match(/object_test\.rb:\d+:in 'block \(2 levels\) in ObjectTests::BacktraceTest#deep'\z/, bt.fetch(0))
      assert_match(/object_test\.rb:\d+:in 'Array#each'\z/, bt.fetch(1))
      assert_match(/object_test\.rb:\d+:in 'block in ObjectTests::BacktraceTest#deep'\z/, bt.fetch(2))
      assert_match(/object_test\.rb:\d+:in 'Array#each'\z/, bt.fetch(3))
      assert_match(/object_test\.rb:\d+:in 'ObjectTests::BacktraceTest#deep'\z/, bt.fetch(4))
      assert_match(/object_test\.rb:\d+:in 'block in ObjectTests::BacktraceTest#test_frames'\z/, bt.fetch(5))
      assert_equal bt.fetch(0).split(":").fetch(1), bt.fetch(1).split(":").fetch(1) # Array#each at the block's line
    end

    def test_reraise_keeps_frames_and_a_handler_starts_new_ones
      e = assert_raises(IOError) { reraise }
      assert_match(/in 'block \(2 levels\) in ObjectTests::BacktraceTest#deep'\z/, frames(e).fetch(0))
      e2 = assert_raises(ArgumentError) { wrap }
      assert_match(/object_test\.rb:\d+:in 'ObjectTests::BacktraceTest#wrap'\z/, frames(e2).fetch(0))
      assert_match(/in 'block in ObjectTests::BacktraceTest#test_reraise_keeps_frames_and_a_handler_starts_new_ones'\z/, frames(e2).fetch(1))
      assert_equal IOError, e2.cause.class
    end

    def test_set_backtrace_and_never_raised
      made = RuntimeError.new("made")
      assert_nil made.backtrace
      made.set_backtrace(["x.rb:1:in 'Object#y'"])
      assert_equal ["x.rb:1:in 'Object#y'"], made.backtrace
      e = assert_raises(RuntimeError) { raise made }
      assert_equal ["x.rb:1:in 'Object#y'"], e.backtrace
    end
  end

  # ruby/spec language/alias and core/struct gaps (#49)
  class AliasGreeter
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def hello = "hi #{name}"
    alias greet hello
    alias_method :salute, :hello
    alias_method "title", :name

    #: () -> String
    def hello = "hello #{name}"
  end

  class AliasSub < AliasGreeter
    #: () -> String
    def greet = "sub " + super
  end

  # #52: alias of an inherited method keeps the ancestor's body, even after an override
  module AliasWave
    #: () -> String
    def wave = "wave"
  end

  class AliasKid < AliasGreeter
    include AliasWave
    alias kid_hello hello
    alias_method :kid_wave, :wave

    #: () -> String
    def hello = "kid"
  end

  RubySpecPoint = Struct.new(:x, :y) #: [Integer, String]

  class ObjectRubySpecTest < Minitest::Test
    def test_alias
      g = AliasGreeter.new("ann")
      assert_equal ["hello ann", "hi ann", "hi ann", "ann"], [g.hello, g.greet, g.salute, g.title]
      assert_equal "sub hi bo", AliasSub.new("bo").greet
    end

    def test_alias_inherited
      k = AliasKid.new("cy")
      assert_equal ["kid", "hello cy", "wave"], [k.hello, k.kid_hello, k.kid_wave]
    end

    def test_struct_enumeration
      pt = RubySpecPoint.new(1, "a")
      assert_equal [2, 2, [1, "a"], [1, "a"]], [pt.size, pt.length, pt.values, pt.deconstruct]
      seen = [] #: Array[untyped]
      pt.each { |v| seen << v }
      pt.each_pair { |k, v| seen << [k, v] }
      assert_equal [1, "a", [:x, 1], [:y, "a"]], seen
    end
  end

  # ruby/spec language gaps (#49): visibility forms, undef, class structure, keyword parameters
  class VisConfig
    class << self
      #: () -> String
      def default_name = "cfg"

      private

      #: () -> Integer
      def secret = 42
    end

    #: () -> Integer
    def self.reveal = secret
  end

  module VisUtil
    module_function

    #: (Integer) -> Integer
    def double(x) = x * 2
  end

  module VisHelpers
    #: (String) -> String
    def shout(s) = s.upcase
    module_function :shout
  end

  class VisAccount
    #: (Integer) -> void
    def initialize(balance)
      @balance = balance
    end

    #: (VisAccount) -> bool
    def richer_than?(other) = balance > other.balance

    protected

    #: () -> Integer
    def balance = @balance
  end

  class VisFactory
    #: () -> VisFactory
    def self.build = new
    private_class_method :new
    LIMIT = 3 #: Integer
    private_constant :LIMIT
    #: () -> Integer
    def limit = LIMIT

    # a private Struct constant: its generated `==` still names it (Homebrew's Vulns::Scanner::Target)
    Pair = Struct.new(:a, :b, keyword_init: true)
    private_constant :Pair

    #: () -> bool
    def pairs_equal? = Pair.new(a: 1, b: 2) == Pair.new(a: 1, b: 2)
  end

  class VisBase
    #: () -> String
    def greet = "base"
  end

  class VisKid < VisBase
    #: () -> String
    def greet = "kid"
    remove_method :greet
  end

  class VisDog < VisBase; end
  class VisCat < VisBase; end

  class KwPoint
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (x: Integer, ?y: Integer) -> void
    def initialize(x:, y: 0)
      @x = x
      @y = y
    end

    #: (?by: Integer) -> KwPoint
    def shift(by: 1) = KwPoint.new(x: x + by, y: y + by)
  end

  class KwBase
    #: (Integer, ?label: String) -> String
    def show(n, label: "n") = "#{label}=#{n}"
  end

  class KwKid < KwBase
    #: (Integer, ?label: String) -> String
    def show(n, label: "k") = super + "!"
  end

  class ObjectRubySpecLanguageTest < Minitest::Test
    def test_visibility_forms
      assert_equal ["cfg", 42, 8, "HI", true, 3], [VisConfig.default_name, VisConfig.reveal, VisUtil.double(4), VisHelpers.shout("hi"), VisAccount.new(10).richer_than?(VisAccount.new(5)), VisFactory.build.limit]
      assert_equal "base", VisKid.new.greet
      assert_nil defined?(VisFactory::LIMIT) # #53: private: no path reaches it
      assert VisFactory.build.pairs_equal?
    end

    def test_class_structure
      assert_equal [VisCat, VisDog, VisKid], VisBase.subclasses
      assert_equal [VisBase, Object], [VisDog.superclass, VisBase.superclass]
      assert_equal StandardError, ArgumentError.superclass
    end

    #: (String, ?greeting: String, punct: String) -> String
    def object_greet(name, greeting: "hello", punct:) = "#{greeting} #{name}#{punct}"

    # @rbs a: Integer
    # @rbs b: Integer
    # @rbs return: Integer
    def object_kw_add(a:, b: a * 2) = a + b

    #: (**Integer opts) -> Array[Symbol]
    def object_keys_of(**opts) = opts.keys

    #: (Integer, *Integer, scale: Integer, **String rest) -> String
    def object_mix(first, *more, scale:, **rest) = "#{(first + more.sum) * scale} #{rest.inspect}"

    def test_keyword_parameters
      assert_equal ["hello ann!", "hi bo?", "yo cy."], [object_greet("ann", punct: "!"), object_greet("bo", greeting: "hi", punct: "?"), object_greet("cy", punct: ".", greeting: "yo")]
      assert_equal [3, 6, 4], [object_kw_add(a: 1), object_kw_add(a: 1, b: 5), object_kw_add(b: 1, a: 3)]
      assert_equal [[:x, :y], [], [:q], [:a, :b]], [object_keys_of(x: 1, y: 2), object_keys_of, object_keys_of(**{ q: 1 }), object_keys_of(**{ a: 1 }, b: 2)]
      assert_equal ["12 {tag: \"a\"}", "4 {}"], [object_mix(1, 2, 3, scale: 2, tag: "a"), object_mix(4, scale: 1)]
      pt = KwPoint.new(x: 1).shift(by: 2)
      assert_equal [3, 2, 6], [pt.x, pt.y, KwPoint.new(y: 5, x: 2).shift.y]
      assert_equal ["k=3!", "z=4!", "n=5"], [KwKid.new.show(3), KwKid.new.show(4, label: "z"), KwBase.new.show(5)]
    end
  end

  # ruby/spec core/kernel gaps (#49): instance variable reflection
  class IvarPt
    attr_reader :x #: Integer
    attr_accessor :label #: String?

    #: (Integer) -> void
    def initialize(x)
      @x = x
      @tags = [] #: Array[String]
      @label = nil
    end
  end

  class ObjectRubySpecIvarTest < Minitest::Test
    def test_instance_variables
      pt = IvarPt.new(3)
      assert_equal [[:@x, :@tags, :@label], 3, [], nil], [pt.instance_variables, pt.instance_variable_get(:@x), pt.instance_variable_get("@tags"), pt.instance_variable_get(:@nope)]
      assert_equal [true, false], [pt.instance_variable_defined?(:@x), pt.instance_variable_defined?(:@nope)]
      pt.instance_variable_set(:@x, 9)
      pt.instance_variable_set(:@label, "hi")
      assert_equal [9, "hi"], [pt.x, pt.label]
      e = assert_raises(NameError) { pt.instance_variable_get(:x) }
      assert_equal "'x' is not allowed as an instance variable name", e.message
      assert_equal [], 5.instance_variables
    end
  end

  # #53: whether an ivar was assigned is a bit on the object (decision 153)
  class IvarMemo
    attr_writer :note #: String?

    #: () -> void
    def initialize
      @hits = 0
    end

    #: () -> Integer
    def value
      return @value || 0 if defined?(@value)

      @value = 42 #: Integer?
      @value || 0
    end

    #: () -> String?
    def seen = defined?(@value)

    #: () -> String?
    def ghost = defined?(@nope)
  end

  class IvarMemoKid < IvarMemo
    #: () -> void
    def initialize
      super
      @extra = nil #: String?
    end

    #: () -> String?
    def hits? = defined?(@hits)
  end

  module IvarCounted
    # @rbs @count: Integer?

    #: () -> String?
    def counted? = defined?(@count)

    #: () -> void
    def bump
      @count = (@count || 0) + 1
    end
  end

  class IvarBox
    include IvarCounted

    #: () -> void
    def initialize
      @w = 1
    end
  end

  class ObjectIvarAssignedTest < Minitest::Test
    def test_defined_ivar
      m = IvarMemo.new
      assert_equal [nil, nil, [:@hits]], [m.seen, m.ghost, m.instance_variables]
      assert_equal [42, "instance-variable", [:@hits, :@value], 42], [m.value, m.seen, m.instance_variables, m.value]
      m.note = nil
      assert_equal [true, false], [m.instance_variable_defined?(:@note), IvarMemo.new.instance_variable_defined?(:@note)]
      assert_nil defined?(@object_top_ivar)
    end

    def test_module_ivar
      b = IvarBox.new
      assert_equal [nil, [:@w]], [b.counted?, b.instance_variables]
      b.bump
      assert_equal ["instance-variable", [:@w, :@count]], [b.counted?, b.instance_variables]
    end

    def test_remove_instance_variable
      m = IvarMemo.new
      m.value
      assert_equal [42, [:@hits], nil], [m.remove_instance_variable(:@value), m.instance_variables, m.seen]
      begin
        m.remove_instance_variable(:@value)
        flunk "no NameError"
      rescue NameError => e
        assert_equal ["instance variable @value not defined", :@value], [e.message, e.name]
      end
      k = IvarMemoKid.new
      assert_equal ["instance-variable", 0], [k.hits?, k.remove_instance_variable(:@hits)]
      assert_equal [nil, [:@extra], nil], [k.hits?, k.instance_variables, k.instance_variable_get(:@hits)]
      k.instance_variable_set(:@hits, 7)
      assert_equal ["instance-variable", 7], [k.hits?, k.instance_variable_get(:@hits)]
      assert_nil k.remove_instance_variable(:@extra)
    end
  end

  # ruby/spec language gaps (#49): class variables
  class CvCounter
    @@count = 0
    @@names = [] #: Array[String]

    #: (String) -> void
    def initialize(name)
      @@count += 1
      @@names << name
    end

    #: () -> Integer
    def self.count = @@count

    #: () -> Array[String]
    def self.names = @@names

    #: () -> void
    def self.reset
      @@count = 0
    end
  end

  class CvSubCounter < CvCounter
    #: () -> Integer
    def peek = @@count
  end

  module CvTally
    @@hits = 10
    #: () -> Integer
    def self.hit = @@hits += 1
  end

  # #52: a class variable first assigned in a method: unset (NameError) until that runs
  class CvLazy
    def self.reset = @@n = 0 #: Integer
    def self.n = @@n
    def self.bump = @@n += 1
    def self.tags = @@tags ||= [] #: Array[String]
  end

  class CvLazyKid < CvLazy
    def self.peek = @@n
  end

  # self::X in a class method is the lexical class's constant
  class SelfConstConf
    LIMIT = 3
    def self.limit = self::LIMIT
  end

  class ObjectSelfConstTest < Minitest::Test
    def test_self_path
      assert_equal 3, SelfConstConf.limit
    end
  end

  class ObjectRubySpecClassVarTest < Minitest::Test
    def test_class_variable_first_assigned_in_method
      e = assert_raises(NameError) { CvLazy.n }
      assert_equal "uninitialized class variable @@n in ObjectTests::CvLazy", e.message
      CvLazy.reset
      CvLazy.bump
      CvLazy.tags << "a"
      assert_equal [1, 1, ["a"]], [CvLazy.n, CvLazyKid.peek, CvLazy.tags]
    end

    def test_class_variables
      CvCounter.reset
      CvCounter.new("a")
      CvSubCounter.new("b")
      assert_equal [2, 3, 3], [CvCounter.count, CvSubCounter.new("c").peek, CvSubCounter.count]
      assert_equal %w[a b c], CvCounter.names.last(3)
      CvCounter.reset
      assert_equal 0, CvCounter.count
      assert_equal [11, 12], [CvTally.hit, CvTally.hit]
    end
  end

  # ruby/spec core/module gaps (#49): ancestors, included_modules, include?
  module AncGreets; end
  module AncMarks; end

  class AncA
    include AncMarks
  end

  class AncB < AncA
    include AncMarks
    include AncGreets
  end

  # A class's own `tap` (Homebrew's Cask#tap: the Tap without a block, Kernel#tap's self with one) passes its void
  # block to super; tap's block value is discarded, so it is typed void (decision 248)
  class TapCask
    def initialize = @tap = "core"

    #: () ?{ (TapCask) -> void } -> untyped
    def tap(&blk)
      return super if block_given?

      @tap
    end
  end

  # A private method a subclass overrides, public or private, dispatches on self as MRI's (decision 253)
  class PvBase
    def initialize = @log = hook
    attr_reader :log

    private

    def hook = "base"
  end

  class PvKid < PvBase
    def hook = "kid"
  end

  class PvKid2 < PvBase
    private

    def hook = "kid2"
  end

  class ObjectPrivateOverrideTest < Minitest::Test
    def test_private_override_dispatches
      assert_equal %w[base kid kid2], [PvBase.new.log, PvKid.new.log, PvKid2.new.log]
    end
  end

  # Module#<, <=, >, >=: ancestry, nil when unrelated (decision 255)
  class McA; end
  class McB < McA; end
  module McM; end
  class McC; include McM; end

  class ObjectModuleCompareTest < Minitest::Test
    def test_module_compare
      assert_equal [true, false, false, true, false, true, true, nil, true], [McB < McA, McA < McB, McA < McA, McA <= McA, McB > McA, McA > McB, McC < McM, McB < McC, McA >= McB]
      using = McB #: Class
      assert_equal true, using.is_a?(Class) && using < McA
      err = begin
        McA < 3
      rescue TypeError => e
        e.class
      end
      assert_equal TypeError, err
    end
  end

  class ObjectTapOverrideTest < Minitest::Test
    def test_tap_override_and_class_through_untyped
      seen = [] #: Array[String]
      c = TapCask.new
      assert_equal "core", c.tap
      back = c.tap { |k| seen << k.class.name.to_s }
      assert_equal [TapCask, ["ObjectTests::TapCask"]], [back.class, seen]
      assert_equal [1, 2], [1, 2].tap { |a| seen << a.size.to_s }
    end
  end

  class ObjectRubySpecAncestorsTest < Minitest::Test
    def test_ancestors
      extra = Object.included_modules - [Kernel] # what other files' libraries mix into Object in one MRI process (pp, json)
      assert_equal [AncB, AncGreets, AncA, AncMarks, Object, Kernel, BasicObject], AncB.ancestors - extra
      kextra = Kernel.ancestors - [Kernel] # MRI prepends <RactorRequire> once any loaded file uses Ractor
      assert_equal [[Object, Kernel, BasicObject], [Comparable], [Kernel], [BasicObject]], [Object.ancestors - extra - kextra, Comparable.ancestors, Kernel.ancestors - kextra, BasicObject.ancestors]
      assert_equal [AncGreets, AncMarks, Kernel], AncB.included_modules - extra
      assert_equal [true, false], [Integer.ancestors.include?(Comparable), AncA.ancestors.include?(AncGreets)]
    end

    # Module#include?: a module among the ancestors, never the class itself or a class
    def test_module_include
      assert_equal [true, true, false, true], [AncB.include?(AncMarks), AncB.include?(AncGreets), AncA.include?(AncGreets), Integer.include?(Comparable)]
      err = begin
        AncB.include?(AncA)
      rescue TypeError => e
        e.class
      end
      assert_equal TypeError, err
      k = AncB #: untyped
      assert_equal [true, false], [k&.include?(AncGreets), Comparable.include?(Kernel)]
    end

    # rugaroo's own IOReadable and IOWritable are left out, as MRI has neither (decision 139)
    def test_no_prelude_modules
      assert_equal [[], []], [File.ancestors.map(&:to_s).grep(/\AIO[A-Z]/), IO.included_modules.map(&:to_s).grep(/\AIO[A-Z]/)]
    end
  end

  # Kernel's module functions are public on Kernel itself (#55, decision 139)
  class ObjectKernelModuleFunctionTest < Minitest::Test
    def test_output
      assert_output("x\nab1\n") do
        Kernel.puts "x"
        Kernel.print "a", "b"
        Kernel.p 1
      end
    end

    def test_values
      assert_equal ["00042", "hi!", 13, 1.5, 0], [Kernel.format("%05d", 42), Kernel.sprintf("%s!", "hi"), Kernel.Integer("12") + 1, Kernel.Float("1.5"), Kernel.rand(1)]
      assert_equal [true, true, false], [Kernel.respond_to?(:puts), Kernel.respond_to?(:format), Kernel.respond_to?(:initialize_copy)]
      assert_equal 7, Kernel.catch(:t) { Kernel.throw :t, 7 }
    end

    def test_control
      e = assert_raises(ArgumentError) { Kernel.raise ArgumentError, "boom" }
      assert_equal "boom", e.message
      n = 0
      Kernel.loop do
        n += 1
        break if n > 2
      end
      assert_equal 3, n
      en = [1, 2].each
      sum = 0
      Kernel.loop { sum += en.next }
      assert_equal 3, sum
    end

    #: () ?{ () -> String } -> String
    def object_kernel_given(&blk)
      Kernel.block_given? ? blk.call : "none"
    end

    def test_block_given_and_public_method
      assert_equal ["blk", "none"], [object_kernel_given { "blk" }, object_kernel_given]
      assert_equal "x", Kernel.public_method(:format).call("x")
    end
  end

  # ruby/spec language gaps (#49): post parameters
  class PostBase
    #: (Integer, *Integer, Integer) -> String
    def f(a, *m, z) = "#{a}#{m}#{z}"
  end

  class PostKid < PostBase
    #: (Integer, *Integer, Integer) -> String
    def f(a, *m, z) = super + "!"
  end

  class ObjectRubySpecPostParamsTest < Minitest::Test
    #: (Integer, ?Integer, Integer) -> Array[Integer]
    def object_om(a, b = 2, c) = [a, b, c]

    #: (Integer, *Integer, Integer) -> String
    def object_mid(a, *rest, z) = "#{a} #{rest} #{z}"

    #: (*String, String) -> String
    def object_last(*xs, y) = "#{xs.size}:#{y}"

    # @rbs a: Integer
    # @rbs b: Integer
    # @rbs c: Integer
    # @rbs return: Integer
    def object_scoped(a, b = a * 10, c) = a + b + c

    def test_post_params
      assert_equal [[1, 2, 3], [1, 5, 3]], [object_om(1, 3), object_om(1, 5, 3)]
      assert_equal ["1 [] 2", "1 [2, 3] 4"], [object_mid(1, 2), object_mid(1, 2, 3, 4)]
      assert_equal ["0:a", "2:c"], [object_last("a"), object_last("a", "b", "c")]
      assert_equal [13, 8], [object_scoped(1, 2), object_scoped(1, 5, 2)]
      assert_equal "1[2]3!", PostKid.new.f(1, 2, 3)
    end
  end

  # ruby/spec language gaps (#49): anonymous *, ** and & forwarding
  class ObjectRubySpecAnonForwardTest < Minitest::Test
    #: (*Integer) -> Integer
    def object_total(*nums) = nums.sum

    #: (*Integer) -> Integer
    def object_fwd(*) = object_total(*)

    #: (**Integer opts) -> Array[Symbol]
    def object_keys_of(**opts) = opts.keys

    #: (**Integer) -> Array[Symbol]
    def object_kw_fwd(**) = object_keys_of(**)

    #: () { (Integer) -> Integer } -> Array[Integer]
    def object_apply(&blk) = [1, 2].map(&blk)

    #: () { (Integer) -> Integer } -> Array[Integer]
    def object_blk_fwd(&) = object_apply(&)

    # @rbs *: Integer
    # @rbs return: Integer
    def object_sum_all(*) = [*].sum

    def test_anonymous_forwarding
      assert_equal [6, 0, [:a, :b], [10, 20], 3], [object_fwd(1, 2, 3), object_fwd, object_kw_fwd(a: 1, b: 2), object_blk_fwd { |x| x * 10 }, object_sum_all(1, 2)]
    end
  end

  # ruby/spec language gaps (#49): def f(...) forwarding
  class FwdP
    #: (Integer, ?y: Integer) -> Integer
    def m(x, y: 0) = x + y
  end

  class FwdC < FwdP
    def m(...) = super(...) * 2
  end

  class FwdSvc
    #: (String, ?times: Integer) -> String
    def run(name, times: 1) = name * times

    #: (Integer) { (Integer) -> Integer } -> Integer
    def twice(x) = yield(yield(x))

    def call(...) = run(...)

    def via(...) = twice(...)
  end

  # #51: leading parameters and arguments around ..., other receivers, prelude literal defaults
  class FwdWrap
    # @rbs @svc: FwdSvc
    def initialize = @svc = FwdSvc.new

    def tagged(tag, ...) = "#{tag}:#{@svc.run(...)}"

    def thrice(...) = @svc.run("z", ...)

    def klass(...) = FwdStatic.pad(...)

    def toss(...) = throw(...)
  end

  class FwdStatic
    #: (String, ?Integer) -> String
    def self.pad(s, n = 3) = s.ljust(n, ".")
  end

  # #51: **h into named keyword parameters, matched at run time
  #: (name: String, ?age: Integer) -> String
  def self.kw_greet(name:, age: 1) = "#{name} #{age}"

  #: (a: Integer, ?b: Integer, ?c: Integer) -> Array[Integer]
  def self.kw_chain(a:, b: a * 2, c: b + 1) = [a, b, c]

  #: (a: Integer, **Integer rest) -> Hash[Symbol, Integer]
  def self.kw_rest(a:, **rest) = rest

  class ObjectKwSplatTest < Minitest::Test
    def test_splat_beside_named_into_rest
      assert_equal({ b: 3 }, ObjectTests.kw_rest(**{ a: 1, b: 3 }, a: 2))
    end

    def test_splat_into_named_keywords
      opts = { name: "ann" }
      assert_equal ["ann 1", "bo 3", "ann 9"], [ObjectTests.kw_greet(**opts), ObjectTests.kw_greet(**{ name: "bo", age: 3 }), ObjectTests.kw_greet(**opts, age: 9)]
      assert_equal [[1, 2, 3], [1, 2, 7], [1, 5, 6]], [ObjectTests.kw_chain(**{ a: 1 }), ObjectTests.kw_chain(**{ a: 1, c: 7 }), ObjectTests.kw_chain(**{ a: 1, b: 5 })]
    end

    def test_splat_missing_and_unknown
      e = assert_raises(ArgumentError) { ObjectTests.kw_greet(**{ age: 2 }) }
      assert_equal "missing keyword: :name", e.message
      e = assert_raises(ArgumentError) { ObjectTests.kw_greet(**{ name: "x", zz: 1 }) }
      assert_equal "unknown keyword: :zz", e.message
    end
  end

  class ObjectRubySpecForwardAllTest < Minitest::Test
    def test_forward_all
      assert_equal [6, 8], [FwdC.new.m(1, y: 2), FwdC.new.m(4)]
      svc = FwdSvc.new
      assert_equal ["abab", "x", 12], [svc.call("ab", times: 2), svc.call("x"), svc.via(3) { |v| v * 2 }]
    end

    def test_forward_leading_and_other_receivers
      w = FwdWrap.new
      assert_equal ["t:abab", "t:a", "zzz", "a..", "a....", nil, 4],
                   [w.tagged("t", "ab", times: 2), w.tagged("t", "a"), w.thrice(times: 3), w.klass("a"), w.klass("a", 5),
                    catch(:k) { w.toss(:k) }, catch(:k) { w.toss(:k, 4) }]
    end
  end

  # a class's own is_a? was folded away (MRI calls it), and a folded is_a? alone as a statement was a Go "is not used" error
  class IsALiar
    #: (untyped) -> bool
    def is_a?(_klass) = true
  end

  class ObjectBugIsAOverrideTest < Minitest::Test
    def test_is_a_override
      assert_equal true, IsALiar.new.is_a?(Integer)
      1.is_a?(String)
      assert_equal false, 1.is_a?(String)
    end
  end

  # a Kernel method on nil held untyped was NoMethodError: nil has no Go methods for the dispatcher to find
  class ObjectBugUntypedNilKernelTest < Minitest::Test
    def test_kernel_method_on_untyped_nil
      x = nil #: untyped
      assert_equal "<NilClass>", x.object_tag
      assert_equal true, x.frozen?
    end
  end

  # #56: Object.new is a plain object, unique by identity
  class ObjectNewTest < Minitest::Test
    def test_object_new
      o = Object.new
      x = Object.new
      assert_equal [true, false, false, true, false, true], [o.equal?(o), o.equal?(x), o == x, o == o, o.eql?(x), o.eql?(o)]
      assert_equal [Object, true, false], [o.class, o.is_a?(Object), o.frozen?]
      h = { o => 1, x => 2 }
      assert_equal [1, 2, 2, 1], [h[o], h[x], [o, x].uniq.size, [o, o].uniq.size]
      assert o.inspect.start_with?("#<Object:0x")
      assert_equal o.inspect, o.to_s
      assert o.object_id != x.object_id
      assert BasicObject.new.equal?(BasicObject.new) == false
    end
  end
  # #56, decision 145: Class.new/Module.new literals are classes declared at compile time
  AnonBase = Class.new do
    def v = 1
  end

  AnonMod = Module.new do
    def mv = 10
  end

  class AnonIncluder
    include AnonMod
  end

  class ClassNewTest < Minitest::Test
    def test_named
      assert_equal ["ObjectTests::AnonBase", 1, 10], [AnonBase.name, AnonBase.new.v, AnonIncluder.new.mv]
    end

    def test_local
      c = Class.new(AnonBase) do
        def w = v + 1
      end
      assert_equal [2, true, true], [c.new.w, c.new.is_a?(AnonBase), c.new.is_a?(c)]
      assert_equal 3, Class.new { def x = 3 }.new.x
      m = Module.new do
        def self.z = 4
      end
      assert_equal 4, m.z
    end

    def test_proc_new
      f = Proc.new { 5 }
      assert_equal 5, f.call
    end
  end

  # decision 147: a module's instance variables live in each includer's object
  module IvarTally
    # @rbs @n: Integer?

    #: () -> Integer
    def bump = (@n = (@n || 0) + 1)
  end

  class IvarHolder
    include IvarTally

    #: () -> Integer?
    def raw = @n
  end

  class IvarHolderKid < IvarHolder
  end

  # decision 322: `@x = v #: T` in a class body declares the class object's @x, as `# @rbs self.@x: T` does
  class BodyRefs
    @refs = {} #: Hash[Symbol, Integer]
    @count = 0 #: Integer

    #: (Symbol) -> Integer
    def self.use(key)
      @count += 1
      @refs[key] ||= @refs.size + 10
    end

    #: () -> Integer
    def self.count = @count
  end

  class BodyRefsTest < Minitest::Test
    def test_class_body_ivars
      assert_equal [10, 11, 10], [BodyRefs.use(:a), BodyRefs.use(:b), BodyRefs.use(:a)]
      assert_equal 3, BodyRefs.count
    end
  end

  # decision 317: a module's ivars are discovered from its own methods, as a class's are
  module IvarDiscovered
    #: () -> Integer
    def hits = (@hits = (@hits || 0) + 1)

    #: (String) -> void
    def label=(s)
      @labels = [] unless @labels
      @labels << s
    end

    #: () -> Array[String]
    def labels = @labels || []
    #: () -> String?
    def last_label = @labels&.last
  end

  class IvarDiscoveredHolder
    include IvarDiscovered
  end

  class ModuleIvarTest < Minitest::Test
    def test_per_object
      a = IvarHolder.new
      b = IvarHolderKid.new
      a.bump
      a.bump
      b.bump
      assert_equal [2, 1, nil], [a.raw, b.raw, IvarHolder.new.raw]
    end

    def test_discovered
      a = IvarDiscoveredHolder.new
      b = IvarDiscoveredHolder.new
      a.hits
      a.hits
      a.label = "x"
      a.label = "y"
      assert_equal [3, 1], [a.hits, b.hits]
      assert_equal [["x", "y"], []], [a.labels, b.labels]
      assert_equal ["y", nil], [a.last_label, b.last_label]
    end
  end

  # decision 148: an ivar narrows like a local after ||=, a write, `if @x`
  # and guards, until a call through self to a method the program defines
  class NarrowBox
    def initialize
      @v = nil #: String?
    end

    def joined
      @v ||= "x"
      File.join(@v, "y")
    end

    def guarded
      return "none" unless @v

      @v.upcase
    end

    def reset = @v = nil

    def after_reset
      @v = "a"
      reset
      @v.nil?
    end

    def counted
      @v = "ab"
      @v += "c"
      @v.length
    end

    def checked = @v.is_a?(String) ? @v.length : 0

    # the checks a narrowed ivar makes redundant still compile, and answer as MRI's
    def redundant
      @v ||= "x"
      [@v || "", @v&.upcase, @v.nil?, @v ? 1 : 2, @v && @v.size]
    end

    def nil_guard
      return 0 if @v.nil?

      @v.size
    end
  end

  # read before any write: MRI's nil, so the ivar is T?
  module NarrowMemo
    def self.names
      unless @names
        @names = %w[a b]
        @names += %w[c]
        @names << "d"
      end
      @names
    end
  end

  class ObjectIvarNarrowTest < Minitest::Test
    def test_narrowing
      b = NarrowBox.new
      assert_equal ["none", "x/y", "X", 1, true, 3], [b.guarded, b.joined, b.guarded, b.checked, b.after_reset, b.counted]
      assert_equal [0, ["x", "X", false, 1, 1], 1], [NarrowBox.new.nil_guard, NarrowBox.new.redundant, NarrowBox.new.tap(&:redundant).nil_guard]
    end

    def test_read_before_write
      assert_equal %w[a b c d], NarrowMemo.names
      assert_equal %w[a b c d], NarrowMemo.names
    end
  end

  # A method returning `self?` keeps the interface-typed forwarder (decision 151).
  class SelfOptNode
    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: () -> self?
    def me = @n > 0 ? self : nil
  end

  class SelfOptReturnTest < Minitest::Test
    def test_self_opt_return
      assert_equal [1, nil], [SelfOptNode.new(1).me&.n, SelfOptNode.new(0).me&.n]
    end
  end

  # A stored block runs later with self rebound by instance_eval (#85).
  class Router
    #: () { () -> String } -> void
    def initialize(&blk)
      @blk = blk
    end

    #: () -> String
    def route
      instance_eval(&@blk)
    end

    #: () -> String
    def hello = "hi"

    #: (Integer) -> Integer
    def double(n) = n * 2
  end

  class ObjectInstanceEvalTest < Minitest::Test
    def test_stored_block_instance_eval
      assert_equal "hi", Router.new { hello }.route
    end

    def test_instance_eval_literal
      r = Router.new { hello }
      assert_equal "hi", r.instance_eval { hello }
    end

    def test_class_eval_defines_method
      assert_equal "from class_eval", CevGreeter.new.greet("class_eval")
    end
  end

  # class_eval in a class body collects its block as class-body code (#85).
  class CevGreeter
    class_eval do
      #: (String) -> String
      def greet(who) = "from #{who}"
    end
  end

  # A class-body if/unless whose condition folds at compile time keeps MRI's branch (#88, #89, decision 167).
  class Guarded
    LIMIT = 3

    if defined?(LIMIT)
      def limit = LIMIT
    end

    if RUBY_VERSION < "2.0"
      def old = "old"
    elsif RUBY_VERSION >= "3.0"
      def era = "modern"
    else
      def era = "middle"
    end

    if RUBY_ENGINE == "truffleruby" # folds: RUBY_ENGINE is "ruby" (prism's serialize.rb, decision 427)
      def engine = "truffle"
    elsif RUBY_ENGINE != "jruby"
      def engine = "mri"
    end

    private

    unless defined?(Nope) || defined?(Process.nope)
      def hidden = "hidden"
    end
  end

  class ObjectClassGuardTest < Minitest::Test
    def test_defined_constant_from_same_body
      assert_equal 3, Guarded.new.limit
    end

    def test_version_elsif
      assert_equal "mri", Guarded.new.engine
      assert_equal "modern", Guarded.new.era
      refute Guarded.new.respond_to?(:old)
    end

    def test_guarded_def_keeps_visibility
      refute Guarded.new.respond_to?(:hidden)
      assert_equal "hidden", Guarded.new.send(:hidden)
    end
  end

  # attr_* and alias inside `class << self` are class-level accessors (#82).
  class ClassSettings
    class << self
      attr_accessor :limit #: Integer?
      attr_reader :label #: String?

      alias max limit
      alias_method :max=, :limit=

      private

      attr_writer :secret #: String
    end

    self.limit = 3
  end

  class SubSettings < ClassSettings
  end

  # attr_* in a module live in the module's ivar struct (decision 147) (#82).
  module AttrTagged
    attr_reader :name #: String
    attr_accessor :tag #: Symbol?

    #: (String) -> void
    def initialize(name)
      @name = name
      @tag = nil
    end

    #: () -> String
    def shout = name.upcase
  end

  class TaggedThing
    include AttrTagged
  end

  class ObjectClassAttrTest < Minitest::Test
    def test_singleton_attr_accessor
      assert_equal 3, ClassSettings.limit
      assert_nil SubSettings.limit
      assert_nil ClassSettings.label
    end

    def test_singleton_alias
      ClassSettings.max = 7
      assert_equal 7, ClassSettings.max
      assert_equal 7, ClassSettings.limit
      ClassSettings.limit = 3
    end

    def test_singleton_private_attr
      refute ClassSettings.respond_to?(:secret=)
    end

    def test_module_attrs
      t = TaggedThing.new("ada")
      assert_equal "ada", t.name
      assert_equal "ADA", t.shout
      assert_nil t.tag
      t.tag = :x
      assert_equal :x, t.tag
    end
  end

  # Object#dup: a shallow copy, then initialize_copy (#82).
  class DupBox
    attr_accessor :n #: Integer
    attr_reader :items #: Array[Integer]
    attr_reader :copied #: bool?

    #: () -> void
    def initialize
      @n = 1
      @items = [1]
      @copied = nil
    end

    #: (DupBox) -> void
    def initialize_copy(other)
      @copied = true
    end
  end

  # A module initialize's super() with nothing after it in any includer is Object's: a no-op (#82).
  module SuperEnv
    attr_reader :env #: Hash[String, String]?

    #: (Hash[String, String]) -> void
    def initialize(env)
      @env = env
      super()
    end
  end

  class SuperEnvUser
    include SuperEnv
  end

  # A class body's locals are visible to its later statements and constants (decision 170).
  class BodyLocals
    factor = 3
    scale = lambda { |x| x * factor } #: ^(Integer) -> Integer
    SCALED = scale.call(4) #: Integer
    NAMES = %w[a b].map { |s| s * factor } #: Array[String]
  end

  # A constant assigned inside a class-body block is the class's, as unicode-display_width's
  # `File.open(INDEX_FILENAME, "rb") { |file| INDEX = Marshal.load(...) }`.
  class BodyBlockConst
    [2, 3].tap do |pair|
      PRODUCT = pair.fetch(0) * pair.fetch(1) #: Integer
    end
    NEXT = PRODUCT + 1
  end

  # `@a = @b = @c = 0` through two untyped ivars (liquid's ResourceLimits#reset): each store boxes the literal (decision 394).
  class ChainedUntypedIvars
    attr_reader :a, :b, :c

    def initialize
      @b = "b" #: untyped
      @c = :c #: untyped
      @a = nil #: Integer?
      @a = @b = @c = 0
    end
  end

  # A class's own `binding` (rubocop-ast's NodePattern::Compiler keeps a Binding of its own as an attr_reader) is an
  # ordinary method, not Kernel#binding (which rugaroo does not support).
  class OwnBinding
    attr_reader :binding

    def initialize
      @binding = { bound: [] }
    end

    def bind(name)
      binding[:bound] << name
      binding[:bound].size
    end
  end

  class ObjectDupTest < Minitest::Test
    def test_own_binding_method_is_ordinary
      b = OwnBinding.new
      assert_equal [1, 2], [b.bind(:x), b.bind(:y)]
      assert_equal({ bound: [:x, :y] }, b.binding)
    end

    def test_chained_assignment_through_untyped_ivars
      o = ChainedUntypedIvars.new
      assert_equal [0, 0, 0], [o.a, o.b, o.c]
      assert_equal Integer, o.b.class
    end

    def test_class_body_block_constant
      assert_equal [6, 7], [BodyBlockConst::PRODUCT, BodyBlockConst::NEXT]
    end

    def test_dup_is_shallow_and_calls_initialize_copy
      a = DupBox.new
      b = a.dup
      b.n = 5
      b.items << 2
      assert_equal [1, 5, true, nil], [a.n, b.n, b.copied, a.copied]
      assert_same a.items, b.items
    end

    def test_dup_of_immediates_and_objects
      assert_equal [3, :s], [3.dup, :s.dup]
      o = Object.new
      refute_same o, o.dup
    end

    def test_module_initialize_super
      assert_equal({ "a" => "b" }, SuperEnvUser.new({ "a" => "b" }).env)
    end

    def test_class_body_locals
      assert_equal 12, BodyLocals::SCALED
      assert_equal %w[aaa bbb], BodyLocals::NAMES
    end

    def test_kernel_string
      assert_equal ["", "12", "a", "s"], [String(nil), String(12), String("a"), String(:s)]
    end
  end

  # Class-body statements each get their own temporaries; `@x = v` there is the class object's ivar (#82).
  class BodyIvars
    class << self
      attr_accessor :limit #: Integer
      attr_accessor :order #: Array[Symbol]
    end

    self.limit = (ENV["RUGAROO_NO_SUCH_LIMIT"] || 128).to_i
    self.limit = (ENV["RUGAROO_NO_SUCH_LIMIT_2"] || limit + 1).to_i
    @order = [:a, :b]
  end

  # dup inside a struct class's own method keeps the class's type.
  class DupSelf
    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: () -> Integer
    def bumped = dup.bump!

    #: () -> Integer
    def bump! = @n += 1
  end

  class ObjectBodyIvarTest < Minitest::Test
    def test_class_body_statements_and_ivars
      assert_equal [129, [:a, :b]], [BodyIvars.limit, BodyIvars.order]
    end

    def test_dup_in_own_method
      d = DupSelf.new(1)
      assert_equal [2, 1], [d.bumped, d.n]
    end
  end

  # rack's multipart MimePart (decision 175).
  class ObjectStructSuperTest < Minitest::Test
    class Part < Struct.new(:body, :name) #: [String, String?]
      def label = "#{name}=#{body}"
    end

    def test_struct_superclass
      pt = Part.new("b", "n")
      assert_equal ["n=b", "b", true], [pt.label, pt.body, pt.is_a?(Struct)]
    end
  end

  class ObjectImplicitConversionLo
    def to_str = "lo"
  end

  class ObjectImplicitConversionTwo
    def to_int = 2
  end

  ObjectUntypedPoint = Struct.new(:x, :y)

  class ObjectNilIvarBase
    def reset
      @fd = nil
    end

    def fd = @fd
  end

  class ObjectNilIvarSub < ObjectNilIvarBase
    def open
      @fd = 3
      @fd + 1
    end
  end

  class ObjectCallerLabels
    def outer = inner
    def inner = caller(0)&.first(2)&.map { |l| l[/in '([^']*)'/, 1] }
    def located = caller_locations(0)&.first(1)&.map { |l| [l.label, l.base_label] }
  end

  class ObjectProtocolHooks
    def foo = true

    def respond_to_missing?(method, priv)
      [:handled].include?(method) || priv
    end
  end

  class ObjectReflectVisibility
    def self.made = 1
    def pub = 1

    private

    def priv = 2
  end

  class ObjectInferredAttrs
    attr_reader :x
    attr_accessor :y

    def initialize(x, y)
      @x = x
      @y = y
    end
  end
  ObjectEmptyData = Data.define
  ObjectKwStruct = Struct.new(:a, :b, keyword_init: true)

  class ObjectImplicitConversionTest < Minitest::Test
    def test_to_str_and_to_int
      assert_equal [true, "abab", "hellolo", [1, 2]],
                   ["hello".include?(ObjectImplicitConversionLo.new), "ab" * ObjectImplicitConversionTwo.new,
                    "hello" + ObjectImplicitConversionLo.new, [1, 2, 3].first(ObjectImplicitConversionTwo.new)]
    end

    def test_private_and_singleton_method_tables
      k = ObjectReflectVisibility
      assert_equal [[:priv], true, [:made], true, true],
                   [k.private_instance_methods(false), k.private_method_defined?(:priv), k.singleton_methods - [:yaml_tag], # yaml_tag: Psych's, when yaml_test loads in the same process
                    Kernel.private_instance_methods(false).include?(:puts), Kernel.public_methods(false).include?(:sprintf)]
    end

    def test_superclass_nil_ivar_typed_by_subclass
      s = ObjectNilIvarSub.new
      s.reset
      a = s.fd
      assert_equal [nil, 4, 3], [a, s.open, s.fd]
    end

    def test_nilclass_method_on_optional_receiver
      none = [1].find { |v| v > 5 }
      one = [1].find { |v| v > 0 }
      assert_equal [:nil, :int], [none.object_tests_kind, one.object_tests_kind]
    end

    def test_unannotated_protocol_hooks
      o = ObjectProtocolHooks.new
      assert_equal [true, true, false], [o.method(:foo).call, o.respond_to?(:handled), o.respond_to?(:other)]
    end

    def test_caller_labels
      assert_equal ["ObjectTests::ObjectCallerLabels#inner", "ObjectTests::ObjectCallerLabels#outer"], ObjectCallerLabels.new.outer
      assert_equal [["ObjectTests::ObjectCallerLabels#located", "located"]], ObjectCallerLabels.new.located
    end

    def test_struct_keyword_init
      assert_equal ["#<struct ObjectTests::ObjectKwStruct a=1, b=2>", true, nil, nil],
                   [ObjectKwStruct.new(a: 1, b: 2).inspect, ObjectKwStruct.keyword_init?, ObjectUntypedPoint.keyword_init?, ObjectKwStruct.new(a: 1).b]
    end

    def test_unannotated_attrs
      a = ObjectInferredAttrs.new(1, "a")
      assert_equal [2, "A"], [a.x + 1, a.y.upcase]
    end

    def test_untyped_and_anonymous_value_classes
      pt = ObjectUntypedPoint.new(1, "a")
      anon = Struct.new(:a, :b)
      s = anon.new(1, 2)
      s[:b] = 5
      d = Data.define(:v).new(v: 3)
      assert_equal [1, "a", [1, 5], [5], 3], [pt.x, pt.y, s.to_a, s.values_at(1), d.v]
      e = assert_raises(IndexError) { s[2] }
      assert_equal "offset 2 too large for struct(size:2)", e.message
      assert_equal [[], {}, true], [ObjectEmptyData.members, ObjectEmptyData.new.to_h, ObjectEmptyData.new == ObjectEmptyData.new]
    end
  end

  # prepend: the module's method runs first, and its super reaches the class's own (decision 199)
  module ObjLoud
    #: () -> String
    def greet = super.upcase + "!"
  end

  class ObjGreeter
    prepend ObjLoud

    #: () -> String
    def greet = "hi"
  end

  # `class << self; prepend M; end` is singleton_class.prepend(M) (Homebrew's `T`, prepending TNoChecks)
  module ObjQuiet
    #: (String) -> String
    def shout(s) = super.downcase
  end

  class ObjCrier
    class << self
      prepend ObjQuiet
    end

    #: (String) -> String
    def self.shout(s) = s + "!"
  end

  # a `-> self` method calling an inherited `-> self` one on self (Homebrew's MacOSVersion#strip_patch)
  class ObjVer
    #: (String) -> void
    def initialize(s)
      @s = s
    end

    #: () -> self
    def major = dup

    #: () -> String
    def to_s = @s
  end

  class ObjMacVer < ObjVer
    #: () -> self
    def strip = major
  end

  # a module the superclass already includes, included again through another module, keeps its place after the
  # superclass, as MRI's include skips it (Homebrew's CurlDownloadStrategy: AbstractDownloadStrategy#ohai, not
  # Utils::Output::Mixin's through Utils::Curl)
  module ObjSay
    #: () -> String
    def say = "mixin"
  end

  module ObjVia
    include ObjSay
  end

  class ObjSayBase
    include ObjSay

    #: () -> String
    def say = "base+" + super
  end

  class ObjSayKid < ObjSayBase
    include ObjVia
  end

  # a module_function's call to another dispatches on self: an includer's override runs (Homebrew's Utils::Curl
  # curl_download calling CurlDownloadStrategy#curl)
  module ObjFetchUtil
    module_function

    #: (String) -> String
    def get(url) = "get #{url}"

    #: (String) -> String
    def download(url) = get(url) + "!"
  end

  class ObjFetcher
    include ObjFetchUtil

    #: (String) -> String
    def get(url) = "auth " + super

    #: () -> String
    def fetch = download("u")
  end

  # an includer's private override of a module's private method runs from the module's other methods
  # (Homebrew's UnpackStrategy::GenericUnar#extract_to_dir)
  module ObjUnpack
    #: (String) -> String
    def extract(to) = "#{extract_to_dir(to)}!"

    private

    #: (String) -> String
    def extract_to_dir(to) = "base #{to}"
  end

  class ObjUnar
    include ObjUnpack

    private

    #: (String) -> String
    def extract_to_dir(to) = "unar #{to}"
  end

  class ObjPlainUnpack
    include ObjUnpack
  end

  module ObjUtil
    extend self

    #: () -> Integer
    def one = 1
  end

  class ObjHost
    #: () -> String
    def ObjHost.hi = "hi"
  end

  class ObjStack < Array
    #: () -> Integer
    def doubled_size = size * 2
  end

  class ObjName < String
    #: () -> String
    def hi = "hi " + self
  end

  module ObjNums
    extend Enumerable

    #: () { (Integer) -> void } -> void
    def self.each
      yield 3
      yield 1
    end
  end

  class ObjRestBase
    #: (*untyped) -> String
    def show(*args) = "base#{args.size}"

    #: (String) -> nil
    def cause(_) = nil

    #: (Integer) -> Integer
    def twice(_) = _ * 2
  end

  # an override of a `*args` method by positional ones answers by argument count
  class ObjRestKid < ObjRestBase
    #: (?Integer?) -> String
    def show(n = nil) = "kid#{n.inspect}"
  end

  class ObjMixinFeaturesTest < Minitest::Test
    def test_rest_override_and_ignored_param
      xs = [ObjRestBase.new, ObjRestKid.new] #: Array[ObjRestBase]
      assert_equal %w[base0 kidnil], xs.map(&:show)
      assert_equal %w[base1 kid3], xs.map { |x| x.show(3) }
      k = ObjRestKid.new #: ObjRestBase
      begin
        k.show(1, 2)
        flunk "no ArgumentError"
      rescue ArgumentError => e
        assert_equal "wrong number of arguments (given 2, expected 0..1)", e.message
      end
      assert_nil ObjRestKid.new.cause("x")
      assert_equal 8, ObjRestKid.new.twice(4)
    end

    def test_prepend
      assert_equal "HI!", ObjGreeter.new.greet
      assert_equal [ObjLoud, ObjGreeter], ObjGreeter.ancestors.take(2)
      assert_equal "hey!", ObjCrier.shout("HEY")
      assert_equal ["a", ObjMacVer], [ObjMacVer.new("a").strip.to_s, ObjMacVer.new("b").strip.class]
      assert_equal ["base+mixin", [ObjSayKid, ObjVia, ObjSayBase, ObjSay]], [ObjSayKid.new.say, ObjSayKid.ancestors.take(4)]
      assert_equal ["auth get u!", "get v!"], [ObjFetcher.new.fetch, ObjFetchUtil.download("v")]
      assert_equal ["unar d!", "base e!"], [ObjUnar.new.extract("d"), ObjPlainUnpack.new.extract("e")]
    end

    def test_extend_self_and_constant_singleton
      assert_equal 1, ObjUtil.one
      assert_equal "hi", ObjHost.hi
    end

    def test_core_subclasses
      s = ObjStack.new
      s << 1
      s.push(2)
      assert_equal 4, s.doubled_size
      assert_equal "hi bob", ObjName.new("bob").hi
    end

    def test_extend_enumerable_from_each
      assert_equal [1, 3], ObjNums.sort
    end

    def test_optional_block_param
      out = []
      [1, 2].each { |x = 0| out << x }
      assert_equal [1, 2], out
    end
  end

  # concurrent-ruby's Obligation: a module whose `wait` returns self, called from its other methods, and overridden
  # with super in an includer (decision 217).
  module Obligation
    # @rbs @state: Symbol
    # @rbs @value: Integer?

    def complete?
      %i[fulfilled rejected].include?(@state)
    end

    def value
      wait
      @value
    end

    def wait
      @state = :fulfilled unless complete?
      self
    end
  end

  class OblIVar
    include Obligation

    def initialize(v)
      @value = v
      @state = :pending
    end
  end

  class OblScheduled < OblIVar
  end

  class OblDelay
    include Obligation

    # @rbs &blk: () -> Integer
    def initialize(&blk)
      @blk = blk
      @state = :pending
      @value = nil
    end

    def wait
      @value = @blk.call
      super
    end
  end

  class ObligationSelfTest < Minitest::Test
    def test_self_returning_module_method
      d = OblDelay.new { 6 }
      assert_equal [1, 2, 6], [OblIVar.new(1).value, OblScheduled.new(2).value, d.value]
      assert d.wait.equal?(d)
      assert_predicate OblScheduled.new(3).wait, :complete?
    end
  end

  # Kernel#freeze on a user object: the flag is the object's, and an override's super reaches it (decision 221).
  class FreezeOpts
    def initialize = @xs = [1]

    def freeze
      @xs.freeze
      super
    end

    attr_reader :xs
  end

  class FreezeSub < FreezeOpts; end

  class ObjectFreezeTest < Minitest::Test
    def test_freeze_user_object
      o = FreezeSub.new
      assert_equal [false, false], [o.frozen?, o.xs.frozen?]
      assert o.freeze.equal?(o)
      assert_equal [true, true, false], [o.frozen?, o.xs.frozen?, FreezeOpts.new.frozen?]
    end
  end

  # super into a block-taking method: its literal, the method's own block (implicitly or as &block), or nil (decision 223).
  class SuperBlkRes
    attr_reader :name, :extra

    #: (?String?) ?{ () -> String } -> void
    def initialize(name = nil, &block)
      @name = name
      @extra = block ? block.call : "none"
    end
  end

  class SuperBlkLocal < SuperBlkRes
    #: (String) -> void
    def initialize(path)
      super(File.basename(path))
    end
  end

  class SuperBlkLit < SuperBlkRes
    #: (String) -> void
    def initialize(path)
      super(path) { "lit" }
    end
  end

  class SuperBlkPass < SuperBlkRes
    #: (String) ?{ () -> String } -> void
    def initialize(path, &block)
      super(path, &block)
    end
  end

  class SuperBlkImplicit < SuperBlkRes
    #: (String) ?{ () -> String } -> void
    def initialize(path, &block)
      super(path)
    end
  end

  class ObjectSuperBlockTest < Minitest::Test
    def test_super_block
      got = [SuperBlkLocal.new("/a/b.rb").name, SuperBlkLocal.new("x").extra, SuperBlkLit.new("y").extra,
             SuperBlkPass.new("z") { "passed" }.extra, SuperBlkPass.new("w").extra, SuperBlkImplicit.new("v") { "imp" }.extra]
      assert_equal ["b.rb", "none", "lit", "passed", "none", "imp"], got
    end
  end

  class SuperSymOrder
    attr_reader :sorted #: Array[Integer]

    #: (Array[Integer] xs) { (Integer, Integer) -> bool } -> void
    def initialize(xs, &block)
      @sorted = xs.sort { |a, b| yield(a, b) ? -1 : 1 }
    end
  end

  class SuperSymMax < SuperSymOrder
    #: (Array[Integer] xs) -> void
    def initialize(xs)
      super(xs, &:>=)
    end
  end

  # `super(&:sym)` passes the symbol's block on; `:sym.to_proc` where a Proc is expected is that lambda.
  class ObjectSuperSymbolBlockTest < Minitest::Test
    def test_super_symbol_block
      assert_equal [3, 2, 1], SuperSymMax.new([2, 3, 1]).sorted
      up = :upcase.to_proc #: ^(String) -> String
      assert_equal "AB", up.call("ab")
    end
  end

  module IsAShim; end

  class IsABase; end

  class IsAWithShim < IsABase
    include IsAShim
  end

  # is_a?(Module) on an untyped value, or a class whose subclass includes it: a class-ID switch (decision 228).
  class ObjectModuleIsATest < Minitest::Test
    def test_module_is_a_at_run_time
      xs = [IsABase.new, IsAWithShim.new, 1, "s", nil] #: Array[untyped]
      assert_equal [false, true, false, false, false], xs.map { |x| x.is_a?(IsAShim) }
      assert_equal [false, false, true, true, false], xs.map { |x| x.is_a?(Comparable) }
      b = IsAWithShim.new #: IsABase
      assert_equal [true, false], [b.is_a?(IsAShim), IsABase.new.is_a?(IsAShim)]
    end
  end

  class HookOpts
    attr_reader :list

    def initialize = @list = [1]

    def initialize_dup(other)
      super
      @list = other.list.dup
    end
  end

  class HookBase
    def self.inherited(sub)
      super
      (@subs ||= []) << sub.name
    end

    def self.subs = @subs
  end

  class HookKid < HookBase; end

  # initialize_dup and inherited overrides calling super (decision 232).
  class ObjectHookSuperTest < Minitest::Test
    def test_hooks
      o = HookOpts.new
      d = o.dup
      d.list << 2
      assert_equal [[1], [1, 2], ["ObjectTests::HookKid"]], [o.list, d.list, HookBase.subs]
    end
  end
end

module ObjectTests
  class RegNumer
    include Enumerable

    def initialize(*list)
      @list = list
    end

    def each
      @list.each { |i| yield i }
    end
  end

  # ruby/spec's EachCounter: an each taking *args answers the parent's each (decisions 206, 233).
  class RegRestCounter < RegNumer
    def each(*_args)
      @list.each { |i| yield i }
    end
  end

  class RegSubTime < Time
  end

  class RegSubThread < Thread
  end

  # ruby/spec's ThreadSpecs::SubThread: initialize's zsuper passes its block to Thread's.
  class RegInitThread < Thread
    def initialize(*args)
      super { args.first << 1 }
    end
  end

  class GoTypeSubclassTest < Minitest::Test
    def test_rest_each_answers_parent_each
      c = RegRestCounter.new(3, 1, 2)
      assert_equal [[3, 1, 2], [1, 2, 3], 6, 1], [c.to_a, c.sort, c.sum, c.min]
    end

    # A Thread subclass starts on its own embedded Thread, so join and value see the goroutine's writes (decision 239).
    def test_thread_subclass_runs_its_block
      t = RegSubThread.new { 1 + 1 }
      assert_equal [true, 2, false, true], [t.join.equal?(t), t.value, t.alive?, t.is_a?(Thread)]
      a = [] #: Array[Integer]
      u = RegInitThread.new(a)
      u.join
      assert_equal [[1], true], [a, u.value.equal?(a)]
      e = assert_raises(ThreadError) { RegSubThread.new }
      assert_equal "must be called with a block", e.message
    end

    # An inherited method of a non-generic @go_type base (Time#-) is its Go method on the embedded value (decision 202).
    def test_inherited_method_on_time_subclass
      y = RegSubTime.at(100) #: untyped
      assert_in_delta 98.5, (y - 1.5).to_f
    end
  end

  # Decision 241: @@x from a prepended module's method is the module's; super in respond_to? is Object's;
  # `singleton_class < M` is whether M extends the class.
  module ObjectCvarExt
    #: () -> Hash[String, Integer]
    def tree = @@tree

    #: () -> void
    def bump! = @@tree["n"] = (@@tree["n"] || 0) + 1

    @@tree = {} #: Hash[String, Integer]
  end

  class ObjectCvarColl
    prepend ObjectCvarExt
  end

  class ObjectRespondVer
    #: (bool) -> void
    def initialize(null) = @null = null

    #: () -> String
    def to_str = "v"

    def size = 1

    #: (Symbol | String, ?bool) -> bool
    def respond_to?(method, include_all = false)
      return !@null if ["to_str", :to_str].include?(method)

      super
    end
  end

  module ObjectSafe; end

  class ObjectSafeBase
    def self.safe? = singleton_class < ObjectSafe
  end

  class ObjectSafeA < ObjectSafeBase
    extend ObjectSafe
  end

  class ObjectSafeB < ObjectSafeA; end
  class ObjectSafeC < ObjectSafeBase; end

  class ObjectUnionMemo
    # @rbs @v: (String | Integer)?
    #: (bool) -> (String | Integer)
    def v(alt)
      return 1 if alt

      @v ||= "memo"
    end
  end

  # An override adding a keyword answers its parent's slot by keyword name; `with:` comes from the caller's **options
  # (decision 241).
  class ObjectKwPkg
    #: (String, ?preinstall: bool, ?verbose: bool, **untyped) -> bool
    def self.install!(name, preinstall: true, verbose: false, **options) = false
  end

  class ObjectKwExt < ObjectKwPkg
    #: (String, ?with: Array[String]?, ?preinstall: bool, ?verbose: bool, **untyped) -> bool
    def self.install!(name, with: nil, preinstall: true, verbose: false, **_options)
      !with.nil? && preinstall
    end
  end

  # A mixed-in `inherited` calling super reaches Class#inherited, which does nothing (decision 241).
  OBJECT_INHERITED_SEEN = [] #: Array[String]

  module ObjectInhDsl
    def inherited(child)
      super
      OBJECT_INHERITED_SEEN << "dsl #{child.name}"
    end
  end

  class ObjectInhReq
    class << self
      include ObjectInhDsl

      def inherited(child)
        super
        OBJECT_INHERITED_SEEN << "req #{child.name}"
      end
    end
  end

  class ObjectInhArch < ObjectInhReq; end

  module ObjectEnvDsl
    # @rbs @env: String?

    def inherited(child)
      super
      child.instance_variable_set(:@env, "e-#{child.name}")
    end

    #: () -> String?
    def env = @env
  end

  class ObjectEnvReq
    @env = nil #: String?
    extend ObjectEnvDsl
  end

  class ObjectEnvArch < ObjectEnvReq; end

  # `def self.included(base) = base.extend(ClassMethods)`: includers and their subclasses get ClassMethods (decision 241).
  module ObjectVolatile
    def self.included(base)
      base.extend(ClassMethods)
    end

    module ClassMethods
      #: () -> String
      def kind = "volatile"
    end
  end

  class ObjectVolObj
    include ObjectVolatile
  end

  class ObjectVolSub < ObjectVolObj; end

  # A class-body `safe_initialization!` whose body is `extend M unless ...`: the class extends M at load (decision 241).
  module ObjectSafeInit
    #: () -> String
    def safe = "safe"
  end

  class ObjectSObj
    def self.object_safe_init!
      extend ObjectSafeInit unless object_safe_init?
    end

    def self.object_safe_init? = singleton_class < ObjectSafeInit
  end

  class ObjectSStack < ObjectSObj
    object_safe_init!
  end

  module ObjectLogg
    DEBUG, INFO, WARN = 0, 1, 2

    def self.lvl(s) = const_get(s)
  end

  class ObjectLexicalTest < Minitest::Test
    #: (String?) -> bool
    def object_titled(t) = t.object_test_present?

    def test_reopened_markers
      v = ENV["RUGAROO_NO_SUCH_VAR"] || false
      assert_equal [false, true, false, false, true, false], [false.object_test_present?, true.object_test_present?, v.object_test_present?,
                                                              object_titled(nil), object_titled("x"), nil.object_test_present?]
      u = { a: nil, b: 1 } #: Hash[Symbol, untyped]
      assert_equal [false, true], u.values.map(&:object_test_present?)
    end

    def test_prepended_module_class_var
      c = ObjectCvColl.new
      first = c.building?
      c.building!
      assert_equal [false, true, true], [first, c.building?, ObjectCvColl.new.building?]
    end

    def test_constant_multi_write
      assert_equal [2, 0, 1], [ObjectLogg::WARN, ObjectLogg::DEBUG, ObjectLogg.lvl(:INFO)]
    end

    def test_self_extend_call
      assert_equal ["safe", true, nil], [ObjectSStack.safe, ObjectSStack.object_safe_init?, ObjectSObj.object_safe_init?]
    end

    def test_included_extend
      assert_equal %w[volatile volatile], [ObjectVolObj.kind, ObjectVolSub.kind]
    end

    def test_class_ivar_set_by_name
      assert_equal ["e-ObjectTests::ObjectEnvArch", nil], [ObjectEnvArch.env, ObjectEnvReq.env]
    end

    def test_mixed_in_inherited_super
      assert_equal ["dsl ObjectTests::ObjectInhArch", "req ObjectTests::ObjectInhArch"], OBJECT_INHERITED_SEEN
    end

    #: (singleton(ObjectKwPkg)) -> bool
    def object_kw_install(k) = k.install!("x", with: ["a"])

    def test_keyword_override_adapter
      assert_equal [true, false, false], [object_kw_install(ObjectKwExt), object_kw_install(ObjectKwPkg), ObjectKwExt.install!("y", preinstall: false, with: [])]
    end

    def test_or_assign_union_ivar
      m = ObjectUnionMemo.new
      assert_equal [1, "memo", "memo"], [m.v(true), m.v(false), m.v(false)]
    end

    def test_prepended_cvar
      c = ObjectCvarColl.new
      c.bump!
      c.bump!
      assert_equal({ "n" => 2 }, c.tree)
    end

    def test_respond_to_super
      a = ObjectRespondVer.new(false)
      b = ObjectRespondVer.new(true)
      assert_equal [true, false, true, false], [a.respond_to?(:to_str), b.respond_to?(:to_str), b.respond_to?(:size), b.respond_to?("nope")]
    end

    def test_singleton_class_lt
      assert_equal [true, true, nil, true], [ObjectSafeA.safe?, ObjectSafeB.safe?, ObjectSafeC.safe?, ObjectSafeA.singleton_class.include?(ObjectSafe)]
    end
  end
end

# Homebrew's OS::Linux::DependencyCollector: @@x set in a module body, read by its methods once prepended (decision 281)
class ObjectCvColl
  def initialize
    init!
  end

  def init!; end
end

module ObjectCvLinux
  @@object_cv_building = false

  def building! = @@object_cv_building = true

  def building? = @@object_cv_building

  def init!
    building?
  end
end

ObjectCvColl.prepend(ObjectCvLinux)

module ObjectTests
  # `#: ::Pathname` names the top-level class even inside a module with its own Pathname (Homebrew's
  # OS::Linux::Pathname; decision 290).
  module AbsOs
    module Pathname
      #: () -> Integer
      def self.helper = 1
    end

    module Lib
      #: (String) -> ::Pathname
      def self.find(path) = Pathname(path) / "libx.so"
    end
  end

  # nil answers freeze, frozen? and dup itself (decision 290): `@name.freeze` on a nil @name is nil, not NoMethodError.
  class NilFreezeTest < Minitest::Test
    #: (String?) -> [String?, bool]
    def nil_freeze(x) = [x.freeze, x.frozen?]

    def test_freeze_on_possibly_nil
      assert_equal [[nil, true], ["a", true]], [nil_freeze(nil), nil_freeze("a")]
    end

    #: (Array[Integer]?) -> Array[Integer]?
    def nil_dup(x)
      y = x.dup
      y << 9 if y
      y
    end

    def test_dup_on_possibly_nil
      a = [1]
      assert_equal [nil, [1, 9], [1]], [nil_dup(nil), nil_dup(a), a]
    end
  end

  class AbsoluteRbsNameTest < Minitest::Test
    def test_absolute_type_name
      assert_equal ["/usr/lib/libx.so", 1], [AbsOs::Lib.find("/usr/lib").to_s, AbsOs::Pathname.helper]
    end
  end

  # `@me = self` in a class with a subclass: the field holds the class's interface, not the generic Self.
  class SelfIvarBase
    def initialize = (@me = self)
    def me = @me
  end

  class SelfIvarSub < SelfIvarBase; end

  module PrependInBody
    def tag = "prepended+#{super}"
  end

  class PrependInBodyTarget
    def tag = "target"
  end

  module PrependInBodyHolder
    ::ObjectTests::PrependInBodyTarget.prepend PrependInBody # a receiver'd prepend inside a module body
  end

  class SelfIvarTest < Minitest::Test
    def test_self_ivar_in_subclassed_class
      b = SelfIvarSub.new
      assert b.me.equal?(b)
    end

    def test_class_body_self_ivar
      sc = Class.new do
        def self.body = @body
        @body = self
      end
      assert sc.body.equal?(sc)
    end

    def test_prepend_inside_module_body
      assert_equal "prepended+target", PrependInBodyTarget.new.tag
    end

    def test_overridden_initialize_arity
      assert_equal(-1, SystemCallError.instance_method(:initialize).arity)
    end
  end

  # #102: NilClass's operators, TrueClass/FalseClass/NilClass.new, Module name checks, frozen structs and objects
  class ObjectRubySpec102Test < Minitest::Test
    Car = Struct.new(:make, :year) #: [String, Integer]
    class Car
      def self.name = "A"
    end

    class Box
      attr_reader :v #: Integer

      def initialize
        @v = 1
      end
    end

    def test_nil_operators
      n = nil #: untyped
      assert_equal [Rational(0, 1), Rational(0, 1), Complex(0, 0), false, true, false, true, nil],
                   [n.to_r, n.rationalize, n.to_c, n & true, n | 1, n | nil, n ^ 2, n =~ /a/]
    end

    def test_immediate_new_undefined
      got = [-> { TrueClass.new }, -> { FalseClass.new }, -> { NilClass.new }].map do |l|
        l.call
        "none"
      rescue NoMethodError => e
        e.message
      end
      assert_equal ["undefined method 'new' for class TrueClass", "undefined method 'new' for class FalseClass", "undefined method 'new' for class NilClass"], got
    end

    def test_module_name_arguments
      got = [-> { Object.method_defined?(1) }, -> { Object.public_method_defined?(nil) }, -> { Object.private_method_defined?(false) }].map do |l|
        l.call
        "none"
      rescue TypeError => e
        e.message
      end
      assert_equal ["1 is not a symbol nor a string", "nil is not a symbol nor a string", "false is not a symbol nor a string"], got
      msg = begin; Module.new.const_set("lower", 1); rescue NameError => e; e.message; end
      assert_equal ["wrong constant name lower", 2], [msg, Module.new.const_set(:Abc, 2)]
    end

    def test_frozen_struct_writers
      c = Car.new("a", 1)
      c.make = "b"
      c.freeze
      m1 = begin; c.make = "x"; rescue FrozenError => e; e.message; end
      m2 = begin; c[:year] = 9; rescue FrozenError => e; e.message; end
      want = "can't modify frozen ObjectTests::ObjectRubySpec102Test::Car: #<struct ObjectTests::ObjectRubySpec102Test::Car make=\"b\", year=1>"
      assert_equal [want, want, "b"], [m1, m2, c.make]
    end

    def test_frozen_instance_variable_set
      b = Box.new.freeze
      got = begin; b.instance_variable_set(:@v, 2); rescue FrozenError => e; e.class; end
      assert_equal [FrozenError, 1], [got, b.v]
    end

    def test_thread_name_and_caller_arguments
      got = [-> { Thread.current.name = "a\0b" }, -> { caller(-1) }, -> { caller(0, -1) }].map do |l|
        l.call
        "none"
      rescue ArgumentError => e
        e.message
      end
      assert_equal ["string contains null byte", "negative level (-1)", "negative size (-1)"], got
    end

    def test_signal_trap_previous_handler
      got = [Signal.trap("PROF", "DEFAULT"), Signal.trap("PROF", "IGNORE"), Signal.trap("PROF", "SYSTEM_DEFAULT"), Signal.trap("USR2", "SIG_DFL"), Signal.trap("USR2", "DEFAULT")]
      msg = begin; Signal.trap("-HUP") {}; rescue ArgumentError => e; e.message; end
      assert_equal ["SYSTEM_DEFAULT", "SYSTEM_DEFAULT", "IGNORE", "DEFAULT", "DEFAULT", "negative signal name: -HUP"], got + [msg]
    end
  end

  # #102: a Struct class includes Enumerable over its values
  class ObjectStructEnumerableTest < Minitest::Test
    EnumPt = Struct.new(:x, :y) #: [Integer, Integer]

    def test_struct_enumerable
      pt = EnumPt.new(3, 4)
      assert_equal [true, true], [EnumPt.include?(Enumerable), pt.is_a?(Enumerable)]
      assert_equal [[6, 8], 3, [3, 4], 3, [[3, 0], [4, 1]], true], [pt.map { |v| v * 2 }, pt.min, pt.sort, pt.to_h[:x], pt.each_with_index.to_a, pt.include?(4)]
    end
  end
end

module ObjectTests
  # Struct/Data members named like a generated helper: the member's accessor wins, as in MRI (#109 D1).
  ShSize = Struct.new(:size, :length, :values) #: [Integer, Integer, Integer]
  ShArr = Struct.new(:to_a, :to_h) #: [Integer, Integer]
  ShStr = Struct.new(:inspect, :to_s) #: [String, String]
  ShIdent = Struct.new(:members, :hash) #: [Integer, Integer]
  ShIter = Struct.new(:each, :each_pair) #: [Integer, Integer]
  ShQuery = Struct.new(:select, :filter, :dig, :values_at) #: [Integer, Integer, Integer, Integer]
  ShDec = Struct.new(:deconstruct, :deconstruct_keys, :a) #: [Integer, Integer, Integer]
  DhHelp = Data.define(:to_h, :hash, :members) #: [Integer, Integer, Integer]
  DhStr = Data.define(:inspect, :to_s, :deconstruct) #: [String, String, Integer]

  # An anonymous Struct/Data/Class/Module built inside a method has no constant: its hidden name stays out of `constants` (#109 D2).
  class AnonBuilder
    LIMIT = 1

    #: () -> Integer
    def build_struct = Struct.new(:a).new(1).a

    #: () -> Integer
    def build_data = Data.define(:a).new(2).a

    #: () -> Integer
    def self.build_class = Class.new { def n = 3 }.new.n

    #: () -> Integer
    def self.build_module = Module.new { def self.n = 4 }.n

    module Inner
      #: () -> Integer
      def self.build = Struct.new(:b).new(6).b
    end

    class FromStruct < Struct.new(:q) #: [Integer]
    end
  end

  # A `T | untyped` parameter checks a typed argument against T and discards the result: the argument runs once (#109 D3).
  GRADUAL_LOG = [] #: Array[String]

  #: () -> [Integer, Integer]
  def self.gradual_pair
    GRADUAL_LOG << "pair"
    [1, 2]
  end

  #: ([Integer?, Integer] | untyped) -> Integer
  def self.gradual_take(x)
    x.is_a?(Array) ? x.size : 0
  end

  class GradualArg < Minitest::Test
    def test_tuple_argument_to_gradual_param_runs_once
      assert_equal 2, ObjectTests.gradual_take(ObjectTests.gradual_pair)
      assert_equal ["pair"], GRADUAL_LOG
    end
  end

  class AnonConstants < Minitest::Test
    def test_anonymous_value_classes_leave_no_constants
      assert_equal 1, AnonBuilder.new.build_struct
      assert_equal 2, AnonBuilder.new.build_data
      assert_equal 3, AnonBuilder.build_class
      assert_equal 4, AnonBuilder.build_module
      assert_equal 6, AnonBuilder::Inner.build
      assert_equal %i[FromStruct Inner LIMIT], AnonBuilder.constants.sort
      assert_equal [], AnonBuilder::Inner.constants
      assert_equal false, AnonBuilder.const_defined?(:A)
      assert_equal false, AnonBuilder.constants.any? { |c| c.to_s.start_with?("RbAnon") }
      assert_equal false, Object.constants.any? { |c| c.to_s.start_with?("RbAnon") }
    end
  end

  class ShHelperNames < Minitest::Test
    def test_struct_member_wins_over_size_values
      s = ShSize.new(5, 6, 7)
      assert_equal 5, s.size
      assert_equal 6, s.length
      assert_equal 7, s.values
      assert_equal [5, 6, 7], s.to_a
      assert_equal({ size: 5, length: 6, values: 7 }, s.to_h)
      assert_equal 6, s[1]
      assert_equal 7, s[:values]
      pairs = [] #: Array[untyped]
      s.each_pair { |k, v| pairs << [k, v] }
      assert_equal [[:size, 5], [:length, 6], [:values, 7]], pairs
      vals = [] #: Array[untyped]
      s.each { |v| vals << v }
      assert_equal [5, 6, 7], vals
      s.size = 9
      assert_equal 9, s.size
      assert_equal [9, 6, 7], s.to_a
      assert_equal ShSize.new(9, 6, 7), s
      assert s.eql?(ShSize.new(9, 6, 7))
    end

    def test_struct_member_wins_over_to_a_to_h
      s = ShArr.new(1, 2)
      assert_equal 1, s.to_a
      assert_equal 2, s.to_h
      assert_equal 2, s.size
      assert_equal 2, s.length
      assert_equal [1, 2], s.values
      assert_equal [1, 2], s.deconstruct
      assert_equal 2, s[1]
      pairs = [] #: Array[untyped]
      s.each_pair { |k, v| pairs << [k, v] }
      assert_equal [[:to_a, 1], [:to_h, 2]], pairs
      vals = [] #: Array[untyped]
      s.each { |v| vals << v }
      assert_equal [1, 2], vals
      assert_equal [1, 2], s.values_at(0, 1)
      assert_equal 1, s.dig(:to_a)
      assert_equal({ to_a: 1, to_h: 2 }, s.deconstruct_keys(nil))
      assert_equal ShArr.new(1, 2), s
      assert_equal :v, { s => :v }[ShArr.new(1, 2)]
      case s
      in [1, 2] then pass
      end
      case s
      in { to_h: 2 } then pass
      end
    end

    def test_struct_member_wins_over_inspect_to_s
      s = ShStr.new("i", "t")
      assert_equal "i", s.inspect
      assert_equal "t", s.to_s
      assert_equal "t", "#{s}"
      assert_equal %w[i t], s.to_a
      s.freeze
      begin
        s.inspect = "x"
        flunk
      rescue FrozenError => e
        assert_equal "can't modify frozen ObjectTests::ShStr: i", e.message # MRI's message calls the overriding inspect
      end
    end

    def test_struct_member_wins_over_members_hash
      s = ShIdent.new(3, 4)
      assert_equal 3, s.members
      assert_equal 4, s.hash
      assert_equal %i[members hash], ShIdent.members
      assert_equal [3, 4], s.to_a
      assert_equal ShIdent.new(3, 4), s
      assert_equal({ members: 3, hash: 4 }, s.to_h)
      s[:hash] = 8
      s[0] = 7
      assert_equal [7, 8], s.to_a
    end

    def test_struct_member_wins_over_each
      s = ShIter.new(1, 2)
      assert_equal 1, s.each
      assert_equal 2, s.each_pair
      assert_equal [1, 2], s.to_a
      assert_equal({ each: 1, each_pair: 2 }, s.to_h)
    end

    def test_struct_member_wins_over_select_dig
      s = ShQuery.new(1, 2, 3, 4)
      assert_equal [1, 2, 3, 4], [s.select, s.filter, s.dig, s.values_at]
      assert_equal [1, 2, 3, 4], s.to_a
      assert_equal 3, s[:dig]
    end

    def test_struct_member_wins_over_deconstruct
      s = ShDec.new(1, 2, 3)
      assert_equal 1, s.deconstruct
      assert_equal 2, s.deconstruct_keys
      assert_equal 3, s.a
      assert_equal [1, 2, 3], s.to_a
      assert_equal({ deconstruct: 1, deconstruct_keys: 2, a: 3 }, s.to_h)
    end

    def test_data_member_wins_over_helpers
      d = DhHelp.new(1, 2, 3)
      assert_equal 1, d.to_h
      assert_equal 2, d.hash
      assert_equal 3, d.members
      assert_equal %i[to_h hash members], DhHelp.members
      assert_equal DhHelp.new(1, 2, 3), d
      assert d.eql?(DhHelp.new(1, 2, 3))
      assert_equal 5, d.with(to_h: 5).to_h
      assert_equal 2, d.with(to_h: 5).hash
    end

    def test_data_member_wins_over_inspect_to_s
      d = DhStr.new("i", "t", 9)
      assert_equal "i", d.inspect
      assert_equal "t", d.to_s
      assert_equal 9, d.deconstruct
      assert_equal DhStr.new("i", "t", 9), d
      assert_equal({ inspect: "i", to_s: "t", deconstruct: 9 }, d.to_h)
    end
  end
end

module ObjectTests
  # `self::X = v` in a class or module body names the constant in that body (decision 362)
  module ObjSelfConst
    self::A = 42
    B = self::A + 1
    class Inner
      self::C = "c"
    end

    def self.read = self::A
  end

  class ObjSingletonAttr
    singleton_class.send(:attr_accessor, :level)
    self.level = 3
  end

  module ObjProtectedMod
    def pub = 1

    protected

    def prot = 2
  end

  # `a_1` and `a1` camel-cased to one Go method name (#109); ı and ſ upper-cased onto i and s.
  class ObjDigitNames
    #: () -> Integer
    def a_1 = 1

    #: () -> Integer
    def a1 = 2

    #: () -> Integer
    def i = 3

    #: () -> Integer
    def ı = 4

    #: () -> Integer
    def s = 5

    #: () -> Integer
    def ſ = 6

    #: () -> Integer
    def a_1b = 7

    #: () -> Integer
    def a1b = 8
  end

  class ObjReflectTest < Minitest::Test
    def test_underscore_digit_and_case_fold_names_stay_distinct
      o = ObjDigitNames.new
      assert_equal [1, 2, 3, 4, 5, 6, 7, 8], [o.a_1, o.a1, o.i, o.ı, o.s, o.ſ, o.a_1b, o.a1b]
    end

    def test_singleton_class_send_attr_accessor
      assert_equal 3, ObjSingletonAttr.level
      ObjSingletonAttr.level = 4
      assert_equal 4, ObjSingletonAttr.level
    end

    def test_self_constant_write
      assert_equal [42, 43, "c", 42], [ObjSelfConst::A, ObjSelfConst::B, ObjSelfConst::Inner::C, ObjSelfConst.read]
    end

    def test_nil_receiver_self_is_nil
      w = /\d/.match("a").object_test_wrap
      assert_equal [true, "nil"], [w.nil_v?, w.v_inspect]
      assert_equal "#<MatchData \"1\">", /\d/.match("1").object_test_wrap.v_inspect
    end

    def test_protected_instance_methods
      assert_equal [:prot], ObjProtectedMod.protected_instance_methods
      assert_equal [:pub], ObjProtectedMod.public_instance_methods
    end
  end
end

module ObjectTests
  # decision 364: a class variable nothing assigns raises NameError where it is read
  class ObjNoCvar
    def m = @@obj_nope
  end

  class ObjNoCvarTest < Minitest::Test
    def test_unassigned_class_variable
      msg = begin
        ObjNoCvar.new.m
        ""
      rescue NameError => e
        e.message
      end
      assert_equal "uninitialized class variable @@obj_nope in ObjectTests::ObjNoCvar", msg
    end
  end
end

module ObjectTests
  # decision 398: a module's singleton class is a leaf; its methods take the concrete *M_Meta
  module ObjLeafModCounter
    @count = 0
    @names = [] #: Array[String]

    #: (String) -> Integer
    def self.add(name)
      @names << name
      @count += 1
    end

    #: () -> Array[String]
    def self.names = @names

    #: () -> Integer
    def self.twice = add("x") + @count
  end

  module ObjLeafModHelper
    #: () -> String
    def helped = "helped"
  end

  module ObjLeafModExtended
    extend ObjLeafModHelper

    @v = 3

    #: () -> String
    def self.both = "#{helped} #{@v}"
  end

  module ObjLeafModFn
    module_function

    #: (Integer) -> Integer
    def dbl(x) = x * 2
  end

  class ObjLeafModTest < Minitest::Test
    def test_module_ivars_through_class_methods
      assert_equal 1, ObjLeafModCounter.add("a")
      assert_equal 2, ObjLeafModCounter.add("b")
      assert_equal 6, ObjLeafModCounter.twice
      assert_equal %w[a b x], ObjLeafModCounter.names
    end

    def test_module_as_value
      mods = [ObjLeafModCounter, ObjLeafModExtended] #: Array[Module]
      assert_equal ["ObjectTests::ObjLeafModCounter", "ObjectTests::ObjLeafModExtended"], mods.map(&:name)
      assert_kind_of Module, ObjLeafModCounter
      assert ObjLeafModExtended.is_a?(ObjLeafModHelper)
      assert_equal "helped 3", ObjLeafModExtended.both
      assert_equal 8, ObjLeafModFn.dbl(4)
    end
  end

  # A Struct can hold itself: ==, eql? and hash stop at a pair or object already in progress, as MRI's
  # rb_exec_recursive_paired and _outer do, instead of overflowing the stack.
  class ObjectRecursiveStructTest < Minitest::Test
    RsCar = Struct.new(:make, :model) #: [untyped, untyped]

    def test_equal_when_the_difference_is_out_of_reach
      x = RsCar.new("a", "b")
      x[:make] = x
      y = RsCar.new("a", "b")
      y[:make] = x
      assert x == y
      assert x.eql?(y)
      assert x.hash == y.hash
    end

    def test_a_difference_found_through_the_cycle
      stepping = RsCar.new("Honda", "Accord")
      stone = RsCar.new(stepping, "Accord")
      stepping[:make] = stone
      z = RsCar.new("Honda", "Accord")
      z[:make] = z
      assert z == stepping
      stone[:model] = "1999"
      assert_equal false, z == stepping
      assert_equal false, z.eql?(stepping)
    end

    def test_plain_structs_compare_as_before
      assert RsCar.new(1, 2) == RsCar.new(1, 2)
      assert RsCar.new(1, 2).hash == RsCar.new(1, 2).hash
      assert_equal false, RsCar.new(1, 2).eql?(RsCar.new(1, 3))
    end
  end
end

# net-ftp's and drb's shapes in a program's own code (decision 416).
module ObjectTests
  class FtpPt
    #: (Integer?) -> void
    def initialize(x)
      @x = x
    end

    #: () -> Integer?
    def x = @x

    private

    #: () -> Integer
    def hidden = 1
  end

  class FtpMade
    private_class_method :new

    #: () -> FtpMade
    def self.make = new
  end

  class FtpShapesTest < Minitest::Test
    def test_allocate
      pt = FtpPt.allocate
      assert_nil(pt.x)
      pt.send(:initialize, 5)
      assert_equal(5, pt.x)
    end

    def test_private_methods
      assert(FtpPt.new(1).private_methods.include?(:hidden))
      refute(FtpPt.new(1).private_methods.include?(:x))
    end

    def test_class_private_methods
      assert(FtpMade.private_methods.include?(:new))
      assert(FtpMade.make.is_a?(FtpMade))
    end

    def test_range_has_no_to_ary
      a, b = (1..2)
      assert_equal([1..2, nil], [a, b])
      r = (1..2) #: untyped
      refute(r.respond_to?(:to_ary))
    end

    def test_weak_map
      m = ObjectSpace::WeakMap.new
      m[1] = "one"
      assert_equal(["one", true, false], [m[1], m.key?(1), m.key?(2)])
    end

    def test_rest_proc_untyped_call
      h = {} #: Hash[Symbol, untyped]
      h[:a] = ->(s, *args) { [s, args] }
      f = h[:a] #: untyped
      assert_equal([[1, []], [1, [2, 3]]], [f.call(1), f.call(1, 2, 3)])
    end

    def test_lambda_conditional_local
      f = ->(x) {
        y = x * 2 if x > 1
        y
      }
      assert_equal([nil, 6], [f.call(1), f.call(3)])
    end
  end

  # a module's override of a method of the module it includes, called from that module (decision 444)
  module NestStyled
    #: () -> Symbol
    def nest_style = nest_style_name.to_sym

    #: () -> String
    def nest_style_name = "plain"
  end

  module NestAlign
    include NestStyled

    #: () -> String
    def nest_style_name = "keyword"
  end

  class NestCopA
    include NestAlign
  end

  class NestCopB
    include NestStyled
  end

  class NestSexp
    #: (String) -> void
    def initialize(name) = @name = name

    #: (?Integer) -> String
    def to_sexp(indent = 0) = "#{'  ' * indent}(#{@name})"

    alias to_s to_sexp

    #: (?Integer) -> String
    def inspect(indent = 0) = "s(#{@name})"
  end

  class NestedModuleOverrideTest < Minitest::Test
    def test_nearer_module_wins
      assert_equal([:keyword, :plain], [NestCopA.new.nest_style, NestCopB.new.nest_style])
    end

    def test_to_s_with_an_optional_argument
      n = NestSexp.new("x")
      assert_equal(["(x)", "  (x)", "(x)", "s(x)", "[s(x)]"], [n.to_s, n.to_s(1), "#{n}", n.inspect, [n].inspect])
    end

    def test_module_spaceship
      assert_equal([-1, 1, 0, nil, nil], [IOError <=> StandardError, StandardError <=> IOError, IOError <=> IOError, IOError <=> ArgumentError, IOError <=> 1])
    end
  end
end

# A class statement inside a block is declared at compile time, in the block's constant scope (decision 451)
module ObjectHoistTests
  [1, 2].each do
    class Counter
      attr_reader :n

      def initialize(n) = @n = n
      def double = n * 2
    end

    module Greet
      def greet = "hi #{name}"
    end
  end

  class Named
    def name = "named"
  end

  HOIST_VALUES = [1].map do
    class Named
      include Greet
    end

    class ExtraHoisted
      def extra = 1
    end
  end

  class HoistTest < Minitest::Test
    def test_class_in_block
      assert_equal 6, Counter.new(3).double
      assert_equal ObjectHoistTests::Counter, Counter
    end

    def test_module_in_block_and_value
      assert_equal [:extra], HOIST_VALUES
      assert_equal "hi named", Named.new.greet
      assert_equal 1, ExtraHoisted.new.extra
    end
  end
end
