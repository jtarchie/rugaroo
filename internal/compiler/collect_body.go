package compiler

import (
	"context"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

func (c *Compiler) collectBody(ctx context.Context, f *File, cls *Class, body parser.Node, scope []*Class, loadTime bool) {
	if body == nil {
		return
	}
	stmts, ok := body.(*parser.StatementsNode)
	if !ok {
		c.errorf(f, body, "unsupported class body %T", body)
	}
	vis := &visibility{}
	locals := bodyLocalNames(stmts.Body)
	prevGroup := c.bodyGroup
	c.bodyGroup = nil
	if loadTime && len(locals) > 0 {
		c.bodyGroup = &classStmt{Node: &parser.StatementsNode{}, file: f, scope: scope, unit: true}
	}
	defer func() { c.bodyGroup = prevGroup }()
	queue := stmts.Body
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		c.keepGoing(func() {
			switch n := n.(type) {
			case *parser.DefNode:
				c.scanAnon(ctx, f, n.Body, scope)
				// a bare `private` does not reach `def self.x`
				c.addMethod(f, cls, n, (vis.private || vis.moduleFunction) && n.Receiver == nil, scope)
				if n.Receiver == nil {
					c.applyVisibility(f, cls, n, vis, scope)
				}
			case *parser.CallNode:
				if isClassEvalCall(n) { // its block is collected as class-body statements, not dispatched here
					c.collectClassEval(ctx, f, cls, n, scope, loadTime)
					return
				}
				if !isDescribe(n) { // a describe's block is a class body, collected as one
					c.scanAnon(ctx, f, n, scope)
				}
				if !c.collectClassCall(ctx, f, cls, n, vis, scope) {
					c.declareBeginConsts(f, n, scope, loadTime) // `File.open(path) { |io| INDEX = Marshal.load(io.read) }` (unicode-display_width)
					c.classBodyCall(f, n, scope, loadTime)
				}
			case *parser.SingletonClassNode:
				c.scanAnon(ctx, f, n.Body, scope) // its defs' Class.new literals, as a class body's (decision 145)
				c.collectSingletonClass(f, cls, n, scope)
			case *parser.AliasMethodNode:
				c.addAlias(f, cls, n, n.NewName, n.OldName)
			case *parser.ClassVariableWriteNode:
				c.scanAnon(ctx, f, n.Value, scope) // liquid's `@@global_strainer = Class.new(Strainer) do ... end` (decision 331)
				c.addClassVar(f, cls, n, scope)
			case *parser.UndefNode:
				for _, name := range c.symbolArgs(f, nil, n.Names) {
					c.removeMethod(cls, name, true)
				}
			case *parser.ClassNode:
				c.collectClass(ctx, f, n, scope)
			case *parser.ModuleNode:
				c.collectModule(ctx, f, n, scope)
			case *parser.ConstantWriteNode:
				c.collectBodyConst(ctx, f, n, scope, locals)
			case *parser.MultiWriteNode, *parser.ConstantPathWriteNode:
				c.collectBodyWrite(ctx, f, n, scope, locals, loadTime)
			case *parser.IfNode, *parser.UnlessNode:
				if ifn, isIf := n.(*parser.IfNode); (!isIf || !ruby2KeywordsShim(ifn)) && c.runsStatements(loadTime) && !declares(n) && !c.folds(f, n, scope) {
					c.classBodyNode(ctx, f, n, scope, loadTime) // `raise "x" if OS.mac?`: a run-time condition around load-time code
					return
				}
				// spliced in place, so the branch shares the body's visibility and sees the constants before it
				queue = append(slices.Clip(c.classBodyBranch(f, n, scope)), queue...)
			default:
				c.declareBeginConsts(f, n, scope, loadTime)
				c.classBodyIvar(f, cls, n, scope)
				c.classBodyNode(ctx, f, n, scope, loadTime)
			}
		})
	}
	c.collectIvarDecls(f, cls, body, scope)
}

// declareBeginConsts declares the constants a class-body begin assigns, in its body, rescue or else clauses, where it
// runs: kramdown's `begin; require 'rouge'; AVAILABLE = true; rescue LoadError; AVAILABLE = false; end`, which MRI's
// `const_defined?(:AVAILABLE)` then sees (decision 334). The first assignment gives the type. A class-body call's block
// is the same: unicode-display_width's `File.open(INDEX_FILENAME, "rb") { |file| INDEX = Marshal.load(...) }`.
func (c *Compiler) declareBeginConsts(f *File, s parser.Node, scope []*Class, loadTime bool) {
	if !loadTime {
		return
	}
	var clauses []*parser.StatementsNode
	switch n := s.(type) {
	case *parser.BeginNode:
		clauses = append(clauses, n.Statements)
		for rc := n.RescueClause; rc != nil; rc = rc.Subsequent {
			clauses = append(clauses, rc.Statements)
		}
		if n.ElseClause != nil {
			clauses = append(clauses, n.ElseClause.Statements)
		}
	case *parser.CallNode:
		if b, ok := n.Block.(*parser.BlockNode); ok {
			if st, ok := b.Body.(*parser.StatementsNode); ok {
				clauses = append(clauses, st)
			}
		}
	default:
		return
	}
	for _, st := range clauses {
		if st == nil {
			continue
		}
		for _, s := range st.Body {
			w, ok := s.(*parser.ConstantWriteNode)
			if !ok || c.consts[qualify(scope, w.Name)] != nil {
				continue
			}
			c.addConst(f, w, w.Name, scope)
			c.consts[qualify(scope, w.Name)].inBody = true
		}
	}
}

// classBodyBranch is the statements a class-body if/unless runs: its folded branch (decision 167), or none for
// MRI's `ruby2_keywords(:m) if respond_to?(:ruby2_keywords, true)` guard (Ruby 4 always has it).
func (c *Compiler) classBodyBranch(f *File, n parser.Node, scope []*Class) []parser.Node {
	if ifn, isIf := n.(*parser.IfNode); isIf && ruby2KeywordsShim(ifn) {
		return nil
	}
	branch, ok := c.takenBranch(f, n, scope)
	if !ok {
		c.errorf(f, n, "unsupported node in class body: %s (only a condition known at compile time, such as RUBY_VERSION or defined?, is folded)", nodeType(n))
	}
	return branch
}

// classBodyCall handles a class-body call that is neither a known declaration
// nor a spec DSL call: a load-time statement where the class is defined, or
// an error when the body does not run at load time (decision 166).
func (c *Compiler) classBodyCall(f *File, n *parser.CallNode, scope []*Class, loadTime bool) {
	if !loadTime && c.anonRun != nil {
		*c.anonRun = append(*c.anonRun, n)
		return
	}
	if !loadTime {
		if n.Receiver != nil {
			c.errorf(f, n, "unsupported statement in class body: %s", f.text(n.Location))
		}
		c.errorf(f, n, "unsupported call in class body: %s", n.Name)
	}
	c.addClassStmt(f, n, scope)
}

// classBodyNode handles any other class-body expression the same way: a
// load-time statement, or an error when the body does not run at load time.
func (c *Compiler) classBodyNode(ctx context.Context, f *File, n parser.Node, scope []*Class, loadTime bool) {
	if !loadTime && c.anonRun != nil {
		c.scanAnon(ctx, f, n, scope)
		*c.anonRun = append(*c.anonRun, n)
		return
	}
	if !loadTime {
		c.errorf(f, n, "unsupported node in class body: %s", nodeType(n))
	}
	c.scanAnon(ctx, f, n, scope)
	c.addClassStmt(f, n, scope)
}

// undefLoop is `list.each { |m| undef_method m }` (or remove_method) over a computed list.
func undefLoop(n parser.Node) bool {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "each" {
		return false
	}
	b, ok := call.Block.(*parser.BlockNode)
	if !ok {
		return false
	}
	st, ok := b.Body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return false
	}
	inner, ok := st.Body[0].(*parser.CallNode)
	return ok && inner.Receiver == nil && (inner.Name == "undef_method" || inner.Name == "remove_method")
}

// ruby2KeywordsShim reports whether a class-body conditional is MRI's
// `ruby2_keywords(:m) if respond_to?(:ruby2_keywords, true)` compatibility
// guard, which does nothing in rugaroo's closed Ruby 4 world.
func ruby2KeywordsShim(n *parser.IfNode) bool {
	if n.Subsequent != nil || n.Statements == nil || len(n.Statements.Body) == 0 {
		return false
	}
	for _, st := range n.Statements.Body {
		call, ok := st.(*parser.CallNode)
		if !ok || call.Receiver != nil || call.Name != "ruby2_keywords" {
			return false
		}
	}
	return true
}

// visibility is the mode a bare private/protected/module_function sets for the defs after it.
type visibility struct {
	private, protected, moduleFunction bool
}

// applyVisibility marks instance method n under the current mode: protected, or with module_function also a module method.
func (c *Compiler) applyVisibility(f *File, cls *Class, n *parser.DefNode, vis *visibility, scope []*Class) {
	if vis.protected {
		cls.Methods[n.Name].Protected = true
	}
	if vis.moduleFunction {
		cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: n, scope: scope, file: f})
	}
}

// removeMethod drops cls's own def of name; undef also hides inherited ones from calls on cls.
func (c *Compiler) removeMethod(cls *Class, name string, undef bool) {
	if m := cls.Methods[name]; m != nil {
		delete(cls.Methods, name)
		cls.MethodList = deleteMethod(cls.MethodList, m)
	}
	if undef {
		if cls.undefs == nil {
			cls.undefs = map[string]bool{}
		}
		cls.undefs[name] = true
	}
}

// collectVisibilityCall handles private/public/protected, module_function,
// private_class_method and undef_method/remove_method in a class body.
func (c *Compiler) collectVisibilityCall(f *File, cls *Class, n *parser.CallNode, args []parser.Node, vis *visibility, scope []*Class) {
	defArg := func() *parser.DefNode { // `private def x ... end`
		if len(args) != 1 {
			return nil
		}
		d, _ := args[0].(*parser.DefNode)
		return d
	}
	named := func(name string) *Method {
		m := cls.Methods[name]
		if m == nil && !f.isPrelude() {
			// ponytail: `private :m` of an inherited or included method is ignored (it only restricts callers); copy it with the new visibility via linkAliases if a program depends on the NoMethodError
			return &Method{}
		}
		if m == nil {
			c.errorf(f, n, "undefined method '%s' for class '%s'", name, cls.RubyName)
		}
		return m
	}
	switch n.Name {
	case "private":
		switch d := defArg(); {
		case len(args) == 0:
			*vis = visibility{private: true}
		case d != nil:
			c.addMethod(f, cls, d, true, scope)
		default:
			for _, name := range c.symbolArgs(f, n, args) {
				named(name).Private = true
			}
		}
	case "public":
		if len(args) == 0 {
			*vis = visibility{}
		}
		for _, name := range c.symbolArgs(f, n, args) {
			m := named(name)
			m.Private, m.Protected = false, false
		}
	case "protected":
		switch d := defArg(); {
		case len(args) == 0:
			*vis = visibility{protected: true}
		case d != nil:
			c.addMethod(f, cls, d, false, scope)
			if d.Receiver != nil && cls.lookup(d.Name) == nil { // `protected def self.x` marks instance method x, as MRI's protected(:x) does
				c.errorf(f, n, "undefined method '%s' for class '%s'", d.Name, cls.RubyName)
			}
			named(d.Name).Protected = true
		default:
			for _, name := range c.symbolArgs(f, n, args) {
				named(name).Protected = true
			}
		}
	}
}

// collectMethodTableCall handles module_function, private_class_method and
// undef_method/remove_method: calls that copy, hide or drop methods.
func (c *Compiler) collectMethodTableCall(f *File, cls *Class, n *parser.CallNode, args []parser.Node, vis *visibility, scope []*Class) {
	named := func(name string) *Method {
		m := cls.Methods[name]
		if m == nil {
			c.errorf(f, n, "undefined method '%s' for class '%s'", name, cls.RubyName)
		}
		return m
	}
	switch n.Name {
	case "module_function":
		if !cls.IsModule {
			c.errorf(f, n, "module_function is only for modules")
		}
		if len(args) == 0 {
			*vis = visibility{moduleFunction: true}
		}
		for _, name := range c.symbolArgs(f, n, args) {
			m := named(name)
			if m.Node == nil {
				c.errorf(f, n, "module_function needs a def, not %s", name)
			}
			m.Private = true
			cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: m.Node, scope: scope, file: f})
		}
	case "private_class_method", "public_class_method":
		names := []string{}
		var d *parser.DefNode // `private_class_method def self.x ... end`
		if len(args) == 1 {
			d, _ = args[0].(*parser.DefNode)
		}
		if d != nil && d.Receiver != nil {
			c.addMethod(f, cls, d, false, scope)
			names = append(names, d.Name)
		} else {
			names = c.symbolArgs(f, n, args)
		}
		for _, name := range names {
			if name == "new" {
				cls.privateNew = n.Name == "private_class_method"
				continue
			}
			found := false
			for i := range cls.singletonDefs {
				if cls.singletonDefs[i].node.Name == name {
					cls.singletonDefs[i].private, found = n.Name == "private_class_method", true
				}
			}
			if !found && f.isPrelude() { // an inherited or extended method's visibility: ignored, as collectVisibilityCall does
				c.errorf(f, n, "undefined method '%s' for class '%s'", name, cls.RubyName)
			}
		}
	case "undef_method", "remove_method":
		for _, name := range c.symbolArgs(f, n, args) {
			if cls.Methods[name] == nil && n.Name == "remove_method" { // undef_method may hide an inherited one, unknown until link
				c.errorf(f, n, "method '%s' not defined in %s", name, cls.RubyName)
			}
			c.removeMethod(cls, name, n.Name == "undef_method")
		}
	}
}

// symbolArgs are a visibility call's method names (`private :a, "b"`).
func (c *Compiler) symbolArgs(f *File, n *parser.CallNode, args []parser.Node) []string {
	var out []string
	for _, a := range args {
		switch a := a.(type) {
		case *parser.SymbolNode:
			out = append(out, a.Unescaped.Value)
		case *parser.StringNode:
			out = append(out, a.Unescaped.Value)
		default:
			form := "undef"
			if n != nil {
				form = n.Name
			}
			c.errorf(f, a, "unsupported %s form", form)
		}
	}
	return out
}

// collectIOExtend takes `$stdout.extend(M)` (Homebrew's Utils::Output::Mixin on $stdout and $stderr) as IO including
// M (decision 229): the closed world has no per-object singleton classes, so every IO answers M's methods, which only
// respond_to? on another IO could tell. Only the standard streams, whose class is known.
func (c *Compiler) collectIOExtend(f *File, n *parser.CallNode, scope []*Class) bool {
	if n.Name != "extend" || f.isPrelude() {
		return false
	}
	var target *Class
	switch r := n.Receiver.(type) {
	case *parser.GlobalVariableReadNode:
		if r.Name == "$stdout" || r.Name == "$stderr" || r.Name == "$stdin" {
			target = c.classes["IO"]
		}
	case *parser.ConstantReadNode: // the one-object classes: ENV (brew's EnvActivation), the standard streams
		switch r.Name {
		case "ENV":
			target = c.classes["ENVClass"]
		case "STDOUT", "STDERR", "STDIN":
			target = c.classes["IO"]
		}
	}
	if target == nil {
		return false
	}
	for _, a := range callArgs(n) {
		c.addInclude(f, n, target, a, scope)
	}
	return true
}

// collectConstExtend is a body's `BinData.extend BitFieldFactory`: extend on a constant naming a class or module is
// that class's own `extend M`, as the body of the class would write it.
func (c *Compiler) collectConstExtend(ctx context.Context, f *File, n *parser.CallNode, scope []*Class) bool {
	if n.Name != "extend" || f.isPrelude() || n.Block != nil || len(callArgs(n)) == 0 {
		return false
	}
	switch n.Receiver.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
	default:
		return false
	}
	target, _ := c.lookupName(scope, constName(n.Receiver))
	if target == nil || target.metaOf != nil {
		return false
	}
	c.collectMixinCall(ctx, f, target, n, callArgs(n), scope)
	return true
}

// collectRefine takes a gem's `refine X do ... end` as reopening X (patchelf's StringIO#fill): the closed world has
// no other caller to keep the method from, so `using` is a no-op (decision 324).
func (c *Compiler) collectRefine(ctx context.Context, f *File, n *parser.CallNode, scope []*Class) bool {
	if n.Name == "using" && n.Receiver == nil && len(callArgs(n)) == 1 && c.gemFile(f) { // its refine blocks reopened their classes
		return true
	}
	blk, ok := n.Block.(*parser.BlockNode)
	if n.Name != "refine" || n.Receiver != nil || !ok || len(callArgs(n)) != 1 || !c.gemFile(f) {
		return false
	}
	target, _ := c.lookupName(scope, constName(callArgs(n)[0]))
	if target == nil || target.metaOf != nil {
		return false
	}
	if blk.Body != nil {
		c.collectBody(ctx, f, target, blk.Body, append(slices.Clip(scope), target), false)
	}
	return true
}

// rejectUnsupportedDecl reports a class-body declaration rugaroo recognizes but
// does not implement, with a clearer message than the load-time call fallback
// would give.
func (c *Compiler) rejectUnsupportedDecl(f *File, n *parser.CallNode) {
	if n.Name == "prepend" && f.isPrelude() {
		c.errorf(f, n, "unsupported call in class body: %s", n.Name)
	}
}

// collectClassCall handles a class-body call that is a declaration (attr_*,
// include, visibility, ...). It returns false for anything else, which a
// load-time class body takes as a statement to run when the class is defined
// (decision 166).
func (c *Compiler) collectClassCall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, vis *visibility, scope []*Class) bool {
	if c.anonDefineMethod(ctx, f, cls, n, scope) || c.collectIOExtend(f, n, scope) || c.collectConstExtend(ctx, f, n, scope) || c.collectRefineOrRequire(ctx, f, n, scope) {
		return true
	}
	if a := singletonAlias(n); a != nil { // `singleton_class.send :alias_method, :[], :new` (concurrent-ruby's LockFreeStack::Node, decision 253)
		cls.singletonMisc = append(cls.singletonMisc, singletonMisc{node: a, scope: scope, file: f})
		return true
	}
	if n.Name == "prepend" && !f.isPrelude() && n.Receiver != nil && prependTarget(n) != nil { // `::Module.prepend M` in a body, as at the top level
		recv, single := prependTarget(n), false
		if sc, ok := recv.(*parser.CallNode); ok {
			recv, single = sc.Receiver, true
		}
		for _, a := range callArgs(n) {
			c.prepends = append(c.prepends, pendingPrepend{targetRef: constRef{node: recv, scope: scope, file: f}, singleton: single, mod: constRef{node: a, scope: scope, file: f}})
		}
		return true
	}
	if names := staticAutoloadLoop(n); len(names) > 0 { // `%w[A B].each { |c| autoload c, path }` (unicode-emoji's lazy constants): each name's autoload, as written out
		al := n.Block.(*parser.BlockNode).Body.(*parser.StatementsNode).Body[0].(*parser.CallNode) //nolint:forcetypeassert // staticAutoloadLoop checked the shape
		for _, name := range names {
			c.noteAutoload(qualify(scope, name))
			one := &parser.CallNode{Location: al.Location, Name: "autoload", Arguments: &parser.ArgumentsNode{Location: al.Arguments.Location,
				Arguments: []parser.Node{&parser.SymbolNode{Location: callArgs(al)[0].GetLocation(), Unescaped: parser.RubyString{Value: name}}, callArgs(al)[1]}}}
			c.loadAutoload(ctx, f, one)
		}
		return true
	}
	if n.Receiver != nil {
		return false
	}
	c.rejectUnsupportedDecl(f, n)
	args := callArgs(n)
	switch n.Name {
	case "attr", "attr_reader", "attr_writer", "attr_accessor":
		c.addAttrs(f, cls, n, args, vis.private, scope)
	case "autoload", "ruby2_keywords", "deprecate_constant":
		// No-ops: autoload's file is loaded at compile time (the boot
		// snapshot, decision 161, supplies what it resolved); ruby2_keywords
		// is MRI's keyword-passthrough marker, which rugaroo passes statically;
		// deprecate_constant warns only with Warning[:deprecated], off by default.
		c.ruby2KeywordsDef(f, cls, n, args, vis.private, scope)
		c.noteAutoloadCall(n, args, scope)
		c.loadAutoload(ctx, f, n)
	case "include", "extend", "prepend", "mixes_in_class_methods":
		c.collectMixinCall(ctx, f, cls, n, args, scope)
	case "private", "public", "protected":
		c.collectVisibilityCall(f, cls, n, args, vis, scope)
	case "module_function", "private_class_method", "public_class_method", "undef_method", "remove_method":
		c.collectMethodTableCall(f, cls, n, args, vis, scope)
	case "private_constant", "public_constant":
		// a private constant reads only lexically: any `M::X` path is MRI's NameError, a compile error here (lookupConst)
		if c.privateConsts == nil {
			c.privateConsts = map[string]bool{}
		}
		for _, name := range c.symbolArgs(f, n, args) {
			c.privateConsts[cls.RubyName+"::"+name] = n.Name == "private_constant"
		}
	case "alias_method":
		if len(args) != 2 {
			c.errorf(f, n, "alias_method takes a new and an old name")
		}
		c.addAlias(f, cls, n, args[0], args[1])
	case "def_delegators", "def_delegator", "delegate":
		c.collectDelegation(f, cls, n, args, scope, cls.fwdSingle)
	case "def_instance_delegators", "def_instance_delegator", "instance_delegate":
		c.collectDelegation(f, cls, n, args, scope, false)
	case "def_single_delegators", "def_single_delegator", "single_delegate":
		if !cls.singleFwd {
			c.errorf(f, n, "%s needs `extend SingleForwardable` first", n.Name)
		}
		c.collectDelegation(f, cls, n, args, scope, true)
	default:
		if specForms[n.Name] && !f.isPrelude() {
			c.collectSpecForm(ctx, f, cls, n, scope)
			return true
		}
		c.collectFFICall(ctx, f, cls, n, scope)    // Library's attach_function/typedef: the call still runs at load (decision 375)
		c.collectFiddleCall(ctx, f, cls, n, scope) // Importer's extern/bind/typealias: the call still runs at load
		return c.selfExtendCall(f, cls, n)
	}
	return true
}

// selfExtendCall is a class-body call of a class method whose whole body is `extend M` (or `extend M unless ...`):
// concurrent-ruby's `safe_initialization!`. The class extends M here, as the call would at load (decision 241).
// ponytail: a guard is taken as true; and the method is found by a name no other class method has.
func (c *Compiler) selfExtendCall(f *File, cls *Class, n *parser.CallNode) bool {
	if len(callArgs(n)) > 0 || n.Block != nil || f.isPrelude() {
		return false
	}
	var found *singletonDef // supers are not linked yet: the one class method of that name, wherever it is
	for _, k := range c.classList {
		for i := range k.singletonDefs {
			if d := &k.singletonDefs[i]; d.node.Name == n.Name {
				if found != nil {
					return false
				}
				found = d
			}
		}
	}
	if found == nil {
		return false
	}
	ext := extendOnly(found.node.Body)
	if ext == nil {
		return false
	}
	for _, a := range callArgs(ext) {
		cls.extends = append(cls.extends, Include{ref: constRef{node: a, scope: found.scope, file: found.file}, srcPos: srcPos{found.file, f.line(n.Location.StartOffset)}, scope: found.scope})
	}
	return true
}

// extendOnly is the `extend M, ...` call a method body consists of, alone or under one if/unless, or nil.
func extendOnly(body parser.Node) *parser.CallNode {
	st, ok := body.(*parser.StatementsNode)
	if !ok || len(st.Body) != 1 {
		return nil
	}
	n := st.Body[0]
	switch x := n.(type) {
	case *parser.IfNode:
		if x.Subsequent != nil || x.Statements == nil {
			return nil
		}
		return extendOnly(x.Statements)
	case *parser.UnlessNode:
		if x.ElseClause != nil || x.Statements == nil {
			return nil
		}
		return extendOnly(x.Statements)
	case *parser.CallNode:
		if x.Receiver != nil || x.Name != "extend" || len(callArgs(x)) == 0 {
			return nil
		}
		for _, a := range callArgs(x) {
			switch a.(type) {
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
			default:
				return nil
			}
		}
		return x
	}
	return nil
}

// bodyLocalNames are the locals a class body assigns at its own level.
func bodyLocalNames(stmts []parser.Node) map[string]bool {
	out := map[string]bool{}
	for _, n := range stmts {
		switch n := n.(type) {
		case *parser.LocalVariableWriteNode:
			out[n.Name] = true
		case *parser.LocalVariableOperatorWriteNode:
			out[n.Name] = true
		case *parser.LocalVariableOrWriteNode:
			out[n.Name] = true
		case *parser.LocalVariableAndWriteNode:
			out[n.Name] = true
		case *parser.MultiWriteNode:
			for _, t := range n.Lefts {
				if lt, ok := t.(*parser.LocalVariableTargetNode); ok {
					out[lt.Name] = true
				}
			}
		}
	}
	return out
}

// readsLocal reports whether n reads one of the named locals.
func readsLocal(n parser.Node, names map[string]bool) bool {
	return anyNode(n, func(x parser.Node) bool {
		r, ok := x.(*parser.LocalVariableReadNode)
		return ok && names[r.Name]
	})
}

// collectIvarDecls picks up `# @rbs @x: T` annotations anywhere in a body.
func (c *Compiler) collectIvarDecls(f *File, cls *Class, body parser.Node, scope []*Class) {
	end := f.line(body.GetLocation().StartOffset + body.GetLocation().Length)
	nested := map[int]bool{} // a nested class's lines: its own annotations (a module's `@rbs @x` must not take a helper class's)
	if st, ok := body.(*parser.StatementsNode); ok {
		for _, s := range st.Body {
			switch s.(type) {
			case *parser.ClassNode, *parser.ModuleNode:
				for ln := f.line(s.GetLocation().StartOffset) + 1; ln <= f.line(s.GetLocation().StartOffset+s.GetLocation().Length); ln++ { // its first line keys the annotations before it, which are ours
					nested[ln] = true
				}
			}
		}
	}
	for ln := cls.Line; ln <= end+1; ln++ {
		if nested[ln] {
			continue
		}
		for _, iv := range f.annotations(ln)["ivar"] {
			name, ty, ok := strings.Cut(iv, ":")
			if !ok {
				c.errorAt(srcPos{f, ln}, "bad ivar annotation %q", iv)
			}
			t, err := rbs.ParseType(strings.TrimSpace(ty))
			if err != nil {
				c.errorAt(srcPos{f, ln}, "%v", err)
			}
			cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: strings.TrimSpace(name), rbs: t, line: ln, scope: scope})
		}
	}
}

// classBodyDecls are class-body calls that declare rather than run.
var classBodyDecls = map[string]bool{
	"attr": true, "attr_reader": true, "attr_writer": true, "attr_accessor": true, "include": true, "extend": true,
	"prepend": true, "private": true, "public": true, "protected": true, "module_function": true, "alias_method": true,
	"private_class_method": true, "public_class_method": true, "private_constant": true, "define_method": true,
	"undef_method": true, "remove_method": true,
}

// declares reports whether n holds a declaration, which a run-time condition cannot guard in a closed world.
func declares(n parser.Node) bool {
	return walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode, *parser.ConstantWriteNode,
			*parser.ConstantPathWriteNode, *parser.AliasMethodNode, *parser.UndefNode:
			return true, false
		case *parser.CallNode:
			return x.Receiver == nil && classBodyDecls[x.Name], true
		}
		return false, true
	})
}

func (c *Compiler) folds(f *File, n parser.Node, scope []*Class) bool {
	_, ok := c.takenBranch(f, n, scope)
	return ok
}

// singletonVisibility is `public :a, :b` / `private :a` inside `class << self`: it sets the named class methods'.
func (c *Compiler) singletonVisibility(f *File, cls *Class, s *parser.CallNode) {
	for _, name := range c.symbolArgs(f, s, callArgs(s)) {
		for i := range cls.singletonDefs {
			if cls.singletonDefs[i].node.Name == name {
				cls.singletonDefs[i].private = s.Name == "private"
			}
		}
	}
}

// collectSingletonCall is one call in a `class << self` body; it returns the private/public default after it.
func (c *Compiler) collectSingletonCall(f *File, cls *Class, s *parser.CallNode, private bool, scope []*Class) bool {
	switch {
	case s.Name == "private" && s.Receiver == nil && s.Arguments == nil:
		private = true
	case s.Name == "public" && s.Receiver == nil && s.Arguments == nil:
		private = false
	case (s.Name == "public" || s.Name == "private") && s.Receiver == nil: // `public :a, :b`: of an extended module's method, ignored as collectVisibilityCall does
		c.singletonVisibility(f, cls, s)
	case s.Receiver == nil && slices.Contains([]string{"attr", "attr_reader", "attr_writer", "attr_accessor", "alias_method"}, s.Name):
		cls.singletonMisc = append(cls.singletonMisc, singletonMisc{node: s, private: private, scope: scope, file: f})
	case (s.Name == "require" || s.Name == "require_relative") && s.Receiver == nil && c.methodRequiresRuntime: // the traced entry loaded it (decision 197)
	case s.Name == "private_constant" && s.Receiver == nil:
	case s.Name == "include" && s.Receiver == nil: // the singleton class's include is the class's extend
		for _, a := range callArgs(s) {
			c.addInclude(f, s, cls, a, scope)
			last := len(cls.Includes) - 1
			cls.extends = append(cls.extends, cls.Includes[last])
			cls.Includes = cls.Includes[:last]
		}
	case s.Name == "extend" && s.Receiver == nil && c.gemFile(f):
		// ponytail: `class << self; extend Forwardable`: the singleton class's own singleton, which only a load-time hook
		// (prime's def_delegator) calls; the hook's methods are made at compile time (decision 410)
	case s.Name == "prepend" && s.Receiver == nil && !f.isPrelude(): // X.singleton_class.prepend(M) (decision 199)
		for _, a := range callArgs(s) {
			c.prepends = append(c.prepends, pendingPrepend{target: cls, singleton: true, mod: constRef{node: a, scope: scope, file: f}})
		}
	case c.onlyDefinesMethods(s): // `CODES.each { |n, c| define_method(n) { ... } }`: boot made the defs (decision 198)
	default:
		c.singletonOtherCall(f, s, scope)
	}
	return private
}

// classBodyIvar records a class-body `@x = v #: T` as `# @rbs self.@x: T` would (decision 322): Homebrew's
// `@type_references = T.let({}, T::Hash[Symbol, TypeReference])`. Without an annotation the write is an ordinary
// load-time statement, typed by discovery like any other.
func (c *Compiler) classBodyIvar(f *File, cls *Class, stmt parser.Node, scope []*Class) {
	n, ok := stmt.(*parser.InstanceVariableWriteNode)
	if !ok || cls == nil {
		return
	}
	ann := f.trailingAnnotation(n)
	if ann == "" {
		if lit := literalRBS(n.Value); lit != "" && !c.gemFile(f) { // what discovery leaves untyped (decision 383)
			t, _ := rbs.ParseType(lit)
			c.classBodyLits = append(c.classBodyLits, classBodyLit{ivarDecl{name: n.Name, rbs: t, line: f.line(n.Location.StartOffset), scope: scope}, cls})
		}
		return
	}
	t, err := rbs.ParseType(ann)
	if err != nil {
		c.errorf(f, n, "%v", err)
	}
	cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: "self." + n.Name, rbs: t, line: f.line(n.Location.StartOffset), scope: scope})
}

// literalRBS is the type a scalar literal spells, "" for anything else.
func literalRBS(n parser.Node) string {
	switch n.(type) {
	case *parser.IntegerNode:
		return "Integer"
	case *parser.FloatNode:
		return "Float"
	case *parser.StringNode:
		return "String"
	case *parser.SymbolNode:
		return "Symbol"
	case *parser.TrueNode, *parser.FalseNode:
		return "bool"
	}
	return ""
}

// singletonOtherCall is any other call in a `class << self` body: a gem's call on a receiver runs where the body loads
// (liquid's `Template.default_exception_renderer = lambda ...`, decision 331); anything else is unsupported.
func (c *Compiler) singletonOtherCall(f *File, s *parser.CallNode, scope []*Class) {
	if s.Receiver == nil || !c.gemFile(f) {
		c.errorf(f, s, "unsupported call in `class << self`: %s", s.Name)
	}
	c.addClassStmt(f, s, scope)
}

// ruby2KeywordsDef collects `ruby2_keywords def method_missing(...)`'s def (drb's DRbObject, decision 416).
func (c *Compiler) ruby2KeywordsDef(f *File, cls *Class, n *parser.CallNode, args []parser.Node, private bool, scope []*Class) {
	if d, ok := firstArg(args).(*parser.DefNode); ok && n.Name == "ruby2_keywords" && len(args) == 1 {
		c.addMethod(f, cls, d, private, scope)
	}
}

// staticAutoloadLoop is the constant names of a class body's `[literal names].each do |c| autoload c, path end`, or nil.
func staticAutoloadLoop(call *parser.CallNode) []string {
	arr, ok := call.Receiver.(*parser.ArrayNode)
	blk, ok2 := call.Block.(*parser.BlockNode)
	if !ok || !ok2 || call.Name != "each" || blk.Body == nil {
		return nil
	}
	params := blockParamNames(blk)
	st, ok := blk.Body.(*parser.StatementsNode)
	if len(params) != 1 || !ok || len(st.Body) != 1 {
		return nil
	}
	al, ok := st.Body[0].(*parser.CallNode)
	if !ok || al.Name != "autoload" || al.Receiver != nil || len(callArgs(al)) != 2 {
		return nil
	}
	if lv, ok := callArgs(al)[0].(*parser.LocalVariableReadNode); !ok || lv.Name != params[0] {
		return nil
	}
	var out []string
	for _, el := range arr.Elements {
		switch e := el.(type) {
		case *parser.SymbolNode:
			out = append(out, e.Unescaped.Value)
		case *parser.StringNode:
			out = append(out, e.Unescaped.Value)
		default:
			return nil
		}
	}
	return out
}
