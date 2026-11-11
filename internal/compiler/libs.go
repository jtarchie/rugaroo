package compiler

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// preludeLibs maps what a user `require` names to the prelude file that defines it (decision 155). Anything not here is
// core: loaded by prelude.rb whether required or not, as MRI 4.0 has it without a require (Set, Pathname, pp, Monitor,
// Process::Status, GC). A lib file require_relatives what MRI's own file loads, so `require "csv"` defines Date and
// StringIO as it does there. A name MRI has and this table lacks stays a no-op, as before.
var preludeLibs = map[string]string{
	"json":                     "prelude/json.rb",
	"__errno_process":          "prelude/errno_process.rb",
	"__reflection":             "prelude/reflection.rb", // no require: a private_methods/protected_methods call (requireScan.visit, decision 416)
	"__bignum":                 "prelude/bignum.rb",     // no require: --int-overflow=promote (decision 372) // no require of its own: named by Errno::ESRCH/EPERM (requireScan.visit)
	"stringio":                 "prelude/stringio.rb",
	"fileutils":                "prelude/fileutils.rb",
	"tempfile":                 "prelude/tempfile.rb",
	"tmpdir":                   "prelude/tmpdir.rb",
	"find":                     "prelude/find.rb",
	"logger":                   "prelude/logger.rb",
	"base64":                   "prelude/base64.rb",
	"digest":                   "prelude/digest.rb",
	"digest/md5":               "prelude/digest.rb",
	"digest/sha1":              "prelude/digest.rb",
	"digest/sha2":              "prelude/digest.rb",
	"zlib":                     "prelude/zlib.rb",
	"securerandom":             "prelude/securerandom.rb",
	"cgi":                      "prelude/cgi.rb",
	"cgi/escape":               "prelude/cgi.rb",
	"cgi/util":                 "prelude/cgi.rb",
	"strscan":                  "prelude/strscan.rb",
	"shellwords":               "prelude/shellwords.rb",
	"abbrev":                   "prelude/abbrev.rb",
	"benchmark":                "prelude/benchmark.rb",
	"csv":                      "prelude/csv.rb",
	"singleton":                "prelude/singleton.rb",
	"etc":                      "prelude/etc.rb",
	"timeout":                  "prelude/timeout.rb",
	"uri":                      "prelude/uri.rb",
	"ipaddr":                   "prelude/ipaddr.rb",
	"openssl":                  "prelude/openssl.rb",
	"net/http":                 "prelude/net_http.rb",
	"net/https":                "prelude/net_http.rb",
	"webrick":                  "prelude/webrick.rb",
	"open-uri":                 "prelude/open_uri.rb",
	"socket":                   "prelude/socket.rb",
	"open3":                    "prelude/open3.rb",
	"tsort":                    "prelude/tsort.rb",
	"observer":                 "prelude/observable.rb",
	"weakref":                  "prelude/weakref.rb",
	"erb":                      "prelude/erb.rb",
	"bigdecimal":               "prelude/bigdecimal.rb",
	"bigdecimal/util":          "prelude/bigdecimal.rb",
	"forwardable":              "prelude/forwardable.rb",
	"optparse":                 "prelude/optparse.rb",
	"optionparser":             "prelude/optparse.rb",
	"minitest":                 "prelude/minitest.rb",
	"minitest/autorun":         "prelude/minitest.rb",
	"minitest/spec":            "prelude/minitest.rb",
	"minitest/mock":            "prelude/minitest.rb",
	"minitest/pride":           "prelude/minitest.rb",
	"rexml":                    "prelude/rexml.rb",
	"rexml/document":           "prelude/rexml.rb",
	"rexml/xmltokens":          "prelude/rexml_tokens.rb",
	"rexml/parsers/baseparser": "prelude/rexml_tokens.rb",
	"ostruct":                  "prelude/ostruct.rb",
	"delegate":                 "prelude/delegate.rb",
	"date":                     "prelude/date.rb",
	"time":                     "prelude/time_parse.rb",
	"sorbet-runtime":           "prelude/sorbet.rb",
	"rubygems":                 "prelude/rubygems.rb",
	"fiddle":                   "prelude/fiddle.rb",
	"fiddle/import":            "prelude/fiddle_import.rb",
	"fiddle/cparser":           "prelude/fiddle_import.rb",
	"fiddle/closure":           "prelude/fiddle.rb",
	"fiddle/function":          "prelude/fiddle.rb",
	"fcntl":                    "prelude/fcntl.rb",
	"ffi":                      "prelude/ffi.rb",
	"yaml":                     "prelude/yaml.rb",
	"psych":                    "prelude/yaml.rb",
	"prism/prism":              "prelude/prism.rb",
}

// mriLibs are MRI 4.0's standard library, by first path segment or whole name (`net/protocol` is the stdlib's; `net/ftp`,
// `net/smtp`, `net/pop` and `net/imap` are bundled gems to look up, decision 409): never a gem to look up, so requiring
// them never starts Ruby.
var mriLibs = map[string]bool{
	"English": true, "bundler": true, "cgi": true, "complex": true, "continuation": true, "coverage": true, "date": true,
	"date_core": true, "delegate": true, "did_you_mean": true, "digest": true, "enc": true, "enumerator": true, "erb": true,
	"error_highlight": true, "etc": true, "expect": true, "fcntl": true, "fiber": true, "fileutils": true, "find": true,
	"forwardable": true, "io": true, "ipaddr": true, "json": true, "mkmf": true, "monitor": true, "net/protocol": true,
	"objspace": true, "open-uri": true, "open3": true, "optionparser": true, "optparse": true,
	"pathname": true, "pp": true, "prettyprint": true, "psych": true, "pty": true, "random": true,
	"rational": true, "rbconfig": true, "resolv": true, "ripper": true, "ruby2_keywords": true, "rubygems": true,
	"securerandom": true, "set": true, "shellwords": true, "singleton": true, "socket": true, "stringio": true,
	"strscan": true, "syntax_suggest": true, "tempfile": true, "thread": true, "time": true, "timeout": true,
	"tmpdir": true, "tsort": true, "un": true, "unicode_normalize": true, "uri": true, "weakref": true, "yaml": true,
	"zlib": true,
}

// vendoredLibs maps a require name to a vendored gem's lib file in the prelude FS (vendored/<gem>/, a mirror of the gems/<gem> submodule): pure Ruby that runs on MRI as
// a gem and is compiled here as user code (strict typing, extern synthesis), never looked up as an installed gem (decision 328).
var vendoredLibs = map[string]string{
	"sqlite3": "vendored/sqlite3-fiddle/sqlite3.rb",
}

// tinygoUnbuildable maps a require name's first segment to why TinyGo cannot build or run it (decision 443): an error at the require, not at link or run time.
var tinygoUnbuildable = map[string]string{
	"socket":   "TinyGo's net has no real sockets",
	"net":      "TinyGo's net has no real sockets",
	"webrick":  "TinyGo's net has no real sockets",
	"open-uri": "TinyGo's net has no real sockets",
	"open3":    "TinyGo has no os/exec",
	"fiddle":   "purego, Fiddle's C calls, does not build under TinyGo",
	"ffi":      "purego, Fiddle's C calls, does not build under TinyGo",
	"sqlite3":  "purego, Fiddle's C calls, does not build under TinyGo",
	"prism":    "wazero, which runs prism.wasm, does not build under TinyGo",
}

// shadowedGemFile is the prelude lib that stands in for a gem file: prism's FFI backend, which the prism/prism lib replaces (decision 427).
func shadowedGemFile(target string) string {
	if strings.HasSuffix(filepath.ToSlash(target), "/lib/prism/ffi.rb") {
		return "prism/prism"
	}
	return ""
}

// mriLib reports a require name MRI's standard library answers (mriLibs, by whole name or first path segment).
func mriLib(name string) bool {
	name = strings.TrimSuffix(name, ".rb")
	return mriLibs[name] || mriLibs[strings.SplitN(name, "/", 2)[0]]
}

// builtinLib reports a require name the prelude or a vendored gem answers.
func builtinLib(name string) bool {
	name = strings.TrimSuffix(name, ".rb")
	return preludeLibs[name] != "" || vendoredLibs[name] != "" || bundledGems[name]
}

// gemRequires lists required names (through -I files too) only an installed gem can answer (decision 173); it only parses.
func gemRequires(ctx context.Context, sources []Source, loadPath, gemDirs []string) (names []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			err = ce
		}
	}()
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	c := &Compiler{parser: p, loadPath: loadPath, gemDirs: gemDirs, env: loadEnv()}
	libs, _, err := c.scanRequires(ctx, sources)
	if c.methodRequiresRuntime {
		return nil, err // decision 197: the traced entry names every library through -I; nothing to look up
	}
	// ponytail: a missing gem's require under `rescue LoadError` still errors; mark guarded requires in requireScan to skip them.
	for _, lib := range libs {
		name := strings.TrimSuffix(lib, ".rb")
		if !builtinLib(name) && !mriLib(name) {
			names = append(names, name)
		}
	}
	return names, err
}

// scanRequires parses the user sources and walks them, and every file they require_relative or require off the -I
// path, for the literal `require "x"` names the program mentions anywhere (top level, in a method, inside
// `begin … rescue LoadError`): each loads at compile time before any user file is collected, so a lib's classes exist
// wherever its require stands, as decision 130 does for files. A non-literal argument at the top level is a compile
// error; inside a method it is a no-op, since a gem's computed require (Rack::Builder.parse_file's `require path`) is
// reached only at run time, when the closed world already holds what it names. A file with
// `__END__` needs StringIO for DATA. The parsed files come back keyed by real path, for the collect pass to reuse.
func (c *Compiler) scanRequires(ctx context.Context, sources []Source) (libs []string, parsed map[string]*File, err error) {
	sc := &requireScan{c: c, ctx: ctx, parsed: map[string]*File{}, seen: map[string]bool{}, top: true}
	for _, src := range sources {
		path := realPath(sourceName(src))
		if sc.seen[path] {
			continue
		}
		sc.seen[path] = true
		uf, perr := parseFile(ctx, c.parser, src.Name, src.Src, originSource)
		if perr != nil {
			return nil, nil, perr
		}
		uf.path = path
		sc.parsed[path] = uf
		sc.walk(uf)
	}
	return sc.libs, sc.parsed, nil
}

// requireScan is scanRequires' state: the files parsed so far by real path, and the lib names found in order.
type requireScan struct {
	c      *Compiler
	ctx    context.Context //nolint:containedctx // a short-lived walker
	parsed map[string]*File
	seen   map[string]bool // real paths walked, and "lib:name" for libs found
	libs   []string
	top    bool // at a file's top level (not inside a method): a computed require there is an error
}

func (sc *requireScan) walk(f *File) {
	if f.data != nil {
		sc.lib("stringio")
	}
	prev := sc.top // a require inside a method that names a file walks it: the caller's position is restored after
	sc.top = true
	sc.visit(f, f.Root)
	sc.top = prev
}

// unlinksPath reports File.delete/unlink and FileUtils.rm/remove: they raise EPERM for a directory on macOS.
func unlinksPath(call *parser.CallNode) bool {
	switch constName(call.Receiver) {
	case "File", "::File":
		return call.Name == "delete" || call.Name == "unlink"
	case "FileUtils", "::FileUtils":
		return call.Name == "rm" || call.Name == "remove"
	}
	return false
}

func (sc *requireScan) lib(name string) {
	if !sc.seen["lib:"+name] {
		sc.seen["lib:"+name] = true
		sc.libs = append(sc.libs, name)
	}
}

func (sc *requireScan) visit(f *File, n parser.Node) {
	if n == nil {
		return
	}
	if call, ok := n.(*parser.CallNode); ok && call.Receiver == nil && (call.Name == "require" || call.Name == "require_relative") {
		if !sc.top && sc.c.methodRequiresRuntime {
			// decision 197: resolved at run time against what top-level requires compiled in; a prelude library is
			// compiled in here too, as MRI's trace never loads one a method requires late (Homebrew's openssl, decision 281)
			if args := callArgs(call); call.Name == "require" && len(args) == 1 {
				if str, ok := args[0].(*parser.StringNode); ok && builtinLib(str.Unescaped.Value) {
					sc.lib(str.Unescaped.Value)
				}
			}
			return
		}
		sc.require(f, call, sc.top)
		return
	}
	if call, ok := n.(*parser.CallNode); ok && sc.top && f.path != "" {
		if p := sc.c.loadedFeature(f, call); p != "" { // a shim stands in for it: never walked (decision 404)
			sc.seen[sc.c.real(p)] = true
			return
		}
	}
	if cp, ok := n.(*parser.ConstantPathNode); ok && cp.Name != nil && (*cp.Name == "ESRCH" || *cp.Name == "EPERM") && constName(cp.Parent) == "Errno" {
		sc.lib("__errno_process") // decision 241
	}
	if call, ok := n.(*parser.CallNode); ok && unlinksPath(call) {
		sc.lib("__errno_process") // unlink(2) of a directory is EPERM on macOS (decision 339)
	}
	if call, ok := n.(*parser.CallNode); ok && (call.Name == "private_methods" || call.Name == "protected_methods") {
		sc.lib("__reflection") // decision 416
	}
	if _, ok := n.(*parser.DefNode); ok { // a method body: a computed require there is reached only at run time
		prev := sc.top
		sc.top = false
		for _, ch := range n.CompactChildNodes() {
			sc.visit(f, ch)
		}
		sc.top = prev
		return
	}
	for _, ch := range n.CompactChildNodes() {
		sc.visit(f, ch)
	}
}

// require handles one require/require_relative call: a lib name is recorded; a file is read, parsed and walked.
func (sc *requireScan) require(f *File, call *parser.CallNode, top bool) {
	args := callArgs(call)
	var str *parser.StringNode
	if len(args) == 1 {
		str, _ = args[0].(*parser.StringNode)
	}
	if str == nil {
		if top && !sc.c.methodRequiresRuntime { // decision 197: the trace compiled in what a computed require loads
			sc.c.errorf(f, call, "%s needs a string literal: rugaroo loads libraries at compile time", call.Name)
		}
		return // a computed require in a method: a no-op at run time
	}
	name := str.Unescaped.Value
	if why := tinygoUnbuildable[strings.SplitN(strings.TrimSuffix(name, ".rb"), "/", 2)[0]]; why != "" && call.Name == "require" && sc.c.env.Compiler == "tinygo" {
		sc.c.errorf(f, call, "require %q is not supported under -compiler tinygo: %s (decision 443)", name, why)
	}
	var target string
	if call.Name == "require_relative" {
		target = relativeTarget(f, name)
		if lib := shadowedGemFile(target); lib != "" {
			sc.lib(lib)
			return
		}
	} else if target = sc.c.findRequire(f, call); target == "" {
		sc.lib(name)
		return
	}
	path := sc.c.real(target)
	if sc.seen[path] {
		return
	}
	sc.seen[path] = true
	src, err := sc.c.readUser(target)
	if err != nil {
		return // the collect pass reports it where it stands
	}
	uf, err := parseFile(sc.ctx, sc.c.parser, path, src, originSource)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	uf.path = path
	if uf.nilCheck == "" && call.Name == "require_relative" { // the program's own files take its mode (decision 404)
		uf.nilCheck = f.nilCheck
	}
	sc.parsed[path] = uf
	sc.walk(uf)
}

// loadLibs loads the prelude files the program's requires name, in require order; every lib when all is set (the stub
// generator, decision 154, and RUGAROO_ALL_LIBS=1 for comparing against the old always-loaded prelude).
func (c *Compiler) loadLibs(ctx context.Context, libs []string, all bool) {
	if all {
		for _, lib := range slices.Sorted(maps.Keys(preludeLibs)) {
			if !gemBackends[preludeLibs[lib]] { // it names its gem's classes: only a program with the gem compiles it (decision 427)
				c.loadPrelude(ctx, preludeLibs[lib])
			}
		}
		for _, lib := range slices.Sorted(maps.Keys(vendoredLibs)) {
			c.loadVendored(ctx, vendoredLibs[lib])
		}
		return
	}
	for _, lib := range libs {
		if file := preludeLibs[strings.TrimSuffix(lib, ".rb")]; file != "" {
			c.loadPrelude(ctx, file)
		} else if file := vendoredLibs[strings.TrimSuffix(lib, ".rb")]; file != "" {
			c.loadVendored(ctx, file)
		}
	}
}

// genRuntimeRequire is a require inside a method under decision 197: true when what it names was compiled in (or is a
// library the prelude or MRI answers), else MRI's LoadError, since the file is outside the closed world.
func (f *fctx) genRuntimeRequire(n *parser.CallNode) expr {
	args := callArgs(n)
	str, ok := firstArg(args).(*parser.StringNode)
	if !ok && len(args) == 1 && n.Name == "require" { // a computed name: checked against the compiled files at run time
		a := f.genExpr(args[0], f.cls("String"))
		if !typeEq(a.typ, f.cls("String")) {
			a.code = "rbMust(" + a.code + ", \"String\").(String)"
		}
		f.c.needRequireTable = true
		return expr{code: "rbRequireComputed(" + a.code + ")", typ: f.cls("Boolean")}
	}
	if !ok || len(args) != 1 {
		return expr{code: "false", typ: f.cls("Boolean"), lit: true} // ponytail: a computed require_relative is assumed compiled in
	}
	name := str.Unescaped.Value
	var target string
	if n.Name == "require_relative" {
		target = relativeTarget(f.f, name)
	} else {
		target = f.c.findRequire(f.f, n)
	}
	switch {
	case target != "" && f.c.loaded[f.c.real(target)]:
	case target == "" && n.Name == "require" && (builtinLib(name) || mriLib(name)):
	default:
		call := &parser.CallNode{Location: n.Location, Name: "__rugaroo_load_error", Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{str}}}
		return f.genExpr(call, nil)
	}
	return expr{code: "false", typ: f.cls("Boolean"), lit: true}
}

// emitRequireTable is rbRequireComputed: `require name` for a name built at run time is false (loaded at start) when
// name, absolute or under a -I or gem lib directory, with or without `.rb`, is a compiled file or a library the prelude
// or MRI answers, or is under RbConfig's rubylibdir (MRI's library is compiled in, decision 296); else MRI's LoadError.
func (c *Compiler) emitRequireTable() {
	names := map[string]bool{}
	for _, f := range c.files {
		if f.isPrelude() || f.path == "" {
			continue
		}
		for _, p := range []string{f.path, c.real(f.path)} {
			p, _ = filepath.Abs(strings.TrimSuffix(p, ".rb"))
			names[p] = true
			for _, dir := range slices.Concat(c.loadPath, c.gemDirs) {
				dir, _ = filepath.Abs(dir)
				if within(dir, p) {
					rel, _ := filepath.Rel(dir, p) // within already checked it
					names[filepath.ToSlash(rel)] = true
				}
			}
		}
	}
	for lib := range preludeLibs {
		names[lib] = true
	}
	for lib := range vendoredLibs {
		names[lib] = true
	}
	c.w("func rbRequireComputed(name String) Boolean {\n\tif strings.HasPrefix(string(name), rbLibDir+\"/\") {\n\t\treturn false\n\t}\n")
	c.w("\tswitch strings.TrimSuffix(string(name), \".rb\") {\n\tcase ")
	for i, n := range slices.Sorted(maps.Keys(names)) {
		if i > 0 {
			c.w(",\n\t\t")
		}
		c.w("%s", strconv.Quote(n))
	}
	c.w(":\n\t\treturn false\n\t}\n")
	c.w("\tswitch first, _, _ := strings.Cut(strings.TrimSuffix(string(name), \".rb\"), \"/\"); first {\n\tcase ")
	for i, n := range slices.Sorted(maps.Keys(mriLibs)) {
		if i > 0 {
			c.w(", ")
		}
		c.w("%q", n)
	}
	c.w(":\n\t\treturn false\n\t}\n")
	c.w("\tpanic(NewLoadError(Ref(String(\"cannot load such file -- \" + string(name)))))\n}\n\n")
}
