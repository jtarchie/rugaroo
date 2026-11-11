package compiler

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func (c *Compiler) emitClassType(cls *Class) {
	switch {
	case cls.universal:
		if !cls.IsModule {
			if c.weakRefs() {
				c.w("type %s struct{ _ *byte }\n\n", cls.Name) // a WeakRef's referent must not be tiny-allocated (weakRefs)
			} else {
				c.w("type %s struct{ _ byte }\n\n", cls.Name) // not zero-size: Go may give every new zero-size value one address, and Object.new must be unique
			}
		}
	case cls.IsModule:
		c.emitModuleInterface(cls)
	case cls.GoType != "":
		tp := ""
		if len(cls.TypeParams) > 0 {
			tp = "[" + strings.Join(cls.TypeParams, ", ") + " comparable]"
		}
		goType := cls.GoType
		if cls.hashBase && len(cls.IvarList) > 0 { // a container subclass's own ivars sit beside the embedded container
			var fields strings.Builder
			for _, iv := range cls.IvarList {
				fmt.Fprintf(&fields, "; %s %s", goFieldName(iv.Name), c.goType(iv.Type))
			}
			if cls.ivarBits {
				fmt.Fprintf(&fields, "; %s uint64", ivarSetField)
			}
			goType = strings.TrimSuffix(goType, " }") + fields.String() + " }"
		}
		c.w("type %s%s %s\n\n", cls.Name, tp, goType)
		inst, decl := cls.Name, ""
		if tps := strings.Join(cls.TypeParams, ", "); tps != "" {
			inst, decl = cls.Name+"["+tps+"]", "["+tps+" comparable]"
		}
		if cls.hashBase && len(cls.IvarList) > 0 {
			c.w("func (self *%s) _%s() *%s { return self }\n\n", inst, cls.Name, inst)
		}
		if cls.hashBase {
			switch cls.Super.RubyName {
			case "Hash":
				c.w("func %s%s() *%s { return &%s{%s: *NewHash[any, %s]()} }\n\n", cls.ctorName(), decl, inst, inst, superField(cls.Super), strings.Join(cls.TypeParams, ", "))
			case "Array", "Set", "Queue":
				c.w("func %s%s() *%s { return &%s{%s: *%s%s()} }\n\n", cls.ctorName(), decl, inst, inst, superField(cls.Super), cls.Super.ctorName(), anyArgs(len(cls.Super.TypeParams)))
			default:
				if sup := cls.Super; sup.hashBase && sup.mutable() && len(sup.TypeParams) == 0 && decl == "" { // Concurrent::Set < CRubySet < ::Set
					c.w("func %s() *%s { return &%s{%s: *%s()} }\n\n", cls.ctorName(), inst, inst, superField(sup), sup.ctorName())
					break
				}
				// a value base starts zero; super in initialize sets it
				c.w("func %s() *%s { return &%s{} }\n\n", cls.ctorName(), inst, inst)
			}
		}
	case cls.isTemplate(): // only its instances are Go structs (decision 308)
		c.w("type %s interface{ _%s() }\n\n", cls.anyName(), cls.anyName())
	default:
		c.emitStructClass(cls)
	}
}

// publicEntries lists the entries that make up a class's Go interface:
// public, non-generic methods.
func (c *Compiler) publicEntries(cls *Class) []entry {
	var out []entry
	for _, e := range cls.methodSet() {
		if staticPrivate(e.M) || e.M.generic() || e.M.Name == "initialize" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// userTypeLine maps a user class's struct or module's constraint to its `class`/`module` line, so a Go error in it points at Ruby and `rugaroo web` can tell the type, and what it names, from the prelude's.
func (c *Compiler) userTypeLine(cls *Class) {
	if cls.File != nil && !cls.File.isPrelude() && cls.metaOf == nil {
		c.lineDirective(cls.File, cls.Line)
	}
}

func (c *Compiler) emitModuleInterface(mod *Class) {
	tps := ""
	if len(mod.TypeParams) > 0 {
		tps = ", " + strings.Join(mod.TypeParams, ", ") + " comparable"
	}
	c.userTypeLine(mod)
	if len(mod.IvarList) > 0 { // its state, a field of each includer (decision 147)
		c.w("type %s struct {\n", ivarsType(mod))
		for _, iv := range mod.IvarList {
			c.w("\t%s %s\n", goFieldName(iv.Name), c.goType(iv.Type))
		}
		if mod.ivarBits {
			c.w("\t%s uint64\n", ivarSetField)
		}
		c.w("}\n\n")
	}
	c.w("type %s[Self any%s] interface {\n", mod.selfName(), tps)
	if len(mod.IvarList) > 0 {
		c.w("\t_%s() *%s\n", mod.Name, ivarsType(mod))
	}
	for _, im := range ivarModules(mod) { // its included modules' state too (Obligation writes Dereferenceable's @value)
		if im != mod {
			c.w("\t_%s() *%s\n", im.Name, ivarsType(im))
		}
	}
	// Constraint = what the module's bodies call on self (docs/design.md).
	called := c.selfCalls(mod)
	for _, e := range c.publicEntries(mod) {
		if !called[e.M.Name] || !c.renderingConstraint && c.dispatchable(e.Owner, e.M) {
			continue // a dispatched name is called through rbSend_, not the constraint (dispatch.go)
		}
		env := composeEnv(e.Env, nil)
		env["Self"] = TVar{Name: "Self"}
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
	}
	c.emitBridgeSigs(mod, TVar{Name: "Self"})
	// self is some includer, whose to_s/inspect/... may override Object's
	if mod.lookup("method_missing") == nil {
		for _, e := range c.publicEntries(c.classes["Object"]) {
			if called[e.M.Name] && mod.lookup(e.M.Name) == nil && c.topDefs[e.M.Name] == nil && !c.dispatchable(e.Owner, e.M) {
				ps, ret := c.sig(e.M, map[string]Type{"Self": TVar{Name: "Self"}})
				c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
			}
		}
	}
	if called["class"] {
		c.w("\t_ClassObj() %s\n", c.goType(TClass{C: c.classes["Class"]}))
	}
	c.w("}\n\n")
}

// bridgeName is the includer method a module method's `super` calls.
func bridgeName(m *Method) string { return "_Super_" + m.Owner.Name + "_" + m.GoName }

// superBridges lists, as seen from cls, the module methods in its
// ancestors (itself, for a module) whose `super` target differs per
// includer. The module's constraint requires a bridge for each, and every
// concrete includer implements it by calling whatever follows the module
// method in its own ancestors.
func (c *Compiler) superBridges(cls *Class) []entry {
	var out []entry
	seen := map[*Method]bool{}
	for _, anc := range append([]*Class{cls}, cls.allAncestors()...) {
		if !anc.IsModule {
			continue
		}
		for _, m := range anc.MethodList {
			if !m.superBridge || seen[m] {
				continue
			}
			seen[m] = true
			for _, e := range cls.defsOf(m.Name) {
				if e.M == m {
					out = append(out, e)
					break
				}
			}
		}
	}
	return out
}

// emitBridgeSigs lists cls's super bridges in its interface.
func (c *Compiler) emitBridgeSigs(cls *Class, self Type) {
	for _, e := range c.superBridges(cls) {
		env := composeEnv(e.Env, nil)
		env["Self"] = self
		if self == nil {
			env["Self"] = c.selfTypeFor(e, cls)
		}
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", bridgeName(e.M), ps, ret)
	}
}

// emitBridges implements cls's super bridges: each calls the definition
// after the module method in cls's ancestors, or fails as MRI does.
func (c *Compiler) emitBridges(cls *Class, recv string) {
	for _, e := range c.superBridges(cls) {
		m := e.M
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		ps, ret := c.sig(m, env)
		defs := cls.defsOf(m.Name)
		i := slices.IndexFunc(defs, func(d entry) bool { return d.M == m })
		var body string
		switch {
		case i+1 < len(defs):
			t := defs[i+1]
			if c.isDirectMethod(t.M) {
				c.errorf(m.File, m.Node, "super into a primitive class method is not supported")
			}
			body = fmt.Sprintf("%s%s(self%s)", freeFuncName(t.M), c.forwardTypeArgs(t, cls), comma(c.argNames(m)))
			if b, ok := c.restBridge(cls, m, t, ret); ok {
				body = b
			} else if b, ok := c.reshapedBridge(cls, m, t, ret); ok {
				body = b
			} else if _, tret := c.sig(t.M, c.forwarderEnv(cls, t)); ret != "" && tret == "" { // a void parent under a valued override's bridge: nil
				body += "; return " + goZero(ret)
			} else if ret != "" && tret != ret && m.Ret != nil {
				// the parent answers another Go type than the override (`[:b, super]` over a Symbol `foo`, ruby/spec's
				// AliasWithSuper): converted, raising where they differ, not a go build failure (decision 416)
				body = fmt.Sprintf("return rbAs[%s](any(%s), %q)", ret, body, m.Ret.String())
			} else if ret != "" {
				body = "return " + body
			}
		case m.Name == "initialize": // BasicObject's does nothing
		case hookNames[m.Name], m.Name == "initialize_dup", m.Name == "initialize_clone":
			// Class's, Module's and Object's hooks do nothing (decision 232): a mixed-in `inherited` (decision 241) or
			// `initialize_dup` (Homebrew's Downloadable, decision 290) calling super
			if ret != "" {
				body = "return " + goZero(ret)
			}
		case m.Name == "const_missing" && len(m.Params) == 1: // Module#const_missing: MRI's NameError (bindata's factories chain to it)
			body = fmt.Sprintf("panic(NewNameError(Ref(String(\"uninitialized constant \" + string(rbToS(%s))))))", strings.Split(c.argNames(m), ", ")[0])
		default:
			body = fmt.Sprintf("panic(NewNoMethodError(Ref(String(\"super: no superclass method '%s' for \" + rbDescribe(self)))))", m.Name)
		}
		c.w("func (self %s) %s(%s) %s { %s }\n", recv, bridgeName(m), ps, ret, body)
	}
}

// restBridge is a gem override `def m(*)` whose zsuper reaches a parent taking fixed positionals (rubocop's
// Ext::RegexpNode#assign_properties(*) over Parser::AST::Node#assign_properties(properties)): the rest spreads into
// them, its count checked as MRI's ArgumentError.
func (c *Compiler) restBridge(cls *Class, m *Method, t entry, ret string) (string, bool) {
	if !c.gemFile(m.File) || len(m.Params) != 1 || !m.Params[0].Rest || m.Block != nil || t.M.Block != nil || t.M.calleeDefaults || t.M.kwMask() {
		return "", false
	}
	tenv := c.forwarderEnv(cls, t)
	var args []string
	for i, p := range t.M.Params {
		if p.Rest || p.KwRest || p.Keyword || p.Post || p.Default != nil {
			return "", false
		}
		gt := c.goType(subst(p.Type, tenv))
		a := fmt.Sprintf("rest_[%d]", i)
		if gt != "any" {
			a = fmt.Sprintf("rbAs[%s](%s, %q)", gt, a, subst(p.Type, tenv).String())
		}
		args = append(args, a)
	}
	n := len(t.M.Params)
	call := fmt.Sprintf("rbArity(len(rest_), %d, %d); %s%s(self%s)", n, n, freeFuncName(t.M), c.forwardTypeArgs(t, cls), comma(strings.Join(args, ", ")))
	_, tret := c.sig(t.M, tenv)
	switch ret {
	case "":
		return call, true
	case tret:
		return strings.Replace(call, "; ", "; return ", 1), true
	}
	if tret == "" {
		return call + "; return " + goZero(ret), true
	}
	return strings.Replace(call, "; ", fmt.Sprintf("; return rbAs[%s](any(", ret), 1) + fmt.Sprintf("), %q)", m.Ret), true
}

// reshapedBridge is a gem override's super into a parent of another shape (Homebrew's
// ObserverPathnameExtension#mkpath, no parameters and no value, over Pathname#mkpath(mode: nil) -> self): the parent's
// missing optional parameters get their zero values, and a parent value the override's type cannot hold is dropped.
// ponytail: super's value is the bridge type's zero then; type the bridge by the parent's return when a gem reads it.
func (c *Compiler) reshapedBridge(cls *Class, m *Method, t entry, ret string) (string, bool) {
	if !c.gemFile(m.File) || len(m.Params) >= len(t.M.Params) || t.M.calleeDefaults || t.M.kwMask() || m.Block != nil || t.M.Block != nil {
		return "", false
	}
	args := strings.Split(c.argNames(m), ", ")
	if args[0] == "" {
		args = nil
	}
	tenv := c.forwarderEnv(cls, t)
	for _, p := range t.M.Params[len(m.Params):] {
		if p.Rest || p.KwRest || p.Default == nil && !p.Keyword {
			return "", false
		}
		args = append(args, "*new("+c.goType(subst(p.Type, tenv))+")")
	}
	call := fmt.Sprintf("%s%s(self%s)", freeFuncName(t.M), c.forwardTypeArgs(t, cls), comma(strings.Join(args, ", ")))
	_, tret := c.sig(t.M, tenv)
	switch ret {
	case "":
		return call, true
	case tret:
		return "return " + call, true
	}
	return call + "; return " + goZero(ret), true
}

// hookNames are Class's and Module's callbacks, whose own definitions do nothing (decision 232).
var hookNames = map[string]bool{"inherited": true, "included": true, "extended": true, "prepended": true, "method_added": true}

var goSelfCall = regexp.MustCompile(`\bself\.([A-Za-z_]\w*)\(`)

// selfCalls lists the method names a module's bodies invoke on self,
// including `self.X(` inside %x{} leaves.
func (c *Compiler) selfCalls(mod *Class) map[string]bool {
	if mod.selfCallCache != nil {
		return mod.selfCallCache
	}
	out := map[string]bool{}
	byGo := map[string]string{}
	for _, e := range mod.methodSet() {
		byGo[e.M.GoName] = e.M.Name
	}
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		if n == nil {
			return
		}
		switch n := n.(type) {
		case *parser.CallNode:
			if n.Receiver == nil {
				out[n.Name] = true
			} else if _, ok := n.Receiver.(*parser.SelfNode); ok {
				out[n.Name] = true
			} else if n.Name == "class" {
				out[n.Name] = true // the receiver may be a copy of self
			}
		case *parser.XStringNode:
			for _, m := range goSelfCall.FindAllStringSubmatch(n.Unescaped.Value, -1) {
				if rb, ok := byGo[m[1]]; ok {
					out[rb] = true
				}
			}
		case *parser.InterpolatedStringNode:
			// interpolation calls to_s on every part
			out["to_s"] = true
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	core := mod.File != nil && mod.File.isPrelude()
	var gem []*Method
	for _, m := range mod.MethodList {
		if m.Node == nil {
			continue
		}
		if core && c.gemFile(m.File) {
			gem = append(gem, m)
			continue
		}
		walk(m.Node.Body)
		if m.Node.Parameters != nil {
			walk(m.Node.Parameters) // defaults run in the callee
		}
	}
	for _, inc := range mod.Includes {
		for k := range c.selfCalls(inc.Mod) {
			out[k] = true
		}
	}
	// A gem's method added to a core module (`module Enumerable; def exclude?(x) = !include?(x)`) calls self
	// dynamically: the core includers (Range, Hash) are Go types whose include? Enumerable's slot can't adapt to.
	own := out
	out = map[string]bool{}
	for _, m := range gem {
		walk(m.Node.Body)
	}
	mod.gemSelfCalls = map[string]bool{}
	for k := range out {
		if !own[k] {
			mod.gemSelfCalls[k] = true
		}
	}
	mod.selfCallCache = own
	return own
}

func (c *Compiler) emitStructClass(cls *Class) {
	if len(cls.Subclasses) > 0 && !cls.universal {
		c.w("type %s = %s\n\n", superField(cls), cls.Name)
	}
	c.userTypeLine(cls)
	if cls.template != nil {
		c.w("type %s struct { // %s\n", cls.Name, TClass{C: cls})
	} else {
		c.w("type %s struct {\n", cls.Name)
	}
	switch {
	case cls.byValue: // its fields only: an embedded Data's byte or a frozen flag is a fifth field Go would not keep in registers (decision 441)
	case cls.Super != nil && !cls.Super.universal:
		c.w("\t%s\n", superField(cls.Super))
	case c.objFreeze() && len(cls.TypeParams) == 0:
		c.w("\t%s bool\n", frozenField) // Kernel#freeze's flag, on the root: every subclass embeds it (decision 221)
	case len(cls.IvarList) == 0:
		// zero-size allocations may share an address, merging distinct instances' identity
		c.w("\t_ byte\n")
	}
	for _, iv := range cls.IvarList {
		c.w("\t%s %s\n", goFieldName(iv.Name), c.goType(iv.Type))
	}
	if (cls.Super == nil || cls.Super.universal) && c.weakRefs() && !cls.byValue {
		c.w("\t_ *byte\n") // a WeakRef's referent must not be tiny-allocated (weakRefs); subclasses embed this root
	}
	if cls.isPerRun {
		c.w("\trbCls_ *%s\n", cls.meta.Name) // the class object whose new made it (decision 259)
	}
	if cls.ivarBits {
		c.w("\t%s uint64\n", ivarSetField)
	}
	for _, mod := range ownIvarModules(cls) {
		c.w("\t%s %s\n", ivarsField(mod), ivarsType(mod))
	}
	c.w("}\n\n")
	// interface
	c.w("type %s interface {\n", cls.ifaceName())
	for _, k := range cls.structChain() {
		c.w("\t_%s() *%s\n", k.Name, k.Name)
	}
	for _, mod := range ivarModules(cls) {
		c.w("\t_%s() *%s\n", mod.Name, ivarsType(mod))
	}
	c.emitEntrySigs(cls)
	c.emitBridgeSigs(cls, nil)
	if c.includerCalls(cls, "class") {
		// forwardTypeArgs binds Self to this interface, so it needs the module's `self.class` hook
		c.w("\t_ClassObj() %s\n", c.goType(TClass{C: c.classes["Class"]}))
	}
	if cls.meta != nil || cls.metaOf != nil {
		// a metaclass inherits _ClassOf from Class/Module; listing it lets a
		// singleton(C) value pass where Class or Module is expected
		c.w("\t_ClassOf() %s\n", c.goType(TClass{C: cls.root().meta}))
	}
	isModule := cls.isSubclassOf(c.classes["Module"])
	if isModule {
		c.w("\t_Kind() string\n")
	}
	c.w("}\n\n")
	if cls.byValue { // a copy's address: reads only, initialize writes the fields themselves (ivarPath)
		c.w("func (self %s) _%s() *%s { return &self }\n\n", cls.Name, cls.Name, cls.Name)
		for _, k := range cls.structChain()[1:] { // Data's marker, which the dropped embed carried
			c.w("func (%s) _%s() *%s { return new(%s) }\n\n", cls.Name, k.Name, k.Name, k.Name)
		}
	} else {
		c.w("func (self *%s) _%s() *%s { return self }\n\n", cls.Name, cls.Name, cls.Name)
	}
	if t := cls.template; t != nil { // the family marker an untyped is_a?/case/rescue asserts (decision 308)
		c.w("func (*%s) _%s() {}\n\n", cls.Name, t.anyName())
	}
	for _, mod := range ownIvarModules(cls) {
		c.w("func (self *%s) _%s() *%s { return &self.%s }\n\n", cls.Name, mod.Name, ivarsType(mod), ivarsField(mod))
	}
	switch {
	case cls.isPerRun:
		c.w("func (self *%s) _ClassOf() %s {\n\tif self.rbCls_ != nil {\n\t\treturn self.rbCls_\n\t}\n\treturn %s\n}\n\n", cls.Name, c.goType(TClass{C: cls.root().meta}), classVar(cls))
	case cls.meta != nil:
		c.w("func (self %s) _ClassOf() %s { return %s }\n\n", c.recvType(cls), c.goType(TClass{C: cls.root().meta}), classVar(cls))
	}
	if isModule {
		c.emitConstTable(cls)
	}
	if cls.metaOf != nil {
		// a metaclass has exactly one instance: the class object
		c.w("var %s = &%s{}\n\n", classVar(cls.metaOf), cls.Name)
		if len(ownIvarModules(cls)) > 0 || len(cls.IvarList) > 0 { // class-level ivars by name: an `inherited`'s `child.instance_eval { @env = ... }` on an untyped child (decisions 241, 259)
			c.emitIvarList(cls)
		}
		return
	}
	c.emitIvarList(cls)
	c.emitCopy(cls)
	c.emitDup(cls)
	c.emitCtor(cls)
}

// emitCtor writes cls's NewX, and NewXIn for a per-run Class.new class (decision 259).
func (c *Compiler) emitCtor(cls *Class) {
	if cls.File != nil && !cls.File.isPrelude() && cls.Line > 0 {
		c.lineDirective(cls.File, cls.Line) // a Go error here names the class, not a line counted on from the last directive
	}
	init := cls.lookup("initialize")
	if init != nil {
		env := composeEnv(init.Env, nil)
		env["Self"] = TClass{C: cls}
		ps, _ := c.sig(init.M, env)
		c.labels[cls.ctorName()] = "Class#new"
		if cls.byValue {
			c.w("func %s(%s) %s {\n\tvar self %s\n\t%s(&self, %s)\n\treturn self\n}\n\n",
				cls.ctorName(), ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
			return
		}
		c.w("func %s(%s) *%s {\n\tself := &%s{}\n\t%s(self, %s)\n\treturn self\n}\n\n",
			cls.ctorName(), ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
		if cls.isPerRun { // its class object's new: initialize already sees self.class (decision 259)
			c.w("func New%sIn(k *%s, %s) *%s {\n\tself := &%s{rbCls_: k}\n\t%s(self, %s)\n\treturn self\n}\n\n",
				cls.Name, cls.meta.Name, ps, cls.Name, cls.Name, freeFuncName(init.M), c.argNames(init.M))
		}
	} else {
		if cls.byValue {
			c.w("func %s() %s { return %s{} }\n\n", cls.ctorName(), cls.Name, cls.Name)
			return
		}
		c.w("func %s() *%s { return &%s{} }\n\n", cls.ctorName(), cls.Name, cls.Name)
		if cls.isPerRun {
			c.w("func New%sIn(k *%s) *%s { return &%s{rbCls_: k} }\n\n", cls.Name, cls.meta.Name, cls.Name, cls.Name)
		}
	}
}

// emitEntrySigs writes cls's interface lines for its public methods, but those dispatched by type switch.
func (c *Compiler) emitEntrySigs(cls *Class) {
	for _, e := range c.publicEntries(cls) {
		if cls.metaOf != nil && e.M.Name == "new" {
			// Subclasses may take different initialize arguments, so `new`
			// is not part of the shared class-object interface; calls
			// through `singleton(C)` assert for it instead.
			continue
		}
		if c.dispatchableAll(cls, e.M.Name) {
			continue // dispatched by type switch, not through this interface (dispatch.go)
		}
		env := composeEnv(e.Env, nil)
		env["Self"] = c.selfTypeFor(e, cls)
		ps, ret := c.sig(e.M, env)
		c.w("\t%s(%s) %s\n", e.M.GoName, ps, ret)
		if e.M.seqAdapter || e.M.seqRest {
			name, ps, ret := c.seqAdapterSig(e.M, env)
			c.w("\t%s(%s) %s\n", name, ps, ret)
		}
		for _, s := range e.M.shadowed {
			ps, ret := c.sig(s.e.M, c.slotEnv(s))
			c.w("\t%s(%s) %s\n", s.e.M.GoName, ps, ret)
		}
		if e.M.GoName == "cmpNil" && len(e.M.Params) == 1 { // emitCmpAdapter's Comparable slot
			c.w("\tOp_cmp(%s) Integer\n", c.goType(c.cmpIncluder(cls)))
		}
	}
}

// superField names the field a subclass embeds cls through: an alias, since
// an embedded field takes its type's name and a method of that Go name
// (`Calc#calc`) would clash with it. Lowercase with a trailing `_`, it is
// neither a method nor an ivar name (goLocalName).
func superField(cls *Class) string { return "super_" + cls.Name + "_" }

// weakRefs reports whether the program loads WeakRef. Go's allocator batches pointer-free objects of 16 bytes or
// less into one slot, and a weak pointer into such a slot "may never become nil" while a neighbour lives (package
// weak), so a WeakRef to a small object stayed alive by luck of placement: example 80's Session(8), a lone Integer
// ivar, did once a Hash change moved its neighbour (decision 351). A pointer field makes every struct class a
// scanned object, which is never batched; it costs 8 bytes an object, only in programs that require weakref.
func (c *Compiler) weakRefs() bool { return c.loaded["prelude/weakref.rb"] }
