package compiler

import (
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// markCalleeDefaults: only a literal default means the same at the call site; defs along an ancestor chain share one Go signature, so the mark spreads (universal free funcs take `any` and need not match).
func (c *Compiler) markCalleeDefaults() {
	names := map[string]bool{}
	for _, m := range c.topDefList {
		m.calleeDefaults = m.Kind == kindDef && scopedDefault(m)
	}
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if !cls.universal && m.Kind == kindDef && scopedDefault(m) {
				m.calleeDefaults = true
				names[m.Name] = true
			}
		}
	}
	for _, cls := range c.classList { // an override whose literal default is another than its ancestor's: the call site would fill the ancestor's (decision 444)
		if cls.universal {
			continue
		}
		for _, anc := range cls.allAncestors() {
			if anc.universal {
				continue
			}
			for name, m := range cls.Methods {
				if a := anc.Methods[name]; a != nil && m.Kind == kindDef && a.Kind == kindDef && !names[name] && otherDefaults(m, a) {
					m.calleeDefaults, a.calleeDefaults = true, true
					names[name] = true
				}
			}
		}
	}
	for changed := len(names) > 0; changed; {
		changed = false
		for _, cls := range c.classList {
			for _, anc := range cls.allAncestors() {
				for name := range names {
					m, a := cls.Methods[name], anc.Methods[name]
					if cls.universal || anc.universal || m == nil || a == nil || m.Kind != kindDef || a.Kind != kindDef || m.calleeDefaults == a.calleeDefaults ||
						a.File != nil && a.File.isPrelude() || m.File != nil && m.File.isPrelude() { // the prelude's Go helpers call its own (NewTypeError(msg)): its signatures are fixed
						continue
					}
					m.calleeDefaults, a.calleeDefaults, changed = true, true, true
				}
			}
		}
	}
	for _, cls := range c.classList {
		if cls.metaOf == nil {
			continue
		}
		if m, init := cls.Methods["new"], cls.metaOf.lookup("initialize"); m != nil && m.Kind == kindSynth && init != nil {
			m.calleeDefaults = init.M.calleeDefaults
		}
	}
}

// otherDefaults reports two user or gem defs of one name whose optional parameters, by position, spell different defaults.
func otherDefaults(m, a *Method) bool {
	if m.Node == nil || a.Node == nil || m.Node.Parameters == nil || a.Node.Parameters == nil || m.File == nil || a.File == nil || m.File.isPrelude() || a.File.isPrelude() {
		return false
	}
	mo, ao := m.Node.Parameters.Optionals, a.Node.Parameters.Optionals
	if len(mo) != len(ao) || len(m.Node.Parameters.Requireds) != len(a.Node.Parameters.Requireds) {
		return false
	}
	for i := range mo {
		x, ok1 := mo[i].(*parser.OptionalParameterNode)
		y, ok2 := ao[i].(*parser.OptionalParameterNode)
		if ok1 && ok2 && m.File.text(x.Value.GetLocation()) != a.File.text(y.Value.GetLocation()) {
			return true
		}
	}
	return false
}

// scopedDefault reports whether some default of m is not a plain literal.
func scopedDefault(m *Method) bool {
	for _, p := range m.Params {
		if p.Default != nil && !literalDefault(p.Default) {
			return true
		}
	}
	return false
}

// literalDefault reports whether a default means the same in any scope.
func literalDefault(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.IntegerNode, *parser.FloatNode, *parser.StringNode, *parser.SymbolNode,
		*parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.RegularExpressionNode:
		return true
	case *parser.ArrayNode:
		for _, e := range n.Elements {
			if !literalDefault(e) {
				return false
			}
		}
		return true
	case *parser.HashNode:
		for _, e := range n.Elements {
			if a, ok := e.(*parser.AssocNode); !ok || !literalDefault(a.Key) || !literalDefault(a.Value) {
				return false
			}
		}
		return true
	}
	return false
}

// nameCmpNil gives a `<=>` returning Integer? the Go name cmpNil: Op_cmp is
// the Integer one Comparable and rbCmp call (emitCmpAdapter).
func nameCmpNil(m *Method) {
	if r, ok := m.Ret.(TOpt); ok && m.Name == "<=>" && isClass(r.Elem, "Integer") {
		m.GoName = "cmpNil"
	}
}

func (c *Compiler) resolveMethod(m *Method) {
	if m.resolved {
		return
	}
	m.resolved = true
	if m.Kind == kindSynth {
		c.resolveSynth(m)
		return
	}
	f := m.File
	if m.sig == nil && m.sigText == "" && isForwardAll(m.Node) {
		c.resolveForwarding(m)
		return
	}
	if m.sig == nil && m.sigText == "" {
		// unannotated override inherits the parent's signature
		if !typesInclude(m) && c.inheritSignature(m) {
			return
		}
		m.sigText, m.inferRet = c.paramSig(m)
	}
	if m.sig == nil {
		sig, err := rbs.ParseMethodType(m.sigText)
		if err != nil {
			c.errorf(f, m.Node, "%v", err)
		}
		m.sig = sig
	}
	sc := typeScope{class: m.Owner, lex: m.Scope, methodTPs: m.sig.TypeParams, srcPos: srcPos{f, m.Line}}
	m.TypeParams = m.sig.TypeParams
	var rest []Param // Ruby puts *rest before the keywords; it goes last, the Go variadic
	for i, p := range m.sig.Params {
		prm := Param{Name: p.Name, Rest: p.Kind == rbs.ParamRest, Keyword: p.Kind == rbs.ParamKeyword || p.Kind == rbs.ParamKwRest, KwRest: p.Kind == rbs.ParamKwRest}
		if want := gradualParam(p.Type); want != nil {
			prm.Type, prm.Want = TAny{}, c.resolveType(want, sc)
		} else {
			prm.Type = c.resolveType(p.Type, sc)
		}
		if pp, ok := m.pendingIdx[i]; ok && p.Kind != rbs.ParamKwRest {
			c.resolvePending(m, &prm, pp)
		}
		if p.Kind == rbs.ParamKwRest {
			prm.Type = TClass{C: c.classes["Hash"], Args: []Type{TClass{C: c.classes["Symbol"]}, prm.Type}}
		}
		if p.Kind == rbs.ParamRest {
			rest = append(rest, prm)
			continue
		}
		m.Params = append(m.Params, prm)
	}
	m.Params = append(m.Params, rest...)
	if m.sig.Block != nil {
		bs := &BlockSig{Ret: c.resolveType(m.sig.Block.Return, sc), Optional: m.sig.Block.Optional}
		if m.sig.Block.Self != nil { // `[self: C]`: a block the method rebinds self for, so it takes self (decision 164)
			bs.Self = c.resolveType(m.sig.Block.Self, sc)
		}
		for i, p := range m.sig.Block.Params {
			t := c.resolveType(p.Type, sc)
			if i < len(m.pendingBlock) {
				prm := Param{Type: t}
				c.resolvePending(m, &prm, m.pendingBlock[i])
				t = prm.Type
				bs.Pending = append(bs.Pending, prm.Pending)
			}
			bs.Params = append(bs.Params, t)
			bs.Rest = bs.Rest || p.Kind == rbs.ParamRest // `{ (*T) -> R }`: a Go variadic, as a lambda's rest (decision 152)
		}
		m.Block = bs
	}
	m.Ret = c.resolveType(m.sig.Return, sc)
	m.retUnion = c.retUnionOf(m, sc)
	_, m.noReturn = m.sig.Return.(rbs.Bot)
	if m.selfText != "" {
		st, err := rbs.ParseType(m.selfText)
		if err != nil {
			c.errorf(f, m.Node, "@self: %v", err)
		}
		if len(m.TypeParams) == 0 {
			c.errorf(f, m.Node, "@self needs type parameters for the receiver to bind")
		}
		m.SelfType = c.resolveType(st, sc)
	}
	if m.inferRet {
		m.Ret = nil
	}
	c.typeStoredBlock(m.Block)
	if m.Block != nil {
		m.Iterator = c.isIterator(m, m.Block)
	}
	c.bindParamNames(m)
}

// gradualParam returns T for a `T | untyped` parameter type, else nil.
func gradualParam(t rbs.Type) rbs.Type {
	u, ok := t.(rbs.Union)
	if !ok || len(u.Elems) != 2 {
		return nil
	}
	if _, ok := u.Elems[1].(rbs.Untyped); !ok {
		return nil
	}
	return u.Elems[0]
}

// overrideYieldShape is the block a gem's non-yielding def takes because an override below it yields (decision 233):
// concurrent-ruby's AbstractLockableObject#synchronize only raises, and MutexLockableObject's yields, which an
// override's inherited signature must then allow.
func (c *Compiler) overrideYieldShape(m *Method) (int, bool, *Method, bool) {
	if m.Owner == nil || m.File == nil || !c.gemFile(m.File) {
		return 0, false, nil, false
	}
	if ps := m.Node.Parameters; ps != nil && ps.Block != nil && ps.Block.Name != nil {
		// `def post(*args, &task)` keeps its block as a value (concurrent-ruby's ExecutorService#post): any arity reaches
		// it, so it stays (*untyped) -> untyped, not an override's yield shape, which RubyExecutorService#post inherited and
		// so passed the posted block one of its two values (decision 257)
		return 0, false, nil, false
	}
	for _, cls := range c.classList {
		o := cls.Methods[m.Name]
		if cls == m.Owner || o == nil || o == m || o.Node == nil || !cls.isSubclassOf(m.Owner) || !sameParamShape(o, m) {
			continue // an override of another arity is not the slot's shape: the iterator adapter would index params it lacks
		}
		if n, optional, ok := c.yieldShape(o); ok {
			return n, optional, o, true
		}
	}
	return 0, false, nil, false
}

// paramSig builds a def's signature from rbs-inline's per-parameter form
// (`# @rbs x: T`, `# @rbs *xs: T`, `# @rbs return: T`). Without a return
// annotation the return type is inferred from the body (inferRet). A def
// with no parameters needs no annotation at all.
func (c *Compiler) paramSig(m *Method) (string, bool) {
	ann := m.File.annotations(m.Line)
	names, defaults, rest := c.defParams(m, m.Node.Parameters)
	var ps []string
	// a parameter with no annotation is typed from use (decision 146): untyped here, resolvePending types it
	pa := func(name string) string {
		t := ann[name+":"]
		if len(t) > 0 {
			return t[0]
		}
		if m.pendingIdx == nil {
			m.pendingIdx = map[int]pendingParam{}
		}
		m.pendingIdx[len(ps)] = pendingParam{key: paramKey(m, name), name: name}
		if c.gemFile(m.File) {
			if c.gemParams == nil {
				c.gemParams = map[pendingKey]bool{}
			}
			c.gemParams[paramKey(m, name)] = true
		}
		if c.gemFile(m.File) && (onlyInterpolated(m.Node.Body, name) || typeTested(m.Node.Body, name)) {
			if c.interpOnly == nil {
				c.interpOnly = map[pendingKey]bool{}
			}
			c.interpOnly[paramKey(m, name)] = true
		}
		return "untyped"
	}
	for i, n := range names {
		t := pa(n)
		if defaults[i] != nil {
			t = "?" + t
		}
		ps = append(ps, t)
	}
	if rest != "" {
		ps = append(ps, "*"+pa("*"+strings.TrimPrefix(rest, anonRest)))
	}
	for _, k := range c.defKeywords(m, m.Node.Parameters) {
		switch {
		case k.rest:
			kt := "untyped" // an unannotated **opts: its values untyped, as an unannotated parameter's (decision 183)
			if t := ann["**"+strings.TrimPrefix(k.name, anonKwrest)+":"]; len(t) > 0 {
				kt = t[0]
			}
			ps = append(ps, "**"+kt+" "+k.name)
		case k.def != nil:
			ps = append(ps, "?"+k.name+": "+pa(k.name))
		default:
			ps = append(ps, k.name+": "+pa(k.name))
		}
	}
	sig := "(" + strings.Join(ps, ", ") + ") "
	n, optional, ok := c.yieldShape(m)
	from := m // the def whose yields decide the block's value
	if !ok {
		var o *Method
		n, optional, o, ok = c.overrideYieldShape(m)
		from = o
	}
	if ok { // a def that yields takes a block typed by what it yields (decision 146)
		bps := make([]string, n)
		for i := range bps {
			bps[i] = "untyped"
			m.pendingBlock = append(m.pendingBlock, pendingParam{key: yieldKey(m, i), name: "yield's argument " + strconv.Itoa(i+1)})
		}
		splat := !c.gemFile(from.File) && splatYields(from.Node.Body)
		if splat { // `yield(*xs)`: the block takes the elements, `{ (*T) -> R }` (decision 363)
			bps[0] = "*untyped"
		}
		ret := "void"
		// a gem method's tail yield is its value (`if owned? then yield else @lock.synchronize { yield } end`, decision 226;
		// liquid's `result = with_profiling(context) { ... }`, decision 331): a gem block's `return` unwinds through the
		// closure (decision 247), so Cuba's `on` returning from inside `try`'s block still works.
		wrapper := c.pruneGems && from.File != nil && c.gemFile(from.File)
		if yieldValueUsed(from.Node.Body, wrapper || splat || c.callValueUsed(m.Name, true)) { // a *rest block is never a Go iterator: a tail yield is the method's value
			ret = "untyped"
		}
		sig += map[bool]string{true: "?", false: ""}[optional] + "{ (" + strings.Join(bps, ", ") + ") -> " + ret + " } "
	}
	sig += "-> "
	if r := ann["return:"]; len(r) > 0 {
		return sig + r[0], false
	}
	if m.Name == "initialize" { // new answers the object; initialize's value is never seen (`def initialize = @x = y`)
		return sig + "void", false
	}
	return sig + "untyped", true
}

// inheritsRet: an override answers what its slot does, unless its gem parent only answers nil, a placeholder (rubocop-ast's
// Node#receiver under MethodDispatchNode's): then it infers its own value, and typeAbstracts widens the parent to it.
func (c *Compiler) inheritsRet(m *Method) bool {
	return m.inherited != nil && (!c.gemFile(m.File) || !onlyLiteral(m.inherited.Node))
}

// inferRet: a dry run joins the body's returns; provisional until ivar discovery finishes (discoverIvars clears it).
func (c *Compiler) inferRet(m *Method) {
	if !m.inferRet || m.Ret != nil {
		return
	}
	if m.inferring && c.pruneGems && c.gemFile(m.File) { // a gem's recursion (bindata's `superclass.dsl_parser`): untyped at the inner call, the outer run types the rest
		m.Ret, m.retWhy = TAny{}, "recursive"
		return
	}
	if m.inferring {
		m.recursed = true
		c.errorf(m.File, m.Node, "method %s is recursive; annotate its return type (`# @rbs return: T`)", m.Name)
	}
	m.inferring = true
	m.recursed = false
	defer func() { m.inferring = false }()
	if c.inheritsRet(m) {
		c.inferRet(m.inherited)
		m.Ret = m.inherited.Ret
		return
	}
	nw := len(c.Warnings)
	var f *fctx
	var ts []Type
	run := func(drop bool) {
		f = c.newFctx(m.File, m.Owner, m)
		f.retVar = "ret_"
		f.dropFailing = drop
		if c.unwindsReturn(m) {
			f.retTok = "rbRetTok_" // a block's return is a returned value too (decision 247)
		}
		ts = nil
		f.retTypes = &ts
		f.genBody(m.Node.Body, c.paramLocals(m), tail{kind: tailReturn, types: &ts}, nil)
	}
	err := catchCompileError(func() { run(false) })
	if err != nil && m.recursed {
		err = catchCompileError(func() { c.inferRecursive(m, run, &ts) })
	}
	if err != nil {
		if c.round { // a body its still-untyped parameters break returns untyped this round, so its callers still record what they pass
			c.dropWarnings(nw)
			m.Ret, m.retWhy, m.roundRet = TAny{}, "body fails with its parameters untyped (inference round)", true
			return
		}
		panic(*err)
	}
	c.dropWarnings(nw) // emitMethod warns for real
	var ret Type = TNil{}
	if len(ts) == 0 && c.pruneGems && c.gemFile(m.File) && endsInRaise(m.Node.Body) {
		// a gem's abstract `def synchronize; raise NotImplementedError; end`: what overrides return, whose
		// inherited signature this is (decision 233)
		ret, m.retWhy = TAny{}, "abstract (raises)"
	}
	ts = c.roundBottomReturns(m, ts)
	for i, t := range ts {
		if isVoid(t) {
			t = TNil{}
		}
		if i == 0 {
			ret = t
			continue
		}
		j, ok := join(ret, t)
		if !ok && c.pruneGems && c.gemFile(m.File) { // a gem's method returning an Enumerator or self: untyped (decision 172)
			j, ok = TAny{}, true
			m.retWhy = "returns both " + typeString(ret) + " and " + typeString(t)
		}
		if !ok {
			c.errorf(m.File, m.Node, "method %s returns both %s and %s; annotate its return type (`# @rbs return: T`)", m.Name, ret, t)
		}
		ret = j
	}
	if isAny(ret) && m.retWhy == "" {
		m.retWhy = "body's value is untyped"
		if b, ok := m.Node.Body.(*parser.StatementsNode); ok && c.env.UntypedWhy && len(b.Body) > 0 {
			why := "?"
			_ = catchCompileError(func() { why = f.untypedWhy(b.Body[len(b.Body)-1], 1) })
			m.retWhy += ": " + why
		}
	}
	m.Ret = ret
	m.roundRet = c.awaitsParams(m, ret)
}

// roundBottomReturns leaves a round's untyped returns out of m's join when a typed one remains and m has a parameter not
// typed from use: `def meld(a, b) = a.nil? ? b : a` returns b's Node, so `@root = meld(@root, n)` types @root and through it
// a, where the untyped `a` would poison both for good (decision 383; 307's lift does this for a branch's value). The final
// compile joins every return as before. A parameter still untyped when the rounds converge never justified the drop
// (liquid's `slice_collection(collection, ...)`, whose `[]` would type every caller's segment): updateInference stops it.
func (c *Compiler) roundBottomReturns(m *Method, ts []Type) []Type {
	if !c.round || !slices.ContainsFunc(ts, func(t Type) bool { return !holdsAny(t) }) || !slices.ContainsFunc(ts, holdsAny) {
		return ts
	}
	var keys []pendingKey
	for _, p := range m.Params {
		if p.Pending != "" && isAny(p.Type) && (c.infer == nil || !c.infer.noDrop[p.Pending]) {
			keys = append(keys, p.Pending)
		}
	}
	if len(keys) == 0 {
		return ts
	}
	if c.roundDrops == nil {
		c.roundDrops = map[pendingKey]bool{}
	}
	for _, k := range keys {
		c.roundDrops[k] = true
	}
	return slices.DeleteFunc(slices.Clone(ts), holdsAny)
}

// inferRecursive: as spinel does, a recursive call's unknown value drops out of the first join, then the join is iterated to a fixpoint (decision 355).
func (c *Compiler) inferRecursive(m *Method, run func(drop bool), ts *[]Type) {
	run(true)
	ret := joinReturns(nil, *ts)
	if ret == nil || isAny(ret) {
		c.errorf(m.File, m.Node, "method %s is recursive and no path returns a typed value without recursing; annotate its return type (`# @rbs return: T`)", m.Name)
	}
	var fresh []*Method
	for _, x := range c.inferredMethods() {
		if x != m && x.Ret == nil {
			fresh = append(fresh, x)
		}
	}
	defer func() { m.Ret = nil }()
	for range 4 {
		m.Ret = ret
		for _, x := range fresh { // a cycle's other methods typed themselves against the last candidate (a 3-cycle kept a stale nil)
			x.Ret = nil
		}
		run(false)
		next := joinReturns(ret, *ts)
		if typeString(next) == typeString(ret) {
			return
		}
		ret = next
	}
	c.errorf(m.File, m.Node, "method %s is recursive and its return type does not settle (last %s); annotate it (`# @rbs return: T`)", m.Name, ret)
}

// joinReturns leaves out a type that does not join: inferRet's final join reports it.
func joinReturns(ret Type, ts []Type) Type {
	for _, t := range ts {
		if isVoid(t) {
			t = TNil{}
		}
		if ret == nil {
			ret = t
		} else if j, ok := join(ret, t); ok {
			ret = j
		}
	}
	return ret
}

// protocolSig is the signature Ruby's hook protocol gives an unannotated respond_to_missing? or method_missing, by its
// parameter shape; "" for any other def.
func protocolSig(n *parser.DefNode) string {
	ps := n.Parameters
	if ps == nil || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil || len(ps.Posts) > 0 {
		return ""
	}
	switch {
	case n.Name == "respond_to_missing?" && len(ps.Requireds) == 2 && len(ps.Optionals) == 0 && ps.Rest == nil:
		return "(Symbol, bool) -> bool"
	case n.Name == "respond_to_missing?" && len(ps.Requireds) == 1 && len(ps.Optionals) == 1 && ps.Rest == nil:
		return "(Symbol, ?bool) -> bool"
	case n.Name == "method_missing" && len(ps.Requireds) == 1 && len(ps.Optionals) == 0 && ps.Rest != nil:
		return "(Symbol, *untyped) -> untyped"
	}
	return ""
}

// superPassesLocals: every super in body is a zsuper or passes only locals (each maybe splatted), so the parameters
// mean what the parent's do.
func superPassesLocals(body parser.Node) bool {
	return !anyNode(body, func(x parser.Node) bool {
		sup, ok := x.(*parser.SuperNode)
		if !ok {
			return false // a zsuper is a ForwardingSuperNode: every parameter passed on
		}
		if sup.Arguments == nil || len(sup.Arguments.Arguments) == 0 {
			return true // `super()`: the parent gets none of these parameters, so their types are this def's own (FormatterSet < Array's `initialize(options = {})`)
		}
		for _, a := range sup.Arguments.Arguments {
			if sp, ok := a.(*parser.SplatNode); ok {
				if sp.Expression == nil { // super(*)
					continue
				}
				a = sp.Expression
			}
			if _, ok := a.(*parser.LocalVariableReadNode); !ok {
				return true
			}
		}
		return false
	})
}

// inheritSignature copies the signature of the method m overrides, if any.
func (c *Compiler) inheritSignature(m *Method) bool {
	if m.Owner == nil {
		return false
	}
	e := c.inheritedSig(m)
	if e == nil {
		return false
	}
	c.resolveMethod(e.M)
	if !c.sameParamShape(m, e.M) { // `def initialize = super(4)`: its own parameters, typed as any def's (decision 146)
		return false
	}
	if m.Name == "initialize" && m.Node != nil && !superPassesLocals(m.Node.Body) {
		return false // `def initialize(executor) = super(Event.new(executor))`: same shape, other meaning (decision 286)
	}
	if e.M.Block != nil && m.Node != nil && !e.M.Block.Rest {
		if n, ok := yieldArity(m.Node.Body); ok && n != len(e.M.Block.Params) {
			return false // Matrix#each_with_index yields e, row, col over Enumerable's (E, Integer): its own block (decision 412)
		}
	}
	m.inherited = e.M
	m.TypeParams = e.M.TypeParams
	for _, p := range e.M.Params {
		m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, e.Env), Default: p.Default, Rest: p.Rest, Keyword: p.Keyword, KwRest: p.KwRest, Post: p.Post, Want: subst(p.Want, e.Env), Pending: p.Pending})
	}
	if m.Node != nil && m.Node.Parameters != nil {
		// its own defaults, not the parent's: rubocop-ast's `updated(type = nil, children = nil, properties = {})` over
		// AST::Node's `properties=nil` reads `properties[:location]` (decision 444)
		var own []parser.Node
		for range m.Node.Parameters.Requireds {
			own = append(own, nil)
		}
		for _, o := range m.Node.Parameters.Optionals {
			if op, ok := o.(*parser.OptionalParameterNode); ok {
				own = append(own, op.Value)
			} else {
				own = append(own, nil)
			}
		}
		i := 0
		for k := range m.Params {
			if p := &m.Params[k]; !p.Rest && !p.Keyword && !p.KwRest {
				if i < len(own) && own[i] != nil && p.Default != nil {
					p.Default = own[i]
				}
				i++
			}
		}
	}
	if e.M.Block != nil && !c.ignoresOwnBlock(m) {
		// all of the parent's block: a dropped Rest made concurrent-ruby's ThenPromise#initialize take one value of the
		// (*untyped) block its parent takes, losing then_on's other arguments (decision 256)
		b := *e.M.Block
		b.Params, b.Ret = substAll(e.M.Block.Params, e.Env), subst(e.M.Block.Ret, e.Env)
		if b.Self != nil {
			b.Self = subst(b.Self, e.Env)
		}
		m.Block = &b
	}
	m.Ret = subst(e.M.Ret, e.Env)
	m.inferRet = e.M.inferRet // the parent's inferred type, once it has one (inferRet)
	// decision 4 holds for the override's own body: a rescue around yield makes it a closure
	m.Iterator = e.M.Iterator && (m.Kind != kindDef || !containsRescueClause(m.Node.Body))
	c.bindParamNames(m)
	return true
}

// yieldArity is how many values every yield in body passes (not counting a nested def's), false when there is none,
// they disagree, or one splats.
func yieldArity(body parser.Node) (int, bool) {
	n, found, ok := 0, false, true
	walkNode(body, func(x parser.Node) (bool, bool) {
		switch y := x.(type) {
		case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
			return false, false
		case *parser.YieldNode:
			k := 0
			if y.Arguments != nil {
				for _, a := range y.Arguments.Arguments {
					if isSplat(a) {
						ok = false
					}
				}
				k = len(y.Arguments.Arguments)
			}
			if found && k != n {
				ok = false
			}
			n, found = k, true
		}
		return false, true
	})
	return n, found && ok
}

// ignoresOwnBlock: a gem override whose body never yields, asks block_given?, names its block, or reaches a super that
// passes it on (a zsuper, or super without a block of its own) takes none, though its parent does: liquid's
// Document#parse calls `super do |tag, params| ... end` over BlockBody#parse's yield (decision 331). In user code a
// block-taking override keeps its parent's signature (it is an error otherwise).
func (c *Compiler) ignoresOwnBlock(m *Method) bool {
	if m.Kind != kindDef || m.Node == nil || m.Node.Body == nil || m.BlockParam != "" || m.File == nil || !c.gemFile(m.File) {
		return false
	}
	return !anyNode(m.Node.Body, func(x parser.Node) bool {
		switch x := x.(type) {
		case *parser.YieldNode, *parser.BlockArgumentNode:
			return true
		case *parser.ForwardingSuperNode: // `super do ... end`, no parentheses: a block of its own, else the method's
			return x.Block == nil
		case *parser.SuperNode:
			return x.Block == nil
		case *parser.CallNode:
			return x.Receiver == nil && (x.Name == "block_given?" || x.Name == "iterator?")
		}
		return false
	})
}

// isIterator decides whether a block-taking method compiles to a Go
// iterator (its block call sites become `for range` loops) or takes the
// block as a closure. Iterators: the block and the method both return
// nothing, the block is only ever yielded to (a %x{} leaf must build an
// iterator itself; one that stores or calls `blk` takes a closure), and
// nothing rescues around the yield, since Go forbids a range function from
// recovering a panic raised in the loop body.
func (c *Compiler) isIterator(m *Method, bs *BlockSig) bool {
	if !isVoid(bs.Ret) || m.sig.Block.Optional || len(bs.Params) > 2 || bs.Rest || bs.Self != nil {
		return false // a range func yields at most two values, none variadic; other blocks are closures
	}
	if _, ok := m.Ret.(TVoid); !ok && m.Ret != nil && !isNil(m.Ret) {
		return false
	}
	if m.Kind == kindPrimitive {
		x := m.Node.Body.(*parser.StatementsNode).Body[0].(*parser.XStringNode)
		return strings.Contains(x.Unescaped.Value, "func(yield ")
	}
	return m.Kind != kindDef || !containsRescueClause(m.Node.Body)
}

// containsNestedYield reports a yield inside a block or lambda in n (not in a nested def).
func containsNestedYield(n parser.Node, inBlock bool) bool {
	switch n.(type) {
	case nil:
		return false
	case *parser.YieldNode:
		return inBlock
	case *parser.DefNode:
		return false
	case *parser.BlockNode, *parser.LambdaNode:
		inBlock = true
	}
	for _, ch := range n.CompactChildNodes() {
		if containsNestedYield(ch, inBlock) {
			return true
		}
	}
	return false
}

// overridesIterator reports whether m takes its block as a closure yet
// overrides an iterator, e.g. an `each` that rescues around yield under
// Enumerable's. Callers of the iterator (Enumerable's bodies, through its
// constraint) keep its Go name, now an iter.Seq adapter over the closure.
func (c *Compiler) overridesIterator(m *Method) bool {
	if m.Owner == nil || m.Block == nil || m.Iterator || !isVoid(m.Block.Ret) {
		return false
	}
	e := c.inheritedSig(m)
	if e == nil || e.M.Block == nil || len(e.M.Block.Params) != len(m.Block.Params) { // another block shape is another method, not the iterator's slot (decision 412)
		return false
	}
	return e.M.Iterator || c.overridesIterator(e.M)
}

// forwardParent is inheritedSig for a method that may be a top-level def.
func (c *Compiler) forwardParent(m *Method) *entry {
	if m.Owner == nil {
		return nil
	}
	return c.inheritedSig(m)
}

// inheritedSig finds the method an unannotated def overrides: a superclass's own def before an included module's
// (regexp_parser's Subexpression#to_h refines Base#to_h, not Enumerable#to_h, whose untyped Hash would widen it;
// the module's slot gets its adapter, decision 206).
func (c *Compiler) inheritedSig(m *Method) *entry {
	cls := m.Owner
	if cls.Super != nil && m.superTarget == nil && m.structDef == nil {
		if e := cls.Super.lookup(m.Name); e != nil && !e.Owner.IsModule {
			return e
		}
	}
	for i := len(cls.Includes) - 1; i >= 0; i-- {
		inc := cls.Includes[i]
		if inc.Mod == nil {
			continue
		}
		if e := inc.Mod.lookup(m.Name); e != nil {
			env := map[string]Type{}
			for j, p := range inc.Mod.TypeParams {
				env[p] = inc.Args[j]
			}
			return &entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: cls}
		}
	}
	if g := m.superTarget; g != nil { // a prepended copy: super is the method it replaced (decision 199)
		return &entry{M: g, Owner: cls, Env: map[string]Type{}, Entry: cls}
	}
	if g := m.structDef; g != nil {
		return &entry{M: g, Owner: cls, Env: map[string]Type{}, Entry: cls}
	}
	if cls.Super != nil {
		return cls.Super.lookup(m.Name)
	}
	return nil
}

// bindParamNames matches RBS positional params to the def's parameter
// names and records literal defaults.
func (c *Compiler) bindParamNames(m *Method) {
	if m.Node == nil {
		return
	}
	names, defaults, rest := c.defParams(m, m.Node.Parameters)
	c.bindKeywords(m)
	nPos := 0
	for i := range m.Params {
		if m.Params[i].Keyword {
			continue
		}
		if m.Params[i].Rest {
			if rest == "" {
				c.errorf(m.File, m.Node, "signature has a rest param but def does not")
			}
			m.Params[i].Name = rest
			continue
		}
		if nPos >= len(names) {
			c.errorf(m.File, m.Node, "signature has %d positional params but def has %d", len(m.Params), len(names))
		}
		m.Params[i].Name = names[nPos]
		m.Params[i].Default = defaults[nPos]
		m.Params[i].Post = m.Node.Parameters != nil && nPos >= len(m.Node.Parameters.Requireds)+len(m.Node.Parameters.Optionals)
		nPos++
	}
	if nPos != len(names) {
		c.errorf(m.File, m.Node, "signature has %d positional params but def has %d", nPos, len(names))
	}
	if len(m.Params) > 0 {
		c.checkKeywordSig(m)
	}
	if rest != "" {
		found := false
		for _, p := range m.Params {
			found = found || p.Rest
		}
		if !found {
			c.errorf(m.File, m.Node, "def has a rest param but signature does not")
		}
	}
}

// bindKeywords matches the signature's keywords to the def's by name and takes their defaults.
func (c *Compiler) bindKeywords(m *Method) {
	ks := c.defKeywords(m, m.Node.Parameters)
	byName := map[string]defKeyword{}
	for _, k := range ks {
		byName[k.name] = k
	}
	seen := 0
	for i := range m.Params {
		p := &m.Params[i]
		if !p.Keyword {
			continue
		}
		if p.KwRest {
			var r *defKeyword
			for j := range ks {
				if ks[j].rest {
					r = &ks[j]
				}
			}
			if r == nil {
				c.errorf(m.File, m.Node, "signature has **%s but def has no ** parameter", p.Type)
			}
			p.Name = r.name
			seen++
			continue
		}
		k, ok := byName[p.Name]
		if !ok || k.rest {
			c.errorf(m.File, m.Node, "signature has keyword %s: but def does not", p.Name)
		}
		p.Default = k.def
		seen++
	}
	if seen != len(ks) {
		c.errorf(m.File, m.Node, "def has %d keyword parameters but its signature has %d: write them as `name: T` (`?name: T` with a default, `**T` for **opts)", len(ks), seen)
	}
}

// checkKeywordSig: an optional keyword in the signature (`?b:`) must have a default in the def, and a required one none.
func (c *Compiler) checkKeywordSig(m *Method) {
	if m.sig == nil {
		return
	}
	for _, sp := range m.sig.Params {
		if sp.Kind != rbs.ParamKeyword {
			continue
		}
		for _, p := range m.Params {
			if p.Keyword && !p.KwRest && p.Name == sp.Name && sp.Optional != (p.Default != nil) {
				c.errorf(m.File, m.Node, "keyword %s: is optional in only one of the signature and the def", p.Name)
			}
		}
	}
}

// typeAbstracts gives a gem's inferred method whose body only raises (`def x = raise NotImplementedError`, the
// abstract-method idiom) its overriders' joined return type: it never returns, so the slot is theirs.
func (c *Compiler) typeAbstracts() {
	for _, k := range c.classList {
		for _, m := range k.MethodList {
			if !m.inferRet || m.Kind != kindDef || !c.gemFile(m.File) || !onlyRaises(m.Node) && !onlyLiteral(m.Node) {
				continue
			}
			ret := c.literalType(m.Node) // the parent's own answer (nil, or `def arity = 1`'s Integer) stays one
			for _, cls := range c.classList {
				if cls == k || !cls.isSubclassOf(k) && !slices.Contains(cls.ancestors(), k) {
					continue
				}
				e := cls.lookup(m.Name)
				if e == nil || e.M == m || e.M.Ret == nil {
					continue
				}
				if ret == nil {
					ret = e.M.Ret
				} else if j, ok := join(ret, e.M.Ret); ok {
					ret = j
				} else {
					ret = TAny{}
				}
			}
			if ret != nil {
				m.Ret = ret
			}
		}
	}
}

// onlyLiteral reports a def whose body is empty or one literal: a placeholder its overrides fill with other kinds
// (rubocop-ast's `def receiver; nil; end`, and NodePattern::Node's `def arity = 1` under Rest's `0..Float::INFINITY`).
func onlyLiteral(d *parser.DefNode) bool {
	if onlyNil(d) {
		return true
	}
	st, ok := d.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return false
	}
	switch st.Body[0].(type) {
	case *parser.IntegerNode, *parser.FloatNode, *parser.TrueNode, *parser.FalseNode, *parser.SymbolNode, *parser.StringNode:
		return true
	}
	return false
}

// literalType is the type of a placeholder def's one literal (onlyLiteral): TNil for nil or an empty body.
func (c *Compiler) literalType(d *parser.DefNode) Type {
	st, ok := d.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return TNil{}
	}
	switch st.Body[0].(type) {
	case *parser.IntegerNode:
		return TClass{C: c.classes["Integer"]}
	case *parser.FloatNode:
		return TClass{C: c.classes["Float"]}
	case *parser.TrueNode, *parser.FalseNode:
		return TClass{C: c.classes["Boolean"]}
	case *parser.SymbolNode:
		return TClass{C: c.classes["Symbol"]}
	case *parser.StringNode:
		return TClass{C: c.classes["String"]}
	}
	return TNil{}
}

// onlyNil reports a def whose body is empty or one `nil`: a placeholder its overrides fill (rubocop-ast's `def receiver; nil; end`).
func onlyNil(d *parser.DefNode) bool {
	if d == nil || d.Body == nil {
		return d != nil
	}
	st, ok := d.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return false
	}
	_, isNil := st.Body[0].(*parser.NilNode)
	return isNil
}

// onlyRaises reports a def whose body is one raise.
func onlyRaises(d *parser.DefNode) bool {
	if d == nil || d.Body == nil {
		return false
	}
	st, ok := d.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return false
	}
	call, ok := st.Body[0].(*parser.CallNode)
	return ok && call.Receiver == nil && (call.Name == "raise" || call.Name == "fail")
}

// endsInRaise: body's last statement is a raise (an abstract method's whole body).
func endsInRaise(body parser.Node) bool {
	st, ok := body.(*parser.StatementsNode)
	if !ok || len(st.Body) == 0 {
		return false
	}
	c, ok := st.Body[len(st.Body)-1].(*parser.CallNode)
	return ok && c.Receiver == nil && (c.Name == "raise" || c.Name == "fail")
}

// retUnionOf is a gem generic method's declared union return when it resolved to untyped (decision 321), else nil.
func (c *Compiler) retUnionOf(m *Method, sc typeScope) Type {
	u, ok := m.sig.Return.(rbs.Union)
	if !ok || !isAny(m.Ret) || len(m.TypeParams) == 0 || !c.gemFile(m.File) {
		return nil
	}
	return c.varUnion(u, sc, m.TypeParams)
}

// varUnion resolves a gem sig's union return whose members include the method's own type variables (Sorbet's
// `T.any(T.type_parameter(:U), String)`): the union as declared, for dropUnboundVars at each call (decision 321).
// nil when a member is anything else resolveUnion would refuse.
func (c *Compiler) varUnion(u rbs.Union, sc typeScope, params []string) Type {
	ms := make([]Type, 0, len(u.Elems))
	vars := 0
	for _, e := range u.Elems {
		t := c.resolveType(e, sc)
		switch m := stripOpt(t).(type) {
		case TVar:
			if !slices.Contains(params, m.Name) {
				return nil
			}
			vars++
		case TClass, TTuple, TFunc, TNil:
		default:
			return nil
		}
		if isAny(t) {
			return nil
		}
		ms = append(ms, t)
	}
	if vars == 0 || vars == len(ms) {
		return nil
	}
	return TUnion{Members: ms}
}

// awaitsParams reports, in a round, an untyped return that a still-untyped parameter typed from use may explain (decision 358).
func (c *Compiler) awaitsParams(m *Method, ret Type) bool {
	// a parameter an earlier round left untyped is not awaited again: its value would flip the local back and forth
	return c.round && isAny(ret) && slices.ContainsFunc(m.Params, func(p Param) bool {
		return p.Pending != "" && isAny(p.Type) && (c.infer == nil || !c.infer.none[p.Pending])
	})
}
