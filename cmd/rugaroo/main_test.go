package main

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/jtarchie/rugaroo"
)

func TestBuildAndRun(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.rb")
	err := os.WriteFile(src, []byte("puts \"hi #{1 + 2}\"\nexit 3\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	opts := buildOpts{}

	bin := filepath.Join(dir, "prog")
	if code := build(t.Context(), src, bin, opts); code != 0 {
		t.Fatalf("build: exit %d", code)
	}
	out, err := exec.CommandContext(t.Context(), bin).Output() //nolint:gosec // built above
	if string(out) != "hi 3\n" {
		t.Errorf("stdout %q", out)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Errorf("want exit 3, got %v", err)
	}

	if code := run(t.Context(), src, nil, opts); code != 3 {
		t.Errorf("run: exit %d, want 3", code)
	}
	if code := run(t.Context(), filepath.Join(dir, "missing.rb"), nil, opts); code != 1 {
		t.Errorf("run missing file: exit %d, want 1", code)
	}
}

// TestInterrupt checks decision 60's catchable Ctrl-C: an untrapped SIGINT or
// SIGTERM reaches a blocking call on the main thread as Interrupt or
// SignalException, so rescue and ensure run; uncaught, it ends the program by
// the signal.
func TestInterrupt(t *testing.T) {
	cases := []struct {
		name, src string
		sig       os.Signal
		stdout    string
		exit0     bool
	}{
		{"rescued_sleep", "$stderr.puts \"ready\"\nbegin\n  sleep 10\nrescue Interrupt => e\n  puts \"caught #{e.class} #{e.message} #{e.signo}\"\nensure\n  puts \"ensure\"\nend\nputs \"after\"\n", os.Interrupt, "caught Interrupt Interrupt 2\nensure\nafter\n", true},
		{"term_in_join", "$stderr.puts \"ready\"\nbegin\n  Thread.new { sleep 10 }.join\nrescue SignalException => e\n  puts \"#{e.class} #{e.message} #{e.signo}\"\nend\n", syscall.SIGTERM, "SignalException SIGTERM 15\n", true},
		{"queue_pop", "q = Queue.new #: Queue[Integer]\n$stderr.puts \"ready\"\nbegin\n  q.pop\nrescue Interrupt\n  puts \"pop interrupted\"\nend\n", os.Interrupt, "pop interrupted\n", true},
		{"uncaught", "$stderr.puts \"ready\"\nsleep 10\n", os.Interrupt, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "main.rb")
			err := os.WriteFile(src, []byte(tc.src), 0o600)
			if err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(dir, "main")
			if code := build(t.Context(), src, bin, buildOpts{}); code != 0 {
				t.Fatalf("build: exit %d", code)
			}
			var out, errOut strings.Builder
			cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // built above
			cmd.Stdout = &out
			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatal(err)
			}
			err = cmd.Start()
			if err != nil {
				t.Fatal(err)
			}
			ready := make([]byte, 6)
			_, err = io.ReadFull(stderr, ready)
			if err != nil {
				t.Fatal(err)
			}
			_ = cmd.Process.Signal(tc.sig)
			rest, _ := io.ReadAll(stderr)
			errOut.Write(rest)
			err = cmd.Wait()
			if out.String() != tc.stdout {
				t.Errorf("stdout %q, want %q (stderr %q)", out.String(), tc.stdout, errOut.String())
			}
			var exit *exec.ExitError
			switch {
			case tc.exit0 && err != nil:
				t.Errorf("want exit 0, got %v (stderr %q)", err, errOut.String())
			case !tc.exit0 && (!errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != tc.sig):
				t.Errorf("want death by %v, got %v", tc.sig, err)
			case !tc.exit0 && !strings.Contains(errOut.String(), "Interrupt (Interrupt)"):
				t.Errorf("stderr %q, want the uncaught Interrupt", errOut.String())
			}
		})
	}
}

func TestSignalFlushes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "spin.rb")
	err := os.WriteFile(src, []byte("print \"x\"\n$stderr.puts \"ready\"\ni = 0\nwhile true\n  i += 1\nend\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "spin")
	if code := build(t.Context(), src, bin, buildOpts{}); code != 0 {
		t.Fatalf("build: exit %d", code)
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		var out strings.Builder
		cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // built above
		cmd.Stdout = &out
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		err = cmd.Start()
		if err != nil {
			t.Fatal(err)
		}
		ready := make([]byte, 6) // wait for the child's own readiness signal instead of a fixed sleep, which raced under load
		_, err = io.ReadFull(stderr, ready)
		if err != nil {
			t.Fatal(err)
		}
		_ = cmd.Process.Signal(sig)
		err = cmd.Wait()
		// MRI flushes buffered output, then dies by the signal.
		if out.String() != "x" {
			t.Errorf("%v: stdout %q, want buffered output flushed", sig, out.String())
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != sig {
			t.Errorf("%v: want death by signal, got %v", sig, err)
		}
	}
}

func TestTestCommand(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{ //nolint:gosec // Ruby sources, not credentials
		"pass_test.rb":     "require \"minitest/autorun\"\n\nclass PassTest < Minitest::Test\n  def test_ok = assert_equal(2, 1 + 1)\nend\n",
		"sub/fail_test.rb": "require \"minitest/autorun\"\n\nclass FailTest < Minitest::Test\n  def test_bad = assert_equal(3, 1 + 1)\n  def test_good = assert(true)\nend\n",
		"helper.rb":        "puts \"not a test file\"\n",
	}
	for name, src := range files {
		p := filepath.Join(dir, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(src), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	// one program, one run, one report, as `minitest dir` gives
	out, code := captureStdout(t, func() int { return testCmd(t.Context(), []string{dir, "--seed", "7"}) })
	if code != 1 {
		t.Errorf("exit %d, want 1 (a test fails)", code)
	}
	for _, want := range []string{
		"Run options: --seed 7\n",
		"FailTest#test_bad [" + filepath.Join(dir, "sub", "fail_test.rb") + ":4]:\nExpected: 3\n  Actual: 2\n",
		"3 runs, 3 assertions, 1 failures, 0 errors, 0 skips",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not a test file") {
		t.Errorf("helper.rb isn't a test file:\n%s", out)
	}

	// minitest's own flags pass through; rugaroo's are taken out by name
	out, code = captureStdout(t, func() int {
		return testCmd(t.Context(), []string{"-n", "/good/", filepath.Join(dir, "sub", "fail_test.rb"), "-v", "--seed", "7"})
	})
	if code != 0 || !strings.Contains(out, "Run options: -n /good/ -v --seed 7\n") || !strings.Contains(out, "FailTest#test_good = ") {
		t.Errorf("-n /good/ -v: exit %d\n%s", code, out)
	}

	opts, rest, err := testBuildFlags([]string{"-v", "-race", "x_test.rb", "-gcflags", "-l", "--seed", "3", "-gcflags=-e", "-I", "a", "-Ib", "-I=c"})
	if err != nil || !opts.race || opts.gcflags != "-e" || !slices.Equal(rest, []string{"-v", "x_test.rb", "--seed", "3"}) || !slices.Equal(opts.loadPath, []string{"a", "b", "c"}) {
		t.Errorf("testBuildFlags: race=%v gcflags=%q -I=%q rest=%q err=%v", opts.race, opts.gcflags, opts.loadPath, rest, err)
	}
}

// TestRunGem runs unmodified Cuba on rack, found as installed gems through the repo's Gemfile, against MRI (decision 173).
func TestRunGem(t *testing.T) {
	gemfile, err := filepath.Abs(filepath.Join("..", "..", "Gemfile"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUNDLE_GEMFILE", gemfile)
	dir := t.TempDir()
	src := filepath.Join(dir, "main.rb")
	err = os.WriteFile(src, []byte(`require "cuba"
Cuba.define { on("users/:id") { |id| res.write "user #{id}" } }
s, _h, b = Cuba.call({ "REQUEST_METHOD" => "GET", "PATH_INFO" => "/users/7", "SCRIPT_NAME" => "", "QUERY_STRING" => "" })
puts "#{s} #{b.join}"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	got, code := captureStdout(t, func() int { return run(t.Context(), src, nil, buildOpts{}) })
	want, err := exec.CommandContext(t.Context(), "ruby", "-rbundler/setup", src).Output() //nolint:gosec // written above
	if err != nil || code != 0 || got != string(want) {
		t.Errorf("rugaroo run (exit %d): %q, ruby (%v): %q", code, got, err, want)
	}

	_, _, err = rugaroo.CompileWithGems(t.Context(), src, []byte("require \"rugaroo_no_such_gem\"\n"))
	if err == nil || !strings.Contains(err.Error(), "cannot load such file -- rugaroo_no_such_gem") {
		t.Errorf("unknown require: %v", err)
	}
	t.Setenv("PATH", "")
	_, _, err = rugaroo.CompileWithGems(t.Context(), src, []byte("require \"set\"\nrequire \"json\"\nrequire \"English\"\nputs 1\n"))
	if err != nil {
		t.Errorf("a program without gems needs no ruby: %v", err)
	}
	_, _, err = rugaroo.CompileWithGems(t.Context(), src, []byte("require \"cuba\"\n"))
	if err == nil || !strings.Contains(err.Error(), "needs ruby on PATH") {
		t.Errorf("gem without ruby: %v", err)
	}
}

// captureStdout runs f with os.Stdout sent to a file, returning what it wrote.
func captureStdout(t *testing.T, f func() int) (string, int) {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = tmp
	code := f()
	os.Stdout = orig
	out, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func TestGen(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.rb")
	err := os.WriteFile(src, []byte("puts \"hi\"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if code := gen(src, buildOpts{}, &out); code != 0 {
		t.Fatalf("gen: exit %d", code)
	}
	if !strings.Contains(out.String(), "\npackage main\n") {
		t.Errorf("not Go source: %.200q", out.String())
	}
	if code := gen(filepath.Join(dir, "missing.rb"), buildOpts{}, io.Discard); code != 1 {
		t.Errorf("gen missing file: exit %d, want 1", code)
	}
}

// An Integer divide by 0 is Go's own panic (decision 151); every way out of
// the program, at_exit included, reports it as MRI's ZeroDivisionError.
func TestUncaughtZeroDivisionMessage(t *testing.T) {
	dir := t.TempDir()
	for name, prog := range map[string]string{
		"main":    "puts 7 / 0\n",
		"at_exit": "at_exit { puts 7 % 0 }\nputs \"main\"\n",
	} {
		src := filepath.Join(dir, name+".rb")
		err := os.WriteFile(src, []byte(prog), 0o600)
		if err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, name)
		if code := build(t.Context(), src, bin, buildOpts{}); code != 0 {
			t.Fatalf("%s: build: exit %d", name, code)
		}
		var errOut strings.Builder
		cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // built above
		cmd.Stderr = &errOut
		err = cmd.Run()
		if err == nil {
			t.Errorf("%s: exit 0, want 1", name)
		}
		if !strings.Contains(errOut.String(), "divided by 0 (ZeroDivisionError)") {
			t.Errorf("%s: stderr %q, want MRI's ZeroDivisionError", name, errOut.String())
		}
	}
}

// RUGAROO_BACKTRACE=1 prints an uncaught exception as MRI does, with its location and from lines (decision 177); unset, `msg (Class)` alone.
func TestUncaughtFullReport(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.rb")
	err := os.WriteFile(src, []byte("def inner = raise(ArgumentError, \"bad\")\ndef outer = inner\nouter\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "main")
	if code := build(t.Context(), src, bin, buildOpts{}); code != 0 {
		t.Fatalf("build: exit %d", code)
	}
	for env, want := range map[string]string{
		"":                    "bad (ArgumentError)\n",
		"RUGAROO_BACKTRACE=1": "main.rb:1:in 'Object#inner': bad (ArgumentError)\n\tfrom main.rb:2:in 'Object#outer'\n\tfrom main.rb:3:in '<main>'\n",
	} {
		var errOut strings.Builder
		cmd := exec.CommandContext(t.Context(), bin) //nolint:gosec // built above
		cmd.Env = append(os.Environ(), env)
		cmd.Stderr = &errOut
		_ = cmd.Run()
		if !strings.HasSuffix(errOut.String(), want) {
			t.Errorf("%q: stderr %q, want suffix %q", env, errOut.String(), want)
		}
	}
	// the binary above has no DWARF (decision 437): the backtrace came from the pclntab; -debug keeps it
	debugBin := filepath.Join(dir, "debug")
	if code := build(t.Context(), src, debugBin, buildOpts{debug: true}); code != 0 {
		t.Fatalf("build -debug: exit %d", code)
	}
	for b, want := range map[string]bool{bin: false, debugBin: true} {
		if got := hasDWARF(t, b); got != want {
			t.Errorf("%s: DWARF %v, want %v", filepath.Base(b), got, want)
		}
	}
}

// hasDWARF reads an ELF or Mach-O binary's debug info.
func hasDWARF(t *testing.T, bin string) bool {
	t.Helper()
	var dwarfOf func() (*dwarf.Data, error)
	ef, err := elf.Open(bin)
	if err == nil {
		defer func() { _ = ef.Close() }()
		dwarfOf = ef.DWARF
	} else {
		mf, merr := macho.Open(bin)
		if merr != nil {
			t.Fatal(merr)
		}
		defer func() { _ = mf.Close() }()
		dwarfOf = mf.DWARF
	}
	_, err = dwarfOf()
	return err == nil
}

// A user -gcflags joins the default -dwarf=false (go build keeps only the last matching one); a pattern one stays separate.
func TestBuildFlags(t *testing.T) {
	for _, c := range []struct {
		opts buildOpts
		want []string
	}{
		{buildOpts{}, []string{"-ldflags=-w", "-gcflags=-dwarf=false"}},
		{buildOpts{gcflags: "-e"}, []string{"-ldflags=-w", "-gcflags=-dwarf=false -e"}},
		{buildOpts{gcflags: "all=-N -l"}, []string{"-ldflags=-w", "-gcflags=-dwarf=false", "-gcflags=all=-N -l"}},
		{buildOpts{debug: true, gcflags: "-e"}, []string{"-gcflags=-e"}},
	} {
		if got := buildFlags(c.opts); !slices.Equal(got, c.want) {
			t.Errorf("%+v: %q, want %q", c.opts, got, c.want)
		}
	}
}
