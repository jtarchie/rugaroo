# rbs_inline: enabled

require "minitest/autorun"
require "strscan"

module RegexpTests
  # /x is stripped before the groups are counted and translated, wherever a pattern is built (decision 24).
  class RegexpExtendedTest < Minitest::Test
    def test_scan_groups_ignore_x_comments
      assert_equal [["a"], ["a"]], "a b a".scan(/(a) # (note)/x)
      assert_equal [["a", "b"]], "ab".scan(/(a) # (note)
                                           (b)/x)
      assert_equal [], "a b a".scan(/(a) b # (note)/x)
      assert_equal ["a", "a"], "a b a".scan(/a # (note)/x) # no groups: strings, not rows
    end

    def test_rooted_regexp_new_lookbehind
      assert ::Regexp.new("(?<=a)b").match?("ab")
      refute ::Regexp.new("(?<=a)b").match?("cb")
    end

    def test_interpolated_x_scan
      note = "(a) # (note)"
      assert_equal ["a"], "a a".match(/#{note}/x).captures
      assert_equal "a", "a".match(/#{note}/x)[1]
    end

    def test_regexp_new_extended_option
      re = Regexp.new("(a) # (note)\n b", Regexp::EXTENDED)
      assert_equal 2, re.options
      assert_equal "(a) # (note)\n b", re.source
      assert_equal ["a"], "ab".match(re).captures
      assert_nil "a b a".match(re)
      assert_equal "a-", "ab".sub(re, "\\1-")
      assert Regexp.new("a b", "x").match?("ab")
      assert_equal "/a b/mx", Regexp.new("a b", "mx").inspect
      assert_equal "/a b/ix", Regexp.new("a b", Regexp::EXTENDED | Regexp::IGNORECASE).inspect
      assert Regexp.new("a b", Regexp::EXTENDED | Regexp::IGNORECASE).match?("AB")
    end

    def test_timeout_error_class
      assert_equal RegexpError, Regexp::TimeoutError.superclass
    end

    # regexp2 patterns (lookaround) take Ruby's empty-match rule too
    def test_alt_find_all
      assert_equal "a-c", "abc".gsub(/(?<=a)b*/, "-")
      assert_equal "é |a\n|b é\n", "é a\nb é\n".gsub(/\b(?=\w)/, "|")
      # Onigmo's ^ does not match after a final newline (regexp2's would)
      assert_equal "> a\n", "a\n".gsub(/^(?=)/, "> ")
      assert_equal [""], "a\n".scan(/^(?!x)/)
      assert_equal "> a\n> b\n", "a\nb\n".gsub(/^(?=.)/, "> ")
      assert_equal false, "a\n".match?(/\n^(?!x)/)
      assert_equal true, "a\n^".match?(/a\n\^(?!x)/)
      assert_equal true, "ab".match?(/[^c]^?(?!x)b/)
    end
  end

  # #102: grep/grep_v with a block set $~ for each test, as MRI's
  class RegexpGrepLastMatchTest < Minitest::Test
    def test_grep_block_last_match
      assert_equal ["b", "b"], ["aba", "aba"].grep(/a(b)a/) { $1 }
      "z" =~ /z/
      got = ["abc", "def"].grep(/b/) { |e| [e, $&] }
      assert_equal [[["abc", "b"]], nil], [got, $~]
      "z" =~ /z/
      assert_equal [["abc"], "z"], [["abc", "def"].grep(/b/), $&]
      inv = ["abc", "def"].grep_v(/e/) { |e| [e, $~.nil?] }
      assert_equal [[["abc", true]], "e"], [inv, $&]
    end

    # `::Regexp.last_match(n)` reads the frame's $~ as `Regexp.last_match(n)` does (Homebrew's expand_elf_dst).
    def test_rooted_last_match
      out = "$ORIGIN/lib:${LIB}".gsub(/\$({?)([A-Z]+)(}?)/) { "<#{::Regexp.last_match(2)}#{::Regexp.last_match(1).empty?}>" }
      assert_equal "<ORIGINtrue>/lib:<LIBfalse>", out
    end

    def test_escape_symbol
      assert_equal ["a\\.b", "x\\+"], [Regexp.escape(:"a.b"), Regexp.quote("x+")]
    end

    # A binary String (not valid UTF-8) matches byte by byte (decision 324): Homebrew's magic-number checks.
    def test_binary_subject
      data = "\xFD7zXZ\x00\x01\x02".b
      assert_equal [true, 2, "z", true, false], [data.match?(/\A\xFD7zXZ\x00/n), data =~ /zX/n, data.match(/7(z)X/n)&.[](1),
                                                 /\A\xFD/n === data, "\x1f\x8b\xff".b.match?(/\A\xFD/n)]
    end
  end
end

module RegexpTests
  class RegexpPosixPropertyTest < Minitest::Test
    def test_posix_named_properties
      assert_equal "ab_1", "ab_1 é"[/\p{Word}+/]
      assert_equal "é", "ab_1 é"[/\p{Word}\z/]
      assert_equal 2, "ab!" =~ /\P{Word}/
      assert_equal 0, "x y" =~ /[\p{Word}]\s[\p{^Word}y]/
      assert_equal 0, "Ab" =~ /\p{Upper}\p{lower}/
      assert_equal %w[a b], "a b".scan(/\p{Alpha}/)
    end

    def test_nil_not_match
      s = ["ab", nil][1]
      assert_equal true, s !~ /b/
      t = ["ab", nil][0]
      assert_equal false, t !~ /b/
    end
  end
end

module RegexpTests
  # A pattern that is one lookahead runs its first match on RE2, its other uses on regexp2 (decision 332).
  class RegexpWholeLookaheadTest < Minitest::Test
    AHEAD = /(?=\*|_|(  |\\)(?=\n)|`)/

    def test_first_match_is_empty_with_groups
      s = "abc def  \nghi *x* `y`"
      assert_equal 7, s =~ AHEAD
      m = AHEAD.match(s)
      assert_equal [7, 7, "  "], [m&.begin(0), m&.end(0), m&.[](1)]
      assert_nil "plain" =~ AHEAD
      assert_equal 0, "aaab".index(/(?=a+(?=b))|(?=b)/)
    end

    def test_every_match_and_scanner
      assert_equal "ab|  \nc|*d|`", "ab  \nc*d`".gsub(AHEAD, "|")
      ss = StringScanner.new("ab*cd`e")
      assert_equal "ab", ss.scan_until(AHEAD)
      ss.getch
      assert_equal "cd", ss.scan_until(AHEAD)
    end
  end
end

module RegexpTests
  # The UTF-8 validity of a large subject is remembered across matches; a broken String of the same length must not inherit it.
  class RegexpLargeSubjectTest < Minitest::Test
    def test_large_subject_validity_is_not_stale
      big = "abc def " * 100
      bad = big[0, big.size - 1] + "\xFF"
      raised = [] #: Array[bool]
      [big, bad, big, bad].each do |s|
        begin
          s.match?(/\Aabc/)
          raised << false
        rescue ArgumentError
          raised << true
        end
      end
      assert_equal [false, true, false, true], raised
      assert_equal [true, 0, "abc"], [bad.b.match?(/\Aabc/n), big =~ /abc/, big.match(/\Aabc/)&.[](0)]
    end
  end
end

module RegexpTests
  # A pattern that is one ASCII class, a class with +, or an ASCII literal matches without RE2's machinery (decision 347):
  # every String and StringScanner entry point answers as MRI does, on ASCII, multibyte and empty subjects.
  class RegexpFastPathTest < Minitest::Test
    def test_simple_patterns_everywhere
      s = "ab12 é日9 x=1/y?z"
      assert_equal "ab23 é日0 x=2/y?z", s.gsub(/\d/) { |d| ((d.to_i + 1) % 10).to_s }
      assert_equal ["12", "9", "1"], s.scan(/\d+/)
      assert_equal ["ab12", "é日9", "x", "1", "y", "z"], s.split(/[\s\/?=]+/)
      assert_equal 2, s =~ /\d/
      assert_equal 10, s =~ /=1/ # a character index, not a byte offset
      assert s.match?(/日9/)
      refute "".match?(/\d/)
      assert_nil "abc" =~ /\d+?x/
      assert_equal "a-b-c", "a b c".gsub(/ /, "-")
      assert_equal ["1", "2"], "1a2".scan(/\d/)
      assert_equal 1, "aab".index(/ab/)
      assert_equal ["ab", "12"], "ab12".match(/\A[a-z]+/).to_a + [$~.post_match]
      assert_equal "X日X", "a日b".gsub(/[ab]/, "X")
    end

    def test_scanner_with_simple_patterns
      ss = StringScanner.new("let x = 10 + foo;")
      toks = [] #: Array[String]
      until ss.eos?
        next if ss.skip(/\s+/)

        t = ss.scan(/\d+/) || ss.scan(/[a-z_]+/) || ss.scan(/[=+;]/) || ss.getch
        toks << t if t
      end
      assert_equal ["let", "x", "=", "10", "+", "foo", ";"], toks
    end
  end
end

module RegexpTests
  class RegexpCoreGapsTest < Minitest::Test
    def test_linear_time
      assert_equal [true, false, true, true, false], [Regexp.linear_time?(/.(?<=(a))/), Regexp.linear_time?(/(a)\1/), Regexp.linear_time?(/[\x80-\xff]/n), Regexp.linear_time?(/(?=a)b/), Regexp.linear_time?(/(?<n>a)\k<n>/)]
      assert_equal [true, false], [Regexp.linear_time?("a+"), Regexp.linear_time?("(a)\\1")]
    end

    def test_fixed_encoding
      # /u is left out: rugaroo's literals drop the u/e/s flags (rubyRegexFlags), so it answers false.
      assert_equal [false, true, false], [/needle/.fixed_encoding?, /文字化け/.fixed_encoding?, /needle/n.fixed_encoding?]
    end

    def test_match_with_pos
      re = /(.).(.)/
      assert_equal [["1", "3"], ["1", "3"], ["が", "う"]], [re.match("01234", 1)&.captures, re.match("01234", -4)&.captures, re.match("ありがとう", 2)&.captures]
      assert_equal [nil, nil, nil], [re.match("01234", 4), re.match("01234", 10), re.match("01234", -10)]
      assert_equal [nil, nil, 1], [/\Ab/.match("ab", 1), /^b/.match("ab", 1), /(?<=a)b/.match("ab", 1)&.begin(0)]
    end

    def test_match_data_deconstruct_keys
      m = /(?<f>foo)(?<b>bar)?/.match("foobar")
      assert m
      if m
        assert_equal({ f: "foo", b: "bar" }, m.deconstruct_keys(nil))
        assert_equal({ f: "foo", b: "bar" }, m.deconstruct_keys([:f, :b]))
        assert_equal [{}, {}, { f: "foo" }, { b: "bar" }], [m.deconstruct_keys([:f, :b, :c]), m.deconstruct_keys([:c, :f]), m.deconstruct_keys([:f, :c]), m.deconstruct_keys([:b])]
      end
      m2 = /(?<f>foo)(?<b>bar)?/.match("foo")
      assert_equal({ f: "foo", b: nil }, m2&.deconstruct_keys(nil))
    end
  end
end

module RegexpExtendedTests
  # decision 364: `$~ = m` sets the frame's last match
  class LastMatchAssignTest < Minitest::Test
    def test_assign_last_match
      "cd" =~ /(c)/
      m = $~
      "x" =~ /y/
      assert_nil $~
      $~ = m
      assert_equal "c", $1
      $~ = nil
      assert_nil $1
    end
  end
end

module RegexpExtendedTests
  # decision 368: re.match(str, pos) sets $~ as the one-argument form does
  class MatchPosLastMatchTest < Minitest::Test
    def test_match_pos_sets_last_match
      m = /(.).(.)/.match("01234", 1)
      assert_equal ["1", "3"], m&.captures
      assert_equal "1", $1
      assert_equal "123", $~&.[](0)
    end

    def test_fixed_encoding_flag
      assert_equal [true, false, 16, "/a/"], [/needle/u.fixed_encoding?, /needle/.fixed_encoding?, /a/u.options, /a/u.inspect]
      assert_equal false, /a/u == /a/
    end

    def test_linebreak_and_match_position
      assert_equal(["x", "y", "z"], "x\r\ny\nz".split(/\R/))
      m = "ab cd".match(/\G\s/, 2)
      assert_equal([" ", 2], [m.to_s, m.begin(0)])
      assert_nil("ab cd".match(/\G\s/, 1))
      assert_equal(" ", /\G\s/.match("ab cd", 2).to_s)
      assert_equal("b", "abc".match(/b/, 1).to_s)
    end
  end
end
