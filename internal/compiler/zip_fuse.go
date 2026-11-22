package compiler

import (
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Decision 389: `xs.zip(ys)` chained into a block builds no Array of pairs.

// zipConsumers are the Enumerator methods `xs.zip(ys).m { |x, y| }` may run on `__zip_view_enum`'s pairs: each returns what Array's does.
var zipConsumers = map[string]bool{
	"sum": true, "count": true, "any?": true, "all?": true, "none?": true, "map": true, "collect": true,
	"filter_map": true, "flat_map": true, "select": true, "filter": true, "reject": true,
}

// zipCall is n as a blockless `recv.zip(arg)`: one plain argument, no `&.`.
func zipCall(n parser.Node) *parser.CallNode {
	z, ok := n.(*parser.CallNode)
	if !ok || z.Name != "zip" || z.Block != nil || z.Receiver == nil || z.IsSAFE_NAVIGATION() {
		return nil
	}
	if args := callArgs(z); len(args) != 1 || isSplatOrBlockArg(args[0]) {
		return nil
	}
	return z
}

// destructures: the block takes two or more plain required params, so it never sees the pair itself.
func destructures(blk *parser.BlockNode) bool {
	bp, ok := blk.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil {
		return false
	}
	p := bp.Parameters
	if len(p.Optionals) > 0 || p.Rest != nil || len(p.Posts) > 0 || len(p.Keywords) > 0 || p.KeywordRest != nil || p.Block != nil || len(p.Requireds) < 2 {
		return false
	}
	for _, r := range p.Requireds {
		if _, ok := r.(*parser.RequiredParameterNode); !ok {
			return false
		}
	}
	return true
}

// preludeArrayZip: z's receiver is an Array whose zip is the prelude's.
func (f *fctx) preludeArrayZip(z *parser.CallNode) bool {
	var rt Type
	f.probe(func() { rt = f.genExpr(z.Receiver, nil).typ })
	if !isClass(rt, "Array") {
		return false
	}
	e := f.resolve(rt, "zip")
	return e != nil && e.M.File != nil && e.M.File.isPrelude()
}

// noteZipChain marks `xs.zip(ys).m { |x, y| }`'s inner call for zipViewEnum, before either call generates.
func (f *fctx) noteZipChain(n *parser.CallNode) {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok || !zipConsumers[n.Name] || n.IsSAFE_NAVIGATION() || !destructures(blk) {
		return
	}
	z := zipCall(n.Receiver)
	if z == nil || !f.preludeArrayZip(z) {
		return
	}
	en := f.c.classes["Enumerator"]
	if ce := en.lookup(n.Name); ce == nil || ce.M.File == nil || !ce.M.File.isPrelude() {
		return
	}
	if f.c.windowChain == nil {
		f.c.windowChain = map[*parser.CallNode]string{}
	}
	f.c.windowChain[z] = n.Name
}

// zipViewEnum is a noteZipChain-marked `xs.zip(ys)`'s twin.
func (f *fctx) zipViewEnum(n parser.Node, e *entry, recvT Type, block parser.Node) *entry {
	cn, ok := n.(*parser.CallNode)
	if !ok || block != nil || e.M.Name != "zip" || e.M.File == nil || !e.M.File.isPrelude() {
		return nil
	}
	if _, ok := f.c.windowChain[cn]; !ok {
		return nil
	}
	return f.resolve(recvT, "__zip_view_enum")
}

// zipEachBlock is a statement `xs.zip(ys).each { ... }` as `xs.__zip_each(ys) { ... }`: the same pairs, and nothing reads each's value.
func (f *fctx) zipEachBlock(n *parser.CallNode, t tail) *parser.CallNode {
	if t.kind != tailNone || n.Name != "each" || n.IsSAFE_NAVIGATION() {
		return n
	}
	if _, ok := n.Block.(*parser.BlockNode); !ok {
		return n
	}
	z := zipCall(n.Receiver)
	if z == nil || !f.preludeArrayZip(z) {
		return n
	}
	return f.c.rewrite(n, func() parser.Node {
		return &parser.CallNode{Location: n.Location, Receiver: z.Receiver, Name: "__zip_each", Arguments: z.Arguments, Block: n.Block}
	}).(*parser.CallNode)
}

// zipLoopHead is `__zip_block`/`__zip_each` as an index loop, with the copies their prelude bodies take.
func (f *fctx) zipLoopHead(m *Method, recv expr, codes []string, gp string) string {
	if m.Owner != f.c.classes["Array"] || m.Name != "__zip_block" && m.Name != "__zip_each" || len(codes) != 1 {
		return ""
	}
	each := m.Name == "__zip_each"
	xa, ya, ys, i := f.newTmp(), f.newTmp(), f.newTmp(), f.newTmp()
	f.emit("%s, %s := %s, %s", xa, ya, recv.code, codes[0]) // both evaluated before zip reads either, as in MRI
	cur := xa + ".s"                                        // the Array's own: MRI reads self at each step
	if each {
		cur = f.newTmp()
		f.emit("%s := slices.Clone(%s.s)", cur, xa)
	}
	f.emit("%s := rbZipTake(%s, %s.s)", ys, cur, ya)
	head := fmt.Sprintf("for %s := range %s {", i, cur)
	if !each { // the length re-read each step: the block may grow or shrink self
		head = fmt.Sprintf("for %s := 0; ; %s++ {\nif %s >= len(%s) {\nbreak\n}", i, i, i, cur)
	}
	if gp == "_" {
		return head
	}
	pair := "rbZipPair"
	if f.zipVal {
		pair = "rbZipPairVal"
	}
	return fmt.Sprintf("%s\n%s := %s(%s, %s, %s)", head, gp, pair, cur, ys, i)
}
