package rugaroo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUntypedReport: RUGAROO_UNTYPED_REPORT counts the dynamic calls the warnings name, groups them by root, and gives an inferred parameter the rule and uses that left it untyped; in both inference lanes.
func TestUntypedReport(t *testing.T) {
	main := filepath.Join("testdata", "run", "gem_inference.rb")
	src, err := os.ReadFile(main) //nolint:gosec // our fixture
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range []string{"0", "1"} {
		t.Run("unknown="+lane, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.txt")
			t.Setenv("RUGAROO_UNTYPED_REPORT", path)
			t.Setenv("RUGAROO_GEM_WARNINGS", "1")
			t.Setenv("RUGAROO_INFER_UNKNOWN", lane)
			_, warnings, err := gemCompile(filepath.Dir(main), src)(t.Context(), main, src, filepath.Join("testdata", "run", "gem_inference", "lib"))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			for _, w := range warnings {
				if strings.Contains(w, "dynamic call") {
					calls++
				}
				if strings.Contains(w, "\x00") {
					t.Errorf("a report entry among the warnings: %q", w)
				}
			}
			data, err := os.ReadFile(path) //nolint:gosec // the test's own temp file
			if err != nil {
				t.Fatal(err)
			}
			report := string(data)
			if calls == 0 || !strings.Contains(report, fmt.Sprintf(": %d dynamic call sites, ", calls)) {
				t.Errorf("the report does not count the %d dynamic calls the warnings name:\n%s", calls, report)
			}
			sites := 0
			for line := range strings.Lines(report) {
				var rank, n int
				_, err := fmt.Sscanf(strings.TrimSpace(line), "%d. %d sites:", &rank, &n)
				if err == nil {
					sites += n
				}
			}
			if sites != calls {
				t.Errorf("the roots hold %d sites, the warnings %d", sites, calls)
			}
			if lane == "0" && !strings.Contains(report, "param CycStrings#advance(p) at testdata/run/gem_inference/lib/cyc_gem.rb:18\n      why: no use carries a type") {
				t.Errorf("no reason for CycStrings#advance(p), untyped in the default lane:\n%s", report)
			}
		})
	}
}
