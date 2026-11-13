package compiler

import (
	"context"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// scanDefBody is scanAnon of a def's body: a class statement there (only ruby/spec's `it` rewrite puts one in a
// def; MRI refuses it in source) is hoisted as one in a block is (decision 451).
func (c *Compiler) scanDefBody(ctx context.Context, f *File, body parser.Node, scope []*Class) {
	c.inBlockScan++
	defer func() { c.inBlockScan-- }()
	c.scanAnon(ctx, f, body, scope)
}

// hoistClass declares a class or module statement written inside a block where the block's constant scope is,
// once, at compile time (decision 451). Its body may only declare, and may not replace a method that exists.
func (c *Compiler) hoistClass(ctx context.Context, f *File, n parser.Node, scope []*Class) {
	if c.hoisted[n] {
		return
	}
	if c.hoisted == nil {
		c.hoisted = map[parser.Node]bool{}
	}
	c.hoisted[n] = true
	var path parser.Node
	var body parser.Node
	switch x := n.(type) {
	case *parser.ClassNode:
		path, body = x.ConstantPath, x.Body
	case *parser.ModuleNode:
		path, body = x.ConstantPath, x.Body
	}
	existing := c.classes[c.declName(f, path, scope)]
	if st, ok := body.(*parser.StatementsNode); ok {
		for _, s := range st.Body {
			c.checkHoistedStmt(f, s, existing)
		}
	} else if body != nil {
		c.errorf(f, body, "a class statement inside a block may only declare (def, attr_*, include, extend, constants, nested classes): its body is hoisted to compile time (decision 451)")
	}
	switch x := n.(type) {
	case *parser.ClassNode:
		c.collectClass(ctx, f, x, scope)
	case *parser.ModuleNode:
		c.collectModule(ctx, f, x, scope)
	}
}

// hoistDecls are the receiverless calls a hoisted class body may make: they declare.
var hoistDecls = map[string]bool{"attr_reader": true, "attr_writer": true, "attr_accessor": true, "include": true, "extend": true,
	"prepend": true, "private": true, "public": true, "protected": true, "module_function": true, "alias_method": true,
	"private_constant": true, "attr": true}

// checkHoistedStmt refuses a hoisted body's statement that runs code, or a def replacing one existing has.
func (c *Compiler) checkHoistedStmt(f *File, s parser.Node, existing *Class) {
	switch x := s.(type) {
	case *parser.DefNode:
		if existing != nil && x.Receiver == nil && existing.Methods[x.Name] != nil && existing.Methods[x.Name].Owner == existing && existing.Methods[x.Name].Node != x {
			c.errorf(f, x, "%s#%s already exists: a class statement inside a block may add methods, not replace them, as rugaroo declares it before the block runs (decision 451)", existing.RubyName, x.Name)
		}
		return
	case *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode, *parser.ConstantWriteNode, *parser.AliasMethodNode:
		return
	case *parser.CallNode:
		if x.Receiver == nil && hoistDecls[x.Name] {
			return
		}
	}
	c.errorf(f, s, "a class statement inside a block may only declare (def, attr_*, include, extend, constants, nested classes): its body is hoisted to compile time (decision 451)")
}

// hoistedBody is a class or module statement's body.
func hoistedBody(n parser.Node) parser.Node {
	switch x := n.(type) {
	case *parser.ClassNode:
		return x.Body
	case *parser.ModuleNode:
		return x.Body
	}
	return nil
}

// lastStmt is a body's last statement, nil when empty.
func lastStmt(body parser.Node) parser.Node {
	if st, ok := body.(*parser.StatementsNode); ok && len(st.Body) > 0 {
		return st.Body[len(st.Body)-1]
	}
	return nil
}
