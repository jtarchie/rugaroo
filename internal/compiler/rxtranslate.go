package compiler

// The Ruby-to-RE2 regexp translator, and the matcher for what no
// translation can express. It is self-contained (stdlib only) because this
// file is also emitted verbatim into every program (rxTranslateGo): an
// interpolated regexp's source only exists at run time.

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// rxPrepare is the one way from a Ruby regexp's source to the pattern rxCompile takes (under goPrefix): /x's spacing and
// comments are dropped (extended is x outside any group), then the syntax RE2 reads differently is rewritten.
func rxPrepare(src string, extended bool) (string, error) {
	return translateRegexp(stripExtended(src, extended))
}

// translateRegexp rewrites Ruby (Onigmo) regexp syntax RE2 spells or reads
// differently; callers want rxPrepare, which strips /x first. Constructs RE2 lacks (lookaround, backreferences, \Z,
// atomic groups) are left in place for rxCompile to hand to the rxAlt engine (decision 242); \g, \G, \K, \R, \X and
// possessive quantifiers are rejected.
func translateRegexp(src string) (string, error) {
	t := rxTranslator{src: src}
	err := t.run()
	if err == nil && t.sawNamed {
		// MRI does not capture plain (...) once a pattern names a group.
		t = rxTranslator{src: src, named: true}
		err = t.run()
	}
	return string(t.out), err
}

type rxTranslator struct {
	src             string
	i               int
	out             []byte
	named, sawNamed bool
}

var (
	rxFlagGroup  = regexp.MustCompile(`^\(\?([imx]*)(?:-([imx]*))?([:)])`)
	rxInterval   = regexp.MustCompile(`^\{(\d*)(,?)(\d*)\}`)
	rxUnicodeEsc = regexp.MustCompile(`^\\u(?:([0-9a-fA-F]{4})|\{ *([0-9a-fA-F]{1,6}(?: +[0-9a-fA-F]{1,6})*) *\})`)
	rxPosixName  = regexp.MustCompile(`^\[:(\^?)([a-z]+):\]`)
)

// rxSpace is Onigmo's \s, which unlike RE2's includes \v.
const rxSpace = `\t\n\v\f\r `

// rxPosix spells Onigmo's POSIX bracket classes, which are Unicode
// properties (RE2's are ASCII), as RE2 class bodies.
var rxPosix = func() map[string]string {
	alpha := `\p{L}\p{Nl}` + rxTable(unicode.Other_Alphabetic)
	graph := `\p{L}\p{M}\p{N}\p{P}\p{S}\p{Cf}\p{Co}`
	return map[string]string{
		"alnum": alpha + `\p{Nd}`, "alpha": alpha, "ascii": `\x00-\x7f`,
		"blank": `\t\p{Zs}`, "cntrl": `\p{Cc}`, "digit": `\p{Nd}`,
		"graph": graph, "lower": `\p{Ll}` + rxTable(unicode.Other_Lowercase),
		"print": graph + `\p{Zs}`, "punct": `\p{P}$+<=>\^` + "`|~",
		"space": `\t-\r\x{85}\p{Z}`, "upper": `\p{Lu}` + rxTable(unicode.Other_Uppercase),
		"word": alpha + `\p{M}\p{Nd}\p{Pc}`, "xdigit": `0-9A-Fa-f`,
	}
}()

func (t *rxTranslator) run() error {
	var groups []int // out offsets of open groups
	atom := -1       // out offset of the last quantifiable atom
	for t.i < len(t.src) {
		start := len(t.out)
		switch c := t.src[t.i]; c {
		case '\\':
			s, err := t.escape(false)
			if err != nil {
				return err
			}
			t.out, atom = append(t.out, s...), start
		case '[':
			err := t.class()
			if err != nil {
				return err
			}
			atom = start
		case '(':
			if strings.HasPrefix(t.src[t.i:], "(?#") { // a comment, which RE2 lacks: up to the first ')', as Onigmo's
				end := strings.IndexByte(t.src[t.i:], ')')
				if end < 0 {
					return errors.New("end pattern in group")
				}
				t.i += end + 1
				continue
			}
			if t.group() {
				groups = append(groups, start)
			}
			atom = -1
		case ')':
			if len(groups) > 0 {
				atom, groups = groups[len(groups)-1], groups[:len(groups)-1]
			}
			t.out = append(t.out, c)
			t.i++
		case '{':
			t.interval(atom)
		case '|', '^', '$':
			t.out, atom = append(t.out, c), -1
			t.i++
		case '*', '+', '?':
			t.out = append(t.out, c)
			t.i++
		default:
			_, n := utf8.DecodeRuneInString(t.src[t.i:])
			t.out, atom = append(t.out, t.src[t.i:t.i+n]...), start
			t.i += n
		}
	}
	return nil
}

// group copies a group opener, reporting whether it opened a group: an
// inline option like (?i) does not. Ruby's m option is RE2's s; RE2's m is
// always on (goPrefix). x off is RE2's only mode, so it is dropped (an
// embedded Regexp#to_s is (?i-mx:...)); x on is left for RE2 to reject.
func (t *rxTranslator) group() bool {
	rest := t.src[t.i:]
	if m := rxFlagGroup.FindStringSubmatch(rest); m != nil {
		flags := m[1]
		if off := strings.ReplaceAll(m[2], "x", ""); off != "" {
			flags += "-" + off
		}
		t.out = append(t.out, "(?"+strings.ReplaceAll(flags, "m", "s")+m[3]...)
		t.i += len(m[0])
		return m[3] == ":"
	}
	open := "("
	switch {
	case strings.HasPrefix(rest, "(?<") && !strings.HasPrefix(rest, "(?<=") && !strings.HasPrefix(rest, "(?<!"):
		t.sawNamed = true
		open = "(?<"
	case strings.HasPrefix(rest, "(?"):
		open = "(?"
	}
	t.i += len(open)
	if open == "(" && t.named {
		open = "(?:"
	}
	t.out = append(t.out, open...)
	return true
}

// interval copies a {n,m} quantifier. Ruby's {,n} is {0,n}; X{n}? and a
// following + or * repeat (?:X{n}) where RE2 reads laziness or rejects.
func (t *rxTranslator) interval(atom int) {
	m := rxInterval.FindStringSubmatch(t.src[t.i:])
	if m == nil || m[1] == "" && m[3] == "" {
		t.out = append(t.out, '{')
		t.i++
		return
	}
	t.i += len(m[0])
	lo := m[1]
	if lo == "" {
		lo = "0"
	}
	q := "{" + lo + m[2] + m[3] + "}"
	if next := byte(0); atom >= 0 {
		if t.i < len(t.src) {
			next = t.src[t.i]
		}
		if next == '+' || next == '*' || next == '?' && m[2] == "" {
			t.out = slices.Insert(t.out, atom, []byte("(?:")...)
			q += ")"
		}
	}
	t.out = append(t.out, q...)
}

// escape translates the escape at t.src[t.i], inside a class body or not.
func (t *rxTranslator) escape(inClass bool) (string, error) {
	if m := rxUnicodeEsc.FindStringSubmatch(t.src[t.i:]); m != nil {
		t.i += len(m[0])
		var b strings.Builder
		for _, h := range strings.Fields(m[1] + m[2]) {
			b.WriteString(`\x{` + h + `}`)
		}
		return b.String(), nil
	}
	if t.i+1 >= len(t.src) {
		t.i++
		return `\`, nil
	}
	_, n := utf8.DecodeRuneInString(t.src[t.i+1:])
	esc := t.src[t.i : t.i+1+n]
	escStart := t.i
	t.i += len(esc)
	class := func(body string) string {
		if inClass {
			return body
		}
		return "[" + body + "]"
	}
	switch esc[1] {
	case 'h':
		return class("0-9a-fA-F"), nil
	case 's':
		return class(rxSpace), nil
	case 'H':
		if inClass {
			return "", errors.New(`\H inside a character class`)
		}
		return "[^0-9a-fA-F]", nil
	case 'S':
		if inClass {
			return rxNot(rxSpace)
		}
		return "[^" + rxSpace + "]", nil
	case 'e':
		return `\x1b`, nil
	case 'b': // backspace in a class (a word boundary outside), which RE2 has no escape for
		if inClass {
			return `\x08`, nil
		}
	case 'Q', 'E':
		return esc[1:], nil
	case 'p', 'P':
		if end := strings.IndexByte(t.src[t.i:], '}'); strings.HasPrefix(t.src[t.i:], "{") && end > 0 {
			name := t.src[t.i+1 : t.i+end]
			esc += t.src[t.i : t.i+end+1]
			t.i += end + 1
			neg := esc[1] == 'P'
			if rest, ok := strings.CutPrefix(name, "^"); ok {
				name, neg = rest, !neg
			}
			// Onigmo's POSIX-named properties (kramdown's `\p{Word}`): RE2 and regexp2 know only general categories and scripts
			if body, ok := rxPosix[strings.ToLower(strings.NewReplacer("_", "", " ", "").Replace(name))]; ok {
				switch {
				case inClass && neg:
					return rxNot(body)
				case inClass:
					return body, nil
				case neg:
					return "[^" + body + "]", nil
				}
				return "[" + body + "]", nil
			}
		}
	case 'Z':
		// end, or before a final newline (amends 24): a lookahead, which the rxAlt engine matches (decision 242)
		if inClass {
			return "", errors.New(`\Z inside a character class`)
		}
		return `(?=\n?\z)`, nil
	case 'g': // a subexpression call, which regexp2 would read as a literal g (decision 242)
		return "", errors.New(`\g (a subexpression call) is not supported`)
	case 'R', 'G':
		return linebreakOrStart(esc, inClass, escStart == 0)
	case 'K', 'X':
		return "", fmt.Errorf(`%s is not supported by RE2`, esc)
	}
	return esc, nil
}

// linebreakOrStart translates \R, a linebreak (\r\n as one, or any single line terminator), and a leading \G: where the
// search began, which for one match is its start, \A of the text searched; Regexp#match(s, pos) searches the text from
// pos for such a pattern. A scan or gsub would match only once (ponytail: \G as the end of the previous match needs the
// engines to carry it). decision 444
func linebreakOrStart(esc string, inClass, leading bool) (string, error) {
	switch {
	case esc[1] == 'R' && inClass:
		return "", errors.New(`\R inside a character class`)
	case esc[1] == 'R':
		return `(?:\r\n|[\n\x0b\x0c\r\x{85}\x{2028}\x{2029}])`, nil
	case leading:
		return `\A`, nil
	}
	return "", fmt.Errorf(`%s is not supported by RE2`, esc)
}

// class translates the bracket expression at t.src[t.i].
func (t *rxTranslator) class() error {
	start := t.i
	body, neg, closed, err := t.classBody()
	if err != nil {
		return err
	}
	switch {
	case !closed:
		// Left for RE2 to reject.
		t.out = append(t.out, t.src[start:]...)
	case body == "" && neg:
		t.out = append(t.out, `[\x00-\x{10ffff}]`...)
	case body == "":
		t.out = append(t.out, `[^\x00-\x{10ffff}]`...)
	case neg:
		t.out = append(t.out, "[^"+body+"]"...)
	default:
		t.out = append(t.out, "["+body+"]"...)
	}
	return nil
}

// classBody reads a bracket expression into a positive RE2 class body ("" is
// empty). Onigmo's set syntax (nested classes, &&) and negated members have
// no RE2 spelling, so those are computed as rune ranges.
func (t *rxTranslator) classBody() (body string, neg, closed bool, err error) {
	t.i++
	if strings.HasPrefix(t.src[t.i:], "^") {
		neg = true
		t.i++
	}
	var union strings.Builder
	var and []string
	for t.i < len(t.src) {
		rest := t.src[t.i:]
		var s string
		switch {
		case rest[0] == ']':
			t.i++
			if and == nil {
				return union.String(), neg, true, nil
			}
			body, err = rxAnd(append(and, union.String()))
			return body, neg, true, err
		case strings.HasPrefix(rest, "&&"):
			and = append(and, union.String())
			union.Reset()
			t.i += 2
		case rest[0] == '[':
			s, err = t.classMember()
		case rest[0] == '\\':
			s, err = t.escape(true)
		default:
			_, n := utf8.DecodeRuneInString(rest)
			s = rest[:n]
			t.i += n
		}
		if err != nil {
			return "", false, false, err
		}
		union.WriteString(s)
	}
	return union.String(), neg, false, nil
}

// classMember reads a POSIX bracket or a nested class inside a class.
func (t *rxTranslator) classMember() (string, error) {
	if m := rxPosixName.FindStringSubmatch(t.src[t.i:]); m != nil {
		body, ok := rxPosix[m[2]]
		if !ok {
			return "", fmt.Errorf("invalid POSIX bracket type [:%s:]", m[2])
		}
		t.i += len(m[0])
		if m[1] != "" {
			return rxNot(body)
		}
		return body, nil
	}
	body, neg, _, err := t.classBody()
	if err != nil || !neg {
		return body, err
	}
	return rxNot(body)
}

// rxAnd intersects class bodies: the complement of their complements' union.
func rxAnd(bodies []string) (string, error) {
	var union strings.Builder
	for _, b := range bodies {
		nb, err := rxNot(b)
		if err != nil {
			return "", err
		}
		union.WriteString(nb)
	}
	return rxNot(union.String())
}

// rxNot is the complement of a class body, computed by RE2's own parser.
func rxNot(body string) (string, error) {
	if body == "" {
		return `\x00-\x{10ffff}`, nil
	}
	re, err := syntax.Parse("[^"+body+"]", syntax.Perl)
	if err != nil {
		return "", fmt.Errorf("character class: %w", err)
	}
	var rs []rune
	// a parsed class is one of these, or OpNoMatch (empty)
	//exhaustive:ignore
	switch re.Op {
	case syntax.OpCharClass:
		rs = re.Rune
	case syntax.OpAnyChar:
		rs = []rune{0, unicode.MaxRune}
	case syntax.OpLiteral: // one rune left, or one case-fold orbit
		for r := re.Rune[0]; ; {
			rs = append(rs, r, r)
			if r = unicode.SimpleFold(r); re.Flags&syntax.FoldCase == 0 || r == re.Rune[0] {
				break
			}
		}
	}
	var b strings.Builder
	for i := 0; i < len(rs); i += 2 {
		b.WriteString(rxRune(rs[i]))
		if rs[i+1] != rs[i] {
			b.WriteString("-" + rxRune(rs[i+1]))
		}
	}
	return b.String(), nil
}

// rxTable spells a Unicode table RE2 cannot name as a class body.
func rxTable(tab *unicode.RangeTable) string {
	var b strings.Builder
	add := func(lo, hi, stride rune) {
		if stride == 1 {
			b.WriteString(rxRune(lo))
			if hi != lo {
				b.WriteString("-" + rxRune(hi))
			}
			return
		}
		for r := lo; r <= hi; r += stride {
			b.WriteString(rxRune(r))
		}
	}
	for _, r := range tab.R16 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride))
	}
	for _, r := range tab.R32 {
		add(rune(r.Lo), rune(r.Hi), rune(r.Stride)) //nolint:gosec // Unicode tables hold runes
	}
	return b.String()
}

func rxRune(r rune) string {
	if r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
		return string(r)
	}
	return `\x{` + strconv.FormatInt(int64(r), 16) + `}`
}

// rxRegexp is a compiled Ruby regexp: RE2, plus Onigmo's readings of ^ and
// \b, which RE2 cannot spell. Onigmo's ^ does not match at the end after a
// final newline, and its \b's word characters are Unicode's (Ruby's
// WORD_BOUND_ALL_RANGE; \w stays ASCII). Subjects where they could change
// the match run on prog, the pattern's program under those readings; prog
// is nil when the pattern has neither and cannot match the empty string.
type rxRegexp struct {
	*regexp.Regexp
	prog        *syntax.Prog
	caret, word bool
	empty       bool    // can match the empty string: Ruby's scan keeps an empty match right after a match, RE2's FindAll drops it
	unsupported string  // a gem's pattern RE2 cannot match: built anyway, it raises when used (rbRegexpLenient)
	alt         rxAlt   // a pattern RE2 cannot match (lookaround, backreferences, \Z): regexp2's (decision 242)
	pattern     string  // alt's pattern, for String
	fast        *rxFast // a pattern simple enough to match without RE2's machinery, or nil (decision 347)
}

// rxFast matches a pattern that is one ASCII character class, such a class with +, or a case-sensitive ASCII
// literal, each optionally \A-anchored, with a byte table or strings.Index. Go's regexp runs even /\d/ through its
// backtracker, at microseconds a line. Exact: an ASCII byte of a UTF-8 string is always a whole character, and one
// of an invalid string is a character to RE2 too; the patterns have no groups, cannot match empty, and use no
// assertion Onigmo reads differently (rxNew sets it only when prog is nil).
type rxFast struct {
	set      [256]bool
	plus     bool
	lit      string // non-empty: a literal
	anchored bool
}

// rxFastOf is t's rxFast, or nil when t is not that simple.
func rxFastOf(t *syntax.Regexp) *rxFast {
	f := &rxFast{}
	if t.Op == syntax.OpConcat && len(t.Sub) == 2 && t.Sub[0].Op == syntax.OpBeginText {
		f.anchored = true
		t = t.Sub[1]
	}
	if t.Op == syntax.OpPlus {
		if t.Flags&syntax.NonGreedy != 0 { // \d+? matches one character
			return nil
		}
		f.plus = true
		t = t.Sub[0]
	}
	if t.Op == syntax.OpCharClass {
		for i := 0; i+1 < len(t.Rune); i += 2 {
			lo, hi := t.Rune[i], t.Rune[i+1]
			if hi >= utf8.RuneSelf {
				return nil
			}
			for c := lo; c <= hi; c++ {
				f.set[c] = true
			}
		}
		return f
	}
	if t.Op != syntax.OpLiteral || f.plus || t.Flags&syntax.FoldCase != 0 || len(t.Rune) == 0 {
		return nil
	}
	for _, c := range t.Rune {
		if c >= utf8.RuneSelf {
			return nil
		}
	}
	f.lit = string(t.Rune)
	return f
}

// at is the leftmost match at byte from or later, as start and end, or -1, -1.
func (f *rxFast) at(s string, from int) (int, int) {
	if f.anchored && from != 0 {
		return -1, -1
	}
	if f.lit != "" {
		if f.anchored {
			if strings.HasPrefix(s, f.lit) {
				return 0, len(f.lit)
			}
			return -1, -1
		}
		i := strings.Index(s[from:], f.lit)
		if i < 0 {
			return -1, -1
		}
		return from + i, from + i + len(f.lit)
	}
	i := from
	for i < len(s) && !f.set[s[i]] {
		if f.anchored {
			return -1, -1
		}
		i++
	}
	if i == len(s) {
		return -1, -1
	}
	j := i + 1
	for f.plus && j < len(s) && f.set[s[j]] {
		j++
	}
	return i, j
}

// find is FindStringSubmatchIndex's answer: the match, or nil.
func (f *rxFast) find(s string) []int {
	if i, j := f.at(s, 0); i >= 0 {
		return []int{i, j}
	}
	return nil
}

// findAll is FindAllStringSubmatchIndex's answer: up to n matches (all when n < 0), each searched from the last's end.
func (f *rxFast) findAll(s string, n int) [][]int {
	var out [][]int
	var buf []int
	for from := 0; n < 0 || len(out) < n; {
		i, j := f.at(s, from)
		if i < 0 {
			break
		}
		if len(buf) < 2 {
			buf = make([]int, 32) // the locations share backing arrays, as Go's FindAll does
		}
		loc := buf[:2:2]
		buf = buf[2:]
		loc[0], loc[1] = i, j
		out = append(out, loc)
		from = j
	}
	return out
}

// rxAlt is a backtracking engine for what RE2 cannot express. Only programs that may need it load one
// (rxalt.go, emitted when needed, sets rxAltCompile), so the rest do not link it (decision 242).
type rxAlt interface {
	// findAll is up to n (all when n < 0) leftmost matches in s from byte from on, each as byte offsets: the match,
	// then each group (-1, -1 when it did not take part), as Go's FindAllStringSubmatchIndex. An engine failure is
	// an error, never a short result; errRxTimeout is the match time limit.
	findAll(s string, from, n int) ([][]int, error)
	names() []string // as Go's SubexpNames: index 0 and unnamed groups ""
}

// errRxTimeout is an rxAlt match that ran past its time limit (rxalt.go's rxMatchTimeout).
var errRxTimeout = errors.New("regexp match timeout")

// RxTimeout is the panic a match raises on errRxTimeout; the runtime makes it a Regexp::TimeoutError.
type RxTimeout string

func (e RxTimeout) Error() string { return string(e) }

// altAll is alt's matches, or the panic its failure is: a timeout is RxTimeout, anything else a RegexpError.
func (r *rxRegexp) altAll(s string, n int) [][]int {
	out, err := r.alt.findAll(s, 0, n)
	switch {
	case errors.Is(err, errRxTimeout):
		panic(RxTimeout(err.Error()))
	case err != nil:
		panic(RxUnsupported(err.Error()))
	}
	return out
}

// rxAltCompile compiles a translated pattern RE2 rejected, when the program loaded an rxAlt engine; nil otherwise.
var rxAltCompile func(pattern string) (rxAlt, error)

// rxCompile compiles a translated pattern: RE2 when it can, else the rxAlt engine (decision 242). The error is RE2's
// when no engine takes the pattern.
func rxCompile(pattern string) (*rxRegexp, error) {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return rxNew(re), nil
	}
	if rxAltCompile != nil {
		alt, aerr := rxAltCompile(pattern)
		if aerr == nil {
			return &rxRegexp{alt: rxAheadOr(pattern, alt), pattern: pattern}, nil
		}
	}
	return nil, err //nolint:wrapcheck // RE2's own error: rbRegexpErr words it by its code
}

// rxAhead runs a pattern that is one lookahead, (?=X), on RE2 when X is RE2's: its first match is empty, at the
// leftmost place X matches, with X's groups, which is what RE2's leftmost-first search for X finds. Other uses
// (every match, for scan and gsub) go to the backtracking engine.
type rxAhead struct {
	body *rxRegexp
	full rxAlt
}

func (a *rxAhead) findAll(s string, from, n int) ([][]int, error) {
	if n != 1 || from != 0 {
		return a.full.findAll(s, from, n)
	}
	loc := a.body.FindStringSubmatchIndex(s)
	if loc == nil {
		return nil, nil
	}
	loc = slices.Clone(loc)
	loc[1] = loc[0]
	return [][]int{loc}, nil
}

func (a *rxAhead) names() []string { return a.body.SubexpNames() }

// rxAheadOr is full, or an rxAhead over it when pattern is one lookahead whose body RE2 compiles.
func rxAheadOr(pattern string, full rxAlt) rxAlt {
	body, ok := rxLookaheadBody(pattern)
	if !ok {
		return full
	}
	re, err := regexp.Compile(body)
	if err != nil {
		return full
	}
	return &rxAhead{rxNew(re), full}
}

// rxLookaheadBody is X for a pattern that is leading flag groups and then exactly (?=X): the flags, then X, with
// a lookahead that ends one of X's branches made a plain group. That lookahead is atomic and nothing follows it
// inside the outer one, so consuming it changes no outcome or group; X's own groups keep their numbers.
func rxLookaheadBody(p string) (string, bool) {
	flags := 0
	for strings.HasPrefix(p[flags:], "(?") {
		end := strings.IndexByte(p[flags:], ')')
		if end < 0 || strings.ContainsAny(p[flags+2:flags+end], ":=!<>P()") {
			break
		}
		flags += end + 1
	}
	rest := p[flags:]
	if !strings.HasPrefix(rest, "(?=") || rxGroupEnd(rest, 0) != len(rest)-1 {
		return "", false
	}
	return p[:flags] + rxLowerTailAhead(rest[3:len(rest)-1]), true
}

// rxGroupEnd is the index of the ')' closing the group opened at x[i], or -1.
func rxGroupEnd(x string, i int) int {
	depth := 0
	for ; i < len(x); i++ {
		switch x[i] {
		case '\\':
			i++
		case '[':
			i = rxClassEnd(x, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// rxClassEnd is the index of the ']' closing the class opened at x[i] (a ']' first in it is literal).
func rxClassEnd(x string, i int) int {
	i++
	if i < len(x) && x[i] == '^' {
		i++
	}
	if i < len(x) && x[i] == ']' {
		i++
	}
	for ; i < len(x); i++ {
		switch x[i] {
		case '\\':
			i++
		case '[':
			if j := strings.Index(x[i:], ":]"); strings.HasPrefix(x[i:], "[:") && j > 0 {
				i += j + 1
			}
		case ']':
			return i
		}
	}
	return len(x)
}

// rxLowerTailAhead makes a lookahead that ends a branch of x a plain group, looking into a non-capturing group that
// ends a branch (a capturing one would then hold more text).
func rxLowerTailAhead(x string) string {
	var out []string
	start, depth := 0, 0
	branch := func(b string) string {
		if !strings.HasSuffix(b, ")") {
			return b
		}
		open := -1
		for i := 0; i < len(b); i++ { // the group that ends b: the last top-level '('
			switch b[i] {
			case '\\':
				i++
			case '[':
				i = rxClassEnd(b, i)
			case '(':
				if end := rxGroupEnd(b, i); end == len(b)-1 {
					open = i
				} else if end > i {
					i = end
				}
			}
		}
		switch {
		case open < 0:
			return b
		case strings.HasPrefix(b[open:], "(?="):
			return b[:open] + "(?:" + b[open+3:]
		case strings.HasPrefix(b[open:], "(?:"):
			return b[:open] + "(?:" + rxLowerTailAhead(b[open+3:len(b)-1]) + ")"
		case strings.HasPrefix(b[open:], "(?") && !strings.ContainsAny(b[open+2:open+3], "=!<>P"):
			colon := strings.IndexByte(b[open:], ':')
			if colon < 0 || strings.ContainsAny(b[open+2:open+colon], "()") {
				return b
			}
			return b[:open+colon+1] + rxLowerTailAhead(b[open+colon+1:len(b)-1]) + ")"
		}
		return b
	}
	for i := 0; i < len(x); i++ {
		switch x[i] {
		case '\\':
			i++
		case '[':
			i = rxClassEnd(x, i)
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				out = append(out, branch(x[start:i]))
				start = i + 1
			}
		}
	}
	return strings.Join(append(out, branch(x[start:])), "|")
}

// The compiler never calls the lazy path; the runtime's rbRegexpLenient does.
var _ = rxCompileLazy

// rxCompileLazy is rxCompile for a pattern whose failure may wait until it is used (a gem's: decision 176): one RE2
// rejects compiles on the rxAlt engine at its first match, since a gem defines many it never uses and regexp2's
// compile is the expensive part (large Unicode classes). src words the error a failed compile raises then.
func rxCompileLazy(pattern, src string) (*rxRegexp, error) {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return rxNew(re), nil
	}
	if rxAltCompile == nil {
		return nil, err //nolint:wrapcheck // RE2's own error, as rxCompile's
	}
	return &rxRegexp{alt: rxAheadOr(pattern, &rxAltLazy{pattern: pattern, src: src, re2: err.Error()}), pattern: pattern}, nil
}

// rxAltLazy is an rxAlt compiled at its first use; a pattern neither engine takes raises then, worded as
// rbRegexpLenient words it at creation (RE2's error).
type rxAltLazy struct {
	pattern, src, re2 string
	once              sync.Once
	alt               rxAlt
}

func (l *rxAltLazy) get() rxAlt {
	l.once.Do(func() {
		alt, err := rxAltCompile(l.pattern)
		if err == nil {
			l.alt = alt
		}
	})
	if l.alt == nil {
		panic(RxUnsupported("regexp /" + l.src + "/ is not supported by rugaroo: " + l.re2))
	}
	return l.alt
}

func (l *rxAltLazy) findAll(s string, from, n int) ([][]int, error) {
	return l.get().findAll(s, from, n)
}
func (l *rxAltLazy) names() []string { return l.get().names() }

// altFirst is alt's first match in s, or nil.
func (r *rxRegexp) altFirst(s string) []int {
	if m := r.altAll(s, 1); len(m) > 0 {
		return m[0]
	}
	return nil
}

// RxUnsupported is the panic a lenient regexp raises when used; the runtime makes it a RegexpError.
type RxUnsupported string

func (e RxUnsupported) Error() string { return string(e) }

func (r *rxRegexp) check() {
	if r.unsupported != "" {
		panic(RxUnsupported(r.unsupported))
	}
}

func (r *rxRegexp) SubexpNames() []string {
	r.check()
	if r.alt != nil {
		return r.alt.names()
	}
	return r.Regexp.SubexpNames()
}

func (r *rxRegexp) NumSubexp() int {
	r.check()
	if r.alt != nil {
		return len(r.alt.names()) - 1
	}
	return r.Regexp.NumSubexp()
}

func (r *rxRegexp) String() string {
	r.check()
	if r.alt != nil {
		return r.pattern
	}
	return r.Regexp.String()
}

func (r *rxRegexp) FindAllStringIndex(s string, n int) [][]int {
	r.check()
	if r.fast != nil {
		return r.fast.findAll(s, n)
	}
	if r.alt == nil && r.prog == nil {
		return r.Regexp.FindAllStringIndex(s, n)
	}
	out := r.FindAllStringSubmatchIndex(s, n)
	for i := range out {
		out[i] = out[i][:2]
	}
	return out
}

// FindAllStringSubmatchIndex is Ruby's scan/gsub matches: RE2's (then Onigmo's ^ and \b are applied where they could differ), except an empty match is kept right after a match.
func (r *rxRegexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	r.check()
	if r.alt != nil {
		return r.altAll(s, n)
	}
	if r.fast != nil {
		return r.fast.findAll(s, n)
	}
	if r.prog == nil {
		return r.Regexp.FindAllStringSubmatchIndex(s, n)
	}
	if !r.empty {
		if out := r.Regexp.FindAllStringSubmatchIndex(s, n); !r.differsAll(s, out) {
			return out
		}
	}
	return r.backtrackAll(s, n)
}

// rxNew wraps re, compiled from a translated pattern (after rxFold).
func rxNew(re *regexp.Regexp) *rxRegexp {
	r := &rxRegexp{Regexp: re}
	tree, _ := syntax.Parse(re.String(), syntax.Perl)
	var walk func(*syntax.Regexp)
	walk = func(t *syntax.Regexp) {
		r.caret = r.caret || t.Op == syntax.OpBeginLine
		r.word = r.word || t.Op == syntax.OpWordBoundary || t.Op == syntax.OpNoWordBoundary
		for _, sub := range t.Sub {
			walk(sub)
		}
	}
	walk(tree)
	r.empty = rxCanEmpty(tree)
	if r.caret || r.word || r.empty {
		r.prog, _ = syntax.Compile(tree.Simplify())
	} else {
		r.fast = rxFastOf(tree.Simplify())
	}
	return r
}

// rxCanEmpty reports whether t can match the empty string; an assertion counts, since it matches nothing but can still match somewhere.
func rxCanEmpty(t *syntax.Regexp) bool {
	// an empty match, a star or quest, and the assertions fall through to true
	//exhaustive:ignore
	switch t.Op {
	case syntax.OpNoMatch, syntax.OpLiteral, syntax.OpCharClass, syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		return t.Op == syntax.OpLiteral && len(t.Rune) == 0
	case syntax.OpConcat:
		return !slices.ContainsFunc(t.Sub, func(s *syntax.Regexp) bool { return !rxCanEmpty(s) })
	case syntax.OpAlternate:
		return slices.ContainsFunc(t.Sub, rxCanEmpty)
	case syntax.OpCapture, syntax.OpPlus:
		return rxCanEmpty(t.Sub[0])
	case syntax.OpRepeat:
		return t.Min == 0 || rxCanEmpty(t.Sub[0])
	}
	return true // an empty match, a star or quest, or an assertion
}

func (r *rxRegexp) MatchString(s string) bool {
	r.check()
	if r.alt != nil {
		return r.altFirst(s) != nil
	}
	if r.fast != nil {
		i, _ := r.fast.at(s, 0)
		return i >= 0
	}
	if !r.caret && !r.word {
		return r.Regexp.MatchString(s)
	}
	return r.FindStringIndex(s) != nil
}

func (r *rxRegexp) FindStringIndex(s string) []int {
	r.check()
	if r.alt != nil {
		if m := r.altFirst(s); m != nil {
			return m[:2]
		}
		return nil
	}
	if r.fast != nil {
		return r.fast.find(s)
	}
	loc := r.Regexp.FindStringIndex(s)
	if r.differs(s, loc) {
		if loc = r.backtrack(s); loc != nil {
			loc = loc[:2]
		}
	}
	return loc
}

func (r *rxRegexp) FindStringSubmatchIndex(s string) []int {
	r.check()
	if r.alt != nil {
		return r.altFirst(s)
	}
	if r.fast != nil {
		return r.fast.find(s)
	}
	loc := r.Regexp.FindStringSubmatchIndex(s)
	if r.differs(s, loc) {
		loc = r.backtrack(s)
	}
	return loc
}

// FindPrefixSubmatchIndex is FindStringSubmatchIndex for a \A-anchored pattern (StringScanner's twins). RE2's
// backtracker clears a bitmap the size of the whole input before every match, so on a long input the match runs
// through a reader instead (NFA or one-pass), whose cost follows the match, not the input. Below rxPrefixDirect bytes
// the string search is cheaper still: kramdown's 2-4 KB rests cost twice as much through the reader (decision 347).
func (r *rxRegexp) FindPrefixSubmatchIndex(s string) []int {
	r.check()
	if r.alt != nil || r.fast != nil || len(s) < rxPrefixDirect {
		return r.FindStringSubmatchIndex(s)
	}
	loc := r.FindReaderSubmatchIndex(strings.NewReader(s))
	if r.differs(s, loc) {
		loc = r.backtrack(s)
	}
	return loc
}

// rxPrefixDirect is the subject length from which FindPrefixSubmatchIndex reads through RE2's reader (decision 347).
const rxPrefixDirect = 8192

// differs reports whether Onigmo could match s otherwise than RE2 did (loc):
// s has a non-ASCII word character, for \b; or RE2's match ends at a final
// newline, where its ^ may have matched and Onigmo's cannot. Onigmo's ^
// matches nowhere RE2's does not, so any other RE2 match is Onigmo's first.
func (r *rxRegexp) differs(s string, loc []int) bool {
	if r.word && strings.ContainsFunc(s, func(c rune) bool { return c >= utf8.RuneSelf && rxWord(c) }) {
		return true
	}
	return r.caret && loc != nil && loc[1] == len(s) && strings.HasSuffix(s, "\n")
}

// differsAll is differs for a match list: RE2's matches are Onigmo's unless s has a non-ASCII word character, or one ends at a final newline.
func (r *rxRegexp) differsAll(s string, locs [][]int) bool {
	return r.differs(s, nil) || r.caret && len(locs) > 0 && r.differs(s, locs[len(locs)-1])
}

// rxWord is Onigmo's Unicode word character: Alphabetic, Mark,
// Decimal_Number or Connector_Punctuation.
func rxWord(c rune) bool {
	return unicode.In(c, unicode.L, unicode.Nl, unicode.Other_Alphabetic, unicode.M, unicode.Nd, unicode.Pc)
}

// rxEmpty reports whether the empty-width assertions op hold at pos, as
// Onigmo reads them.
func rxEmpty(op syntax.EmptyOp, s string, pos int) bool {
	before, _ := utf8.DecodeLastRuneInString(s[:pos])
	after, _ := utf8.DecodeRuneInString(s[pos:])
	flags := syntax.EmptyNoWordBoundary
	if rxWord(before) != rxWord(after) {
		flags = syntax.EmptyWordBoundary
	}
	if pos == 0 {
		flags |= syntax.EmptyBeginText | syntax.EmptyBeginLine
	} else if before == '\n' && pos < len(s) {
		flags |= syntax.EmptyBeginLine
	}
	if pos == len(s) {
		flags |= syntax.EmptyEndText | syntax.EmptyEndLine
	} else if after == '\n' {
		flags |= syntax.EmptyEndLine
	}
	return op&^flags == 0
}

// backtrack is regexp/backtrack.go's bit-state backtracker over prog:
// leftmost-first, as RE2 is, with rxEmpty's assertions.
// ponytail: visited is len(prog)×len(s) bits; a Pike VM would bound it for huge subjects.
func (r *rxRegexp) backtrack(s string) []int {
	return r.newBacktrack(s).search(0)
}

// backtrackAll is Ruby's scan loop on the backtracker: after a match the next search starts at its end, or one character on when it was empty.
func (r *rxRegexp) backtrackAll(s string, n int) [][]int {
	b := r.newBacktrack(s)
	var out [][]int
	for from := 0; from <= len(s) && (n < 0 || len(out) < n); {
		loc := b.search(from)
		if loc == nil {
			break
		}
		out = append(out, slices.Clone(loc))
		from = loc[1]
		if loc[0] == loc[1] {
			_, w := utf8.DecodeRuneInString(s[from:])
			from += max(w, 1)
		}
	}
	return out
}

func (r *rxRegexp) newBacktrack(s string) *rxBacktrack {
	np := len(r.prog.Inst)
	return &rxBacktrack{prog: r.prog, s: s, np: np, visited: make([]uint64, (np*(len(s)+1)+63)/64), caps: make([]int, 2*r.NumSubexp()+2)}
}

type rxBacktrack struct {
	prog    *syntax.Prog
	s       string
	np      int      // len(prog.Inst)
	visited []uint64 // (pos, pc) pairs already tried: they cannot match
	hi      int      // the furthest pos visited since the last search began
	caps    []int
	jobs    []rxJob
}

// search is the leftmost match starting at from or later (the whole of s is its context), in caps, or nil. Searches on one rxBacktrack must not go backwards.
func (b *rxBacktrack) search(from int) []int {
	if lo, hi := from*b.np/64, min(((b.hi+1)*b.np+63)/64, len(b.visited)); lo < hi {
		clear(b.visited[lo:hi]) // what the last search tried at from on may now match
	}
	b.hi = from
	for start := from; ; {
		if b.try(start) {
			return b.caps
		}
		if start == len(b.s) {
			return nil
		}
		_, w := utf8.DecodeRuneInString(b.s[start:])
		start += w
	}
}

// rxJob is a lower-priority thread to try, or (slot >= 0) a capture to
// restore to pos when the threads after it have failed.
type rxJob struct{ pc, pos, slot int }

// try reports whether a match starts at start, leaving it in caps.
func (b *rxBacktrack) try(start int) bool {
	for i := range b.caps {
		b.caps[i] = -1
	}
	b.caps[0] = start
	b.jobs = append(b.jobs[:0], rxJob{b.prog.Start, start, -1})
	for len(b.jobs) > 0 {
		j := b.jobs[len(b.jobs)-1]
		b.jobs = b.jobs[:len(b.jobs)-1]
		if j.slot >= 0 {
			b.caps[j.slot] = j.pos
		} else if b.thread(j.pc, j.pos) {
			return true
		}
	}
	return false
}

// thread runs one thread until it fails or matches, queueing the
// alternatives it passes.
func (b *rxBacktrack) thread(pc, pos int) bool {
	for {
		k := pos*b.np + pc
		b.hi = max(b.hi, pos)
		if b.visited[k/64]&(1<<(k%64)) != 0 {
			return false
		}
		b.visited[k/64] |= 1 << (k % 64)
		inst := &b.prog.Inst[pc]
		switch inst.Op {
		case syntax.InstMatch:
			b.caps[1] = pos
			return true
		case syntax.InstFail:
			return false
		case syntax.InstAlt, syntax.InstAltMatch:
			b.jobs = append(b.jobs, rxJob{int(inst.Arg), pos, -1})
		case syntax.InstCapture:
			if slot := int(inst.Arg); slot < len(b.caps) {
				b.jobs = append(b.jobs, rxJob{0, b.caps[slot], slot})
				b.caps[slot] = pos
			}
		case syntax.InstEmptyWidth:
			if !rxEmpty(syntax.EmptyOp(inst.Arg), b.s, pos) { //nolint:gosec // an EmptyWidth Arg holds EmptyOp bits
				return false
			}
		case syntax.InstNop:
		case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			c, w := utf8.DecodeRuneInString(b.s[pos:])
			if w == 0 || !inst.MatchRune(c) {
				return false
			}
			pos += w
		}
		pc = int(inst.Out)
	}
}

// rxFold expands pattern's case-insensitive literals to also match
// Unicode's multi-character case folds, as Onigmo's /i does and RE2's does
// not: "ss" and "ß", "ff" and "ﬀ". Other patterns are returned unchanged.
func rxFold(pattern string) string {
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return pattern // for regexp.Compile to report
	}
	root := &syntax.Regexp{Op: syntax.OpConcat, Sub: []*syntax.Regexp{tree}}
	changed := false
	var walk func(*syntax.Regexp)
	walk = func(t *syntax.Regexp) {
		for i, sub := range t.Sub {
			if sub.Op == syntax.OpLiteral && sub.Flags&syntax.FoldCase != 0 {
				if e := rxFoldLiteral(sub); e != nil {
					t.Sub[i], changed = e, true
				}
			}
			walk(sub)
		}
	}
	walk(root)
	if !changed {
		return pattern
	}
	return root.String()
}

// rxFoldLiteral is lit, a case-insensitive literal, spelled to match every
// string whose full case folding is lit's, or nil when that is lit itself.
// It is cut where no multi-character fold spans the cut, and each piece
// lists its alternatives; a piece stops growing at 8 runes (Onigmo also
// caps its expansion) to bound the product.
func rxFoldLiteral(lit *syntax.Regexp) *syntax.Regexp {
	var f []rune // lit's runes folded, as minimal fold runes like lit's own
	for _, c := range lit.Rune {
		if m, ok := rxFoldMulti[c]; ok {
			f = append(f, []rune(strings.Map(rxMinFold, m))...)
		} else {
			f = append(f, c)
		}
	}
	out := &syntax.Regexp{Op: syntax.OpConcat, Flags: lit.Flags}
	multi := false
	for i := 0; i < len(f); {
		j := i + 1
		for p := i; p < j && j-i < 8; p++ {
			for k := 2; k <= 3 && p+k <= len(f); k++ {
				if rxUnfold[string(f[p:p+k])] != nil {
					j, multi = max(j, p+k), true
				}
			}
		}
		out.Sub = append(out.Sub, rxFoldAlts(f[i:j], lit.Flags))
		i = j
	}
	if !multi {
		return nil
	}
	return out
}

// rxFoldAlts matches the strings that fold to f (not empty): its first rune,
// or a rune whose multi-character fold is a prefix of f, then the rest.
func rxFoldAlts(f []rune, flags syntax.Flags) *syntax.Regexp {
	alt := &syntax.Regexp{Op: syntax.OpAlternate, Flags: flags}
	add := func(c rune, rest []rune) {
		e := &syntax.Regexp{Op: syntax.OpLiteral, Flags: flags, Rune: []rune{c}}
		if len(rest) > 0 {
			e = &syntax.Regexp{Op: syntax.OpConcat, Flags: flags, Sub: []*syntax.Regexp{e, rxFoldAlts(rest, flags)}}
		}
		alt.Sub = append(alt.Sub, e)
	}
	add(f[0], f[1:])
	for k := 2; k <= 3 && k <= len(f); k++ {
		for _, c := range rxUnfold[string(f[:k])] {
			add(c, f[k:])
		}
	}
	if len(alt.Sub) == 1 {
		return alt.Sub[0]
	}
	return alt
}

// rxMinFold is the least rune c simply case-folds to, as RE2's parser
// stores a case-insensitive literal's runes.
func rxMinFold(c rune) rune {
	m := c
	for d := unicode.SimpleFold(c); d != c; d = unicode.SimpleFold(d) {
		m = min(m, d)
	}
	return m
}

// rxUnfold maps a multi-character fold, as minimal fold runes, to the
// minimal fold runes that fold to it.
var rxUnfold = func() map[string][]rune {
	m := map[string][]rune{}
	for c, f := range rxFoldMulti {
		if c != rxMinFold(c) {
			continue
		}
		k := strings.Map(rxMinFold, f)
		m[k] = append(m[k], c)
	}
	for _, cs := range m {
		slices.Sort(cs)
	}
	return m
}()

// rxFoldMulti is Unicode's multi-character case folding (CaseFolding.txt,
// status F).
var rxFoldMulti = map[rune]string{
	0xdf: "ss", 0x130: "i\u0307", 0x149: "\u02bcn", 0x1f0: "j\u030c", 0x390: "\u03b9\u0308\u0301",
	0x3b0: "\u03c5\u0308\u0301", 0x587: "\u0565\u0582", 0x1e96: "h\u0331", 0x1e97: "t\u0308", 0x1e98: "w\u030a",
	0x1e99: "y\u030a", 0x1e9a: "a\u02be", 0x1e9e: "ss", 0x1f50: "\u03c5\u0313", 0x1f52: "\u03c5\u0313\u0300",
	0x1f54: "\u03c5\u0313\u0301", 0x1f56: "\u03c5\u0313\u0342", 0x1f80: "\u1f00\u03b9", 0x1f81: "\u1f01\u03b9", 0x1f82: "\u1f02\u03b9",
	0x1f83: "\u1f03\u03b9", 0x1f84: "\u1f04\u03b9", 0x1f85: "\u1f05\u03b9", 0x1f86: "\u1f06\u03b9", 0x1f87: "\u1f07\u03b9",
	0x1f88: "\u1f00\u03b9", 0x1f89: "\u1f01\u03b9", 0x1f8a: "\u1f02\u03b9", 0x1f8b: "\u1f03\u03b9", 0x1f8c: "\u1f04\u03b9",
	0x1f8d: "\u1f05\u03b9", 0x1f8e: "\u1f06\u03b9", 0x1f8f: "\u1f07\u03b9", 0x1f90: "\u1f20\u03b9", 0x1f91: "\u1f21\u03b9",
	0x1f92: "\u1f22\u03b9", 0x1f93: "\u1f23\u03b9", 0x1f94: "\u1f24\u03b9", 0x1f95: "\u1f25\u03b9", 0x1f96: "\u1f26\u03b9",
	0x1f97: "\u1f27\u03b9", 0x1f98: "\u1f20\u03b9", 0x1f99: "\u1f21\u03b9", 0x1f9a: "\u1f22\u03b9", 0x1f9b: "\u1f23\u03b9",
	0x1f9c: "\u1f24\u03b9", 0x1f9d: "\u1f25\u03b9", 0x1f9e: "\u1f26\u03b9", 0x1f9f: "\u1f27\u03b9", 0x1fa0: "\u1f60\u03b9",
	0x1fa1: "\u1f61\u03b9", 0x1fa2: "\u1f62\u03b9", 0x1fa3: "\u1f63\u03b9", 0x1fa4: "\u1f64\u03b9", 0x1fa5: "\u1f65\u03b9",
	0x1fa6: "\u1f66\u03b9", 0x1fa7: "\u1f67\u03b9", 0x1fa8: "\u1f60\u03b9", 0x1fa9: "\u1f61\u03b9", 0x1faa: "\u1f62\u03b9",
	0x1fab: "\u1f63\u03b9", 0x1fac: "\u1f64\u03b9", 0x1fad: "\u1f65\u03b9", 0x1fae: "\u1f66\u03b9", 0x1faf: "\u1f67\u03b9",
	0x1fb2: "\u1f70\u03b9", 0x1fb3: "\u03b1\u03b9", 0x1fb4: "\u03ac\u03b9", 0x1fb6: "\u03b1\u0342", 0x1fb7: "\u03b1\u0342\u03b9",
	0x1fbc: "\u03b1\u03b9", 0x1fc2: "\u1f74\u03b9", 0x1fc3: "\u03b7\u03b9", 0x1fc4: "\u03ae\u03b9", 0x1fc6: "\u03b7\u0342",
	0x1fc7: "\u03b7\u0342\u03b9", 0x1fcc: "\u03b7\u03b9", 0x1fd2: "\u03b9\u0308\u0300", 0x1fd3: "\u03b9\u0308\u0301", 0x1fd6: "\u03b9\u0342",
	0x1fd7: "\u03b9\u0308\u0342", 0x1fe2: "\u03c5\u0308\u0300", 0x1fe3: "\u03c5\u0308\u0301", 0x1fe4: "\u03c1\u0313", 0x1fe6: "\u03c5\u0342",
	0x1fe7: "\u03c5\u0308\u0342", 0x1ff2: "\u1f7c\u03b9", 0x1ff3: "\u03c9\u03b9", 0x1ff4: "\u03ce\u03b9", 0x1ff6: "\u03c9\u0342",
	0x1ff7: "\u03c9\u0342\u03b9", 0x1ffc: "\u03c9\u03b9", 0xfb00: "ff", 0xfb01: "fi", 0xfb02: "fl",
	0xfb03: "ffi", 0xfb04: "ffl", 0xfb05: "st", 0xfb06: "st", 0xfb13: "\u0574\u0576",
	0xfb14: "\u0574\u0565", 0xfb15: "\u0574\u056b", 0xfb16: "\u057e\u0576", 0xfb17: "\u0574\u056d",
}

// dropX is inline group flags without x: `x-mi` is `-mi`, and a `-` with nothing after it goes.
func dropX(flags string) string {
	on, off, _ := strings.Cut(strings.ReplaceAll(flags, "x", ""), "-")
	if off == "" {
		return on
	}
	return on + "-" + off
}

// stripExtended drops /x's insignificant whitespace and `#` comments,
// leaving escapes (`\ `, `\#`) and character classes as they are; ext is
// whether x is on outside any group. An inline group turns x off or on for
// its span, as an interpolated Regexp's `(?-mix:...)` or a union's
// `(?x-mi:...)` does, and its `x` is dropped, since RE2 has none.
func stripExtended(src string, ext bool) string {
	var b strings.Builder
	var saved []bool // ext outside each open group
	depth := 0       // inside [...], where space and # are literal
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src):
			b.WriteByte(c)
			i++
			b.WriteByte(src[i])
			continue
		case c == '[':
			depth++
		case c == ']' && depth > 0:
			depth--
		case depth > 0:
		case strings.HasPrefix(src[i:], "(?#") && strings.IndexByte(src[i:], ')') > 0: // a comment, under x or not: before # reads as one
			i += strings.IndexByte(src[i:], ')')
			continue
		case c == '(':
			next, k, on := ext, i+1, true
			if strings.HasPrefix(src[i:], "(?") {
				for k = i + 2; k < len(src) && strings.IndexByte("mixn-", src[k]) >= 0; k++ {
					switch src[k] {
					case '-':
						on = false
					case 'x':
						next = on
					}
				}
			}
			if k < len(src) && src[k] == ')' && k > i+2 { // `(?x)`: the rest of the enclosing group
				if fl := dropX(src[i+2 : k]); fl != "" {
					b.WriteString("(?" + fl + ")")
				}
				ext, i = next, k
				continue
			}
			saved = append(saved, ext)
			ext = next
			if k > i+2 && k < len(src) && src[k] == ':' { // `(?x-mi:`: the same group without x
				b.WriteString("(?" + dropX(src[i+2:k]) + ":")
				i = k
				continue
			}
		case c == ')':
			if n := len(saved); n > 0 {
				ext, saved = saved[n-1], saved[:n-1]
			}
		case !ext:
		case strings.IndexByte(" \t\n\r\f\v", c) >= 0:
			continue
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
