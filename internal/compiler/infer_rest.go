package compiler

import (
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Positional types through a `*args` forwarder, in the not-known-yet rounds (docs/research/inference-2026-11.md).
//
// A rest parameter is one Array, so `def push(*args) = Literal.new(self, *args)` hands Literal#initialize untyped
// elements and its parameters stay untyped (parser's LexerStrings#push_literal: Literal@heredoc_e, read as
// `p = literal.heredoc_e - 1` in the lexer's hot loop). Each call of the forwarder records its arguments by position
// under the rest parameter's key, and a `*args` of that parameter, when the body does nothing else with it, passes
// position j's type on as a use of the parameter it lands in. The call still passes the Array's elements, converted
// where they land, so a value another route brings is checked there as any untyped argument is.

// restPosKey is the j-th value a rest parameter (key k) receives; j < 0 is the mark of a call that splats into it,
// whose positions are not known.
func restPosKey(k pendingKey, j int) pendingKey {
	if j < 0 {
		return k + "@*"
	}
	return k + pendingKey("@"+strconv.Itoa(j))
}

func isRestPosKey(k pendingKey) bool {
	i := strings.LastIndexByte(string(k), '@')
	if i < 0 || i == len(k)-1 {
		return false
	}
	if k[i+1:] == "*" {
		return true
	}
	_, err := strconv.Atoi(string(k[i+1:]))
	return err == nil
}

// noteRestPos records the j-th rest argument of a call (j < 0: a splat among them).
func (f *fctx) noteRestPos(p Param, n parser.Node, j int, t Type) {
	if !f.c.env.InferUnknown || p.Pending == "" || f.c.uses == nil {
		return
	}
	if j < 0 {
		t = f.cls("Boolean") // a mark, not a value's type: any typed use keeps the key in the state
	}
	f.noteUse(Param{Pending: restPosKey(p.Pending, j)}, n, t)
}

// restForwardType is the type of element j of `*args` at sp, when args is the enclosing def's rest parameter, the
// body only ever splats it, and every call of the def passed that position one type; nil otherwise.
func (f *fctx) restForwardType(sp *parser.SplatNode, j int) Type {
	c := f.c
	if sp == nil || !c.round || !c.env.InferUnknown || c.infer == nil || f.m == nil || f.m.Node == nil || j < 0 {
		return nil
	}
	r, ok := sp.Expression.(*parser.LocalVariableReadNode)
	if !ok {
		return nil
	}
	var rest *Param
	for i := range f.m.Params {
		if p := &f.m.Params[i]; p.Rest && !p.Keyword && !p.KwRest && p.Name == r.Name && p.Pending != "" {
			rest = p
		}
	}
	if rest == nil || !onlySplatted(f.m.Node.Body, r.Name) {
		return nil
	}
	if _, splatted := c.infer.types[restPosKey(rest.Pending, -1)]; splatted {
		return nil
	}
	t, ok := c.infer.types[restPosKey(rest.Pending, j)]
	if !ok || isAny(t) {
		return nil
	}
	return c.rehome(t)
}

// onlySplatted reports that body reads local name only as `*name` and never writes it.
func onlySplatted(body parser.Node, name string) bool {
	reads, splats := 0, 0
	bad := walkNode(body, func(x parser.Node) (bool, bool) {
		switch x := x.(type) {
		case *parser.LocalVariableReadNode:
			if x.Name == name {
				reads++
			}
		case *parser.SplatNode:
			if r, ok := x.Expression.(*parser.LocalVariableReadNode); ok && r.Name == name {
				splats++
			}
		case *parser.LocalVariableWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableOperatorWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableOrWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableAndWriteNode:
			return x.Name == name, true
		case *parser.LocalVariableTargetNode:
			return x.Name == name, true
		}
		return false, true
	})
	return !bad && reads == splats && splats > 0
}
