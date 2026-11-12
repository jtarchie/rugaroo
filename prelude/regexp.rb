# prelude/regexp.rb
# rbs_inline: enabled

#
# Regexp on Go's RE2. The transpiler translates literals (every Ruby regexp
# gets `(?m)`: `^`/`$` are line anchors in Ruby) and rejects what RE2 cannot
# match; see internal/compiler/regexp.go. rxRegexp (rxtranslate.go) matches
# Onigmo's `^`, `\b` and multi-character `/i` folds where RE2's differ.

# @go_type struct { re *rxRegexp; src string; opts string; anch *atomic.Pointer[rxRegexp]; u8 bool }
class Regexp < Object
  # Regexp.last_match(n): the compiler passes the caller's $~ (decision 232).
  #: (MatchData?, untyped) -> String?
  def self.__last_match_at(m, n) = m ? m[n] : nil

  IGNORECASE = 1 #: Integer
  EXTENDED = 2 #: Integer
  MULTILINE = 4 #: Integer
  FIXEDENCODING = 16 #: Integer
  NOENCODING = 32 #: Integer

  # A regexp2 match (lookaround, backreference) that ran past 10 seconds; RE2 matches are linear and never raise it (decision 242).
  class TimeoutError < RegexpError; end

  # A Regexp from a String at run time, translated from Ruby's syntax as an
  # interpolated literal is (decision 115). options: an Integer of
  # IGNORECASE | MULTILINE, a String of "m"/"i" flags, true for /i, or nil.
  #: (untyped, ?untyped) -> Regexp
  def self.new(pattern, options = nil) = %x{ return rbRegexpFromValue(pattern, options) }

  #: (untyped, ?untyped) -> Regexp
  def self.compile(pattern, options = nil) = %x{ return rbRegexpFromValue(pattern, options) }

  #: () -> Integer
  def options = %x{
    n := 0
    for _, f := range self.opts {
      n |= map[rune]int{'i': 1, 'x': 2, 'm': 4, 'u': 16, 'e': 16, 's': 16}[f]
    }
    return Integer(n)
  }

  #: () -> bool
  def casefold? = %x{ return Boolean(strings.ContainsRune(self.opts, 'i')) }

  # US-ASCII for an ASCII-only source, else UTF-8 (or with //u), as MRI derives it (decision 191).
  # ponytail: //n with a \xNN escape is BINARY in MRI and US-ASCII here.
  #: () -> Encoding
  def encoding = %x{
    if strings.ContainsRune(self.opts, 'u') || strings.Contains(self.src, `\\u`) || strings.IndexFunc(self.src, func(r rune) bool { return r >= 0x80 }) >= 0 {
      return Encoding_UTF_8
    }
    return Encoding_US_ASCII
  }

  # A subject is a String for typed callers; an untyped one may also be a
  # Symbol (its name is matched) or nil (no match), as in MRI.
  #: (String | untyped) -> bool
  def match?(s) = %x{
    str, ok := rbSubject(s)
    return Boolean(ok && rbMatches(self, str))
  }

  #: (String | untyped) -> MatchData?
  def match(s) = %x{
    str, ok := rbSubject(s)
    if !ok {
      return nil
    }
    m := rbMatch(self, str)
    if m == nil {
      return nil
    }
    return &m
  }

  # The first match starting at or after character pos (negative counts from the end), with the whole String as context.
  #: (String, Integer) -> MatchData?
  def __match_2(s, pos) = %x{
    str := string(s)
    n := Integer(utf8.RuneCountInString(str))
    if pos < 0 {
      pos += n
    }
    if pos < 0 || pos > n {
      return nil
    }
    from := len(string([]rune(str)[:pos]))
    var loc []int
    if rbValidUTF8(str) {
      loc = slices.Clone(rbFindFrom(self, str)(from))
    } else if loc = rbFindLoc(self, str[from:]); loc != nil { // ponytail: a binary subject loses the context before pos
      for i := range loc {
        if loc[i] >= 0 {
          loc[i] += from
        }
      }
    }
    if loc == nil {
      return nil
    }
    m := rbMatchAt(self, str, loc)
    return &m
  }

  # A backreference makes Onigmo's match non-linear; everything else here is (RE2, or regexp2's memoised lookarounds).
  #: (untyped) -> bool
  def self.linear_time?(re) = %x{ return Boolean(!rbHasBackref(rbRegexpFromValue(re, nil).src)) }

  # Whether the encoding is fixed: the u, e or s flag, or a non-ASCII source.
  #: () -> bool
  def fixed_encoding? = %x{
    return Boolean(strings.ContainsAny(self.opts, "ues") || strings.Contains(self.src, `\\u`) || strings.IndexFunc(self.src, func(r rune) bool { return r >= 0x80 }) >= 0)
  }

  # The character (not byte) index of the first match.
  #: (String | untyped) -> Integer?
  def =~(s) = %x{
    str, ok := rbSubject(s)
    if !ok {
      return rbOpt[Integer]{}
    }
    loc := rbFindLoc(self, str)
    if loc == nil {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(rbStrLen(str[:loc[0]])))
  }

  # Ruby's rb_reg_quote: metacharacters, space and the whitespace escapes.
  # A Symbol's name too, as MRI's (Homebrew's `Version.formula_optionally_versioned_regex(:glibc)`).
  #: (String | Symbol) -> String
  def self.escape(str) = %x{ return String(rbRegexpEscape(string(rbToS(any(str))))) }

  # `case x when /re/` calls this; unlike match?, other types are false.
  #: (untyped) -> bool
  def ===(other) = %x{
    switch other.(type) {
    case String, Symbol:
      s, _ := rbSubject(other)
      return Boolean(rbMatches(self, s))
    }
    return false
  }

  #: () -> String
  def source = %x{ String(self.src) }

  # A binary String subject (decision 352): each byte is a character, and =~ is a byte index.
  #: (String) -> Integer?
  def __bin_eqtilde(s) = %x{
    loc := rbFindLocBin(self, string(s))
    if loc == nil {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(loc[0]))
  }

  # @binary
  #: (String) -> MatchData?
  def __bin_match(s) = %x{
    loc := rbFindLocBin(self, string(s))
    if loc == nil {
      return nil
    }
    m := rbMatchAt(self, string(s), loc)
    m.bin = true // begin/end/offset count bytes (decision 352)
    return &m
  }

  #: (String) -> bool
  def __bin_match_q(s) = %x{ return Boolean(rbFindLocBin(self, string(s)) != nil) }

  #: (String) -> bool
  def __bin_eqq(s) = %x{ return Boolean(rbFindLocBin(self, string(s)) != nil) }

  # Whether the pattern has capture groups: String#scan then gives each match's groups (decision 331).
  #: () -> bool
  def __groups? = %x{ Boolean(self.re.NumSubexp() > 0) }

  #: (String | Symbol) -> String
  def self.quote(str) = escape(str)

  # A Regexp matching any of pats: a String matches itself, a Regexp keeps its flags (its to_s).
  #: (*untyped) -> Regexp
  def self.union(*pats)
    pats = pats[0] if pats.size == 1 && pats[0].is_a?(Array)
    # ponytail: MRI's empty union is /(?!)/, which RE2 cannot compile; this never-matching class inspects differently
    return Regexp.new("[^\\s\\S]") if pats.empty?
    return pats[0] if pats.size == 1 && pats[0].is_a?(Regexp)

    parts = pats.map { |x| x.is_a?(Regexp) ? x.to_s : Regexp.escape(x.to_s) }
    Regexp.new(parts.join("|"))
  end

  #: () -> Array[String]
  def names = %x{ return rbUniqNames(self.re.SubexpNames()) }

  #: () -> Hash[String, Array[Integer]]
  def named_captures
    out = {} #: Hash[String, Array[Integer]]
    __group_names.each_with_index do |n, i|
      next if n.empty?

      (out[n] ||= []) << i
    end
    out
  end

  #: () -> Array[String]
  def __group_names = %x{
    out := &Array[String]{}
    for _, n := range self.re.SubexpNames() {
      out.s = append(out.s, String(n))
    }
    return out
  }

  # Every Regexp is a literal, and literals are frozen.
  #: () -> bool
  def frozen? = true

  #: () -> self
  def freeze = self

  #: () -> String
  def inspect = %x{ String("/" + rbRegexpDesc(self.src) + "/" + strings.Trim(self.opts, "ues")) }

  #: () -> String
  def to_s = %x{
    var on, off strings.Builder
    for _, f := range "mix" {
      if strings.ContainsRune(self.opts, f) {
        on.WriteRune(f)
      } else {
        off.WriteRune(f)
      }
    }
    flags := on.String()
    if off.Len() > 0 {
      flags += "-" + off.String()
    }
    return String("(?" + flags + ":" + rbRegexpDesc(self.src) + ")")
  }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(*Regexp)
    return Boolean(ok && o.src == self.src && o.opts == self.opts)
  }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ Integer(rbHashString(self.src+"/"+strings.Trim(self.opts, "ues"))) }
end

# names are the groups' names ("" when unnamed), as Go's SubexpNames; loc holds byte offsets into subj.
# @go_type struct { groups []*String; names []string; pre string; post string; subj string; loc []int; bin bool }
class MatchData < Object
  # @binary self
  #: (Integer) -> String?
  def [](i) = %x{
    if i < 0 {
      // MRI (rb_reg_nth_match): a negative index never reaches group 0.
      i += Integer(len(self.groups))
      if i <= 0 {
        return nil
      }
    }
    if int(i) >= len(self.groups) {
      return nil
    }
    return self.groups[i]
  }

  # The whole match, then the groups.
  #: () -> Array[String?]
  def to_a = %x{
    out := &Array[*String]{}
    out.s = append(out.s, self.groups...)
    return out
  }

  #: () -> Array[String?]
  def captures = %x{
    out := &Array[*String]{}
    out.s = append(out.s, self.groups[1:]...)
    return out
  }

  # @binary self
  #: (String) -> String?
  def __idx_string(name) = %x{ return rbMatchNamed(self, string(name)) }

  # @binary self
  #: (Symbol) -> String?
  def __idx_symbol(name) = %x{ return rbMatchNamed(self, string(name)) }

  #: () -> Integer
  def size = %x{ Integer(len(self.groups)) }

  #: () -> Integer
  def length = size

  # @binary self
  #: () -> String
  def string = %x{ String(self.subj) }

  #: () -> Array[String]
  def names = %x{ return rbUniqNames(self.names) }

  #: () -> Hash[String, String?]
  def named_captures
    out = {} #: Hash[String, String?]
    names.each { |n| out[n] = self[n] }
    out
  end

  # Pattern matching: nil is every named capture; more keys than names is {}; else the keys up to the first non-name.
  #: (Array[Symbol]?) -> Hash[Symbol, String?]
  def deconstruct_keys(keys)
    out = {} #: Hash[Symbol, String?]
    if keys.nil?
      names.each { |n| out[n.to_sym] = self[n] }
      return out
    end
    return out if keys.size > names.size

    keys.each do |k|
      break unless names.include?(k.to_s)

      out[k] = __idx_symbol(k)
    end
    out
  end

  #: (*(Integer | String | Symbol)) -> Array[String?]
  def values_at(*idx)
    idx.map do |i|
      case i
      when Integer then self[i]
      when Symbol then __idx_symbol(i)
      else __idx_string(i)
      end
    end
  end

  # @binary self
  #: (Integer) -> String?
  def match(n)
    self.begin(n)
    self[n]
  end

  #: (Integer) -> Integer?
  def match_length(n) = match(n)&.size

  #: (Integer) -> Integer?
  def begin(n) = %x{ return rbOptFromPtr(rbMatchOffset(self, n, 0)) }

  #: (Integer) -> Integer?
  def end(n) = %x{ return rbOptFromPtr(rbMatchOffset(self, n, 1)) }

  #: (Integer) -> [Integer?, Integer?]
  def offset(n) = [self.begin(n), self.end(n)]

  #: (Integer) -> [Integer?, Integer?]
  def byteoffset(n) = %x{
    if n < 0 || int(n) >= len(self.groups) {
      panic(NewIndexError(Ref(String(fmt.Sprintf("index %d out of matches", n)))))
    }
    if self.loc[2*n] < 0 {
      return Tuple2[rbOpt[Integer], rbOpt[Integer]]{}
    }
    return Tuple2[rbOpt[Integer], rbOpt[Integer]]{rbSome(Integer(self.loc[2*n])), rbSome(Integer(self.loc[2*n+1]))}
  }

  # A named group's index (the last of a repeated name, as MRI), for the name twins of begin/end/offset/byteoffset.
  #: (String | Symbol) -> Integer
  def __group(name) = %x{
    var s string
    switch v := name.(type) {
    case String:
      s = string(v)
    case Symbol:
      s = string(v)
    }
    for i := len(self.names) - 1; i > 0; i-- {
      if self.names[i] == s {
        return Integer(i)
      }
    }
    panic(NewIndexError(Ref(String("undefined group name reference: " + s))))
  }

  #: (String) -> Integer?
  def __begin_string(name) = self.begin(__group(name))

  #: (Symbol) -> Integer?
  def __begin_symbol(name) = self.begin(__group(name))

  #: (String) -> Integer?
  def __end_string(name) = self.end(__group(name))

  #: (Symbol) -> Integer?
  def __end_symbol(name) = self.end(__group(name))

  #: (String) -> [Integer?, Integer?]
  def __offset_string(name) = offset(__group(name))

  #: (Symbol) -> [Integer?, Integer?]
  def __offset_symbol(name) = offset(__group(name))

  #: (String) -> [Integer?, Integer?]
  def __byteoffset_string(name) = byteoffset(__group(name))

  #: (Symbol) -> [Integer?, Integer?]
  def __byteoffset_symbol(name) = byteoffset(__group(name))

  # Ruby 3.4's bytebegin/byteend: a group's byte offsets (byteoffset's halves).
  #: (Integer) -> Integer?
  def bytebegin(n) = byteoffset(n)[0]

  #: (Integer) -> Integer?
  def byteend(n) = byteoffset(n)[1]

  # @binary self
  #: () -> String
  def pre_match = %x{ String(self.pre) }

  # @binary self
  #: () -> String
  def post_match = %x{ String(self.post) }

  # @binary self
  #: () -> String
  def to_s = self[0].to_s

  #: () -> String
  def inspect = %x{
    var b strings.Builder
    b.WriteString("#<MatchData ")
    for i, g := range self.groups {
      switch {
      case i == 0:
      case self.names[i] != "":
        b.WriteString(" " + self.names[i] + ":")
      default:
        fmt.Fprintf(&b, " %d:", i)
      }
      if g == nil {
        b.WriteString("nil")
      } else {
        b.WriteString(string(rbStringInspect(string(*g))))
      }
    }
    b.WriteString(">")
    return String(b.String())
  }
end

class String
  #: (Regexp) -> Integer?
  def =~(re) = re =~ self

  #: (Regexp) -> Integer?
  def __index_regexp(re) = re =~ self

  # The match or a group of it, by index or by name (Homebrew's `ref[REGEX, :name]`, decision 255).
  #: (Regexp, ?untyped) -> String?
  def __idx_regexp(re, n = 0)
    m = re.match(self)
    return nil unless m

    case n
    when Integer then m[n]
    when Symbol then m[n]
    when String then m[n]
    else raise TypeError, __idx_conv_message(n)
    end
  end

  # MRI's message for a bad group argument, from Go (an interpolated `n.class` cost every program with a dynamic `[]`
  # a thousand functions).
  #: (untyped) -> String
  def __idx_conv_message(n) = %x{ return String("no implicit conversion of " + rbClassName(n) + " into Integer") }

  #: (Regexp) -> bool
  def !~(re) = !(re =~ self)

  #: (Regexp) -> MatchData?
  def match(re) = re.match(self)

  # match with a block: the block's value on a match, else nil (decision 444).
  #: [U] (Regexp) { (MatchData) -> U } -> U?
  def __match_block(re)
    md = re.match(self)
    md.nil? ? nil : yield(md)
  end

  #: (Regexp) -> bool
  def match?(re) = re.match?(self)

  # A binary String's matches read its bytes (decision 352).
  #: (Regexp) -> Integer?
  def __bin_eqtilde(re) = re.__bin_eqtilde(self)

  # @binary
  #: (Regexp) -> MatchData?
  def __bin_match(re) = re.__bin_match(self)

  #: (Regexp) -> bool
  def __bin_match_q(re) = re.__bin_match_q(self)

  #: (Regexp) -> Integer?
  def __bin_index_regexp(re) = re.__bin_eqtilde(self)

  #: (Regexp) -> Integer?
  def __bin_rindex_regexp(re) = %x{
    s := string(self)
    for i := len(s); i >= 0; i-- {
      if loc := rbFindLocBin(re, s[i:]); loc != nil && loc[0] == 0 {
        return rbSome(Integer(i))
      }
    }
    return rbOpt[Integer]{}
  }

  # @binary
  #: (Regexp, ?untyped) -> String?
  def __bin_idx_regexp(re, n = 0)
    m = re.__bin_match(self)
    return nil unless m

    case n
    when Integer then m[n]
    when Symbol then m[n]
    when String then m[n]
    else raise TypeError, __idx_conv_message(n)
    end
  end

  # A match that starts at 0 (Go's leftmost match starts at 0 whenever one can). ponytail: $~ is not set; set it
  # when a caller reads it after start_with?.
  #: (Regexp) -> bool
  def __start_with_q_regexp(re) = %x{
    locs := rbFindAll(re, string(self), 1)
    return Boolean(len(locs) > 0 && locs[0][0] == 0)
  }

  # Replacement strings expand \0 \& \1-\9 \k<name> \` \' and \\.
  #: (Regexp, String) -> String
  def __sub_regexp(re, to) = %x{ String(rbReSub(re, string(self), 1, rbReRepl(string(to)))) }

  #: (Regexp, String) -> String
  def __gsub_regexp(re, to) = %x{ String(rbReSub(re, string(self), -1, rbReRepl(string(to)))) }

  # Each match is replaced by its value in the Hash, "" when it has none (nil.to_s), as MRI's.
  #: (Regexp, Hash[String, String]) -> String
  def __sub_regexp_hash(re, to) = %x{ String(rbReSub(re, string(self), 1, rbReHash(to))) }

  #: (Regexp, Hash[String, String]) -> String
  def __gsub_regexp_hash(re, to) = %x{ String(rbReSub(re, string(self), -1, rbReHash(to))) }

  # The block gets each match; its value's to_s replaces it.
  #: (untyped) { (String) -> untyped } -> String
  def __sub_block(pat) = %x{ String(rbReSub(rbPattern(pat), string(self), 1, rbReBlock(blk))) }

  #: (untyped) { (String) -> untyped } -> String
  def __gsub_block(pat) = %x{ String(rbReSub(rbPattern(pat), string(self), -1, rbReBlock(blk))) }

  # A pattern with groups yields each match's groups instead: that is
  # __scan_groups, which the compiler picks for a literal with groups.
  #: (Regexp) -> Array[String]
  def scan(re) = %x{
    if re.re.NumSubexp() > 0 {
      panic(NewNotImplementedError(Ref(String("rugaroo: String#scan with capture groups needs a Regexp literal"))))
    }
    return rbScanLocs(self, rbFindAll(re, string(self), -1))
  }

  # scan where the pattern is known only at run time (a gem's untyped receiver, decision 331): each match, or each
  # match's groups when the pattern has them, as MRI decides.
  #: (Regexp) -> untyped
  def __scan_any(re) = re.__groups? ? __scan_groups(re) : scan(re)

  # The block form: the same values, yielded; self after.
  #: (Regexp) { (untyped) -> untyped } -> String
  def __scan_block(re)
    if re.__groups?
      __scan_groups(re).each { |g| yield g }
    else
      scan(re).each { |m| yield m }
    end
    self
  end

  #: (Regexp) -> Array[Array[String?]]
  def __scan_groups(re) = %x{ return rbScanGroupLocs(self, rbFindAll(re, string(self), -1)) }

  # MRI's rb_str_split_m (rbSplitLocs). Its searches are the matches of scan, so they come from FindAll.
  #: (Regexp, ?Integer) -> Array[String]
  def __split_regexp(re, limit = 0) = %x{
    s := string(self)
    return rbSplitLocs(s, rbFindAll(re, s, -1), int(limit), !rbValidUTF8(s) && strings.ContainsRune(re.opts, 'n')) // rbFindAll's byte path
  }

  # A binary String's matches read its bytes (decision 352).
  #: (Regexp) -> Array[String]
  def __bin_scan(re) = %x{
    if re.re.NumSubexp() > 0 {
      panic(NewNotImplementedError(Ref(String("rugaroo: String#scan with capture groups needs a Regexp literal"))))
    }
    return rbScanLocs(self, rbFindAllBin(re, string(self), -1))
  }

  #: (Regexp) -> untyped
  def __bin_scan_any(re) = re.__groups? ? __bin_scan_groups(re) : __bin_scan(re)

  #: (Regexp) { (untyped) -> untyped } -> String
  def __bin_scan_block(re)
    if re.__groups?
      __bin_scan_groups(re).each { |g| yield g }
    else
      __bin_scan(re).each { |m| yield m }
    end
    self
  end

  #: (Regexp) -> Array[Array[String?]]
  def __bin_scan_groups(re) = %x{ return rbScanGroupLocs(self, rbFindAllBin(re, string(self), -1)) }

  #: (Regexp, ?Integer) -> Array[String]
  def __bin_split_regexp(re, limit = 0) = %x{ return rbSplitLocs(string(self), rbFindAllBin(re, string(self), -1), int(limit), true) }

  # @binary
  #: (Regexp, String) -> String
  def __bin_sub_regexp(re, to) = %x{ String(rbReSubBin(re, string(self), 1, rbReRepl(string(to)))) }

  # @binary
  #: (Regexp, String) -> String
  def __bin_gsub_regexp(re, to) = %x{ String(rbReSubBin(re, string(self), -1, rbReRepl(string(to)))) }

  # @binary block
  #: (untyped) { (String) -> untyped } -> String
  def __bin_sub_block(pat) = %x{ String(rbReSubBin(rbPattern(pat), string(self), 1, rbReBlock(blk))) }

  # @binary block
  #: (untyped) { (String) -> untyped } -> String
  def __bin_gsub_block(pat) = %x{ String(rbReSubBin(rbPattern(pat), string(self), -1, rbReBlock(blk))) }
end

# A Symbol matches as its name does (regexp_parser's `token.token =~ /\A(?:zero_or_one|...)\z/x`).
class Symbol
  #: (Regexp) -> Integer?
  def =~(re) = re =~ name

  #: (Regexp) -> bool
  def !~(re) = !(re =~ name)

  #: (Regexp) -> MatchData?
  def match(re) = re.match(name)

  #: (Regexp) -> bool
  def match?(re) = re.match?(name)
end

class RegexpError < StandardError; end
