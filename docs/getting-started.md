# Getting started

rugaroo compiles a Ruby program into a single Go file and builds it into a
native binary. The program stays plain Ruby: it runs on `ruby` unchanged, and
the compiled binary prints the same output. This guide takes you from install
to porting your own scripts. For what is and is not supported, see
[support.md](support.md).

## Install

You need Go 1.26 or newer on your `PATH`; rugaroo calls `go build` for you.
Ruby is not needed to compile (the Ruby parser is built in), only to compare
output or to compile programs that load gems.

```sh
go install github.com/jtarchie/rugaroo/cmd/rugaroo@latest
```

Or with Homebrew (`brew tap jtarchie/rugaroo https://github.com/jtarchie/rugaroo`,
`brew trust jtarchie/rugaroo`, since Homebrew 7 loads casks only from trusted taps,
then `brew install rugaroo`), or a binary from the
[releases page](https://github.com/jtarchie/rugaroo/releases) for Linux and
macOS. Go is still needed either way, since rugaroo runs `go build`.

To try it without installing anything, use the [playground](https://play.rugaroo.dev).

## Your first program

Save this as `words.rb`:

```ruby
# rbs_inline: enabled

module Words
  #: (String, Integer) -> Array[[String, Integer]]
  def self.top(text, n)
    text.downcase.scan(/[a-z']+/).tally.sort_by { |w, c| [-c, w] }.first(n)
  end
end

if __FILE__ == $0
  Words.top($stdin.read, 3).each { |w, c| puts "#{c} #{w}" }
end
```

```console
$ echo "the cat and the hat and the bat" | rugaroo run words.rb
3 the
2 and
1 bat
$ echo "the cat and the hat and the bat" | ruby words.rb     # same output
```

The three commands you will use most:

```sh
rugaroo run words.rb [args]     # compile, build and run
rugaroo build words.rb          # write the binary ./words (-o to name it)
rugaroo gen words.rb > main.go  # print the Go it compiles to
```

`gen` is worth a look: your methods become ordinary typed Go functions.
`Words.top` becomes a function taking a `String` and an `Integer` and returning
`*Array[Tuple2[String, Integer]]`, with the block as a Go closure. The prelude
(rugaroo's core library, itself written in Ruby) comes along in the same file,
pruned to what your program uses.

### What your Ruby becomes

| Ruby | Go |
|---|---|
| `Integer`, `Float`, `String`, `Symbol` | named Go types over `int`, `float64`, `string` (values, with methods) |
| `true` / `false` | one `Boolean` type |
| `T?` (maybe `nil`) | `*T`; `if x` narrows it |
| `A \| B` (one of a few classes) | Go `any`; each call is a type switch with a typed call per member, and `is_a?`/`case` narrow it |
| `Array[T]`, `Hash[K, V]` | generic Go types; `Hash` keeps insertion order like Ruby |
| a class | a struct, plus an interface so subclasses dispatch correctly |
| a module | generic free functions constrained by what the module calls on `self` |
| `raise` / `rescue` / `ensure` | `panic` / `recover` / `defer` |
| `Thread`, `Queue`, `Mutex` | goroutines, channels, `sync` |
| `untyped` | Go `any`, with calls dispatched through generated switches (the compiler warns at each one) |

Compile errors, warnings and run-time errors all point at `file:line` in your
Ruby source.

## Do I need type annotations?

Less than you might think. rugaroo infers the types of locals, block
parameters, instance variables and return values, and it types a method's
parameters from the calls the program makes to it. This compiles as it is:

```ruby
class Point
  attr_accessor :x, :y
  def initialize(x, y)
    @x = x
    @y = y
  end
  def to_s = "(#{x}, #{y})"
end

pt = Point.new(1, 2)
pt.x = 5
puts pt
```

Annotate when the compiler asks (a warning names the spot), and on methods
that are your program's public surface, so a wrong call fails to compile
instead of misbehaving. Annotations are [rbs-inline](https://github.com/soutaro/rbs-inline)
comments, so `ruby` ignores them. Put `# rbs_inline: enabled` at the top of
the file, then:

```ruby
#: (String, ?Integer) -> Array[String]    # a method's signature (? = optional)
def split_words(text, limit = 10) = text.split.first(limit)

# @rbs name: String                        # one parameter, when the rest infer
@cache = {} #: Hash[String, Integer]       # an assignment's type
attr_reader :name #: String                # an attribute's type
nums = data["a"] #: Array[Integer]         # what an untyped value holds
```

`T?` means "T or nil". `untyped` means "anything": it always compiles, and
calls on it are checked while the program runs. Signatures can also live in
`.rbs` files in a `sig/` directory beside the program, and Sorbet `sig` blocks
work when the program requires `sorbet-runtime` (experimental).

## Reading warnings

A warning never stops the build. Each one points at a line and usually says
how to silence it.

- **`dynamic call: [] on untyped`**: the compiler doesn't know what the value
  is, so the call is dispatched while the program runs (through generated code,
  not reflection). It works, but a mistake surfaces as a run-time
  `NoMethodError` instead of a compile error, and it is slower. Values from
  `JSON.parse` and `YAML.load` are the common source; annotate what you take out of them
  (`nums = data["a"] #: Array[Integer]`).
- **`method add's parameter x is untyped: no call in the program gives it a type`**:
  nothing calls the method with a typed argument, so inference had nothing to
  go on. Add a signature, or `# @rbs x: T`.
- **`instance variable @v is only ever nil, so it is untyped`**: the same, for
  an instance variable; declare it with `# @rbs @v: T`.
- **`+ called on possibly-nil Integer (raises NoMethodError on nil)`**: the
  value can be nil (`Hash#[]`, `Array#find`, `first`), and the call raises if it
  is, as in Ruby. Use `fetch`, a default (`h["a"] || 0`), or a check.
- **`possibly-nil Float? where Float is expected is checked at run time`**:
  an unannotated `[]` or `{}` was typed by what the program puts in it, and a
  read from it (`x > @box[i]`) can be nil. Instead of making the container
  untyped, rugaroo checks that read when it runs and raises MRI's error on nil
  ([decision 440](decisions/440.md)). A check or `fetch` removes the warning.

## Testing

Minitest files compile too. `rugaroo test` builds every test file in a
directory into one binary and runs it, with minitest's flags:

```ruby
# test/words_test.rb
require "minitest/autorun"
require_relative "../words"

class WordsTest < Minitest::Test
  def test_top
    assert_equal [["the", 2], ["cat", 1]], Words.top("The cat the", 2)
  end
end
```

```sh
rugaroo test test/                      # like `minitest test/`
rugaroo test test/words_test.rb --seed 1 -v
```

## Libraries and gems

`require` works for most of the standard library, listed with its tier in
[support.md](support.md#standard-library). Your own files load with
`require_relative`, or `require` plus `-I dir` (as `ruby -I`), as long as the
path is a string literal at the top level of a file.

Gems are experimental: rugaroo compiles a gem's source into the binary, which
needs `ruby` on `PATH` at build time and *runs the gem's load phase* while
compiling. See [support.md](support.md#gems--experimental) for which gems are
tested and the `-trace` route for gems that load files by computed name.

## Porting a Ruby script: common errors

Every message below is what rugaroo prints today, prefixed by `file:line`.

| Message | Why | Fix |
|---|---|---|
| `possibly-nil Integer? where Integer is expected; check it first` | The value can be nil (`find`, `Hash#[]`, `first`) and the method you pass it to takes a non-nil one. | `if x`, `return unless x`, `x ||= default`, or `fetch`. |
| `String where Integer is expected` | A typed parameter got the wrong type. | Fix the call, or widen the signature (`Integer \| String`). |
| `undefined method upcse for String` | A typo, or a method rugaroo's prelude doesn't define (check [support.md](support.md)). | Fix the name. |
| `undefined method << for String: Strings are immutable in rugaroo; ...` | `upcase!`, `gsub!` and `[]=` would change a String in place, and rugaroo's Strings are immutable. | `s += "x"`, `s = s.upcase`, or build into a `StringIO` or an Array and `join`. |
| `undefined method << for String: Strings are immutable in rugaroo, and `s` is passed to keep on line 8 before its last append` | `<<` works on a local only when nothing else can hold its String while it grows (decision 382); the message names what could. | Pass or store the String after the last `<<`, build it in a fresh local (`out = +""`) instead of a parameter, or return it and append in the caller. |
| `undefined method << for String: Strings are immutable in rugaroo, and `r` is a block parameter, ...` | `rows.each { \|r\| r << x }` would change Strings an Array holds; a block parameter gets a copy in rugaroo, so the Array would not see the append (decision 58). | `rows.map! { \|r\| r + x }`, or build the rows into a `StringIO`. |
| `undefined method setbyte for String: Strings are immutable in rugaroo, and `s.[]` may share the String's bytes on line 4 before its last change` | `setbyte` writes in place, so before the last one the String may only be read for numbers (`getbyte`, `bytesize`, `==`); a slice, copy or interpolation could share its bytes (decision 395). On an ivar it works only in `initialize`, before `self` escapes. | Take slices and copies after the last `setbyte`, or `dup` into a fresh local, `setbyte` that, and return it. |
| `undefined method eval for Object: rugaroo compiles the program ahead of time and the binary has no interpreter, ...` | No `eval`. | Write the code out. |
| `define_method is not supported` | Methods must exist at compile time. | `def`, or `method_missing` with a signature. |
| `non-local return from a block is not supported` | A `return` inside a block that is stored as a closure (`map`, `select`, ...) can't unwind the method in Go. | Use `find`/`any?`/`each_with_object`, or loop with `each`, where `return` works. |
| `break with a value is not supported` / `next with a value is not supported` | `break v`, and `next v` where the value is thrown away. | Compute the value first (`find`, `index`), then plain `break`/`next`. |
| `blocks on untyped receivers are not supported` | A block passed to a method called on an `untyped` value. | Give the receiver a type: a signature on the method it came in through, or `list = value #: Array[Integer]`. |
| `overloaded signatures (#\|) are not supported` | Inline overloads. | Put them in a `.rbs` file under `sig/`. |
| `require_relative must be a top-level statement: rugaroo loads files at compile time` | A `require_relative` inside a method or condition. | Move it to the top of the file. |
| `encoding ISO-2022-JP is not supported` | 60 encodings are known; the others are listed in [support](support.md#encodings) with why. | Convert at the program's edges. |

## When rugaroo is not the tool

Programs whose shape is decided while they run: building classes from data,
`eval`ing configuration, `define_method` loops, mutating strings in place
throughout. A framework built on that (Rails, for one) is out of reach for now,
though a Rails app that [roundhouse](https://rubys.github.io/roundhouse/) has
transpiled to plain Ruby compiles ([example 135](../examples/135_roundhouse_blog/README.md)).
Scripts, command-line tools, data processing, servers on `net/http`/`webrick`,
and libraries with clear types are the sweet spot.
