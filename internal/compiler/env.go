package compiler

import "os"

// envConfig is the RUGAROO_* environment, read once per compile (not per process) so a test's t.Setenv is seen; flags are "non-empty".
type envConfig struct {
	NoWasmCache        bool   // RUGAROO_NO_WASM_CACHE: no wazero compilation cache for prism.wasm
	InferDebug         bool   // RUGAROO_INFER_DEBUG: what each inference round typed and could not compile
	ConstDebug         bool   // RUGAROO_CONST_DEBUG: why a gem constant was left nil (decision 205), even when a discarded pass found it
	Timing             bool   // RUGAROO_TIMING: per-phase durations on stderr
	ErrStack           bool   // RUGAROO_ERR_STACK: the Go stack of whatever pass raised an error
	AllLibs            bool   // RUGAROO_ALL_LIBS: load every prelude lib, required or not
	GemWarnings        bool   // RUGAROO_GEM_WARNINGS: print gem files' warnings too
	PruneDebug         bool   // RUGAROO_PRUNE_DEBUG: why gem pruning reached each method name
	NoPrune            bool   // RUGAROO_NO_PRUNE: keep every declaration
	NoSpecialize       bool   // RUGAROO_NO_SPECIALIZE: keep module funcs generic over Self (decision 377)
	NoWindowReuse      bool   // RUGAROO_NO_WINDOW_REUSE: each_cons/each_slice copy every window (decision 386)
	NoCountedLoop      bool   // RUGAROO_NO_COUNTED_LOOP: times/upto/downto/(a..b).each range over their iterators, Array#[]= stays a call (decision 387)
	NoZipFuse          bool   // RUGAROO_NO_ZIP_FUSE: zip builds its Array of pairs before a chained consumer or block runs (decision 389)
	PruneWhy           bool   // RUGAROO_PRUNE_WHY: the keep chains of the pruner
	KeepGoing          bool   // RUGAROO_KEEP_GOING: record a collect error and go on
	DispatchDebug      string // RUGAROO_DISPATCH_DEBUG: "all", or any other non-empty value for Kernel/Object/module owners
	InferUnknown       bool   // RUGAROO_INFER_UNKNOWN: inference rounds with an explicit not-known-yet type (docs/research/inference-2026-11.md), until its gates pass
	InferUses          string // RUGAROO_INFER_USES: print each round's recorded uses of the parameters whose key holds this text (file:line:col:Class#name)
	NoAnyUse           bool   // RUGAROO_NO_ANY_USE: keep a gem parameter typed when a gem call also passes it an untyped value (A/B for decision 444's settled-round rule)
	TreeCheck          bool   // RUGAROO_TREE_CHECK: report each def whose syntax tree a pass rewrote in place (decision 478)
	NoFileReuse        bool   // RUGAROO_NO_FILE_REUSE: parse the sources again in every round and pass (A/B for decision 479)
	NoWidenChain       bool   // RUGAROO_NO_WIDEN_CHAIN: a final pass that widens a parameter does not widen what it passes on in the same pass (A/B for decision 480)
	NoModuleArms       bool   // RUGAROO_NO_MODULE_ARMS: a non-dispatched module method keeps a Dyn wrapper on each includer (A/B for decision 485)
	NoPrivateDispatch  bool   // RUGAROO_NO_PRIVATE_DISPATCH: private defs stay out of type-switch dispatch (A/B for decision 486)
	NoIncluderDispatch bool   // RUGAROO_NO_INCLUDER_DISPATCH: a module's self call of its includers' class method stays dynamic and keeps that method a Go method (A/B for decision 486)
	NoEarlyRounds      bool   // RUGAROO_NO_EARLY_ROUNDS: run inference rounds to the cap even once they have settled (A/B for decision 444)
	ArmDebug           bool   // RUGAROO_ARM_DEBUG: print why an arm of an untyped receiver's block call does not compile
	GopoisonDump       string // RUGAROO_GOPOISON_DUMP: path prefix for the Go before rejected gem methods become stubs
	UntypedWhy         bool   // RUGAROO_UNTYPED_WHY: each dynamic-call warning names where its untyped receiver came from
	UntypedReport      string // RUGAROO_UNTYPED_REPORT: the roots of untyped receivers ranked by dynamic-call sites, to stderr (1) or a file (untypedreport.go)
	IntOverflow        string // RUGAROO_INT_OVERFLOW: raise (default), wrap or promote; the CLI's --int-overflow sets it (decision 372)
	RegexpCompile      string // RUGAROO_REGEXP_COMPILE: lazy (default) or eager; the CLI's --regexp-compile sets it (decision 471)
	NilCheck           string // RUGAROO_NIL_CHECK: compile (default) or runtime; the CLI's --nil-check sets it (decision 393)
	CGO                bool   // RUGAROO_CGO: Fiddle's backend is cgo, not purego; the CLI's --cgo sets it (decision 426)
	Compiler           string // RUGAROO_COMPILER: the Go compiler the output is for, gc (default) or tinygo; the CLI's -compiler sets it (decision 443)
	RBSCollection      string // RUGAROO_RBS_COLLECTION: an `rbs collection install` directory to read gem signatures from (decision 457)
}

// why reports that untyped values' roots are tracked: for the warnings' text or for the report.
func (e envConfig) why() bool { return e.UntypedWhy || e.UntypedReport != "" }

func loadEnv() envConfig {
	set := func(name string) bool { return os.Getenv(name) != "" }
	return envConfig{
		NoWasmCache:        set("RUGAROO_NO_WASM_CACHE"),
		InferDebug:         set("RUGAROO_INFER_DEBUG"),
		ConstDebug:         set("RUGAROO_CONST_DEBUG"),
		Timing:             set("RUGAROO_TIMING"),
		ErrStack:           set("RUGAROO_ERR_STACK"),
		AllLibs:            set("RUGAROO_ALL_LIBS"),
		GemWarnings:        set("RUGAROO_GEM_WARNINGS"),
		PruneDebug:         set("RUGAROO_PRUNE_DEBUG"),
		NoPrune:            set("RUGAROO_NO_PRUNE"),
		NoSpecialize:       set("RUGAROO_NO_SPECIALIZE"),
		NoWindowReuse:      set("RUGAROO_NO_WINDOW_REUSE"),
		NoCountedLoop:      set("RUGAROO_NO_COUNTED_LOOP"),
		NoZipFuse:          set("RUGAROO_NO_ZIP_FUSE"),
		PruneWhy:           set("RUGAROO_PRUNE_WHY"),
		KeepGoing:          set("RUGAROO_KEEP_GOING"),
		DispatchDebug:      os.Getenv("RUGAROO_DISPATCH_DEBUG"),
		InferUnknown:       os.Getenv("RUGAROO_INFER_UNKNOWN") != "",
		InferUses:          os.Getenv("RUGAROO_INFER_USES"),
		NoAnyUse:           os.Getenv("RUGAROO_NO_ANY_USE") != "",
		NoModuleArms:       os.Getenv("RUGAROO_NO_MODULE_ARMS") != "",
		NoPrivateDispatch:  set("RUGAROO_NO_PRIVATE_DISPATCH"),
		NoIncluderDispatch: set("RUGAROO_NO_INCLUDER_DISPATCH"),
		NoWidenChain:       os.Getenv("RUGAROO_NO_WIDEN_CHAIN") != "",
		NoFileReuse:        os.Getenv("RUGAROO_NO_FILE_REUSE") != "",
		TreeCheck:          os.Getenv("RUGAROO_TREE_CHECK") != "",
		NoEarlyRounds:      os.Getenv("RUGAROO_NO_EARLY_ROUNDS") != "",
		ArmDebug:           os.Getenv("RUGAROO_ARM_DEBUG") != "",
		GopoisonDump:       os.Getenv("RUGAROO_GOPOISON_DUMP"),
		UntypedWhy:         set("RUGAROO_UNTYPED_WHY"),
		UntypedReport:      os.Getenv("RUGAROO_UNTYPED_REPORT"),
		IntOverflow:        os.Getenv("RUGAROO_INT_OVERFLOW"),
		RegexpCompile:      os.Getenv("RUGAROO_REGEXP_COMPILE"),
		NilCheck:           os.Getenv("RUGAROO_NIL_CHECK"),
		CGO:                set("RUGAROO_CGO"),
		Compiler:           os.Getenv("RUGAROO_COMPILER"),
		RBSCollection:      os.Getenv("RUGAROO_RBS_COLLECTION"),
	}
}
