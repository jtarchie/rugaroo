//go:build rugaroo_prelude

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// prism's backend (decision 427): the gem's Ruby loads what pm_serialize_* in prism.wasm writes, as its FFI backend does.

// rbPrismFn indexes rbPrismInstance.fns, the exports a call names.
const (
	rbPrismParse = iota
	rbPrismLex
	rbPrismParseComments
	rbPrismParseLex
	rbPrismSuccess
	rbPrismQueryLocal
	rbPrismQueryConstant
	rbPrismQueryMethodName
)

var rbPrismExports = [...]string{"pm_serialize_parse", "pm_serialize_lex", "pm_serialize_parse_comments", "pm_serialize_parse_lex", "pm_parse_success_p", "pm_string_query_local", "pm_string_query_constant", "pm_string_query_method_name"}

// rbPrismModule shares the compiler's wazero cache (decision 156): same bytes and wazero, so ~7 ms instead of a ~160 ms AOT compile.
var rbPrismModule = sync.OnceValues(func() (wazero.Runtime, wazero.CompiledModule) {
	ctx := context.Background()
	cfg := wazero.NewRuntimeConfig()
	if base, err := os.UserCacheDir(); err == nil && os.Getenv("RUGAROO_NO_WASM_CACHE") == "" {
		if cache, err := wazero.NewCompilationCacheWithDir(filepath.Join(base, "rugaroo", "wazero")); err == nil {
			cfg = cfg.WithCompilationCache(cache)
		}
	}
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	gz, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(rbPrismWasm))))
	if err != nil {
		panic(NewRuntimeError(Ref(String("prism.wasm: " + err.Error()))))
	}
	bin, err := io.ReadAll(gz)
	if err != nil {
		panic(NewRuntimeError(Ref(String("prism.wasm: " + err.Error()))))
	}
	mod, err := rt.CompileModule(ctx, bin)
	if err != nil {
		panic(NewRuntimeError(Ref(String("prism.wasm: " + err.Error()))))
	}
	return rt, mod
})

// rbPrismInstance is one instantiation: its memory is not safe for two goroutines, so a call takes one from rbPrismPool.
type rbPrismInstance struct {
	mem                                  api.Memory
	calloc, free                         api.Function
	bufSizeof, bufInit, bufValue, bufLen api.Function
	bufFree                              api.Function
	fns                                  [len(rbPrismExports)]api.Function
}

var rbPrismPool = sync.Pool{New: func() any {
	rt, mod := rbPrismModule()
	m, err := rt.InstantiateModule(context.Background(), mod, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		panic(NewRuntimeError(Ref(String("prism.wasm: " + err.Error()))))
	}
	in := &rbPrismInstance{mem: m.Memory(), calloc: m.ExportedFunction("calloc"), free: m.ExportedFunction("free"),
		bufSizeof: m.ExportedFunction("pm_buffer_sizeof"), bufInit: m.ExportedFunction("pm_buffer_init"),
		bufValue: m.ExportedFunction("pm_buffer_value"), bufLen: m.ExportedFunction("pm_buffer_length"), bufFree: m.ExportedFunction("pm_buffer_free")}
	for i, name := range rbPrismExports {
		in.fns[i] = m.ExportedFunction(name)
	}
	return in
}}

// wasmCall uses CallWithStack: goCalled reads a Go selector named Call as Ruby call and keeps every class's as a Go method (decision 427).
func (in *rbPrismInstance) wasmCall(f api.Function, args ...uint64) uint64 {
	stack := append(make([]uint64, 0, 4), args...)
	if len(stack) == 0 {
		stack = append(stack, 0) // room for the result
	}
	if err := f.CallWithStack(context.Background(), stack); err != nil {
		panic(NewRuntimeError(Ref(String("prism.wasm: " + err.Error()))))
	}
	return stack[0]
}

// cstr copies s into wasm memory with a NUL after it; the caller frees it.
func (in *rbPrismInstance) cstr(s string) uint64 {
	p := in.wasmCall(in.calloc, 1, uint64(len(s))+1)
	if p == 0 || !in.mem.WriteString(uint32(p), s) {
		panic(NewNoMemoryError(Ref(String("prism.wasm: out of memory"))))
	}
	return p
}

// rbPrismSerialize is one pm_serialize_* call over src with dump_options' bytes: what it wrote, as the FFI backend's buffer.read.
func rbPrismSerialize(fn int, src, opts string) String {
	in := rbPrismPool.Get().(*rbPrismInstance)
	defer rbPrismPool.Put(in)
	sp, op := in.cstr(src), in.cstr(opts)
	defer in.wasmCall(in.free, sp)
	defer in.wasmCall(in.free, op)
	buf := in.wasmCall(in.calloc, in.wasmCall(in.bufSizeof), 1)
	if buf == 0 || in.wasmCall(in.bufInit, buf) == 0 {
		panic(NewNoMemoryError(Ref(String("prism.wasm: out of memory"))))
	}
	defer in.wasmCall(in.bufFree, buf) // frees the pm_buffer_t too
	in.wasmCall(in.fns[fn], buf, sp, uint64(len(src)), op)
	b, ok := in.mem.Read(uint32(in.wasmCall(in.bufValue, buf)), uint32(in.wasmCall(in.bufLen, buf)))
	if !ok {
		panic(NewRuntimeError(Ref(String("prism.wasm: buffer outside memory"))))
	}
	return String(b) // a copy: the instance's memory is reused
}

// rbPrismCall is pm_parse_success_p (src, opts) or a pm_string_query_* (src, encoding name): its int result.
func rbPrismCall(fn int, src, arg string) Integer {
	in := rbPrismPool.Get().(*rbPrismInstance)
	defer rbPrismPool.Put(in)
	sp, ap := in.cstr(src), in.cstr(arg)
	defer in.wasmCall(in.free, sp)
	defer in.wasmCall(in.free, ap)
	return Integer(int32(uint32(in.wasmCall(in.fns[fn], sp, uint64(len(src)), ap)))) //nolint:gosec // a C int, sign-extended
}
