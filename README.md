<p align="center">
  <img src="site/logo.svg" alt="rugaroo: a rougarou pup wearing a ruby, in front of the moon" width="160">
</p>

<h1 align="center">rugaroo</h1>

<p align="center"><em>Ruby by day. Go by night.</em><br>
Compile typed Ruby to a native Go binary.<br>
<a href="https://rugaroo.dev">rugaroo.dev</a> · <a href="https://play.rugaroo.dev">playground</a></p>

A [rougarou](https://en.wikipedia.org/wiki/Rougarou) is a shape-shifter from
Cajun folklore. This one turns Ruby into Go.

rugaroo reads a Ruby program whose methods carry
[rbs-inline](https://github.com/soutaro/rbs-inline) type annotations
(`#: (String) -> Integer` comments), translates it to a single Go file, and
builds it with the Go toolchain. The result is a static executable with no
Ruby interpreter and no runtime library: Ruby's core classes are compiled in
from a prelude that is itself written in Ruby.

The source stays plain Ruby. Every program rugaroo accepts also runs on MRI
(the standard `ruby`) unchanged, and the test suite holds rugaroo to that: for
each example, the compiled binary's stdout and exit code must match
`ruby main.rb` byte for byte.

> **Status:** the core language, core classes and most of the standard library
> are **stable**: tested against MRI, and a difference is a bug. Gems, `fiddle`
> and a few newer features are **experimental**. It is not a drop-in
> replacement for `ruby`; [docs/support.md](docs/support.md) says what is in
> which tier.

## Example

```ruby
# rbs_inline: enabled

class Greeter
  #: (String) -> void
  def initialize(name)
    @name = name #: String
  end

  #: (Integer) -> Array[String]
  def greet(times)
    (1..times).map { |i| "#{i}. hello, #{@name}" }
  end
end

Greeter.new(ARGV.first || "world").greet(3).each { puts it }
counts = %w[a b a c a b].tally
p counts.max_by { |_, n| n }
```

```console
$ rugaroo run greet.rb Go
1. hello, Go
2. hello, Go
3. hello, Go
["a", 3]

$ ruby greet.rb Go        # same output
```

The class becomes a Go struct, and its methods become ordinary typed Go
(excerpt of `rugaroo gen`, minus `//line` directives):

```go
type Greeter struct {
	name String
}

func Greeter_Greet(self *Greeter, times Integer) *Array[String] {
	return Range_Map[Integer, String](&Range[Integer]{b: 1, e: times}, func(i Integer) String {
		return Integer.ToS(i) + ". hello, " + self._Greeter().name
	})
}
```

## Install

You need **Go 1.26 or newer** on your `PATH`: rugaroo calls `go build` to
produce the binary. Ruby is *not* needed to compile programs (the Ruby parser,
Prism, is embedded as WebAssembly).

```sh
go install github.com/jtarchie/rugaroo/cmd/rugaroo@latest
```

Or with Homebrew (`brew tap jtarchie/rugaroo https://github.com/jtarchie/rugaroo`,
`brew trust jtarchie/rugaroo`, since Homebrew 7 loads casks only from trusted taps,
then `brew install rugaroo`), or a binary from the
[releases page](https://github.com/jtarchie/rugaroo/releases) for Linux and
macOS. Go is still needed either way, since rugaroo runs `go build`.

```sh
rugaroo run main.rb [args...]      # compile, build and run
rugaroo build -o prog main.rb      # write a binary
rugaroo gen main.rb > main.go      # print the generated Go
rugaroo test test/                 # run minitest files as one compiled program
```

New to rugaroo? [docs/getting-started.md](docs/getting-started.md) goes from a
first program to porting your own scripts, with the common compile errors and
their fixes. Or try it in the browser at the [playground](https://play.rugaroo.dev).

## What you can use

| | Tier |
|---|---|
| Classes, modules, blocks, procs, exceptions, pattern matching, `Struct`/`Data`, threads and fibers, Ruby 4.0 syntax, reflection the compiler can answer (`send`, `method_missing`, `const_get`, `instance_variable_get`) | stable |
| Core classes: `String`, `Array`, `Hash`, `Integer`, `Float`, `Range`, `Set`, `Enumerable`, `Regexp`, `Time`, `File`, `IO`, `Kernel`, ... (Array 87%, Hash 89%, String 76% of the ruby/spec examples that run; per class in docs/support.md) | stable |
| Standard library: `json`, `csv`, `date`, `time`, `net/http`, `webrick`, `socket`, `fileutils`, `optparse`, `logger`, `erb`, `bigdecimal`, `rexml`, `minitest` and about 25 more | stable |
| Gems compiled from source (rack, Cuba, pairing_heap; kramdown and liquid through `-trace`), a Rails app transpiled by [roundhouse](https://rubys.github.io/roundhouse/) to its Spinel shape, `fiddle`, `sqlite3`, `prism`, `yaml` (loads by Psych's rules; dump text is not libyaml's), `openssl` (RSA verify, HMAC, PBKDF2), Sorbet signatures, `Ractor`, `--int-overflow=promote`, `--nil-check=runtime` | experimental |
| `eval`, `define_method` in your own code, in-place String mutation, encodings beyond ten | not supported |

[docs/support.md](docs/support.md) lists every feature, core class and
`require` name with its tier and the tests behind it.
[`examples/`](examples/) has one program per feature, each a normal Ruby
script you can run with `ruby`.

## What it can't do

rugaroo compiles a *closed world*: every class and method must be visible when
the program compiles, and the binary has no interpreter. So there is no `eval`
and no `define_method` in your own code (a gem's load-time `define_method` is
compiled in). Strings are immutable Go strings, so `upcase!` is a compile
error and `<<`, `replace` and `setbyte` work only on a local nothing else holds (built fresh, not
stored or passed before its last change; `setbyte` also on an ivar built in `initialize`) or through a method's String parameter it appends to; not through a block parameter (an Array element) or an attribute: build new strings (`map! { |r| r + x }`) or use `StringIO`. Integers are 64-bit and
raise `RangeError` on overflow unless you opt into `--int-overflow=wrap` or
`promote`. `Hash#[]` returns `T?`, so typed code checks for nil or uses
`fetch`, unless you opt into `--nil-check=runtime`. Each of these trade-offs has a numbered decision in the
[design record](docs/design.md); [docs/support.md](docs/support.md#language--not-supported)
lists them with workarounds.

## Performance and conformance

A compiled program does no interpretation. Over the workloads in
[`benchmarks/`](benchmarks/) rugaroo runs about 3.6x faster than MRI;
[BENCHMARKS.md](BENCHMARKS.md) has the table, the comparison columns (YJIT,
spinel, Crystal, hand-written Go) and where rugaroo trails. On spinel's own
73-program suite it builds 65 unmodified (47 by default, 18 more with
`--nil-check=runtime`) and runs them 18.0x faster than MRI (spinel: 23.8x on
the same programs).
A hot program can go a further 0-15% with a profile of its own run
(`-profile-out`, then `-pgo`: [docs/cli.md](docs/cli.md#make-a-hot-program-faster)).

Correctness is measured against MRI: every [example](examples/), minitest
suite and compile-error archive runs against `ruby` under `go test`. As a
wider probe, `cmd/rubyspec` runs [ruby/spec](https://github.com/ruby/spec):
12,217 of its examples pass (9,098 in `core`, 1,937 in `library`, 1,182 in `language`;
71% of the `core` examples that run), and most of the rest are skipped for something rugaroo leaves out by
design. [docs/support.md](docs/support.md#core-classes--stable) has the rate
for each core class.

## Docs

- [Getting started](docs/getting-started.md): install, first program, annotations, warnings, tests, porting errors.
- [Support](docs/support.md): what is stable, experimental or not supported.
- [Gems](docs/gems.md): compiling programs that load gems.
- [Command line](docs/cli.md): commands, flags and environment variables.
- [Design record](docs/design.md): every trade-off, as numbered decisions.

## How it works

1. Parse the prelude and the program with Prism (the official Ruby parser,
   run as WebAssembly via [wazero](https://wazero.io)).
2. Collect every class, module, method and constant into one closed world;
   resolve superclasses, mixins and RBS signatures.
3. Infer local, instance-variable and return types by dry-running code
   generation.
4. Emit Go: generics for containers and mixins, interfaces for virtual
   dispatch, iterators (`iter.Seq`) for yielding methods.
5. Prune everything the program can't reach, `gofmt`, and hand the file to
   `go build`.

The core library is [`prelude/`](prelude/): Ruby source that reopens core
classes, with leaf primitives dropping into Go through `%x{ ... }` bodies.
Generated programs use only the Go standard library and never `reflect`;
calls on `untyped` values go through switches generated from the whole program
([docs/dynamic-dispatch.md](docs/dynamic-dispatch.md)).

## Development

Running the test suite needs Ruby ≥ 4.0 (to produce the expected output),
Bundler and [golangci-lint](https://golangci-lint.run) on `PATH`:

```sh
bundle install          # rbs, rbs-inline, webrick, rackup
go test ./...           # every example and test file, compared against MRI
go test -run 'TestExamples/each/005_word_count' .    # one example
```

There are no golden files: MRI's output is the only expectation. Each
`examples/*/main.rb` must pass `rbs validate`, the generated Go must pass
`gofmt`, `go vet` and `golangci-lint`, and the binary's stdout and exit code
must equal `ruby main.rb`. Behaviour checks live in `testdata/test/*_test.rb`
as minitest files that must pass on both MRI and rugaroo. A cold run can take
more than ten minutes; pass `-timeout 30m`.

```
cmd/rugaroo/          the CLI
internal/compiler/  Ruby → Go: declarations, type inference, codegen
internal/rbs/       the RBS type-syntax subset the compiler understands
prelude.rb, prelude/  the core and standard library, in Ruby (+ Go helpers in prelude/go/)
examples/           one program per feature, each checked against MRI
testdata/           minitest suites, print-and-compare snippets, compile-error cases
docs/               user docs, design record and deep dives
```

[CLAUDE.md](CLAUDE.md) has the full command list and contributor conventions.

## Prior art

[Opal](https://opalrb.com) (Ruby → JavaScript, core library in Ruby with
`%x{}` escapes), [Crystal](https://crystal-lang.org) (Ruby-like syntax,
static types, same 64-bit Integer trade-off), and TruffleRuby/Rubinius (core
written in Ruby over primitives).

## License

[MIT](LICENSE). The minitest port in `prelude/minitest.rb` keeps minitest's
own MIT license ([prelude/minitest.LICENSE](prelude/minitest.LICENSE)).
