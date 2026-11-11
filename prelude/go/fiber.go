//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"iter"
	"sync"
)

type rbFiberState int

const (
	rbFiberCreated rbFiberState = iota
	rbFiberResumed
	rbFiberSuspended
	rbFiberDead
)

func (s rbFiberState) String() string {
	return [...]string{"created", "resumed", "suspended", "terminated"}[s]
}

var (
	rbFiberOf    sync.Map // goroutine id → the *Fiber it runs, for catch/throw's tag list (decision 104)
	rbRootFibers sync.Map // *Thread → its root *Fiber
)

// rbCurrentFiber is the fiber this goroutine runs (its tag's, decision 403), else the running thread's root fiber.
func rbCurrentFiber() *Fiber {
	if t := (*rbGoTag)(rbGetGoTag()); t != nil && t.fiber != nil {
		return t.fiber
	}
	f, _ := rbRootFibers.LoadOrStore(rbCurrentThread(), &Fiber{state: rbFiberResumed})
	return f.(*Fiber)
}

// rbFiberPack is a value list as Ruby passes it between fibers: none is nil, one is itself, more an Array.
func rbFiberPack(vals []any) any {
	switch len(vals) {
	case 0:
		return nil
	case 1:
		return vals[0]
	}
	return &Array[any]{s: vals}
}

// resume hands control to f until it yields or ends; its exception re-raises here. The checks and messages are MRI's.
// The body runs as a coroutine (iter.Pull, decision 403): resume is one next() call, Fiber.yield one yield() call,
// so the switch is a coroutine handoff, not a goroutine wake through channels.
func (f *Fiber) resume(args []any) any {
	cur := rbCurrentFiber()
	switch {
	case f == cur:
		panic(NewFiberError(Ref(String("attempt to resume the current fiber"))))
	case f.state == rbFiberDead:
		panic(NewFiberError(Ref(String("attempt to resume a terminated fiber"))))
	case f.state == rbFiberResumed:
		panic(NewFiberError(Ref(String("attempt to resume a resuming fiber"))))
	}
	if f.state == rbFiberCreated {
		parent := rbGoID()
		rbSpawned.Store(true)
		f.next, f.stop = iter.Pull(func(yield func(any) bool) { f.run(parent, yield) })
	}
	f.state = rbFiberResumed
	f.arg = rbFiberPack(args)
	v, ok := f.next()
	if !ok {
		f.state = rbFiberDead
		f.stop()
		if err := f.err; err != nil {
			f.err = nil
			panic(err)
		}
		return f.val
	}
	f.state = rbFiberSuspended
	return v
}

// run is the fiber's body on its coroutine's goroutine, which belongs to the thread and ractor of the goroutine that
// first resumed it. The block's value or exception is kept for the resume that sees the coroutine end.
func (f *Fiber) run(parent int64, yield func(any) bool) {
	id := rbTagGoroutineFiber(f)
	defer rbUntagGoroutine()
	rbFiberOf.Store(id, f)
	defer rbFiberOf.Delete(id)
	if t, ok := rbThreadOf.Load(parent); ok {
		rbThreadOf.Store(id, t)
		defer rbThreadOf.Delete(id)
	}
	if r, ok := rbRactorOf.Load(parent); ok {
		rbRactorOf.Store(id, r)
		defer rbRactorOf.Delete(id)
	}
	defer func() {
		if r := recover(); r != nil {
			f.err = rbWrapPanic(rbCaptureBacktrace(r))
		}
	}()
	f.yield = yield
	f.val = f.blk(f.arg)
}

// rbFiberYield is Fiber.yield: hand vals to the resumer and wait for the next resume's arguments.
func rbFiberYield(vals []any) any {
	t := (*rbGoTag)(rbGetGoTag())
	if t == nil || t.fiber == nil {
		panic(NewFiberError(Ref(String("attempt to yield on a not resumed fiber"))))
	}
	f := t.fiber
	f.yield(rbFiberPack(vals))
	return f.arg
}
