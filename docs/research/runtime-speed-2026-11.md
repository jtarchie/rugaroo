# Runtime speed, 2026-11-15: why compiled rubocop is not 10x MRI, and what moves it

Goal set by the user: compiled rubocop at least 10x faster than `ruby`, as a baseline that lifts every gem program,
and at least level with spinel on its suite. Branch `perf/rubocop-10x` (decisions 463-467). This file records what
was measured, what landed, and what is still between here and 10x. 
## How to measure

- **Instructions retired, not wall clock.** `/usr/bin/time -l cmd` on macOS; the machine ran at load 12-26 all
  session. `GOMAXPROCS=1` (and `GOGC=off` when the collector is not the subject) makes one binary repeat to under 1%.
- **Not inside `$TMPDIR`.** `overrides/rubocop/bench` runs there, a directory with 65k entries that rubocop's config
  search walks: both MRI and the compiled binary pay 0.4-0.5 s of system time for it. Inputs were copied to
  `~/.cache/rugaroo-rbtest` (`a.rb`; `dir20/`, the first 20 files of rubocop's `cop/style/`; `runner.rb`).
- **Profiles on Linux.** On darwin a Go CPU profile puts 15-75% of the samples in `runtime.madvise` and
  `pthread_cond_wait`, and `sample` cannot walk Go stacks. Build with `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 rugaroo
  build -profile-out /l/pp.pprof ...` and run in `debian:stable-slim` with the output directory mounted (`docker run -v
  ~/.cache/rugaroo-linux:/l`); rubocop also needs its patched gems and the `unicode-display_width`/`unicode-emoji`
  data files mounted at the paths the build saw.
- **`RUGAROO_NO_INFER_CACHE=1` after a compiler change.** The inference cache is keyed by the input files, not by
  the compiler: a changed inference rule otherwise reads the old types back.
- **Small programs first.** A rubocop build is 20 minutes and 16 GB. Each finding below was reduced to a program of
  50 lines or fewer built with `-gem dir`, then checked on the parser gem alone (`require "parser/ruby33"`, 70 s
  build), then on rubocop.

## Baseline (main be30d759, 2026-11-15)

| input | MRI | compiled | compiled / MRI |
|---|---:|---:|---|
| `--version` | 3,693M | 1,912M | 1.9x faster |
| `a.rb` (2 lines) | 5,790M | 3,239M | 1.8x faster |
| `cli.rb` | 6,483M | 5,229M | 1.2x faster |
| `runner.rb` | 7,364M | 10,758M | 1.5x slower |
| `dir20` (20 files, 3,292 lines) | 11,526M | 23,072M | 2.0x slower |

Start-up is 1.8x better than MRI; the work per file is 3-5x worse (`dir20` minus `a.rb`: MRI 5.7G, compiled 19.8G).
10x on both needs start-up down 5.5x and per-file work down about 35x.

"The benchmarks got slower" did not reproduce for the typed rows: against 7c2e5b62 (decision 402), `benchmarks/` rows
are within ±1% or better (objects −12%, collections −7%), and spinel's suite is 11% fewer instructions in total. The
two gem rows did regress: liquid +15%, kramdown +8%. `git bisect` on kramdown's dynamic call sites (948 at 7c2e5b62,
1,463 at be30d759) lands on bae8243e (2026-11-12, decision 444's `anyUse`): once the rounds settle, a gem parameter
that any gem call passes an untyped value becomes untyped, so that the call cannot raise TypeError where MRI runs.
That commit alone takes kramdown to 1,287 sites; later rubocop commits add the rest. It is the same trade as
everywhere below: a typed parameter fed an untyped value must either assert (and may raise) or give up its type.

## Where the per-file time goes

Linux profile of `dir20`, collector off, flat time by kind:

| share | kind |
|---:|---|
| 29% | untyped dispatch: `rbDyn*`, `rbView*`, `rbEq`/`rbUnbox`, `Dyn*` wrappers |
| 21% | allocation (boxed results and argument slices of those calls, for the most part) |
| 19% | whole-String rune conversions (`String#index(s, pos)`, `Regexp#match(s, pos)`) |
| 11% | interface lookups, type assertions, boxing |
| 7% | typed core library |
| 4% | the gems' own compiled bodies |
| 9% | regexp, maps, system calls, the rest |

About 60% is the price of values being untyped. rubocop's generated Go has 36,186 dynamic call sites and 8,899
dispatchers. A dynamic call's result is always untyped (`genDynCall` returns `TAny`), so one untyped receiver makes
everything computed from it untyped too.

## What landed

| decision | what | measured |
|---|---|---|
| 463 | 16 MiB heap ballast: the first collection at 32 MiB, not 4 MiB | strings −36%, objects −22%, collections −20%, json −19%, liquid −17%, kramdown −13%, regexp −11% |
| 464 | a long String's ASCII flag, validity and length are cached; `index`/`rindex`/`match` with a position no longer convert the String to runes | untyped-library repro 31,261M → 130M |
| 465 | `unpack("U*")` and other starred integer formats are `Array[Integer]`; `rbMustO` inlines | Ragel-shaped scanner 3,383M → 349M (MRI 4,653M) |
| 466 | a call that passes real types to a callee still waiting for them has a provisional result | `Parser::Source::Buffer@source` typed |
| 467 | Integer `case` as a Go `switch`; untyped `ary[start, len]` and `ary[start, len] = []` without copying the Array | parser gem 2,470M → 1,751M |
| sig/gems/parser | the lexer's ivars and five methods | parser gem 7,945M → 2,470M |
| overrides/rubocop | `parser-3.3.13.0.patch`: no UTF-32LE slice copy | a file with a non-ASCII character lexed to `"\u0000"` tokens before (present on main; the reason `dir20` printed 25 offenses against MRI's 24) |

The parser gem alone, 21 files: MRI 2,312M (1,360M of it parsing); compiled 7,945M → 1,751M.

rubocop rebuilt with decisions 463-467 and the parser patch, before `sig/gems/parser` was added (every input prints
what MRI prints, `dir20` included; the same 5 gem methods fail to compile as on main):

| input | MRI | main | branch | branch / MRI |
|---|---:|---:|---:|---|
| `a.rb` (2 lines) | 6,119M | 3,239M | 3,056M | 2.0x faster |
| `cli.rb` | 6,884M | 5,229M | 4,518M | 1.5x faster |
| `runner.rb` | 7,668M | 10,758M | 6,715M | 1.1x faster |
| `dir20` | 11,726M | 23,072M | 15,910M | 1.4x slower |
| 60 files of `cop/lint/` | 15,675M | - | 24,025M | 1.5x slower |

Per-file work fell by about a third (`dir20` minus `a.rb`: 19.8G → 12.9G; MRI 5.6G). The 10x budget is 580M for
`a.rb` and 1,150M for `dir20`: start-up is 5x over it and untouched, per-file work 22x over it.

## What is still in the way

In the order the parser gem's profile shows them:

1. **racc's tables are untyped.** `_racc_do_reduce` destructures `Racc_arg`, one constant Array of Arrays, Integers
   and a boolean, so each table is untyped and each `reduce_table[i]`, `goto_table[i]` is a dynamic call through
   `rbViewCore`'s switch on the method name (21% of the parse after the fixes above). A constant Array literal of
   mixed classes that is only ever read by position could be typed as a tuple (#122 is the same idea for stacks).
2. **Stored lambdas.** An ivar assigned from other ivars holding lambdas (`@num_xfrm = @emit_integer`) loses the
   lambda's parameter and result types; `sig/gems/parser` covers the lexer's by hand.
3. **A dynamic call's result is untyped even when every method of that name in the program returns one type.**
   `node.loc.expression.begin_pos` restarts typed after one hop only if `loc`'s result is known. The closed world
   knows every definer of a name; where they agree, the result can be that type behind one assertion.
4. **Start-up** (3.2G for a 2-line file; the 10x target is 0.58G). Linux profile of `a.rb`: 29% scheduler and
   system calls (config and glob), 23% allocation, 20% interface lookups (`getitab`: the first assertion of each
   (interface, class) pair across ~2,000 class objects), 11% maps. Not worked on yet.
5. **prism instead of the Ragel lexer and racc.** rubocop's default engine is `parser_whitequark`; with
   `ParserEngine: parser_prism` and `TargetRubyVersion: 3.4` MRI itself spends 10% fewer instructions on `dir20`
   (10,577M against 11,726M), and compiled prism is C code run as wasm. Not measured compiled: any `.rubocop.yml`
   beside the input stops the compiled binary (next item).
6. **Known wrong results** found on the way, not fixed: a project `.rubocop.yml` raises `no block given (yield)` in
   `RuboCop::Config#each_key` (`config_loader_resolver.rb:166`, a blockless `each_key`), so only the default
   configuration runs; a rational literal (`2.5r`) raises TypeError in the parser gem (`Rational("2.5")`), on main
   too; `kids << a << i` on an Array first seen holding one class
   raises TypeError in gem code (the chain's second push is a dynamic call on the first's result, and the element
   type never widens); `Layout/LineLength` raises on `prelude/string.rb` as an input.

## A different inference, if the rounds are not enough

Every root found here was a cycle the round-based inference resolved pessimistically: a value is untyped in the first
round because its producer has not been typed yet, and that untyped value is then what types the producer. Decisions
358, 383 and 466 each exempt one more shape. Two designs would replace the exemptions:

- **Optimistic fixpoint.** Start every slot (parameter, ivar, return, local) at bottom, join in what is written, and
  only then check. This is spinel's order (`docs/internals/fixpoint-convergence.md` there) and needs the type
  computation separated from code generation, which today is the same pass run in `probe`.
- **Seeds from the MRI run, verified by the closed world.** `-trace` already runs the program under MRI. Recording
  the classes each ivar, parameter and return held gives a starting point for the same fixpoint: a slot starts at
  what MRI saw instead of at bottom, and widens wherever a write in the compiled program does not fit. The seed is
  never trusted (one run on one input proves nothing); it only decides where the fixpoint starts, so a cycle that
  MRI resolved to Integer starts as Integer. The same recording, printed as RBS, is a first draft of a `sig/gems`
  file for a person to review.
