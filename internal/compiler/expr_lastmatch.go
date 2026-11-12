package compiler

import (
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// subBlockLastMatch is `s.sub(re) { $1 }` / gsub in a frame that reads $~ or $1: MRI sets them to each match while the
// block runs, so the block runner writes the frame's last match first (decision 241).
func (f *fctx) subBlockLastMatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	b, ok := block.(*parser.BlockNode)
	if f.lastMatch == "" || !ok || name != "sub" && name != "gsub" || len(args) != 1 || !isClass(recv.typ, "String") {
		return expr{}, false
	}
	e := f.c.classes["String"].lookup("__" + name + "_block")
	if e == nil || e.M.Block == nil {
		return expr{}, false
	}
	r := f.newTmp()
	f.emit("%s := %s", r, recv.code)
	pat := f.newTmp()
	f.emit("%s := rbPattern(%s)", pat, f.coerce(args[0], f.genExpr(args[0], nil), TAny{}))
	f.binNext = recv.bin // a binary String's matches and block param are bytes (decision 352)
	blk := f.genClosure(n, b, e.M.Block, map[string]Type{})
	count := "-1"
	if name == "sub" {
		count = "1"
	}
	sub := "rbReSub"
	if recv.bin {
		sub = "rbReSubBin"
	}
	code := fmt.Sprintf("String(%s(%s, string(%s), %s, rbReBlockLM(&%s, %s, %s)))", sub, pat, r, count, f.lastMatch, pat, blk)
	return expr{code: code, typ: f.cls("String"), bin: recv.bin, binDeps: recv.binDeps}, true
}

// scanBlockLastMatch is `s.scan(re) { ... Regexp.last_match ... }` with a block of no parameters, in a frame that reads
// $~ (rubocop's FormatString): each match's MatchData is the frame's last match while the block runs, as MRI's. It runs
// as `s.__scan_matches(re).each { |m| <$~ = m>; ... }` and answers the receiver (decision 444).
func (f *fctx) scanBlockLastMatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	b, ok := block.(*parser.BlockNode)
	if f.lastMatch == "" || !ok || name != "scan" || len(args) != 1 || b.Parameters != nil || b.Body == nil || !f.heldAsAny(recv.typ) && !isClass(recv.typ, "String") {
		return expr{}, false
	}
	st, ok := b.Body.(*parser.StatementsNode)
	if !ok {
		return expr{}, false
	}
	loc, m := b.Location, "rb_m_"
	r := f.newTmp()
	f.emit("%s := %s", r, recv.code)
	set := &exprNode{Node: &parser.NilNode{Location: loc}, e: expr{code: fmt.Sprintf("func() { m_ := %s; %s = &m_ }()", goLocalName(m), f.lastMatch), typ: TVoid{}, stmt: true}}
	blk := &parser.BlockNode{Location: loc, Locals: append([]string{m}, b.Locals...),
		Parameters: &parser.BlockParametersNode{Location: loc, Parameters: &parser.ParametersNode{Location: loc, Requireds: []parser.Node{&parser.RequiredParameterNode{Location: loc, Name: m}}}},
		Body:       &parser.StatementsNode{Location: st.Location, Body: append([]parser.Node{set}, st.Body...)}}
	matches := f.genMethodCall(n, expr{code: r, typ: recv.typ}, "__scan_matches", args, nil)
	ms := f.newTmp()
	f.emit("%s := %s", ms, matches.code)
	f.genStmt(&parser.CallNode{Location: loc, Receiver: &exprNode{Node: &parser.NilNode{Location: loc}, e: expr{code: ms, typ: matches.typ}}, Name: "each", Block: blk}, tail{})
	return expr{code: r, typ: recv.typ}, true
}

// grepBlockLastMatch is `xs.grep(re) { $1 }` / grep_v on an Array in a frame that reads $~: each test sets the
// frame's last match before the block runs, as MRI's (decision 171's frame variable, handed to the runner).
func (f *fctx) grepBlockLastMatch(n parser.Node, recv expr, name string, args []parser.Node, block parser.Node) (expr, bool) {
	b, ok := block.(*parser.BlockNode)
	if f.lastMatch == "" || !ok || name != "grep" && name != "grep_v" || len(args) != 1 {
		return expr{}, false
	}
	at, ok := recv.typ.(TClass)
	if !ok || at.C.RubyName != "Array" || len(at.Args) != 1 {
		return expr{}, false
	}
	if !isClass(f.probeType(args[0], nil), "Regexp") {
		return expr{}, false
	}
	e := f.c.classes["Array"].lookup("__" + name + "_block")
	if e == nil || e.M.Block == nil {
		return expr{}, false
	}
	r := f.newTmp()
	f.emit("%s := %s", r, recv.code)
	pat := f.newTmp()
	f.emit("%s := %s", pat, f.genExpr(args[0], nil).code)
	env := map[string]Type{"E": at.Args[0]}
	blk := f.genClosure(n, b, e.M.Block, env)
	code := fmt.Sprintf("rbGrepLM(&%s, %s, %s.s, %v, %s)", f.lastMatch, pat, r, name == "grep_v", blk)
	return expr{code: code, typ: TClass{C: f.c.classes["Array"], Args: []Type{subst(e.M.Block.Ret, env)}}}, true
}

// lastMatchVar is the Go variable holding the frame's last match ($~, decision 171).
func (f *fctx) lastMatchVar(n parser.Node) string {
	if f.lastMatch == "" {
		f.errorf(n, "$~ and $1..$9 are read only in the method whose =~ sets them (decision 171)")
	}
	return f.lastMatch
}

// genMatchSet is `str =~ re` / `re =~ str` in a frame that reads $~ or $1..$9: it records the match (decision 171).
func (f *fctx) genMatchSet(n *parser.CallNode) (expr, bool) {
	args := callArgs(n)
	if f.lastMatch == "" || f.inMatchSet || n.Receiver == nil || n.Block != nil {
		return expr{}, false
	}
	if len(args) == 2 && n.Name == "match" && isClass(f.probeType(n.Receiver, nil), "Regexp") { // re.match(str, pos) (decision 368)
		f.inMatchSet = true
		m := f.genExpr(n, nil)
		f.inMatchSet = false
		return expr{code: fmt.Sprintf("rbMatchKeep(&%s, %s)", f.lastMatch, m.code), typ: m.typ}, true
	}
	if len(args) != 1 {
		return expr{}, false
	}
	var recvE, argE expr
	f.probe(func() { recvE, argE = f.genExpr(n.Receiver, nil), f.genExpr(args[0], nil) })
	recvT, argT := recvE.typ, argE.typ
	re, subj, bin := args[0], n.Receiver, recvE.bin
	if isClass(recvT, "Regexp") {
		re, subj, bin = n.Receiver, args[0], argE.bin
	}
	switch {
	case n.Name == "=~":
	case n.Name == "index" && isClass(recvT, "String") && isClass(argT, "Regexp"): // String#index(re) sets $~ too
	case n.Name == "match" && (isClass(recvT, "Regexp") || isClass(argT, "Regexp")):
	default:
		return expr{}, false
	}
	f.inMatchSet = true // the match call below is the plain one
	m := f.genExpr(&parser.CallNode{Location: n.Location, Receiver: re, Name: "match", MessageLoc: n.MessageLoc,
		Arguments: &parser.ArgumentsNode{Location: n.Location, Arguments: []parser.Node{subj}}}, nil)
	f.inMatchSet = false
	if n.Name == "match" {
		return expr{code: fmt.Sprintf("rbMatchKeep(&%s, %s)", f.lastMatch, m.code), typ: m.typ}, true
	}
	set := "rbMatchSet"
	if bin { // a binary subject's =~ is a byte index (decision 352)
		set = "rbMatchSetBin"
	}
	return expr{code: fmt.Sprintf("rbOptFromPtr(%s(&%s, %s))", set, f.lastMatch, m.code), typ: TOpt{Elem: f.cls("Integer")}}, true
}

// readsLastMatch reports whether body reads $~, $& or $1..$9, so its frame keeps the last match (decision 171).
func readsLastMatch(body parser.Node) bool {
	return anyNode(body, func(n parser.Node) bool {
		switch n := n.(type) {
		case *parser.NumberedReferenceReadNode, *parser.BackReferenceReadNode:
			return true
		case *parser.GlobalVariableReadNode:
			return n.Name == "$~" || n.Name == "$LAST_MATCH_INFO" || englishMatchRefs[n.Name] != ""
		case *parser.CallNode: // Regexp.last_match reads the frame's $~ too
			return isRegexpConst(n.Receiver) && n.Name == "last_match"
		}
		return false
	})
}

// genMatchRef is $1..$9 or $&: a group of the frame's last match (decision 171).
func (f *fctx) genMatchRef(n parser.Node) expr {
	group := 0
	switch n := n.(type) {
	case *parser.NumberedReferenceReadNode:
		group = int(n.Number)
	case *parser.BackReferenceReadNode:
		if fn := map[string]string{"$`": "rbLMPre", "$'": "rbLMPost", "$+": "rbLMLast"}[n.Name]; fn != "" { // decision 414
			return expr{code: fn + "(" + f.lastMatchVar(n) + ")", typ: TOpt{Elem: f.cls("String")}}
		}
	}
	return expr{code: fmt.Sprintf("rbLMGroup(%s, %d)", f.lastMatchVar(n), group), typ: TOpt{Elem: f.cls("String")}}
}
