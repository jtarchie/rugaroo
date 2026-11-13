package compiler

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// definedIf is `defined?`'s answer kind (a frozen literal) when the Go bool cond holds at run time, else nil.
func (f *fctx) definedIf(kind, cond string) expr {
	f.c.strLits[kind] = true
	t := TOpt{Elem: f.cls("String")}
	return expr{code: fmt.Sprintf("func() %s { if %s { return Ref[String](%q) }; return nil }()", f.c.goType(t), cond, kind), typ: t}
}

// genSugar renders syntax that reduces to a literal or a plain read:
// `defined?` (answered at compile time), `__LINE__`, `{ a: }`, `:"#{x}"`.
func (f *fctx) genSugar(n parser.Node) expr {
	switch n := n.(type) {
	case *parser.DefinedNode:
		if _, ok := n.Value.(*parser.YieldNode); ok && f.m.optionalBlockLocal() != "" { // the block may be missing: answered at run time
			c, _ := f.genCond(&parser.LocalVariableReadNode{Name: f.m.optionalBlockLocal(), Location: n.Location})
			return f.definedIf("yield", c)
		}
		switch v := n.Value.(type) {
		case *parser.InstanceVariableReadNode: // decision 153: the object's assigned bit, or nil for an ivar self's class never has
			var iv *Ivar
			if f.owner != nil && !f.owner.universal {
				iv = f.c.findIvar(f.owner, v.Name)
			}
			if iv == nil {
				return expr{code: "nil", typ: TNil{}}
			}
			set := ivarIsSet(f.selfCode, iv)
			switch {
			case set == "" && f.c.ivarBitsMarked:
				f.errorf(v, "rugaroo: defined?(%s) in a class it does not mark (decision 153)", v.Name)
			case set == "": // discovery, before markIvarBits: only the type matters
				set = "true"
			}
			return f.definedIf("instance-variable", set)
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			// a constant main may read before its assignment runs (BEGIN, a method called first) is defined once it has
			var k *Const
			_ = catchCompileError(func() { _, k = f.c.lookupConst(f.f, v, f.lex) }) // a private path: definedKind's nil
			if k != nil && k.guarded {
				return f.definedIf("constant", constSet(k))
			}
		}
		kind := f.definedKind(n.Value)
		if kind == "" {
			return expr{code: "nil", typ: TNil{}}
		}
		f.c.strLits[kind] = true // MRI's answers are frozen literals
		return expr{code: strconv.Quote(kind), typ: f.cls("String"), lit: true}
	case *parser.SourceLineNode:
		return expr{code: strconv.Itoa(f.f.line(n.Location.StartOffset)), typ: f.cls("Integer"), lit: true}
	case *parser.ImplicitNode: // `{ a: }` reads the local or method a
		return f.genExpr(n.Value, nil)
	case *parser.InterpolatedSymbolNode:
		s := f.genInterp(&parser.InterpolatedStringNode{Location: n.Location, Parts: n.Parts})
		return expr{code: "Symbol(" + s.code + ")", typ: f.cls("Symbol")}
	}
	f.c.unsupported(f.f, n)
	return expr{}
}

// genLiteral handles the leaf expressions.
func (f *fctx) genLiteral(n parser.Node) (expr, bool) {
	switch n := n.(type) {
	case *parser.StringNode:
		f.c.strLits[n.Unescaped.Value] = true
		return expr{code: strconv.Quote(n.Unescaped.Value), typ: f.cls("String"), lit: true}, true
	case *parser.InterpolatedStringNode:
		return f.genInterp(n), true
	case *parser.XStringNode:
		if f.f.isPrelude() {
			f.errorf(n, "%%x{} is only allowed as the whole body of a prelude method")
		}
		f.c.strLits[n.Unescaped.Value] = true
		cmd := expr{code: strconv.Quote(n.Unescaped.Value), typ: f.cls("String"), lit: true}
		return f.kernelCall(n, "__backtick", []parser.Node{&exprNode{Node: n, e: cmd}}), true
	case *parser.InterpolatedXStringNode:
		cmd := f.genInterp(&parser.InterpolatedStringNode{Location: n.Location, Parts: n.Parts})
		return f.kernelCall(n, "__backtick", []parser.Node{&exprNode{Node: n, e: cmd}}), true
	case *parser.SourceFileNode:
		f.c.strLits[f.f.Name] = true
		return expr{code: strconv.Quote(f.f.Name), typ: f.cls("String"), lit: true}, true
	case *parser.DefinedNode, *parser.SourceLineNode, *parser.ImplicitNode, *parser.InterpolatedSymbolNode:
		return f.genSugar(n), true
	case *parser.GlobalVariableReadNode:
		return f.genGlobalRead(n), true
	case *parser.NumberedReferenceReadNode, *parser.BackReferenceReadNode:
		return f.genMatchRef(n), true
	case *parser.GlobalVariableWriteNode:
		return f.genGlobalWrite(n), true
	case *parser.GlobalVariableOperatorWriteNode, *parser.GlobalVariableOrWriteNode, *parser.GlobalVariableAndWriteNode:
		return f.genGlobalUpdate(n), true
	case *parser.IntegerNode:
		return f.genInteger(n), true
	case *parser.FloatNode:
		if n.Value == 0 && math.Signbit(n.Value) {
			// Go's constant -0.0 is +0; Ruby's is negative zero.
			return expr{code: "rbNegZero()", typ: f.cls("Float")}, true
		}
		return expr{code: f.f.text(n.Location), typ: f.cls("Float"), lit: true}, true
	case *parser.RationalNode:
		return expr{code: fmt.Sprintf("rbRat(%d, %d)", n.Numerator, n.Denominator), typ: f.cls("Rational")}, true
	case *parser.ImaginaryNode:
		im := f.genExpr(n.Numeric, nil)
		return expr{code: "(&Complex{Integer(0), " + f.coerce(n, im, TAny{}) + "})", typ: f.cls("Complex")}, true
	case *parser.TrueNode:
		return expr{code: "true", typ: f.cls("Boolean"), lit: true}, true
	case *parser.FalseNode:
		return expr{code: "false", typ: f.cls("Boolean"), lit: true}, true
	case *parser.NilNode:
		return expr{code: "nil", typ: TNil{}}, true
	case *parser.SelfNode:
		return expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}, true
	case *parser.SymbolNode:
		return expr{code: "Symbol(" + strconv.Quote(n.Unescaped.Value) + ")", typ: f.cls("Symbol")}, true
	case *withMember:
		return f.genMethodCall(n, n.recv, n.name, nil, nil), true
	case *exprNode:
		return n.e, true
	case *parser.ClassNode, *parser.ModuleNode:
		if f.c.hoisted[n] { // declared at compile time (decision 451)
			return f.genHoisted(n, nil), true
		}
	}
	return expr{}, false
}

// genInterp compiles an interpolated string (`"a#{b}"`) to a Go string concatenation of its parts.
func (f *fctx) genInterp(n *parser.InterpolatedStringNode) expr {
	parts := f.interpParts(n.Parts, nil)
	if len(parts) == 0 {
		return expr{code: `""`, typ: f.cls("String"), lit: true}
	}
	allLit := true
	for _, p := range parts {
		if !strings.HasPrefix(p, `"`) {
			allLit = false
		}
	}
	if allLit {
		return expr{code: "String(" + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
	}
	if len(parts) == 1 { // "#{x}" is a new String, never x or x.to_s itself
		return expr{code: "rbStrClone(" + parts[0] + ")", typ: f.cls("String")}
	}
	// Go's + hands back its lone non-empty operand itself, so `"#{a}#{""}"` would be a (and frozen with it): two
	// non-empty literal parts can never be alone, one is checked for by identity, and none goes through rbConcat
	var lits []string
	for _, p := range parts {
		if strings.HasPrefix(p, `"`) && p != `""` {
			lits = append(lits, p)
		}
	}
	switch len(lits) {
	case 0:
		return expr{code: "rbConcat(" + strings.Join(parts, ", ") + ")", typ: f.cls("String")}
	case 1:
		return expr{code: "rbNewStr(" + lits[0] + ", " + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
	}
	return expr{code: "(" + strings.Join(parts, " + ") + ")", typ: f.cls("String")}
}

// interpParts appends the Go string pieces of an interpolation's parts.
// Adjacent literals (`"a" "#{b}"`) nest an InterpolatedStringNode as a part.
func (f *fctx) interpParts(ps []parser.Node, parts []string) []string {
	for _, p := range ps {
		switch p := p.(type) {
		case *parser.StringNode:
			parts = append(parts, strconv.Quote(p.Unescaped.Value))
		case *parser.InterpolatedStringNode:
			parts = f.interpParts(p.Parts, parts)
		case *parser.EmbeddedStatementsNode:
			st := p.Statements
			if st == nil || len(st.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			e := f.genExpr(st.Body[0], nil)
			parts = append(parts, f.toS(st.Body[0], e))
		default:
			f.c.unsupported(f.f, p)
		}
	}
	return parts
}

func (f *fctx) toS(n parser.Node, e expr) string {
	if isClass(e.typ, "String") {
		return e.code
	}
	r := f.genMethodCall(n, e, "to_s", nil, nil)
	if !isClass(r.typ, "String") { // a dynamic to_s (a module's `"mkdir -p #{self}"`, brew's ObserverPathnameExtension)
		return "rbToS(" + f.coerce(n, r, TAny{}) + ")"
	}
	return r.code
}

// genInteger is an Integer literal: Go's spelling, a lenient gem's out-of-range one clamped (decision 35).
func (f *fctx) genInteger(n *parser.IntegerNode) expr {
	code := strings.ReplaceAll(f.f.text(n.Location), "_", "")
	if n.Location.Length == 0 { // a synthesized literal (lowerProcArgs)
		code = strconv.FormatInt(n.Value, 10)
	}
	if digits := strings.TrimPrefix(code, "-"); len(digits) > 1 && (digits[1] == 'd' || digits[1] == 'D') {
		// Ruby's 0d decimal prefix has no Go spelling; 0x/0o/0b/0 carry over.
		code = code[:len(code)-len(digits)] + strings.TrimLeft(digits[2:], "0")
		if strings.TrimPrefix(code, "-") == "" {
			code += "0"
		}
	}
	_, err := strconv.ParseInt(code, 0, 64)
	if errors.Is(err, strconv.ErrRange) && f.intMode() == "promote" { // --int-overflow=promote (decisions 372, 381)
		return expr{code: "rbBigLit(" + strconv.Quote(code) + ")", typ: f.cls("Bignum")}
	}
	if errors.Is(err, strconv.ErrRange) && f.lenient() { // a gem's unsigned 64-bit bound (bindata's Uint64 clamp): the nearest Integer, as no Integer exceeds it
		f.warn(n, "Integer literal %s does not fit in 64 bits; it is clamped to the Integer range (decision 35)", code)
		bound := "9223372036854775807" // math.MaxInt64 spelled out: a gem local named math would shadow the package
		if strings.HasPrefix(code, "-") {
			bound = "-9223372036854775808"
		}
		return expr{code: "Integer(" + bound + ")", typ: f.cls("Integer")}
	}
	if errors.Is(err, strconv.ErrRange) { // raised where it runs, as an overflowing operation is: mspec's bignum_value helper is never called (decision 361)
		f.warn(n, "Integer literal %s does not fit in 64 bits: there is no Bignum, so evaluating it raises RangeError (decision 361)", code)
		return expr{code: "rbBigLiteral(" + strconv.Quote(code) + ")", typ: f.cls("Integer")}
	}
	return expr{code: code, typ: f.cls("Integer"), lit: true}
}
