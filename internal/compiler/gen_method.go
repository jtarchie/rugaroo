package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genBody runs the two-pass body generation into f.buf.
func (f *fctx) genBody(body parser.Node, params []*local, t tail, prologue func()) {
	final := f.buf
	savedRoot, savedProofs, savedBufs := f.scopeRoot, f.appendProofs, f.ivarBufs
	f.scopeRoot, f.appendProofs, f.ivarBufs = body, nil, nil
	defer func() { f.scopeRoot, f.appendProofs, f.ivarBufs = savedRoot, savedProofs, savedBufs }()
	f.unset = maybeUnset(body, params)
	if f.intMode() == "promote" {
		f.promoted = growthLocals(body)
	}
	savedNil := f.nilElemLocals
	defer func() { f.nilElemLocals = savedNil }()
	f.nilElemLocals = nil
	if f.lenient() {
		f.nilElemLocals = nilElemLocals(body)
	}
	closures, lambdaClosure, loops, begins, unionArm, retryFlag, retryDepth := f.closures, f.lambdaClosure, f.loops, f.begins, f.unionArm, f.retryFlag, f.retryDepth
	rbScope, rbFrames, breakCatch := f.rbScope, slices.Clone(f.rbFrames), f.breakCatch
	fresh := func() {
		f.locals = map[localKey]*localInfo{}
		for _, p := range params {
			f.locals[localKey{methodScope, p.name}] = &localInfo{declBlock: "", typ: p.typ, annotated: true, reads: 1, noHoist: true, param: true}
		}
	}
	run := func(pass int) {
		f.pass = pass
		// a run a compile error unwound (a gem method's first attempt, decision 251) leaves its nesting behind
		f.closures, f.lambdaClosure, f.loops, f.begins, f.unionArm, f.retryFlag, f.retryDepth = closures, lambdaClosure, loops, begins, unionArm, retryFlag, retryDepth
		// the Ruby block it was in too: a retry that kept it declared the method's locals under that block, and a later
		// block's read of one found no such local (rubocop-ast's generated matchers; decision 444)
		f.rbScope, f.rbFrames, f.breakCatch = rbScope, slices.Clone(rbFrames), breakCatch
		f.iterBreakValue, f.iterAsValue, f.argcPass, f.storing = false, false, "", 0
		f.buf = &strings.Builder{}
		f.tmp, f.blockCtr, f.block = 0, 0, ""
		f.scope = &scope{vars: map[string]*local{}}
		for _, p := range params {
			f.scope.vars[p.name] = &local{name: p.name, goName: p.goName, typ: p.typ, declared: true, info: f.locals[localKey{methodScope, p.name}]}
		}
		if prologue != nil {
			prologue()
		}
		for _, p := range params { // after the prologue, which binds a rest param from rest_
			if w := f.paramWiden[p.name]; w != nil { // its widened copy, which the body reads and writes (decision 241)
				v := f.scope.vars[p.name]
				goName := p.goName + "W_"
				f.emit("var %s %s = %s", goName, f.c.goType(w), f.coerce(body, expr{code: p.goName, typ: p.typ}, w))
				f.emit("_ = %s", goName)
				v.goName, v.typ = goName, w
				if v.info != nil {
					v.info.typ = w
				}
			}
		}
		f.lastMatch = ""
		if readsLastMatch(body) {
			f.lastMatch = "rbLM_"
			f.emit("var rbLM_ %s", f.c.goType(TOpt{Elem: f.cls("MatchData")}))
			f.emit("_ = rbLM_") // the read may sit in a gem statement that was skipped (decision 205)
		}
		f.hoistLocals(methodScope)
		if f.owner != nil && f.m != nil && f.m.Node != nil && body == f.m.Node.Body {
			for _, name := range slices.Sorted(maps.Keys(f.owner.scratchIvars)) {
				f.emit("var %s any", scratchIvar(name))
				f.emit("_ = %s", scratchIvar(name))
			}
		}
		f.bodyStmts = body
		f.genStmts(body, t)
	}
	fresh()
	whole := f.m != nil && f.m.Node != nil && body == f.m.Node.Body && f.lenient() // a gem method's own body: poisonable (decision 205)
	switch {
	case f.discover:
		run(0)
	case whole && f.m.poison != "":
		f.poisonedBody(t, prologue)
	case whole:
		err := catchCompileError(func() { f.analyze(fresh, run) })
		if err != nil && f.c.dryTry {
			// refineIvars is trying an ivar typing: a gem method it stops compiling vetoes the typing, as a user method's
			// error does, instead of being poisoned for good (rubocop's `@options = {}` typed Hash[Symbol, Boolean] by its
			// one typed store; decision 444)
			panic(*err)
		}
		if err != nil && f.c.ivarsDiscovered { // again with each branch that does not compile raising if reached (decision 251)
			nw := len(f.c.Warnings)
			f.branchLenient = true
			fresh()
			err = catchCompileError(func() { f.analyze(fresh, run) })
			f.branchLenient = false
			if err != nil {
				f.c.dropWarnings(nw)
			}
		}
		if err != nil {
			f.m.poison = err.Error()
			fresh()
			f.poisonedBody(t, prologue)
			if f.c.ivarsDiscovered {
				f.c.warn(f.f, f.m.Node, "%s does not compile, so calling it raises NotImplementedError: %s (decision 205)", f.m, err.Error())
			} else {
				f.m.poison = "" // an ivar discovery has yet to type can fail it now: poison this run's output only (decision 220)
			}
		}
	default:
		f.analyze(fresh, run)
	}
	final.WriteString(f.buf.String())
	f.buf = final
}

// poisonedBody is a gem method body rugaroo cannot compile: it raises NotImplementedError naming why, where MRI would
// have run it, and the rest of the program still builds (decision 205).
func (f *fctx) poisonedBody(t tail, prologue func()) {
	f.buf = &strings.Builder{}
	f.scope = &scope{vars: map[string]*local{}}
	if prologue != nil && catchCompileError(prologue) != nil { // an iterator's wrapper opens here; a default that fails is skipped too
		f.buf = &strings.Builder{}
		if f.m.Iterator { // the wrapper's close still follows (genFunc): open it again
			ps := make([]string, len(f.m.Block.Params))
			for i, p := range f.m.Block.Params {
				ps[i] = f.c.goType(p)
			}
			f.emit("return func(yield func(%s) bool) {", strings.Join(ps, ", "))
		}
	}
	f.emit("panic(NewNotImplementedError(Ref(String(%q))))", "rugaroo: "+f.m.String()+" does not compile: "+f.m.poison)
	if t.kind != tailNone && t.typ == nil && t.types != nil {
		t.record(TAny{})
	}
}

func (c *Compiler) newFctx(f *File, owner *Class, m *Method) *fctx {
	fc := &fctx{c: c, f: f, owner: owner, m: m, buf: &strings.Builder{}, nilGuarded: map[parser.Node]bool{}}
	switch {
	case owner == nil:
		fc.selfType = TClass{C: c.classes["Object"]}
		fc.selfCode = "rb_main"
	case owner.GoType != "":
		fc.selfType = owner.instance()
		fc.selfCode = "self"
	default:
		fc.selfType = TVar{Name: "Self"}
		fc.selfCode = "self"
	}
	if m != nil && m.SelfType != nil {
		fc.selfType = m.SelfType
	}
	if m != nil && valueInit(m) {
		fc.selfCode = "(*self)" // its fields are written in place; anything else sees the value
	}
	if m != nil {
		fc.ret = m.Ret
		fc.iterator = m.Iterator
		fc.blockSig = m.Block
		fc.lex = m.Scope
	}
	return fc
}

func (c *Compiler) paramLocals(m *Method) []*local {
	ps := make([]*local, 0, len(m.Params))
	for i, p := range m.Params {
		t := p.Type
		if p.Rest {
			t = TClass{C: c.classes["Array"], Args: []Type{p.Type}}
		}
		goName := goParamName(p.Name, i)
		if p.Rest || p.Out {
			goName = goLocalName(p.Name) // bound from rest_ or the out pointer in the prologue
		}
		ps = append(ps, &local{name: p.Name, goName: goName, typ: t, declared: true})
	}
	if name := m.optionalBlockLocal(); name != "" {
		ps = append(ps, &local{name: name, goName: optBlockGo, typ: optBlockType(m), declared: true})
	}
	return ps
}

// optBlockGo is the Go variable of a def's optional block as a Proc? (optionalBlockLocal).
const optBlockGo = "blkOpt_"

func optBlockType(m *Method) Type {
	return TOpt{Elem: TFunc{Params: m.Block.Params, Ret: m.Block.Ret, Proc: true, Self: m.Block.Self, Rest: m.Block.Rest, Src: m.Block.Src}}
}

func (c *Compiler) emitMethod(m *Method) {
	c.keepGoing(func() { c.emitMethod0(m) }) // RUGAROO_KEEP_GOING: every body's error in one run
}

func (c *Compiler) emitMethod0(m *Method) {
	cls := m.Owner
	if m.Kind == kindSynth {
		c.emitSynth(m)
		return
	}
	if m.Kind == kindAttrReader {
		iv := c.findIvar(cls, m.Attr)
		if m.Ret != nil && c.goType(m.Ret) != c.goType(iv.Type) { // a gem reader typed apart from its inherited ivar (`Exception` over @cause's `Exception?`)
			f := c.newFctx(m.File, cls, m)
			code := f.coerce(nil, expr{code: ivarPath("self", iv), typ: iv.Type}, m.Ret)
			c.w("func (self %s) %s() %s { return %s }\n\n", c.recvType(cls), m.GoName, c.goType(m.Ret), code)
			return
		}
		c.w("func (self %s) %s() %s { return %s }\n\n", c.recvType(cls), m.GoName, c.goType(iv.Type), ivarPath("self", iv)) // a module's ivar too (decision 291)
		return
	}
	if m.Kind == kindAttrWriter {
		iv := c.findIvar(cls, m.Attr)
		mark := ivarAssigned("self", iv)
		if mark != "" {
			mark = "; " + mark
		}
		if len(m.Params) == 1 && valOpt(m.Params[0].Type) && typeEq(stripOpt(m.Params[0].Type), iv.Type) {
			c.w("func (self *%s) %s(v %s) {\n\tif !v.ok {\n\t\t%s = rbZero[%s]()\n\t} else {\n\t\t%s = v.v\n\t}%s\n}\n\n",
				cls.Name, m.GoName, c.goType(m.Params[0].Type), ivarPath("self", iv), c.goType(iv.Type), ivarPath("self", iv), strings.ReplaceAll(mark, "; ", "\n\t"))
			return
		}
		if len(m.Params) == 1 && isOpt(m.Params[0].Type) && !isOpt(iv.Type) && c.goType(m.Params[0].Type) == "*"+c.goType(iv.Type) {
			// a gem's writer takes T? (decision 323) of an ivar its class body set non-nil (Livecheck's @processed_urls):
			// nil stores the slot's zero value, MRI's nil there
			c.w("func (self *%s) %s(v %s) {\n\tif v == nil {\n\t\t%s = rbZero[%s]()\n\t} else {\n\t\t%s = *v\n\t}%s\n}\n\n",
				cls.Name, m.GoName, c.goType(m.Params[0].Type), ivarPath("self", iv), c.goType(iv.Type), ivarPath("self", iv), strings.ReplaceAll(mark, "; ", "\n\t"))
			return
		}
		c.w("func (self *%s) %s(v %s) { %s = v%s }\n\n", cls.Name, m.GoName, c.goType(iv.Type), ivarPath("self", iv), mark)
		return
	}
	env := map[string]Type{}
	if cls.GoType != "" {
		env["Self"] = cls.instance()
		cls.containerEnv(env)
	} else {
		env["Self"] = TVar{Name: "Self"}
	}
	params, ret := c.sig(m, env)
	namedRet := (containsRescue(m.Node.Body) || c.unwindsReturn(m)) && ret != ""
	retDecl := ret
	if namedRet {
		retDecl = "(ret_ " + ret + ")"
	}
	c.lineDirective(m.File, m.Line)
	if c.isDirectMethod(m) {
		c.labels[cls.Name+"."+m.GoName] = c.frameLabel(m)
		if c.goFuncs != nil && c.gemFile(m.File) {
			c.goFuncs[cls.Name+"."+m.GoName] = goFunc{methodKey(m), m.String()}
		}
		c.w("func (self %s) %s(%s) %s {\n", c.recvType(cls), m.GoName, params, retDecl)
	} else {
		c.labels[freeFuncName(m)] = c.frameLabel(m)
		if c.goFuncs != nil && c.gemFile(m.File) {
			c.goFuncs[freeFuncName(m)] = goFunc{methodKey(m), m.String()}
		}
		self := "self Self"
		switch {
		case m.SelfType != nil:
			self = "self " + c.goType(m.SelfType)
		case cls.GoType != "":
			self = "self " + c.recvType(cls)
		case valueInit(m):
			self = "self *" + cls.Name // the one method that writes a value's fields (decision 441)
		case c.plainSelf(m):
			self = "self " + c.plainSelfType(cls)
		}
		c.w("func %s%s(%s%s) %s {\n", freeFuncName(m), c.typeParamDecl(m), self, comma(params), retDecl)
	}
	if !c.isDirectMethod(m) && c.plainSelf(m) {
		// the body is generated with self typed Self: name it the interface (decision 210)
		saved := c.out
		c.out = strings.Builder{} // ponytail: same address trick as constraintNamesSelf
		c.emitBody(m, namedRet)
		body := c.out.String()
		c.out = saved
		if selfWord.MatchString(body) {
			c.w("\ttype Self = %s\n", c.plainSelfType(cls))
		}
		c.out.WriteString(body)
	} else {
		c.emitBody(m, namedRet)
	}
	c.w("}\n\n")
}

// valueInit is a value class's initialize: it takes *C and writes the fields NewC returns (decision 441).
func valueInit(m *Method) bool {
	return m.Owner != nil && m.Owner.byValue && m.Name == "initialize" && m.Owner.metaOf == nil
}

func (c *Compiler) emitTopDef(m *Method) {
	params, ret := c.sig(m, nil)
	namedRet := (containsRescue(m.Node.Body) || c.unwindsReturn(m)) && ret != ""
	retDecl := ret
	if namedRet {
		retDecl = "(ret_ " + ret + ")"
	}
	c.lineDirective(m.File, m.Line)
	c.labels[m.GoName] = "Object#" + m.Name
	c.w("func %s%s(%s) %s {\n", m.GoName, topTypeParamDecl(m), params, retDecl)
	c.emitBody(m, namedRet)
	c.w("}\n\n")
}

// frameLabel is m's name as MRI 4.0 labels its frames: 'K#m', 'K.s' for a
// singleton method. A prelude overload twin (`__integer_string`, decision
// 12) is labelled as the public method it stands in for.
func (c *Compiler) frameLabel(m *Method) string {
	owner, name := m.Owner, m.Name
	if m.File.isPrelude() && strings.HasPrefix(name, "__") {
		// the longest public prefix: `__each_with_index_enum` is each_with_index's helper, not each's. Map order
		// used to pick, so the label table (and the Go build cache key of every program that reads it) varied per run.
		for pub := range owner.Methods {
			if strings.HasPrefix(strings.ToLower(m.Name), "__"+strings.ToLower(pub)+"_") && (name == m.Name || len(pub) > len(name)) {
				name = pub
			}
		}
	}
	if owner.metaOf != nil {
		return owner.RubyName + "." + name
	}
	return owner.RubyName + "#" + name
}

// topTypeParamDecl is typeParamDecl for a top-level def, which has no Owner.
func topTypeParamDecl(m *Method) string {
	if len(m.TypeParams) == 0 {
		return ""
	}
	return "[" + strings.Join(m.TypeParams, ", ") + " comparable]"
}

func (c *Compiler) emitBody(m *Method, namedRet bool) {
	if m.Kind == kindPrimitive {
		x := m.Node.Body.(*parser.StatementsNode).Body[0].(*parser.XStringNode)
		body := strings.TrimSpace(x.Unescaped.Value)
		if !isVoid(m.Ret) && !m.Iterator && !strings.Contains(body, "\n") && !strings.HasPrefix(body, "return ") {
			body = "return " + body
		}
		c.lineDirective(m.File, m.File.line(x.Location.StartOffset))
		c.w("\t%s\n", strings.ReplaceAll(body, "\n", "\n\t"))
		return
	}
	f := c.newFctx(m.File, m.Owner, m)
	f.hasNamedRet = namedRet
	f.retVar = "ret_"
	if c.unwindsReturn(m) {
		f.returnUnwind(m, namedRet)
	}
	params := c.paramLocals(m)
	prologue := func() {
		for i, p := range m.Params {
			if p.Out { // copied in, and back out however the method returns, so a raise keeps the appends as in MRI
				f.emit("%s := *%s.s", goLocalName(p.Name), outParamName(p, i))
				f.emit("defer func() { *%s.s = %s }()", outParamName(p, i), goLocalName(p.Name))
			}
		}
		for _, p := range m.Params {
			if p.Rest && p.Name != "_" { // `*_` binds nothing
				f.emit("%s := &Array[%s]{s: rest_}", goLocalName(p.Name), c.goType(p.Type))
				f.emit("_ = %s", goLocalName(p.Name)) // `*_args` may go unused
			}
		}
		kwBit := 0
		for i, p := range m.Params {
			switch {
			case p.Keyword && m.calleeDefaults && p.Default != nil:
				f.fillDefault(fmt.Sprintf("rbKw&%d == 0", 1<<kwBit), p)
			case m.calleeDefaults && p.Default != nil:
				f.fillDefault(fmt.Sprintf("rbArgc <= %d", i), p)
			}
			if p.Keyword {
				kwBit++
			}
		}
		if m.optionalBlockLocal() != "" {
			f.emit("var %s %s", optBlockGo, c.goType(optBlockType(m)))
			f.emit("if blk != nil {")
			f.emit("\tp := &blk")
			f.emit("\t%s = &p", optBlockGo)
			f.emit("}")
			f.emit("_ = %s", optBlockGo)
		}
		if m.Iterator {
			ps := make([]string, len(m.Block.Params))
			for i, p := range m.Block.Params {
				ps[i] = c.goType(p)
			}
			f.emit("return func(yield func(%s) bool) {", strings.Join(ps, ", "))
			f.indent++
			if m.Node != nil && containsNestedYield(m.Node.Body, false) {
				f.emit("defer rbIterBreakRecover() // a yield inside a block stops the iteration by panicking (decision 226)")
			}
		}
	}
	t := tail{kind: tailReturn, typ: m.Ret}
	if m.Iterator {
		t = tail{}
	}
	f.indent = 1
	f.genBody(m.Node.Body, params, t, prologue)
	if m.Iterator {
		f.indent--
		f.emit("}")
	}
	body := f.buf.String()
	if i := strings.LastIndexByte(strings.TrimSuffix(body, "\n"), '\n') + 1; strings.HasPrefix(body[i:], "//line ") {
		body = body[:i] // a last statement that emitted no Go (a trailing nil): its directive alone reads as a blank line to the whitespace linter
	}
	c.out.WriteString(body)
}

// fillDefault runs a left-out param's default where Ruby does: in the callee, after the params before it.
func (f *fctx) fillDefault(missing string, p Param) {
	name := goLocalName(p.Name)
	if info := f.localInfo(p.Name); f.pass == 2 && info != nil && info.reads == 1 {
		name = "_" // never read: evaluated for its effects only
		switch p.Default.(type) {
		case *parser.NilNode, *parser.TrueNode, *parser.FalseNode, *parser.IntegerNode, *parser.SymbolNode, *parser.StringNode:
			return // a literal has none, and Go rejects `_ = nil`
		}
	}
	f.emit("if %s {", missing)
	f.indent++
	d := f.genExpr(p.Default, p.Type)
	var code string
	err := f.try(func() { code = f.coerce(p.Default, d, p.Type) })
	if err != nil && f.lenient() { // a gem instance's `priority = elem` where Elem is not Priority (decision 312): checked when the default runs, as MRI's value would be wherever used
		code = f.noteConv(p.Default, fmt.Sprintf("rbAs[%s](any(%s), %q)", f.c.goType(p.Type), d.code, p.Type.String()))
	} else if err != nil {
		panic(*err)
	}
	f.emit("%s = %s", name, code)
	f.indent--
	f.emit("}")
}
