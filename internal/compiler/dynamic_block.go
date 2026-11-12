package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// dynBlockable reports a block-taking method a dynamic call with a block reaches (decision 243): only names some gem
// call site sends with a block get such wrappers, and not iterators (a range loop) or blocks bound to another self.
func (c *Compiler) dynBlockable(name string, m *Method) bool {
	// ponytail: Kernel's (`tap`, `then`, `open`) would get a wrapper on every class (+5,000 functions in brew); a class's
	// own override still gets one
	if m.Owner != nil && m.Owner.universal {
		return false
	}
	// an optional block's method called dynamically without one (matrix's `r.column(0)`) takes nil for it (decision 414)
	return (c.dynBlockNames[name] || c.dynBlockAll || m.Block.Optional && !m.Iterator) && (!m.Iterator || !m.hasKeywords()) && (!m.Block.Rest || anyRestBlock(m.Block))
}

// anyRestBlock is a gem's unannotated `&block`, `(*untyped) -> untyped`: the dynamic call's rbBlockArg as it is.
func anyRestBlock(b *BlockSig) bool {
	return b.Rest && len(b.Params) == 1 && isAny(b.Params[0]) && isAny(b.Ret)
}

// dynBlockArg is the block a dynamic call passes as its last argument (rbBlockArg), as the method's typed block: each
// value boxed in, its result asserted out; nil when the call passed none and the block is optional (decision 243).
func (c *Compiler) dynBlockArg(f *fctx, m *Method, env map[string]Type) parser.Node {
	if anyRestBlock(m.Block) { // the call's block as it is, a nil Proc when it gave none
		f.emit("var blkFn_ func(...any) any")
		f.emit("if blk_ != nil {")
		f.emit("\tblkFn_ = blk_.fn")
		f.emit("}")
		ft := TFunc{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}}
		return &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: "blkFn_", typ: ft}}}
	}
	params := make([]string, len(m.Block.Params))
	vals := make([]string, len(m.Block.Params))
	ps := make([]Type, len(m.Block.Params))
	for i, p := range m.Block.Params {
		ps[i] = subst(p, env)
		params[i] = fmt.Sprintf("a%d_ %s", i, c.goType(ps[i]))
		vals[i] = fmt.Sprintf("a%d_", i)
	}
	ret := subst(m.Block.Ret, env)
	args := strings.Join(vals, ", ")
	var self Type
	if m.Block.Self != nil { // a bound block takes self first; only a bound rbBlockArg runs on it (decision 281)
		self = subst(m.Block.Self, env)
		params = append([]string{"self_ " + c.goType(self)}, params...)
		args = "rbBoundArgs(blk_, " + strings.Join(append([]string{"self_"}, vals...), ", ") + ")..."
	}
	body := "blk_.fn(" + args + ")"
	sig := "func(" + strings.Join(params, ", ") + ")"
	switch {
	case isVoid(ret):
	case isAny(ret):
		sig += " any"
		body = "return " + body
	default:
		sig += " " + c.goType(ret)
		body = fmt.Sprintf("return rbAs[%s](%s, %q)", c.goType(ret), body, ret.String())
	}
	ft := TFunc{Params: ps, Ret: ret, Proc: true, Self: self}
	f.emit("blkFn_ := %s {", sig)
	f.emit("\tif blk_ == nil {")
	f.emit("\t\tpanic(NewLocalJumpError(Ref(String(\"no block given (yield)\"))))")
	f.emit("\t}")
	f.emit("\t%s", body)
	f.emit("}")
	if m.Block.Optional { // the method sees no block, as `block_given?` does
		f.emit("if blk_ == nil {\n\tblkFn_ = nil\n}")
	}
	f.emit("_ = blkFn_")
	if m.Block.Optional && !m.Iterator && self == nil { // as it is: a Proc wrapped around it would never be nil (decision 307)
		return &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: "blkFn_", typ: TFunc{Params: ps, Ret: ret}}}}
	}
	return &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: "(&blkFn_)", typ: ft}}} // a Proc value is boxed: .call derefs it
}

// dynCallBlock takes a dynamic call's block off its arguments (decision 243): the method's block, or dropped when it
// takes none, as MRI ignores it (decision 251).
func (c *Compiler) dynCallBlock(f *fctx, m *Method, env map[string]Type) parser.Node {
	if m.Block == nil && m.Kind == kindSynth && m.Name == "new" && m.Owner.metaOf != nil {
		// a synthesized new takes initialize's block (concurrent-ruby's `new(delayed, 1, *args, &block)` built at run
		// time, decision 255): a block dropped here left BlockedTaskPromise's @Task nil
		if init := m.Owner.metaOf.lookup("initialize"); init != nil && init.M.Block != nil {
			f.emit("blk_, args := rbPopBlock(args)")
			return c.dynBlockArg(f, init.M, env)
		}
	}
	if m.Block != nil {
		f.emit("blk_, args := rbPopBlock(args)")
		return c.dynBlockArg(f, m, env)
	}
	if c.dynBlockNames[m.Name] || c.dynBlockAll { // a computed send with a block may reach any name (Homebrew's cask stanzas)
		f.emit("_, args = rbPopBlock(args)")
	}
	return nil
}

// dynBlockTwin routes a dynamic call that brings a block to the class's `__<name>_block` twin, when the method itself
// takes none: kramdown's `el.value.gsub(/\A\n/) { ... }` on an untyped String is String#__gsub_block (decision 330).
func (c *Compiler) dynBlockTwin(f *fctx, e *entry, recv expr, env map[string]Type) {
	m, cls := e.M, classOf(recv.typ)
	if m.Block != nil || cls == nil || strings.HasPrefix(m.Name, "__") || !c.dynBlockNames[m.Name] && !c.dynBlockAll {
		return
	}
	tw := cls.lookup("__" + overloadBase(m.Name) + "_block")
	if tw != nil && tw.M.Owner != m.Owner && (m.File == nil || !m.File.isPrelude()) {
		return // a gem class's own method shadows an included module's twin: MRI ignores the block (MatchLength#max)
	}
	if tw == nil || tw.M.Block == nil || tw.M.generic() || tw.M.hasKeywords() ||
		slices.ContainsFunc(tw.M.Params, func(p Param) bool { return p.Rest || p.Default != nil }) {
		return
	}
	c.resolveMethod(tw.M)
	env = twinEnv(tw.M, env)
	f.emit("if blk_, args := rbPopBlock(args); blk_ != nil {")
	f.indent++
	f.emit("rbArity(len(args), %d, %d)", len(tw.M.Params), len(tw.M.Params))
	blk := c.dynBlockArg(f, tw.M, env)
	nodes := make([]parser.Node, len(tw.M.Params))
	for i, p := range tw.M.Params {
		pt := subst(p.Type, env)
		nodes[i] = &exprNode{e: expr{code: c.dynArg(pt, i), typ: pt}}
	}
	res := f.callEntry(&parser.NilNode{}, tw, recv, nodes, blk)
	f.emit("return %s", f.coerce(&parser.NilNode{}, res, TAny{}))
	f.indent--
	f.emit("}")
}

// twinEnv binds what the twin's signature names beyond the method's own env to untyped: an Enumerable includer with its
// own `max` reaches Enumerable#__max_block, whose `E` the class's env does not carry (regexp_parser's MatchLength).
func twinEnv(tw *Method, env map[string]Type) map[string]Type {
	var vars []string
	for _, p := range tw.Params {
		freeVars(p.Type, &vars)
	}
	for _, p := range tw.Block.Params {
		freeVars(p, &vars)
	}
	freeVars(tw.Block.Ret, &vars)
	out := maps.Clone(env)
	for _, v := range vars {
		if _, ok := out[v]; !ok && v != "Self" {
			if out == nil {
				out = map[string]Type{}
			}
			out[v] = TAny{}
		}
	}
	return out
}

// dynIterCall is a dynamic call with a block of an iterator method: a range over it, each yield into the call's block.
// MRI's value is the method's, often the block's (`synchronize { ... }`): the last block value stands in (decision 243).
func (c *Compiler) dynIterCall(f *fctx, e *entry, recv expr, env map[string]Type, nodes []parser.Node) {
	codes, _ := f.genArgs(&parser.NilNode{}, e.M, env, nodes, nil) // fills defaults and rbArgc as a typed call does
	vars := make([]string, len(e.M.Block.Params))
	for i := range vars {
		vars[i] = fmt.Sprintf("y%d_", i)
	}
	call := f.callCode(e, recv, codes, env)
	f.emit("var last_ any")
	f.emit("if blk_ == nil {")
	if len(vars) == 1 && e.M.File.isPrelude() && !mentionsVar(subst(e.M.Block.Params[0], env)) { // a prelude iterator without a block answers an Enumerator, as iterEnum's typed call does; not over a type variable, whose Enumerator would instantiate itself (Homebrew's `each_strongly_connected_component.to_a` on a TSort includer held untyped)
		f.emit("\treturn rbEnumOf(%s, any(self), %q, nil, nil)", call, e.M.Name)
	} else {
		f.emit("\tpanic(NewLocalJumpError(Ref(String(\"no block given (yield)\"))))")
	}
	f.emit("}")
	f.emit("%s", rangeHead(strings.Join(vars, ", "), call))
	f.emit("\tlast_ = blk_.fn(%s)", strings.Join(vars, ", "))
	f.emit("}")
	f.emit("return last_")
}
