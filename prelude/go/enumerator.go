//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
)

type Enumerator_Map_Any interface{ _ToAny() *Enumerator_Map[any] }

type Enumerator_Select_Any interface {
	_ToAny() *Enumerator_Select[any]
}

// rbExt is an enumerator's external iteration (next/peek/rewind): iter.Pull over its sequence, started on the first next. A pull never finished nor rewound leaves its coroutine parked until exit (decision 140).
type rbExt[E comparable] struct {
	pull    func() (E, bool)
	stop    func()
	peeked  bool
	val     E
	done    bool
	started bool
}

func (x *rbExt[E]) fill(seq func(func(E) bool), result func() any) {
	if x.peeked {
		return
	}
	if !x.done {
		if !x.started {
			x.pull, x.stop = iter.Pull(iter.Seq[E](seq))
			x.started = true
		}
		v, ok := x.pull()
		if ok {
			x.val, x.peeked = v, true
			return
		}
		x.done = true
	}
	panic(rbStopIteration(result()))
}

func (x *rbExt[E]) next(seq func(func(E) bool), result func() any) E {
	x.fill(seq, result)
	x.peeked = false
	return x.val
}

func (x *rbExt[E]) peek(seq func(func(E) bool), result func() any) E {
	x.fill(seq, result)
	return x.val
}

func (x *rbExt[E]) rewind() {
	if x.stop != nil {
		x.stop()
	}
	*x = rbExt[E]{}
}

// rbStopIteration is what next raises at the end: MRI's message, the iteration's return value as result.
func rbStopIteration(result any) *StopIteration {
	e := NewStopIteration(Ref(String("iteration reached an end")))
	e.__SetResult(result)
	return e
}

// rbEnumOf wraps a sequence as an Enumerator; recv and meth are what inspect shows.
func rbEnumOf[E comparable](seq func(func(E) bool), recv any, meth string, size func() *Integer, result any) *Enumerator[E] {
	e := &Enumerator[E]{seq: seq, recv: recv, meth: meth, size: size}
	if result != nil {
		e.res = &result
	}
	return e
}

// rbEnumNew is Enumerator.new: the block runs on each iteration, feeding a Yielder; a consumer that stops early unwinds it (rbSeq's rbStop). The block is void, so any statement may end it (decision 140), and StopIteration#result is nil.
func rbEnumNew[E comparable](size *Integer, blk func(*Enumerator_Yielder[E])) *Enumerator[E] {
	e := &Enumerator[E]{meth: "each"}
	e.recv = rbEnumGenerator{}
	e.seq = rbSeq(func(f func(E)) { blk(&Enumerator_Yielder[E]{fn: f}) })
	if size != nil {
		n := *size
		e.size = func() *Integer { return &n }
	}
	return e
}

// rbEnumGenerator stands for MRI's Enumerator::Generator in inspect.
type rbEnumGenerator struct{}

func (g rbEnumGenerator) Inspect() String {
	return String(fmt.Sprintf("#<Enumerator::Generator:%p>", &g))
}

func rbEnumResult(res *any, recv any) any {
	if res != nil {
		return *res
	}
	if _, ok := recv.(rbEnumGenerator); ok {
		return nil
	}
	return recv
}

func rbEnumInspect(recv any, meth string) String {
	return "#<Enumerator: " + rbInspect(recv) + ":" + String(meth) + ">"
}

// rbWithIndex pairs each element with its index from offset.
func rbWithIndex[E comparable](seq func(func(E) bool), offset Integer) func(func(Tuple2[E, Integer]) bool) {
	return func(yield func(Tuple2[E, Integer]) bool) {
		i := offset
		for x := range seq {
			if !yield(Tuple2[E, Integer]{x, i}) {
				return
			}
			i++
		}
	}
}

// rbZipTake is MRI's take_items for `xs.zip(ys) { }`: ys's first len(xs) elements, copied when the call starts (decision 389).
func rbZipTake[E, U comparable](xs []E, ys []U) []U {
	return slices.Clone(ys[:min(len(xs), len(ys))])
}

// rbZipPair is the i-th pair of `xs.zip(ys) { }` over rbZipTake's ys (decision 389).
func rbZipPair[E, U comparable](xs []E, ys []U, i int) Tuple2[E, *U] {
	if i < len(ys) {
		return Tuple2[E, *U]{xs[i], &ys[i]}
	}
	return Tuple2[E, *U]{xs[i], nil}
}

// rbZipPairVal is rbZipPair with U = Integer or Float: the U? a copy (decision 396).
func rbZipPairVal[E, U comparable](xs []E, ys []U, i int) Tuple2[E, rbOpt[U]] {
	if i < len(ys) {
		return Tuple2[E, rbOpt[U]]{xs[i], rbOpt[U]{ys[i], true}}
	}
	return Tuple2[E, rbOpt[U]]{xs[i], rbOpt[U]{}}
}

// rbSizeOf is a fixed size, for the enumerators that know theirs up front.
func rbSizeOf(n Integer) func() *Integer {
	return func() *Integer { return &n }
}

// rbCountSize is MRI's enum_size: the receiver's own size, else nil, never a count by iterating (which may not end).
func rbCountSize(recv any) func() *Integer {
	return func() *Integer { // ponytail: a user class's own `size` is not consulted (MRI calls it); emit rbSize for classes defining size
		if e, ok := recv.(rbSized); ok {
			return e.rbSize()
		}
		return nil
	}
}

type rbSized interface{ rbSize() *Integer }

func (a *Array[E]) rbSize() *Integer { return Ref(Integer(len(a.s))) }

func (h *Hash[K, V]) rbSize() *Integer { return Ref(Integer(int(h.live))) }

func (s *Set[E]) rbSize() *Integer { return Ref(Integer(int(s.h.live))) }

// rbSize is MRI's Range#size where it is a number: nil for a non-Integer begin, and for an endless range (Infinity).
func (r *Range[E]) rbSize() *Integer {
	b, ok := any(r.b).(Integer)
	if !ok || r.beginless || r.endless {
		return nil
	}
	return Ref(rbRangeIntCount(b, any(r.e).(Integer), r.excl))
}

func (e *Enumerator[E]) rbSize() *Integer {
	if e.size == nil {
		return nil
	}
	return e.size()
}

// rbSlabRun is how many windows of n elements one slab holds: rbSlices and rbCons carve their windows from shared
// backing arrays (one allocation per run, not per window), each capped at n so an append reallocates rather than
// writing into its neighbour; a window a block keeps pins at most ~4,096 elements (decision 348).
func rbSlabRun(n int) int { return max(1, 4096/max(n, 1)) }

// rbSlabArray is a fresh Array over w, its header from hdrs (refilled when empty).
func rbSlabArray[E comparable](hdrs *[]Array[E], w []E, run int) *Array[E] {
	if len(*hdrs) == 0 {
		*hdrs = make([]Array[E], run)
	}
	a := &(*hdrs)[0]
	*hdrs = (*hdrs)[1:]
	a.s = w
	return a
}

// rbSlices is each_slice(n) as a sequence: each slice a fresh Array, the last possibly shorter.
func rbSlices[E comparable](seq func(func(E) bool), n int) func(func(*Array[E]) bool) {
	return func(yield func(*Array[E]) bool) {
		run := rbSlabRun(n)
		var slab []E
		var hdrs []Array[E]
		var cur []E
		for x := range seq {
			if cur == nil {
				switch {
				case n > 4096: // each_slice(10**9) on a short list: grow, as before, rather than allocate n up front
					cur = []E{}
				case len(slab) < n:
					slab = make([]E, n*run)
					fallthrough
				default:
					cur, slab = slab[:0:n], slab[n:]
				}
			}
			cur = append(cur, x)
			if len(cur) == n {
				if !yield(rbSlabArray(&hdrs, cur, run)) {
					return
				}
				cur = nil
			}
		}
		if len(cur) > 0 {
			yield(rbSlabArray(&hdrs, cur, run))
		}
	}
}

// rbCons is each_cons(n) as a sequence: each window a fresh Array (a copy, as MRI's), from a slab.
func rbCons[E comparable](seq func(func(E) bool), n int) func(func(*Array[E]) bool) {
	return func(yield func(*Array[E]) bool) {
		run := rbSlabRun(n)
		var slab []E
		var hdrs []Array[E]
		win := make([]E, 0, n) // slid in place: `win = win[1:]` gave up capacity and reallocated every few elements
		for x := range seq {
			if len(win) == n {
				copy(win, win[1:])
				win[n-1] = x
			} else {
				win = append(win, x)
			}
			if len(win) == n {
				if len(slab) < n {
					slab = make([]E, n*run)
				}
				w := slab[:n:n]
				slab = slab[n:]
				copy(w, win)
				if !yield(rbSlabArray(&hdrs, w, run)) {
					return
				}
			}
		}
	}
}

// rbConsView is each_cons(n) yielding one Array for every window, slid in place: the compiler picks it only for a
// block that cannot keep, mutate or hand on the window (windowSafe, decision 386), so no copy per window.
func rbConsView[E comparable](seq func(func(E) bool), n int) func(func(*Array[E]) bool) {
	return func(yield func(*Array[E]) bool) {
		win := &Array[E]{s: make([]E, 0, min(n, 4096))}
		for x := range seq {
			if len(win.s) == n {
				copy(win.s, win.s[1:])
				win.s[n-1] = x
			} else {
				win.s = append(win.s, x)
			}
			if len(win.s) == n && !yield(win) {
				return
			}
		}
	}
}

// rbSlicesView is each_slice(n) yielding one Array refilled for every slice, the last possibly shorter (decision 386).
func rbSlicesView[E comparable](seq func(func(E) bool), n int) func(func(*Array[E]) bool) {
	return func(yield func(*Array[E]) bool) {
		cur := &Array[E]{s: make([]E, 0, min(n, 4096))}
		for x := range seq {
			cur.s = append(cur.s, x)
			if len(cur.s) == n {
				if !yield(cur) {
					return
				}
				cur.s = cur.s[:0]
			}
		}
		if len(cur.s) > 0 {
			yield(cur)
		}
	}
}

type Enumerator_Any interface{ _ToAny() *Enumerator[any] }

type Enumerator_Yielder_Any interface {
	_ToAny() *Enumerator_Yielder[any]
}

// rbEnumAny views an Enumerator untyped: the same sequence, each element boxed.
func rbEnumAny[E comparable](e *Enumerator[E]) *Enumerator[any] {
	if same, ok := any(e).(*Enumerator[any]); ok {
		return same
	}
	return &Enumerator[any]{seq: rbSeqAny(e.seq), recv: e.recv, meth: e.meth, size: e.size, res: e.res}
}

func rbSeqAny[E comparable](seq func(func(E) bool)) func(func(any) bool) {
	return func(yield func(any) bool) {
		for x := range seq {
			if !yield(x) {
				return
			}
		}
	}
}

// rbPairSeq is a two-value iterator's sequence as [k, v] pairs, what a blockless Hash#each enumerates.
func rbPairSeq[K, V comparable](seq func(func(K, V) bool)) func(func(Tuple2[K, V]) bool) {
	return func(yield func(Tuple2[K, V]) bool) {
		for k, v := range seq {
			if !yield(Tuple2[K, V]{k, v}) {
				return
			}
		}
	}
}

type Enumerator_Lazy_Any interface {
	_ToAny() *Enumerator_Lazy[any]
}

// rbLazy is one step of a lazy chain; src and meth are what inspect shows.
func rbLazy[E comparable](seq func(func(E) bool), src any, meth string) *Enumerator_Lazy[E] {
	return &Enumerator_Lazy[E]{seq: seq, src: src, meth: meth}
}

// rbLazyCompactNil is Lazy#compact of a Lazy[C?] whose C holds nil itself: a leaf's *C or a non-leaf's interface (decision 378).
func rbLazyCompactNil[E comparable](l *Enumerator_Lazy[E]) *Enumerator_Lazy[E] {
	var z E
	return rbLazyFilter(l, func(x E) bool { return x != z }, "compact")
}

// rbLazyCompactVal is Lazy#compact of a Lazy[Integer?] or Lazy[Float?] (decision 396).
func rbLazyCompactVal[E comparable](l *Enumerator_Lazy[rbOpt[E]]) *Enumerator_Lazy[E] {
	return rbLazy(func(yield func(E) bool) {
		for x := range l.seq {
			if x.ok && !yield(x.v) {
				return
			}
		}
	}, any(l), "compact")
}

func rbLazyFilter[E comparable](l *Enumerator_Lazy[E], keep func(E) bool, meth string) *Enumerator_Lazy[E] {
	return rbLazy(func(yield func(E) bool) {
		for x := range l.seq {
			if keep(x) && !yield(x) {
				return
			}
		}
	}, any(l), meth)
}

type Enumerator_ArithmeticSequence_Any interface {
	_ToAny() *Enumerator_ArithmeticSequence[any]
}

// rbArithRange is Range#% / Range#step without a block: numeric ranges count by n, others take every nth element (MRI's plain Enumerator, by inspect).
func rbArithRange[E comparable](r *Range[E], n Integer, meth string) *Enumerator_ArithmeticSequence[E] {
	if n == 0 {
		panic(NewArgumentError(Ref(String("step can't be 0"))))
	}
	a := &Enumerator_ArithmeticSequence[E]{b: r.b, e: r.e, nth: n, excl: r.excl, endless: r.endless, rng: r}
	switch any(r.b).(type) {
	case Integer:
		a.by, a.numeric = any(n).(E), true
	case Float:
		a.by, a.numeric = any(Float(n)).(E), true
	}
	call := meth + "(" + strconv.FormatInt(int64(n), 10) + ")"
	if !a.numeric {
		if n < 0 {
			panic(NewArgumentError(Ref(String("step can't be negative"))))
		}
		a.insp = "#<Enumerator: " + string(rbRangeStr(r, rbInspect)) + ":" + call + ">"
		return a
	}
	a.insp = "((" + string(rbRangeStr(r, rbInspect)) + ")." + call + ")"
	return a
}

// rbArithRangeFloat is Range#step(Float) without a block: Float elements whatever E is, as MRI's.
func rbArithRangeFloat[E comparable](r *Range[E], by Float) *Enumerator_ArithmeticSequence[Float] {
	if by == 0 {
		panic(NewArgumentError(Ref(String("step can't be 0"))))
	}
	rbRangeNoBegin(r)
	a := &Enumerator_ArithmeticSequence[Float]{b: Float(rbNumFloat(any(r.b))), by: by, nth: 1, excl: r.excl, endless: r.endless, numeric: true,
		insp: "((" + string(rbRangeStr(r, rbInspect)) + ").step(" + string(rbInspect(by)) + "))"}
	if !r.endless {
		a.e = Float(rbNumFloat(any(r.e)))
	}
	return a
}

// rbArithNum is Integer#step / Float#step without a block; by shows in inspect only when given.
func rbArithNum[E comparable](b, e, by E, shown bool) *Enumerator_ArithmeticSequence[E] {
	if by == *new(E) {
		panic(NewArgumentError(Ref(String("step can't be 0"))))
	}
	args := string(rbInspect(e))
	if shown {
		args += ", " + string(rbInspect(by))
	}
	return &Enumerator_ArithmeticSequence[E]{b: b, e: e, by: by, numeric: true, insp: "(" + string(rbInspect(b)) + ".step(" + args + "))"}
}

// rbArithSeq is the sequence's elements: Integers by addition, Floats as MRI's counted ruby_float_step.
func rbArithSeq[E comparable](a *Enumerator_ArithmeticSequence[E]) func(func(E) bool) {
	return func(yield func(E) bool) {
		if !a.numeric {
			i := Integer(0)
			rbRangeEach(a.rng, func(x E) bool {
				ok := i%a.nth != 0 || yield(x)
				i++
				return ok
			})
			return
		}
		if a.rng != nil {
			rbRangeNoBegin(a.rng)
		}
		if b, ok := any(a.b).(Integer); ok && !a.endless {
			if e, ok := any(a.e).(Float); ok { // (-5..5.0), an untyped E: MRI steps in Floats
				rbFloatStepX(float64(b), float64(e), rbNumFloat(any(a.by)), a.excl, func(x Float) bool { return yield(any(x).(E)) })
				return
			}
		}
		switch b := any(a.b).(type) {
		case Integer:
			by := any(a.by).(Integer)
			e, _ := any(a.e).(Integer)
			for i := b; ; i += by {
				if !a.endless && (by > 0 && (i > e || a.excl && i == e) || by < 0 && (i < e || a.excl && i == e)) {
					return
				}
				if !yield(any(i).(E)) {
					return
				}
				if by > 0 && i > math.MaxInt-by || by < 0 && i < math.MinInt-by { // i += by would wrap: past e, or a Bignum for an endless sequence (decision 35)
					if a.endless {
						rbIntOverflow(i, "+", by)
					}
					return
				}
			}
		case Float:
			by := rbNumFloat(any(a.by))
			if a.endless {
				for i := 0.0; ; i++ {
					if !yield(any(Float(float64(b) + i*by)).(E)) {
						return
					}
				}
			}
			e := rbNumFloat(any(a.e))
			rbFloatStepX(float64(b), float64(e), by, a.excl, func(x Float) bool { return yield(any(x).(E)) })
		}
	}
}

// rbArithAll is a finite sequence's elements, for last.
func rbArithAll[E comparable](a *Enumerator_ArithmeticSequence[E]) []E {
	if a.endless {
		panic(NewRangeError(Ref(String("cannot get the last element of endless arithmetic sequence"))))
	}
	return slices.Collect(iter.Seq[E](rbArithSeq(a)))
}

func (m *Enumerator_Map[E]) rbSize() *Integer { return Ref(Integer(len(m.items.s))) }

func (s *Enumerator_Select[E]) rbSize() *Integer { return Ref(Integer(len(s.items.s))) }

// rbSize counts an Integer sequence in O(1), so `(1..10**12).step(2).size` does not walk it; nil when endless (MRI's Infinity).
func (a *Enumerator_ArithmeticSequence[E]) rbSize() *Integer {
	if a.endless {
		return nil
	}
	b, ok := any(a.b).(Integer)
	if !a.numeric || !ok {
		n := Integer(0)
		for range rbArithSeq(a) {
			n++
		}
		return &n
	}
	if a.rng != nil {
		rbRangeNoBegin(a.rng)
	}
	e, ok := any(a.e).(Integer)
	if !ok { // (1..5.0), an untyped E: Floats, counted
		n := Integer(0)
		for range rbArithSeq(a) {
			n++
		}
		return &n
	}
	by := any(a.by).(Integer)
	span, step := uint64(e)-uint64(b), uint64(by) // unsigned: e-b may pass MaxInt
	if by < 0 {
		span, step = uint64(b)-uint64(e), -step
		if e > b {
			return Ref(Integer(0))
		}
	} else if e < b {
		return Ref(Integer(0))
	}
	n := span / step
	wrapped := false
	if !a.excl || span%step != 0 {
		n++
		wrapped = n == 0 // 2**64 elements
	}
	if wrapped || n > math.MaxInt { // MRI's Bignum
		rbIntOverflow(b, "step", e)
	}
	return Ref(Integer(n))
}
