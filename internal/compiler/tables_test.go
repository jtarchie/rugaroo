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
