package compiler

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// treeSums is RUGAROO_TREE_CHECK's picture of the parsed files right after the require scan: one sum per def (and one
// for the rest of the file), over each node's type and place. A pass that rewrites a tree in place shows as a sum that
// differs when the pass ends, which is what stands between one compile's passes and sharing the trees (decision 478).
type treeSums map[*File]map[int]int

// nodeSum is a tree as one number: each node's type and place, in order. With defs, a def is summed into defs by
// its place and left out of the answer.
func nodeSum(n parser.Node, defs map[int]int) int {
	sum := 1469598103
	mix := func(v int) { sum = (sum ^ v) * 1099511 }
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if d, ok := n.(*parser.DefNode); ok && defs != nil {
			at := d.Location.StartOffset
			defs[at] = nodeSum(d.Body, nil)
			if d.Parameters != nil {
				defs[at] ^= nodeSum(d.Parameters, nil)
			}
			return
		}
		loc := n.GetLocation()
		mix(nodeKind(n))
		mix(loc.StartOffset)
		mix(loc.Length)
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
		mix(0)
	}
	if n != nil {
		walk(n)
	}
	return sum
}

var nodeKinds sync.Map // reflect.Type -> int

// nodeKind numbers a node's Go type.
func nodeKind(n parser.Node) int {
	t := reflect.TypeOf(n)
	if k, ok := nodeKinds.Load(t); ok {
		return k.(int) //nolint:forcetypeassert // only ints are stored
	}
	h := 0
	for _, b := range []byte(t.String()) {
		h = h*31 + int(b)
	}
	nodeKinds.Store(t, h)
	return h
}

func fileSums(f *File) map[int]int {
	defs := map[int]int{}
	defs[-1] = nodeSum(f.Root, defs)
	return defs
}

func (c *Compiler) takeTreeSums() treeSums {
	if !c.env.TreeCheck {
		return nil
	}
	out := treeSums{}
	for _, f := range c.parsed {
		out[f] = fileSums(f)
	}
	return out
}

// report prints each def whose tree is not what it was when before was taken.
func (before treeSums) report() {
	var lines []string
	for f, was := range before {
		now := fileSums(f)
		for at, sum := range was {
			if now[at] == sum {
				continue
			}
			if at < 0 {
				lines = append(lines, fmt.Sprintf("rugaroo: tree changed: %s: outside a def", f.Name))
				continue
			}
			lines = append(lines, fmt.Sprintf("rugaroo: tree changed: %s:%d", f.Name, f.line(at)))
		}
	}
	slices.Sort(lines)
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
}

// fileCache keeps one compile's parsed source files for its next rounds and passes (decision 479): a file is parsed
// through prism's wasm once, not once per pass. A pass that rewrote a file's tree in place (a `def m(...)`'s forwarded
// arguments, filled in at link time) loses it at the pass's end, so the next pass parses it fresh, as every pass used to.
type fileCache struct {
	files map[fileKey]*cachedFile
}

type fileKey struct{ name, src string }

type cachedFile struct {
	f   *File
	sum int
}

// parseSource is parseFile for a program's source or gem file, answered from the compile's cache when it holds it.
func (c *Compiler) parseSource(ctx context.Context, name string, src []byte) (*File, error) {
	if c.srcCache == nil || c.env.NoFileReuse { // no cache: a scan outside a compile (gemRequires)
		return parseFile(ctx, c.parser, name, src, originSource)
	}
	k := fileKey{name, string(src)}
	if cf := c.srcCache.files[k]; cf != nil {
		c.filesUsed = append(c.filesUsed, k)
		return cf.f, nil
	}
	f, err := parseFile(ctx, c.parser, name, src, originSource)
	if err != nil {
		return nil, err
	}
	c.srcCache.files[k] = &cachedFile{f: f, sum: nodeSum(f.Root, nil)}
	c.filesUsed = append(c.filesUsed, k)
	return f, nil
}

// passTrees starts a pass's watch over its parsed files, and answers what ends it: RUGAROO_TREE_CHECK's report, and
// the cache losing the files whose trees are not what the parser gave.
func (c *Compiler) passTrees() func() {
	sums := c.takeTreeSums()
	return func() {
		if sums != nil {
			sums.report()
		}
		c.dropRewritten()
	}
}

func (c *Compiler) dropRewritten() {
	for _, k := range c.filesUsed {
		if cf := c.srcCache.files[k]; cf != nil && nodeSum(cf.f.Root, nil) != cf.sum {
			delete(c.srcCache.files, k)
		}
	}
}

// fileCache is the compile's parsed-file cache, made at its first pass.
func (o *options) fileCache() *fileCache {
	if o.files == nil {
		o.files = &fileCache{files: map[fileKey]*cachedFile{}}
	}
	return o.files
}
