package compiler

import (
	"slices"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// untypedGemUse is a gem file's use passing an Array or Hash of untyped (optparse's `define(*opts)` passing opts on
// to `make_switch(opts)`): the parameter takes it, not a narrower type another call gave, so the method's branches
// for the other element kinds compile. Not when another use passes some other class (kramdown's
// `Options.define(name, type, default, desc)` with a `{}`, a String and an Integer default): mixed, so untyped.
func (c *Compiler) untypedGemUse(uses []paramUse) (u paramUse, ok, mixed bool) {
	for _, u := range uses {
		if cl, ok := u.typ.(TClass); ok && len(cl.Args) > 0 && c.gemFile(u.file) && (cl.C.RubyName == "Array" || cl.C.RubyName == "Hash") &&
			isAny(cl.Args[len(cl.Args)-1]) && !mentionsVar(u.typ) {
			return u, true, slices.ContainsFunc(uses, func(x paramUse) bool { return c.otherClassUse(x, cl.C) })
		}
	}
	return paramUse{}, false, false
}

// otherClassUse is a use passing a value that may not be a cls: another class, or a gem's untyped value (kramdown's
// `item.value, ... = parse_first_list_line(...)` into the writer `[item.value]` also calls). Nil is not.
func (c *Compiler) otherClassUse(u paramUse, cls *Class) bool {
	if _, ok := u.typ.(TAny); ok && c.gemFile(u.file) {
		return true
	}
	if cl, ok := stripOpt(u.typ).(TClass); ok { // its class is known even when its elements are untyped (`[drop, assigns]`)
		return cl.C != cls
	}
	if _, void := u.typ.(TVoid); u.typ == nil || void || isNil(u.typ) || holdsAny(u.typ) || mentionsVar(u.typ) {
		return false
	}
	return true
}

// gemUsesDisagree reports gem-file uses that joinUses left out of t because they do not join it.
// For a gem method's own parameter (gemParam) a user's calls count too: liquid's render!(*args) takes `{...}` and
// `strict_filters: true` from one program (decision 331).
func (c *Compiler) gemUsesDisagree(uses []paramUse, t Type, gemParam bool) bool {
	for _, u := range uses {
		// a bare untyped use is a round's not-yet-typed value, which counts for nothing (decision 307): only a container
		// of untyped elements is a kind for a gem's own parameter (decision 345)
		if u.typ == nil || isVoid(u.typ) || isAny(stripOpt(u.typ)) || holdsAny(u.typ) && !gemParam || mentionsVar(u.typ) || !gemParam && !c.gemFile(u.file) {
			continue
		}
		if j, ok := join(t, u.typ); !ok || !typeEq(j, t) {
			return true
		}
	}
	return false
}

// onlyInterpolated reports whether body reads local name, and only as a whole `#{name}` interpolation or the receiver of
// a conversion every object answers (no other call on it, no write): what it holds needs only to_s.
func onlyInterpolated(body parser.Node, name string) bool {
	reads, ok := 0, true
	var walk func(parser.Node, bool)
	walk = func(n parser.Node, interp bool) {
		switch x := n.(type) {
		case nil, *parser.DefNode:
			return
		case *parser.LocalVariableReadNode:
			if x.Name == name && x.Depth == 0 {
				reads++
				ok = ok && interp
			}
			return
		case *parser.LocalVariableWriteNode:
			ok = ok && x.Name != name
		case *parser.LocalVariableOperatorWriteNode:
			ok = ok && x.Name != name
		case *parser.LocalVariableOrWriteNode:
			ok = ok && x.Name != name
		case *parser.LocalVariableAndWriteNode:
			ok = ok && x.Name != name
		case *parser.LocalVariableTargetNode:
			ok = ok && x.Name != name
		case *parser.EmbeddedStatementsNode:
			if x.Statements != nil && len(x.Statements.Body) == 1 {
				if r, isRead := x.Statements.Body[0].(*parser.LocalVariableReadNode); isRead {
					walk(r, true)
					return
				}
			}
		case *parser.CallNode: // `method.to_s` (liquid's Strainer.invokable?): what every object answers
			if r, isRead := x.Receiver.(*parser.LocalVariableReadNode); isRead && universalConversions[x.Name] && x.Block == nil {
				walk(r, true)
				for _, a := range callArgs(x) {
					walk(a, false)
				}
				return
			}
		}
		for _, ch := range n.CompactChildNodes() {
			walk(ch, false)
		}
	}
	walk(body, false)
	return ok && reads > 0
}

// anyGemUseWidens: a gem passing an untyped value where the other uses joined to a class object (liquid's
// add_filter(f) for each filter, after add_filter(StandardFilters)) or to a union (BlockBody's render_node_to_output
// from `when Variable`, `when Block` and an `else` arm's untyped node), which Go holds as any already (decision 331).
//
// A union with a dynamic call's use is untyped too: that call may be meant for another class's method of the name
// (Context#evaluate's `context.evaluate(@name)` is no VariableLookup#evaluate call), so its classes do not bound it.
func (c *Compiler) anyGemUseWidens(t Type, uses []paramUse) bool {
	cl, isClass := stripOpt(t).(TClass)
	union := isUnion(stripOpt(t))
	if union && slices.ContainsFunc(uses, func(u paramUse) bool { return u.dyn }) {
		return true
	}
	if (!isClass || cl.C.metaOf == nil) && !union {
		return false
	}
	return slices.ContainsFunc(uses, func(u paramUse) bool { return isAny(u.typ) && c.gemFile(u.file) })
}

// universalConversions are calls every object answers the same way through Kernel, whatever its class.
var universalConversions = map[string]bool{"to_s": true, "inspect": true, "nil?": true, "==": true, "!=": true, "hash": true, "eql?": true, "equal?": true}

// typeTested reports whether body tests local name's class (`case obj when Float ... when String`, `obj.is_a?(X)`,
// `obj.respond_to?(:m)`): a parameter meant to take several classes (liquid's Utils.to_number, decision 331).
func typeTested(body parser.Node, name string) bool {
	isName := func(n parser.Node) bool {
		r, ok := n.(*parser.LocalVariableReadNode)
		return ok && r.Name == name && r.Depth == 0
	}
	return anyNode(body, func(n parser.Node) bool {
		switch x := n.(type) {
		case *parser.CaseNode:
			return isName(x.Predicate)
		case *parser.CallNode:
			return isName(x.Receiver) && (x.Name == "is_a?" || x.Name == "kind_of?" || x.Name == "instance_of?" || x.Name == "respond_to?")
		}
		return false
	})
}

// gemUseType is a parameter's type when its uses settle it before joining: anything for a widening, untyped-rest or
// mixed-class gem use or an untyped one into a parameter its body only interpolates or type-tests (decisions 324,
// 330, 331), and an untyped container a gem passes (decision 244).
func (c *Compiler) gemUseType(key pendingKey, uses []paramUse) (Type, string, bool) {
	if i := slices.IndexFunc(uses, func(u paramUse) bool { return u.widen || u.anyRest }); i >= 0 {
		// concurrent-ruby's new_blocked_by1(self, executor, executor, args): a rest parameter collecting any values
		c.noteWhy(key, uses, "a gem's untyped value into an overridden method or a rest parameter, at "+uses[i].at().String()+" (decisions 253, 324)")
		return TAny{}, uses[i].at().String(), true
	}
	if u, ok, mixed := c.untypedGemUse(uses); mixed { // and other calls pass other classes: anything (decision 330)
		c.noteWhy(key, uses, "two uses disagree: a gem passes "+typeString(u.typ)+" at "+u.at().String()+" and another call passes another class (decision 330)")
		return TAny{}, u.at().String(), true
	} else if ok { // a gem passing an untyped container: its elements are anything (decision 244)
		c.noteWhy(key, uses, "a gem passes the untyped container "+typeString(u.typ)+" at "+u.at().String()+", which the parameter takes over narrower uses (decision 244)")
		t := u.typ
		if slices.ContainsFunc(uses, func(x paramUse) bool { return isNil(x.typ) }) { // `default = nil` too (bindata's parent_attribute): nilable
			t = optOf(t)
		}
		return t, u.at().String(), true
	}
	if i := slices.IndexFunc(uses, func(u paramUse) bool { return isAny(u.typ) && c.gemFile(u.file) }); i >= 0 && c.interpOnly[key] {
		// kramdown's format_as_span_html(name, ...) gets "code" from some calls and el.type, a Symbol, from others
		c.noteWhy(key, uses, "untyped gem argument at "+uses[i].at().String()+" into a parameter its body only interpolates or type-tests (decisions 330, 331)")
		return TAny{}, uses[i].at().String(), true
	}
	return nil, "", false
}
