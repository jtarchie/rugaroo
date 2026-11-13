# overrides/rubocop

Compiles the rubocop CLI with rugaroo (#149, decision 444): rubocop 1.92.1, rubocop-ast 1.50.0, parser 3.3.13.0 and
regexp_parser 2.13.1, every registered cop, no `eval`. The compiled binary's stdout and exit status equal MRI's on
the test inputs, and it runs 1.4-5.3x faster than `ruby` (`docs/research/rubocop/results-2026-11-12.md`).

- `gem-patches/<gem>-<version>.patch`: applied with `patch -p1` to a copy of the installed gem. Each one replaces a
  dynamic Ruby feature a compiled program cannot do, or rewrites a line the same way under MRI. Two drop behaviour:
  ERB in `.rubocop.yml` raises `RuboCop::Error`, and the config duplicate-key warning is gone. Decision 444 lists
  every hunk and why.
- `main.rb`: `exe/rubocop`'s non-server path (`--server` is not supported).
- `setup`: copies the installed gems into the work dir once and applies the patches (`GEM_DIR` to use other gems).
  The work dir is `RUBOCOP_WORK`, else `$TMPDIR/rugaroo-rubocop`: outside any project, because rubocop reads the
  `.rubocop.yml` above its inputs (this repository's loads Bundler, which is not compiled).
- `build [rugaroo]`: traces `main.rb` under MRI and builds `$W/rubocop`. `DEBUG=1` builds with `-gcflags='-N -l'`.
- `check path...`: compiled vs MRI on each file or directory (relative to the work dir): SAME, or DIFF with the cops
  that raised. Copy inputs into the work dir.
- `bench path...`: median of 5 interleaved runs, MRI vs compiled.

```sh
./build                                          # ~4-5 min transpile, then the Go build (~7-10 min optimized, ~16 GB)
W=${TMPDIR:-/tmp}/rugaroo-rubocop
cp ../../prelude/array.rb $W/array.rb; cp ../../examples/005_word_count/main.rb $W/word_count.rb
./check a.rb array.rb word_count.rb
./bench array.rb
DEBUG=1 ./build                                  # ~4 min Go build; check works the same, bench is meaningless
```

To move to a new rubocop: install it, update the version in `setup`, regenerate each patch from the old one
(`patch` reports what did not apply), then `./build` and `./check` until every input is SAME.
