//go:build rugaroo_prelude && rugaroo_tinygo

// Package prelude: what differs per Go compiler, TinyGo's side; compiler_gc.go is gc's (decision 443). The linknames name TinyGo internals, so rerun the lane after a TinyGo upgrade.
package prelude

import (
	"sync"
	"unsafe"
)

// TinyGo cannot build hash/maphash (it reaches gc's map internals), so the hashes are its runtime's map hashes.
var rbHashSeed, rbHashSeed2 = uintptr(rbTinyRand()), uintptr(rbTinyRand())

//go:linkname rbTinyRand runtime.rand
func rbTinyRand() uint64

//go:linkname rbTinyHash64 runtime.hash64
func rbTinyHash64(p unsafe.Pointer, n, seed uintptr) uint64

//go:linkname rbTinyIfaceHash runtime.hashmapInterfaceHash
func rbTinyIfaceHash(x any, seed uintptr) uint32

// rbHashString is a string's hash under the process seed.
func rbHashString(s string) uint64 {
	return rbTinyHash64(unsafe.Pointer(unsafe.StringData(s)), uintptr(len(s)), rbHashSeed)
}

// rbHashComparable is k's hash under the process seed; TinyGo's interface hash is 32 bits, so two seeds make 64.
func rbHashComparable(k any) uint64 {
	return uint64(rbTinyIfaceHash(k, rbHashSeed))<<32 | uint64(rbTinyIfaceHash(k, rbHashSeed2))
}

// TinyGo has no profiler-label slot, so a goroutine's tag is keyed by its task; Task.Ptr and Data look free but channels, sync.Cond and the scheduler use them.
var rbTinyGoTags sync.Map // *task.Task → *rbGoTag

//go:linkname rbTinyTask internal/task.Current
func rbTinyTask() unsafe.Pointer

// rbGetGoTag is the running goroutine's tag; unlike gc's label slot, a goroutine does not start with its parent's.
func rbGetGoTag() unsafe.Pointer {
	if t, ok := rbTinyGoTags.Load(rbTinyTask()); ok {
		return unsafe.Pointer(t.(*rbGoTag))
	}
	return nil
}

func rbSetGoTag(t *rbGoTag) { rbTinyGoTags.Store(rbTinyTask(), t) }

// rbUntagGoroutine drops the ending goroutine's entry: the key keeps its task, and through it the goroutine's stack, alive.
func rbUntagGoroutine() { rbTinyGoTags.Delete(rbTinyTask()) }

// rbGoIDStack is 1, gc's id for main: TinyGo's runtime.Stack writes nothing, so main and an untagged goroutine share it. Not 0, which Mutex reads as no owner.
func rbGoIDStack() int64 { return 1 }
