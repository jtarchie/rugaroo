# Inference with an explicit "not known yet" (plan, 2026-11-16)

The user's direction (2026-11-16): a better inference algorithm in the compiler; MRI tracing stays a development
aid, never where a production program's types come from. This is the plan and its gates. It becomes a decision file
when the first gate passes; until then nothing here is true of the compiler.

## What is wrong with the rounds

Typing is code generation run in `probe`, so there is one set of typing rules and it must stay that way: a second
solver would re-implement every rule. What needs to change is the lattice and the driver around it.

- **Untyped stands for two things.** `TAny` is both "this value is untyped" (a declared `untyped`, a mixed join, a
  computed send) and "nothing has typed this yet" (a parameter in the round before a caller is typed, a method's
  return while its parameters wait, an ivar discovery has not reached). `join` lets it absorb everything, so the
  second meaning poisons like the first. `expr.provisional`, `Method.roundRet`, `roundBottomReturns`, `awaitsParams`,
  `typedThisCall`/`hardAny` (decision 466) and `provisionalIvarValue` (decision 383) each exempt one shape.
- **The state is rebuilt every round.** `updateInference` computes `types` and `none` from that round's uses alone, so
  a parameter can be typed in round 3, untyped in round 4 and typed again in round 5. rubocop's rounds never
  converge; `settledRounds` stops them when few parameters change, and `anyUse` (decision 444) untypes what is left.
- **A cap of 8 whole-program rounds**, and a type travels one call level per round.

spinel's notes on its own fixpoint (`docs/internals/fixpoint-convergence.md` in its tree) say the same from the other
side: a pass must be idempotent on its own output, and a round must not discard what it could not re-derive.

## The algorithm

1. **`unknown` is a type**, below every other: `join(unknown, T) = T`. It is `any` in Go, so emission is unchanged;
   only the lattice tells it from untyped. A parameter typed by use that no call has typed yet is `unknown` during
   the rounds, as is the return of a method being inferred and an ivar not yet discovered.
2. **It propagates.** A call on an `unknown` receiver answers `unknown`; an operation on one answers `unknown`. In a
   round such a call records no dynamic use, warns of nothing and is no error. An argument that is `unknown` is not a
   use: it neither types nor untypes the parameter it is passed to.
3. **The state only goes up.** Each slot's type after a round is the join of its type before and what the round
   derived. A slot can go `unknown` → `T` → `T | U` or untyped, never back. The rounds end when nothing changed, which
   a finite lattice guarantees; the cap stays as a safety net and reaching it is a test failure.
4. **Every slot is in the state**: method parameters, block and proc parameters and results, returns, ivars.
5. **Once at the end**, what is still `unknown` is untyped (nothing in the program gives it a type), and the final
   compile finds nothing to widen.
6. Whole-program rounds first. Incremental rounds (only the methods whose inputs changed) afterwards, required to
   produce byte-identical output.

What it does not reach: a parameter only dynamic calls pass a value to (`cop.public_send(callback, node)` with the
name in a variable). That is the second phase, and it needs its own measurement first.

## Gates, in order

1. `cyc` extended with a lambda aliased through a second ivar (the parser lexer's `@num_xfrm = @emit_integer`) fails
   to type today; it, `buf` and `rg` (the session's repros, to become `testdata/run/gem_lenient` fixtures) type with
   the special cases of decisions 358, 383 and 466 deleted.
2. The parser gem on 22 files reaches the `Parser_Lexer` field types `sig/gems/parser`'s lexer part gives it, and
   about 1,780M instructions, with that part of the file removed.
3. Gen-compare of `examples/`, `testdata/`, `benchmarks/` and spinel's suite. A strict-mode program that newly
   compiles, or whose types change, goes to the user before it lands.
4. `overrides/rubocop/observe.rb` as an oracle only: every class MRI observed for a parameter fits the type inferred
   for it.
5. rubocop `check` SAME on the session's inputs; transpile time and `TestCompileCost` no worse.

The algorithm sits behind an environment switch until gate 5, and the inference cache's key carries its version.
