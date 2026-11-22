package compiler

import "github.com/danielgatis/go-ruby-prism/parser"

// Decision 386: a block that provably cannot keep, mutate or hand on its window gets one Array refilled per window.

// windowState is the compile's window-reuse memo.
type windowState struct {
	windowSafeMemo map[*parser.BlockNode]bool
	windowChain    map[*parser.CallNode]string // `xs.each_cons(n)` whose Enumerator a whitelisted consumer runs at once, to that consumer's name
}

// windowConsumers are the Enumerator methods a `xs.each_cons(n).m { |w| ... }` chain may reuse the window for: each runs the block once per window and keeps no element. The value is whether the block's value is used at once (a sum, a truth test, dropped) rather than kept (map's elements).
var windowConsumers = map[string]bool{
	"each": true, "count": true, "sum": true, "any?": true, "all?": true, "none?": true,
	"map": false, "collect": false, "filter_map": false, "flat_map": false,
}

// windowReaders are the Array methods w may receive anywhere in the block: none mutates w, returns it, or returns storage it shares (each answers a new value, or a copy: min/max copy the element, map/sort/dup build a new Array).
var windowReaders = map[string]bool{
	"sum": true, "size": true, "length": true, "count": true, "include?": true, "empty?": true, "any?": true,
	"all?": true, "none?": true, "join": true, "to_s": true, "inspect": true, "hash": true, "==": true, "!=": true,
	"eql?": true, "index": true, "find_index": true, "map": true, "inject": true, "reduce": true, "sort": true,
	"reverse": true, "uniq": true, "min": true, "max": true, "dup": true,
}

// windowPtrReaders answer an element as a pointer into the window (`&self.s[i]`), so w may receive them only where the value is used at once: the receiver or an operand of an operator, a condition, a dropped statement.
var windowPtrReaders = map[string]bool{"first": true, "last": true, "[]": true, "at": true}

// windowValueOps use their receiver and operands at once and answer a new value.
var windowValueOps = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "**": true, "==": true, "!=": true, "<": true, ">": true,
	"<=": true, ">=": true, "<=>": true, "&": true, "|": true, "^": true, "!": true, "-@": true, "+@": true, "~": true,
	"to_s": true, "to_i": true, "to_f": true, "inspect": true, "nil?": true, "zero?": true, "positive?": true,
	"negative?": true, "even?": true, "odd?": true, "abs": true, "hash": true,
}

// windowSafe reports whether blk cannot let its yielded window escape; immediate says the caller uses the block's value at once (see windowConsumers).
func (c *Compiler) windowSafe(blk *parser.BlockNode, immediate bool) bool {
	if ok, seen := c.windowSafeMemo[blk]; seen {
		return ok
	}
	ok := windowBlockSafe(blk, immediate)
	if c.windowSafeMemo == nil {
		c.windowSafeMemo = map[*parser.BlockNode]bool{}
	}
	c.windowSafeMemo[blk] = ok
	return ok
}

func windowBlockSafe(blk *parser.BlockNode, immediate bool) bool {
	if blk.Parameters == nil {
		return true
	}
	bp, ok := blk.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil { // `it`, `_1`: not proven
		return false
	}
	p := bp.Parameters
	if len(p.Optionals) > 0 || p.Rest != nil || len(p.Posts) > 0 || len(p.Keywords) > 0 || p.KeywordRest != nil || p.Block != nil {
		return false
	}
	if len(p.Requireds) == 0 {
		return false
	}
	if len(p.Requireds) > 1 {
		return true // destructured: each param an element, copied out of the window (rbSplatVal)
	}
	req, ok := p.Requireds[0].(*parser.RequiredParameterNode)
	if !ok {
		return false
	}
	w := windowCheck{name: req.Name}
	return w.ok(blk.Body, immediate, false)
}

type windowCheck struct{ name string }

func (w windowCheck) isW(n parser.Node) bool {
	r, ok := n.(*parser.LocalVariableReadNode)
	return ok && r.Name == w.name
}

// ok walks n: immediate says n's value is used at once, nested that n is inside a block, lambda or def in the window's block, where w may not appear at all (a closure could keep it).
func (w windowCheck) ok(n parser.Node, immediate, nested bool) bool {
	switch x := n.(type) {
	case nil:
		return true
	case *parser.LocalVariableReadNode:
		return x.Name != w.name
	case *parser.LocalVariableWriteNode:
		return x.Name != w.name && w.ok(x.Value, false, nested)
	case *parser.LocalVariableTargetNode:
		return x.Name != w.name
	case *parser.LocalVariableOperatorWriteNode:
		return x.Name != w.name && w.ok(x.Value, false, nested)
	case *parser.LocalVariableAndWriteNode:
		return x.Name != w.name && w.ok(x.Value, false, nested)
	case *parser.LocalVariableOrWriteNode:
		return x.Name != w.name && w.ok(x.Value, false, nested)
	case *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
		nested = true
	case *parser.StatementsNode:
		for i, s := range x.Body {
			if !w.ok(s, immediate || i < len(x.Body)-1, nested) { // a statement before the last: its value is dropped
				return false
			}
		}
		return true
	case *parser.IfNode, *parser.UnlessNode, *parser.ElseNode:
		return w.branchOK(x, immediate, nested)
	case *parser.CallNode:
		return w.callOK(x, immediate, nested)
	}
	for _, ch := range n.CompactChildNodes() {
		if !w.ok(ch, false, nested) {
			return false
		}
	}
	return true
}

// branchOK: a typed nil field is a non-nil Node, so each is checked before it is walked.
func (w windowCheck) branchOK(n parser.Node, immediate, nested bool) bool {
	switch x := n.(type) {
	case *parser.IfNode:
		return w.ok(x.Predicate, true, nested) && (x.Statements == nil || w.ok(x.Statements, immediate, nested)) && w.ok(x.Subsequent, immediate, nested)
	case *parser.UnlessNode:
		return w.ok(x.Predicate, true, nested) && (x.Statements == nil || w.ok(x.Statements, immediate, nested)) && (x.ElseClause == nil || w.ok(x.ElseClause, immediate, nested))
	case *parser.ElseNode:
		return x.Statements == nil || w.ok(x.Statements, immediate, nested)
	}
	return false
}

func (w windowCheck) callOK(x *parser.CallNode, immediate, nested bool) bool {
	if x.Receiver == nil && (x.Name == "binding" || x.Name == "eval" || x.Name == "local_variables") {
		return false
	}
	op := windowValueOps[x.Name]
	if w.isW(x.Receiver) {
		switch {
		case nested:
			return false
		case windowReaders[x.Name], (x.Name == "first" || x.Name == "last") && len(callArgs(x)) > 0:
		case windowPtrReaders[x.Name] && immediate:
		default:
			return false
		}
	} else if !w.ok(x.Receiver, op, nested) {
		return false
	}
	if x.Arguments != nil {
		for _, a := range x.Arguments.Arguments {
			if !w.ok(a, op, nested) {
				return false
			}
		}
	}
	return w.ok(x.Block, false, true)
}

// noteWindowChain marks `xs.each_cons(n).m { ... }`'s inner call when m is a windowConsumers method and its block is windowSafe, before either call generates.
func (f *fctx) noteWindowChain(n *parser.CallNode) {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		return
	}
	r, ok := n.Receiver.(*parser.CallNode)
	if !ok || r.Block != nil || r.Name != "each_cons" && r.Name != "each_slice" || r.IsSAFE_NAVIGATION() || n.IsSAFE_NAVIGATION() {
		return
	}
	if args := callArgs(r); len(args) != 1 || isSplatOrBlockArg(args[0]) {
		return
	}
	immediate, ok := windowConsumers[n.Name]
	if !ok || !f.c.windowSafe(blk, immediate) {
		return
	}
	if f.c.windowChain == nil {
		f.c.windowChain = map[*parser.CallNode]string{}
	}
	f.c.windowChain[r] = n.Name
}

func isSplatOrBlockArg(n parser.Node) bool {
	switch n.(type) {
	case *parser.SplatNode, *parser.BlockArgumentNode, *parser.KeywordHashNode:
		return true
	}
	return false
}

// windowMethod is the prelude's Enumerable#each_cons or #each_slice.
func (c *Compiler) windowMethod(m *Method) bool {
	return m.Owner != nil && m.Owner == c.classes["Enumerable"] && (m.Name == "each_cons" || m.Name == "each_slice") &&
		m.File != nil && m.File.isPrelude()
}

// windowViewIter is `xs.each_cons(n) { ... }`'s reusing twin, when the block is windowSafe.
func (f *fctx) windowViewIter(n *parser.CallNode, e *entry, recvT Type) *entry {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok || !f.c.windowMethod(e.M) || !f.c.windowSafe(blk, true) {
		return nil
	}
	return f.resolve(recvT, "__"+e.M.Name+"_view")
}

// windowViewEnum is a noteWindowChain-marked blockless `xs.each_cons(n)`'s reusing twin, when the consumer is the prelude's own Enumerator method.
func (f *fctx) windowViewEnum(n parser.Node, e *entry, recvT Type, block parser.Node) *entry {
	cn, ok := n.(*parser.CallNode)
	if !ok || block != nil {
		return nil
	}
	consumer, ok := f.c.windowChain[cn]
	if !ok || !f.c.windowMethod(e.M) {
		return nil
	}
	en := f.c.classes["Enumerator"]
	if en == nil {
		return nil
	}
	if ce := en.lookup(consumer); ce == nil || ce.M.File == nil || !ce.M.File.isPrelude() {
		return nil
	}
	return f.resolve(recvT, "__"+e.M.Name+"_view_enum")
}

// windowSplat is whether a block's destructured params copy their elements out (rbSplatVal): a windowSafe block may be handed a reused window.
func (f *fctx) windowSplat(block parser.Node) bool {
	blk, ok := block.(*parser.BlockNode)
	return ok && f.c.windowSafeMemo[blk]
}
