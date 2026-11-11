package compiler

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
)

// sendSpec is a type-switch dispatcher for a struct class's method that some subclass overrides (#96 phase 3b).
type sendSpec struct {
	owner *Class
	m     *Method
}

// sendName names the dispatcher of owner's method m.
func sendName(owner *Class, m *Method) string { return "rbSend_" + owner.Name + "_" + m.GoName }

// dispatchable reports whether a call of owner's method m on a receiver of static type owner can bypass Go method
// dispatch: a direct call to the free func when nothing below owner overrides it, else an rbSend_ type switch over the
// classes that do. Then no class needs a Go forwarder for the name and owner's interface does not declare it. Only the
// plain case qualifies: a struct class (not a module, metaclass or universal owner), a plainSelf public method
// (decision 210) nothing reaches through a Go method set (the std library, prelude Go's interface assertions, a
// universal protocol name, an including module's self call), and every override below with the same Go signature.
func (c *Compiler) dispatchable(owner *Class, m *Method) bool {
	if v, ok := c.dispatchMemo[m]; ok {
		return v
	}
	if c.dispatchMemo == nil {
		c.dispatchMemo = map[*Method]bool{}
	}
	c.dispatchMemo[m] = false // a cycle (constraintNamesSelf renders the constraint, which asks again) answers no
	pending := c.pendingSigs
	ok := c.dispatchableUncached(owner, m)
	c.dispatchMemo[m] = ok
	if c.pendingSigs != pending { // read a signature still being inferred: ask again once it is
		delete(c.dispatchMemo, m)
	}
	return ok
}

func (c *Compiler) dispatchableUncached(owner *Class, m *Method) bool {
	ok, why := c.dispatchableWhy(owner, m)
	if dbg := c.env.DispatchDebug; dbg != "" && owner != nil && (dbg == "all" || owner.universal || owner.IsModule) && !ok {
		fmt.Fprintf(os.Stderr, "dispatch-debug %s#%s: %s\n", owner.Name, m.Name, why)
	}
	return ok
}

func (c *Compiler) dispatchableWhy(owner *Class, m *Method) (bool, string) {
	if why := c.dispatchableOwner(owner, m); why != "" {
		return false, why
	}
	if owner.IsModule && !owner.universal {
		return c.moduleOverrides(owner, m)
	}
	ps, ret := c.sig(m, map[string]Type{})
	for _, k := range c.classList {
		if k.IsModule || k.universal || !k.isSubclassOf(owner) {
			continue
		}
		if !owner.universal && c.includerCalls(k, m.Name) {
			return false, "includerCalls" // a module body reaches it through its constraint interface (a universal name leaves the constraint instead)
		}
		impl := k.lookup(m.Name)
		if impl == nil || impl.M == m {
			continue
		}
		if len(k.TypeParams) > 0 {
			return false, "override in generic " + k.Name // a non-generic dispatcher has no arm for Array[E]
		}
		if !c.plainMethod(impl.M) || impl.Owner.isStruct() && !c.plainSelf(impl.M) || !impl.Owner.isStruct() && impl.Owner.GoType == "" {
			return false, "override in " + impl.Owner.Name + " not plain"
		}
		if p2, r2 := c.sig(impl.M, map[string]Type{}); p2 != ps || r2 != ret {
			return false, "override in " + impl.Owner.Name + " has another signature" // an adapter answers it (shadowed)
		}
	}
	return true, ""
}

// moduleDispatchableOwner: a module's own method qualifies when its free func can take the constraint as Self
// (decision 208: the constraint names no Self, nor does the signature) and no other module that includes this one
// self-calls the name (its constraint would have to declare it).
func (c *Compiler) moduleDispatchableOwner(owner *Class, m *Method) string {
	switch {
	case !c.plainMethod(m):
		return "not plain method"
	case stdMethodNames[m.GoName] || genericProtocol[m.Name] || c.goCalled()[m.GoName] || c.universalHas(m.Name):
		return "protocol/goCalled"
	case m.Name == "each" || m.Iterator:
		return "iterator" // the constraint's Each, and Go iterators the includer adapts (seqAdapter)
	case sigMentions(m, owner) || c.constraintNamesSelf(owner):
		return "Self in signature or constraint"
	}
	return "" // a module that includes owner and calls m on self reaches the dispatcher too (decision 234)
}

// moduleOverrides checks every includer's own resolution of a module method against the module's signature, with the
// includer's type parameters read back as the module's (alignedArgs), so a dispatcher arm can call it.
func (c *Compiler) moduleOverrides(owner *Class, m *Method) (bool, string) {
	modEnv := map[string]Type{}
	for _, p := range owner.TypeParams {
		modEnv[p] = TVar{Name: p}
	}
	ps, ret := c.sig(m, modEnv)
	for _, k := range c.classList {
		if k.IsModule || k.universal || !k.isSubclassOf(owner) {
			continue
		}
		impl := k.lookup(m.Name)
		if impl == nil {
			return false, "undefined in " + k.Name
		}
		if impl.M == m {
			continue
		}
		if impl.Owner.IsModule || impl.Owner.universal || !c.plainMethod(impl.M) || impl.Owner.isStruct() && !c.plainSelf(impl.M) {
			return false, "override in " + impl.Owner.Name + " not plain"
		}
		if impl.Owner != k {
			return false, "override inherited into " + k.Name // the arm would need the subclass's own resolution per class
		}
		env, ok := c.alignedArgs(k, owner)
		if !ok {
			return false, "include args of " + k.Name + " are not the class's parameters"
		}
		if p2, r2 := c.sig(impl.M, env); p2 != ps || r2 != ret {
			return false, "override in " + k.Name + " has another signature"
		}
	}
	return true, ""
}

// alignedArgs maps an includer's own type parameters to the module's, when `include Mod #[A, B]` names each of the
// class's parameters once; the env renders the class's method in the module's parameter names, and the module's
// parameters in the class's order are the arm's type arguments.
func (c *Compiler) alignedArgs(k, owner *Class) (map[string]Type, bool) {
	var inc *Include
	for cls := k; cls != nil && inc == nil; cls = cls.Super {
		for i := range cls.Includes {
			if cls.Includes[i].Mod == owner {
				inc = &cls.Includes[i]
			}
		}
	}
	env := map[string]Type{}
	if inc == nil {
		return env, len(k.TypeParams) == 0 && len(owner.TypeParams) == 0
	}
	if len(inc.Args) != len(owner.TypeParams) {
		return nil, false
	}
	seen := map[string]bool{}
	for i, a := range inc.Args {
		v, ok := a.(TVar)
		if !ok || seen[v.Name] || !slices.Contains(k.TypeParams, v.Name) {
			return nil, false
		}
		seen[v.Name] = true
		env[v.Name] = TVar{Name: owner.TypeParams[i]}
	}
	if len(seen) != len(k.TypeParams) {
		return nil, false
	}
	return env, true
}

// tvarEnv binds type parameters to themselves.
func tvarEnv(tps []string) map[string]Type {
	env := map[string]Type{}
	for _, p := range tps {
		env[p] = TVar{Name: p}
	}
	return env
}

// armTypeArgs renders an includer's free func type arguments in the module's parameter names.
func (c *Compiler) armTypeArgs(k, owner *Class) string {
	env, _ := c.alignedArgs(k, owner)
	if len(k.TypeParams) == 0 {
		return ""
	}
	out := make([]string, len(k.TypeParams))
	for i, p := range k.TypeParams {
		out[i] = c.goType(env[p])
	}
	return "[" + strings.Join(out, ", ") + "]"
}

// dispatchableAll reports whether every ancestor of cls (cls included) that defines name dispatches it, so no interface
// above cls declares it and cls needs no Go method for it; one ancestor that keeps the Go method keeps it on every
// descendant (its interface still declares the name).
func (c *Compiler) dispatchableAll(cls *Class, name string) bool {
	if c.includerCalls(cls, name) {
		return false // an included module's constraint declares it (its own method is not dispatched)
	}
	found := false
	for k := cls; k != nil; k = k.Super {
		if e := k.lookup(name); e != nil {
			found = true
			if !c.dispatchable(e.Owner, e.M) {
				return false // a module's or an ancestor's kept method: its interface declares the name
			}
		}
	}
	return found
}

// dispatchableOwner checks the owner and the method itself; "" when they qualify, else why not.
func (c *Compiler) dispatchableOwner(owner *Class, m *Method) string {
	switch {
	case owner == nil || m.Owner != owner || owner.hashBase:
		return "owner"
	case owner.IsModule && !owner.universal:
		return c.moduleDispatchableOwner(owner, m)
	case len(owner.TypeParams) > 0:
		return "generic owner"
	case !owner.isStruct() && !owner.universal:
		return "owner is not a struct class"
	case !c.plainMethod(m):
		return "not plain method"
	case !c.plainSelf(m):
		return "not plainSelf"
	case stdMethodNames[m.GoName] || genericProtocol[m.Name] || c.goCalled()[m.GoName] || !owner.universal && c.universalHas(m.Name):
		return "protocol/goCalled"
	}
	return ""
}

// universalDispatch reports whether name resolves on every object to a universal owner's dispatchable method, so no
// module constraint declares it and no class carries a forwarder for it.
func (c *Compiler) universalDispatch(name string) bool {
	e := c.classes["Object"].lookup(name)
	return e != nil && e.Owner.universal && c.dispatchable(e.Owner, e.M)
}

// plainMethod: a public def with one Go signature and no adapters.
func (c *Compiler) plainMethod(m *Method) bool {
	return (m.Kind == kindDef || m.Kind == kindPrimitive) && !m.Private && !m.generic() && m.Name != "initialize" && !m.seqAdapter && len(m.shadowed) == 0 && m.SelfType == nil
}

// sendArms lists the classes below owner whose own resolution of m is not owner's body: the dispatcher's cases.
func (c *Compiler) sendArms(owner *Class, m *Method) []*Class {
	if len(c.classList) != c.sendArmsN { // memoized per call site's (owner, m): brew asked 62 CPU-s of it (decision 240)
		c.sendArmsMemo, c.sendArmsN = map[[2]any][]*Class{}, len(c.classList)
	}
	key := [2]any{owner, m}
	if out, ok := c.sendArmsMemo[key]; ok {
		return out
	}
	var out []*Class
	defer func() { c.sendArmsMemo[key] = out }()
	for _, k := range c.sortedClasses() {
		if k == owner || k.IsModule || k.universal || !k.isSubclassOf(owner) {
			continue
		}
		if impl := k.lookup(m.Name); impl != nil && impl.M != m {
			out = append(out, k)
		}
	}
	return out
}

// sendCall renders a dispatchable call: the free func when nothing overrides m below the receiver's static class,
// else its rbSend_ dispatcher, registered for emitSends.
func (c *Compiler) sendCall(e *entry, recv, args string) string {
	owner := e.Owner
	if !owner.IsModule || owner.universal {
		if len(c.sendArms(owner, e.M)) == 0 {
			return freeFuncName(e.M) + "(" + recv + comma(args) + ")"
		}
		return c.registerSend(e) + "(" + recv + comma(args) + ")"
	}
	targs := c.moduleCallArgs(e)
	if len(c.sendArms(owner, e.M)) == 0 {
		return freeFuncName(e.M) + moduleSelfArgs(owner, targs) + "(" + recv + comma(args) + ")" // the module body with the constraint as Self (decision 208)
	}
	return c.registerSend(e) + bracket(targs) + "(" + recv + comma(args) + ")"
}

// registerSend notes a dispatcher for emitSends and names it.
func (c *Compiler) registerSend(e *entry) string {
	name := sendName(e.Owner, e.M)
	if c.sends == nil {
		c.sends = map[string]sendSpec{}
	}
	c.sends[name] = sendSpec{e.Owner, e.M}
	return name
}

// moduleSelfArgs renders a module free func's type arguments with the constraint as Self: `[Mod_Self[any, E], E]`.
func moduleSelfArgs(mod *Class, targs []string) string {
	return "[" + mod.selfName() + "[any" + comma(strings.Join(targs, ", ")) + "]" + comma(strings.Join(targs, ", ")) + "]"
}

func bracket(targs []string) string {
	if len(targs) == 0 {
		return ""
	}
	return "[" + strings.Join(targs, ", ") + "]"
}

// moduleCallArgs renders a module method's type arguments at a call: the module's parameters as the receiver binds them.
func (c *Compiler) moduleCallArgs(e *entry) []string {
	out := make([]string, len(e.Owner.TypeParams))
	for i, p := range e.Owner.TypeParams {
		t := e.Env[p]
		if t == nil {
			t = TVar{Name: p}
		}
		out[i] = c.goType(t)
	}
	return out
}

// emitSends emits the dispatchers kept code reaches, one round of emitNext.
func (c *Compiler) emitSends(reached func(string) bool) {
	for _, name := range slices.Sorted(maps.Keys(c.sends)) {
		if c.sendOut[name] || !c.dynEvery && !reached(name) {
			continue
		}
		if c.sendOut == nil {
			c.sendOut = map[string]bool{}
		}
		c.sendOut[name] = true
		s := c.sends[name]
		ps, ret := c.sig(s.m, map[string]Type{})
		args := c.argNames(s.m)
		call := func(recv string, m *Method) string { return c.sendArm(recv, m, args, ret, "", map[string]Type{}) }
		armType := c.sendArmType(s.owner)
		arms := c.sendArms(s.owner, s.m)
		c.labels[name] = s.owner.RubyName + "#" + s.m.Name
		selfType, tpDecl, dflt := c.plainSelfType(s.owner), "", freeFuncName(s.m)+"(self"+comma(args)+")"
		if s.owner.IsModule && !s.owner.universal {
			// a generic dispatcher over the module's parameters; an arm names an includer at those parameters
			tps := s.owner.TypeParams
			if len(tps) > 0 {
				tpDecl = "[" + strings.Join(tps, ", ") + " comparable]"
			}
			ps, ret = c.sig(s.m, tvarEnv(tps))
			selfType = s.owner.selfName() + "[any" + comma(strings.Join(tps, ", ")) + "]"
			dflt = freeFuncName(s.m) + moduleSelfArgs(s.owner, tps) + "(self" + comma(args) + ")"
			call = func(recv string, m *Method) string {
				return c.sendArm(recv, m, args, ret, c.armTypeArgs(m.Owner, s.owner), tvarEnv(tps))
			}
		}
		c.w("func %s%s(self %s%s) %s {\n", name, tpDecl, selfType, comma(ps), ret)
		c.writeSendArms(arms, s.m.Name, armType, call)
		if ret == "" {
			c.w("\t%s\n}\n\n", dflt)
		} else {
			c.w("\treturn %s\n}\n\n", dflt)
		}
	}
}

// sendArm is one dispatcher arm's body: the arm method's call (typeArgs instantiate a module's includer), returning its
// value; a void override under a valued slot (`def frozen_string_literal; end`, rubocop's VimComment) answers nil.
func (c *Compiler) sendArm(recv string, m *Method, args, ret, typeArgs string, env map[string]Type) string {
	code := freeFuncName(m) + typeArgs + "(" + recv + comma(args) + ")"
	if typeArgs == "" && c.isDirectMethod(m) {
		code = recv + "." + m.GoName + "(" + args + ")" // a primitive's own Go method (String.FrozenQ)
	}
	if ret == "" {
		return code + "\n\t\treturn"
	}
	if _, mret := c.sig(m, env); mret == "" {
		return code + "\n\t\treturn " + goZero(ret)
	}
	return "return " + code
}

// sendArmType is the Go type a dispatcher arm matches: an includer at the module's parameters, a struct class's concrete
// type (its interface would also match every subclass), or a primitive's value type (String, not *String).
func (c *Compiler) sendArmType(owner *Class) func(k *Class) string {
	if owner.IsModule && !owner.universal {
		return func(k *Class) string { return "*" + k.Name + c.armTypeArgs(k, owner) }
	}
	return func(k *Class) string {
		if k.isStruct() {
			return k.concreteType()
		}
		return c.goType(TClass{C: k})
	}
}

// writeSendArms writes the dispatcher's type switch over arms (an `if` for one arm: gocritic rejects a one-case switch).
// v_: a Ruby local `v_` is v__ (goLocalName), so no parameter shadows it.
func (c *Compiler) writeSendArms(arms []*Class, name string, armType func(*Class) string, call func(string, *Method) string) {
	if len(arms) == 1 {
		k := arms[0]
		c.w("\tif v_, ok := self.(%s); ok {\n\t\t%s\n\t}\n", armType(k), call("v_", k.lookup(name).M))
		return
	}
	c.w("\tswitch v_ := self.(type) {\n")
	for _, k := range arms {
		c.w("\tcase %s:\n\t\t%s\n", armType(k), call("v_", k.lookup(name).M))
	}
	c.w("\t}\n")
}

// goCalled is every capitalized method name the prelude's Go text (`%x{}` bodies, prelude/go helpers) calls by selector:
// Go code reaches those through a method set, so they stay Go methods. Ruby method names are lowercase, so the scan over
// the .rb sources only sees their Go escapes. A selector on a std package name (`os.Exit(`) is not a method call;
// stdImports knows the package names once loadPreludeGo has merged the helpers' imports, which every compile does first.
func (c *Compiler) goCalled() map[string]bool {
	if c.goCalledNames != nil {
		return c.goCalledNames
	}
	c.goCalledNames = map[string]bool{}
	for _, pattern := range []string{"prelude.rb", "prelude/*.rb", "prelude/go/*.go"} {
		names, _ := fs.Glob(c.preludeFS, pattern)
		for _, name := range names {
			src, err := fs.ReadFile(c.preludeFS, name)
			if err != nil {
				panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
			}
			goSelectorCalls(src, func(recv, name []byte) {
				if _, std := stdImports[string(recv)]; !std { // os.Exit(, time.Sleep(: a std package's function, not a method on a Ruby value
					c.goCalledNames[string(name)] = true
				}
			})
		}
	}
	return c.goCalledNames
}
