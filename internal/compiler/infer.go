package compiler

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Parameter types from use (decision 146). A def parameter with no
// annotation takes the join of what the program passes it. A compile that
// meets one first aborts (errNeedInfer); compile then runs rounds in which
// such parameters are untyped and every user body is dry-run to record the
// arguments' types, each round typing what the last one learned, until
// nothing changes. The final compile uses the types; a parameter no call
// types keeps the missing-annotation error, and a call whose argument does
// not join the earlier ones (in source order) fails at that call, as a
// mismatch does against an annotation.

var (
	errNeedInfer  = errors.New("parameter types to infer")
	errInferRound = errors.New("inference round done")
)

// Inference carries parameter types from use from one compile to the next
// of nearly the same sources (cmd/rubyspec recompiles after each cut): a
// compile starts from them, and runs no rounds while they still cover every
// untyped parameter.
type Inference struct{ inf *inference }

// NewInference is an empty Inference.
func NewInference() *Inference { return &Inference{} }

// needInfer is the panic that aborts a compile meeting an untyped parameter before inference ran.
type needInfer struct{}

const maxInferRounds = 8

// settledRounds is how many settled rounds in a row end the rounds early (inferRounds). 2 is too few: untyping spreads
// one call level per round (Layout/IndentationWidth's indentation_to_check?(base_loc) untyped in one, then the
// skip_check?(base_loc) it calls in the next), so stopping a round sooner left skip_check? asserting a Range it is not
// always passed, and the cop raised on every input; untyping the parameters still changing at the stop did not help.
// 3 matches MRI on all 15 test inputs (2026-11-12).
const settledRounds = 3

// inference is what the rounds learned: each parameter key's type, from
// the last round's compile (rehome moves it into the next one's classes).
type inference struct {
	types   map[pendingKey]Type
	noUnion map[pendingKey]bool   // parameters whose union type broke their method's body: joined without unions from then on (decision 150)
	none    map[pendingKey]bool   // pending in the last round and typed by no call: a seeded compile reports them without rounds
	noDrop  map[pendingKey]bool   // still untyped when rounds converged with its method's untyped returns dropped: no longer dropped (decision 383)
	settled int                   // rounds in a row that changed at most 2% of the parameters (decision 444)
	anyUse  map[pendingKey]bool   // a gem parameter typed by some calls while a gem call passes an untyped value: untyped once the rounds converged (decision 444)
	from    map[pendingKey]string // the first call each type came from, file:line, for a mismatch's message
	changed bool
}

// pendingParam is a parameter whose type comes from use: its key across compiles, and its name for messages.
type pendingParam struct {
	key  pendingKey
	name string
}

// paramUse is one argument passed to a pending parameter.
type paramUse struct {
	typ     Type
	file    *File
	off     int
	anyRest bool // a gem's untyped value into a rest parameter: its elements are anything (decision 253)
	widen   bool // a gem's untyped value into an overridden method: the parameter is untyped (decision 324)
	dyn     bool // a gem's dynamic call by this name, which may reach another class's method of that name (decision 331)
	deflt   bool // the parameter's own default, not a call's argument
}

// at is the call the argument came from.
func (u paramUse) at() srcPos { return srcPos{u.file, u.file.line(u.off)} }

// pendingKey names, stably across compiles, a parameter typed from use (decisions 146, 277): the key of its
// recorded arguments and of its inferred type. Its text is printed by RUGAROO_INFER_DEBUG and quoted in warnings, so
// it is built only by the constructors below.
type pendingKey string

// paramKey is def m's parameter name.
func paramKey(m *Method, name string) pendingKey {
	owner := ""
	if m.Owner != nil { // one template's defs in several classes (concurrent-ruby's attr_atomic class_eval) share a position (decision 253)
		owner = m.Owner.Name + "#"
	}
	return pendingKey(m.File.posKey(m.Node.Location.StartOffset) + owner + name)
}

// yieldKey is the i-th value def m yields to its block.
func yieldKey(m *Method, i int) pendingKey { return paramKey(m, "&"+strconv.Itoa(i)) }

// selfKey is the self of def m's block once it escapes (decision 164).
func selfKey(m *Method) pendingKey { return paramKey(m, "&self") }

// lambdaSrc names a lambda literal typed from its calls: the prefix of its parameters' keys.
type lambdaSrc string

// blockSrcAt is the unannotated def's &block parameter at offset off of f (decision 307): its Proc's calls record
// each argument by position, and their count under argc.
func blockSrcAt(f *File, off int) lambdaSrc { return lambdaSrc(f.posKey(off) + "&") }

func (s lambdaSrc) storedBlock() bool { return strings.HasSuffix(string(s), "&") }

func (s lambdaSrc) argc(n int) pendingKey { return pendingKey(string(s) + "n" + strconv.Itoa(n)) }

// typeStoredBlock types an unannotated def's &block from the calls of the Proc it became (`@order = block`, then
// `@order[a, b]`): when every call passed the same count, the block takes those parameters, each the join of what
// was passed there (untyped where nothing typed it), in place of decision 183's `(*untyped)`. Its result stays
// untyped. In a round, or with no calls, it stays variadic.
func (c *Compiler) typeStoredBlock(b *BlockSig) {
	if b == nil || b.Src == "" || c.infer == nil || c.round {
		return
	}
	n := -1
	for k := range maxStoredBlockArgs + 1 {
		if _, ok := c.infer.types[b.Src.argc(k)]; ok {
			if n >= 0 {
				return // calls disagree on the count: a Proc takes any, so it stays variadic
			}
			n = k
		}
	}
	if n <= 0 || n > maxStoredBlockArgs { // none, or a splat or more than this many: any count
		return
	}
	ps := make([]Type, n)
	for i := range ps {
		ps[i] = TAny{}
		if t, ok := c.infer.types[b.Src.arg(i)]; ok && !isNil(t) {
			ps[i] = c.rehome(t)
		}
	}
	b.Params, b.Rest = ps, false
}

const maxStoredBlockArgs = 8

// lambdaSrcAt is the lambda literal at offset off of f.
func lambdaSrcAt(f *File, off int) lambdaSrc { return lambdaSrc(f.posKey(off) + "#") }

// arg is the lambda's i-th parameter.
func (s lambdaSrc) arg(i int) pendingKey { return pendingKey(string(s) + strconv.Itoa(i)) }

// posKey is a source position as file:line:column:, which a rewrite that
// keeps lines (cmd/rubyspec's) keeps too, unlike a byte offset.
func (f *File) posKey(off int) string {
	path := f.path
	if path == "" {
		path = f.Name
	}
	ln := f.line(off)
	return path + ":" + strconv.Itoa(ln) + ":" + strconv.Itoa(off-f.lines[ln-1]) + ":"
}

// pendingOutcome is how a pending key stands in this compile (pendingType).
type pendingOutcome int

const (
	pendTyped pendingOutcome = iota // the last round's type
	pendRound                       // untyped: this round's calls will teach it
	pendGem                         // a gem's, no typed call reached it (decision 169): only a dynamic call can
	pendNone                        // the rounds ran and no call typed it: the caller reports or defaults it
)

// pendingType is the one state machine every inferred-from-use key goes through (parameter, yield, lambda
// argument, escaped self): the last round's type if any; else untyped during a round; else the abort that runs
// the rounds; else, once they ran, a gem's untyped or the caller's own say. The type is set only for pendTyped.
func (c *Compiler) pendingType(key pendingKey, file *File) (Type, pendingOutcome) {
	if c.infer != nil {
		if t, ok := c.infer.types[key]; ok {
			return c.rehome(t), pendTyped
		}
	}
	switch {
	case c.round:
		return nil, pendRound
	case !c.inferDone && (c.infer == nil || !c.infer.none[key]):
		panic(needInfer{})
	case c.pruneGems && c.gemFile(file):
		return nil, pendGem
	}
	return nil, pendNone
}

// resolvePending types a parameter that has no annotation (resolveMethod).
func (c *Compiler) resolvePending(m *Method, prm *Param, pp pendingParam) {
	if hookNames[m.Name] && c.gemFile(m.File) { // a gem hook's base is whichever class or module includes, extends or inherits (decision 241)
		prm.Type = TAny{}
		return
	}
	prm.Pending = pp.key
	c.notePending(pp.key)
	t, how := c.pendingType(pp.key, m.File)
	switch {
	case how == pendTyped:
		prm.Type = t
		if isNil(prm.Type) && c.pruneGems && c.gemFile(m.File) { // a gem's adapters and dynamic calls pass it what no typed call did
			prm.Type = TAny{}
		}
	case how != pendNone: // learned from this round's calls, or only a dynamic call can reach it (decision 169)
		prm.Type = TAny{}
	case m.attrWriter, m.Owner != nil && m.Owner.isTemplate(): // a template's body is never compiled: its instances' are (decision 308)
		prm.Type = TAny{} // an attr writer no call types: only a dynamic call can reach it (decision 179)
		prm.UncalledWriter = m.attrWriter // until discovery gives it its ivar's type (decision 445)
	default: // untyped, as a gem def's: only a dynamic call (send, a hook, an untyped receiver) can pass it anything (decision 183)
		c.warn(m.File, m.Node, "method %s's parameter %s is untyped: no call in the program gives it a type (`# @rbs %s: T` for static calls; decision 183)", m.Name, strings.TrimLeft(pp.name, "*"), pp.name)
		prm.Type = TAny{}
	}
}

// yieldValueUsed reports whether a yield in m's body is not a statement of its own (`out << yield`): the block's value
// is then used, so it returns untyped rather than void (decision 183). With tailUsed (gem code), so is a yield whose
// value the method returns (`@lock.synchronize { yield }` last, decision 226).
func yieldValueUsed(body parser.Node, tailUsed bool) bool {
	used := false
	valueBlocks := map[parser.Node]bool{} // blocks whose callee takes their value: `Array.new(n) { |i| yield i }` (matrix's build, decision 414)
	anyNode(body, func(x parser.Node) bool {
		if c, ok := x.(*parser.CallNode); ok && c.Block != nil && blockValueTaken[c.Name] {
			valueBlocks[c.Block] = true
		}
		return false
	})
	var walk func(parser.Node, bool, bool)
	walk = func(x parser.Node, stmt, tail bool) {
		if x == nil || opensScope(x) {
			return
		}
		if valueBlocks[x] {
			for _, ch := range x.CompactChildNodes() {
				walk(ch, true, true) // the body's last statement is the callee's value
			}
			return
		}
		switch x := x.(type) {
		case *parser.YieldNode:
			if !stmt || tail {
				used = true
			}
		case *parser.StatementsNode: // the last statement is in the body's own position: `found = (c ? yield : true)` uses it
			for i, ch := range x.Body {
				last := i == len(x.Body)-1
				walk(ch, stmt || !last, tail && last)
			}
			return
		case *parser.BlockNode, *parser.LambdaNode:
			tail = false // a block's value goes to its callee (each drops it), not to the method's caller
			stmt = true  // its body's value is the callee's business
		case *parser.WhileNode, *parser.UntilNode:
			stmt = true // a loop body's value is dropped
		}
		control := false
		switch x.(type) {
		case *parser.IfNode, *parser.UnlessNode, *parser.ElseNode, *parser.CaseNode, *parser.WhenNode, *parser.InNode,
			*parser.BeginNode, *parser.RescueNode, *parser.EnsureNode, *parser.ParenthesesNode,
			*parser.BlockNode, *parser.LambdaNode, *parser.WhileNode, *parser.UntilNode:
			control = true
		}
		for _, ch := range x.CompactChildNodes() {
			switch ch.(type) {
			case *parser.StatementsNode, *parser.ElseNode, *parser.WhenNode, *parser.InNode, *parser.RescueNode, *parser.EnsureNode:
				if control { // a branch body is where its node is: a statement `if` leaves its arms' values unused
					walk(ch, stmt, tail)
					continue
				}
			}
			walk(ch, false, tail)
		}
	}
	walk(body, true, tailUsed)
	return used
}

// blockValueTaken are the core methods that use their block's value (a constructor's `new` included: Array.new, Hash.new).
var blockValueTaken = map[string]bool{"map": true, "collect": true, "flat_map": true, "collect_concat": true, "select": true,
	"filter": true, "reject": true, "filter_map": true, "find": true, "detect": true, "find_index": true, "sort_by": true,
	"min_by": true, "max_by": true, "group_by": true, "partition": true, "sum": true, "count": true, "any?": true, "all?": true,
	"none?": true, "one?": true, "inject": true, "reduce": true, "to_h": true, "uniq": true, "take_while": true,
	"drop_while": true, "new": true, "fetch": true, "map!": true, "collect!": true, "select!": true, "reject!": true,
	"keep_if": true, "delete_if": true, "sort": true, "min": true, "max": true, "sum_by": true, "then": true, "yield_self": true,
	"fill": true, "sub": true, "gsub": true, "bsearch": true, "bsearch_index": true, "index": true, "rindex": true, "find_all": true,
	"each_with_object": false, "minmax_by": true, "chunk_while": true, "slice_when": true, "sort!": true, "sort_by!": true, "uniq!": true, "tally_by": true,
	"filter!": true, "transform_values": true, "transform_keys": true, "merge": true, "merge!": true, "update": true, "match": true}

// callValueUsed reports a call named name (with a block, if withBlock) somewhere in the program's own files whose value is used
// (not a statement of its own; a def's or block's last statement counts as used): only then is a def's tail yield or
// tail core each its value, as MRI's, and a def that only yields otherwise stays a Go iterator (decision 365).
func (c *Compiler) callValueUsed(name string, withBlock bool) bool {
	if c.valueCalls == nil {
		c.valueCalls = map[string]int{}
		for _, f := range c.userFiles {
			if !c.gemFile(f) {
				callsUsed(f.Root, c.valueCalls)
			}
		}
	}
	return c.valueCalls[name] > 1 || !withBlock && c.valueCalls[name] > 0
}

// callsUsed records each call's name in n whose value is used: 2 for a call with a block, else 1.
func callsUsed(n parser.Node, used map[string]int) {
	var walk func(parser.Node, bool, bool)
	walk = func(x parser.Node, stmt, tail bool) {
		switch x := x.(type) {
		case nil:
			return
		case *parser.CallNode:
			if !stmt || tail {
				used[x.Name] = max(used[x.Name], map[bool]int{true: 2, false: 1}[x.Block != nil])
			}
		case *parser.StatementsNode:
			for i, ch := range x.Body {
				last := i == len(x.Body)-1
				walk(ch, stmt || !last, tail && last)
			}
			return
		case *parser.DefNode, *parser.BlockNode, *parser.LambdaNode: // their value may be used: their last statement's is
			stmt, tail = true, true
		case *parser.ClassNode, *parser.ModuleNode, *parser.SingletonClassNode, *parser.WhileNode, *parser.UntilNode:
			stmt, tail = true, false
		}
		control := false
		switch x.(type) {
		case *parser.IfNode, *parser.UnlessNode, *parser.ElseNode, *parser.CaseNode, *parser.WhenNode, *parser.InNode,
			*parser.BeginNode, *parser.RescueNode, *parser.EnsureNode, *parser.ParenthesesNode, *parser.DefNode,
			*parser.BlockNode, *parser.LambdaNode, *parser.WhileNode, *parser.UntilNode, *parser.ClassNode, *parser.ModuleNode,
			*parser.SingletonClassNode, *parser.ProgramNode:
			control = true
		}
		for _, ch := range x.CompactChildNodes() {
			switch ch.(type) {
			case *parser.StatementsNode, *parser.ElseNode, *parser.WhenNode, *parser.InNode, *parser.RescueNode, *parser.EnsureNode:
				if control {
					walk(ch, stmt, tail)
					continue
				}
			}
			walk(ch, false, tail)
		}
	}
	walk(n, true, false)
}

// splatYields reports a body with a yield that passes a splat, `yield(*xs)`: its block takes any count.
func splatYields(body parser.Node) bool {
	found := false
	walkNode(body, func(x parser.Node) (bool, bool) {
		if opensScope(x) {
			return false, false
		}
		if y, ok := x.(*parser.YieldNode); ok && y.Arguments != nil && slices.ContainsFunc(y.Arguments.Arguments, isSplat) {
			found = true
		}
		return false, true
	})
	return found
}

// yieldShape is the block an unannotated def yields to: how many values
// each yield passes (all must agree), and whether block_given? makes it
// optional. ok is false for a def that does not yield, or names its block.
func (c *Compiler) yieldShape(m *Method) (n int, optional, ok bool) {
	if m.Node == nil || m.Node.Body == nil || m.BlockParam != "" {
		return 0, false, false
	}
	n = -1
	agree := true
	if !c.gemFile(m.File) && splatYields(m.Node.Body) { // a gem's `yield(*args)` spreads at run time (decision 250)
		n = 1 // the *rest (decision 363)
		walkNode(m.Node.Body, func(x parser.Node) (bool, bool) {
			if c, ok := x.(*parser.CallNode); ok && c.Receiver == nil && c.Name == "block_given?" {
				optional = true
			}
			return false, !opensScope(x)
		})
		return n, optional, true
	}
	walkNode(m.Node.Body, func(x parser.Node) (bool, bool) {
		if opensScope(x) {
			return false, false
		}
		switch x := x.(type) {
		case *parser.YieldNode:
			k := 0
			if x.Arguments != nil {
				k = len(x.Arguments.Arguments)
			}
			if n >= 0 && k != n && agree {
				agree = false // reported at this yield when the body compiles, not at link, where it would stop every round
				m.yieldDisagree = fmt.Sprintf("method %s yields %d values here and %d before; annotate its block (`#: (...) { (T, ...) -> void } -> R`)", m.Name, k, n)
				m.yieldDisagreeAt = x
			}
			n = k
		case *parser.CallNode:
			if x.Receiver == nil && x.Name == "block_given?" {
				optional = true
			}
		}
		return false, true
	})
	return n, optional, n >= 0 && agree
}

// includeFromEach types `include Enumerable` written without type args
// from the includer's own `each`: what it yields is the element type, as
// MRI's Enumerable has whatever each yields.
func (c *Compiler) includeFromEach() {
	for _, cls := range c.classList {
		for i := range cls.Includes {
			inc := &cls.Includes[i]
			if !inc.fromEach {
				continue
			}
			each := cls.Methods["each"]
			c.resolveMethod(each)
			if neverYields(each) { // nothing is ever an element: nil types them, unobservably (decision 149)
				inc.Args = make([]Type, len(inc.Mod.TypeParams))
				for i := range inc.Args {
					inc.Args[i] = TNil{}
				}
				each.Block = &BlockSig{Params: slices.Clone(inc.Args), Ret: TVoid{}}
				resetMsets(cls)
				continue
			}
			if each.Block == nil || len(each.Block.Params) != len(inc.Mod.TypeParams) {
				if c.round {
					continue // untyped this round; the final compile reports it
				}
				c.errorAt(inc.srcPos, "include %s needs %d type args (`include %s #[...]`): %s#each does not yield %d values to infer them from", inc.Mod.RubyName, len(inc.Mod.TypeParams), inc.Mod.RubyName, cls.RubyName, len(inc.Mod.TypeParams))
			}
			inc.Args = slices.Clone(each.Block.Params)
			resetMsets(cls)
		}
	}
}

// resetMsets drops method sets built while an include held placeholder args.
func resetMsets(k *Class) {
	k.msetCache, k.msetIndex = nil, nil
	for _, s := range k.Subclasses {
		resetMsets(s)
	}
}

// neverYields is an unannotated each with no block that yields nothing.
func neverYields(m *Method) bool {
	if m.Block != nil || m.BlockParam != "" || m.Node == nil {
		return false
	}
	return !anyNode(m.Node.Body, func(n parser.Node) bool {
		switch n := n.(type) {
		case *parser.YieldNode, *parser.BlockArgumentNode:
			return true
		case *parser.CallNode:
			return n.Name == "block_given?" || n.Name == "to_enum" || n.Name == "enum_for"
		}
		return false
	})
}

// typesInclude reports an `each` whose class's include takes its type args
// from it: it has its own signature, not the module's.
func typesInclude(m *Method) bool {
	if m.Name != "each" || m.Owner == nil {
		return false
	}
	for _, inc := range m.Owner.Includes {
		if inc.fromEach {
			return true
		}
	}
	return false
}

// lambdaParams types a lambda's parameters from its calls, when nothing
// gives it an expected type: the last round's types, untyped during a
// round, else the error an annotation would have prevented.
func (f *fctx) lambdaParams(n parser.Node, names blockParams, body parser.Node) ([]Type, lambdaSrc) {
	src := lambdaSrcAt(f.f, n.GetLocation().StartOffset)
	ps := make([]Type, len(names))
	for i := range ps {
		f.c.notePending(src.arg(i))
		t, how := f.c.pendingType(src.arg(i), f.f)
		switch {
		case how == pendTyped:
			ps[i] = t
		case how != pendNone: // untyped this round, or a gem's lambda no call types (decision 169)
			ps[i] = TAny{}
		case !readsParam(body, names[i].Name):
			ps[i] = TAny{} // a parameter the body never reads loses nothing untyped (decision 179)
		default: // untyped, its calls dynamic, as a gem lambda's (decision 180)
			if f.pass == 2 && i == 0 {
				f.warn(n, "a lambda's parameters are untyped: no call in the program gives them types (annotate `#: ^(T) -> R` for static calls; decision 180)")
			}
			ps[i] = TAny{}
		}
	}
	return ps, src
}

func readsParam(body parser.Node, name string) bool {
	return anyNode(body, func(n parser.Node) bool {
		r, ok := n.(*parser.LocalVariableReadNode)
		return ok && r.Name == name
	})
}

// sameParamShape reports whether def m's parameter list fits parent's
// signature: positional count, rest and keyword names. An unannotated
// override inherits the signature only then.
func (c *Compiler) sameParamShape(m, parent *Method) bool {
	if m.Node == nil || m.Kind != kindDef {
		return true
	}
	// read off the node, not defParams: that rejects a &block before m.Block is known
	own, ownRest := 0, false
	if ps := m.Node.Parameters; ps != nil {
		own, ownRest = len(ps.Requireds)+len(ps.Optionals)+len(ps.Posts), ps.Rest != nil
	}
	npos, hasRest, hasKwRest := 0, false, false
	var kws []string
	for _, p := range parent.Params {
		switch {
		case p.Keyword && !p.KwRest:
			kws = append(kws, p.Name)
		case p.Keyword:
			hasKwRest = true
		case p.Rest:
			hasRest = true
		default:
			npos++
		}
	}
	var ownKws []string
	ownKwRest := false
	for _, k := range c.defKeywords(m, m.Node.Parameters) {
		if !k.rest {
			ownKws = append(ownKws, k.name)
		} else {
			ownKwRest = true
		}
	}
	slices.Sort(kws)
	slices.Sort(ownKws)
	// a `**` too: drb's DRbProtocol#open(uri, config, first = true) is not Kernel#open(path, mode, perm, **opts) (decision 416)
	return npos == own && hasRest == ownRest && hasKwRest == ownKwRest && slices.Equal(kws, ownKws)
}

// rehome rebuilds a type from an earlier compile with this one's classes, by name.
func (c *Compiler) rehome(t Type) Type {
	switch t := t.(type) {
	case TClass:
		var cls *Class
		if t.C.metaOf != nil {
			if o := c.classes[t.C.metaOf.RubyName]; o != nil {
				cls = o.meta
			}
		} else {
			cls = c.classes[t.C.RubyName]
		}
		if t.C.template != nil { // an instance by its key: absent until this compile creates it (decision 308)
			if cls = c.insts[instKey(t.C.template, t.C.instArgs)]; cls == nil && c.classes[t.C.RubyName] != nil {
				args := make([]Type, len(t.C.instArgs))
				for i, a := range t.C.instArgs {
					args[i] = c.rehome(a)
				}
				return c.classType(c.classes[t.C.RubyName], args, srcPos{})
			}
		}
		if cls == nil {
			return TAny{}
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.rehome(a)
		}
		return TClass{C: cls, Args: args}
	case TOpt:
		return TOpt{Elem: c.rehome(t.Elem)}
	case TTuple:
		els := make([]Type, len(t.Elems))
		for i, e := range t.Elems {
			els[i] = c.rehome(e)
		}
		return TTuple{Elems: els}
	case TFunc:
		ps := make([]Type, len(t.Params))
		for i, p := range t.Params {
			ps[i] = c.rehome(p)
		}
		t.Params, t.Ret, t.Self = ps, c.rehome(t.Ret), c.rehome(t.Self)
		return t
	case TUnion:
		ms := make([]Type, 0, len(t.Members))
		for _, m := range t.Members {
			if r := c.rehome(m); !isAny(r) { // a member this compile lacks is left out, not an untyped union (its uses are reported at their calls)
				ms = append(ms, r)
			}
		}
		if !slices.ContainsFunc(ms, func(m Type) bool { return !isNil(m) }) {
			return TAny{} // every member's class is gone (nil alone is no type for it): untyped, as one class would be (unionOf() is nil)
		}
		return unionOf(ms...)
	case TAny, TNil, TVar, TVoid: // no classes inside
	}
	return t
}

// notePending records, during a round, a parameter key waiting on its uses.
func (c *Compiler) notePending(key pendingKey) {
	if c.round {
		if c.pendingSeen == nil {
			c.pendingSeen = map[pendingKey]bool{}
		}
		c.pendingSeen[key] = true
	}
}

// noteYield records what a yield passes to a block parameter typed from the yields.
func (f *fctx) noteYield(i int, n parser.Node, t Type) {
	if i < len(f.blockSig.Pending) && f.blockSig.Pending[i] != "" {
		f.noteUse(Param{Pending: f.blockSig.Pending[i]}, n, t)
	}
}

// noteUse records an argument for a pending parameter, during a round.
func (f *fctx) noteUse(p Param, n parser.Node, t Type) {
	if p.Pending == "" || f.c.uses == nil || n == nil {
		return
	}
	if f.superTarget != nil && slices.ContainsFunc(f.superTarget.Params, func(q Param) bool { return q.Pending == p.Pending }) {
		return
	}
	if en, ok := n.(*exprNode); ok && en.use != nil { // one use per source argument, not a member per arm: the param would flip between them (decision 150)
		t = en.use
	}
	f.c.uses[p.Pending] = append(f.c.uses[p.Pending], paramUse{typ: t, file: f.f, off: n.GetLocation().StartOffset})
}

// pendingWant is the expected type for an argument: none for a pending parameter in a round, so the argument types itself.
func (f *fctx) pendingWant(p Param, t Type) Type {
	if p.Pending != "" && f.c.round && isAny(p.Type) {
		return nil
	}
	return t
}

// collectUses dry-runs every user body for the arguments it passes.
func (c *Compiler) collectUses() {
	c.uses = map[pendingKey][]paramUse{}
	c.loadCode = map[*File]string{}
	for _, m := range c.userMethods() {
		err := catchCompileError(func() {
			c.inferRet(m) // the round's pass typed it already; this retries one that failed there
			if m.Owner == nil {
				c.emitTopDef(m)
			} else {
				c.emitMethod(m)
			}
		})
		c.debugInfer(err)
		if err != nil {
			c.demoteUnions(m)
		}
		c.out.Reset()
		for _, p := range m.Params {
			if p.Pending == "" || p.Default == nil {
				continue
			}
			// a default passed by no call still types its parameter: its value, as a body of the method's own
			var ts []Type
			c.debugInfer(catchCompileError(func() {
				f := c.newFctx(m.File, m.Owner, m)
				f.retVar = "ret_"
				f.genBody(&parser.StatementsNode{Body: []parser.Node{p.Default}}, c.paramLocals(m), tail{kind: tailReturn, types: &ts}, nil)
			}))
			if len(ts) > 0 {
				t := ts[len(ts)-1]
				if hc, ok := t.(TClass); ok && c.gemFile(m.File) && hc.C.RubyName == "Hash" && len(hc.Args) == 2 {
					if _, lit := p.Default.(*parser.HashNode); lit { // kramdown's `opts = {indent: 0}`: an options bag callers add any value to (decision 330)
						t = TClass{C: hc.C, Args: []Type{hc.Args[0], TAny{}}}
					}
				}
				if k := classOf(stripOpt(t)); k != nil && k.constSingleton && c.gemFile(m.File) {
					// a sentinel object with its own methods (rubocop's `named: ANY`, whose `==` answers true): callers pass
					// values of other classes, so it types nothing (decision 444)
					t = TAny{}
				}
				c.uses[p.Pending] = append(c.uses[p.Pending], paramUse{typ: t, file: m.File, off: p.Default.GetLocation().StartOffset, deflt: true})
			}
		}
	}
	_, all := c.mainBodies()
	for _, b := range all {
		c.debugInfer(catchCompileError(func() {
			f := c.newFctx(b.f, nil, nil)
			f.retVar = ""
			f.genBody(&parser.StatementsNode{Body: b.stmts}, nil, tail{}, nil)
		}))
	}
}

// demoteUnions marks m's parameters typed as unions when m's body did not
// compile: their uses join as before unions from the next round, a use
// that does not join being reported at its call rather than breaking the
// method every call shares (decision 150).
func (c *Compiler) demoteUnions(m *Method) {
	if c.infer == nil {
		return
	}
	for _, p := range m.Params {
		if p.Pending == "" || !holdsUnion(p.Type) {
			continue
		}
		if c.infer.noUnion == nil {
			c.infer.noUnion = map[pendingKey]bool{}
		}
		c.infer.noUnion[p.Pending] = true
	}
}

// holdsUnion reports whether t is or contains a union.
func holdsUnion(t Type) bool {
	switch t := t.(type) {
	case TUnion:
		return true
	case TOpt:
		return holdsUnion(t.Elem)
	case TClass:
		return slices.ContainsFunc(t.Args, holdsUnion)
	case TTuple:
		return slices.ContainsFunc(t.Elems, holdsUnion)
	case TFunc:
		return slices.ContainsFunc(t.Params, holdsUnion) || holdsUnion(t.Ret)
	case TAny, TNil, TVar, TVoid:
	}
	return false
}

// debugInfer prints what a round's dry run could not compile, under RUGAROO_INFER_DEBUG.
func (c *Compiler) debugInfer(err *compileError) {
	if err != nil && c.env.InferDebug {
		fmt.Fprintln(os.Stderr, "rugaroo: infer round:", err.Error())
	}
}

func (c *Compiler) userMethods() []*Method {
	var out []*Method
	for _, m := range c.topDefList {
		if !m.File.isPrelude() && m.Kind == kindDef {
			out = append(out, m)
		}
	}
	for _, cls := range c.classList {
		if cls.isTemplate() {
			continue
		}
		for _, m := range cls.MethodList {
			if !m.File.isPrelude() && m.Kind == kindDef {
				out = append(out, m)
			}
		}
	}
	return out
}

// updateInference joins each pending parameter's uses, in source order: a
// use that does not join the ones before it is left out, so the final
// compile reports it at its call.
func (c *Compiler) updateInference(inf *inference) {
	order := map[*File]int{}
	for i, f := range c.files {
		order[f] = i
	}
	next, from := map[pendingKey]Type{}, map[pendingKey]string{}
	var bets []pendingKey // typed parameters a gem call also passes an untyped value
	for _, key := range slices.Sorted(maps.Keys(c.uses)) {
		uses := c.lastPassUses(key, order)
		if !slices.ContainsFunc(uses, func(u paramUse) bool { return !isNil(u.typ) && !holdsAny(u.typ) || !u.deflt && isNil(u.typ) }) {
			continue // nothing but its `= nil` default types it: no call does, so it is untyped (decision 361)
		}
		if slices.ContainsFunc(uses, func(u paramUse) bool { return u.dyn }) && !slices.ContainsFunc(uses, func(u paramUse) bool { return !u.dyn && realType(u.typ) }) {
			// dynamic calls by its name (maybe meant for another class's method) widen a parameter but never type it alone:
			// untyped until a static call carries a type (decision 331). ProcessedSource's `parser.parse(@buffer)` reaches
			// every `parse` with a Buffer; rubocop-ast's NodePattern::Parser#parse(source) takes a String from
			// NodePattern#initialize, which is untyped this round
			continue
		}
		if inf.anyUse[key] {
			next[key] = TAny{}
			continue
		}
		if c.gemParams[key] && slices.ContainsFunc(uses, func(u paramUse) bool { return !u.dyn && !u.deflt && isAny(u.typ) && c.gemFile(u.file) }) {
			bets = append(bets, key)
		}
		if t, at, ok := c.gemUseType(key, uses); ok {
			next[key], from[key] = t, at
			continue
		}
		if t, at := joinUses(uses, inf.noUnion[key]); t != nil {
			if c.gemUsesDisagree(uses, t, c.gemParams[key]) { // a gem's calls passing kinds no type joins: untyped, not the first kind (decision 244)
				t = TAny{}
			}
			if c.anyGemUseWidens(t, uses) {
				t = TAny{}
			}
			next[key], from[key] = t, at
		}
	}
	diff := c.roundDiff(inf, next)
	if c.env.Timing {
		fmt.Fprintf(os.Stderr, "rugaroo: round typed %d parameters, %d changed, %d with an untyped gem argument\n", len(next), diff, len(bets))
	}
	settled := !inf.changed || diff*50 <= len(next)
	c.settleRound(inf, settled, bets, next, from)
	inf.types, inf.from = next, from
	inf.none = map[pendingKey]bool{}
	for k := range c.pendingSeen {
		if _, ok := next[k]; !ok {
			inf.none[k] = true
		}
	}
	c.stopUnjustifiedDrops(inf, settled)
	if c.env.InferDebug {
		for _, k := range slices.Sorted(maps.Keys(next)) {
			fmt.Fprintf(os.Stderr, "rugaroo: inferred %s: %s\n", k, next[k])
		}
	}
}

// roundDiff counts the parameters next types otherwise than the previous round did, and sets inf.changed.
func (c *Compiler) roundDiff(inf *inference, next map[pendingKey]Type) int {
	inf.changed = len(next) != len(inf.types)
	diff := 0
	for k, t := range next {
		if old, ok := inf.types[k]; !ok || fmt.Sprint(old) != fmt.Sprint(t) {
			inf.changed = true
			diff++
			if c.env.Timing && diff <= 3 {
				fmt.Fprintf(os.Stderr, "rugaroo: round changed %v: %v -> %v\n", k, old, t)
			}
		}
	}
	return diff
}

// lastPassUses are key's uses in source order, each site once with the type of the last pass that reached it (a body
// generates more than once).
func (c *Compiler) lastPassUses(key pendingKey, order map[*File]int) []paramUse {
	seen := map[[2]int]int{}
	var uses []paramUse
	for _, u := range c.uses[key] {
		k := [2]int{order[u.file], u.off}
		if i, ok := seen[k]; ok {
			uses[i] = u
			continue
		}
		seen[k] = len(uses)
		uses = append(uses, u)
	}
	slices.SortFunc(uses, func(a, b paramUse) int {
		return cmp.Or(cmp.Compare(order[a.file], order[b.file]), cmp.Compare(a.off, b.off))
	})
	return uses
}

// settleRound counts settled rounds (a round that changed at most 2% of the parameters) and, in one, makes each gem
// parameter an untyped gem argument reaches untyped. A large gem program's last few parameters flip between rounds until
// the cap: an untyped argument is now untyped for good, not a parameter still waiting for its type. A typed parameter
// would assert the value's class at the call, and raise where MRI runs (parser's StaticEnvironment#declared?(name) typed
// Symbol by the parser, called by the lexer with an untyped String it calls to_sym on). The rounds go on (decision 444).
func (c *Compiler) settleRound(inf *inference, settled bool, bets []pendingKey, next map[pendingKey]Type, from map[pendingKey]string) {
	if !settled {
		inf.settled = 0
		return
	}
	inf.settled++
	for _, key := range bets {
		if t := next[key]; t != nil && !isAny(t) && c.goType(t) != "any" {
			if inf.anyUse == nil {
				inf.anyUse = map[pendingKey]bool{}
			}
			inf.anyUse[key], next[key], inf.changed = true, TAny{}, true
			delete(from, key)
		}
	}
}

// realType is a use's type that says something: not nil, void, Ruby nil or untyped.
func realType(t Type) bool { return t != nil && !isVoid(t) && !isNil(t) && !isAny(stripOpt(t)) }

// stopUnjustifiedDrops: rounds that converged with a parameter still untyped whose method's untyped returns a round
// dropped never justified the drop, so the next rounds join that method's returns honestly (decision 383).
//
// settled stands for converged on a large gem program, whose rounds never do (decision 444): rubocop's
// `split_comment(comment)` answers false or an untyped `match.captures`, and stayed Boolean.
func (c *Compiler) stopUnjustifiedDrops(inf *inference, settled bool) {
	if !settled {
		return
	}
	for k := range c.roundDrops {
		if inf.none[k] || isAny(inf.types[k]) { // nothing typed it, or its uses joined to untyped (a gem's disagreeing calls, decision 244)
			if inf.noDrop == nil {
				inf.noDrop = map[pendingKey]bool{}
			}
			inf.noDrop[k], inf.changed = true, true
		}
	}
}

// joinUses joins a parameter's uses in source order, returning the type and
// where its first use is. Object, BasicObject and modules are Go any: they
// count only when nothing concrete is passed, or one Object.new would
// untype the rest. noUnion leaves out a use that would make a union.
func joinUses(uses []paramUse, noUnion bool) (Type, string) {
	for _, abstract := range []bool{false, true} {
		var t Type
		at := ""
		for _, u := range uses {
			if _, void := u.typ.(TVoid); u.typ == nil || void || holdsAny(u.typ) || mentionsVar(u.typ) || isAbstract(stripOpt(u.typ)) != abstract {
				continue // untyped this round (another pending parameter's), or no value
			}
			if t == nil {
				t, at = u.typ, u.at().String()
				continue
			}
			j, ok := join(t, u.typ)
			if ok && noUnion && holdsUnion(j) && !holdsUnion(t) && !holdsUnion(u.typ) {
				ok = false // demoted: a use with no common class is left out, as before unions
			}
			if ok && !isAny(j) && (abstract || !isAbstract(stripOpt(j))) {
				t = j
			}
		}
		if t != nil {
			return t, at
		}
	}
	return nil, ""
}

// coercePending is coerceArg's check for a parameter typed from use: a
// mismatch names the call its type came from.
func (f *fctx) coercePending(p Param, code func() string) string {
	if p.Pending == "" || f.c.infer == nil || f.c.round {
		return code()
	}
	var out string
	err := catchCompileError(func() { out = code() })
	if err != nil {
		if from := f.c.infer.from[p.Pending]; from != "" {
			err.msg += fmt.Sprintf("; parameter %s takes its type from the call at %s (decision 146): annotate it to take both", strings.TrimLeft(p.Name, "*"), from)
		}
		panic(*err)
	}
	return out
}

// gemMethodsNamed lists the gem methods (instance and class) named name, built once per compile.
func (c *Compiler) gemMethodsNamed(name string) []*Method {
	if c.gemByName == nil {
		c.gemByName = map[string][]*Method{}
		for _, cls := range c.classList {
			for _, m := range cls.MethodList {
				if m.Node != nil && m.File != nil && c.gemFile(m.File) && m.Owner == cls {
					c.gemByName[m.Name] = append(c.gemByName[m.Name], m)
				}
			}
		}
	}
	return c.gemByName[name]
}
