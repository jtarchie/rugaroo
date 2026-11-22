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
	ips := f.interpParts(n.Parts, nil)
	if f.interpAppend(ips) {
		return expr{code: "String(" + ips[0].code + ")", typ: f.cls("String")}
	}
	parts := make([]string, len(ips))
	for i, p := range ips {
		parts[i] = p.code
	}
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

// interpPart is one piece of an interpolation: its Go String expression, and what interpAppend needs to build it in place.
type interpPart struct {
	code string
	kind byte        // 'l' a literal, 's' a String value, 'i' an Integer with the prelude's to_s, 'o' anything else
	int  string      // kind 'i': the Integer expression, written as digits
	node parser.Node // an embedded part's expression; nil for a literal
	held bool        // already read into a temporary
}

// interpParts appends the Go string pieces of an interpolation's parts.
// Adjacent literals (`"a" "#{b}"`) nest an InterpolatedStringNode as a part.
func (f *fctx) interpParts(ps []parser.Node, parts []interpPart) []interpPart {
	for _, p := range ps {
		switch p := p.(type) {
		case *parser.StringNode:
			parts = append(parts, interpPart{code: strconv.Quote(p.Unescaped.Value), kind: 'l'})
		case *parser.InterpolatedStringNode:
			parts = f.interpParts(p.Parts, parts)
		case *parser.EmbeddedStatementsNode:
			st := p.Statements
			if st == nil || len(st.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			mark := f.buf.Len()
			e := f.genExpr(st.Body[0], nil)
			ip := interpPart{code: f.toS(st.Body[0], e), kind: 'o', node: st.Body[0]}
			switch {
			case isClass(e.typ, "String"):
				ip.kind = 's'
			case f.plainIntToS(e, ip.code):
				ip.kind, ip.int = 'i', e.code
			}
			f.pinInterp(mark, parts, ip)
			parts = append(parts, ip)
		default:
			f.c.unsupported(f.f, p)
		}
	}
	return parts
}

// pinInterp: MRI reads each part at its turn, Go orders only calls and hoists statements, so earlier parts a later one may change go to temporaries (decision 489).
func (f *fctx) pinInterp(mark int, parts []interpPart, later interpPart) {
	emitted := f.emittedSince(mark)
	if !emitted && later.kind != 'o' && pureRead(later.node) {
		return
	}
	f.insertAt(mark, func() {
		for i, p := range parts {
			if p.kind == 'l' || p.held || !emitted && !f.callVisible(p.node) || appendsInPlace(later.node, p.node) || f.localKept(p.node, later.node) {
				continue
			}
			if p.kind == 'i' {
				if pinnable(p.int) {
					t := f.newTmp()
					f.emit("%s := %s", t, p.int)
					parts[i].int, parts[i].code, parts[i].held = t, "Integer.ToS("+t+")", true // plainIntToS's spelling
				}
				continue
			}
			if pinnable(p.code) {
				t := f.newTmp()
				f.emit("%s := %s", t, p.code)
				parts[i].code, parts[i].held = t, true
			}
		}
	})
}

// pureRead: no part read after n can see a change made by evaluating it.
func pureRead(n parser.Node) bool {
	switch n.(type) {
	case *parser.LocalVariableReadNode, *parser.InstanceVariableReadNode, *parser.GlobalVariableReadNode,
		*parser.ClassVariableReadNode, *parser.ConstantReadNode, *parser.SelfNode:
		return true
	}
	return sideEffectFree(n)
}

// callVisible: n reads memory a call can change; only a block or lambda can assign a local of the current scope.
func (f *fctx) callVisible(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.InstanceVariableReadNode, *parser.GlobalVariableReadNode, *parser.ClassVariableReadNode:
		return true
	case *parser.LocalVariableReadNode:
		return n.Depth > 0 || f.scopeRoot == nil || writtenInBlock(f.scopeRoot, n.Name, false)
	}
	return false
}

// localKept: earlier reads a local of this scope that neither later nor any block assigns, so later's statements cannot change it.
func (f *fctx) localKept(earlier, later parser.Node) bool {
	lv, ok := earlier.(*parser.LocalVariableReadNode)
	return ok && !f.callVisible(lv) && !anyNode(later, func(x parser.Node) bool { return assignsLocalHere(x, lv.Name) })
}

// appendsInPlace: later appends to (or setbytes) the local or ivar that earlier reads.
func appendsInPlace(later, earlier parser.Node) bool {
	var name string
	switch e := earlier.(type) {
	case *parser.LocalVariableReadNode:
		name = e.Name
	case *parser.InstanceVariableReadNode:
		name = e.Name
	default:
		return false
	}
	return anyNode(later, func(x parser.Node) bool {
		c, ok := x.(*parser.CallNode)
		if !ok || !strAppendNames[c.Name] && c.Name != "setbyte" {
			return false
		}
		switch r := c.Receiver.(type) {
		case *parser.LocalVariableReadNode:
			return r.Name == name
		case *parser.InstanceVariableReadNode:
			return r.Name == name
		}
		return false
	})
}

// plainIntToS: e is a 64-bit Integer whose to_s compiled to the prelude's own (no program or gem redefines Integer#to_s),
// so its digits can be written straight into the interpolation's buffer.
func (f *fctx) plainIntToS(e expr, code string) bool {
	if !isClass(e.typ, "Integer") || f.intMode() == "promote" || code != "Integer.ToS("+e.code+")" {
		return false
	}
	cls := f.c.classes["Integer"]
	if cls == nil {
		return false
	}
	m := cls.lookup("to_s")
	return m != nil && m.M.File != nil && m.M.File.isPrelude()
}

// interpAppend builds an interpolation of literals, Strings and Integers in one stack buffer (decision 488): each
// Integer's digits go in with strconv.AppendInt, where `+ Integer.ToS(i) +` allocated a String per Integer and then
// the concatenation. It emits the appends as statements, after any the parts emitted, so the parts run in their order
// (an emitted statement makes an earlier argument a temporary, as any hoisted code does), and leaves the buffer's name
// in parts[0].code; the String conversion copies it once, a new String as the + chain's is. No helper or closure: no
// function is added to the program. A leading String value keeps the rbConcat path, which extends a buffer rbStrCat
// made in place (`s = "#{s}#{i}"` in a loop).
func (f *fctx) interpAppend(parts []interpPart) bool {
	ints := 0
	for _, p := range parts {
		switch p.kind {
		case 'i':
			ints++
		case 'o':
			return false
		}
	}
	if ints == 0 || parts[0].kind == 's' {
		return false
	}
	arr, buf := f.newTmp(), f.newTmp()
	f.emit("var %s [64]byte", arr)
	f.emit("%s := %s[:0]", buf, arr)
	for _, p := range parts {
		switch {
		case p.kind == 'i':
			f.emit("%s = strconv.AppendInt(%s, int64(%s), 10)", buf, buf, p.int)
		case p.code != `""`:
			f.emit("%s = append(%s, %s...)", buf, buf, p.code)
		}
	}
	parts[0].code = buf
	return true
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
