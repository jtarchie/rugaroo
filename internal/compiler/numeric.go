package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// numTower orders the number classes as MRI's coerce converts between them: a mix of two runs as the higher one (decision 142); Bignum exists only under --int-overflow=promote (decision 372).
var numTower = []string{"Integer", "Bignum", "Rational", "Float", "BigDecimal", "Complex"}

// numLevel is c's place in numTower, -1 for any other class.
func numLevel(c *Class) int {
	if c == nil {
		return -1
	}
	return slices.Index(numTower, c.Name)
}

// towerOps coerce mixed operands; not `**`, whose class in MRI depends on the values (2 ** Rational(1, 2) is a Float).
var towerOps = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true, "modulo": true, "remainder": true, "div": true, "divmod": true,
	"fdiv": true, "quo": true, "<=>": true, "<": true, "<=": true, ">": true, ">=": true, "step": true,
}

// numericTower: a mix no twin takes converts its lower operand up numTower and runs typed, as MRI's coerce; Comparable's go through rbNum, as clamp answers the bound itself. Arguments are generated once and handed back as exprNodes, since probing them first made nested arithmetic exponential.
func (f *fctx) numericTower(n parser.Node, recv expr, e *entry, args []parser.Node, block parser.Node) (expr, []parser.Node, bool) {
	a := classOf(recv.typ)
	la := numLevel(a)
	name := e.M.Name
	viaCmp := e.M.Owner == f.c.classes["Comparable"]
	if la < 0 || len(args) == 0 || !viaCmp && !towerOps[name] {
		return expr{}, args, false
	}
	self := map[string]Type{"Self": recv.typ}
	xs, nodes := f.genNumArgs(e.M, self, args)
	if xs == nil {
		return expr{}, args, false
	}
	f.nilNumArgs(e.M, recv.typ, xs, nodes)
	f.curBlock = block
	if tw := f.overload(e, recv.typ, nodes); tw != nil {
		if len(args) == 1 && classTwins(name) && tw.M.Name != "__"+overloadBase(name)+"_"+strconv.Itoa(len(args)) && !strings.HasSuffix(tw.M.Name, "_enum") {
			f.curBlock = nil
			return expr{}, nodes, false // a twin for the argument's class: MRI's own mix
		}
		e = tw // `r.step(2)` is __step_enum: its parameters are the ones to convert to
	}
	f.curBlock = nil
	m := e.M
	if len(args) > len(m.Params) || (block != nil) != (m.Block != nil) {
		return expr{}, nodes, false
	}
	for i := range args {
		if m.Params[i].Rest || !typeEq(subst(m.Params[i].Type, self), TClass{C: a}) {
			return expr{}, nodes, false
		}
	}
	top, mixed, intFloat := la, false, isNumeric(recv.typ)
	for _, x := range xs {
		if l := numLevel(classOf(x.typ)); l >= 0 && l != la {
			mixed, top, intFloat = true, max(top, l), intFloat && isNumeric(x.typ)
		}
	}
	if !mixed || intFloat || top > la && (len(args) != 1 || block != nil) {
		return expr{}, nodes, false
	}
	if viaCmp {
		codes := make([]string, len(xs))
		for i, x := range xs {
			codes[i] = f.coerce(args[i], x, TAny{})
		}
		return f.rbNumCall(n, m, f.coerce(n, recv, TAny{}), codes), nil, true
	}
	target := f.c.classes[numTower[top]]
	if target.Name == "Bignum" && f.bignumLacks(name) {
		return expr{}, nodes, false // Bignum would run Integer's (bignumFallback), which converts back up: the argument converts down instead
	}
	if top == la {
		return f.callEntry(n, e, recv, f.towerArgs(xs, args, target), block), nil, true
	}
	return f.genMethodCall(n, f.numConvert(n, recv, target), name, f.towerArgs(xs, args, target), block), nil, true
}

// towerArgs are the arguments converted up to target.
func (f *fctx) towerArgs(xs []expr, args []parser.Node, target *Class) []parser.Node {
	nodes := make([]parser.Node, len(xs))
	for i, x := range xs {
		nodes[i] = &exprNode{Node: args[i], e: f.numConvert(args[i], x, target)}
	}
	return nodes
}

// genNumArgs generates plain arguments once, hinted as numericMix does by m's parameter types; nil for a splat or keywords.
func (f *fctx) genNumArgs(m *Method, self map[string]Type, args []parser.Node) ([]expr, []parser.Node) {
	for _, x := range args {
		switch x.(type) {
		case *parser.SplatNode, *parser.KeywordHashNode:
			return nil, nil
		}
	}
	xs := make([]expr, len(args))
	nodes := make([]parser.Node, len(args))
	for i, x := range args {
		var hint Type
		if i < len(m.Params) && !m.Params[i].Rest {
			hint = subst(m.Params[i].Type, self)
		}
		xs[i] = f.genExpr(x, hint)
		nodes[i] = &exprNode{Node: x, e: xs[i]}
	}
	return xs, nodes
}

// checkNumeric: Numeric's methods only type calls on a Numeric value (Go any), so each number class must define or undefine every one, or a call would reach a body that raises.
func (c *Compiler) checkNumeric() {
	num := c.classes["Numeric"]
	if num == nil {
		return
	}
	for _, name := range numTower {
		cls := c.classes[name]
		if cls == nil {
			continue
		}
		for _, m := range num.MethodList {
			if e := cls.lookup(m.Name); e != nil && e.Owner == num && !m.File.isPrelude() && m.Kind == kindDef {
				cp := c.copyAlias(cls, m, m.Name) // `class Numeric; def blank? = false; end`: every number has it, as MRI's inheritance gives
				cp.Owner = cls
				continue
			}
			if e := cls.lookup(m.Name); e != nil && e.Owner == num {
				c.errorAt(cls.srcPos, "%s must define or undef Numeric#%s", cls.RubyName, m.Name)
			}
		}
	}
}

// numericIterArgs is numericTower for an iterator call (`Rational(1, 3).step(1) { }`): lower arguments convert, an Integer receiver with a Float one widens.
func (f *fctx) numericIterArgs(e *entry, recv expr, args []parser.Node) (*entry, expr, []parser.Node) {
	a := classOf(recv.typ)
	la := numLevel(a)
	if la < 0 || !towerOps[e.M.Name] || len(args) > len(e.M.Params) {
		return e, recv, args
	}
	xs := make([]expr, len(args))
	top := la
	f.probe(func() {
		for i, x := range args {
			if _, splat := x.(*parser.SplatNode); !splat {
				xs[i] = f.genExpr(x, nil)
				top = max(top, numLevel(classOf(xs[i].typ)))
			}
		}
	})
	target := a
	if top > la {
		fe := f.c.classes["Float"].lookup(e.M.Name)
		if a.Name != "Integer" || numTower[top] != "Float" || fe == nil || !fe.M.Iterator {
			return e, recv, args
		}
		e, target, recv = fe, fe.Owner, f.numConvert(nil, recv, f.c.classes["Float"])
	}
	out := slices.Clone(args)
	self := map[string]Type{"Self": TClass{C: target}}
	for i, x := range args {
		if l := numLevel(classOf(xs[i].typ)); l >= 0 && typeEq(subst(e.M.Params[i].Type, self), TClass{C: target}) {
			out[i] = &exprNode{Node: x, e: f.numConvert(x, f.genExpr(x, nil), target)}
		}
	}
	return e, recv, out
}

// numConvert converts number x to class target, higher in numTower; a BigDecimal has no Complex form here.
func (f *fctx) numConvert(n parser.Node, x expr, target *Class) expr {
	from := classOf(x.typ)
	if numLevel(from) < 0 || from == target {
		return x
	}
	name := map[string]string{"Bignum": "__to_big", "Rational": "to_r", "Float": "to_f", "BigDecimal": "to_d", "Complex": "to_c"}[target.Name]
	switch {
	case target.Name == "BigDecimal" && from.Name == "Rational":
		name = "__to_d" // to_d needs digits; MRI's coerce takes the BigDecimal's
	case target.Name == "Complex" && from.Name == "BigDecimal":
		f.errorf(n, "a BigDecimal does not mix with a Complex (rugaroo's Complex parts are Integer, Rational or Float; decision 142)")
	}
	return f.genMethodCall(n, x, name, nil, nil)
}

// numericMix compiles a call on an Integer or Float whose numeric
// parameter gets the other class, as MRI's coerce does: the classes' own
// operators widen the Integer side to Float (still typed and unboxed), and
// Comparable's methods run on rbNum, since clamp hands back the winning
// argument itself. Arguments are generated once and passed on as exprNodes.
// Any other call is plain callEntry.
func (f *fctx) numericMix(n parser.Node, recv expr, e *entry, args []parser.Node, block parser.Node) expr {
	if op, ok := intCmpOps[e.M.Name]; ok && len(args) == 1 && f.lenient() && isClass(recv.typ, "Integer") && anyOrUnion(f.probeType(args[0], nil)) {
		// gem code: an Integer against an untyped value that is a Float when it runs compares as numbers, where the
		// implicit conversion truncated it or raised (prism's `common_whitespace < result`, Float::MAX; decision 496)
		x := f.genExpr(args[0], nil)
		return expr{code: fmt.Sprintf("rbIntCmpAny('%c', %s, %s)", op, recv.code, f.coerce(args[0], x, TAny{})), typ: f.cls("Boolean")}
	}
	x, args, ok := f.numericTower(n, recv, e, args, block)
	if ok {
		return x
	}
	m := e.M
	if !isNumeric(recv.typ) || block != nil || len(args) == 0 || len(args) != len(m.Params) {
		return f.callEntry(n, e, recv, args, block)
	}
	self := map[string]Type{"Self": recv.typ}
	for i, p := range m.Params {
		if _, splat := args[i].(*parser.SplatNode); splat || p.Rest || !isNumeric(subst(p.Type, self)) {
			return f.callEntry(n, e, recv, args, block)
		}
	}
	xs := make([]expr, len(args))
	nodes := make([]parser.Node, len(args))
	mixed := false
	for i, a := range args {
		pt := subst(m.Params[i].Type, self)
		xs[i] = f.genExpr(a, pt)
		mixed = mixed || isNumeric(xs[i].typ) && !typeEq(xs[i].typ, pt)
		nodes[i] = &exprNode{Node: a, e: xs[i]}
	}
	if !mixed {
		return f.callEntry(n, e, recv, nodes, nil)
	}
	if m.Owner == f.c.classes["Comparable"] {
		codes := make([]string, len(xs))
		for i, x := range xs {
			codes[i] = f.coerce(args[i], x, TAny{})
		}
		return f.rbNumCall(n, m, f.coerce(n, recv, TAny{}), codes)
	}
	fe := f.c.classes["Float"].lookup(m.Name)
	if fe == nil || len(fe.M.Params) != len(args) {
		f.errorf(n, "%s#%s with an Integer and a Float is not supported", classOf(recv.typ).RubyName, m.Name)
	}
	widen := func(x expr) expr {
		if isClass(x.typ, "Integer") {
			return expr{code: "Float(" + x.code + ")", typ: f.cls("Float")}
		}
		return x
	}
	for i, x := range xs {
		nodes[i] = &exprNode{Node: args[i], e: widen(x)}
	}
	return f.callEntry(n, fe, widen(recv), nodes, nil)
}

// anyOrUnion reports an untyped value or a union (a local written a Float and an Integer).
func anyOrUnion(t Type) bool {
	_, u := t.(TUnion)
	return u || isAny(t)
}

// intCmpOps are Integer's ordering operators and rbIntCmpAny's code for each.
var intCmpOps = map[string]byte{"<": '<', "<=": 'l', ">": '>', ">=": 'g'}

// rbNumCall calls Comparable method m with Self = rbNum, one Go type for
// Integer and Float values (recv and args are untyped Go code).
func (f *fctx) rbNumCall(n parser.Node, m *Method, recv string, args []string) expr {
	boxed := make([]string, 0, 1+len(args))
	for _, a := range append([]string{recv}, args...) {
		boxed = append(boxed, "rbNum{"+a+"}")
	}
	code := freeFuncName(m) + "[rbNum](" + strings.Join(boxed, ", ") + ")"
	if v, ok := m.Ret.(TVar); ok && v.Name == "Self" {
		return expr{code: code + ".v", typ: TAny{}}
	}
	if m.generic() || mentionsVar(m.Ret) {
		f.errorf(n, "%s with an Integer and a Float is not supported", m.Name)
	}
	return expr{code: code, typ: m.Ret}
}

// isFloatInfinity is the constant Float::INFINITY.
func isFloatInfinity(n parser.Node) bool {
	cp, ok := n.(*parser.ConstantPathNode)
	if !ok || cp.Name == nil || *cp.Name != "INFINITY" {
		return false
	}
	p, ok := cp.Parent.(*parser.ConstantReadNode)
	return ok && p.Name == "Float"
}
