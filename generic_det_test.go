package rugaroo

import (
	"bytes"
	"context"
	"testing"
)

// genericDetProgram names three instances of one generic superclass's subclasses only through open `X.new` locals (decision 310): refining them walks f.locals, whose map order used to order the instance requests, and with it the classes in main.go.
const genericDetProgram = `# @rbs generic E
# @rbs generic P
class Heap
  #: () -> void
  def initialize
    @items = [] #: Array[[E, P]]
  end

  #: (E, P) -> self
  def push(e, pr)
    @items << [e, pr]
    self
  end

  #: () -> Integer
  def size = @items.size
end

# @rbs generic E
# @rbs generic P
class MinQ < Heap #[E, P]
end

# @rbs generic E
# @rbs generic P
class MaxQ < Heap #[E, P]
end

# @rbs generic E
# @rbs generic P
class SimpleQ < Heap #[E, P]
end

a = MinQ.new
b = MaxQ.new
c = SimpleQ.new
a.push("a", 1)
b.push("b", 2)
c.push("c", 3)
puts a.size, b.size, c.size
`

// The same program must compile to the same bytes every time: Go's build cache and byte comparisons depend on it.
func TestGenericInstancesDeterministic(t *testing.T) {
	t.Parallel()
	var first []byte
	for i := range 30 {
		out, _, err := Compile(context.Background(), "main.rb", []byte(genericDetProgram))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = out
		} else if !bytes.Equal(out, first) {
			t.Fatalf("compile %d differs from compile 0 (%d vs %d bytes)", i, len(out), len(first))
		}
	}
}
