# Snails on rugaroo

[Snails](https://snails-rb.org/) is a Rails-style JSON API framework written for the
[Spinel](https://github.com/matz/spinel) AOT compiler. `./build [workdir]` assembles the app
`snails new demo --scaffold` makes (posts with nested comments, SQLite), runs it on MRI, compiles it with
this checkout's rugaroo, and diffs the HTTP answers: they are equal (13 requests: health, CRUD, a 422, 404s).

Snails' runtime is copied unchanged except for what is listed under "Upstream candidates".

## Serving: Tep::APP on WEBrick

Everything Snails does per request sits behind `Tep::APP.dispatch(req, res)`, over plain request and response
objects. `lib/tep_webrick.rb` serves that on WEBrick, which under rugaroo is Go's `net/http` (a goroutine per
request) and under MRI the webrick gem. tep's own server, with its fd numbers, `IO.for_fd` and prefork `fork`,
stays loaded but never runs. The request goes through tep's own `Parser.parse` and `parse_body_params`, so
query, cookie and body handling stay Snails' code. Not served yet (they answer 501): streaming, WebSocket
upgrades, `Set-Cookie`.

## Layout

- `build`: the whole loop, from a pinned Snails commit.
- `main.rb`: the driver. It marks `snails-core/tep/net.rb` loaded, applies the schema, starts
  `Tep::WEBrickServer.start_app` on a thread, and sends the requests.
- `lib/tep_webrick.rb`: the adapter above. `start_app` copies `Snails::Main.start`'s setup (an upstream `serve`
  hook would remove the copy).
- `lib/tep_net.rb`: stands in for `tep/net.rb`, whose `sp_net_*` functions live inside Spinel's C runtime: the
  clock, `Tep::Crypto` on OpenSSL, writes that fail (Spinel's load-time type seeding writes to fd 0), and the
  fd-level calls raising `NotImplementedError`.
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
   (a thread-local read before its first write). Filed as
   [snails-rb/snails#105](https://github.com/snails-rb/snails/issues/105).
3. Model finders and associations are `untyped` in the generated `.rbs`. Filed as
   [snails-rb/snails#104](https://github.com/snails-rb/snails/issues/104).
4. A server seam: split `Snails::Main.start` into setup and `serve(port, workers)`, so another server (this
   adapter, or a Rack adapter) is one override and prefork stays inside tep's own server.
5. FFI in the ffi gem's spelling (`to_ffi_gem.rb` output) makes the database adapters run on MRI too. The one
   semantic gap: Spinel converts an Integer passed to `:ptr` (sqlite's `SQLITE_TRANSIENT`, `-1`) and the gem
   does not, so `sqlite3_bind_text`'s destructor slot becomes a pointer-sized `:long`. Check first that Spinel
   keeps direct C calls for the gem spelling when libffi is installed.

### rugaroo, fixed on this branch

1. In a method reopened on `Object`, `to_s`/`inspect`/`==` on self were bound to Kernel's generic version
   (`"abc".my_to_s` was `"#<String>"`), so Snails' `Object#blank?` was false for `""`.
2. `is_a?(C)` on self in an `Object` reopening was folded to `false`.
3. `Net::HTTP` reported a refused connection as `Net::OpenTimeout`; MRI raises `Errno::ECONNREFUSED`.
4. `WEBrick::HTTPRequest#header` joined a repeated header's values; MRI's keeps them apart
   (`{"x-a" => ["1", "2"]}`).
5. `IO#wait_readable` was missing. It now answers at once, as `BasicSocket`'s already did, and the read after it
   blocks (ponytail: poll(2) with the timeout). The WEBrick path no longer needs it.

### rugaroo, open

- `accept_nonblock(exception: false)` / `read_nonblock(..., exception: false)`: the keyword is refused. Snails no
  longer needs it.
- `app/` compiles only leniently (`-gem app -gem gen`) until model types are declared (#104 above).
- Spinel's `ffi_*` syntax is not compiled natively. The gem-spelling rewrite (or a native mapping onto decision
  375's `attach_function` path) is the choice to make.
