package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// concreteSelf is the class an escaped block's self is bound to at t: a
// struct method's `self` is the generic Self, but its class is what
// instance_eval/call/yield actually bind (decision 164).
func (f *fctx) concreteSelf(t Type) Type {
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		return TClass{C: f.owner}
	}
	return t
}

// blockSelf resolves the self type an escaped block carries as its leading Go
// param, from inference (decision 164): instance_eval records the receiver,
// Proc#call and yield the lexical self. It marks the block escaped.
func (f *fctx) blockSelf(n parser.Node) Type {
	b := f.blockSig
	if b.Self != nil && b.SelfPending == "" { // the signature's `[self: C]`
		return b.Self
	}
	if b.SelfPending == "" {
		b.SelfPending = selfKey(f.m)
		f.c.notePending(b.SelfPending)
	}
	t, how := f.c.pendingType(b.SelfPending, f.f)
	switch how {
	case pendTyped:
		b.Self = t
	case pendRound, pendGem: // a gem's stored block nothing in the program calls: any self (decision 172)
		b.Self = TAny{}
	case pendNone:
		ce := catchCompileError(func() {
			f.errorf(n, "the block &%s is stored but never called, so its self type is unknown (no instance_eval, call or yield gives it one; decision 164)", f.m.BlockParam)
		})
		ce.untyped = true
		panic(*ce)
	}
	return b.Self
}

// forwardIter passes the method's own block on to an iterator:
// `list.each(&block)` re-yields every value (or calls the closure with it).
func (f *fctx) forwardIter(n parser.Node, call string, yields []Type) {
	if f.blockSig == nil {
		f.errorf(n, "a block is forwarded from a method that takes none")
	}
	vars := make([]string, len(yields))
	for i := range yields {
		vars[i] = f.newTmp()
	}
	list := strings.Join(vars, ", ")
	args := list
	var tt TTuple
	if len(yields) == 1 {
		tt, _ = yields[0].(TTuple)
	}
	// A proc auto-splats a lone yielded tuple, like bindBlockParams for a literal block.
	switch {
	case len(f.blockSig.Params) > 1 && len(tt.Elems) == len(f.blockSig.Params):
		fields := make([]string, len(tt.Elems))
		for i := range fields {
			fields[i] = fmt.Sprintf("%s.F%d", list, i)
		}
		args = strings.Join(fields, ", ")
	case len(yields) != len(f.blockSig.Params):
		f.errorf(n, "the forwarded block takes %d values but %d are yielded", len(f.blockSig.Params), len(yields))
	default:
		conv := make([]string, len(vars)) // a Hash subclass's untyped keys into a block typed String
		for i, v := range vars {
			conv[i] = f.coerce(n, expr{code: v, typ: yields[i]}, f.blockSig.Params[i])
		}
		args = strings.Join(conv, ", ")
	}
	guard := !f.iterator && f.m != nil && f.m.Block != nil && (f.m.Block.Optional || f.m.Block.Untyped)
	if guard { // `generator.each(&block)` called without one (prime's Prime.each(20)): no block, so nothing is yielded (decision 410)
		f.emit("if blk != nil {")
	}
	f.emit("%s", rangeHead(list, call))
	if f.iterator {
		f.emit("\tif !yield(%s) {", args)
		f.emit("\t\treturn")
		f.emit("\t}")
	} else {
		f.emit("\tblk(%s)", args)
	}
	f.emit("}")
	if guard {
		f.emit("}")
	}
}

// forwardClosure passes the method's own block on to a closure-taking
// method: `list.map(&block)`.
func (f *fctx) forwardClosure(n parser.Node, sig *BlockSig, env map[string]Type) string {
	code := f.forwardClosureCode(n, sig, env)
	if code != "blk" && f.m != nil && f.m.Block != nil && (f.m.Block.Optional || f.m.Block.Untyped) {
		return "rbIfBlk(blk != nil, " + code + ")" // an adapter over a missing block is no block (bindata's `write(io, &block)`)
	}
	return code
}

func (f *fctx) forwardClosureCode(n parser.Node, sig *BlockSig, env map[string]Type) string {
	if f.iterator {
		f.errorf(n, "this method's block is an iterator (it yields); it can only be passed on to another iterator")
	}
	if sig.Rest != f.blockSig.Rest && f.lenient() { // `&task` typed func(any) where the callee takes (*untyped) -> untyped, or back
		return f.restBlockAdapter(n, sig, env)
	}
	if len(sig.Params) != len(f.blockSig.Params) {
		if f.lenient() {
			return f.procArityAdapter(sig, env)
		}
		f.errorf(n, "the forwarded block takes %d values but %d are passed", len(f.blockSig.Params), len(sig.Params))
	}
	for i, p := range sig.Params {
		unify(p, f.blockSig.Params[i], env)
	}
	if isVoid(sig.Ret) && !isNil(sig.Ret) && !isVoid(f.blockSig.Ret) && f.lenient() && sig.Self == nil && f.blockSig.Self == nil {
		// a value block where the callee's is void (concurrent-ruby's ns_execute passing &task to fallback_action): its
		// value dropped
		params := make([]string, len(sig.Params))
		args := make([]string, len(sig.Params))
		for i, p := range sig.Params {
			params[i] = fmt.Sprintf("p%d %s", i, f.c.goType(subst(p, env)))
			args[i] = f.coerce(nil, expr{code: fmt.Sprintf("p%d", i), typ: subst(p, env)}, f.blockSig.Params[i])
		}
		return f.adapterFunc(params, "blk("+strings.Join(args, ", ")+")", sig.Ret, f.blockSig.Ret)
	}
	if tv, ok := sig.Ret.(TVar); ok && env[tv.Name] == nil && isVoid(f.blockSig.Ret) && !isNil(f.blockSig.Ret) {
		// a void block where the callee's returns X (`return super if block_given?` in Homebrew's Cask#tap over
		// Kernel#tap): X is untyped, the block's value nil
		env[tv.Name] = TAny{}
		params := make([]string, len(sig.Params))
		args := make([]string, len(sig.Params))
		for i, p := range sig.Params {
			params[i] = fmt.Sprintf("p%d %s", i, f.c.goType(subst(p, env)))
			args[i] = fmt.Sprintf("p%d", i)
		}
		return f.adapterFunc(params, "blk("+strings.Join(args, ", ")+")", TAny{}, f.blockSig.Ret)
	}
	if sig.Self != nil && f.blockSig.Self != nil && !sig.Rest && !f.blockSig.Rest && !typeEq(subst(sig.Self, env), f.blockSig.Self) {
		return f.selfBlockAdapter(sig, env)
	}
	unify(sig.Ret, f.blockSig.Ret, env)
	return "blk"
}

// selfBlockAdapter passes this method's bound block (`T.proc.bind(Resource::Patch)`) on to a callee whose block binds
// a superclass (`T.proc.bind(Resource)`): Go func types are invariant, and the self the callee binds is this object,
// so the adapter asserts it back down (Homebrew's Resource::Patch#initialize; decision 290).
func (f *fctx) selfBlockAdapter(sig *BlockSig, env map[string]Type) string {
	self := subst(sig.Self, env)
	params := make([]string, 0, 1+len(sig.Params))
	params = append(params, "s "+f.c.goType(self))
	args := make([]string, 0, 1+len(sig.Params))
	args = append(args, fmt.Sprintf("rbAs[%s](s, %q)", f.c.goType(f.blockSig.Self), f.blockSig.Self.String()))
	for i, p := range sig.Params {
		pt := subst(p, env)
		params = append(params, fmt.Sprintf("p%d %s", i, f.c.goType(pt)))
		args = append(args, f.coerce(nil, expr{code: fmt.Sprintf("p%d", i), typ: pt}, f.blockSig.Params[i]))
	}
	call := "blk(" + strings.Join(args, ", ") + ")"
	return f.adapterFunc(params, call, f.adapterRet(sig, env, f.lenient()), f.blockSig.Ret)
}

// adapterRet is sig's result for an adapter; `x.tap(&block)`'s X is bound by nothing at a forwarded block, so untyped (when untypedVar).
func (f *fctx) adapterRet(sig *BlockSig, env map[string]Type, untypedVar bool) Type {
	ret := subst(sig.Ret, env)
	if tv, ok := ret.(TVar); ok && tv.Name != "Self" && untypedVar {
		return TAny{}
	}
	return ret
}

// adapterFunc wraps call of the forwarded block: value dropped for a void ret, a zero for a void block, else coerced from blkRet.
func (f *fctx) adapterFunc(params []string, call string, ret, blkRet Type) string {
	head := "func(" + strings.Join(params, ", ") + ")"
	switch {
	case isVoid(ret):
		return head + " { " + call + " }"
	case isVoid(blkRet):
		return head + " " + f.c.goType(ret) + " { " + call + "; return " + goZero(f.c.goType(ret)) + " }"
	}
	return head + " " + f.c.goType(ret) + " { return " + f.coerce(nil, expr{code: call, typ: blkRet}, ret) + " }"
}

// restBlockFunc is a Dyn wrapper's block (dynBlockArg) for a `(*untyped) -> untyped` block: already that func, nil
// for none.
func restBlockFunc(b *parser.BlockArgumentNode, sig *BlockSig) (string, bool) {
	en, ok := b.Expression.(*exprNode)
	if !ok || !anyRestBlock(sig) {
		return "", false
	}
	ft, ok := en.e.typ.(TFunc)
	return en.e.code, ok && !ft.Proc && ft.Rest
}

// restBlockAdapter passes the method's block where the callee's block differs in being variadic (gem code, decision
// 248): a variadic callee's values fill the block's parameters (missing ones nil), a variadic block takes the callee's.
func (f *fctx) restBlockAdapter(n parser.Node, sig *BlockSig, env map[string]Type) string {
	if sig.Self != nil || f.blockSig.Self != nil {
		f.errorf(n, "a forwarded block with a bound self and a rest parameter on one side only is not supported")
	}
	if env != nil { // an untyped proc binds nothing: what the callee's block names beyond the call's env is untyped (`target.to_h(&block)`'s [A, B]; decision 444)
		var vars []string
		for _, p := range sig.Params {
			freeVars(p, &vars)
		}
		freeVars(sig.Ret, &vars)
		for _, v := range vars {
			if _, ok := env[v]; !ok && v != "Self" {
				env[v] = TAny{}
			}
		}
	}
	var params, args []string
	if sig.Rest { // func(xs ...E) R calling blk(p0, p1)
		elem := subst(sig.Params[len(sig.Params)-1], env)
		params = append(params, "xs ..."+f.c.goType(elem))
		for i, want := range f.blockSig.Params {
			args = append(args, f.coerce(nil, expr{code: fmt.Sprintf("rbArgAt(xs, %d)", i), typ: elem}, want))
		}
	} else { // func(p0, p1) R calling blk(p0, p1) with blk variadic
		elem := f.blockSig.Params[len(f.blockSig.Params)-1]
		for i, p := range sig.Params {
			params = append(params, fmt.Sprintf("p%d %s", i, f.c.goType(subst(p, env))))
			args = append(args, f.coerce(nil, expr{code: fmt.Sprintf("p%d", i), typ: subst(p, env)}, elem))
		}
	}
	call := "blk(" + strings.Join(args, ", ") + ")"
	return f.adapterFunc(params, call, f.adapterRet(sig, env, true), f.blockSig.Ret) // its only caller is lenient
}

// procArityAdapter passes the method's block where the callee yields another count of values, as a Ruby proc
// tolerates: extra values dropped, missing ones nil (gem code, decision 172).
func (f *fctx) procArityAdapter(sig *BlockSig, env map[string]Type) string {
	var params, args []string
	switch { // `T.proc.bind(DSL)`: the self it is called with leads
	case sig.Self != nil && f.blockSig.Self != nil:
		params = append(params, "s_ "+f.c.goType(subst(sig.Self, env)))
		args = append(args, f.coerce(nil, expr{code: "s_", typ: subst(sig.Self, env)}, f.blockSig.Self))
	case sig.Self != nil: // a closure keeps its own self
		params = append(params, "_ "+f.c.goType(subst(sig.Self, env)))
	case f.blockSig.Self != nil:
		args = append(args, "*new("+f.c.goType(f.blockSig.Self)+")")
	}
	for i, p := range sig.Params {
		params = append(params, fmt.Sprintf("p%d %s", i, f.c.goType(subst(p, env))))
	}
	for i, want := range f.blockSig.Params {
		if i < len(sig.Params) {
			args = append(args, f.coerce(nil, expr{code: fmt.Sprintf("p%d", i), typ: subst(sig.Params[i], env)}, want))
		} else {
			args = append(args, "*new("+f.c.goType(want)+")")
		}
	}
	call := "blk(" + strings.Join(args, ", ") + ")"
	return f.adapterFunc(params, call, subst(sig.Ret, env), f.blockSig.Ret)
}
