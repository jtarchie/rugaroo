package compiler

import (
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
	"strings"
)

// maxTuple is the widest RBS tuple: Socket.getaddrinfo's rows have 7 elements (decision 135).
const maxTuple = 7

// emitTuples emits every arity: the pruner keeps the ones kept code names (decision 432).
func (c *Compiler) emitTuples() {
	for n := 2; n <= maxTuple; n++ {
		tps := make([]string, 0, n)
		fields := make([]string, 0, n)
		cmp := make([]string, 0, n)
		vals, anys := make([]string, 0, n), make([]string, 0, n)
		insp := make([]string, 0, n)
		eq := make([]string, 0, n)
		from := make([]string, 0, n)
		plain := make([]string, 0, n)
		for i := range n {
			p := fmt.Sprintf("T%d", i)
			plain = append(plain, "rbPlainKey["+p+"]()")
			tps = append(tps, p)
			fields = append(fields, fmt.Sprintf("F%d %s", i, p))
			cmp = append(cmp, fmt.Sprintf("if c := rbCmp(t.F%d, o.F%d); c != 0 {\n\t\treturn c\n\t}", i, i))
			vals = append(vals, fmt.Sprintf("t.F%d", i))
			anys = append(anys, fmt.Sprintf("rbUnbox(t.F%d)", i)) // a T? field (a *T box) reaches untyped as its value or nil, as an Array's does (decision 392)
			insp = append(insp, fmt.Sprintf("rbInspect(t.F%d)", i))
			eq = append(eq, fmt.Sprintf("rbEq(t.F%d, o2.F%d)", i, i))
			from = append(from, fmt.Sprintf("\tif t.F%d, ok = rbConv[T%d](s[%d]); !ok {\n\t\treturn t, false\n\t}", i, i, i)) // elements convert as an Array's do
		}
		name := fmt.Sprintf("Tuple%d", n)
		c.w("type %s[%s comparable] struct {\n\t%s\n}\n\n", name, strings.Join(tps, ", "), strings.Join(fields, "\n\t"))
		full := fmt.Sprintf("%s[%s]", name, strings.Join(tps, ", "))
		c.w("func (t %s) Op_cmp(o %s) Integer {\n\t%s\n\treturn 0\n}\n\n", full, full, strings.Join(cmp, "\n\t"))
		c.w("func (%s) rbCmpPair(t, o %s) Integer { return t.Op_cmp(o) }\n\n", full, full) // rbCmpFunc: no boxing per comparison
		c.w("func (t %s) Inspect() String { return \"[\" + %s + \"]\" }\n\n", full, strings.Join(insp, ` + ", " + `))
		c.w("func (t %s) ToS() String { return t.Inspect() }\n\n", full)
		c.w("func (t %s) ToJson(state ...any) String { return rbJSONArray([]any{%s}, state) }\n\n", full, strings.Join(vals, ", "))
		// a tuple that reaches untyped answers as an Array (decision 22)
		c.w("func (t %s) _ToAny() *Array[any] { return %s }\n\n", full, arrayLit("any", anys))
		c.w("func (t %s) _Tuple() *Array[any] { return t._ToAny() }\n\n", full) // what a dynamic call on it answers through (rbDelegateObj)
		// and converts back where a dynamic call's parameter is a tuple (rbAs)
		c.w("func (%s) _FromAny(a any) (t %s, ok bool) {\n\tarr, ok := a.(Array_Any)\n\tif !ok {\n\t\treturn t, false\n\t}\n\ts := arr._ToAny().s\n\tif len(s) != %d {\n\t\treturn t, false\n\t}\n%s\n\treturn t, true\n}\n\n", full, full, n, strings.Join(from, "\n"))
		c.w("func (t %s) Op_eq(o any) Boolean {\n\to2, ok := o.(%s)\n\tif !ok {\n\t\tif a, isArr := o.(Array_Any); isArr {\n\t\t\treturn t._ToAny().Op_eq(a._ToAny())\n\t\t}\n\t\treturn false\n\t}\n\treturn %s\n}\n\n", full, full, strings.Join(eq, " && "))
		c.w("func (t %s) rbPlain() bool { return %s }\n\n", full, strings.Join(plain, " && "))
	}
}

// unionFn names the generated helpers for union u (decision 150): a hash of
// its members, so the name does not depend on what the program compiled first.
func (c *Compiler) unionFn(u TUnion) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(u.String())) // a hash.Hash never fails
	name := fmt.Sprintf("rbUnion_%016x", h.Sum64())
	if old, ok := c.unions[name]; ok && old.String() != u.String() {
		panic(compileError{msg: "rugaroo: union helper name collision: " + old.String() + " and " + u.String()}) // never silently share one checker
	}
	c.unions[name] = u
	return name
}

// emitUnions emits, per union the program converts into or out of:
// rbUnion_<hash>(a) checks an untyped value is one of the members (MRI's
// TypeError otherwise), converting an Array or Hash of another
// instantiation as rbAs does; rbUnion_<hash>Out(a) shows a tuple member to
// untyped code as the Array it is (decision 22).
func (c *Compiler) emitUnions(reached func(string) bool) {
	for _, name := range slices.Sorted(maps.Keys(c.unions)) {
		if c.unionsOut[name] || !reached(name) && !reached(name+"Out") {
			continue
		}
		c.unionsOut[name] = true
		u := c.unions[name]
		exact := make([]string, 0, len(u.Members))
		var conv, tuples []string
		for _, m := range u.Members {
			g := "nil"
			if !isNil(m) {
				g = c.goType(m)
			}
			exact = append(exact, g)
			switch m := m.(type) {
			case TTuple:
				conv = append(conv, g)
				tuples = append(tuples, g)
			case TClass:
				if len(m.Args) > 0 {
					conv = append(conv, g)
				}
			case TAny, TFunc, TNil, TOpt, TUnion, TVar, TVoid: // matched exactly or not at all
			}
		}
		subs := c.valueSubclassArms(u)
		sw := "switch a.(type)"
		if subs != "" {
			sw = "switch v := a.(type)"
		}
		c.w("func %s(a any) any {\n\t%s {\n\tcase %s:\n\t\treturn a\n%s\t}\n", name, sw, strings.Join(exact, ", "), subs)
		for _, g := range conv {
			c.w("\tif v, ok := rbConv[%s](a); ok {\n\t\treturn v\n\t}\n", g)
		}
		c.w("\tpanic(rbConvError(a, %q))\n}\n\n", u.String())
		if len(tuples) > 0 {
			c.w("func %sOut(a any) any {\n\tswitch v := a.(type) {\n", name)
			for _, g := range tuples {
				c.w("\tcase %s:\n\t\treturn v._ToAny()\n", g)
			}
			c.w("\t}\n\treturn a\n}\n\n")
		}
	}
}

// valueSubclassArms are a union's cases for a subclass of a value member (Homebrew's `Cask::DSL::Version < String`
// into `String | Version`): MRI's object is a String, so the union holds its String (decision 374).
func (c *Compiler) valueSubclassArms(u TUnion) string {
	var b strings.Builder
	for _, m := range u.Members {
		mc, ok := m.(TClass)
		if !ok || mc.C.isStruct() || mc.C.IsModule || c.goType(m) != mc.C.Name {
			continue
		}
		for _, k := range c.classList {
			if k.Super == mc.C && k.metaOf == nil && !slices.ContainsFunc(u.Members, func(x Type) bool { xc, ok := x.(TClass); return ok && xc.C == k }) { // a member itself is matched exactly: a second arm would be a duplicate case
				fmt.Fprintf(&b, "\tcase *%s:\n\t\treturn v.%s\n", k.Name, superField(mc.C))
			}
		}
	}
	return b.String()
}

// emitBoxes emits the helpers that open a T? box (*T) seen as `any`: generic
// code holding E = T? hands the box itself to rbInspect, rbEq, rbCmp...,
// where a nil *T is not a nil interface and a non-nil one has the wrong
// method set. Only concrete T? types rendered somewhere can reach them.
func (c *Compiler) emitBoxes() {
	boxes := slices.Sorted(maps.Keys(c.boxes))
	all := maps.Clone(c.argBoxes)
	maps.Copy(all, c.boxes)
	unbox := slices.Sorted(maps.Keys(all))
	leaves := slices.Sorted(maps.Keys(c.leafBoxes))
	c.w("func rbUnbox(a any) any {\n\tswitch v := a.(type) {\n")
	for _, b := range leaves { // generic code holding E = C? of a leaf: a nil *C is nil, any other the object (decision 360)
		c.w("\tcase %s:\n\t\tif v == nil {\n\t\t\treturn nil\n\t\t}\n", b)
	}
	for _, b := range unbox {
		if all[b] {
			c.w("\tcase %s:\n\t\treturn rbUnbox(Opt(v))\n", b)
		} else {
			c.w("\tcase %s:\n\t\treturn Opt(v)\n", b)
		}
	}
	c.w("\tcase rbOpt[Integer]:\n\t\treturn v.rbAny()\n\tcase rbOpt[Float]:\n\t\treturn v.rbAny()\n") // Integer?/Float? held by value (decision 396)
	c.w("\t}\n\treturn a\n}\n\n")
	// rbKeyUnbox opens one level of a box used as a Hash key or in uniq/tally, which match by what it holds
	c.w("func rbKeyUnbox(k any) (any, bool) {\n\tswitch v := k.(type) {\n")
	for _, b := range leaves {
		c.w("\tcase %s:\n\t\tif v == nil {\n\t\t\treturn nil, true\n\t\t}\n", b)
	}
	for _, b := range unbox {
		c.w("\tcase %s:\n\t\treturn Opt(v), true\n", b)
	}
	c.w("\tcase rbOpt[Integer]:\n\t\treturn v.rbAny(), true\n\tcase rbOpt[Float]:\n\t\treturn v.rbAny(), true\n")
	c.w("\t}\n\treturn nil, false\n}\n\n")
	c.w("func rbCmpBox(a, b any) Integer {\n\tswitch v := a.(type) {\n")
	for _, b := range boxes {
		c.w("\tcase %s:\n\t\treturn rbCmpOpt(v, b.(%s))\n", b, b)
	}
	c.w("\tcase rbOptCmp:\n\t\treturn v.rbCmpWith(b)\n")
	c.w("\t}\n\treturn rbCmpFailed(a, b)\n}\n\n")
	// Marshal.load fills a T? slot of generic code (rbMAs): a box of what it holds
	c.w("func rbMBox(dst, v any) bool {\n\tswitch d := dst.(type) {\n\tcase nil:\n\t\treturn false\n")
	for _, b := range unbox {
		c.w("\tcase *%s:\n\t\tx := rbMAs[%s](v)\n\t\t*d = &x\n", b, b[1:])
	}
	c.w("\tcase rbOptMSet:\n\t\td.rbMSet(v)\n")
	c.w("\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n")
}
