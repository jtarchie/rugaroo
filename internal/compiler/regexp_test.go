package compiler

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"
	"time"
)

// compileRuby mirrors genRegexp's static path and rbRegexpNew: translate,
// then compile under the flag prefix.
func compileRuby(src string, f rubyRegexFlags) (*rxRegexp, error) {
	pat, err := translateRegexp(src)
	if err != nil {
		return nil, err
	}
	re, err := rxCompile(rxFold(f.goPrefix() + pat))
	if err != nil {
		return nil, fmt.Errorf("RE2: %w", err)
	}
	return re, nil
}

func TestRegexpFlags(t *testing.T) {
	cases := []struct {
		name         string
		f            rubyRegexFlags
		prefix, opts string
	}{
		{"none", rubyRegexFlags{}, "(?m)", ""},
		{"i", rubyRegexFlags{ignoreCase: true}, "(?mi)", "i"},
		{"m", rubyRegexFlags{multiline: true}, "(?ms)", "m"},
		// MRI prints /a/im as /a/mi
		{"mi", rubyRegexFlags{ignoreCase: true, multiline: true}, "(?mis)", "mi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.f.goPrefix(); got != c.prefix {
				t.Errorf("goPrefix() = %q, want %q", got, c.prefix)
			}
			if got := c.f.opts(); got != c.opts {
				t.Errorf("opts() = %q, want %q", got, c.opts)
			}
		})
	}
}

func TestTranslateRegexp(t *testing.T) {
	cases := []struct{ in, want string }{
		{`abc`, `abc`},
		{`\h`, `[0-9a-fA-F]`},
		{`\h+`, `[0-9a-fA-F]+`},
		{`[\h_]`, `[0-9a-fA-F_]`},
		{`[^\h]`, `[^0-9a-fA-F]`},
		{`\H`, `[^0-9a-fA-F]`},
		{`[a][\h]\h`, `[a][0-9a-fA-F][0-9a-fA-F]`},
		{`[[:alpha:]\h]`, "[" + rxPosix["alpha"] + "0-9a-fA-F]"},
		{`[\]h]`, `[\]h]`},
		{`\\h`, `\\h`},
		{`\d\w\s\b\A\z`, `\d\w[\t\n\v\f\r ]\b\A\z`},
		{`a\/b`, `a\/b`},
		{`trailing\`, `trailing\`},
		{`(?<y>\d+)`, `(?<y>\d+)`},
		{`a]`, `a]`},
		{`[a]\H`, `[a][^0-9a-fA-F]`},
		{`[^\]]\h`, `[^\]][0-9a-fA-F]`},
		{`\\\h`, `\\[0-9a-fA-F]`},
		{`é\h`, `é[0-9a-fA-F]`},
		{``, ``},
		{`(?m:a)(?-m)(?mi)`, `(?s:a)(?-s)(?si)`},
		// Regexp#to_s embedded by interpolation: x off is dropped, x on left for RE2.
		{`(?i-mx:a)(?-x)(?mix:b)`, `(?i-s:a)(?)(?six:b)`},
		{`a{,2}b{2}?(cd){3}+e{1,2}?`, `a{0,2}(?:b{2})?(?:(cd){3})+e{1,2}?`},
		{`(?<a>x)(y)`, `(?<a>x)(?:y)`},
		{`\u0062\u{61 62}\e`, `\x{0062}\x{61}\x{62}\x1b`},
		{`[a-z&&[^b-y]]`, `[az]`},
		{`[a[^\x00-y]]`, `[az-\x{10ffff}]`},
		{`[\b]`, `[\x08]`}, // backspace in a class; \b outside is a word boundary
		{`a\b[x\by]`, `a\b[x\x08y]`},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := translateRegexp(c.in)
			if err != nil {
				t.Fatalf("translateRegexp(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("translateRegexp(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Expectations are MRI 4.0's `!!(re =~ s)`.
func TestRegexpMatchesMRI(t *testing.T) {
	m := rubyRegexFlags{multiline: true}
	i := rubyRegexFlags{ignoreCase: true}
	cases := []struct {
		name  string
		src   string
		f     rubyRegexFlags
		in    string
		want  bool
		known string
	}{
		{"caret is line anchor", `^b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"dollar is line anchor", `a$`, rubyRegexFlags{}, "a\nb", true, ""},
		{"dollar before final newline", `abc$`, rubyRegexFlags{}, "abc\n", true, ""},
		{"dot skips newline", `a.b`, rubyRegexFlags{}, "a\nb", false, ""},
		{"m_flag_dot_matches_newline", `a.b`, m, "a\nb", true, ""},
		{"i_flag", `abc`, i, "ABC", true, ""},
		{"inline_i", `(?i)abc`, rubyRegexFlags{}, "ABC", true, ""},
		{"hex_escape", `\A\h+\z`, rubyRegexFlags{}, "09afAF", true, ""},
		{"hex_escape_non_hex", `\A\h+\z`, rubyRegexFlags{}, "g", false, ""},
		{"non_hex_escape", `\A\H\z`, rubyRegexFlags{}, "g", true, ""},
		{"non_hex_escape_hex", `\A\H\z`, rubyRegexFlags{}, "a", false, ""},
		{"hex_escape_in_class", `\A[\h_]+\z`, rubyRegexFlags{}, "a_F9", true, ""},
		{"hex_escape_in_negated_class", `\A[^\h]\z`, rubyRegexFlags{}, "a", false, ""},
		{"escaped backslash then h", `\\h`, rubyRegexFlags{}, `\h`, true, ""},
		{"escaped slash", `a\/b`, rubyRegexFlags{}, "a/b", true, ""},
		{"named group", `(?<y>\d+)-(?<m>\d+)`, rubyRegexFlags{}, "2024-05", true, ""},
		{"z_is_absolute_end", `\Aab\z`, rubyRegexFlags{}, "ab\n", false, ""},
		{"posix class", `[[:alpha:]]+`, rubyRegexFlags{}, "abc", true, ""},
		{"posix_class_plus_hex_escape", `\A[[:xdigit:]\h]+\z`, rubyRegexFlags{}, "fF0", true, ""},
		{"escaped_bracket_in_class", `\A[\]h]+\z`, rubyRegexFlags{}, "]h", true, ""},
		{"dot is one rune", `\A.\z`, rubyRegexFlags{}, "é", true, ""},
		{"word_is_ascii", `\A\w\z`, rubyRegexFlags{}, "é", false, ""},
		{"digit_is_ascii", `x\d`, rubyRegexFlags{}, "x٣", false, ""},
		{"rbracket_outside_class", `a]`, rubyRegexFlags{}, "a]", true, ""},
		{"i_flag_unicode", `é`, i, "É", true, ""},
		{"hex_escape_after_class", `\A[a]\h\z`, rubyRegexFlags{}, "a9", true, ""},
		{"non_hex_escape_after_negated_class", `\A[^a]\H\z`, rubyRegexFlags{}, "bg", true, ""},
		{"escaped_backslash_then_hex_escape", `\A\\\h\z`, rubyRegexFlags{}, `\f`, true, ""},
		{"inline_i_minus_m_group", `(?i-m:a.b)`, rubyRegexFlags{}, "A\nB", false, ""},
		// Ruby's inline m is dot-all; RE2's is multi-line anchors, which (?m) already turns on.
		{"inline_m_is_dotall", `(?m)a.b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"group_m_is_dotall", `(?m:a.b)`, rubyRegexFlags{}, "a\nb", true, ""},
		{"inline_mi_is_dotall", `(?mi)a.b`, rubyRegexFlags{}, "A\nB", true, ""},
		{"inline_minus_m_keeps_line_anchors", `(?-m)^b`, rubyRegexFlags{}, "a\nb", true, ""},
		{"space_matches_vertical_tab", `\A\s\z`, rubyRegexFlags{}, "\v", true, ""},
		{"non_space_rejects_vertical_tab", `\A\S\z`, rubyRegexFlags{}, "\v", false, ""},
		{"non_space_in_negated_class", `\A[^\S\n]\z`, rubyRegexFlags{}, "\v", true, ""},
		{"posix_negated_space", `x[[:^space:]]`, rubyRegexFlags{}, "x\u3000", false, ""},
		{"class_intersection", `\A[a-z&&[^aeiou]]\z`, rubyRegexFlags{}, "b", true, ""},
		{"nested_class", `\A[a[bc]]\z`, rubyRegexFlags{}, "b", true, ""},
		{"posix_alpha_is_unicode", `\A[[:alpha:]]\z`, rubyRegexFlags{}, "é", true, ""},
		{"posix_upper_is_unicode", `\A[[:upper:]]\z`, rubyRegexFlags{}, "É", true, ""},
		{"posix_digit_is_unicode", `\A[[:digit:]]\z`, rubyRegexFlags{}, "٣", true, ""},
		{"posix_space_is_unicode", `\A[[:space:]]\z`, rubyRegexFlags{}, "\u00a0", true, ""},
		{"posix_word_is_unicode", `\A[[:word:]]\z`, rubyRegexFlags{}, "é", true, ""},
		{"word_boundary_is_unicode", `\bcafé\b`, rubyRegexFlags{}, "un café noir", true, ""},
		{"caret_not_after_final_newline", `\n^`, rubyRegexFlags{}, "a\n", false, ""},
		{"caret_before_final_newline", `^$`, rubyRegexFlags{}, "a\n\n", true, ""},
		{"word_boundary_beside_non_ascii", `caf\b`, rubyRegexFlags{}, "café", false, ""},
		{"i_flag_multichar_fold", `STRASSE`, i, "straße", true, ""},
		{"i_flag_multichar_fold_in_pattern", `ß`, i, "SS", true, ""},
		{"i_flag_multichar_fold_overlap", `sss`, i, "sß", true, ""},
		{"i_flag_multichar_fold_stays_in_literal", `s+`, i, "ß", false, ""},
		{"multichar_fold_needs_i", `ss`, rubyRegexFlags{}, "ß", false, ""},
		{"open_interval_is_zero_to_n", `\Aa{,3}\z`, rubyRegexFlags{}, "aa", true, ""},
		{"fixed_interval_then_question_is_optional", `\Aa{2}?\z`, rubyRegexFlags{}, "", true, ""},
		{"Q_is_literal", `\Q.`, rubyRegexFlags{}, "Qx", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.known != "" {
				known(t, c.known)
			}
			re, err := compileRuby(c.src, c.f)
			if err != nil {
				t.Fatalf("/%s/%s rejected: %v", c.src, c.f.opts(), err)
			}
			if got := re.MatchString(c.in); got != c.want {
				t.Errorf("/%s/%s =~ %q: got %v, MRI gives %v", c.src, c.f.opts(), c.in, got, c.want)
			}
		})
	}
}

// Where Onigmo's ^ and \b agree with RE2's (ASCII, no final newline), the
// backtracker must find RE2's match, captures included.
func TestBacktrackMatchesRE2(t *testing.T) {
	pats := []string{`(?m)^(a+)(b*?)$`, `(?m)\b(\w+)\b`, `(?m)(a|ab)(c|bcd)(d*)`, `(?mi)^x(?:(y)|z)*`, `(?m)(?P<n>\d+)?-\B`, `(?ms)a.*?b|$`}
	subjects := []string{"", "aab", "abcd", "xyzY", "foo bar", "12-x", "-", "a\nb", "b\naab"}
	for _, p := range pats {
		tree, _ := syntax.Parse(p, syntax.Perl)
		prog, _ := syntax.Compile(tree.Simplify())
		r := &rxRegexp{Regexp: regexp.MustCompile(p), prog: prog}
		for _, s := range subjects {
			if got, want := r.backtrack(s), r.Regexp.FindStringSubmatchIndex(s); !slices.Equal(got, want) {
				t.Errorf("/%s/ on %q: backtrack %v, RE2 %v", p, s, got, want)
			}
		}
	}
}

// Decision 242: what RE2 rejects and regexp2 matches as Onigmo does compiles, on the rxAlt engine.
func TestRegexpAlt(t *testing.T) {
	for _, src := range []string{`a(?=b)`, `a(?!b)`, `(?<=a)b`, `(?<!a)b`, `(a)\1`, `(?<x>a)\k<x>`, `(?<x>a)\k'x'`, `(?>a)`, `(?x) a b`, `b\Z`} {
		re, err := compileRuby(src, rubyRegexFlags{})
		if err != nil || re.alt == nil {
			t.Errorf("/%s/: err %v, alt %v; want regexp2", src, err, re != nil && re.alt != nil)
		}
	}
}

// A timeout is the RxTimeout panic (a Regexp::TimeoutError), not a short result; MRI memoizes and never times out, so no MRI comparison.
func TestRegexpAltTimeout(t *testing.T) {
	saved := rxMatchTimeout
	t.Cleanup(func() { rxMatchTimeout = saved })
	rxMatchTimeout = 20 * time.Millisecond
	re, err := compileRuby(`^(?!x)(a+)+$`, rubyRegexFlags{}) // the lookahead routes it to regexp2; (a+)+ backtracks exponentially on a miss
	if err != nil || re.alt == nil {
		t.Fatalf("err %v, alt %v; want regexp2", err, re != nil && re.alt != nil)
	}
	slow := strings.Repeat("a", 40) + "b"
	panics := func(f func()) (v any) {
		defer func() { v = recover() }()
		f()
		return nil
	}
	for name, f := range map[string]func(){
		"MatchString":   func() { re.MatchString(slow) },
		"FindAll":       func() { re.FindAllStringSubmatchIndex(slow, -1) },
		"FindAllIndex":  func() { re.FindAllStringIndex(slow, -1) },
		"SubmatchIndex": func() { re.FindStringSubmatchIndex(slow) },
	} {
		if _, ok := panics(f).(RxTimeout); !ok {
			t.Errorf("%s: want an RxTimeout panic", name)
		}
	}
	if !re.MatchString("aaa") || re.MatchString("aab") {
		t.Error("a match inside the limit must still answer")
	}
}

// Decision 24: constructs neither engine matches as Onigmo does are rejected at transpile time.
func TestRegexpRejected(t *testing.T) {
	cases := []struct{ name, src, errSub string }{
		{"Z_in_class", `[\Z]`, `\Z inside a character class`},
		{"H_in_class", `[\H]`, `\H inside a character class`},
		{"H_in_negated_class", `[^\H]`, `\H inside a character class`},
		{"subexpression call", `(a)\g<1>`, `\g (a subexpression call)`},
		{"G_anchor", `a\Gb`, `\G is not supported by RE2`}, // a leading \G is \A (decision 444)
		{"K_keep", `a\Kb`, `\K is not supported by RE2`},
		{"R_linebreak_in_class", `[\R]`, `\R inside a character class`}, // \R itself is a linebreak (decision 444)
		{"X_cluster", `\X`, `\X is not supported by RE2`},
		{"possessive", `a++`, ""},
		{"absent operator", `(?~abc)`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := compileRuby(c.src, rubyRegexFlags{})
			if err == nil {
				t.Fatalf("/%s/ compiled; want it rejected", c.src)
			}
			if c.errSub != "" && !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("/%s/: error %q does not mention %q", c.src, err, c.errSub)
			}
		})
	}
}
