package compiler

import (
	"context"
	"fmt"
	"hash/fnv"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/boot"
)

// MRI's Class.new/Module.new make a class at run time; a closed world
// declares one per literal at compile time, as describe does (decision 145).

// anonClassCall recognizes `Class.new(Super?) { ... }` and `Module.new { ... }`.
func anonClassCall(n parser.Node) (*parser.CallNode, bool) {
	call, ok := n.(*parser.CallNode)
	if !ok || call.Name != "new" {
		return nil, false
	}
	r, ok := call.Receiver.(*parser.ConstantReadNode)
	if !ok || r.Name != "Class" && r.Name != "Module" {
		return nil, false
	}
	return call, r.Name == "Module"
}

// collectNamedAnon declares `Name = Class.new(Super) do ... end` as
// `class Name < Super`: the constant names the class, as in MRI.
func (c *Compiler) collectNamedAnon(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, isModule bool, scope []*Class) {
	cls := c.declareAnon(ctx, f, call, isModule, qualify(scope, n.Name), scope, false)
	if !isModule && !f.isPrelude() {
		c.hooks = append(c.hooks, classHook{name: "inherited", cls: cls, node: n, file: f})
	}
}

// scanAnon declares a hidden class for each Class.new/Module.new inside n
// (a method body, a block, a top-level statement). Class and module
// statements are collected where they stand; an anonymous class's block is
// its body, collected by declareAnon.
func (c *Compiler) scanAnon(ctx context.Context, f *File, n parser.Node, scope []*Class) {
	if n == nil || f.isPrelude() {
		return
	}
	switch n := n.(type) {
	case *parser.ClassNode, *parser.ModuleNode:
		if c.inBlockScan > 0 && !c.gemFile(f) {
			c.hoistClass(ctx, f, n, scope)
		}
		return
	case *parser.BlockNode, *parser.LambdaNode:
		c.inBlockScan++
		defer func() { c.inBlockScan-- }()
	case *parser.CallNode:
		if call, kind := valueClass(n); call != nil && len(callArgs(call)) > 0 && c.anonClasses[call] == nil {
			name := c.anonName(f, call)
			// `Struct.new(:a)` anywhere but a constant assignment: a hidden value class per literal, as Class.new's
			c.anonDepth++
			err := catchCompileError(func() {
				c.collectValueClassAs(ctx, f, &parser.ConstantWriteNode{Name: name, Location: call.Location}, call, kind, scope, true)
				c.anonClasses[call] = c.classes[qualify(scope, name)]
			})
			c.anonDepth--
			if err != nil {
				if c.anonErrors == nil {
					c.anonErrors = map[*parser.CallNode]compileError{}
				}
				c.anonErrors[call] = *err
			}
			return
		}
		if call, isModule := anonClassCall(n); call != nil {
			c.scanAnonClass(ctx, f, call, isModule, scope)
			return
		}
	}
	for _, ch := range n.CompactChildNodes() {
		c.scanAnon(ctx, f, ch, scope)
	}
}

// scanAnonClass declares the hidden class of one Class.new/Module.new literal, once.
func (c *Compiler) scanAnonClass(ctx context.Context, f *File, call *parser.CallNode, isModule bool, scope []*Class) {
	if _, failed := c.anonErrors[call]; c.anonClasses[call] != nil || failed {
		return // visited before: a second class for one literal would carry another name
	}
	name := c.anonName(f, call)
	// an anonymous class's body fails where the literal stands (a test's
	// skip, decision 127), not for the whole program: the error waits there
	c.anonDepth++
	err := catchCompileError(func() { c.declareAnon(ctx, f, call, isModule, name, scope, true) })
	c.anonDepth--
	if err != nil {
		if c.anonErrors == nil {
			c.anonErrors = map[*parser.CallNode]compileError{}
		}
		c.anonErrors[call] = *err
	}
	cls := c.anonClasses[call] // not c.classes[name]: that may be a user's class the error was about
	if cls == nil {
		return
	}
	// MRI's anonymous class prints its address; one class per literal has none, so its place stands in
	cls.Display = fmt.Sprintf("#<%s:%s:%d>", map[bool]string{true: "Module", false: "Class"}[isModule], f.Name, f.line(call.Location.StartOffset))
	if !isModule {
		if c.unnamedAnon == nil {
			c.unnamedAnon = map[*parser.CallNode]bool{}
		}
		c.unnamedAnon[call] = true
	}
}

// anonName names a literal's hidden class by its file and offset, not by a counter: decision 146's rounds carry a
// parameter's type from one compile to the next by class name, and a counter renumbers every later literal when an
// earlier one is added or dropped (ruby/spec core/kernel: `extended(o)` typed singleton(RbAnon18) met RbAnon17).
func (c *Compiler) anonName(f *File, call *parser.CallNode) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(f.Name))
	base := fmt.Sprintf("RbAnon%08x_%d", h.Sum32(), call.Location.StartOffset)
	name := base
	for i := 2; c.classes[name] != nil; i++ { // one literal seen in two parses of its file
		name = fmt.Sprintf("%s_%d", base, i)
	}
	return name
}

func (c *Compiler) declareAnon(ctx context.Context, f *File, call *parser.CallNode, isModule bool, name string, scope []*Class, hidden bool) *Class {
	line := f.line(call.Location.StartOffset)
	args := callArgs(call)
	if isModule && len(args) > 0 || len(args) > 1 {
		c.errorf(f, call, "wrong number of arguments (given %d, expected 0%s)", len(args), map[bool]string{true: "", false: "..1"}[isModule])
	}
	if c.classes[name] != nil {
		c.errorf(f, call, "%s is already defined", name)
	}
	declare := c.declareClass
	if hidden {
		declare = c.declareHidden
	}
	cls := declare(f, name, line, isModule)
	c.anonClasses[call] = cls
	if len(args) == 1 {
		sup := args[0]
		switch a := sup.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
		case *parser.ClassVariableReadNode: // `Class.new(@@global_strainer)`: the class that cvar's own Class.new literal made (decision 331)
			hidden := c.cvarAnonClass(a.Name, scope)
			if hidden == nil {
				c.errorf(f, a, "Class.new's superclass %s must hold a Class.new literal of its class body (decision 331)", a.Name)
			}
			sup = &parser.ConstantReadNode{Name: hidden.Name, Location: a.Location}
		default:
			c.errorf(f, args[0], "Class.new's superclass must be a constant (decision 145)")
		}
		cls.superRef = &constRef{node: sup, scope: scope, file: f}
	}
	if call.Block == nil {
		return cls
	}
	blk, ok := call.Block.(*parser.BlockNode)
	if !ok || blk.Parameters != nil {
		c.errorf(f, call, "%s.new needs a block without parameters (decision 145)", map[bool]string{true: "Module", false: "Class"}[isModule])
	}
	// a block opens no constant scope in Ruby: the body's constants are the enclosing scope's
	var run []parser.Node
	saved := c.anonRun
	if !isModule {
		c.anonRun = &run
	}
	defer func() { c.anonRun = saved }()
	c.collectBody(ctx, f, cls, blk.Body, scope, false)
	if len(run) > 0 {
		c.declareAnonRun(f, call, cls, run, scope)
	}
	return cls
}

// declareAnonRun makes a Class.new block's statements (`@x = v`, `desc st.desc`, ...) the class's singleton
// `__rugaroo_body`, its parameters the enclosing method's locals they read: each evaluation of the literal makes a
// fresh class object and calls it there, so each class object holds its own class-level state (decision 259).
func (c *Compiler) declareAnonRun(f *File, call *parser.CallNode, cls *Class, run []parser.Node, scope []*Class) {
	caps := []string{}
	var walk func(n parser.Node, nest uint32)
	walk = func(n parser.Node, nest uint32) {
		switch n := n.(type) {
		case nil, *parser.DefNode:
			return
		case *parser.LocalVariableReadNode:
			if n.Depth > nest && !slices.Contains(caps, n.Name) {
				caps = append(caps, n.Name)
			}
		case *parser.LocalVariableWriteNode:
			if n.Depth > nest {
				c.errorf(f, n, "a Class.new block assigns the enclosing local %s (decision 259)", n.Name)
			}
		case *parser.LocalVariableOperatorWriteNode, *parser.LocalVariableOrWriteNode, *parser.LocalVariableAndWriteNode:
			c.errorf(f, n, "a Class.new block assigns an enclosing local (decision 259)")
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
	for _, n := range run {
		walk(n, 0)
	}
	ps := &parser.ParametersNode{Location: call.Location}
	for _, name := range caps {
		ps.Requireds = append(ps.Requireds, &parser.RequiredParameterNode{Name: name, Location: call.Location})
	}
	if len(caps) == 0 {
		ps = nil
	}
	def := &parser.DefNode{Name: anonBodyName, Receiver: &parser.SelfNode{Location: call.Location}, Parameters: ps,
		Body: &parser.StatementsNode{Body: run, Location: call.Location}, Location: call.Location, NameLoc: call.Location}
	cls.singletonDefs = append(cls.singletonDefs, singletonDef{node: def, scope: scope, file: f})
	cls.perRun, cls.isPerRun = caps, true
	if !slices.Contains(c.keepNames, anonBodyName) { // the literal calls it; its statements are walked where they stand (decision 169)
		c.keepNames = append(c.keepNames, anonBodyName)
	}
}

// anonBodyName is a per-run Class.new block's statements as a singleton method (decision 259); not a Ruby identifier
// a program can call by accident.
const anonBodyName = "rugaroo_class_body"

// anonDefineMethod is `define_method(:name) do ... end` in a per-run Class.new block: a def of the hidden class at
// compile time when its block reads no enclosing local, the call itself a no-op (decisions 198, 259).
func (c *Compiler) anonDefineMethod(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) bool {
	if c.anonRun == nil || n.Name != "define_method" || n.Receiver != nil {
		return false
	}
	blk, ok := n.Block.(*parser.BlockNode)
	args := callArgs(n)
	if !ok || len(args) != 1 {
		return false
	}
	name, ok := symbolName(args[0])
	if !ok {
		return false
	}
	site := &dmSite{file: f, call: n, block: blk, scope: scope}
	if caps := c.dmCaptures(site); len(caps) > 0 {
		c.errorf(f, n, "define_method %s in a Class.new block reads the enclosing local %s; store it on the class (`@x = %s`) and read that (decision 259)", name, caps[0], caps[0])
	}
	src := c.defineMethodSource(site, boot.MethodInfo{Name: name})
	sf, def := c.synthDef(ctx, f, n, "define_method "+name, src, synthOpt{Origin: originSource, Pin: true})
	def.Name = name
	c.addDef(sf, cls, def, false, scope)
	if c.dmSites == nil {
		c.dmSites = map[*parser.CallNode]bool{}
	}
	c.dmSites[n] = true
	return true
}

// cvarAnonClass is the hidden class of the Class.new literal a class body assigned to class variable name, in the
// innermost class of scope; nil when it holds anything else.
func (c *Compiler) cvarAnonClass(name string, scope []*Class) *Class {
	if len(scope) == 0 {
		return nil
	}
	k := scope[len(scope)-1].cvars[name]
	if k == nil {
		return nil
	}
	call, isModule := anonClassCall(k.Value)
	if call == nil || isModule {
		return nil
	}
	return c.anonClasses[call]
}
