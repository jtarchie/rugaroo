//go:build rugaroo_prelude

package prelude

import (
	"fmt"
	"slices"
	"strings"
)

// rbView: six accessors per Array instantiation let rbViewCall's one body answer whole-array dynamic calls (decision 236); _At is the raw E, rbUnbox gives the Ruby value.
type rbView interface {
	_Len() Integer
	_At(i Integer) any
	_Set(i Integer, v any)
	_Append(v any)
	_Resize(n Integer)
	_Drop(n Integer)
	_Make() any
	rbCheckFrozen()
	rbFrozen() bool
}

// rbHashView is the same for a Hash: keys in insertion order, values by key.
type rbHashView interface {
	_Len() Integer
	_KeysAny() any
	_PairsAny() any
	_Has(k any) Boolean
	_Get(k any) any
	_Store(k, v any)
	_Delete(k any)
	_Make() any
	_Dup() any
	rbCheckFrozen()
}

func rbViewElems(s rbView) []any {
	n := int(s._Len())
	out := make([]any, n)
	for i := range n {
		out[i] = s._At(Integer(i))
	}
	return out
}

func rbViewSorted(s rbView) []any {
	xs := rbViewElems(s)
	rbSortStable(xs, func(a, b any) int { return int(rbCmp(rbUnbox(a), rbUnbox(b))) })
	return xs
}

func rbViewFrom(s rbView, xs []any) any {
	out := s._Make()
	for _, x := range xs {
		out.(rbView)._Append(x)
	}
	return out
}

// rbViewFromWide is rbViewFrom for a new Array that may mix classes (`host.split(".") + port.divmod(256)`, net-ftp's
// sendport): an Array[untyped] when an element is not s's, as MRI's holds any (decision 416). No per-instantiation
// accessor answers "is it an E", so the typed build's TypeError says it.
func rbViewFromWide(s rbView, xs []any) (out any) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(TypeErrorI); !ok {
				panic(r)
			}
			out = &Array[any]{s: xs}
		}
	}()
	return rbViewFrom(s, xs)
}

// rbViewReplace makes s hold xs (raw elements), in place.
func rbViewReplace(s rbView, xs []any) {
	s.rbCheckFrozen()
	s._Resize(0)
	for _, x := range xs {
		s._Append(x)
	}
}

// rbViewArg is an Array argument's raw elements (a view's keep their E, so a T? value round-trips), else as untyped code sees them.
func rbViewArg(v any, what string) []any {
	switch a := rbUnbox(v).(type) {
	case rbView:
		return rbViewElems(a)
	case Array_Any:
		return a._ToAny().s
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into " + what))))
}

// rbViewCmp compares raw elements x and y: by the block's -1/0/1 when there is one (sort { |a, b| ... }), else <=>.
func rbViewCmp(b *rbBlockArg, x, y any) Integer {
	if b == nil {
		return rbCmp(rbUnbox(x), rbUnbox(y))
	}
	r := rbUnbox(b.fn(x, y))
	if r == nil {
		panic(rbCmpErr(rbUnbox(x), rbUnbox(y)))
	}
	return rbCmp[any](r, Integer(0))
}

func rbViewSortedBy(s rbView, b *rbBlockArg) []any {
	if b == nil {
		return rbViewSorted(s)
	}
	xs := rbViewElems(s)
	rbSortStable(xs, func(x, y any) int { return int(rbViewCmp(b, x, y)) })
	return xs
}

// rbViewUniqBy is rbViewUniq keyed by the block's result when there is one.
func rbViewUniqBy(xs []any, b *rbBlockArg) []any {
	if b == nil {
		return rbViewUniq(xs)
	}
	seen := rbNewKeySet[any](nil)
	var out []any
	for _, x := range xs {
		k := rbUnbox(b.fn(x))
		if !seen.has(k) {
			seen.put(k)
			out = append(out, x)
		}
	}
	return out
}

// rbViewUniq keeps the first of each element by eql?/hash, as Array#uniq.
func rbViewUniq(xs []any) []any {
	seen := rbNewKeySet[any](nil)
	var out []any
	for _, x := range xs {
		u := rbUnbox(x)
		if !seen.has(u) {
			seen.put(u)
			out = append(out, x)
		}
	}
	return out
}

func rbViewIndex(s rbView, v any, fromEnd bool) any {
	xs := rbViewElems(s)
	for n := range xs {
		i := n
		if fromEnd {
			i = len(xs) - 1 - n
		}
		if rbEq[any](rbUnbox(xs[i]), v) {
			return Integer(i)
		}
	}
	return nil
}

func rbViewAssoc(s rbView, key any, at int) any {
	for _, x := range rbViewElems(s) {
		if a, ok := rbUnbox(x).(Array_Any); ok {
			if inner := a._ToAny().s; len(inner) > at && rbEq[any](inner[at], key) {
				return rbUnbox(x)
			}
		}
	}
	return nil
}

func rbViewTakeDrop(s rbView, args []any, take bool) any {
	rbArity(len(args), 1, 1)
	n := int(rbAs[Integer](args[0], "Integer"))
	what := "drop"
	if take {
		what = "take"
	}
	if n < 0 {
		panic(NewArgumentError(Ref(String("attempt to " + what + " negative size"))))
	}
	xs := rbViewElems(s)
	n = min(n, len(xs))
	if take {
		return rbViewFrom(s, xs[:n])
	}
	return rbViewFrom(s, xs[n:])
}

// rbViewCall answers a dynamic call of name on the view, or false when name is not one of its names.
func rbViewCall(name string, s rbView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo,maintidx // one switch per name, as the dispatchers are
	b, args := rbPopBlock(args) // the block forms (decision 302): sort/max/min compare with it, count/index test with it, uniq keys by it
	switch name {
	case "sort":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewSortedBy(s, b)), true
	case "sort!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		for i, x := range rbViewSortedBy(s, b) {
			s._Set(Integer(i), x)
		}
		return s, true
	case "reverse", "reverse!":
		rbArity(len(args), 0, 0)
		xs := rbViewElems(s)
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		if name == "reverse" {
			return rbViewFrom(s, xs), true
		}
		s.rbCheckFrozen()
		for i, x := range xs {
			s._Set(Integer(i), x)
		}
		return s, true
	case "compact":
		rbArity(len(args), 0, 0)
		var xs []any
		for _, x := range rbViewElems(s) {
			if rbUnbox(x) != nil {
				xs = append(xs, x)
			}
		}
		return rbViewFrom(s, xs), true
	case "include?":
		rbArity(len(args), 1, 1)
		return Boolean(rbViewIndex(s, args[0], false) != nil), true
	case "index", "find_index", "rindex":
		if b != nil && len(args) == 0 {
			return rbViewOrNil(rbViewIndexWhere(s, b, name == "rindex")), true
		}
		rbArity(len(args), 1, 1)
		return rbViewIndex(s, args[0], name == "rindex"), true
	case "count":
		rbArity(len(args), 0, 1)
		if b != nil && len(args) == 0 {
			return Integer(len(rbViewKeep(s, b, true))), true
		}
		if len(args) == 0 {
			return s._Len(), true
		}
		n := 0
		for _, x := range rbViewElems(s) {
			if rbEq[any](rbUnbox(x), args[0]) {
				n++
			}
		}
		return Integer(n), true
	case "max", "min":
		rbArity(len(args), 0, 0)
		var best, raw any
		for i, x := range rbViewElems(s) {
			if i == 0 {
				best, raw = rbUnbox(x), x
				continue
			}
			c := rbViewCmp(b, x, raw)
			if (name == "max") == (c > 0) && c != 0 {
				best, raw = rbUnbox(x), x
			}
		}
		return best, true
	case "join":
		rbArity(len(args), 0, 1)
		sep := String("")
		if len(args) == 1 && args[0] != nil {
			sep = rbAs[String](args[0], "String")
		}
		parts := make([]string, int(s._Len()))
		for i := range parts {
			x := rbUnbox(s._At(Integer(i)))
			if a, ok := x.(Array_Any); ok {
				parts[i] = string(Array_Join(a._ToAny(), sep))
				continue
			}
			parts[i] = string(rbToS(x))
		}
		return rbStrClone(String(strings.Join(parts, string(sep)))), true
	case "uniq":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewUniqBy(rbViewElems(s), b)), true
	case "uniq!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		u := rbViewUniqBy(xs, b)
		if len(u) == len(xs) {
			return nil, true
		}
		rbViewReplace(s, u)
		return s, true
	case "+":
		rbArity(len(args), 1, 1)
		return rbViewFromWide(s, append(rbViewElems(s), rbViewArg(args[0], "Array")...)), true
	case "-", "difference":
		rbArity(len(args), 1, 1)
		drop := rbNewKeySet(rbViewArg(args[0], "Array"))
		var xs []any
		for _, x := range rbViewElems(s) {
			if !drop.has(rbUnbox(x)) {
				xs = append(xs, x)
			}
		}
		return rbViewFrom(s, xs), true
	case "&", "intersection", "intersect?":
		rbArity(len(args), 1, 1)
		keep := rbNewKeySet(rbViewArg(args[0], "Array"))
		var xs []any
		for _, x := range rbViewElems(s) {
			if keep.has(rbUnbox(x)) {
				xs = append(xs, x)
			}
		}
		xs = rbViewUniq(xs)
		if name == "intersect?" {
			return Boolean(len(xs) > 0), true
		}
		return rbViewFrom(s, xs), true
	case "|", "union":
		rbArity(len(args), 1, 1)
		return rbViewFrom(s, rbViewUniq(append(rbViewElems(s), rbViewArg(args[0], "Array")...))), true
	case "*":
		rbArity(len(args), 1, 1)
		if sep, ok := rbUnbox(args[0]).(String); ok {
			return rbViewCall("join", s, []any{sep})
		}
		n := int(rbAs[Integer](args[0], "Integer"))
		if n < 0 {
			panic(NewArgumentError(Ref(String("negative argument"))))
		}
		xs := rbViewElems(s)
		var out []any
		if len(xs) > 0 {
			if rbAllocTooBig(n, len(xs)*16) {
				panic(NewArgumentError(Ref(String("argument too big"))))
			}
			out = make([]any, 0, len(xs)*n)
			for range n {
				out = append(out, xs...)
			}
		}
		return rbViewFrom(s, out), true
	case "delete":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		var found any
		var xs []any
		hit := false
		for _, x := range rbViewElems(s) {
			if rbEq[any](rbUnbox(x), args[0]) {
				found, hit = rbUnbox(x), true
				continue
			}
			xs = append(xs, x)
		}
		if hit {
			rbViewReplace(s, xs)
		}
		return found, true
	case "delete_at":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		i := int(rbAs[Integer](args[0], "Integer"))
		if i < 0 {
			i += len(xs)
		}
		if i < 0 || i >= len(xs) {
			return nil, true
		}
		x := xs[i]
		rbViewReplace(s, append(xs[:i:i], xs[i+1:]...))
		return rbUnbox(x), true
	case "concat":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		for _, x := range rbViewArg(args[0], "Array") {
			s._Append(x)
		}
		return s, true
	case "replace":
		rbArity(len(args), 1, 1)
		rbViewReplace(s, rbViewArg(args[0], "Array"))
		return s, true
	case "fill":
		rbArity(len(args), 1, 3)
		s.rbCheckFrozen()
		n := int(s._Len())
		from, to := 0, n
		if len(args) > 1 { // fill(v, start, len = to the end): past the end the Array grows, nil between
			if _, isRange := rbUnbox(args[1]).(Integer); !isRange && rbUnbox(args[1]) != nil {
				return nil, false // a Range: the typed twin's
			}
			if st, ok := rbUnbox(args[1]).(Integer); ok {
				if from = int(st); from < 0 {
					from = max(from+n, 0)
				}
			}
			to = max(n, from)
			if len(args) > 2 && rbUnbox(args[2]) != nil {
				to = from + max(int(rbAs[Integer](rbUnbox(args[2]), "Integer")), 0)
			}
		}
		for i := n; i < to; i++ {
			s._Append(nil)
		}
		for i := from; i < to; i++ {
			s._Set(Integer(i), args[0])
		}
		return s, true
	case "insert":
		rbArity(len(args), 1, -1)
		s.rbCheckFrozen()
		xs := rbViewElems(s)
		i := int(rbAs[Integer](args[0], "Integer"))
		if i < 0 {
			i += len(xs) + 1
		}
		if i < 0 {
			panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", int(rbAs[Integer](args[0], "Integer")), len(xs)+1)))))
		}
		if i > len(xs) {
			rbCheckPad(i)
		}
		for len(xs) < i {
			xs = append(xs, nil)
		}
		out := append(append(append([]any{}, xs[:i]...), args[1:]...), xs[i:]...)
		rbViewReplace(s, out)
		return s, true
	case "rotate":
		rbArity(len(args), 0, 1)
		xs := rbViewElems(s)
		if len(xs) == 0 {
			return rbViewFrom(s, nil), true
		}
		n := 1
		if len(args) == 1 {
			n = int(rbAs[Integer](args[0], "Integer"))
		}
		k := ((n % len(xs)) + len(xs)) % len(xs)
		return rbViewFrom(s, append(append([]any{}, xs[k:]...), xs[:k]...)), true
	case "take":
		return rbViewTakeDrop(s, args, true), true
	case "drop":
		return rbViewTakeDrop(s, args, false), true
	case "assoc":
		rbArity(len(args), 1, 1)
		return rbViewAssoc(s, args[0], 0), true
	case "rassoc":
		rbArity(len(args), 1, 1)
		return rbViewAssoc(s, args[0], 1), true
	case "transpose":
		rbArity(len(args), 0, 0)
		return rbTransposeAny(rbViewElems(s)), true
	case "flatten", "flatten!":
		rbArity(len(args), 0, 1)
		depth := -1
		if len(args) == 1 {
			depth = int(rbAs[Integer](args[0], "Integer"))
		}
		xs := rbViewElems(s)
		nested := false
		for i, x := range xs {
			xs[i] = rbUnbox(x)
			if _, isArr := xs[i].(Array_Any); isArr {
				nested = true
			}
		}
		if name == "flatten!" {
			s.rbCheckFrozen()
			a, untyped := s.(*Array[any]) // a typed E cannot hold what its nested Arrays held (decision 92): nil, as the typed path
			if !nested || !untyped {
				return nil, true
			}
			out := &Array[any]{}
			rbFlattenInto(out, a.s, depth)
			a.s = out.s
			return s, true
		}
		if !nested {
			return rbViewFrom(s, rbViewElems(s)), true
		}
		out := &Array[any]{}
		rbFlattenInto(out, xs, depth)
		return out, true
	default:
	}
	return nil, false
}

// rbHashViewArg is a Hash argument as a view (raw values round-trip), else its untyped copy, which is one too.
func rbHashViewArg(v any) rbHashView {
	switch h := rbUnbox(v).(type) {
	case rbHashView:
		return h
	case Hash_Any:
		return h._ToAny()
	}
	panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(v) + " into Hash"))))
}

func rbHashViewKeys(s rbHashView) []any {
	return s._KeysAny().([]any)
}

func rbHashViewCopy(s rbHashView) rbHashView {
	return s._Dup().(rbHashView)
}

// rbHashViewCall answers a dynamic call of name on a Hash's view, or false when name is not one of its names.
func rbHashViewCall(name string, s rbHashView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo // one switch per name
	switch name {
	case "merge":
		rbArity(len(args), 1, 1)
		out := rbHashViewCopy(s)
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(other) {
			out._Store(k, other._Get(k))
		}
		return out, true
	case "merge!", "update":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(other) {
			s._Store(k, other._Get(k))
		}
		return s, true
	case "except", "slice":
		ks := rbNewKeySet(args)
		out := s._Make().(rbHashView)
		if name == "slice" {
			for _, k := range args {
				if s._Has(k) {
					out._Store(k, s._Get(k))
				}
			}
			return out, true
		}
		for _, k := range rbHashViewKeys(s) {
			if !ks.has(rbUnbox(k)) {
				out._Store(k, s._Get(k))
			}
		}
		return out, true
	case "to_h":
		rbArity(len(args), 0, 0)
		return s, true // MRI's Hash#to_h without a block is self
	case "compact":
		rbArity(len(args), 0, 0)
		out := s._Make().(rbHashView)
		for _, k := range rbHashViewKeys(s) {
			if v := s._Get(k); rbUnbox(v) != nil {
				out._Store(k, v)
			}
		}
		return out, true
	case "compact!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		changed := false
		for _, k := range rbHashViewKeys(s) {
			if rbUnbox(s._Get(k)) == nil {
				s._Delete(k)
				changed = true
			}
		}
		if !changed {
			return nil, true
		}
		return s, true
	case "flatten":
		rbArity(len(args), 0, 1)
		out := &Array[any]{}
		for _, k := range rbHashViewKeys(s) {
			out.s = append(out.s, rbUnbox(k), rbUnbox(s._Get(k)))
		}
		if len(args) == 1 {
			if depth := rbAs[Integer](args[0], "Integer"); depth > 1 {
				return Array_Flatten(out, depth-1), true
			}
		}
		return out, true
	case "value?", "has_value?":
		rbArity(len(args), 1, 1)
		for _, k := range rbHashViewKeys(s) {
			if rbEq[any](rbUnbox(s._Get(k)), args[0]) {
				return Boolean(true), true
			}
		}
		return Boolean(false), true
	case "key":
		rbArity(len(args), 1, 1)
		for _, k := range rbHashViewKeys(s) {
			if rbEq[any](rbUnbox(s._Get(k)), args[0]) {
				return rbUnbox(k), true
			}
		}
		return nil, true
	case "replace":
		rbArity(len(args), 1, 1)
		s.rbCheckFrozen()
		other := rbHashViewArg(args[0])
		for _, k := range rbHashViewKeys(s) {
			s._Delete(k)
		}
		for _, k := range rbHashViewKeys(other) {
			s._Store(k, other._Get(k))
		}
		return s, true
	default:
	}
	return nil, false
}

// rbViewYield calls a dynamic call's block with raw elements, as the per-shape wrappers did, or raises as a missing yield does.
func rbViewYield(b *rbBlockArg, xs ...any) any {
	if b == nil {
		panic(NewLocalJumpError(Ref(String("no block given (yield)"))))
	}
	return b.fn(xs...)
}

// rbViewTest is the block's result for x as a condition: a raw E result (an Integer? element's nil pointer) unboxed first.
func rbViewTest(b *rbBlockArg, xs ...any) bool {
	return rbTruthy(rbUnbox(rbViewYield(b, xs...)))
}

// rbViewLive is a live loop's bound: i is still inside s, read again each pass, as MRI's each sees what the block appends.
func rbViewLive(s rbView, i int) bool { return i < int(s._Len()) }

// rbViewKeep is the raw elements whose block result is truthy (or falsy when keep is false).
func rbViewKeep(s rbView, b *rbBlockArg, keep bool) []any {
	var out []any
	for _, x := range rbViewElems(s) {
		if rbViewTest(b, x) == keep {
			out = append(out, x)
		}
	}
	return out
}

// rbViewKeepBang is select!/reject!/keep_if/delete_if: in place, nil when nothing changed unless always.
func rbViewKeepBang(s rbView, b *rbBlockArg, keep, always bool) any {
	s.rbCheckFrozen()
	n := int(s._Len())
	xs := rbViewKeep(s, b, keep)
	if len(xs) == n && !always {
		return nil
	}
	rbViewReplace(s, xs)
	return s
}

// rbViewBy is the element whose block key is greatest (max_by) or least, as its Ruby value.
func rbViewBy(s rbView, b *rbBlockArg, most bool) any {
	var best, key any
	for i, x := range rbViewElems(s) {
		k := rbUnbox(rbViewYield(b, x))
		if i == 0 {
			best, key = x, k
			continue
		}
		if c := rbCmp(k, key); most && c > 0 || !most && c < 0 {
			best, key = x, k
		}
	}
	return rbUnbox(best)
}

// rbViewSortBy is the raw elements ordered by their block keys, stable as the typed path.
func rbViewSortBy(s rbView, b *rbBlockArg) []any {
	xs := rbViewElems(s)
	keys := make([]any, len(xs))
	for i, x := range xs {
		keys[i] = rbUnbox(rbViewYield(b, x))
	}
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	rbSortStable(idx, func(i, j int) int { return int(rbCmp(keys[i], keys[j])) })
	out := make([]any, len(xs))
	for i, j := range idx {
		out[i] = xs[j]
	}
	return out
}

// rbViewBsearch is the index bsearch finds: find-minimum for a true/false/nil block, find-any for a numeric one; -1 when none.
func rbViewBsearch(s rbView, b *rbBlockArg) int {
	lo, hi, found := 0, int(s._Len()), -1
	for lo < hi {
		mid := lo + (hi-lo)/2
		switch r := rbUnbox(rbViewYield(b, s._At(Integer(mid)))).(type) {
		case Integer, Float:
			c := rbCmp[any](r, Integer(0))
			if c == 0 {
				return mid
			}
			if c < 0 {
				hi = mid
			} else {
				lo = mid + 1
			}
		default:
			if rbTruthy(r) {
				found, hi = mid, mid
			} else {
				lo = mid + 1
			}
		}
	}
	return found
}

// rbViewIndexWhere is the first (or, fromEnd, last) index whose block result is truthy, or -1.
func rbViewIndexWhere(s rbView, b *rbBlockArg, fromEnd bool) int {
	n := int(s._Len())
	for k := range n {
		i := k
		if fromEnd {
			i = n - 1 - k
		}
		if rbViewTest(b, s._At(Integer(i))) {
			return i
		}
	}
	return -1
}

func rbViewOrNil(i int) any {
	if i < 0 {
		return nil
	}
	return Integer(i)
}

// rbViewSizeArg is each_slice/each_cons's size: positive, else ArgumentError with MRI's message.
func rbViewSizeArg(args []any, msg string) int {
	rbArity(len(args), 1, 1)
	n := int(rbAs[Integer](args[0], "Integer"))
	if n <= 0 {
		panic(NewArgumentError(Ref(String(msg))))
	}
	return n
}

// rbViewBlockCall answers a block-taking whole-array name on the view (decision 302): one body for every element type,
// where each Array[E] instantiation compiled a wrapper and the Enumerable chain it names. Blocks get raw elements.
func rbViewBlockCall(name string, s rbView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo,maintidx // one switch per name, as the dispatchers are
	b, args := rbPopBlock(args)
	if b == nil { // without a block: an Array standing in for the Enumerator, as the prelude's __each_enum twins (regexp_parser's `expressions.reverse_each.find { }`)
		switch name {
		case "each", "each_entry":
			rbArity(len(args), 0, 0)
			return &Array[any]{s: slices.Clone(rbViewElems(s))}, true
		case "reverse_each":
			rbArity(len(args), 0, 0)
			xs := slices.Clone(rbViewElems(s))
			slices.Reverse(xs)
			return &Array[any]{s: xs}, true
		case "each_with_index":
			rbArity(len(args), 0, 0)
			out := &Array[any]{}
			for i, x := range rbViewElems(s) {
				out.s = append(out.s, &Array[any]{s: []any{x, Integer(i)}})
			}
			return out, true
		case "each_index":
			rbArity(len(args), 0, 0)
			out := &Array[any]{}
			for i := range rbViewElems(s) {
				out.s = append(out.s, Integer(i))
			}
			return out, true
		}
	}
	switch name {
	case "each", "each_entry":
		rbArity(len(args), 0, 0)
		for i := 0; rbViewLive(s, i); i++ { // MRI's each sees elements the block appends
			rbViewYield(b, s._At(Integer(i)))
		}
		return s, true
	case "each_with_index":
		rbArity(len(args), 0, 0)
		for i := 0; rbViewLive(s, i); i++ {
			rbViewYield(b, s._At(Integer(i)), Integer(i))
		}
		return s, true
	case "each_index":
		rbArity(len(args), 0, 0)
		for i := 0; rbViewLive(s, i); i++ {
			rbViewYield(b, Integer(i))
		}
		return s, true
	case "reverse_each":
		rbArity(len(args), 0, 0)
		for _, x := range slices.Backward(rbViewElems(s)) {
			rbViewYield(b, x)
		}
		return s, true
	case "each_slice", "each_cons":
		msg := "invalid slice size"
		if name == "each_cons" {
			msg = "invalid size"
		}
		n := rbViewSizeArg(args, msg)
		xs := rbViewElems(s)
		if name == "each_slice" {
			for i := 0; i < len(xs); i += n {
				rbViewYield(b, rbViewFrom(s, xs[i:min(i+n, len(xs))]))
			}
		} else {
			for i := 0; i+n <= len(xs); i++ {
				rbViewYield(b, rbViewFrom(s, xs[i:i+n]))
			}
		}
		return s, true
	case "cycle":
		rbArity(len(args), 0, 1)
		times := -1
		if len(args) == 1 && args[0] != nil {
			times = max(int(rbAs[Integer](args[0], "Integer")), 0)
		}
		for k := 0; times < 0 || k < times; k++ {
			if s._Len() == 0 {
				break
			}
			for i := 0; rbViewLive(s, i); i++ {
				rbViewYield(b, s._At(Integer(i)))
			}
		}
		return nil, true
	case "first":
		rbArity(len(args), 0, 1)
		if len(args) == 0 {
			if s._Len() == 0 {
				return nil, true
			}
			return rbUnbox(s._At(0)), true
		}
		n := int(rbAs[Integer](args[0], "Integer"))
		if n < 0 {
			panic(NewArgumentError(Ref(String("negative array size"))))
		}
		xs := rbViewElems(s)
		return rbViewFrom(s, xs[:min(n, len(xs))]), true
	case "select", "filter", "find_all":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewKeep(s, b, true)), true
	case "reject":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewKeep(s, b, false)), true
	case "select!", "filter!", "keep_if":
		rbArity(len(args), 0, 0)
		return rbViewKeepBang(s, b, true, name == "keep_if"), true
	case "reject!", "delete_if":
		rbArity(len(args), 0, 0)
		return rbViewKeepBang(s, b, false, name == "delete_if"), true
	case "map!", "collect!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		for i := 0; rbViewLive(s, i); i++ {
			s._Set(Integer(i), rbViewYield(b, s._At(Integer(i))))
		}
		return s, true
	case "find", "detect":
		rbArity(len(args), 0, 0)
		if i := rbViewIndexWhere(s, b, false); i >= 0 {
			return rbUnbox(s._At(Integer(i))), true
		}
		return nil, true
	case "rfind":
		rbArity(len(args), 0, 0)
		if i := rbViewIndexWhere(s, b, true); i >= 0 {
			return rbUnbox(s._At(Integer(i))), true
		}
		return nil, true
	case "inject", "reduce":
		rbArity(len(args), 0, 1)
		xs := rbViewElems(s)
		var memo any
		if len(args) == 1 {
			memo = args[0]
		} else if len(xs) > 0 {
			memo, xs = xs[0], xs[1:]
		}
		for _, x := range xs {
			memo = rbViewYield(b, memo, x)
		}
		return rbUnbox(memo), true
	case "each_with_object":
		rbArity(len(args), 1, 1)
		for _, x := range rbViewElems(s) {
			rbViewYield(b, x, args[0])
		}
		return args[0], true
	case "sort_by":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewSortBy(s, b)), true
	case "sort_by!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		rbViewReplace(s, rbViewSortBy(s, b))
		return s, true
	case "max_by", "min_by":
		rbArity(len(args), 0, 0)
		return rbViewBy(s, b, name == "max_by"), true
	case "take_while", "drop_while":
		rbArity(len(args), 0, 0)
		xs := rbViewElems(s)
		n := 0
		for n < len(xs) && rbViewTest(b, xs[n]) {
			n++
		}
		if name == "take_while" {
			return rbViewFrom(s, xs[:n]), true
		}
		return rbViewFrom(s, xs[n:]), true
	case "bsearch":
		rbArity(len(args), 0, 0)
		if i := rbViewBsearch(s, b); i >= 0 {
			return rbUnbox(s._At(Integer(i))), true
		}
		return nil, true
	case "bsearch_index":
		rbArity(len(args), 0, 0)
		return rbViewOrNil(rbViewBsearch(s, b)), true
	default:
	}
	return nil, false
}

// rbViewPredicate answers all?/any?/none?/one? with a block, a pattern (===) or neither (truthiness); its own function
// so only programs that call one of them dynamically keep the === dispatcher.
func rbViewPredicate(name string, s rbView, args []any) (any, bool) {
	b, args := rbPopBlock(args)
	rbArity(len(args), 0, 1)
	test := func(x any) bool {
		switch {
		case len(args) == 1:
			return rbTruthy(rbDynOp_eqq(rbCall, args[0], x))
		case b != nil:
			return rbTruthy(rbUnbox(b.fn(x)))
		}
		return rbTruthy(rbUnbox(x))
	}
	hits := 0
	for _, x := range rbViewElems(s) {
		if !test(x) {
			if name == "all?" {
				return Boolean(false), true
			}
			continue
		}
		hits++
		if name == "any?" || name == "none?" || name == "one?" && hits > 1 {
			return Boolean(name == "any?"), true
		}
	}
	return Boolean(name != "any?" && (name != "one?" || hits == 1)), true
}

// rbViewSum is sum(init = 0) with an optional block: Integer and Float add natively, anything else through +.
func rbViewSum(_ string, s rbView, args []any) (any, bool) {
	b, args := rbPopBlock(args)
	rbArity(len(args), 0, 1)
	var acc any = Integer(0)
	if len(args) == 1 {
		acc = args[0]
	}
	for _, x := range rbViewElems(s) {
		v := rbUnbox(x)
		if b != nil {
			v = rbUnbox(b.fn(x))
		}
		switch a := acc.(type) {
		case Integer:
			if n, ok := v.(Integer); ok {
				acc = a.Op_plus(n)
				continue
			}
			if f, ok := v.(Float); ok {
				acc = Float(a) + f
				continue
			}
		case Float:
			switch n := v.(type) {
			case Float:
				acc = a + n
				continue
			case Integer:
				acc = a + Float(n)
				continue
			}
		}
		acc = rbDynOp_plus(rbCall, acc, v)
	}
	return acc, true
}

// rbHashViewKeep is a copy of s with the pairs whose block result (two arguments, key and value, as Hash#select yields)
// is truthy, or falsy when keep is false.
func rbHashViewKeep(s rbHashView, b *rbBlockArg, keep bool) rbHashView {
	out := s._Make().(rbHashView)
	for _, k := range rbHashViewKeys(s) {
		v := s._Get(k)
		if rbViewTest(b, k, v) == keep {
			out._Store(k, v)
		}
	}
	return out
}

// rbHashViewKeepBang is select!/reject!/keep_if/delete_if: in place, nil when nothing changed unless always.
func rbHashViewKeepBang(s rbHashView, b *rbBlockArg, keep, always bool) any {
	s.rbCheckFrozen()
	changed := false
	for _, k := range rbHashViewKeys(s) {
		if rbViewTest(b, k, s._Get(k)) != keep {
			s._Delete(k)
			changed = true
		}
	}
	if !changed && !always {
		return nil
	}
	return s
}

func rbHashViewPairs(s rbHashView) []any {
	return s._PairsAny().([]any)
}

// rbHashViewBlockCall answers a block-taking Hash name on the view (decision 302), yielding what each per-shape
// wrapper yielded: the pair (a [K, V] tuple) for iteration and Enumerable names, key and value for the select family.
func rbHashViewBlockCall(name string, s rbHashView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo,maintidx // one switch per name
	b, args := rbPopBlock(args)
	switch name {
	case "each", "each_pair", "each_entry":
		rbArity(len(args), 0, 0)
		for _, p := range rbHashViewPairs(s) {
			rbViewYield(b, p)
		}
		return s, true
	case "each_key":
		rbArity(len(args), 0, 0)
		for _, k := range rbHashViewKeys(s) {
			rbViewYield(b, k)
		}
		return s, true
	case "each_value":
		rbArity(len(args), 0, 0)
		for _, k := range rbHashViewKeys(s) {
			rbViewYield(b, s._Get(k))
		}
		return s, true
	case "each_with_index":
		rbArity(len(args), 0, 0)
		for i, p := range rbHashViewPairs(s) {
			rbViewYield(b, p, Integer(i))
		}
		return s, true
	case "each_slice", "each_cons":
		msg := "invalid slice size"
		if name == "each_cons" {
			msg = "invalid size"
		}
		n := rbViewSizeArg(args, msg)
		ps := rbHashViewPairs(s)
		if name == "each_slice" {
			for i := 0; i < len(ps); i += n {
				rbViewYield(b, &Array[any]{s: slices.Clone(ps[i:min(i+n, len(ps))])})
			}
		} else {
			for i := 0; i+n <= len(ps); i++ {
				rbViewYield(b, &Array[any]{s: slices.Clone(ps[i : i+n])})
			}
		}
		return s, true
	case "cycle":
		rbArity(len(args), 0, 1)
		times := -1
		if len(args) == 1 && args[0] != nil {
			times = max(int(rbAs[Integer](args[0], "Integer")), 0)
		}
		for k := 0; times < 0 || k < times; k++ {
			ps := rbHashViewPairs(s)
			if len(ps) == 0 {
				break
			}
			for _, p := range ps {
				rbViewYield(b, p)
			}
		}
		return nil, true
	case "select", "filter":
		rbArity(len(args), 0, 0)
		return rbHashViewKeep(s, b, true), true
	case "reject":
		rbArity(len(args), 0, 0)
		return rbHashViewKeep(s, b, false), true
	case "select!", "filter!", "keep_if":
		rbArity(len(args), 0, 0)
		return rbHashViewKeepBang(s, b, true, name == "keep_if"), true
	case "reject!", "delete_if":
		rbArity(len(args), 0, 0)
		return rbHashViewKeepBang(s, b, false, name == "delete_if"), true
	case "count":
		rbArity(len(args), 0, 1)
		if len(args) == 1 {
			n := 0
			for _, p := range rbHashViewPairs(s) {
				if rbEq[any](rbUnbox(p), args[0]) {
					n++
				}
			}
			return Integer(n), true
		}
		if b == nil {
			return s._Len(), true
		}
		n := 0
		for _, p := range rbHashViewPairs(s) {
			if rbViewTest(b, p) {
				n++
			}
		}
		return Integer(n), true
	case "find", "detect":
		rbArity(len(args), 0, 0)
		for _, p := range rbHashViewPairs(s) {
			if rbViewTest(b, p) {
				return p, true
			}
		}
		return nil, true
	case "inject", "reduce":
		rbArity(len(args), 0, 1)
		ps := rbHashViewPairs(s)
		var memo any
		if len(args) == 1 {
			memo = args[0]
		} else if len(ps) > 0 {
			memo, ps = ps[0], ps[1:]
		}
		for _, p := range ps {
			memo = rbViewYield(b, memo, p)
		}
		return rbUnbox(memo), true
	case "each_with_object":
		rbArity(len(args), 1, 1)
		for _, p := range rbHashViewPairs(s) {
			rbViewYield(b, p, args[0])
		}
		return args[0], true
	case "transform_values!":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		for _, k := range rbHashViewKeys(s) {
			s._Store(k, rbViewYield(b, s._Get(k)))
		}
		return s, true
	default:
	}
	return nil, false
}

// rbHashViewPredicate is all?/any?/none?/one? on a Hash's pairs: pattern (===), block or truthiness, as rbViewPredicate.
func rbHashViewPredicate(name string, s rbHashView, args []any) (any, bool) {
	b, args := rbPopBlock(args)
	rbArity(len(args), 0, 1)
	hits := 0
	for _, p := range rbHashViewPairs(s) {
		var ok bool
		switch {
		case len(args) == 1:
			ok = rbTruthy(rbDynOp_eqq(rbCall, args[0], p))
		case b != nil:
			ok = rbTruthy(rbUnbox(b.fn(p)))
		default:
			ok = true // a pair is never nil or false
		}
		if !ok {
			if name == "all?" {
				return Boolean(false), true
			}
			continue
		}
		hits++
		if name == "any?" || name == "none?" || name == "one?" && hits > 1 {
			return Boolean(name == "any?"), true
		}
	}
	return Boolean(name != "any?" && (name != "one?" || hits == 1)), true
}

// rbViewCore answers the element-level Array methods (fetch, [], []=, push, pop, shift, ...) on the view, as the
// prelude's bodies do: one body for every element type instead of a _DynGen case and its callees per type (decision 304).
func rbViewCore(name string, s rbView, args []any) (any, bool) { //nolint:cyclop,gocognit,gocyclo,maintidx // one switch per name, as the dispatchers are
	b, args := rbPopBlock(args)
	n := s._Len()
	switch name {
	case "length", "size":
		rbArity(len(args), 0, 0)
		return n, true
	case "empty?":
		rbArity(len(args), 0, 0)
		return Boolean(n == 0), true
	case "frozen?":
		rbArity(len(args), 0, 0)
		return Boolean(s.rbFrozen()), true
	case "to_a", "entries", "deconstruct", "itself":
		rbArity(len(args), 0, 0)
		return s, true
	case "dup":
		rbArity(len(args), 0, 0)
		return rbViewFrom(s, rbViewElems(s)), true
	case "clear":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		s._Resize(0)
		return s, true
	case "[]", "slice":
		if len(args) == 2 {
			return rbViewIdx2(s, rbAs[Integer](args[0], "Integer"), rbAs[Integer](args[1], "Integer")), true
		}
		rbArity(len(args), 1, 1)
		if sl, ok := rbRangeSlice(args[0], n); ok {
			if sl == nil {
				return nil, true
			}
			return rbViewIdx2(s, sl.F0, sl.F1), true
		}
		return rbViewAt(s, rbAs[Integer](args[0], "Integer")), true
	case "at":
		rbArity(len(args), 1, 1)
		return rbViewAt(s, rbAs[Integer](args[0], "Integer")), true
	case "fetch":
		rbArity(len(args), 1, 2)
		i := rbAs[Integer](args[0], "Integer")
		if i >= -n && i < n {
			return rbViewAt(s, i), true
		}
		switch {
		case b != nil:
			return b.fn(i), true
		case len(args) == 2:
			return args[1], true
		}
		panic(NewIndexError(Ref(String(fmt.Sprintf("index %d outside of array bounds: %d...%d", i, -n, n)))))
	case "fetch_values":
		var out []any
		for _, a := range args {
			i := rbAs[Integer](a, "Integer")
			if i < -n || i >= n {
				panic(NewIndexError(Ref(String(fmt.Sprintf("index %d outside of array bounds: %d...%d", i, -n, n)))))
			}
			out = append(out, s._At(rbViewNorm(i, n)))
		}
		return rbViewFrom(s, out), true
	case "dig":
		rbArity(len(args), 1, -1)
		rbAs[Integer](args[0], "Integer")
		return rbDig(s, args), true
	case "last":
		if len(args) == 1 {
			k := rbAs[Integer](args[0], "Integer")
			if k < 0 {
				panic(NewArgumentError(Ref(String("negative array size"))))
			}
			return rbViewFrom(s, rbViewElems(s)[max(n-k, 0):]), true
		}
		rbArity(len(args), 0, 0)
		return rbViewAt(s, -1), true
	case "[]=":
		return rbViewIdxSet(s, args), true
	case "<<", "push", "append":
		if name == "<<" {
			rbArity(len(args), 1, 1)
		}
		xs := rbViewChecked(s, args)
		s.rbCheckFrozen()
		for _, x := range xs {
			s._Append(x)
		}
		return s, true
	case "unshift", "prepend":
		xs := rbViewChecked(s, args)
		s.rbCheckFrozen()
		rbViewReplace(s, append(xs, rbViewElems(s)...))
		return s, true
	case "pop", "shift":
		rbArity(len(args), 0, 0)
		s.rbCheckFrozen()
		if n == 0 {
			return nil, true
		}
		if name == "pop" {
			x := s._At(n - 1)
			s._Resize(n - 1)
			return rbUnbox(x), true
		}
		x := s._At(0)
		s._Drop(1)
		return rbUnbox(x), true
	case "shuffle", "shuffle!":
		rbArity(len(args), 0, 1)
		xs := rbViewElems(s)
		rbShuffle(xs, &rbViewRandom(args, 0).mt)
		if name == "shuffle" {
			return rbViewFrom(s, xs), true
		}
		rbViewReplace(s, xs)
		return s, true
	case "sample":
		rbArity(len(args), 0, 2)
		count := -1 // no count: one element or nil
		opts := 0
		if len(args) > 0 {
			if _, ok := args[0].(*Hash[Symbol, *Random]); !ok {
				count, opts = int(rbAs[Integer](args[0], "Integer")), 1
				if count < 0 {
					panic(NewArgumentError(Ref(String("negative sample number"))))
				}
			}
		}
		rng := rbViewRandom(args, opts)
		if count < 0 {
			if idx := rbSampleIdx(1, int(n), &rng.mt); len(idx) > 0 {
				return rbUnbox(s._At(Integer(idx[0]))), true
			}
			return nil, true
		}
		var out []any
		for _, i := range rbSampleIdx(count, int(n), &rng.mt) {
			out = append(out, s._At(Integer(i)))
		}
		return rbViewFrom(s, out), true
	}
	return nil, false
}

// rbViewNorm is index i counted from the end when negative.
func rbViewNorm(i, n Integer) Integer {
	if i < 0 {
		return i + n
	}
	return i
}

// rbViewAt is the element at i (from the end when negative) as Ruby sees it, nil outside the array.
func rbViewAt(s rbView, i Integer) any {
	n := s._Len()
	if i = rbViewNorm(i, n); i < 0 || i >= n {
		return nil
	}
	return rbUnbox(s._At(i))
}

// rbViewIdx2 is self[start, count]: a new array of the same element type, nil when start is out of bounds.
func rbViewIdx2(s rbView, start, count Integer) any {
	n := s._Len()
	start = rbViewNorm(start, n)
	if start < 0 || start > n || count < 0 {
		return nil
	}
	return rbViewFrom(s, rbViewElems(s)[start:min(start+count, n)])
}

// rbViewChecked converts xs to the array's element type before anything is written, as a typed call converts its
// arguments: a TypeError leaves the array as it was.
func rbViewChecked(s rbView, xs []any) []any {
	tmp := s._Make().(rbView)
	for _, x := range xs {
		tmp._Append(x)
	}
	return rbViewElems(tmp)
}

// rbViewIdxSet is []= with an index, a start and length, or a range, as Array#[]= and __idxset_3.
func rbViewIdxSet(s rbView, args []any) any {
	rbArity(len(args), 2, 3)
	n := s._Len()
	var start, count Integer
	v := args[len(args)-1]
	switch sp, ok := rbRangeSpan(args[0], n); {
	case len(args) == 3:
		start, count = rbAs[Integer](args[0], "Integer"), rbAs[Integer](args[1], "Integer")
	case ok:
		if sp == nil {
			panic(NewRangeError(Ref(rbInspect(args[0]) + " out of range")))
		}
		start, count = sp.F0, sp.F1
	default: // one index: the element itself, grown with E's zero value up to it
		i := rbAs[Integer](args[0], "Integer")
		s.rbCheckFrozen()
		if i < 0 && -i > n {
			panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", i, n)))))
		}
		i = rbViewNorm(i, n)
		if i >= n {
			s._Resize(i + 1)
		}
		s._Set(i, v)
		return v
	}
	s.rbCheckFrozen()
	if start < 0 {
		if -start > n {
			panic(NewIndexError(Ref(String(fmt.Sprintf("index %d too small for array; minimum: -%d", start, n)))))
		}
		start += n
	}
	if count < 0 {
		panic(NewIndexError(Ref(String(fmt.Sprintf("negative length (%d)", count)))))
	}
	items := []any{v}
	if a, ok := v.(Array_Any); ok {
		items = a._ToAny().s
	}
	items = rbViewChecked(s, items)
	if n < start {
		s._Resize(start)
	}
	xs := rbViewElems(s)
	end := min(start+count, Integer(len(xs)))
	rbViewReplace(s, slices.Concat(xs[:start:start], items, xs[end:]))
	return v
}

// rbViewRandom is the random: option at args[i], else the default generator.
func rbViewRandom(args []any, i int) *Random {
	if i < len(args) {
		if r := Hash_Op_idx(rbAs[*Hash[Symbol, *Random]](args[i], "Hash[Symbol, Random]"), Symbol("random")); r != nil {
			return *r
		}
	}
	return rbDefaultRandom
}

// rbViewGrep is grep/grep_v (pattern ===), mapping each match through the block when there is one; its own function so
// only programs that call it dynamically keep the === dispatcher.
func rbViewGrep(name string, s rbView, args []any) (any, bool) {
	b, args := rbPopBlock(args)
	rbArity(len(args), 1, 1)
	var out []any
	for _, x := range rbViewElems(s) {
		if rbTruthy(rbDynOp_eqq(rbCall, args[0], x)) == (name == "grep") {
			if b != nil {
				x = b.fn(x)
			}
			out = append(out, x)
		}
	}
	if b != nil {
		return &Array[any]{s: out}, true
	}
	return rbViewFrom(s, out), true
}

// rbRangeSlice is __slice on an Integer Range held untyped, of either instantiation (an end may be nil: Range[Integer?]).
func rbRangeSlice(a any, n Integer) (*Tuple2[Integer, Integer], bool) {
	switch r := a.(type) {
	case *Range[Integer]:
		return Range___Slice[Integer](r, n), true
	case *Range[rbOpt[Integer]]:
		return Range___Slice[rbOpt[Integer]](r, n), true
	case *Range[any]: // a gem's `lines[0...(tok.line - 1)]` with an untyped bound (rubocop's FrozenStringLiteral; decision 444)
		if x, ok := (*Range[Integer])(nil).rbFrom(r); ok {
			return Range___Slice[Integer](x, n), true
		}
	}
	return nil, false
}

// rbRangeSpan is __span, as rbRangeSlice.
func rbRangeSpan(a any, n Integer) (*Tuple2[Integer, Integer], bool) {
	switch r := a.(type) {
	case *Range[Integer]:
		return Range___Span[Integer](r, n), true
	case *Range[rbOpt[Integer]]:
		return Range___Span[rbOpt[Integer]](r, n), true
	case *Range[any]: // a gem's `lines[0...(tok.line - 1)]` with an untyped bound (rubocop's FrozenStringLiteral; decision 444)
		if x, ok := (*Range[Integer])(nil).rbFrom(r); ok {
			return Range___Span[Integer](x, n), true
		}
	}
	return nil, false
}
