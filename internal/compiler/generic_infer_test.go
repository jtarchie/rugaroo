package compiler

import (
	"context"
	"os"
	"strings"
	"testing"
)

// RUGAROO_INFER_GENERIC types a user parameter whose calls disagree with a type parameter, only for a program that does
// not compile without it (decision 503); one that compiles is the same Go either way.
func TestInferGeneric(t *testing.T) {
	failing := "def id(x)\n  x\nend\na = id(1)\nb = id(\"a\")\np a + 1, b.upcase\n"
	compiling := "def pick(c)\n  c ? 1 : \"s\"\nend\np pick(true).to_s\n"
	gen := func(src string) (string, error) {
		out, err := Compile(context.Background(), os.DirFS("../.."), "main.rb", []byte(src))
		return string(out), err
	}
	t.Setenv("RUGAROO_INFER_GENERIC", "")
	_, err := gen(failing)
	if err == nil {
		t.Fatal("without the option the disagreeing calls should not compile")
	}
	before, err2 := gen(compiling)
	err = err2
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUGAROO_INFER_GENERIC", "1")
	out, err := gen(failing)
	if err != nil {
		t.Fatalf("with the option: %v", err)
	}
	if !strings.Contains(out, "func rb_Id[T0 comparable](x T0) T0") || !strings.Contains(out, "rb_Id(Integer(1))") {
		t.Errorf("id is not one generic func typed at each call:\n%s", grepLines(out, "rb_Id"))
	}
	after, err := gen(compiling)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Error("a program that compiles without the option generated other Go with it")
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
