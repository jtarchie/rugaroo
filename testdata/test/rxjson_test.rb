# rbs_inline: enabled

require "json"
require "minitest/autorun"

#: (Regexp, String) -> String
def rxjson_show(re, s) = "#{re.inspect} =~ #{s.inspect}: #{(re =~ s).inspect}"

#: (String) -> String
def rxjson_kind(s)
  case s
  when /\A\d+\z/ then "int"
  when /\A\d+\.\d+\z/, /\A\.\d+\z/ then "float"
  when "yes", /\Ano\z/i then "bool"
  when /^#/ then "comment"
  when // then "other"
  else "unreachable"
  end
end

#: (String, ?Regexp) -> bool
def ok?(s, re = /\S/) = re.match?(s)

#: () -> untyped
def untyped_str = "cat"

#: (String) -> Regexp?
def rxjson_pick(s) = s.empty? ? nil : /#{s}/

# Helpers for the checks that were testdata/run/rxjson_dynamic.rb.
#: (String) -> Regexp
def rxjson_word(w) = /\b#{w}\b/i

#: (Regexp, String) -> String
def dyn_show(re, s) = "#{re.inspect} #{re.source.inspect} #{re.to_s.inspect} =~ #{s.inspect}: #{(re =~ s).inspect}"

#: (Integer) -> String
def rxjson_rep(i) = "x" * i

#: (String) -> String
def try(src)
  re = /#{src}/
  "ok #{re.inspect} #{re.match?("a(b")}"
rescue RegexpError => e
  "RegexpError #{e.class} #{e.is_a?(StandardError)}"
end

# Helpers for the checks that were testdata/run/rxjson_matchdata.rb.
#: (MatchData?) -> Array[String]
def md_dump(m)
  return ["no match"] unless m

  out = [m.inspect, m.to_s.inspect, m.pre_match.inspect, m.post_match.inspect, m.captures.size.to_s]
  out << "[0]=#{m[0].inspect} [1]=#{m[1].inspect} [9]=#{m[9].inspect} [-9]=#{m[-9].inspect}"
  out << "[-1]=#{m[-1].inspect}" if m.captures.size > 0
  out
end

#: (String) -> String?
def num(s)
  m = s.match(/(\d+)/)
  return nil unless m

  m[1]
end

#: () -> untyped
def five = 5

#: () -> untyped
def nothing = nil

#: () -> untyped
def sym = :cat

#: (untyped) -> String
def enc(v) = JSON.generate(v)

module RxjsonTests
  # Helpers for the checks that were testdata/run/rxjson_regexp.rb.
  DIGITS = /\d+/ #: Regexp

  class Finder
    attr_reader :re #: Regexp

    #: (Regexp) -> void
    def initialize(re)
      @re = re
    end

    #: (String) -> Integer?
    def at(s) = s =~ re
  end

  # Helpers for the checks that were testdata/run/rxjson_regexp_objects.rb.
  module Validates
    #: () -> bool
    def valid? = pattern.match?(value)

    #: () -> String?
    def first_group
      m = pattern.match(value)
      m ? m[1] : nil
    end
  end

  class Email
    include Validates
    PATTERN = /\A([^@\s]+)@([^@\s]+)\z/ #: Regexp

    attr_reader :value #: String

    #: (String) -> void
    def initialize(value)
      @value = value
    end

    #: () -> Regexp
    def pattern = PATTERN
  end

  class Loose < Email
    #: () -> Regexp
    def pattern = /@/
  end

  # A /o literal in a module method two classes include: MRI keeps one Regexp per literal, whichever class reaches it first.
  module OnceSource
    #: (String) -> String
    def once_source(x) = /#{x}/o.source
  end

  class OnceA
    include OnceSource
  end

  class OnceB
    include OnceSource
  end

  # `when` with a Regexp held in a constant or a local is still Regexp#===.
  DIG = /\d/ #: Regexp

  # Helpers for the checks that were testdata/run/rxjson_regexp_values.rb.
  Rule = Struct.new(:name, :re) #: [String, Regexp]

  class Lexer
    attr_reader :last #: MatchData?

    #: (String) -> void
    def initialize(prefix)
      @prefix = prefix
      @last = nil
    end

    #: () -> Regexp
    def pat = /\A#{@prefix}(\d+)/

    #: (String) -> String?
    def scan(s)
      @last = pat.match(s)
      l = @last
      l ? l[1] : nil
    end
  end

  class Base
    #: () -> Regexp
    def re = /base/

    #: (String) -> bool
    def ok?(s) = re.match?(s)
  end

  class Child < Base
    #: () -> Regexp
    def re = /child/i
  end

  # Helpers for the checks that were testdata/run/rxjson_mid.rb.
  class Ctr
    #: () -> void
    def initialize
      @n = 0
    end

    #: () -> String
    def nxt
      @n += 1
      @n.to_s
    end
  end

  class MidPt
    #: () -> String
    def to_s = "pt"
  end

  class J
    #: (*untyped) -> String
    def to_json(*_a) = "{\"j\":1}"
  end

  # Helpers for the checks that were testdata/run/rxjson_bug_to_json_signature.rb.
  class OptState
    #: (?untyped) -> String
    def to_json(state = nil) = "\"opt\""
  end

  class NoArg
    #: () -> String
    def to_json = "\"custom\""

    #: () -> String
    def to_s = "noarg"
  end

  # Helpers for the checks that were testdata/run/rxjson_json_collections.rb.
  P = Struct.new(:x, :y) #: [Integer, Integer]

  D = Data.define(:a) #: [String]

  class Pt
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: () -> String
    def to_s = "Pt(#{x})"
  end

  # The json gem passes a generator state to nested to_json calls, hence *untyped.
  class Post
    attr_reader :id #: Integer?
    attr_reader :tags #: Array[String]

    #: (Integer?, Array[String]) -> void
    def initialize(id, tags)
      @id = id
      @tags = tags
    end

    #: (*untyped) -> String
    def to_json(*_state) = { "id" => id, "tags" => tags, "pt" => Pt.new(1) }.to_json
  end

  class Special < Post
    #: (*untyped) -> String
    def to_json(*state) = "{\"special\":#{super}}"
  end

  # Helpers for the checks that were testdata/run/rxjson_json_values.rb.
  VP = Struct.new(:x, :y) do
    #: (*untyped) -> String
    def to_json(*a) = { "x" => x, "y" => y }.to_json(*a)
  end #: [Integer, Integer]

  VD = Data.define(:name) do
    #: (*untyped) -> String
    def to_json(*a) = [name].to_json(*a)
  end #: [String]

  class Q
    #: () -> String
    def to_s = "say \"hi\"\n"
  end

  module Named
    #: () -> String
    def to_s = "named"
  end

  class Thing
    include Named
  end

  class Parent
    #: () -> String
    def to_s = "parent"
  end

  class Kid < Parent
  end

  # super from a to_json override reaches Kernel#to_json (the to_s JSON), then a subclass's super reaches that.
  class Wrap
    #: () -> String
    def to_s = "w"

    #: (*untyped) -> String
    def to_json(*a) = "{\"wrapped\":#{super}}"
  end

  class Str < Wrap
    #: (*untyped) -> String
    def to_json(*a) = "[#{super(*a)}]"
  end

  # Helpers for the checks that were testdata/run/rxjson_json_objects.rb.
  class Tag
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: (*untyped) -> String
    def to_json(*a) = { "tag" => name }.to_json(*a)
  end

  class SubTag < Tag
  end

  class Plain
    #: () -> String
    def to_s = "plain!"
  end

  module Hashable
    #: (*untyped) -> String
    def to_json(*a) = to_h.to_json(*a)
  end

  class Rec
    include Hashable

    #: () -> Hash[String, Integer]
    def to_h = { "r" => 1 }
  end

  # Helpers for the checks that were testdata/run/rxjson_json_generator_state.rb.
  class GsTag
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: (*untyped) -> String
    def to_json(*a) = { "tag" => name, "kids" => [1, 2] }.to_json(*a)
  end

  class Legacy
    #: (?untyped) -> String
    def to_json(state = nil) = [state.nil?, 1].to_json(state)
  end

  class RxjsonRegexpTest < Minitest::Test
    def test_regexp_literals_and_matching
      got = [/(\d+)-(\d+)/, /abc/i, /a.c/m, /a.c/mi, /x/, //, /tab\there/].map do |re|
        "#{re.inspect} #{re.source.inspect} #{re.to_s.inspect}"
      end
      assert_equal [
        "/(\\d+)-(\\d+)/ \"(\\\\d+)-(\\\\d+)\" \"(?-mix:(\\\\d+)-(\\\\d+))\"",
        "/abc/i \"abc\" \"(?i-mx:abc)\"",
        "/a.c/m \"a.c\" \"(?m-ix:a.c)\"",
        "/a.c/mi \"a.c\" \"(?mi-x:a.c)\"",
        "/x/ \"x\" \"(?-mix:x)\"",
        "// \"\" \"(?-mix:)\"",
        "/tab\\there/ \"tab\\\\there\" \"(?-mix:tab\\\\there)\""
      ], got
      assert_equal "/a/mi", (/a/im).inspect
      assert_equal "(?m-ix:y)|(?-mix:z)", "#{/y/m}|#{/z/}"
      assert_equal "/\\d+/", (%r{\d+}).inspect
      assert_equal "\\d+", %r{\d+}.source
      assert_equal true, (/ab/ == /ab/)
      assert_equal false, (/ab/ == /ab/i)
      assert_equal true, (/ab/i == /ab/i)
      assert_equal false, (/ab/m == /ab/i)
      assert_equal false, (/ab/ == /abc/)
      assert_equal false, (/ab/ != /ab/)
      assert_equal false, (/ab/ == "ab")
      assert_equal true, ([/a/, /b/i].include?(/b/i))
      assert_equal false, ([/a/, /b/i].include?(/b/))
      assert_equal true, /(\d+)-(\d+)/.match?("10-20")
      assert_equal false, /(\d+)-(\d+)/.match?("10-")
      assert_equal false, /x/.match?("")
      assert_equal true, //.match?("")
      assert_equal "/\\d+/ =~ \"ab 10-20\": 3", (rxjson_show(/\d+/, "ab 10-20"))
      assert_equal "/zzz/ =~ \"abc\": nil", (rxjson_show(/zzz/, "abc"))
      assert_equal "// =~ \"\": 0", (rxjson_show(//, ""))
      assert_equal "/$/ =~ \"abc\": 3", (rxjson_show(/$/, "abc"))
      assert_equal "/l/ =~ \"héllo\": 2", (rxjson_show(/l/, "héllo"))
      assert_equal "/w/ =~ \"héllo wörld\": 6", (rxjson_show(/w/, "héllo wörld"))
      assert_equal "/語/ =~ \"日本語\": 2", (rxjson_show(/語/, "日本語"))
      assert_equal "/!/ =~ \"😀😀!\": 2", (rxjson_show(/!/, "😀😀!"))
      assert_equal true, DIGITS.match?("x9")
      assert_equal 2, (DIGITS =~ "ab12")
      f = Finder.new(/o+/)
      assert_equal 1, f.at("foo")
      assert_nil f.at("bar")
      assert_equal "o+", f.re.source
      assert_equal "/\\h+/ =~ \"zz 0fA9\": 3", (rxjson_show(/\h+/, "zz 0fA9"))
      assert_equal "/[\\h]+/ =~ \"zz 0fA9\": 3", (rxjson_show(/[\h]+/, "zz 0fA9"))
      assert_equal "/[x\\h]/ =~ \"--x\": 2", (rxjson_show(/[x\h]/, "--x"))
      assert_equal "/\\H/ =~ \"0fz9\": 2", (rxjson_show(/\H/, "0fz9"))
    end

    # Ruby's ^ and $ are always line anchors, so every pattern gets Go's (?m).
    def test_ruby_s_and_are_always
      assert_equal "/^b$/ =~ \"a\\nb\\nc\": 2", (rxjson_show(/^b$/, "a\nb\nc"))
      assert_equal "/\\Ab/ =~ \"a\\nb\": nil", (rxjson_show(/\Ab/, "a\nb"))
      assert_equal "/c$/ =~ \"abc\\n\": 2", (rxjson_show(/c$/, "abc\n"))
      assert_equal "/c\\z/ =~ \"abc\\n\": nil", (rxjson_show(/c\z/, "abc\n"))
      assert_equal "/\\Aabc\\z/ =~ \"abc\": 0", (rxjson_show(/\Aabc\z/, "abc"))
      assert_equal "/^$/ =~ \"a\\n\\nb\": 2", (rxjson_show(/^$/, "a\n\nb"))
    end

    # Ruby's /m is dot-all: Go's (?s), not (?m).
    def test_ruby_s_m_is_dot
      assert_equal "/a.b/ =~ \"a\\nb\": nil", (rxjson_show(/a.b/, "a\nb"))
      assert_equal "/a.b/m =~ \"a\\nb\": 0", (rxjson_show(/a.b/m, "a\nb"))
      assert_equal "/HELLO/i =~ \"say hello\": 4", (rxjson_show(/HELLO/i, "say hello"))
      assert_equal "/a.B/mi =~ \"A\\nb\": 0", (rxjson_show(/a.B/mi, "A\nb"))
      assert_equal "/^.$/ =~ \"é\": 0", (rxjson_show(/^.$/, "é"))
      assert_equal "/(?i)x/ =~ \"aX\": 1", (rxjson_show(/(?i)x/, "aX"))
      assert_equal "/(?i:y)z/ =~ \"Yz\": 0", (rxjson_show(/(?i:y)z/, "Yz"))
      assert_equal "/(?i:y)z/ =~ \"YZ\": nil", (rxjson_show(/(?i:y)z/, "YZ"))
      assert_equal "/a{2,3}/ =~ \"caaaa\": 1", (rxjson_show(/a{2,3}/, "caaaa"))
      assert_equal "/a*?b/ =~ \"aab\": 0", (rxjson_show(/a*?b/, "aab"))
      assert_equal "/(?:ab)+/ =~ \"xabab\": 1", (rxjson_show(/(?:ab)+/, "xabab"))
      assert_equal "/[^a-z]/ =~ \"abc1\": 3", (rxjson_show(/[^a-z]/, "abc1"))
      assert_equal "/\\p{L}+/ =~ \"12日本\": 2", (rxjson_show(/\p{L}+/, "12日本"))
      assert_equal "/\\P{L}/ =~ \"é1\": 1", (rxjson_show(/\P{L}/, "é1"))
      assert_equal "/\\x41/ =~ \"zA\": 1", (rxjson_show(/\x41/, "zA"))
      assert_equal "/\\101/ =~ \"zA\": 1", (rxjson_show(/\101/, "zA"))
      assert_equal "/\\./ =~ \"a.c\": 1", (rxjson_show(/\./, "a.c"))
      assert_equal "/[[:digit:]]+/ =~ \"ab42\": 2", (rxjson_show(/[[:digit:]]+/, "ab42"))
      assert_equal "/[[:space:]]/ =~ \"a b\": 1", (rxjson_show(/[[:space:]]/, "a b"))
      assert_equal "/\\bb/ =~ \"a b\": 2", (rxjson_show(/\bb/, "a b"))
      assert_equal "/\\Bb/ =~ \"ab\": 1", (rxjson_show(/\Bb/, "ab"))
      assert_equal "/a|ab/ =~ \"ab\": 0", (rxjson_show(/a|ab/, "ab"))
      assert_equal "/\\w+/ =~ \"  a1_ \": 2", (rxjson_show(/\w+/, "  a1_ "))
      assert_equal "/\\W/ =~ \"ab!\": 2", (rxjson_show(/\W/, "ab!"))
      assert_equal "/\\S\\s+/ =~ \"a \\t\": 0", (rxjson_show(/\S\s+/, "a \t"))
      assert_equal true, (/a/ === "cat")
      assert_equal false, (/a/ === "dog")
      assert_equal false, (/a/ === 1)
      assert_equal false, (/a/ === nil)
      assert_equal false, (/1/ === 1)
      assert_equal ["\"12\" int", "\"1.5\" float", "\".5\" float", "\"yes\" bool", "\"NO\" bool", "\"x\\n#c\" comment", "\"\" other", "\"ab\" other"],
                   ["12", "1.5", ".5", "yes", "NO", "x\n#c", "", "ab"].map { |s| "#{s.inspect} #{rxjson_kind(s)}" }
      v = 5 #: untyped
      r1 = case v
           when /5/ then "untyped int matched"
           else "untyped int not matched"
           end
      assert_equal "untyped int not matched", r1
      w = "abc" #: untyped
      r2 = case w
           when /b/ then "untyped string matched"
           else "untyped string not matched"
           end
      assert_equal "untyped string matched", r2
      n = nil #: String?
      r3 = case n
           when /x/ then "nil matched"
           else "nil not matched"
           end
      assert_equal "nil not matched", r3
      res = [/a/, /b/i, /c/m] #: Array[Regexp]
      assert_equal "[/a/, /b/i, /c/m]", (res).inspect
      assert_equal "[/a/, /b/i, /c/m]", res.to_s
      assert_equal ["a", "b", "c"], (res.map { |r| r.source })
      assert_equal [false, true, false], (res.map { |r| r.match?("ABC") })
      assert_equal 2, (res.select { |r| r.match?("bab") }.size)
      assert_equal "if-match", ("cat" =~ /a/ ? "if-match" : "")
      assert_equal "unless-match", ("cat" =~ /z/ ? "" : "unless-match")
    end

    # Untyped receivers reach the same methods through generated dynamic dispatch (decision 32).
    def test_untyped_receivers_reach_the_same
      ux = "abc" #: untyped
      assert_equal 1, (ux =~ /b/)
      assert_equal true, ux.match?(/c/)
      assert_equal "#<MatchData \"bc\" 1:\"b\" 2:\"c\">", (ux.match(/(b)(c)/)).inspect
      assert_equal "b", ux.match(/(b)(c)/)[1]
      ur = /q/i #: untyped
      assert_equal true, ur.match?("Q")
      assert_equal "q", ur.source
      assert_equal 1, (ur =~ "aq")
      assert_equal "/q/i", (ur).inspect
      assert_equal "(?i-mx:q)", ur.to_s
    end

    # /i folds non-ASCII letters one rune at a time, as Onigmo does.
    def test_i_folds_non_ascii_letters
      assert_equal "/é/i =~ \"É\": 0", (rxjson_show(/é/i, "É"))
      assert_equal "/àb/i =~ \"xÀB\": 1", (rxjson_show(/àb/i, "xÀB"))
      assert_equal "/σ/i =~ \"Σ\": 0", (rxjson_show(/σ/i, "Σ"))
      assert_equal "/ÿ/i =~ \"y\": nil", (rxjson_show(/ÿ/i, "y"))
    end

    # Escaped punctuation and control escapes are literal characters in both engines.
    def test_escaped_punctuation_and_control_escapes
      assert_equal true, /a\-b/.match?("a-b")
      assert_equal true, /a\#b/.match?("a#b")
      assert_equal true, /\//.match?("/")
      assert_equal true, /\:/.match?(":")
      assert_equal true, /a\_b/.match?("a_b")
      assert_equal true, (/a\ b/.match?("a b"))
      assert_equal true, /\<b\>/.match?("<b>")
      assert_equal true, /\%\@\~\&\=\,\;\!\`/.match?("%@~&=,;!`")
      assert_equal true, /a\"b\'c/.match?("a\"b'c")
      assert_equal true, /\n/.match?("\n")
      assert_equal true, /\t/.match?("\t")
      assert_equal true, /\0/.match?("\0")
      assert_equal true, /\a/.match?("\a")
      assert_equal true, /\x1b\[\d+m/.match?("\e[31m")
      assert_equal true, /\033/.match?("\e")
      assert_equal true, /[\]]/.match?("]")
      assert_equal true, /[\[]/.match?("[")
      assert_equal true, /[a\-z]/.match?("-")
      assert_equal false, /[a\-z]/.match?("b")
      assert_equal true, /[\\]/.match?("\\")
      assert_equal false, /[.]/.match?("a")
      assert_equal "/a{2}/ =~ \"caaa\": 1", (rxjson_show(/a{2}/, "caaa"))
      assert_equal "/a{2,}/ =~ \"a aaa\": 2", (rxjson_show(/a{2,}/, "a aaa"))
      assert_equal "/(a|b)*c/ =~ \"xababc\": 1", (rxjson_show(/(a|b)*c/, "xababc"))
      assert_equal "/a+?/ =~ \"aaa\": 0", (rxjson_show(/a+?/, "aaa"))
      assert_equal "/a??b/ =~ \"ab\": 0", (rxjson_show(/a??b/, "ab"))
      assert_equal "/a/i", (%r{a}i).inspect
      assert_equal "x/y", %r[x/y].source
      assert_equal "/q/", (%r!q!).inspect
      assert_equal "(?m-ix:\\d+)", %r{\d+}m.to_s
    end

    # \d, \w and \s are ASCII-only in Ruby, as in RE2.
    def test_d_w_and_s_are
      assert_equal false, /\d/.match?("٣")
      assert_equal false, /\w/.match?("é")
      assert_equal false, /\s/.match?("\u00a0")
      assert_equal true, /[[:digit:]]/.match?("7")
      assert_equal true, /[[:punct:]]/.match?("!")
      assert_equal false, /./.match?("\n")
      assert_equal true, /[^a]/.match?("\n")
      assert_equal true, /\A\z/.match?("")
      assert_equal true, /\A$/.match?("\n")
    end
  end

  class RxjsonRegexpObjectsTest < Minitest::Test
    # a Regexp constant through a mixed-in method and a subclass override
    def test_regexp_through_module_and_override
      es = [Email.new("a@b.c"), Email.new("bad"), Loose.new("x@@y")] #: Array[Email]
      assert_equal ["a@b.c true \"a\"", "bad false nil", "x@@y true nil"],
                   es.map { |e| "#{e.value} #{e.valid?} #{e.first_group.inspect}" }
      assert_equal false, ok?(" ")
      assert_equal true, ok?("x")
      assert_equal false, ok?("x", /y/)
      assert_equal "\\A([^@\\s]+)@([^@\\s]+)\\z", Email::PATTERN.source
      assert_equal true, Email::PATTERN.match?("q@r")
    end

    # regexps built in blocks and used as block predicates
    def test_regexps_in_blocks_and_collections
      res = [1, 2].map { |i| /#{i}+/ }
      assert_equal "[/1+/, /2+/]", res.inspect
      assert_equal [false, true], res.map { |r| r.match?("22") }
      words = %w[apple Banana cherry avocado]
      assert_equal ["apple", "avocado"], words.select { |w| w.match?(/\Aa/i) }
      assert_equal ["apple", "cherry", "avocado"], words.reject { |w| w.match?(/an/) }
      assert_equal 2, words.select { |w| w.match?(/e/) }.size
      assert_equal "cherry", words.find { |w| w.match?(/rr/) }
      assert_equal ["Banana", "apple", "cherry", "avocado"], words.sort_by { |w| w.match?(/\A[A-Z]/) ? 0 : 1 }
      assert_equal "{false => [\"apple\", \"cherry\", \"avocado\"], true => [\"Banana\"]}", words.group_by { |w| w.match?(/an/) }.inspect
      assert_equal ["apple", "Banana", "avocado"], words.select { |w| /a/ === w }
      assert_equal false, words.any? { |w| w.match?(/z/) }
      assert_equal true, words.all? { |w| w.match?(/\w/) }
      assert_equal "eayo", words.map { |w| w.match(/(.)\z/).to_s }.join
    end

    # `when` with a Regexp held in a constant or a local is still Regexp#===.
    def test_when_with_constant_or_local_regexp
      re = /b/
      got = ["abc", "a1", "zz"].map do |s|
        case s
        when DIG then "const"
        when re then "local"
        else "none"
        end
      end
      assert_equal ["local", "const", "none"], got
      assert_equal true, re === "b"
      assert_equal true, DIG === "5"
      assert_equal false, re.===("x")
    end

    # Regexp Hash keys
    def test_regexp_hash_keys
      pats = { /\A\d+\z/ => "int", /\A[a-z]+\z/ => "word" } #: Hash[Regexp, String]
      got = ["12", "ab", "?"].map do |s|
        hit = pats.find { |re2, v| re2.match?(s) && v != "" }
        hit ? hit[1] : "none"
      end
      assert_equal ["int", "word", "none"], got
      assert_equal ["\\A\\d+\\z", "\\A[a-z]+\\z"], pats.keys.map(&:source)
      assert_equal "{/\\A\\d+\\z/ => \"int\", /\\A[a-z]+\\z/ => \"word\"}", pats.inspect
    end

    # Untyped values reach the typed Regexp methods by assertion (decision 32).
    def test_untyped_values_reach_typed_regexp
      assert_equal true, /a/.match?(untyped_str)
      assert_equal "1", (/a/ =~ untyped_str).inspect
      assert_equal true, "a cat!".match?(/#{untyped_str}!/)
      assert_equal "#<MatchData \"a\" 1:\"a\">", /(a)/.match(untyped_str).inspect
      v = untyped_str
      assert_equal "#<MatchData \"t\">", /t\z/.match(v).inspect
      h = { "re" => /t$/, "s" => "cat" } #: Hash[String, untyped]
      hre = h["re"]
      assert_equal true, hre.match?("cat")
      assert_equal "/t$/", hre.inspect
      assert_equal "2", (h["s"] =~ /t/).to_s
      assert_equal "2", (h["s"] =~ hre).inspect
    end
  end

  class RxjsonRegexpValuesTest < Minitest::Test
    # Regexps in Structs, ivars, overrides and Hash values
    def test_regexps_in_structs_ivars_and_overrides
      rules = [Rule.new("num", /\A\d+\z/), Rule.new("word", /\A\w+\z/)]
      got = ["12", "ab", "!"].map do |tok|
        r = rules.find { |ru| ru.re.match?(tok) }
        "#{tok}: #{r ? r.name : "none"}"
      end
      assert_equal ["12: num", "ab: word", "!: none"], got
      assert_equal ["\\A\\d+\\z", "\\A\\w+\\z"], rules.map { |ru| ru.re.source }
      assert_equal "#<struct RxjsonTests::Rule name=\"num\", re=/\\A\\d+\\z/>", rules[0].inspect
      lx = Lexer.new("id")
      assert_equal "\"42\"", lx.scan("id42x").inspect
      assert_equal "nil", lx.scan("x").inspect
      assert_equal "nil", lx.last.inspect
      assert_equal "/\\Aid(\\d+)/", lx.pat.inspect
      lx.scan("id7")
      assert_equal "#<MatchData \"id7\" 1:\"7\">", lx.last.inspect
      assert_equal "\"\"", lx.last&.pre_match.inspect
      bs = [Base.new, Child.new] #: Array[Base]
      assert_equal [false, true], bs.map { |b| b.ok?("CHILD") }
      assert_equal ["/base/", "/child/i"], bs.map { |b| b.re.inspect }
      pairs = { "x" => /x+/, "y" => /y/ } #: Hash[String, Regexp]
      assert_equal ["x true 1", "y false nil"], pairs.map { |k, v| "#{k} #{v.match?("xx")} #{("axx" =~ v).inspect}" }
      assert_equal ["y"], pairs.select { |_k, v| v.match?("y") }.keys
    end

    # Assignment in a condition, `||` defaults, optional groups through blocks.
    def test_assignment_in_condition_and_optional_groups
      line = "name=bob"
      got = [] #: Array[String]
      if (m = line.match(/(\w+)=(\w+)/))
        got = [m[1].to_s, m[2].to_s]
      end
      assert_equal ["name", "bob"], got
      m2 = "x".match(/(y)?x/)
      assert_equal "default", (m2 ? m2[1] || "default" : "")
      m3 = "k=".match(/(\w)=(\w)?/)
      assert m3
      if m3
        assert_equal ["k", ""], m3.captures.map { |c| c.to_s }
        assert_equal ["\"k\"", "nil"], m3.captures.map { |c| c.inspect }
      end
      p1 = /(\d+)/
      assert_equal ["1", "22", ""], ["a1", "b22", "c"].map { |s| (mm = p1.match(s)) ? mm[1].to_s : "" }
      assert_equal [1, 3], ["1", "x", "3"].select { |s| s.match?(/\A\d+\z/) }.map(&:to_i)
      cm = "ab".match(/(a)/)
      assert cm
      if cm
        cs = cm.captures
        cs << "extra"
        assert_equal 2, cs.size
        assert_equal 1, cm.captures.size
      end
    end

    # Regexp?: nil, narrowing, &., and a method that may return nil.
    def test_optional_regexp
      rr = nil #: Regexp?
      assert_equal "nil", rr.inspect
      assert_equal "\"\"", rr.to_s.inspect
      assert_equal true, rr.nil?
      rr = /z/
      assert_equal "/z/", rr.inspect
      assert_equal "true", (rr&.match?("z")).inspect
      assert_equal "nil", rxjson_pick("").inspect
      assert_equal "/q/", rxjson_pick("q").inspect
      assert_equal "\"q\"", rxjson_pick("q")&.source.inspect
    end

    # Object-level behaviour: literals are frozen Regexp objects.
    def test_regexp_object_behaviour
      assert_equal true, /a/.frozen?
      assert_equal "Regexp", /a/.class.to_s
      assert_equal "MatchData", "a".match(/a/).class.to_s
      assert_equal true, /a/.is_a?(Regexp)
      assert_equal true, /a/.is_a?(Object)
      assert_equal "\\n", /\n/.source
      assert_equal "/\\n/", /\n/.inspect
      assert_equal "(?-mix:\\t)", /\t/.to_s
      assert_equal "/\"/", /"/.inspect
      assert_equal "\\\\", /\\/.source
      assert_equal "/\\\\/", /\\/.inspect
      assert_equal "/é日/", /é日/.inspect
      assert_equal "é日", /é日/.source
      assert_equal "/a b/", /a b/.inspect
      assert_equal "(?-mix:\\s+)", /\s+/.to_s
      assert_equal 11, /[a-z]\d{2,}/.source.length
      assert_equal 3, /x/.inspect.size
    end

    # Collections of matches, chained through &. and blocks.
    def test_collections_of_matches
      re = /(\w+)@(\w+)\.com/
      emails = ["a@b.com", "bad", "c@d.com"]
      assert_equal ["a", "?", "c"], emails.map { |e| re.match(e) }.map { |mm| mm ? mm[1].to_s : "?" }
      assert_equal ["\"b\"", "\"d\""], emails.select { |e| re.match?(e) }.map { |e| e.match(re)&.captures&.last.inspect }
      assert_equal 2, emails.select { |e| e.match?(re) }.size
      assert_equal "bad", emails.find { |e| !e.match?(re) }
      assert_equal "a@b.com", emails.min_by { |e| (e =~ /@/) || 99 }
      assert_equal ["A", "B"], "a-b".match(/(\w)-(\w)/)&.captures&.map { |c| c.to_s.upcase }
      assert_nil "ab".match(/(\w)-(\w)/)&.captures&.map { |c| c.to_s.upcase }
      assert_equal "K|V", "k=v; x=y".match(/(\w)=(\w)/).to_s.split("=").map(&:upcase).join("|")
      assert_equal "\"A\"", "abc".match(/b/)&.pre_match&.upcase.inspect
      assert_equal "\"2\"", ("abc" =~ /c/)&.to_s.inspect
    end

    # Escaped interpolation is literal text; a newline inside a literal is part of the pattern; {x} is not a quantifier.
    def test_literal_edge_cases
      iq = "q"
      assert_equal true, /\#{iq}/.match?('#{iq}')
      assert_equal "\\\#{iq}", /\#{iq}/.source
      assert_equal "/a#b/", /a#b/.inspect
      assert_equal "\\\#{iq}q", /\#{iq}#{iq}/.source
      nl = /a
b/
      assert_equal true, nl.match?("a\nb")
      assert_equal 3, nl.source.size
      assert_equal 5, nl.inspect.size
      assert_equal true, /a{x}/.match?("a{x}")
      assert_equal "#<MatchData \"aa\">", /a{2,}?/.match("aaa").inspect
    end

    # Untyped receivers and arguments through dynamic dispatch (decision 32).
    def test_untyped_receivers_and_arguments
      ux = "abc" #: untyped
      assert_equal false, ux !~ /b/
      assert_equal true, ux !~ /z/
      ur = /a/ #: untyped
      assert_raises(TypeError) { ur.match?(five) }
      assert_equal true, ur === "cat"
      assert_equal false, ur === 5
      assert_equal true, ur == /a/
      assert_equal true, /a/ == ur
      assert_equal true, ur != /b/
    end
  end

  class RxjsonDynamicTest < Minitest::Test
    # interpolated values are inserted raw; inspect, source, to_s and =~ agree with MRI
    def test_interpolation_inserts_raw_values
      dot = "."
      assert_equal "/a.c/ \"a.c\" \"(?-mix:a.c)\" =~ \"xabc\": 1", (dyn_show(/a#{dot}c/, "xabc"))
      assert_equal "/a.c/ \"a.c\" \"(?-mix:a.c)\" =~ \"ac\": nil", (dyn_show(/a#{dot}c/, "ac"))
      assert_equal "/\\bHello\\b/i \"\\\\bHello\\\\b\" \"(?i-mx:\\\\bHello\\\\b)\" =~ \"say hello there\": 4", (dyn_show(rxjson_word("Hello"), "say hello there"))
      assert_equal "/\\bHello\\b/i \"\\\\bHello\\\\b\" \"(?i-mx:\\\\bHello\\\\b)\" =~ \"sayhello\": nil", (dyn_show(rxjson_word("Hello"), "sayhello"))
      n = 42
      assert_equal "/id=42$/m \"id=42$\" \"(?m-ix:id=42$)\" =~ \"x\\nid=42\": 2", (dyn_show(/id=#{n}$/m, "x\nid=42"))
      assert_equal "/id=42$/m \"id=42$\" \"(?m-ix:id=42$)\" =~ \"id=421\": nil", (dyn_show(/id=#{n}$/m, "id=421"))
      sym = :key
      assert_equal "/key:/ \"key:\" \"(?-mix:key:)\" =~ \"a key: b\": 2", (dyn_show(/#{sym}:/, "a key: b"))
      empty = ""
      assert_equal "// \"\" \"(?-mix:)\" =~ \"abc\": 0", (dyn_show(/#{empty}/, "abc"))
      assert_equal "/xy/ \"xy\" \"(?-mix:xy)\" =~ \"axy\": 1", (dyn_show(/x#{empty}y/, "axy"))
      bs = "\\d+"
      assert_equal "/\\d+/ \"\\\\d+\" \"(?-mix:\\\\d+)\" =~ \"ab12\": 2", (dyn_show(/#{bs}/, "ab12"))
      assert_equal "/^\\d+\\h$/ \"^\\\\d+\\\\h$\" \"(?-mix:^\\\\d+\\\\h$)\" =~ \"12f\": 0", (dyn_show(/^#{bs}\h$/, "12f"))
      alt = "cat|dog"
      assert_equal "/^(cat|dog)s?$/ \"^(cat|dog)s?$\" \"(?-mix:^(cat|dog)s?$)\" =~ \"dogs\": 0", (dyn_show(/^(#{alt})s?$/, "dogs"))
      assert_equal "/^(cat|dog)s?$/ \"^(cat|dog)s?$\" \"(?-mix:^(cat|dog)s?$)\" =~ \"cow\": nil", (dyn_show(/^(#{alt})s?$/, "cow"))
      first = "a"
      last = "z"
      assert_equal "/[a-z]+/i \"[a-z]+\" \"(?i-mx:[a-z]+)\" =~ \"09AbZ\": 2", (dyn_show(/[#{first}-#{last}]+/i, "09AbZ"))
      assert_equal "/a.z/m \"a.z\" \"(?m-ix:a.z)\" =~ \"a\\nz\": 0", (dyn_show(/#{first}.#{last}/m, "a\nz"))
      assert_equal true, (/x#{dot}/ == /x#{dot}/)
      assert_equal true, (/x#{dot}/ == /x./)
      assert_equal false, (/x#{dot}/ == /x./i)
      assert_equal true, (/x#{dot}/i == /x./i)
      pats = ["^a", "b$", "c+"].map { |s| /#{s}/ }
      assert_equal "[/^a/, /b$/, /c+/]", (pats).inspect
      assert_equal [true, false, true], (pats.map { |r| r.match?("abccc") })
    end

    # Any value interpolates through to_s: nil, Float, Array, true, untyped, calls, literals.
    def test_any_value_interpolates_through_to
      none = nil #: String?
      assert_equal "/ab/ \"ab\" \"(?-mix:ab)\" =~ \"xab\": 1", (dyn_show(/a#{none}b/, "xab"))
      assert_equal "/1.5/ \"1.5\" \"(?-mix:1.5)\" =~ \"1x5\": 0", (dyn_show(/#{1.5}/, "1x5"))
      assert_equal "/[1, 2]/ \"[1, 2]\" \"(?-mix:[1, 2])\" =~ \" \": 0", (dyn_show(/#{[1, 2]}/, " "))
      assert_equal "/true/ \"true\" \"(?-mix:true)\" =~ \"untrue\": 2", (dyn_show(/#{true}/, "untrue"))
      ub = "b+" #: untyped
      ui = 7 #: untyped
      assert_equal "/ab+/ \"ab+\" \"(?-mix:ab+)\" =~ \"cabbb\": 1", (dyn_show(/a#{ub}/, "cabbb"))
      assert_equal "/7/ \"7\" \"(?-mix:7)\" =~ \"17\": 1", (dyn_show(/#{ui}/, "17"))
      assert_equal "/^xx$/ \"^xx$\" \"(?-mix:^xx$)\" =~ \"xx\": 0", (dyn_show(/^#{rxjson_rep(2)}$/, "xx"))
      assert_equal "/^xx$/ \"^xx$\" \"(?-mix:^xx$)\" =~ \"xxx\": nil", (dyn_show(/^#{rxjson_rep(2)}$/, "xxx"))
      assert_equal "/lit/ \"lit\" \"(?-mix:lit)\" =~ \"a lit\": 2", (dyn_show(/#{"lit"}/, "a lit"))
      assert_equal "/a2b/ \"a2b\" \"(?-mix:a2b)\" =~ \"a2b\": 0", (dyn_show(/a#{1 + 1}b/, "a2b"))
      wv = "w"
      wre = /#{wv}/i #: Regexp
      rh = { "k" => wre, "j" => /j/ } #: Hash[String, Regexp]
      assert_equal "/w/i", (rh["k"]).inspect
      assert_equal ["w", "j"], rh.values.map(&:source)
      assert_equal "{\"k\" => /w/i, \"j\" => /j/}", (rh).inspect
      assert_equal ["w", "z"], ([wre, /z/].map(&:source))
      ny = "q"
      nm = /(?<n>#{ny})(?<o>y)?/.match("aq")
      assert nm
      if nm
        assert_equal "\"q\"", nm[1].inspect
        assert_equal "nil", nm[2].inspect
        assert_equal "a", nm.pre_match
      end
      assert_equal 1, (/(?<n>#{ny})/ =~ "aq")
      assert_equal "ok /a/ true", try("a")
      assert_equal "RegexpError RegexpError true", try("(")
      assert_equal "RegexpError RegexpError true", try("a[b")
      assert_equal "ok /\\(/ true", try("\\(")
      assert_equal "RegexpError RegexpError true", try("x{2,1}")
      bad = "("
      assert_raises(RegexpError) { /a#{bad}b/.match?("x") }
    end

    # an interpolated pattern that fails to compile raises RegexpError
    def test_unclosed_class_raises
      unclosed = "["
      # only the class: MRI says "premature end of char-class: /[/", rugaroo reports Go's regexp error
      assert_raises(RegexpError) { /#{unclosed}/.inspect }
    end
  end

  class RxjsonMatchdataTest < Minitest::Test
    # inspect, to_s, pre/post_match, captures and indexing across encodings and escapes
    def test_match_data_accessors
      assert_equal ["#<MatchData \"10-20\" 1:\"10\" 2:\"20\">", "\"10-20\"", "\"ab \"", "\" cd\"", "2", "[0]=\"10-20\" [1]=\"10\" [9]=nil [-9]=nil", "[-1]=\"20\""],
                   md_dump(/(\d+)-(\d+)/.match("ab 10-20 cd"))
      assert_equal ["#<MatchData \"ör\" 1:\"ö\" 2:\"r\">", "\"ör\"", "\"héllo w\"", "\"ld\"", "2", "[0]=\"ör\" [1]=\"ö\" [9]=nil [-9]=nil", "[-1]=\"r\""],
                   md_dump("héllo wörld".match(/(ö)(r)/))
      assert_equal ["#<MatchData \"本語\" 1:\"語\">", "\"本語\"", "\"日\"", "\"テキスト\"", "1", "[0]=\"本語\" [1]=\"語\" [9]=nil [-9]=nil", "[-1]=\"語\""],
                   md_dump("日本語テキスト".match(/本(.)/))
      assert_equal ["#<MatchData \"😀 ok\" 1:\"ok\">", "\"😀 ok\"", "\"emoji \"", "\"\"", "1", "[0]=\"😀 ok\" [1]=\"ok\" [9]=nil [-9]=nil", "[-1]=\"ok\""],
                   md_dump("emoji 😀 ok".match(/😀 (\w+)/))
      assert_equal ["#<MatchData \"\">", "\"\"", "\"\"", "\"\"", "0", "[0]=\"\" [1]=nil [9]=nil [-9]=nil"],
                   md_dump("".match(/^$/))
      assert_equal ["#<MatchData \"b\">", "\"b\"", "\"a\"", "\"c\"", "0", "[0]=\"b\" [1]=nil [9]=nil [-9]=nil"],
                   md_dump("abc".match(/b/))
      assert_equal ["no match"],
                   md_dump("abc".match(/z/))
      assert_equal ["#<MatchData \"\\th\" 1:\"h\">", "\"\\th\"", "\"tab\"", "\"ere\"", "1", "[0]=\"\\th\" [1]=\"h\" [9]=nil [-9]=nil", "[-1]=\"h\""],
                   md_dump("tab\there".match(/\t(h)/))
      assert_equal ["#<MatchData \"\\\"x\\\\\" 1:\"x\" 2:\"\\\\\">", "\"\\\"x\\\\\"", "\"q\"", "\"\"", "2", "[0]=\"\\\"x\\\\\" [1]=\"x\" [9]=nil [-9]=nil", "[-1]=\"\\\\\""],
                   md_dump("q\"x\\".match(/"(x)(\\)/))
      assert_equal ["#<MatchData \"line2\" 1:\"line2\">", "\"line2\"", "\"line1\\n\"", "\"\"", "1", "[0]=\"line2\" [1]=\"line2\" [9]=nil [-9]=nil", "[-1]=\"line2\""],
                   md_dump("line1\nline2".match(/^(line2)$/))
    end

    # An unmatched optional group is nil, not "".
    def test_an_unmatched_optional_group_is
      m = /(\d+)-(\d+)?(x)?/.match("ab 10- cd")
      assert m
      if m
        assert_equal ["#<MatchData \"10-\" 1:\"10\" 2:nil 3:nil>", "\"10\"", "nil", "nil", "nil", "nil", "\"10\""],
                     [m.inspect, m[1].inspect, m[2].inspect, m[3].inspect, m[-1].inspect, m[-2].inspect, m[-3].inspect]
        assert_equal 3, m.captures.size
        assert_equal "\"10\"", m.captures[0].inspect
      end
      alt = /(a)|(b)/.match("b")
      assert_equal "#<MatchData \"b\" 1:nil 2:\"b\">", (alt).inspect
      assert alt
      if alt
        assert_equal "nil", alt[1].inspect
        assert_equal "\"b\"", alt[2].inspect
      end
      m2 = "key=value; other=x".match(/(\w+)=(\w+)/)
      assert m2
      if m2
        assert_equal "key=value", m2.to_s
        assert_equal "key=value / key / value", "#{m2} / #{m2[1]} / #{m2[2]}"
        k, v = m2.captures
        assert_equal "\"key\"", k.inspect
        assert_equal "\"value\"", v.inspect
        assert_equal ["KEY", "VALUE"], m2.captures.map { |c| c.to_s.upcase }
      end
      path = "/posts/42.json"
      assert_equal 7, (path =~ /\d+/)
      assert_nil (path =~ /zzz/)
      assert_equal true, (path !~ /\.html$/)
      assert_equal false, (path !~ /json/)
      assert_equal true, path.match?(/json/)
      assert_equal false, "abc".match?(/^b/)
      assert_equal true, "Hello".match?(/hello/i)
      assert_equal true, "".match?(//)
      assert_nil path.match(/xml/)
      pm = path.match(%r{\A/(\w+)/(\d+)(\.\w+)?\z})
      assert pm
      if pm
        assert_equal "#<MatchData \"/posts/42.json\" 1:\"posts\" 2:\"42\" 3:\".json\">", pm.inspect
        assert_equal "posts#42", pm[1].to_s + "#" + pm[2].to_s
      end
    end

    # Edge matches: an empty group is "", not nil; a repeated group keeps its last iteration.
    def test_edge_matches_an_empty_group
      assert_equal "#<MatchData \"b\" 1:\"\">", ("abc".match(/(x?)b/)).inspect
      assert_equal "#<MatchData \"\">", ("abc".match(//)).inspect
      assert_equal "#<MatchData \"\">", ("abc".match(/$/)).inspect
      assert_equal "#<MatchData \"123\" 1:\"3\">", ("123".match(/(\d)+/)).inspect
      assert_equal "#<MatchData \"ab\" 1:\"ab\" 2:\"a\" 3:\"b\">", ("ab".match(/((a)(b))/)).inspect
      assert_equal "#<MatchData \"ab\" 1:\"b\">", ("ab".match(/(?:a)(b)/)).inspect
      em = "abc".match(/$/)
      assert em
      if em
        assert_equal "abc", em.pre_match
        assert_equal "", em.post_match
      end
      em2 = "abc".match(//)
      assert em2
      if em2
        assert_equal "", em2.pre_match
        assert_equal "abc", em2.post_match
      end
    end

    # Safe navigation and boolean contexts over MatchData? and Integer?.
    def test_safe_navigation_and_boolean_contexts
      assert_equal "a+b", "a-b".match(/(\w)-(\w)/)&.captures&.join("+")
      assert_nil "ab".match(/(\w)-(\w)/)&.captures&.join("+")
      assert_equal "b", "a-b".match(/(\w)-(\w)/)&.[](2)
      assert_equal 2, ("x9" =~ /\d/)&.succ
      assert_equal "a", "a-b".match(/-/)&.pre_match
      assert_equal 13, ("ab12cd".match(/\d+/).to_s.to_i + 1)
      assert_equal "", "ab".match(/\d+/).to_s
      assert_equal true, (!("abc" =~ /z/))
      assert_equal true, ("abc" =~ /z/).nil?
      assert_equal 2, (("abc" =~ /c/) || -1)
      assert_equal(-1, (("abc" =~ /z/) || -1))
      assert_equal true, ("abc" =~ /b/) == 1
      assert_equal true, ("abc" =~ /z/) == nil
      assert_equal false, "abc".match(/b/).nil?
      assert_equal true, ("abc".match(/z/) == nil)
      assert_equal "#<MatchData \"c\">", (("abc".match(/z/) || "abc".match(/c/))).inspect
      assert_equal "none", ("x".match(/y/) || "none")
      got = nil #: Integer?
      if (i = "héllo" =~ /l/)
        got = i + 1
      end
      assert_equal 3, got
      wgot = nil #: String?
      while (wm = "abc".match(/c/))
        wgot = wm.to_s
        break
      end
      assert_equal "c", wgot
      assert_equal "12", num("ab12")
      assert_nil num("ab")
      ms = ["a1", "b", "c3"].map { |s| s.match(/(\w)(\d)/) }
      assert_equal ["#<MatchData \"a1\" 1:\"a\" 2:\"1\">", "nil", "#<MatchData \"c3\" 1:\"c\" 2:\"3\">"], ms.map(&:inspect)
      assert_equal 2, (ms.reject { |mm| mm.nil? }.size)
      assert_equal ["a", "-", "c"], (ms.map { |mm| mm ? mm[1] : "-" })
      store = {} #: Hash[String, MatchData]
      xm = "xy".match(/x(y)/)
      store["m"] = xm if xm
      assert_equal "#<MatchData \"xy\" 1:\"y\">", (store["m"]).inspect
      assert_equal "{\"m\" => #<MatchData \"xy\" 1:\"y\">}", (store).inspect
      kv = {} #: Hash[String, String?]
      "a=1\nb=2\nc".split("\n").each do |l|
        lm = l.match(/\A(\w)=(\d)\z/)
        next unless lm

        key = lm[1]
        kv[key] = lm[2] if key
      end
      assert_equal "{\"a\" => \"1\", \"b\" => \"2\"}", (kv).inspect
      mu = "k=v".match(/(\w)=(\w)/) #: untyped
      assert_equal "k", mu[1]
      assert_equal ["k", "v"], mu.captures
      assert_equal "", mu.pre_match
      assert_equal "", mu.post_match
      assert_equal "k=v", mu.to_s
      assert_equal "#<MatchData \"k=v\" 1:\"k\" 2:\"v\">", (mu).inspect
      assert_equal "v", mu[-1]
    end

    # Calling a method on a failed match raises NoMethodError, as Ruby does.
    def test_calling_a_method_on_a
      e = assert_raises(NoMethodError) { "abc".match(/z/)[0].inspect }
      assert_equal "undefined method '[]' for nil", e.message
      e2 = assert_raises(NoMethodError) { ("abc" =~ /z/) + 1 }
      assert_equal "undefined method '+' for nil", e2.message
      e3 = assert_raises(NoMethodError) { "abc".match(/q/).pre_match }
      assert_equal "undefined method 'pre_match' for nil", e3.message
    end
  end

  class RxjsonBugsTest < Minitest::Test
    # captures.compact drops the unmatched optional group; compact on a MatchData? array
    def test_captures_compact_drops_the_unmatched
      m_compact = /(a)(b)?/.match("a")
      assert_equal 1, (m_compact ? m_compact.captures.compact.size : -1)
      found = ["a1", "b", "c3"].map { |s| s.match(/\d/) }.compact
      assert_equal 2, found.size
    end

    # an unmatched group is nil in captures, destructuring, join and inspect
    def test_an_unmatched_group_is_nil
      m_nil = /(a)(b)?/.match("a")
      assert m_nil
      if m_nil
        c = m_nil.captures
        assert_equal 2, c.size
        assert_equal "nil", c[1].inspect
        k, v = m_nil.captures
        assert_equal "\"a\"", k.inspect
        assert_equal "nil", v.inspect
        assert_equal "[\"a\", nil]", c.inspect
        assert_equal "a,", c.join(",")
        assert_equal 2, m_nil.captures.join("-").size
      end
      assert_equal [1, nil], (%w[cat dog].map { |w| w =~ /a/ })
    end

    # ^ does not match after a string's final newline
    def test_does_not_match_after_a
      assert_nil ("a\n" =~ /^$/)
      assert_nil ("a\n" =~ /^\z/)
      assert_equal false, "a\n".match?(/\n^/)
    end

    # && class intersection and a nested class
    def test_class_intersection_and_a_nested
      assert_equal "#<MatchData \"bcd\">", ("aebcd".match(/[a-z&&[^aeiou]]+/)).inspect
      assert_nil "&]".match(/[a-z&&[^aeiou]]/)
      assert_equal true, "b".match?(/[a[bc]]/)
      assert_equal "#<MatchData \"b\">", ("b]".match(/[a[bc]]/)).inspect
    end

    # an interpolated Regexp keeps its own flags
    def test_an_interpolated_regexp_keeps_its
      inner = /ab/i
      outer = /x#{inner}y/
      assert_equal "/x(?i-mx:ab)y/", (outer).inspect
      assert_equal true, outer.match?("xABy")
      assert_equal true, outer.match?("xaby")
      assert_equal false, outer.match?("xy")
      dotall = /a.b/m
      assert_equal true, /#{dotall}/.match?("a\nb")
    end

    # Regexp#=== on a Symbol, directly and in case/when
    def test_regexp_on_a_symbol_directly
      assert_equal true, (/b/ === :abc)
      assert_equal false, (/z/ === :abc)
      r = case :abc
          when /b/ then "symbol matched"
          else "symbol not matched"
          end
      assert_equal "symbol matched", r
    end

    # inline (?m) groups and a scoped (?-m:)
    def test_inline_m_groups_and_a
      assert_equal true, "a\nb".match?(/(?m:a.b)/)
      assert_equal true, "a\nb".match?(/(?m)a.b/)
      assert_equal true, "A\nb".match?(/(?mi)a.b/)
      assert_equal false, "xa\nb".match?(/x(?-m:a.b)/m)
    end

    # a class split across an interpolation still translates \h
    def test_a_class_split_across_an
      x_class = "z"
      assert_equal true, "z".match?(/[#{x_class}\h]/)
      assert_equal true, "f".match?(/[#{x_class}\h]/)
      assert_equal false, "]".match?(/[#{x_class}\h]/)
      assert_equal "#<MatchData \"z\">", ("z]".match(/[#{x_class}\h]/)).inspect
    end

    # interpolated text is Ruby regexp syntax too
    def test_interpolated_text_is_ruby_regexp
      h_interp = "\\h+"
      assert_equal "#<MatchData \"F0\">", (/#{h_interp}/.match("zzF0")).inspect
    end

    # Float#to_json uses the shortest round-trip digits
    def test_float_to_json_uses_the
      assert_equal "9.999999999999999e+22", 1e23.to_json
      assert_equal "1234567890123456.7", 1234567890123456.8.to_json
      assert_equal "[9.999999999999999e+22]", [1e23].to_json
      assert_equal "-9.999999999999999e+22", JSON.generate(-1e23)
      assert_equal "5.3261726645502314e-12", 5.326172664550231e-12.to_json
      assert_equal "8.920432871205619e+16", 8.92043287120562e+16.to_json
    end

    # invalid UTF-8 raises JSON::GeneratorError, bare and nested
    def test_invalid_utf_8_raises_json
      e = assert_raises(JSON::GeneratorError) { "\xff".to_json }
      assert_equal "JSON::GeneratorError", e.class.to_s
      assert_equal "source sequence is illegal/malformed utf-8", e.message
      e2 = assert_raises(JSON::GeneratorError) { ["ok", "a\xffb"].to_json }
      assert_equal "source sequence is illegal/malformed utf-8", e2.message
    end

    # nil inside typed containers serializes as null
    def test_nil_inside_typed_containers_serializes
      a_json = [1, nil] #: Array[Integer?]
      assert_equal "[1,null]", a_json.to_json
      assert_equal "[\"a\",null]", (["a", nil].to_json)
      assert_equal "[1.5,null]", ([1.5, nil].to_json)
      h_json = { "a" => nil, "b" => 1 } #: Hash[String, Integer?]
      assert_equal "{\"a\":null,\"b\":1}", h_json.to_json
      t_json = [1, nil] #: [Integer, String?]
      assert_equal "[1,null]", t_json.to_json
      assert_equal "{\"1\":2,\"\":3}", ({ 1 => 2, nil => 3 }.to_json)
      assert_equal "[null,2]", (JSON.generate([nil, 2]))
    end

    # MatchData#[] with negative indexes, including past the start
    def test_matchdata_with_negative_indexes_including
      m_neg = "abc".match(/b/)
      assert_equal "nil", m_neg[-1].inspect if m_neg
      m2_neg = /(a)/.match("a")
      assert m2_neg
      if m2_neg
        assert_equal "\"a\"", m2_neg[-1].inspect
        assert_equal "nil", m2_neg[-2].inspect
        assert_equal "nil", m2_neg[-3].inspect
      end
    end

    # /i with multi-character case folds
    def test_i_with_multi_character_case
      assert_equal true, "straße".match?(/STRASSE/i)
      assert_equal true, "SS".match?(/ß/i)
      assert_equal true, "ﬀ".match?(/FF/i)
    end

    # named groups: numbered access, captures and inspect with an unmatched group
    def test_named_groups_numbered_access_captures
      m_named = /(?<year>\d+)-(?<mon>\d+)(?<day>-\d+)?/.match("2024-05")
      assert m_named
      if m_named
        assert_equal "\"2024\"", m_named[1].inspect
        assert_equal "\"05\"", m_named[2].inspect
        assert_equal "nil", m_named[3].inspect
        assert_equal 3, m_named.captures.size
        assert_equal "#<MatchData \"2024-05\" year:\"2024\" mon:\"05\" day:nil>", m_named.inspect
      end
    end

    # named groups make plain parens non-capturing
    def test_named_groups_make_plain_parens
      m_plain = /(?<a>x)(y)/.match("xy")
      assert m_plain
      if m_plain
        assert_equal 1, m_plain.captures.size
        assert_equal "\"x\"", m_plain[1].inspect
        assert_equal "nil", m_plain[2].inspect
        assert_equal "x", m_plain.captures.join(",")
      end
    end

    # nil captures answer NilClass#to_i/#to_f; =~ on a nil String?
    def test_nil_captures_answer_nilclass_to
      m_nilclass = /(\d+)(?:\.(\d+))?/.match("v12")
      assert m_nilclass
      if m_nilclass
        assert_equal 12, m_nilclass[1].to_i
        assert_equal 0, m_nilclass[2].to_i
        assert_equal "0.0", m_nilclass[2].to_f.to_s
      end
      line = nil #: String?
      assert_nil (line =~ /x/)
    end

    # /o interpolates once, on first evaluation
    def test_o_interpolates_once_on_first
      got = ["a", "b"].map do |x|
        re = /#{x}/o
        "#{re.source} #{re.match?("b")}"
      end
      assert_equal ["a false", "a false"], got
    end

    # /o is one Regexp per literal, shared by every class that includes its method
    def test_o_once_per_literal_across_includers
      assert_equal ["a", "a"], [OnceA.new.once_source("a"), OnceB.new.once_source("b")]
    end

    # {,n} is an open-min quantifier
    def test_n_is_an_open_min
      assert_equal "#<MatchData \"aa\">", ("aaa".match(/a{,2}/)).inspect
      assert_equal "#<MatchData \"xb\">", ("xbb".match(/xb{,1}/)).inspect
      assert_equal 0, ("{,2}" =~ /a{,2}/)
    end

    # a parenthesized numeric literal as receiver
    def test_a_parenthesized_numeric_literal_as
      assert_equal "-1", (-1).to_json
      assert_equal "0.5", (0.5).to_json
      assert_equal "-0.5", (-0.5).to_json
    end

    # POSIX classes are Unicode-aware
    def test_posix_classes_are_unicode_aware
      assert_equal "#<MatchData \"é\">", ("é".match(/[[:alpha:]]/)).inspect
      assert_equal "#<MatchData \"É\">", ("É".match(/[[:upper:]]+/)).inspect
      assert_equal "#<MatchData \"日本1\">", ("日本1".match(/[[:alnum:]]+/)).inspect
      assert_equal 0, ("é" =~ /[[:word:]]/)
      assert_equal 0, ("é" =~ /[[:lower:]]/)
      assert_equal true, "٣".match?(/[[:digit:]]/)
      assert_equal true, "　".match?(/[[:space:]]/)
      assert_equal false, "x　".match?(/x[[:^space:]]/)
    end

    # \u, \u{...} and \e escapes in a pattern
    def test_u_u_and_e_escapes
      assert_equal true, "ab".match?(/ab/)
      assert_equal true, "é".match?(/é/)
      assert_equal true, "é".match?(/\u{e9}/)
      assert_equal true, ("日本".match?(/\u{65e5 672c}/))
      assert_equal "#<MatchData \"\\e[31m\">", ("\e[31mred\e[0m".match(/\e\[\d+m/)).inspect
    end

    # equal Regexps, literal or interpolated, are one Hash key
    def test_equal_regexps_literal_or_interpolated
      x_key = "a"
      h_key = { /a/ => 1 } #: Hash[Regexp, Integer]
      assert_equal 1, h_key[/a/]
      assert_equal 1, h_key[/#{x_key}/]
      res = [/a/, /#{x_key}/, /b/] #: Array[Regexp]
      assert_equal 2, res.uniq.size
    end

    # source/inspect/to_s escape / as the literal form does
    def test_source_inspect_to_s_escape
      x_slash = "b/c"
      assert_equal "a/b", /a\/b/.source
      assert_equal "/a\\/b/", (%r{a/b}).inspect
      assert_equal "(?-mix:a\\/b)", %r{a/b}.to_s
      assert_equal "a/b", %r{a/b}.source
      assert_equal "/ab\\/c/", (/a#{x_slash}/).inspect
      assert_equal "(?-mix:ab\\/c)", /a#{x_slash}/.to_s
      assert_equal "ab/c", /a#{x_slash}/.source
      w_slash = "w"
      assert_equal "/w\\/x/", (%r{#{w_slash}/x}).inspect
      assert_equal "w/", /#{w_slash}\//.source
      assert_equal "/w\\//", (/#{w_slash}\//).inspect
    end

    # \s includes \v
    def test_s_includes_v
      assert_equal 0, ("\v" =~ /\s/)
      assert_equal true, "a\vb".match?(/a\sb/)
      assert_equal false, "\v".match?(/\S/)
    end

    # to_json state options: space, indent, newlines, script_safe
    def test_to_json_state_options_space
      assert_equal "{\"a\": [1]}", ({ "a" => [1] }.to_json(space: " "))
      assert_equal "[\n  1,\n  {\n    \"a\": 2\n  }\n]", ([1, { "a" => 2 }].to_json(indent: "  ", object_nl: "\n", array_nl: "\n", space: " "))
      assert_equal "\"\\u2028\\/\"", (" /".to_json(script_safe: true))
    end

    # when with an untyped or optional Regexp is not a static match
    def test_when_with_an_untyped_or
      ur = /x/ #: untyped
      opt = /d/ #: Regexp?
      got = ["xy", "ad", "zz"].map do |s|
        case s
        when ur then "untyped"
        when opt then "optional"
        else "none"
        end
      end
      assert_equal ["untyped", "optional", "none"], got
    end

    # \b uses Unicode word characters
    def test_b_uses_unicode_word_characters
      assert_nil "café".match(/caf\b/)
      assert_nil "éb".match(/\bb/)
      assert_nil ("日本 x".match(/\b本/))
    end
  end

  class RxjsonMidTest < Minitest::Test
    # #{} in a regexp literal is evaluated exactly once
    def test_interpolation_evaluated_once
      ctr = Ctr.new
      interp_re = /x#{ctr.nxt}/
      assert_equal "x1", interp_re.source
      assert_equal true, interp_re.match?("x1")
      assert_equal "2", ctr.nxt
    end

    # to_json on optional elements and refs: nil stays null, user to_s/to_json used
    def test_to_json_optional_elements
      assert_equal "[[1],null]", [[1], nil].to_json
      assert_equal "{\"a\":[1],\"b\":null}", { "a" => [1], "b" => nil }.to_json
      assert_equal "{\"a\":{\"x\":1},\"b\":null}", { "a" => { "x" => 1 }, "b" => nil }.to_json
      assert_equal "[\"pt\",null]", [MidPt.new, nil].to_json
      js = [J.new] #: Array[J?]
      assert_equal "[{\"j\":1}]", js.to_json
      assert_equal "{\"j\":[{\"j\":1}]}", JSON.generate({ "j" => js })
      rs = [/a/] #: Array[Regexp?]
      assert_equal "[\"(?-mix:a)\"]", rs.to_json
      assert_equal "{\"m\":\"a\"}", { "m" => "ab".match(/(a)/) }.to_json
      assert_equal "[\"1\",null]", ["a1", "b"].map { |word| word.match(/\d/) }.to_json
    end

    # matching against invalid UTF-8 raises ArgumentError like MRI
    def test_invalid_utf8_subject_raises
      bad_utf8 = "a\xffb"
      assert_raises(ArgumentError) { bad_utf8.match?(/b/) }
      assert_raises(ArgumentError) { bad_utf8 =~ /b/ }
      assert_raises(ArgumentError) { bad_utf8.match(/(b)/) }
    end

    # an untyped Integer argument to match raises TypeError, not a Go panic
    def test_untyped_integer_subject_raises
      assert_raises(TypeError) { /a/.match?(five) }
      assert_raises(TypeError) { /a/ =~ five }
      err = begin # rugaroo's dynamic argument check (decision 20) words this TypeError differently from MRI's Regexp check
        "abc".match?(five)
        nil
      rescue TypeError => e
        e
      end
      assert_equal false, err.nil?
    end

    # untyped nil/Symbol subjects through dynamic and typed regexps
    def test_untyped_nil_and_symbol_subjects
      ur = /a/ #: untyped
      assert_equal false, ur.match?(nothing)
      assert_equal "nil", (ur =~ nothing).inspect
      assert_equal "nil", ur.match(nothing).inspect
      assert_equal false, /a/.match?(nothing)
      assert_equal "nil", (/a/ =~ nothing).inspect
      assert_equal "nil", /a/.match(nothing).inspect
      assert_equal true, /a/.match?(sym)
      assert_equal "1", (/a/ =~ sym).inspect
      assert_equal "#<MatchData \"a\" 1:\"a\">", /(a)/.match(sym).inspect
      assert_equal 0, "cat" =~ /#{sym}/
    end

    # sub/gsub with backrefs, blocks and String patterns
    def test_sub_and_gsub
      assert_equal "a#b#c#", "a1b22c333".gsub(/\d+/, "#")
      assert_equal "a<1>b2", "a1b2".sub(/\d/, "<\\0>")
      assert_equal "Smith, John", "John Smith".sub(/(\w+) (\w+)/, "\\2, \\1")
      assert_equal "a2b4", "a1b2".gsub(/\d/) { |d| (d.to_i * 2).to_s }
      assert_equal "hellO wOrld", "hello world".gsub(/o/) { |m| m.upcase }
      assert_equal "x+y", "x-y".gsub("-") { "+" }
      assert_equal "05/03/2024", "2024-03-05".sub(/(?<y>\d+)-(?<m>\d+)-(?<d>\d+)/, "\\k<d>/\\k<m>/\\k<y>")
      assert_equal "a!b", "a.b".gsub(".", "!")
      assert_equal "path\\to", "path/to".gsub("/", "\\\\")
      assert_equal "-a-b-c-", "abc".gsub(/x*/, "-")
      assert_equal "\\\\\\", "aaa".gsub(/a/, "\\\\")
      assert_equal "a[a|c]c", "abc".sub(/b/, "[\\`|\\']")
      assert_equal "a<>c", "abc".gsub(/(b)|(z)/, "<\\2>")
      assert_equal "camel_case_string", "camelCaseString".gsub(/([A-Z])/) { |m| "_" + m.downcase }
    end

    # scan and split with regexps
    def test_scan_and_split
      assert_equal ["1", "22", "333"], "a1b22c333".scan(/\d+/)
      assert_equal [["k1", "v1"], ["k2", "v2"]], "k1=v1; k2=v2".scan(/(\w+)=(\w+)/)
      assert_equal "[[\"a\", nil], [nil, \"b\"]]", "ab".scan(/(a)|(b)/).inspect
      assert_equal "[]", "none".scan(/\d/).inspect
      assert_equal ["a", "b", "c", "d"], "a, b,c ,d".split(/\s*,\s*/)
      assert_equal ["a", "b", "c"], "a1b2c3".split(/\d/)
      assert_equal ["a", "b", "c"], "abc".split(//)
      assert_equal ["a", "-", "b", "_", "c"], "a-b_c".split(/([-_])/)
      assert_equal ["1", "2"], "1,2,,".split(/,/)
      assert_equal ["", "a"], ",a".split(/,/)
      assert_equal ["one", "", "two"], "one  two".split(/ /)
      assert_equal "1,234,567", 1234567.to_s.reverse.scan(/\d{1,3}/).join(",").reverse
    end

    # Regexp.escape quotes metacharacters, space and whitespace escapes (MRI's rb_reg_quote).
    def test_regexp_escape
      assert_equal "a\\.b\\*c\\?d\\+e\\^f\\$g\\|h\\(i\\)j\\[k\\]l\\{m\\}n\\\\o/p\\-q\\ r\\tt\\nu\\#y",
                   Regexp.escape("a.b*c?d+e^f$g|h(i)j[k]l{m}n\\o/p-q r\tt\nu#y")
      assert_equal true, "1.5".match?(/\A#{Regexp.escape("1.5")}\z/)
      assert_equal false, "105".match?(/#{Regexp.escape("1.5")}/)
    end
  end

  class RxjsonBugToJsonSignatureTest < Minitest::Test
    # a to_json with an optional state is called with the generator state
    def test_optional_state_to_json
      assert_equal "\"opt\"", OptState.new.to_json
      assert_equal "[\"opt\"]", [OptState.new].to_json
      assert_equal "{\"o\":\"opt\"}", JSON.generate({ "o" => OptState.new })
    end

    # a zero-arity to_json works directly, but the generator passes it a state
    def test_zero_arity_to_json
      assert_equal "\"custom\"", NoArg.new.to_json
      e = assert_raises(ArgumentError) { [NoArg.new].to_json }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
    end
  end

  class RxjsonJsonScalarsTest < Minitest::Test
    def test_scalars_to_json
      assert_equal "\"\"", "".to_json
      assert_equal "\"plain\"", "plain".to_json
      assert_equal "\"a\\\"b\"", "a\"b".to_json
      assert_equal "\"back\\\\slash\"", "back\\slash".to_json
      assert_equal "\"a/b</script>\"", "a/b</script>".to_json
      assert_equal "\"\\n\\r\\t\\b\\f\"", "\n\r\t\b\f".to_json
      assert_equal "\"\\u0000\\u0001\\b\\u000b\\u000e\\u001f\"", "\u0000\u0001\u0008\u000b\u000e\u001f".to_json
      assert_equal "\"\u007F\\u001b\"", "\u007f\e".to_json
      assert_equal "\"héllo 日本 😀   \u2028\u2029\"", ("héllo 日本 😀     ".to_json)
      assert_equal "\"ümlaut\\n\"", "ümlaut\n".to_json
      assert_equal "\"\u2028\u2029\"", "\u2028\u2029".to_json
      assert_equal "\" ­\"", "\u00a0\u00ad".to_json
      assert_equal "\"\u{10FFFF}\"", "\u{10FFFF}".to_json
      assert_equal "\"😀\"", "\u{1F600}".to_json
      assert_equal "\"\u007F\"", "\u{7f}".to_json
      assert_equal "\"multi\\nline\\ttext with \\\"quotes\\\" and \\\\ back\"", ("multi\nline\ttext with \"quotes\" and \\ back".to_json)
      assert_equal "\"str\"", JSON.generate("str")
      assert_equal "\"\"", JSON.generate("")
      assert_equal "\"\\u0002\"", JSON.generate("\u0002")
      assert_equal "0", 0.to_json
      assert_equal "-1", -1.to_json
      assert_equal "42", 42.to_json
      assert_equal "123456789012345678", 123456789012345678.to_json
      assert_equal "-9223372036854775807", -9223372036854775807.to_json
      assert_equal "7", JSON.generate(7)
      assert_equal "0", JSON.generate(-0)
      assert_equal "true", true.to_json
      assert_equal "false", false.to_json
      assert_equal "null", nil.to_json
      assert_equal "true", JSON.generate(true)
      assert_equal "false", JSON.generate(false)
      assert_equal "null", JSON.generate(nil)
      assert_equal "\"sym\"", :sym.to_json
      assert_equal "\"with space\"", (:"with space".to_json)
      assert_equal "\"quote\\\"sym\"", :"quote\"sym".to_json
      assert_equal "\"é\"", :"é".to_json
      assert_equal "\"s\"", JSON.generate(:s)
      assert_equal "0.0", 0.0.to_json
      assert_equal "-0.0", -0.0.to_json
      assert_equal "1.0", 1.0.to_json
      assert_equal "-1.5", -1.5.to_json
      assert_equal "0.1", 0.1.to_json
      assert_equal "0.30000000000000004", (0.1 + 0.2).to_json
      assert_equal "0.3333333333333333", (1.0 / 3).to_json
      assert_equal "0.6666666666666666", (2.0 / 3).to_json
      assert_equal "2.5", JSON.generate(2.5)
      assert_equal "-0.0", JSON.generate(-0.0)
      assert_equal "1e+20", JSON.generate(1e20)
      floats = [
        1.5e-12, 1e-12, 1.5e-11, 1e-11, 1.5e-10, 1e-10, 1.5e-9, 1e-9,
        1.5e-8, 1e-8, 1.5e-7, 1e-7, 1.5e-6, 1e-6, 1.5e-5, 1e-5,
        1.5e-4, 1e-4, 1.5e-3, 1e-3, 1.5e-2, 1e-2, 1.5e-1, 1e-1,
        1.5e0, 1e0, 1.5e1, 1e1, 1.5e2, 1e2, 1.5e3, 1e3,
        1.5e4, 1e4, 1.5e5, 1e5, 1.5e6, 1e6, 1.5e7, 1e7,
        1.5e8, 1e8, 1.5e9, 1e9, 1.5e10, 1e10, 1.5e11, 1e11,
        1.5e12, 1e12, 1.5e13, 1e13, 1.5e14, 1e14, 1.5e15, 1e15,
        1.5e16, 1e16, 1.5e17, 1e17, 1.5e18, 1e18, 1.5e19, 1e19,
        1.5e20, 1e20, 1.5e21, 1e21, 1.5e22, 1e22, 1.5e23, 1.5e24, 1e24,
        1.2345678901234567e-7, 1.2345678901234567e14, 1.2345678901234567e16, 1.2345678901234567e22, 0.2,
        123.456, 1e300, 1e-300, 5e-324, 2.2250738585072014e-308, 1.7976931348623157e308, 9007199254740993.0, 99999999999999.9,
        999999999999999.9, 0.00001234, 12345678.9, 3.14159, 100.0, 12.5
      ] #: Array[Float]
      # each float and its negation, one to_json call apiece
      each_json = [
        "1.5e-12 -1.5e-12", "1e-12 -1e-12", "1.5e-11 -1.5e-11", "1e-11 -1e-11",
        "1.5e-10 -1.5e-10", "1e-10 -1e-10", "0.0000000015 -0.0000000015", "0.000000001 -0.000000001",
        "0.000000015 -0.000000015", "0.00000001 -0.00000001", "0.00000015 -0.00000015", "0.0000001 -0.0000001",
        "0.0000015 -0.0000015", "0.000001 -0.000001", "0.000015 -0.000015", "0.00001 -0.00001",
        "0.00015 -0.00015", "0.0001 -0.0001", "0.0015 -0.0015", "0.001 -0.001",
        "0.015 -0.015", "0.01 -0.01", "0.15 -0.15", "0.1 -0.1",
        "1.5 -1.5", "1.0 -1.0", "15.0 -15.0", "10.0 -10.0",
        "150.0 -150.0", "100.0 -100.0", "1500.0 -1500.0", "1000.0 -1000.0",
        "15000.0 -15000.0", "10000.0 -10000.0", "150000.0 -150000.0", "100000.0 -100000.0",
        "1500000.0 -1500000.0", "1000000.0 -1000000.0", "15000000.0 -15000000.0", "10000000.0 -10000000.0",
        "150000000.0 -150000000.0", "100000000.0 -100000000.0", "1500000000.0 -1500000000.0", "1000000000.0 -1000000000.0",
        "15000000000.0 -15000000000.0", "10000000000.0 -10000000000.0", "150000000000.0 -150000000000.0", "100000000000.0 -100000000000.0",
        "1500000000000.0 -1500000000000.0", "1000000000000.0 -1000000000000.0", "15000000000000.0 -15000000000000.0", "10000000000000.0 -10000000000000.0",
        "150000000000000.0 -150000000000000.0", "100000000000000.0 -100000000000000.0", "1.5e+15 -1.5e+15", "1e+15 -1e+15",
        "1.5e+16 -1.5e+16", "1e+16 -1e+16", "1.5e+17 -1.5e+17", "1e+17 -1e+17",
        "1.5e+18 -1.5e+18", "1e+18 -1e+18", "1.5e+19 -1.5e+19", "1e+19 -1e+19",
        "1.5e+20 -1.5e+20", "1e+20 -1e+20", "1.5e+21 -1.5e+21", "1e+21 -1e+21",
        "1.5e+22 -1.5e+22", "1e+22 -1e+22", "1.5e+23 -1.5e+23", "1.5e+24 -1.5e+24",
        "1e+24 -1e+24", "0.00000012345678901234566 -0.00000012345678901234566", "123456789012345.67 -123456789012345.67", "1.2345678901234568e+16 -1.2345678901234568e+16",
        "1.2345678901234568e+22 -1.2345678901234568e+22", "0.2 -0.2", "123.456 -123.456", "1e+300 -1e+300",
        "1e-300 -1e-300", "5e-324 -5e-324", "2.2250738585072014e-308 -2.2250738585072014e-308", "1.7976931348623157e+308 -1.7976931348623157e+308",
        "9.007199254740992e+15 -9.007199254740992e+15", "99999999999999.9 -99999999999999.9", "999999999999999.9 -999999999999999.9", "0.00001234 -0.00001234",
        "12345678.9 -12345678.9", "3.14159 -3.14159", "100.0 -100.0", "12.5 -12.5"
      ]
      assert_equal each_json, floats.map { |f| "#{f.to_json} #{(-f).to_json}" }
      assert_equal "[1.5e-12,1e-12,1.5e-11,1e-11,1.5e-10,1e-10,0.0000000015,0.000000001,0.000000015,0.00000001,0.00000015,0.0000001,0.0000015,0.000001,0.000015,0.00001,0.00015,0.0001,0.0015,0.001,0.015,0.01,0.15,0.1,1.5,1.0,15.0,10.0,150.0,100.0,1500.0,1000.0,15000.0,10000.0,150000.0,100000.0,1500000.0,1000000.0,15000000.0,10000000.0,150000000.0,100000000.0,1500000000.0,1000000000.0,15000000000.0,10000000000.0,150000000000.0,100000000000.0,1500000000000.0,1000000000000.0,15000000000000.0,10000000000000.0,150000000000000.0,100000000000000.0,1.5e+15,1e+15,1.5e+16,1e+16,1.5e+17,1e+17,1.5e+18,1e+18,1.5e+19,1e+19,1.5e+20,1e+20,1.5e+21,1e+21,1.5e+22,1e+22,1.5e+23,1.5e+24,1e+24,0.00000012345678901234566,123456789012345.67,1.2345678901234568e+16,1.2345678901234568e+22,0.2,123.456,1e+300,1e-300,5e-324,2.2250738585072014e-308,1.7976931348623157e+308,9.007199254740992e+15,99999999999999.9,999999999999999.9,0.00001234,12345678.9,3.14159,100.0,12.5]", floats.to_json
    end

    # NaN and Infinity have no JSON form: JSON::GeneratorError, a StandardError.
    def test_nan_and_infinity_have_no
      nan = 0.0 / 0.0
      inf = 1.0 / 0.0
      msgs = [nan, inf, -inf].map do |f|
        e = assert_raises(JSON::GeneratorError) { f.to_json }
        e.message
      end
      assert_equal ["NaN not allowed in JSON", "Infinity not allowed in JSON", "-Infinity not allowed in JSON"], msgs
      e1 = assert_raises(StandardError) { JSON.generate(-inf) }
      assert_equal "JSON::GeneratorError", e1.class.to_s
      assert_equal true, e1.is_a?(JSON::GeneratorError)
      e2 = assert_raises(JSON::GeneratorError) { [1.0, nan].to_json }
      assert_equal "NaN not allowed in JSON", e2.message
      e3 = assert_raises(JSON::GeneratorError) { JSON.generate({ "x" => inf }) }
      assert_equal "Infinity not allowed in JSON", e3.message
    end
  end

  class RxjsonJsonCollectionsTest < Minitest::Test
    def test_collections_to_json
      assert_equal "[]", [].to_json
      assert_equal "[[]]", [[]].to_json
      assert_equal "[1,[2,[3,[]]]]", ([1, [2, [3, []]]].to_json)
      assert_equal "[1,2,3]", ([1, 2, 3].to_json)
      assert_equal "[\"a\",\"b\\\"c\"]", (["a", "b\"c"].to_json)
      assert_equal "[true,false]", ([true, false].to_json)
      assert_equal "[\"a\",\"b\"]", ([:a, :b].to_json)
      assert_equal "[1.5,2.0]", ([1.5, 2.0].to_json)
      assert_equal "[null]", [nil].to_json
      assert_equal "[null,null]", ([nil, nil].to_json)
      assert_equal "{}", {}.to_json
      assert_equal "{\"a\":{}}", ({ "a" => {} }.to_json)
      assert_equal "{\"z\":1,\"a\":2,\"m\":[3],\"é/ü\":\"日本\"}", ({ "z" => 1, "a" => 2, "m" => [3], "é/ü" => "日本" }.to_json)
      assert_equal "{\"name\":\"sym\",\"é\":\"ünï\",\"nested\":{\"deep\":[true,{\"k\":\"v\"}]}}", ({ name: "sym", "é" => "ünï", nested: { deep: [true, { "k" => :v }] } }.to_json)
      assert_equal "{\"1\":\"one\",\"-2\":\"neg\",\"1.5\":\"float\",\"true\":\"t\",\"false\":\"f\"}", ({ 1 => "one", -2 => "neg", 1.5 => "float", true => "t", false => "f" }.to_json)
      assert_equal "{\"[1, 2]\":3,\"s\":4,\"x\\ny\":5}", ({ [1, 2] => 3, :s => 4, "x\ny" => 5 }.to_json)
      assert_equal "{\"Pt(9)\":\"user key\"}", ({ Pt.new(9) => "user key" }.to_json)
      h = {} #: Hash[String, Integer]
      h["b"] = 2
      h["a"] = 1
      h["b"] = 3
      assert_equal "{\"b\":3,\"a\":1}", h.to_json
      h.delete("b")
      assert_equal "{\"a\":1}", h.to_json
      assert_equal "{\"a\":1}", JSON.generate(h)
      assert_equal "\"Pt(3)\"", Pt.new(3).to_json
      assert_equal "[\"Pt(1)\",\"Pt(2)\"]", ([Pt.new(1), Pt.new(2)].to_json)
      assert_equal "{\"p\":\"Pt(4)\"}", (JSON.generate({ "p" => Pt.new(4) }))
      assert_equal "{\"id\":1,\"tags\":[\"a\"],\"pt\":\"Pt(1)\"}", (Post.new(1, ["a"]).to_json)
      assert_equal "{\"id\":null,\"tags\":[],\"pt\":\"Pt(1)\"}", (Post.new(nil, []).to_json)
      assert_equal "[{\"id\":2,\"tags\":[\"x\",\"y\"],\"pt\":\"Pt(1)\"}]", ([Post.new(2, ["x", "y"])].to_json)
      assert_equal "{\"posts\":[{\"id\":3,\"tags\":[],\"pt\":\"Pt(1)\"}]}", (JSON.generate({ "posts" => [Post.new(3, [])] }))
      assert_equal "{\"special\":{\"id\":4,\"tags\":[\"s\"],\"pt\":\"Pt(1)\"}}", (Special.new(4, ["s"]).to_json)
      assert_equal "[{\"special\":{\"id\":5,\"tags\":[],\"pt\":\"Pt(1)\"}}]", ([Special.new(5, [])].to_json)
      posts = [Post.new(6, []), Special.new(7, [])] #: Array[Post]
      assert_equal "[{\"id\":6,\"tags\":[],\"pt\":\"Pt(1)\"},{\"special\":{\"id\":7,\"tags\":[],\"pt\":\"Pt(1)\"}}]", posts.to_json
      assert_equal "\"#<struct RxjsonTests::P x=1, y=2>\"", (P.new(1, 2).to_json)
      assert_equal "[\"#<struct RxjsonTests::P x=3, y=4>\"]", ([P.new(3, 4)].to_json)
      assert_equal "\"#<data RxjsonTests::D a=\\\"q\\\">\"", D.new("q").to_json
      assert_equal "\"#<data RxjsonTests::D a=\\\"r\\\">\"", JSON.generate(D.new("r"))
      assert_equal "\"boom\"", StandardError.new("boom").to_json
      assert_equal "[\"bad\"]", [ArgumentError.new("bad")].to_json
      md = "k=v".match(/(\w)=(\w)/)
      assert_equal "\"(?mi-x:a.b)\"", /a.b/mi.to_json
      assert_equal "[\"(?-mix:x)\",\"(?-mix:\\\"q\\\")\"]", ([/x/, /"q"/].to_json)
      assert_equal "\"k=v\"", md.to_json
      assert_equal "{\"re\":\"(?-mix:\\\\d)\"}", (JSON.generate({ "re" => /\d/ }))
      pair = [1, "a"]
      assert_equal "[1,\"a\"]", pair.to_json
      assert_equal "[1,\"a\"]", JSON.generate(pair)
      triple = [2, "s", 1.5]
      assert_equal "[2,\"s\",1.5]", triple.to_json
      rows = [[1, "a"], [2, "b"]]
      assert_equal "[[1,\"a\"],[2,\"b\"]]", rows.to_json
      assert_equal "{\"rows\":[[1,\"a\"],[2,\"b\"]]}", ({ "rows" => rows }.to_json)
      u = [1, "two", nil, 3.0, [nil], { "k" => nil }, :sym, false] #: untyped
      assert_equal "[1,\"two\",null,3.0,[null],{\"k\":null},\"sym\",false]", u.to_json
      assert_equal "[1,\"two\",null,3.0,[null],{\"k\":null},\"sym\",false]", JSON.generate(u)
      un = nil #: untyped
      assert_equal "null", un.to_json
      assert_equal "null", JSON.generate(un)
      hu = { "a" => nil, "b" => 1, "c" => "s" } #: Hash[String, untyped]
      assert_equal "{\"a\":null,\"b\":1,\"c\":\"s\"}", hu.to_json
      x = nil #: Integer?
      y = 5 #: Integer?
      s = "q" #: String?
      assert_equal "null", x.to_json
      assert_equal "5", y.to_json
      assert_equal "\"q\"", s.to_json
      assert_equal "null", JSON.generate(x)
      assert_equal "5", JSON.generate(y)
      assert_equal "[2,4,6]", ([1, 2, 3].map { |i| i * 2 }.to_json)
      assert_equal "[\"a\",\"b\"]", (["b", "a"].sort.to_json)
      assert_equal "{\"k\":[\"1\",\"2\"]}", ({ "k" => [1, 2].map { |i| i.to_s } }.to_json)
      assert_equal "[1]", [1].to_json(nil)
      assert_equal "\"s\"", ("s".to_json(1, 2))
    end
  end

  class RxjsonJsonValuesTest < Minitest::Test
    def test_values_to_json
      assert_equal "{\"x\":1,\"y\":2}", (VP.new(1, 2).to_json)
      assert_equal "[{\"x\":3,\"y\":4}]", ([VP.new(3, 4)].to_json)
      assert_equal "{\"p\":{\"x\":5,\"y\":6}}", (JSON.generate({ "p" => VP.new(5, 6) }))
      assert_equal "[\"d\"]", VD.new("d").to_json
      assert_equal "{\"d\":[[\"e\"]]}", ({ "d" => [VD.new("e")] }.to_json)
      assert_equal "\"String\"", String.to_json
      assert_equal "\"RxjsonTests::Q\"", JSON.generate(Q)
      assert_equal "\"say \\\"hi\\\"\\n\"", Q.new.to_json
      assert_equal "[\"say \\\"hi\\\"\\n\"]", [Q.new].to_json
      assert_equal "[\"named\",\"parent\"]", ([Thing.new, Kid.new].to_json)
      assert_equal "{\"k\":\"parent\"}", (JSON.generate({ "k" => Kid.new }))
      assert_equal "\"parent\"", Kid.new.to_json
      assert_equal "[1]", [1].send(:to_json)
      assert_equal "\"x\"", "x".public_send(:to_json)
      assert_equal "{\"wrapped\":\"w\"}", Wrap.new.to_json
      assert_equal "[{\"wrapped\":\"w\"}]", [Wrap.new].to_json
      assert_equal "[{\"wrapped\":\"w\"}]", Str.new.to_json
      assert_equal "{\"s\":[[{\"wrapped\":\"w\"}]]}", (JSON.generate({ "s" => [Str.new] }))
      fh = { "a" => 1.5, "b" => 1e20 } #: Hash[String, Float]
      assert_equal "{\"a\":1.5,\"b\":1e+20}", fh.to_json
      fo = 2.5 #: Float?
      bo = true #: bool?
      assert_equal "2.5", fo.to_json
      assert_equal "true", bo.to_json
      sh = { a: [:x, :y], b: [] } #: Hash[Symbol, Array[Symbol]]
      assert_equal "{\"a\":[\"x\",\"y\"],\"b\":[]}", sh.to_json
      mk = { 1 => 2, "a" => 3, :s => 4 }
      assert_equal "{\"1\":2,\"a\":3,\"s\":4}", mk.to_json
      e = RuntimeError.new("bad \"q\"")
      assert_equal "\"bad \\\"q\\\"\"", e.to_json
      assert_equal "[\"bad \\\"q\\\"\"]", JSON.generate([e.message])
      assert_equal "[1,[2,\"x\"],{\"k\":null}]", ([1, [2, "x"], { "k" => nil }].to_json)
      assert_equal "[[\"x1\",true],[\"y2\",false]]", (["x1", "y2"].map { |s| [s, s.match?(/1/)] }.to_json)
      assert_equal "1", enc(1)
      assert_equal "\"s\"", enc("s")
      assert_equal "null", enc(nil)
      assert_equal "{\"a\":1}", (enc({ a: 1 }))
      assert_equal "\"say \\\"hi\\\"\\n\"", enc(Q.new)
      assert_equal "\"s\"", enc(:s)
      assert_equal "1.0", enc(1.0)
      assert_equal "true", enc(true)
      assert_equal "{\"x\":0,\"y\":0}", (enc(VP.new(0, 0)))
      um = "ab".match(/(b)/) #: untyped
      ur = /x/i #: untyped
      up = VP.new(7, 8) #: untyped
      assert_equal "\"b\"", JSON.generate(um)
      assert_equal "\"(?i-mx:x)\"", JSON.generate(ur)
      assert_equal "{\"x\":7,\"y\":8}", JSON.generate(up)
      assert_equal "[\"b\",\"(?i-mx:x)\",{\"x\":7,\"y\":8}]", (JSON.generate([um, ur, up]))
      assert_equal "{\"x\":7,\"y\":8}", up.to_json
    end

    # to_json returns a fresh String: later mutation of the source does not change it.
    def test_to_json_returns_a_fresh
      arr = [1, 2]
      s = arr.to_json
      arr << 3
      assert_equal "[1,2]", s
      assert_equal "[1,2,3]", arr.to_json
      assert_equal 9, ({ "k" => "v" }.to_json.length)
      assert_equal false, [].to_json.empty?
      assert_equal "x", JSON::GeneratorError.new("x").message
    end
  end

  class RxjsonJsonObjectsTest < Minitest::Test
    def test_objects_to_json
      u = Tag.new("a") #: untyped
      assert_equal "{\"tag\":\"a\"}", u.to_json
      assert_equal "{\"tag\":\"a\"}", JSON.generate(u)
      assert_equal "[{\"tag\":\"a\"}]", [u].to_json
      up = Plain.new #: untyped
      assert_equal "\"plain!\"", up.to_json
      assert_equal "{\"p\":\"plain!\"}", ({ "p" => up }.to_json)
      mixed = [Tag.new("x"), Plain.new, 1, nil, "s", Rec.new] #: Array[untyped]
      assert_equal "[{\"tag\":\"x\"},\"plain!\",1,null,\"s\",{\"r\":1}]", mixed.to_json
      assert_equal "[{\"tag\":\"x\"},\"plain!\",1,null,\"s\",{\"r\":1}]", JSON.generate(mixed)
      assert_equal "{\"r\":1}", Rec.new.to_json
      assert_equal "[{\"r\":1}]", [Rec.new].to_json
      assert_equal "{\"rec\":{\"r\":1}}", (JSON.generate({ "rec" => Rec.new }))
      assert_equal "{\"tag\":\"s\"}", SubTag.new("s").to_json
      assert_equal "[{\"tag\":\"t\"}]", [SubTag.new("t")].to_json
      assert_equal "{\"tag\":\"g\"}", JSON.generate(SubTag.new("g"))
      tags = [Tag.new("p"), SubTag.new("q")] #: Array[Tag]
      assert_equal "[{\"tag\":\"p\"},{\"tag\":\"q\"}]", tags.to_json
      assert_equal "{\"tags\":[{\"tag\":\"p\"},{\"tag\":\"q\"}],\"n\":2}", ({ "tags" => tags, "n" => tags.size }.to_json)
      assert_equal "{\"tags\":[\"p\",\"q\"]}", ({ tags: tags.map(&:name) }.to_json)
      assert_equal "{\"a\":[1,{\"b\":null}]}", ({ a: [1, { b: nil }] }.to_json)
      assert_equal "{\"a\":1,\"b\":\"two\"}", ({ a: 1, b: "two" }.to_json)
      deep = { "x" => [[1, 2], [3]] } #: Hash[String, Array[Array[Integer]]]
      assert_equal "{\"x\":[[1,2],[3]]}", deep.to_json
      assert_equal "{\"a\":{\"b\":{\"c\":[true,0.00001]}}}", (JSON.generate({ "a" => { "b" => { "c" => [true, 1.0e-5] } } }))
      assert_equal "[1,[2.5,[\"s\",[\"t\",[false]]]]]", ([1, [2.5, ["s", [:t, [false]]]]].to_json)
      ih = { 1 => [1], 2 => [] } #: Hash[Integer, Array[Integer]]
      assert_equal "{\"1\":[1],\"2\":[]}", ih.to_json
      assert_equal "{\"1.0e+20\":1,\"100.0\":2,\"-0.5\":3}", ({ 1e20 => 1, 100.0 => 2, -0.5 => 3 }.to_json)
      list = [] #: Array[String]
      list << "a" << "b"
      assert_equal "[\"a\",\"b\"]", list.to_json
      assert_equal "{\"k\":[\"A\",\"B\"]}", ({ "k" => list.map(&:upcase) }.to_json)
      empty = [] #: Array[Integer]
      eh = {} #: Hash[String, Integer]
      assert_equal "[]", empty.to_json
      assert_equal "{}", eh.to_json
      assert_equal "[[],[]]", ([empty, empty].to_json)
      j = [1, 2].to_json
      assert_equal "[1,2]!", "#{j}!"
      assert_equal 5, j.size
      assert_equal "String", (j.class).to_s
      assert_equal "[1,2]x", (j + "x")
      assert_equal "\"1\"", 1.to_json.to_json
      assert_equal "\"\\\"x\\\"\"", "x".to_json.to_json
      assert_equal "String", (:a.to_json.class).to_s
    end
  end

  class RxjsonJsonGeneratorStateTest < Minitest::Test
    def test_to_json_generator_state_options
      pretty = { indent: "  ", space: " ", object_nl: "\n", array_nl: "\n" }
      assert_equal "{\n  \"t\": {\n    \"tag\": \"a\",\n    \"kids\": [\n      1,\n      2\n    ]\n  },\n  \"e\": [],\n  \"h\": {}\n}", ({ "t" => GsTag.new("a"), "e" => [], "h" => {} }.to_json(pretty))
      assert_equal "[\n\t{\t\t\"tag\":\"b\",\t\t\"kids\":[\n\t\t\t1,\n\t\t\t2\n\t\t]},\n\t[\n\t\tfalse,\n\t\t1\n\t]\n]", ([GsTag.new("b"), Legacy.new].to_json(indent: "\t", array_nl: "\n"))
      assert_equal "{  \"a\":1,  \"b\":[    1,    2]}", ({ "a" => 1, "b" => [1, 2] }.to_json(indent: "  "))
      assert_equal "{\"a\" : 1}", ({ "a" => 1 }.to_json(space_before: " ", space: " "))
      assert_equal "[\n    1\n  ]", ([1].to_json(depth: 1, indent: "  ", array_nl: "\n"))
      assert_equal "\"\\ud83d\\ude00\\u00e9/\\u2029\"", ("😀é/ ".to_json(ascii_only: true))
      assert_equal "\"é\\/\"", ("é/".to_json(escape_slash: true))
      assert_equal "[NaN,-Infinity,1.5]", ([0.0 / 0, -1.0 / 0, 1.5].to_json(allow_nan: true))
      assert_equal "[1]", ([1].to_json("array_nl" => "\n"))
      assert_equal "[1]", ([1].to_json(array_nl: nil))
      assert_equal "\"sym\"", (:sym.to_json(script_safe: true))
      assert_equal "{\"\\/\":\"\\/\"}", ({ "/" => "/" }.to_json(script_safe: true))
      u = [1, { "k" => "v" }] #: untyped
      assert_equal "[\n1,\n{\"k\":\"v\"}\n]", (u.to_json(array_nl: "\n"))
      o = { "x" => 1 } #: Hash[String, Integer]?
      assert_equal "{\"x\": 1}", (o.to_json(space: " "))
      assert_equal "[ 1, \"two\" ]", ([1, "two"].to_json(space: " ", array_nl: " "))
      assert_equal "[[false,1]]", JSON.generate([Legacy.new])
      e = assert_raises(TypeError) { [1].to_json(indent: 5) }
      assert_equal "wrong argument type Integer (expected String)", e.message
      e2 = assert_raises(JSON::GeneratorError) { [1.0 / 0].to_json }
      assert_equal "Infinity not allowed in JSON", e2.message
    end
  end

  # Regexp.new / Regexp.compile from a String at run time (decision 115, #34).
  class RxjsonRegexpNewTest < Minitest::Test
    #: (String) -> String
    def rx_error(src)
      Regexp.new(src)
      "compiled"
    rescue RegexpError => e
      e.message
    end

    def test_new_from_string
      re = Regexp.new("a+")
      assert_equal true, re.match?("caat")
      assert_equal "/a+/", re.inspect
      assert_equal "a+", re.source
      assert_equal 0, re.options
      assert_equal "/a+/mi", Regexp.new("a+", "im").inspect
      assert_equal "/a/i", Regexp.new("a", Regexp::IGNORECASE).inspect
      assert_equal "/a/m", Regexp.new("a", Regexp::MULTILINE).inspect
      assert_equal "/a/i", Regexp.new("a", true).inspect
      assert_equal "/a/", Regexp.new("a", nil).inspect
      assert_equal 5, Regexp.new("a", "im").options
      assert_equal true, Regexp.new("A", "i").casefold?
      assert_equal "/x/i", Regexp.new(/x/i).inspect
      assert_equal "/q/", Regexp.compile("q").inspect
      assert_equal true, Regexp.new("x", "m").match?("x")
      assert_equal "ruby", Regexp.new("(?<lang>r\\w+)").match("I like ruby")&.[](1)
      assert_equal true, Regexp.new("\\Ahello\\z").match?("hello")
      assert_equal true, Regexp.new("^b$").match?("a\nb\nc")
      assert_equal ["ab", "AB"], %w[ab AB cd].grep(Regexp.new("ab", "i"))
      assert_equal Regexp.new("a+"), /a+/
    end

    def test_new_errors
      assert_equal "end pattern with unmatched parenthesis: /a(b/", rx_error("a(b")
      assert_equal "unmatched close parenthesis: /a)b/", rx_error("a)b")
      e = assert_raises(ArgumentError) { Regexp.new("a", "z") }
      assert_equal "unknown regexp option: z", e.message
    end
  end
end
