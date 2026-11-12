package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// ---- calls

// stringMutators are MRI's in-place String methods without a bang, which rugaroo's immutable Strings lack.
var stringMutators = map[string]bool{"<<": true, "concat": true, "append_as_bytes": true, "[]=": true, "insert": true, "prepend": true, "replace": true, "clear": true, "setbyte": true}

const noEval = "rugaroo compiles the program ahead of time and the binary has no interpreter, so code cannot be built at run time"

// whyUndefined raises the undefined-method error with its reason when the method is one rugaroo leaves out on purpose.
func (f *fctx) whyUndefined(n parser.Node, t TClass, recv expr, name string) {
	if plain := strings.TrimSuffix(name, "!"); t.C.RubyName == "String" && plain != name && t.C.lookup(plain) != nil {
		f.errorf(n, "undefined method %s for String: Strings are immutable in rugaroo, so assign the result of %s instead (decision 136)", name, plain)
	}
	if t.C.RubyName == "String" && stringMutators[name] {
		f.errorf(n, "undefined method %s for String: Strings are immutable in rugaroo; build a new one (s += x, s = s.sub(...)), write into a StringIO (decision 58), or append to (decision 382) or setbyte (decision 395) a String local nothing else holds", name)
	}
	if name == "eval" {
		f.errorf(n, "undefined method eval for %s: %s", recv.typ, noEval)
	}
}

// genKernelIntrinsic handles the receiverless calls the compiler builds itself.
func (f *fctx) genKernelIntrinsic(n *parser.CallNode, expected Type) (expr, bool) {
	switch n.Name {
	case "raise", "fail":
		return f.genRaise(n), true
	case "require_relative", "require":
		return f.genInMethodRequire(n), true
	case "__method__":
		if n.Arguments != nil || n.Block != nil {
			return expr{}, false
		}
		if f.m == nil {
			return expr{code: "nil", typ: TNil{}}, true
		}
		return expr{code: "Symbol(" + strconv.Quote(f.m.Name) + ")", typ: f.cls("Symbol")}, true
	case "__dir__":
		if n.Arguments != nil || n.Block != nil {
			return expr{}, false
		}
		return expr{code: "rbSourceDir(" + strconv.Quote(f.f.Name) + ")", typ: f.cls("String")}, true
	case "lambda", "proc":
		if bn, ok := n.Block.(*parser.BlockNode); ok && n.Arguments == nil {
			return f.genLambda(n, bn, bn.Parameters, expected), true
		}
		f.errorf(n, "%s needs a literal block", n.Name)
	case "block_given?":
		if n.Arguments != nil || n.Block != nil {
			break
		}
		switch {
		case f.m.optionalBlockLocal() != "":
			c, _ := f.genCond(n)
			return expr{code: "Boolean(" + c + ")", typ: f.cls("Boolean")}, true
		case f.m != nil && f.m.Block != nil: // a required block (or an iterator's loop body) is always there
			return expr{code: "Boolean(true)", typ: f.cls("Boolean")}, true
		}
		return expr{code: "Boolean(false)", typ: f.cls("Boolean")}, true
	case "alias_method":
		if f.c.dmSites[n] {
			return expr{code: "nil", typ: TNil{}, lit: true}, true // boot made its alias (decision 330)
		}
		return expr{}, false
	case "define_method", "define_singleton_method":
		if f.c.dmSites[n] {
			return expr{code: "nil", typ: TNil{}, lit: true}, true // boot made its methods defs (decision 198)
		}
		if rec := f.c.smSites[n]; rec != nil {
			return f.genExpr(rec, expected), true
		}
		f.errorf(n, "%s is not supported", n.Name)
	case "send", "__send__", "public_send":
		if len(callArgs(n)) == 0 {
			f.errorf(n, "%s needs a method name", n.Name)
		}
		return f.genSend(n, expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}, n.Name, callArgs(n), n.Block), true // `send(:x)` is self's
	case "binding", "method_missing":
		if f.ownBinding(n) {
			return expr{}, false
		}
		f.errorf(n, "%s is not supported", n.Name)
	}
	return expr{}, false
}

// ownBinding is a receiverless `binding` the class defines itself (rubocop-ast's NodePattern::Compiler keeps a Binding
// of its own as an attr_reader): an ordinary call, where Kernel#binding is unsupported.
func (f *fctx) ownBinding(n *parser.CallNode) bool {
	if n.Name != "binding" {
		return false
	}
	e := f.resolve(f.selfType, n.Name)
	return e != nil && e.M.File != nil && !e.M.File.isPrelude()
}

// genKernelCall compiles a receiverless call, or `Kernel.x` for one of Kernel's module functions, as a Kernel intrinsic when it is one (decision 139).
func (f *fctx) genKernelCall(n *parser.CallNode, expected Type) (expr, bool) {
	if n.Receiver == nil && f.owner == nil && f.m == nil && (n.Name == "public" || n.Name == "private") && n.Block == nil &&
		!slices.ContainsFunc(callArgs(n), func(a parser.Node) bool { _, sym := a.(*parser.SymbolNode); return !sym }) {
		// main's `public`/`private`: top-level defs are private methods of Object already, and rugaroo calls them only
		// receiverless, so a visibility switch there changes nothing it compiles (decision 364)
		return expr{code: "nil", typ: TNil{}, lit: true}, true
	}
	if n.Receiver == nil {
		return f.genKernelIntrinsic(n, expected)
	}
	runtimeRequire := n.Name == "require" && f.c.methodRequiresRuntime && !f.f.isPrelude() // `Kernel.require path` (decision 246)
	if !isTopConst(n.Receiver, "Kernel") || !kernelModuleNames[n.Name] && !runtimeRequire || n.Name == "require" && !runtimeRequire || n.Name == "require_relative" {
		return expr{}, false
	}
	t := f.probeType(n.Receiver, nil)
	if !kernelMeta(t) {
		return expr{}, false
	}
	bare := *n
	bare.Receiver = nil
	return f.genKernelIntrinsic(&bare, expected)
}

// genFrozenLiteral is `{...}.freeze` or `[...].freeze` where a type is expected: freeze answers its receiver, so the
// literal is built as that type (Homebrew's `T.let({ alpine: ->(query) { ... } }.freeze, T::Hash[Symbol, T.proc...])`
// types each lambda's parameter).
func (f *fctx) genFrozenLiteral(n *parser.CallNode, expected Type) (expr, bool) {
	if n.Name != "freeze" || expected == nil || n.Block != nil || len(callArgs(n)) > 0 {
		return expr{}, false
	}
	switch n.Receiver.(type) {
	case *parser.HashNode, *parser.ArrayNode:
	default:
		return expr{}, false
	}
	lit := f.genExpr(n.Receiver, expected)
	frozen := *n
	frozen.Receiver = &exprNode{Node: n.Receiver, e: lit}
	return f.genCall(&frozen, nil), true
}

func (f *fctx) genCall(n *parser.CallNode, expected Type) expr {
	defer f.afterCall(n) // after the arguments, which still read narrowed ivars
	if e, ok := f.genFusedIndexAssign(n); ok {
		return e
	}
	if e, ok := f.hoistedStrCall(n); ok {
		return e
	}
	if e, ok := f.genCallRewrite(n, expected); ok {
		return e
	}
	if e, ok := f.genFrozenLiteral(n, expected); ok {
		return e
	}
	if e, ok := f.lenientCall(n); ok {
		return e
	}
	if e, ok := f.genCallForm(n, expected); ok {
		return e
	}
	if strings.HasSuffix(n.Name, "=") && (n.Receiver == nil || isSelf(n.Receiver)) {
		f.unnarrow(attrNarrowKey(strings.TrimSuffix(n.Name, "=")))
	}
	if e, ok := f.genERBCall(n); ok {
		return e
	}
	if err, ok := f.c.anonErrors[n]; ok {
		panic(err)
	}
	if cls := f.c.anonClasses[n]; cls != nil {
		return f.genAnonClass(n, cls, expected)
	}
	if cls := f.classRef(n.Receiver); cls != nil {
		if err, bad := f.c.badClasses[cls]; bad {
			panic(err)
		}
		if e, ok := f.genSpecialClassCall(n, cls, expected); ok {
			return e
		}
		// Direct constructor unless Foo defines self.new; Hash is the one @go_type class with a Go constructor (NewHash).
		// A generic @go_type class's own self.new takes arguments; bare `.new` is its annotated zero value.
		if f.useGenNew(cls, n) {
			return f.genNew(n, cls, callArgs(n), nil, expected, n.Block)
		}
		if cls.meta == nil {
			f.errorf(n, "%s has no class methods", cls.RubyName)
		}
		recv := expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	if n.IsSAFE_NAVIGATION() {
		return f.genSafeNav(n)
	}
	var recv expr
	switch {
	case (n.Receiver == nil || isSelf(n.Receiver)) && f.dynSelfCall(n.Name):
		recv = expr{code: "any(" + f.selfCode + ")", typ: TAny{}}
	case n.Receiver == nil:
		recv = expr{code: f.selfCode, typ: f.selfType, classObj: f.selfClassObj}
	default:
		recv = f.valueOf(f.genExpr(n.Receiver, nil))
	}
	if raisesFirst(recv) { // `Missing.record(x)`: the NameError comes first (decision 363)
		return recv
	}
	f.noteElems(n)
	f.widenIvarElem(n)
	if e, ok := f.genInstanceEval(n, recv); ok {
		return e
	}
	if v, ok := f.iterCallValue(n, recv); ok {
		return v
	}
	return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
}

func (f *fctx) genSafeNav(n *parser.CallNode) expr {
	recv := f.genExpr(n.Receiver, nil)
	u, nilUnion := recv.typ.(TUnion)
	nilUnion = nilUnion && slices.ContainsFunc(u.Members, isNil)
	if !isOpt(recv.typ) && !isAny(recv.typ) && !nilUnion {
		return f.genMethodCall(n, recv, n.Name, callArgs(n), n.Block)
	}
	rt := f.newTmp()
	inner := expr{code: rt, typ: recv.typ} // an untyped nil is nil too
	if o, ok := recv.typ.(TOpt); ok {
		inner = expr{code: derefOpt(rt, o), typ: o.Elem}
	}
	if nilUnion { // `@url&.to_s` of a String | URL | nil: a union holds nil as Go's nil, and the call sees the rest
		inner = expr{code: rt, typ: unionOf(slices.DeleteFunc(slices.Clone(u.Members), isNil)...)}
	}
	call := func() expr { return f.genMethodCall(n, inner, n.Name, callArgs(n), n.Block) }
	probe, once := f.genOperandOnce(call)
	if vt := stripOpt(probe.e.typ); isVoid(vt) || isNil(vt) { // also a union whose members' calls are void or nil (`error_pipe&.close`)
		f.emit("if %s := %s; %s {", rt, recv.code, optNotNil(rt, recv.typ))
		f.indent++
		e := f.operandAgain(probe, once, call)
		f.emitExprStmt(n, e)
		f.indent--
		f.emit("}")
		return expr{code: "", typ: TVoid{}, stmt: true}
	}
	resT := optOf(probe.e.typ)
	tmp := f.newTmp()
	f.emit("var %s %s", tmp, f.c.goType(resT))
	f.emit("if %s := %s; %s {", rt, recv.code, optNotNil(rt, recv.typ))
	f.indent++
	e := f.operandAgain(probe, once, call)
	f.emit("%s = %s", tmp, f.coerce(n, e, resT))
	f.indent--
	f.emit("}")
	return expr{code: tmp, typ: resT}
}

// resolve finds the method entry for name on a receiver type, or nil.
// Top-level defs are private methods on Object, so a receiver-less call
// anywhere can reach them.
func (f *fctx) resolve(recvT Type, name string) *entry {
	var e *entry
	switch t := recvT.(type) {
	case TClass:
		e = t.C.lookup(name)
	case TVar:
		if t.Name == "Self" && f.owner != nil {
			e = f.owner.lookup(name)
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // no class here to look in: only a top-level def can answer
	}
	if e == nil || f.c.topOverKernel(e, name) {
		if td := f.c.topDefs[name]; td != nil {
			e = &entry{M: td}
		}
	}
	if e != nil {
		f.c.inferRet(e.M)
	}
	return e
}

// raisesFirst is a receiver whose evaluation raises (a constant nothing defines, decision 363): the call is never made.
func raisesFirst(recv expr) bool {
	return recv.noreturn && recv.stmt || strings.HasPrefix(recv.code, "rbConstRead[any](false, nil, ")
}

// coreEach is a core each-like iterator, whose value is its receiver (decision 365).
func coreEach(m *Method) bool {
	return eachReturnsSelf[m.Name] && m.File != nil && m.File.isPrelude() && isVoid(m.Ret)
}

// topOverKernel reports a Kernel method a top-level def of the same name hides: the def is Object's, ahead of Kernel
// in every ancestry (decision 363).
func (c *Compiler) topOverKernel(e *entry, name string) bool {
	return e.Owner != nil && e.Owner.RubyName == "Kernel" && c.topDefs[name] != nil
}

// storeNames are the Hash and Array methods that put their arguments in the container: a gem's untyped value converted
// there could hold another class at run time, while a converted lookup key (`@nodes.delete(elem)`) only raises
// TypeError where MRI answers nil (ponytail).
var storeNames = map[string]bool{"<<": true, "push": true, "append": true, "unshift": true, "prepend": true, "insert": true, "[]=": true, "store": true, "fill": true, "concat": true, "replace": true, "merge!": true, "update": true}

// genMethodCall dispatches a call on an already-generated receiver.
func (f *fctx) genMethodCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if storeNames[name] && (isClass(recv.typ, "Hash") || isClass(recv.typ, "Array")) {
		f.storing++
		defer func() { f.storing-- }()
	}
	if name == "__getobj__" && len(args) == 0 && block == nil {
		return f.getobjCall(n, recv)
	}
	if k := f.valueAnnRecv(recv); k != nil && slices.Contains(valueOps, name) {
		f.errorf(n, "%s on %s, a # @value class: its copies have no identity and cannot be changed (decision 441)", name, k.RubyName)
	}
	if e, ok := f.lenientLookup(n, recv, name, args, block); ok {
		return e
	}
	if e, ok := f.nilSpaceship(n, recv, name, args, block); ok {
		return e
	}
	return f.genMethodCall1(n, recv, name, args, block)
}

// getobjCall is `__getobj__`, typed by what the delegator wraps when delegateType knows it (decision 284).
func (f *fctx) getobjCall(n parser.Node, recv expr) expr {
	x := f.genMethodCall1(n, recv, "__getobj__", nil, nil)
	cls := classOf(recv.typ)
	if isSelfVar(recv.typ) {
		cls = f.owner
	}
	if dt := f.c.delegateType(cls); dt != nil && isAny(x.typ) {
		return expr{code: f.coerce(n, x, dt), typ: dt}
	}
	return x
}

func (f *fctx) genMethodCall1(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if _, ok := recv.typ.(TVoid); ok { // a void call's value is nil, as in Ruby: run it, then call on nil (decision 119)
		recv = f.voidAsNil(recv)
	}
	if e, ok := f.genIntrinsic(n, recv, name, args, block); ok {
		return e
	}
	if e, ok := f.sureTakeCall(n, recv, name, args, block); ok {
		return e
	}
	if x, ok := f.bignumFallback(recv, name); ok {
		return f.genMethodCall(n, x, name, args, block)
	}
	if e, ok := f.lenientMember(n, recv, name, args); ok {
		return e
	}
	if meta := f.metaOfType(recv.typ); name == "new" && meta != nil && len(meta.metaOf.Subclasses) == 0 {
		if vr := meta.metaOf.valueRoot(); vr != nil && generatedInit(meta.metaOf) { // `klass.new(a: 1)` on a held Struct/Data class: keywords by member, as genNew's
			args = f.keywordMembers(n, vr, args)
		}
	}
	if e, ok := f.classObjCall(n, recv, name, args, block); ok {
		return e
	}
	f.checkEncodingNames(n, recv, name, args)
	if recv.view != "" && block == nil {
		if e, ok := f.viewCall(n, recv, name, args); ok {
			return e
		}
	}
	if len(args) > 0 && !isUnion(recv.typ) && slices.ContainsFunc(args, f.readsUnionLocal) {
		// an argument over a union local may need unionArgCall: generated once
		// here, the failed dispatch and the retry reuse it instead of each
		// regenerating the subtree (exponential in its nesting otherwise)
		args = f.pinArgs(args, len(args))
	}
	if len(args) > 0 && !isUnion(recv.typ) && !allLiteral(args) {
		// a union argument the method does not take as it is: once per member (decision 150)
		var e expr
		err := f.attempt(func() { e = f.dispatch(n, recv, name, args, block) })
		if err == nil {
			return f.narrowRaises(name, args, e)
		}
		if e, ok := f.unionArgCall(n, recv, name, args, block); ok {
			return e
		}
		if f.lenient() && block == nil && f.unionArm == 0 { // gem code: matched per call at run time, as MRI does
			return f.genDynCall(n, recv, name, args)
		}
		panic(*err)
	}
	return f.narrowRaises(name, args, f.dispatch(n, recv, name, args, block))
}

// narrowRaises types `assert_raises(K) { }` and `_ { }.must_raise(K)` with
// one class literal as K, not Exception (decision 116, #37): the result is
// an exception the assertion checked is a K, so the Go type assertion
// cannot fail.
func (f *fctx) narrowRaises(name string, args []parser.Node, e expr) expr {
	if name != "assert_raises" && name != "must_raise" || len(args) != 1 {
		return e
	}
	exc := f.c.classes["Exception"]
	got, ok := e.typ.(TClass)
	if !ok || got.C != exc {
		return e
	}
	cls := f.classRef(args[0])
	if cls == nil || cls == exc || !cls.isSubclassOf(exc) {
		return e
	}
	want := TClass{C: cls}
	return expr{code: "(" + e.code + ").(" + f.c.goType(want) + ")", typ: want, assert: true}
}

// genBareName is a receiverless, argumentless call that names a narrowed
// attribute, or a caller's local inside a template parsed on its own
// (decision 111).
func (f *fctx) genBareName(n *parser.CallNode, expected Type) (expr, bool) {
	if n.Receiver != nil || n.Arguments != nil || n.Block != nil {
		return expr{}, false
	}
	if v := f.scope.lookup(attrNarrowKey(n.Name)); v != nil {
		return expr{code: v.goName, typ: v.typ}, true
	}
	if f.erbDepth > 0 && f.visibleLocal(n.Name) != nil {
		return f.genExpr(&parser.LocalVariableReadNode{Name: n.Name, Location: n.Location}, expected), true
	}
	return expr{}, false
}

// genSpecialClassCall is the class-method calls the compiler answers itself: Ractor.new (decision 103), ERB.new (decision 111) and URI.open (decision 134).
func (f *fctx) genSpecialClassCall(n *parser.CallNode, cls *Class, expected Type) (expr, bool) {
	switch {
	case cls.RubyName == "Ractor":
		return f.genRactorCall(n, cls)
	case cls.RubyName == "URI" && n.Name == "open":
		return f.genOpenURI(n, nil, "open", callArgs(n), n.Block), true
	case cls.RubyName == "ERB" && n.Name == "new":
		f.c.erbNewTemplate(f.f, n) // checked here; the object only marks the template
		return expr{code: "NewERB()", typ: TClass{C: cls}, ctor: true}, true
	case n.Name == "new" && cls.RubyName == "Proc" && n.Block != nil:
		if bn, ok := n.Block.(*parser.BlockNode); ok && n.Arguments == nil { // Proc.new { } is proc { }
			return f.genLambda(n, bn, bn.Parameters, expected), true
		}
		f.errorf(n, "Proc.new needs a literal block")
	case n.Name == "new" && cls.universal && !cls.IsModule:
		// a plain object, made to be unique: specs compare it by identity (#56)
		if args := callArgs(n); len(args) > 0 {
			f.errorf(n, "wrong number of arguments (given %d, expected 0)", len(args))
		}
		return expr{code: "&" + cls.Name + "{}", typ: TClass{C: cls}, ctor: true}, true
	case n.Name == "new" && cls.isSubclassOf(f.c.classes["BasicSocket"]) && cls.lookup("initialize") == nil:
		// a socket object is only made with its handle: BasicSocket/IPSocket/Socket.new would be a nil one (decision 135)
		if cls.isSubclassOf(f.c.classes["Socket"]) {
			f.errorf(n, "Socket.new is not supported (decision 135): raw sockets bind and connect to packed sockaddr Strings; use TCPSocket, TCPServer, UDPSocket, UNIXSocket or Socket.tcp")
		}
		f.errorf(n, "undefined method initialize for %s (decision 135): use TCPSocket, TCPServer, UDPSocket, UNIXSocket or Socket.tcp", cls.RubyName)
	}
	return expr{}, false
}

// viewCall sends a block-less call on a converted Array/Hash (expr.view)
// to the value itself, dynamically: the conversion is a copy unless the
// value already is Array[untyped], and Ruby's call would mutate the value.
// The result is typed as the typed call's. Calls that cannot be sent
// dynamically (blocks, generic methods) use the copy.
func (f *fctx) viewCall(n parser.Node, recv expr, name string, args []parser.Node) (expr, bool) {
	t, ok := recv.typ.(TClass)
	if !ok || !converts(t) {
		return expr{}, false
	}
	if e, private := f.c.dynEntry(t.C, name); e == nil || private {
		return expr{}, false
	}
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			return expr{}, false
		}
	}
	var typed expr
	f.probe(func() { typed = f.dispatch(n, recv, name, args, nil) })
	f.c.noteDyn(name)
	codes := []string{"rbCall", recv.view}
	for _, a := range args {
		codes = append(codes, f.coerce(a, f.genExpr(a, nil), TAny{}))
	}
	raw := expr{code: rbDynFn(name) + "(" + strings.Join(codes, ", ") + ")", typ: TAny{}}
	// The raw result stays the view: `x << 1 << 2` sends both to the value.
	return expr{code: f.coerce(n, raw, typed.typ), typ: typed.typ, view: raw.code}, true
}

// genIntrinsic compiles the methods the transpiler answers itself:
// class, with (Data), respond_to?, is_a?/kind_of?.
func (f *fctx) genIntrinsic(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if e, ok, stop := f.intrinsicGate(n, recv, name, args, block); stop {
		return e, ok
	}
	if f.isOpenURIRecv(recv.typ, name) {
		return f.genOpenURI(n, &recv, name, args, block), true
	}
	if e, ok := f.rubyShapedCall(n, recv, name, args, block); ok {
		return e, true
	}
	if name == "class" && len(args) == 0 && block == nil {
		if e, ok := f.genClassOf(n, recv); ok {
			return e, true
		}
	}
	if name == "with" && block == nil {
		if e, ok := f.genDataWith(n, recv, args); ok {
			return e, true
		}
	}
	if name == "respond_to?" && block == nil && len(args) >= 1 {
		if e, ok := f.genRespondTo(n, recv, args); ok {
			return e, true
		}
		return f.genDynRespondTo(n, recv, args), true
	}
	if (name == "send" || name == "__send__" || name == "public_send") && len(args) >= 1 && !f.ownSend(recv.typ, name) {
		return f.genSend(n, recv, name, args, block), true
	}
	if e, ok := f.genMethodObject(n, recv, name, args, block); ok {
		return e, true
	}
	if name == "is_a?" || name == "kind_of?" {
		if len(args) != 1 || block != nil {
			f.errorf(n, "%s takes one class", name)
		}
		check := f.isACheck(n, recv, args[0])
		return expr{code: "Boolean(" + check + ")", typ: f.cls("Boolean"), lit: check == "true" || check == "false"}, true
	}
	// `!=` is `!(==)` unless overridden: BasicObject#!= as a free func over any would bind == to identity.
	if name == "!=" {
		if e := f.resolve(recv.typ, name); e == nil || e.Owner == f.c.classes["BasicObject"] {
			eq := f.genMethodCall(n, recv, "==", args, block)
			return expr{code: "Boolean(!(" + f.truthy(n, eq) + "))", typ: f.cls("Boolean")}, true
		}
	}
	return expr{}, false
}

// ownSend is true when the receiver's class defines its own send (Ractor's, BasicSocket's), which shadows Kernel#send as in MRI.
func (f *fctx) ownSend(t Type, name string) bool {
	if name != "send" {
		return false
	}
	c := classOf(stripOpt(t)) // a Socket? is a Socket's send too
	return c != nil && c.lookup(name) != nil
}

// dispatch resolves a call by the receiver's static type.
func (f *fctx) dispatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	switch t := recv.typ.(type) {
	case TOpt:
		return f.optCall(n, recv, name, args, block)
	case TUnion:
		return f.unionCall(n, recv, t, name, args, block)
	case TTuple:
		return f.tupleCall(n, recv, name, args, block)
	case TFunc:
		if t.Proc {
			return f.procCall(n, recv, t, name, args, block)
		}
	case TClass:
		if e, ok := f.methodValueCall(n, recv, t, name, args, block); ok {
			return e
		}
		return f.classCall(n, t, recv, name, args, block)
	case TVar:
		if t.Name == "Self" && f.owner != nil {
			return f.selfCall(n, recv, name, args, block)
		}
		return f.universalCall(n, recv, name, args, block)
	case TAny, TNil:
		return f.universalCall(n, recv, name, args, block)
	case TVoid: // a void call's value is nil, as in Ruby: run it, then call on nil (`log(x).nil?`)
		return f.universalCall(n, f.voidAsNil(recv), name, args, block)
	}
	f.errorf(n, "undefined method %s for %s", name, recv.typ)
	return expr{}
}

// voidAsNil runs a void call as a statement and stands nil in for its value (decision 119).
func (f *fctx) voidAsNil(recv expr) expr {
	if recv.code != "" && !recv.done {
		f.emit("%s", recv.code)
	}
	return expr{code: "nil", typ: TNil{}}
}

// partlyLoaded is a block-less call, in a gem method, on a gem class that may define the method in a file the
// traced run never loaded.
func (f *fctx) partlyLoaded(c *Class, block parser.Node) bool {
	return f.lenient() && f.m != nil && block == nil && c.isStruct() && c.File != nil && !c.File.isPrelude() && !c.universal
}

// classCall dispatches on a class-typed receiver.
func (f *fctx) classCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	f.noteYielderFeed(recv, name, args)
	if t.C.RubyName == "String" && (name == "end_with?" || name == "start_with?") && len(args) == 1 && isSplat(args[0]) && block == nil {
		// `path.end_with?(*DL_EXTENSIONS)` (rubocop's ResultCache): the list's length is a run-time fact, zero included
		return f.genMethodCall(n, recv, "__"+strings.TrimSuffix(name, "?")+"_any", []parser.Node{args[0].(*parser.SplatNode).Expression}, nil)
	}
	if e, ok := f.moduleCmp(t, recv, name, args, block); ok {
		return e
	}
	if isAbstract(t) && recv.code != f.selfCode {
		return f.abstractCall(n, t, recv, name, args, block)
	}
	e := t.C.lookup(name)
	if (e == nil || f.c.topOverKernel(e, name)) && recv.code == f.selfCode {
		if td := f.c.topDefs[name]; td != nil {
			e = &entry{M: td}
		}
	}
	if e == nil {
		if mm := t.C.lookup("method_missing"); mm != nil {
			if recv.code == f.selfCode {
				f.missingCalls++
			}
			return f.callMissing(n, mm, recv, name, args, block)
		}
		if f.c.isDelegator(t.C) {
			return f.delegateCall(n, recv, name, args, block)
		}
		// a Module/Class-typed value is some class object, and a
		// struct-typed value may be a subclass defining the method:
		// either way the method is found at run time
		if (t.C.RubyName == "Module" || t.C.RubyName == "Class" || (t.C.isStruct() && t.C.descendantDefines(name, false))) && block == nil {
			return f.genDynCall(n, recv, name, args)
		}
		f.whyUndefined(n, t, recv, name)
		if f.partlyLoaded(t.C, block) { // a gem class whose methods are not all in the closed world (Cask::Cask when the traced run never loaded cask/cask.rb): found, or not, at run time
			return f.genDynCall(n, recv, name, args)
		}
		f.errorf(n, "undefined method %s for %s", name, recv.typ)
	}
	if f.lenient() && e.Owner != nil && e.Owner.universal && e.M.File != nil && e.M.File.isPrelude() && t.C.isStruct() && !t.C.universal && t.C.descendantDefines(name, false) {
		// Kernel's `then` on an AbstractEventFuture whose Event and Future define their own (concurrent-ruby's
		// `.future.then(*args, &task)`): the object's method, found at run time as MRI does (decision 253)
		anyRecv := expr{code: f.coerce(n, recv, TAny{}), typ: TAny{}}
		if block != nil {
			return f.universalBlockCall(n, anyRecv, name, args, block)
		}
		return f.genDynCall(n, anyRecv, name, args)
	}
	if x, ok := f.untypedCmpOp(n, t, recv, e, args, block); ok {
		return x
	}
	return f.numericMix(n, recv, e, args, block)
}

// selfEntryCall is a call on self (Self) of its class's method e, through a `<=>` that takes untyped for a Comparable
// op (MacOSVersion's `self >= "27"`, decision 296).
func (f *fctx) selfEntryCall(n parser.Node, e *entry, recv expr, args []parser.Node, block parser.Node) expr {
	if x, ok := f.untypedCmpOp(n, TClass{C: f.owner}, recv, e, args, block); ok {
		return x
	}
	return f.callEntry(n, e, recv, args, block)
}

// untypedCmpOp is Comparable's `<`, `<=`, `>` or `>=` on a class whose own `<=>` takes untyped (Homebrew's
// `Version < "2.39"`): the argument goes to that `<=>` as it is, as MRI's Comparable passes it, and a nil result is
// MRI's ArgumentError; Comparable's own `(self)` signature would convert it first (decision 287).
func (f *fctx) untypedCmpOp(n parser.Node, t TClass, recv expr, e *entry, args []parser.Node, block parser.Node) (expr, bool) {
	switch e.M.Name {
	case "<", "<=", ">", ">=":
	default:
		return expr{}, false
	}
	if e.M.Owner != f.c.classes["Comparable"] || len(args) != 1 || block != nil || isNumeric(recv.typ) {
		return expr{}, false
	}
	ce := t.C.lookup("<=>")
	if ce == nil || ce.M.Owner == e.M.Owner || len(ce.M.Params) != 1 || !isAny(ce.M.Params[0].Type) {
		return expr{}, false
	}
	r := expr{code: f.pinSimple(recv.code), typ: recv.typ}
	a := f.genExpr(args[0], nil)
	a.code = f.pinSimple(f.coerce(args[0], a, TAny{}))
	a.typ = TAny{}
	c := f.genMethodCall(n, r, "<=>", []parser.Node{&exprNode{Node: args[0], e: a}}, nil)
	code := fmt.Sprintf("rbCmpOp(%s, %q, %s, %s)", f.coerce(n, c, TAny{}), e.M.Name, f.coerce(n, r, TAny{}), a.code)
	return expr{code: code, typ: f.cls("Boolean")}, true
}

// abstractCall: an Object or module value is Go any, so it dispatches as untyped.
func (f *fctx) abstractCall(n parser.Node, t TClass, recv expr, name string, args []parser.Node, block parser.Node) expr {
	d := f.universalCall(n, recv, name, args, block)
	// the declared result keeps the caller typed instead of cascading dynamic calls
	if e := t.C.lookup(name); e != nil && isAny(d.typ) {
		f.c.inferRet(e.M)
		env := composeEnv(e.Env, nil)
		env["Self"] = t // a module's `self` is the value itself, of the module's type
		ret := subst(e.M.Ret, env)
		if name == "<=>" && f.c.isNumericMod(t.C) {
			ret = optOf(f.cls("Integer")) // nil for an incomparable number (a Complex), unlike Comparable's own
		}
		if !isVoid(ret) && !mentionsVar(ret) {
			d = expr{code: f.coerce(n, d, ret), typ: ret}
		}
	}
	return d
}

// snake is a class name as a method-name part: `DateTime` → `date_time`.
func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		if r == ':' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// untypedContainers are the core containers whose element types fall back to untyped when nothing types them (decision 192).
var untypedContainers = map[string]bool{"Hash": true, "Array": true, "Set": true, "Queue": true, "SizedQueue": true, "Thread::Queue": true, "Thread::SizedQueue": true}

// containerOf is the core container whose (class) method m is, or "".
func containerOf(m *Method) string {
	k := m.Owner
	if k != nil && k.metaOf != nil {
		k = k.metaOf
	}
	if k != nil && untypedContainers[k.RubyName] {
		return k.RubyName
	}
	return ""
}

// intrinsicGate decides a call before the intrinsics: true/false/nil's reopened marker classes answer (before a
// program's Object#name, which Boolean would take), and a program's own override is called as MRI calls it.
func (f *fctx) intrinsicGate(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool, bool) {
	if e, ok := f.markerCall(n, recv, name, args, block); ok {
		return e, true, true
	}
	if e := f.resolve(recv.typ, name); e != nil && !e.M.File.isPrelude() {
		return expr{}, false, true
	}
	return expr{}, false, false
}

// genCallValue renders a call whose value is used. For an attribute write
// (`recv.x = v`, `recv[k] = v`) Ruby's value is v, whatever the setter
// returns, so v is evaluated once and named after the call.
func (f *fctx) genCallValue(n *parser.CallNode, expected Type) expr {
	f.noteWindowChain(n)
	f.noteZipChain(n)
	if r := f.loopRescue(n); r != nil {
		return f.lift(r, expected, func(t tail) { f.genStmt(r, t) })
	}
	if expected != nil && !isAny(expected) { // `puts xs.sum { }`: an untyped slot says nothing about the call's type parameters (decision 348)
		saved, savedNode := f.retHint, f.retHintNode
		f.retHint, f.retHintNode = expected, n
		defer func() { f.retHint, f.retHintNode = saved, savedNode }()
	}
	args := callArgs(n)
	if !n.IsATTRIBUTE_WRITE() || len(args) == 0 || n.IsSAFE_NAVIGATION() {
		return f.genCall(n, expected)
	}
	var val expr
	c, a := *n, *n.Arguments
	a.Arguments = append(append([]parser.Node{}, args[:len(args)-1]...), &assignedArg{Node: args[len(args)-1], out: &val})
	c.Arguments = &a
	e := f.genCall(&c, expected)
	if val.typ == nil {
		return e // the setter never generated the value as an expression
	}
	f.emitExprStmt(n, e)
	if val.lit {
		val.code = f.c.goType(val.typ) + "(" + val.code + ")"
	}
	return expr{code: val.code, typ: val.typ, classObj: val.classObj, done: true}
}

// assignedArg is an attribute write's value argument: it records the
// generated value (in a temp unless it is side-effect free) for genCallValue.
type assignedArg struct {
	parser.Node
	out *expr
}

func (f *fctx) genAssignedArg(n *assignedArg, expected Type) expr {
	e := f.genExpr(n.Node, expected)
	if !e.lit && !isVoid(e.typ) && !isSimpleGo(e.code) {
		tmp := f.newTmp()
		f.emit("%s := %s", tmp, e.code)
		e.code = tmp
	}
	*n.out = e
	return e
}

// kernelCall calls a private Kernel prelude method as a receiverless call
// on main would: what user syntax like backticks and $? stands for.
func (f *fctx) kernelCall(n parser.Node, name string, args []parser.Node) expr {
	f.implicitCall = true
	defer func() { f.implicitCall = false }()
	return f.genMethodCall(n, expr{code: "rb_main", typ: f.cls("Object")}, name, args, nil)
}

// selfCall is a call on self typed Self: the owner's method, a top-level def, a Class method answered by the class
// object a module extends, method_missing, a delegator's object, a subclass's override, else Object's.
func (f *fctx) selfCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) expr {
	if e := f.owner.lookup(name); e != nil && !f.c.topOverKernel(e, name) {
		return f.selfEntryCall(n, e, recv, args, block)
	}
	if td := f.c.topDefs[name]; td != nil {
		return f.callEntry(n, &entry{M: td}, recv, args, block)
	}
	if f.owner.IsModule && (name == "<" || name == "<=" || name == ">" || name == ">=") && len(args) == 1 && block == nil &&
		!slices.ContainsFunc(f.owner.includers, func(k *Class) bool { return k.metaOf == nil }) {
		// a module only ever extended onto classes (regexp_parser's Shared::ClassMethods): self is a class object, and
		// `self < Sequence` is an ancestry test (decision 255), not a Module method every class object would answer
		a := f.genExpr(args[0], nil)
		code := fmt.Sprintf("rbModuleCmp(%s, %s, %t, %t)", f.coerce(nil, recv, TAny{}), f.coerce(args[0], a, TAny{}), name[0] == '>', len(name) == 2)
		return expr{code: code, typ: TOpt{Elem: f.cls("Boolean")}}
	}
	if f.owner.IsModule && f.lenient() && block == nil && f.c.classes["Class"].lookup(name) != nil {
		// a module extended onto classes calling a Class method on self (bindata's DSLMixin: `superclass.dsl_parser`):
		// the class object answers it, before the module's method_missing
		return f.genDynCall(n, recv, name, args)
	}
	if mm := f.owner.lookup("method_missing"); mm != nil {
		f.missingCalls++
		return f.callMissing(n, mm, recv, name, args, block)
	}
	if f.c.isDelegator(f.owner) && f.c.classes["Object"].lookup(name) == nil {
		return f.delegateCall(n, recv, name, args, block)
	}
	if f.owner.isStruct() && f.owner.descendantDefines(name, false) && block == nil {
		return f.genDynCall(n, recv, name, args)
	}
	if e := f.c.classes["Object"].lookup(name); e != nil {
		return f.callEntry(n, e, recv, args, block)
	}
	if !f.owner.IsModule && !untypedIntrinsics[name] && !descendantResponds(f.owner, name) {
		if name == "eval" {
			f.errorf(n, "undefined method eval for %s: %s", f.owner.displayName(), noEval)
		}
		f.errorf(n, "undefined method %s for %s", name, f.owner.displayName())
	}
	return f.universalCall(n, recv, name, args, block)
}

// iteratorValue is an iterator call in value position: a gem's `loop do ... break ... end` as a block's last value
// (drb's main_loop) runs, then is nil (decisions 233, 416); anything else is an error.
// iterCallValue is an iterator call with a literal block in value position, which only gem code may use.
func (f *fctx) iterCallValue(n *parser.CallNode, recv expr) (expr, bool) {
	b, ok := n.Block.(*parser.BlockNode)
	if !ok {
		return expr{}, false
	}
	e := f.resolve(recv.typ, n.Name)
	switch {
	case e == nil || !e.M.Iterator:
		return expr{}, false
	case !coreEach(e.M):
		return f.iteratorValue(n), true
	case f.lenient() && (blockBreaks(b.Body) || blockBreaksValue(b.Body)):
		return f.brokenIterValue(n, recv), true
	}
	return expr{}, false
}

// brokenIterValue is a gem's `return false if r.times do ... break if x == m ... end` (decision 417): the receiver, or
// nil when the block broke out, through genBrokenIterLoop; recv is already evaluated, so the loop reuses it.
func (f *fctx) brokenIterValue(n *parser.CallNode, recv expr) expr {
	tmp := f.newTmp()
	f.emit("var %s any", tmp)
	call := *n
	call.Receiver = &exprNode{Node: n.Receiver, e: recv}
	f.iterBreakValue = true
	defer func() { f.iterBreakValue = false }() // an errorf recovered by probe must not leave it set for the next call
	if !f.genIterCall(&call, tail{kind: tailAssign, target: tmp, typ: TAny{}}) {
		f.errorf(n, "%s with a block is an iterator and can only be used as a statement", n.Name)
	}
	return expr{code: tmp, typ: TAny{}}
}

func (f *fctx) iteratorValue(n *parser.CallNode) expr {
	if f.lenient() && n.Name == "loop" && kernelRecv(n) {
		f.genStmt(n, tail{})
		return expr{code: "nil", typ: TNil{}, lit: true}
	}
	f.errorf(n, "%s with a block is an iterator and can only be used as a statement", n.Name)
	return expr{}
}

// classObjCall is a held class object's `new` of a Hash subclass (`klass.new` on a singleton(HashSubclass), as rack's
// `@params_class.new`) or `allocate`.
func (f *fctx) classObjCall(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if meta := f.metaOfType(recv.typ); name == "new" && meta != nil && meta.metaOf.hashBase && len(meta.metaOf.Subclasses) == 0 && len(args) == 0 {
		f.emit("_ = %s", recv.code)
		return f.genNew(n, meta.metaOf, nil, nil, nil, block), true
	}
	if meta := f.metaOfType(recv.typ); name == "new" && meta != nil && !recv.classObj && len(meta.metaOf.Subclasses) == 0 && meta.metaOf.Super == f.c.classes["String"] {
		// `self.class.new(text)` in a leaf String subclass (rainbow's Presenter#wrap_with_sgr) is its own constructor, not String's (decision 444)
		f.emit("_ = %s", recv.code)
		return f.genNew(n, meta.metaOf, args, nil, nil, block), true
	}
	return f.genAllocate(n, recv, name, args, block)
}

// genAllocate is Class#allocate on a struct class: the object without initialize, as Marshal.load makes one (net-ftp's
// specs, decision 416); its ivars must read nil (or false, as falsy) unset: an Integer's would read 0.
func (f *fctx) genAllocate(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	meta := f.metaOfType(recv.typ)
	if name != "allocate" || meta == nil || len(args) != 0 || block != nil || !meta.metaOf.isStruct() ||
		len(meta.metaOf.Subclasses) != 0 || meta.metaOf.lookup("allocate") != nil {
		return expr{}, false
	}
	for _, iv := range f.c.ivarOrder(meta.metaOf) {
		if g := f.c.goType(iv.Type); g != "any" && !strings.HasPrefix(g, "*") && !strings.HasSuffix(g, "I") && g != "Boolean" && !strings.HasPrefix(g, "rbOpt[") {
			f.errorf(n, "%s.allocate: %s is %s, which would read as its zero value, not nil, before initialize", meta.metaOf.RubyName, iv.Name, iv.Type)
		}
	}
	f.emit("_ = %s", recv.code)
	return expr{code: "(&" + meta.metaOf.Name + "{})", typ: TClass{C: meta.metaOf}}, true
}
