package compiler

import (
	"fmt"
	"slices"
	"strings"
)

// marshalKinds are the generic @go_type classes Marshal round-trips; the others (Enumerator, Queue) MRI cannot dump either.
var marshalKinds = map[string]bool{"Array": true, "Hash": true, "Set": true, "Range": true}

// noteMarshal records a value type Marshal may meet as its exact Go type (decision 137); one with a type variable is generic code's, never a value's.
func (c *Compiler) noteMarshal(t Type) {
	switch t := t.(type) {
	case TOpt:
		c.noteMarshal(t.Elem)
		return
	case TClass:
		if len(t.Args) == 0 || !marshalKinds[t.C.Name] {
			return
		}
	case TTuple:
	default:
		return
	}
	var vars []string
	if freeVars(t, &vars); len(vars) > 0 {
		return
	}
	if k := marshalKey(t); c.marshalSeen[k] == nil {
		c.marshalSeen[k] = t
	}
}

// marshalKey: by Go class name, since two namespaces' classes may share a Ruby name and String() shows only that.
func marshalKey(t Type) string {
	switch t := t.(type) {
	case TClass:
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			parts[i] = marshalKey(a)
		}
		return t.C.Name + "[" + strings.Join(parts, ",") + "]"
	case TTuple:
		parts := make([]string, len(t.Elems))
		for i, e := range t.Elems {
			parts[i] = marshalKey(e)
		}
		return "(" + strings.Join(parts, ",") + ")"
	case TOpt:
		return marshalKey(t.Elem) + "?"
	default:
		return t.String()
	}
}

// Marshal's generated cases name their types weakly, so pruning keeps the program's own; nil/"" and default keep gocritic's singleCaseSwitch quiet.
const (
	marshalDumpHead = "\tswitch x := v.(type) {\n\tcase nil:\n\t\treturn false\n"
	marshalDumpTail = "\tdefault:\n\t\treturn false\n\t}\n\treturn true\n}\n\n"
	marshalLoadHead = "\tswitch key {\n\tcase \"\":\n\t\treturn nil, false\n"
	marshalLoadTail = "\tdefault:\n\t\treturn nil, false\n\t}\n}\n\n"
)

// prepareMarshal renders rbMDumpGen/rbMLoadGen before the tables, whose boxes and tuples its types add to, for a later round to emit once Marshal is reached.
func (c *Compiler) prepareMarshal() {
	var dump, load strings.Builder
	dump.WriteString("func rbMDumpGen(w *rbMW, v any) bool {\n" + marshalDumpHead)
	load.WriteString("func rbMLoadGen(r *rbMR, key string) (any, bool) {\n" + marshalLoadHead)
	// the untyped forms always: rbMDumpAnyForm writes an instantiation only Go code builds as one
	for _, k := range []string{"Array", "Set", "Range"} {
		c.noteMarshal(TClass{C: c.classes[k], Args: []Type{TAny{}}})
	}
	c.noteMarshal(TClass{C: c.classes["Hash"], Args: []Type{TAny{}, TAny{}}})
	done := map[string]bool{}
	for {
		var todo []string
		for k := range c.marshalSeen {
			if !done[k] {
				todo = append(todo, k)
			}
		}
		if len(todo) == 0 {
			break
		}
		slices.Sort(todo)
		for _, k := range todo {
			done[k] = true
			c.emitMarshalType(&dump, &load, c.marshalSeen[k]) // rendering notes the nested types: the loop takes them next
		}
	}
	for _, cls := range c.classList {
		if hook := cls.lookup("marshal_dump"); hook != nil && cls.metaOf == nil && cls.isStruct() && len(cls.TypeParams) == 0 {
			c.marshalHook(&dump, &load, cls, hook)
		}
	}
	c.marshalCode = dump.String() + marshalDumpTail + load.String() + marshalLoadTail
	c.marshalAt = len(c.marshalSeen)
}

// emitMarshalClasses comes last: only classes reached by then get a case, as thousands of pending cases slow the pruner, and reaching a skipped one later recompiles eagerly.
func (c *Compiler) emitMarshalClasses(reached func(string) bool) {
	use := func(goName string) bool {
		if c.dynEvery || reached(goName) {
			return true
		}
		c.marshalSkipped = append(c.marshalSkipped, goName)
		return false
	}
	var dump, load strings.Builder
	dump.WriteString("func rbMDumpObjGen(w *rbMW, v any) bool {\n" + marshalDumpHead)
	load.WriteString("func rbMLoadObjGen(r *rbMR, key string) (any, bool) {\n" + marshalLoadHead)
	for _, cls := range c.classList {
		if cls.metaOf != nil {
			continue
		}
		tag := cls.displayName()
		if cls.meta != nil && use(cls.meta.Name) {
			fmt.Fprintf(&dump, "\tcase *%s:\n\t\tw.record('c', %q)\n", cls.meta.Name, tag)
			fmt.Fprintf(&load, "\tcase rbKeyed[*%s](%q):\n\t\treturn %s, true\n", cls.meta.Name, "c"+tag, classVar(cls))
		}
		if !cls.isStruct() || cls.universal || cls.byValue || len(cls.TypeParams) > 0 || cls.lookup("marshal_dump") != nil || !use(cls.Name) { // ponytail: a value class does not Marshal; load it by value when one must
			continue
		}
		fmt.Fprintf(&dump, "\tcase *%s:\n\t\trbMDumpObj(w, %q, x)\n", cls.Name, tag)
		fmt.Fprintf(&load, "\tcase rbKeyed[*%s](%q):\n\t\treturn rbMLoadObj(r, &%s{}), true\n", cls.Name, "o"+tag, cls.Name)
	}
	c.w("%s%s%s%s", dump.String(), marshalDumpTail, load.String(), marshalLoadTail)
}

// emitMarshalType: two Ruby types may share a Go type (an optional's box), so cases are keyed by the Go name.
func (c *Compiler) emitMarshalType(dump, load *strings.Builder, t Type) {
	name := strings.TrimPrefix(c.goType(t), "*")
	if c.marshalGo[name] {
		return
	}
	c.marshalGo[name] = true
	if tt, ok := t.(TTuple); ok {
		fields := make([]string, len(tt.Elems))
		conv := make([]string, len(tt.Elems))
		for i, e := range tt.Elems {
			fields[i] = fmt.Sprintf("x.F%d", i)
			conv[i] = fmt.Sprintf("F%d: rbMAs[%s](xs[%d])", i, c.goType(e), i)
		}
		fmt.Fprintf(dump, "\tcase %s:\n\t\trbMDumpTuple(w, %q, %s)\n", name, name, strings.Join(fields, ", "))
		fmt.Fprintf(load, "\tcase rbKeyed[%s](%q):\n\t\txs := rbMTupleItems(r, %d)\n\t\treturn %s{%s}, true\n",
			name, "v"+name, len(tt.Elems), name, strings.Join(conv, ", "))
		return
	}
	ct := t.(TClass)
	kind := ct.C.Name
	fmt.Fprintf(dump, "\tcase *%s:\n\t\trbMDump%s(w, %q, x, x)\n", name, kind, name)
	fmt.Fprintf(load, "\tcase rbKeyed[*%s](%q):\n\t\treturn rbMLoad%s[%s](r), true\n", name, "o"+name, kind, c.goTypes(ct.Args))
}

// marshalHook keeps a bad hook's compile error for when Marshal is reached: a program that never marshals may name a method marshal_dump.
func (c *Compiler) marshalHook(dump, load *strings.Builder, cls *Class, hook *entry) {
	defer func() {
		if r := recover(); r != nil {
			ce, ok := r.(compileError)
			if !ok {
				panic(r)
			}
			if c.marshalErr == nil {
				c.marshalErr = &ce
			}
		}
	}()
	c.emitMarshalHook(dump, load, cls, hook)
}

// emitMarshalHook: the load allocates without initialize, as MRI does, so marshal_load sees a blank object.
func (c *Compiler) emitMarshalHook(dump, load *strings.Builder, cls *Class, hook *entry) {
	tag := cls.displayName()
	if len(hook.M.Params) != 0 || isVoid(hook.M.Ret) {
		c.errorf(hook.M.File, hook.M.Node, "marshal_dump takes no arguments and returns what to dump")
	}
	// the free func, not the method: a private hook has no forwarder, and MRI calls it regardless
	fmt.Fprintf(dump, "\tcase *%s:\n\t\trbMDumpHook(w, %q, x, func() any { return %s })\n", cls.Name, tag, staticCallCode(hook.M, c.forwardTypeArgs(*hook, cls), "x", ""))
	fmt.Fprintf(load, "\tcase rbKeyed[*%s](%q):\n\t\tx := &%s{}\n\t\tr.reg(x)\n", cls.Name, "u"+tag, cls.Name)
	ld := cls.lookup("marshal_load")
	if ld == nil {
		fmt.Fprintf(load, "\t\tpanic(NewTypeError(Ref(String(%q))))\n", "instance of "+tag+" needs to have method 'marshal_load'")
		return
	}
	if len(ld.M.Params) != 1 || ld.M.Params[0].Rest || ld.M.Block != nil {
		c.errorf(ld.M.File, ld.M.Node, "marshal_load takes one argument, the data marshal_dump returned")
	}
	env := composeEnv(ld.Env, nil)
	env["Self"] = TClass{C: cls}
	arg := "rbMAs[" + c.goType(subst(ld.M.Params[0].Type, env)) + "](r.value())"
	fmt.Fprintf(load, "\t\t%s\n\t\treturn x, true\n", staticCallCode(ld.M, c.forwardTypeArgs(*ld, cls), "x", arg))
}
