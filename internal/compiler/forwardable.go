package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// delegation is one Forwardable-generated method (decision 99): name calls
// method on accessor (`@ivar` or a method name).
type delegation struct {
	file     *File
	node     *parser.CallNode
	scope    []*Class
	accessor string
	method   string
	name     string
	single   bool // SingleForwardable: a class method (decision 132)
	nilable  bool // the accessor is T?: the forwarder raises NoMethodError on nil (decision 320)
}

// collectDelegation records `def_delegators :@x, :a, :b`,
// `def_delegator :@x, :a, :alias` and `delegate [:a, :b] => :@x`, and
// their def_instance_/def_single_ spellings.
func (c *Compiler) collectDelegation(f *File, cls *Class, n *parser.CallNode, args []parser.Node, scope []*Class, single bool) {
	sym := func(a parser.Node) string {
		switch s := a.(type) {
		case *parser.SymbolNode:
			return s.Unescaped.Value
		case *parser.StringNode:
			return s.Unescaped.Value
		}
		c.errorf(f, n, "%s takes literal Symbols or Strings", n.Name)
		return ""
	}
	add := func(accessor, method, name string) {
		cls.delegations = append(cls.delegations, delegation{file: f, node: n, scope: scope, accessor: accessor, method: method, name: name, single: single})
	}
	switch strings.Replace(strings.Replace(n.Name, "_single", "", 1), "_instance", "", 1) {
	case "def_delegators":
		if len(args) < 2 {
			c.errorf(f, n, "def_delegators needs an accessor and method names")
		}
		if names, ok := c.bootDelegators[f.bootLoc(n.Location.StartOffset)]; ok && !allLiteralNames(args[1:]) {
			for _, name := range names { // a list built at load time: the names boot saw (decision 204)
				add(sym(args[0]), name, name)
			}
			return
		}
		for _, a := range c.splatConsts(args[1:], scope) {
			add(sym(args[0]), sym(a), sym(a))
		}
	case "def_delegator":
		if len(args) < 2 || len(args) > 3 {
			c.errorf(f, n, "def_delegator takes an accessor, a method and an optional alias")
		}
		name := sym(args[1])
		if len(args) == 3 {
			name = sym(args[2])
		}
		add(sym(args[0]), sym(args[1]), name)
	default: // delegate [:a, :b] => :@x, :c => :@y
		h, ok := args[0].(*parser.KeywordHashNode)
		if len(args) != 1 || !ok {
			c.errorf(f, n, "%s takes `[:methods] => :accessor` pairs", n.Name)
		}
		for _, el := range h.Elements {
			as, ok := el.(*parser.AssocNode)
			if !ok {
				c.errorf(f, n, "%s takes `[:methods] => :accessor` pairs", n.Name)
			}
			acc := sym(as.Value)
			if arr, ok := as.Key.(*parser.ArrayNode); ok {
				for _, m := range arr.Elements {
					add(acc, sym(m), sym(m))
				}
				continue
			}
			add(acc, sym(as.Key), sym(as.Key))
		}
	}
}

// expandDelegations turns each delegation into an ordinary def, written
// out as Ruby with the target method's signature: `def size = @items.size`
// under `#: () -> Integer`. It runs in link once supers and includes are
// known and before signatures resolve, so the accessor's type must be
// declared: `# @rbs @items: T`, a `#:` on its assignment in initialize,
// or an annotated reader.
func (c *Compiler) expandDelegations(ctx context.Context) {
	added := false
	for _, cls := range c.classList {
		for _, d := range cls.delegations {
			owner := cls
			if d.single {
				owner = cls.meta
			}
			c.expandDelegation(ctx, owner, d)
			added = true
		}
	}
	if added { // lookups above cached method sets without the new defs
		for _, cls := range c.classList {
			cls.msetCache, cls.msetIndex = nil, nil
		}
	}
}

func (c *Compiler) expandDelegation(ctx context.Context, cls *Class, d delegation) {
	f, n := d.file, d.node
	recvT := c.delegateTarget(cls, d)
	if isAny(recvT) { // an untyped target (gem code): forward everything, dispatched at run time
		c.untypedDelegateDef(ctx, cls, d)
		return
	}
	if o, ok := recvT.(TOpt); ok { // `@dsl = T.let(DSL.new(self), T.nilable(DSL))`: the forwarder raises on nil, as MRI's does (decision 320)
		recvT, d.nilable = o.Elem, true
	}
	rc, ok := recvT.(TClass)
	if !ok {
		c.errorf(f, n, "%s: cannot delegate to %s of type %s", n.Name, d.accessor, recvT)
	}
	e := rc.C.lookup(d.method)
	if e == nil && c.gemFile(f) {
		c.untypedDelegateDef(ctx, cls, d) // MRI's forwarder raises NoMethodError when called; so does this one
		return
	}
	if e == nil {
		c.errorf(f, n, "%s: undefined method %s for %s", n.Name, d.method, recvT)
	}
	c.resolveMethod(e.M)
	m := e.M
	if m.inferRet && m.Ret == nil {
		if c.gemFile(f) {
			c.untypedDelegateDef(ctx, cls, d) // its return type is inferred later: forward without a signature
			return
		}
		c.errorf(f, n, "%s: %s#%s needs a return type annotation to be delegated to", n.Name, recvT, d.method)
	}
	env := composeEnv(e.Env, nil)
	for i, p := range rc.C.TypeParams {
		if i < len(rc.Args) {
			env[p] = rc.Args[i]
		}
	}
	env["Self"] = recvT
	required := 0
	for _, p := range m.Params {
		if !p.Rest && p.Default == nil {
			required++
		}
	}
	// One def per arity, as decision 12's overloads: omitted optional
	// arguments stay omitted, so the target runs its own defaults.
	c.delegateDef(ctx, cls, d, m, env, required, d.name, true)
	seen := map[string]bool{}
	for i, p := range m.Params {
		if p.Default != nil {
			name := fmt.Sprintf("__%s_%d", overloadBase(d.name), i+1)
			seen[name] = true
			c.delegateDef(ctx, cls, d, m, env, i+1, name, m.Block != nil)
		}
	}
	if strings.HasPrefix(d.method, "__") {
		return
	}
	// the target's arity overloads (`Enumerable#first(n)` beside `__first_0`) are delegated too
	prefix := "__" + overloadBase(d.method) + "_"
	for _, x := range rc.C.methodSet() {
		k, ok := strings.CutPrefix(x.M.Name, prefix)
		name := "__" + overloadBase(d.name) + "_" + k
		if !ok || k == "" || strings.Trim(k, "0123456789") != "" || seen[name] {
			continue
		}
		seen[name] = true
		o := d
		o.method, o.name = x.M.Name, name
		c.expandDelegation(ctx, cls, o)
	}
}

// delegateDef writes one delegated method taking the target's first
// count positional parameters (and its rest parameter, if any). With
// full, it carries the target's whole signature; otherwise per-parameter
// annotations, and its return is inferred from the forwarded call.
func (c *Compiler) delegateDef(ctx context.Context, cls *Class, d delegation, m *Method, env map[string]Type, count int, name string, full bool) {
	f, n := d.file, d.node
	if m.hasKeywords() && d.method != "[]" { // keywords: Ruby's own `...` forwarding passes them as written
		acc := d.accessor
		if acc[0] >= 'A' && acc[0] <= 'Z' {
			acc = "::" + acc
		}
		src := "def " + name + "(...) = " + acc + "." + d.method + "(...)\n"
		if d.nilable {
			src = "def " + name + "(...)\n" + nilGuard(acc, d.method) + "  target." + d.method + "(...)\nend\n"
		}
		sf, def := c.synthDef(ctx, f, n, n.Name+" "+d.name, src, synthOpt{Origin: f.synthOrigin(), Pad: max(f.line(n.Location.StartOffset)-1, 0)})
		c.addDef(sf, cls, def, false, d.scope)
		cls.Methods[name].fwdTarget = m.Owner // the delegation knows the target's class; `...` needs it before bodies type
		return
	}
	var ps, params, args []string
	var anns []string
	i := 0
	for _, p := range m.Params {
		if !p.Rest && i >= count {
			continue
		}
		a := fmt.Sprintf("a%d", len(params))
		t := subst(p.Type, env)
		if p.Rest {
			ps, params, args = append(ps, "*"+t.String()), append(params, "*"+a), append(args, "*"+a)
			anns = append(anns, fmt.Sprintf("# @rbs *%s: %s", a, t))
			continue
		}
		i++
		ps, params, args = append(ps, t.String()), append(params, a), append(args, a)
		anns = append(anns, fmt.Sprintf("# @rbs %s: %s", a, t))
	}
	sig := "(" + strings.Join(ps, ", ") + ")"
	if len(m.TypeParams) > 0 {
		sig = "[" + strings.Join(m.TypeParams, ", ") + "] " + sig
	}
	block := ""
	if m.Block != nil {
		bs := make([]string, len(m.Block.Params))
		ts := make([]string, len(m.Block.Params))
		for i, t := range m.Block.Params {
			bs[i] = fmt.Sprintf("b%d", i)
			ts[i] = subst(t, env).String()
		}
		self := ""
		if m.Block.Self != nil { // `T.proc.bind(Livecheck)`: the forwarder's block binds the same self
			self = " [self: " + subst(m.Block.Self, env).String() + "]"
		}
		if m.Block.Optional { // `livecheck` reads without a block and configures with one: pass on whichever the caller gave
			sig += fmt.Sprintf(" ?{ (%s)%s -> %s }", strings.Join(ts, ", "), self, subst(m.Block.Ret, env))
			params, args = append(params, "&blk"), append(args, "&blk")
		} else {
			sig += fmt.Sprintf(" { (%s)%s -> %s }", strings.Join(ts, ", "), self, subst(m.Block.Ret, env))
			block = fmt.Sprintf(" { |%s| yield %s }", strings.Join(bs, ", "), strings.Join(bs, ", "))
		}
	}
	sig += " -> " + subst(m.Ret, env).String()
	call := d.accessor
	if call[0] >= 'A' && call[0] <= 'Z' { // resolved from the top level, as delegateTarget typed it
		call = "::" + call
	}
	guard := ""
	if d.nilable {
		guard, call = nilGuard(call, d.method), "target"
	}
	if d.method == "[]" {
		call += "[" + strings.Join(args, ", ") + "]"
	} else {
		call += "." + d.method + "(" + strings.Join(args, ", ") + ")"
	}
	head := "#: " + sig
	if !full {
		head = strings.Join(anns, "\n")
	}
	// padded so the def's line (errors, //line) is the def_delegators call's
	line := f.line(n.Location.StartOffset)
	src := head + "\ndef " + name + "(" + strings.Join(params, ", ") + ")\n" + guard + "  " + call + block + "\nend\n"
	sf, def := c.synthDef(ctx, f, n, n.Name+" "+d.name, src, synthOpt{Origin: f.synthOrigin(), Pad: max(line-1-strings.Count(head, "\n")-1, 0)})
	c.addDef(sf, cls, def, false, d.scope)
}

// delegateTarget is the declared type of a delegation's accessor.
func (c *Compiler) delegateTarget(cls *Class, d delegation) Type {
	f, n := d.file, d.node
	sc := typeScope{class: cls, lex: d.scope, srcPos: srcPos{f, f.line(n.Location.StartOffset)}}
	switch acc := d.accessor; {
	case strings.Contains(acc, ".") && acc != "self.class" && c.gemFile(f): // `:"active_spec.resources"`: an expression, forwarded untyped
		return TAny{}
	case acc == "self.class" && cls.meta != nil: // `delegate [:desc] => :"self.class"`: the receiver's class methods
		return TClass{C: cls.meta}
	case acc == "$stdin" || acc == "$stdout" || acc == "$stderr": // the globals rugaroo has that hold objects (decision 61)
		return TClass{C: c.classes["IO"]}
	case strings.HasPrefix(acc, "$"):
		c.errorf(f, n, "%s: only $stdin, $stdout and $stderr can be delegated to", n.Name)
	case acc != "" && acc[0] >= 'A' && acc[0] <= 'Z':
		return c.delegateConst(f, n, acc)
	}
	if !strings.HasPrefix(d.accessor, "@") {
		e := cls.lookup(d.accessor)
		if e == nil {
			c.errorf(f, n, "%s: undefined method %s", n.Name, d.accessor)
		}
		c.resolveMethod(e.M)
		if e.M.Ret == nil {
			if c.gemFile(f) { // ruby-macho's `attr_reader :header` set from a call: forwarded untyped
				return TAny{}
			}
			c.errorf(f, n, "%s: %s needs a return type annotation to be delegated through", n.Name, d.accessor)
		}
		return e.M.Ret
	}
	if iv := c.findIvar(cls, d.accessor); iv != nil && iv.Type != nil {
		return iv.Type
	}
	for k := cls; k != nil; k = k.Super {
		// initialize first, then any method: Homebrew's Cask assigns `@dsl = T.let(..., T.nilable(DSL))` in refresh (decision 320)
		defs := append([]*Method{k.Methods["initialize"]}, k.MethodList...)
		var ann string
		for _, m := range defs {
			if m == nil || m.Node == nil {
				continue
			}
			anyNode(m.Node.Body, func(x parser.Node) bool {
				if w, ok := x.(*parser.InstanceVariableWriteNode); ok && w.Name == d.accessor && ann == "" {
					ann = m.File.trailingAnnotation(w)
				}
				return false
			})
		}
		if ann != "" {
			t, err := rbs.ParseType(ann)
			if err != nil {
				c.errorf(f, n, "%v", err)
			}
			return c.resolveType(t, sc)
		}
	}
	if c.gemFile(f) {
		return TAny{} // gem code: delegated calls dispatch dynamically (decision 172)
	}
	if cls.metaOf != nil {
		c.errorf(f, n, "%s: the type of %s must be declared to delegate to it (`# @rbs self.%s: T`, or `%s = ... #: T` in a class method)", n.Name, d.accessor, d.accessor, d.accessor)
	}
	c.errorf(f, n, "%s: the type of %s must be declared to delegate to it (`# @rbs %s: T`, or `%s = ... #: T` in initialize)", n.Name, d.accessor, d.accessor, d.accessor)
	return nil
}

// nilGuard reads a nilable accessor into `target` and raises MRI's NoMethodError when it is nil (decision 320).
func nilGuard(acc, method string) string {
	return "  target = " + acc + "\n  raise NoMethodError, \"undefined method '" + method + "' for nil\" unless target\n"
}

// splatConsts expands each `*NAMES` of a constant Array literal declared above (Rack::Lint's StreamWrapper).
func (c *Compiler) splatConsts(args []parser.Node, scope []*Class) []parser.Node {
	var out []parser.Node
	for _, a := range args {
		if sp, ok := a.(*parser.SplatNode); ok {
			if cr, ok := sp.Expression.(*parser.ConstantReadNode); ok && c.consts[qualify(scope, cr.Name)] != nil {
				if arr, ok := c.consts[qualify(scope, cr.Name)].Value.(*parser.ArrayNode); ok {
					out = append(out, arr.Elements...)
					continue
				}
			}
		}
		out = append(out, a)
	}
	return out
}

func allLiteralNames(args []parser.Node) bool {
	for _, a := range args {
		switch a.(type) {
		case *parser.SymbolNode, *parser.StringNode:
		default:
			return false
		}
	}
	return true
}

// untypedDelegateDef forwards name to the accessor with no signature, so inference types it like any gem def.
func (c *Compiler) untypedDelegateDef(ctx context.Context, cls *Class, d delegation) {
	f, n := d.file, d.node
	call, params := "target."+d.method+"(*args)", "*args" // ponytail: no block through an untyped target but each's: `m(*args, &block)` on an untyped receiver does not compile yet
	switch d.method {
	case "[]":
		call = "target[*args]"
	case "each": // its block, as Forwardable's forwarder passes it (rubocop's Config#each over @hash; decision 444)
		call, params = "target.each(&block)", "&block"
	}
	src := "def " + d.name + "(" + params + ")\n  target = " + d.accessor + " #: untyped\n  " + call + "\nend\n"
	sf, def := c.synthDef(ctx, f, n, n.Name+" "+d.name, src, synthOpt{Origin: f.synthOrigin(), Pad: max(f.line(n.Location.StartOffset)-3, 0)})
	c.addDef(sf, cls, def, false, d.scope)
}

// delegateConst types a delegation to a constant (`def_delegators :Foo, ...`): MRI evaluates the accessor inside
// Forwardable, so only top-level names resolve.
func (c *Compiler) delegateConst(f *File, n *parser.CallNode, acc string) Type {
	var path parser.Node
	for i, part := range strings.Split(acc, "::") {
		if i == 0 {
			path = &parser.ConstantReadNode{Name: part, Location: n.Location}
			continue
		}
		path = &parser.ConstantPathNode{Parent: path, Name: &part, Location: n.Location}
	}
	target, k := c.lookupConst(f, path, nil)
	switch {
	case k != nil:
		return c.constType(k)
	case target != nil:
		return TClass{C: c.metaFor(target)}
	}
	c.errorf(f, n, "%s: uninitialized constant %s", n.Name, acc)
	return nil
}
