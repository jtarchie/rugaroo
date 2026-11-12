package compiler

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// No run-time define_method: boot's methods become defs with captures baked in, and the call is a no-op (decision 198).

// dmSite is one define_method call in a compiled file, with the lexical scope it stands in.
type dmSite struct {
	file  *File
	call  *parser.CallNode
	block *parser.BlockNode
	scope []*Class
}

// collectDefineMethods declares the boot-recorded define_method methods whose site is in the closed world.
func (c *Compiler) collectDefineMethods(ctx context.Context, entries []boot.MethodInfo) {
	if len(entries) == 0 {
		return
	}
	sites := map[boot.Loc]*dmSite{}
	for _, uf := range c.userFiles {
		c.indexDefineMethods(uf, uf.Root, nil, sites)
	}
	for _, e := range entries {
		c.keepGoing(func() { c.collectDefineMethod(ctx, sites, e) })
	}
}

func (c *Compiler) collectDefineMethod(ctx context.Context, sites map[boot.Loc]*dmSite, e boot.MethodInfo) {
	{
		site := sites[boot.Loc{File: realPath(e.File), Line: e.Line}]
		owner := c.classes[e.Owner]
		if site == nil || owner == nil {
			return // a site outside the closed world (sorbet-runtime's own), or an anonymous owner
		}
		src := c.defineMethodSource(site, e)
		sf, def := c.synthDef(ctx, site.file, site.call, "define_method "+e.Name, src, synthOpt{Origin: originSource, Pin: true})
		def.Name = e.Name
		if c.dmDefined == nil {
			c.dmDefined = map[*parser.CallNode]bool{}
		}
		c.dmDefined[site.call] = true
		switch {
		case e.Singleton:
			owner.singletonDefs = append(owner.singletonDefs, singletonDef{node: def, scope: site.scope, file: sf})
		case e.ModuleFunction:
			c.addDef(sf, owner, def, true, site.scope)
			owner.singletonDefs = append(owner.singletonDefs, singletonDef{node: def, scope: site.scope, file: sf})
		default:
			c.addDef(sf, owner, def, e.Private, site.scope)
		}
	}
}

// indexDefineMethods records each define_method call with a literal block by file:line, with its lexical scope.
func (c *Compiler) indexDefineMethods(f *File, n parser.Node, scope []*Class, out map[boot.Loc]*dmSite) {
	switch n := n.(type) {
	case nil:
		return
	case *parser.ClassNode:
		if cls := c.classes[c.declName(f, n.ConstantPath, scope)]; cls != nil {
			scope = append(slices.Clip(scope), cls)
		}
	case *parser.ModuleNode:
		if cls := c.classes[c.declName(f, n.ConstantPath, scope)]; cls != nil {
			scope = append(slices.Clip(scope), cls)
		}
	case *parser.CallNode:
		if blk, ok := n.Block.(*parser.BlockNode); ok && isDefineMethodCall(n) {
			loc := n.Location
			if n.MessageLoc != nil {
				loc = *n.MessageLoc
			}
			out[f.bootLoc(loc.StartOffset)] = &dmSite{file: f, call: n, block: blk, scope: scope}
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.indexDefineMethods(f, ch, scope, out)
	}
}

// isDefineMethodCall is `define_method(...)`, `define_singleton_method(...)`, or either through `send`
// (kramdown's `singleton_class.send(:define_method, name) do |data| ... end`).
func isDefineMethodCall(n *parser.CallNode) bool {
	switch n.Name {
	case "define_method", "define_singleton_method":
		return true
	case "send", "__send__", "public_send":
		if args := callArgs(n); len(args) > 0 {
			if sym, ok := args[0].(*parser.SymbolNode); ok {
				return sym.Unescaped.Value == "define_method" || sym.Unescaped.Value == "define_singleton_method"
			}
		}
	}
	return false
}

// defineMethodSource rebuilds the block as a def; captured locals become literal assignments so each method is self-contained.
func (c *Compiler) defineMethodSource(site *dmSite, e boot.MethodInfo) string {
	f, blk := site.file, site.block
	var b strings.Builder
	b.WriteString("def ")
	if e.Singleton {
		b.WriteString("self.")
	}
	b.WriteString("__rugaroo_dm") // the real name may not be an identifier (`gcc-11`); collectDefineMethod sets it on the node
	if bp, ok := blk.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
		b.WriteString("(" + f.text(bp.Parameters.Location) + ")")
	}
	b.WriteString("\n")
	for _, name := range c.dmCaptures(site) {
		lit, ok := e.Captures[name]
		if !ok {
			c.errorf(f, site.call, "define_method %s captures %s, which boot could not record as plain data or a constant's element (decision 198)", e.Name, name)
		}
		if loc, ok := strings.CutPrefix(lit, "proc@"); ok {
			lit = c.procLiteral(f, site.call, loc)
		}
		b.WriteString(name + " = " + lit + "\n")
	}
	if blk.Body != nil {
		b.WriteString(blockAsMethodBody(f, blk))
	}
	b.WriteString("\nend\n")
	return b.String()
}

// dmCaptures finds outer locals the block reads; a write is an error since baked-in copies can't share state.
func (c *Compiler) dmCaptures(site *dmSite) []string {
	var out []string
	var walk func(n parser.Node, nest uint32)
	walk = func(n parser.Node, nest uint32) {
		switch n := n.(type) {
		case nil:
			return
		case *parser.LocalVariableReadNode:
			if n.Depth > nest && !slices.Contains(out, n.Name) {
				out = append(out, n.Name)
			}
		case *parser.LocalVariableWriteNode:
			if n.Depth > nest {
				c.errorf(site.file, n, "define_method block assigns the captured local %s (decision 198)", n.Name)
			}
		case *parser.LocalVariableOperatorWriteNode:
			if n.Depth > nest {
				c.errorf(site.file, n, "define_method block assigns the captured local %s (decision 198)", n.Name)
			}
		case *parser.BlockNode, *parser.LambdaNode:
			for _, ch := range n.CompactChildNodes() {
				walk(ch, nest+1)
			}
			return
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch, nest)
		}
	}
	walk(site.block.Body, 0)
	return out
}

// blockAsMethodBody is body's text with each `next` that leaves the block itself (not a nested one) as `return`.
func blockAsMethodBody(f *File, blk *parser.BlockNode) string {
	body := blk.Body
	loc := body.GetLocation()
	if bn, ok := body.(*parser.BeginNode); ok { // a block with rescue: its implicit begin starts at the block's `do`
		from := bn.Location.StartOffset
		switch {
		case bn.Statements != nil:
			from = bn.Statements.Location.StartOffset
		case bn.RescueClause != nil:
			from = bn.RescueClause.Location.StartOffset
		}
		loc = parser.Location{StartOffset: from, Length: blk.ClosingLoc.StartOffset - from}
	}
	start := loc.StartOffset
	text := []byte(f.text(loc))
	var nexts []int
	var walk func(n parser.Node)
	walk = func(n parser.Node) {
		switch n := n.(type) {
		case nil, *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
			return
		case *parser.NextNode:
			nexts = append(nexts, n.KeywordLoc.StartOffset-start)
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch)
		}
	}
	walk(body)
	var b strings.Builder
	prev := 0
	for _, at := range nexts {
		b.Write(text[prev:at])
		b.WriteString("return")
		prev = at + len("next")
	}
	b.Write(text[prev:])
	return b.String()
}

// markDefineMethods marks the define_method calls boot recorded before collect, so a class body can drop a statement
// that only makes methods.
func (c *Compiler) markDefineMethods(entries []boot.MethodInfo) {
	keys := map[boot.Loc]bool{}
	for _, e := range entries {
		keys[boot.Loc{File: realPath(e.File), Line: e.Line}] = true
	}
	c.dmSites = map[*parser.CallNode]bool{}
	for _, f := range c.parsed {
		anyNode(f.Root, func(n parser.Node) bool {
			if call, ok := n.(*parser.CallNode); ok && isDefineMethodCall(call) {
				loc := call.Location
				if call.MessageLoc != nil {
					loc = *call.MessageLoc
				}
				if keys[f.bootLoc(loc.StartOffset)] {
					c.dmSites[call] = true
				}
			}
			return false
		})
	}
}

// onlyDefinesMethods reports whether n's calls into define_method are all ones boot recorded.
func (c *Compiler) onlyDefinesMethods(n parser.Node) bool {
	found := false
	anyNode(n, func(x parser.Node) bool {
		if call, ok := x.(*parser.CallNode); ok && c.dmSites[call] {
			found = true
		}
		return false
	})
	return found
}

// procLiteral is `proc <block>` for the literal block boot saw a captured Proc made from, at "path:line".
func (c *Compiler) procLiteral(f *File, at parser.Node, loc string) string {
	src, ln := c.callerFile(loc)
	var found *parser.BlockNode
	if src != nil {
		anyNode(src.Root, func(n parser.Node) bool {
			if b, ok := n.(*parser.BlockNode); ok && src.line(b.Location.StartOffset) == ln {
				found = b
				return true
			}
			return false
		})
	}
	if found == nil {
		c.errorf(f, at, "define_method captures a Proc from %s, which is not compiled (decision 198)", loc)
	}
	return "proc " + src.text(found.Location)
}

// collectSingletonTemplates takes a gem method's `@x.define_singleton_method(name) do ... end`, whose block reads only
// the name it is given (Homebrew's CLI::Parser#set_args_method: `instance_variable_get(:@table).fetch(name)` per parsed
// flag), as a template over that name: @x's class (its `#:`/T.let type) gets `method_missing(name, *)` with the block's
// body, so `args.sizes?` is that body at :sizes?, and the run-time call is a no-op (decision 230). No method is made
// at run time; what MRI would make for any name is compiled once.
func (c *Compiler) collectSingletonTemplates(ctx context.Context) {
	var sites []selfSingletonSite
	for _, cls := range slices.Clone(c.classList) {
		for _, m := range slices.Clone(cls.MethodList) {
			if m.Node == nil || m.File == nil || !c.gemFile(m.File) {
				continue
			}
			c.keepGoing(func() { c.singletonTemplatesIn(ctx, cls, m, m.Node.Body, &sites) })
		}
		for _, d := range cls.singletonDefs { // class methods (bindata's DSLBigAndLittleEndianHandler): literal names on a receiver only
			if d.file == nil || !c.gemFile(d.file) {
				continue
			}
			m := &Method{Name: d.node.Name, Node: d.node, srcPos: srcPos{d.file, d.file.line(d.node.Location.StartOffset)}, Scope: d.scope}
			var found []selfSingletonSite
			c.keepGoing(func() { c.singletonTemplatesIn(ctx, cls, m, d.node.Body, &found) })
			for _, s := range found {
				if s.name != "" && s.call.Receiver != nil {
					sites = append(sites, s)
				}
			}
		}
	}
	if len(sites) == 0 {
		return
	}
	byClass := map[*Class][]int{}
	var order []*Class
	add := func(k *Class, i int) {
		if byClass[k] == nil {
			order = append(order, k)
		}
		byClass[k] = append(byClass[k], i)
	}
	for i, s := range sites {
		if s.name == "" {
			add(s.cls, i)
			continue
		}
		for _, k := range c.classList { // each class whose own def the singleton method would override
			if om := k.Methods[s.name]; om != nil && om.Owner == k && om.Node != nil && om.Node.Receiver == nil && om.File != nil && c.gemFile(om.File) {
				add(k, i)
			}
		}
	}
	for _, k := range order {
		c.keepGoing(func() { c.emitSelfSingletons(ctx, k, sites, byClass[k]) })
	}
}

// selfSingletonSite is a gem method's `define_singleton_method(name) do |params| ... end` run on objects at run time:
// receiverless with a computed name (bindata's Struct#define_field_accessors_for(name, index), once per field), or on
// any receiver with a literal name (bindata's `parser.define_singleton_method(:parse_and_append_field)`).
type selfSingletonSite struct {
	cls  *Class
	m    *Method
	call *parser.CallNode
	blk  *parser.BlockNode
	caps []string
	name string // the literal name, or "" for a computed one
}

// selfSingleton matches a selfSingletonSite: one name argument and a literal block.
func (c *Compiler) selfSingleton(cls *Class, m *Method, n *parser.CallNode) (selfSingletonSite, bool) {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok || n.Name != "define_singleton_method" || len(callArgs(n)) != 1 || blk.Body == nil {
		return selfSingletonSite{}, false
	}
	name := ""
	if sym, ok := callArgs(n)[0].(*parser.SymbolNode); ok {
		name = sym.Unescaped.Value
	} else if n.Receiver != nil || c.dmDefined[n] {
		return selfSingletonSite{}, false
	}
	if c.dmDefined[n] {
		return selfSingletonSite{}, false // on a class object: boot made it a def (decision 198)
	}
	if bp, ok := blk.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
		if ps := bp.Parameters; len(ps.Keywords) > 0 || ps.KeywordRest != nil || len(ps.Posts) > 0 || name == "" && ps.Block != nil {
			return selfSingletonSite{}, false
		}
	} else if blk.Parameters != nil {
		return selfSingletonSite{}, false
	}
	site := &dmSite{file: m.File, call: n, block: blk, scope: m.Scope}
	return selfSingletonSite{cls: cls, m: m, call: n, blk: blk, caps: c.dmCaptures(site), name: name}, true
}

// emitSelfSingletons compiles each site's block once, as a method taking its captures then its parameters; the
// run-time call records (site, name, captures) on the object (a new Array each time: a clone shares the old one).
// A computed name is found by method_missing/respond_to_missing?; a literal one by a check at the top of the method it
// overrides (decision 324). No method is made at run time.
func (c *Compiler) emitSelfSingletons(ctx context.Context, cls *Class, sites []selfSingletonSite, ids []int) {
	if c.smSites == nil {
		c.smSites = map[*parser.CallNode]*parser.CallNode{}
	}
	var b, dispatch strings.Builder
	computed := false
	overrides := map[string][]int{}
	for _, i := range ids {
		s := sites[i]
		if s.name == "" {
			if mm := cls.lookup("method_missing"); mm != nil && mm.Owner == cls {
				continue
			}
			computed = true
		} else {
			overrides[s.name] = append(overrides[s.name], i)
		}
		params := slices.Clone(s.caps)
		if bp, ok := s.blk.Parameters.(*parser.BlockParametersNode); ok && bp.Parameters != nil {
			params = append(params, s.m.File.text(bp.Parameters.Location))
		}
		capArgs := strings.Join(s.caps, ", ")
		fmt.Fprintf(&b, "def __rugaroo_smdef_%d(__rugaroo_name%s)\n  @__rugaroo_sm = (@__rugaroo_sm || []) + [[%d, __rugaroo_name.to_s%s]]\n  nil\nend\n",
			i, prefixComma(capArgs), i, prefixComma(capArgs))
		fmt.Fprintf(&b, "def __rugaroo_sm_%d(%s)\n%s\nend\n", i, strings.Join(params, ", "), blockAsMethodBody(s.m.File, s.blk))
		if s.name == "" {
			fmt.Fprintf(&dispatch, "    return __rugaroo_sm_%d(%s*__rugaroo_args) if __rugaroo_e[0] == %d\n", i, smCaps(len(s.caps)), i)
		}
		if c.smSites[s.call] == nil {
			loc := s.call.Location
			args := make([]parser.Node, 0, 1+len(s.caps))
			args = append(args, callArgs(s.call)[0])
			for _, name := range s.caps {
				args = append(args, &parser.LocalVariableReadNode{Location: loc, Name: name})
			}
			c.smSites[s.call] = &parser.CallNode{Location: loc, Receiver: s.call.Receiver, Name: fmt.Sprintf("__rugaroo_smdef_%d", i), Arguments: &parser.ArgumentsNode{Location: loc, Arguments: args}}
		}
		c.keepNames = append(c.keepNames, fmt.Sprintf("__rugaroo_smdef_%d", i), fmt.Sprintf("__rugaroo_sm_%d", i))
	}
	b.WriteString("def __rugaroo_smfind(s)\n  l = @__rugaroo_sm\n  return nil unless l\n  i = l.size - 1\n  while i >= 0\n    e = l[i]\n    return e if e[1] == s\n    i -= 1\n  end\n  nil\nend\n")
	if computed {
		b.WriteString("def method_missing(__rugaroo_sym, *__rugaroo_args)\n  __rugaroo_e = __rugaroo_smfind(__rugaroo_sym.to_s)\n  if __rugaroo_e\n")
		b.WriteString(dispatch.String())
		b.WriteString("  end\n  super\nend\n")
		b.WriteString("def respond_to_missing?(__rugaroo_sym, __rugaroo_all = false)\n  !__rugaroo_smfind(__rugaroo_sym.to_s).nil? || super\nend\n")
	}
	f := sites[ids[0]].m.File
	c.addSynthDefs(ctx, f, sites[ids[0]].call, cls, b.String())
	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		c.overrideWithSingletons(ctx, cls, name, sites, overrides[name])
	}
	resetMsets(cls)
	c.keepNames = append(c.keepNames, "__rugaroo_smfind")
	if computed {
		for _, name := range []string{"method_missing", "respond_to_missing?"} {
			if !slices.Contains(c.keepNames, name) {
				c.keepNames = append(c.keepNames, name)
			}
		}
	}
}

// addSynthDefs parses src's defs and adds them to cls, pinned to at's line in f.
func (c *Compiler) addSynthDefs(ctx context.Context, f *File, at parser.Node, cls *Class, src string, public ...string) {
	const label = "define_singleton_method template"
	sf := c.synthFile(ctx, f, at, label, src, synthOpt{Origin: originSource, Pin: true})
	for _, def := range c.synthDefs(f, at, label, sf) {
		c.addDef(sf, cls, def, def.Name != "method_missing" && !strings.HasPrefix(def.Name, "__rugaroo_smdef_") && !slices.Contains(public, def.Name), c.nestingOf(cls)) // a recording call may have a receiver
	}
}

// overrideWithSingletons re-reads cls's own def of name with a first statement that runs the object's latest
// recorded singleton method of that name, when it has one.
func (c *Compiler) overrideWithSingletons(ctx context.Context, cls *Class, name string, sites []selfSingletonSite, ids []int) {
	om := cls.Methods[name]
	d, f := om.Node, om.File
	fwd, ok := defForwardArgs(d)
	if !ok {
		c.warn(f, d, "define_singleton_method(:%s) cannot be checked for in this def's parameters; the def runs as is", name)
		return
	}
	var pro strings.Builder
	pro.WriteString("; __rugaroo_e = __rugaroo_smfind(" + fmt.Sprintf("%q", name) + "); if __rugaroo_e;")
	for _, i := range ids {
		fmt.Fprintf(&pro, " return __rugaroo_sm_%d(%s%s) if __rugaroo_e[0] == %d;", i, smCaps(len(sites[i].caps)), fwd, i)
	}
	pro.WriteString(" end")
	c.prependToDef(ctx, cls, om, pro.String())
}

// defForwardArgs is a call passing d's parameters on as they came; false when one cannot be named (`*`, a pattern).
func defForwardArgs(d *parser.DefNode) (string, bool) {
	ps := d.Parameters
	if ps == nil {
		return "", true
	}
	var out []string
	for _, p := range slices.Concat(ps.Requireds, ps.Optionals) {
		switch p := p.(type) {
		case *parser.RequiredParameterNode:
			out = append(out, p.Name)
		case *parser.OptionalParameterNode:
			out = append(out, p.Name)
		default:
			return "", false
		}
	}
	if r, ok := ps.Rest.(*parser.RestParameterNode); ok && r.Name != nil {
		out = append(out, "*"+*r.Name)
	} else if ps.Rest != nil {
		return "", false
	}
	for _, p := range ps.Posts {
		r, ok := p.(*parser.RequiredParameterNode)
		if !ok {
			return "", false
		}
		out = append(out, r.Name)
	}
	for _, k := range ps.Keywords {
		switch k := k.(type) {
		case *parser.RequiredKeywordParameterNode:
			out = append(out, k.Name+": "+k.Name)
		case *parser.OptionalKeywordParameterNode:
			out = append(out, k.Name+": "+k.Name)
		}
	}
	if kr, ok := ps.KeywordRest.(*parser.KeywordRestParameterNode); ok && kr.Name != nil {
		out = append(out, "**"+*kr.Name)
	} else if ps.KeywordRest != nil {
		return "", false
	}
	if ps.Block != nil {
		if ps.Block.Name == nil {
			return "", false
		}
		out = append(out, "&"+*ps.Block.Name)
	}
	return strings.Join(out, ", "), true
}

// smCaps is a recorded entry's captures as arguments: `e[2], e[3], ` for two.
func smCaps(n int) string {
	var b strings.Builder
	for j := range n {
		fmt.Fprintf(&b, "__rugaroo_e[%d], ", j+2)
	}
	return b.String()
}

func prefixComma(s string) string {
	if s == "" {
		return ""
	}
	return ", " + s
}

func (c *Compiler) singletonTemplatesIn(ctx context.Context, cls *Class, m *Method, n parser.Node, sites *[]selfSingletonSite) {
	switch n := n.(type) {
	case nil, *parser.DefNode:
		return
	case *parser.CallNode:
		if c.singletonTemplate(ctx, cls, m, n) {
			return
		}
		if s, ok := c.selfSingleton(cls, m, n); ok {
			*sites = append(*sites, s)
			return
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.singletonTemplatesIn(ctx, cls, m, ch, sites)
	}
}

func (c *Compiler) singletonTemplate(ctx context.Context, cls *Class, m *Method, n *parser.CallNode) bool {
	blk, ok := n.Block.(*parser.BlockNode)
	iv, isIvar := n.Receiver.(*parser.InstanceVariableReadNode)
	args := callArgs(n)
	if !ok || !isIvar || n.Name != "define_singleton_method" || len(args) != 1 || blk.Parameters != nil || blk.Body == nil {
		return false
	}
	nameVar, ok := args[0].(*parser.LocalVariableReadNode)
	if !ok || !defParamNamed(m.Node, nameVar.Name) {
		return false
	}
	site := &dmSite{file: m.File, call: n, block: blk, scope: m.Scope}
	if caps := c.dmCaptures(site); len(caps) != 1 || caps[0] != nameVar.Name {
		return false
	}
	target := c.ivarAnnotatedClass(cls, iv.Name)
	if target == nil || target.lookup("method_missing") != nil && target.lookup("method_missing").Owner == target {
		return false
	}
	src := "def method_missing(" + nameVar.Name + ", *_rugaroo_rest)\n" + blockAsMethodBody(m.File, blk) + "\nend\n"
	sf, def := c.synthDef(ctx, m.File, n, "define_singleton_method template", src, synthOpt{Origin: originSource, Pin: true})
	c.addDef(sf, target, def, false, c.nestingOf(target))
	resetMsets(target)
	if c.dmSites == nil {
		c.dmSites = map[*parser.CallNode]bool{}
	}
	c.dmSites[n] = true // the run-time call: a no-op, as a boot define_method's is (decision 198)
	if !slices.Contains(c.keepNames, "method_missing") {
		c.keepNames = append(c.keepNames, "method_missing") // reached by any unknown call, never by its name
	}
	return true
}

// ivarAnnotatedClass is the class an ivar of cls is declared as by a `#:` (or T.let) annotation on one of its
// assignments in cls's methods, resolved in the method's lexical scope.
func (c *Compiler) ivarAnnotatedClass(cls *Class, name string) *Class {
	for _, m := range cls.MethodList {
		if m.Node == nil || m.File == nil {
			continue
		}
		var found *Class
		var walk func(parser.Node)
		walk = func(n parser.Node) {
			if found != nil || n == nil {
				return
			}
			if w, ok := n.(*parser.InstanceVariableWriteNode); ok && w.Name == name {
				if ann := strings.TrimSpace(m.File.trailingAnnotation(w)); ann != "" {
					found = c.scopedClass(ann, append(slices.Clip(m.Scope), cls))
				}
			}
			for _, ch := range n.CompactChildNodes() {
				walk(ch)
			}
		}
		walk(m.Node.Body)
		if found != nil {
			return found
		}
	}
	return nil
}

// scopedClass resolves a constant path as Ruby would from inside scope (innermost first), then at top level.
func (c *Compiler) scopedClass(name string, scope []*Class) *Class {
	name = strings.TrimPrefix(name, "::")
	for i := len(scope) - 1; i >= 0; i-- {
		if k := c.classes[scope[i].RubyName+"::"+name]; k != nil {
			return k
		}
	}
	return c.classes[name]
}

// nestingOf is the lexical scope a def written in cls's body has: each enclosing constant of its path, then cls.
func (c *Compiler) nestingOf(cls *Class) []*Class {
	var out []*Class
	parts := strings.Split(cls.RubyName, "::")
	for i := range parts {
		if k := c.classes[strings.Join(parts[:i+1], "::")]; k != nil {
			out = append(out, k)
		}
	}
	return out
}

// defParamNamed: d has a required or optional parameter called name (Method.Params are filled only at link).
func defParamNamed(d *parser.DefNode, name string) bool {
	ps := d.Parameters
	if ps == nil {
		return false
	}
	for _, p := range ps.Requireds {
		if r, ok := p.(*parser.RequiredParameterNode); ok && r.Name == name {
			return true
		}
	}
	for _, p := range ps.Optionals {
		if o, ok := p.(*parser.OptionalParameterNode); ok && o.Name == name {
			return true
		}
	}
	return false
}

// collectBootAliases declares the aliases boot's alias_method calls with a computed name made, and makes those calls
// no-ops (decision 330): `%w[b i].each { |t| alias_method("convert_#{t}".to_sym, :convert_em) }` in a class body.
func (c *Compiler) collectBootAliases(entries []boot.AliasInfo) {
	if len(entries) == 0 {
		return
	}
	byLine := map[boot.Loc][]boot.AliasInfo{}
	for _, e := range entries {
		k := boot.Loc{File: realPath(e.File), Line: e.Line}
		byLine[k] = append(byLine[k], e)
	}
	for _, path := range slices.Sorted(maps.Keys(c.parsed)) {
		f := c.parsed[path]
		anyNode(f.Root, func(n parser.Node) bool {
			call, ok := n.(*parser.CallNode)
			if !ok || call.Name != "alias_method" || call.Receiver != nil || allLiteralNames(callArgs(call)) {
				return false
			}
			loc := call.Location
			if call.MessageLoc != nil {
				loc = *call.MessageLoc
			}
			es := byLine[f.bootLoc(loc.StartOffset)]
			if len(es) == 0 {
				return false
			}
			c.dmSites[call] = true
			for _, e := range es {
				// ponytail: a computed singleton alias stays a run-time NoMethodError; route it through singletonMisc when a gem needs one
				if cls := c.classes[e.Owner]; cls != nil && !e.Singleton {
					c.addAliasNamed(f, cls, call, e.Name, e.Old)
				}
			}
			return false
		})
	}
}

// latentIvar is a class object's hidden ivar listing the latent includes its run-time include calls made (decision 331).
const latentIvar = "@__rugaroo_included"

// collectBootIncludes adds each module a run-time include call added during boot (liquid's Strainer.add_filter:
// `send(:include, filter)`) to its class as a latent include (decision 331): the methods are compiled in, while
// include? and ancestors report the module only once that class object's include call has run (rbIncludeAtRun).
// A class body's own `include M` is already there and is skipped.
func (c *Compiler) collectBootIncludes(entries []boot.IncludeInfo) {
	if len(entries) == 0 {
		return
	}
	sites := map[boot.Loc]*Class{} // each anonymous class's Class.new call
	for _, f := range c.parsed {
		anyNode(f.Root, func(n parser.Node) bool {
			if call, ok := n.(*parser.CallNode); ok && c.anonClasses[call] != nil {
				sites[f.bootLoc(call.Location.StartOffset)] = c.anonClasses[call]
			}
			return false
		})
	}
	for _, e := range entries {
		target := c.classes[e.Owner]
		if e.Site != "" {
			at, _ := boot.ParseLoc(e.Site)
			target = sites[boot.Loc{File: realPath(at.File), Line: at.Line}]
		}
		mod := c.classes[e.Module]
		if target == nil || mod == nil || !mod.IsModule || target.IsModule || c.staticallyIncludes(target, mod) {
			continue
		}
		f := c.parsed[realPath(e.File)]
		if f == nil {
			continue // an include from outside the closed world (a gem rugaroo does not compile)
		}
		target.Includes = append(target.Includes, Include{ref: constRef{node: constPathNode(e.Module, parser.Location{}), file: f},
			srcPos: srcPos{f, e.Line}, latent: true})
	}
}

// declareLatentIvars gives each class object with a latent include the ivar its include calls fill (after buildMetas).
func (c *Compiler) declareLatentIvars() {
	for _, cls := range c.classList {
		for _, inc := range cls.Includes {
			if inc.latent && cls.meta != nil {
				c.declareIvar(cls.meta, latentIvar, TAny{}, inc.File, inc.Line)
				break
			}
		}
	}
}

// staticallyIncludes reports whether cls's own body (or an earlier boot record) includes mod.
func (c *Compiler) staticallyIncludes(cls *Class, mod *Class) bool {
	return c.staticallyIncludesIn(cls, mod, map[*Class]bool{})
}

// staticallyIncludesIn also looks through the modules cls includes: one that includes mod already put it in cls's
// ancestors, after itself, and a second include would put mod's methods before the nearer module's overrides
// (rubocop's EndKeywordAlignment#style_parameter_name over ConfigurableEnforcedStyle's; decision 444).
func (c *Compiler) staticallyIncludesIn(cls *Class, mod *Class, seen map[*Class]bool) bool {
	if seen[cls] {
		return false
	}
	seen[cls] = true
	for _, inc := range cls.Includes {
		k := inc.Mod
		if k == nil && inc.ref.node != nil {
			_ = catchCompileError(func() { k, _ = c.lookupConst(inc.ref.file, inc.ref.node, inc.ref.scope) })
		}
		if k == mod || k != nil && c.staticallyIncludesIn(k, mod, seen) {
			return true
		}
	}
	return false
}

// staticDefineMethods are the boot records a gem's `[:a, :b].each { |m| define_method(m) { ... } }` in a class body would
// make (net-ftp's BufferedSocket forwarding send, shutdown, ...), for a compile with no boot (cmd/rubyspec's, decision 416):
// one per Symbol, its block param captured as that Symbol.
func (c *Compiler) staticDefineMethods() []boot.MethodInfo {
	var out []boot.MethodInfo
	for _, path := range slices.Sorted(maps.Keys(c.parsed)) {
		f := c.parsed[path]
		if !c.gemFile(f) {
			continue
		}
		var walk func(n parser.Node, owner []string)
		walk = func(n parser.Node, owner []string) {
			switch x := n.(type) {
			case *parser.ClassNode:
				owner = append(slices.Clone(owner), constPathText(f, x.ConstantPath))
				walk(x.Body, owner)
				return
			case *parser.ModuleNode:
				owner = append(slices.Clone(owner), constPathText(f, x.ConstantPath))
				walk(x.Body, owner)
				return
			case *parser.DefNode, *parser.SingletonClassNode:
				return
			case *parser.CallNode:
				out = append(out, staticDefineLoop(f, x, owner)...)
			}
			if n == nil {
				return
			}
			for _, ch := range n.CompactChildNodes() {
				walk(ch, owner)
			}
		}
		walk(f.Root, nil)
	}
	return out
}

// staticDefineLoop is staticDefineMethods for one `[...].each do |m| define_method(m) { } end` in owner's body.
func staticDefineLoop(f *File, call *parser.CallNode, owner []string) []boot.MethodInfo {
	arr, ok := call.Receiver.(*parser.ArrayNode)
	blk, ok2 := call.Block.(*parser.BlockNode)
	if !ok || !ok2 || call.Name != "each" || len(owner) == 0 || blk.Body == nil {
		return nil
	}
	params := blockParamNames(blk)
	st, ok := blk.Body.(*parser.StatementsNode)
	if len(params) != 1 || !ok || len(st.Body) != 1 {
		return nil
	}
	dm, ok := st.Body[0].(*parser.CallNode)
	if !ok || !isDefineMethodCall(dm) || dm.Receiver != nil || len(callArgs(dm)) != 1 {
		return nil
	}
	if lv, ok := callArgs(dm)[0].(*parser.LocalVariableReadNode); !ok || lv.Name != params[0] {
		return nil
	}
	loc := dm.Location
	if dm.MessageLoc != nil {
		loc = *dm.MessageLoc
	}
	var out []boot.MethodInfo
	for _, el := range arr.Elements {
		sym, ok := el.(*parser.SymbolNode)
		if !ok {
			return nil
		}
		name := sym.Unescaped.Value
		out = append(out, boot.MethodInfo{Owner: strings.Join(owner, "::"), Name: name, Loc: f.bootLoc(loc.StartOffset),
			Captures: map[string]string{params[0]: ":" + name}})
	}
	return out
}

// constPathText is a class or module statement's name as written (`FTP`, `Net::FTP`).
func constPathText(f *File, n parser.Node) string {
	return strings.TrimPrefix(f.text(n.GetLocation()), "::")
}

// blockParamNames are a block's plain required params, nil when it has any other kind.
func blockParamNames(b *parser.BlockNode) []string {
	bp, ok := b.Parameters.(*parser.BlockParametersNode)
	if !ok || bp.Parameters == nil {
		return nil
	}
	ps := bp.Parameters
	if len(ps.Optionals) > 0 || ps.Rest != nil || len(ps.Posts) > 0 || len(ps.Keywords) > 0 || ps.KeywordRest != nil || ps.Block != nil {
		return nil
	}
	var out []string
	for _, r := range ps.Requireds {
		rp, ok := r.(*parser.RequiredParameterNode)
		if !ok {
			return nil
		}
		out = append(out, rp.Name)
	}
	return out
}

// withStaticDefineMethods is boot's define_method records, or, with no boot to record them, the literal ones a gem's class
// body makes (decision 416).
func (c *Compiler) withStaticDefineMethods(opts *options, dms []boot.MethodInfo) []boot.MethodInfo {
	if opts.boot == nil && len(opts.gemDirs) > 0 {
		return c.staticDefineMethods()
	}
	return dms
}
