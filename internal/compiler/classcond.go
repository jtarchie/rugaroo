package compiler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// rubyVersion is what class-body RUBY_VERSION guards fold against: rugaroo compiles Ruby 4.0 (decision 167).
const rubyVersion = "4.0.0"

// takenBranch is the branch MRI would run of a class-body if/unless whose condition folds (decision 167).
func (c *Compiler) takenBranch(f *File, n parser.Node, scope []*Class) (stmts []parser.Node, ok bool) {
	switch n := n.(type) {
	case *parser.IfNode:
		v, ok := c.foldCond(f, n.Predicate, scope)
		if !ok && c.gemFile(f) { // boot's answer folds gem code only (decision 201): a program's own `if ENV["PORT"]` is decided when it runs
			v, ok = c.bootTook(f, n.Location, false)
		}
		if !ok {
			return nil, false
		}
		if v {
			return statements(n.Statements), true
		}
		switch s := n.Subsequent.(type) {
		case *parser.ElseNode:
			return statements(s.Statements), true
		case *parser.IfNode: // elsif
			return c.takenBranch(f, s, scope)
		}
		return nil, true
	case *parser.UnlessNode:
		v, ok := c.foldCond(f, n.Predicate, scope)
		if !ok && c.gemFile(f) {
			v, ok = c.bootTook(f, n.Location, true)
		}
		if !ok {
			return nil, false
		}
		if !v {
			return statements(n.Statements), true
		}
		if n.ElseClause != nil {
			return statements(n.ElseClause.Statements), true
		}
		return nil, true
	}
	return nil, false
}

func statements(s *parser.StatementsNode) []parser.Node {
	if s == nil {
		return nil
	}
	return s.Body
}

// foldCond checks defined? against the world collected so far: what MRI sees at that point of the load.
func (c *Compiler) foldCond(f *File, n parser.Node, scope []*Class) (value, ok bool) {
	switch n := n.(type) {
	case *parser.TrueNode:
		return true, true
	case *parser.FalseNode, *parser.NilNode:
		return false, true
	case *parser.ParenthesesNode:
		if s, isStmts := n.Body.(*parser.StatementsNode); isStmts && len(s.Body) == 1 {
			return c.foldCond(f, s.Body[0], scope)
		}
	case *parser.AndNode:
		l, ok := c.foldCond(f, n.Left, scope)
		if !ok || !l {
			return false, ok
		}
		return c.foldCond(f, n.Right, scope)
	case *parser.OrNode:
		l, ok := c.foldCond(f, n.Left, scope)
		if !ok || l {
			return l, ok
		}
		return c.foldCond(f, n.Right, scope)
	case *parser.CallNode:
		if n.Name == "!" && n.Receiver != nil && n.Arguments == nil {
			v, ok := c.foldCond(f, n.Receiver, scope)
			return !v, ok
		}
		if args := callArgs(n); n.Name == "respond_to?" && len(args) == 1 && n.Block == nil {
			if sym, isSym := args[0].(*parser.SymbolNode); isSym && n.Receiver == nil && len(scope) > 0 {
				// a class body's own `respond_to?(:to_a)`: what the class object answers (drb's `undef :to_a if respond_to?(:to_a)`,
				// decision 416): its singleton methods, else Class's (Module's, Object's, Kernel's)
				// read the method tables directly: lookup at collect time would cache them before link adds the includes
				name := sym.Unescaped.Value
				if m := scope[len(scope)-1].meta; m != nil && m.Methods[name] != nil {
					return true, true
				}
				for _, k := range []string{"Class", "Module", "Object", "Kernel", "BasicObject"} {
					if cls := c.classes[k]; cls != nil && cls.Methods[name] != nil {
						return true, true
					}
				}
				return false, true
			}
			if sym, isSym := args[0].(*parser.SymbolNode); isSym { // `Const.respond_to?(:m)`, as defined?(Const.m)
				return c.foldDefined(f, &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: sym.Unescaped.Value}, scope)
			}
		}
		return foldVersion(n)
	case *parser.DefinedNode:
		return c.foldDefined(f, n.Value, scope)
	}
	return false, false
}

// foldVersion compares as strings, not versions, because MRI does ("10.0" < "4.0").
// RUBY_ENGINE is "ruby" (prelude/io.rb): `RUBY_ENGINE == "truffleruby"` folds too (prism's serialize.rb, decision 427).
func foldVersion(n *parser.CallNode) (value, ok bool) {
	args := callArgs(n)
	engine := isTopConstNamed(n.Receiver, "RUBY_ENGINE")
	if len(args) != 1 || !isTopConstNamed(n.Receiver, "RUBY_VERSION") && !engine {
		return false, false
	}
	s, isStr := args[0].(*parser.StringNode)
	if !isStr {
		return false, false
	}
	v := s.Unescaped.Value
	if engine {
		switch n.Name {
		case "==":
			return v == "ruby", true
		case "!=":
			return v != "ruby", true
		}
		return false, false
	}
	switch n.Name {
	case ">=":
		return rubyVersion >= v, true
	case ">":
		return rubyVersion > v, true
	case "<=":
		return rubyVersion <= v, true
	case "<":
		return rubyVersion < v, true
	case "==":
		return rubyVersion == v, true
	case "!=":
		return rubyVersion != v, true
	}
	return false, false
}

func isTopConstNamed(n parser.Node, name string) bool {
	switch n := n.(type) {
	case *parser.ConstantReadNode:
		return n.Name == name
	case *parser.ConstantPathNode:
		return n.Parent == nil && *n.Name == name
	}
	return false
}

// foldDefined folds defined?(Const), defined?(A::B) and defined?(Const.meth).
func (c *Compiler) foldDefined(f *File, n parser.Node, scope []*Class) (value, ok bool) {
	switch n := n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, k := c.lookupConst(f, n, scope)
		name := constText(f, n)
		return cls != nil || k != nil || c.autoloads[name] || c.autoloads[qualify(scope, name)], true
	case *parser.CallNode:
		switch n.Receiver.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
		default:
			return false, false
		}
		if n.Arguments != nil || n.Block != nil {
			return false, false
		}
		cls, _ := c.lookupConst(f, n.Receiver, scope)
		if cls != nil && (cls.RubyName == "Data" && n.Name == "define" || cls.RubyName == "Struct" && n.Name == "new") {
			return true, true // compiled forms (valueClass), not methods
		}
		return cls != nil && c.hasClassMethod(cls, n.Name), true
	}
	return false, false
}

// hasClassMethod reports whether cls or a superclass declares `def self.name`.
// ponytail: methods from `extend` and Module's own instance methods read as
// absent; walk cls.extends and Module when a guard needs them.
func (c *Compiler) hasClassMethod(cls *Class, name string) bool {
	for k := cls; k != nil; {
		for _, d := range k.singletonDefs {
			if d.node.Name == name {
				return true
			}
		}
		switch {
		case k.Super != nil:
			k = k.Super
		case k.superRef != nil:
			k = c.resolveClassRef(k.superRef)
		default:
			k = nil
		}
	}
	return false
}

// absentGuard reports whether pred is `defined?(C)`, or an `&&` led by one, for a constant no file defines: false at
// run time as well, since the closed world is the whole program, so the guarded code (which names C) is dropped.
func (c *Compiler) absentGuard(f *File, pred parser.Node, scope []*Class) bool {
	switch p := pred.(type) {
	case *parser.ParenthesesNode:
		if s, ok := p.Body.(*parser.StatementsNode); ok && len(s.Body) == 1 {
			return c.absentGuard(f, s.Body[0], scope)
		}
	case *parser.AndNode:
		return c.absentGuard(f, p.Left, scope)
	case *parser.DefinedNode:
		switch v := p.Value.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			var cls *Class
			var k *Const
			ce := catchCompileError(func() { cls, k = c.lookupConst(f, v, scope) })
			return ce == nil && cls == nil && k == nil
		}
	}
	return false
}

// loadAutoload loads a boot-resolved autoload's file where it registers, so its load-time code runs before main's (decision 175).
func (c *Compiler) loadAutoload(ctx context.Context, f *File, n *parser.CallNode, scope []*Class) {
	args := callArgs(n)
	if len(args) != 2 || f.isPrelude() {
		return
	}
	target := c.findRequire(f, &parser.CallNode{Location: n.Location, Name: "require", Arguments: &parser.ArgumentsNode{Arguments: args[1:]}})
	if p, ok := staticPath(f, args[1]); target == "" && ok && filepath.IsAbs(p) {
		if filepath.Ext(p) != ".rb" {
			p += ".rb"
		}
		info, err := os.Stat(p) //nolint:gosec // a path the program names
		if err == nil && !info.IsDir() {
			target = p
		}
	}
	if target == "" {
		if !c.gemFile(f) {
			c.errorf(f, n, "autoload: cannot load such file -- %s (an autoload is a require at compile time, decision 195)", f.text(args[1].GetLocation()))
		}
		if sym, ok := symbolName(args[0]); ok { // a computed path (unicode-emoji's `File.join(dir, name.downcase)`): the file that defines the constant, once loaded
			c.pendingAutoloads = append(c.pendingAutoloads, pendingAutoload{f: f, n: n, full: qualify(scope, sym)})
		}
		return
	}
	if sym, _ := symbolName(args[0]); c.gemFile(f) && (c.booted || c.bundledFile(f)) && !c.autoloaded[c.real(target)] && !c.backendNamed(sym) {
		return // a gem autoload boot never loaded: nothing the program runs reaches it (decision 175)
	} // without a boot (cmd/rubyspec's in-memory suites) it loads where it registers, as a require (matrix's LUPDecomposition, decision 414)
	src, err := os.ReadFile(target) //nolint:gosec // a gem file boot loaded
	if err != nil {
		c.errorf(f, n, "cannot load such file -- %s", strings.TrimSuffix(target, ".rb"))
	}
	if uf := c.loadUserFile(ctx, target, src); uf != nil {
		c.addMainStmt(f, &loadFile{call: n, file: uf})
	}
}

// pendingAutoload is a gem's `autoload :X, computed_path` (decision 175): X's file is placed where it registers.
type pendingAutoload struct {
	f    *File
	n    *parser.CallNode
	full string
}

// placePendingAutoloads moves each pending autoload's file, when another require (a traced entry's, decision 296) loaded
// it, to where the autoload registers, as a resolved autoload loads (decision 175): unicode-display_width's class body
// reads Unicode::Emoji::REGEX_TEXT_PRESENTATION, which MRI autoloaded there and the traced entry requires only later.
func (c *Compiler) placePendingAutoloads() {
	for _, p := range c.pendingAutoloads {
		var file *File
		if k := c.consts[p.full]; k != nil {
			file = k.File
		} else if cls := c.classes[p.full]; cls != nil {
			file = cls.File
		}
		if file == nil || file == p.f {
			continue
		}
		for _, s := range c.mainStmts {
			if lf, ok := s.(*loadFile); ok && lf.file == file && c.stmtFile[lf] != p.f {
				c.stmtFile[lf], lf.call = p.f, p.n
				break
			}
		}
	}
}

// backendNamed reports a constant a loaded gem-backend prelude file names (prism's Serialize): MRI's boot ran the gem's C
// extension instead, so it never resolved that autoload, but the backend needs it (decision 427).
func (c *Compiler) backendNamed(name string) bool {
	for _, f := range c.files {
		if gemBackends[f.Name] && anyNode(f.Root, func(n parser.Node) bool {
			r, ok := n.(*parser.ConstantReadNode)
			return ok && r.Name == name
		}) {
			return true
		}
	}
	return false
}

// staticPath folds a path expression known at compile time: a String, `__dir__`, `File.join(...)`,
// `File.expand_path(name, dir)` and `"#{__dir__}/x"` over those (decision 195).
func staticPath(f *File, n parser.Node) (string, bool) {
	dir := filepath.Dir(f.path)
	if f.path == "" {
		return "", false
	}
	switch x := n.(type) {
	case *parser.StringNode:
		return x.Unescaped.Value, true
	case *parser.InterpolatedStringNode:
		var b strings.Builder
		for _, part := range x.Parts {
			switch p := part.(type) {
			case *parser.StringNode:
				b.WriteString(p.Unescaped.Value)
			case *parser.EmbeddedStatementsNode:
				if p.Statements == nil || len(p.Statements.Body) != 1 {
					return "", false
				}
				v, ok := staticPath(f, p.Statements.Body[0])
				if !ok {
					return "", false
				}
				b.WriteString(v)
			default:
				return "", false
			}
		}
		return b.String(), true
	case *parser.CallNode:
		args := callArgs(x)
		switch {
		case x.Receiver == nil && x.Name == "__dir__" && len(args) == 0:
			return dir, true
		case isConstNamed(x.Receiver, "File") && (x.Name == "join" || x.Name == "expand_path"):
			parts := make([]string, 0, len(args))
			for _, a := range args {
				v, ok := staticPath(f, a)
				if !ok {
					return "", false
				}
				parts = append(parts, v)
			}
			if x.Name == "expand_path" && len(parts) == 2 && !filepath.IsAbs(parts[0]) {
				return filepath.Clean(filepath.Join(parts[1], parts[0])), true
			}
			if x.Name == "expand_path" && len(parts) == 1 && !filepath.IsAbs(parts[0]) {
				return filepath.Clean(filepath.Join(dir, parts[0])), true // MRI's is the working directory: the program's own directory stands in
			}
			return filepath.Join(parts...), true
		}
	}
	return "", false
}

func isConstNamed(n parser.Node, name string) bool {
	switch c := n.(type) {
	case *parser.ConstantReadNode:
		return c.Name == name
	case *parser.ConstantPathNode:
		return c.Parent == nil && c.Name != nil && *c.Name == name
	}
	return false
}

// noteAutoload records an `autoload :X` constant: defined? answers for it before its file loads, as in MRI.
func (c *Compiler) noteAutoload(full string) {
	if c.autoloads == nil {
		c.autoloads = map[string]bool{}
	}
	c.autoloads[full] = true
}

// constText is a constant reference's qualified spelling, without a leading `::`.
func constText(f *File, n parser.Node) string {
	return strings.TrimPrefix(f.text(n.GetLocation()), "::")
}

func firstArg(args []parser.Node) parser.Node {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

func lastNode(args []parser.Node) parser.Node {
	if len(args) == 0 {
		return nil
	}
	return args[len(args)-1]
}

func (c *Compiler) indexBootBranches(bs []boot.BranchInfo) {
	c.bootBranches = map[branchKey]boot.BranchInfo{}
	for _, b := range bs {
		c.bootBranches[branchKey{Loc: boot.Loc{File: realPath(b.File), Line: b.Line}, col: b.Col}] = b
	}
}

// bootTook is the condition value MRI's load phase saw for the if (or unless) at loc, when boot ran it (decision 201).
func (c *Compiler) bootTook(f *File, loc parser.Location, unless bool) (value, ok bool) {
	b, seen := c.bootBranches[f.branchKey(loc.StartOffset)]
	switch {
	case !seen || b.Then > 0 && b.Else > 0 || b.Then == 0 && b.Else == 0:
		return false, false // not reached, or both arms ran (in a method body run twice): no single answer
	case unless:
		return b.Then == 0, true
	}
	return b.Then > 0, true
}

// loadTimeArm is the statements of the arm MRI's load phase ran of an if/unless/case (a constant's value): folded,
// or as boot saw it (decision 201).
func (c *Compiler) loadTimeArm(f *File, n parser.Node, scope []*Class) ([]parser.Node, bool) {
	switch n := n.(type) {
	case *parser.IfNode, *parser.UnlessNode:
		return c.takenBranch(f, n, scope)
	case *parser.CaseNode:
		b, seen := c.bootBranches[f.branchKey(n.Location.StartOffset)]
		if !seen {
			return nil, false
		}
		var ran []string
		for _, a := range b.Arms {
			if a.Count > 0 {
				ran = append(ran, fmt.Sprintf("%d:%d", a.Line, a.Col))
			}
		}
		if len(ran) != 1 {
			return nil, false
		}
		at := func(st *parser.StatementsNode) bool {
			if st == nil {
				return false
			}
			off := st.Location.StartOffset
			l := f.line(off)
			return fmt.Sprintf("%d:%d", l, off-f.lines[l-1]) == ran[0]
		}
		for _, w := range n.Conditions {
			if wn, ok := w.(*parser.WhenNode); ok && at(wn.Statements) {
				return wn.Statements.Body, true
			}
		}
		if n.ElseClause != nil && at(n.ElseClause.Statements) {
			return n.ElseClause.Statements.Body, true
		}
	}
	return nil, false
}
