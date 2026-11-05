package rugaroo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// costLog is set on go build when the test binary is its -toolexec (TestCompileCost).
const costLog = "RUGAROO_COST_LOG"

// The test binary is its own -toolexec so the report reads rusage portably, without /usr/bin/time's per-OS flags.
func TestMain(m *testing.M) {
	log := os.Getenv(costLog)
	if log == "" || len(os.Args) < 2 {
		os.Exit(m.Run())
	}
	tool := strings.TrimSuffix(filepath.Base(os.Args[1]), ".exe")
	cmd := exec.Command(os.Args[1], os.Args[2:]...) //nolint:gosec,noctx // go build's own tool invocation
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	start := time.Now()
	err := cmd.Run()
	if tool == "compile" || tool == "link" {
		pkg := "?"
		for i, a := range os.Args[:len(os.Args)-1] {
			if a == "-p" {
				pkg = os.Args[i+1]
			}
		}
		st := cmd.ProcessState
		rss := int64(0)
		if ru, ok := st.SysUsage().(*syscall.Rusage); ok {
			rss = ru.Maxrss // bytes on darwin, KiB on linux
			if runtime.GOOS != "darwin" {
				rss *= 1024
			}
		}
		line := fmt.Sprintf("%s %s %.2f %.2f %d\n", tool, pkg, time.Since(start).Seconds(), (st.UserTime() + st.SystemTime()).Seconds(), rss)
		f, ferr := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the test's temp file
		if ferr == nil {
			_, _ = f.WriteString(line)
			_ = f.Close()
		}
	}
	if err != nil {
		if st := cmd.ProcessState; st != nil {
			os.Exit(st.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}

// TestCompileCost is #96's yardstick, opt-in and sequential since it measures memory; no DWARF, as test builds (decision 88, #97): RUGAROO_COST=1 (or a comma list of examples) go test -run TestCompileCost -v .
func TestCompileCost(t *testing.T) {
	want := os.Getenv("RUGAROO_COST")
	if want == "" {
		t.Skip("set RUGAROO_COST=1 (or a comma list of examples)")
	}
	requireToolchain(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"005_word_count", "064_observable", "116_cuba_web"}
	if want != "1" {
		names = strings.Split(want, ",")
	}
	t.Logf("%-16s %8s %9s %7s %8s %8s %8s %8s", "program", "srcfuncs", "compiled", "inst", "archMB", "wall", "cpu", "rssMB")
	for _, name := range names {
		rb := filepath.Join("examples", name, "main.rb")
		src, rerr := os.ReadFile(rb) //nolint:gosec // example path
		if rerr != nil {
			t.Fatal(rerr)
		}
		gen := t.TempDir()
		writeModule(t, gen)
		writeGenerated(t, gen, rb, src, CompileWithGems)
		mainGo := filepath.Join(gen, "main.go")
		code, rerr := os.ReadFile(mainGo) //nolint:gosec // under t.TempDir()
		if rerr != nil {
			t.Fatal(rerr)
		}
		// a fresh trailing comment misses the build cache, so main always compiles and is measured
		code = fmt.Appendf(code, "\n// cost %d\n", time.Now().UnixNano())
		werr := os.WriteFile(mainGo, code, 0o600) //nolint:gosec // under t.TempDir()
		if werr != nil {
			t.Fatal(werr)
		}
		log := filepath.Join(t.TempDir(), "cost.log")
		pio := progIO{env: []string{costLog + "=" + log}}
		_, stderr, status, berr := runIO(t, gen, pio, "go", "build", "-trimpath", "-gcflags=-dwarf=false", "-work", "-toolexec", exe, "-o", filepath.Join(gen, "prog"), ".")
		if berr != nil || status != 0 {
			t.Fatalf("%s: go build: %v\n%s", name, berr, stderr)
		}
		work := regexp.MustCompile(`(?m)^WORK=(.*)$`).FindStringSubmatch(stderr)
		if work == nil {
			t.Fatalf("%s: no WORK= in go build output:\n%s", name, stderr)
		}
		archive := filepath.Join(work[1], "b001", "_pkg_.a")
		nm, nerr := run(t, gen, "go", "tool", "nm", archive)
		if nerr != nil {
			t.Fatalf("%s: go tool nm: %v\n%s", name, nerr, nm)
		}
		compiled, inst := 0, 0
		for l := range strings.Lines(nm) {
			f := strings.Fields(l)
			if len(f) >= 3 && f[1] == "T" {
				compiled++
				if strings.Contains(f[2], "[") {
					inst++
				}
			}
		}
		fi, serr := os.Stat(archive)
		if serr != nil {
			t.Fatal(serr)
		}
		_ = os.RemoveAll(work[1])
		logged, lerr := os.ReadFile(log) //nolint:gosec // under t.TempDir()
		if lerr != nil {
			t.Fatal(lerr)
		}
		var wall, cpu string
		var rss int64
		for l := range strings.Lines(string(logged)) {
			f := strings.Fields(l)
			if len(f) == 5 && f[0] == "compile" && f[1] == "main" {
				wall, cpu = f[2], f[3]
				rss, _ = strconv.ParseInt(f[4], 10, 64)
			}
		}
		srcFuncs := len(regexp.MustCompile(`(?m)^func `).FindAll(code, -1))
		t.Logf("%-16s %8d %9d %7d %8d %8s %8s %8d", name, srcFuncs, compiled, inst, fi.Size()>>20, wall, cpu, rss>>20)
	}
}
