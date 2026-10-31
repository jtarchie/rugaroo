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

## Lever list, ranked (gain ÷ effort ÷ risk)

| # | lever | evidence | expected | effort | risk |
|---|---|---|---|---|---|
| 1 | memoize `attrTarget` scan | measured | -43% tokenize gen (done) | tiny | none: keyed on `len(classList)` because `metaFor` (metaclass.go:52) and `declare` append during gen; outputs byte-identical on tokenize, 123, 124 |
| 2 | persist `Inference` (param types from use, decision 146) on disk keyed by source+gem hashes+compiler version; rerun = 1 pass, no rounds | rounds = 60% of gen after #1; `Inference` already carries types between compiles in-process ("runs no rounds while they still cover every untyped parameter") | warm gen ~79 → ~25 s (**estimate: the in-process second-compile bound was not measured**); no gain cold | medium (Type serialization) | stale cache on compiler change → key on binary build ID |
| 3 | rounds re-gen only methods whose pending-param inputs changed (dirty set per round) | measured above: rounds 4-8 move ≤3 keys but cost 4.5-6.6 s each (~26 s) | ~-20 s cold on tokenize | high (probe results feed each other) | medium: inference order determinism (decision 146) |
| 4 | the subagent's code-read hot spots (below), after profiling each: #3 typeAbstracts, #4/5 pruneGemMethods fixpoint + computeLive, #10 realPath negative cache | read only, **not profiled**; none shows in the top flat list after fix #1 | small each; larger at rubocop scale (10k classes) | small | low |
| 5 | emit-side: dynWrappers D×C codegen; refineIvars dry-runs (`c.out.String()` copy per try) | code read only | unknown | medium | low |
| 6 | split output into packages | **not done** (see below) | go build 65 s is ~47% of the full build | high | high: closed-world dispatch crosses packages |

Not done: memory profile / `-memprofile` (item 2 beyond GOGC), `go tool nm` dead-function reach and census (item 3), per-gem partition prototype (item 4), per-file IR cache (item 5 beyond #2), gem-lane pruned/stub generation share (item 8), liquid and kramdown baselines, and the rubocop CLI at 54 min. The 65 s `go build` half of the budget is unprofiled.

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
4. `go build` (65 s here, ~50 min at rubocop scale) needs its own profile (`-gcflags=-c=N`, census, `nm` reach) before choosing between pruning more and partitioning.
