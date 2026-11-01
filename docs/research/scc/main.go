package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
)

var nerr int

func main() {
	defer func() { fmt.Println("type errors:", nerr) }()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, os.Args[1], nil, 0)
	if err != nil {
		panic(err)
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(e error) {
		nerr++
		if nerr <= 5 {
			fmt.Fprintln(os.Stderr, e)
		}
	}}
	pkg, _ := conf.Check("main", fset, []*ast.File{f}, info)
	_ = pkg
	// node = top-level func/method/type/var object
	id := map[types.Object]int{}
	var objs []types.Object
	node := func(o types.Object) (int, bool) {
		if fn, ok := o.(*types.Func); ok && os.Getenv("CONTRACT") != "" {
			if rn := recvNamed(fn); rn != nil {
				o = rn.Obj()
			}
		}
		if o == nil || o.Pkg() == nil || o.Pkg().Path() != "main" {
			return 0, false
		}
		if v, ok := o.(*types.Var); ok && (v.IsField() || v.Parent() != nil && v.Parent() != v.Pkg().Scope()) {
			return 0, false
		}
		if i, ok := id[o]; ok {
			return i, true
		}
		i := len(objs)
		id[o] = i
		objs = append(objs, o)
		return i, true
	}
	var edges = map[int]map[int]bool{}
	span := map[int]int{}
	origin := map[int]string{}
	classify := func(fn string) string {
		if strings.HasPrefix(fn, "prelude") {
			return "prelude"
		}
		return "other"
	}
	add := func(a, b int) {
		if os.Getenv("CUT") != "" && strings.HasPrefix(objs[b].Name(), os.Getenv("CUT")) && !strings.HasPrefix(objs[a].Name(), os.Getenv("CUT")) {
			return
		}
		if a == b {
			return
		}
		if edges[a] == nil {
			edges[a] = map[int]bool{}
		}
		edges[a][b] = true
	}
	for _, d := range f.Decls {
		var owners []types.Object
		switch d := d.(type) {
		case *ast.FuncDecl:
			owners = append(owners, info.Defs[d.Name])
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					owners = append(owners, info.Defs[s.Name])
				case *ast.ValueSpec:
					for _, n := range s.Names {
						owners = append(owners, info.Defs[n])
					}
				}
			}
		}
		for _, o := range owners {
			a, ok := node(o)
			if !ok {
				continue
			}
			// method also depends on its receiver type, and a type on its methods? (type -> methods is not a dep; method -> type is)
			origin[a] = classify(fset.Position(d.Pos()).Filename)
			span[a] += fset.Position(d.End()).Line - fset.Position(d.Pos()).Line + 1
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil {
				if fn, ok := o.(*types.Func); ok {
					if rn := recvNamed(fn); rn != nil {
						if b, ok := node(rn.Obj()); ok {
							add(a, b)
						}
					}
				}
			}
			ast.Inspect(d, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Ident:
					if u := info.Uses[n]; u != nil {
						if b, ok := node(u); ok {
							add(a, b)
						}
					}
				case *ast.SelectorExpr:
					if s := info.Selections[n]; s != nil {
						if b, ok := node(s.Obj()); ok {
							add(a, b)
						}
					}
				}
				return true
			})
		}
	}
	if os.Getenv("ORIGIN") != "" {
		cnt := map[string]int{}
		lines := map[string]int{}
		for i := range objs {
			cnt[origin[i]]++
			lines[origin[i]] += span[i]
		}
		fmt.Println("nodes by origin", cnt, "lines", lines)
		up := map[string]int{}
		for a, m := range edges {
			if origin[a] != "prelude" {
				continue
			}
			for b := range m {
				if origin[b] == "other" {
					k := fmt.Sprintf("%T", objs[b])
					if strings.HasPrefix(objs[b].Name(), "rbDyn") {
						k = "rbDyn"
					} else if strings.HasPrefix(objs[b].Name(), "Dyn") {
						k = "Dyn"
					}
					up[k]++
				}
			}
		}
		fmt.Println("edges prelude -> non-prelude by target kind:", up)
		srcs := map[string]int{}
		for a, m := range edges {
			if origin[a] != "prelude" {
				continue
			}
			for b := range m {
				if origin[b] == "other" {
					srcs[objs[a].Name()]++
				}
			}
		}
		tg := map[string]int{}
		for a, m := range edges {
			if origin[a] == "prelude" {
				for b := range m {
					if origin[b] == "other" {
						tg[objs[b].Name()]++
					}
				}
			}
		}
		type kv struct {
			k string
			v int
		}
		var l []kv
		for k, v := range tg {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
		fmt.Println("distinct upward targets:", len(tg))
		for _, e := range l[:min(25, len(l))] {
			fmt.Println("  ", e.v, e.k)
		}
		fmt.Println("distinct prelude sources with an upward edge:", len(srcs))
		var pf int
		for i, o := range objs {
			if fn, ok := o.(*types.Func); ok && origin[i] != "prelude" {
				if rn := recvNamed(fn); rn != nil {
					if j, ok := id[rn.Obj()]; ok && origin[j] == "prelude" {
						pf++
					}
				}
			}
		}
		fmt.Println("non-prelude methods on prelude-origin types:", pf)
	}
	// make sure every top-level obj has a node
	n := len(objs)
	// Tarjan iterative
	idx := make([]int, n)
	low := make([]int, n)
	on := make([]bool, n)
	for i := range idx {
		idx[i] = -1
	}
	comp := make([]int, n)
	var stack []int
	counter, ncomp := 0, 0
	type fr struct {
		v    int
		nbrs []int
		i    int
	}
	nb := func(v int) []int {
		var r []int
		for w := range edges[v] {
			r = append(r, w)
		}
		sort.Ints(r)
		return r
	}
	for s := 0; s < n; s++ {
		if idx[s] != -1 {
			continue
		}
		cs := []fr{{s, nb(s), 0}}
		idx[s], low[s] = counter, counter
		counter++
		stack = append(stack, s)
		on[s] = true
		for len(cs) > 0 {
			t := &cs[len(cs)-1]
			if t.i < len(t.nbrs) {
				w := t.nbrs[t.i]
				t.i++
				if idx[w] == -1 {
					idx[w], low[w] = counter, counter
					counter++
					stack = append(stack, w)
					on[w] = true
					cs = append(cs, fr{w, nb(w), 0})
				} else if on[w] && idx[w] < low[t.v] {
					low[t.v] = idx[w]
				}
				continue
			}
			v := t.v
			cs = cs[:len(cs)-1]
			if len(cs) > 0 {
				p := cs[len(cs)-1].v
				if low[v] < low[p] {
					low[p] = low[v]
				}
			}
			if low[v] == idx[v] {
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					on[w] = false
					comp[w] = ncomp
					if w == v {
						break
					}
				}
				ncomp++
			}
		}
	}
	size := make([]int, ncomp)
	for _, c := range comp {
		size[c]++
	}
	var sz []int
	for _, s := range size {
		sz = append(sz, s)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sz)))
	fmt.Printf("nodes %d, SCCs %d, largest SCCs %v\n", n, ncomp, sz[:min(8, len(sz))])
	// what is in the biggest SCC: by kind and name prefix
	big := 0
	for c, s := range size {
		if s == sz[0] {
			big = c
		}
	}
	inBig, total := 0, 0
	for i := range objs {
		total += span[i]
		if comp[i] == big {
			inBig += span[i]
		}
	}
	fmt.Printf("lines in biggest SCC: %d of %d (%.0f%%)\n", inBig, total, 100*float64(inBig)/float64(total))
	kinds := map[string]int{}
	pre := map[string]int{}
	for i, o := range objs {
		if comp[i] != big {
			continue
		}
		switch o.(type) {
		case *types.Func:
			kinds["func/method"]++
		case *types.TypeName:
			kinds["type"]++
		default:
			kinds["var"]++
		}
		nm := o.Name()
		if strings.HasPrefix(nm, "rbDyn") {
			pre["rbDyn*"]++
		} else if strings.HasPrefix(nm, "Dyn") {
			pre["Dyn*"]++
		} else {
			pre["other"]++
		}
	}
	fmt.Println("biggest SCC kinds", kinds, "names", pre)
}

func recvNamed(fn *types.Func) *types.Named {
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil {
		return nil
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := t.(*types.Named)
	return n
}
