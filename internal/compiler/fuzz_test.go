package compiler

import (
	"go/token"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// \p{Ll} so non-ASCII lead bytes (ç, é) reach goMethodName too.
var snakeName = regexp.MustCompile(`\A[\p{Ll}_][\p{Ll}0-9_]*[?!=]?\z`)

// internalDigit: `__` names keep `__read_1` and `__call1` as written (decision 339), so `__a_1` and `__a1` still merge. Known; the prelude depends on both.
var internalDigit = regexp.MustCompile(`\A__.*[0-9]`)

// FuzzGoMethodName: two distinct snake_case Ruby method names never share a Go name (decision 3).
func FuzzGoMethodName(f *testing.F) {
	for _, p := range [][2]string{{"empty?", "empty_q"}, {"x=", "x_set"}, {"upcase!", "upcase_bang"}, {"__write", "_write"}, {"to_s", "to_str"}, {"ça", "éa"}, {"éa", "Éa"}, {"a_1", "a1"}, {"ı", "i"}, {"ſ", "s"}} {
		f.Add(p[0], p[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if !snakeName.MatchString(a) || !snakeName.MatchString(b) || internalDigit.MatchString(a) || internalDigit.MatchString(b) {
			return
		}
		ga, gb := goMethodName(a), goMethodName(b)
		if !token.IsIdentifier(ga) || !token.IsIdentifier(gb) {
			t.Fatalf("goMethodName(%q) = %q, goMethodName(%q) = %q: not both Go identifiers", a, ga, b, gb)
		}
		if a != b && ga == gb {
			t.Fatalf("goMethodName collision: %q and %q both map to %q", a, b, ga)
		}
	})
}

// FuzzTranslateRegexp: translation never panics, and what it emits is for regexp.Compile to accept or reject, not to crash on.
func FuzzTranslateRegexp(f *testing.F) {
	for _, s := range []string{`\A\d+\z`, `(?<y>\d{4})-(?<m>\d\d)`, `[[:alpha:]&&[^aeiou]]`, `a{,3}`, `\h+\R`, `(?i)straße`, `^\s*#`, `[\w&&\D]`, `\p{Greek}`, `x(?=y)`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		out, err := translateRegexp(src)
		if err != nil {
			return
		}
		_, _ = regexp.Compile(out)
	})
}

// rxFuzzOK parses p as RE2 would and bounds the work (the backtracker's visited set is len(prog)*len(s) bits).
func rxFuzzOK(p, s string) (*regexp.Regexp, bool) {
	if len(p) > 64 || len(s) > 64 {
		return nil, false
	}
	re, err := regexp.Compile(p)
	return re, err == nil
}

var rxFuzzSeeds = [][2]string{{`\d+`, "a12é"}, {`\Aab`, "ab\xff"}, {`[a-z]+`, "日x"}, {`(?m)^(a+)(b*?)$`, "aab"}, {`(?m)\b(\w+)\b`, "foo bar"}, {`(?m)(a|ab)(c|bcd)(d*)`, "abcd"}, {`(?mi)^x(?:(y)|z)*`, "xyzY"}, {`(?ms)a.*?b|$`, "b\naab"}}

// FuzzRxFast: when rxFast takes a pattern, find/findAll/MatchString equal RE2's on any subject (decision 242's fast path must be invisible).
func FuzzRxFast(f *testing.F) {
	for _, s := range rxFuzzSeeds {
		f.Add(s[0], s[1], 2)
	}
	f.Fuzz(func(t *testing.T, p, s string, n int) {
		re, ok := rxFuzzOK(p, s)
		if !ok {
			return
		}
		r := rxNew(re)
		if r.fast == nil {
			return
		}
		if got, want := r.fast.find(s), re.FindStringSubmatchIndex(s); !slices.Equal(got, want) {
			t.Fatalf("/%s/ on %q: find %v, RE2 %v", p, s, got, want)
		}
		n = n%5 - 1
		if got, want := r.fast.findAll(s, n), re.FindAllStringSubmatchIndex(s, n); !reflect.DeepEqual(got, want) {
			t.Fatalf("/%s/ on %q n=%d: findAll %v, RE2 %v", p, s, n, got, want)
		}
		if got, want := r.MatchString(s), re.MatchString(s); got != want {
			t.Fatalf("/%s/ on %q: MatchString %v, RE2 %v", p, s, got, want)
		}
	})
}

// FuzzBacktrack: the backtracker has RE2's leftmost-first semantics, so its captures equal RE2's.
func FuzzBacktrack(f *testing.F) {
	for _, s := range rxFuzzSeeds {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, p, s string) {
		re, ok := rxFuzzOK(p, s)
		if !ok {
			return
		}
		r := rxNew(re)
		want := re.FindStringSubmatchIndex(s)
		if r.prog == nil || r.differs(s, want) { // Onigmo words and a final newline differ from RE2 by design (rxRegexp.differs)
			return
		}
		if got := r.backtrack(s); !slices.Equal(got, want) {
			t.Fatalf("/%s/ on %q: backtrack %v, RE2 %v", p, s, got, want)
		}
	})
}

var rubyLocal = regexp.MustCompile(`\A[\p{L}_][\p{L}0-9_]*\z`)

// FuzzGoLocalName: distinct Ruby locals (and main's `@ivar` locals) never share a Go name, and the name is a Go identifier that is not a keyword.
func FuzzGoLocalName(f *testing.F) {
	for _, p := range [][2]string{{"len", "len_"}, {"x_", "x__"}, {"_", "__"}, {"@a", "iv_a_m"}, {"blk", "blk_"}, {"rbU0_", "_"}} {
		f.Add(p[0], p[1])
	}
	valid := func(s string) bool {
		return rubyLocal.MatchString(strings.TrimPrefix(s, "@")) && !strings.HasPrefix(strings.TrimPrefix(s, "@"), "@")
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if !valid(a) || !valid(b) {
			return
		}
		ga, gb := goLocalName(a), goLocalName(b)
		if !token.IsIdentifier(ga) || !token.IsIdentifier(gb) || goKeywords[ga] {
			t.Fatalf("goLocalName(%q) = %q, (%q) = %q: not a plain Go identifier", a, ga, b, gb)
		}
		if a != b && ga == gb {
			t.Fatalf("goLocalName collision: %q and %q both map to %q", a, b, ga)
		}
	})
}

// FuzzParseLiteralFormat: never panics, and a directive it accepts has width and prec within the bound it checks (a digit run must not wrap past it).
func FuzzParseLiteralFormat(f *testing.F) {
	for _, s := range []string{"%05.2f", "%-+ #0x", "%%", "%*d", "%1$s", "%<a>d", "%99999999999999999999d", "%18446744073709551616d", "%9227000000000000000B", "%.", "%", "x%d%s!"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		dirs, _, ok := parseLiteralFormat(s)
		if !ok {
			return
		}
		for _, d := range dirs {
			if d.width < 0 || d.width > 4096 || d.prec < -1 || d.prec > 4096 {
				t.Fatalf("parseLiteralFormat(%q): accepted width %d prec %d", s, d.width, d.prec)
			}
		}
	})
}
