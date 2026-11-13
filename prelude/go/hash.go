//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"math"
	"math/bits"
	"slices"
	"strings"
)

// Ruby 3.4+ prints symbol keys as labels: `a: 1`, or `"a b": 1` when the
// symbol is not an identifier (rbIsLabelSymbol, prelude/symbol.rb).

func rbInspectPair(k, v any, seen *rbSeen) string {
	if s, ok := rbUnbox(k).(Symbol); ok {
		if rbIsLabelSymbol(string(s)) {
			return string(s) + ": " + string(rbInspectIn(v, seen))
		}
		return string(rbStringInspectAs(string(s), false)) + ": " + string(rbInspectIn(v, seen))
	}
	return string(rbInspectIn(k, seen)) + " => " + string(rbInspectIn(v, seen))
}

// rbKeyIndex buckets the keys of a Hash (or uniq's seen set) that match
// by value (rbValueKey) by their hash. plain (rbPlainKey[K], computed
// once) skips it. It is not a Hash[K, ...] because Hash's methods would
// make uniq a Go instantiation cycle (decision 9).
type rbKeyIndex[K comparable] struct {
	plain  bool
	ident  bool // compare_by_identity: plain was set by it
	str    bool // a Hash's String? keys: hashed and compared by their bytes, not canonicalized through byHash (decision 448)
	byHash map[uint64][]K
}

// find is the map key Ruby's eql? gives k: the stored key eql? to k, or
// k itself. Keys that do not match by value are their own map keys.
func (idx *rbKeyIndex[K]) find(k K) (key K, h uint64, byValue bool) {
	if idx.plain || idx.str {
		return k, 0, false
	}
	if p, ok := any(&k).(**String); ok && *p != nil { // String? keys: String#hash and ==, not boxed through rbValueKey (decision 397)
		h = uint64((**p).Hash())
		for _, c := range idx.byHash[h] {
			if cp := any(&c).(**String); *cp != nil && **cp == **p {
				return c, h, true
			}
		}
		return k, h, true
	}
	if h, byValue = rbValueKey(k); !byValue {
		return k, 0, false
	}
	for _, c := range idx.byHash[h] {
		if rbKeyEql(c, k) {
			return c, h, true
		}
	}
	return k, h, true
}

// rbStrPEq: two String? keys (K is *String) are eql? when both are nil or their Strings are equal.
func rbStrPEq[K comparable](a, b K) bool {
	pa, pb := any(&a).(**String), any(&b).(**String)
	return *pa == *pb || *pa != nil && *pb != nil && **pa == **pb
}

// rbIdxHash is the bucket find files k under: a String? key's String#hash, as find's fast path, else rbValueKey's.
func rbIdxHash[K comparable](k K) (uint64, bool) {
	if p, ok := any(&k).(**String); ok && *p != nil {
		return uint64((**p).Hash()), true
	}
	return rbValueKey(k)
}

func (idx *rbKeyIndex[K]) add(k K, h uint64) {
	if idx.byHash == nil {
		idx.byHash = map[uint64][]K{}
	}
	idx.byHash[h] = append(idx.byHash[h], k)
}

func (idx *rbKeyIndex[K]) remove(k K, h uint64) {
	b := slices.DeleteFunc(idx.byHash[h], func(c K) bool { return c == k })
	if len(b) == 0 {
		delete(idx.byHash, h) // an empty bucket would keep its map entry and backing array for every distinct hash ever seen
		return
	}
	idx.byHash[h] = b
}

// rbFrom converts v, a Hash of any instantiation, into this one: a
// copy, keys and values converted (rbConv), as Array's rbFrom. The
// pairs come straight from v, not through a Hash[any, any] copy with
// its own key index (decision 329).
func (*Hash[K, V]) rbFrom(v any) (*Hash[K, V], bool) {
	h, ok := v.(interface{ rbEachAny(func(k, v any) bool) })
	if !ok {
		a, ok := v.(Hash_Any)
		if !ok {
			return nil, false
		}
		h = a._ToAny()
	}
	if out, ok := v.(*Hash[any, any]); ok {
		if same, ok := any(out).(*Hash[K, V]); ok {
			return same, true
		}
	}
	out := NewHash[K, V]()
	conv := true
	h.rbEachAny(func(k, x any) bool {
		ck, ok := rbConv[K](k)
		cv, ok2 := rbConv[V](x)
		if conv = ok && ok2; conv {
			Hash_Op_idxSet(out, ck, cv)
		}
		return conv
	})
	if d, ok := v.(interface{ _DefAny() (any, bool) }); ok && conv { // Hash.new(default)'s default comes along (decision 416)
		// ponytail: a default proc does not: its func type is the source's; convert it when a program needs one
		if dv, has := d._DefAny(); has {
			out.defVal, out.hasDef = rbConv[V](dv)
		}
	}
	return out, conv
}

// _DefAny is the Hash.new(default) value, boxed, and whether there is one.
func (self *Hash[K, V]) _DefAny() (any, bool) { return self.defVal, self.hasDef }

// rbEachAny yields each pair unboxed, as _ToAny's copy holds them.
func (self *Hash[K, V]) rbEachAny(yield func(k, v any) bool) {
	for i := int(self.head); i < len(self.ents); i++ {
		if !rbGone(self.gone, i) && !yield(rbUnbox(self.ents[i].k), rbUnbox(self.ents[i].v)) {
			return
		}
	}
}

// rbGet is the value at k. It is kept out of line so that Hash#[] fits
// Go's inlining budget: inlined, the V? it returns is not heap-allocated.
//
//go:noinline
func (self *Hash[K, V]) rbGet(k K) (V, bool) {
	if !self.idx.plain {
		if self.idx.str {
			if i := rbHashFindStr(self, k); i >= 0 {
				return self.ents[i].v, true
			}
			var zero V
			return zero, false
		}
		k, _, _ = self.idx.find(k)
	}
	if self.slots != nil {
		if i, _ := rbHashFind(self, k); i >= 0 {
			return self.ents[i].v, true
		}
	} else {
		for i := range self.ents { // the scan is repeated in []= and delete rather than shared: one less function per instantiation
			if self.ents[i].k == k && (int(self.live) == len(self.ents) || !rbGone(self.gone, i)) {
				return self.ents[i].v, true
			}
		}
	}
	var zero V
	return zero, false
}

// rbHashEnt is one entry of ents (insertion order); a deleted one stays a tombstone (gone bit) until rbHashCompact, never run while iter > 0, so an iterator's index stays valid (decision 350).
type rbHashEnt[K, V comparable] struct {
	k K
	v V
}

// rbSmallHash is the most entries a Hash holds without slots: a scan of a few keys costs less than hashing one, and most Hashes are this small.
const rbSmallHash = 8

// rbGone reports whether Hash entry i is a tombstone; not generic, so it compiles once.
func rbGone(gone []uint64, i int) bool {
	w := i >> 6
	return w < len(gone) && gone[w]&(1<<(uint(i)&63)) != 0
}

// The helpers below are free functions, not methods, so a Hash instantiation compiles only the ones it calls.

// rbHashScan finds k (canonical: idx.find) in a Hash small enough to have no slots.
func rbHashScan[K, V comparable](h *Hash[K, V], k K) (int, bool) {
	for i := range h.ents {
		if h.ents[i].k == k && (int(h.live) == len(h.ents) || !rbGone(h.gone, i)) {
			return i, true
		}
	}
	return 0, false
}

// rbHashPairs does not bump iter, so its loop body must not run Ruby code that could mutate the Hash (rbHashEachKV does).
func rbHashPairs[K, V comparable](h *Hash[K, V]) func(func(K, V) bool) {
	return func(yield func(K, V) bool) {
		for i := int(h.head); i < len(h.ents); i++ {
			if !rbGone(h.gone, i) && !yield(h.ents[i].k, h.ents[i].v) {
				return
			}
		}
	}
}

// rbHashEachKV is rbHashPairs for a body that runs Ruby code: an open iterator, as each is.
func rbHashEachKV[K, V comparable](h *Hash[K, V]) func(func(K, V) bool) {
	return func(yield func(K, V) bool) {
		h.iter++
		defer func() { h.iter-- }()
		rbHashPairs(h)(yield)
	}
}

// rbHashKeys is the live keys, in order.
func rbHashKeys[K, V comparable](h *Hash[K, V]) []K {
	out := make([]K, 0, int(h.live))
	for i := int(h.head); i < len(h.ents); i++ {
		if !rbGone(h.gone, i) {
			out = append(out, h.ents[i].k)
		}
	}
	return out
}

// rbHashSlot is k's value in place (nil when absent) for the fused `h[k] op= v`: one lookup; valid only until the next insertion.
func rbHashSlot[K, V comparable](h *Hash[K, V], k K) *V {
	if !h.idx.plain {
		if h.idx.str {
			if i := rbHashFindStr(h, k); i >= 0 {
				return &h.ents[i].v
			}
			return nil
		}
		k, _, _ = h.idx.find(k)
	}
	if h.slots != nil {
		if i, _ := rbHashFind(h, k); i >= 0 {
			return &h.ents[i].v
		}
	} else if i, ok := rbHashScan(h, k); ok {
		return &h.ents[i].v
	}
	return nil
}

// rbHashDropAt tombstones live entry i; with no iterator open an emptied Hash resets and a mostly-dead one compacts.
func rbHashDropAt[K, V comparable](h *Hash[K, V], i int) {
	k := h.ents[i].k
	if h.slots != nil {
		m := uint64(len(h.slots) - 1)
		j := rbHashOf(k) & m
		for h.slots[j] != 0 && int(h.slots[j]) != i+1 {
			j = (j + 1) & m
		}
		if h.slots[j] != 0 {
			h.slots[j] = -1
		}
	}
	if !h.idx.plain && !h.idx.str {
		if hv, byValue := rbIdxHash(k); byValue {
			h.idx.remove(k, hv)
		}
	}
	h.ents[i] = rbHashEnt[K, V]{}
	for w := i >> 6; len(h.gone) <= w; {
		h.gone = append(h.gone, 0)
	}
	h.gone[i>>6] |= 1 << (uint(i) & 63)
	h.live--
	for int(h.head) < len(h.ents) && rbGone(h.gone, int(h.head)) {
		h.head++
	}
	if h.iter == 0 && (h.live == 0 || 2*int(h.live) < len(h.ents) && len(h.ents) >= 16) {
		rbHashCompact(h)
	}
}

// rbHashDropWhere is delete_if: the block runs as under MRI's foreach, so it cannot add a key and nothing compacts under it.
func rbHashDropWhere[K, V comparable](h *Hash[K, V], drop func(K, V) bool) {
	h.iter++
	defer func() {
		h.iter--
		if h.iter == 0 && int(h.live) < len(h.ents) && (h.live == 0 || 2*int(h.live) < len(h.ents)) {
			rbHashCompact(h)
		}
	}()
	for i := int(h.head); i < len(h.ents); i++ {
		if !rbGone(h.gone, i) && drop(h.ents[i].k, h.ents[i].v) && !rbGone(h.gone, i) {
			rbHashDropAt(h, i)
		}
	}
}

// rbHashCompact drops the tombstones and rebuilds slots, keeping the storage; no iterator may be open.
func rbHashCompact[K, V comparable](h *Hash[K, V]) {
	n := 0
	for i := int(h.head); i < len(h.ents); i++ {
		if rbGone(h.gone, i) {
			continue
		}
		if n != i {
			h.ents[n] = h.ents[i]
		}
		n++
	}
	clear(h.ents[n:])
	h.ents = h.ents[:n]
	clear(h.gone)
	h.gone = h.gone[:0]
	h.head = 0
	if h.slots != nil {
		rbHashRehash(h)
	}
}

// rbHashClear tombstones every entry in place while an iterator is open, so its indices stay valid.
func rbHashClear[K, V comparable](h *Hash[K, V]) {
	clear(h.slots)
	clear(h.idx.byHash)
	if h.iter == 0 {
		clear(h.ents)
		h.ents = h.ents[:0]
		clear(h.gone)
		h.gone = h.gone[:0]
		h.live, h.head = 0, 0
		return
	}
	for i := int(h.head); i < len(h.ents); i++ {
		if !rbGone(h.gone, i) {
			h.ents[i] = rbHashEnt[K, V]{}
			for w := i >> 6; len(h.gone) <= w; {
				h.gone = append(h.gone, 0)
			}
			h.gone[i>>6] |= 1 << (uint(i) & 63)
			h.live--
		}
	}
	h.head = int32(len(h.ents)) //nolint:gosec // a Hash holds fewer than 2^31 entries
}

// rbHashFirst is the index of the first live entry, or -1.
func rbHashFirst[K, V comparable](h *Hash[K, V]) int {
	for i := int(h.head); i < len(h.ents); i++ {
		if !rbGone(h.gone, i) {
			return i
		}
	}
	return -1
}

// rbHashAt is Hash#[] for V = Integer or Float: the value as a T?, no *V to open again (decision 396).
func rbHashAt[K, V comparable](self *Hash[K, V], k K) rbOpt[V] {
	v, ok := rbHashLookup(self, k)
	return rbOpt[V]{v, ok}
}

// rbHashLookup is Hash#[]'s value: the value at k, else the default proc's
// result or the default value; false only when there is neither. Out of
// line for the same reason as rbGet: Hash#[] then inlines.
//
//go:noinline
func rbHashLookup[K, V comparable](self *Hash[K, V], k K) (V, bool) {
	if v, ok := self.rbGet(k); ok {
		return v, true
	}
	if self.defProc != nil {
		return self.defProc(self, k), true
	}
	return self.defVal, self.hasDef
}

// rbKeySet is a set keyed by eql?/hash, as Hash keys are (Array#-, #&).
type rbKeySet[K comparable] struct {
	m   map[K]bool
	idx rbKeyIndex[K]
}

func rbNewKeySet[K comparable](xs []K) *rbKeySet[K] {
	s := &rbKeySet[K]{m: map[K]bool{}, idx: rbKeyIndex[K]{plain: rbPlainKey[K]()}}
	for _, x := range xs {
		s.put(x)
	}
	return s
}

func (s *rbKeySet[K]) put(x K) {
	k, h, byValue := s.idx.find(x)
	if !s.m[k] {
		s.m[k] = true
		if byValue {
			s.idx.add(k, h)
		}
	}
}

func (s *rbKeySet[K]) has(x K) bool {
	k, _, _ := s.idx.find(x)
	return s.m[k]
}

// rbHashSplat is `{ **o }` inside a Hash literal: o's pairs in order, later keys overwriting.
func rbHashSplat[K, V comparable](h, o *Hash[K, V]) *Hash[K, V] {
	for i := int(o.head); i < len(o.ents); i++ {
		if !rbGone(o.gone, i) {
			Hash___Set(h, o.ents[i].k, o.ents[i].v)
		}
	}
	return h
}

// rbDig is Array#dig and Hash#dig: each key indexes the value before it,
// through its untyped view; nil ends the walk, as MRI's does.
func rbDig(v any, keys []any) any {
	for _, k := range keys {
		switch x := rbUnbox(v).(type) {
		case nil:
			return nil
		case interface{ _ToAny() *Array[any] }:
			i, ok := rbUnbox(k).(Integer)
			if !ok {
				panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(k) + " into Integer"))))
			}
			a := x._ToAny().s
			if i < 0 {
				i += Integer(len(a))
			}
			if i < 0 || int(i) >= len(a) {
				return nil
			}
			v = a[i]
		case interface{ _ToAny() *Hash[any, any] }:
			v, _ = x._ToAny().rbGet(rbUnbox(k))
		default:
			panic(NewTypeError(Ref(String(rbClassName(x) + " does not have #dig method"))))
		}
	}
	return rbUnbox(v)
}

// rbKwSplat is `f(**h)` into named keyword parameters: a key that names
// none of them is MRI's ArgumentError (decision 23).
func rbKwSplat[V comparable](h *Hash[Symbol, V], names ...string) *Hash[Symbol, V] {
	var bad []string
	for i := int(h.head); i < len(h.ents); i++ {
		if k := h.ents[i].k; !rbGone(h.gone, i) && !slices.Contains(names, string(k)) {
			bad = append(bad, string(rbInspect(k)))
		}
	}
	switch len(bad) {
	case 0:
		return h
	case 1:
		panic(NewArgumentError(Ref(String("unknown keyword: " + bad[0]))))
	}
	panic(NewArgumentError(Ref(String("unknown keywords: " + strings.Join(bad, ", ")))))
}

// rbKwMissing is a required keyword `**h` did not hold.
func rbKwMissing(name string) {
	panic(NewArgumentError(Ref(String("missing keyword: :" + name))))
}

// rbKwBit is a keyword's rbKw bit when `**h` gave it.
func rbKwBit(given bool, bit int) int {
	if given {
		return 1 << bit
	}
	return 0
}

// rbKwRestWithout is a `**rest` built with `**h` splats, less the keys the
// method's named keywords took by name, as MRI's.
func rbKwRestWithout[V comparable](h *Hash[Symbol, V], names ...string) *Hash[Symbol, V] {
	out := NewHash[Symbol, V]()
	for i := int(h.head); i < len(h.ents); i++ {
		if k := h.ents[i].k; !rbGone(h.gone, i) && !slices.Contains(names, string(k)) {
			Hash___Set(out, k, h.ents[i].v)
		}
	}
	return out
}

// An atomic flag: a Thread may freeze what another is reading.
func (self *Hash[K, V]) rbFrozen() bool { return self.frozen.Load() }
func (self *Hash[K, V]) rbSetFrozen()   { self.frozen.Store(true) }

// rbCheckFrozen is every mutator's FrozenError check: one field load, inlined.
func (self *Hash[K, V]) rbCheckFrozen() {
	if self.frozen.Load() {
		rbFrozenErr(self)
	}
}

// Hash's index past rbSmallHash entries (decision 388) is slots, open-addressed with linear probing: a power-of-two count, at most half used; each holds an entry index + 1, 0 is empty and -1 a deleted entry's slot (probing goes on past it).

// rbHashFind is k's entry index (canonical: idx.find), or -1, and k's hash.
func rbHashFind[K, V comparable](h *Hash[K, V], k K) (int, uint64) {
	hv := rbHashOf(k)
	m := uint64(len(h.slots) - 1)
	for i := hv & m; ; i = (i + 1) & m {
		s := h.slots[i]
		if s == 0 {
			return -1, hv
		}
		if s > 0 && h.ents[s-1].k == k {
			return int(s - 1), hv
		}
	}
}

// rbHashFindStr is k's entry index (-1 if absent) in a Hash of String? keys, compared by their bytes (decision 448).
func rbHashFindStr[K, V comparable](h *Hash[K, V], k K) int {
	if h.slots == nil {
		for i := range h.ents {
			if rbStrPEq(h.ents[i].k, k) && (int(h.live) == len(h.ents) || !rbGone(h.gone, i)) {
				return i
			}
		}
		return -1
	}
	m := uint64(len(h.slots) - 1)
	for i := rbHashOf(k) & m; ; i = (i + 1) & m {
		s := h.slots[i]
		if s == 0 {
			return -1
		}
		if s > 0 && rbStrPEq(h.ents[s-1].k, k) {
			return int(s - 1)
		}
	}
}

// rbHashIndex records the entry just appended (key hash hv) in slots, rebuilding them once past half full (or at the first entry past rbSmallHash).
func rbHashIndex[K, V comparable](h *Hash[K, V], hv uint64) {
	n := len(h.ents)
	if 2*n > len(h.slots) {
		rbHashRehash(h)
		return
	}
	m := uint64(len(h.slots) - 1)
	i := hv & m
	for h.slots[i] > 0 {
		i = (i + 1) & m
	}
	h.slots[i] = int32(n) //nolint:gosec // a Hash holds fewer than 2^31 entries
}

// rbHashRehash rebuilds slots from the live entries, at least twice their count, reusing the storage.
func rbHashRehash[K, V comparable](h *Hash[K, V]) {
	n := 16
	for n < 2*len(h.ents) {
		n <<= 1
	}
	if cap(h.slots) >= n {
		h.slots = h.slots[:n]
		clear(h.slots)
	} else {
		h.slots = make([]int32, n)
	}
	m := uint64(n - 1)
	for j := range h.ents {
		if rbGone(h.gone, j) {
			continue
		}
		i := rbHashOf(h.ents[j].k) & m
		for h.slots[i] != 0 {
			i = (i + 1) & m
		}
		h.slots[i] = int32(j + 1) //nolint:gosec // a Hash holds fewer than 2^31 entries
	}
}

// rbHashOf is k's hash for slots, agreeing with Go == (the canonical key: by-value keys go through idx.find first). An Integer folds a multiply, so every bit of k reaches the low bits a probe starts from; it inlines, the rest is rbHashOfAny.
func rbHashOf[K comparable](k K) uint64 {
	if x, ok := any(k).(Integer); ok {
		return rbMixHash(uint64(x)) //nolint:gosec // a bit pattern, not a size
	}
	return rbHashOfAny(k)
}

func rbMixHash(u uint64) uint64 {
	hi, lo := bits.Mul64(u^rbIntHashSeed, 0x9E3779B97F4A7C15)
	return hi ^ lo
}

// rbHashOfAny is rbHashOf past Integer: String and Symbol hash their bytes, Float (-0.0 as 0.0) and Boolean mix as an Integer, the rest go through rbHashComparable.
func rbHashOfAny[K comparable](k K) uint64 {
	switch x := any(k).(type) {
	case Integer:
		return rbMixHash(uint64(x)) //nolint:gosec // a bit pattern, not a size
	case String:
		return rbHashString(string(x))
	case Symbol:
		return rbHashString(string(x))
	case *String: // a String? key by its bytes, as rbStrPEq compares it (decision 448)
		if x == nil {
			return rbMixHash(0)
		}
		return rbHashString(string(*x))
	case Float:
		if x == 0 {
			x = 0 // -0.0 == 0.0
		}
		return rbMixHash(math.Float64bits(float64(x)))
	case Boolean:
		if x {
			return rbMixHash(1)
		}
		return rbMixHash(0)
	}
	return rbHashComparable(k)
}

// rbIntHashSeed varies rbHashOf's Integer hashes per process, as the hash seed does strings'.
var rbIntHashSeed = rbHashComparable(uint64(0))
