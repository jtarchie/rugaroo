# A Crystal-style type graph beside the compiler (spike, 2026-11-27)

The user's question, after decision 502 built Crystal's propagation at method granularity: from scratch, should rugaroo
type programs with a graph, as Crystal does, instead of running code generation over rounds? This spike measures it.
Nothing here changes what the compiler outputs.

## What was built

`RUGAROO_TYPEGRAPH=1` (`internal/compiler/typegraph.go`) runs after the compiler's own inference, in the final compile,
and prints how the two agree.

- **Slots** are what Crystal's nodes are at the level that matters here: each parameter typed from use, each method's
  return, each local of each method, each ivar of each class, each yield position, each value constant.
- **One evaluator** walks a method's body over Prism's tree and gives each expression a type from its parts:
  literals, locals, ivars, `if`/`case`/`&&`/`||`, `rescue => e`, multiple assignment, calls. A call on a program method
  flows its arguments (positional, keyword, defaults) into that method's parameter slots and answers its return slot;
  a call on a prelude method answers the method's RBS return under the receiver's type arguments, with block
  parameters bound from the block's signature. Intrinsics with no method entry (`raise`, `is_a?`, `class`, `==`) are
  listed by name.
- **Propagation** is Crystal's: reading a slot registers the method as an observer, a slot that changes re-queues its
  observers, and the work list runs until nothing changes. A slot changed more than 40 times widens to untyped, which
  keeps the lattice finite.
- **No code is generated** and no Go type is chosen. The compiler's own classes, signatures and method table are the
  graph's inputs; the program's top-level bodies are evaluated as methods with no owner.

## Results

Agreement counts only slots the compiler typed (not untyped); "never reached" is a slot no evaluation flowed into.

| program | params same | returns same | graph | compiler's inference (link, ivars, refine, rets, uses, infer) |
|---|---:|---:|---:|---:|
| `124_liquid` | 30 of 46 (65%), 11 never reached | 132 of 225 (59%), 43 never reached | 3 ms | 3.6 s |
| `123_kramdown` | 31 of 106 (29%), 48 never reached | 70 of 116 (60%), 6 never reached | 8 ms | 13.6 s |
| `116_cuba_web` | 13 of 23 (57%), 3 never reached | 18 of 33 (55%), 2 never reached | 2 ms | 1.1 s |
| rubocop (18,085 methods) | 673 of 1,356 (50%), 492 never reached | 3,496 of 5,302 (66%), 560 never reached | 285 ms | 312 s |

The examples and benchmarks that are not gem programs have nothing to compare: their methods are annotated, so the
compiler infers no parameter of theirs. `TestTypeGraphAgrees` holds a small unannotated program at 3 of 3 and 3 of 3.

**How agreement moved with the rules added**, liquid's parameters: 16 same with the first version (calls, literals,
locals); 64 of 401 overall once the top-level bodies were evaluated; 30 of 46 typed ones (65%) once defaults, keyword
arguments and `rescue => e` flowed. Each batch was an hour or less.

**What the graph does not know yet**, rubocop's largest gaps (counts are expressions the graph left untyped):

| gap | count | what it is |
|---|---:|---|
| a call on an untyped receiver | 52,100 | the cascade: everything below a gap is untyped too |
| `children` and other node matchers | 2,876 + | methods rubocop-ast defines by `def_node_matcher` and `method_missing`, which the compiler knows from its boot record (decisions 330, 444) |
| a call on nil | 1,931 | a value the compiler narrows (`x && x.y`, `if x`), which the graph does not |
| splats, `super`, `super(...)` forwarding | 1,415 + 802 + 293 | argument forms the evaluator skips |
| the compiler's own rewritten nodes (`exprNode`) | 1,390 | a pass that rewrites the tree before the final compile |
| constants by path | 861 | constants the lexical lookup did not resolve |

## What it says

- **Speed is the case for the graph, and it is large.** On rubocop the graph's whole solve is 285 ms against 312 s of
  the compiler's inference: three orders of magnitude, because it evaluates each method a couple of times (30,702
  evaluations for 18,085 methods) where the rounds generate code for every method in each of 21 rounds. Even if the
  missing language made the graph ten times slower, inference would drop from minutes to seconds.
- **Agreement after a day is 50-66%, and it rises with each rule.** The gaps are listed above, and none is a wall:
  narrowing, splats, `super` and the boot record's methods are each a known piece of the compiler that the graph
  would have to have. The long tail is the compiler's ~500 decisions; most of them are about what Go can represent
  (unions, tuples, nil boxing), which a graph that only types does not have to model, and the rest is gem leniency.
- **The risk is not two engines disagreeing forever** if the graph becomes the only source of types and code
  generation follows it, as in Crystal. It is that code generation must then express every type the graph picks.
  Today inference and code generation cannot disagree, because they are one walk.

## Recommendation

Do not replace the rounds in one step. The cheapest experiment that uses the graph's speed and keeps today's
correctness is **graph-seeded rounds**: run the graph first and give the rounds its types as their starting state,
only for slots the graph reached with no gap on the way (no untyped input), so that the rounds start near their fixed
point and code generation still decides every type. Measure on rubocop: rounds (21 today), transpile time, and
gen-compare against unseeded rounds; a seed that is wrong is joined by the rounds' own uses, and under the
not-known-yet rounds the state only goes up, so a seed may only be a type the rounds would reach anyway, which the
gen-compare checks. If that halves the rounds, migrating construct by construct (narrowing, splats, `super`, the boot
record) raises the share of seeded slots each time, and the rounds become the checker rather than the solver.
