# CLAUDE.md

rugaroo: transpiles a typed subset of Ruby (rbs-inline `#:` annotations) to a single Go file. README.md is the user-facing intro and docs/README.md indexes the user docs (getting-started, support, gems, cli; a new library or CLI flag needs its row in docs/support.md or docs/cli.md, enforced by `TestSupportDoc`/`TestCLIDoc`, decision 373); docs/design.md holds the goals and the index of numbered decisions, one file each in `docs/decisions/NNN.md` ("decision N" anywhere means that file; docs/history.md is the early walkthrough, superseded where they disagree). The decisions are binding: read the relevant file before changing behavior, and add or amend one when a new decision is made. A new decision is the next-numbered file (last: 462): `# Decision N`, then a `**Rule:**` line (one present-tense sentence, what is true now; it is the index row), then the text. A decision fully replaced later becomes a stub (`*Superseded by decision M.*` + gist + the `git show` of its full text); amending one updates its Rule line if the rule changed; when one amends an older decision, also add a "see also" pointer inside the old file. The index table in design.md is generated: run `scripts/design-index` after adding or retitling a decision (`TestDesignIndex` fails when it is stale).

## Commands

```sh
git submodule update --init         # gems/*: vendored gem sources; the build embeds their mirror in vendored/ (scripts/vendor-gems; decision 328)
bundle install                      # rbs, rbs-inline, webrick (needed by tests)
bundle install --gemfile examples/135_roundhouse_blog/Gemfile   # example 135's roundhouse-rugaroo gem, from GitHub
go test ./...                       # the oracle; see below. ~6 min warm (2026-11-01); a prelude line shift costs nothing more (decision 434); an edit to a helper the big programs keep rebuilds and re-lints them, ~40 min (docs/research/test-time-2026-11.md)
go test -run 'TestExamples/each/005_word_count' .   # one example
go test -run 'TestRun/^string_' .             # behaviour snippets by prefix
go test -run 'TestErrors/^regexp$' .          # one compile-error archive
go test -run 'TestMinitest/^string$' .        # one minitest file (testdata/test/string_test.rb), vs MRI
go run ./cmd/rugaroo test testdata/test --seed 1  # the whole suite as one program, as `minitest testdata/test` (slow to build: decision 81)
go run ./cmd/rugaroo test testdata/test/array_test.rb --seed 1 -v   # one test file (any minitest flags); a dir runs them all as one program
RUGAROO_RUN_SKIPPED=1 go test -run TestRun .    # also run `# skip:` known failures
RUGAROO_INT_OVERFLOW=wrap go test ./...         # the suite in another --int-overflow lane (wrap or promote); testdata/int-overflow/<mode>-skip.txt leaves out what pins another mode (decision 372)
RUGAROO_TEST_TIMING=run.tsv RUGAROO_TEST_KEEP=keep/ go test -json ./... > run.json; scripts/test-time run.json run.tsv   # where a run's time goes: per-step log (build cache hit/miss, -race, MRI/rbs cache, lint) and every program's main.go (docs/research/test-time-2026-11.md)
RUGAROO_NO_MRI_CACHE=1 go test ./...            # rerun MRI and rbs instead of their cached results (`os.UserCacheDir()/rugaroo-test/{mri,rbs}`; key: Ruby description, TZ, checkout dir, sha256 of Gemfile.lock, extra args, sources; decisions 272, 435)
RUGAROO_NO_PRUNE=1 go run ./cmd/rugaroo build -work ...  # emit the whole prelude (debugging the pruner)
RUGAROO_NO_SPECIALIZE=1 go run ./cmd/rugaroo build ...  # keep module funcs generic over Self (A/B for decision 377)
go -C scripts/gen-encoding-fixes run .        # regenerate MRI's corrections to x/text's encoding tables, Encoding constants and names (decision 380); rerun after a Ruby or x/text upgrade
RUGAROO_INFER_DEBUG=1 go run ./cmd/rugaroo run x.rb    # what each parameter-inference round typed and could not compile (decision 146)
RUGAROO_PRUNE_DEBUG=1 go test -run TestBootAutoload .  # gem programs: why each method name was reached, live gem classes (decision 169)
RUGAROO_DISPATCH_DEBUG=1 go run ./cmd/rugaroo build x.rb   # why a Kernel/Object name keeps a Go method instead of a type-switch dispatcher (decisions 212-213); =all for every owner
RUGAROO_GEM_WARNINGS=1 go run ./cmd/rugaroo gen ...       # also print gem files' warnings (skipped statements, nil constants: decision 205)
RUGAROO_ARM_DEBUG=1 go run ./cmd/rugaroo gen ...          # why an untyped receiver's block call has no arm for a class (it raises NoMethodError there; decision 444)
RUGAROO_NO_EARLY_ROUNDS=1 go run ./cmd/rugaroo gen ...    # run inference rounds to the cap instead of stopping after 3 settled ones (A/B for decision 444)
RUGAROO_GEM_WARNINGS=1 RUGAROO_UNTYPED_WHY=1 go run ./cmd/rugaroo gen ...  # each dynamic-call warning names the root of its untyped receiver (sig untyped, widened local, unannotated return, ...)
RUGAROO_BACKTRACE=1 ./prog                       # a built program reports an uncaught exception as MRI does, file:line and from lines (decision 178)
RUGAROO_GO_STACK=1 ./prog                        # a Go runtime panic turned into a Ruby exception carries its Go stack (decision 324)
RUGAROO_COST=1 go test -run TestCompileCost -v .    # compiled functions + compile time/memory of 005_word_count, 064_observable, 116_cuba_web (#96); or a comma list of examples
RUGAROO_CPUPROFILE=gen.prof go run ./cmd/rugaroo gen x.rb > /dev/null  # CPU profile of the compile itself (go tool pprof -top gen.prof)
go -C scripts/census run . /path/main.go           # what a generated main.go spends its functions on (forwarders, Dyn wrappers, stubs); ./inst walks generic instantiations
go test -run '^$' -fuzz FuzzCompile -fuzztime 60s .              # fuzz targets: FuzzCompile (.),
go test -run '^$' -fuzz FuzzParseType -fuzztime 30s ./internal/rbs # FuzzParseType/FuzzParseMethodType (rbs),
go test -run '^$' -fuzz FuzzTranslateRegexp ./internal/compiler    # FuzzGoMethodName/FuzzTranslateRegexp (compiler)
go run ./cmd/rugaroo stubs && go vet -tags rugaroo_prelude ./prelude/go   # type-check prelude/go helpers against generated stubs (decision 154; TestPreludeGo does the same)
go test ./internal/rbs              # RBS parser unit tests
golangci-lint run ./...             # repo lint (.golangci.yml)
BUNDLE_GEMFILE=gemfiles/rubocop.gemfile bundle exec rubocop                 # Ruby lint, default cops (.rubocop.yml). Autocorrect with care outside scripts/ and cmd/: rugaroo reads the exact syntax of testdata/, examples/, prelude/ and benchmarks/ (`#:` comments, `%x{}` Go bodies, param names) and fixtures pin constructs, so each cop disabled in .rubocop.yml says what it would break; run the narrow test for any file you autocorrect there; `-a` is fine on scripts/ and cmd/
govulncheck ./...                   # known vulnerabilities in deps/std the code reaches; needs network, so not part of go test (decision 371)
go run ./cmd/rugaroo run examples/NN_x/main.rb        # transpile, build, run; warnings go to stderr
RUGAROO_CGO=1 go test -run 'TestMinitest/^(fiddle|ffi|sqlite3)$' .   # Fiddle's cgo backend lane instead of purego (decision 426)
go run ./cmd/rugaroo run --int-overflow=promote x.rb  # Integer past 64 bits: raise (default), wrap, or promote to a Bignum; a `# int_overflow: promote` line in the file does the same (decision 372)
go run ./cmd/rugaroo build -work -gcflags=-e examples/NN_x/main.rb  # keep main.go (WORK= path on stderr), show every Go error
scripts/rubyspec-coverage --missing   # ruby/spec core method files the prelude names, per class (#49; clones ruby/spec to the cache)
go run ./cmd/rubyspec -reasons 20 core/integer   # run ruby/spec directories under rugaroo (decision 127); `core` runs each core/* dir, -v lists every skip; the pinned checkout lives in `os.UserCacheDir()/rugaroo-rubyspec`
go run ./cmd/rubyspec library/matrix              # a library spec's gems come from the repo Gemfile (-gems dir; decision 409): add the gem there before measuring it
bundle exec tyda path/to/file.rb        # inferred RBS for unannotated Ruby (dev aid, see below); --diagnostics for JSON Lines
bundle exec scripts/prelude-untyped-audit [--summary]   # the prelude's untyped signatures beside rbs core/stdlib's, with a verdict (#106)
```

`go test` fails fast unless `ruby` is the release `.mri-version` names (decision 413: moving releases is one commit with the Gemfile pins), `bundle install` has been run, and `golangci-lint` is on PATH. Gem executables are not on PATH on this machine, so use `bundle exec rbs-inline ...` for manual runs. Stderr lines like rdoc "already initialized constant" warnings are noise.

## When to use tyda

`tyda` (Gemfile, development group) infers RBS from unannotated Ruby and prints it. It is not part of the build or tests: rugaroo's own inference (decisions 36, 146, 183) is the authority, and the two disagree by design (tyda emits `untyped` where unsure and keeps literal types like `1 | "one"`). Reach for it when:
- annotating a legacy file or a vendored gem (decision 110/160): run it, keep the concrete signatures, widen literals, fill the `untyped` ones by hand.
- a program fails inference and you want a second opinion on a parameter or return type before adding a `#:`.
- an rugaroo inference result looks wrong: if tyda agrees with MRI's behaviour and rugaroo does not, that is a bug to file.
Never feed its output into the compiler automatically, and never loosen rugaroo's typing to match it.

## What a passing example means

For each `examples/*/main.rb`, `TestExamples` (rugaroo_test.go):
1. runs `rbs-inline` + `rbs validate`. A `require "x/y"` line adds `-r x-y`, so annotations must be valid RBS.
2. runs `Compile`, then `gofmt`, `go vet`, `golangci-lint` with `.golangci.generated.yml` (the generated Go must lint clean), and `go build`.
3. requires stdout **and** exit code to equal `ruby main.rb`.

Generated programs are built with `-trimpath`, plus `-race` when they start goroutines (decision 88) (`-trimpath` lets the build cache hit across temp dirs; the first run after a Go upgrade rebuilds the race std once). Examples are vetted and linted in one pass over a shared module after all of them pass. MRI output is cached by source, Ruby version, TZ and checkout directory (so worktrees do not share entries holding `__dir__`).

Add one new example per new feature, numbered next in sequence, always `NNN_name` (three digits): a plain program that shows the feature and prints what it computes (examples are scripts, as in a Ruby project's `examples/`; behaviour checks go in `testdata/test`). User code must run on MRI unchanged. There are no golden Go files; MRI's output is the only expectation.

Smaller cases go in `testdata/`:
- `testdata/test/<area>_test.rb`: minitest files (decision 81), preferred for new behaviour checks. `TestMinitest` runs each with `--seed 1`; MRI must pass (else the test is wrong), then rugaroo's output must equal MRI's apart from timings, and every assertion's operands must equal MRI's (the trace oracle, decision 105: `RUGAROO_MT_TRACE` on both sides, MRI preloading `testdata/mt_trace.rb`; the failure names the test and both operands). So a message rugaroo words differently by design (decision 20's `TypeError`) is checked with `begin`/`rescue` and a class check, not `assert_raises`, which logs the message. Tests must not print. One file per area (`array`, `control`, `number`, ...); all files load together into one program (decision 84), as Ruby loads a suite into one process, so each file keeps its classes and constants inside a `module <Area>Tests` namespace, prefixes its top-level helper defs (`array_ident`: a top-level def is a private method on every object), and reopens core classes only to add methods. Don't name a top-level method `describe` (minitest's Kernel#describe; decision 83). Check the suite loads in one MRI process: `ruby $(ruby -e 'print Gem.bin_path("minitest","minitest")') testdata/test --seed 1`.
- `testdata/run/<area>_*.rb` (print-and-compare; keep for what tests output itself: `puts`/`p`/`print` formatting, exit status, crashes, stdin): `TestRun` gives each file the MRI stdout/exit-code comparison but skips rbs and lint, so each file costs one `go build` (the prelude is pruned to what the file reaches, decision 49; built with `-race -gcflags=-l`, prelude `//line`s pinned to `:1` and no DWARF so the build cache only grows for what an edit reaches, decision 88). A program's fixed cost on a warm run is about 1 s (link + run), its compile 2-50 s, so one file per area (`<area>_output.rb`) keeps the suite readable more than fast: add checks to the area's file (renaming top-level defs, constants and locals that collide, decision 14) rather than a new file. Files that exit non-zero, call `exit`, or need a file-wide magic comment stay standalone.
- `testdata/errors/<area>.txtar`: each `-- name.rb --` is compiled as `main.rb`. `# error: text` lines must all appear in the compile error. `# warning: text` lines must each match a warning. Every case needs at least one of them (a case with only `# warning:` lines must compile); what must compile cleanly belongs in `testdata/test`.
- `# args: a b`, `# env: K=V` and `# stdin: "Go-quoted\n"` (repeatable) feed both MRI and the binary; `# load_path: dir` (repeatable, relative to the file) is `-I dir` for both `ruby` and the compile; `# stderr: match` also compares stderr (only where MRI prints no backtrace).
- `# skip: reason` in either marks a known failure, which is skipped unless `RUGAROO_RUN_SKIPPED=1`. When you fix the bug, remove the line.

## Architecture

- `rugaroo.go`: public `Compile`. It embeds `prelude.rb` + `prelude/*.rb` + `prelude/go/*.go` via `//go:embed`. `prelude.rb` `require_relative`s the core (what MRI defines without a require); a library file is instead mapped from its require name in `internal/compiler/libs.go` and loads only when user code has a literal `require` for it (decision 155), `require_relative`-ing what MRI's own file loads. A new `.rb` prelude file goes in one of those two places; `prelude/go/*.go` files are globbed and always loaded, so a core helper may name a lib's type only inside a single-type `case` (the pruner drops it).
- `internal/compiler`: one `Compiler` holds the closed world (prelude + one user file). Parsing uses Prism via `go-ruby-prism` (WASM/wazero; its compilation cache is `os.UserCacheDir()/rugaroo/wazero`, off with `RUGAROO_NO_WASM_CACHE=1`). Pipeline in `compiler.go`:
  `loadPrelude` → `collect` (declare classes/methods/consts: `collect.go`, types in `model.go`) → `link` (resolve supers, includes, signatures: `link.go`) → `discoverIvars` (dry-run bodies for ivar types: `gen_ivars.go`) → `emitProgram` (`decls.go`, with the emitters in `emit_*.go`) → std-import table + gofmt (`format.go`; no go command needed).
  - `gen.go` and `gen_*.go`: per-function statement codegen (`fctx` in `gen.go`; loops, case, rescue, locals, conditions, analysis, method and main emission each in their own `gen_*.go`; expressions in `expr*.go`). A "tail" (none/return/assign) threads the value-producing position through statements. Type inference works by running gen inside `probe` (output discarded, types recorded) and then running it again for real.
  - `dynamic.go` (and `dynamic_*.go`: wrappers, blocks, send, the runtime text): generated `rbDynName` dispatchers + `DynName(...any) any` wrappers for calls on `untyped` values. There is no `reflect`: everything comes from the closed world.
  - `naming.go`: the operator → Go name table. It must stay injective against camel-cased names (decision 3).
  - Errors are `panic(compileError)` via `errorf`/`unsupported`, recovered in `compile`. Messages carry `file:line`. Warnings (`warn`) are deduped and printed by the CLI/test.
  - When the generated Go doesn't parse, the raw output is written to `$TMPDIR/rugaroo-bad-output.go`.

- `prelude/`: the core library in Ruby; its conventions (`%x{}`, `@go_type`, `prelude/go/` helpers) are in `prelude/CLAUDE.md`.
- `gems/<name>/`: vendored pure-Ruby gems as git submodules, mirrored into `vendored/<name>/` by `scripts/vendor-gems` (committed; `TestVendoredMirror` fails on drift) so the embed works from the module proxy; they run on MRI unchanged and are embedded and compiled as user code when required (`vendoredLibs` in `libs.go`, decision 328); `TestVendoredGemsMRI` runs each one's `test/*_test.rb` under MRI with its lib on the load path. sqlite3-fiddle (github.com/jtarchie/sqlite3-fiddle) is the first; its CI also compiles its test with rugaroo.


### Key representation rules (details in docs/decisions/)

- `T?` → `*T`, except a leaf class's `C?`, which is its `*C` with nil as nil (decisions 359-360). `Opt()`/`Ref()` at `untyped`/optional boundaries.
- Methods on struct classes/modules → free funcs `Owner_Name[Self]` + forwarding methods on each concrete class (Go embedding would bind `self` wrongly); a class body whose signature names no `Self` takes `self OwnerI` instead (decision 210). Module constraints are derived from what the module body calls on `self`.
- Generic methods (RBS `[X]`) are always free funcs.
- Blocks: a method is a Go iterator (`iter.Seq`) only when block and method both return void and the method only yields. Otherwise the block is a closure; a valueless `break` out of a call statement unwinds with a sentinel panic (`rbBreak`, so `ensure` runs; decision 241), and a gem method's block `return` does the same through a per-call `rbReturn` token (decision 247). In user code a non-local `return` from a closure block is still a compile error (decision 4), as is `break v` or a `break` whose call value is used.
- `raise` → `panic`, `rescue` → `defer`/`recover`. Uncaught exceptions print `msg (Class)` and exit 1, like MRI.
- `Hash` is insertion-ordered. `Hash#[]` returns `V?`. `Hash.new(default)` and `Hash.new { |h, k| ... }` are supported (decision 5), but `[]` stays `V?` so a typed caller still nil-checks or uses `fetch`.

## Repo conventions

- Lint: `.golangci.yml` covers the repo (depguard, cyclop, wrapcheck, …). `.golangci.generated.yml` is the same set minus `unused`/`unparam`/`revive` naming, and it applies to transpiler output. Fix codegen instead of loosening it.
- Commit after each green milestone (e.g. a new example passing), in the style `rugaroo: <feature> (example NN)`.
- GitHub issues use three sections: `## Why` (the problem, and what Ruby code fails today), `## What` (a bullet list of the behaviour to build, ending with which example or testdata file proves it against MRI), and `## Notes` (pointers to existing helpers, gotchas, and relevant docs/design.md decisions). Write for a junior PM and a junior engineer: plain words, a short explanation of any Ruby or Go term, and no assumed context. Before filing, check the roadmap issues (#1 stdlib, #2 CLI) so you don't duplicate them, and link #1 or #2 from new issues.
