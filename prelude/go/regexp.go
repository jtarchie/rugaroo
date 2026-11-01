//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// rbRegexpOf wraps a compiled pattern; anch is the slot StringScanner fills with the \A-anchored twin, so the twin lives and dies with the Regexp.
func rbRegexpOf(re *rxRegexp, src, opts string) *Regexp {
	return &Regexp{re: re, src: src, opts: opts, anch: new(atomic.Pointer[rxRegexp]), u8: rbRegexpUTF8(src, opts)}
}

// rbRxCompiled is a compiled pattern and its \A twin's slot, shared by every Regexp built from the same pattern.
type rbRxCompiled struct {
	pat  string
	re   *rxRegexp
	anch *atomic.Pointer[rxRegexp]
	lazy bool // compiled by rxCompileLazy: not validated, so a strict caller compiles its own
}

// rbRxCache remembers the last pattern compiled into each of its slots (by hash), so an interpolated regexp rebuilt
// with the same text (a parser's per-instance patterns, a loop's /#{x}/) compiles once; each build is still its own
// Regexp. Direct-mapped: bounded, no lock, and a collision only costs a recompile (decision 332).
var rbRxCache [256]atomic.Pointer[rbRxCompiled]

// rbRegexpCompiled is rxCompile through rbRxCache, as a Regexp; lazy compiles what only RE2 rejects at its first use
// (rxCompileLazy).
func rbRegexpCompiled(pat, src, opts string, lazy bool) (*Regexp, error) {
	h := uint32(2166136261)
	for i := range len(pat) {
		h = (h ^ uint32(pat[i])) * 16777619
	}
	slot := &rbRxCache[h%uint32(len(rbRxCache))]
	if c := slot.Load(); c != nil && c.pat == pat && (lazy || !c.lazy) {
		return &Regexp{re: c.re, src: src, opts: opts, anch: c.anch, u8: rbRegexpUTF8(src, opts)}, nil
	}
	compile := rxCompile
	if lazy {
		compile = func(p string) (*rxRegexp, error) { return rxCompileLazy(p, src) }
	}
	re, err := compile(pat)
	if err != nil {
		return nil, err
	}
	c := &rbRxCompiled{pat, re, new(atomic.Pointer[rxRegexp]), lazy}
	slot.Store(c)
	return &Regexp{re: re, src: src, opts: opts, anch: c.anch, u8: rbRegexpUTF8(src, opts)}, nil
}

// rbRegexpNew compiles a translated pattern; dynamic (interpolated) ones
// may fail at run time, like Ruby's.
func rbRegexpNew(pattern, src, opts string) *Regexp {
	r, err := rbRegexpCompiled(rxFold(pattern), src, opts, false)
	if err != nil {
		panic(NewRegexpError(Ref(String(rbRegexpErr(err, src)))))
	}
	return r
}

// rbRxAlt is r, built where the regexp2 engine must be in the program: naming rxAltReady keeps it (decisions 242, 432).
func rbRxAlt(_ bool, r *Regexp) *Regexp { return r }

// rbRegexpLenient is a gem's regexp (decision 176): MRI builds it, so rugaroo does too, and one RE2 cannot match raises
// RegexpError when it is used rather than when the gem loads (Rack::Lint's URI patterns).
func rbRegexpLenient(prefix, src, opts string) *Regexp {
	pat, err := rxPrepare(src, strings.Contains(opts, "x"))
	var r *Regexp
	if err == nil {
		r, err = rbRegexpCompiled(rxFold(prefix+pat), src, opts, true)
	}
	if err != nil {
		return rbRegexpOf(&rxRegexp{unsupported: "regexp /" + src + "/ is not supported by rugaroo: " + err.Error()}, src, opts)
	}
	return r
}

// rbRegexpErr words a Go regexp syntax error as Onigmo words the same
// mistake, with the Ruby source after it (`end pattern with unmatched
// parenthesis: /a(b/`); an error with no Onigmo twin keeps Go's wording.
func rbRegexpErr(err error, src string) string {
	var se *syntax.Error
	if !errors.As(err, &se) {
		return err.Error()
	}
	msg := ""
	// the codes with an Onigmo twin; the rest keep Go's wording
	//exhaustive:ignore
	switch se.Code {
	case syntax.ErrMissingParen:
		msg = "end pattern with unmatched parenthesis"
	case syntax.ErrUnexpectedParen:
		msg = "unmatched close parenthesis"
	case syntax.ErrMissingBracket:
		msg = "premature end of char-class"
	case syntax.ErrMissingRepeatArgument:
		msg = "target of repeat operator is not specified"
	case syntax.ErrTrailingBackslash:
		msg = "too short escape sequence"
	case syntax.ErrInvalidCharRange:
		msg = "empty range in char class"
	case syntax.ErrInvalidRepeatSize:
		if lo, hi, ok := strings.Cut(strings.Trim(se.Expr, "{}"), ","); ok && hi != "" && len(hi) <= len(lo) && hi < lo {
			msg = "upper is smaller than lower in repeat range"
		}
	}
	if msg == "" {
		return err.Error()
	}
	return msg + ": /" + rbRegexpDesc(src) + "/"
}

// rbRegexpDyn compiles an interpolated regexp from its Ruby source, which
// only exists at run time; translateRegexp is the compiler's own
// (internal/compiler/rxtranslate.go, emitted into every program).
func rbRegexpDyn(prefix, src, opts string) *Regexp {
	// the interpolated values' spacing too under /x, as MRI's; an inline (?x-mi:...) from a value either way
	pat, err := rxPrepare(src, strings.Contains(opts, "x"))
	if err != nil {
		panic(NewRegexpError(Ref(String(err.Error()))))
	}
	return rbRegexpNew(prefix+pat, src, opts)
}

// rbRegexpFromValue is Regexp.new(pattern, options): a Regexp is copied
// (options ignored, as MRI warns); a String is translated like an
// interpolated literal's source.
func rbRegexpFromValue(pattern, options any) *Regexp {
	switch p := rbUnbox(pattern).(type) {
	case *Regexp:
		return p
	case String:
		ignore, multi, ext, noenc := false, false, false, false
		switch o := rbUnbox(options).(type) {
		case nil:
		case Boolean:
			ignore = bool(o)
		case Integer:
			ignore, multi, ext, noenc = o&1 != 0, o&4 != 0, o&2 != 0, o&32 != 0
		case String:
			for _, f := range string(o) {
				switch f {
				case 'i':
					ignore = true
				case 'm':
					multi = true
				case 'x':
					ext = true
				default:
					panic(NewArgumentError(Ref(String("unknown regexp option: " + string(o)))))
				}
			}
		default:
			ignore = true // any other truthy value is /i, as MRI
		}
		prefix, opts := "(?m", ""
		if ignore {
			prefix += "i"
		}
		if multi {
			prefix += "s"
			opts += "m"
		}
		if ignore {
			opts += "i"
		}
		if ext {
			opts += "x"
		}
		if noenc { // ponytail: only shown (/n); matching stays UTF-8 as RE2's
			opts += "n"
		}
		return rbRegexpDyn(prefix+")", string(p), opts)
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(pattern) + " into String"))))
}

// rbRegexpDesc is the source as inspect and to_s show it: a bare / is
// escaped, as MRI's rb_reg_desc does.
func rbRegexpDesc(src string) string {
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '\\' && i+1 < len(src):
			b.WriteString(src[i : i+2])
			i++
		case src[i] == '/':
			b.WriteString(`\/`)
		default:
			b.WriteByte(src[i])
		}
	}
	return b.String()
}

func rbMatch(r *Regexp, s string) *MatchData {
	loc := rbFindLoc(r, s)
	if loc == nil {
		return nil
	}
	return rbMatchAt(r, s, loc)
}

// rbFindLoc is r's first match of s with its groups, as byte offsets. A binary Regexp (/n) reads a String that is not
// valid UTF-8 byte by byte: each byte is the character of its value (Latin-1), so `/\xFD7zXZ/n` matches the byte 0xFD
// (Homebrew's UnpackStrategy::Xz.can_extract?), and the offsets are mapped back to the bytes (decision 324). Any other
// Regexp raises ArgumentError on it, as MRI's does on a UTF-8 String with a bad byte.
func rbFindLoc(r *Regexp, s string) []int {
	if rbValidUTF8(s) {
		return r.re.FindStringSubmatchIndex(s)
	}
	if !strings.ContainsRune(r.opts, 'n') { // MRI's UTF-8 Regexp on a broken UTF-8 String
		panic(NewArgumentError(Ref(String("invalid byte sequence in UTF-8"))))
	}
	t, back := rbLatin1(s)
	loc := r.re.FindStringSubmatchIndex(t)
	rbLatin1Back(loc, back)
	return loc
}

// rbFindLocBin is rbFindLoc for a String the compiler knows is binary (decision 352): its bytes past ASCII are
// characters of their value even where they form valid UTF-8 ("é".b is two).
func rbFindLocBin(r *Regexp, s string) []int {
	if !rbHighBytes(r, s) {
		return r.re.FindStringSubmatchIndex(s)
	}
	t, back := rbLatin1(s)
	loc := r.re.FindStringSubmatchIndex(t)
	rbLatin1Back(loc, back)
	return loc
}

// rbFindAllBin is rbFindAll for a String the compiler knows is binary (decision 352), as rbFindLocBin is rbFindLoc.
func rbFindAllBin(r *Regexp, s string, n int) [][]int {
	if !rbHighBytes(r, s) {
		return r.re.FindAllStringSubmatchIndex(s, n)
	}
	t, back := rbLatin1(s)
	locs := r.re.FindAllStringSubmatchIndex(t, n)
	for _, loc := range locs {
		rbLatin1Back(loc, back)
	}
	return locs
}

// rbScanLocs is String#scan's matches at locs.
func rbScanLocs(s String, locs [][]int) *Array[String] {
	out := &Array[String]{}
	for _, loc := range locs {
		out.s = append(out.s, s[loc[0]:loc[1]])
	}
	return out
}

// rbScanGroupLocs is String#scan's rows of groups at locs, for a pattern with groups.
func rbScanGroupLocs(s String, locs [][]int) *Array[*Array[*String]] {
	out := &Array[*Array[*String]]{}
	for _, loc := range locs {
		row := &Array[*String]{}
		for i := 2; i < len(loc); i += 2 {
			if loc[i] < 0 {
				row.s = append(row.s, nil)
			} else {
				row.s = append(row.s, Ref(s[loc[i]:loc[i+1]]))
			}
		}
		out.s = append(out.s, row)
	}
	return out
}

// rbSplitLocs is MRI's rb_str_split_m over the matches at locs: captured groups are kept, an empty match at the field
// start does not split, and a pattern matching the empty string splits between characters (bytes when bin).
func rbSplitLocs(s string, locs [][]int, lim int, bin bool) *Array[String] {
	out := &Array[String]{}
	if s == "" {
		return out
	}
	if lim == 1 {
		out.s = append(out.s, String(s))
		return out
	}
	beg, start, fields, lastNull := 0, 0, 1, false
	field := func(end int, loc []int) bool { // pushes s[beg:end] and the groups; reports the limit reached
		out.s = append(out.s, String(s[beg:end]))
		for i := 2; i < len(loc); i += 2 {
			if loc[i] >= 0 {
				out.s = append(out.s, String(s[loc[i]:loc[i+1]]))
			}
		}
		fields++
		return lim > 0 && lim <= fields
	}
	for _, loc := range locs {
		w := 1
		if !bin {
			_, w = utf8.DecodeRuneInString(s[loc[1]:])
		}
		if loc[0] == start && loc[1] == start { // empty at the search start: skip a character, then split before the next match
			if !lastNull {
				start += max(w, 1)
				lastNull = true
				continue
			}
			done := field(start, loc)
			beg = start
			if done {
				break
			}
		} else {
			done := field(loc[0], loc)
			beg, start = loc[1], loc[1]
			if done {
				break
			}
			if loc[0] != loc[1] {
				lastNull = false
				continue
			}
		}
		start += max(w, 1)
		lastNull = true
	}
	if lim != 0 || len(s) > beg {
		out.s = append(out.s, String(s[beg:]))
	}
	for lim == 0 && len(out.s) > 0 && out.s[len(out.s)-1] == "" {
		out.s = out.s[:len(out.s)-1]
	}
	return out
}

// rbHighBytes reports whether binary s has a byte past ASCII, so a match reads it byte by byte; then a Regexp fixed to
// UTF-8 (rbRegexpUTF8) raises MRI's Encoding::CompatibilityError.
func rbHighBytes(r *Regexp, s string) bool {
	if !strings.ContainsFunc(s, func(c rune) bool { return c >= utf8.RuneSelf }) {
		return false
	}
	if r.u8 {
		panic(NewEncoding_CompatibilityError(Ref(String("incompatible encoding regexp match (UTF-8 regexp with BINARY (ASCII-8BIT) string)"))))
	}
	return true
}

// rbRegexpUTF8 reports a source fixed to UTF-8 without /n: a non-ASCII character, or an unescaped \u or \x escape
// past 0x7F (Regexp#u8, read by rbHighBytes).
func rbRegexpUTF8(src, opts string) bool {
	if strings.ContainsRune(opts, 'n') {
		return false
	}
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] >= utf8.RuneSelf:
			return true
		case src[i] != '\\' || i+1 == len(src):
		case src[i+1] == 'u', src[i+1] == 'x' && i+2 < len(src) && strings.IndexByte("89abcdefABCDEF", src[i+2]) >= 0:
			return true
		default:
			i++ // the escaped character
		}
	}
	return false
}

// rbLatin1 reads each byte of s as the character of its value, for a binary Regexp (/n) on a String that is not valid
// UTF-8; back[j] is the byte of s where offset j of the result starts.
func rbLatin1(s string) (string, []int) {
	var b strings.Builder
	back := make([]int, 0, len(s)*2+1)
	for i := range len(s) {
		n := b.Len()
		b.WriteRune(rune(s[i]))
		for range b.Len() - n {
			back = append(back, i)
		}
	}
	return b.String(), append(back, len(s))
}

// rbLatin1Back maps rbLatin1 offsets in loc back to bytes of the original String.
func rbLatin1Back(loc, back []int) {
	for i, x := range loc {
		if x >= 0 {
			loc[i] = back[x]
		}
	}
}

// rbFindAll is r's first n matches of s (all when n < 0) with their groups, as byte offsets: rbFindLoc's byte reading
// for a binary Regexp on a String that is not valid UTF-8 (scan, sub, gsub, split; decision 324). Any other pair
// keeps rxRegexp's FindAll.
func rbFindAll(r *Regexp, s string, n int) [][]int {
	if rbValidUTF8(s) || !strings.ContainsRune(r.opts, 'n') {
		return r.re.FindAllStringSubmatchIndex(s, n)
	}
	t, back := rbLatin1(s)
	locs := r.re.FindAllStringSubmatchIndex(t, n)
	for _, loc := range locs {
		rbLatin1Back(loc, back)
	}
	return locs
}

// rbMatches is whether r matches s: Go's MatchString, or rbFindLoc's byte reading of a binary String.
func rbMatches(r *Regexp, s string) bool {
	if rbValidUTF8(s) {
		return r.re.MatchString(s)
	}
	return rbFindLoc(r, s) != nil
}

// rbMatchAt is the MatchData of r's match of s at loc.
func rbMatchAt(r *Regexp, s string, loc []int) *MatchData {
	groups := make([]*String, len(loc)/2)
	for i := range groups {
		if loc[2*i] >= 0 {
			g := String(s[loc[2*i]:loc[2*i+1]])
			groups[i] = &g
		}
	}
	return &MatchData{groups: groups, names: r.re.SubexpNames(), pre: s[:loc[0]], post: s[loc[1]:], subj: s, loc: loc}
}

// rbSubject is the text a Regexp matches: a String, or a Symbol's name.
// nil matches nothing (ok is false); anything else is MRI's TypeError, and
// a String that is not valid UTF-8 is MRI's ArgumentError.
func rbSubject(v any) (string, bool) {
	switch s := rbUnbox(v).(type) { // a String? held untyped is a *String box (liquid's `markup = $2; markup =~ re`, decision 331)
	case nil:
		return "", false
	case String: // not valid UTF-8: ASCII-8BIT (decision 136), matched byte by byte (rbFindLoc)
		return string(s), true
	case Symbol:
		return string(s), true
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into String"))))
}

// rbPattern is a sub/gsub pattern: a Regexp, or a String matched literally.
func rbPattern(p any) *Regexp {
	switch v := rbUnbox(p).(type) {
	case *Regexp:
		return v
	case String:
		return rbRegexpNew(regexp.QuoteMeta(string(v)), string(v), "")
	}
	panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(p) + " (expected Regexp)"))))
}

// rbReRepl expands a replacement string's back-references for one match.
func rbReRepl(rep string) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, names []string) string {
		group := func(i int) string {
			if 2*i+1 >= len(loc) || loc[2*i] < 0 {
				return ""
			}
			return s[loc[2*i]:loc[2*i+1]]
		}
		var b strings.Builder
		for j := 0; j < len(rep); j++ {
			if rep[j] != '\\' || j+1 == len(rep) {
				b.WriteByte(rep[j])
				continue
			}
			j++
			switch c := rep[j]; {
			case c == '&':
				b.WriteString(group(0))
			case c >= '0' && c <= '9':
				b.WriteString(group(int(c - '0')))
			case c == '+': // the last group that matched, empty with none
				for i := len(loc)/2 - 1; i >= 1; i-- {
					if loc[2*i] >= 0 {
						b.WriteString(group(i))
						break
					}
				}
			case c == '`':
				b.WriteString(s[:loc[0]])
			case c == '\'':
				b.WriteString(s[loc[1]:])
			case c == '\\':
				b.WriteByte('\\')
			case c == 'k' && j+1 < len(rep) && rep[j+1] == '<':
				end := strings.IndexByte(rep[j:], '>')
				if end < 0 {
					b.WriteString(rep[j-1 : j+1])
					continue
				}
				name := rep[j+2 : j+end]
				if i := slices.Index(names, name); i > 0 {
					b.WriteString(group(i))
				}
				j += end
			default:
				b.WriteString(rep[j-1 : j+1])
			}
		}
		return b.String()
	}
}

// rbReBlock replaces each match with the block's value, as a String.
func rbReBlock(blk func(String) any) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, _ []string) string {
		return string(rbToS(blk(String(s[loc[0]:loc[1]]))))
	}
}

// rbReHash replaces a match by its value in h (sub/gsub with a Hash).
func rbReHash(h *Hash[String, String]) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, _ []string) string {
		v, _ := h.rbGet(String(s[loc[0]:loc[1]]))
		return string(v)
	}
}

// rbReBlockLM is rbReBlock in a frame that reads $~ or $1: each match is the frame's last match while its block runs
// (decision 241).
func rbReBlockLM(lm ***MatchData, re *Regexp, blk func(String) any) func(s string, loc []int, names []string) string {
	return func(s string, loc []int, _ []string) string {
		m := rbMatchAt(re, s, loc)
		*lm = &m
		return string(rbToS(blk(String(s[loc[0]:loc[1]]))))
	}
}

// rbReSub replaces the first n matches of re in s (all when n < 0); rxRegexp's FindAll applies the Onigmo corrections.
func rbReSub(re *Regexp, s string, n int, repl func(s string, loc []int, names []string) string) string {
	return rbReplaceLocs(s, rbFindAll(re, s, n), re.re.SubexpNames(), repl)
}

// rbReSubBin is rbReSub on a binary String (decision 352).
func rbReSubBin(re *Regexp, s string, n int, repl func(s string, loc []int, names []string) string) string {
	return rbReplaceLocs(s, rbFindAllBin(re, s, n), re.re.SubexpNames(), repl)
}

// rbReplaceLocs is s with each match at locs replaced by repl's text.
func rbReplaceLocs(s string, locs [][]int, names []string, repl func(s string, loc []int, names []string) string) string {
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(s[last:loc[0]])
		b.WriteString(repl(s, loc, names))
		last = loc[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// rbRegexpEscape is Regexp.escape (MRI's rb_reg_quote).
func rbRegexpEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '[', ']', '{', '}', '(', ')', '|', '-', '*', '.', '\\', '?', '+', '^', '$', '#':
			b.WriteByte('\\')
			b.WriteRune(r)
		case ' ':
			b.WriteString(`\ `)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rbUniqNames is the named groups in order, each once (MRI's Regexp#names).
func rbUniqNames(names []string) *Array[String] {
	out := &Array[String]{}
	for _, n := range names {
		if n != "" && !slices.Contains(out.s, String(n)) {
			out.s = append(out.s, String(n))
		}
	}
	return out
}

// rbMatchNamed is MatchData#[] by group name: the last group of that name that matched, as Onigmo picks.
func rbMatchNamed(m *MatchData, name string) *String {
	found := false
	var out *String
	for i, n := range m.names {
		if n == name {
			found = true
			if m.groups[i] != nil {
				out = m.groups[i]
			}
		}
	}
	if !found {
		panic(NewIndexError(Ref(String("undefined group name reference: " + name))))
	}
	return out
}

// rbMatchOffset is MatchData#begin (side 0) or #end (side 1) in characters.
func rbMatchOffset(m *MatchData, n Integer, side int) *Integer {
	if n < 0 || int(n) >= len(m.groups) {
		panic(NewIndexError(Ref(String(fmt.Sprintf("index %d out of matches", n)))))
	}
	b := m.loc[2*int(n)+side]
	if b < 0 {
		return nil
	}
	if m.bin { // a binary subject's characters are bytes (decision 352)
		return Ref(Integer(b))
	}
	return Ref(Integer(utf8.RuneCountInString(m.subj[:b])))
}

// rbMatchCapture is a named group's local after `/(?<name>..)/ =~ s`: nil without a match.
func rbMatchCapture(m **MatchData, name string) *String {
	if m == nil {
		return nil
	}
	return rbMatchNamed(*m, name)
}

// rbMatchPos is that =~'s value: the match's character index, or nil.
func rbMatchPos(m **MatchData) *Integer {
	if m == nil {
		return nil
	}
	return rbMatchOffset(*m, 0, 0)
}

// rbMatchSet is `=~` in a frame that reads $~ (decision 171): it records m as the frame's last match and
// answers the match's character index, or nil.
func rbMatchSet(lm ***MatchData, m **MatchData) *Integer {
	*lm = m
	if m == nil {
		return nil
	}
	return Ref(Integer(utf8.RuneCountInString((*m).subj[:(*m).loc[0]])))
}

// rbMatchSetBin is rbMatchSet for a binary subject: =~ is a byte index (decision 352).
func rbMatchSetBin(lm ***MatchData, m **MatchData) *Integer {
	*lm = m
	if m == nil {
		return nil
	}
	(*m).bin = true
	return Ref(Integer((*m).loc[0]))
}

// rbLMGroup is $n (or $& for 0): group n of the last match, nil without one.
func rbLMGroup(m **MatchData, n int) *String {
	if m == nil || n >= len((*m).groups) {
		return nil
	}
	return (*m).groups[n]
}

// rbLMPre, rbLMPost and rbLMLast are $`, $' and $+ (English's $PREMATCH, $POSTMATCH, $LAST_PAREN_MATCH, decision 414):
// the last match's text before and after it, and its highest-numbered group that matched.
func rbLMPre(m **MatchData) *String {
	if m == nil {
		return nil
	}
	return Ref(String((*m).pre))
}

func rbLMPost(m **MatchData) *String {
	if m == nil {
		return nil
	}
	return Ref(String((*m).post))
}

func rbLMLast(m **MatchData) *String {
	if m == nil {
		return nil
	}
	for i := len((*m).groups) - 1; i >= 1; i-- {
		if (*m).groups[i] != nil {
			return (*m).groups[i]
		}
	}
	return nil
}

// rbMatchKeep is `match` in a frame that reads $~: the MatchData, also kept as the last match.
func rbMatchKeep(lm ***MatchData, m **MatchData) **MatchData {
	*lm = m
	return m
}

// rbGrepLM is `xs.grep(re) { ... }` / grep_v in a frame that reads $~: each test sets the frame's last match before
// the block runs, as MRI's grep_iter_i (a non-String, non-Symbol element is a failed test, nil match).
func rbGrepLM[E any, U comparable](lm ***MatchData, re *Regexp, xs []E, invert bool, blk func(E) U) *Array[U] {
	out := &Array[U]{}
	for _, x := range xs {
		var m **MatchData
		switch any(x).(type) {
		case String, Symbol:
			s, _ := rbSubject(any(x))
			if md := rbMatch(re, s); md != nil {
				m = &md
			}
		}
		*lm = m
		if (m != nil) != invert {
			out.s = append(out.s, blk(x))
		}
	}
	return out
}

// rbRindexLoc is r's match of s with the last start (String#rindex, rpartition). Onigmo tries each start from the end
// with the whole of s as context, so \A, ^, \b and lookbehind see the real neighbours; here the engine steps forward
// over every start (matches may overlap) and keeps the last. A subject that is not valid UTF-8 keeps rbFindLoc's
// trial of each rest of the String.
func rbRindexLoc(r *Regexp, s string) []int {
	if !rbValidUTF8(s) {
		for i := len(s); i >= 0; i-- {
			if loc := rbFindLoc(r, s[i:]); loc != nil && loc[0] == 0 {
				return []int{i, i + loc[1]}
			}
		}
		return nil
	}
	find := rbFindFrom(r, s)
	var last []int
	for from := 0; from <= len(s); {
		loc := find(from)
		if loc == nil {
			break
		}
		last = slices.Clone(loc)
		_, w := utf8.DecodeRuneInString(s[loc[0]:])
		from = loc[0] + max(w, 1)
	}
	return last
}

// rbFindFrom is r's search of a valid UTF-8 s for the first match starting at or after a byte offset, with the whole
// of s as context (\A, ^ and lookbehind see what precedes it): rbRindexLoc and Regexp#match(s, pos).
func rbFindFrom(r *Regexp, s string) func(from int) []int {
	rx := r.re
	rx.check()
	if rx.alt != nil {
		return func(from int) []int {
			out, err := rx.alt.findAll(s, from, 1)
			switch {
			case errors.Is(err, errRxTimeout):
				panic(RxTimeout(err.Error()))
			case err != nil:
				panic(RxUnsupported(err.Error()))
			case len(out) == 0:
				return nil
			}
			return out[0]
		}
	}
	prog := rx.prog
	if prog == nil {
		tree, _ := syntax.Parse(rx.Regexp.String(), syntax.Perl)
		prog, _ = syntax.Compile(tree.Simplify())
	}
	np := len(prog.Inst)
	b := &rxBacktrack{prog: prog, s: s, np: np, visited: make([]uint64, (np*(len(s)+1)+63)/64), caps: make([]int, 2*rx.NumSubexp()+2)}
	return b.search
}

// rbHasBackref reports a \1..\9 or \k<name> / \k'name' backreference in a Ruby regexp source.
func rbHasBackref(src string) bool {
	for i := 0; i+1 < len(src); i++ {
		if src[i] != '\\' {
			continue
		}
		c := src[i+1]
		if c >= '1' && c <= '9' || c == 'k' && i+2 < len(src) && (src[i+2] == '<' || src[i+2] == '\'') {
			return true
		}
		i++
	}
	return false
}
