package compiler

import (
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// RUGAROO_TYPEGRAPH spike: does Crystal-style propagation with a cheap evaluator agree with codegen-driven inference?

// tgNode is one slot of the graph: a parameter, a return, a local, an ivar or a yield.
type tgNode struct {
	t       Type
	readers map[*Method]bool
	evals   int
}

type typeGraph struct {
	c       *Compiler
	nodes   map[string]*tgNode
	queue   []*Method
	queued  map[*Method]bool
	evals   map[*Method]int
	cur     *Method
	gaps    map[string]int
	methods []*Method
	main    []*Method // the top-level bodies, evaluated as methods with no owner
}

const tgMaxEvals = 40 // a method re-evaluated this often widens its slots to untyped: a finite lattice in practice

// typeGraphReport runs the spike after the compiler's own inference and prints how the two agree.
func (c *Compiler) typeGraphReport() {
	if !c.env.TypeGraph {
		return
	}
	defer func() { // a report-only spike never fails a compile
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "rugaroo: typegraph: gave up: %v\n%s", r, debug.Stack())
		}
	}()
	g := &typeGraph{c: c, nodes: map[string]*tgNode{}, queued: map[*Method]bool{}, evals: map[*Method]int{}, gaps: map[string]int{}}
	for _, m := range c.allMethods() {
		if m.Kind == kindDef && m.Node != nil && m.Node.Body != nil && m.File != nil && !m.File.isPrelude() {
			g.methods = append(g.methods, m)
			g.enqueue(m)
		}
	}
	_, bodies := c.mainBodies()
	for i, b := range bodies { // the program's top level calls into its methods: without it their parameters see no arguments
		if len(b.stmts) == 0 || b.f == nil {
			continue
		}
		loc := b.stmts[0].GetLocation()
		mm := &Method{Name: fmt.Sprintf("<main %d>", i), Kind: kindDef, Node: &parser.DefNode{Location: loc, Body: &parser.StatementsNode{Location: loc, Body: b.stmts}}}
		mm.File = b.f
		g.methods = append(g.methods, mm)
		g.main = append(g.main, mm)
		g.enqueue(mm)
	}
	start := time.Now()
	steps := 0
	for len(g.queue) > 0 {
		m := g.queue[0]
		g.queue = g.queue[1:]
		g.queued[m] = false
		g.evals[m]++
		g.evalMethod(m)
		steps++
	}
	took := time.Since(start)
	g.compare(took, steps)
}

// allMethods includes the top level's defs, which no class holds.
func (c *Compiler) allMethods() []*Method {
	out := slices.Clone(c.topDefList)
	for _, cls := range c.classList {
		out = append(out, cls.MethodList...)
	}
	return out
}

func (g *typeGraph) enqueue(m *Method) {
	if !g.queued[m] {
		g.queued[m] = true
		g.queue = append(g.queue, m)
	}
}

func (g *typeGraph) node(k string) *tgNode {
	n := g.nodes[k]
	if n == nil {
		n = &tgNode{readers: map[*Method]bool{}}
		g.nodes[k] = n
	}
	return n
}

// read registers the reader so a later change to the slot re-evaluates it, as Crystal's observers do.
func (g *typeGraph) read(k string) Type {
	n := g.node(k)
	if g.cur != nil {
		n.readers[g.cur] = true
	}
	return n.t
}

// flow joins t into slot k; a change re-queues every method that read k.
func (g *typeGraph) flow(k string, t Type) {
	if t == nil {
		return
	}
	n := g.node(k)
	var nt Type
	switch {
	case n.t == nil:
		nt = t
	case n.evals > tgMaxEvals:
		nt = TAny{}
	default:
		j, ok := join(n.t, t)
		if !ok {
			j = TAny{}
		}
		nt = j
	}
	if n.t != nil && typeEq(nt, n.t) {
		return
	}
	n.t = nt
	n.evals++
	for m := range n.readers {
		g.enqueue(m)
	}
}

func paramSlot(k pendingKey) string { return "p:" + string(k) }
func retSlot(m *Method) string      { return "r:" + walkKey(m) }
func localSlot(m *Method, name string) string {
	return "l:" + walkKey(m) + ":" + name
}
func ivarSlot(cls *Class, name string) string { return "i:" + cls.Name + name }
func yieldSlot(m *Method, i int) string       { return "y:" + walkKey(m) + ":" + strconv.Itoa(i) }

// evalMethod is one pass over m's body: its locals, ivar writes, calls' arguments and returns flow into their slots.
func (g *typeGraph) evalMethod(m *Method) {
	if g.evals[m] > tgMaxEvals {
		g.flow(retSlot(m), TAny{})
		return
	}
	g.cur = m
	defer func() { g.cur = nil }()
	for _, p := range m.Params {
		if p.Pending == "" && p.Type != nil {
			g.flow(localSlot(m, p.Name), p.Type)
		}
		if p.Pending != "" && p.Default != nil { // a default types its parameter when no call passes it
			g.flow(paramSlot(p.Pending), g.eval(m, p.Default))
		}
	}
	v := g.eval(m, m.Node.Body)
	g.flow(retSlot(m), v)
}

// paramType is a parameter's type as the graph sees it: its slot when typed from use, else its signature's.
func (g *typeGraph) paramType(m *Method, name string) Type {
	for _, p := range m.Params {
		if p.Name == name || strings.TrimLeft(p.Name, "*&") == name {
			if p.Pending != "" {
				return g.read(paramSlot(p.Pending))
			}
			return p.Type
		}
	}
	return nil
}

func (g *typeGraph) self(m *Method) Type {
	if m.Owner == nil || m.Owner.IsModule {
		return TAny{}
	}
	return TClass{C: m.Owner}
}

func (g *typeGraph) gap(n parser.Node) Type {
	g.gaps[fmt.Sprintf("%T", n)]++
	return TAny{}
}

func (g *typeGraph) cls(name string) Type { return TClass{C: g.c.classes[name]} }

func (g *typeGraph) joinAll(ts []Type) Type {
	var out Type
	for _, t := range ts {
		if t == nil {
			continue
		}
		if out == nil {
			out = t
			continue
		}
		j, ok := join(out, t)
		if !ok {
			return TAny{}
		}
		out = j
	}
	return out
}

func (g *typeGraph) eval(m *Method, n parser.Node) Type { //nolint:cyclop,gocyclo,funlen,gocognit,maintidx // one case per node kind, as the codegen's
	if n == nil || reflect.ValueOf(n).IsNil() { // prism leaves an absent else or body as a nil pointer in the interface
		return TNil{}
	}
	switch n := n.(type) {
	case *parser.StatementsNode:
		var last Type = TNil{}
		for _, s := range n.Body {
			last = g.eval(m, s)
		}
		return last
	case *parser.BeginNode:
		v := g.eval(m, n.Statements)
		for r := n.RescueClause; r != nil; r = r.Subsequent {
			if lv, ok := r.Reference.(*parser.LocalVariableTargetNode); ok { // `rescue E => e`: the first class named, else StandardError
				et := g.cls("StandardError")
				if len(r.Exceptions) > 0 {
					if t, ok := g.eval(m, r.Exceptions[0]).(TClass); ok && t.C.metaOf != nil {
						et = TClass{C: t.C.metaOf}
					}
				}
				g.flow(localSlot(m, lv.Name), et)
			}
			v = g.joinAll([]Type{v, g.eval(m, r.Statements)})
		}
		if n.EnsureClause != nil {
			g.eval(m, n.EnsureClause.Statements)
		}
		return v
	case *parser.ParenthesesNode:
		return g.eval(m, n.Body)
	case *parser.IntegerNode:
		return g.cls("Integer")
	case *parser.FloatNode:
		return g.cls("Float")
	case *parser.StringNode, *parser.InterpolatedStringNode, *parser.XStringNode:
		if in, ok := n.(*parser.InterpolatedStringNode); ok {
			for _, p := range in.Parts {
				if e, ok := p.(*parser.EmbeddedStatementsNode); ok {
					g.eval(m, e.Statements)
				}
			}
		}
		return g.cls("String")
	case *parser.SymbolNode, *parser.InterpolatedSymbolNode:
		return g.cls("Symbol")
	case *parser.NilNode:
		return TNil{}
	case *parser.TrueNode, *parser.FalseNode:
		return g.cls("Boolean")
	case *parser.SelfNode:
		return g.self(m)
	case *parser.ArrayNode:
		var ts []Type
		for _, e := range n.Elements {
			ts = append(ts, g.eval(m, e))
		}
		return TClass{C: g.c.classes["Array"], Args: []Type{tgKnown(g.joinAll(ts))}}
	case *parser.HashNode:
		var ks, vs []Type
		for _, e := range n.Elements {
			if a, ok := e.(*parser.AssocNode); ok {
				ks, vs = append(ks, g.eval(m, a.Key)), append(vs, g.eval(m, a.Value))
			}
		}
		return TClass{C: g.c.classes["Hash"], Args: []Type{tgKnown(g.joinAll(ks)), tgKnown(g.joinAll(vs))}}
	case *parser.LocalVariableWriteNode:
		v := g.eval(m, n.Value)
		g.flow(localSlot(m, n.Name), v)
		return v
	case *parser.LocalVariableOperatorWriteNode, *parser.LocalVariableOrWriteNode, *parser.LocalVariableAndWriteNode:
		var name string
		var val parser.Node
		switch x := n.(type) {
		case *parser.LocalVariableOperatorWriteNode:
			name, val = x.Name, x.Value
		case *parser.LocalVariableOrWriteNode:
			name, val = x.Name, x.Value
		case *parser.LocalVariableAndWriteNode:
			name, val = x.Name, x.Value
		}
		v := g.eval(m, val)
		g.flow(localSlot(m, name), v)
		return g.read(localSlot(m, name))
	case *parser.LocalVariableReadNode:
		if t := g.paramType(m, n.Name); t != nil {
			return t
		}
		if t := g.read(localSlot(m, n.Name)); t != nil {
			return t
		}
		return nil
	case *parser.InstanceVariableWriteNode:
		v := g.eval(m, n.Value)
		if m.Owner != nil {
			g.flow(ivarSlot(m.Owner, n.Name), v)
		}
		return v
	case *parser.InstanceVariableOrWriteNode:
		v := g.eval(m, n.Value)
		if m.Owner != nil {
			g.flow(ivarSlot(m.Owner, n.Name), v)
			return g.read(ivarSlot(m.Owner, n.Name))
		}
		return v
	case *parser.InstanceVariableReadNode:
		if m.Owner == nil {
			return TAny{}
		}
		if t := g.read(ivarSlot(m.Owner, n.Name)); t != nil {
			return t
		}
		if iv := g.c.findIvar(m.Owner, n.Name); iv != nil && iv.Type != nil && !isAny(iv.Type) && g.annotatedIvar(iv) { // an annotation, which no write types
			return iv.Type
		}
		return nil
	case *parser.IfNode:
		g.eval(m, n.Predicate)
		t := g.eval(m, n.Statements)
		var e Type = TNil{}
		if n.Subsequent != nil {
			e = g.eval(m, n.Subsequent)
		}
		return g.joinAll([]Type{t, e})
	case *parser.ElseNode:
		return g.eval(m, n.Statements)
	case *parser.UnlessNode:
		g.eval(m, n.Predicate)
		t := g.eval(m, n.Statements)
		var e Type = TNil{}
		if n.ElseClause != nil {
			e = g.eval(m, n.ElseClause)
		}
		return g.joinAll([]Type{t, e})
	case *parser.AndNode:
		return g.joinAll([]Type{g.eval(m, n.Left), g.eval(m, n.Right)})
	case *parser.OrNode:
		l := g.eval(m, n.Left)
		if o, ok := l.(TOpt); ok {
			l = o.Elem
		}
		return g.joinAll([]Type{l, g.eval(m, n.Right)})
	case *parser.WhileNode:
		g.eval(m, n.Predicate)
		g.eval(m, n.Statements)
		return TNil{}
	case *parser.UntilNode:
		g.eval(m, n.Predicate)
		g.eval(m, n.Statements)
		return TNil{}
	case *parser.CaseNode:
		g.eval(m, n.Predicate)
		var ts []Type
		for _, w := range n.Conditions {
			if wn, ok := w.(*parser.WhenNode); ok {
				ts = append(ts, g.eval(m, wn.Statements))
			}
		}
		if n.ElseClause != nil {
			ts = append(ts, g.eval(m, n.ElseClause))
		} else {
			ts = append(ts, TNil{})
		}
		return g.joinAll(ts)
	case *parser.ReturnNode:
		if n.Arguments != nil && len(n.Arguments.Arguments) == 1 {
			g.flow(retSlot(m), g.eval(m, n.Arguments.Arguments[0]))
		} else {
			g.flow(retSlot(m), TNil{})
		}
		return nil
	case *parser.YieldNode:
		if n.Arguments != nil {
			for i, a := range n.Arguments.Arguments {
				g.flow(yieldSlot(m, i), g.eval(m, a))
			}
		}
		return TAny{}
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		k := g.c.classes[constName(n)]
		if k == nil {
			k = g.lookupClass(m, n)
		}
		if k != nil && k.meta != nil {
			return TClass{C: k.meta}
		}
		if t := g.read("k:" + constName(n)); t != nil {
			return t
		}
		if _, k := g.lookupConstVal(m, n); k != nil {
			return g.read("k:" + k.RubyName)
		}
		return g.gap(n)
	case *parser.CallNode:
		return g.call(m, n)
	case *classStmt:
		return g.eval(m, n.Node)
	case *constInit: // a constant's value, read back by name
		v := g.eval(m, n.k.Value)
		g.flow("k:"+n.k.RubyName, v)
		return v
	case *loadFile: // the required file's own top level is a body of its own
		return TNil{}
	case *parser.ClassNode:
		return g.eval(m, n.Body)
	case *parser.ModuleNode:
		return g.eval(m, n.Body)
	case *parser.DefNode, *parser.AliasMethodNode, *parser.UndefNode:
		return g.cls("Symbol")
	case *parser.MultiWriteNode:
		v := g.eval(m, n.Value)
		for _, t := range n.Lefts {
			if lw, ok := t.(*parser.LocalVariableTargetNode); ok {
				g.flow(localSlot(m, lw.Name), elemOf(v))
			}
		}
		return v
	case *parser.InstanceVariableOperatorWriteNode:
		v := g.eval(m, n.Value)
		if m.Owner != nil {
			g.flow(ivarSlot(m.Owner, n.Name), v)
			return g.read(ivarSlot(m.Owner, n.Name))
		}
		return v
	case *parser.RangeNode:
		return TClass{C: g.c.classes["Range"], Args: []Type{tgKnown(g.joinAll([]Type{g.eval(m, n.Left), g.eval(m, n.Right)}))}}
	case *parser.KeywordHashNode:
		for _, e := range n.Elements {
			if a, ok := e.(*parser.AssocNode); ok {
				g.eval(m, a.Value)
			}
		}
		return TClass{C: g.c.classes["Hash"], Args: []Type{g.cls("Symbol"), TAny{}}}
	case *parser.BreakNode, *parser.NextNode:
		return nil
	case *parser.RegularExpressionNode, *parser.InterpolatedRegularExpressionNode:
		return g.cls("Regexp")
	}
	return g.gap(n)
}

// call reads prelude signatures but never prelude bodies: the core library is typed by its RBS, as Crystal's is by its source.
func (g *typeGraph) call(m *Method, n *parser.CallNode) Type {
	var recv Type
	if n.Receiver != nil {
		recv = g.eval(m, n.Receiver)
	}
	ca := callArgs(n)
	args := make([]Type, 0, len(ca))
	for _, a := range ca {
		args = append(args, g.eval(m, a))
	}
	if n.Receiver == nil {
		if d := g.c.topDefs[n.Name]; d != nil && (m.Owner == nil || m.Owner.lookup(n.Name) == nil) { // a top-level def is a private method of every object
			return g.invoke(m, d, args, n)
		}
		recv = g.self(m)
	}
	if recv == nil {
		return nil
	}
	if o, ok := recv.(TOpt); ok {
		recv = o.Elem
	}
	switch r := recv.(type) {
	case TUnion:
		ts := make([]Type, 0, len(r.Members))
		for _, mem := range r.Members {
			ts = append(ts, g.callOn(m, n, mem, args))
		}
		return g.joinAll(ts)
	case TClass:
		return g.callOn(m, n, r, args)
	case TAny, TFunc, TNil, TOpt, TTuple, TVar, TVoid:
	}
	if t, ok := intrinsic(g, n.Name, recv); ok {
		return t
	}
	g.gaps["call on "+fmt.Sprintf("%T", recv)]++
	return TAny{}
}

func (g *typeGraph) callOn(m *Method, n *parser.CallNode, recv Type, args []Type) Type {
	if t, ok := intrinsic(g, n.Name, recv); ok {
		return t
	}
	rc, ok := recv.(TClass)
	if !ok {
		return TAny{}
	}
	if rc.C.metaOf != nil && n.Name == "new" { // Foo.new: initialize's parameters, an instance
		inst := rc.C.metaOf
		if e := inst.lookup("initialize"); e != nil && e.M.File != nil && !e.M.File.isPrelude() {
			g.flowArgs(e.M, positional(n, args))
			g.flowKeywords(m, e.M, n)
		}
		return TClass{C: inst}
	}
	e := rc.C.lookup(n.Name)
	if e == nil {
		g.gaps["no method "+n.Name]++
		return TAny{}
	}
	if e.M.File != nil && !e.M.File.isPrelude() && e.M.Kind == kindDef && e.M.Node != nil {
		return g.invoke(m, e.M, args, n)
	}
	if t, ok := g.attrCall(e, args); ok {
		return t
	}
	env := map[string]Type{"Self": recv}
	for k, v := range e.Env {
		env[k] = subst(v, map[string]Type{})
	}
	if len(rc.C.TypeParams) == len(rc.Args) {
		for i, tp := range rc.C.TypeParams {
			env[tp] = rc.Args[i]
		}
	}
	for i, p := range e.M.Params {
		if i < len(args) && args[i] != nil && p.Type != nil && !p.Rest && complete(args[i]) {
			unify(p.Type, args[i], env)
		}
	}
	if b, ok := n.Block.(*parser.BlockNode); ok && e.M.Block != nil {
		bt := g.block(m, b, e.M.Block, env)
		if e.M.Block.Ret != nil && bt != nil {
			unify(e.M.Block.Ret, bt, env)
		}
	}
	ret := e.M.Ret
	if ret == nil {
		g.gaps["prelude method without a return"]++
		return TAny{}
	}
	out := subst(ret, env)
	if mentionsVar(out) {
		return TAny{}
	}
	return out
}

// block evaluates a block literal with its parameters typed from the method's block signature, and answers its value.
func (g *typeGraph) block(m *Method, b *parser.BlockNode, sig *BlockSig, env map[string]Type) Type {
	if bp, ok := b.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
		for i, r := range bp.Parameters.Requireds {
			if rp, ok := r.(*parser.RequiredParameterNode); ok && i < len(sig.Params) {
				t := subst(sig.Params[i], env)
				if mentionsVar(t) {
					t = TAny{}
				}
				g.flow(localSlot(m, rp.Name), t)
			}
		}
	}
	return g.eval(m, b.Body)
}

// invoke is a call of a program method: each argument flows into its parameter, the result is its return slot.
func (g *typeGraph) invoke(m, callee *Method, args []Type, n *parser.CallNode) Type {
	g.flowArgs(callee, positional(n, args))
	g.flowKeywords(m, callee, n)
	if b, ok := n.Block.(*parser.BlockNode); ok {
		if bp, ok := b.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
			for i, r := range bp.Parameters.Requireds {
				if rp, ok := r.(*parser.RequiredParameterNode); ok {
					if t := g.read(yieldSlot(callee, i)); t != nil {
						g.flow(localSlot(m, rp.Name), t)
					}
				}
			}
		}
		g.eval(m, b.Body)
	}
	if !callee.inferRet && callee.Ret != nil {
		return callee.Ret
	}
	return g.read(retSlot(callee))
}

func (g *typeGraph) flowArgs(callee *Method, args []Type) {
	pos := 0
	for _, p := range callee.Params {
		if p.Keyword || p.KwRest || p.Rest {
			continue
		}
		if pos < len(args) && p.Pending != "" {
			g.flow(paramSlot(p.Pending), args[pos])
		}
		pos++
	}
}

// flowKeywords passes a call's `name: value` arguments to the keyword parameters of the same name.
func (g *typeGraph) flowKeywords(m, callee *Method, n *parser.CallNode) {
	for _, a := range callArgs(n) {
		kh, ok := a.(*parser.KeywordHashNode)
		if !ok {
			continue
		}
		for _, e := range kh.Elements {
			as, ok := e.(*parser.AssocNode)
			if !ok {
				continue
			}
			sym, ok := as.Key.(*parser.SymbolNode)
			if !ok {
				continue
			}
			for _, p := range callee.Params {
				if p.Keyword && p.Pending != "" && p.Name == sym.Unescaped.Value {
					g.flow(paramSlot(p.Pending), g.eval(m, as.Value))
				}
			}
		}
	}
}

// compare reports the graph's types beside the compiler's: parameters typed from use and inferred returns.
func (g *typeGraph) compare(took time.Duration, steps int) {
	var pSame, pDiff, pGraphNone, pCompAny, pTotal int
	var diffs []string
	for _, m := range g.methods {
		for _, p := range m.Params {
			if p.Pending == "" {
				continue
			}
			pTotal++
			gt := g.nodes[paramSlot(p.Pending)]
			ct := p.Type
			switch {
			case gt == nil || gt.t == nil:
				pGraphNone++
			case isAny(ct) && !isAny(gt.t):
				pCompAny++
			case sameType(m, ct, gt.t):
				pSame++
			default:
				pDiff++
				if len(diffs) < 12 {
					diffs = append(diffs, fmt.Sprintf("param %s: compiler %s, graph %s", p.Pending, typeString(ct), typeString(gt.t)))
				}
			}
		}
	}
	var rSame, rDiff, rGraphNone, rCompAny, rTotal int
	for _, m := range g.methods {
		if !m.inferRet || m.Ret == nil {
			continue
		}
		rTotal++
		gt := g.nodes[retSlot(m)]
		switch {
		case gt == nil || gt.t == nil:
			rGraphNone++
		case isAny(m.Ret) && !isAny(gt.t):
			rCompAny++
		case sameType(m, m.Ret, gt.t):
			rSame++
		default:
			rDiff++
			if len(diffs) < 24 {
				diffs = append(diffs, fmt.Sprintf("return %s: compiler %s, graph %s", walkKey(m), typeString(m.Ret), typeString(gt.t)))
			}
		}
	}
	type kv struct {
		k string
		n int
	}
	gaps := make([]kv, 0, len(g.gaps))
	for k, n := range g.gaps {
		gaps = append(gaps, kv{k, n})
	}
	slices.SortFunc(gaps, func(a, b kv) int { return b.n - a.n })
	fmt.Fprintf(os.Stderr, "rugaroo: typegraph: %d methods, %d evaluations, %d slots, %v\n", len(g.methods), steps, len(g.nodes), took.Round(time.Millisecond))
	g.agreement()
	fmt.Fprintf(os.Stderr, "rugaroo: typegraph params: %d same, %d differ, %d graph has none, %d compiler untyped where graph typed, of %d\n", pSame, pDiff, pGraphNone, pCompAny, pTotal)
	fmt.Fprintf(os.Stderr, "rugaroo: typegraph returns: %d same, %d differ, %d graph has none, %d compiler untyped where graph typed, of %d\n", rSame, rDiff, rGraphNone, rCompAny, rTotal)
	for i, kvp := range gaps {
		if i == 15 {
			break
		}
		fmt.Fprintf(os.Stderr, "rugaroo: typegraph gap %6d %s\n", kvp.n, kvp.k)
	}
	for _, d := range diffs {
		fmt.Fprintf(os.Stderr, "rugaroo: typegraph %s\n", d)
	}
}

// lookupClass resolves a constant by the method's lexical scope, as Ruby does; a non-class constant is nil here.
func (g *typeGraph) lookupClass(m *Method, n parser.Node) (k *Class) {
	defer func() {
		if recover() != nil { // lookupConst reports an unknown name as a compile error: the graph only skips it
			k = nil
		}
	}()
	k, _ = g.c.lookupConst(m.File, n, m.Scope)
	return k
}

// sameType compares the way a reader would: the compiler's Self and an owner's own type parameters name the graph's concrete class.
func sameType(m *Method, compiler, graph Type) bool {
	if typeString(compiler) == typeString(graph) {
		return true
	}
	if m.Owner == nil {
		return false
	}
	env := map[string]Type{"Self": TClass{C: m.Owner}}
	if tc, ok := graph.(TClass); ok && tc.C == m.Owner && len(tc.Args) == 0 {
		return isVarOf(compiler, m.Owner) || typeString(subst(compiler, env)) == typeString(graph)
	}
	return typeString(subst(compiler, env)) == typeString(graph)
}

// isVarOf: the compiler's `Array[E]` in an Array method is the receiver itself.
func isVarOf(t Type, cls *Class) bool {
	tc, ok := t.(TClass)
	return ok && tc.C == cls && slices.ContainsFunc(tc.Args, func(a Type) bool { _, v := a.(TVar); return v })
}

// elemOf is what one target of a multiple assignment from v holds.
func elemOf(v Type) Type {
	if tc, ok := v.(TClass); ok && tc.C.RubyName == "Array" && len(tc.Args) == 1 {
		return tc.Args[0]
	}
	if tt, ok := v.(TTuple); ok && len(tt.Elems) > 0 {
		return joinAllUnion(tt.Elems)
	}
	return TAny{}
}

// lookupConstVal is a value constant n names from m's scope, so a constant assigned in another file is one slot.
func (g *typeGraph) lookupConstVal(m *Method, n parser.Node) (cls *Class, k *Const) {
	defer func() {
		if recover() != nil {
			cls, k = nil, nil
		}
	}()
	return g.c.lookupConst(m.File, n, m.Scope)
}

// annotatedIvar: only a declared type stands without a write; a discovered one is the compiler's answer, not an input.
func (g *typeGraph) annotatedIvar(iv *Ivar) bool {
	return iv.Owner != nil && iv.Owner.File != nil && iv.Owner.File.isPrelude()
}

// agreement is the comparison on what the compiler typed: of those slots, how many the graph gives the same type.
func (g *typeGraph) agreement() {
	var pTyped, pSame, pNone, rTyped, rSame, rNone int
	for _, m := range g.methods {
		for _, p := range m.Params {
			if p.Pending == "" || p.Type == nil || isAny(p.Type) {
				continue
			}
			pTyped++
			switch gt := g.nodes[paramSlot(p.Pending)]; {
			case gt == nil || gt.t == nil:
				pNone++
			case sameType(m, p.Type, gt.t):
				pSame++
			}
		}
		if m.inferRet && m.Ret != nil && !isAny(m.Ret) {
			rTyped++
			switch gt := g.nodes[retSlot(m)]; {
			case gt == nil || gt.t == nil:
				rNone++
			case sameType(m, m.Ret, gt.t):
				rSame++
			}
		}
	}
	pct := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return 100 * float64(a) / float64(b)
	}
	fmt.Fprintf(os.Stderr, "rugaroo: typegraph agreement where the compiler typed: params %d of %d same (%.0f%%, %d never reached), returns %d of %d same (%.0f%%, %d never reached)\n",
		pSame, pTyped, pct(pSame, pTyped), pNone, rSame, rTyped, pct(rSame, rTyped), rNone)
}

// positional drops the trailing keyword hash a call's argument list ends with: it is not a positional argument.
func positional(n *parser.CallNode, args []Type) []Type {
	ca := callArgs(n)
	if len(ca) > 0 && len(args) == len(ca) {
		if _, kw := ca[len(ca)-1].(*parser.KeywordHashNode); kw {
			return args[:len(args)-1]
		}
	}
	return args
}

// intrinsic is what the codegen answers for a name with no method entry (raise, is_a?, class, ...); false for any other name.
func intrinsic(g *typeGraph, name string, recv Type) (Type, bool) {
	switch name {
	case "raise", "fail", "exit", "abort", "throw":
		return nil, true
	case "is_a?", "kind_of?", "instance_of?", "respond_to?", "nil?", "==", "!=", "!", "equal?", "eql?", "frozen?", "block_given?", "===", "=~":
		return g.cls("Boolean"), true
	case "class":
		if tc, ok := recv.(TClass); ok && tc.C.meta != nil {
			return TClass{C: tc.C.meta}, true
		}
	case "to_s", "inspect":
		if _, ok := recv.(TClass); ok {
			return g.cls("String"), true
		}
	case "freeze", "dup", "clone", "itself", "tap":
		return recv, true
	}
	return nil, false
}

// attrCall reads or writes the ivar an attr reader or writer stands for, so it is the ivar's slot and not a signature.
func (g *typeGraph) attrCall(e *entry, args []Type) (Type, bool) {
	if ivar, ok := strings.CutSuffix(e.M.Name, "="); ok && e.M.Kind == kindAttrWriter && len(args) == 1 {
		g.flow(ivarSlot(e.Owner, "@"+ivar), args[0])
		return args[0], true
	}
	if e.M.Kind != kindAttrReader {
		return nil, false
	}
	if t := g.read(ivarSlot(e.Owner, "@"+e.M.Name)); t != nil {
		return t, true
	}
	if e.M.Ret != nil && !mentionsVar(e.M.Ret) {
		return e.M.Ret, true
	}
	return nil, true
}

// tgKnown stands untyped for a part nothing has typed yet: a type holds no nil part.
func tgKnown(t Type) Type {
	if t == nil {
		return TAny{}
	}
	return t
}

// complete reports a type with no nil part anywhere, which unify and String may then read.
func complete(t Type) bool {
	switch t := t.(type) {
	case nil:
		return false
	case TClass:
		return !slices.ContainsFunc(t.Args, func(a Type) bool { return !complete(a) })
	case TOpt:
		return complete(t.Elem)
	case TUnion:
		return !slices.ContainsFunc(t.Members, func(a Type) bool { return !complete(a) })
	case TTuple:
		return !slices.ContainsFunc(t.Elems, func(a Type) bool { return !complete(a) })
	case TAny, TFunc, TNil, TVar, TVoid:
	}
	return true
}
