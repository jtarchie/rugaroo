//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"slices"
	"sync"
	"sync/atomic"
)

// rbPort is Ractor::Port's queue, a lock per port; a blocked receive or select parks on its own one-slot channel (rbWaiter), so a send wakes one receiver, as a channel would, while the queue stays unbounded and `closed?` stays a flag (decision 103).
type rbPort struct {
	mu      sync.Mutex
	items   []any
	closed  bool
	waiters []*rbWaiter
}

// rbWaiter is one parked receive or select; done marks a select that finished through another port, so a wake spent on it is passed on (leave).
type rbWaiter struct {
	ch   chan struct{}
	done atomic.Bool
}

var (
	rbRactorIDs  atomic.Int64 // main is #1
	rbPortIDs    atomic.Int64 // for Port#inspect
	rbRactorLive atomic.Int64 // started and not yet finished, for Ractor.count
	rbRactorOf   sync.Map     // goroutine id → *Ractor, for a ractor's goroutine and the threads it starts (rbThreadRun)
	rbMainRactor = rbNewMainRactor()
)

func rbNewMainRactor() *Ractor {
	r := &Ractor{done: make(chan struct{}), id: 1}
	r.port = &Ractor_Port{owner: r}
	return r
}

// rbCurrentRactor is the ractor whose goroutine (or thread) this is, main when none (decision 104).
func rbCurrentRactor() *Ractor {
	if r, ok := rbRactorOf.Load(rbGoID()); ok {
		return r.(*Ractor)
	}
	return rbMainRactor
}

func rbPortClosed() any {
	return NewRactor_ClosedError(Ref(String("The port was already closed")))
}

func (p *rbPort) send(x any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		panic(rbPortClosed())
	}
	p.items = append(p.items, x)
	p.wake(false)
}

// wake signals one parked waiter (all for close), skipping selects already done elsewhere; the one-slot send never blocks.
func (p *rbPort) wake(all bool) {
	for len(p.waiters) > 0 {
		w := p.waiters[0]
		p.waiters = p.waiters[1:]
		if w.done.Load() {
			continue
		}
		select {
		case w.ch <- struct{}{}:
		default:
		}
		if !all {
			return
		}
	}
}

// tryTake pops a message (ok) or reports the port closed and drained.
func (p *rbPort) tryTake() (x any, ok, closed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) == 0 {
		return nil, false, p.closed
	}
	x = p.items[0]
	p.items[0] = nil
	p.items = p.items[1:]
	return x, true, false
}

// park registers w under the lock unless a message or close arrived since tryTake (then the caller polls again): checking and parking in one step is what rules out a lost wake-up.
func (p *rbPort) park(w *rbWaiter) (again bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) > 0 || p.closed {
		return true
	}
	if !slices.Contains(p.waiters, w) {
		p.waiters = append(p.waiters, w)
	}
	return false
}

// leave forgets a finished waiter; a message still queued wakes the next one, since the wake for it may have gone to w.
func (p *rbPort) leave(w *rbWaiter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waiters = slices.DeleteFunc(p.waiters, func(x *rbWaiter) bool { return x == w })
	if len(p.items) > 0 {
		p.wake(false)
	}
}

func (p *rbPort) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.wake(true)
}

func (p *rbPort) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// rbPortReceive blocks for the first message on any of ports, answering its index; every port closed and drained raises ClosedError.
func rbPortReceive(ports []*rbPort) (int, any) {
	var w *rbWaiter // allocated on the first miss: a message already queued never parks
	defer func() {
		if w == nil {
			return
		}
		w.done.Store(true)
		for _, p := range ports {
			p.leave(w)
		}
	}()
	for {
		allClosed := true
		for i, p := range ports {
			x, ok, closed := p.tryTake()
			if ok {
				return i, x
			}
			allClosed = allClosed && closed
		}
		if allClosed {
			panic(rbPortClosed())
		}
		if w == nil {
			w = &rbWaiter{ch: make(chan struct{}, 1)}
		}
		again := false
		for _, p := range ports {
			again = p.park(w) || again
		}
		if again {
			continue
		}
		<-w.ch
	}
}

func (p *rbPort) receive() any {
	_, x := rbPortReceive([]*rbPort{p})
	return x
}

// rbPortSelect is Ractor.select: the first port holding a message, with it. A port another ractor created is ClosedError, as MRI's select treats it.
func rbPortSelect(ports []*Ractor_Port) (*Ractor_Port, any) {
	if len(ports) == 0 {
		panic(NewArgumentError(Ref(String("specify at least one port"))))
	}
	cur := rbCurrentRactor()
	qs := make([]*rbPort, len(ports))
	for i, p := range ports {
		if p.owner != cur {
			panic(rbPortClosed())
		}
		qs[i] = &p.q
	}
	i, x := rbPortReceive(qs)
	return ports[i], x
}

// receiveOwned is Port#receive: only the creating ractor may receive, as MRI's.
func (p *Ractor_Port) receiveOwned() any {
	if p.owner != rbCurrentRactor() {
		panic(NewRactor_Error(Ref(String("only allowed from the creator Ractor of this port"))))
	}
	return p.q.receive()
}

func rbNewRactor() *Ractor {
	r := &Ractor{done: make(chan struct{}), id: int(rbRactorIDs.Add(1)) + 1, loc: rbCallerPCs()} // resolved by inspect (decision 403)
	r.port = &Ractor_Port{owner: r, id: int(rbPortIDs.Add(1))}
	return r
}

// start runs the block on a goroutine registered as r's. Its port closes when it ends, so later sends raise ClosedError; an exception is kept for value/join and reported on stderr like a thread's.
func (r *Ractor) start(run func() any) *Ractor {
	rbRactorLive.Add(1)
	rbSpawned.Store(true)
	go func() {
		id := rbTagGoroutine()
		defer rbUntagGoroutine()
		rbRactorOf.Store(id, r)
		defer rbRactorOf.Delete(id)
		defer close(r.done)
		defer r.port.q.close()
		defer rbRactorLive.Add(-1)
		defer func() {
			if p := recover(); p != nil {
				r.err = rbThreadAbort(p)
			}
		}()
		r.val = run()
	}()
	return r
}

// reraise, after done, raises RemoteError around the exception that ended the ractor.
func (r *Ractor) reraise() {
	if r.err == nil {
		return
	}
	if e, ok := r.err.(ExceptionI); ok {
		panic(NewRactor_RemoteError(r, e))
	}
	panic(r.err)
}

// rbCopier is what Ractor messages copy through: generated on every struct class (emitCopy), written here for Array and Hash and for the classes MRI refuses to copy.
type rbCopier interface{ _Copy(seen map[any]any) any }

// rbRactorCopy deep-copies a message or Ractor.new argument, as MRI does for a non-shareable object: frozen Arrays/Hashes, values, classes and ractors are shared; identity inside the graph is kept (seen).
func rbRactorCopy(v any) any { return rbCopyDeep(v, map[any]any{}) }

func rbCopyDeep(v any, seen map[any]any) any {
	c, ok := v.(rbCopier)
	if !ok || rbIsFrozen(v) {
		return v
	}
	if dup, ok := seen[v]; ok {
		return dup
	}
	return c._Copy(seen)
}

// rbCopyAs copies a typed value; a nil interface stays nil, since asserting it to an interface type would panic.
func rbCopyAs[T any](v T, seen map[any]any) T {
	a := any(v)
	if a == nil {
		return v
	}
	return rbCopyDeep(a, seen).(T)
}

func (self *Array[E]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	out := &Array[E]{s: make([]E, len(self.s))}
	seen[self] = out
	for i, x := range self.s {
		out.s[i] = rbCopyAs(x, seen)
	}
	return out
}

func (self *Hash[K, V]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	out := NewHash[K, V]()
	seen[self] = out
	for k, hv := range rbHashPairs(self) {
		Hash_Op_idxSet(out, rbCopyAs(k, seen), rbCopyAs(hv, seen))
	}
	return out
}

// A Set copies only while every element is shareable, as MRI's does.
func (self *Set[E]) _Copy(seen map[any]any) any {
	if self == nil {
		return self
	}
	for k := range rbHashPairs(self.h) {
		if _, ok := any(k).(rbCopier); ok && !rbIsFrozen(k) {
			panic(NewRactor_Error(Ref(String("can not copy Set object."))))
		}
	}
	out := &Set[E]{h: NewHash[E, Boolean]()}
	seen[self] = out
	for k, hv := range rbHashPairs(self.h) {
		Hash_Op_idxSet(out.h, k, hv)
	}
	return out
}

// What MRI cannot copy into a ractor, with its errors.
func (*Queue[E]) _Copy(map[any]any) any {
	panic(NewNoMethodError(Ref(String("undefined method 'initialize_copy' for an instance of Thread::Queue"))))
}

func (*SizedQueue[E]) _Copy(map[any]any) any {
	panic(NewNoMethodError(Ref(String("undefined method 'initialize_copy' for an instance of Thread::SizedQueue"))))
}

func (*Thread) _Copy(map[any]any) any {
	panic(NewTypeError(Ref(String("allocator undefined for Thread"))))
}
