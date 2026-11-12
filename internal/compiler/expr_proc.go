package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genLambda builds a Proc value (`->(x) { }`, `lambda { |x| }`, `proc { }`)
// as a pointer to a Go closure. Its parameter types come from the expected
// type (`#: ^(Integer) -> Integer`); a lambda without parameters may infer
// its return type from its body.
func (f *fctx) genLambda(n, block, params parser.Node, expected Type) expr {
	sig := &BlockSig{Ret: TVar{Name: "Ret_"}}
	if o, ok := expected.(TOpt); ok { // a lambda literal where a Proc? is expected is never nil
		expected = o.Elem
	}
	var src lambdaSrc
	if ft, ok := expected.(TFunc); ok && ft.Proc {
		sig = &BlockSig{Params: ft.Params, Ret: ft.Ret, Rest: ft.Rest}
		if names := f.blockParamList(params); ft.Rest && (len(names) != len(ft.Params) || !names.hasRest()) && !isProcLiteral(n, ft) {
			f.errorf(n, "a lambda for %s needs %d params, the last a *rest", ft, len(ft.Params))
		}
	} else if names := f.blockParamList(params); len(names) > 0 {
		sig.Params, src = f.lambdaParams(n, names, block) // from its calls (decision 146)
		sig.Rest = names.hasRest()
	}
	env := map[string]Type{}
	saved := f.lambdaClosure
	f.lambdaClosure = f.closures + 1
	defer func() { f.lambdaClosure = saved }()
	var code string
	if src != "" && f.c.round { // its still-untyped parameters may break the body this round: its calls must still record what they pass
		if f.attempt(func() { code = f.genClosure(n, block, sig, env) }) != nil {
			return expr{code: "nil", typ: TFunc{Params: sig.Params, Ret: TAny{}, Proc: true, Src: src, Rest: sig.Rest}}
		}
	} else {
		code = f.genClosure(n, block, sig, env)
	}
	ft := TFunc{Params: sig.Params, Ret: subst(sig.Ret, env), Proc: true, Src: src, Rest: sig.Rest}
	if l, ok := n.(*parser.LambdaNode); ok && f.f.lambdaOpt[l] > 0 { // `->(value, local = false)` takes 1 or 2 (net-ftp's TIME_PARSER, decision 416)
		f.c.noteProcMinArity(ft, len(sig.Params)-f.f.lambdaOpt[l])
	}
	return expr{code: "Ref(" + code + ")", typ: ft}
}

// noteProcMinArity records that a Proc of ft's Go type may be called with as few as n arguments: a dynamic call checks the
// fewest any such lambda takes.
func (c *Compiler) noteProcMinArity(ft TFunc, n int) {
	g := c.goType(ft)
	if c.procMinArity == nil {
		c.procMinArity = map[string]int{}
	}
	if cur, ok := c.procMinArity[g]; !ok || n < cur {
		c.procMinArity[g] = n
	}
}

// isProcLiteral is `proc { }` or `Proc.new { }` for an untyped `^(*untyped)` Proc: a proc, not a lambda, takes any count,
// binding its names from the rest as a block's (decision 364).
func isProcLiteral(n parser.Node, ft TFunc) bool {
	c, ok := n.(*parser.CallNode)
	return ok && untypedRest(ft.Params, ft.Rest) && (c.Name == "proc" && c.Receiver == nil || c.Name == "new" && isTopConst(c.Receiver, "Proc"))
}

// selfProcArg passes `&p` straight on when p is a Proc whose self is rebound and the block wanted rebinds the same
// self (decision 164): its Go func already takes self first, and wrapping it in a lexical call cannot.
func (f *fctx) selfProcArg(b *parser.BlockArgumentNode, self Type) (string, bool) {
	if self == nil {
		return "", false
	}
	pt := f.probeType(b.Expression, nil)
	o, isOpt := pt.(TOpt)
	if isOpt {
		pt = o.Elem
	}
	ft, ok := pt.(TFunc)
	if !ok || !ft.Proc || ft.Self == nil || !typeEq(ft.Self, self) {
		return "", false
	}
	p := f.genExpr(b.Expression, nil)
	if isOpt {
		return "rbBlockOf(" + p.code + ")", true // `&nil` passes no block, as MRI's (Formula#service's `&self.class.service`)
	}
	return derefCode(p.code), true
}

// blockArgBlock is the block `&expr` stands for: a Method taken by name, a Method value or a Proc.
func (f *fctx) blockArgBlock(b *parser.BlockArgumentNode, nparams int) *parser.BlockNode {
	if mb := f.methodRefBlock(b, nparams); mb != nil {
		return mb
	}
	if pb := f.procBlock(b); pb != nil {
		return pb
	}
	f.errorf(b, "only &:symbol, a Proc, a Method and a method's own &block are supported as block arguments")
	return nil
}

// procBlock desugars `&f` for a Proc f to `{ |x_0, ...| f.call(x_0, ...) }`, or nil when f is not a Proc.
func (f *fctx) procBlock(ba *parser.BlockArgumentNode) *parser.BlockNode {
	pt := f.probeType(ba.Expression, nil)
	if tc, ok := pt.(TClass); ok && tc.C.RubyName == yielderClass {
		return yielderBlock(ba)
	}
	if kind, _, typed := methodFn(pt); f.lenient() && (isAny(pt) || kind == "Method" && !typed) {
		// a gem's untyped callable as a block (rubocop's `aligned_with_any_line_range?(ranges, range, &predicate)`, where
		// predicate is one of two Method objects): `{ |*xs| predicate.call(*xs) }`, called when it runs (decision 444)
		loc := ba.Location
		xs := "xs_"
		call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "call", Arguments: &parser.ArgumentsNode{Location: loc,
			Arguments: []parser.Node{&parser.SplatNode{Location: loc, Expression: &parser.LocalVariableReadNode{Location: loc, Name: xs}}}}}
		return &parser.BlockNode{
			Location:   loc,
			Locals:     []string{xs},
			Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Rest: &parser.RestParameterNode{Location: loc, Name: &xs}}},
			Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
		}
	}
	ft, ok := pt.(TFunc)
	if kind, mf, typed := methodFn(pt); kind == "Method" {
		if !typed {
			f.errorf(ba, "&%s needs the method's signature, which a %s lost (decision 141)", f.f.text(ba.Expression.GetLocation()), pt)
		}
		ft, ok = mf, true
	}
	if !ok || !ft.Proc {
		return nil
	}
	loc := ba.Location
	var ps, args []parser.Node
	var locals []string
	for i := range ft.Params {
		name := "x_" + strconv.Itoa(i)
		locals = append(locals, name)
		ps = append(ps, &parser.RequiredParameterNode{Location: loc, Name: name})
		args = append(args, &parser.LocalVariableReadNode{Location: loc, Name: name})
	}
	call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "call", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: args}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     locals,
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: ps}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// genInstanceEval compiles instance_eval/class_eval/module_eval (and the
// *_exec forms) with a Proc or a literal block: the block runs with self
// rebound to the receiver (decision 164). A Proc stored from an escaping
// &block takes the receiver as its leading self param, and the receiver's
// class is recorded as that block's self type.
func (f *fctx) genInstanceEval(n *parser.CallNode, recv expr) (expr, bool) {
	switch n.Name {
	case "instance_eval", "class_eval", "module_eval", "instance_exec", "class_exec", "module_exec":
	default:
		return expr{}, false
	}
	args := callArgs(n)
	exec := strings.HasSuffix(n.Name, "_exec")
	if units := f.runEvalsAt(n); len(units) > 0 {
		return f.genRunEval(n, units), true
	}
	if n.Block == nil && f.lenient() && len(args) > 0 { // gem code (decision 172)
		return f.gemStringEval(n), true
	}
	if n.Block == nil {
		f.errorf(n, "%s needs a block or a Proc (a String to eval is not supported)", n.Name)
	}
	switch b := n.Block.(type) {
	case *parser.BlockNode:
		if b.Parameters != nil {
			f.errorf(b, "%s with a literal block that takes parameters is not supported yet", n.Name)
		}
		if exec && len(args) > 0 {
			f.errorf(n, "%s with a literal block and arguments is not supported yet", n.Name)
		}
		if e, ok := f.genIvarsEval(b, recv); ok {
			return e, true
		}
		env := map[string]Type{}
		sig := &BlockSig{Ret: TVar{Name: "Ret_"}, Self: recv.typ}
		code := f.genClosure(n, b, sig, env)
		return expr{code: code + "(" + recv.code + ")", typ: subst(sig.Ret, env)}, true
	case *parser.BlockArgumentNode:
		return f.evalProcArg(n, b, recv, args, exec), true
	}
	f.errorf(n, "%s needs a literal block or a Proc", n.Name)
	return expr{}, false
}

// evalProcArg is instance_eval/instance_exec with `&proc`: a Proc that escaped takes the receiver as its self
// (decision 164); a gem's untyped one rebinds through rbSelfArg (decision 324).
func (f *fctx) evalProcArg(n *parser.CallNode, b *parser.BlockArgumentNode, recv expr, args []parser.Node, exec bool) expr {
	if lr, ok := b.Expression.(*parser.LocalVariableReadNode); ok && (n.Receiver == nil || isSelf(n.Receiver)) && f.localProcLiteral(lr.Name) {
		// `instance_exec(*a, &blk)` on self with a proc this method made: its self is already ours, so it is blk.call(*a)
		return f.genMethodCall(n, f.genExpr(lr, nil), "call", args, nil)
	}
	target := b.Expression
	if c, ok := target.(*parser.CallNode); ok && f.lenient() && f.f.sorbetCasts[c] != "" && isAny(f.probeType(callArgs(c)[0], nil)) {
		target = callArgs(c)[0] // `&T.cast(block, T.proc.returns(T.anything))` of an untyped block still rebinds (Homebrew's AbstractFlightBlock#abstract_phase)
	}
	p := f.genExpr(target, nil)
	if isAny(p.typ) && target != b.Expression {
		ft := TFunc{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}, Proc: true}
		p = expr{code: f.coerce(b, p, ft), typ: ft}
	}
	if o, isOpt := p.typ.(TOpt); isOpt { // instance_eval(&nil) raises, as MRI does
		if ft, ok := o.Elem.(TFunc); ok && ft.Proc {
			tmp := f.newTmp()
			f.emit("%s := %s", tmp, p.code)
			f.emit("if %s == nil {\n\tpanic(NewArgumentError(Ref(String(%q))))\n}", tmp, "wrong number of arguments (given 0, expected 1..3)")
			p = expr{code: "(*" + tmp + ")", typ: ft}
		}
	}
	pt, ok := p.typ.(TFunc)
	if !ok || !pt.Proc {
		f.errorf(b, "%s needs a Proc, got %s", n.Name, p.typ)
	}
	if pt.Self == nil && f.lenient() && pt.Rest && len(pt.Params) == 1 && isAny(pt.Params[0]) && isAny(pt.Ret) {
		return f.evalUntypedBlock(n, p, recv, args, exec)
	}
	if pt.Self == nil {
		f.errorf(b, "%s on a Proc that does not escape cannot rebind self (decision 164)", n.Name)
	}
	if len(args) != len(pt.Params) {
		f.errorf(n, "%s passes %d values but the block takes %d", n.Name, len(args), len(pt.Params))
	}
	if pt.SelfPending != "" {
		f.noteUse(Param{Pending: pt.SelfPending}, n, f.concreteSelf(recv.typ))
	}
	codes := make([]string, 0, len(args)+1)
	self := recv.code
	if o, ok := recv.typ.(TOpt); ok && !isOpt(pt.Self) { // `@livecheck.instance_eval(&block)` of a Livecheck?: the object itself
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, recv.code)
		// ponytail: MRI runs the block with self nil, failing at its first call; this raises before it runs
		f.emit("if %s == nil {\n\tpanic(rbNoMethod(%q, nil, false))\n}", tmp, n.Name)
		self = f.coerce(n, expr{code: derefOpt(tmp, o), typ: o.Elem}, pt.Self)
	}
	codes = append(codes, self)
	for i, a := range args {
		e := f.genExpr(a, pt.Params[i])
		codes = append(codes, f.coerce(a, e, pt.Params[i]))
	}
	return expr{code: derefCode(p.code) + "(" + strings.Join(codes, ", ") + ")", typ: pt.Ret}
}

// gemStringEval is a gem's run-time string eval: it fails only if it runs, as a ScriptError a loader can rescue
// (Formulary's `module_eval(contents, path)`, decision 301).
func (f *fctx) gemStringEval(n *parser.CallNode) expr {
	if f.m != nil && f.m.Name == "method_missing" && n.Name != "instance_eval" {
		// a method_missing memoizing the name it answers as a def (bindata's BasePrimitive): without it, the next
		// call reaches method_missing again and gets the same answer (decision 324)
		return expr{code: "nil", typ: TNil{}, lit: true}
	}
	msg := fmt.Sprintf("rugaroo: %s of a String at run time is not supported (%s:%d)", n.Name, f.f.Name, f.f.line(n.Location.StartOffset))
	return expr{code: fmt.Sprintf("panic(NewNotImplementedError(Ref(String(%q))))", msg), typ: TVoid{}, stmt: true, noreturn: true}
}

// evalUntypedBlock is a gem's instance_eval/instance_exec of an untyped &block (bindata's `parser.instance_eval(&block)`):
// a dynamic block rebinds the self it is given first (decision 324).
func (f *fctx) evalUntypedBlock(n parser.Node, p, recv expr, args []parser.Node, exec bool) expr {
	self := f.coerce(n, recv, TAny{})
	codes := []string{"*" + p.code, self}
	if !exec {
		codes = append(codes, self) // instance_eval yields the receiver too
	}
	for _, a := range args {
		if exec {
			codes = append(codes, f.coerce(a, f.genExpr(a, nil), TAny{}))
		}
	}
	return expr{code: "rbEvalBlock(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// procCall is a method on a Proc value.
func (f *fctx) procCall(n parser.Node, recv expr, t TFunc, name string, args []parser.Node, block parser.Node) expr {
	if block != nil && slices.Contains([]string{"call", "()", "[]", "yield", "===", "arity", "lambda?", "parameters", "to_proc", ">>", "<<"}, name) {
		f.errorf(n, "a Proc's %s does not take a block", name)
	}
	switch name {
	case "call", "()", "[]", "yield", "===":
		return f.procInvoke(n, recv, t, args)
	case "arity":
		f.discard(recv)
		arity := len(t.Params)
		if t.Rest {
			arity = -arity // MRI: ->(a, *b) is -2
		}
		return expr{code: strconv.Itoa(arity), typ: f.cls("Integer"), lit: true}
	case "lambda?":
		f.discard(recv)
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}
	case "parameters":
		return f.procParameters(recv, t)
	case "to_proc":
		return recv
	case ">>", "<<":
		return f.procCompose(n, recv, t, name, args)
	}
	if e := f.c.classes["Proc"].lookup(name); e != nil && !untypedIntrinsics[name] {
		if e.Owner.Name == "Proc" {
			f.errorf(n, "a Proc is a Go func: a method defined on Proc (%s) cannot be called on one", name)
		}
		return f.callEntry(n, e, recv, args, block)
	}
	return f.universalCall(n, recv, name, args, block)
}

// procParameters is Proc#parameters from the Proc's Go type: each parameter `[:req, :name]` (`:rest` for a `*rest`), as a
// lambda's (lambda? is always true here). A Go func carries no names, so they are positional (`:p1`, `:p2`, ...):
// rubocop-ast's `with_temp_variables(&block)` reads `block.parameters.map(&:last)` only for the text of the Ruby
// source it generates, which boot's eval record already compiled (decision 204). ponytail: a literal block's names
// would need carrying beside the func.
func (f *fctx) procParameters(recv expr, t TFunc) expr {
	f.discard(recv)
	items := make([]string, 0, len(t.Params))
	for i := range t.Params {
		kind := "req"
		if t.Rest && i == len(t.Params)-1 {
			kind = "rest"
		}
		items = append(items, fmt.Sprintf(`&Array[Symbol]{s: []Symbol{Symbol(%q), Symbol("p%d")}}`, kind, i+1))
	}
	sym := f.cls("Symbol")
	inner := TClass{C: f.c.classes["Array"], Args: []Type{sym}}
	code := fmt.Sprintf("&Array[*Array[Symbol]]{s: []*Array[Symbol]{%s}}", strings.Join(items, ", "))
	return expr{code: code, typ: TClass{C: f.c.classes["Array"], Args: []Type{inner}}}
}

// procInvoke is a Proc's call/()/[]/yield/===: an arity check, each argument
// coerced to its parameter, then a Go call through the function pointer.
func (f *fctx) procInvoke(n parser.Node, recv expr, t TFunc, args []parser.Node) expr {
	if f.lenient() && !t.Rest && t.Self == nil && (len(args) != len(t.Params) || slices.ContainsFunc(args, isSplat)) {
		return f.procInvokeLoose(recv, t, args)
	}
	last := len(t.Params) - 1
	if t.Rest && isAny(t.Params[last]) && t.Self == nil && len(args) > last && slices.ContainsFunc(args[last:], isSplat) {
		// `task.call(*args)` of a (*untyped) Proc: the leading values, then every rest value spread into one slice
		codes := make([]string, last, last+1)
		for i, a := range args[:last] {
			codes[i] = f.coerce(a, f.genExpr(a, t.Params[i]), t.Params[i])
		}
		if t.Src.storedBlock() {
			f.noteUse(Param{Pending: t.Src.argc(maxStoredBlockArgs + 1)}, n, TNil{}) // a splat: any count
		}
		spread, ok := f.anySpread(args[last:])
		if !ok {
			spread = "[]any{" + strings.Join(f.anyArgs(args[last:]), ", ") + "}"
		}
		return expr{code: derefCode(recv.code) + "(" + strings.Join(append(codes, spread+"..."), ", ") + ")", typ: t.Ret}
	}
	switch {
	case t.Rest && len(args) < last:
		f.errorf(n, "wrong number of arguments (given %d, expected %d+)", len(args), last)
	case !t.Rest && len(args) != len(t.Params):
		f.errorf(n, "wrong number of arguments (given %d, expected %d)", len(args), len(t.Params))
	}
	codes := make([]string, len(args))
	for i, a := range args {
		pi := i
		if t.Rest {
			pi = min(i, last) // every argument past the leading ones is the rest's element
		}
		want := t.Params[pi]
		if t.Src != "" && f.c.round && isAny(want) {
			want = nil
		}
		e := f.genExpr(a, want)
		switch {
		case t.Src.storedBlock(): // each argument by position, and the count (typeStoredBlock)
			f.noteUse(Param{Pending: t.Src.arg(i)}, a, e.typ)
		case t.Src != "":
			f.noteUse(Param{Pending: t.Src.arg(pi)}, a, e.typ)
		}
		codes[i] = f.coerce(a, e, t.Params[pi])
	}
	if t.Src.storedBlock() {
		f.noteUse(Param{Pending: t.Src.argc(min(len(args), maxStoredBlockArgs+1))}, n, TNil{})
	}
	if t.Self != nil { // an escaped block: its self is rebound by instance_eval, so Proc#call is a compile error (decision 164)
		if t.SelfPending != "" {
			f.noteUse(Param{Pending: t.SelfPending}, n, f.concreteSelf(f.selfType))
		}
		f.errorf(n, "a Proc whose self is rebound by instance_eval cannot be called with call/./[]: it takes no lexical self (decision 164)")
	}
	return expr{code: derefCode(recv.code) + "(" + strings.Join(codes, ", ") + ")", typ: t.Ret}
}

// procCompose is a Proc's >> or <<: a ref'd closure calling one Proc then the
// other, the receiver first for >> and the argument first for <<.
func (f *fctx) procCompose(n parser.Node, recv expr, t TFunc, name string, args []parser.Node) expr {
	if len(args) != 1 {
		f.errorf(n, "%s takes one Proc", name)
	}
	mark := f.buf.Len()
	g := f.genExpr(args[0], nil)
	pin := []expr{recv} // the receiver runs first, whichever side of the pair it ends up on (decision 98)
	f.pinExprs(mark, pin)
	recv = pin[0]
	gt, ok := g.typ.(TFunc)
	if !ok || !gt.Proc {
		f.errorf(n, "%s takes a Proc, got %s", name, g.typ)
	}
	firstT, secondT := t, gt
	if name == "<<" {
		firstT, secondT = gt, t
	}
	if len(secondT.Params) != 1 || !typeEq(secondT.Params[0], firstT.Ret) {
		f.errorf(n, "cannot compose %s with %s", firstT, secondT)
	}
	out := TFunc{Params: firstT.Params, Ret: secondT.Ret, Proc: true}
	ps := make([]string, 0, len(firstT.Params))
	as := make([]string, 0, len(firstT.Params))
	for i, p := range firstT.Params {
		ps = append(ps, "a"+strconv.Itoa(i)+" "+f.c.goType(p))
		as = append(as, "a"+strconv.Itoa(i))
	}
	inner := "(*first)(" + strings.Join(as, ", ") + ")"
	body := "return (*second)(" + inner + ")"
	if isVoid(secondT.Ret) {
		body = "(*second)(" + inner + ")"
	}
	ret := ""
	if !isVoid(secondT.Ret) {
		ret = " " + f.c.goType(secondT.Ret)
	}
	// the literal's parameters follow the written order of the operands, so Go evaluates the receiver first
	params := "first " + f.c.goType(firstT) + ", second " + f.c.goType(secondT)
	if name == "<<" {
		params = "second " + f.c.goType(secondT) + ", first " + f.c.goType(firstT)
	}
	code := fmt.Sprintf("func(%s) %s { return Ref(func(%s)%s { %s }) }(%s, %s)",
		params, f.c.goType(out), strings.Join(ps, ", "), ret, body, recv.code, g.code)
	return expr{code: code, typ: out}
}

// localProcLiteral reports a local of the current method assigned a `proc { }` literal, whose self is the method's.
func (f *fctx) localProcLiteral(name string) bool {
	if f.m == nil || f.m.Node == nil {
		return false
	}
	return anyNode(f.m.Node.Body, func(n parser.Node) bool {
		w, ok := n.(*parser.LocalVariableWriteNode)
		if !ok || w.Name != name {
			return false
		}
		c, ok := w.Value.(*parser.CallNode)
		return ok && c.Receiver == nil && (c.Name == "proc" || c.Name == "lambda") && c.Block != nil
	})
}

// procInvokeLoose calls a proc as Ruby's proc semantics do (gem code): a splat spread by position, extra arguments
// dropped, missing ones nil.
func (f *fctx) procInvokeLoose(recv expr, t TFunc, args []parser.Node) expr {
	var vals []string // each argument as untyped, a splat's elements spread
	spread := ""
	for _, a := range args {
		if sp, ok := a.(*parser.SplatNode); ok {
			e := f.genExpr(sp.Expression, nil)
			tmp := f.newTmp()
			f.emit("%s := rbAs[*Array[any]](%s, \"Array\").s", tmp, f.coerce(sp, e, TAny{}))
			spread = tmp
			continue
		}
		vals = append(vals, f.coerce(a, f.genExpr(a, nil), TAny{}))
	}
	all := f.newTmp()
	f.emit("%s := []any{%s}", all, strings.Join(vals, ", "))
	if spread != "" {
		f.emit("%s = append(%s, %s...)", all, all, spread)
	}
	codes := make([]string, len(t.Params))
	for i, p := range t.Params {
		codes[i] = fmt.Sprintf("rbAs[%s](rbAt(%s, %d), %q)", f.c.goType(p), all, i, p.String())
	}
	return expr{code: derefCode(recv.code) + "(" + strings.Join(codes, ", ") + ")", typ: t.Ret}
}

// procLiteral: `-> { }`, `proc { }`, `lambda { }`.
func procLiteral(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LambdaNode:
		return true
	case *parser.CallNode:
		_, blk := n.Block.(*parser.BlockNode)
		return blk && n.Receiver == nil && n.Arguments == nil && (n.Name == "proc" || n.Name == "lambda")
	}
	return false
}

// adoptBind is ann with the bound self of a Proc value of the same shape (decision 231): Sorbet's
// `T.let(block, T.nilable(T.proc.void))` of a `T.proc.bind(Parser)` block drops the bind its type cannot carry, where
// rugaroo's Proc type takes self as a parameter.
func adoptBind(ann, val Type) Type {
	vf, ok := stripOpt(val).(TFunc)
	if !ok || vf.Self == nil {
		return ann
	}
	o, opt := ann.(TOpt)
	af, ok := stripOpt(ann).(TFunc)
	if !ok || af.Self != nil || len(af.Params) != len(vf.Params) || af.Rest != vf.Rest {
		return ann
	}
	af.Self = vf.Self
	if opt {
		o.Elem = af
		return o
	}
	return af
}

// derefCode is `(*p)` for a Proc pointer p, and x itself for `(&x)`: a func just boxed as a Proc, whose `(*(&x))`
// staticcheck's SA4001 rejects.
func derefCode(p string) string {
	if inner, ok := strings.CutPrefix(p, "(&"); ok && strings.HasSuffix(inner, ")") && balancedParens(inner[:len(inner)-1]) {
		return inner[:len(inner)-1]
	}
	return "(*" + p + ")"
}
