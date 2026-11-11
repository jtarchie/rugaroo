package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// hoistLocals declares, at the top of a Ruby scope's Go body, the locals
// of that scope that sameScopeLocal found outside their Go block: a block's
// are fresh for each call, as in Ruby.
func (f *fctx) hoistLocals(scope int) {
	if f.pass != 2 {
		return
	}
	if scope == methodScope {
		for _, name := range slices.Sorted(maps.Keys(f.ivarBufs)) {
			f.emit("var %s []byte", ivarBuf(name))
		}
	}
	for _, li := range sortedLocals(f.locals) {
		if li.scope == scope && li.appended && !strings.HasSuffix(f.appendBufRef(li.name), ".b") { // an out parameter appends through its caller's buffer
			f.emit("var %s []byte", appendBuf(li.name))
			if li.plusBuf {
				f.emit("_ = %s", appendBuf(li.name))
			}
		}
		if li.scope != scope || !li.hoist || li.noHoist || li.typ == nil {
			continue
		}
		switch {
		case !isNil(li.typ):
			f.emit("var %s %s", goLocalName(li.name), f.c.goType(li.typ))
		case f.lenient(): // a gem's local only ever nil is untyped (decision 183), and its assignments expect a declaration (decision 244)
			f.emit("var %s any", goLocalName(li.name))
		default:
			continue
		}
		if f.lenient() && goLocalName(li.name) != "_" { // a gem arm that does not compile (decision 244) may have been its only reader
			f.emit("_ = %s", goLocalName(li.name))
		}
	}
}

func (f *fctx) declareLocal(name string, typ Type) *local {
	key := f.localKey(name)
	info := f.locals[key]
	if info == nil {
		info = &localInfo{declBlock: f.block, declRuby: f.rbScope, typ: typ}
		f.locals[key] = info
	}
	if info.declPass != f.pass {
		info.declPass = f.pass
		info.declBlock, info.declRuby = f.block, f.rbScope
	}
	if !f.declBin && f.pass < 2 { // a block param, rescue var or other binding: not known binary (decision 352)
		info.text = true
	}
	if info.noHoist {
		// block params are fresh per block; never share analysis
		info.declBlock = f.block
		info.typ = typ
		info.hoist = false
	} else if f.pass < 2 && info.typ != nil && !info.annotated {
		if j, ok := join(info.typ, typ); ok {
			info.typ = j
		}
	}
	goName := goLocalName(name)
	if f.pass == 2 && info.typ != nil {
		typ = info.typ
	}
	v := &local{name: name, goName: goName, typ: typ, info: info}
	f.scope.vars[name] = v
	return v
}

func (f *fctx) noteUnused(v *local) {
	info := v.owner()
	// `|_, v|`: Go's blank needs no use. A gem's read may sit in a branch that raises instead (decision 252): used too.
	if f.pass == 2 && info != nil && (info.reads == 0 || f.lenient()) && v.goName != "_" {
		f.emit("_ = %s", v.goName)
	}
}

func (f *fctx) readLocal(n *parser.LocalVariableReadNode) *local {
	v := f.visibleLocal(n.Name)
	if v == nil {
		v = f.sameScopeLocal(n.Name)
	}
	if v == nil && f.m != nil && n.Name == f.m.BlockParam {
		f.errorf(n, "the block parameter &%s can only be called (%s.call) or passed on (&%s)", n.Name, n.Name, n.Name)
	}
	if v == nil {
		f.errorf(n, "undefined local %s", n.Name)
	}
	info := f.localInfo(n.Name)
	if info != nil {
		info.reads++
		if !isAncestorBlock(info.declBlock, f.block) {
			info.hoist = true
		}
	}
	return v
}

// assignLocal emits `x := v` / `x = v` and returns the local.
// gemVoidValue is a gem's void value as an assignment's: `last_value = yield x` into a block the gem's sig says is void
// (prime's each) runs the call and assigns nil (decisions 119, 410).
func (f *fctx) gemVoidValue(val expr) expr {
	if isVoid(val.typ) && !isNil(val.typ) && f.lenient() {
		return f.voidAsNil(val)
	}
	return val
}

func (f *fctx) assignLocal(n parser.Node, name string, val expr, annotated Type) expr {
	val = f.gemVoidValue(val)
	existing := f.visibleLocal(name)
	if existing == nil {
		existing = f.sameScopeLocal(name)
	}
	info := f.localInfo(name)
	annotated = f.promotedType(name, val, annotated)
	var typ Type
	switch {
	case annotated != nil:
		typ = annotated
	case existing != nil && existing.base == nil:
		typ = existing.typ
	default:
		typ = val.typ
		if isNil(typ) && info != nil && info.typ != nil {
			typ = info.typ
		}
	}
	narrowed := existing != nil && existing.base != nil
	if e, ok := f.reassignView(n, name, info, existing, val, annotated); ok {
		return e
	}
	if narrowed {
		existing = existing.base
	}
	if isNil(typ) && f.pass == 2 && f.lenient() {
		typ = TAny{} // gem code: a local only ever nil is untyped (decision 183)
	}
	if isNil(typ) && f.pass == 2 { // a local only ever nil is a NilClass, its calls nil's own (decision 361)
		typ = f.cls("NilClass")
	}
	if existing == nil {
		f.noteAnyOrigin(n, name, val)
		return f.declareAssign(n, name, typ, val, annotated)
	}
	if e, ok := f.rebindFuncLocal(name, existing, val, annotated); ok {
		return e
	}
	f.noteAnyOrigin(n, name, val)
	f.widenParam(name, info, existing, val)
	if info != nil {
		f.noteReassign(n, name, info, existing, val)
	}
	target := existing.typ
	if f.pass < 2 && info != nil && !info.annotated && !isOpt(val.typ) && !isOpt(info.typ) {
		target = info.typ // the join so far: `x = B.new; x = C.new` makes x their common superclass, decided by pass 2; a T? into a T local stays an error (decision 20)
	}
	f.emit("%s = %s", existing.goName, f.coerce(n, val, target))
	if narrowed {
		f.unnarrow(name)
		if isOpt(existing.typ) && typeEq(stripOpt(existing.typ), val.typ) { // a T? local given a T stays narrowed
			f.applyNarrow([]narrowInfo{{local: existing, typ: stripOpt(existing.typ)}})
			return expr{code: derefOpt(existing.goName, existing.typ), typ: stripOpt(existing.typ), stmt: true, done: true}
		}
	}
	if e, ok := f.narrowLeafAssign(existing, val, narrowed); ok {
		return e
	}
	if e, ok := f.narrowAssign(existing, val); ok {
		return e
	}
	return f.narrowSet(existing, val)
}

// rebindFuncLocal: a gem rebinds its block parameter to what the block's call returned (`task = ScheduledTask.execute(&task)`):
// a new Go variable of the value's type takes the name from here on.
func (f *fctx) rebindFuncLocal(name string, existing *local, val expr, annotated Type) (expr, bool) {
	if !f.lenient() || f.pass != 2 || annotated != nil || existing.base != nil || isNil(val.typ) || isAny(val.typ) || val.code == "nil" || isFuncOrOptFunc(val.typ) || !isFuncOrOptFunc(existing.typ) {
		return expr{}, false
	}
	v := &local{name: name, goName: f.newTmp(), typ: val.typ, declared: true, info: existing.info}
	f.scope.vars[name] = v
	f.emit("%s := %s", v.goName, val.code)
	f.emit("_ = %s", v.goName)
	return expr{code: v.goName, typ: v.typ, stmt: true, done: true}, true
}

// reassignView writes a narrowAssign view's class again: the view stays, its var takes the value too (decision 283).
func (f *fctx) reassignView(n parser.Node, name string, info *localInfo, view *local, val expr, annotated Type) (expr, bool) {
	if view == nil || view.base == nil || !view.assigned || !typeEq(view.typ, val.typ) || annotated != nil {
		return expr{}, false
	}
	if info != nil {
		f.noteReassign(n, name, info, view.base, val)
	}
	f.emit("%s = %s", view.goName, f.coerce(n, val, view.typ))
	f.emit("%s = %s", view.base.goName, f.coerce(n, expr{code: view.goName, typ: view.typ}, view.base.typ))
	f.emit("_ = %s", view.base.goName) // later reads may all go through the view
	return expr{code: view.goName, typ: view.typ, stmt: true, done: true}, true
}

// narrowLeafAssign: a leaf class's C? local (a nil-able *C, decision 360) given a C is not nil until its next write, so
// calls on it need no NoMethodError guard (decision 371). Not when a block writes it, as for narrowAssign.
func (f *fctx) narrowLeafAssign(v *local, val expr, narrowed bool) (expr, bool) {
	if narrowed || !leafOpt(v.typ) || !typeEq(stripOpt(v.typ), val.typ) || f.m == nil || f.m.Node == nil || blockWritesLocal(f.m.Node.Body, v.name, f.isParam(v.name)) {
		return expr{}, false
	}
	f.applyNarrow([]narrowInfo{{local: v, typ: val.typ}})
	return expr{code: v.goName, typ: val.typ, stmt: true, done: true}, true
}

// narrowAssign: `x = v` into a union local, v of one class that fits it, leaves x that class until its next write
// or the end of the scope (`src = Pathname(src)` with src: String | Pathname, then `src.dirname`), one Go assertion
// here instead of a type switch or a dynamic call at each use (decision 283). Not when a block or lambda writes x,
// which would leave the view stale.
func (f *fctx) narrowAssign(v *local, val expr) (expr, bool) {
	u, ok := v.typ.(TUnion)
	if !ok || isAny(val.typ) || isNil(val.typ) || isUnion(val.typ) || isOpt(val.typ) || !fitsValue(val, u) {
		return expr{}, false
	}
	if _, cls := val.typ.(TClass); !cls || f.m == nil || f.m.Node == nil || writtenInBlock(f.m.Node.Body, v.name, false) {
		return expr{}, false
	}
	tmp := f.newTmp()
	f.emit("%s := %s.(%s)", tmp, v.goName, f.c.goType(val.typ))
	f.emit("_ = %s", tmp)
	f.applyNarrow([]narrowInfo{{local: v, typ: val.typ, code: tmp, assigned: true}})
	return expr{code: tmp, typ: val.typ, stmt: true, done: true}, true
}

// blockWritesLocal is writtenInBlock for the scope's own local name: a block before name's first assignment outside
// blocks (or its parameter) writes a block-local of its own, by Ruby's scoping, so only later blocks count.
func blockWritesLocal(body parser.Node, name string, param bool) bool {
	first := -1
	if param {
		first = 0
	} else {
		walkNode(body, func(x parser.Node) (bool, bool) {
			switch x.(type) {
			case *parser.BlockNode, *parser.LambdaNode, *parser.DefNode:
				return false, false
			}
			if at := x.GetLocation().StartOffset; assignsLocalHere(x, name) && (first < 0 || at < first) {
				first = at
			}
			return false, true
		})
	}
	if first < 0 {
		return writtenInBlock(body, name, false)
	}
	return walkNode(body, func(x parser.Node) (bool, bool) {
		switch x.(type) {
		case *parser.DefNode:
			return false, false
		case *parser.BlockNode, *parser.LambdaNode:
			return x.GetLocation().StartOffset > first && writtenInBlock(x, name, false), false
		}
		return false, true
	})
}

// isParam reports whether name is a parameter of the method being generated.
func (f *fctx) isParam(name string) bool {
	return f.m != nil && (name == f.m.BlockParam || slices.ContainsFunc(f.m.Params, func(p Param) bool { return p.Name == name }))
}

// assignsLocalHere reports whether node x itself assigns local name.
func assignsLocalHere(x parser.Node, name string) bool {
	switch x := x.(type) {
	case *parser.LocalVariableWriteNode:
		return x.Name == name
	case *parser.LocalVariableOperatorWriteNode:
		return x.Name == name
	case *parser.LocalVariableOrWriteNode:
		return x.Name == name
	case *parser.LocalVariableAndWriteNode:
		return x.Name == name
	case *parser.LocalVariableTargetNode:
		return x.Name == name
	}
	return false
}

// writtenInBlock reports an assignment to local name inside a block or lambda of n.
func writtenInBlock(n parser.Node, name string, inBlock bool) bool {
	switch x := n.(type) {
	case nil, *parser.DefNode:
		return false
	case *parser.BlockNode, *parser.LambdaNode:
		inBlock = true
	case *parser.LocalVariableWriteNode:
		if inBlock && x.Name == name {
			return true
		}
	case *parser.LocalVariableOperatorWriteNode:
		if inBlock && x.Name == name {
			return true
		}
	case *parser.LocalVariableOrWriteNode:
		if inBlock && x.Name == name {
			return true
		}
	case *parser.LocalVariableAndWriteNode:
		if inBlock && x.Name == name {
			return true
		}
	case *parser.LocalVariableTargetNode:
		if inBlock && x.Name == name {
			return true
		}
	}
	for _, ch := range n.CompactChildNodes() {
		if writtenInBlock(ch, name, inBlock) {
			return true
		}
	}
	return false
}

// widenParam notes a gem method's param given what its type does not hold (`rebuild = ".#{rebuild}"` for an Integer):
// from the next pass on the body works on a copy of the join, or untyped (decision 241).
func (f *fctx) widenParam(name string, info *localInfo, existing *local, val expr) {
	if info == nil || !info.param || !f.lenient() || isVoid(val.typ) || isNil(val.typ) || isAny(existing.typ) {
		return
	}
	// an untyped value fits any type by assertion, which fails when the guess from a default was wrong
	// (`level = :FATAL; level = const_get(level)`): an unsigned param takes it untyped
	// a Hash[untyped, V] into a Hash[Symbol, V] param fits by a converted copy, which fails on another key (exec3's
	// `options = options.merge(FD => fd)`): the param widens to the Hash of untyped keys (decision 256)
	keysWider := hashKeysWider(existing.typ, val.typ)
	// fits lets any non-class type through, so a T? is judged by its T (`rebuild = (".#{rebuild}" if x)`, decision 281)
	if fitsValue(expr{typ: stripOpt(val.typ), lit: val.lit}, existing.typ) && (!isAny(val.typ) || f.m == nil || f.m.declared) && !keysWider {
		return
	}
	w, ok := join(existing.typ, val.typ)
	if !ok {
		w = TAny{}
	}
	if keysWider {
		w = val.typ
	}
	if f.paramWiden == nil {
		f.paramWiden = map[string]Type{}
	}
	f.paramWiden[name] = w
	info.typ, existing.typ = w, w
}

// noteReassign records another assignment of a local: where it happens and the join of what it holds.
func (f *fctx) noteReassign(n parser.Node, name string, info *localInfo, existing *local, val expr) {
	info.writes++
	info.noteBin(f.pass, val)
	if !isAncestorBlock(info.declBlock, f.block) {
		info.hoist = true
	}
	if f.pass < 2 && !info.annotated && !val.provisional { // spinel's bottom: the next round types it (decision 358)
		if hashKeysWider(info.typ, val.typ) && f.lenient() { // `opts = opts.merge(198 => io)`: the Hash of untyped keys (decision 257)
			info.typ = val.typ
		} else if j, ok := join(info.typ, val.typ); ok {
			if isAny(j) && !isAny(info.typ) {
				f.noteAnyJoin(n, name)
			}
			info.typ = j
		} else if !isVoid(val.typ) && f.lenient() { // a gem's local holding Array[Formula] and Array[Formula | Keg]: untyped (decision 233)
			f.setAnyOrigin(name, "mixed: "+typeString(info.typ)+" / "+typeString(val.typ))
			info.typ = TAny{}
		} else if !isVoid(val.typ) {
			f.errorf(n, "%s is assigned both %s and %s", name, info.typ, val.typ)
		}
	}
	if f.pass == 2 || isNil(val.typ) || f.lenient() && !typeEq(existing.typ, info.typ) && !isVoid(val.typ) {
		existing.typ = info.typ // `s = "a"; s = nil` widens s to String? (decision 14); a gem's retyped local reads untyped (241)
	}
}

// narrowSet: a maybe-unset local is T? only for the paths that skip its assignments, so after one it is non-nil.
func (f *fctx) narrowSet(v *local, val expr) expr {
	if !f.unset[f.localKey(v.name)] || !isOpt(v.typ) || isOpt(val.typ) || isVoid(val.typ) || isAny(val.typ) {
		return expr{code: v.goName, typ: v.typ, stmt: true, done: true}
	}
	nw := narrowInfo{local: v, typ: stripOpt(v.typ)}
	f.applyNarrow([]narrowInfo{nw})
	return expr{code: derefOpt(v.goName, v.typ), typ: nw.typ, stmt: true, done: true}
}

// declareAssign emits the first assignment of a local.
func (f *fctx) declareAssign(n parser.Node, name string, typ Type, val expr, annotated Type) expr {
	prev := f.localInfo(name)
	hoisted := f.pass == 2 && prev != nil && prev.hoist
	f.declBin = val.bin
	v := f.declareLocal(name, typ)
	f.declBin = false
	info := v.info
	info.noteBin(f.pass, val)
	switch {
	case annotated != nil:
		info.annotated = true
		info.typ = annotated
	case f.pass < 2:
		info.typ = typ
		if j, ok := join(info.typ, val.typ); ok {
			info.typ = j
		}
		if f.unset[f.localKey(name)] && !isVoid(info.typ) {
			info.typ = optOf(info.typ) // a path that skips this reads nil (decision 14)
		}
	}
	if f.pass == 2 {
		v.typ = info.typ
	}
	if v.typ == nil && f.pass == 2 && f.lenient() && isVoid(val.typ) && !isNil(val.typ) && !val.stmt && val.code != "" { // a gem's `result = super(*mods)` of a void method (Module#include): the call runs, the local is nil
		f.emitExprStmt(n, val)
		val = expr{code: "nil", typ: TNil{}}
		v.typ, info.typ = TAny{}, TAny{}
	}
	code := f.coerce(n, val, v.typ)
	if val.lit && typeEq(val.typ, v.typ) {
		code = f.c.goType(v.typ) + "(" + code + ")"
	}
	v.declared = true
	if v.typ == nil {
		f.errorf(n, "internal: local %s has no type (its value: %s)", name, code)
	}
	switch {
	case hoisted:
		f.emit("%s = %s", v.goName, code)
	case code == "nil":
		f.emit("var %s %s", v.goName, f.c.goType(v.typ))
	case f.concreteInit(val, code, v.typ), isAny(v.typ) && !isAny(val.typ):
		// concreteInit, or an untyped local seeded with a typed value: `:=`
		// would give the local the value's Go type; later writes need goType
		f.emit("var %s %s = %s", v.goName, f.c.goType(v.typ), code)
	case v.goName == "_": // `_, err = capture_io { }`: Go's blank declares nothing
		f.emit("_ = %s", code)
	default:
		f.emit("%s := %s", v.goName, code)
	}
	if f.pass < 2 {
		info.writes++
	}
	f.noteUnused(v)
	return f.narrowSet(v, val)
}

// concreteInit reports whether code, the coerced first value of a local of
// type t, has a Go type other than the interface goType(t): `:=` would then
// give the local the concrete type (*X, *X_Meta, Integer) and a later
// assignment of a subclass, another class object or another value fails.
func (f *fctx) concreteInit(val expr, code string, t Type) bool {
	gt := f.c.goType(t)
	if cls, ok := t.(TClass); gt != "any" && (!ok || !cls.C.isStruct()) {
		return false
	}
	if code != val.code && !val.lit { // coerce converted it to gt
		return false
	}
	return val.classObj || val.ctor || f.c.goType(val.typ) != gt
}

type namedInfo struct {
	localKey
	*localInfo
}

func sortedLocals(m map[localKey]*localInfo) []namedInfo {
	out := make([]namedInfo, 0, len(m))
	for k, v := range m {
		out = append(out, namedInfo{k, v})
	}
	// deterministic order
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].name < out[j-1].name; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// renameReassignedParams: a gem block's top-level `p = v` of its own param p starts a new local for the rest of the
// block (`env = env.to_s` turns a Symbol param into a String), as a block param's Go type is fixed by the block's
// signature. Ruby reads the same values, unless a closure made before the write captured p: then nothing is renamed
// (decision 241).
func renameReassignedParams(b *parser.BlockNode, params []string) {
	st, ok := b.Body.(*parser.StatementsNode)
	if !ok {
		return
	}
	for i, n := range st.Body {
		w, ok := n.(*parser.LocalVariableWriteNode)
		if !ok || w.Depth != 0 || !slices.Contains(params, w.Name) || slices.ContainsFunc(st.Body[:i], func(x parser.Node) bool { return capturesLocal(x, w.Name) }) {
			continue
		}
		old, fresh := w.Name, fmt.Sprintf("%s__r%d", w.Name, i)
		w.Name = fresh
		for _, later := range st.Body[i+1:] {
			renameLocal(later, old, fresh)
		}
		b.Locals = append(b.Locals, fresh)
	}
}

// localNamePtr covers a local's read and all five write forms so capturesLocal and renameLocal cannot disagree.
func localNamePtr(n parser.Node) *string {
	switch x := n.(type) {
	case *parser.LocalVariableReadNode:
		return &x.Name
	case *parser.LocalVariableWriteNode:
		return &x.Name
	case *parser.LocalVariableTargetNode:
		return &x.Name
	case *parser.LocalVariableOperatorWriteNode:
		return &x.Name
	case *parser.LocalVariableOrWriteNode:
		return &x.Name
	case *parser.LocalVariableAndWriteNode:
		return &x.Name
	}
	return nil
}

// capturesLocal reports a block or lambda inside n that reads or writes the local name.
func capturesLocal(n parser.Node, name string) bool {
	found := false
	var walk func(parser.Node, bool)
	walk = func(n parser.Node, inBlock bool) {
		if found || n == nil {
			return
		}
		switch n.(type) {
		case *parser.BlockNode, *parser.LambdaNode:
			inBlock = true
		}
		if p := localNamePtr(n); p != nil {
			found = inBlock && *p == name
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch, inBlock)
		}
	}
	walk(n, false)
	return found
}

// renameLocal renames the local old to fresh in n, leaving scopes that declare their own old (a block param, a def).
func renameLocal(n parser.Node, old, fresh string) {
	walkNode(n, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.DefNode:
			return false, false
		case *parser.BlockNode:
			if slices.Contains(x.Locals, old) {
				return false, false
			}
		case *parser.LambdaNode:
			if slices.Contains(x.Locals, old) {
				return false, false
			}
		}
		if p := localNamePtr(x); p != nil && *p == old {
			*p = fresh
		}
		return false, true
	})
}

// hashKeysWider: v is a Hash of untyped keys where t is a Hash of typed ones (a gem's merge of another kind of key):
// a converted copy back to t fits by type but fails on such a key (decisions 256, 257).
func hashKeysWider(t, v Type) bool {
	vc, ok := v.(TClass)
	if !ok || !isClass(vc, "Hash") || !isAny(vc.Args[0]) {
		return false
	}
	tc, ok := stripOpt(t).(TClass)
	return ok && isClass(tc, "Hash") && !isAny(tc.Args[0])
}

// isFuncOrOptFunc is a block (Proc) type, optional or not.
func isFuncOrOptFunc(t Type) bool {
	_, ok := stripOpt(t).(TFunc)
	return ok
}
