package compiler

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// MRI builds specs with Class.new and define_method at run time; a closed world compiles them to classes and methods instead (decision 83).

// specForms are the class-body calls a Minitest::Spec descendant may make.
var specForms = map[string]bool{"it": true, "specify": true, "before": true, "after": true, "let": true, "subject": true, "describe": true}

// specUse waits for link: only then is it known whether its class descends from Minitest::Spec.
type specUse struct {
	cls  *Class
	f    *File
	node *parser.CallNode
}

// isDescribe reports whether a top-level statement is a spec's `describe X do ... end`.
func isDescribe(n *parser.CallNode) bool {
	_, ok := n.Block.(*parser.BlockNode)
	return n.Receiver == nil && n.Name == "describe" && ok
}

// collectDescribe declares the class a `describe` stands for: a subclass of
// Minitest::Spec (of the enclosing spec, when nested) named like MRI's,
// whose body is the block's.
func (c *Compiler) collectDescribe(ctx context.Context, f *File, outer *Class, n *parser.CallNode) {
	blk, ok := n.Block.(*parser.BlockNode)
	if !ok || blk.Parameters != nil {
		c.errorf(f, n, "describe needs a block without parameters")
	}
	args := callArgs(n)
	if len(args) == 0 {
		c.errorf(f, n, "describe needs a description")
	}
	parts := make([]string, 0, len(args)+1)
	if outer != nil {
		parts = append(parts, outer.displayName())
	}
	for _, a := range args {
		parts = append(parts, c.specDesc(f, a, "describe"))
	}
	c.specClasses++
	key := fmt.Sprintf("RbSpec%d", c.specClasses) // not a name user code can reach: MRI's class is anonymous
	cls := c.declareHidden(f, key, f.line(n.Location.StartOffset), false)
	cls.Display = strings.Join(parts, "::")
	cls.specChild = outer != nil
	cls.describe = true
	super := parser.Node(&parser.ConstantPathNode{Parent: &parser.ConstantReadNode{Name: "Minitest"}, Name: ptr("Spec")})
	if outer != nil {
		super = &parser.ConstantReadNode{Name: outer.RubyName}
	}
	cls.superRef = &constRef{node: super, file: f}
	c.specUses = append(c.specUses, specUse{cls: cls, f: f, node: n})
	c.collectBody(ctx, f, cls, blk.Body, []*Class{cls}, false)
}

// specDesc is a describe/it description, which must be known at compile
// time: a string or symbol literal, or a constant (its path as written).
func (c *Compiler) specDesc(f *File, a parser.Node, form string) string {
	switch a := a.(type) {
	case *parser.StringNode:
		return a.Unescaped.Value
	case *parser.SymbolNode:
		return a.Unescaped.Value
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return f.text(a.GetLocation())
	}
	c.errorf(f, a, "%s needs a literal description (a string, symbol or constant)", form)
	return ""
}

// collectSpecForm declares the method a DSL call in a spec's class body stands for.
func (c *Compiler) collectSpecForm(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) {
	c.specUses = append(c.specUses, specUse{cls: cls, f: f, node: n})
	if n.Name == "describe" {
		c.collectDescribe(ctx, f, cls, n)
		return
	}
	blk, _ := n.Block.(*parser.BlockNode)
	if n.Block != nil && blk == nil {
		c.errorf(f, n, "%s needs a literal block", n.Name)
	}
	if blk != nil && blk.Parameters != nil {
		c.errorf(f, blk, "%s's block takes no parameters", n.Name)
	}
	args := callArgs(n)
	loc := n.Location
	var body []parser.Node
	var locals []string
	if blk != nil {
		body = stmtsOf(blk.Body)
		locals = blk.Locals
	}
	switch n.Name {
	case "it", "specify":
		desc := "anonymous"
		if len(args) > 0 {
			desc = c.specDesc(f, args[0], n.Name)
		}
		if blk == nil { // MRI: proc { skip "(no tests defined)" }
			body = []parser.Node{&parser.CallNode{Location: loc, Name: "skip", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{
				&parser.StringNode{Location: loc, Unescaped: parser.RubyString{Value: "(no tests defined)"}},
			}}}}
		}
		cls.specTests++
		c.addSpecDef(f, cls, fmt.Sprintf("test_%04d_%s", cls.specTests, desc), body, locals, loc, scope)
	case "before", "after":
		if blk == nil {
			c.errorf(f, n, "%s needs a block", n.Name)
		}
		// MRI: define_method :setup { super(); instance_eval(&block) }, and after the reverse
		sup := parser.Node(&parser.ForwardingSuperNode{Location: loc})
		name := "setup"
		if n.Name == "before" {
			body = append([]parser.Node{sup}, body...)
		} else {
			name = "teardown"
			body = append(body, sup)
		}
		c.addSpecDef(f, cls, name, body, locals, loc, scope)
	case "let", "subject":
		if blk == nil {
			c.errorf(f, n, "%s needs a block", n.Name)
		}
		name := "subject"
		if n.Name == "let" {
			if len(args) != 1 {
				c.errorf(f, n, "let needs one name")
			}
			name = c.specDesc(f, args[0], "let")
		}
		c.declareIvar(cls, "@__let_"+name+"_set", TClass{C: c.classes["Boolean"]}, f, f.line(loc.StartOffset))
		c.addSpecDef(f, cls, name, letBody(name, body, loc), locals, loc, scope)
	}
}

// letBody memoizes behind a flag, not ||=, so nil and false are memoized too, as MRI's @_memoized hash does; the value is written before it is read, which is how ivar discovery types it.
func letBody(name string, body []parser.Node, loc parser.Location) []parser.Node {
	val, set := "@__let_"+name, "@__let_"+name+"_set"
	notSet := &parser.CallNode{Location: loc, Name: "!", Receiver: &parser.InstanceVariableReadNode{Location: loc, Name: set}}
	return []parser.Node{
		&parser.IfNode{Location: loc, Predicate: notSet, Statements: &parser.StatementsNode{Location: loc, Body: []parser.Node{
			&parser.InstanceVariableWriteNode{Location: loc, Name: val, Value: valueOf(body, loc)},
			&parser.InstanceVariableWriteNode{Location: loc, Name: set, Value: &parser.TrueNode{Location: loc}},
		}}},
		&parser.InstanceVariableReadNode{Location: loc, Name: val},
	}
}

// addSpecDef declares a DSL call's method. A later one of the same name
// replaces it, as a later define_method does.
func (c *Compiler) addSpecDef(f *File, cls *Class, name string, body []parser.Node, locals []string, loc parser.Location, scope []*Class) {
	if old := cls.Methods[name]; old != nil {
		delete(cls.Methods, name)
		cls.MethodList = deleteMethod(cls.MethodList, old)
	}
	def := &parser.DefNode{Location: loc, Name: name, NameLoc: loc, DefKeywordLoc: loc, Locals: locals,
		Body: &parser.StatementsNode{Location: loc, Body: body}}
	c.addDef(f, cls, def, false, scope)
	cls.Methods[name].specForm = true
}

// checkSpecUses runs once superclasses are linked: the DSL exists only on
// Minitest::Spec's descendants, and a let may not shadow a test or a
// Minitest::Spec method (MRI raises ArgumentError for both).
func (c *Compiler) checkSpecUses() {
	spec := c.classes["Minitest::Spec"]
	for _, u := range c.specUses {
		if spec == nil || !u.cls.isSubclassOf(spec) {
			c.errorf(u.f, u.node, "%s is Minitest::Spec's DSL: call it in a describe block or a Minitest::Spec subclass", u.node.Name)
		}
		if u.node.Name != "let" {
			continue
		}
		name := c.specDesc(u.f, callArgs(u.node)[0], "let")
		pre, post := "let '"+name+"' cannot ", ". Please use another name."
		if strings.HasPrefix(name, "test") {
			c.errorf(u.f, u.node, "%sbegin with 'test'%s", pre, post)
		}
		if name != "subject" && spec.lookup(name) != nil {
			c.errorf(u.f, u.node, "%soverride a method in Minitest::Spec%s", pre, post)
		}
	}
}

// isLetFlag: a let's memo flag shares the let's line, so a `#:` there types the let's value, not the flag.
func isLetFlag(ivar string) bool {
	return strings.HasPrefix(ivar, "@__let_") && strings.HasSuffix(ivar, "_set")
}

// valueOf is a statement list as one expression: the statement itself, or `begin ... end` around several.
func valueOf(body []parser.Node, loc parser.Location) parser.Node {
	switch len(body) {
	case 0:
		return &parser.NilNode{Location: loc}
	case 1:
		return body[0]
	}
	return &parser.BeginNode{Location: loc, Statements: &parser.StatementsNode{Location: loc, Body: body}}
}

func stmtsOf(n parser.Node) []parser.Node {
	switch b := n.(type) {
	case nil:
		return nil
	case *parser.StatementsNode:
		return b.Body
	}
	return []parser.Node{n}
}

func deleteMethod(ms []*Method, m *Method) []*Method {
	out := ms[:0]
	for _, x := range ms {
		if x != m {
			out = append(out, x)
		}
	}
	return out
}

func ptr[T any](v T) *T { return &v }

// skipFailingTests dry-emits each user test_ method and spec form (before, let), and one that fails to compile gets `skip "rugaroo: <error>"` for a body, so one compile finds every failing test.
func (c *Compiler) skipFailingTests() []SkippedTest {
	var out []SkippedTest
	for _, cls := range c.classList {
		for _, m := range cls.MethodList {
			if m.Kind != kindDef || m.File.isPrelude() || !strings.HasPrefix(m.Name, "test_") && !m.specForm {
				continue
			}
			nw := len(c.Warnings)
			err := catchCompileError(func() {
				if m.inferRet {
					m.Ret = nil
					c.inferRet(m)
				}
				c.emitMethod(m)
			})
			c.out.Reset() // nothing is emitted before emitProgram
			c.dropWarnings(nw)
			if err == nil {
				continue
			}
			if err.nilVeto && c.nilDeferUsed != nil && *c.nilDeferUsed {
				panic(*err) // the recompile without decision 440 decides (compileOnce)
			}
			reason := err.Error()
			if err.pos.File != nil && err.pos.File.Name == m.File.Name {
				reason = err.msg
			}
			out = append(out, SkippedTest{File: m.File.Name, Line: m.Line, Name: m.Name, Reason: reason})
			loc := m.Node.Location
			m.Node = &parser.DefNode{Location: loc, Name: m.Node.Name, NameLoc: loc, DefKeywordLoc: loc, Body: &parser.StatementsNode{Location: loc, Body: []parser.Node{
				&parser.CallNode{Location: loc, Name: "skip", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{
					&parser.StringNode{Location: loc, Unescaped: parser.RubyString{Value: "rugaroo: " + reason}},
				}}},
			}}}
			m.Ret = nil
		}
	}
	return out
}
