package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// fctx is the per-function code generation context.
type fctx struct {
	iterValue        *tail                         // the value tail the next genIterLoop gives its loop frame, for `break v` (decision 410)
	iterBroke        *brokeTarget                  // the label the next genIterLoop's valueless break jumps to (decision 417)
	iterBreakValue   bool                          // the next genIterCall is a gem's iterator in value position whose block breaks (decision 417)
	splittingForward bool                          // splitBlockForward's with-a-block side: the plain union loop (decision 418)
	untypedRows      int                           // f.closures of a gem's `Array.new(n) { row }` block whose rows are untyped elements (decision 420)
	storageWrite     bool                          // discovery: the ivar write generating stores a value read out of a container (decision 392)
	destructuring    bool                          // discovery: a multiple assignment's targets take fields of a container (decision 392)
	aliasSrc         map[localKey]*parser.CallNode // a gem local holding an Array slot's String, which its bang edits write back (decision 331)
	noStrIndexSet    bool                          // generating an untyped local's `[]=` as other receivers take it (genStringBang's fallback)
	noFuse           *parser.CallNode              // the `h[k] = (h[k] || d) op v` whose miss arm is generating unfused (genFusedIndexAssign)
	superTarget      *Method                       // a gem super call's target while its arguments generate: they type none of its parameters
	nilRecv          Type                          // the receiver of the call whose arguments genArgs is about to generate (decision 393)
	nilArg           nilCall                       // the call whose argument is being coerced: a nil one raises MRI's error for it (decision 393)
	probeDepth       int                           // inside probe: output is discarded
	unionArm         int                           // inside a union argument's arm: a call it cannot make raises there, not dynamic (unionArgCall)
	erbDepth         int                           // inside a spliced template, where a bare name may be the caller's local
	curBlock         parser.Node                   // the block of the call overload is resolving
	lambdaClosure    int                           // f.closures inside the innermost lambda body, whose `return` is its own
	inDynContainer   bool                          // dynContainerCall is generating its default arm: the plain dispatcher
	retHint          Type                          // the expected type of retHintNode's result, for type params its arguments leave open
	retHintNode      parser.Node
	c                *Compiler
	f                *File
	owner            *Class
	m                *Method
	implicitCall     bool     // calling method_missing/respond_to_missing? on the program's behalf
	plainCalls       bool     // a struct's Dyn wrapper body: calls stay `self.M()`, the form dynShareable and freeCall read
	staticDef        *Method  // an UnboundMethod's dyn: this definition is called by its free func, not dispatched to an override (decision 141)
	lex              []*Class // lexical scope for constant lookup
	selfType         Type
	selfCode         string
	selfClassObj     bool // self is exactly a class constant (a class body)
	missingCalls     int  // receiverless calls compiled to self's method_missing, so far: a DSL block that needs them is rebound (blockArg)
	ret              Type
	iterator         bool
	lastMatch        string // the Go variable of the frame's last match, when the body reads $~ or $1..$9 (decision 171)
	inMatchSet       bool   // generating genMatchSet's own match call
	blockSig         *BlockSig
	buf              *strings.Builder
	indent           int
	tmp              int
	pass             int  // 0 = ivar discovery, 1 = local analysis, 2 = emit
	bodyUnit         bool // a class body's grouped statements: its constant assignments are statements (decision 170)
	discover         bool
	nilGuarded       map[parser.Node]bool // calls on a T? local that raise NoMethodError on nil: the local survives them non-nil (decision 357)
	nestedDone       map[parser.Node]bool // discovery: compound statements whose branch bodies discoverNested already dry-ran
	promoted         map[string]bool      // --int-overflow=promote: locals typed Bignum from their first write (decision 372)
	nilElemLocals    map[string]bool      // a gem method's locals that store nil into an element (`segment[i] = nil`, decision 411)
	dropFailing      bool                 // a recursive method's first inference pass: a statement that fails drops out of the return join (decision 355)
	locals           map[localKey]*localInfo
	refined          map[localKey]Type // `x = []`/`{}` typed by what is put in it (analyze)
	openArgs         map[int]localKey  // an open literal passed as a memo (`each_with_object({})`): its offset -> the block param it binds (decision 344)
	convs            map[int]bool      // pass 1's rbAs/OptOf conversion sites, by offset
	retTypes         *[]Type           // `return` values, while inferring the method's return type
	unset            map[localKey]bool // locals read where they may be unassigned (maybeUnset)
	scope            *scope
	block            string
	rbScope          string    // the block path where the current Ruby scope (method or block) starts
	rbFrames         []rbFrame // the Ruby blocks enclosing the current position, outermost first
	blockCtr         int
	loops            []*loopFrame
	switches         int  // nesting depth of emitted Go switch statements
	patQuiet         bool // matching a pattern: its dynamic calls are the match itself, so they do not warn (decision 143)
	labels           int  // not rewound by probe, so labels stay unique
	closures         int  // nesting depth of Go closures (non-iterator blocks)
	storing          int  // generating what a gem stores (an element added, an ivar assigned): only its conversions veto an ivar's typing (decision 307)
	nextTail         tail // the innermost closure's result, for `next`
	begins           int  // nesting depth of rescue wrappers
	retVar           string
	wrap             *wrapFrame  // the innermost begin wrapper
	retryFlag        string      // set by `retry` in the rescue clauses being generated
	declBin          bool        // declareAssign's value is a binary String: declareLocal leaves the local's bin state to it
	binNext          bool        // the next closure is a `@binary block` method's: its String params are binary (decision 352)
	binParams        bool        // binding that closure's params
	viewSplat        bool        // binding a windowSafe block's params: destructuring copies out of the window (decision 386)
	zipVal           bool        // the iterator loop is zip's [E, U?] with U = Integer/Float: pairs open to rbOpt (decision 396)
	bodyStmts        parser.Node // the statements of the method or block body being generated (noteBinStmt)
	noOwnSend        bool        // generating ownSendOrName's by-name arm: Kernel#send, not a class's own
	retryDepth       int         // f.closures where retryFlag was set: a block's retry can't reach it
	hasNamedRet      bool
	branchLenient    bool                // a gem method's retry: an if/else branch that does not compile raises if reached (decision 251)
	retTok           string              // a gem method whose block returns: the per-call token its rbReturn panics carry (decision 247)
	rescues          int                 // nesting depth of rescue clause bodies, where `raise` sets the new exception's cause (r_)
	yielderFed       *[]Type             // what a probed Enumerator.new block feeds its yielder (inferYielder)
	paramWiden       map[string]Type     // a gem method's params reassigned what their type does not hold: their widened type (decision 241)
	anyOrigin        map[localKey]string // RUGAROO_UNTYPED_WHY: where an untyped local's value came from (untypedWhy)
	blockWhy         string              // RUGAROO_UNTYPED_WHY: the root of the call whose block params are being bound (blockParam)
	breakCatch       *breakCatch         // the statement call whose closure block `break`s out of it (decision 241)
	inMarker         bool                // markerCall's own Boolean arm: past its check (decision 241)
	scopeRoot        parser.Node         // the method or program body genBody is generating: where an appended local's proof looks (decision 382)
	appendProofs     map[localKey]string // appendWhy's answers for scopeRoot
	ivarBufs         map[string]bool     // initialize's setbyte'd ivars, whose buffers hoistLocals declares (decision 395)
}

// genEnd is `END { }`: Kernel#at_exit with that block, registered the
// first time the statement runs only, as MRI's.
func (f *fctx) genEnd(n *parser.PostExecutionNode) {
	guard := fmt.Sprintf("rbEnd%d_%d", slices.Index(f.c.files, f.f), n.Location.StartOffset) // offsets repeat across files
	f.c.addPkgVar("var " + guard + " bool")
	call := f.c.rewrite(n, func() parser.Node {
		blk := &parser.BlockNode{Location: n.Location, Body: n.Statements}
		return &parser.CallNode{Location: n.Location, Name: "at_exit", Block: blk}
	})
	f.emit("if !%s {", guard)
	f.indent++
	f.emit("%s = true", guard)
	f.genStmt(call, tail{})
	f.indent--
	f.emit("}")
}

// localKey names one Ruby local: Ruby scopes are the method and each
// block, so the same name in two blocks (or in a block and the method) is
// two variables with their own type, hoisting and use counts.
type localKey struct {
	scope int // the block's source offset; methodScope for the method
	name  string
}

const methodScope = -1

// rbFrame is a Ruby block scope: the names prism puts in it (params,
// `|x; y|` block locals, locals first assigned inside) are its own.
type rbFrame struct {
	key    int
	locals []string
}

type localInfo struct {
	declBlock string
	declRuby  string // Ruby scope (method or block) the local belongs to
	declPass  int    // the generation pass that last saw its first assignment
	hoist     bool
	reads     int
	writes    int
	typ       Type
	annotated bool
	appended  bool      // a user String local appended to in place (decision 382)
	noHoist   bool      // params and block params: declared by Go syntax
	param     bool      // a method parameter
	open      bool      // an unannotated `[]`/`{}` that later writes may type (analyze)
	seeded    bool      // a non-empty `[...]` open to widening: elems start with its own element type (decision 193)
	filled    bool      // seeded by a gem's `Array.new(n, x)`: an untyped write widens it to untyped (decision 414)
	elems     [2][]Type // what those writes put in it: elements, or keys and values
	targs     [][]Type  // an open generic `X.new`: what later calls bind each type parameter to (decision 310)
	// bin/text: some write of the first passes held a binary String, or anything else; binDeps are the locals those
	// binary values were read from (decision 352)
	bin, text bool
	binDeps   []*localInfo
}

// noteBinStmt: `s.force_encoding("BINARY")` as a statement retags s in place in MRI, so the local is binary from here
// (its writes so far count as binary; decision 352). A pure-Ruby statement keeps the flag; the Go value is unchanged.
func (f *fctx) noteBinStmt(n parser.Node) {
	c, ok := n.(*parser.CallNode)
	if !ok || c.Name != "force_encoding" || f.pass >= 2 || c.Arguments == nil || len(c.Arguments.Arguments) != 1 || !binaryEncodingArg(f.f, c.Arguments.Arguments[0]) {
		return
	}
	lv, ok := c.Receiver.(*parser.LocalVariableReadNode)
	st, direct := f.bodyStmts.(*parser.StatementsNode)
	if !ok || !direct || !slices.Contains(st.Body, n) { // under an if or loop it may not run: the local stays text
		return
	}
	if v := f.scope.lookup(lv.Name); v != nil && isClass(stripOpt(v.typ), "String") {
		if info := v.owner(); info != nil && info.declRuby == f.rbScope {
			info.bin, info.text = true, false
		}
	}
}

// noteBin records what a write of the first passes put in the local: a binary String or not (decision 352).
func (li *localInfo) noteBin(pass int, val expr) {
	switch {
	case li == nil || pass >= 2:
	case val.bin:
		li.bin = true
		for _, d := range val.binDeps { // each probe run writes again
			if !slices.Contains(li.binDeps, d) {
				li.binDeps = append(li.binDeps, d)
			}
		}
	default:
		li.text = true
	}
}

// isBin reports whether every write of the local holds a binary String, through the locals they were read from.
func (li *localInfo) isBin(seen map[*localInfo]bool) bool {
	if li == nil || !li.bin || li.text {
		return false
	}
	if seen[li] {
		return true
	}
	if seen == nil {
		seen = map[*localInfo]bool{}
	}
	seen[li] = true
	for _, d := range li.binDeps {
		if !d.isBin(seen) {
			return false
		}
	}
	return true
}

type scope struct {
	parent *scope
	vars   map[string]*local
	killed []killedView // outer narrowed views a write in this scope dropped; leaveBranch may put them back (decision 357)
}

// killedView is a narrowed view unnarrow removed from an enclosing scope: sc held view, and holds repl since.
type killedView struct {
	sc         *scope
	name       string
	view, repl *local
}

type local struct {
	name     string
	goName   string
	typ      Type
	base     *local // non-nil for a narrowed view of another local
	view     string // see expr.view
	declared bool
	info     *localInfo // the Ruby local this binds; nil when not tracked
	assigned bool       // a view narrowAssign made: a Go var a later write of its class updates in place (decision 283)
}

// owner is the Ruby local v binds, looking through narrowed views.
func (v *local) owner() *localInfo {
	if v.base != nil {
		return v.base.owner()
	}
	return v.info
}

func (s *scope) lookup(name string) *local {
	for sc := s; sc != nil; sc = sc.parent {
		if v := sc.vars[name]; v != nil {
			return v
		}
	}
	return nil
}

func (f *fctx) push() { f.scope = &scope{parent: f.scope, vars: map[string]*local{}} }
func (f *fctx) pop() {
	inner := f.scope
	f.scope = inner.parent
	for _, k := range inner.killed { // still dropped for the enclosing scope, unless it is where the view lived
		if k.sc != f.scope && f.scope != nil {
			f.scope.killed = append(f.scope.killed, k)
		}
	}
}
func (f *fctx) errorf(n parser.Node, format string, args ...any) {
	f.c.errorf(f.f, n, format, args...)
}

// warn reports from the emitting pass only: earlier passes see provisional types.
func (f *fctx) warn(n parser.Node, format string, args ...any) {
	if f.pass == 2 {
		f.c.warn(f.f, n, format, args...)
	}
}

func (f *fctx) emit(format string, args ...any) {
	f.buf.WriteString(strings.Repeat("\t", f.indent))
	fmt.Fprintf(f.buf, format, args...)
	f.buf.WriteByte('\n')
}

// lineOf emits a //line directive (column 1) for node n.
func (f *fctx) lineOf(n parser.Node) {
	fmt.Fprintf(f.buf, "//line %s:%d\n", f.f.Name, f.f.shownLine(n.GetLocation().StartOffset))
}

func (f *fctx) newTmp() string {
	f.tmp++
	return fmt.Sprintf("t%d_", f.tmp)
}

// enterBlock starts a new Go block scope (for local hoisting analysis).
func (f *fctx) enterBlock() string {
	saved := f.block
	f.blockCtr++
	f.block = fmt.Sprintf("%s.%d", f.block, f.blockCtr)
	f.push()
	return saved
}

// enterRubyBlock starts a Go block that is also a Ruby block (a closure or
// an iterator's loop body): locals first assigned inside stay inside.
func (f *fctx) enterRubyBlock(block parser.Node, params []string) (string, string) {
	if b, ok := block.(*parser.BlockNode); ok && f.lenient() {
		renameReassignedParams(b, params)
	}
	saved := f.enterBlock()
	savedRuby := f.rbScope
	f.rbScope = f.block
	fr := rbFrame{key: block.GetLocation().StartOffset, locals: params}
	switch b := block.(type) {
	case *parser.BlockNode:
		fr.locals = append(fr.locals, b.Locals...)
	case *parser.LambdaNode:
		fr.locals = append(fr.locals, b.Locals...)
	}
	f.rbFrames = append(f.rbFrames, fr)
	return saved, savedRuby
}

func (f *fctx) leaveRubyBlock(saved, savedRuby string) {
	f.scope.killed = nil // a block may run any time later: what it dropped stays dropped
	f.rbFrames = f.rbFrames[:len(f.rbFrames)-1]
	f.rbScope = savedRuby
	f.leaveBlock(saved)
}

// localKey resolves name as Ruby does: to the innermost enclosing block
// that owns it, else to the method.
func (f *fctx) localKey(name string) localKey {
	for i := len(f.rbFrames) - 1; i >= 0; i-- {
		if slices.Contains(f.rbFrames[i].locals, name) {
			return localKey{f.rbFrames[i].key, name}
		}
	}
	return localKey{methodScope, name}
}

func (f *fctx) localInfo(name string) *localInfo { return f.locals[f.localKey(name)] }

// visibleLocal is the Go binding of the Ruby local name in scope here,
// skipping a same-named local of an outer Ruby scope that a block param or
// a `|x; y|` block local shadows.
func (f *fctx) visibleLocal(name string) *local {
	v := f.scope.lookup(name)
	if v != nil && v.owner() != nil && v.owner() != f.localInfo(name) {
		return nil
	}
	return v
}

// sameScopeLocal finds a local assigned earlier in the same Ruby scope but
// inside another Go block (an if branch, a begin body read from ensure):
// Ruby locals are method- or block-scoped, not branch-scoped. It is
// hoisted to a var at the top of its Ruby scope (hoistLocals).
func (f *fctx) sameScopeLocal(name string) *local {
	info := f.localInfo(name)
	if info == nil || info.noHoist || info.typ == nil || info.declPass != f.pass || !isAncestorBlock(info.declRuby, f.rbScope) {
		return nil
	}
	info.hoist = true
	return &local{name: name, goName: goLocalName(name), typ: info.typ, declared: true, info: info}
}

func (f *fctx) leaveBlock(saved string) {
	f.pop()
	f.block = saved
}

func isAncestorBlock(decl, use string) bool {
	return use == decl || strings.HasPrefix(use, decl+".")
}

// ---- tails

type tailKind int

const (
	tailNone tailKind = iota
	tailReturn
	tailAssign
)

type tail struct {
	kind   tailKind
	target string
	typ    Type    // nil: infer (probe)
	types  *[]Type // collector while inferring
	lits   *[]bool // with types: whether each value was an Array or Hash literal (untypedContainerJoin, decision 414)
	hint   Type    // while inferring: what the value is expected to be, for literals (`T.let(if c then [[], x] ... end, [A, B])`)
}

// want is the expected type of the value in tail position: its type, or while inferring the hint.
func (t tail) want() Type {
	if t.typ != nil {
		return t.typ
	}
	return t.hint
}

func (t tail) record(ty Type) { t.recordFrom(nil, ty) }

// recordFrom records the type of n's value.
func (t tail) recordFrom(n parser.Node, ty Type) {
	if t.types != nil {
		*t.types = append(*t.types, ty)
	}
	if t.lits != nil {
		*t.lits = append(*t.lits, isContainerLit(n))
	}
}

// applyTail finishes a value-producing statement in tail position.
func (f *fctx) applyTail(n parser.Node, e expr, t tail) {
	if e.noreturn {
		f.emit("%s", e.code)
		return
	}
	if _, void := e.typ.(TVoid); void && t.kind != tailNone && t.typ != nil && (isAny(t.typ) || isOpt(t.typ)) {
		// a void call where an untyped or optional value is wanted (`tap { work }`, `yield if c` last in a method answering
		// bool?) runs as a statement and yields nil (decision 364)
		f.emitExprStmt(n, e)
		e = expr{code: "nil", typ: TNil{}}
	}
	switch t.kind {
	case tailNone:
		f.emitExprStmt(n, e)
	case tailReturn:
		if t.typ == nil {
			t.recordFrom(n, e.typ)
			f.emit("_ = %s", e.code)
			return
		}
		e = f.untypedRow(n, e, t)
		if isVoid(t.typ) {
			f.emitExprStmt(n, e)
			return
		}
		if f.begins > 0 {
			f.emit("%s = %s", f.retVar, f.coerce(n, e, t.typ))
			return
		}
		f.emit("return %s", f.coerce(n, e, t.typ))
	case tailAssign:
		if t.typ == nil {
			t.recordFrom(n, e.typ)
			f.emit("_ = %s", e.code)
			return
		}
		f.emit("%s = %s", t.target, f.coerce(n, e, t.typ))
	}
}

func (f *fctx) emitExprStmt(n parser.Node, e expr) {
	f.noteBinStmt(n)
	if e.code == "" || e.code == "nil" || e.done || e.lit {
		return
	}
	switch n.(type) {
	case *parser.NilNode, *parser.SelfNode, *parser.InstanceVariableReadNode:
		return
	case *parser.LocalVariableReadNode: // a read alone (`{ |x| x }`) still counts as a use, or Go rejects the variable
		f.emit("_ = %s", e.code)
		return
	}
	if e.view != "" {
		e.code = e.view // the same call, without converting its result
	}
	switch n.(type) {
	case *parser.CallNode, *parser.YieldNode, *parser.SuperNode, *parser.ForwardingSuperNode,
		*parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode,
		*parser.LocalVariableOperatorWriteNode, *parser.InstanceVariableOperatorWriteNode,
		*parser.CallOperatorWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode, *parser.IndexOperatorWriteNode:
		if e.stmt || strings.HasSuffix(e.code, ")") && !e.assert && (e.typ == nil || !strings.HasPrefix(e.code, f.c.goType(e.typ)+"(")) { // a conversion is no statement
			f.emit("%s", e.code)
			return
		}
	}
	if e.stmt || isVoid(e.typ) && strings.HasSuffix(e.code, ")") { // a void call has no value to drop
		f.emit("%s", e.code)
		return
	}
	f.emit("_ = %s", e.code)
}

// ---- statements

func (f *fctx) genStmts(n parser.Node, t tail) {
	stmts, ok := n.(*parser.StatementsNode)
	// An empty body arrives as a typed-nil *StatementsNode, which is != nil.
	if n == nil || ok && stmts == nil {
		var at parser.Node
		if f.m != nil && f.m.Node != nil {
			at = f.m.Node // an empty def's nil, reported at the def
		}
		f.emptyTail(at, t)
		return
	}
	if !ok {
		f.genStmt(n, t)
		return
	}
	if len(stmts.Body) == 0 {
		f.emptyTail(n, t)
		return
	}
	for i, s := range stmts.Body {
		if f.m == nil && f.owner == nil && f.block == "" && f.lenient() && !f.c.round && !f.discover {
			// decision 205: a gem file's top-level statement that does not compile is skipped, warned
			err := f.tryClean(func() { f.genStmt(s, tail{}) })
			if err != nil {
				f.c.warn(f.f, s, "a top-level statement does not compile, so it is skipped: %s (decision 205)", err.Error())
			}
			continue
		}
		if f.genStmtPastFailure(s, i == len(stmts.Body)-1, t) {
			continue
		}
		switch {
		case i == len(stmts.Body)-1:
			f.genStmt(s, t)
		case f.absentReturn(s):
			f.genStmt(s, tail{}) // `return unless defined?(Gem)`: it always returns, so what follows (which names Gem) is dead
			return
		default:
			f.genStmt(s, tail{})
			if f.endsUnreachable() { // `require 'rouge'` when rouge is not compiled in: LoadError, so `AVAILABLE = true` after it never runs
				return
			}
			f.applyNarrow(f.guardNarrowing(s))
			f.applyNarrow(f.callNarrowing(s))
		}
	}
}

// genStmtPastFailure generates s where a dry run reads past what does not compile, reporting whether it did.
func (f *fctx) genStmtPastFailure(s parser.Node, last bool, t tail) bool {
	switch {
	case f.dropFailing:
		tt := tail{}
		if last {
			tt = t
		}
		n := len(*f.retTypes)
		if f.try(func() { f.genStmt(s, tt) }) != nil {
			*f.retTypes = (*f.retTypes)[:n] // a failed statement's half-recorded returns are not paths
			return true
		}
	case (f.c.round || f.discover) && !last:
		// an inference round reads calls, and ivar discovery assignments, past a statement it cannot compile (decisions 146, 191)
		err := f.try(func() { f.genStmt(s, tail{}) })
		f.c.debugInfer(err)
		if err != nil && f.discover {
			f.discoverNested(s)
		}
	case f.discover:
		err := f.try(func() { f.genStmt(s, t) })
		if err != nil {
			f.discoverNested(s)
			panic(*err)
		}
		return true
	default:
		return false
	}
	f.applyNarrow(f.guardNarrowing(s))
	f.applyNarrow(f.callNarrowing(s))
	return true
}

// discoverNested: a condition reading a not-yet-typed ivar (`if empty_p`) must not hide the writes in its branches (decision 356).
func (f *fctx) discoverNested(s parser.Node) {
	if f.nestedDone[s] { // an enclosing failure re-runs its descendants: without this, doubling per level
		return
	}
	if f.nestedDone == nil {
		f.nestedDone = map[parser.Node]bool{}
	}
	f.nestedDone[s] = true
	var bodies []parser.Node
	add := func(b *parser.StatementsNode) {
		if b != nil {
			bodies = append(bodies, b)
		}
	}
	addElse := func(e *parser.ElseNode) {
		if e != nil {
			add(e.Statements)
		}
	}
	switch n := s.(type) {
	case *parser.IfNode:
		add(n.Statements)
		switch e := n.Subsequent.(type) {
		case *parser.ElseNode:
			addElse(e)
		case *parser.IfNode:
			f.discoverNested(e)
		}
	case *parser.UnlessNode:
		add(n.Statements)
		addElse(n.ElseClause)
	case *parser.WhileNode:
		add(n.Statements)
	case *parser.UntilNode:
		add(n.Statements)
	case *parser.CaseNode:
		for _, w := range n.Conditions {
			if w, ok := w.(*parser.WhenNode); ok {
				add(w.Statements)
			}
		}
		addElse(n.ElseClause)
	case *parser.BeginNode:
		add(n.Statements)
		for r := n.RescueClause; r != nil; r = r.Subsequent {
			add(r.Statements)
		}
		addElse(n.ElseClause)
		if n.EnsureClause != nil {
			add(n.EnsureClause.Statements)
		}
	}
	for _, b := range bodies {
		saved := f.enterBlock()
		_ = f.try(func() { f.genStmts(b, tail{}) })
		f.leaveBlock(saved)
	}
}

// endsUnreachable reports whether the code just emitted ends in a call that never returns (expr.noreturn's marker), at
// this level: a statement after it is dead, and go vet rejects it.
func (f *fctx) endsUnreachable() bool {
	return strings.HasSuffix(f.buf.String(), "\npanic(\"rugaroo: unreachable\")\n")
}

// emptyTail handles an empty body in value position: its value is nil.
func (f *fctx) emptyTail(n parser.Node, t tail) {
	if t.kind == tailNone {
		return
	}
	if t.typ == nil {
		t.record(TNil{})
		return
	}
	if isVoid(t.typ) {
		return
	}
	f.applyTail(n, expr{code: "nil", typ: TNil{}}, t)
}

func (f *fctx) genStmt(n parser.Node, t tail) {
	if f.genStandIn(n, t) || f.noopStmt(n, t) || f.genLogicJump(n, t) {
		return
	}
	f.lineOf(n)
	if f.breakCatchStmt(n, t) {
		return
	}
	switch n := n.(type) {
	case *parser.IfNode:
		f.genIf(n, n.Predicate, n.Statements, n.Subsequent, false, t)
	case *parser.UnlessNode:
		var els parser.Node
		if n.ElseClause != nil {
			els = n.ElseClause
		}
		f.genIf(n, n.Predicate, n.Statements, els, true, t)
	case *parser.CaseNode:
		f.genCase(n, t)
	case *parser.CaseMatchNode:
		f.genCaseMatch(n, t)
	case *parser.BeginNode:
		f.genBegin(n, t)
	case *parser.WhileNode:
		f.genWhile(n.Predicate, n.Statements, false, n.IsBEGIN_MODIFIER(), t)
	case *parser.UntilNode:
		f.genWhile(n.Predicate, n.Statements, true, n.IsBEGIN_MODIFIER(), t)
	case *parser.ForNode:
		f.genFor(n, t)
	case *parser.ReturnNode:
		f.genReturn(n)
	case *parser.BreakNode:
		f.genBreak(n)
	case *parser.NextNode:
		f.genNext(n)
	case *parser.PostExecutionNode:
		f.genEnd(n)
	case *parser.RedoNode:
		var l *loopFrame
		if len(f.loops) > 0 {
			l = f.loops[len(f.loops)-1]
		}
		if l == nil || l.redo == "" || l.closures != f.closures || l.begins != f.begins {
			f.errorf(n, "redo is only supported directly in a loop or block body (not inside a begin/rescue)")
		}
		f.emit("goto %s", l.redo)
	case *parser.RetryNode:
		if f.retryFlag == "" || f.closures != f.retryDepth {
			f.errorf(n, "retry is only supported directly in a rescue clause")
		}
		f.emit("%s = true", f.retryFlag)
		f.emit("return")
	case *parser.ParenthesesNode:
		f.genStmts(n.Body, t)
	case *parser.CallNode:
		f.genCallNodeStmt(n, t)
	default:
		e := f.genExpr(n, t.want())
		f.applyTail(n, e, t)
	}
}

// genCallNodeStmt is a call in statement position: Sorbet's `T.bind(self, X)`, a loop with a rescue modifier, or a call.
func (f *fctx) genCallNodeStmt(n *parser.CallNode, t tail) {
	f.noteWindowChain(n)
	f.noteZipChain(n)
	if isTCall(n, "bind") && bindsSelf(n) {
		f.genSelfBind(n)
		f.emptyTail(n, t)
		return
	}
	if r := f.loopRescue(n); r != nil {
		f.genBegin(r, t)
		return
	}
	f.genCallStmt(n, t)
}

// genSelfBind is Sorbet's `T.bind(self, X)`: from here to the end of the block or method, self is X where X is a
// subclass of self's static type (Homebrew's `satisfy(build_env: false) do T.bind(self, MacOSRequirement); ... end`
// on a block bound to Requirement), asserted once; otherwise the closed world already knows self's class.
func (f *fctx) genSelfBind(n *parser.CallNode) {
	t := f.parseTypeAnn(n, sorbetType(callArgs(n)[1]))
	to, ok := t.(TClass)
	from := classOf(f.selfType)
	if !ok || from == nil || to.C == from || !to.C.isSubclassOf(from) || to.C.metaOf != nil {
		return
	}
	tmp := f.newTmp()
	f.emit("%s := %s", tmp, f.coerce(n, expr{code: "any(" + f.selfCode + ")", typ: TAny{}}, t))
	f.selfType, f.selfCode, f.owner = t, tmp, to.C // its ivars are self's (ArchRequirement's `@arch`)
}

// absentReturn reports whether s is an if/unless on an absent constant (absentGuard) whose taken branch ends in return.
func (f *fctx) absentReturn(s parser.Node) bool {
	var taken *parser.StatementsNode
	switch n := s.(type) {
	case *parser.UnlessNode:
		if !f.c.absentGuard(f.f, n.Predicate, f.lex) {
			return false
		}
		taken = n.Statements
	case *parser.IfNode:
		if !f.c.absentGuard(f.f, n.Predicate, f.lex) {
			return false
		}
		if e, ok := n.Subsequent.(*parser.ElseNode); ok {
			taken = e.Statements
		}
	}
	if taken == nil || len(taken.Body) == 0 {
		return false
	}
	switch last := taken.Body[len(taken.Body)-1].(type) {
	case *parser.ReturnNode:
		return true
	case *parser.CallNode: // liquid's `raise "Profiler not loaded" unless defined?(Liquid::Profiler)`: what follows is dead (decision 331)
		return last.Receiver == nil && (last.Name == "raise" || last.Name == "fail")
	}
	return false
}

func (f *fctx) genReturn(n *parser.ReturnNode) {
	if f.closures > 0 && f.closures == f.lambdaClosure {
		f.closureValue(n, n.Arguments) // a lambda's return leaves only the lambda
		return
	}
	if f.closures > 0 && f.retTok != "" { // unwinds to the method (decision 247)
		e := expr{code: "nil", typ: TNil{}}
		switch {
		case n.Arguments != nil && len(n.Arguments.Arguments) > 1: // `return row, col` (matrix's index) is [row, col] (decision 414)
			e = f.genExpr(&parser.ArrayNode{Location: n.Arguments.Location, Elements: n.Arguments.Arguments}, f.ret)
		case n.Arguments != nil:
			e = f.genExpr(n.Arguments.Arguments[0], f.ret)
		}
		if f.retTypes != nil {
			*f.retTypes = append(*f.retTypes, e.typ)
		}
		v := "nil"
		switch {
		case f.ret != nil && !isVoid(f.ret):
			v = f.newTmp() // typed, so an untyped constant (`return false`) is boxed as the method's Boolean, not Go's bool
			f.emit("var %s %s = %s", v, f.c.goType(f.ret), f.coerce(n, e, f.ret))
		case n.Arguments != nil:
			f.emitExprStmt(n, e)
		}
		f.emit("panic(&rbReturn{tok: %s, v: %s})", f.retTok, v)
		return
	}
	if f.closures > 0 && f.lenient() { // gem code (decision 172): fails only if it runs
		f.emit("panic(NewNotImplementedError(Ref(String(%q))))", fmt.Sprintf("rugaroo: non-local return from a block at %s:%d (decision 4)", f.f.Name, f.f.line(n.Location.StartOffset)))
		return
	}
	if f.closures > 0 {
		f.errorf(n, "non-local return from a block is not supported (docs/design.md decision 4)")
	}
	var e expr
	hasVal := false
	if n.Arguments != nil {
		val := n.Arguments.Arguments[0]
		if len(n.Arguments.Arguments) > 1 { // `return a, b` returns [a, b]
			val = &parser.ArrayNode{Location: n.Arguments.Location, Elements: n.Arguments.Arguments}
		}
		e = f.genExpr(val, f.ret)
		hasVal = true
	}
	if f.retTypes != nil && f.closures == 0 {
		*f.retTypes = append(*f.retTypes, e.typ)
	}
	if f.iterator || isVoid(f.ret) || f.ret == nil {
		if hasVal {
			f.emitExprStmt(n.Arguments.Arguments[0], e)
		}
		f.emitReturn()
		return
	}
	if !hasVal {
		e = expr{code: "nil", typ: TNil{}}
	}
	if f.begins > 0 {
		f.emit("%s = %s", f.retVar, f.coerce(n, e, f.ret))
		f.leaveWrapper(jumpReturn)
		return
	}
	f.emit("return %s", f.coerce(n, e, f.ret))
}

// rewrite returns n's desugared form, building it once so every pass (probe and emit) sees the same nodes.
func (c *Compiler) rewrite(n parser.Node, build func() parser.Node) parser.Node {
	if r := c.rewrites[n]; r != nil {
		return r
	}
	if c.rewrites == nil {
		c.rewrites = map[parser.Node]parser.Node{}
	}
	c.rewrites[n] = build()
	return c.rewrites[n]
}

// capture runs gen and returns the statements it emitted instead of
// emitting them, for code that must run conditionally.
func (f *fctx) capture(gen func()) string {
	saved := f.buf
	f.buf = &strings.Builder{}
	defer func() { f.buf = saved }()
	gen()
	return f.buf.String()
}

// symbolBlock desugars `&:name` to `{ |x_| x_.name }`.
func symbolBlock(ba *parser.BlockArgumentNode, name string) *parser.BlockNode {
	loc := ba.Location
	x := &parser.RequiredParameterNode{Location: loc, Name: "x_"}
	call := &parser.CallNode{Location: loc, Receiver: &parser.LocalVariableReadNode{Location: loc, Name: "x_"}, Name: name}
	return &parser.BlockNode{
		Location:   loc,
		Locals:     []string{"x_"},
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{x}}},
		Body:       &parser.StatementsNode{Location: loc, Body: []parser.Node{call}},
	}
}

// genCallStmt is a call in statement position: an iterator with a block
// (literal, `&:name` or `&proc`) becomes a loop.
func (f *fctx) genCallStmt(n *parser.CallNode, t tail) {
	if ba, ok := n.Block.(*parser.BlockArgumentNode); ok && !f.isBlockParam(ba.Expression) {
		c := *n
		if sym, ok := ba.Expression.(*parser.SymbolNode); ok {
			c.Block = symbolBlock(ba, sym.Unescaped.Value) // `workers.each(&:join)`
		} else if mb := f.methodRefBlock(ba, f.blockArity(n)); mb != nil {
			c.Block = mb // `xs.each(&method(:show))`
		} else if pb := f.procBlock(ba); pb != nil {
			c.Block = pb // `xs.each(&printer)`
		}
		if c.Block != n.Block && f.genIterCall(&c, t) {
			return
		}
	}
	if n.Block != nil {
		ba, forwards := n.Block.(*parser.BlockArgumentNode)
		if _, ok := n.Block.(*parser.BlockNode); ok || (forwards && f.isBlockParam(ba.Expression)) {
			if f.genIterCall(n, t) {
				return
			}
		}
	}
	var e expr
	if t.kind == tailNone || t.kind == tailReturn && t.typ != nil && isVoid(t.typ) {
		e = f.genCall(n, t.typ) // value unused: a setter needs no temp for it
	} else {
		e = f.genExpr(n, t.want())
	}
	f.applyTail(n, e, t)
}

// cannotRespond reports `x.respond_to?(:m)` that is false for x's static class: a @go_type or a class no subclass
// extends, without m or method_missing (decision 167: the branch it guards cannot run).
func (f *fctx) cannotRespond(pred parser.Node) bool {
	call, ok := pred.(*parser.CallNode)
	if !ok || call.Name != "respond_to?" || call.Receiver == nil || len(callArgs(call)) != 1 {
		return false
	}
	sym, ok := callArgs(call)[0].(*parser.SymbolNode)
	if !ok {
		return false
	}
	var recv expr
	if catchCompileError(func() { f.probe(func() { recv = f.genExpr(call.Receiver, nil) }) }) != nil {
		return false
	}
	rc, ok := recv.typ.(TClass)
	if !ok || rc.C.universal || rc.C.IsModule || rc.C.metaOf != nil || len(rc.C.Subclasses) > 0 && rc.C.GoType == "" {
		return false
	}
	return rc.C.lookup(sym.Unescaped.Value) == nil && rc.C.lookup("method_missing") == nil && rc.C.lookup("respond_to_missing?") == nil
}

// unwindsReturn: a gem method (decision 172) with a `return` inside a block that is a Go closure: the return panics
// rbReturn with this call's token, and the method recovers it as its result (decision 247). An iterator's blocks
// are loops, where return is plain Go.
func (c *Compiler) unwindsReturn(m *Method) bool {
	return c.pruneGems && m.Node != nil && !m.Iterator && c.gemFile(m.File) && blockReturns(m.Node.Body, false)
}

// blockReturns reports a `return` inside a block of n (inBlock: n is one), not in a lambda's or a nested def.
func blockReturns(n parser.Node, inBlock bool) bool {
	switch x := n.(type) {
	case nil:
		return false
	case *parser.ReturnNode:
		if inBlock {
			return true
		}
	case *parser.DefNode, *parser.LambdaNode, *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode:
		return false
	case *parser.CallNode:
		var args parser.Node
		if x.Arguments != nil { // a nil *ArgumentsNode is a non-nil Node
			args = x.Arguments
		}
		if x.Receiver == nil && x.Name == "lambda" { // `lambda do return end` returns from the lambda
			return blockReturns(args, inBlock)
		}
		if b, ok := x.Block.(*parser.BlockNode); ok {
			return blockReturns(x.Receiver, inBlock) || blockReturns(args, inBlock) || b.Body != nil && blockReturns(b.Body, true)
		}
	}
	for _, ch := range n.CompactChildNodes() {
		if blockReturns(ch, inBlock) {
			return true
		}
	}
	return false
}

// returnUnwind opens a method whose blocks return from it (decision 247): its call's token, and the recover that
// takes a block's rbReturn as the method's result.
func (f *fctx) returnUnwind(m *Method, namedRet bool) {
	f.retTok = "rbRetTok_"
	f.indent = 1
	f.emit("rbRetTok_ := new(byte)")
	switch gt := f.c.goType(m.Ret); {
	case !namedRet:
		f.emit("defer rbReturnRecover(rbRetTok_, nil)")
	case gt == "any":
		f.emit("defer rbReturnRecover(rbRetTok_, func(v any) { ret_ = v })")
	default:
		f.emit("defer rbReturnRecover(rbRetTok_, func(v any) {")
		f.emit("\tif v != nil {") // a nil-valued result stays ret_'s zero value
		f.emit("\t\tret_ = v.(%s)", gt)
		f.emit("\t}")
		f.emit("})")
	}
}

// noopStmt is a boot-recorded define_method statement whose value nothing reads (decision 198): it emits nothing, not
// even a //line a closing brace would follow (the whitespace linter's trailing newline).
func (f *fctx) noopStmt(n parser.Node, t tail) bool {
	call, ok := n.(*parser.CallNode)
	return ok && f.c.dmSites[call] && f.c.smSites[call] == nil && (t.kind == tailNone || isVoid(t.typ)) // an object's singleton method still records (decision 324)
}

// genLogicJump compiles a statement `x and return v` as `return v if x` (`or`: unless; kramdown's table parser,
// decision 334). A void x's value is lost (`a, b = c, d` as a method's last statement), so that stays unsupported.
func (f *fctx) genLogicJump(n parser.Node, t tail) bool {
	l, r, isOr, ok := jumpLogic(n)
	if !ok || isVoid(f.probeType(l, nil)) {
		return false
	}
	f.lineOf(n)
	f.genIf(n, l, &parser.StatementsNode{Location: r.GetLocation(), Body: []parser.Node{r}}, nil, isOr, t)
	return true
}

// jumpLogic splits a statement `l and j` / `l or j` whose right side j leaves the method or loop (return, break,
// next): its condition, its jump, and whether the jump is taken when l is falsy.
func jumpLogic(n parser.Node) (l, j parser.Node, isOr, ok bool) {
	switch n := n.(type) {
	case *parser.AndNode:
		l, j = n.Left, n.Right
	case *parser.OrNode:
		l, j, isOr = n.Left, n.Right, true
	default:
		return nil, nil, false, false
	}
	switch l.(type) {
	case *parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode: // `x = v or return`: x would also need narrowing after it
		return nil, nil, false, false
	}
	switch j.(type) {
	case *parser.ReturnNode, *parser.BreakNode, *parser.NextNode:
		return l, j, isOr, true
	}
	return nil, nil, false, false
}
