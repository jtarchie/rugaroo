package compiler

import (
	"maps"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Record shapes: what each Symbol key of an options Hash holds, in the not-known-yet rounds
// (docs/research/inference-2026-11.md).
//
// A Hash literal of mixed values is `Hash[Symbol, untyped]`, so `opts = {text_type: :raw, stack: []}.merge(opts)`
// then `@src = opts[:src]` reads untyped values (kramdown's `reset_env`). A shape is side information, never part
// of a type: the fields of a literal with Symbol-literal keys, of `a.merge(b)` when both have one, of a local every
// write of which has one, and of a parameter every call passes such a literal. It is trusted only for a local the
// def's body uses read-only as a Hash (recordOnly), so a store, a delete, or handing the Hash to other code falls
// back to the untyped read. `opts[:key]` and `opts.fetch(:key)` of a shape's field are then the field's type,
// converted where the untyped value is read, as any untyped value into a typed slot is.

type recField struct {
	typ Type
	opt bool // some source does not have the key: `[]` answers nil for it
	// empty: the `[]` literal that is the field's value in every source, all of them in this def. An ivar assigned
	// the field once, straight in the def's body, is then an open Array as `@stack = []` is (recEmptyLit)
	empty parser.Node
}

// recShape is a record's fields by key; nil is "no shape", which an empty literal's empty shape is not.
type recShape map[string]recField

// Keys of a parameter's shape in the inference state, beside the parameter's own key k: the mark that a call
// passed a record, the flag that the shape is off, a field's type and the flag that a field is optional.
func recMarkKey(k pendingKey) pendingKey       { return k + "%" }
func recBadKey(k pendingKey) pendingKey        { return k + "%!" }
func recKey(k pendingKey, f string) pendingKey { return k + pendingKey("%"+f) }
func recOptKey(k pendingKey, f string) pendingKey {
	return k + pendingKey("%?"+f)
}

// recordSafe are the Hash methods that neither change their receiver nor hand it on.
var recordSafe = map[string]bool{
	"[]": true, "fetch": true, "key?": true, "has_key?": true, "include?": true, "member?": true, "merge": true,
	"dig": true, "size": true, "length": true, "empty?": true, "keys": true, "values": true, "each": true,
	"each_pair": true, "nil?": true, "inspect": true, "to_s": true, "any?": true,
}

// recordOnly reports that body uses local name only as a read-only Hash: every read is the receiver of a
// recordSafe call or the argument of a merge, every write a plain assignment, and nothing else binds the name.
func recordOnly(body parser.Node, name string) bool {
	reads, good := 0, 0
	isRead := func(x parser.Node) bool {
		r, ok := x.(*parser.LocalVariableReadNode)
		return ok && r.Name == name
	}
	bad := walkNode(body, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.LocalVariableReadNode:
			if x.Name == name {
				reads++
			}
		case *parser.CallNode:
			if x.Receiver != nil && isRead(x.Receiver) && recordSafe[x.Name] {
				good++
			}
			if args := callArgs(x); x.Name == "merge" && len(args) == 1 && isRead(args[0]) {
				good++
			}
		case *parser.LocalVariableOperatorWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableOrWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableAndWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableTargetNode:
			return x.Name == name, true
		case *parser.RequiredParameterNode:
			return x.Name == name, true
		case *parser.OptionalParameterNode:
			return x.Name == name, true
		case *parser.BlockLocalVariableNode:
			return x.Name == name, true
		}
		return false, true
	})
	return !bad && reads == good
}

// recOfNode is the shape of the value n evaluates to, nil when it has none.
func (f *fctx) recOfNode(n parser.Node) recShape {
	switch n := n.(type) {
	case *parser.ParenthesesNode:
		if st, ok := n.Body.(*parser.StatementsNode); ok && len(st.Body) == 1 {
			return f.recOfNode(st.Body[0])
		}
	case *parser.HashNode:
		return f.recOfLiteral(n.Elements)
	case *parser.KeywordHashNode:
		return f.recOfLiteral(n.Elements)
	case *parser.LocalVariableReadNode:
		return f.recOfLocal(n.Name)
	case *parser.CallNode:
		if args := callArgs(n); n.Name == "merge" && n.Receiver != nil && len(args) == 1 && n.Block == nil {
			return recMerge(f.recOfNode(n.Receiver), f.recOfNode(args[0]))
		}
	}
	return nil
}

func (f *fctx) recOfLiteral(elements []parser.Node) recShape {
	out := recShape{}
	for _, el := range elements {
		a, ok := el.(*parser.AssocNode)
		if !ok {
			return nil // `**h`: its keys are not written here
		}
		k, ok := a.Key.(*parser.SymbolNode)
		if !ok {
			return nil
		}
		t := f.probeType(a.Value, nil)
		if t == nil || isVoid(t) {
			t = TAny{}
		}
		fld := recField{typ: t}
		if ary, ok := a.Value.(*parser.ArrayNode); ok && len(ary.Elements) == 0 {
			fld.empty = ary
		}
		out[k.Unescaped.Value] = fld
	}
	return out
}

// recEmptyIvarWrite is an ivar write of a record's `[]` field (recEmptyLit): the ivar is the open Array `@x = []`
// declares. The Hash's own value is checked to be that empty Array, then left behind.
func (f *fctx) recEmptyIvarWrite(n *parser.InstanceVariableWriteNode) (expr, bool) {
	lit, key := f.recEmptyLit(n)
	if lit == nil {
		return expr{}, false
	}
	v := f.genExpr(n.Value, nil)
	f.emit("if a, ok := any(%s).(*Array[any]); !ok || len(a.s) != 0 {", v.code)
	f.emit("\tpanic(NewTypeError(Ref(String(%q))))", "rugaroo: key :"+key+" is not the empty Array every call was seen to give it")
	f.emit("}")
	w := *n
	w.Value = lit
	return f.genIvarWrite1(&w), true
}

// recEmptyLit is the `[]` literal the value of ivar write w is, when w assigns a record field that is that literal
// in every source: `opts = {stack: []}.merge(opts)` then `@stack = opts[:stack]`, where no call passes `stack:`.
// The write must be a statement of the def's own body and the only read of that key, so the Array it stands for
// was created by this call and nothing else holds it.
func (f *fctx) recEmptyLit(w *parser.InstanceVariableWriteNode) (parser.Node, string) {
	n, ok := w.Value.(*parser.CallNode)
	if !ok || !f.c.env.InferUnknown || !f.lenient() || f.m == nil || f.m.Node == nil || n.Block != nil || n.Name != "[]" {
		return nil, ""
	}
	r, ok := n.Receiver.(*parser.LocalVariableReadNode)
	args := callArgs(n)
	if !ok || len(args) != 1 {
		return nil, ""
	}
	k, ok := args[0].(*parser.SymbolNode)
	if !ok {
		return nil, ""
	}
	key := k.Unescaped.Value
	fld, ok := f.recOfLocal(r.Name)[key]
	body, isBody := f.m.Node.Body.(*parser.StatementsNode)
	if !ok || fld.empty == nil || fld.opt || !isBody || !slices.Contains(body.Body, parser.Node(w)) {
		return nil, ""
	}
	reads := 0
	walkNode(f.m.Node.Body, func(x parser.Node) (bool, bool) {
		if c, ok := x.(*parser.CallNode); ok && (c.Name == "[]" || c.Name == "fetch" || c.Name == "dig") {
			if cr, ok := c.Receiver.(*parser.LocalVariableReadNode); ok && cr.Name == r.Name {
				if ca := callArgs(c); len(ca) >= 1 {
					if ck, ok := ca[0].(*parser.SymbolNode); ok && ck.Unescaped.Value == key {
						reads++
					}
				}
			}
		}
		return false, true
	})
	if reads != 1 {
		return nil, ""
	}
	return fld.empty, key
}

// recOfLocal is the shape of local name in the def being compiled: the join of its parameter's and of each
// assignment's, when all have one and the body only reads it as a record.
func (f *fctx) recOfLocal(name string) recShape {
	if f.m == nil || f.m.Node == nil || !f.c.env.InferUnknown {
		return nil
	}
	if s, ok := f.recLocals[name]; ok {
		return s // also the answer to a write that reads the local itself: what it held before
	}
	if f.recLocals == nil {
		f.recLocals = map[string]recShape{}
	}
	body := f.m.Node.Body
	var out recShape
	isParam := false
	for _, p := range f.m.Params {
		if p.Name == name {
			isParam = true
			if !p.Rest && !p.Keyword && !p.KwRest {
				out = f.c.paramShape(p.Pending)
			}
		}
	}
	f.recLocals[name] = out
	if isParam && out == nil || !recordOnly(body, name) {
		f.recLocals[name] = nil
		return nil
	}
	var writes []*parser.LocalVariableWriteNode
	walkNode(body, func(x parser.Node) (bool, bool) {
		if w, ok := x.(*parser.LocalVariableWriteNode); ok && w.Name == name {
			writes = append(writes, w)
		}
		return false, true
	})
	if !isParam && len(writes) == 0 {
		return nil
	}
	for i, w := range writes {
		s := f.recOfNode(w.Value)
		if s == nil {
			f.recLocals[name] = nil
			return nil
		}
		if i == 0 && (!isParam || len(writes) == 1 && firstStatement(body, w)) {
			out = s // also `opts = DEFAULTS.merge(opts)` opening the def: every later read sees the merged Hash
		} else {
			out = recJoin(out, s)
		}
	}
	f.recLocals[name] = out
	return out
}

// recJoin is the shape of a value that is one of a and b.
func recJoin(a, b recShape) recShape {
	if a == nil || b == nil {
		return nil
	}
	out := recShape{}
	for _, k := range slices.Sorted(maps.Keys(a)) {
		fa := a[k]
		fb, ok := b[k]
		if !ok {
			out[k] = recField{typ: fa.typ, opt: true}
			continue
		}
		j := recField{typ: recJoinType(fa.typ, fb.typ), opt: fa.opt || fb.opt}
		if fa.empty != nil && fb.empty != nil {
			j.empty = fa.empty
		}
		out[k] = j
	}
	for k, fb := range b {
		if _, ok := a[k]; !ok {
			out[k] = recField{typ: fb.typ, opt: true}
		}
	}
	return out
}

func recJoinType(a, b Type) Type {
	if j, ok := join(a, b); ok {
		return j
	}
	return TAny{}
}

// recMerge is the shape of `a.merge(b)`: b's keys win, unless b may lack one.
func recMerge(a, b recShape) recShape {
	if a == nil || b == nil {
		return nil
	}
	out := maps.Clone(a)
	for k, fb := range b {
		fa, had := a[k]
		switch {
		case !fb.opt:
			out[k] = fb
		case had:
			out[k] = recField{typ: recJoinType(fa.typ, fb.typ), opt: fa.opt}
		default:
			out[k] = fb
		}
	}
	return out
}

// recRead is the type of `h[:key]` or `h.fetch(:key)` at n when h has a shape with that field; nil otherwise.
func (f *fctx) recRead(n *parser.CallNode) Type {
	if !f.c.env.InferUnknown || !f.lenient() || n.Receiver == nil || n.Block != nil || n.Name != "[]" && n.Name != "fetch" {
		return nil
	}
	args := callArgs(n)
	if len(args) != 1 {
		return nil
	}
	k, ok := args[0].(*parser.SymbolNode)
	if !ok {
		return nil
	}
	fld, ok := f.recOfNode(n.Receiver)[k.Unescaped.Value]
	if !ok || fld.typ == nil || isAny(stripOpt(fld.typ)) || isNil(fld.typ) || isVoid(fld.typ) {
		return nil
	}
	if fld.opt && n.Name == "[]" {
		return optOf(fld.typ)
	}
	return fld.typ
}

// recAbsent is `h[:key]` of a key no source of h's shape has: nil, after a check that the Hash indeed lacks it
// (a caller the rounds did not see would otherwise have its value dropped without a word).
func (f *fctx) recAbsent(n *parser.CallNode) (expr, bool) {
	if !f.c.env.InferUnknown || !f.lenient() || n.Block != nil || n.Name != "[]" {
		return expr{}, false
	}
	r, ok := n.Receiver.(*parser.LocalVariableReadNode)
	args := callArgs(n)
	if !ok || len(args) != 1 {
		return expr{}, false
	}
	k, ok := args[0].(*parser.SymbolNode)
	if !ok {
		return expr{}, false
	}
	shape := f.recOfLocal(r.Name)
	if _, has := shape[k.Unescaped.Value]; shape == nil || has {
		return expr{}, false
	}
	recv := f.genExpr(r, nil)
	if hc, ok := recv.typ.(TClass); !ok || hc.C.RubyName != "Hash" || len(hc.Args) != 2 || !isClass(hc.Args[0], "Symbol") || !isAny(hc.Args[1]) {
		return expr{}, false // not typed as the record's Hash (yet): the ordinary read
	}
	key := k.Unescaped.Value
	f.emit("if _, has := %s.rbGet(Symbol(%q)); has {", recv.code, key)
	f.emit("\tpanic(NewTypeError(Ref(String(%q))))", "rugaroo: key :"+key+" in a Hash no call was seen to give it")
	f.emit("}")
	return expr{code: "nil", typ: TNil{}}, true
}

// noteRecUse records what argument an gives parameter p's shape: its fields when it is a record, a mark that the
// parameter was passed one, and the flag that turns the shape off when the argument is no record and its own use
// was not recorded (a value not known yet).
func (f *fctx) noteRecUse(p Param, an parser.Node, t Type) {
	if !f.c.env.InferUnknown || p.Pending == "" || f.c.uses == nil || an == nil || p.Rest || p.Keyword {
		return
	}
	s := f.recOfNode(an)
	if s == nil {
		if holdsUnknown(t) {
			f.noteUse(Param{Pending: recBadKey(p.Pending)}, an, f.cls("Boolean"))
		}
		return
	}
	f.noteUse(Param{Pending: recMarkKey(p.Pending)}, an, f.cls("Boolean"))
	for k, fld := range s {
		f.noteUse(Param{Pending: recKey(p.Pending, k)}, an, fld.typ)
	}
}

// inferRecords settles each parameter's shape after a round: off unless every recorded use of the parameter was a
// record (a dynamic call by name, a variable, a non-empty default are not), a field untyped when one call passed
// it something untyped, and optional when a call left it out.
func (c *Compiler) inferRecords(next map[pendingKey]Type, order map[*File]int) {
	boolean := TClass{C: c.classes["Boolean"]}
	for _, mk := range slices.Sorted(maps.Keys(c.uses)) {
		p, ok := strings.CutSuffix(string(mk), "%")
		if !ok {
			continue
		}
		key := pendingKey(p)
		marks := len(c.lastPassUses(mk, order))
		if marks != len(c.lastPassUses(key, order)) {
			next[recBadKey(key)] = boolean
			continue
		}
		// every call passes a literal with Symbol keys, so the parameter is that Hash and not the join of the literals'
		// own types (`{}` beside `{src: scanner}`), which is untyped: its reads are then Hash reads, not dynamic calls
		next[key] = TClass{C: c.classes["Hash"], Args: []Type{TClass{C: c.classes["Symbol"]}, TAny{}}}
		prefix := p + "%"
		for _, fk := range slices.Sorted(maps.Keys(c.uses)) {
			name, ok := strings.CutPrefix(string(fk), prefix)
			if !ok || name == "" || name == "!" || strings.HasPrefix(name, "?") {
				continue
			}
			uses := c.lastPassUses(fk, order)
			if slices.ContainsFunc(uses, func(u paramUse) bool { return isAny(u.typ) }) {
				next[fk] = TAny{}
			}
			if len(uses) < marks {
				next[recOptKey(key, name)] = boolean
			}
		}
	}
}

// paramShape is the shape the rounds gave the parameter of key k; nil when they gave none.
func (c *Compiler) paramShape(k pendingKey) recShape {
	if k == "" || c.infer == nil || !c.env.InferUnknown {
		return nil
	}
	if s, ok := c.recMemo[k]; ok {
		return s
	}
	if c.recMemo == nil {
		c.recMemo = map[pendingKey]recShape{}
	}
	var out recShape
	_, marked := c.infer.types[recMarkKey(k)]
	_, bad := c.infer.types[recBadKey(k)]
	if marked && !bad {
		out = recShape{}
		prefix := string(k) + "%"
		for fk, t := range c.infer.types {
			name, ok := strings.CutPrefix(string(fk), prefix)
			if !ok || name == "" || name == "!" || strings.HasPrefix(name, "?") {
				continue
			}
			_, opt := c.infer.types[recOptKey(k, name)]
			out[name] = recField{typ: c.rehome(t), opt: opt}
		}
		for fk := range c.infer.types { // a field every call left untyped has no type key: optional, untyped
			if name, ok := strings.CutPrefix(string(fk), prefix+"?"); ok {
				if _, has := out[name]; !has {
					out[name] = recField{typ: TAny{}, opt: true}
				}
			}
		}
	}
	c.recMemo[k] = out
	return out
}

// firstStatement reports that n is the first statement of body.
func firstStatement(body, n parser.Node) bool {
	st, ok := body.(*parser.StatementsNode)
	return ok && len(st.Body) > 0 && st.Body[0] == n
}
