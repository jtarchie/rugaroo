package compiler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

func isIsA(n *parser.CallNode) bool {
	return (n.Name == "is_a?" || n.Name == "kind_of?") && n.Arguments != nil && len(n.Arguments.Arguments) == 1
}

// isACheck renders `recv.is_a?(C)` as a Go bool: a constant when the static
// type decides it, otherwise a type assertion.
func (f *fctx) isACheck(n parser.Node, recv expr, classNode parser.Node) string {
	cls := f.classRef(classNode)
	if cls == nil {
		// a class value: its class object answers at run time (decision 76)
		k := f.genExpr(classNode, nil)
		switch t := k.typ.(type) {
		case TAny:
		case TClass:
			if !t.C.isSubclassOf(f.c.classes["Module"]) {
				f.errorf(classNode, "is_a? needs a class or module, not %s", k.typ)
			}
		default:
			f.errorf(classNode, "is_a? needs a class or module, not %s", k.typ)
		}
		return "rbIsInstanceOf(" + f.coerce(classNode, k, TAny{}) + ", " + f.coerce(n, recv, TAny{}) + ")"
	}
	c := f.isA(n, recv, cls)
	if c == "true" || c == "false" {
		f.discard(recv)
	}
	return c
}

// unionIsA is is_a?(cls) on a union (decision 150), decided per member:
// true when every member is a cls, false when none can be, else a test of
// the members that are (or may be, a subclass of a member) a cls.
func (f *fctx) unionIsA(n parser.Node, recv expr, u TUnion, cls *Class) string {
	var tests []string
	all := true
	for _, m := range u.Members {
		mc := classOf(m)
		if _, ok := m.(TTuple); ok {
			mc = f.c.classes["Array"]
		}
		switch {
		case mc != nil && mc.isSubclassOf(cls):
			tests = append(tests, f.c.goType(m))
			continue
		case mc != nil && !cls.IsModule && cls.isSubclassOf(mc):
			tests = append(tests, f.c.isAGoType(cls))
		case mc != nil && cls.IsModule && mc.includedBelow(cls):
			f.errorf(n, "is_a?(%s) on %s cannot be checked: rugaroo has no runtime record of included modules", cls.RubyName, m)
		}
		all = false
	}
	switch {
	case all:
		return "true"
	case len(tests) == 0:
		return "false"
	}
	code := f.pinSimple(recv.code)
	parts := make([]string, 0, len(tests))
	for _, g := range slices.Compact(tests) {
		parts = append(parts, "rbIsA["+g+"]("+code+")")
	}
	return "(" + strings.Join(parts, " || ") + ")"
}

func (f *fctx) isA(n parser.Node, recv expr, cls *Class) string {
	if cls.universal { // every value, nil included, is an Object
		return "true"
	}
	if c, ok := f.markerIsA(recv, cls); ok {
		return c
	}
	if isClass(recv.typ, "Bignum") { // an Integer to Ruby (decision 372)
		recv.typ = f.cls("Integer")
	}
	t := recv.typ
	if o, ok := t.(TOpt); ok {
		recv.code = f.pinSimple(recv.code)
		inner := f.isA(n, expr{code: derefOpt(recv.code, o), typ: o.Elem}, cls)
		return "(" + optNotNil(recv.code, o) + " && " + inner + ")"
	}
	if isNil(t) {
		return "false"
	}
	if u, ok := t.(TUnion); ok {
		return f.unionIsA(n, recv, u, cls)
	}
	if c, ok := f.numericIsA(recv, cls); ok {
		return c
	}
	if cls.IsModule {
		if c, ok := f.runtimeModuleIsA(recv, cls); ok {
			return c
		}
		return f.moduleIsA(n, t, cls)
	}
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
		if f.owner.IsModule || f.owner.universal { // self is some includer, or any object in an Object reopening: ask it at run time
			t = TAny{}
		}
	}
	if isAbstract(t) && recv.code != f.selfCode {
		t = TAny{}
	}
	switch t := t.(type) {
	case TClass:
		switch {
		case t.C.isSubclassOf(cls):
			return "true"
		case cls.isStruct() && t.C.isStruct() && cls.isSubclassOf(t.C):
			return "rbIsA[" + f.c.goType(TClass{C: cls}) + "](" + recv.code + ")"
		}
		return "false"
	case TTuple:
		return strconv.FormatBool(cls.RubyName == "Array")
	case TAny, TVar: // a generic T is some value known at run time, as untyped is (Self was resolved above)
		return "rbIsA[" + f.c.isAGoType(cls) + "](" + recv.code + ")"
	case TVoid: // a void call's value is nil
		f.voidAsNil(recv)
		return "false"
	case TFunc, TNil, TOpt, TUnion: // handled before the switch
	}
	f.errorf(n, "is_a? on %s is not supported", t)
	return ""
}

// markerIsA is is_a? for TrueClass, FalseClass and NilClass, which have no
// instances of their own: the Boolean or nil value decides.
func (f *fctx) markerIsA(recv expr, cls *Class) (string, bool) {
	name := cls.RubyName
	t := recv.typ
	if u, ok := t.(TUnion); ok {
		return unionMarkerIsA(recv.code, u, name)
	}
	if name == "Proc" {
		if ft, ok := t.(TFunc); ok && ft.Proc {
			return "true", true
		}
		if isAny(t) || isAbstract(t) {
			return "rbIsProc(" + f.coerce(nil, recv, TAny{}) + ")", true
		}
		return "false", true
	}
	if name != "TrueClass" && name != "FalseClass" && name != "NilClass" {
		return "", false
	}
	if name == "NilClass" {
		switch {
		case isNil(t):
			return "true", true
		case isOpt(t):
			return "(" + optIsNil(recv.code, t) + ")", true
		case isAny(t) || isAbstract(t):
			return "(rbUnbox(" + f.coerce(nil, recv, TAny{}) + ") == nil)", true
		}
		return "false", true
	}
	not := map[bool]string{true: "", false: "!"}[name == "TrueClass"]
	switch {
	case isClass(t, "Boolean"):
		return not + "bool(" + recv.code + ")", true
	case isClass(stripOpt(t), "Boolean"):
		return "(" + recv.code + " != nil && " + not + "bool(*" + recv.code + "))", true
	case isAny(t) || isAbstract(t):
		return "rbIsBool(" + f.coerce(nil, recv, TAny{}) + ", " + strconv.FormatBool(name == "TrueClass") + ")", true
	}
	return "false", true
}

// unionMarkerIsA is markerIsA on a union (decision 150): a test of the
// value when a member can be one, else false.
func unionMarkerIsA(code string, u TUnion, name string) (string, bool) {
	has := func(p func(Type) bool) bool { return slices.ContainsFunc(u.Members, p) }
	switch name {
	case "NilClass":
		if has(isNil) {
			return "(" + code + " == nil)", true
		}
	case "TrueClass", "FalseClass":
		if has(func(m Type) bool { return isClass(m, "Boolean") }) {
			return "rbIsBool(" + code + ", " + strconv.FormatBool(name == "TrueClass") + ")", true
		}
	case "Proc":
		if has(func(m Type) bool { ft, ok := m.(TFunc); return ok && ft.Proc }) {
			return "rbIsProc(" + code + ")", true
		}
	default:
		return "", false
	}
	return "false", true
}

// isAGoType is the Go type an untyped value is asserted to for is_a?(cls).
func (c *Compiler) isAGoType(cls *Class) string {
	if len(cls.TypeParams) > 0 && cls.hashBase { // a gem's `class X < Hash` (bindata's SanitizedParameters): its instances hold untyped values
		return c.goType(TClass{C: cls, Args: anyTypes(len(cls.TypeParams))})
	}
	if len(cls.TypeParams) > 0 {
		return cls.anyName()
	}
	return c.goType(TClass{C: cls})
}

func anyTypes(n int) []Type {
	out := make([]Type, n)
	for i := range out {
		out[i] = TAny{}
	}
	return out
}

// numericIsA is is_a?(Numeric) on a value whose class only the run time knows (untyped, a module type,
// a type variable): the class ancestry table answers, as for Module#=== (decision 142).
func (f *fctx) numericIsA(recv expr, cls *Class) (string, bool) {
	if !f.c.isNumericMod(cls) || !f.numericAtRunTime(recv.typ) {
		return "", false
	}
	return fmt.Sprintf("rbKindOf(%s, %d)", recv.code, f.c.classID(cls)), true
}

// numericAtRunTime: whether a value of static type t is a Numeric is decided at run time.
func (f *fctx) numericAtRunTime(t Type) bool {
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil && !f.owner.IsModule {
		t = TClass{C: f.owner}
	}
	switch t := t.(type) {
	case TAny, TUnion, TVar: // a union's member is chosen at run time
		return true
	case TClass:
		return isAbstract(t) && !t.C.isSubclassOf(f.c.classes["Numeric"])
	case TFunc, TNil, TOpt, TTuple, TVoid: // statically never a Numeric (T? is unwrapped before)
	}
	return false
}

func (c *Compiler) isNumericMod(cls *Class) bool { return cls != nil && cls == c.classes["Numeric"] }

// runtimeModuleIsA is `x.is_a?(mod)` where x's static type cannot decide it (untyped, or a class some subclass of
// which includes mod): a switch on x's class ID over the closed world's classes that include mod (decision 228).
func (f *fctx) runtimeModuleIsA(recv expr, mod *Class) (string, bool) {
	t := stripOpt(recv.typ)
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
	}
	c, ok := t.(TClass)
	if mod.universal || isNil(t) || ok && (c.C.isSubclassOf(mod) || !c.C.includedBelow(mod)) {
		return "", false // decided by the static type (moduleIsA)
	}
	if !ok && !isAny(t) {
		return "", false
	}
	if f.c.modIsA == nil {
		f.c.modIsA = map[*Class]bool{}
	}
	f.c.modIsA[mod] = true // emitted with the program's tail (emitModIsA): a body is being written now
	return "rbHasIsA_" + mod.Name + "(" + f.coerce(nil, recv, TAny{}) + ")", true
}

// moduleIsA decides `x.is_a?(mod)` for a module mod and x of static type t.
// There is no runtime record of included modules, so it is "true" or "false"
// when t's class decides it, and a compile error when t is untyped or a
// subclass of its class includes mod (decision 21).
func (f *fctx) moduleIsA(n parser.Node, t Type, mod *Class) string {
	t = stripOpt(t)
	if v, ok := t.(TVar); ok && v.Name == "Self" && f.owner != nil {
		t = TClass{C: f.owner}
	}
	if _, ok := t.(TTuple); ok {
		t = TClass{C: f.c.classes["Array"]}
	}
	c, ok := t.(TClass)
	switch {
	case mod.universal || ok && c.C.isSubclassOf(mod):
		return "true"
	case isNil(t):
	case !ok || c.C.includedBelow(mod):
		f.errorf(n, "is_a?(%s) on %s cannot be checked: rugaroo has no runtime record of included modules", mod.RubyName, t)
	}
	return "false"
}

// narrowIsA renders `x.is_a?(C)` in a condition and narrows x to C inside.
func (f *fctx) narrowIsA(call *parser.CallNode, v *local) (string, []narrowInfo) {
	recv := expr{code: v.goName, typ: v.typ}
	cond := f.isACheck(call, recv, call.Arguments.Arguments[0])
	cls := f.classRef(call.Arguments.Arguments[0])
	if cls == nil { // a class value: nothing static to narrow to
		return cond, nil
	}
	base := stripOpt(v.typ)
	code := v.goName
	if isOpt(v.typ) {
		code = derefOpt(v.goName, v.typ)
	}
	if cond == "true" || cond == "false" || cls.universal {
		return cond, nil
	}
	if cls == f.c.classes["Proc"] && isAny(base) {
		// a Proc is a Go func (rbIsProc's cases), not the prelude's Proc struct: an untyped value stays untyped
		// (rubocop-ast's `tokens.is_a?(Proc)` on prism's deferred-token lambda; decision 496)
		return cond, nil
	}
	if nw, ok := f.narrowUnion(v, func(m Type) bool { mc := classOf(m); return mc != nil && mc.isSubclassOf(cls) }); ok {
		return cond, []narrowInfo{nw} // the members that are a cls (decision 150)
	}
	if f.c.isNumericMod(cls) && !isOpt(v.typ) && f.numericAtRunTime(base) {
		if _, tv := base.(TVar); tv {
			code = "any(" + code + ")" // a Go type parameter is not an interface: a later type assertion on it would not compile
		}
		return cond, []narrowInfo{{local: v, typ: TClass{C: cls}, code: code}}
	}
	if bt, ok := base.(TClass); cls.IsModule || ok && bt.C.isSubclassOf(cls) {
		// the class part is static: all the check can rule out is nil
		if isOpt(v.typ) && !isAny(base) {
			return cond, []narrowInfo{{local: v, typ: base}}
		}
		return cond, nil
	}
	typ := TClass{C: cls}
	for range cls.TypeParams {
		typ.Args = append(typ.Args, TAny{})
	}
	if bt, ok := base.(TClass); ok && !isAbstract(bt) && !cls.isSubclassOf(bt.C) {
		// a String? tested for Regexp: never one, so the branch never runs; its x is cls's zero value, which compiles
		return cond, []narrowInfo{{local: v, typ: typ, code: "*new(" + f.c.goType(typ) + ")"}}
	}
	if len(cls.TypeParams) > 0 {
		if cls.hashBase {
			return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + f.c.goType(typ) + ")"}}
		}
		return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + cls.anyName() + ")._ToAny()", view: code}}
	}
	return cond, []narrowInfo{{local: v, typ: typ, code: code + ".(" + f.c.goType(typ) + ")"}}
}

// moduleCmp is `C < D` (<=, >, >=) on a class or module object: an ancestry test (rbModuleCmp), compiled where the
// receiver is one rather than a Module method every class object would answer through dynamic `<` (decision 255).
func (f *fctx) moduleCmp(t TClass, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	if len(args) != 1 || block != nil || name != "<" && name != "<=" && name != ">" && name != ">=" {
		return expr{}, false
	}
	if t.C.metaOf == nil && t.C.RubyName != "Class" && t.C.RubyName != "Module" {
		return expr{}, false
	}
	if t.C.metaOf != nil && t.C.lookup(name) != nil && !t.C.lookup(name).M.File.isPrelude() { // a class's own `self.<`
		return expr{}, false
	}
	a := f.genExpr(args[0], nil)
	code := fmt.Sprintf("rbModuleCmp(%s, %s, %t, %t)", f.coerce(nil, recv, TAny{}), f.coerce(args[0], a, TAny{}), name[0] == '>', len(name) == 2)
	return expr{code: code, typ: TOpt{Elem: f.cls("Boolean")}}, true
}
