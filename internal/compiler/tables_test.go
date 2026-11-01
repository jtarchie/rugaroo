package compiler

import (
	"os"
	"strings"
	"testing"
)

func compileOne(t *testing.T, src string) string {
	t.Helper()
	out, _, err := CompileFilesWithWarnings(t.Context(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte(src)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// Every tuple arity is emitted and the pruner keeps what kept code names (decision 432).
func TestTuplesByPruner(t *testing.T) {
	out := compileOne(t, "# rbs_inline: enabled\n\n#: () -> [Integer, String]\ndef pair = [1, \"a\"]\n\np pair\n")
	if !strings.Contains(out, "type Tuple2[") {
		t.Error("Tuple2 missing")
	}
	if strings.Contains(out, "type Tuple5[") {
		t.Error("Tuple5 kept though nothing names it")
	}
}

// The regexp2 engine is in a program when kept code names it: an alt pattern's var, or a body building one at run
// time; pattern vars are named by pattern (decision 432).
func TestRxAltByPruner(t *testing.T) {
	plain := compileOne(t, "p(\"ab\" =~ /b/)\n")
	if strings.Contains(plain, "rxAltReady") {
		t.Error("engine kept for an RE2 pattern")
	}
	if !strings.Contains(plain, "var rbRe_") {
		t.Error("pattern var not named by its pattern")
	}
	look := compileOne(t, "p(\"ab\" =~ /(?<=a)b/)\n")
	if !strings.Contains(look, "rbRxAlt(rxAltReady, rbRegexpNew(") || !strings.Contains(look, "var rxAltReady") {
		t.Error("lookbehind pattern without the engine")
	}
	dyn := compileOne(t, "s = gets.to_s\np(Regexp.new(s).match?(\"x\"))\n")
	if !strings.Contains(dyn, "var rxAltReady") {
		t.Error("Regexp.new of a run-time string without the engine")
	}
	dead := compileOne(t, "def never = (\"ab\" =~ /(?<=a)b/)\np 1\n")
	if strings.Contains(dead, "rxAltReady") {
		t.Error("engine kept for a pattern only dead code names")
	}
}
