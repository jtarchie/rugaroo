//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"bufio"
	"fmt"
	"iter"
	"math"
	"math/bits"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"
	"weak"
)

var stdout = bufio.NewWriter(os.Stdout)

// stdoutMu serializes writes: threads are goroutines, and there is no GVL.
var stdoutMu sync.Mutex

// stdoutTTY: MRI flushes a terminal's stdout on every write, so prompts and progress show at once; pipes stay buffered, as in MRI.
var stdoutTTY = rbIsTTY(os.Stdout)

// rbIsTTY is isatty(3) for IO#tty? and stdout's flushing: /dev/null is a character device too, but not a terminal (decision 401).
// ponytail: other character devices (/dev/zero, a serial port) still count; a real isatty needs per-OS ioctl constants.
func rbIsTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	nul, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, nul)
}

// stdoutSync is `$stdout.sync = true`: flush every write, terminal or not.
var stdoutSync atomic.Bool

// rbWriteOut is Kernel#puts/print/p's write: to $stdout's object when one is assigned (decision 109), else stdout.
func rbWriteOut(s string) {
	if rbRedirected.Load() {
		if t := rbRedirectTarget(1); t != nil {
			t.Write(String(s))
			return
		}
	}
	if c := rbStdoutConv.Load(); c != nil { // STDOUT's own encoding, not a redirect target's
		s = (*c)(s)
	}
	rbWrite(s)
}

func rbWrite(s string) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	_, _ = stdout.WriteString(s)
	if stdoutTTY || stdoutSync.Load() {
		_ = stdout.Flush()
	}
}

// `$stdout = io` / `$stderr = io` (decision 109): Kernel's output and
// writes through `$stdout`/`$stderr` go to the object, through its
// `write`, until an IO on that fd is assigned back; STDOUT itself keeps
// writing to the real stream, as in MRI. Reading `$stdout` while one is
// assigned gives an IO bound to the object (rbStdoutIO), so `orig =
// $stdout` restores exactly it. A program that never assigns pays one
// atomic load per Kernel write.
var (
	rbRedirected atomic.Bool
	rbRedirectMu sync.RWMutex
	rbRedirects  [3]rbWriter
)

type rbWriter interface{ Write(any) Integer }

func rbRedirectTarget(fd int) rbWriter {
	rbRedirectMu.RLock()
	defer rbRedirectMu.RUnlock()
	return rbRedirects[fd]
}

func rbSetStdout(v any) any { return rbRedirect(1, v, "$stdout") }
func rbSetStderr(v any) any { return rbRedirect(2, v, "$stderr") }

// rbRS is `$/`, the input record separator chomp defaults to (decision 180).
var rbRS = Ref(String("\n"))

// rbFS, rbOFS and rbORS are `$;`, `$,` and `$\`: split's default pattern, join's and print's separator, and what print
// writes last; nil unless the program sets them (decision 414).
var rbFS, rbOFS, rbORS *String

// rbVerbose is `$VERBOSE`: false by default, nil silences Kernel#warn.
var rbVerbose = Ref(Boolean(false))

func rbStdoutIO() *IO { return rbStreamIO(1, STDOUT) }
func rbStderrIO() *IO { return rbStreamIO(2, STDERR) }

func rbStreamIO(fd int, real *IO) *IO {
	if rbRedirected.Load() {
		if t := rbRedirectTarget(fd); t != nil {
			return &IO{fd: fd, via: t}
		}
	}
	return real
}

func rbRedirect(fd int, v any, name string) any {
	var w rbWriter
	switch x := rbUnbox(v).(type) {
	case *IO:
		switch {
		case x.own || x.fd != fd: // STDERR (or a pipe) as $stdout: writes go to it
			w = x
		case x.via != nil: // an earlier $stdout, bound to its object
			w = x.via
		}
	case rbWriter:
		w = x
	default:
		panic(NewTypeError(Ref(String(name + " must have write method, " + rbClassName(v) + " given"))))
	}
	rbRedirectMu.Lock()
	rbRedirects[fd] = w
	rbRedirected.Store(rbRedirects[1] != nil || rbRedirects[2] != nil)
	rbRedirectMu.Unlock()
	return v
}

// rbFlushIfTTY keeps a terminal's stdout ahead of stderr; on a pipe MRI leaves it buffered.
func rbFlushIfTTY() {
	if stdoutTTY {
		rbFlush()
	}
}

var rbStdin = bufio.NewReader(os.Stdin)

func (self *File) rbOpen() {
	if self.f == nil {
		panic(NewIOError(Ref[String]("closed stream")))
	}
}

// rbNum2Int is MRI's NUM2INT: a value past a C int is a RangeError.
func rbNum2Int(n Integer) int {
	if n > math.MaxInt32 || n < math.MinInt32 {
		panic(NewRangeError(Ref(String("integer " + strconv.FormatInt(int64(n), 10) + " too " + map[bool]string{true: "big", false: "small"}[n > 0] + " to convert to 'int'"))))
	}
	return int(n)
}

func (self *File) rbReadable() {
	if self.f == nil {
		panic(NewIOError(Ref[String]("closed stream")))
	}
	if self.r == nil {
		panic(NewIOError(Ref[String]("not opened for reading")))
	}
}

func (self *StringIO) rbReadable() {
	if !self.readOpen {
		panic(NewIOError(Ref[String]("not opened for reading")))
	}
}

func (self *StringIO) rbWritable() {
	if !self.writeOpen {
		panic(NewIOError(Ref[String]("not opened for writing")))
	}
}

func rbFlush() {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	_ = stdout.Flush()
}

// rbTrapSignals starts the signal loop. Untrapped, SIGINT/SIGTERM flush
// stdout and then kill the program by the signal (exit 130/143 in a shell),
// as MRI's default does; Kernel#trap replaces that per signal (decision 100).
// ponytail: MRI raises Interrupt/SignalException in the main thread, so rescue and ensure run; Go can't inject a panic into another goroutine.
func rbTrapSignals() {
	rbSetGoTag(&rbGoTag{id: rbMainGoID}) // main's tag, which goroutines it starts carry until they tag themselves
	signal.Notify(rbSigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range rbSigCh {
			if rbTrapHook != nil {
				rbSpawned.Store(true) // the hook runs Ruby here, off main
				rbTagGoroutine()
			}
			if rbTrapHook != nil && rbTrapHook(sig) {
				continue
			}
			rbInterruptPost(sig)
		}
	}()
}

// Catchable Ctrl-C (decision 60). An untrapped SIGINT/SIGTERM is posted
// here; a blocking call on the main goroutine (sleep, Thread#join/value,
// Queue#pop, ConditionVariable#wait) takes it (rbTakeInterrupt) and raises
// Interrupt/SignalException there, so rescue and ensure run as in MRI.
// Go cannot interrupt a computing goroutine, so after rbInterruptGrace
// with nothing taking it the program dies by the signal, as before.
var (
	rbMainGoID       = rbGoIDStack() // package init runs on the main goroutine
	rbInterruptMu    sync.Mutex
	rbInterruptSig   os.Signal             // pending, not yet taken
	rbInterruptCh    = make(chan struct{}) // closed while one is pending; a fresh one after it is taken
	rbInterruptTimer *time.Timer
	rbInterruptWake  sync.Map // blocked main-goroutine waiters that need a nudge (a Queue's cond), by owner → func()
)

const rbInterruptGrace = 200 * time.Millisecond

func rbInterruptPost(sig os.Signal) {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	if rbInterruptSig != nil {
		return
	}
	rbInterruptSig = sig
	close(rbInterruptCh)
	rbInterruptTimer = time.AfterFunc(rbInterruptGrace, func() { rbDieBySignal(sig) })
	rbInterruptWake.Range(func(_, f any) bool {
		f.(func())()
		return true
	})
}

// rbInterruptC is the channel a blocking call selects on: closed while a signal is pending.
func rbInterruptC() <-chan struct{} {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	return rbInterruptCh
}

func rbInterruptPending() bool {
	rbInterruptMu.Lock()
	defer rbInterruptMu.Unlock()
	return rbInterruptSig != nil
}

// rbTakeInterrupt raises the pending signal on the main goroutine as
// Interrupt (SIGINT) or SignalException; on another goroutine, or with
// none pending, it returns.
func rbTakeInterrupt() {
	if rbGoID() != rbMainGoID {
		return
	}
	rbInterruptMu.Lock()
	sig := rbInterruptSig
	if sig == nil {
		rbInterruptMu.Unlock()
		return
	}
	rbInterruptSig = nil
	rbInterruptTimer.Stop()
	rbInterruptCh = make(chan struct{})
	rbInterruptMu.Unlock()
	if sig == os.Interrupt {
		panic(NewInterrupt(nil))
	}
	panic(NewSignalException(Integer(sig.(syscall.Signal))))
}

// rbDieBySignal flushes and ends the program by sig's default action, as
// MRI does for an unhandled one; 128+sig if the signal is ignored (a
// background job) and does not end it.
func rbDieBySignal(sig os.Signal) {
	rbFlush()
	signal.Reset(sig)
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(sig)
	time.Sleep(100 * time.Millisecond)
	os.Exit(128 + int(sig.(syscall.Signal)))
}

// rbTrapHook consults the trap table; Kernel#trap sets it, so a program without trap carries no table (decision 49).
var rbTrapHook func(sig os.Signal) bool

// rbTrapped runs or ignores a trapped signal, reporting whether sig had a handler.
func rbTrapped(sig os.Signal) bool {
	rbTrapMu.Lock()
	h, ok := rbTraps[sig]
	rbTrapMu.Unlock()
	switch {
	case ok && h.blk != nil:
		rbRunTrap(h.blk, sig)
	case ok && h.cmd == "IGNORE":
	default:
		return false
	}
	return true
}

// rbTrap is one Kernel#trap handler: a block, or a command string.
type rbTrap struct {
	blk func(Integer)
	cmd string
}

var (
	rbSigCh  = make(chan os.Signal, 8)
	rbTrapMu sync.Mutex
	rbTraps  = map[os.Signal]rbTrap{}
)

// rbRunTrap runs a trap block on the signal goroutine (MRI: the main
// thread). An exception from it ends the program as an uncaught one.
func rbRunTrap(blk func(Integer), sig os.Signal) {
	defer func() {
		if r := recover(); r != nil {
			status, e := rbExitStatus(rbWrapPanic(r))
			rbFinish(status, e)
			os.Exit(0)
		}
	}()
	blk(Integer(sig.(syscall.Signal)))
}

var rbSignalNums = map[string]syscall.Signal{
	"HUP": syscall.SIGHUP, "INT": syscall.SIGINT, "QUIT": syscall.SIGQUIT, "ILL": syscall.SIGILL,
	"TRAP": syscall.SIGTRAP, "ABRT": syscall.SIGABRT, "IOT": syscall.SIGABRT, "BUS": syscall.SIGBUS,
	"FPE": syscall.SIGFPE, "KILL": syscall.SIGKILL, "USR1": syscall.SIGUSR1, "SEGV": syscall.SIGSEGV,
	"USR2": syscall.SIGUSR2, "PIPE": syscall.SIGPIPE, "ALRM": syscall.SIGALRM, "TERM": syscall.SIGTERM,
	"CHLD": syscall.SIGCHLD, "CONT": syscall.SIGCONT, "STOP": syscall.SIGSTOP, "TSTP": syscall.SIGTSTP,
	"TTIN": syscall.SIGTTIN, "TTOU": syscall.SIGTTOU, "URG": syscall.SIGURG, "XCPU": syscall.SIGXCPU,
	"XFSZ": syscall.SIGXFSZ, "VTALRM": syscall.SIGVTALRM, "PROF": syscall.SIGPROF, "WINCH": syscall.SIGWINCH,
	"IO": syscall.SIGIO, "SYS": syscall.SIGSYS, "EXIT": 0,
}

// rbSignalArg reads a signal as MRI's trap and Process.kill take it: a
// name with or without SIG (String or Symbol) or a number.
func rbSignalArg(v any) syscall.Signal {
	switch v := rbUnbox(v).(type) {
	case Integer:
		for _, s := range rbSignalNums {
			if int(s) == int(v) {
				return s
			}
		}
		panic(NewArgumentError(Ref(String(fmt.Sprintf("invalid signal number (%d)", v)))))
	case String, Symbol:
		if s := fmt.Sprint(v); strings.HasPrefix(s, "-") {
			panic(NewArgumentError(Ref(String("negative signal name: " + s))))
		}
		name := strings.TrimPrefix(fmt.Sprint(v), "SIG")
		if s, ok := rbSignalNums[name]; ok {
			return s
		}
		panic(NewArgumentError(Ref(String("unsupported signal 'SIG" + name + "'"))))
	}
	panic(NewArgumentError(Ref(String("bad signal type " + rbClassName(v)))))
}

// rbSetTrap is Kernel#trap: it installs blk or cmd ("IGNORE", "DEFAULT",
// "SYSTEM_DEFAULT", "EXIT" or "") and returns the previous command, nil
// when that was a block (MRI returns the Proc).
func rbSetTrap(v any, blk func(Integer), cmd string) *String {
	sig := rbSignalArg(v)
	switch {
	case sig == syscall.SIGKILL || sig == syscall.SIGSTOP:
		panic(NewErrno_EINVAL(Ref(String("Invalid argument - SIG" + rbSignalName(sig)))))
	case slices.Contains([]syscall.Signal{syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGILL, syscall.SIGFPE, syscall.SIGVTALRM}, sig):
		panic(NewArgumentError(Ref(String("can't trap reserved signal: SIG" + rbSignalName(sig)))))
	case sig == 0: // "EXIT": an at_exit handler
		if blk != nil {
			rbAtExitPush(func() { blk(0) })
		}
		return nil
	}
	switch {
	case blk != nil:
	case cmd == "SIG_DFL":
		cmd = "DEFAULT"
	case cmd == "SIG_IGN" || cmd == "":
		cmd = "IGNORE"
	}
	// MRI installs its own handler for these, which DEFAULT names; for the rest DEFAULT is the OS's (SYSTEM_DEFAULT)
	rubyOwn := slices.Contains([]syscall.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGALRM, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGCHLD}, sig)
	rbTrapMu.Lock()
	defer rbTrapMu.Unlock()
	rbTrapHook = rbTrapped
	old, had := rbTraps[sig]
	rbTraps[sig] = rbTrap{blk: blk, cmd: cmd}
	switch {
	case blk != nil || cmd == "IGNORE":
		signal.Notify(rbSigCh, sig)
	case sig != syscall.SIGINT && sig != syscall.SIGTERM:
		signal.Reset(sig)
	}
	switch {
	case !had && sig == syscall.SIGPIPE: // MRI ignores SIGPIPE itself and reports it as nil
		return nil
	case had && old.blk != nil:
		return nil
	case rubyOwn && (!had || old.cmd == "DEFAULT"):
		return Ref(String("DEFAULT"))
	case !had || old.cmd == "DEFAULT":
		return Ref(String("SYSTEM_DEFAULT"))
	}
	return Ref(String(old.cmd))
}

func rbSignalName(sig syscall.Signal) string {
	for n, s := range rbSignalNums {
		if s == sig && n != "IOT" {
			return n
		}
	}
	return strconv.Itoa(int(sig))
}

// rbAtExit holds Kernel#at_exit handlers; rbTopRecover pops them LIFO, so
// one registered while handlers run is next.
var (
	rbAtExitMu sync.Mutex
	rbAtExit   []func()
)

func rbAtExitPush(f func()) {
	rbAtExitMu.Lock()
	defer rbAtExitMu.Unlock()
	rbAtExit = append(rbAtExit, f)
}

func rbAtExitPop() func() {
	rbAtExitMu.Lock()
	defer rbAtExitMu.Unlock()
	if len(rbAtExit) == 0 {
		return nil
	}
	f := rbAtExit[len(rbAtExit)-1]
	rbAtExit = rbAtExit[:len(rbAtExit)-1]
	return f
}

// rbTopRecover ends the program as MRI does: at_exit handlers run first;
// then the uncaught exception, if any, prints `msg (Class)`. The status is
// 1 after an uncaught exception, SystemExit's status after exit, and a
// handler's exit or exception replaces it. Output is flushed first so
// partial output before a crash matches.
func rbTopRecover() {
	status, main := 0, any(nil)
	if r := recover(); r != nil {
		if rbFullReport {
			r = rbCaptureBacktrace(r) // the panicking frames are still on the stack here
		}
		status, main = rbExitStatus(r)
	}
	rbFinish(status, main)
}

// rbFullReport: RUGAROO_BACKTRACE set prints an uncaught exception as MRI does, `file:line:in 'x': msg (Class)` and its `from` lines, where the default is `msg (Class)` alone (decision 106).
var rbFullReport = os.Getenv("RUGAROO_BACKTRACE") != ""

// rbFinish runs the at_exit handlers, prints main (an uncaught exception
// or nil) and exits with the resulting status; it returns only for 0.
func rbFinish(status int, main any) {
	for f := rbAtExitPop(); f != nil; f = rbAtExitPop() {
		rbExitStatusNow.Store(int64(status))
		func() {
			defer func() {
				if r := recover(); r != nil {
					var err any
					if status, err = rbExitStatus(r); err != nil {
						rbFlushIfTTY()
						rbPrintUncaught(err)
					}
				}
			}()
			f()
		}()
	}
	rbFlush()
	if main != nil {
		rbPrintUncaught(main)
		if se, ok := main.(interface{ Signo() Integer }); ok { // an uncaught Interrupt ends the program by its signal, as in MRI
			rbDieBySignal(syscall.Signal(se.Signo()))
		}
	}
	if status != 0 {
		os.Exit(status)
	}
}

// rbExitStatusNow is the status the program would exit with, for at_exit handlers.
var rbExitStatusNow atomic.Int64

// rbIsRef reports whether a's address is its identity: its class's Go type is a pointer (rbClassRefs, decision 89).
func rbIsRef(a any) bool {
	v, ok := a.(interface{ _ClassID() int })
	return ok && rbClassRefs[v._ClassID()]
}

// rbObjectID is Kernel#object_id: a pointer's address, else the value's hash.
func rbObjectID(a any) Integer {
	if rbIsRef(a) {
		return Integer(rbHashComparable(a) >> 2) //nolint:gosec // an id, not arithmetic
	}
	return rbHash(a) & (1<<62 - 1)
}

// rbExitStatus is the exit status r ends the program with, and r itself
// when it is an error to print rather than a SystemExit.
func rbExitStatus(r any) (int, any) {
	r = rbWrapPanic(r) // a Go runtime panic (an Integer divide by 0, decision 151) ends the program as the exception it stands for
	if e, ok := r.(SystemExitI); ok {
		return int(e.Status()), nil
	}
	return 1, r
}

func rbPrintUncaught(r any) {
	if e, ok := r.(ExceptionI); ok {
		if bt := rbBacktrace(e); rbFullReport && bt != nil && len((*bt).s) > 0 {
			fmt.Fprintf(os.Stderr, "%s: %s (%s)\n", (*bt).s[0], e.Message(), rbClassName(r))
			for _, l := range (*bt).s[1:] {
				fmt.Fprintf(os.Stderr, "\tfrom %s\n", l)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "%s (%s)\n", e.Message(), rbClassName(r))
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", r)
	}
}

// rbArg is a dispatcher's args[i], once rbArity has checked the count (dynArg).
func rbArg(args []any, i int) any {
	if i >= len(args) { // a gem lambda's optional param the call left out: nil, whose default the body fills (decision 416)
		return nil
	}
	return args[i]
}

// Ref boxes a value into T? (represented as *T).
func Ref[T any](v T) *T { return &v }

// rbSplatElem is element i of a splat into positional parameters, or the zero value past its end (decision 184).
func rbSplatElem[E comparable](a *Array[E], i int) E {
	if i < len(a.s) {
		return a.s[i]
	}
	var z E
	return z
}

// rbSplatArity raises MRI's ArgumentError when a splat leaves n arguments outside min..max.
func rbSplatArity(n, lo, hi int) {
	if n >= lo && n <= hi {
		return
	}
	want := strconv.Itoa(lo)
	if hi != lo {
		want += ".." + strconv.Itoa(hi)
	}
	panic(NewArgumentError(Ref(String(fmt.Sprintf("wrong number of arguments (given %d, expected %s)", n, want)))))
}

// rbRubyPlatform is RUBY_PLATFORM: MRI's names for Go's GOARCH and GOOS, without the OS version MRI appends.
func rbRubyPlatform() string {
	arch := map[string]string{"amd64": "x86_64", "386": "i386"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	return arch + "-" + runtime.GOOS
}

// rbZero fills a left-out argument; the callee sees rbArgc and runs its own default.
func rbZero[T any]() (z T) { return z }

// Opt converts T? to untyped: a nil *T must become an untyped nil or a
// `case nil` type switch misses it.
func Opt[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// rbOpt is Integer?/Float? held by value (zero is nil): no alias into an Array's storage, no allocation (decision 396).
type rbOpt[T any] struct {
	v  T
	ok bool
}

// rbSome is a T where T? is wanted.
func rbSome[T any](v T) rbOpt[T] { return rbOpt[T]{v, true} }

// rbOptFromPtr copies generic code's *T (Array#[]'s pointer into its storage) into a T?: the copy decision 396 is for.
func rbOptFromPtr[T any](p *T) rbOpt[T] {
	if p == nil {
		return rbOpt[T]{}
	}
	return rbOpt[T]{*p, true}
}

// rbOptToPtr is a T? where generic code takes V? as Go *V.
func rbOptToPtr[T any](o rbOpt[T]) *T {
	if !o.ok {
		return nil
	}
	return &o.v
}

// rbAny is the T? as untyped: nil, or the value.
func (o rbOpt[T]) rbAny() any {
	if !o.ok {
		return nil
	}
	return o.v
}

type rbOptCmp interface{ rbCmpWith(b any) Integer }

func (o rbOpt[T]) rbCmpWith(b any) Integer {
	p, _ := b.(rbOpt[T])
	if !o.ok || !p.ok {
		return rbCmpFailed(o.rbAny(), p.rbAny())
	}
	return rbCmp(any(o.v), any(p.v))
}

type rbOptMSet interface{ rbMSet(v any) }

func (o *rbOpt[T]) rbMSet(v any) {
	if v == nil {
		*o = rbOpt[T]{}
		return
	}
	*o = rbOpt[T]{rbMAs[T](v), true}
}

// rbOptOf converts untyped to T? (OptOf for an rbOpt).
func rbOptOf[T any](a any, want string) rbOpt[T] {
	if a == nil {
		return rbOpt[T]{}
	}
	return rbOpt[T]{rbAs[T](a, want), true}
}

// rbOptArrayFrom opens a generic Array[E?] with E = Integer or Float: each *T slot to a T?.
func rbOptArrayFrom[T comparable](a *Array[*T]) *Array[rbOpt[T]] {
	out := make([]rbOpt[T], len(a.s))
	for i, p := range a.s {
		out[i] = rbOptFromPtr(p)
	}
	return &Array[rbOpt[T]]{s: out}
}

// rbOptPairFrom opens a generic [E?, E?] (minmax) with E = Integer or Float.
func rbOptPairFrom[A, B comparable](t Tuple2[*A, *B]) Tuple2[rbOpt[A], rbOpt[B]] {
	return Tuple2[rbOpt[A], rbOpt[B]]{rbOptFromPtr(t.F0), rbOptFromPtr(t.F1)}
}

// rbFlat collapses a generic E? instantiated with E = T? (a **T) to T?:
// Ruby has one nil.
func rbFlat[T any](p **T) *T {
	if p == nil {
		return nil
	}
	return *p
}

// OptOf converts untyped to T?; want names T for rbAs's TypeError.
func OptOf[T any](a any, want string) *T {
	if a == nil {
		return nil
	}
	v := rbAs[T](a, want)
	return &v
}

// rbFlatNil opens a generic V? whose V is a class that holds nil itself (a leaf's *C, decision 360; any other struct
// class's interface CI, decision 378): Go's *V to the V its C? is.
func rbFlatNil[T any](p *T) T {
	if p == nil {
		var z T
		return z
	}
	return *p
}

// rbFlatPair opens a generic [E?, F?] whose E and F hold nil themselves: Go's two *V slots, to the V each C? is.
func rbFlatPair[A, B comparable](t Tuple2[*A, *B]) Tuple2[A, B] {
	return Tuple2[A, B]{rbFlatNil(t.F0), rbFlatNil(t.F1)}
}

// rbFlatArray opens a generic Array[E?] whose E holds nil itself: each *V slot to the V its C? is.
func rbFlatArray[T comparable](a *Array[*T]) *Array[T] {
	out := make([]T, len(a.s))
	for i, p := range a.s {
		out[i] = rbFlatNil(p)
	}
	return &Array[T]{s: out}
}

// rbBoxNil is a C? that holds nil itself (*C or CI) where generic code takes V? as Go *V.
func rbBoxNil[T comparable](v T) *T {
	var z T
	if v == z {
		return nil
	}
	return &v
}

// rbOptNil is OptOf for a class whose C? is its own Go type (*C or CI), nil its nil.
func rbOptNil[T any](a any, want string) T {
	if a == nil {
		var z T
		return z
	}
	return rbAs[T](a, want)
}

// rbIfaceOf is a leaf's C? (*C) where a superclass's C? (its interface I) is wanted: nil stays nil, not a typed nil in I.
func rbIfaceOf[I any, T any](p *T) I {
	if p == nil {
		var z I
		return z
	}
	return any(p).(I) //nolint:forcetypeassert // *T implements I: the compiler emits this only for a subclass
}

// rbAs converts untyped to T, raising MRI's TypeError when a is not one:
// a bare a.(T) would panic as a Go error, a StandardError.
func rbAs[T any](a any, want string) T {
	v, ok := a.(T)
	if !ok {
		return rbAsSlow[T](a, want)
	}
	return v
}

// rbTryBox is rbMBox without Marshal's TypeError on a mismatch, so rbAsSlow's own fallbacks and message still apply.
func rbTryBox(dst, v any) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return rbMBox(dst, v)
}

// rbAsOrZero is rbAs without the TypeError: a value that fits T, else T's zero (a gem override's untyped result under
// a typed slot whose callers ignore it).
func rbAsOrZero[T any](a any) T {
	if v, ok := a.(T); ok {
		return v
	}
	if v, ok := rbConv[T](a); ok {
		return v
	}
	var z T
	return z
}

// rbAsSlow converts as rbConv does (nil for untyped, other Array/Hash
// instantiations, an Array into a tuple), else raises.
func rbAsSlow[T any](a any, want string) T {
	if v, ok := rbConv[T](a); ok {
		return v
	}
	if v, ok := rbUnbox(a).(T); ok { // a T? held untyped is a *T box (liquid's `markup = $2` then `markup =~ re`, decision 331)
		return v
	}
	var zero T
	if a != nil && rbTryBox(&zero, a) { // T is a box where generic code binds a type parameter to C? (Homebrew's set_unique_stanza with U = DSL::Version?)
		return zero
	}
	if f, ok := a.(**func(...any) any); ok && f != nil { // an optional &block's Proc? box held untyped (Homebrew's stored postflight block cast back)
		if t, ok := any(*f).(T); ok {
			return t
		}
	}
	if a == nil && any(zero) == nil { // an object type (an interface) holds nil, as MRI passes nil anywhere: a call on it raises (decision 253)
		return zero
	}
	// a Pathname where a String is taken (a dynamic File.basename(self)): MRI's path parameters call to_path. ponytail:
	// any String parameter takes it here, where MRI's non-path ones raise TypeError (decision 254)
	switch p := a.(type) { // a single-type case: dropped from programs without Pathname
	case *Pathname:
		if t, ok := any(p.ToS()).(T); ok {
			return t
		}
	}
	if o, ok := rbDelegateObj(a); ok && o != nil { // a Delegator (Homebrew's Dependencies) where its object's class is wanted: the object, as a Sorbet T::Array check lets it through
		if t, ok := o.(T); ok {
			return t
		}
		if t, ok := rbConv[T](o); ok {
			return t
		}
	}
	if want == "element" { // stored into a typed container through the untyped view: MRI's Array keeps the value as it is, so
		// no to_int/to_str (Float#to_int put 0 for matrix's eigenvectors' 0.82; decision 420)
		panic(rbConvError(a, want))
	}
	if t, ok := rbImplicitConv[T](a); ok {
		return t
	}
	panic(rbConvError(a, want))
}

// rbImplicitConv is MRI's implicit to_int/to_str of an object where an Integer or String is taken (bindata's
// primitives answer them through method_missing; decision 324).
func rbImplicitConv[T any](a any) (T, bool) {
	var zero T
	name := ""
	switch any(zero).(type) {
	case Integer:
		name = "to_int"
		if r, ok := a.(interface{ DynToInt(...any) any }); ok {
			return rbConv[T](r.DynToInt())
		}
	case String:
		name = "to_str"
		if r, ok := a.(interface{ DynToStr(...any) any }); ok {
			return rbConv[T](r.DynToStr())
		}
	}
	if r, ok := a.(interface{ DynMethodMissing(...any) any }); ok && name != "" {
		return rbConv[T](r.DynMethodMissing(Symbol(name)))
	}
	return zero, false
}

// rbConvError is MRI's "no implicit conversion" TypeError.
func rbConvError(a any, want string) any {
	name := rbClassName(a)
	switch r := a.(type) {
	case nil:
		name = "nil"
	case Boolean:
		name = strconv.FormatBool(bool(r))
	case rbModule:
		name = strings.ToUpper(r._Kind()[:1]) + r._Kind()[1:]
	}
	return NewTypeError(Ref(String("no implicit conversion of " + name + " into " + want)))
}

type I_ToS interface{ ToS() String }
type I_Inspect interface{ Inspect() String }

// rbToS, rbInspect, rbEq and rbCmp take E = T? values from generic
// code as the box itself; rbUnbox (generated) opens it.
func rbToS(a any) String {
	a = rbUnbox(a)
	if a == nil {
		return ""
	}
	if s, ok := a.(I_ToS); ok {
		return s.ToS()
	}
	return rbObjToS(a)
}

// rbObjToS is Kernel#to_s; only heap objects have an address to show.
func rbObjToS(a any) String {
	if a == any(rb_main) { // MRI's main has a singleton to_s and inspect
		return "main"
	}
	s := "#<" + rbClassName(a)
	if rbIsRef(a) {
		addr := strings.TrimPrefix(fmt.Sprintf("%p", a), "0x") // %p is a pointer's address
		s += ":0x" + strings.Repeat("0", max(16-len(addr), 0)) + addr
	}
	return String(s + ">")
}

// rbIvar feeds Kernel#inspect; !opt means nil was never assigned, which MRI doesn't list.
type rbIvar struct {
	name  string
	val   any
	opt   bool
	isNil bool // a nil pointer or interface, decided from the field's type when _Ivars is generated
}

// rbObjInspect is Kernel#inspect; rbSeen gives MRI's "..." for an object that holds itself.
func rbObjInspect(a any) String { return rbObjInspectIn(a, nil) }

func rbObjInspectIn(a any, seen *rbSeen) String {
	o, ok := a.(interface{ _Ivars(name string) []rbIvar })
	if !ok {
		return rbObjToS(a)
	}
	s := string(rbObjToS(a))
	var b strings.Builder
	b.WriteString(s[:len(s)-1])
	seen = rbSeenOf(seen)
	if !seen.enter(a, nil) {
		return String(b.String() + " ...>")
	}
	defer seen.leave(a, nil)
	sep := " "
	for _, iv := range o._Ivars("") {
		if !iv.opt && (iv.val == nil || iv.isNil) {
			continue
		}
		b.WriteString(sep + iv.name + "=")
		sep = ", "
		if _, ok := iv.val.(I_Inspect); ok || iv.val == nil {
			b.WriteString(string(rbInspectIn(iv.val, seen)))
		} else { // a Go value behind a prelude ivar
			b.WriteString(string(rbObjToS(iv.val)))
		}
	}
	return String(b.String() + ">")
}

func rbInspect(a any) String {
	a = rbUnbox(a)
	if a == nil {
		return "nil"
	}
	if s, ok := a.(I_Inspect); ok {
		return s.Inspect()
	}
	return rbObjInspect(a) // a bare Object (Object.new) has only Kernel's
}

// rbSeen is the containers an inspect, ==, eql? or hash is inside, passed
// down the recursion (the _..._rec methods) so one that holds itself prints
// [...] and compares or hashes like MRI's rb_exec_recursive instead of
// overflowing Go's stack. Each top-level call owns its own, made only when
// it first reaches a nested container: nothing is shared between threads
// and nothing is locked. A cycle that passes through a user-defined inspect
// or == starts a fresh set there and is not caught (MRI's set is per
// thread; Go has no goroutine-locals).
type rbSeen struct{ m map[[2]any]struct{} }

// rbSeenOf is the set passed as untyped (a prelude method's parameter), or a fresh one.
func rbSeenOf(seen any) *rbSeen {
	if s, ok := seen.(*rbSeen); ok && s != nil {
		return s
	}
	return &rbSeen{m: map[[2]any]struct{}{}}
}

// enter marks (a, b) as being worked on; false means it already is.
func (s *rbSeen) enter(a, b any) bool {
	k := [2]any{a, b}
	if _, ok := s.m[k]; ok {
		return false
	}
	s.m[k] = struct{}{}
	return true
}

func (s *rbSeen) leave(a, b any) { delete(s.m, [2]any{a, b}) }

// The containers' recursion-aware forms, which each top-level method calls with no set.
type (
	rbInspectRec interface{ _InspectRec(seen any) String }
	rbEqRec      interface{ _EqRec(other, seen any) Boolean }
	rbEqlRec     interface{ _EqlRec(other, seen any) Boolean }
	rbHashRec    interface{ _HashRec(seen any) Integer }
)

// rbInspectIn is rbInspect inside a container's inspect: nested containers,
// and objects whose inspect is Kernel's, carry the set on.
func rbInspectIn(a any, seen *rbSeen) String {
	a = rbUnbox(a)
	if r, ok := a.(rbInspectRec); ok {
		return r._InspectRec(seen)
	}
	if _, ivars := a.(interface{ _Ivars(name string) []rbIvar }); ivars {
		if id, ok := a.(interface{ _ClassID() int }); ok && rbKernelInspect[id._ClassID()] {
			return rbObjInspectIn(a, seen)
		}
	}
	return rbInspect(a)
}

func rbTruthy(a any) bool {
	switch v := a.(type) {
	case nil:
		return false
	case Boolean:
		return bool(v)
	}
	return true
}

// rbTruthyOpt tests a Boolean?: false is as falsy as nil.
func rbTruthyOpt(p *Boolean) bool { return p != nil && bool(*p) }

// rbOptEq is T? == T for a T whose == is Go's (plainOptEq): nil is never equal.
func rbOptEq[T comparable](p *T, v T) bool { return p != nil && *p == v }

// rbOptEqV is rbOptEq for an rbOpt (decision 396).
func rbOptEqV[T comparable](o rbOpt[T], v T) bool { return o.ok && o.v == v }

func rbEq[T comparable](a, b T) Boolean {
	x, y := rbUnbox(any(a)), rbUnbox(any(b))
	if e, ok := x.(interface{ Op_eq(any) Boolean }); ok {
		return e.Op_eq(y)
	}
	if e, ok := x.(interface{ _EqAny(any) Boolean }); ok {
		return e._EqAny(y) // a == typed on its argument; see emitEqAdapter
	}
	if e, ok := x.(interface{ Op_eq(any) any }); ok { // a gem's untyped == (bindata's Base: `other == snapshot`)
		return Boolean(rbTruthy(e.Op_eq(y)))
	}
	return Boolean(x == y)
}

// Hash keys, uniq and tally match by Ruby's eql?/hash. For most keys that
// is Go ==: strings, numbers, symbols, and objects by identity. Arrays,
// hashes, Regexps, Structs and anything else defining both eql? and hash
// match by value, and so does a T? box (*T) by what it points at.

// rbPlainKey reports whether K's Go == is eql?, so a map can key by K.
func rbPlainKey[K comparable]() bool {
	var z K
	switch z := any(z).(type) {
	case String, Symbol, Integer, Float, Boolean:
		return true
	case interface{ rbPlain() bool }:
		return z.rbPlain()
	}
	return false
}

type rbEqlHash interface {
	EqlQ(other any) Boolean
	Hash() Integer
}

// rbValueKey is k's hash when k matches by value rather than Go ==.
func rbValueKey(k any) (uint64, bool) {
	if p, ok := rbKeyUnbox(k); ok {
		return rbKeyHash(p), true
	}
	if v, ok := k.(rbEqlHash); ok {
		return uint64(v.Hash()), true
	}
	return 0, false
}

func rbKeyHash(k any) uint64 {
	if h, ok := rbValueKey(k); ok {
		return h
	}
	return rbHashComparable(k)
}

func rbKeyEql(a, b any) bool {
	if p, ok := rbKeyUnbox(a); ok {
		a = p
	}
	if p, ok := rbKeyUnbox(b); ok {
		b = p
	}
	if a == b {
		return true
	}
	if e, ok := a.(rbEqlHash); ok {
		return bool(e.EqlQ(b))
	}
	return false
}

// rbEql is #eql? on an Object-typed value: its class's eql?, else identity (Kernel's).
func rbEql(a, b any) Boolean {
	if e, ok := a.(interface{ EqlQ(any) Boolean }); ok {
		return e.EqlQ(b)
	}
	return Boolean(rbKeyEql(a, b))
}

// rbHash is #hash on an untyped value.
func rbHash(a any) Integer {
	if h, ok := a.(interface{ Hash() Integer }); ok {
		return h.Hash()
	}
	return Integer(rbKeyHash(a))
}

// rbCmp is <=> for sort, min and max. Typed values have Op_cmp(T);
// T? boxes compare their values (rbCmpBox); untyped ones (T is any) go
// through the generated DynOp_cmp wrappers (rbCmpFailed), which answer
// nil for an incomparable argument, as MRI's <=> does.
func rbCmp[T comparable](a, b T) Integer {
	switch x := any(a).(type) { // the common keys by a static call: the interface assertion below costs an itab lookup
	case Integer:
		if y, ok := any(b).(Integer); ok {
			return x.Op_cmp(y)
		}
	case String:
		if y, ok := any(b).(String); ok {
			return x.Op_cmp(y)
		}
	case Float:
		if y, ok := any(b).(Float); ok {
			return x.Op_cmp(y)
		}
	}
	if c, ok := any(a).(interface{ Op_cmp(T) Integer }); ok {
		return c.Op_cmp(b)
	}
	return rbCmpBox(any(a), any(b))
}

// rbCmpFunc is rbCmp for T, resolved once per sort or min/max scan. A
// tuple key (sort_by { [a, b] }) reaches its Op_cmp in rbCmp only through
// an interface, which copies the tuple to the heap on every comparison;
// its rbCmpPair method value takes both by value instead.
func rbCmpFunc[T comparable]() func(a, b T) Integer {
	var z T
	if c, ok := any(z).(interface{ rbCmpPair(T, T) Integer }); ok {
		return c.rbCmpPair
	}
	return rbCmp[T]
}

// rbCmpOp is Comparable's op on a `<=>` result c of a against b: MRI's ArgumentError when c is nil (decision 287).
func rbCmpOp(c any, op string, a, b any) Boolean {
	n, ok := rbUnbox(c).(Integer)
	if !ok {
		panic(rbCmpErr(a, b))
	}
	switch op {
	case "<":
		return n < 0
	case "<=":
		return n <= 0
	case ">":
		return n > 0
	}
	return n >= 0
}

// rbCmpOpt compares the values in two T? boxes (see rbCmpBox).
func rbCmpOpt[T comparable](a, b *T) Integer {
	if a == nil || b == nil {
		return rbCmpFailed(a, b)
	}
	return rbCmp(*a, *b)
}

// rbCmpFailed is <=> where no Op_cmp(T) applies: an untyped value asks
// its DynOp_cmp wrapper, and a nil answer raises MRI's rb_cmperr.
func rbCmpFailed(a, b any) Integer {
	a, b = rbUnbox(a), rbUnbox(b)
	if c, ok := a.(interface{ DynOp_cmp(...any) any }); ok {
		if r, ok := c.DynOp_cmp(b).(Integer); ok {
			return r
		}
	}
	panic(rbCmpErr(a, b))
}

// rbCmpErr is MRI's rb_cmperr, raised where <=> answered nil: it
// inspects immediates and Floats and names the class of anything else.
func rbCmpErr(a, b any) *ArgumentError {
	with := rbClassName(b)
	switch b.(type) {
	case nil, Boolean, Integer, Float, Symbol:
		with = string(rbInspect(b))
	}
	return NewArgumentError(Ref(String("comparison of " + rbClassName(a) + " with " + with + " failed")))
}

// rbCInt and rbIntDigitsArg are rbNum2Int under the names file.rb, io.rb and float.rb call.
func rbCInt(n Integer) int { return rbNum2Int(n) }

func rbIntDigitsArg(digits Integer) { rbNum2Int(digits) }

func rbIdentical(a, b any) bool {
	if as, ok := a.(String); ok {
		bs, ok := b.(String)
		// Identity of immutable strings is their backing pointer.
		return ok && len(as) == len(bs) && unsafe.StringData(string(as)) == unsafe.StringData(string(bs)) //nolint:gosec // pointer compare only
	}
	return a == b
}

// rbNewStr is a String method's result, a new object in MRI: when it is
// the receiver's own bytes (nothing to strip, replace or pad), it is
// copied, so equal? and frozen? don't take it for the receiver. Only that
// case pays for the copy.
// ponytail: other shared bytes still look identical (equal slices of one string, strconv's small-number table, a "" Go boxes to zeroVal); a boxed String would fix them.
func rbNewStr(recv, s String) String {
	if len(s) == len(recv) && unsafe.StringData(string(s)) == unsafe.StringData(string(recv)) { //nolint:gosec // pointer compare only
		return rbStrClone(s)
	}
	return s
}

// rbStrClone copies s to bytes of its own; "" gets an address of its own.
func rbStrClone(s String) String {
	if s == "" {
		return String(unsafe.String(new(byte), 0)) //nolint:gosec // an address for identity only
	}
	return String(strings.Clone(string(s)))
}

// rbStrID is a String's identity: backing pointer and length, as in rbIdentical.
type rbStrID struct {
	p *byte
	n int
}

// rbFrozenLits is every literal's bytes, fixed at start: read with no lock.
var rbFrozenLits = func() map[rbStrID]struct{} {
	m := make(map[rbStrID]struct{}, len(rbStringLits))
	for _, l := range rbStringLits {
		m[rbStrID{unsafe.StringData(l), len(l)}] = struct{}{} //nolint:gosec // identity only
	}
	return m
}()

// rbFrozenShards hold the computed strings String#freeze marked, by address and length, in 256 shards by address,
// each a plain map behind its own mutex (held for a map operation; no process-wide lock, decision 147). A freeze
// records the string itself; rbFrozenConvert, a goroutine, swaps each such entry for a weak pointer to its bytes
// soon after, off the freezing thread (weak.Make is ~400 ns, most of a freeze; decision 329). So the set pins a
// string only until then: an entry whose bytes died reads as unfrozen, so a new string at a reused address is not
// frozen, and a shard drops its dead entries when it has doubled. (The copy-on-write shards before copied about
// n/256 entries per freeze and kept every frozen string alive.)
var rbFrozenShards [256]struct {
	mu      sync.Mutex
	m       map[rbStrKey]rbFrozenEntry
	pending []rbStrKey // entries still holding their string, for rbFrozenConvert
	next    int        // the size that triggers the next sweep
	_       [16]byte   // a cache line each
}

// rbStrKey is rbStrID without the pointer, so the set does not keep the bytes alive.
type rbStrKey struct {
	p uintptr
	n int
}

// rbFrozenEntry is a frozen string's bytes: the string itself until rbFrozenConvert runs, then weakly, or immortal
// for Go's constant data, which weak.Make rejects.
type rbFrozenEntry struct {
	s        string
	strong   bool
	w        weak.Pointer[byte]
	immortal bool
}

// live reports whether the frozen bytes are still the ones at the key's address.
func (e rbFrozenEntry) live() bool { return e.immortal || e.strong || e.w.Value() != nil }

var (
	rbFrozenNoBytes atomic.Bool  // whether the "" with a nil data pointer was frozen
	rbFrozenPending atomic.Int64 // entries waiting for rbFrozenConvert
	rbFrozenWake    = make(chan struct{}, 1)
	rbFrozenStart   sync.Once
)

// The linker's bounds of read-only data, where Go's string constants live (strconv's digits, one-byte strings).
//
//go:linkname rbRodata runtime.rodata
var rbRodata byte

//go:linkname rbErodata runtime.erodata
var rbErodata byte

// rbStrFrozen reports whether s is a literal or was frozen, and with
// freeze also marks it. Strings built at run time have fresh backing
// arrays, so they aren't in the set, like MRI's unfrozen strings.
func rbStrFrozen(s string, freeze bool) bool {
	p := unsafe.StringData(s) //nolint:gosec // identity only
	if _, ok := rbFrozenLits[rbStrID{p, len(s)}]; ok {
		return true
	}
	if p == nil { // a "" with no bytes at all: one identity, as before
		if freeze {
			return rbFrozenNoBytes.Swap(true)
		}
		return rbFrozenNoBytes.Load()
	}
	key := rbStrKey{uintptr(unsafe.Pointer(p)), len(s)} //nolint:gosec // a key, never dereferenced
	sh := &rbFrozenShards[(key.p>>4^key.p>>12)&255]
	sh.mu.Lock()
	old, ok := sh.m[key]
	was := ok && old.live()
	if !freeze || was {
		sh.mu.Unlock()
		return was
	}
	if sh.m == nil {
		sh.m = map[rbStrKey]rbFrozenEntry{}
	}
	immortal := key.p >= uintptr(unsafe.Pointer(&rbRodata)) && key.p < uintptr(unsafe.Pointer(&rbErodata)) //nolint:gosec // address compare only
	if immortal {
		sh.m[key] = rbFrozenEntry{immortal: true}
	} else {
		sh.m[key] = rbFrozenEntry{s: s, strong: true}
		sh.pending = append(sh.pending, key)
	}
	if len(sh.m) >= max(sh.next, 64) { // a fresh map: deleting in place leaves tombstones that lengthen every probe
		live := make(map[rbStrKey]rbFrozenEntry, len(sh.m))
		for k, e := range sh.m {
			if e.live() {
				live[k] = e
			}
		}
		sh.m = live
		sh.next = 2 * len(sh.m)
	}
	sh.mu.Unlock()
	if !immortal && rbFrozenPending.Add(1)&255 == 0 { // wake the converter every 256 freezes
		rbFrozenStart.Do(func() { go rbFrozenConvert() })
		select {
		case rbFrozenWake <- struct{}{}:
		default:
		}
	}
	return false
}

// rbFrozenConvert turns each shard's pending entries into weak ones, a shard's batch at a time: weak.Make runs
// outside the shard's lock, and an entry replaced meanwhile (its string died and the address was refrozen) is left.
func rbFrozenConvert() {
	for range rbFrozenWake {
		for i := range rbFrozenShards {
			sh := &rbFrozenShards[i]
			sh.mu.Lock()
			keys := sh.pending
			sh.pending = nil
			ents := make([]rbFrozenEntry, len(keys))
			for k, key := range keys {
				ents[k] = sh.m[key]
			}
			sh.mu.Unlock()
			if len(keys) == 0 {
				continue
			}
			weaks := make([]weak.Pointer[byte], len(keys))
			for k, e := range ents {
				if e.strong {
					weaks[k] = weak.Make(unsafe.StringData(e.s)) //nolint:gosec // its own bytes
				}
			}
			sh.mu.Lock()
			for k, key := range keys {
				if cur, ok := sh.m[key]; ok && cur.strong && ents[k].strong {
					sh.m[key] = rbFrozenEntry{w: weaks[k]}
				}
			}
			sh.mu.Unlock()
			rbFrozenPending.Add(-int64(len(keys)))
		}
	}
}

func rbIsA[I any](r any) bool {
	_, ok := r.(I)
	return ok
}

// rbSplat converts a splatted array's elements to a rest param's type.
func rbSplat[T, E any](s []T, conv func(T) E) []E {
	out := make([]E, len(s))
	for i, v := range s {
		out[i] = conv(v)
	}
	return out
}

func rbClassName(a any) string {
	if id := rbClassID(a); id >= 0 {
		return rbClassNames[id]
	}
	return "Object" // a Go value behind a prelude ivar
}

// rbClassID is the generated ID of a's class (decision 82): nil and procs
// by value, a T? box by what it holds, anything else by its _ClassID.
func rbClassID(a any) int {
	switch v := rbUnbox(a).(type) {
	case nil:
		return rbNilClassID
	case interface{ _ClassID() int }:
		return v._ClassID()
	}
	if rbIsProc(a) {
		return rbProcClassID
	}
	return -1
}

// rbKindOf is is_a?(target) by ID: target is among a's class's ancestors.
func rbKindOf(a any, target int) bool {
	id := rbClassID(a)
	return id >= 0 && slices.Contains(rbAncestry[id], target)
}

// rbWrapPanic converts Go runtime panics into Ruby exceptions so a
// catch-all rescue sees a StandardError.
func rbWrapPanic(r any) any {
	switch e := r.(type) {
	case ExceptionI:
		e._Exception().__seen = true // a later capture knows this is a re-raise (decision 106)
		return r
	case rbStop, rbThrow, rbBreak, rbIterBreak, *rbReturn:
		return r
	case rbIntOverflowed:
		return rbCaptureBacktrace(e.err()) // now, while the overflow's frames are on the stack: a rescue that does not match re-raises the RangeError, not this value
	case RxUnsupported:
		return NewRegexpError(Ref(String(e)))
	case RxTimeout:
		return NewRegexp_TimeoutError(Ref(String(e)))
	}
	if err, ok := r.(runtime.Error); ok {
		msg := err.Error()
		if strings.Contains(msg, "divide by zero") {
			return NewZeroDivisionError(Ref[String]("divided by 0"))
		}
		if strings.Contains(msg, "nil pointer") {
			return NewNoMethodError(Ref[String]("undefined method for nil"))
		}
		if strings.Contains(msg, "index out of range") {
			return NewIndexError(Ref[String](String(msg)))
		}
		if os.Getenv("RUGAROO_GO_STACK") != "" { // debugging: where in the Go program it failed (decision 324)
			msg += "\n" + string(debug.Stack())
		}
		return NewStandardError(Ref[String](String(msg)))
	}
	return NewStandardError(Ref[String](String(fmt.Sprint(r))))
}

// rbThrow unwinds from Kernel#throw to its Kernel#catch. Like rbStop it
// is not an exception: ensure runs, rescue passes it on.
type rbThrow struct{ tag, val any }

// rbCatches is the running goroutine's catch tags: its Fiber's, else its
// Thread's, else its Ractor's, else the main thread's (decision 104 maps
// goroutines to them). Only that goroutine touches the list, so there is no
// lock, and a fiber or thread cannot throw to another's catch, as in MRI.
// A throw with no match on the list raises UncaughtThrowError where it is.
func rbCatches() *[]any {
	id := rbGoID()
	if f, ok := rbFiberOf.Load(id); ok { // before rbThreadOf, which maps a fiber's goroutine to its thread
		return &f.(*Fiber).catches
	}
	if t, ok := rbThreadOf.Load(id); ok {
		return &t.(*Thread).catches
	}
	if r, ok := rbRactorOf.Load(id); ok {
		return &r.(*Ractor).catches
	}
	return &rbMainThread.catches
}

func rbCatch(tag any, blk func(any) any) (res any) {
	tags := rbCatches()
	*tags = append(*tags, tag)
	n := len(*tags)
	defer func() {
		*tags = (*tags)[:n-1] // catches nest: this one is last
		if r := recover(); r != nil {
			if t, ok := r.(rbThrow); ok && rbIdentical(t.tag, tag) {
				res = t.val
				return
			}
			panic(r)
		}
	}()
	return blk(tag)
}

func rbThrowTag(tag, val any) {
	if !slices.ContainsFunc(*rbCatches(), func(t any) bool { return rbIdentical(t, tag) }) {
		panic(NewUncaughtThrowError("uncaught throw "+rbInspect(tag), tag, val))
	}
	panic(rbThrow{tag, val})
}

// rbStop unwinds a closure-taking each when the loop over its rbSeq
// adapter stops early, like MRI's break: ensure runs, rescue passes it on.
type rbStop struct{}

// rbSeq adapts a closure-taking each to the iter.Seq its callers range
// over (decision 4). A loop body's own exception still unwinds through
// each; if each rescues it, Go aborts, since a range function may not
// recover one.
func rbSeq[E any](each func(func(E))) iter.Seq[E] {
	return func(yield func(E) bool) {
		defer rbStopped()
		stopped := false
		each(func(x E) {
			if stopped || !yield(x) {
				stopped = true
				panic(rbStop{})
			}
		})
	}
}

func rbSeq2[K, V any](each func(func(K, V))) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		defer rbStopped()
		stopped := false
		each(func(k K, v V) {
			if stopped || !yield(k, v) {
				stopped = true
				panic(rbStop{})
			}
		})
	}
}

func rbStopped() {
	if r := recover(); r != nil {
		if _, ok := r.(rbStop); !ok {
			panic(r)
		}
	}
}

func NewHash[K, V comparable]() *Hash[K, V] {
	return &Hash[K, V]{idx: rbKeyIndex[K]{plain: rbPlainKey[K]()}} // slots are made by the ninth key (decision 388): an empty Hash allocates no index
}

// `when Array` / `when Hash` in a type switch can't match a generic
// instantiation; every instantiation implements these instead.
type Array_Any interface{ _ToAny() *Array[any] }
type Hash_Any interface{ _ToAny() *Hash[any, any] }

// rbNum2Long is MRI's NUM2LONG for a Float argument: truncated, a RangeError past Integer's range (decision 181).
func rbNum2Long(f Float) Integer {
	v := float64(f)
	if math.IsNaN(v) || v >= 9.223372036854775807e18 || v < -9.223372036854775808e18 {
		s := strconv.FormatFloat(v, 'g', -1, 64)
		if math.IsInf(v, 0) {
			s = map[bool]string{true: "Inf", false: "-Inf"}[v > 0]
		}
		panic(NewRangeError(Ref(String("float " + s + " out of range of integer"))))
	}
	return Integer(v)
}

// rbFloatToI converts like MRI: a non-finite Float raises FloatDomainError.
func rbFloatToI(f float64) Integer {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		panic(NewFloatDomainError(Ref(rbFloatToS(f))))
	}
	if f < -(1<<63) || f >= 1<<63 {
		panic(NewRangeError(Ref("float " + rbFloatToS(f) + " out of range of integer")))
	}
	return Integer(f)
}

// rbIntOverflow raises where MRI would promote to a Bignum: Integer is a
// Go int (decision 35). Out of line so the checked operators still inline.
//
//go:noinline
func rbIntOverflow(a Integer, op string, b Integer) {
	panic(rbIntOverflowed{a, b, op}.err())
}

// rbIntOverflowed is panicked as a value, not built by a call: Go's inliner charges a call ~57 of 80, a panic 1; rbWrapPanic makes it the RangeError (decision 400).
type rbIntOverflowed struct {
	a, b Integer
	op   string
}

func (o rbIntOverflowed) err() *RangeError {
	return NewRangeError(Ref(String(fmt.Sprintf("%d %s %d overflows Integer (64-bit; no Bignum)", o.a, o.op, o.b))))
}

// rbIntMulOk is a*b and whether it fits: the signed high word of the
// 128-bit product must be the low word's sign extension.
func rbIntMulOk(a, b Integer) (Integer, bool) {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	if a < 0 {
		hi -= uint64(b)
	}
	if b < 0 {
		hi -= uint64(a)
	}
	return Integer(lo), int64(hi) == int64(lo)>>63
}

func rbFloatToS(f float64) String {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	// MRI's flo_to_s: exponent form when the decimal point sits more than
	// DBL_DIG (15) digits in and the shortest digits have no fraction. In
	// [1e15, 1e16) the shortest digits lack a fraction iff f is integral.
	abs := math.Abs(f)
	var s string
	if abs != 0 && (abs >= 1e16 || abs < 1e-4 || abs >= 1e15 && f == math.Trunc(f)) {
		s = strconv.FormatFloat(f, 'e', -1, 64)
		mant, exp, _ := strings.Cut(s, "e")
		if !strings.Contains(mant, ".") {
			mant += ".0"
		}
		return String(mant + "e" + exp)
	}
	s = strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsAny(s, ".") {
		s += ".0"
	}
	return String(s)
}

// rbCallerLoc is the "file:line" of the nearest user-code frame (a Ruby
// body outside the prelude), as MRI shows where a Thread or Ractor was made.
func rbCallerLoc() string { return rbCallerLocN(0) }

// rbCallerLocN skips n user-code frames first (warn's uplevel:); "" past the stack.
func rbCallerLocN(n int) string {
	var pcs [64]uintptr
	return rbUserLoc(pcs[:runtime.Callers(2, pcs[:])], n)
}

// rbCallerFrames is the top of a call stack as runtime.Callers records it: cheap to take, resolved to a file:line
// only when something prints it (Thread#inspect, Ractor#inspect; decision 403). The creating user frame sits a few
// frames up, so 16 is room enough.
type rbCallerFrames struct {
	pcs [16]uintptr
	n   int
}

// rbCallerPCs records the caller's stack.
func rbCallerPCs() rbCallerFrames {
	var c rbCallerFrames
	c.n = runtime.Callers(2, c.pcs[:])
	return c
}

// loc is the file:line of the first user-code frame after skipping n of them; "" past the stack or when nothing was recorded.
func (c *rbCallerFrames) loc(n int) string { return rbUserLoc(c.pcs[:c.n], n) }

// rbUserLoc is the file:line of the (n+1)th user-code frame among pcs; "" when there is none.
func rbUserLoc(pcs []uintptr, n int) string {
	if len(pcs) == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs)
	_, self, _, _ := runtime.Caller(0)
	mod := strings.TrimSuffix(self, "prelude/go/runtime.go")
	for {
		f, more := frames.Next()
		file := strings.TrimPrefix(f.File, mod)
		if strings.HasSuffix(file, ".rb") && !strings.HasPrefix(file, "prelude/") && !strings.Contains(f.Function, ".(") {
			if n == 0 {
				return file + ":" + strconv.Itoa(f.Line)
			}
			n--
		}
		if !more {
			return ""
		}
	}
}

// rbStringInspect is String#inspect for a UTF-8 string: controls are \u00XX, as MRI prints them for a UTF-8 string.
func rbStringInspect(s string) String { return rbStringInspectAs(s, true) }

// rbStringInspectAs inspects s; utf8 false gives the US-ASCII form of a control (\xXX), which Symbol#inspect uses.
func rbStringInspectAs(s string, utf8Controls bool) String {
	if rbInspectPlain(s) {
		return String(`"` + s + `"`)
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&b, `\x%02X`, s[i])
			i++
			continue
		}
		i += n
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		case '\a':
			b.WriteString(`\a`)
		case '\b':
			b.WriteString(`\b`)
		case 0x1b:
			b.WriteString(`\e`)
		case '#':
			if i < len(s) && (s[i] == '{' || s[i] == '$' || s[i] == '@') {
				b.WriteByte('\\') // #{ #$ #@ would interpolate if the result were read back
			}
			b.WriteByte('#')
		default:
			switch {
			case (r < 0x20 || r == 0x7f) && utf8Controls:
				// ponytail: strings carry no encoding, so every String gets the
				// UTF-8 form; MRI prints a US-ASCII string's controls (Integer#chr) as \x00.
				fmt.Fprintf(&b, `\u%04X`, r)
			case r < 0x20 || r == 0x7f:
				fmt.Fprintf(&b, `\x%02X`, r)
			case unicode.IsGraphic(r) || unicode.In(r, unicode.Cf, unicode.Co):
				// MRI's "printable" for UTF-8: graphic, format or private use.
				b.WriteRune(r)
			case r > 0xFFFF:
				fmt.Fprintf(&b, `\u{%X}`, r)
			default:
				fmt.Fprintf(&b, `\u%04X`, r)
			}
		}
	}
	b.WriteByte('"')
	return String(b.String())
}

// rbInspectPlain is whether inspect prints s between quotes unchanged: printable ASCII, no quote, backslash or interpolation opener.
func rbInspectPlain(s string) bool {
	for i := range len(s) {
		switch c := s[i]; {
		case c < 0x20 || c >= 0x7f || c == '"' || c == '\\':
			return false
		case c == '#' && i+1 < len(s) && (s[i+1] == '{' || s[i+1] == '$' || s[i+1] == '@'):
			return false
		}
	}
	return true
}

// rbSleep is Kernel#sleep: MRI's errors, and the whole seconds slept.
func rbSleep(secs float64) Integer {
	if secs < 0 {
		panic(NewArgumentError(Ref(String("time interval must not be negative"))))
	}
	start := time.Now()
	d := time.Duration(math.MaxInt64) // past ~292 years a float->int64 conversion is implementation-defined and can wrap negative
	if secs < float64(math.MaxInt64)/float64(time.Second) {
		d = time.Duration(secs * float64(time.Second))
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-rbInterruptC():
		rbTakeInterrupt() // Interrupt on the main goroutine; another sleeps on
		<-timer.C
	}
	return Integer(math.Round(time.Since(start).Seconds()))
}

func rbSleepForever() Integer {
	<-rbInterruptC()
	rbTakeInterrupt()
	select {}
}

// rbFreezable is a value carrying its own frozen flag (Array, Hash: decision 96).
// There is no global registry: freezing one object costs nothing elsewhere.
type rbFreezable interface {
	rbFrozen() bool
	rbSetFrozen()
}

func rbIsFrozen(p any) bool {
	if f, ok := p.(rbFreezable); ok {
		return f.rbFrozen()
	}
	fl := rbObjFrozenFlag(p) // a user object Kernel#freeze froze (decision 221)
	return fl != nil && *fl
}

// rbFrozenCheck raises MRI's FrozenError before a mutation of a frozen p.
// Array and Hash mutators call their typed rbCheckFrozen instead, which inlines.
func rbFrozenCheck(p any) {
	if rbIsFrozen(p) {
		rbFrozenErr(p)
	}
}

func rbFrozenErr(p any) {
	panic(NewFrozenError(Ref("can't modify frozen " + String(rbClassName(p)) + ": " + rbInspect(p))))
}

// rbIvarNames is Kernel#instance_variables: the ivars inspect would list (one not assigned yet reads as nil and is left out).
func rbIvarNames(a any) *Array[Symbol] {
	out := &Array[Symbol]{}
	if o, ok := a.(interface{ _Ivars(name string) []rbIvar }); ok {
		for _, iv := range o._Ivars("") {
			if iv.opt || iv.val != nil && !iv.isNil {
				out.s = append(out.s, Symbol(iv.name))
			}
		}
	}
	return out
}

// rbIvarName reads an ivar name argument (Symbol or String), checking MRI's "@" form.
func rbIvarName(name any) string {
	var s string
	switch n := rbUnbox(name).(type) {
	case Symbol, String:
		s = fmt.Sprint(n)
	default:
		panic(NewTypeError(Ref(rbInspect(name) + " is not a symbol nor a string")))
	}
	if !strings.HasPrefix(s, "@") || strings.HasPrefix(s, "@@") || len(s) < 2 {
		panic(NewNameError(Ref(String("'" + s + "' is not allowed as an instance variable name"))))
	}
	return s
}

// rbIvarGet is Kernel#instance_variable_get: nil for an ivar the class never has.
func rbIvarGet(a, name any) any {
	n := rbIvarName(name)
	if o, ok := a.(interface{ _Ivars(name string) []rbIvar }); ok {
		for _, iv := range o._Ivars(n) {
			if iv.name == n && !iv.isNil {
				return iv.val
			}
		}
	}
	return nil
}

// rbIvarDefined is Kernel#instance_variable_defined?.
func rbIvarDefined(a, name any) bool {
	n := rbIvarName(name)
	return slices.Contains(rbIvarNames(a).s, Symbol(n))
}

// rbIvarSet is Kernel#instance_variable_set over the generated _IvarSet; an ivar the class lacks cannot be added.
func rbIvarSet(a, name, v any) any {
	n := rbIvarName(name)
	rbFrozenCheck(a)
	o, ok := a.(interface{ _IvarSet(string, any) bool })
	if !ok || !o._IvarSet(n, rbUnbox(v)) {
		panic(NewNameError(Ref(String("rugaroo: " + rbClassName(a) + " has no instance variable " + n + " (the closed world fixes each class's ivars)"))))
	}
	return v
}

// rbIvarRemove is Kernel#remove_instance_variable over the generated _IvarDel (decision 153).
func rbIvarRemove(a, name any) any {
	n := rbIvarName(name)
	rbFrozenCheck(a)
	o, ok := a.(interface{ _IvarDel(string) })
	if !ok || !rbIvarDefined(a, name) {
		e := NewNameError(Ref(String("instance variable " + n + " not defined")))
		e.__SetCall(Symbol(n), a)
		panic(e)
	}
	v := rbIvarGet(a, name)
	o._IvarDel(n)
	return v
}

func rbIvarClear[T any](p *T) {
	var zero T
	*p = zero
}

// rbIvarAssign converts an untyped value to an ivar's type, as instance_variable_set writes it.
func rbIvarAssign[T any](dst *T, v any, name string) {
	x, ok := rbConv[T](v)
	if !ok {
		panic(NewTypeError(Ref(String("rugaroo: " + rbClassName(v) + " cannot be stored in " + name))))
	}
	*dst = x
}

// rbIvarAssignPtr is rbIvarAssignOpt for a C? that is its own Go type: a leaf's *C (decision 360), another class's CI (378).
func rbIvarAssignPtr[T any](dst *T, v any, name string) {
	if v == nil {
		var z T
		*dst = z
		return
	}
	rbIvarAssign(dst, v, name)
}

func rbIvarAssignVal[T any](dst *rbOpt[T], v any, name string) {
	if v == nil {
		*dst = rbOpt[T]{}
		return
	}
	var x T
	rbIvarAssign(&x, v, name)
	*dst = rbOpt[T]{x, true}
}

func rbIvarAssignOpt[T any](dst **T, v any, name string) {
	if v == nil {
		*dst = nil
		return
	}
	var x T
	rbIvarAssign(&x, v, name)
	*dst = &x
}

// rbDup is Object#dup: a struct object's generated _Dup copy; anything else (Integer, Symbol, nil, true) is itself.
func rbDup[T any](x T) T {
	if d, ok := any(x).(interface{ _Dup() any }); ok {
		return d._Dup().(T)
	}
	return x
}

// rbIntShlWrap is a gem's Integer#<<: mod 2**64 past 64 bits, as Go's shift, where MRI would make a Bignum (decision 324).
func rbIntShlWrap(x, n Integer) Integer {
	if n < 0 {
		return rbIntShl(x, n)
	}
	return x << n
}

// rbHasMissing reports whether v's class answers unknown names through method_missing.
func rbHasMissing(v any) bool {
	_, ok := v.(interface{ DynMethodMissing(...any) any })
	return ok
}

// rbIfBlk is an adapter over a forwarded block when there is one, else no block.
func rbIfBlk[F any](ok bool, fn F) F {
	if ok {
		return fn
	}
	var none F
	return none
}

// rbArgAt is a block parameter bound from a variadic call: the i-th value, or the zero value (nil) past the end.
func rbArgAt[T any](xs []T, i int) T {
	if i < len(xs) {
		return xs[i]
	}
	var zero T
	return zero
}

// rbMust is a gem's T? where T is wanted (decision 172): nil raises TypeError.
func rbMust[T any](p *T, want string) T {
	if p == nil {
		panic(NewTypeError(Ref(String("no implicit conversion of nil into " + want))))
	}
	return *p
}

// rbMustO is rbMust for an rbOpt (decision 396).
func rbMustO[T any](o rbOpt[T], want string) T {
	if !o.ok {
		panic(NewTypeError(Ref(String("no implicit conversion of nil into " + want))))
	}
	return o.v
}

// rbNilArg is a T? where T is wanted under --nil-check=runtime (decision 393): nil raises the error MRI raises there.
func rbNilArg[T any](p *T, msg string, arg bool) T {
	if p == nil {
		panic(rbNilErr(msg, arg))
	}
	return *p
}

// rbNilArgO is rbNilArg for an rbOpt (decision 396).
func rbNilArgO[T any](o rbOpt[T], msg string, arg bool) T {
	if !o.ok {
		panic(rbNilErr(msg, arg))
	}
	return o.v
}

// rbNilArgV is rbNilArg for a C? that holds nil itself, a leaf's *C or an interface (decisions 360, 378).
func rbNilArgV[T comparable](v T, msg string, arg bool) T {
	var z T
	if v == z {
		panic(rbNilErr(msg, arg))
	}
	return v
}

func rbNilErr(msg string, arg bool) any {
	if arg {
		return NewArgumentError(Ref(String(msg)))
	}
	return NewTypeError(Ref(String(msg)))
}

// rbDerefOrNil is a gem's T? where object type T is wanted: nil stays a nil interface (decision 257).
func rbDerefOrNil[T any](p *T) (z T) {
	if p == nil {
		return z
	}
	return *p
}

// rbBlockArg is a block passed to a method through a dynamic call, last among its arguments (decision 243); a bound
// one (a forwarded `T.proc.bind` &block) takes the self it runs on as its first value (decision 281).
type rbBlockArg struct {
	fn    func(...any) any
	bound bool
}

// rbSelfArg leads a dynamic block's values when instance_eval rebinds its self (decision 324).
type rbSelfArg struct{ v any }

// rbEvalBlock is instance_eval/instance_exec of a gem's untyped block: self, then the values it yields.
func rbEvalBlock(blk func(...any) any, self any, vals ...any) any {
	if blk == nil {
		panic(NewArgumentError(Ref(String("no block given"))))
	}
	return blk(append([]any{rbSelfArg{self}}, vals...)...)
}

// rbSelfOr is a dynamic block's self: the one instance_eval gave, else its lexical one; and its values after it.
func rbSelfOr(self any, xs []any) (any, []any) {
	if len(xs) > 0 {
		if s, ok := xs[0].(rbSelfArg); ok {
			return s.v, xs[1:]
		}
	}
	return self, xs
}

// rbBlockArgOf is a method's own optional `&block` passed on to a dynamic call: an rbBlockArg, or one with no fn when
// the caller gave none.
func rbBlockArgOf(p **func(...any) any) rbBlockArg {
	if p == nil {
		return rbBlockArg{}
	}
	return rbBlockArg{fn: *(*p)}
}

// rbKwArgs is a dynamic call's keywords as arguments: none for an empty Hash (`send(m, *args, **{})`).
func rbKwArgs(h any) []any {
	if x, ok := h.(Hash_Any); ok && int(x._ToAny().live) == 0 {
		return nil
	}
	return []any{h}
}

// rbBlockArgAny is an untyped value passed as `&blk` to a dynamic call: no block for nil, else the Proc it holds.
// ponytail: a Proc that binds self (instance_eval'd) is not callable this way; it raises as a non-Proc would.
func rbBlockArgAny(p any) rbBlockArg {
	if p == nil {
		return rbBlockArg{}
	}
	if !rbIsProc(p) {
		panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(p) + " (expected Proc)"))))
	}
	return rbBlockArg{fn: func(xs ...any) any {
		v, ok := rbDynProcCall(p, xs)
		if !ok {
			panic(NewTypeError(Ref(String("rugaroo: this Proc cannot be passed on as a block dynamically"))))
		}
		return v
	}}
}

// rbBoundArgs is what a bound block's adapter passes on: self first only to a bound rbBlockArg (decision 281).
func rbBoundArgs(b *rbBlockArg, self any, xs ...any) []any {
	if b.bound {
		return append([]any{self}, xs...)
	}
	return xs
}

// rbPopBlock pops a dynamic call's block off its arguments: nil when it passed none.
func rbPopBlock(args []any) (*rbBlockArg, []any) {
	if n := len(args); n > 0 {
		if b, ok := args[n-1].(rbBlockArg); ok {
			if b.fn == nil { // a forwarded &block the caller did not give
				return nil, args[:n-1]
			}
			return &b, args[:n-1]
		}
	}
	return nil, args
}

// rbBreak unwinds a closure block's `break` to the call statement that passed the block (decision 241); rescue passes
// it on, ensure runs.
type rbBreak struct{ id string }

// rbBreakRecover ends the call whose block broke with id, and lets any other panic through.
func rbBreakRecover(id string) {
	if r := recover(); r != nil {
		if b, ok := r.(rbBreak); !ok || b.id != id {
			panic(r)
		}
	}
}

// rbReturn unwinds a gem block's `return` to the method call it returns from (decision 247): tok is that call's own,
// so a recursive call of the same method does not take it. Rescue passes it on, ensure runs.
type rbReturn struct {
	_   [0]func() // not comparable: Go generates no equality function for it in every program
	tok *byte
	v   any
}

// rbReturnRecover ends the call whose token tok is, with the returned value (set through set, nil for a void
// method), and lets any other panic through.
func rbReturnRecover(tok *byte, set func(any)) {
	if r := recover(); r != nil {
		x, ok := r.(*rbReturn)
		if !ok || x.tok != tok {
			panic(r)
		}
		if set != nil {
			set(x.v)
		}
	}
}

// rbIterBreak unwinds an iterator method from a yield inside a block it passed on, when the consumer broke out
// (decision 226): Go's yield returned false, and a return there would leave only the block.
type rbIterBreak struct{}

// rbIterBreakRecover ends such an iteration quietly, and lets any other panic through.
func rbIterBreakRecover() {
	if r := recover(); r != nil {
		if _, ok := r.(rbIterBreak); !ok {
			panic(r)
		}
	}
}

// rbLibDir is RbConfig's rubylibdir: MRI's library is compiled in, so a require under it names a library (decision 296).
const rbLibDir = "/rugaroo/lib/ruby/4.0.0"

// rbBlockOf is a T? Proc passed as `&p`: its func, or no block (a nil func) for nil.
func rbBlockOf[F any](p **F) F {
	if p == nil || *p == nil {
		var none F
		return none
	}
	return **p
}

// rbAutoSplat is Ruby's auto-splat for a block of two or more parameters called with untyped values: one Array (or
// tuple) argument is its elements, so `|k, v|` given a Hash pair binds both.
func rbAutoSplat(xs []any) []any {
	if len(xs) == 1 {
		if a, ok := rbUnbox(xs[0]).(Array_Any); ok {
			return a._ToAny().s
		}
	}
	return xs
}

// rbPackYield is the one value an Enumerable-style iterator hands on for a `yield a, b` (an Array) or `yield a` (a).
func rbPackYield(xs []any) any {
	switch len(xs) {
	case 0:
		return nil
	case 1:
		return xs[0]
	}
	return &Array[any]{s: slices.Clone(xs)}
}

// rbSlotShared reports whether r is a container whose `r[i]` is the very object it holds (an Array or a Hash), so a
// String edit on what it returned is seen through r, as in MRI (decision 334).
func rbSlotShared(r any) bool {
	switch rbUnbox(r).(type) {
	case Array_Any, Hash_Any:
		return true
	}
	return false
}

// rbMtTraceDepth is how many assert_raises blocks are running, across threads (minitest.go).
var rbMtTraceDepth atomic.Int64
