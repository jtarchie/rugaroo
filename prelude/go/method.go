//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"strings"
)

// rbParamInfo is one Method#parameters entry; name "" is a C method's anonymous parameter.
type rbParamInfo struct{ kind, name string }

// rbMethodInfo is what the compiler knows of a Method's target at the call site (decision 141).
type rbMethodInfo struct {
	name   string
	key    string // owner and name: equal methods share it
	owner  ModuleI
	params []rbParamInfo
	arity  int
	loc    string // " file:line" of a user def; MRI shows none for C methods, which the prelude's stand for
	single bool   // a singleton method: Recv.name, its owner a rbSingletonClass
	known  bool   // false: taken from an untyped receiver, so only name and receiver are known
}

// Method_Any is the untyped view a class switch and rbFrom ask for.
type Method_Any interface{ _ToAny() *Method[any] }

// UnboundMethod_Any is Method_Any for UnboundMethod.
type UnboundMethod_Any interface{ _ToAny() *UnboundMethod[any] }

// rbSingletonClass is a singleton method's owner, MRI's #<Class:Foo>; rugaroo has no other use for singleton classes.
type rbSingletonClass struct {
	Class
	of ModuleI
}

func (*rbSingletonClass) Name() String      { return "" }
func (s *rbSingletonClass) ToS() String     { return "#<Class:" + s.of.Name() + ">" }
func (s *rbSingletonClass) Inspect() String { return s.ToS() }

// rbFrom converts a Method of another instantiation: back to its typed func, or to Method[untyped].
func (*Method[F]) rbFrom(v any) (*Method[F], bool) {
	m, ok := v.(Method_Any)
	if !ok {
		return nil, false
	}
	a := m._ToAny()
	fn, ok := a.fn.(F)
	if !ok {
		return nil, false
	}
	return &Method[F]{fn: fn, dyn: a.dyn, recv: a.recv, info: a.info}, true
}

// rbFrom is Method's for UnboundMethod.
func (*UnboundMethod[F]) rbFrom(v any) (*UnboundMethod[F], bool) {
	m, ok := v.(UnboundMethod_Any)
	if !ok {
		return nil, false
	}
	a := m._ToAny()
	fn, ok := a.fn.(F)
	if !ok {
		return nil, false
	}
	return &UnboundMethod[F]{fn: fn, dyn: a.dyn, info: a.info}, true
}

// rbMethodCallDyn calls through the dynamic dispatcher: the method's own arity and argument checks apply.
func rbMethodCallDyn(dyn func(any, ...any) any, recv any, info *rbMethodInfo, args []any) any {
	if dyn == nil {
		panic(NewNotImplementedError(Ref(String("rugaroo: " + info.name + " cannot be called without its signature (decision 141)"))))
	}
	return dyn(recv, args...)
}

// rbMethodKnown is info, or NotImplementedError for what a Method taken from an untyped receiver cannot know.
func rbMethodKnown(info *rbMethodInfo, what string) *rbMethodInfo {
	if !info.known {
		panic(NewNotImplementedError(Ref(String("rugaroo: Method#" + what + " of a method taken from an untyped receiver is not known at compile time (decision 141)"))))
	}
	return info
}

// rbMethodParams is Method#parameters.
func rbMethodParams(info *rbMethodInfo) *Array[*Array[Symbol]] {
	out := &Array[*Array[Symbol]]{s: make([]*Array[Symbol], 0, len(info.params))}
	for _, p := range info.params {
		e := &Array[Symbol]{s: []Symbol{Symbol(p.kind)}}
		if p.name != "" {
			e.s = append(e.s, Symbol(p.name))
		}
		out.s = append(out.s, e)
	}
	return out
}

// rbMethodUnbind is Method#unbind: the receiver goes, the dispatcher stays.
func rbMethodUnbind[F comparable](m *Method[F]) *UnboundMethod[any] {
	return &UnboundMethod[any]{dyn: m.dyn, info: m.info}
}

// rbUnboundBind is UnboundMethod#bind on an untyped one, MRI's TypeError for a receiver that is not an owner's instance.
func rbUnboundBind(dyn func(any, ...any) any, info *rbMethodInfo, obj any) *Method[any] {
	if s, ok := rbMethodKnown(info, "bind").owner.(*rbSingletonClass); ok {
		if !rbIdentical(rbUnbox(obj), s.of) {
			panic(NewTypeError(Ref(String("singleton method called for a different object"))))
		}
	} else if !rbIsInstanceOf(info.owner, obj) {
		panic(NewTypeError(Ref(String("bind argument must be an instance of " + string(info.owner.Name())))))
	}
	return &Method[any]{dyn: dyn, recv: obj, info: info}
}

// rbMethodEq is Method#== (bound) and UnboundMethod#==: the same definition, and for a Method the same receiver.
func rbMethodEq(bound bool, recv any, info *rbMethodInfo, other any) bool {
	if bound {
		o, ok := other.(Method_Any)
		if !ok {
			return false
		}
		a := o._ToAny()
		return a.info.key == info.key && rbIdentical(rbUnbox(a.recv), rbUnbox(recv))
	}
	o, ok := other.(UnboundMethod_Any)
	return ok && o._ToAny().info.key == info.key
}

// rbMethodHash hashes the receiver by identity, as rbMethodEq compares it: its own #hash changes when it is mutated.
func rbMethodHash(recv any, info *rbMethodInfo) Integer {
	h := Integer(rbKeyHash(String(info.key)))
	if recv != nil {
		h = h*31 + Integer(rbHashComparable(rbUnbox(recv)))
	}
	return h
}

// rbMethodInspect is MRI 4.0's #<Method: Recv(Owner)#name(params) file:line>.
func rbMethodInspect(kind string, recv any, info *rbMethodInfo) String {
	var b strings.Builder
	b.WriteString("#<" + kind + ": ")
	switch {
	case !info.known:
		b.WriteString(rbClassName(recv) + "#" + info.name + "(?)>")
		return String(b.String())
	case kind == "UnboundMethod":
		b.WriteString(string(info.owner.Inspect()) + "#")
	case info.single:
		self, of := string(rbInspect(recv)), string(info.owner.(*rbSingletonClass).of.Name())
		b.WriteString(self)
		if self != of {
			b.WriteString("(" + of + ")")
		}
		b.WriteString(".")
	default:
		cls, own := rbClassName(recv), string(info.owner.Name())
		b.WriteString(cls)
		if cls != own {
			b.WriteString("(" + own + ")")
		}
		b.WriteString("#")
	}
	b.WriteString(info.name + "(" + rbParamsText(info.params) + ")" + info.loc + ">")
	return String(b.String())
}

// rbParamsText renders parameters as MRI's inspect does: `...` for anonymous *, ** and &.
func rbParamsText(ps []rbParamInfo) string {
	if len(ps) == 3 && ps[0] == (rbParamInfo{"rest", "*"}) && ps[1] == (rbParamInfo{"keyrest", "**"}) && ps[2] == (rbParamInfo{"block", "&"}) {
		return "..."
	}
	parts := make([]string, len(ps))
	for i, p := range ps {
		switch p.kind {
		case "req":
			parts[i] = p.name
			if p.name == "" {
				parts[i] = "_"
			}
		case "opt":
			parts[i] = p.name + "=..."
		case "rest":
			parts[i] = "*" + strings.TrimPrefix(p.name, "*")
		case "keyreq":
			parts[i] = p.name + ":"
		case "key":
			parts[i] = p.name + ": ..."
		case "keyrest":
			parts[i] = "**" + strings.TrimPrefix(p.name, "**")
		case "block":
			parts[i] = "&" + strings.TrimPrefix(p.name, "&")
		}
	}
	return strings.Join(parts, ", ")
}
