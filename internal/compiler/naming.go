package compiler

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Operator name table (docs/design.md decision 3): `_` + lowercase never comes out of camel-casing, so `plus`, Integer#div, IO#pos cannot meet an operator.
var opNames = map[string]string{
	"==": "Op_eq", "!=": "Op_ne", "<=>": "Op_cmp", "<": "Op_lt", "<=": "Op_le", ">": "Op_gt", ">=": "Op_ge",
	"+": "Op_plus", "-": "Op_minus", "*": "Op_mul", "/": "Op_div", "%": "Op_mod", "**": "Op_pow",
	"-@": "Op_neg", "+@": "Op_pos", "!": "Op_not", "~": "Op_inv", "<<": "Op_shl", ">>": "Op_shr",
	"&": "Op_bitAnd", "|": "Op_bitOr", "^": "Op_bitXor", "=~": "Op_eqTilde", "!~": "Op_notTilde", "===": "Op_eqq",
	"[]": "Op_idx", "[]=": "Op_idxSet", "`": "Op_backtick",
}

var suffixWords = map[string]bool{"q": true, "bang": true, "set": true}

// goMethodName maps a Ruby method name to an exported Go identifier.
// `end_with?` → EndWithQ, `upcase!` → UpcaseBang, `x=` → XSet, `__write` → __Write.
func goMethodName(name string) string {
	if n, ok := opNames[name]; ok {
		return n
	}
	if r, n := utf8.DecodeRuneInString(name); name != "" && unicode.IsUpper(r) { // Kernel#String must not meet MatchData#string
		if l := unicode.ToLower(r); unicode.ToUpper(l) == r {
			return "Cap_" + goMethodName(string(l)+name[n:])
		}
		return "Cap_" + fmt.Sprintf("_%X_", r) + goMethodName(name[n:]) // İ, K (Kelvin) lower-case to ASCII i, k
	}
	suffix := ""
	switch {
	case strings.HasSuffix(name, "?"):
		suffix = "Q"
		name = name[:len(name)-1]
	case strings.HasSuffix(name, "!"):
		suffix = "Bang"
		name = name[:len(name)-1]
	case strings.HasSuffix(name, "="):
		suffix = "Set"
		name = name[:len(name)-1]
	}
	var b strings.Builder
	// Leading underscores are kept so private helpers stay distinct.
	i := 0
	for i < len(name) && name[i] == '_' {
		b.WriteByte('_')
		i++
	}
	parts := strings.Split(name[i:], "_")
	internal := i >= 2 // the prelude's `__read_1` and `__call1` helpers are named by Go helpers and use both digit shapes, so they keep the old mapping
	for j, part := range parts {
		if part == "" {
			b.WriteByte('_')
			continue
		}
		// `empty_q` → Empty_q, so it cannot meet `empty?` → EmptyQ
		if j > 0 && j == len(parts)-1 && suffix == "" && suffixWords[part] {
			b.WriteString("_" + part)
			continue
		}
		// `a_1`/`a1` both camel-case to `A1`: a word-initial digit gets `_n` (`A_n1`); `_` + lowercase is never camel-case output.
		prevDigit := false
		for k, r := range part {
			digit := unicode.IsDigit(r)
			if digit && !prevDigit && !internal && k == 0 && j > 0 {
				b.WriteString("_n")
			}
			prevDigit = digit
			switch u := unicode.ToUpper(r); {
			case k > 0:
				b.WriteRune(r)
			case u == r || unicode.ToLower(u) == r:
				b.WriteRune(u)
			default:
				fmt.Fprintf(&b, "_%X_", r) // ı, ſ upper-case to ASCII I, S; ς and σ both to Σ
			}
		}
	}
	b.WriteString(suffix)
	return goIdent(b.String())
}

// goIdent escapes what a Go identifier can't hold (a spec's `it "has spaces"` method): each such rune becomes _XX, its code in hex.
func goIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "_%X_", r)
	}
	return b.String()
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
	// predeclared identifiers we'd rather not shadow
	"len": true, "cap": true, "new": true, "make": true, "append": true, "copy": true,
	"panic": true, "recover": true, "print": true, "println": true, "string": true,
	"int": true, "bool": true, "any": true, "error": true, "nil": true, "true": true, "false": true, "byte": true, "rune": true,
	"main": true, "init": true, "stdout": true,
	// the generated &block parameter
	"blk": true,
}

// goLocalName maps a Ruby local/param name to a Go identifier. Locals the
// compiler introduces (temporaries `t1_`, recovered panics `r_`, the named
// result `ret_`, the rest parameter `rest_`) end in a single `_`, so a Ruby
// name ending in `_` gets another and can never meet one.
func goLocalName(name string) string {
	if strings.HasPrefix(name, "@") { // main's ivar as a local (decision 362)
		return "iv_" + name[1:] + "_m_" // ends in one `_`, the compiler-local shape no Ruby name can reach
	}
	if name == lastLineLocal { // $_ as a frame's local (decision 446)
		return "lastLine_"
	}
	if goKeywords[name] || name != "_" && strings.HasSuffix(name, "_") {
		return name + "_"
	}
	return name
}

// goFieldName maps an ivar (`@x`) to a struct field name.
func goFieldName(ivar string) string {
	name := strings.TrimPrefix(ivar, "@")
	if name != "" && name[0] >= 'A' && name[0] <= 'Z' { // `@Value` (concurrent-ruby's style) as a field would collide with a Value method
		return "iv_" + name
	}
	return goLocalName(name)
}

// goFuncName maps a top-level Ruby def to a Go function name.
func goFuncName(name string) string {
	return "rb_" + goMethodName(name)
}

// goParamName is a method parameter's Go name: `def cause(_)`'s ignored `_` gets a real one, since a forwarder passes
// its own parameter on and Go cannot read `_`.
func goParamName(name string, i int) string {
	if name == "_" {
		return fmt.Sprintf("rbU%d_", i)
	}
	return goLocalName(name)
}
