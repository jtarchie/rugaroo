package compiler

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// probe runs gen without emitting, restoring the output/temp state after, even when gen panics with a compile error.
func (f *fctx) probe(fn func()) {
	savedBuf, savedTmp, savedCtr, savedIndent := f.buf, f.tmp, f.blockCtr, f.indent
	f.buf = &strings.Builder{}
	f.probeDepth++
	sc := f.scope
	var had map[string]*local
	if sc != nil {
		had = maps.Clone(sc.vars)
	}
	type declState struct {
		pass        int
		block, ruby string
		hoist       bool
	}
	decls := make(map[localKey]declState, len(f.locals))
	for k, li := range f.locals {
		decls[k] = declState{li.declPass, li.declBlock, li.declRuby, li.hoist}
	}
	// the nesting fn entered: balanced when it returns, left behind when a caller recovers its compile error (an untyped call's retry)
	frames, block, rbScope := len(f.rbFrames), f.block, f.rbScope
	closures, lambdaClosure, loops, begins, unionArm, retryFlag, retryDepth := f.closures, f.lambdaClosure, f.loops, f.begins, f.unionArm, f.retryFlag, f.retryDepth
	defer func() {
		f.probeDepth--
		f.buf, f.tmp, f.blockCtr, f.indent = savedBuf, savedTmp, savedCtr, savedIndent
		f.rbFrames, f.scope, f.block, f.rbScope = f.rbFrames[:frames], sc, block, rbScope
		f.closures, f.lambdaClosure, f.loops, f.begins, f.unionArm, f.retryFlag, f.retryDepth = closures, lambdaClosure, loops, begins, unionArm, retryFlag, retryDepth
		for k, li := range f.locals { // the discarded output declared no local: the real run must declare it (not find it "hoisted" from this pass)
			if was, ok := decls[k]; ok {
				li.declPass, li.declBlock, li.declRuby, li.hoist = was.pass, was.block, was.ruby, was.hoist
			} else {
				li.declPass = 0
			}
		}
		if sc != nil { // locals the discarded output declared (`x = begin v = 1 end`, `xs << (w = 5)`) are not declared yet
			for name := range sc.vars {
				if _, ok := had[name]; !ok {
					delete(sc.vars, name)
				}
			}
		}
	}()
	fn()
}

// probeType is the type n compiles to, without emitting it.
func (f *fctx) probeType(n parser.Node, expected Type) Type {
	var t Type
	f.probe(func() { t = f.genExpr(n, expected).typ })
	return t
}

// analyze runs the local-analysis pass and the emitting pass (decision 13).
// An unannotated `x = []`/`{}` is typed by the join of what later writes
// put in it (x << v, x.push(v), x[i] = v, h[k] = v), re-running pass 1
// until no more locals are typed: one typed literal can type the next.
// A typing that fails to compile or adds a conversion is dropped, and its
// literal stays untyped: an rbAs between instantiations copies, and the
// copy would lose Ruby's aliasing.
// typeTestsWidened reports a param widened since before (not in before) whose type the body tests (`b.is_a? Matrix`,
// `case b`): its old type may have folded a branch dead that its widened copy reaches, as lup's solve (decision 414).
func (f *fctx) typeTestsWidened(before map[string]Type) bool {
	if f.bodyStmts == nil {
		return false
	}
	return anyNode(f.bodyStmts, func(n parser.Node) bool {
		var r parser.Node
		switch x := n.(type) {
		case *parser.CallNode:
			if x.Name == "is_a?" || x.Name == "kind_of?" || x.Name == "instance_of?" {
				r = x.Receiver
			}
		case *parser.CaseNode:
			r = x.Predicate
		}
		lv, ok := r.(*parser.LocalVariableReadNode)
		return ok && f.paramWiden[lv.Name] != nil && before[lv.Name] == nil
	})
}

func (f *fctx) analyze(fresh func(), run func(int)) {
	nw := len(f.c.Warnings)
	if f.openArgs == nil {
		f.openArgs = map[int]localKey{} // before the snapshot: every run below shares it
	}
	saved := *f
	nRet := 0
	if f.retTypes != nil {
		nRet = len(*f.retTypes)
	}
	restore := func() { // the return evidence of a run that is redone (untyped, refined) must not join the next's
		*f = saved
		if f.retTypes != nil {
			*f.retTypes = (*f.retTypes)[:nRet]
		}
	}
	f.convs = map[int]bool{}
	orig := catchCompileError(func() { run(1) }) // `[].max` fails until the literal is typed
	for range 4 {                                // a reassigned gem param fails the rest of the run that widened it: again on its widened copy (decision 241)
		if len(f.paramWiden) == len(saved.paramWiden) || orig == nil && !f.typeTestsWidened(saved.paramWiden) {
			break
		}
		w := f.paramWiden
		restore()
		f.paramWiden, saved.paramWiden = w, w
		f.convs = map[int]bool{}
		fresh()
		orig = catchCompileError(func() { run(1) })
	}
	for range 8 { // --int-overflow=promote: again with each local a run typed Bignum promoted (decision 372)
		add := f.bignumLocals()
		if len(add) == 0 {
			break
		}
		p := maps.Clone(f.promoted)
		maps.Copy(p, add)
		restore()
		f.promoted, saved.promoted = p, p
		f.convs = map[int]bool{}
		fresh()
		orig = catchCompileError(func() { run(1) })
	}
	before := f.convs
	refined := map[localKey]Type{}
	var order []localKey
	for range 8 {
		add := f.emptyRefinements()
		maps.DeleteFunc(add, func(k localKey, t Type) bool { r, ok := refined[k]; return ok && typeEq(r, t) }) // a seeded local's same widening again
		if len(add) == 0 {
			break
		}
		for _, k := range slices.SortedFunc(maps.Keys(add), func(a, b localKey) int { return strings.Compare(a.name, b.name) }) {
			if _, ok := refined[k]; !ok {
				order = append(order, k)
			}
			refined[k] = add[k]
		}
		restore()
		f.refined, f.convs = refined, map[int]bool{}
		fresh()
		_ = catchCompileError(func() { run(1) }) // discovery only: try() decides
	}
	var proofErr *compileError // a typing the String alias proof refused: leaving it untyped only moves the error to run time
	try := func(set map[localKey]Type) bool {
		f.c.dropWarnings(nw) // the untyped run's, e.g. dynamic calls on elements
		restore()
		f.refined, f.convs = set, map[int]bool{}
		fresh()
		err := catchCompileError(func() { run(1) })
		if err == nil && !f.newConv(before) {
			err = catchCompileError(func() { run(2) })
			if err == nil {
				return true
			}
		}
		if err != nil && err.proof && proofErr == nil {
			proofErr = err
		}
		return false
	}
	if len(order) > 0 {
		ok := salvage(order, func(set map[localKey]bool) bool {
			sub := map[localKey]Type{}
			for k := range set {
				sub[k] = refined[k]
			}
			return len(sub) > 0 && try(sub)
		})
		if proofErr != nil { // `rows = ["a"]; rows.each { |r| r << "x" }`: Array[untyped] compiled a `<<` that raised NoMethodError
			panic(*proofErr)
		}
		if ok {
			return
		}
		f.c.dropWarnings(nw)
		restore()
		fresh()
		run(1)
	} else if orig != nil {
		panic(*orig)
	}
	run(2)
}

func (f *fctx) newConv(before map[int]bool) bool {
	for off := range f.convs {
		if !before[off] {
			return true
		}
	}
	return false
}

// tryClean is try that also drops what fn emitted when it fails: a skipped statement leaves no half-open block.
func (f *fctx) tryClean(fn func()) *compileError {
	out := f.buf
	f.buf = &strings.Builder{}
	err := f.try(fn)
	if err == nil {
		out.WriteString(f.buf.String())
	}
	f.buf = out
	return err
}

// try is catchCompileError for generation that goes on past the error:
// an error inside a block would leave its closure depth, buffer and scope
// behind, so f is restored (labels excepted: they stay unique).
func (f *fctx) try(fn func()) *compileError {
	saved := *f
	sc := f.scope
	var had map[string]*local
	if sc != nil {
		had = maps.Clone(sc.vars)
	}
	err := catchCompileError(fn)
	if err != nil {
		labels := f.labels
		*f = saved
		f.labels = labels
		if sc != nil { // a local the failed attempt declared is not declared (`push(*(sw = mk(o)))`, decision 244)
			for name := range sc.vars {
				if _, ok := had[name]; !ok {
					delete(sc.vars, name)
				}
			}
		}
	}
	return err
}

// catchCompileError runs fn, returning the compile error it raised, if any.
func catchCompileError(fn func()) (ce *compileError) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			ce = &e
		}
	}()
	fn()
	return nil
}

// emptyRefinements types each open, still untyped local by the join of
// what pass 1 saw put in it. No evidence, or evidence that is or holds
// untyped, leaves it untyped.
func (f *fctx) emptyRefinements() map[localKey]Type {
	out := map[localKey]Type{}
	// in key order: refineTemplate may request generic instances (decision 308), and their order orders main.go
	for _, k := range slices.SortedFunc(maps.Keys(f.locals), func(a, b localKey) int {
		return cmp.Or(cmp.Compare(a.scope, b.scope), strings.Compare(a.name, b.name))
	}) {
		li := f.locals[k]
		if li.seeded {
			if li.filled && slices.ContainsFunc(li.elems[0], holdsAny) { // a gem's Array.new(n, 0) given an untyped value holds any class
				if c, ok := stripOpt(li.typ).(TClass); ok {
					out[k] = TClass{C: c.C, Args: []Type{TAny{}}}
				}
				continue
			}
			if t := widenContainer(li.typ, li.elems); t != nil {
				out[k] = t
			}
			continue
		}
		if t := refineContainer(li.typ, li.open, li.elems); t != nil {
			out[k] = t
		}
		if r, ok := f.refined[k]; ok && li.open {
			if t := rewiden(r, li.elems); t != nil {
				out[k] = t
			}
		}
		if t := f.refineTemplate(li); t != nil {
			out[k] = t
		}
	}
	return out
}

// widenContainer is a non-empty literal's Array type joined with what was written to it, a union where they differ
// (decision 193); nil when untyped evidence leaves it undecided.
func widenContainer(t Type, elems [2][]Type) Type {
	c, ok := stripOpt(t).(TClass)
	if !ok || len(c.Args) == 0 || len(c.Args) > 2 {
		return nil
	}
	args := make([]Type, len(c.Args))
	for i := range args {
		typed := slices.DeleteFunc(slices.Clone(elems[i]), holdsAny) // as refineContainer: untyped evidence only because the local is (decision 384)
		if len(typed) == 0 {
			return nil
		}
		j := typed[0]
		for _, e := range typed[1:] {
			j = joinUnion(j, e)
		}
		if holdsAny(j) {
			return nil
		}
		args[i] = j
	}
	return TClass{C: c.C, Args: args}
}

// rewiden is an already refined `[]`/`{}` joined with what its refined run put in it, or nil when that adds nothing
// typed: `node, state = stack.pop` re-pushed as `[node, :close]` makes Array[[Node, Symbol]] Array[[Node?, Symbol]].
func rewiden(t Type, elems [2][]Type) Type {
	c, ok := t.(TClass)
	if !ok || len(c.Args) == 0 || len(c.Args) > 2 {
		return nil
	}
	args := slices.Clone(c.Args)
	for i := range args {
		for _, e := range elems[i] {
			if holdsAny(e) {
				continue
			}
			j, ok := join(args[i], e)
			if !ok || holdsAny(j) || isUnion(j) && !isUnion(args[i]) {
				return nil
			}
			args[i] = j
		}
	}
	if slices.EqualFunc(args, c.Args, typeEq) {
		return nil
	}
	return TClass{C: c.C, Args: args}
}

// refineContainer is the type an open `[]`/`{}` of type t gets from what
// was put in it, or nil: still typed, or no typed evidence to join.
// Untyped evidence is skipped (`h[k] = h[k] + 1` is untyped only because h
// is): if it really is untyped, the typed run converts it and is rejected.
func refineContainer(t Type, open bool, elems [2][]Type) Type {
	c, ok := stripOpt(t).(TClass)
	if !open || !ok || !slices.ContainsFunc(c.Args, isAny) {
		return nil
	}
	args := make([]Type, len(c.Args))
	for i := range args {
		if !isAny(c.Args[i]) { // `Hash.new(0)`: V is the default's, only K is open (decision 344)
			args[i] = c.Args[i]
			continue
		}
		typed := slices.DeleteFunc(slices.Clone(elems[i]), holdsAny)
		if args[i] = joinOrAny(typed); len(typed) == 0 || holdsAny(args[i]) {
			return nil
		}
	}
	if isOpt(t) { // a container ivar a gem resets to nil stays optional (decision 410)
		return TOpt{Elem: TClass{C: c.C, Args: args}}
	}
	return TClass{C: c.C, Args: args}
}
