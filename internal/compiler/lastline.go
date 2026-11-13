package compiler

import (
	"reflect"
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// lastLineLocal is $_ as a per-frame local (decision 446); the `$` keeps any Ruby local name from reaching it.
const lastLineLocal = "$_"

// lastLineNames are $_ and English's name for it.
var lastLineNames = map[string]bool{"$_": true, "$LAST_READ_LINE": true}

// lowerLastLine: MRI keeps $_ per frame, so a frame that uses it gets a local its gets calls assign (decision 446).
func lowerLastLine(root parser.Node) map[*parser.CallNode]bool {
	prog, ok := root.(*parser.ProgramNode)
	if !ok {
		return nil
	}
	prints := map[*parser.CallNode]bool{}
	lowerFrame(reflect.ValueOf(&prog.Statements).Elem(), &prog.Locals, prints)
	return prints
}

// lowerFrame lowers one frame's body (held in slot) when it uses $_, then each frame nested in it.
func lowerFrame(slot reflect.Value, locals *[]string, prints map[*parser.CallNode]bool) {
	body, _ := slot.Interface().(parser.Node)
	if body == nil || reflect.ValueOf(body).IsNil() {
		return
	}
	if usesLastLine(body) {
		walkLastLine(slot, 0, prints)
		loc := body.GetLocation()
		init := &parser.LocalVariableWriteNode{Location: loc, Name: lastLineLocal, NameLoc: loc, Value: &parser.CallNode{Location: loc, Name: "__no_line"}}
		if st, ok := body.(*parser.StatementsNode); ok {
			st.Body = append([]parser.Node{init}, st.Body...)
		} else {
			slot.Set(reflect.ValueOf(parser.Node(&parser.StatementsNode{Location: loc, Body: []parser.Node{init, body}})))
		}
		if !slices.Contains(*locals, lastLineLocal) {
			*locals = append(*locals, lastLineLocal)
		}
	}
	eachFrame(reflect.ValueOf(body), prints)
}

// eachFrame lowers the frames (defs, class and module bodies) nested in the node v points to.
func eachFrame(v reflect.Value, prints map[*parser.CallNode]bool) {
	forNodeChildren(v, func(child reflect.Value) {
		switch x := child.Interface().(type) {
		case *parser.DefNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals, prints)
		case *parser.ClassNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals, prints)
		case *parser.ModuleNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals, prints)
		case *parser.SingletonClassNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals, prints)
		case *parser.CallNode:
			if b := specFrameBlock(x); b != nil {
				lowerFrame(reflect.ValueOf(&b.Body).Elem(), &b.Locals, prints)
				return
			}
			eachFrame(child, prints)
		default:
			eachFrame(child, prints)
		}
	})
}

// opensFrame: a def or class body has its own $_, as has a spec DSL block (spec.go makes it a class body or method).
func opensFrame(n parser.Node) bool {
	switch x := n.(type) {
	case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
		return true
	case *parser.CallNode:
		return specFrameBlock(x) != nil
	}
	return false
}

// ponytail: any receiverless spec-DSL call with a block, not only those inside a describe; track describe nesting if a program's own `it` method takes a block.
var specFrameNames = map[string]bool{"describe": true, "it": true, "specify": true, "before": true, "after": true, "let": true, "subject": true}

// specFrameBlock is a block with a $_ of its own: a spec DSL block, or a new thread's (MRI's $_ is thread-local too).
func specFrameBlock(c *parser.CallNode) *parser.BlockNode {
	b, ok := c.Block.(*parser.BlockNode)
	if !ok {
		return nil
	}
	if c.Receiver == nil && specFrameNames[c.Name] {
		return b
	}
	if r, ok := c.Receiver.(*parser.ConstantReadNode); ok && r.Name == "Thread" && (c.Name == "new" || c.Name == "start" || c.Name == "fork") {
		return b
	}
	return nil
}

// usesLastLine: the frame's own code (not a nested def or class) reads or writes $_.
func usesLastLine(n parser.Node) bool {
	if n == nil || reflect.ValueOf(n).IsNil() || opensFrame(n) {
		return false
	}
	switch x := n.(type) {
	case *parser.GlobalVariableReadNode:
		return lastLineNames[x.Name]
	case *parser.GlobalVariableWriteNode:
		if lastLineNames[x.Name] {
			return true
		}
	case *parser.CallNode:
		if lastLineCall(x) {
			return true
		}
	}
	found := false
	forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) {
		found = found || usesLastLine(child.Interface().(parser.Node))
	})
	return found
}

// lastLineCall is a call that reads $_ implicitly: a bare `print`, `~ /re/`.
func lastLineCall(c *parser.CallNode) bool {
	switch c.Name {
	case "print": // IO#print and StringIO#print with no argument print the caller's $_ as Kernel#print does
		return c.Arguments == nil && c.Block == nil
	case "~":
		_, re := c.Receiver.(*parser.RegularExpressionNode)
		_, ire := c.Receiver.(*parser.InterpolatedRegularExpressionNode)
		return (re || ire) && c.Arguments == nil
	}
	return false
}

// walkLastLine stops at nested frames; depth counts blocks up to the frame, as a local read's Depth does.
func walkLastLine(slot reflect.Value, depth uint32, prints map[*parser.CallNode]bool) {
	n, _ := slot.Interface().(parser.Node)
	if n == nil || reflect.ValueOf(n).IsNil() || opensFrame(n) {
		return
	}
	switch x := n.(type) {
	case *parser.BlockNode, *parser.LambdaNode:
		forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth+1, prints) })
		return
	case *parser.GlobalVariableReadNode:
		if lastLineNames[x.Name] {
			slot.Set(reflect.ValueOf(parser.Node(&parser.LocalVariableReadNode{Location: x.Location, Name: lastLineLocal, Depth: depth})))
		}
		return
	case *parser.GlobalVariableWriteNode:
		if lastLineNames[x.Name] {
			w := &parser.LocalVariableWriteNode{Location: x.Location, Name: lastLineLocal, NameLoc: x.NameLoc, Value: x.Value, OperatorLoc: x.OperatorLoc, Depth: depth}
			slot.Set(reflect.ValueOf(parser.Node(w)))
			walkLastLine(reflect.ValueOf(&w.Value).Elem(), depth, prints)
		}
		return
	case *parser.CallNode:
		forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth, prints) })
		read := &parser.LocalVariableReadNode{Location: x.Location, Name: lastLineLocal, Depth: depth}
		switch {
		case lastLineCall(x) && x.Name == "print": // its argument is added where the call resolves to Kernel#print
			x.Arguments = &parser.ArgumentsNode{Location: x.Location, Arguments: []parser.Node{read}}
			prints[x] = true
		case lastLineCall(x): // `~ /re/` is `$_ ? /re/ =~ $_ : nil`: nil before any line, as MRI's
			x.Name, x.Arguments = "=~", &parser.ArgumentsNode{Location: x.Location, Arguments: []parser.Node{read}}
			l := x.Location
			slot.Set(reflect.ValueOf(parser.Node(&parser.IfNode{Location: l, Predicate: &parser.LocalVariableReadNode{Location: l, Name: lastLineLocal, Depth: depth},
				Statements: &parser.StatementsNode{Location: l, Body: []parser.Node{x}},
				Subsequent: &parser.ElseNode{Location: l, Statements: &parser.StatementsNode{Location: l, Body: []parser.Node{&parser.NilNode{Location: l}}}}})))
		case (x.Name == "gets" || x.Name == "readline") && x.Block == nil:
			slot.Set(reflect.ValueOf(parser.Node(&parser.LocalVariableWriteNode{Location: x.Location, Name: lastLineLocal, NameLoc: x.Location, Value: x, Depth: depth})))
		case sentName(x) != nil: // $_ = __line_or(m, recv.send(m, ...), $_)
			l := x.Location
			args := []parser.Node{sentName(x), x, read}
			call := &parser.CallNode{Location: l, Name: "__line_or", Arguments: &parser.ArgumentsNode{Location: l, Arguments: args}}
			slot.Set(reflect.ValueOf(parser.Node(&parser.LocalVariableWriteNode{Location: l, Name: lastLineLocal, NameLoc: l, Value: call, Depth: depth})))
		}
		return
	}
	forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth, prints) })
}

// sentName is the method name of a `send`/`public_send`/`__send__` whose name reads without side effects, so it can be read twice.
func sentName(c *parser.CallNode) parser.Node {
	if c.Receiver == nil || c.Block != nil || c.Arguments == nil || len(c.Arguments.Arguments) == 0 {
		return nil
	}
	if c.Name != "send" && c.Name != "public_send" && c.Name != "__send__" {
		return nil
	}
	switch a := c.Arguments.Arguments[0].(type) {
	case *parser.SymbolNode, *parser.StringNode, *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode:
		return a
	}
	return nil
}

// forNodeChildren calls fn with a settable slot for each Node child (fields and slice elements) of the struct v points to.
func forNodeChildren(v reflect.Value, fn func(reflect.Value)) {
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	s := v.Elem()
	for i := range s.NumField() {
		fv := s.Field(i)
		if !fv.CanSet() {
			continue
		}
		switch {
		case fv.Type().Implements(nodeIface) || fv.Type() == nodeIface:
			if !fv.IsNil() {
				fn(fv)
			}
		case fv.Kind() == reflect.Slice && (fv.Type().Elem() == nodeIface || fv.Type().Elem().Implements(nodeIface)):
			for j := range fv.Len() {
				if !fv.Index(j).IsNil() {
					fn(fv.Index(j))
				}
			}
		}
	}
}

// lastLinePrint is a bare `print` lowerLastLine gave $_ when it does not reach Kernel#print (a program's own print
// method): the call as written, with no argument (decision 446).
func (f *fctx) lastLinePrint(n *parser.CallNode) *parser.CallNode {
	if !f.f.lastLinePrints[n] {
		return nil
	}
	if n.Receiver != nil {
		if e := f.resolve(f.probeType(n.Receiver, nil), "print"); e != nil && e.M.File != nil && e.M.File.isPrelude() {
			return nil
		}
	} else if d := f.c.topDefs["print"]; d == nil || d.File == nil || d.File.isPrelude() { // a top-level def print is a private method of every object
		if e := f.resolve(f.selfType, "print"); e == nil || e.M.Owner == nil || e.M.Owner.RubyName == "Kernel" {
			return nil
		}
	}
	c := *n
	c.Arguments = nil
	return &c
}
