//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// rbMRIMagic starts MRI's own marshal stream (format 4.8). Marshal.load reads such a stream when it holds only core
// values: nil, true, false, Integer, Float, String, Symbol, Array, Hash and their back-references, which is what a gem's
// data file holds (unicode-display_width's and unicode-emoji's `*.marshal.gz`). A record naming a class (an object, a
// Struct, a subclass, `_dump` data) is not read: dumps stay rugaroo's own (decision 137).
const rbMRIMagic = "\x04\x08"

// rbMarshalIsMRI reports whether head starts an MRI stream.
func rbMarshalIsMRI(head any) bool {
	h, ok := rbUnbox(head).(String)
	return ok && strings.HasPrefix(string(h), rbMRIMagic)
}

// rbMRIReader is one MRI stream: its symbol table (`:` adds, `;` refers) and its object table (`@` refers; every
// String, Float, Array and Hash is numbered as its record starts, as MRI numbers them, so copies keep the indices).
type rbMRIReader struct {
	b     []byte
	syms  []Symbol
	refs  []any
	depth int
}

func rbMarshalLoadMRI(data []byte) any {
	r := &rbMRIReader{b: data[len(rbMRIMagic):]}
	return r.value()
}

func (r *rbMRIReader) byte() byte {
	if len(r.b) == 0 {
		panic(rbMarshalShort())
	}
	c := r.b[0]
	r.b = r.b[1:]
	return c
}

// long is MRI's w_long: one signed byte encodes a small value (0, 5..127 as c-5, -128..-5 as c+5) or the count of
// little-endian bytes that follow (1..4 positive, -1..-4 negative, sign-extended).
func (r *rbMRIReader) long() int64 {
	c := int8(r.byte())
	switch {
	case c == 0:
		return 0
	case c >= 5:
		return int64(c) - 5
	case c <= -5:
		return int64(c) + 5
	case c > 0:
		var x int64
		for i := range int(c) {
			x |= int64(r.byte()) << (8 * i)
		}
		return x
	default:
		x := int64(-1)
		for i := range int(-c) {
			x &^= 0xff << (8 * i)
			x |= int64(r.byte()) << (8 * i)
		}
		return x
	}
}

func (r *rbMRIReader) bytes() []byte {
	n := r.long()
	if n < 0 || n > int64(len(r.b)) {
		panic(rbMarshalShort())
	}
	s := r.b[:n]
	r.b = r.b[n:]
	return s
}

// symbol reads a symbol record: `:` names a new one, `;` refers to one, `I:` is a new one with its encoding ivars.
func (r *rbMRIReader) symbol(c byte) Symbol {
	switch c {
	case ':':
		s := Symbol(r.bytes())
		r.syms = append(r.syms, s)
		return s
	case ';':
		i := r.long()
		if i < 0 || i >= int64(len(r.syms)) {
			panic(rbMarshalBad("symbol link"))
		}
		return r.syms[i]
	case 'I':
		s := r.symbol(r.byte())
		r.ivars()
		return s
	default:
		panic(rbMarshalBad(fmt.Sprintf("symbol record 0x%x", c)))
	}
}

// ivars reads the `I` wrapper's instance variables and drops them: `:E`/`:encoding` name a String's encoding, which
// rugaroo keeps only at compile time (decision 352).
func (r *rbMRIReader) ivars() {
	for range r.long() {
		r.symbol(r.byte())
		r.value()
	}
}

func (r *rbMRIReader) reg(v any) { r.refs = append(r.refs, v) }

func (r *rbMRIReader) value() any {
	r.depth++
	defer func() { r.depth-- }()
	if r.depth > rbMarshalMaxDepth {
		panic(NewArgumentError(Ref(String("exceed depth limit"))))
	}
	switch c := r.byte(); c {
	case '0':
		return nil
	case 'T':
		return Boolean(true)
	case 'F':
		return Boolean(false)
	case 'i':
		return Integer(r.long())
	case ':', ';':
		return r.symbol(c)
	case 'I':
		v := r.value()
		r.ivars()
		return v
	case '"':
		s := String(r.bytes())
		r.reg(s)
		return s
	case 'f':
		return r.float()
	case 'l':
		return r.bignum()
	case '[':
		return r.array()
	case '{', '}':
		return r.hash(c == '}')
	case '@':
		i := r.long()
		if i < 0 || i >= int64(len(r.refs)) {
			panic(rbMarshalBad("unlinked"))
		}
		return r.refs[i]
	default:
		panic(NewTypeError(Ref(String(fmt.Sprintf("incompatible marshal file format (can't be read)\n\tMRI's marshal data holds a %q record; rugaroo reads only its own format and MRI's core values (decision 137)", c)))))
	}
}

func (r *rbMRIReader) float() any {
	s := string(r.bytes())
	var f float64
	switch s {
	case "inf":
		f = math.Inf(1)
	case "-inf":
		f = math.Inf(-1)
	case "nan":
		f = math.NaN()
	default:
		var err error
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			panic(rbMarshalBad("float " + s))
		}
	}
	r.reg(Float(f))
	return Float(f)
}

// bignum is a value past MRI's fixnums (62 bits): sign, length in 16-bit words, little-endian bytes. One that fits
// 64 bits is an Integer; past that rugaroo's default lane has no value (decision 372), so it is MRI's RangeError.
func (r *rbMRIReader) bignum() any {
	sign := r.byte()
	n := r.long()
	if n < 0 || 2*n > int64(len(r.b)) {
		panic(rbMarshalShort())
	}
	le := r.b[:2*n]
	r.b = r.b[2*n:]
	be := make([]byte, len(le))
	for i, b := range le {
		be[len(le)-1-i] = b
	}
	v := new(big.Int).SetBytes(be)
	if sign == '-' {
		v.Neg(v)
	}
	if !v.IsInt64() { // ponytail: promote lane could hold it as a Bignum (rbBigOf)
		panic(NewRangeError(Ref(String("bignum too big to convert into 'long'"))))
	}
	r.reg(Integer(v.Int64()))
	return Integer(v.Int64())
}

func (r *rbMRIReader) array() any {
	a := &Array[any]{}
	r.reg(a)
	n := r.long()
	if n < 0 || n > int64(len(r.b)) {
		panic(rbMarshalShort())
	}
	a.s = make([]any, 0, n)
	for range n {
		a.s = append(a.s, r.value())
	}
	return a
}

func (r *rbMRIReader) hash(withDefault bool) any {
	h := NewHash[any, any]()
	r.reg(h)
	n := r.long()
	if n < 0 || n > int64(len(r.b)) {
		panic(rbMarshalShort())
	}
	for range n {
		k := r.value()
		Hash_Op_idxSet(h, k, r.value())
	}
	if withDefault {
		h.hasDef = true
		h.defVal = r.value()
	}
	return h
}
