package compiler

import (
	"slices"
	"strings"
)

// emitClassOf emits rbClassOf, `.class` for a value known only at run
// time: a type switch over the @go_type classes, and over each struct
// hierarchy, whose instances answer _ClassOf. nil is Go nil, not an
// instance of a class, so its class object is a bare Class named NilClass.
func (c *Compiler) emitClassOf() {
	if !c.classOf {
		return
	}
	c.w("type rbNilClass struct{ Class }\n\n")
	for _, m := range []string{"Name", "ToS", "Inspect"} {
		c.w("func (*rbNilClass) %s() String { return \"NilClass\" }\n\n", m)
	}
	c.w("var rbNilClassObj = &rbNilClass{}\n\n")
	c.w("func rbClassOf(a any) ClassI {\n\tswitch v := a.(type) {\n\tcase nil:\n\t\treturn rbNilClassObj\n")
	for _, cls := range c.classList {
		switch {
		case cls.universal || cls.IsModule || cls.meta == nil:
		case cls.RubyName == "Boolean":
			c.w("\tcase Boolean:\n\t\treturn rbBoolClass(v)\n")
		case cls.GoType != "" && len(cls.TypeParams) > 0:
			c.w("\tcase %s:\n\t\treturn %s\n", cls.anyName(), classVar(cls))
		case cls.GoType != "":
			c.w("\tcase %s:\n\t\treturn %s\n", c.goType(TClass{C: cls}), classVar(cls))
		case cls.root() == cls && len(cls.TypeParams) == 0:
			// a metaclass's interface lacks the _ClassOf its struct inherits
			c.w("\tcase %s:\n\t\treturn v._ClassOf().(ClassI)\n", c.goType(TClass{C: cls}))
		}
	}
	if obj := c.classes["Object"]; obj.meta != nil { // main and Object.new: universal, so the loop skipped it
		c.w("\tcase *Object:\n\t\treturn %s\n", classVar(obj))
	}
	c.w("\t}\n")
	if proc := c.classes["Proc"]; proc.meta != nil { // a func type has no methods to switch on
		c.w("\tif rbIsProc(a) {\n\t\treturn %s\n\t}\n", classVar(proc))
	}
	// ponytail: generic struct classes fall to NoMethodError; give them an _Any interface to switch on
	c.w("\tpanic(rbNoMethod(\"class\", a, false))\n}\n\n")
}

// emitDelegateObj emits rbDelegateObj(x): the object a Delegator (SimpleDelegator, a LazyObject's evaluated block, ...)
// stands for, through its own __getobj__, so untyped code calls, iterates and splats what MRI's method_missing would
// reach (decision 295). Each delegator class gets a _DelegateObj method; with none, rbDelegateObj answers false.
func (c *Compiler) emitDelegateObj() {
	for _, cls := range c.classList {
		if cls.IsModule || cls.metaOf != nil || !c.isDelegator(cls) {
			continue
		}
		e := cls.lookup("__getobj__")
		if e == nil {
			continue
		}
		body := c.dynWrapperBodyOrWarn(cls, e, "__getobj__")
		if body == "" {
			continue
		}
		c.w("func (self %s) _DelegateObj() any {\n\tvar args []any\n%s}\n\n", c.recvType(cls), body)
	}
	// a tuple in untyped code is an Array there (decision 22): `.map(&:last)` over Homebrew's [type, line] output pairs
	c.w("func rbDelegateObj(x any) (any, bool) {\n\tswitch d := x.(type) {\n\tcase interface{ _DelegateObj() any }:\n\t\treturn d._DelegateObj(), true\n\tcase interface{ _Tuple() *Array[any] }:\n\t\treturn d._Tuple(), true\n\t}\n\treturn nil, false\n}\n\n")
}

// emitDynEachV emits rbEachAnyV: rbEachAny for a forwarded block whose value matters, and each's own value back (prime's
// `generator.each(&block)` over an untyped generator whose each answers `break last_value`; decision 418). A gem class's
// closure each with an untyped block gets the block's values; anything else is rbEachAny's, valued its receiver.
func (c *Compiler) emitDynEachV() {
	if !c.dynEachV {
		return
	}
	c.w("func rbEachAnyV(recv any, arity int, blk func(...any) any) any {\n\tswitch r := recv.(type) {\n")
	seen := map[string]bool{}
	for _, cls := range c.classList {
		m := cls.Methods["each"]
		if m == nil || m.File == nil || m.File.isPrelude() || m.Iterator || m.Block == nil || len(m.Params) > 0 || m.generic() ||
			len(cls.TypeParams) > 0 || len(m.Block.Params) != 1 || !isAny(m.Block.Ret) || m.Block.Self != nil || m.Block.Rest {
			continue
		}
		params, ret := c.sig(m, nil)
		shape := "interface{ " + m.GoName + "(" + params + ") " + ret + " }"
		if seen[shape] || ret == "" {
			continue
		}
		seen[shape] = true
		c.w("\tcase %s:\n\t\treturn r.%s(func(a %s) any { return blk(a) })\n", shape, m.GoName, c.goType(m.Block.Params[0]))
	}
	c.w("\tdefault:\n\t}\n\treturn rbEachAny(recv, arity, func(xs ...any) { blk(xs...) })\n}\n\n")
}

// emitDynEach emits rbEachAny (dynEach): an Array's elements, a Hash's pairs, then one case per Go shape of a user or
// gem class's own each; a block of two splits a lone Array value and a block of one gets a pair as an Array, as yield does.
func (c *Compiler) emitDynEach() {
	if !c.dynEach {
		return
	}
	c.w("func rbEachAny(recv any, arity int, blk func(...any)) any {\n")
	c.w("\tif o, ok := rbDelegateObj(recv); ok { // a Delegator's each is its object's (Homebrew's Dependencies)\n\t\trbEachAny(o, arity, blk)\n\t\treturn recv\n\t}\n")
	c.w("\tyield := func(xs ...any) {\n\t\tswitch {\n")
	c.w("\t\tcase arity >= 2 && len(xs) == 1:\n\t\t\ta := rbToAry(xs[0]).s\n\t\t\txs = append(a[:len(a):len(a)], make([]any, arity)...)[:arity]\n")
	c.w("\t\tcase arity == 1 && len(xs) == 2:\n\t\t\txs = []any{&Array[any]{s: xs}}\n\t\t}\n\t\tblk(xs...)\n\t}\n")
	c.w("\tswitch r := recv.(type) {\n\tcase Array_Any:\n\t\tfor _, x := range r._ToAny().s {\n\t\t\tyield(x)\n\t\t}\n")
	c.w("\tcase Hash_Any:\n\t\th := r._ToAny()\n\t\tfor k, v := range rbHashEachKV(h) {\n\t\t\tyield(k, v)\n\t\t}\n")
	// an untyped Enumerator (matrix's `each(:strict_upper).all?`, decision 414); the pruner drops the case when nothing makes one
	c.w("\tcase Enumerator_Any:\n\t\tfor x := range r._ToAny().seq {\n\t\t\tyield(x)\n\t\t}\n")

	seen := map[string]bool{}
	viaDyn := false // an each no shape below fits (a forwarder's `each(*args, &block)`, rubocop's Config): through its Dyn wrapper (decision 444)
	for _, cls := range c.classList {
		m := cls.Methods["each"]
		if m != nil && m.File != nil && !m.File.isPrelude() && m.Block != nil && !m.Iterator && (len(m.Params) > 0 || m.Block.Rest) && len(cls.TypeParams) == 0 {
			viaDyn = true
		}
		if m == nil || m.File == nil || m.File.isPrelude() || m.Block == nil || len(m.Params) > 0 || m.generic() || len(cls.TypeParams) > 0 ||
			len(m.Block.Params) == 0 || len(m.Block.Params) > 2 || !isVoid(m.Block.Ret) && (m.Iterator || !isAny(m.Block.Ret)) || m.Block.Self != nil || m.Block.Rest {
			continue
		}
		if slices.ContainsFunc(m.Block.Params, mentionsVar) {
			continue
		}
		params, ret := c.sig(m, nil)
		shape := "interface{ " + m.GoName + "(" + params + ") " + ret + " }" // Each_blk when a seq adapter is its Each (decision 418)
		if seen[shape] {
			continue
		}
		seen[shape] = true
		vars := []string{"a", "b"}[:len(m.Block.Params)]
		c.w("\tcase %s:\n", shape)
		if m.Iterator {
			c.w("\t\t%s\n\t\t\tyield(%s)\n\t\t}\n", rangeHead(strings.Join(vars, ", "), "r.Each()"), strings.Join(vars, ", "))
			continue
		}
		typed := make([]string, len(vars))
		for i, v := range vars {
			typed[i] = v + " " + c.goType(m.Block.Params[i])
		}
		if isAny(m.Block.Ret) { // a gem each that reads its block's value (prime's `last_value = yield prime`): a void block's is nil
			c.w("\t\tr.%s(func(%s) any { yield(%s); return nil })\n", m.GoName, strings.Join(typed, ", "), strings.Join(vars, ", "))
			continue
		}
		c.w("\t\tr.%s(func(%s) { yield(%s) })\n", m.GoName, strings.Join(typed, ", "), strings.Join(vars, ", "))
	}
	if viaDyn {
		c.noteDyn("each")
		if c.dynBlockNames == nil {
			c.dynBlockNames = map[string]bool{}
		}
		c.dynBlockNames["each"] = true
		c.w("\tcase interface{ DynEach(...any) any }:\n\t\tr.DynEach(rbBlockArg{fn: func(xs ...any) any { yield(xs...); return nil }})\n")
	}
	c.w("\tdefault:\n\t\tpanic(rbNoMethod(\"each\", recv, false))\n\t}\n\treturn recv\n}\n\n")
	c.emitDynEachV()
	// rbElemsAny is an untyped receiver's elements as an Array[untyped] (decision 224): an Array's own, else what its each yields
	c.w("func rbElemsAny(recv any) *Array[any] {\n")
	c.w("\tif o, ok := rbDelegateObj(recv); ok {\n\t\trecv = o\n\t}\n")
	c.w("\tif a, ok := recv.(Array_Any); ok {\n\t\treturn a._ToAny()\n\t}\n")
	c.w("\tout := &Array[any]{}\n\trbEachAny(recv, 1, func(xs ...any) { out.s = append(out.s, xs[0]) })\n\treturn out\n}\n\n")
	// rbZipAny is zip on untyped values: each element with the others' at its index, nil past their ends
	c.w("func rbZipAny(recv any, others ...any) *Array[any] {\n\tout := &Array[any]{}\n\tos := make([]*Array[any], len(others))\n\tfor i, o := range others {\n\t\tos[i] = rbElemsAny(o)\n\t}\n")
	c.w("\tfor i, x := range rbElemsAny(recv).s {\n\t\trow := &Array[any]{s: []any{x}}\n\t\tfor _, o := range os {\n\t\t\tvar v any\n\t\t\tif i < len(o.s) {\n\t\t\t\tv = o.s[i]\n\t\t\t}\n\t\t\trow.s = append(row.s, v)\n\t\t}\n\t\tout.s = append(out.s, row)\n\t}\n\treturn out\n}\n\n")
	// rbEachEnumAny is a blockless each on an untyped receiver: an Enumerator over its elements
	c.w("func rbEachEnumAny(recv any) any {\n\ta := rbElemsAny(recv)\n\treturn rbEnumOf(Array_Each(a), recv, \"each\", rbSizeOf(Integer(len(a.s))), nil)\n}\n\n")
}
