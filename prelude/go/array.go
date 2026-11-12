//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"slices"
)

// rbFrom converts v, an Array of any instantiation, into this one: a
// copy, each element converted (rbConv). The receiver only names the
// instantiation.
func (*Array[E]) rbFrom(v any) (*Array[E], bool) {
	a, ok := v.(Array_Any)
	if !ok {
		return nil, false
	}
	src := a._ToAny()
	if out, ok := any(src).(*Array[E]); ok {
		return out, true // E is untyped: _ToAny copied
	}
	out := make([]E, 0, len(src.s))
	for _, x := range src.s {
		e, ok := rbConv[E](x)
		if !ok {
			return nil, false
		}
		out = append(out, e)
	}
	return &Array[E]{s: out}, true
}

func NewArray[E comparable]() *Array[E] { return &Array[E]{} }

// rbSplatAt is element i of a yielded Array splatted across block params, or nil past its end.
func rbSplatAt[E comparable](a *Array[E], i int) *E {
	if a == nil || i >= len(a.s) {
		return nil
	}
	x := a.s[i] // a copy, not the slot (decision 397)
	return &x
}

// rbIntEqGo says Integer#== is the prelude's, Go's ==; emitProgram flips it for a program that redefines it.
const rbIntEqGo = true

// rbArrayIdxSet is Array#[]= past the in-bounds fast path: frozen, negative or growing.
func rbArrayIdxSet[E comparable](self *Array[E], i Integer, v E) E {
	self.rbCheckFrozen()
	if i < 0 {
		if -i > Integer(len(self.s)) {
			panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", i, len(self.s))))))
		}
		i += Integer(len(self.s))
	}
	if int(i) >= len(self.s) {
		rbCheckPad(int(i))
	}
	for int(i) >= len(self.s) {
		// nil when E holds it; a non-nilable E pads with its zero value (decision 7)
		var zero E
		self.s = append(self.s, zero)
	}
	self.s[i] = v
	return v
}

// rbSplatVal is rbSplatAt for a reused window (decision 386): the element copied out, so a block param a block keeps
// does not see the next window.
func rbSplatVal[E comparable](a *Array[E], i int) *E {
	if i >= len(a.s) {
		return nil
	}
	x := a.s[i]
	return &x
}

// rbCombinations is every k-combination of all, in MRI's order.
func rbCombinations[E comparable](all []E, k int) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || k > n {
		return out
	}
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		c := make([]E, k)
		for i, j := range idx {
			c[i] = all[j]
		}
		out.s = append(out.s, &Array[E]{s: c})
		i := k - 1
		for i >= 0 && idx[i] == n-k+i {
			i--
		}
		if i < 0 {
			return out
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
}

// rbPermutations is every k-permutation of all, in MRI's order.
func rbPermutations[E comparable](all []E, k int) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || k > n {
		return out
	}
	used := make([]bool, n)
	cur := make([]E, 0, k)
	var rec func()
	rec = func() {
		if len(cur) == k {
			out.s = append(out.s, &Array[E]{s: slices.Clone(cur)})
			return
		}
		for i := range n {
			if used[i] {
				continue
			}
			used[i] = true
			cur = append(cur, all[i])
			rec()
			cur = cur[:len(cur)-1]
			used[i] = false
		}
	}
	rec()
	return out
}

// rbToAry is destructuring's implicit to_ary on an untyped value: an
// Array (any instantiation, tuples too) splits, anything else is one value.
// ponytail: a user class's to_ary is not called; dispatch it through rbDyn if a program needs it.
func rbToAry(x any) *Array[any] {
	if a, ok := x.(Array_Any); ok {
		return a._ToAny()
	}
	return &Array[any]{s: []any{x}}
}

// rbTransposeAny is transpose of untyped rows (matrix's @rows, decision 414): each must be an Array, as MRI's to_ary check.
func rbTransposeAny(xs []any) *Array[*Array[any]] {
	rows := make([]*Array[any], len(xs))
	for i, x := range xs {
		a, ok := rbUnbox(x).(Array_Any)
		if !ok {
			panic(rbConvError(x, "Array"))
		}
		rows[i] = a._ToAny()
	}
	out := &Array[*Array[any]]{}
	if len(rows) == 0 {
		return out
	}
	n := len(rows[0].s)
	for _, r := range rows {
		if len(r.s) != n {
			panic(NewIndexError(Ref(String(fmt.Sprintf("element size differs (%d should be %d)", len(r.s), n)))))
		}
	}
	for j := range n {
		col := &Array[any]{s: make([]any, len(rows))}
		for i, r := range rows {
			col.s[i] = r.s[j]
		}
		out.s = append(out.s, col)
	}
	return out
}

// rbFlattenInto appends xs to out, splicing in Arrays (any instantiation,
// tuples too) down to depth levels; depth < 0 is all of them.
// An Array holding itself (through any depth) is MRI's ArgumentError, as Go's stack would otherwise overflow, which no rescue catches; a bounded depth cannot loop, so only depth < 0 tracks the Arrays being flattened.
func rbFlattenInto(out *Array[any], xs []any, depth int) {
	var path map[*Array[any]]struct{}
	if depth < 0 {
		path = map[*Array[any]]struct{}{}
	}
	rbFlattenWalk(out, xs, depth, path)
}

func rbFlattenWalk(out *Array[any], xs []any, depth int, path map[*Array[any]]struct{}) {
	for _, x := range xs {
		if a, ok := x.(Array_Any); ok && depth != 0 {
			inner := a._ToAny()
			if path != nil {
				if _, again := path[inner]; again {
					panic(NewArgumentError(Ref(String("tried to flatten recursive array"))))
				}
				path[inner] = struct{}{}
			}
			rbFlattenWalk(out, inner.s, depth-1, path)
			if path != nil {
				delete(path, inner)
			}
			continue
		}
		out.s = append(out.s, x)
	}
}

// rbFlattenRows is one level of flatten over typed rows. When U is untyped
// the rest of depth flattens dynamically; a typed U that is itself an
// Array only arises past what flatten's @self forms cover.
func rbFlattenRows[U comparable](rows []*Array[U], depth int) *Array[U] {
	out := &Array[U]{}
	for _, r := range rows {
		out.s = append(out.s, r.s...)
	}
	if depth == 1 {
		return out
	}
	if a, ok := any(out).(*Array[any]); ok {
		flat := &Array[any]{}
		rbFlattenInto(flat, a.s, depth-1)
		return any(flat).(*Array[U])
	}
	var z U
	if _, ok := any(z).(Array_Any); ok {
		panic(NewNotImplementedError(Ref(String("rugaroo: flatten of this depth over typed Arrays nested this deep"))))
	}
	return out
}

// rbRepeated is Array#repeated_combination (combo) or #repeated_permutation, eagerly in MRI's order.
func rbRepeated[E comparable](all []E, k int, combo bool) *Array[*Array[E]] {
	out := &Array[*Array[E]]{}
	n := len(all)
	if k < 0 || n == 0 && k > 0 {
		return out
	}
	if rbAllocTooBig(k, 8) { // MRI: ArgumentError from rb_ary_new's size check
		panic(NewArgumentError(Ref(String("array size too big"))))
	}
	idx := make([]int, k)
	for {
		if rbAllocTooBig(len(out.s)+1, 24+k*8) { // eager, so the combinations built so far count
			panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
		}
		c := make([]E, k)
		for i, j := range idx {
			c[i] = all[j]
		}
		out.s = append(out.s, &Array[E]{s: c})
		i := k - 1
		for i >= 0 && idx[i] == n-1 {
			i--
		}
		if i < 0 {
			return out
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			if combo {
				idx[j] = idx[i]
			} else {
				idx[j] = 0
			}
		}
	}
}

// rbMidSplat is `a, *mid, z = arr`'s mid: what the lead and trail targets leave, possibly empty.
func rbMidSplat[E comparable](a *Array[E], lead, trail int) *Array[E] {
	end := max(len(a.s)-trail, lead)
	if lead >= len(a.s) {
		return &Array[E]{}
	}
	return &Array[E]{s: append([]E{}, a.s[lead:end]...)}
}

// rbTrailIdx is trailing target j's element: counted from the end, never overlapping the leading targets.
func rbTrailIdx[E comparable](a *Array[E], lead, trail, j int) *E {
	i := max(len(a.s)-trail, lead) + j
	if i >= len(a.s) {
		return nil
	}
	x := a.s[i] // a copy, not the slot (decision 397)
	return &x
}

// rbAssoc is Array#assoc (at 0) and #rassoc (at 1): elements that are not Arrays are skipped.
func rbAssoc[E comparable](a *Array[E], key any, at int) *E {
	key = rbUnbox(key)
	for i := range a.s {
		if x, ok := any(a.s[i]).(interface{ _ToAny() *Array[any] }); ok {
			if xs := x._ToAny().s; len(xs) > at && bool(rbEq[any](xs[at], key)) {
				x := a.s[i] // a copy (decision 397)
				return &x
			}
		}
	}
	return nil
}

// An atomic flag: a Thread may freeze what another is reading.
func (self *Array[E]) rbFrozen() bool { return self.frozen.Load() }
func (self *Array[E]) rbSetFrozen()   { self.frozen.Store(true) }

// rbCheckFrozen is every mutator's FrozenError check: one field load, inlined.
func (self *Array[E]) rbCheckFrozen() {
	if self.frozen.Load() {
		rbFrozenErr(self)
	}
}

// rbAnySlice is an Array's elements as untyped values, for a splat into a `*rest` of untyped.
func rbAnySlice[E comparable](a *Array[E]) []any {
	out := make([]any, len(a.s))
	for i, x := range a.s {
		out[i] = rbUnbox(x)
	}
	return out
}

// rbSplatAny is `*x` of an untyped x into a call (decision 227): an Array's elements, nil none, a Hash its pairs, anything
// else itself, as MRI's splat (to_a) does. ponytail: a user class's to_a is not called.
func rbSplatAny(x any) []any {
	if o, ok := rbDelegateObj(x); ok { // `*lazy_languages`: a Delegator spreads its object (Homebrew's LazyObject)
		x = o
	}
	switch v := x.(type) {
	case nil:
		return nil
	case Array_Any:
		return v._ToAny().s
	case Hash_Any:
		h := v._ToAny()
		out := make([]any, 0, int(h.live))
		for k, hv := range rbHashPairs(h) {
			out = append(out, &Array[any]{s: []any{k, hv}})
		}
		return out
	case Set_Any: // `each_node(*OPERATOR_KEYWORDS)`, a frozen Set constant (decision 444)
		return Set_ToA(v._ToAny()).s
	}
	return []any{x}
}

// rbArgsCat joins a call's argument pieces, splats spread, into one slice.
func rbArgsCat(parts ...[]any) []any {
	var out []any
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// rbScanEq is Array#include? of a value no element's class holds: `elem == obj` each, as MRI's rb_equal, so an object
// whose == compares by value (Integer#== asks it back) can match (decision 324).
func rbScanEq[E comparable](a *Array[E], key any) Boolean {
	for _, e := range a.s {
		if rbEq[any](any(e), key) {
			return true
		}
	}
	return false
}

// rbArrAt is Array#[](i) for E = Integer or Float: the element copied into a T?, one bounds test (decision 396).
func rbArrAt[T comparable](a *Array[T], i Integer) rbOpt[T] {
	if i < 0 {
		i += Integer(len(a.s))
	}
	if uint64(i) >= uint64(len(a.s)) {
		return rbOpt[T]{}
	}
	return rbOpt[T]{a.s[i], true}
}

// rbZipValArray opens zip's Array[[E, U?]] with U = Integer or Float: each *U to a U? (decision 396).
func rbZipValArray[E, U comparable](a *Array[Tuple2[E, *U]]) *Array[Tuple2[E, rbOpt[U]]] {
	out := make([]Tuple2[E, rbOpt[U]], len(a.s))
	for i, t := range a.s {
		out[i] = Tuple2[E, rbOpt[U]]{t.F0, rbOptFromPtr(t.F1)}
	}
	return &Array[Tuple2[E, rbOpt[U]]]{s: out}
}

// rbZip2ValArray is rbZipValArray for zip(b, c)'s [E, U?, V?].
func rbZip2ValArray[E, U, V comparable](a *Array[Tuple3[E, *U, *V]]) *Array[Tuple3[E, rbOpt[U], rbOpt[V]]] {
	out := make([]Tuple3[E, rbOpt[U], rbOpt[V]], len(a.s))
	for i, t := range a.s {
		out[i] = Tuple3[E, rbOpt[U], rbOpt[V]]{t.F0, rbOptFromPtr(t.F1), rbOptFromPtr(t.F2)}
	}
	return &Array[Tuple3[E, rbOpt[U], rbOpt[V]]]{s: out}
}

// rbZipValSeq is rbZipValArray for zip's block forms.
func rbZipValSeq[E, U comparable](seq func(func(Tuple2[E, *U]) bool)) func(func(Tuple2[E, rbOpt[U]]) bool) {
	return func(yield func(Tuple2[E, rbOpt[U]]) bool) {
		seq(func(t Tuple2[E, *U]) bool { return yield(Tuple2[E, rbOpt[U]]{t.F0, rbOptFromPtr(t.F1)}) })
	}
}

// rbZipViewEnumVal is Array#__zip_view_enum with U = Integer or Float (decision 396).
func rbZipViewEnumVal[E, U comparable](self *Array[E], other *Array[U]) *Enumerator[Tuple2[E, rbOpt[U]]] {
	xs, ys := slices.Clone(self.s), slices.Clone(other.s)
	return rbEnumOf(func(yield func(Tuple2[E, rbOpt[U]]) bool) {
		for i, x := range xs {
			var u rbOpt[U]
			if i < len(ys) {
				u = rbOpt[U]{ys[i], true}
			}
			if !yield(Tuple2[E, rbOpt[U]]{x, u}) {
				return
			}
		}
	}, any(self), "each", nil, nil)
}

// rbZipValEnumerator is rbZipValArray for zip's lazy Enumerator form.
func rbZipValEnumerator[E, U comparable](e *Enumerator[Tuple2[E, *U]]) *Enumerator[Tuple2[E, rbOpt[U]]] {
	return rbEnumOf(func(yield func(Tuple2[E, rbOpt[U]]) bool) {
		e.seq(func(t Tuple2[E, *U]) bool { return yield(Tuple2[E, rbOpt[U]]{t.F0, rbOptFromPtr(t.F1)}) })
	}, e.recv, e.meth, e.size, nil)
}

// rbCompactVal is compact of an Array[Integer?] or Array[Float?] (decision 396).
func rbCompactVal[T comparable](a *Array[rbOpt[T]]) *Array[T] {
	out := &Array[T]{s: make([]T, 0, len(a.s))}
	for _, x := range a.s {
		if x.ok {
			out.s = append(out.s, x.v)
		}
	}
	return out
}

// rbCompactNil is compact of an Array[C?] whose C holds nil itself (a leaf's *C, another class's CI): the nils dropped,
// the elements already the C that the result's Array[C] holds (decision 378).
func rbCompactNil[T comparable](a *Array[T]) *Array[T] {
	var z T
	out := &Array[T]{s: make([]T, 0, len(a.s))}
	for _, x := range a.s {
		if x != z {
			out.s = append(out.s, x)
		}
	}
	return out
}

// rbArrayCmpTry is Array#<=> answering nil where two elements do not compare, as MRI's does (rbCmp raises there, for
// sort's sake): gem code reads the nil (rubocop's `(x <=> y || 0) <= 0` over lists of exception classes; decision 444).
func rbArrayCmpTry(a *Array[any], other any) (res rbOpt[Integer]) {
	o, ok := rbUnbox(other).(Array_Any)
	if !ok {
		return rbOpt[Integer]{}
	}
	b := o._ToAny()
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(interface{ Message() String }); ok && strings.HasPrefix(string(e.Message()), "comparison of ") {
				res = rbOpt[Integer]{}
				return
			}
			panic(r)
		}
	}()
	for i := range min(len(a.s), len(b.s)) {
		if c := rbCmp(a.s[i], b.s[i]); c != 0 {
			return rbSome(c)
		}
	}
	return rbSome(Integer(cmp.Compare(len(a.s), len(b.s))))
}

// rbArrayCmpTryAny is rbArrayCmpTry as an untyped value: nil, or the Integer.
func rbArrayCmpTryAny(a *Array[any], other any) any {
	if r := rbArrayCmpTry(a, other); r.ok {
		return r.v
	}
	return nil
}
