package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

func sigReturn(t *testing.T, c *Compiler, name string) string {
	t.Helper()
	m := c.sigMethod(&Class{RubyName: "Foo"}, name)
	if m == nil {
		return ""
	}
	return m.Overloads[0].Return.String()
}

// A reopened class keeps every sig; the user's declaration beats a vendored gem's for the same method.
func TestSigsMergeUserWins(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}
	write("a.rbs", "class Foo[E]\n  def a: () -> Integer\n  attr_reader x: Integer\n  include Comparable\nend\n")
	write("b.rbs", "class Foo\n  def b: () -> String\n  def a: () -> bool\n  prepend Enumerable[E]\nend\n")
	c := &Compiler{sigs: map[string]*rbs.ClassDecl{}, gemSigs: fstest.MapFS{
		"sig/gems/g/foo.rbs": {Data: []byte("class Foo\n  def a: () -> Symbol\n  def g: () -> Float\n  attr_reader x: Symbol\n  attr_reader y: Symbol\nend\n")},
	}}
	c.loadSigDir(dir, false)
	c.loadGemSigs()
	for name, want := range map[string]string{"a": "Integer", "b": "String", "g": "Float"} {
		if got := sigReturn(t, c, name); got != want {
			t.Errorf("Foo#%s returns %q, want %q", name, got, want)
		}
	}
	if got := c.sigAttr(&Class{RubyName: "Foo"}, "x"); got == nil || got.Type.String() != "Integer" {
		t.Errorf("attr x = %+v, want the user's Integer", got)
	}
	if got := c.sigAttr(&Class{RubyName: "Foo"}, "y"); got == nil {
		t.Error("the gem's attr y was dropped")
	}
	d := c.sigs["Foo"]
	if len(d.TypeParams) != 1 || len(d.Includes) != 2 {
		t.Errorf("type params %v, includes %v: want E and two includes", d.TypeParams, d.Includes)
	}
}

// A gem's shipped sig/ loads after the vendored sigs, leniently, its aliases followed (decision 306); generics erased only over a core @go_type (decision 312).
func TestSigsShipped(t *testing.T) {
	root := t.TempDir()
	for name, src := range map[string]string{
		"sig/foo.rbs":           "class Foo[E]\n  def a: () -> E\n  def n: () -> Integer\n  def g: () -> Integer\n  def h: (E) -> Array[E]?\n  alias b a\n  alias m n\n  alias c m\nend\n",
		"sig/bad.rbs":           "class Foo\n  def z: (((\nend\n",
		"sig/bar.rbs":           "class Bar[V] < Hash[String, V]\n  def v: () -> V\n  def k: () -> Integer\nend\n",
		"sig/_scripts/skip.rbs": "class Foo\n  def s: () -> String\nend\n",
	} {
		p := filepath.Join(root, name)
		err := os.MkdirAll(filepath.Dir(p), 0o750)
		if err == nil {
			err = os.WriteFile(p, []byte(src), 0o600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	c := &Compiler{sigs: map[string]*rbs.ClassDecl{}, classes: map[string]*Class{"Hash": {RubyName: "Hash", GoType: "*rbHash"}}, warned: map[string]bool{}, env: envConfig{GemWarnings: true},
		gemDirs: []string{filepath.Join(root, "lib")}, gemSigs: fstest.MapFS{
			"sig/gems/g/foo.rbs": {Data: []byte("class Foo\n  def g: () -> Float\nend\n")},
		}}
	c.loadSigs(nil)
	for name, want := range map[string]string{"a": "E", "b": "E", "h": "Array[E]?", "n": "Integer", "m": "Integer", "c": "Integer", "g": "Float", "s": ""} {
		if got := sigReturn(t, c, name); got != want {
			t.Errorf("Foo#%s returns %q, want %q", name, got, want)
		}
	}
	if tps := c.sigs["Foo"].TypeParams; len(tps) != 1 { // a generic struct class keeps them (decision 312)
		t.Errorf("type params %v dropped", tps)
	}
	if bar := c.sigs["Bar"]; len(bar.TypeParams) != 0 || len(bar.Methods) != 1 || bar.Methods[0].Name != "k" { // over a core @go_type: erased
		t.Errorf("Bar = %+v, want V and v dropped", bar)
	}
	if len(c.Warnings) != 2 {
		t.Errorf("warnings = %q, want bad.rbs skipped and Bar[V] erased", c.Warnings)
	}
}

// `A & B` resolves to one operand: a class over a module, a module over an RBS interface (decision 315).
func TestResolveIntersection(t *testing.T) {
	c := &Compiler{classes: map[string]*Class{"Node": {RubyName: "Node"}, "Comparable": {RubyName: "Comparable", IsModule: true}}}
	for in, want := range map[string]string{"Node & _Node": "Node", "_Node & Node": "Node", "Comparable & Node": "Node", "Comparable & _Each": "Comparable"} {
		ty, err := rbs.ParseType(in)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := c.resolveType(ty, typeScope{}).(TClass)
		if got.C == nil || got.C.RubyName != want {
			t.Errorf("%s resolved to %+v, want %s", in, got, want)
		}
	}
}

// An `rbs collection install` directory beside the program types a gem's methods, the highest version of each gem,
// after the vendored sig/gems and before the gem's own sig/ (decision 457, #105).
func TestRBSCollection(t *testing.T) {
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("boot needs ruby")
	}
	dir := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		werr := os.MkdirAll(filepath.Dir(p), 0o750)
		if werr == nil {
			werr = os.WriteFile(p, []byte(src), 0o600)
		}
		if werr != nil {
			t.Fatal(werr)
		}
	}
	write("lib/colgem.rb", "class Colgem\n  def scale(n) = n * 2\nend\n")
	write(".gem_rbs_collection/colgem/1.9/colgem.rbs", "class Colgem\n  def scale: (Integer) -> Integer\nend\n")
	write(".gem_rbs_collection/colgem/1.10/colgem.rbs", "class Colgem\n  def scale: (Float) -> Float\nend\n")
	app := "# load_path: lib\nrequire \"colgem\"\nv = [1.5].first #: untyped\np Colgem.new.scale(v)\n"
	write("app.rb", app)
	fsys := os.DirFS("../..")
	out, _, err := CompileWithBoot(t.Context(), fsys, fsys, []Source{{Name: "app.rb", Src: []byte(app), Path: filepath.Join(dir, "app.rb")}}, []string{filepath.Join(dir, "lib")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Colgem_Scale(self *Colgem, n Float) Float") {
		t.Errorf("the collection's 1.10 sig (not 1.9) did not type Colgem#scale")
	}
}
