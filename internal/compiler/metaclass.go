package compiler

import (
	"github.com/danielgatis/go-ruby-prism/parser"
)

// buildMetas gives every struct class, and every class or module with
// `def self.` methods, a metaclass: a struct class whose methods are the
// class methods, whose superclass is the parent's metaclass (so class
// methods inherit and dispatch virtually), and whose single instance is the
// class object.
func (c *Compiler) buildMetas() {
	for _, cls := range append([]*Class(nil), c.classList...) {
		if (cls.RubyName == "BasicObject" || cls.RubyName == "Kernel") && len(cls.singletonDefs) > 0 {
			c.errorf(cls.singletonDefs[0].file, cls.singletonDefs[0].node, "class methods on %s are not supported", cls.RubyName)
		}
		if cls.metaOf == nil {
			c.metaFor(cls)
		}
	}
	// `extend M` is `include M` on the class object; resolved once every
	// class object exists, since its type args may name them.
	c.linkExtends()
}

func (c *Compiler) metaFor(cls *Class) *Class {
	if cls.meta != nil {
		return cls.meta
	}
	if cls.template != nil { // instances share the template's class object (decision 308)
		cls.meta = c.metaFor(cls.template)
		return cls.meta
	}
	if len(cls.TypeParams) > 0 && len(cls.singletonDefs) > 0 && cls.GoType == "" && !c.gemFile(cls.File) { // a gem's class methods see its type params as untyped (decision 312)
		c.errorf(cls.singletonDefs[0].file, cls.singletonDefs[0].node, "class methods on generic class %s are not supported", cls.RubyName)
	}
	// A class object is a Class (a module's, a Module); a subclass's class
	// object inherits from its superclass's, so class methods inherit.
	var sup *Class
	switch {
	case cls.IsModule:
		sup = c.classes["Module"]
	case cls.Super != nil && !cls.Super.universal:
		sup = c.metaFor(cls.Super)
	default:
		sup = c.classes["Class"]
	}
	m := &Class{Name: cls.metaName(), RubyName: cls.RubyName, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{},
		srcPos: srcPos{cls.File, cls.Line}, Super: sup, metaOf: cls}
	cls.meta = m
	sup.Subclasses = append(sup.Subclasses, m)
	c.classList = append(c.classList, m)
	for _, d := range cls.singletonDefs {
		c.addDef(d.file, m, d.node, d.private, d.scope)
	}
	var ofSynth []singletonMisc // `alias_method :[], :new`: after the synthesized new it names (decision 253)
	for _, d := range cls.singletonMisc {
		switch n := d.node.(type) {
		case *parser.AliasMethodNode:
			c.addAlias(d.file, m, n, n.NewName, n.OldName)
		case *parser.CallNode:
			if args := callArgs(n); n.Name == "alias_method" {
				if len(args) != 2 {
					c.errorf(d.file, n, "alias_method takes a new and an old name")
				}
				if sym, ok := args[1].(*parser.SymbolNode); ok && sym.Unescaped.Value == "new" && m.Methods["new"] == nil {
					ofSynth = append(ofSynth, d)
					continue
				}
				c.addAlias(d.file, m, n, args[0], args[1])
			} else {
				c.addAttrs(d.file, m, n, args, d.private, d.scope)
			}
		}
	}
	for _, d := range ofSynth { // applied by link once the synthesized new exists (applyAliasNews)
		c.aliasNews = append(c.aliasNews, aliasNewReq{d: d, m: m})
	}
	synth := []string{"name", "to_s", "inspect"}
	if cls.isStruct() && !cls.universal && m.Methods["new"] == nil {
		synth = append(synth, "new")
	}
	for _, name := range synth {
		if m.Methods[name] != nil || inheritsUserDef(sup, name) {
			continue
		}
		sm := &Method{Name: name, GoName: goMethodName(name), Owner: m, Kind: kindSynth, srcPos: srcPos{cls.File, cls.Line}, Private: name == "new" && cls.privateNew}
		m.Methods[name] = sm
		m.MethodList = append(m.MethodList, sm)
	}
	return m
}

// inheritsUserDef: a user-defined class-level name/to_s/inspect wins over a subclass's generated one.
func inheritsUserDef(m *Class, name string) bool {
	for ; m != nil && m.metaOf != nil; m = m.Super {
		if d := m.Methods[name]; d != nil && d.Kind != kindSynth {
			return true
		}
	}
	return false
}

// resolveSynth fills in a metaclass's generated methods. `new` takes the
// described class's initialize params and returns the hierarchy's root
// type, so that every metaclass in a hierarchy shares one Go signature and
// `singleton(Base)` can hold any subclass whose initialize matches.
func (c *Compiler) resolveSynth(m *Method) {
	cls := m.Owner.metaOf
	if m.Name != "new" {
		m.Ret = TClass{C: c.classes["String"]}
		return
	}
	m.Ret = TClass{C: cls.root()}
	init := cls.lookup("initialize")
	if init == nil {
		return
	}
	c.resolveMethod(init.M)
	env := composeEnv(init.Env, nil)
	env["Self"] = TClass{C: cls}
	for _, p := range init.M.Params {
		m.Params = append(m.Params, Param{Name: p.Name, Type: subst(p.Type, env), Default: p.Default, Rest: p.Rest, Keyword: p.Keyword, KwRest: p.KwRest, Post: p.Post, Want: subst(p.Want, env), Pending: p.Pending})
	}
	m.Block = init.M.Block // the block, its escaped self resolved during discoverIvars (decision 164)
}

// isSynthNew reports whether e is a metaclass's generated `new`.
func isSynthNew(e *entry) bool { return e != nil && e.M.Kind == kindSynth && e.M.Name == "new" }

// classVar is the Go variable holding a class object.
func classVar(cls *Class) string {
	if cls.template != nil {
		cls = cls.template
	}
	return cls.Name + "_class"
}

// descendantDefines reports whether a subclass of c defines a public name
// (or a private one, with private).
func (c *Class) descendantDefines(name string, private bool) bool {
	for _, sub := range c.Subclasses {
		if sub.metaOf != nil && c.metaOf == nil {
			continue
		}
		if m := sub.Methods[name]; m != nil && (private || !m.Private) {
			return true
		}
		for _, inc := range sub.Includes { // a module the subclass includes overrides it too (rubocop-ast's BuilderExtensions#n over Parser::Builders::Default's private n; decision 444)
			if inc.Mod == nil || inc.Mod.universal {
				continue
			}
			if e := inc.Mod.lookup(name); e != nil && e.M.Owner != nil && !e.M.Owner.universal && (private || !e.M.Private) {
				return true
			}
		}
		if sub.descendantDefines(name, private) {
			return true
		}
	}
	return false
}
