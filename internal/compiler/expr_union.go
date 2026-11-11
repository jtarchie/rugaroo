package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ---- calls on nilable, tuple and untyped receivers

func (f *fctx) optCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch name {
	case "nil?":
		return expr{code: "Boolean(" + optIsNil(recv.code, recv.typ) + ")", typ: f.cls("Boolean")}
	case "to_s":
		return expr{code: "rbToS(" + f.c.optAny(recv.code, recv.typ) + ")", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(" + f.c.optAny(recv.code, recv.typ) + ")", typ: f.cls("String")}
	case "to_json":
		return expr{code: "rbToJson(" + strings.Join(append([]string{f.c.optAny(recv.code, recv.typ)}, f.jsonArgs(args)...), ", ") + ")", typ: f.cls("String")}
	case "==", "equal?":
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		a := f.genExpr(args[0], nil)
		if name == "==" && f.plainOptEq(recv.typ, a.typ) { // `flags[i] == 1`: no boxing into rbEq (decision 387)
			if valOpt(recv.typ) {
				return expr{code: "Boolean(rbOptEqV(" + recv.code + ", " + a.code + "))", typ: f.cls("Boolean")}
			}
			return expr{code: "Boolean(rbOptEq(" + recv.code + ", " + a.code + "))", typ: f.cls("Boolean")}
		}
		arg := f.coerce(args[0], a, TAny{})
		if name == "equal?" {
			return expr{code: "Boolean(rbIdentical(" + f.c.optAny(recv.code, recv.typ) + ", " + arg + "))", typ: f.cls("Boolean")}
		}
		return expr{code: "rbEq[any](" + f.c.optAny(recv.code, recv.typ) + ", " + arg + ")", typ: f.cls("Boolean")}
	case "!":
		if isClass(recv.typ.(TOpt).Elem, "Boolean") {
			return expr{code: "Boolean(!" + optTruthy(recv.code, recv.typ) + ")", typ: f.cls("Boolean")}
		}
		return expr{code: "Boolean(" + optIsNil(recv.code, recv.typ) + ")", typ: f.cls("Boolean")}
	case "to_i", "to_f", "to_a", "to_h", "=~":
		if e, ok := f.nilClassCall(n, recv, name, args); ok {
			return e
		}
	case "!~": // Kernel#!~ is !(x =~ y), so nil !~ y is true (kramdown's `@src.pre_match[-1, 1] !~ /\s/` at the start)
		if e, ok := f.nilClassCall(n, recv, "=~", args); ok {
			return expr{code: "Boolean(!(" + optTruthy(e.code, e.typ) + "))", typ: f.cls("Boolean")}
		}
	case "freeze", "frozen?", "dup", "clone": // nil's own: frozen already, its dup/clone itself (Homebrew's Resource#freeze, Downloadable#initialize_dup)
		if len(args) == 0 && block == nil {
			return f.nilFreeze(n, recv, name)
		}
	}
	if e, ok := f.nilClassArm(n, recv, name, args, block); ok {
		return e
	}
	return f.genMethodCall(n, f.nilGuard(n, recv, name), name, args, block)
}

// nilClassArm is a call on a T? receiver whose method NilClass defines too (a program's own `class NilClass; def m`):
// nil takes NilClass's, a value T's, as Ruby dispatches on the object (decision 186).
func (f *fctx) nilClassArm(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	nc := f.c.classes["NilClass"]
	if nc == nil || nc.lookup(name) == nil || nc.lookup(name).M.File == nil || nc.lookup(name).M.File.isPrelude() {
		return expr{}, false
	}
	elem := recv.typ.(TOpt).Elem
	rt := f.newTmp()
	f.emit("var %s %s = %s", rt, f.c.goType(recv.typ), recv.code) // typed: `$!` outside a rescue is the literal nil
	nt := f.cls("NilClass")
	nilRecv, valRecv := expr{code: "(*" + nc.Name + ")(nil)", typ: nt}, expr{code: derefOpt(rt, recv.typ), typ: elem} // a typed nil: its methods still run
	var nilE, valE expr
	f.probe(func() {
		nilE = f.genMethodCall(n, nilRecv, name, args, block)
		valE = f.genMethodCall(n, valRecv, name, args, block)
	})
	if isClass(stripOpt(nilE.typ), "NilClass") { // `nil.presence` returning self: nil, not the NilClass receiver (decision 252)
		nilE.typ = TNil{}
	}
	void := isVoid(nilE.typ) && isVoid(valE.typ)
	t, ok := join(nilE.typ, valE.typ)
	if !ok {
		t = TAny{}
	}
	out := ""
	if !void {
		out = f.newTmp()
		f.emit("var %s %s", out, f.c.goType(t))
	}
	arm := func(r expr) {
		f.indent++
		e := f.genMethodCall(n, r, name, args, block)
		switch {
		case void:
			f.emitExprStmt(n, e)
		case isClass(stripOpt(e.typ), "NilClass"):
			f.emitExprStmt(n, e)
			f.emit("%s = %s", out, f.coerce(n, expr{code: "nil", typ: TNil{}}, t))
		default:
			f.emit("%s = %s", out, f.coerce(n, e, t))
		}
		f.indent--
	}
	f.emit("if %s {", optIsNil(rt, recv.typ))
	arm(nilRecv)
	f.emit("} else {")
	arm(valRecv)
	f.emit("}")
	if void {
		return expr{code: "", typ: TVoid{}, stmt: true}, true
	}
	return expr{code: out, typ: t}, true
}

// nilAnswers reports the methods nil answers itself (universalCall); any
// other call on a union's nil raises NoMethodError, as on a nil T?.
func nilAnswers(name string) bool {
	return untypedIntrinsics[name] || name == "eql?" || nilConversions[name] != ""
}

// unionCall is a call on a union (decision 150): a Go type switch with one
// arm per member, each the typed call on that member, so no dispatcher and
// no boxed arguments. The arms' results join into the call's type (a union
// again when they have no common class). A member without the method is a
// compile error; nil, when a member, answers what nil answers and otherwise
// raises NoMethodError, as a nil T? does (decision 20).
func (f *fctx) unionCall(n parser.Node, recv expr, u TUnion, name string, args []parser.Node, block parser.Node) expr {
	nilOwn := f.reopenedMarker("NilClass", name) != nil
	raises := func(m Type) bool { return isNil(m) && !nilAnswers(name) && !nilOwn }
	if slices.ContainsFunc(u.Members, raises) {
		f.warn(n, "%s called on %s, which may be nil (raises NoMethodError on nil)", name, u)
	}
	// The arguments once, before the switch: generated in every arm they cost
	// members^depth for nested union arithmetic, and a local they assign
	// would be declared only in the arm's scope.
	if f.lenient() && slices.ContainsFunc(u.Members, func(m Type) bool { // gem code: a member without the method dispatches at run time
		mc, ok := m.(TClass)
		return ok && f.resolve(mc, name) == nil
	}) {
		return f.genMethodCall(n, expr{code: recv.code, typ: TAny{}}, name, args, block)
	}
	args = f.pinArgs(args, len(args))
	return f.unionSwitch(n, recv.code, u, func(m Type, code string) (expr, bool) {
		if raises(m) {
			f.emit("panic(rbNoMethod(%q, nil, false))", name)
			return expr{}, false
		}
		if isNil(m) && nilOwn { // the program's own NilClass method (decision 186)
			return f.genMethodCall(n, expr{code: "(*NilClass)(nil)", typ: f.cls("NilClass")}, name, args, block), true
		}
		return f.genMethodCall(n, expr{code: code, typ: m}, name, args, block), true
	}, func(m Type, msg string) string { return fmt.Sprintf("%s (a member of %s)", msg, u) })
}

// pinArgs evaluates args[:upto] that are not pure into temps, in order, so
// a type switch's arms reuse their values. A literal (its type follows the
// parameter), a block, a splat or keywords is left to each arm.
func (f *fctx) pinArgs(args []parser.Node, upto int) []parser.Node {
	out := slices.Clone(args)
	for j := range upto {
		switch args[j].(type) {
		case *parser.SplatNode, *parser.KeywordHashNode, *parser.BlockArgumentNode, *parser.ForwardingArgumentsNode,
			*parser.ArrayNode, *parser.HashNode, *parser.LambdaNode:
			continue
		}
		if pureNode(args[j]) {
			continue
		}
		e := f.genExpr(args[j], nil)
		if isVoid(e.typ) {
			continue // a void call has no value to pin; dispatch reports it
		}
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(e))
		out[j] = &exprNode{Node: args[j], e: expr{code: tmp, typ: e.typ}}
	}
	return out
}

// unionSwitch emits `switch v := subj.(type)` over u's members (decision
// 150). arm generates a member's value from its Go value (nil for nil), or
// emits a statement that never completes and reports false. The arms' types
// join into the result: a temp, or nothing when every arm is void. A member
// whose arm does not compile is an error, worded by fail.
func (f *fctx) unionSwitch(n parser.Node, subj string, u TUnion, arm func(m Type, code string) (expr, bool), fail func(m Type, msg string) string) expr {
	sw := f.newTmp()
	code := func(m Type) string {
		if isNil(m) {
			return "nil"
		}
		return sw
	}
	var res Type
	bad := map[int]string{} // a gem's member no arm compiles for: a run-time TypeError (decision 172)
	for i, m := range u.Members {
		var t Type
		var err *compileError
		f.probe(func() {
			err = f.try(func() {
				if e, ok := arm(m, code(m)); ok {
					t = e.typ
				}
			})
		})
		switch {
		case err != nil && f.lenient():
			bad[i] = fail(m, err.msg)
		case err != nil:
			f.errorf(n, "%s", fail(m, err.msg))
		case t == nil:
		case res == nil:
			res = t
		default:
			res = joinUnion(res, t)
		}
	}
	if res == nil && len(bad) > 0 { // every arm raises: still a value for whatever reads it
		res = TAny{}
	}
	if res == nil {
		res = TVoid{}
	}
	tmp := ""
	if !isVoid(res) || isNil(res) {
		tmp = f.newTmp()
		goT := f.c.goType(res)
		if strings.TrimLeft(goT, "*") == "" { // arms answering nil or void (`pipe&.puts` on an IO or a channel): no Go type of their own
			goT = "any"
		}
		f.emit("var %s %s", tmp, goT)
	}
	f.emit("switch %s := %s.(type) {", sw, subj)
	f.switches++
	for i, m := range u.Members {
		goT := "nil"
		if !isNil(m) {
			goT = f.c.goType(m)
		}
		f.emit("case %s:", goT)
		if msg, ok := bad[i]; ok {
			f.emit("\t_ = %s", sw)
			f.emit("\tpanic(NewTypeError(Ref(String(%q))))", "rugaroo: "+msg)
			continue
		}
		saved := f.enterBlock()
		f.indent++
		e, ok := arm(m, code(m))
		switch {
		case !ok:
		case tmp != "" && !e.noreturn && (e.code == "" || isVoid(e.typ) && !isNil(e.typ)): // a void arm where another is untyped: run, and nil
			f.emitExprStmt(n, e)
		case tmp != "" && !e.noreturn:
			f.emit("%s = %s", tmp, f.coerce(n, e, res))
		default:
			f.emitExprStmt(n, e)
		}
		f.indent--
		f.leaveBlock(saved)
	}
	f.switches--
	f.emit("}")
	if tmp == "" {
		return expr{typ: TVoid{}, done: true}
	}
	return expr{code: tmp, typ: res}
}

// attempt runs gen as it is emitted, unless it fails to compile: then what
// it emitted is dropped, as probe would, and the error returned. Only the
// failing path copies the output.
func (f *fctx) attempt(gen func()) *compileError {
	buf, mark, warned := f.buf, f.buf.Len(), len(f.c.Warnings)
	err := f.try(gen)
	if err != nil {
		kept := buf.String()[:mark]
		buf.Reset()
		buf.WriteString(kept)
		f.buf = buf
		f.c.dropWarnings(warned) // and forget them, so the retry may warn again
	}
	return err
}

// unionArgCall is a call whose argument is a union the method does not
// take as it is (decision 150): a type switch on that argument, calling
// with each member, so a typed parameter or a class twin (`__split_regexp`)
// is picked per member. The receiver and the arguments before it are
// evaluated first, as Ruby does; false when no argument is a union.
func (f *fctx) unionArgCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	at := -1
	var u TUnion
	for i, a := range args {
		switch a.(type) {
		case *parser.SplatNode, *parser.KeywordHashNode, *parser.BlockArgumentNode, *parser.ForwardingArgumentsNode:
			return expr{}, false
		}
		t := f.probeType(a, nil)
		if tu, ok := t.(TUnion); ok {
			at, u = i, tu
			break
		}
	}
	if at < 0 {
		return expr{}, false
	}
	if f.lenient() && !slices.ContainsFunc(u.Members, func(m Type) bool { // no member takes it: the caller's dynamic call
		var err *compileError
		f.probe(func() {
			err = f.try(func() {
				margs := slices.Clone(args)
				margs[at] = &exprNode{Node: args[at], e: expr{code: "x", typ: m}, use: u}
				f.unionArm++
				defer func() { f.unionArm-- }()
				f.genMethodCall(n, recv, name, margs, block)
			})
		})
		return err == nil
	}) {
		return expr{}, false
	}
	if !isSimpleGo(recv.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(recv))
		recv.code = tmp
	}
	nargs := f.pinArgs(args, at)
	a := f.genExpr(args[at], nil)
	return f.unionSwitch(n, a.code, u, func(m Type, code string) (expr, bool) {
		margs := slices.Clone(nargs)
		margs[at] = &exprNode{Node: args[at], e: expr{code: code, typ: m}, use: u}
		f.unionArm++
		defer func() { f.unionArm-- }()
		return f.genMethodCall(n, recv, name, margs, block), true
	}, func(m Type, msg string) string {
		return fmt.Sprintf("%s (argument %d is %s, and %s is a member; narrow it first)", msg, at+1, u, m)
	}), true
}

// allLiteral reports arguments that are all literals, which are never unions.
func allLiteral(args []parser.Node) bool {
	return !slices.ContainsFunc(args, func(a parser.Node) bool {
		switch a.(type) {
		case *parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.IntegerNode, *parser.FloatNode,
			*parser.StringNode, *parser.SymbolNode, *parser.RegularExpressionNode:
			return false
		}
		return true
	})
}

// pureNode reports a node whose value is the same however often, and
// whenever, it is generated: evaluating it inside each arm keeps Ruby's order.
func pureNode(n parser.Node) bool {
	switch n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.SelfNode, *parser.NilNode,
		*parser.TrueNode, *parser.FalseNode, *parser.IntegerNode, *parser.FloatNode, *parser.StringNode,
		*parser.SymbolNode, *parser.ConstantReadNode, *parser.ConstantPathNode, *parser.RegularExpressionNode:
		return true
	}
	return false
}

// literalMember is the member of u a literal takes as its expected type:
// of the members each preference holds for, in turn, the first gen compiles
// with; nil (nothing expected, so the literal types itself) when none does.
func (f *fctx) literalMember(u TUnion, gen func(Type), prefs ...func(Type) bool) Type {
	for _, pref := range prefs {
		for _, m := range u.Members {
			if !pref(m) {
				continue
			}
			var err *compileError
			f.probe(func() { err = f.try(func() { gen(m) }) })
			if err == nil {
				return m
			}
		}
	}
	return nil
}

// joinUnion is join, falling back to the union where join finds no common
// class (or gives untyped for Integer and Float) (decision 150).
func joinUnion(a, b Type) Type {
	if isAny(a) || isAny(b) {
		return TAny{}
	}
	if j, ok := join(a, b); ok && !isAny(j) {
		return j
	}
	return unionOf(a, b)
}

// nilGuard emits Ruby's NoMethodError for a call on a nil T? receiver and
// returns the receiver dereferenced to T.
func (f *fctx) nilGuard(n parser.Node, recv expr, name string) expr {
	elem := recv.typ.(TOpt).Elem
	f.warn(n, "%s called on possibly-nil %s (raises NoMethodError on nil)", name, elem)
	f.nilGuarded[n] = true
	code := f.pinSimple(recv.code)
	f.emit("if %s {", optIsNil(code, recv.typ))
	f.emit("\tpanic(rbNoMethod(%q, nil, false))", name)
	f.emit("}")
	return expr{code: derefOpt(code, recv.typ), typ: elem, bin: recv.bin, binDeps: recv.binDeps}
}

// nilFreeze is freeze/frozen?/dup/clone on a T?: nil's own answers (itself, true, itself), T's otherwise.
func (f *fctx) nilFreeze(n parser.Node, recv expr, name string) expr {
	rt := f.pinSimple(recv.code)
	inner := expr{code: derefOpt(rt, recv.typ), typ: recv.typ.(TOpt).Elem}
	if name == "frozen?" {
		var e expr
		body := f.capture(func() { e = f.genMethodCall(n, inner, name, nil, nil) })
		if body != "" { // the call needs statements: it cannot sit in one expression
			return f.nilGuard(n, recv, name)
		}
		return expr{code: "Boolean(" + optIsNil(rt, recv.typ) + " || bool(" + e.code + "))", typ: f.cls("Boolean")}
	}
	if name == "dup" || name == "clone" { // a copy of T, or nil
		out := f.newTmp()
		f.emit("var %s %s", out, f.c.goType(recv.typ))
		f.emit("if %s {", optNotNil(rt, recv.typ))
		f.indent++
		e := f.genMethodCall(n, inner, name, nil, nil)
		f.emit("%s = %s", out, f.coerce(n, e, recv.typ))
		f.indent--
		f.emit("}")
		return expr{code: out, typ: recv.typ}
	}
	f.emit("if %s {", optNotNil(rt, recv.typ))
	f.indent++
	f.discard(f.genMethodCall(n, inner, name, nil, nil))
	f.indent--
	f.emit("}")
	return expr{code: rt, typ: recv.typ}
}

// nilClassCall: nil answers to_i/=~/... itself (0, nil), not NoMethodError, when T's method's type can hold that answer.
func (f *fctx) nilClassCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	if (name == "=~") != (len(args) == 1) || len(args) > 1 {
		return expr{}, false
	}
	rt := f.newTmp()
	inner := expr{code: derefOpt(rt, recv.typ), typ: recv.typ.(TOpt).Elem}
	call := func() expr { return f.genMethodCall(n, inner, name, args, nil) }
	probe, once := f.genOperandOnce(call)
	t := probe.e.typ
	var zero string // "" is Go's zero value
	switch {
	case name == "to_i" && isClass(t, "Integer"), name == "to_f" && isClass(t, "Float"),
		name == "=~" && (isOpt(t) || isAny(t)):
	case name == "to_a" && isClass(t, "Array"):
		zero = "(&Array[" + f.c.goType(t.(TClass).Args[0]) + "]{})"
	case name == "to_h" && isClass(t, "Hash"):
		a := t.(TClass).Args
		zero = "NewHash[" + f.c.goType(a[0]) + ", " + f.c.goType(a[1]) + "]()"
	default:
		return expr{}, false
	}
	// ponytail: =~'s argument is only evaluated when the receiver is non-nil; hoist it if a side-effecting pattern ever matters
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(t))
	f.emit("if %s := %s; %s {", rt, recv.code, optNotNil(rt, recv.typ))
	f.indent++
	e := f.operandAgain(probe, once, call)
	f.emit("%s = %s", tmp, f.coerce(n, e, t))
	f.indent--
	if zero != "" {
		f.emit("} else {")
		f.emit("\t%s = %s", tmp, zero)
	}
	f.emit("}")
	return expr{code: tmp, typ: t}, true
}

// readsUnionLocal reports whether n reads a local whose type is a union (decision 150).
func (f *fctx) readsUnionLocal(n parser.Node) bool {
	if n == nil {
		return false
	}
	switch r := n.(type) {
	case *parser.LocalVariableReadNode:
		info := f.localInfo(r.Name)
		return info != nil && isUnion(stripOpt(info.typ))
	case *exprNode, *assignedArg, *withMember, *constInit, *loadFile:
		return false // the compiler's own nodes: already generated, and their embedded Node may be nil
	}
	return slices.ContainsFunc(n.CompactChildNodes(), f.readsUnionLocal)
}

// unionHasVar reports whether type variable name is a member of u.
func unionHasVar(u TUnion, name string) bool {
	return slices.ContainsFunc(u.Members, func(m Type) bool { mv, ok := m.(TVar); return ok && mv.Name == name })
}

// varToUnion is a generic value into a union: boxed when its variable is a member (U into `Array | String | U`,
// Homebrew's deep_remove_placeholders: a package-level checker cannot name U), else checked at run time, as untyped is.
func (f *fctx) varToUnion(n parser.Node, e expr, t TVar, to TUnion) string {
	if unionHasVar(to, t.Name) {
		return "any(" + e.code + ")"
	}
	return f.noteConv(n, f.c.unionFn(to)+"(any("+e.code+"))")
}

// plainOptEq: an Integer?, Float?, Symbol? or String? (decision 399) against a value of its own class, whose == (the prelude's) is Go's.
func (f *fctx) plainOptEq(recv, arg Type) bool {
	o, ok := recv.(TOpt)
	if !ok {
		return false
	}
	c, ok := o.Elem.(TClass)
	if !ok || !typeEq(c, arg) {
		return false
	}
	switch c.C.RubyName {
	case "Integer", "Float", "Symbol", "String":
	default:
		return false
	}
	eq := c.C.lookup("==")
	return eq != nil && eq.M.File != nil && eq.M.File.isPrelude() && (valOpt(recv) || f.c.goType(recv) == "*"+f.c.goType(c))
}
