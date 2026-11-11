package compiler

import (
	"slices"
	"strconv"

	parser "github.com/danielgatis/go-ruby-prism/parser"
)

// identityNames tell MRI's one object from its copies: any of them in user or gem code keeps Data a pointer (decision 441).
var identityNames = []string{"equal?", "object_id", "__id__", "compare_by_identity", "assert_same", "refute_same",
	"must_be_same_as", "wont_be_same_as", "instance_variable_set", "remove_instance_variable"}

// valueOps are the typed calls a `# @value` class refuses: identity, and the writes MRI makes on a frozen-able object.
var valueOps = []string{"equal?", "object_id", "__id__", "freeze", "instance_variable_set", "remove_instance_variable"}

// valueAnnRecv is the `# @value` class a receiver is typed as (self inside its own methods too), or nil.
func (f *fctx) valueAnnRecv(recv expr) *Class {
	var k *Class
	switch t := stripOpt(recv.typ).(type) {
	case TClass:
		k = t.C
	case TVar:
		if t.Name == "Self" {
			k = f.owner
		}
	}
	if k == nil || !k.valueAnn || !k.byValue {
		return nil
	}
	return k
}

// valueClassNames are the value classes' Go names, in class order.
func (c *Compiler) valueClassNames() []string {
	var out []string
	for _, k := range c.classList {
		if k.byValue {
			out = append(out, k.Name)
		}
	}
	return out
}

// markValueClasses runs once ivars are final and before any body is emitted, so a class's Go type is decided once (decision 441).
func (c *Compiler) markValueClasses() {
	scanned, identity := false, false
	for _, cls := range c.classList {
		data := cls.value != nil && cls.value.kind == valueData
		if !data && !cls.valueAnn {
			continue
		}
		why := c.valueBlocker(cls)
		if why == "" && !cls.valueAnn {
			if !scanned {
				scanned, identity = true, c.observesIdentity()
			}
			if identity {
				why = "identity"
			}
		}
		if why != "" {
			if cls.valueAnn {
				c.errorAt(cls.srcPos, "%s is marked # @value but %s (decision 441)", cls.RubyName, why)
			}
			continue
		}
		cls.byValue = true
	}
}

// observesIdentity reports an identity call or ObjectSpace/WeakRef anywhere in user or gem code (identityNames).
func (c *Compiler) observesIdentity() bool {
	for _, f := range c.files {
		if f.isPrelude() || f.valueGen || f.Root == nil {
			continue
		}
		if anyNode(f.Root, func(n parser.Node) bool {
			switch n := n.(type) {
			case *parser.CallNode:
				return slices.Contains(identityNames, n.Name)
			case *parser.ConstantReadNode:
				return n.Name == "ObjectSpace" || n.Name == "WeakRef" || n.Name == "Marshal" // Marshal: a value class has no dump case (ponytail)
			}
			return false
		}) {
			return true
		}
	}
	return false
}

// valueBlocker is why cls cannot be a value struct, or "".
func (c *Compiler) valueBlocker(cls *Class) string {
	switch {
	case !cls.leaf():
		return "it is not a leaf class (a subclass, a type parameter or a user module)"
	case cls.isPerRun:
		return "it is a per-run Class.new class"
	case cls.ivarBits:
		return "the program asks which of its instance variables are set"
	case cls.Super != nil && !cls.Super.universal && cls.Super.RubyName != "Data":
		return "it has a superclass"
	case len(cls.IvarList) > 4:
		return "it has more than 4 instance variables (Go keeps at most 4 fields of a struct in registers)"
	}
	size, align := 0, 1
	for _, iv := range cls.IvarList {
		if iv.Type == nil || mentionsValueCandidate(iv.Type) {
			return "@" + iv.Name[1:] + " holds a class that may itself be a value"
		}
		s, a := c.valueFieldSize(iv.Type)
		if s == 0 {
			return iv.Name + " is " + iv.Type.String() + ", which a value struct does not hold"
		}
		size = (size+a-1)/a*a + s
		align = max(align, a)
	}
	if size = (size + align - 1) / align * align; size > 32 {
		return "its fields take more than 32 bytes (Go keeps at most 32 bytes of a struct in registers)"
	}
	data := cls.value != nil
	for _, m := range cls.MethodList {
		switch {
		case m.Kind == kindAttrWriter:
			return "it has a writer, " + m.Name
		case m.Node == nil || m.valueGen:
		case m.Name == "initialize" && data:
			return "its Data block defines initialize"
		case m.Name == "initialize":
			if n := selfAsValue(m.Node.Body); n != nil {
				return "its initialize uses self as a value (line " + strconv.Itoa(m.File.line(n.GetLocation().StartOffset)) + ")"
			}
		default:
			if n := ivarWrite(m.Node.Body); n != nil {
				return m.Name + " writes an instance variable (line " + strconv.Itoa(m.File.line(n.GetLocation().StartOffset)) + ")"
			}
		}
	}
	return ""
}

// mentionsValueCandidate reports a Data or `# @value` class in t: its Go type is still being decided.
func mentionsValueCandidate(t Type) bool {
	switch t := t.(type) {
	case TClass:
		return t.C.value != nil && t.C.value.kind == valueData || t.C.valueAnn || slices.ContainsFunc(t.Args, mentionsValueCandidate)
	case TOpt:
		return mentionsValueCandidate(t.Elem)
	case TTuple:
		return slices.ContainsFunc(t.Elems, mentionsValueCandidate)
	case TUnion:
		return slices.ContainsFunc(t.Members, mentionsValueCandidate)
	case TFunc:
		return true // a Proc field is refused anyway
	case TAny, TNil, TVar, TVoid:
	}
	return false
}

// valueFieldSize is 0 for a field a value struct refuses: a Proc, untyped, anything whose size is not known here.
func (c *Compiler) valueFieldSize(t Type) (size, align int) {
	if valOpt(t) {
		return 16, 8
	}
	switch g := c.goType(t); {
	case g == "Integer", g == "Float":
		return 8, 8
	case g == "Boolean":
		return 1, 1
	case g == "String", g == "Symbol":
		return 16, 8
	case len(g) > 1 && g[0] == '*':
		return 8, 8
	}
	if k, ok := stripOpt(t).(TClass); ok && ifaceObj(k.C) {
		return 16, 8
	}
	return 0, 0
}

// ivarWrite is the first instance variable write in n, or nil.
func ivarWrite(n parser.Node) parser.Node {
	var hit parser.Node
	walkNode(n, func(x parser.Node) (bool, bool) {
		switch x.(type) {
		case *parser.InstanceVariableWriteNode, *parser.InstanceVariableOperatorWriteNode, *parser.InstanceVariableOrWriteNode,
			*parser.InstanceVariableAndWriteNode, *parser.InstanceVariableTargetNode:
			hit = x
			return true, false
		}
		return false, !opensScope(x)
	})
	return hit
}

// selfAsValue is the first `self` in n that is not a call's receiver: initialize's self is still being built.
func selfAsValue(n parser.Node) parser.Node {
	var hit parser.Node
	walkNode(n, func(x parser.Node) (bool, bool) {
		if call, ok := x.(*parser.CallNode); ok {
			if _, ok := call.Receiver.(*parser.SelfNode); ok {
				for _, ch := range x.CompactChildNodes() {
					if ch != call.Receiver && hit == nil {
						hit = selfAsValue(ch)
					}
				}
				return hit != nil, false
			}
		}
		if _, ok := x.(*parser.SelfNode); ok {
			hit = x
			return true, false
		}
		return false, !opensScope(x)
	})
	return hit
}
