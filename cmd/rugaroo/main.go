// Command rugaroo builds or runs a typed Ruby file as Go, like `go build` and `go run`, or prints the Go it compiles to.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/jtarchie/rugaroo"
)

const usage = `usage: rugaroo build [-o prog] [-I dir]... [-gem dir]... [-main file | -trace [-trace-also args]... [-rubyopt opt]...] [-int-overflow raise|wrap|promote] [-cgo] [-race] [-gcflags flags] [-work] main.rb [traced args...]
       rugaroo run [-I dir]... [-gem dir]... [-main file | -trace [-rubyopt opt]...] [-int-overflow raise|wrap|promote] [-cgo] [-race] [-gcflags flags] [-work] main.rb [args...]
       rugaroo gen [-I dir]... [-gem dir]... [-main file | -trace [-rubyopt opt]...] [-int-overflow raise|wrap|promote] [-cgo] main.rb [traced args...] > main.go
       rugaroo test [-I dir]... [-int-overflow raise|wrap|promote] [-cgo] [-race] [-gcflags flags] [-work] [paths...] [minitest flags: -v, -s seed, -n pattern, -e pattern, -S kinds, --show-skips]
       rugaroo web [-addr 127.0.0.1:8080]
       rugaroo web -static dir -assets https://host/path/`

func main() {
	tuneGC()
	rugaroo.SetInferCache(os.Getenv("RUGAROO_NO_INFER_CACHE") == "") // the next compile of a program starts from this one's inferred types (decision 430)
	if len(os.Args) < 2 || os.Args[1] != "build" && os.Args[1] != "run" && os.Args[1] != "test" && os.Args[1] != "gen" && os.Args[1] != "web" && os.Args[1] != "stubs" {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if os.Args[1] == "test" {
		ctx, stop := notifyContext()
		code := testCmd(ctx, os.Args[2:])
		stop()
		os.Exit(code)
	}
	if os.Args[1] == "web" {
		os.Exit(webCmd(os.Args[2:]))
	}
	if os.Args[1] == "stubs" { // hidden: prelude/go/0_stubs.go for gopls (decision 154); TestPreludeGo regenerates it too
		os.Exit(stubs())
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		fs.PrintDefaults()
	}
	var out *string
	if cmd == "build" {
		out = fs.String("o", "", "output binary (default: the file's name without .rb)")
	}
	var opts buildOpts
	fs.BoolVar(&opts.race, "race", false, "build with the race detector")
	fs.StringVar(&opts.gcflags, "gcflags", "", "passed to go build (e.g. -e for every error)")
	fs.BoolVar(&opts.work, "work", false, "keep the temp module (main.go, go.mod) and print its path")
	fs.StringVar(&opts.main, "main", "", "the file is a traced entry (the requires MRI loaded) and this is the program it runs (decisions 197, 288)")
	fs.BoolVar(&opts.trace, "trace", false, "run the program once under MRI with its args (it EXECUTES) and compile what it loaded (decision 296)")
	fs.Func("trace-also", "another `args` list to trace, space-separated: its loads join the compile, for one binary that serves several commands (repeatable)", func(a string) error {
		opts.also = append(opts.also, strings.Fields(a))
		return nil
	})
	fs.Func("rubyopt", "an MRI option for the -trace run, as the program's launcher passes it (repeatable)", func(o string) error {
		opts.rubyOpts = append(opts.rubyOpts, o)
		return nil
	})
	fs.Func("int-overflow", "what an Integer result past 64 bits does: `raise` (RangeError, the default), wrap or promote (to a Bignum; decision 372)", func(m string) error {
		return os.Setenv("RUGAROO_INT_OVERFLOW", m) // read per compile, as the other RUGAROO_* settings
	})
	fs.Func("nil-check", "a possibly-nil value where a value is expected: a `compile` error (the default), or a runtime check that raises on nil as MRI does (decision 393)", func(m string) error {
		return os.Setenv("RUGAROO_NIL_CHECK", m)
	})
	fs.BoolFunc("cgo", "Fiddle and FFI call C through cgo instead of purego, which is then not linked (needs a C compiler; decision 426)", func(string) error {
		return os.Setenv("RUGAROO_CGO", "1")
	})
	fs.Func("I", "a directory `require` searches for your own files, as `ruby -I` (repeatable)", func(dir string) error {
		opts.loadPath = append(opts.loadPath, dir)
		return nil
	})
	fs.Func("gem", "a directory of library code the program carries, compiled as an installed gem is: only what the program reaches, leniently (repeatable; decision 406)", func(dir string) error {
		opts.gemDirs = append(opts.gemDirs, dir)
		return nil
	})
	_ = fs.Parse(os.Args[2:]) // ExitOnError
	if fs.NArg() < 1 || cmd != "run" && fs.NArg() != 1 && !opts.trace {
		fs.Usage()
		os.Exit(2)
	}
	file := fs.Arg(0)
	opts.args = fs.Args()[1:]
	if cmd == "gen" {
		os.Exit(gen(file, opts, os.Stdout))
	}
	ctx, stop := notifyContext() // os.Exit skips defers
	var code int
	if cmd == "build" {
		bin := *out
		if bin == "" {
			bin = strings.TrimSuffix(filepath.Base(file), ".rb")
		}
		code = build(ctx, file, bin, opts)
	} else {
		code = run(ctx, file, opts.args, opts)
	}
	stop()
	os.Exit(code)
}

// notifyContext is cancelled by the first SIGINT/SIGTERM, with the signal as its cause. Installed before the compile and the go build, so the deferred cleanup of the temp module always runs; the handler stays for the process's life.
func notifyContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigs:
			cancel(signalError{sig})
		case <-ctx.Done():
		}
	}()
	return ctx, func() { signal.Stop(sigs); cancel(nil) }
}

// signalError is the cancel cause of notifyContext.
type signalError struct{ sig os.Signal }

func (e signalError) Error() string { return "interrupted by " + e.sig.String() }

// interrupted is the shell's 128+signal status when ctx was cancelled by a signal, else 0.
func interrupted(ctx context.Context) int {
	var se signalError
	if errors.As(context.Cause(ctx), &se) {
		if sig, ok := se.sig.(syscall.Signal); ok {
			return 128 + int(sig)
		}
	}
	return 0
}

// gen writes the generated Go to out and warnings to stderr: the main.go that build -work keeps, without needing the go command.
func stubs() int {
	out, err := rugaroo.PreludeStubs(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = os.WriteFile(rugaroo.StubsPath, out, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func gen(file string, opts buildOpts, out io.Writer) int {
	src, err := os.ReadFile(file) //nolint:gosec // the user's program
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if path := os.Getenv("RUGAROO_CPUPROFILE"); path != "" { // where a slow compile spends its time (go tool pprof)
		f, perr := os.Create(path) //nolint:gosec // a path the developer named
		if perr == nil {
			_ = pprof.StartCPUProfile(f)
			defer func() { pprof.StopCPUProfile(); _ = f.Close() }()
		}
	}
	code, warnings, err := compileFile(context.Background(), file, src, opts)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err == nil {
		_, err = out.Write(code)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

type buildOpts struct {
	race     bool
	gcflags  string
	work     bool
	main     string     // -main: the file is a traced entry and this its program (decision 288)
	trace    bool       // -trace: compile what a run of the file with args loaded (decision 296)
	rubyOpts []string   // -rubyopt
	args     []string   // the program's args
	also     [][]string // -trace-also
	loadPath []string   // -I
	gemDirs  []string   // -gem
}

// compileFile is the CLI's compile: a traced entry with -main (decision 288), else a program.
func compileFile(ctx context.Context, file string, src []byte, opts buildOpts) ([]byte, []string, error) {
	if opts.trace {
		return rugaroo.CompileTrace(ctx, file, append([][]string{opts.args}, opts.also...), opts.rubyOpts, opts.loadPath...) //nolint:wrapcheck // already "rugaroo: "-prefixed
	}
	if opts.main != "" {
		return rugaroo.CompileTraced(ctx, file, src, opts.main, opts.loadPath...) //nolint:wrapcheck // already "rugaroo: "-prefixed
	}
	return rugaroo.CompileWithGemDirs(ctx, file, src, opts.gemDirs, opts.loadPath...) //nolint:wrapcheck // already "rugaroo: "-prefixed
}

func build(ctx context.Context, file, bin string, opts buildOpts) int {
	abs, err := filepath.Abs(bin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := compile(ctx, file, abs, opts, os.Stderr)
	defer cleanup(dir, opts)
	if err != nil {
		return failed(ctx, err)
	}
	return 0
}

// failed reports err and picks the exit status: the signal's when one cancelled ctx, else 1.
func failed(ctx context.Context, err error) int {
	if code := interrupted(ctx); code != 0 {
		return code
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func run(ctx context.Context, file string, args []string, opts buildOpts) int {
	dir, err := compile(ctx, file, "", opts, os.Stderr)
	defer cleanup(dir, opts)
	if err != nil {
		return failed(ctx, err)
	}
	return runProg(ctx, filepath.Join(dir, "prog"), args)
}

// runProg runs a built program in the caller's cwd, as `ruby main.rb`, and returns its exit status. It outlives the child so the caller's temp dir is removed, and forwards what didn't come via the process group (kill -TERM <rugaroo>).
func runProg(ctx context.Context, bin string, args []string) int {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // before the check below, so no signal falls between it and the forwarder
	defer signal.Stop(sigs)
	if code := interrupted(ctx); code != 0 {
		return code
	}
	cmd := exec.CommandContext(context.WithoutCancel(ctx), bin, args...) //nolint:gosec // the binary we just built; a signal is forwarded, not a kill
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	go func() {
		for sig := range sigs {
			_ = cmd.Process.Signal(sig)
		}
	}()
	err = cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exitStatus(exit)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// exitStatus is the child's exit code, or 128+signal for a signaled child (the shell's convention).
func exitStatus(exit *exec.ExitError) int {
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exit.ExitCode()
}

// compile builds into a fresh temp module each time: stdlib-only, so no network, and Go's content-keyed cache keeps rebuilds fast. Warnings and go build's output go to diag.
func compile(ctx context.Context, file, bin string, opts buildOpts, diag io.Writer) (string, error) {
	src, err := os.ReadFile(file) //nolint:gosec // the user's program
	if err != nil {
		return "", err //nolint:wrapcheck // the *PathError already names the file
	}
	code, warnings, err := abandonable(ctx, func() ([]byte, []string, error) {
		return compileFile(ctx, file, src, opts)
	})
	return buildModule(ctx, code, warnings, err, bin, opts, diag)
}

// abandonable returns when ctx is cancelled even while the compile runs on: it is CPU-bound Go that never checks ctx, and nothing it holds needs cleanup (the temp module comes after).
func abandonable(ctx context.Context, f func() ([]byte, []string, error)) ([]byte, []string, error) {
	type result struct {
		code     []byte
		warnings []string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		var r result
		r.code, r.warnings, r.err = f()
		done <- r
	}()
	select {
	case r := <-done:
		return r.code, r.warnings, r.err
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("rugaroo: %w", context.Cause(ctx))
	}
}

// compileFiles builds several files as one program, as Ruby loads them into one process (decision 84).
func compileFiles(ctx context.Context, files []string, bin string, opts buildOpts, diag io.Writer) (string, error) {
	srcs := make([]rugaroo.File, 0, len(files))
	for _, f := range files {
		src, err := os.ReadFile(f) //nolint:gosec // the user's program
		if err != nil {
			return "", err //nolint:wrapcheck // the *PathError already names the file
		}
		srcs = append(srcs, rugaroo.File{Name: f, Src: src})
	}
	code, warnings, err := abandonable(ctx, func() ([]byte, []string, error) {
		return rugaroo.CompileFiles(ctx, srcs, opts.loadPath...)
	})
	return buildModule(ctx, code, warnings, err, bin, opts, diag)
}

// buildModule writes the compiled Go into a fresh module and builds it.
func buildModule(ctx context.Context, code []byte, warnings []string, err error, bin string, opts buildOpts, diag io.Writer) (string, error) {
	for _, w := range warnings {
		_, _ = fmt.Fprintln(diag, "warning:", w)
	}
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "rugaroo-")
	if err != nil {
		return "", fmt.Errorf("temp module: %w", err)
	}
	err = rugaroo.WriteModule(dir)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "main.go"), code, 0o600)
	}
	if err != nil {
		return dir, fmt.Errorf("temp module: %w", err)
	}
	if bin == "" {
		bin = filepath.Join(dir, "prog")
	}
	args := []string{"build", "-trimpath"}
	if opts.race {
		args = append(args, "-race")
	}
	if opts.gcflags != "" {
		args = append(args, "-gcflags="+opts.gcflags)
	}
	cmd := exec.CommandContext(ctx, "go", append(args, "-o", bin, ".")...) //nolint:gosec // flags are the user's own
	cmd.Dir = dir
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) } // go build stops its compilers on SIGINT; a kill would orphan them
	cmd.WaitDelay = 10 * time.Second
	// The user's go.work and GOFLAGS belong to their projects, not this module.
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	cmd.Stdout, cmd.Stderr = diag, diag
	if errors.Is(cmd.Err, exec.ErrNotFound) {
		return dir, errors.New("rugaroo needs the go command on PATH to build")
	}
	err = cmd.Run()
	if err != nil {
		return dir, fmt.Errorf("go build: %w", err)
	}
	return dir, nil
}

func cleanup(dir string, opts buildOpts) {
	if dir == "" {
		return
	}
	if opts.work {
		fmt.Fprintln(os.Stderr, "WORK="+dir)
		return
	}
	_ = os.RemoveAll(dir) //nolint:gosec // dir is our MkdirTemp
}

// tuneGC trades memory for compile speed: the collector was 38% of brew's generation CPU at GOGC=100, and cuba's CPU
// halves at 400 for twice the peak (225 → 490 MB); a 4 GiB soft limit tightens it again on the largest programs
// (decision 240). GOGC and GOMEMLIMIT in the environment win.
func tuneGC() {
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(400)
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(4 << 30)
	}
}
