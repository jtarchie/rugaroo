package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func (f *fctx) genIf(n parser.Node, pred parser.Node, then parser.Node, els parser.Node, negate bool, t tail) {
	isA, folded := f.foldedIsA(pred)
	folded = folded && isA == negate                                          // only a branch that cannot run is dropped: one that always runs keeps its if, so the code after it stays reachable for go vet
	if f.c.absentGuard(f.f, pred, f.lex) || f.cannotRespond(pred) || folded { // the guarded branch names what no file defines: it cannot run (decision 167)
		taken := els
		if negate && !folded {
			taken = then
		}
		switch e := taken.(type) {
		case *parser.ElseNode:
			f.genStmts(e.Statements, t)
		case *parser.IfNode:
			f.genIf(e, e.Predicate, e.Statements, e.Subsequent, false, t)
		default:
			f.genStmts(taken, t)
		}
		return
	}
	elseNarrow := f.elseNarrowing(pred)
	cond, narrow := f.genCond(pred)
	f.applyNarrow(f.callNarrowing(pred)) // the condition ran before either branch: a local it called a method on is not nil (decision 357)
	if negate {
		if inner, ok := strings.CutPrefix(cond, "!"); ok && strings.HasSuffix(inner, ".ok") && !strings.ContainsAny(inner, " &|!") {
			cond = inner // `unless x.nil?` of an rbOpt: staticcheck's SA4013 rejects `!(!x.ok)` (decision 396)
		} else {
			cond = "!(" + cond + ")"
		}
		narrow, elseNarrow = elseNarrow, narrow
	}
	f.emit("if %s {", cond)
	saved := f.enterBlock()
	f.indent++
	f.applyNarrow(narrow)
	f.c.markSureTake(pred, !negate, then)
	f.genBranch(then, t)
	f.indent--
	f.leaveBranch(saved, then)
	switch e := els.(type) {
	case nil:
		if t.kind != tailNone {
			f.emit("} else {")
			saved := f.enterBlock()
			f.indent++
			f.emptyTail(n, t)
			f.indent--
			f.leaveBlock(saved)
		}
		f.emit("}")
	case *parser.ElseNode:
		f.emit("} else {")
		saved := f.enterBlock()
		f.indent++
		f.applyNarrow(elseNarrow)
		f.c.markSureTake(pred, negate, e.Statements)
		f.genBranch(e.Statements, t)
		f.indent--
		f.leaveBranch(saved, e.Statements)
		f.emit("}")
	case *parser.IfNode:
		saved := f.enterBlock()
		f.indent++
		f.applyNarrow(elseNarrow)
		body := f.capture(func() { f.genIf(e, e.Predicate, e.Statements, e.Subsequent, false, t) })
		f.indent--
		f.leaveBlock(saved)
		if chain, ok := elseIfChain(body, f.indent+1); ok && e.Subsequent == nil { // gocritic's elseif; a longer chain would be its ifElseChain
			f.emit("} else %s", chain)
			return
		}
		f.emit("} else {")
		f.buf.WriteString(body)
		f.emit("}")
	default:
		f.c.unsupported(f.f, els)
	}
}

// leaveBranch: a branch that re-narrows a local it reassigned (`t = y; break if t.nil?`) leaves it non-nil on every path past it (decision 357).
func (f *fctx) leaveBranch(saved string, body parser.Node) {
	sc := f.scope
	st, _ := body.(*parser.StatementsNode)
	if len(sc.killed) == 0 || st == nil {
		f.leaveBlock(saved)
		return
	}
	ends := terminates(st)
	jumps := anyNode(st, func(x parser.Node) bool {
		switch x.(type) {
		case *parser.BreakNode, *parser.NextNode, *parser.RedoNode, *parser.RetryNode:
			return true
		}
		return false
	})
	var loop *scope // a break or next reaches code after the loop, where views outside it must stay dropped
	if len(f.loops) > 0 {
		loop = f.loops[len(f.loops)-1].scope
	}
	if len(st.Body) > 0 && !ends { // genStmts narrows after every statement but the last
		last := st.Body[len(st.Body)-1]
		f.applyNarrow(f.guardNarrowing(last))
		f.applyNarrow(f.callNarrowing(last))
	}
	kept := sc.killed[:0]
	for _, k := range sc.killed {
		now := sc.vars[k.name]
		again := ends || now != nil && now.base != nil && now.base.goName == k.view.base.goName && typeEq(now.typ, k.view.typ)
		if again && k.sc.vars[k.name] == k.repl && (!jumps || loop == nil || insideScope(k.sc, loop)) {
			k.sc.vars[k.name] = k.view
			continue
		}
		kept = append(kept, k)
	}
	sc.killed = kept
	f.leaveBlock(saved)
}

// insideScope reports whether sc lies strictly inside outer.
func insideScope(sc, outer *scope) bool {
	for p := sc; p != nil; p = p.parent {
		if p.parent == outer {
			return true
		}
	}
	return false
}

// callNarrowing: a T? local that survived a nil-raising call n always runs is non-nil after it, since MRI raised otherwise (decision 357).
func (f *fctx) callNarrowing(n parser.Node) []narrowInfo {
	var ns []narrowInfo
	seen := map[string]bool{}
	var body parser.Node
	if f.m != nil && f.m.Node != nil {
		body = f.m.Node.Body
	} else {
		body = f.bodyStmts
	}
	var walk func(x parser.Node)
	walk = func(x parser.Node) {
		switch x := x.(type) {
		case *parser.CallNode:
			walk(x.Receiver)
			if x.Arguments != nil {
				for _, a := range x.Arguments.Arguments {
					walk(a)
				}
			}
			lv, ok := x.Receiver.(*parser.LocalVariableReadNode)
			if !ok || !f.nilGuarded[x] || seen[lv.Name] {
				return
			}
			seen[lv.Name] = true
			v := f.narrowLocal(lv.Name)
			if v == nil || v.base != nil || !isOpt(v.typ) || isAny(stripOpt(v.typ)) || assignsLocal(n, lv.Name) || body != nil && blockWritesLocal(body, lv.Name, f.isParam(lv.Name)) {
				return
			}
			ns = append(ns, narrowInfo{local: v, typ: stripOpt(v.typ)})
		case *parser.AndNode:
			walk(x.Left)
		case *parser.OrNode:
			walk(x.Left)
		case *parser.ParenthesesNode:
			walk(x.Body)
		case *parser.StatementsNode:
			for _, s := range x.Body {
				walk(s)
			}
		case *parser.IfNode:
			walk(x.Predicate)
		case *parser.UnlessNode:
			walk(x.Predicate)
		case *parser.LocalVariableWriteNode:
			walk(x.Value)
		case *parser.InstanceVariableWriteNode:
			walk(x.Value)
		case *parser.ReturnNode:
			if x.Arguments != nil {
				for _, a := range x.Arguments.Arguments {
					walk(a)
				}
			}
		}
	}
	walk(n)
	return ns
}

// assignsLocal reports whether n assigns local name in any form.
func assignsLocal(n parser.Node, name string) bool {
	return anyNode(n, func(x parser.Node) bool {
		switch x := x.(type) {
		case *parser.LocalVariableWriteNode:
			return x.Name == name
		case *parser.LocalVariableOperatorWriteNode:
			return x.Name == name
		case *parser.LocalVariableOrWriteNode:
			return x.Name == name
		case *parser.LocalVariableAndWriteNode:
			return x.Name == name
		case *parser.LocalVariableTargetNode:
			return x.Name == name
		}
		return false
	})
}

// Narrowed ivars/attr readers share scope.vars with locals under ":" keys no identifier can spell.
func ivarNarrowKey(ivar string) string { return "ivar:" + ivar } // ivar keeps its "@"
func attrNarrowKey(name string) string { return "attr:" + name } // name has no "@" or "="
func isIvarNarrowKey(key string) bool  { return strings.HasPrefix(key, "ivar:") }

type narrowInfo struct {
	local    *local
	typ      Type
	code     string // Go expression for the narrowed view; "" means deref
	view     string // see expr.view
	assigned bool   // see local.assigned
}

// applyNarrow shadows narrowed locals in the current scope.
func (f *fctx) applyNarrow(ns []narrowInfo) {
	for _, nw := range ns {
		base := nw.local
		if base.base != nil {
			base = base.base
		}
		code := nw.code
		if code == "" {
			code = derefOpt(nw.local.goName, nw.local.typ)
		}
		f.scope.vars[nw.local.name] = &local{name: nw.local.name, goName: code, typ: nw.typ, base: base, view: nw.view, declared: true, assigned: nw.assigned}
	}
}

// unnarrow forgets every narrowed view of a local after it is reassigned:
// the new value may be nil or another class again.
func (f *fctx) unnarrow(name string) {
	info := f.localInfo(name)
	hoisted := info != nil && !info.noHoist && info.typ != nil && info.declPass == f.pass && isAncestorBlock(info.declRuby, f.rbScope)
	for sc := f.scope; sc != nil; sc = sc.parent {
		if v := sc.vars[name]; v != nil && v.base != nil {
			delete(sc.vars, name)
			if !hoisted && (sc.parent == nil || sc.parent.lookup(name) == nil) { // the view replaced a parameter here (`param ||= x`): it stays
				sc.vars[name] = v.base
			}
			if sc != f.scope {
				f.scope.killed = append(f.scope.killed, killedView{sc: sc, name: name, view: v, repl: sc.vars[name]})
			}
		}
	}
}

// guardNarrowing is what an early-exit guard proves about the statements
// after it: `return x unless cond` narrows like `if cond`, and
// `return if x.nil?` narrows x to non-nil.
func (f *fctx) guardNarrowing(s parser.Node) []narrowInfo {
	var ns []narrowInfo
	switch s := s.(type) {
	case *parser.UnlessNode:
		if s.ElseClause == nil && terminates(s.Statements) {
			f.probe(func() { _, ns = f.genCond(s.Predicate) })
		}
	case *parser.IfNode:
		if s.Subsequent != nil || !terminates(s.Statements) {
			return nil
		}
		call, ok := s.Predicate.(*parser.CallNode)
		if !ok || call.Arguments != nil || (call.Name != "nil?" && call.Name != "!") {
			return f.elseNarrowing(s.Predicate) // `return x.size if x.is_a?(String)`: the rest has the other members
		}
		if lv, ok := call.Receiver.(*parser.LocalVariableReadNode); ok {
			if v := f.narrowLocal(lv.Name); v != nil && isOpt(v.typ) {
				ns = []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
			}
		}
		if w := parenWrite(call.Receiver); w != nil && call.Name == "nil?" { // `return if (data = build).nil?` (decision 249)
			if v := f.narrowLocal(w.Name); v != nil && isOpt(v.typ) {
				ns = []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
			}
		}
		if iv, ok := call.Receiver.(*parser.InstanceVariableReadNode); ok && f.owner != nil {
			if v := f.ivarLocal(iv); v != nil && v.base == nil && isOpt(v.typ) && !isAny(stripOpt(v.typ)) {
				ns = []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
			}
		}
		if ns == nil {
			return f.elseNarrowing(s.Predicate) // a union: `return if x.nil?` drops nil, `return if !x.is_a?(C)` keeps the C members
		}
	}
	return ns
}

// narrowUnion narrows a union-typed local to the members keep holds
// (decision 150). The narrowed view stays the local's any while it is a
// union, asserts the Go type when one class is left, and opens a T? box
// when one class and nil are. False when nothing is ruled out, or nothing
// (or only nil) is left.
func (f *fctx) narrowUnion(v *local, keep func(Type) bool) (narrowInfo, bool) {
	if v == nil {
		return narrowInfo{}, false
	}
	u, ok := v.typ.(TUnion)
	if !ok {
		return narrowInfo{}, false
	}
	var rest []Type
	for _, m := range u.Members {
		if keep(m) {
			rest = append(rest, m)
		}
	}
	if len(rest) == len(u.Members) || len(rest) == 0 {
		return narrowInfo{}, false
	}
	to := unionOf(rest...)
	switch to := to.(type) {
	case TUnion:
		return narrowInfo{local: v, typ: to, code: v.goName}, true
	case TNil:
		return narrowInfo{}, false
	case TOpt:
		return narrowInfo{local: v, typ: to, code: f.c.optOf(to.Elem, v.goName)}, true
	case TAny, TClass, TFunc, TTuple, TVar, TVoid:
	}
	return narrowInfo{local: v, typ: to, code: v.goName + ".(" + f.c.goType(to) + ")"}, true
}

// elseNarrowing is what a condition's being false proves about union-typed
// locals (decision 150): `x.is_a?(C)` false drops the members that are a C,
// `x.nil?` false drops nil, a falsy `x` leaves nil (and false), `!c` false
// is c true, and `a || b` false is both false.
func (f *fctx) elseNarrowing(n parser.Node) []narrowInfo {
	n = f.classEqqAsIsA(n)
	localOf := func(r parser.Node) *local {
		if lv, ok := r.(*parser.LocalVariableReadNode); ok {
			return f.narrowLocal(lv.Name) // also one assigned in an earlier branch or loop body, as readLocal finds it
		}
		if r == nil {
			return nil
		}
		return f.attrLocal(r)
	}
	one := func(v *local, keep func(Type) bool) []narrowInfo {
		if v == nil {
			return nil
		}
		if nw, ok := f.narrowUnion(v, keep); ok {
			return []narrowInfo{nw}
		}
		return nil
	}
	switch n := n.(type) {
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.elseNarrowing(st.Body[0])
		}
	case *parser.OrNode:
		left := f.elseNarrowing(n.Left)
		f.push()
		f.applyNarrow(left)
		right := f.elseNarrowing(n.Right) // from the types the left side's falsity left
		f.pop()
		return append(left, right...)
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode:
		return one(localOf(n), func(m Type) bool { return isNil(m) || isClass(m, "Boolean") })
	case *parser.CallNode:
		switch {
		case n.Name == "!" && n.Arguments == nil && n.Receiver != nil:
			var nw []narrowInfo
			f.probe(func() { _, nw = f.genCond(n.Receiver) })
			return nw
		case n.Name == "nil?" && n.Arguments == nil:
			if v := localOf(n.Receiver); v != nil && isOpt(v.typ) && v.base == nil && !isAny(stripOpt(v.typ)) { // `return c unless c.nil?`
				return []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
			}
			return one(localOf(n.Receiver), func(m Type) bool { return !isNil(m) })
		case isIsA(n) && n.Name != "instance_of?":
			cls := f.classRef(n.Arguments.Arguments[0])
			if cls == nil {
				return nil
			}
			return one(localOf(n.Receiver), func(m Type) bool { mc := classOf(m); return mc == nil || !mc.isSubclassOf(cls) })
		}
	}
	return nil
}

// classEqqAsIsA reads `Array === o` on a local as `o.is_a?(Array)`, the same test for a class, which narrows o for
// the branch (decision 244).
func (f *fctx) classEqqAsIsA(n parser.Node) parser.Node {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "===" || call.Receiver == nil || len(callArgs(call)) != 1 || f.classRef(call.Receiver) == nil {
		return n
	}
	if _, lv := callArgs(call)[0].(*parser.LocalVariableReadNode); !lv {
		return n
	}
	return &parser.CallNode{Location: call.Location, Receiver: callArgs(call)[0], Name: "is_a?",
		Arguments: &parser.ArgumentsNode{Location: call.Location, Arguments: []parser.Node{call.Receiver}}}
}

// genCond renders a Ruby truthiness test as a Go bool expression.
// foldedIsA decides `x.is_a?(C)` on a variable from its static type: the branch it rules out is not compiled, so its impossible values never reach inference (decision 404).
func (f *fctx) foldedIsA(pred parser.Node) (value, ok bool) {
	call, isCall := pred.(*parser.CallNode)
	if !isCall || !isIsA(call) {
		return false, false
	}
	switch call.Receiver.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode:
	default:
		return false, false
	}
	cls := f.classRef(call.Arguments.Arguments[0])
	if cls == nil || cls.IsModule || cls.universal || slices.Contains([]string{"TrueClass", "FalseClass", "NilClass"}, cls.RubyName) {
		return false, false
	}
	t, isClass := f.probeType(call.Receiver, nil).(TClass)
	if !isClass || isAbstract(t) || t.C.RubyName == "Bignum" || t.C.IsModule {
		return false, false
	}
	switch {
	case t.C.isSubclassOf(cls):
		return true, true
	case cls.isSubclassOf(t.C):
		return false, false
	}
	return false, true
}

func (f *fctx) genCond(n parser.Node) (string, []narrowInfo) {
	n = f.classEqqAsIsA(n)
	switch n := n.(type) {
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			c, nw := f.genCond(st.Body[0])
			return "(" + c + ")", nw
		}
	case *parser.AndNode:
		l, nl := f.genCond(n.Left)
		var r string
		var nr []narrowInfo
		f.push()
		f.applyNarrow(nl)
		f.applyNarrow(f.callNarrowing(n.Left)) // the left ran first: a local it called a guarded method on is not nil
		stmts := f.capture(func() { r, nr = f.genCond(n.Right) })
		f.pop()
		if stmts == "" {
			return l + " && " + r, append(nl, nr...)
		}
		// the right side needs statements: run them only if the left holds. A local it assigns
		// lives past the Go if (the branch reads it), so it is declared at the function's top
		for _, w := range condWrites(n) {
			if info := f.localInfo(w); info != nil {
				info.hoist = true
			}
		}
		tmp := f.newTmp()
		f.emit("%s := false", tmp)
		f.emit("if %s {", l)
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, r)
		f.emit("}")
		return tmp, append(nl, nr...) // tmp holds only when both sides did
	case *parser.OrNode:
		lFalse := f.elseNarrowing(n.Left)
		l, _ := f.genCond(n.Left)
		var r string
		f.push()
		f.applyNarrow(lFalse) // `h.nil? || h.zero?`: the right runs only when the left was false
		f.applyNarrow(f.callNarrowing(n.Left))
		stmts := f.capture(func() { r, _ = f.genCond(n.Right) })
		f.pop()
		if stmts == "" {
			return l + " || " + r, nil
		}
		for _, w := range condWrites(n) { // as for &&: `next if k.nil? || (blk = h[k]).nil?` then reads blk
			if info := f.localInfo(w); info != nil {
				info.hoist = true
			}
		}
		tmp := f.newTmp()
		f.emit("%s := true", tmp)
		f.emit("if %s {", notCond(l))
		f.buf.WriteString(stmts)
		f.emit("\t%s = %s", tmp, r)
		f.emit("}")
		return tmp, nil
	case *parser.CallNode:
		if c, nw, ok := f.genCondCall(n); ok {
			return c, nw
		}
	case *parser.LocalVariableReadNode:
		v := f.readLocal(n)
		if isOpt(v.typ) && !isAny(v.typ.(TOpt).Elem) {
			return optTruthy(v.goName, v.typ), []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
		}
		if nw, ok := f.narrowUnion(v, func(m Type) bool { return !isNil(m) }); ok {
			return f.truthy(n, expr{code: v.goName, typ: v.typ}), []narrowInfo{nw}
		}
	case *parser.MatchPredicateNode:
		return f.condMatchPredicate(n)
	case *parser.LocalVariableWriteNode:
		// `if (x = h[k])` / `while (job = q.pop)`: assign, then test and narrow x like a read
		f.genStmt(n, tail{})
		return f.genCond(&parser.LocalVariableReadNode{Name: n.Name, Depth: n.Depth, Location: n.Location})
	}
	if call, ok := n.(*parser.CallNode); ok && isIsA(call) {
		if lv, ok := call.Receiver.(*parser.LocalVariableReadNode); ok {
			return f.narrowIsA(call, f.readLocal(lv))
		}
		if v := f.attrLocal(call.Receiver); v != nil {
			return f.narrowIsA(call, v)
		}
	}
	if v := f.attrLocal(n); v != nil && v.base == nil && isOpt(v.typ) && !isAny(v.typ.(TOpt).Elem) {
		return optTruthy(v.goName, v.typ), []narrowInfo{{local: v, typ: v.typ.(TOpt).Elem}}
	}
	if v := f.attrLocal(n); v != nil {
		if nw, ok := f.narrowUnion(v, func(m Type) bool { return !isNil(m) }); ok {
			return f.truthy(n, expr{code: v.goName, typ: v.typ}), []narrowInfo{nw}
		}
	}
	e := f.genExpr(n, nil)
	return f.truthy(n, e), nil
}

// condWrites names the locals a condition assigns at its own level (`(w = x) && ...`), not in a block.
func condWrites(n parser.Node) []string {
	var out []string
	walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.BlockNode, *parser.LambdaNode:
			return false, false
		case *parser.LocalVariableWriteNode:
			out = append(out, x.Name)
		}
		return false, true
	})
	return out
}

// genCondCall is genCond for the calls it reads directly: block_given?
// (narrowing an optional block), !x and x.nil?.
func (f *fctx) genCondCall(n *parser.CallNode) (string, []narrowInfo, bool) {
	if name := f.m.optionalBlockLocal(); name != "" && n.Name == "block_given?" && kernelRecv(n) && n.Arguments == nil {
		c, nw := f.genCond(&parser.LocalVariableReadNode{Name: name, Location: n.Location}) // narrows the block to present
		return c, nw, true
	}
	if n.Name == "!" && n.Arguments == nil && n.Receiver != nil {
		nw := f.elseNarrowing(n.Receiver)
		c, _ := f.genCond(n.Receiver)
		if inner, ok := strings.CutPrefix(c, "!"); ok && strings.HasSuffix(inner, ".ok") && !strings.ContainsAny(inner, " &|!") {
			return inner, nw, true // `!x.nil?` of an rbOpt: staticcheck's SA4013 rejects `!(!x.ok)` (decision 396)
		}
		return "!(" + c + ")", nw, true
	}
	if n.Name == "nil?" && n.Arguments == nil && n.Receiver != nil {
		// typed first: a receiver this does not read is generated again by the call, so emitting it here would run it twice
		switch t := f.probeType(n.Receiver, nil); {
		case isOpt(t) || isAny(t):
			e := f.genExpr(n.Receiver, nil)
			return optIsNil(e.code, e.typ), nil, true
		case f.lenientObject(t):
			return "any(" + f.genExpr(n.Receiver, nil).code + ") == nil", nil, true
		}
	}
	return "", nil, false
}

// optTruthy tests a T? value: non-nil, and not false for a Boolean?.
func optTruthy(code string, t Type) string {
	if isClass(stripOpt(t), "Boolean") {
		return "rbTruthyOpt(" + code + ")"
	}
	return optNotNil(code, t)
}

func (f *fctx) truthy(n parser.Node, e expr) string {
	switch {
	case isClass(e.typ, "Boolean"):
		// genCond's other results (rbIsA, `!= nil`, temps) are Go bool; mixing in Boolean fails go build
		return "bool(" + e.code + ")"
	case isOpt(e.typ):
		return optTruthy(e.code, e.typ)
	case isNil(e.typ) || isVoid(e.typ):
		if !e.lit && e.code != "" && e.code != "nil" { // `puts "x" if log_it` with log_it typed nil: the call still runs (decision 334)
			f.emitExprStmt(n, e)
		}
		return "false"
	case isAny(e.typ), unionFalsy(e.typ):
		return "rbTruthy(" + e.code + ")"
	}
	if iv, ok := n.(*parser.InstanceVariableReadNode); ok && f.scope.lookup(ivarNarrowKey(iv.Name)) != nil {
		return "true" // narrowed (decision 148): the check is the program's, not a mistake
	}
	if lv, ok := n.(*parser.LocalVariableReadNode); ok && lv.Name == lastLineLocal {
		return "true" // `~ /re/`'s own check of $_ (decision 446), not the program's
	}
	if f.lenient() {
		// a gem's object may hold nil its type does not say (Homebrew's `if (inheritance = @sandbox_inheritance)`, an IO
		// attr_writer's ivar never set): tested at run time, where Go can compare it to nil (decision 257)
		if c, ok := e.typ.(TClass); ok && (c.C.isStruct() && c.C.metaOf == nil && !c.C.universal && !c.C.byValue || strings.HasPrefix(f.c.goType(e.typ), "*")) {
			return "(" + e.code + ") != nil"
		}
	}
	f.warn(n, "condition of type %s is always true", e.typ)
	// value types (Integer, String) can't be compared to nil; keep the evaluation and the local's use
	f.emit("_ = %s", e.code)
	return "true"
}

// notCode is Go's negation of the bool expression c: `!(x)` for x, and x for `!(x)`, since staticcheck's SA4013 rejects
// `!(!(x))` (an until's, or `while !done`'s, break test).
func notCode(c string) string {
	if inner, ok := strings.CutPrefix(c, "!("); ok && strings.HasSuffix(inner, ")") && balancedParens(inner[:len(inner)-1]) {
		return inner[:len(inner)-1]
	}
	return "!(" + c + ")"
}

// balancedParens reports whether s never closes more parentheses than it opened, and closes them all (string
// literals aside: a generated condition's quotes hold Ruby names, not parentheses that would unbalance it).
func balancedParens(s string) bool {
	depth := 0
	inStr := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case inStr && c == '\\':
			i++
		case c == '"':
			inStr = !inStr
		case inStr:
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && !inStr
}

// elseIfChain is body, an elsif's code at indent, dedented one level when it is only an if chain; ok is false when other statements come with it.
func elseIfChain(body string, indent int) (string, bool) {
	tabs := strings.Repeat("\t", indent)
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if !strings.HasPrefix(lines[0], tabs+"if ") {
		return "", false
	}
	for i, ln := range lines {
		if strings.HasPrefix(ln, "\t") {
			lines[i] = ln[1:]
		}
		if i > 0 && strings.HasPrefix(ln, tabs) && !strings.HasPrefix(ln, tabs+"\t") && (!strings.HasPrefix(ln, tabs+"}") || strings.HasPrefix(ln, tabs+"} else")) {
			return "", false
		}
	}
	return strings.TrimLeft(strings.Join(lines, "\n"), "\t"), true
}

// notCond negates a Go condition, unwrapping one that is already `!(...)` (staticcheck SA4013).
func notCond(c string) string {
	if strings.HasPrefix(c, "!(") && strings.HasSuffix(c, ")") && !strings.ContainsAny(c, "\"`'") { // a literal could hold a paren
		depth := 0
		for i, r := range c[1:] {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				if i == len(c)-2 {
					return c[2 : len(c)-1]
				}
				break
			}
		}
	}
	return "!(" + c + ")"
}

// narrowLocal: a write in a branch drops a hoisted local's view from scope.vars, and a guard after it must still find it (decision 357).
func (f *fctx) narrowLocal(name string) *local {
	if v := f.scope.lookup(name); v != nil {
		return v
	}
	return f.sameScopeLocal(name)
}
