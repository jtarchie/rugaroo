package compiler

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// constFctx is the codegen context a constant's initializer runs in: the
// class body, whose self is the class object (main at top level).
func (c *Compiler) constFctx(k *Const) *fctx {
	f := c.newFctx(k.File, nil, nil)
	if n := len(k.Scope); n > 0 && k.Scope[n-1].meta != nil {
		cls := k.Scope[n-1]
		f.selfType, f.selfCode, f.selfClassObj = TClass{C: cls.meta}, classVar(cls), true
	}
	f.lex = k.Scope
	f.locals = map[localKey]*localInfo{}
	f.scope = &scope{vars: map[string]*local{}}
	f.pass = 2
	return f
}

// probeTopLocals types, in f, the top-level local assignments of k's file before k (decision 296); one that does not
// compile is left undefined, so k's own probe names it.
func (c *Compiler) probeTopLocals(f *fctx, k *Const) {
	at := k.Value.GetLocation().StartOffset
	for _, n := range k.File.Root.Statements.Body {
		if w, ok := n.(*parser.LocalVariableWriteNode); ok && w.Location.StartOffset < at {
			_ = catchCompileError(func() { f.genStmt(w, tail{}) }) // f is k's own: what it emits is dropped
		}
	}
}

// constType is the type of constant k: its `#: T` annotation, or else the
// type of its initializer.
func (c *Compiler) constType(k *Const) (typ Type) {
	if c.env.KeepGoing && c.anonDepth == 0 {
		defer func() {
			if r := recover(); r != nil {
				ce, ok := r.(compileError)
				if !ok {
					panic(r)
				}
				c.kept = append(c.kept, ce.Error())
				k.Type, k.resolving, typ = TAny{}, false, TAny{}
			}
		}()
	}
	if k.Type != nil {
		return k.Type
	}
	if k.first != nil {
		return c.constType(k.first)
	}
	if _, boot := c.bootConstClasses[k.RubyName]; boot {
		c.constClass(k) // a class picked at load time: its value becomes that class (decision 203)
	}
	if k.ann != "" {
		t, err := rbs.ParseType(k.ann)
		if err != nil {
			c.errorf(k.File, k.Value, "%v", err)
		}
		k.Type = c.resolveType(t, typeScope{lex: k.Scope, srcPos: srcPos{k.File, k.Line}})
		return k.Type
	}
	if t := c.sigConst(k); t != nil { // a `.rbs` constant (decision 160)
		k.Type = c.resolveType(t, typeScope{lex: k.Scope, srcPos: srcPos{k.File, k.Line}})
		return k.Type
	}
	if k.inBody && c.gemFile(k.File) {
		k.Type = TAny{} // gem code: untyped rather than typed outside its body's unit
		return k.Type
	}
	if k.inBody {
		c.errorf(k.File, k.Value, "constant %s is assigned inside its class body's statements (a local it reads, a begin or a block): give it a type (`#: T`, or `%s: T` in a .rbs file)", k.RubyName, k.RubyName[strings.LastIndex(k.RubyName, ":")+1:])
	}
	if k.resolving && c.gemFile(k.File) {
		return TAny{} // gem code: a constant whose inferred type loops back to itself is untyped (decision 172)
	}
	if k.resolving {
		c.errorf(k.File, k.Value, "constant %s depends on itself (via %s); annotate it with `#: T`", k.RubyName, strings.Join(c.constChain, " -> "))
	}
	k.resolving = true
	c.constChain = append(c.constChain, k.RubyName)
	f := c.constFctx(k)
	if k.inMain {
		c.probeTopLocals(f, k)
	}
	var e expr
	if c.gemFile(k.File) {
		err := catchCompileError(func() { f.probe(func() { e = f.genExpr(k.Value, nil) }) })
		if err != nil {
			c.constChain = c.constChain[:len(c.constChain)-1]
			k.resolving, k.poison, k.Type = false, err.Error(), TAny{}
			if c.env.ConstDebug { // the warning below is dropped when a discarded pass (a probe, a dry run) is the first to type the constant
				fmt.Fprintf(os.Stderr, "rugaroo: constant %s does not compile: %s\n", k.RubyName, err.Error())
			}
			c.warn(k.File, k.Value, "constant %s does not compile, so it is nil: %s (decision 205)", k.RubyName, err.Error())
			return k.Type
		}
	} else {
		f.probe(func() { e = f.genExpr(k.Value, nil) })
	}
	c.constChain = c.constChain[:len(c.constChain)-1]
	k.resolving = false
	if (isNil(e.typ) || isVoid(e.typ)) && (c.gemFile(k.File) || k.typeAlias != "") {
		e.typ = TAny{} // a Sorbet type alias's run-time value, or a gem constant only ever nil: untyped
	}
	if isNil(e.typ) || isVoid(e.typ) {
		c.errorf(k.File, k.Value, "cannot infer the type of constant %s; annotate it with `#: T`", k.RubyName)
	}
	k.Type = c.joinAgains(k, e.typ)
	if c.gemFile(k.File) {
		k.Type = c.widenConstElems(k, f, k.Type)
	}
	return k.Type
}

// widenConstElems is a gem's Hash or Array constant t with its element untyped when a `K[k] = v` (or `K << v`) in its file
// stores a v its element type cannot hold: net-ftp's `FACT_PARSERS = Hash.new(CASE_DEPENDENT_PARSER)` given a two-param
// TIME_PARSER, as MRI's Hash holds any value (decision 416).
func (c *Compiler) widenConstElems(k *Const, f *fctx, t Type) Type {
	tc, ok := t.(TClass)
	if !ok || len(tc.Args) == 0 || tc.C.RubyName != "Hash" && tc.C.RubyName != "Array" {
		return t
	}
	short := k.RubyName[strings.LastIndex(k.RubyName, ":")+1:]
	vi := len(tc.Args) - 1
	if isAny(tc.Args[vi]) {
		return t
	}
	widen := false
	anyNode(k.File.Root, func(n parser.Node) bool {
		call, ok := n.(*parser.CallNode)
		if widen || !ok || call.Name != "[]=" && call.Name != "<<" || !namesConst(call.Receiver, short) {
			return false
		}
		args := callArgs(call)
		if len(args) == 0 {
			return false
		}
		var v expr
		if catchCompileError(func() { f.probe(func() { v = f.genExpr(args[len(args)-1], nil) }) }) != nil {
			return false
		}
		_, vf := v.typ.(TFunc)
		_, ef := tc.Args[vi].(TFunc)
		widen = vf && ef && c.goType(v.typ) != c.goType(tc.Args[vi]) || !vf && !isAny(v.typ) && !fits(v.typ, tc.Args[vi])
		return false
	})
	if !widen {
		return t
	}
	args := slices.Clone(tc.Args)
	args[vi] = TAny{}
	return TClass{C: tc.C, Args: args}
}

// namesConst reports n reading the constant short, bare or as the last segment of a path.
func namesConst(n parser.Node, short string) bool {
	switch r := n.(type) {
	case *parser.ConstantReadNode:
		return r.Name == short
	case *parser.ConstantPathNode:
		return r.Name != nil && *r.Name == short
	}
	return false
}

// joinAgains is a constant's type with what its later assignments give it, as a local joins its writes (decision 363).
func (c *Compiler) joinAgains(k *Const, t Type) Type {
	for _, a := range k.rewrites {
		f := c.constFctx(a)
		var e expr
		f.probe(func() { e = f.genExpr(a.Value, nil) })
		j, ok := join(t, e.typ)
		if !ok {
			c.errorf(a.File, a.Value, "constant %s is assigned %s here and %s before; annotate the first with `#: T`", k.RubyName, e.typ, t)
		}
		t = j
	}
	return t
}

// emitConst declares a constant's package variable. Its value is assigned
// in main, in source order, where MRI would evaluate it (see constInit).
func (c *Compiler) emitConst(k *Const) {
	if k.first != nil {
		return
	}
	c.lineDirective(k.File, k.Line)
	c.w("var %s %s\n\n", k.GoName, c.goType(c.constType(k)))
	if k.guarded {
		c.w("var %s bool\n\n", constSet(k))
	}
}

// constSet names the flag main sets once a guarded constant is assigned.
func constSet(k *Const) string { return "rbSet_" + k.GoName }

// constEntries lists the constants a class object answers to: its own in
// definition order, then those of its ancestors (Object's are the top-level
// ones).
func (c *Compiler) constEntries(cls *Class) []string {
	if cls.RubyName == "Object" {
		return c.topConstNames
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range cls.ancestors() {
		for _, n := range k.constNames {
			if !seen[n] {
				seen[n] = true
				out = append(out, k.RubyName+"::"+n)
			}
		}
	}
	return out
}

// constValue is the Go expression for a constant's current value, boxed.
func (c *Compiler) constValue(full string) (string, bool) {
	if cls := c.classes[full]; cls != nil {
		if cls.meta == nil {
			return "", false
		}
		return classVar(cls), true
	}
	k := c.consts[full]
	if isOpt(c.constType(k)) {
		return c.optAny(k.GoName, c.constType(k)), true
	}
	return k.GoName, true
}

// emitConstTable gives a class object its constant table. Values are read
// when the table is asked for, so constants assigned later are seen.
func (c *Compiler) emitConstTable(cls *Class) {
	desc := cls.metaOf
	kind := "class"
	if desc != nil && desc.IsModule {
		kind = "module"
	}
	c.w("func (self *%s) _Kind() string { return %q }\n\n", cls.Name, kind)
	c.w("func (self *%s) _IsInstance(v any) bool { return %s }\n\n", cls.Name, c.isInstanceTest(desc))
	if desc != nil {
		c.w("func (self *%s) _DescID() int { return %d }\n\n", cls.Name, c.classID(desc))
		c.emitDescendants(cls, desc)
	}
	c.emitMethodTable(cls, desc)
	c.emitPrivateMethodTable(cls, desc)
	c.emitSingletonMethodTable(cls, desc)
	if desc == nil {
		c.w("func (self *%s) _Consts(string) []rbConst { return nil }\n\n", cls.Name)
		return
	}
	if flag := c.evalDefFlag(desc); flag != "" {
		c.w("var %s bool\n\n", flag)
	}
	// One name's entries come from a switch, so const_get does not build the whole table per lookup (decision 332);
	// "" is the whole table, for constants and the like.
	var all []string
	byName := map[string][]string{}
	var names []string
	for _, full := range c.constEntries(desc) {
		if v, ok := c.constValue(full); ok {
			short := full[strings.LastIndex(full, ":")+1:]
			inherited := strings.Contains(full, "::") && strings.TrimSuffix(full, "::"+short) != desc.RubyName
			if k := c.classes[full]; k != nil && c.evalDefFlag(k) != "" { // defined when its eval runs (decision 324)
				v = "rbEvalConst{&" + c.evalDefFlag(k) + ", " + v + "}"
			} else if k := c.consts[full]; k != nil && k.guarded { // listed once main assigns it: `V2 = all(:V2)` must not find V2 itself (regexp_parser's UnicodeProperty)
				v = "rbEvalConst{&" + constSet(k) + ", " + v + "}"
			}
			entry := fmt.Sprintf("{%q, %s, %t}", short, v, inherited)
			all = append(all, entry)
			if _, seen := byName[short]; !seen {
				names = append(names, short)
			}
			byName[short] = append(byName[short], entry)
		}
	}
	if len(all) == 0 {
		c.w("func (self *%s) _Consts(string) []rbConst { return nil }\n\n", cls.Name)
		return
	}
	c.w("func (self *%s) _Consts(name string) []rbConst {\n\tswitch name {\n\tcase \"\":\n\t\treturn []rbConst{\n", cls.Name)
	for _, e := range all {
		c.w("\t\t\t%s,\n", e)
	}
	c.w("\t\t}\n")
	for _, n := range names {
		c.w("\tcase %q:\n\t\treturn []rbConst{%s}\n", n, strings.Join(byName[n], ", "))
	}
	c.w("\t}\n\treturn nil\n}\n\n")
}
