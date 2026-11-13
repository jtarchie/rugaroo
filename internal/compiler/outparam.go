package compiler

import (
	"fmt"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// outParamName is an out parameter's Go name: the *String the caller's local is reached through.
func outParamName(p Param, i int) string { return goParamName(p.Name, i) + "_out" }

// markOutParams (decision 405): a public gem method must override nothing, since a call through its parent passes a plain String.
func (c *Compiler) markOutParams() {
	var cands []*Method
	add := func(m *Method) {
		if m.Node == nil || m.File == nil {
			return
		}
		if m.Owner != nil && m.Owner.metaOf == nil { // an instance method: an override or a dynamic call would pass a String where a *String is taken
			if !c.gemFile(m.File) && !m.File.isPrelude() || m.Owner.IsModule || m.Owner.descendantDefines(m.Name, true) || !m.Private && overridesAncestor(m) {
				return
			}
		}
		cands = append(cands, m)
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			add(m)
		}
	}
	for _, m := range c.topDefList {
		add(m)
	}
	for changed := true; changed; {
		changed = false
		for _, m := range cands {
			for i := range m.Params {
				p := &m.Params[i]
				if p.Out || p.Rest || p.Keyword || p.KwRest || p.Default != nil || !isClass(p.Type, "String") {
					continue
				}
				if appendsTo(m.Node.Body, p.Name, func(call *parser.CallNode, j int) bool { return c.outTarget(m, call, j) }) {
					p.Out, changed = true, true
				}
			}
		}
	}
	for _, m := range cands {
		for i := range m.Params {
			if p := &m.Params[i]; p.Out {
				p.OutRet = mayReturnLocal(m.Node.Body, p.Name)
			}
		}
	}
}

// overridesAncestor: an ancestor of m's class (a superclass or an included module) defines m's name too.
func overridesAncestor(m *Method) bool {
	for _, k := range m.Owner.ancestors()[1:] {
		if k.Methods[m.Name] != nil {
			return true
		}
	}
	return false
}

// outTarget: call passes its argument j to an out parameter of the method it names statically (self's class, a top-level def, a constant's class method).
func (c *Compiler) outTarget(m *Method, call *parser.CallNode, j int) bool {
	args := callArgs(call)
	if slices.ContainsFunc(args[:j], isSplat) {
		return false
	}
	var owner *Class
	switch r := call.Receiver.(type) {
	case nil, *parser.SelfNode:
		owner = m.Owner
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		var k *Class
		if catchCompileError(func() { k, _ = c.lookupConst(m.File, r, m.Scope) }) != nil || k == nil || k.meta == nil {
			return false
		}
		owner = k.meta
	default:
		return false
	}
	var target *Method
	if owner != nil {
		for _, k := range owner.ancestors() {
			if target = k.Methods[call.Name]; target != nil {
				break
			}
		}
	}
	if target == nil && (call.Receiver == nil || owner == nil) {
		target = c.topDefs[call.Name]
	}
	return target != nil && j < len(target.Params) && target.Params[j].Out
}

// mayReturnLocal: the method's value or a `return` may be name's String (a call it is passed to may hand it back).
func mayReturnLocal(body parser.Node, name string) bool {
	var tail func(n parser.Node) bool
	tail = func(n parser.Node) bool {
		switch t := n.(type) {
		case *parser.LocalVariableReadNode:
			return t.Name == name
		case *parser.StatementsNode:
			return len(t.Body) > 0 && tail(t.Body[len(t.Body)-1])
		case *parser.ParenthesesNode, *parser.IfNode, *parser.UnlessNode, *parser.ElseNode, *parser.CaseNode, *parser.WhenNode,
			*parser.BeginNode, *parser.RescueNode, *parser.AndNode, *parser.OrNode: // every child, conditions too: may only over-count
			return slices.ContainsFunc(n.CompactChildNodes(), tail)
		case *parser.CallNode:
			return strAppendNames[t.Name] && tail(t.Receiver) || slices.ContainsFunc(callArgs(t), func(a parser.Node) bool {
				lv, ok := a.(*parser.LocalVariableReadNode)
				return ok && lv.Name == name
			})
		}
		return false
	}
	if tail(body) {
		return true
	}
	return anyNode(body, func(n parser.Node) bool {
		r, ok := n.(*parser.ReturnNode)
		return ok && r.Arguments != nil && slices.ContainsFunc(r.Arguments.Arguments, tail)
	})
}

// appendsTo: body appends to name, or passes it where passes says an out parameter takes it, and never assigns name.
func appendsTo(body parser.Node, name string, passes func(*parser.CallNode, int) bool) bool {
	appends, rebinds := false, false
	walkNode(body, func(n parser.Node) (bool, bool) {
		switch t := n.(type) {
		case *parser.CallNode:
			if lv, ok := t.Receiver.(*parser.LocalVariableReadNode); ok && lv.Name == name && (strAppendNames[t.Name] || strRebindNames[t.Name]) {
				appends = true
			}
			for j, a := range callArgs(t) {
				if lv, ok := a.(*parser.LocalVariableReadNode); ok && lv.Name == name && !appends && passes(t, j) {
					appends = true
				}
			}
		case *parser.LocalVariableWriteNode:
			rebinds = rebinds || t.Name == name
		case *parser.LocalVariableOrWriteNode:
			rebinds = rebinds || t.Name == name
		case *parser.LocalVariableAndWriteNode:
			rebinds = rebinds || t.Name == name
		case *parser.LocalVariableOperatorWriteNode:
			rebinds = rebinds || t.Name == name
		case *parser.LocalVariableTargetNode:
			rebinds = rebinds || t.Name == name
		}
		return false, !opensScope(n)
	})
	return appends && !rebinds
}

// outArgs passes a local's address for each out parameter: the callee appends to the caller's String, as in MRI.
func (f *fctx) outArgs(n parser.Node, m *Method, args []parser.Node, codes []string) {
	off := 0
	if m.calleeDefaults {
		off++
	}
	if m.kwMask() {
		off++
	}
	for i, p := range m.Params {
		if !p.Out {
			continue
		}
		var lv *parser.LocalVariableReadNode
		if i < len(args) {
			lv, _ = args[i].(*parser.LocalVariableReadNode)
		}
		if off+i >= len(codes) {
			f.errorf(n, "%s appends to its `%s` parameter, so it takes a local String variable (decision 405)", m.Name, p.Name)
		}
		if lv == nil || f.lenient() && !typeEq(f.probeType(lv, nil), f.cls("String")) {
			gemCallee := m.File != nil && f.c.gemFile(m.File)
			if !f.lenient() && (!gemCallee || i >= len(args) || !(&appendProof{f: f}).fresh(args[i])) {
				f.errorf(n, "%s appends to its `%s` parameter, so it takes a local String variable (decision 405)", m.Name, p.Name)
			}
			// ponytail: a gem's other argument (an ivar, `el.options[:raw_text] = +''`, a dynamic call's) gets the appends on a copy, as before decision 405; writing back to the expression's target is the upgrade
			tmp := f.newTmp()
			f.emit("%s := String(%s)", tmp, codes[off+i])
			codes[off+i] = "rbStrOut{&" + tmp + ", nil}"
			continue
		}
		if f.lenient() { // a gem caller: no alias proof and no shared buffer; the callee rebinds its copy and writes it back on return
			codes[off+i] = "rbStrOut{&" + f.genExpr(lv, nil).code + ", nil}"
			continue
		}
		if why := f.appendWhy(lv.Name); why != "" {
			f.proofErrorf(n, "%s appends to `%s`: Strings are immutable in rugaroo, and %s (decision 405)", m.Name, lv.Name, why)
		}
		if li := f.localInfo(lv.Name); li != nil {
			li.appended = true // its buffer goes along, so the callee appends in place
		}
		codes[off+i] = "rbStrOut{&" + f.genExpr(lv, nil).code + ", " + f.appendBufRef(lv.Name) + "}"
	}
}

// outCallArg: c passes arg to an out parameter of the method it calls (decision 405); ret: c's value may be that String.
func (f *fctx) outCallArg(c *parser.CallNode, arg parser.Node) (out, ret bool) {
	m, idx := f.callParam(c, arg)
	if m == nil || !m.Params[idx].Out {
		return false, false
	}
	return true, m.Params[idx].OutRet && mayHoldStr(m.Ret)
}

// callParam is the method c calls and the index of its parameter arg binds, or nil.
func (f *fctx) callParam(c *parser.CallNode, arg parser.Node) (*Method, int) {
	idx := slices.Index(callArgs(c), arg)
	if idx < 0 || slices.ContainsFunc(callArgs(c)[:idx], isSplat) {
		return nil, 0
	}
	recv := f.selfType
	if c.Receiver != nil {
		recv = f.probeType(c.Receiver, nil)
	}
	e := f.resolve(recv, c.Name)
	if e == nil || idx >= len(e.M.Params) {
		return nil, 0
	}
	return e.M, idx
}

// untypedOutParam names an appended-to parameter left untyped (its only callers failed an inference round), which hides the real error.
func (f *fctx) untypedOutParam(c *parser.CallNode, arg parser.Node) string {
	m, idx := f.callParam(c, arg)
	if m == nil || m.Node == nil || m.File == nil || m.File.isPrelude() {
		return ""
	}
	p := m.Params[idx]
	if p.Out || isClass(p.Type, "String") || p.Rest || p.Keyword || p.KwRest || p.Default != nil ||
		!appendsTo(m.Node.Body, p.Name, func(*parser.CallNode, int) bool { return false }) {
		return ""
	}
	return fmt.Sprintf(" (whose `%s` parameter it appends to is %s, not String: annotate it `# @rbs %s: String`)", p.Name, typeString(p.Type), p.Name)
}

// mayHoldStr: a value of t may be a String (unknown types may).
func mayHoldStr(t Type) bool {
	switch t := stripOpt(t).(type) {
	case TClass:
		return t.C.RubyName == "String"
	case TUnion:
		return slices.ContainsFunc(t.Members, mayHoldStr)
	case TNil, TVoid, TFunc, TTuple:
		return false
	case TAny, TOpt, TVar:
	}
	return true
}
