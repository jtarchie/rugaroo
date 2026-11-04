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

// rbValidCache is the last large subject found valid UTF-8; holding the string keeps its memory from being reused, and an atomic pointer to an immutable entry needs no lock.
var rbValidCache atomic.Pointer[string]

// rbValidUTF8 is utf8.ValidString with MRI's cached coderange for the large subject a loop matches again and again: == short-circuits on the same bytes (pointer-equal) and on differing lengths.
// The cache's store is out of line: taking &s moves s to the heap at entry, which in one function cost every short
// String a 16-byte allocation that never reached the cache (decision 349).
func rbValidUTF8(s string) bool {
	if len(s) < 512 {
		return utf8.ValidString(s)
	}
	return rbValidUTF8Long(s)
}

//go:noinline
func rbValidUTF8Long(s string) bool {
	if p := rbValidCache.Load(); p != nil && *p == s {
		return true
	}
	if !utf8.ValidString(s) {
		return false
	}
	rbValidCache.Store(&s)
	return true
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
