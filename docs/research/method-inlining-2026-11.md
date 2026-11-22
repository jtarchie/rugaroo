# Inlining small leaf methods at the call site (research, 2026-11)

*rugaroo at 73664191; spinel at 14f3ade5; Go 1.27.4. "(inferred)" marks what I reasoned but did not read or run. Written by a read-only planning agent; nothing here is built. It would be a new decision (488 is taken by the runtime hot-spot fixes).*

## Summary and recommendation

Do both, in this order.

1. **Fuse checked arithmetic so Go inlines small methods itself.** This keeps every frame and needs no flag.
2. **Paste the methods that still don't fit** with a Go-AST pass after pruning (decision 402's mechanism). It is on by default, with a permanent opt-out flag, and pastes only where Go refuses to inline.

Fitting Go's budget alone is not enough. The best exact open-coded form of `Vec2#length_sq` costs 95, and `dot` costs 115 (measured below). The hand paste took `bm_inline` from 1,210M to 710M instructions (−28% wall).

The one observable change: an exception raised inside a pasted body loses that method's frame.

## The cheaper alternative, measured

**Sources.** jub0bs.com, "make this Go function inlinable" (2025-04-30, as reported to me):
- The budget is 80, and a function cannot be forced inline (Go issue #21536).
- PGO raises the budget for hot call sites.
- The cost model is unspecified and can change between releases.
- Its biggest lever was hand-inlining helpers that were each inlinable (121 → 88), because a call to an inlinable helper is charged the helper's whole body.

That is exactly our case: `Op_mul` costs 60 and `Op_plus` 26 (`prelude/integer.rb:52-92`). The constants are in `$GOROOT/src/cmd/compile/internal/inline/inl.go:50-59`: budget 80, a call 57, a panic 1, and the "big function" limits of 5,000 nodes and cost 20.

**How I measured.** `go tool compile -m=2` on scratch Go fed through stdin. It used today's `Op_mul` and `Op_plus` bodies, with `rbIntWrap` as a false constant.

| Shape (exact overflow semantics) | Cost | Under 80? |
|---|---|---|
| `length_sq` / `dot` today (helper calls) | 170 / 171 | no |
| per-op `*` without the 32-bit guard, and with a division check | 48 and 40 per op; `length_sq` 134 and 118 | no |
| `length_sq` open-coded: one range guard, shared slow-path call | 95 (`dot` 115) | no |
| `length_sq` open-coded, `bits.Mul64` on absolute values, `bits.Add64`, one panic | 98 | no |
| the same, branch-free abs, named result, naked return | 95 | no |
| `dot` open-coded, signed `bits.Mul64` checks, one panic, named result | 117 | no |
| `def area = @w * @h` (one `*`) | 66 | yes, already |
| `@w + @h + @w` (two `+`), `@w = @w + @h` | 61, 37 | yes, already |
| `@w > o.w` (a comparison) | 6 | yes |
| `@w * @h + 1` today / open-coded, one panic | 94 / 59 | no / **yes** |
| `(@w + @h) * 2` today / open-coded, one panic | 94 / 39 | no / **yes** |

- A 30-bit range guard whose slow path only panics costs 42, but it is wrong: it raises on large products that fit.
- Naked returns and named results saved about 3.

**The codegen change: "fused checked arithmetic".** When a method's Integer expression has two checked operators, codegen writes it inline:
- the wrapping operations,
- one combined overflow test,
- one `panic(rbIntOverflowedN{inputs})`.

`rbWrapPanic` (decision 400) would recompute with the checked operators to name the operator that overflowed, so the RangeError message stays the same (inferred to be feasible).

Single-operator methods already inline; this extends inlining to "one operation plus a constant or a second operation". By the table, that covers the common computed-reader shapes.

**Rejected alternatives:**
- `GOEXPERIMENT=newinliner` with `-d=inlbudgetslack`: experimental and whole-program.
- A synthetic `-pgo` profile (decision 449): it keeps frames, but likely recompiles std for each distinct profile (inferred). Experiment E3 settles it; if it is cheap, it replaces pasting.

**Fragility.** Every threshold here depends on Go's unspecified cost model. A `TestInlineCosts` should pin it: build a few canonical generated methods with `-gcflags=-m` and grep the output, expecting:
- `Integer#+` and `Integer#*` inlinable,
- `area` and the fused `@w * @h + 1` inlinable,
- `length_sq` not inlinable.

A Go upgrade that moves the model then fails the test, and the thresholds get re-measured.

## Mechanism of the paste

**Where: a new pass `inlineLeafCalls` (`internal/compiler/inline_leaf.go`).**
- It runs in `formatGo` after `specializeSelf` and before `inlineBlockLoops` / `inlineIdxSet` / `inlinePush` (`format.go:172-178`). It also runs after the sweep (`:166`) and before `addFrameLabels`, which lists only functions still declared (`:90-115`).
- `formatGo` runs once, after codegen (`compiler.go:823`). So probes and inference rounds see only the real call, with the same types as before.

**Why not paste at gen time** (`callCode`, `expr_callcode.go:639`; `directCall`, `:1177`): it would compile the callee's arithmetic and nil checks in the caller file's mode, against decisions 381 and 393. It would also need a fake `fctx` for `self`. In the AST pass, the callee body is final and already in its own file's mode.

**Selection is a plain call `F(args)`** to a declared, non-generic, receiverless free function. Codegen emits that form only when it has proved which definition runs:
- a leaf receiver (`leafCall`, `expr_callcode.go:1172`; decision 359),
- `sendCall` with no overriding arms (`dispatch.go:363-367`), or
- a top-level def (`gen_method.go:322-334`).

These are never pasted:
- `rbSend_` dispatchers (decision 212)
- interface calls
- generic `Mod_M[Self]` calls
- method values
- Dyn wrappers

Those keep the function referenced, so it still exists for dynamic calls, `send` and `method`.

**Side input from codegen.** Codegen passes a set `c.inlineCands`, filled beside the labels at `gen_method.go:283` and `:330`, like `blockLoopHelpers` (`inline_blockloop.go:12-16`). It holds the Ruby-level facts the Go body cannot show.

**How the callee maps into the caller:**
- `self` and each parameter are replaced by the argument, so an ivar `self._Vec2().x` becomes `v._Vec2().x`.
- `return E` becomes `(E)` in the call's place.
- An untyped `E` (a constant or a comparison) is wrapped as `ResultT(E)`, as `typedConst` does (`inline_idxset.go:104`).
- Pasted nodes take the call's position (`blockInliner.pos`, `inline_blockloop.go:22`).
- A candidate left with no reference in the file is deleted.

**How the Go team's hazards are handled** (go.dev/blog/inliner):

| Hazard | Stage 2 rule |
|---|---|
| Evaluation order | Arguments must be pure: an identifier, a literal or a package constant. |
| A parameter used more than once | Safe for an identifier the caller never assigns inside a function literal and never takes `&` of. |
| Shadowing | Refused if a free identifier of the body (`bits`, `math`, `rbIntWrap`) is declared in the caller. `goLocalName` (`naming.go:118`) does not reserve those names. |
| `defer` | Refused: a rescue or ensure in the body, or a named result. |
| Constant folding | Constant arguments are converted to the parameter type. Refused if both sides of an operator end up constant (`goConstExpr`, `expr_callcode.go:1226`). |

Stage 3 adds hoisted arguments and multi-statement bodies, reusing `stmtSlots` / `scan` and `returnsToGoto` (`inline_blockloop.go:112-193`, `:424`).

**Lint (the generated Go must lint clean).** Refuse a paste when:
- the body compares a parameter with nil (`nilness`),
- two sides of an operator become textually equal (`dupSubExpr`), or
- a parameter is an interface type (`unconvert`, `.golangci.generated.yml:29`, blocks the conversion that would fix it).

## Eligibility rule

**Ruby level:**
- Kind: a `def` in user or gem code. Not an attr method, not a prelude method, not `initialize`, not `method_missing` / `respond_to_missing?`, not `define_method`.
- Parameters: required positional only.
- No block, `yield`, `block_given?`, `super`, `rescue`/`ensure` or loop.
- Leaf: every call goes to a prelude method, an attr reader or another candidate.
- No frame-observing names: `raise`, `fail`, `caller`, `caller_locations`, `warn`, `__method__`, `__callee__`, `binding`.
- The defining file has not opted out.

**Go level:**
- No type parameters, no receiver, no `type Self =` line (`gen_method.go:300-310`).
- A single `return E` with no function literal; no parameter assigned or address-taken.
- Not on a cycle among candidates (as spinel's `fi_reaches`, `codegen.c:5374-5385`).
- Estimated Go cost between 81 and 320. The estimate counts 1 per node, 57 per call that won't inline, a known callee's own cost when it is ≤ 80, and 1 per panic.
- At most 12 call sites after pruning (spinel's `callmax`, `codegen.c:4797`).
- The caller stays under 4,000 nodes. Past 5,000, Go caps inlinees at cost 20 there, and `Op_mul` (60) would stop inlining.

**Thresholds:**
- The lower bound of 81 means "Go refuses", so Go-inlined methods keep exact frames.
- 320 is 4× Go's budget and covers 170 and 178.
- spinel's 90/250 (`codegen.c:4792`, `:5354`) count Ruby syntax nodes, and spinel asks the C compiler (`inline SP_ALWAYS_INLINE`, `:6068`) rather than pasting.

**Effect on existing tests:**
- `TestIntOverflowValuePanic`'s `add` (cost 30) and `mul` (cost 64) are not pasted.
- `ObjectCallerLabels#outer` (`object_test.rb:5511`) is not a leaf.

## Option, backtraces and interactions

**The flag: `-inline-methods on|off`, default `on`**, on build, run, gen and test.
- Environment form: `RUGAROO_INLINE_METHODS`.
- File form: `# inline_methods: off`, for methods defined in that file (the callee's frame; as decision 381).
- Plumbing, the same as `-regexp-compile`:
  - `cmd/rugaroo/main.go:80-88` (flag registration),
  - `cmd/rugaroo/test.go:195` (test's flag mapping),
  - `internal/compiler/env.go:27-29` (environment field),
  - the mode check at `compiler.go:1070-1072`,
  - docs/cli.md rows (`TestCLIDoc`).
- `-debug` implies `off` unless `on` is given explicitly. spinel does the same: "debug builds want real frames" (`codegen.c:4760-4762`; its switch is `--no-inline-hot`, `main.c:1071`).
- This is decision 487's permanent first kind, not a temporary off switch: `off` changes observable backtraces and `caller` output, and a user may want MRI's frames. The `off` lane is today's compiler minus one pass.

**`RUGAROO_BACKTRACE` stays run-time only** (decision 178). The compiler can't see it, so the flag is the only control.

**What a backtrace looks like** for an overflow in a pasted `length_sq` called at `bm.rb:42`. rbBacktraceFrames uses `runtime.CallersFrames` (`prelude/go/backtrace.go:173-182`), which expands Go-inlined frames. That is why fusion keeps frames and pasting does not.
- MRI and `off`: `bm.rb:11:in 'Vec2#length_sq'` then `from bm.rb:42:in '<main>'`.
- `on`, with only statement `//line` directives (`decls.go:17`): `bm.rb:42:in '<main>'`.
- `on`, with inline `/*line bm.rb:11*/ … /*line bm.rb:42*/` around the paste: `bm.rb:11:in '<main>'` (inferred; experiment E1).

**Interactions with existing decisions:**

| Decision | What happens |
|---|---|
| 372/381/400 | The body is pasted already lowered in the callee's mode; the message is unchanged. In the wrap lane `Op_mul` folds, so less is pasted (inferred). |
| 393/440/221 | Checks are pasted as they are. |
| Pruner, 485/486 | Functions are only removed, never added. |
| 449 PGO | Pasting stays on with `-pgo`. |
| Compile errors | Unchanged; a pass bug shows as a `go build` failure caught by examples and lint. |
| `TestCompileCost` | Compiled-function count equal or lower. |
| Determinism | Declaration-order walk. |
| 402 | Precedent for "the code is the caller's" frames. |

## Plan

**Stage 0: experiments E1–E4 (about one day).**

**Stage 1: fused checked arithmetic plus `TestInlineCosts`** (no flag; frames kept).
- Expected: `area + 1`-like readers inline.
- `bm_inline`: ~0 (`length_sq` is still 95).
- `benchmarks/objects.rb`: ±noise. It allocates in `Vec#+`, and `area` dispatches.

**Stage 2: the pass.** Pure arguments, single-return bodies, the flag, environment variable, file comment, `-debug` coupling and a new decision.
- Tests:
  - `testdata/test/inline_test.rb` against MRI.
  - `testdata/test/inline_off_test.rb` with `# inline_methods: off`, comparing backtrace and `caller` frames against MRI. This puts the `off` lane in the default suite.
  - `TestInlineMethodsMode`, in the style of `rugaroo_test.go:1840`, pinning the `on` backtrace.
  - One example program.
- Size: about 500 lines.
- Expected: `bm_inline` 1,210M → about 710M. Gem programs: 0–2% (inferred).

**Stage 3:** hoisted arguments and multi-statement bodies.

**Stage 4 (owner's call):** non-leaf bodies, dispatcher arms.

**Measurement at each stage:**
- `scripts/gen-compare`.
- `go test ./...` in the default lane, and with `RUGAROO_INT_OVERFLOW=wrap`, `RUGAROO_REGEXP_COMPILE=eager` and `RUGAROO_INLINE_METHODS=off`.
- ruby/spec `core`.
- `benchmarks/` and spinel's suite: instructions retired, GOMAXPROCS=1, alternating, median of 5.
- Compiled rubocop against MRI, plus its compile time and binary size.

**Abandon pasting if:**
- any output other than a backtrace differs from MRI,
- the lint exclusions keep growing,
- `bm_inline` gains less than 15%, or
- rubocop's compile time or binary grows more than 2%.

## Needs the owner's sign-off

1. On by default changes backtraces and `caller` for pasted methods (a note in decisions 106 and 178).
2. The flag's name, its file-comment scope, and `-debug` implying `off`.
3. Its classification as a permanent option under decision 487.
4. Stage 4 at all.

## Open questions, each with the experiment that settles it

- **E1:** Do `/*line*/` comments within the call's position range survive `go/printer`, and does `RUGAROO_BACKTRACE` print `bm.rb:11`? Hand-edit bm_inline's `main.go`.
- **E2:** Does the cost estimate agree with `-m=2`? Compare on `benchmarks/` and rubocop; target ≥ 95% agreement.
- **E3:** What do a synthetic pprof profile's gain and std rebuild cost come to (`go build -x`, cold cache)?
- **E4:** Is 320 the right cap? Sweep 200/320/500 on spinel's suite against binary size.

### Critical Files for Implementation
- /Users/jtarchie/workspace/transpiler/internal/compiler/format.go
- /Users/jtarchie/workspace/transpiler/internal/compiler/inline_blockloop.go
- /Users/jtarchie/workspace/transpiler/internal/compiler/gen_method.go
- /Users/jtarchie/workspace/transpiler/internal/compiler/expr_callcode.go
- /Users/jtarchie/workspace/transpiler/prelude/integer.rb
