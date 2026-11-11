//go:build rugaroo_prelude

package prelude

import (
	"math"
	"runtime"
	"slices"
	"strconv"
	"sync/atomic"
	"unsafe"
)

// Fiddle's C half (#32, decision 326); addresses are Integers, Pointer and friends are Ruby (fiddle.rb). The calls, dlopen and
// callbacks are a backend: fiddle_purego.go by default, fiddle_cgo.go under --cgo (decision 426).

// rbFiddleAt converts through memory: vet's unsafeptr flags a direct uintptr conversion, but this memory is never Go's.
// Address 0 is MRI's DLError, never a Go nil dereference.
func rbFiddleAt(addr Integer) unsafe.Pointer {
	if addr == 0 {
		panic(rbFiddleDLError("NULL pointer dereference"))
	}
	p := uintptr(addr)
	return *(*unsafe.Pointer)(unsafe.Pointer(&p)) //nolint:gosec // C memory, never Go's
}

// rbFiddleDLError is Fiddle::DLError with msg.
func rbFiddleDLError(msg string) any {
	return NewFiddle_DLError(Ref(String(msg)))
}

// Each resolves on its own: a platform missing one fails at that function's first call, not at start.
var (
	rbFiddleMallocFn  = rbFiddleLibc("malloc")
	rbFiddleFreeFn    = rbFiddleLibc("free")
	rbFiddleReallocFn = rbFiddleLibc("realloc")
)

// rbFiddleLibcFn is fn, or DLError naming sym when libc lacks it (calling address 0 would crash).
func rbFiddleLibcFn(fn uintptr, sym string) uintptr {
	if fn == 0 {
		panic(rbFiddleDLError("unknown symbol \"" + sym + "\""))
	}
	return fn
}

// rbFiddleMalloc is libc malloc, so C code may free what Ruby allocated; NoMemoryError when it fails.
func rbFiddleMalloc(n Integer) Integer {
	rbCheckAlloc(int(n), 1)
	addr := rbFiddleCall1(rbFiddleLibcFn(rbFiddleMallocFn, "malloc"), uintptr(n))
	if addr == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(addr)
}

// rbFiddleRealloc is libc realloc.
func rbFiddleRealloc(addr, n Integer) Integer {
	rbCheckAlloc(int(n), 1)
	in := [8]uintptr{uintptr(addr), uintptr(n)}
	var fl [8]uint64
	r, _, _ := rbFiddleCall(&rbFiddleLibcSlot, rbFiddleLibcFn(rbFiddleReallocFn, "realloc"), &in, &fl, false)
	if r == 0 && n > 0 {
		panic(NewNoMemoryError(Ref(String("failed to allocate memory"))))
	}
	return Integer(r)
}

// rbFiddleFree is libc free.
func rbFiddleFree(addr Integer) {
	rbFiddleCall1(rbFiddleLibcFn(rbFiddleFreeFn, "free"), uintptr(addr))
}

// rbFiddleFreeAddr is the address of libc's free (Fiddle::RUBY_FREE's role: a free function C memory can carry).
func rbFiddleFreeAddr() Integer { return Integer(rbFiddleFreeFn) }

// rbFiddleBytes is n bytes at addr.
func rbFiddleBytes(addr, n Integer) []byte {
	return unsafe.Slice((*byte)(rbFiddleAt(addr)), int(n)) //nolint:gosec // C memory the program owns
}

// rbFiddleRead is n bytes at addr as a binary String.
func rbFiddleRead(addr, n Integer) String {
	if n <= 0 {
		return ""
	}
	return String(rbFiddleBytes(addr, n))
}

// rbFiddleCString is the NUL-terminated string at addr.
func rbFiddleCString(addr Integer) String {
	n := 0
	for *(*byte)(rbFiddleAt(addr + Integer(n))) != 0 {
		n++
	}
	return String(rbFiddleBytes(addr, Integer(n)))
}

// rbFiddleWrite copies s to addr.
func rbFiddleWrite(addr Integer, s String) {
	if len(s) > 0 {
		copy(rbFiddleBytes(addr, Integer(len(s))), s)
	}
}

// rbFiddleChar is C's char at addr (Pointer#[]): signed, except on Linux arm64, where char is unsigned.
func rbFiddleChar(addr Integer) Integer {
	b := *(*byte)(rbFiddleAt(addr))
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		return Integer(b)
	}
	return Integer(int8(b)) //nolint:gosec // C's signed char
}

// rbFiddleWord is the pointer-sized value at addr (Pointer#ptr).
func rbFiddleWord(addr Integer) Integer {
	return Integer(*(*uintptr)(rbFiddleAt(addr)))
}

// rbFiddleLibcSlot is the argument block libc's malloc, free and realloc share.
var rbFiddleLibcSlot atomic.Pointer[rbFiddleArgs]

// rbFiddleCall1 is a one-argument libc call (malloc, free).
func rbFiddleCall1(fn, a uintptr) uintptr {
	in := [8]uintptr{a}
	var fl [8]uint64
	r, _, _ := rbFiddleCall(&rbFiddleLibcSlot, fn, &in, &fl, false)
	return r
}

// rbFiddleCFunc is a bound C function: its address and TYPE_* codes; every register-passed signature goes through
// rbFiddleCall, the rest (a variadic one, or one with stack arguments) through the backend's rbFiddleCallSpread.
type rbFiddleCFunc struct {
	slot     atomic.Pointer[rbFiddleArgs]
	addr     uintptr
	args     []Integer // the fixed ones, without (void) and TYPE_VARIADIC
	ret      Integer
	float    bool
	variadic bool // TYPE_VARIADIC came last: the call adds type and value pairs
	spread   bool // more than 8 integer or 8 floating arguments
}

// rbFiddleIsFloat reports a TYPE_FLOAT or TYPE_DOUBLE.
func rbFiddleIsFloat(t Integer) bool { return t == 7 || t == 8 }

// rbFiddleCheckType rejects what purego cannot pass.
func rbFiddleCheckType(t Integer) {
	switch {
	case t == 9:
		panic(NewNotImplementedError(Ref(String("rugaroo: Fiddle::TYPE_VARIADIC (purego cannot make a C variadic call)"))))
	case t < -6 || t > 11:
		panic(rbFiddleDLError("unknown type: " + strconv.Itoa(int(t))))
	}
}

// rbFiddleCheckArgs rejects argument types the registers cannot pass, past 8 int or 8 float (where the stack layout
// differs per platform), unless the backend lays out stack arguments and spread allows them (a call, not a callback).
// It reports whether some do not fit.
func rbFiddleCheckArgs(args []Integer, spread bool) bool {
	ni, nf := 0, 0
	for _, t := range args {
		rbFiddleCheckType(t)
		switch {
		case t == 0:
		case rbFiddleIsFloat(t):
			nf++
		default:
			ni++
		}
	}
	if (ni > 8 || nf > 8) && !(spread && rbFiddleSpread) {
		panic(NewNotImplementedError(Ref(String("rugaroo: Fiddle calls take at most 8 integer and 8 floating-point arguments (more need --cgo; decision 426)"))))
	}
	return ni > 8 || nf > 8
}

// rbFiddlePrepare binds addr (Function.new).
func rbFiddlePrepare(addr Integer, args *Array[Integer], ret Integer) any {
	if addr == 0 {
		panic(rbFiddleDLError("NULL function pointer"))
	}
	fn := &rbFiddleCFunc{addr: uintptr(addr), ret: ret, float: rbFiddleIsFloat(ret)}
	types := args.s
	if k := slices.Index(types, 9); k >= 0 && rbFiddleSpread { // MRI checks it is last in Function#initialize (fiddle.rb)
		types, fn.variadic = types[:k], true
	}
	fn.spread = rbFiddleCheckArgs(types, true)
	for _, t := range types {
		if t != 0 { // (void)
			fn.args = append(fn.args, t)
		}
	}
	rbFiddleCheckType(ret)
	return fn
}

// rbFiddleWord64 is one Ruby value as the 64-bit register C reads type t from.
func rbFiddleWord64(t Integer, v any) (uintptr, float64) {
	var i int64
	var f float64
	switch x := v.(type) {
	case nil:
	case Integer:
		i, f = int64(x), float64(x)
	case Float:
		i, f = int64(x), float64(x)
	case Boolean:
		if x {
			i = 1
		}
	default:
		panic(NewTypeError(Ref(String("rugaroo: Fiddle cannot pass " + rbClassName(v)))))
	}
	if t == 7 { // a float sits in the low half of its register
		return 0, math.Float64frombits(uint64(math.Float32bits(float32(f))))
	}
	return uintptr(i), f //nolint:gosec // C's own wraparound for a negative value
}

// rbFiddleCString is s NUL-terminated in memory C may read during the call; kept (not freed) when the result may point into it.
func rbFiddleCStr(s String, keep bool) (uintptr, []byte) {
	if keep { // ponytail: a void* result may point into the copy (strchr), so it is C memory that leaks; tie it to the Pointer if needed.
		mem := rbFiddleMalloc(Integer(len(s) + 1))
		b := rbFiddleBytes(mem, Integer(len(s)+1))
		b[copy(b, s)] = 0
		return uintptr(mem), nil
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	return uintptr(unsafe.Pointer(&b[0])), b //nolint:gosec // Go's heap does not move; the caller keeps b alive across the call
}

// rbFiddleInvoke calls a prepared function with Fiddle's argument conversions (Function#call; objects are addresses already)
// and stores errno as Fiddle.last_error.
func rbFiddleInvoke(prepared any, args *Array[any]) any {
	fn, ok := prepared.(*rbFiddleCFunc)
	if !ok {
		panic(rbFiddleDLError("rugaroo: not a prepared Fiddle function"))
	}
	return rbFiddleInvokeArgs(fn, args.s)
}

// rbFiddleFast is Function#call's result without call's *args Array, or f itself when Function#call must run: f is
// a subclass (which may override call), or an argument is an object other than a plain Pointer (decision 329).
func rbFiddleFast(f Fiddle_FunctionI, a ...any) any {
	x, exact := f.(*Fiddle_Function)
	if !exact {
		return f
	}
	fn, ok := x.fn.(*rbFiddleCFunc)
	if !ok || len(a) != len(fn.args) {
		return f
	}
	for k, v := range a {
		switch v.(type) {
		case nil, Integer, Float, String, Boolean:
			continue
		}
		p, ok := v.(*Fiddle_Pointer) // always in a program that calls C: Fiddle.__ret makes them
		if t := fn.args[k]; !ok || rbFiddleIsFloat(t) || t == 11 {
			return f
		}
		a[k] = p.addr
	}
	return Fiddle_Meta___Ret(Fiddle_class, x.ret, rbFiddleInvokeArgs(fn, a))
}

// rbFiddleTyped is nil unless f is still the plain Function the compiler read, so an extern's twin never bypasses an override or a redefinition (decision 442).
func rbFiddleTyped(f Fiddle_FunctionI, ret Integer, args ...Integer) *rbFiddleCFunc {
	x, ok := f.(*Fiddle_Function)
	if !ok || x.ret != ret {
		return nil
	}
	fn, ok := x.fn.(*rbFiddleCFunc)
	if !ok || fn.variadic || fn.spread || !slices.Equal(fn.args, args) {
		return nil
	}
	return fn
}

// rbFiddleCallTyped is rbFiddleInvokeArgs's call for arguments a twin placed itself.
func rbFiddleCallTyped(fn *rbFiddleCFunc, in *[8]uintptr, fl *[8]uint64) (uintptr, float64) {
	r, f, errno := rbFiddleCall(&fn.slot, fn.addr, in, fl, fn.float)
	rbFiddleSetErrno(errno)
	return r, math.Float64frombits(f)
}

// rbFiddleIntArg is rbFiddleWord64's word for an Integer.
func rbFiddleIntArg(a Integer) uintptr {
	return uintptr(a) //nolint:gosec // C's own wraparound for a negative value
}

// rbFiddleFloatArg is rbFiddleWord64's register bits for a Float as C type t.
func rbFiddleFloatArg(t Integer, a Float) uint64 {
	if t == 7 { // a float sits in the low half of its register
		return uint64(math.Float32bits(float32(a)))
	}
	return math.Float64bits(float64(a))
}

// rbFiddleIntValue is rbFiddleValue for an integer or pointer type, unboxed.
func rbFiddleIntValue(t Integer, r uintptr) Integer {
	switch t {
	case 2:
		return Integer(int8(r)) //nolint:gosec // by width: the ABI leaves upper bits unspecified
	case -2:
		return Integer(uint8(r)) //nolint:gosec // as above
	case 3:
		return Integer(int16(r)) //nolint:gosec // as above
	case -3:
		return Integer(uint16(r)) //nolint:gosec // as above
	case 4:
		return Integer(int32(r)) //nolint:gosec // as above
	case -4:
		return Integer(uint32(r)) //nolint:gosec // as above
	}
	return Integer(r) //nolint:gosec // ponytail: an unsigned long above 2**63 wraps (Integer is int64)
}

// rbFiddleFloatValue is rbFiddleValue for a float or double, unboxed.
func rbFiddleFloatValue(t Integer, f float64) Float {
	if t == 7 {
		return Float(math.Float32frombits(uint32(math.Float64bits(f)))) //nolint:gosec // the float's own low half
	}
	return Float(f)
}

// rbFiddleBoolValue is rbFiddleValue for a bool, unboxed.
func rbFiddleBoolValue(r uintptr) Boolean {
	return Boolean(uint8(r) != 0) //nolint:gosec // C's _Bool is one byte
}

// rbFiddleInvokeArgs converts args as Fiddle does, calls fn, and sets Fiddle.last_error.
func rbFiddleInvokeArgs(fn *rbFiddleCFunc, args []any) any {
	if fn.variadic || fn.spread {
		return rbFiddleInvokeSpread(fn, args)
	}
	if len(args) != len(fn.args) {
		panic(NewArgumentError(Ref(String("wrong number of arguments (given " + strconv.Itoa(len(args)) + ", expected " + strconv.Itoa(len(fn.args)) + ")"))))
	}
	var in [8]uintptr
	var fl [8]uint64
	var keep [8][]byte // String copies in Go memory, alive until the call returns
	ni, nf := 0, 0
	for k, a := range args {
		t := fn.args[k]
		switch {
		case rbFiddleIsFloat(t):
			_, f := rbFiddleWord64(t, a)
			fl[nf] = math.Float64bits(f)
			nf++
			continue
		case t == 11: // TYPE_BOOL: Ruby truthiness
			b, isBool := a.(Boolean)
			if a != nil && (!isBool || bool(b)) {
				in[ni] = 1
			}
		default:
			if s, isStr := a.(String); isStr && (t == 1 || t == 10) {
				in[ni], keep[ni] = rbFiddleCStr(s, fn.ret == 1)
			} else {
				in[ni], _ = rbFiddleWord64(t, a)
			}
		}
		ni++
	}
	r, f, errno := rbFiddleCall(&fn.slot, fn.addr, &in, &fl, fn.float)
	runtime.KeepAlive(&keep)
	rbFiddleSetErrno(errno)
	return rbFiddleValue(fn.ret, r, math.Float64frombits(f))
}

// rbFiddleInvokeSpread is rbFiddleInvokeArgs for a variadic function (args end in type and value pairs, the types
// already Integers: Function#call) or one with stack arguments; the backend places each argument (decision 426).
func rbFiddleInvokeSpread(fn *rbFiddleCFunc, args []any) any {
	types := slices.Clone(fn.args)
	vals := args
	if fn.variadic {
		if len(args) < len(fn.args) {
			panic(NewArgumentError(Ref(String("wrong number of arguments (given " + strconv.Itoa(len(args)) + ", expected " + strconv.Itoa(len(fn.args)) + "+)"))))
		}
		vals = slices.Clone(args[:len(fn.args)])
		for k := len(fn.args); k+1 < len(args); k += 2 {
			t, ok := args[k].(Integer)
			if !ok || t == 0 || t == 9 {
				panic(NewArgumentError(Ref("unknown type: " + rbInspect(args[k]))))
			}
			rbFiddleCheckType(t)
			if t == 7 || t == 11 || t == 2 || t == -2 || t == 3 || t == -3 { // libffi's ffi_prep_cif_var refuses what C would promote
				panic(NewRuntimeError(Ref(String("error creating CIF 3"))))
			}
			types, vals = append(types, t), append(vals, args[k+1])
		}
	} else if len(args) != len(fn.args) {
		panic(NewArgumentError(Ref(String("wrong number of arguments (given " + strconv.Itoa(len(args)) + ", expected " + strconv.Itoa(len(fn.args)) + ")"))))
	}
	words := make([]uintptr, len(vals))
	keep := make([][]byte, len(vals)) // String copies in Go memory, alive until the call returns
	for k, a := range vals {
		t := types[k]
		switch {
		case rbFiddleIsFloat(t):
			_, f := rbFiddleWord64(t, a)
			words[k] = uintptr(math.Float64bits(f))
		case t == 11:
			b, isBool := a.(Boolean)
			if a != nil && (!isBool || bool(b)) {
				words[k] = 1
			}
		default:
			if s, isStr := a.(String); isStr && (t == 1 || t == 10) {
				words[k], keep[k] = rbFiddleCStr(s, fn.ret == 1)
			} else {
				words[k], _ = rbFiddleWord64(t, a)
			}
		}
	}
	r, f, errno := rbFiddleCallSpread(&fn.slot, fn.addr, types, words, len(fn.args), fn.variadic, fn.float)
	runtime.KeepAlive(keep)
	rbFiddleSetErrno(errno)
	return rbFiddleValue(fn.ret, r, math.Float64frombits(f))
}

// rbFiddleSetErrno is Fiddle.last_error = errno, a thread-local. The Thread's fiddleErrno (errno+1; 0: unknown, which
// Thread#[]= resets it to) says what the last call stored, so an unchanged errno skips the store (decision 329).
func rbFiddleSetErrno(errno Integer) {
	t := rbCurrentThread()
	if t.fiddleErrno.Load() == int64(errno)+1 {
		return
	}
	t.locals.Store("__FIDDLE_LAST_ERROR__", errno)
	t.fiddleErrno.Store(int64(errno) + 1)
}

// rbFiddleValue reads type t from a register; narrow types are extended here, as the ABI leaves upper bits unspecified.
func rbFiddleValue(t Integer, r uintptr, f float64) any {
	switch t {
	case 0:
		return nil
	case 7, 8:
		return rbFiddleFloatValue(t, f)
	case 11:
		return rbFiddleBoolValue(r)
	}
	return rbFiddleIntValue(t, r)
}

// rbFiddleGetInt reads an integer member of C type t at addr.
func rbFiddleGetInt(addr, t Integer) Integer {
	p := rbFiddleAt(addr)
	switch t {
	case 2:
		return Integer(*(*int8)(p))
	case -2, 11:
		return Integer(*(*uint8)(p))
	case 3:
		return Integer(*(*int16)(p))
	case -3:
		return Integer(*(*uint16)(p))
	case 4:
		return Integer(*(*int32)(p))
	case -4:
		return Integer(*(*uint32)(p))
	}
	return Integer(*(*int64)(p))
}

// rbFiddleSetInt writes an integer member of C type t at addr, wrapping as C does.
func rbFiddleSetInt(addr, t, v Integer) {
	p := rbFiddleAt(addr)
	switch t {
	case 2, -2, 11:
		*(*uint8)(p) = uint8(v) //nolint:gosec // C's truncation
	case 3, -3:
		*(*uint16)(p) = uint16(v) //nolint:gosec // C's truncation
	case 4, -4:
		*(*uint32)(p) = uint32(v) //nolint:gosec // C's truncation
	default:
		*(*int64)(p) = int64(v)
	}
}

// rbFiddleGetFloat reads a float (t 7) or double member at addr.
func rbFiddleGetFloat(addr, t Integer) Float {
	if t == 7 {
		return Float(*(*float32)(rbFiddleAt(addr)))
	}
	return Float(*(*float64)(rbFiddleAt(addr)))
}

// rbFiddleSetFloat writes a float (t 7) or double member at addr.
func rbFiddleSetFloat(addr, t Integer, v Float) {
	if t == 7 {
		*(*float32)(rbFiddleAt(addr)) = float32(v)
		return
	}
	*(*float64)(rbFiddleAt(addr)) = float64(v)
}
