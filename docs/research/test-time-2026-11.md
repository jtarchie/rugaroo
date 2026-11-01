# Test-suite time research, 2026-11-01

Branch `test-time` (worktree beside the main checkout, from main b05a97d4). Machine: 10 cores, 32 GB, Go 1.27.3, golangci-lint 2.15.0, ruby 4.0.9, Go build cache 400 GB at the start. Every number below was measured in this session with the harness's `RUGAROO_TEST_TIMING` log (timing_test.go) and `go test -json`, summarized by `scripts/test-time`. Full runs went through the machine-wide lock (`lockf -k /tmp/claude-501/rugaroo-fullsuite.lock go test -json ./... -timeout 90m`), one at a time, with the lint and experiment jobs below sometimes running beside them (noted where it matters).

## How to reproduce

```sh
RUGAROO_TEST_TIMING=run.timing RUGAROO_TEST_KEEP=keep/ go test -json ./... -timeout 90m > run.json
scripts/test-time run.json run.timing -n 25
```

`RUGAROO_TEST_TIMING` appends one tab-separated line per step: kind (compile, rbs-inline, rbs-validate, build, mri, run, vet, lint), test name, seconds, `@` end time since the binary started, then per kind; a `build` row carries -race, cache miss (the `-x` trace shows a ` -p main ` compile only on a miss), source funcs, lines, bytes and a sha256 prefix of main.go. `RUGAROO_TEST_KEEP=dir` keeps every program's main.go as `<test>.go`, so two runs can be diffed.

## Baseline

Full `go test ./...`, main b05a97d4 plus the timing log. "CPU" is `/usr/bin/time -l` user time of the whole run; "misses" are `go build` runs whose main package compiled (the rest only linked); "lint" is the examples' golangci-lint pass, which runs alone after the last example.

| scenario | wall | CPU user | programs built | build misses | lint pass | build-cache growth |
|---|---|---|---|---|---|---|
| 0. fresh worktree: Go cache warm from main, MRI cache cold (its key holds the checkout dir) | 950 s | 2847 s | 245 | 17 | 490 s | 1.0 GB |
| a. warm, nothing changed | 337 s | 1195 s | 245 | 0 | 6 s | 11 MB |
| b1. one comment line near the top of prelude/string.rb | 1974 s | 9355 s | 245 | 147 | 1530 s | 4.0 GB |
| b2. one comment line inside `rbDump` (prelude/go/string.go), a helper 114 of the 245 programs keep | 2396 s | 11637 s | 245 | 114 | 1804 s | 4.8 GB |
| c. one comment line in prelude/abbrev.rb | SCENARIO_C |

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

`goBuild` adds `-race` when the generated Go has more than one `go` statement. Attributing the `go` statements in the 54 programs to their functions: `rbTrapSignals` (the always-on one) in all 54; `rbThreadStart` in 51; `rbStartChild` (spawn) in 6; `Timeout_Meta___Race` in 1. Only 61_timeout, 23_threads, 42_queues, 79_monitor, 77_ractor and the thread/fiber/ractor/process minitest files start goroutines in their own code; the other ~40 (every minitest file, kramdown, liquid, cuba, rack, resty, prism...) keep `Thread.new`'s helper because minitest or a gem names Thread, and never run it in the test. The cost of `-race` per program: RACE_PAIRED

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

PARALLEL_MEASURED

### 9. rugaroo compile is the largest CPU item of a warm run

846 s over 237 programs (123_kramdown 50 s, TestMinitest/object 70 s, prism 54 s, 135_roundhouse_blog 28 s, 124_liquid 25 s, 136_prism_parse 27 s, dynamic 32 s, control 28 s in run a), plus TestPairingHeapGemTyped 167 s and TestGemCorpus 155 s. That is the compiler's own speed (decision 430's inference cache stays off in tests by design, so every run compiles cold), the subject of docs/research/compile-time-2026-11.md, not of the harness.

### 10. Observed, not caused here

`internal/compiler.TestStringLitsKept` failed in runs a, b1 and b2 ("a kept literal is missing from rbStringLits") and passed in run 0 and 30 of 30 solo runs (`go test -run TestStringLitsKept -count=30 ./internal/compiler`): a flake under the package's parallel load, pre-existing on main b05a97d4.

## Levers

LEVERS

## Ruled out

RULED_OUT

## Recommendation

RECOMMENDATION
