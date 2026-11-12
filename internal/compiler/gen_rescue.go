package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// jumpKind is how a jump left a begin wrapper's func literal. Go's
// return/break/continue stop at the func literal, so the wrapper records
// the jump in its flag and the jump is re-issued after the call.
type jumpKind int

const (
	jumpReturn jumpKind = iota + 1
	jumpBreak
	jumpNext
)

type wrapFrame struct {
	flag    string
	used    [jumpNext + 1]bool
	endsRet bool // the call is followed by the function's return
}

// leaveWrapper jumps out of the innermost begin wrapper.
func (f *fctx) leaveWrapper(k jumpKind) {
	if k == jumpReturn && f.wrap.endsRet {
		f.emit("return")
		return
	}
	f.wrap.used[k] = true
	f.emit("%s = %d", f.wrap.flag, k)
	f.emit("return")
}

// emitReturn leaves the method once its value (if any) is in place.
func (f *fctx) emitReturn() {
	if f.begins > 0 {
		f.leaveWrapper(jumpReturn)
		return
	}
	f.emit("return")
}

// containsJump reports a return, break or next anywhere under n.
func containsJump(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x.(type) {
		case *parser.ReturnNode, *parser.BreakNode, *parser.NextNode:
			return true, false
		case *parser.DefNode:
			return false, false
		}
		return false, true
	})
}

func containsRescue(n parser.Node) bool { return rescues(n, true) }

// containsSuper reports whether a method body calls super (nested defs
// are other methods).
func containsSuper(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x.(type) {
		case *parser.SuperNode, *parser.ForwardingSuperNode:
			return true, false
		case *parser.DefNode:
			return false, false
		}
		return false, true
	})
}

// containsRescueClause is containsRescue without ensure-only begins.
func containsRescueClause(n parser.Node) bool { return rescues(n, false) }

// rescues finds a begin with a rescue clause (or, with ensure, an ensure), a rescue modifier or a rescuing loop, outside nested defs.
func rescues(n parser.Node, ensure bool) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch b := x.(type) {
		case *parser.BeginNode:
			if b.RescueClause != nil || ensure && b.EnsureClause != nil {
				return true, false
			}
		case *parser.RescueModifierNode:
			return true, false
		case *parser.CallNode:
			if isStopLoop(b) {
				return true, false // loopRescue makes it a begin/rescue
			}
		case *parser.DefNode:
			return false, false
		}
		return false, true
	})
}

func (f *fctx) genBegin(n *parser.BeginNode, t tail) {
	if n.RescueClause == nil && n.EnsureClause == nil {
		f.genStmts(n.Statements, t)
		return
	}
	// Where does the value go?
	inner := t
	if t.kind == tailReturn && t.typ != nil && !isVoid(t.typ) {
		inner = tail{kind: tailAssign, target: f.retVar, typ: t.typ}
	}
	if t.kind == tailAssign && t.typ == nil || t.kind == tailReturn && t.typ == nil {
		// probing: just collect types
		inner = t
	}
	if n.EnsureClause != nil && terminates(n.EnsureClause.Statements) {
		// an ensure ending in `return` decides the value; the body's and
		// rescue's values are discarded, as in Ruby
		inner = tail{}
	}
	retry := n.RescueClause != nil && hasRetry(n.RescueClause)
	if retry && n.EnsureClause != nil {
		// retry reruns the body, not ensure: ensure goes in an outer begin around the retried one
		f.genBegin(f.c.rewrite(n, func() parser.Node {
			in := &parser.BeginNode{Location: n.Location, Statements: n.Statements, RescueClause: n.RescueClause, ElseClause: n.ElseClause}
			return &parser.BeginNode{Location: n.Location, Statements: &parser.StatementsNode{Location: n.Location, Body: []parser.Node{in}}, EnsureClause: n.EnsureClause}
		}).(*parser.BeginNode), t)
		return
	}
	w := &wrapFrame{flag: f.newTmp(), endsRet: t.kind == tailReturn && t.typ != nil && f.begins == 0}
	retryFlag := ""
	if retry {
		retryFlag = f.newTmp()
		f.indent++
	}
	call := f.capture(func() {
		saved, savedDepth := f.retryFlag, f.retryDepth
		f.retryFlag, f.retryDepth = retryFlag, f.closures
		f.genWrapper(n, inner, w)
		f.retryFlag, f.retryDepth = saved, savedDepth
	})
	if slices.Contains(w.used[:], true) {
		f.emit("%s := 0", w.flag)
	}
	if retry {
		// `retry` leaves the rescue with the flag set; the begin runs again
		f.indent--
		f.emit("for {")
		f.emit("\t%s := false", retryFlag)
		f.buf.WriteString(call)
		f.emit("\tif !%s {", retryFlag)
		f.emit("\t\tbreak")
		f.emit("\t}")
		f.emit("}")
	} else {
		f.buf.WriteString(call)
	}
	for k, used := range w.used {
		if used {
			f.emit("if %s == %d {", w.flag, k)
			f.indent++
			f.reissue(n, jumpKind(k))
			f.indent--
			f.emit("}")
		}
	}
	if w.endsRet {
		f.emit("return") // the value is already in the named result
	}
}

// reissue repeats, after a begin wrapper's call, the jump that left it.
func (f *fctx) reissue(n parser.Node, k jumpKind) {
	switch k {
	case jumpReturn:
		f.emitReturn()
	case jumpBreak:
		f.emitBreak(f.loops[len(f.loops)-1])
	case jumpNext:
		f.emitNext(n, f.loops[len(f.loops)-1])
	}
}

// genWrapper emits a begin/rescue/ensure as a called func literal.
func (f *fctx) genWrapper(n *parser.BeginNode, inner tail, w *wrapFrame) {
	f.emit("rbBegin(func() {") // through rbBegin, so a backtrace knows this literal is no block (decision 106)
	saved := f.enterBlock()
	f.indent++
	f.begins++
	savedWrap := f.wrap
	f.wrap = w
	// Generate in Ruby's order (body, rescue, ensure) so locals assigned in
	// the body are known to rescue and ensure; emit in Go's order, since
	// the defers must be registered before the body runs.
	// With an else clause the body's value is dropped: else's (or a rescue's) is the result.
	bodyTail, ran := inner, ""
	if n.ElseClause != nil {
		bodyTail, ran = tail{}, f.newTmp()
		f.indent++ // body and rescues go one level down, in their own literal
	}
	body := f.capture(func() {
		// its own block, so the rescue/ensure closures (emitted before it)
		// see body locals as foreign and hoist them
		saved := f.enterBlock()
		f.genStmts(n.Statements, bodyTail)
		if ran != "" && !terminates(n.Statements) {
			f.emit("%s = true", ran)
		}
		f.leaveBlock(saved)
	})
	rescue := f.capture(func() {
		if n.RescueClause != nil {
			f.emit("defer func() {")
			saved := f.enterBlock()
			f.indent++
			f.emit("if r_ := recover(); r_ != nil {")
			f.indent++
			for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
				if rc.Reference != nil || readsErrorInfo(rc.Statements) { // `=> e` or $! may ask for its backtrace: record the frames now (decisions 106, 414)
					f.emit("r_ = rbCaptureBacktrace(r_)")
					break
				}
			}
			f.emit("r_ = rbWrapPanic(r_)")
			for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
				f.genRescueClause(rc, inner)
			}
			f.emit("panic(r_)")
			f.indent--
			f.emit("}")
			f.indent--
			f.leaveBlock(saved)
			f.emit("}()")
		}
	})
	if ran != "" {
		f.indent--
	}
	ensure := f.capture(func() {
		if n.EnsureClause != nil {
			f.emit("defer func() {")
			saved := f.enterBlock()
			f.indent++
			// A jump in ensure discards a pending exception, as in Ruby:
			// hold it, and re-raise only if the ensure body falls through.
			pending := ""
			switch {
			case terminates(n.EnsureClause.Statements):
				f.emit("_ = recover()") // it always jumps: the exception is dropped
			case containsJump(n.EnsureClause):
				pending = f.newTmp()
				f.emit("%s := recover()", pending)
			}
			f.genStmts(n.EnsureClause.Statements, tail{})
			if pending != "" {
				f.emit("if %s != nil {", pending)
				f.emit("\tpanic(%s)", pending)
				f.emit("}")
			}
			f.indent--
			f.leaveBlock(saved)
			f.emit("}()")
		}
	})
	f.buf.WriteString(ensure)
	if ran == "" {
		f.buf.WriteString(rescue)
		f.buf.WriteString(body)
	} else {
		// begin/else: the rescues guard the body only, so they live in an inner
		// literal; else runs after it when the body finished, under ensure
		f.emit("%s := false", ran)
		f.emit("rbBegin(func() {")
		f.buf.WriteString(rescue)
		f.buf.WriteString(body)
		f.emit("})")
		f.emit("if %s {", ran)
		saved := f.enterBlock()
		f.indent++
		f.genStmts(n.ElseClause.Statements, inner)
		f.indent--
		f.leaveBlock(saved)
		f.emit("}")
	}
	f.begins--
	f.wrap = savedWrap
	f.indent--
	f.leaveBlock(saved)
	f.emit("})")
}

// hasRetry reports a `retry` in rescue clauses rc (not in a nested begin's own rescues, a block or a def).
func hasRetry(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.RetryNode:
			return true, false
		case *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
			return false, false
		case *parser.BeginNode: // its own rescues own their retries; nil clauses are typed nils
			return x.Statements != nil && hasRetry(x.Statements) || x.ElseClause != nil && hasRetry(x.ElseClause) || x.EnsureClause != nil && hasRetry(x.EnsureClause), false
		}
		return false, true
	})
}

func (f *fctx) genRescueClause(rc *parser.RescueNode, t tail) {
	var classes []*Class
	if len(rc.Exceptions) == 0 {
		classes = append(classes, f.c.classes["StandardError"])
	}
	var conds []string
	for _, cls := range classes { // the bare rescue's StandardError
		conds = append(conds, fmt.Sprintf("rbIsA[%s](r_)", f.c.goType(TClass{C: cls})))
	}
	for _, ex := range rc.Exceptions {
		if sp, ok := ex.(*parser.SplatNode); ok { // `rescue *ERRORS`: class objects checked at run time
			e := f.genExpr(sp.Expression, nil)
			if !isClass(e.typ, "Array") {
				f.errorf(ex, "rescue *%s needs an Array of exception classes", f.f.text(sp.Expression.GetLocation()))
			}
			conds = append(conds, fmt.Sprintf("slices.ContainsFunc(%s.s, func(k %s) bool { return rbIsInstanceOf(k, r_) })", f.materialize(e), f.c.goType(e.typ.(TClass).Args[0])))
			classes = append(classes, f.c.classes["Exception"])
			continue
		}
		cls := f.classRef(ex)
		if _, k := f.c.lookupConst(f.f, ex, f.lex); cls == nil && k == nil && f.lenient() && isConstNode(ex) {
			// a gem's `rescue ::Bundler::GemfileNotFound` of a library the program never loads (rubocop's Lockfile): MRI
			// reads the constant only when an exception reaches the clause, and raises NameError there (decision 444)
			f.c.warn(f.f, ex, "uninitialized constant %s: raises NameError if an exception reaches this rescue", f.f.text(ex.GetLocation()))
			conds = append(conds, fmt.Sprintf("rbRescueMissing(%q)", f.f.text(ex.GetLocation())))
			classes = append(classes, f.c.classes["Exception"])
			continue
		}
		if cls == nil {
			f.errorf(ex, "rescue needs exception class names")
		}
		classes = append(classes, cls)
		conds = append(conds, fmt.Sprintf("rbIsA[%s](r_)", f.c.goType(TClass{C: cls})))
	}
	f.emit("if %s {", strings.Join(conds, " || "))
	saved := f.enterBlock()
	f.indent++
	if rc.Reference != nil {
		lt, ok := rc.Reference.(*parser.LocalVariableTargetNode)
		if !ok {
			f.errorf(rc.Reference, "unsupported rescue target")
		}
		bind := classes[0]
		if len(classes) > 1 {
			bind = f.c.classes["Exception"]
		}
		f.bindRescue(rc.Reference, lt.Name, TClass{C: bind})
	}
	if f.c.gemBareRaise() {
		f.emit("defer rbErrinfoSet(r_)()") // $! for a bare raise this clause reaches through a call (decision 331)
	}
	f.rescues++
	f.genStmts(rc.Statements, t)
	f.rescues--
	if !terminates(rc.Statements) {
		f.emit("return")
	}
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
}

// bindRescue binds `rescue => name`. The name is a local of the enclosing
// method or block, nil when nothing was rescued: when it is read outside the
// clause, the clause assigns it and narrows it to non-nil; otherwise each
// clause gets a fresh Go binding of its own class. A local of that name
// assigned before keeps its one type, so the clause shadows it.
func (f *fctx) bindRescue(n parser.Node, name string, bind Type) {
	val := expr{code: "r_.(" + f.c.goType(bind) + ")", typ: bind}
	if outer := f.visibleLocal(name); outer != nil {
		v := &local{name: name, goName: goLocalName(name), typ: bind, declared: true, info: outer.owner()}
		f.scope.vars[name] = v
		f.emit("%s := %s", v.goName, val.code)
		f.emit("_ = %s", v.goName) // the reads counted are the outer local's too
		return
	}
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{typ: TOpt{Elem: bind}}
		f.locals[key] = info
	}
	if info.declPass != f.pass {
		info.declPass, info.declBlock, info.declRuby = f.pass, f.block, f.rbScope
	}
	if f.pass < 2 && !info.annotated {
		if j, ok := join(info.typ, TOpt{Elem: bind}); ok {
			info.typ = j
		}
	}
	if f.pass < 2 || !info.hoist {
		v := &local{name: name, goName: goLocalName(name), typ: bind, declared: true, info: info}
		f.scope.vars[name] = v
		f.emit("%s := %s", v.goName, val.code)
		f.noteUnused(v)
		return
	}
	outer := &local{name: name, goName: goLocalName(name), typ: info.typ, declared: true, info: info}
	f.emit("%s = %s", outer.goName, f.coerce(n, val, outer.typ))
	if isOpt(outer.typ) {
		f.applyNarrow([]narrowInfo{{local: outer, typ: stripOpt(outer.typ)}})
	}
}

// terminates reports whether a statement list ends in a jump, so no Go
// `return` may follow it (vet flags unreachable code).
func terminates(st *parser.StatementsNode) bool {
	if st == nil || len(st.Body) == 0 {
		return false
	}
	switch last := st.Body[len(st.Body)-1].(type) {
	case *parser.ReturnNode, *parser.BreakNode, *parser.NextNode, *parser.RetryNode, *parser.RedoNode:
		return true
	case *parser.CallNode:
		return kernelRecv(last) && (last.Name == "raise" || last.Name == "fail" || last.Name == "throw")
	}
	return false
}

// readsErrorInfo reports n reading the handled exception through $! or $@ (English's $ERROR_INFO, $ERROR_POSITION).
func readsErrorInfo(n *parser.StatementsNode) bool {
	if n == nil { // an empty rescue body
		return false
	}
	return anyNode(n, func(x parser.Node) bool {
		g, ok := x.(*parser.GlobalVariableReadNode)
		return ok && (g.Name == "$!" || g.Name == "$@" || g.Name == "$ERROR_INFO" || g.Name == "$ERROR_POSITION")
	})
}
