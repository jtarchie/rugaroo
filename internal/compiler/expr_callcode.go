package compiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	"go/token"
	"math/big"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// closed returns subst(t, env) if it has no unbound method type vars.
func closed(t Type, env map[string]Type) Type {
	s := subst(t, env)
	var vars []string
	freeVars(s, &vars)
	for _, v := range vars {
		if _, ok := env[v]; !ok {
			return nil
		}
	}
	return s
}

// nilableFetch reroutes Hash#fetch whose default may be nil: MRI returns that nil, so the result is V? (decision 12).
func (f *fctx) nilableFetch(m *Method, args []parser.Node, block parser.Node) *entry {
	if m.Name != "fetch" || m.Owner == nil || m.Owner.RubyName != "Hash" || len(args) != 2 || block != nil {
		return nil
	}
	var d expr
	f.probe(func() { d = f.genExpr(args[1], nil) })
	if !isNil(d.typ) && !isOpt(d.typ) && !isAny(d.typ) {
		return nil
	}
	return m.Owner.lookup("__fetch_opt")
}

// unusedBlock is a block given to a method that never yields: MRI ignores it (decision 364), but a core method's block
// form the prelude lacks (`xs.uniq(&:name)`) is an error, not a silently different answer.
func (f *fctx) unusedBlock(n parser.Node, m *Method) {
	switch {
	case m.File == nil || m.File.isPrelude():
		f.errorf(n, "%s does not take a block", m.Name)
	case f.pass == 2 && !f.lenient():
		f.c.warn(f.f, n, "%s never yields: its block is ignored, as MRI does", m.Name)
	}
}

// paramlessBlock reports a call whose block literal takes nothing (`Hash.new { 5 }`): it binds no type parameter.
func paramlessBlock(n parser.Node) bool {
	c, ok := n.(*parser.CallNode)
	if !ok {
		return false
	}
	b, ok := c.Block.(*parser.BlockNode)
	return ok && b.Parameters == nil
}

// paramlessBlockEnv binds, for a block that takes nothing, the type parameters only its parameters name (`Hash.new { 5 }`'s
// K): untyped, as an empty container's (decision 192); what the block returns still binds the rest.
func (f *fctx) paramlessBlockEnv(n parser.Node, m *Method, env map[string]Type) {
	if !paramlessBlock(n) || containerOf(m) == "" || f.f == nil || f.f.isPrelude() {
		return
	}
	if body := n.(*parser.CallNode).Block.(*parser.BlockNode).Body; body != nil { //nolint:forcetypeassert // paramlessBlock
		if st, ok := body.(*parser.StatementsNode); ok {
			var e expr
			if catchCompileError(func() {
				f.probe(func() { e = f.genExpr(&parser.BeginNode{Location: st.Location, Statements: st}, nil) })
			}) == nil {
				unify(m.Block.Ret, e.typ, env) // what the block returns binds V
			}
		}
	}
	for _, tp := range m.TypeParams {
		if _, ok := env[tp]; !ok {
			env[tp] = TAny{}
		}
	}
}

// bindTypeParams checks every type parameter of m is bound, first
// unifying m's return type with the call's expected type, if any
// (`SizedQueue.new(2) #: SizedQueue[Integer]`).
func (f *fctx) bindTypeParams(n parser.Node, m *Method, env map[string]Type) {
	if n == f.retHintNode && f.retHint != nil {
		unify(m.Ret, f.retHint, env)
	}
	for _, tp := range m.TypeParams {
		if _, ok := env[tp]; !ok {
			if f.c.pruneGems && f.c.gemFile(f.f) { // a gem's `Set[]` is a Set[untyped] (decision 169)
				env[tp] = TAny{}
				continue
			}
			if owner := containerOf(m); owner != "" && (m.Block == nil || paramlessBlock(n)) && f.f != nil && !f.f.isPrelude() { // a block with parameters was typed with it already
				// an empty core container nothing types: untyped elements, its calls dynamic (decision 192)
				f.warn(n, "%s's %s is untyped: nothing in the program gives it a type (annotate `#: %s[...]` for static calls; decision 192)", m, tp, owner)
				env[tp] = TAny{}
				continue
			}
			f.errorf(n, "cannot infer type parameter %s of %s", tp, m)
		}
	}
}

// callEntry calls e; in gem code, a call the static signature cannot take falls back to a run-time dispatch (genDynCall).
func (f *fctx) callEntry(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	return f.instanceResult(e, recv, f.callEntryAny(n, e, recv, args, block))
}

// instanceResult types a class method's `-> instance` result as the class it was called on: `Article.find` of `Base.find` (decision 404).
func (f *fctx) instanceResult(e *entry, recv expr, r expr) expr {
	if e.M.sig == nil || e.M.Owner == nil || e.M.Owner.metaOf == nil {
		return r
	}
	ret, opt := e.M.sig.Return, false
	if o, ok := ret.(rbs.Optional); ok {
		ret, opt = o.Elem, true
	}
	rc, ok := recv.typ.(TClass)
	if _, inst := ret.(rbs.Instance); !inst || !ok || rc.C.metaOf == nil || rc.C.metaOf == e.M.Owner.metaOf || !rc.C.metaOf.isSubclassOf(e.M.Owner.metaOf) {
		return r
	}
	want := TClass{C: rc.C.metaOf}
	g := f.c.goType(want)
	if opt {
		return expr{code: "func() " + g + " { v, _ := any(" + r.code + ").(" + g + "); return v }()", typ: TOpt{Elem: want}}
	}
	return expr{code: "any(" + r.code + ").(" + g + ")", typ: want}
}

func (f *fctx) callEntryAny(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if f.lenient() && block == nil && len(args) > 0 && f.unionArm == 0 && !slices.ContainsFunc(args, f.readsUnionLocal) { // a union argument: unionArgCall first
		// gem code: arguments the static signature cannot take (a splat, a computed keyword) are matched per call at run time, as MRI does
		if f.probing() { // output is discarded: no dry run needed
			var r expr
			err := f.try(func() { r = f.callEntry0(n, e, recv, args, block) })
			if err != nil {
				return f.genDynCall(n, recv, e.M.Name, args)
			}
			return r
		}
		err := f.try(func() { f.probe(func() { f.callEntry0(n, e, recv, args, block) }) })
		if err != nil {
			return f.genDynCall(n, recv, e.M.Name, args)
		}
	}
	return f.callEntry0(n, e, recv, args, block)
}

// elemWriters are the Array methods that store their last argument as an element.
var elemWriters = map[string]bool{"[]=": true, "<<": true, "push": true, "unshift": true}

// widenIvarElem: discovery seeing a gem store a value its ivar's element type does not hold (eigenvalue_decomposition's
// `@d = Array.new(n, 0)`, then `@d[i] = Math.sqrt(x)`) makes that element untyped, at the depth the `[]` reads reach
// (`@v[i][j] = 0.5`), as MRI's Array holds anything (decision 414). Discovery accepts a Float where an Integer is
// expected (pass 2 rejects it), so the value is checked here, not by the call failing.
func (f *fctx) widenIvarElem(n *parser.CallNode) {
	args := callArgs(n)
	if !f.discover || !f.lenient() || f.owner == nil || !elemWriters[n.Name] || len(args) == 0 || n.Block != nil {
		return
	}
	depth, r := 0, n.Receiver
	for {
		c, ok := r.(*parser.CallNode)
		if !ok || c.Name != "[]" || c.Receiver == nil {
			break
		}
		depth, r = depth+1, c.Receiver
	}
	rd, ok := r.(*parser.InstanceVariableReadNode)
	if !ok {
		return
	}
	iv := f.c.findIvar(f.owner, rd.Name)
	if iv == nil || iv.open {
		return
	}
	elem, ok := elemAt(iv.Type, depth)
	if !ok {
		return
	}
	v := f.probeType(args[len(args)-1], nil) // an untyped value may be any class, as eigenvalue_decomposition's Floats from @v are
	if v == nil || !isAny(v) && fitsValue(expr{typ: v}, elem) && (!isClass(v, "Float") || !isClass(elem, "Integer")) {
		return
	}
	if t, ok := untypedElemAt(iv.Type, depth); ok {
		iv.Type = t
	}
}

// elemAt is the Array element type depth `[]` reads into t reach, through T? wrappers.
func elemAt(t Type, depth int) (Type, bool) {
	tc, ok := stripOpt(t).(TClass)
	if !ok || tc.C.RubyName != "Array" || len(tc.Args) != 1 || isAny(tc.Args[0]) {
		return nil, false
	}
	if depth == 0 {
		return tc.Args[0], true
	}
	return elemAt(tc.Args[0], depth-1)
}

// untypedElemAt is t with its Array element at depth (through T? wrappers) untyped; false when t has no Array there.
func untypedElemAt(t Type, depth int) (Type, bool) {
	if o, ok := t.(TOpt); ok {
		e, ok := untypedElemAt(o.Elem, depth)
		return TOpt{Elem: e}, ok
	}
	tc, ok := t.(TClass)
	if !ok || tc.C.RubyName != "Array" || len(tc.Args) != 1 || isAny(tc.Args[0]) {
		return t, false
	}
	elem := Type(TAny{})
	if depth > 0 {
		if elem, ok = untypedElemAt(tc.Args[0], depth-1); !ok {
			return t, false
		}
	}
	return TClass{C: tc.C, Args: []Type{elem}}, true
}

func isSplat(a parser.Node) bool {
	_, ok := a.(*parser.SplatNode)
	return ok
}

// callEntry0 calls e; a call to a `bot` method is noreturn, and Go must
// see its panic to know the statement list ends there.
func (f *fctx) callEntry0(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if r, ok := f.gemCallShortcut(n, e, recv, args, block); ok {
		return r
	}
	if block == nil {
		if r, ok := f.mtLiteral(n, e, recv, args); ok {
			return r
		}
	}
	r := f.callMethod(n, e, recv, args, block)
	if e.M.noReturn && !r.noreturn {
		r = expr{code: r.code + "\npanic(\"rugaroo: unreachable\")", typ: TVoid{}, stmt: true, noreturn: true}
	}
	return r
}

// receiverEnv binds the type variables a call to e on recv starts from: the receiver's class arguments and Self.
func receiverEnv(e *entry, recv expr) map[string]Type {
	env := map[string]Type{}
	if rt, ok := recv.typ.(TClass); ok {
		classEnv := map[string]Type{}
		for i, p := range rt.C.TypeParams {
			if i < len(rt.Args) {
				classEnv[p] = rt.Args[i]
			}
		}
		env = composeEnv(e.Env, classEnv)
		rt.C.containerEnv(env) // also a Hash def aliased into the subclass (linkAliases), which keeps Hash's K
	} else {
		for k, v := range e.Env {
			env[k] = v
		}
	}
	env["Self"] = recv.typ
	return env
}

// callEnv is receiverEnv plus what the method's `@self` binds (decision 92).
func (f *fctx) callEnv(n parser.Node, e *entry, recv expr) map[string]Type {
	m := e.M
	env := receiverEnv(e, recv)
	if m.SelfType != nil && !unify(m.SelfType, recv.typ, env) {
		f.errorf(n, "%s needs a receiver of type %s, not %s", m.Name, m.SelfType, recv.typ)
	}
	return env
}

// checkVisibility rejects at compile time what MRI's NoMethodError would: a private method with a receiver, a protected one from outside its owner's family.
func (f *fctx) checkVisibility(n parser.Node, m *Method, recv expr) {
	if recv.code == f.selfCode || f.implicitCall {
		return
	}
	if m.Private && !kernelModuleFunction(m, recv.typ) {
		f.errorf(n, "private method %s called on %s", m.Name, recv.typ)
	}
	if m.Protected && (f.owner == nil || !f.owner.isSubclassOf(m.Owner)) {
		f.errorf(n, "protected method %s called on %s", m.Name, recv.typ)
	}
}

// MRI's Kernel.singleton_methods; the rest of Kernel's private methods (pp, initialize_copy) stay private on Kernel too.
var kernelModuleNames = func() map[string]bool {
	out := map[string]bool{}
	for _, s := range strings.Fields("Array Complex Float Hash Integer Pathname Rational String __callee__ __dir__ __method__ ` abort at_exit autoload autoload? binding block_given? caller caller_locations catch eval exec exit exit! fail fork format gets global_variables iterator? lambda load local_variables loop open p print printf proc putc puts raise rand readline readlines require require_relative select set_trace_func sleep spawn sprintf srand syscall system test throw trace_var trap untrace_var warn") {
		out[s] = true
	}
	return out
}()

func kernelMeta(t Type) bool {
	c, ok := t.(TClass)
	return ok && c.C.metaOf != nil && c.C.metaOf.RubyName == "Kernel"
}

// isTopConst is a syntactic guess (`Kernel`, `::Kernel`) for passes with no scope to resolve it in; genCall checks the type.
func isTopConst(n parser.Node, name string) bool {
	switch r := n.(type) {
	case *parser.ConstantReadNode:
		return r.Name == name
	case *parser.ConstantPathNode:
		return r.Parent == nil && r.Name != nil && *r.Name == name
	}
	return false
}

// kernelRecv: a receiverless call, or one on Kernel, the module function form.
func kernelRecv(n *parser.CallNode) bool {
	return n.Receiver == nil || isTopConst(n.Receiver, "Kernel")
}

// `Kernel.puts`: module_function makes these public on Kernel itself (decision 139).
func kernelModuleFunction(m *Method, recv Type) bool {
	if !kernelMeta(recv) || m.Owner == nil || m.Owner.RubyName != "Kernel" {
		return false
	}
	if kernelModuleNames[m.Name] {
		return true
	}
	for pub := range kernelModuleNames { // an overload twin (`__Integer_string`, decision 12) stands in for its public method
		if strings.HasPrefix(m.Name, "__"+overloadBase(pub)+"_") {
			return true
		}
	}
	return false
}

// callMethod is callMethod0 with binary Strings (decision 352): a `@binary` method's String result is binary.
func (f *fctx) callMethod(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	out := f.callMethod0(n, e, recv, args, block)
	f.noteForceEncoding(e.M, recv, args)
	if b := e.M.binary; (b == 1 || b == 2 && recv.bin) && binCarrier(out.typ) && !readAll(e.M, args) {
		out.bin = true
		if b == 2 {
			out.binDeps = recv.binDeps
		}
	}
	if recv.bin && e.M.Name == "+" && isClass(out.typ, "String") && len(args) == 1 { // bin + bin, or + ASCII text: binary, as MRI
		var a expr
		f.probe(func() { a = f.genExpr(args[0], nil) })
		if s, ok := args[0].(*parser.StringNode); a.bin || ok && isASCII(s.Unescaped.Value) {
			out.bin, out.binDeps = true, append(slices.Clone(recv.binDeps), a.binDeps...)
		}
	}
	return out
}

// noteForceEncoding: `s.force_encoding(enc)` of anything but a literal BINARY may make the binary String it was read
// from text (MRI changes it in place), so those locals are text in every read (decision 352).
func (f *fctx) noteForceEncoding(m *Method, recv expr, args []parser.Node) {
	if m.Name != "force_encoding" || !recv.bin || f.pass >= 2 || len(args) == 1 && binaryEncodingArg(f.f, args[0]) {
		return
	}
	for _, d := range recv.binDeps {
		d.text = true
	}
}

// binCarrier: the types the binary flag rides on, a String or the MatchData of a binary match (decision 352).
func binCarrier(t Type) bool {
	t = stripOpt(t)
	return isClass(t, "String") || isClass(t, "MatchData")
}

// readAll is `io.read(nil)`: the whole stream in the external encoding (UTF-8), not read(n)'s binary bytes.
func readAll(m *Method, args []parser.Node) bool {
	if m.Name != "__read_1" || len(args) != 1 {
		return false
	}
	_, ok := args[0].(*parser.NilNode)
	return ok
}

// binRegexpTwins are the Regexp methods whose String argument, when binary, takes a `__bin_*` twin.
var binRegexpTwins = map[string]bool{"=~": true, "match": true, "match?": true, "===": true}

// binTwin is m's `__bin_<name>` on a binary receiver, or a Regexp's on a binary argument (decision 352). It runs
// after overload picks a twin (decision 12), so `__gsub_block` has `__bin_gsub_block`.
func (f *fctx) binTwin(m *Method, recv expr, args []parser.Node) *entry {
	c, ok := recv.typ.(TClass)
	if !ok || strings.HasPrefix(m.Name, "__bin_") {
		return nil
	}
	base := "__bin_" + overloadBase(m.Name)
	if strings.HasPrefix(m.Name, "__") {
		base = "__bin_" + m.Name[2:]
	}
	switch {
	case recv.bin && c.C.RubyName == "String":
	case m.Name == "p" && m.Owner != nil && m.Owner.RubyName == "Kernel" && len(args) == 1:
		var a expr
		f.probe(func() { a = f.genExpr(args[0], nil) })
		if !a.bin {
			return nil
		}
		return c.C.lookup(base)
	case c.C.RubyName == "Regexp" && len(args) == 1 && binRegexpTwins[m.Name]:
		var a expr
		f.probe(func() { a = f.genExpr(args[0], nil) })
		if !a.bin {
			return nil
		}
		return c.C.lookup(base)
	default:
		return nil
	}
	if e := c.C.lookup(base); e != nil && len(args) >= requiredArgs(e.M) && len(args) <= len(e.M.Params) {
		return e
	}
	return nil
}

// callTwin is the twin a call takes instead of e: a reused window's (decision 386), its overload (decision 12), or a binary String's (decision 352).
func (f *fctx) callTwin(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) *entry {
	if o := f.windowViewEnum(n, e, recv.typ, block); o != nil {
		return o
	}
	if o := f.zipViewEnum(n, e, recv.typ, block); o != nil {
		return o
	}
	if o := f.fiddleTwinCall(e, recv, args, block); o != nil {
		return o
	}
	f.curBlock = block
	o := f.overload(e, recv.typ, args)
	f.curBlock = nil
	if o == nil {
		o = f.binTwin(e.M, recv, args)
	}
	return o
}

func (f *fctx) callMethod0(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	m := e.M
	f.c.inferRet(m)
	if o := f.nilableFetch(m, args, block); o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	if r, ok := f.gemCallForm(n, e, recv, args, block); ok {
		return r
	}
	if o := f.callTwin(n, e, recv, args, block); o != nil {
		return f.callEntry(n, o, recv, args, block)
	}
	env := f.callEnv(n, e, recv)
	f.checkVisibility(n, m, recv)
	recv = f.leafRecvSelf(recv, m)
	var armT Type // the `.rbs` overload arm this call takes (decision 12), picked before the merged parameters check the arguments
	if len(m.arms) > 0 {
		armT = f.armRet(n, m, args)
	}
	f.nilRecv = recv.typ
	codes, restIdx := f.genArgs(n, m, env, args, nil)
	f.outArgs(n, m, args, codes)
	if m.Block != nil && m.Iterator && block == nil && !m.Block.Optional {
		if r, ok := f.iterEnum(n, e, recv, codes, env); ok {
			return r
		}
	}
	if m.Block != nil {
		if m.Iterator {
			f.errorf(n, "%s is an iterator (its block returns void); call it as a statement with a block", m.Name)
		}
		// Bind type params from the expected type before the block's closure,
		// whose parameter types may name them (`Hash.new { |h, k| }`).
		if n == f.retHintNode && f.retHint != nil {
			unify(m.Ret, f.retHint, env)
		}
		if b, ok := block.(*parser.BlockNode); ok && f.f.kwBlocks[b] && yieldsKeywords(m) {
			f.errorf(n, "the block's keyword parameters would take %s's yielded keywords: not supported (decision 369)", m.Name)
		}
		f.paramlessBlockEnv(n, m, env)
		blkCode := f.blockArg(n, m, env, block)
		if restIdx >= 0 {
			codes = slices.Insert(codes, restIdx, blkCode)
		} else {
			codes = append(codes, blkCode)
		}
	} else if block != nil {
		f.unusedBlock(n, m)
	}
	f.bindTypeParams(n, m, env)
	f.valOptNils(m, codes, restIdx, env)
	ret := subst(m.Ret, env)
	out := expr{code: flatCall(f.callCode(e, recv, codes, env), m, env), typ: ret}
	out = f.blocklessReturn(n, m, block, env, out)
	if m.Name == "const_get" && m.Owner.RubyName == "Module" {
		if t := f.constGetType(recv, args); t != nil {
			if isOpt(t) {
				// the constant table holds T? as T or nil (see Opt)
				out.code = f.coerce(n, expr{code: out.code, typ: TAny{}}, t)
			} else {
				out.code, out.assert = out.code+".("+f.c.goType(t)+")", true
			}
			out.typ = t
		}
	}
	// `klass.new` returns the hierarchy's root type; narrow to the class the
	// receiver is statically known to be.
	if m.Kind == kindSynth && m.Name == "new" {
		if meta := f.metaOfType(recv.typ); meta != nil && meta.metaOf != meta.metaOf.root() {
			out.typ = TClass{C: meta.metaOf}
			out.code, out.assert = out.code+".("+f.c.goType(out.typ)+")", true
		}
	}
	if f.c.selfReturnAsserts(e, recv.typ) {
		out.code, out.assert = out.code+".("+f.c.goType(recv.typ)+")", true
	} else if f.c.selfVarReturns(e, recv.typ) {
		out.code, out.assert = "any("+out.code+").(Self)", true
	}
	out.provisional = f.c.provisional(m, out.typ)
	if armT != nil {
		return f.narrowToArm(n, flatOpt(out), subst(armT, env))
	}
	return flatOpt(out)
}

// flatOpt collapses a nested optional result: a generic E? instantiated
// with E = T? is a Go **T, and with E = untyped a *any, but Ruby has one nil.
func flatOpt(e expr) expr {
	o, ok := e.typ.(TOpt)
	switch {
	case !ok:
	case isAny(o.Elem):
		return expr{code: "Opt(" + e.code + ")", typ: o.Elem, nilable: true}
	case isUnion(o.Elem): // E? with E = A | B: the box opens into the union, nil a member (decision 150)
		return expr{code: "Opt(" + e.code + ")", typ: optOf(o.Elem)}
	case isOpt(o.Elem): // E? with E = T?: Go's *(T?), opened to T? whatever T? is (a box, a leaf's *C, an interface)
		return expr{code: "rbFlatNil(" + e.code + ")", typ: o.Elem}
	}
	return e
}

// newBySubclass is `klass.new(v)` where klass is a class or one of its subclasses and some subclass's new takes other Go
// parameter types (Homebrew's Version::Token.create: StringToken.new(String), NumericToken.new(String | Integer)): one
// interface assertion cannot name both, so a type switch picks the class and converts each argument to its new's
// parameter (decision 290). Positional parameters only; anything else keeps the single assertion.
func (f *fctx) newBySubclass(m *Method, recv expr, args []string, env map[string]Type, ret string) (string, bool) {
	if m.calleeDefaults || m.kwMask() || m.Block != nil || len(args) != len(m.Params) || slices.ContainsFunc(m.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest }) {
		return "", false
	}
	type arm struct {
		meta *Class
		m    *Method
	}
	var arms []arm
	queue := []*Class{m.Owner.metaOf}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		queue = append(queue, k.Subclasses...)
		if k.meta == nil {
			continue
		}
		e := k.meta.lookup("new")
		if e == nil || e.M == m || len(e.M.Params) != len(m.Params) || e.M.calleeDefaults || e.M.kwMask() || e.M.Block != nil ||
			slices.ContainsFunc(e.M.Params, func(p Param) bool { return p.Rest || p.Keyword || p.KwRest }) {
			continue
		}
		arms = append(arms, arm{k.meta, e.M})
	}
	if len(arms) == 0 {
		return "", false
	}
	pinned := make([]string, len(args))
	var b strings.Builder
	b.WriteString("func() " + ret + " {\n")
	for i, a := range args {
		pinned[i] = fmt.Sprintf("a%d", i)
		fmt.Fprintf(&b, "var %s %s = %s\n", pinned[i], f.c.goType(subst(m.Params[i].Type, env)), a)
	}
	b.WriteString("r := any(" + recv.code + ")\nswitch c := r.(type) {\n")
	for _, a := range arms {
		conv := make([]string, len(pinned))
		for i, p := range a.m.Params {
			base := subst(m.Params[i].Type, env)
			if typeEq(base, p.Type) {
				conv[i] = pinned[i]
				continue
			}
			// the subclass may take less than the base (String where String? is): checked when this class is the one
			boxed := f.coerce(nil, expr{code: pinned[i], typ: base}, TAny{})
			conv[i] = f.coerce(nil, expr{code: boxed, typ: TAny{}}, p.Type)
		}
		fmt.Fprintf(&b, "case *%s:\nreturn c.New(%s)\n", a.meta.Name, strings.Join(conv, ", "))
	}
	ps, _ := f.c.sig(m, env)
	fmt.Fprintf(&b, "}\nreturn r.(interface{ New(%s) %s }).New(%s)\n}()", ps, ret, strings.Join(pinned, ", "))
	return b.String(), true
}

// callCode renders the call for entry e.
func (f *fctx) callCode(e *entry, recv expr, args []string, env map[string]Type) string {
	m := e.M
	argList := strings.Join(args, ", ")
	recv.code = f.materialize(recv)
	if m.Owner == nil {
		return m.GoName + "(" + argList + ")"
	}
	if code, ok := f.containerCallCode(e, recv, args, env); ok {
		return code
	}
	if m.Owner.metaOf != nil && m.Name == "new" && !recv.classObj {
		// `new` on a class object of unknown exact class: assert for it.
		ps, ret := f.c.sig(m, env)
		if code, ok := f.newBySubclass(m, recv, args, env, ret); ok {
			return code
		}
		return "any(" + recv.code + ").(interface{ New(" + ps + ") " + ret + " }).New(" + argList + ")"
	}
	selfRecv := f.nilSelf(recv, m)
	if code, ok := f.directCall(e, selfRecv, recv, args, env); ok {
		return code
	}
	// A primitive's non-direct method is called by its free func, never its forwarder, so the pruner drops unused forwarders (decision 86).
	direct := f.c.isDirectMethod(m)
	free := f.staticDef == m || m.generic() || (staticPrivate(m) && !direct) || (m.Owner.GoType == "" && !f.hasForwarder(recv.typ, e)) || (m.Owner.GoType != "" && !direct) ||
		m.Owner.universal && mentionsSelf(m.Ret) || // an interface declares `-> self` as `any`; the free func returns Self
		f.leafCall(recv)
	if !free && f.plainCalls && recv.code == "self" && f.owner != nil && !direct {
		if code, ok := f.wrapperSelfCall(e, argList); ok {
			return code
		}
	}
	if !free && m.Name == "initialize" && !inInterface(m) {
		// an overridden private initialize dispatches (decision 253) but is never an interface slot: through an
		// interface-typed receiver (SystemCallErrorI) it is the free func
		t := f.methodExprType(m, recv)
		free = t != "" && !strings.HasPrefix(t, "(")
	}
	if !free {
		if t := f.methodExprType(m, recv); t != "" {
			return t + "." + m.GoName + "(" + recv.code + comma(argList) + ")"
		}
		return recv.code + "." + m.GoName + "(" + argList + ")"
	}
	return staticCallCode(m, f.typeArgs(m, recv.typ, env), selfRecv.code, argList)
}

// coreEachValue is a core each-like iterator's value in value position (`f(xs.each { })`, `xs.send(:each) { }`): its
// receiver, as MRI's each answers self (decision 365); the call runs as the iterator statement on the pinned receiver.
func (f *fctx) coreEachValue(n parser.Node, m *Method, recv expr, block parser.Node) (expr, bool) {
	c, ok := n.(*parser.CallNode)
	if !ok || f.lenient() || !m.Iterator || block == nil || !coreEach(m) {
		return expr{}, false
	}
	tmp := f.newTmp()
	pinned := *c
	pinned.Receiver = &exprNode{Node: c.Receiver, e: expr{code: tmp, typ: recv.typ}}
	if a := callArgs(c); c.Name != m.Name && len(a) > 0 { // `xs.send(:each) { }`: the call it names
		pinned.Name = m.Name
		pinned.Arguments = &parser.ArgumentsNode{Location: c.Location, Arguments: a[1:]}
	}
	iter := false
	f.probe(func() { iter = f.genIterCall(&pinned, tail{}) }) // a form genIterCall does not take keeps callMethod0's error
	if !iter {
		return expr{}, false
	}
	f.emit("%s := %s", tmp, f.materialize(recv))
	f.genIterCall(&pinned, tail{})
	return expr{code: tmp, typ: recv.typ}, true
}

// nilSelf is recv as an Object free func's `self any`: nil itself, not a typed nil pointer (decision 362).
func (f *fctx) nilSelf(recv expr, m *Method) expr {
	if m.Owner.universal && isClass(recv.typ, "NilClass") && f.c.plainSelf(m) {
		recv.code = "rbNilAny(" + recv.code + ")"
	}
	return recv
}

// wrapperSelfCall is a wrapper body's self call: the owner's free func over its interface (any for Kernel), so no
// forwarder is selected (decision 209).
func (f *fctx) wrapperSelfCall(e *entry, argList string) (string, bool) {
	m := e.M
	targ := "any"
	if !m.Owner.universal {
		targ = f.c.ownerIface(f.owner, e)
	}
	switch {
	case targ != "" && f.c.plainSelf(m):
		return freeFuncName(m) + "(self" + comma(argList) + ")", true
	case targ != "":
		return freeFuncName(m) + "[" + targ + "](self" + comma(argList) + ")", true
	}
	return "", false
}

// dispatchCall renders a call that bypasses Go method dispatch (dispatch.go): a direct call or an rbSend_ type switch,
// for a dispatchable method on a struct-typed receiver. Not for the method a Method object binds (staticDef: MRI runs
// that definition, not an override) nor for a Dyn wrapper body's self call (its owner's free func, decision 209).
func (f *fctx) dispatchCall(e *entry, recv expr, argList string) (string, bool) {
	if code, ok := f.intOpCode(e.M, recv, argList); ok { // not a dispatch, but the same early exit (decision 381)
		return code, true
	}
	if f.staticDef == e.M || f.plainCalls && recv.code == "self" {
		return "", false
	}
	owner := f.staticStruct(recv)
	if owner == nil || !f.c.dispatchable(e.Owner, e.M) {
		return "", false
	}
	if owner.IsModule && !owner.universal && !e.Owner.universal && e.Owner != owner && e.Owner.IsModule {
		// self in a module body reaching a module it includes: everything that includes this module includes that
		// one, so self is one of its includers, asserted to its constraint (decision 234)
		mod := e.Owner.selfName() + "[any" + comma(strings.Join(f.c.moduleCallArgs(e), ", ")) + "]"
		return f.c.sendCall(e, "any("+recv.code+").("+mod+")", argList), true
	}
	if owner.IsModule && !owner.universal && !e.Owner.universal && e.Owner != owner {
		return "", false // self in a module body reaching another class's method: that is the constraint's
	}
	return f.c.sendCall(e, recv.code, argList), true
}

// staticStruct is the non-generic struct class a receiver is statically known to be an instance of (a subclass at run
// time), or nil: a class-typed value, or self in a struct class body.
func (f *fctx) staticStruct(recv expr) *Class {
	switch t := recv.typ.(type) {
	case TClass:
		if t.C.isStruct() && len(t.C.TypeParams) == 0 && !t.C.hashBase && !t.C.universal {
			return t.C
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && (f.owner.isStruct() && len(f.owner.TypeParams) == 0 || f.owner.IsModule) {
			return f.owner // a struct class's body, or a module's: self is some includer, a universal name dispatches over all
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid:
	}
	return nil
}

// methodExprType is the Go type a call on a receiver of static type t is written as a method expression of
// (`String.Upcase(s)`, `FooI.Bar(x)`), so the pruner keeps the method on that type and its implementers alone
// instead of on every class with the name (decision 122); "" keeps the plain `x.M()` form: a generic class, a
// module constraint (no method expressions on type parameters), or a struct's Dyn wrapper body, which shared
// arms compare textually.
func (f *fctx) methodExprType(m *Method, recv expr) string {
	if f.plainCalls {
		return ""
	}
	switch t := recv.typ.(type) {
	case TClass:
		if t.C.universal || t.C.IsModule || len(t.C.TypeParams) > 0 {
			return ""
		}
		if t.C.metaOf != nil && m.Name == "new" { // `new` is not in a metaclass's interface (its signature is the class's own); a class object is the concrete *Foo_Meta
			return "(*" + t.C.Name + ")"
		}
		if gt := f.c.goType(t); strings.HasPrefix(gt, "*") {
			return "(" + gt + ")"
		} else {
			return gt
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && f.owner.isStruct() && !f.owner.universal && !f.owner.IsModule && len(f.owner.TypeParams) == 0 {
			return f.owner.ifaceName()
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid:
	}
	return ""
}

// typeArgs are explicit because Go would infer from argument Go types (untyped const, *Foo, *Foo_Meta), not the Ruby binding.
func (f *fctx) typeArgs(m *Method, recvT Type, env map[string]Type) string {
	ts := []Type{}
	if m.Owner.GoType == "" && !f.c.plainSelf(m) {
		ts = append(ts, recvT)
	}
	for _, p := range m.Owner.TypeParams {
		ts = append(ts, env[p])
	}
	for _, p := range m.TypeParams {
		ts = append(ts, env[p])
		if o, ok := env[p].(TOpt); ok && isClassValue(o.Elem) { // the generic body's rbAs[U] boxes into it at run time
			if f.c.optConv == nil {
				f.c.optConv = map[string]bool{}
			}
			f.c.optConv[f.c.goType(o.Elem)] = true
		}
	}
	if len(ts) == 0 {
		return ""
	}
	gs := make([]string, len(ts))
	for i, t := range ts {
		if t == nil {
			return "" // unbound: leave it to Go's inference
		}
		if gs[i] = f.c.goType(t); gs[i] == "" {
			return ""
		}
	}
	return "[" + strings.Join(gs, ", ") + "]"
}

// hasForwarder reports whether the receiver's Go type carries a method for e.
func (f *fctx) hasForwarder(recvT Type, e *entry) bool {
	if e.Owner == nil || staticPrivate(e.M) || e.M.generic() {
		return false
	}
	switch t := recvT.(type) {
	case TClass:
		if t.C.universal || t.C.IsModule {
			return false
		}
		return f.c.wantsForwarder(t.C, *e)
	case TVar:
		if t.Name != "Self" || f.owner == nil || f.owner.universal {
			return false
		}
		if f.owner.isStruct() {
			return true
		}
		if f.owner.IsModule {
			return f.c.selfCalls(f.owner)[e.M.Name]
		}
		return e.Owner == f.owner
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no Go method set to forward to
	}
	return false
}

// selfReturnAsserts reports whether a `-> self` call on a struct value needs a Go type assertion: an inherited
// forwarder returns the defining class's interface. A universal method's is its typed free func (callCode).
func (c *Compiler) selfReturnAsserts(e *entry, recvT Type) bool {
	v, ok := e.M.Ret.(TVar)
	if !ok || v.Name != "Self" || e.Owner == nil || e.M.Owner.universal {
		return false
	}
	rc, ok := recvT.(TClass)
	return ok && rc.C.isStruct() && e.Entry != rc.C && !rc.C.leaf() // a leaf's generic call is instantiated with *C: it returns *C (decision 359)
}

// selfVarReturns reports whether a `-> self` call on self in a generic method body (MacOSVersion#strip_patch calling
// Version#major_minor) needs asserting back to Self: the Go interface method returns its class's interface.
func (c *Compiler) selfVarReturns(e *entry, recvT Type) bool {
	v, ok := e.M.Ret.(TVar)
	return ok && v.Name == "Self" && isSelfVar(recvT) && e.Owner != nil && !e.M.Owner.universal
}

// genInMethodRequire is a require in a method body: resolved at run time against what was compiled (decision 197),
// else a no-op for a library the prelude has, and a compile error for a file rugaroo would have to load.
func (f *fctx) genInMethodRequire(n *parser.CallNode) expr {
	if f.c.methodRequiresRuntime && !f.f.isPrelude() {
		return f.genRuntimeRequire(n)
	}
	switch {
	case n.Name == "require_relative" && !f.f.isPrelude():
		f.errorf(n, "require_relative must be a top-level statement: rugaroo loads files at compile time")
	case n.Name == "require" && f.c.findRequire(f.f, n) != "":
		f.errorf(n, "require of a file on the -I path must be a top-level statement: rugaroo loads files at compile time")
	}
	return expr{code: "", stmt: true, typ: TVoid{}}
}

// bindUntyped binds each of m's type parameters ts name that nothing bound yet to untyped.
func bindUntyped(m *Method, env map[string]Type, ts ...Type) {
	var vs []string
	for _, t := range ts {
		freeVars(t, &vs)
	}
	for _, v := range vs {
		if _, bound := env[v]; !bound && slices.Contains(m.TypeParams, v) {
			env[v] = TAny{}
		}
	}
}

// isClassValue: a concrete class type (no type variables, not Object's any), whose T? is a pointer to its Go type.
func isClassValue(t Type) bool {
	c, ok := t.(TClass)
	return ok && !c.C.universal && !mentionsVar(t)
}

// blocklessReturn converts a call's untyped result to the gem sig's `U | String` without the members only the
// absent block could bind (decision 321): `popen_read(cmd)` is a String where `popen_read(cmd) { |io| ... }` is not.
func (f *fctx) blocklessReturn(n parser.Node, m *Method, block parser.Node, env map[string]Type, out expr) expr {
	if m.retUnion == nil || !isAny(out.typ) || block != nil || m.Block == nil {
		return out
	}
	var blockVars []string
	freeVars(TFunc{Params: m.Block.Params, Ret: m.Block.Ret}, &blockVars)
	t := subst(dropVars(m.retUnion, blockVars), env)
	if isAny(t) || mentionsVar(t) {
		return out
	}
	return expr{code: f.coerce(n, out, t), typ: t}
}

// dropVars removes from a union return type (retUnion) the method type variables in vars, those only a block the
// call did not pass could bind (decision 321): Sorbet's `.returns(T.any(T.type_parameter(:U), String))` is String
// when no block is given.
func dropVars(t Type, vars []string) Type {
	u, ok := t.(TUnion)
	if !ok {
		return t
	}
	var keep []Type
	for _, m := range u.Members {
		if v, isVar := stripOpt(m).(TVar); isVar && slices.Contains(vars, v.Name) {
			continue
		}
		keep = append(keep, m)
	}
	if len(keep) == 0 {
		return TAny{}
	}
	return unionOf(keep...)
}

// foldInt is an Integer expression of literals only (`(1 << 64) - 1`), computed exactly.
func (f *fctx) foldInt(n parser.Node) (*big.Int, bool) {
	switch n := n.(type) {
	case *parser.IntegerNode:
		v, ok := new(big.Int).SetString(strings.ReplaceAll(f.f.text(n.Location), "_", ""), 0)
		return v, ok
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.foldInt(st.Body[0])
		}
	case *parser.CallNode:
		if n.Receiver == nil || n.Block != nil {
			return nil, false
		}
		a, ok := f.foldInt(n.Receiver)
		if !ok {
			return nil, false
		}
		args := callArgs(n)
		if n.Name == "-@" && len(args) == 0 {
			return new(big.Int).Neg(a), true
		}
		if len(args) != 1 {
			return nil, false
		}
		b, ok := f.foldInt(args[0])
		if !ok {
			return nil, false
		}
		switch n.Name {
		case "+":
			return new(big.Int).Add(a, b), true
		case "-":
			return new(big.Int).Sub(a, b), true
		case "*":
			return new(big.Int).Mul(a, b), true
		case "**":
			if f.intMode() == "promote" && b.Sign() >= 0 && b.IsInt64() && b.Int64() < 1024 { // promote only: a gem's overflowing power still raises by default
				return new(big.Int).Exp(a, b, nil), true
			}
		case "<<":
			if b.Sign() >= 0 && b.IsInt64() && b.Int64() < 1024 {
				return new(big.Int).Lsh(a, uint(b.Int64())), true //nolint:gosec // 0 <= b < 1024, checked above
			}
		}
	}
	return nil, false
}

// newSigsDiffer reports whether a subclass's new takes keywords or a rest that m's Go signature does not name alike.
func (f *fctx) newSigsDiffer(m *Method) bool {
	ps, _ := f.c.sig(m, nil)
	queue := slices.Clone(m.Owner.metaOf.Subclasses)
	for len(queue) > 0 {
		k := queue[0]
		queue = append(queue[1:], k.Subclasses...)
		if k.meta == nil {
			continue
		}
		if e := k.meta.lookup("new"); e != nil && e.M != m && (e.M.kwMask() || m.kwMask() || slices.ContainsFunc(e.M.Params, func(p Param) bool { return p.Keyword || p.KwRest })) {
			if qs, _ := f.c.sig(e.M, nil); qs != ps {
				return true
			}
		}
	}
	return false
}

// gemCallShortcut is a gem's call that decision 324 compiles otherwise: a `new` over subclasses of another keyword
// shape (dispatched at run time), unpack's wrapping twin, literal Integer arithmetic folded, a wrapping `<<`.
func (f *fctx) gemCallShortcut(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	if f.lenient() && f.m != nil && e.M.Name == "new" && e.M.Owner != nil && e.M.Owner.metaOf != nil && !recv.classObj && f.newSigsDiffer(e.M) {
		// a gem's `klass.new(h, s, *args, **kw)` over subclasses whose initialize takes other keywords (elftools'
		// Section.create): dispatched on the class object at run time (decision 324)
		return f.genDynCall(n, recv, "new", args), true
	}
	if f.lenient() && block == nil && (e.M.Name == "unpack" || e.M.Name == "unpack1") && e.M.Owner == f.c.classes["String"] {
		if w := f.c.classes["String"].lookup("__" + e.M.Name + "_wrap"); w != nil { // a gem's 64-bit fields wrap (decision 324)
			return f.callMethod(n, w, recv, args, block), true
		}
	}
	if f.lenient() && block == nil && e.M.Owner == f.c.classes["Integer"] {
		return f.gemIntegerCall(n, e, recv, args)
	}
	if call, ok := n.(*parser.CallNode); ok && call.Name == "**" && e.M.Name == "**" && f.intMode() == "promote" && e.M.Owner == f.c.classes["Integer"] {
		if v, ok := f.foldInt(n); ok && v.IsInt64() { // `10 ** 12` under promote: a constant that fits stays an Integer (decision 372)
			return expr{code: "Integer(" + v.String() + ")", typ: f.cls("Integer"), lit: true}, true
		}
	}
	return expr{}, false
}

// gemIntegerCall is a gem's Integer arithmetic: literals folded exactly and saturated to int64, `<<` mod 2**64, as
// Go's shift (decision 324).
func (f *fctx) gemIntegerCall(n parser.Node, e *entry, recv expr, args []parser.Node) (expr, bool) {
	call, isCall := n.(*parser.CallNode)
	if isCall && call.Name == e.M.Name { // not case's `when 0`, which hands its === the literal
		if v, ok := f.foldInt(n); ok {
			if v.IsInt64() {
				return expr{code: "Integer(" + v.String() + ")", typ: f.cls("Integer"), lit: true}, true
			}
			f.warn(n, "Integer %s does not fit in 64 bits (no Bignum); using the nearest", v)
			bound := "Integer(9223372036854775807)" // math.MaxInt64 spelled out: a gem local named math would shadow the package
			if v.Sign() < 0 {
				bound = "Integer(-9223372036854775808)"
			}
			return expr{code: bound, typ: f.cls("Integer"), lit: true}, true
		}
	}
	if e.M.Name == "<<" && len(args) == 1 && !isSplat(args[0]) {
		a := f.genExpr(args[0], f.cls("Integer"))
		return expr{code: "rbIntShlWrap(" + f.coerce(n, recv, f.cls("Integer")) + ", " + f.coerce(args[0], a, f.cls("Integer")) + ")", typ: f.cls("Integer")}, true
	}
	return expr{}, false
}

// untypedTwinArg: a gem's call whose first argument is untyped, of a method with decision-12 twins by class
// (`sub`'s `__sub_regexp`): the dispatcher picks the twin by the argument's run-time class (dynClassOverloads).
func (f *fctx) untypedTwinArg(e *entry, recvT Type, args []parser.Node, block parser.Node) bool {
	m := e.M
	rc, ok := recvT.(TClass)
	if !f.lenient() || block != nil || len(args) == 0 || !ok || strings.HasPrefix(m.Name, "__") || !classTwins(m.Name) {
		return false
	}
	prefix := "__" + overloadBase(m.Name) + "_"
	twin := slices.ContainsFunc(rc.C.methodSet(), func(x entry) bool {
		rest := strings.TrimPrefix(x.M.Name, prefix)
		// `__tcp_enum` is the block-less form, not a by-class twin (net-ftp's `Socket.tcp(host, port)`, decision 416)
		return rest != x.M.Name && rest != "" && rest != "enum" && (rest[0] < '0' || rest[0] > '9') && len(x.M.Params) == len(m.Params)
	})
	if !twin {
		return false
	}
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	return isAny(a.typ)
}

// nilKeyLookup is a gem's `h[k]` or `h.key?(k)` with k a K? where the Hash's K holds no nil (kramdown's
// `HTML_MARKDOWN_ATTR_MAP[attrs.delete('markdown')]`): MRI's nil key is a miss, so nil answers nil (or false).
func (f *fctx) nilKeyLookup(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	name := e.M.Name
	miss := map[string]string{"[]": "nil", "key?": "false", "has_key?": "false", "include?": "false", "member?": "false"}[name]
	if !f.lenient() || block != nil || len(args) != 1 || miss == "" || !isClass(recv.typ, "Hash") {
		return expr{}, false
	}
	kt := recv.typ.(TClass).Args[0]
	var a expr
	f.probe(func() { a = f.genExpr(args[0], nil) })
	o, ok := a.typ.(TOpt)
	if !ok || isOpt(kt) || isAny(kt) || !typeEq(o.Elem, kt) {
		return expr{}, false
	}
	k := f.newTmp()
	f.emit("%s := %s", k, f.genExpr(args[0], nil).code)
	hit := f.callEntry(n, e, recv, []parser.Node{&exprNode{Node: args[0], e: expr{code: derefOpt(k, a.typ), typ: kt}}}, nil)
	res := f.newTmp()
	if miss == "false" {
		f.emit("%s := Boolean(false)", res)
		f.emit("if %s {\n\t%s = %s\n}", optNotNil(k, a.typ), res, f.coerce(n, hit, f.cls("Boolean")))
		return expr{code: res, typ: f.cls("Boolean")}, true
	}
	t := optOf(hit.typ)
	f.emit("var %s %s", res, f.c.goType(t))
	f.emit("if %s {\n\t%s = %s\n}", optNotNil(k, a.typ), res, f.coerce(n, hit, t))
	return expr{code: res, typ: t}, true
}

// gemCallForm is a gem's call a typed call cannot make as written (decision 330): a nil key into a nil-free Hash, or an
// untyped first argument to a method with by-class twins (kramdown's `result.sub!(indent_re, ”)`, a Regexp or a
// String at run time), which the dispatcher picks.
func (f *fctx) gemCallForm(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) (expr, bool) {
	if r, ok := f.coreEachValue(n, e.M, recv, block); ok { // user code's, not a gem form: here for callMethod0's size
		return r, true
	}
	if r, ok := f.nilKeyLookup(n, e, recv, args, block); ok {
		return r, true
	}
	if f.untypedTwinArg(e, recv.typ, args, block) {
		return f.genDynCall(n, expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}, e.M.Name, args), true
	}
	return expr{}, false
}

func isASCII(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return r >= utf8.RuneSelf })
}

// provisional reports a round's untyped result of m, untyped only until its parameters are typed (decision 358).
func (c *Compiler) provisional(m *Method, t Type) bool { return c.round && m.roundRet && isAny(t) }

// leafSelf is a generic body's Self value (self, a `self`-typed parameter) where leaf class cls's *C is wanted: one
// assertion, as Self is always *C there (decision 359).
func (f *fctx) leafSelf(e expr, cls *Class) expr {
	if !isSelfVar(e.typ) || cls == nil || !cls.leaf() || f.m == nil || f.c.plainSelf(f.m) {
		return e
	}
	e.code = "any(" + e.code + ").(*" + cls.Name + ")"
	return e
}

// leafCall reports a receiver that is a leaf's object, typed so or self in its body: the method found is the one
// that runs, so the call is its free func, no forwarder (decision 359).
func (f *fctx) leafCall(recv expr) bool {
	return leafRecv(recv.typ) || isSelfVar(recv.typ) && f.owner != nil && f.owner.leaf()
}

// directCall is a call settled before the Go method or free func choice: a dispatcher, an operator body, a class constant's own method.
func (f *fctx) directCall(e *entry, selfRecv, recv expr, args []string, env map[string]Type) (string, bool) {
	argList := strings.Join(args, ", ")
	if code, ok := f.dispatchCall(e, selfRecv, argList); ok {
		return code, true
	}
	if code, ok := inlineOperator(e.M, recv, args); ok {
		return code, true
	}
	if exactMetaCall(recv, e.M) {
		return staticCallCode(e.M, f.typeArgs(e.M, recv.typ, env), selfRecv.code, argList), true
	}
	return "", false
}

// exactMetaCall: `Math.sqrt(x)` on the constant itself can only be Math's own singleton method, so its free func, which Go inlines, not a Math_MetaI call (decision 401).
func exactMetaCall(recv expr, m *Method) bool {
	c, ok := recv.typ.(TClass)
	return ok && recv.classObj && c.C.metaOf != nil && m.Owner == c.C && (m.Kind == kindDef || m.Kind == kindPrimitive) && !m.generic()
}

// operatorBodies are the prelude bodies inlineOperator writes in place of a call; arith ones fold if both sides are constant.
var operatorBodies = map[string]bool{
	"self + other": true, "self - other": true, "self / other": true, "Float(float64(self * other))": true,
	"Boolean(self < other)": false, "Boolean(self <= other)": false, "Boolean(self > other)": false, "Boolean(self >= other)": false,
	"Boolean(self == other)": false,
}

// inlineOperator: Go inlines `Float.Op_mul(a, b)` too, but leaves a NOP inline mark per call, a third of mandelbrot's inner loop (decision 401).
func inlineOperator(m *Method, recv expr, args []string) (string, bool) {
	if m.Kind != kindPrimitive || len(args) != 1 || len(m.Params) != 1 || m.Params[0].Name != "other" || classOf(recv.typ) != m.Owner || classOf(m.Params[0].Type) != m.Owner {
		return "", false
	}
	body := strings.TrimSpace(m.Node.Body.(*parser.StatementsNode).Body[0].(*parser.XStringNode).Unescaped.Value)
	arith, ok := operatorBodies[body]
	if !ok || arith && goConstExpr(recv.code) && goConstExpr(args[0]) { // `0.1 + 0.2` would fold exactly to 0.3
		return "", false
	}
	if recv.code == args[0] && !strings.Contains(body, "*") && !strings.Contains(body, "+") { // gocritic's dupSubExpr rejects `x == x` (NaN's test) and `x - x`
		return "", false
	}
	i, j := strings.Index(body, "self"), strings.Index(body, "other")
	code := body[:i] + goOperand(recv.code) + body[i+len("self"):j] + goOperand(args[0]) + body[j+len("other"):]
	if strings.HasPrefix(body, "self") { // a bare `a + b`: callers may append `.M()` to it
		code = "(" + code + ")"
	}
	return code, true
}

// goConstExpr reports Go code naming nothing but numbers and conversions, which Go evaluates as a constant.
func goConstExpr(code string) bool {
	var s scanner.Scanner
	fs := token.NewFileSet()
	s.Init(fs.AddFile("", -1, len(code)), []byte(code), nil, 0)
	for {
		_, tok, lit := s.Scan()
		switch {
		case tok == token.EOF:
			return true
		case tok == token.IDENT && lit != "Float" && lit != "Integer" && lit != "float64" && lit != "int64",
			tok != token.IDENT && !tok.IsLiteral() && !tok.IsOperator():
			return false
		}
	}
}

// goOperand parenthesizes code that is an operator expression.
func goOperand(code string) string {
	switch x, err := goparser.ParseExpr(code); x.(type) {
	case *ast.Ident, *ast.BasicLit, *ast.CallExpr, *ast.SelectorExpr, *ast.ParenExpr, *ast.IndexExpr:
		if err == nil {
			return code
		}
	}
	return "(" + code + ")"
}

// leafRecv reports a receiver typed as a leaf class (decision 359).
func leafRecv(t Type) bool {
	c, ok := t.(TClass)
	return ok && c.C.leaf()
}

// leafRecvSelf is leafSelf for the receiver of m: only a plain func takes *C, a generic one takes Self as it is.
func (f *fctx) leafRecvSelf(recv expr, m *Method) expr {
	if !f.c.plainSelf(m) {
		return recv
	}
	return f.leafSelf(recv, m.Owner)
}

// flatCall is flatLeafOpt for a call of m: only when the code calls the generic func itself, since a forwarder method
// has opened its box already.
// valOptNils: `reduce(nil) { }` binds A to Integer? by the block, after its nil argument was coerced (decision 396).
func (f *fctx) valOptNils(m *Method, codes []string, restIdx int, env map[string]Type) {
	for i, p := range m.Params {
		if i < len(codes) && (restIdx < 0 || i < restIdx) && codes[i] == "nil" && valOpt(subst(p.Type, env)) && !boxedValOpt(p.Type, env) {
			codes[i] = f.c.goType(subst(p.Type, env)) + "{}"
		}
	}
}

// genericOpt is flatOpt for code generic Go produced as *E: an Integer?/Float? is copied out to its rbOpt (decision 396).
func genericOpt(e expr) expr {
	if valOpt(e.typ) {
		e.code = "rbOptFromPtr(" + e.code + ")"
		return e
	}
	if o, ok := e.typ.(TOpt); ok {
		if c, ok := o.Elem.(TClass); ok && nilObj(c) { // a leaf's C? is *C: Go's generic **C opened (decision 360; `a, b = tokens` of a Struct class, regexp_parser's Lexer)
			e.code = "rbFlatNil(" + e.code + ")"
			return e
		}
	}
	return flatOpt(e)
}

func flatCall(code string, m *Method, env map[string]Type) string {
	if flat := flatLeafOpt(code, m.Ret, env); flat != code && (m.Owner == nil || strings.HasPrefix(code, freeFuncName(m)) || strings.HasPrefix(code, "rbSend_")) {
		return flat
	}
	return code
}

// flatLeafOpt is a call's code with a generic V? result, V = *C a leaf's, opened from Go's **C to the *C its C? is
// (decision 360).
func flatLeafOpt(code string, decl Type, env map[string]Type) string {
	if boxedLeafOpt(decl, env) != nil {
		return "rbFlatNil(" + code + ")"
	}
	if boxedValOpt(decl, env) {
		if rest, ok := strings.CutPrefix(code, "Array_Op_idx["); ok { // the hot read: built as an rbOpt, not a *E opened again
			return "rbArrAt[" + rest
		}
		if rest, ok := strings.CutPrefix(code, "Hash_Op_idx["); ok {
			return "rbHashAt[" + rest
		}
		return "rbOptFromPtr(" + code + ")" // the copy: Array#[]'s pointer into its storage must not outlive a write (decision 396)
	}
	if t, ok := decl.(TClass); ok && t.C.RubyName == "Array" && len(t.Args) == 1 { // values_at's Array[E?]
		if c := boxedLeafOpt(t.Args[0], env); c != nil {
			return "rbFlatArray(" + code + ")"
		}
		if boxedValOpt(t.Args[0], env) {
			return "rbOptArrayFrom(" + code + ")"
		}
	}
	if t, ok := decl.(TClass); ok && len(t.Args) == 1 && (t.C.RubyName == "Array" || t.C.RubyName == "Enumerator") { // zip's [E, U?] pairs
		if p, ok := t.Args[0].(TTuple); ok && len(p.Elems) == 2 && !isOpt(p.Elems[0]) && boxedValOpt(p.Elems[1], env) {
			if rest, ok := strings.CutPrefix(code, "Array___ZipViewEnum["); ok { // its pairs built as rbOpt, no per-pair adapter
				return "rbZipViewEnumVal[" + rest
			}
			return "rbZipVal" + t.C.RubyName + "(" + code + ")" // ponytail: Lazy[[E, U?]] keeps the *U box
		}
		if p, ok := t.Args[0].(TTuple); ok && len(p.Elems) == 3 && t.C.RubyName == "Array" && boxedValOpt(p.Elems[1], env) && boxedValOpt(p.Elems[2], env) {
			return "rbZip2ValArray(" + code + ")" // ponytail: one of U, V an Integer/Float and the other not keeps the boxes
		}
	}
	if t, ok := decl.(TTuple); ok && len(t.Elems) == 2 { // minmax's [E?, E?] ponytail: other tuple and container shapes keep the boxes
		a, b := boxedLeafOpt(t.Elems[0], env), boxedLeafOpt(t.Elems[1], env)
		if a != nil && b != nil {
			return "rbFlatPair[" + a.objType() + ", " + b.objType() + "](" + code + ")"
		}
		if boxedValOpt(t.Elems[0], env) && boxedValOpt(t.Elems[1], env) {
			return "rbOptPairFrom(" + code + ")"
		}
	}
	return code
}
