package compiler

import (
	"go/ast"
	"go/token"
)

// inlineIdxSet: Array#[]= costs Go's inliner over its budget (the slow path's generic call alone is ~70), so a statement `Array_Op_idxSet[T](a, i, v)` gets the in-bounds store at the site and calls it only otherwise (decision 387).
func inlineIdxSet(f *ast.File) {
	if !preludeIdxSet(f) {
		return // a program's own `def []=`: every store must call it, as in MRI
	}
	ast.Inspect(f, func(n ast.Node) bool {
		var list []ast.Stmt
		switch b := n.(type) {
		case *ast.BlockStmt:
			list = b.List
		case *ast.CaseClause:
			list = b.Body
		case *ast.CommClause:
			list = b.Body
		default:
			return true
		}
		for i, s := range list {
			if r := idxSetFast(s); r != nil {
				list[i] = r
			}
		}
		return true
	})
}

// preludeIdxSet is whether Array_Op_idxSet is the prelude's body (its slow path calls rbArrayIdxSet), not a program's override.
func preludeIdxSet(f *ast.File) bool {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name.Name != "Array_Op_idxSet" || fd.Body == nil {
			continue
		}
		found := false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && isIdent(c.Fun, "rbArrayIdxSet") {
				found = true
			}
			return !found
		})
		return found
	}
	return false
}

func idxSetFast(s ast.Stmt) ast.Stmt {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 3 || call.Ellipsis.IsValid() {
		return nil
	}
	ix, ok := call.Fun.(*ast.IndexExpr)
	if !ok || !isIdent(ix.X, "Array_Op_idxSet") || isIdent(call.Args[0], "rbIdxA") { // rbIdxA: the slow call already rewritten
		return nil
	}
	pos := call.Pos()
	id := func(name string) *ast.Ident { return &ast.Ident{NamePos: pos, Name: name} }
	a, i, v := "rbIdxA", "rbIdxI", "rbIdxV"
	sel := func(x, f string) ast.Expr { return &ast.SelectorExpr{X: id(x), Sel: id(f)} }
	conv := func(t string, x ast.Expr) ast.Expr {
		return &ast.CallExpr{Fun: id(t), Lparen: pos, Args: []ast.Expr{x}, Rparen: pos}
	}
	inBounds := &ast.BinaryExpr{X: conv("uint64", id(i)), OpPos: pos, Op: token.LSS, Y: conv("uint64", conv("len", sel(a, "s")))}
	notFrozen := &ast.UnaryExpr{OpPos: pos, Op: token.NOT, X: &ast.CallExpr{Fun: &ast.SelectorExpr{X: sel(a, "frozen"), Sel: id("Load")}, Lparen: pos, Rparen: pos}}
	store := &ast.AssignStmt{Lhs: []ast.Expr{&ast.IndexExpr{X: sel(a, "s"), Lbrack: pos, Index: id(i), Rbrack: pos}}, TokPos: pos, Tok: token.ASSIGN, Rhs: []ast.Expr{id(v)}}
	slow := &ast.ExprStmt{X: &ast.CallExpr{Fun: call.Fun, Lparen: pos, Args: []ast.Expr{id(a), id(i), id(v)}, Rparen: pos}}
	return &ast.IfStmt{
		If:   pos,
		Init: &ast.AssignStmt{Lhs: []ast.Expr{id(a), id(i), id(v)}, TokPos: pos, Tok: token.DEFINE, Rhs: []ast.Expr{headSafe(call.Args[0]), typedConst(call.Args[1], id("Integer")), headSafe(typedConst(call.Args[2], ix.Index))}},
		Cond: &ast.BinaryExpr{X: inBounds, OpPos: pos, Op: token.LAND, Y: notFrozen},
		Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{store}, Rbrace: pos},
		Else: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{slow}, Rbrace: call.End()},
	}
}

// headSafe parenthesizes a composite literal, which an if header would otherwise read as its body (`out[0] = [7, 7]` into a tuple Array).
func headSafe(x ast.Expr) ast.Expr {
	lit := x
	if u, ok := x.(*ast.UnaryExpr); ok {
		lit = u.X
	}
	if _, ok := lit.(*ast.CompositeLit); ok {
		return &ast.ParenExpr{X: x}
	}
	return x
}

func isIdent(x ast.Expr, name string) bool {
	id, ok := x.(*ast.Ident)
	return ok && id.Name == name
}

// typedConst gives an untyped constant argument its parameter's type, which a := would make Go's int or reject (nil).
func typedConst(x, t ast.Expr) ast.Expr {
	switch v := x.(type) {
	case *ast.BasicLit:
	case *ast.UnaryExpr:
		if _, ok := v.X.(*ast.BasicLit); !ok {
			return x
		}
	case *ast.Ident:
		if v.Name != "nil" && v.Name != "true" && v.Name != "false" {
			return x
		}
	default:
		return x
	}
	if _, ok := t.(*ast.Ident); !ok {
		t = &ast.ParenExpr{X: t}
	}
	return &ast.CallExpr{Fun: t, Args: []ast.Expr{x}}
}

// inlinePush: a one-element `Array_Push`/`Array_Op_lshift` statement costs Go's inliner over its budget (push is variadic: 87), so the append runs at the site and the call only for a frozen Array (decision 391).
func inlinePush(f *ast.File) {
	own := map[string]bool{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && (fd.Name.Name == "Array_Push" || fd.Name.Name == "Array_Op_lshift") && preludeAppend(fd.Body) {
			own[fd.Name.Name] = true
		}
	}
	if len(own) == 0 {
		return
	}
	ast.Inspect(f, func(n ast.Node) bool {
		var list []ast.Stmt
		switch b := n.(type) {
		case *ast.BlockStmt:
			list = b.List
		case *ast.CaseClause:
			list = b.Body
		case *ast.CommClause:
			list = b.Body
		default:
			return true
		}
		for i, s := range list {
			if r := pushFast(s, own); r != nil {
				list[i] = r
			}
		}
		return true
	})
}

// preludeAppend is the prelude's body (frozen check, doubling growth, append, return self), not a program's own `def push`.
func preludeAppend(b *ast.BlockStmt) bool {
	if b == nil || len(b.List) != 4 {
		return false
	}
	if _, ok := b.List[1].(*ast.IfStmt); !ok {
		return false
	}
	as, ok := b.List[2].(*ast.AssignStmt)
	if !ok || len(as.Rhs) != 1 {
		return false
	}
	call, ok := as.Rhs[0].(*ast.CallExpr)
	return ok && isIdent(call.Fun, "append")
}

func pushFast(s ast.Stmt, own map[string]bool) ast.Stmt {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || call.Ellipsis.IsValid() {
		return nil
	}
	ix, ok := call.Fun.(*ast.IndexExpr)
	if !ok || isIdent(call.Args[0], "rbIdxA") {
		return nil
	}
	if fn, ok := ix.X.(*ast.Ident); !ok || !own[fn.Name] {
		return nil
	}
	pos := call.Pos()
	id := func(name string) *ast.Ident { return &ast.Ident{NamePos: pos, Name: name} }
	a, v := "rbIdxA", "rbIdxV"
	sel := func(x, f string) ast.Expr { return &ast.SelectorExpr{X: id(x), Sel: id(f)} }
	notFrozen := &ast.UnaryExpr{OpPos: pos, Op: token.NOT, X: &ast.CallExpr{Fun: &ast.SelectorExpr{X: sel(a, "frozen"), Sel: id("Load")}, Lparen: pos, Rparen: pos}}
	app := &ast.CallExpr{Fun: id("append"), Lparen: pos, Args: []ast.Expr{sel(a, "s"), id(v)}, Rparen: pos}
	store := &ast.AssignStmt{Lhs: []ast.Expr{sel(a, "s")}, TokPos: pos, Tok: token.ASSIGN, Rhs: []ast.Expr{app}}
	// Past 256 elements the backing array doubles, as the prelude's `<<` (decision 488): Go's own 1.25x growth copies an Array built by pushes about 4x over.
	// Not slices.Grow: each element type would instantiate it, one more compiled function per type.
	lenS := func() ast.Expr {
		return &ast.CallExpr{Fun: id("len"), Lparen: pos, Args: []ast.Expr{sel(a, "s")}, Rparen: pos}
	}
	full := &ast.BinaryExpr{X: lenS(), OpPos: pos, Op: token.EQL, Y: &ast.CallExpr{Fun: id("cap"), Lparen: pos, Args: []ast.Expr{sel(a, "s")}, Rparen: pos}}
	big := &ast.BinaryExpr{X: lenS(), OpPos: pos, Op: token.GEQ, Y: &ast.BasicLit{ValuePos: pos, Kind: token.INT, Value: "256"}}
	grow := &ast.IfStmt{If: pos, Cond: &ast.BinaryExpr{X: full, OpPos: pos, Op: token.LAND, Y: big}, Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{
		&ast.AssignStmt{Lhs: []ast.Expr{sel(a, "s")}, TokPos: pos, Tok: token.ASSIGN, Rhs: []ast.Expr{doubled(lenS, sel(a, "s"), ix.Index, pos)}},
	}, Rbrace: pos}}
	slow := &ast.ExprStmt{X: &ast.CallExpr{Fun: call.Fun, Lparen: pos, Args: []ast.Expr{id(a), id(v)}, Rparen: pos}}
	return &ast.IfStmt{
		If:   pos,
		Init: &ast.AssignStmt{Lhs: []ast.Expr{id(a), id(v)}, TokPos: pos, Tok: token.DEFINE, Rhs: []ast.Expr{headSafe(call.Args[0]), headSafe(typedConst(call.Args[1], ix.Index))}},
		Cond: notFrozen,
		Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{grow, store}, Rbrace: pos},
		Else: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{slow}, Rbrace: call.End()},
	}
}

// doubled is `append(s, make([]T, len(s))...)[:len(s)]` for a full s: Go's append-of-make extends without a temporary, to at least twice the capacity.
func doubled(lenS func() ast.Expr, s, t ast.Expr, pos token.Pos) ast.Expr {
	mk := &ast.CallExpr{Fun: &ast.Ident{NamePos: pos, Name: "make"}, Lparen: pos, Args: []ast.Expr{&ast.ArrayType{Lbrack: pos, Elt: t}, lenS()}, Rparen: pos}
	app := &ast.CallExpr{Fun: &ast.Ident{NamePos: pos, Name: "append"}, Lparen: pos, Args: []ast.Expr{s, mk}, Ellipsis: pos, Rparen: pos}
	return &ast.SliceExpr{X: app, Lbrack: pos, High: lenS(), Rbrack: pos}
}
