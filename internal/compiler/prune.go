package compiler

import (
	"cmp"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// stdMethodNames are called by the standard library through its interfaces (fmt.Stringer, error), never by a selector here.
var stdMethodNames = map[string]bool{
	"String": true, "Error": true, "Unwrap": true, "Is": true, "As": true,
	"Format": true, "GoString": true, "MarshalJSON": true, "UnmarshalJSON": true,
	"ServeHTTP": true, "Read": true, "Write": true, "Close": true,
	"Len": true, "Less": true, "Swap": true, "Lock": true, "Unlock": true,
}

// parkedField is a named interface's method waiting to be selected: by name anywhere, or by a method expression on that interface.
type parkedField struct {
	iface, name string
}

type pruneMethod struct {
	recv string
	decl *ast.FuncDecl
	fwd  bool // a generic type's forwarder to its free func: kept only when a kept interface declares it (decision 86)
}

// pruner keeps what main reaches like a linker would; names match unscoped, which only over-keeps (decision 49), and a method nothing selects survives only as a stub for an interface literal's run-time assertion, since named interfaces drop it instead (decision 89).
type pruner struct {
	byName        map[string][]ast.Node     // funcs, type specs and value specs by declared name
	constBlock    map[ast.Node]*ast.GenDecl // an iota block stays whole: its specs repeat the one before
	methods       []pruneMethod
	roots         []ast.Node
	kept          map[ast.Node]bool
	names         map[string]bool            // identifiers used by kept code
	methodNames   map[string]bool            // selectors used by kept code, and methods the standard library calls
	scoped        map[string]map[string]bool // method expressions in kept code, by type: `String.Upcase(s)` selects Upcase on String alone, `FooI.Bar(x)` on FooI and its implementers (decision 122)
	ifaceName     map[*ast.InterfaceType]string
	recvMethods   map[string]map[string]bool // method names by receiver type, for the `_Foo` markers that say which named interfaces a type implements
	recvIfaces    map[string][]string        // memo of ifacesOf
	ifaceAnc      map[string][]string        // memo of ancestorsOf
	scopedBy      map[string][]string        // the types a method name is scoped on, for scopedOn
	declared      map[string]bool            // methods declared by kept interfaces
	asserted      map[string]bool            // methods declared by interface literals in kept code: runtime assertions, so every type keeps them (decision 89)
	named         map[*ast.InterfaceType]bool
	stubs         map[*ast.FuncDecl]bool
	pending       map[*ast.CaseClause][]string // type-switch cases waiting for their declared type names to be kept
	parked        map[*ast.Field]parkedField   // named interfaces' unselected methods: visited (their types kept) only once selected, as sweep slims them otherwise
	cur           ast.Node                     // the node being visited, for why
	why           map[ast.Node]ast.Node        // the node whose visit first kept each kept node (RUGAROO_PRUNE_WHY=1 dumps the chains)
	lazy          map[string]bool              // prelude constants: main's assignment to one waits for something else to name it
	deferred      map[ast.Stmt]string          // those assignments, by constant
	queue         []ast.Node
	written       shapeSet                          // every func type kept code writes: no *func value of another shape can exist (decision 431)
	made          shapeSet                          // the *func types kept code's creation sites (Ref(f), &f, new, a *func type written) make: rbIsProc's cases
	procValve     string                            // a creation site's expression whose func type funcTypesOf cannot tell: made falls back to written
	funcFields    map[string][]*ast.FuncType        // struct fields of func type, by field name
	methodsByName map[string][]*ast.FuncDecl        // methods by name, for a method value or a call's result
	parsedShapes  map[string]*ast.FuncType          // keptFunc's queries, parsed once
	genCalls      []genCall                         // calls of generic funcs in kept code, for settleMade
	addrOf        map[*ast.FuncDecl]map[string]bool // a generic func's type parameters whose values it points at
	settled       map[argKey]bool
	tparamOwner   map[*ast.Field]*ast.FuncDecl            // a generic func's type parameter fields
	genRanges     [][2]token.Pos                          // generic funcs' and generic types' methods' extents, for inGeneric
	genUnknown    bool                                    // a creation site in generic code holds a value funcTypesOf cannot type: instArgs
	inTypeArgs    []*ast.FuncType                         // func types written inside type arguments
	typeSwitchOf  map[*ast.AssignStmt]*ast.TypeSwitchStmt // a type switch's `v := x.(type)`, for caseFuncs
	funcElems     []*ast.FuncType                         // element types of kept arrays, slices and maps of funcs
	wantElems     bool                                    // a creation site indexes a container funcTypesOf cannot type: made takes funcElems
	instDone      int                                     // genCalls instArgs has read
	typeParams    map[string]bool                         // type parameter names any declaration binds
}

// newPruner drops what main cannot reach, like the linker, so a program compiles only the prelude it uses; names match unscoped, which only over-keeps. Declarations arrive in batches (add, then run): the dynamic dispatchers are emitted only for what the first batch reaches.
func newPruner(lazy map[string]bool) *pruner {
	p := &pruner{lazy: lazy, deferred: map[ast.Stmt]string{}, byName: map[string][]ast.Node{}, constBlock: map[ast.Node]*ast.GenDecl{}, kept: map[ast.Node]bool{}, names: map[string]bool{}, methodNames: map[string]bool{}, declared: map[string]bool{}, asserted: map[string]bool{}, named: map[*ast.InterfaceType]bool{}, stubs: map[*ast.FuncDecl]bool{}, pending: map[*ast.CaseClause][]string{}, written: newShapeSet(), made: newShapeSet(), funcFields: map[string][]*ast.FuncType{}, methodsByName: map[string][]*ast.FuncDecl{}, parsedShapes: map[string]*ast.FuncType{}, addrOf: map[*ast.FuncDecl]map[string]bool{}, settled: map[argKey]bool{}, tparamOwner: map[*ast.Field]*ast.FuncDecl{}, typeSwitchOf: map[*ast.AssignStmt]*ast.TypeSwitchStmt{}, typeParams: map[string]bool{}, parked: map[*ast.Field]parkedField{}, why: map[ast.Node]ast.Node{}, scoped: map[string]map[string]bool{}, ifaceName: map[*ast.InterfaceType]string{}, recvMethods: map[string]map[string]bool{}, recvIfaces: map[string][]string{}, ifaceAnc: map[string][]string{}, scopedBy: map[string][]string{}}
	for n := range stdMethodNames {
		p.methodNames[n] = true
	}
	return p
}

// add indexes a batch; a root, or a declaration an earlier batch already named, is kept.
func (p *pruner) add(decls []ast.Decl) {
	roots := len(p.roots)
	for _, d := range decls {
		p.index(d)
		for _, n := range declNodes(d) {
			if p.names[n.name] {
				p.keep(n.node)
			}
		}
	}
	for _, r := range p.roots[roots:] {
		p.keep(r)
	}
}

// funcShape is ft as goFuncType renders it: parameter types without names, a single result.
func funcShape(ft *ast.FuncType) string {
	var b strings.Builder
	b.WriteString("func(")
	writeFieldTypes(&b, ft.Params)
	b.WriteString(")")
	switch n := ft.Results.NumFields(); {
	case n == 1:
		b.WriteString(" ")
		writeFieldTypes(&b, ft.Results)
	case n > 1:
		b.WriteString(" (")
		writeFieldTypes(&b, ft.Results)
		b.WriteString(")")
	}
	return b.String()
}

func writeFieldTypes(b *strings.Builder, l *ast.FieldList) {
	first := true
	for _, fl := range l.List {
		t := types.ExprString(fl.Type)
		if ft, ok := fl.Type.(*ast.FuncType); ok {
			t = funcShape(ft)
		}
		for range max(len(fl.Names), 1) {
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(t)
		}
	}
}

// shapeSet is a set of func shapes; one naming a type parameter is also a pattern any instance matches.
func newShapeSet() shapeSet { return shapeSet{exact: map[string]bool{}, memo: map[string]bool{}} }

type shapeSet struct {
	exact map[string]bool
	gen   []*ast.FuncType
	memo  map[string]bool // match's positive answers: the set only grows
}

func (p *pruner) note(set *shapeSet, ft *ast.FuncType) {
	s := funcShape(ft)
	if set.exact[s] {
		return
	}
	set.exact[s] = true
	if p.isGeneric(ft) {
		set.gen = append(set.gen, ft)
	}
}

func (p *pruner) match(set *shapeSet, s string, ft *ast.FuncType) bool {
	if set.exact[s] || set.memo[s] {
		return true
	}
	for _, g := range set.gen {
		if p.unifyFunc(g, ft) {
			set.memo[s] = true
			return true
		}
	}
	return false
}

func (p *pruner) isGeneric(e ast.Node) bool {
	generic := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && p.isTypeParam(id.Name) {
			generic = true
		}
		return !generic
	})
	return generic
}

func (p *pruner) isTypeParam(name string) bool { return p.typeParams[name] && len(p.byName[name]) == 0 }

// keptFunc: kept code can hold a *func value of shape s: one a creation site makes, or any written once procValve opens.
func (p *pruner) keptFunc(s string) bool {
	ft, ok := p.parsedShapes[s]
	if !ok {
		e, err := goparser.ParseExpr(s)
		if err == nil {
			ft, _ = e.(*ast.FuncType)
		}
		p.parsedShapes[s] = ft
	}
	if ft == nil {
		return true
	}
	p.settleMade()
	if p.procValve != "" {
		return p.match(&p.written, s, ft)
	}
	if p.genUnknown {
		p.instArgs()
	}
	if p.wantElems {
		for _, ft := range p.funcElems {
			p.note(&p.made, ft)
		}
	}
	return p.match(&p.made, s, ft)
}

// instArgs: a type parameter is instantiated by a written type argument or an inferred call argument, so those func types may be made.
func (p *pruner) instArgs() {
	for _, ft := range p.inTypeArgs {
		p.note(&p.made, ft)
	}
	for ; p.instDone < len(p.genCalls); p.instDone++ {
		for _, a := range p.genCalls[p.instDone].call.Args {
			if fts, ok := p.funcTypesOf(a, 0); ok {
				for _, ft := range fts {
					p.note(&p.made, ft)
				}
			}
		}
	}
}

// inGeneric: pos is inside a generic func, or a method of a generic type.
func (p *pruner) inGeneric(pos token.Pos) bool {
	for _, r := range p.genRanges {
		if r[0] <= pos && pos < r[1] {
			return true
		}
	}
	return false
}

// noteMade records the func types a pointer made from e (Ref(e), &e) can have; an e it cannot type opens procValve.
func (p *pruner) noteMade(e ast.Expr) {
	if id, ok := e.(*ast.Ident); ok {
		if fd, tp, ok := p.tparamOf(id); ok { // made where fd is called, from the argument bound to tp
			if p.addrOf[fd] == nil {
				p.addrOf[fd] = map[string]bool{}
			}
			p.addrOf[fd][tp] = true
			return
		}
	}
	fts, known := p.funcTypesOf(e, 0)
	switch {
	case !known && p.inGeneric(e.Pos()): // a value of a type parameter: any func type that instantiates one (instArgs)
		p.genUnknown = true
		return
	case !known:
		if p.procValve == "" {
			p.procValve = types.ExprString(e)
			if fd, ok := p.cur.(*ast.FuncDecl); ok {
				p.procValve += " in " + fd.Name.Name
			}
		}
		return
	}
	for _, ft := range fts {
		p.note(&p.made, ft)
	}
}

// funcTypesOf: e's possible func types, fields and methods matched by name since there are no go/types here; known false when untellable.
func (p *pruner) funcTypesOf(e ast.Expr, depth int) (fts []*ast.FuncType, known bool) {
	if depth > 8 {
		return nil, false
	}
	switch x := e.(type) {
	case *ast.FuncLit:
		return []*ast.FuncType{x.Type}, true
	case *ast.BasicLit, *ast.CompositeLit, *ast.BinaryExpr, *ast.SliceExpr: // never a func
		return nil, true
	case *ast.ParenExpr:
		return p.funcTypesOf(x.X, depth+1)
	case *ast.TypeAssertExpr:
		return p.typeFuncs(x.Type)
	case *ast.UnaryExpr:
		return nil, x.Op != token.ARROW
	case *ast.Ident:
		return p.identFuncs(x, depth)
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Obj == nil && stdImports[id.Name] != "" && len(p.byName[id.Name]) == 0 {
			return nil, false
		}
		out := slices.Clone(p.funcFields[x.Sel.Name])
		for _, m := range p.methodsByName[x.Sel.Name] { // a method value
			out = append(out, m.Type)
		}
		return out, true
	case *ast.CallExpr:
		return p.callFuncs(x, 0, depth)
	case *ast.StarExpr: // `*p` of a *F: some creation site already made that *F, so a copy's &dup adds no shape
		return nil, true
	case *ast.IndexExpr: // an element of an array, slice or map variable
		if t := p.declType(x.X); t != nil {
			switch t := t.(type) {
			case *ast.ArrayType:
				return p.typeFuncs(t.Elt)
			case *ast.MapType:
				return p.typeFuncs(t.Value)
			}
		}
		p.wantElems = true // any kept array, slice or map of funcs, added once all are seen; std containers hold none Ref'd
		return nil, true
	}
	return nil, false
}

// declType is the declared type of a variable e names, or nil.
func (p *pruner) declType(e ast.Expr) ast.Expr {
	id, ok := e.(*ast.Ident)
	if !ok {
		return nil
	}
	var decl any
	if id.Obj != nil {
		decl = id.Obj.Decl
	} else if ds := p.byName[id.Name]; len(ds) > 0 {
		decl = ds[0]
	}
	switch d := decl.(type) {
	case *ast.Field:
		return d.Type
	case *ast.ValueSpec:
		return d.Type
	case *ast.AssignStmt: // `x := make(T, n)` or `x := T{...}`
		if len(d.Lhs) != 1 || len(d.Rhs) != 1 {
			return nil
		}
		switch r := d.Rhs[0].(type) {
		case *ast.CompositeLit:
			return r.Type
		case *ast.CallExpr:
			if f, ok := r.Fun.(*ast.Ident); ok && f.Name == "make" && len(r.Args) > 0 {
				return r.Args[0]
			}
		}
	}
	return nil
}

func (p *pruner) typeFuncs(t ast.Expr) ([]*ast.FuncType, bool) {
	switch t := t.(type) {
	case *ast.FuncType:
		return []*ast.FuncType{t}, true
	case *ast.Ident:
		if p.isTypeParam(t.Name) {
			return nil, false
		}
		for _, d := range p.byName[t.Name] {
			if ts, ok := d.(*ast.TypeSpec); ok {
				return p.typeFuncs(ts.Type)
			}
		}
	}
	return nil, true
}

func (p *pruner) identFuncs(x *ast.Ident, depth int) ([]*ast.FuncType, bool) {
	if x.Obj == nil { // another batch's declaration, or a predeclared name
		for _, d := range p.byName[x.Name] {
			switch d := d.(type) {
			case *ast.FuncDecl:
				return []*ast.FuncType{d.Type}, true
			case *ast.ValueSpec:
				return p.specFuncs(d, x.Name, depth)
			}
		}
		return nil, true
	}
	switch d := x.Obj.Decl.(type) {
	case *ast.Field:
		return p.typeFuncs(d.Type)
	case *ast.ValueSpec:
		return p.specFuncs(d, x.Name, depth)
	case *ast.FuncDecl:
		return []*ast.FuncType{d.Type}, true
	case *ast.TypeSpec:
		return nil, true
	case *ast.AssignStmt:
		for i, l := range d.Lhs {
			if li, ok := l.(*ast.Ident); ok && li.Name == x.Name {
				if ts := p.typeSwitchOf[d]; ts != nil {
					return p.caseFuncs(ts)
				}
				if r, ok := d.Rhs[0].(*ast.UnaryExpr); ok && r.Op == token.RANGE {
					return p.rangeFuncs(r.X, i)
				}
				if len(d.Rhs) == len(d.Lhs) {
					return p.funcTypesOf(d.Rhs[i], depth+1)
				}
				if call, ok := d.Rhs[0].(*ast.CallExpr); ok && len(d.Rhs) == 1 {
					return p.callFuncs(call, i, depth+1)
				}
				if i > 0 { // the ok of `v, ok := m[k]`, `x.(T)` or `<-ch`
					return nil, true
				}
				return p.funcTypesOf(d.Rhs[0], depth+1)
			}
		}
	}
	return nil, false
}

func (p *pruner) specFuncs(d *ast.ValueSpec, name string, depth int) ([]*ast.FuncType, bool) {
	if d.Type != nil {
		return p.typeFuncs(d.Type)
	}
	for i, n := range d.Names {
		if n.Name == name && len(d.Values) == len(d.Names) {
			return p.funcTypesOf(d.Values[i], depth+1)
		}
	}
	return nil, false
}

// rangeFuncs is the func types of a range's key (i 0) or value: an array, slice or map variable's, else any kept container's.
func (p *pruner) rangeFuncs(x ast.Expr, i int) ([]*ast.FuncType, bool) {
	switch t := p.declType(x).(type) {
	case *ast.ArrayType:
		if i == 0 {
			return nil, true
		}
		return p.typeFuncs(t.Elt)
	case *ast.MapType:
		if i == 0 {
			return nil, true // a func is not comparable
		}
		return p.typeFuncs(t.Value)
	}
	p.wantElems, p.genUnknown = true, true // a kept container's element, or an iterator's yield: a type argument (iter.Seq[F])
	return nil, true
}

// caseFuncs is a type switch's bound variable's func types: those of every case.
func (p *pruner) caseFuncs(ts *ast.TypeSwitchStmt) ([]*ast.FuncType, bool) {
	var out []*ast.FuncType
	for _, st := range ts.Body.List {
		for _, t := range st.(*ast.CaseClause).List {
			fts, ok := p.typeFuncs(t)
			if !ok {
				return nil, false
			}
			out = append(out, fts...)
		}
	}
	return out, true
}

// callFuncs is the func types a call's result ri may have: a conversion's type, a func's or method's (by name) result.
func (p *pruner) callFuncs(x *ast.CallExpr, ri, depth int) ([]*ast.FuncType, bool) {
	var decls []*ast.FuncType
	switch fn := x.Fun.(type) {
	case *ast.FuncLit:
		decls = []*ast.FuncType{fn.Type}
	case *ast.Ident, *ast.IndexExpr, *ast.IndexListExpr:
		id, _ := fn.(*ast.Ident)
		if id == nil {
			id, _ = recvTypeIdent(fn)
		}
		if id == nil {
			return nil, false
		}
		if types.Universe.Lookup(id.Name) != nil && len(p.byName[id.Name]) == 0 {
			return nil, true // string(x), len(x), ...
		}
		for _, d := range p.byName[id.Name] {
			switch d := d.(type) {
			case *ast.TypeSpec:
				return p.typeFuncs(d.Type)
			case *ast.FuncDecl:
				decls = append(decls, d.Type)
			}
		}
		if len(decls) == 0 {
			return nil, false
		}
	case *ast.SelectorExpr:
		if id, ok := fn.X.(*ast.Ident); ok && id.Obj == nil && stdImports[id.Name] != "" && len(p.byName[id.Name]) == 0 {
			return nil, true // ponytail: a std func returning a func (sync.OnceValue) is not Ref'd by generated code; type it if one is
		}
		for _, m := range p.methodsByName[fn.Sel.Name] {
			decls = append(decls, m.Type)
		}
		decls = append(decls, p.funcFields[fn.Sel.Name]...)
	default:
		return nil, false
	}
	var out []*ast.FuncType
	for _, d := range decls {
		rs := fieldTypes(d.Results)
		if ri >= len(rs) {
			continue
		}
		fts, known := p.typeFuncs(rs[ri])
		if !known {
			fts, known = p.instResult(x, d, rs[ri], depth)
		}
		if !known && p.isGeneric(rs[ri]) { // a generic type's method returning its parameter: an instance's type argument (instArgs)
			p.genUnknown, known = true, true
		}
		if !known {
			return nil, false
		}
		out = append(out, fts...)
	}
	return out, true
}

// instResult types a generic call's bare type parameter result: its explicit type argument, else the arguments it was inferred from.
func (p *pruner) instResult(x *ast.CallExpr, d *ast.FuncType, r ast.Expr, depth int) ([]*ast.FuncType, bool) {
	t, ok := r.(*ast.Ident)
	if !ok || d.TypeParams == nil {
		return nil, false
	}
	tps := fieldTypes(d.TypeParams) // one entry per name, in order
	names := make([]string, 0, len(tps))
	for _, f := range d.TypeParams.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	i := slices.Index(names, t.Name)
	if i < 0 {
		return nil, false
	}
	if _, args := typeArgs(x.Fun); i < len(args) {
		return p.typeFuncs(args[i])
	}
	var out []*ast.FuncType
	found := false
	for j, pt := range fieldTypes(d.Params) {
		if id, ok := pt.(*ast.Ident); ok && id.Name == t.Name && j < len(x.Args) {
			fts, ok := p.funcTypesOf(x.Args[j], depth+1)
			if !ok {
				return nil, false
			}
			found = true
			out = append(out, fts...)
		}
	}
	return out, found
}

func recvTypeIdent(e ast.Expr) (*ast.Ident, bool) {
	x, _ := typeArgs(e)
	id, ok := x.(*ast.Ident)
	return id, ok
}

// visitMade notes kept code's creation sites; a type parameter's value is made where the func is called (settleMade).
func (p *pruner) visitMade(x ast.Node) {
	switch x := x.(type) {
	case *ast.StarExpr:
		if ft, ok := x.X.(*ast.FuncType); ok {
			p.note(&p.made, ft) // a *func type written: a value of it may come from anywhere
		}
	case *ast.ArrayType:
		if ft, ok := x.Elt.(*ast.FuncType); ok {
			p.funcElems = append(p.funcElems, ft)
		}
	case *ast.MapType:
		if ft, ok := x.Value.(*ast.FuncType); ok {
			p.funcElems = append(p.funcElems, ft)
		}
	case *ast.ChanType:
		if ft, ok := x.Value.(*ast.FuncType); ok {
			p.funcElems = append(p.funcElems, ft)
		}
	case *ast.UnaryExpr:
		if _, lit := x.X.(*ast.CompositeLit); x.Op == token.AND && !lit {
			p.noteMade(x.X)
		}
	case *ast.IndexExpr, *ast.IndexListExpr: // an explicit instantiation with a func type: generic code may point at one
		_, args := typeArgs(x.(ast.Expr))
		for _, a := range args {
			if ft, ok := a.(*ast.FuncType); ok {
				p.note(&p.made, ft)
			}
			ast.Inspect(a, func(n ast.Node) bool {
				if ft, ok := n.(*ast.FuncType); ok {
					p.inTypeArgs = append(p.inTypeArgs, ft)
				}
				return true
			})
		}
	case *ast.CallExpr:
		id, _ := x.Fun.(*ast.Ident)
		if id == nil {
			id, _ = recvTypeIdent(x.Fun)
		}
		if id == nil {
			return
		}
		if id.Name == "new" && len(x.Args) == 1 && len(p.byName["new"]) == 0 {
			if ft, ok := x.Args[0].(*ast.FuncType); ok {
				p.note(&p.made, ft)
			}
			return
		}
		for _, d := range p.byName[id.Name] {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Type.TypeParams != nil {
				p.genCalls = append(p.genCalls, genCall{x, fd})
			}
		}
	}
}

type genCall struct {
	call *ast.CallExpr
	fd   *ast.FuncDecl
}

// settleMade: an argument bound to a type parameter whose value the callee points at (Ref's `&v`) is a creation site.
func (p *pruner) settleMade() {
	for changed := true; changed; {
		changed = false
		for _, g := range p.genCalls {
			for i, pt := range fieldTypes(g.fd.Type.Params) {
				t, ok := pt.(*ast.Ident)
				if !ok || i >= len(g.call.Args) || !p.addrOf[g.fd][t.Name] {
					continue
				}
				k := argKey{g.call, i}
				if p.settled[k] {
					continue
				}
				p.settled[k] = true
				changed = true
				p.noteMade(g.call.Args[i])
			}
		}
	}
}

type argKey struct {
	call *ast.CallExpr
	i    int
}

// tparamOf: id is a value of a type parameter some generic func declares, which it names.
func (p *pruner) tparamOf(id *ast.Ident) (*ast.FuncDecl, string, bool) {
	if id.Obj == nil {
		return nil, "", false
	}
	var t ast.Expr
	switch d := id.Obj.Decl.(type) {
	case *ast.Field:
		t = d.Type
	case *ast.ValueSpec:
		t = d.Type
	}
	tid, ok := t.(*ast.Ident)
	if !ok || tid.Obj == nil || tid.Obj.Kind != ast.Typ {
		return nil, "", false
	}
	f, _ := tid.Obj.Decl.(*ast.Field)
	if fd := p.tparamOwner[f]; fd != nil {
		return fd, tid.Name, true
	}
	return nil, "", false
}

// unifyFunc reports whether concrete func type x is an instance of pattern g, each type parameter standing for any one type.
func (p *pruner) unifyFunc(g, x *ast.FuncType) bool {
	return p.unifyFields(g.Params, x.Params) && p.unifyFields(g.Results, x.Results)
}

func (p *pruner) unifyFields(g, x *ast.FieldList) bool {
	gs, xs := fieldTypes(g), fieldTypes(x)
	if len(gs) != len(xs) {
		return false
	}
	for i := range gs {
		if !p.unify(gs[i], xs[i]) {
			return false
		}
	}
	return true
}

func fieldTypes(l *ast.FieldList) []ast.Expr {
	if l == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range l.List {
		for range max(len(f.Names), 1) {
			out = append(out, f.Type)
		}
	}
	return out
}

func (p *pruner) unify(g, x ast.Expr) bool {
	switch g := g.(type) {
	case *ast.Ident:
		if p.isTypeParam(g.Name) {
			return true
		}
	case *ast.StarExpr:
		y, ok := x.(*ast.StarExpr)
		return ok && p.unify(g.X, y.X)
	case *ast.Ellipsis:
		y, ok := x.(*ast.Ellipsis)
		return ok && p.unify(g.Elt, y.Elt)
	case *ast.ArrayType:
		y, ok := x.(*ast.ArrayType)
		return ok && (g.Len == nil) == (y.Len == nil) && (g.Len == nil || types.ExprString(g.Len) == types.ExprString(y.Len)) && p.unify(g.Elt, y.Elt)
	case *ast.MapType:
		y, ok := x.(*ast.MapType)
		return ok && p.unify(g.Key, y.Key) && p.unify(g.Value, y.Value)
	case *ast.ChanType:
		y, ok := x.(*ast.ChanType)
		return ok && g.Dir == y.Dir && p.unify(g.Value, y.Value)
	case *ast.FuncType:
		y, ok := x.(*ast.FuncType)
		return ok && p.unifyFunc(g, y)
	case *ast.IndexExpr, *ast.IndexListExpr:
		gx, gi := typeArgs(g)
		yx, yi := typeArgs(x)
		if yx == nil || len(gi) != len(yi) || !p.unify(gx, yx) {
			return false
		}
		for i := range gi {
			if !p.unify(gi[i], yi[i]) {
				return false
			}
		}
		return true
	}
	return types.ExprString(g) == types.ExprString(x)
}

// typeArgs splits an instantiated type into its generic type and arguments; nil for anything else.
func typeArgs(e ast.Expr) (ast.Expr, []ast.Expr) {
	switch e := e.(type) {
	case *ast.IndexExpr:
		return e.X, []ast.Expr{e.Index}
	case *ast.IndexListExpr:
		return e.X, e.Indices
	}
	return nil, nil
}

// reached reports whether kept code names ident, selected whether it selects or asserts method m.
func (p *pruner) reached(ident string) bool { return p.names[ident] }
func (p *pruner) selected(m string) bool {
	return p.methodNames[m] || p.asserted[m] || p.declared[m]
}

type declNode struct {
	name string
	node ast.Node
}

// declNodes is what d declares by name: funcs, types and vars (not methods, which the run loop takes by receiver).
func declNodes(d ast.Decl) []declNode {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil {
			return []declNode{{d.Name.Name, d}}
		}
	case *ast.GenDecl:
		var out []declNode
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				out = append(out, declNode{s.Name.Name, s})
			case *ast.ValueSpec:
				for _, n := range s.Names {
					out = append(out, declNode{n.Name, s})
				}
			}
		}
		return out
	}
	return nil
}

// run keeps everything the queue reaches; the method pass runs at least once, since a batch of only methods leaves the queue empty.
func (p *pruner) run() {
	for {
		for len(p.queue) > 0 {
			n := p.queue[len(p.queue)-1]
			p.queue = p.queue[:len(p.queue)-1]
			p.cur = n
			ast.Inspect(n, p.visit)
		}
		for st, name := range p.deferred {
			if p.names[name] {
				delete(p.deferred, st)
				p.queue = append(p.queue, st)
			}
		}
		for m, pf := range p.parked {
			if name := pf.name; p.methodNames[name] || p.mustDeclare(name) || p.ifaceSelected(pf.iface, name) {
				delete(p.parked, m)
				p.queue = append(p.queue, m)
			}
		}
		for cc, names := range p.pending {
			if !slices.ContainsFunc(names, func(n string) bool { return !p.names[n] }) {
				delete(p.pending, cc)
				p.queue = append(p.queue, cc)
			}
		}
		for _, m := range p.methods {
			if !p.names[m.recv] {
				continue
			}
			name := m.decl.Name.Name
			// a selector matches unscoped, and Ruby's names (size, first, ...) are on every class: a generic
			// type's forwarder taken on such a match would instantiate its free func for every type argument
			selected := (p.methodNames[name] || p.scopedOn(m.recv, name)) && (!m.fwd || p.declared[name])
			switch {
			case selected && p.stubs[m.decl]:
				delete(p.stubs, m.decl)
				p.queue = append(p.queue, m.decl.Body)
			case selected:
				if ts := p.byName[m.recv]; len(ts) > 0 { // kept for its receiver, as far as why can tell
					p.cur = ts[0]
				}
				p.keep(m.decl)
			case p.mustDeclare(name) && !p.kept[m.decl]:
				p.kept[m.decl] = true
				p.stubs[m.decl] = true
				p.queue = append(p.queue, m.decl.Recv, m.decl.Type)
			}
		}
		if len(p.queue) == 0 {
			break
		}
	}
}

func (p *pruner) index(d ast.Decl) {
	switch d := d.(type) {
	case *ast.FuncDecl:
		p.noteTypeParams(d.Type.TypeParams)
		if d.Type.TypeParams != nil {
			for _, f := range d.Type.TypeParams.List {
				p.tparamOwner[f] = d
			}
		}
		if d.Recv != nil {
			p.noteRecvTypeParams(d.Recv.List[0].Type)
		}
		if d.Type.TypeParams != nil || d.Recv != nil && p.isGeneric(d.Recv) {
			p.genRanges = append(p.genRanges, [2]token.Pos{d.Pos(), d.End()})
		}
		switch {
		case d.Recv != nil:
			recv := recvName(d.Recv.List[0].Type)
			p.methods = append(p.methods, pruneMethod{recv, d, isGenericForwarder(d)})
			if p.recvMethods[recv] == nil {
				p.recvMethods[recv] = map[string]bool{}
			}
			p.recvMethods[recv][d.Name.Name] = true
			p.methodsByName[d.Name.Name] = append(p.methodsByName[d.Name.Name], d)
			delete(p.recvIfaces, recv)
		case d.Name.Name == "main":
			p.kept[d] = true
			p.queue = append(p.queue, d.Type)
			for _, st := range d.Body.List {
				if name := p.lazyConst(st); name != "" {
					p.deferred[st] = name
				} else {
					p.queue = append(p.queue, st)
				}
			}
		case d.Name.Name == "init":
			p.roots = append(p.roots, d)
		default:
			p.byName[d.Name.Name] = append(p.byName[d.Name.Name], d)
		}
	case *ast.GenDecl:
		for _, s := range d.Specs {
			p.indexSpec(d, s)
		}
	}
}

func (p *pruner) noteTypeParams(l *ast.FieldList) {
	if l == nil {
		return
	}
	for _, f := range l.List {
		for _, n := range f.Names {
			p.typeParams[n.Name] = true
		}
	}
}

// noteRecvTypeParams: a generic type's method names its parameters in the receiver (`*Hash[K, V]`).
func (p *pruner) noteRecvTypeParams(e ast.Expr) {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	var idx []ast.Expr
	switch x := e.(type) {
	case *ast.IndexExpr:
		idx = []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		idx = x.Indices
	}
	for _, i := range idx {
		if id, ok := i.(*ast.Ident); ok {
			p.typeParams[id.Name] = true
		}
	}
}

func (p *pruner) indexSpec(d *ast.GenDecl, s ast.Spec) {
	switch s := s.(type) {
	case *ast.TypeSpec:
		p.byName[s.Name.Name] = append(p.byName[s.Name.Name], s)
		p.noteTypeParams(s.TypeParams)
		if st, ok := s.Type.(*ast.StructType); ok {
			for _, fl := range st.Fields.List {
				if ft, ok := fl.Type.(*ast.FuncType); ok {
					for _, n := range fl.Names {
						p.funcFields[n.Name] = append(p.funcFields[n.Name], ft)
					}
				}
			}
		}
		if it, ok := s.Type.(*ast.InterfaceType); ok {
			p.named[it] = true
			p.ifaceName[it] = s.Name.Name
		}
	case *ast.ValueSpec:
		if d.Tok == token.CONST {
			p.constBlock[s] = d
		}
		if !slices.ContainsFunc(s.Names, func(n *ast.Ident) bool { return n.Name != "_" }) {
			p.roots = append(p.roots, s) // `var _ = ...` is a marker; `var x, _ = f()` is kept by x alone
		}
		for _, n := range s.Names {
			if n.Name != "_" { // kept code's own blanks would name it
				p.byName[n.Name] = append(p.byName[n.Name], s)
			}
		}
	}
}

// lazyConst is the prelude constant st assigns, or "": an unused one's initializer (an allocation, never a side effect) need not run, and what it alone reaches goes too.
func (p *pruner) lazyConst(st ast.Stmt) string {
	as, ok := st.(*ast.AssignStmt)
	if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 {
		return ""
	}
	if id, ok := as.Lhs[0].(*ast.Ident); ok && p.lazy[id.Name] {
		return id.Name
	}
	return ""
}

func (p *pruner) keep(n ast.Node) {
	if p.kept[n] {
		return
	}
	p.kept[n] = true
	if p.cur != nil {
		p.why[n] = p.cur
	}
	p.queue = append(p.queue, n)
	if d := p.constBlock[n]; d != nil {
		for _, s := range d.Specs {
			p.keep(s)
		}
	}
}

func (p *pruner) visit(x ast.Node) bool {
	p.visitMade(x)
	switch x := x.(type) {
	case *ast.FuncType: // a signature, literal or type: what a Ref(f), &f or method value can point at
		p.note(&p.written, x)
	case *ast.TypeSwitchStmt:
		if as, ok := x.Assign.(*ast.AssignStmt); ok {
			p.typeSwitchOf[as] = x
		}
		if x.Init != nil {
			ast.Inspect(x.Init, p.visit)
		}
		ast.Inspect(x.Assign, p.visit)
		for _, st := range x.Body.List {
			cc := st.(*ast.CaseClause)
			// only a single concrete type: narrowing `case A, B:` would change its variable's type,
			// and values implementing an interface exist without anything naming the interface
			if names := p.caseTypeNames(cc.List...); len(cc.List) == 1 && !p.isInterface(cc.List[0]) && len(names) > 0 {
				p.pending[cc] = names // visited once every name is kept
				continue
			}
			if len(cc.List) == 1 && p.undeclaredType(cc.List[0]) {
				p.pending[cc] = []string{""} // a lib's type the program never loaded (decision 155): the case is dead, never kept
				continue
			}
			ast.Inspect(cc, p.visit)
		}
		return false
	case *ast.SwitchStmt:
		p.visitSwitch(x)
		return false
	case *ast.Ident:
		if !p.names[x.Name] {
			p.names[x.Name] = true
			for _, d := range p.byName[x.Name] {
				p.keep(d)
			}
		}
	case *ast.SelectorExpr:
		return p.visitSelector(x)
	case *ast.InterfaceType:
		for _, m := range x.Methods.List {
			for _, mn := range m.Names {
				p.declared[mn.Name] = true
				if !p.named[x] {
					p.asserted[mn.Name] = true
				}
			}
		}
		if p.named[x] {
			for _, m := range x.Methods.List {
				if len(m.Names) > 0 && !p.methodNames[m.Names[0].Name] && !p.mustDeclare(m.Names[0].Name) && !p.ifaceSelected(p.ifaceName[x], m.Names[0].Name) {
					p.parked[m] = parkedField{p.ifaceName[x], m.Names[0].Name}
					continue
				}
				ast.Inspect(m, p.visit)
			}
			return false
		}
	}
	return true
}

// visitSelector records what a selector selects: nothing for a std package's member (Obj == nil: not a local such as
// `net`, as format.go's import scan tells them apart; os.Interrupt is not class Interrupt, time.Sleep selects no
// class's Sleep), the method on one type for a method expression (`String.Upcase(s)`; the Ident keeps the type), else
// the name on every class.
func (p *pruner) visitSelector(x *ast.SelectorExpr) bool {
	if id, ok := x.X.(*ast.Ident); ok && id.Obj == nil && stdImports[id.Name] != "" && len(p.byName[id.Name]) == 0 {
		return false
	}
	if t := p.methodExprType(x.X); t != "" {
		if p.scoped[t] == nil {
			p.scoped[t] = map[string]bool{}
		}
		if !p.scoped[t][x.Sel.Name] {
			p.scoped[t][x.Sel.Name] = true
			p.scopedBy[x.Sel.Name] = append(p.scopedBy[x.Sel.Name], t)
		}
		return true
	}
	p.methodNames[x.Sel.Name] = true
	return true
}

// methodExprType is the declared type a method expression selects on (`String` in `String.Upcase(s)`, `Regexp` in `(*Regexp).Match(r)`), or "".
func (p *pruner) methodExprType(x ast.Expr) string {
	if pe, ok := x.(*ast.ParenExpr); ok {
		if se, ok := pe.X.(*ast.StarExpr); ok {
			x = se.X
		}
	}
	id, ok := x.(*ast.Ident)
	if !ok || id.Obj != nil && id.Obj.Kind != ast.Typ { // resolved to a local or a func, not a type
		return ""
	}
	for _, d := range p.byName[id.Name] {
		if _, ok := d.(*ast.TypeSpec); ok {
			return id.Name
		}
	}
	return ""
}

// scopedOn reports whether a method expression selects name on recv: on the type itself, on a named interface recv
// implements (a call through the superclass's interface reaches the subclass's override), or on the interface of a
// class that descends from recv (a call through the subclass's interface reaches the method recv's struct promotes).
func (p *pruner) scopedOn(recv, name string) bool {
	for _, t := range p.scopedBy[name] {
		if t == recv || t == recv+"I" || slices.Contains(p.ifacesOf(recv), t) || slices.Contains(p.ancestorsOf(t), recv+"I") {
			return true
		}
	}
	return false
}

// ifacesOf lists the named interfaces recv implements, read off its `_Foo` markers (decision 89): `_Foo` on a type
// means it is a Foo, so it is a FooI. A subclass's markers come through its embedded `super_Foo_` alias, so the
// struct's embedded fields are followed.
func (p *pruner) ifacesOf(recv string) []string {
	if out, ok := p.recvIfaces[recv]; ok {
		return out
	}
	var out []string
	seen := map[string]bool{}
	var walk func(t string)
	walk = func(t string) {
		if seen[t] {
			return
		}
		seen[t] = true
		for name := range p.recvMethods[t] {
			if i := markerIface(name); i != "" && p.isInterface(&ast.Ident{Name: i}) {
				out = append(out, i)
			}
		}
		for _, d := range p.byName[t] {
			ts, ok := d.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Assign.IsValid() { // `type super_Foo_ = Foo`
				if n := recvName(ts.Type); n != "" {
					walk(n)
				}
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					if len(f.Names) == 0 {
						if n := recvName(f.Type); n != "" {
							walk(n)
						}
					}
				}
			}
		}
	}
	walk(recv)
	slices.Sort(out)
	p.recvIfaces[recv] = out
	return out
}

// markerIface is the interface a `_Foo` marker stands for (FooI), or "".
func markerIface(name string) string {
	if !strings.HasPrefix(name, "_") || strings.HasPrefix(name, "__") || len(name) < 2 {
		return ""
	}
	return name[1:] + "I"
}

// ancestorsOf lists the named interfaces iface's class descends from, itself included: the `_Foo()` markers a class's
// interface declares. A method selected on an ancestor's interface stays declared in iface, so a `Self` constrained by
// iface still satisfies the ancestor's (a subclass's super call, `Exception_Initialize[Self](self)`).
func (p *pruner) ancestorsOf(iface string) []string {
	if out, ok := p.ifaceAnc[iface]; ok {
		return out
	}
	var out []string
	for _, d := range p.byName[iface] {
		ts, ok := d.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if it, ok := ts.Type.(*ast.InterfaceType); ok {
			for _, m := range it.Methods.List {
				if len(m.Names) > 0 {
					if i := markerIface(m.Names[0].Name); i != "" && i != iface && p.isInterface(&ast.Ident{Name: i}) {
						out = append(out, i)
					}
				}
			}
		}
	}
	p.ifaceAnc[iface] = out
	return out
}

// ifaceSelected reports whether a method expression selects name on iface or on an interface iface's class descends from.
func (p *pruner) ifaceSelected(iface, name string) bool {
	if p.scoped[iface][name] {
		return true
	}
	for _, a := range p.ancestorsOf(iface) {
		if p.scoped[a][name] {
			return true
		}
	}
	return false
}

// mustDeclare reports whether every kept type has to keep a method of this name even when nothing calls it: an interface literal asserts it at run time, or it is a `_` marker (`_Foo()` decides `rescue`/`is_a?` matches through FooI).
func (p *pruner) mustDeclare(name string) bool {
	return p.asserted[name] || (p.declared[name] && p.identity(name))
}

// identity: `_X()` on a kept interface, X a declared type, is what makes a value of XI an X (decision 89); other `_` methods (`_ClassOf`, `__Sleep0`) are accessors, kept only when selected.
func (p *pruner) identity(name string) bool {
	if !strings.HasPrefix(name, "_") || strings.HasPrefix(name, "__") {
		return false
	}
	return slices.ContainsFunc(p.byName[name[1:]], func(n ast.Node) bool { _, ok := n.(*ast.TypeSpec); return ok })
}

// slimInterfaces drops from named interfaces the methods kept code never selects: the classes then need no stub for them (decision 89).
func (p *pruner) slimInterfaces(f *ast.File) {
	tested := map[string]bool{} // interfaces a type switch or assertion tests values against
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeAssertExpr:
			if id, ok := x.Type.(*ast.Ident); ok {
				tested[id.Name] = true
			}
		case *ast.TypeSwitchStmt:
			for _, st := range x.Body.List {
				for _, e := range st.(*ast.CaseClause).List { //nolint:forcetypeassert // a type switch body holds case clauses
					if id, ok := e.(*ast.Ident); ok {
						tested[id.Name] = true
					}
				}
			}
		}
		return true
	})
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			it, ok := s.(*ast.TypeSpec).Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			iface := s.(*ast.TypeSpec).Name.Name
			slim := slices.DeleteFunc(slices.Clone(it.Methods.List), func(m *ast.Field) bool {
				return len(m.Names) > 0 && !p.methodNames[m.Names[0].Name] && !p.mustDeclare(m.Names[0].Name) && !p.ifaceSelected(iface, m.Names[0].Name)
			})
			if len(slim) == 0 && len(it.Methods.List) > 0 && tested[iface] {
				// emptied, it would match every value in a type switch (rbClassOf's `case Method_Any:`); a method no type
				// has matches none, as the kept types implemented none of the dropped ones
				slim = []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("_rbNever")}, Type: &ast.FuncType{Params: &ast.FieldList{}}}}
			}
			it.Methods.List = slim
		}
	}
}

// isInterface reports whether type expression e is an interface: a literal, or a declared interface type.
func (p *pruner) isInterface(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.InterfaceType:
		return true
	case *ast.IndexExpr:
		return p.isInterface(x.X)
	case *ast.IndexListExpr:
		return p.isInterface(x.X)
	case *ast.Ident:
		for _, d := range p.byName[x.Name] {
			if ts, ok := d.(*ast.TypeSpec); ok {
				if _, iface := ts.Type.(*ast.InterfaceType); iface {
					return true
				}
			}
		}
	}
	return false
}

// dropDeadCases removes the cases whose type nothing kept. The closing brace
// moves up to the last case left (else the printer keeps the gap as blank
// lines), and a switch left with no case drops its variable (else Go reports
// it unused).
func (p *pruner) dropDeadCases(ts *ast.TypeSwitchStmt) {
	if !p.dropDeadClauses(ts.Body, ts.Assign) {
		return
	}
	// `x :=` with no surviving case that reads x would be "declared and not used"
	if as, ok := ts.Assign.(*ast.AssignStmt); ok && !slices.ContainsFunc(ts.Body.List, func(st ast.Stmt) bool { return mentions(st, as.Lhs[0].(*ast.Ident).Name) }) {
		ts.Assign = &ast.ExprStmt{X: as.Rhs[0]}
	}
}

// dropDeadClauses reports whether any clause went. A variable declared
// before the switch that only dropped clauses read (a union call's pinned
// argument, decision 150) is read in a `default:` instead, else Go reports
// it unused; own is the type switch's `x :=`, which is not such a variable.
func (p *pruner) dropDeadClauses(body *ast.BlockStmt, own ast.Stmt) bool {
	n := len(body.List)
	var dead []ast.Stmt
	body.List = slices.DeleteFunc(body.List, func(st ast.Stmt) bool {
		_, d := p.pending[st.(*ast.CaseClause)]
		if d {
			dead = append(dead, st)
		}
		return d
	})
	if len(body.List) == n {
		return false
	}
	var orphans []string
	for _, st := range dead {
		ast.Inspect(st, func(x ast.Node) bool {
			id, ok := x.(*ast.Ident)
			if !ok || id.Obj == nil || id.Obj.Kind != ast.Var || id.Obj.Decl == own || slices.Contains(orphans, id.Name) {
				return true
			}
			if d, ok := id.Obj.Decl.(ast.Node); ok && d.Pos() >= st.Pos() && d.End() <= st.End() {
				return true // declared in the clause itself
			}
			if !slices.ContainsFunc(body.List, func(live ast.Stmt) bool { return mentions(live, id.Name) }) {
				orphans = append(orphans, id.Name)
			}
			return true
		})
	}
	if len(orphans) > 0 {
		at := body.Lbrace + 1
		def := &ast.CaseClause{Case: at, Colon: at}
		for _, o := range orphans {
			def.Body = append(def.Body, &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: at, Name: "_"}}, TokPos: at, Tok: token.ASSIGN, Rhs: []ast.Expr{&ast.Ident{NamePos: at, Name: o}}})
		}
		if i := slices.IndexFunc(body.List, func(st ast.Stmt) bool { return st.(*ast.CaseClause).List == nil }); i >= 0 {
			cc := body.List[i].(*ast.CaseClause)
			cc.Body = append(def.Body, cc.Body...)
		} else {
			body.List = append(body.List, def)
		}
	}
	if len(body.List) == 0 {
		body.Rbrace = body.Lbrace + 1
	} else {
		body.Rbrace = body.List[len(body.List)-1].End()
	}
	return true
}

// inlineDispatcher rewrites a dispatcher's type switch (dispatch.go) left with one arm into an `if`, or with none into
// nothing, for the classes the program makes; done is false for any other switch.
func inlineDispatcher(ts *ast.TypeSwitchStmt) (ast.Stmt, bool) {
	a, isAssign := ts.Assign.(*ast.AssignStmt)
	if !isAssign || ts.Init != nil || len(ts.Body.List) > 1 {
		return nil, false
	}
	if len(ts.Body.List) == 0 {
		return nil, true // the guard is an identifier (self): nothing to keep
	}
	cc := ts.Body.List[0].(*ast.CaseClause)
	if len(cc.List) != 1 {
		return nil, false
	}
	v := a.Lhs[0].(*ast.Ident).Name
	if !mentions(cc, v) {
		v = "_" // an if's variable must be used; a switch's need not
	}
	ok := &ast.Ident{NamePos: ts.Pos(), Name: "ok_"} // not a Ruby parameter's name (goLocalName doubles a trailing _)
	init := &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: ts.Pos(), Name: v}, ok}, TokPos: ts.Pos(), Tok: token.DEFINE,
		Rhs: []ast.Expr{&ast.TypeAssertExpr{X: a.Rhs[0].(*ast.TypeAssertExpr).X, Type: cc.List[0]}}}
	return &ast.IfStmt{If: ts.Pos(), Init: init, Cond: ok, Body: &ast.BlockStmt{Lbrace: ts.Pos(), List: cc.Body, Rbrace: ts.End() - 1}}, true
}

// inlineDefaultOnly replaces a switch whose cases were all dropped with its default body (gocritic's singleCaseSwitch); the guard's expression stays as `_ = e` in case it has effects.
func inlineDefaultOnly(list []ast.Stmt) []ast.Stmt {
	out := make([]ast.Stmt, 0, len(list))
	for _, st := range list {
		if es, ok := st.(*ast.SwitchStmt); ok && len(es.Body.List) == 1 && es.Body.List[0].(*ast.CaseClause).List == nil && es.Init == nil {
			// an expression switch whose cases were all dropped (a _DynGen switch, decision 209)
			if _, ident := es.Tag.(*ast.Ident); es.Tag != nil && !ident {
				out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: es.Pos(), Name: "_"}}, TokPos: es.Pos(), Tok: token.ASSIGN, Rhs: []ast.Expr{es.Tag}})
			}
			if body := es.Body.List[0].(*ast.CaseClause).Body; len(body) > 0 {
				out = append(out, &ast.BlockStmt{Lbrace: es.Pos(), List: body, Rbrace: es.End() - 1})
			}
			continue
		}
		ts, ok := st.(*ast.TypeSwitchStmt)
		if ok {
			if rewritten, done := inlineDispatcher(ts); done {
				if rewritten != nil {
					out = append(out, rewritten)
				}
				continue
			}
		}
		if !ok || len(ts.Body.List) != 1 || ts.Body.List[0].(*ast.CaseClause).List != nil {
			out = append(out, st)
			continue
		}
		var body []ast.Stmt
		if ts.Init != nil {
			body = append(body, ts.Init)
		}
		var guard ast.Expr
		switch a := ts.Assign.(type) {
		case *ast.ExprStmt:
			guard = a.X.(*ast.TypeAssertExpr).X
		case *ast.AssignStmt:
			guard = a.Rhs[0].(*ast.TypeAssertExpr).X
			if mentions(ts.Body.List[0], a.Lhs[0].(*ast.Ident).Name) { // a default clause's variable is the guard itself
				body = append(body, &ast.AssignStmt{Lhs: a.Lhs, TokPos: ts.Pos(), Tok: token.DEFINE, Rhs: []ast.Expr{guard}})
				guard = nil
			}
		}
		if _, ident := guard.(*ast.Ident); guard != nil && !ident {
			body = append(body, &ast.AssignStmt{Lhs: []ast.Expr{&ast.Ident{NamePos: ts.Pos(), Name: "_"}}, TokPos: ts.Pos(), Tok: token.ASSIGN, Rhs: []ast.Expr{guard}})
		}
		body = append(body, ts.Body.List[0].(*ast.CaseClause).Body...)
		if len(body) > 0 {
			out = append(out, &ast.BlockStmt{Lbrace: ts.Pos(), List: body, Rbrace: ts.End() - 1})
		}
	}
	return out
}

// mentions reports whether n uses the identifier name anywhere.
func mentions(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

// visitSwitch visits an expression switch, parking its rbKeyed cases like weak type-switch cases.
func (p *pruner) visitSwitch(x *ast.SwitchStmt) {
	if x.Init != nil {
		ast.Inspect(x.Init, p.visit)
	}
	if x.Tag != nil {
		ast.Inspect(x.Tag, p.visit)
	}
	dynGen := false
	if id, ok := x.Tag.(*ast.Ident); ok && id.Name == "rbDynName" {
		dynGen = true // a generic class's _DynGen switch (decision 209): a case lives while its name's dispatcher does
	}
	for _, st := range x.Body.List {
		cc := st.(*ast.CaseClause)
		if lit, ok := singleString(cc); ok && dynGen {
			if d := "rbDyn" + goMethodName(strings.TrimPrefix(lit, dynPrivKey)); !p.names[d] {
				p.pending[cc] = []string{d}
				continue
			}
		}
		if t := keyedType(cc); t != nil {
			if names := p.caseTypeNames(t); len(names) > 0 {
				p.pending[cc] = names
				continue
			}
			if p.undeclaredType(t) {
				p.pending[cc] = []string{""} // a lib's type the program never loaded (decision 155)
				continue
			}
		}
		ast.Inspect(cc, p.visit)
	}
}

// singleString is the Ruby name in a `case "name":` clause.
func singleString(cc *ast.CaseClause) (string, bool) {
	if len(cc.List) != 1 {
		return "", false
	}
	bl, ok := cc.List[0].(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(bl.Value)
	return v, err == nil
}

// keyedType is T in a `case rbKeyed[T](k):` clause, which names T as weakly as a type-switch case does (decision 137).
func keyedType(cc *ast.CaseClause) ast.Expr {
	if len(cc.List) != 1 {
		return nil
	}
	call, ok := cc.List[0].(*ast.CallExpr)
	if !ok {
		return nil
	}
	ix, ok := call.Fun.(*ast.IndexExpr)
	if !ok {
		return nil
	}
	if id, ok := ix.X.(*ast.Ident); !ok || id.Name != "rbKeyed" {
		return nil
	}
	return ix.Index
}

// unkeyCases drops the rbKeyed hint once read: constant cases compile to a binary search with no per-class instantiation (#96).
func unkeyCases(sw *ast.SwitchStmt) {
	seen := map[string]bool{}
	sw.Body.List = slices.DeleteFunc(sw.Body.List, func(st ast.Stmt) bool {
		cc, ok := st.(*ast.CaseClause)
		if !ok || keyedType(cc) == nil {
			return false
		}
		lit := cc.List[0].(*ast.CallExpr).Args[0] //nolint:forcetypeassert // keyedType checked the shape
		key, _ := strconv.Unquote(lit.(*ast.BasicLit).Value)
		if seen[key] {
			return true // two classes with one tag (an alias): the first case won as a call, and Go forbids it as a constant
		}
		seen[key] = true
		cc.List[0] = lit
		return false
	})
}

// caseTypeNames lists the declared names a type-switch case's types use that kept code has not named yet.
func (p *pruner) caseTypeNames(types ...ast.Expr) []string {
	var out []string
	for _, e := range types {
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && len(p.byName[id.Name]) > 0 && !p.names[id.Name] && !slices.Contains(out, id.Name) {
				out = append(out, id.Name)
			}
			return true
		})
	}
	return out
}

// isTupleName is a tuple type's Go name, Tuple2 and up.
func isTupleName(s string) bool {
	n, ok := strings.CutPrefix(s, "Tuple")
	_, err := strconv.Atoi(n)
	return ok && n != "" && err == nil
}

// undeclaredType reports a case type naming a capitalized identifier nothing declares: a class from a prelude lib the
// program did not require (decision 155), so `*BigDecimal` in a core helper's switch would be `undefined`. Type
// parameters and declared names have an Obj or a declaration; std packages are selectors.
func (p *pruner) undeclaredType(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			return false
		case *ast.Ident:
			// TupleN is declared in a later batch than the code naming it, and always where a value of it exists (decision 241)
			if x.Obj == nil && len(p.byName[x.Name]) == 0 && x.Name != "" && unicode.IsUpper(rune(x.Name[0])) && !isTupleName(x.Name) {
				found = true
			}
		}
		return !found
	})
	return found
}

// sweep deletes unkept declarations and the comments (//line directives included) no longer inside one; a stub's body becomes a panic.
func (p *pruner) sweep(f *ast.File) {
	for _, d := range f.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.TypeSwitchStmt:
				p.dropDeadCases(n)
			case *ast.SwitchStmt:
				p.dropDeadClauses(n.Body, nil)
				unkeyCases(n)
			}
			return true
		})
		ast.Inspect(d, func(n ast.Node) bool { // after every switch is slimmed: one left with only `default` becomes its body
			switch n := n.(type) {
			case *ast.BlockStmt:
				n.List = inlineDefaultOnly(n.List)
			case *ast.CaseClause:
				n.Body = inlineDefaultOnly(n.Body)
			case *ast.CommClause:
				n.Body = inlineDefaultOnly(n.Body)
			}
			return true
		})
	}
	var dead [][2]token.Pos // main's unreached constant assignments, each with the //line before it
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "main" {
			prev := fd.Body.Lbrace + 1
			fd.Body.List = slices.DeleteFunc(fd.Body.List, func(st ast.Stmt) bool {
				_, drop := p.deferred[st]
				if drop {
					dead = append(dead, [2]token.Pos{prev, st.End()})
				}
				prev = st.End()
				return drop
			})
		}
	}
	var dropped [][2]token.Pos // deleted declarations, or specs of kept ones: their comments go; in order and disjoint, for within's search
	f.Decls = slices.DeleteFunc(f.Decls, func(d ast.Decl) bool {
		span := [2]token.Pos{declStart(d), d.End()} // before the specs go: a GenDecl's End reads its last spec
		drop := false
		var specs [][2]token.Pos
		switch d := d.(type) {
		case *ast.FuncDecl:
			drop = !p.kept[d]
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				d.Specs = slices.DeleteFunc(d.Specs, func(s ast.Spec) bool {
					if !p.kept[s] {
						specs = append(specs, [2]token.Pos{s.Pos(), s.End()})
					}
					return !p.kept[s]
				})
				drop = len(d.Specs) == 0
			}
		}
		if drop {
			dropped = append(dropped, span)
		} else {
			dropped = append(dropped, specs...)
		}
		return drop
	})
	p.slimInterfaces(f)
	for d := range p.stubs {
		if d.Body != nil { // `{ panic(...) }` on the brace's line; the old body's //line comments now sit between declarations
			at := d.Body.Lbrace + 1
			d.Body = &ast.BlockStmt{Lbrace: d.Body.Lbrace, Rbrace: at + 1, List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{
				Fun: &ast.Ident{NamePos: at, Name: "panic"}, Lparen: at, Rparen: at,
				Args: []ast.Expr{&ast.BasicLit{ValuePos: at, Kind: token.STRING, Value: `"rugaroo: pruned"`}},
			}}}}
		}
	}
	spans := make([][2]token.Pos, 0, len(f.Decls))
	for _, d := range f.Decls {
		spans = append(spans, [2]token.Pos{declStart(d), d.End()})
	}
	within := func(c *ast.CommentGroup, spans [][2]token.Pos) bool { // spans are sorted and disjoint: the candidate is the last one starting at or before c
		i, _ := slices.BinarySearchFunc(spans, c.Pos(), func(s [2]token.Pos, p token.Pos) int { return cmp.Compare(s[0], p) })
		if i < len(spans) && spans[i][0] == c.Pos() {
			return c.End() <= spans[i][1]
		}
		return i > 0 && c.End() <= spans[i-1][1]
	}
	f.Comments = slices.DeleteFunc(f.Comments, func(c *ast.CommentGroup) bool {
		switch {
		case within(c, dead):
			return true
		case within(c, spans), c.Pos() < f.Name.End(): // inside a kept declaration, or the header above the package clause
			return false
		case within(c, dropped):
			return true
		}
		// a //line directive between declarations maps the code after it, which may be kept
		return !strings.HasPrefix(c.List[0].Text, "//line ")
	})
}

func declStart(d ast.Decl) token.Pos {
	if fd, ok := d.(*ast.FuncDecl); ok && fd.Doc != nil {
		return fd.Doc.Pos()
	}
	if gd, ok := d.(*ast.GenDecl); ok && gd.Doc != nil {
		return gd.Doc.Pos()
	}
	return d.Pos()
}

// recvName is a method receiver's base type name: `*Foo[E]` → Foo.
func recvName(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

// isGenericForwarder reports whether d is a method on a generic type whose whole body calls the
// free func of the same name (`func (self *Array[E]) Size() Integer { return Array_Size[E](self) }`).
func isGenericForwarder(d *ast.FuncDecl) bool {
	recv := d.Recv.List[0].Type
	if s, ok := recv.(*ast.StarExpr); ok {
		recv = s.X
	}
	switch recv.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
	default:
		return false
	}
	if d.Body == nil || len(d.Body.List) != 1 {
		return false
	}
	var call ast.Expr
	switch st := d.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) != 1 {
			return false
		}
		call = st.Results[0]
	case *ast.ExprStmt:
		call = st.X
	default:
		return false
	}
	ce, ok := call.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun := ce.Fun
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = x.X
	case *ast.IndexListExpr:
		fun = x.X
	}
	id, ok := fun.(*ast.Ident)
	return ok && id.Name == recvName(recv)+"_"+d.Name.Name
}

// explain writes one line per kept declaration: its name, then the chain of nodes whose visits kept it, back to a root.
func (p *pruner) explain(f *ast.File, w io.Writer) {
	name := func(n ast.Node) string {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Recv != nil {
				return recvName(n.Recv.List[0].Type) + "." + n.Name.Name
			}
			return n.Name.Name
		case *ast.TypeSpec:
			return n.Name.Name
		case *ast.ValueSpec:
			return n.Names[0].Name
		case *ast.CaseClause:
			return "case"
		case ast.Stmt: // one of main's, queued one by one (lazyConst)
			call := "main"
			ast.Inspect(n, func(x ast.Node) bool {
				if c, ok := x.(*ast.CallExpr); ok && call == "main" {
					if id, ok := c.Fun.(*ast.Ident); ok {
						call = "main:" + id.Name
					}
				}
				return call == "main"
			})
			return call
		}
		return fmt.Sprintf("%T", n)
	}
	for _, d := range f.Decls {
		var nodes []ast.Node
		switch d := d.(type) {
		case *ast.FuncDecl:
			nodes = []ast.Node{d}
		case *ast.GenDecl:
			nodes = make([]ast.Node, 0, len(d.Specs))
			for _, s := range d.Specs {
				nodes = append(nodes, s)
			}
		}
		for _, n := range nodes {
			chain := []string{name(n)}
			for seen := map[ast.Node]bool{n: true}; ; {
				n = p.why[n]
				if n == nil || seen[n] {
					break
				}
				seen[n] = true
				chain = append(chain, name(n))
			}
			_, _ = fmt.Fprintln(w, strings.Join(chain, " <- "))
		}
	}
}
