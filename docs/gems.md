# Gems (experimental)

rugaroo has no gem loader at run time: a gem is compiled into the binary from
its Ruby source, like your own files. Gem support is **experimental**
([decision 373](decisions/373.md)). It works on the gems listed below, and a gem
that is not listed may well fail to compile.

**Compiling a program that loads a gem runs Ruby code.** rugaroo runs the
gem's load phase (or, with `-trace`, the whole program) under `ruby` at build
time, with your environment and no sandbox, to learn what metaprogramming did
([decision 161](decisions/161.md)). Only compile code you would run. The run is
bounded (10 minutes, capped output), and programs without gems never start
`ruby`.

The binary itself needs neither Ruby nor the gems.

## Which route to use

| Your program | Route | Tested with |
|---|---|---|
| `require`s a pure-Ruby gem by a literal name | [Installed gems](#installed-gems) | rack, rackup, Cuba, pairing_heap |
| loads files by computed name (`require "x/#{name}"`), or builds classes at load time from data | [`-trace`](#-trace) | kramdown, liquid, Homebrew's `brew` |
| needs a gem with a C extension | Not supported, unless rugaroo carries a pure-Ruby replacement ([vendored gems](#vendored-gems)) | sqlite3, prism |

## Installed gems

```sh
gem install pairing_heap          # or a Gemfile + bundle install beside the program
rugaroo run main.rb
```

A `require` that names neither your own file nor a standard library is looked
up as an installed gem, through Bundler when a `Gemfile` sits beside the
program. `ruby` must be on `PATH` at build time. rugaroo runs the gem's load
phase once under MRI, records what metaprogramming produced (methods from
`define_method`, `attr_*` built in loops, constants), and compiles the gem's
methods with those results written in. Only gem methods the program can reach
are compiled.

Warnings inside gem files are hidden; `RUGAROO_GEM_WARNINGS=1` shows them.

Proof: [examples 115](../examples/115_rack_server/main.rb) (rack),
[116](../examples/116_cuba_web/main.rb) (Cuba),
[120](../examples/120_pairing_heap/main.rb) (pairing_heap).

## `-trace`

```sh
rugaroo build -trace main.rb sample-args...      # runs `ruby main.rb sample-args...` once, then compiles
rugaroo build -trace -trace-also "other args" main.rb args...   # one binary for several commands
```

Some gems choose their files while running: kramdown loads its converters with
`require "kramdown/converter/#{name}"`, and Homebrew loads `cmd/#{command}`.
`-trace` runs the program once under MRI with the arguments you give, records
every file it loaded, and compiles exactly those. The arguments should exercise
the paths the binary needs: a file the traced run never loaded is not in the
binary. `-trace-also` adds more runs, `-rubyopt` passes options to the MRI run,
and `-main` compiles from an entry file a trace wrote ([cli.md](cli.md)).

The traced run executes the whole program, side effects included.

Proof: [examples 123](../examples/123_kramdown/main.rb) (kramdown, with its
rexml dependency) and [124](../examples/124_liquid/main.rb) (liquid), which
compile unmodified and match MRI byte for byte; Homebrew's `brew info` and
`brew install wget` (below).

## Roundhouse output

[Roundhouse](https://rubys.github.io/roundhouse/) transpiles a Rails app into
plain Ruby with `.rbs` files beside each `.rb` (`roundhouse --target spinel`),
for the Spinel compiler. rugaroo compiles that output unmodified: the `.rbs`
files type it, and the few files that call C through Spinel's FFI (its HTTP
server, its sqlite binding) are replaced by Ruby shims that the program loads
first, marking the originals loaded with
`$LOADED_FEATURES << File.expand_path("blog/runtime/db.rb", __dir__)`. The
shims are the [roundhouse-rugaroo](https://github.com/jtarchie/roundhouse-rugaroo)
gem, whose README walks from `rails new` to a running binary.

Roundhouse's `runtime/` is a framework the app carries, not the app's own code,
so it is compiled as an installed gem is (only what the app reaches, leniently):
`rugaroo build -gem blog/runtime main.rb` ([decision 406](decisions/406.md)).

Proof: [example 135](../examples/135_roundhouse_blog/README.md), roundhouse's
Blog app (articles, nested comments, validations, partials, JSON views) served
over HTTP, its answers equal to MRI's on the same files
([decisions 404](decisions/404.md), [405](decisions/405.md)).

## Vendored gems

A gem whose real version is a C extension can be replaced by a pure-Ruby gem
that rugaroo carries and compiles as your code. Today that is `sqlite3`: the
sqlite3 gem's API from [sqlite3-fiddle](https://github.com/jtarchie/sqlite3-fiddle),
which calls libsqlite3 through `fiddle` (no cgo). The program writes
`require "sqlite3"` as usual and runs unchanged on MRI with the real gem. The
machine that runs the binary needs libsqlite3. Proof:
[example 122](../examples/122_sqlite3/main.rb).

`require "prism"` is bundled too: the prism gem's own Ruby (1.9.0), compiled
as an installed gem is, with its C extension replaced by the prism.wasm rugaroo
itself parses with. No installed gem or `ruby` is needed. Proof:
[example 136](../examples/136_prism_parse/main.rb)
([decision 427](decisions/427.md)).

## Large programs: Homebrew

[`overrides/brew`](../overrides/brew/README.md) compiles Homebrew's `brew` and
compares its output with MRI's. It is the proving ground for the routes above
and the pattern for any large upstream program: pin the upstream commit, trace
it, and keep a short series of upstream-shaped patches for the constructs
rugaroo will not compile (30 to Homebrew, 2 to its vendored gems, each one
unchanged in behaviour under MRI). It is a maintainer harness, not a
workflow we support for users.

## When a gem doesn't compile

1. Run with `RUGAROO_GEM_WARNINGS=1` to see what rugaroo skipped or could not
   type in the gem's files.
2. Check whether the gem ships RBS signatures in a `sig/` directory; rugaroo
   reads them.
3. Try `-trace` if the gem loads files by computed name.
4. Open an issue with the gem's name and version and the first error.
