package compiler

import (
	"reflect"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// procRunNames are the calls that run a Proc value with the arguments as given.
var procRunNames = map[string]bool{"call": true, "yield": true, "[]": true, "===": true}

// lowerProcParams: a Proc literal with keyword or `&b` parameters, held in a local that is only ever called, becomes a
// Proc of positional parameters, and each call passes the keywords and the block by position (decision 456): a Proc's
// Go type has no keyword or block slot.
func lowerProcParams(root parser.Node) {
	prog, ok := root.(*parser.ProgramNode)
	if !ok {
		return
	}
	lowerProcFrame(prog.Statements)
	walkFrames(prog.Statements, lowerProcFrame)
}

// walkFrames calls fn with each def body nested in n.
func walkFrames(n parser.Node, fn func(parser.Node)) {
	forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) {
		c := child.Interface().(parser.Node)
		if d, ok := c.(*parser.DefNode); ok {
			fn(d.Body)
		}
		walkFrames(c, fn)
	})
}

// procLitParams is the parameter list of a proc/lambda literal, or nil.
func procLitParams(v parser.Node) *parser.ParametersNode {
	var bp parser.Node
	switch x := v.(type) {
	case *parser.LambdaNode:
		bp = x.Parameters
	case *parser.CallNode:
		b, ok := x.Block.(*parser.BlockNode)
		if !ok || x.Receiver != nil || x.Name != "proc" && x.Name != "lambda" || x.Arguments != nil {
			return nil
		}
		bp = b.Parameters
	}
	if p, ok := bp.(*parser.BlockParametersNode); ok {
		return p.Parameters
	}
	return nil
}

// procUse is one read of a lowered Proc's local: the call it is the receiver of.
type procUse struct {
	call *parser.CallNode
}

// lowerProcFrame lowers the Proc locals one frame (the program or a def body) assigns once.
func lowerProcFrame(body parser.Node) {
	if body == nil || reflect.ValueOf(body).IsNil() {
		return
	}
	writes := map[string][]*parser.LocalVariableWriteNode{}
	frameNodes(body, func(n, _ parser.Node, depth int) {
		if w, ok := n.(*parser.LocalVariableWriteNode); ok && int(w.Depth) == depth {
			writes[w.Name] = append(writes[w.Name], w)
		}
	})
	for name, ws := range writes {
		ps := procLitParams(ws[0].Value)
		if len(ws) != 1 || ps == nil || len(ps.Keywords) == 0 && ps.KeywordRest == nil && ps.Block == nil {
			continue
		}
		if len(ps.Optionals) > 0 || len(ps.Posts) > 0 {
			continue
		}
		if _, anon := ps.KeywordRest.(*parser.NoKeywordsParameterNode); anon {
			continue
		}
		var uses []procUse
		ok := true
		frameNodes(body, func(n, parent parser.Node, depth int) {
			r, isRead := n.(*parser.LocalVariableReadNode)
			if !isRead || r.Name != name || int(r.Depth) != depth {
				return
			}
			c, isCall := parent.(*parser.CallNode)
			if !isCall || c.Receiver != n || !procRunNames[c.Name] || !callFits(c, ps) {
				ok = false
				return
			}
			uses = append(uses, procUse{call: c})
		})
		if !ok || !keywordDefaultsSafe(ps) {
			continue
		}
		for _, u := range uses {
			rewriteProcCall(u.call, ps)
		}
		flattenProcParams(ps)
	}
}

// callFits: the call passes exactly the required positionals (any count with a rest), then keywords only by literal
// Symbol, then at most a block; no splats.
func callFits(c *parser.CallNode, ps *parser.ParametersNode) bool {
	args := callArgs(c)
	pos := args
	if len(args) > 0 {
		if _, kw := args[len(args)-1].(*parser.KeywordHashNode); kw {
			pos = args[:len(args)-1]
			for _, el := range args[len(args)-1].(*parser.KeywordHashNode).Elements {
				a, isAssoc := el.(*parser.AssocNode)
				if !isAssoc {
					return false
				}
				if _, sym := a.Key.(*parser.SymbolNode); !sym {
					return false
				}
			}
		}
	}
	if slices.ContainsFunc(pos, isSplat) || slices.ContainsFunc(pos, func(a parser.Node) bool { _, ok := a.(*parser.BlockArgumentNode); return ok }) {
		return false
	}
	if ps.Rest == nil && len(pos) != len(ps.Requireds) || ps.Rest != nil && len(pos) < len(ps.Requireds) {
		return false
	}
	if ps.KeywordRest == nil && len(args) > len(pos) { // every keyword passed must be one the literal names
		for _, el := range args[len(args)-1].(*parser.KeywordHashNode).Elements {
			if keywordIndex(ps, kwSymName(el.(*parser.AssocNode).Key)) < 0 {
				return false
			}
		}
	}
	switch c.Block.(type) {
	case nil, *parser.BlockNode, *parser.BlockArgumentNode:
		return ps.Block != nil || c.Block == nil
	}
	return false
}

func kwSymName(n parser.Node) string {
	if s, ok := n.(*parser.SymbolNode); ok {
		return s.Unescaped.Value
	}
	return ""
}

func keywordIndex(ps *parser.ParametersNode, name string) int {
	return slices.IndexFunc(ps.Keywords, func(k parser.Node) bool { return keywordName(k) == name })
}

func keywordName(k parser.Node) string {
	switch x := k.(type) {
	case *parser.RequiredKeywordParameterNode:
		return x.Name
	case *parser.OptionalKeywordParameterNode:
		return x.Name
	}
	return ""
}

// keywordDefaultsSafe: each optional keyword's default is a literal, so a call that omits it can carry a copy.
func keywordDefaultsSafe(ps *parser.ParametersNode) bool {
	for _, k := range ps.Keywords {
		if o, ok := k.(*parser.OptionalKeywordParameterNode); ok {
			switch o.Value.(type) {
			case *parser.IntegerNode, *parser.FloatNode, *parser.StringNode, *parser.SymbolNode, *parser.NilNode,
				*parser.TrueNode, *parser.FalseNode:
			default:
				return false
			}
		}
	}
	return true
}

// rewriteProcCall passes the keywords (each by its position, a default where omitted; `**kw`'s rest as a Hash) and
// the block (a proc of it, or nil) after the required positionals, before any rest.
func rewriteProcCall(c *parser.CallNode, ps *parser.ParametersNode) {
	args := callArgs(c)
	var kwh *parser.KeywordHashNode
	if len(args) > 0 {
		if k, ok := args[len(args)-1].(*parser.KeywordHashNode); ok {
			kwh, args = k, args[:len(args)-1]
		}
	}
	loc := c.Location
	given := map[string]parser.Node{}
	var rest []parser.Node
	if kwh != nil {
		for _, el := range kwh.Elements {
			a := el.(*parser.AssocNode)
			name := kwSymName(a.Key)
			if keywordIndex(ps, name) >= 0 {
				given[name] = a.Value
			} else {
				rest = append(rest, a)
			}
		}
	}
	out := slices.Clone(args[:len(ps.Requireds)])
	for _, k := range ps.Keywords {
		name := keywordName(k)
		switch v, ok := given[name]; {
		case ok:
			out = append(out, v)
		case isOptionalKeyword(k):
			out = append(out, k.(*parser.OptionalKeywordParameterNode).Value)
		default: // MRI's ArgumentError, at the call
			out = append(out, &parser.CallNode{Location: loc, Name: "raise", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{
				&parser.ConstantReadNode{Location: loc, Name: "ArgumentError"},
				&parser.StringNode{Location: parser.Location{StartOffset: loc.StartOffset}, Unescaped: parser.RubyString{Value: "missing keyword: :" + name}}}}})
		}
	}
	if ps.KeywordRest != nil {
		out = append(out, &parser.HashNode{Location: loc, Elements: rest})
	}
	if ps.Block != nil {
		switch b := c.Block.(type) {
		case *parser.BlockNode:
			out = append(out, &parser.CallNode{Location: loc, Name: "proc", Block: b})
		case *parser.BlockArgumentNode:
			out = append(out, b.Expression)
		default:
			out = append(out, &parser.NilNode{Location: loc})
		}
		c.Block = nil
	}
	out = append(out, args[len(ps.Requireds):]...)
	c.Arguments = &parser.ArgumentsNode{Location: loc, Arguments: out}
}

func isOptionalKeyword(k parser.Node) bool {
	_, ok := k.(*parser.OptionalKeywordParameterNode)
	return ok
}

// flattenProcParams makes the keywords, `**kw` and `&b` required positionals, after the literal's own and before its rest.
func flattenProcParams(ps *parser.ParametersNode) {
	reqs := slices.Clone(ps.Requireds)
	for _, k := range ps.Keywords {
		reqs = append(reqs, &parser.RequiredParameterNode{Location: k.GetLocation(), Name: keywordName(k)})
	}
	if kr, ok := ps.KeywordRest.(*parser.KeywordRestParameterNode); ok && kr.Name != nil {
		reqs = append(reqs, &parser.RequiredParameterNode{Location: kr.Location, Name: *kr.Name})
	} else if ps.KeywordRest != nil {
		reqs = append(reqs, &parser.RequiredParameterNode{Location: ps.KeywordRest.GetLocation(), Name: "_"})
	}
	if ps.Block != nil {
		name := "_"
		if ps.Block.Name != nil {
			name = *ps.Block.Name
		}
		reqs = append(reqs, &parser.RequiredParameterNode{Location: ps.Block.Location, Name: name})
	}
	ps.Requireds, ps.Keywords, ps.KeywordRest, ps.Block = reqs, nil, nil, nil
}

// frameNodes visits every node of a frame's body, not entering nested defs or classes, with its parent and the
// number of blocks between it and the frame (a local's Depth there).
func frameNodes(body parser.Node, visit func(n, parent parser.Node, depth int)) {
	var walk func(n, parent parser.Node, depth int)
	walk = func(n, parent parser.Node, depth int) {
		if n == nil || reflect.ValueOf(n).IsNil() || opensFrame(n) {
			return
		}
		visit(n, parent, depth)
		d := depth
		switch n.(type) {
		case *parser.BlockNode, *parser.LambdaNode:
			d++
		}
		forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walk(child.Interface().(parser.Node), n, d) })
	}
	walk(body, nil, 0)
}
