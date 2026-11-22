# Making inference rounds cheaper (design, 2026-11-21)

Read-only plan, written against `main` at ff19ddb1 by a planning agent; nothing here is built. Every claim about today's code cites file:line. Anything worked out rather than read in the code is marked *(inferred)*. Timings are from `RUGAROO_TIMING=1` on an idle machine (decision 478). One thing in them does not add up: if `infer` really costs ~15 s per final pass, rubocop's phases add to about 241 s, not 222 s. So each stage is estimated as savings subtracted from 222 s. Open question 5 settles which reading is right.

## Outcome (2026-11-22)

S0 was built (the `round work` line and `RUGAROO_ROUND_DUMP`, decision 491). It measured s, the share of `collectUses` time in seen classes, at about 7% on rubocop, not the 60% assumed below, so S2 and S3 would save about 1 s a round and were not built. What was built instead is the one-walk part of stage 1 without its semantic change: the return walk records the round's uses and emission runs only where that walk does not count (decision 491; rubocop 227 s -> 200 s, Go identical). S1 and S4 remain open; S4 is the large one (`r-refine` is about 38 s of rubocop's 200 s, most of it the one searching round).

## Summary and recommendation

**Where a round's time goes.** A round runs `refineIvars`, then resets and re-infers every return, then emits every user method to record call uses (`compiler.go:518-533`). Only the uses feed the next round (`infer.go:923`). Most of that work is done twice:

- **Returns are inferred twice.** Every dry run in `refineIvars` already infers returns and emits each method of the classes that can see a candidate ivar (`gen_ivars.go:170-179`). The try that passes leaves the accepted typing in place. Then the round throws those returns away and infers them all again (`compiler.go:521-526`).
- **Emission is repeated.** `collectUses` emits the same methods once more (`infer.go:801-809`).
- **The tries differ in very little.** Within one `refineIvars` call, the only inputs that change between the baseline and the tries are the n candidate ivar types (`setIvarTypes`, `gen_ivars.go:121`). Parameter types are fixed at link (`signatures.go:200`, through `pendingType`, `infer.go:259`). Yet each try re-emits every method of every class that can see a candidate.

**Recommendation.** Don't do the one-walk redesign (stage 1), the persistent world (stage 2) or a general cross-round memo (stage 3). Instead, in this order:

1. **S0:** a per-round state dump to check that each later stage changes nothing.
2. **S1:** drop exact duplicate tries in `salvage`.
3. **S2:** take each round's call uses from the try that passed.
4. **S3:** take the round's returns from that try when nothing could make them depend on order.
5. **S4:** inside one `refineIvars` call, reuse a method's dry-run result when it reads no candidate whose typing changed.
6. **S5 (optional, needs the owner's approval):** a rerun final pass tries the previous final pass's kept typings first.

**Expected rubocop transpile** (central estimate, 222 s today): S1 ~220 s, S2 ~196 s, S3 ~182 s, S4 ~158 s (range 150-170), S5 ~150 s.

**Parser gem test program** (42 s today): about 41.5, 38.5, 37.5, 28, 27 s.

**Where to stop:** after S4, then measure. At that point `format` (33 s, outside the rounds) is rubocop's largest phase.

**Is the one-walk redesign worth doing? No.** It does not reach the same fixed point (reasons below). It would need amendments to decisions 15, 146 and 444. Returns would cross one call level per round, so more rounds would be needed. Its round would cost about one emission walk plus setup, roughly 9 s on rubocop *(inferred)*, against about 11 s per round after S2-S4. Spread over more rounds, it ends up no faster.

## How a round works today

- **A new compiler every pass.** Each round and final pass is a new `Compiler` (`compiler.go:691`). The round branch is `compiler.go:787-791`.
- **Round limits.** At most 8 rounds (`infer.go:43`), ×6 under `RUGAROO_INFER_UNKNOWN` (`compiler.go:605`). Three settled rounds in a row end them early (`infer.go:50`, `compiler.go:621`).
- **`refineIvars`:**
  - Re-discovers ivars whenever new candidates appear (`gen_ivars.go:20-36`).
  - Runs the untyped baseline (`:54`), then the seed try (`:84`), then `salvage` (`:135-157`): 2n+2 tries.
  - Each try is `dryRunIvarUsers`: it resets every inferred `Ret` (`:163-165`) and runs `inferRet` + `emitMethod` over the seen classes inside **one** `catchCompileError`, so a try stops at its first error (`:169-181`).
  - A try passes when it has no error and every conversion site it emits was already in the baseline's (`:61-63`). A nil veto adds dry runs through `nilRetry` (`nilcheck.go:86-111`).
- **Dry-run-only behaviour** (`inDryRun`/`dryTry`):
  - `vetoDryTry` (`gen_method.go:121-135`).
  - Seeded local search (`gen_analyze.go:455-497`, used at `:367`).
  - `noteWidenPending`, which never runs in a round anyway (`expr_coerce.go:623`).
  - Poison is sticky across tries (`gen_method.go:98-106`).
- **Uses are recorded only while `c.uses != nil`.** `collectUses` sets it (`infer.go:799`); the record sites check it (`infer.go:718`, `expr_args.go:1047,1061`, `expr_untyped.go:1141,1348`, `infer_rest.go:43`). Per call site the last pass wins (`infer.go:1182-1198`).
- **`inferRet` depends on order.** A gem recursion gives the *inner* call `TAny` and the outer one overwrites it (`signatures.go:400-402`, `:480`). So the method a cycle is entered from decides the result.

## Stages, in order of saving per unit of risk

### S0. Oracle and counters (no behaviour change)

**What changes.**
- `RUGAROO_ROUND_DUMP=path` writes, for each round: the sorted `next` map (as the `inferred` lines at `infer.go:967-971` do); `refineIvars`' accepted vector (`gen_ivars.go:87-90`); every inferred method's `Ret` and `roundRet`.
- `RUGAROO_TIMING` gains counters: dry runs per `refineIvars`, split into pass, fail and nil-retry; the share of `collectUses` time spent on methods of seen classes; how often the passing try entered a return cycle or took the seed-kept shortcut.

**Invariant.** Generated Go unchanged. The dump becomes the check for S1-S4: identical per-round dumps imply identical output, and the first round that differs points at the cause.

**Verify.** `go test ./...`. **Saving.** None.

### S1. Exact duplicate tries in `salvage` (`RUGAROO_NO_TRY_DEDUP=1`)

**What changes.** Keep a per-`refineIvars` map from a set's key to {passed, the `c.nilDefer` it left}. When a try repeats a set already tried, apply the ivar types (`setIvarTypes`), restore `nilDefer`, and return the recorded result. Duplicates happen in three places:
- `salvage`'s last `try(set)` (`:156`), whenever phase 3 passed at least once;
- in a round, a seed set equal to "all", which `salvage` tries again at `:140`;
- a seed set equal to "all but k", which the leave-one-out loop tries again for k.

**Invariant.** This is `salvage`'s own premise, that a try is a predicate of the set. The dropped repeat could only differ through local seeds (the last dry run's are never read afterwards, because `localSeedKey` needs `inDryRun`, `gen_analyze.go:465`) and catalogs (they only grow, and the earlier run of the same set already added its entries *(inferred: equal unless that method's seeded local search diverges)*).

**Verify.** gen-compare, plus rubocop's Go and round dumps. **Saving.** About 1 dry run per search: ~0.5 s on the parser, ~1-2 s on rubocop. **Risk.** Very low.

### S2. Call uses from the try that passed (`RUGAROO_NO_TRY_USES=1`)

**What changes.**
- In a round, every try records uses into its own buffer (set `c.uses` during `dryRunIvarUsers`). The buffer of the try that ends up accepted is kept. With S1 that is the last try that passed.
- No uses are recorded while inside `inferRet`: add a depth counter checked at the gates listed above. This keeps today's rule that return inference records no uses (r-rets runs with `c.uses == nil`).
- Snapshot every `Ret` and `roundRet` at the end of the accepted try. After r-rets (`compiler.go:521-526`), compare them with the freshly inferred values.
- If they all match, `collectUses` (`infer.go:801`) skips `emitMethod` for user methods the accepted try emitted, and merges the try's buffer in. Defaults and main bodies are still run.
- Fall back per method when its emission took the seed-kept shortcut (`seededLocals` returned true through `seed.kept`, `gen_analyze.go:484`). Fall back for the whole round when any `Ret` differs.

**Invariant.** In a round the only differences between a dry-run emission and `collectUses`' are the seeded local search (handled per method), `vetoDryTry` (a passing try raised no veto in an emitted method; gem methods that fail the baseline go through the same branch-lenient/poison path in both) and `c.convs` recording (output only). `emitMethod`'s inputs are then equal: same params, same `Ret`s (checked), same poison (sticky), same `nilDefer` (the accepted try's, kept into `collectUses` today, `gen_ivars.go:79-81`). `lastPassUses` sorts by position, so the order of emission does not matter.

**Verify.** Round dumps identical on the parser program and on rubocop; gen-compare (272 inputs); the full suite.

**Saving** (assumes a share s of `collectUses` time in seen classes, s≈0.6 *(inferred, S0 measures it)*): rubocop 44 s × 0.6 ≈ 26 s; parser 5.6 s × 0.6 ≈ 3.4 s; zero if `Ret`s differ every round (open question 2). **Risk.** Low to moderate. Each fallback is a check, not an assumption.

### S3. Returns from the try that passed (`RUGAROO_NO_TRY_RETS=1`)

**What changes.** Skip the reset at `compiler.go:521-523` when the accepted try had all three of:
1. no `inferRet` entered while its method was already inferring (`signatures.go:400,404`), and no `inferRecursive`;
2. no seed-kept local shortcut inside an `inferRet`;
3. no compile error caught by `inferRet` inside the dry run (`signatures.go:430-438`; a `vetoDryTry` panic there sets `roundRet`, where a non-dry run would poison).

The loop at `:524-526` then fills only the `Ret`s that are still nil.

**Invariant.** With no cycle, every `Ret` is a function of fixed inputs (params, accepted ivars, poison) and not of the order it was computed in, so it equals r-rets' result. `hardAny` (`signatures.go:482`) is written by the same computation either way. Without the conditions, today's path runs, and S2's comparison still guards the uses.

**Verify.** As in S2. **Saving.** 30 s × the share of methods in seen classes × the share of rounds with no fallback: rubocop 0-18 s, estimated ~14 s; parser ~1 s. **Risk.** Low, given the three conditions.

### S4. Candidate read-set memo inside `refineIvars` (`RUGAROO_NO_REFINE_MEMO=1`, check mode `RUGAROO_REFINE_MEMO_CHECK=1`)

**The idea.** Inside one `refineIvars` call the world is frozen except the candidate typings. So a method's read set only has to say which of the n candidates it read, as a bitset. It does not need to cover the whole world, which is why this is affordable where a general read-set memo is not.

**What changes.**
- Make `Ivar.Type` unexported behind an accessor that ORs a candidate's bit into the current step's set. The Go compiler then finds every direct read; there are about 100 sites matching `iv.Type`/`.Ivars[`.
- Add an accessor for `Method.Ret`. A `Ret` computed during a dry run carries the bits read while computing it, and a read of it ORs them in.
- A step is one method's `inferRet` + `emitMethod` (`gen_ivars.go:176-177`). Its memo entry holds its outcome (error or not, and the error's `nilVeto` flag), its conversion sites, every `Ret` (with `roundRet` and bits) that it assigned, and its recorded uses (for S2).
- Key: (method, `dryTry`, `nilDefer` identity, typing restricted to its bits).
- On a hit, replay the entry instead of emitting. A baseline step with no error and no bits is a hit for every try. A memo hit for every step of a try is S1 for free.
- Apply the same memo to the final pass's search (`gen_ivars.go:84-86` with `!c.round`).

**Invariant.** The memo is exact if the read set is complete and no other state written by one step and read by a later one depends on the candidates. Candidates for such a leak: the memo maps in `memoState`, `strCacheIvars`, catalogs that emission reads back. The compile-time-2026-11 research behind decision 433 found leaks of this kind once before. Check mode runs every step anyway and returns an internal error when the replay differs.

**Verify.** Check mode over gen-compare's inputs, the parser program and rubocop (once, slow); then gen-compare and round dumps with the memo on against off.

**Saving** (f = share of a try's step cost whose steps read a candidate, f≈0.2 *(inferred)*):
- **rubocop:** the six seed rounds go from ~2.5 s to ~1.7 s each (−5 s); the searching round from 25 s to ~6 s (−19 s); the final passes' searches about −10 s. Total ≈ −24 to −34 s at f 0.2-0.5.
- **parser:** r-refine 17.8 s goes to ~7 s, and the final infer saves ~1.5 s. Total ≈ −11 s.

**Risk.** Moderate. It is the largest refactor here, two accessors, but completeness of direct reads is checked by the compiler and the rest by check mode.

### S5 (optional, needs an amendment to decision 444). A rerun final pass is seeded (`RUGAROO_NO_REDO_SEED=1`)

**What changes.** A rerun final pass (`compileOnce` loop, `compiler.go:561-567`; `widenRedo`, `:537`) first tries the previous final pass's accepted set (`opts.refineOut`, refinereplay.go). It runs `salvage` only if that fails, as rounds do (`gen_ivars.go:84`). Decision 444 says "The final pass always searches", so this amends it.

**What may change in the output.** A typing the first pass dropped could compile once the parameter is widened to an untyped container, and then today's search would keep it. Settle how often with open question 6 before proposing it.

**Saving.** Rubocop ~5-8 s once S4 has cut the search; parser ≤1 s.

## Rejected, with reasons

### Stage 1, one walk per round (Jacobi)

Parameters are already Jacobi today: every round reads the previous round's types, fixed at link (`signatures.go:200`). Ivars (a search) and returns (lazy, recursive over the whole call chain) are solved inside the round. Making them Jacobi too:

1. **Returns.** A return type would cross one call level per round instead of a whole chain in one. That means more rounds, against a cap of 8 and a settled-round rule that counts rounds (`infer.go:50`).
2. **Ivars.** A veto needs a compile attempt. One walk under the previous round's typing records uses from bodies that may not compile, and the round after vetoes them. The fixed point it settles on is "some set that compiles", not `salvage`'s pick of all, then all-but-one, then greedy. On conflicting typings, that is a different set.
3. **Recursion.** Iterating returns finds a fixed point from the last round's values. The lazy rule gives the inner call `TAny` (`signatures.go:400-402`). The two can differ.
4. **Not monotone.** The default lane is not monotone: state is rebuilt from uses every round (`infer.go:923`), plus vetoes, decision 383's drops, 466's `hardAny`, and 444's bets in settled rounds. So the theorem that chaotic iteration reaches the same least fixed point does not apply. Rubocop does not converge today at all; its output depends on the path the rounds take, so it would change.

This needs amendments to decisions 15, 146 and 444, and may loosen types (runtime speed). The part worth having (one emission whose returns are filled lazily, Gauss-Seidel style) is what S2+S3 build without the semantic change. Revisit it only as the worklist of the not-known-yet lane (docs/research/inference-2026-11.md, step 6), where the state only goes up.

### Stage 2, keeping the linked world across rounds

Link has to run again anyway, because parameter types enter at link (`resolvePending`, `infer.go:277`, called at `signatures.go:200,248`, `forwarding.go:49`). So only collect and the prelude would be saved: at most ~5-10 s on rubocop (user 7 s + part of link 16 s, over 7 of 11 passes) and ~0.5 s on the parser.

A reset between rounds would have to restore:
- per method: `Param.Type`, `Ret`, `roundRet`, `retWhy`, `poison`, `inferring`, `recursed`, `resolved`, `shadowed`, `calleeDefaults`, `staticPriv`;
- per class: its ivars (`Type`, `open`, `elems`);
- the generic instances (a round can name a new one and restart, `compiler.go:611-613`);
- every map in `memoState`, `tableState`, `dynState`, `marshalState`, `refineState` (`compiler.go:163-311`);
- `initWrites`, `nilIvars`, `ivarReadFirst`, `untypedIvars`, `gemNilIvars`, `uncalledWriters`, `interpOnly`, `gemParams`, `warned`.

Missing one of these would make a round read the last round's state without anyone noticing. Not worth the saving.

### Stage 3, a general cross-round memo

Across rounds the read set is the whole typing environment: params, every `Ret` (reset each round), ivars, constants, class shapes, and flags such as `noDrop`, `anyUse`, `hardAny`, `settled`. And a ≤2% change in parameters spreads through `Ret`s. Tracking all of that across 67k lines of codegen is speculative abstraction. S4 is the bounded version: n candidates and a frozen world.

### A provable cut of the searching round by reordering

Emitting first the method that failed in the previous try changes which error a try finds first. Retrying a nil veto depends on that error's `nilVeto` flag (`nilcheck.go:104`), so the accepted set can change. The only exact cuts are S1 (proved by `salvage`'s premise) and S4 (exact under a complete read set, enforced by check mode).

## Open questions, each with the experiment that settles it

1. **What share s of r-uses/r-rets time goes to methods of seen classes?** It decides S2 and S3. Experiment: S0's counter on rubocop and the parser program.
2. **How often does the accepted try's `Ret` snapshot differ from r-rets, or hit a cycle?** It is the fallback rate of S2 and S3; rubocop-ast's recursive walkers may trigger it every round *(inferred)*. Experiment: S0's counter; if it is high, find whether a few cycles cause it, and count per round.
3. **What is f, the share of steps in a try that read a candidate?** It decides S4's saving. If a candidate sits on rubocop's `Cop::Base` and an attr reader every cop calls reads it, f could be large. Experiment: a throwaway branch with only the `Ivar.Type` accessor plus a hook on `inferRet`'s early return, printing the steps with non-empty bits per try.
4. **Which state leaks between steps within one try?** S4's soundness. Experiment: `RUGAROO_REFINE_MEMO_CHECK=1` over gen-compare's 272 inputs and rubocop; every mismatch names a field to add to the key or the entry.
5. **Is the final passes' `infer` ~15 s per pass or in total?** Per pass, rubocop's phases sum to ~241 s against 222 s. Experiment: rerun with `RUGAROO_TIMING=1` and sum the ticks per pass.
6. **How often would S5's seed change the output?** Experiment: in check mode, run both the seed and the search in every rerun final pass on rubocop, liquid, kramdown and the parser program, and count disagreements.
7. **Where do the searching round's dry runs go?** Pass, fail, nil retries (`nilcheck.go:86-100`), and how far a failing try gets before it stops. This calibrates S1 and S4. Experiment: S0's counters.

## Files the stages touch

`internal/compiler/gen_ivars.go`, `infer.go`, `compiler.go`, `signatures.go`, `gen_method.go`, `gen_analyze.go`.
