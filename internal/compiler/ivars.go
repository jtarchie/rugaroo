package compiler

import (
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ivarKey names one class's ivar in the discovery-time sets (nilIvars, ivarReadFirst).
type ivarKey struct {
	cls  *Class
	name string
}

// declareIvarAnnotations declares each `# @rbs @x: T`, and each
// `# @rbs self.@x: T` on the class object.
func (c *Compiler) declareIvarAnnotations() {
	for _, cls := range c.classList {
		for _, d := range cls.ivarDecls {
			t := c.resolveType(d.rbs, typeScope{class: cls, lex: d.scope, srcPos: srcPos{cls.File, d.line}})
			owner, name := cls, d.name
			if iv, ok := strings.CutPrefix(name, "self."); ok {
				owner, name = cls.meta, iv
			}
			c.ivarDeclWhy = "an annotation, or the gem's .rbs signature of the attribute (decision 160; the .rbs file is not recorded), says so: "
			c.declareIvar(owner, name, t, cls.File, d.line)
			c.ivarDeclWhy = ""
		}
	}
}

// declareIvar records an ivar on the topmost class of the chain that
// mentions it, so subclasses share the parent's field.
func (c *Compiler) declareIvar(cls *Class, name string, t Type, f *File, line int) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			narrows := isAny(iv.Type) || isNil(t) && holdsNil(iv.Type) || unionHas(iv.Type, t) || // a gem subclass's reader narrowing the parent's (`returns(String)` over Integer | String | nil)
				typeEq(t, optOf(iv.Type)) // an attr_writer-only T? (decision 323) of an ivar its class body already set: Livecheck's `@processed_urls = T.let({}, ...)`
			if !typeEq(iv.Type, t) && (!narrows || !c.gemFile(f)) {
				c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s has it as %s", name, t, k.Name, iv.Type)
			}
			return iv
		}
	}
	for _, mod := range ivarModules(cls) { // an included module's (decision 147): Homebrew's Downloadable `attr_reader :checksum`
		if iv := mod.Ivars[name]; iv != nil { // and Resource's `attr_writer :checksum` are one @checksum, as one object's are in MRI
			if !typeEq(iv.Type, t) && !isAny(iv.Type) && !c.gemFile(f) {
				c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s has it as %s", name, t, mod.Name, iv.Type)
			}
			return iv
		}
	}
	if iv := c.siblingIvar(cls, name, t, f, line); iv != nil {
		return iv
	}
	if c.nilIvars[ivarKey{cls, name}] {
		t = optOf(t) // this class assigns it nil too
	}
	for k := cls.Super; k != nil; k = k.Super { // an ancestor that assigned it only nil owns it, nilable (decision 182)
		if c.nilIvars[ivarKey{k, name}] {
			cls, t = k, optOf(t)
		}
	}
	if f != nil && c.gemFile(f) {
		for k := cls.Super; k != nil; k = k.Super { // a gem ancestor's attr_reader of it owns it: Formula's `class << self; attr_reader :loaded_from_api` (decision 259)
			if m := k.Methods[strings.TrimPrefix(name, "@")]; m != nil && (m.Kind == kindAttrReader && m.Attr == name || readsOnlyIvar(m, name)) {
				cls, t = k, optOf(t)
			}
		}
	}
	if cls.IsModule {
		if iv := c.coIncludedIvar(cls, name, t, f, line); iv != nil {
			return iv
		}
	}
	iv := &Ivar{Name: name, Type: t, Owner: cls}
	cls.Ivars[name] = iv
	cls.IvarList = append(cls.IvarList, iv)
	if holdsAny(t) {
		c.noteIvar(iv, srcPos{f, line}, c.ivarDeclWhy+"declared "+typeString(t), untypedRoot{})
	}
	return iv
}

// subclassIvar moves an ivar a subclass declared up to cls, nilable, when cls assigns it only nil: one field holds it
// for the whole hierarchy, as one object's ivar is the same name in every method MRI runs on it.
func (c *Compiler) subclassIvar(cls *Class, name string) *Ivar {
	for _, sub := range cls.Subclasses {
		iv := sub.Ivars[name]
		if iv == nil {
			if moved := c.subclassIvar(sub, name); moved != nil {
				iv = moved
			} else {
				continue
			}
		}
		delete(iv.Owner.Ivars, name)
		iv.Owner.IvarList = slices.DeleteFunc(iv.Owner.IvarList, func(x *Ivar) bool { return x == iv })
		iv.Owner, iv.Type = cls, optOf(iv.Type)
		cls.Ivars[name] = iv
		cls.IvarList = append(cls.IvarList, iv)
		return iv
	}
	return nil
}

func (c *Compiler) findIvar(cls *Class, name string) *Ivar {
	for k := cls; k != nil; k = k.Super {
		if iv := k.Ivars[name]; iv != nil {
			return iv
		}
	}
	for _, mod := range ivarModules(cls) { // an included module's (decision 147)
		if iv := mod.Ivars[name]; iv != nil {
			return iv
		}
	}
	if cls.IsModule {
		for _, p := range c.coIncluded(cls) { // a sibling module every includer also has (decision 497)
			if iv := p.Ivars[name]; iv != nil {
				return iv
			}
		}
	}
	return nil
}

// siblingIvar is module cls's @name held by a module every includer of cls also includes (decision 497).
func (c *Compiler) siblingIvar(cls *Class, name string, t Type, f *File, line int) *Ivar {
	if !cls.IsModule {
		return nil
	}
	for _, p := range c.coIncluded(cls) {
		if iv := p.Ivars[name]; iv != nil {
			if !typeEq(iv.Type, t) && !isAny(iv.Type) && !c.gemFile(f) {
				c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s, which every includer of %s also includes, has it as %s: one object has one %s (decision 497)", name, t, p.RubyName, cls.RubyName, iv.Type, name)
			}
			return iv
		}
	}
	return nil
}

// coIncluded lists the other modules every class including mod also includes: rubocop's FirstHashElementIndentation
// includes Alignment and MultilineElementIndentation, both of which set @column_delta, one ivar per object in MRI. A
// module's ivar another of these holds is that one's field (decision 497). Empty for a module no struct class includes.
func (c *Compiler) coIncluded(mod *Class) []*Class {
	if out, ok := c.coIncludedMemo[mod]; ok {
		return out
	}
	var out []*Class
	first := true
	for _, k := range c.classList {
		if k.IsModule || !k.isStruct() || k.universal {
			continue
		}
		anc := k.allAncestors()
		if !slices.Contains(anc, mod) {
			continue
		}
		var mods []*Class
		for _, a := range anc {
			if a.IsModule && a != mod && !a.universal && len(a.TypeParams) == 0 && !slices.Contains(mod.allAncestors(), a) {
				mods = append(mods, a)
			}
		}
		if first {
			out, first = mods, false
		} else {
			out = slices.DeleteFunc(out, func(a *Class) bool { return !slices.Contains(mods, a) })
		}
		if len(out) == 0 {
			break
		}
	}
	if c.coIncludedMemo == nil {
		c.coIncludedMemo = map[*Class][]*Class{}
	}
	c.coIncludedMemo[mod] = out
	return out
}

// coIncludedIvar moves @name, declared earlier by a module whose includers all include mod, onto mod, its type joined
// with t: the module declaring it second still makes one field (decision 497).
func (c *Compiler) coIncludedIvar(mod *Class, name string, t Type, f *File, line int) *Ivar {
	for _, other := range c.classList {
		if !other.IsModule || other == mod || !slices.Contains(c.coIncluded(other), mod) {
			continue
		}
		iv := other.Ivars[name]
		if iv == nil {
			continue
		}
		if !typeEq(iv.Type, t) && !isAny(iv.Type) && !isAny(t) && !c.gemFile(f) {
			c.errorAt(srcPos{f, line}, "ivar %s declared as %s but %s, which every includer of %s also includes, has it as %s: one object has one %s (decision 497)", name, t, other.RubyName, mod.RubyName, iv.Type, name)
		}
		delete(other.Ivars, name)
		other.IvarList = slices.DeleteFunc(other.IvarList, func(x *Ivar) bool { return x == iv })
		iv.Owner = mod
		if !typeEq(iv.Type, t) {
			iv.Type = TAny{} // gem code: the two writes' classes differ, so untyped, as a gem module's undeclared ivar is (decision 172)
		}
		mod.Ivars[name] = iv
		mod.IvarList = append(mod.IvarList, iv)
		return iv
	}
	return nil
}

// declareComputedIvars declares, untyped on the method's class, each ivar a gem method sets by a name built from one
// of its parameters (`instance_variable_set("@tl_#{sym}", v)`, bindata's top_level_set) when every call of a method
// by that name on self passes a Symbol or String literal there: the names are those literals behind the prefix (decision 324).
// A local the body binds once to a literal names the ivar itself: a define_method's baked-in capture (ruby-macho's
// def_mask_reader, decision 325).
func (c *Compiler) declareComputedIvars() {
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) {
				continue
			}
			lits := literalLocals(m.Node.Body)
			anyNode(m.Node.Body, func(n parser.Node) bool {
				prefix, local, ok := computedIvarSet(n)
				if !ok {
					return false
				}
				var names []string
				if lit, isLit := lits[local]; isLit {
					names = []string{lit}
				} else if param := requiredIndex(m, local); param >= 0 {
					names, ok = c.literalSelfArgs(m.Name, param)
				}
				for _, name := range names {
					if ok && c.findIvar(cls, prefix+name) == nil {
						c.declareIvar(cls, prefix+name, TAny{}, m.File, m.File.line(m.Node.Location.StartOffset))
					}
				}
				return false
			})
		}
	}
}

// computedIvarSet matches `instance_variable_set("@prefix#{local}", v)`: the prefix and the local's name.
func computedIvarSet(n parser.Node) (string, string, bool) {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "instance_variable_set" || len(callArgs(call)) != 2 {
		return "", "", false
	}
	str, ok := callArgs(call)[0].(*parser.InterpolatedStringNode)
	if !ok || len(str.Parts) != 2 {
		return "", "", false
	}
	lit, ok1 := str.Parts[0].(*parser.StringNode)
	emb, ok2 := str.Parts[1].(*parser.EmbeddedStatementsNode)
	if !ok1 || !ok2 || emb.Statements == nil || len(emb.Statements.Body) != 1 || !strings.HasPrefix(lit.Unescaped.Value, "@") {
		return "", "", false
	}
	rd, ok := emb.Statements.Body[0].(*parser.LocalVariableReadNode)
	if !ok || rd.Depth != 0 {
		return "", "", false
	}
	return lit.Unescaped.Value, rd.Name, true
}

// requiredIndex is the position of m's required parameter name, or -1.
func requiredIndex(m *Method, name string) int {
	if m.Node.Parameters == nil {
		return -1
	}
	for i, p := range m.Node.Parameters.Requireds {
		if r, ok := p.(*parser.RequiredParameterNode); ok && r.Name == name {
			return i
		}
	}
	return -1
}

// literalLocals maps each local body writes exactly once, with a Symbol or String literal, to that literal.
func literalLocals(body parser.Node) map[string]string {
	out, writes := map[string]string{}, map[string]int{}
	anyNode(body, func(n parser.Node) bool {
		switch w := n.(type) {
		case *parser.LocalVariableWriteNode:
			writes[w.Name]++
			if lit := literalName(w.Value); lit != "" {
				out[w.Name] = lit
			}
		case *parser.LocalVariableOperatorWriteNode:
			writes[w.Name] += 2
		case *parser.LocalVariableOrWriteNode:
			writes[w.Name] += 2
		case *parser.LocalVariableAndWriteNode:
			writes[w.Name] += 2
		case *parser.LocalVariableTargetNode:
			writes[w.Name] += 2
		}
		return false
	})
	for name, k := range writes {
		if k != 1 {
			delete(out, name)
		}
	}
	return out
}

// literalSelfArgs is the Symbol or String literal every receiverless (or self) call of name passes at position param;
// false when one passes anything else.
func (c *Compiler) literalSelfArgs(name string, param int) ([]string, bool) {
	var names []string
	ok := true
	for _, f := range c.files {
		anyNode(f.Root, func(n parser.Node) bool {
			call, isCall := n.(*parser.CallNode)
			if !isCall || call.Name != name {
				return false
			}
			if _, self := call.Receiver.(*parser.SelfNode); call.Receiver != nil && !self { // another object's method of that name
				return false
			}
			lit := ""
			if args := callArgs(call); param < len(args) {
				lit = literalName(args[param])
			}
			if lit == "" {
				ok = false
				return true
			}
			names = append(names, lit)
			return false
		})
	}
	return names, ok
}

// usesIvarOf reports whether mod's methods name an ivar p holds: its constraint then requires p's accessor.
func (c *Compiler) usesIvarOf(mod, p *Class) bool {
	for _, m := range mod.MethodList {
		if m.Node != nil && anyNode(m.Node, func(n parser.Node) bool { name, _ := ivarNodeName(n); return name != "" && p.Ivars[name] != nil }) {
			return true
		}
	}
	return false
}
