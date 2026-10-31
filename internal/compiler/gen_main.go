package compiler

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func (c *Compiler) emitMain() {
	c.w("var rb_main = &Object{}\n\n")
	c.labels["main"] = "<main>"
	c.w("func main() {\n\trbTrapSignals()\n\tdefer rbFlush()\n\tdefer rbTopRecover()\n")
	prelude, all := c.mainBodies()
	gen := func(file *File, stmts []parser.Node, indent int) string {
		f := c.newFctx(file, nil, nil)
		f.indent = indent
		f.retVar = ""
		f.genBody(&parser.StatementsNode{Body: stmts}, nil, tail{}, nil)
		return f.buf.String()
	}
	// a required file runs where its require_relative is, as a nested block
	// (decision 130); it comes later in load order than its requirer, so
	// generating backwards has its code ready for the loadFile that splices it
	c.loadCode = map[*File]string{}
	var bodies []fileBody
	for i := len(all) - 1; i >= 0; i-- {
		if c.required[all[i].f] {
			c.loadCode[all[i].f] = gen(all[i].f, all[i].stmts, 2)
		} else {
			bodies = append([]fileBody{all[i]}, bodies...)
		}
	}
	if len(bodies) == 1 { // one file: its top level is main's
		c.w("%s", gen(bodies[0].f, append(prelude, bodies[0].stmts...), 1))
	} else {
		c.w("%s", gen(c.mainFile, prelude, 1))
		for _, b := range bodies { // top-level locals are file-scoped in Ruby: a Go block each
			c.w("\t{\n%s\t}\n", gen(b.f, b.stmts, 2))
		}
	}
	c.w("}\n\n")
}

// emitSynth emits a metaclass's generated method as a Go method.
func (c *Compiler) emitSynth(m *Method) {
	meta := m.Owner
	cls := meta.metaOf
	if cls.File != nil && !cls.File.isPrelude() && cls.Line > 0 {
		c.lineDirective(cls.File, cls.Line) // its Go errors name the class, not a line counted on from the last directive
	}
	params, ret := c.sig(m, map[string]Type{"Self": TClass{C: cls}})
	body := fmt.Sprintf("return String(%q)", cls.displayName())
	if m.Name == "new" {
		body = "return " + cls.ctorName() + "(" + c.argNames(m) + ")"
		if cls.isPerRun {
			body = "return New" + cls.Name + "In(" + strings.TrimSuffix("self, "+c.argNames(m), ", ") + ")" // the object remembers this class object (decision 259)
		}
	}
	if m.Name == "new" && cls.isTemplate() { // ponytail: no class-object new on a generic class held untyped; build its erasure instance
		return
	}
	if m.Name == "new" {
		c.labels[meta.Name+"."+m.GoName] = "Class#new"
	}
	c.w("func (self *%s) %s(%s) %s { %s }\n\n", meta.Name, m.GoName, params, ret, body)
}

// classStmt stands in main's statement list for a class/module body's
// load-time statement (decision 166): it runs where the class is defined,
// in source order, with the class object as `self` and its lexical scope.
type classStmt struct {
	parser.Node
	file  *File
	scope []*Class
	unit  bool // a whole body's statements, sharing its locals (decision 170)
}

// genClassStmt emits a class-body statement in its class's lexical scope.
func (f *fctx) genClassStmt(cs *classStmt) {
	if f.c.gemFile(cs.file) { // decision 205: a gem load-time statement that does not compile is skipped, warned
		err := f.tryClean(func() { f.genClassStmt0(cs) })
		if err != nil {
			f.c.warn(cs.file, cs.Node, "a load-time statement does not compile, so it is skipped: %s (decision 205)", err.Error())
		}
		return
	}
	f.c.keepGoing(func() { f.genClassStmt0(cs) }) // RUGAROO_KEEP_GOING: main's load-time statements too
}

func (f *fctx) genClassStmt0(cs *classStmt) {
	sub := f.c.newFctx(cs.file, nil, nil)
	if n := len(cs.scope); n > 0 && cs.scope[n-1].meta != nil {
		cls := cs.scope[n-1]
		sub.selfType, sub.selfCode, sub.selfClassObj = TClass{C: cls.meta}, classVar(cls), true
		sub.owner = cls.meta // `@x = v` in a class body is the class object's ivar
	}
	sub.lex = cs.scope
	sub.indent = f.indent
	sub.retVar = ""
	sub.bodyUnit = cs.unit
	// its own Go block: each statement's temporaries (and a unit's locals) restart in a fresh fctx
	sub.indent++
	f.emit("{")
	defer f.emit("}")
	sub.genBody(cs.Node, nil, tail{}, nil)
	f.buf.WriteString(sub.buf.String())
}

// constInit stands in main's statement list for a constant assignment, so
// constants are evaluated in source order like MRI (prelude ones first).
type constInit struct {
	parser.Node
	k *Const
}

func (ci *constInit) GetLocation() parser.Location {
	loc := ci.k.Value.GetLocation()
	if ci.k.initAt != nil { // a constant lifted out of another's value runs where that one does, just before it
		loc.StartOffset = *ci.k.initAt
	}
	return loc
}

func (ci *constInit) CompactChildNodes() []parser.Node { return nil }

func (ci *constInit) ChildNodes() []parser.Node { return nil }

// loadFile stands in main's statement list for a top-level
// `require_relative`: the required file's top level runs there (decision 130).
type loadFile struct {
	parser.Node
	call *parser.CallNode
	file *File
}

func (lf *loadFile) GetLocation() parser.Location { return lf.call.Location }

func (lf *loadFile) CompactChildNodes() []parser.Node { return nil }

func (lf *loadFile) ChildNodes() []parser.Node { return nil }

// fileBody is what one user file's top level runs, in source order.
type fileBody struct {
	f     *File
	stmts []parser.Node
}

// mainBodies splits what main runs into the prelude's constant assignments
// and each user file's top level, in load order (decision 84): the file's
// statements, and what its class bodies run (constant assignments, hook
// calls), by position. A file runs whole before the next, as `require` does.
func (c *Compiler) mainBodies() ([]parser.Node, []fileBody) {
	var prelude []parser.Node
	own := map[*File][]parser.Node{}
	for _, k := range c.constList {
		if k.inMethod || k.inBody || k.inMain { // assigned where its method, its body's unit or its top-level statement runs
			continue
		}
		if k.File.isPrelude() {
			prelude = append(prelude, &constInit{k: k})
		} else {
			own[k.File] = append(own[k.File], &constInit{k: k})
		}
	}
	for _, h := range c.hooks {
		if n := c.hookCall(h); n != nil {
			own[h.file] = append(own[h.file], n)
		}
	}
	for _, cs := range c.classStmts {
		own[cs.file] = append(own[cs.file], cs)
	}
	stmts := map[*File][]parser.Node{}
	for _, n := range c.mainStmts {
		stmts[c.stmtFile[n]] = append(stmts[c.stmtFile[n]], n)
	}
	bodies := make([]fileBody, 0, len(c.userFiles))
	for _, f := range c.userFiles {
		mine, rest := own[f], stmts[f]
		slices.SortStableFunc(mine, func(a, b parser.Node) int {
			return cmp.Compare(a.GetLocation().StartOffset, b.GetLocation().StartOffset)
		})
		mine = spliceIntoUnits(mine)
		body := append([]parser.Node(nil), c.beginStmts[f]...)
		for len(rest) > 0 || len(mine) > 0 {
			if len(mine) > 0 && (len(rest) == 0 || mine[0].GetLocation().StartOffset < rest[0].GetLocation().StartOffset) {
				body, mine = append(body, mine[0]), mine[1:]
				continue
			}
			body, rest = append(body, rest[0]), rest[1:]
		}
		bodies = append(bodies, fileBody{f: f, stmts: body})
	}
	return prelude, bodies
}

// spliceIntoUnits moves a constant's init that falls inside a class body's unit (decision 170: a body with locals runs
// as one statement, placed at its first) into that unit, in source order: racc's generated parsers assign
// `Racc_token_to_s_table = [...]` (no local read, so not the unit's) after the unit's first statements and read it in
// the unit's `Ractor.make_shareable(Racc_token_to_s_table)` right after. The rest of mine is answered.
func spliceIntoUnits(mine []parser.Node) []parser.Node {
	for _, r := range slices.Clone(mine) {
		cs, ok := r.(*classStmt)
		if !ok || !cs.unit {
			continue
		}
		st := cs.Node.(*parser.StatementsNode) //nolint:forcetypeassert // a unit's node is its statements
		if len(st.Body) == 0 {
			continue
		}
		first, last := st.Body[0].GetLocation().StartOffset, endOf(st.Body[len(st.Body)-1])
		kept := mine[:0:0]
		for _, m := range mine {
			at := m.GetLocation().StartOffset
			if _, isConst := m.(*constInit); !isConst || at <= first || at >= last {
				kept = append(kept, m)
				continue
			}
			i, _ := slices.BinarySearchFunc(st.Body, at, func(n parser.Node, off int) int { return cmp.Compare(n.GetLocation().StartOffset, off) })
			st.Body = slices.Insert(st.Body, i, m)
		}
		mine = kept
	}
	return mine
}

// mainBody is everything main runs, flattened, required files where they load.
func (c *Compiler) mainBody() []parser.Node {
	prelude, bodies := c.mainBodies()
	of := map[*File][]parser.Node{}
	for _, b := range bodies {
		of[b.f] = b.stmts
	}
	var add func([]parser.Node)
	add = func(stmts []parser.Node) {
		for _, n := range stmts {
			if lf, ok := n.(*loadFile); ok {
				add(of[lf.file])
				continue
			}
			prelude = append(prelude, n)
		}
	}
	for _, b := range bodies {
		if !c.required[b.f] {
			add(b.stmts)
		}
	}
	return prelude
}

// hookCall is the call MRI makes at hook site h, or nil when the receiver
// does not define the hook (Ruby's own are no-ops).
func (c *Compiler) hookCall(h classHook) parser.Node {
	recv := h.cls.Super
	if h.mod != nil {
		recv = c.resolveClassRef(h.mod)
	}
	if recv == nil || recv.meta == nil || recv.meta.lookup(h.name) == nil {
		return nil
	}
	obj := func(k *Class) parser.Node {
		return &exprNode{e: expr{code: classVar(k), typ: TClass{C: k.meta}, classObj: true}}
	}
	return &parser.CallNode{Location: h.node.GetLocation(), Receiver: obj(recv), Name: h.name, Arguments: &parser.ArgumentsNode{Arguments: []parser.Node{obj(h.cls)}}}
}

// genStandIn emits what main's statement list holds in place of source: a constant's assignment, a required file,
// a class body's statement; false for an ordinary node.
func (f *fctx) genStandIn(n parser.Node, t tail) bool {
	switch n := n.(type) {
	case *constInit:
		f.genConstInit(n.k)
	case *loadFile:
		f.buf.WriteString("{\n" + f.c.loadCode[n.file] + "}\n")
	case *classStmt:
		f.genClassStmt(n)
	case *parser.ConstantWriteNode:
		if k := f.c.consts[qualify(f.lex, n.Name)]; k == nil || !f.bodyUnit && !k.inMain && !k.inBody { // nil: written inside a block, which collect never sees; inBody: also inside a body's begin (decision 334)
			return false
		}
		f.genBodyConst(n, t)
	default:
		return false
	}
	return true
}

// genBodyConst assigns a constant that reads its class body's locals, inside the body's unit (decision 170).
func (f *fctx) genBodyConst(n *parser.ConstantWriteNode, t tail) {
	k := f.c.consts[qualify(f.lex, n.Name)]
	typ := f.c.constType(k)
	f.lineOf(n)
	f.emit("%s = %s", k.GoName, f.coerce(n.Value, f.genExpr(n.Value, typ), typ))
	if g := cmp.Or(k.first, k); g.guarded {
		f.emit("%s = true", constSet(g))
	}
	if t.kind != tailNone { // last in a block: the assignment's value is the block's
		f.applyTail(n, expr{code: k.GoName, typ: typ}, t)
	}
}

// genConstInit assigns a constant's package variable.
func (f *fctx) genConstInit(k *Const) {
	f.c.keepGoing(func() { f.genConstInit0(k) }) // RUGAROO_KEEP_GOING: main's load-time statements too
}

func (f *fctx) genConstInit0(k *Const) {
	if f.c.constType(k); k.poison != "" {
		return // decision 205: left nil
	}
	if f.c.gemFile(k.File) {
		err := f.tryClean(func() { f.genConstInit1(k) })
		if err != nil {
			k.poison = err.Error()
			f.c.warn(k.File, k.Value, "constant %s does not compile, so it is nil: %s (decision 205)", k.RubyName, err.Error())
		}
		return
	}
	f.genConstInit1(k)
}

func (f *fctx) genConstInit1(k *Const) {
	typ := f.c.constType(k)
	sub := f.c.constFctx(k)
	sub.indent = f.indent + 1
	if lambdaInside(k.Value) { // a block's locals are hoisted at its top from what pass 1 saw (net-ftp's TIME_PARSER, decision 416)
		sub.unset = maybeUnset(k.Value, nil) // `usec = ... if fractions` is nil when it does not run
		sub.pass = 1
		_ = catchCompileError(func() { sub.probe(func() { sub.genExpr(k.Value, typ) }) })
		sub.pass = 2
	}
	e := sub.genExpr(k.Value, typ)
	code := sub.coerce(k.Value, e, typ)
	if e.lit && typeEq(e.typ, typ) { // coerce boxed or converted any other literal
		code = f.c.goType(typ) + "(" + code + ")"
	}
	body := sub.buf
	sub.buf = &strings.Builder{}
	sub.hoistLocals(methodScope) // `X = if (path = a).exist? || (path = b).exist?`: assigned inside conditions
	sub.buf.WriteString(body.String())
	fmt.Fprintf(f.buf, "//line %s:%d\n", k.File.Name, k.Line)
	name := k.GoName
	if p := k.first; p != nil { // MRI's warning, then the new value (decision 363)
		name = p.GoName
		short := k.RubyName[strings.LastIndex(k.RubyName, ":")+1:]
		msg := fmt.Sprintf("%s:%d: warning: already initialized constant %s\n%s:%d: warning: previous definition of %s was here\n", k.File.Name, k.Line, k.RubyName, p.File.Name, p.Line, short)
		f.emit("rbStderrIO().Write(String(%q))", msg)
	}
	if sub.buf.Len() == 0 {
		f.emit("%s = %s", name, code)
	} else {
		f.emit("%s = rbBeginV(func() %s {", name, f.c.goType(typ))
		f.buf.WriteString(sub.buf.String())
		f.emit("\treturn %s", code)
		f.emit("})")
	}
	if t := cmp.Or(k.first, k); t.guarded {
		f.emit("%s = true", constSet(t))
	}
}

// lambdaInside reports a lambda, proc or block in n, whose body hoists its own locals.
func lambdaInside(n parser.Node) bool {
	return anyNode(n, func(x parser.Node) bool {
		switch x.(type) {
		case *parser.LambdaNode, *parser.BlockNode:
			return true
		}
		return false
	})
}

// guardConsts marks main.rb's constants that may be read before their
// assignment runs, where MRI raises NameError but a Go package variable
// reads as its zero value. A constant needs no guard when only other
// constant assignments precede it that call no method main.rb defines and
// read no constant not yet assigned, which is how programs usually start.
func (c *Compiler) guardConsts() {
	defs := map[string]bool{}
	for _, uf := range c.userFiles {
		anyNode(uf.Root, func(n parser.Node) bool {
			if d, ok := n.(*parser.DefNode); ok {
				defs[d.Name] = true
			}
			return false
		})
	}
	done := map[*Const]bool{}
	ran := false
	for _, n := range c.mainBody() {
		ci, ok := n.(*constInit)
		if !ok {
			ran = true
			continue
		}
		k := ci.k
		if !k.File.isPrelude() && (ran || c.initRuns(k, defs, done)) {
			k.guarded, ran = true, true
		}
		done[k] = true
	}
}

// initRuns reports whether k's initializer may run main.rb's code or read
// a constant not yet assigned. Methods match by name, erring toward a guard.
func (c *Compiler) initRuns(k *Const, defs map[string]bool, done map[*Const]bool) bool {
	return anyNode(k.Value, func(n parser.Node) bool {
		switch n := n.(type) {
		case *parser.CallNode:
			return defs[n.Name] || n.Name == "new" && defs["initialize"]
		case *parser.EmbeddedStatementsNode:
			return defs["to_s"]
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			_, r := c.lookupConst(k.File, n, k.Scope)
			return r != nil && !done[r]
		}
		return false
	})
}
