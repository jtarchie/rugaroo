package compiler

import (
	"context"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// collect declares the classes, modules, methods and constants of f's top-level statements (the first pass over a file).
func (c *Compiler) collect(ctx context.Context, f *File) {
	if c.gemFile(f) {
		lenientBlockParams(f)
	}
	queue := f.Root.Statements.Body
	locals := map[string]bool{} // the top level's, so far
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if w, ok := n.(*parser.LocalVariableWriteNode); ok {
			locals[w.Name] = true
		}
		c.keepGoing(func() {
			switch n := n.(type) {
			case *parser.IfNode, *parser.UnlessNode: // a condition known at compile time keeps one branch (decision 167)
				if branch, ok := c.takenBranch(f, n, nil); ok {
					queue = append(slices.Clip(branch), queue...)
					return
				}
				c.scanAnon(ctx, f, n, nil)
				c.addMainStmt(f, n)
			case *parser.ClassNode:
				c.collectClass(ctx, f, n, nil)
			case *parser.ModuleNode:
				c.collectModule(ctx, f, n, nil)
			case *parser.DefNode:
				c.collectTopDef(f, n)
				c.scanDefBody(ctx, f, n.Body, nil)
			case *parser.ConstantWriteNode:
				if call, kind := valueClass(n.Value); call != nil {
					c.collectValueClass(ctx, f, n, call, kind, nil)
					return
				}
				if call, isModule := anonClassCall(n.Value); call != nil && !f.isPrelude() {
					c.collectNamedAnon(ctx, f, n, call, isModule, nil)
					return
				}
				c.scanAnon(ctx, f, n.Value, nil)
				c.addConst(f, n, n.Name, nil)
				if readsLocal(n.Value, locals) { // `dir = __dir__ || raise; LIB = Pathname(dir)`: runs where it stands, with them
					c.consts[n.Name].inMain = true
					c.addMainStmt(f, n)
				}
			case *parser.ConstantPathWriteNode: // `Rack::X = v` at the top level
				c.scanAnon(ctx, f, n.Value, nil)
				c.addConstAt(f, n, c.declName(f, n.Target, nil), n.Value, nil)
			case *parser.PreExecutionNode: // BEGIN runs before the rest of its file, in the top-level scope
				if c.beginStmts == nil {
					c.beginStmts = map[*File][]parser.Node{}
				}
				if n.Statements != nil {
					c.beginStmts[f] = append(c.beginStmts[f], n.Statements.Body...)
				}
			case *parser.XStringNode:
				if !f.isPrelude() {
					c.errorf(f, n, "top-level %%x{} is only allowed in the prelude")
				}
				c.verbatim = append(c.verbatim, verbatim{srcPos: srcPos{f, f.line(n.Location.StartOffset)}, code: n.Unescaped.Value})
			case *parser.CallNode:
				c.collectTopCall(ctx, f, n)
			case *parser.BeginNode:
				c.addMainStmt(f, c.beginRequires(ctx, f, n))
			default:
				c.scanAnon(ctx, f, n, nil)
				c.addMainStmt(f, n)
			}
		})
	}
}
