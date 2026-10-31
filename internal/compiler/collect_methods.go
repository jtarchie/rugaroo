package compiler

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// addAlias is `alias new old` / `alias_method :new, :old`: a copy of the
// method as it stands now, so redefining old later leaves new alone (as MRI).
func (c *Compiler) addAlias(f *File, cls *Class, n parser.Node, newName, oldName parser.Node) {
	name := func(x parser.Node) string {
		switch x := x.(type) {
		case *parser.SymbolNode:
			return x.Unescaped.Value
		case *parser.StringNode:
			return x.Unescaped.Value
		}
		c.errorf(f, x, "alias names must be literal symbols")
		return ""
	}
	c.addAliasNamed(f, cls, n, name(newName), name(oldName))
}

// addAliasNamed is addAlias of names already read: a literal's, or one boot's alias_method made (decision 330).
func (c *Compiler) addAliasNamed(f *File, cls *Class, n parser.Node, nn, on string) {
	old := cls.Methods[on]
	if old == nil { // an ancestor's, perhaps: supers resolve in link (linkAliases)
		c.inheritedAliases = append(c.inheritedAliases, pendingAlias{f: f, cls: cls, n: n, newName: nn, oldName: on})
		return
	}
	c.copyAlias(cls, old, nn)
}

// pendingAlias is an alias of a method cls does not define above it.
type pendingAlias struct {
	f                *File
	cls              *Class
	n                parser.Node
	newName, oldName string
}

// linkAliases copies each inherited alias's target into its class, as MRI's
// alias copies the method found from the superclass at that point: a later
// override of the old name leaves the alias on the ancestor's body.
func (c *Compiler) linkAliases(ctx context.Context) {
	for _, a := range c.inheritedAliases {
		if a.cls.isTemplate() { // each instance has its own copy (decision 309)
			continue
		}
		var e *entry
		if a.cls.Super != nil {
			e = a.cls.Super.lookup(a.oldName)
		}
		for i := len(a.cls.Includes) - 1; e == nil && i >= 0; i-- {
			if mod := a.cls.Includes[i].Mod; mod != nil {
				e = mod.lookup(a.oldName)
			}
		}
		if e == nil && a.oldName == "respond_to?" && c.gemFile(a.f) { // an intrinsic with no Method (genRespondTo): forward the call
			c.addForwarder(ctx, a, "(symbol, include_private = false) = respond_to?(symbol, include_private)")
			continue
		}
		if e == nil {
			c.errorf(a.f, a.n, "undefined method '%s' for class '%s'", a.oldName, a.cls.RubyName)
		}
		// a Hash subclass's alias of a Hash method stays Hash's (a free func on the embedded Hash, decision 165), under the new name
		fromHash := a.cls.hashBase && e.Owner == a.cls.Super && e.M.Node != nil && (e.M.Kind == kindDef || e.M.Kind == kindPrimitive)
		if !fromHash && (len(e.Owner.TypeParams) > 0 || e.M.Node == nil || e.M.Kind != kindDef || containsSuper(e.M.Node.Body)) {
			if c.gemFile(a.f) { // a forwarder instead of a copy: differs from MRI only if the old name is redefined later
				fwd, params := "*args", "*args"
				if e.M.Block != nil {
					fwd, params = "*args, &blk", "*args, &blk"
				}
				call := a.oldName + "(" + fwd + ")"
				if !isIdentMethod(a.oldName) {
					call = "self.send(:" + strconv.Quote(a.oldName) + ", " + fwd + ")"
				}
				c.addForwarder(ctx, a, "("+params+") = "+call)
				continue
			}
			c.errorf(a.f, a.n, "alias of %s#%s: rugaroo copies an inherited method only when it is a plain def without super from a non-generic class", e.Owner.RubyName, a.oldName)
		}
		m := c.copyAlias(a.cls, e.M, a.newName)
		if !fromHash {
			m.Owner = a.cls
		}
	}
	if len(c.inheritedAliases) > 0 {
		for _, cls := range c.classList {
			cls.msetCache, cls.msetIndex = nil, nil
		}
	}
}

// addForwarder defines the alias's new name as `def new<rest>`, on the alias's own line.
func (c *Compiler) addForwarder(ctx context.Context, a pendingAlias, rest string) {
	src := "def " + a.newName + rest + "\n"
	sf, def := c.synthDef(ctx, a.f, a.n, "alias "+a.newName, src, synthOpt{Origin: originSource, Pad: max(a.f.line(a.n.GetLocation().StartOffset)-1, 0)})
	c.addDef(sf, a.cls, def, false, nil)
}

func (c *Compiler) copyAlias(cls *Class, old *Method, nn string) *Method {
	m := *old
	m.Name, m.GoName = nn, goMethodName(nn)
	if prev := cls.Methods[nn]; prev != nil {
		cls.MethodList = deleteMethod(cls.MethodList, prev)
	}
	cls.MethodList = append(cls.MethodList, &m)
	cls.Methods[nn] = &m
	return &m
}

func (c *Compiler) addMethod(f *File, cls *Class, n *parser.DefNode, private bool, scope []*Class) {
	if r, ok := n.Receiver.(*parser.ConstantReadNode); ok && cls != nil && !f.isPrelude() && c.consts[qualify(scope, r.Name)] != nil {
		c.addConstSingletonDef(f, n, r, scope)
		return
	}
	if n.Receiver != nil && !f.isPrelude() {
		if _, self := n.Receiver.(*parser.SelfNode); !self {
			if target, _ := c.lookupConst(f, n.Receiver, scope); target != nil { // `def BasePrimitive.bit_aligned`: that class's method
				target.singletonDefs = append(target.singletonDefs, singletonDef{node: n, private: private, scope: scope, file: f})
				return
			}
		}
	}
	if n.Receiver != nil {
		if _, ok := n.Receiver.(*parser.SelfNode); !ok || cls == nil {
			c.errorf(f, n, "singleton methods are only supported as `def self.x` in a class or module body")
		}
		cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: n, private: private, scope: scope, file: f})
		return
	}
	c.addDef(f, cls, n, private, scope)
}

// defAnnotations applies a prelude def's `# @dynamic`, `# @binary` and `# @self` lines.
func (c *Compiler) defAnnotations(f *File, cls *Class, n *parser.DefNode, m *Method, line int) {
	ann := f.annotations(line)
	if _, ok := ann["dynamic"]; ok {
		if !f.isPrelude() && !f.valueGen {
			c.errorf(f, n, "@dynamic is only allowed in the prelude")
		}
		m.quietDynamic = true
	}
	if b, ok := ann["binary"]; ok {
		if !f.isPrelude() {
			c.errorf(f, n, "@binary is only allowed in the prelude")
		}
		m.binary = 1
		if slices.Contains(b, "self") {
			m.binary = 2
		}
		m.binBlock = slices.Contains(b, "block")
	}
	if st := ann["self"]; len(st) > 0 {
		if !f.isPrelude() || cls == nil || cls.GoType == "" {
			c.errorf(f, n, "@self is only allowed on a prelude @go_type class's methods")
		}
		m.selfText = st[0]
		cls.selfDefs = true
	}
}

func (c *Compiler) addDef(f *File, cls *Class, n *parser.DefNode, private bool, scope []*Class) {
	line := f.line(n.Location.StartOffset)
	m := &Method{Name: n.Name, GoName: goMethodName(n.Name), Owner: cls, Node: n, srcPos: srcPos{f, line}, Private: private || cls != nil && cls.metaOf == nil && rubyPrivate[n.Name], Scope: scope}
	if sig, found := f.sigComment(line); found {
		if sig == "" {
			c.errorf(f, n, "overloaded signatures (#|) are not supported")
		}
		m.sigText, m.declared = sig, true
	}
	if m.sigText == "" && cls != nil {
		m.sigText = protocolSig(n) // `def respond_to_missing?(name, priv)`: Ruby's protocol fixes its types (decision 182)
	}
	c.defAnnotations(f, cls, n, m, line)
	if m.sigText == "" && !f.isPrelude() {
		// A `.rbs` signature fills a method with no inline `#:` (decision 160);
		// overloads compile once, merged, and each call picks its arm (decision 12).
		sd := c.sigMethod(cls, n.Name)
		if sd != nil && c.gemFile(f) && !slices.ContainsFunc(sd.Overloads, func(o *rbs.MethodType) bool { return sigFitsDef(o, n.Parameters) }) {
			sd = nil // a stale shipped sig (prism 1.9.0's `Source.for: (String) -> Source` for a 3-param def): infer, as without one (decision 427)
		}
		if sd != nil && len(sd.Overloads) == 1 {
			m.sig = sd.Overloads[0]
			if sd.Advisory {
				c.inferAdvisoryParams(m, n)
			}
		} else if merged := mergeArms(sd); merged != nil {
			m.sig, m.arms = merged, sd.Overloads
		}
	}
	if body, ok := n.Body.(*parser.StatementsNode); ok && f.isPrelude() && len(body.Body) == 1 {
		if _, ok := body.Body[0].(*parser.XStringNode); ok {
			m.Kind = kindPrimitive
		}
	}
	if cls == nil {
		m.GoName = goFuncName(n.Name)
		if old := c.topDefs[n.Name]; old != nil { // as Ruby: a later def replaces an earlier one
			c.topDefList = deleteMethod(c.topDefList, old)
		}
		c.topDefs[n.Name] = m
		c.topDefList = append(c.topDefList, m)
		return
	}
	if old := cls.Methods[n.Name]; old != nil {
		// reopening: replace
		for i, x := range cls.MethodList {
			if x == old {
				cls.MethodList[i] = m
			}
		}
	} else {
		cls.MethodList = append(cls.MethodList, m)
	}
	cls.Methods[n.Name] = m
	c.addBlocklessTwin(f, cls, n, m)
}

// addBlocklessTwin gives a gem method that begins `return x unless block_given?` (prime's PseudoPrimeGenerator#each,
// `return self.dup unless block_given?`) a `__name_enum` twin whose body is x: as an iterator, block_given? is true and
// that branch is dead, so a call without a block takes the twin instead (decision 418). Not a to_enum, which decision
// 414 already answers.
func (c *Compiler) addBlocklessTwin(f *File, cls *Class, n *parser.DefNode, m *Method) {
	x := blocklessGuard(n)
	if x == nil || !c.gemFile(f) {
		return
	}
	name := "__" + overloadBase(n.Name) + "_enum"
	if cls.Methods[name] != nil {
		return
	}
	tw := *n
	tw.Name = name
	tw.Body = &parser.StatementsNode{Location: x.GetLocation(), Body: []parser.Node{x}}
	if n.Parameters != nil {
		ps := *n.Parameters
		ps.Block = nil
		tw.Parameters = &ps
	}
	t := &Method{Name: name, GoName: goMethodName(name), Owner: cls, Node: &tw, srcPos: m.srcPos, Private: m.Private, Scope: m.Scope, blocklessOf: m}
	cls.MethodList = append(cls.MethodList, t)
	cls.Methods[name] = t
	if c.blocklessTwin == nil {
		c.blocklessTwin = map[string]bool{}
	}
	c.blocklessTwin[n.Name] = true
}

// blocklessGuard is x in a def whose first statement is `return x unless block_given?`, x not to_enum/enum_for.
func blocklessGuard(n *parser.DefNode) parser.Node {
	body, ok := n.Body.(*parser.StatementsNode)
	if !ok || len(body.Body) < 2 {
		return nil
	}
	u, ok := body.Body[0].(*parser.UnlessNode)
	if !ok || u.ElseClause != nil || u.Statements == nil || len(u.Statements.Body) != 1 {
		return nil
	}
	if p, ok := u.Predicate.(*parser.CallNode); !ok || p.Name != "block_given?" || p.Receiver != nil || p.Arguments != nil {
		return nil
	}
	r, ok := u.Statements.Body[0].(*parser.ReturnNode)
	if !ok || r.Arguments == nil || len(r.Arguments.Arguments) != 1 {
		return nil
	}
	x := r.Arguments.Arguments[0]
	if c, ok := x.(*parser.CallNode); ok && (c.Name == "to_enum" || c.Name == "enum_for") {
		return nil
	}
	return x
}

// attrKind is which of attr_reader / attr_writer / attr_accessor declared an accessor.
type attrKind uint8

const (
	attrReader attrKind = iota
	attrWriter
	attrAccessor
)

// attrKindOf maps a call name; Ruby's `attr :x` is attr_reader.
func attrKindOf(name string) attrKind {
	switch name {
	case "attr", "attr_reader":
		return attrReader
	case "attr_writer":
		return attrWriter
	}
	return attrAccessor
}

func (k attrKind) reads() bool  { return k != attrWriter }
func (k attrKind) writes() bool { return k != attrReader }

func (c *Compiler) addAttrs(f *File, cls *Class, n *parser.CallNode, args []parser.Node, private bool, scope []*Class) {
	line := f.line(n.Location.StartOffset)
	if !cls.isStruct() && !cls.IsModule {
		c.errorf(f, n, "%s on a non-struct class", n.Name)
	}
	kind := attrKindOf(n.Name)
	ty, hasTy := f.trailing[line]
	var rt rbs.Type
	if hasTy {
		parsed, err := rbs.ParseType(ty)
		if err != nil {
			c.errorf(f, n, "%v", err)
		}
		rt = parsed
	}
	for _, a := range args {
		sym, ok := a.(*parser.SymbolNode)
		if !ok {
			c.errorf(f, a, "%s argument must be a symbol", n.Name)
		}
		name := sym.Unescaped.Value
		at := rt
		if !hasTy {
			// No inline `#:`: a `.rbs` attribute supplies the type (decision 160).
			if ad := c.sigAttr(cls, name); ad != nil && !f.isPrelude() {
				at = ad.Type
			} else { // the defs it stands for, inferred like any def (decision 172 for gems, 179 for user code)
				c.addModuleAttr(f, cls, n, kind, name, nil, private, scope)
				continue
			}
		}
		if kind.writes() && !kind.reads() && c.gemFile(f) {
			if _, opt := at.(rbs.Optional); !opt { // an attr_writer's ivar holds nil until it is called (SystemCommand's `@sandbox_inheritance = T.let(nil, ...)`)
				at = rbs.Optional{Elem: at}
			}
		}
		cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: "@" + name, rbs: at, line: line, scope: scope})
		if cls.IsModule {
			c.addModuleAttr(f, cls, n, kind, name, at, private, scope)
			continue
		}
		if kind.reads() {
			m := &Method{Name: name, GoName: goMethodName(name), Owner: cls, Kind: kindAttrReader, Attr: "@" + name, srcPos: srcPos{f, line}, Private: private, Scope: scope}
			m.sig = &rbs.MethodType{Return: at}
			cls.Methods[name] = m
			cls.MethodList = append(cls.MethodList, m)
		}
		if kind.writes() {
			m := &Method{Name: name + "=", GoName: goMethodName(name + "="), Owner: cls, Kind: kindAttrWriter, Attr: "@" + name, srcPos: srcPos{f, line}, Private: private, Scope: scope}
			m.sig = &rbs.MethodType{Params: []rbs.Param{{Type: at, Name: name}}, Return: rbs.Void{}}
			cls.Methods[name+"="] = m
			cls.MethodList = append(cls.MethodList, m)
		}
	}
}

// addModuleAttr declares a module's attr_* as the defs it stands for (`def x = @x`, `def x=(v) = @x = v`): a
// module's ivars live in its M_Ivars field (decision 147), not a struct an accessor method could be promoted from.
func (c *Compiler) addModuleAttr(f *File, cls *Class, n *parser.CallNode, kind attrKind, name string, at rbs.Type, private bool, scope []*Class) {
	loc, iv := n.Location, "@"+name
	def := func(dname string, params *parser.ParametersNode, locals []string, body parser.Node, sig *rbs.MethodType) {
		c.addDef(f, cls, &parser.DefNode{Location: loc, Name: dname, NameLoc: loc, DefKeywordLoc: loc, Parameters: params, Locals: locals,
			Body: &parser.StatementsNode{Location: loc, Body: []parser.Node{body}}}, private, scope)
		if sig != nil {
			m := cls.Methods[dname]
			m.sigText, m.sig = "", sig
		}
	}
	var rsig, wsig *rbs.MethodType // nil: inferred
	if at != nil {
		rsig, wsig = &rbs.MethodType{Return: at}, &rbs.MethodType{Params: []rbs.Param{{Type: at, Name: "v"}}, Return: rbs.Void{}}
	}
	if kind.reads() {
		def(name, nil, nil, &parser.InstanceVariableReadNode{Location: loc, Name: iv}, rsig)
	}
	if kind.writes() {
		// the parameter is named for the attribute: one attr_accessor's writers share a position, and a parameter's
		// inference key is position + name (decision 307)
		params := &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: name}}}
		write := &parser.InstanceVariableWriteNode{Location: loc, Name: iv, Value: &parser.LocalVariableReadNode{Location: loc, Name: name}}
		def(name+"=", params, []string{name}, write, wsig)
		cls.Methods[name+"="].attrWriter = wsig == nil
	}
}

func (c *Compiler) collectTopDef(f *File, n *parser.DefNode) {
	c.addMethod(f, nil, n, false, nil)
}

// defKeyword is a def's keyword parameter (`a:`, `b: 1`) or its `**opts` (rest).
type defKeyword struct {
	name string
	def  parser.Node // nil: required
	rest bool
}

// defKeywords lists a def's keyword parameters in order, `**opts` last.
func (c *Compiler) defKeywords(m *Method, ps *parser.ParametersNode) []defKeyword {
	if ps == nil {
		return nil
	}
	var out []defKeyword
	for _, k := range ps.Keywords {
		switch k := k.(type) {
		case *parser.RequiredKeywordParameterNode:
			out = append(out, defKeyword{name: k.Name})
		case *parser.OptionalKeywordParameterNode:
			out = append(out, defKeyword{name: k.Name, def: k.Value})
		default:
			c.errorf(m.File, k, "unsupported keyword parameter")
		}
	}
	switch r := ps.KeywordRest.(type) {
	case nil, *parser.ForwardingParameterNode: // `...` is resolveForwarding's
	case *parser.KeywordRestParameterNode:
		name := anonKwrest // `def f(**)`: g(**) reads it
		if r.Name != nil {
			name = *r.Name
		}
		out = append(out, defKeyword{name: name, rest: true})
	default:
		c.errorf(m.File, r, "unsupported keyword rest parameter")
	}
	return out
}

// defParams reads a def's parameter list: positional names, their literal
// defaults, the rest parameter, and the &block parameter's name.
func (c *Compiler) defParams(m *Method, ps *parser.ParametersNode) (names []string, defaults []parser.Node, rest string) {
	if ps == nil {
		return nil, nil, ""
	}
	for _, p := range ps.Requireds {
		rp, ok := p.(*parser.RequiredParameterNode)
		if !ok {
			c.errorf(m.File, p, "unsupported parameter form")
		}
		names = append(names, rp.Name)
		defaults = append(defaults, nil)
	}
	for _, p := range ps.Optionals {
		op, ok := p.(*parser.OptionalParameterNode)
		if !ok {
			c.errorf(m.File, p, "unsupported parameter form")
		}
		names = append(names, op.Name)
		defaults = append(defaults, op.Value)
	}
	if ps.Rest != nil {
		rp, ok := ps.Rest.(*parser.RestParameterNode)
		if !ok {
			c.errorf(m.File, ps.Rest, "unsupported rest parameter")
		}
		rest = anonRest // `def f(*)`: g(*) reads it
		if rp.Name != nil {
			rest = *rp.Name
		}
	}
	for _, p := range ps.Posts {
		rp, ok := p.(*parser.RequiredParameterNode)
		if !ok {
			c.errorf(m.File, p, "unsupported parameter form")
		}
		names = append(names, rp.Name)
		defaults = append(defaults, nil)
	}
	if ps.Block != nil {
		if m.Block == nil && m.sigText == "" && !m.File.isPrelude() { // unannotated: an untyped block, any arguments, optional (decision 183)
			m.Block = &BlockSig{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}, Optional: true, Untyped: true, Src: blockSrcAt(m.File, ps.Block.GetLocation().StartOffset)}
		}
		if m.Block == nil {
			c.errorf(m.File, ps, "a &block parameter needs a block in the signature (`#: () { (T) -> U } -> R`)")
		}
		m.BlockParam = anonBlock // `def f(&)`: g(&) forwards it
		if ps.Block.Name != nil {
			m.BlockParam = *ps.Block.Name
		}
	}
	return names, defaults, rest
}

// addConstSingletonDef is `X = K.new(...)` then `def X.m`: X's singleton class becomes a hidden subclass of K holding
// m, and X is built from it (ponytail: X.class names the subclass; give it K's name if a program prints it).
func (c *Compiler) addConstSingletonDef(f *File, n *parser.DefNode, r *parser.ConstantReadNode, scope []*Class) {
	k := c.consts[qualify(scope, r.Name)]
	call, ok := k.Value.(*parser.CallNode)
	if !ok || call.Receiver == nil {
		c.errorf(f, n, "def %s.%s: rugaroo gives a singleton method only to a constant built by `Klass.new(...)` or `Klass[...]`", r.Name, n.Name)
	}
	subName := r.Name + "__Singleton"
	sub := c.classes[qualify(scope, subName)]
	if sub == nil {
		sub = c.declareClass(f, qualify(scope, subName), f.line(n.Location.StartOffset), false)
		sub.superRef = &constRef{node: call.Receiver, scope: k.Scope, file: k.File}
		built := *call
		built.Receiver = &parser.ConstantReadNode{Name: subName, Location: call.Receiver.GetLocation()}
		k.Value = &built
	}
	d := *n
	d.Receiver = nil
	c.addDef(f, sub, &d, false, scope)
}

// isIdentMethod reports a method name callable as written (`to_a`, `empty?`), not an operator (`==`, `[]`).
func isIdentMethod(name string) bool {
	for i, r := range name {
		ok := r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) && i > 0 || (r == '?' || r == '!') && i == len(name)-1
		if !ok {
			return false
		}
	}
	return name != ""
}

// singletonAlias is `singleton_class.alias_method(:a, :b)` or `singleton_class.send(:alias_method, :a, :b)` in a class
// body, as the `alias_method :a, :b` of a `class << self` body, and the same for the attr family (liquid's
// `singleton_class.send(:attr_accessor, :cache_classes)`, decision 331); nil for anything else.
func singletonAlias(n *parser.CallNode) *parser.CallNode {
	r, ok := n.Receiver.(*parser.CallNode)
	if !ok || r.Name != "singleton_class" || r.Receiver != nil && !isSelf(r.Receiver) || r.Arguments != nil {
		return nil
	}
	args := callArgs(n)
	name := n.Name
	if (name == "send" || name == "__send__") && len(args) > 1 {
		sym, ok := args[0].(*parser.SymbolNode)
		if !ok {
			return nil
		}
		name, args = sym.Unescaped.Value, args[1:]
	}
	switch {
	case name == "alias_method" && len(args) == 2:
	case singletonAttrs[name] && len(args) > 0 && allLiteralNames(args):
	default:
		return nil
	}
	return &parser.CallNode{Location: n.Location, Name: name, Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: args}}
}

// aliasNewReq is a class's `alias_method :x, :new` waiting for its synthesized new.
type aliasNewReq struct {
	d singletonMisc
	m *Class
}

// applyAliasNews makes each queued alias of a synthesized new (decision 253).
func (c *Compiler) applyAliasNews(ctx context.Context) {
	for len(c.aliasNews) > 0 {
		r := c.aliasNews[0]
		c.aliasNews = c.aliasNews[1:]
		c.aliasNew(ctx, r.d, r.m, r.d.node.(*parser.CallNode)) //nolint:forcetypeassert // queued from a CallNode
	}
}

// aliasNew is a class's `alias_method :[], :new` on its synthesized new: a class method forwarding initialize's
// required parameters to new (`Node[value, next]`); an initialize taking any other kind of parameter keeps the plain
// alias (decision 253).
func (c *Compiler) aliasNew(ctx context.Context, d singletonMisc, m *Class, n *parser.CallNode) {
	args := callArgs(n)
	newName, ok := args[0].(*parser.SymbolNode)
	init := m.metaOf.lookup("initialize")
	var names []string
	if ok && init != nil && init.M.Node != nil {
		names = []string{}
		if ps := init.M.Node.Parameters; ps != nil {
			if len(ps.Optionals) > 0 || ps.Rest != nil || len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
				names = nil
			}
			for _, r := range ps.Requireds {
				rp, isReq := r.(*parser.RequiredParameterNode)
				if !isReq {
					names = nil
					break
				}
				names = append(names, rp.Name)
			}
		}
	}
	if names == nil {
		c.addAlias(d.file, m, n, args[0], args[1])
		return
	}
	list := strings.Join(names, ", ")
	untyped := strings.TrimSuffix(strings.Repeat("untyped, ", len(names)), ", ")
	// untyped: what callers pass goes to initialize as it is, typing its parameters from the calls
	src := "#: (" + untyped + ") -> untyped\ndef " + newName.Unescaped.Value + "(" + list + ") = new(" + list + ")\n"
	sf, err := parseFile(ctx, c.parser, d.file.Name, []byte(src), originSource)
	if err != nil || len(sf.Root.Statements.Body) != 1 {
		c.addAlias(d.file, m, n, args[0], args[1])
		return
	}
	sf.path, sf.pinLine = d.file.path, d.file.line(n.Location.StartOffset)
	c.addDef(sf, m, c.synthDefs(d.file, n, "alias new", sf)[0], false, d.scope)
}

// singletonAttrs are the attr calls singletonAlias takes onto a class's singleton.
var singletonAttrs = map[string]bool{"attr_reader": true, "attr_writer": true, "attr_accessor": true}

// sigFitsDef reports a sig arm whose positional params the def can bind: as many as the def's, or a rest on both sides.
func sigFitsDef(o *rbs.MethodType, ps *parser.ParametersNode) bool {
	n, rest := 0, false
	for _, p := range o.Params {
		switch p.Kind {
		case rbs.ParamPositional:
			n++
		case rbs.ParamRest:
			rest = true
		}
	}
	if ps == nil {
		return n == 0 && !rest
	}
	return n == len(ps.Requireds)+len(ps.Optionals)+len(ps.Posts) && rest == (ps.Rest != nil)
}
