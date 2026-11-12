package compiler

import (
	"cmp"
	"context"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// No run-time eval: what boot's string evals defined is collected as source where it ran, and the call is a no-op (decision 204).

// collectBootEvals collects each recorded string eval whose call site is compiled, into the module it ran on.
func (c *Compiler) collectBootEvals(ctx context.Context, evals []boot.EvalInfo) {
	for _, e := range evals {
		c.keepGoing(func() { c.collectBootEval(ctx, e) })
	}
}

// callerFile is the compiled file and line of a boot "path:line", or nil.
func (c *Compiler) callerFile(loc string) (*File, int) {
	l, ok := boot.ParseLoc(loc)
	if !ok || l.Line == 0 {
		return nil, 0
	}
	return c.parsed[c.real(l.File)], l.Line
}

func (c *Compiler) collectBootEval(ctx context.Context, e boot.EvalInfo) {
	f, ln := c.callerFile(e.Caller)
	if f == nil {
		return // evaluated by code outside the closed world (a replaced gem's own)
	}
	if e.Kind == "run" {
		c.collectRunEval(ctx, f, ln, e)
		return
	}
	if e.Kind == "class_eval_block" {
		c.collectBlockEval(ctx, f, ln, e)
		return
	}
	if e.Kind == "const_set" {
		// a load-time `Mod.const_set(name, plain_value)` (rubocop-ast's NodePattern::Sets): the constant its module would
		// have assigned, declared there and set once the file that made it has loaded (decision 444)
		target := c.classes[e.Self]
		if target == nil {
			return
		}
		sf, err := parseFile(ctx, c.parser, f.Name, []byte(e.Source), originSource)
		if err != nil {
			return
		}
		sf.path, sf.pinLine = f.path, ln
		before := len(c.constList)
		c.collectBody(ctx, sf, target, sf.Root.Statements, c.lexicalOf(target), true)
		for _, k := range c.constList[before:] {
			c.addMainStmt(f, &constInit{k: k})
		}
		return
	}
	site := boot.Loc{File: f.path, Line: ln}
	c.evalSites[site] = true
	e.Source = wrapBigLiterals(e.Source)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(e.Source), originSource)
	if err != nil {
		c.errorAt(srcPos{f, ln}, "eval: %v", err)
	}
	sf.path, sf.pinLine = f.path, ln
	target := c.classes[e.Self]
	switch {
	case e.Kind == "class_eval" && target != nil:
		c.noteEvalUnit(site, e.Source, sf)
		c.collectBody(ctx, sf, target, sf.Root.Statements, c.lexicalOf(target), true)
		c.markEvalClasses(f, ln, sf)
	case e.Kind == "instance_eval" && target != nil:
		c.noteEvalUnit(site, e.Source, sf)
		for _, st := range sf.Root.Statements.Body {
			switch n := st.(type) {
			case *parser.DefNode: // an instance_eval'd def is the module's singleton method
				target.singletonDefs = append(target.singletonDefs, singletonDef{node: n, scope: c.lexicalOf(target), file: sf})
			case *parser.ClassNode:
				c.collectClass(ctx, sf, n, nil)
			case *parser.ModuleNode:
				c.collectModule(ctx, sf, n, nil)
			default:
				c.errorf(sf, st, "instance_eval of %s: only defs and classes are compiled from boot (decision 204)", e.Self)
			}
		}
		c.markEvalClasses(f, ln, sf)
	case e.Kind == "eval" && e.Self == "":
		c.collect(ctx, sf)
	default:
		c.errorAt(srcPos{f, ln}, "a %s on %q: rugaroo compiles a string eval only on a named module, from boot (decision 204)", e.Kind, e.Self)
	}
}

// collectRunEval keeps a string the trace saw evaluated at run time, at line ln of f, for its call site (decision 374):
// Homebrew's CaskLoader instance_evals a tap's cask file. A site rugaroo does not compile this way ignores it.
func (c *Compiler) collectRunEval(ctx context.Context, f *File, ln int, e boot.EvalInfo) {
	site := boot.Loc{File: f.path, Line: ln}
	for _, u := range c.runEvals[site] {
		if u.src == e.Source {
			return
		}
	}
	name := f.Name
	if e.File != "" { // positions and backtraces name the eval's file and its own lines, as MRI's (Homebrew's deprecation notice points into Casks/x.rb)
		name = e.File
	}
	sf, err := parseFile(ctx, c.parser, name, []byte(e.Source), originSource)
	if err != nil {
		return // Prism rejects what MRI ran (an encoding comment it does not know): the site raises, as for any other string
	}
	sf.path = f.path // gem or user code as its call site is
	if e.File == "" {
		sf.pinLine = ln
	}
	if c.runEvals == nil {
		c.runEvals = map[boot.Loc][]evalUnit{}
	}
	c.runEvals[site] = append(c.runEvals[site], evalUnit{src: e.Source, file: sf})
}

// runEvalsAt is what the trace saw an `instance_eval(string, ...)` on self evaluate here.
func (f *fctx) runEvalsAt(n *parser.CallNode) []evalUnit {
	if n.Name != "instance_eval" || n.Block != nil || len(callArgs(n)) == 0 || f.c.runEvals == nil {
		return nil
	}
	if _, self := n.Receiver.(*parser.SelfNode); n.Receiver != nil && !self {
		return nil
	}
	loc := n.Location
	if n.MessageLoc != nil {
		loc = *n.MessageLoc
	}
	return f.c.runEvals[f.f.bootLoc(loc.StartOffset)]
}

// genRunEval is a run-time instance_eval site: the evaluated string picks the source the trace saw, which runs here as a
// block on this self, its last value the call's. Another string raises: the binary is the closed world of what the
// trace ran (decisions 296, 374). The file and line arguments only name the source for MRI's messages.
func (f *fctx) genRunEval(n *parser.CallNode, units []evalUnit) expr {
	str := f.cls("String")
	src := f.coerce(n, f.genExpr(callArgs(n)[0], str), str)
	codes := make([]string, len(units))
	types := make([]Type, len(units))
	for i, u := range units {
		env := map[string]Type{}
		sig := &BlockSig{Ret: TVar{Name: "R"}}
		var body parser.Node
		if u.file.Root.Statements != nil {
			body = u.file.Root.Statements
		}
		blk := &parser.BlockNode{Location: n.Location, Body: body}
		codes[i] = f.inFile(u.file, func() string { return f.genClosure(blk, blk, sig, env) })
		types[i] = subst(sig.Ret, env)
	}
	t := types[0]
	if len(units) > 1 {
		t = TAny{}
	}
	res, tmp := f.newTmp(), f.newTmp()
	if !isVoid(t) {
		f.emit("var %s %s", res, f.c.goType(t))
	}
	f.emit("switch %s := string(%s); %s {", tmp, src, tmp)
	for i, u := range units {
		f.emit("case %s:", strconv.Quote(u.src))
		if isVoid(types[i]) {
			f.emit("	(%s)()", codes[i])
			if !isVoid(t) {
				f.emit("	%s = nil", res)
			}
		} else {
			f.emit("	%s = %s", res, f.coerce(n, expr{code: "(" + codes[i] + ")()", typ: types[i]}, t))
		}
	}
	f.emit("default:")
	f.emit("	panic(NewNotImplementedError(Ref(String(%q + %s))))", "rugaroo: a string eval the trace did not run: ", tmp)
	f.emit("}")
	if isVoid(t) {
		return expr{code: "nil", typ: TNil{}, lit: true}
	}
	return expr{code: res, typ: t}
}

// inFile runs gen with sf as the file being compiled, restored however gen leaves (a compile error unwinds to a retry).
func (f *fctx) inFile(sf *File, gen func() string) string {
	saved := f.f
	f.f = sf
	defer func() { f.f = saved }()
	return gen()
}

// lexicalOf is the lexical scope code written inside cls's body sees: each enclosing namespace, then cls.
func (c *Compiler) lexicalOf(cls *Class) []*Class {
	var out []*Class
	parts := strings.Split(cls.RubyName, "::")
	for i := range parts {
		if k := c.classes[strings.Join(parts[:i+1], "::")]; k != nil {
			out = append(out, k)
		}
	}
	return out
}

// evalUnitsAt is what an instance_eval call site ran at boot, when it defined classes there.
func (f *fctx) evalUnitsAt(n *parser.CallNode) []evalUnit {
	if n.Name != "instance_eval" && n.Name != "class_eval" && n.Name != "module_eval" || !f.isEvalSite(n) {
		return nil
	}
	loc := n.Location
	if n.MessageLoc != nil {
		loc = *n.MessageLoc
	}
	units := f.c.evalUnits[f.f.bootLoc(loc.StartOffset)]
	if !slices.ContainsFunc(units, func(u evalUnit) bool { return len(f.c.unitClasses(u.file)) > 0 }) {
		return nil // only defs (concurrent-ruby's make_synchronized_on_cruby): collected already, the call a no-op
	}
	return units
}

// isEvalSite reports a string eval boot ran here: its source is compiled, so the call does nothing (decision 204).
func (f *fctx) isEvalSite(n *parser.CallNode) bool {
	switch n.Name {
	case "eval", "class_eval", "module_eval", "instance_eval":
	default:
		return false
	}
	if _, ok := firstArg(callArgs(n)).(*parser.StringNode); !ok {
		if _, ok := firstArg(callArgs(n)).(*parser.InterpolatedStringNode); !ok {
			return false
		}
	}
	loc := n.Location
	if n.MessageLoc != nil {
		loc = *n.MessageLoc
	}
	return f.c.evalSites[f.f.bootLoc(loc.StartOffset)]
}

// collectBlockEval collects `klass.class_eval do ... end`'s block, at line ln of f, as a body of the module boot ran it
// on; the call is a no-op (decision 204). A block evaluated on several modules is each one's body.
func (c *Compiler) collectBlockEval(ctx context.Context, f *File, ln int, e boot.EvalInfo) {
	target := c.classes[e.Self]
	var call *parser.CallNode
	anyNode(f.Root, func(n parser.Node) bool {
		if x, ok := n.(*parser.CallNode); ok && (x.Name == "class_eval" || x.Name == "module_eval") {
			if b, ok := x.Block.(*parser.BlockNode); ok && f.line(b.Location.StartOffset) == ln && x.Arguments == nil {
				call = x
				return true
			}
		}
		return false
	})
	if call == nil || target == nil {
		return
	}
	body, ok := call.Block.(*parser.BlockNode).Body.(*parser.StatementsNode)
	if !ok {
		return
	}
	if seedsFromLocals(body) {
		return // `subclass.class_eval { @x = x_local }` in a hook: run where it stands (genIvarsEval), not as the class's body
	}
	if c.blockEvalSites == nil {
		c.blockEvalSites = map[*parser.CallNode]bool{}
	}
	c.blockEvalSites[call] = true
	c.collectBody(ctx, f, target, body, c.lexicalOf(target), true)
}

// seedsFromLocals is a block of instance variable writes at least one of which reads an enclosing local (ruby-macho's
// inherited hook copying its class-level state to the subclass, rubocop-ast's `base.class_eval { @registry =
// us.registry.dup }`): the values exist only where the hook runs.
func seedsFromLocals(body *parser.StatementsNode) bool {
	reads := false
	for _, st := range body.Body {
		w, ok := st.(*parser.InstanceVariableWriteNode)
		if !ok {
			return false
		}
		if anyNode(w.Value, func(n parser.Node) bool { _, isLocal := n.(*parser.LocalVariableReadNode); return isLocal }) {
			reads = true
		}
	}
	return reads
}

// evalUnit is one source a string eval site ran at boot, and the file its classes were collected from.
type evalUnit struct {
	src  string
	file *File
}

// evalDefFlag names the flag a class a run-time string eval defines sets once that eval has run, or "".
func (c *Compiler) evalDefFlag(cls *Class) string {
	if cls == nil || !c.evalClasses[cls] {
		return ""
	}
	return "rbEvalDef_" + cls.Name
}

// markEvalClasses records the classes sf's source defines when its eval site, line ln of f, is in a method: they are
// defined when that method runs, not at load.
func (c *Compiler) markEvalClasses(f *File, ln int, sf *File) {
	inDef := false
	anyNode(f.Root, func(n parser.Node) bool {
		if d, ok := n.(*parser.DefNode); ok && f.line(d.Location.StartOffset) <= ln && ln <= f.line(d.Location.StartOffset+d.Location.Length) {
			inDef = true
		}
		return inDef
	})
	if !inDef {
		return
	}
	if c.evalClasses == nil {
		c.evalClasses = map[*Class]bool{}
	}
	for _, k := range c.unitClasses(sf) {
		c.evalClasses[k] = true
	}
}

// unitClasses are the classes sf's source defines.
func (c *Compiler) unitClasses(sf *File) []*Class {
	var out []*Class
	for _, k := range c.classList {
		if k.File == sf && k.metaOf == nil {
			out = append(out, k)
		}
	}
	return out
}

func (c *Compiler) noteEvalUnit(key boot.Loc, src string, sf *File) {
	if c.evalUnits == nil {
		c.evalUnits = map[boot.Loc][]evalUnit{}
	}
	for _, u := range c.evalUnits[key] {
		if u.src == src {
			return
		}
	}
	c.evalUnits[key] = append(c.evalUnits[key], evalUnit{src: src, file: sf})
}

// unitNodes are what loading sf runs: its constants' assignments, class hooks and class-body statements, in order.
func (c *Compiler) unitNodes(sf *File) []parser.Node {
	var out []parser.Node
	for _, k := range c.constList {
		if k.File == sf && !k.inMethod && !k.inBody && !k.inMain {
			out = append(out, &constInit{k: k})
		}
	}
	for _, h := range c.hooks {
		if h.file == sf {
			if n := c.hookCall(h); n != nil {
				out = append(out, n)
			}
		}
	}
	for _, cs := range c.classStmts {
		if cs.file == sf {
			out = append(out, cs)
		}
	}
	slices.SortStableFunc(out, func(a, b parser.Node) int {
		return cmp.Compare(a.GetLocation().StartOffset, b.GetLocation().StartOffset)
	})
	return out
}

// genEvalSwitch is an instance_eval site that defined classes at boot (bindata's endian subclasses, made inside a
// method each record's class body calls): the evaluated string picks the source boot saw, and that source's load-time
// statements (its classes' inherited hooks, their bodies) run here, each time, as MRI's eval runs them. Another
// string raises (decision 324).
func (f *fctx) genEvalSwitch(n *parser.CallNode, units []evalUnit) expr {
	src := f.genExpr(firstArg(callArgs(n)), f.cls("String"))
	tmp := f.newTmp()
	f.emit("switch %s := string(%s); %s {", tmp, f.coerce(n, src, f.cls("String")), tmp)
	for _, u := range units {
		f.emit("case %s:", strconv.Quote(u.src))
		f.indent++
		for _, k := range f.c.unitClasses(u.file) { // defined from here: const_get finds it (decision 324)
			if flag := f.c.evalDefFlag(k); flag != "" {
				f.emit("%s = true", flag)
			}
		}
		for _, st := range f.c.unitNodes(u.file) {
			f.genStmt(st, tail{})
		}
		f.indent--
	}
	f.emit("default:")
	f.emit("\tpanic(NewNotImplementedError(Ref(String(%q + %s))))", "rugaroo: a string eval boot did not run: ", tmp)
	f.emit("}")
	return expr{code: "nil", typ: TNil{}, lit: true}
}

// wrapBigLiterals rewrites each decimal Integer literal past int64 in an eval's source as its value mod 2**64, as a
// gem's run-time Integer#<< wraps (decision 324): the string the program builds (bindata's `"val &= #{(1 << nbits) -
// 1}"` for 64 bits) then equals the one boot recorded, and the code means what Go's 64-bit arithmetic does with it.
func wrapBigLiterals(src string) string {
	var b strings.Builder
	word := func(c byte) bool {
		return c == '_' || c == '.' || c == '$' || c == '@' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for i := 0; i < len(src); {
		if src[i] < '0' || src[i] > '9' || i > 0 && word(src[i-1]) {
			b.WriteByte(src[i])
			i++
			continue
		}
		j := i
		for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '_') {
			j++
		}
		lit := src[i:j]
		n, ok := new(big.Int).SetString(strings.ReplaceAll(lit, "_", ""), 10)
		if j < len(src) && (word(src[j]) && src[j] != '.' || src[j] == '.' && j+1 < len(src) && src[j+1] >= '0' && src[j+1] <= '9') || !ok || n.IsInt64() {
			b.WriteString(lit)
		} else {
			b.WriteString(strconv.FormatInt(int64(new(big.Int).And(n, new(big.Int).SetUint64(math.MaxUint64)).Uint64()), 10)) //nolint:gosec // the wrap is the point
		}
		i = j
	}
	return b.String()
}
