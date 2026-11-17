# Start-up below the floor: what a compile-time snapshot could be (proposal, 2026-11-16)

A proposal for the owner to choose from. Nothing here is built, and every size below is read off measurements that
already exist (`docs/research/startup-2026-11.md`, `docs/research/runtime-speed-2026-11.md`); none was taken for this
document. The owner's direction (2026-11-16): bring start-up down with general speed-ups **and** a compile-time
snapshot, not with rubocop-only gem patches.

## The floor today

Compiled rubocop at main e90cff36, instructions retired:

| run | compiled | what the step adds |
|---|---:|---|
| load only (every gem class body, no `main.rb` statement) | 556M | constants, class bodies, cop registration |
| `--version` | 1,165M | +609M: options, `config/default.yml` parsed twice (about 320M), the result-cache checksum |
| an empty file | 1,552M | +387M: config for the file, validation, registry queries, the Team, the runner |
| 20 files | 9,224M | +7,672M of per-file work |

MRI spends 5,727M on the empty file. Ten times faster than MRI is about 570M for one file and 1,140M for twenty,
so the 1,552M of start-up alone is above both budgets. No amount of per-file work changes that.

What the 1,552M is, from the profile taken before decisions 471-473 (shares of that run; the regexp, `__dir__` and
glob rows are now mostly gone): YAML 16%, the collector 18%, the result-cache checksum 7%, cop registry queries and
Team 7%, config validation 5%, cop class bodies 4%, `Ractor.make_shareable` 2%, the rest spread thin. All of it is
the program's own Ruby, run faithfully.

## Three things "snapshot" can mean

They differ in what is frozen at build time, and so in what can go stale.

### A. Parse a gem's data files at build time (content-keyed)

`YAML.load`, `JSON.parse` and `Marshal.load` are pure functions of their input. The compiler already embeds what
MRI loaded; it can also parse the data files a traced run read from gem directories (`config/default.yml`,
unicode-display_width's index) and emit each result as Go code that builds the value. At run time the loader hashes
the string it was given; on a match with an embedded file's hash it builds the value from the generated code instead
of parsing, and a mismatch (the file changed, another file) parses as today.

- **Sound without conditions:** the result is the same value the parse would give, for that exact input; a fresh
  copy each call, so a caller that mutates it sees no difference.
- **General:** any program that reads YAML, JSON or Marshal data shipped in its gems (Rails apps, i18n tables,
  mime types, unicode tables).
- **Size on rubocop:** the two parses are about 320M; building the value instead might cost a tenth of that. So
  roughly 1,552M → 1,270M. Not measured.
- **Cost:** the hash of the input on each load (about 1M instructions for 300 KB), binary size (the built value as
  code), and a rule for which files qualify (read during the traced run, inside a gem directory).
- **Fits the rules:** compile-time, no tracing input beyond the file list `-trace` already records, no new untyped
  dispatch. It needs `-trace` (or `-gem`) to know the files; a program compiled without either gets nothing.

### B. Emit load-time constants as static Go data

Today a gem's constant (`TABLE = [1, 2, 3].freeze`, a racc table, a frozen Hash of symbols) is built by code that
runs at start: allocate, append, freeze. Where the whole value is literal and frozen, the compiler can emit it as
package-level Go data the linker lays out, so it costs nothing at start and nothing to collect.

- **Sound** where the value is a frozen literal of literals; nothing can observe the difference. Anything computed
  or unfrozen stays as today.
- **General:** every program, typed or not; racc and Ragel tables, lookup tables, option lists.
- **Size on rubocop:** part of the 556M load phase and part of the collector's share (the 150 MB allocated to lint
  an empty file includes these). Unknown without a profile of the current load phase; I would not guess beyond
  "tens of millions, perhaps a hundred".
- **Cost:** Array and Hash need a representation whose backing store can be static (an Array over a static slice is
  easy; an insertion-ordered Hash needs its index either static or built on first lookup).
- **Fits the rules** with no question. This is ordinary constant folding.

### C. Snapshot the state after start-up

Run the program's start-up at build time, up to a point, and embed the objects it built (rubocop's loaded and
validated `Config`, the cop registry, the Team's set-up), so a run starts from them.

This is the only one of the three that can reach the budget, and the only one that is not sound by construction:

- **The state depends on more than the program.** rubocop's start-up reads argv (`--only`, `--config`), the
  environment, the working directory and every `.rubocop.yml` above the inputs, and the gem's files. A snapshot is
  right only for a run with the same answers. So it needs a key (what start-up read: files with their content
  hashes, env names, argv) checked at start, and a fall-back to the normal start-up when it does not match. That is
  rubocop's own result cache, one level down, and it is the tracing-for-production shape the owner asked to avoid:
  the binary is fast only for inputs like the build's.
- **Where the snapshot point is** is not something a compiler can know. It would be a marker in the program (a
  block the program wraps its set-up in: "what this block builds may be reused when what it read is unchanged"),
  which makes it an opt-in library feature rather than a compiler mode, and makes the key explicit.
- **What can be embedded** is what can be serialized: no procs, no IO, no threads. rubocop's registry and config are
  plain data plus class references, so they qualify; the Team holds cop instances with lambdas, so it does not.
- **Size on rubocop:** if everything from options to config and registry is reused, about 600-900M of the 1,552M.
  With A and B as well, an empty file might reach 500-700M, about the one-file budget. Not measured, and the
  restore itself (building 10-30 MB of live objects) has a cost I cannot size without building it.

A narrower C is honest and may be enough: **memoize a pure method's result across runs by the content of the files
it read**, as an explicit annotation on the method (`ConfigLoader.default_configuration`), with the value stored in
the binary at build time when the build's run produced it. It is A generalised from "parse" to "any function the
author marks pure over its file inputs". It still needs the purity promise from a human; the compiler can check
that the method's body reads no env, time or argv on the traced run, but not prove it for all runs.

## General speed-ups, the other half

These are in `docs/research/runtime-speed-2026-11.md` and need a rubocop build each to measure:

- the YAML loader (go-yaml's scanner is 60% of a load, 40% of that rune-slice-to-string conversion): helps every
  YAML program and stacks with A for files A does not cover;
- the collector's share of start-up (about 200M after decision 463's amendment): fewer allocations in class bodies
  is B's side effect;
- the result-cache checksum digesting 310 files where MRI digests 53 (`$LOADED_FEATURES` is complete from the
  start, decision 444): about 50M, a correctness difference worth fixing for its own sake;
- untyped dispatch and allocation under config validation and registry queries: the same runtime every cop runs on.

Taken together I would expect 1,552M → about 1,000-1,100M from these and A and B, with wide error bars: 5x MRI on one
file, not 10x.

## Recommendation

1. **B, then A.** Both are sound without conditions, general, and inside every standing rule. Measure each on
   rubocop, kramdown, liquid and example 116 (their load phases are the comparison) and on `TestCompileCost`.
2. **The YAML loader and `$LOADED_FEATURES`** alongside, as ordinary fixes.
3. **Decide C after 1 and 2 are measured.** If one file is then near 1,000M, 10x on small inputs needs C or it is not
   reachable; C's honest form is the opt-in, content-keyed memo of a method the author marks pure, and it should be
   judged as a language feature (does a Ruby author accept writing that marker, and does MRI ignore it cleanly), not
   as a compiler trick.

## Questions for the owner

- **C's shape.** Is an explicit marker in Ruby source acceptable (a method annotation or a block from a small
  `rugaroo` helper that is a no-op under MRI)? Without one, C needs the compiler to guess a snapshot point, which I
  would not build.
- **Staleness.** A snapshot keyed by file contents falls back to the slow path when a `.rubocop.yml` changes. Is
  "fast for the configuration the binary was built with, correct for any other" acceptable for the 10x claim?
- **Binary size.** A and B grow the binary (rubocop's is 295 MB today). Is there a limit?
- **Twenty files.** The 1,140M budget for twenty files cannot be met by start-up work alone: per-file work is
  7,672M. That half is the untyped-value cost (43% of per-file time is untyped dispatch), where today's measurements
  say inference from callers does not reach rubocop's AST code. It needs its own decision: typed signatures for
  rubocop-ast's node accessors, written and reviewed by hand, are the lever that has evidence behind it (the lexer's
  eight lines took 30% off).
