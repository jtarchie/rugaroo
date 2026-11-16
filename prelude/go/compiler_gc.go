//go:build rugaroo_prelude && !rugaroo_tinygo

// Package prelude: what differs per Go compiler, gc's side; compiler_tinygo.go is TinyGo's (decision 443).
package prelude

import (
	"hash/maphash"
	"os"
	"runtime"
	"runtime/debug"
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

// rbGCBallast raises the heap size at which Go collects (decision 463). Go starts collecting at 4 MB and again at
// every doubling of the live heap, so a program with a few MB of live data collects dozens of times a second; MRI's
// own heap starts larger. The ballast is never written, so the OS never backs it with memory; the collector counts
// it as live, which puts the next collection at twice its size and adds that much headroom, a constant, to a large
// heap's goal. Raising GOGC instead would scale with the heap (and with the globals Go adds to the goal).
var rbGCBallast []byte

// rbGCSentinel is an object nothing holds: its finalizer runs after the first collection.
type rbGCSentinel struct{ _ [32]byte }

// rbGCFirst is the GOGC until the first collection: Go's 4 MB minimum heap times 8, so that collection waits for
// about 32 MiB as every later one will.
const rbGCFirst = 800

// The first package variable to do any work puts the first collection off to the ballast's floor and arranges for
// the ballast itself, with GOGC back at 100, once that collection has run. It is an initializer, not a call in
// main, because Go runs the program's own package initializers (its eager Regexp literals, the tables of its
// constants) before main, and those collected at 4 MB (decision 463). A program that never allocates that much
// never collects and pays nothing: allocating 16 MiB up front costs every program 0.3M instructions, and a raised
// GOGC kept for good would scale a large heap's goal (decision 151). A GOGC or GOMEMLIMIT in the environment wins.
var _ = func() bool {
	if os.Getenv("GOGC") != "" || os.Getenv("GOMEMLIMIT") != "" {
		return false
	}
	debug.SetGCPercent(rbGCFirst)
	runtime.SetFinalizer(new(rbGCSentinel), func(*rbGCSentinel) {
		rbGCBallast = make([]byte, 16<<20)
		debug.SetGCPercent(100)
	})
	return true
}()
