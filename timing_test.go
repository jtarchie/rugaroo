package rugaroo

import (
	"fmt"
	"os"
	"strings"
	"sync"
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
