// Package compiler turns typed Ruby (rbs-inline) into Go.
package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/jtarchie/rugaroo/internal/boot"
	"github.com/jtarchie/rugaroo/internal/rbs"
	"github.com/tetratelabs/wazero"
)

// Compiler holds the whole program: the prelude and the user files, one closed world.
type Compiler struct {
	// grouped state; fields are promoted, so c.name still works
	accessorAny  map[string]bool   // anyClassHasAccessor's memo, valid for len(classList) == accessorAnyN
	inputs       map[string]string // every file read from disk beyond the sources, by path -> content hash ("" absent): the inference cache's manifest (decision 430)
	accessorAnyN int
	dynState
	memoState
	bootFacts
	marshalState
	instState
	pathCache
	tableState
	refineState
	windowState

	genCount, genLimit    *int                        // test hook (decision 240): genExpr calls, and the count after which they fail the compile
	erbSnippets           map[*parser.CallNode]*File  // compiled ERB templates by their result call (decision 111)
	rewrites              map[parser.Node]parser.Node // desugared nodes, built once so every pass sees one tree
	loopInner             map[*parser.CallNode]bool   // the loop calls loopRescue wrapped, not wrapped again
	sureTake              map[*parser.CallNode]bool   // shift/pop an emptiness guard proves non-nil (decision 391)
	erbCounter            int
	labels                map[string]string // Go function (its table key) → Ruby backtrace label, for every function emitted (decision 106)
	classes               map[string]*Class
	classList             []*Class
	topDefs               map[string]*Method
	consts                map[string]*Const
	topConstNames         []string        // constants declared at top level: Object's table
	ivarsDiscovered       bool            // discoverIvars has run: a gem body that fails now is poisoned for good (decision 220)
	keepNames             []string        // method names the gem pruner keeps though no call names them (decision 230)
	modIsA                map[*Class]bool // modules an untyped value's is_a? tests at run time (decision 228)
	symLits               map[string]bool // symbol literals in gem and user files (symbolLiterals)
	warned                map[string]bool
	dataVar               bool                         // rbDATA is declared
	beginStmts            map[*File][]parser.Node      // BEGIN { } bodies, run first in their file
	regexpVars            map[string]string            // literal → its variable, to share one per pattern
	fiddleAliases         map[*Class]map[string]string // Fiddle::Importer typealiases read so far, per module (decision 326)
	ffiNamed              map[*Class]map[string]string // FFI::Library callbacks ("callback") and enums ("enum") read so far, per module
	ffiAliases            map[*Class]map[string]string // FFI::Library typedefs read so far, per module (decision 375)
	ffiLayouts            map[string]*ffiLayout        // FFI::Struct subclasses' literal layouts, by Ruby name (decision 426)
	ffiStubs              []string                     // C stubs for by-value struct signatures under --cgo (decision 426)
	ffiStubTypes          []string                     // their struct typedefs
	fiddleSlots           map[*Class]int               // extern/bind methods synthesized so far, per module: each one's __fiddle_func_at slot (decision 329)
	aliasNews             []aliasNewReq                // `alias_method :[], :new` on a synthesized new, made in link (decision 253)
	needRequireTable      bool                         // a require of a computed name: rbRequireComputed checks it against the compiled files (decision 246)
	programFile           *File                        // the file $0 names: the main file, or a traced compile's -main (decisions 241, 288)
	constList             []*Const
	globals               map[string]*Const // a program's own $globals, by name (decision 361)
	hooks                 []classHook       // main.rb's inherited/included/extended sites
	topDefList            []*Method
	verbatim              []verbatim
	mainStmts             []parser.Node
	classStmts            []*classStmt     // class/module body statements that run at load time (decision 166)
	classBodyLits         []classBodyLit   // unannotated class-body `@x = 1`s, declared after discovery if still untyped (decision 383)
	bodyGroup             *classStmt       // collecting a class body with locals: its statements, as one unit (decision 170)
	mainFile              *File            // the first user file: $0, and the generated header
	userFiles             []*File          // in load order
	loadCode              map[*File]string // a required file's top level, generated, for the loadFile that splices it
	required              map[*File]bool   // user files require_relative loaded: they run where required, not as a top level of their own
	stmtFile              map[parser.Node]*File
	selfTyped             map[*Class]bool // a module whose constraint names Self (decision 208)
	files                 []*File
	preludeFS             fs.FS
	parser                *parser.Parser
	loaded                map[string]bool           // prelude names, and user files' real paths
	parsed                map[string]*File          // user files by real path, parsed by scanRequires before the prelude loads
	resolvingAliases      []string                  // sig `type` aliases being resolved, innermost last: a recursive one is untyped
	promote               bool                      // the flag or some file's comment is int_overflow promote: Bignum is loaded (decision 381)
	loadPath              []string                  // -I directories (decision 131)
	gemDirs               []string                  // installed gems' lib dirs, searched after loadPath: their files are gem code (decision 173)
	sigs                  map[string]*rbs.ClassDecl // .rbs signatures by qualified class name (decision 160)
	gemSigs               fs.FS                     // vendored gem `.rbs` signatures (decision 160), or nil
	autoloaded            map[string]bool           // boot-resolved autoload files: loaded where their autoload registers (decision 175)
	lazyAutoloads         map[string]bool           // autoload files boot registered and never loaded (decision 444)
	droppedLazy           map[string]bool           // of those, the ones that did not compile (options.droppedLazy)
	constNamed            map[string]bool           // names the kept files read as constants (neededAutoloads)
	pruneGems             bool                      // a gem program: unreached gem methods are dropped (decision 169)
	usedBundled           bool                      // a bundled gem's lib joined the gem dirs (decision 427)
	gemNilIvars           []gemNilIvar              // gem ivars discovery saw only nil assigned to (decision 169)
	untypedIvars          map[ivarKey]*pendingIvar  // gem ivars discovery saw only untyped values assigned to, so far (decision 392)
	uncalledWriters       map[*Method]ivarKey       // attr writers no call types, by the ivar they write (decision 445)
	writerReach           map[string]bool           // writerReachable's memo
	nilIvars              map[ivarKey]bool          // class#@ivar assigned nil where no type was known: a subclass's assignment types it (decision 182)
	autoloads             map[string]bool           // `autoload :X` constants by qualified name: defined? answers for them (decision 167)
	pendingAutoloads      []pendingAutoload         // gem autoloads whose path is computed: placed by the constant's file once every file is collected (decision 175)
	out                   strings.Builder
	convs                 map[string]bool // conversion sites emitted during a refineIvars dry run
	Warnings              []string
	methodRequiresRuntime bool                              // traced mode (-main): a require in a method resolves at run time, not at compile time (decisions 197, 288)
	prepends              []pendingPrepend                  // prepends in load order, applied at link (decision 199)
	badClasses            map[*Class]compileError           // gem classes that cannot compile, an error where reachable code names them
	goFuncs               map[string]goFunc                 // each gem method's Go func name -> its key and name (decision 303)
	kept                  []string                          // RUGAROO_KEEP_GOING: collect errors recorded instead of stopping
	anonDepth             int                               // collecting an anonymous class body, whose errors scanAnon defers
	constChain            []string                          // constants whose types are being inferred, for a cycle's message
	classIDs              map[*Class]int                    // index in classList: the class's ID in the generated tables
	specClasses           int                               // describes declared, for their classes' Go names
	anonClasses           map[*parser.CallNode]*Class       // each literal's class
	inheritedAliases      []pendingAlias                    // aliases of an ancestor's method, copied once supers resolve
	unnamedAnon           map[*parser.CallNode]bool         // Class.new literals no constant names: they run inherited where evaluated
	anonRun               *[]parser.Node                    // collecting a Class.new block inside a method: its statements that run per evaluation (decision 259)
	ivarReadFirst         map[ivarKey]bool                  // owner#@x read before any assignment typed it, in discovery: declared T?
	initWrites            map[*Class]*initWrites            // what each class's construction settles before other methods run (decision 394)
	interpOnly            map[pendingKey]bool               // a gem parameter its body only interpolates: any value does (decision 330)
	gemByName             map[string][]*Method              // gem methods by name, for a dynamic call's uses (decision 331)
	gemParams             map[pendingKey]bool               // a gem method's parameters: every caller's disagreeing uses widen them (decision 331)
	ivarBitsMarked        bool                              // markIvarBits ran: an unmarked defined?(@x) is a bug, not discovery
	privateConsts         map[string]bool                   // private_constant's full names: no `M::X` path reaches them
	anonErrors            map[*parser.CallNode]compileError // a literal whose body did not collect: raised where it is generated
	specUses              []specUse                         // Minitest::Spec DSL calls, checked after link
	fiddleTwins           map[*parser.DefNode]fiddleTwin    // an extern's typed twin, by the extern's def (decision 442)

	// parameter types from use (decision 146)
	infer       *inference
	round       bool                      // an inference round: pending parameters are untyped, their arguments recorded
	inferDone   bool                      // the rounds ran: a parameter still untyped is an error
	noPrune     bool                      // formatGo keeps every declaration
	cPreamble   string                    // the C above `import "C"` under --cgo (decision 426)
	env         envConfig                 // the RUGAROO_* environment, read when this compile started
	uses        map[pendingKey][]paramUse // this round's arguments, by parameter key
	pendingSeen map[pendingKey]bool       // this round's untyped parameter keys
	roundDrops  map[pendingKey]bool       // this round's untyped parameters whose method's untyped returns were dropped (decision 383)
}

// dynState: dispatchers, wrappers and forwarders for calls on untyped values (dynamic*.go, dispatch.go).
type dynState struct {
	dynNames      []string        // method names called on untyped values
	blocklessTwin map[string]bool // gem method names with a synthesized `__name_enum` (decision 418)
	usesLineNo    bool            // the program reads $. (decision 425)
	aliasKeys     map[string]bool // names a dynamic method(:name) keys by def, for aliases (emitAliasKeys, decision 418)
	dynSeen       map[string]bool
	sends         map[string]sendSpec // rbSend_ dispatchers calls named (dispatch.go)
	sendOut       map[string]bool     // dispatchers emitted
	dynAll        bool                // a computed send/respond_to?: every method may be named
	dynAllStrict  bool                // ... from user code: prelude-only names too (gem code narrows, decision 235)
	respondNames  []string            // names asked about with respond_to? at run time
	respondSeen   map[string]bool
	markers       map[string]bool         // rbHas<marker> helpers emitted
	classOf       bool                    // `.class` on a value only known at run time: emit rbClassOf
	dynGo         map[string]string       // Go name → the Ruby name its dispatchers serve
	dynWrapped    map[*Class][]dynWrapped // wrappers emitted, per class, for _Call
	sendOn        map[string][]*Class     // per Go suffix, the struct classes a computed send/respond_to? on a receiver of known class may hold (decision 316)
	dynBlockNames map[string]bool         // names a gem calls with a block on an untyped receiver: their block-taking methods get Dyn wrappers (decision 243)
	dynBlockAll   bool                    // a gem sends a block to a computed name: every block-taking method gets a block wrapper (decision 243)
	dynEvery      bool                    // emit every noted dispatcher and forwarder up front: computed send is reachable, or a lazy pass came up short
	respondEvery  bool                    // emit every noted rbResponds up front: a computed respond_to? on an untyped value is reachable (decision 319); dispatchers stay lazy
	dynLazy       map[string]bool         // names noted before any dispatcher went out: emitted only when reached (emitDynamic)
	dynOut        map[string]bool         // dispatchers emitted
	dynGen        map[*Class][]dynGenCase // a generic class's wrappers of this emitDynamic batch, emitted as one switch (decision 209)
	dynGenRound   int                     // batches whose generic wrappers are out: names the next _DynGen method
	respondOut    map[string]bool         // respond_to? checks emitted
	fwdOut        map[*Class]bool         // forwarders emitted
	callable      map[string]bool         // what the _Call tables switch over, once kept code asks for _Call (callableNames)
	dynEach       bool                    // a gem file calls each with a block on an untyped receiver: emit rbEachAny (decision 176)
	dynEachV      bool                    // ... forwarding a block whose value each reads: emit rbEachAnyV (decision 418)
}

// memoState: memos and re-entrancy flags of the analysis helpers.
type memoState struct {
	universalNames      map[string]bool     // memo of universalHas
	dispatchMemo        map[*Method]bool    // memo of dispatchable
	sendArmsMemo        map[[2]any][]*Class // memo of sendArms, valid while classList has sendArmsN classes
	sendArmsN           int
	privateInMemo       map[string][]*Class // memo of privateIn, valid while classList has privateInN classes
	privateInN          int
	objFreezeMemo       int             // objFreeze: 0 unasked, 1 no, 2 yes
	pendingSigs         int             // signatures rendered while their return was still being inferred: memos taken across one are provisional
	goCalledNames       map[string]bool // memo of goCalled
	renderingConstraint bool            // constraintNamesSelf is rendering a module interface
	coerceOwned         *bool           // memo of ownCoerce
	bareRaise           int             // gemBareRaise's memo: 0 unknown, 1 no, 2 yes
}

// branchKey is where an if/unless/case starts: its line and the column on it (boot.BranchInfo's Loc and Col).
type branchKey struct {
	boot.Loc
	col int
}

// bootFacts: what the MRI boot run recorded about a gem program (decisions 198-204, 241, 301, 324).
type bootFacts struct {
	dmSites          map[*parser.CallNode]bool             // define_method calls boot recorded: no-ops at run time (decision 198)
	dmDefined        map[*parser.CallNode]bool             // the define_method calls boot's methods became defs from
	valueCalls       map[string]int                        // names of calls whose value is used, 2 with a block (callValueUsed)
	smSites          map[*parser.CallNode]*parser.CallNode // a gem's run-time define_singleton_method: the call recording it (decision 324)
	bootBranches     map[branchKey]boot.BranchInfo         // each if/unless boot ran, by where it starts (decision 201)
	bootConstClasses map[string]string                     // constant -> the class boot saw it name (decision 203)
	evalSites        map[boot.Loc]bool                     // where each string eval boot compiled is: a no-op (decision 204)
	evalUnits        map[boot.Loc][]evalUnit               // where an instance_eval boot ran that defined classes: each source and its parsed file, whose load-time statements run at the call (decision 324)
	runEvals         map[boot.Loc][]evalUnit               // where a run-time instance_eval the trace saw was called: each source, compiled to run there (decision 374)
	evalClasses      map[*Class]bool                       // classes a run-time string eval defines: constant lookups see them once it has run (decision 324)
	blockEvalSites   map[*parser.CallNode]bool             // `klass.class_eval do ... end` boot ran: its block is that module's body (decision 204)
	bootDelegators   map[boot.Loc][]string                 // def_delegators' names boot saw, by call site (decision 204)
	bootLoadPath     []string                              // $LOAD_PATH as boot left it, nil without a boot (decision 301)
	booted           bool                                  // a boot snapshot ran: a gem autoload it did not resolve is never reached (decision 175)
	bootStructs      map[string][]boot.StructProp          // each T::Struct's props boot saw (decision 241)
}

// marshalState: Marshal's generated cases (decision 137).
type marshalState struct {
	marshalSeen    map[string]Type // concrete container and tuple types of values, by Ruby type: Marshal's cases (decision 137)
	marshalGo      map[string]bool // their Go types with a case emitted
	marshalSkipped []string        // types left out of Marshal's cases as unreached: reaching one later recompiles eagerly
	marshalCode    string          // rbMDumpGen/rbMLoadGen, rendered when the tables go out
	marshalErr     *compileError   // a bad marshal_dump/marshal_load, raised once Marshal is reached
	marshalAt      int             // len(marshalSeen) then
	marshalOut     bool            // rbMDumpGen/rbMLoadGen emitted
	marshalLate    bool            // rbMDumpObjGen/rbMLoadObjGen emitted
}

// instState: generic class instances (decision 308).
type instState struct {
	insts       map[string]*Class // generic instances by instKey (decision 308)
	instReqs    *[]string         // instKeys to create before link: grows when a compile names a new one
	instAt      map[string]srcPos // where each instance was first named (checkBounds)
	instOpen    bool              // makeInstances is running: instOf may create
	instMissing bool              // this compile named an instance it lacks: its result is discarded for a restart
}

// pathCache: per-compile caches of path and pattern lookups.
type pathCache struct {
	gemDirsReal    []string                  // gemDirs with symlinks resolved, once (gemFile)
	realPaths      map[string]string         // realPath per path, for this compile (c.real)
	requireTargets map[string]string         // findRequire per required name, for this compile
	reCache        map[string]*regexp.Regexp // c.re's compiled patterns
}

// tableState: what emitTables reads (see tableInputs) and the tables' bookkeeping.
type refineState struct {
	refineIn, refineOut map[string]*refineRecord // decision 433: the cache's records and this compile's
	roundRefine         map[string]*refineRecord // options.roundRefine (decision 444)
	inDryRun            bool                     // inside dryRunIvarUsers
	dryTry              bool                     // inside a refineIvars try (not its untyped baseline)
	baseFailed          map[*Method]bool         // gem methods that fail in refineIvars' untyped baseline: their failure vetoes nothing
	widenPending        map[pendingKey]Type      // noteWidenPending's parameters (decision 444)
	loadedFeatures      bool                     // some gem code reads $LOADED_FEATURES: emit rbLoadedFeatures
	localSeeds          map[*Method]localSeed    // decision 444
	nilDefer            *nilDeferral             // the nil sites the ivar typings refineIvars kept are checked at (decision 440)
	noNilDefer          bool                     // options.noNilDefer
	nilDeferUsed        *bool                    // options.nilDeferred: a typing kept by decision 440
}

type tableState struct {
	pkgVars      map[string]bool // package-level var declarations (regexp literals, rbDATA, once-guards), named by what they are; the pruner drops the unnamed
	strLits      map[string]bool // String literal texts: String#frozen? knows them by identity
	tablesOut    bool
	tablesAt     tableInputs       // tableInputs when the tables went out
	procsOut     bool              // rbIsProc/rbDynProcCall emitted, after every other table (emitProcTables)
	procsAt      int               // len(keptProcs()) then
	optConv      map[string]bool   // class T?s bound to a method type parameter: rbOptClassFrom's cases (decision 295)
	procTypes    map[string]bool   // Proc Go types rendered (*func(...)), probes and dry-runs too: rbIsProc lists those keptProcs finds in kept code (decision 431)
	procFuncs    map[string]TFunc  // the type each procTypes entry renders, for rbDynProcCall
	procShapes   map[string]string // per procTypes entry: its funcShape, once (keptProcs)
	keptView     keptCode          // the pruner's view of the kept program (emitNext sets it)
	procMinArity map[string]int    // per procTypes entry: the fewest arguments a gem lambda of that Go type takes (decision 416)
	argBoxes     map[string]bool   // T? boxes of generic type arguments, which generic code may hold
	// concrete T? Go types (*T) rendered anywhere, for rbUnbox; the value
	// says whether T is itself optional
	boxes     map[string]bool
	leafBoxes map[string]bool   // a leaf's C? rendered somewhere (decision 360): the box helpers' nil *C is nil
	unions    map[string]TUnion // rbUnion_<hash>: an untyped value checked into a union (decision 150), by Go name
	unionsOut map[string]bool   // those emitted: a body emitted after the tables can name a new one (emitNext)
}

type verbatim struct {
	srcPos
	code string
}

type compileError struct {
	pos     srcPos // where; zero for an error with no location, or one whose text names it
	msg     string // without the location
	untyped bool   // a parameter inference left untyped (decision 146): a failed round's own error explains it better
	proof   bool   // an in-place String edit the alias proof refused (decisions 382, 395, 405): no untyped retry may hide it
	nilVeto bool   // decision 20's T? where T is expected (decision 440)
}

// Error is `file:line: msg`, or msg alone when there is no position.
func (e compileError) Error() string {
	if e.pos.File == nil {
		return e.msg
	}
	return e.pos.String() + ": " + e.msg
}

// errorf aborts compilation. If n is non-nil the error is located at n's line in f.
func (c *Compiler) errorf(f *File, n parser.Node, format string, args ...any) {
	if n != nil && f != nil {
		c.errorAt(srcPos{f, f.shownLine(n.GetLocation().StartOffset)}, format, args...)
	}
	panic(compileError{msg: fmt.Sprintf(format, args...)})
}

func (c *Compiler) unsupported(f *File, n parser.Node) {
	if os.Getenv("RUGAROO_UNSUPPORTED_STACK") != "" {
		debug.PrintStack()
	}
	c.errorf(f, n, "unsupported syntax: %s", nodeType(n))
}

func nodeType(n parser.Node) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", n), "*parser.")
}

// sharedParser is one Prism pool per process: instantiating the WASM module costs ~0.5s, more than the rest of a compile.
var sharedParser = sync.OnceValues(func() (*parser.Parser, error) {
	opts := []parser.ParserOption{parser.WithVersion(parser.SyntaxVersionLatest), parser.WithPoolSize(runtime.GOMAXPROCS(0))}
	if cfg := wazeroConfig(); cfg != nil {
		opts = append(opts, parser.WithRuntimeConfig(cfg))
	}
	return parser.NewParser(context.Background(), opts...)
})

// wazeroConfig carries a compilation cache under the user's cache directory, so AOT-compiling prism.wasm (~150 ms)
// happens once per wazero version and machine, not per process (#81 item 4). nil when no cache directory is usable,
// or RUGAROO_NO_WASM_CACHE is set; wazero validates the cache against its own version and the wasm's hash.
func wazeroConfig() wazero.RuntimeConfig {
	if loadEnv().NoWasmCache {
		return nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	cache, err := wazero.NewCompilationCacheWithDir(filepath.Join(base, "rugaroo", "wazero"))
	if err != nil {
		return nil
	}
	return wazero.NewRuntimeConfig().WithCompilationCache(cache)
}

// Compile transpiles the prelude (prelude.rb in preludeFS, plus whatever it
// require_relatives) and the main source into one Go file.
func Compile(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte) ([]byte, error) {
	out, _, err := CompileWithWarnings(ctx, preludeFS, mainName, mainSrc)
	return out, err
}

// Source is one user file: its name as messages and //line show it, and its text.
type Source struct {
	Name string
	Src  []byte
	Path string // where it is on disk, for require_relative; Name when empty
}

// options are a compile's settings beyond its sources.
type options struct {
	noNilDefer   bool          // decision 440 off: the recompile after a nil error its dry runs did not see
	nilDeferred  bool          // out: the last compile kept a typing decision 440's run-time checks allowed
	purego       bool          // Fiddle's default backend whatever RUGAROO_CGO says: PreludeWhole type-checks without cgo (decision 426)
	skipTests    bool          // a test_ method that fails to compile becomes a skip (SkippedTest) instead of an error
	skipped      []SkippedTest // what skipTests skipped
	warnings     []string
	loadPath     []string // -I directories, which a user `require` searches first
	gemDirs      []string // gem lib dirs, searched after loadPath (decision 173)
	infer        *inference
	round        bool
	inferDone    bool
	seed         *Inference               // types from an earlier compile of these sources, updated by this one
	allLibs      bool                     // load every prelude lib, required or not (the stub generator, decision 154)
	gemSigs      fs.FS                    // vendored `.rbs` signatures for gems (decision 160), or nil
	pruneGems    bool                     // drop gem methods no reachable code names (decision 169)
	autoloaded   map[string][]string      // real path -> constants of each file boot's autoloads loaded (decision 175)
	mainLast     bool                     // traced mode: the last source is the program's main file, and a require in a method resolves at run time (decisions 197, 288)
	boot         *boot.Manifest           // what boot recorded: define_method calls (decision 198), branches taken (decision 201)
	goFuncs      map[string]goFunc        // out: each gem method's Go func name -> its methodKey and name
	noPrune      bool                     // keep every declaration (RUGAROO_NO_PRUNE=1 also sets it): the whole-prelude program (decision 207)
	env          envConfig                // the RUGAROO_* environment, read once by compile
	testHook     func()                   // tests only: runs at the start of compileWith, to inject a panic
	genCount     *int                     // test hook: genExpr calls, summed over every pass
	genLimit     *int                     // test hook: the count after which genExpr fails the compile
	instances    []string                 // generic instances to create before link, by instKey (decision 308)
	instAt       map[string]srcPos        // where each was first named
	instRestarts int                      // compiles and rounds rerun for a missing instance (RUGAROO_TIMING)
	inputs       map[string]string        // out: the final compile's input files and their hashes (decision 430)
	refineIn     map[string]*refineRecord // refineIvars' records from the cache, by refineKind (decision 433)
	refineOut    map[string]*refineRecord // out: this compile's, for the cache
	roundRefine  map[string]*refineRecord // the previous inference round's candidates and kept typings, tried first by the next (decision 444)
	widenRedo    int                      // final passes rerun by noteWidenPending
	droppedLazy  map[string]bool          // lazy autoload files that did not compile: left out (collectLazy)
}

// ErrInternal marks an error that is a compiler bug (a panic that was neither a compile error nor an inference request), not a fault in the user's program.
var ErrInternal = errors.New("internal error")

// internalError is a recovered non-compile panic (decision 263).
type internalError struct {
	val  any
	file string
}

func (e internalError) Error() string {
	msg := fmt.Sprintf("internal error: %v", e.val)
	if e.file != "" {
		msg += " while compiling " + e.file
	}
	return msg
}

func (e internalError) Is(target error) bool { return target == ErrInternal }

// SkippedTest is a test method skipTests turned into a minitest skip.
type SkippedTest struct {
	File   string
	Line   int
	Name   string // the Go method's Ruby name: test_0001_desc for an `it`
	Reason string // the compile error, without its file:line
}

func compileFresh(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options) ([]byte, error) {
	for range maxInstanceRestarts {
		opts.round, opts.inferDone = false, false
		out, err := compileOnce(ctx, preludeFS, sources, opts)
		var drop lazyDrop
		if errors.As(err, &drop) && !opts.droppedLazy[drop.path] { // compile again without it
			if opts.droppedLazy == nil {
				opts.droppedLazy = map[string]bool{}
			}
			opts.droppedLazy[drop.path] = true
			continue
		}
		if !errors.Is(err, errNeedInstances) {
			if opts.instRestarts > 0 && opts.env.Timing {
				fmt.Fprintf(os.Stderr, "rugaroo: %d compiles restarted for generic instances (%d instances)\n", opts.instRestarts, len(opts.instances))
			}
			return out, err
		}
		opts.instRestarts++
	}
	return nil, fmt.Errorf("generic instances still missing after %d compiles: %s", maxInstanceRestarts, strings.Join(opts.instances, ", "))
}

// lazyDrop: a file only an autoload the traced run never resolved would load (decision 444) does not compile. It is
// optional, so the compile runs again without it, and its constant raises NameError if the program ever names it, as
// before such files were offered (kramdown's Unidecoder, whose module body branches on a run-time `if`).
type lazyDrop struct {
	path string
	err  compileError
}

func (d lazyDrop) Error() string { return d.err.Error() }

// collectLazy is collect, with a lazily offered file's compile error a lazyDrop.
func (c *Compiler) collectLazy(ctx context.Context, path string, uf *File) {
	if !c.lazyAutoloads[path] {
		c.collect(ctx, uf)
		return
	}
	kept := len(c.kept)
	err := catchCompileError(func() { c.collect(ctx, uf) })
	if err != nil {
		panic(lazyDrop{path: path, err: *err})
	}
	if len(c.kept) > kept {
		panic(lazyDrop{path: path, err: compileError{msg: c.kept[kept]}})
	}
}

// errWidenRedo: the final pass widened parameters the rounds typed too narrowly; it runs again (decision 444).
var errWidenRedo = errors.New("parameters widened")

const maxWidenRedo = 4

// inferenceRound is decision 146's round after link: ivars refined, returns inferred, and the parameter uses gathered.
func (c *Compiler) inferenceRound(opts *options) {
	_ = catchCompileError(c.refineIvars)
	for _, m := range c.inferredMethods() {
		m.Ret = nil
	}
	for _, m := range c.inferredMethods() {
		_ = catchCompileError(func() { c.inferRet(m) })
	}
	c.collectUses()
	if !c.instMissing { // what a template stood in for is not evidence: the restart has the instance (decision 308)
		c.updateInference(opts.infer)
	}
}

// widenRedo reports a final pass that widened parameters (noteWidenPending), with the widened types in opts for the
// next one.
func (c *Compiler) widenRedo(opts *options) bool {
	if len(c.widenPending) == 0 || opts.widenRedo >= maxWidenRedo {
		return false
	}
	opts.widenRedo++
	for k, t := range c.widenPending {
		opts.infer.types[k] = t
	}
	if opts.env.Timing {
		fmt.Fprintf(os.Stderr, "rugaroo: final pass again: %d parameters widened to an untyped container\n", len(c.widenPending))
	}
	return true
}

func compileOnce(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options) ([]byte, error) {
	opts.env = loadEnv()
	if opts.seed != nil {
		opts.infer = opts.seed.inf
	}
	final := compileWith
	compileWith := func(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options, dynEvery bool) ([]byte, error) {
		out, err := final(ctx, preludeFS, sources, opts, dynEvery)
		for errors.Is(err, errWidenRedo) {
			opts.skipped = nil
			out, err = final(ctx, preludeFS, sources, opts, dynEvery)
		}
		return out, err
	}
	out, err := compileWith(ctx, preludeFS, sources, opts, false)
	if errors.Is(err, errNeedInfer) {
		if opts.infer == nil {
			opts.infer = &inference{}
		}
		if opts.seed != nil {
			defer func() { opts.seed.inf = opts.infer }()
		}
		roundErr := inferRounds(ctx, preludeFS, sources, opts)
		opts.inferDone = true
		out, err = compileWith(ctx, preludeFS, sources, opts, false)
		var ce compileError
		if roundErr != nil && errors.As(err, &ce) && ce.untyped {
			err = roundErr
		}
	}
	if errors.Is(err, errPruneIncomplete) {
		if opts.env.Timing {
			fmt.Fprintln(os.Stderr, "rugaroo: fallback: recompiling with every dispatcher")
		}
		opts.skipped = nil
		out, err = compileWith(ctx, preludeFS, sources, opts, true)
	}
	var ce compileError
	if opts.nilDeferred && !opts.noNilDefer && errors.As(err, &ce) && ce.nilVeto { // a nil error decision 440's dry runs did not see (a reader's caller): compile as before it
		opts.noNilDefer, opts.skipped = true, nil
		out, err = compileOnce(ctx, preludeFS, sources, opts)
		opts.noNilDefer = false
	}
	return out, err
}

// inferRounds runs decision 146's rounds until nothing changes; the error is one that stopped them.
func inferRounds(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options) (roundErr error) {
	for range maxInferRounds {
		opts.round = true
		_, err := compileWith(ctx, preludeFS, sources, opts, false)
		opts.round = false
		if errors.Is(err, errNeedInstances) {
			opts.instRestarts++
			continue // the next round creates the generic instance this one named (decision 308)
		}
		if err != nil && !errors.Is(err, errInferRound) {
			roundErr = err // it stopped inference: the final compile's untyped parameters are its doing
			if opts.env.InferDebug {
				fmt.Fprintln(os.Stderr, "rugaroo: infer round failed:", err)
			}
		}
		if !errors.Is(err, errInferRound) || !opts.infer.changed {
			break
		}
		if opts.infer.settled >= settledRounds && !opts.env.NoEarlyRounds {
			// a large gem program's rounds never converge: a few parameters flip until the cap, each round a full dry
			// run. The first settled round untypes what an untyped call reaches and stops decision 383's drops; the
			// next ones carry that through, and the final pass redoes what is still off (errWidenRedo). decision 444
			break
		}
	}
	return roundErr
}

// panicToError is compileWith's recover: compileError and needInfer are the compiler's own control flow; any other panic is a compiler bug, returned as an internalError so a CLI or server reports it instead of dying with a Go trace.
func panicToError(r any, sources []Source, mainLast, errStack bool) error {
	var err error
	switch r := r.(type) {
	case compileError:
		err = r
	case needInfer:
		return errNeedInfer
	case needInstances:
		return errNeedInstances
	case lazyDrop:
		return r
	default:
		ie := internalError{val: r}
		if len(sources) > 0 {
			ie.file = sources[0].Name
			if mainLast {
				ie.file = sources[len(sources)-1].Name
			}
		}
		err = ie
	}
	if errStack { // debugging: which pass raised it
		fmt.Fprintf(os.Stderr, "%s\n%s\n", err, debug.Stack())
	}
	return err
}

func compileWith(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options, dynEvery bool) (out []byte, err error) {
	var c *Compiler
	defer func() {
		if r := recover(); r != nil {
			err = panicToError(r, sources, opts.mainLast, opts.env.ErrStack)
		}
		if c != nil && c.instMissing { // whatever failed may be the template standing in for a missing instance
			out, err = nil, errNeedInstances
		}
	}()
	if opts.testHook != nil {
		opts.testHook()
	}
	tick := phaseTimer(opts.env.Timing)
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	tick("parser")

	c = &Compiler{classes: map[string]*Class{}, topDefs: map[string]*Method{}, consts: map[string]*Const{}, anonClasses: map[*parser.CallNode]*Class{}, regexpVars: map[string]string{}, warned: map[string]bool{},
		preludeFS: preludeFS, parser: p, loaded: map[string]bool{}, selfTyped: map[*Class]bool{}, labels: map[string]string{}, erbSnippets: map[*parser.CallNode]*File{}, sigs: map[string]*rbs.ClassDecl{},
		dynState:     dynState{dynEvery: dynEvery, dynSeen: map[string]bool{}, respondSeen: map[string]bool{}, markers: map[string]bool{}, dynGo: map[string]string{}, dynWrapped: map[*Class][]dynWrapped{}, dynOut: map[string]bool{}, respondOut: map[string]bool{}, fwdOut: map[*Class]bool{}},
		marshalState: marshalState{marshalSeen: map[string]Type{}, marshalGo: map[string]bool{}},
		tableState:   tableState{procTypes: map[string]bool{}, procFuncs: map[string]TFunc{}, procShapes: map[string]string{}, argBoxes: map[string]bool{}, boxes: map[string]bool{}, unions: map[string]TUnion{}, unionsOut: map[string]bool{}, strLits: map[string]bool{}, pkgVars: map[string]bool{}}}
	c.loadPath, c.gemDirs, c.pruneGems = opts.loadPath, opts.gemDirs, opts.pruneGems
	c.refineIn, c.refineOut = opts.refineMaps()
	c.roundRefine = opts.roundRefines()
	c.gemSigs = opts.gemSigs
	c.infer, c.round, c.inferDone = opts.infer, opts.round, opts.inferDone
	c.noPrune = opts.noPrune
	c.methodRequiresRuntime = opts.mainLast
	c.env = compileEnv(opts)
	c.genCount, c.genLimit = opts.genCount, opts.genLimit
	c.noNilDefer, c.nilDeferUsed, opts.nilDeferred = opts.noNilDefer, &opts.nilDeferred, false
	c.loadPreludeGo()
	libs, parsed, perr := c.scanRequires(ctx, sources)
	if perr != nil {
		return nil, perr
	}
	c.parsed = parsed
	c.lazyAutoloads, c.droppedLazy = opts.lazyAutoloads(), opts.droppedLazy
	c.autoloaded = c.neededAutoloads(ctx, c.bundledAutoloads(ctx, opts.autoloaded))
	var dms []boot.MethodInfo
	if opts.boot != nil {
		c.booted = true
		dms = opts.boot.DefineMethods
		c.indexBootBranches(opts.boot.Branches)
		c.bootConstClasses = opts.boot.ConstClasses
		c.bootDelegators = parseDelegators(opts.boot.Delegators)
		c.bootLoadPath = opts.boot.LoadPath
		c.bootStructs = opts.boot.Structs
	}
	dms = c.withStaticDefineMethods(opts, dms)
	c.markDefineMethods(dms)
	c.loadPrelude(ctx, "prelude.rb")
	c.loadIntOverflow(ctx)
	c.loadLibs(ctx, libs, opts.allLibs || c.env.AllLibs)
	c.loadSigs(sources)
	tick("prelude")
	c.evalSites = c.classEvalSites(opts.boot)
	for i, src := range sources {
		if i == len(sources)-1 && opts.mainLast && opts.boot != nil { // load-time evals run before the program's main file (decision 204)
			c.collectBootEvals(ctx, opts.boot.Evals)
		}
		path := realPath(sourceName(src))
		if c.loaded[path] { // a file an earlier one require_relative'd: Ruby loads it once
			continue
		}
		c.loaded[path] = true
		uf := parsed[path]
		c.files = append(c.files, uf)
		c.userFiles = append(c.userFiles, uf)
		c.collectLazy(ctx, path, uf)
	}
	for _, uf := range c.userFiles {
		c.keepGoing(func() { c.collectERB(ctx, uf) }) // templates need every constant collected (decision 111)
	}
	c.collectDefineMethods(ctx, dms)
	if opts.boot != nil {
		c.collectBootAliases(opts.boot.Aliases)
		c.collectBootIncludes(opts.boot.Includes)
	}
	c.collectSingletonTemplates(ctx)
	c.collectObjectExtends(ctx)
	c.placePendingAutoloads()
	if len(c.kept) > 0 {
		c.errorf(nil, nil, "%d collect errors:\n%s", len(c.kept), strings.Join(c.kept, "\n"))
	}
	tick("user")
	if c.pruneGems { // set by opts, or by a bundled gem's use (decision 427)
		c.pruneGemMethods()
	}
	c.mainFile = c.userFiles[0]
	c.programFile = c.mainFile
	if opts.mainLast { // a traced compile's -main is the program `ruby` ran: its $0 (decision 197)
		c.programFile = c.userFiles[len(c.userFiles)-1]
	}
	if opts.instAt == nil {
		opts.instAt = map[string]srcPos{}
	}
	c.makeInstances(&opts.instances, opts.instAt)
	c.nameGo()
	c.link(ctx)
	c.checkAbstractIncludes()
	c.guardConsts() // before any body is typed: defined?(X) on a guarded constant is String?
	tick("link")
	for _, cls := range c.classList {
		cls.shapeFixed = true
	}
	c.discoverIvars()
	c.ivarsDiscovered = true
	if c.round {
		c.inferenceRound(opts)
		tick("round")
		return nil, errInferRound
	}
	c.refineIvars()
	c.markIvarBits() // ivars are final; inference below sees defined?(@x)'s String?
	c.markValueClasses()
	if opts.skipTests {
		opts.skipped = c.skipFailingTests()
	}
	c.inferReturns()
	c.relinkOverrides()
	tick("infer")
	c.goFuncs = map[string]goFunc{}
	c.emitProgram()
	opts.goFuncs = c.goFuncs
	if len(c.kept) > 0 {
		c.errorf(nil, nil, "%d errors:\n%s", len(c.kept), strings.Join(c.kept, "\n"))
	}
	tick("emit")
	if c.widenRedo(opts) {
		return nil, errWidenRedo
	}
	src := []byte(c.out.String())
	lazy := map[string]bool{}
	for _, k := range c.constList {
		if k.File.isPrelude() {
			lazy[k.GoName] = true
		}
	}
	formatted, ferr := formatGo(src, lazy, c.emitNext, c.labels, c.noPrune, c.env, c.ffiStubPreamble()+c.cPreamble)
	tick("format")
	opts.warnings = c.Warnings // the tail's bodies warn too
	if errors.Is(ferr, errPruneIncomplete) {
		return nil, ferr
	}
	if ferr != nil {
		// Keep the raw output around for debugging.
		return nil, fmt.Errorf("internal error: generated Go does not parse (%w); %s", ferr, dumpBadOutput(slices.Concat(src, []byte("\n// ---- tail ----\n"), []byte(c.out.String()))))
	}
	opts.inputs = c.cacheInputs()
	return formatted, nil
}

// sortedClasses returns classes in a stable emission order: prelude order
// first (as declared), then user classes.
func (c *Compiler) sortedClasses() []*Class {
	out := append([]*Class(nil), c.classList...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].File.isPrelude() && !out[j].File.isPrelude()
	})
	return out
}

// CompileWithWarnings is Compile plus the warnings collected.
func CompileWithWarnings(ctx context.Context, preludeFS fs.FS, mainName string, mainSrc []byte, loadPath ...string) ([]byte, []string, error) {
	return CompileFilesWithWarnings(ctx, preludeFS, []Source{{Name: filepath.Base(mainName), Src: mainSrc, Path: mainName}}, loadPath...)
}

// CompileFilesWithWarnings compiles several user files into one program, loaded in order (decision 84).
func CompileFilesWithWarnings(ctx context.Context, preludeFS fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	opts := options{loadPath: loadPath}
	out, err := compile(ctx, preludeFS, sources, &opts)
	return out, opts.warnings, err
}

// CompileTestsSkipping is CompileFilesWithWarnings for a test suite that should run what compiles: each test_ method rugaroo cannot compile becomes a skip carrying the error, and is listed.
// seed, when not nil, carries inferred parameter types between compiles of nearly the same sources.
func CompileTestsSkipping(ctx context.Context, preludeFS fs.FS, sources []Source, seed *Inference) ([]byte, []string, []SkippedTest, error) {
	if len(sources) == 0 {
		return nil, nil, nil, errors.New("no Ruby files to compile")
	}
	opts := options{skipTests: true, seed: seed}
	out, err := compile(ctx, preludeFS, sources, &opts)
	return out, opts.warnings, opts.skipped, err
}

// loadPrelude parses and collects one prelude file. `require_relative` at
// its top level pulls in further files, in place, so emission order follows
// require order.
func (c *Compiler) loadPrelude(ctx context.Context, name string) {
	if c.loaded[name] {
		return
	}
	c.loaded[name] = true
	src, err := fs.ReadFile(c.preludeFS, name)
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
	}
	f, err := preludeFile(ctx, c.parser, name, src)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	c.files = append(c.files, f)
	before := len(c.mainStmts) // a vendored file collected earlier may have its own
	c.collect(ctx, f)
	if len(c.mainStmts) > before {
		c.errorf(f, c.mainStmts[before], "prelude must not have top-level statements")
	}
}

// loadVendored loads a vendored gem's lib file from the prelude FS as user code (decision 328): the %x{} escape stays closed, extern
// signatures are compiled, dynamic calls warn, and its top-level statements run as a required file's would.
func (c *Compiler) loadVendored(ctx context.Context, name string) {
	if c.loaded[name] {
		return
	}
	c.loaded[name] = true
	src, err := fs.ReadFile(c.preludeFS, name)
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("vendored gem: %v", err)})
	}
	var libs []string                                           // its own top-level `require "x"` lines: the require scan covers only the program's files
	root := strings.Join(strings.SplitN(name, "/", 3)[:2], "/") // vendored/<gem>
	for _, m := range vendoredRequire.FindAllSubmatch(src, -1) {
		// a file of the same gem (`require "sqlite3/database"`) is vendored too: loaded where MRI's require would load it
		if own := root + "/" + strings.TrimSuffix(string(m[1]), ".rb") + ".rb"; fileExists(c.preludeFS, own) {
			c.loadLibs(ctx, libs, false)
			libs = nil
			c.loadVendored(ctx, own)
			continue
		}
		libs = append(libs, string(m[1]))
	}
	c.loadLibs(ctx, libs, false)
	f, err := parseFile(ctx, c.parser, name, src, originUser)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	c.files = append(c.files, f)
	c.userFiles = append(c.userFiles, f) // a user file, not a required one: its body is a file block before main's (no require site to splice it at)
	c.collect(ctx, f)
}

func fileExists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

var vendoredRequire = regexp.MustCompile(`(?m)^require "([^"]+)"$`)

// preludeFiles caches each prelude file's parse for the process: the prelude is constant, a File is never written after parseFile, and under wasm Prism runs interpreted, where parsing it costs 2.5 s of a 5.5 s compile.
var preludeFiles sync.Map // name → *File

func preludeFile(ctx context.Context, p *parser.Parser, name string, src []byte) (*File, error) {
	if f, ok := preludeFiles.Load(name); ok {
		return f.(*File), nil
	}
	f, err := parseFile(ctx, p, name, src, originPrelude)
	if err != nil {
		return nil, err
	}
	preludeFiles.Store(name, f)
	return f, nil
}

// intOverflowComment is a source's `# int_overflow: mode` line (decision 372).
var intOverflowComment = regexp.MustCompile(`(?m)^# int_overflow: (\S+)\s*$`)

// loadIntOverflow loads Bignum when the flag or any file's comment says promote (decisions 372, 381).
func (c *Compiler) loadIntOverflow(ctx context.Context) {
	c.promote = c.env.IntOverflow == "promote" || slices.ContainsFunc(slices.Collect(maps.Values(c.parsed)), func(uf *File) bool { return uf.overflow == "promote" })
	if c.promote {
		c.loadPrelude(ctx, preludeLibs["__bignum"])
	}
}

// checkOverflowMode rejects a mode other than raise, wrap or promote ("" is raise).
func checkOverflowMode(where, mode string) string {
	if !slices.Contains([]string{"", "raise", "wrap", "promote"}, mode) {
		panic(compileError{msg: fmt.Sprintf("%s: %q is not raise, wrap or promote", where, mode)})
	}
	return mode
}

var intOps = map[string]string{"Op_plus": "+", "Op_minus": "-", "Op_mul": "*", "Op_neg": "-"}

// intMode is the overflow mode of the Integer arithmetic being compiled: its file's own comment, else the flag's.
func (f *fctx) intMode() string {
	if f.f != nil && f.f.overflow != "" {
		return f.f.overflow
	}
	return f.c.env.IntOverflow
}

// intOpCode writes Integer +, -, * and -@ in a file whose mode differs from the program's on wrapping: plain Go
// arithmetic in a wrap file, the checked helpers in a raise or promote file of a wrap program (decision 381).
func (f *fctx) intOpCode(m *Method, recv expr, argList string) (string, bool) {
	wrapFile, wrapProg := f.intMode() == "wrap", f.c.env.IntOverflow == "wrap"
	if wrapFile == wrapProg || m.Owner != f.c.classes["Integer"] || !isClass(recv.typ, "Integer") {
		return "", false
	}
	op, ok := intOps[m.GoName]
	switch {
	case !ok:
		return "", false
	case wrapFile && m.GoName == "Op_neg":
		return "(-" + recv.code + ")", true
	case wrapFile:
		return "(" + recv.code + " " + op + " " + argList + ")", true
	}
	return "rbIntChecked" + strings.TrimPrefix(m.GoName, "Op_") + "(" + recv.code + comma(argList) + ")", true
}

// classEvalSites are boot's class_eval call sites, known before any file is collected: a gem file the traced entry loads
// collects its class_eval before main's turn collects the evals (racc's do_parse, decision 427).
func (c *Compiler) classEvalSites(m *boot.Manifest) map[boot.Loc]bool {
	sites := map[boot.Loc]bool{}
	if m == nil {
		return sites
	}
	for _, e := range m.Evals {
		if l, ok := boot.ParseLoc(e.Caller); ok && e.Kind == "class_eval" && e.Self != "" {
			sites[boot.Loc{File: c.real(l.File), Line: l.Line}] = true
		}
	}
	return sites
}

// loadPreludeGo embeds prelude/go/*.go verbatim: pure Go with no self/param binding, so it skips Ruby parsing entirely.
// Each file is real Go (decision 154): its import block names the std packages its helpers use, which join stdImports,
// so the table only needs hand entries for `%x{}` bodies in .rb files.
func (c *Compiler) loadPreludeGo() {
	names, err := fs.Glob(c.preludeFS, "prelude/go/[a-z]*.go") // the embed pattern: a checkout on disk also holds the generated 0_stubs.go
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
	}
	sort.Strings(names)
	var imports []string
	for _, name := range names {
		src, err := fs.ReadFile(c.preludeFS, name)
		if err != nil {
			panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
		}
		body, line, fileImports, err := stripGoPackage(src)
		if err != nil {
			panic(compileError{msg: fmt.Sprintf("%s: %v", name, err)})
		}
		imports = append(imports, fileImports...) // every backend's, so the once-per-process table serves either lane
		if !c.goFileSelected(src) {
			continue
		}
		c.cPreamble += c.cgoPreamble(name, src)
		if c.env.IntOverflow == "wrap" {
			body = strings.Replace(body, "const rbIntWrap = false", "const rbIntWrap = true", 1) // decision 372
		}
		c.verbatim = append(c.verbatim, verbatim{srcPos: srcPos{&File{Name: name, origin: originPrelude}, line}, code: body})
	}
	preludeImports.Do(func() { mergeStdImports(stdImports, imports) })
}

// mergeStdImports adds each import to table under its last path element. A name the table already holds keeps its entry (the hand table wins); one held for a different path is returned, for TestPreludeImportsResolve to fail on.
func mergeStdImports(table map[string]string, imports []string) (collisions []string) {
	for _, imp := range imports {
		base := path.Base(imp)
		if cur, ok := table[base]; !ok {
			table[base] = imp
		} else if cur != imp {
			collisions = append(collisions, fmt.Sprintf("%s: table has %s, prelude/go imports %s", base, cur, imp))
		}
	}
	return collisions
}

// preludeImports guards the one-time merge of prelude/go's imports into stdImports: every compile runs loadPreludeGo
// before it reads the table, and Once orders those reads after the write.
var preludeImports sync.Once

// compileEnv is opts' environment with the modes checked; PreludeWhole's stays on purego and gc (decisions 426, 443).
func compileEnv(opts *options) envConfig {
	env := opts.env
	env.IntOverflow = checkOverflowMode("--int-overflow", env.IntOverflow)
	env.NilCheck = checkNilMode("--nil-check", env.NilCheck)
	env.CGO = env.CGO && !opts.purego
	if !slices.Contains([]string{"", "gc", "tinygo"}, env.Compiler) {
		panic(compileError{msg: fmt.Sprintf("-compiler: %q is not gc or tinygo", env.Compiler)})
	}
	if opts.purego {
		env.Compiler = ""
	}
	return env
}

// goFileSelected evaluates a prelude/go file's //go:build line: rugaroo_prelude holds, rugaroo_cgo under --cgo, rugaroo_tinygo under -compiler tinygo (decisions 426, 443).
func (c *Compiler) goFileSelected(src []byte) bool {
	for line := range strings.Lines(string(src)) {
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(strings.TrimSpace(line))
		if err != nil {
			panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
		}
		return expr.Eval(func(tag string) bool {
			return tag == "rugaroo_prelude" || tag == "rugaroo_cgo" && c.env.CGO || tag == "rugaroo_tinygo" && c.env.Compiler == "tinygo"
		})
	}
	return true
}

// cgoPreamble is the comment above a file's `import "C"`, each `#include "x.h"` replaced by prelude/go/x.h, so the
// one emitted main.go carries the C (decision 426).
func (c *Compiler) cgoPreamble(name string, src []byte) string {
	f, err := goparser.ParseFile(token.NewFileSet(), name, src, goparser.ImportsOnly|goparser.ParseComments)
	if err != nil {
		panic(compileError{msg: fmt.Sprintf("%s: %v", name, err)})
	}
	var out strings.Builder
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT || len(g.Specs) != 1 || g.Specs[0].(*ast.ImportSpec).Path.Value != `"C"` || g.Doc == nil {
			continue
		}
		for line := range strings.Lines(g.Doc.Text()) {
			hdr, ok := strings.CutPrefix(strings.TrimSpace(line), "#include \"")
			if !ok || !strings.HasSuffix(hdr, "\"") {
				out.WriteString(line)
				continue
			}
			text, err := fs.ReadFile(c.preludeFS, path.Join(path.Dir(name), strings.TrimSuffix(hdr, "\"")))
			if err != nil {
				panic(compileError{msg: fmt.Sprintf("%s: %v", name, err)})
			}
			out.Write(text)
		}
	}
	return out.String()
}

// stripGoPackage drops the file header, `package` clause and import block, keeping only what follows; line is where
// that remainder starts, for the //line directive. imports are the std packages the block named.
func stripGoPackage(src []byte) (string, int, []string, error) {
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", src, goparser.ImportsOnly)
	if err != nil {
		return "", 0, nil, fmt.Errorf("parse: %w", err)
	}
	end := f.Name.End()
	var imports []string
	for _, d := range f.Decls {
		end = d.End()
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			for _, s := range g.Specs {
				if p := strings.Trim(s.(*ast.ImportSpec).Path.Value, `"`); p != "C" { // cgo's pseudo-package: the preamble (cgoPreamble)
					imports = append(imports, p)
				}
			}
		}
	}
	off := fset.Position(end).Offset
	rest := string(src[off:])
	// emitProgram trims the leading blank lines, so the directive names the first non-blank line
	line := fset.Position(end).Line + strings.Count(rest[:len(rest)-len(strings.TrimLeft(rest, " \t\n"))], "\n")
	return rest, line, imports, nil
}

// requireRelative resolves `require_relative "x"` inside prelude file f.
func (c *Compiler) requireRelative(ctx context.Context, f *File, n *parser.CallNode) {
	args := callArgs(n)
	var str *parser.StringNode
	if len(args) == 1 {
		str, _ = args[0].(*parser.StringNode)
	}
	if str == nil {
		c.errorf(f, n, "require_relative needs a string literal")
	}
	target := path.Join(path.Dir(f.Name), str.Unescaped.Value)
	if !strings.HasSuffix(target, ".rb") {
		target += ".rb"
	}
	c.loadPrelude(ctx, target)
}

// userRequireRelative loads the file a user file's top-level
// `require_relative "x"` names (decision 130), as MRI does: by real path,
// once, its `__FILE__` that path. It returns nil when the file is already
// loaded.
func (c *Compiler) userRequireRelative(ctx context.Context, f *File, n *parser.CallNode) *File {
	args := callArgs(n)
	if len(args) != 1 {
		c.errorf(f, n, "require_relative needs a string literal")
	}
	str, ok := args[0].(*parser.StringNode)
	if !ok {
		c.errorf(f, n, "require_relative needs a string literal: rugaroo loads files at compile time")
	}
	target := relativeTarget(f, str.Unescaped.Value)
	if shadowedGemFile(target) != "" {
		return nil // the prelude lib the require scan loaded in its place
	}
	src, err := c.readUser(target)
	if err != nil {
		c.errorf(f, n, "cannot load such file -- %s", strings.TrimSuffix(target, ".rb"))
	}
	return c.loadUserFile(ctx, target, src)
}

// relativeTarget is the path `require_relative "name"` in user file f names: beside f, with `.rb` added.
func relativeTarget(f *File, name string) string {
	target := filepath.Join(filepath.Dir(f.path), filepath.FromSlash(name))
	if filepath.Ext(target) != ".rb" {
		target += ".rb"
	}
	return target
}

// userRequire loads the file a user file's top-level `require "x"` finds
// on the load path (-I, decision 131), as userRequireRelative does; ok is
// false when it names none, so the require is the prelude's to handle.
func (c *Compiler) userRequire(ctx context.Context, f *File, n *parser.CallNode) (*File, bool) {
	target := c.findRequire(f, n)
	if target == "" {
		return nil, false
	}
	src, err := c.readUser(target)
	if err != nil {
		c.errorf(f, n, "cannot load such file -- %s", strings.TrimSuffix(target, ".rb"))
	}
	return c.loadUserFile(ctx, target, src), true
}

// findRequire is the file `require "x"` names on the load path, the first
// -I directory holding x.rb, as MRI searches $LOAD_PATH; "" when none does.
func (c *Compiler) findRequire(f *File, n *parser.CallNode) string {
	if f.isPrelude() {
		return ""
	}
	args := callArgs(n)
	if len(args) != 1 {
		return ""
	}
	str, ok := args[0].(*parser.StringNode)
	if !ok {
		return ""
	}
	if t, ok := c.requireTargets[str.Unescaped.Value]; ok { // one stat per load-path dir per require was most of brew's gen syscalls
		return t
	}
	if c.requireTargets == nil {
		c.requireTargets = map[string]string{}
	}
	t := c.findRequireUncached(str.Unescaped.Value)
	c.requireTargets[str.Unescaped.Value] = t
	return t
}

func (c *Compiler) findRequireUncached(required string) string {
	name := filepath.FromSlash(required)
	if filepath.Ext(name) != ".rb" {
		name += ".rb"
	}
	dirs := c.loadPath
	first, _, _ := strings.Cut(required, "/")
	if !builtinLib(required) && !builtinLib(first) { // the prelude's lib wins over a gem's copy (base64, forwardable), as does a vendored gem's (sqlite3, its own `sqlite3/x` files too)
		dirs = append(slices.Clip(dirs), c.gemDirs...)
	} else if bundledGems[first] && preludeLibs[required] == "" && c.preludeFS != nil { // prism and prism/x, not the prism/prism backend
		dirs = append(slices.Clip(dirs), c.useBundled(first))
	}
	return resolveOnPath(dirs, name)
}

// resolveOnPath is the file name (already `.rb`-suffixed, OS separators) names in dirs: an absolute name as itself, as MRI loads it, else its first match that stays inside its dir.
func resolveOnPath(dirs []string, name string) string {
	if filepath.IsAbs(name) {
		dirs = []string{""}
	}
	for _, dir := range dirs {
		target := filepath.Join(dir, name)
		if dir != "" && !within(dir, target) { // `../x` climbs out of -I and the gem dirs
			continue
		}
		info, err := os.Stat(target) //nolint:gosec // a require path the program names, kept within -I and the gem dirs
		if err == nil && !info.IsDir() {
			return target
		}
	}
	return ""
}

// loadUserFile parses and collects a required file, keyed by real path as
// MRI's loaded features are; nil when it is already loaded.
func (c *Compiler) loadUserFile(ctx context.Context, target string, src []byte) *File {
	path := c.real(target)
	if c.loaded[path] {
		return nil
	}
	c.loaded[path] = true
	uf := c.parsed[path] // scanRequires parsed it
	if uf == nil {
		var err error
		uf, err = parseFile(ctx, c.parser, path, src, originSource)
		if err != nil {
			panic(compileError{msg: err.Error()})
		}
		uf.path = path
	}
	c.files = append(c.files, uf)
	c.userFiles = append(c.userFiles, uf)
	if c.required == nil {
		c.required = map[*File]bool{}
	}
	c.required[uf] = true
	c.collect(ctx, uf)
	return uf
}

// dumpBadOutput saves unparsable output for debugging in a fresh temp file (a fixed name follows a planted symlink and is shared by concurrent compiles) and says where, or why it could not.
func dumpBadOutput(raw []byte) string {
	f, err := os.CreateTemp("", "rugaroo-bad-output-*.go")
	if err != nil {
		return fmt.Sprintf("raw output not saved: %v", err)
	}
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Sprintf("raw output not saved to %s: %v", f.Name(), werr)
	}
	return "raw output written to " + f.Name()
}

// within reports whether p is dir or under it; a first element merely named `..foo` is inside, only `..` climbs out (ponytail: lexical, symlinks are followed as MRI's $LOAD_PATH does).
func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// safeName is name fit for a `//line` directive and a `//` comment (a newline would land the rest as Go source); File.path keeps the real path for I/O.
func safeName(name string) string {
	if !strings.ContainsFunc(name, unsafeNameRune) && utf8.ValidString(name) {
		return name
	}
	return strings.Map(func(r rune) rune {
		if unsafeNameRune(r) || r == utf8.RuneError {
			return '_'
		}
		return r
	}, name)
}

func unsafeNameRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r < 0xa0) || r == 0x2028 || r == 0x2029 || r == 0xfeff
}

// realPath is name absolute with symlinks resolved, as MRI keys loaded features; name itself when that fails.
// real is realPath, remembered for this compile: the require scan and loader resolve the same files many times.
func (c *Compiler) real(name string) string {
	if r, ok := c.realPaths[name]; ok {
		return r
	}
	if c.realPaths == nil {
		c.realPaths = map[string]string{}
	}
	r := realPath(name)
	c.realPaths[name] = r
	return r
}

func realPath(name string) string {
	p, err := filepath.Abs(name)
	if err != nil {
		return name
	}
	if r, ok := realPaths.Load(p); ok {
		return r.(string) //nolint:forcetypeassert // only strings are stored
	}
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	realPaths.Store(p, r)
	return r
}

// realPaths memoizes realPath's resolved paths for the process: a traced compile resolves each boot branch's file
// once per pass, five passes (decision 303). A path that does not resolve yet is not cached.
var realPaths sync.Map

// warn records a warning once, with its source position.
func (c *Compiler) warn(f *File, n parser.Node, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if n != nil { // a nil node's message carries its own position, as errorf's
		msg = srcPos{f, f.shownLine(n.GetLocation().StartOffset)}.String() + ": " + msg
	}
	c.addWarning(f, msg)
}

// addWarning records msg once, unless it is gem code's.
func (c *Compiler) addWarning(f *File, msg string) {
	if c.gemFile(f) && !c.env.GemWarnings { // not the user's to act on: gem code is compiled leniently (decision 173)
		return
	}
	if c.warned[msg] {
		return
	}
	c.warned[msg] = true
	c.Warnings = append(c.Warnings, msg)
}

// dropWarnings forgets the warnings after the first n, for a re-run pass.
func (c *Compiler) dropWarnings(n int) {
	for _, w := range c.Warnings[n:] {
		delete(c.warned, w)
	}
	c.Warnings = c.Warnings[:n]
}

// noteDyn asks for dynamic wrappers of a method name.
func (c *Compiler) noteDyn(name string) {
	if c.dynSeen[name] {
		return
	}
	c.dynSeen[name] = true
	c.dynNames = append(c.dynNames, name)
	c.noteDyn("method_missing")
}

// noteRespond asks for a run-time respond_to? check of a method name.
func (c *Compiler) noteRespond(name string) {
	if c.respondSeen[name] {
		return
	}
	c.respondSeen[name] = true
	c.respondNames = append(c.respondNames, name)
	c.noteDyn("respond_to_missing?")
}

// phaseTimer reports each phase's duration to stderr when RUGAROO_TIMING is set: where a compile spends its time, natively or as wasm.
func phaseTimer(timing bool) func(string) {
	if !timing {
		return func(string) {}
	}
	last := time.Now()
	return func(name string) {
		now := time.Now()
		fmt.Fprintf(os.Stderr, "rugaroo: %-8s %6dms\n", name, now.Sub(last).Milliseconds())
		last = now
	}
}

// readUser reads a user file to compile.
func (c *Compiler) readUser(target string) ([]byte, error) {
	data, err := os.ReadFile(target) //nolint:gosec // the user's program names it
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", target, err)
	}
	if c.methodRequiresRuntime && bundlerStandalone(target, data) {
		return nil, nil // its $LOAD_PATH is the trace's, resolved at compile time; the rest shims RubyGems (decision 296)
	}
	return data, nil
}

// bundlerStandalone is the setup.rb `bundle install --standalone` writes: RubyGems shims and $LOAD_PATH entries only.
func bundlerStandalone(path string, data []byte) bool {
	return strings.HasSuffix(path, "/bundler/setup.rb") && bytes.Contains(data, []byte("\n$:.unshift File.expand_path("))
}

// parseDelegators keys def_delegators' names by call site; a key with no line never matched a call.
func parseDelegators(m map[string][]string) map[boot.Loc][]string {
	out := make(map[boot.Loc][]string, len(m))
	for site, names := range m {
		if l, ok := boot.ParseLoc(site); ok {
			out[l] = names
		}
	}
	return out
}

// addPkgVar declares a package-level var once.
func (c *Compiler) addPkgVar(decl string) { c.pkgVars[decl] = true }
