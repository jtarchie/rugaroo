package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// genSuperNode is `super(...)` or a bare `super`, which forwards the method's arguments.
func (f *fctx) genSuperNode(n parser.Node) expr {
	if s, ok := n.(*parser.SuperNode); ok {
		return f.genSuper(n, s.Arguments, false)
	}
	return f.genSuper(n, nil, true)
}

// superRespondTo is super in a respond_to? override: Object's, the class's own methods by name at run time (decision 241).
func (f *fctx) superRespondTo(n parser.Node, args *parser.ArgumentsNode, forwarding bool) expr {
	var sargs []parser.Node
	if forwarding || args == nil {
		for _, p := range f.m.Params {
			sargs = append(sargs, &parser.LocalVariableReadNode{Name: p.Name, Location: n.GetLocation()})
		}
	} else {
		sargs = args.Arguments
	}
	if len(sargs) == 0 || len(sargs) > 2 {
		f.errorf(n, "super in respond_to? needs the method name")
	}
	return f.genDynRespondTo(n, expr{code: f.selfCode, typ: f.selfType}, sargs)
}

func (f *fctx) genSuper(n parser.Node, args *parser.ArgumentsNode, forwarding bool) expr {
	if f.m == nil || f.owner == nil {
		f.errorf(n, "super outside a method")
	}
	e := f.c.inheritedSig(f.m)
	if e == nil && f.m.aliasOf != "" { // `alias on_csend on_send`: the copy's super is on_send's, as MRI's (rubocop's IndentationWidth)
		orig := *f.m
		orig.Name = f.m.aliasOf
		e = f.c.inheritedSig(&orig)
	}
	if e != nil {
		f.c.inferRet(e.M)
		if f.lenient() { // an override's super is no evidence of what the method's other (dynamic) callers pass (bindata's String#assign)
			saved := f.superTarget
			f.superTarget = e.M
			defer func() { f.superTarget = saved }()
		}
	}
	if e == nil && f.m.superBridge {
		// the target depends on the includer: its bridge calls it (superBridges)
		env := map[string]Type{"Self": f.selfType}
		codes, _ := f.genArgs(n, f.m, env, f.superArgs(n, args, forwarding), nil)
		ret := subst(f.m.Ret, env)
		if f.m.Ret == nil && f.m.inferring { // while the method's own return is still being inferred, the super value is untyped (bindata's `v, p, q = super(...)`)
			ret = TAny{}
		}
		return expr{code: f.selfCode + "." + bridgeName(f.m) + "(" + strings.Join(codes, ", ") + ")", typ: ret}
	}
	if e == nil {
		switch f.m.Name {
		case "initialize":
			if e, ok := f.superValueInit(n, args, forwarding); ok {
				return e
			}
			return expr{code: "", typ: TVoid{}, stmt: true}
		case "respond_to_missing?": // Object's answers false
			return expr{code: "Boolean(false)", typ: f.cls("Boolean")}
		case "respond_to?":
			return f.superRespondTo(n, args, forwarding)
		case "inherited", "initialize_dup", "initialize_clone", "initialize_copy", "included", "extended", "prepended", "method_added":
			// Class's and Object's hooks do nothing (decision 232; initialize_copy's copy is done by then, decision 414): an
			// override's super reaches no method, so no slot
			return expr{code: "", typ: TVoid{}, stmt: true}
		case "method_missing": // Object's raises NoMethodError for the name
			var name parser.Node
			if forwarding && len(f.m.Params) > 0 {
				name = &parser.LocalVariableReadNode{Name: f.m.Params[0].Name, Location: n.(*parser.ForwardingSuperNode).Location}
			} else if args != nil && len(args.Arguments) > 0 {
				name = args.Arguments[0]
			}
			if name == nil {
				f.errorf(n, "super in method_missing needs the method name")
			}
			sym := f.coerce(name, f.genExpr(name, f.cls("Symbol")), f.cls("Symbol"))
			return expr{code: "panic(rbNoMethod(string(" + sym + "), " + f.selfCode + ", false))", typ: TVoid{}, stmt: true, noreturn: true}
		}
		if f.m.Name == "new" && f.owner != nil && f.owner.metaOf != nil && len(f.owner.metaOf.Subclasses) == 0 {
			// a class's own `def self.new(...)` ending in super (rubocop's Team.new): Class#new, the object through
			// initialize. ponytail: a class with subclasses still errors, since self may be one of them (decision 444)
			return f.genNew(n, f.owner.metaOf, f.superArgs(n, args, forwarding), nil, nil, nil)
		}
		f.errorf(n, "super: no parent method %s", f.m.Name)
	}
	if e.Owner.GoType != "" {
		if r, ok := f.superValueInit(n, args, forwarding); ok && f.m.Name == "initialize" {
			return r
		}
		return f.genContainerSuper(n, e, args, forwarding)
	}
	env := map[string]Type{}
	for k, v := range e.Env {
		env[k] = v
	}
	env["Self"] = f.selfType
	codes, restIdx := f.genArgs(n, e.M, env, f.superArgs(n, args, forwarding), nil)
	if e.M.Block != nil {
		blk := f.superBlock(n, e.M, env)
		if restIdx >= 0 {
			codes = slices.Insert(codes, restIdx, blk)
		} else {
			codes = append(codes, blk)
		}
	}
	if isSynthNew(e) && f.owner.metaOf != nil {
		return f.superSynthNew(e, codes, subst(e.M.Ret, env))
	}
	code, ret := f.superModuleCall(e, codes), subst(e.M.Ret, env)
	return expr{code: code, typ: ret}
}

// superSynthNew is `super` from a class's own `def self.new` into Class#new (elftools' ELFStruct.new): it makes an
// object of the class self is, which only the run time knows, so a switch over the class objects that share the
// constructor picks its Go constructor.
func (f *fctx) superSynthNew(e *entry, codes []string, ret Type) expr {
	base := e.M.Owner.metaOf
	init := base.lookup("initialize")
	tmps := make([]string, len(codes))
	for i, c := range codes {
		tmps[i] = f.newTmp()
		spread := strings.HasSuffix(c, "...") // a splat's slice, spread at the call
		f.emit("%s := %s", tmps[i], strings.TrimSuffix(c, "..."))
		if spread {
			tmps[i] += "..."
		}
	}
	args := strings.Join(tmps, ", ")
	rt := f.c.goType(ret)
	sw := f.newTmp()
	f.emit("var %s %s", sw, rt)
	f.emit("switch any(%s).(type) {", f.selfCode)
	family := []*Class{base}
	for i := 0; i < len(family); i++ {
		family = append(family, family[i].Subclasses...)
	}
	for _, k := range family {
		ki := k.lookup("initialize")
		if k.meta == nil || !k.isStruct() || k.isTemplate() || k.isPerRun || (ki == nil) != (init == nil) || ki != nil && ki.M != init.M {
			continue
		}
		f.emit("case *%s:\n\t%s = %s(%s)", k.meta.Name, sw, k.ctorName(), args)
	}
	f.emit("default:\n\t%s = %s(%s)", sw, base.ctorName(), args)
	f.emit("}")
	return expr{code: sw, typ: ret}
}

// superBlock is the block `super` passes to m (decision 223): its own literal, else the current method's block, which
// Ruby's super passes on implicitly, else nil when m's block is optional.
func (f *fctx) superBlock(n parser.Node, m *Method, env map[string]Type) string {
	var lit parser.Node
	switch s := n.(type) {
	case *parser.SuperNode:
		lit = s.Block
	case *parser.ForwardingSuperNode:
		if s.Block != nil {
			lit = s.Block
		}
	}
	if b, ok := lit.(*parser.BlockNode); ok {
		return f.blockArg(n, m, env, b)
	}
	if ba, ok := lit.(*parser.BlockArgumentNode); ok && !f.isBlockParam(ba.Expression) {
		return f.blockArg(n, m, env, ba) // `super(&:<=)`, `super(&proc)`: as a call's &expr
	}
	if f.iterator {
		f.errorf(n, "super from an iterator (its block returns void) is not supported: Go cannot pass a yield on")
	}
	if f.blockSig != nil {
		return f.forwardClosure(n, m.Block, env)
	}
	if !m.Block.Optional {
		f.errorf(n, "super to %s needs a block, and this method has none to pass on", m.Name)
	}
	return "nil"
}

// superModuleCall is the static call `super` makes to e. When e is a module body whose constraint names Self (`def
// wait = self`), this class's own override returns its interface, so a generic Self param cannot satisfy it (have
// Wait() DelayI, want Wait() Self): the call names the class interface, and a Self result converts back (decision 217).
func (f *fctx) superModuleCall(e *entry, codes []string) string {
	o := f.owner
	if !e.Owner.IsModule || e.Owner.universal || len(e.Owner.TypeParams) > 0 || o.IsModule || !o.isStruct() || len(o.TypeParams) > 0 ||
		mentionsVar(e.M.Ret) && !isSelfVar(e.M.Ret) || !f.c.constraintNamesSelf(e.Owner) {
		return staticCallCode(e.M, "", f.selfCode, strings.Join(codes, ", "))
	}
	code := staticCallCode(e.M, "["+o.ifaceName()+"]", f.selfCode, strings.Join(codes, ", "))
	if isSelfVar(e.M.Ret) {
		code = "rbAs[Self](" + code + ", \"self\")" // a call, so it stands as a statement too
	}
	return code
}

// superArgs is what `super` passes: its arguments, or for a zsuper the
// current params re-read as locals, so genArgs coerces them to the
// parent's types and fills the parent's remaining defaults.
func (f *fctx) superArgs(n parser.Node, args *parser.ArgumentsNode, forwarding bool) []parser.Node {
	if !forwarding {
		if args == nil {
			return nil
		}
		return args.Arguments
	}
	var an []parser.Node
	var kws []parser.Node // zsuper passes keywords on by name
	loc := n.(*parser.ForwardingSuperNode).Location
	var posts []parser.Node // Ruby passes them after *rest, which Params keeps last
	for _, p := range f.m.Params {
		var a parser.Node = &parser.LocalVariableReadNode{Name: p.Name, Location: loc}
		switch {
		case p.Post:
			posts = append(posts, a)
			continue
		case p.KwRest:
			kws = append(kws, &parser.AssocSplatNode{Value: a, Location: loc})
			continue
		case p.Keyword:
			key := &parser.SymbolNode{Location: loc, Unescaped: parser.RubyString{Value: p.Name}}
			kws = append(kws, &parser.AssocNode{Key: key, Value: a, Location: loc})
			continue
		case p.Rest:
			a = &parser.SplatNode{Expression: a, Location: loc}
		}
		an = append(an, a)
	}
	an = append(an, posts...)
	if len(kws) > 0 {
		an = append(an, &parser.KeywordHashNode{Elements: kws, Location: loc})
	}
	return an
}

// genContainerSuper is `super` into a core class's method from its subclass (decision 202): the method on the
// embedded base.
func (f *fctx) genContainerSuper(n parser.Node, e *entry, args *parser.ArgumentsNode, forwarding bool) expr {
	if f.owner == nil || !f.owner.hashBase || e.Owner != f.owner.Super {
		f.errorf(n, "super into a primitive class method is not supported")
	}
	env := map[string]Type{}
	for k, v := range e.Env {
		env[k] = v
	}
	env["Self"] = f.selfType
	f.owner.containerEnv(env)
	targs := ""
	if st, ok := f.selfType.(TClass); ok {
		for i, p := range st.C.TypeParams {
			if i < len(st.Args) {
				env[p] = st.Args[i]
			}
		}
		targs = f.c.containerTargs(f.owner, st.Args)
	}
	codes, restIdx := f.genArgs(n, e.M, env, f.superArgs(n, args, forwarding), nil)
	if e.M.Block != nil { // Thread#initialize's block (decisions 223, 239)
		blk := f.superBlock(n, e.M, env)
		if restIdx >= 0 {
			codes = slices.Insert(codes, restIdx, blk)
		} else {
			codes = append(codes, blk)
		}
	}
	call := freeFuncName(e.M) + targs + "(" + embeddedSuper(f.owner, f.selfCode) + comma(strings.Join(codes, ", ")) + ")"
	if f.c.isDirectMethod(e.M) { // a non-generic base's own Go method: no free func exists
		call = embeddedSuper(f.owner, f.selfCode) + "." + e.M.GoName + "(" + strings.Join(codes, ", ") + ")"
	}
	if tv, ok := e.M.Ret.(TVar); ok && tv.Name == "Self" { // `super` into Set#<<: the subclass object, not its embedded base
		t := f.c.goType(f.selfType)
		return expr{code: fmt.Sprintf("func() %s { %s; return %s }()", t, call, f.selfCode), typ: f.selfType}
	}
	return expr{code: call, typ: subst(e.M.Ret, env)}
}

// superValueInit is `super(path)` in initialize of a Pathname (String) subclass: the embedded base is Base.new(path).
func (f *fctx) superValueInit(n parser.Node, args *parser.ArgumentsNode, forwarding bool) (expr, bool) {
	o := f.owner
	if f.m.Name == "initialize" && o != nil && o.hashBase && o.Super.RubyName == "Range" && o.Super.meta != nil && args != nil && len(args.Arguments) >= 2 {
		// `super(first, last)` in a Range subclass (rubocop's DirectiveRange): the embedded Range is Range.new's, its
		// bounds untyped as the subclass holds them (decision 444)
		rt := TClass{C: o.Super, Args: []Type{TAny{}}}
		base := expr{code: classVar(o.Super), typ: TClass{C: o.Super.meta}, classObj: true}
		sa := f.superArgs(n, args, forwarding)
		for i, a := range sa {
			if i < 2 {
				sa[i] = &exprNode{Node: a, e: expr{code: f.coerce(a, f.genExpr(a, nil), TAny{}), typ: TAny{}}}
			}
		}
		built := f.genMethodCall(n, base, "new", sa, nil)
		return expr{code: f.selfCode + "." + superField(o.Super) + " = *" + f.coerce(n, built, rt), typ: TVoid{}, stmt: true}, true
	}
	if f.m.Name != "initialize" || o == nil || !o.hashBase || o.Super.mutable() || o.Super.meta == nil {
		return expr{}, false
	}
	base := expr{code: classVar(o.Super), typ: TClass{C: o.Super.meta}, classObj: true}
	built := f.genMethodCall(n, base, "new", f.superArgs(n, args, forwarding), nil)
	return expr{code: f.selfCode + "." + superField(o.Super) + " = " + f.coerce(n, built, TClass{C: o.Super}), typ: TVoid{}, stmt: true}, true
}
