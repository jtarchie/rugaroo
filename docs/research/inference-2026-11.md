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

## What it can reach (rubocop, measured 2026-11-16)

`RUGAROO_TIMING=1` on the rubocop transpile, the last round, of about 14,000 parameters typed from use:

| | parameters |
|---|---:|
| typed | 1,692 |
| untyped, with a static call that passed something untyped | 8,712 |
| untyped, with dynamic calls only (`send`, an untyped receiver) | 350 |
| untyped, with no call in the program | 3,247 |

The 8,712 are what this plan is for: each has a call the compiler resolved, whose argument was untyped when it was
read. The 350 are out of its reach. The typed count also shows the rounds going back: 1,603, 1,938, 2,050, 2,082,
2,094, then 1,755 and 1,692 once `settledRounds` and `anyUse` apply.

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

## What was built, and the gates (2026-11-21)

Branch `infer-unknown-resolve`. Everything here is behind `RUGAROO_INFER_UNKNOWN=1` unless it says default lane.
The default-lane part is decision 474.

**The schedule is three phases, not one resolution step.** Each runs to its own fixed point; the state only goes up.

1. *Types only.* A round's untyped result for a slot is dropped, and so is a container of untyped
   (`Hash[untyped, untyped]`): it may be a value whose type arrives a round later (an ivar discovered late, a Hash
   whose values are typed in round 2), and kept it would untype for good everything it reaches. This is what the plan
   above did not have. With "the state only goes up" alone, one early untyped value in the parser lexer
   (`@source_pts` in rounds 1 and 2, before `source_buffer=`'s parameter was typed) made `pe`, then `p`, then every
   position passed on, untyped for the rest of the compile.
2. *Settling.* Untyped results are kept; values may still be not known yet. A parameter that really is untyped now
   makes its calls dynamic, and those calls' arguments type what only they reach (liquid's `Tag#render(context)`,
   reached through `node.render(context)` on an element of the untyped nodelist).
3. *Resolved.* Nothing is "not known yet"; what was is untyped. An untyped gem argument, static or through a dynamic
   call by name, untypes the parameter it may reach for good. Without this a parameter typed in phase 1 from one
   early call kept its type and raised at run time (parser's `loc(token)`, typed Integer, handed an Array).

**What "not known yet" covers now**, beyond the plan's parameters and returns: a lambda's parameters; an ivar that is
only ever nil or has no typed write yet; the parts of a destructured value; any call on a receiver not known yet
(an intrinsic such as `class` answered a plain untyped value); what ivar discovery reads from a class it has not
walked yet, in every compile and not only in a round (three walks instead of two).

**New evidence rules.**

- Dynamic calls that all pass one class type a parameter no static call reaches (`dynUsesAgree`).
- Lambdas one slot holds (`@xfrm = flag ? @plain : @skip`, or two writes of one ivar) share their parameter keys
  during the rounds, so the slot is one Go func type and not a union only a dynamic call can call (`procAlias`).
- A `*args` forwarder passes positional types on (`infer_rest.go`): each call records its rest arguments by
  position, and `Literal.new(self, *args)` hands position j's type to the parameter it lands in, when the body only
  ever splats `args` and no call splatted into it or passed that position something untyped.
- `self` passed from a method generic over Self is a use of its class (`LexerStrings.new(self, @version)`).
- The per-shape exemptions of decisions 358, 383 and 466 are off under the switch (`RUGAROO_INFER_SPECIAL=1` keeps
  them, for an A/B).

**Gate 1: passes.** `testdata/run/gem_inference` (now with `lexen_gem` and `fwd_gem`): 19 dynamic calls by default,
0 with the switch, output equal to MRI in both lanes.

**Gate 2: passes.** The parser gem on 20 files (`~/.cache/rugaroo-parser`), the `Lexer` and `LexerStrings` parts of
`sig/gems/parser/parser.rbs` removed:

| | dynamic-call warnings | instructions |
|---|---:|---:|
| MRI | | 2,001M |
| main, with the sig | 2,374 | 1,269M |
| switch on, without the lexer sig | 2,237 | 1,233M (measured one change before the last; see the note below) |

The lexer's own dynamic calls go 953 → 117 without the sig, its untyped fields 23 → 5 (`@version`, `@_lex_actions`,
`@force_utf32`, `@cond`, `@cmdarg`). Both compiled programs print 5,651 nodes and 11,332 tokens; MRI prints 5,750
and 11,533 on the same files, on main too, which is not this work's and is not explained yet.

**Gate 3: run; two things for the user.** `scripts/gen-compare` over 345 inputs (examples, testdata, benchmarks,
spinel's suite). Default lane, main against the branch: only this work's own fixture and test differ. Switch off
against switch on: 14 inputs differ, none newly compiles and none newly fails. The gem programs change as meant
(liquid 469 → 392 dynamic calls, kramdown 1,165 → 1,112, pairing_heap's one dispatcher gone). Of the strict
programs, `bm_micro_lisp` and `bm_tree_walker_frames` fail on both with a different message, and two others
(`bm_stark_field`, `bm_jekyll_lite`) had gained three dispatchers each before phase 1 also deferred containers of
untyped; that is fixed, and the comparison was rerun.

**Gates 4 and 5 (rubocop): not run.**

**Known costs.** Rounds: the parser program takes 16, liquid 21, where the default lane stops at 8 or earlier. The
cap is 48. Rubocop's rounds are about 16 s each, so incremental rounds (plan step 6) come before the switch can be
the default there.

## Techniques other tools use, checked against this design (2026-11-21)

A read of TypeProf (v1 and v2), pyrefly, pytype, `dmypy suggest`, Shed Skin and Crystal for what they do where
rugaroo still answers untyped. Sources were read through a summarising fetch, so constants are worth a direct look.

- **Heterogeneous containers** (kramdown's `Element#children`, parser stacks of `[node, src, flags]`). TypeProf v2's
  Array type carries one slot per literal index beside a unified element type, and its Record type one slot per
  Symbol key; a write at a non-literal index or a push goes to the unified type. As TypeProf does it this is not
  sound for a compiler (a literal-index read still sees only the slot): the rule here would be to demote the tuple
  to its unified type on any non-literal write, push or escape. This is issue #122's tuple stacks.
- **Dispatch by computed name.** TypeProf v2 gives a Symbol literal a singleton type and lets it flow into
  `send`; an interpolated name is a plain Symbol and resolves nothing. No tool narrows the argument per target
  method, so rubocop's `public_send(callback, node)` correlation is not solved anywhere that was read. Decision
  353's prefix match already goes further than TypeProf.
- **Cycles.** TypeProf v2 is a monotone graph with a worklist, "not known yet" being an empty vertex. It reaches the
  same fixed point as rounds and only reschedules less; termination still needs a finite lattice. Pyrefly and pytype
  collapse a recursive lookup to Any, so neither helps. The worklist is the route to incremental rounds.
- **Context sensitivity.** Shed Skin's Cartesian Product Algorithm clones a function per tuple of argument classes,
  with a hard limit and a merge by signature. It would type a helper called with different classes from different
  sites; its cost is code size, which the Go build is already bound by.
- **`*args` and stored procs.** TypeProf v2 keeps a rest parameter as a tuple of the call's argument vertices and
  reads `elems[i]` at a splat: the same idea as `infer_rest.go`.
- **Not borrowable:** `dmypy suggest` (guess and check), pyrefly's first-use pinning of an empty container (a
  checker's heuristic; a compiler needs every use).

In order of what it would buy here: tuple and record slots with demote-on-escape; Symbol-literal types flowing into
`send`; a worklist for incremental rounds; per-argument-tuple clones with a limit.
