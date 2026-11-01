package rugaroo

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// timingLog is the file RUGAROO_TEST_TIMING names: one tab-separated line per timed step of the harness
// (kind, test name, seconds, then step-specific fields), for scripts/test-time to turn into a table. Off when unset.
var timingLog = os.Getenv("RUGAROO_TEST_TIMING")

var (
	timingMu    sync.Mutex
	timingStart = time.Now()
)

// timed appends one line to the timing log; a no-op unless RUGAROO_TEST_TIMING is set.
func timed(kind, name string, d time.Duration, fields ...any) {
	if timingLog == "" {
		return
	}
	parts := []string{kind, name, fmt.Sprintf("%.3f", d.Seconds()), fmt.Sprintf("@%.1f", time.Since(timingStart).Seconds())} // the @ column: when the step ended, seconds since the test binary started
	for _, f := range fields {
		parts = append(parts, fmt.Sprint(f))
	}
	timingMu.Lock()
	defer timingMu.Unlock()
	f, err := os.OpenFile(timingLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // a developer-chosen path
	if err != nil {
		return
	}
	_, _ = f.WriteString(strings.Join(parts, "\t") + "\n")
	_ = f.Close()
}

func TestStableLines(t *testing.T) {
	t.Parallel()
	in := "//line prelude/a.rb:40\nx\ny\n//line prelude/b.rb:7\nz\n//line prelude/a.rb:90\nw\n//line main.rb:3\nq\n"
	want := "//line prelude/a.rb:1\nx\ny\n//line prelude/b.rb:1\nz\n//line prelude/a.rb:3\nw\n//line main.rb:3\nq\n"
	if got := string(stableLines([]byte(in), false)); got != want {
		t.Errorf("consecutive:\n%s\nwant:\n%s", got, want)
	}
	pinned := "//line prelude/a.rb:1\nx\ny\n//line prelude/b.rb:1\nz\n//line prelude/a.rb:1\nw\n//line main.rb:3\nq\n"
	if got := string(stableLines([]byte(in), true)); got != pinned {
		t.Errorf("pinned:\n%s\nwant:\n%s", got, pinned)
	}
}
