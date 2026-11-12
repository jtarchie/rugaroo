package compiler

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// Decision 169: a gem defines far more than a program runs, and code that never runs need not type-check.

// implicitNames are called by Ruby itself or by the runtime's Go helpers, never spelled at a call site.
var implicitNames = []string{
	"initialize", "initialize_copy", "initialize_dup", "initialize_clone", "to_s", "inspect", "to_str", "to_ary", "to_a",
	"to_h", "to_hash", "to_proc", "to_i", "to_int", "to_f", "to_r", "to_c", "to_sym", "to_io", "to_path", "each", "call",
	"==", "!=", "!", "eql?", "equal?", "hash", "<=>", "===", "=~", "coerce", "succ", "method_missing",
	"respond_to_missing?", "respond_to?", "inherited", "included", "extended", "prepended", "method_added",
	"singleton_method_added", "const_missing", "message", "full_message", "backtrace", "exception", "deconstruct",
	"deconstruct_keys", "marshal_dump", "marshal_load", "_dump", "_load", "freeze", "dup", "clone", "new", "allocate",
}

// reflectiveCalls take a method name as their first argument; a computed one could name any method.
var reflectiveCalls = map[string]bool{
	"send": true, "public_send": true, "__send__": true, "method": true, "public_method": true,
	"instance_method": true, "public_instance_method": true, "respond_to?": true, "define_method": true,
	"const_get": true,
}

// declarationCalls name methods or modules without calling or instantiating them.
var declarationCalls = map[string]bool{
	"attr": true, "attr_reader": true, "attr_writer": true, "attr_accessor": true, "private": true, "public": true,
	"protected": true, "module_function": true, "private_class_method": true, "public_class_method": true,
	"private_constant": true, "public_constant": true, "include": true, "extend": true, "prepend": true, "ruby2_keywords": true,
	"deprecate_constant": true,
}

// gemFile reports whether f was loaded from a gem's lib dir (decision 173), the code decision 169 prunes.
func (c *Compiler) gemFile(f *File) bool {
	if f == nil || f.isPrelude() || f.path == "" {
		return false
	}
	if f.origin == originSource { // asked per expression (lenient): resolving the gem dirs' symlinks each time was 44% of brew's gen CPU
		if c.gemDirsReal == nil {
			c.gemDirsReal = make([]string, len(c.gemDirs))
			for i, dir := range c.gemDirs {
				c.gemDirsReal[i] = realPath(dir)
			}
		}
		f.origin = originUser
		for _, dir := range c.gemDirsReal {
			if within(dir, f.path) {
				f.origin = originGem
				break
			}
		}
	}
	return f.origin == originGem
}

// userFile is a file of the program's own: not the prelude and not a gem's.
func (c *Compiler) userFile(f *File) bool { return !f.isPrelude() && !c.gemFile(f) }

// reachDef is one method body: walked once its name is reached and, for a gem class's, the class is live.
type reachDef struct {
	name   string
	owner  *Class // nil for a top-level def
	body   parser.Node
	file   *File
	scope  []*Class // the def's lexical scope, which its constants resolve in
	params map[string]*Class
	walked bool
}

type reach struct {
	c         *Compiler
	names     map[string]bool     // method names reachable code calls
	consts    map[string]bool     // constant names (last segment) reachable code may instantiate or call through
	reflected []string            // module prefixes whose constants a computed const_get or a `constants` call may name: none are pruned (regexp_parser's UnicodeProperty.all)
	aliases   map[string][]string // alias new name -> old names: reaching the new one reaches the old
	defs      []*reachDef
	goNames   map[string][]string // Go method name -> Ruby names, for names Go code calls
	live      map[*Class]bool
	dynamic   string // a computed reflective call, which reaches any name; "" when there is none
	from      string
	why       map[string]string // RUGAROO_PRUNE_DEBUG: what first reached each name
	named     map[*Class]bool   // the classes reachable code's constants resolve to
	byName    map[string]bool   // constants that resolve to no class here: live by last segment
	file      *File             // what the walk is in, for resolving constants lexically
	scope     []*Class
	pending   map[string][]pendingConst // a gem constant's effect-free initializer, walked once its name is reached
	params    map[string]*Class         // the walked def's parameters its signature types as a prelude class
	typed     map[string][]*Class       // names reached only on those classes (`res.cookies` with res a WEBrick::HTTPResponse)
}

type pendingConst struct {
	value parser.Node
	file  *File
	scope []*Class
}

// pruneGemMethods drops a gem's methods reachable code cannot call (decision 169): by name, and for a gem class's
// methods only while the class is live (reachable code names it, a subclass, or an includer).
// gemBackends are prelude libs that stand in for a gem's native code: compiled whole like any prelude file, so the gem
// methods every def calls stay, called or not (prism's Prism.lex needs Serialize.load_lex; decision 427).
var gemBackends = map[string]bool{"prelude/prism.rb": true}

func (c *Compiler) pruneGemMethods() {
	r := &reach{c: c, names: map[string]bool{}, consts: map[string]bool{}, aliases: map[string][]string{}, goNames: map[string][]string{},
		named: map[*Class]bool{}, byName: map[string]bool{}, pending: map[string][]pendingConst{}, typed: map[string][]*Class{}}
	debug := c.env.PruneDebug
	if debug {
		r.why = map[string]string{}
	}
	r.index()
	r.from = "implicit"
	for _, n := range implicitNames {
		r.add(n)
	}
	for _, n := range c.keepNames { // compiled stand-ins a call reaches without naming them (decision 230's method_missing)
		r.add(n)
	}
	for _, v := range c.verbatim {
		r.from = "Go in " + v.File.Name
		r.scanGo(v.code)
	}
	for _, f := range c.files {
		r.from, r.file, r.scope = f.Name, f, nil
		r.walk(f.Root, !c.userFile(f) && !gemBackends[f.Name]) // a user file is compiled whole; elsewhere a def runs only when called
	}
	for _, f := range c.erbSnippets {
		r.from, r.file, r.scope = f.Name, f, nil
		r.walk(f.Root, false)
	}
	for _, units := range c.runEvals { // a run-time eval's source runs where its site is (decision 374)
		for _, u := range units {
			r.from, r.file, r.scope = "eval at "+u.file.path, u.file, nil
			r.walk(u.file.Root, false)
		}
	}
	for changed := true; changed && r.dynamic == ""; {
		changed = false
		r.computeLive()
		for _, d := range r.defs {
			if !d.walked && r.reached(d.name, d.owner) {
				d.walked, changed = true, true
				r.from, r.file, r.scope, r.params = "def "+d.name, d.file, d.scope, d.params
				r.walk(d.body, false)
				r.params = nil
			}
		}
		for name, ps := range r.pending {
			if r.consts[name] {
				delete(r.pending, name)
				changed = true
				for _, p := range ps {
					r.from, r.file, r.scope = "constant "+name, p.file, p.scope
					r.nestedWrites(p.value)
					r.walk(p.value, false)
				}
			}
		}
	}
	if debug {
		r.dump()
	}
	if r.dynamic == "" {
		r.prune()
	}
}

func (r *reach) dump() {
	lines := make([]string, 0, len(r.why))
	for name, from := range r.why {
		lines = append(lines, fmt.Sprintf("rugaroo: reach %s <- %s", name, from))
	}
	for cls := range r.live {
		if r.gemClass(cls) {
			lines = append(lines, "rugaroo: live "+cls.RubyName)
		}
	}
	sort.Strings(lines)
	fmt.Fprintln(os.Stderr, strings.Join(lines, "\n"))
	if r.dynamic != "" {
		fmt.Fprintf(os.Stderr, "rugaroo: gem methods are not pruned: %s\n", r.dynamic)
	}
}

func (r *reach) index() {
	note := func(name string, owner *Class, n *parser.DefNode, file *File, scope []*Class) {
		var body parser.Node = n
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			if x, ok := st.Body[0].(*parser.XStringNode); ok {
				body = x
			}
		}
		r.defs = append(r.defs, &reachDef{name: name, owner: owner, body: body, file: file, scope: scope})
		g := goMethodName(name)
		r.goNames[g] = append(r.goNames[g], name)
	}
	for _, cls := range r.c.classList {
		for _, m := range cls.MethodList {
			if m.Node != nil {
				note(m.Name, cls, m.Node, m.File, m.Scope)
				r.defs[len(r.defs)-1].params = r.typedParams(m)
			}
		}
		for _, d := range cls.singletonDefs {
			note(d.node.Name, cls, d.node, d.file, d.scope)
		}
	}
	for _, m := range r.c.topDefList {
		if m.Node != nil {
			note(m.Name, nil, m.Node, m.File, m.Scope)
		}
	}
}

// gemClass is a class or module a gem declares; its methods count only while it is live.
func (r *reach) gemClass(cls *Class) bool { return cls != nil && r.c.gemFile(cls.File) }

func (r *reach) reached(name string, owner *Class) bool {
	if !r.names[name] && !slices.ContainsFunc(r.typed[name], func(c *Class) bool { return c == owner || r.inherits(c, owner) }) {
		return false
	}
	return !r.gemClass(owner) || r.live[owner]
}

// inherits reports whether owner is a superclass of cls.
func (r *reach) inherits(cls, owner *Class) bool {
	for c, n := cls, 0; c != nil && c.superRef != nil && n <= len(r.c.classList); n++ { // n: this runs before checkCycles, so a cycle not through owner would never end
		if c = r.resolve(c.superRef); c == owner {
			return true
		}
	}
	return false
}

// typedParams maps a gem method's parameters its signature types as a prelude class, whose calls then reach only that
// class's method: rackup's `res.cookies` is WEBrick's, not Rack::Request's (whose body needs more than the program does).
func (r *reach) typedParams(m *Method) map[string]*Class {
	if m.sig == nil || m.Node == nil || m.Node.Parameters == nil || !r.c.gemFile(m.File) {
		return nil
	}
	reqs := m.Node.Parameters.Requireds
	var out map[string]*Class
	for i, p := range m.sig.Params {
		name, ok := p.Type.(rbs.Name)
		rp, isReq := (*parser.RequiredParameterNode)(nil), false
		if i < len(reqs) {
			rp, isReq = reqs[i].(*parser.RequiredParameterNode)
		}
		if !ok || !isReq || p.Optional || p.Kind != rbs.ParamPositional {
			continue
		}
		cls := r.c.classes[strings.TrimPrefix(name.Name, "::")]
		if cls == nil || r.gemClass(cls) || writesLocal(m.Node.Body, rp.Name) {
			continue
		}
		if out == nil {
			out = map[string]*Class{}
		}
		out[rp.Name] = cls
	}
	return out
}

// writesLocal reports whether body writes local name.
func writesLocal(body parser.Node, name string) bool {
	return anyNode(body, func(n parser.Node) bool {
		w, ok := n.(*parser.LocalVariableWriteNode)
		return ok && w.Name == name
	})
}

// computeLive marks each gem class reachable code names, and its ancestors (superclasses, included and extended
// modules), live: only then can an instance or the class object receive a call.
func (r *reach) computeLive() {
	r.live = map[*Class]bool{}
	var mark func(cls *Class)
	mark = func(cls *Class) {
		if cls == nil || r.live[cls] {
			return
		}
		r.live[cls] = true
		if cls.superRef != nil {
			mark(r.resolve(cls.superRef))
		}
		for i := range cls.Includes {
			mark(r.resolve(&cls.Includes[i].ref))
		}
		for i := range cls.extends {
			mark(r.resolve(&cls.extends[i].ref))
		}
		for i := range r.c.prepends { // a prepended module's methods are the class's (decisions 199, 444)
			p := &r.c.prepends[i]
			if t := p.target; t == cls || t == nil && r.resolve(&p.targetRef) == cls {
				mark(r.resolve(&p.mod))
			}
		}
	}
	for _, cls := range r.c.classList {
		name := cls.RubyName
		if i := strings.LastIndex(name, "::"); i >= 0 {
			name = name[i+2:]
		}
		if !r.gemClass(cls) || r.named[cls] || r.byName[name] {
			mark(cls)
		}
	}
}

// resolve is the class a superclass/include reference names, or nil when it names none.
func (r *reach) resolve(ref *constRef) (cls *Class) {
	if ref.node == nil {
		return nil
	}
	_ = catchCompileError(func() { cls, _ = r.c.lookupConst(ref.file, ref.node, ref.scope) })
	return cls
}

func (r *reach) add(name string) {
	if r.names[name] {
		return
	}
	if r.why != nil {
		r.why[name] = r.from
	}
	r.names[name] = true
	for _, old := range r.aliases[name] {
		r.add(old)
	}
}

// alias notes `alias nn on`: on is reached once nn is.
func (r *reach) alias(nn, on string) {
	r.aliases[nn] = append(r.aliases[nn], on)
	if r.names[nn] {
		r.add(on)
	}
}

func symbolName(n parser.Node) (string, bool) {
	switch n := n.(type) {
	case *parser.SymbolNode:
		return n.Unescaped.Value, true
	case *parser.StringNode:
		return n.Unescaped.Value, true
	}
	return "", false
}

// scanGo reaches every method a Go call names, so what the runtime calls stays.
func (r *reach) scanGo(code string) {
	goCallNames(code, func(name string) {
		for _, n := range r.goNames[name] {
			r.add(n)
		}
	})
}

// walk adds the method and constant names n's code uses; skipDefs leaves out def bodies, which only a call reaches.
func (r *reach) walk(n parser.Node, skipDefs bool) {
	if n == nil {
		return
	}
	switch n := n.(type) {
	case *parser.DefNode:
		if skipDefs {
			return
		}
	case *parser.ClassNode: // its name and a constant superclass only declare; the body runs
		switch n.Superclass.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode, nil:
		default:
			r.walk(n.Superclass, skipDefs)
		}
		r.walkBody(n.ConstantPath, n.Body, skipDefs)
		return
	case *parser.ModuleNode:
		r.walkBody(n.ConstantPath, n.Body, skipDefs)
		return
	case *parser.ConstantReadNode:
		r.consts[n.Name] = true
		r.nameConst(n, n.Name)
	case *parser.ConstantPathNode: // Rack::Headers names Headers; Rack is only its namespace
		r.consts[*n.Name] = true
		r.nameConst(n, *n.Name)
		return
	case *parser.ClassVariableReadNode, *parser.ClassVariableWriteNode, *parser.ClassVariableOrWriteNode, *parser.ClassVariableOperatorWriteNode:
		r.consts[classVarName(n)] = true // a reached method's @@x keeps the body's assignment (decision 281)
	case *parser.ConstantPathWriteNode: // its target declares
		r.walk(n.Value, skipDefs)
		return
	case *parser.ConstantWriteNode: // Rack::Lint's HOST_PATTERN, read only by constants nothing reads
		if r.c.gemFile(r.file) && !r.consts[n.Name] && effectFree(n.Value) {
			r.pending[n.Name] = append(r.pending[n.Name], pendingConst{value: n.Value, file: r.file, scope: r.scope})
			return
		}
		r.nestedWrites(n.Value) // the write runs: the constants its value assigns are initialized with it (decision 410)
	case *parser.XStringNode:
		r.scanGo(n.Unescaped.Value)
	case *parser.CallNode:
		if cls := r.c.anonClasses[n]; cls != nil && cls.isPerRun {
			r.named[cls] = true // evaluated here: its class object's methods run (decision 259)
		}
		if n.Receiver == nil && len(r.scope) > 0 && slices.ContainsFunc(r.scope[len(r.scope)-1].singletonDefs, func(d singletonDef) bool { return d.node.Name == n.Name }) {
			r.named[r.scope[len(r.scope)-1]] = true // a body's `register :webrick, WEBrick` calls its own class method
		}
		if r.call(n) {
			return
		}
	case *parser.AliasMethodNode:
		nn, ok1 := symbolName(n.NewName)
		on, ok2 := symbolName(n.OldName)
		if ok1 && ok2 {
			r.alias(nn, on)
			return
		}
	default:
		r.writeNames(n)
	}
	for _, ch := range n.CompactChildNodes() {
		r.walk(ch, skipDefs)
	}
}

// walkBody walks a class or module body with its constants resolving inside it.
func (r *reach) walkBody(path parser.Node, body parser.Node, skipDefs bool) {
	prev := r.scope
	var name string
	if r.file != nil && catchCompileError(func() { name = r.c.declName(r.file, path, r.scope) }) == nil && r.c.classes[name] != nil {
		r.scope = append(slices.Clip(r.scope), r.c.classes[name])
	}
	r.walk(body, skipDefs)
	r.scope = prev
}

// nameConst marks the class a constant resolves to in the walk's lexical scope; one that resolves to none here
// (a constant, or one not declared yet) is live by its last segment, as rackup's two Servers would both be.
func (r *reach) nameConst(n parser.Node, name string) {
	var cls *Class
	if r.file != nil {
		cls = r.resolve(&constRef{node: n, scope: r.scope, file: r.file})
	}
	if cls != nil {
		r.named[cls] = true
	} else {
		r.byName[name] = true
	}
}

// nestedWrites reaches the constants a constant's value assigns as it is evaluated (`ORDERINGS = [REQUIRE_ORDER = 0,
// PERMUTE = 1]`, getoptlong): a live value initializes them, whether or not anything names them (decision 410).
func (r *reach) nestedWrites(value parser.Node) {
	var inner []*parser.ConstantWriteNode
	liftConstWrites(value, &inner)
	for _, w := range inner {
		r.consts[w.Name] = true
	}
}

// writeNames reaches the methods an operator-assignment or symbol node calls.
func (r *reach) writeNames(n parser.Node) {
	switch n := n.(type) {
	case *parser.CallOperatorWriteNode:
		r.add(n.ReadName)
		r.add(n.WriteName)
		r.add(n.BinaryOperator)
	case *parser.CallOrWriteNode:
		r.add(n.ReadName)
		r.add(n.WriteName)
	case *parser.CallAndWriteNode:
		r.add(n.ReadName)
		r.add(n.WriteName)
	case *parser.IndexOperatorWriteNode:
		r.add("[]")
		r.add("[]=")
		r.add(n.BinaryOperator)
	case *parser.IndexOrWriteNode, *parser.IndexAndWriteNode, *parser.IndexTargetNode:
		r.add("[]")
		r.add("[]=")
	case *parser.LocalVariableOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.InstanceVariableOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.ClassVariableOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.GlobalVariableOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.ConstantOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.ConstantPathOperatorWriteNode:
		r.add(n.BinaryOperator)
	case *parser.CallTargetNode:
		r.add(n.Name)
	case *parser.SymbolNode:
		r.add(n.Unescaped.Value)
	}
}

// constGet: a computed const_get (rackup's Handler.[]) names classes, not methods, so it makes classes live rather than stop pruning.
func (r *reach) constGet(n *parser.CallNode) {
	prefix := ""
	if _, self := n.Receiver.(*parser.SelfNode); (self || n.Receiver == nil) && len(r.scope) > 0 {
		prefix = r.scope[len(r.scope)-1].RubyName + "::"
	}
	r.reflected = append(r.reflected, prefix)
	for _, cls := range r.c.classList {
		if strings.HasPrefix(cls.RubyName, prefix) {
			r.named[cls] = true
		}
	}
}

// call reaches a call's method; true when its arguments name methods or modules without reaching them.
func (r *reach) call(n *parser.CallNode) bool {
	if lv, ok := n.Receiver.(*parser.LocalVariableReadNode); ok && r.params[lv.Name] != nil {
		r.typed[n.Name] = append(r.typed[n.Name], r.params[lv.Name])
		return false
	}
	r.add(n.Name)
	args := callArgs(n)
	if n.Name == "constants" && len(args) == 0 { // `constants.grep(/V1/).flat_map(&method(:const_get))`: every constant of the module may be named
		r.constGet(n)
	}
	// respond_to_missing? forwarding respond_to?(name) passes on a name some other call already spelled
	if reflectiveCalls[n.Name] && len(args) > 0 && (n.Name != "respond_to?" || r.from != "def respond_to_missing?") {
		if name, ok := symbolName(args[0]); !ok && n.Name == "const_get" {
			r.constGet(n)
		} else if ok {
			r.add(name) // a string literal names the method as a symbol literal does (walk adds the symbol's)
			if n.Name == "const_get" {
				r.consts[name] = true // `self.class.const_get(:END_RE)`: each class's END_RE may be the one
			}
		} else {
			r.dynamic = "a computed " + n.Name + " in " + r.from
		}
	}
	if declarationCalls[n.Name] && n.Receiver == nil {
		return true
	}
	if n.Name == "alias_method" && n.Receiver == nil && len(args) == 2 {
		nn, ok1 := symbolName(args[0])
		on, ok2 := symbolName(args[1])
		if ok1 && ok2 {
			r.alias(nn, on)
			return true
		}
	}
	return false
}

func (r *reach) prune() {
	c := r.c
	keep := func(f *File, name string, owner *Class) bool { return !c.gemFile(f) || r.reached(name, owner) }
	for _, cls := range c.classList {
		var list []*Method
		for _, m := range cls.MethodList {
			if keep(m.File, m.Name, cls) {
				list = append(list, m)
				continue
			}
			cls.prunedDefs = append(cls.prunedDefs, m) // still a method of the class to reflection (method_defined?, instance_methods; decision 444)
			if cls.Methods[m.Name] == m {
				delete(cls.Methods, m.Name)
			}
		}
		cls.MethodList = list
		var defs []singletonDef
		for _, d := range cls.singletonDefs {
			if keep(d.file, d.node.Name, cls) {
				defs = append(defs, d)
			}
		}
		cls.singletonDefs = defs
		cls.delegations = slices.DeleteFunc(cls.delegations, func(d delegation) bool { return !keep(d.file, d.name, cls) })
	}
	// an inherited alias is not a Method until linkAliases: unreached, it would look up an old name pruned above
	c.inheritedAliases = slices.DeleteFunc(c.inheritedAliases, func(a pendingAlias) bool { return !keep(a.f, a.newName, a.cls) })
	r.pruneConsts()
	var tops []*Method
	for _, m := range c.topDefList {
		if keep(m.File, m.Name, nil) {
			tops = append(tops, m)
		} else {
			delete(c.topDefs, m.Name)
		}
	}
	c.topDefList = tops
}

// pruneConsts drops a gem constant no reachable code names whose initializer has no effect to keep (a lambda,
// a proc, a literal): Rack::BUILDER_TOPLEVEL_BINDING's lambda calls `binding`, and only config.ru loading reads it.
func (r *reach) pruneConsts() {
	c := r.c
	var keep []*Const
	for _, k := range c.constList {
		name := k.RubyName[strings.LastIndex(k.RubyName, ":")+1:]
		if !c.gemFile(k.File) || r.consts[name] || !effectFree(k.Value) || k.typeAlias != "" || r.reflectedConst(k.RubyName) { // a Sorbet type alias types sigs and includes, unread by name
			keep = append(keep, k)
			continue
		}
		delete(c.consts, k.RubyName)
		if i := strings.LastIndex(k.RubyName, "::"); i >= 0 {
			if parent := c.classes[k.RubyName[:i]]; parent != nil {
				parent.constNames = slices.DeleteFunc(parent.constNames, func(s string) bool { return s == name })
			}
		} else { // Object's table (constEntries) would read the deleted entry
			c.topConstNames = slices.DeleteFunc(c.topConstNames, func(s string) bool { return s == name })
		}
	}
	c.constList = keep
}

// reflectedConst reports a constant some reachable `constants` or computed `const_get` may name.
func (r *reach) reflectedConst(full string) bool {
	return slices.ContainsFunc(r.reflected, func(prefix string) bool { return strings.HasPrefix(full, prefix) })
}

// effectFree reports whether evaluating n can do nothing but build a value.
func effectFree(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.LambdaNode, *parser.StringNode, *parser.SymbolNode, *parser.IntegerNode, *parser.FloatNode,
		*parser.RegularExpressionNode, *parser.NilNode, *parser.TrueNode, *parser.FalseNode:
		return true
	case *parser.CallNode:
		if n.Name == "freeze" && n.Arguments == nil && n.Block == nil {
			return effectFree(n.Receiver)
		}
		return n.Receiver == nil && n.Arguments == nil && n.Block != nil && (n.Name == "lambda" || n.Name == "proc")
	case *parser.ArrayNode:
		return !slices.ContainsFunc(n.Elements, func(e parser.Node) bool { return !effectFree(e) })
	case *parser.InterpolatedRegularExpressionNode: // `/\A#{HOST_PATTERN}\z/`: a constant's to_s
		return !slices.ContainsFunc(n.Parts, func(p parser.Node) bool {
			es, ok := p.(*parser.EmbeddedStatementsNode)
			if !ok {
				return false
			}
			if es.Statements == nil || len(es.Statements.Body) != 1 {
				return true
			}
			switch es.Statements.Body[0].(type) {
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				return false
			}
			return true
		})
	}
	return false
}

// classVarName is the @@x a class-variable read or write names.
func classVarName(n parser.Node) string {
	switch n := n.(type) {
	case *parser.ClassVariableReadNode:
		return n.Name
	case *parser.ClassVariableWriteNode:
		return n.Name
	case *parser.ClassVariableOrWriteNode:
		return n.Name
	case *parser.ClassVariableOperatorWriteNode:
		return n.Name
	}
	return ""
}
