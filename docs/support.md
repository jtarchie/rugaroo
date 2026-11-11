# What rugaroo supports

This page answers "will my program compile, and how far can I trust it?". Every
feature is in one of three tiers ([decision 373](decisions/373.md)):

| Tier | What it means |
|---|---|
| **stable** | Tested against MRI under `go test` (an example, a test file, or ruby/spec). Output that differs from `ruby` is a bug, unless a numbered decision documents the difference. |
| **experimental** | Works on the programs it is tested with. It may change or break between commits: the design is still moving, it depends on something outside the binary, or the tests cover far less than the feature. |
| **not supported** | A compile error that names the reason. Each one is a deliberate trade-off with its own decision. |

"Proof" links point at the programs that hold each line to MRI's output. The
ruby/spec numbers come from a dated `go run ./cmd/rubyspec` run (see
[the README](../README.md#performance-and-conformance) for the totals).

## Language — stable

| Feature | Proof | Notes |
|---|---|---|
| Classes, inheritance, `super`, modules, `include`/`extend`/`prepend` | [01](../examples/001_inheritance/main.rb), [15](../examples/015_namespaces/main.rb), [16](../examples/016_class_methods/main.rb), [27](../examples/027_extend_block/main.rb) | Every class and method must be visible at compile time (a closed world). |
| Blocks, procs, lambdas, `yield`, `Method` objects | [44](../examples/044_procs/main.rb), [98](../examples/098_method_objects/main.rb), [105](../examples/105_variadic_lambdas/main.rb), [108](../examples/108_proc_instance_eval/main.rb) | Some `break`/`return` forms out of blocks are compile errors (below). |
| Exceptions: `raise`, `rescue`, `ensure`, `retry`, custom hierarchies, `catch`/`throw` | [04](../examples/004_exceptions/main.rb), [13](../examples/013_exceptions_flow/main.rb), [14](../examples/014_uncaught/main.rb), [68](../examples/068_catch_throw/main.rb), [78](../examples/078_backtrace/main.rb) | Uncaught exceptions print and exit as MRI; `RUGAROO_BACKTRACE=1` adds MRI-style backtraces. |
| Control flow, `case`/`when`, pattern matching (`case`/`in`) | [08](../examples/008_control_flow/main.rb), [93](../examples/093_pattern_matching/main.rb) | |
| Arguments: optional, splat, keyword, block; multiple assignment; `\|\|=` | [12](../examples/012_optionals/main.rb), [21](../examples/021_or_assign/main.rb), [22](../examples/022_multiple_assign/main.rb), [88](../examples/088_keyword_arguments/main.rb) | |
| `Struct`, `Data` | [09](../examples/009_structs/main.rb), [28](../examples/028_struct_data/main.rb), [137](../examples/137_value_structs/main.rb) | A `Data.define` class with typed members, at most 4 of them in at most 32 bytes (two Strings, or four Integers/Floats), is held as a Go value, so `new` and `with` allocate nothing. A program that calls `equal?`, `object_id`, `assert_same` or `instance_variable_set` anywhere, or uses `ObjectSpace` or `Marshal`, keeps every Data class a pointer, as do a Data block that defines `initialize` or writes an instance variable ([decision 441](decisions/441.md)). |
| Ruby 4.0 syntax (`it`, endless methods, leading `&&`) | [07](../examples/007_ruby4_syntax/main.rb), [87](../examples/087_ruby_spec_syntax/main.rb) | |
| Constants and reflection: `const_get`, `constants`, `instance_variable_get`/`set`, `defined?`, `respond_to?` | [26](../examples/026_constants_reflection/main.rb), [106](../examples/106_ivar_defined/main.rb) | Answered from the whole program at compile time, never by `reflect` ([decision 123](decisions/123.md)). |
| `method_missing`, `send` and `public_send` | [29](../examples/029_method_missing/main.rb), [30](../examples/030_dynamic_send/main.rb) | On `untyped` values the call goes through a generated dispatcher ([dynamic dispatch](dynamic-dispatch.md)). |
| `Class.new`/`Module.new` literals, `class_eval`/`module_eval` in a class body | [100](../examples/100_class_new/main.rb), [109](../examples/109_class_eval/main.rb) | One class per literal, declared at compile time ([decision 145](decisions/145.md)). |
| `require_relative`, `require` with `-I`, `autoload` | [89](../examples/089_require_relative/main.rb), [90](../examples/090_load_path/main.rb), [118](../examples/118_autoload/main.rb) | The path must be a string literal at a file's top level. |
| `at_exit`, `trap`, `freeze`/`frozen?` | [72](../examples/072_freeze/main.rb), [75](../examples/075_trap/main.rb) | |
| Types: rbs-inline `#:` annotations, `.rbs` files in `sig/` (with overloads), union types, inference | [20](../examples/020_untyped/main.rb), [34](../examples/034_inferred_returns/main.rb), [101](../examples/101_inferred_params/main.rb), [104](../examples/104_union_types/main.rb), [107](../examples/107_rbs_sig/main.rb), [117](../examples/117_rbs_overloads/main.rb) | Inline overloads (`#\|`) are not supported; put them in a `.rbs` file. A local or unannotated instance variable assigned several classes is their union ([decision 150](decisions/150.md), [decision 15](decisions/015.md)). A union member must be a class, tuple, proc or nil, not a module ([decision 385](decisions/385.md)), and two instantiations of one generic class (`Array[Integer]`, `Array[String]`) are no union. |
| Threads, `Queue`, `Mutex`, `Monitor`, `Fiber`, enumerator generators | [23](../examples/023_threads/main.rb), [42](../examples/042_queues/main.rb), [79](../examples/079_monitor/main.rb), [92](../examples/092_generators/main.rb) | Goroutines and channels underneath, a Fiber an `iter.Pull` coroutine, and `rand` lock-free unless the program seeds (decision 403); programs that start goroutines are built with `-race` in the tests. |

## Language — experimental

| Feature | Proof | Notes |
|---|---|---|
| `Ractor`, `Ractor::Port` | [77](../examples/077_ractor/main.rb), [ractor_test](../testdata/test/ractor_test.rb) | A goroutine and a queue; messages are deep-copied and isolation is checked at compile time. The isolation rules are new. |
| `# @value` on a plain class: hold its objects as Go values, not pointers | [137](../examples/137_value_structs/main.rb), [value_test](../testdata/test/value_test.rb) | A comment line `# @value` right above `class C` says C's objects are values: nothing changes them after `initialize`, and nothing needs to tell two equal ones apart. rugaroo then copies them instead of allocating each on the heap, which makes small vector or point classes several times faster. The default `==`, `eql?` and `hash` compare instance variables (as `Data` does) where MRI compares identity, and the default `inspect`/`to_s` shows no `0x...` address. It is a compile error, naming the reason, when C has a subclass, superclass or user module, more than 4 instance variables or more than 32 bytes of them, a Proc or untyped instance variable, a writer, an instance variable written outside `initialize`, `self` used as a value in `initialize`, an `initialize_copy`, or when typed code calls `equal?`, `object_id`, `freeze` or `instance_variable_set` on it ([decision 441](decisions/441.md)). |

## Language — not supported

| Feature | Why | Instead |
|---|---|---|
| `eval`, `instance_eval(string)`, `load`, code built at run time | The binary has no interpreter; everything is compiled ahead of time. | Write the code out. Gems that `eval` at load time can still compile (see [gems](#gems--experimental)). |
| `define_method`, `define_singleton_method` in your own code | Methods must be known at compile time. | `def`, or `method_missing` with a signature ([29](../examples/029_method_missing/main.rb)). A gem's load-time `define_method` is supported ([decision 198](decisions/198.md)). |
| In-place String mutation (`upcase!`, `gsub!`, `[]=`; `<<` on a String something else may hold) | A `String` is an immutable Go string value ([decision 58](decisions/058.md)). `s << x` and `s.concat(x)` do work on a local the compiler proves nothing else holds: built fresh, not a parameter, not stored or passed before its last append ([decision 382](decisions/382.md)). A method may append to its String parameter, return it, or pass it on to another such method: the caller passes a local, and the call's value counts as that local ([decision 405](decisions/405.md)). An append through a block parameter (`rows.each { \|r\| r << "\n" }`, an Array element) or an attribute (`page.out << x`) stays a compile error by design; in gem code it raises `NotImplementedError` instead of losing the append. `s.setbyte(i, b)` works on such a local too, and on an ivar that `initialize` first sets to a new String and changes before `self` or the String escapes; before the last `setbyte` the String may only be read for numbers (`getbyte`, `bytesize`, `==`, ...), not sliced, copied or interpolated ([decision 395](decisions/395.md)). The error names what breaks the proof. | Build new strings (`s += "x"`, `s = s.upcase`), append to a local and pass it on after the last `<<` ([130](../examples/130_string_append/main.rb)), `setbyte` a `dup` and return it ([133](../examples/133_string_setbyte/main.rb)), replace elements with new Strings (`rows.map! { \|r\| r + "\n" }`), or use `StringIO`. |
| The 34 encodings listed under [Encodings](#encodings) (ISO-2022-JP, Big5-HKSCS, macGreek, ...) | x/text has no table for them, or they keep state between characters ([decision 380](decisions/380.md)). | Convert at the edges with another tool. |
| `break v`; a `break` out of a block whose call's value is used; `return` from a block stored as a closure; `next v` where the value is thrown away (`while`, `each`) | Go closures cannot unwind their caller ([decision 4](decisions/004.md)). | Restructure: plain `break`/`return` work in `while` and in blocks that run inline (like `each`). |
| Inline overloaded signatures (`#\|`) | | Declare overloads in a `.rbs` file ([117](../examples/117_rbs_overloads/main.rb)). |
| `require` of a computed path, `$LOAD_PATH` changes at run time | Files are read when the program compiles. | String-literal paths at a file's top level; `-I dir` for load paths; `-trace` for gems ([gems.md](gems.md)). |

## Core classes — stable

ruby/spec numbers: `go run ./cmd/rubyspec core` on 2026-10-30 (commit f393057d; `core/io` at 657de61d). "Of those run" leaves out examples skipped because they use something rugaroo leaves out by design (Bignum by default, String mutation, `eval`, ...); the skipped column counts those.

| Class | Tier | ruby/spec pass (of those run) | Skipped | Notes |
|---|---|---:|---:|---|
| `Array` | stable | 612 of 704 (87%) | 511 |  |
| `Array#pack`, `String#unpack` | stable | 2,556 of 3,004 (85%) | 173 | No C pointers (`P`, `p`); Bignum directives need `promote`. |
| `Hash` | stable | 245 of 276 (89%) | 286 | Insertion-ordered; `Hash#[]` is typed `V?`. |
| `String` | stable | 1,019 of 1,333 (76%) | 1,030 | Immutable ([decision 58](decisions/058.md)), except `<<`/`concat` on an unaliased local ([decision 382](decisions/382.md)) and `setbyte` on one, or on an ivar in `initialize` ([decision 395](decisions/395.md)). |
| `Symbol` | stable | 84 of 104 (81%) | 74 |  |
| `Integer` | stable | 185 of 265 (70%) | 333 | 64-bit; see below. |
| `Float` | stable | 151 of 181 (83%) | 80 |  |
| `Rational`, `Complex` | stable | 144 of 168 (86%) | 158 | Mixed arithmetic coerces as MRI. |
| `Numeric` | stable | 27 of 55 (49%) | 260 |  |
| `Range` | stable | 296 of 339 (87%) | 267 |  |
| `Set` | stable | 69 of 79 (87%) | 97 | Core, as in MRI 4.0. |
| `Struct`, `Data` | stable | 102 of 142 (72%) | 116 |  |
| `Enumerable` | stable | 166 of 219 (76%) | 272 | Elements with equal keys can come out of `sort_by`, `max_by(n)` and `min_by(n)` in another order than MRI's, whose order depends on the platform's `qsort_r`. |
| `Enumerator`, `Enumerator::Lazy`, arithmetic sequences | stable | 47 of 70 (67%) | 278 | `next`/`peek`, `Enumerator.new` generators. |
| `Comparable` | stable | 0 of 46 (0%) | 8 | ruby/spec's fixture class doesn't compile, so every example errors; covered by rugaroo's own tests. |
| `Regexp`, `MatchData` | stable | 253 of 342 (74%) | 98 | Ruby syntax translated to Go's regexp engines. |
| `Time` | stable | 326 of 407 (80%) | 245 |  |
| `Random` | stable | 53 of 54 (98%) | 33 | The same sequence as MRI for a given seed. |
| `Math` | stable | 116 of 117 (99%) | 126 |  |
| `File`, `Dir`, `FileTest` | stable | 685 of 805 (85%) | 425 |  |
| `File::Stat` | stable | 8 of 108 (7%) | 27 | Partial. |
| `IO`, `ARGF`, `$stdin` | stable | 377 of 598 (63%) | 861 | `IO.popen`, `system` and backticks run real processes. |
| `ENV` | stable | 75 of 97 (77%) | 96 |  |
| `Kernel` | stable | 690 of 1,147 (60%) | 749 | No `eval`, `binding` or `load`. |
| `Exception` and its subclasses | stable | 90 of 155 (58%) | 95 |  |
| `Proc`, `Method`, `UnboundMethod` | stable | 97 of 245 (40%) | 153 |  |
| `Module`, `Class` | stable | 189 of 332 (57%) | 772 | No `define_method` or runtime reopening in user code. |
| `BasicObject`, `Object`, `nil`, `true`, `false` | stable | 66 of 117 (56%) | 66 |  |
| `Thread`, `Queue`, `SizedQueue`, `Mutex`, `ThreadGroup` | stable | 158 of 240 (66%) | 238 | Goroutines and channels; `SizedQueue` is a Go channel (decision 403). |
| `Fiber` | stable | 25 of 68 (37%) | 121 |  |
| `Signal`, `Process` | stable | 35 of 81 (43%) | 236 | `trap`, `Process.kill`, `spawn`, `wait`. |
| `Marshal` | stable | 30 of 255 (12%) | 97 | Round-trips its own bytes, not MRI's, so specs that compare bytes fail; `load` also reads an MRI stream of core values (a gem's data file). |
| `Encoding` | stable | 73 of 125 (58%) | 13 | 60 encodings ([list](#encodings)), no per-String tag ([decisions 136](decisions/136.md), [439](decisions/439.md)). |
| `Warning`, `GC`, `ObjectSpace` | stable | 15 of 22 (68%) | 93 | The parts a program can observe; no heap walking. |
| `Encoding::Converter` | not supported | 0 of 142 (0%) | 1 | Use `String#encode`. |
| `Binding`, `TracePoint`, refinements | not supported | 0 of 58 (0%) | 104 | Need an interpreter at run time. |
| `IO::Buffer`, `ObjectSpace::WeakMap`/`WeakKeyMap`, `Enumerator::Chain`/`Product` | not supported | 9 of 250 (4%) | 47 | Not written yet, except an `ObjectSpace::WeakMap` that holds its values strongly; `WeakRef` (`require "weakref"`) covers weak references. |

`Integer` is 64-bit: overflow raises `RangeError` unless the program opts into
`--int-overflow=wrap` (stable) or `promote` (Bignum, experimental;
[decision 372](decisions/372.md)). `Hash#[]` is typed `V?`, so typed code
nil-checks it or uses `fetch` ([decision 5](decisions/005.md)), unless the
program opts into `--nil-check=runtime` or a `# nil_check: runtime` line
(experimental; [decision 393](decisions/393.md)), which checks it when it runs
as MRI does. A read from an unannotated `[]`/`{}` that rugaroo typed itself is
checked that way by default, with a warning ([decision 440](decisions/440.md)). `Marshal`
round-trips its own bytes, not MRI's.

### Encodings

`String#encode`, `Integer#chr`, `File.read`/`File.open`/`IO#set_encoding` and
`Encoding.find` handle 60 encodings. Past the ten rugaroo does itself, they
come from `golang.org/x/text`, with MRI's own mapping, byte rules and error
messages wherever x/text's differ ([decision 380](decisions/380.md)): every
byte and character was compared with MRI 4.0.

- **Unicode and binary:** UTF-8, UTF-16, UTF-16BE, UTF-16LE, UTF-32, UTF-32BE, UTF-32LE, US-ASCII, ASCII-8BIT.
- **Single-byte:** ISO-8859-1 to -11 and -13 to -16, TIS-620, Windows-874 and -1250 to -1257, KOI8-R, KOI8-U,
  IBM437, CP850, IBM852, CP852, IBM855, CP855, IBM860, IBM862, IBM863, IBM865, IBM866, macRoman, macCyrillic.
- **Multibyte:** Shift_JIS, Windows-31J (SJIS, CP932), EUC-JP, CP51932, EUC-KR, CP949, GBK (CP936), GB2312,
  GB18030, Big5, CP950.

A String carries no encoding of its own: it is text (UTF-8) or, where the
compiler sees it, binary ([decision 352](decisions/352.md)). `force_encoding`
with any other name (`"ISO-8859-1"`, `"Shift_JIS"`) keeps the bytes and reads
them as UTF-8 (a compile-time warning when the name is a literal), and US-ASCII reports as UTF-8; convert text in another encoding
where it enters, with `s.encode("UTF-8", "ISO-8859-1")`
([decision 439](decisions/439.md)).

Not supported, though MRI converts them: naming one in a literal is a compile
error, and a name built at run time raises `Encoding::ConverterNotFoundError`.

| Encodings | Why |
|---|---|
| ISO-2022-JP, stateless-ISO-2022-JP, CP50220, CP50221, ISO-2022-JP-KDDI, stateless-ISO-2022-JP-KDDI | Escape sequences switch a state that lasts across characters; rugaroo's transcoder reads one character at a time. |
| Big5-HKSCS | Some of its characters decode to two code points. |
| Big5-UAO, CP951, eucJP-ms, EUC-JIS-2004, GB12345 | No x/text table. |
| IBM720, IBM737, IBM775, IBM857, IBM861, IBM864, IBM869, macCroatian, macGreek, macIceland, macRomania, macTurkish, macUkraine | No x/text table. |
| IBM037 | EBCDIC: not ASCII-compatible. |
| SJIS-DoCoMo, SJIS-KDDI, SJIS-SoftBank, UTF8-DoCoMo, UTF8-KDDI, UTF8-SoftBank | Carrier emoji tables, which x/text does not have. |
| UTF8-MAC, CESU-8 | Unicode variants with their own rules (decomposed file names, surrogate pairs in UTF-8). |

MRI has no converter for Windows-1258, EUC-TW, Emacs-Mule, GB1988,
macCentEuro, macThai, ISO-2022-JP-2, MacJapanese or UTF-7 either.

## Standard library

A library loads only when the program has a literal `require` for it
([decision 155](decisions/155.md)); the names below are every `require` the
compiler answers. A standard library name not listed compiles as a no-op.

| `require` | Tier | Proof | Notes |
|---|---|---|---|
| `abbrev` | stable | [63](../examples/063_abbrev/main.rb) | |
| `base64` | stable | [46](../examples/046_digests/main.rb), [stdlib_test](../testdata/test/stdlib_test.rb) | |
| `benchmark` | stable | [48](../examples/048_time_parse/main.rb), [stdlib_test](../testdata/test/stdlib_test.rb) | |
| `bigdecimal`, `bigdecimal/util` | stable | [82](../examples/082_bigdecimal/main.rb), [bigdecimal_test](../testdata/test/bigdecimal_test.rb) | Over Go's `math/big`, with bigdecimal 4.1's precision rules. |
| `cgi`, `cgi/escape`, `cgi/util` | stable | [46](../examples/046_digests/main.rb), [stdlib_test](../testdata/test/stdlib_test.rb) | Escaping helpers. |
| `csv` | stable | [49](../examples/049_csv/main.rb), [85](../examples/085_csv_headers/main.rb) | |
| `date` | stable | [39](../examples/039_date/main.rb), [94](../examples/094_datetime/main.rb), [date_test](../testdata/test/date_test.rb) | Proleptic Gregorian only: MRI's Julian dates before 1582 are not modelled. |
| `delegate` | stable | [86](../examples/086_delegation/main.rb) | `DelegateClass(Foo)` is typed; `SimpleDelegator` dispatches dynamically. |
| `digest`, `digest/md5`, `digest/sha1`, `digest/sha2` | stable | [46](../examples/046_digests/main.rb) | |
| `English` | stable | [globals_test](../testdata/test/globals_test.rb), ruby/spec `library/English` | English.rb's names for the punctuation globals, which rugaroo knows whether or not it is required ([decision 415](decisions/415.md)); `$;`, `$,` and `$\` are honored by `split`, `join` and `print`. `$.` (`$NR`) is the last read's line number ([decision 425](decisions/425.md)); no `$_` (`$LAST_READ_LINE`): `gets` keeps no last line. |
| `erb` | stable | [81](../examples/081_erb/main.rb), [erb_test](../testdata/test/erb_test.rb) | A template that is a string literal compiles with the program ([decision 111](decisions/111.md)). |
| `etc` | stable | [60](../examples/060_etc/main.rb) | |
| `fcntl` | stable | [stdlib_test](../testdata/test/stdlib_test.rb) | Constants for `IO#fcntl`. |
| `fileutils` | stable | [54](../examples/054_fileutils/main.rb), [fileutils_test](../testdata/test/fileutils_test.rb) | |
| `find` | stable | [56](../examples/056_find/main.rb) | |
| `forwardable` | stable | [74](../examples/074_forwardable/main.rb) | |
| `ipaddr` | stable | [59](../examples/059_ipaddr/main.rb) | |
| `json` | stable | [19](../examples/019_json/main.rb), [json_test](../testdata/test/json_test.rb) | |
| `logger` | stable | [58](../examples/058_logger/main.rb) | |
| `minitest`, `minitest/autorun`, `minitest/spec`, `minitest/mock`, `minitest/pride` | stable | [65](../examples/065_minitest/main.rb), [66](../examples/066_minitest_spec/main.rb), and rugaroo's own suite | A port of minitest 6.0.6; `rugaroo test` runs a directory of test files as one program. |
| `net/http`, `net/https` | stable | [24](../examples/024_net_http/main.rb), [net_test](../testdata/test/net_test.rb) | Client and server. |
| `observer` | stable | [64](../examples/064_observable/main.rb) | |
| `open-uri` | stable | [95](../examples/095_open_uri/main.rb) | |
| `open3` | stable | [57](../examples/057_open3/main.rb) | |
| `optparse`, `optionparser` | stable | [76](../examples/076_optparse/main.rb) | |
| `ostruct` | stable | [86](../examples/086_delegation/main.rb) | Fields are `untyped`. |
| `rexml`, `rexml/document` | stable | [84](../examples/084_rexml/main.rb), [rexml_test](../testdata/test/rexml_test.rb) | A DOM and an XPath subset over a Go tokenizer. |
| `securerandom` | stable | [46](../examples/046_digests/main.rb) | |
| `shellwords` | stable | [47](../examples/047_strscan/main.rb) | |
| `singleton` | stable | [50](../examples/050_format_regexp/main.rb) | |
| `socket` | stable | [91](../examples/091_socket/main.rb), [socket_test](../testdata/test/socket_test.rb) | TCP, UDP and Unix sockets over Go's `net`; `Socket.new` is not supported. |
| `stringio` | stable | [45](../examples/045_stringio/main.rb) | |
| `strscan` | stable | [47](../examples/047_strscan/main.rb) | |
| `tempfile` | stable | [54](../examples/054_fileutils/main.rb) | No finalizer: close and unlink explicitly. |
| `time` | stable | [48](../examples/048_time_parse/main.rb) | `Time.parse`, `iso8601`, `strptime`, ... |
| `timeout` | stable | [61](../examples/061_timeout/main.rb) | |
| `tmpdir` | stable | [56](../examples/056_find/main.rb) | |
| `tsort` | stable | [62](../examples/062_tsort/main.rb) | |
| `uri` | stable | [stdlib_test](../testdata/test/stdlib_test.rb) | |
| `weakref` | stable | [80](../examples/080_weakref/main.rb) | |
| `webrick` | stable | [24](../examples/024_net_http/main.rb), [115](../examples/115_rack_server/main.rb) | |
| `zlib` | stable | [46](../examples/046_digests/main.rb) | Checksums, Deflate/Inflate, gzip. |
| `fiddle`, `fiddle/import`, `fiddle/closure`, `fiddle/cparser`, `fiddle/function` | experimental | [121](../examples/121_fiddle/main.rb), [fiddle_test](../testdata/test/fiddle_test.rb) | Calls C libraries without cgo, through purego ([decision 326](decisions/326.md)), or through cgo with `-cgo`, which also makes variadic calls (`TYPE_VARIADIC`) and calls with more than 8 arguments ([decision 426](decisions/426.md)). Needs the library on the machine that runs the binary. |
| `ffi` | experimental | [127](../examples/127_ffi/main.rb), [ffi_test](../testdata/test/ffi_test.rb) | The ffi gem's `FFI::Library`, `Pointer`, `MemoryPointer`, `Struct`, `Union`, callbacks and `enum` over `fiddle`'s purego path ([decision 375](decisions/375.md)). `attach_function` needs literal types; no `attach_variable`, and varargs, more than 8 arguments and `Struct.by_value` (with a literal `layout`) need `-cgo` ([decision 426](decisions/426.md)). Needs the library at run time. |
| `sqlite3` | experimental | [122](../examples/122_sqlite3/main.rb), [sqlite3_test](../testdata/test/sqlite3_test.rb) | The sqlite3 gem's API from the vendored pure-Ruby [sqlite3-fiddle](https://github.com/jtarchie/sqlite3-fiddle) over `fiddle` ([decision 328](decisions/328.md)). Needs libsqlite3 at run time. |
| `openssl` | experimental | [openssl_test](../testdata/test/openssl_test.rb) | RSA public-key verification, `HMAC.digest`/`hexdigest`, `PKCS5.pbkdf2_hmac` (digests named by String) and SSL verify modes: no signing, key generation or TLS of its own ([decisions 281](decisions/281.md), [404](decisions/404.md)). |
| `sorbet-runtime` | experimental | [119](../examples/119_sorbet_sigs/main.rb), [sorbet_test](../testdata/test/sorbet_test.rb) | `sig`, `T.let`, `T.cast`, `T.must` are read as types; what sorbet-runtime would raise is a compile error ([decision 196](decisions/196.md)). |
| `rubygems` | experimental | (Homebrew) | Only the surface a program that requires RubyGems itself touches; no gem loading. |
| `yaml`, `psych` | experimental | [128](../examples/128_yaml/main.rb), [yaml_test](../testdata/test/yaml_test.rb) | Psych's `load`/`safe_load`/`unsafe_load` (permitted classes and symbols, aliases, merge keys, YAML 1.1 scalars), `load_file`, `load_stream`, `dump`/`to_yaml`, parsed by [go-yaml](https://github.com/goccy/go-yaml) ([decision 376](decisions/376.md)). Dumped text and syntax-error messages are not libyaml's, but load back the same. No collection keys when loading, no `!ruby/object`, no `Psych.parse` tree. |
| `prism`, `prism/prism` | experimental | [136](../examples/136_prism_parse/main.rb), [prism_test](../testdata/test/prism_test.rb) | The prism gem's own Ruby, bundled at 1.9.0 with the prism.wasm the compiler parses with, which runs in the program ([decision 427](decisions/427.md)): `parse`, `lex`, `parse_lex`, `parse_comments`, `dump`/`load`, `parse_success?`, `StringQuery`, visitors. No `ruby` or installed gem needed. `Translation::*` (parser, ruby_parser, Ripper) and `lex_compat` do not run yet; `parse_stream` reads the whole stream; not in the browser playground. |
| `rexml/xmltokens`, `rexml/parsers/baseparser` | experimental | [123](../examples/123_kramdown/main.rb) | The name patterns only, for gems (kramdown) that build regexps from them. |

## Gems — experimental

How to use each route, and what to do when a gem fails: [gems.md](gems.md).

Compiling a program that loads a gem *runs* the gem's load phase under `ruby`
at build time, with your environment and no sandbox
([decision 161](decisions/161.md)). Only point rugaroo at code you would run.

| How the gem gets in | Tested with | Notes |
|---|---|---|
| Installed gem, compiled from source (`require "x"`, through Bundler when a `Gemfile` is beside the program) | rack, rackup, Cuba ([115](../examples/115_rack_server/main.rb), [116](../examples/116_cuba_web/main.rb)), pairing_heap ([120](../examples/120_pairing_heap/main.rb)); Ruby's bundled matrix, prime, getoptlong, net-ftp and drb, measured by ruby/spec's `library/` ([decisions 409](decisions/409.md)-[420](decisions/420.md)) | Needs `ruby` on PATH at build time, not at run time. Metaprogramming that ran at load time is recorded and compiled in. |
| Roundhouse's `--target spinel` output: a Rails app as plain Ruby and `.rbs` sidecars | roundhouse's Blog ([135](../examples/135_roundhouse_blog/README.md)) | Spinel's FFI files are replaced by shims the program loads first; the sidecars type the rest ([decisions 404](decisions/404.md), [405](decisions/405.md)). |
| `-trace`: run the program once under MRI and compile what it loaded | kramdown ([123](../examples/123_kramdown/main.rb)), liquid ([124](../examples/124_liquid/main.rb)), Homebrew's `brew` (with a [maintainer harness](../overrides/brew/README.md) that patches what rugaroo won't compile) | `-trace-also` merges more runs into one binary, `-main` names the program a traced entry runs ([decisions 296](decisions/296.md), [297](decisions/297.md)). |
| Vendored gem (`gems/`, compiled as your code) | sqlite3-fiddle | See `sqlite3` above. |

## Tools

| Tool | Tier | Notes |
|---|---|---|
| `rugaroo run`, `build`, `gen`, `test` | stable | Shaped like `go run`/`build`/`test`; `-race`, `-gcflags`, `-o`, `-work`, `-I` pass through; no Go debug info unless `-debug` (decision 437). |
| `--int-overflow=raise` (default), `wrap` | stable | [decision 372](decisions/372.md) |
| `--int-overflow=promote` | experimental | Bignum where MRI would grow past 64 bits ([126](../examples/126_bignum/main.rb)). |
| `--nil-check=compile` (default), `runtime` | experimental (`runtime`) | A `T?` where `T` is expected raises on nil when it runs instead of failing the compile ([132](../examples/132_nil_check_runtime/main.rb), [decision 393](decisions/393.md)). |
| `-trace`, `-trace-also`, `-rubyopt`, `-main` | experimental | See gems above. |
| `-compiler tinygo` | experimental | Builds with TinyGo instead of `go build`: smaller binaries; no sockets, `os/exec`, C calls or backtraces, and threads are slow. Proven by an opt-in lane, `RUGAROO_COMPILER=tinygo go test -run TestExamples .`, with a skip list ([decision 443](decisions/443.md)). |
| `rugaroo web` and the [playground](https://play.rugaroo.dev) | experimental | [decision 102](decisions/102.md) |
