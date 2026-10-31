package compiler

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

var (
	gemDir = regexp.MustCompile(`/gems/([^/]+?)-\d[^/]*/`)
	extDir = regexp.MustCompile(`/extensions/[^/]+/[^/]+/([^/]+?)-\d[^/]*/`)
)

// TracedEntry is a traced run's decision 197 entry for main, and the -I dirs its requires resolve from (decision 296).
// pureFallback are gems whose C extension rugaroo does without: racc's parser.rb rescues the LoadError of racc/cparse into
// its Ruby routines (decision 427).
var pureFallback = map[string]bool{"racc": true}

func TracedEntry(l *boot.Loaded, main string) (src []byte, dirs []string) {
	var b bytes.Buffer
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			fmt.Fprintf(&b, "require %s\n", rubyQuote(name))
		}
	}
	abs, err := filepath.Abs(main)
	if err == nil {
		main = abs
	}
	var gemFiles []string       // loaded files of -I dirs, scanned for their gem's own requires below
	native := map[string]bool{} // a gem with a C extension (fiddle) cannot compile: its requires stay unresolved
	for _, f := range l.Features {
		if m := extDir.FindStringSubmatch(f); m != nil {
			native[m[1]] = true
		}
	}
	for _, f := range l.Features {
		if f == main {
			continue
		}
		if m := gemDir.FindStringSubmatch(f); m != nil && (builtinLib(m[1]) || mriLib(m[1])) {
			add(m[1]) // a gem the prelude is: the vendored copy's files are its
			continue
		} else if lib := strings.ReplaceAll(m1(m), "-", "/"); m != nil && builtinLib(lib) {
			add(lib) // the net-http gem is the prelude's net/http, by RubyGems' naming (decision 404)
			continue
		} else if m != nil && native[m[1]] && !pureFallback[m[1]] {
			continue
		}
		if dir, rel := under(f, l.StdDirs); dir != "" {
			name := strings.TrimSuffix(rel, filepath.Ext(rel))
			if first, _, _ := strings.Cut(name, "/"); builtinLib(name) || mriLib(name) && first != "enc" && first != "bundler" && first != "rubygems" { // Bundler set up the load path the trace recorded; rerun at boot it hangs (decision 404)
				add(name) // MRI internals (encodings, portable Ruby's helpers) have no rugaroo meaning
			}
			continue
		}
		dir, rel := under(f, l.LoadPath)
		if dir == "" || !strings.HasSuffix(rel, ".rb") {
			continue // require_relative from a compiled file reaches it again; a C extension cannot compile
		}
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
		add(strings.TrimSuffix(rel, ".rb"))
		gemFiles = append(gemFiles, f)
	}
	addGemOwnRequires(l.LoadPath, gemFiles, seen, add)
	return b.Bytes(), dirs
}

// addGemOwnRequires adds each loaded gem file's literal require of a file in the same gem (patchelf's
// `require 'patchelf/alt_saver'` in Patcher#save, which MRI ran only in Homebrew's sandbox child), closed over what
// those require (decision 324). Only a vendored gem's own files: an application's lazy requires stay as traced.
func addGemOwnRequires(loadPath, gemFiles []string, seen map[string]bool, add func(string)) {
	traced := len(gemFiles)
	for i := 0; i < len(gemFiles); i++ { // appended to while walking
		f := gemFiles[i]
		root := gemDir.FindStringIndex(f)
		if root == nil {
			continue
		}
		src, err := os.ReadFile(f) //nolint:gosec // a file the traced run loaded
		if err != nil {
			continue
		}
		for _, name := range defRequires(src, i >= traced) { // a file added here loads its load-time requires too
			for _, d := range loadPath {
				cand := filepath.Join(d, name+".rb")
				if !strings.HasPrefix(cand, f[:root[1]]) || seen[name] {
					continue
				}
				_, err := os.Stat(cand)
				if err == nil {
					add(name)
					gemFiles = append(gemFiles, cand)
				}
			}
		}
	}
}

var (
	literalRequire = regexp.MustCompile(`^(\s*)require[ (]\s*['"]([^'"#]+)['"]`)
	defLine        = regexp.MustCompile(`^(\s*)(?:private |protected |public )?def [^=]*$|^(\s*)(?:private |protected |public )?def \S+\(.*\)\s*$`)
	endLine        = regexp.MustCompile(`^(\s*)end\b`)
)

// defRequires is the literal names a file requires inside a def's body (every one with all) (by indentation: a def line opens, the end at
// its indent closes), not a load-time require under a platform check (concurrent-ruby's TruffleRuby backend).
func defRequires(src []byte, all bool) []string {
	var out []string
	var open []int // indents of the defs being read
	for line := range strings.Lines(string(src)) {
		if m := defLine.FindStringSubmatch(line); m != nil {
			open = append(open, len(m[1])+len(m[2]))
			continue
		}
		if m := endLine.FindStringSubmatch(line); m != nil && len(open) > 0 && len(m[1]) == open[len(open)-1] {
			open = open[:len(open)-1]
			continue
		}
		if m := literalRequire.FindStringSubmatch(line); m != nil && (all || len(open) > 0 && len(m[1]) > open[len(open)-1]) {
			out = append(out, m[2])
		}
	}
	return out
}

// rubyQuote: Go's %q leaves `#` bare, so `"a#{b}"` in entry.rb would run b.
func rubyQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		switch ch := s[i]; {
		case ch == '\\' || ch == '"' || ch == '#':
			b.WriteByte('\\')
			b.WriteByte(ch)
		case ch < 0x20 || ch == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// under is the longest of dirs holding f, and f relative to it.
func under(f string, dirs []string) (dir, rel string) {
	for _, d := range dirs {
		if r, ok := strings.CutPrefix(f, strings.TrimSuffix(d, "/")+"/"); ok && len(d) > len(dir) {
			dir, rel = d, r
		}
	}
	return dir, rel
}

// CompileTrace runs main with args under MRI (rubyOpts first, then the -I dirs) and compiles what that run loaded as a
// traced compile of main (decision 296).
func CompileTrace(ctx context.Context, preludeFS, gemSigs fs.FS, main string, runs [][]string, rubyOpts []string, loadPath ...string) ([]byte, []string, error) {
	opts := slices.Clone(rubyOpts)
	for _, d := range loadPath {
		opts = append(opts, "-I", d)
	}
	tick := phaseTimer(loadEnv().Timing)
	l := &boot.Loaded{}
	for _, args := range runs { // each run's loads join the first's: one binary for several commands
		r, err := boot.Trace(ctx, main, args, opts)
		if err != nil {
			return nil, nil, err //nolint:wrapcheck // boot's errors name the trace
		}
		l.Features = appendUnseen(l.Features, r.Features)
		l.LoadPath = appendUnseen(l.LoadPath, r.LoadPath)
		l.StdDirs = appendUnseen(l.StdDirs, r.StdDirs)
		for _, e := range r.Evals {
			if !slices.Contains(l.Evals, e) {
				l.Evals = append(l.Evals, e)
			}
		}
	}
	tick("trace")
	src, dirs := TracedEntry(l, main)
	more, err := mainRequires(ctx, main, append(slices.Clone(loadPath), dirs...), src)
	if err != nil {
		return nil, nil, err
	}
	src = append(src, more...)
	dir, err := os.MkdirTemp("", "rugaroo-entry-")
	if err != nil {
		return nil, nil, fmt.Errorf("trace: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	entry := filepath.Join(dir, "entry.rb")
	err = os.WriteFile(entry, src, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("trace: %w", err)
	}
	if keep := os.Getenv("RUGAROO_KEEP_ENTRY"); keep != "" { // experiment
		_ = os.WriteFile(keep, src, 0o600)
	}
	srcs := []Source{{Name: "entry.rb", Src: src, Path: entry}}
	return compileTraced(ctx, preludeFS, gemSigs, srcs, main, l.Evals, append(slices.Clone(loadPath), dirs...)...)
}

// mainRequires is a `require` for each literal require in main, outside its methods, of a file on dirs that the entry
// lacks: the trace saw only the paths that ran, and main's own requires are its static closure (brew.rb's
// `rescue` requiring utils/backtrace, decision 296).
func mainRequires(ctx context.Context, main string, dirs []string, entry []byte) ([]byte, error) {
	data, err := os.ReadFile(main) //nolint:gosec // the program's main file, named by the caller
	if err != nil {
		return nil, fmt.Errorf("main: %w", err)
	}
	p, err := sharedParser()
	if err != nil {
		return nil, fmt.Errorf("prism: %w", err)
	}
	f, err := parseFile(ctx, p, main, data, originUser)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		if _, ok := n.(*parser.DefNode); ok {
			return
		}
		if call, ok := n.(*parser.CallNode); ok && call.Name == "require" && call.Receiver == nil {
			if str, ok := firstArg(callArgs(call)).(*parser.StringNode); ok && len(callArgs(call)) == 1 {
				line := fmt.Sprintf("require %s\n", rubyQuote(str.Unescaped.Value))
				if !bytes.Contains(entry, []byte(line)) && !bytes.Contains(b.Bytes(), []byte(line)) && onPath(str.Unescaped.Value, dirs) {
					b.WriteString(line)
				}
			}
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(f.Root)
	return b.Bytes(), nil
}

// appendUnseen is to with each of from it lacks, in order.
func appendUnseen(to, from []string) []string {
	for _, s := range from {
		if !slices.Contains(to, s) {
			to = append(to, s)
		}
	}
	return to
}

// onPath reports whether name.rb is a file in one of dirs.
func onPath(name string, dirs []string) bool {
	for _, d := range dirs {
		fi, err := fs.Stat(os.DirFS(d), name+".rb")
		if err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// m1 is a match's first group, "" without a match.
func m1(m []string) string {
	if m == nil {
		return ""
	}
	return m[1]
}
