package compiler

import (
	"go/parser"
	"go/token"
	"testing"
)

// TestKeptFuncMade: rbIsProc's cases are the *func types kept code makes, not every func shape it writes (decision 431).
func TestKeptFuncMade(t *testing.T) {
	src := `package main

func Ref[T any](v T) *T { return &v }

func Wrap[F any](f F) any { return Ref(f) }

func set(a, b, c any) any { return nil }

func dead() any { return Ref(func(s string) bool { return true }) }

func keep(blk func(int) string) any { return Ref(blk) }

func main() {
	_ = Ref(func(x any) any { return x })
	_ = keep
	_ = set
	_ = Wrap(func(n int, m int) int { return n })
}
`
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := newPruner(nil)
	p.add(f.Decls)
	p.run()
	for s, want := range map[string]bool{
		"func(any) any":           true,  // Ref(func literal)
		"func(int) string":        true,  // Ref(blk) of a func-typed parameter
		"func(int, int) int":      true,  // made inside Wrap through Ref, from Wrap's argument
		"func(any, any, any) any": false, // only a signature: nothing points at set
		"func(string) bool":       false, // made in dead code only
	} {
		if got := p.keptFunc(s); got != want {
			t.Errorf("keptFunc(%q) = %v, want %v (valve %q)", s, got, want, p.procValve)
		}
	}
}
