package compiler

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// collectTopRequire: a file on the -I path loads like require_relative (decision 131); stdlib requires are meaningless
// here, unless the prelude hooks one (decision 78): `require "a/b"` → __require_a_b.
func (c *Compiler) collectTopRequire(ctx context.Context, f *File, n *parser.CallNode) {
	if uf, ok := c.userRequire(ctx, f, n); ok {
		if uf != nil {
			c.addMainStmt(f, &loadFile{call: n, file: uf})
		}
	} else if hook := c.requireHook(f, n); hook != nil {
		c.addMainStmt(f, hook)
	}
}

// collectTopPrepend is `X.prepend(M)` or `X.singleton_class.prepend(M)` (decision 199).
func (c *Compiler) collectTopPrepend(f *File, n *parser.CallNode) {
	recv, single := prependTarget(n), false
	if sc, ok := recv.(*parser.CallNode); ok {
		recv, single = sc.Receiver, true
	}
	for _, a := range callArgs(n) {
		c.prepends = append(c.prepends, pendingPrepend{targetRef: constRef{node: recv, file: f}, singleton: single, mod: constRef{node: a, file: f}})
	}
}

// collectTopCall takes the top-level calls that act at compile time: require_relative, describe, include, require.
func (c *Compiler) collectTopCall(ctx context.Context, f *File, n *parser.CallNode) {
	switch {
	case n.Receiver == nil && n.Name == "require_relative" && f.isPrelude():
		c.requireRelative(ctx, f, n)
	case !f.isPrelude() && c.loadedFeature(f, n) != "": // marked loaded: a later require of it loads nothing (decision 404)
		c.loaded[c.real(c.loadedFeature(f, n))] = true
	case n.Receiver == nil && n.Name == "require_relative":
		if uf := c.userRequireRelative(ctx, f, n); uf != nil {
			c.addMainStmt(f, &loadFile{call: n, file: uf})
		}
	case !f.isPrelude() && isDescribe(n):
		c.collectDescribe(ctx, f, nil, n)
	case c.collectIOExtend(f, n, nil): // before objectConstExtend, which would drop `ENV.extend(EnvSensitive)` (decision 248)
	case c.topConstVisibility(f, n), c.objectConstExtend(f, n):
	case n.Receiver == nil && n.Name == "include" && !f.isPrelude():
		for _, a := range callArgs(n) { // main's include is Object's, as in MRI
			c.addInclude(f, n, c.classes["Object"], a, nil)
		}
	case n.Receiver == nil && n.Name == "require":
		c.collectTopRequire(ctx, f, n)
	case n.Name == "prepend" && !f.isPrelude() && prependTarget(n) != nil: // `X.prepend(M)`, `X.singleton_class.prepend(M)` (decision 199)
		c.collectTopPrepend(f, n)
	case n.Receiver == nil && n.Name == "autoload":
		if sym, ok := symbolName(firstArg(callArgs(n))); ok {
			c.noteAutoload(sym)
		}
		c.loadAutoload(ctx, f, n, nil) // compile-time autoload (decision 161)
	default:
		c.scanAnon(ctx, f, n, nil)
		c.addMainStmt(f, n)
	}
}

// loadedFeature is the file `$LOADED_FEATURES << File.expand_path("x.rb", __dir__)` marks loaded, so a shim can stand in for it (decision 404); "" for any other statement.
func (c *Compiler) loadedFeature(f *File, n *parser.CallNode) string {
	if n.Name != "<<" && n.Name != "push" {
		return ""
	}
	if g, ok := n.Receiver.(*parser.GlobalVariableReadNode); !ok || g.Name != "$LOADED_FEATURES" && g.Name != `$"` {
		return ""
	}
	args := callArgs(n)
	if len(args) == 1 {
		if ex, ok := args[0].(*parser.CallNode); ok && ex.Name == "expand_path" && constName(ex.Receiver) == "File" {
			if a := callArgs(ex); len(a) == 2 {
				str, sok := a[0].(*parser.StringNode)
				dir, dok := a[1].(*parser.CallNode)
				if sok && dok && dir.Receiver == nil && dir.Name == "__dir__" && strings.HasSuffix(str.Unescaped.Value, ".rb") {
					return filepath.Join(filepath.Dir(f.path), filepath.FromSlash(str.Unescaped.Value))
				}
			}
		}
	}
	c.errorf(f, n, `$LOADED_FEATURES takes File.expand_path("file.rb", __dir__) only: rugaroo resolves requires at compile time (decision 404)`)
	return ""
}

// beginRequires splices the files a top-level begin requires where they stand, as rackup.rb's `begin; require_relative ...; rescue LoadError`.
func (c *Compiler) beginRequires(ctx context.Context, f *File, n *parser.BeginNode) parser.Node {
	if f.isPrelude() || n.Statements == nil {
		c.scanAnon(ctx, f, n, nil)
		return n
	}
	body := slices.Clone(n.Statements.Body)
	for i, s := range body {
		call, ok := s.(*parser.CallNode)
		if !ok || call.Receiver != nil {
			continue
		}
		var uf *File
		switch call.Name {
		case "require_relative":
			uf = c.userRequireRelative(ctx, f, call)
		case "require":
			uf, _ = c.userRequire(ctx, f, call)
		default:
			continue
		}
		body[i] = &parser.NilNode{Location: call.Location} // already loaded: false in MRI, unused here
		if uf != nil {
			body[i] = &loadFile{call: call, file: uf}
		}
	}
	b := *n
	b.Statements = &parser.StatementsNode{Location: n.Statements.Location, Body: body}
	c.scanAnon(ctx, f, &b, nil)
	return &b
}

// requireHook is the call to Kernel#__require_<lib> that stands in for a
// user file's top-level `require "lib"`, or nil when the prelude has none.
func (c *Compiler) requireHook(f *File, n *parser.CallNode) *parser.CallNode {
	if f.isPrelude() || n.Arguments == nil || len(n.Arguments.Arguments) != 1 {
		return nil
	}
	lib, ok := n.Arguments.Arguments[0].(*parser.StringNode)
	if !ok {
		return nil
	}
	name := "__require_" + strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, lib.Unescaped.Value)
	if k := c.classes["Kernel"]; k == nil || k.Methods[name] == nil {
		return nil
	}
	hook := *n
	hook.Name = name
	hook.Arguments = nil
	return &hook
}

func (c *Compiler) declareClass(f *File, name string, line int, isModule bool) *Class {
	return c.declare(f, name, line, isModule, true)
}

// declareHidden declares a class only its generated name reaches (anonymous Class.new, describe), so it stays out of `constants`.
func (c *Compiler) declareHidden(f *File, name string, line int, isModule bool) *Class {
	return c.declare(f, name, line, isModule, false)
}

func (c *Compiler) declare(f *File, name string, line int, isModule, named bool) *Class {
	cls := c.classes[name]
	if cls == nil {
		cls = &Class{Name: goClassName(name), RubyName: name, IsModule: isModule, Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, srcPos: srcPos{f, line}}
		cls.universal = name == "BasicObject" || name == "Object" || name == "Kernel"
		c.classes[name] = cls
		c.classList = append(c.classList, cls)
		if named {
			c.noteConstName(name)
		}
	} else if cls.IsModule != isModule && (name != "Numeric" || f.isPrelude()) { // MRI's class Numeric is the prelude's module (decision 142): `class Numeric` reopens it
		c.errorAt(srcPos{f, line}, "%s is already defined as a %s", name, map[bool]string{true: "module", false: "class"}[cls.IsModule])
	}
	if c.consts[name] != nil {
		c.errorAt(srcPos{f, line}, "%s is already a constant", name)
	}
	ann := f.annotations(line)
	for _, g := range ann["generic"] {
		fields := strings.Fields(g)
		if len(fields) == 0 {
			continue
		}
		// `unchecked out E < Bound` → take the identifier
		for i, fld := range fields {
			if fld == "unchecked" || fld == "in" || fld == "out" {
				continue
			}
			cls.TypeParams = append(cls.TypeParams, fld)
			if i+2 < len(fields) && fields[i+1] == "<" { // its bound, checked per instance (decision 312)
				if cls.bounds == nil {
					cls.bounds = map[string]string{}
				}
				cls.bounds[fld], _, _ = strings.Cut(fields[i+2], "[")
			}
			break
		}
	}
	if len(cls.TypeParams) == 0 && !f.isPrelude() {
		// A `.rbs` `class Foo[E]` supplies the generics an unannotated class lacks (decision 160).
		cls.TypeParams = c.sigTypeParams(cls)
		if sd := c.sigs[cls.RubyName]; sd != nil && len(cls.TypeParams) > 0 {
			cls.bounds = sd.Bounds
		}
	}
	if _, ok := ann["hidden"]; ok && f.isPrelude() {
		cls.hidden = true
	}
	if g := ann["go_type"]; len(g) > 0 {
		if isModule {
			c.errorAt(srcPos{f, line}, "@go_type on a module")
		}
		cls.GoType = g[0]
	}
	return cls
}

// readSuperArgs reads `class X < Base #[A]`, or a sig's `< Base[A]`: the superclass's type arguments (decision 309).
func (c *Compiler) readSuperArgs(f *File, n *parser.ClassNode, cls *Class, line int) {
	if t, ok := f.typeArgs[line]; ok {
		tup, err := rbs.ParseType(t)
		if err != nil {
			c.errorf(f, n, "bad superclass type args %q: %v", t, err)
		}
		tuple, isTuple := tup.(rbs.Tuple)
		if !isTuple { // `#[A] | nil` parses as a union, not the `[T, ...]` the scanner expects
			c.errorf(f, n, "bad superclass type args %q: want [T, ...]", t)
		}
		cls.superArgs = tuple.Elems
		return
	}
	if sd := c.sigs[cls.RubyName]; sd != nil {
		if sn, ok := sd.Super.(rbs.Name); ok {
			cls.superArgs = sn.Args
		}
	}
}

// goClassName maps a constant path to a Go identifier.
func goClassName(rubyName string) string { return strings.ReplaceAll(rubyName, "::", "_") }

// nameGo keeps user classes and constants off every other package-level Go
// name: runtime helpers (`Opt`) and what other classes generate (`NewUser`,
// `ShapeI`, `Foo::Bar`'s `Foo_Bar`). Prelude names are fixed, since `%x{}`
// spells them; a clashing user name gains `_` until it is free.
func (c *Compiler) nameGo() {
	taken := map[string]bool{}
	for n := 2; n <= maxTuple; n++ {
		taken[fmt.Sprintf("Tuple%d", n)] = true
	}
	for _, v := range c.verbatim {
		declaredGoNames(v.code, func(n string) { taken[n] = true }) // a package-level name declared in a top-level %x{}
	}
	claim := func(f *File, name *string, gen func() []string) { // gen reads *name
		for !f.isPrelude() && slices.ContainsFunc(gen(), func(n string) bool { return taken[n] }) {
			*name += "_"
		}
		for _, n := range gen() {
			taken[n] = true
		}
	}
	for _, cls := range c.classList {
		claim(cls.File, &cls.Name, cls.goNames)
	}
	for _, k := range c.constList {
		if k.first != nil {
			continue
		}
		claim(k.File, &k.GoName, func() []string { return []string{k.GoName} })
	}
}

// Emitters call these and goNames lists them, so a derived Go name added to one cannot be missed by nameGo's clash check.

func (cls *Class) ifaceName() string { return cls.Name + "I" }     // the interface of its objects
func (cls *Class) ctorName() string  { return "New" + cls.Name }   // the constructor
func (cls *Class) selfName() string  { return cls.Name + "_Self" } // a module's Self constraint
func (cls *Class) anyName() string   { return cls.Name + "_Any" }  // the type-switch case for the class and its subclasses
func (cls *Class) metaName() string  { return cls.Name + "_Meta" } // its metaclass's Go name

// freeFunc is the Go name of m, a method of cls, as a free function (before separateFreeFuncs).
func (cls *Class) freeFunc(m *Method) string { return cls.Name + "_" + m.GoName }

// goNames lists every package-level Go name generated for cls.
func (cls *Class) goNames() []string {
	out := make([]string, 0, 8+len(cls.MethodList)+len(cls.singletonDefs))
	out = append(out, cls.Name, cls.ifaceName(), cls.ctorName(), cls.selfName(), cls.anyName())
	if cls.template == nil { // an instance's class object is its template's
		out = append(out, classVar(cls), cls.metaName(), cls.metaName()+"I")
	}
	for _, m := range cls.MethodList {
		out = append(out, cls.freeFunc(m))
	}
	for _, d := range cls.singletonDefs {
		out = append(out, cls.metaName()+"_"+goMethodName(d.node.Name))
	}
	return out
}

// qualify names the constant `name` declared directly inside scope.
func qualify(scope []*Class, name string) string {
	if len(scope) == 0 {
		return name
	}
	return scope[len(scope)-1].RubyName + "::" + name
}

// declName resolves the name of a `class X` / `class A::X` / `module X`.
func (c *Compiler) declName(f *File, path parser.Node, scope []*Class) string {
	switch p := path.(type) {
	case *parser.ConstantReadNode:
		return qualify(scope, p.Name)
	case *parser.ConstantPathNode:
		if p.Parent == nil {
			return *p.Name
		}
		if _, ok := p.Parent.(*parser.SelfNode); ok && len(scope) > 0 { // `self::X = v` in a class body: self is that class (decision 362)
			return qualify(scope, *p.Name)
		}
		parent, _ := c.lookupConst(f, p.Parent, scope)
		if parent == nil {
			c.errorf(f, p.Parent, "unknown namespace %s", f.text(p.Parent.GetLocation()))
		}
		return parent.RubyName + "::" + *p.Name
	}
	c.errorf(f, path, "unsupported class name %s", f.text(path.GetLocation()))
	return ""
}

func (c *Compiler) collectClass(ctx context.Context, f *File, n *parser.ClassNode, scope []*Class) {
	line := f.line(n.Location.StartOffset)
	name := c.declName(f, n.ConstantPath, scope)
	reopen := c.classes[name] != nil
	cls := c.declareClass(f, name, line, false)
	if n.Superclass != nil {
		// The superclass expression is evaluated outside the class body.
		sup := n.Superclass
		if call, ok := sup.(*parser.CallNode); ok && call.Name == "DelegateClass" {
			sup = c.delegateClassSuper(ctx, f, call, scope)
		}
		if call, kind := valueClass(sup); call != nil && !reopen { // `class X < Struct.new(:a)`: a hidden value class as the super, as MRI's anonymous one
			anon := "Struct_" + strings.ReplaceAll(name, "::", "_")
			c.collectValueClassAs(ctx, f, &parser.ConstantWriteNode{Name: anon, Value: call, Location: call.Location, NameLoc: call.Location}, call, kind, scope, true)
			sup = &parser.ConstantReadNode{Name: anon, Location: call.Location}
		}
		if _, self := sup.(*parser.SelfNode); self && len(scope) > 0 { // `class NoArgument < self` in Switch's body: the enclosing class (decision 244)
			sup = absConstPath(scope[len(scope)-1].RubyName, sup.GetLocation())
		}
		ref := &constRef{node: sup, scope: scope, file: f}
		if !reopen {
			c.readSuperArgs(f, n, cls, line)
		}
		if reopen {
			// the first declaration fixed the superclass (Object when it named none)
			cls.reSupers = append(cls.reSupers, ref)
		} else {
			cls.superRef = ref
		}
	}
	if !reopen && !f.isPrelude() {
		c.hooks = append(c.hooks, classHook{name: "inherited", cls: cls, node: n, file: f})
	}
	c.collectBody(ctx, f, cls, n.Body, append(append([]*Class(nil), scope...), cls), true)
	c.addInstanceDelegators(ctx, cls, append(append([]*Class(nil), scope...), cls))
	if props, ok := f.sorbetStructs[n]; ok && cls.Methods["initialize"] == nil {
		c.addStructInit(ctx, f, n, cls, props, append(append([]*Class(nil), scope...), cls))
	}
	c.addHashDefaultIdx(ctx, f, n, cls, append(append([]*Class(nil), scope...), cls))
}

// addInstanceDelegators makes each public instance method a delegator hook saw defined (after it, in its file) a
// class method forwarding to the hook's target, `def self.each(...) = instance.each(...)`, as the hook's
// def_delegator would at load time (prime's Prime.each, decision 410). A class method of the name already wins.
func (c *Compiler) addInstanceDelegators(ctx context.Context, cls *Class, scope []*Class) {
	h := cls.instanceDelegator
	if h == nil {
		return
	}
	var src strings.Builder
	for _, m := range cls.MethodList {
		if m.Kind != kindDef || m.Private || m.Node == nil || m.File != h.file || m.Node.Location.StartOffset < h.offset || !isIdentMethod(m.Name) ||
			slices.ContainsFunc(cls.singletonDefs, func(d singletonDef) bool { return d.node.Name == m.Name }) {
			continue
		}
		if def, ok := delegatorDef(m, h.target); ok {
			src.WriteString(def)
		}
	}
	if src.Len() == 0 {
		return
	}
	const label = "instance delegators"
	sf := c.synthFile(ctx, h.file, h.at, label, src.String(), synthOpt{Origin: originSource, Pin: true})
	for _, d := range c.synthDefs(h.file, h.at, label, sf) {
		c.addMethod(sf, cls, d, false, scope)
	}
}

// delegatorDef is `def self.name(<m's parameters>) = target.name(<each passed on>)`: m's own parameter list, defaults
// and all, so the gem's signature for the class method types it (prime's sig/), and a block passed on whether or not m
// names one. False for a parameter list it cannot spell (a destructuring parameter).
func delegatorDef(m *Method, target string) (string, bool) {
	params, args, ok := delegatorParams(m)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("def self.%s(%s) = %s.%s(%s)\n", m.Name, strings.Join(params, ", "), target, m.Name, strings.Join(args, ", ")), true
}

// delegatorParams is m's parameter list as written and the arguments that pass each on, a block last whether or not m
// names one; false for one it cannot spell (a destructuring parameter).
func delegatorParams(m *Method) (params, args []string, ok bool) {
	block := "&__blk"
	defer func() {
		if ok {
			params, args = append(params, block), append(args, block)
		}
	}()
	ps := m.Node.Parameters
	if ps == nil {
		return nil, nil, true
	}
	if ps.Block != nil && ps.Block.Name != nil {
		block = "&" + *ps.Block.Name
	}
	for _, n := range slices.Concat(ps.Requireds, ps.Optionals) {
		switch p := n.(type) {
		case *parser.RequiredParameterNode:
			params, args = append(params, p.Name), append(args, p.Name)
		case *parser.OptionalParameterNode:
			params, args = append(params, p.Name+" = "+m.File.text(p.Value.GetLocation())), append(args, p.Name)
		default:
			return nil, nil, false
		}
	}
	rest, ok := spreadParam(ps.Rest, "*", "__rest")
	if !ok {
		return nil, nil, false
	}
	params, args = append(params, rest...), append(args, rest...)
	for _, n := range ps.Posts {
		p, isReq := n.(*parser.RequiredParameterNode)
		if !isReq {
			return nil, nil, false
		}
		params, args = append(params, p.Name), append(args, p.Name)
	}
	for _, n := range ps.Keywords {
		switch p := n.(type) {
		case *parser.RequiredKeywordParameterNode:
			params, args = append(params, p.Name+":"), append(args, p.Name+": "+p.Name)
		case *parser.OptionalKeywordParameterNode:
			params, args = append(params, p.Name+": "+m.File.text(p.Value.GetLocation())), append(args, p.Name+": "+p.Name)
		}
	}
	kwrest, ok := spreadParam(ps.KeywordRest, "**", "__kw")
	if !ok {
		return nil, nil, false
	}
	return append(params, kwrest...), append(args, kwrest...), true
}

// spreadParam is a `*rest` or `**opts` parameter as both its declaration and its pass-on (one string, used twice), named
// fallback when anonymous; empty for none, false for a form it cannot spell (`**nil`).
func spreadParam(n parser.Node, sigil, fallback string) ([]string, bool) {
	var name *string
	switch p := n.(type) {
	case nil:
		return nil, true
	case *parser.RestParameterNode:
		name = p.Name
	case *parser.KeywordRestParameterNode:
		name = p.Name
	default:
		return nil, false
	}
	if name == nil {
		return []string{sigil + fallback}, true
	}
	return []string{sigil + *name}, true
}

// addHashDefaultIdx gives a Hash subclass that overrides `default(key)` but not `[]` a `[]` that calls it for a
// missing key, as MRI's rb_hash_aref does through the method table. Hash#[] itself reads the stored default, so the
// override would otherwise never be seen: plain Hash pays nothing for it.
func (c *Compiler) addHashDefaultIdx(ctx context.Context, f *File, n *parser.ClassNode, cls *Class, scope []*Class) {
	sup, ok := n.Superclass.(*parser.ConstantReadNode)
	if !ok || sup.Name != "Hash" || f.isPrelude() || cls.Methods["[]"] != nil {
		return
	}
	d := cls.Methods["default"]
	if d == nil || d.Owner != cls || d.Node == nil || d.Node.Parameters == nil || len(d.Node.Parameters.Requireds) != 1 {
		return
	}
	sf, def := c.synthDef(ctx, f, n, "Hash subclass []", "#: (untyped) -> untyped\ndef [](key) = key?(key) ? super : default(key)\n", synthOpt{Origin: originSource, Pin: true})
	// ponytail: untyped, since the override's answer need not be a V; type it from default's signature when one names V
	c.addDef(sf, cls, def, false, scope)
}

// bootStructProps are the props boot saw cls's T::Struct end up with that its source does not name literally: a load-time
// loop's `const name, T` (decision 241). Their readers come with them.
func (c *Compiler) bootStructProps(ctx context.Context, f *File, cls *Class, props []structProp) ([]structProp, string) {
	if c.bootStructs == nil {
		return nil, ""
	}
	var extra []structProp
	var readers strings.Builder
	for _, bp := range c.bootStructs[cls.RubyName] {
		if slices.ContainsFunc(props, func(p structProp) bool { return p.name == bp.Name }) {
			continue
		}
		tf, err := parseFile(ctx, c.parser, f.Name, []byte(bp.Type), originSource)
		if err != nil || len(tf.Root.Statements.Body) != 1 {
			continue
		}
		p := structProp{name: bp.Name, rbs: sorbetType(tf.Root.Statements.Body[0])}
		p.optional = strings.HasSuffix(p.rbs, "?") || p.rbs == "untyped"
		if p.optional {
			p.def = "nil"
		}
		if bp.Default != "" {
			p.def, p.optional = bp.Default, true
		}
		extra = append(extra, p)
		readers.WriteString("#: () -> " + p.rbs + "\ndef " + p.name + " = @" + p.name + "\n")
	}
	return extra, readers.String()
}

// computedStructProp reports a `const name, T` / `prop name, T` whose name is not a literal Symbol: boot's props cover it.
func computedStructProp(n parser.Node) bool {
	found := false
	anyNode(n, func(x parser.Node) bool {
		if c, ok := x.(*parser.CallNode); ok && c.Receiver == nil && (c.Name == "const" || c.Name == "prop") {
			if args := callArgs(c); len(args) >= 2 {
				if _, sym := args[0].(*parser.SymbolNode); !sym {
					found = true
				}
			}
		}
		return found
	})
	return found
}

// addStructInit is the keyword initialize a T::Struct's props generate (decision 196).
func (c *Compiler) addStructInit(ctx context.Context, f *File, n *parser.ClassNode, cls *Class, props []structProp, scope []*Class) {
	extra, readers := c.bootStructProps(ctx, f, cls, props)
	props = append(slices.Clone(props), extra...)
	sig, params, body := make([]string, 0, len(props)), make([]string, 0, len(props)), make([]string, 0, len(props))
	for _, p := range props {
		opt := ""
		param := p.name + ":"
		if p.optional {
			opt, param = "?", p.name+": "+p.def
		}
		sig = append(sig, opt+p.name+": "+p.rbs)
		params = append(params, param)
		body = append(body, "@"+p.name+" = "+p.name)
	}
	syms := make([]string, len(props))
	for i, p := range props {
		syms[i] = ":" + p.name
	}
	// sorbet-runtime's `decorator.all_props` (Homebrew's FormulaStruct.from_hash slices by it), the props known here
	// (decision 249). ponytail: the class itself answers all_props, which MRI's does not; a Decorator class to fix.
	decorator := "#: () -> singleton(" + cls.RubyName + ")\ndef self.decorator = self\n#: () -> Array[Symbol]\ndef self.all_props = [" + strings.Join(syms, ", ") + "]\n"
	src := "#: (" + strings.Join(sig, ", ") + ") -> void\ndef initialize(" + strings.Join(params, ", ") + ")\n" + strings.Join(body, "\n") + "\nend\n" + readers + decorator
	const label = "T::Struct initialize"
	sf := c.synthFile(ctx, f, n, label, src, synthOpt{Origin: originSource, Pin: true})
	for _, d := range c.synthDefs(f, n, label, sf) {
		if d.Receiver != nil { // `def self.decorator`: a class method, as a class body's
			c.addMethod(sf, cls, d, false, scope)
			continue
		}
		c.addDef(sf, cls, d, false, scope)
	}
}

func (c *Compiler) collectModule(ctx context.Context, f *File, n *parser.ModuleNode, scope []*Class) {
	name := c.declName(f, n.ConstantPath, scope)
	cls := c.declareClass(f, name, f.line(n.Location.StartOffset), true)
	inner := append(append([]*Class(nil), scope...), cls)
	c.includedExtends(f, cls, n.Body, inner)
	c.collectBody(ctx, f, cls, n.Body, inner, true)
}

// collectSingletonClass is `class << self`: its defs are class methods, as `def self.x` (a bare private there makes them private).
// delegatorHook is the target of `def method_added(m) = (class << self; self; end).def_delegator(:instance, m)` (or
// `singleton_class.def_delegator`), a hook that forwards each instance method defined after it from the class: "" when
// d is not one.
func delegatorHook(d *parser.DefNode) string {
	if d.Name != "method_added" || d.Parameters == nil || len(d.Parameters.Requireds) != 1 {
		return ""
	}
	param, ok := d.Parameters.Requireds[0].(*parser.RequiredParameterNode)
	if !ok {
		return ""
	}
	var call *parser.CallNode
	switch b := d.Body.(type) {
	case *parser.StatementsNode:
		if len(b.Body) == 1 {
			call, _ = b.Body[0].(*parser.CallNode)
		}
	case *parser.CallNode:
		call = b
	}
	if call == nil || call.Name != "def_delegator" || !singletonSelf(call.Receiver) {
		return ""
	}
	args := callArgs(call)
	if len(args) != 2 {
		return ""
	}
	target, ok := symbolName(args[0])
	if r, isLocal := args[1].(*parser.LocalVariableReadNode); !ok || !isLocal || r.Name != param.Name {
		return ""
	}
	return target
}

// singletonSelf is `(class << self; self; end)` or `singleton_class`: the class's singleton class.
func singletonSelf(n parser.Node) bool {
	if p, ok := n.(*parser.ParenthesesNode); ok {
		if st, ok := p.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			n = st.Body[0]
		}
	}
	switch x := n.(type) {
	case *parser.CallNode:
		return x.Name == "singleton_class" && (x.Receiver == nil || isSelf(x.Receiver)) && x.Arguments == nil
	case *parser.SingletonClassNode:
		_, self := x.Expression.(*parser.SelfNode)
		st, ok := x.Body.(*parser.StatementsNode)
		if !self || !ok || len(st.Body) != 1 {
			return false
		}
		_, ret := st.Body[0].(*parser.SelfNode)
		return ret
	}
	return false
}

func (c *Compiler) collectSingletonClass(f *File, cls *Class, n *parser.SingletonClassNode, scope []*Class) {
	if _, ok := n.Expression.(*parser.SelfNode); !ok || cls == nil {
		target, _ := c.lookupConst(f, n.Expression, scope) // `class << Section`: that class's singleton, from wherever it is opened
		if target == nil || cls == nil {
			c.errorf(f, n, "only `class << self` or `class << Const` in a class or module body is supported")
		}
		cls = target
	}
	st, _ := n.Body.(*parser.StatementsNode)
	if st == nil {
		return
	}
	private := false
	for _, s := range st.Body {
		switch s := s.(type) {
		case *parser.DefNode:
			if s.Receiver != nil {
				c.errorf(f, s, "a def with a receiver inside `class << self` is not supported")
			}
			if target := delegatorHook(s); target != "" && c.gemFile(f) { // its methods are made here, at compile time (decision 410)
				cls.instanceDelegator = &instanceDelegator{target: target, file: f, offset: s.Location.StartOffset, at: s}
				continue
			}
			cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: s, private: private, scope: scope, file: f})
		case *parser.CallNode:
			private = c.collectSingletonCall(f, cls, s, private, scope)
		case *parser.AliasMethodNode:
			cls.singletonMisc = append(cls.singletonMisc, singletonMisc{node: s, scope: scope, file: f})
		case *parser.ConstantWriteNode: // ponytail: the owner's constant, not the singleton class's; lexical lookup from its defs is the same
			c.addConst(f, s, s.Name, scope)
		default:
			c.errorf(f, s, "unsupported node in `class << self`: %s", nodeType(s))
		}
	}
}

// isClassEvalCall reports whether n is a class-body `class_eval`/`module_eval`
// (or the *_exec forms), whose block is collected as class-body statements
// rather than dispatched or scanned here.
func isClassEvalCall(n *parser.CallNode) bool {
	return n.Receiver == nil && (n.Name == "class_eval" || n.Name == "module_eval" || n.Name == "class_exec" || n.Name == "module_exec")
}

// collectClassEval is `class_eval do ... end` in a class body: the block's
// body is collected as class-body statements in the same class and lexical
// scope, so `def` inside defines a method (decision 164). A String argument
// would be a real eval, which the boot snapshot handles at compile time
// (decision 161); block parameters belong to the *_exec forms, not supported.
func (c *Compiler) collectClassEval(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class, loadTime bool) {
	if len(callArgs(n)) > 0 && c.evalSites[f.bootLoc(n.Location.StartOffset)] {
		return // boot ran it: its source is already this class's body (decision 204; racc's `class_eval <<~RUBY`)
	}
	if len(callArgs(n)) > 0 {
		c.errorf(f, n, "%s with an argument is not supported (a String to eval is not)", n.Name)
	}
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok {
		c.errorf(f, n, "%s needs a literal block", n.Name)
	}
	if blk.Parameters != nil {
		c.errorf(f, blk, "%s needs a block without parameters", n.Name)
	}
	c.collectBody(ctx, f, cls, blk.Body, scope, loadTime)
}

// addSingletonInstance gives a class that includes Singleton its
// `instance`, memoized in a class-level ivar (decision 54).
func (c *Compiler) addSingletonInstance(ctx context.Context, f *File, n *parser.CallNode, cls *Class, scope []*Class) {
	src := fmt.Sprintf("#: () -> %s\ndef self.instance = @__singleton_instance ||= new\n", cls.RubyName)
	sf, err := parseFile(ctx, c.parser, f.Name, []byte(src), f.synthOrigin())
	if err != nil {
		c.errorf(f, n, "%v", err)
	}
	c.addMethod(sf, cls, sf.Root.Statements.Body[0].(*parser.DefNode), false, scope)
}

// addClassStmt records a class/module body statement that runs at load time,
// where the class is defined (decision 166).
func (c *Compiler) addClassStmt(f *File, n parser.Node, scope []*Class) {
	if c.gemFile(f) && computedStructProp(n) && len(scope) > 0 && c.bootStructs[scope[len(scope)-1].RubyName] != nil {
		return // a load-time loop declaring T::Struct props: boot's props and define_methods are what it made (decision 241)
	}
	if c.gemFile(f) && undefLoop(n) {
		// ponytail: a gem's blank slate (`(instance_methods - ...).each { |m| undef_method m }`, Homebrew's InstallSteps
		// sandbox) keeps its methods, so only a misused DSL behaves differently; the boot snapshot's method list would undo it
		return
	}
	if g := c.bodyGroup; g != nil { // a body with locals runs as one unit (decision 170)
		st := g.Node.(*parser.StatementsNode)
		if len(st.Body) == 0 {
			st.Location = n.GetLocation()
			c.classStmts = append(c.classStmts, g)
		}
		st.Body = append(st.Body, n)
		return
	}
	c.classStmts = append(c.classStmts, &classStmt{Node: n, file: f, scope: scope})
}

// addMainStmt records a top-level statement and the file it runs in.
func (c *Compiler) addMainStmt(f *File, n parser.Node) {
	if c.stmtFile == nil {
		c.stmtFile = map[parser.Node]*File{}
	}
	c.stmtFile[n] = f
	c.mainStmts = append(c.mainStmts, n)
}

// noteAutoloadCall records `autoload :X, path`'s constant: it answers defined? before it loads, as in MRI.
func (c *Compiler) noteAutoloadCall(n *parser.CallNode, args []parser.Node, scope []*Class) {
	if sym, ok := symbolName(firstArg(args)); ok && n.Name == "autoload" {
		c.noteAutoload(qualify(scope, sym))
	}
}

// keepGoing runs fn; under RUGAROO_KEEP_GOING=1 a compile error is recorded and collection goes on, listing at once
// every construct a large program needs (a debugging aid, like RUGAROO_NO_PRUNE).
func (c *Compiler) keepGoing(fn func()) {
	if !c.env.KeepGoing || c.anonDepth > 0 { // an anonymous class's error already waits for its literal
		fn()
		return
	}
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			c.kept = append(c.kept, ce.Error())
		}
	}()
	fn()
}

// deferClassError records why cls cannot compile; codegen raises it only where reachable code names the class.
func (c *Compiler) deferClassError(cls *Class, msg string) {
	if c.badClasses == nil {
		c.badClasses = map[*Class]compileError{}
	}
	c.badClasses[cls] = compileError{pos: cls.srcPos, msg: msg}
}

// runsStatements reports a class body whose run-time statements are kept: a load-time body, or a per-run Class.new
// block's (decision 259).
func (c *Compiler) runsStatements(loadTime bool) bool { return loadTime || c.anonRun != nil }
