package compiler

import (
	"slices"
	"strings"
	"testing"
)

// MRI has one @v: a module reading what the includer's superclass assigns in any write form must read it by name (decision 324).
func TestIncluderClassWritesAnyForm(t *testing.T) {
	for name, write := range map[string]string{
		"plain":  "@v = 5",
		"or":     "@v ||= 5",
		"and":    "@v &&= 5",
		"target": "@v, @w = 5, 6",
	} {
		t.Run(name, func(t *testing.T) {
			gem := "class Seg\n  def init\n    " + write + "\n  end\nend\nmodule Shower\n  def show = @v\nend\nclass Holder < Seg\n  include Shower\nend\n"
			out, err := compileGemApp(t, map[string]string{"thing.rb": gem}, "require \"thing\"\nh = Holder.new\nh.init\nputs h.show\n")
			if err != nil {
				t.Fatal(err)
			}
			_, body, ok := strings.Cut(out, "func Shower_Show")
			if !ok {
				t.Fatal("Shower_Show not emitted")
			}
			body, _, _ = strings.Cut(body, "\n}\n")
			if !strings.Contains(body, `rbIvarGet(self, Symbol("@v"))`) {
				t.Errorf("Shower_Show does not read the includer's @v by name:\n%s", body)
			}
		})
	}
}

// Values read back out of storage (kramdown's env save/restore) leave @tree to its typed writes; an untyped return (liquid's Cycle @name) still declares untyped (decision 392).
func TestStoredIvarTyped(t *testing.T) {
	gem := `class Node
  attr_reader :name, :kids
  def initialize(name)
    @name = name
    @kids = []
  end
end

class Parse
  def self.it(s) = s.start_with?("'") ? s[1..] : Node.new(s)
end

class Tree
  def initialize
    reset_env
  end

  def reset_env(opts = {})
    opts = {stack: []}.merge(opts)
    @tree = opts[:tree]
    @stack = opts[:stack]
  end

  def save_env = [@tree, @stack]

  def restore_env(env)
    @tree, @stack = *env
  end

  def open(el)
    @stack.push([@tree, 1])
    @tree = el
  end

  def close
    @tree, _ = *@stack.pop
  end

  def nested(el)
    env = save_env
    reset_env
    open(el)
    out = @tree.name
    restore_env(env)
    out
  end

  def label(s)
    @label = Parse.it(s)
    @label = s.upcase if s.empty?
    @label
  end
end
`
	app := "require \"tree\"\nt = Tree.new\nt.open(Node.new(\"a\"))\nputs t.nested(Node.new(\"b\"))\nt.close\np t.label(\"'x\")\n"
	out, err := compileGemApp(t, map[string]string{"tree.rb": gem}, app)
	if err != nil {
		t.Fatal(err)
	}
	_, decl, ok := strings.Cut(out, "type Tree struct {")
	if !ok {
		t.Fatal("Tree not emitted")
	}
	decl, _, _ = strings.Cut(decl, "\n}\n")
	fields := strings.Join(strings.Fields(decl), " ")
	for _, want := range []string{"tree *Node", "label any"} {
		if !strings.Contains(fields, want) {
			t.Errorf("Tree's fields lack %q:\n%s", want, decl)
		}
	}
}

// Counters `reset` sets and attr writers bump are Integer; a storage value initialize stores stays nilable (decision 394).
func TestWriterIvarTyped(t *testing.T) {
	gem := `class Limits
  attr_accessor :length, :score
  attr_reader :cur

  def initialize(max, opts = {})
    @max = max
    reset(opts)
  end

  def reached? = @length > @max || @score > @max

  def reset(opts)
    @length = @score = 0
    @cur = opts[:cur]
  end

  def enter(s) = @cur = s
end

class Body
  def initialize(nodes)
    @nodes = nodes
  end

  def render(limits, s)
    limits.score += @nodes.length
    limits.length += s.length
    limits.reached?
  end
end
`
	app := "require \"limits\"\nl = Limits.new(9)\nb = Body.new([1, 2])\nb.render(l, \"abc\")\np b.render(l, \"defghij\"), l.length, l.score, l.cur\nl.enter(\"x\")\np l.cur\n"
	out, err := compileGemApp(t, map[string]string{"limits.rb": gem}, app)
	if err != nil {
		t.Fatal(err)
	}
	_, decl, ok := strings.Cut(out, "type Limits struct {")
	if !ok {
		t.Fatal("Limits not emitted")
	}
	decl, _, _ = strings.Cut(decl, "\n}\n")
	fields := strings.Join(strings.Fields(decl), " ")
	for _, want := range []string{"length Integer", "score Integer"} {
		if !strings.Contains(fields, want) {
			t.Errorf("Limits' fields lack %q:\n%s", want, decl)
		}
	}
	if strings.Contains(fields, "cur String") {
		t.Errorf("Limits' @cur lost its nil:\n%s", decl)
	}
}

// An unannotated attr writer nothing calls takes its ivar's type from the ivar's other writes (decision 445, #152);
// one a computed send may reach keeps today's untyped ivar.
func TestUncalledWriterIvarTyped(t *testing.T) {
	src := `class Cell
  attr_accessor :value
  def initialize = @value = nil
  def set(v) = @value = v
  def get = @value
end

def render_one(v)
  v.is_a?(String) ? "s" : "o"
end

c = Cell.new
c.set("hello")
c.set(42)
p render_one(c.get)
`
	errText, ws := compileSrc(t, src)
	if errText != "" {
		t.Fatal(errText)
	}
	for _, w := range ws {
		if strings.Contains(w, "untyped") || strings.Contains(w, "dynamic call") {
			t.Errorf("unexpected warning: %s", w)
		}
	}
	_, ws = compileSrc(t, src+"c.send(ARGV.first || \"value=\", 1)\n")
	if !slices.ContainsFunc(ws, func(w string) bool { return strings.Contains(w, "render_one's parameter v is untyped") }) {
		t.Errorf("a computed send reaches value=, so @value stays untyped; warnings: %q", ws)
	}
}
