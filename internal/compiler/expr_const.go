package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// insideClass reports code in cls's own class or instance methods (or a subclass's), where a private class method may be called.
func (f *fctx) insideClass(cls *Class) bool {
	o := f.owner
	if o != nil && o.metaOf != nil {
		o = o.metaOf
	}
	return o != nil && o.isSubclassOf(cls)
}

// definedKind answers `defined?(v)` at compile time: the closed world knows every local, constant, method and global; "" is nil.
func (f *fctx) definedKind(v parser.Node) string {
	switch v := v.(type) {
	case *parser.ParenthesesNode:
		if st, ok := v.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.definedKind(st.Body[0])
		}
		return "expression"
	case *parser.LocalVariableReadNode:
		if e := f.genExpr(v, nil); e.code != "" {
			f.emit("_ = %s", e.code) // read only by defined?: Go still wants it used
		}
		if strings.HasPrefix(v.Name, "@") { // main's ivar, a local (decision 362)
			return "instance-variable"
		}
		return "local-variable"
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		var cls *Class
		var k *Const
		err := catchCompileError(func() { cls, k = f.c.lookupConst(f.f, v, f.lex) })
		if err != nil {
			return "" // a private constant's path: MRI answers nil
		}
		if cls != nil || k != nil {
			return "constant"
		}
		return ""
	case *parser.SelfNode:
		return "self"
	case *parser.NilNode:
		return "nil"
	case *parser.TrueNode:
		return "true"
	case *parser.FalseNode:
		return "false"
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		if f.m != nil && f.owner != nil && f.c.inheritedSig(f.m) != nil {
			return "super"
		}
		return ""
	case *parser.LocalVariableWriteNode, *parser.InstanceVariableWriteNode, *parser.ConstantWriteNode,
		*parser.LocalVariableOperatorWriteNode, *parser.LocalVariableOrWriteNode, *parser.LocalVariableAndWriteNode,
		*parser.InstanceVariableOperatorWriteNode, *parser.InstanceVariableOrWriteNode, *parser.MultiWriteNode:
		return "assignment"
	case *parser.ClassVariableReadNode:
		if f.lookupClassVar(v.Name) != nil {
			return "class variable"
		}
		return ""
	case *parser.GlobalVariableReadNode: // MRI's builtins always are; rugaroo rejects assigning any other (decision 109), so it never is
		if slices.Contains(mriGlobals, v.Name) {
			return "global-variable"
		}
		return ""
	case *parser.InstanceVariableReadNode: // a bare defined?(@x) is genSugar's run-time answer
		f.errorf(v, "defined?(%s) inside another expression is not supported; use defined?(@%s) first", f.f.text(v.GetLocation()), strings.TrimPrefix(v.Name, "@"))
	case *parser.YieldNode: // a bare defined?(yield) with an optional block is genSugar's run-time answer
		if f.m.optionalBlockLocal() != "" {
			f.errorf(v, "defined?(yield) inside another expression is not supported with an optional block; use block_given?")
		}
		if f.m != nil && f.m.Block != nil {
			return "yield"
		}
		return ""
	case *parser.CallNode:
		return f.definedCall(v)
	}
	return f.definedOther(v)
}

// definedOther is defined? of the forms MRI answers without running them (decision 367): an assignment, and an Array
// literal, nil when an element is not defined.
func (f *fctx) definedOther(v parser.Node) string {
	switch v := v.(type) {
	case *parser.GlobalVariableWriteNode, *parser.ClassVariableWriteNode, *parser.GlobalVariableOperatorWriteNode,
		*parser.GlobalVariableOrWriteNode, *parser.GlobalVariableAndWriteNode, *parser.ClassVariableOperatorWriteNode,
		*parser.ClassVariableOrWriteNode, *parser.ClassVariableAndWriteNode, *parser.InstanceVariableAndWriteNode,
		*parser.ConstantOperatorWriteNode, *parser.ConstantOrWriteNode, *parser.IndexOperatorWriteNode,
		*parser.CallOperatorWriteNode, *parser.CallOrWriteNode, *parser.IndexOrWriteNode:
		return "assignment"
	case *parser.ArrayNode:
		for _, e := range v.Elements {
			if f.definedKind(e) == "" {
				return ""
			}
		}
	}
	return "expression"
}

// mriGlobals is MRI 4.0's startup `global_variables` minus $& $` $' $+ (Prism's back references).
var mriGlobals = strings.Fields(`$! $" $$ $* $, $-0 $-F $-I $-W $-a $-d $-i $-l $-p $-v $-w $. $/ $0 $: $; $< $= $> $? $@
	$DEBUG $FILENAME $LOADED_FEATURES $LOAD_PATH $PROGRAM_NAME $VERBOSE $\ $_ $stderr $stdin $stdout $~`)

// definedCall is `defined?(recv.name)`: "method" when the receiver's static type has a public name (any visibility without a receiver).
func (f *fctx) definedCall(v *parser.CallNode) string {
	if v.Receiver == nil {
		switch {
		case f.c.topDefs[v.Name] != nil, f.resolve(f.selfType, v.Name) != nil:
			return "method"
		case slices.Contains([]string{"raise", "fail", "require", "require_relative", "lambda", "proc"}, v.Name):
			return "method"
		}
		return ""
	}
	if f.definedKind(v.Receiver) == "" {
		return ""
	}
	if cls := f.classRef(v.Receiver); cls != nil {
		if v.Name == "new" && !cls.IsModule || cls.meta != nil && f.resolve(TClass{C: cls.meta}, v.Name) != nil {
			return "method"
		}
		return ""
	}
	rt := f.probeType(v.Receiver, nil)
	if isAny(rt) {
		f.errorf(v, "defined?(...%s) on an untyped receiver is not supported; use respond_to?", v.Name)
	}
	e := f.resolve(stripOpt(rt), v.Name)
	if e == nil || e.M.Private {
		return ""
	}
	return "method"
}

// classRef resolves n to a class or module when it is a constant naming one.
func (f *fctx) classRef(n parser.Node) *Class {
	switch n.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		cls, k := f.c.lookupConst(f.f, n, f.lex)
		for i := 0; cls == nil && k != nil && i < 8; i++ { // `Alias = SomeClass` (rack's ParamsTooDeepError) names the class too
			switch k.Value.(type) {
			case *parser.ConstantReadNode, *parser.ConstantPathNode:
				cls, k = f.c.lookupConst(k.File, k.Value, k.Scope)
			default:
				return nil
			}
		}
		return cls
	}
	return nil
}

// genConstRead reads a constant: a Go package variable.
func (f *fctx) genConstRead(n parser.Node) expr {
	cls, k := f.c.lookupConst(f.f, n, f.lex)
	if err, bad := f.c.badClasses[cls]; bad && cls != nil {
		panic(err)
	}
	switch {
	case k != nil && k.guarded:
		code := fmt.Sprintf("rbConstRead(%s, %s, %q)", constSet(k), k.GoName, f.constMissing(n))
		return expr{code: code, typ: f.c.constType(k)}
	case k != nil:
		return expr{code: k.GoName, typ: f.c.constType(k)}
	case cls != nil && cls.meta != nil:
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}, classObj: true}
	case cls != nil:
		f.errorf(n, "class %s used as a value is not supported", cls.RubyName)
	}
	if r, ok := n.(*parser.ConstantReadNode); ok && r.Name == "DATA" && f.f.data != nil {
		return f.genData()
	}
	if p, ok := n.(*parser.ConstantPathNode); ok && p.Parent != nil && f.lenient() {
		switch p.Parent.(type) {
		case *parser.ConstantReadNode, *parser.ConstantPathNode:
		default: // `self.class::PARAGRAPH_END` (kramdown): the run-time class's constant, as const_get finds it (decision 330)
			sym := &parser.SymbolNode{Unescaped: parser.RubyString{Value: *p.Name}, Location: p.Location}
			return f.genExpr(&parser.CallNode{Location: p.Location, Receiver: p.Parent, Name: "const_get",
				Arguments: &parser.ArgumentsNode{Location: p.Location, Arguments: []parser.Node{sym}}}, nil)
		}
	}
	if f.lenient() { // gem code: MRI raises NameError where the read runs, not when the method loads (decision 218)
		return expr{code: fmt.Sprintf("rbConstRead[any](false, nil, %q)", f.constMissing(n)), typ: TAny{}}
	}
	if f.m == nil && f.closures == 0 { // load-time code always runs: MRI's NameError at load, reported now (decision 363)
		f.errorf(n, "uninitialized constant %s", f.f.text(n.GetLocation()))
	}
	if f.pass == 2 { // MRI raises NameError where the read runs: code that never runs it still compiles (decision 363)
		f.c.warn(f.f, n, "uninitialized constant %s: raises NameError if this line runs", f.f.text(n.GetLocation()))
	}
	return expr{code: fmt.Sprintf("rbConstRead[any](false, nil, %q)", f.constMissing(n)), typ: TAny{}} // a value anywhere: inside a literal too
}

// genData is the main file's DATA: one StringIO over the text after `__END__` (MRI's is a File at that offset; reading it reads the same).
func (f *fctx) genData() expr {
	t := f.cls("StringIO")
	if !f.c.dataVar {
		f.c.dataVar = true
		text := &exprNode{e: expr{code: strconv.Quote(*f.f.data), typ: f.cls("String"), lit: true}}
		io := f.genMethodCall(&parser.NilNode{}, f.genConstRead(&parser.ConstantReadNode{Name: "StringIO"}), "new", []parser.Node{text}, nil)
		f.c.addPkgVar("var rbDATA = " + io.code)
	}
	return expr{code: "rbDATA", typ: t}
}

// constMissing is MRI's name for constant read n in a NameError: a bare
// name is qualified by the innermost class or module around it.
func (f *fctx) constMissing(n parser.Node) string {
	r, ok := n.(*parser.ConstantReadNode)
	if !ok {
		return strings.TrimPrefix(f.f.text(n.GetLocation()), "::")
	}
	if len(f.lex) > 0 {
		return f.lex[len(f.lex)-1].RubyName + "::" + r.Name
	}
	return r.Name
}

// metaOfType returns the metaclass a receiver type denotes, if any.
func (f *fctx) metaOfType(t Type) *Class {
	switch t := t.(type) {
	case TClass:
		if t.C.metaOf != nil {
			return t.C
		}
	case TVar:
		if t.Name == "Self" && f.owner != nil && f.owner.metaOf != nil {
			return f.owner
		}
	case TAny, TFunc, TNil, TOpt, TTuple, TUnion, TVoid: // not a class object
	}
	return nil
}

// genClassOf renders `x.class` for an instance of a class with a metaclass.
// A receiver whose class is known only at run time (untyped, Object, a
// module, nil, T?) asks rbClassOf, typed as a plain Class.
func (f *fctx) genClassOf(n parser.Node, recv expr) (expr, bool) {
	var cls *Class
	switch t := recv.typ.(type) {
	case TClass:
		cls = t.C
		if cls.Name == "Bignum" { // MRI's class of every integer (decision 372)
			cls = f.c.classes["Integer"]
		}
	case TVar:
		if t.Name == "Self" {
			cls = f.owner
		}
		if cls != nil && cls.IsModule && !cls.universal { // self is some includer
			return expr{code: recv.code + "._ClassObj()", typ: TClass{C: f.c.classes["Class"]}}, true
		}
	case TAny, TNil, TOpt, TUnion:
		return f.dynClassOf(n, recv), true
	case TFunc:
		p := f.c.classes["Proc"]
		f.discard(recv)
		return expr{code: classVar(p), typ: TClass{C: p.meta}}, true
	case TTuple: // a tuple is an Array at run time (decision 22)
		a := f.c.classes["Array"]
		f.discard(recv)
		return expr{code: classVar(a), typ: TClass{C: a.meta}}, true
	case TVoid: // a void call's value is nil
		return f.dynClassOf(n, f.voidAsNil(recv)), true
	}
	if cls != nil && (cls.universal || cls.IsModule) { // Go any: asked at run time
		return f.dynClassOf(n, recv), true
	}
	if cls != nil && cls.metaOf != nil {
		// the class of a class object is Class; of a module, Module
		k := f.c.classes["Class"]
		if cls.metaOf.IsModule {
			k = f.c.classes["Module"]
		}
		f.discard(recv)
		return expr{code: classVar(k), typ: TClass{C: k.meta}, classObj: true}, true
	}
	if cls == nil || cls.meta == nil {
		return expr{}, false
	}
	if cls.RubyName == "Boolean" {
		return expr{code: "rbBoolClass(" + recv.code + ")", typ: f.cls("Class")}, true
	}
	if !cls.isStruct() {
		f.discard(recv) // `d.pkg.class` still calls pkg
		return expr{code: classVar(cls), typ: TClass{C: cls.meta}}, true
	}
	out := expr{code: recv.code + "._ClassOf()", typ: TClass{C: cls.meta}}
	if cls != cls.root() {
		out.code, out.assert = out.code+".("+f.c.goType(out.typ)+")", true
	}
	return out, true
}

// constGetType narrows `M.const_get(name)` when M is statically known: a
// literal name gets that constant's type; any other name the join of the
// constants of M and its subclasses (whose tables M's may stand for).
func (f *fctx) constGetType(recv expr, args []parser.Node) Type {
	meta := f.metaOfType(recv.typ)
	if meta == nil || len(args) == 0 {
		return nil
	}
	mod := meta.metaOf
	if lit := literalName(args[0]); lit != "" {
		scope := []*Class{mod}
		if mod.RubyName == "Object" {
			scope = nil
		}
		// Through a non-module MRI raises TypeError at run time; lookupConst would fail compilation.
		parts := strings.Split(lit, "::")
		for i := 1; i < len(parts); i++ {
			if cls, k := f.c.lookupConst(f.f, constPath(strings.Join(parts[:i], "::")), scope); cls == nil && k != nil {
				return nil
			}
		}
		return f.c.constTypeOf(f.c.lookupConst(f.f, constPath(lit), scope))
	}
	var ts []Type
	var walk func(c *Class)
	walk = func(c *Class) {
		for _, full := range f.c.constEntries(c) {
			cls, k := f.c.classes[full], f.c.consts[full]
			if t := f.c.constTypeOf(cls, k); t != nil {
				ts = append(ts, t)
			}
		}
		for _, sub := range c.Subclasses {
			if sub.metaOf == nil {
				walk(sub)
			}
		}
	}
	walk(mod)
	if len(ts) == 0 {
		return nil
	}
	if t := joinOrAny(ts); !isAny(t) {
		return t
	}
	return nil
}

func (c *Compiler) constTypeOf(cls *Class, k *Const) Type {
	switch {
	case cls != nil && cls.meta != nil:
		return TClass{C: cls.meta}
	case k != nil:
		return c.constType(k)
	}
	return nil
}

// englishGlobals are English.rb's aliases of the globals genGlobalRead knows (Homebrew's `$CHILD_STATUS`; library/English,
// decision 414; `$NR` for $., decision 425). $_ ($LAST_READ_LINE) is not: rugaroo's gets keeps no last line.
var englishGlobals = map[string]string{
	"$CHILD_STATUS": "$?", "$PROCESS_ID": "$$", "$PID": "$$", "$LAST_MATCH_INFO": "$~",
	"$INPUT_RECORD_SEPARATOR": "$/", "$RS": "$/", "$ERROR_INFO": "$!", "$ERROR_POSITION": "$@",
	"$DEFAULT_OUTPUT": "$>", "$DEFAULT_INPUT": "$<", "$ARGV": "$*",
	"$FS": "$;", "$FIELD_SEPARATOR": "$;", "$OFS": "$,", "$OUTPUT_FIELD_SEPARATOR": "$,",
	"$ORS": "$\\", "$OUTPUT_RECORD_SEPARATOR": "$\\", "$NR": "$.", "$INPUT_LINE_NUMBER": "$.",
}

// separatorVars are the Go variables of $;, $, and $\\, which String#split, Array#join and Kernel#print default to.
var separatorVars = map[string]string{"$;": "rbFS", "$,": "rbOFS", "$\\": "rbORS"}

// englishMatchRefs are English.rb's names for the last match's back-references.
var englishMatchRefs = map[string]string{"$MATCH": "$&", "$PREMATCH": "$`", "$POSTMATCH": "$'", "$LAST_PAREN_MATCH": "$+"}

// genGlobalRead maps the read-only globals rugaroo knows (decision 61); $0 is the Ruby file as named at compile time, as `ruby main.rb` sets it.
func (f *fctx) genGlobalRead(n *parser.GlobalVariableReadNode) expr {
	if short, ok := englishGlobals[n.Name]; ok { // `require "English"`'s names for the punctuation globals
		alias := *n
		alias.Name = short
		return f.genGlobalRead(&alias)
	}
	if ref := englishMatchRefs[n.Name]; ref != "" {
		return f.genMatchRef(&parser.BackReferenceReadNode{Name: ref, Location: n.Location})
	}
	switch n.Name {
	case "$@": // the handled exception's backtrace (decision 414), nil outside a rescue as $! is
		if f.rescues > 0 {
			return f.genExpr(&parser.CallNode{Receiver: &parser.GlobalVariableReadNode{Name: "$!", Location: n.Location}, Name: "backtrace", Location: n.Location}, nil)
		}
		return expr{code: "nil", typ: TOpt{Elem: TClass{C: f.c.classes["Array"], Args: []Type{f.cls("String")}}}}
	case "$>":
		return f.genGlobalRead(&parser.GlobalVariableReadNode{Name: "$stdout", Location: n.Location})
	case "$<":
		return f.genConstRead(&parser.ConstantReadNode{Name: "ARGF", Location: n.Location})
	case "$*":
		return f.genConstRead(&parser.ConstantReadNode{Name: "ARGV", Location: n.Location})
	case "$;", "$,", "$\\": // the separators split, join and print default to (decision 414)
		return expr{code: separatorVars[n.Name], typ: TOpt{Elem: f.cls("String")}}
	}
	switch n.Name {
	case "$!": // the exception every rescue form handles, never nil there (decision 244); ponytail: elsewhere nil, where MRI's is the frame's last
		if f.rescues > 0 {
			return expr{code: "rbAs[ExceptionI](r_, \"Exception\")", typ: f.cls("Exception")}
		}
		return expr{code: "nil", typ: TOpt{Elem: f.cls("Exception")}}
	case "$0", "$PROGRAM_NAME":
		name := f.c.programFile.Name
		f.c.strLits[name] = true
		return expr{code: strconv.Quote(name), typ: f.cls("String"), lit: true}
	case "$stdin":
		return f.genConstRead(&parser.ConstantReadNode{Name: "STDIN", Location: n.Location})
	case "$stdout", "$stderr": // the IO bound to the assigned object, if any (decision 109)
		return expr{code: "rb" + strings.ToUpper(n.Name[1:2]) + n.Name[2:] + "IO()", typ: f.cls("IO")}
	case "$?": // read directly: a Kernel call on main would go dynamic inside another class's method
		return expr{code: "rbLastStatusOpt()", typ: TOpt{Elem: TClass{C: f.c.classes["Process::Status"]}}}
	case "$$": // Process.pid
		return expr{code: "rbPid()", typ: f.cls("Integer")}
	case "$~": // the frame's last match (decision 171)
		return expr{code: f.lastMatchVar(n), typ: TOpt{Elem: f.cls("MatchData")}}
	case "$LOAD_PATH", "$:", "$-I": // fixed when boot ran (decision 301): the compiled program has no load path of its own
		if f.c.bootLoadPath != nil {
			elems := make([]string, len(f.c.bootLoadPath))
			for i, d := range f.c.bootLoadPath {
				f.c.strLits[d] = true
				elems[i] = strconv.Quote(d)
			}
			return expr{code: arrayLit("String", elems), typ: TClass{C: f.c.classes["Array"], Args: []Type{f.cls("String")}}}
		}
	case "$LOADED_FEATURES", `$"`:
		// the files compiled into the program, the same list wherever it is read: every require ran at compile time, so
		// none is added later (rubocop's `$LOADED_FEATURES - before_us` is empty, its cache checksum reads the list;
		// decision 444). Gem code only: user code still gets the error that names what rugaroo knows.
		if f.lenient() {
			f.c.loadedFeatures = true
			return expr{code: "rbLoadedFeatures()", typ: TClass{C: f.c.classes["Array"], Args: []Type{f.cls("String")}}}
		}
	case "$VERBOSE":
		return expr{code: "rbVerbose", typ: TOpt{Elem: f.cls("Boolean")}}
	case "$/": // the input record separator: chomp's default (decision 180)
		return expr{code: "rbRS", typ: TOpt{Elem: f.cls("String")}}
	case "$.": // the last line number an IO's gets read, 0 before any (decision 425)
		f.c.usesLineNo = true
		return expr{code: "Integer(rbLineNo.Load())", typ: f.cls("Integer")}
	}
	if k := f.c.globals[n.Name]; k != nil { // the program's own (decision 361)
		return expr{code: k.GoName, typ: f.c.constType(k)}
	}
	f.errorf(n, "global variable %s is never assigned; rugaroo knows $0, $PROGRAM_NAME, $$, $!, $stdin, $stdout, $stderr, $VERBOSE, $/, $? and the program's own assigned globals (decisions 61, 361)", n.Name)
	return expr{}
}

// genGlobalWrite is `$stdout = io` / `$stderr = io` (decision 109): the
// object takes the stream's writes. Reading the global still answers the
// IO constant (decision 61), so the assignment's value is what was given.
func (f *fctx) genGlobalWrite(n *parser.GlobalVariableWriteNode) expr {
	if short, ok := englishGlobals[n.Name]; ok {
		alias := *n
		alias.Name = short
		return f.genGlobalWrite(&alias)
	}
	if v, t := map[string]string{"$VERBOSE": "rbVerbose", "$/": "rbRS", "$;": "rbFS", "$,": "rbOFS", "$\\": "rbORS"}[n.Name], map[string]string{"$VERBOSE": "Boolean", "$/": "String", "$;": "String", "$,": "String", "$\\": "String"}[n.Name]; v != "" {
		// a statement, then the global's value: the assignment may be a block's or method's value
		want := TOpt{Elem: f.cls(t)}
		e := f.genExpr(n.Value, want)
		f.emit("%s = %s", v, f.coerce(n.Value, e, want))
		return expr{code: v, typ: want}
	}
	if n.Name == "$." { // MRI's lineno setter: an Integer, a Float truncated, else to_int (decision 425)
		f.c.usesLineNo = true
		it := f.cls("Integer")
		e := f.genExpr(n.Value, it)
		var code string
		if typeEq(e.typ, f.cls("Float")) { // through a variable: a literal is a Go constant, which Integer(123.5) rejects
			fl := f.newTmp()
			f.emit("var %s Float = %s", fl, e.code)
			code = "Integer(" + fl + ")"
		} else {
			code = f.coerce(n.Value, e, it)
		}
		tmp := f.newTmp()
		f.emit("var %s Integer = %s", tmp, code) // typed: a literal alone would be Go's int
		f.emit("rbLineNo.Store(int64(%s))", tmp)
		return expr{code: tmp, typ: it}
	}
	if n.Name == "$~" || n.Name == "$LAST_MATCH_INFO" { // the frame's last match, set as a MatchData or nil (decision 364)
		want := TOpt{Elem: f.cls("MatchData")}
		v := f.lastMatchVar(n)
		f.emit("%s = %s", v, f.coerce(n.Value, f.genExpr(n.Value, want), want))
		return expr{code: v, typ: want, done: true}
	}
	if k := f.c.globals[n.Name]; k != nil {
		t := f.c.constType(k)
		val := f.genExpr(n.Value, t)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		return expr{code: k.GoName, typ: t, done: true}
	}
	fn := map[string]string{"$stdout": "rbSetStdout", "$stderr": "rbSetStderr"}[n.Name]
	if fn == "" {
		f.errorf(n, "global variable %s cannot be assigned; only $stdout and $stderr can (docs/design.md decision 109)", n.Name)
	}
	v := f.genExpr(n.Value, nil)
	return expr{code: fn + "(" + f.coerce(n.Value, v, TAny{}) + ")", typ: TAny{}, stmt: true}
}

// genGlobalUpdate is `$x op= v`, `$x ||= v` and `$x &&= v` on the program's own global (decision 361).
func (f *fctx) genGlobalUpdate(n parser.Node) expr {
	var name string
	switch n := n.(type) {
	case *parser.GlobalVariableOperatorWriteNode:
		name = n.Name
	case *parser.GlobalVariableOrWriteNode:
		name = n.Name
	case *parser.GlobalVariableAndWriteNode:
		name = n.Name
	}
	k := f.c.globals[name]
	if k == nil {
		f.errorf(n, "global variable %s cannot be updated; only the program's own assigned globals can (decision 361)", name)
	}
	t := f.c.constType(k)
	cur := expr{code: k.GoName, typ: t}
	switch n := n.(type) {
	case *parser.GlobalVariableOperatorWriteNode:
		val := f.genOp(n, cur, n.BinaryOperator, n.Value)
		f.emit("%s = %s", k.GoName, f.coerce(n, val, t))
		return expr{code: k.GoName, typ: t, done: true}
	case *parser.GlobalVariableOrWriteNode:
		return f.genOrAssign(n, cur, n.Value)
	case *parser.GlobalVariableAndWriteNode: // `$x &&= v` is `$x && ($x = v)`
		w := &parser.GlobalVariableWriteNode{Name: n.Name, NameLoc: n.NameLoc, Value: n.Value, Location: n.Location}
		return f.genExpr(&parser.AndNode{Left: &parser.GlobalVariableReadNode{Name: n.Name, Location: n.NameLoc}, Right: w, Location: n.Location}, nil)
	}
	return expr{}
}
