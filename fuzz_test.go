package rugaroo

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jtarchie/rugaroo/internal/compiler"
)

// FuzzCompile: any input either compiles or fails with a compile error; an internal panic is a bug, whether compileSafe caught it or the compiler returned it as compiler.ErrInternal (decision 263). Under -fuzz it is seeded from every example and testdata/run file; a plain `go test` skips them (each ran through TestExamples/TestRun already, and as seeds they cost ~58 s serially, decision 88).
// "generated Go does not parse" counts too: compiler.go returns it as a plain error, so it would otherwise pass silently.
// A crash the fuzzer finds is kept in testdata/fuzz/FuzzCompile, which every run replays.
func FuzzCompile(f *testing.F) {
	if fz := flag.Lookup("test.fuzz"); fz != nil && fz.Value.String() != "" {
		seeds, _ := filepath.Glob("testdata/run/*.rb")
		examples, _ := filepath.Glob("examples/*/main.rb")
		for _, p := range append(seeds, examples...) {
			src, err := os.ReadFile(p) //nolint:gosec // repo path
			if err == nil {
				f.Add(src)
			}
		}
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		_, _, err := compileSafe("main.rb", src)
		if err != nil && (errors.Is(err, compiler.ErrInternal) || strings.HasPrefix(err.Error(), "compiler panic") || strings.Contains(err.Error(), "generated Go does not parse")) {
			t.Fatal(err)
		}
	})
}
