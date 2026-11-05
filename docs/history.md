# rugaroo design history

How rugaroo got its shape: the first worked examples and the prelude idea. Historical
record; the numbered decisions in [decisions/](decisions/) win where the two disagree.

## Simplest case

```ruby
"hello, world".upcase
```

```go
type String string
func (s String) Upcase() String { return String(strings.ToUpper(string(s))) }
```

A named Go type over the primitive, methods attached. Zero-cost.

## The class hierarchy

`String` doesn't stand alone: `String.ancestors` is
`[String, Comparable, Object, Kernel, BasicObject]`, and ordinary calls resolve
to every level of it:

```ruby
s = "hello"
s.upcase             # String       → "HELLO"
s < "world"          # Comparable   → true   (String defines <=>, not <)
s.clamp("a", "c")    # Comparable   → "c"
s.then { _1 * 2 }    # Kernel       → "hellohello"
s.equal?(s)          # BasicObject  → true
```

(`Object` defines no methods of its own in MRI — `Object.instance_methods(false)`
is `[]` — everything comes from `Kernel`.)

Example: [000_string_hierarchy](../examples/000_string_hierarchy/) —
[main.rb](../examples/000_string_hierarchy/main.rb).

### The prelude idea

The real question: can the whole object model be *written in Ruby*, with the
transpiler only providing syntax sugar? Yes — [prelude.rb](../prelude.rb) is the
core library, compiled by the same transpiler as user code. Only leaf
primitives drop into Go through an escape hatch.

Prior art: Opal (Ruby→JS, corelib in Ruby, raw JS in `%x{}`), Crystal's prelude
(`@[Primitive]`), TruffleRuby/Rubinius (core in Ruby, `Primitive` leaves).

The prelude is transpiler input only — it reopens core classes, so it never
runs on MRI.

Rules the prelude relies on:

- **`%x{}` escape hatch** — valid Ruby (Prism, rbs-inline, Steep still parse
  the prelude), Opal precedent. Inside, `self` and parameter names bind to the
  Go receiver/args. No `.value` wrapper.
- **Primitives must be annotated** — the Go body is opaque, so the RBS
  signature is the only source of the return type. Pure-Ruby methods can infer.
- **`@go_type`** — tells the transpiler `String` is a named Go `string`, not a
  struct. Classes without it become structs (and are passed as pointers).
  A `@go_type` class has no struct to embed, so subclassing one is a
  compile error — *except* `Hash`, whose subclass is a struct embedding it
  (decision 165).
- **Mixins are just Ruby** — `Comparable#<` and `#clamp` are written once;
  `String` only supplies `<=>`. Compiles to `Comparable_Op_lt[T Comparable_Self[T]]`.
  The constraint interface is *derived from the module body*: whatever the
  module calls on `self` is what an includer must provide. Including
  `Comparable` without `<=>` fails at `go build`, which is the right place.
- **Generics pick the Go shape** — a method with type params (`then`'s `[X]`)
  must be a free func (`Kernel_Then[Self, X]`), since Go methods can't be
  generic. Otherwise emit a method. Derived from the RBS, not hand-picked.
- **Inherited/included methods → free funcs + forwarders.** `Owner_Name[Self]`
  free func, generic over the receiver; each concrete class gets a thin
  forwarding method for the non-generic ones so it satisfies Go interfaces.
  Generic ones (`then`) have no forwarder; call sites call the free func.
- **Bootstrapping** — `Comparable#<` calls `Integer#<`, which must be a
  primitive. Every call chain must bottom out in a `%x{}` leaf. With static
  dispatch, the transpiler can verify this at compile time.
- **No separate runtime** — helpers that aren't methods (e.g. `identical`) go
  in a top-level `%x{}` in the prelude and are emitted verbatim. One source of
  truth, one `//line` map. Imports are added by the `stdImports` table in
  `format.go` (selector name → std package; it replaced `goimports`, which
  needs the go command), so `%x{}` bodies don't declare imports.
  *Amended (decisions 154, 263):* helpers now live in `prelude/go/*.go`, real
  Go with its own import block; the table stays for `%x{}` bodies.
- **Testing** — the prelude can't run on MRI, so it's tested by transpiling and
  running the Go. `examples/000_string_hierarchy/main.rb` is the prelude's
  first test.
- **Error mapping** — `%x{}` bodies aren't type-checked until `go build`. Emit
  `//line prelude.rb:N` directives so Go errors point back to Ruby source.
- **Frozen strings.** `# frozen_string_literal: true` semantics only; no
  `upcase!`/`<<`. That's what lets `String` be a Go `string` value. `equal?`
  (identity) on a value type compares the backing pointer — a hack that only
  works because strings are immutable. Go hands back the input when there
  is nothing to do (`strip`, `sub`, `Repeat(s, 1)`, a lone split part),
  where MRI makes a new String, so a method whose result is its receiver's
  own bytes copies them (`rbNewStr`); only that case pays for a copy, and
  `"#{x}"`, `Symbol#to_s` and `dup` always copy. Other shared bytes still
  read as one object: two equal slices of one string (`s.split("b")[0]`
  twice), `strconv`'s small numbers (`5.to_s`), a `""` whose address Go
  drops when boxing it. Fixing those takes a boxed `String`. `frozen?` uses
  the same identity: a string is frozen if it shares a literal's bytes (the
  compiler emits a table of every literal, `rbStringLits`) or was passed to
  `freeze`; strings built at run time are not, as in MRI. Other values:
  immediates, `Regexp` and `Data` are frozen; objects, `Array`, `Hash` and
  `Struct` are not.
- **Literals need wrapping only for interface targets** — Go's untyped
  constants convert to `String` when the parameter is `String`
  (`s.Op_lt("world")` compiles), but become Go `string` when the parameter is
  `any`. Always emit `String("...")` when the target type is an interface or
  `untyped` (see [006_puts](../examples/006_puts/)), and when the literal is a
  receiver or bound with `:=` (`(-1).abs`, `case 3`, `false && x`), where Go
  would infer `int`/`string`/`bool`.

## Examples

*Historical record: this walkthrough covers the first examples as they were
written (01-32). Later behavior is in the numbered decisions below, which win
where the two disagree.*

Each `main.rb` produces identical output under `ruby` and as transpiled Go.
User code is plain Ruby that runs on MRI, so the transpiler is tested by
diffing against MRI output. The design notes below describe the Go shape
each feature compiles to.

### 01 — Inheritance, `super`, overriding

[main.rb](../examples/001_inheritance/main.rb)

`describe` is defined on `Shape` but must call the subclass's `area`/`name`.

- **Per-class interface.** Every class gets a Go interface of its full method
  set (own + inherited). Methods defined on a superclass take that interface as
  `self`. Go resolves the vtable.
- **Embedding alone is wrong.** Go embedding promotes methods but binds `self`
  to the embedded field: `Square#describe` would print `Rect`. Forwarders are
  re-emitted on every subclass — same rule as the `String` forwarders.
- **`super`** in `initialize` → call the parent's `_Initialize` on the embedded
  field. `new` → generated `NewRect` = allocate + `initialize`.
- **Struct classes are pointers** (`*Rect`). Value semantics only for
  `@go_type` frozen primitives.
- **`Float#to_s`** is a primitive trap: Go prints `6`, Ruby `6.0`. Expect many
  of these in the prelude.

### 02 — Enumerable, blocks, generic containers

[main.rb](../examples/002_enumerable/main.rb)

`Enumerable` written once in the prelude against `each`; Go generics, with
type arguments spelled out from the Ruby types (Go would infer `*Foo` or an
untyped constant from the arguments).

- **`each` → `iter.Seq[E]`** (Go 1.23+ range-over-func). A Ruby
  `def each; ...; yield x; ...; end` body compiles to a push iterator:
  `yield x` → `if !yield(x) { return }`. This is real compiler work, not a
  mapping table.
- **Constraint derived from the module body.** `Enumerable` calls `self.each` →
  `Enumerable_Self[E]{ Each() iter.Seq[E] }`.
- **Go infers `E`** from the constraint method, so call sites don't spell type
  args.
- **Generic methods → free funcs.** `map`'s `[U]` means
  `Enumerable_Map(nums, ...)` at every call site. Verbose, unavoidable.
- **Mutable types are pointers.** `b = a; b << 1` mutates `a` in Ruby →
  `Array` is always `*Array[E]`. Opposite of frozen `String`. Rule: frozen
  `@go_type` → value; mutable → pointer.
- **`&:even?`** resolves statically to a closure. No `Symbol#to_proc` at
  runtime.
- **Non-local return is the hidden problem.** `arr.each { |x| return x if x > 2 }`
  is everyday Ruby; a Go closure can't return from the enclosing func.
  Options: (a) when the receiver is a known `Array`/`Hash`/`Range`, inline the
  block as a `for` loop — no closure, `return` just works; (b) otherwise panic
  with a per-call sentinel and `recover` at the method boundary. Do (a) where
  possible, (b) as fallback.

### 03 — `nil` and optional types

[main.rb](../examples/003_nil/main.rb)

`T?`, `&.`, `||`, truthiness narrowing. The biggest cross-cutting decision: it
affects `Hash#[]`, `find`, `first`, and every `if x`.

- **`T?` → `*T`** for `@go_type` values. `nil` = nil pointer. Struct classes
  are already pointers, so `Rect?` is still `*Rect` — one representation.
- **Narrowing is free.** `if name` → `if name != nil`; inside, `name.Size()`
  works because Go auto-derefs `*String` for value-receiver methods.
- **`&.` and `||` become statements.** Short-circuit expressions need
  temporaries in Go; the transpiler lifts expressions to statement sequences.
  Union `String? | String` collapses to `String`.
- **Truthiness rule:** only `nil` and `false` are falsy. `Boolean` → native
  `!`; `T?` → `!= nil`, except `Boolean?`, which can hold `false` (a
  `Hash[String, bool]` lookup): `rbTruthyOpt(x)`, i.e. `x != nil && bool(*x)`;
  everything else → constant true (warn).
- **Methods on `nil`.** `x.inspect` where `x: String?` → static branch:
  nil → `NilClass_Inspect()`, else `x.Inspect()`. No runtime `NilClass`
  class (`nil.class` is a bare class object, decision 19).
- **RBS core lies about `Hash#[]`.** Core signatures say `(K) -> V` (because
  of defaults); the prelude should declare `(K) -> V?`. `Hash.new(0)` +
  `h[k] += 1` then fails to typecheck — see 05.
- **Alternative rejected:** `Option[T]` struct `{v T; ok bool}`. Uniform, no
  pointer escape, but `if x` → `x.ok` everywhere and doubles every prelude
  signature. `*T` wins on simplicity; revisit only if escapes show in profiles.

### 04 — Exceptions

[main.rb](../examples/004_exceptions/main.rb)

`raise`/`rescue`/`ensure`, user-defined hierarchies. Feasibility proof more than
a design driver.

- **`raise` → `panic`, `rescue` → `defer`/`recover`.** Alternative is Go
  `error` returns: idiomatic, but changes every signature and needs
  raise-analysis. `panic` keeps signatures and matches Ruby unwinding. Slow,
  which is fine — exceptions are exceptional.
- **Class matching via marker methods.** `rescue AppError` must match
  `NotFound`. `r.(*AppError)` fails on `*NotFound`;
  `r.(interface{ isAppError() })` succeeds because embedding promotes the
  marker. Embedding works here (not in 01) because markers never touch `self`.
- **Catch-all `rescue`** also catches Go runtime panics (nil deref, index out
  of range). Aligns with Ruby (`NoMethodError`, `IndexError` are
  `StandardError`s). Wrap Go runtime errors into prelude exception types in the
  recover path.
- **`ensure`** → outer `defer`; LIFO gives rescue-then-ensure. `return` inside
  `rescue` needs a named result. Go's `return`/`break`/`continue` stop at the
  `func(){}` wrapper, so a `return`, `break`, `next` (or an iterator's stopped
  `yield`) that leaves it sets the wrapper's flag and is re-issued after the call.
- **Not shown:** `retry`, bare `raise` re-raise, `backtrace`, `rescue` in
  blocks. `retry` → loop around the `func(){}` wrapper.

### 05 — A real program: word count

[main.rb](../examples/005_word_count/main.rb)

Three mechanism tests prove feasibility; one small real program shows what a
user would actually write, and surfaces what the toys don't.

- **Hash ordering.** Go maps are unordered *and randomized per run*; Ruby
  preserves insertion order. This example happens not to depend on it (the full
  sort key decides), but any program that prints or iterates an unsorted hash
  would have nondeterministic output — which also kills MRI-diff testing.
  Recommended: ordered `Hash` as `map[K]int` index into `[]entry` with
  tombstones (Python-dict style, ~40 lines, O(1) ops). Omitted here
  for brevity.
- **`tally` instead of `Hash.new(0)` + `+= 1`.** The default-value idiom needs
  the transpiler to track hash defaults through types (or accept RBS's unsound
  `V`). `tally` types cleanly. Prelude should still support `Hash.new(default)`
  eventually — it's common.
- **`[-n, w]` sort key is an RBS tuple `[Integer, String]`** → `Tuple2[A,B]`,
  lexicographic `<=>`. Because Go constraints are satisfied by *methods*, each
  tuple instantiation used as a `Comparable` needs a named wrapper type
  (`Tuple2IS`) — generated on demand.
- **`sort_by`** = compute keys once, then sort. Ruby's `sort_by` is unstable;
  `slices.SortFunc` is too. Same contract.
- **`|w, n|` block destructuring** of a pair → `p.F0, p.F1`. Arity-based
  auto-splat resolved from the element type.
- **`first(3)`** exercises early termination: `break` in the `range` loop makes
  `yield` return false and stops the upstream iterator. Lazy chains are cheap.
- **What this surfaced that the toys didn't:** hash ordering, tuples-as-keys,
  wrapper types for generic constraints, interpolation calling `to_s` on every
  part. Expect every real program to add 2–3 prelude items.

### 06 — `puts`

[main.rb](../examples/006_puts/main.rb) ·
[prelude `Kernel#puts`](../prelude.rb)

`puts` is not `fmt.Println`: `nil` → blank line, arrays flatten recursively,
no doubled trailing `\n`, bare `puts` → newline, `to_s` dispatch on user
classes. It's written in the prelude as plain Ruby; only `__write` is a
primitive.

- **`case a when nil / when Array` → Go type switch.** `untyped` → `any`.
  First example that needs a runtime type test — everything before was static.
- **Any other `case` → `switch {}` over `cond === subj`.** A class is
  `is_a?`; a condition whose class defines `===` (`Regexp`, a user class)
  calls it, dynamically when the condition is `untyped` or `T?`; anything
  else is `==`, `Object#===`'s default, so typed `when 0` stays a typed `==`.
- **`when Array` vs. generics.** A Go type switch can't match `*Array[E]` for
  unknown `E`. Every `Array` instantiation implements a non-generic
  `Array_Any{ ToAAny() []any }`; the switch matches that. General rule for
  `when GenericClass`: match a non-generic interface the class implements.
- **`a.to_s` on `untyped`** → `a.(Kernel_ToS).ToS()`. Safe because `Kernel`
  guarantees `to_s` on every object; the prelude must give every `@go_type`
  and struct class a `ToS`.
- **`T?` into `any` needs `Opt()`.** A nil `*String` stored in `any` is a
  *typed* nil, so `case nil` misses it. The transpiler knows the static type
  is `String?` and emits `Opt(words.Index(5))`, converting to untyped nil.
- **Untyped literals into `any` become Go `string`**, not `String` — the
  first run of this example panicked on exactly that. Emit `String("...")`
  for interface-typed targets.
- **Splats into a rest param** → one fresh slice: `f(1, *a)` →
  `f(slices.Concat[[]T]([]T{1}, *a)...)`. Go spreads only a lone slice, and
  Ruby's rest param is a new array, so the callee must not alias the caller's.
  Elements convert (`rbSplat`) when the Go types differ (`Array[Integer]` into `*untyped`).
- **Output is buffered** (`bufio.Writer`), flushed by `defer` in `main` —
  runs on return and on panic, so partial output before a crash matches Ruby.
  `os.Exit` skips defers: `Kernel#exit` must flush first. A terminal
  stdout flushes on every write, and SIGINT/SIGTERM flush (decision 60).
  Interleaving with `$stderr` on a pipe would need `$stdout.sync`-style
  flushing; not handled yet.
- **Return type `nil` → no Go return value.** `puts`'s result is rarely
  used; when it is (`x || puts(...)`, `puts(...).inspect`), the call runs as
  a statement and the value is `nil`. A block whose value is nil
  (`map { |n| puts n }`) binds its type variable to `untyped`.
- **`//line` directive granularity.** The panic trace mapped back to
  `prelude.rb`/`main.rb`, but to the wrong line inside `puts` — one directive
  per function isn't enough; emit one per statement.

### 07–24 — one feature each

Ruby 4.0 syntax (a line starting with `&&`, `it`), control flow, struct
classes, strings, hashes, optionals, exception flow, an uncaught exception,
namespaces and constants, class methods and class objects, symbols and
braceless hash arguments, regexps, JSON, `untyped` and `is_a?`, `||=`,
multiple assignment, threads, and a WEBrick server with a `Net::HTTP`
client in one process.

### 26–31 — reflection, and the Ruby resty leans on

Class objects and constant reflection (26), `extend` and `&block` (27),
`Struct.new` and `Data.define` (28), `method_missing` and `respond_to?` on
typed receivers (29), dynamic `send` on untyped values (30), and everyday
Ruby semantics such as `rescue` modifiers, `return` in `ensure`, and `&&`
returning values (31).

### 25 and 32 — resty: a web framework over `net/http`

[main.rb](../examples/025_resty/main.rb) is a typed port of
[jtarchie/resty](https://github.com/jtarchie/resty), a Rack framework that
forces RESTful conventions. The program mounts it on WEBrick, then drives
it with `Net::HTTP`, replaying resty's own integration specs over a real
socket. On Go, WEBrick and `Net::HTTP` are the prelude's classes over
`net/http`'s `http.Server` and `http.Client`; the printed transcript is
byte-for-byte MRI's.

What survives from resty is its architecture: a request picks its action
class and its format class by asking each candidate *class* `matches?`
(`Actions::ALL.detect { |a| a.matches?(request) }`), then instantiates the
winner through `singleton(Actions::Base)`, and actions find the app's
controller action by `self.class.name`. What had to change is exactly the
anti-goal list above:

- `"#{ns}::#{path.camelize}Controller".constantize` and
  `const_get(action_name)` become a typed registry,
  `Hash[String, singleton(Resty::Action)]`;
- `Actions.constants.map { const_get }` becomes an explicit `ALL` list;
- `NullController`'s `method_missing` becomes a `NullAction` class;
- `Rack::Request` becomes `Resty::Request`, built from WEBrick's request;
  ActiveRecord becomes an in-memory `Resty::Model`;
- idioms whose types are unions (`path =~ re || …` as a `bool`,
  `match(...)[1]` on a possible nil) are spelled with `match?` and a check.

[Example 32](../examples/032_resty_reflective/main.rb) then compiles resty's
own `lib/` code as written, reflection and all: `constantize`,
`const_get`, `constants.map { const_get }`, `NullAction`'s
`method_missing`, `Struct.new ... do`, `extend Enumerable`. It adds type
annotations and changes two lines (marked `rugaroo:`); `Rack::Request`,
ActiveSupport's `camelize`/`constantize` and ActiveRecord are small shims.
Its transcript is the same as example 25's. The compiler warns at each of
its dynamic calls, which is where the program's types run out.
