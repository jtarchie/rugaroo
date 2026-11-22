package compiler

import (
	"cmp"
	"maps"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// refineIvars types an ivar first assigned an unannotated `[]`/`{}` by
// what the class's methods put in it (decision 15), as analyze does for
// locals. A typing is dropped when a dry-run emission of the classes that
// see the ivar fails to compile or adds a conversion; the rest are kept,
// greedily in name order.
func (c *Compiler) refineIvars() {
	c.nilDefer = nil
	var cands []*Ivar
	typed, orig := map[*Ivar]Type{}, map[*Ivar]Type{}
	for range 8 { // one typed ivar can type the next: re-discover with it typed
		n := len(cands)
		for _, cls := range c.classList {
			for _, name := range slices.Sorted(maps.Keys(cls.Ivars)) {
				iv := cls.Ivars[name]
				if t := refineContainer(iv.Type, iv.open, iv.elems, iv.Owner != nil && c.gemFile(iv.Owner.File), iv.Owner != nil && c.gemFile(iv.Owner.File) && c.env.InferUnknown); t != nil && typed[iv] == nil {
					cands = append(cands, iv)
					typed[iv], orig[iv] = t, iv.Type
					iv.Type = t
				}
			}
		}
		if len(cands) == n {
			break
		}
		c.discoverIvars()
	}
	if len(cands) == 0 {
		return
	}
	for _, iv := range cands {
		iv.Type = orig[iv]
	}
	kind, sig := c.refineKind(), refineCands(cands, typed, orig)
	if r := c.refineIn[kind]; !c.round && r != nil && slices.Equal(r.Cands, sig) && len(r.Accepted) == len(cands) && (r.NilSites == nil || !c.noNilDefer) { // a hit: the same candidates from the same state (decision 433)
		c.replayRefine(r, cands, typed, orig)
		c.refineOut[kind] = r
		return
	}
	var snap refineSnap
	if !c.round {
		snap = c.takeRefineSnap()
	}
	c.baseFailed = nil
	base, _ := c.dryRunIvarUsers(cands)
	attempt := func(d *nilDeferral) (*compileError, bool) {
		c.nilDefer = d
		c.dryTry = true
		convs, err := c.dryRunIvarUsers(cands)
		c.dryTry = false
		ok := err == nil
		for k := range convs {
			ok = ok && base[k]
		}
		c.rs.try(ok)
		return err, ok
	}
	var retry nilRetry
	var set map[*Ivar]bool
	baseline := func(d *nilDeferral) bool { // the candidates untyped
		setIvarTypes(cands, orig, typed, nil)
		err, _ := attempt(d)
		setIvarTypes(cands, orig, typed, set)
		return err == nil
	}
	try := func(s map[*Ivar]bool) bool {
		set = s
		setIvarTypes(cands, orig, typed, set)
		err, ok := attempt(nil)
		ok = c.retryNil(&retry, ok, err, baseline, func(d *nilDeferral) bool { _, ok := attempt(d); return ok })
		if !ok {
			c.nilDefer = nil
		}
		return ok
	}
	if prev := c.roundRefine[kind]; !c.round || prev == nil || !try(seedSet(cands, sig, prev)) {
		salvage(cands, try) // a failed last try leaves the original types
	}
	accepted := make([]bool, len(cands))
	for i, iv := range cands {
		accepted[i] = typeEq(iv.Type, typed[iv])
	}
	c.noteRefineDrops(cands, typed, accepted)
	if c.round {
		c.roundRefine[kind] = &refineRecord{Cands: sig, Accepted: accepted}
		c.rs.accepted(c, sig, accepted)
		return
	}
	if c.refineOut == nil {
		return
	}
	if rec := c.recordRefine(snap, sig, accepted); rec != nil {
		c.refineOut[kind] = rec
	}
}

// seedSet is every candidate but the typings the previous inference round dropped: a round tries that set first and
// searches only when it does not compile (decision 444). The final pass always searches.
func seedSet(cands []*Ivar, sig []string, prev *refineRecord) map[*Ivar]bool {
	dropped := map[string]bool{}
	for i, s := range prev.Cands {
		dropped[s] = !prev.Accepted[i]
	}
	set := map[*Ivar]bool{}
	for i, iv := range cands {
		if !dropped[sig[i]] {
			set[iv] = true
		}
	}
	return set
}

// setIvarTypes gives each candidate its typed type when set has it, else its original.
func setIvarTypes(cands []*Ivar, orig, typed map[*Ivar]Type, set map[*Ivar]bool) {
	for _, iv := range cands {
		iv.Type = orig[iv]
		if set[iv] {
			iv.Type = typed[iv]
		}
	}
}

// salvage finds a set of typings that try accepts: all of them, else all
// but one (the usual lone culprit; an untyped baseline may not compile,
// so adding one at a time can fail throughout), else those that pass when
// added one at a time. The last try's state is left applied; the result
// says whether it passed.
func salvage[K comparable](cands []K, try func(map[K]bool) bool) bool {
	set := map[K]bool{}
	for _, k := range cands {
		set[k] = true
	}
	if try(set) {
		return true
	}
	for _, k := range cands {
		delete(set, k)
		if try(set) {
			return true
		}
		set[k] = true
	}
	clear(set)
	for _, k := range cands {
		if set[k] = true; !try(set) {
			delete(set, k)
		}
	}
	return try(set)
}

// dryRunIvarUsers emits, and discards, every method of the classes that
// can see one of ivs, returning the conversion sites and its compile error.
func (c *Compiler) dryRunIvarUsers(ivs []*Ivar) (map[string]bool, *compileError) {
	saved, nw := c.out.String(), len(c.Warnings)
	for _, m := range c.inferredMethods() {
		m.Ret = nil // provisional: may depend on the ivar types being tried
	}
	c.convs = map[string]bool{}
	c.inDryRun = true
	defer func() { c.inDryRun = false }()
	err := catchCompileError(func() {
		for _, cls := range c.classList {
			if !slices.ContainsFunc(ivs, func(iv *Ivar) bool { return cls.isSubclassOf(iv.Owner) }) {
				continue
			}
			c.rs.sees(cls)
			for _, m := range cls.MethodList {
				if m.Kind == kindDef {
					c.inferRet(m)
					c.emitMethod(m)
				}
			}
		}
	})
	convs := c.convs
	c.convs = nil
	c.out.Reset()
	c.out.WriteString(saved)
	c.dropWarnings(nw)
	return convs, err
}

// inferReturns re-infers every unannotated return type (decision 36): the
// ones inferred during discoverIvars saw ivars not yet typed.
func (c *Compiler) inferReturns() {
	for _, m := range c.inferredMethods() {
		m.Ret = nil
	}
	for _, m := range c.inferredMethods() {
		c.keepGoing(func() { c.inferRet(m) })
		if m.Ret == nil { // RUGAROO_KEEP_GOING after a body failed: untyped, so the rest still types
			m.Ret = TAny{}
		}
	}
}

func (c *Compiler) inferredMethods() []*Method {
	var out []*Method
	for _, m := range c.topDefList {
		if m.inferRet {
			out = append(out, m)
		}
	}
	for _, cls := range c.classList {
		if cls.isTemplate() {
			continue
		}
		for _, m := range cls.MethodList {
			if m.inferRet {
				out = append(out, m)
			}
		}
	}
	return out
}

// discoverIvars dry-runs struct class method bodies to learn ivar types
// from assignments.
func (c *Compiler) discoverIvars() {
	c.untypedIvars = nil
	c.ivarDeclWhy = "a gem module's ivar, laid out untyped before any body is generated (decision 147): "
	c.declareGemModuleIvars()
	c.ivarDeclWhy = ""
	c.declareComputedIvars()
	c.walkIvarClasses()
	for _, d := range c.classBodyLits {
		if k := d.cls.meta; k != nil && c.findIvar(k, d.name) == nil {
			c.declareIvar(k, d.name, c.resolveType(d.rbs, typeScope{class: d.cls, lex: d.scope, srcPos: srcPos{d.cls.File, d.line}}), d.cls.File, d.line)
		}
	}
	for _, k := range slices.SortedFunc(maps.Keys(c.untypedIvars), func(a, b ivarKey) int {
		return cmp.Or(cmp.Compare(a.cls.Name, b.cls.Name), cmp.Compare(a.name, b.name))
	}) {
		if p := c.untypedIvars[k]; c.findIvar(k.cls, k.name) == nil { // no typed write: untyped, as its first write declared it before (decision 392)
			c.ivarDeclWhy = "every write stores a value read back out of untyped storage (an element of an untyped container; decision 392): "
			c.declareIvar(k.cls, k.name, TAny{Unknown: c.unknownNow()}, p.File, p.Line) // in a round, maybe only waiting for a parameter's type
		}
	}
	c.ivarDeclWhy = ""
	c.untypedIvars = nil
	c.ivarDeclWhy = "an attr writer no call types and no other write (decision 445): "
	c.typeUncalledWriters()
	c.ivarDeclWhy = ""
	for _, g := range c.gemNilIvars { // only ever nil in what is compiled (a gem's pruned writes, decision 169; a user's nil-only ivar, 183)
		if c.findIvar(g.cls, g.name) == nil {
			c.warnAt(g.srcPos, "instance variable %s is only ever nil, so it is untyped (`# @rbs %s: T` for static calls; decision 183)", g.name, g.name)
			c.ivarDeclWhy = "only ever assigned nil in what is compiled (decisions 169, 183): "
			c.declareIvar(g.cls, g.name, TAny{Unknown: c.unknownNow()}, g.File, g.Line)
		}
	}
	c.ivarDeclWhy = "discovery saw no write that types it (the gem's methods only read it, or its writes did not compile; decision 220): "
	c.declareGemClassIvars()
	c.ivarDeclWhy = ""
}

// walkIvarClasses dry-runs every class's methods that name an ivar, so each write declares its ivar.
func (c *Compiler) walkIvarClasses() {
	walks := 2
	if c.env.InferUnknown {
		walks = 3 // a type crosses one class per walk: Lexer reads LexerStrings' ivars through its methods' results
	}
	c.discovering = true
	for range walks {
		for _, cls := range c.classList {
			for _, iv := range cls.Ivars {
				iv.elems = [2][]Type{} // the last round's, when every ivar has a type
			}
		}
		// class objects first: an instance method's `@cs = self.class.lex_en_start` reads a class-level accessor, whose
		// ivar must have its type by then or @cs is declared untyped for good (Ragel's generated state constants)
		order := slices.Clone(c.classList)
		slices.SortStableFunc(order, func(a, b *Class) int {
			switch {
			case a.metaOf != nil && b.metaOf == nil:
				return -1
			case a.metaOf == nil && b.metaOf != nil:
				return 1
			}
			return 0
		})
		for _, cls := range order {
			module := cls.IsModule && len(cls.TypeParams) == 0 && !cls.universal // a module's ivars are discovered from its own methods too (decision 317)
			if !cls.isStruct() && !cls.hashBase && !module || cls.universal || cls.isTemplate() {
				continue
			}
			ms := append([]*Method(nil), cls.MethodList...)
			for i, m := range ms {
				if m.Name == "initialize" && i > 0 {
					ms[0], ms[i] = ms[i], ms[0]
				}
			}
			for _, m := range ms {
				if m.Kind == kindDef && mentionsIvar(m) {
					_ = catchCompileError(func() { c.discoverMethod(cls, m) }) // a dry run: emitMethod reports errors
				}
			}
		}
	}
	c.discovering = false
}

// typeUncalledWriters gives each attr writer no call types its ivar's type from the ivar's other writes; with none,
// the writer declares it untyped as before (decision 445).
func (c *Compiler) typeUncalledWriters() {
	for _, m := range slices.SortedFunc(maps.Keys(c.uncalledWriters), func(a, b *Method) int {
		return cmp.Or(cmp.Compare(a.Owner.Name, b.Owner.Name), cmp.Compare(a.Name, b.Name))
	}) {
		k := c.uncalledWriters[m]
		iv := c.findIvar(k.cls, k.name)
		if iv == nil {
			iv = c.declareIvar(k.cls, k.name, TAny{}, m.File, m.File.line(m.Node.GetLocation().StartOffset))
		}
		m.Params[0].Type = iv.Type
	}
	c.uncalledWriters = nil
}

// declareGemClassIvars declares, untyped, each instance variable a gem class's own methods name that discovery left
// without a type (decision 220): before any struct is laid out, as declareGemModuleIvars does for a gem module's.
func (c *Compiler) declareGemClassIvars() {
	if !c.pruneGems {
		return
	}
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal || !cls.isStruct() {
			continue
		}
		for _, m := range cls.MethodList {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) {
				continue
			}
			c.eachIvarName(m.Node.Body, func(name string, n parser.Node) {
				if c.findIvar(cls, name) == nil {
					c.declareIvar(cls, name, TAny{Unknown: c.unknownNow()}, m.File, m.File.line(n.GetLocation().StartOffset))
				}
			})
		}
	}
}

// ivarNodeName is the ivar n reads or writes in any form (a multi-assign target too), and whether it writes; "" for other nodes.
func ivarNodeName(n parser.Node) (name string, write bool) {
	switch x := n.(type) {
	case *parser.InstanceVariableReadNode:
		return x.Name, false
	case *parser.InstanceVariableWriteNode:
		return x.Name, true
	case *parser.InstanceVariableTargetNode:
		return x.Name, true
	case *parser.InstanceVariableOrWriteNode:
		return x.Name, true
	case *parser.InstanceVariableAndWriteNode:
		return x.Name, true
	case *parser.InstanceVariableOperatorWriteNode:
		return x.Name, true
	}
	return "", false
}

// eachIvarName calls fn for each instance variable n reads or writes, outside nested defs and class bodies. The nodes
// are found once per body and pass: includerClassWrites asks about every method of every includer for each ivar a
// module names, which walked rubocop's bodies for 4% of its compile.
func (c *Compiler) eachIvarName(n parser.Node, fn func(string, parser.Node)) {
	if n == nil {
		return
	}
	nodes, ok := c.ivarNodes[n]
	if !ok {
		walkNode(n, func(x parser.Node) (bool, bool) {
			if opensScope(x) {
				return false, false
			}
			if name, _ := ivarNodeName(x); name != "" {
				nodes = append(nodes, x)
			}
			return false, true
		})
		if c.ivarNodes == nil {
			c.ivarNodes = map[parser.Node][]parser.Node{}
		}
		c.ivarNodes[n] = nodes
	}
	for _, x := range nodes {
		name, _ := ivarNodeName(x)
		fn(name, x)
	}
}

// gemNilIvar is a gem class's ivar discovery saw assigned only nil (decision 169).
// pendingIvar is an ivar discovery has seen only untyped writes to (decision 392): its stand-in and first write.
type pendingIvar struct {
	iv *Ivar
	srcPos
}

// pendingIvar finds cls's (or an ancestor's) stand-in for an ivar only untyped values were written to.
func (c *Compiler) pendingIvar(cls *Class, name string) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if p := c.untypedIvars[ivarKey{k, name}]; p != nil {
			return p.iv
		}
	}
	return nil
}

type gemNilIvar struct {
	cls  *Class
	name string
	srcPos
}

// includerClassWrites reports whether every class that includes mod has a superclass chain assigning name in a def:
// only then is the class's field the one storage (elftools' Dynamic, included by DynamicSegment < Segment).
func (c *Compiler) includerClassWrites(mod *Class, name string) bool {
	any := false
	for _, inc := range mod.includers {
		if inc.IsModule {
			continue
		}
		writes := false
		for k := inc; k != nil && !writes; k = k.Super {
			for _, m := range k.MethodList {
				if m.Node == nil || m.Owner != k {
					continue
				}
				c.eachIvarName(m.Node.Body, func(n string, x parser.Node) {
					if _, w := ivarNodeName(x); w && n == name {
						writes = true
					}
				})
			}
		}
		if !writes {
			return false
		}
		any = true
	}
	return any
}

// declareGemModuleIvars declares, untyped, each instance variable a gem module's methods use: its state struct and
// the includers' fields are laid out before any body is generated (decision 147; lenient, decision 172).
func (c *Compiler) declareGemModuleIvars() {
	if !c.pruneGems {
		return
	}
	for _, mod := range c.classList {
		if !mod.IsModule || mod.universal || len(mod.TypeParams) > 0 || mod.File == nil || !c.gemFile(mod.File) {
			continue
		}
		valueIncluded := slices.ContainsFunc(c.classList, func(k *Class) bool {
			return !k.IsModule && (!k.isStruct() || k.universal) && slices.Contains(k.allAncestors(), mod)
		})
		for _, m := range mod.MethodList {
			if m.Node == nil || !c.gemFile(m.File) {
				continue
			}
			c.eachIvarName(m.Node.Body, func(name string, n parser.Node) {
				switch {
				case c.findIvar(mod, name) != nil:
				case c.includerClassWrites(mod, name): // the includer's class keeps it (elftools' Dynamic reading Segment's @offset_from_vma): read by name (decision 324)
				case valueIncluded: // Pathname has no struct to hold it: memoizing recomputes per call (decision 172)
					if mod.scratchIvars == nil {
						mod.scratchIvars = map[string]*Ivar{}
					}
					if mod.scratchIvars[name] == nil {
						mod.scratchIvars[name] = &Ivar{Name: name, Type: TAny{}, Owner: mod, scratch: true}
					}
				default:
					c.declareIvar(mod, name, TAny{}, m.File, m.File.line(n.GetLocation().StartOffset))
				}
			})
		}
	}
}

// mentionsIvar reports whether m's body names an instance variable or sets one by name: only those bodies tell the
// discovery dry run anything, and brew's two rounds over every body were 29% of its generation CPU.
func mentionsIvar(m *Method) bool {
	if m.ivarUse != nil {
		return *m.ivarUse
	}
	found := false
	if m.Node != nil {
		found = walkNode(m.Node.Body, func(n parser.Node) (bool, bool) {
			if name, _ := ivarNodeName(n); name != "" {
				return true, false
			}
			call, ok := n.(*parser.CallNode)
			return ok && (call.Name == "instance_variable_set" || call.Name == "instance_variable_get"), true
		})
	}
	m.ivarUse = &found
	return found
}

func (c *Compiler) discoverMethod(cls *Class, m *Method) {
	f := c.newFctx(m.File, cls, m)
	f.discover = true
	f.retVar = "ret_"
	t := tail{kind: tailReturn, typ: m.Ret}
	if m.Iterator {
		t = tail{}
	}
	f.genBody(m.Node.Body, c.paramLocals(m), t, nil)
}

// initWrites: a non-nil ivar initialize's statements set is no read-first nil for methods initialize cannot run (decision 394).
type initWrites struct {
	cls         *Class
	ivars, runs map[string]bool
}

func (c *Compiler) initWritesOf(cls *Class) *initWrites {
	if w := c.initWrites[cls]; w != nil {
		return w
	}
	w := &initWrites{cls: cls, ivars: map[string]bool{}, runs: map[string]bool{"initialize": true}}
	if c.initWrites == nil {
		c.initWrites = map[*Class]*initWrites{}
	}
	c.initWrites[cls] = w
	for k := cls; k != nil; k = k.Super {
		w.run(k.Methods["initialize"])
	}
	w.settle(cls.Methods["initialize"], map[*Method]bool{})
	return w
}

// find is the method name self calls in cls.
func (w *initWrites) find(name string) *Method {
	for k := w.cls; k != nil; k = k.Super {
		if m := k.Methods[name]; m != nil {
			return m
		}
	}
	return nil
}

// run notes every method m can call on self, transitively.
func (w *initWrites) run(m *Method) {
	if m == nil || m.Node == nil {
		return
	}
	walkNode(m.Node.Body, func(n parser.Node) (bool, bool) {
		if call, ok := n.(*parser.CallNode); ok && selfCall(call) && !w.runs[call.Name] {
			w.runs[call.Name] = true
			w.run(w.find(call.Name))
		}
		return false, !opensScope(n)
	})
}

// settle notes the ivars m's statements set to a non-nil value, and those of the self calls that are statements.
func (w *initWrites) settle(m *Method, seen map[*Method]bool) {
	if m == nil || m.Node == nil || seen[m] {
		return
	}
	seen[m] = true
	body, ok := m.Node.Body.(*parser.StatementsNode)
	if !ok {
		return
	}
	for _, st := range body.Body {
		switch st := st.(type) {
		case *parser.InstanceVariableWriteNode:
			var names []string
			var v parser.Node = st
			for iw, ok := v.(*parser.InstanceVariableWriteNode); ok; iw, ok = v.(*parser.InstanceVariableWriteNode) {
				names, v = append(names, iw.Name), iw.Value // `@a = @b = 0` writes both
			}
			if nonNilValue(v) { // `@tree = opts[:tree]` may store nil: a read after it may see nil
				for _, name := range names {
					w.ivars[name] = true
				}
			}
		case *parser.CallNode:
			if st.Receiver == nil && st.Block == nil {
				w.settle(w.find(st.Name), seen)
			}
		}
	}
}

// nonNilValue is a value that is never nil: a literal other than nil, or a constant's `new`.
func nonNilValue(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.IntegerNode, *parser.FloatNode, *parser.StringNode, *parser.InterpolatedStringNode, *parser.SymbolNode,
		*parser.TrueNode, *parser.FalseNode, *parser.ArrayNode, *parser.HashNode:
		return true
	case *parser.CallNode:
		switch n.Receiver.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			return n.Name == "new"
		}
	}
	return false
}

// selfCall is a call on self, implicit or explicit.
func selfCall(n *parser.CallNode) bool {
	_, self := n.Receiver.(*parser.SelfNode)
	return n.Receiver == nil || self
}
