# prelude/string.rb
# rbs_inline: enabled

#
# String as a named Go string (frozen: no mutating methods).

# @go_type string
class String < Object
  # A String as it is, else nil (rugaroo has no implicit to_str conversion to try).
  #: (untyped) -> String?
  def self.try_convert(obj) = %x{
    if v, ok := rbUnbox(obj).(String); ok {
      return &v
    }
    return nil
  }

  # What slice!(range) leaves (decision 233): self without the part self[range] is.
  #: (Range[Integer]) -> String
  def __rest_of_slice(r)
    got = self[r]
    return self if got.nil?

    start = r.begin
    start += size if start < 0
    self[0, start].to_s + self[(start + got.size)..].to_s
  end

  # What a gem's `s[range] = v` leaves (decision 324): self with the part self[range] is replaced by v.
  #: (Range[Integer], String) -> String
  def __with_range(r, v)
    s = r.__slice(length)
    raise RangeError, "#{r.inspect} out of range" unless s

    __with_span(s[0], s[1], v)
  end

  # `s[start, count] = v` (and `s[i] = v`, a count of 1), as a new String.
  #: (Integer, Integer, String) -> String
  def __with_span(start, count, v)
    start += length if start < 0
    raise IndexError, "index #{start} out of string" if start < 0 || start > length

    self[0, start].to_s + v + self[(start + count)..].to_s
  end

  include Comparable

  #: (String) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  # == on two Strings (decision 12's class overload): a plain Go compare,
  # inlined, with no boxing of the argument into untyped.
  #: (String) -> bool
  def __eq_string(other) = %x{ Boolean(self == other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(String)
    return Boolean(ok && self == o)
  }

  #: (String) -> String
  def +(other) = %x{
    if len(self) != 0 && len(other) != 0 && len(self) < 64 { // inlines: two non-empty operands make a new String, never one of them
      return self + other
    }
    return rbStrPlus(self, other)
  }

  # @binary self
  #: (Integer) -> String
  def *(n) = %x{
    if n < 0 {
      panic(NewArgumentError(Ref[String]("negative argument")))
    }
    if len(self) > 0 {
      rbCheckAlloc(int(n), len(self))
    }
    return rbNewStr(self, String(strings.Repeat(string(self), int(n))))
  }

  #: () -> String
  def to_s = self

  #: () -> String
  def to_str = self

  #: () -> String
  def inspect = %x{ rbStringInspect(string(self)) }

  # @binary self
  #: () -> String
  def dup = %x{ rbStrClone(self) }

  # `+s` is s when not frozen, else an unfrozen copy; `-s` is s when
  # frozen, else a frozen copy (MRI also dedups it).
  # @binary self
  #: () -> String
  def +@ = frozen? ? dup : self

  # @binary self
  #: () -> String
  def -@ = frozen? ? self : dup.freeze

  #: () -> String
  def upcase = %x{ rbNewStr(self, String(rbCaseMap(string(self), 2))) }

  #: () -> String
  def downcase = %x{ rbNewStr(self, String(rbCaseMap(string(self), 0))) }

  # Titlecase the first character (ǆ -> ǅ, ß -> Ss), downcase the rest.
  #: () -> String
  def capitalize = %x{
    s := string(self)
    if s == "" {
      return ""
    }
    if rbASCII(s) {
      return String(rbCapitalizeASCII(s))
    }
    if !utf8.ValidString(s) { // a binary String: ASCII letters only, its other bytes kept (decision 324)
      return String(rbCaseMap(s[:1], 2) + rbCaseMap(s[1:], 0))
    }
    _, n := utf8.DecodeRuneInString(s)
    return rbNewStr(self, String(rbCaseMap(s[:n], 1)+rbCaseMap(s[n:], 0))) // one unchanged character would be self
  }

  #: () -> String
  def reverse = %x{
    if rbASCII(string(self)) || !utf8.ValidString(string(self)) { // ASCII-8BIT: byte by byte (decision 324)
      if self == "" {
        return rbStrClone(self)
      }
      return String(rbReverseBytes(string(self)))
    }
    r := []rune(string(self))
    for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
      r[i], r[j] = r[j], r[i]
    }
    return String(r)
  }

  # MRI's whitespace is ASCII only, plus NUL for strip; not unicode.IsSpace.
  #: () -> String
  def strip = %x{ rbNewStr(self, String(strings.Trim(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  #: () -> String
  def lstrip = %x{ rbNewStr(self, String(strings.TrimLeft(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  #: () -> String
  def rstrip = %x{ rbNewStr(self, String(strings.TrimRight(string(self), " \\t\\n\\v\\f\\r\\x00"))) }

  # chomp's default is $/: "\n" also takes "\r\n" and "\r", nil leaves the String as it is
  #: () -> String
  def chomp
    rs = $/
    return self if rs.nil?
    return __chomp_1(rs) unless rs == "\n"

    __chomp_nl
  end

  #: () -> String
  def __chomp_nl = %x{ rbNewStr(self, String(strings.TrimSuffix(strings.TrimSuffix(string(self), "\\n"), "\\r"))) }

  # chomp(suffix): that suffix once ("" is MRI's paragraph mode: every trailing newline).
  #: (String) -> String
  def __chomp_1(suffix) = %x{
    switch suffix {
    case "": // every trailing \\n or \\r\\n, not a lone \\r
      s := string(self)
      for strings.HasSuffix(s, "\\n") {
        s = strings.TrimSuffix(strings.TrimSuffix(s, "\\n"), "\\r")
      }
      return String(s)
    case "\\n":
      return String(strings.TrimSuffix(strings.TrimSuffix(string(self), "\\n"), "\\r"))
    }
    return String(strings.TrimSuffix(string(self), string(suffix)))
  }

  #: () -> Integer
  def size = %x{ Integer(rbStrLen(string(self))) }

  #: () -> Integer
  def length = %x{ Integer(rbStrLen(string(self))) }

  #: () -> Integer
  def bytesize = %x{ Integer(len(self)) }

  #: () -> bool
  def empty? = %x{ self == "" }

  #: (String) -> bool
  def end_with?(s) = %x{ Boolean(strings.HasSuffix(string(self), string(s))) }

  #: (String) -> bool
  def start_with?(s) = %x{ Boolean(strings.HasPrefix(string(self), string(s))) }

  #: (String, String) -> bool
  def __start_with_q_2(a, b) = start_with?(a) || start_with?(b)

  #: (String, String, String) -> bool
  def __start_with_q_3(a, b, c) = start_with?(a) || start_with?(b) || start_with?(c)

  #: (String, String) -> bool
  def __end_with_q_2(a, b) = end_with?(a) || end_with?(b)

  #: (String, String, String) -> bool
  def __end_with_q_3(a, b, c) = end_with?(a) || end_with?(b) || end_with?(c)

  #: (String) -> String
  def delete_prefix(s) = %x{ rbNewStr(self, String(strings.TrimPrefix(string(self), string(s)))) }

  #: (String) -> String
  def delete_suffix(s) = %x{ rbNewStr(self, String(strings.TrimSuffix(string(self), string(s)))) }

  #: (String) -> [String, String, String]
  def partition(sep) = %x{
    before, after, ok := strings.Cut(string(self), string(sep))
    if !ok {
      return Tuple3[String, String, String]{rbNewStr(self, self), "", ""}
    }
    return Tuple3[String, String, String]{String(before), rbNewStr(sep, sep), String(after)}
  }

  #: (String) -> [String, String, String]
  def rpartition(sep) = %x{
    i := strings.LastIndex(string(self), string(sep))
    if i < 0 {
      return Tuple3[String, String, String]{"", "", rbNewStr(self, self)}
    }
    return Tuple3[String, String, String]{self[:i], rbNewStr(sep, sep), self[i+len(sep):]}
  }

  #: (String) -> bool
  def include?(s) = %x{ Boolean(strings.Contains(string(self), string(s))) }

  # Strings are values in rugaroo (decision 136): a new one is its argument's bytes.
  #: (String) -> String
  def self.new(s) = s

  # MRI's String.new with no argument is an empty BINARY String (decision 365).
  # @binary
  #: () -> String
  def self.__new_0 = ""

  #: (String) -> Integer?
  def index(s) = %x{
    if v, ok := rbStrIndex(self, s); ok {
      return rbSome(v)
    }
    return rbOpt[Integer]{}
  }

  #: (String, Integer) -> Integer?
  def __index_2(s, start) = %x{
    r := []rune(string(self))
    if start < 0 {
      start += Integer(len(r))
    }
    if start < 0 || int(start) > len(r) {
      return rbOpt[Integer]{}
    }
    rest := string(r[start:])
    i := strings.Index(rest, string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(start + Integer(utf8.RuneCountInString(rest[:i])))
  }

  #: (Integer) -> String?
  def [](i) = %x{
    if v, ok := rbStrAt(self, i); ok {
      return &v
    }
    return nil
  }

  # The String itself when self contains it, else nil.
  #: (String) -> String?
  def __idx_string(str) = include?(str) ? str : nil

  #: (Range[Integer]) -> String?
  def __idx_range(r) = %x{
    if v, ok := rbStrRange(self, r); ok {
      return &v
    }
    return nil
  }

  #: (Integer, Integer) -> String?
  def __idx_2(start, count) = %x{
    if v, ok := rbStrSub(self, start, count); ok {
      return &v
    }
    return nil
  }

  #: () -> Array[Integer]
  def bytes = %x{
    out := &Array[Integer]{s: make([]Integer, len(self))}
    for i := range len(self) {
      out.s[i] = Integer(self[i])
    }
    return out
  }

  #: () -> Array[String]
  def chars = %x{
    out := &Array[String]{}
    for c := range rbEachChar(string(self)) {
      out.s = append(out.s, String(c))
    }
    return out
  }

  #: () -> Enumerator[String]
  def __each_char_enum = %x{ return rbEnumOf(self.EachChar(), any(self), "each_char", rbSizeOf(Integer(rbStrLen(string(self)))), nil) }

  #: () { (String) -> void } -> void
  def each_line
    lines.each { |l| yield l }
  end

  #: () -> Enumerator[String]
  def __each_line_enum = %x{ return rbEnumOf(Array_Each(self.Lines()), any(self), "each_line", nil, nil) }

  #: () { (String) -> void } -> void
  def each_char = %x{
    return func(yield func(String) bool) {
      for c := range rbEachChar(string(self)) {
        if !yield(String(c)) {
          return
        }
      }
    }
  }

  #: () -> Array[String]
  def lines = %x{
    out := &Array[String]{}
    for _, l := range strings.SplitAfter(string(self), "\\n") {
      if l != "" {
        out.s = append(out.s, rbNewStr(self, String(l)))
      }
    }
    return out
  }

  # Without a separator, or with " " (awk mode): split on ASCII whitespace,
  # dropping empties. With one: split on it, dropping trailing empties, like MRI.
  #: (?String?) -> Array[String]
  def split(sep = nil) = %x{
    out := &Array[String]{}
    if sep == nil { // $; when the program set it (decision 414)
      sep = rbFS
    }
    if sep == nil || *sep == " " {
      isSpace := func(r rune) bool { return r == ' ' || r >= '\\t' && r <= '\\r' }
      for _, f := range strings.FieldsFunc(string(self), isSpace) {
        out.s = append(out.s, rbNewStr(self, String(f)))
      }
      return out
    }
    str, sp := string(self), string(*sep)
    if sp == "" {
      for _, p := range strings.Split(str, "") {
        out.s = append(out.s, String(p))
      }
      return out
    }
    out.s = make([]String, 0, strings.Count(str, sp)+1) // straight into the result: no []string to copy from
    for {
      i := strings.Index(str, sp)
      if i < 0 {
        break
      }
      out.s = append(out.s, String(str[:i]))
      str = str[i+len(sp):]
    }
    out.s = append(out.s, rbNewStr(self, String(str)))
    for len(out.s) > 0 && out.s[len(out.s)-1] == "" {
      out.s = out.s[:len(out.s)-1]
    }
    return out
  }

  #: (String, String) -> String
  def sub(from, to) = %x{ rbNewStr(self, String(rbSub(string(self), string(from), string(to), 1))) }

  #: (String, String) -> String
  def gsub(from, to) = %x{ rbNewStr(self, String(rbSub(string(self), string(from), string(to), -1))) }

  #: (String, String) -> String
  def tr(from, to) = %x{ rbTr(self, string(from), string(to), !utf8.ValidString(string(self)), false, false) }

  # tr, then a run of translated characters giving one character keeps one. ponytail: characters, not a binary String's bytes.
  #: (String, String) -> String
  def tr_s(from, to) = %x{ rbTr(self, string(from), string(to), false, false, true) }

  # An Array argument supplies every directive's value, as MRI's.
  #: (untyped) -> String
  def %(arg) = %x{
    if a, ok := rbUnbox(arg).(Array_Any); ok {
      return String(rbFormat(string(self), a._ToAny().s))
    }
    return String(rbFormat(string(self), []any{arg}))
  }

  # Base 0 reads a 0b/0o/0x prefix (or a bare leading 0 as octal); a
  # prefix matching the base is skipped. Invalid digits end the number.
  #: (Integer) -> Integer
  def __to_i_1(base) = %x{
    if base < 0 {
      panic(NewArgumentError(Ref(String(fmt.Sprintf("invalid radix %d", base)))))
    }
    return Integer(rbStrToIBase(string(self), int(base)))
  }

  #: () -> Integer
  def hex = %x{ Integer(rbStrToIBase(string(self), 16)) }

  # Octal, unless a 0b/0o/0x prefix says otherwise.
  #: () -> Integer
  def oct = %x{ Integer(rbStrToIBase(string(self), -8)) }

  # MRI's String#succ: the rightmost alphanumeric increments with carry
  # (a digit to a digit, a letter to a letter of its case), growing on the
  # left; without alphanumerics the rightmost character increments.
  #: () -> String
  def succ = %x{ String(rbStrSucc(string(self))) }

  #: () -> String
  def next = succ

  # One character set only (no intersection of several, as MRI's).
  #: (String) -> Integer
  def count(chars) = %x{
    in := rbCharSet(string(chars))
    n := 0
    for _, r := range string(self) {
      if in(r) {
        n++
      }
    }
    return Integer(n)
  }

  #: () -> String
  def squeeze = %x{ String(rbSqueeze(string(self), func(rune) bool { return true })) }

  #: (String) -> String
  def __squeeze_1(chars) = %x{ String(rbSqueeze(string(self), rbCharSet(string(chars)))) }

  #: () -> String
  def swapcase = %x{ rbNewStr(self, String(rbSwapcase(string(self), false))) }

  # One option Symbol (decision 12's arity overloads): :ascii maps ASCII letters only, :turkic adds the dotted/dotless
  # i pairs, :lithuanian is the plain mapping; :fold is downcase's alone. ponytail: two options are not taken.
  #: (Symbol) -> String
  def __upcase_1(opt)
    case __case_opt(opt, false)
    when 1 then __ascii_case(2)
    when 2 then __case_x(6)
    else upcase
    end
  end

  #: (Symbol) -> String
  def __downcase_1(opt)
    case __case_opt(opt, true)
    when 1 then __ascii_case(0)
    when 2 then __case_x(4)
    when 3 then __case_x(3)
    else downcase
    end
  end

  #: (Symbol) -> String
  def __capitalize_1(opt)
    case __case_opt(opt, false)
    when 1 then empty? ? "" : self[0].to_s.__ascii_case(2) + self[1..].to_s.__ascii_case(0)
    when 2 then empty? ? "" : self[0].to_s.__case_x(5) + self[1..].to_s.__case_x(4)
    else capitalize
    end
  end

  #: (Symbol) -> String
  def __swapcase_1(opt)
    case __case_opt(opt, false)
    when 1 then __ascii_case(3)
    when 2 then __swap_turkic
    else swapcase
    end
  end

  #: (Symbol, bool) -> Integer
  def __case_opt(opt, down) = %x{ Integer(rbCaseOpt(opt, bool(down))) }

  #: (Integer) -> String
  def __ascii_case(mode) = %x{ rbNewStr(self, String(rbASCIICase(string(self), int(mode)))) }

  # x/text caser k (rbCaseX's kinds: 3 fold, 4 lower, 5 title, 6 upper, the last three Turkish).
  #: (Integer) -> String
  def __case_x(k) = %x{ rbNewStr(self, String(rbCaseX(int(k), string(self)))) }

  #: () -> String
  def __swap_turkic = %x{ rbNewStr(self, String(rbSwapcase(string(self), true))) }

  # ASCII-only case folding, as MRI's casecmp.
  #: (String) -> Integer
  def casecmp(other) = %x{
    lower := func(s string) string {
      return strings.Map(func(r rune) rune {
        if r >= 'A' && r <= 'Z' {
          return r + 32
        }
        return r
      }, s)
    }
    return Integer(strings.Compare(lower(string(self)), lower(string(other))))
  }

  #: (String) -> bool
  # Unicode's full case folding, as MRI's (ß matches SS; decision 380).
  def casecmp?(other) = %x{
    a, b := string(self), string(other)
    if rbASCII(a) && rbASCII(b) {
      return Boolean(strings.EqualFold(a, b))
    }
    return Boolean(rbCaseX(rbCaseFold, a) == rbCaseX(rbCaseFold, b))
  }

  # Drops the last character; a trailing \r\n goes as one.
  #: () -> String
  def chop = %x{
    s := string(self)
    if strings.HasSuffix(s, "\\r\\n") {
      return String(s[:len(s)-2])
    }
    _, w := utf8.DecodeLastRuneInString(s)
    return String(s[:len(s)-w])
  }

  #: () -> String
  def chr = %x{
    _, w := utf8.DecodeRuneInString(string(self))
    return self[:w]
  }

  #: () -> bool
  def ascii_only? = %x{
    for i := range len(self) {
      if self[i] >= 0x80 {
        return false
      }
    }
    return true
  }

  # A positive limit caps the field count, the last field keeping the rest;
  # a negative one keeps trailing empty fields.
  #: (String?, Integer) -> Array[String]
  def __split_2(sep, limit) = %x{
    out := &Array[String]{}
    for _, p := range rbSplitLimit(string(self), sep, int(limit)) {
      out.s = append(out.s, String(p))
    }
    return out
  }

  # slice is [] (decision 416: its forms mirror []'s, so an untyped receiver's call picks the same one).
  #: (Integer) -> String?
  def slice(i) = self[i]

  #: (Integer, Integer) -> String?
  def __slice_2(start, len) = self[start, len]

  # slice(range) on an untyped receiver (a typed call is []).
  #: (Range[Integer]) -> String?
  def __slice_range(r) = __idx_range(r)

  # slice's other forms, as []'s (net-ftp's `lines.last.slice(/\A([0-9a-zA-Z]{3})-/, 1)` on an untyped line, decision 416).
  #: (String) -> String?
  def __slice_string(str) = __idx_string(str)

  #: (Regexp, ?untyped) -> String?
  def __slice_regexp(re, n = 0) = __idx_regexp(re, n)

  # One character set only (no intersection of several, as MRI's).
  #: (String) -> String
  def delete(chars) = tr(chars, "")

  #: () -> Integer
  def to_i = %x{
    if n, ok := rbDigits(string(self)); ok {
      return Integer(n)
    }
    m := rbIntPrefix.FindStringSubmatch(string(self))
    if m == nil {
      return 0
    }
    digits := m[1] + strings.ReplaceAll(m[2], "_", "")
    n, err := strconv.Atoi(digits)
    if errors.Is(err, strconv.ErrRange) { // MRI returns a Bignum (decision 35)
      panic(NewRangeError(Ref(String(digits + " overflows Integer (64-bit; no Bignum)"))))
    }
    return Integer(n)
  }

  #: () -> Float
  def to_f = %x{
    m := rbFloatPrefix.FindStringSubmatch(string(self))
    if m == nil {
      return 0
    }
    f, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], "_", ""), 64)
    return Float(f)
  }

  #: () -> Integer
  def ord = %x{
    if self == "" {
      panic(NewArgumentError(Ref[String]("empty string")))
    }
    r, n := utf8.DecodeRuneInString(string(self))
    if r == utf8.RuneError && n == 1 {
      return Integer(self[0]) // a binary byte, e.g. 200.chr
    }
    return Integer(r)
  }

  #: () -> Integer
  def hash = %x{
    h := uint64(14695981039346656037) // FNV-1a, inline: hash/fnv's Write would copy the String to bytes per call
    for i := range len(self) {
      h = (h ^ uint64(self[i])) * 1099511628211
    }
    return Integer(h)
  }

  # @binary self
  #: () -> String
  def freeze = %x{
    rbStrFrozen(string(self), true)
    return self
  }

  # Literals are frozen (frozen_string_literal); strings built at run time
  # are not until frozen. See rbStrFrozen.
  #: () -> bool
  def frozen? = %x{ Boolean(rbStrFrozen(string(self), false)) }

  #: (Integer, ?String) -> String
  def center(width, pad = " ") = %x{
    n := int(width) - utf8.RuneCountInString(string(self))
    if pad == "" {
      panic(NewArgumentError(Ref(String("zero width padding"))))
    }
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(rbPad(string(pad), n/2) + string(self) + rbPad(string(pad), n-n/2))
  }

  #: (Integer, ?String) -> String
  def ljust(width, pad = " ") = %x{
    n := int(width) - rbStrLen(string(self))
    if pad == "" {
      panic(NewArgumentError(Ref(String("zero width padding"))))
    }
    if n <= 0 {
      return rbStrClone(self)
    }
    return self + String(rbPad(string(pad), n))
  }

  #: (Integer, ?String) -> String
  def rjust(width, pad = " ") = %x{
    n := int(width) - rbStrLen(string(self))
    if pad == "" {
      panic(NewArgumentError(Ref(String("zero width padding"))))
    }
    if n <= 0 {
      return rbStrClone(self)
    }
    return String(rbPad(string(pad), n)) + self
  }

  #: (String) -> Integer?
  def rindex(s) = %x{
    if v, ok := rbStrRindex(self, s); ok {
      return rbSome(v)
    }
    return rbOpt[Integer]{}
  }

  # The last occurrence starting at or before character pos (negative counts from the end).
  #: (String, Integer) -> Integer?
  def __rindex_2(s, pos) = %x{
    r := []rune(string(self))
    if pos < 0 {
      pos += Integer(len(r))
    }
    if pos < 0 {
      return rbOpt[Integer]{}
    }
    hay := string(r[:min(int(pos)+utf8.RuneCountInString(string(s)), len(r))])
    i := strings.LastIndex(hay, string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(utf8.RuneCountInString(hay[:i])))
  }

  #: (String) -> Integer?
  def byteindex(s) = %x{
    i := strings.Index(string(self), string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(i))
  }

  # The Regexp twins of index, rindex, partition and rpartition (decision 12's `__<name>_regexp`).
  # rindex and rpartition match with the whole String as context (rbRindexLoc), so \A, ^, \b and lookbehind see their neighbours.
  #: (Regexp) -> Integer?
  def __index_regexp(re) = re =~ self

  #: (Regexp) -> Integer?
  def __rindex_regexp(re) = %x{
    loc := rbRindexLoc(re, string(self))
    if loc == nil {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(rbStrLen(string(self)[:loc[0]])))
  }

  #: (Regexp) -> Integer?
  def __byteindex_regexp(re)
    m = re.match(self)
    m ? m.pre_match.bytesize : nil
  end

  #: (Regexp) -> Integer?
  def __byterindex_regexp(re)
    i = __rindex_regexp(re)
    i ? (self[0, i] || "").bytesize : nil
  end

  #: (Regexp) -> [String, String, String]
  def __partition_regexp(re)
    m = re.match(self)
    return [self, "", ""] unless m

    [m.pre_match, m[0] || "", m.post_match]
  end

  #: (Regexp) -> [String, String, String]
  def __rpartition_regexp(re) = %x{
    loc := rbRindexLoc(re, string(self))
    if loc == nil {
      return Tuple3[String, String, String]{"", "", rbNewStr(self, self)}
    }
    return Tuple3[String, String, String]{self[:loc[0]], self[loc[0]:loc[1]], self[loc[1]:]}
  }

  # byteindex(s, offset): from byte offset (negative from the end); MRI's IndexError off a character boundary.
  #: (String, Integer) -> Integer?
  def __byteindex_2(s, offset) = %x{
    n := Integer(len(self))
    if offset < 0 {
      offset += n
    }
    if offset < 0 || offset > n {
      return rbOpt[Integer]{}
    }
    if offset < n && !utf8.RuneStart(self[offset]) {
      panic(NewIndexError(Ref(String(fmt.Sprintf("offset %d does not land on character boundary", offset)))))
    }
    i := strings.Index(string(self[offset:]), string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(i) + offset)
  }

  #: (String, Integer) -> Integer?
  def __byterindex_2(s, offset) = %x{
    n := Integer(len(self))
    if offset < 0 {
      offset += n
    }
    if offset < 0 {
      return rbOpt[Integer]{}
    }
    end := min(int(offset)+len(s), len(self))
    i := strings.LastIndex(string(self[:end]), string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(i))
  }

  #: (String) -> Integer?
  def byterindex(s) = %x{
    i := strings.LastIndex(string(self), string(s))
    if i < 0 {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(i))
  }

  #: (Range[Integer]) -> String?
  def __byteslice_range(r)
    span = r.__slice(bytesize)
    span ? byteslice(span[0], span[1]) : nil
  end

  # One byte without a length, as MRI (byteslice(3) of "abc" is nil, byteslice(3, 1) is "").
  # @binary self
  #: (Integer, ?Integer?) -> String?
  def byteslice(start, count = nil) = %x{
    n := Integer(len(self))
    if start < 0 {
      start += n
    }
    if !count.ok {
      if start < 0 || start >= n {
        return nil
      }
      return Ref(String(self[start : start+1]))
    }
    if start < 0 || start > n || count.v < 0 {
      return nil
    }
    return Ref(String(strings.Clone(string(self[start:min(start+count.v, n)]))))
  }

  #: (Integer) -> Integer?
  def getbyte(i) = %x{
    if i < 0 {
      i += Integer(len(self))
    }
    if i < 0 || int(i) >= len(self) {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(self[i]))
  }

  #: () -> Array[Integer]
  def codepoints = %x{
    out := &Array[Integer]{}
    for _, r := range string(self) {
      out.s = append(out.s, Integer(r))
    }
    return out
  }

  #: () { (Integer) -> void } -> void
  def each_byte = %x{
    return func(yield func(Integer) bool) {
      for i := range len(self) {
        if !yield(Integer(self[i])) {
          return
        }
      }
    }
  }

  #: () { (Integer) -> void } -> void
  def each_codepoint = %x{
    return func(yield func(Integer) bool) {
      for _, r := range string(self) {
        if !yield(Integer(r)) {
          return
        }
      }
    }
  }

  #: (?Integer) -> Integer
  def sum(bits = 16) = %x{
    var t uint64
    for i := range len(self) {
      t += uint64(self[i])
    }
    if bits > 0 && bits < 64 {
      t &= 1<<bits - 1
    }
    return Integer(t)
  }

  #: (String) { (String) -> void } -> void
  def upto(last)
    if __digits? && last.__digits?
      i = to_i
      while i <= last.to_i
        yield i.to_s.rjust(size, "0")
        i += 1
      end
      return
    end
    return if (self <=> last) > 0

    s = self
    while true
      yield s
      break if s == last

      s = s.succ
      break if s.size > last.size || s.empty?
    end
  end

  #: () -> bool
  def __digits? = %x{ Boolean(self != "" && strings.Trim(string(self), "0123456789") == "") }

  #: () -> Symbol
  def intern = to_sym

  # A binary String: the compiler reads it byte by byte from here (decision 352).
  # @binary
  #: () -> String
  def b = %x{ rbStrClone(self) }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(String)
    return Boolean(ok && o == self)
  }

  #: () -> String
  def dump = %x{ String(rbStrDump(string(self))) }

  #: () -> String
  def undump = %x{ return rbStrUndump(string(self)) }

  #: (String, ?offset: Integer) -> Array[untyped]
  def unpack(format, offset: 0) = %x{ return rbUnpack(string(self), string(format), offset) }

  #: (String, ?offset: Integer) -> untyped
  def unpack1(format, offset: 0) = %x{
    if out := rbUnpack(string(self), string(format), offset).s; len(out) > 0 {
      return out[0]
    }
    return nil
  }

  # A gem's unpack (decision 324): an unsigned value past 2**63 is its 64 bits as an Integer, as the gem's Integer#<< wraps.
  #: (String, ?offset: Integer) -> Array[untyped]
  def __unpack_wrap(format, offset: 0) = %x{ return rbUnpackWrap(string(self), string(format), true, offset) }

  #: (String, ?offset: Integer) -> untyped
  def __unpack1_wrap(format, offset: 0) = %x{
    if out := rbUnpackWrap(string(self), string(format), true, offset).s; len(out) > 0 {
      return out[0]
    }
    return nil
  }
end
