# Decision S2

**Rule:** `__dir__` is resolved once per source file, at the file's first `__dir__`, by a `sync.OnceValue` package variable of that file; later calls answer the kept String.

**`__dir__` once per file** (2026-11-16). *See also decisions 155, 163.*

`__dir__` compiled to `rbSourceDir("file.rb")` at each call: `filepath.Abs`, then `filepath.EvalSymlinks`, one `lstat` per path component. rubocop's `cop/style.rb` and its siblings call it once per cop they register (`autoload`-style paths built from `__dir__`): 619 calls, 8,030 `lstat` system calls for 8 distinct files, 115M of the 2,193M instructions of a run on an empty file (`docs/research/startup-2026-11.md`).

- **The mechanism.** `sourceDirVar` emits `var rbDir_<hash of the file name> = sync.OnceValue(func() String { return rbSourceDir("file.rb") })` and the call is `rbDir_<hash>()`. One variable per source file that says `__dir__`, dropped by the pruner with the code that named it; no table and no lock shared between files. A file that never calls it resolves nothing, so start-up does not pay for the files that mention `__dir__` only in methods that do not run.
- **Why this is MRI's behaviour, more than before.** MRI fixes a file's real path when it loads the file, and `__dir__` is `File.dirname` of that: it does not change if the program later changes directory or a directory on the path is replaced by a symlink. The per-call resolution could answer differently on a later call (`Dir.chdir` with a relative source name); the kept value cannot. What still differs from MRI: the value is fixed at the first `__dir__`, not at load, so a relative source name is resolved against the working directory of that first call; and the main file's name is the name the compiler was given (the CLI passes its base name), so run from another directory its `__dir__` is the working directory. Both were true before.
- **Measured:** 619 `filepath.EvalSymlinks` calls on rubocop's path are 115M instructions on macOS (a replica); rubocop itself in the branch's end-of-work table in decision S1's file.
- **Proof:** `testdata/test/control_test.rb` (`__dir__` against `File.expand_path(__FILE__)`), `testdata/run/gem_lenient.rb` and `require_shims.rb`.
