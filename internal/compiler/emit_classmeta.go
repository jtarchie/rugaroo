package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// emitDescendants gives a class object its descendant classes in definition
// order, which is the order MRI's inherited hooks would see them (rugaroo's
// classes are all declared statically). Only rbDescendants asks for it.
func (c *Compiler) emitDescendants(meta, desc *Class) {
	var subs []string
	for _, k := range c.classList {
		if k != desc && !k.IsModule && k.metaOf == nil && k.meta != nil && k.isSubclassOf(desc) {
			subs = append(subs, classVar(k))
		}
	}
	c.w("func (self *%s) _Descendants() []any { return []any{%s} }\n\n", meta.Name, strings.Join(subs, ", "))
	var anc []string
	for _, k := range fullAncestors(desc) {
		for i := len(k.prepended) - 1; i >= 0; i-- { // `prepend M` puts M before the class (decision 199)
			if m := k.prepended[i]; m.meta != nil {
				anc = append(anc, classVar(m))
			}
		}
		if k.meta != nil && (!k.hidden || k == desc) {
			anc = append(anc, classVar(k))
		}
	}
	var latent []string // run-time includes boot saw: listed once this object's include call ran (decision 331)
	for k := desc; k != nil && !k.IsModule; k = k.Super {
		for _, inc := range k.Includes {
			if inc.latent && inc.Mod != nil && inc.Mod.meta != nil {
				latent = append(latent, classVar(inc.Mod))
			}
		}
	}
	if len(latent) > 0 {
		c.w("func (self *%s) _Latent() []any { return []any{%s} }\n\n", meta.Name, strings.Join(latent, ", "))
		c.w("func (self *%s) _Ancestors() []any { return rbActiveAncestors(self, []any{%s}) }\n\n", meta.Name, strings.Join(anc, ", "))
	} else {
		c.w("func (self *%s) _Ancestors() []any { return []any{%s} }\n\n", meta.Name, strings.Join(anc, ", "))
	}
	if desc.IsModule {
		return
	}
	// Class#subclasses: the direct ones, newest first as MRI lists them
	var direct []string
	for _, k := range slices.Backward(c.classList) {
		if k.Super == desc && k.metaOf == nil && k.meta != nil {
			direct = append(direct, classVar(k))
		}
	}
	c.w("func (self *%s) _Subclasses() []any { return []any{%s} }\n\n", meta.Name, strings.Join(direct, ", "))
	sup := "nil"
	if desc.Super != nil && desc.Super.meta != nil {
		sup = classVar(desc.Super)
	}
	c.w("func (self *%s) _Superclass() any { return %s }\n\n", meta.Name, sup)
}

// fullAncestors is Module#ancestors: k, its modules (last included first,
// each with its own), then its superclass's, through Object, Kernel and
// BasicObject. A module already above keeps that place only, as MRI skips
// re-including it.
func fullAncestors(k *Class) []*Class {
	var all []*Class
	var walk func(k *Class)
	walk = func(k *Class) {
		all = append(all, k)
		for i := len(k.Includes) - 1; i >= 0; i-- {
			if m := k.Includes[i].Mod; m != nil {
				walk(m)
			}
		}
		if k.Super != nil && !k.IsModule {
			walk(k.Super)
		}
	}
	walk(k)
	out := make([]*Class, 0, len(all))
	for i, x := range all {
		if !slices.Contains(all[i+1:], x) {
			out = append(out, x)
		}
	}
	return out
}

// emitMethodTable gives a class object the names of its public instance
// methods, its ancestors' included (decision 77). Object, Kernel and
// BasicObject are left out; MRI lists theirs too. Only
// Module#public_instance_methods selects _Methods, so the pruner drops the
// tables from programs that don't reflect.
func (c *Compiler) emitMethodTable(cls, desc *Class) {
	if desc == nil {
		c.w("func (self *%s) _Methods() []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Methods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	seen := map[string]bool{}
	for _, k := range desc.ancestors() {
		for _, m := range k.reflected() {
			if m.Private || m.Protected || m.Name == "initialize" || strings.HasPrefix(m.Name, "__") || seen[m.Name] {
				continue
			}
			if desc.specChild && k != desc && strings.HasPrefix(m.Name, "test_") {
				continue // a nested describe runs only its own tests: MRI undefines the rest (nuke_test_methods!)
			}
			seen[m.Name] = true
			c.w("\t\t{%q, nil, %t},\n", m.Name, k != desc)
		}
	}
	c.w("\t}\n}\n\n")
}

// emitSingletonMethodTable lists a class object's own public methods (`def self.x`), and Kernel's module functions
// (each Kernel method is callable as `Kernel.x`), for Module#singleton_methods and public_methods(false) (decision 181).
func (c *Compiler) emitSingletonMethodTable(cls, desc *Class) {
	c.w("func (self *%s) _SingletonMethods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	seen := map[string]bool{}
	add := func(ms []*Method, priv bool) {
		for _, m := range ms {
			if (m.Private && !priv) || m.Node == nil || m.Name == "initialize" || strings.HasPrefix(m.Name, "__") || seen[m.Name] { // m.Node nil: generated (`new`, `name`)
				continue
			}
			seen[m.Name] = true
			c.w("\t\t{%q, nil, false},\n", m.Name)
		}
	}
	add(cls.MethodList, false)
	if desc != nil && desc.RubyName == "Kernel" {
		add(desc.MethodList, true)
	}
	c.w("\t}\n}\n\n")
	// its private ones (`private_class_method :new`), for a class object's private_methods (matrix's specs, decision 416)
	c.w("func (self *%s) _PrivateSingletonMethods() []rbConst {\n\treturn []rbConst{\n", cls.Name)
	for _, m := range cls.MethodList {
		if m.Private && m.Name != "initialize" && !strings.HasPrefix(m.Name, "__") {
			c.w("\t\t{%q, nil, false},\n", m.Name)
		}
	}
	c.w("\t}\n}\n\n")
}

// emitPrivateMethodTable is emitMethodTable's private half, for Module#private_instance_methods only (decision 181),
// and its protected one for protected_instance_methods (liquid's Strainer.add_filter asks both, decision 331).
func (c *Compiler) emitPrivateMethodTable(cls, desc *Class) {
	c.emitVisibilityTable(cls, desc, "_PrivateMethods", func(m *Method) bool { return m.Private })
	c.emitVisibilityTable(cls, desc, "_ProtectedMethods", func(m *Method) bool { return m.Protected && !m.Private })
}

func (c *Compiler) emitVisibilityTable(cls, desc *Class, fn string, has func(*Method) bool) {
	if desc == nil {
		c.w("func (self *%s) %s() []rbConst { return nil }\n\n", cls.Name, fn)
		return
	}
	c.w("func (self *%s) %s() []rbConst {\n\treturn []rbConst{\n", cls.Name, fn)
	seen := map[string]bool{}
	for _, k := range desc.ancestors() {
		for _, m := range k.reflected() {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true // a redefinition hides an ancestor's of another visibility
			if has(m) && m.Name != "initialize" && !strings.HasPrefix(m.Name, "__") {
				c.w("\t\t{%q, nil, %t},\n", m.Name, k != desc)
			}
		}
	}
	c.w("\t}\n}\n\n")
}

// isInstanceTest is Module#=== for desc's class object, over an untyped v:
// the run-time half of is_a? (decision 76), a lookup in the generated
// ancestry table by class ID (decision 82).
func (c *Compiler) isInstanceTest(desc *Class) string {
	switch {
	case desc == nil:
		return "false"
	case desc.universal:
		return "true"
	}
	return fmt.Sprintf("rbKindOf(v, %d)", c.classID(desc))
}

// emitClassMeta gives every Go type that holds a Ruby value its class ID,
// and the program the tables they index: names, and ancestors (the class,
// its superclasses, and every module they include). Class names, is_a? on a
// class value and Module#=== read these instead of reflect (decision 82).
// Each piece is kept only where selected (decision 49).
func (c *Compiler) emitClassMeta() {
	id := func(name string) int { return c.classID(c.classes[name]) }
	c.w("const (\n\trbNilClassID = %d\n\trbProcClassID = %d\n)\n\n", id("NilClass"), id("Proc"))
	names := make([]string, 0, len(c.classList))
	ancestry := make([]string, 0, len(c.classList))
	refs := make([]string, 0, len(c.classList))
	kernelInspect := make([]string, 0, len(c.classList))
	for _, cls := range c.classList {
		e := cls.lookup("inspect") // Kernel's: an object's inspect carries a container's rbSeen into its ivars
		kernelInspect = append(kernelInspect, strconv.FormatBool((cls.isStruct() || cls.RubyName == "Object") && !cls.IsModule && e != nil && e.Owner == c.classes["Kernel"]))
		names = append(names, strconv.Quote(cls.displayName()))
		var ids []string
		for _, k := range c.classList {
			if !k.universal && cls.isSubclassOf(k) {
				ids = append(ids, strconv.Itoa(c.classID(k)))
			}
		}
		ancestry = append(ancestry, "{"+strings.Join(ids, ", ")+"}")
		refs = append(refs, strconv.FormatBool(c.emitClassID(cls)))
	}
	c.w("var rbClassNames = [...]string{%s}\n\n", strings.Join(names, ", "))
	c.w("var rbAncestry = [...][]int{%s}\n\n", strings.Join(ancestry, ", "))
	// a table, not a marker method: a `_Ref()` on every pointer class was 11k of the test programs' functions (decision 89)
	c.w("var rbClassRefs = [...]bool{%s}\n\n", strings.Join(refs, ", "))
	c.w("var rbKernelInspect = [...]bool{%s}\n\n", strings.Join(kernelInspect, ", "))
	c.w("func rbRubyClassID(id int) int {\n\tswitch id {\n") // a generic instance is its template to Ruby (decision 308)
	for _, cls := range c.classList {
		if len(cls.instances) > 0 {
			ids := make([]string, len(cls.instances))
			for i, inst := range cls.instances {
				ids[i] = strconv.Itoa(c.classID(inst))
			}
			c.w("\tcase %s:\n\t\treturn %d\n", strings.Join(ids, ", "), c.classID(cls))
		}
	}
	c.w("\tdefault: // gocritic rejects the one-case switch\n\t\treturn id\n\t}\n}\n\n")
	array := c.classID(c.classes["Array"])
	for n := 2; n <= maxTuple; n++ { // every arity, as emitTuples
		tps := make([]string, n)
		for i := range n {
			tps[i] = fmt.Sprintf("T%d", i)
		}
		c.w("func (Tuple%d[%s]) _ClassID() int { return %d }\n\n", n, strings.Join(tps, ", "), array)
	}
}

// emitProcTables emits rbIsProc and rbDynProcCall last, once kept code has stopped growing: their cases are the Proc
// types kept code makes (decision 431). Func types carry no methods, so a type switch lists them.
func (c *Compiler) emitProcTables() {
	c.w("func rbIsProc(a any) bool {\n")
	if procs := c.keptProcs(); len(procs) > 0 {
		c.w("\tswitch a.(type) {\n")
		for _, p := range procs { // one case each, so the pruner can drop the unused (decision 49)
			c.w("\tcase %s:\n\t\treturn true\n", p)
		}
		c.w("\tdefault: // gocritic rejects the one-case switch pruning can leave\n\t\treturn false\n\t}\n}\n\n")
		c.emitDynProcCall(procs)
		return
	}
	c.w("\treturn false\n}\n\n")
	c.emitDynProcCall(nil)
}

// keptProcs is rbIsProc's cases: rendered Proc types whose shape kept code writes, since no other *func value can exist (decision 431).
func (c *Compiler) keptProcs() []string {
	var out []string
	for _, p := range slices.Sorted(maps.Keys(c.procTypes)) {
		s, ok := c.procShapes[p]
		if !ok {
			s = p
			e, err := goparser.ParseExpr(p)
			if err == nil {
				if st, ok := e.(*ast.StarExpr); ok {
					if ft, ok := st.X.(*ast.FuncType); ok {
						s = funcShape(ft)
					}
				}
			}
			c.procShapes[p] = s
		}
		if c.keptView == nil || c.keptView.keptFunc(s) {
			out = append(out, p)
		}
	}
	return out
}

// emitDynProcCall emits rbDynProcCall: `call` on a Proc held untyped, switching over the program's Proc types (amends decision 47).
func (c *Compiler) emitDynProcCall(procs []string) {
	c.w("func rbDynProcCall(recv any, args []any) (any, bool) {\n")
	c.w("\tswitch p := recv.(type) {\n")
	for _, p := range procs {
		t := c.procFuncs[p]
		if t.Self == nil && t.Rest && len(t.Params) == 1 && isAny(t.Params[0]) && isAny(t.Ret) { // a gem's `(*untyped)` block (decision 250)
			c.w("\tcase %s:\n\t\treturn (*p)(args...), true\n", p)
			continue
		}
		if t.Self != nil || slices.ContainsFunc(t.Params, isVoid) || t.Rest && len(t.Params) == 0 {
			continue
		}
		fixed := len(t.Params)
		if t.Rest {
			fixed--
		}
		args := make([]string, fixed)
		for i, pt := range t.Params[:fixed] {
			args[i] = c.dynArg(pt, i)
		}
		least, most := fixed, fixed
		if n, ok := c.procMinArity[p]; ok { // a gem lambda's optional params, nil when not given (decision 416)
			least = n
		}
		if t.Rest { // `-> s, *args { }` (net-ftp's fixture handlers, decision 416): the rest after the fixed ones
			et := t.Params[fixed]
			args = append(args, fmt.Sprintf("rbRest[%s](args, %d, %q)...", c.goType(et), fixed, et.String()))
			most = -1
		}
		call := "(*p)(" + strings.Join(args, ", ") + ")"
		c.w("\tcase %s:\n\t\trbArity(len(args), %d, %d)\n", p, least, most)
		switch {
		case isVoid(t.Ret):
			c.w("\t\t%s\n\t\treturn nil, true\n", call)
		case isOpt(t.Ret):
			c.w("\t\treturn %s, true\n", c.optAny(call, t.Ret))
		default:
			c.w("\t\treturn %s, true\n", call)
		}
	}
	c.w("\tdefault: // gocritic rejects the one-case switch pruning can leave\n\t\treturn nil, false\n\t}\n}\n\n")
}

// emitClassID emits cls's _ClassID and reports whether its values are pointers, whose address is their identity (rbClassRefs: #inspect, object_id).
func (c *Compiler) emitClassID(cls *Class) bool {
	if cls.universal && !cls.IsModule { // Object.new, BasicObject.new and main: heap objects of their own (#56)
		c.w("func (*%s) _ClassID() int { return %d }\n\n", cls.Name, c.classID(cls))
		c.w("func (self *%s) _Dup() any { dup := *self; return &dup }\n\n", cls.Name)
		return true
	}
	if cls.IsModule || cls.universal || cls.GoType == "" && !cls.isStruct() {
		return false
	}
	recv := c.recvType(cls)
	if cls == c.classes["Boolean"] { // a metaclass shares its class's RubyName, so compare the class itself
		c.w("func (self Boolean) _ClassID() int {\n\tif self {\n\t\treturn %d\n\t}\n\treturn %d\n}\n\n",
			c.classID(c.classes["TrueClass"]), c.classID(c.classes["FalseClass"]))
		return false
	}
	c.w("func (self %s) _ClassID() int { return %d }\n\n", recv, c.classID(cls))
	return strings.HasPrefix(recv, "*")
}

// classID is cls's index in the class list: its ID in the generated tables.
func (c *Compiler) classID(cls *Class) int {
	if cls == nil {
		return -1
	}
	if c.classIDs == nil {
		c.classIDs = make(map[*Class]int, len(c.classList))
		for i, k := range c.classList {
			c.classIDs[k] = i
		}
	}
	return c.classIDs[cls]
}

// noteArgBoxes records a T? box for each concrete type argument: generic code may hold E? (*E) of it.
func (c *Compiler) noteArgBoxes(args []Type) {
	for _, a := range args {
		var vars []string
		if freeVars(a, &vars); len(vars) > 0 {
			continue
		}
		switch a.(type) {
		case TFunc, TAny, TVoid, TUnion: // a union is any: E? of it is a member set, not a box (decision 150)
			continue
		case TClass, TNil, TOpt, TTuple, TVar: // a concrete argument type: boxed below (nil is skipped there)
		}
		if isNil(a) {
			continue
		}
		c.argBoxes["*"+c.goType(a)] = isOpt(a)
	}
}

// frozenField is the root struct's Kernel#freeze flag: lowercase with a trailing `_`, never an ivar's Go name.
const frozenField = "frozen_"

// objFreeze reports whether user or gem code names freeze at all: only then does a struct class carry the flag, so a
// program that never freezes its objects pays no byte per object for it (decision 221).
func (c *Compiler) objFreeze() bool {
	if c.objFreezeMemo == 0 {
		c.objFreezeMemo = 1
		for _, uf := range c.userFiles {
			if bytes.Contains(uf.Src, []byte("freeze")) { // ponytail: a textual scan; a call-site index would also skip comments
				c.objFreezeMemo = 2
				break
			}
		}
	}
	return c.objFreezeMemo == 2
}

// emitFrozenFlag emits rbObjFrozenFlag: the frozen flag of a struct object, by its concrete class (a switch Go compiles
// to a search on the type hash), or nil for anything else, which Kernel#freeze leaves as it is.
func (c *Compiler) emitFrozenFlag() {
	c.w("func rbObjFrozenFlag(a any) *bool {\n")
	var cases []string
	if c.objFreeze() {
		for _, cls := range c.classList {
			if cls.isStruct() && !cls.universal && !cls.IsModule && len(cls.TypeParams) == 0 && len(cls.root().TypeParams) == 0 && !cls.hashBase && !cls.byValue {
				cases = append(cases, fmt.Sprintf("\tcase *%s:\n\t\treturn &v.%s\n", cls.Name, frozenField))
			}
		}
	}
	if len(cases) > 0 {
		c.w("\tswitch v := a.(type) {\n%s\t}\n", strings.Join(cases, ""))
	}
	c.w("\treturn nil\n}\n\n")
}

// emitModIsA emits rbHasIsA_Mod for each module runtimeModuleIsA tested an untyped value against: a class-ID switch
// over the classes that include it (decision 228).
func (c *Compiler) emitModIsA() {
	for _, mod := range c.classList {
		if !c.modIsA[mod] {
			continue
		}
		var classes []*Class
		for _, k := range c.classList {
			if !k.IsModule && !k.universal && slices.Contains(k.allAncestors(), mod) {
				classes = append(classes, k)
			}
		}
		c.emitMarkers("IsA_"+mod.Name, classes)
	}
}
