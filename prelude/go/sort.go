//go:build rugaroo_prelude

package prelude

import (
	"iter"
	"slices"
)

// rbSortStable is slices.SortStableFunc at O(n log n): a run past one insertion-sort block is pdqsorted with ties
// broken by position, which is the same order. Up to 20 elements it is SortStableFunc itself, so a small sort compares
// the same pairs in the same order (a failure names the pair it does; decision 332).
func rbSortStable[E any](s []E, cmp func(a, b E) int) {
	if len(s) <= 20 {
		slices.SortStableFunc(s, cmp)
		return
	}
	type at struct {
		i int
		e E
	}
	t := make([]at, len(s))
	for i, e := range s {
		t[i] = at{i, e}
	}
	slices.SortFunc(t, func(a, b at) int {
		if c := cmp(a.e, b.e); c != 0 {
			return c
		}
		return a.i - b.i
	})
	for i := range t {
		s[i] = t[i].e
	}
}

// rbMinmax is Enumerable#minmax with MRI's pairing (enum.c minmax_i, minmax_ii): a pair is ordered first, so ties keep
// the earlier element and the block of a side-effecting each runs once; cmp is <=> or minmax's block.
func rbMinmax[E any](seq iter.Seq[E], cmp func(a, b E) int) (*E, *E) {
	var lo, hi, last *E
	update := func(i, j E) {
		if lo == nil {
			lo, hi = &i, &j
			return
		}
		if cmp(i, *lo) < 0 {
			lo = &i
		}
		if cmp(j, *hi) > 0 {
			hi = &j
		}
	}
	for x := range seq {
		if last == nil {
			last = &x
			continue
		}
		j, i := *last, x
		last = nil
		if n := cmp(j, i); n == 0 {
			i = j
		} else if n < 0 {
			i, j = j, i
		}
		update(i, j)
	}
	if last != nil {
		update(*last, *last)
	}
	return lo, hi
}
