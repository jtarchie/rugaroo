# Compile-time research, 2026-10-31

Branch `compile-time` (worktree beside the main checkout). Machine: M-series Mac, 10 cores. Every number below was measured in this session; "not done" marks what was not.

## Reproduction

Tokenize program (`main.rb`): `require "prism"; require "parser"; require "rubocop-ast"` then `ProcessedSource.new(src, 3.4, parser_engine: :parser_prism).tokens.size`.
Gems on `-I`: rubocop-ast 1.48.0 (a **patched copy**, below), parser 3.3.10.0, ast 2.4.3, racc 1.8.1; prism is bundled (decision 427).

```sh
RUGAROO_TIMING=1 RUGAROO_CPUPROFILE=gen.prof rugaroo gen -I <rubocop-ast>/lib -I <parser>/lib -I <ast>/lib -I <racc>/lib -trace main.rb > main.go
go tool pprof -top -cum rugaroo gen.prof
```

The unmodified rubocop-ast 1.48 does not compile on main (4f61f0f2): the research branch's gem rewrites were lost with its scratchpad. Needed so far (each a general compiler gap, not fixed here):
1. `collection_node.rb`: `def_delegators :to_a, *ARRAY_METHODS` (`forwardable.go:37` wants literal symbols) → expand the list literally; and the accessor `to_a` is not a method of the mixin → add `def to_a = children`.
2. `casgn_node.rb`: `alias name short_name` (alias of a mixin method) → `def name = short_name`; same for `lhs`. Other alias sites may follow.

## Baseline (tokenize, main 4f61f0f2, no profiling overhead where stated)

| step | wall | notes |
|---|---|---|
| `gen -trace` (profiled run) | 139 s | 89 s CPU samples in 139 s wall |
| output | 23.1 MB, 516,896 lines, 46,546 funcs | one package |
| compile passes | 10 | 1 pre-pass, 8 inference rounds (hits `maxInferRounds` = 8), 1 final |
| inference rounds | ~91 s of 139 s | 8-17 s each before the fix, plus 1-4 s setup per pass (prelude 0.5-3.5 s, user 0.1-0.9 s, link 0.2-2 s) |
| format (gofmt) | 6.4 s | gocheck 8.3 s |
| `build -trace` after fix | 138 s total | ~72 s gen + ~65 s `go build`; 81.7 MB binary |

## Three sizes, main vs fixed (`gen -trace`, wall, one run each, output compared with the random `rugaroo-entry-N` tmpdir name normalized)

| program | main | fixed | funcs | output |
|---|---|---|---|---|
| 124_liquid | 12 s | 10 s | 14,482 | identical |
| 123_kramdown | 40 s | 36 s | 10,210 | identical |
| tokenize | 139 s (profiled) | 79 s profiled, 64 s unprofiled | 46,546 | identical |

TestCompileCost (05 / 64 / 116): 2065 / 7950 / 30491 compiled before and after.

## Profile findings

1. **`attrTarget` scanned every class per call** (expr_stringbang.go): for an untyped receiver of `<<`/`gsub!`, `slices.ContainsFunc(c.classList, … lookup(name+"=") … lookup(name))`, per call, per round. 23.5 s of 89 s CPU (26%). The answer depends only on the name. **Fixed** (commit "memoize attrTarget…"): gen 139 s → 79 s profiled (64 s unprofiled), main.go byte-identical (`cmp`), `TestCompileCost` unchanged (2065 / 7950 / 30491 compiled; the same on main: cuba's 30,491 vs 30,379 in decision 427 predates this branch).
2. After the fix the profile is flat: no leaf over 7% (madvise 6.3%, memclr 6.3% = allocation; map access 6.5%, walkNode 7.7%). Inclusive: `inferRounds` 34%, `collectUses` 18%, `emitMethod` 21%, `numericMix` (speculative gen via `attempt`) 17%, `probe` nested inside `try`: each expression is generated more than once (probe, then real).
3. After the fix the 8 rounds are still ~46 s of 79 s (60%). `RUGAROO_INFER_DEBUG=1` diff of consecutive rounds' inferred sets (keys / new / retyped): r1 328; r2 369 / 47 / 9; r3 396 / 29 / 4; r4 410 / 15 / 2; r5 416 / 11 / 3; r6 415 / 2 / 1; r7 416 / 3 / 0; r8 415 / 0 / 0. So the types are stable by round 6-7 and round 8 changes nothing, yet each of rounds 4-8 regenerates every body (4.5-6.6 s) to move 0-3 keys. One oscillation: `Parser_Lexer#e` Integer? ↔ Integer in rounds 3-5. The cap (8) is reached only because a key set still differs by one at round 8; not a convergence bug.
4. **GC tuning is not a lever**: unprofiled gen, default 64 s / GOGC=400 70 s (1.7 GB RSS) / GOGC=800 69 s (2.4 GB). Output identical.

## `go build` of the tokenize output (module kept by `build -work`)

- `go build -trimpath -gcflags=-c=N` of main (std cached; includes link; 81 MB): c=1 88.7 s, c=4 52.7 s, c=8 53.4 s. The default already sits on the plateau, so backend concurrency is exhausted; the remainder is the serial frontend (typecheck/escape/SSA of one 23 MB package) plus link. The earlier full-build figure (~65 s) was with a cold main package.
- Census (`scripts/census`), 46,546 funcs: free.plain 11,892; Dyn wrappers 10,743 (23%), but only **3,549 distinct bodies** after normalizing the receiver (7,194 redundant, 15% of all funcs); method.body 9,166; forwarders 6,740 (14%; 1,053 to `Kernel_Loop`, 868 to `BasicObject_Op_eq`, 601 to `Module_Include`); const-return 3,952; rbDyn dispatchers 2,219 (42k lines); generic 1,093; pruned stubs 741. Interface methods: 10,373.
- By name match against `go tool nm` of the linked binary: 40,140 of 46,526 source funcs appear (free funcs 9,223 of 12,985, methods 19,423 of 21,900, Dyn wrappers 9,275 of 9,422). The ~6.4k absent are linker-dead **or inlined away everywhere**: this does not prove them dead, and I did not separate the two.

## Where `go build` time goes, and splitting main.go

`go build -gcflags=-bench=f` on the tokenize module (total 53.6 s for the compile of `main`): frontend 20.3 s, all serial (parse+typecheck 8.6, devirtualize+inline 8.6, escape 3.0); backend 33.4 s (compilefuncs 28.0 at the default `-c`, dumpobj 5.3 serial). It compiled 147,149 funcs for 46.5k source funcs (instantiations, closures and generated wrappers; not broken down).

- `-c` is function-level backend concurrency **inside one package**. Splitting main.go into several *files* of the same package changes nothing: the compiler sees one package either way, and `-c=4` = `-c=8` shows the backend is already saturated. Only splitting into *packages* lets `go build` compile units in parallel (frontend included). Unproven upper bound: with ~4 balanced packages and 10 cores, wall time could approach the largest package plus link; not prototyped, no measured number.
- What a package split needs (reasoned from decisions 206/212/213, not tried): an acyclic layering (prelude, gems, user) while today every dispatcher and `Dyn` wrapper type-switches over every class, so those closed-world switches could only live in the top package, and lower layers would reach user classes through Go interfaces or init-time hooks (function values, which sits next to the no-map-dispatch constraint and needs the user's OK); exported (capitalized) names across packages; generic free funcs `Owner_Name[Self]` instantiated across the boundary; decision 49 prunes the prelude per program, so a shared cached prelude package needs pruning by visibility instead; and `rugaroo gen > main.go` building with go.mod alone (a stated goal) becomes a directory of files.

## Package split: the dependency graph says no (measured)

`docs/research/scc` (own module; `cd <generated module> && CONTRACT=1 [CUT=rbDyn] scc main.go`) typechecks the generated main.go with go/types (0 errors) and builds the graph of top-level funcs, methods, types and vars (an edge = a reference), with each type and its methods contracted into one node (Go requires them in one package), then runs Tarjan. Tokenize output:

| graph | nodes | largest cycle (SCC) | lines in it |
|---|---|---|---|
| funcs only, methods separate | 64,199 | 219 | n/a |
| types+methods contracted | 31,768 | 15,584 (49% of nodes; includes all 2,219 `rbDyn*`) | 386,595 of 425,782 (**91%**) |
| same, edges into `rbDyn*` cut (dispatchers hoisted to a top package) | 31,768 | 13,126 | 341,837 (**80%**) |

So the classes, their methods and the dispatchers form one strongly connected core holding 80-91% of the code; a package must hold a whole SCC. Hoisting the dispatchers moves at most ~11% of the lines out; the rest of the cycle is the object model itself (core classes calling each other directly). At most ~20% of the code could go to other packages, so the achievable parallel gain is small (a few seconds of the 20 s serial frontend), far below what the added dispatch indirection and the lost single-file output would cost. Breaking the core cycle means replacing direct method calls between classes with interface/hook calls across layers, which is the redesign decisions 206/212/213 avoided. **For build parallelism: drop the package-split lever. For build-cache reuse across edits, see the next subsection: not rejected, but a design decision first.**

### The cache argument for layered packages (user, 2026-10-31)

Go caches builds per package, so a prelude package (and gem packages) that did not change would not rebuild when only user code does, and a prelude shared across programs would build once. That is a different benefit from the parallelism measured above, and the SCC table does not rule it out by itself. What `ORIGIN=1 scc main.go` measures on the tokenize output (origin = the `//line` file of the declaration; prelude = `prelude/*`):

- Prelude declarations are 32,693 nodes / 255,955 lines (60% of lines); gem/user 17,396 nodes / 169,827 lines.
- **3,019 prelude declarations (9%) reference gem/user declarations** (11,966 func edges, 21,248 type edges, 707 var edges; 7,655 distinct targets, led by `AST_NodeI`, `Prism_*`, `RuboCop_AST_*`). These are the closed-world pieces: Kernel/Object methods whose type-switch arms name every program class (decisions 212/213), forwarders, `Dyn` wrappers. A lower package cannot hold them; they would move to the top package or become interface calls.
- **624 gem/user methods are declared on prelude-origin types** (a gem reopening a core class, and generated wrappers). Go forbids methods on another package's type, so those become free funcs plus dispatch.
- The prelude is pruned per program (decision 49) and specialized (377), so its package text differs program to program; a shared cached prelude needs a program-independent text. `RUGAROO_NO_PRUNE=1` is a debugging mode that does not build (hello world: 86,940 lines, 9,928 funcs, `undefined: BigDecimal` from library-gated types), so the one-time cost of a full core prelude is **unmeasured**.

So the cache benefit is real but needs: a program-independent prelude text (or a monotone pruning), the closed-world dispatchers hoisted to the top package, interface dispatch (allowed by the compiled-dispatch constraint, but it adds methods per class, which is what 212/213 avoided: forwarders are 6,740 funcs today), and reopened core types as free funcs. That is a design (a decision), not a prototype-sized change, and it gives an incremental-rebuild win (user edit rebuilds only the top package), not a cold-build win. Gems cache the same way once the dispatchers sit above them.

## Dead-code elimination before emit (Go's analysis)

`golang.org/x/tools/cmd/deadcode` (RTA, run from the scratchpad; not a repo dependency) on the module: 1,035 of 46.5k funcs unreachable (2.2%), 19 s to run. The compiler's own reach pruning (decision 49, reach.go) already removes the rest; the ~6.4k funcs missing from the linked binary are mostly inlined away, not dead. Typechecking the output before writing it costs about the frontend time again, for ~2% fewer functions. Not worth adopting; stdlib `go/types` would do the same job without the dependency if it ever is.

## Lever list, ranked (gain ÷ effort ÷ risk)

| # | lever | evidence | expected | effort | risk |
|---|---|---|---|---|---|
| 1 | memoize `attrTarget` scan | measured | -43% tokenize gen (done) | tiny | none: keyed on `len(classList)` because `metaFor` (metaclass.go:52) and `declare` append during gen; outputs byte-identical on tokenize, 123, 124 |
| 2 | persist `Inference` (param types from use, decision 146) on disk keyed by source+gem hashes+compiler version; rerun = 1 pass, no rounds | rounds = 60% of gen after #1; `Inference` already carries types between compiles in-process ("runs no rounds while they still cover every untyped parameter") | warm gen ~79 → ~25 s (**estimate: the in-process second-compile bound was not measured**); no gain cold | medium (Type serialization) | stale cache on compiler change → key on binary build ID |
| 3 | rounds re-gen only methods whose pending-param inputs changed (dirty set per round) | measured above: rounds 4-8 move ≤3 keys but cost 4.5-6.6 s each (~26 s) | ~-20 s cold on tokenize | high (probe results feed each other) | medium: inference order determinism (decision 146) |
| 4 | the subagent's code-read hot spots (below), after profiling each: #3 typeAbstracts, #4/5 pruneGemMethods fixpoint + computeLive, #10 realPath negative cache | read only, **not profiled**; none shows in the top flat list after fix #1 | small each; larger at rubocop scale (10k classes) | small | low |
| 5 | emit-side: dynWrappers D×C codegen; refineIvars dry-runs (`c.out.String()` copy per try) | code read only | unknown | medium | low |
| 5b | share Dyn-wrapper bodies across receivers (3,549 distinct of 10,743) as a free func the per-class method forwards to | census above | up to ~7k fewer function bodies (15%); build time ∝ funcs (decision 235) | medium | runtime: one extra call per dynamic dispatch unless inlined; needs A/B on benchmarks (memory: no-speed-for-compile-time) |
| 6 | split output into packages | parallelism: 80-91% of lines in one cycle, <= ~20% movable: rejected. Cache reuse: 9% of prelude decls point up, 624 reopened-type methods (above) | incremental rebuild win only; cold build unchanged | very high | needs a decision and a prototype of the hoisted dispatchers |
| 7 | pre-emit dead-code elimination via RTA | 2.2% unreachable (above) | ~2% of funcs | medium | rejected |

Not done: memory profile / `-memprofile` (item 2 beyond GOGC), per-gem partition prototype (item 4), per-file IR cache (item 5 beyond #2), gem-lane pruned/stub generation share (item 8), liquid and kramdown baselines, and the rubocop CLI at 54 min. The 65 s `go build` half of the budget is unprofiled.

Reasoning on partitioning (unmeasured): Go parallelizes across packages, and within one package only the backend (`-c`). Every class's type switches (decisions 212/213) name every other class, so a gem package would need the user/prelude types exported and the dispatchers registered at init, which is the map-based dispatch the user ruled out (memory: compiled-dispatch-no-maps). A prelude-only package (compiled once, cached across programs) conflicts with decision 49's per-program pruning. Needs a prototype before any claim.

## Code-read hot spots (subagent, **estimates, not profiled**)

Ranked by likely cost: `refineIvars`/`dryRunIvarUsers` (gen_ivars.go:17-115, up to 8 rounds × 2 `discoverIvars` passes, `c.out.String()` copy per try); `dynWrappers` D×C codegen before `dynShareable` dedupe (dynamic_wrapper.go:111); `typeAbstracts` classList × methods × classList with uncached `ancestors()` (signatures.go:820); `pruneGemMethods` fixpoint R × defs × `inherits()` (reach.go:112) and `computeLive` re-resolving refs each round (reach.go:282); `neededAutoloads` fixpoint (boot.go:146); `relinkModuleOverrides` (overrides.go:225); `ancestors()` uncached (link.go); `realPath` negative results uncached (compiler.go:1168); `os.Stat` per require candidate × load path (traced.go:98). The profile above shows none of these in the top flat entries on the tokenize program, so verify with a profile of the rubocop CLI (n ≈ 10k classes) before fixing.

## Literature (subagent; "read" = page fetched, else unverified)

- **Crystal**: transfers: cache per-module results, invalidate by dependency (the shape of lever 2). Does not: inference depends on calling context, and release time is mostly LLVM. 2023-24 work not found.
- **Sorbet**: transfers: parse/collect files in parallel, merge sequentially (rugaroo's parse and collect are serial; read: namer-resolver-pipeline.md). Does not: it never generates code.
- **TypeScript**: transfers: project references/`tsc -b` rebuild only out-of-date units via emitted `.d.ts` (read). Does not: boundaries are hand-drawn; `--incremental` details unverified.
- **Nim**: transfers: per-module cache with an interface cookie so private-body edits do not cascade (read). Does not: its own IC is slower cold (1m39 vs 1m00 on a 32-core build) and the cache is large.
- **OCaml/MLton**: transfers: `.cmi` interface files and `-opaque` cut recompiles (read). MLton half unverified; whole-program cost grows with the program, rugaroo's position.
- **Go**: transfers: export data lets dependents skip implementations (read). Does not: GC-shape stenciling is a runtime-cost design (read).
- **Opal**: transfers: per-file output depends on that file only. Does not: no type inference to learn from.

## Recommendation

1. Merge the `attrTarget` memo now (done, no decision needed: output unchanged).
2. Next: lever 2, the on-disk `Inference` cache. It is the only lever whose gain is measured-bounded (rounds 60% of gen) and cheap to reason about; needs a decision (cache key = source and gem file hashes + compiler build ID; a miss is the current path). Goal: tokenize warm gen ≤ 25 s.
3. Then profile the **rubocop CLI** gen (54 min) once, to see whether the code-read hot spots (levers 4-5) dominate at n = 10k classes; on tokenize they do not. Goal there: under 10 min needs lever 3 or cache hits.
4. `go build`: backend concurrency is exhausted (-c=4 = -c=8), so only fewer/smaller functions or a package split help. First prototype lever 5b (Dyn bodies) behind a flag and A/B the benchmarks; partitioning stays unproven.

## Rubocop CLI gen profile (2026-10-31, main 3a2aa775 + attrTarget memo)

Repro: `docs/research/rubocop/run.sh <rugaroo>` (rubocop 1.92.1, rubocop-ast 1.50.0, regexp_parser 2.13.1; patches: `Struct.new(*TOKEN_KEYS)` and an alias of a class_eval'd method in regexp_parser, the two tokenize rubocop-ast rewrites, three `define_predicate ..., child_index: 1..-2` Range captures (decision 198), and the Psych tree API (`YAMLDuplicationChecker` stubbed, config loaded with `YAML.safe_load(permitted_classes: [Regexp, Symbol], aliases: true)`); `main.rb` is exe/rubocop's non-server path, since exe/rubocop stops at `RuboCop::Server::CLI`, an autoload named only from the main file (compiler bug, not filed). MRI on the patched copies prints the unpatched result: "1 file inspected, 1 offense detected, 1 offense autocorrectable".

| measure | value |
|---|---|
| gen wall | 6 m 28 s unprofiled (was 54 min on 10-31 before the attrTarget memo; the pre-memo binary was not re-run on this repro) |
| output | 86 MB, 1,792,805 lines, 168,868 funcs |
| passes | 10 (1 pre-pass, 8 inference rounds at ~35 s each = 284 s, 73%; final infer 21 s, format 28 s, gocheck 14 s) |

CPU profile: flat (top leaf 3.6%, map access). Stacks are deeper than the profiler keeps (recursive gen), so only ~35% of samples reach `main`; use the phase timer for the split. Inclusive: `inferRounds` 29%, `emitMethod` 19%, **`refineIvars` / `dryRunIvarUsers` 16-17%**, `collectUses` 8%, `link` 4%. go/format 1%.

**`refineIvars` is ~190 s of 407 s (47%)** (instrumented run, uncommitted): it runs once per pass (9 passes), each time 12-15 container-ivar candidates and 26-32 dry-runs of 17-19 classes / ~1,000 methods (0.6-1.0 s each, since every try also resets every inferred return program-wide). Tries = 2n+2 in every pass: the all-typings try fails, every all-but-one try fails (there are three culprits, the same in every pass: `Racc::Parser#@racc_tstack`, `Parser::CurrentArgStack#@stack`, `RuboCop::FilePatterns#@strings`), then the one-at-a-time phase decides. The accepted set is the same from pass 2 to 8. Levers, none built yet: (a) exact: none cheap found (tries mutate shared state, so no parallelism; the culprit count is only known by running); (b) behind a flag, empirically checked: in rounds, seed the greedy phase with the previous pass's accepted set and skip the all-but-one phase when the previous pass had at least two culprits among the same candidates; the final pass keeps the full search. Bound: ~150 s of 407 s.

**Determinism (fixed, ec3acb46):** two `gen -trace` runs of one program differed in `rugaroo-boot-<random>` (the boot hook's `-I` temp dir, recorded in `$LOAD_PATH`) and `rugaroo-entry-<random>` (the synthetic entry file, in the runtime require table), against decision 88, so a regen of an unchanged program always missed the Go build cache. Hooks now load by path (`-r /abs/hook.rb`), and the entry lives at a content-addressed path under the user cache dir. `TestTraceDeterministic` (fixture `testdata/traced_det`) fails without the fix and passes with it; traced examples 78/123/124 still match MRI.

## Inference cache, measured (decision 430, d5025fea)

`gen -trace`, wall, output compared byte for byte:

| program | cold (cache saved) | hit | rounds on hit | output |
|---|---|---|---|---|
| 124 liquid | 8 s | 4 s | 0 | identical |
| 123 kramdown | 27 s | 10 s | 0 | identical |
| rubocop (`RUBOCOP_WORK=dir docs/research/rubocop/run.sh`) | 3 m 37 s | **41 s** | 0 | identical |

The rubocop hit: infer 15 s (nearly all `refineIvars`), format 10.6 s, gocheck 8.7 s, emit 2.6 s, trace+boot 2 s. This repro compiles a smaller program than the scratchpad run above (44.7 MB and 993 cached keys against 86 MB and 1,612; it runs from a different directory, so rubocop resolves its config and cop set differently, not investigated), so its cold time is not comparable to the 6 m 28 s there; the cold/hit ratio is.

Partial reuse (drop a changed file's keys, run the rounds from the rest) was built and rejected: liquid with one line added to main.rb converged in 3 rounds instead of 8 (5 s) but typed `Liquid::ResourceLimits#render_length` untyped where cold types it `Integer?`. The rounds are not confluent, so any changed input compiles cold. TestCompileCost: 2065 / 7950 / 30491, unchanged (tests never enable the cache).
