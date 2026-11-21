# Snails on rugaroo

[Snails](https://snails-rb.org/) is a Rails-style JSON API framework written for the
[Spinel](https://github.com/matz/spinel) AOT compiler. `./build [workdir]` assembles the app
`snails new demo --scaffold` makes (posts with nested comments, SQLite), runs it on MRI, compiles it with
this checkout's rugaroo, and diffs the HTTP answers: they are equal (13 requests: health, CRUD, a 422, 404s).

Snails' runtime is copied unchanged except for what is listed under "Upstream candidates".

For building your own Snails app, use the [snails-rugaroo](https://github.com/jtarchie/snails-rugaroo) gem
(`snails-rugaroo build`): it packages the stand-ins, fixes and signatures here and leaves the app untouched. This
directory stays as the test and benchmark harness.

## Serving: Tep::APP on WEBrick

Everything Snails does per request sits behind `Tep::APP.dispatch(req, res)`, over plain request and response
objects. `lib/tep_webrick.rb` serves that on WEBrick, which under rugaroo is Go's `net/http` (a goroutine per
request) and under MRI the webrick gem. tep's own server, with its fd numbers, `IO.for_fd` and prefork `fork`,
stays loaded but never runs. The request goes through tep's own `Parser.parse` and `parse_body_params`, so
query, cookie and body handling stay Snails' code. Not served yet (they answer 501): streaming, WebSocket
upgrades, `Set-Cookie`.

## Benchmark

`bench/setup workdir spinel-checkout` builds [snails-benchmark](https://github.com/snails-rb/snails-benchmark)'s
scenario app (`GET /up`, and `GET /items/:id` as a primary-key read over 10,000 SQLite rows) with Spinel (tep's own
server, as `snails build-binary` makes it) and with rugaroo (`Tep::APP` on WEBrick/`net/http`), both on Snails main
91b921e5. `bench/run workdir` drives them with k6 the way its `bin/bench` does: constant connections, an 8-connection
warmup, the contenders' order rotating each round, logging off.

2026-11-21, Apple M4 (10 cores), macOS. 15 s × 3 rounds, median RPS (p95 ms):

| route | connections | Spinel | rugaroo | rugaroo / Spinel |
|---|---|---|---|---|
| `/up` | 8 | 18,234 (0.96) | 24,744 (0.59) | 1.36× |
| `/up` | 32 | 18,878 (4.78) | 29,639 (2.64) | 1.57× |
| `/up` | 128 | 17,173 (22.60) | 28,413 (14.67) | 1.65× |
| `/items/:id` | 8 | 12,463 (1.29) | 16,452 (0.88) | 1.32× |
| `/items/:id` | 32 | 12,408 (7.31) | 18,318 (4.79) | 1.48× |
| `/items/:id` | 128 | 12,404 (32.97) | 17,506 (26.54) | 1.41× |

Peak RSS: Spinel 51 MiB, rugaroo 164 MiB. No request failed. MRI (WEBrick) was about 200 RPS in a 3 s trial.

Read with care. This compares two stacks, not only two compilers: tep's server against Go's `net/http`. The host
was shared (load average about 15 from other work), k6 ran on the same cores (the published method pins it apart
with taskset on Linux), and Spinel's build had no jemalloc, which its default `spin.toml` asks for. The published
Linux numbers for Spinel are higher (50,865 and 35,610 RPS at 32 connections), so only the ratio here means
anything, and it should be re-measured on Linux.

## Layout

- `build`: the whole loop, from a pinned Snails commit.
- `main.rb`: the driver. It marks `snails-core/tep/net.rb` loaded, applies the schema, starts
  `Tep::WEBrickServer.start_app` on a thread, and sends the requests.
- `lib/tep_webrick.rb`: the adapter above. `start_app` copies `Snails::Main.start`'s setup (an upstream `serve`
  hook would remove the copy).
- `lib/tep_net.rb`: stands in for `tep/net.rb`, whose `sp_net_*` functions live inside Spinel's C runtime: the
  clock, `Tep::Crypto` on OpenSSL, writes that fail (Spinel's load-time type seeding writes to fd 0), and the
  fd-level calls raising `NotImplementedError`.
- FFI: Snails' `database/*.rb` compile as written; rugaroo reads Spinel's `ffi_*` syntax (decision 375's Snails
  amendment). MRI gets it from `testdata/run/spinel_ffi_mri/spinel/ffi.rb` on `RUBYLIB`.
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

### rugaroo, fixed on this branch

1. In a method reopened on `Object`, `to_s`/`inspect`/`==` on self were bound to Kernel's generic version
   (`"abc".my_to_s` was `"#<String>"`), so Snails' `Object#blank?` was false for `""`.
2. `is_a?(C)` on self in an `Object` reopening was folded to `false`.
3. `Net::HTTP` reported a refused connection as `Net::OpenTimeout`; MRI raises `Errno::ECONNREFUSED`.
4. `WEBrick::HTTPRequest#header` joined a repeated header's values; MRI's keeps them apart
   (`{"x-a" => ["1", "2"]}`).
5. A WEBrick request ran on the main Thread, so concurrent requests shared `Thread.current` locals. Snails keeps
   its SQLite lease there, so two requests used one connection and `sqlite3_step` segfaulted. Each request now
   runs in a Thread of its own (per request; MRI keeps one per connection).
6. `IO#wait_readable` was missing. It now answers at once, as `BasicSocket`'s already did, and the read after it
   blocks (ponytail: poll(2) with the timeout). The WEBrick path no longer needs it.
7. Spinel's `ffi_*` syntax is read as the ffi gem's (decision 375's Snails amendment), so Snails' database adapters
   compile unchanged.

### rugaroo, open

- `accept_nonblock(exception: false)` / `read_nonblock(..., exception: false)`: the keyword is refused. Snails no
  longer needs it.
- `app/` compiles only leniently (`-gem app -gem gen`) until model types are declared (#104 above).
- Spinel's `ffi_struct` and `ffi_source` (a C fragment) are not read; Snails uses neither.
