package compiler

import "os"

// envConfig is the RUGAROO_* environment, read once per compile (not per process) so a test's t.Setenv is seen; flags are "non-empty".
type envConfig struct {
	NoWasmCache   bool   // RUGAROO_NO_WASM_CACHE: no wazero compilation cache for prism.wasm
	InferDebug    bool   // RUGAROO_INFER_DEBUG: what each inference round typed and could not compile
	ConstDebug    bool   // RUGAROO_CONST_DEBUG: why a gem constant was left nil (decision 205), even when a discarded pass found it
	Timing        bool   // RUGAROO_TIMING: per-phase durations on stderr
	ErrStack      bool   // RUGAROO_ERR_STACK: the Go stack of whatever pass raised an error
	AllLibs       bool   // RUGAROO_ALL_LIBS: load every prelude lib, required or not
	GemWarnings   bool   // RUGAROO_GEM_WARNINGS: print gem files' warnings too
	PruneDebug    bool   // RUGAROO_PRUNE_DEBUG: why gem pruning reached each method name
	NoPrune       bool   // RUGAROO_NO_PRUNE: keep every declaration
	NoSpecialize  bool   // RUGAROO_NO_SPECIALIZE: keep module funcs generic over Self (decision 377)
	NoWindowReuse bool   // RUGAROO_NO_WINDOW_REUSE: each_cons/each_slice copy every window (decision 386)
	NoCountedLoop bool   // RUGAROO_NO_COUNTED_LOOP: times/upto/downto/(a..b).each range over their iterators, Array#[]= stays a call (decision 387)
	NoZipFuse     bool   // RUGAROO_NO_ZIP_FUSE: zip builds its Array of pairs before a chained consumer or block runs (decision 389)
	PruneWhy      bool   // RUGAROO_PRUNE_WHY: the keep chains of the pruner
	KeepGoing     bool   // RUGAROO_KEEP_GOING: record a collect error and go on
	DispatchDebug string // RUGAROO_DISPATCH_DEBUG: "all", or any other non-empty value for Kernel/Object/module owners
	GopoisonDump  string // RUGAROO_GOPOISON_DUMP: path prefix for the Go before rejected gem methods become stubs
	UntypedWhy    bool   // RUGAROO_UNTYPED_WHY: each dynamic-call warning names where its untyped receiver came from
	IntOverflow   string // RUGAROO_INT_OVERFLOW: raise (default), wrap or promote; the CLI's --int-overflow sets it (decision 372)
	NilCheck      string // RUGAROO_NIL_CHECK: compile (default) or runtime; the CLI's --nil-check sets it (decision 393)
	CGO           bool   // RUGAROO_CGO: Fiddle's backend is cgo, not purego; the CLI's --cgo sets it (decision 426)
	Compiler      string // RUGAROO_COMPILER: the Go compiler the output is for, gc (default) or tinygo; the CLI's -compiler sets it (decision 443)
}

func loadEnv() envConfig {
	set := func(name string) bool { return os.Getenv(name) != "" }
	return envConfig{
		NoWasmCache:   set("RUGAROO_NO_WASM_CACHE"),
		InferDebug:    set("RUGAROO_INFER_DEBUG"),
		ConstDebug:    set("RUGAROO_CONST_DEBUG"),
		Timing:        set("RUGAROO_TIMING"),
		ErrStack:      set("RUGAROO_ERR_STACK"),
		AllLibs:       set("RUGAROO_ALL_LIBS"),
		GemWarnings:   set("RUGAROO_GEM_WARNINGS"),
		PruneDebug:    set("RUGAROO_PRUNE_DEBUG"),
		NoPrune:       set("RUGAROO_NO_PRUNE"),
		NoSpecialize:  set("RUGAROO_NO_SPECIALIZE"),
		NoWindowReuse: set("RUGAROO_NO_WINDOW_REUSE"),
		NoCountedLoop: set("RUGAROO_NO_COUNTED_LOOP"),
		NoZipFuse:     set("RUGAROO_NO_ZIP_FUSE"),
		PruneWhy:      set("RUGAROO_PRUNE_WHY"),
		KeepGoing:     set("RUGAROO_KEEP_GOING"),
		DispatchDebug: os.Getenv("RUGAROO_DISPATCH_DEBUG"),
		GopoisonDump:  os.Getenv("RUGAROO_GOPOISON_DUMP"),
		UntypedWhy:    set("RUGAROO_UNTYPED_WHY"),
		IntOverflow:   os.Getenv("RUGAROO_INT_OVERFLOW"),
		NilCheck:      os.Getenv("RUGAROO_NIL_CHECK"),
		CGO:           set("RUGAROO_CGO"),
		Compiler:      os.Getenv("RUGAROO_COMPILER"),
	}
}
