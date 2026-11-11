package compiler

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// nilCheckComment is a source's `# nil_check: mode` line (decision 393).
var nilCheckComment = regexp.MustCompile(`(?m)^# nil_check: (\S+)\s*$`)

// checkNilMode rejects a mode other than compile or runtime ("" is compile).
func checkNilMode(where, mode string) string {
	if !slices.Contains([]string{"", "compile", "runtime"}, mode) {
		panic(compileError{msg: fmt.Sprintf("%s: %q is not compile or runtime", where, mode)})
	}
	return mode
}

// nilRuntime: a T? where T is expected raises on nil at run time instead of decision 20's error (file's line, else the flag; decision 393).
func (f *fctx) nilRuntime() bool {
	if f.f != nil && f.f.isPrelude() {
		return false
	}
	if f.f != nil && f.f.nilCheck != "" {
		return f.f.nilCheck == "runtime"
	}
	return f.c.env.NilCheck == "runtime"
}

// nilSite is where a T? meets a T: the file and the offset of the node coerced.
type nilSite struct {
	file *File
	off  int
}

// nilDeferral: sites an inferred container's typing made possibly-nil are checked at run time (decision 440).
type nilDeferral struct {
	rec   map[nilSite]bool // the untyped baseline run: every site deferred and noted
	pre   map[nilSite]bool // while a typing is tried: the baseline's sites, not the typing's, stay errors
	sites map[nilSite]bool // the deferred sites
}

// at reports whether d checks site k at run time; decided is false when d has no say.
func (d *nilDeferral) at(k nilSite) (ok, decided bool) {
	switch {
	case d == nil:
		return false, false
	case d.rec != nil:
		d.rec[k] = true
		return true, true
	case d.sites[k]:
		return true, true
	case d.pre != nil && !d.pre[k]:
		d.sites[k] = true
		return true, true
	}
	return false, false
}

// nilAt: the T? coerced at n is checked at run time, under --nil-check=runtime or at a site decision 440 deferred.
func (f *fctx) nilAt(n parser.Node) bool {
	if f.nilRuntime() {
		return true
	}
	if n == nil || f.f == nil || f.f.isPrelude() || f.lenient() {
		return false
	}
	k := nilSite{f.f, n.GetLocation().StartOffset}
	if ok, decided := f.c.nilDefer.at(k); decided { // an ivar's typing (refineIvars)
		return ok
	}
	ok, _ := f.nilDefer.at(k) // a local's (analyze)
	return ok
}

// nilRetry is decision 440's second try of a container typing a nil error alone vetoed.
type nilRetry struct {
	pre  map[nilSite]bool // the untyped baseline's nil sites, which stay errors
	done bool
}

// try: a failed baseline gives no retry, since the sites it missed could be anyone's.
func (r *nilRetry) try(baseline, attempt func(*nilDeferral) bool) (ok, used bool) {
	if !r.done {
		r.done = true
		if rec := (&nilDeferral{rec: map[nilSite]bool{}}); baseline(rec) {
			r.pre = rec.rec
		}
	}
	if r.pre == nil {
		return false, false
	}
	d := &nilDeferral{pre: r.pre, sites: map[nilSite]bool{}}
	ok = attempt(d)
	d.pre = nil // later sites are not the typing's
	return ok, ok && len(d.sites) > 0
}

// retryNil is ok, else r's retry of a typing err vetoed when err is a nil veto (decision 440).
func (c *Compiler) retryNil(r *nilRetry, ok bool, err *compileError, baseline, attempt func(*nilDeferral) bool) bool {
	if ok || err == nil || !err.nilVeto || c.noNilDefer {
		return ok
	}
	retried, used := r.try(baseline, attempt)
	if used {
		c.noteNilDefer()
	}
	return retried
}

// noteNilDefer records that this compile kept a decision 440 typing (options.nilDeferred).
func (c *Compiler) noteNilDefer() {
	if c.nilDeferUsed != nil {
		*c.nilDeferUsed = true
	}
}

// nilErrorf marks decision 20's error a nil veto: a typing it alone vetoes is retried with the sites deferred (decision 440).
func (f *fctx) nilErrorf(n parser.Node, format string, args ...any) {
	err := catchCompileError(func() { f.errorf(n, format, args...) })
	err.nilVeto = true
	panic(*err)
}

// nilCall is the call whose argument is being coerced, for the error MRI raises when that argument is nil.
type nilCall struct {
	m     *Method
	recv  Type
	param Type // the parameter as declared: an `E` element (Array#[]=) is no conversion MRI makes
}

var (
	nilCoerceOps  = []string{"+", "-", "*", "/", "%", "**", "&", "|", "^", "fdiv", "div", "divmod", "modulo", "quo", "pow", "remainder"}
	nilCompareOps = []string{"<", ">", "<=", ">=", "between?"}
	nilEqualOps   = map[string]bool{"==": true, "!=": true, "<=>": true, "eql?": true, "equal?": true, "===": true}
)

// nilMessage is MRI's error where a core method's argument is nil, else a TypeError at the boundary (decision 393).
func (f *fctx) nilMessage(to Type) (string, bool) {
	want := to.String()
	if c, ok := to.(TClass); ok {
		want = c.C.RubyName
	}
	m := f.nilArg.m
	if m == nil || m.Owner == nil || m.Owner.File == nil || !m.Owner.File.isPrelude() {
		return "no implicit conversion of nil into " + want, false
	}
	recv := m.Owner.RubyName
	if rc, ok := stripOpt(f.nilArg.recv).(TClass); ok {
		recv = rc.C.RubyName
	}
	numeric := recv == "Integer" || recv == "Float"
	_, declared := f.nilArg.param.(TClass)
	switch {
	case slices.Contains(nilCompareOps, m.Name):
		return "comparison of " + recv + " with nil failed", true
	case numeric && slices.Contains(nilCoerceOps, m.Name):
		return "nil can't be coerced into " + recv, false
	case !declared || recv == "Integer" && (m.Name == "<<" || m.Name == ">>"):
	case want == "Integer":
		return "no implicit conversion from nil to integer", false
	case want == "Float":
		return "can't convert nil into Float", false
	}
	return "no implicit conversion of nil into " + want, false
}

// nilTwin: only numeric operators pick a twin by the unwrapped T; a twin taking T? (`Integer(h[:k])`) already has MRI's message.
func (f *fctx) nilTwin(n parser.Node, m *Method, o TOpt) bool {
	return towerOps[m.Name] && m.Name != "<=>" && numLevel(classOf(o.Elem)) >= 0 && f.nilAt(n)
}

// nilNumArgs unwraps a numeric T? argument before numericTower picks the class to convert to, as MRI's coerce fails on nil first.
func (f *fctx) nilNumArgs(m *Method, recv Type, xs []expr, nodes []parser.Node) {
	if nilEqualOps[m.Name] {
		return
	}
	for i, x := range xs {
		if o, ok := x.typ.(TOpt); ok && numLevel(classOf(o.Elem)) >= 0 && f.nilAt(nodes[i]) {
			prev := f.nilArg
			f.nilArg = nilCall{m: m, recv: recv}
			xs[i] = f.nilUnwrap(nodes[i], x, o.Elem)
			f.nilArg = prev
			nodes[i].(*exprNode).e = xs[i] //nolint:forcetypeassert // genNumArgs makes exprNodes
		}
	}
}

// nilSpaceship is `a <=> b` of a T? b: MRI answers nil for a nil b, where the typed `<=>` would take its unwrapped T.
func (f *fctx) nilSpaceship(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if name != "<=>" || len(args) != 1 || block != nil || classOf(recv.typ) == nil {
		return expr{}, false
	}
	if e := classOf(recv.typ).lookup(name); e == nil || e.Owner.File == nil || !e.Owner.File.isPrelude() {
		return expr{}, false // a program's own <=> is called with the nil, as MRI does
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	o, ok := a.typ.(TOpt)
	if !ok || isAny(o.Elem) || !f.nilAt(args[0]) {
		return expr{}, false
	}
	rt, at, res := f.newTmp(), f.newTmp(), f.newTmp()
	f.emit("%s := %s", rt, f.materialize(recv))
	a = f.genExpr(args[0], nil)
	f.emit("%s := %s", at, a.code)
	ret := TOpt{Elem: f.cls("Integer")}
	f.emit("var %s %s", res, f.c.goType(ret))
	f.emit("if %s {", optNotNil(at, a.typ))
	x := f.genMethodCall(n, expr{code: rt, typ: recv.typ}, name, []parser.Node{&exprNode{Node: args[0], e: expr{code: derefOpt(at, a.typ), typ: o.Elem}}}, nil)
	f.emit("%s = %s", res, f.coerce(n, x, ret))
	f.emit("}")
	return expr{code: res, typ: ret}, true
}

// optWhereClass is a T? where class T is expected: decision 20's error, or under --nil-check=runtime its T, checked.
func (f *fctx) optWhereClass(n parser.Node, e expr, to TClass) string {
	if f.nilAt(n) {
		return f.coerceClass(n, f.nilUnwrap(n, e, to), to)
	}
	f.nilErrorf(n, "possibly-nil %s where %s is expected; check it first (`if x`, `x ||= ...`, `return unless x`)", e.typ, to)
	return ""
}

// unionMismatch is a value no member of to takes; a T? whose T one does is checked under --nil-check=runtime.
func (f *fctx) unionMismatch(n parser.Node, e expr, to TUnion) string {
	if isOpt(e.typ) && f.nilAt(n) {
		return f.coerceToUnion(n, f.nilUnwrap(n, e, to), to)
	}
	if isOpt(e.typ) {
		f.nilErrorf(n, "%s where %s is expected", e.typ, to)
	}
	f.errorf(n, "%s where %s is expected", e.typ, to)
	return ""
}

// nilUnwrap is a T? value as its T under --nil-check=runtime, raising nilMessage's error when it is nil (decision 393).
func (f *fctx) nilUnwrap(n parser.Node, e expr, to Type) expr {
	o := e.typ.(TOpt) //nolint:forcetypeassert // callers test isOpt
	msg, arg := f.nilMessage(to)
	if !f.nilRuntime() && n != nil { // decision 440's deferred site: not silent
		f.warn(n, "possibly-nil %s where %s is expected is checked at run time: an inferred `[]`/`{}` keeps its element type (decision 440)", e.typ, to)
	}
	if leafOpt(o) { // a leaf's *C or an interface: nil is the value's own zero (decisions 360, 378)
		return expr{code: fmt.Sprintf("rbNilArgV(%s, %q, %t)", e.code, msg, arg), typ: o.Elem}
	}
	if valOpt(o) {
		return expr{code: fmt.Sprintf("rbNilArgO(%s, %q, %t)", e.code, msg, arg), typ: o.Elem}
	}
	return expr{code: fmt.Sprintf("rbNilArg(%s, %q, %t)", e.code, msg, arg), typ: o.Elem}
}
