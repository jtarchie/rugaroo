package compiler

import (
	"context"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// A rejected func becomes a raise in the Go text, and an import only stubbed bodies used is dropped, one other code still uses is kept (decision 303).
func TestStubGoFuncs(t *testing.T) {
	code := []byte(`package main

import (
	"fmt"
	"strings"
)

type String string

func Ref[T any](v T) *T { return &v }

func NewNotImplementedError(*String) any { return nil }

type X[T any] struct{}

func (self *X[T]) Bad() int {
	return strings.ToUpper(1)
}

func Good() int { return Bad2() + 1 }

func Bad2() int { var s string = 1; return len(s + fmt.Sprint()) }

func main() { fmt.Println(Good(), (&X[int]{}).Bad()) }
`)
	stubbed := map[string]bool{}
	code = stubRejected(context.Background(), code, func(fn, _ string) (string, bool) {
		stubbed[fn] = true
		return "rugaroo: " + fn + " does not compile", fn != ""
	})
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&types.Config{Importer: importer.Default()}).Check("main", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("still rejected: %v\n%s", err, code)
	}
	if !stubbed["X.Bad"] || !stubbed["Bad2"] || stubbed["Good"] || strings.Contains(string(code), `"strings"`) {
		t.Fatalf("stubbed %v\n%s", stubbed, code)
	}
}
