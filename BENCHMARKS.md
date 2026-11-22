# Benchmarks

Wall-clock times for the workloads in [`benchmarks/`](benchmarks/), measured
on one Apple-silicon machine (MRI 4.0.9, Crystal 1.22.1, Go 1.27.4, spinel `14f3ade5`) on 2026-11-22, at commit 6ff749a1 (decisions 469-487). Each cell is the median of 9 runs.
To reproduce:

```sh
RUNS=9 MARKDOWN=1 scripts/benchmark          # this table; SPINEL=/path/to/spinel adds that column
COLS=ruby,rugaroo scripts/benchmark fib array  # some columns, some workloads
```

| workload | MRI | MRI+YJIT | rugaroo | spinel | Crystal | Go |
|---|---:|---:|---:|---:|---:|---:|
| `fib(32)` | 0.26s | 0.091s | 0.015s | 0.014s | 0.017s | 0.014s |
| `Hash` counting, 500k | 0.11s | 0.097s | 0.012s | 0.007s | 0.013s | 0.009s |
| `Array#map`/`select`/`sum`, 1M | 0.15s | 0.13s | 0.017s | 0.010s | 0.012s | 0.011s |
| string build/split, 100k | 0.12s | 0.12s | 0.021s | 0.020s | 0.027s | 0.019s |
| `Math.sin`/`cos`, 200k | 0.087s | 0.072s | 0.009s | 0.006s | 0.008s | 0.007s |
| classes, polymorphic calls, Comparable | 0.26s | 0.17s | 0.024s | - | 0.015s | 0.012s |
| binary trees (allocation), depth 10 | 0.083s | 0.068s | 0.009s | 0.008s | 0.010s | 0.007s |
| splay tree | 0.22s | 0.12s | 0.014s | 0.019s | 0.019s | 0.012s |
| blocks, procs, lambdas, enumerator chains | 0.36s | 0.33s | 0.036s | 0.12s | 0.037s | 0.012s |
| raise/rescue/retry, catch/throw | 0.28s | 0.25s | 0.11s | 0.076s | 2.51s | 0.018s |
| regexp match/gsub/scan/tr, StringScanner | 0.75s | 0.72s | 0.20s | 0.38s | 0.20s | 0.28s |
| sort_by/group_by/tally, Set, Struct | 0.57s | 0.54s | 0.11s | 0.33s | 0.054s | 0.070s |
| format, StringIO, case mapping | 0.49s | 0.46s | 0.13s | 0.21s | 0.14s | 0.10s |
| JSON generate + parse | 0.14s | 0.15s | 0.11s | 0.090s | 0.13s | 0.25s |
| Bignum: Fibonacci, 2000!, 1,200 digits of pi | 0.21s | 0.21s | 0.058s | 0.12s | 0.082s | 0.037s |
| pairing_heap gem: Dijkstra + heapsort | 0.67s | 0.35s | 0.091s | - | 0.039s | 0.036s |
| liquid gem: a template rendered 400 times | 0.38s | 0.30s | 0.17s | - | - | - |
| kramdown gem: a 20-section document to HTML, 30 times | 0.46s | 0.59s | 0.40s | - | - | - |
| total (rows each one ran) | 5.60s | 4.77s | 1.54s | 1.41s | 3.31s | 0.89s |
| vs MRI on those rows | 1.0x | 1.2x | 3.6x | 2.7x | 1.4x | 5.3x |

Re-measured on 2026-11-22 at commit 6ff749a1 (decisions 469-487) with spinel rebuilt from its head that day (`14f3ade5`): rugaroo's total is 1.54s against 1.54s and its rows are within noise of the 2026-11-16 ones, as expected: that work went to transpile time, generated-code size and gem programs, not to these workloads. spinel's regexp row is 0.81s → 0.38s on its newer source, which takes its total from 1.85s to 1.41s (2.0x → 2.7x MRI).
Re-measured on 2026-11-16 with decisions 463-468 (a heap floor of about 32 MiB, cached facts of long Strings, typed
`unpack`, `class` of an untyped value by class ID) and a spinel built on this machine: against the 2026-11-13 table
rugaroo's objects row is 0.029s → 0.023s, collections 0.12s → 0.11s, json 0.12s → 0.11s, bignum 0.066s → 0.058s,
pairing_heap 0.097s → 0.091s, kramdown 0.42s → 0.40s. Wall time on rows this short is mostly start-up; in instructions
retired the same change is −10.7% over all 24 workloads (objects −40%, strings −35%, json −27%, collections −22%;
`docs/research/runtime-speed-2026-11.md`).
Re-measured on 2026-10-29 after decisions 403-407 and a rubocop pass over the Ruby sources, with no performance work aimed at
these rows: rugaroo's total is 1.55s against 1.53s and MRI's 5.58s against 5.50s, so the 3.6x holds and the rows are within noise.
Re-measured on 2026-11-13 at commit 0f0a8bf3 (decisions 408-457): rugaroo 1.60s against 1.55s, MRI 5.62s, 3.5x. The Bignum row's
0.058s → 0.066s is noise: its instruction count is the same (≈543M) before and after.
Struct `==`/`eql?`/`hash` now guard against a struct that holds itself (decision 407) only where a member could lead back to it;
a struct of Integer, String and the like compiles to the code it had, and 20M `==` calls on untyped integer members take 0.22s
against 0.46s before.
`TestCompileCost` compiles 2,086 / 8,274 / 33,430 functions for 005_word_count / 064_observable / 116_cuba_web on 2026-11-22
(commit 6ff749a1; 2,071 / 8,334 / 33,302 on 2026-11-13, commit 6296b90d; 2,058 / 7,923 / 30,325 on 2026-10-29, 2,032 / 7,948 / 30,419 on 2026-10-26).

## What the columns are

- **rugaroo** and **spinel** ([matz/spinel](https://github.com/matz/spinel), a
  Ruby-to-C compiler) compile the same unmodified `.rb` file. A "-" means it
  did not compile it or printed something other than MRI: spinel cannot load
  the gems or one `objects.rb` construct.
- **Crystal** and **Go** are hand-written ports in
  [`benchmarks/crystal`](benchmarks/crystal) and [`benchmarks/go`](benchmarks/go):
  the same algorithm and sizes, written as a programmer in that language would,
  stdlib only, printing exactly what MRI prints. The deviations are noted at the
  top of each file: for example, Go uses error values instead of exceptions and a
  value struct for `objects`' vectors. kramdown and liquid have no port, since a
  port would be a different markdown or template library.
- Each "vs MRI" compares a column only on the rows it ran.

## Reading the numbers

Where rugaroo trails the native columns, the cost is mostly in its Ruby
semantics: a frozen check on every mutation, a Hash
that keeps insertion order, and objects that are always heap pointers (decisions 359, 360 and 378 removed the box around a nilable object, which put splay ahead
of spinel and Crystal and halved pairing_heap). It is ahead of spinel on
regexp, collections, blocks, format and JSON, where spinel's runtime library is
thinner. Against Crystal it wins on exceptions, which Crystal unwinds with a
backtrace per raise.

The closest rows to MRI are JSON and kramdown. MRI's `json` is a C extension; rugaroo's
generator writes into one buffer instead of a String per nested value
(decision 346), and its parser builds Hashes on an entry array (decision 350). kramdown spends its regex time partly on
patterns RE2 cannot match (lookbehinds, atomic groups, backreferences), which
run on the regexp2 backtracker (decisions 242, 332, 347).

`Math` calls Go's `math` package directly, so a printed transcendental can
differ from MRI in the last digit — Go and MRI's libm are both within one
ulp and neither is uniquely correct ([decision 43](docs/decisions/043.md)).
Ahead-of-time compilation is the other trade: `rugaroo build` is ~0.3s warm
on hello-world, but the resulting binary starts in under 10ms (MRI ~60ms).

## Integer overflow modes

`benchmarks/bignum.rb` carries `# int_overflow: promote`, so rugaroo compiles it
with Bignums ([decision 372](docs/decisions/372.md)) and `scripts/benchmark`
hands spinel the same `--int-overflow=promote`. A Bignum is a Go `math/big`
value only past 64 bits; smaller values stay machine words. That puts rugaroo
ahead of MRI's bignum code (3.6x) and spinel (2x), and between the Crystal
(GMP) and Go (`math/big`) ports.

What the default `raise` mode's overflow checks cost, measured as rugaroo built
with `--int-overflow=wrap` (no checks on `+`, `-`, `*`, `-@`) against the
default, median of 9 paired runs per workload:

| workload | raise | wrap | wrap vs raise |
|---|---:|---:|---:|
| array | 0.020s | 0.023s | +14.0% |
| binary_trees | 0.008s | 0.008s | -2.7% |
| blocks | 0.065s | 0.062s | -3.6% |
| collections | 0.125s | 0.125s | +0.3% |
| exceptions | 0.117s | 0.115s | -1.4% |
| fib | 0.016s | 0.016s | +3.7% |
| format | 0.237s | 0.238s | +0.2% |
| hash | 0.014s | 0.015s | +0.3% |
| json | 0.114s | 0.114s | +0.1% |
| kramdown | 0.405s | 0.401s | -1.0% |
| liquid | 0.164s | 0.164s | +0.0% |
| math | 0.008s | 0.008s | -0.7% |
| objects | 0.035s | 0.035s | +1.0% |
| pairing_heap | 0.205s | 0.203s | -0.8% |
| regexp | 0.212s | 0.211s | -0.2% |
| splay | 0.013s | 0.012s | -2.3% |
| strings | 0.029s | 0.029s | +2.1% |

No workload gets faster without the checks beyond run-to-run noise (the
`array` row is a 20 ms run). The checked operators inline into one compare and
a branch the CPU predicts, so `raise` stays the default at no measurable cost
and `wrap` is for programs whose arithmetic is meant to wrap (hashes,
checksums, random number generators), not for speed.

## spinel's benchmark suite

spinel ships its own benchmarks: 73 programs in
[`benchmark/`](https://github.com/matz/spinel/tree/master/benchmark), mostly
micro-benchmarks of one VM path (ivar reads and writes, sends, allocation,
loops) plus a few small applications (a template engine, a CSV pipeline, a
Lisp). Neither compiler was tuned against the other's suite, so this is the
away game for rugaroo, as `benchmarks/` is for spinel.

Re-measured on 2026-11-22 at commit 6ff749a1 (decisions 470-487) with spinel rebuilt from its head that day (`14f3ade5`), every program under `--nil-check=runtime` (bm_fannkuch left out: it does not end there), median of 5 runs:
rugaroo builds 67 of the 73 programs, spinel all of them; on the 67 both ran, MRI 105.0s, rugaroo 5.48s (19.2x MRI),
spinel 5.04s (20.8x). rugaroo is as fast or faster on 19 rows. It trails most on `bm_getivar_module` (0.058s
against 0.005s: a loop of ten additions of one module ivar, which the C compiler folds), `bm_nested_loop` (3.4x),
`bm_inline`, `bm_partial_sums` and `bm_so_lists` (2.5-2.8x); `bm_histogram` is 0.79s against 0.39s.
The six it does not build are the same six as on 2026-11-16 (rugaroo 5.58s, 18.8x; spinel `ae83631a` 5.17s, 20.3x).
The table below is that run.

```sh
S=/path/to/spinel
NIL_CHECK=runtime:bm_fasta,bm_histogram,... INT_OVERFLOW=promote:bm_bigint_fib,bm_pidigits \
  RUNS=5 MARKDOWN=1 BENCH_DIR=$S/benchmark SPINEL=$S/spinel COLS=ruby,yjit,rugaroo,spinel scripts/benchmark
```

`bm_bigint_fib` and `bm_pidigits` build with `--int-overflow=promote` in both
compilers: spinel promotes their growing locals in its default mode too, and
the files carry no `# int_overflow:` line (decision 381). In this run every row builds
rugaroo with `--nil-check=runtime` (decision 393, below; 47 of them also build by default); spinel needs no flag.

| workload | MRI | MRI+YJIT | rugaroo | spinel |
|---|---:|---:|---:|---:|
| bm_ackermann | 0.49s | 0.12s | 0.066s | 0.069s |
| bm_ao_render | 2.69s | 1.15s | 0.099s | 0.11s |
| bm_attr_accessor | 1.55s | 1.49s | 0.058s | 0.055s |
| bm_bigint_fib | 0.057s | 0.058s | 0.007s | 0.005s |
| bm_binary_trees | 0.084s | 0.068s | 0.009s | 0.008s |
| bm_csv_build | 1.87s | 1.64s | - | 0.35s |
| bm_csv_process | 0.80s | 0.70s | 0.16s | 0.21s |
| bm_fasta | 0.060s | 0.061s | 0.007s | 0.005s |
| bm_fib | 0.60s | 0.15s | 0.030s | 0.030s |
| bm_gcbench | 3.25s | 1.67s | 0.21s | 0.27s |
| bm_getivar | 0.18s | 0.18s | 0.012s | 0.005s |
| bm_getivar_module | 1.23s | 1.23s | 0.058s | 0.005s |
| bm_handle_read_loop | 3.08s | 1.92s | 0.074s | 0.20s |
| bm_handle_readonly_param | 2.63s | 1.22s | 0.052s | 0.072s |
| bm_hash_delete | 2.43s | 2.03s | 0.75s | 0.96s |
| bm_histogram | 18.63s | 18.52s | 0.79s | 0.39s |
| bm_huffman | 0.12s | 0.12s | 0.020s | 0.012s |
| bm_inline | 1.75s | 1.22s | 0.091s | 0.032s |
| bm_io_wordcount | 0.13s | 0.12s | 0.036s | 0.033s |
| bm_jekyll_lite | 0.056s | 0.056s | 0.007s | 0.005s |
| bm_json_parse | 0.41s | 0.31s | 0.036s | 0.048s |
| bm_keyword_args | 0.26s | 0.24s | 0.008s | 0.005s |
| bm_life | 0.88s | 0.42s | 0.047s | 0.029s |
| bm_linked_list | 0.35s | 0.28s | 0.041s | 0.043s |
| bm_loops_times | 1.11s | 0.35s | 0.038s | 0.042s |
| bm_mandel_term | 0.071s | 0.062s | 0.008s | 0.006s |
| bm_matmul | 0.41s | 0.42s | 0.022s | 0.011s |
| bm_micro_lisp | 0.059s | 0.073s | - | 0.006s |
| bm_nbody | 0.063s | 0.068s | 0.007s | 0.005s |
| bm_nested_loop | 0.66s | 0.67s | 0.017s | 0.005s |
| bm_nil_recv_loop | 1.36s | 0.29s | 0.084s | 0.070s |
| bm_nqueens | 0.32s | 0.33s | 0.025s | 0.013s |
| bm_object_new | 0.18s | 0.18s | 0.008s | 0.006s |
| bm_object_new_init | 0.26s | 0.19s | 0.008s | 0.006s |
| bm_object_new_no_escape | 0.30s | 0.18s | 0.009s | 0.005s |
| bm_partial_sums | 1.30s | 1.29s | 0.065s | 0.026s |
| bm_pidigits | 0.055s | 0.056s | 0.007s | 0.005s |
| bm_point_in_polygon | 1.01s | 0.35s | 0.065s | 0.065s |
| bm_poly_cells | 0.055s | 0.056s | 0.007s | 0.005s |
| bm_range_each | 22.39s | 14.79s | 0.67s | 0.44s |
| bm_rbs_items | 0.24s | 0.21s | 0.013s | 0.017s |
| bm_rbs_items_control | 0.23s | 0.21s | 0.012s | 0.010s |
| bm_rbtree | 0.53s | 0.19s | 0.025s | 0.033s |
| bm_reader_index | 11.96s | 11.73s | 0.39s | 0.23s |
| bm_ruby_xor | 1.44s | 0.39s | 0.076s | 0.092s |
| bm_send_bmethod | 0.31s | 0.28s | - | 0.005s |
| bm_send_cfunc_block | 1.07s | 1.07s | 0.031s | 0.17s |
| bm_send_rubyfunc_block | 1.83s | 1.05s | 0.073s | 0.071s |
| bm_setivar | 0.29s | 0.29s | 0.018s | 0.014s |
| bm_setivar_object | 0.30s | 0.30s | 0.018s | 0.014s |
| bm_setivar_young | 0.29s | 0.29s | 0.018s | 0.015s |
| bm_sieve | 0.67s | 0.66s | 0.037s | 0.025s |
| bm_sinatra_mini | 0.057s | 0.059s | 0.007s | 0.005s |
| bm_so_lists | 0.42s | 0.26s | 0.076s | 0.031s |
| bm_so_mandelbrot | 1.53s | 1.53s | 0.045s | 0.033s |
| bm_sort_by | 0.066s | 0.067s | 0.016s | 0.011s |
| bm_spectral_norm | 0.10s | 0.085s | 0.009s | 0.006s |
| bm_splay | 0.21s | 0.12s | 0.014s | 0.019s |
| bm_stark_field | 1.23s | 0.91s | 0.12s | 0.10s |
| bm_str_concat | 0.056s | 0.058s | 0.007s | 0.005s |
| bm_str_readonly_param | 0.055s | 0.056s | 0.013s | 0.008s |
| bm_structaref | 0.24s | 0.23s | 0.012s | 0.007s |
| bm_structaset | 0.46s | 0.46s | 0.018s | 0.014s |
| bm_sudoku | 0.18s | 0.11s | 0.013s | 0.008s |
| bm_tak | 0.56s | 0.14s | 0.031s | 0.027s |
| bm_tarai | 0.45s | 0.13s | 0.027s | 0.022s |
| bm_template | 0.91s | 0.71s | 0.033s | 0.11s |
| bm_template_render | 1.39s | 1.19s | - | 0.29s |
| bm_threaded_render | 6.20s | 5.85s | 0.53s | 0.45s |
| bm_throw | 0.29s | 0.24s | - | 0.005s |
| bm_tree_walker | 2.11s | 0.47s | 0.084s | 0.18s |
| bm_tree_walker_frames | 0.40s | 0.13s | - | 0.042s |
| bm_wordfreq | 0.056s | 0.058s | 0.008s | 0.005s |
| total (rows run) | 109.32s | 83.86s | 5.48s | 5.73s |
| vs MRI | 1.0x | 1.3x | 19.2x | 19.1x |

The 2026-10-27 run (spinel `baea3e28`, after decision 402), for comparison:
rugaroo built 65 of the 73 unmodified: 47 by default, 18 more with
`--nil-check=runtime` (on 2026-11-13, commit 0f0a8bf3: 66 of 74, 47 and 19; the table is not re-timed). On the 65 rows both ran,
the totals are MRI 98.7s, YJIT 77.3s, rugaroo 5.48s (18.0x MRI), spinel 4.15s
(23.8x MRI); before decisions 381-388 that was 9.5x against 23.3x on 44 rows.
The footer's ratios above are over different row sets, so spinel's there
includes the 8 programs rugaroo does not build.

- **rugaroo ahead of spinel:** `bm_send_cfunc_block` (5.5x), `bm_rbs_items*`
  (1.9-2.6x), `bm_tree_walker` † (2.1x), `bm_handle_readonly_param` †,
  `bm_ruby_xor` †, `bm_splay` (1.35x), `bm_gcbench`, `bm_rbtree`,
  `bm_linked_list`, `bm_ao_render` (1.1-1.25x): the allocation and block-call
  rows, as in the main table. `bm_loops_times`, `bm_ackermann`, `bm_fib` and
  `bm_attr_accessor` are even.
- **Closed by decision 387** (counted loops as Go `for` loops, in-place
  `Array#[]=`, unboxed compares): `bm_loops_times` 0.19s to 0.040s,
  `bm_range_each` 2.15s to 0.67s, `bm_sieve` 0.099s to 0.035s,
  `bm_structaset` 0.064s to 0.047s, `bm_life` 0.16s to 0.088s.
- **Closed by decision 391** (a `shift`/`pop` an `empty?` check guards is
  its element, one-element `push`/`<<` appends at the site):
  `bm_so_lists` 0.20s to 0.084s (4,323M to 684M instructions; spinel 282M).
- **Closed by #134** (`Float#**`): `bm_partial_sums` 0.43s to 0.074s.
- **Closed by decisions 398-399** (a module's singleton class as a leaf;
  ASCII fast paths for `String#length`/`[]`, unboxed `String? == String`):
  `bm_getivar_module` 0.15s to 0.070s, `bm_template` † 0.73s to 0.19s,
  `bm_json_parse` † 0.25s to 0.065s, `bm_huffman` † 0.028s to 0.019s; #139
  has the rest of the String gap.
- **Closed by decisions 400-401** (an Integer overflow panics a value, so
  small arithmetic methods inline; a Struct writer's frozen check folds away
  when nothing calls `freeze`; `/dev/null` is not a terminal, so `print` stays
  buffered, as in MRI; `Math.sqrt` called directly; Float operators written at
  the call site): `bm_so_mandelbrot` 0.12s to 0.044s, `bm_structaset` 0.046s
  to 0.018s, `bm_reader_index` † 0.51s to 0.35s, `bm_ao_render` 0.13s to
  0.099s, `bm_keyword_args` 168M to 42M instructions. `bm_nil_recv_loop` went
  0.069s to 0.085s with the same instruction count: Go now loads `v` and `nxt`
  with one pair load, which lengthens its pointer-chasing chain on Apple cores.
- **Closed by decision 402** (`Array#count`/`#sum` and `Array.new(n) { }`
  blocks as Go loops at the call, #140) and the `Integer#*` part of 400:
  `bm_life` 0.081s to 0.050s (1,082M to 615M instructions; spinel 478M),
  `bm_matmul` † 249M to 205M instructions.
- **Closed by decision 440** (an inferred `[]`/`{}` whose only veto is a nil
  read keeps its element type, the read checked at run time, #141):
  `bm_point_in_polygon` 0.18s to 0.079s (1,515M to 440M instructions; spinel
  300M), `bm_csv_process` 2,087M to 2,009M, `bm_wordfreq` 3,292M to 1,417M at 1M
  words (its `freq` is now `Hash[String?, Integer]`).
- **Furthest behind:** `bm_getivar_module` (10x; Go does not hoist the ivar
  load out of the loop, as clang does for spinel; decision 398),
  `bm_nested_loop` (3.2x: spinel's C compiler folds the loops away),
  `bm_inline`, `bm_so_lists`, `bm_getivar`,
  `bm_partial_sums` (2.6-2.8x: Go's pure-Go `math.Sin`/`Cos` on arm64).
- Rows under 10 ms are mostly process start-up for both binaries.

Why 26 do not compile by default, by the first error each reports (18 of them build with `--nil-check=runtime`):

| cause | programs | |
|---|---:|---|
| a possibly-nil value used as non-nil (`Array#[]`, `Hash#[]` (decision 5) and friends return `T?`; #129) | 18 | fannkuch, fasta, handle_readonly_param, histogram, huffman, jekyll_lite, json_parse, matmul, nqueens, reader_index, ruby_xor, sinatra_mini, spectral_norm, stark_field, str_readonly_param, sudoku, template, tree_walker |
| `String#<<` on a String the caller or a container also holds (a parameter, an Array element; decision 382, #128) | 3 | csv_build, handle_read_loop, template_render |
| a recursive method with no typed non-recursive path (decision 355) | 2 | micro_lisp, tree_walker_frames |
| an ivar assigned several types (`Array \| Integer \| String`; #127) | 1 | poly_cells |
| `define_method` | 1 | send_bmethod |
| `return` out of a block (decision 4) | 1 | throw |

spinel compiles and runs all 73. The nil row is the design choice that costs
rugaroo the most programs here: by default it makes a program prove `a[i]` is
not `nil` before using it as the element type. `--nil-check=runtime` (decision
393) puts a run-time check in its place, as spinel and MRI do, and is the
program's choice; `NIL_CHECK=runtime[:name,...]` passes it to rugaroo only. With
it, all 18 compile and print what MRI prints (the † rows; three of them also
need `setbyte` on a String only they hold, decision 395, and `bm_fannkuch` and
`bm_stark_field` needed decision 396's by-value `Integer?`, #133).
