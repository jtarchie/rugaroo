package compiler

import (
	"fmt"
	"slices"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

// ivarSetField: bit k records IvarList[k] was assigned (decision 153); the trailing `_` keeps it off every ivar's name.
const ivarSetField = "ivset_"

// ivarAskers answer differently for an ivar never assigned; a call on any receiver tracks every user class.
var ivarAskers = []string{"instance_variables", "instance_variable_defined?", "remove_instance_variable"}

// markIvarBits tracks only classes whose program asks, so the rest pay no field and no write.
// ponytail: an asker call tracks every user class; its receiver's static type would narrow it to one hierarchy.
func (c *Compiler) markIvarBits() {
	c.ivarBitsMarked = true
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.Node == nil || m.File == nil || m.File.isPrelude() {
				continue
			}
			anyNode(m.Node, func(n parser.Node) bool {
				if d, ok := n.(*parser.DefinedNode); ok {
					if r, ok := d.Value.(*parser.InstanceVariableReadNode); ok {
						if iv := c.findIvar(cls, r.Name); iv != nil {
							iv.Owner.ivarBits = true
						}
					}
				}
				return false
			})
		}
	}
	asks := false
	for _, uf := range c.userFiles {
		asks = asks || anyNode(uf.Root, func(n parser.Node) bool {
			call, ok := n.(*parser.CallNode)
			return ok && slices.Contains(ivarAskers, call.Name)
		})
	}
	for _, cls := range c.classList {
		if asks && cls.File != nil && !cls.File.isPrelude() && len(cls.IvarList) > 0 {
			cls.ivarBits = true
		}
		if cls.ivarBits && len(cls.IvarList) > 64 {
			c.errorAt(cls.srcPos, "%s has %d instance variables; rugaroo records which were assigned in 64 bits (decision 153)", cls.RubyName, len(cls.IvarList))
		}
	}
}

// ivarPath reaches iv's field through its owner's accessor: a parent's embedded struct or a module's ivar struct alike.
func ivarPath(recv string, iv *Ivar) string {
	if iv.scratch {
		return scratchIvar(iv.Name)
	}
	if iv.Owner.byValue { // the field itself: a value's _C() is a copy, so initialize's writes go through its *C (decision 441)
		return recv + "." + goFieldName(iv.Name)
	}
	return fmt.Sprintf("%s._%s().%s", recv, iv.Owner.Name, goFieldName(iv.Name))
}

// ivarBit is "" when iv's owner keeps no bits.
func ivarBit(recv string, iv *Ivar) (field, mask string) {
	if iv.scratch || !iv.Owner.ivarBits {
		return "", ""
	}
	return fmt.Sprintf("%s._%s().%s", recv, iv.Owner.Name, ivarSetField), fmt.Sprintf("1<<%d", slices.Index(iv.Owner.IvarList, iv))
}

func ivarAssigned(recv string, iv *Ivar) string {
	if field, mask := ivarBit(recv, iv); field != "" {
		return fmt.Sprintf("%s |= %s", field, mask)
	}
	return ""
}

func ivarIsSet(recv string, iv *Ivar) string {
	if field, mask := ivarBit(recv, iv); field != "" {
		return fmt.Sprintf("%s&(%s) != 0", field, mask)
	}
	return ""
}

func (f *fctx) markIvar(iv *Ivar) {
	if s := ivarAssigned(f.selfCode, iv); s != "" {
		f.emit("%s", s)
	}
}

// scratchIvar is the Go local holding a scratch ivar for one call.
func scratchIvar(name string) string { return "rbIvS_" + goFieldName(name) }
