package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// markSuperBridges flags module methods whose `super` finds nothing in the
// module itself: the target is whatever follows the module in each
// includer's ancestors. Object's fallbacks (decision 31) stay static.
func (c *Compiler) markSuperBridges() {
	for _, mod := range c.classList {
		if !mod.IsModule || mod.universal {
			continue
		}
		for _, m := range mod.MethodList {
			if m.Kind != kindDef || m.Block != nil || m.generic() || m.Name == "method_missing" || m.Name == "respond_to_missing?" {
				continue
			}
			m.superBridge = containsSuper(m.Node.Body) && c.inheritedSig(m) == nil
			if m.superBridge && m.Name == "initialize" && !c.hasNextDef(m) {
				m.superBridge = false // super reaches Object#initialize in every includer: a no-op, whatever it passes
			}
		}
	}
}

// hasNextDef reports whether some class including m's module defines m's name after it in its lookup order.
func (c *Compiler) hasNextDef(m *Method) bool {
	for _, cls := range c.classList {
		if cls.IsModule {
			continue
		}
		defs := cls.defsOf(m.Name)
		for i, e := range defs {
			if e.M == m {
				if i+1 < len(defs) {
					return true
				}
				break
			}
		}
	}
	return false
}

// linkOverrides names the Go slot each override of a struct class fills. An
// override with the parent's Go signature takes the parent's Go name; one
// that differs (another arity, a narrower return) gets its own name, and the
// class keeps answering to the parent's slot through an adapter (decision 8).
func (c *Compiler) linkOverrides(cls *Class) {
	sup := cls.Super
	if cls.slotsLinked || !cls.isStruct() || cls.universal || sup == nil || sup.universal {
		return
	}
	cls.slotsLinked = true
	c.linkOverrides(sup)
	for _, m := range cls.MethodList {
		pe := sup.lookup(m.Name)
		if pe == nil || !inInterface(m) || !inInterface(pe.M) || m.seqAdapter != pe.M.seqAdapter {
			continue
		}
		own := entry{M: m, Owner: cls, Env: map[string]Type{}, Entry: cls}
		if c.slotKey(slot{e: own, in: cls}) == c.slotKey(slot{e: *pe, in: sup}) {
			m.GoName = pe.M.GoName
			m.shadowed = pe.M.shadowed
			continue
		}
		// `_` + lowercase never comes out of camel-casing (decision 3)
		m.GoName = goMethodName(m.Name) + "_of" + cls.Name
		m.shadowed = append([]slot{{e: *pe, in: sup}}, pe.M.shadowed...)
		kept := m.shadowed[:0]
		for _, s := range m.shadowed {
			c.adoptSlotBlock(m, s)
			if c.gemFile(m.File) && catchCompileError(func() { c.checkAdaptable(slot{e: own, in: cls}, s) }) != nil {
				if c.dynAdaptable(m, s, cls) {
					s.dyn = true
					kept = append(kept, s)
				}
				continue
			}
			c.checkAdaptable(slot{e: own, in: cls}, s)
			kept = append(kept, s)
		}
		m.shadowed = kept
	}
}

// inInterface reports whether m is a slot of its owner's Go interface.
func inInterface(m *Method) bool {
	return !staticPrivate(m) && !m.generic() && m.Name != "initialize" && (m.Owner.metaOf == nil || m.Name != "new")
}

// includerDefines reports whether a class including module c (or a module including it), or a subclass of one,
// defines name: a module_function's call to another dispatches on self, so CurlDownloadStrategy's `curl` override
// runs from Utils::Curl#curl_download, as MRI's (decision 296).
func (c *Class) includerDefines(name string, seen map[*Class]bool) bool {
	for _, inc := range c.includers {
		if seen[inc] {
			continue
		}
		seen[inc] = true
		if inc.Methods[name] != nil || inc.descendantDefines(name, true) || inc.includerDefines(name, seen) {
			return true
		}
	}
	return false
}

// staticPrivate: a private method no subclass overrides, called directly. One a subclass overrides still dispatches
// on self, as MRI's (concurrent-ruby's private __initialize_atomic_fields__, which attr_atomic redefines per class;
// decision 253).
func staticPrivate(m *Method) bool {
	if !m.Private || m.Owner == nil {
		return m.Private
	}
	if m.staticPriv != 0 {
		return m.staticPriv == 1
	}
	r := staticPrivate0(m)
	if m.Owner.shapeFixed { // includerDefines walks every includer's subclasses: rubocop's mixins have hundreds (decision 444)
		m.staticPriv = 2
		if r {
			m.staticPriv = 1
		}
	}
	return r
}

func staticPrivate0(m *Method) bool {
	if m.Owner.descendantDefines(m.Name, true) || !m.Owner.universal && m.Owner.includerDefines(m.Name, map[*Class]bool{}) {
		return false
	}
	if sup := m.Owner.Super; sup != nil { // an override of a dispatched private method: its slot
		if pe := sup.lookup(m.Name); pe != nil && pe.M.Private && !staticPrivate(pe.M) {
			return false
		}
	}
	for _, inc := range m.Owner.Includes { // or of an included module's (UnpackStrategy::GenericUnar#extract_to_dir)
		if inc.Mod == nil {
			continue
		}
		if pe := inc.Mod.lookup(m.Name); pe != nil && pe.M.Private && pe.M != m && !staticPrivate(pe.M) {
			return false
		}
	}
	return true
}

// inheritRedeclaredReaders drops a gem subclass's attr_reader that its superclass already has (Sorbet sigs narrow its
// type, `returns(NilClass)` over T?): one field and one reader serve the hierarchy, as one ivar does in MRI.
func (c *Compiler) inheritRedeclaredReaders() {
	for _, cls := range c.classList {
		if cls.Super == nil || cls.Super.universal || !c.gemFile(cls.File) {
			continue
		}
		for _, m := range slices.Clone(cls.MethodList) {
			if m.Kind != kindAttrReader || !c.gemFile(m.File) {
				continue
			}
			if e := cls.Super.lookup(m.Name); e != nil && e.M.Kind == kindAttrReader && e.M.Attr == m.Attr {
				delete(cls.Methods, m.Name)
				cls.MethodList = deleteMethod(cls.MethodList, m)
				cls.ivarDecls = slices.DeleteFunc(cls.ivarDecls, func(d ivarDecl) bool { return d.name == m.Attr })
			}
		}
		cls.msetCache, cls.msetIndex = nil, nil
	}
}

// blockEachOfEnumerable is an Enumerable includer's `each` written with a block and returning self (Ruby's idiom)
// rather than as an iterator: Enumerable's methods reach it through the seqAdapter iterator.
func (c *Compiler) blockEachOfEnumerable(cls *Class, m *Method) bool {
	if m.Name != "each" || m.Block == nil || m.Iterator || len(m.Block.Params) != 1 || cls.IsModule || m.File == nil || m.File.isPrelude() {
		return false
	}
	enum := c.classes["Enumerable"]
	return enum != nil && slices.Contains(cls.ancestors(), enum)
}

// restEachOfEnumerable reports an iterator `each(*args) { }` under Enumerable, whose slot is `each()`: the iterator
// keeps its rest under GoName_rest and a no-argument adapter answers the slot, as seqAdapter does for a closure override.
func (c *Compiler) restEachOfEnumerable(cls *Class, m *Method) bool {
	if m.Name != "each" || !m.Iterator || cls.IsModule || len(m.Params) == 0 || !m.Params[len(m.Params)-1].Rest {
		return false
	}
	for sup := cls.Super; sup != nil; sup = sup.Super {
		if sup.lookup("each") != nil && sup.lookup("each").Owner == sup {
			return false // a superclass's each is the slot it answers: shadowSlot adapts it (decision 206, ruby/spec's EachCounter)
		}
	}
	enum := c.classes["Enumerable"]
	return enum != nil && slices.Contains(cls.ancestors(), enum)
}

// relinkOverrides links, in gem code, the overrides only return inference tells apart from their parent's slot
// (linkOverrides runs before decision 36), and overrides of an included module's method, which the module's Self
// constraint calls through its own signature: each gets an adapter for the slot it no longer answers.
func (c *Compiler) relinkOverrides() {
	c.typeAbstracts()
	done := map[*Class]bool{}
	var link func(cls *Class)
	link = func(cls *Class) {
		if done[cls] {
			return
		}
		done[cls] = true
		if cls.Super != nil {
			link(cls.Super)
		}
		if !cls.isStruct() && !cls.hashBase || cls.universal {
			return
		}
		for _, m := range cls.MethodList {
			if m.Kind != kindDef || !inInterface(m) || m.seqAdapter || !c.gemFile(m.File) {
				continue
			}
			own := slot{e: entry{M: m, Owner: cls, Env: map[string]Type{}, Entry: cls}, in: cls}
			for _, s := range c.parentSlots(cls, m) {
				if !inInterface(s.e.M) || s.e.M.seqAdapter || slices.ContainsFunc(m.shadowed, func(x slot) bool { return x.e.M == s.e.M }) {
					continue
				}
				c.shadowSlot(m, own, s, cls.Name)
			}
		}
		c.relinkModuleOverrides(cls)
	}
	for _, cls := range c.classList {
		link(cls)
	}
}

// relinkModuleOverrides: a module's method that overrides a superclass method of an includer with another Go
// signature (concurrent-ruby's ConditionSignalling#ns_broadcast returning self over AbstractLockableObject's) gets its
// own Go name, and the includer answers the superclass's slot through an adapter, as a class's own override does
// (decision 249). The name is the module's, shared by its includers. A program's own module whose override no adapter
// fits is a compile error, as a class's is, where it used to reach `go build` as a type error (decision 408).
func (c *Compiler) relinkModuleOverrides(cls *Class) {
	if cls.Super == nil {
		return
	}
	for _, inc := range cls.Includes {
		mod := inc.Mod
		if mod == nil || !c.gemFile(mod.File) && !c.userFile(mod.File) {
			continue
		}
		for _, m := range mod.MethodList {
			if m.Kind != kindDef || !inInterface(m) || m.seqAdapter || !c.gemFile(m.File) && !c.userFile(m.File) {
				continue
			}
			pe := cls.Super.lookup(m.Name)
			oe := cls.lookup(m.Name)
			if pe == nil || oe == nil || oe.M != m || pe.M == m || !inInterface(pe.M) || pe.M.seqAdapter {
				continue
			}
			if pe.Owner.IsModule && (pe.Owner == mod || pe.Owner.universal || pe.Owner.File == nil || pe.Owner.File.isPrelude()) {
				continue // a gem or user module a superclass includes is a slot to shadow (Util#indent under Cop::Base, by ConditionalAssignmentHelper#indent); the prelude's (Kernel#to_json) are not
			}
			ps := slot{e: *pe, in: cls.Super}
			if slices.ContainsFunc(m.shadowed, func(x slot) bool { return x.e.M == pe.M }) {
				continue
			}
			if !c.gemFile(m.File) {
				c.checkAdaptable(slot{e: *oe, in: cls}, ps)
			}
			c.shadowSlot(m, slot{e: *oe, in: cls}, ps, mod.Name)
		}
	}
}

// parentSlots are the interface slots m's name has above cls: the superclass's method and what it answers, then each
// module cls itself includes.
func (c *Compiler) parentSlots(cls *Class, m *Method) []slot {
	var out []slot
	if sup := cls.Super; sup != nil && !sup.universal {
		if pe := sup.lookup(m.Name); pe != nil {
			in := sup
			if cls.hashBase && sup.GoType != "" && !sup.hashBase {
				in = cls // a core container's own slot, typed as the subclass embeds it (containerEnv)
			}
			out = append(append(out, slot{e: *pe, in: in}), pe.M.shadowed...)
		}
	}
	for _, inc := range cls.Includes {
		mod := inc.Mod
		if mod == nil || len(inc.Args) != len(mod.TypeParams) || mod.Methods[m.Name] == nil || mod.Methods[m.Name].Kind != kindDef {
			continue
		}
		if len(out) > 0 && mod.Methods[m.Name].GoName == out[0].e.M.GoName {
			continue // the superclass's slot owns the Go name; an included module's same-named method cannot also be answered under it (Subexpression#to_h: Base's, not Enumerable's); one renamed `_ofM` is its own slot (MethodDispatchNode#receiver under LambdaNode's)
		}
		env := map[string]Type{}
		for i, tp := range mod.TypeParams {
			env[tp] = inc.Args[i]
		}
		out = append(out, slot{e: entry{M: mod.Methods[m.Name], Owner: mod, Env: env, Entry: cls}, in: cls})
	}
	// and each module those include (rubocop's LineLength#register_offense under Alignment's, which LineLengthHelp
	// includes): the constraint of the module between declares the name too (decision 444)
	seen := map[*Class]bool{}
	var nested func(k *Class, top bool)
	nested = func(k *Class, top bool) {
		for _, inc := range k.Includes {
			mod := inc.Mod
			if mod == nil || seen[mod] {
				continue
			}
			seen[mod] = true
			if pm := mod.Methods[m.Name]; !top && pm != nil && pm.Kind == kindDef && len(mod.TypeParams) == 0 &&
				!slices.ContainsFunc(out, func(x slot) bool { return x.e.M == pm || x.e.M.GoName == pm.GoName }) {
				out = append(out, slot{e: entry{M: pm, Owner: mod, Env: map[string]Type{}, Entry: cls}, in: cls})
			}
			nested(mod, false)
		}
	}
	nested(cls, true)
	return out
}

// publicGemOverrides keeps a gem's private override of an ancestor's public method in the Go interface
// (SystemCommand's private `debug?` over Context's, which Context's methods call on self). MRI differs only for a
// caller outside the class, where the private method raises NoMethodError (decision 172).
func (c *Compiler) publicGemOverrides() {
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if !m.Private || m.Kind != kindDef || !c.gemFile(m.File) || rubyPrivate[m.Name] || m.Name == "initialize" {
				continue
			}
			for _, a := range cls.allAncestors() {
				if pm := a.Methods[m.Name]; pm != nil && !pm.Private {
					m.Private = false
					break
				}
				if pm := a.Methods[m.Name]; pm != nil && pm.Kind == kindDef && a.IsModule && !a.universal && pm.File != nil && c.gemFile(pm.File) {
					// both private, and the module's own methods call it on self (rubocop's Alignment#register_offense(node,
					// message_node) under RedundantLineBreak's register_offense(node)): the includer answers the module's
					// slot through an adapter, which needs both in the interface (decision 444)
					m.Private, pm.Private = false, false
					break
				}
			}
		}
	}
}

// sameParamShape: a and b take the same parameters by count and kind (positional, optional, rest, keyword).
func sameParamShape(a, b *Method) bool {
	if len(a.Params) != len(b.Params) {
		return false
	}
	for i := range a.Params {
		p, q := a.Params[i], b.Params[i]
		if p.Rest != q.Rest || (p.Default == nil) != (q.Default == nil) || p.Keyword != q.Keyword || p.KwRest != q.KwRest {
			return false
		}
	}
	return true
}

// dynAdaptable reports a gem override m no typed adapter fits to slot s (TimerSet#post(delay, *args, &task) over
// RubyExecutorService#post(*args, &task)) that a dynamic one can serve: the slot boxes its positional arguments and an
// untyped block, and calls m through its Dyn wrapper, as MRI calls the override (decision 249).
func (c *Compiler) dynAdaptable(m *Method, s slot, cls *Class) bool {
	sm := s.e.M
	if m.Iterator || sm.Iterator || sm.calleeDefaults || sm.kwMask() || staticPrivate(m) || sm.generic() {
		return false
	}
	// a module cls includes itself: its slot is its Self constraint's, which names the module's own signature (one a
	// superclass includes is that class's interface slot); `<=>`'s Op_cmp slot has emitCmpAdapter
	if s.e.Owner != nil && s.e.Owner.IsModule && s.in == cls && sm.GoName == m.GoName || sm.GoName == "Op_cmp" || m.GoName == "cmpNil" {
		return false // under another Go name (the module's own `_of` one) the adapter answers the constraint (LambdaNode#receiver under MethodDispatchNode's)
	}
	if slices.ContainsFunc(sm.Params, func(p Param) bool { return p.Keyword || p.KwRest }) {
		return false
	}
	typedBlock := func(b *BlockSig) bool { return b != nil && (b.Self != nil || b.Rest && !anyRestBlock(b)) }
	if typedBlock(sm.Block) || typedBlock(m.Block) {
		return false
	}
	if m.Block != nil {
		if c.dynBlockNames == nil {
			c.dynBlockNames = map[string]bool{}
		}
		c.dynBlockNames[m.Name] = true
	}
	return true
}

// shadowSlot has m (own) answer slot s it overrides with another Go signature: m takes its own Go name (`_of` + of)
// and s an adapter, a dynamic one when no typed adapter fits (decision 249); nothing when neither does.
func (c *Compiler) shadowSlot(m *Method, own, s slot, of string) {
	c.adoptSlotBlock(m, s)
	same := s.e.M.GoName == m.GoName
	if same && c.slotKey(own) == c.slotKey(s) {
		return
	}
	if catchCompileError(func() { c.checkAdaptable(own, s) }) != nil {
		switch {
		case c.dynAdaptable(m, s, own.in):
			s.dyn = true
		case c.gemFile(m.File) && s.e.M.GoName != "Op_cmp" && m.GoName != "cmpNil": // `<=>` has emitCmpAdapter
			s.raise = true // rubocop's ConditionalAssignmentHelper#indent(cop, source) under Util#indent(node, offset: 0): the slot's call would be MRI's ArgumentError
		default:
			return
		}
	}
	if same {
		m.GoName = goMethodName(m.Name) + "_of" + of
	}
	m.shadowed = append(m.shadowed, s)
}

// adoptSlotBlock: a gem override whose block its callers typed, where the slot's is the untyped (*untyped) -> untyped
// (concurrent-ruby's IVar#add_observer over Observable's, TimerSet#post over RubyExecutorService's), takes the slot's,
// which is wider, so it keeps or adapts the slot (decision 250).
func (c *Compiler) adoptSlotBlock(m *Method, s slot) {
	sb := s.e.M.Block
	if sb == nil || m.Block == nil || !anyRestBlock(sb) || anyRestBlock(m.Block) || m.Iterator || s.e.M.Iterator || m.Block.Self != nil || !c.gemFile(m.File) {
		return
	}
	b := *sb
	b.Optional = m.Block.Optional
	m.Block = &b
}

// readsOnlyIvar reports a def whose body is the read of ivar name: an attr_reader as gem code compiles it.
func readsOnlyIvar(m *Method, name string) bool {
	if m.Node == nil || m.Node.Body == nil {
		return false
	}
	body := m.Node.Body
	if st, ok := body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
		body = st.Body[0]
	}
	r, ok := body.(*parser.InstanceVariableReadNode)
	return ok && r.Name == name
}
