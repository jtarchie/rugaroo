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
func lowerLastLine(root parser.Node) {
	prog, ok := root.(*parser.ProgramNode)
	if !ok {
		return
	}
	lowerFrame(reflect.ValueOf(&prog.Statements).Elem(), &prog.Locals)
}

// lowerFrame lowers one frame's body (held in slot) when it uses $_, then each frame nested in it.
func lowerFrame(slot reflect.Value, locals *[]string) {
	body, _ := slot.Interface().(parser.Node)
	if body == nil || reflect.ValueOf(body).IsNil() {
		return
	}
	if usesLastLine(body) {
		walkLastLine(slot, 0)
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
	eachFrame(reflect.ValueOf(body))
}

// eachFrame lowers the frames (defs, class and module bodies) nested in the node v points to.
func eachFrame(v reflect.Value) {
	forNodeChildren(v, func(child reflect.Value) {
		switch x := child.Interface().(type) {
		case *parser.DefNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals)
		case *parser.ClassNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals)
		case *parser.ModuleNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals)
		case *parser.SingletonClassNode:
			lowerFrame(reflect.ValueOf(&x.Body).Elem(), &x.Locals)
		default:
			eachFrame(child)
		}
	})
}

// opensFrame: a def or class body has its own $_.
func opensFrame(n parser.Node) bool {
	switch n.(type) {
	case *parser.DefNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
		return true
	}
	return false
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
	case "print":
		return c.Receiver == nil && c.Arguments == nil && c.Block == nil
	case "~":
		_, re := c.Receiver.(*parser.RegularExpressionNode)
		_, ire := c.Receiver.(*parser.InterpolatedRegularExpressionNode)
		return (re || ire) && c.Arguments == nil
	}
	return false
}

// walkLastLine stops at nested frames; depth counts blocks up to the frame, as a local read's Depth does.
func walkLastLine(slot reflect.Value, depth uint32) {
	n, _ := slot.Interface().(parser.Node)
	if n == nil || reflect.ValueOf(n).IsNil() || opensFrame(n) {
		return
	}
	switch x := n.(type) {
	case *parser.BlockNode, *parser.LambdaNode:
		forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth+1) })
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
			walkLastLine(reflect.ValueOf(&w.Value).Elem(), depth)
		}
		return
	case *parser.CallNode:
		forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth) })
		read := &parser.LocalVariableReadNode{Location: x.Location, Name: lastLineLocal, Depth: depth}
		switch {
		case lastLineCall(x) && x.Name == "print":
			x.Arguments = &parser.ArgumentsNode{Location: x.Location, Arguments: []parser.Node{read}}
		case lastLineCall(x): // `~ /re/` is `$_ ? /re/ =~ $_ : nil`: nil before any line, as MRI's
			x.Name, x.Arguments = "=~", &parser.ArgumentsNode{Location: x.Location, Arguments: []parser.Node{read}}
			l := x.Location
			slot.Set(reflect.ValueOf(parser.Node(&parser.IfNode{Location: l, Predicate: &parser.LocalVariableReadNode{Location: l, Name: lastLineLocal, Depth: depth},
				Statements: &parser.StatementsNode{Location: l, Body: []parser.Node{x}},
				Subsequent: &parser.ElseNode{Location: l, Statements: &parser.StatementsNode{Location: l, Body: []parser.Node{&parser.NilNode{Location: l}}}}})))
		case (x.Name == "gets" || x.Name == "readline") && x.Block == nil:
			slot.Set(reflect.ValueOf(parser.Node(&parser.LocalVariableWriteNode{Location: x.Location, Name: lastLineLocal, NameLoc: x.Location, Value: x, Depth: depth})))
		}
		return
	}
	forNodeChildren(reflect.ValueOf(n), func(child reflect.Value) { walkLastLine(child, depth) })
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
