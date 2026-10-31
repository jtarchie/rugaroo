package compiler

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// forwarderEnv is the signature environment for a forwarder of cls.
func (c *Compiler) forwarderEnv(cls *Class, e entry) map[string]Type {
	env := composeEnv(e.Env, nil)
	env["Self"] = c.selfTypeFor(e, cls)
	cls.containerEnv(env)
	return env
}

// emitForwarders emits, for a concrete class, a Go method per inherited or
// included public non-generic method so the class satisfies its interface.
func (c *Compiler) emitForwarders(cls *Class) {
	if cls.IsModule || cls.universal {
		return
	}
	recv := c.recvType(cls)
	for _, e := range c.publicEntries(cls) {
		m := e.M
		if e.Owner == cls && c.isDirectMethod(m) {
			continue
		}
		if cls.hashBase && e.Owner != cls && slices.Contains(cls.Super.ancestors(), e.Owner) {
			continue // promoted from the embedded Hash; a typed call is intercepted in callCode
		}
		if m.Kind == kindAttrReader || m.Kind == kindAttrWriter {
			continue // promoted through embedding; never touches virtual self
		}
		if !c.wantsForwarder(cls, e) {
			continue
		}
		env := c.forwarderEnv(cls, e)
		ps, ret := c.sig(m, env)
		targs := c.forwardTypeArgs(e, cls)
		body := flatLeafOpt(fmt.Sprintf("%s%s(self%s)", freeFuncName(m), targs, comma(c.argNames(m))), m.Ret, env)
		if ret == "" {
			c.w("func (self %s) %s(%s) { %s }\n", recv, m.GoName, ps, body)
		} else {
			c.w("func (self %s) %s(%s) %s { return %s }\n", recv, m.GoName, ps, ret, body)
		}
	}
	c.emitBridges(cls, recv)
	for _, e := range c.publicEntries(cls) {
		if e.M.seqRest {
			env := c.forwarderEnv(cls, e)
			name, ps, ret := c.seqAdapterSig(e.M, env)
			c.w("func (self %s) %s(%s) %s { return self.%s(%s) }\n", recv, name, ps, ret, e.M.GoName, c.seqArgs(e.M))
			continue
		}
		if !e.M.seqAdapter {
			continue
		}
		env := c.forwarderEnv(cls, e)
		name, ps, ret := c.seqAdapterSig(e.M, env)
		if c.slotSeqAdapter(cls, e, env, recv) {
			continue
		}
		seq := "rbSeq"
		if len(e.M.Block.Params) == 2 {
			seq = "rbSeq2"
		}
		if !isVoid(e.M.Block.Ret) && len(e.M.Block.Params) == 1 { // `each { |x| ... }` whose block value is ignored: drop it
			elem := c.goType(subst(e.M.Block.Params[0], env))
			args := c.seqArgs(e.M)
			wrapped := c.ignoredBlockValue(e.M, elem, env)
			args = strings.Replace(args, "blk", wrapped, 1)
			if !strings.Contains(c.argNames(e.M), "blk") {
				args = strings.TrimPrefix(args+", "+wrapped, ", ")
			}
			c.w("func (self %s) %s(%s) %s {\n\treturn %s(func(blk func(%s)) { %s })\n}\n",
				recv, name, ps, ret, seq, elem, c.blkCall(cls, e, args))
			continue
		}
		c.w("func (self %s) %s(%s) %s {\n\treturn %s(func(blk %s) { %s })\n}\n",
			recv, name, ps, ret, seq, c.blockGoType(e.M.Block, env), c.blkCall(cls, e, c.seqArgs(e.M)))
	}
	for _, e := range c.publicEntries(cls) {
		for _, s := range e.M.shadowed {
			ps, ret := c.sig(s.e.M, c.slotEnv(s))
			c.w("func (self %s) %s(%s) %s {\n%s}\n", recv, s.e.M.GoName, ps, ret, c.adapterBody(cls, e, s))
		}
	}
	if c.includerCalls(cls, "class") {
		// a module's `self.class`; a class object's class is Class (a module's, Module)
		k := cls
		switch {
		case cls.metaOf != nil && cls.metaOf.IsModule:
			k = c.classes["Module"]
		case cls.metaOf != nil:
			k = c.classes["Class"]
		}
		c.w("func (self %s) _ClassObj() %s { return %s }\n", recv, c.goType(TClass{C: c.classes["Class"]}), classVar(k))
	}
	c.w("\n")
	c.emitEqAdapter(cls, recv)
	c.emitCmpAdapter(cls, recv)
}

// emitCmpAdapter: Comparable_Self and rbCmp need Op_cmp(T) Integer, so a nil <=> (Float's NaN) raises there, as MRI's rb_cmpint.
func (c *Compiler) emitCmpAdapter(cls *Class, recv string) {
	e := cls.lookup("<=>")
	if e == nil || e.M.GoName != "cmpNil" || len(e.M.Params) != 1 || !c.wantsForwarder(cls, *e) {
		return
	}
	arg := c.argNames(e.M)
	// Comparable's slot is `Op_cmp(Self)`; a wider parameter (Sorbet's `other: BasicObject`) still takes a Self
	call := "self.cmpNil(" + arg + ")"
	if strings.Contains(recv, "[") {
		// a generic type's cmpNil forwarder is pruned unless an interface declares the name (isGenericForwarder), and none does: call the free func
		call = freeFuncName(e.M) + c.forwardTypeArgs(*e, cls) + "(self, " + arg + ")"
	}
	c.w("func (self %s) Op_cmp(%s %s) Integer {\n\tif r := %s; r.ok {\n\t\treturn r.v\n\t}\n\tpanic(rbCmpErr(self, %s))\n}\n\n",
		recv, arg, c.goType(c.cmpIncluder(cls)), call, arg)
}

// cmpIncluder is the type the Op_cmp adapter takes for cls: the topmost superclass that has Comparable (whose Self
// binds there) or the adapter itself, so each subclass answers the one Op_cmp slot.
func (c *Compiler) cmpIncluder(cls *Class) Type {
	cmp := c.classes["Comparable"]
	k := cls
	for sup := k.Super; sup != nil && !sup.universal; sup = k.Super {
		if e := sup.lookup("<=>"); !slices.Contains(sup.ancestors(), cmp) && (e == nil || e.M.GoName != "cmpNil") {
			break
		}
		k = sup
	}
	if k.metaOf != nil || !k.isStruct() {
		return c.selfTypeFor(entry{Owner: k, Entry: k}, k)
	}
	return TClass{C: k}
}

// emitEqAdapter lets rbEq (include?, Array#==, == on untyped or T?) reach
// a == typed on its argument, Op_eq(VecI), which Go cannot also declare as
// Op_eq(any): _EqAny asserts the class and answers false for anything else.
func (c *Compiler) emitEqAdapter(cls *Class, recv string) {
	e := cls.lookup("==")
	if !cls.isStruct() || e == nil || e.M.Private || e.M.generic() || len(e.M.Params) != 1 {
		return
	}
	env := composeEnv(e.Env, nil)
	env["Self"] = c.selfTypeFor(*e, cls)
	t, ok := subst(e.M.Params[0].Type, env).(TClass)
	r, isCls := subst(e.M.Ret, env).(TClass)
	if !ok || !isCls || r.C != c.classes["Boolean"] || c.goType(t) == "any" {
		return // ponytail: T? or union params keep identity; assert via rbOptArg to widen
	}
	c.w("func (self %s) _EqAny(o any) Boolean {\n\tif o, ok := o.(%s); ok {\n\t\treturn self.%s(o)\n\t}\n\treturn false\n}\n\n",
		recv, c.goType(t), e.M.GoName)
}

// slotEnv binds the type variables of slot s's signature, Self included.
func (c *Compiler) slotEnv(s slot) map[string]Type {
	env := composeEnv(s.e.Env, nil)
	env["Self"] = c.selfTypeFor(s.e, s.in)
	s.in.containerEnv(env) // a Set subclass's slot of Set's own: E is untyped
	return env
}

// slotKey is slot s's Go signature without parameter names.
func (c *Compiler) slotKey(s slot) string {
	m := *s.e.M
	m.Params = make([]Param, len(s.e.M.Params))
	for i, p := range s.e.M.Params {
		m.Params[i] = Param{Name: "_", Type: p.Type, Rest: p.Rest}
	}
	ps, ret := c.sig(&m, c.slotEnv(s))
	return "(" + ps + ") " + ret
}

// sigText renders slot s's signature, block aside, in RBS for messages.
func (c *Compiler) sigText(s slot) string {
	env := c.slotEnv(s)
	ps := make([]string, len(s.e.M.Params))
	for i, p := range s.e.M.Params {
		ps[i] = subst(p.Type, env).String()
		if p.Rest {
			ps[i] = "*" + ps[i]
		}
	}
	ret := "untyped" // a return inferred later (decision 36)
	if s.e.M.Ret != nil {
		ret = subst(s.e.M.Ret, env).String()
	}
	return "(" + strings.Join(ps, ", ") + ") -> " + ret
}

// arity is the argument count range m takes; max is -1 with a rest param.
func arity(m *Method) (req, maxArgs int) {
	for _, p := range m.Params {
		switch {
		case p.Rest:
			return req, -1
		case p.Default == nil:
			req++
		}
		maxArgs++
	}
	return req, maxArgs
}

// paramAt is the parameter taking positional argument i.
func paramAt(m *Method, i int) Param {
	for j, p := range m.Params {
		if p.Rest || j == i {
			return p
		}
	}
	panic("paramAt: out of range")
}

// adapts reports whether an adapter can hand a from where a to is expected:
// the same Go type, through untyped, or up a struct hierarchy (down, with a
// type assertion, for arguments).
func (c *Compiler) adapts(from, to Type, arg bool) bool {
	switch {
	case isVoid(to) && !arg:
		return true
	case isNil(from) && holdsNil(to) && !arg: // `-> nil` over a nilable result
		return true
	case !arg && isOpt(to) && !isOpt(from) && c.adapts(from, to.(TOpt).Elem, false): // `-> T` over `-> T?`
		return true
	case !arg && isOpt(to) && isOpt(from) && c.adapts(from.(TOpt).Elem, to.(TOpt).Elem, false): // `-> Sub?` over `-> Parent?` (decision 404)
		return true
	case !arg && widerResult(from, to): // `-> Array[Sub]` over `-> Array[Parent]`: the adapter returns a converted copy (decision 404)
		return true
	case isVoid(from):
		return false
	case c.goType(from) == c.goType(to) || isAny(to) || c.goType(to) == "any": // a union is Go any: any value boxes into it
		return true
	case isAny(from):
		_, ok := to.(TClass)
		_, opt := to.(TOpt)
		return ok || opt
	}
	fc, ok1 := from.(TClass)
	tc, ok2 := to.(TClass)
	if !ok1 || !ok2 || !fc.C.isStruct() || !tc.C.isStruct() {
		return false
	}
	return fc.C.isSubclassOf(tc.C) || arg && tc.C.isSubclassOf(fc.C)
}

// widerResult reports a converting result (Array, Hash) of a narrower instantiation than to's, as coerce converts it.
func widerResult(from, to Type) bool {
	tc, ok := to.(TClass)
	return ok && converts(tc) && widerInstance(from, tc)
}

// checkAdaptable rejects an override whose class cannot answer slot s of an
// ancestor's interface through an adapter.
func (c *Compiler) checkAdaptable(own, s slot) {
	m, sm := own.e.M, s.e.M
	fail := func(why string) {
		c.errorAt(m.srcPos, "%s: %s overrides %s: %s, but %s; give it the parent's signature or another name",
			m, c.sigText(own), sm, c.sigText(s), why)
	}
	if (m.Block != nil || sm.Block != nil) && !c.sameIterator(own, s) && !ignoresSlotBlock(m, sm) {
		fail("a block-taking override must keep the signature")
	}
	if restSlot(own.e.M, sm) { // Object#to_json(*args) answered by `to_json(options = nil)`: by argument count
		elem := subst(sm.Params[0].Type, c.slotEnv(s))
		for _, p := range m.Params {
			if to := subst(p.Type, c.slotEnv(own)); !c.adapts(elem, to, true) {
				fail(fmt.Sprintf("its rest elements cannot pass a %s as a %s", elem, to))
			}
		}
		from, to := subst(m.Ret, c.slotEnv(own)), subst(sm.Ret, c.slotEnv(s))
		if !c.adapts(from, to, false) {
			fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
		}
		return
	}
	for _, p := range sm.Params {
		if p.Rest {
			fail("an override of a rest-parameter method must keep the signature")
		}
	}
	if kwSlot(m, sm) {
		c.checkKwAdaptable(own, s, fail)
		return
	}
	req, maxArgs := arity(m)
	if n := len(sm.Params); n < req || maxArgs >= 0 && n > maxArgs {
		return // the adapter raises ArgumentError, as MRI would (or checks the count passed, when the slot counts it: fewerParams)
	}
	env, oenv := c.slotEnv(s), c.slotEnv(own)
	// a gem's Hash[String, untyped] over Hash[String, String]: the adapter converts a copy (rbAs, decision 172)
	gemConv := func(from, to Type) bool {
		tc, ok := to.(TClass)
		return c.gemFile(m.File) && ok && converts(tc) && sameButUntyped(from, to)
	}
	for i, p := range sm.Params {
		from, to := subst(p.Type, env), subst(paramAt(m, i).Type, oenv)
		if !c.adapts(from, to, true) && !gemConv(from, to) {
			fail(fmt.Sprintf("argument %d cannot pass a %s as a %s", i+1, from, to))
		}
	}
	from, to := subst(m.Ret, oenv), subst(sm.Ret, env)
	if !c.adapts(from, to, false) && !gemConv(from, to) {
		fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
	}
}

// kwSlot reports an override and a slot that both take keywords: the adapter passes them by name (decision 241).
func kwSlot(m, sm *Method) bool {
	kw := func(p Param) bool { return p.Keyword || p.KwRest }
	return slices.ContainsFunc(m.Params, kw) && slices.ContainsFunc(sm.Params, kw) && m.Block == nil && sm.Block == nil
}

// ignoresSlotBlock reports an override without a block over a slot whose block is optional: as MRI's, a block passed to
// a method that never yields is ignored (BlockNode#lambda? over Node's matcher).
func ignoresSlotBlock(m, sm *Method) bool {
	return m.Block == nil && sm.Block != nil && sm.Block.Optional && !sm.Iterator && !m.Iterator
}

// fewerParams reports an override taking fewer positional parameters than a slot that counts what a call passed
// (rbArgc, decision 8): the adapter checks that count and passes the leading ones; the slot's keywords are not passed
// on. ponytail: keywords a call did pass should raise ArgumentError, as MRI's; they are dropped.
func fewerParams(m, sm *Method) bool {
	if !sm.calleeDefaults || m.Block != nil || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest || p.Post }) {
		return false
	}
	_, maxArgs := arity(m)
	return maxArgs >= 0 && len(sm.Params) > maxArgs
}

// positionals is m's leading positional parameters (no rest, keywords or post).
func positionals(m *Method) []Param {
	var out []Param
	for _, p := range m.Params {
		if p.Rest || p.Keyword || p.KwRest || p.Post {
			break
		}
		out = append(out, p)
	}
	return out
}

// checkKwAdaptable checks a keyword slot: the positionals pair up by place, the slot's keywords the override takes by
// name; one the override lacks goes to its **rest, or the slot cannot be answered; an override keyword the slot lacks
// comes from the slot's **rest, or is defaulted (decision 241).
func (c *Compiler) checkKwAdaptable(own, s slot, fail func(string)) {
	m, sm := own.e.M, s.e.M
	env, oenv := c.slotEnv(s), c.slotEnv(own)
	pos, spos := positionals(m), positionals(sm)
	if len(spos) > len(pos) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Post }) {
		fail("its positional parameters do not line up")
	}
	for i, p := range spos {
		if from, to := subst(p.Type, env), subst(pos[i].Type, oenv); !c.adapts(from, to, true) {
			fail(fmt.Sprintf("argument %d cannot pass a %s as a %s", i+1, from, to))
		}
	}
	for i := len(spos); i < len(pos); i++ {
		if pos[i].Default == nil {
			fail(fmt.Sprintf("its argument %d is required", i+1))
		}
	}
	ownRest := slices.ContainsFunc(m.Params, func(p Param) bool { return p.KwRest })
	for _, p := range sm.Params {
		if !p.Keyword || p.KwRest {
			continue
		}
		i := slices.IndexFunc(m.Params, func(q Param) bool { return q.Keyword && !q.KwRest && q.Name == p.Name })
		switch {
		case i >= 0:
			if from, to := subst(p.Type, env), subst(m.Params[i].Type, oenv); !c.adapts(from, to, true) && !c.gemFile(m.File) {
				fail(fmt.Sprintf("keyword %s cannot pass a %s as a %s", p.Name, from, to))
			}
		case !ownRest:
			fail("it takes no keyword " + p.Name)
		}
	}
	from, to := subst(m.Ret, oenv), subst(sm.Ret, env)
	if !c.adapts(from, to, false) && !c.gemFile(m.File) {
		fail(fmt.Sprintf("its %s result cannot stand in for %s", from, to))
	}
}

// kwAdapterArgs is a keyword slot's call of the override: the slot's positionals, its keywords by name, and its
// **rest splatted, from which the override's own keywords are taken (decision 241).
func kwAdapterArgs(sm *Method, env map[string]Type) []parser.Node {
	var args []parser.Node
	kh := &parser.KeywordHashNode{}
	for i, p := range sm.Params {
		a := &exprNode{e: expr{code: goParamName(p.Name, i), typ: subst(p.Type, env)}}
		switch {
		case p.KwRest:
			kh.Elements = append(kh.Elements, &parser.AssocSplatNode{Value: a})
		case p.Keyword:
			kh.Elements = append(kh.Elements, &parser.AssocNode{Key: &parser.SymbolNode{Unescaped: parser.RubyString{Value: p.Name}}, Value: a})
		default:
			args = append(args, a)
		}
	}
	if len(kh.Elements) > 0 {
		args = append(args, kh)
	}
	return args
}

// seqDropped counts the trailing nil-defaulted params an `each` seq adapter leaves out, so it answers Enumerable's
// `each()` slot (concurrent-ruby's `each(head = nil)`); it passes nil for them. When the callee runs its own defaults
// (rbArgc), every trailing optional param is left out and runs its default (prime's `each(ubound = nil, generator =
// EratosthenesGenerator.new)`, decision 410).
func seqDropped(m *Method) int {
	if m.Name != "each" {
		return 0
	}
	k := 0
	for i := len(m.Params) - 1; i >= 0; i-- {
		p := m.Params[i]
		if p.Rest && i == len(m.Params)-1 && (m.seqAdapter || m.seqRest) { // `each(*args, &block)` forwarding to a container (regexp_parser's Subexpression): the slot passes none
			k++
			continue
		}
		if p.Keyword || p.Rest || p.Default == nil {
			break
		}
		if _, isNil := p.Default.(*parser.NilNode); !isNil && !m.calleeDefaults {
			if _, ok := goLiteralDefault(p.Default); !ok && !emptyContainerDefault(p.Default) { // `each(which = :all)` (matrix's): a literal is passed as written; `each(opts = {})` (regexp_parser's MatchLength) a fresh container
				break
			}
		}
		k++
	}
	return k
}

// seqArgs is argNames(m) with the params seqDropped leaves out passed as nil: a value-optional's zero (`ubound = nil`
// that a call typed Integer is an rbOpt[Integer], prime's each, decision 410), else Go's nil.
func (c *Compiler) seqArgs(m *Method) string {
	as := strings.Split(c.argNames(m), ", ")
	dropped := seqDropped(m)
	for _, p := range m.Params[len(m.Params)-dropped:] {
		if p.Rest { // a dropped rest: the Go variadic gets no arguments
			as = slices.DeleteFunc(as, func(a string) bool { return a == "rest_..." })
			continue
		}
		if i := slices.Index(as, goLocalName(p.Name)); i >= 0 {
			as[i] = "nil"
			if t := c.goType(p.Type); m.calleeDefaults {
				as[i] = "rbZero[" + t + "]()" // the callee runs the default: rbArgc says it was not passed
			} else if lit, ok := goLiteralDefault(p.Default); ok {
				as[i] = lit
				if strings.HasPrefix(t, "*") {
					as[i] = "Ref(" + lit + ")" // `which = :all` typed Symbol?
				}
			} else if emptyContainerDefault(p.Default) {
				as[i] = emptyContainerCode(p.Default, t)
			} else if strings.HasPrefix(t, "rbOpt[") {
				as[i] = t + "{}"
			}
		}
	}
	if m.calleeDefaults && len(as) > 0 && as[0] == "rbArgc" {
		as[0] = strconv.Itoa(len(m.Params) - dropped)
	}
	return strings.Join(as, ", ")
}

// sameIterator reports two iterators yielding the same Go types: an adapter returns the override's sequence.
func (c *Compiler) sameIterator(own, s slot) bool {
	m, sm := own.e.M, s.e.M
	if !m.Iterator || !sm.Iterator || len(m.Block.Params) != len(sm.Block.Params) {
		return false
	}
	if c.iterGoType(m.Block, c.slotEnv(own)) == c.iterGoType(sm.Block, c.slotEnv(s)) {
		return true
	}
	return len(sm.Block.Params) == 1 && c.goType(subst(sm.Block.Params[0], c.slotEnv(s))) == "any" // elements boxed (iterAdapterBody)
}

// iterAdapterBody answers an iterator slot whose elements are untyped with an override yielding typed ones.
func (c *Compiler) iterAdapterBody(cls *Class, e entry, s slot) string {
	env, oenv := c.slotEnv(s), c.forwarderEnv(cls, e)
	args := make([]string, 0, len(e.M.Params))
	for i, p := range e.M.Params {
		if i >= len(s.e.M.Params) { // `each(*arg)` answering `each`: the slot passes nothing for the rest (ruby/spec's EachCounter)
			if p.Rest {
				continue
			}
			panic(compileError{msg: fmt.Sprintf("%s#%s takes %d parameters where %s#%s's iterator takes %d", cls.RubyName, e.M.Name, len(e.M.Params), s.e.Owner.RubyName, s.e.M.Name, len(s.e.M.Params))})
		}
		from, to := c.goType(subst(s.e.M.Params[i].Type, env)), c.goType(subst(p.Type, oenv))
		a := goParamName(s.e.M.Params[i].Name, i)
		if from != to && to != "any" {
			a = fmt.Sprintf("rbAs[%s](any(%s), %q)", to, a, to)
		}
		args = append(args, a)
	}
	elem, x := "any", "x"
	if len(s.e.M.Block.Params) == 1 && len(e.M.Block.Params) == 1 { // the slot's element type: typed when the parent's yields are
		elem = c.goType(subst(s.e.M.Block.Params[0], env))
		if own := c.goType(subst(e.M.Block.Params[0], oenv)); own != elem && elem != "any" {
			x = fmt.Sprintf("rbAs[%s](any(x), %q)", elem, elem)
		}
	}
	return fmt.Sprintf("\treturn func(y func(%s) bool) {\n\t\tfor x := range %s {\n\t\t\tif !y(%s) {\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n",
		elem, c.blkCall(cls, e, strings.Join(args, ", ")), x)
}

// adapterBody answers slot s of an ancestor's interface with cls's method e
// (checked by checkAdaptable): an argument count e cannot take raises MRI's
// ArgumentError, anything else converts across the two signatures.
func (c *Compiler) adapterBody(cls *Class, e entry, s slot) string {
	if s.dyn {
		return c.dynAdapterBody(cls, e, s)
	}
	file := cls.File
	if e.M.File != nil {
		file = e.M.File
	}
	f := c.newFctx(file, cls, nil)
	f.lex = e.M.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	f.indent = 1
	if restSlot(e.M, s.e.M) {
		return c.restAdapterBody(f, cls, e, s)
	}
	if e.M.Iterator && s.e.M.Iterator {
		return c.iterAdapterBody(cls, e, s)
	}
	if kwSlot(e.M, s.e.M) {
		env := c.slotEnv(s)
		recv := expr{code: "self", typ: TClass{C: cls}}
		res := f.callEntry(&parser.NilNode{}, &e, recv, kwAdapterArgs(s.e.M, env), nil)
		ret := subst(s.e.M.Ret, env)
		switch {
		case !isVoid(ret):
			f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
		case res.code != "":
			f.emit("%s", res.code)
		}
		return f.buf.String()
	}
	n := len(s.e.M.Params)
	if req, maxArgs := arity(e.M); n < req || maxArgs >= 0 && n > maxArgs {
		if fewerParams(e.M, s.e.M) { // `def lambda?` over a matcher's `lambda?(node = self, **defaults, &block)`: what the call passed decides (rubocop-ast)
			f.emit("rbArity(rbArgc, %d, %d)", req, maxArgs)
			n = maxArgs
		} else {
			f.emit("rbArity(%d, %d, %d)\n\tpanic(\"unreachable\")", n, req, maxArgs)
			return f.buf.String()
		}
	}
	recv := expr{code: "self", typ: TClass{C: cls}}
	env, oenv := c.slotEnv(s), composeEnv(e.Env, nil)
	oenv["Self"] = recv.typ
	args := make([]parser.Node, n)
	for i, p := range s.e.M.Params[:n] {
		a := expr{code: goParamName(p.Name, i), typ: subst(p.Type, env)}
		to := subst(paramAt(e.M, i).Type, oenv)
		if tc, ok := to.(TClass); ok && tc.C.isStruct() && !c.adapts(a.typ, to, false) {
			a = expr{code: a.code + ".(" + c.goType(to) + ")", typ: to} // narrower: assert
		}
		args[i] = &exprNode{e: a}
	}
	res := f.callEntry(&parser.NilNode{}, &e, recv, args, nil)
	ret := subst(s.e.M.Ret, env)
	switch {
	case !isVoid(ret):
		f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
	case res.code != "":
		f.emit("%s", res.code)
	}
	return f.buf.String()
}

// restSlot reports a slot of only `*args` that m, with positional parameters alone, answers by argument count.
func restSlot(m, sm *Method) bool {
	if len(sm.Params) != 1 || !sm.Params[0].Rest || m.Block != nil || sm.Block != nil || m.kwMask() || m.calleeDefaults {
		return false
	}
	return !slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest || p.Post })
}

// restAdapterBody calls m with as many of rest_'s elements as were passed, or raises MRI's ArgumentError.
func (c *Compiler) restAdapterBody(f *fctx, cls *Class, e entry, s slot) string {
	recv := expr{code: "self", typ: TClass{C: cls}}
	env := c.slotEnv(s)
	elem := subst(s.e.M.Params[0].Type, env)
	ret := subst(s.e.M.Ret, env)
	req, maxArgs := arity(e.M)
	for k := req; k <= maxArgs; k++ {
		args := make([]parser.Node, k)
		for i := range k {
			args[i] = &exprNode{e: expr{code: fmt.Sprintf("rest_[%d]", i), typ: elem}}
		}
		f.emit("if len(rest_) == %d {", k)
		f.indent++
		res := f.callEntry(&parser.NilNode{}, &e, recv, args, nil)
		switch {
		case !isVoid(ret):
			f.emit("return %s", f.coerce(&parser.NilNode{}, res, ret))
		case res.code != "":
			f.emit("%s", res.code)
			f.emit("return")
		default:
			f.emit("return")
		}
		f.indent--
		f.emit("}")
	}
	f.emit("rbArity(len(rest_), %d, %d)\n\tpanic(\"unreachable\")", req, maxArgs)
	return f.buf.String()
}

// seqAdapterSig renders the iter.Seq adapter a closure override of an
// iterator answers to under the iterator's Go name (decision 4).
func (c *Compiler) seqAdapterSig(m *Method, env map[string]Type) (name, params, ret string) {
	it := *m
	it.Iterator = true
	it.Params = m.Params[:len(m.Params)-seqDropped(m)]
	it.calleeDefaults = m.calleeDefaults && slices.ContainsFunc(it.Params, func(p Param) bool { return p.Default != nil }) // the adapter passes the count, its callers do not
	params, ret = c.sig(&it, env)
	return goMethodName(m.Name), params, ret
}

// sigMentions reports whether m's parameters, block or return name cls, which
// the free func types as Self: bound to *C, they would not match the
// forwarder's CI (a `self?` return is **C where *CI is wanted).
// ponytail: matches by rendered name, so a same-named class elsewhere also opts out; walk the Type if that costs a hot method.
func sigMentions(m *Method, cls *Class) bool {
	sig := fmt.Sprint(m.Params, m.Block, m.Ret)
	return strings.Contains(sig, cls.RubyName) || strings.Contains(sig, "Self")
}

// forwardTypeArgs renders explicit type args for a forwarder call.
func (c *Compiler) forwardTypeArgs(e entry, cls *Class) string {
	var args []string
	switch {
	case e.Owner.GoType != "", c.plainSelf(e.M):
	case e.Owner == cls && cls.isStruct() && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls):
		// the class's own method: Self is *C, so the body's self._C() and
		// self-calls are direct calls Go can inline, not interface calls
		args = append(args, c.recvType(cls))
	case cls.isStruct() && e.Owner.universal && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls):
		args = append(args, "any") // Self is any: one instantiation for every class, where *C was a dictionary wrapper per class (decision 208)
	case cls.isStruct() && e.Owner.IsModule && !e.Owner.universal && len(cls.TypeParams) == 0 && !sigMentions(e.M, cls) && !c.constraintNamesSelf(e.Owner):
		// the constraint itself, one instantiation for every includer; an includer's own CI is a GC shape per class (decision 208)
		tps := make([]string, 0, len(e.Owner.TypeParams))
		for _, p := range e.Owner.TypeParams {
			tps = append(tps, c.goType(e.Env[p]))
		}
		args = append(args, e.Owner.selfName()+"[any"+comma(strings.Join(tps, ", "))+"]")
	case cls.isStruct() && !e.Owner.universal:
		// Self matches the forwarder's `self` (CI), else *C fails Comparable_Self[*C] and `(self)` args don't pass
		args = append(args, c.goType(c.selfTypeFor(e, cls)))
	default:
		args = append(args, c.recvType(cls))
	}
	for _, p := range e.Owner.TypeParams {
		args = append(args, c.goType(e.Env[p]))
	}
	if len(args) == 0 {
		return ""
	}
	return "[" + strings.Join(args, ", ") + "]"
}

// constraintNamesSelf reports a module whose Self constraint has a method typed by Self (Comparable_Self's
// Op_cmp(other Self)): only the includer's own interface satisfies it.
func (c *Compiler) constraintNamesSelf(owner *Class) bool {
	if v, ok := c.selfTyped[owner]; ok {
		return v
	}
	pending := c.pendingSigs
	saved := c.out
	c.out = strings.Builder{}    // ponytail: renders the constraint to read it; saved restores into the same address, so Builder's copy check holds
	c.renderingConstraint = true // every self-called name, dispatched or not: dispatchable asks this, so it must not ask back
	c.emitModuleInterface(owner)
	c.renderingConstraint = false
	decl := c.out.String()
	c.out = saved
	_, body, _ := strings.Cut(decl, "_Self[Self any")
	_, body, _ = strings.Cut(body, "{")
	v := selfWord.MatchString(body)
	if c.pendingSigs == pending { // a return still being inferred (`wait`'s `self`, asked about from inside wait) may name Self yet
		c.selfTyped[owner] = v
	}
	return v
}

var selfWord = regexp.MustCompile(`\bSelf\b`)

// blkCall calls the closure form of a seq-adapted method: a Go method, or on a @go_type subclass (whose own methods
// are free funcs, decision 202) its free func.
func (c *Compiler) blkCall(cls *Class, e entry, args string) string {
	if cls.GoType != "" && e.Owner == cls && !c.isDirectMethod(e.M) {
		return freeFuncName(e.M) + c.forwardTypeArgs(e, cls) + "(self" + comma(args) + ")"
	}
	return "self." + e.M.GoName + "(" + args + ")"
}

// slotSeqAdapter emits the iterator adapter of a gem's closure override typed apart from the inherited iterator it
// answers (`tsort_each_child(node)` yielding Strings, where TSort's Node is untyped): the slot's signature, converting
// across (decision 206). False when there is no such slot.
func (c *Compiler) slotSeqAdapter(cls *Class, e entry, env map[string]Type, recv string) bool {
	m := e.M
	if !c.gemFile(m.File) || len(m.Block.Params) != 1 || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword }) {
		return false
	}
	for _, s := range c.parentSlots(cls, m) {
		sm := s.e.M
		if !sm.Iterator || sm.Block == nil || len(sm.Block.Params) != 1 || len(sm.Params) != len(m.Params) {
			continue
		}
		senv := c.slotEnv(s)
		sps, sret := c.sig(sm, senv)
		_, ps, ret := c.seqAdapterSig(m, env)
		if sps == ps && sret == ret {
			return false
		}
		conv := func(code, from, to string) string {
			if from == to || to == "any" {
				return code
			}
			return fmt.Sprintf("rbAs[%s](any(%s), %q)", to, code, to)
		}
		var args []string
		for i, p := range m.Params {
			args = append(args, conv(goParamName(sm.Params[i].Name, i), c.goType(subst(sm.Params[i].Type, senv)), c.goType(subst(p.Type, env))))
		}
		own, slotElem := c.goType(subst(m.Block.Params[0], env)), c.goType(subst(sm.Block.Params[0], senv))
		blk := fmt.Sprintf("func(x %s) { y(%s) }", own, conv("x", own, slotElem))
		if r := subst(m.Block.Ret, env); !isVoid(r) {
			blk = fmt.Sprintf("func(x %s) %s { y(%s); return *new(%s) }", own, c.goType(r), conv("x", own, slotElem), c.goType(r))
		}
		args = append(args, blk)
		c.w("func (self %s) %s(%s) %s {\n\treturn rbSeq(func(y func(%s)) { %s })\n}\n",
			recv, sm.GoName, sps, sret, slotElem, c.blkCall(cls, e, strings.Join(args, ", ")))
		return true
	}
	return false
}

// ignoredBlockValue is the closure an iterator adapter hands a closure-taking each: it passes the value on and drops what
// the block returns. A `|*xs|` block packs several yielded values into one Array, as Enumerable's each does.
func (c *Compiler) ignoredBlockValue(m *Method, elem string, env map[string]Type) string {
	bret := c.goType(subst(m.Block.Ret, env))
	if !m.Block.Rest {
		return fmt.Sprintf("func(x %s) %s { blk(x); return %s }", elem, bret, goZero(bret))
	}
	pack := "x[0]" // ponytail: a typed rest takes the first value; pack like the untyped one if a gem needs it
	if elem == "any" {
		pack = "rbPackYield(x)"
	}
	return fmt.Sprintf("func(x ...%s) %s { blk(%s); return %s }", elem, bret, pack, goZero(bret))
}

// dynAdapterBody is a dynamic slot's body (decision 249): its arguments boxed, its block an rbBlockArg, the override
// called through the dispatcher, the result converted to the slot's return type (TypeError when it is not one).
func (c *Compiler) dynAdapterBody(cls *Class, e entry, s slot) string {
	f := c.newFctx(cls.File, cls, nil)
	f.pass = 2
	f.indent = 1
	env := c.slotEnv(s)
	sm := s.e.M
	var vals []string
	rest := ""
	for i, p := range sm.Params {
		t := subst(p.Type, env)
		if p.Rest {
			rest = t.String()
			f.emit("args_ := make([]any, 0, %d+len(rest_))", len(sm.Params))
			continue
		}
		vals = append(vals, f.coerce(nil, expr{code: goParamName(p.Name, i), typ: t}, TAny{}))
	}
	if rest == "" {
		f.emit("args_ := make([]any, 0, %d)", len(sm.Params)+1)
	}
	if len(vals) > 0 {
		f.emit("args_ = append(args_, %s)", strings.Join(vals, ", "))
	}
	for _, p := range sm.Params {
		if p.Rest {
			f.emit("for _, x := range rest_ {")
			f.emit("\targs_ = append(args_, %s)", f.coerce(nil, expr{code: "x", typ: subst(p.Type, env)}, TAny{}))
			f.emit("}")
		}
	}
	if sm.Block != nil {
		fn := "blk"
		if !anyRestBlock(sm.Block) { // the slot's typed block as an untyped one: each value asserted in, its result boxed out
			bargs := make([]string, len(sm.Block.Params))
			for i, p := range sm.Block.Params {
				bargs[i] = f.coerce(nil, expr{code: fmt.Sprintf("rbArgAt(xs, %d)", i), typ: TAny{}}, subst(p, env))
			}
			call := "blk(" + strings.Join(bargs, ", ") + ")"
			if bret := subst(sm.Block.Ret, env); isVoid(bret) {
				fn = "func(xs ...any) any { " + call + "; return nil }"
			} else {
				fn = "func(xs ...any) any { return " + f.coerce(nil, expr{code: call, typ: bret}, TAny{}) + " }"
			}
		}
		f.emit("if blk != nil {")
		f.emit("\targs_ = append(args_, rbBlockArg{fn: %s})", fn)
		f.emit("}")
	}
	c.noteDyn(e.M.Name)
	call := "rbDyn" + goMethodName(e.M.Name) + "(rbFCall, self, args_...)" // a slot is self's own call: private methods answer
	ret := subst(sm.Ret, env)
	if isVoid(ret) {
		f.emit("%s", call)
	} else {
		f.emit("return %s", f.coerce(nil, expr{code: call, typ: TAny{}}, ret))
	}
	return f.buf.String()
}

// goLiteralDefault is a parameter default an adapter can pass as Go: a Symbol, Integer, String or boolean literal (matrix's
// `each(which = :all)`, decision 412); nil defaults pass nil, as before.
// emptyContainerDefault reports an `= {}` or `= []` default: the adapter passes a fresh one, as the callee would make.
func emptyContainerDefault(n parser.Node) bool {
	switch d := n.(type) {
	case *parser.HashNode:
		return len(d.Elements) == 0
	case *parser.ArrayNode:
		return len(d.Elements) == 0
	}
	return false
}

// emptyContainerCode is the Go expression for an empty container default of a parameter whose Go type is t
// (`*Hash[K, V]`, `*Array[E]`, or any: an untyped `opts = {}` is a Hash of untyped).
func emptyContainerCode(n parser.Node, t string) string {
	if _, isHash := n.(*parser.HashNode); isHash {
		if strings.HasPrefix(t, "*Hash[") {
			return "NewHash" + strings.TrimPrefix(t, "*Hash") + "()"
		}
		return "NewHash[any, any]()"
	}
	if strings.HasPrefix(t, "*Array[") {
		return "NewArray" + strings.TrimPrefix(t, "*Array") + "()"
	}
	return "NewArray[any]()"
}

func goLiteralDefault(n parser.Node) (string, bool) {
	switch d := n.(type) {
	case *parser.SymbolNode:
		return "Symbol(" + strconv.Quote(d.Unescaped.Value) + ")", true
	case *parser.IntegerNode:
		return "Integer(" + strconv.FormatInt(d.Value, 10) + ")", true
	case *parser.StringNode:
		return "String(" + strconv.Quote(d.Unescaped.Value) + ")", true
	case *parser.TrueNode:
		return "Boolean(true)", true
	case *parser.FalseNode:
		return "Boolean(false)", true
	}
	return "", false
}
