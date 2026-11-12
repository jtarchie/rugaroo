package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// breakCatch is a call statement whose closure block has a `break`: the block panics rbBreak with id, and the
// statement, wrapped in a function, recovers it (decision 241).
type breakCatch struct {
	block *parser.BlockNode
	id    string
	used  bool
}

type loopKind int

const (
	loopFor loopKind = iota
	loopClosure
	loopIter
)

// loopFrame lets a `break` inside a Go switch (case/when) exit the loop via a label, not just the switch.
type loopFrame struct {
	kind     loopKind
	switches int          // f.switches when the loop was entered
	begins   int          // f.begins when the loop was entered
	closures int          // f.closures when the loop was entered
	start    int          // where the label goes
	label    string       // set by the first `break` that needs it
	redo     string       // the label opening the body, when it has a `redo`
	next     string       // a do-while's label before its condition, when the body has a `next`
	scope    *scope       // the scope around the loop: a jump out of a branch skips only what lies inside the loop (leaveBranch)
	value    *tail        // a while used as a value: where `break v` puts v (decision 364)
	broke    *brokeTarget // an iterator used as a value: a valueless break jumps past its "ran to the end" value (decision 417)
}

// brokeTarget is the label after an iterator call's loop that its valueless break jumps to, set used by the first one.
type brokeTarget struct {
	label string
	used  bool
	res   string // the loop's value: a `break value` assigns it before the jump (decision 444)
}

// pushLoop goes just before the `for`; a label sits above its //line directive so the `for` keeps its Ruby line.
func (f *fctx) pushLoop(kind loopKind) {
	s := f.buf.String()
	start := len(s)
	if i := strings.LastIndexByte(strings.TrimSuffix(s, "\n"), '\n') + 1; strings.HasPrefix(s[i:], "//line ") {
		start = i
	}
	f.loops = append(f.loops, &loopFrame{kind: kind, switches: f.switches, begins: f.begins, closures: f.closures, start: start, scope: f.scope})
}

// redoLabel opens a loop body that contains `redo` with a label for its goto.
func (f *fctx) redoLabel(body parser.Node) {
	if body == nil || !hasRedo(body) {
		return
	}
	l := f.loops[len(f.loops)-1]
	l.redo = "redo" + f.newTmp()
	f.emit("%s:", l.redo)
}

// hasRedo finds a `redo` for the enclosing loop (not one in a nested loop, block or def).
func hasRedo(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.RedoNode:
			return true, false
		case *parser.StatementsNode:
			return false, x != nil
		case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
			return false, false
		}
		return false, true
	})
}

// popLoop inserts the label only once a break used it: Go rejects unused labels.
func (f *fctx) popLoop() {
	l := f.loops[len(f.loops)-1]
	f.loops = f.loops[:len(f.loops)-1]
	if l.label == "" {
		return
	}
	s := f.buf.String()
	f.buf.Reset()
	f.buf.WriteString(s[:l.start] + l.label + ":\n" + s[l.start:])
}

func (f *fctx) genWhile(pred parser.Node, body *parser.StatementsNode, negate bool, doWhile bool, t tail) {
	if doWhile {
		f.genDoWhile(pred, body, negate, t)
		return
	}
	valued := f.loopValue(body, t)
	defer f.hoistStrLoop(pred, body)()
	f.pushLoop(loopFor)
	f.loops[len(f.loops)-1].value = valued
	saved := f.enterBlock()
	f.indent++
	var cond string
	var narrow []narrowInfo
	// the condition reruns each iteration, so its statements (`while (x = q.shift)`) go inside the loop
	stmts := f.capture(func() { cond, narrow = f.genCond(pred) })
	if negate {
		cond = "!(" + cond + ")"
		narrow = nil
	}
	brk := notCode(cond) // the break test
	f.indent--
	if stmts == "" {
		f.emit("for %s {", cond)
	} else {
		f.emit("for {")
		f.buf.WriteString(stmts)
		f.emit("\tif %s {", brk)
		f.emit("\t\tbreak")
		f.emit("\t}")
	}
	f.indent++
	f.applyNarrow(narrow)
	f.redoLabel(body)
	f.c.markSureTake(pred, !negate, body)
	f.genStmts(body, tail{})
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.popLoop()
	if valued == nil || t.kind != tailAssign {
		f.emptyTail(pred, t)
	}
}

// loopValue is the tail a `break v` in a value-position loop's body assigns, or nil; an assigned value starts nil,
// since a break skips what follows the loop.
func (f *fctx) loopValue(body *parser.StatementsNode, t tail) *tail {
	if t.kind == tailNone || body == nil || !walkNode(body, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.BreakNode:
			return x.Arguments != nil, false
		case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.BlockNode, *parser.LambdaNode:
			return false, false
		}
		return false, !opensScope(x)
	}) {
		return nil
	}
	if t.kind == tailAssign {
		f.emptyTail(body, t)
	}
	return &t
}

// genDoWhile is `begin ... end while c`: the body runs first and the condition, which may read its locals, ends each pass.
func (f *fctx) genDoWhile(pred parser.Node, body *parser.StatementsNode, negate bool, t tail) {
	valued := f.loopValue(body, t)
	f.pushLoop(loopFor)
	f.loops[len(f.loops)-1].value = valued
	saved := f.enterBlock()
	f.emit("for {")
	f.indent++
	f.redoLabel(body)
	if body != nil && loopNext(body) != nil {
		// continue would skip the condition, so next is a goto; the inner block keeps it from jumping over declarations
		l := f.loops[len(f.loops)-1]
		l.next = "next" + f.newTmp()
		f.emit("{")
		inner := f.enterBlock()
		f.indent++
		f.genStmts(body, tail{})
		f.indent--
		f.leaveBlock(inner)
		f.emit("}")
		f.emit("%s:", l.next)
	} else {
		f.genStmts(body, tail{})
	}
	cond, _ := f.genCond(pred)
	if negate {
		f.emit("if %s {", cond)
	} else {
		f.emit("if %s {", notCode(cond))
	}
	f.emit("\tbreak")
	f.emit("}")
	f.indent--
	f.leaveBlock(saved)
	f.emit("}")
	f.popLoop()
	if valued == nil || t.kind != tailAssign {
		f.emptyTail(pred, t)
	}
}

// loopNext finds a `next` that targets the enclosing loop, not a nested loop, block or def.
func loopNext(n parser.Node) parser.Node {
	var found parser.Node
	walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.StatementsNode:
			return false, x != nil
		case *parser.NextNode:
			found = x
			return true, false
		case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
			return false, false
		}
		return false, true
	})
	return found
}

func (f *fctx) genBreak(n *parser.BreakNode) {
	if n.Arguments != nil && len(f.loops) > 0 && f.loops[len(f.loops)-1].value != nil {
		l := f.loops[len(f.loops)-1]
		args := n.Arguments.Arguments
		var v parser.Node = &parser.ArrayNode{Location: n.Location, Elements: args} // `break 1, 2` and `break *xs` are an Array
		if len(args) == 1 && !isSplat(args[0]) {
			v = args[0]
		}
		f.applyTail(n, f.genExpr(v, l.value.want()), *l.value)
		if l.value.kind == tailReturn && f.begins == l.begins && l.value.typ != nil && !isVoid(l.value.typ) {
			return // the return left the loop
		}
		f.emitBreak(l)
		return
	}
	if n.Arguments != nil && f.lenient() && len(f.loops) > 0 && f.loops[len(f.loops)-1].kind == loopIter {
		// a gem's `break v` out of an iterator whose value nothing reads (prime's each, yielding into a void block): v
		// runs for its effects, and the call's value is nil, as a broken-out gem loop's is (decisions 233, 410)
		l := f.loops[len(f.loops)-1]
		if args := n.Arguments.Arguments; l.broke != nil && l.broke.res != "" && len(args) == 1 && f.begins <= l.begins {
			f.emit("%s = %s", l.broke.res, f.coerce(n, f.genExpr(args[0], nil), TAny{})) // the loop is a value: the break's
			f.emitBreak(l)
			return
		}
		for _, a := range n.Arguments.Arguments {
			if v := f.genExpr(a, nil); v.code != "" && !v.done && !pureNode(a) {
				f.emit("_ = %s", v.code)
			}
		}
		f.emitBreak(l)
		return
	}
	if n.Arguments != nil && f.lenient() && len(f.loops) > 0 && f.loops[len(f.loops)-1].kind == loopClosure {
		// a gem's `break result if result` out of a closure block (rubocop's `extractors.find do ... end`): the call's
		// value is the break's (decision 444)
		if bc := f.breakCatch; bc != nil && len(f.rbFrames) > 0 && f.rbFrames[len(f.rbFrames)-1].key == bc.block.Location.StartOffset {
			var v expr
			if args := n.Arguments.Arguments; len(args) == 1 {
				v = f.genExpr(args[0], nil)
			} else {
				v = f.genExpr(&parser.ArrayNode{Location: n.Location, Elements: args}, nil)
			}
			bc.used = true
			f.emit("panic(rbBreak{id: %q, val: %s})", bc.id, f.coerce(n, v, TAny{}))
			return
		}
	}
	if n.Arguments != nil {
		f.errorf(n, "break with a value is not supported")
	}
	if len(f.loops) == 0 {
		f.errorf(n, "break outside a loop")
	}
	l := f.loops[len(f.loops)-1]
	if l.kind == loopClosure {
		if bc := f.breakCatch; bc != nil && len(f.rbFrames) > 0 && f.rbFrames[len(f.rbFrames)-1].key == bc.block.Location.StartOffset {
			bc.used = true
			f.emit("panic(rbBreak{id: %q})", bc.id)
			return
		}
		f.errorf(n, "break inside a non-iterator block is not supported (only one whose call is a statement, decision 241)")
	}
	f.emitBreak(l)
}

// breakCatchStmt generates n through genBreakCatch when it is a call statement whose block breaks out of it.
func (f *fctx) breakCatchStmt(n parser.Node, t tail) bool {
	c, ok := n.(*parser.CallNode)
	if !ok || t.kind != tailNone && !f.lenient() || f.breakCatch != nil && f.breakCatch.block == c.Block {
		return false
	}
	b, ok := c.Block.(*parser.BlockNode)
	if !ok || b.Body == nil || !blockBreaks(b.Body) && !(f.lenient() && blockBreaksValue(b.Body)) {
		return false
	}
	if t.kind != tailNone { // a gem's `segment.each do ... break ... end` as a block's value (liquid's For#render_segment, decision 331)
		f.genBreakCatchValue(c, b, t)
		return true
	}
	f.genBreakCatch(c, b, t)
	return true
}

// genBreakCatchValue is genBreakCatch for a call whose value is used: nil when the block broke, as MRI's valueless
// break makes it.
func (f *fctx) genBreakCatchValue(c *parser.CallNode, b *parser.BlockNode, t tail) {
	saved, out := f.breakCatch, f.buf
	bc := &breakCatch{block: b, id: fmt.Sprintf("%s:%d", f.f.Name, b.Location.StartOffset)}
	tmp := f.newTmp()
	f.breakCatch, f.buf = bc, &strings.Builder{}
	f.indent++
	e := f.genExpr(c, nil)
	if isVoid(e.typ) || e.stmt {
		f.emitExprStmt(c, e)
	} else {
		f.emit("%s = %s", tmp, f.coerce(c, e, TAny{}))
	}
	f.indent--
	body := f.buf.String()
	f.breakCatch, f.buf = saved, out
	f.emit("var %s any", tmp)
	f.emit("func() {")
	f.emit("\tdefer rbBreakRecoverTo(%q, &%s)", bc.id, tmp)
	out.WriteString(body)
	f.emit("}()")
	f.applyTail(c, expr{code: tmp, typ: TAny{}}, t)
}

// genBreakCatch is a call statement whose block has a `break`: when the block is a closure (decision 4), the
// statement runs inside a function that recovers the block's rbBreak, so the break leaves the call as on MRI;
// when the call compiles to a loop, the break is a plain Go break and nothing is wrapped (decision 241).
func (f *fctx) genBreakCatch(c *parser.CallNode, b *parser.BlockNode, t tail) {
	saved, out := f.breakCatch, f.buf
	bc := &breakCatch{block: b, id: fmt.Sprintf("%s:%d", f.f.Name, b.Location.StartOffset)}
	f.breakCatch, f.buf = bc, &strings.Builder{}
	defer func() { f.breakCatch, f.buf = saved, out }()
	f.indent++
	f.genStmt(c, t)
	f.indent--
	body := f.buf.String()
	f.buf = out
	if !bc.used {
		f.indent++ // the body was generated one level in
		out.WriteString(dedent(body))
		f.indent--
		return
	}
	f.emit("func() {")
	f.emit("\tdefer rbBreakRecover(%q)", bc.id)
	out.WriteString(body)
	f.emit("}()")
}

// dedent drops one leading tab from each line.
func dedent(s string) string {
	lines := strings.SplitAfter(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "\t")
	}
	return strings.Join(lines, "")
}

// blockBreaks reports a valueless `break` that leaves this block: not one inside a nested loop, block or definition.
func blockBreaks(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.BreakNode:
			return x.Arguments == nil, false
		case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.BlockNode, *parser.LambdaNode:
			return false, false
		}
		return false, !opensScope(x)
	})
}

// blockBreaksValue is blockBreaks for a `break value`.
func blockBreaksValue(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.BreakNode:
			return x.Arguments != nil, false
		case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.BlockNode, *parser.LambdaNode:
			return false, false
		}
		return false, !opensScope(x)
	})
}

func (f *fctx) emitBreak(l *loopFrame) {
	if f.begins > l.begins {
		f.leaveWrapper(jumpBreak)
		return
	}
	if l.broke != nil { // Go has no for-else: the jump skips the value a loop that ran to its end has
		l.broke.used = true
		f.emit("goto %s", l.broke.label)
		return
	}
	if f.switches > l.switches {
		if l.label == "" {
			f.labels++
			l.label = fmt.Sprintf("loop%d", f.labels)
		}
		f.emit("break %s", l.label)
		return
	}
	f.emit("break")
}

func (f *fctx) genNext(n *parser.NextNode) {
	if len(f.loops) == 0 {
		f.errorf(n, "next outside a loop")
	}
	l := f.loops[len(f.loops)-1]
	if n.Arguments != nil {
		if l.kind == loopClosure {
			f.closureValue(n, n.Arguments)
			return
		}
		f.errorf(n, "next with a value is not supported")
	}
	f.emitNext(n, l)
}

// closureValue ends a value-returning block (`next v`) or lambda (`return v`) with v.
func (f *fctx) closureValue(n parser.Node, args *parser.ArgumentsNode) {
	if args == nil {
		f.emitNext(n, f.loops[len(f.loops)-1])
		return
	}
	if len(args.Arguments) != 1 {
		f.errorf(n, "multiple values are not supported")
	}
	t := f.nextTail
	if t.kind != tailReturn {
		f.errorf(n, "a block's value is not used here")
	}
	f.applyTail(n, f.genExpr(args.Arguments[0], t.typ), t)
}

// emitNext leaves the innermost loop's iteration; n locates coerce errors.
func (f *fctx) emitNext(n parser.Node, l *loopFrame) {
	switch {
	case f.begins > l.begins:
		f.leaveWrapper(jumpNext)
	case l.kind == loopClosure:
		// the block's value is nil
		switch t := f.nextTail; {
		case t.kind == tailNone:
			f.emit("return")
		case t.typ == nil:
			t.record(TNil{})
			f.emit("return")
		case isClass(t.typ, "Boolean"): // nil is falsy
			f.emit("return false")
		default:
			f.emit("return %s", f.coerce(n, expr{code: "nil", typ: TNil{}}, t.typ))
		}
	case l.next != "":
		f.emit("goto %s", l.next)
	default:
		f.emit("continue")
	}
}
