package compiler

import (
	"slices"
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

// noteDynUse records a dynamic call's argument a of type t as a use of p, a method of the call's name: when it has a
// type, or, once the rounds resolved, when it is untyped (an untyped argument is then one for good, and untypes what
// it may reach). An argument a rest parameter takes is recorded by its position j too.
func (f *fctx) noteDynUse(p Param, a parser.Node, t Type, j int, inRest bool) {
	resolved := f.c.infer != nil && f.c.infer.resolved
	if t == nil || isAny(t) && (!resolved || isUnknown(t)) {
		return
	}
	u := paramUse{typ: t, file: f.f, off: a.GetLocation().StartOffset, dyn: true, setter: f.dynSetter, why: f.useWhy(a, t)}
	f.c.uses[p.Pending] = append(f.c.uses[p.Pending], u)
	if f.c.env.InferUnknown && inRest {
		k := restPosKey(p.Pending, j)
		f.c.uses[k] = append(f.c.uses[k], u)
	}
}

// noteDynPostUses is noteDynUses for a method with parameters after its rest (`def m(a, *mid, z)`): the leading
// arguments are the leading parameters', the last ones the post parameters', what is between the rest's. Matched
// left to right, a call's first argument typed the post parameter (an Integer for `evaluate_to(*args, block)`'s
// block, which then raised on the Proc it is passed).
func (f *fctx) noteDynPostUses(m *Method, args []parser.Node, typeOf func(int) Type) {
	var lead, posts []Param
	var rest *Param
	for i := range m.Params {
		switch p := m.Params[i]; {
		case p.Keyword || p.KwRest:
			return
		case p.Rest:
			rest = &m.Params[i]
		case p.Post:
			posts = append(posts, p)
		default:
			lead = append(lead, p)
		}
	}
	if len(args) < len(posts) || slices.ContainsFunc(args, func(a parser.Node) bool { _, kw := a.(*parser.KeywordHashNode); return kw }) {
		return
	}
	firstPost := len(args) - len(posts)
	for i, a := range args {
		switch {
		case i >= firstPost:
			if p := posts[i-firstPost]; p.Pending != "" {
				f.noteDynUse(p, a, typeOf(i), 0, false)
			}
		case i < len(lead):
			if p := lead[i]; p.Pending != "" {
				f.noteDynUse(p, a, typeOf(i), 0, false)
			}
		case rest != nil && rest.Pending != "":
			f.noteDynUse(*rest, a, typeOf(i), i-len(lead), true)
		}
	}
}

// noteRestForward records, for parameter p fed element j of the splat at sp, what every call of the enclosing
// forwarder passed at that position.
func (f *fctx) noteRestForward(p Param, n parser.Node, sp *parser.SplatNode, j int) {
	if t := f.restForwardType(sp, j); t != nil {
		f.noteUse(p, n, t)
	}
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

// noteDynSplat records a dynamic call whose arguments hold a splat at place si (the parser gem's `@builder.loop(:while,
// val[0], *val[1], val[2], val[3])`): nobody can count what the splat puts where, so once the rounds resolve every
// parameter from si on has an untyped use, and the calls without a splat do not type it by agreeing (`loop`'s do_t was
// a token pair, and took the body's Array).
func (f *fctx) noteDynSplat(name string, splat parser.Node, si int) {
	if !f.c.env.InferUnknown || f.c.infer == nil || !f.c.infer.resolved {
		return
	}
	u := paramUse{typ: TAny{}, file: f.f, off: splat.GetLocation().StartOffset, dyn: true}
	for _, m := range f.c.gemMethodsNamed(name) {
		for i := si; i < len(m.Params); i++ {
			if p := m.Params[i]; p.Pending != "" && !p.Keyword && !p.KwRest {
				f.c.uses[p.Pending] = append(f.c.uses[p.Pending], u)
			}
		}
	}
}

// noteSplatUse records element j of a static call's splat as parameter p's argument: a forwarded rest's place, and
// the element's type as a use like any other argument's (decision 477).
func (f *fctx) noteSplatUse(p Param, n parser.Node, sp *parser.SplatNode, j int, elem Type) {
	f.noteRestForward(p, n, sp, j)
	if sp != nil {
		f.noteUse(p, sp, elem)
	}
}
