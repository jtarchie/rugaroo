package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// strHoist names the Go locals holding a String local's length and rbStrLenA's byte-index bound, set before a loop that cannot change it.
type strHoist struct {
	length, lim string
	frames      int // len(rbFrames) at the loop: a block inside may run after it, when the local holds another String
}

// hoistStrLoop scans, once before a while/until loop, each String local whose length its condition reads (a scan that condition paid anyway) and the loop cannot change, so the loop's length/[] calls skip their per-call scan (decision 399); the func it returns ends the hoists.
func (f *fctx) hoistStrLoop(pred parser.Node, body *parser.StatementsNode) func() {
	if f.lenient() {
		return func() {}
	}
	var hoisted []*localInfo
	walkNode(pred, func(n parser.Node) (bool, bool) {
		c, ok := n.(*parser.CallNode)
		if !ok || c.Name != "length" && c.Name != "size" || c.Block != nil || len(callArgs(c)) != 0 || c.IsSAFE_NAVIGATION() {
			return false, !opensScope(n)
		}
		r, ok := c.Receiver.(*parser.LocalVariableReadNode)
		if !ok {
			return false, true
		}
		v := f.visibleLocal(r.Name)
		if v == nil || v.owner() == nil || f.strHoists[v.owner()] != nil || !f.strInvariant(r.Name, v, pred, body) {
			return false, true
		}
		h := &strHoist{length: f.newTmp(), lim: f.newTmp(), frames: len(f.rbFrames)}
		f.emit("%s, %s := rbStrLenA(%s)", h.length, h.lim, f.genExpr(r, nil).code)
		f.emit("_, _ = %s, %s", h.length, h.lim) // a use the loop's own code may not make, as when `s[x]` takes a Range
		if f.strHoists == nil {
			f.strHoists = map[*localInfo]*strHoist{}
		}
		f.strHoists[v.owner()] = h
		hoisted = append(hoisted, v.owner())
		return false, true
	})
	return func() {
		for _, li := range hoisted {
			delete(f.strHoists, li)
		}
	}
}

// strInvariant: local name keeps one String through the loop; a block anywhere in the scope that assigns it may run inside the loop, and an out parameter's callee rebinds it (decision 405).
func (f *fctx) strInvariant(name string, v *local, pred parser.Node, body *parser.StatementsNode) bool {
	str := f.cls("String")
	if f.scopeRoot == nil || !typeEq(v.typ, str) || v.view != "" || v.base != nil || f.isOutParam(name) || writtenInBlock(f.scopeRoot, name, false) {
		return false
	}
	for _, m := range []string{"length", "size", "[]", "__idx_2"} {
		if e := f.c.classes["String"].lookup(m); e == nil || e.M.File == nil || !e.M.File.isPrelude() {
			return false
		}
	}
	changes := func(n parser.Node) (bool, bool) {
		if opensScope(n) {
			return false, false
		}
		if assignsLocalHere(n, name) {
			return true, false
		}
		switch c := n.(type) {
		case *parser.CallNode:
			if r, ok := c.Receiver.(*parser.LocalVariableReadNode); ok && r.Name == name && !readOnlyStrMethods[c.Name] {
				return true, false // `s << x`, `s.setbyte`, `s.sub!`: each rebinds the local (decisions 58, 382, 395)
			}
			for _, a := range callArgs(c) {
				if r, ok := a.(*parser.LocalVariableReadNode); ok && r.Name == name && f.mayAppendArg(c, a) {
					return true, false
				}
			}
		case *parser.SuperNode, *parser.ForwardingSuperNode:
			return anyNode(n, func(x parser.Node) bool { r, ok := x.(*parser.LocalVariableReadNode); return ok && r.Name == name }), true
		}
		return false, true
	}
	return !walkNode(pred, changes) && (body == nil || !walkNode(body, changes))
}

// mayAppendArg: c may append to its argument a (decision 405); a receiver outCallArg cannot probe yet (a local the loop declares is not in scope before it) is answered by whether any method of c's name has an out parameter.
func (f *fctx) mayAppendArg(c *parser.CallNode, a parser.Node) bool {
	switch r := c.Receiver.(type) {
	case nil, *parser.SelfNode, *parser.ConstantReadNode, *parser.ConstantPathNode:
		out, _ := f.outCallArg(c, a)
		return out
	case *parser.LocalVariableReadNode:
		if f.visibleLocal(r.Name) != nil {
			out, _ := f.outCallArg(c, a)
			return out
		}
	}
	hasOut := func(m *Method) bool {
		return m != nil && slices.ContainsFunc(m.Params, func(p Param) bool { return p.Out })
	}
	if hasOut(f.c.topDefs[c.Name]) {
		return true
	}
	for _, k := range f.c.classList {
		if hasOut(k.Methods[c.Name]) {
			return true
		}
	}
	return false
}

// isOutParam: name is a String parameter the method appends to through its caller's String (decision 405).
func (f *fctx) isOutParam(name string) bool {
	if f.m == nil {
		return false
	}
	for _, p := range f.m.Params {
		if p.Out && p.Name == name {
			return true
		}
	}
	return false
}

// hoistedStrCall answers `s.length`/`s.size`/`s[i]`/`s[i, n]` of a hoisted String from hoistStrLoop's values, not in a block, which may run after the loop.
func (f *fctx) hoistedStrCall(n *parser.CallNode) (expr, bool) {
	if e, ok := f.cachedIvarStrCall(n); ok {
		return e, true
	}
	r, ok := n.Receiver.(*parser.LocalVariableReadNode)
	if !ok || len(f.strHoists) == 0 || n.Block != nil || n.IsSAFE_NAVIGATION() {
		return expr{}, false
	}
	v := f.visibleLocal(r.Name)
	if v == nil || v.owner() == nil {
		return expr{}, false
	}
	h := f.strHoists[v.owner()]
	if h == nil || h.frames != len(f.rbFrames) {
		return expr{}, false
	}
	args := callArgs(n)
	intT := f.cls("Integer")
	if n.Name != "[]" && (n.Name != "length" && n.Name != "size" || len(args) != 0) || len(args) > 2 {
		return expr{}, false
	}
	for _, a := range args {
		if isSplat(a) || !typeEq(f.probeType(a, intT), intT) {
			return expr{}, false
		}
	}
	switch {
	case (n.Name == "length" || n.Name == "size") && len(args) == 0:
		return expr{code: h.length, typ: intT}, true
	case n.Name == "[]" && (len(args) == 1 || len(args) == 2):
		dst := f.newTmp() // this call's own String: declared here, so a loop's next pass, or a kept String?, never shares it
		f.emit("var %s String", dst)
		codes := []string{f.genExpr(r, nil).code}
		for _, a := range args {
			codes = append(codes, f.genExpr(a, intT).code)
		}
		fn := map[int]string{1: "rbStrIdxA", 2: "rbStrSubA"}[len(args)]
		return expr{code: fmt.Sprintf("%s(%s, %s, &%s)", fn, strings.Join(codes, ", "), h.lim, dst), typ: TOpt{Elem: f.cls("String")}}, true
	}
	return expr{}, false
}

// strCached: iv is a String ivar its own class reads `length`/`size`/`[]` of, in a struct class (not a value class,
// whose methods read a copy): it gets a cache field for its length scan (decision 454).
func (c *Compiler) strCached(iv *Ivar) bool {
	cls := iv.Owner
	if iv.scratch || cls == nil || cls.byValue || !cls.isStruct() || cls.universal || !isClass(iv.Type, "String") {
		return false
	}
	if c.strCacheIvars == nil {
		c.strCacheIvars = map[*Class]map[string]bool{}
	}
	names, ok := c.strCacheIvars[cls]
	if !ok {
		names = map[string]bool{}
		for _, m := range cls.MethodList {
			if m.Owner != cls || m.Node == nil || m.File == nil || m.File.isPrelude() || c.gemFile(m.File) {
				continue
			}
			walkNode(m.Node.Body, func(n parser.Node) (bool, bool) {
				if call, ok := n.(*parser.CallNode); ok && (call.Name == "length" || call.Name == "size" || call.Name == "[]") {
					if r, ok := call.Receiver.(*parser.InstanceVariableReadNode); ok {
						names[r.Name] = true
					}
				}
				return false, true
			})
		}
		c.strCacheIvars[cls] = names
	}
	return names[iv.Name]
}

// strCacheField is the cache field of a String ivar strCached keeps.
func strCacheField(iv *Ivar) string { return goFieldName(iv.Name) + "C_" }

// cachedIvarStrCall is `@s.length` and `@s[i]` / `@s[i, n]` of a strCached ivar: its length and ASCII bound come from
// the object's cache, not a scan per call (decision 454).
func (f *fctx) cachedIvarStrCall(n *parser.CallNode) (expr, bool) {
	r, ok := n.Receiver.(*parser.InstanceVariableReadNode)
	if !ok || f.lenient() || n.Block != nil || n.IsSAFE_NAVIGATION() || f.owner == nil || f.scope.lookup(ivarNarrowKey(r.Name)) != nil {
		return expr{}, false
	}
	iv := f.c.findIvar(f.owner, r.Name)
	if iv == nil || iv.Owner != f.owner || !f.c.strCached(iv) {
		return expr{}, false
	}
	args := callArgs(n)
	intT := f.cls("Integer")
	if n.Name != "[]" && (n.Name != "length" && n.Name != "size" || len(args) != 0) || n.Name == "[]" && (len(args) < 1 || len(args) > 2) {
		return expr{}, false
	}
	for _, a := range args {
		if isSplat(a) || !typeEq(f.probeType(a, intT), intT) {
			return expr{}, false
		}
	}
	s := f.ivarCode(iv)
	cache := "&" + ivarPath(f.selfCode, iv)[:len(ivarPath(f.selfCode, iv))-len(goFieldName(iv.Name))] + strCacheField(iv)
	if n.Name != "[]" {
		ln := f.newTmp()
		f.emit("%s, _ := rbStrCacheLenA(%s, %s)", ln, cache, s)
		return expr{code: ln, typ: intT}, true
	}
	lim, dst := f.newTmp(), f.newTmp()
	f.emit("_, %s := rbStrCacheLenA(%s, %s)", lim, cache, s)
	f.emit("var %s String", dst)
	codes := []string{s}
	for _, a := range args {
		codes = append(codes, f.genExpr(a, intT).code)
	}
	fn := map[int]string{1: "rbStrIdxA", 2: "rbStrSubA"}[len(args)]
	return expr{code: fmt.Sprintf("%s(%s, %s, &%s)", fn, strings.Join(codes, ", "), lim, dst), typ: TOpt{Elem: f.cls("String")}}, true
}
