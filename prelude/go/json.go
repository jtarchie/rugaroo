//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type I_ToJson interface{ ToJson(...any) String }

// rbToJson is `a.to_json(*args)` on any value. The generator passes its
// state as the one argument, as the gem does, so a user to_json with a
// signature other than (*untyped) is reached through its Dyn wrapper.
func rbToJson(a any, args ...any) String {
	a = rbUnbox(a)
	if a == nil {
		return "null"
	}
	if j, ok := a.(I_ToJson); ok {
		return j.ToJson(args...)
	}
	if j, ok := a.(interface{ DynToJson(...any) any }); ok {
		r := j.DynToJson(args...)
		if s, ok := r.(String); ok {
			return s
		}
		panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(r) + " (expected String)"))))
	}
	return rbToS(a).ToJson(args...)
}

// rbJSONHardNesting bounds the recursion where max_nesting is off (false, 0) or higher: a Go stack overflow is fatal and no rescue catches it (decision 25).
const rbJSONHardNesting = 10000

// rbJSONMaxNesting reads a max_nesting: option as the gem does: nil and false are 0 (off), a Float truncates, any other non-Integer is a TypeError.
func rbJSONMaxNesting(v any) int {
	switch n := rbUnbox(v).(type) {
	case nil:
		return 0
	case Boolean:
		if !bool(n) {
			return 0
		}
		panic(NewTypeError(Ref(String("no implicit conversion of true into Integer"))))
	case Integer:
		return int(n)
	case Float:
		return int(n)
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Integer"))))
}

// rbJSONLimit is the depth no container may pass: max, or the hard cap when max is off or above it.
func rbJSONLimit(max int) int {
	if max == 0 || max > rbJSONHardNesting {
		return rbJSONHardNesting
	}
	return max
}

func rbJSONNesting(depth int, tail string) any {
	return NewJSON_NestingError(Ref(String("nesting of " + strconv.Itoa(depth) + " is too deep" + tail)))
}

// rbJSONParse walks the token stream itself, not json.Unmarshal into a map, so object keys keep insertion order (rugaroo's Hash). opts is JSON.parse/.load's: symbolize_names, and max_nesting (100 unless given).
func rbJSONParse(s string, opts *Hash[Symbol, any]) any {
	symbolizeNames, freeze, limit := false, false, 100
	if opts != nil {
		v, ok := opts.rbGet(Symbol("symbolize_names"))
		symbolizeNames = ok && rbTruthy(v)
		fv, _ := opts.rbGet(Symbol("freeze"))
		freeze = rbTruthy(fv)
		if m, ok := opts.rbGet(Symbol("max_nesting")); ok {
			limit = rbJSONMaxNesting(m)
		}
		for k, hv := range rbHashPairs(opts) { // the generator's rule (decision 25): what is not implemented raises, other keys are ignored
			switch k {
			case Symbol("allow_nan"), Symbol("object_class"), Symbol("array_class"), Symbol("decimal_class"), Symbol("create_additions"), Symbol("match_string"):
				if rbTruthy(hv) {
					panic(NewNotImplementedError(Ref(String("JSON parser option " + string(k) + " is not supported"))))
				}
			}
		}
	}
	limit = rbJSONLimit(limit)
	s = rbJSONPrep(s)
	var (
		fast any
		ok   bool
	)
	if symbolizeNames {
		fast, ok = rbJSONFastParse(s, limit, func(k string) Symbol { return Symbol(k) })
	} else {
		fast, ok = rbJSONFastParse(s, limit, func(k string) String { return String(k) })
	}
	if ok {
		if freeze {
			rbJSONFreeze(fast)
		}
		return fast
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()

	var (
		rv  rbJSONVal
		err error
	)
	if symbolizeNames {
		rv, err = rbJSONDecodeValue(dec, 0, limit, func(k string) Symbol { return Symbol(k) })
	} else {
		rv, err = rbJSONDecodeValue(dec, 0, limit, func(k string) String { return String(k) })
	}
	if err != nil {
		panic(NewJSON_ParserError(Ref(String(rbJSONDecodeErr(err, s)))))
	}
	if rest := rbJSONSkipSpace(s, int(dec.InputOffset())); rest < len(s) {
		panic(NewJSON_ParserError(Ref(String("unexpected token at end of stream '" + rbJSONSnippet(s, rest) + "'" + rbJSONAt(s, rest)))))
	}
	if freeze {
		rbJSONFreeze(rv.v)
	}
	return rv.v
}

// rbJSONPrep is the json gem's two parse-time rules Go's decoder lacks: `//` and `/* */` comments are whitespace (blanked here, same
// length with newlines kept, so offsets and line numbers in messages stay), and a high-surrogate \u escape needs a low one after it.
func rbJSONPrep(s string) string {
	if strings.IndexByte(s, '/') < 0 && !strings.Contains(s, "\\u") {
		return s
	}
	var b []byte // a copy, made at the first comment
	blank := func(from, to int) {
		if b == nil {
			b = []byte(s)
		}
		for k := from; k < to; k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	hex4 := func(at int) (int, bool) {
		if at+4 > len(s) {
			return 0, false
		}
		n, err := strconv.ParseUint(s[at:at+4], 16, 32)
		return int(n), err == nil
	}
	inStr := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case inStr && c == '\\' && i+1 < len(s):
			if s[i+1] == 'u' {
				if hi, ok := hex4(i + 2); ok && hi >= 0xd800 && hi < 0xdc00 {
					lo, ok := hex4(i + 8)
					if !strings.HasPrefix(s[i+6:], "\\u") || !ok || lo < 0xdc00 || lo > 0xdfff {
						panic(NewJSON_ParserError(Ref(String("incomplete surrogate pair at '" + rbJSONSnippet(s, i) + "'" + rbJSONAt(s, i)))))
					}
				}
			}
			i++
		case c == '"':
			inStr = !inStr
		case !inStr && c == '/' && i+1 < len(s) && s[i+1] == '/':
			j := i
			for j < len(s) && s[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j - 1
		case !inStr && c == '/' && i+1 < len(s) && s[i+1] == '*':
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				panic(NewJSON_ParserError(Ref(String("unterminated comment, expected closing '*/'" + rbJSONAt(s, i)))))
			}
			blank(i, i+2+j+2)
			i += 2 + j + 1
		}
	}
	if b != nil {
		return string(b)
	}
	return s
}

// rbJSONFast parses well-formed JSON a byte at a time, several times faster than json.Decoder's Token stream. It gives
// up (ok false) on anything else: a syntax error, a container past the nesting limit, a surrogate \u escape or invalid
// UTF-8; the Decoder path then reparses, so errors and their messages stay that path's (decision 332).
type rbJSONFast[K comparable] struct {
	s     string
	i     int
	limit int
	keyOf func(string) K
}

func rbJSONFastParse[K comparable](s string, limit int, keyOf func(string) K) (any, bool) {
	p := &rbJSONFast[K]{s: s, limit: limit, keyOf: keyOf}
	v, ok := p.value(0)
	if !ok || rbJSONSkipSpace(s, p.i) != len(s) {
		return nil, false
	}
	return v, true
}

func (p *rbJSONFast[K]) value(depth int) (any, bool) {
	p.i = rbJSONSkipSpace(p.s, p.i)
	if p.i >= len(p.s) {
		return nil, false
	}
	switch c := p.s[p.i]; {
	case c == '{':
		p.i++
		h := NewHash[K, any]()
		if p.i = rbJSONSkipSpace(p.s, p.i); p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return h, true
		}
		if depth >= p.limit {
			return nil, false
		}
		for {
			if p.i = rbJSONSkipSpace(p.s, p.i); p.i >= len(p.s) || p.s[p.i] != '"' {
				return nil, false
			}
			k, ok := p.str()
			if !ok {
				return nil, false
			}
			if p.i = rbJSONSkipSpace(p.s, p.i); p.i >= len(p.s) || p.s[p.i] != ':' {
				return nil, false
			}
			p.i++
			v, ok := p.value(depth + 1)
			if !ok {
				return nil, false
			}
			Hash_Op_idxSet(h, p.keyOf(k), v)
			if p.i = rbJSONSkipSpace(p.s, p.i); p.i >= len(p.s) {
				return nil, false
			}
			p.i++
			switch p.s[p.i-1] {
			case '}':
				return h, true
			case ',':
			default:
				return nil, false
			}
		}
	case c == '[':
		p.i++
		arr := &Array[any]{}
		if p.i = rbJSONSkipSpace(p.s, p.i); p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			return arr, true
		}
		if depth >= p.limit {
			return nil, false
		}
		for {
			v, ok := p.value(depth + 1)
			if !ok {
				return nil, false
			}
			arr.s = append(arr.s, v)
			if p.i = rbJSONSkipSpace(p.s, p.i); p.i >= len(p.s) {
				return nil, false
			}
			p.i++
			switch p.s[p.i-1] {
			case ']':
				return arr, true
			case ',':
			default:
				return nil, false
			}
		}
	case c == '"':
		v, ok := p.str()
		return String(v), ok
	case c == '-' || c >= '0' && c <= '9':
		return p.num()
	}
	for _, lit := range [...]struct {
		word string
		v    any
	}{{"true", Boolean(true)}, {"false", Boolean(false)}, {"null", nil}} {
		if strings.HasPrefix(p.s[p.i:], lit.word) {
			p.i += len(lit.word)
			return lit.v, true
		}
	}
	return nil, false
}

// str reads a string at p.i (its opening quote).
func (p *rbJSONFast[K]) str() (string, bool) {
	start := p.i + 1
	for j := start; j < len(p.s); j++ {
		switch c := p.s[j]; {
		case c == '"':
			p.i = j + 1
			return p.s[start:j], true
		case c == '\\':
			return p.escaped(start)
		case c < 0x20:
			return "", false
		case c >= utf8.RuneSelf:
			r, n := utf8.DecodeRuneInString(p.s[j:])
			if r == utf8.RuneError && n == 1 {
				return "", false
			}
			j += n - 1
		}
	}
	return "", false
}

// escaped is str for a string with a backslash in it.
func (p *rbJSONFast[K]) escaped(start int) (string, bool) {
	var b strings.Builder
	for j := start; j < len(p.s); j++ {
		c := p.s[j]
		switch {
		case c == '"':
			p.i = j + 1
			return b.String(), true
		case c < 0x20:
			return "", false
		case c >= utf8.RuneSelf:
			r, n := utf8.DecodeRuneInString(p.s[j:])
			if r == utf8.RuneError && n == 1 {
				return "", false
			}
			b.WriteString(p.s[j : j+n])
			j += n - 1
		case c != '\\':
			b.WriteByte(c)
		case j+1 >= len(p.s):
			return "", false
		default:
			j++
			switch p.s[j] {
			case '"', '\\', '/':
				b.WriteByte(p.s[j])
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if j+4 >= len(p.s) {
					return "", false
				}
				r, err := strconv.ParseUint(p.s[j+1:j+5], 16, 32)
				if err != nil || utf16.IsSurrogate(rune(r)) {
					return "", false
				}
				b.WriteRune(rune(r))
				j += 4
			default:
				return "", false
			}
		}
	}
	return "", false
}

// num reads a number at p.i by JSON's grammar: -?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?
func (p *rbJSONFast[K]) num() (any, bool) {
	s, j := p.s, p.i
	digits := func() bool {
		k := j
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		return j > k
	}
	if s[j] == '-' {
		j++
	}
	if j < len(s) && s[j] == '0' {
		j++
	} else if !digits() {
		return nil, false
	}
	if j < len(s) && s[j] == '.' {
		j++
		if !digits() {
			return nil, false
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		j++
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if !digits() {
			return nil, false
		}
	}
	if j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
		return nil, false // a leading zero, as 01
	}
	v := rbJSONParseNumber(s[p.i:j])
	p.i = j
	return v, true
}

// rbJSONFreeze freezes every Array and Hash of a parsed document (freeze: true); Strings are values, always frozen.
func rbJSONFreeze(v any) {
	switch x := v.(type) {
	case *Array[any]:
		x.frozen.Store(true)
		for _, e := range x.s {
			rbJSONFreeze(e)
		}
	case *Hash[String, any]:
		x.frozen.Store(true)
		for _, e := range rbHashPairs(x) {
			rbJSONFreeze(e)
		}
	case *Hash[Symbol, any]:
		x.frozen.Store(true)
		for _, e := range rbHashPairs(x) {
			rbJSONFreeze(e)
		}
	}
}

// rbJSONSkipSpace is the index of the first non-whitespace byte of s at or after i.
func rbJSONSkipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

// rbJSONSnippet is the input from i to the next whitespace, at most 32 bytes: what the json gem quotes in its messages.
func rbJSONSnippet(s string, i int) string {
	end := i
	for end < len(s) && end-i < 32 && !(s[end] == ' ' || s[end] == '\t' || s[end] == '\n' || s[end] == '\r') {
		end++
	}
	return s[i:end]
}

// rbJSONAt is the json gem's " at line L column C" for the byte at i.
func rbJSONAt(s string, i int) string {
	if i > len(s) {
		i = len(s)
	}
	line := 1 + strings.Count(s[:i], "\n")
	col := i - strings.LastIndexByte(s[:i], '\n')
	return fmt.Sprintf(" at line %d column %d", line, col)
}

// rbJSONContext scans s up to i: the containers still open, the last
// significant byte, whether i is inside a string, and where a bare word
// (a literal like `nul`) started, or -1.
func rbJSONContext(s string, upto int) (stack []byte, lastSig byte, inString bool, litStart int) {
	escaped := false
	litStart = -1
	for i := 0; i < upto && i < len(s); i++ {
		c := s[i]
		switch {
		case inString:
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
		case c == '"':
			inString, lastSig, litStart = true, c, -1
		case c == '{' || c == '[':
			stack, lastSig, litStart = append(stack, c), c, -1
		case c == '}' || c == ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			lastSig, litStart = c, -1
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			litStart = -1
		case c >= 'a' && c <= 'z':
			if litStart < 0 {
				litStart = i
			}
			lastSig = c
		default:
			lastSig, litStart = c, -1
		}
	}
	return stack, lastSig, inString, litStart
}

// rbJSONEOFErr is the message for input that ends too soon: inside a
// string, inside a literal, after an object's `{` or `,`, or anywhere else.
func rbJSONEOFErr(s string) string {
	stack, lastSig, inString, litStart := rbJSONContext(s, len(s))
	switch {
	case inString:
		return "unexpected end of input, expected closing \"" + rbJSONAt(s, len(s))
	case litStart >= 0:
		return "unexpected token '" + rbJSONSnippet(s, litStart) + "'" + rbJSONAt(s, litStart)
	case len(stack) > 0 && stack[len(stack)-1] == '{' && (lastSig == '{' || lastSig == ','):
		return "expected object key, got EOF" + rbJSONAt(s, len(s))
	}
	return "unexpected end of input" + rbJSONAt(s, len(s))
}

// rbJSONVal wraps a decoded value: a JSON null is a real `any(nil)` v, not a decode failure, so rbJSONDecodeValue returns this struct rather than (any, error), which would read as nilnil's ambiguous "nil value, nil error".
type rbJSONVal struct{ v any }

// rbJSONDecodeValue decodes one JSON value inside depth containers; keyOf (String or Symbol per symbolize_names) fixes every nested Hash to the same key type. A container deeper than limit raises NestingError unless it is empty, as in the gem.
func rbJSONDecodeValue[K comparable](dec *json.Decoder, depth, limit int, keyOf func(string) K) (rbJSONVal, error) {
	tok, err := dec.Token()
	if err != nil {
		return rbJSONVal{}, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			if depth >= limit && dec.More() {
				panic(rbJSONNesting(depth+1, ""))
			}
			h := NewHash[K, any]()
			for dec.More() {
				kt, kerr := dec.Token()
				if kerr != nil {
					return rbJSONVal{}, kerr
				}
				ks, ok := kt.(string)
				if !ok {
					return rbJSONVal{}, fmt.Errorf("expected object key, got %v", kt)
				}
				v, verr := rbJSONDecodeValue(dec, depth+1, limit, keyOf)
				if verr != nil {
					return rbJSONVal{}, verr
				}
				Hash_Op_idxSet(h, keyOf(ks), v.v)
			}
			if _, cerr := dec.Token(); cerr != nil { // consume '}'
				return rbJSONVal{}, cerr
			}
			return rbJSONVal{h}, nil
		case '[':
			if depth >= limit && dec.More() {
				panic(rbJSONNesting(depth+1, ""))
			}
			arr := &Array[any]{}
			for dec.More() {
				v, verr := rbJSONDecodeValue(dec, depth+1, limit, keyOf)
				if verr != nil {
					return rbJSONVal{}, verr
				}
				arr.s = append(arr.s, v.v)
			}
			if _, cerr := dec.Token(); cerr != nil { // consume ']'
				return rbJSONVal{}, cerr
			}
			return rbJSONVal{arr}, nil
		}
	case string:
		return rbJSONVal{String(t)}, nil
	case json.Number:
		return rbJSONVal{rbJSONParseNumber(string(t))}, nil
	case bool:
		return rbJSONVal{Boolean(t)}, nil
	case nil:
		return rbJSONVal{}, nil // v's zero value is untyped nil: JSON null
	}
	return rbJSONVal{}, fmt.Errorf("unexpected token %v", tok)
}

// rbJSONParseNumber: a decimal point or exponent is a Float (MRI's json gem); past 64 bits raises RangeError, as String#to_i (decision 35: no Bignum).
func rbJSONParseNumber(s string) any {
	if strings.ContainsAny(s, ".eE") {
		f, _ := strconv.ParseFloat(s, 64) // ErrRange (too large) saturates to +-Inf, as MRI
		return Float(f)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(NewRangeError(Ref(String(s + " overflows Integer (64-bit; no Bignum)"))))
	}
	return Integer(n)
}

// rbJSONDecodeErr turns a decode error into JSON::ParserError's message,
// worded as the json gem's parser words it for the same input (its tests
// are MRI's own messages); an error with no twin keeps Go's wording.
func rbJSONDecodeErr(err error, s string) string {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return rbJSONEOFErr(s)
	}
	var se *json.SyntaxError
	if !errors.As(err, &se) {
		return err.Error()
	}
	msg := se.Error()
	pos := min(max(int(se.Offset)-1, 0), len(s)-1) // Offset is past the offending byte
	switch {
	case strings.HasPrefix(msg, "unexpected end of JSON input"):
		return rbJSONEOFErr(s)
	case strings.HasSuffix(msg, "looking for beginning of value"), strings.HasPrefix(msg, "missing value after object key"), strings.HasPrefix(msg, "object member name must be a string"):
		stack, lastSig, _, _ := rbJSONContext(s, pos)
		if s[pos] == ',' && lastSig != ',' { // a trailing comma is reported at the comma; the gem reports what follows it
			if next := rbJSONSkipSpace(s, pos+1); next < len(s) && (s[next] == ']' || s[next] == '}') {
				pos = next
				stack, lastSig, _, _ = rbJSONContext(s, pos)
			}
		}
		if len(stack) > 0 && stack[len(stack)-1] == '{' && lastSig != ':' { // where a key should be
			if lastSig == ',' {
				return "expected object key, got: '" + rbJSONSnippet(s, pos) + "'" + rbJSONAt(s, pos)
			}
			return "expected object key, got '" + rbJSONSnippet(s, pos) + "'" + rbJSONAt(s, pos)
		}
		return "unexpected character: '" + rbJSONSnippet(s, pos) + "'" + rbJSONAt(s, pos)
	case strings.HasSuffix(msg, "after array element"):
		return "expected ',' or ']' after array value" + rbJSONAt(s, pos)
	case strings.HasSuffix(msg, "after object key"):
		return "expected ':' after object key" + rbJSONAt(s, pos)
	case strings.HasSuffix(msg, "after object key:value pair"):
		return "expected ',' or '}' after object value, got: '" + rbJSONSnippet(s, pos) + "'" + rbJSONAt(s, pos)
	case strings.Contains(msg, " in literal "):
		start := pos
		for start > 0 && s[start-1] >= 'a' && s[start-1] <= 'z' {
			start--
		}
		return "unexpected token '" + rbJSONSnippet(s, start) + "'" + rbJSONAt(s, start)
	case strings.Contains(msg, "numeric literal") || strings.Contains(msg, "after decimal point") || strings.Contains(msg, "in exponent"):
		start := pos
		for start > 0 && strings.IndexByte("0123456789+-.eE", s[start-1]) >= 0 {
			start--
		}
		return "invalid number: '" + rbJSONSnippet(s, start) + "'" + rbJSONAt(s, start)
	}
	return msg + rbJSONAt(s, pos)
}

// rbJSONState is the gem's generator State. It is handed to every
// nested to_json, which is how depth reaches the indentation.
type rbJSONState struct {
	indent, space, spaceBefore, objectNl, arrayNl string
	scriptSafe, asciiOnly, allowNaN               bool
	depth, maxNesting                             int
	buf                                           []byte // the document so far: core values append here instead of returning a String each
}

// enter is the gem's depth check: a container one level deeper than max_nesting (or the hard cap) raises NestingError, which also ends a cycle.
func (st *rbJSONState) enter() {
	st.depth++
	if st.depth > rbJSONLimit(st.maxNesting) {
		panic(rbJSONNesting(st.depth-1, ". Did you try to serialize objects with circular references?"))
	}
}

// rbJSONStateOf is State.from_state on a to_json's arguments: the state
// passed down, a new one configured by an options Hash, or a default.
func rbJSONStateOf(args []any) *rbJSONState { return rbJSONStateOfMax(args, 100, false) }

// rbJSONStateOfMax is rbJSONStateOf with other defaults (JSON.dump's: max_nesting 0, allow_nan true).
func rbJSONStateOfMax(args []any, maxNesting int, allowNaN bool) *rbJSONState {
	if len(args) > 0 {
		if o, ok := args[0].(*rbJSONState); ok { // the nested case: no allocation
			return o
		}
	}
	st := &rbJSONState{maxNesting: maxNesting, allowNaN: allowNaN}
	if len(args) == 0 {
		return st
	}
	switch o := rbUnbox(args[0]).(type) {
	case *rbJSONState:
		return o
	case Hash_Any:
		opts := o._ToAny()
		for k, hv := range rbHashPairs(opts) {
			v := hv
			switch k {
			case Symbol("indent"):
				st.indent = rbJSONOpt(v)
			case Symbol("space"):
				st.space = rbJSONOpt(v)
			case Symbol("space_before"):
				st.spaceBefore = rbJSONOpt(v)
			case Symbol("object_nl"):
				st.objectNl = rbJSONOpt(v)
			case Symbol("array_nl"):
				st.arrayNl = rbJSONOpt(v)
			case Symbol("script_safe"), Symbol("escape_slash"):
				st.scriptSafe = rbTruthy(v)
			case Symbol("ascii_only"):
				st.asciiOnly = rbTruthy(v)
			case Symbol("allow_nan"):
				st.allowNaN = rbTruthy(v)
			case Symbol("max_nesting"):
				st.maxNesting = rbJSONMaxNesting(v)
			case Symbol("depth"):
				if d, ok := v.(Integer); ok {
					st.depth = int(d)
				}
			case Symbol("sort_keys"), Symbol("strict"), Symbol("as_json"):
				if rbTruthy(v) {
					panic(NewNotImplementedError(Ref(String("JSON generator option " + string(k.(Symbol)) + " is not supported"))))
				}
			}
		}
	}
	return st
}

// rbJSONOpt reads a String option; nil means "".
func rbJSONOpt(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case String:
		return string(s)
	}
	panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(v) + " (expected String)"))))
}

// rbJSONGenerate is JSON.generate's body: obj written into st.buf as the gem's generate_json does.
func rbJSONGenerate(obj any, st *rbJSONState) String {
	n := len(st.buf)
	rbJSONValueTo(st, obj)
	out := String(st.buf[n:])
	st.buf = st.buf[:n]
	return out
}

// rbJSONCore: false when v is not self, since a subclass promotes the method and the gem calls a subclass's to_json (decision 346).
type rbJSONCore interface {
	rbJSONTo(st *rbJSONState, v any) bool
}

func (self *Array[E]) rbJSONTo(st *rbJSONState, v any) bool {
	if any(self) != v {
		return false
	}
	rbJSONArrayTo(st, self.s)
	return true
}

func (self *Hash[K, V]) rbJSONTo(st *rbJSONState, v any) bool {
	if any(self) != v {
		return false
	}
	rbJSONHashTo(st, self)
	return true
}

// rbJSONValueTo is the gem's generate_json: exact core values skip their to_json, as json 2.x does (decision 346).
func rbJSONValueTo(st *rbJSONState, a any) {
	switch v := a.(type) {
	case String:
		rbJSONStringTo(st, string(v))
		return
	case Integer:
		st.buf = strconv.AppendInt(st.buf, int64(v), 10)
		return
	case Float:
		st.buf = append(st.buf, rbJSONFloat(float64(v), st)...)
		return
	case Boolean:
		st.buf = strconv.AppendBool(st.buf, bool(v))
		return
	case nil:
		st.buf = append(st.buf, "null"...)
		return
	case rbJSONCore:
		if v.rbJSONTo(st, a) {
			return
		}
	}
	u := rbUnbox(a) // a T? box (*T, **Array) or a typed nil
	switch v := u.(type) {
	case String, Integer, Float, Boolean, nil:
		rbJSONValueTo(st, u)
		return
	case rbJSONCore:
		if v.rbJSONTo(st, u) {
			return
		}
	}
	n := len(st.buf)
	r := rbToJson(u, st)
	st.buf = append(st.buf[:n], r...) // [:n]: a to_json that rescued a nested error may have left a partial value
}

// rbJSONArray and rbJSONHash are Array#/Hash#to_json: the value written past the buffer's end, then taken out as a String.
func rbJSONArray[E any](xs []E, args []any) String {
	st := rbJSONStateOf(args)
	n := len(st.buf)
	rbJSONArrayTo(st, xs)
	out := String(st.buf[n:])
	st.buf = st.buf[:n]
	return out
}

func rbJSONHash[K, V comparable](h *Hash[K, V], args []any) String {
	st := rbJSONStateOf(args)
	n := len(st.buf)
	rbJSONHashTo(st, h)
	out := String(st.buf[n:])
	st.buf = st.buf[:n]
	return out
}

func rbJSONIndent(st *rbJSONState, depth int) {
	if st.indent != "" {
		for range depth {
			st.buf = append(st.buf, st.indent...)
		}
	}
}

// rbJSONArrayTo and rbJSONHashTo port the gem's generate_json_array and
// generate_json_object: indent per depth before each element, the
// newline string after the opener and each ",", and before the closer
// (then indented) only when set.
func rbJSONArrayTo[E any](st *rbJSONState, xs []E) {
	st.enter()
	depth := st.depth
	if len(xs) == 0 {
		st.depth--
		st.buf = append(st.buf, "[]"...)
		return
	}
	st.buf = append(st.buf, '[')
	st.buf = append(st.buf, st.arrayNl...)
	for i, x := range xs {
		if i > 0 {
			st.buf = append(st.buf, ',')
			st.buf = append(st.buf, st.arrayNl...)
		}
		rbJSONIndent(st, depth)
		rbJSONValueTo(st, x)
		st.depth = depth
	}
	st.depth--
	if st.arrayNl != "" {
		st.buf = append(st.buf, st.arrayNl...)
		rbJSONIndent(st, st.depth)
	}
	st.buf = append(st.buf, ']')
}

func rbJSONHashTo[K, V comparable](st *rbJSONState, h *Hash[K, V]) {
	st.enter()
	depth := st.depth
	if int(h.live) == 0 {
		st.depth--
		st.buf = append(st.buf, "{}"...)
		return
	}
	st.buf = append(st.buf, '{')
	first := true
	for k, hv := range rbHashPairs(h) {
		if !first {
			st.buf = append(st.buf, ',')
		}
		st.buf = append(st.buf, st.objectNl...)
		rbJSONIndent(st, depth)
		if s, ok := any(k).(String); ok {
			rbJSONStringTo(st, string(s))
		} else {
			rbJSONStringTo(st, string(rbToS(k)))
		}
		st.buf = append(st.buf, st.spaceBefore...)
		st.buf = append(st.buf, ':')
		st.buf = append(st.buf, st.space...)
		first = false
		rbJSONValueTo(st, hv)
		st.depth = depth
	}
	st.depth--
	if st.objectNl != "" {
		st.buf = append(st.buf, st.objectNl...)
		rbJSONIndent(st, st.depth)
	}
	st.buf = append(st.buf, '}')
}

// rbJSONStringTo: not jsontext.AppendQuote (same escapes) because that needs go 1.27 in the generated module (decision 346).
func rbJSONStringTo(st *rbJSONState, s string) {
	if st.scriptSafe || st.asciiOnly {
		st.buf = append(st.buf, rbJSONString(s, st)...)
		return
	}
	b, ok := rbJSONQuoteTo(st.buf, s)
	if !ok {
		panic(NewJSON_GeneratorError(Ref(String("source sequence is illegal/malformed utf-8"))))
	}
	st.buf = b
}

const rbJSONHex = "0123456789abcdef"

func rbJSONQuoteTo(dst []byte, s string) ([]byte, bool) {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= utf8.RuneSelf {
			r, n := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && n == 1 {
				return dst, false
			}
			i += n
			continue
		}
		if c >= 0x20 && c != '"' && c != '\\' {
			i++
			continue
		}
		dst = append(dst, s[start:i]...)
		switch c {
		case '"', '\\':
			dst = append(dst, '\\', c)
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		default:
			dst = append(dst, '\\', 'u', '0', '0', rbJSONHex[c>>4], rbJSONHex[c&0xf])
		}
		i++
		start = i
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"'), true
}

// rbJSONString is String#to_json: it escapes like the json gem's generator: quotes,
// backslashes and control characters; "/" and non-ASCII stay as-is
// unless st asks for script_safe (also U+2028/9) or ascii_only.
// Invalid UTF-8 is the gem's GeneratorError.
func rbJSONString(s string, st *rbJSONState) String {
	if !utf8.ValidString(s) {
		panic(NewJSON_GeneratorError(Ref(String("source sequence is illegal/malformed utf-8"))))
	}
	if !st.scriptSafe && !st.asciiOnly && !strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == '"' || r == '\\' }) {
		return String(`"` + s + `"`) // nothing to escape: one allocation
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i, r := range s {
		if st.scriptSafe && (r == '/' || r == 0x2028 || r == 0x2029) || st.asciiOnly && r >= 0x80 {
			switch {
			case r == '/':
				b.WriteString(`\/`)
			case r > 0xffff:
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
			continue
		}
		c := s[i]
		if r >= 0x80 {
			b.WriteRune(r)
			continue
		}
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return String(b.String())
}

// rbJSONFloat ports the json gem's fpconv_dtoa: Grisu2's digits (not
// always the shortest: 1e23 is 9.999999999999999e+22), then
// emit_digits' plain decimal for moderate exponents, else d.ddde[+-]N.
func rbJSONFloat(f float64, st *rbJSONState) String {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if st.allowNaN {
			return rbFloatToS(f)
		}
		panic(NewJSON_GeneratorError(Ref(String(string(rbFloatToS(f)) + " not allowed in JSON"))))
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
		f = -f
	}
	if f == 0 {
		return String(sign + "0.0")
	}
	digits, k := rbGrisu2(f)
	nd := len(digits)
	exp := k + nd - 1
	if exp < 0 {
		exp = -exp
	}
	switch {
	case k >= 0 && exp < 15:
		return String(sign + digits + strings.Repeat("0", k) + ".0")
	case k < 0 && (k > -7 || exp < 10):
		offset := nd + k
		if offset <= 0 {
			return String(sign + "0." + strings.Repeat("0", -offset) + digits)
		}
		return String(sign + digits[:offset] + "." + digits[offset:])
	}
	nd = min(nd, 18-len(sign))
	out := sign + digits[:1]
	if nd > 1 {
		out += "." + digits[1:nd]
	}
	esign := "+"
	if k+nd-1 < 0 {
		esign = "-"
	}
	return String(out + "e" + esign + strconv.Itoa(exp))
}

// rbFp is fpconv's Fp: frac * 2^exp.
type rbFp struct {
	frac uint64
	exp  int
}

// rbPowersTen is fpconv's cached powers of ten, 10^-348 to 10^340 in
// steps of 8.
var rbPowersTen = [...]rbFp{
	{18054884314459144840, -1220}, {13451937075301367670, -1193},
	{10022474136428063862, -1166}, {14934650266808366570, -1140},
	{11127181549972568877, -1113}, {16580792590934885855, -1087},
	{12353653155963782858, -1060}, {18408377700990114895, -1034},
	{13715310171984221708, -1007}, {10218702384817765436, -980},
	{15227053142812498563, -954}, {11345038669416679861, -927},
	{16905424996341287883, -901}, {12595523146049147757, -874},
	{9384396036005875287, -847}, {13983839803942852151, -821},
	{10418772551374772303, -794}, {15525180923007089351, -768},
	{11567161174868858868, -741}, {17236413322193710309, -715},
	{12842128665889583758, -688}, {9568131466127621947, -661},
	{14257626930069360058, -635}, {10622759856335341974, -608},
	{15829145694278690180, -582}, {11793632577567316726, -555},
	{17573882009934360870, -529}, {13093562431584567480, -502},
	{9755464219737475723, -475}, {14536774485912137811, -449},
	{10830740992659433045, -422}, {16139061738043178685, -396},
	{12024538023802026127, -369}, {17917957937422433684, -343},
	{13349918974505688015, -316}, {9946464728195732843, -289},
	{14821387422376473014, -263}, {11042794154864902060, -236},
	{16455045573212060422, -210}, {12259964326927110867, -183},
	{18268770466636286478, -157}, {13611294676837538539, -130},
	{10141204801825835212, -103}, {15111572745182864684, -77},
	{11258999068426240000, -50}, {16777216000000000000, -24},
	{12500000000000000000, 3}, {9313225746154785156, 30},
	{13877787807814456755, 56}, {10339757656912845936, 83},
	{15407439555097886824, 109}, {11479437019748901445, 136},
	{17105694144590052135, 162}, {12744735289059618216, 189},
	{9495567745759798747, 216}, {14149498560666738074, 242},
	{10542197943230523224, 269}, {15709099088952724970, 295},
	{11704190886730495818, 322}, {17440603504673385349, 348},
	{12994262207056124023, 375}, {9681479787123295682, 402},
	{14426529090290212157, 428}, {10748601772107342003, 455},
	{16016664761464807395, 481}, {11933345169920330789, 508},
	{17782069995880619868, 534}, {13248674568444952270, 561},
	{9871031767461413346, 588}, {14708983551653345445, 614},
	{10959046745042015199, 641}, {16330252207878254650, 667},
	{12166986024289022870, 694}, {18130221999122236476, 720},
	{13508068024458167312, 747}, {10064294952495520794, 774},
	{14996968138956309548, 800}, {11173611982879273257, 827},
	{16649979327439178909, 853}, {12405201291620119593, 880},
	{9242595204427927429, 907}, {13772540099066387757, 933},
	{10261342003245940623, 960}, {15290591125556738113, 986},
	{11392378155556871081, 1013}, {16975966327722178521, 1039},
	{12648080533535911531, 1066},
}

// rbFpMul is fpconv's multiply: the high 64 bits of a*b, rounded.
func rbFpMul(a, b rbFp) rbFp {
	hi, lo := bits.Mul64(a.frac, b.frac)
	_, c := bits.Add64(lo, 1<<63, 0)
	return rbFp{hi + c, a.exp + b.exp + 64}
}

// rbGrisu2 is fpconv's grisu2 and generate_digits on a positive finite
// f: its digits d and exponent k, f ~= d * 10^k.
func rbGrisu2(f float64) (string, int) {
	b := math.Float64bits(f)
	w := rbFp{b & (1<<52 - 1), int(b >> 52)}
	if w.exp != 0 {
		w.frac += 1 << 52
		w.exp -= 1075
	} else {
		w.exp = -1074
	}
	upper := rbFp{w.frac<<1 + 1, w.exp - 1}
	for upper.frac&(1<<53) == 0 {
		upper.frac <<= 1
		upper.exp--
	}
	upper.frac <<= 10
	upper.exp -= 10
	ls := 1
	if w.frac == 1<<52 {
		ls = 2
	}
	lower := rbFp{(w.frac<<ls - 1) << (w.exp - ls - upper.exp), upper.exp}
	for w.frac&(1<<52) == 0 {
		w.frac <<= 1
		w.exp--
	}
	w.frac <<= 11
	w.exp -= 11

	idx := (int(float64(-(upper.exp+87))*0.30102999566398114) + 348) / 8
	for {
		if c := upper.exp + rbPowersTen[idx].exp + 64; c < -60 {
			idx++
		} else if c > -32 {
			idx--
		} else {
			break
		}
	}
	k := 348 - idx*8
	cp := rbPowersTen[idx]
	w, upper, lower = rbFpMul(w, cp), rbFpMul(upper, cp), rbFpMul(lower, cp)
	lower.frac++
	upper.frac--

	wfrac := upper.frac - w.frac
	delta := upper.frac - lower.frac
	sh := uint(-upper.exp)
	one := uint64(1) << sh
	part1, part2 := upper.frac>>sh, upper.frac&(one-1)
	var d []byte
	kappa := 10
	for div := uint64(1e9); kappa > 0; div /= 10 {
		digit := part1 / div
		if digit != 0 || len(d) > 0 {
			d = append(d, byte('0'+digit))
		}
		part1 -= digit * div
		kappa--
		if rem := part1<<sh + part2; rem <= delta {
			rbRoundDigit(d, delta, rem, div<<sh, wfrac)
			return string(d), k + kappa
		}
	}
	for unit := uint64(10); ; unit *= 10 {
		part2 *= 10
		delta *= 10
		kappa--
		if digit := part2 >> sh; digit != 0 || len(d) > 0 {
			d = append(d, byte('0'+digit))
		}
		part2 &= one - 1
		if part2 < delta {
			rbRoundDigit(d, delta, part2, one, wfrac*unit)
			return string(d), k + kappa
		}
	}
}

// rbRoundDigit is fpconv's round_digit: step the last digit down while
// that moves closer to w and stays inside the boundaries.
func rbRoundDigit(d []byte, delta, rem, kappa, frac uint64) {
	for rem < frac && delta-rem >= kappa && (rem+kappa < frac || frac-rem > rem+kappa-frac) {
		d[len(d)-1]--
		rem += kappa
	}
}
