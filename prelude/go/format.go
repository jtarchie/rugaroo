//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// rbFormat is Kernel#format: each directive is MRI's, handed to fmt with
// the argument converted as MRI converts it. %<name>… and %{name} read
// a Hash argument. The directive parser follows MRI's sprintf.c (posarg:
// 0 none yet, >0 the unnumbered count, -1 numbered, -2 named), so its
// ArgumentErrors are MRI's.
func rbFormat(format string, args []any) string {
	var b strings.Builder
	posarg, nextarg := 0, 0
	raise := func(msg string) { panic(NewArgumentError(Ref(String(msg)))) }
	nextArg := func() any {
		switch posarg {
		case -1:
			raise("unnumbered(" + strconv.Itoa(nextarg+1) + ") mixed with numbered")
		case -2:
			raise("unnumbered(" + strconv.Itoa(nextarg+1) + ") mixed with named")
		}
		nextarg++
		posarg = nextarg
		if nextarg > len(args) {
			raise("too few arguments")
		}
		return rbUnbox(args[nextarg-1])
	}
	posArg := func(n int) any {
		switch {
		case posarg > 0:
			raise("numbered(" + strconv.Itoa(n) + ") after unnumbered(" + strconv.Itoa(posarg) + ")")
		case posarg == -2:
			raise("numbered(" + strconv.Itoa(n) + ") after named")
		case n < 1:
			raise("invalid index - " + strconv.Itoa(n) + "$")
		}
		posarg = -1
		if n > len(args) {
			raise("too few arguments")
		}
		return rbUnbox(args[n-1])
	}
	named := func(name string) any {
		switch {
		case posarg > 0:
			raise("named" + name + " after unnumbered(" + strconv.Itoa(posarg) + ")")
		case posarg == -1:
			raise("named" + name + " after numbered")
		}
		posarg = -2
		if len(args) != 1 {
			raise("one hash required")
		}
		h, ok := rbUnbox(args[0]).(Hash_Any)
		if !ok {
			raise("one hash required")
		}
		key := Symbol(name[1 : len(name)-1])
		v, ok := h._ToAny().rbGet(key)
		if !ok {
			e := NewKeyError(Ref(String("key" + name + " not found")))
			e.receiver, e.key, e.has_key = args[0], key, true
			panic(e)
		}
		return rbUnbox(v)
	}
	// num is MRI's GETNUM: the digits at format[i:] and the index after them; running off the end is malformed.
	num := func(i int, what string) (int, int) {
		n := 0
		for ; i < len(format) && format[i] >= '0' && format[i] <= '9'; i++ {
			n = n*10 + int(format[i]-'0')
			if n > math.MaxInt32 {
				raise(what + " too big")
			}
		}
		if i >= len(format) {
			raise("malformed format string - %*[0-9]")
		}
		return n, i
	}
	toInt := func(v any) int {
		var w Integer
		switch x := v.(type) {
		case Integer:
			w = x
		case Float:
			w = rbFloatToI(float64(x))
		default:
			panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
		}
		return rbNum2Int(w)
	}
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		i++
		if i >= len(format) {
			raise("incomplete format specifier; use %% (double %) instead")
		}
		var v any
		have, name := false, ""
		flags := ""
		width, prec := 0, 0
		hasWidth, hasPrec := false, false
		checkWidth := func() {
			if hasWidth {
				raise("width given twice")
			}
			if hasPrec {
				raise("width after precision")
			}
		}
		// aster is MRI's GETASTER at the `*` format[i]: the next argument, or argument N for `*N$`.
		aster := func(what string) int {
			n, j := num(i+1, what)
			if format[j] == '$' {
				i = j
				return toInt(posArg(n))
			}
			return toInt(nextArg())
		}
	directive:
		for ; ; i++ {
			if i >= len(format) {
				raise("malformed format string")
			}
			d := format[i]
			switch {
			case strings.IndexByte("-+ 0#", d) >= 0:
				if hasWidth {
					raise("flag after width")
				}
				if hasPrec {
					raise("flag after precision")
				}
				flags += string(d)
			case d >= '1' && d <= '9':
				n, j := num(i, "width")
				if format[j] == '$' {
					if have {
						raise("value given twice - " + strconv.Itoa(n) + "$")
					}
					v, have = posArg(n), true
					i = j
					continue
				}
				checkWidth()
				width, hasWidth = n, true
				i = j - 1
			case d == '<' || d == '{':
				closer := byte('>')
				if d == '{' {
					closer = '}'
				}
				end := strings.IndexByte(format[i:], closer)
				if end < 0 {
					raise("malformed name - unmatched parenthesis")
				}
				if name != "" {
					raise("named" + format[i:i+end+1] + " after " + name)
				}
				if have {
					raise("named" + format[i:i+end+1] + " after numbered")
				}
				name = format[i : i+end+1]
				v, have = named(name), true
				i += end
				if d == '{' {
					b.WriteString(rbFormatOne(rbFormatSpec(flags, width, hasWidth, prec, hasPrec), 's', v))
					break directive
				}
			case d == '*':
				checkWidth()
				hasWidth = true
				width = aster("width")
				if width < 0 {
					flags += "-"
					width = -width
				}
			case d == '.':
				if hasPrec {
					raise("precision given twice")
				}
				hasPrec = true
				if i+1 < len(format) && format[i+1] == '*' {
					i++
					if prec = aster("precision"); prec < 0 {
						hasPrec = false // a negative precision is ignored
					}
					continue
				}
				n, j := num(i+1, "precision")
				prec, i = n, j-1
			case d == '%':
				if flags != "" || hasWidth || hasPrec || have {
					raise("invalid format character - %")
				}
				b.WriteByte('%')
				break directive
			case strings.IndexByte("spcdiuxXobBaAfeEgG", d) >= 0:
				if !have {
					v = nextArg()
				}
				b.WriteString(rbFormatOne(rbFormatSpec(flags, width, hasWidth, prec, hasPrec), d, v))
				break directive
			case d > ' ' && d < 0x7f:
				raise("malformed format string - %" + string(d))
			default:
				raise("malformed format string")
			}
		}
	}
	if posarg >= 0 && nextarg < len(args) && (len(args) != 1 || !rbFormatIsHash(args[0])) && rbVerbose != nil && bool(*rbVerbose) {
		rbWarnLoc("too many arguments for format string")
	}
	return b.String()
}

func rbFormatIsHash(a any) bool {
	_, ok := rbUnbox(a).(Hash_Any)
	return ok
}

// rbFormatSpec is the fmt directive prefix for parsed flags, width and precision.
func rbFormatSpec(flags string, width int, hasWidth bool, prec int, hasPrec bool) string {
	spec := "%" + flags
	if hasWidth {
		spec += strconv.Itoa(width)
	}
	if hasPrec {
		spec += "." + strconv.Itoa(prec)
	}
	return spec
}

// rbFormatOne formats v for one directive; spec is its flags, width and precision.
func rbFormatOne(spec string, verb byte, v any) string {
	if s, ok := rbFormatFast(spec, verb, v); ok {
		return s
	}
	if verb == 's' || verb == 'p' || verb == 'c' {
		spec = rbFormatNoZeroFlag(spec) // the 0 flag pads numbers only: Go's fmt would zero-pad these
	}
	switch verb {
	case 's':
		return fmt.Sprintf(spec+"s", string(rbToS(v)))
	case 'p':
		return fmt.Sprintf(spec+"s", string(rbInspect(v)))
	case 'c':
		if k := strings.IndexByte(spec, '.'); k >= 0 {
			spec = spec[:k] // MRI ignores a precision on %c
		}
		var r rune
		if f, ok := v.(Float); ok {
			v = rbFloatToI(float64(f))
		}
		switch x := v.(type) {
		case Integer:
			if x > math.MaxInt32 { // a C int, as MRI's NUM2INT
				panic(NewRangeError(Ref(String("integer " + strconv.Itoa(int(x)) + " too big to convert to 'int'"))))
			}
			if x < math.MinInt32 {
				panic(NewRangeError(Ref(String("integer " + strconv.Itoa(int(x)) + " too small to convert to 'int'"))))
			}
			if x < 0 || x > utf8.MaxRune || !utf8.ValidRune(rune(x)) {
				panic(NewArgumentError(Ref(String("invalid character"))))
			}
			r = rune(x)
		case String:
			if x == "" {
				return fmt.Sprintf(spec+"s", "")
			}
			r, _ = utf8.DecodeRuneInString(string(x))
		case nil:
			panic(NewTypeError(Ref(String("no implicit conversion from nil to integer"))))
		default:
			panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
		}
		return fmt.Sprintf(spec+"c", r)
	case 'd', 'i', 'u':
		n := rbFormatInt(v)
		if z, ok := rbFormatZeroPrec(spec, n); ok {
			return z
		}
		return fmt.Sprintf(spec+"d", n)
	case 'x', 'X', 'o', 'b', 'B':
		n := rbFormatInt(v)
		if z, ok := rbFormatZeroPrec(spec, n); ok && !(verb == 'o' && strings.Contains(spec, "#")) {
			return z
		}
		if n < 0 && !strings.ContainsAny(spec, "+ ") {
			return rbFormatTwos(spec, verb, n)
		}
		if n == 0 {
			if verb == 'o' && strings.Contains(spec, "#") {
				if k := strings.IndexByte(spec, '.'); k >= 0 {
					if p, _ := strconv.Atoi(spec[k+1:]); p == 0 {
						spec = spec[:k] // %#o always shows the leading 0, even at precision 0; a larger precision is the zeros themselves
					}
				}
			}
			spec = strings.ReplaceAll(spec, "#", "") // MRI's %#x of 0 is "0", with no prefix
		}
		flags := spec[1:]
		if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
			flags = flags[:k]
		}
		if verb != 'o' && strings.Contains(flags, "#") && strings.Contains(flags, "0") && !strings.Contains(flags, "-") && !strings.Contains(spec, ".") {
			// MRI's width counts the sign and the 0x/0b prefix, and the zeros go after both; Go's width counts neither
			w, _ := strconv.Atoi(strings.TrimLeft(spec[1:], "#0+ "))
			prefix := map[byte]string{'x': "0x", 'X': "0X", 'b': "0b", 'B': "0B"}[verb]
			m := uint64(n)
			sign := ""
			switch {
			case n < 0:
				sign, m = "-", -m
			case strings.Contains(flags, "+"):
				sign = "+"
			case strings.Contains(flags, " "):
				sign = " "
			}
			body := strconv.FormatUint(m, map[byte]int{'x': 16, 'X': 16, 'b': 2, 'B': 2}[verb])
			if verb == 'X' || verb == 'B' {
				body = strings.ToUpper(body)
			}
			return sign + prefix + strings.Repeat("0", max(w-len(sign)-len(prefix)-len(body), 0)) + body
		}
		if verb == 'B' {
			return strings.ToUpper(fmt.Sprintf(spec+"b", n))
		}
		return fmt.Sprintf(spec+string(verb), n)
	case 'a', 'A':
		return rbFormatHexFloat(spec, verb, rbFormatFloat(v))
	case 'f', 'e', 'E', 'g', 'G':
		if x, ok := v.(Integer); ok && verb == 'f' && (x >= 1<<53 || x <= -1<<53 || strings.Contains(spec, "#")) {
			// MRI prints an Integer's digits exactly, not through a float64, and # forces no point on them
			bs := strings.ReplaceAll(spec, "#", "")
			flags := bs[1:]
			if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
				flags = flags[:k]
			}
			if strings.Contains(flags, "-") {
				bs = rbFormatNoZeroFlag(bs) // - beats 0
			}
			return fmt.Sprintf(bs+"f", new(big.Float).SetInt64(int64(x)))
		}
		f := rbFormatFloat(v)
		if (verb == 'g' || verb == 'G') && !strings.Contains(spec, ".") {
			spec += ".6" // C's default precision; Go's %g is shortest-exact
		}
		if math.IsInf(f, 0) || math.IsNaN(f) {
			s := "Inf"
			if math.IsNaN(f) {
				s = "NaN"
			}
			switch {
			case f < 0:
				s = "-Inf"
			case strings.Contains(spec, "+"): // NaN takes the sign flags too
				s = "+" + s
			case strings.Contains(spec, " "):
				s = " " + s
			}
			w, _ := strconv.Atoi(strings.TrimLeft(strings.SplitN(spec[1:], ".", 2)[0], "-+ 0#"))
			if strings.Contains(spec, "-") {
				return fmt.Sprintf("%-*s", w, s)
			}
			return fmt.Sprintf("%*s", w, s)
		}
		return rbFmtFloatSpec(f, verb, spec) // MRI's digits, not Go's correctly rounded ones (decision 379)
	}
	panic(NewArgumentError(Ref(String("malformed format string - %" + string(verb)))))
}

// rbFormatInt reads a %d argument as MRI: a Float truncated, a String parsed.
func rbFormatInt(v any) int {
	switch n := v.(type) {
	case Integer:
		return int(n)
	case Float:
		return int(rbFloatToI(float64(n))) // truncates, as MRI; NaN and Inf are FloatDomainError, past 64 bits a RangeError (decision 35)
	case String:
		return int(rbStrictInt(n, 0)) // Kernel#Integer's rules: "1__2" is invalid
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into Integer"))))
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(v) + " into Integer"))))
}

func rbFormatFloat(v any) float64 {
	switch n := v.(type) {
	case Integer:
		return float64(n)
	case Float:
		return float64(n)
	case String:
		return float64(rbStrictFloat(n)) // Kernel#Float's rules
	case nil:
		panic(NewTypeError(Ref(String("can't convert nil into Float"))))
	}
	panic(NewTypeError(Ref(String("can't convert " + rbClassName(v) + " into Float"))))
}

// rbFormatHexFloat is %a/%A as C prints it: Go's hex float without the exponent's leading zero, then flags and width.
func rbFormatHexFloat(spec string, verb byte, f float64) string {
	prec := -1
	flags, rest := spec[1:], ""
	if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
		flags, rest = flags[:k], flags[k:]
	}
	width := rest
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		width = rest[:dot]
		prec, _ = strconv.Atoi(rest[dot+1:])
	}
	var s string
	switch {
	case math.IsNaN(f):
		s = "NaN"
	case math.IsInf(f, 0):
		s = map[bool]string{true: "Inf", false: "-Inf"}[f > 0]
	default:
		if prec == 13 && !math.IsInf(f, 0) {
			f = rbHexFloat13(f)
		}
		s = strconv.FormatFloat(f, 'x', prec, 64)
		if p := strings.LastIndexAny(s, "+-"); p > 0 && s[p-1] == 'p' {
			exp := strings.TrimLeft(s[p+1:], "0")
			if exp == "" {
				exp = "0"
			}
			s = s[:p+1] + exp
		}
		if p := strings.IndexByte(s, 'p'); strings.Contains(flags, "#") && !strings.Contains(s, ".") {
			s = s[:p] + "." + s[p:] // # forces the point, as C's
		}
	}
	if !strings.HasPrefix(s, "-") {
		switch {
		case strings.Contains(flags, "+"):
			s = "+" + s
		case strings.Contains(flags, " "):
			s = " " + s
		}
	}
	if verb == 'A' && !math.IsNaN(f) && !math.IsInf(f, 0) {
		s = strings.ToUpper(s)
	}
	w, _ := strconv.Atoi(width)
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", w, s)
	}
	if x := strings.IndexAny(s, "xX"); x >= 0 && strings.Contains(flags, "0") && len(s) < w {
		s = s[:x+1] + strings.Repeat("0", w-len(s)) + s[x+1:] // zeros go after the 0x, as C's
	}
	return fmt.Sprintf("%*s", w, s)
}

// rbHexFloat13 is what MRI's %.13a prints for f: it rounds the 52-bit mantissa half-to-even at its last bit (BSD hdtoa's digit
// count is off by one there), so ...039 prints as ...038 though 13 digits are exact. Subnormals are left alone.
func rbHexFloat13(f float64) float64 {
	bits := math.Float64bits(f)
	exp := bits >> 52 & 0x7ff
	if exp == 0 || exp == 0x7ff {
		return f
	}
	mant := bits & (1<<52 - 1)
	if mant&1 == 1 {
		q := mant >> 1
		if q&1 == 1 {
			q++
		}
		bits = bits&^(1<<52-1) + q<<1 // a carry into the exponent is the next power of two, as it should
	}
	return math.Float64frombits(bits)
}

// rbFormatZeroPrec is C's zero at precision 0 (%.0d of 0): no digits, but the sign flag's sign stays, padded to the width; false for any other value or precision.
func rbFormatZeroPrec(spec string, n int) (string, bool) {
	dot := strings.IndexByte(spec, '.')
	if n != 0 || dot < 0 {
		return "", false
	}
	if p, _ := strconv.Atoi(spec[dot+1:]); p != 0 {
		return "", false
	}
	i := 1
	for i < dot && strings.IndexByte("-+ #0", spec[i]) >= 0 {
		i++
	}
	flags := spec[1:i]
	sign := ""
	switch {
	case strings.Contains(flags, "+"):
		sign = "+"
	case strings.Contains(flags, " "):
		sign = " "
	}
	w, _ := strconv.Atoi(spec[i:dot])
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", w, sign), true
	}
	return fmt.Sprintf("%*s", w, sign), true
}

// rbFormatTwos is MRI's %x/%o/%b of a negative number without + or space: its two's complement with ".." for the
// endless leading ones (-196 is "..f3c"); zero padding and precision fill with that digit, both counting the "..".
func rbFormatTwos(spec string, verb byte, n int) string {
	bits := map[byte]uint{'x': 4, 'X': 4, 'o': 3, 'b': 1, 'B': 1}[verb]
	digits := "0123456789abcdef"
	if verb == 'X' {
		digits = "0123456789ABCDEF"
	}
	var rev []byte
	for n != -1 {
		rev = append(rev, digits[n&(1<<bits-1)])
		n >>= bits
	}
	slices.Reverse(rev)
	top := digits[1<<bits-1]
	body := append([]byte{top}, rev...)
	flags, rest := spec[1:], ""
	if k := strings.IndexFunc(flags, func(r rune) bool { return r >= '1' && r <= '9' || r == '.' }); k >= 0 {
		flags, rest = flags[:k], flags[k:]
	}
	var width int
	prec := -1
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		width, _ = strconv.Atoi(rest[:dot])
		prec, _ = strconv.Atoi(rest[dot+1:])
	} else {
		width, _ = strconv.Atoi(rest)
	}
	prefix := ""
	if strings.Contains(flags, "#") && verb != 'o' {
		prefix = map[byte]string{'x': "0x", 'X': "0X", 'b': "0b", 'B': "0B"}[verb]
	}
	fill := 0
	switch {
	case prec >= 0:
		fill = prec - 2 - len(body)
	case strings.Contains(flags, "0") && !strings.Contains(flags, "-"):
		fill = width - len(prefix) - 2 - len(body)
	}
	if fill > 0 {
		rbCheckAlloc(fill, 1)
		body = append(bytes.Repeat([]byte{top}, fill), body...)
	}
	s := prefix + ".." + string(body)
	if strings.Contains(flags, "-") {
		return fmt.Sprintf("%-*s", width, s)
	}
	return fmt.Sprintf("%*s", width, s)
}

// rbFormatNoZeroFlag drops the 0 from spec's flags (the run of -+ #0 after the %), leaving width digits alone.
func rbFormatNoZeroFlag(spec string) string {
	i := 1
	for i < len(spec) && strings.IndexByte("-+ #0", spec[i]) >= 0 {
		i++
	}

	return spec[:1] + strings.ReplaceAll(spec[1:i], "0", "") + spec[i:]
}

// rbFmtInt is a compiled %d/%i/%u/%x/%X directive (decision 377); spec and verb are rbFormat's, for what the fast path leaves out.
func rbFmtInt(b []byte, n Integer, verb byte, width int, left, zero bool, spec string) []byte {
	base := 10
	if verb == 'x' || verb == 'X' {
		if n < 0 { // MRI's ..f two's complement
			return append(b, rbFormatOne(spec, verb, n)...)
		}
		base = 16
	}
	var tmp [24]byte
	d := strconv.AppendInt(tmp[:0], int64(n), base)
	if verb == 'X' {
		for i, c := range d {
			if c >= 'a' {
				d[i] = c - 'a' + 'A'
			}
		}
	}
	return rbFmtPad(b, d, len(d), width, left, zero)
}

// rbFmtFloat is a compiled %f directive; prec is 6 when the directive gives none.
func rbFmtFloat(b []byte, x Float, prec, width int, left, zero bool, spec string) []byte {
	f := float64(x)
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return append(b, rbFormatOne(spec, 'f', x)...)
	}
	var tmp [64]byte
	d := tmp[:0]
	if math.Signbit(f) {
		d, f = append(d, '-'), -f
	}
	d = rbFmtFixedAppend(d, f, prec, false) // MRI's digits (decision 379)
	return rbFmtPad(b, d, len(d), width, left, zero)
}

// rbFmtStr is a compiled %s directive of a String: MRI's width counts characters.
func rbFmtStr(b []byte, s String, width int, left bool) []byte {
	n := len(s)
	if width > 0 { // a multibyte String's byte length can reach the width while its character count does not
		n = utf8.RuneCountInString(string(s))
	}
	return rbFmtPad(b, s, n, width, left, false)
}

// rbFmtPad appends d (n characters) padded to width: spaces on the right for -, zeros after the sign for 0, else spaces on the left.
func rbFmtPad[T ~string | ~[]byte](b []byte, d T, n, width int, left, zero bool) []byte {
	pad := width - n
	switch {
	case pad <= 0:
		return append(b, d...)
	case left:
		b = append(b, d...)
		return rbFmtFill(b, ' ', pad)
	case zero:
		if d[0] == '-' {
			b, d = append(b, '-'), d[1:]
		}
		return append(rbFmtFill(b, '0', pad), d...)
	}
	return append(rbFmtFill(b, ' ', pad), d...)
}

func rbFmtFill(b []byte, c byte, n int) []byte {
	for range n {
		b = append(b, c)
	}
	return b
}

// rbFmtAny is a compiled directive the fast helpers do not cover: rbFormat's own path, minus the parse.
func rbFmtAny(b []byte, v any, verb byte, spec string) []byte {
	return append(b, rbFormatOne(spec, verb, rbUnbox(v))...)
}

var rbTens = [...]float64{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22}
var rbBigTens = [...]float64{1e16, 1e32, 1e64, 1e128, 1e256}

// rbDtoa is MRI's BSD__dtoa (missing/dtoa.c) for a finite d > 0 in mode 2 (ndigits significant digits, %e/%g) or 3
// (ndigits after the point, %f): the digits, in tmp where they fit, and decpt, the value being 0.digits × 10^decpt
// (no digits is 0). MRI first tries floating-point arithmetic; where the remainder is too close to one half to call,
// its copy rounds an odd last digit up and keeps an even one, which is why "%.2f" % 2.675 is 2.68 and
// "%.2f" % 8.345 is 8.34. Otherwise rounding is exact, ties to even, as strconv's (decision 379).
func rbDtoa(tmp *[32]byte, d float64, mode, ndigits int) ([]byte, int) {
	if digits, decpt, ok := rbDtoaQuick(tmp, d, mode, ndigits); ok {
		return digits, decpt
	}
	return rbDtoaExact(tmp[:0], d, mode, ndigits)
}

// rbDtoaExact is dtoa's exact path: strconv's correctly rounded digits, trailing zeros dropped.
func rbDtoaExact(dst []byte, d float64, mode, ndigits int) ([]byte, int) {
	if mode == 2 {
		s := strconv.AppendFloat(dst, d, 'e', max(ndigits, 1)-1, 64) // d.ddde±XX
		e := bytes.IndexByte(s, 'e')
		exp, neg := 0, s[e+1] == '-'
		for _, c := range s[e+2:] {
			exp = exp*10 + int(c-'0')
		}
		if neg {
			exp = -exp
		}
		digits := s[:e]
		if len(digits) > 1 { // drop the point after the first digit
			digits = append(digits[:1], digits[2:]...)
		}
		return bytes.TrimRight(digits, "0"), exp + 1
	}
	s := strconv.AppendFloat(dst, d, 'f', ndigits, 64)
	whole := len(s)
	if dot := bytes.IndexByte(s, '.'); dot >= 0 {
		whole = dot
		s = append(s[:dot], s[dot+1:]...)
	}
	lead := 0
	for lead < len(s) && s[lead] == '0' {
		lead++
	}
	return bytes.TrimRight(s[lead:], "0"), whole - lead
}

// rbDtoaQuick is dtoa's "try to get by with floating-point arithmetic" path; ok is false where dtoa falls back to its
// exact one (fast_failed), except that an undecided remainder settles there as described in rbDtoa.
func rbDtoaQuick(tmp *[32]byte, d float64, mode, ndigits int) ([]byte, int, bool) {
	bits := math.Float64bits(d)
	bexp := int(bits>>52) & 0x7ff
	if bexp == 0 { // denormal: the exact path
		return nil, 0, false
	}
	d2 := math.Float64frombits(bits&(1<<52-1) | 0x3ff<<52)
	ds := (d2-1.5)*0.289529654602168 + 0.1760912590558 + float64(bexp-1023)*0.301029995663981
	k := int(ds)
	if ds < 0 && ds != float64(k) {
		k--
	}
	kCheck := true
	if k >= 0 && k < len(rbTens) {
		if d < rbTens[k] {
			k--
		}
		kCheck = false
	}
	var ilim, ilim1 int
	if mode == 2 {
		ndigits = max(ndigits, 1)
		ilim, ilim1 = ndigits, ndigits
	} else {
		ilim = ndigits + k + 1
		ilim1 = ilim - 1
	}
	if ilim < 0 || ilim > 14 { // Quick_max
		return nil, 0, false
	}
	ieps := 2
	x := d
	if k > 0 {
		div := rbTens[k&0xf]
		j := k >> 4
		if j&0x10 != 0 {
			j &= 0x10 - 1
			x /= rbBigTens[len(rbBigTens)-1]
			ieps++
		}
		for n := 0; j != 0; j, n = j>>1, n+1 {
			if j&1 != 0 {
				ieps++
				div *= rbBigTens[n]
			}
		}
		x /= div
	} else if j1 := -k; j1 != 0 {
		x *= rbTens[j1&0xf]
		for j, n := j1>>4, 0; j != 0; j, n = j>>1, n+1 {
			if j&1 != 0 {
				ieps++
				x *= rbBigTens[n]
			}
		}
	}
	if kCheck && x < 1 && ilim > 0 {
		if ilim1 <= 0 {
			return nil, 0, false
		}
		ilim = ilim1
		k--
		x *= 10
		ieps++
	}
	eps := math.Ldexp(float64(ieps)*x+7, -52) // word0(eps) -= (P-1)*Exp_msk1
	buf := tmp[:0]
	if ilim == 0 {
		x -= 5
		switch {
		case x > eps:
			return append(buf, '1'), k + 2, true // one_digit
		case x < -eps:
			return buf, -ndigits, true // no_digits
		}
		return nil, 0, false
	}
	eps *= rbTens[ilim-1]
	for i := 1; ; i++ {
		l := int64(x)
		x -= float64(l)
		if x == 0 {
			ilim = i
		}
		buf = append(buf, byte('0'+l))
		if i == ilim {
			switch {
			case x > 0.5+eps:
				return rbDtoaBump(buf, k)
			case x < 0.5-eps:
				return bytes.TrimRight(buf, "0"), k + 1, true
			}
			if (buf[len(buf)-1]-'0')&1 != 0 { // undecided, odd: up
				return rbDtoaBump(buf, k)
			}
			// undecided, even: kept whatever the exact remainder, but where that is over a half dtoa's round-off branch
			// skips its zero trim, so %g shows them ("%g" % 609.8305 is 609.830)
			if rbDtoaExactUp(d, mode, ndigits, buf) {
				return buf, k + 1, true
			}
			return bytes.TrimRight(buf, "0"), k + 1, true
		}
		x *= 10
	}
}

// rbDtoaExactUp reports that exact rounding (strconv's, ties to even) takes d past the truncated digits trunc.
func rbDtoaExactUp(d float64, mode, ndigits int, trunc []byte) bool {
	var tmp [64]byte
	var s []byte
	if mode == 2 {
		s = strconv.AppendFloat(tmp[:0], d, 'e', len(trunc)-1, 64)
		s = s[:bytes.IndexByte(s, 'e')]
	} else {
		s = strconv.AppendFloat(tmp[:0], d, 'f', ndigits, 64)
	}
	if dot := bytes.IndexByte(s, '.'); dot >= 0 {
		s = append(s[:dot], s[dot+1:]...)
	}
	return !bytes.Equal(bytes.TrimLeft(s, "0"), bytes.TrimLeft(trunc, "0"))
}

// rbDtoaBump is dtoa's bump_up: the last digit incremented, carrying through 9s (which drop off).
func rbDtoaBump(buf []byte, k int) ([]byte, int, bool) {
	s := len(buf) - 1
	for buf[s] == '9' {
		if s == 0 {
			k++
			buf[0] = '0'
			break
		}
		s--
	}
	buf[s]++
	return buf[:s+1], k + 1, true
}

// rbFmtFloatSpec is a finite f under %f, %e, %E, %g or %G with MRI's digits (rbDtoa) and BSD_vfprintf's layout; spec
// is rbFormatSpec's ("%" flags width .prec).
func rbFmtFloatSpec(f float64, verb byte, spec string) string {
	i := 1
	for ; i < len(spec) && strings.IndexByte("-+ 0#", spec[i]) >= 0; i++ {
	}
	flags := spec[1:i]
	width := 0
	for ; i < len(spec) && spec[i] >= '0' && spec[i] <= '9'; i++ {
		width = width*10 + int(spec[i]-'0')
	}
	prec := 6
	if i < len(spec) && spec[i] == '.' {
		prec = 0
		for i++; i < len(spec) && spec[i] >= '0' && spec[i] <= '9'; i++ {
			prec = prec*10 + int(spec[i]-'0')
		}
	}
	alt := strings.Contains(flags, "#")
	var tmp [64]byte
	body := tmp[:0]
	switch {
	case math.Signbit(f):
		body, f = append(body, '-'), -f
	case strings.Contains(flags, "+"):
		body = append(body, '+')
	case strings.Contains(flags, " "):
		body = append(body, ' ')
	}
	signLen := len(body)
	switch verb {
	case 'f':
		body = rbFmtFixedAppend(body, f, prec, alt)
	case 'e', 'E':
		var dt [32]byte
		digits, decpt := []byte("0"), 1
		if f != 0 {
			digits, decpt = rbDtoa(&dt, f, 2, prec+1)
		}
		body = rbFmtExpDigits(body, digits, decpt, prec, alt, verb, false)
	default: // g, G
		p := max(prec, 1)
		var dt [32]byte
		digits, decpt := []byte("0"), 1
		if f != 0 {
			digits, decpt = rbDtoa(&dt, f, 2, p)
		}
		if x := decpt - 1; x < -4 || x >= p {
			body = rbFmtExpDigits(body, digits, decpt, p-1, alt, verb-'g'+'e', !alt)
		} else {
			body = rbFmtFixedDigits(body, digits, decpt, p-1-x, alt, !alt)
		}
	}
	if pad := width - len(body); pad > 0 {
		switch {
		case strings.Contains(flags, "-"):
			body = rbFmtFill(body, ' ', pad)
		case strings.Contains(flags, "0"):
			body = append(body[:signLen], append(bytes.Repeat([]byte{'0'}, pad), body[signLen:]...)...)
		default:
			body = append(bytes.Repeat([]byte{' '}, pad), body...)
		}
	}
	return string(body)
}

// rbFmtFixedAppend appends %.<prec>f of f >= 0.
func rbFmtFixedAppend(b []byte, f float64, prec int, alt bool) []byte {
	var dt [32]byte
	var digits []byte
	decpt := 1
	if f != 0 {
		digits, decpt = rbDtoa(&dt, f, 3, prec)
	}
	return rbFmtFixedDigits(b, digits, decpt, prec, alt, false)
}

// rbFmtFixedDigits appends 0.digits × 10^decpt with prec places; asGiven (%g without #) prints dtoa's digits as they
// are, unpadded, as BSD_vfprintf does: dtoa drops trailing zeros, except in rbDtoaExactUp's case.
func rbFmtFixedDigits(b, digits []byte, decpt, prec int, alt, asGiven bool) []byte {
	if decpt <= 0 { // 0.000ddd
		b = append(b, '0')
		zeros := min(-decpt, prec)
		fracLen := min(zeros+len(digits), prec)
		n := fracLen
		if !asGiven {
			n = prec
		}
		if n == 0 && !alt {
			return b
		}
		b = append(b, '.')
		b = rbFmtFill(b, '0', zeros)
		b = append(b, digits[:fracLen-zeros]...)
		return rbFmtFill(b, '0', n-fracLen)
	}
	whole := digits[:min(decpt, len(digits))]
	b = append(b, whole...)
	b = rbFmtFill(b, '0', decpt-len(whole))
	frac := digits[len(whole):]
	frac = frac[:min(len(frac), prec)]
	n := len(frac)
	if !asGiven {
		n = prec
	}
	if n == 0 && !alt {
		return b
	}
	b = append(b, '.')
	b = append(b, frac...)
	return rbFmtFill(b, '0', n-len(frac))
}

// rbFmtExpDigits appends 0.digits × 10^decpt as d.ddd e±XX with prec places (asGiven as for rbFmtFixedDigits).
func rbFmtExpDigits(b, digits []byte, decpt, prec int, alt bool, verb byte, asGiven bool) []byte {
	if len(digits) == 0 {
		digits = []byte("0")
	}
	b = append(b, digits[0])
	frac := digits[1:]
	n := len(frac)
	if !asGiven {
		n = max(prec, len(frac))
	}
	if n > 0 || alt {
		b = append(b, '.')
		b = append(b, frac...)
		b = rbFmtFill(b, '0', n-len(frac))
	}
	e := decpt - 1
	if digits[0] == '0' {
		e = 0
	}
	b = append(b, verb)
	if e < 0 {
		b, e = append(b, '-'), -e
	} else {
		b = append(b, '+')
	}
	if e < 10 {
		b = append(b, '0')
	}
	return strconv.AppendInt(b, int64(e), 10)
}
