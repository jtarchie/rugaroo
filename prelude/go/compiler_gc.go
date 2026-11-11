//go:build rugaroo_prelude && !rugaroo_tinygo

// Package prelude: what differs per Go compiler, gc's side; compiler_tinygo.go is TinyGo's (decision 443).
package prelude

import (
	"hash/maphash"
	"runtime"
	"unsafe"
)

var rbHashSeed = maphash.MakeSeed()

// rbHashString is a string's hash under the process seed.
func rbHashString(s string) uint64 { return maphash.String(rbHashSeed, s) }

// rbHashComparable is k's hash under the process seed. Not generic: gc inlines this one (cost 80, its budget) and not a generic one, whose shape body costs 82.
func rbHashComparable(k any) uint64 { return maphash.Comparable(rbHashSeed, k) }

//go:linkname rbGetGoTag runtime/pprof.runtime_getProfLabel
func rbGetGoTag() unsafe.Pointer // the label slot's accessors: runtime/pprof's, which the runtime keeps for linkname users (go.dev/issue/67401)

//go:linkname rbSetGoTag runtime/pprof.runtime_setProfLabel
func rbSetGoTag(*rbGoTag) // takes the tag's own pointer type (the same word as unsafe.Pointer), so no conversion is written

// rbUntagGoroutine is a no-op here: the label slot dies with its goroutine.
func rbUntagGoroutine() {}

// rbGoIDStack parses runtime.Stack's "goroutine N [" header: Go exposes no other identity. Its cost grows with stack depth (the walk formats every frame before truncating).
func rbGoIDStack() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id int64
	for _, c := range buf[len("goroutine "):n] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}
