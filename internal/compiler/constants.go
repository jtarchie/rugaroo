package compiler

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// addConst records `NAME = value` declared in scope.
func (c *Compiler) addConst(f *File, n parser.Node, name string, scope []*Class) {
	var value parser.Node
	if w, ok := n.(*parser.ConstantWriteNode); ok {
		value = w.Value
	}
	c.addConstAt(f, n, qualify(scope, name), value, scope)
}

// addConstAt records constant full, assigned value in lexical scope (a `A::B = v` names its namespace itself).
func (c *Compiler) addConstAt(f *File, n parser.Node, full string, value parser.Node, scope []*Class) {
	if prev := c.consts[full]; prev != nil && c.classes[full] == nil && value != nil && !f.isPrelude() && prev.File == f {
		k := &Const{RubyName: full, Value: value, srcPos: srcPos{f, f.line(n.GetLocation().StartOffset)}, Scope: scope, first: prev}
		prev.rewrites = append(prev.rewrites, k)
		c.constList = append(c.constList, k)
		return
	}
	if c.consts[full] != nil || c.classes[full] != nil {
		c.errorf(f, n, "constant %s is already defined", full)
	}
	k := &Const{RubyName: full, GoName: goClassName(full), Value: value, srcPos: srcPos{f, f.line(n.GetLocation().StartOffset)}, Scope: scope}
	k.ann = f.trailingAnnotation(n)
	k.typeAlias = f.sorbetAliases[n]
	c.consts[full] = k
	c.constList = append(c.constList, k)
	c.noteConstName(full)
}

// noteConstName records a new constant in its namespace's table, in
// definition order (top-level ones belong to Object).
func (c *Compiler) noteConstName(full string) {
	i := strings.LastIndex(full, "::")
	if i < 0 {
		c.topConstNames = append(c.topConstNames, full)
		return
	}
	if parent := c.classes[full[:i]]; parent != nil {
		parent.constNames = append(parent.constNames, full[i+2:])
	}
}

// collectBodyConst declares a class-body constant; one that reads the body's locals runs in its unit (decision 170).
func (c *Compiler) collectBodyConst(ctx context.Context, f *File, n *parser.ConstantWriteNode, scope []*Class, locals map[string]bool) {
	if c.collectFiddleStruct(ctx, f, n, scope) {
		return
	}
	var inner []*parser.ConstantWriteNode
	if v := liftConstWrites(n.Value, &inner); v != n.Value {
		cp := *n // the parsed prelude is shared by concurrent compiles: rewrite a copy, never the cached node
		cp.Value = v
		n = &cp
	}
	for _, w := range inner { // `NoArgument = [NO_ARGUMENT = :NONE, nil]`: NO_ARGUMENT first, then a read of it (decision 244)
		c.collectBodyConst(ctx, f, w, scope, locals)
		if k := c.consts[qualify(scope, w.Name)]; k != nil {
			at := n.Value.GetLocation().StartOffset // at the outer one's offset, and added first: a stable sort keeps it first
			k.initAt = &at
		}
	}
	if arm, ok := c.loadTimeArm(f, n.Value, scope); ok && len(arm) > 0 && declares(n.Value) && len(scope) > 0 {
		// `Impl = case when on_cruby? then class CImpl < Base; end; CImpl ... end`: the arm boot ran declares, and its last value is the constant's
		c.collectBody(ctx, f, scope[len(scope)-1], &parser.StatementsNode{Location: n.Location, Body: arm[:len(arm)-1]}, scope, true)
		cp := *n
		cp.Value = arm[len(arm)-1]
		n = &cp
	}
	if call, kind := valueClass(n.Value); call != nil {
		c.collectValueClass(ctx, f, n, call, kind, scope)
		return
	}
	if call, isModule := anonClassCall(n.Value); call != nil && !f.isPrelude() {
		c.collectNamedAnon(ctx, f, n, call, isModule, scope)
		return
	}
	c.scanAnon(ctx, f, n.Value, scope)
	c.addConst(f, n, n.Name, scope)
	if c.bodyGroup != nil && readsLocal(n.Value, locals) {
		c.consts[qualify(scope, n.Name)].inBody = true
		c.addClassStmt(f, n, scope)
	}
}

// addClassVar declares `@@x = v` in a class body: a constant-like package
// variable, initialized where the body runs, that methods may also assign.
func (c *Compiler) addClassVar(f *File, cls *Class, n *parser.ClassVariableWriteNode, scope []*Class) {
	if cls.cvars[n.Name] != nil {
		c.errorf(f, n, "class variable %s is assigned twice in %s's body; assign it again in a method", n.Name, cls.RubyName)
	}
	k := &Const{RubyName: cls.RubyName + "::" + n.Name, GoName: cls.Name + "_cv_" + strings.TrimPrefix(n.Name, "@@"), Value: n.Value, srcPos: srcPos{f, f.line(n.Location.StartOffset)}, Scope: scope}
	k.ann = f.trailingAnnotation(n)
	if cls.cvars == nil {
		cls.cvars = map[string]*Const{}
	}
	cls.cvars[n.Name] = k
	c.constList = append(c.constList, k)
}

// methodClassVars declares the class variables a class's methods assign
// that no body did (#52): typed by the write's `#: T` or a literal value,
// unset until a write runs, so an earlier read is MRI's NameError.
func (c *Compiler) methodClassVars() {
	for _, cls := range c.classList {
		owner := cls
		if cls.metaOf != nil {
			owner = cls.metaOf
		}
		if cls.File == nil || cls.File.isPrelude() {
			continue
		}
		for _, m := range cls.MethodList {
			if m.Node == nil || m.File == nil || m.File.isPrelude() {
				continue
			}
			if len(m.Scope) > 0 && m.Scope[len(m.Scope)-1] != owner && m.Scope[len(m.Scope)-1].IsModule {
				continue // a prepended module's copy: @@x is the module's (Ruby's lexical lookup, decision 281)
			}
			anyNode(m.Node.Body, func(n parser.Node) bool {
				var name string
				var value parser.Node
				switch w := n.(type) {
				case *parser.ClassVariableWriteNode:
					name, value = w.Name, w.Value
				case *parser.ClassVariableOrWriteNode:
					name, value = w.Name, w.Value
				default:
					return false
				}
				for _, a := range owner.ancestors() {
					if a.cvars[name] != nil {
						return false
					}
				}
				k := &Const{RubyName: owner.RubyName + "::" + name, GoName: owner.Name + "_cv_" + strings.TrimPrefix(name, "@@"), Value: value, srcPos: srcPos{m.File, m.File.line(n.GetLocation().StartOffset)}, Scope: m.Scope, inMethod: true, guarded: true}
				k.ann = m.File.trailingAnnotation(n)
				if k.ann == "" && !isPlainLiteral(value) {
					c.errorf(m.File, n, "class variable %s is first assigned in a method: annotate it (`%s = v #: T`)", name, name)
				}
				if owner.cvars == nil {
					owner.cvars = map[string]*Const{}
				}
				owner.cvars[name] = k
				c.constList = append(c.constList, k)
				return false
			})
		}
	}
}

// privateRef: generated source may name a private constant by path (a Struct's `==` names Homebrew's private Vulns::Scanner::Target).
func (c *Compiler) privateRef(f *File, full string) bool { return c.privateConsts[full] && !f.synth }

// selfConstVaries reports a subclass of parent with its own constant (or class) name: `self::name` depends on the receiver.
func (c *Compiler) selfConstVaries(parent *Class, name string) bool {
	if parent.metaOf != nil {
		parent = parent.metaOf
	}
	for _, sub := range c.classList {
		if sub != parent && sub.isSubclassOf(parent) && (c.consts[sub.RubyName+"::"+name] != nil || c.classes[sub.RubyName+"::"+name] != nil) {
			return true
		}
	}
	return false
}

// selfConstParent is `self::X` in a class body or class method: the lexical class, unless a subclass has its own X,
// which MRI would pick by the receiver at run time.
func (c *Compiler) selfConstParent(f *File, n *parser.ConstantPathNode, scope []*Class) *Class {
	parent := scope[len(scope)-1]
	for _, sub := range c.classList {
		if sub != parent && sub.isSubclassOf(parent) && (c.consts[sub.RubyName+"::"+*n.Name] != nil || c.classes[sub.RubyName+"::"+*n.Name] != nil) {
			c.errorf(f, n, "self::%s: %s has its own %s, which MRI picks by the receiver at run time; name the class", *n.Name, sub.RubyName, *n.Name)
		}
	}
	return parent
}

// lookupConst resolves Foo, Foo::Bar, ::Foo as Ruby does: lexical scope innermost-out, the innermost class's ancestors, then top level; at most one result is non-nil.
func (c *Compiler) lookupConst(f *File, n parser.Node, scope []*Class) (*Class, *Const) {
	switch n := n.(type) {
	case *parser.ConstantReadNode:
		return c.lookupName(scope, n.Name)
	case *parser.ConstantPathNode:
		if n.Parent == nil {
			return c.classes[*n.Name], c.consts[*n.Name]
		}
		var parent *Class
		var k *Const
		if _, ok := n.Parent.(*parser.SelfNode); ok && len(scope) > 0 {
			if f != nil && c.gemFile(f) && c.selfConstVaries(scope[len(scope)-1], *n.Name) {
				return nil, nil // no one constant: genConstRead asks the receiver at run time (decision 444)
			}
			parent = c.selfConstParent(f, n, scope)
		} else {
			parent, k = c.lookupConst(f, n.Parent, scope)
		}
		if parent == nil {
			if k != nil {
				c.errorf(f, n, "%s is not a class or module", k.RubyName)
			}
			return nil, nil
		}
		for _, anc := range parent.ancestors() {
			full := anc.RubyName + "::" + *n.Name
			if cls, k := c.classes[full], c.consts[full]; cls != nil || k != nil {
				if c.privateRef(f, full) {
					c.errorf(f, n, "private constant %s referenced", full)
				}
				if cls == nil {
					if t := c.constClass(k); t != nil { // `Collection::MapImplementation` names a class (decision 203)
						return t, nil
					}
				}
				return cls, k
			}
		}
	}
	return nil, nil
}

func (c *Compiler) lookupName(scope []*Class, name string) (*Class, *Const) {
	// A qualified name from RBS ("A::B") resolves its head lexically.
	head, rest, qualified := strings.Cut(name, "::")
	find := func(full string) (*Class, *Const) {
		if qualified {
			full += "::" + rest
		}
		cls, k := c.classes[full], c.consts[full]
		if cls == nil && k != nil {
			if t := c.constClass(k); t != nil { // `B = A`: B names A's class (decision 203)
				return t, nil
			}
		}
		return cls, k
	}
	for i := len(scope) - 1; i >= 0; i-- {
		if cls, k := find(scope[i].RubyName + "::" + head); cls != nil || k != nil {
			return cls, k
		}
	}
	if len(scope) > 0 {
		for _, anc := range scope[len(scope)-1].ancestors()[1:] {
			if cls, k := find(anc.RubyName + "::" + head); cls != nil || k != nil {
				return cls, k
			}
		}
	}
	return find(head)
}

// resolveClassRef resolves a constRef that must name a class or module.
func (c *Compiler) resolveClassRef(r *constRef) *Class {
	cls, _ := c.lookupConst(r.file, r.node, r.scope)
	if cls == nil {
		c.errorf(r.file, r.node, "unknown class or module %s", r.file.text(r.node.GetLocation()))
	}
	return cls
}

// constClass is the class a constant names when its value is one: `B = A`, or what boot saw a load-time expression
// pick (`Impl = case ... when on_cruby? then MutexImpl ... end`, decision 203).
func (c *Compiler) constClass(k *Const) *Class {
	if k.resolving {
		return nil
	}
	k.resolving = true
	defer func() { k.resolving = false }()
	switch k.Value.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, _ := c.lookupConst(k.File, k.Value, k.Scope)
		return cls
	}
	if name, ok := c.bootConstClasses[k.RubyName]; ok {
		cls := c.classes[name]
		if t := c.classes[strings.TrimPrefix(name, "Thread::")]; t != nil && strings.HasPrefix(name, "Thread::") {
			cls = t // MRI names ::Queue, ::Mutex... Thread::Queue; the prelude keeps them top-level
		}
		if cls != nil { // the load-time expression is what boot saw it evaluate to: the class, named
			k.Value = constPathNode(cls.RubyName, k.Value.GetLocation())
		}
		return cls
	}
	return nil
}

// constPathNode is `::A::B::C` as Prism would parse it.
func constPathNode(name string, loc parser.Location) parser.Node {
	var n parser.Node
	for _, part := range strings.Split(name, "::") {
		p := part
		n = &parser.ConstantPathNode{Location: loc, Parent: n, Name: &p}
	}
	return n
}

// topConstVisibility collects `Klass.private_constant :X` at the top level as inside Klass's body (decision 241).
func (c *Compiler) topConstVisibility(f *File, n *parser.CallNode) bool {
	if n.Name != "private_constant" && n.Name != "public_constant" || n.Receiver == nil {
		return false
	}
	cls := c.lookupClassRef(f, n.Receiver)
	if cls == nil {
		return false
	}
	if c.privateConsts == nil {
		c.privateConsts = map[string]bool{}
	}
	for _, name := range c.symbolArgs(f, n, callArgs(n)) {
		c.privateConsts[cls.RubyName+"::"+name] = n.Name == "private_constant"
	}
	return true
}

// lookupClassRef is the class a top-level constant expression names, or nil.
func (c *Compiler) lookupClassRef(f *File, n parser.Node) *Class {
	switch n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, _ := c.lookupConst(f, n, nil)
		return cls
	}
	return nil
}

// collectBodyWrite is a body's `::X = v` / `Other::X = v` (that namespace's constant, valued in this scope) or
// `A, B = x, y` (each its own constant when all targets are, else a load-time statement), decision 241.
func (c *Compiler) collectBodyWrite(ctx context.Context, f *File, n parser.Node, scope []*Class, locals map[string]bool, loadTime bool) {
	if pw, ok := n.(*parser.ConstantPathWriteNode); ok {
		c.scanAnon(ctx, f, pw.Value, scope)
		c.addConstAt(f, pw, c.declName(f, pw.Target, scope), pw.Value, scope)
		return
	}
	mw := n.(*parser.MultiWriteNode) //nolint:forcetypeassert // collectBody's case
	if ws := constMultiWrite(mw); ws != nil {
		for _, w := range ws {
			c.collectBodyConst(ctx, f, w, scope, locals)
		}
		return
	}
	c.classBodyNode(ctx, f, mw, scope, loadTime)
}

// constMultiWrite is `A, B = x, y` with constant targets and as many plain values: one constant write each, or nil.
func constMultiWrite(n *parser.MultiWriteNode) []*parser.ConstantWriteNode {
	arr, ok := n.Value.(*parser.ArrayNode)
	if !ok || n.Rest != nil || len(n.Rights) > 0 || len(arr.Elements) != len(n.Lefts) || slices.ContainsFunc(arr.Elements, isSplat) {
		return nil
	}
	out := make([]*parser.ConstantWriteNode, len(n.Lefts))
	for i, l := range n.Lefts {
		t, ok := l.(*parser.ConstantTargetNode)
		if !ok {
			return nil
		}
		out[i] = &parser.ConstantWriteNode{Location: t.Location, Name: t.Name, NameLoc: t.Location, Value: arr.Elements[i]}
	}
	return out
}

// absConstPath is `::A::B` for the constant name "A::B".
func absConstPath(name string, loc parser.Location) parser.Node {
	var n *parser.ConstantPathNode
	for _, part := range strings.Split(name, "::") {
		var parent parser.Node
		if n != nil {
			parent = n
		}
		n = &parser.ConstantPathNode{Location: loc, Parent: parent, Name: &part}
	}
	return n
}

// objectConstExtend reports a gem's top-level `ARGV.extend(OptionParser::Arguable)`, on an object constant, which is
// dropped (ponytail: a closed world gives objects no singleton modules; decision 244).
func (c *Compiler) objectConstExtend(f *File, n *parser.CallNode) bool {
	return n.Name == "extend" && c.gemFile(f) && isConstNode(n.Receiver) && c.lookupClassRef(f, n.Receiver) == nil
}

// isConstNode reports a constant read or path.
func isConstNode(n parser.Node) bool {
	switch n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return true
	}
	return false
}

// liftConstWrites takes the constant writes nested in a constant's value (an Array literal's element, a call's
// receiver or argument) out into out, leaving a read of each in its place. Copy-on-write: the parsed prelude is shared
// by concurrent compiles, so a changed node is a copy and the cached tree is never written.
func liftConstWrites(n parser.Node, out *[]*parser.ConstantWriteNode) parser.Node {
	switch x := n.(type) {
	case *parser.ConstantWriteNode:
		cp := *x
		cp.Value = liftConstWrites(x.Value, out)
		*out = append(*out, &cp)
		return &parser.ConstantReadNode{Location: x.NameLoc, Name: x.Name}
	case *parser.ArrayNode:
		if els, changed := liftEach(x.Elements, out); changed {
			cp := *x
			cp.Elements = els
			return &cp
		}
	case *parser.CallNode:
		recv := x.Receiver
		if recv != nil {
			recv = liftConstWrites(recv, out)
		}
		args, changed := liftEach(callArgs(x), out)
		if recv != x.Receiver || changed {
			cp := *x
			cp.Receiver = recv
			if changed {
				an := *x.Arguments
				an.Arguments = args
				cp.Arguments = &an
			}
			return &cp
		}
	}
	return n
}

// liftEach is liftConstWrites over nodes: a copy of the slice when any changed.
func liftEach(nodes []parser.Node, out *[]*parser.ConstantWriteNode) ([]parser.Node, bool) {
	var outNodes []parser.Node
	for i, el := range nodes {
		if v := liftConstWrites(el, out); v != el {
			if outNodes == nil {
				outNodes = slices.Clone(nodes)
			}
			outNodes[i] = v
		}
	}
	return outNodes, outNodes != nil
}

// builtinGlobals are the globals genGlobalRead answers itself (decision 61); any other $name is the program's own.
var builtinGlobals = map[string]bool{"$!": true, "$0": true, "$PROGRAM_NAME": true, "$stdin": true, "$stdout": true, "$stderr": true,
	"$?": true, "$$": true, "$~": true, "$LOAD_PATH": true, "$:": true, "$-I": true, "$VERBOSE": true, "$/": true,
	"$@": true, "$>": true, "$<": true, "$*": true, "$;": true, "$,": true, "$\\": true, "$.": true}

// userGlobals declares the program's own globals (decision 361): every $name a non-prelude file writes, a package
// variable typed by its first write, as a class variable first assigned in a method is: its `#: T`, else a literal.
func (c *Compiler) userGlobals() {
	for _, f := range c.files {
		if f.isPrelude() || f.Root == nil {
			continue
		}
		anyNode(f.Root, func(n parser.Node) bool {
			var name string
			var value parser.Node
			switch w := n.(type) {
			case *parser.GlobalVariableWriteNode:
				name, value = w.Name, w.Value
			case *parser.GlobalVariableOrWriteNode:
				name, value = w.Name, w.Value
			default:
				return false
			}
			if builtinGlobals[name] || englishGlobals[name] != "" || englishMatchRefs[name] != "" || c.globals[name] != nil {
				return false
			}
			k := &Const{RubyName: name, GoName: "rbGv_" + globalGoName(name), Value: value, srcPos: srcPos{f, f.line(n.GetLocation().StartOffset)}, inMethod: true}
			k.ann = f.trailingAnnotation(n)
			if k.ann == "" && !isPlainLiteral(value) {
				c.errorf(f, n, "global variable %s is first assigned here: annotate it (`%s = v #: T`)", name, name)
			}
			if c.globals == nil {
				c.globals = map[string]*Const{}
			}
			c.globals[name] = k
			c.constList = append(c.constList, k)
			return false
		})
	}
}

// globalGoName spells a global's name as Go: `$-w`'s dash and the like as their hex code, letters, digits and _ as is.
func globalGoName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimPrefix(name, "$") {
		if r == '_' || r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "x%02x", r)
		}
	}
	return b.String()
}
