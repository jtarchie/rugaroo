package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// RUGAROO_UNTYPED_WHY (debug only): the root a dynamic call's untyped receiver came from, so work on typing gem code
// goes where the untyped values start, not where they are used.

// noteAnyOrigin records, for an untyped local, what its first untyped value was.
func (f *fctx) noteAnyOrigin(n parser.Node, name string, val expr) {
	if !f.c.env.why() || f.pass != 2 {
		return
	}
	w, ok := n.(*parser.LocalVariableWriteNode)
	if !ok {
		return
	}
	k := f.localKey(name)
	if o, seen := f.anyOrigin[k]; seen && o.text != "" {
		return
	}
	var o why
	switch {
	case isAny(val.typ):
		o = f.untypedWhy(w.Value, 1)
	case f.localInfo(name) != nil && isAny(f.localInfo(name).typ):
		o = f.localWhy(n, name, "local assigned values of different types: "+typeString(f.localInfo(name).typ)+" / "+typeString(val.typ)) // ponytail: the first type is lost once widened; record it at the widening
	default:
		return
	}
	if f.anyOrigin == nil {
		f.anyOrigin = map[localKey]why{}
	}
	f.anyOrigin[k] = o
}

// untypedWhy names the root an untyped value at n came from.
func (f *fctx) untypedWhy(n parser.Node, depth int) why {
	if depth > 6 {
		return f.exprWhy(n, "deep chain")
	}
	switch n := n.(type) {
	case nil, *parser.SelfNode:
		return why{text: "self", root: untypedRoot{kind: "self", name: f.scopeName()}}
	case *parser.ParenthesesNode:
		if b, ok := n.Body.(*parser.StatementsNode); ok && len(b.Body) == 1 {
			return f.untypedWhy(b.Body[0], depth+1)
		}
	case *parser.LocalVariableReadNode:
		if why, ok := f.anyOrigin[f.localKey(n.Name)]; ok {
			return why
		}
		return f.paramWhy(n.Name)
	case *parser.InstanceVariableReadNode:
		return f.ivarWhy(n.Name)
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return why{text: "constant", root: untypedRoot{kind: "constant", name: f.f.text(n.GetLocation())}}
	case *parser.CallNode:
		return f.callWhy(n, depth)
	}
	if recv, ok := opReceiver(n); ok && f.c.env.UntypedReport != "" { // `(h[k] ||= {})`: an element of h, or h's own root
		w := f.untypedWhy(recv, depth+1)
		w.text = nodeType(n)
		return w
	}
	return f.exprWhy(n, nodeType(n))
}

func (f *fctx) paramWhy(name string) why {
	local := untypedRoot{kind: "local", name: name + " in " + f.scopeName()}
	if f.m == nil {
		return why{text: "block param or local", root: local}
	}
	local.pos = f.m.srcPos
	label := methodLabel(f.m) + "(" + name + ")"
	i := slices.IndexFunc(f.m.Params, func(p Param) bool { return p.Name == name })
	switch {
	case i < 0 && f.blockSig != nil:
		return why{text: "block param (or local set by a non-plain write)", root: local}
	case i < 0:
		return why{text: "local set by a non-plain write", root: local}
	case f.paramWiden[name] != nil:
		return why{text: "param widened by reassignment", root: untypedRoot{kind: "param reassigned", name: label, pos: f.m.srcPos, note: f.paramWidenWhy[name]}}
	case f.m.Params[i].Pending != "":
		return why{text: "param: no typed caller " + f.m.Name + "(" + name + ")", root: untypedRoot{kind: "param", name: label, pos: f.m.srcPos, key: f.m.Params[i].Pending}}
	case f.m.declared:
		return why{text: "param: sig untyped " + f.m.Name + "(" + name + ")", root: untypedRoot{kind: "sig param", name: label, pos: f.m.srcPos, note: sigNote(f.m)}}
	}
	return why{text: "param: unannotated " + f.m.Name + "(" + name + ")", root: untypedRoot{kind: "param", name: label, pos: f.m.srcPos}}
}

func (f *fctx) callWhy(n *parser.CallNode, depth int) why {
	var rt Type
	if n.Receiver == nil {
		rt = f.selfType
	} else {
		rt = f.probeType(n.Receiver, nil)
	}
	if isAny(rt) && n.Receiver != nil {
		return f.untypedWhy(n.Receiver, depth+1) // the root is further up
	}
	if n.Name == "[]" || n.Name == "fetch" || n.Name == "dig" {
		return f.elemWhy(n.Receiver, rt, "element of "+typeString(rt), depth)
	}
	if isSelfVar(rt) && f.owner != nil {
		rt = TClass{C: f.owner}
	}
	c, ok := stripOpt(rt).(TClass)
	if !ok {
		return f.exprWhy(n, "return of "+n.Name+" on "+typeString(rt))
	}
	e := c.C.lookup(n.Name)
	label := c.C.RubyName + "#" + n.Name
	switch {
	case e == nil:
		return why{text: "return of missing " + label, root: untypedRoot{kind: "missing method", name: label}}
	case e.M.File != nil && e.M.File.isPrelude():
		if holdsAny(rt) && f.c.env.UntypedReport != "" { // `xs.first`, `h.values`: the container's elements, not the core method
			return f.elemWhy(n.Receiver, rt, "return of core "+n.Name, depth)
		}
		return why{text: "return of core " + n.Name, root: untypedRoot{kind: "core return", name: label, pos: e.M.srcPos, note: sigNote(e.M)}}
	case e.M.declared:
		return why{text: "return: sig untyped " + label, root: untypedRoot{kind: "sig return", name: label, pos: e.M.srcPos, note: sigNote(e.M)}}
	}
	root := untypedRoot{kind: "return", name: label, pos: e.M.srcPos, m: e.M}
	via := ""
	if e.M.retRoot.passes() {
		root, via = e.M.retRoot, label
	}
	if e.M.Kind == kindAttrReader && f.c.env.UntypedReport != "" { // an attr reader returns its ivar
		root, via = f.c.ivarRoot(c.C, e.M.Attr)
	}
	if e.M.retWhy != "" {
		return why{text: "return: unannotated " + label + ": " + e.M.retWhy, root: root, via: via}
	}
	return why{text: "return: unannotated " + label, root: root, via: via}
}

func typeString(t Type) string {
	if t == nil {
		return "?"
	}
	return t.String()
}

// setAnyOrigin records why for local name, when RUGAROO_UNTYPED_WHY is on.
func (f *fctx) setAnyOrigin(name string, o why) {
	if !f.c.env.why() {
		return
	}
	if f.anyOrigin == nil {
		f.anyOrigin = map[localKey]why{}
	}
	f.anyOrigin[f.localKey(name)] = o
}

// noteAnyJoin records a pass-1 write that turned local name untyped.
func (f *fctx) noteAnyJoin(n parser.Node, name string) {
	if w, ok := n.(*parser.LocalVariableWriteNode); ok && f.c.env.why() {
		f.setAnyOrigin(name, f.untypedWhy(w.Value, 1).pre("pass-1 untyped write: "))
	}
}

// blockParamWhy names the root of an untyped block parameter: the untyped receiver (or argument, for inject's memo)
// of the call n whose block it is, else what the yielding method is.
func (f *fctx) blockParamWhy(n parser.Node) why {
	call, ok := n.(*parser.CallNode)
	if _, lam := n.(*parser.LambdaNode); lam {
		return f.lambdaWhy(n, "block param of "+nodeType(n))
	}
	if !ok {
		return f.exprWhy(n, "block param of "+nodeType(n))
	}
	if call.Receiver != nil {
		if rt := f.probeType(call.Receiver, nil); isAny(rt) {
			return f.untypedWhy(call.Receiver, 1).pre("block param: receiver untyped: ")
		} else if call.Name == "inject" || call.Name == "reduce" || call.Name == "each_with_object" {
			for _, a := range callArgs(call) {
				if isAny(f.probeType(a, nil)) {
					return f.untypedWhy(a, 1).pre("block param: " + call.Name + "'s memo untyped: ")
				}
			}
		} else {
			return f.yieldWhy(call, rt, "block param: yield of "+typeString(rt)+"#"+call.Name)
		}
	}
	return f.yieldWhy(call, f.selfType, "block param: yield of "+call.Name)
}

// noteAnyTargets records why each local target of a multiple assignment from untyped value v is untyped.
func (f *fctx) noteAnyTargets(value parser.Node, v expr, lefts []parser.Node) {
	if !f.c.env.why() || !isAny(v.typ) {
		return
	}
	o := f.untypedWhy(value, 1).pre("multi-assign from: ")
	for _, l := range lefts {
		if t, ok := l.(*parser.LocalVariableTargetNode); ok {
			if _, seen := f.anyOrigin[f.localKey(t.Name)]; !seen {
				f.setAnyOrigin(t.Name, o)
			}
		}
	}
}
