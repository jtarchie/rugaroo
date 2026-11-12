package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// widenSeed is an unannotated `x = [1, 2]`'s element type, for later writes to widen (decision 193): in pass 1 the
// local is Array[untyped] so any write is evidence; in pass 2 it is what the evidence joined to. nil otherwise.
func (f *fctx) widenSeed(n *parser.LocalVariableWriteNode, ann Type, exp *Type) Type {
	if hn, ok := n.Value.(*parser.HashNode); ok && (f.lenient() || f.localCalled(n.Name, "[]=", "store")) { // user code too: a Hash literal its later writes widen (decision 366)
		return f.widenHashSeed(n, hn, ann, exp)
	}
	if fill, ok := f.filledArray(n.Value); ok && ann == nil && *exp == nil {
		return f.widenFillSeed(n, fill, exp)
	}
	an, ok := n.Value.(*parser.ArrayNode)
	if !ok || ann != nil || *exp != nil || len(an.Elements) == 0 || slices.ContainsFunc(an.Elements, func(e parser.Node) bool { _, s := e.(*parser.SplatNode); return s }) {
		return nil
	}
	if r := f.refined[f.localKey(n.Name)]; r != nil {
		*exp = r
		return nil
	}
	if f.pass != 1 && !f.usedAsArray(n.Name) {
		return nil
	}
	var nat expr
	if f.attempt(func() { f.probe(func() { nat = f.genExpr(n.Value, nil) }) }) != nil {
		return nil
	}
	if tt, ok := nat.typ.(TTuple); ok && f.usedAsArray(n.Name) { // `a = [1, "two"]; a << 3.0`: an Array of the union (decision 366)
		*exp = TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}} // pass 2 with no refined type: its writes did not join
		if f.pass != 1 {
			return nil
		}
		return joinAllUnion(tt.Elems)
	}
	if f.pass != 1 {
		return nil
	}
	tc, ok := nat.typ.(TClass)
	if !ok || tc.C.RubyName != "Array" || len(tc.Args) != 1 || holdsAny(tc.Args[0]) {
		return nil
	}
	*exp = TClass{C: tc.C, Args: []Type{TAny{}}}
	return tc.Args[0]
}

// filledArray is a gem's `Array.new(n, x)` with a non-nil x and no block, and x.
func (f *fctx) filledArray(n parser.Node) (parser.Node, bool) {
	c, ok := n.(*parser.CallNode)
	if !ok || !f.lenient() || c.Name != "new" || c.Block != nil || emptyNewClass(&parser.CallNode{Receiver: c.Receiver, Name: "new"}) != "Array" {
		return nil, false
	}
	args := callArgs(c)
	if len(args) != 2 || isNilLit(args[1]) || isSplat(args[0]) || isSplat(args[1]) {
		return nil, false
	}
	return args[1], true
}

// widenFillSeed opens `row = Array.new(size, 0)` (matrix's diagonal, then `row[j] = values[j]`) as a literal is
// (decision 193): what is stored over the fill types it, so a Rational or untyped write is no conversion into Integer.
func (f *fctx) widenFillSeed(n *parser.LocalVariableWriteNode, fill parser.Node, exp *Type) Type {
	if r := f.refined[f.localKey(n.Name)]; r != nil {
		*exp = r
		return nil
	}
	if f.pass != 1 {
		return nil
	}
	t := f.probeType(fill, nil)
	if t == nil || holdsAny(t) || isVoid(t) {
		return nil
	}
	*exp = TClass{C: f.c.classes["Array"], Args: []Type{TAny{}}}
	return t
}

// joinAllUnion folds joinUnion over ts.
func joinAllUnion(ts []Type) Type {
	out := ts[0]
	for _, t := range ts[1:] {
		out = joinUnion(out, t)
	}
	return out
}

// localCalled reports a call of one of names on local name in the scope being compiled.
func (f *fctx) localCalled(name string, names ...string) bool {
	return f.scopeCallsOn(name, func(c *parser.CallNode) bool { return slices.Contains(names, c.Name) })
}

// scopeCallsOn reports a call on local name, in the scope being compiled, that pred accepts.
func (f *fctx) scopeCallsOn(name string, pred func(*parser.CallNode) bool) bool {
	var body parser.Node = f.f.Root
	if f.m != nil && f.m.Node != nil {
		body = f.m.Node.Body
	}
	return walkNode(body, func(x parser.Node) (bool, bool) {
		c, ok := x.(*parser.CallNode)
		if !ok {
			return false, !opensScope(x)
		}
		r, ok := c.Receiver.(*parser.LocalVariableReadNode)
		return ok && r.Name == name && pred(c), true
	})
}

// usedAsArray reports a call on local name, in the scope being compiled, that a tuple cannot answer: a mutator, a
// `!` method or an iterator of Array's.
func (f *fctx) usedAsArray(name string) bool {
	arr := f.c.classes["Array"]
	return f.scopeCallsOn(name, func(c *parser.CallNode) bool {
		e := arr.lookup(c.Name)
		return e != nil && (tupleMutators[c.Name] || strings.HasSuffix(c.Name, "!") || e.M.Iterator)
	})
}

// widenHashSeed is widenSeed for a gem's `opts = { pgroup: true }`: its key and value types as a pair, for later
// `opts[:chdir] = dir` writes to widen (SystemCommand#each_output_line).
func (f *fctx) widenHashSeed(n *parser.LocalVariableWriteNode, hn *parser.HashNode, ann Type, exp *Type) Type {
	if ann != nil || *exp != nil || len(hn.Elements) == 0 || slices.ContainsFunc(hn.Elements, func(e parser.Node) bool { _, ok := e.(*parser.AssocNode); return !ok }) {
		return nil
	}
	if r := f.refined[f.localKey(n.Name)]; r != nil {
		*exp = r
		return nil
	}
	if f.pass != 1 {
		return nil
	}
	var nat expr
	if f.attempt(func() { f.probe(func() { nat = f.genExpr(n.Value, nil) }) }) != nil {
		return nil
	}
	tc, ok := nat.typ.(TClass)
	if !ok || tc.C.RubyName != "Hash" || len(tc.Args) != 2 || holdsAny(tc.Args[0]) || holdsAny(tc.Args[1]) {
		return nil
	}
	*exp = TClass{C: tc.C, Args: []Type{TAny{}, TAny{}}}
	return TTuple{Elems: []Type{tc.Args[0], tc.Args[1]}}
}

// openNewType is an empty `X.new`'s type with untyped elements, so what is put in it can type it (decision 182); nil
// for anything else.
func (c *Compiler) openNewType(n parser.Node) Type {
	call, ok := n.(*parser.CallNode)
	if !ok {
		return nil
	}
	cls := c.classes[emptyNewClass(call)]
	if cls == nil {
		return nil
	}
	args := make([]Type, len(cls.TypeParams))
	for i := range args {
		args[i] = TAny{}
	}
	return TClass{C: cls, Args: args}
}

func isNilLit(n parser.Node) bool {
	_, ok := n.(*parser.NilNode)
	return ok
}

// isContainerLit reports an Array or Hash literal, whose elements a sibling branch's type can type.
func isContainerLit(n parser.Node) bool {
	switch n.(type) {
	case *parser.ArrayNode, *parser.HashNode:
		return true
	}
	return false
}

func isEmptyLit(n parser.Node) bool {
	switch n := n.(type) {
	case *parser.ArrayNode:
		return len(n.Elements) == 0
	case *parser.HashNode:
		return len(n.Elements) == 0
	case *parser.CallNode:
		return emptyNewClass(n) != ""
	}
	return false
}

// emptyNewClass is the class of an empty container's `X.new` (`Hash.new`, `Queue.new`, `SizedQueue.new(n)`): an open
// literal, typed from what is put in it as `[]` and `{}` are (decision 182).
func emptyNewClass(n *parser.CallNode) string {
	var name string
	switch r := n.Receiver.(type) {
	case *parser.ConstantReadNode:
		name = r.Name
	case *parser.ConstantPathNode: // ::Array
		if r.Parent == nil && r.Name != nil {
			name = *r.Name
		}
	}
	if name == "" || n.Name != "new" || n.Block != nil {
		return ""
	}
	switch args := callArgs(n); {
	case name == "SizedQueue" && len(args) == 1, len(args) == 0 && (name == "Hash" || name == "Array" || name == "Set" || name == "Queue"):
		return name
	case name == "Array" && len(args) == 2 && isNilLit(args[1]):
		return name // Array.new(n, nil): n nils, typed by what is stored over them
	case name == "Hash" && len(args) == 1 && !isKeywordArgs(args[0]):
		return name // Hash.new(default): the default types V, what is stored types K (decision 344)
	}
	return ""
}

func isKeywordArgs(n parser.Node) bool {
	_, ok := n.(*parser.KeywordHashNode)
	return ok
}

// openType is openNewType with `Hash.new(default)`'s V bound to the default's type when that is concrete, so the
// open literal keeps the V decision 5 gives it and only K is left to what is stored (decision 344).
func (f *fctx) openType(n parser.Node) Type {
	t := f.c.openNewType(n)
	call, ok := n.(*parser.CallNode)
	tc, isClass := t.(TClass)
	if !ok || !isClass || tc.C.RubyName != "Hash" || len(callArgs(call)) != 1 {
		return t
	}
	if v := f.probeType(callArgs(call)[0], nil); v != nil && !isNil(v) && !holdsAny(v) {
		tc.Args = []Type{TAny{}, v}
	}
	return tc
}

// noteElems records what `x << v`, `x.push(v)`, `x[i] = v` and
// `h[k] = v` put in an open `[]`/`{}`: a local in pass 1 (analyze), an
// ivar during discovery (refineIvars).
func (f *fctx) noteElems(n *parser.CallNode) {
	if n.Block != nil {
		return
	}
	var t Type
	var open *bool
	var elems *[2][]Type
	seeded := false
	switch r := n.Receiver.(type) {
	case *parser.LocalVariableReadNode:
		if info := f.localInfo(r.Name); f.pass == 1 && info != nil && info.open {
			if tc, ok := info.typ.(TClass); ok && tc.C.isTemplate() {
				f.noteTemplateArgs(n, tc.C, info)
				return
			}
			t, open, elems, seeded = info.typ, &info.open, &info.elems, info.seeded
		}
	case *parser.InstanceVariableReadNode:
		if f.discover && f.owner != nil {
			if iv := f.c.findIvar(f.owner, r.Name); iv != nil && iv.open {
				t, open, elems = iv.Type, &iv.open, &iv.elems
			}
		}
	}
	if elems == nil {
		return
	}
	args := callArgs(n)
	switch hash := isClass(stripOpt(t), "Hash"); {
	case !hash && slices.Contains([]string{"<<", "push", "append", "unshift", "prepend", "add", "enq"}, n.Name):
		f.noteElem(open, &elems[0], args...)
	case !hash && n.Name == "fill" && len(args) >= 1:
		f.noteElem(open, &elems[0], args[0])
	case !hash && n.Name == "[]=" && len(args) == 2 && !isRangeNode(args[0]): // a[i] = v; a[range] = xs splices xs
		f.noteElem(open, &elems[0], args[1])
		if !seeded {
			elems[0] = append(elems[0], TNil{}) // a gap before the index reads nil (a literal's own elements say otherwise)
		}
	case hash && (n.Name == "[]=" || n.Name == "store") && len(args) == 2:
		f.noteElem(open, &elems[0], args[0])
		f.noteElem(open, &elems[1], args[1])
	}
}

// noteIndexWrite records the key of `h[k] op= v` and `h[k] ||= v` on an open Hash as `h[k] = v` does (noteElems),
// and v of `||=` as a value: `counts[w] += 1` on `Hash.new(0)` is what types K (decision 344). An op-write's value
// reads h[k] itself, so it is no evidence for V.
func (f *fctx) noteIndexWrite(recv parser.Node, args *parser.ArgumentsNode, value parser.Node) {
	if args == nil || len(args.Arguments) != 1 {
		return
	}
	call := &parser.CallNode{Receiver: recv, Name: "[]=", Arguments: args}
	var t Type
	var open *bool
	var elems *[2][]Type
	switch r := recv.(type) {
	case *parser.LocalVariableReadNode:
		if info := f.localInfo(r.Name); f.pass == 1 && info != nil && info.open {
			t, open, elems = info.typ, &info.open, &info.elems
		}
	case *parser.InstanceVariableReadNode:
		if f.discover && f.owner != nil {
			if iv := f.c.findIvar(f.owner, r.Name); iv != nil && iv.open {
				t, open, elems = iv.Type, &iv.open, &iv.elems
			}
		}
	}
	if elems == nil || !isClass(stripOpt(t), "Hash") {
		return
	}
	f.noteElem(open, &elems[0], callArgs(call)[0])
	if value != nil {
		f.noteElem(open, &elems[1], value)
	}
}

// memoParam is the index of the block param an open literal argument binds: `each_with_object({}) { |x, acc| }` and
// `inject(Hash.new(0)) { |acc, x| }` (decision 344); -1 otherwise.
func memoParam(n *parser.CallNode) int {
	blk, ok := n.Block.(*parser.BlockNode)
	args := callArgs(n)
	if !ok || n.Receiver == nil || len(args) != 1 || !isEmptyLit(args[0]) {
		return -1
	}
	_ = blk
	switch n.Name {
	case "each_with_object":
		return 1
	case "inject", "reduce":
		return 0
	}
	return -1
}

// openMemo makes the memo block param of an open literal argument an open local in pass 1, typed by what the block
// puts in it as `acc = {}` would be, and remembers which argument it came from: pass 2 passes that literal its
// refined type (openArgType).
func (f *fctx) openMemo(n *parser.CallNode, names blockParams) {
	i := memoParam(n)
	if i < 0 || i >= len(names) || names[i].Sub != nil || f.openArgs == nil {
		return
	}
	key := f.localKey(names[i].Name)
	f.openArgs[callArgs(n)[0].GetLocation().StartOffset] = key
	if info := f.locals[key]; f.pass == 1 && info != nil {
		info.open = true
	}
}

// openArgType is the refined type of an open literal passed as a memo (openMemo), or nil.
func (f *fctx) openArgType(n parser.Node) Type {
	if f.openArgs == nil {
		return nil
	}
	k, ok := f.openArgs[n.GetLocation().StartOffset]
	if !ok {
		return nil
	}
	return f.refined[k]
}

func (f *fctx) noteElem(open *bool, elems *[]Type, args ...parser.Node) {
	for _, a := range args {
		if _, ok := a.(*parser.SplatNode); ok {
			*open = false
			return
		}
		t := f.probeType(a, nil)
		*elems = append(*elems, t)
	}
}

// noteConv records a conversion site for analyze.
func (f *fctx) noteConv(n parser.Node, code string) string {
	if n == nil { // an adapter's own conversion (procArityAdapter): no source site to record
		return code
	}
	if f.pass == 1 && f.convs != nil {
		f.convs[n.GetLocation().StartOffset] = true
	}
	if f.pass == 2 && f.c.convs != nil && (f.storing > 0 || !f.lenient()) {
		// keyed by what it converts to as well: the untyped baseline's rbAs[*Hash[any, any]] at a site is not the typing's
		// rbAs[*Hash[Symbol, Boolean]] there, which can fail on the Hash's contents (decision 444)
		target, _, _ := strings.Cut(code, "(")
		f.c.convs[fmt.Sprintf("%s:%d:%s", f.f.Name, n.GetLocation().StartOffset, target)] = true
	}
	return code
}

// selfCallingProc is `x = lambda { ... x ... }` (or proc, or ->): a literal block reading the local it is assigned to.
func selfCallingProc(n *parser.LocalVariableWriteNode) bool {
	var body parser.Node
	switch v := n.Value.(type) {
	case *parser.LambdaNode:
		body = v.Body
	case *parser.CallNode:
		if blk, ok := v.Block.(*parser.BlockNode); ok && v.Receiver == nil && (v.Name == "lambda" || v.Name == "proc") {
			body = blk.Body
		}
	}
	return body != nil && anyNode(body, func(x parser.Node) bool {
		r, ok := x.(*parser.LocalVariableReadNode)
		return ok && r.Name == n.Name && r.Depth > 0
	})
}

// selfProcDecl declares, untyped, a gem's local assigned a literal block that reads it (kramdown's `append_text =
// lambda do |child| ... append_text.call(c) ... end`): Go must declare it first, and its call then goes through the
// Proc dispatcher (decision 330). exp otherwise.
func (f *fctx) selfProcDecl(n *parser.LocalVariableWriteNode, ann, exp Type) Type {
	if exp != nil || ann != nil || !f.lenient() || !selfCallingProc(n) {
		return exp
	}
	f.assignLocal(n, n.Name, expr{code: "nil", typ: TNil{}}, TAny{})
	return TAny{}
}

// noteAliasSrc records a gem's `prev = @nodelist.last` when it is prev's only assignment in the method: a String
// edit of prev (`prev.rstrip!`) is MRI's edit of that slot's object, so it writes the slot back too (decision 331).
func (f *fctx) noteAliasSrc(n *parser.LocalVariableWriteNode) {
	call, ok := n.Value.(*parser.CallNode)
	if !ok || !f.lenient() || f.m == nil || f.m.Node == nil {
		return
	}
	if _, ok := f.elementTarget(call); !ok {
		return
	}
	writes := 0
	anyNode(f.m.Node, func(x parser.Node) bool {
		switch w := x.(type) {
		case *parser.LocalVariableWriteNode:
			if w.Name == n.Name {
				writes++
			}
		case *parser.LocalVariableTargetNode, *parser.LocalVariableOperatorWriteNode, *parser.LocalVariableOrWriteNode:
			if name, _, _ := localVarRef(w); name == n.Name {
				writes += 2
			}
		}
		return false
	})
	if writes != 1 {
		return
	}
	if f.aliasSrc == nil {
		f.aliasSrc = map[localKey]*parser.CallNode{}
	}
	f.aliasSrc[f.localKey(n.Name)] = call
}
