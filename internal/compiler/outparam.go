package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// outParamName is an out parameter's Go name: the *String the caller's local is reached through.
func outParamName(p Param, i int) string { return goParamName(p.Name, i) + "_out" }

// markOutParams finds the String parameters a class method or top-level def appends to and never rebinds (decision 405);
// in a gem, a private instance method no subclass redefines qualifies too (prism's Lexer#escape_read(result, ...)), since
// only its own class calls it. A gem parameter inference typed String in an earlier round (decision 146) is one here.
func (c *Compiler) markOutParams() {
	mark := func(m *Method) {
		if m.Node == nil || m.File == nil || m.File.isPrelude() {
			return
		}
		if m.Owner != nil && m.Owner.metaOf == nil { // an instance method: an override or a dynamic call would pass a String where a *String is taken
			if !c.gemFile(m.File) || !m.Private || m.Owner.IsModule || m.Owner.descendantDefines(m.Name, true) {
				return
			}
		}
		for i := range m.Params {
			p := &m.Params[i]
			if p.Rest || p.Keyword || p.KwRest || p.Default != nil || !isClass(p.Type, "String") {
				continue
			}
			p.Out = appendsTo(m.Node.Body, p.Name)
		}
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			mark(m)
		}
	}
	for _, m := range c.topDefList {
		mark(m)
	}
}

// appendsTo: body has `name << x`, `name.concat(...)` or `name.append_as_bytes(...)` and never assigns name.
func appendsTo(body parser.Node, name string) bool {
	appends, rebinds := false, false
	walkNode(body, func(n parser.Node) (bool, bool) {
		switch t := n.(type) {
		case *parser.CallNode:
			if lv, ok := t.Receiver.(*parser.LocalVariableReadNode); ok && lv.Name == name && strAppendNames[t.Name] {
				appends = true
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
		if lv == nil || off+i >= len(codes) {
			f.errorf(n, "%s appends to its `%s` parameter, so it takes a local String variable (decision 405)", m.Name, p.Name)
			return
		}
		if f.lenient() { // a gem caller: no alias proof and no shared buffer; the callee rebinds its copy and writes it back on return
			codes[off+i] = "rbStrOut{&" + f.genExpr(lv, nil).code + ", nil}"
			continue
		}
		if why := f.appendWhy(lv.Name); why != "" {
			f.errorf(n, "%s appends to `%s`: Strings are immutable in rugaroo, and %s (decision 405)", m.Name, lv.Name, why)
		}
		if li := f.localInfo(lv.Name); li != nil {
			li.appended = true // its buffer goes along, so the callee appends in place
		}
		codes[off+i] = "rbStrOut{&" + f.genExpr(lv, nil).code + ", " + f.appendBufRef(lv.Name) + "}"
	}
}

// outCallArg: c passes arg to an out parameter of the method it calls (decision 405).
func (f *fctx) outCallArg(c *parser.CallNode, arg parser.Node) bool {
	idx := -1
	for i, a := range callArgs(c) {
		if a == arg {
			idx = i
		}
	}
	if idx < 0 {
		return false
	}
	recv := f.selfType
	if c.Receiver != nil {
		recv = f.probeType(c.Receiver, nil)
	}
	e := f.resolve(recv, c.Name)
	return e != nil && idx < len(e.M.Params) && e.M.Params[idx].Out
}
