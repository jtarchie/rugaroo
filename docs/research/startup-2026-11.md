# Start-up time of a compiled gem program, 2026-11-16

Where the compiled rubocop spends its start-up, measured at main 3561a4e9 (decisions 463-468 in), and which of it
is general to every compiled program. Report only: nothing here is built. Follows
`docs/research/runtime-speed-2026-11.md`, whose start-up paragraph (item 4: "20% `getitab`, 29% config and glob
system calls") was measured before decision 468 and is no longer true.

## Summary

- Compiled rubocop on an empty file retires 2,193M instructions (MRI 5,847M). 1,170M of that is spent before the
  first line of `main.rb` runs: every gem class body, and before those every Regexp literal in the program.
- About 680M of the 2,193M (31%) is an artefact of how rugaroo compiles: every Regexp literal in the program
  compiled at start, including those in files MRI never loads (340M; MRI compiles a file's literals when it loads
  the file, and far more cheaply), six collections during package initialisation before
  decision 463's `rbGCStart` runs (about 100M), a `realpath` for each of 619 `__dir__` calls (115M), a `stat` per
  directory entry in `Dir.glob`, a SHA1 of 310 gem files because `$LOADED_FEATURES` is complete from the start, and
  boxing racc's tables for `Ractor.make_shareable`.
- The rest is work MRI also does, and the compiled program is not much faster at it: parsing
  `config/default.yml` twice costs 322M compiled against 376M under MRI.
- Removing every artefact found gives about 1,500M (about 1,350M if rubocop also parsed its YAML once), not the
  580M that 10x needs. The remaining gap is rubocop's own
  start-up work (YAML twice, config validation, the cop registry) run through untyped dispatch, plus the collector.
- `runtime.getitab`, interface switches and type assertions are now 4.3% of start-up cumulatively (2.8% as leaf
  time; about 20% before decision 468). Not a target any more.
- About a third of the load's CPU time on Linux is kernel time the Go profile does not see (12,373 page faults,
  103 MB resident while the live heap is 3 MB). It is inside the instruction counts but not broken down here.

## Ranked fixes

Sizes are instructions retired on macOS arm64 for rubocop on an empty file (2,193M with the collector on, 1,848M
with `GOGC=off GOMAXPROCS=1`). "Measured" means measured in instructions, either on the binary or on a replica that
runs the same code on the same inputs. "Inferred" means a share of the Linux CPU profile multiplied by the macOS
total, which assumes the two machines spend instructions in the same proportions.

| # | fix | size | general? | code | risk |
|---|---|---|---|---|---|
| 1 | Compile a static Regexp literal at its first use, not in a package variable initialiser. | 340M measured (15.5%) is the upper bound: what start-up really matches is still compiled. At least 265M: six unicode-emoji patterns are 78% of the bucket, MRI never loads their files, and no run here matched one. Also removes most of fix 3a's 100M, since the compiles are what allocates 26 MB during initialisation. | Every program with Regexp literals. kramdown's load is 33% this, liquid's 14%, cuba's 8%. | `genStaticRegexp` in `internal/compiler/regexp.go`; `rbRegexpNew` in `prelude/go/regexp.go`; every read of `Regexp`'s `re` field in `prelude/regexp.rb` and `prelude/go/`. | Medium. The literal is already validated at compile time, so no error moves. Needs a once-only compile per literal (`sync.Once` or an atomic pointer per variable, no shared lock) and costs one load and branch per match. |
| 1b | Run `rxFold` at compile time for a static literal. | 80M measured, inside fix 1's 340M. | As fix 1. | `genStaticRegexp` emits the folded pattern; `rbRegexpNew` stops calling `rxFold`. | Low. The pattern is a compile-time constant. Worth doing even if fix 1 is not. |
| 1c | Compile a gem's interpolated Regexp (`rbRegexpLenient`) at its first match. | About 56M inferred for rubocop (3.1%). Far larger elsewhere: 33% of kramdown's start-up is `rbRegexpLenient` called from class bodies, and Regexps built at run time are 38% of cuba's and 24% of liquid's. | Every gem program. | `rbRegexpLenient` and `rxCompileLazy` in `prelude/go/regexp.go` and `internal/compiler/rxtranslate.go`. | Medium. Decision 176 already defers a gem regexp's error to its first use, and `rxAltLazy` already defers the regexp2 compile; this extends the same rule to RE2. User-code interpolated regexps must keep raising at creation. |
| 2 | Parse `config/default.yml` once, or make the YAML loader cheaper. | 322M measured for the two parses (14.7%); MRI spends 376M on the same two. | The second parse is upstream rubocop's own (`CacheConfig.root_dir_from_toplevel_config` reads the file again because it contains the word `CacheRootDirectory`; our gem patch only replaces its ERB step), so MRI pays it too. A cheaper loader helps every program that reads YAML. | Rubocop only: one hunk in `overrides/rubocop/gem-patches`. General: go-yaml's scanner is 60% of the load, and about 40% of that is converting rune slices to strings; the loader's scalar typing runs regexps. | The gem patch is low risk and a last resort by the project's rules. The general fix is in a dependency and needs a decision (upstream change, or a scanner of our own). |
| 3a | Call `rbGCStart` before package initialisation. | About 100M measured (load-only run: 1,166M default, 1,064M with `GOGC=800`). Six collections run at a 4 MB goal before `main` starts. | Any program whose package initialisers allocate past 4 MB. Only rubocop does among the programs measured; it mostly disappears if fix 1 lands. | `emitProgram` (make `rbGCStart` the first package variable initialiser) or `prelude/go/runtime.go`. | Low. |
| 3b | Do not start a collection straight after the ballast is allocated. | One collection of about ten; roughly 40M inferred. `gctrace` shows `gc 8 64->65->13 MB` and 3 ms later `gc 9 30->31->30 MB, 30 MB goal`. | Every program that reaches its first collection. | The finalizer in `rbGCStart` (decision 463). | Low to medium. Amends decision 463; needs its benchmark table rerun. |
| 4 | Resolve `__dir__` once per source file. | 115M measured (5.2%): 619 calls, 8,030 `lstat` system calls, 8 distinct files. | Every program that calls `__dir__` more than once per file. liquid does it 3 times. | `rbSourceDir` in `prelude/go/file.go`, or the compiler emits one `sync.OnceValue` per source file. | Low. MRI resolves the path once at load, so a memo is closer to MRI than today. |
| 5 | In `Dir.glob`, take the entry type from `os.ReadDir` instead of calling `os.Stat` on every entry. | The two globs of `lib/rubocop/**/*.rb` cost 94M measured (MRI 57M), about 2,400 `stat` calls for 1,908 results. Expect roughly half of the 94M back (inferred). | Every program that globs. | `rbGlobWalk` in `prelude/go/file.go`. | Low. A symlink or an unknown type still needs the `stat`. |
| 6 | Make `$LOADED_FEATURES` grow as files load, or patch the gem. | About 50M inferred of a 75M bucket (4%). `ResultCache.source_checksum` reads and digests about 310 files; under MRI it digests 53. See "Differences from MRI". | The idiom `$LOADED_FEATURES - before` is general; this cost is rubocop's. | `emitLoadedFeatures` in `internal/compiler/decls.go` (decision 444), or one gem-patch hunk. | Medium for the general fix: the list has to follow the order class bodies run in. |
| 7 | Freeze a typed Array in `Ractor.make_shareable` without converting it to `Array[untyped]`. | Up to 55M inferred (2.9%). 62% of it is `_ToAny` on racc's `Array[Integer?]` tables. | Every program with a racc-generated parser (each ends with `Ractor.make_shareable(Racc_arg)`). | `prelude/ractor.rb`. | Low. |
| 8 | Start `-profile-out`'s profile in a package variable initialiser of a file that sorts first. | No run-time gain. Today the profile starts in an `init()` function, after every package variable is initialised, so it cannot see fix 1's bucket at all (37% of rubocop's load). | The tool. | `profileGo` in `cmd/rugaroo/main.go`. | Low. This task used that change as an experiment and reverted it. |

Fixes 1, 1b, 1c, 3a, 4, 5 and 7 are general, touch no dispatch, add no lock shared across the process, add no
dependency and cannot slow a small program: hello-world has no Regexp literal, no glob and no `__dir__`.

What these do not reach, by the same profile (collector off, 176.8 ms of CPU per run on Linux):

- rubocop's config processing after the YAML is parsed (`add_missing_namespaces`, validation, obsoletion rules,
  qualified cop names): about 20 ms, 11%, about 210M inferred.
- Class bodies: the cops' `inherited` hook, registry and callback tables are 6.9 ms (3.9%); every other class body
  and constant is 10.3 ms (5.8%).
- The unicode-display_width index (gunzip and `Marshal.load`): 4.1 ms (2.3%). MRI does the same.
- The collector with fixes 3a and 3b in: about 200M. Start-up allocates about 150 MB to lint an empty file.

## How it was measured

- **Instructions:** `/usr/bin/time -l`, the second run of a binary, inputs in `~/.cache/rugaroo-rbtest`. The macOS
  binary is `$W/rubocop-perf8`, built from the same commit.
- **Profiles:** a Linux arm64 build of the same commit with `-profile-out`, run in `debian:stable-slim` under Docker.
  The gems, the unicode data files and the inputs were copied into the container's own file system at the paths the
  build saw, so no file access crosses the bind mount. Each configuration ran 100 times (300 for the small
  programs) and the profiles were merged. The binary's output and exit status equal MRI's on `--version`,
  `empty.rb`, `a.rb` and `dir20`.
- **Profile start moved earlier.** `-profile-out` starts its profile in an `init()` function, which Go runs after
  all package variables are initialised. For this task the CLI was changed (and reverted) to start it in a package
  variable initialiser of `a_profile.go`. Without that, 37% of the load is invisible.
- **Load-only run.** Running the binary without `RUBOCOP_LIB` raises `KeyError` at `main.rb:3`, after every gem
  class body has run (rugaroo runs them before the main file's first statement) and before `RuboCop::CLI` exists.
  That run is "load" below. Under MRI the same failure happens before any gem is required, so it has no MRI twin.
- **Replicas.** Three buckets were measured in instructions outside rubocop: the 463 static regexps taken from the
  generated `main.go` and compiled with the compiler's own `rxFold` and `rxCompile` (a temporary test in
  `internal/compiler`, removed); 619 `filepath.EvalSymlinks` calls on the real path; and a 15-line rugaroo program
  that parses `default.yml` twice or globs `lib/rubocop/**/*.rb` twice, also run under MRI.
- Scripts, profiles and binaries are in `~/.cache/rugaroo-linux/su` (`prof.sh`, `b2.sh`, `spec-start.txt`,
  `spec-file.txt`, `join.sh`).

## Deliverable 1: start-up profile

Instructions retired, macOS:

| run | compiled | compiled, `GOGC=off GOMAXPROCS=1` | MRI |
|---|---:|---:|---:|
| load only (stops at `main.rb:3`) | 1,170M | 1,035M | - |
| `--version` | 1,756M | 1,468M | 3,538M |
| `empty.rb` | 2,193M | 1,848M | 5,847M |
| `a.rb` (2 lines) | 2,276M | 1,931M | 5,918M |
| `dir20` | 14,024M | 12,696M | 11,487M |

Linux CPU profile, milliseconds of CPU per run, each sample counted once in the first row it matches:

| cause | load | `--version` | `empty.rb` | share of `empty.rb` | `empty.rb`, collector off |
|---|---:|---:|---:|---:|---:|
| collector (background workers, assists, sweep) | 11.0 | 33.5 | 42.8 | 17.9% | 0.0 |
| package initialisation: compiling static Regexp literals | 39.0 | 41.3 | 41.5 | 17.3% | 38.3 |
| Regexps built while the program runs | 2.5 | 5.8 | 5.8 | 2.4% | 5.4 |
| `rbSourceDir`: `realpath` per `__dir__` | 17.7 | 18.1 | 17.5 | 7.3% | 16.0 |
| `Dir.glob` | 5.7 | 5.1 | 11.7 | 4.9% | 10.3 |
| YAML: `default.yml` twice | 0.0 | 37.3 | 37.4 | 15.6% | 32.3 |
| `Ractor.make_shareable` on racc tables | 5.6 | 5.7 | 5.8 | 2.4% | 5.2 |
| unicode-display_width index | 4.8 | 5.2 | 4.1 | 1.7% | 4.1 |
| regexp_parser class bodies | 2.4 | 2.4 | 1.7 | 0.7% | 1.9 |
| `ResultCache`: source checksum, and config work reached through `cache_root` | 0.0 | 11.0 | 15.8 | 6.6% | 17.5 |
| config: loader, validation, obsoletion | 0.1 | 0.3 | 11.9 | 5.0% | 10.2 |
| cop class bodies at load (inherited hook, registry, callbacks) | 5.7 | 6.8 | 9.4 | 3.9% | 6.9 |
| cop registry queries during config load, Team | 0.0 | 0.0 | 15.9 | 6.6% | 16.6 |
| runner, target finder, formatter, option parsing | 0.0 | 0.5 | 1.1 | 0.5% | 1.2 |
| scheduler and idle threads | 2.1 | 3.5 | 3.9 | 1.6% | 0.2 |
| rest of `main()`: other class bodies and constants | 10.2 | 12.1 | 12.4 | 5.2% | 10.3 |
| total | 107.2 | 188.9 | 239.2 | | 176.8 |

The same `empty.rb` samples by the kind of function the sample was in (leaf), collector on: allocation and
copying 43.1 ms (18%), system calls 34.3 (14%), Go runtime scheduling and idle collector workers 34.2 (14%),
collector marking 24.7 (10%), regexp compiling 22.5 (9%), maps and hashing 17.0 (7%), Go standard library 16.0
(7%), untyped dispatch 14.6 (6%), typed core library 9.7 (4%), go-yaml 8.8 (4%), interface tables and type
assertions 6.7 (2.8%), the gems' own bodies 5.0 (2%).

What each bucket is:

- **Static Regexp literals.** `genStaticRegexp` emits `var rbRe_<hash> = rbRegexpNew(...)`, and Go runs all 463 of
  them before `main`. Six of them are unicode-emoji's generated patterns, 37 KB to 75 KB of source each, which RE2
  rejects and regexp2 then compiles: those six are 78% of the bucket. MRI never compiles them, because the gem
  autoloads the files that hold them, and no run measured here matched one (no input had an emoji). `rxFold` parses
  every pattern a second time at run time although the pattern is a constant. Package initialisation allocates
  27 MB in 229,297 allocations (`GODEBUG=inittrace=1`), all but a few KB of it here.
- **Collector.** Ten collections for `empty.rb`. Six or seven run during package initialisation at a 4 MB goal,
  because `rbGCStart` is the first statement of `main` and package initialisers run before it. One runs 3 ms after
  the first real collection, when the finalizer allocates the ballast and sets GOGC to 100.
- **`rbSourceDir`.** `cop/style.rb` and its siblings call `__dir__` once per registered cop. Each call is
  `filepath.EvalSymlinks`, one `lstat` per path component. 8,030 of the 10,453 `stat` calls in an `empty.rb` run
  are these.
- **`Dir.glob`.** `rubocop.rb:155` and `ResultCache.rubocop_extra_features` each glob `lib/rubocop/**/*.rb`.
  MRI does both too. `rbGlobWalk` calls `os.Stat` on every entry although `os.ReadDir` already gave its type.
- **YAML.** rubocop parses `default.yml` (200 KB, 6,598 lines) twice, under MRI as well. go-yaml's `lexer.Tokenize`
  is 60% of the load, its parser 12%, building the Ruby values 27%.
- **`ResultCache.source_checksum`** runs even with `--cache false`, under MRI too (`runner.rb:85`).

Work MRI also does, against artefacts of compilation (instructions, `empty.rb`):

| | size | how known |
|---|---:|---|
| artefact: static Regexp literals compiled at start (MRI loads 2 of unicode-emoji's 20 generated regex files; the six largest patterns, 78% of this, are in files it never loads) | 340M | measured (replica); the MRI side from `$LOADED_FEATURES` at exit of a `dir20` run |
| artefact: collections during package initialisation | about 100M | measured (`GOGC=800` on the load run) |
| artefact: `realpath` per `__dir__` | 115M | measured (replica) |
| artefact: `stat` per glob entry, the part above MRI's cost | about 40M | measured (94M against MRI's 57M) |
| artefact: digesting 310 files instead of 53 | about 50M | inferred |
| artefact: boxing in `Ractor.make_shareable` | about 35M | inferred |
| MRI also: YAML twice | 322M | measured (replica; MRI 376M) |
| MRI also: the two globs at MRI's cost | 57M | measured |
| MRI also: config processing, cop registry queries, class bodies, unicode index, source checksum, runner (70 ms of the 176.8 ms profile, less the two artefacts above) | about 690M | inferred |
| MRI also: Regexps built while class bodies run | about 56M | inferred |
| collector after package initialisation | about 240M | measured (342M total, minus the 100M above) |
| unattributed: process start, page faults, exit, and the error of the conversions above | about 150M | the remainder of 2,193M |

Not found, although looked for: Go package initialisation outside `main` (main's own starts 1.8 ms after the
runtime's on Linux, 5.6 ms on macOS); itab construction (4.3% cumulative, about 20% before decision 468); the frozen string literal table (4,321 entries, under 1 ms); symbol set-up (a Symbol is a string
conversion); class objects (static data).

**Kernel time the profile does not see.** 100 load-only runs in the container take 15.4 to 16.2 s of user plus
system time, 154 to 162 ms each, of which 47 to 51 ms is system time; the merged profile holds 107 ms per run. The
difference, about a third, is process start and page faults: a load-only run takes 12,373 minor faults and reaches
103 MB resident (hello-world: 310 faults, 2 MB), although its live heap is 3 MB. The macOS load-only run reports
0.03 s of system time in 0.12 s. What touches that much of a 291 MB binary (94 MB of it is `gopclntab`, 17 MB type
data) was not investigated; the 27 MB of package-initialisation garbage is part of it, so fix 1 removes some.

## Deliverable 1, second half: `dir20` from the same build

100 runs, 1,654 ms of CPU per run with the collector on (1,486 ms with it off). Start-up is the `empty.rb` column
above; everything else is the next section.

| cause | `dir20` | `empty.rb` |
|---|---:|---:|
| collector | 220.1 ms (13.3%) | 42.8 ms |
| package initialisation (static Regexp literals) | 40.1 ms (2.4%) | 41.5 ms |
| lexer | 375.3 ms (22.7%) | 0.0 |
| racc parser | 99.8 ms (6.0%) | 0.1 |
| AST building | 87.8 ms (5.3%) | 0.0 |
| `ProcessedSource` rest | 59.2 ms (3.6%) | 0.2 |
| cops | 529.4 ms (32.0%) | 8.8 |
| Commissioner and traversal, minus the cops | 29.1 ms (1.8%) | 3.7 |
| Team, per-file cop set-up, comment config | 69.4 ms (4.2%) | 23.2 |
| runner, config, target finder, formatter | 57.7 ms (3.5%) | 52.0 |
| everything else (start-up's file and YAML work for the most part) | 86.2 ms (5.2%) | 66.9 |

## Deliverable 2: per-file work (`dir20` minus `empty.rb`)

Collector off (`GOGC=off GOMAXPROCS=1`), milliseconds of CPU per run of 20 files; 1,309 ms in total. The
collector, from the default pair, adds 177 ms.

| subsystem (cumulative, first match wins) | ms | share |
|---|---:|---:|
| lexer (`Parser::Lexer#advance` and helpers) | 391.2 | 29.9% |
| racc parser (`do_parse`, reductions, grammar actions) | 97.1 | 7.4% |
| AST building (`Parser::Builders::Default`, nodes, source maps) | 88.1 | 6.7% |
| `ProcessedSource` rest (tokens, comments, lines) | 64.4 | 4.9% |
| cops: callbacks and what they call | 538.4 | 41.1% |
| Commissioner and AST traversal, minus the cops | 26.6 | 2.0% |
| Team, per-file cop set-up, comment config, corrections | 54.5 | 4.2% |
| runner, config lookup, formatter | 14.0 | 1.1% |
| other | 34.8 | 2.7% |

| kind of function the sample was in (leaf) | ms | share |
|---|---:|---:|
| untyped dispatch (`rbDyn*`, `rbView*`, `rbEq`, `rbUnbox`, `rbSend*`, `Dyn*`, `rbAs`) | 566.4 | 43.3% |
| allocation and copying | 232.4 | 17.8% |
| the gems' own compiled bodies | 103.9 | 7.9% |
| regexp matching | 91.7 | 7.0% |
| typed core library | 85.1 | 6.5% |
| interface tables, type assertions, interface equality | 65.1 | 5.0% |
| maps and hashing | 64.5 | 4.9% |
| boxing into an interface (`runtime.convT*`) | 24.9 | 1.9% |
| Go runtime, other | 24.8 | 1.9% |
| Go standard library | 21.6 | 1.6% |
| system calls | 15.3 | 1.2% |
| collector work on the mutator, stack growth, panics | 11.9 | 0.9% |

Notes for whoever works on this:

- **Node-pattern matchers cannot be separated.** Decision 204 compiles each matcher as an ordinary method of its
  cop class, so by name they are part of "cops".
- **One construct is 20% of per-file work.** `Parser::Lexer#advance` calls `rbEq[any]` directly for 269 ms per
  run. Ragel's `case _lex_trans_actions[_trans]` has about 370 `when <Integer>` arms; its subject is the result of
  a dynamic `[]` (`t11_ := rbDynOp_idx(...)`), so it compiles to a chain of `case bool(rbEq[any](t11_,
  Integer(65)))` tests tried in order. Each test calls `rbUnbox` twice (203 ms flat in total, the largest single
  function in the profile) and then fails three interface assertions. Decision 467 made this a Go `switch` only
  when the subject is typed, so the fix that fits the rules is typing `_lex_trans_actions` (a `sig/gems/parser`
  line, or inference of a class-level table read through `self.class.send`). An open question for the user: an
  untyped subject whose every `when` is an Integer literal could compile to one assertion to Integer in front of a
  `switch`. That is a compile-time rule, but it sits close to the "no untyped fast path" constraint.
- `Layout/TrailingEmptyLines#on_new_investigation` spends 24 ms per run (1.6% of the whole run) in regexp2
  backtracking, and `Lint/DuplicateMethods#humanize_scope` 12 ms.

## Deliverable 3: generality

Instructions retired on macOS, and system calls on Linux (`strace -f -c`; non-profiling builds for the small
programs, the profiling build for rubocop):

| program | binary | compiled | MRI | package initialisation | collections | system calls |
|---|---:|---:|---:|---|---:|---:|
| plain Go hello-world | | 17M | | | | |
| hello-world | 1.8 MB | 19M | 360M | 0.02 ms, 13 allocations | 0 | 345 |
| liquid: load, render `{{ a }}` | 18 MB | 54M | 740M | 0.95 ms, 6,441 allocations | 0 | 1,252 |
| cuba (example 116): load only | 16 MB | 75M | 2,551M under `bundle exec`, of which Bundler alone is 1,917M | 0.88 ms, 5,509 allocations | 0 | 1,152 |
| kramdown: load, convert `"x"` | 21 MB | 217M | 828M | 2.2 ms, 13,089 allocations | 0 | 1,732 |
| rubocop: load only | 295 MB | 1,170M | - | 39 ms, 229,301 allocations | 6 | 13,237 |
| rubocop `--version` | | 1,756M | 3,538M | | | 13,740 |
| rubocop `empty.rb` | | 2,193M | 5,847M | | 10 | 20,196 |

- Example 116 has no way to exit before it starts its server. The row is the example with
  `exit if ENV["LOAD_ONLY"]` added after `Cuba.define`; the whole example is 77M.
- The small programs' system calls are the Go runtime's (`rt_sigaction` 114, `futex`, `nanosleep`, thread set-up).
  None makes more than 25 file calls. rubocop's are file calls: 9,065 `stat` at load, 8,030 of them from
  `rbSourceDir`; an `empty.rb` run makes 367 `openat` and 568 `read` calls (51 and 18 at load), about 310 of the
  opens being `source_checksum`.

Share of each program's start-up CPU (Linux profile, 300 runs each, profile writer excluded):

| cause | kramdown | liquid | cuba | rubocop `empty.rb` |
|---|---:|---:|---:|---:|
| static Regexp literals at package initialisation | 33.1% | 14.0% | 7.8% | 17.3% |
| Regexps built at run time (class-body constants; for kramdown about a quarter of it is during the parse) | 43.6% | 23.7% | 38.3% | 2.4% |
| collector | 6.1% | 0% | 0% | 17.9% |
| `realpath` per `__dir__`, glob | 0% | 2.1% | 0% | 12.2% |
| everything else | 17.2% | 60.2% | 53.9% | 50.2% |
| CPU per run | 30.1 ms | 6.2 ms | 4.7 ms | 239.2 ms |

The three gems' directories were not copied into the container, so the little file work these programs do
(liquid's three `__dir__` calls) fails early there and is understated.

So Regexp construction is the general start-up cost of a gem program: 77% of kramdown's load, 46% of cuba's, 38% of
liquid's. The collector, the system calls and YAML are rubocop's, and a program has to be that large or read that
much before it meets them. A program with no gems starts in 19M instructions, 2M more than an empty Go program.

## Deliverable 5: differences from MRI

Neither changes what rubocop prints.

1. **`$LOADED_FEATURES` is complete before the first `require` runs.** `rbLoadedFeatures()` returns the fixed list
   of compiled source files (decision 444), so `before = $LOADED_FEATURES.dup; require ...; $LOADED_FEATURES -
   before` is empty where MRI gives the files just loaded. rubocop uses exactly that (`rubocop.rb:9` and `:154`) to
   build `ResultCache.rubocop_required_features`. Under MRI the list has 1,365 entries and `rubocop_extra_features`
   has 53 files; compiled, the list holds only the glob's files, so `source_checksum` reads and digests about 310
   files of parser, rubocop-ast, regexp_parser and unicode-emoji. The checksum is a result-cache key, so a compiled
   rubocop and an MRI rubocop compute different keys. It is by design, not a regression, and user code cannot read
   `$LOADED_FEATURES` at all (compile error, decisions 61 and 361).
2. **`__dir__` is resolved on each call.** MRI fixes a file's real path when it loads the file. A compiled program
   that changes directory, or whose source directory is replaced by a symlink while it runs, can answer differently
   on a later call. No input here shows it.
