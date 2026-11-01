package compiler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// compileSrc compiles one main.rb against the repo's embedded prelude; the error is "" on success.
func compileSrc(t *testing.T, src string) (errText string, warnings []string) {
	t.Helper()
	_, ws, err := CompileWithWarnings(context.Background(), os.DirFS("../.."), "main.rb", []byte(src))
	if err != nil {
		errText = err.Error()
	}
	return errText, ws
}

// A warning prints once however many passes, probes and retries compile the line (it bypassed the dedupe).
func TestWarningsAreDeduped(t *testing.T) {
	src := `def f(xs) #: (Array[Float]) -> Integer
  n = 0
  xs.each do |x|
    n += 1 if x
    n += (x ? 1 : 2)
  end
  while n < 3
    n += 1 if 1.5
  end
  n
end
puts f([1.0])
`
	_, ws := compileSrc(t, src)
	seen := map[string]int{}
	for _, w := range ws {
		seen[w]++
	}
	if len(seen) == 0 {
		t.Fatal("expected always-true warnings")
	}
	for w, n := range seen {
		if n != 1 {
			t.Errorf("warning printed %d times: %q", n, w)
		}
	}
}

// A gem method named only by a string literal send/respond_to?/method is reached as a symbol literal's is (#101).
func TestGemMethodNamedByStringLiteralIsKept(t *testing.T) {
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("boot needs ruby")
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	err = os.MkdirAll(lib, 0o750)
	if err != nil {
		t.Fatal(err)
	}
	gem := "class Thing\n  def by_send = 1\n  def by_public_send = 2\n  def by_method = 3\n  def by_respond = 4\n  def never_named = 5\nend\n"
	err = os.WriteFile(filepath.Join(lib, "thing.rb"), []byte(gem), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	app := "# load_path: lib\nrequire \"thing\"\no = Thing.new\np o.send(\"by_send\"), o.public_send(\"by_public_send\"), o.method(\"by_method\").call, o.respond_to?(\"by_respond\")\n"
	appPath := filepath.Join(dir, "app.rb")
	err = os.WriteFile(appPath, []byte(app), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fsys := os.DirFS("../..")
	out, _, err := CompileWithBoot(context.Background(), fsys, fsys, []Source{{Name: "app.rb", Src: []byte(app), Path: appPath}}, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Boolean(true))\n}") { // respond_to? folds to a constant: the last argument of p
		t.Error("respond_to?(\"by_respond\") is not true for a gem method a string literal names")
	}
	for _, name := range []string{"by_send", "by_public_send", "by_method"} { // by_respond has no Go function: it folds
		if !strings.Contains(string(out), "Thing_"+goMethodName(name)) {
			t.Errorf("gem method %s pruned although a string literal names it", name)
		}
	}
	if strings.Contains(string(out), "Thing_"+goMethodName("never_named")) {
		t.Error("never_named was kept: the test cannot tell pruning from keeping")
	}
}

// Regexp.new of a literal RE2 rejects needs the alternate engine, however Regexp is spelled (::Regexp too).
func TestRegexpNewNotesAlternateEngine(t *testing.T) {
	for name, src := range map[string]string{
		"plain":    "puts Regexp.new(\"(?<=a)b\").match?(\"ab\")\n",
		"rooted":   "puts ::Regexp.new(\"(?<=a)b\").match?(\"ab\")\n",
		"extended": "puts Regexp.new(\"(?<=a) b\", Regexp::EXTENDED).match?(\"ab\")\n",
	} {
		t.Run(name, func(t *testing.T) {
			out, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte(src)}}, &options{})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "var rxAltReady") {
				t.Error("a lookbehind pattern built by Regexp.new did not get the regexp2 engine")
			}
		})
	}
	out, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte("puts ::Regexp.new(\"ab\").match?(\"ab\")\n")}}, &options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "var rxAltReady") {
		t.Error("an RE2 pattern must not pull in regexp2")
	}
}

// Several unknown keywords: the diagnostic names the first in source order, every time (it ranged over a map).
func TestUnknownKeywordIsFirstInSourceOrder(t *testing.T) {
	progs := map[string]string{
		"data_new":   "Coord = Data.define(:lat) #: [Float]\nCoord.new(lat: 1.0, zz3: 1, aa2: 2, mm1: 3)\n",
		"struct_new": "Pt = Struct.new(:x) #: [Integer]\nPt.new(x: 1, zz3: 1, aa2: 2, mm1: 3)\n",
		"data_with":  "Coord = Data.define(:lat) #: [Float]\nc = Coord.new(1.0)\nc.with(zz3: 1, aa2: 2, mm1: 3)\n",
	}
	for name, src := range progs {
		t.Run(name, func(t *testing.T) {
			for range 6 { // random map order: a wrong pick of 1 of 3 keys survives 6 compiles with p=(1/3)^6
				got, _ := compileSrc(t, src)
				if !strings.Contains(got, "unknown keyword: :zz3") {
					t.Fatalf("want the first unknown keyword :zz3, got %q", got)
				}
			}
		})
	}
}
