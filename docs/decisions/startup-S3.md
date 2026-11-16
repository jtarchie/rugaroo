# Decision S3

**Rule:** `Dir.glob` reads an entry's type from the directory listing and calls `stat` only for a symlink (to learn whether it leads to a directory); a symlink that leads nowhere is listed, as in MRI.

**Glob entry types** (amends decision 94; 2026-11-16).

`rbGlobWalk` called `os.Stat` on every entry of every directory it read, although `os.ReadDir` had already returned each entry's type, and skipped an entry whose `stat` failed. rubocop globs `lib/rubocop/**/*.rb` twice while it starts (as it does under MRI): about 2,400 `stat` system calls for 1,908 results, 94M instructions against MRI's 57M (`docs/research/startup-2026-11.md`).

- **The rule.** A directory entry that is not a symlink is a directory or not by its own type (`DirEntry.IsDir`). A symlink is followed with one `stat`: to a directory, it is matched and entered by name (`ldir/*`, `*/`) but neither listed with a slash nor entered by `**`, as before and as MRI; to anything else, or to nothing, it is an entry that is not a directory. A literal path segment (`dangling`, no wildcard) falls back to `lstat` when `stat` fails, for the same reason.
- **A difference from MRI this removes.** A dangling symlink was missing from `Dir.glob("*")`, `Dir.glob("**/*")`, `Dir.glob("dangling")` and `Pathname.glob`; MRI lists it. Flags (`File::FNM_DOTMATCH`), braces, hidden names and the order of results are unchanged.
- **Measured:** the two globs of rubocop's 954 library files, 93.6M → 49.6M instructions (macOS arm64; MRI 57M).
- **Proof:** `testdata/test/stdlib_test.rb`'s `test_glob_symlinks` (a symlinked directory, a symlinked file, a dangling symlink and a symlink inside a subdirectory, under `*`, `**`, a trailing slash, braces and `FNM_DOTMATCH`), beside `test_glob`.
