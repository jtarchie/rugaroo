package compiler

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// strAppendNames are the String appends rugaroo compiles as a rebinding or a buffer append: `<<`, `concat`, and Ruby 3.4's
// `append_as_bytes` (each argument's bytes, an Integer's low byte; prism's translation lexer builds unescaped strings with it).
var strAppendNames = setOf("<<", "concat", "append_as_bytes")

// readOnlyStrMethods neither keep, return nor change their receiver, so calling one is not an alias (decision 382).
var readOnlyStrMethods = setOf("size", "length", "bytesize", "empty?", "include?", "start_with?", "end_with?",
	"getbyte", "[]", "byteslice", "slice", "==", "!=", "eql?", "hash", "=~", "match", "match?", "index", "rindex",
	"byteindex", "count", "sum", "split", "lines", "chars", "bytes", "each_char", "each_byte", "each_line",
	"each_grapheme_cluster", "<=>", "<", ">", "<=", ">=", "casecmp", "casecmp?", "!", "+", "*", "%", "ord", "to_i",
	"to_f", "to_sym", "intern", "hex", "oct", "unpack", "unpack1", "upcase", "downcase", "capitalize", "swapcase",
	"strip", "lstrip", "rstrip", "chomp", "chop", "chr", "reverse", "center", "ljust", "rjust", "tr", "delete",
	"squeeze", "sub", "gsub", "scan", "inspect", "dump", "dup", "clone", "encoding", "valid_encoding?",
	"ascii_only?", "frozen?", "nil?", "is_a?", "kind_of?", "instance_of?", "respond_to?", "class", "succ", "next",
	"upto", "between?", "partition", "rpartition", "format", "crypt", "unicode_normalize", "encode", "scrub", "b",
	"codepoints", "grapheme_clusters")

// strArgCalls take a String argument without keeping it or returning it.
var strArgCalls = setOf("puts", "print", "+", "==", "!=", "eql?", "include?", "start_with?", "end_with?", "<=>",
	"index", "rindex", "casecmp", "casecmp?", "=~", "match", "match?", "count", "delete", "split", "write")

// discardBlocks are core iterators that ignore their block's value.
var discardBlocks = setOf("each", "each_with_index", "each_with_object", "each_char", "each_byte", "each_line",
	"each_key", "each_value", "each_pair", "each_slice", "each_cons", "each_index", "reverse_each", "times", "upto",
	"downto", "step", "loop", "tap", "each_entry", "each_codepoint")

// laterBlocks are calls whose block may run after the call returns: an append inside one is not provable.
var laterBlocks = setOf("lambda", "proc", "new", "start", "fork", "define_method", "define_singleton_method",
	"at_exit", "trap", "to_proc", "on", "subscribe", "add_observer")

// freshStrCalls return a new String, never their receiver or an argument.
var freshStrCalls = setOf("dup", "clone", "+", "*", "%", "join", "format", "sprintf", "upcase", "downcase",
	"capitalize", "swapcase", "strip", "lstrip", "rstrip", "chomp", "chop", "center", "ljust", "rjust", "reverse",
	"tr", "delete", "squeeze", "sub", "gsub", "inspect", "b")

// scalarStrMethods return no String, so none shares the receiver's bytes: the only reads a setbyte target allows before its last write (decision 395).
// ponytail: by name; a per-method "always copies in Go" table would also allow upcase, center, ... before a write.
var scalarStrMethods = setOf("size", "length", "bytesize", "empty?", "getbyte", "bytes", "codepoints", "==", "!=", "eql?",
	"hash", "<=>", "<", ">", "<=", ">=", "casecmp", "casecmp?", "include?", "start_with?", "end_with?", "index", "rindex",
	"byteindex", "count", "match?", "ascii_only?", "valid_encoding?", "encoding", "frozen?", "nil?", "is_a?", "kind_of?",
	"instance_of?", "respond_to?", "class", "to_i", "to_f", "hex", "oct", "ord", "sum", "!")

// scalarArgCalls take a String argument and keep none of its bytes.
var scalarArgCalls = setOf("==", "!=", "eql?", "include?", "start_with?", "end_with?", "<=>", "index", "rindex",
	"casecmp", "casecmp?", "match?", "count")

// selfFreeCalls are receiverless calls that cannot reach an ivar through self: initialize's setbyte proof allows only these (decision 395).
var selfFreeCalls = setOf("puts", "print", "p", "pp", "warn", "raise", "fail", "format", "sprintf", "rand", "srand",
	"sleep", "Integer", "Float", "loop", "require", "require_relative")

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

var frozenLiteralComment = regexp.MustCompile(`(?m)^#.*frozen_string_literal:\s*true`)

// appendProof walks the Ruby scope of one local for what could make its String shared with anything else.
type appendProof struct {
	f      *fctx
	name   string
	root   parser.Node      // the scope: a method or program body, or the block whose local it is
	rootFn *parser.CallNode // the call a block root belongs to
	frozen bool             // the file freezes its literals
	ivar   bool             // name is an ivar of initialize's self (decision 395)
	strict bool             // the String is setbyte'd: no read may share its bytes before the seal (decision 395)
	out    bool             // an out parameter: its caller appends after the method returns, so nothing may keep it (decision 405)
	depth  int              // blocks entered below root
	loops  []int            // end offsets of the loops and blocks enclosing the position, outermost first
	later  int              // > 0 inside a block that may run after its call
	seal   int              // past the last append (the end of its outermost loop)
	leaks  []appendLeak
	bad    string
}

type appendLeak struct {
	off int
	why string
}

// appendWhy is "" when the local name in this scope is proven unaliased wherever it is appended to, else why not.
func (f *fctx) appendWhy(name string) string {
	key := f.localKey(name)
	if why, ok := f.appendProofs[key]; ok {
		return why
	}
	if f.appendProofs == nil {
		f.appendProofs = map[localKey]string{}
	}
	// The proof probes the type of calls it walks (outCallArg), and probing `s.concat` asks for this same proof again:
	// answer that nested ask "fine", since a probe's output is discarded and the verdict below is the one the real
	// codegen sees. A panic out of the proof must not leave the stand-in cached.
	f.appendProofs[key] = ""
	done := false
	defer func() {
		if !done {
			delete(f.appendProofs, key)
		}
	}()
	why := f.proveAppend(key)
	f.appendProofs[key] = why
	done = true
	return why
}

func (f *fctx) proveAppend(key localKey) string {
	out := key.scope == methodScope && f.m != nil && slices.ContainsFunc(f.m.Params, func(p Param) bool { return p.Out && p.Name == key.name })
	if li := f.locals[key]; li != nil && (li.param || li.noHoist) && !out {
		return fmt.Sprintf("`%s` is a parameter, so the caller's String would see the append", key.name)
	}
	p := &appendProof{f: f, name: key.name, root: f.scopeRoot, frozen: frozenLiteralComment.Match(f.f.Src), out: out}
	if p.root == nil {
		return "rugaroo cannot see the scope of `" + key.name + "`"
	}
	p.strict = setbyteOn(p.root, key.name)
	if key.scope != methodScope {
		walkNode(f.scopeRoot, func(n parser.Node) (bool, bool) {
			if c, ok := n.(*parser.CallNode); ok && c.Block != nil && c.Block.GetLocation().StartOffset == key.scope {
				p.root, p.rootFn = c.Block, c
				return true, false
			}
			return false, !opensScope(n)
		})
		if p.rootFn == nil {
			return "`" + key.name + "` belongs to a block rugaroo cannot follow"
		}
		bn, ok := p.root.(*parser.BlockNode)
		if !ok || bn.Body == nil {
			return "`" + key.name + "` belongs to a block rugaroo cannot follow"
		}
		p.walk(bn.Body, []parser.Node{bn})
	} else {
		p.walk(p.root, nil)
	}
	return p.verdict()
}

func (p *appendProof) verdict() string {
	if p.bad != "" {
		return p.bad
	}
	last := " before its last append"
	if p.strict {
		last = " before its last change"
	}
	for _, l := range p.leaks {
		if p.out {
			return l.why + ", while the caller's String it appends to is still in use (decision 405)"
		}
		if l.off < p.seal {
			return l.why + last
		}
	}
	return ""
}

// setbyteOn: root calls setbyte on the local or ivar name.
func setbyteOn(root parser.Node, name string) bool {
	return anyNode(root, func(x parser.Node) bool {
		c, ok := x.(*parser.CallNode)
		if !ok || c.Name != "setbyte" {
			return false
		}
		switch r := c.Receiver.(type) {
		case *parser.LocalVariableReadNode:
			return r.Name == name
		case *parser.InstanceVariableReadNode:
			return r.Name == name
		}
		return false
	})
}

// ivarSetbyteWhy is "" when initialize may setbyte the ivar in place: first set fresh, and neither it nor self escapes before its last setbyte (decision 395).
func (f *fctx) ivarSetbyteWhy(name string) string {
	key := localKey{methodScope, name}
	if why, ok := f.appendProofs[key]; ok {
		return why
	}
	why := f.proveIvarSetbyte(name)
	if f.appendProofs == nil {
		f.appendProofs = map[localKey]string{}
	}
	f.appendProofs[key] = why
	return why
}

func (f *fctx) proveIvarSetbyte(name string) string {
	if f.m == nil || f.m.Name != "initialize" || f.m.Node == nil || f.scopeRoot == nil || f.scopeRoot != f.m.Node.Body {
		return "`" + name + "` is changed outside initialize"
	}
	st, ok := f.scopeRoot.(*parser.StatementsNode)
	if !ok {
		return "rugaroo cannot follow initialize's body"
	}
	p := &appendProof{f: f, name: name, root: st, frozen: frozenLiteralComment.Match(f.f.Src), ivar: true, strict: true}
	for _, s := range st.Body {
		if !anyNode(s, func(x parser.Node) bool { return ivarNamed(x) == name }) {
			continue
		}
		if w, ok := s.(*parser.InstanceVariableWriteNode); !ok || w.Name != name || !p.fresh(w.Value) {
			return fmt.Sprintf("`%s` is not first assigned a new String in initialize (line %d)", name, p.line(s))
		}
		break
	}
	p.walk(st, nil)
	return p.verdict()
}

// ivarNamed is the ivar a read or write node names, or "".
func ivarNamed(n parser.Node) string {
	switch t := n.(type) {
	case *parser.InstanceVariableReadNode:
		return t.Name
	case *parser.InstanceVariableWriteNode:
		return t.Name
	case *parser.InstanceVariableOrWriteNode:
		return t.Name
	case *parser.InstanceVariableAndWriteNode:
		return t.Name
	case *parser.InstanceVariableOperatorWriteNode:
		return t.Name
	case *parser.InstanceVariableTargetNode:
		return t.Name
	}
	return ""
}

func (p *appendProof) line(n parser.Node) int { return p.f.f.line(n.GetLocation().StartOffset) }

func (p *appendProof) fail(n parser.Node, format string, args ...any) {
	if p.bad == "" {
		p.bad = fmt.Sprintf(format, args...) + fmt.Sprintf(" (line %d)", p.line(n))
	}
}

func (p *appendProof) leak(n parser.Node, why string) {
	p.leaks = append(p.leaks, appendLeak{off: n.GetLocation().StartOffset, why: fmt.Sprintf("%s on line %d", why, p.line(n))})
}

func (p *appendProof) mine(name string, depth uint32) bool {
	return name == p.name && int(depth) == p.depth
}

func (p *appendProof) walk(n parser.Node, stack []parser.Node) {
	if n == nil || opensScope(n) {
		return
	}
	if p.ivar {
		p.walkIvar(n, stack)
		return
	}
	switch t := n.(type) {
	case *parser.LocalVariableReadNode:
		if p.mine(t.Name, t.Depth) {
			p.read(t, stack)
		}
		return
	case *parser.LocalVariableWriteNode:
		if p.mine(t.Name, t.Depth) && !p.fresh(t.Value) {
			p.fail(t, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.LocalVariableOrWriteNode:
		if p.mine(t.Name, t.Depth) && !p.fresh(t.Value) {
			p.fail(t, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.LocalVariableAndWriteNode:
		if p.mine(t.Name, t.Depth) && !p.fresh(t.Value) {
			p.fail(t, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.LocalVariableOperatorWriteNode:
		if p.mine(t.Name, t.Depth) && t.BinaryOperator != "+" && t.BinaryOperator != "*" {
			p.fail(t, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.LocalVariableTargetNode:
		if p.mine(t.Name, t.Depth) {
			p.fail(t, "`%s` is bound to a String that something else may hold", p.name)
		}
		return
	}
	p.enter(n, stack)
}

// walkIvar is walk for initialize's ivar: self, its methods and its other ivars are how the String could escape.
func (p *appendProof) walkIvar(n parser.Node, stack []parser.Node) {
	switch t := n.(type) {
	case *parser.InstanceVariableReadNode:
		if t.Name == p.name {
			p.read(t, stack)
		}
		return
	case *parser.InstanceVariableWriteNode:
		if t.Name == p.name && !p.fresh(t.Value) {
			p.fail(t, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.InstanceVariableOrWriteNode, *parser.InstanceVariableAndWriteNode, *parser.InstanceVariableOperatorWriteNode,
		*parser.InstanceVariableTargetNode:
		if ivarNamed(n) == p.name {
			p.fail(n, "`%s` is assigned a String that something else may hold", p.name)
		}
	case *parser.SelfNode:
		p.leak(n, "self is used")
	case *parser.SuperNode, *parser.ForwardingSuperNode:
		p.leak(n, "super is called")
	case *parser.YieldNode:
		p.leak(n, "initialize yields")
	case *parser.CallNode:
		if t.Receiver == nil && !selfFreeCalls[t.Name] {
			p.leak(n, fmt.Sprintf("`%s` is called on self", t.Name))
		}
	}
	p.enter(n, stack)
}

// enter tracks the loops and blocks around n, then walks its children.
func (p *appendProof) enter(n parser.Node, stack []parser.Node) {
	switch t := n.(type) {
	case *parser.WhileNode, *parser.UntilNode, *parser.ForNode:
		p.loops = append(p.loops, endOf(n))
		defer func() { p.loops = p.loops[:len(p.loops)-1] }()
	case *parser.BeginNode:
		if t.RescueClause != nil && anyNode(t.RescueClause, func(x parser.Node) bool { _, ok := x.(*parser.RetryNode); return ok }) { // retry runs the body again
			p.loops = append(p.loops, endOf(n))
			defer func() { p.loops = p.loops[:len(p.loops)-1] }()
		}
	case *parser.BlockNode, *parser.LambdaNode:
		p.depth++
		p.loops = append(p.loops, endOf(n))
		later := true
		if c, ok := parentCall(stack, n); ok {
			later = p.runsLater(c)
		}
		if later {
			p.later++
		}
		defer func() {
			p.depth--
			p.loops = p.loops[:len(p.loops)-1]
			if later {
				p.later--
			}
		}()
	}
	stack = append(stack, n)
	for _, ch := range n.CompactChildNodes() {
		p.walk(ch, stack)
	}
}

// parentCall is the call a block literal belongs to.
func parentCall(stack []parser.Node, blk parser.Node) (*parser.CallNode, bool) {
	if len(stack) == 0 {
		return nil, false
	}
	c, ok := stack[len(stack)-1].(*parser.CallNode)
	return c, ok && c.Block == blk
}

// runsLater: c's block may run after c returns (a lambda, a thread, a method that keeps its &block).
func (p *appendProof) runsLater(c *parser.CallNode) bool {
	if laterBlocks[c.Name] {
		return true
	}
	if m := p.f.c.topDefs[c.Name]; m != nil && m.BlockParam != "" {
		return true
	}
	for _, k := range p.f.c.classList {
		if m := k.Methods[c.Name]; m != nil && m.BlockParam != "" && m.File != nil && !m.File.isPrelude() {
			return true
		}
	}
	return false
}

func (p *appendProof) fresh(v parser.Node) bool {
	switch t := v.(type) {
	case *parser.StringNode:
		return !p.frozen
	case *parser.InterpolatedStringNode:
		return true
	case *parser.ParenthesesNode:
		if st, ok := t.Body.(*parser.StatementsNode); ok && len(st.Body) > 0 {
			return p.fresh(st.Body[len(st.Body)-1])
		}
	case *parser.CallNode:
		if t.Block != nil {
			return false
		}
		switch t.Name {
		case "+@":
			switch t.Receiver.(type) {
			case *parser.StringNode, *parser.InterpolatedStringNode:
				return true
			}
			return false
		case "new":
			c, ok := t.Receiver.(*parser.ConstantReadNode)
			return ok && c.Name == "String"
		case "format", "sprintf":
			return t.Receiver == nil
		}
		return t.Receiver != nil && freshStrCalls[t.Name]
	}
	return false
}

func (p *appendProof) read(n parser.Node, stack []parser.Node) {
	if len(stack) == 0 {
		p.leak(n, fmt.Sprintf("`%s` is stored or passed", p.name))
		return
	}
	parent := stack[len(stack)-1]
	if p.later > 0 { // another goroutine or a later call would read the local while it is appended to
		if c, ok := parent.(*parser.CallNode); !ok || c.Receiver != n || !strAppendNames[c.Name] && c.Name != "setbyte" {
			p.leak(n, fmt.Sprintf("`%s` is captured by a block that may run after its call returns", p.name))
			return
		}
	}
	if c, ok := parent.(*parser.CallNode); ok && c.Receiver == n {
		switch {
		case c.Name == "setbyte":
			p.changeAt(c)
		case c.Name == "concat" && len(callArgs(c)) == 0: // appends nothing: its value is the String itself, used as the local is
			if why := p.use(c, stack[:len(stack)-1]); why != "" {
				p.leak(c, why)
			}
		case strAppendNames[c.Name] && !p.ivar:
			p.appendAt(c, stack[:len(stack)-1])
		case p.strict:
			if !scalarStrMethods[c.Name] || c.Block != nil {
				p.leak(n, fmt.Sprintf("`%s.%s` may share the String's bytes", p.name, c.Name))
			}
		case readOnlyStrMethods[c.Name] && c.Block == nil || c.Name == "each_char" || c.Name == "each_byte" || c.Name == "each_line":
		default:
			p.leak(n, fmt.Sprintf("`%s.%s` may keep, return or change the String", p.name, c.Name))
		}
		return
	}
	if why := p.use(n, stack); why != "" {
		p.leak(n, why)
	}
}

// appendAt records an append c on the local; stack ends at c's parent.
func (p *appendProof) appendAt(c *parser.CallNode, stack []parser.Node) {
	if p.later > 0 {
		p.fail(c, "`%s` is appended to inside a block that may run after its call returns", p.name)
		return
	}
	p.sealAt(c)
	top := parser.Node(c) // `s << a << b`: the chain's value is s itself
	for len(stack) > 0 {
		up, ok := stack[len(stack)-1].(*parser.CallNode)
		if !ok || up.Receiver != top || up.Name != "<<" {
			break
		}
		top, stack = up, stack[:len(stack)-1]
	}
	if why := p.use(top, stack); why != "" {
		p.leak(top, "the value of `"+p.name+" <<` (the String itself)"+why[len("`"+p.name+"`"):])
	}
}

// changeAt records a setbyte c on the target; its value is the byte argument, not the String.
func (p *appendProof) changeAt(c *parser.CallNode) {
	if p.later > 0 {
		p.fail(c, "`%s` is changed inside a block that may run after its call returns", p.name)
		return
	}
	p.sealAt(c)
}

// sealAt moves the seal past c, or past the outermost loop or block around it.
func (p *appendProof) sealAt(c parser.Node) {
	end := endOf(c)
	if len(p.loops) > 0 {
		end = p.loops[0]
	}
	p.seal = max(p.seal, end)
}

// useAsArgument is why n, one of the arguments of the call (or return, or next) above up, holds the String elsewhere, or "".
func (p *appendProof) useAsArgument(n parser.Node, up []parser.Node, held string) string {
	if len(up) == 0 {
		return held
	}
	switch g := up[len(up)-1].(type) {
	case *parser.ReturnNode:
		if p.depth == 0 && !p.out {
			return ""
		}
	case *parser.NextNode:
		for i := len(up) - 1; i >= 0; i-- {
			if b, ok := up[i].(*parser.BlockNode); ok {
				return p.use(b.Body, append(up[:i:i], b))
			}
		}
	case *parser.CallNode:
		if p.argSafe(g) {
			return ""
		}
		if p.f.outCallArg(g, n) { // the callee appends through it: an append here (decision 405)
			if p.later > 0 {
				return fmt.Sprintf("`%s` is passed to %s inside a block that may run after its call returns", p.name, g.Name)
			}
			p.sealAt(g)
			return ""
		}
		return fmt.Sprintf("`%s` is passed to %s", p.name, g.Name)
	}
	return held
}

// use is why n's value (the local's String) may end up held elsewhere, or "" (stack ends at n's parent).
func (p *appendProof) use(n parser.Node, stack []parser.Node) string {
	held := "`" + p.name + "` is stored or passed"
	if n == p.root && !p.out {
		return "" // the method's value: returned when nothing can append any more
	}
	if len(stack) == 0 {
		return held
	}
	parent, up := stack[len(stack)-1], stack[:len(stack)-1]
	switch t := parent.(type) {
	case *parser.StatementsNode:
		if t.Body[len(t.Body)-1] != n {
			return ""
		}
		return p.use(t, up)
	case *parser.EmbeddedStatementsNode:
		if p.strict { // a lone `"#{s}"` may be s itself
			return fmt.Sprintf("`%s` is interpolated", p.name)
		}
		return "" // interpolation copies
	case *parser.IfNode:
		if t.Predicate == n {
			return ""
		}
		return p.use(t, up)
	case *parser.UnlessNode:
		if t.Predicate == n {
			return ""
		}
		return p.use(t, up)
	case *parser.WhileNode, *parser.UntilNode, *parser.ForNode, *parser.EnsureNode:
		return ""
	case *parser.CaseNode:
		if t.Predicate == n {
			return ""
		}
		return p.use(t, up)
	case *parser.ElseNode, *parser.WhenNode, *parser.InNode, *parser.ParenthesesNode, *parser.BeginNode,
		*parser.RescueNode, *parser.AndNode, *parser.OrNode, *parser.CaseMatchNode:
		return p.use(t, up)
	case *parser.BlockNode:
		if c, ok := parentCall(up, t); ok && discardBlocks[c.Name] {
			return ""
		}
		if t == p.root && p.rootFn != nil && discardBlocks[p.rootFn.Name] {
			return ""
		}
		return fmt.Sprintf("`%s` is a block's value", p.name)
	case *parser.ArgumentsNode:
		return p.useAsArgument(n, up, held)
	}
	return held
}

// argSafe: g takes the String as an argument without keeping it (nor, for a setbyte target, any of its bytes).
func (p *appendProof) argSafe(g *parser.CallNode) bool {
	if !p.strict {
		return strArgCalls[g.Name]
	}
	return scalarArgCalls[g.Name] || g.Receiver == nil && (g.Name == "puts" || g.Name == "print")
}

// userAppend compiles user code's `s << x` (and chains of it), `s.concat(a, ...)` or `s.append_as_bytes(a, ...)` on a
// String local (decision 382).
func (f *fctx) userAppend(n *parser.CallNode) (expr, bool) {
	if f.lenient() || !strAppendNames[n.Name] || n.Block != nil || n.IsSAFE_NAVIGATION() || f.c.classes["String"].lookup(n.Name) != nil {
		return expr{}, false
	}
	links := []*parser.CallNode{n} // innermost last
	for n.Name == "<<" {
		r, ok := links[len(links)-1].Receiver.(*parser.CallNode)
		if !ok || r.Name != "<<" || r.Block != nil {
			break
		}
		links = append(links, r)
	}
	lv, ok := links[len(links)-1].Receiver.(*parser.LocalVariableReadNode)
	if !ok || !typeEq(f.probeType(lv, nil), f.cls("String")) {
		return expr{}, false
	}
	if n.Name == "concat" && len(callArgs(n)) == 0 { // appends nothing and returns self: the local, no proof needed
		v := f.genExpr(lv, nil)
		return expr{code: v.code, typ: v.typ}, true
	}
	if why := f.appendWhy(lv.Name); why != "" {
		f.errorf(n, "undefined method %s for String: Strings are immutable in rugaroo, and %s (decision 382)", n.Name, why)
	}
	_, write, _ := f.stringBangTarget(n, lv)
	if li := f.localInfo(lv.Name); li != nil {
		li.appended = true
	}
	str := f.cls("String")
	buf := f.appendBufRef(lv.Name)
	for i := len(links) - 1; i >= 0; i-- {
		args := callArgs(links[i])
		if len(args) == 0 || links[i].Name == "<<" && len(args) != 1 {
			f.errorf(links[i], "wrong number of arguments for String#%s", links[i].Name)
		}
		codes := make([]string, len(args))
		for j, a := range args { // MRI's concat evaluates every argument before it appends
			if isSplat(a) {
				f.errorf(a, "a splat argument to String#%s is not supported", n.Name)
			}
			x := f.genExpr(a, nil)
			switch {
			case n.Name == "append_as_bytes": // a String's bytes or an Integer's low byte, checked at run time as MRI does
				codes[j] = "rbBytesArg(" + f.coerce(a, x, TAny{}) + ")"
			case typeEq(x.typ, TOpt{Elem: str}): // `s << nil` raises TypeError in MRI too
				codes[j] = "rbMustStr(" + x.code + ")"
			case typeEq(x.typ, str):
				codes[j] = x.code
			default:
				f.errorf(a, "String#%s takes a String here, not %s (append an Integer codepoint as i.chr(Encoding::UTF_8))", n.Name, x.typ)
			}
			if len(args) > 1 {
				tmp := f.newTmp()
				f.emit("%s := String(%s)", tmp, codes[j])
				codes[j] = tmp
			}
		}
		for _, c := range codes {
			write(expr{code: fmt.Sprintf("rbStrAppend(%s, %s, %s)", buf, f.genExpr(lv, nil).code, c), typ: str})
		}
	}
	v := f.genExpr(lv, nil)
	return expr{code: v.code, typ: v.typ, stmt: true, done: true}, true
}

// userSetbyte compiles user code's `s.setbyte(i, b)` on a String local, or an ivar in initialize, that the proof clears (decision 395).
func (f *fctx) userSetbyte(n *parser.CallNode) (expr, bool) {
	if f.lenient() || n.Name != "setbyte" || n.Block != nil || n.IsSAFE_NAVIGATION() || f.c.classes["String"].lookup(n.Name) != nil {
		return expr{}, false
	}
	var why, buf string
	str := f.cls("String")
	switch r := n.Receiver.(type) {
	case *parser.LocalVariableReadNode:
		if !typeEq(f.probeType(r, nil), str) {
			return expr{}, false
		}
		why, buf = f.appendWhy(r.Name), f.appendBufRef(r.Name)
		if li := f.localInfo(r.Name); li != nil && why == "" {
			li.appended = true
		}
	case *parser.InstanceVariableReadNode:
		if !typeEq(f.probeType(r, nil), str) {
			return expr{}, false
		}
		why, buf = f.ivarSetbyteWhy(r.Name), "&"+ivarBuf(r.Name)
		if why == "" {
			if f.ivarBufs == nil {
				f.ivarBufs = map[string]bool{}
			}
			f.ivarBufs[r.Name] = true
		}
	default:
		return expr{}, false
	}
	if why != "" {
		f.errorf(n, "undefined method setbyte for String: Strings are immutable in rugaroo, and %s (decision 395)", why)
	}
	args := callArgs(n)
	if len(args) != 2 || isSplat(args[0]) || isSplat(args[1]) {
		f.errorf(n, "wrong number of arguments (given %d, expected 2) for String#setbyte", len(args))
	}
	intT := f.cls("Integer")
	codes := [2]string{}
	for j, a := range args { // MRI evaluates both before it checks the index
		codes[j] = f.newTmp()
		f.emit("%s := Integer(%s)", codes[j], f.coerce(a, f.genExpr(a, intT), intT))
	}
	_, write, _ := f.stringBangTarget(n, n.Receiver)
	write(expr{code: fmt.Sprintf("rbStrSetbyte(%s, %s, %s, %s)", buf, f.genExpr(n.Receiver, nil).code, codes[0], codes[1]), typ: str})
	return expr{code: codes[1], typ: intT, stmt: true, done: true}, true
}

// ivarBuf is the Go name of initialize's buffer for a setbyte'd ivar, declared by hoistLocals.
func ivarBuf(name string) string { return "rbBufIv_" + goLocalName(strings.TrimPrefix(name, "@")) }

// appendBuf is the Go name of an appended local's buffer, declared with it by hoistLocals.
func appendBuf(name string) string { return "rbBuf_" + goLocalName(name) }

// appendBufRef is the *[]byte a local's appends go through: its own buffer, or an out parameter's caller's (decision 405).
func (f *fctx) appendBufRef(name string) string {
	if f.m != nil {
		for i, p := range f.m.Params {
			if p.Out && p.Name == name {
				return outParamName(p, i) + ".b"
			}
		}
	}
	return "&" + appendBuf(name)
}

func endOf(n parser.Node) int { l := n.GetLocation(); return l.StartOffset + l.Length }
