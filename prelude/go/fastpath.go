//go:build rugaroo_prelude

package prelude

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"
)

// rbBDStripZeros is a copy of the nonzero mant with its k trailing decimal zeros removed.
func rbBDStripZeros(mant *big.Int) (*big.Int, int) {
	tz := mant.TrailingZeroBits() // 10^k divides mant only if 2^k does
	if tz == 0 {
		return new(big.Int).Set(mant), 0
	}
	m := new(big.Int).Set(mant)
	if tz > 16 { // many zeros are possible: count them in the decimal string, divide once
		s := mant.String()
		k := len(s) - len(strings.TrimRight(s, "0"))
		if k == 0 {
			return m, 0
		}
		return m.Quo(m, new(big.Int).Exp(rbBDTen, big.NewInt(int64(k)), nil)), k
	}
	r := new(big.Int)
	k := 0
	for {
		q, rem := new(big.Int).QuoRem(m, rbBDTen, r)
		if rem.Sign() != 0 {
			return m, k
		}
		m, k = q, k+1
	}
}

// rbReadUntil reads through the next want (to EOF when none); the separator's last byte is the only place a match can end.
func rbReadUntil(r *bufio.Reader, want []byte) []byte {
	if len(want) == 0 {
		out, _ := io.ReadAll(r)
		return out
	}
	last := want[len(want)-1]
	var out []byte
	for {
		chunk, err := r.ReadSlice(last)
		out = append(out, chunk...)
		if err == nil && bytes.HasSuffix(out, want) || err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return out
		}
	}
}

// rbStrFact is what MRI keeps on a String as its coderange and length: computed once for a large String, then read
// again by every index, slice, length and match against the same bytes (decision 464).
type rbStrFact struct {
	p     *byte // the key: the String's data pointer and length. Holding p keeps the bytes from being reused.
	n     int
	level int // how much is known: rbFactASCII, then valid, then chars. An ASCII String knows all three.
	ascii bool
	valid bool // valid UTF-8
	chars int  // its length in characters: bytes unless valid and not ASCII
}

const (
	rbFactASCII = iota
	rbFactValid
	rbFactChars
)

// rbStrFactMin is the length from which a String's facts are cached; a shorter one costs less to scan than to look up.
const rbStrFactMin = 256

// rbStrFacts is a direct-mapped cache by length: whole immutable entries swapped atomically, so no lock; a
// collision only rescans, and at most 16 Strings are kept alive by it. Go strings never change in place except
// under rbStrSetbyte, which clears its slot.
var rbStrFacts [16]atomic.Pointer[rbStrFact]

// rbStrMutable is set when rbStrSetbyte first gives a long String a buffer it writes in place (decision 395). From
// then on no fact is cached: a cached one could describe bytes a later setbyte changed, and testing for that on every
// setbyte cost a byte-twiddling loop 7%. Entries cached before stay valid: they are of other, immutable Strings, and
// holding their bytes keeps a new buffer from taking their address.
var rbStrMutable atomic.Bool

func rbStrFactSlot(n int) *atomic.Pointer[rbStrFact] { return &rbStrFacts[(n^n>>4)&15] }

// rbStrFactOf is s's facts up to level; len(s) >= rbStrFactMin. A non-ASCII String is validated and counted only
// when asked, as its ASCII scan stops at the first high byte.
//
//go:noinline
func rbStrFactOf(s string, level int) *rbStrFact {
	p := unsafe.StringData(s) //nolint:gosec // the key: compared only
	slot := rbStrFactSlot(len(s))
	e := slot.Load()
	if e != nil && e.p == p && e.n == len(s) {
		if e.level >= level {
			return e
		}
		c := *e
		e = &c
	} else {
		e = &rbStrFact{p: p, n: len(s), level: rbFactASCII, chars: len(s)}
		if e.ascii = rbASCII(s); e.ascii {
			e.valid, e.level = true, rbFactChars
		}
	}
	if e.level < level {
		if e.level < rbFactValid {
			e.valid = utf8.ValidString(s)
		}
		if level == rbFactChars && e.valid {
			e.chars = utf8.RuneCountInString(s)
		}
		e.level = level
	}
	if !rbStrMutable.Load() {
		slot.Store(e)
	}
	return e
}

// rbValidUTF8 is utf8.ValidString with MRI's cached coderange for the large subject a loop matches again and again.
func rbValidUTF8(s string) bool {
	if len(s) < rbStrFactMin {
		return utf8.ValidString(s)
	}
	return rbStrFactOf(s, rbFactValid).valid
}

// rbCharOffset is the byte offset of s's character k (k <= its length), and whether s has that many characters:
// k itself in an ASCII String.
func rbCharOffset(s string, k int) (int, bool) {
	if k <= len(s) && rbASCIIFact(s) {
		return k, true
	}
	return rbRuneOffset(s, k)
}

// rbCharCount is how many characters s[:b] holds, b a character boundary of the valid UTF-8 s.
func rbCharCount(s string, b int) int {
	if rbASCIIFact(s) {
		return b
	}
	return utf8.RuneCountInString(s[:b])
}

// rbFormatFast is rbFormatOne for the flag-free %d, %s and %f with only a width (and a precision on %f): strconv and a space pad, no fmt parse. ok is false for everything else.
func rbFormatFast(spec string, verb byte, v any) (string, bool) {
	if verb != 'd' && verb != 'i' && verb != 'u' && verb != 's' && verb != 'f' {
		return "", false
	}
	i, width := 1, 0
	if i < len(spec) && spec[i] >= '1' && spec[i] <= '9' { // a leading 0 is the zero flag, not a width
		for ; i < len(spec) && spec[i] >= '0' && spec[i] <= '9'; i++ {
			width = width*10 + int(spec[i]-'0')
			if width > 4096 {
				return "", false
			}
		}
	}
	prec, hasPrec := 0, false
	if i < len(spec) && spec[i] == '.' {
		for i++; i < len(spec) && spec[i] >= '0' && spec[i] <= '9'; i++ {
			prec = prec*10 + int(spec[i]-'0')
			if prec > 4096 {
				return "", false
			}
		}
		hasPrec = true
	}
	if i != len(spec) || hasPrec && verb != 'f' {
		return "", false
	}
	var s string
	switch verb {
	case 'f':
		if x, ok := v.(Integer); ok && (x >= 1<<53 || x <= -1<<53) { // MRI prints an Integer's digits exactly (rbFormatOne)
			return "", false
		}
		f := rbFormatFloat(v)
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return "", false
		}
		if !hasPrec {
			prec = 6
		}
		s = rbFmtFloatSpec(f, 'f', "%."+strconv.Itoa(prec)) // MRI's digits (decision 379)
	case 's':
		s = string(rbToS(v))
		if n := utf8.RuneCountInString(s); n < width {
			return strings.Repeat(" ", width-n) + s, true
		}
		return s, true
	default:
		s = strconv.Itoa(rbFormatInt(v))
	}
	if len(s) < width {
		s = strings.Repeat(" ", width-len(s)) + s
	}
	return s, true
}
