package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// CompileWithBoot is CompileFilesWithWarnings for a program that requires a
// gem: it runs the program's load phase under MRI first (internal/boot,
// decision 161) and adds each autoload boot resolved as a source, so the
// compiler sees classes the source graph's literal requires do not name. A
// program whose entry is not on disk is compiled as usual. gemSigs holds
// vendored `.rbs` signatures (decision 160), keyed under `sig/gems/<gem>/`.
// gemDirs are the gems' lib dirs, searched after loadPath (the program's own
// -I): only files under them are gem code (decisions 169, 172, 173).
func CompileWithBoot(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, gemDirs []string, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	aug, autoloaded, m, err := bootSources(ctx, sources, append(slices.Clip(loadPath), gemDirs...))
	if err != nil {
		return nil, nil, err
	}
	opts := options{loadPath: loadPath, gemDirs: gemDirs, gemSigs: gemSigs, pruneGems: true, autoloaded: autoloaded, boot: m}
	out, err := compile(ctx, preludeFS, aug, &opts)
	return out, opts.warnings, err
}

// CompileWithGems compiles with the installed gems MRI finds for requires only a gem answers (decision 173); without such a require it never starts Ruby.
func CompileWithGems(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, loadPath ...string) ([]byte, []string, error) {
	return CompileWithGemDirs(ctx, preludeFS, gemSigs, sources, nil, loadPath...)
}

// CompileWithGemDirs is CompileWithGems with directories the program carries as libraries (`-gem dir`, decision 406): gem code, searched before the installed gems.
func CompileWithGemDirs(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, gemDirs []string, loadPath ...string) ([]byte, []string, error) {
	if len(sources) == 0 {
		return nil, nil, errors.New("no Ruby files to compile")
	}
	names, err := gemRequires(ctx, sources, loadPath, gemDirs)
	var found []string
	if err == nil && len(names) > 0 {
		found, err = boot.FindGems(ctx, filepath.Dir(sourceName(sources[0])), names)
	}
	if err == nil {
		err = checkPrismGem(names, append(slices.Clip(gemDirs), found...))
	}
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // the caller adds "rugaroo: "
	}
	all := carriedGemDirs(gemDirs, found)
	if len(all) == 0 {
		return CompileFilesWithWarnings(ctx, preludeFS, sources, loadPath...)
	}
	return CompileWithBoot(ctx, preludeFS, gemSigs, sources, all, loadPath...)
}

// carriedGemDirs is gemDirs plus the installed gems' lib dirs found, less a gem the prelude is (base64, sqlite3) that came
// back as a dependency: the prelude's copy serves, and its shipped sig/ would type the prelude's by its own (decisions 173, 406).
func carriedGemDirs(gemDirs, found []string) []string {
	all := slices.Clip(gemDirs)
	for _, d := range found {
		if m := gemDir.FindStringSubmatch(d + "/"); m != nil && builtinLib(m[1]) {
			continue
		}
		all = append(all, d)
	}
	return all
}

// CompileTestsSkippingWithGems is CompileTestsSkipping for a suite whose requires an installed gem answers (cmd/rubyspec's
// library specs, decision 409): the gems are looked up under MRI from dir (its Gemfile, when it has one; decision 173) and
// compiled as a program's gems are (leniently, pruned to what the suite reaches). The sources are in memory, so nothing is
// booted: a gem whose load phase defines methods (decision 198) is compiled from its source alone.
func CompileTestsSkippingWithGems(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, seed *Inference, dir string) ([]byte, []string, []SkippedTest, error) {
	if len(sources) == 0 {
		return nil, nil, nil, errors.New("no Ruby files to compile")
	}
	names, err := gemRequires(ctx, sources, nil, nil)
	var found []string
	if err == nil && len(names) > 0 {
		found, err = boot.FindGems(ctx, dir, names)
	}
	if err != nil {
		return nil, nil, nil, err //nolint:wrapcheck // the caller adds "rugaroo: "
	}
	opts := options{skipTests: true, seed: seed, gemDirs: carriedGemDirs(nil, found), gemSigs: gemSigs}
	opts.pruneGems = len(opts.gemDirs) > 0
	out, err := compile(ctx, preludeFS, sources, &opts)
	return out, opts.warnings, opts.skipped, err
}

// bootSources runs boot on sources[0] and appends the files its resolved
// autoloads loaded, in first-seen order, that are not already sources, with each one's constant by real path.
func bootSources(ctx context.Context, sources []Source, loadPath []string) ([]Source, map[string][]string, *boot.Manifest, error) {
	entry := sourceName(sources[0])
	_, statErr := os.Stat(entry)
	if statErr != nil {
		return sources, nil, nil, nil //nolint:nilerr // no file on disk (the playground): nothing to boot
	}
	m, err := boot.Capture(ctx, entry, loadPath...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("boot: %w", err)
	}
	consts := map[string][]string{}
	seen := map[string]bool{}
	for _, s := range sources {
		seen[realPath(sourceName(s))] = true
	}
	out := sources
	for _, a := range m.Autoloads {
		if !a.Resolved || a.Loaded == "" || !strings.HasSuffix(a.Loaded, ".rb") {
			continue
		}
		if _, ok := consts[realPath(a.Loaded)]; ok { // one file under several autoloads (prism's visitor.rb: BasicVisitor, Visitor)
			consts[realPath(a.Loaded)] = append(consts[realPath(a.Loaded)], a.Const)
			continue
		}
		if seen[realPath(a.Loaded)] {
			continue
		}
		if d, _ := under(a.Loaded, m.StdDirs); d != "" { // MRI's own library (Bundler's and Thor's autoloads under `bundler/setup`): the prelude is rugaroo's copy, never a source (decisions 173, 404)
			continue
		}
		seen[realPath(a.Loaded)] = true
		data, rerr := os.ReadFile(a.Loaded)
		if rerr != nil {
			return nil, nil, nil, fmt.Errorf("boot: %w", rerr)
		}
		out = append(out, Source{Name: filepath.Base(a.Loaded), Src: data, Path: a.Loaded})
		consts[realPath(a.Loaded)] = []string{a.Const}
	}
	return out, consts, m, nil
}

// neededAutoloads keeps the boot-resolved autoload files whose constant another program file names (decision 175); the rest are marked loaded, so never compiled.
func (c *Compiler) neededAutoloads(consts map[string][]string) map[string]bool {
	if len(consts) == 0 {
		return nil
	}
	named := map[string]bool{}
	note := func(f *File) {
		anyNode(f.Root, func(n parser.Node) bool {
			switch n := n.(type) {
			case *parser.ConstantReadNode:
				named[n.Name] = true
			case *parser.ConstantPathNode:
				named[*n.Name] = true
			}
			return false
		})
	}
	for path, f := range c.parsed {
		if _, ok := consts[path]; !ok {
			note(f)
		}
	}
	needed := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for path, ks := range consts {
			if !needed[path] && slices.ContainsFunc(ks, func(k string) bool { return named[k] }) && c.parsed[path] != nil {
				needed[path], changed = true, true
				note(c.parsed[path])
			}
		}
	}
	for path := range consts {
		if !needed[path] {
			c.loaded[path] = true
		}
	}
	return needed
}

func sourceName(s Source) string {
	if s.Path != "" {
		return s.Path
	}
	return s.Name
}

// CompileTraced compiles a decision 197 entry, a plain Ruby file of the requires MRI loaded, then main (the CLI's
// -main, decision 288): the -I dirs are gem dirs (boot, leniency and pruning, decisions 161/169/172), boot runs the
// entry's load list, and main is compiled after it, never booted.
func CompileTraced(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, main string, gemDirs ...string) ([]byte, []string, error) {
	return compileTraced(ctx, preludeFS, gemSigs, sources, main, nil, gemDirs...)
}

// compileTraced is CompileTraced with the run-time string evals a trace saw, each compiled at its call site (decision
// 374).
func compileTraced(ctx context.Context, preludeFS, gemSigs fs.FS, sources []Source, main string, evals []boot.EvalInfo, gemDirs ...string) ([]byte, []string, error) {
	if len(sources) == 0 || main == "" {
		return nil, nil, errors.New("a traced compile needs an entry file and a main file")
	}
	tick := phaseTimer(loadEnv().Timing)
	aug, autoloaded, m, err := bootSources(ctx, sources, gemDirs)
	if err != nil {
		return nil, nil, err
	}
	tick("boot")
	if m != nil {
		m.Evals = append(m.Evals, evals...)
	}
	data, rerr := os.ReadFile(main) //nolint:gosec // the program's main file, named by the caller
	if rerr != nil {
		return nil, nil, fmt.Errorf("main: %w", rerr)
	}
	abs, aerr := filepath.Abs(main)
	if aerr == nil {
		main = abs // __dir__ is absolute, as MRI's
	}
	aug = append(aug, Source{Name: main, Src: data, Path: main})
	opts := options{gemDirs: gemDirs, gemSigs: gemSigs, pruneGems: true, autoloaded: autoloaded, boot: m, mainLast: true}
	out, err := compile(ctx, preludeFS, aug, &opts)
	warnings := opts.warnings
	tick("compile")
	if err != nil {
		return out, warnings, err
	}
	if dump := opts.env.GopoisonDump; dump != "" { // debugging: the Go before its rejected gem methods become stubs
		_ = os.WriteFile(dump+".0", out, 0o600)
	}
	out = stubRejected(out, func(fn, msg string) (string, bool) {
		g, ok := opts.goFuncs[fn]
		if ok {
			warnings = append(warnings, fmt.Sprintf("%s does not type-check in Go, so calling it raises NotImplementedError: %s (decision 205)", g.key, msg))
		}
		return "rugaroo: " + g.name + " does not compile: Go rejects it: " + msg, ok
	})
	tick("gocheck")
	return out, warnings, nil
}
