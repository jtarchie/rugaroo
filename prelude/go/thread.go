//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
)

var (
	rbThreadOf   sync.Map // goroutine id → *Thread, for Thread.current (decision 104)
	rbMainThread = &Thread{done: make(chan struct{})}
)

// rbSpawned is set before the first goroutine that can run Ruby code starts (Thread, Fiber, Ractor, Timeout, a WEBrick handler, a trap hook). Until then every caller is main, so rbGoID skips the stack walk: catch/throw and Monitor call it per use (decision 332).
var rbSpawned atomic.Bool

// rbGoID is the running goroutine's id (decision 104): main's until another goroutine may run Ruby, then its tag
// (rbTagGoroutine), else the stack's.
func rbGoID() int64 {
	if !rbSpawned.Load() {
		return rbMainGoID
	}
	if p := rbGetGoTag(); p != nil {
		return (*rbGoTag)(p).id
	}
	return rbGoIDStack()
}

// rbGoTag is a goroutine's id kept in its profiler-label slot, which the runtime gives every goroutine and copies
// into the goroutines it starts. It begins as runtime/pprof's label map does (one slice of pairs, here empty), so a
// CPU profile reads a tagged goroutine as unlabelled (decision 332).
type rbGoTag struct {
	_     []struct{ key, value string }
	id    int64
	fiber *Fiber // the Fiber this goroutine runs, for Fiber.current and Fiber.yield (decision 403)

	recDepth, recHashDepth int // Struct comparisons and hashes in progress (decision 407)
}

// rbTagGoroutine gives the running goroutine its own tag and returns its id: the next counter value, offset past any
// id runtime.Stack reports (main's, decision 104), so no stack walk (decision 403). A goroutine that runs Ruby code
// calls it first: until then it carries the tag of the goroutine that started it.
func rbTagGoroutine() int64 {
	id := rbGoCounter.Add(1) + rbGoIDBase
	rbSetGoTag(&rbGoTag{id: id})
	return id
}

// rbGoCounter numbers the goroutines rbTagGoroutine tags; rbGoIDBase keeps them clear of the goroutine numbers
// rbGoIDStack parses (main's id, and an untagged goroutine's fallback).
var rbGoCounter atomic.Int64

// rbTagGoroutineFiber is rbTagGoroutine for a fiber's coroutine, which carries its Fiber.
func rbTagGoroutineFiber(f *Fiber) int64 {
	id := rbGoCounter.Add(1) + rbGoIDBase
	rbSetGoTag(&rbGoTag{id: id, fiber: f})
	return id
}

const rbGoIDBase = 1 << 40

// rbCurrentThread is Thread.current: the Thread this goroutine runs, else main (a ractor's own goroutine is its main thread, as in MRI).
func rbCurrentThread() *Thread {
	if rbThreadSeq.Load() == 0 {
		return rbMainThread // no Thread started yet, so every goroutine is main's: skips rbGoID, which Fiddle's last_error pays per C call
	}
	if t, ok := rbThreadOf.Load(rbGoID()); ok {
		return t.(*Thread)
	}
	return rbMainThread
}

// notSelf is join/value's guard: MRI raises rather than deadlock when a thread joins itself.
func (t *Thread) notSelf() {
	if t == rbCurrentThread() {
		panic(NewThreadError(Ref(String("Target thread must not be current thread"))))
	}
}

// rbThreadRun starts run on a goroutine and returns its Thread handle: a panic re-raises on join (or, for SystemExit, exits like MRI's main thread would). The thread belongs to the ractor that started it, so Ractor.receive inside it reads that ractor's port.
func rbThreadRun(run func() any) *Thread {
	return rbThreadStart(&Thread{}, run)
}

// rbThreadStart starts run on t in place: a Thread subclass's embedded Thread (decision 202), whose goroutine must
// write the val and err that join and value read on the same struct, not a copy.
func rbThreadStart(t *Thread, run func() any) *Thread {
	t.done, t.loc = make(chan struct{}), rbCallerPCs() // resolved by inspect, not per start (decision 403)
	parent := rbGoID()
	creator := rbMainThread // the main goroutine has no rbThreadOf entry
	if p, ok := rbThreadOf.Load(parent); ok {
		creator = p.(*Thread)
	}
	t.group.Store(creator.group.Load()) // a new thread joins its creator's ThreadGroup, as in MRI (decision 132)
	ractor, inRactor := rbRactorOf.Load(parent)
	rbLiveThreads.Store(t, rbThreadSeq.Add(1))
	rbSpawned.Store(true)
	go func() {
		id := rbTagGoroutine()
		defer rbUntagGoroutine()
		rbThreadOf.Store(id, t)
		defer rbThreadOf.Delete(id)
		defer rbRootFibers.Delete(t) // Fiber.current's root fiber for this thread
		if inRactor {
			rbRactorOf.Store(id, ractor)
			defer rbRactorOf.Delete(id)
		}
		defer close(t.done)
		defer rbLiveThreads.Delete(t) // before done closes: a joined thread is off the list
		defer func() {
			if r := recover(); r != nil {
				t.aborting.Store(true)
				t.err = rbThreadAbort(r)
			}
		}()
		t.val = run()
	}()
	return t
}

// rbThreadAbort ends a thread's or ractor's goroutine on a panic: SystemExit exits the program, anything else is reported on stderr as MRI does and returned wrapped, for join to re-raise.
func rbThreadAbort(r any) any {
	if e, ok := r.(SystemExitI); ok {
		// ponytail: MRI re-raises exit in the main thread, running its ensures; this exits here.
		rbFinish(int(e.Status()), nil)
		os.Exit(0)
	}
	err := rbWrapPanic(rbCaptureBacktrace(r))                                 // join/value re-raise it on another goroutine: these are its frames
	if t, ok := rbThreadOf.Load(rbGoID()); !ok || !t.(*Thread).quiet.Load() { // report_on_exception = false keeps it quiet
		fmt.Fprintln(os.Stderr, "#<Thread> terminated with exception (report_on_exception is true):", rbToS(err), "("+rbClassName(err)+")")
	}
	return err
}

// rbThreadKey is a thread-local's key: a Symbol or String, as MRI takes either (decision 108).
func rbThreadKey(key any) string {
	switch k := rbUnbox(key).(type) {
	case Symbol:
		return string(k)
	case String:
		return string(k)
	}
	panic(NewTypeError(Ref(String(string(rbInspect(key)) + " is not a symbol nor a string"))))
}

// rbThreadKeys lists a local map's keys as Symbols, sorted (sync.Map has no order).
func rbThreadKeys(m *sync.Map) *Array[Symbol] {
	out := &Array[Symbol]{}
	m.Range(func(k, _ any) bool {
		out.s = append(out.s, Symbol(k.(string)))
		return true
	})
	slices.Sort(out.s)
	return out
}

// rbLiveThreads are the threads not yet finished, each with its creation number, for Thread.list.
var (
	rbLiveThreads sync.Map
	rbThreadSeq   atomic.Int64
)

// rbThreadList is Thread.list: the main thread, then the live threads in creation order.
func rbThreadList() *Array[*Thread] {
	type entry struct {
		t   *Thread
		seq int64
	}
	var live []entry
	rbLiveThreads.Range(func(k, v any) bool {
		live = append(live, entry{k.(*Thread), v.(int64)})
		return true
	})
	slices.SortFunc(live, func(a, b entry) int { return cmp.Compare(a.seq, b.seq) })
	out := &Array[*Thread]{s: []*Thread{rbMainThread}}
	for _, e := range live {
		out.s = append(out.s, e.t)
	}
	return out
}

// rbThreadGroupDefault is ThreadGroup::Default; a Thread whose group is nil is in it.
var rbThreadGroupDefault = &ThreadGroup{}

// threadGroup is Thread#group: a thread keeps its group after it ends, as in MRI 4.0.
func (t *Thread) threadGroup() *ThreadGroup {
	if g := t.group.Load(); g != nil {
		return g
	}
	return rbThreadGroupDefault
}

// rbThreadGroupAdd is ThreadGroup#add with MRI's ThreadError messages; freezing a group is not modeled.
func rbThreadGroupAdd(g *ThreadGroup, t *Thread) {
	if g.enclosed.Load() {
		panic(NewThreadError(Ref(String("can't move to the enclosed thread group"))))
	}
	if t.threadGroup().enclosed.Load() {
		panic(NewThreadError(Ref(String("can't move from the enclosed thread group"))))
	}
	if g == rbThreadGroupDefault {
		g = nil
	}
	t.group.Store(g)
}

// rbThreadGroupList is ThreadGroup#list: Thread.list's live threads that are in g.
func rbThreadGroupList(g *ThreadGroup) *Array[*Thread] {
	out := &Array[*Thread]{}
	for _, t := range rbThreadList().s {
		if t.threadGroup() == g {
			out.s = append(out.s, t)
		}
	}
	return out
}

// rbErrinfoSet makes e the thread's $! while a gem's rescue clause runs, returning what restores the one before
// (decision 331): a bare `raise` in a method that clause calls (liquid's `->(e) { raise }` renderer) re-raises it.
func rbErrinfoSet(e any) func() {
	t := rbCurrentThread()
	prev := t.errinfo.Swap(&e)
	return func() { t.errinfo.Store(prev) }
}

// rbErrinfo is the thread's $!, nil outside every rescue clause.
func rbErrinfo() any {
	if p := rbCurrentThread().errinfo.Load(); p != nil {
		return *p
	}
	return nil
}
