//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

// The numeric prefix String#to_i / #to_f read, as MRI scans it: leading
// whitespace, a sign, digits with single `_` between them; to_i takes a
// `0d` prefix, to_f a fraction and exponent. The rest of the string is ignored.
var (
	rbIntPrefix   = regexp.MustCompile(`\A[ \t\n\v\f\r]*([+-]?)(?:0[dD])?(\d+(?:_\d+)*)`)
	rbFloatPrefix = regexp.MustCompile(`\A[ \t\n\v\f\r]*([+-]?(?:\d+(?:_\d+)*(?:\.(?:\d+(?:_\d+)*)?)?|\.\d+(?:_\d+)*)(?:[eE][+-]?\d+(?:_\d+)*)?)`)
)

// rbCaseMap is MRI's full case mapping, i 0 down, 1 title, 2 up: ASCII by hand, a binary String's ASCII letters only,
// else x/text's cases (decision 380).
func rbCaseMap(s string, i int) string {
	if out, ok := rbCaseMapASCII(s, i == 0); ok {
		return out
	}
	if !utf8.ValidString(s) { // a binary String: ASCII letters only, its other bytes kept (decision 324)
		lower := i == 0
		return rbASCIIMapBytes(s, func(c byte) byte {
			switch {
			case lower && c >= 'A' && c <= 'Z':
				return c + ('a' - 'A')
			case !lower && c >= 'a' && c <= 'z':
				return c - ('a' - 'A')
			}
			return c
		})
	}
	return rbCaseX([...]int{rbCaseLower, rbCaseTitle, rbCaseUpper}[i], s)
}

// The casers rbCaseX hands out: MRI's mappings in x/text terms.
const (
	rbCaseLower = iota
	rbCaseTitle
	rbCaseUpper
	rbCaseFold
	rbCaseLowerTR
	rbCaseTitleTR
	rbCaseUpperTR
)

// rbCaserPools hold x/text casers, which keep state while they run and so are one per call, by kind.
var rbCaserPools [rbCaseUpperTR + 1]sync.Pool

// rbCaseX is s through caser k. MRI does not apply the final-sigma rule (ΣΑΣ downcases to σασ), which x/text's Lower
// does by default; its Title, applied to one character at a time here, is that character's titlecase mapping alone.
func rbCaseX(k int, s string) string {
	switch k {
	case rbCaseLowerTR: // MRI's :turkic is the dotted/dotless i pairs and else the plain mapping, without Turkish's
		// context rule, which makes "I\u0307" one i where MRI keeps ı and the dot
		s, k = strings.Map(func(r rune) rune {
			switch r {
			case 'I':
				return 'ı'
			case 'İ':
				return 'i'
			}
			return r
		}, s), rbCaseLower
	case rbCaseTitle, rbCaseTitleTR: // only ever one character: a Georgian Mtavruli letter titlecases to its lower case in MRI
		if r, _ := utf8.DecodeRuneInString(s); r >= 0x1C90 && r <= 0x1CBF {
			k = rbCaseLower
		}
	}
	out := rbCaser(k, s)
	if k == rbCaseFold { // CaseFolding.txt folds Cherokee to its capitals, as MRI does; x/text folds it to the small letters
		out = strings.Map(func(r rune) rune {
			switch {
			case r >= 0xAB70 && r <= 0xABBF:
				return r - 0xAB70 + 0x13A0
			case r >= 0x13F8 && r <= 0x13FD:
				return r - 8
			}
			return r
		}, out)
	}
	return out
}

// rbCaser is s through x/text caser k, pooled: s itself where nothing changes (Span), else one buffer (Transform),
// not transform.String's per-call scratch.
func rbCaser(k int, s string) string {
	c := rbCaserGet(k)
	defer rbCaserPools[k].Put(c)
	return rbCaserRun(c, s)
}

func rbCaserGet(k int) *cases.Caser {
	if c, _ := rbCaserPools[k].Get().(*cases.Caser); c != nil {
		return c
	}
	tag := language.Und
	if k >= rbCaseLowerTR {
		tag = language.Turkish
	}
	var cc cases.Caser
	switch k {
	case rbCaseLower, rbCaseLowerTR:
		cc = cases.Lower(tag, cases.HandleFinalSigma(false))
	case rbCaseTitle, rbCaseTitleTR:
		cc = cases.Title(tag, cases.NoLower, cases.HandleFinalSigma(false))
	case rbCaseUpper, rbCaseUpperTR:
		cc = cases.Upper(tag)
	default:
		cc = cases.Fold()
	}
	return &cc
}

func rbCaserRun(c *cases.Caser, s string) string {
	if s == "" {
		return s
	}
	src := unsafe.Slice(unsafe.StringData(s), len(s)) //nolint:gosec // read only: x/text takes []byte
	c.Reset()
	if n, err := c.Span(src, true); err == nil && n == len(src) {
		return s
	}
	dst := make([]byte, len(s)+len(s)/4+8)
	for {
		c.Reset()
		n, _, err := c.Transform(dst, src, true)
		if errors.Is(err, transform.ErrShortDst) {
			dst = make([]byte, 2*len(dst))
			continue
		}
		if n == 0 {
			return ""
		}
		return unsafe.String(&dst[0], n) //nolint:gosec // dst is never written again
	}
}

// rbCaseMapASCII is rbCaseMap for an ASCII s, to lower case or else upper (title case is upper in ASCII); s itself when
// nothing changes. ok is false for any other s.
func rbCaseMapASCII(s string, lower bool) (string, bool) {
	var b strings.Builder // one allocation, and none when nothing changes: []byte(s) then string(b) was two
	for j := range len(s) {
		c := s[j]
		if c >= utf8.RuneSelf {
			return "", false
		}
		d := c
		if lower && c >= 'A' && c <= 'Z' {
			d = c + 'a' - 'A'
		} else if !lower && c >= 'a' && c <= 'z' {
			d = c - ('a' - 'A')
		}
		if d != c && b.Cap() == 0 {
			b.Grow(len(s))
			b.WriteString(s[:j])
		}
		if b.Cap() != 0 {
			b.WriteByte(d)
		}
	}
	if b.Cap() == 0 {
		return s, true
	}
	return b.String(), true
}

// rbTitleSwap is swapcase of a titlecase letter (Unicode Lt), MRI's: the digraph's first part lowered, its second
// uppered (ǅ is dŽ). Go's unicode package has no such mapping; generated from MRI 4.0.7.
var rbTitleSwap = map[rune]string{
	0x1C5: "dŽ", 0x1C8: "lJ", 0x1CB: "nJ", 0x1F2: "dZ",
	0x1F88: "ἀΙ", 0x1F89: "ἁΙ", 0x1F8A: "ἂΙ", 0x1F8B: "ἃΙ",
	0x1F8C: "ἄΙ", 0x1F8D: "ἅΙ", 0x1F8E: "ἆΙ", 0x1F8F: "ἇΙ",
	0x1F98: "ἠΙ", 0x1F99: "ἡΙ", 0x1F9A: "ἢΙ", 0x1F9B: "ἣΙ",
	0x1F9C: "ἤΙ", 0x1F9D: "ἥΙ", 0x1F9E: "ἦΙ", 0x1F9F: "ἧΙ",
	0x1FA8: "ὠΙ", 0x1FA9: "ὡΙ", 0x1FAA: "ὢΙ", 0x1FAB: "ὣΙ",
	0x1FAC: "ὤΙ", 0x1FAD: "ὥΙ", 0x1FAE: "ὦΙ", 0x1FAF: "ὧΙ",
	0x1FBC: "αΙ", 0x1FCC: "ηΙ", 0x1FFC: "ωΙ",
}

// rbSwapcase is String#swapcase: by full case mapping, a character with a lower case mapping lowered and else uppered
// (ß is SS, İ is i̇, Ⅰ and Ⓐ too, though not of category Lu), titlecase digraphs split as MRI does, ASCII only for a
// String that is not UTF-8 (a binary String's other bytes are kept, decision 324). turkic is :turkic's dotted i rules.
func rbSwapcase(s string, turkic bool) string {
	if !utf8.ValidString(s) || rbASCII(s) && !turkic {
		return rbASCIIMapBytes(s, func(c byte) byte {
			switch {
			case c >= 'a' && c <= 'z':
				return c - ('a' - 'A')
			case c >= 'A' && c <= 'Z':
				return c + ('a' - 'A')
			}
			return c
		})
	}
	lk, uk := rbCaseLower, rbCaseUpper
	if turkic {
		lk, uk = rbCaseLowerTR, rbCaseUpperTR
	}
	lower, upper := rbCaserGet(lk), rbCaserGet(uk)
	defer rbCaserPools[lk].Put(lower)
	defer rbCaserPools[uk].Put(upper)
	var b strings.Builder
	b.Grow(len(s))
	// runs of characters that go one way, each through its caser once: a character lowering changes is lowered (so Ⅰ
	// and Ⓐ, not of category Lu, count), any other uppered; a titlecase digraph splits as MRI does
	start, down := 0, false
	flush := func(end int) {
		if end > start {
			run := s[start:end]
			switch {
			case turkic && down:
				run = rbCaseX(rbCaseLowerTR, run)
			case down:
				run = rbCaserRun(lower, run)
			default:
				run = rbCaserRun(upper, run)
			}
			b.WriteString(run)
		}
		start = end
	}
	for i, r := range s {
		if m, ok := rbTitleSwap[r]; ok {
			flush(i)
			b.WriteString(m)
			start = i + utf8.RuneLen(r)
			continue
		}
		lower.Reset()
		c := s[i : i+utf8.RuneLen(r)]
		n, _ := lower.Span(unsafe.Slice(unsafe.StringData(c), len(c)), true) //nolint:gosec // read only
		if d := n < len(c); d != down {
			flush(i)
			down = d
		}
	}
	flush(len(s))
	return b.String()
}

// rbASCIIMapBytes maps each byte of s through f: the case maps of a String that is not UTF-8, which MRI applies to
// ASCII letters only, keeping every other byte (U+FFFD would replace them as runes).
func rbASCIIMapBytes(s string, f func(byte) byte) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		b.WriteByte(f(s[i]))
	}
	return b.String()
}

// rbASCII reports s is all ASCII, where a character is a byte: eight bytes a step (Go combines the loads), as
// String#length and #[] ask it of the String on every call (decision 399).
func rbASCII(s string) bool {
	for len(s) >= 8 {
		w := uint64(s[0]) | uint64(s[1])<<8 | uint64(s[2])<<16 | uint64(s[3])<<24 | uint64(s[4])<<32 | uint64(s[5])<<40 | uint64(s[6])<<48 | uint64(s[7])<<56
		if w&0x8080808080808080 != 0 {
			return false
		}
		s = s[8:]
	}
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// rbCapitalizeASCII is capitalize of an ASCII s, a new String in one allocation.
func rbCapitalizeASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	c := s[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	b.WriteByte(c)
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// rbReverseBytes is s backwards byte by byte (ASCII, or a String that is not UTF-8), in one allocation.
func rbReverseBytes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := len(s) - 1; i >= 0; i-- {
		b.WriteByte(s[i])
	}
	return b.String()
}

// rbSub replaces the first n (all if n < 0) occurrences of pat, expanding
// MRI's backslash sequences in rep: \0 \& match, \` \' pre/post-match,
// \\ a backslash; \1-\9 and \+ are empty (a String pattern has no groups).
// ponytail: \k<name> stays literal; MRI raises IndexError.
func rbSub(s, pat, rep string, n int) string {
	if !strings.Contains(rep, "\\") {
		return strings.Replace(s, pat, rep, n)
	}
	var b strings.Builder
	done, pos := 0, 0
	for ; n != 0; n-- {
		i := strings.Index(s[pos:], pat)
		if i < 0 {
			break
		}
		m, e := pos+i, pos+i+len(pat)
		b.WriteString(s[done:m])
		for j := 0; j < len(rep); j++ {
			if rep[j] != '\\' || j+1 == len(rep) {
				b.WriteByte(rep[j])
				continue
			}
			j++
			switch rep[j] {
			case '0', '&':
				b.WriteString(pat)
			case '`':
				b.WriteString(s[:m])
			case '\'':
				b.WriteString(s[e:])
			case '\\':
				b.WriteByte('\\')
			case '1', '2', '3', '4', '5', '6', '7', '8', '9', '+':
			default:
				b.WriteString(rep[j-1 : j+1])
			}
		}
		done, pos = e, e
		if pat == "" { // step past a rune so an empty pattern matches between each
			if pos == len(s) {
				break
			}
			_, w := utf8.DecodeRuneInString(s[pos:])
			pos += w
		}
	}
	b.WriteString(s[done:])
	return b.String()
}

// rbPad is n runes of pad repeated, as String#ljust/rjust/center fill.
func rbPad(pad string, n int) string {
	if pad == "" {
		panic(NewArgumentError(Ref(String("zero width padding"))))
	}
	rbCheckAlloc(n, utf8.UTFMax)
	r := []rune(pad)
	return strings.Repeat(pad, n/len(r)) + string(r[:n%len(r)])
}

// rbTrExpand is a tr-style character list expanded, as MRI's trnext reads it: `\x` is x for a range's start, `a-z` a range whose
// end is read raw (so `a-\z` ends at the backslash), a `-` first or last itself; a reversed range is an ArgumentError.
// It stops once it has limit characters (limit < 0: all), since MRI reads a to-list only as far as the from-list needs.
func rbTrExpand(cs []rune, limit int) []rune {
	var out []rune
	for i := 0; i < len(cs) && (limit < 0 || len(out) < limit); {
		start := cs[i]
		if start == '\\' && i+1 < len(cs) {
			i++
			start = cs[i]
		}
		i++
		if i < len(cs)-1 && cs[i] == '-' {
			end := cs[i+1]
			if start > end {
				panic(NewArgumentError(Ref(String(fmt.Sprintf("invalid range \"%c-%c\" in string transliteration", start, end)))))
			}
			for r := start; r <= end; r++ {
				out = append(out, r)
			}
			i += 2
			continue
		}
		out = append(out, start)
	}
	return out
}

// rbCharSet is a tr/count/squeeze character set: rbTrExpand's list, and a leading ^ to negate.
func rbCharSet(spec string) func(rune) bool {
	rs := []rune(spec)
	negate := len(rs) > 1 && rs[0] == '^'
	if negate {
		rs = rs[1:]
	}
	set := map[rune]bool{}
	for _, r := range rbTrExpand(rs, -1) {
		set[r] = true
	}
	return func(r rune) bool { return set[r] != negate }
}

// rbSqueeze collapses runs of one repeated character that in accepts.
func rbSqueeze(s string, in func(rune) bool) string {
	var b strings.Builder
	prev := rune(-1)
	for _, r := range s {
		if r == prev && in(r) {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// rbStrToIBase is String#to_i(base) (base -8 is #oct: octal unless a
// prefix names another base). Leading whitespace, a sign and single
// underscores between digits are allowed; parsing stops at the first
// invalid digit.
func rbStrToIBase(s string, base int) int {
	if base == 1 || base > 36 || base < -8 {
		panic(NewArgumentError(Ref(String("invalid radix " + strconv.Itoa(base)))))
	}
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg, s = s[0] == '-', s[1:]
	}
	// a 0b/0o/0x/0d prefix names its radix (rbRadixPrefixes), honored when base is auto or the same
	if len(s) >= 2 {
		if pb, ok := rbRadixPrefixes[s[:2]]; ok && (base <= 0 || base == pb) {
			base, s = pb, s[2:]
		}
	}
	switch {
	case base == 0 && strings.HasPrefix(s, "0"):
		base = 8
	case base == 0:
		base = 10
	case base < 0:
		base = -base
	}
	n := 0
	for i := range len(s) {
		c := s[i]
		if c == '_' && i > 0 && i+1 < len(s) && s[i-1] != '_' {
			continue
		}
		d := 99
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		}
		if d >= base {
			break
		}
		if n > (math.MaxInt-d)/base { // MRI returns a Bignum (decision 35)
			panic(NewRangeError(Ref(String(s + " overflows Integer (64-bit; no Bignum)"))))
		}
		n = n*base + d
	}
	if neg {
		return -n
	}
	return n
}

// rbSplitLimit is String#split(sep, limit); sep nil or " " is awk mode.
func rbSplitLimit(s string, sep *String, limit int) []string {
	awk := sep == nil || *sep == " "
	var parts []string
	switch {
	case limit == 1:
		if s == "" {
			return nil
		}
		return []string{s}
	case awk:
		rest := strings.TrimLeft(s, " \t\n\v\f\r")
		full := false
		for rest != "" {
			if limit > 0 && len(parts) == limit-1 {
				parts, full = append(parts, rest), true
				break
			}
			i := strings.IndexAny(rest, " \t\n\v\f\r")
			if i < 0 {
				parts = append(parts, rest)
				break
			}
			parts = append(parts, rest[:i])
			rest = strings.TrimLeft(rest[i:], " \t\n\v\f\r")
		}
		if limit != 0 && !full && len(s) > 0 && strings.ContainsAny(s[len(s)-1:], " \t\n\v\f\r") {
			parts = append(parts, "")
		}
		return parts
	case limit > 0:
		parts = strings.SplitN(s, string(*sep), limit)
	default:
		parts = strings.Split(s, string(*sep))
	}
	if *sep == "" && limit != 0 && s != "" && (limit < 0 || limit > len(parts)) { // MRI keeps the empty field after the last character
		parts = append(parts, "")
	}
	if limit == 0 {
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	if s == "" {
		return nil
	}
	return parts
}

// rbStrDump is String#dump: inspect's escapes, with every non-ASCII character as \u and invalid bytes as \x.
func rbStrDump(s string) string { return rbDump(s, false) }

// rbDump is String#dump; bytes reads s one byte per character (ASCII-8BIT, as MRI's encoding errors quote bytes),
// else as UTF-8 with invalid bytes as \x.
func rbDump(s string, bytes bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, n := rune(s[i]), 1
		if !bytes {
			r, n = utf8.DecodeRuneInString(s[i:])
		}
		switch {
		case bytes && r >= 0x80, r == utf8.RuneError && n == 1:
			fmt.Fprintf(&b, "\\x%02X", s[i])
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '#' && i+1 < len(s) && strings.IndexByte("{$@", s[i+1]) >= 0:
			b.WriteString("\\#")
		case r >= 0x20 && r < 0x7f:
			b.WriteRune(r)
		case r < 0x80:
			if e, ok := rbDumpEscapes[r]; ok {
				b.WriteString("\\" + e)
			} else {
				fmt.Fprintf(&b, "\\x%02X", r)
			}
		case r > 0xffff:
			fmt.Fprintf(&b, "\\u{%X}", r)
		default:
			fmt.Fprintf(&b, "\\u%04X", r)
		}
		i += n
	}
	b.WriteByte('"')
	return b.String()
}

var rbDumpEscapes = map[rune]string{'\n': "n", '\t': "t", '\r': "r", '\f': "f", '\v': "v", '\b': "b", '\a': "a", 0x1b: "e"}

// rbStrUndump is String#undump, dump's inverse: a double-quoted ASCII
// string whose escapes (\n \t \r \f \v \b \a \e \" \\ \# \xHH \uHHHH
// \u{H...}) are decoded; another escaped character keeps its backslash.
func rbStrUndump(s string) String {
	fail := func(msg string) { panic(NewRuntimeError(Ref(String(msg)))) }
	for i := range len(s) {
		if s[i] >= 0x80 {
			fail("non-ASCII character detected")
		}
	}
	if !strings.HasPrefix(s, `"`) {
		fail(`invalid dumped string; not wrapped with '"' nor '"...".force_encoding("...")' form`)
	}
	if len(s) < 2 || !strings.HasSuffix(s, `"`) || strings.HasSuffix(s, `\"`) && !strings.HasSuffix(s, `\\"`) {
		fail("unterminated dumped string")
	}
	hex := func(t string) (int, int) { // value and digits read
		n, v := 0, 0
		for ; n < len(t); n++ {
			d := strings.IndexByte("0123456789abcdef", t[n]|0x20)
			if d < 0 {
				break
			}
			v = v*16 + d
		}
		return v, n
	}
	in, out := s[1:len(s)-1], []byte{}
	for i := 0; i < len(in); i++ {
		if in[i] != '\\' || i+1 == len(in) {
			out = append(out, in[i])
			continue
		}
		i++
		c := in[i]
		switch {
		case strings.IndexByte(`"\#`, c) >= 0:
			out = append(out, c)
		case strings.IndexByte("ntrfvbae", c) >= 0:
			out = append(out, "\n\t\r\f\v\b\a\x1b"[strings.IndexByte("ntrfvbae", c)])
		case c == 'x':
			v, n := hex(in[i+1 : min(i+3, len(in))])
			if n == 0 {
				fail("invalid hex escape")
			}
			out = append(out, byte(v))
			i += n
		case c == 'u' && i+1 < len(in) && in[i+1] == '{':
			end := strings.IndexByte(in[i:], '}')
			if end < 0 {
				fail("unterminated Unicode escape")
			}
			for _, h := range strings.Fields(in[i+2 : i+end]) {
				v, n := hex(h)
				if n != len(h) {
					fail("invalid Unicode escape")
				}
				out = utf8.AppendRune(out, rune(v))
			}
			i += end
		case c == 'u':
			v, n := hex(in[i+1 : min(i+5, len(in))])
			if n != 4 {
				fail("invalid Unicode escape")
			}
			out = utf8.AppendRune(out, rune(v))
			i += 4
		default:
			out = append(out, '\\', c)
		}
	}
	return String(out)
}

// rbStrChanged is a bang method's result once its local holds after (decision 172): after, or nil when unchanged.
func rbStrChanged(before, after String) *String {
	if before == after {
		return nil
	}
	return &after
}

// rbMustStr is a String? where `<<` needs a String: nil raises MRI's TypeError.
func rbMustStr(p *String) String {
	if p == nil {
		panic(NewTypeError(Ref(String("no implicit conversion of nil into String"))))
	}
	return *p
}

// rbStrReplace is a gem's `s.replace(with)` on a local (decision 172): the receiver was only checked to be a String.
func rbStrReplace(_ String, with String) String { return with }

// rbStrSliceBang is `s.slice!(start, len)` (decision 172): the removed characters (nil out of range, as s[start, len])
// and what is left.
func rbStrSliceBang(s String, start, n Integer) (*String, String) {
	rs := []rune(string(s))
	i := int(start)
	if i < 0 {
		i += len(rs)
	}
	if i < 0 || i > len(rs) || n < 0 {
		return nil, s
	}
	j := min(i+int(n), len(rs))
	got := String(rs[i:j])
	return &got, String(string(rs[:i]) + string(rs[j:]))
}

// rbStrRecv is a String? local a bang method rebinds (decision 172): nil raises NoMethodError, as MRI's call on nil.
func rbStrRecv(p *String, name string) String {
	if p == nil {
		panic(rbNoMethod(name, nil, false))
	}
	return *p
}

// rbStrRecvAny is rbStrRecv for a local also holding other classes: only a String has the method.
func rbStrRecvAny(v any, name string) String {
	s, ok := v.(String)
	if !ok {
		panic(rbNoMethod(name, v, false))
	}
	return s
}

// rbStrLen is String#length: characters of UTF-8, bytes of anything else (ASCII-8BIT by decision 136).
// rbEachChar is s's characters: runes, or each byte of a String that is not valid UTF-8 (ASCII-8BIT, decision 324).
func rbEachChar(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		if !utf8.ValidString(s) {
			for i := range len(s) {
				if !yield(s[i : i+1]) {
					return
				}
			}
			return
		}
		for i, r := range s {
			if !yield(s[i : i+utf8.RuneLen(r)]) {
				return
			}
		}
	}
}

func rbStrLen(s string) int {
	if rbASCII(s) { // one pass, not a validity pass and then a rune count (decision 399)
		return len(s)
	}
	if utf8.ValidString(s) {
		return utf8.RuneCountInString(s)
	}
	return len(s)
}

// rbDigits parses s when it is only ASCII digits, too few to overflow: to_i's common case, without rbIntPrefix's regexp.
func rbDigits(s string) (int, bool) {
	if s == "" || len(s) > 18 {
		return 0, false
	}
	n := 0
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// rbStrBufs are the buffers rbStrCat may extend in place, in slots by their address: a loop's `s << x` or `s += x` is
// `s = s + x` on immutable Strings, which copied all of s each time. Taking an entry is an atomic swap, so goroutines
// never share one; a nested builder (a recursive converter's own result) sits in another slot (decision 332).
var rbStrBufs [16]atomic.Pointer[[]byte]

func rbStrSlot(p *byte) *atomic.Pointer[[]byte] {
	return &rbStrBufs[(uintptr(unsafe.Pointer(p))>>4)%uintptr(len(rbStrBufs))] //nolint:gosec // the address only picks a slot
}

// rbStrPlus is String#+ past its inlined short case: never one of its operands (MRI's result is a new String).
func rbStrPlus(a, b String) String {
	return rbNewStr(a, rbNewStr(b, String(rbStrCat(string(a), string(b)))))
}

// rbConcat is a chain of String#+ in one allocation (decision 377); a long first operand goes through rbStrCat, which may extend it in place.
func rbConcat(first String, rest ...String) String {
	n := 0
	for _, p := range rest {
		n += len(p)
	}
	var b strings.Builder
	if len(first) >= 64 {
		b.Grow(n)
		for _, p := range rest {
			b.WriteString(string(p))
		}
		return rbStrPlus(first, String(b.String()))
	}
	if n+len(first) == 0 {
		return rbStrClone("")
	}
	b.Grow(n + len(first))
	b.WriteString(string(first))
	for _, p := range rest {
		b.WriteString(string(p))
	}
	return String(b.String())
}

// rbStrCat is a + b. When a is the whole of a buffer rbStrCat made, b is written past a's end, where no String can see
// it (every String over the buffer is at most as long as a): a is not copied. Otherwise it copies into a new buffer,
// exactly sized the first time and doubled when that result is appended to, so a build loop is amortized linear.
func rbStrCat(a, b string) string {
	if len(b) == 0 || len(a) < 64 { // short: Go's concatenation; the buffer's bookkeeping would cost more than the copy
		return a + b
	}
	pa := unsafe.StringData(a) //nolint:gosec // pointer compare only
	slot := rbStrSlot(pa)
	var buf []byte
	if p := slot.Swap(nil); p != nil {
		if old := *p; len(old) == len(a) && unsafe.SliceData(old) == pa { //nolint:gosec // pointer compare only
			buf = old
			if cap(buf)-len(buf) < len(b) {
				buf = append(make([]byte, 0, 2*(len(a)+len(b))), a...)
			}
		} else {
			slot.CompareAndSwap(nil, p) // another String's buffer: put it back
		}
	}
	if buf == nil {
		buf = make([]byte, 0, len(a)+len(b))
		buf = append(buf, a...)
	}
	buf = append(buf, b...)
	out := unsafe.String(unsafe.SliceData(buf), len(buf)) //nolint:gosec // buf's bytes up to here are never written again
	rbStrSlot(unsafe.SliceData(buf)).Store(&buf)          //nolint:gosec // the address only picks a slot
	return out
}

// rbStrOut is a String out parameter (decision 405): the caller's String and the buffer its appends share, so a callee appends in place.
type rbStrOut struct {
	s *String
	b *[]byte
}

// rbBytesArg is one String#append_as_bytes argument as bytes: an Integer's low byte, a String's bytes (no encoding check),
// anything else MRI's TypeError.
func rbBytesArg(v any) String {
	switch x := rbUnbox(v).(type) {
	case Integer:
		return String([]byte{byte(x)})
	case String:
		return x
	}
	panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(v) + " (expected String or Integer)"))))
}

// rbStrAppend is an unaliased local's `s << x` (decision 382): buf is the local's own, so no other builder evicts it as in rbStrCat's slots.
// A nil buf is a gem caller's out parameter (decision 405): the append takes a buffer of its own.
func rbStrAppend(buf *[]byte, s, x String) String {
	if buf == nil {
		buf = new([]byte)
	}
	b := *buf
	if len(s) == 0 || len(b) != len(s) || unsafe.SliceData(b) != unsafe.StringData(string(s)) { //nolint:gosec // pointer compare only
		if len(x) == 0 {
			return s
		}
		b = make([]byte, len(s), max(2*(len(s)+len(x)), 32))
		copy(b, s)
	}
	b = append(b, x...)
	*buf = b
	return String(unsafe.String(unsafe.SliceData(b), len(b))) //nolint:gosec // buf's bytes up to here are never written again
}

// rbStrSetbyte is a proven `s.setbyte(i, v)` (decision 395): it writes inside bytes other Strings over buf could cover, so the proof that none exist is what keeps them unchanged.
func rbStrSetbyte(buf *[]byte, s String, i, v Integer) String {
	j := i
	if j < 0 {
		j += Integer(len(s))
	}
	if j < 0 || j >= Integer(len(s)) {
		panic(NewIndexError(Ref(String(fmt.Sprintf("index %d out of string", i)))))
	}
	b := *buf
	if len(b) != len(s) || unsafe.SliceData(b) != unsafe.StringData(string(s)) { //nolint:gosec // pointer compare only
		b = make([]byte, len(s))
		copy(b, s)
		*buf = b
		s = String(unsafe.String(unsafe.SliceData(b), len(b))) //nolint:gosec // the proof: s is the only String over buf, and every later read of it follows this store
	}
	b[j] = byte(v)
	return s
}

// rbRuneOffset is the byte offset of rune k in s, or len(s) when s has exactly k runes; past that, len(s) and false.
func rbRuneOffset(s string, k int) (int, bool) {
	i := 0
	for ; k > 0 && i < len(s); k-- {
		if s[i] < utf8.RuneSelf {
			i++
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
	return i, k == 0
}

var rbRadixPrefixes = map[string]int{"0b": 2, "0B": 2, "0o": 8, "0O": 8, "0x": 16, "0X": 16, "0d": 10, "0D": 10}

// The String#[] family's work, out of line: each returns (value, found) so that the public method, a few lines
// around one call, fits Go's inlining budget and its String? or Integer? stays on the caller's stack (decision 349;
// Hash#[] does the same, decision 342). Characters are walked, never a []rune of the whole String.

// rbStrAt is String#[](i): the character at i (negative from the end).
//
//go:noinline
func rbStrAt(s String, i Integer) (String, bool) {
	if i >= 0 && int(i) < len(s) && rbASCII(string(s[:i+1])) { // an ASCII prefix: character i is byte i, UTF-8 or binary (decision 399)
		return rbNewStr(s, s[i:i+1]), true
	}
	if !rbValidUTF8(string(s)) { // ASCII-8BIT by decision 136: a byte is a character
		return rbBinAt(s, i)
	}
	if i < 0 {
		if i += Integer(utf8.RuneCountInString(string(s))); i < 0 {
			return "", false
		}
	}
	b, ok := rbRuneOffset(string(s), int(i))
	if !ok || b >= len(s) {
		return "", false
	}
	_, n := utf8.DecodeRuneInString(string(s[b:]))
	return rbNewStr(s, s[b:b+n]), true
}

// rbStrLenA is a String's length, computed once before a loop that cannot change it, and the bound below which its
// character i is byte i: its length when it is all ASCII and longer than one byte (a one-byte s[0] must not be s itself), else 0 (decision 399).
func rbStrLenA(s String) (Integer, uint64) {
	if rbASCII(string(s)) {
		if len(s) > 1 {
			return Integer(len(s)), uint64(len(s))
		}
		return Integer(len(s)), 0
	}
	return Integer(rbStrLen(string(s))), 0
}

// rbStrCacheEnt is a String ivar's rbStrLenA, for the String whose bytes start at p and run n (decision 454).
type rbStrCacheEnt struct {
	p   *byte
	n   int
	ln  Integer
	lim uint64
}

// rbStrCacheLenA is rbStrLenA of an indexed String ivar, kept per object in its cache field c: a String of 32 bytes
// or more is scanned once per value it holds (any write gives a new data pointer or length), and threads reading
// one object swap whole entries, so no lock and no torn read (decision 454).
func rbStrCacheLenA(c *unsafe.Pointer, s String) (Integer, uint64) {
	if len(s) < 32 {
		return rbStrLenA(s)
	}
	p := unsafe.StringData(string(s))                                                        //nolint:gosec // the key: the String's data pointer, compared only
	if e := (*rbStrCacheEnt)(atomic.LoadPointer(c)); e != nil && e.p == p && e.n == len(s) { //nolint:gosec // c holds only *rbStrCacheEnt
		return e.ln, e.lim
	}
	ln, lim := rbStrLenA(s)
	atomic.StorePointer(c, unsafe.Pointer(&rbStrCacheEnt{p: p, n: len(s), ln: ln, lim: lim})) //nolint:gosec // a whole new entry, swapped in
	return ln, lim
}

// rbStrIdxA is String#[](i) of a hoisted String, written to the call site's own dst, so the String? needs no allocation
// whether or not this inlines; below lim, byte i with no prefix scan.
func rbStrIdxA(s String, i Integer, lim uint64, dst *String) *String {
	if uint64(i) < lim {
		*dst = s[i : i+1]
		return dst
	}
	return rbStrIdxSlow(s, i, dst)
}

//go:noinline
func rbStrIdxSlow(s String, i Integer, dst *String) *String {
	v, ok := rbStrAt(s, i)
	if !ok {
		return nil
	}
	*dst = v
	return dst
}

// rbStrSubA is String#[](start, count) of a hoisted String, as rbStrIdxA.
func rbStrSubA(s String, start, count Integer, lim uint64, dst *String) *String {
	if lim != 0 && start >= 0 && count >= 0 && int(start) <= len(s) {
		e := len(s)
		if int(count) < e-int(start) {
			e = int(start + count)
		}
		*dst = rbNewStr(s, s[start:e])
		return dst
	}
	v, ok := rbStrSub(s, start, count)
	if !ok {
		return nil
	}
	*dst = v
	return dst
}

// rbStrSub is String#[](start, count).
//
//go:noinline
func rbStrSub(s String, start, count Integer) (String, bool) {
	if start >= 0 && count >= 0 && int(start) <= len(s) { // an ASCII prefix through the end: characters are bytes (decision 399)
		e := len(s)
		if int(count) < e-int(start) {
			e = int(start + count)
		}
		if rbASCII(string(s[:e])) {
			return rbNewStr(s, s[start:e]), true
		}
	}
	if !rbValidUTF8(string(s)) { // ASCII-8BIT by decision 136: bytes, not runes (patchelf's section data)
		return rbBinSub(s, start, count)
	}
	if start < 0 || count < 0 {
		n := Integer(utf8.RuneCountInString(string(s)))
		if start < 0 {
			start += n
		}
		if start < 0 || start > n || count < 0 {
			return "", false
		}
	}
	b, ok := rbRuneOffset(string(s), int(start))
	if !ok {
		return "", false
	}
	e, _ := rbRuneOffset(string(s[b:]), int(count))
	return rbNewStr(s, s[b:b+e]), true
}

// rbStrRange is String#[](range): Range#__slice's start and count over the String's length, then rbStrSub.
//
//go:noinline
func rbStrRange(s String, r *Range[Integer]) (String, bool) {
	if !rbValidUTF8(string(s)) {
		return rbBinRange(s, r)
	}
	b, count, ok := rbStrSpan(r, Integer(utf8.RuneCountInString(string(s))))
	if !ok {
		return "", false
	}
	return rbStrSub(s, b, count)
}

// rbBinAt is String#[](i) on a binary String: a byte is a character (decisions 136, 352).
func rbBinAt(s String, i Integer) (String, bool) {
	if i < 0 {
		i += Integer(len(s))
	}
	if i < 0 || int(i) >= len(s) {
		return "", false
	}
	return s[i : i+1], true
}

// rbBinSub is String#[](start, count) on a binary String.
func rbBinSub(s String, start, count Integer) (String, bool) {
	n := Integer(len(s))
	if start < 0 {
		start += n
	}
	if start < 0 || start > n || count < 0 {
		return "", false
	}
	return rbNewStr(s, s[start:min(start+count, n)]), true
}

// rbBinRange is String#[](range) on a binary String.
func rbBinRange(s String, r *Range[Integer]) (String, bool) {
	b, count, ok := rbStrSpan(r, Integer(len(s)))
	if !ok {
		return "", false
	}
	return rbBinSub(s, b, count)
}

// rbStrSpan is r over n characters as a start and count, as String#[](range) reads it.
func rbStrSpan(r *Range[Integer], n Integer) (Integer, Integer, bool) {
	b := r.b
	if r.beginless {
		b = 0
	}
	e := n
	if !r.endless {
		if e = r.e; e < 0 {
			e += n
		}
		if !r.excl {
			e++
		}
	}
	if b < 0 {
		b += n
	}
	if b < 0 || b > n {
		return 0, 0, false
	}
	return b, max(min(e, n)-b, 0), true
}

// rbStrIndex is String#index(t): the character offset of t's first occurrence.
//
//go:noinline
func rbStrIndex(s, t String) (Integer, bool) {
	i := strings.Index(string(s), string(t))
	if i < 0 {
		return 0, false
	}
	return Integer(utf8.RuneCountInString(string(s)[:i])), true
}

// rbStrRindex is String#rindex(t): the character offset of t's last occurrence.
//
//go:noinline
func rbStrRindex(s, t String) (Integer, bool) {
	i := strings.LastIndex(string(s), string(t))
	if i < 0 {
		return 0, false
	}
	return Integer(utf8.RuneCountInString(string(s)[:i])), true
}

// rbTr is String#tr; bin reads s byte by byte, each byte the character of its value (a String that is not valid UTF-8,
// decision 324, or one the compiler knows is binary, decision 352), and so its lists when they are not valid UTF-8 or
// binLists (a binary String's twin, decision 352). sq is tr_s: a run of translated characters giving one character
// keeps one.
func rbTr(s String, from, to string, bin, binLists, sq bool) String {
	if len(s) == 0 { // MRI returns before reading either list
		return s
	}
	runes := func(x string) []rune { // a binary list's bytes are characters too
		if !bin || !binLists && utf8.ValidString(x) {
			return []rune(x)
		}
		out := make([]rune, len(x))
		for i := range len(x) {
			out[i] = rune(x[i])
		}
		return out
	}
	f := runes(from)
	negate := len(f) > 1 && f[0] == '^'
	if negate {
		f = f[1:]
	}
	f = rbTrExpand(f, -1)
	var t []rune
	if len(f) > 0 { // MRI reads the to-list only as the from-list's characters need it
		t = rbTrExpand(runes(to), len(f))
	}
	if negate { // only the to-list's last character counts, so it is read whole
		t = rbTrExpand(runes(to), -1)
	}
	// An empty to-list deletes; otherwise it pads with its last rune.
	last := rune(-1)
	if len(t) > 0 {
		last = t[len(t)-1]
	}
	var ascii [128]rune // ASCII lookups skip the map: -1 is absent (a value is never negative)
	for i := range ascii {
		ascii[i] = -1
	}
	var m map[rune]rune
	for i, c := range f {
		v := last
		if !negate && i < len(t) {
			v = t[i]
		}
		if c < 128 {
			ascii[c] = v
			if v < 0 {
				ascii[c] = -2 // deleted, still present
			}
			continue
		}
		if m == nil {
			m = map[rune]rune{}
		}
		m[c] = v
	}
	hit := false // conv's last rune was translated
	conv := func(r rune) rune {
		var v rune
		var ok bool
		if r < 128 {
			v, ok = ascii[r], ascii[r] != -1
			if v == -2 {
				v = -1
			}
		} else {
			v, ok = m[r]
		}
		hit = ok != negate
		if negate {
			if ok {
				return r
			}
			return last
		}
		if ok {
			return v
		}
		return r
	}
	if sq {
		var b strings.Builder
		prev := rune(-1)
		for _, r := range string(s) {
			v := conv(r)
			switch {
			case !hit:
				b.WriteRune(v)
				prev = -1
			case v >= 0 && v != prev:
				b.WriteRune(v)
				prev = v
			}
		}
		return rbNewStr(s, String(b.String()))
	}
	if bin { // ASCII-8BIT: each byte is the character of its value (decision 324)
		var out []byte
		for i := range len(s) {
			switch v := conv(rune(s[i])); {
			case v < 0:
			case v < 256:
				out = append(out, byte(v))
			default:
				out = utf8.AppendRune(out, v)
			}
		}
		return rbNewStr(s, String(out))
	}
	return rbNewStr(s, String(strings.Map(conv, string(s))))
}

// rbCaseOpt reads a case-mapping option Symbol: 0 none (:lithuanian too), 1 :ascii, 2 :turkic, 3 :fold (downcase only).
func rbCaseOpt(opt Symbol, down bool) int {
	switch opt {
	case "lithuanian":
		return 0
	case "ascii":
		return 1
	case "turkic":
		return 2
	case "fold":
		if down {
			return 3
		}
		panic(NewArgumentError(Ref(String("option :fold only allowed for downcasing"))))
	}
	panic(NewArgumentError(Ref(String("invalid option"))))
}

// rbASCIICase maps s's ASCII letters only: mode 0 down, 2 up, 3 swap.
func rbASCIICase(s string, mode int) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z' && mode != 0:
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z' && mode != 2:
			return r + ('a' - 'A')
		}
		return r
	}, s)
}
