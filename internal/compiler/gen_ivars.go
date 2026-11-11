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
				if t := refineContainer(iv.Type, iv.open, iv.elems); t != nil && typed[iv] == nil {
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
	base, _ := c.dryRunIvarUsers(cands)
	var pre map[nilSite]bool // the untyped run's nil sites: not a typing's (decision 440)
	preDone := false
	attempt := func(d *nilDeferral) (*compileError, bool) {
		c.nilDefer = d
		convs, err := c.dryRunIvarUsers(cands)
		ok := err == nil
		for k := range convs {
			ok = ok && base[k]
		}
		return err, ok
	}
	try := func(set map[*Ivar]bool) bool {
		for _, iv := range cands {
			iv.Type = orig[iv]
			if set[iv] {
				iv.Type = typed[iv]
			}
		}
		err, ok := attempt(nil)
		if ok || err == nil || !err.nilVeto || c.noNilDefer {
			return ok
		}
		if !preDone {
			preDone = true
			types := map[*Ivar]Type{}
			for _, iv := range cands {
				types[iv], iv.Type = iv.Type, orig[iv]
			}
			rec := &nilDeferral{rec: map[nilSite]bool{}}
			if err, _ := attempt(rec); err == nil {
				pre = rec.rec
			}
			for _, iv := range cands {
				iv.Type = types[iv]
			}
		}
		if pre == nil {
			c.nilDefer = nil
			return false
		}
		d := &nilDeferral{pre: pre, sites: map[nilSite]bool{}}
		if _, ok = attempt(d); !ok {
			c.nilDefer = nil
		}
		d.pre = nil // the emission's other sites are not the typing's
		return ok
	}
	salvage(cands, try) // a failed last try leaves the original types
	c.nilDeferUsed = c.nilDeferUsed || c.nilDefer != nil && len(c.nilDefer.sites) > 0
	if c.round || c.refineOut == nil {
		return
	}
	accepted := make([]bool, len(cands))
	for i, iv := range cands {
		accepted[i] = typeEq(iv.Type, typed[iv])
	}
	if rec := c.recordRefine(snap, sig, accepted); rec != nil {
		c.refineOut[kind] = rec
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
	err := catchCompileError(func() {
		for _, cls := range c.classList {
			if !slices.ContainsFunc(ivs, func(iv *Ivar) bool { return cls.isSubclassOf(iv.Owner) }) {
				continue
			}
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
	c.declareGemModuleIvars()
	c.declareComputedIvars()
	for range 2 {
		for _, cls := range c.classList {
			for _, iv := range cls.Ivars {
				iv.elems = [2][]Type{} // the last round's, when every ivar has a type
			}
		}
		for _, cls := range c.classList {
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
	for _, d := range c.classBodyLits {
		if k := d.cls.meta; k != nil && c.findIvar(k, d.name) == nil {
			c.declareIvar(k, d.name, c.resolveType(d.rbs, typeScope{class: d.cls, lex: d.scope, srcPos: srcPos{d.cls.File, d.line}}), d.cls.File, d.line)
		}
	}
	for _, k := range slices.SortedFunc(maps.Keys(c.untypedIvars), func(a, b ivarKey) int {
		return cmp.Or(cmp.Compare(a.cls.Name, b.cls.Name), cmp.Compare(a.name, b.name))
	}) {
		if p := c.untypedIvars[k]; c.findIvar(k.cls, k.name) == nil { // no typed write: untyped, as its first write declared it before (decision 392)
			c.declareIvar(k.cls, k.name, TAny{}, p.File, p.Line)
		}
	}
	c.untypedIvars = nil
	for _, g := range c.gemNilIvars { // only ever nil in what is compiled (a gem's pruned writes, decision 169; a user's nil-only ivar, 183)
		if c.findIvar(g.cls, g.name) == nil {
			c.warnAt(g.srcPos, "instance variable %s is only ever nil, so it is untyped (`# @rbs %s: T` for static calls; decision 183)", g.name, g.name)
			c.declareIvar(g.cls, g.name, TAny{}, g.File, g.Line)
		}
	}
	c.declareGemClassIvars()
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
			eachIvarName(m.Node.Body, func(name string, n parser.Node) {
				if c.findIvar(cls, name) == nil {
					c.declareIvar(cls, name, TAny{}, m.File, m.File.line(n.GetLocation().StartOffset))
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

// eachIvarName calls fn for each instance variable n reads or writes, outside nested defs and class bodies.
func eachIvarName(n parser.Node, fn func(string, parser.Node)) {
	walkNode(n, func(x parser.Node) (bool, bool) {
		if opensScope(x) {
			return false, false
		}
		if name, _ := ivarNodeName(x); name != "" {
			fn(name, x)
		}
		return false, true
	})
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
				eachIvarName(m.Node.Body, func(n string, x parser.Node) {
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
			eachIvarName(m.Node.Body, func(name string, n parser.Node) {
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
