# rbs_inline: enabled

require "minitest/autorun"
require "json"

# max_nesting (decision 25): the json gem's depth limit, 100 by default, on both sides. The gem's messages are checked in testdata/test/stdlib_test.rb.
module JsonTests
  #: (Integer) -> String
  def self.parse_text(n) = ("[" * n) + "1" + ("]" * n)

  #: (Integer) -> untyped
  def self.nested(n)
    a = 1 #: untyped
    n.times { a = [a] }
    a
  end

  #: () { () -> untyped } -> Class?
  def self.raised
    yield
    nil
  rescue Exception => e # rubocop:disable Lint/RescueException
    e.class
  end

  class Rescuer
    #: (*untyped) -> String
    def to_json(*a)
      JsonTests.nested(3).to_json(*a)
    rescue JSON::NestingError
      "\"rescued\""
    end
  end

  class PrettyPt
    #: (*untyped) -> String
    def to_json(*a) = { "x" => 1, "ys" => [2, 3] }.to_json(*a)
  end

  # @rbs generic V
  # @rbs inherits Hash[untyped, V]
  class Tagged < Hash
    #: (*untyped) -> String
    def to_json(*a) = "{\"tagged\":" + super + "}"
  end

  class NestingTest < Minitest::Test
    # Found by TestFuzzDiff: the json gem skips // and /* */ comments as whitespace, and a lone high surrogate escape is a ParserError.
    def test_parse_comments
      assert_equal [1, 2], JSON.parse("[1,/*c*/2]")
      assert_equal [1], JSON.parse("/*c*/[1] // end")
      assert_equal({ "a" => 1 }, JSON.parse("{\"a\"//c\n:1}"))
      assert_equal({ "a" => "x//y" }, JSON.parse('{"a":"x//y"}'))
      assert_equal ["/*"], JSON.parse('["/*"]')
      assert_equal JSON::ParserError, begin; JSON.parse("[1] /* x"); rescue JSON::ParserError => e; e.class; end
    end

    def test_parse_lone_surrogate
      assert_equal ["\u{1F600}"], JSON.parse('["\\ud83d\\ude00"]')
      assert_equal JSON::ParserError, begin; JSON.parse('["\\ud800"]'); rescue JSON::ParserError => e; e.class; end
      assert_equal JSON::ParserError, begin; JSON.parse('["\\ud800x"]'); rescue JSON::ParserError => e; e.class; end
    end

    def test_parse_depth_limit_is_100
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(100)) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.parse(JsonTests.parse_text(101)) }
    end

    def test_nesting_error_is_a_parser_error
      assert_equal true, JSON::NestingError.new.is_a?(JSON::ParserError)
      caught = begin
        JSON.parse(JsonTests.parse_text(101))
        false
      rescue JSON::ParserError
        true
      end
      assert_equal true, caught
    end

    def test_parse_objects_count
      assert_equal JSON::NestingError, JsonTests.raised { JSON.parse(('{"a":' * 101) + "1" + ("}" * 101)) }
      assert_nil JsonTests.raised { JSON.parse(('{"a":' * 100) + "1" + ("}" * 100)) }
    end

    def test_parse_empty_containers_do_not_count
      assert_nil JsonTests.raised { JSON.parse(("[" * 101) + ("]" * 101)) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.parse(("[" * 102) + ("]" * 102)) }
    end

    def test_parse_max_nesting_option
      assert_equal JSON::NestingError, JsonTests.raised { JSON.parse(JsonTests.parse_text(3), max_nesting: 2) }
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(2), max_nesting: 2) }
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(101), max_nesting: 200) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.parse(JsonTests.parse_text(2), max_nesting: -1) }
    end

    def test_parse_max_nesting_disabled
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(101), max_nesting: false) }
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(101), max_nesting: 0) }
      assert_nil JsonTests.raised { JSON.parse(JsonTests.parse_text(101), max_nesting: nil) }
    end

    def test_load_is_limited_like_parse
      assert_equal JSON::NestingError, JsonTests.raised { JSON.load(JsonTests.parse_text(101)) }
      assert_nil JsonTests.raised { JSON.load(JsonTests.parse_text(101), max_nesting: false) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.load(JsonTests.parse_text(101), max_nesting: 5) }
    end

    # the gem hits end of input (ParserError), rugaroo its internal cap (NestingError); Go's own decoder also stops at 10000, so this guards the class, not a crash
    def test_parse_unlimited_still_cannot_overflow_the_stack
      klass = JsonTests.raised { JSON.parse("[" * 8_000_000, max_nesting: false) }
      assert_equal true, [JSON::ParserError, JSON::NestingError].include?(klass)
    end

    def test_generate_depth_limit_is_100
      assert_nil JsonTests.raised { JsonTests.nested(100).to_json }
      assert_equal JSON::NestingError, JsonTests.raised { JsonTests.nested(101).to_json }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate(JsonTests.nested(101)) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.pretty_generate(JsonTests.nested(101)) }
      assert_equal "{\n  \"a\": [\n    1\n  ]\n}", JSON.pretty_generate({ "a" => [1] }, nil) # Homebrew's Tab#to_json(options = nil)
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate({ "a" => JsonTests.nested(100) }) }
    end

    def test_generate_cycles_raise
      a = [] #: Array[untyped]
      a << a
      assert_equal JSON::NestingError, JsonTests.raised { a.to_json }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate(a) }
      h = {} #: Hash[String, untyped]
      h["self"] = h
      assert_equal JSON::NestingError, JsonTests.raised { h.to_json }
    end

    def test_generate_max_nesting_option
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate(JsonTests.nested(3), max_nesting: 2) }
      assert_equal JSON::NestingError, JsonTests.raised { JsonTests.nested(3).to_json(max_nesting: 2) }
      assert_nil JsonTests.raised { JSON.generate(JsonTests.nested(2), max_nesting: 2) }
      assert_nil JsonTests.raised { JSON.generate(JsonTests.nested(101), max_nesting: 101) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate(JsonTests.nested(1), max_nesting: -1) }
    end

    def test_generate_max_nesting_disabled
      assert_nil JsonTests.raised { JSON.generate(JsonTests.nested(101), max_nesting: false) }
      assert_nil JsonTests.raised { JSON.generate(JsonTests.nested(101), max_nesting: 0) }
    end

    def test_generate_counts_empty_containers_and_the_depth_option
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate([[]], max_nesting: 1) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate({ "a" => {} }, max_nesting: 1) }
      assert_nil JsonTests.raised { JSON.generate([[]], max_nesting: 2) }
      assert_equal JSON::NestingError, JsonTests.raised { JSON.generate(JsonTests.nested(2), depth: 99) }
      assert_nil JsonTests.raised { JSON.generate(JsonTests.nested(1), depth: 99) }
    end

    def test_dump_has_no_limit
      assert_nil JsonTests.raised { JSON.dump(JsonTests.nested(101)) }
    end

    # the generator writes core values into one buffer: a to_json that rescues a nested error leaves no partial value behind
    def test_generate_rescued_nested_error_leaves_nothing
      assert_equal "[1,\"rescued\",2]", JSON.generate([1, Rescuer.new, 2], max_nesting: 3)
    end

    def test_pretty_generate_through_user_to_json
      assert_equal "{\n  \"p\": [\n    {\n      \"x\": 1,\n      \"ys\": [\n        2,\n        3\n      ]\n    }\n  ]\n}", JSON.pretty_generate({ "p" => [PrettyPt.new] })
    end

    # the gem writes a plain Hash itself but calls a subclass's to_json
    def test_generate_calls_a_hash_subclass_to_json
      t = Tagged.new #: Tagged[Integer]
      t["a"] = 1
      xs = [] #: Array[untyped]
      xs << t << { "b" => 2 }
      assert_equal "[{\"tagged\":{\"a\":1}},{\"b\":2}]", JSON.generate(xs)
    end

    def test_generate_escapes_nested_strings
      assert_equal "[\"a /\\u0001\\\"\\\\é😀\"]", JSON.generate(["a /\u0001\"\\é😀"])
    end

    # the gem's unlimited cycle ends in SystemStackError, rugaroo's in its internal cap
    def test_generate_unlimited_cycle_cannot_overflow_the_stack
      a = [] #: Array[untyped]
      a << a
      klass = JsonTests.raised { JSON.generate(a, max_nesting: false) }
      assert_equal true, [JSON::NestingError, SystemStackError].include?(klass)
    end
  end
end
