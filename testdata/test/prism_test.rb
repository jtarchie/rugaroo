require "minitest/autorun"
require "prism"

# require "prism": the gem's Ruby over prism.wasm (decision 427)
module PrismTests
  SOURCE = "def foo(a) = a + 2 # hi\n=begin\ndoc\n=end\nfoo(1)\n".freeze

  class ParseTest < Minitest::Test
    def test_tree
      result = Prism.parse(SOURCE)
      assert result.success?
      program = result.value
      assert_equal "Prism::ProgramNode", program.class.name
      body = program.statements.body
      assert_equal 2, body.size
      assert_equal "Prism::DefNode", body[0].class.name
      assert_equal "Prism::CallNode", body[1].class.name
      assert_equal "foo(1)", body[1].slice
    end

    def test_inspect
      assert_equal Prism.parse("1 + 2").value.inspect, Prism.parse("1 + 2").value.inspect
      assert_includes Prism.parse("1 + 2").value.inspect, "@ CallNode (location: (1,0)-(1,5))"
    end

    def test_errors
      result = Prism.parse("def x(\n")
      refute result.success?
      assert_equal ["unexpected end-of-input; expected a `)` to close the parameters"], result.errors.map(&:message).first(1)
      assert_equal 1, result.errors.first.location.start_line
      assert Prism.parse_failure?("x = ")
      assert Prism.parse_success?("x = 1")
    end

    def test_comments
      comments = Prism.parse(SOURCE).comments
      assert_equal ["Prism::InlineComment", "Prism::EmbDocComment"], comments.map { |c| c.class.name }
      assert_equal ["# hi", "=begin\ndoc\n=end\n"], comments.map { |c| c.location.slice }
      assert_equal ["# a"], Prism.parse_comments("# a\n1\n").map { |c| c.location.slice }
    end

    def test_locations
      call = Prism.parse(SOURCE).value.statements.body[1]
      loc = call.location
      assert_equal [5, 0, 5, 6], [loc.start_line, loc.start_column, loc.end_line, loc.end_column]
      assert_equal [SOURCE.index("foo(1)"), 6], [loc.start_offset, loc.length]
      assert_equal "foo(1)", Prism.parse("foo(1)").value.statements.body.first.slice
    end

    def test_lex
      types = Prism.lex("a + 1").value.map { |pair| pair[0].type }
      assert_equal [:IDENTIFIER, :PLUS, :INTEGER, :EOF], types
      assert_equal 2, Prism.parse_lex("x").value.size
    end

    def test_dump_load
      dumped = Prism.dump(SOURCE)
      assert_equal "PRISM", dumped[0, 5]
      assert_equal Prism.parse(SOURCE).value.inspect, Prism.load(SOURCE, dumped).value.inspect
    end

    def test_string_query
      assert Prism::StringQuery.local?("foo")
      refute Prism::StringQuery.local?("Foo")
      assert Prism::StringQuery.constant?("Foo")
      assert Prism::StringQuery.method_name?("foo=")
    end

    def test_version
      assert_equal "1.9.0", Prism::VERSION
    end
  end
end
