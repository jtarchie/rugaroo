# Command line

`rugaroo` is shaped like the `go` command. Flags go before the Ruby file;
anything after the file is passed to the program. Tiers are as in
[support.md](support.md).

| Command | Tier | What it does |
|---|---|---|
| `rugaroo run [flags] main.rb [args...]` | stable | Compile, build and run; exits with the program's status. |
| `rugaroo build [flags] [-o prog] main.rb` | stable | Write a binary (default: the file's name without `.rb`). |
| `rugaroo gen [flags] main.rb > main.go` | stable | Print the generated Go. Needs no `go` command. |
| `rugaroo test [flags] [paths...] [minitest flags]` | stable | Compile minitest files into one binary and run it. |
| `rugaroo web [-addr host:port]` | experimental | A local playground: Ruby in, Go out, a Run button. |
| `rugaroo web -static dir -assets URL` | experimental | Write the same playground as static files that compile in the browser. |

## Build flags

For `run`, `build` and `gen`; `test` takes `-I`, `-int-overflow`, `-nil-check`, `-cgo`, `-compiler`, `-race`,
`-gcflags`, `-debug` and `-work`.

| Flag | Tier | What it does |
|---|---|---|
| `-o prog` | stable | `build` only: the output binary. |
| `-I dir` | stable | A directory `require` searches for your own files, as `ruby -I`. Repeatable. |
| `-gem dir` | experimental | A directory of library code the program carries (a framework's runtime, a vendored gem), compiled as an installed gem is: only what the program reaches, leniently, its `sig/` beside a `lib/` read. `-I` is for your own code, compiled whole and strictly. Repeatable ([decision 406](decisions/406.md)). |
| `-int-overflow mode` | stable (`raise`, `wrap`), experimental (`promote`) | What an Integer result past 64 bits does: `raise` a `RangeError` (the default), `wrap` around as two's complement, or `promote` to a Bignum as MRI does. A `# int_overflow: mode` line in a file does the same for the arithmetic written in that file, whatever the flag says (decision 381). |
| `-nil-check mode` | experimental (`runtime`) | A possibly-nil value (`T?`) where a `T` is expected: `compile` makes it a compile error until you check it (the default), `runtime` compiles it to a check that raises when it is nil, MRI's own `TypeError`/`ArgumentError` at a core operator (`"x" + nil`, `1 + nil`, `1 < nil`). A `# nil_check: mode` line in a file does the same for the code written in that file (decision 393). Under `compile`, a read that only an unannotated `[]`/`{}` rugaroo typed itself makes possibly-nil is still checked at run time, with a warning (decision 440). |
| `-cgo` | experimental | Fiddle, FFI and sqlite3 call C through cgo instead of purego, so purego is not linked; needs a C compiler at build time. ([decision 426](decisions/426.md)). |
| `-compiler gc\|tinygo` | experimental (`tinygo`) | The Go compiler that builds the program: `gc` (the default, the `go` command) or [TinyGo](https://tinygo.org), which needs `tinygo` on PATH and makes binaries several times smaller, builds several times slower and runs many-thread programs poorly. Under `tinygo`, `-race`, `-gcflags` and `-pgo` are errors, and so is a `require` of `socket`, `net/*`, `webrick`, `open-uri`, `open3`, `fiddle`, `ffi`, `sqlite3` or `prism`; no Ruby backtraces (`RUGAROO_BACKTRACE`), and backticks/`system` do not run ([decision 443](decisions/443.md)). |
| `-race` | stable | Build with Go's race detector. |
| `-gcflags flags` | stable | Passed to `go build`, after the default `-dwarf=false`; `-gcflags=-e` shows every Go error when generated code fails to build (please report those). |
| `-debug` | stable | Keep Go's debug info (DWARF) for a debugger such as Delve on the generated Go. By default programs are built without it (`-gcflags=-dwarf=false -ldflags=-w`): smaller binaries and a quarter less compile memory; `RUGAROO_BACKTRACE=1` and panic traces do not need it ([decision 437](decisions/437.md)). |
| `-pgo profile` | experimental | `build` and `run`: a CPU profile (pprof format) recorded from this same program, passed to `go build -pgo` so the Go compiler inlines more at the program's hot call sites (profile-guided optimization; 0-15% faster in our benchmarks). A profile only helps the program it came from, and goes stale as the Ruby source changes. A missing or unreadable file is an error naming it. |
| `-work` | stable | Keep the temporary module (`main.go`, `go.mod`) and print its path as `WORK=...`. |
| `-trace` | experimental | Run the program once under MRI, with the arguments after the file, and compile every file that run loaded. For gems that `require` files by computed name. **The run executes the program.** |
| `-trace-also "args"` | experimental | With `-trace`: one more traced run with these arguments, merged into the same binary, for a program that serves several commands. Repeatable. |
| `-rubyopt opt` | experimental | With `-trace`: an option for the MRI run, as the program's own launcher would pass it. Repeatable. |
| `-main file` | experimental | Compile from a traced entry file (the requires an MRI run loaded) instead of tracing again; `file` is the program that entry runs. |

## `rugaroo test`

Test files are found as minitest finds them (`test_*.rb`, `*_test.rb`,
`spec_*.rb`, `*_spec.rb`); with no path, `test/`. All of them load into one
program, as a Ruby test run loads them into one process. minitest's own flags
pass through: `-v`, `-s seed` (`--seed`), `-n pattern` (`--name`), `-e
pattern` (`--exclude`), `-S kinds` (`--skip`), `--show-skips`.

## `rugaroo web`

| Flag | What it does |
|---|---|
| `-addr host:port` | Listen address (default `127.0.0.1:8080`). The Run button executes code, so keep it on loopback; requests from other hosts and origins are refused. |
| `-static dir` | Instead of serving, write a static playground to `dir`: `index.html` for any web host, plus the files for `-assets`. |
| `-assets URL` | With `-static`: the URL, ending in `/`, that serves `rugaroo.wasm`, `wasm_exec.js` and `examples.json`. |

## Environment variables

For a built program:

| Variable | What it does |
|---|---|
| `RUGAROO_BACKTRACE=1` | Report an uncaught exception as MRI does, with `file:line` and `from` lines. |
| `RUGAROO_GO_STACK=1` | A Go runtime panic turned into a Ruby exception also carries its Go stack. |

For the compiler:

| Variable | What it does |
|---|---|
| `RUGAROO_INT_OVERFLOW=mode` | The same as `-int-overflow`. |
| `RUGAROO_NIL_CHECK=mode` | The same as `-nil-check`. |
| `RUGAROO_CGO=1` | The same as `-cgo`. |
| `RUGAROO_COMPILER=tinygo` | The same as `-compiler tinygo`. |
| `RUGAROO_GEM_WARNINGS=1` | Also print warnings from gem files (hidden by default, since they are not yours to fix). |
| `RUGAROO_UNTYPED_WHY=1` | Each dynamic-call warning also names where the untyped value came from. |
| `RUGAROO_INFER_DEBUG=1` | Show what each round of parameter inference typed and what it could not compile. |
| `RUGAROO_NO_INFER_CACHE=1` | Do not use the inference cache. By default `gen`, `build` and `run` keep the parameter types they inferred in the user cache directory (`rugaroo/infer/`), so the next compile of an unchanged program skips the inference rounds; any changed input file compiles from scratch. The output is the same either way. |

The repository's [CLAUDE.md](../CLAUDE.md) lists the variables for working on
rugaroo itself.
