package compiler

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// dynEach is a gem's `x.each { |a, b| }` on an untyped x, as a Rack app's headers and body are (decision 176).
func (f *fctx) dynEach(n parser.Node, recv expr, b *parser.BlockNode) expr {
	k := len(f.blockParamList(b.Parameters))
	f.c.dynEach = true
	if k == 0 { // liquid's `@input.each { return false }`: each element runs the block, which takes none (decision 331)
		clo := f.genClosure(n, b, &BlockSig{Ret: TVoid{}}, nil)
		return expr{code: fmt.Sprintf("rbEachAny(%s, 1, func(_ ...any) { (%s)() })", recv.code, clo), typ: TAny{}}
	}
	params, xs := make([]Type, k), make([]string, k) // 3+ parameters splat each element across them, as MRI's
	for i := range k {
		params[i], xs[i] = TAny{}, fmt.Sprintf("xs_[%d]", i)
	}
	clo := f.genClosure(n, b, &BlockSig{Params: params, Ret: TVoid{}}, nil)
	return expr{code: fmt.Sprintf("rbEachAny(%s, %d, func(xs_ ...any) { (%s)(%s) })", recv.code, k, clo, strings.Join(xs, ", ")), typ: TAny{}}
}

// gemDynShortcut is a gem's untyped call the runtime answers without a dispatcher (decision 324): `names.zip(objs)`
// (bindata's Struct#each_pair) and a blockless `pairs.each`, an Enumerator over the elements.
func (f *fctx) gemDynShortcut(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	switch {
	case !f.lenient():
	case name == "zip" && len(args) > 0 && !slices.ContainsFunc(args, isSplat):
		f.c.dynEach = true
		codes := make([]string, 0, len(args)+1)
		codes = append(codes, f.coerce(n, recv, TAny{}))
		for _, a := range args {
			codes = append(codes, f.coerce(a, f.genExpr(a, nil), TAny{}))
		}
		return expr{code: "rbZipAny(" + strings.Join(codes, ", ") + ")", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}, true
	case name == "each" && len(args) == 0:
		f.c.dynEach = true
		return expr{code: "rbEachEnumAny(" + f.coerce(n, recv, TAny{}) + ")", typ: TAny{}}, true
	case name == "scan" && len(args) == 1 && f.c.classes["String"].lookup("__scan_any") != nil:
		return f.dynStringScan(n, recv, args), true
	case (name == "each_cons" || name == "each_slice") && len(args) == 1 && !isSplat(args[0]):
		// `tokens.each_cons(2).all? { |a, b| ... }` with no block on an untyped list (rubocop-ast's tokens_sorted?): the
		// Array's own blockless form over its elements (decision 444)
		f.c.dynEach = true
		elems := expr{code: "rbElemsAny(" + f.coerce(n, recv, TAny{}) + ")", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		return f.genMethodCall(n, elems, name, args, nil), true
	case (name == "with_index" || name == "each_with_index") && len(args) == 0:
		// `x.each.with_index` with no block on an untyped Enumerator (or anything with an each): the pairs, as an
		// Array's each_with_index enumerator gives them (decision 444)
		f.c.dynEach = true
		elems := expr{code: "rbElemsAny(" + f.coerce(n, recv, TAny{}) + ")", typ: TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}}
		return f.genMethodCall(n, f.genMethodCall(n, elems, "each", nil, nil), "__with_index_enum", nil, nil), true
	}
	return expr{}, false
}

// dynStringScan is a gem's `x.scan(re)` on an untyped x: a String's matches or groups as its pattern has them
// (liquid's `token.scan(ContentOfVariable)`), anything else's scan (a StringScanner's) as before (decision 331).
func (f *fctx) dynStringScan(n parser.Node, recv expr, args []parser.Node) expr {
	r, res, sv := f.newTmp(), f.newTmp(), f.newTmp()
	f.emit("%s := %s", r, f.coerce(n, recv, TAny{}))
	f.emit("var %s any", res)
	f.emit("if %s, ok := %s.(String); ok {", sv, r)
	f.indent++
	saved := f.enterBlock()
	e := f.genMethodCall(n, expr{code: sv, typ: f.cls("String")}, "__scan_any", args, nil)
	f.emit("%s = %s", res, f.coerce(n, e, TAny{}))
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	f.c.noteDyn("scan")
	codes := make([]string, 0, 2+len(args))
	codes = append(codes, "rbCall", r)
	for _, a := range args {
		codes = append(codes, f.coerce(a, f.genExpr(a, nil), TAny{}))
	}
	f.emit("%s = rbDyn%s(%s)", res, f.c.dynGoName("scan"), strings.Join(codes, ", "))
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	return expr{code: res, typ: TAny{}}
}

// universalBlockCall is universalCall with a block: one method every receiver shares, or a gem's untyped each.
func (f *fctx) universalBlockCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if e := f.c.universalOnly(name); e != nil { // the same method on every receiver: no dispatch needed
		return f.callEntry(n, e, recv, args, block)
	}
	if f.c.isNumericMod(classOf(recv.typ)) {
		f.errorf(n, "a block on a Numeric is not supported: narrow it with is_a? first (decision 142)")
	}
	if b, ok := block.(*parser.BlockNode); ok && name == "each" && len(args) == 0 && f.lenient() && isAny(recv.typ) {
		return f.dynEach(n, recv, b)
	}
	if ba, ok := block.(*parser.BlockArgumentNode); ok && name == "each" && len(args) == 0 && f.lenient() && f.c.goType(recv.typ) != "" && (isAny(recv.typ) || isTypeVar(recv.typ)) &&
		f.isBlockParam(ba.Expression) && !f.iterator && f.blockSig != nil && len(f.blockSig.Params) >= 1 && len(f.blockSig.Params) <= 2 {
		return f.dynEachForward(n, recv)
	}
	if e, ok := f.forwardedEnum(n, recv, name, args, block); ok {
		return e
	}
	if f.lenient() && f.heldAsAny(recv.typ) && name == "with_object" && len(args) == 1 {
		// an untyped Enumerator's with_object is each_with_object over what it yields (rubocop's
		// `lines.each.with_index.with_object({}) do |(line, i), acc|`; decision 444)
		name = "each_with_object"
	}
	if f.lenient() && f.heldAsAny(recv.typ) && dynEnumNames[name] {
		return f.dynEnumBlock(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, name, args, block)
	}
	if b, ok := block.(*parser.BlockNode); ok && f.lenient() {
		return f.genDynBlockCall(n, recv, name, args, b)
	}
	if _, ok := block.(*parser.BlockArgumentNode); ok && f.lenient() { // a method's own `&block` passed on (concurrent-ruby's `ns_initialize(opts, &block)`)
		if v, ok := f.dynBlockValue(n, block); ok {
			if f.c.dynBlockNames == nil {
				f.c.dynBlockNames = map[string]bool{}
			}
			f.c.dynBlockNames[name] = true
			return f.genDynCall(n, recv, name, append(slices.Clone(args), &exprNode{e: expr{code: v, typ: TAny{}}, blockArg: true}))
		}
	}
	if ba, ok := block.(*parser.BlockArgumentNode); ok && f.lenient() && ba.Expression != nil {
		if pb := f.procBlock(ba); pb != nil { // `&order`, a callable or a Symbol decided when it runs (decision 444)
			if ps, ok := pb.Parameters.(*parser.BlockParametersNode); ok && ps.Parameters != nil && ps.Parameters.Rest != nil && len(pb.Locals) == 1 {
				// a dynamic block takes named parameters: `{ |x| xs = [x]; ... }`, the one value an iterator yields
				loc, x, xs := pb.Location, "x1_", pb.Locals[0]
				st, _ := pb.Body.(*parser.StatementsNode)
				one := &parser.LocalVariableWriteNode{Location: loc, Name: xs, Value: &parser.ArrayNode{Location: loc, Elements: []parser.Node{&parser.LocalVariableReadNode{Location: loc, Name: x}}}}
				pb = &parser.BlockNode{Location: loc, Locals: []string{x, xs},
					Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: x}}}},
					Body:       &parser.StatementsNode{Location: loc, Body: append([]parser.Node{one}, st.Body...)}}
			}
			return f.genDynBlockCall(n, recv, name, args, pb)
		}
	}
	f.errorf(n, "blocks on untyped receivers are not supported")
	return expr{}
}

// forwardedEnum is a forwarder's `target.map(*args, &block)` on an untyped target (Forwardable's def_delegators over an
// untyped ivar, rubocop's Config; decision 444). An iterator (each_key) goes to the dispatcher, whose wrappers take the
// block; a generic Enumerable method (map) has no such wrapper, so with no arguments it is dynEnumBlock's call, and
// with some the dispatcher's.
func (f *fctx) forwardedEnum(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	_, fwd := block.(*parser.BlockArgumentNode)
	if !fwd || !f.lenient() || !f.heldAsAny(recv.typ) || !dynEnumNames[name] || len(args) != 1 || !isSplat(args[0]) {
		return expr{}, false
	}
	v, ok := f.dynBlockValue(n, block)
	if !ok {
		return expr{}, false
	}
	dispatch := func() expr {
		return f.genDynCall(n, recv, name, append(slices.Clone(args), &exprNode{e: expr{code: v, typ: TAny{}}, blockArg: true}))
	}
	iter := func(cls string) bool { e := f.c.classes[cls].lookup(name); return e != nil && e.M.Iterator }
	if iter("Hash") || iter("Array") {
		if f.c.dynBlockNames == nil {
			f.c.dynBlockNames = map[string]bool{}
		}
		f.c.dynBlockNames[name] = true // only here: a generic method's wrapper cannot bind its type parameters to a block
		return dispatch(), true
	}
	rest := f.genExpr(args[0].(*parser.SplatNode).Expression, nil)
	if !isClass(rest.typ, "Array") {
		return expr{}, false
	}
	res := f.newTmp()
	f.emit("var %s any", res)
	f.emit("if len(%s.s) == 0 && blk != nil {", f.materialize(rest)) // no block: the plain call, through the dispatcher
	f.indent++
	saved := f.enterBlock()
	f.emit("%s = %s", res, f.coerce(n, f.dynEnumBlock(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, name, nil, block), TAny{}))
	f.leaveBlock(saved)
	f.indent--
	f.emit("} else {")
	f.indent++
	saved = f.enterBlock()
	f.emit("%s = %s", res, f.genDynCall(n, recv, name, args).code) // ponytail: a block with arguments is dropped here, as before this rule
	f.leaveBlock(saved)
	f.indent--
	f.emit("}")
	return expr{code: res, typ: TAny{}}, true
}

// heldAsAny is untyped, a gem module's self, or a value held as Go any (a Sorbet T::Enumerable[X]).
func (f *fctx) heldAsAny(t Type) bool {
	_, tv := t.(TVar)
	return isAny(t) || tv || f.c.goType(t) == "any"
}

// autoSplat is the statement that splats a lone Array argument across a literal block of two or more parameters, as
// MRI does for a proc (decision 302); empty for fewer.
func autoSplat(params int, args string) string {
	if params < 2 {
		return ""
	}
	return fmt.Sprintf("%s = rbAutoSplat(%s); ", args, args)
}

// genDynBlockCall is a gem's call with a block on an untyped receiver (concurrent-ruby's `synchronize { }` on a mixin's
// self): the block goes last, as an rbBlockArg each wrapper of a block-taking method of that name adapts (decision 243).
func (f *fctx) genDynBlockCall(n parser.Node, recv expr, name string, args []parser.Node, b *parser.BlockNode) expr {
	names := f.blockParamList(b.Parameters)
	if slices.ContainsFunc(names, func(p blockParam) bool { return p.Rest }) { // a destructuring one binds as any closure's does (rubocop's `with_object({}) do |(line, i), acc|`; decision 444)
		f.errorf(n, "a block with a rest parameter on an untyped receiver is not supported")
	}
	if b.Body != nil && (blockBreaks(b.Body) || blockBreaksValue(b.Body)) && (f.breakCatch == nil || f.breakCatch.block != b) {
		return f.dynBlockBreakCatch(n, recv, name, args, b)
	}
	if f.c.dynBlockNames == nil {
		f.c.dynBlockNames = map[string]bool{}
	}
	f.c.dynBlockNames[name] = true
	ps := make([]Type, len(names))
	vals := make([]string, len(names))
	for i := range ps {
		ps[i] = TAny{}
		vals[i] = fmt.Sprintf("rbArgAt(xs_, %d)", i)
	}
	clo := f.genClosure(n, b, &BlockSig{Params: ps, Ret: TAny{}}, map[string]Type{})
	arg := fmt.Sprintf("rbBlockArg{fn: func(xs_ ...any) any { %sreturn (%s)(%s) }}", autoSplat(len(names), "xs_"), clo, strings.Join(vals, ", "))
	return f.genDynCall(n, recv, name, append(slices.Clone(args), &exprNode{e: expr{code: arg, typ: TAny{}}}))
}

// dynBlockBreakCatch is genDynBlockCall for a block that breaks: the call runs inside a function that recovers the
// block's rbBreak, and its value is the break's when one leaves it (decisions 241, 444).
func (f *fctx) dynBlockBreakCatch(n parser.Node, recv expr, name string, args []parser.Node, b *parser.BlockNode) expr {
	saved, out := f.breakCatch, f.buf
	bc := &breakCatch{block: b, id: fmt.Sprintf("%s:%d", f.f.Name, b.Location.StartOffset)}
	tmp := f.newTmp()
	f.breakCatch, f.buf = bc, &strings.Builder{}
	restored := false
	defer func() {
		if !restored {
			f.breakCatch, f.buf = saved, out
		}
	}()
	f.indent++
	e := f.genDynBlockCall(n, recv, name, args, b)
	f.emit("%s = %s", tmp, e.code)
	f.indent--
	body := f.buf.String()
	f.breakCatch, f.buf, restored = saved, out, true
	f.emit("var %s any", tmp)
	f.emit("func() {")
	f.emit("\tdefer rbBreakRecoverTo(%q, &%s)", bc.id, tmp)
	out.WriteString(body)
	f.emit("}()")
	return expr{code: tmp, typ: TAny{}}
}

// dynEnumNames are the Enumerable, Array and Hash methods that take a block and leave their receiver as it is: what
// dynEnumBlock answers on an untyped receiver from a Hash or Array copy of it.
var dynEnumNames = map[string]bool{
	"select": true, "filter": true, "reject": true, "map": true, "collect": true, "flat_map": true, "collect_concat": true,
	"filter_map": true, "find": true, "detect": true, "find_index": true, "find_all": true, "any?": true, "all?": true,
	"none?": true, "one?": true, "count": true, "sum": true, "min_by": true, "max_by": true, "sort_by": true, "group_by": true,
	"partition": true, "each_with_index": true, "each_with_object": true, "inject": true, "reduce": true, "to_h": true,
	"fetch": true, "uniq": true, "sort": true, "min": true, "max": true, "each_slice": true, "each_cons": true,
	"sum_by": true, "tally_by": true, "chunk_while": true, "slice_when": true, "zip": true, "each_key": true,
	"each_value": true, "each_pair": true, "transform_values": true, "transform_keys": true, "index_by": true, "to_set": true,
}

// dynEnumBlock is a gem's `x.select { }` on an untyped x (decision 224): a Hash takes Hash[untyped, untyped]'s method,
// anything else Array[untyped]'s over its elements (an Array's own, else what its each yields, rbEnumAny). Each branch
// compiles on its own; one that cannot (a Hash-only name on the Array side) raises NoMethodError if taken.
func (f *fctx) dynEnumBlock(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	f.c.dynEach = true
	hashT := TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}
	arrT := TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}
	tmp, r := f.newTmp(), f.newTmp()
	f.emit("var %s any", tmp)
	f.emit("switch %s := rbUnbox(%s).(type) {", r, recv.code)
	branch := func(head string, on expr) {
		f.emit("%s", head)
		if call, ok := n.(*parser.CallNode); ok {
			if e := classOf(on.typ).lookup(name); e != nil && e.M.Iterator && e.M.Block != nil {
				if _, isBlock := call.Block.(*parser.BlockNode); isBlock { // `each_key do ... end`, `each_cons(2) do |a, b| ... end` (rubocop's token pairs): a range loop, valued its receiver as MRI's
					var err *compileError
					f.probe(func() { err = catchCompileError(func() { f.genIterLoop(call, e, on) }) })
					f.indent++
					if err != nil {
						f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
					} else {
						f.genIterLoop(call, e, on)
						f.emit("%s = %s", tmp, r)
					}
					f.indent--
					return
				}
			}
		}
		var err *compileError
		f.probe(func() { err = catchCompileError(func() { f.genMethodCall(n, on, name, args, block) }) })
		f.indent++
		if err != nil && f.c.env.ArmDebug && !f.probing() { // RUGAROO_ARM_DEBUG: why a receiver class of an untyped call raises NoMethodError
			fmt.Fprintf(os.Stderr, "rugaroo: arm %s of %s does not compile: %s\n", strings.TrimSuffix(strings.TrimPrefix(head, "case "), ":"), name, err.Error())
		}
		if err != nil {
			f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
		} else if e := f.genMethodCall(n, on, name, args, block); isVoid(e.typ) || e.stmt {
			f.emitExprStmt(n, e)
		} else {
			f.emit("%s = %s", tmp, f.coerce(n, e, TAny{}))
		}
		f.indent--
	}
	branch("case Hash_Any:", expr{code: r + "._ToAny()", typ: hashT})
	if env := f.c.classes["ENVClass"]; env != nil && env.lookup("to_h") != nil { // ENV is Hash-like: its pairs, as a Hash (decision 229)
		branch("case *ENVClass:", expr{code: r + ".ToH()._ToAny()", typ: hashT})
	}
	seenOwner := map[*Class]bool{}
	for _, m := range f.c.gemMethodsNamed(name) {
		// a gem class with its own method of the name (rubocop's Config#transform_values, a forwarder to its Hash) is
		// neither a Hash nor a list of elements: its method (decision 444)
		o := m.Owner
		if o == nil || seenOwner[o] || o.IsModule || o.metaOf != nil || len(o.TypeParams) > 0 || !o.isStruct() || o.hashBase || m.Block == nil {
			continue
		}
		seenOwner[o] = true
		t := TClass{C: o}
		branch("case "+f.c.goType(t)+":", expr{code: r, typ: t})
	}
	branch("default:", expr{code: "rbElemsAny(" + r + ")", typ: arrT})
	f.emit("}")
	return expr{code: tmp, typ: TAny{}}
}

// universalCall handles Kernel-level methods on values of unknown type.
func (f *fctx) universalCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil {
		return f.universalBlockCall(n, recv, name, args, block)
	}
	one := func(exp Type) expr {
		if len(args) != 1 {
			f.errorf(n, "%s takes one argument", name)
		}
		return f.genExpr(args[0], exp)
	}
	switch name {
	case "to_s":
		return expr{code: "rbToS(" + recv.code + ")", typ: f.cls("String")}
	case "inspect":
		return expr{code: "rbInspect(" + recv.code + ")", typ: f.cls("String")}
	case "to_json":
		return expr{code: "rbToJson(" + strings.Join(append([]string{recv.code}, f.jsonArgs(args)...), ", ") + ")", typ: f.cls("String")}
	case "nil?":
		return expr{code: "Boolean(rbUnbox(any(" + recv.code + ")) == nil)", typ: f.cls("Boolean")} // T may be X?, a *X box
	case "!":
		return expr{code: "Boolean(!rbTruthy(" + recv.code + "))", typ: f.cls("Boolean")}
	case "==":
		a := one(recv.typ)
		code := "rbEq(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")"
		if isAny(recv.typ) || isNil(recv.typ) || isAbstract(recv.typ) || isAny(a.typ) { // `x == v` with v untyped: Enumerable#include?
			code = "rbEq[any](" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")"
		}
		return expr{code: code, typ: f.cls("Boolean")}
	case "equal?":
		a := one(nil)
		return expr{code: "Boolean(rbIdentical(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + "))", typ: f.cls("Boolean")}
	case "<=>":
		if isAny(recv.typ) || isNil(recv.typ) || f.c.isNumericMod(classOf(recv.typ)) { // a Numeric's <=> is nil for an incomparable argument too
			break // DynOp_cmp: untyped, since MRI answers nil for incomparable values
		}
		a := one(recv.typ)
		return expr{code: "rbCmp(" + recv.code + ", " + f.coerce(args[0], a, recv.typ) + ")", typ: f.cls("Integer")}
	case "hash":
		return expr{code: "rbHash(" + recv.code + ")", typ: f.cls("Integer")}
	case "eql?":
		if c := classOf(recv.typ); c != nil && c.universal && !c.IsModule { // Object has no eql? to dispatch to: the value's own, else identity
			a := one(nil)
			return expr{code: "rbEql(" + recv.code + ", " + f.coerce(args[0], a, TAny{}) + ")", typ: f.cls("Boolean")}
		}
	}
	if isNil(recv.typ) && len(args) == 0 {
		if t, ok := map[string]Type{"to_a": TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}, "to_h": TClass{C: f.c.classes["Hash"], Args: []Type{TAny{}, TAny{}}}, "to_i": f.cls("Integer"), "to_f": f.cls("Float")}[name]; ok {
			f.discard(recv)
			return expr{code: nilConversions[name], typ: t}
		}
	}
	if recv.nilable {
		f.warn(n, "%s called on possibly-nil untyped (raises NoMethodError on nil)", name)
	}
	return f.genDynCall(n, recv, name, args)
}

// descendantResponds: a self call the class can't answer may still reach a subclass's method (or one it includes) at run time.
func descendantResponds(c *Class, name string) bool {
	for _, sub := range c.Subclasses {
		if sub.metaOf != nil && c.metaOf == nil {
			continue
		}
		if sub.lookup(name) != nil || descendantResponds(sub, name) {
			return true
		}
	}
	return false
}

// untypedIntrinsics are what universalCall answers inline, for any value.
var untypedIntrinsics = map[string]bool{"to_s": true, "inspect": true, "to_json": true, "nil?": true, "!": true, "==": true, "equal?": true, "<=>": true, "hash": true}

// nilConversions are NilClass's conversions, for nil held statically or untyped.
var nilConversions = map[string]string{"to_a": "(&Array[any]{})", "to_h": "NewHash[any, any]()", "to_i": "Integer(0)", "to_f": "Float(0)"}

// nilOps are NilClass's other methods, answered for an untyped nil by its dispatcher: arity bounds and the value.
var nilOps = map[string]struct {
	min, max int
	code     string
}{
	"to_r": {0, 0, "rbRat(0, 1)"}, "rationalize": {0, 1, "rbRat(0, 1)"}, "to_c": {0, 0, "&Complex{Integer(0), Integer(0)}"},
	"&": {1, 1, "Boolean(false)"}, "|": {1, 1, "Boolean(rbTruthy(args[0]))"}, "^": {1, 1, "Boolean(rbTruthy(args[0]))"}, "=~": {1, 1, "nil"}, "!~": {1, 1, "Boolean(true)"},
}

func (f *fctx) dynClassOf(n parser.Node, recv expr) expr {
	f.c.classOf = true
	return expr{code: "rbClassOf(" + f.coerce(n, recv, TAny{}) + ")", typ: f.cls("Class")}
}

// callMissing sends an unknown method to the receiver's method_missing,
// with the name as a Symbol; Ruby calls it even when it is private.
func (f *fctx) callMissing(n parser.Node, mm *entry, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if block != nil && mm.M.Block == nil {
		f.errorf(n, "a block passed to %s (handled by method_missing) is not supported", name)
	}
	nodes := append([]parser.Node{&parser.SymbolNode{Unescaped: parser.RubyString{Value: name}}}, args...)
	if b, ok := block.(*parser.BlockNode); ok && anyRestBlock(mm.M.Block) {
		if fn, ok := f.rebindableBlock(n, b, false); ok {
			block = &parser.BlockArgumentNode{Expression: &exprNode{e: expr{code: fn, typ: TFunc{Params: []Type{TAny{}}, Rest: true, Ret: TAny{}}}}}
		}
	}
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.callEntry(n, mm, recv, nodes, block) // a method_missing taking &block gets the call's (bindata's `struct :e_ident do ... end`)
}

// notImplemented are MRI's rb_f_notimplement methods here (a Go program cannot fork): respond_to? is false (decision 326).
var notImplemented = map[string]bool{"fork": true}

// rubyPrivate names methods Ruby makes private whoever defines them.
var rubyPrivate = map[string]bool{"initialize": true, "respond_to_missing?": true, "initialize_copy": true}

// genRespondTo decides `recv.respond_to?(:name, include_all)` for a typed
// receiver, a literal name and a literal include_all: true for a public
// method (any method with include_all), else respond_to_missing?, else
// false. Returns false when the answer depends on the runtime class.
func (f *fctx) genRespondTo(n parser.Node, recv expr, args []parser.Node) (expr, bool) {
	name := literalName(args[0])
	var includeAll parser.Node = &parser.FalseNode{}
	if len(args) > 1 {
		switch args[1].(type) {
		case *parser.TrueNode:
			includeAll = args[1]
		case *parser.FalseNode, *parser.NilNode:
		default:
			return expr{}, false
		}
	}
	_, priv := includeAll.(*parser.TrueNode)
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no class known here: answered at run time
	}
	if name == "" || cls == nil {
		return expr{}, false
	}
	if cls.RubyName == "String" && stringMutators[name] { // MRI's in-place methods, which rugaroo compiles as rebinding (decisions 172, 382): answered, so prism's polyfill (`String.include(Module.new { def append_as_bytes ... })` unless `"".respond_to?(:append_as_bytes)`) stays out
		f.discard(recv)
		return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
	}
	if e := cls.lookup(name); e != nil && e.M.File != nil && e.M.File.isPrelude() && notImplemented[name] {
		f.discard(recv) // MRI's rb_f_notimplement: defined, raises NotImplementedError, and respond_to? is false
		return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
	}
	if e := cls.lookup(name); e != nil && (priv || !e.M.Private && !rubyPrivate[name] || kernelModuleFunction(e.M, recv.typ)) {
		f.discard(recv)
		return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
	}
	if cls.IsModule || cls.descendantDefines(name, priv) { // a module's value is some includer
		return expr{}, false
	}
	if rm := cls.lookup("respond_to_missing?"); rm != nil {
		f.implicitCall = true
		defer func() { f.implicitCall = false }()
		// Ruby hands respond_to_missing? a Symbol even for respond_to?("x").
		sym := &parser.SymbolNode{Unescaped: parser.RubyString{Value: name}, Location: args[0].GetLocation()}
		return f.callEntry(n, rm, recv, []parser.Node{sym, includeAll}, nil), true
	}
	f.discard(recv)
	return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
}

// genDynCall sends a method to an untyped value: see docs/dynamic-dispatch.md.
// Like MRI, only a call with an explicit receiver (other than self) cannot
// reach a private method; send (implicitCall) can.
func (f *fctx) genDynCall(n parser.Node, recv expr, name string, args []parser.Node) expr {
	if e, ok := f.gemDynShortcut(n, recv, name, args); ok {
		return e
	}
	if (name == "unpack" || name == "unpack1") && f.lenient() { // a gem's 64-bit fields wrap (decision 324)
		name = "__" + name + "_wrap"
	}
	if e, ok := f.dynContainerCall(n, recv, name, args); ok {
		return e
	}
	f.c.noteDyn(name)
	f.noteDynUses(name, args)
	if (f.m == nil || !f.m.quietDynamic) && !f.patQuiet && !f.c.isNumericMod(classOf(recv.typ)) { // a Numeric's class is chosen at run time by design (decision 142)
		if call, ok := n.(*parser.CallNode); ok && f.c.env.UntypedWhy && isAny(recv.typ) {
			f.warn(n, "dynamic call: %s on %s [why: %s]", name, recv.typ, f.untypedWhy(call.Receiver, 0))
		} else {
			f.warn(n, "dynamic call: %s on %s", name, recv.typ)
		}
	}
	how := "rbCall"
	call, _ := n.(*parser.CallNode)
	switch {
	case call != nil && call.IsVARIABLE_CALL():
		how = "rbVCall"
	case f.implicitCall || call != nil && (call.Receiver == nil || isSelf(call.Receiver)):
		how = "rbFCall"
	case call != nil && hookNames[name] && isClassObjHook(call.Receiver): // the runtime calls a hook, private or not (ruby-macho's `private def inherited`)
		how = "rbFCall"
	}
	if spread, ok := f.anySpread(args); ok { // `x.m(a, *rest, *more, **kw)` untyped: one slice into the wrapper's ...any (decision 227)
		return expr{code: rbDynFn(name) + "(" + how + ", " + f.coerce(n, recv, TAny{}) + ", " + spread + "...)", typ: TAny{}}
	}
	argCodes := f.anyArgs(args)
	if i := slices.IndexFunc(argCodes, func(c string) bool { return strings.HasSuffix(c, "...") }); i >= 0 {
		// an argument already spread (a forwarded *rest) with more after it: one slice, spread once at the end
		spread := "append([]any{" + strings.Join(argCodes[:i], ", ") + "}, " + argCodes[i] + ")"
		if len(argCodes) > i+1 {
			spread = "append(" + spread + ", " + strings.Join(argCodes[i+1:], ", ") + ")"
		}
		return expr{code: rbDynFn(name) + "(" + how + ", " + f.coerce(n, recv, TAny{}) + ", " + spread + "...)", typ: TAny{}}
	}
	codes := append([]string{how, f.coerce(n, recv, TAny{})}, argCodes...)
	return expr{code: rbDynFn(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// jsonArgs is anyArgs for to_json, plus its `to_json(*a)` idiom: a lone
// splat of an Array passes the elements on.
func (f *fctx) jsonArgs(args []parser.Node) []string {
	if len(args) != 1 {
		return f.anyArgs(args)
	}
	sp, ok := args[0].(*parser.SplatNode)
	if !ok {
		return f.anyArgs(args)
	}
	a := f.genExpr(sp.Expression, nil)
	ac, ok := a.typ.(TClass)
	if !ok || ac.C.Name != "Array" {
		f.errorf(sp, "splat of non-array %s", a.typ)
	}
	if !isAny(ac.Args[0]) {
		a.code += "._ToAny()"
	}
	return []string{"(" + a.code + ").s..."}
}

// dynContainerCall is an untyped `x.keys`, `x.values`, `x.to_a`: a Hash or Array method decision 207 gives no Dyn
// wrapper, since its result grows the class's type (Array[K]). On the [untyped] instantiation it grows nothing, so the
// receiver is switched on (Hash_Any, Array_Any) and the method called there; anything else goes to the dispatcher
// (decision 232).
func (f *fctx) dynContainerCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	if f.inDynContainer {
		return expr{}, false
	}
	hash, arr := f.c.classes["Hash"], f.c.classes["Array"]
	grows := func(cls *Class) bool {
		e := cls.lookup(name)
		return e != nil && !e.M.generic() && e.M.Block == nil && growsTypeParams(cls, e)
	}
	onHash, onArr := grows(hash), grows(arr)
	if !onHash && !onArr || slices.ContainsFunc(args, func(a parser.Node) bool { _, sp := a.(*parser.SplatNode); return sp }) {
		return expr{}, false
	}
	f.inDynContainer = true // the [untyped] method may itself call name dynamically (an arithmetic mix): plain dispatch there
	defer func() { f.inDynContainer = false }()
	tmp, r := f.newTmp(), f.newTmp()
	f.emit("var %s any", tmp)
	f.emit("switch %s := rbUnbox(%s).(type) {", r, f.coerce(n, recv, TAny{}))
	branch := func(head string, on expr) {
		f.emit("%s", head)
		var err *compileError
		f.probe(func() { err = catchCompileError(func() { f.genMethodCall(n, on, name, args, nil) }) })
		f.indent++
		if err != nil {
			f.emit("panic(rbNoMethod(%q, %s, false))", name, r)
		} else {
			f.emit("%s = %s", tmp, f.coerce(n, f.genMethodCall(n, on, name, args, nil), TAny{}))
		}
		f.indent--
	}
	if onHash {
		branch("case Hash_Any:", expr{code: r + "._ToAny()", typ: TClass{C: hash, Args: []Type{TAny{}, TAny{}}}})
	}
	if onArr {
		branch("case Array_Any:", expr{code: r + "._ToAny()", typ: TClass{C: arr, Args: []Type{TAny{}}}})
	}
	f.emit("default:")
	f.indent++
	other := f.genDynCall(n, expr{code: r, typ: TAny{}}, name, args)
	f.emit("%s = %s", tmp, other.code)
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: TAny{}}, true
}

// anySpread is args as one []any when any of them is a splat: each splat's elements (an Array's own, else rbSplatAny's
// MRI to_a), the rest in place, keywords last as a Hash (rbDynKw takes them back). ok is false without a splat.
func (f *fctx) anySpread(args []parser.Node) (string, bool) {
	if !slices.ContainsFunc(args, func(a parser.Node) bool { _, ok := a.(*parser.SplatNode); return ok }) {
		return "", false
	}
	var parts, plain []string
	flush := func() {
		if len(plain) > 0 {
			parts = append(parts, "[]any{"+strings.Join(plain, ", ")+"}")
			plain = nil
		}
	}
	for _, a := range args {
		if kh, ok := a.(*parser.KeywordHashNode); ok { // `**kwargs` of an empty Hash passes nothing, as in Ruby
			flush()
			parts = append(parts, "rbKwArgs("+f.anyArgs([]parser.Node{kh})[0]+")")
			continue
		}
		sp, ok := a.(*parser.SplatNode)
		if !ok {
			plain = append(plain, f.anyArgs([]parser.Node{a})...)
			continue
		}
		flush()
		e := f.genExpr(sp.Expression, nil)
		if ac, ok := e.typ.(TClass); ok && ac.C.RubyName == "Array" && len(ac.Args) == 1 {
			if isAny(ac.Args[0]) || f.c.goType(ac.Args[0]) == "any" { // a union element is already Array[any] in Go
				parts = append(parts, "("+e.code+").s")
			} else {
				parts = append(parts, "("+e.code+")._ToAny().s")
			}
			continue
		}
		parts = append(parts, "rbSplatObj("+f.coerce(sp, e, TAny{})+")")
	}
	flush()
	if len(parts) == 1 {
		return parts[0], true
	}
	return "rbArgsCat(" + strings.Join(parts, ", ") + ")", true
}

// anyArgs generates call arguments for an `...any` parameter.
func (f *fctx) anyArgs(args []parser.Node) []string {
	codes := make([]string, 0, len(args))
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			f.errorf(a, "splat arguments in a dynamic call are not supported")
		}
		if kh, ok := a.(*parser.KeywordHashNode); ok { // keywords travel as a Hash; the wrapper's rbDynKw takes them back
			a = &parser.HashNode{Location: kh.Location, Elements: kh.Elements}
		}
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	return codes
}

// ownSendOrName is a gem's `x.send(a, b)` on an untyped x when some class defines its own send: that class's method for
// its objects (BasicSocket#send(mesg, flags), which net-ftp's BufferedSocket forwards OOB data through), Kernel#send's
// call by name for the rest, decided at run time as MRI's method lookup decides it (decision 416).
func (f *fctx) ownSendOrName(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if name != "send" || !isAny(recv.typ) || block != nil || !f.lenient() || f.noOwnSend || slices.ContainsFunc(args, isSplat) {
		return expr{}, false
	}
	var owners []string
	for _, cls := range f.c.classList {
		if m := cls.Methods["send"]; m != nil && !cls.universal && !cls.IsModule && cls.metaOf == nil && m.Owner == cls {
			owners = append(owners, f.c.goType(cls.instance()))
		}
	}
	if len(owners) == 0 {
		return expr{}, false
	}
	slices.Sort(owners)
	owners = slices.Compact(owners)
	r := f.newTmp()
	f.emit("%s := %s", r, f.coerce(n, recv, TAny{}))
	pinned := make([]parser.Node, len(args))
	for i, a := range args {
		t := f.newTmp()
		f.emit("%s := %s", t, f.coerce(a, f.genExpr(a, nil), TAny{}))
		pinned[i] = &exprNode{Node: a, e: expr{code: t, typ: TAny{}}}
	}
	ra := expr{code: r, typ: TAny{}}
	res := f.newTmp()
	f.emit("var %s any", res)
	f.emit("switch %s.(type) {", r)
	f.emit("case %s:", strings.Join(owners, ", "))
	f.indent++
	own := f.genDynCall(n, ra, "send", pinned)
	f.emit("%s = %s", res, f.coerce(n, own, TAny{}))
	f.indent--
	f.emit("default:")
	f.indent++
	f.noOwnSend = true
	byName := f.genSend(n, ra, name, pinned, nil)
	f.noOwnSend = false
	f.emit("%s = %s", res, f.coerce(n, byName, TAny{}))
	f.indent--
	f.emit("}")
	return expr{code: res, typ: TAny{}}, true
}

// genSend compiles send/public_send/__send__. A literal name is an
// ordinary call (send may reach private methods); a computed one switches
// over every method name at run time.
func (f *fctx) genSend(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if lit := literalName(args[0]); lit != "" {
		if name != "public_send" {
			f.implicitCall = true
		}
		e := f.genMethodCall(n, recv, lit, args[1:], block)
		f.implicitCall = false
		return e
	}
	if e, ok := f.ownSendOrName(n, recv, name, args, block); ok {
		return e
	}
	var blockArg string
	if block != nil {
		code, ok := f.dynBlockValue(n, block)
		if !ok || !f.lenient() {
			f.errorf(n, "a block with a computed send is not supported")
		}
		blockArg = code
		f.c.dynBlockAll = true // any block-taking method a computed name reaches (decision 243)
	}
	if blockArg == "" && name != "public_send" {
		if on := f.sendOnClasses(recv.typ); on != nil {
			if e, ok := f.sendSwitch(n, recv, on, args); ok {
				return e
			}
		}
	}
	fn := "rbSendByName"
	if on := f.sendOnClasses(recv.typ); on != nil { // decision 316: only what the receiver's classes and their subclasses answer
		fn = "rbSendOn_" + f.c.noteSendOn(on)
		f.noteSendUses(on, args[0], args[1:])
		if f.m == nil || !f.m.quietDynamic {
			f.warn(n, "dynamic call: %s with a computed name", name)
		}
	} else {
		f.noteDynAll()
		if f.m == nil || !f.m.quietDynamic {
			f.warn(n, "dynamic call: %s with a computed name on %s%s", name, recv.typ, f.everyNameWhy(n, recv))
		}
	}
	how := "rbFCall"
	if name == "public_send" {
		how = "rbCall"
	}
	if _, ok := args[0].(*parser.SplatNode); ok && f.lenient() { // `send(*args, &block)`: the name is the first element
		spread, ok := f.anySpread(args)
		if !ok {
			f.errorf(n, "%s(*args) needs an Array of the name and arguments", name)
		}
		tail := "a_[1:]"
		if blockArg != "" {
			tail = "append(a_[1:len(a_):len(a_)], " + blockArg + ")"
		}
		return expr{code: fmt.Sprintf("func() any { a_ := %s; if len(a_) == 0 { panic(NewArgumentError(Ref(String(\"no method name given\")))) }; return %s(%s, rbConstName(a_[0]), %s, %s...) }()",
			spread, fn, f.coerce(n, recv, TAny{}), how, tail), typ: TAny{}}
	}
	nameExpr := f.genExpr(args[0], nil)
	codes := []string{f.coerce(n, recv, TAny{}), "rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + ")", how}
	if spread, ok := f.anySpread(args[1:]); ok { // `send(m, state, *args)`
		if blockArg != "" {
			spread = "append(" + spread + ", " + blockArg + ")"
		}
		return expr{code: fn + "(" + strings.Join(codes, ", ") + ", " + spread + "...)", typ: TAny{}}
	}
	for _, a := range args[1:] {
		e := f.genExpr(a, nil)
		codes = append(codes, f.coerce(a, e, TAny{}))
	}
	if blockArg != "" {
		codes = append(codes, blockArg)
	}
	return expr{code: fn + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
}

// dynBlockValue is a dynamic call's block as an rbBlockArg (decision 243): a literal block's closure over untyped
// values, or a method's own `&block` passed on (nil when the caller gave none).
func (f *fctx) dynBlockValue(n parser.Node, block parser.Node) (string, bool) {
	switch b := block.(type) {
	case *parser.BlockNode:
		names := f.blockParamList(b.Parameters)
		if names.special() {
			return "", false
		}
		ps := make([]Type, len(names))
		vals := make([]string, len(names))
		for i := range ps {
			ps[i] = TAny{}
			vals[i] = fmt.Sprintf("rbArgAt(xs_, %d)", i)
		}
		if fn, ok := f.rebindableBlock(n, b, false); ok {
			return "rbBlockArg{fn: " + fn + "}", true
		}
		clo := f.genClosure(n, b, &BlockSig{Params: ps, Ret: TAny{}}, map[string]Type{})
		return fmt.Sprintf("rbBlockArg{fn: func(xs_ ...any) any { _, xs_ = rbSelfOr(nil, xs_); %sreturn (%s)(%s) }}", autoSplat(len(names), "xs_"), clo, strings.Join(vals, ", ")), true
	case *parser.BlockArgumentNode:
		if b.Expression == nil {
			return "", false
		}
		if sym, ok := b.Expression.(*parser.SymbolNode); ok { // `emit :tSYMBOL, &:to_sym` on a method only subclasses define (rubocop-ast's rex lexer)
			return "rbBlockArg{fn: " + f.symbolProcAny(sym.Unescaped.Value) + "}", true
		}
		e := f.genExpr(b.Expression, nil)
		if isAny(e.typ) { // `public_send(key, *args, &block)` of a Proc or nil held untyped (Homebrew's cask artifact stanzas)
			return "rbBlockArgAny(" + e.code + ")", true
		}
		fn, opt := e.typ.(TFunc) // the method's own &block, or a Proc value (a tuple's `T.nilable(T.proc.void)`)
		if o, ok := e.typ.(TOpt); ok {
			fn, opt = o.Elem.(TFunc)
			if opt && fn.Rest && len(fn.Params) == 1 && isAny(fn.Params[0]) && isAny(fn.Ret) {
				return "rbBlockArgOf(" + e.code + ")", true
			}
			if opt && !fn.Rest {
				gt := f.c.goType(e.typ)
				deref := strings.Repeat("*", len(gt)-len(strings.TrimLeft(gt, "*"))) + e.code // a Proc value is boxed again
				return fmt.Sprintf("func() rbBlockArg {\n\tif %s == nil {\n\t\treturn rbBlockArg{}\n\t}\n\treturn %s\n}()", e.code, f.typedBlockArg(n, deref, fn)), true
			}
			return "", false
		}
		if opt && fn.Rest && len(fn.Params) == 1 && isAny(fn.Params[0]) && isAny(fn.Ret) {
			if strings.HasPrefix(f.c.goType(e.typ), "*") { // a Proc value is boxed (elftools' `tag_at(i).tap(&block)`)
				return "rbBlockArg{fn: *" + e.code + "}", true
			}
			return "rbBlockArg{fn: " + e.code + "}", true
		}
		if opt && !fn.Rest {
			code := e.code
			if strings.HasPrefix(f.c.goType(e.typ), "*") { // a Proc value is boxed
				code = "*" + code
			}
			return f.typedBlockArg(n, code, fn), true
		}
	}
	return "", false
}

// rebindableBlock is a gem's DSL block in a class body (bindata's `struct :e_ident do ... end`), or nested in one, as
// a dynamic block: its self is whatever instance_eval later gives it, so its own calls are dynamic on that, else the
// class it was written in (decision 324).
func (f *fctx) rebindableBlock(n parser.Node, b *parser.BlockNode, typedSelf bool) (string, bool) {
	if !f.lenient() || !typedSelf && !f.selfClassObj && !isAny(f.selfType) || f.selfCode == "" {
		return "", false
	}
	names := f.blockParamList(b.Parameters)
	if names.special() {
		return "", false
	}
	ps := make([]Type, len(names))
	vals := []string{"s_"}
	for i := range ps {
		ps[i] = TAny{}
		vals = append(vals, fmt.Sprintf("rbArgAt(xs_, %d)", i))
	}
	clo := f.genClosure(n, b, &BlockSig{Params: ps, Ret: TAny{}, Self: TAny{}}, map[string]Type{})
	return fmt.Sprintf("func(xs_ ...any) any { s_, xs_ := rbSelfOr(%s, xs_); %sreturn (%s)(%s) }",
		f.coerce(n, expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}, TAny{}), autoSplat(len(names), "xs_"), clo, strings.Join(vals, ", ")), true
}

// typedBlockArg is a method's own typed `&block` passed on to a dynamic call: its values asserted in, a bound block's
// self first (Homebrew's `def bottle(&block) = stable.bottle(&block)`, a `T.proc.bind`; decision 281).
func (f *fctx) typedBlockArg(n parser.Node, fnCode string, fn TFunc) string {
	ps := fn.Params
	if fn.Self != nil {
		ps = append([]Type{fn.Self}, ps...)
	}
	vals := make([]string, len(ps))
	for i, p := range ps {
		vals[i] = f.coerce(n, expr{code: fmt.Sprintf("rbArgAt(xs_, %d)", i), typ: TAny{}}, p)
	}
	call := fmt.Sprintf("(%s)(%s)", fnCode, strings.Join(vals, ", "))
	body := call + "; return nil"
	if !isVoid(fn.Ret) {
		body = "return " + f.coerce(n, expr{code: call, typ: fn.Ret}, TAny{})
	}
	return fmt.Sprintf("rbBlockArg{fn: func(xs_ ...any) any { %s }, bound: %t}", body, fn.Self != nil)
}

// noteDynAll: a computed name may be any method; from a gem file, or a prelude forwarder (SimpleDelegator's
// respond_to_missing?, which only passes a name on), only names gem and user code answer (decision 235).
func (f *fctx) noteDynAll() {
	f.c.dynAll = true
	if !f.lenient() && (!f.f.isPrelude() || !f.c.pruneGems) {
		f.c.dynAllStrict = true
	}
}

// universalNames answer respond_to? for every object.
var universalNames = map[string]bool{"to_s": true, "inspect": true, "==": true, "!=": true, "!": true,
	"nil?": true, "equal?": true, "class": true, "is_a?": true, "kind_of?": true, "respond_to?": true,
	"send": true, "public_send": true, "hash": true, "then": true, "frozen?": true, "to_json": true}

// genDynRespondTo answers respond_to? when the static type cannot.
func (f *fctx) genDynRespondTo(n parser.Node, recv expr, args []parser.Node) expr {
	r := f.coerce(n, recv, TAny{})
	includeAll := func() string {
		if len(args) < 2 {
			return "false"
		}
		return "rbTruthy(" + f.coerce(args[1], f.genExpr(args[1], nil), TAny{}) + ")"
	}
	if lit := literalName(args[0]); lit != "" {
		if universalNames[lit] {
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}
		}
		f.c.noteRespond(lit)
		return expr{code: "rbResponds" + goMethodName(lit) + "(" + r + ", " + includeAll() + ")", typ: f.cls("Boolean")}
	}
	fn := "rbRespondsByName"
	if on := f.sendOnClasses(recv.typ); on != nil {
		fn = "rbRespondsOn_" + f.c.noteSendOn(on)
	} else {
		f.noteDynAll()
	}
	nameExpr := f.genExpr(args[0], nil)
	return expr{code: fn + "(" + r + ", rbConstName(" + f.coerce(args[0], nameExpr, TAny{}) + "), " + includeAll() + ")", typ: f.cls("Boolean")}
}

// sendOnClasses are the classes a computed send or respond_to? on a receiver of type t can be scoped to (decision
// 316): the value is an instance of one of them or a subclass, so only their method names can answer. A class object
// (a metaclass) counts, with its subclasses' metaclasses; a module (or `Self` in a module body) is the module and
// everything that includes it. nil for an untyped, `T?` or universal receiver (or a union holding one), which keep
// the every-name switch.
func (f *fctx) sendOnClasses(t Type) []*Class {
	var cls *Class
	switch t := t.(type) {
	case TClass:
		cls = t.C
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
	case TOpt:
		return nil // nil answers only Kernel's names, which a class's set would not promise
	case TUnion:
		var out []*Class
		for _, m := range t.Members {
			on := f.sendOnClasses(m)
			if on == nil {
				return nil
			}
			out = append(out, on...)
		}
		return out
	case TAny, TFunc, TNil, TTuple, TVoid:
	}
	if cls == nil || cls.universal {
		return nil
	}
	if cls.IsModule {
		return moduleIncluders(cls, nil)
	}
	return []*Class{cls}
}

// moduleIncluders is mod and every class or module that includes it, transitively.
func moduleIncluders(mod *Class, out []*Class) []*Class {
	if slices.Contains(out, mod) {
		return out
	}
	out = append(out, mod)
	for _, inc := range mod.includers {
		out = moduleIncluders(inc, out)
	}
	return out
}

// universalOnly is Object's (or Kernel's) name when no class of the program defines its own, so a call on any value
// reaches it; nil otherwise.
func (c *Compiler) universalOnly(name string) *entry {
	e := c.classes["Object"].lookup(name)
	if e == nil || !e.Owner.universal {
		return nil
	}
	for _, cls := range c.classList {
		if !cls.universal && cls.Methods[name] != nil {
			return nil
		}
	}
	return e
}

// dynSelfCall reports a self call in a gem method added to a core module, which dispatches dynamically (selfCalls).
func (f *fctx) dynSelfCall(name string) bool {
	if f.owner == nil || !f.owner.IsModule || f.m == nil || !f.c.gemFile(f.m.File) {
		return false
	}
	f.c.selfCalls(f.owner)
	return f.owner.gemSelfCalls[name]
}

// dynEachForward is a gem's `x.each(&block)` on an untyped x passing the method's own closure block on.
func (f *fctx) dynEachForward(n parser.Node, recv expr) expr {
	k := len(f.blockSig.Params)
	f.c.dynEach = true
	xs := make([]string, k)
	for i := range k {
		xs[i] = f.coerce(n, expr{code: fmt.Sprintf("xs_[%d]", i), typ: TAny{}}, f.blockSig.Params[i])
	}
	call := "blk(" + strings.Join(xs, ", ") + ")"
	each, blkT := "rbEachAny", ""
	if isAny(f.blockSig.Ret) && f.lenient() { // the block's value reaches each (prime's `last_value = yield prime`), and each's value the caller
		f.c.dynEachV = true
		each, call, blkT = "rbEachAnyV", "return "+call, " any"
	} else if !isVoid(f.blockSig.Ret) {
		call = "_ = " + call
	}
	if f.m == nil || f.m.Block == nil || !f.m.Block.Optional && !f.m.Block.Untyped {
		return expr{code: fmt.Sprintf("%s(any(%s), %d, func(xs_ ...any)%s { %s })", each, recv.code, k, blkT, call), typ: TAny{}}
	}
	// called without a block (prime's `Prime.instance.each` over a generator a mock made untyped): x.each with none,
	// as MRI's `x.each(&nil)` is (decision 410)
	rt, out := f.newTmp(), f.newTmp()
	f.emit("var %s any = %s", rt, f.coerce(n, recv, TAny{}))
	// a lazy Enumerator over x's each, pulled one value at a time as MRI's is: an eager one collects every value first,
	// which never ends for prime's infinite generator
	without := fmt.Sprintf("rbEnumOf(rbSeq(func(y func(any)) { rbEachAny(%s, 1, func(xs_ ...any) { y(xs_[0]) }) }), %s, \"each\", nil, nil)", rt, rt)
	if k != 1 {
		without = f.coerce(n, f.genDynCall(n, expr{code: rt, typ: TAny{}}, "each", nil), TAny{})
	} else if f.c.blocklessTwin["each"] { // a gem each's own `return self.dup unless block_given?` (prime's generators, decision 418)
		f.c.noteDyn("__each_enum")
		w := "Dyn" + goMethodName("__each_enum")
		without = fmt.Sprintf("func() any { if t_, ok := %s.(interface{ %s(...any) any }); ok { return t_.%s() }; return %s }()", rt, w, w, without)
	}
	f.emit("var %s any", out)
	f.emit("if blk == nil {")
	f.emit("\t%s = %s", out, without)
	f.emit("} else {")
	f.emit("\t%s = %s(%s, %d, func(xs_ ...any)%s { %s })", out, each, rt, k, blkT, call)
	f.emit("}")
	return expr{code: out, typ: TAny{}}
}

// isClassObjHook is the synthesized receiver of a class hook call (hookCall): a class object.
func isClassObjHook(n parser.Node) bool {
	e, ok := n.(*exprNode)
	return ok && e.e.classObj
}

// everyNameWhy: under RUGAROO_UNTYPED_WHY, the root of an untyped receiver whose computed send keeps the every-name switch.
func (f *fctx) everyNameWhy(n parser.Node, recv expr) string {
	call, ok := n.(*parser.CallNode)
	if !ok || !f.c.env.UntypedWhy || !isAny(recv.typ) {
		return ""
	}
	return " [why: " + f.untypedWhy(call.Receiver, 0) + "]"
}

// noteSendUses: `send("convert_#{x}", el)` on known classes reaches only their matching methods, so its args type those params like a plain call (decision 353).
// In user code, a wholly computed name (`send(command.downcase.to_sym, argument)`, net-ftp's fixture server) may reach
// any of their methods taking that many arguments; its args then type only parameters with a default, whose literal
// (`param = :default`) is the weakest evidence of their type (decision 423).
func (f *fctx) noteSendUses(on []*Class, name parser.Node, args []parser.Node) {
	if f.c.uses == nil || slices.ContainsFunc(args, func(a parser.Node) bool {
		switch a.(type) {
		case *parser.SplatNode, *parser.KeywordHashNode, *parser.BlockArgumentNode, *parser.ForwardingArgumentsNode:
			return true
		}
		return false
	}) {
		return
	}
	pre, suf, ok := namePattern(name)
	optionalOnly := !ok                                  // decision 423
	if optionalOnly && (len(args) == 0 || f.lenient()) { // gem code: its dynamic calls type what they reach (decision 331); a gem class's many defaults widened by every computed send broke kramdown's converter
		return
	}
	var types []Type
	seen := map[*Method]bool{}
	var walk func(k *Class)
	walk = func(k *Class) {
		for _, e := range k.methodSet() {
			m := e.M
			if seen[m] || len(m.Name) < len(pre)+len(suf) || !strings.HasPrefix(m.Name, pre) || !strings.HasSuffix(m.Name, suf) {
				continue
			}
			seen[m] = true
			if len(args) > len(m.Params) || slices.ContainsFunc(m.Params[:len(args)], func(p Param) bool { return p.Rest || p.Keyword || p.KwRest }) {
				continue
			}
			if types == nil {
				types = make([]Type, len(args))
				for i, a := range args {
					types[i] = f.probeType(a, nil)
				}
			}
			for i, a := range args {
				if types[i] != nil && (!optionalOnly || m.Params[i].Default != nil) {
					f.noteUse(m.Params[i], a, types[i])
				}
			}
		}
		for _, sub := range k.Subclasses {
			walk(sub)
		}
	}
	for _, k := range on {
		walk(k)
	}
}

// sendSwitch compiles a pattern-named send on known classes to a Go switch of direct typed calls, one per matching method that compiles (decision 354).
func (f *fctx) sendSwitch(n parser.Node, recv expr, on []*Class, args []parser.Node) (expr, bool) {
	rest := args[1:]
	if slices.ContainsFunc(rest, func(a parser.Node) bool {
		switch a.(type) {
		case *parser.SplatNode, *parser.KeywordHashNode, *parser.BlockArgumentNode, *parser.ForwardingArgumentsNode:
			return true
		}
		return false
	}) {
		return expr{}, false
	}
	pre, suf, ok := namePattern(args[0])
	if !ok {
		return expr{}, false
	}
	call := func(name string, as []parser.Node) expr {
		f.implicitCall = true // send reaches private methods
		defer func() { f.implicitCall = false }()
		return f.genMethodCall(n, recv, name, as, nil)
	}
	var names []string
	var res Type
	complete := true // every matching name is an arm: the default is only NoMethodError
	for _, name := range f.c.sendOnNames(on) {
		if len(name) < len(pre)+len(suf) || !strings.HasPrefix(name, pre) || !strings.HasSuffix(name, suf) {
			continue
		}
		if slices.ContainsFunc(on, func(k *Class) bool { return k.lookup(name) == nil }) { // a subclass's own method: the default's
			complete = false
			continue
		}
		var t Type
		var err *compileError
		f.probe(func() { err = f.try(func() { t = call(name, rest).typ }) })
		if err != nil {
			complete = false
			continue
		}
		names = append(names, name)
		if t != nil && !isVoid(t) {
			res = joinOrNil(res, t)
		}
	}
	if len(names) == 0 {
		return expr{}, false
	}
	if complete && f.definesMethodMissing(on) {
		complete = false
	}
	if !complete {
		f.noteSendUses(on, args[0], rest) // the default's names still take the call's types (decision 353)
	}
	if res == nil {
		res = TNil{}
	}
	if !isSimpleGo(recv.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, f.materialize(recv))
		recv.code = tmp
	}
	pinned := f.pinArgs(args, len(args))
	nameExpr := f.genExpr(pinned[0], nil)
	key := f.newTmp()
	f.emit("%s := rbConstName(%s)", key, f.coerce(args[0], nameExpr, TAny{}))
	tmp := f.newTmp()
	goT := f.c.goType(res)
	if strings.TrimLeft(goT, "*") == "" { // arms answering only nil
		goT = "any"
	}
	f.emit("var %s %s", tmp, goT)
	f.emit("switch %s {", key)
	f.switches++
	for _, name := range names {
		f.emit("case %q:", name)
		saved := f.enterBlock()
		f.indent++
		e := call(name, pinned[1:])
		switch {
		case e.noreturn:
			f.emitExprStmt(n, e)
		case isVoid(e.typ) || e.code == "":
			f.emitExprStmt(n, e)
			f.emit("%s = %s", tmp, nilCode(goT))
		default:
			f.emit("%s = %s", tmp, f.coerce(n, e, res))
		}
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	f.indent++
	if complete {
		f.emit("panic(rbNoMethod(%s, %s, false))", key, f.coerce(n, recv, TAny{}))
	} else {
		if f.m == nil || !f.m.quietDynamic {
			f.warn(n, "dynamic call: send with a computed name (the %s*%s names that are not direct calls)", pre, suf)
		}
		codes := []string{f.coerce(n, recv, TAny{}), key, "rbFCall"}
		for i, a := range rest {
			codes = append(codes, f.coerce(a, f.genExpr(pinned[i+1], nil), TAny{}))
		}
		dyn := expr{code: "rbSendOn_" + f.c.noteSendOn(on) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
		f.emit("%s = %s", tmp, f.coerce(n, dyn, res))
	}
	f.indent--
	f.switches--
	f.emit("}")
	return expr{code: tmp, typ: res}, true
}

// joinOrNil is joinUnion with nil as the empty start.
func joinOrNil(a, b Type) Type {
	if a == nil {
		return b
	}
	return joinUnion(a, b)
}

// definesMethodMissing reports a class in on, or a subclass, whose method_missing is not the prelude's.
func (f *fctx) definesMethodMissing(on []*Class) bool {
	var walk func(k *Class) bool
	walk = func(k *Class) bool {
		if e := k.lookup("method_missing"); e != nil && e.M.File != nil && !e.M.File.isPrelude() {
			return true
		}
		return slices.ContainsFunc(k.Subclasses, walk)
	}
	return slices.ContainsFunc(on, walk)
}

// namePattern is the literal text around an interpolated method name; ok is false when there is none.
func namePattern(n parser.Node) (pre, suf string, ok bool) {
	var parts []parser.Node
	switch n := n.(type) {
	case *parser.InterpolatedStringNode:
		parts = n.Parts
	case *parser.InterpolatedSymbolNode:
		parts = n.Parts
	default:
		return "", "", false
	}
	if len(parts) < 2 { // Prism merges adjacent literal text, so only the ends can be literal
		return "", "", false
	}
	if s, isStr := parts[0].(*parser.StringNode); isStr {
		pre = s.Unescaped.Value
	}
	if s, isStr := parts[len(parts)-1].(*parser.StringNode); isStr {
		suf = s.Unescaped.Value
	}
	return pre, suf, pre+suf != ""
}

// noteDynUses records, during an inference round, a gem's dynamic call's arguments as uses of each gem method of that
// name the call may reach: liquid's `context.handle_error(e, line)` on an untyped context passes the StandardError a
// rescue caught, where the one typed call passes a MemoryError (decision 331). An untyped argument adds nothing.
func (f *fctx) noteDynUses(name string, args []parser.Node) {
	if f.c.uses == nil || !f.lenient() || slices.ContainsFunc(args, isSplat) {
		return
	}
	if k := len(args) - 1; k >= 0 {
		if en, ok := args[k].(*exprNode); ok && en.blockArg { // the block goes to the method's &block, not a parameter (rubocop-ast's `emit ss.matched, &:to_sym`)
			args = args[:k]
		}
	}
	for _, m := range f.c.gemMethodsNamed(name) {
		if len(args) > len(m.Params) {
			continue
		}
		for i, a := range args {
			p := m.Params[i]
			if p.Pending == "" || p.Rest || p.Keyword {
				break
			}
			if _, kw := a.(*parser.KeywordHashNode); kw {
				break
			}
			if t := f.probeType(a, nil); t != nil && !isAny(t) {
				f.c.uses[p.Pending] = append(f.c.uses[p.Pending], paramUse{typ: t, file: f.f, off: a.GetLocation().StartOffset, dyn: true})
			}
		}
	}
}
