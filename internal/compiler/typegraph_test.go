package compiler

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// The type-graph spike types a small program's unannotated parameters and returns as the compiler does.
func TestTypeGraphAgrees(t *testing.T) {
	src := `class Box
  def initialize(v)
    @v = v
  end

  def get = @v
end

def twice(n)
  n * 2
end

def label(b)
  "box #{b.get}"
end

p twice(21), label(Box.new(1))
`
	t.Setenv("RUGAROO_TYPEGRAPH", "1")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	_, cerr := Compile(context.Background(), os.DirFS("../.."), "main.rb", []byte(src))
	os.Stderr = saved
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if cerr != nil {
		t.Fatal(cerr)
	}
	report := string(out)
	for _, want := range []string{"params 3 of 3 same", "returns 3 of 3 same"} {
		if !strings.Contains(report, want) {
			t.Errorf("want %q in the report:\n%s", want, report)
		}
	}
}
