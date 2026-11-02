# Test-suite time research, 2026-11-01

Branch `test-time` (worktree beside the main checkout, from main b05a97d4). Machine: 10 cores, 32 GB, Go 1.27.3, golangci-lint 2.15.0, ruby 4.0.9, Go build cache 400 GB at the start. Every number below was measured in this session with the harness's `RUGAROO_TEST_TIMING` log (timing_test.go) and `go test -json`, summarized by `scripts/test-time`. Full runs went through the machine-wide lock (`lockf -k /tmp/claude-501/rugaroo-fullsuite.lock go test -json ./... -timeout 90m`), one at a time, with the lint and experiment jobs below sometimes running beside them (noted where it matters).

## Checklist

Done (merged to main 2026-11-01, 69928ae0):

- [x] Instrumentation: `RUGAROO_TEST_TIMING` per-step log, `RUGAROO_TEST_KEEP`, `scripts/test-time` (bdd5b0e2)
- [x] Baseline measured: fresh worktree 950 s, warm 337 s, string.rb comment 1974 s, prelude/go helper comment 2396 s
- [x] L1 stable `//line` numbers in every test program, examples consecutive per prelude file; lint parity checked (decision 434, 3e200ddd)
- [x] L1b Method-key prelude offsets renumbered per file (decision 434 amended, d22b3754); TestPrimeVendoredSig pinned (37f287fc)
- [x] L2 rbs-inline + rbs validate pass cached (decision 435, 89e24772)
- [x] L5 compile-only top-level tests parallel (9f1a3b76)
- [x] `-race` cost measured on 6 paired builds (2x wall, 0-22% CPU)
- [x] `-parallel 20` measured and ruled out (46% slower)
- [x] Lint pass shown per-package incremental; cost is the every-name programs
- [x] CLAUDE.md timing line and testdata guidance corrected; memory notes updated
- [x] After: warm 232 s; string.rb comment 254 s (0 rebuilds, 8 s lint); abbrev.rb comment 248 s

To do (not started; each needs its own measurement before and after):

- [ ] L3 reach-based `-race` rule: `-race` only when goroutines start outside the prelude's always-kept helpers (51 of 54 race programs only keep `rbThreadStart`); decision amending 88; check the thread, ractor, fiber, process, timeout, queue, monitor, socket, net and WEBrick programs keep it
- [ ] L6 relative `//line` paths for `-I`, `-gem` and traced files in test programs, so worktrees share the 17 programs that now embed the checkout path (419 s builds + 490 s lint per worktree)
- [ ] L4 overlap the examples' lint pass with the compile tail (kramdown, liquid, roundhouse), or order the examples biggest-compile first; expected ≤ 60-100 s
- [ ] The b2 case (an edit to a helper the big programs keep, ~40 min): bounded by program size (decision 235) and compiler speed (docs/research/compile-time-2026-11.md); nothing harness-side left to try beyond L3/L4
- [ ] `TestStringLitsKept` flakes under full-suite load (6 of 10 runs here, 30/30 alone): pre-existing, find the shared state in internal/compiler's parallel tests
- [ ] First full run after the merge is cold once (every example's bytes changed, ~38 min measured); nothing to do, just expected

## How to reproduce

```sh
RUGAROO_TEST_TIMING=run.timing RUGAROO_TEST_KEEP=keep/ go test -json ./... -timeout 90m > run.json
scripts/test-time run.json run.timing -n 25
```

`RUGAROO_TEST_TIMING` appends one tab-separated line per step: kind (compile, rbs-inline, rbs-validate, build, mri, run, vet, lint), test name, seconds, `@` end time since the binary started, then per kind; a `build` row carries -race, cache miss (the `-x` trace shows a ` -p main ` compile only on a miss), source funcs (`func` lines of main.go, not compiled functions: TestCompileCost's nm count is 3-4x that), lines, bytes and a sha256 prefix of main.go. An `rbs-cache` row (decision 435) says hit or miss; an example that writes into its own directory would miss forever (none does: P2 hit 137 of 137). `RUGAROO_TEST_KEEP=dir` keeps every program's main.go as `<test>.go`, so two runs can be diffed.

Lint parity (decision 434), on a module of N kept examples (`writeModule`'s go.mod/go.sum, `.golangci.generated.yml` as `.golangci.yml`, `prelude/go/*.go` copied in, one directory per example with its kept main.go), real lines against renumbered (the Ruby twin of `stableLines` in consecutive mode is ten lines: for each `^(//line prelude/[^:\s]+):\d+$` set the number to that file's running block total + 1):

```sh
export GOFLAGS=-trimpath
golangci-lint cache clean; golangci-lint run --allow-parallel-runners --timeout=0 ./...   # cold, then again: warm
sed -i '' -E 's|[[:space:]]*//nolint:[a-z,]+.*$||' */main.go                              # every suppression off, both variants
golangci-lint run --allow-parallel-runners --timeout=0 ./... | sed -E 's/^[^ ]+:[0-9]+(:[0-9]+)?: //' | sort   # compare the two sorted lists
```

`-race` pairs: a fresh `GOCACHE`, the race and plain std built once from a hello program, then per kept program `go build -race -trimpath -gcflags='-l -dwarf=false' .` and the same without `-race`, each under `/usr/bin/time -p`, wiping and re-warming the cache between.

## Baseline

Full `go test ./...`, main b05a97d4 plus the timing log. "CPU" is `/usr/bin/time -l` user time of the whole run; "misses" are `go build` runs whose main package compiled (the rest only linked); "lint" is the examples' golangci-lint pass, which runs alone after the last example.

| scenario | wall | CPU user | programs built | build misses | lint pass | build-cache growth |
|---|---|---|---|---|---|---|
| 0. fresh worktree: Go cache warm from main, MRI cache cold (its key holds the checkout dir) | 950 s | 2847 s | 245 | 17 | 490 s | 1.0 GB |
| a. warm, nothing changed | 337 s | 1195 s | 245 | 0 | 6 s | 11 MB |
| b1. one comment line near the top of prelude/string.rb | 1974 s | 9355 s | 245 | 147 | 1530 s | 4.0 GB |
| b2. one comment line inside `rbDump` (prelude/go/string.go), a helper 114 of the 245 programs keep | 2396 s | 11637 s | 245 | 114 | 1804 s | 4.8 GB |
| c. one comment line in prelude/abbrev.rb | not run on main: by finding 1's mechanism it shifts `//line prelude/abbrev.rb:N` only in the 6 programs that keep abbrev, all small; on the prototype it is run P5 |
| P0. prototype (decision 434), first run: every example's bytes change once | 2297 s | 9138 s | 245 | 144 | 1735 s | one-off |
| P1. prototype, warm (rbs cache filling) | 353 s | 1175 s | 245 | 0 | 10 s | 0 |
| P2. prototype, b1's comment line in prelude/string.rb | 668 s | 1369 s | 245 | 3 | 320 s | 0.1 GB |
| P3. prototype + rbs cache warm + parallel top-level tests, nothing changed; the `-race` experiment (fresh GOCACHE, std rebuilds) ran beside it | 358 s | 976 s | 245 | 0 | 11 s | 0 |
| P4. first run after the Method-key renumbering (L1b): the 7 programs that build Method tables change once | 669 s | 1530 s | 245 | 7 | 324 s | one-off |
| P5. prototype, c's comment line in prelude/abbrev.rb (6 programs keep it) | 248 s | 930 s | 245 | 0 | 8 s | 0 |
| P6. prototype (L1, L1b, L2, L5), warm, machine otherwise idle | 232 s | 890 s | 245 | 0 | 9 s | 0 |
| P8. prototype (all levers), b1's comment line in prelude/string.rb, machine under a load average of 240-290 from other sessions (video call, sqlite jobs): every step 5x slower, TestPairingHeapGemTyped hit its 10 min boot timeout; 0 build misses, which is the result that matters | 1502 s | 1538 s | 245 | 0 | 35 s | 0 |
| P9. the same, started once the load average fell below 8, but other sessions' load came back (43-94 during the run; compile sum 3191 s against 783 s in P6): 1 miss, TestPrimeVendoredSig's first build after its pinning | 778 s | 1319 s | 245 | 1 | 18 s | 0 |
| P10. the same, started at load 4.9 (it rose to ~18 by the end; compile sum 1021 s against 783 s in P6) | 254 s | 933 s | 245 | 0 | 8 s | 0 |
| P7. P6 with `-parallel 20` | 338 s | 1023 s (+426 s sys) | 245 | 0 | 9 s | 0 |

CLAUDE.md's "~7 min warm / ~23 min after a prelude edit" is 5.6 min / 33-40 min on this day: b1 and b2 are the case that hurts.

Per-step totals (CPU seconds summed over the steps, which overlap under `-parallel 10`):

| step | 0. fresh | a. warm | b1. string.rb | b2. go helper |
|---|---|---|---|---|
| rugaroo compile (237 programs through `writeGenerated`) | 1008 | 846 | 672 | 849 |
| rbs-inline (137 examples) | 231 | 166 | 202 | 219 |
| rbs validate (137 examples) | 681 | 474 | 572 | 637 |
| go build, cache misses | 419 (17) | 0 | 883 (147) | 2467 (114) |
| go build, cache hits (link only) | 148 | 162 | 62 | 50 |
| program runs (241) | 123 | 126 | 120 | 126 |
| go vet (examples module) | 3.5 | 0.4 | 8.7 | 11.6 |
| golangci-lint (examples module, serial tail) | 490 | 6.2 | 1530 | 1804 |
| MRI (238 runs) | 76 (237 misses) | 0 (238 hits) | 0 | 0 |

Not in those rows (they compile without `writeGenerated`): TestPairingHeapGemTyped 167-199 s and TestGemCorpus 155-259 s per run, both rugaroo compiles of gem programs; TestGenericInstancesDeterministic 26-28 s (30 compiles of one program); TestPreludeWhole/TestPreludeGo 17-55 s.

Timeline of run a (seconds from the test binary's start): 0-65 package tests and the serial top-level tests; 65-327 the parallel suites (TestErrors done at 106, TestRun at 121, TestMinitest at 209, the examples at 327, the tail being kramdown/liquid/roundhouse whose compiles take 25-50 s each and sit late in the glob order); 327-334 vet + lint. In b1 the same shape with the tail at 452 s, then 1530 s of lint alone.

## Where the time goes

### 1. A prelude line shift rebuilds and re-lints every example (b1)

Decision 88 pins prelude `//line`s to `:1` in TestRun, TestMinitest and (it says) TestMulti, so a line shift changes no testdata program's bytes. The examples were left out on purpose: lint needs distinct positions to correlate `//nolint` comments. Diffing the kept main.go files of runs a and b1 (`RUGAROO_TEST_KEEP`):

- all 137 examples changed, and in each the only difference is `//line prelude/string.rb:N` lines (05_word_count: 28 directives, nothing else);
- TestMulti/basic and TestMulti/require changed the same way: TestMulti never called `stablePreludeLines` (the decision's text is wrong there), nor did TestBootAutoload, TestTracedEntry, TestTraceEval, TestTraceRun, TestGemCorpus, TestPrimeVendoredSig;
- TestMinitest/method and TestMinitest/prism changed through `rbMethodInfo{... key: "String#@prelude/string.rb:2874"}`: the method key carries a prelude byte offset, which a line shift moves (a leak of 2 programs, as decision 88's `assert_respond_to` note anticipated).

Cost of those 147 misses: 883 s of `go build` CPU (the biggest, under contention: TestGemCorpus 76 s, 30_dynamic_send 74 s, 124_liquid 57 s, 116_cuba_web 52 s, TestMinitest/prism 47 s) and a 1530 s lint pass, serial after the last example. Wall 1974 s against 337 s warm.

### 2. The lint pass is per-package incremental, and a few programs are most of it

On a 5-example module (30_dynamic_send 123k lines + 05_word_count, 109_class_eval, 107_rbs_sig, 14_uncaught), `golangci-lint run` with the generated config, `GOFLAGS=-trimpath`, cache cleaned first (beside a full run, so contended):

| state | wall |
|---|---|
| cold | 344 s |
| unchanged | 3.3 s |
| after appending a comment to 14_uncaught | 4.8 s |
| after appending a comment to 30_dynamic_send | 395 s |

So the cache works per package (decision 88's `-trimpath` fix holds), and one every-name program costs 5-6 minutes of lint CPU on its own. b2 re-linted 42 examples in 1804 s, b1 137 in 1530 s: the time is the big programs, which every prelude edit of a widely kept helper reaches.

### 3. A prelude/go helper edit reaches exactly its keepers (b2), but each miss is expensive

A comment inside `rbDump` changed 114 programs: the 114 that keep `func rbDump` in run a (42 examples, 49 minitest files, 16 testdata/run files, 7 others). Reach-scoping works as decision 88 intends. The misses cost 2467 s of build CPU, because the keepers are the big programs built with `-race` under contention: TestMinitest/object 186 s, dynamic 167 s, hash 144 s, array 116 s, control 110 s, 123_kramdown 99 s (the same programs build in 3.6, 3.3, 2.8, 2.4, 1.8, 0.7 s when the cache hits, which is the link). A comment line at the top of a prelude/go file would behave like b1: each file is emitted verbatim under one `//line prelude/go/x.go:N` naming where its body starts, so a line added above shifts N in every program keeping anything from the file.

### 4. `-race`: 54 of 245 programs, 51 of them only because Thread is reachable

`goBuild` adds `-race` when the generated Go has more than one `go` statement. Attributing the `go` statements in the 54 programs to their functions: `rbTrapSignals` (the always-on one) in all 54; `rbThreadStart` in 51; `rbStartChild` (spawn) in 6; `Timeout_Meta___Race` in 1. Only 61_timeout, 23_threads, 42_queues, 79_monitor, 77_ractor and the thread/fiber/ractor/process minitest files start goroutines in their own code; the other ~40 (every minitest file, kramdown, liquid, cuba, rack, resty, prism...) keep `Thread.new`'s helper because minitest or a gem names Thread, and never run it in the test. The cost of `-race`, paired cold builds of run a's kept programs (fresh GOCACHE with the race and plain std prebuilt, machine otherwise idle, `-trimpath -gcflags=-dwarf=false`, `-l` for the minitest ones as the harness does):

| program | -race wall | -race CPU | plain wall | plain CPU |
|---|---|---|---|---|
| TestMinitest/object | 39.7 s | 51.7 s | 19.1 s | 47.9 s |
| TestMinitest/array | 21.5 s | 28.4 s | 9.9 s | 25.8 s |
| TestMinitest/hash | 27.6 s | 36.4 s | 15.6 s | 37.7 s |
| 116_cuba_web | 29.8 s | 52.4 s | 18.8 s | 50.3 s |
| 123_kramdown | 39.9 s | 53.1 s | 15.9 s | 43.5 s |
| 66_minitest_spec | 11.7 s | 16.4 s | 7.0 s | 16.4 s |

`-race` doubles the wall of a build (it turns off the compiler's concurrent backend, decision 88) but adds only 0-22% CPU. In a full run the builds overlap ten-wide and the machine is CPU-bound, so dropping `-race` from the 40 programs that never start a goroutine would save CPU in that range, not the 2x; the 2x shows on an idle machine building one program, as `go test -run TestMinitest/object` does after an edit.

### 5. rbs-inline + rbs validate: 640-910 s of CPU per run, deterministic

Every run spends 166-231 s in rbs-inline and 474-681 s in `rbs validate` over the 137 examples (3.5-5 s each; 25_resty, 95_open_uri, 24_net_http 9 s: they load the net-http signature set). The inputs are the example's files, `sig/*.rbs`, and the rbs/rbs-inline gems Gemfile.lock pins: a pass is reproducible from those, exactly as MRI's output is (decision 272). It is 35-40% of a warm run's CPU.

### 6. MRI cache: hits on reruns; a worktree pays 76 s once

Run 0 (fresh worktree) missed all 237 MRI runs; they cost 76 s CPU in total (TestMinitest/process 3.3 s the slowest), spread over the parallel phase. Runs a, b1, b2 hit all 238. A worktree's real first-run cost is elsewhere: 17 programs embed absolute paths in `//line` directives for `-I`, `-gem` and traced files (`//line /Users/.../transpiler-test-time/examples/90_load_path/lib/units/report.rb:7`), so every worktree misses the Go build cache for 89_require_relative, 90_load_path, 123_kramdown, 124_liquid, 135_roundhouse_blog, TestRun/gem_lenient, TestGemCorpus, TestBootAutoload, the traced tests: 419 s of builds and the 490 s lint pass of run 0.

### 7. Program count

A full run compiles 237 programs through the shared path (137 examples, 53 testdata/run, 49 minitest, 2 multi, the trace/boot tests) plus 8 outside it, builds 245, runs 241 against MRI, and compiles 562 error cases (TestErrors: 41 s of wall in run a, done by 106 s). Per-program fixed costs in run a: link on a cache hit 0.66 s average (162 s total), program run 0.5 s average, MRI 0 (cached). Merging two small testdata/run files saves one link and one run, about 1.2 s of CPU; the per-program costs that matter are compile (2-50 s) and, on a miss, build and lint, which merging would not reduce (a merged program is the union of what both reach, decision 49). File count drives the suite only where the fixed costs dominate, which is no longer the case.

### 8. Parallelism and serial phases

`go test` runs the root package's tests with `-parallel` = GOMAXPROCS = 10; TestExamples, TestRun, TestMinitest and TestErrors leaves all share those 10 slots (the "slowest subtests" column of `go test -json` includes queue wait: 123_kramdown shows 258 s of which 58 s are its own steps). Two serial phases bound the wall:

- before the parallel suites start (0-65 s in run a): the sequential top-level tests run alone: TestBootAutoload (61 s in run 0, 11 s warm; `t.Setenv` forbids `t.Parallel`), TestGenericInstancesDeterministic (26 s, 30 compiles), TestGemAutoloadWithoutBoot (18 s), TestGemDefineMethodWithoutBoot, TestTraceRun (`t.Setenv`). A `t.Parallel()` test only resumes after every sequential test in the package has finished.
- after the last example: vet + lint, 6 s warm, 490-1804 s after an edit.

Measured: P6 (`-parallel` 10, the default) 232 s / 890 s user; P7 (`-parallel 20`, same state) 338 s / 1023 s user + 426 s sys, peak RSS 3.4 GB against 2.0 GB. The compile step's summed time went from 783 s to 2848 s: twenty rugaroo compiles in one process contend for the GC and for memory bandwidth, and the builds and program runs they overlap with slow down too (build 128 → 198 s, run 109 → 141 s). The default is the right setting; the remaining wall after the levers (232 s) is the compile tail (kramdown 50 s, roundhouse 28 s, liquid 25 s, object 70 s) plus ~10 s of lint, not idle slots. Ordering the examples by expected compile time (biggest first) could trim the tail by tens of seconds; not measured.

### 9. rugaroo compile is the largest CPU item of a warm run

846 s over 237 programs (123_kramdown 50 s, TestMinitest/object 70 s, prism 54 s, 135_roundhouse_blog 28 s, 124_liquid 25 s, 136_prism_parse 27 s, dynamic 32 s, control 28 s in run a), plus TestPairingHeapGemTyped 167 s and TestGemCorpus 155 s. That is the compiler's own speed (decision 430's inference cache stays off in tests by design, so every run compiles cold), the subject of docs/research/compile-time-2026-11.md, not of the harness.

### 10. Observed, not caused here

`internal/compiler.TestStringLitsKept` failed in runs a, b1 and b2 ("a kept literal is missing from rbStringLits") and passed in run 0 and 30 of 30 solo runs (`go test -run TestStringLitsKept -count=30 ./internal/compiler`): a flake under the package's parallel load, pre-existing on main b05a97d4.

## Levers

Ranked by measured gain per effort and risk. "Prototyped" levers are commits on this branch with full-run numbers below.

### L1. Stable `//line` numbers for every test program (prototyped: decision 434, commit 3e200ddd)

Gain: scenario b1 (comment line in prelude/string.rb) 668 s against 1974 s (run P2); the 147 rebuilds become 3 (TestBootAutoload, 98_method_objects, 136_prism_parse: the `rbMethodInfo` key offsets, L1b) and the lint pass 320 s, all of it 136_prism_parse (105k lines) re-linting. With L1b the run should be a warm run. Effort: 40 lines in the harness. Risk: lint positions on a failing example name a consecutive line instead of the prelude's (`rugaroo build -work` on the example gives the real one); the parity checks above found no difference in what lint reports. One-off cost: every example's bytes change once, a cold run (2297 s measured, run P0).

What it does not cover: an edit that changes kept code (b2) still rebuilds and re-lints exactly its keepers, and those are the big programs (L3, L4).

### L1b. Renumber Method keys' prelude byte offsets in test programs (prototyped: commit d22b3754, decision 434 amended)

Decisions 414/418 key a typed Method by its def so aliases compare equal, spelled `Owner#@file:<byte offset>`. The offset moves with any edit above the def, so a prelude line shift changes the programs that build a Method of a prelude def (P2's remaining 3 misses, 320 s of lint; 7 programs carry such keys). The harness maps each prelude file's distinct offsets to 1..n per program, an injective rewrite, so equality is unchanged and the bytes stop moving. A compiler-side key by def *name* was tried and reverted: `alias old_foo foo; def foo` (the alias-method-chain idiom) is two defs of one name in one file, which MRI's `==` tells apart and a name key would not.

### L2. Cache the examples' rbs-inline + rbs validate pass (prototyped: decision 435, commit 89e24772)

Gain: 640-910 s of CPU per run; with L5 it takes the warm run from 337 s / 1195 s CPU to 232 s / 890 s CPU (P6; P5, with a prelude edit nothing kept, 248 s). The two are not separable in the full runs: L5 removes ~45 s of serial work at the start, L2 ~65 s of CPU per parallel slot. Effort: 50 lines, the MRI cache's pattern. Risk: a stale pass replaying; the key holds every input (sources, sig/, Ruby, Gemfile.lock) and only passes are stored.

### L3. `-race` only for programs that start goroutines in their own code (not prototyped)

51 programs carry `-race` because `rbThreadStart` is kept, not called. RACE_PAIRED_TEXT. Rule candidate: `-race` when the user file or a gem file (not the prelude) contains a `go` statement after pruning, or when a prelude `go` statement sits in a function the user code reaches by name (Thread.new, Ractor.new, Timeout.timeout, spawn, Process.*). Needs a decision amending 88 and a check that the thread, ractor, fiber, process, timeout, queue, monitor, socket, net and WEBrick programs keep it. A missed race is a test weakened, so the rule must be reach-based, not name-based.

### L4. Lint only what changed, earlier (not prototyped)

The lint pass is per-package incremental (finding 2) but runs alone after the last example. After an edit reaching the big programs it is 1530-1804 s of a 10-core machine idle otherwise except for one golangci-lint process. Two shapes: (a) start the lint pass over the examples that are done while the slow tail (kramdown, liquid, roundhouse: 25-50 s compiles, late in the glob order) is still compiling; (b) order the examples so the biggest compile first. (a) needs the module split in two lint invocations, which doubles the fixed `go list` cost (6 s warm). Expected gain is the overlap, ≤ the tail (~60-100 s), small against L1; worth it only once L1 is in and the remaining case (b2) is what hurts.

### L5. Make the sequential top-level tests parallel (prototyped: commit 9f1a3b76)

TestGenericInstancesDeterministic, TestGemAutoloadWithoutBoot, TestGemDefineMethodWithoutBoot, TestPrimeVendoredSig: 45-50 s of compile that ran alone before the suites (finding 8). TestBootAutoload and TestTraceRun stay sequential (`t.Setenv`). Gain: part of P6's 337 → 232 s with L2 (above); in P6 the first parallel leaf starts at 1 s instead of 65 s.

### L6. Relative paths in `//line` for `-I`, `-gem` and traced files (not prototyped)

17 programs embed the checkout's absolute path and so miss the build cache and re-lint in every worktree (419 s of builds and the 490 s lint of run 0). A test-only rewrite to paths relative to the checkout in `stableLines` would make worktrees share those entries. Only worktree users pay this; the main checkout never does. Low effort, no risk to the oracle (run-time backtraces of test programs already read pinned lines).

### L7. Compiler speed (out of scope here)

rugaroo's own compile is 846-1008 s of CPU per run and the critical-path tail (kramdown 50 s, object 70 s). Decision 430's cache is off in tests by design. This is docs/research/compile-time-2026-11.md's subject; every second there is a second here, times one.

## Ruled out

- **Merging testdata programs** (item 7): a program's fixed costs on a warm run are 0.66 s link + 0.5 s run; the variable costs (compile 2-50 s, build and lint on a miss) are what a merge would add up, not remove (decision 49's pruning makes a merged program the union). CLAUDE.md's "file count, not size, drives suite time" described the pre-decision-88 world; it no longer holds for the warm or the post-edit run.
- **Pinning the examples to `:1` like testdata**: positions collide across blocks (every block starts at line 1), so an inline `//nolint` could cover an unrelated issue at the same offset in another block, the coincidence the memory note on generated-code nolint already recorded for gosec. Consecutive numbering keeps uniqueness at the same cache behaviour.
- **Skipping lint or vet on unchanged examples by hand**: golangci-lint already does this per package (finding 2); the pass is 6 s when nothing changed.
- **A smaller lint set for generated code**: every linter in `.golangci.generated.yml` is there by decision (371 for nilness); the cost is the big programs' size, which is decision 81/235's classes × names problem, not the linter list.
- **More test parallelism**: `-parallel 20` is 46% slower than the default 10 (P7 against P6) and uses 1.7x the memory; the suite is CPU-bound in rugaroo's own compile, so more slots only add GC and cache contention.

## Recommendation

1. Merge L1 (decision 434) and L2 (decision 435) and L5 as they stand: together they make the first run after a prelude .rb line shift the same as a warm run, and take ~300 s of CPU and ~100 s of wall off every run. Goal, measured on this branch: comment line in a core prelude file 1974 s → 668 s measured (P2), → 254 s with L1b in (P10: 0 rebuilds, 8 s lint; P8/P9 ran the same scenario under other sessions' load, 1502 s and 778 s, 0 and 1 rebuilds); warm 337 s → 232 s (P6).
2. Then L3 with its own decision (amending 88): the remaining expensive case is an edit that changes a widely kept helper (b2: 2396 s), whose cost is 114 big `-race` builds plus their lint; `-race` only where goroutines run halves the wall of each such build on an idle machine (0-22% of its CPU in a saturated full run) and leaves lint, which L4 can overlap.
3. L6 when more than one worktree is in use (the user's workflow today).
4. The rest of the post-edit cost is program size (decision 235) and compiler speed (L7): track them in the compile-time research, not here.
