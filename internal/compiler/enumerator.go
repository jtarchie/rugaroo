package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// yielderClass is Enumerator::Yielder, whose element type Enumerator.new infers from what its block feeds (decision 140).
const yielderClass = "Enumerator::Yielder"

// inferYielder binds X of a block taking a Yielder[X] (`Enumerator.new { |y| y << 1 }`) from the annotation, else from what a probe of the block feeds y.
func (f *fctx) inferYielder(n parser.Node, m *Method, env map[string]Type, block parser.Node) {
	if m.Block == nil || len(m.Block.Params) != 1 {
		return
	}
	yt, ok := m.Block.Params[0].(TClass)
	if !ok || yt.C.RubyName != yielderClass || len(yt.Args) != 1 {
		return
	}
	v, ok := yt.Args[0].(TVar)
	if !ok {
		return
	}
	if _, bound := env[v.Name]; bound {
		return
	}
	if n == f.retHintNode && f.retHint != nil && unify(m.Ret, f.retHint, env) {
		if _, bound := env[v.Name]; bound {
			return
		}
	}
	var fed []Type
	saved := f.yielderFed
	f.yielderFed = &fed
	probeSig := &BlockSig{Params: []Type{TClass{C: yt.C, Args: []Type{TAny{}}}}, Ret: TVoid{}}
	f.probe(func() { f.genClosure(n, block, probeSig, env) })
	f.yielderFed = saved
	if len(fed) == 0 {
		f.errorf(n, "cannot infer the element type of %s: the block feeds its yielder nothing (`y << value`); annotate the result (`#: Enumerator[Integer]`)", m.Name)
	}
	env[v.Name] = f.joinAll(n, fed)
}

// isProbedYielder is a receiver of the Yielder[untyped] inferYielder probes with.
func (f *fctx) isProbedYielder(t Type) bool {
	tc, ok := t.(TClass)
	return ok && f.yielderFed != nil && tc.C.RubyName == yielderClass && len(tc.Args) == 1 && isAny(tc.Args[0])
}

// noteYielderFeed records the type of what `y << v` (or y.yield v) feeds the probed yielder.
func (f *fctx) noteYielderFeed(recv expr, name string, args []parser.Node) {
	if !f.isProbedYielder(recv.typ) || len(args) != 1 || (name != "<<" && name != "yield") {
		return
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	*f.yielderFed = append(*f.yielderFed, a.typ)
}

// yielderBlock is `&y` for a Yielder y: the block `{ |x_0| y << x_0 }`.
func yielderBlock(ba *parser.BlockArgumentNode) *parser.BlockNode {
	loc := ba.Location
	arg := &parser.LocalVariableReadNode{Location: loc, Name: "x_0"}
	call := &parser.CallNode{Location: loc, Receiver: ba.Expression, Name: "<<", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{arg}}}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     []string{"x_0"},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: "x_0"}}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// iterEnum is a blockless call of a prelude iterator with no `__<name>_enum` (`(1..3).each`): an Enumerator over its sequence (a user's each without a block is MRI's LocalJumpError).
func (f *fctx) iterEnum(n parser.Node, e *entry, recv expr, codes []string, env map[string]Type) (expr, bool) {
	m := e.M
	rt, ok := recv.typ.(TClass)
	if !ok || rt.C.IsModule || !m.File.isPrelude() || len(m.Block.Params) == 0 || len(codes) != len(m.Params) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword }) {
		return expr{}, false
	}
	enum := f.c.classes["Enumerator"]
	f.bindTypeParams(n, m, env)
	params := substAll(m.Block.Params, env)
	elem := params[0]
	if len(params) == 2 {
		elem = TTuple{Elems: params}
	}
	et := TClass{C: enum, Args: []Type{elem}}
	ps := []string{"r_ " + f.c.goType(recv.typ)}
	names := make([]string, len(codes))
	inspected := []string{}
	for i := range codes {
		names[i] = "a" + strconv.Itoa(i) + "_"
		ps = append(ps, names[i]+" "+f.c.goType(subst(m.Params[i].Type, env)))
		inspected = append(inspected, "string(rbInspect("+names[i]+"))")
	}
	seq := f.callCode(e, expr{code: "r_", typ: recv.typ}, names, env)
	if len(params) == 2 {
		seq = "rbPairSeq(" + seq + ")"
	}
	meth := strconv.Quote(m.Name)
	if len(codes) > 0 {
		meth += ` + "(" + ` + strings.Join(inspected, ` + ", " + `) + ` + ")"`
	}
	code := fmt.Sprintf("func(%s) %s { return rbEnumOf(%s, any(r_), %s, nil, nil) }(%s)",
		strings.Join(ps, ", "), f.c.goType(et), seq, meth, strings.Join(append([]string{f.materialize(recv)}, codes...), ", "))
	return expr{code: code, typ: et}, true
}

// enumerable is m's block an Enumerator can feed: one closure block of 1-4 values (a Tuple for several).
func enumerable(m *Method) bool {
	return m != nil && m.Block != nil && !m.Iterator && len(m.Block.Params) > 0 && len(m.Block.Params) <= 4 &&
		(len(m.Block.Params) == 1 || !m.Block.Rest) && // callee-side defaults too: enumArgs passes their hidden counts (decision 444)
		(!m.calleeDefaults && !m.kwMask() || m.Block.Ret == nil || isVoid(m.Block.Ret)) // a Go range loop gives yield no value back: matrix's build keeps to_enum's run-time dispatch (decision 460)
}

// genToEnum is `to_enum`/`enum_for` (bare, or naming the enclosing method) in a method taking a closure block: an
// Enumerator that calls the method again with a block feeding it (rbSeq stops it early).
func (f *fctx) genToEnum(n *parser.CallNode) (expr, bool) {
	m := f.m
	if n.Name != "to_enum" && n.Name != "enum_for" || n.Receiver != nil && !isSelf(n.Receiver) || n.Block != nil ||
		!enumerable(m) || !inInterface(m) && !f.lenient() {
		return expr{}, false
	}
	args := callArgs(n)
	name := m.Name
	if len(args) > 0 {
		name = enumName(args[0], m)
		if !f.namesMethod(name, m) || len(args)-1 > len(m.Params) && !slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest }) {
			return expr{}, false
		}
		args = args[1:]
	}
	as, rest, ok := f.enumArgs(n, m, args)
	if !ok {
		return expr{}, false
	}
	et := m.Block.Params[0]
	if len(m.Block.Params) > 1 { // `yield e, row, col` (matrix's each_with_index): each value is one Array, as MRI's Enumerator packs them (decision 414)
		et = TTuple{Elems: m.Block.Params}
	}
	elem := f.c.goType(et)
	as = append(as, f.enumBlock(m, elem))
	if rest != "" {
		as = append(as, rest)
	}
	call := f.selfCode + "." + m.GoName + "(" + strings.Join(as, ", ") + ")"
	if m.Owner != nil && (m.Owner.IsModule || f.lenient()) && !f.c.isDirectMethod(m) { // the free func: a module's constraint, or a gem class's pruned interface, may not name it (elftools' each_symbol)
		targs := "[Self]"
		if f.c.plainSelf(m) {
			targs = ""
		}
		call = freeFuncName(m) + targs + "(" + f.selfCode + comma(strings.Join(as, ", ")) + ")"
	}
	code := fmt.Sprintf("rbEnumOf(rbSeq(func(y func(%s)) { %s }), any(%s), %q, nil, nil)", elem, call, f.selfCode, name)
	return expr{code: code, typ: TClass{C: f.c.classes["Enumerator"], Args: []Type{et}}}, true
}

// namesMethod reports name being m or another name for it (matrix's `alias_method :map, :collect`, `index`'s
// `to_enum :find_index`, decision 414).
func (f *fctx) namesMethod(name string, m *Method) bool {
	if name == m.Name {
		return true
	}
	if name == "" || f.owner == nil || m.Node == nil {
		return false
	}
	e := f.owner.lookup(name)
	return e != nil && e.M.Node == m.Node
}

// enumArgs are the arguments to_enum passes m again: each param's, a default for one it leaves out, and a trailing
// rest param's as Go variadic arguments after the block (index's `to_enum :find_index, which, *args`).
func (f *fctx) enumArgs(n parser.Node, m *Method, args []parser.Node) ([]string, string, bool) {
	var as []string
	bare := len(args) == 0
	kw := map[string]parser.Node{} // `enum_for(__method__, named: named)`: each keyword to its parameter (decision 444)
	if len(args) > 0 {
		if h, ok := args[len(args)-1].(*parser.KeywordHashNode); ok {
			for _, el := range h.Elements {
				a, ok := el.(*parser.AssocNode)
				sym, isSym := (*parser.SymbolNode)(nil), false
				if ok {
					sym, isSym = a.Key.(*parser.SymbolNode)
				}
				if !isSym {
					return nil, "", false
				}
				kw[sym.Unescaped.Value] = a.Value
			}
			args = args[:len(args)-1]
		}
	}
	// the hidden counts a callee-side default reads (sig): the method's own for a bare to_enum, else every one given,
	// since the defaults of what is left out are passed below
	if m.calleeDefaults {
		if bare {
			as = append(as, "rbArgc")
		} else {
			as = append(as, strconv.Itoa(len(m.Params)))
		}
	}
	if m.kwMask() {
		if bare {
			as = append(as, "rbKw")
		} else {
			as = append(as, "-1")
		}
	}
	for i, p := range m.Params {
		switch {
		case p.Keyword && !p.KwRest && !bare:
			switch v := kw[p.Name]; {
			case v != nil:
				as = append(as, f.coerce(n, f.genExpr(v, p.Type), p.Type))
			case p.Default != nil:
				as = append(as, f.coerce(n, f.genExpr(p.Default, p.Type), p.Type))
			default:
				return nil, "", false
			}
		case p.KwRest && !bare:
			return nil, "", false
		case p.Rest:
			if i != len(m.Params)-1 || p.Keyword {
				return nil, "", false
			}
			r, ok := f.enumRest(n, p, args[min(i, len(args)):], len(args) == 0)
			return as, r, ok
		case i < len(args):
			if isSplat(args[i]) {
				return nil, "", false
			}
			as = append(as, f.coerce(n, f.genExpr(args[i], p.Type), p.Type))
		case len(args) > 0 && p.Default != nil: // `to_enum(:each)` leaves the rest at their defaults
			as = append(as, f.coerce(n, f.genExpr(p.Default, p.Type), p.Type))
		default:
			as = append(as, goLocalName(p.Name))
		}
	}
	return as, "", true
}

// enumRest is a rest param's variadic arguments: the method's own (`to_enum` naming no arguments), or the given ones,
// a splat spread (into a Go any only).
func (f *fctx) enumRest(n parser.Node, p Param, args []parser.Node, own bool) (string, bool) {
	if own {
		return goLocalName(p.Name) + "...", true
	}
	if len(args) == 0 {
		return "", true
	}
	if sp, ok := args[0].(*parser.SplatNode); ok && len(args) == 1 && sp.Expression != nil && f.c.goType(p.Type) != "any" {
		// `to_enum(__method__, *types)` passing a typed rest on (rubocop-ast's each_ancestor): the Array's own elements
		// (decision 444)
		if e := f.genExpr(sp.Expression, nil); isClass(e.typ, "Array") && typeEq(e.typ.(TClass).Args[0], p.Type) {
			return "(" + f.materialize(e) + ").s...", true
		}
		return "", false
	}
	var lits, parts []string
	for _, a := range args {
		if sp, ok := a.(*parser.SplatNode); ok {
			if f.c.goType(p.Type) != "any" || sp.Expression == nil { // a union's Go type is any too (`Integer | Symbol`)
				return "", false
			}
			if len(lits) > 0 {
				parts = append(parts, "[]any{"+strings.Join(lits, ", ")+"}")
				lits = nil
			}
			parts = append(parts, "rbSplatObj("+f.coerce(sp, f.genExpr(sp.Expression, nil), TAny{})+")")
			continue
		}
		lits = append(lits, f.coerce(n, f.genExpr(a, p.Type), p.Type))
	}
	if len(parts) == 0 {
		return strings.Join(lits, ", "), true
	}
	if len(lits) > 0 {
		parts = append(parts, "[]any{"+strings.Join(lits, ", ")+"}")
	}
	return "slices.Concat(" + strings.Join(parts, ", ") + ")...", true
}

// genRecvToEnum is `x.to_enum(:each, :diagonal)` where `x.each(:diagonal)` without a block is already an Enumerator
// (matrix's `return to_enum :each, which unless block_given?`): that call, which defers the iteration as MRI's to_enum
// does (decision 414). Any other method keeps to_enum's ordinary dispatch.
func (f *fctx) genRecvToEnum(n *parser.CallNode, expected Type) (expr, bool) {
	args := callArgs(n)
	if n.Name != "to_enum" && n.Name != "enum_for" || n.Receiver == nil || isSelf(n.Receiver) || n.Block != nil || len(args) == 0 {
		return expr{}, false
	}
	sym, ok := args[0].(*parser.SymbolNode)
	if !ok || sym.Unescaped.Value == "to_enum" || sym.Unescaped.Value == "enum_for" {
		return expr{}, false
	}
	call := &parser.CallNode{Location: n.Location, Receiver: n.Receiver, Name: sym.Unescaped.Value, MessageLoc: n.MessageLoc}
	if len(args) > 1 {
		call.Arguments = &parser.ArgumentsNode{Location: n.Arguments.Location, Arguments: args[1:]}
	}
	var recv Type
	if catchCompileError(func() { f.probe(func() { recv = f.genExpr(n.Receiver, nil).typ }) }) != nil {
		return expr{}, false
	}
	rc, ok := stripOpt(recv).(TClass)
	if !ok {
		return expr{}, false
	}
	e := rc.C.lookup(call.Name)
	if e == nil || e.M.Node == nil || e.M.Node.Body == nil || !enumsItself(e.M) {
		return expr{}, false
	}
	return f.genExpr(call, expected), true
}

// enumsItself reports m's body answering an Enumerator of m itself without a block (`return to_enum(:each) unless block_given?`).
func enumsItself(m *Method) bool {
	return anyNode(m.Node.Body, func(x parser.Node) bool {
		c, ok := x.(*parser.CallNode)
		return ok && (c.Name == "to_enum" || c.Name == "enum_for") && (c.Receiver == nil || isSelf(c.Receiver)) && enumName(firstArg(callArgs(c)), m) == m.Name
	})
}

// eachReturnsSelf are the iterators whose value in MRI is their receiver.
var eachReturnsSelf = map[string]bool{"each": true, "each_with_index": true, "each_pair": true, "each_key": true, "each_value": true, "reverse_each": true,
	"each_line": true, "each_char": true, "each_byte": true, "times": true}

// enumName is the method a to_enum names: a Symbol literal, or `__method__` for m itself (patchelf's
// `to_enum(__method__, offset)`); "" otherwise.
func enumName(arg parser.Node, m *Method) string {
	switch a := arg.(type) {
	case *parser.SymbolNode:
		return a.Unescaped.Value
	case *parser.CallNode:
		if a.Name == "__method__" && a.Receiver == nil && a.Arguments == nil {
			return m.Name
		}
	}
	return ""
}

// enumBlock is the block to_enum hands m: each value it yields goes to the Enumerator's y.
func (f *fctx) enumBlock(m *Method, elem string) string {
	if n := len(m.Block.Params); n > 1 {
		ps, fs := make([]string, n), make([]string, n)
		for i, p := range m.Block.Params {
			ps[i] = fmt.Sprintf("x%d %s", i, f.c.goType(p))
			fs[i] = fmt.Sprintf("F%d: x%d", i, i)
		}
		feed := fmt.Sprintf("y(%s{%s})", elem, strings.Join(fs, ", "))
		if isVoid(m.Block.Ret) {
			return fmt.Sprintf("func(%s) { %s }", strings.Join(ps, ", "), feed)
		}
		r := f.c.goType(m.Block.Ret)
		return fmt.Sprintf("func(%s) %s { %s; return %s }", strings.Join(ps, ", "), r, feed, goZero(r))
	}
	if isVoid(m.Block.Ret) {
		return fmt.Sprintf("func(x %s) { y(x) }", elem)
	}
	r := f.c.goType(m.Block.Ret)
	if m.Block.Rest { // `def each_symbol(&block)` of a gem: the block is variadic
		pack := "x[0]"
		if elem == "any" {
			pack = "rbPackYield(x)"
		}
		return fmt.Sprintf("func(x ...%s) %s { y(%s); return %s }", elem, r, pack, goZero(r))
	}
	return fmt.Sprintf("func(x %s) %s { y(x); return %s }", elem, r, goZero(r))
}
