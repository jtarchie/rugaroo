package compiler

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func (f *fctx) genCase(n *parser.CaseNode, t tail) {
	if n.Predicate == nil {
		f.genStmt(f.c.caseAsIf(n), t)
		return
	}
	typeSwitch := true
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		for _, cond := range wn.Conditions {
			switch c := cond.(type) {
			case *parser.NilNode:
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				if cls := f.classRef(c); cls == nil || slices.Contains([]string{"TrueClass", "FalseClass", "NilClass", "Proc"}, cls.RubyName) {
					typeSwitch = false
				}
			default:
				typeSwitch = false
			}
		}
	}
	if typeSwitch {
		f.genTypeCase(n, t)
		return
	}
	subj := f.genExpr(n.Predicate, nil)
	tmp := f.newTmp()
	if isNil(subj.typ) { // `case nil`: Go has no `x := nil`
		f.emit("var %s any", tmp)
		f.emitExprStmt(n.Predicate, subj)
	} else {
		f.emit("%s := %s", tmp, f.materialize(subj))
	}
	held := expr{code: tmp, typ: subj.typ, bin: subj.bin, binDeps: subj.binDeps} // a binary String stays binary (decision 352)
	st := stripOpt(subj.typ)
	u, isUnion := st.(TUnion)
	_, localSubj := n.Predicate.(*parser.LocalVariableReadNode)
	strUnion := isUnion && (f.lenient() || localSubj) && slices.ContainsFunc(u.Members, func(m Type) bool { return isClass(m, "String") })
	if f.lastMatch != "" && caseHasRegexp(n, f.whenRegexp) && (isClass(st, "String") || isAny(st) || strUnion) { // `when /re/` sets $~ for the arm: tests in order, as an if chain (decision 244)
		var subjNode parser.Node = &exprNode{Node: n.Predicate, e: held}
		if lv, ok := n.Predicate.(*parser.LocalVariableReadNode); ok {
			subjNode = lv // a local's own reads: each test narrows it for its arm (`when Array` then o.partition)
			f.emit("_ = %s", tmp)
		}
		isCls := func(c parser.Node) bool { return f.classRef(c) != nil }
		f.genStmt(caseAsMatchIf(n, subjNode, !isClass(st, "String"), isCls, f.whenRegexp), t)
		return
	}
	if f.genIntCase(n, t, held) {
		return
	}
	// each arm's test is built before the switch: one may need statements (a union subject's type switch, a ternary), which cannot sit between cases
	arms, pre := make([]string, len(n.Conditions)), make([]string, len(n.Conditions))
	for i, w := range n.Conditions {
		build := func() {
			pre[i] = f.capture(func() {
				var conds []string
				for _, cond := range w.(*parser.WhenNode).Conditions {
					conds = append(conds, f.caseEqq(cond, held))
				}
				arms[i] = strings.Join(conds, " || ")
			})
		}
		if i == 0 {
			build()
			continue
		}
		saved := f.enterBlock() // may land in a default: a local first assigned in its test is then hoisted, not scoped to the case
		build()
		f.leaveBlock(saved)
	}
	f.genCaseSwitch(n, t, 0, arms, pre)
}

// intCaseMin is the fewest arms worth a Go switch on the value: below it the chain of tests is as good.
const intCaseMin = 4

// genIntCase is a `case` of an Integer (or Integer?) subject whose every `when` is a distinct Integer literal, as one
// Go `switch` on the value, which the Go compiler turns into a jump table or a binary search: a Ragel scanner's
// `case _lex_trans_actions[_trans]` has hundreds of arms, tested one by one otherwise (decision 467). A nil subject
// takes a value no arm names, so it reaches the else.
func (f *fctx) genIntCase(n *parser.CaseNode, t tail, subj expr) bool {
	if !isClass(stripOpt(subj.typ), "Integer") || len(n.Conditions) < intCaseMin {
		return false
	}
	seen := map[int64]bool{}
	labels := make([]string, len(n.Conditions))
	spare := int64(math.MinInt64)
	for i, w := range n.Conditions {
		var vals []string
		for _, cond := range w.(*parser.WhenNode).Conditions {
			lit, ok := cond.(*parser.IntegerNode)
			if !ok || lit.Location.Length == 0 {
				return false
			}
			v, err := strconv.ParseInt(strings.ReplaceAll(f.f.text(lit.Location), "_", ""), 0, 64) // Go's reading: 0d and out-of-range literals stay with genInteger
			if err != nil || seen[v] || v == math.MaxInt64 {
				return false
			}
			seen[v] = true
			spare = max(spare, v+1)
			vals = append(vals, strconv.FormatInt(v, 10))
		}
		labels[i] = strings.Join(vals, ", ")
	}
	key := subj.code
	if isOpt(subj.typ) {
		key = f.newTmp()
		f.emit("%s := Integer(%d)", key, spare)
		f.emit("if %s.ok {", subj.code)
		f.emit("\t%s = %s.v", key, subj.code)
		f.emit("}")
	}
	f.emit("switch %s {", key)
	f.switches++
	for i, w := range n.Conditions {
		f.emit("case %s:", labels[i])
		saved := f.enterBlock()
		f.indent++
		f.genWhenArm(w.(*parser.WhenNode), t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	if n.ElseClause != nil {
		f.genStmts(n.ElseClause.Statements, t)
	} else {
		f.emptyTail(n, t)
	}
	f.indent--
	f.leaveBlock(saved)
	f.switches--
	f.emit("}")
	return true
}

// genCaseSwitch nests an arm whose test needed statements (pre) in the previous switch's default, so a later `when` runs only if no earlier one matched, as MRI's.
func (f *fctx) genCaseSwitch(n *parser.CaseNode, t tail, from int, arms, pre []string) {
	f.buf.WriteString(pre[from])
	to := from + 1
	for to < len(arms) && pre[to] == "" {
		to++
	}
	f.emit("switch {")
	f.switches++
	for i := from; i < to; i++ {
		f.emit("case %s:", arms[i])
		saved := f.enterBlock()
		f.indent++
		f.genWhenArm(n.Conditions[i].(*parser.WhenNode), t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	switch {
	case to < len(arms):
		f.genCaseSwitch(n, t, to, arms, pre)
	case n.ElseClause != nil:
		f.genStmts(n.ElseClause.Statements, t)
	default:
		f.emptyTail(n, t)
	}
	f.indent--
	f.leaveBlock(saved)
	f.switches--
	f.emit("}")
}

// caseAsIf is `case; when a, b then x; else y; end` as `if a || b then x else y end`.
func (c *Compiler) caseAsIf(n *parser.CaseNode) *parser.IfNode {
	return c.rewrite(n, func() parser.Node {
		var els parser.Node
		if n.ElseClause != nil {
			els = n.ElseClause
		}
		for i := len(n.Conditions) - 1; i >= 0; i-- {
			w := n.Conditions[i].(*parser.WhenNode)
			pred := w.Conditions[0]
			for _, c := range w.Conditions[1:] {
				pred = &parser.OrNode{Location: c.GetLocation(), Left: pred, Right: c}
			}
			els = &parser.IfNode{Location: w.Location, Predicate: pred, Statements: w.Statements, Subsequent: els}
		}
		return els
	}).(*parser.IfNode)
}

// caseHasRegexp reports a when with a pattern isRx accepts.
func caseHasRegexp(n *parser.CaseNode, isRx func(parser.Node) bool) bool {
	for _, w := range n.Conditions {
		if slices.ContainsFunc(w.(*parser.WhenNode).Conditions, isRx) {
			return true
		}
	}
	return false
}

// whenRegexp is a when's Regexp literal, or a constant holding one (liquid's `when SINGLE_QUOTED_STRING,
// DOUBLE_QUOTED_STRING then $1`, decision 331): its === is a match that sets $~.
func (f *fctx) whenRegexp(cond parser.Node) bool {
	switch cond.(type) {
	case *parser.RegularExpressionNode, *parser.InterpolatedRegularExpressionNode:
		return true
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return isClass(f.probeType(cond, nil), "Regexp")
	}
	return false
}

// caseAsMatchIf is `case s when /re/ ... when X ... else ... end` as `if re =~ s ... elsif X === s ... else ... end`,
// the regexp tests through =~ so the matching arm reads its own $~.
func caseAsMatchIf(n *parser.CaseNode, subj parser.Node, untyped bool, isCls, isRx func(parser.Node) bool) parser.Node {
	var tail parser.Node
	if n.ElseClause != nil {
		tail = n.ElseClause
	}
	for i := len(n.Conditions) - 1; i >= 0; i-- {
		w := n.Conditions[i].(*parser.WhenNode) //nolint:forcetypeassert // a case's conditions are whens
		var pred parser.Node
		for _, cond := range w.Conditions {
			loc := cond.GetLocation()
			call := func(recv parser.Node, name string) parser.Node {
				return &parser.CallNode{Location: loc, Receiver: recv, Name: name, Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{subj}}}
			}
			test := call(cond, "===")
			if isCls(cond) { // `when Array`: is_a?, which narrows the subject for its arm
				test = &parser.CallNode{Location: loc, Receiver: subj, Name: "is_a?", Arguments: &parser.ArgumentsNode{Location: loc, Arguments: []parser.Node{cond}}}
			}
			switch c := cond.(type) {
			case *parser.SplatNode: // ponytail: `when *list` as list.include?(s), == per element where MRI's is ===
				test = call(c.Expression, "include?")
			default:
				if !isRx(cond) {
					break
				}
				test = call(cond, "=~")
				if untyped { // Regexp#=== is false for a non-String, where =~ raises
					test = &parser.AndNode{Location: loc, Left: call(&parser.ConstantReadNode{Location: loc, Name: "String"}, "==="), Right: test}
				}
			}
			if pred == nil {
				pred = test
			} else {
				pred = &parser.OrNode{Location: cond.GetLocation(), Left: pred, Right: test}
			}
		}
		tail = &parser.IfNode{Location: w.Location, Predicate: pred, Statements: w.Statements, Subsequent: tail}
	}
	return tail
}

// caseEqq renders a `when`'s `cond === subj` as a Go bool: is_a? for a
// class, the condition's own === when its class defines one (at run time
// when it is untyped or T?), else ==, Object#==='s default.
func (f *fctx) caseEqq(cond parser.Node, subj expr) string {
	if sp, ok := cond.(*parser.SplatNode); ok { // `when *LIST`: any element's === matches
		e := f.genExpr(sp.Expression, nil)
		at, ok := e.typ.(TClass)
		if !ok || at.C.RubyName != "Array" {
			f.errorf(cond, "when *%s needs an Array", f.f.text(sp.Expression.GetLocation()))
		}
		list := f.materialize(e)
		x := f.newTmp()
		eqq := f.caseEqq(&exprNode{Node: sp, e: expr{code: x, typ: at.Args[0]}}, subj)
		return fmt.Sprintf("slices.ContainsFunc(%s.s, func(%s %s) bool { return %s })", list, x, f.c.goType(at.Args[0]), eqq)
	}
	if cls := f.classRef(cond); cls != nil {
		// not isACheck: its discard of a folded subject would land in the previous arm's body
		return f.isA(cond, subj, cls)
	}
	condT := f.probeType(cond, subj.typ)
	c := classOf(stripOpt(condT))
	if !isAny(condT) && (c == nil || c.lookup("===") == nil) {
		return "bool(" + f.genMethodCall(cond, subj, "==", []parser.Node{cond}, nil).code + ")"
	}
	e := f.genExpr(cond, nil)
	arg := []parser.Node{&exprNode{e: subj}}
	if isAny(condT) || isOpt(condT) {
		e = expr{code: f.coerce(cond, e, TAny{}), typ: TAny{}}
		return "rbTruthy(" + f.genDynCall(cond, e, "===", arg).code + ")"
	}
	return "bool(" + f.genMethodCall(cond, e, "===", arg, nil).code + ")"
}

func (f *fctx) genTypeCase(n *parser.CaseNode, t tail) {
	subj := f.genExpr(n.Predicate, nil)
	var subjLocal *local
	if lv, ok := n.Predicate.(*parser.LocalVariableReadNode); ok {
		subjLocal = f.scope.lookup(lv.Name)
	} else {
		subjLocal = f.attrLocal(n.Predicate) // an ivar or attribute reader narrows like a local (decision 148)
	}
	code := f.coerce(n.Predicate, subj, TAny{})
	// a Go type switch needs an interface operand: box a @go_type value (Integer, String)
	if cls, ok := subj.typ.(TClass); !isOpt(subj.typ) && f.c.goType(subj.typ) != "any" && (!ok || !cls.C.isStruct()) {
		code = "any(" + code + ")"
	}
	// a fresh name, not the local's: default and multi-class arms still see the local as declared
	name := f.newTmp()
	numSubj := name // what the `when Numeric` arm reads: a switch with no other case may lose its variable
	if slices.ContainsFunc(n.Conditions, func(w parser.Node) bool { return f.numericWhen(w.(*parser.WhenNode), subj.typ) != nil }) {
		numSubj = f.newTmp()
		f.emit("%s := %s", numSubj, code)
		code = numSubj
	}
	f.emit("switch %s := %s.(type) {", name, code)
	f.switches++
	hasNil := false
	u, isUnion := subj.typ.(TUnion)
	var remaining []Type // a union subject's members no arm has taken yet (decision 150)
	if isUnion {
		remaining = slices.Clone(u.Members)
	}
	listed := map[string]bool{} // an earlier arm already takes these Go types: Go rejects a repeat, and Ruby's first match wins anyway
	var numArm *parser.WhenNode // `when Numeric` asks the ancestry table in default: a case naming all five classes would keep BigDecimal and Complex in every program
	for _, w := range n.Conditions {
		wn := w.(*parser.WhenNode)
		if cls := f.numericWhen(wn, subj.typ); cls != nil {
			if numArm == nil {
				numArm = wn
				goTypes, _, _ := f.whenClass(cls)
				for _, g := range goTypes {
					listed[g] = true
				}
			}
			continue
		}
		arm := f.typeCaseArm(wn, subj.typ, isUnion, &remaining)
		hasNil = hasNil || arm.hasNil
		cases, armType, convert := arm.cases, arm.typ, arm.convert
		cases = slices.DeleteFunc(cases, func(c string) bool { return listed[c] })
		if len(cases) == 0 {
			continue // only modules the subject statically lacks, or classes an earlier arm takes: never matches
		}
		for _, c := range cases {
			listed[c] = true
		}
		if len(wn.Conditions) != 1 {
			convert = ""
		}
		slices.Sort(cases) // Go rejects a type listed twice, e.g. `when nil, Object`
		f.emit("case %s:", strings.Join(slices.Compact(cases), ", "))
		saved := f.enterBlock()
		f.indent++
		armName, view := name, ""
		if convert != "" {
			armName, view = name+convert, name // converted where read, like is_a? narrowing
		}
		if subjLocal != nil && (len(wn.Conditions) == 1 || arm.exact) {
			f.applyNarrow([]narrowInfo{{local: subjLocal, typ: armType, code: armName, view: view}})
		}
		f.genWhenArm(wn, t)
		f.indent--
		f.leaveBlock(saved)
	}
	f.emit("default:")
	saved := f.enterBlock()
	f.indent++
	f.emit("_ = %s", name)
	if isUnion && len(remaining) == 0 && n.ElseClause == nil {
		// exhaustive: every member has its arm, so the case never falls through to nil (decision 150)
		f.emit("panic(\"rugaroo: no union member matched\")")
	} else {
		if isUnion && subjLocal != nil {
			f.applyNarrow(f.remainingNarrow(subjLocal, remaining)) // else holds the members no arm took
		}
		f.genCaseDefault(n, t, subj.typ, subjLocal, hasNil, numArm, numSubj)
	}
	f.indent--
	f.leaveBlock(saved)
	f.switches--
	f.emit("}")
}

// genBranch is an if/else branch's statements. In gem code a branch that does not compile (concurrent-ruby's
// `if Concurrent.on_jruby?` body calling `java.util...`) raises NotImplementedError if reached, as a when arm does,
// rather than costing its whole method (decision 251).
func (f *fctx) genBranch(n parser.Node, t tail) {
	if !f.branchLenient || n == nil { // the last resort of a gem method that otherwise does not compile
		f.genStmts(n, t)
		return
	}
	err := f.tryClean(func() { f.genStmts(n, t) })
	if err != nil {
		f.emit("panic(NewNotImplementedError(Ref(String(%q))))", "rugaroo: a branch does not compile: "+err.Error())
	}
}

// genWhenArm is a type switch arm's statements. In gem code an arm that does not compile (one the subject's inferred type
// says cannot match, as optparse's `when Array` on a String-typed option) raises NotImplementedError if reached,
// rather than costing its whole method (decision 244).
func (f *fctx) genWhenArm(wn *parser.WhenNode, t tail) {
	if !f.lenient() {
		f.genStmts(wn.Statements, t)
		return
	}
	err := f.tryClean(func() { f.genStmts(wn.Statements, t) })
	if err != nil {
		f.emit("panic(NewNotImplementedError(Ref(String(%q))))", "rugaroo: a when arm does not compile: "+err.Error())
	}
}

// whenArm is one `when` of a type switch: its Go cases, the subject's type
// there and how it reads as that type.
type whenArm struct {
	cases   []string
	typ     Type
	convert string
	exact   bool // a union subject's arm, holding exactly the members it took (decision 150)
	hasNil  bool
}

// typeCaseArm builds a `when` arm. On a union subject a class takes the
// members that are one, by their own Go types, so the arm is typed as
// those members and they leave *remaining; a class narrower than a member
// is tested as for any subject.
func (f *fctx) typeCaseArm(wn *parser.WhenNode, subjT Type, isUnion bool, remaining *[]Type) whenArm {
	arm := whenArm{typ: TAny{}, exact: isUnion}
	var took []Type
	for _, cond := range wn.Conditions {
		switch c := cond.(type) {
		case *parser.NilNode:
			arm.cases = append(arm.cases, "nil")
			arm.typ, arm.hasNil = TNil{}, true
			if isUnion {
				took = append(took, takeMembers(remaining, isNil)...)
			}
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
			cls := f.classRef(c)
			if isUnion {
				if ms := takeMembers(remaining, func(m Type) bool { mc := classOf(m); return mc != nil && mc.isSubclassOf(cls) }); len(ms) > 0 {
					for _, m := range ms {
						arm.cases = append(arm.cases, f.c.goType(m))
					}
					took = append(took, ms...)
					continue
				}
				arm.exact = false
			}
			if cls.IsModule && (!f.c.isNumericMod(cls) || !f.numericAtRunTime(subjT)) && f.moduleIsA(c, subjT, cls) == "false" {
				continue
			}
			if isClass(subjT, "Bignum") && cls.Name == "Integer" { // an Integer to Ruby (decision 372)
				cls = classOf(subjT)
			}
			var goTypes []string
			goTypes, arm.typ, arm.convert = f.whenClass(cls)
			arm.cases = append(arm.cases, goTypes...)
		}
	}
	arm.exact = arm.exact && len(took) > 0
	if arm.exact {
		arm.typ, arm.convert = unionOf(took...), ""
	}
	return arm
}

// takeMembers removes from *remaining the members mine holds for and returns them.
func takeMembers(remaining *[]Type, mine func(Type) bool) []Type {
	var took []Type
	for _, m := range *remaining {
		if mine(m) {
			took = append(took, m)
		}
	}
	*remaining = slices.DeleteFunc(*remaining, mine)
	return took
}

// remainingNarrow narrows a union case subject to the members no arm took.
func (f *fctx) remainingNarrow(v *local, remaining []Type) []narrowInfo {
	nw, ok := f.narrowUnion(v, func(m Type) bool { return slices.ContainsFunc(remaining, func(r Type) bool { return typeEq(r, m) }) })
	if !ok {
		return nil
	}
	return []narrowInfo{nw}
}

// genCaseDefault is a type switch's default arm: the `when Numeric` arm, asked of the ancestry table, then else.
func (f *fctx) genCaseDefault(n *parser.CaseNode, t tail, subjT Type, subjLocal *local, hasNil bool, numArm *parser.WhenNode, numSubj string) {
	var elseSaved string
	if numArm != nil {
		f.emit("if rbKindOf(%s, %d) {", numSubj, f.c.classID(f.c.classes["Numeric"]))
		inner := f.enterBlock()
		f.indent++
		if subjLocal != nil {
			f.applyNarrow([]narrowInfo{{local: subjLocal, typ: f.cls("Numeric"), code: numSubj}})
		}
		f.genStmts(numArm.Statements, t)
		f.indent--
		f.leaveBlock(inner)
		f.emit("} else {")
		elseSaved = f.enterBlock()
		f.indent++
	}
	if o, ok := subjT.(TOpt); ok && hasNil && subjLocal != nil && !isAny(o.Elem) {
		f.applyNarrow([]narrowInfo{{local: subjLocal, typ: o.Elem}})
	}
	if n.ElseClause != nil {
		f.genStmts(n.ElseClause.Statements, t)
	} else {
		f.emptyTail(n, t)
	}
	if numArm != nil {
		f.indent--
		f.leaveBlock(elseSaved)
		f.emit("}")
	}
}

// numericWhen is Numeric when wn is `when Numeric` alone on a subject whose class only the run time knows.
func (f *fctx) numericWhen(wn *parser.WhenNode, subj Type) *Class {
	if len(wn.Conditions) != 1 {
		return nil
	}
	if _, ok := subj.(TUnion); ok {
		return nil // a union's Numeric members are taken by their Go types (decision 150)
	}
	if cls := f.classRef(wn.Conditions[0]); f.c.isNumericMod(cls) && f.numericAtRunTime(subj) {
		return cls
	}
	return nil
}

// whenClass is the type-switch case for `when cls`: the Go types it lists,
// the arm's type, and the conversion that gives the arm that type.
func (f *fctx) whenClass(cls *Class) ([]string, Type, string) {
	switch {
	case cls.isTemplate(): // any instance: the arm keeps the value untyped (decision 308)
		return []string{cls.anyName()}, TAny{}, ""
	case len(cls.TypeParams) > 0:
		args := make([]Type, len(cls.TypeParams))
		for i := range args {
			args[i] = TAny{}
		}
		return []string{cls.anyName()}, TClass{C: cls, Args: args}, "._ToAny()"
	case cls.universal: // nil is an Object too, but a nil interface misses `case any:`
		return []string{f.c.goType(TClass{C: cls}), "nil"}, TClass{C: cls}, ""
	case f.c.isNumericMod(cls): // its includers are the prelude's number classes, each a Go type of its own
		var goTypes []string
		for _, k := range f.c.classList {
			if !k.IsModule && k.metaOf == nil && k.isSubclassOf(cls) {
				goTypes = append(goTypes, f.c.goType(TClass{C: k}))
			}
		}
		return goTypes, TClass{C: cls}, ""
	}
	return []string{f.c.goType(TClass{C: cls})}, TClass{C: cls}, ""
}
