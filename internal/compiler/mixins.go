package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// abstractStub reports a prelude module method whose body only raises NotImplementedError: a requirement the module
// types its own calls by (Comparable#<=>), which the includer must meet.
func abstractStub(m *Method) bool {
	if m.File == nil || !m.File.isPrelude() || m.Node == nil {
		return false
	}
	body := m.Node.Body
	if st, ok := body.(*parser.StatementsNode); ok && st != nil && len(st.Body) == 1 {
		body = st.Body[0]
	}
	c, ok := body.(*parser.CallNode)
	if !ok || c.Name != "raise" || c.Receiver != nil || len(callArgs(c)) != 1 {
		return false
	}
	k, ok := callArgs(c)[0].(*parser.ConstantReadNode)
	return ok && k.Name == "NotImplementedError"
}

// checkAbstractIncludes rejects an include whose module requires a method (Comparable's `<=>` as `(self) -> Integer`)
// the class inherits with another signature: Go could not build the module's calls on it. A compile error at the
// include, which a caller can act on, instead of a Go build error (decision 186).
func (c *Compiler) checkAbstractIncludes() {
	for _, cls := range c.classList {
		if cls.IsModule || cls.Super == nil || cls.File == nil || cls.File.isPrelude() {
			continue
		}
		for _, inc := range cls.Includes {
			if inc.Mod == nil {
				continue
			}
			for _, stub := range inc.Mod.MethodList {
				if !abstractStub(stub) || cls.Methods[stub.Name] != nil {
					continue
				}
				e := cls.Super.lookup(stub.Name)
				if e == nil || abstractStub(e.M) || e.M.File == nil || e.M.File.isPrelude() {
					continue
				}
				c.resolveMethod(e.M)
				c.resolveMethod(stub)
				ok := len(e.M.Params) == len(stub.Params) && e.M.Ret != nil && stub.Ret != nil && typeEq(e.M.Ret, stub.Ret)
				for i := 0; ok && i < len(e.M.Params); i++ {
					_, isClass := e.M.Params[i].Type.(TClass)
					ok = isClass
				}
				if !ok {
					c.errorAt(inc.srcPos, "include %s needs %s#%s typed as %s's (self) -> %s; it inherits %s's, typed otherwise: annotate it", inc.Mod.RubyName, cls.RubyName, stub.Name, inc.Mod.RubyName, stub.Ret, e.Owner.RubyName)
				}
			}
		}
	}
}

// includedExtends takes a module's `def self.included(base); base.extend(ClassMethods); end` as
// mixes_in_class_methods(ClassMethods): each includer extends it at link, and the run-time extend, which a closed world
// cannot do, is dropped from the hook (decision 241).
func (c *Compiler) includedExtends(f *File, mod *Class, body parser.Node, scope []*Class) {
	st, ok := body.(*parser.StatementsNode)
	if !ok {
		return
	}
	for _, x := range st.Body {
		d, ok := x.(*parser.DefNode)
		if !ok || d.Name != "included" || !isSelf(d.Receiver) || d.Parameters == nil || len(d.Parameters.Requireds) != 1 {
			continue
		}
		base, ok := d.Parameters.Requireds[0].(*parser.RequiredParameterNode)
		hook, ok2 := d.Body.(*parser.StatementsNode)
		if !ok || !ok2 {
			continue
		}
		for i, y := range hook.Body {
			call, ok := y.(*parser.CallNode)
			if !ok || call.Name != "extend" || len(callArgs(call)) == 0 {
				continue
			}
			r, ok := call.Receiver.(*parser.LocalVariableReadNode)
			if !ok || r.Name != base.Name || slices.ContainsFunc(callArgs(call), func(a parser.Node) bool {
				_, cr := a.(*parser.ConstantReadNode)
				_, cp := a.(*parser.ConstantPathNode)
				return !cr && !cp
			}) {
				continue
			}
			for _, a := range callArgs(call) {
				mod.mixesIn = append(mod.mixesIn, Include{ref: constRef{node: a, scope: scope, file: f}, srcPos: srcPos{f, f.line(call.Location.StartOffset)}, scope: scope})
			}
			hook.Body[i] = &parser.NilNode{Location: call.Location}
		}
	}
}

// classHook is a hook MRI calls while it evaluates a class body in main.rb:
// Super.inherited(cls) where the class is first opened, Mod.included(cls)
// and Mod.extended(cls) at the include/extend.
type classHook struct {
	name string
	cls  *Class
	mod  *constRef // nil: the superclass
	node parser.Node
	file *File  // where the hook site is, for its place in load order
	arg  string // method_added: the method's name, passed as a Symbol
}

// noteHooks records `include`/`extend`'s hooks: the last module first, as
// MRI includes them.
func (c *Compiler) noteHooks(f *File, name string, cls *Class, n *parser.CallNode, args []parser.Node, scope []*Class) {
	if f.isPrelude() {
		return
	}
	for _, a := range slices.Backward(args) {
		c.hooks = append(c.hooks, classHook{name: name, cls: cls, mod: &constRef{node: a, scope: scope, file: f}, node: n, file: f})
	}
}

func (c *Compiler) addInclude(f *File, n *parser.CallNode, cls *Class, a parser.Node, scope []*Class) {
	switch a.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
	default:
		c.errorf(f, a, "unsupported include argument")
	}
	inc := Include{ref: constRef{node: a, scope: scope, file: f}, srcPos: srcPos{f, f.line(n.Location.StartOffset)}, scope: scope}
	if t, ok := f.typeArgs[inc.Line]; ok {
		tup, err := rbs.ParseType(t)
		if err != nil {
			c.errorf(f, n, "bad include type args %q: %v", t, err)
		}
		tuple, ok := tup.(rbs.Tuple)
		if !ok {
			c.errorf(f, n, "bad include type args %q: want [T, ...]", t)
		}
		inc.args = tuple.Elems
	}
	cls.Includes = append(cls.Includes, inc)
}

// superclassOf resolves cls's superclass (nil for modules and BasicObject)
// and checks that every reopening that names one names the same class.
func (c *Compiler) superclassOf(cls *Class) *Class {
	var sup *Class
	switch {
	case cls.superInst != nil: // a generic superclass's instance (decision 309)
		sup = cls.superInst
	case cls.superRef != nil:
		sup = c.resolveClassRef(cls.superRef)
		if c.isNumericMod(sup) { // a user `< Numeric` class: an Object with Comparable, not one of the closed Numeric set (decision 365)
			if cls.File != nil && cls.File.isPrelude() {
				c.errorAt(cls.srcPos, "subclassing Numeric is not supported in the prelude (decision 142)")
			}
			sup = c.classes["Object"]
			if cmp := c.classes["Comparable"]; !slices.ContainsFunc(cls.Includes, func(i Include) bool { return i.Mod == cmp }) {
				ref := constRef{file: cls.File, node: &parser.ConstantReadNode{Name: "Comparable"}}
				cls.Includes = append(cls.Includes, Include{Mod: cmp, ref: ref, srcPos: cls.srcPos})
			}
		}
		if sup.IsModule {
			c.errorAt(cls.srcPos, "superclass %s is a module", sup.RubyName)
		}
		// a @go_type is a Go value type (string, []E), not a struct a subclass can embed
		switch {
		case sup.RubyName == "Hash" || sup.RubyName == "Array":
			// a Hash subclass embeds a Hash[any, V]: keys are untyped, since
			// the usual subclass downcases or otherwise rewrites them; an Array subclass embeds an Array[E]
			cls.hashBase = true
			switch {
			case sup.RubyName == "Array": // ponytail: elements untyped (`Array[any]`); a typed `class X < Array #[E]` needs generic method bodies over E
				cls.TypeParams = nil
				cls.GoType = "struct { " + superField(sup) + "[any] }"
			case len(cls.TypeParams) == 0:
				cls.TypeParams = []string{"V"}
			case len(cls.TypeParams) > 1:
				c.errorAt(cls.srcPos, "a Hash subclass takes one type parameter, the value type (keys are untyped)")
			}
			if sup.RubyName == "Hash" {
				cls.GoType = "struct { " + superField(sup) + "[" + strings.Join(cls.TypeParams, ", ") + "] }"
			}
		case sup.GoType != "" && !c.isNumericMod(sup) && (len(sup.TypeParams) == 0 || sup.mutable()):
			cls.hashBase = true // any other @go_type (Pathname; Set, Queue with untyped elements): embedded the same way (decision 202)
			cls.TypeParams = nil
			cls.GoType = "struct { " + superField(sup) + anyArgs(len(sup.TypeParams)) + " }"
		case sup.GoType != "":
			msg := fmt.Sprintf("subclassing %s is not supported (it is a @go_type class; hold one in an ivar instead)", sup.RubyName)
			if !c.gemFile(cls.File) {
				c.errorAt(cls.srcPos, "%s", msg)
			}
			c.deferClassError(cls, msg) // gem code: an error only where reachable code uses the class (decision 169)
			sup = c.classes["Object"]
		}
	case !cls.IsModule && cls.RubyName != "BasicObject":
		sup = c.classes["Object"]
	}
	for _, r := range cls.reSupers {
		if c.resolveClassRef(r) != sup {
			c.errorf(r.file, r.node, "class %s reopened with a different superclass", cls.RubyName)
		}
	}
	return sup
}

// checkCycles rejects `class A < B; end; class B < A; end` and mutually
// including modules: every later walk of Super/Includes (metaFor,
// isSubclassOf, hasClassMethod) would recurse until a fatal stack overflow.
func (c *Compiler) checkCycles() {
	for _, cls := range c.classList {
		chain := []string{cls.RubyName}
		for k, n := cls.Super, 0; k != nil && n <= len(c.classList); k, n = k.Super, n+1 { // n: a cycle not through cls is reported at its own members
			chain = append(chain, k.RubyName)
			if k == cls {
				c.errorAt(cls.srcPos, "class %s inherits from itself (%s)", cls.RubyName, strings.Join(chain, " < "))
			}
		}
	}
	state := map[*Class]int{} // 1 on the current path, 2 done
	var visit func(cls *Class, path []*Class)
	visit = func(cls *Class, path []*Class) {
		switch state[cls] {
		case 2:
			return
		case 1:
			i := slices.Index(path, cls)
			names := []string{}
			for _, k := range append(path[i:], cls) {
				names = append(names, k.RubyName)
			}
			c.errorAt(cls.srcPos, "module %s includes itself (%s)", cls.RubyName, strings.Join(names, " includes "))
		}
		state[cls] = 1
		for _, inc := range cls.Includes {
			if inc.Mod != nil {
				visit(inc.Mod, append(path, cls))
			}
		}
		state[cls] = 2
	}
	for _, cls := range c.classList {
		visit(cls, nil)
	}
}

// linkInclude resolves an include's module and its type args.
func (c *Compiler) linkInclude(cls *Class, inc *Include) {
	mod := c.resolveClassRef(&inc.ref)
	if c.isNumericMod(mod) && !inc.File.isPrelude() {
		c.errorAt(inc.srcPos, "Numeric is a class, not a module: a Numeric is one of Integer, Float, Rational, Complex and BigDecimal (decision 142)")
	}
	if !mod.IsModule {
		c.errorAt(inc.srcPos, "%s is not a module", mod.RubyName)
	}
	inc.Mod = mod
	mod.includers = append(mod.includers, cls)
	c.resolveMixinArgs(cls, cls, inc, "include")
}

// resolveMixinArgs types an include's or extend's module args; host supplies `each` and Sorbet Elem (the metaclass for an extend), cls the type scope.
func (c *Compiler) resolveMixinArgs(cls, host *Class, inc *Include, verb string) {
	mod := inc.Mod
	untyped := func() {
		for range mod.TypeParams {
			inc.Args = append(inc.Args, TAny{})
		}
	}
	if len(inc.args) == 0 && mod.RubyName == "Enumerable" {
		if at := c.sorbetElem(host); at != nil { // Sorbet's `Elem = type_member { { fixed: T } }` is Enumerable's T (decision 196)
			inc.args = []rbs.Type{at}
		}
	}
	if len(inc.args) == 0 && len(mod.TypeParams) > 0 && !inc.File.isPrelude() && host.Methods["each"] != nil {
		inc.fromEach = true // typed once each's signature is resolved (includeFromEach)
		untyped()
		return
	}
	// include: a pruned gem class's each (decision 169), or a user class with none of its own to infer from (decision 183); extend: any untyped `extend Enumerable`
	if len(inc.args) == 0 && (verb == "include" && (c.gemFile(inc.File) || !inc.File.isPrelude()) || verb == "extend" && len(mod.TypeParams) > 0) {
		if !c.gemFile(inc.File) && len(mod.TypeParams) > 0 { // a module with no type parameters has nothing untyped
			if verb == "include" {
				c.warnAt(inc.srcPos, "include %s is untyped: %s has no each to infer its element type from (`include %s #[...]` for static calls; decision 183)", mod.RubyName, cls.RubyName, mod.RubyName)
			} else {
				c.warnAt(inc.srcPos, "extend %s is untyped (`extend %s #[...]` for static calls; decision 183)", mod.RubyName, mod.RubyName)
			}
		}
		untyped()
		return
	}
	if len(inc.args) != len(mod.TypeParams) {
		c.errorAt(inc.srcPos, "%s %s needs %d type args (`%s %s #[...]`)", verb, mod.RubyName, len(mod.TypeParams), verb, mod.RubyName)
	}
	for _, a := range inc.args {
		inc.Args = append(inc.Args, c.resolveType(a, typeScope{class: cls, lex: inc.scope, srcPos: srcPos{inc.File, inc.Line}}))
	}
}

// pendingPrepend is `X.prepend(M)`, `X.singleton_class.prepend(M)` or a class body's `prepend M` (decision 199).
type pendingPrepend struct {
	target    *Class   // nil: resolve targetRef
	targetRef constRef // the receiver constant of a top-level prepend
	singleton bool     // X.singleton_class.prepend
	mod       constRef
}

// applyPrepends copies each prepended module's methods into its class, in load order: a method the class defines
// itself moves to a hidden private name that the copy's super calls, as MRI's ancestor order puts M before X.
func (c *Compiler) applyPrepends() {
	for i, p := range c.prepends {
		target := p.target
		if target == nil {
			target = c.resolveClassRef(&p.targetRef)
		}
		if p.singleton {
			target = c.metaFor(target)
		}
		mod := c.resolveClassRef(&p.mod)
		if !mod.IsModule {
			c.errorf(p.mod.file, p.mod.node, "prepend %s: not a module", mod.RubyName)
		}
		target.prepended = append(target.prepended, mod)
		for _, mm := range slices.Clone(mod.MethodList) {
			if mm.Kind != kindDef || mm.Node == nil {
				continue
			}
			cp := *mm
			cp.Owner = target
			if orig := target.Methods[mm.Name]; orig != nil {
				hidden := fmt.Sprintf("__prepended%d_%s", i, goMethodName(mm.Name))
				target.MethodList = deleteMethod(target.MethodList, orig)
				orig.Name, orig.GoName, orig.Private = hidden, hidden, true
				target.Methods[hidden] = orig
				target.MethodList = append(target.MethodList, orig)
				cp.superTarget = orig
			}
			target.Methods[cp.Name] = &cp
			target.MethodList = append(target.MethodList, &cp)
		}
		target.msetCache, target.msetIndex = nil, nil
	}
	for _, p := range c.prepends { // a module only prepended has no includer to compile its own copies (and their super) for
		mod := c.resolveClassRef(&p.mod)
		if !c.mixedIn(mod) {
			mod.prunedDefs = append(mod.prunedDefs, mod.MethodList...) // still the module's methods to reflection (decision 444)
			mod.Methods, mod.MethodList = map[string]*Method{}, nil
		}
	}
}

func (c *Compiler) mixedIn(mod *Class) bool {
	for _, cls := range c.classList {
		for _, inc := range append(slices.Clip(cls.Includes), cls.extends...) {
			if inc.Mod == mod {
				return true
			}
		}
	}
	return false
}

// prependTarget is the receiver of `X.prepend(M)` or `X.singleton_class.prepend(M)`, or nil for any other prepend
// (String#prepend).
func prependTarget(n *parser.CallNode) parser.Node {
	if len(callArgs(n)) == 0 {
		return nil
	}
	switch r := n.Receiver.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return r
	case *parser.CallNode:
		if r.Name == "singleton_class" && r.Arguments == nil {
			switch r.Receiver.(type) {
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				return r
			}
		}
	}
	return nil
}

// collectMixinCall is a class body's include, extend or prepend (decision 199: prepend applies at link).
func (c *Compiler) collectMixinCall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, args []parser.Node, scope []*Class) {
	switch n.Name {
	case "mixes_in_class_methods":
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
			last := len(cls.Includes) - 1
			cls.mixesIn = append(cls.mixesIn, cls.Includes[last])
			cls.Includes = cls.Includes[:last]
		}
	case "prepend":
		for _, a := range args {
			c.prepends = append(c.prepends, pendingPrepend{target: cls, mod: constRef{node: a, scope: scope, file: f}})
		}
	case "include":
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
			if r, ok := a.(*parser.ConstantReadNode); ok && r.Name == "Singleton" && !cls.IsModule {
				c.addSingletonInstance(ctx, f, n, cls, scope)
			}
		}
		c.noteHooks(f, "included", cls, n, args, scope)
	case "extend":
		args = slices.Clone(args)
		for i, a := range args {
			if self, ok := a.(*parser.SelfNode); ok && cls.IsModule { // `extend self`: the module's methods are its module methods too
				args[i] = &parser.ConstantReadNode{Name: cls.RubyName, Location: self.Location}
			}
		}
		for _, a := range args {
			c.addInclude(f, n, cls, a, scope)
			last := len(cls.Includes) - 1
			cls.extends = append(cls.extends, cls.Includes[last])
			cls.Includes = cls.Includes[:last]
		}
		for _, a := range args { // MRI's def_delegator is the most recently extended module's (decision 132)
			if r, ok := a.(*parser.ConstantReadNode); ok && (r.Name == "Forwardable" || r.Name == "SingleForwardable") {
				cls.fwdSingle = r.Name == "SingleForwardable"
				cls.singleFwd = cls.singleFwd || cls.fwdSingle
			}
		}
		c.noteHooks(f, "extended", cls, n, args, scope)
	}
}

// sorbetElem is the type of the includer's fixed Sorbet `Elem` type_member, written in its body or (for a
// `class << self`) its attached class's; nil when there is none.
func (c *Compiler) sorbetElem(cls *Class) rbs.Type {
	names := []string{cls.RubyName + "::Elem"}
	if cls.metaOf != nil {
		names = append(names, cls.metaOf.RubyName+"::Elem")
	}
	for _, n := range names {
		if k := c.consts[n]; k != nil && k.typeAlias != "" && k.typeAlias != "untyped" {
			at, err := rbs.ParseType(k.typeAlias)
			if err == nil {
				return at
			}
		}
	}
	return nil
}

// linkExtends resolves each `extend M` as an include of M on the class object.
func (c *Compiler) linkExtends() {
	for _, cls := range c.classList { // a module's mixes_in_class_methods(M): each includer extends M, as Sorbet's included hook does
		for _, inc := range cls.Includes {
			if mod := c.resolveClassRef(&inc.ref); mod != nil {
				cls.extends = append(cls.extends, mod.mixesIn...)
			}
		}
	}
	for _, cls := range c.classList {
		for _, ext := range cls.extends {
			ext.Mod = c.resolveClassRef(&ext.ref)
			if !ext.Mod.IsModule {
				c.errorAt(ext.srcPos, "%s is not a module", ext.Mod.RubyName)
			}
			c.resolveMixinArgs(cls, cls.meta, &ext, "extend") // `class << self; Elem = ...` and `def self.each` are the metaclass's
			cls.meta.Includes = append(cls.meta.Includes, ext)
		}
	}
}

// copyIncluderTemplates copies into each including class a module's unannotated method that calls, on self, a
// name only its includers define (pairing_heap's MergePairs#merge_pairs calling each heap's own meld): such a method
// is a template over its includers, whose parameters take a different type in each (their own Node). One shared free
// func would be untyped, and its self call dynamic (decision 307). The copy is MRI's method as the includer sees it: an
// includer that defines the name keeps its own; a method naming an ivar (the module's own state, decision 147) or
// calling super stays shared.
func (c *Compiler) copyIncluderTemplates() {
	obj := c.classes["Object"]
	for _, mod := range c.classList {
		if !mod.IsModule || mod.File == nil || mod.File.isPrelude() || len(mod.extends) > 0 {
			continue
		}
		var incs []*Class
		for _, cls := range mod.includers {
			if !cls.IsModule && cls.isStruct() && !cls.universal {
				incs = append(incs, cls)
			}
		}
		if len(incs) == 0 || len(incs) != len(mod.includers) {
			continue
		}
		var copied []*Method
		for _, mm := range slices.Clone(mod.MethodList) {
			if mm.Kind != kindDef || mm.Node == nil || mm.sigText != "" || mm.sig != nil || mentionsIvar(mm) || containsSuper(mm.Node.Body) ||
				!callsIncluderOnly(mm, mod, obj, incs, c.topDefs) {
				continue
			}
			// an includer's own def calls it through super (rubocop-ast's IfNode#modifier_form? over ModifierNode's): the
			// module keeps its own beside the other includers' copies (decision 444)
			supered := slices.ContainsFunc(incs, func(cls *Class) bool {
				own := cls.Methods[mm.Name]
				if own == nil || own.Node == nil {
					return false
				}
				// or overrides it with another parameter count (rubocop's RedundantLineBreak#register_offense(node) under
				// Alignment's (node, message_node)): the module's stays, as the slot the includer's adapter answers
				return containsSuper(own.Node.Body) || paramCount(own.Node) != paramCount(mm.Node)
			})
			for _, cls := range incs {
				if cls.Methods[mm.Name] != nil {
					continue
				}
				cp := *mm
				cp.Owner = cls
				cls.Methods[cp.Name] = &cp
				cls.MethodList = append(cls.MethodList, &cp)
				cls.msetCache, cls.msetIndex = nil, nil
			}
			if !supered {
				copied = append(copied, mm)
			}
		}
		for _, mm := range copied { // every includer has its copy: the module's own would be compiled for no caller
			delete(mod.Methods, mm.Name)
			mod.MethodList = deleteMethod(mod.MethodList, mm)
			mod.prunedDefs = append(mod.prunedDefs, mm) // still the module's method to reflection (decision 444)
		}
		if len(copied) > 0 {
			mod.msetCache, mod.msetIndex = nil, nil
		}
	}
}

// callsIncluderOnly reports whether m's body calls on self a name that neither m's module nor Object (nor a top-level
// def) answers and every includer does, with no block: a yielding includer method may be an iterator, losing the value.
func callsIncluderOnly(m *Method, mod, obj *Class, incs []*Class, top map[string]*Method) bool {
	found, blocked := false, false
	var walk func(parser.Node)
	walk = func(n parser.Node) {
		switch n := n.(type) {
		case nil, *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
			return
		case *parser.CallNode:
			if _, self := n.Receiver.(*parser.SelfNode); (n.Receiver == nil || self) && mod.lookup(n.Name) == nil &&
				(obj == nil || obj.lookup(n.Name) == nil) && top[n.Name] == nil &&
				!slices.ContainsFunc(incs, func(k *Class) bool { return k.lookup(n.Name) == nil }) {
				found, blocked = true, blocked || n.Block != nil
			}
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(m.Node.Body)
	return found && !blocked
}

// paramCount is how many positional parameters a def writes.
func paramCount(n *parser.DefNode) int {
	if n == nil || n.Parameters == nil {
		return 0
	}
	return len(n.Parameters.Requireds) + len(n.Parameters.Optionals) + len(n.Parameters.Posts)
}
