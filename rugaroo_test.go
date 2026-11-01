package rugaroo

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/tools/txtar"

	"github.com/jtarchie/rugaroo/internal/compiler"
)

// gemCmd runs a Gemfile executable through bundler, so the versions pinned
// in Gemfile.lock are used regardless of what is on PATH.
func gemCmd(t *testing.T, exe string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.CommandContext(t.Context(), "bundle", append([]string{"exec", exe}, args...)...) //nolint:gosec // test helper; args are ours
}

// toolchainErr checks the whole toolchain once per process (it spawns ruby and two bundle execs): the ruby .mri-version names, rbs-inline,
// rbs and golangci-lint.
// Every caller fails with the one error, so the fail-fast stays; it runs on a background context since the first test to ask may end before the others.
var toolchainErr = sync.OnceValue(func() error {
	ctx := context.Background()
	out, err := exec.CommandContext(ctx, "ruby", "-e", "print RUBY_VERSION").Output()
	if err != nil {
		return fmt.Errorf("ruby is required: %w", err)
	}
	// MRI is the oracle and its bundled gems are the Gemfile's pins, so a patch release moves expectations (decision 413);
	// not .ruby-version, which version managers (mise) would act on
	want, err := os.ReadFile(".mri-version")
	if err != nil {
		return fmt.Errorf("reading .mri-version: %w", err)
	}
	if v := strings.TrimSpace(string(want)); string(out) != v {
		return fmt.Errorf("ruby %s is required (.mri-version), found %s; to move to %s, see docs/decisions/413.md", v, out, out)
	}
	for _, c := range [][]string{{"rbs-inline", "--help"}, {"rbs", "--version"}} {
		out, err = exec.CommandContext(ctx, "bundle", append([]string{"exec"}, c...)...).CombinedOutput() //nolint:gosec // fixed args
		if err != nil {
			return fmt.Errorf("%s is required (run `bundle install`): %w\n%s", c[0], err, rdocNoise.ReplaceAll(out, nil))
		}
	}
	_, err = exec.LookPath("golangci-lint")
	if err != nil {
		return fmt.Errorf("golangci-lint is required: %w", err)
	}
	return nil
})

// requireToolchain fails the test unless ruby is .mri-version's, the Gemfile's rbs tools and golangci-lint are present.
func requireToolchain(t *testing.T) {
	t.Helper()
	err := toolchainErr()
	if err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, name string, args ...string) (string, error) {
	t.Helper()
	out, stderr, code, err := runIO(t, dir, progIO{}, name, args...)
	if err == nil && code != 0 {
		err = errors.New(stderr)
	}
	return out, err
}

// progIO is what a case feeds its program: `# args:` (space-separated), `# env: K=V` and `# stdin: "Go-quoted"` lines.
type progIO struct {
	env   []string
	stdin string
	trace bool // the trace oracle (decision 105): RUGAROO_MT_TRACE on both runs, MRI with testdata/mt_trace.rb preloaded
}

func runIO(t *testing.T, dir string, pio progIO, name string, args ...string) (stdout, stderr string, code int, err error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // test helper; args are ours
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), pio.env...)
	cmd.Stdin = strings.NewReader(pio.stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.String(), errOut.String(), ee.ExitCode(), nil
	}
	if err != nil {
		return out.String() + errOut.String(), errOut.String(), -1, err
	}
	return out.String(), errOut.String(), 0, nil
}

// mustGlob is filepath.Glob that fails the test on an error or no match: a moved directory must not turn a suite into zero subtests that pass.
func mustGlob(t *testing.T, pattern string) []string {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no files match %s", pattern)
	}
	return paths
}

func TestExamples(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dirs := mustGlob(t, "examples/*/main.rb")
	// Every example's Go lands in one module, so vet and golangci-lint load
	// the standard library once for all of them instead of once per example.
	mod := filepath.Join(t.TempDir(), "gen")
	writeModule(t, mod)
	t.Run("each", func(t *testing.T) {
		for _, rb := range dirs {
			dir := filepath.Dir(rb)
			t.Run(filepath.Base(dir), func(t *testing.T) {
				t.Parallel()
				testExample(t, dir, filepath.Join(mod, filepath.Base(dir)))
			})
		}
	})
	if t.Failed() {
		return
	}
	lintGenerated(t, mod)
}

// lintGenerated vets and lints every generated example at once with the generated-code config.
func lintGenerated(t *testing.T, mod string) {
	t.Helper()
	// GOFLAGS=-trimpath: vet and golangci-lint compile the module for export data through the Go build cache, keyed by directory unless trimmed; with it, the fresh t.TempDir hits what the examples' builds and the last run's lint already cached (4s instead of ~5 min when nothing changed).
	trim := progIO{env: []string{"GOFLAGS=-trimpath"}}
	start := time.Now()
	vetOut, vetErr, code, err := runIO(t, mod, trim, "go", "vet", "./...")
	timed("vet", t.Name(), time.Since(start))
	if err != nil || code != 0 {
		err = errors.Join(err, errors.New(vetErr))
		t.Fatalf("go vet: %v\n%s", err, vetOut)
	}
	lintCfg, err := os.ReadFile(".golangci.generated.yml")
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(mod, ".golangci.yml"), lintCfg, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	copyPreludeGo(t, mod)
	// --timeout=0: under the full suite's parallel builds this pass outlasts the config's 5m; go test's -timeout still bounds it
	start = time.Now()
	lintOut, lintErr, code, err := runIO(t, mod, trim, "golangci-lint", "run", "--allow-parallel-runners", "--timeout=0", "./...")
	timed("lint", t.Name(), time.Since(start))
	if err != nil || code != 0 {
		err = errors.Join(err, errors.New(lintErr))
		t.Fatalf("golangci-lint on generated code: %v\n%s", err, lintOut)
	}
}

// writeModule makes dir a Go module the generated programs build in.
func writeModule(t *testing.T, dir string) {
	t.Helper()
	err := os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = WriteModule(dir)
	if err != nil {
		t.Fatal(err)
	}
}

// TestRun holds each testdata/run/*.rb to the same MRI comparison as the
// examples, without the rbs and lint gates: one go build per file.
func TestRun(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	files := mustGlob(t, "testdata/run/*.rb")
	for _, rb := range files {
		t.Run(strings.TrimSuffix(filepath.Base(rb), ".rb"), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(rb) //nolint:gosec // testdata path
			if err != nil {
				t.Fatal(err)
			}
			skipIfMarked(t, src)
			gen := transpile(t, rb, src) // Compile names it by its base; the path is for require_relative
			// -l skips inlining, ~40% of compile CPU; TestExamples keeps the default build users get.
			sameAsRuby(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-l"))
		})
	}
}

// TestMinitest runs each testdata/test/*_test.rb with a fixed seed. MRI must
// pass it, or the test itself is wrong; rugaroo's output must then match MRI's
// apart from timings. One build per file for now: the files load together
// as one program (decision 84, `rugaroo test testdata/test`), but that program
// takes Go ~30 minutes to build: Dyn wrappers grow as classes × names
// (decision 81).
func TestMinitest(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	files := mustGlob(t, "testdata/test/*_test.rb")
	if os.Getenv("RUGAROO_CGO") != "" { // what only the cgo Fiddle backend can do: cgo/<area> (decision 426)
		files = append(files, mustGlob(t, "testdata/cgo/*_test.rb")...)
	}
	for _, rb := range files {
		name := strings.TrimSuffix(filepath.Base(rb), "_test.rb")
		if filepath.Base(filepath.Dir(rb)) == "cgo" {
			name = "cgo_" + name
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(rb) //nolint:gosec // testdata path
			if err != nil {
				t.Fatal(err)
			}
			skipIfMarked(t, src)
			gen := transpile(t, rb, src) // Compile names it by its base; the path is for require_relative
			extra := []string{"--seed", "1"}
			if _, ex := laneSkips(t); len(ex) > 0 {
				extra = append(extra, "-e", "/^("+strings.Join(ex, "|")+")$/")
			}
			sameAsRubyWith(t, filepath.Dir(rb), filepath.Base(rb), goBuild(t, gen, "-l"), extra)
		})
	}
}

// TestMulti compiles each testdata/multi/<case>/*.rb, sorted, as one program
// (decision 84) and compares it with MRI loading the same files in one
// process, as rake's test loader does.
func TestMulti(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dirs := mustGlob(t, "testdata/multi/*")
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			paths := mustGlob(t, filepath.Join(dir, "*.rb"))
			files := make([]File, 0, len(paths))
			for _, p := range paths {
				src, rerr := os.ReadFile(p) //nolint:gosec // testdata path
				if rerr != nil {
					t.Fatal(rerr)
				}
				files = append(files, File{Name: p, Src: src})
			}
			code, warnings, err := CompileFiles(t.Context(), files)
			if err != nil {
				t.Fatalf("rugaroo: %v", err)
			}
			for _, w := range warnings {
				t.Logf("warning: %s", w)
			}
			gen := t.TempDir()
			writeModule(t, gen)
			err = os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			bin := goBuild(t, gen, "-l")
			loader := append([]string{"-e", "ARGV.each { |f| require File.expand_path(f) }"}, paths...)
			wantOut, wantErr, wantCode, rerr := runIO(t, ".", progIO{}, "ruby", loader...)
			if rerr != nil || wantCode != 0 { // MRI must pass, or two identical crashes would agree
				t.Fatalf("ruby failed (exit %d, err %v): the case is wrong\n%s\n%s", wantCode, rerr, wantOut, wantErr)
			}
			gotOut, _, gotCode, _ := runIO(t, ".", progIO{}, bin)
			if wantOut != gotOut || wantCode != gotCode {
				t.Errorf("ruby (exit %d):\n%s\n--- go (exit %d):\n%s", wantCode, wantOut, gotCode, gotOut)
			}
		})
	}
}

// TestGemAutoloadWithoutBoot compiles an in-memory suite (cmd/rubyspec's, decision 409) requiring a gem whose class
// arrives through autoload: with no boot to say which autoloads run, each loads where it registers (decision 414).
func TestGemAutoloadWithoutBoot(t *testing.T) {
	requireToolchain(t)
	src := []byte("require \"matrix\"\np Matrix[[1, 2], [3, 4]].lup.solve(Vector[1, 2])\n")
	code, _, _, err := CompileTestsSkippingWithGems(t.Context(), []File{{Name: "lup_test.rb", Src: src}}, nil, ".")
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	if !bytes.Contains(code, []byte("type Matrix_LUPDecomposition struct")) {
		t.Error("Matrix::LUPDecomposition, matrix's autoload, was not compiled")
	}
}

// TestGemDefineMethodWithoutBoot: with no boot, a gem class body's `[:send, ...].each { |m| define_method(m) { } }` still
// defines each method (net-ftp's BufferedSocket#send, decision 416).
func TestGemDefineMethodWithoutBoot(t *testing.T) {
	requireToolchain(t)
	src := []byte("require \"net/ftp\"\nsock = Net::FTP::BufferedSocket.new(STDOUT)\np sock.respond_to?(:shutdown)\n")
	code, _, _, err := CompileTestsSkippingWithGems(t.Context(), []File{{Name: "dm_test.rb", Src: src}}, nil, ".")
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	if !bytes.Contains(code, []byte("func Net_FTP_BufferedSocket_Shutdown(")) {
		t.Error("Net::FTP::BufferedSocket#shutdown, a literal define_method, was not compiled")
	}
}

// TestPrimeVendoredSig: prime's shipped sig types each `-> void`, but its value is `break last_value` (7, or nil with
// no prime) and a generator without a block; sig/gems/prime corrects it (decision 419), so the value MRI gives comes out.
func TestPrimeVendoredSig(t *testing.T) {
	requireToolchain(t)
	src := []byte("require \"prime\"\np Prime.each(10) { |x| x }, Prime.each(1) { :v }, Prime.instance.each(7).to_a\n")
	code, _, _, err := CompileTestsSkippingWithGems(t.Context(), []File{{Name: "prime_sig_test.rb", Src: src}}, nil, ".")
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	gen := t.TempDir()
	writeModule(t, gen)
	err = os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	out, _, exit, _ := runIO(t, ".", progIO{}, goBuild(t, gen, "-l"))
	if want := "7\nnil\n[2, 3, 5, 7]\n"; out != want || exit != 0 {
		t.Errorf("got (exit %d):\n%s\nwant:\n%s", exit, out, want)
	}
}

// TestBootAutoload compiles a program whose class arrives through autoload:
// the boot snapshot records which file MRI loaded, and the compiler takes it
// as a source (decisions 131/161). The declaration itself is compile-time.
func TestBootAutoload(t *testing.T) {
	requireToolchain(t)
	dir := filepath.Join("testdata", "boot")
	app := filepath.Join(dir, "app.rb")
	src, err := os.ReadFile(app) //nolint:gosec // testdata path
	if err != nil {
		t.Fatal(err)
	}
	lp := loadPath(dir, src) // # load_path: lib
	srcs := []compiler.Source{{Name: "app.rb", Src: src, Path: app}}
	t.Setenv("RUGAROO_GEM_WARNINGS", "1")
	code, warnings, err := compiler.CompileWithBoot(t.Context(), Prelude, GemSigs, srcs, lp)
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}
	// sigdemo's own sig/ types Meter (push through its `alias push add`); its malformed file only warns (decision 306)
	if !slices.ContainsFunc(warnings, func(w string) bool { return strings.Contains(w, "broken.rbs: skipped") }) {
		t.Error("no warning for sigdemo/sig/broken.rbs")
	}
	for _, want := range []string{"Sigdemo_Meter_Total(self *Sigdemo_Meter) Integer", "Sigdemo_Meter_Push[Self Sigdemo_MeterI](self Self, value Integer) Self",
		// `Meter & _Reading`, a 'single-quoted' record, a backquoted and a proc-param name (decision 315)
		"(self Self, other *Sigdemo_Meter) Self", "src *Hash[String, String]) String", "sink *func(Integer))"} {
		if !bytes.Contains(code, []byte(want)) {
			t.Errorf("main.go lacks %q: the gem's sig/ did not type Sigdemo::Meter", want)
		}
	}
	gen := t.TempDir()
	writeModule(t, gen)
	werr := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if werr != nil {
		t.Fatal(werr)
	}
	if keep := os.Getenv("RUGAROO_BOOT_KEEP"); keep != "" { // the generated main.go, for a failing build
		_ = os.WriteFile(keep, code, 0o600) //nolint:gosec // a developer-chosen path
	}
	sameAsRuby(t, dir, "app.rb", goBuild(t, gen, ""))

	// The same lib as the program's own -I code is not a gem's: compiled whole and strictly, so its never-called methods fail (decision 173).
	_, _, err = compiler.CompileWithBoot(t.Context(), Prelude, GemSigs, srcs, nil, lp...)
	if err == nil || !strings.Contains(err.Error(), filepath.Join("lib", "demo")) {
		t.Errorf("user -I code compiled as a gem's: err = %v", err)
	}
}

// TestTracedEntry compiles a decision 197 entry (an MRI load list) with main.rb as -main (decision 288), as Homebrew's harness does:
// its -I dirs are gem dirs, and a require inside a method resolves at run time (decision 246).
func TestTracedEntry(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dir := filepath.Join("testdata", "traced")
	entry := filepath.Join(dir, "entry.rb")
	src, err := os.ReadFile(entry) //nolint:gosec // testdata path
	if err != nil {
		t.Fatal(err)
	}
	main, rerr := os.ReadFile(filepath.Join(dir, "main.rb")) //nolint:gosec // testdata path
	if rerr != nil {
		t.Fatal(rerr)
	}
	srcs := []compiler.Source{{Name: "entry.rb", Src: src, Path: entry}}
	code, _, err := compiler.CompileTraced(t.Context(), Prelude, GemSigs, srcs, filepath.Join(dir, "main.rb"), loadPath(dir, main)...)
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	gen := t.TempDir()
	writeModule(t, gen)
	werr := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if werr != nil {
		t.Fatal(werr)
	}
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen, ""))
}

// TestTraceEval compiles testdata/traced_eval from a run of main.rb: the string its loader instance_evals at run time (a
// cask file's text) runs compiled at the call site (decision 374).
func TestTraceEval(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dir := filepath.Join("testdata", "traced_eval")
	main := filepath.Join(dir, "main.rb")
	src, err := os.ReadFile(main) //nolint:gosec // testdata path
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := CompileTrace(t.Context(), main, [][]string{nil}, []string{"--disable=gems"}, loadPath(dir, src)...)
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	gen := t.TempDir()
	writeModule(t, gen)
	werr := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if werr != nil {
		t.Fatal(werr)
	}
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen, ""))
}

// TestTraceDeterministic: a temp dir named in the output makes every regen miss the Go build cache (decision 88).
func TestTraceDeterministic(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dir := filepath.Join("testdata", "traced_det")
	main := filepath.Join(dir, "main.rb")
	src, err := os.ReadFile(main) //nolint:gosec // testdata path
	if err != nil {
		t.Fatal(err)
	}
	var outs [2][]byte
	for i := range outs {
		outs[i], _, err = CompileTrace(t.Context(), main, [][]string{nil}, []string{"--disable=gems"}, loadPath(dir, src)...)
		if err != nil {
			t.Fatalf("rugaroo: %v", err)
		}
	}
	if !bytes.Equal(outs[0], outs[1]) {
		t.Error("two traced compiles of one program differ")
	}
	for _, tmp := range []string{"rugaroo-boot-", "rugaroo-trace-", "rugaroo-entry-"} {
		if bytes.Contains(outs[0], []byte(tmp)) {
			t.Errorf("output names a %s temp dir", tmp)
		}
	}
}

// TestTraceRun compiles testdata/traced from runs of main.rb, with no entry file: the second (-trace-also) loads traced_argv (decision 296).
func TestTraceRun(t *testing.T) {
	requireToolchain(t)
	dir := filepath.Join("testdata", "traced")
	main := filepath.Join(dir, "main.rb")
	src, err := os.ReadFile(main) //nolint:gosec // testdata path
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRACED_SPAWN", "1") // the run sees main.rb's `# env:`, as the program does
	code, _, err := CompileTrace(t.Context(), main, [][]string{nil, {"argv"}}, []string{"--disable=gems"}, loadPath(dir, src)...)
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	gen := t.TempDir()
	writeModule(t, gen)
	werr := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if werr != nil {
		t.Fatal(werr)
	}
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen, ""))
}

var directive = regexp.MustCompile(`(?m)^# (error|warning|skip|args|env|stdin|stderr|load_path|trace|gem): (.*)$`)

// directives returns the text of each `# kind: text` line in src.
func directives(src []byte, kind string) []string {
	var out []string
	for _, m := range directive.FindAllSubmatch(src, -1) {
		if string(m[1]) == kind {
			out = append(out, strings.TrimSpace(string(m[2])))
		}
	}
	return out
}

// skipIfMarked skips a case carrying `# skip: reason` (a known failure),
// unless RUGAROO_RUN_SKIPPED is set.
func skipIfMarked(t *testing.T, src []byte) {
	t.Helper()
	reasons := directives(src, "skip")
	if len(reasons) > 0 && os.Getenv("RUGAROO_RUN_SKIPPED") == "" {
		t.Skip("known failure: " + strings.Join(reasons, "; "))
	}
	if skip, _ := laneSkips(t); skip {
		t.Skip("pins another lane's answer (testdata/int-overflow, decision 372; testdata/cgo, decision 426)")
	}
}

// laneSkips reads testdata/int-overflow/<mode>-skip.txt for RUGAROO_INT_OVERFLOW's lane (decision 372): a line naming
// the test skips it; one naming the test and a minitest method excludes that method on both sides.
func laneSkips(t *testing.T) (skip bool, methods []string) {
	t.Helper()
	backend := "purego"
	if os.Getenv("RUGAROO_CGO") != "" {
		backend = "cgo"
	}
	var data []byte
	for _, f := range []string{ // a Fiddle backend's lane is testdata/cgo/<backend>-skip.txt (decision 426)
		filepath.Join("testdata", "int-overflow", cmp.Or(os.Getenv("RUGAROO_INT_OVERFLOW"), "raise")+"-skip.txt"),
		filepath.Join("testdata", "cgo", backend+"-skip.txt"),
	} {
		b, err := os.ReadFile(f) //nolint:gosec // testdata path
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		data = append(data, b...)
	}
	for line := range strings.Lines(string(data)) {
		line, _, _ = strings.Cut(line, "#")
		switch f := strings.Fields(line); {
		case len(f) == 1 && f[0] == t.Name():
			skip = true
		case len(f) == 2 && f[0] == t.Name():
			methods = append(methods, f[1])
		}
	}
	return skip, methods
}

// TestTraceOracle checks decision 105's point: an assertion that passes on
// both sides but sees a different operand (here a process id) is reported.
func TestTraceOracle(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	dir := t.TempDir()
	src := []byte(`require "minitest/autorun"

class PidTest < Minitest::Test
  def test_pid = refute_equal(0, Process.pid)
end
`)
	err := os.WriteFile(filepath.Join(dir, "pid_test.rb"), src, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	bin := goBuild(t, transpile(t, "pid_test.rb", src), "-l")
	pio := progIO{trace: true}
	want := rubyOutput(t, dir, "pid_test.rb", src, pio, []string{"--seed", "1"})
	got := binOutput(t, dir, pio, bin, []string{"--seed", "1"})
	if want.Code != 0 || got.Code != 0 {
		t.Fatalf("both runs should pass: ruby %d, go %d\n%s%s", want.Code, got.Code, want.Stdout, got.Stdout)
	}
	d := traceDiff(want.Trace, got.Trace)
	if !strings.Contains(d, "PidTest#test_pid refute_equal 0 ") {
		t.Fatalf("the oracle should report the operand difference, got:\n%q\n--- traces ---\n%s---\n%s", d, want.Trace, got.Trace)
	}
	if traceDiff(want.Trace, want.Trace) != "" {
		t.Error("identical traces should not differ")
	}
}

// TestTypedAssertionsStayStatic checks decision 93's point: typed
// assertions with literal operators keep no run-time lookup by method name.
func TestTypedAssertionsStayStatic(t *testing.T) {
	t.Parallel()
	src := []byte(`require "minitest/autorun"

class T < Minitest::Test
  def test_x
    h = { a: 3 } #: Hash[Symbol, Integer]
    assert_equal 3, h[:a]
    assert_includes [1, 2], 2
    refute_empty "x"
    assert_operator 1, :<, 2
    assert_predicate 2, :even?
    assert_respond_to "x", :upcase
  end
end
`)
	out, _, err := compileSafe("main.rb", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"func rbRespondsByName", "func rbDyn"} {
		if bytes.Contains(out, []byte(name)) {
			t.Errorf("generated code has %s", name)
		}
	}
}

// TestErrors compiles each file of testdata/errors/*.txtar as main.rb.
// Every `# error: text` must appear in the compile error (so compilation
// must fail), and every `# warning: text` in some warning; with no expected
// error, compilation must succeed.
// TestPreludeGo regenerates prelude/go's type-checking stubs (decision 154) and vets the helpers against them, so a
// broken helper fails here instead of through some example's go build.
func TestPreludeGo(t *testing.T) {
	t.Parallel()
	src, err := preludeWhole()
	if err != nil {
		t.Fatal(err)
	}
	out, err := compiler.StubsFrom(src, Prelude)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(StubsPath, out, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	for _, tags := range []string{"rugaroo_prelude", "rugaroo_prelude,rugaroo_cgo"} { // each Fiddle backend (decision 426)
		cmd := exec.CommandContext(t.Context(), "go", "vet", "-tags", tags, "./prelude/go") //nolint:gosec // the two tag sets above
		got, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go vet -tags %s ./prelude/go: %v\n%s", tags, err, got)
		}
	}
}

// preludeWhole is the unpruned every-library program, compiled once for TestPreludeGo and TestPreludeWhole.
var preludeWhole = sync.OnceValues(func() ([]byte, error) {
	return compiler.PreludeWhole(context.Background(), Prelude)
})

// exportData finds the compiled export data of f's imports through the repo's module, so runtime modules (purego) resolve too.
func exportData(t *testing.T, f *ast.File) func(path string) (io.ReadCloser, error) {
	t.Helper()
	args := append(make([]string, 0, 5+len(f.Imports)), "list", "-export", "-deps", "-f", "{{.ImportPath}} {{.Export}}")
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		args = append(args, path)
	}
	out, err := exec.CommandContext(t.Context(), "go", args...).Output() //nolint:gosec // import paths of the prelude's own Go
	if err != nil {
		t.Fatalf("go list -export: %v", err)
	}
	files := map[string]string{}
	for line := range strings.Lines(string(out)) {
		if path, file, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			files[path] = file
		}
	}
	return func(path string) (io.ReadCloser, error) {
		file, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file) //nolint:gosec // a path go list printed
	}
}

// TestPreludeWhole type-checks the unpruned prelude (decision 207): pruning must not be what keeps a generic
// instantiation cycle or an ill-typed method out of programs. A type check, as building it takes 6.8 GB.
func TestPreludeWhole(t *testing.T) {
	t.Parallel()
	src, err := preludeWhole()
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var errs []string
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", exportData(t, f)), Error: func(err error) { errs = append(errs, err.Error()) }}
	_, _ = conf.Check("main", fset, []*ast.File{f}, nil)
	if len(errs) > 0 {
		t.Errorf("%d type errors in the unpruned prelude, first ones:\n%s", len(errs), strings.Join(errs[:min(20, len(errs))], "\n"))
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	archives := mustGlob(t, "testdata/errors/*.txtar")
	for _, path := range archives {
		ar, err := txtar.ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		area := strings.TrimSuffix(filepath.Base(path), ".txtar")
		for _, f := range ar.Files {
			t.Run(area+"/"+strings.TrimSuffix(f.Name, ".rb"), func(t *testing.T) {
				t.Parallel()
				skipIfMarked(t, f.Data)
				wantErrs, wantWarns := directives(f.Data, "error"), directives(f.Data, "warning")
				if len(wantErrs)+len(wantWarns) == 0 {
					t.Fatal("case declares no `# error:` or `# warning:`")
				}
				_, warnings, err := compileSafe("main.rb", f.Data)
				switch {
				case len(wantErrs) > 0 && err == nil:
					t.Errorf("compiled; want error %q", wantErrs)
				case len(wantErrs) == 0 && err != nil:
					t.Errorf("unexpected error: %v", err)
				}
				for _, w := range wantErrs {
					if err != nil && !strings.Contains(err.Error(), w) {
						t.Errorf("error %q does not contain %q", err, w)
					}
				}
				for _, w := range wantWarns {
					if !slices.ContainsFunc(warnings, func(s string) bool { return strings.Contains(s, w) }) {
						t.Errorf("no warning contains %q; got %q", w, warnings)
					}
				}
			})
		}
	}
}

var rdocNoise = regexp.MustCompile(`(?m)^.*rdoc.*warning.*\n`)

func testExample(t *testing.T, dir, gen string) {
	tmp := t.TempDir()
	// 1. rbs-inline + rbs validate
	sig := filepath.Join(tmp, "sig")
	start := time.Now()
	out, err := gemCmd(t, "rbs-inline", "--output="+sig, dir).CombinedOutput() // the whole directory: main.rb and what it require_relatives
	timed("rbs-inline", t.Name(), time.Since(start))
	if err != nil {
		t.Fatalf("rbs-inline failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	args := append(rbsLibraries(t, filepath.Join(dir, "main.rb")), "-I", sig, "validate")
	start = time.Now()
	out, err = gemCmd(t, "rbs", args...).CombinedOutput()
	timed("rbs-validate", t.Name(), time.Since(start))
	if err != nil {
		t.Fatalf("rbs validate failed: %v\n%s", err, rdocNoise.ReplaceAll(out, nil))
	}
	// 2. transpile and build (vet and lint run once over all examples: lintGenerated)
	src, err := os.ReadFile(filepath.Join(dir, "main.rb")) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	compile := gemCompile(dir, src)
	if len(directives(src, "trace")) > 0 { // `# trace: why`: compiled from what an MRI run loads, as `rugaroo build -trace` (decision 296)
		compile = func(ctx context.Context, name string, _ []byte, lp ...string) ([]byte, []string, error) {
			return CompileTrace(ctx, name, [][]string{nil}, nil, lp...) // ponytail: one run with no args; pass `# args:` here when a traced example takes some
		}
	}
	writeGenerated(t, gen, filepath.Join(dir, "main.rb"), src, compile) // its real path, for require_relative
	fmtOut, err := run(t, gen, "gofmt", "-l", "main.go")
	if err != nil || strings.TrimSpace(fmtOut) != "" {
		t.Fatalf("gofmt: %v %s", err, fmtOut)
	}
	// 3. compare with MRI; examples keep inlining, the default build users get
	sameAsRuby(t, dir, "main.rb", goBuild(t, gen, ""))
}

// compileSafe is Compile with an internal compiler panic turned into an
// error, so one bad case fails its test instead of the whole binary.
func compileSafe(name string, src []byte) (code []byte, warnings []string, err error) {
	return compileSafeWith(name, src, Compile)
}

func compileSafeWith(name string, src []byte, compile func(context.Context, string, []byte, ...string) ([]byte, []string, error)) (code []byte, warnings []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compiler panic: %v\n%s", r, debug.Stack())
		}
	}()
	return compile(context.Background(), name, src, loadPath(filepath.Dir(name), src)...)
}

// onLoadPath reports whether `require name` finds a file on src's `# load_path:`.
func onLoadPath(dir string, src []byte, name string) bool {
	for _, d := range loadPath(dir, src) {
		_, err := os.Stat(filepath.Join(d, name+".rb"))
		if err == nil {
			return true
		}
	}
	return false
}

// loadPath is src's `# load_path: dir` directories, relative to dir where the file is: `ruby -I` for MRI, -I for rugaroo.
func loadPath(dir string, src []byte) []string {
	dirs := directives(src, "load_path")
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, filepath.Join(dir, d))
	}
	return out
}

// transpile compiles src into a fresh Go module and returns its directory.
func transpile(t *testing.T, name string, src []byte) string {
	t.Helper()
	gen := t.TempDir()
	writeModule(t, gen)
	writeGenerated(t, gen, name, src, gemCompile(filepath.Dir(name), src)) // a test file requiring rackup compiles the gem (decision 176)
	stablePreludeLines(t, filepath.Join(gen, "main.go"))
	return gen
}

var preludeLine = regexp.MustCompile(`(//line prelude/[^:\s]+):\d+`)

// stablePreludeLines sets every prelude //line to line 1, so the Go build cache keys on what the program reaches, not where in the prelude it sits (decision 88); the examples keep real lines for lint.
func stablePreludeLines(t *testing.T, path string) {
	t.Helper()
	code, err := os.ReadFile(path) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, preludeLine.ReplaceAll(code, []byte("${1}:1")), 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
}

// gemCompile is CompileWithGems, which finds gems as the CLI does (decision 173), with each `# gem: dir` (relative to dir)
// compiled as a gem the program carries, as `rugaroo build -gem dir` (decision 406).
func gemCompile(dir string, src []byte) func(context.Context, string, []byte, ...string) ([]byte, []string, error) {
	gems := directives(src, "gem")
	if len(gems) == 0 {
		return CompileWithGems
	}
	gemDirs := make([]string, len(gems))
	for i, g := range gems {
		gemDirs[i] = filepath.Join(dir, g)
	}
	return func(ctx context.Context, name string, s []byte, lp ...string) ([]byte, []string, error) {
		return CompileWithGemDirs(ctx, name, s, gemDirs, lp...)
	}
}

// writeGenerated compiles src into dir/main.go.
func writeGenerated(t *testing.T, dir, name string, src []byte, compile func(context.Context, string, []byte, ...string) ([]byte, []string, error)) {
	t.Helper()
	start := time.Now()
	code, warnings, err := compileSafeWith(name, src, compile)
	timed("compile", t.Name(), time.Since(start))
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	for _, w := range warnings {
		t.Logf("warning: %s", w)
		if strings.HasPrefix(w, "prelude/") {
			t.Errorf("a prelude warning reaches users (mark the method # @dynamic if intended): %s", w)
		}
	}
	err = os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "main.go"), code, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("generated Go: %s", filepath.Join(dir, "main.go"))
}

// copyPreludeGo mirrors prelude/go/*.go into gen at the same relative path so golangci-lint can open the //line targets it remaps positions to (nolint matching needs the real file, not just the label).
func copyPreludeGo(t *testing.T, gen string) {
	t.Helper()
	files, err := filepath.Glob("prelude/go/*.go")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(gen, "prelude", "go")
	err = os.MkdirAll(dir, 0o755) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, rerr := os.ReadFile(f) //nolint:gosec // repo path
		if rerr != nil {
			t.Fatal(rerr)
		}
		err = os.WriteFile(filepath.Join(dir, filepath.Base(f)), data, 0o600) //nolint:gosec // under t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// goBuild builds gen's program; gcflags is extra compiler flags ("-l" skips inlining).
func goBuild(t *testing.T, gen string, gcflags string) string {
	t.Helper()
	bin := filepath.Join(gen, "prog")
	// -dwarf=false: debug info is a quarter of each cache entry and no test reads it (decision 88); a repeated -gcflags replaces the earlier one, so it is composed here
	gcflags = strings.TrimSpace(gcflags + " -dwarf=false")
	// -trimpath: the build cache hits across temp dirs.
	args := []string{"build", "-trimpath", "-gcflags=" + gcflags, "-o", bin, "."}
	code, err := os.ReadFile(filepath.Join(gen, "main.go")) //nolint:gosec // under t.TempDir()
	if err != nil || len(goStmt.FindAllIndex(code, 2)) > 1 {
		// -race only where a program starts goroutines beyond the always-on signal loop: it turns off the compiler's parallel backend, doubling compile time (decision 88)
		args = slices.Insert(args, 1, "-race")
	}
	if timingLog != "" {
		args = slices.Insert(args, 1, "-x") // the compile step shows up in -x only on a build-cache miss
	}
	start := time.Now()
	out, stderr, bcode, err := runIO(t, gen, progIO{}, "go", args...)
	if err == nil && bcode != 0 {
		err = errors.New(stderr)
	}
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	if timingLog != "" {
		miss := strings.Contains(stderr, " -p main ") // the main package's compile step runs only on a build-cache miss (a rebuilt std shows other -p values)
		funcs := bytes.Count(code, []byte("\nfunc "))
		lines := bytes.Count(code, []byte("\n"))
		sum := sha256.Sum256(code)
		timed("build", t.Name(), time.Since(start), slices.Contains(args, "-race"), miss, funcs, lines, len(code), hex.EncodeToString(sum[:8]))
		if keep := os.Getenv("RUGAROO_TEST_KEEP"); keep != "" { // each program's main.go, named by its test, for diffing two runs
			_ = os.WriteFile(filepath.Join(keep, strings.ReplaceAll(t.Name(), "/", "__")+".go"), code, 0o600) //nolint:gosec // a developer-chosen path
		}
	}
	return bin
}

// goStmt matches a gofmt-ed go statement.
var goStmt = regexp.MustCompile(`(?m)^\t+go \S`)

// mtTiming matches minitest's run timings: the `Finished in` line and -v's per-test seconds.
var mtTiming = regexp.MustCompile(`(?m)^Finished in .*$| = \d+\.\d\d s = `)

// mtTimings blanks minitest's timings, the only output that differs between two runs with one seed.
func mtTimings(s string) string { return mtTiming.ReplaceAllString(s, "<timing>") }

// sameAsRuby runs `ruby file` and bin in dir; stdout and exit code must match, and stderr too under `# stderr: match`.
func sameAsRuby(t *testing.T, dir, file, bin string) {
	t.Helper()
	sameAsRubyWith(t, dir, file, bin, nil)
}

// sameAsRubyWith is sameAsRuby with extra arguments after the file's `# args:`.
func sameAsRubyWith(t *testing.T, dir, file, bin string, extra []string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(dir, file)) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	pio := progIO{env: directives(src, "env")}
	args := []string{} //nolint:prealloc // field count unknown until split
	for _, a := range directives(src, "args") {
		args = append(args, strings.Fields(a)...)
	}
	args = append(args, extra...)
	for _, q := range directives(src, "stdin") {
		in, qerr := strconv.Unquote(q)
		if qerr != nil {
			t.Fatalf("# stdin: %s: %v", q, qerr)
		}
		pio.stdin += in
	}
	mt := bytes.Contains(src, []byte(`require "minitest`))
	pio.trace = mt
	want := rubyOutput(t, dir, file, src, pio, args)
	if extra != nil && want.Code != 0 {
		t.Fatalf("MRI fails %s, so the test itself is wrong:\n%s%s", file, want.Stdout, want.Stderr)
	}
	got := binOutput(t, dir, pio, bin, args)
	if mt {
		want.Stdout, got.Stdout = mtTimings(want.Stdout), mtTimings(got.Stdout)
	}
	if want.Stdout != got.Stdout {
		t.Errorf("stdout differs\n--- ruby ---\n%s\n--- go ---\n%s", want.Stdout, got.Stdout)
	}
	if want.Code != got.Code {
		t.Errorf("exit code: ruby %d, go %d; go stderr:\n%s", want.Code, got.Code, got.Stderr)
	}
	if slices.Contains(directives(src, "stderr"), "match") && want.Stderr != got.Stderr {
		t.Errorf("stderr differs\n--- ruby ---\n%s\n--- go ---\n%s", want.Stderr, got.Stderr)
	}
	if d := traceDiff(want.Trace, got.Trace); d != "" {
		t.Errorf("assertion operands differ from MRI's (trace oracle, decision 105):\n%s", d)
	}
}

// binOutput runs the compiled program as rubyOutput runs MRI, collecting its trace when pio.trace is set.
func binOutput(t *testing.T, dir string, pio progIO, bin string, args []string) mriResult {
	t.Helper()
	var got mriResult
	var tracePath string
	if pio.trace {
		tracePath = filepath.Join(t.TempDir(), "trace")
		pio.env = append(slices.Clone(pio.env), "RUGAROO_MT_TRACE="+tracePath)
	}
	start := time.Now()
	got.Stdout, got.Stderr, got.Code, _ = runIO(t, dir, pio, bin, args...)
	timed("run", t.Name(), time.Since(start))
	if pio.trace {
		got.Trace = readTrace(t, tracePath)
	}
	return got
}

// readTrace is the trace file's content, empty when the program wrote none.
func readTrace(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // our temp path
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(data)
}

// traceAddr masks object addresses and ids, as minitest's own diff does (rbMtPPForDiff), and Ractor numbers: none agree across runs.
var traceAddr = regexp.MustCompile(`0x[0-9a-f]+|oid=\d+|Ractor:#\d+|id:\d+`)

// traceDiff describes the first line where two assertion traces differ, or is empty when they agree.
func traceDiff(want, got string) string {
	if want == got {
		return ""
	}
	w := strings.Split(strings.TrimSuffix(traceAddr.ReplaceAllString(want, "#"), "\n"), "\n")
	g := strings.Split(strings.TrimSuffix(traceAddr.ReplaceAllString(got, "#"), "\n"), "\n")
	for i := range max(len(w), len(g)) {
		wl, gl := "<end>", "<end>"
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d of %d (ruby) / %d (go):\n--- ruby ---\n%s\n--- go ---\n%s", i+1, len(w), len(g), wl, gl)
		}
	}
	return ""
}

// rubyDescription keys the MRI output cache: another Ruby may print differently.
var rubyDescription = sync.OnceValue(func() string {
	out, _ := exec.Command("ruby", "-e", "print RUBY_DESCRIPTION").Output()
	return string(out)
})

type mriResult struct {
	Stdout, Stderr string
	Trace          string // the assertion trace (decision 105), when asked for
	Code           int
}

// mriCached holds output as []byte (base64 in JSON): a string field would turn non-UTF-8 bytes into U+FFFD.
type mriCached struct {
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
	Trace  []byte `json:"trace,omitempty"`
	Code   int    `json:"code"`
}

// gemfileLock keys the cache on the gem versions (minitest, rack) MRI loads: a bundle update must not replay old output.
var gemfileLock = sync.OnceValue(func() string {
	data, _ := os.ReadFile("Gemfile.lock") // absent just keys as empty
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
})

// mriCacheable is whether a run's output is MRI's own: not a signalled run (-1), nor one that could not load a library, which is the machine's state (decision 88's cache replays what it stores forever).
func mriCacheable(r mriResult) bool {
	return r.Code >= 0 && !strings.Contains(r.Stderr, "LoadError") && !strings.Contains(r.Stderr, "cannot load such file")
}

// writeCacheFile stores data at path atomically: a concurrent reader (parallel subtests, another `go test`) sees the old file or the whole new one, never a torn write.
func writeCacheFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o755) //nolint:gosec // cache dir
	if err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("cache temp: %w", err)
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path) //nolint:gosec // our CreateTemp file, our cache path
	}
	if err != nil {
		_ = os.Remove(f.Name()) //nolint:gosec // our CreateTemp file
		return fmt.Errorf("cache write: %w", err)
	}
	return nil
}

// mtTracePreload is the MRI half of the trace oracle, read once: its source keys the cache too.
var mtTracePreload = sync.OnceValues(func() (string, []byte) {
	path, err := filepath.Abs("testdata/mt_trace.rb")
	if err != nil {
		panic(err)
	}
	src, err := os.ReadFile(path) //nolint:gosec // testdata path
	if err != nil {
		panic(err)
	}
	return path, src
})

// rubyOutput is `ruby file`'s result, cached by source (which holds the args/env/stdin directives), Ruby, TZ, Gemfile.lock and checkout (RUGAROO_NO_MRI_CACHE=1 skips the cache).
func rubyOutput(t *testing.T, dir, file string, src []byte, pio progIO, args []string) mriResult {
	t.Helper()
	dirs := directives(src, "load_path")
	flags := make([]string, 0, 2*len(dirs))
	for _, d := range dirs { // ruby runs in dir, so as written
		flags = append(flags, "-I", d)
	}
	if file == "main.rb" || len(dirs) > 0 { // an example, or a file with a load path: key the cache on the files it requires too
		src = exampleSources(t, dir)
	}
	return mriRun(t, dir, src, pio, slices.Concat(flags, []string{file}, args))
}

// exampleSources is every file under an example's directory with its path: main.rb and what it requires, and the data it reads (a .csv, a .rbs), so editing either misses the MRI cache.
func exampleSources(t *testing.T, dir string) []byte {
	t.Helper()
	var out []byte
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		src, err := os.ReadFile(p) //nolint:gosec // example path
		out = append(append(append(out, p...), 0), src...)
		return err //nolint:wrapcheck // the *PathError names the file
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// mriRun runs `ruby rubyArgs...` in dir, cached by src (every source it reads), Ruby's version, TZ, the checkout and the arguments past the first.
func mriRun(t *testing.T, dir string, src []byte, pio progIO, rubyArgs []string) mriResult {
	t.Helper()
	wd, _ := os.Getwd() // the checkout: output holding __dir__ or a testdata path differs per worktree
	key := "v6\x00" +   // v6: exampleSources hashes non-.rb files too (decision 272)
		rubyDescription() + "\x00" + os.Getenv("TZ") + "\x00" + wd + "\x00" + gemfileLock() + "\x00"
	if args := rubyArgs[1:]; len(args) > 0 { // `# args:` are in src; TestMinitest adds more
		key += strings.Join(args, "\x00") + "\x00args\x00"
	}
	var tracePath string
	if pio.trace {
		preload, preloadSrc := mtTracePreload()
		key += "trace\x00" + string(preloadSrc) + "\x00"
		tracePath = filepath.Join(t.TempDir(), "trace")
		pio.env = append(slices.Clone(pio.env), "RUGAROO_MT_TRACE="+tracePath)
		rubyArgs = append([]string{"-r", preload}, rubyArgs...)
	}
	sum := sha256.Sum256(slices.Concat([]byte(key), src))
	cacheDir, err := os.UserCacheDir()
	cached := filepath.Join(cacheDir, "rugaroo-test", "mri", hex.EncodeToString(sum[:]))
	if err == nil && os.Getenv("RUGAROO_NO_MRI_CACHE") == "" {
		data, rerr := os.ReadFile(cached) //nolint:gosec // our cache path
		var c mriCached
		if rerr == nil && json.Unmarshal(data, &c) == nil {
			timed("mri", t.Name(), 0, "hit")
			return mriResult{Stdout: string(c.Stdout), Stderr: string(c.Stderr), Trace: string(c.Trace), Code: c.Code}
		}
	}
	var r mriResult
	start := time.Now()
	r.Stdout, r.Stderr, r.Code, _ = runIO(t, dir, pio, "ruby", rubyArgs...)
	timed("mri", t.Name(), time.Since(start), "miss")
	if pio.trace {
		r.Trace = readTrace(t, tracePath)
	}
	if err == nil && mriCacheable(r) {
		data, err := json.Marshal(mriCached{Stdout: []byte(r.Stdout), Stderr: []byte(r.Stderr), Trace: []byte(r.Trace), Code: r.Code})
		if err != nil {
			t.Fatal(err)
		}
		_ = writeCacheFile(cached, data) // a cache that cannot be written only costs the next run a rerun
	}
	return r
}

var requireLine = regexp.MustCompile(`(?m)^require "([^"]+)"`)

// rbsLibraryNames covers requires whose gem name differs from its RBS stdlib signature directory, like `require "observer"` (defines Observable) shipping sigs under "observable".
var rbsLibraryNames = map[string]string{"observer": "observable", "minitest-autorun": "minitest", "bigdecimal-util": "bigdecimal", "rexml-document": "rexml", "fiddle-import": "fiddle"}

// rbsLibraries turns an example's `require "net/http"` lines into the `-r net-http` flags rbs needs to see those libraries' signatures. A library the rbs gem has no signatures for (`weakref`, `rexml`) has them vendored in sig/<name>.rbs (decision 110), loaded with -I instead.
func rbsLibraries(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path) //nolint:gosec // example path
	if err != nil {
		t.Fatal(err)
	}
	vendored, err := filepath.Abs("sig")
	if err != nil {
		t.Fatal(err)
	}
	matches := requireLine.FindAllStringSubmatch(string(src), -1)
	// only sig/*.rbs: -I loads a directory recursively, and sig/gems/ (decision 160) would clash with an example's own classes
	stdlib := t.TempDir()
	files, err := filepath.Glob(filepath.Join(vendored, "*.rbs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		lerr := os.Symlink(f, filepath.Join(stdlib, filepath.Base(f)))
		if lerr != nil {
			t.Fatal(lerr)
		}
	}
	args := []string{"-I", stdlib}
	for _, m := range matches {
		if onLoadPath(filepath.Dir(path), src, m[1]) {
			continue // the program's own file, not a library
		}
		name := strings.ReplaceAll(m[1], "/", "-")
		if strings.HasPrefix(m[1], "bundler") || bundledGem(filepath.Dir(path), name) {
			continue // Bundler, or a gem the example's own Gemfile brings: its types come from the gem's sig/, not rbs's library set
		}
		if alias, ok := rbsLibraryNames[name]; ok {
			name = alias
		}
		_, serr := os.Stat(filepath.Join(vendored, name+".rbs")) //nolint:gosec // a require name from our own examples
		if serr == nil {
			continue
		}
		args = append(args, "-r", name)
	}
	return args
}

// bundledGem reports whether the Gemfile beside an example names gem.
func bundledGem(dir, gem string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "Gemfile")) //nolint:gosec // an example's own Gemfile
	return err == nil && strings.Contains(string(data), `gem "`+gem+`"`)
}

func TestCompileTestsSkipping(t *testing.T) {
	src := `require "minitest/autorun"

describe "s" do
  before do
    @x = nope_before
  end

  it "compiles" do
    assert_equal 1, 1
  end

  it "does not" do
    nope_it(1)
  end
end
`
	code, _, skipped, err := CompileTestsSkipping(context.Background(), []File{{Name: "s_test.rb", Src: []byte(src)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(skipped))
	for _, s := range skipped {
		got = append(got, fmt.Sprintf("%s:%d %s: %s", s.File, s.Line, s.Name, s.Reason))
	}
	want := []string{"s_test.rb:4 setup: undefined method nope_before for s", "s_test.rb:12 test_0002_does not: undefined method nope_it for s"}
	if !slices.Equal(got, want) {
		t.Fatalf("skipped:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !bytes.Contains(code, []byte(`rugaroo: undefined method nope_it for s`)) {
		t.Fatal("the failing test's body is not its skip")
	}
}

func TestGoModMatchesRepo(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, mod := range []string{"github.com/dlclark/regexp2", "github.com/ebitengine/purego", "github.com/goccy/go-yaml", "github.com/tetratelabs/wazero", "golang.org/x/text"} {
		m := regexp.MustCompile(`(?m)^\s*(?:require\s+)?` + regexp.QuoteMeta(mod) + ` (\S+)`).FindSubmatch(data)
		if m == nil {
			t.Fatalf("go.mod does not require %s", mod)
		}
		if want := mod + " " + string(m[1]) + "\n"; !strings.Contains(GoMod, want) {
			t.Errorf("GoMod %q does not require the repo's %s %s", GoMod, mod, m[1])
		}
		if sum := GoSum(); !strings.Contains(sum, mod+" "+string(m[1])+" h1:") || !strings.Contains(sum, mod+" "+string(m[1])+"/go.mod h1:") {
			t.Errorf("GoSum lacks %s %s's two lines:\n%s", mod, m[1], sum)
		}
	}
	if n := strings.Count(GoSum(), "\n"); n != 10 {
		t.Errorf("GoSum has %d lines, want 10", n)
	}
}

func TestMRICacheable(t *testing.T) {
	for _, tc := range []struct {
		r    mriResult
		want bool
	}{
		{mriResult{Code: 0}, true},
		{mriResult{Code: 1, Stderr: "boom (RuntimeError)"}, true}, // MRI's own failure is part of the oracle
		{mriResult{Code: -1}, false},
		{mriResult{Code: 1, Stderr: "x.rb:1:in 'require': cannot load such file -- rack (LoadError)"}, false},
		{mriResult{Code: 1, Stderr: "LoadError"}, false},
	} {
		if got := mriCacheable(tc.r); got != tc.want {
			t.Errorf("mriCacheable(%+v) = %v, want %v", tc.r, got, tc.want)
		}
	}
}

func TestWriteCacheFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub") // writeCacheFile makes it
	path := filepath.Join(dir, "k")
	for _, body := range []string{"one", "two longer"} {
		err := writeCacheFile(path, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path) //nolint:gosec // t.TempDir
		if err != nil || string(got) != body {
			t.Fatalf("read %q, %v; want %q", got, err, body)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temp files left behind: %v, %v", entries, err)
	}
	empty := sha256.Sum256(nil)
	if gemfileLock() == hex.EncodeToString(empty[:]) {
		t.Fatal("Gemfile.lock not read: the cache key ignores gem versions")
	}
}

// A call no `.rbs` overload arm fits is a compile error naming the arms (decision 12).
func TestRBSOverloadNoArm(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "sig"), 0o750)
	if err != nil {
		t.Fatal(err)
	}
	sig := "class Twice\n  def double: (Integer) -> Integer\n            | (String) -> String\nend\n"
	err = os.WriteFile(filepath.Join(dir, "sig", "twice.rbs"), []byte(sig), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("class Twice\n  def double(x)\n    x * 2\n  end\nend\n\nputs Twice.new.double(1.5)\n")
	_, _, err = Compile(t.Context(), filepath.Join(dir, "main.rb"), src)
	if err == nil || !strings.Contains(err.Error(), "no signature of double matches this call: (Integer) -> Integer | (String) -> String") {
		t.Fatalf("err = %v", err)
	}
}

// A `.rbs` instance variable's type resolves in its class's nesting, as the class's own defs do (decision 160).
func TestRBSIvarNestedType(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "sig"), 0o750)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(dir, "sig", "m.rbs"), []byte("module M\n  class F\n    @p: P\n  end\nend\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("module M\n  class P\n    def touch = 1\n  end\n\n  class F\n    def initialize(pr)\n      @p = pr\n    end\n\n    def go = @p.touch\n  end\nend\n\np M::F.new(M::P.new).go\n")
	_, warns, err := Compile(t.Context(), filepath.Join(dir, "main.rb"), src)
	if err != nil || slices.ContainsFunc(warns, func(w string) bool { return strings.Contains(w, "untyped") }) {
		t.Fatalf("err = %v, warnings %v", err, warns)
	}
}

// TestNilLinksTyped checks decision 307's typing, which minitest cannot see, compiles with no dynamic call.
func TestNilLinksTyped(t *testing.T) {
	src := []byte(`module Merge
  def merge_all(first)
    acc = first
    cur = first&.sibling
    while cur
      nxt = cur.sibling
      acc = meld(acc, cur)
      cur = nxt
    end
    acc
  end
end

class Node
  attr_accessor :val, :child, :sibling

  def initialize(val)
    @val = val
    @child = nil
    @sibling = nil
  end
end

class Heap
  include Merge

  def initialize(&block)
    @root = nil
    @order = block || :<=.to_proc
  end

  def push(v)
    n = Node.new(v)
    @root = @root ? meld(@root, n) : n
    self
  end

  def meld(a, b)
    if @order[a.val, b.val]
      b.sibling = a.child
      a.child = b
      a
    else
      a.sibling = b.child
      b.child = a
      b
    end
  end

  def pop
    r = @root
    return nil unless r

    @root = merge_all(r.child)
    r.val
  end
end

h = Heap.new { |a, b| a >= b }
[5, 3, 8].each { |x| h.push(x) }
p h.pop
`)
	out, warns, err := Compile(t.Context(), "main.rb", src)
	if err != nil || slices.ContainsFunc(warns, func(w string) bool { return strings.Contains(w, "dynamic call") }) || bytes.Contains(out, []byte("rbDyn")) {
		t.Fatalf("err = %v, warnings %v", err, warns)
	}
}

// TestIvarCyclesTyped checks decision 383's typing, which minitest cannot see: an ivar typed only through a call whose
// parameter it types, a class body's unannotated `@levar = 1`, and counters bumped through their attr writers
// (`l.score += n`, decision 394) compile with no dynamic call.
func TestIvarCyclesTyped(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "test", "infer_test.rb"))
	if err != nil {
		t.Fatal(err)
	}
	from := bytes.Index(src, []byte("  class IvCycNode"))
	to := bytes.Index(src, []byte("  class IvarCycleTest"))
	if from < 0 || to < from {
		t.Fatal("infer_test.rb: IvCyc classes not found")
	}
	prog := append(append([]byte(nil), src[from:to]...), `h = IvCycHeap.new
[5, 3, 8].each { |x| h.push(x) }
acc = IvCycAcc.new
[1, 2, 3].each { |x| acc.add(x) }
pg = IvCycPing.new
3.times { |i| pg.step(i) }
l = IvCycLimits.new(10)
IvCycBody.new([1, 2]).render(l, "abc")
p h.peek, acc.total, pg.show, IvCycLevar.get, l.length, l.score
`...)
	out, warns, err := Compile(t.Context(), "main.rb", prog)
	if err != nil || slices.ContainsFunc(warns, func(w string) bool { return strings.Contains(w, "dynamic call") || strings.Contains(w, "untyped") }) || bytes.Contains(out, []byte("rbDyn")) {
		t.Fatalf("err = %v, warnings %v", err, warns)
	}
}

// TestTupleStackTyped checks decision 384: Arrays used as stacks of tuples (array_test.rb ArrayTupleStackTest) compile with no dynamic call.
func TestTupleStackTyped(t *testing.T) {
	t.Parallel()
	src := []byte(`class Node
  attr_reader :name, :kids

  def initialize(name, kids = [])
    @name = name
    @kids = kids
  end
end

class Parser
  def initialize
    @stack = []
    @tree = Node.new("root")
    @depth = 0
  end

  def enter(el)
    @stack.push([@tree, @depth])
    @tree = el
    @depth += 1
  end

  def leave
    top = @stack.pop
    @tree, @depth = top if top
  end

  def path = @stack.map { |t, d| "#{t.name}:#{d}" } + ["#{@tree.name}:#{@depth}"]
end

def mixed(root)
  stack = []
  stack << [root, :open]
  out = []
  until stack.empty?
    node, state = stack.pop
    out << "#{state}:#{node.name}"
    stack << [node, :close] if state == :open && node.name == "never"
    node.kids.reverse_each { |k| stack << [k, :open] }
  end
  out
end

def pairs(n)
  stack = [[n, 0]]
  out = []
  until stack.empty?
    k, d = stack.pop
    out << "#{k}@#{d}"
    stack.push([k - 1, d + 1]) if k > 0
  end
  out
end

def triples(root)
  stack = [[root, "", 0]]
  out = []
  while (top = stack.pop)
    node, path, depth = top
    out << "#{path}/#{node.name}:#{depth}"
    node.kids.each { |k| stack.push([k, path + "/" + node.name, depth + 1]) }
  end
  out
end

tree = Node.new("a", [Node.new("b", [Node.new("d")]), Node.new("c")])
p mixed(tree), pairs(3), triples(tree)
pr = Parser.new
pr.enter(Node.new("a"))
pr.leave
p pr.path
`)
	out, warns, err := Compile(t.Context(), "main.rb", src)
	if err != nil || slices.ContainsFunc(warns, func(w string) bool { return strings.Contains(w, "dynamic call") }) || bytes.Contains(out, []byte("rbDyn")) {
		t.Fatalf("err = %v, warnings %v", err, warns)
	}
}

// TestSureTake: a shift/pop an emptiness check guards is lowered to the element at the site, typing the Array it fills; a call before it, redo, or a program's own Array#shift keep E? (decision 391).
func TestSureTake(t *testing.T) {
	t.Parallel()
	take := regexp.MustCompile(`t\d+_ := \w+\.s\[(0|len\(\w+\.s\)-1)\]`)
	for _, c := range []struct {
		name, src string
		lowered   bool
	}{
		{"lists", "def m\n  a = (1..9).to_a\n  b = Array.new\n  while (not a.empty?)\n    b.push(a.shift)\n  end\n  until b.empty?\n    a << b.pop\n  end\n  a\nend\np m\n", true},
		{"call_first", "#: (Array[Integer], Array[Integer?]) -> Array[Integer?]\ndef d(a, o) = (a.clear; o)\ndef m\n  a = [1]\n  o = [] #: Array[Integer?]\n  d(a, o).push(a.shift) until a.empty?\n  o\nend\np m\n", false},
		{"redo", "def m\n  a = [1]\n  o = []\n  while !a.empty?\n    o << a.shift\n    redo if o.size == 1\n  end\n  o\nend\np m\n", false},
		{"own_shift", "class Array\n  def shift = nil\nend\ndef m\n  a = [1]\n  o = []\n  o << a.shift unless a.empty?\n  o\nend\np m\n", false},
	} {
		out, warns, err := Compile(t.Context(), "main.rb", []byte(c.src))
		if err != nil || bytes.Contains(out, []byte("rbDyn")) {
			t.Fatalf("%s: err = %v, warnings %v", c.name, err, warns)
		}
		if got := take.Match(out); got != c.lowered {
			t.Errorf("%s: lowered = %v, want %v", c.name, got, c.lowered)
		}
	}
}

// TestInlinePush: a one-element push or << statement appends at the site, unless the program defines its own (decision 391).
func TestInlinePush(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]bool{
		"a = [1]\na << 2\na.push(3)\np a\n": true,
		"class Array\n  #: (E) -> self\n  def <<(x) = self\n\n  #: (*E) -> self\n  def push(*x) = self\nend\na = [1]\na << 2\na.push(3)\np a\n": false,
	} {
		out, _, err := Compile(t.Context(), "main.rb", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Contains(out, []byte("rbIdxA.s = append(rbIdxA.s, rbIdxV)")); got != want {
			t.Errorf("%q: inlined = %v, want %v", src, got, want)
		}
	}
}

// TestIntOverflowValuePanic: no MRI oracle, MRI promotes to a Bignum; rescue, ensure and the top level must still see the RangeError (decision 400).
func TestIntOverflowValuePanic(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	src := "# int_overflow: raise\ndef add(a, b) = a + b\ndef neg(a) = -a\n\ndef check\n  yield\nrescue RangeError => e\n  puts \"#{e.class}: #{e.message} #{(e.backtrace || [])[0]}\"\nend\n\n" +
		"big = 4611686018427387904\ncheck { add(big, big) }\ncheck { add(-big, -big - 1) }\ncheck { neg(-big - big) }\ncheck { (-big - big) / -1 }\n" +
		"def mul(a, b) = a * b\ncheck { mul(big, 2) }\ncheck { mul(-big - big, -1) }\ncheck { mul(-1, -big - big) }\nputs mul(-big, 2)\nputs mul(1099511627776, 1048576)\ncheck { mul(4294967296, 2147483648) }\n" +
		"x = begin\n  add(big, big)\nrescue => e\n  e.class.name\nensure\n  puts \"ensure\"\nend\nputs x\n" +
		"def inner(b)\n  add(b, b)\nrescue ZeroDivisionError\n  0\nend\ncheck { inner(big) }\nadd(big, big)\n"
	gen := transpile(t, "overflow.rb", []byte(src))
	code, err := os.ReadFile(filepath.Join(gen, "main.go")) //nolint:gosec // under t.TempDir()
	if err != nil || !bytes.Contains(code, []byte("panic(rbIntOverflowed{")) || bytes.Contains(code, []byte("rbIntMul(")) {
		t.Fatalf("Integer#+ or #* no longer panics rbIntOverflowed (err %v)", err)
	}
	got := binOutput(t, gen, progIO{}, goBuild(t, gen, ""), nil)
	const msg = " overflows Integer (64-bit; no Bignum)"
	want := "RangeError: 4611686018427387904 + 4611686018427387904" + msg + " overflow.rb:2\n" +
		"RangeError: -4611686018427387904 + -4611686018427387905" + msg + " overflow.rb:2\n" +
		"RangeError: 0 - -9223372036854775808" + msg + " overflow.rb:3\n" +
		"RangeError: -9223372036854775808 / -1" + msg + " overflow.rb:15\n" +
		"RangeError: 4611686018427387904 * 2" + msg + " overflow.rb:16\n" +
		"RangeError: -9223372036854775808 * -1" + msg + " overflow.rb:16\n" +
		"RangeError: -1 * -9223372036854775808" + msg + " overflow.rb:16\n" +
		"-9223372036854775808\n1152921504606846976\n" +
		"RangeError: 4294967296 * 2147483648" + msg + " overflow.rb:16\n" +
		"ensure\nRangeError\n" +
		"RangeError: 4611686018427387904 + 4611686018427387904" + msg + " overflow.rb:2\n" // through a rescue that did not match: the overflow's own frames
	if stdout := regexp.MustCompile(`:in '[^']*'`).ReplaceAllString(got.Stdout, ""); stdout != want || got.Code != 1 || !strings.Contains(got.Stderr, "4611686018427387904 + 4611686018427387904"+msg+" (RangeError)") {
		t.Errorf("exit %d\nstdout:\n%s\nwant:\n%s\nstderr:\n%s", got.Code, got.Stdout, want, got.Stderr)
	}
}

// TestStructWriterFrozenConst: the writer's frozen check folds away only when nothing can freeze (decision 400).
func TestStructWriterFrozenConst(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]bool{
		"P = Struct.new(:x) #: [Integer]\np = P.new(1)\np.x = 2\np p\n":           false,
		"P = Struct.new(:x) #: [Integer]\np = P.new(1)\np.freeze\np.x = 2\np p\n": true,
	} {
		out, _, err := Compile(t.Context(), "main.rb", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Contains(out, []byte("Struct___FrozenFlagQ(self)")); got != want {
			t.Errorf("%q: writer checks the flag = %v, want %v", src, got, want)
		}
	}
}

// TestSpinelBenchmarksTyped guards spinel's unmodified benchmarks against falling back to dynamic calls (decisions 355-357).
func TestSpinelBenchmarksTyped(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"binary_trees", "splay"} {
		path := filepath.Join("benchmarks", name+".rb")
		src, err := fs.ReadFile(os.DirFS("benchmarks"), name+".rb")
		if err != nil {
			t.Fatal(err)
		}
		out, warns, err := Compile(t.Context(), path, src)
		if err != nil || slices.ContainsFunc(warns, func(w string) bool { return strings.Contains(w, "dynamic call") || strings.Contains(w, "untyped") }) || bytes.Contains(out, []byte("rbDyn")) {
			t.Errorf("%s: err = %v, warnings %v", name, err, warns)
		}
	}
}

// TestPairingHeapGemTyped compiles benchmarks/pairing_heap.rb, the unmodified gem with decrease_key (an alias a generic
// subclass makes of its parent's method): its node links stay static as decision 307 left them. Two regressions did not show in
// TestNilLinksTyped: a gem parameter's untyped use in an early round counted as a kind (decision 331), and a subclass instance
// discovered before its parent's declared @root untyped (decision 345). Each left ~45 rbDyn calls; NodeVisitor's 5 `stack.pop` sites typed by decision 384.
func TestPairingHeapGemTyped(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("benchmarks", "pairing_heap.rb"))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := CompileWithGems(t.Context(), filepath.Join("benchmarks", "pairing_heap.rb"), src)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(regexp.MustCompile(`rbDyn[A-Z]\w*\(rbCall`).FindAll(out, -1)); n > 0 {
		t.Errorf("%d rbDyn call sites, want none", n)
	}
}

// TestVendoredGemsMRI runs each vendored gem's own minitest files (gems/<name>/test, a submodule; decision 328) under MRI with
// the gem's lib first on the load path, so the pure-Ruby copy is checked as a gem, not only as compiled code.
func TestVendoredGemsMRI(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	gems, err := filepath.Glob(filepath.Join("gems", "*", "lib"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gems) == 0 {
		t.Fatal("no gems/*/lib: run `git submodule update --init`")
	}
	for _, lib := range gems {
		dir := filepath.Dir(lib)
		tests, err := filepath.Glob(filepath.Join(dir, "test", "*_test.rb"))
		if err != nil || len(tests) == 0 {
			t.Fatalf("%s/test has no *_test.rb (%v)", dir, err)
		}
		for _, test := range tests {
			t.Run(filepath.Base(dir)+"/"+filepath.Base(test), func(t *testing.T) {
				out, err := exec.CommandContext(t.Context(), "ruby", "-I", lib, test, "--seed", "1").CombinedOutput() //nolint:gosec // our own paths
				if err != nil {
					t.Fatalf("ruby -I %s %s: %v\n%s", lib, test, err, rdocNoise.ReplaceAll(out, nil))
				}
			})
		}
	}
}

// TestGofmt fails on any tracked Go file gofmt would change: `golangci-lint run` does not check formatters (v2 runs them under `fmt`), so a file
// split or move that leaves whitespace drift would otherwise pass every gate.
func TestGofmt(t *testing.T) {
	t.Parallel()

	out, err := exec.CommandContext(t.Context(), "git", "ls-files", "*.go").Output()
	if err != nil {
		t.Fatal(err)
	}

	var files []string

	for _, f := range strings.Fields(string(out)) {
		_, statErr := os.Stat(f)
		if statErr == nil { // a tracked file deleted in the working tree is not drift
			files = append(files, f)
		}
	}

	if len(files) == 0 {
		t.Fatal("git ls-files found no Go files: nothing was checked")
	}

	bad, err := exec.CommandContext(t.Context(), "gofmt", append([]string{"-l"}, files...)...).Output() //nolint:gosec // git's own file list
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(bad)) != "" {
		t.Errorf("gofmt would change:\n%s(run `gofmt -w` on them)", bad)
	}
}

// TestGoModNoReplace keeps go.mod installable: `go install .../cmd/rugaroo@latest` is refused for a module whose go.mod has a replace directive.
func TestGoModNoReplace(t *testing.T) {
	t.Parallel()

	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(mod, []byte("module github.com/jtarchie/rugaroo")) {
		t.Fatal("go.mod does not look like this module's: nothing was checked")
	}

	for line := range strings.SplitSeq(string(mod), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "replace") {
			t.Errorf("go.mod has a replace directive, which breaks `go install ...@latest`: %q", line)
		}
	}
}

// walkFiles lists the files under dir (slash paths relative to the working directory), failing on any walk error.
func walkFiles(t *testing.T, fsys fs.FS, dir string, keep func(path string) bool) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && keep(path) {
			out = append(out, path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}

// TestVendoredMirror checks vendored/ against the gems/*/lib submodules in both directions (decision 328): a file kept after its gem dropped it is stale.
func TestVendoredMirror(t *testing.T) {
	gems, err := filepath.Glob(filepath.Join("gems", "*", "lib"))
	if err != nil {
		t.Fatal(err)
	}
	isRb := func(path string) bool { return strings.HasSuffix(path, ".rb") }
	want := map[string]string{} // vendored/<name>/<rel> => its gems/ source
	for _, dir := range gems {
		for _, lib := range walkFiles(t, os.DirFS("."), filepath.ToSlash(dir), isRb) {
			parts := strings.SplitN(lib, "/", 4) // gems/<name>/lib/<rel>
			want["vendored/"+parts[1]+"/"+parts[3]] = lib
		}
	}
	if len(want) == 0 {
		t.Fatal("no gems/*/lib/**/*.rb: run `git submodule update --init`")
	}
	for mirror, lib := range want {
		src, err := os.ReadFile(lib) //nolint:gosec // our own paths
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(mirror) //nolint:gosec // our own paths
		if err != nil || !bytes.Equal(src, got) {
			t.Errorf("%s differs from %s (or is missing): run scripts/vendor-gems", mirror, lib)
		}
	}
	for _, f := range walkFiles(t, os.DirFS("."), "vendored", func(string) bool { return true }) {
		if strings.HasPrefix(f, "vendored/prism/") { // a bundled gem: scripts/vendor-prism, held by TestPrismWasm (decision 427)
			continue
		}
		if _, ok := want[f]; !ok {
			t.Errorf("%s has no source in gems/*/lib (stale): run scripts/vendor-gems", f)
		}
	}
}

// TestVendoredEmbedded: the //go:embed patterns stop at a depth, so a deeper gem file would silently not ship from the module proxy.
func TestVendoredEmbedded(t *testing.T) {
	onDisk := walkFiles(t, os.DirFS("."), "vendored", func(string) bool { return true })
	embedded := walkFiles(t, Prelude, "vendored", func(string) bool { return true })
	if len(onDisk) == 0 || len(embedded) == 0 {
		t.Fatalf("compared nothing: %d files on disk, %d embedded", len(onDisk), len(embedded))
	}
	for _, f := range onDisk {
		if !slices.Contains(embedded, f) {
			t.Errorf("%s is vendored but not embedded: widen the //go:embed patterns in rugaroo.go (or the file is not .rb)", f)
		}
	}
	for _, f := range embedded {
		if !slices.Contains(onDisk, f) {
			t.Errorf("%s is embedded but not on disk", f)
		}
	}
}

var claudeLast = regexp.MustCompile(`\(last: (\d+)\)`)

// TestDesignIndex keeps docs/design.md's generated index (scripts/design-index also checks docs/decisions/ is
// numbered 1..N) and CLAUDE.md's "(last: N)" in step.
func TestDesignIndex(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "ruby", "scripts/design-index", "--check").CombinedOutput()
	if err != nil {
		t.Errorf("scripts/design-index --check: %v (exit 1 = stale index: run scripts/design-index)\n%s", err, out)
	}
	files, err := filepath.Glob(filepath.Join("docs", "decisions", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	claude, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	m := claudeLast.FindSubmatch(claude)
	switch {
	case m == nil:
		t.Error(`CLAUDE.md has no "(last: N)"`)
	case string(m[1]) != strconv.Itoa(len(files)):
		t.Errorf("CLAUDE.md says (last: %s), docs/decisions/ holds %d", m[1], len(files))
	}
}

// gemCorpusKnown are the kramdown testcases whose HTML differs from MRI's, and why; a case that starts to match must
// leave the list (decision 334).
var gemCorpusKnown = map[string]string{
	"/block/14_table/escaping.text":   "`x and return` on a method rugaroo types void (its last statement is a multiple assignment)",
	"/span/05_html/normal.text":       "`x and return` on a method rugaroo types void (its last statement is a multiple assignment)",
	"/encoding.text":                  "byte indexing after force_encoding('ASCII-8BIT') on valid UTF-8: Strings carry no encoding (decision 136)",
	"/span/05_html/across_lines.text": "`h.each_value { |v| v.gsub!(...) }` edits a copy (decision 136)",
}

// TestGemCorpus runs a gem's own testcases through a program compiled from a traced run over all of them and compares
// each case's output with MRI's (decision 334): one document per feature, as examples and benchmarks give, misses most
// of a gem's paths.
func TestGemCorpus(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	gemDir, err := exec.CommandContext(t.Context(), "ruby", "-e", `print Gem::Specification.find_by_name("kramdown").gem_dir`).Output()
	if err != nil {
		t.Skip("kramdown is not installed")
	}
	cases := filepath.Join(string(gemDir), "test", "testcases")
	main, err := filepath.Abs(filepath.Join("testdata", "corpus", "kramdown.rb"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := CompileTrace(t.Context(), main, [][]string{{cases}}, nil)
	if err != nil {
		t.Fatalf("rugaroo: %v", err)
	}
	gen := t.TempDir()
	writeModule(t, gen)
	werr := os.WriteFile(filepath.Join(gen, "main.go"), code, 0o600)
	if werr != nil {
		t.Fatal(werr)
	}
	bin := goBuild(t, gen, "")
	mri, err := exec.CommandContext(t.Context(), "ruby", main, cases).Output() //nolint:gosec // our own paths
	if err != nil {
		t.Fatalf("ruby: %v", err)
	}
	got, err := exec.CommandContext(t.Context(), bin, cases).Output() //nolint:gosec // the binary just built
	if err != nil {
		t.Fatalf("%s: %v", bin, err)
	}
	split := func(out []byte) map[string]string {
		m := map[string]string{}
		var name string
		for _, line := range strings.SplitAfter(string(out), "\n") {
			if rest, ok := strings.CutPrefix(line, "== "); ok {
				name = strings.TrimSpace(rest)
				continue
			}
			m[name] += line
		}
		return m
	}
	want, have := split(mri), split(got)
	if len(want) < 100 {
		t.Fatalf("MRI converted only %d testcases", len(want))
	}
	for name, w := range want {
		reason, known := gemCorpusKnown[name]
		switch {
		case have[name] != w && !known:
			t.Errorf("%s differs from MRI:\nruby:  %q\nrugaroo: %q", name, w, have[name])
		case have[name] == w && known:
			t.Errorf("%s now matches MRI: remove it from gemCorpusKnown (%s)", name, reason)
		}
	}
}
