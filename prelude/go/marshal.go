//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"
	"time"
)

// rugaroo's own stream, never MRI's (format in decision 137): magic, payload length (lets load read one dump off an IO), one value.
const rbMarshalMagic = "RUGAROO\x04\x08"

// rbKeyed is k; as a switch case it names T weakly, so the pruner drops the case when nothing else keeps T (decision 137).
func rbKeyed[T any](k string) string { return k }

// rbMW is one dump: reference values written so far (by identity, for '@') and depth left (never negative: no limit means rbMarshalMaxDepth).
type rbMW struct {
	buf   []byte
	refs  map[any]int
	limit int
}

func rbMarshalDump(v any, limit int) String {
	const head = len(rbMarshalMagic) + 8
	if limit < 0 || limit > rbMarshalMaxDepth {
		limit = rbMarshalMaxDepth
	}
	w := &rbMW{buf: make([]byte, head, 64), refs: map[any]int{}, limit: limit}
	copy(w.buf, rbMarshalMagic)
	w.value(v)
	binary.LittleEndian.PutUint64(w.buf[len(rbMarshalMagic):], uint64(len(w.buf)-head))
	return String(w.buf)
}

func (w *rbMW) str(s string) {
	w.buf = binary.AppendUvarint(w.buf, uint64(len(s)))
	w.buf = append(w.buf, s...)
}

func (w *rbMW) count(n int) { w.buf = binary.AppendUvarint(w.buf, uint64(n)) }

// begin starts a reference value: a back-reference when v was written already (false), else its record.
func (w *rbMW) begin(v any, kind byte, tag string) bool {
	if i, ok := w.refs[v]; ok {
		w.buf = append(w.buf, '@')
		w.count(i)
		return false
	}
	w.refs[v] = len(w.refs)
	w.record(kind, tag)
	return true
}

func (w *rbMW) start(isNil bool, v any, tag string) bool {
	if isNil {
		w.buf = append(w.buf, '0')
		return false
	}
	return w.begin(v, 'o', tag)
}

func (w *rbMW) record(kind byte, tag string) {
	w.buf = append(w.buf, kind)
	w.str(tag)
}

func (w *rbMW) value(v any) {
	if w.limit == 0 {
		panic(NewArgumentError(Ref(String("exceed depth limit"))))
	}
	w.limit--
	defer func() { w.limit++ }()
	switch x := rbUnbox(v).(type) {
	case nil:
		w.buf = append(w.buf, '0')
	case Boolean:
		if x {
			w.buf = append(w.buf, 'T')
		} else {
			w.buf = append(w.buf, 'F')
		}
	case Integer:
		w.buf = append(w.buf, 'i')
		w.buf = binary.AppendVarint(w.buf, int64(x))
	case Float:
		w.buf = append(w.buf, 'f')
		w.buf = binary.LittleEndian.AppendUint64(w.buf, math.Float64bits(float64(x)))
	case String:
		w.buf = append(w.buf, '"')
		w.str(string(x))
	case Symbol:
		w.buf = append(w.buf, ':')
		w.str(string(x))
	default:
		if !rbMDumpCore(w, x) && !rbMDumpGen(w, x) && !rbMDumpObjGen(w, x) && !rbMDumpAnyForm(w, x) {
			panic(rbMarshalUndumpable(x))
		}
	}
}

// rbMarshalUndumpable words MRI's TypeError: "can't dump" for IO and the thread queues, "no _dump_data" for the rest.
func rbMarshalUndumpable(v any) any {
	name := rbClassName(v)
	switch name {
	case "Queue", "SizedQueue", "ConditionVariable": // MRI's Thread:: names, which rugaroo's classes lack
		return NewTypeError(Ref(String("can't dump Thread::" + name)))
	case "Mutex":
		name = "Thread::Mutex"
	case "IO", "File":
		return NewTypeError(Ref(String("can't dump " + name)))
	}
	return NewTypeError(Ref(String("no _dump_data is defined for class " + name)))
}

// rbMDumpCore writes the core classes whose layout is Go's; each case keeps its type only weakly.
func rbMDumpCore(w *rbMW, v any) bool {
	// each case returns: the pruner can drop every one, and a `return true` after the switch would then be unreachable (go vet)
	switch x := v.(type) {
	case nil:
		return false
	case *Time:
		if w.start(x == nil, x, "Time") {
			name, off := x.t.Zone()
			w.buf = binary.AppendVarint(w.buf, x.t.Unix())
			w.buf = binary.AppendVarint(w.buf, int64(x.t.Nanosecond()))
			w.value(Boolean(x.utc))
			w.value(Boolean(x.t.Location() == time.Local))
			w.str(name)
			w.buf = binary.AppendVarint(w.buf, int64(off))
		}
		return true
	case *Rational:
		if w.start(x == nil, x, "Rational") {
			w.str(x.v.RatString())
		}
		return true
	case *Complex:
		if w.start(x == nil, x, "Complex") {
			w.value(x.re)
			w.value(x.im)
		}
		return true
	case *BigDecimal:
		if w.start(x == nil, x, "BigDecimal") {
			mant := ""
			if x.mant != nil {
				mant = x.mant.String()
			}
			w.str(mant)
			w.buf = binary.AppendVarint(w.buf, int64(x.exp))
			w.value(Boolean(x.neg))
			w.buf = binary.AppendVarint(w.buf, int64(x.kind))
		}
		return true
	case *Regexp:
		if w.start(x == nil, x, "Regexp") {
			w.str(x.src)
			w.str(x.opts)
		}
		return true
	case *Random:
		if w.start(x == nil, x, "Random") {
			x.mt.mu.Lock()
			w.buf = binary.AppendVarint(w.buf, int64(x.seed))
			w.buf = binary.AppendVarint(w.buf, int64(x.mt.i))
			for _, s := range x.mt.s {
				w.buf = binary.AppendVarint(w.buf, int64(s))
			}
			x.mt.mu.Unlock()
		}
		return true
	default:
		return false
	}
}

// rbMDumpAnyForm: an instantiation only Go code builds (JSON.parse's) has no case, so it loads back untyped; v stays the identity.
func rbMDumpAnyForm(w *rbMW, v any) bool {
	// each case returns: the pruner can drop every one, and a `return true` after the switch would then be unreachable (go vet)
	switch x := v.(type) {
	case nil:
		return false
	case Array_Any:
		rbMDumpArray(w, "Array[any]", v, x._ToAny())
		return true
	case Hash_Any:
		rbMDumpHash(w, "Hash[any, any]", v, x._ToAny())
		return true
	case Set_Any:
		rbMDumpSet(w, "Set[any]", v, x._ToAny())
		return true
	case Range_Any:
		rbMDumpRange(w, "Range[any]", v, x._ToAny())
		return true
	default:
		return false
	}
}

// rbMDumpObj writes a struct class's assigned ivars by name (decision 123's _Ivars).
func rbMDumpObj(w *rbMW, tag string, v any) {
	if !w.begin(v, 'o', tag) {
		return
	}
	o, ok := v.(interface{ _Ivars(name string) []rbIvar })
	if !ok {
		w.count(0)
		return
	}
	var set []rbIvar
	for _, iv := range o._Ivars("") {
		if iv.opt || iv.val != nil && !iv.isNil {
			set = append(set, iv)
		}
	}
	w.count(len(set))
	for _, iv := range set {
		w.str(iv.name)
		w.value(iv.val)
	}
}

// The container dumps take id apart from the value: rbMDumpAnyForm's _ToAny copy must keep the original's identity.
func rbMDumpArray[E comparable](w *rbMW, tag string, id any, a *Array[E]) {
	if !w.start(a == nil, id, tag) {
		return
	}
	w.count(len(a.s))
	for _, x := range a.s {
		w.value(any(x))
	}
}

func rbMDumpHash[K, V comparable](w *rbMW, tag string, id any, h *Hash[K, V]) {
	if !w.start(h == nil, id, tag) {
		return
	}
	w.count(int(h.live))
	for k, hv := range rbHashPairs(h) {
		w.value(any(k))
		w.value(any(hv))
	}
}

func rbMDumpSet[E comparable](w *rbMW, tag string, id any, s *Set[E]) {
	if !w.start(s == nil, id, tag) {
		return
	}
	w.count(int(s.h.live))
	for k := range rbHashPairs(s.h) {
		w.value(any(k))
	}
}

func rbMDumpRange[E comparable](w *rbMW, tag string, id any, r *Range[E]) {
	if !w.start(r == nil, id, tag) {
		return
	}
	w.value(any(r.b))
	w.value(any(r.e))
	w.value(Boolean(r.excl))
	w.value(Boolean(r.endless))
	w.value(Boolean(r.beginless))
}

// rbMDumpTuple: a tuple is a value, so it takes no reference index.
func rbMDumpTuple(w *rbMW, tag string, xs ...any) {
	w.record('v', tag)
	w.count(len(xs))
	for _, x := range xs {
		w.value(x)
	}
}

// rbMDumpHook writes v through its marshal_dump; the object is registered first, as MRI does, so data may refer back to it.
func rbMDumpHook(w *rbMW, tag string, v any, data func() any) {
	if w.begin(v, 'u', tag) {
		w.value(data())
	}
}

// rbMR is one load: the unread bytes, the reference values read so far, and how deep value is nested.
type rbMR struct {
	b     []byte
	refs  []any
	depth int
}

// rbMarshalMaxDepth bounds dump's and load's recursion alike: a Go stack overflow is fatal and no rescue catches it (MRI's SystemStackError is catchable), so hostile nesting is an ArgumentError instead. Measured ~2.6 KB of peak memory per level (stack plus the nested values) for Array, Hash, Object and Struct chains, against Go's 1 GB stack limit; MRI itself gives up past ~20k-50k levels.
const rbMarshalMaxDepth = 100000

// rbMarshalBad is the error for decoded state no dump of this program holds.
func rbMarshalBad(what string) any {
	return NewArgumentError(Ref(String("dump format error (" + what + ")")))
}

func rbMarshalShort() any { return NewArgumentError(Ref(String("marshal data too short"))) }

// rbMarshalBodySize checks a dump's header and answers its payload's length.
func rbMarshalBodySize(head any) int {
	h, ok := rbUnbox(head).(String)
	if !ok && head != nil {
		panic(NewTypeError(Ref(String("instance of IO needed"))))
	}
	if !strings.HasPrefix(string(h), rbMarshalMagic) {
		if !strings.HasPrefix(rbMarshalMagic, string(h)) {
			panic(NewTypeError(Ref(String("incompatible marshal file format (can't be read)\n\tformat RUGAROO 4.8 required"))))
		}
		panic(rbMarshalShort())
	}
	if len(h) < len(rbMarshalMagic)+8 {
		panic(rbMarshalShort())
	}
	n := binary.LittleEndian.Uint64([]byte(h[len(rbMarshalMagic):]))
	if n > math.MaxInt32 {
		panic(NewArgumentError(Ref(String("marshal data too long"))))
	}
	return int(n)
}

// rbMarshalExactIO: these read exactly n bytes, so dumps written one after another load one at a time, as MRI's do; another IO's read reads to its end.
func rbMarshalExactIO(src any) bool {
	switch src.(type) {
	case nil:
		return false
	case *File:
		return true
	case *IO:
		return true
	case interface{ __Read1(n Integer) *String }:
		return true
	default:
		return false
	}
}

// rbMarshalReadN: the length comes from the data, so a File reads through a LimitReader rather than allocating it up front.
func rbMarshalReadN(src any, n int) String {
	switch s := src.(type) {
	case nil:
		return ""
	case *File:
		s.rbReadable()
		b, _ := io.ReadAll(io.LimitReader(s.r, int64(n)))
		return String(b)
	case *IO:
		b, _ := io.ReadAll(io.LimitReader(*s.rbReader(), int64(n)))
		return String(b)
	case interface{ __Read1(n Integer) *String }:
		var b strings.Builder
		for b.Len() < n {
			p := s.__Read1(Integer(n - b.Len()))
			if p == nil || *p == "" {
				break
			}
			b.WriteString(string(*p))
		}
		return String(b.String())
	default:
		return ""
	}
}

// rbMarshalLoadIO: an IO at its end is MRI's EOFError, which loops reading dumps until EOF rescue.
func rbMarshalLoadIO(src any) any {
	head := rbMarshalReadN(src, len(rbMarshalMagic)+8)
	if head == "" {
		panic(NewEOFError(Ref(String("end of file reached"))))
	}
	if rbMarshalIsMRI(head) { // ponytail: MRI's stream carries no length, so an IO holding one dump is read to its end
		return rbMarshalLoad(head, rbMarshalReadN(src, math.MaxInt32))
	}
	return rbMarshalLoad(head, rbMarshalReadN(src, rbMarshalBodySize(head)))
}

// rbMarshalLoad: from an IO, the body is read separately once the header gives its length.
func rbMarshalLoad(head, body any) any {
	if rbMarshalIsMRI(head) {
		data := []byte(rbUnbox(head).(String))
		if b, ok := rbUnbox(body).(String); ok {
			data = append(data, b...)
		}
		return rbMarshalLoadMRI(data)
	}
	n := rbMarshalBodySize(head)
	data := []byte(rbUnbox(head).(String)[len(rbMarshalMagic)+8:])
	if b, ok := rbUnbox(body).(String); ok {
		data = append(data, b...)
	}
	if len(data) < n {
		panic(rbMarshalShort())
	}
	r := &rbMR{b: data[:n]}
	return r.value()
}

func (r *rbMR) byte() byte {
	if len(r.b) == 0 {
		panic(rbMarshalShort())
	}
	c := r.b[0]
	r.b = r.b[1:]
	return c
}

func (r *rbMR) count() int {
	n, k := binary.Uvarint(r.b)
	if k <= 0 || n > uint64(len(r.b)) {
		panic(rbMarshalShort())
	}
	r.b = r.b[k:]
	return int(n)
}

func (r *rbMR) varint() int64 {
	n, k := binary.Varint(r.b)
	if k <= 0 {
		panic(rbMarshalShort())
	}
	r.b = r.b[k:]
	return n
}

func (r *rbMR) str() string {
	n := r.count()
	s := string(r.b[:n])
	r.b = r.b[n:]
	return s
}

// reg must run as a record starts, before its contents, which may refer back to it.
func (r *rbMR) reg(v any) { r.refs = append(r.refs, v) }

func (r *rbMR) bool() bool { return rbTruthy(r.value()) }

func (r *rbMR) value() any {
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
		return Integer(r.varint())
	case 'f':
		if len(r.b) < 8 {
			panic(rbMarshalShort())
		}
		f := math.Float64frombits(binary.LittleEndian.Uint64(r.b))
		r.b = r.b[8:]
		return Float(f)
	case '"':
		return String(r.str())
	case ':':
		return Symbol(r.str())
	case '@':
		i := r.count()
		if i >= len(r.refs) {
			panic(NewArgumentError(Ref(String("dump format error (unlinked)"))))
		}
		return r.refs[i]
	case 'o', 'u', 'v', 'c':
		tag := r.str()
		key := string(rune(c)) + tag
		if v, ok := rbMLoadCore(r, key); ok {
			return v
		}
		if v, ok := rbMLoadGen(r, key); ok {
			return v
		}
		if v, ok := rbMLoadObjGen(r, key); ok {
			return v
		}
		panic(NewArgumentError(Ref(String("undefined class/module " + tag))))
	default:
		panic(NewArgumentError(Ref(String(fmt.Sprintf("dump format error(0x%x)", c)))))
	}
}

func rbMLoadCore(r *rbMR, key string) (any, bool) {
	switch key {
	case "":
		return nil, false
	case rbKeyed[*Time]("oTime"):
		x := &Time{}
		r.reg(x)
		sec, nsec := r.varint(), r.varint()
		utc, local := r.bool(), r.bool()
		name, off := r.str(), r.varint()
		t := time.Unix(sec, nsec)
		switch {
		case utc:
			t = t.UTC()
		case local:
			t = t.In(time.Local)
		default:
			t = t.In(time.FixedZone(name, int(off)))
		}
		x.t, x.utc = t, utc
		return x, true
	case rbKeyed[*Rational]("oRational"):
		x := &Rational{}
		r.reg(x)
		if _, ok := x.v.SetString(r.str()); !ok {
			panic(NewArgumentError(Ref(String("marshal data holds a bad Rational"))))
		}
		return x, true
	case rbKeyed[*Complex]("oComplex"):
		x := &Complex{}
		r.reg(x)
		x.re = r.value()
		x.im = r.value()
		return x, true
	case rbKeyed[*BigDecimal]("oBigDecimal"):
		x := &BigDecimal{}
		r.reg(x)
		x.mant = new(big.Int) // every BigDecimal method reads mant, so it is never nil
		if m := r.str(); m != "" {
			if _, ok := x.mant.SetString(m, 10); !ok || x.mant.Sign() < 0 {
				panic(rbMarshalBad("BigDecimal digits"))
			}
		}
		x.exp = int(r.varint())
		if x.exp > rbBDExpMax || x.exp < -rbBDExpMax {
			panic(rbMarshalBad("BigDecimal exponent"))
		}
		x.neg = r.bool()
		x.kind = int(r.varint())
		if x.kind < rbBDFinite || x.kind > rbBDNaN {
			panic(rbMarshalBad("BigDecimal kind"))
		}
		return x, true
	case rbKeyed[*Regexp]("oRegexp"):
		i := len(r.refs)
		r.reg(nil) // compiled once its source is read: nothing inside a Regexp refers back to it
		src, opts := r.str(), r.str()
		x := rbRegexpFromValue(String(src), String(opts))
		r.refs[i] = x
		return x, true
	case rbKeyed[*Random]("oRandom"):
		x := &Random{}
		r.reg(x)
		x.seed = Integer(r.varint())
		x.mt.i = int(r.varint())
		if x.mt.i < 0 || x.mt.i > len(x.mt.s) { // next() indexes s[i], and regenerates from i == len(s)
			panic(rbMarshalBad("Random state"))
		}
		for k := range x.mt.s {
			x.mt.s[k] = uint32(r.varint())
		}
		return x, true
	default:
		return nil, false
	}
}

// rbMLoadObj goes through decision 123's _IvarSet, which converts each value to the ivar's type.
func rbMLoadObj(r *rbMR, v any) any {
	r.reg(v)
	n := r.count()
	o, ok := v.(interface{ _IvarSet(string, any) bool })
	for range n {
		name := r.str()
		x := r.value()
		if !ok || !o._IvarSet(name, x) {
			panic(NewTypeError(Ref(String("rugaroo: marshal data gives " + rbClassName(v) + " an instance variable " + name + " it does not have"))))
		}
	}
	return v
}

// rbMAs: anything rbMBox and rbConv cannot convert is data from another program.
func rbMAs[T any](v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	var zero T
	if v == nil || rbMBox(&zero, v) {
		return zero
	}
	if t, ok := rbConv[T](v); ok {
		return t
	}
	panic(NewTypeError(Ref(String("rugaroo: marshal data holds " + rbClassName(v) + " where this program has another type"))))
}

func rbMLoadArray[E comparable](r *rbMR) *Array[E] {
	a := &Array[E]{}
	r.reg(a)
	n := r.count()
	a.s = make([]E, 0, n)
	for range n {
		a.s = append(a.s, rbMAs[E](r.value()))
	}
	return a
}

func rbMLoadHash[K, V comparable](r *rbMR) *Hash[K, V] {
	h := NewHash[K, V]()
	r.reg(h)
	for range r.count() {
		k := rbMAs[K](r.value())
		Hash_Op_idxSet(h, k, rbMAs[V](r.value()))
	}
	return h
}

func rbMLoadSet[E comparable](r *rbMR) *Set[E] {
	s := NewSet[E]()
	r.reg(s)
	for range r.count() {
		Hash_Op_idxSet(s.h, rbMAs[E](r.value()), true)
	}
	return s
}

func rbMLoadRange[E comparable](r *rbMR) *Range[E] {
	x := &Range[E]{}
	r.reg(x)
	x.b = rbMAs[E](r.value())
	x.e = rbMAs[E](r.value())
	x.excl, x.endless, x.beginless = r.bool(), r.bool(), r.bool()
	return x
}

func rbMTupleItems(r *rbMR, n int) []any {
	if r.count() != n {
		panic(NewTypeError(Ref(String("rugaroo: marshal data holds a tuple of another size"))))
	}
	xs := make([]any, n)
	for i := range xs {
		xs[i] = r.value()
	}
	return xs
}
