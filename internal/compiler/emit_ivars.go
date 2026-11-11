package compiler

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// emitIvarList feeds Kernel#inspect; a class adding no ivars inherits its parent's through embedding.
func (c *Compiler) emitIvarList(cls *Class) {
	if len(cls.IvarList) == 0 && len(ownIvarModules(cls)) == 0 {
		return
	}
	var ivs []string
	for _, iv := range c.ivarOrder(cls) {
		field := ivarPath("self", iv)
		val, opt, isNilCode := field, isAny(iv.Type), "false"
		switch t := iv.Type.(type) {
		case TOpt:
			val, opt = c.optAny(val, t), true
		case TFunc:
			isNilCode = field + " == nil"
		case TVar:
			isNilCode = "rbUnbox(any(" + field + ")) == nil"
		case TClass:
			if t.C.isStruct() && !t.C.byValue || t.C.mutable() { // an interface or a pointer
				isNilCode = field + " == nil"
			}
		case TUnion: // an any, like untyped
			opt = true
		case TAny, TNil, TTuple, TVoid: // held by value: never a nil pointer of its own
		}
		if set := ivarIsSet("self", iv); set != "" { // the bit, not the value, says whether it was assigned
			ivs = append(ivs, fmt.Sprintf("{%q, %s, %s, !(%s)}", iv.Name, val, set, set))
			continue
		}
		ivs = append(ivs, fmt.Sprintf("{%q, %s, %t, %s}", iv.Name, val, opt, isNilCode))
	}
	// One name's entry comes from a switch, so instance_variable_get does not build the whole list (decision 332).
	if len(ivs) == 0 {
		c.w("func (self %s) _Ivars(string) []rbIvar { return nil }\n\n", c.recvType(cls))
	} else {
		c.w("func (self %s) _Ivars(name string) []rbIvar {\n\tswitch name {\n\tcase \"\":\n\t\treturn []rbIvar{%s}\n", c.recvType(cls), strings.Join(ivs, ", "))
		seen := map[string]bool{}
		for i, iv := range c.ivarOrder(cls) {
			if !seen[iv.Name] { // the first, as the loop over the whole list finds
				seen[iv.Name] = true
				c.w("\tcase %q:\n\t\treturn []rbIvar{%s}\n", iv.Name, ivs[i])
			}
		}
		c.w("\t}\n\treturn nil\n}\n\n")
	}
	if cls.byValue { // frozen: instance_variable_set has nothing to write (rbIvarSet raises)
		return
	}
	// Kernel#instance_variable_set: the closed world knows every ivar's type, so a write converts to it
	c.w("func (self *%s) _IvarSet(name string, v any) bool {\n\tswitch name {\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		field := "&" + ivarPath("self", iv)
		o, ok := iv.Type.(TOpt)
		switch {
		case ok && leafOpt(o):
			c.w("\tcase %q:\n\t\trbIvarAssignPtr(%s, v, %q)\n", iv.Name, field, iv.Name)
		case ok && valOpt(o):
			c.w("\tcase %q:\n\t\trbIvarAssignVal(%s, v, %q)\n", iv.Name, field, iv.Name)
		case ok:
			c.w("\tcase %q:\n\t\trbIvarAssignOpt[%s](%s, v, %q)\n", iv.Name, c.goType(o.Elem), field, iv.Name)
		default:
			c.w("\tcase %q:\n\t\trbIvarAssign(%s, v, %q)\n", iv.Name, field, iv.Name)
		}
		if s := ivarAssigned("self", iv); s != "" {
			c.w("\t\t%s\n", s)
		}
	}
	c.w("\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n")
	c.emitIvarDel(cls)
}

// emitIvarDel feeds Kernel#remove_instance_variable: the field back to its zero value and its bit cleared.
func (c *Compiler) emitIvarDel(cls *Class) {
	var cases []string
	for _, iv := range c.ivarOrder(cls) {
		if field, mask := ivarBit("self", iv); field != "" {
			cases = append(cases, fmt.Sprintf("\tcase %q:\n\t\trbIvarClear(&%s)\n\t\t%s &^= %s\n", iv.Name, ivarPath("self", iv), field, mask))
		}
	}
	if len(cases) == 0 {
		return
	}
	c.w("func (self *%s) _IvarDel(name string) {\n\tswitch name {\n%s\tdefault: // an ivar the class lacks: nothing to clear (and gocritic rejects a one-case switch)\n\t}\n}\n\n", cls.Name, strings.Join(cases, ""))
}

// emitCopy feeds Ractor's deep copy (decision 103): a fresh struct with every
// ivar copied through rbCopyAs, identity kept through seen. Pruned with
// rbCopyDeep when no Ractor is used.
func (c *Compiler) emitCopy(cls *Class) {
	if cls.byValue { // a value has no identity to keep: its fields are copied into the copy Go already made
		c.w("func (self %s) _Copy(seen map[any]any) any {\n", cls.Name)
		for _, iv := range c.ivarOrder(cls) {
			c.w("\t%s = rbCopyAs(%s, seen)\n", ivarPath("self", iv), ivarPath("self", iv))
		}
		c.w("\treturn self\n}\n\n")
		return
	}
	c.w("func (self *%s) _Copy(seen map[any]any) any {\n\tif self == nil {\n\t\treturn self\n\t}\n\tdup := *self\n\tseen[self] = &dup\n", cls.Name)
	for _, iv := range c.ivarOrder(cls) {
		c.w("\t%s = rbCopyAs(%s, seen)\n", ivarPath("dup", iv), ivarPath("self", iv))
	}
	c.w("\treturn &dup\n}\n\n")
}

// emitDup is Object#dup for a struct class: a shallow copy, then the class's own initialize_copy (rbDup).
func (c *Compiler) emitDup(cls *Class) {
	if cls.byValue { // no initialize_copy: markValueClasses refused one
		c.w("func (self %s) _Dup() any { return self }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Dup() any {\n\tdup := *self\n", cls.Name)
	call := func(e *entry) { // `def initialize_copy` without the source (ruby/spec's def_spec): MRI's ArgumentError, as dup passes it (decision 416)
		if len(e.M.Params) == 0 {
			c.w("\tpanic(NewArgumentError(Ref(String(\"wrong number of arguments (given 1, expected 0)\"))))\n")
			return
		}
		c.w("\t%s%s(&dup, self)\n", freeFuncName(e.M), c.forwardTypeArgs(*e, cls))
	}
	if e := cls.lookup("initialize_dup"); e != nil && e.M.File != nil && !e.M.File.isPrelude() {
		// ponytail: its super reaches Kernel's no-op, not the class's initialize_copy as MRI's Object#initialize_dup would
		call(e)
	} else if e := cls.lookup("initialize_copy"); e != nil && e.M.File != nil && !e.M.File.isPrelude() {
		call(e) // private: no forwarder
	}
	c.w("\treturn &dup\n}\n\n")
}

// ivarOrder approximates MRI's (first assignment) with initialize's write order, super splicing in the parent's.
func (c *Compiler) ivarOrder(cls *Class) []*Ivar {
	var out []*Ivar
	seen := map[string]bool{} // by name: a subclass's and an ancestor's @x are one Ruby ivar
	add := func(iv *Ivar) {
		if iv != nil && !seen[iv.Name] {
			seen[iv.Name] = true
			out = append(out, iv)
		}
	}
	var fromInit func(k *Class)
	fromInit = func(k *Class) {
		if k == nil || k.universal {
			return
		}
		e := k.lookup("initialize")
		if e == nil || e.M.Node == nil {
			return
		}
		var walk func(n parser.Node)
		walk = func(n parser.Node) {
			if n == nil {
				return
			}
			for _, ch := range n.CompactChildNodes() {
				walk(ch)
			}
			switch n := n.(type) {
			case *parser.InstanceVariableWriteNode:
				add(c.findIvar(cls, n.Name))
			case *parser.InstanceVariableOrWriteNode:
				add(c.findIvar(cls, n.Name))
			case *parser.SuperNode, *parser.ForwardingSuperNode:
				fromInit(e.Owner.Super)
			}
		}
		walk(e.M.Node.Body)
	}
	fromInit(cls)
	chain := cls.structChain()
	for i := len(chain) - 1; i >= 0; i-- {
		for _, iv := range chain[i].IvarList {
			add(iv)
		}
	}
	for _, mod := range ivarModules(cls) { // decision 147's, in each includer's struct
		for _, iv := range mod.IvarList {
			add(iv)
		}
	}
	return out
}
