//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Array#pack and String#unpack share one parser and one sizing table so the two sides agree (decision 138).

// rbPackDir is one directive with its count and its `_`/`!` (native size) and `<`/`>` (byte order) modifiers.
type rbPackDir struct {
	d             byte
	count         int
	star, counted bool
	native        bool
	order         byte
}

// rbPackBigEndian is the platform's byte order, for directives without `<`/`>`.
var rbPackBigEndian = binary.NativeEndian.Uint16([]byte{0, 1}) == 1

// rbPackParse splits format into directives; what is "pack" or "unpack" for MRI's messages.
func rbPackParse(format, what string) []rbPackDir {
	var out []rbPackDir
	for i := 0; i < len(format); {
		d := format[i]
		i++
		switch d {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			continue
		case '#': // a comment to the end of the line, as MRI's
			for i < len(format) && format[i] != '\n' {
				i++
			}
			continue
		}
		if d == 'P' || d == 'p' {
			panic(NewArgumentError(Ref(String("rugaroo: " + what + " directive '" + string(d) + "' (a C pointer) is not supported"))))
		}
		if strings.IndexByte("aAZBbHhuMmUwCcSsLlQqJjIinNvVDdFfEeGgxX@", d) < 0 {
			shown := string(d)
			if d < ' ' || d > '~' { // MRI shows an unprintable byte as \xNN
				shown = fmt.Sprintf("\\x%02x", d)
			}
			shownFmt := format
			if strings.ContainsFunc(format, func(r rune) bool { return r < ' ' || r == 0x7f }) {
				shownFmt = rbPackEscape(format)
			}
			panic(NewArgumentError(Ref(String("unknown " + what + " directive '" + shown + "' in '" + shownFmt + "'"))))
		}
		dir := rbPackDir{d: d, count: 1}
		for i < len(format) && strings.IndexByte("<>_!", format[i]) >= 0 {
			if strings.IndexByte("sSiIlLqQjJ", d) < 0 {
				panic(NewArgumentError(Ref(String("'" + format[i:i+1] + "' allowed only after types sSiIlLqQjJ"))))
			}
			if format[i] == '<' || format[i] == '>' {
				dir.order = format[i]
			} else {
				dir.native = true
			}
			i++
		}
		switch {
		case i < len(format) && format[i] == '*':
			dir.star = true
			i++
		case i < len(format) && format[i] >= '0' && format[i] <= '9':
			j := i
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			n, err := strconv.Atoi(format[i:j])
			if err != nil {
				panic(NewRangeError(Ref(String("pack length too big"))))
			}
			dir.count, dir.counted = n, true
			i = j
		}
		out = append(out, dir)
	}
	return out
}

// rbPackInt sizes an integer directive: bytes, signedness and byte order. ok is false for a non-integer directive.
func (d rbPackDir) rbPackInt() (size int, signed, big, ok bool) {
	big = rbPackBigEndian
	switch d.d {
	case 'C', 'c':
		size = 1
	case 'S', 's':
		size = 2
	case 'I', 'i':
		size = 4
	case 'L', 'l':
		size = 4
		if d.native {
			size = strconv.IntSize / 8 // C's long
		}
	case 'Q', 'q':
		size = 8
	case 'J', 'j':
		size = strconv.IntSize / 8 // intptr_t
	case 'n':
		return 2, false, true, true
	case 'N':
		return 4, false, true, true
	case 'v':
		return 2, false, false, true
	case 'V':
		return 4, false, false, true
	default:
		return 0, false, false, false
	}
	if d.order != 0 {
		big = d.order == '>'
	}
	return size, d.d >= 'a', big, true
}

// rbPackFloat sizes a float directive: D d F f native order, E e little-endian, G g big-endian.
func (d rbPackDir) rbPackFloat() (size int, big, ok bool) {
	switch d.d {
	case 'D', 'd':
		return 8, rbPackBigEndian, true
	case 'F', 'f':
		return 4, rbPackBigEndian, true
	case 'E':
		return 8, false, true
	case 'e':
		return 4, false, true
	case 'G':
		return 8, true, true
	case 'g':
		return 4, true, true
	}
	return 0, false, false
}

// rbPackName is how MRI names v in a conversion error: nil, true and false by value.
func rbPackName(v any) string {
	switch x := rbUnbox(v).(type) {
	case nil:
		return "nil"
	case Boolean:
		if x {
			return "true"
		}
		return "false"
	}
	return rbClassName(v)
}

// rbPackPut appends u's low size bytes in the given order.
func rbPackPut(b []byte, u uint64, size int, big bool) []byte {
	for k := range size {
		if big {
			b = append(b, byte(u>>(8*(size-1-k))))
		} else {
			b = append(b, byte(u>>(8*k)))
		}
	}
	return b
}

// rbPackGet reads size bytes at s in the given order.
func rbPackGet(s string, size int, big bool) uint64 {
	var u uint64
	for k := range size {
		c := uint64(s[k])
		if big {
			u = u<<8 | c
		} else {
			u |= c << (8 * k)
		}
	}
	return u
}

// rbPackHexNibble is MRI's H/h digit: a letter's low four bits plus 9, else the low four bits.
func rbPackHexNibble(c byte) byte {
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return ((c & 15) + 9) & 15
	}
	return c & 15
}

// rbPackUU is the uuencode alphabet: 0 is a backquote, as MRI's.
const rbPackUU = "`!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_"

// rbPackEncodes is MRI's encodes: one u or m line of s, ending in "\n" when lf.
func rbPackEncodes(b []byte, s string, uu, lf bool) []byte {
	if !uu {
		b = base64.StdEncoding.AppendEncode(b, []byte(s))
	} else {
		b = append(b, byte(len(s))+' ')
		for i := 0; i < len(s); i += 3 {
			var c [3]byte
			n := copy(c[:], s[i:])
			b = append(b, rbPackUU[c[0]>>2], rbPackUU[(c[0]<<4|c[1]>>4)&63])
			if n > 1 {
				b = append(b, rbPackUU[(c[1]<<2|c[2]>>6)&63])
			} else {
				b = append(b, '`')
			}
			if n > 2 {
				b = append(b, rbPackUU[c[2]&63])
			} else {
				b = append(b, '`')
			}
		}
	}
	if lf {
		b = append(b, '\n')
	}
	return b
}

// rbPackQP is MRI's qpencode: quoted-printable with soft breaks past width columns.
func rbPackQP(b []byte, s string, width int) []byte {
	n, prev := 0, -1
	for i := range len(s) {
		c := s[i]
		switch {
		case c > 126 || c < 32 && c != '\n' && c != '\t' || c == '=':
			b = append(b, '=', "0123456789ABCDEF"[c>>4], "0123456789ABCDEF"[c&15])
			n += 3
			prev = -1
		case c == '\n':
			if prev == ' ' || prev == '\t' {
				b = append(b, '=', c)
			}
			b = append(b, c)
			n, prev = 0, int(c)
		default:
			b = append(b, c)
			n++
			prev = int(c)
		}
		if n > width {
			b = append(b, '=', '\n')
			n, prev = 0, '\n'
		}
	}
	if n > 0 {
		b = append(b, '=', '\n')
	}
	return b
}

// rbPack is Array#pack. Integers past 64 bits cannot occur (decision 35), so Q/q/J/j wrap like MRI's for any Integer.
func rbPack(items []any, format string) string {
	var b []byte
	next := func() any {
		if len(items) == 0 {
			panic(NewArgumentError(Ref(String("too few arguments"))))
		}
		v := items[0]
		items = items[1:]
		return rbUnbox(v)
	}
	str := func(nilOK bool) string {
		v := next()
		switch x := v.(type) {
		case String:
			return string(x)
		case nil:
			if nilOK {
				return ""
			}
		}
		panic(rbConvError(v, "String"))
	}
	integer := func() int {
		switch x := next().(type) {
		case Integer:
			return int(x)
		case Float:
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) || math.Abs(float64(x)) >= 1<<63 {
				panic(NewRangeError(Ref(String("float " + string(rbToS(x)) + " out of range of integer"))))
			}
			return int(x)
		default:
			panic(rbConvError(x, "Integer"))
		}
	}
	for _, d := range rbPackParse(format, "pack") {
		if size, _, big, ok := d.rbPackInt(); ok {
			n := d.count
			if d.star {
				n = len(items)
			}
			for range n {
				b = rbPackPut(b, uint64(integer()), size, big)
			}
			continue
		}
		if size, big, ok := d.rbPackFloat(); ok {
			n := d.count
			if d.star {
				n = len(items)
			}
			for range n {
				var f float64
				switch x := next().(type) {
				case Float:
					f = float64(x)
				case Integer:
					f = float64(x)
				default:
					panic(NewTypeError(Ref(String("can't convert " + rbPackName(x) + " into Float"))))
				}
				if size == 8 {
					b = rbPackPut(b, math.Float64bits(f), 8, big)
				} else {
					b = rbPackPut(b, uint64(math.Float32bits(float32(f))), 4, big)
				}
			}
			continue
		}
		switch d.d {
		case 'a', 'A', 'Z':
			s := str(true)
			n := d.count
			if d.star {
				n = len(s)
				if d.d == 'Z' {
					n++
				}
			}
			rbCheckAlloc(n, 1)
			pad := byte(0)
			if d.d == 'A' {
				pad = ' '
			}
			for k := range n {
				if k < len(s) {
					b = append(b, s[k])
				} else {
					b = append(b, pad)
				}
			}
		case 'B', 'b', 'H', 'h':
			s := str(true)
			per := 8
			if d.d == 'H' || d.d == 'h' {
				per = 2
			}
			n := d.count
			if d.star || n > len(s) {
				n = len(s)
			}
			width := d.count
			if d.star {
				width = len(s)
			}
			rbCheckAlloc(width/per, 1)
			out := make([]byte, (width+per-1)/per)
			for k := range n {
				c := s[k]
				var v byte
				shift := 0
				switch d.d {
				case 'B':
					v, shift = c&1, 7-k%8
				case 'b':
					v, shift = c&1, k%8
				case 'H':
					v, shift = rbPackHexNibble(c), 4*(1-k%2)
				case 'h':
					v, shift = rbPackHexNibble(c), 4*(k%2)
				}
				out[k/per] |= v << shift
			}
			b = append(b, out...)
		case 'u', 'm':
			s := str(false)
			if d.d == 'm' && d.counted && d.count == 0 {
				b = rbPackEncodes(b, s, false, false)
				continue
			}
			width := d.count
			switch {
			case d.star || width <= 2:
				width = 45
			case width > 63 && d.d == 'u':
				width = 63
			default:
				width = width / 3 * 3
			}
			for len(s) > 0 {
				k := min(width, len(s))
				b = rbPackEncodes(b, s[:k], d.d == 'u', true)
				s = s[k:]
			}
		case 'M':
			width := d.count
			if d.star || width <= 1 {
				width = 72
			}
			b = rbPackQP(b, string(rbToS(next())), width)
		case 'U':
			n := d.count
			if d.star {
				n = len(items)
			}
			for range n {
				b = rbPackU(b, integer())
			}
		case 'w':
			n := d.count
			if d.star {
				n = len(items)
			}
			for range n {
				c := integer()
				if c < 0 {
					panic(NewArgumentError(Ref(String("can't compress negative numbers"))))
				}
				var grp []byte
				for {
					grp = append(grp, byte(c&0x7f))
					if c >>= 7; c == 0 {
						break
					}
				}
				for k := len(grp) - 1; k >= 0; k-- {
					if k > 0 {
						grp[k] |= 0x80
					}
					b = append(b, grp[k])
				}
			}
		case 'x':
			if !d.star {
				rbCheckAlloc(d.count, 1)
				b = append(b, make([]byte, d.count)...)
			}
		case 'X':
			n := d.count
			if d.star {
				n = 0
			}
			if n > len(b) {
				panic(NewArgumentError(Ref(String("X outside of string"))))
			}
			b = b[:len(b)-n]
		case '@':
			n := d.count
			if d.star {
				n = 0
			}
			if n > len(b) {
				rbCheckAlloc(n, 1)
				b = append(b, make([]byte, n-len(b))...)
			}
			b = b[:n]
		}
	}
	return string(b)
}

// rbPackU is MRI's rb_uv_to_utf8: UTF-8's bit pattern up to six bytes, surrogates and values past U+10FFFF included.
func rbPackU(out []byte, n int) []byte {
	size := 0
	for i, limit := range []int{0x80, 0x800, 0x10000, 0x200000, 0x4000000, 0x80000000} {
		if n >= 0 && n < limit {
			size = i + 1
			break
		}
	}
	switch size {
	case 0:
		panic(NewRangeError(Ref(String("pack(U): value out of range"))))
	case 1:
		return append(out, byte(n))
	}
	b := make([]byte, size)
	for i := size - 1; i > 0; i-- {
		b[i] = byte(0x80 | n&0x3F)
		n >>= 6
	}
	b[0] = byte(0xFF<<(8-size)) | byte(n)
	return append(out, b...)
}

// rbUnpackPad refuses an integer or float directive whose count would pad the result with nils past what an Array can hold: MRI's IndexError past its own array limit, else rbCheckAlloc's.
func rbUnpackPad(n, have int) {
	if n > have {
		if n >= 1<<60 {
			panic(NewIndexError(Ref(String("index " + strconv.Itoa(n-1) + " too big"))))
		}
		rbCheckAlloc(n-have, 16)
	}
}

// rbUnpack is String#unpack; an unsigned value past 2**63 raises RangeError where MRI makes a Bignum (decision 35).
func rbUnpack(s, format string, offset Integer) *Array[any] {
	return rbUnpackWrap(s, format, false, offset)
}

// rbUnpackWrap is rbUnpack; wrap keeps an unsigned value past 2**63 as its 64 bits (a gem's, decision 324). offset is
// `offset:`, where reading starts (prism's serialize.rb reads its constant pool so); `@` stays from the string's start, as MRI's.
func rbUnpackWrap(s, format string, wrap bool, offset Integer) *Array[any] {
	if offset < 0 || int(offset) > len(s) {
		panic(NewArgumentError(Ref(String("offset outside of string"))))
	}
	out := &Array[any]{}
	pos := int(offset)
	big := func(u uint64) Integer {
		if u > math.MaxInt64 && !wrap {
			panic(NewRangeError(Ref(String(strconv.FormatUint(u, 10) + " overflows Integer (64-bit; no Bignum)"))))
		}
		return Integer(u) //nolint:gosec // wraps only when asked
	}
	for _, d := range rbPackParse(format, "unpack") {
		rest := s[pos:]
		count := d.count
		if !d.counted && d.d == '@' {
			count = 0
		}
		if d.star {
			count = len(rest)
		}
		if size, signed, bigEnd, ok := d.rbPackInt(); ok {
			n := count
			if d.star {
				n = len(rest) / size
			}
			rbUnpackPad(n, len(rest)/size)
			for range n {
				if pos+size > len(s) {
					out.s = append(out.s, nil)
					continue
				}
				u := rbPackGet(s[pos:], size, bigEnd)
				switch {
				case signed && size < 8 && u&(1<<(8*size-1)) != 0:
					out.s = append(out.s, Integer(int64(u)-int64(1)<<(8*size)))
				case signed:
					out.s = append(out.s, Integer(int64(u)))
				default:
					out.s = append(out.s, big(u))
				}
				pos += size
			}
			continue
		}
		if size, bigEnd, ok := d.rbPackFloat(); ok {
			n := count
			if d.star {
				n = len(rest) / size
			}
			rbUnpackPad(n, len(rest)/size)
			for range n {
				if pos+size > len(s) {
					out.s = append(out.s, nil)
					continue
				}
				u := rbPackGet(s[pos:], size, bigEnd)
				if size == 8 {
					out.s = append(out.s, Float(math.Float64frombits(u)))
				} else {
					out.s = append(out.s, Float(math.Float32frombits(uint32(u))))
				}
				pos += size
			}
			continue
		}
		switch d.d {
		case 'a', 'A', 'Z':
			n := min(count, len(rest))
			v := rest[:n]
			switch {
			case d.d == 'Z' && d.star:
				if k := strings.IndexByte(rest, 0); k >= 0 {
					v, n = rest[:k], k+1
				}
			case d.d == 'Z':
				if k := strings.IndexByte(v, 0); k >= 0 {
					v = v[:k]
				}
			case d.d == 'A':
				v = strings.TrimRight(v, " \x00")
			}
			out.s = append(out.s, String(v))
			pos += n
		case 'x':
			if count > len(rest) {
				panic(NewArgumentError(Ref(String("x outside of string"))))
			}
			pos += count
		case 'X':
			if count > pos {
				panic(NewArgumentError(Ref(String("X outside of string"))))
			}
			pos -= count
		case '@': // `@*` is the remaining length as an offset from the start, as MRI's
			if count > len(s) {
				panic(NewArgumentError(Ref(String("@ outside of string"))))
			}
			pos = count
		case 'U':
			for n := 0; n < count && pos < len(s); n++ {
				r, size := rbUnpackU(s[pos:])
				out.s = append(out.s, Integer(r))
				pos += size
			}
		case 'w':
			var u uint64
			for n := 0; n < count && pos < len(s); pos++ {
				if u > math.MaxInt64>>7 {
					panic(NewRangeError(Ref(String("BER-compressed integer overflows Integer (64-bit; no Bignum)"))))
				}
				u = u<<7 | uint64(s[pos]&0x7f)
				if s[pos]&0x80 == 0 {
					out.s = append(out.s, big(u))
					u = 0
					n++
				}
			}
		case 'H', 'h', 'B', 'b':
			per := 8
			if d.d == 'H' || d.d == 'h' {
				per = 2
			}
			n := count
			if d.star || n > len(rest)*per {
				n = len(rest) * per
			}
			var sb strings.Builder
			for k := range n {
				c := rest[k/per]
				switch d.d {
				case 'H':
					sb.WriteByte("0123456789abcdef"[c>>(4*(1-k%2))&0xf])
				case 'h':
					sb.WriteByte("0123456789abcdef"[c>>(4*(k%2))&0xf])
				case 'B':
					sb.WriteByte('0' + c>>(7-k%8)&1)
				case 'b':
					sb.WriteByte('0' + c>>(k%8)&1)
				}
			}
			out.s = append(out.s, String(sb.String()))
			pos += (n + per - 1) / per
		case 'u':
			v, used := rbUnpackUU(rest)
			out.s = append(out.s, String(v))
			pos += used
		case 'M':
			out.s = append(out.s, String(rbUnpackQP(rest)))
			pos = len(s)
		case 'm':
			if d.counted && d.count == 0 {
				if strings.ContainsAny(rest, "\r\n") {
					panic(NewArgumentError(Ref(String("invalid base64"))))
				}
				v, err := base64.StdEncoding.Strict().DecodeString(rest)
				if err != nil {
					panic(NewArgumentError(Ref(String("invalid base64"))))
				}
				out.s = append(out.s, String(v))
				pos = len(s)
			} else {
				v, used := rbUnpackB64(rest)
				out.s = append(out.s, String(v))
				pos += used
			}
		}
	}
	return out
}

// rbUnpackU is MRI's utf8_to_uv, rbPackU's inverse: up to six bytes, so surrogates and values past U+10FFFF round-trip.
func rbUnpackU(s string) (int, int) {
	c := s[0]
	size := 0
	switch {
	case c < 0x80:
		return int(c), 1
	case c&0x40 == 0:
	case c&0x20 == 0:
		size = 2
	case c&0x10 == 0:
		size = 3
	case c&0x08 == 0:
		size = 4
	case c&0x04 == 0:
		size = 5
	case c&0x02 == 0:
		size = 6
	}
	if size == 0 {
		panic(NewArgumentError(Ref(String("malformed UTF-8 character"))))
	}
	if size > len(s) {
		panic(NewArgumentError(Ref(String("malformed UTF-8 character (expected " + strconv.Itoa(size) + " bytes, given " + strconv.Itoa(len(s)) + " bytes)"))))
	}
	u := int(c) & (0x7F >> size)
	for k := 1; k < size; k++ {
		if s[k]&0xC0 != 0x80 {
			panic(NewArgumentError(Ref(String("malformed UTF-8 character"))))
		}
		u = u<<6 | int(s[k]&0x3F)
	}
	if u < [...]int{0, 0, 0x80, 0x800, 0x10000, 0x200000, 0x4000000}[size] {
		panic(NewArgumentError(Ref(String("redundant UTF-8 sequence"))))
	}
	return u, size
}

// rbUnpackUU is MRI's u decoder: length-prefixed lines while the length byte is in range; used is how far it read.
func rbUnpackUU(s string) (string, int) {
	var b []byte
	i := 0
	val := func() byte {
		if i < len(s) && s[i] >= ' ' && s[i] < 'a' {
			i++
			return (s[i-1] - ' ') & 63
		}
		return 0
	}
	limit := len(s) * 3 / 4 // MRI sizes its buffer by the input and cuts an over-long length byte to it
	for i < len(s) && s[i] > ' ' && s[i] < 'a' {
		n := min(int((s[i]-' ')&63), limit-len(b))
		i++
		for n > 0 {
			a, bb, c, d := val(), val(), val(), val()
			hunk := [3]byte{a<<2 | bb>>4, bb<<4 | c>>2, c<<6 | d}
			k := min(n, 3)
			b = append(b, hunk[:k]...)
			n -= k
		}
		if i < len(s) && s[i] != '\r' && s[i] != '\n' {
			i++ // a checksum byte
		}
		if i < len(s) && s[i] == '\r' {
			i++
		}
		if i < len(s) && s[i] == '\n' {
			i++
		}
	}
	return string(b), i
}

// rbUnpackQP is MRI's M decoder: =XX escapes and soft breaks; a malformed escape keeps the rest as it is.
func rbUnpackQP(s string) string {
	hex := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10
		}
		return -1
	}
	var b []byte
	i, kept := 0, 0
	for i < len(s) {
		if s[i] == '=' {
			if i++; i == len(s) {
				break
			}
			if i+1 < len(s) && s[i] == '\r' && s[i+1] == '\n' {
				i++
			}
			if s[i] != '\n' {
				c1 := hex(s[i])
				if c1 < 0 {
					break
				}
				if i++; i == len(s) {
					break
				}
				c2 := hex(s[i])
				if c2 < 0 {
					break
				}
				b = append(b, byte(c1<<4|c2))
			}
		} else {
			b = append(b, s[i])
		}
		i++
		kept = i
	}
	return string(b) + s[kept:]
}

// rbUnpackB64 is MRI's lenient m decoder: other bytes are skipped and a `=` in a quad's third or fourth place ends it, where the next directive starts.
func rbUnpackB64(s string) (string, int) {
	val := func(c byte) int {
		switch {
		case c >= 'A' && c <= 'Z':
			return int(c - 'A')
		case c >= 'a' && c <= 'z':
			return int(c-'a') + 26
		case c >= '0' && c <= '9':
			return int(c-'0') + 52
		case c == '+':
			return 62
		case c == '/':
			return 63
		}
		return -1
	}
	var b []byte
	var q [4]int
	n, i := 0, 0
	for ; i < len(s) && (s[i] != '=' || n < 2); i++ {
		v := val(s[i])
		if v < 0 {
			continue
		}
		q[n] = v
		if n++; n == 4 {
			b = append(b, byte(q[0]<<2|q[1]>>4), byte(q[1]<<4|q[2]>>2), byte(q[2]<<6|q[3]))
			n = 0
		}
	}
	if n >= 2 {
		b = append(b, byte(q[0]<<2|q[1]>>4))
	}
	if n == 3 {
		b = append(b, byte(q[1]<<4|q[2]>>2))
	}
	return string(b), i
}

// rbPackEscape is MRI's rb_str_escape, which names an unknown directive's format when it holds an unprintable byte.
func rbPackEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0:
			b.WriteString(`\0`)
		case r == 0x7f:
			b.WriteString(`\c?`)
		case strings.ContainsRune("\n\r\t\f\v\b\a\x1b", r):
			b.WriteString(map[rune]string{'\n': `\n`, '\r': `\r`, '\t': `\t`, '\f': `\f`, '\v': `\v`, '\b': `\b`, '\a': `\a`, 0x1b: `\e`}[r])
		case r < ' ' || r > '~':
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
