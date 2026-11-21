# Snails on rugaroo

[Snails](https://snails-rb.org/) is a Rails-style JSON API framework written for the
[Spinel](https://github.com/matz/spinel) AOT compiler. `./build [workdir]` assembles the app
`snails new demo --scaffold` makes (posts with nested comments, SQLite), runs it on MRI, compiles it with
this checkout's rugaroo, and diffs the HTTP answers: they are equal (11 requests: health, CRUD, a 422, 404s).

Snails' runtime is copied unchanged except for what is listed under "Upstream candidates".

## Layout

- `build`: the whole loop, from a pinned Snails commit.
- `main.rb`: the driver. It marks `snails-core/tep/net.rb` loaded, applies the schema, starts the server on a
  thread, and sends the requests.
- `lib/tep_net.rb`: stands in for `tep/net.rb`, whose `sp_net_*` functions live inside Spinel's C runtime.
  It provides the same fd-based `Tep::Sock` and `Tep::Crypto` API on `Socket`/`OpenSSL`.
- `to_ffi_gem.rb`: rewrites Spinel's `ffi_func`/`ffi_const`/`ffi_buffer`/... into the ffi gem's
  `attach_function` spelling, which MRI, Spinel and rugaroo all accept.
- `patches/`: fixes to Snails' Ruby.
- `sig/snails_fixes.rbs`: corrections to Snails' `.rbs` sidecars (decision 404's member-by-member override).
- `nil_default_audit.rb`: finds parameters whose Ruby default is `nil` while the sidecar type rejects nil.

## Findings

### Spinel bugs (verified against Spinel 858dde41b, Snails' pin)

1. **`private_constant` is not enforced.** `Form::FIELDS`, from inside or outside the class, resolves in
   Spinel; MRI raises `NameError: private constant Form::FIELDS referenced`. This hid finding 1 below.
   Reported as [matz/spinel#12343](https://github.com/matz/spinel/issues/12343) (#6910 fixed only `self::Name`).

### Upstream candidates (Snails)

1. `runtime/snails/form.rb`: `Form::FIELDS` names a private constant; MRI raises at the first `validates`.
   Lexical `FIELDS` is the fix (`patches/0001-form-private-constant.patch`).
2. `.rbs` sidecars say non-nil where the Ruby can be nil: `render(body = nil)`, `Request#initialize(env = nil)`,
   `find_by(order: nil)`, `fetch_many(order:/limit:/offset: nil)`, and `Logger::Context.controller/action/format`
   (a thread-local read before its first write). Spinel reads sidecars as hints, so it never noticed.
3. FFI in the ffi gem's spelling (`to_ffi_gem.rb` output) makes the database adapters run on MRI too. The one
   semantic gap: Spinel converts an Integer passed to `:ptr` (sqlite's `SQLITE_TRANSIENT`, `-1`) and the gem
   does not, so `sqlite3_bind_text`'s destructor slot becomes a pointer-sized `:long`. Check first that Spinel
   keeps direct C calls for the gem spelling when libffi is installed.
4. A `Socket`-backed `tep/net.rb` next to the `sp_net` one would let Snails' own tests run on MRI.

### rugaroo, fixed on this branch

1. In a method reopened on `Object`, `to_s`/`inspect`/`==` on self were bound to Kernel's generic version
   (`"abc".my_to_s` was `"#<String>"`), so Snails' `Object#blank?` was false for `""`.
2. `is_a?(C)` on self in an `Object` reopening was folded to `false`.
3. `Net::HTTP` reported a refused connection as `Net::OpenTimeout`; MRI raises `Errno::ECONNREFUSED`.
4. `IO#wait_readable` was missing. It now answers at once, as `BasicSocket`'s already did, and the read after it
   blocks (ponytail: poll(2) with the timeout).

### rugaroo, open

- `accept_nonblock(exception: false)` / `read_nonblock(..., exception: false)`: the keyword is refused.
- `app/` compiles only leniently (`-gem app -gem gen`). Strict typing stops at `@post.comments.map`, because
  `Sqlx::Query#find` is `-> untyped` and Snails leaves model types to Spinel's whole-program inference.
- Spinel's `ffi_*` syntax is not compiled natively. The gem-spelling rewrite (or a native mapping onto decision
  375's `attach_function` path) is the choice to make.
- Prefork (`-w N`) needs `fork`; Snails 1.0 clamps workers to 1, so it is not reached.
