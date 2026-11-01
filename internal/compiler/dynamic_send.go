package compiler

import (
	"maps"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// emitMarkers emits rbHas<marker>(recv), true for the classes given: one
// switch on the class ID, where a marker method was a function per class
// (and per instantiation of a generic one), kept as a stub whenever the
// assertion was (decisions 87, 209). Boolean keeps a method: no value
// answers its ID.
func (c *Compiler) emitMarkers(marker string, classes []*Class) {
	if c.markers[marker] {
		return
	}
	c.markers[marker] = true
	var ids []string
	methods := false
	for _, cls := range classes {
		if cls == c.classes["Boolean"] { // true and false answer TrueClass's and FalseClass's IDs, never Boolean's
			c.w("func (self %s) %s() {}\n\n", c.recvType(cls), marker)
			methods = true
		} else {
			ids = append(ids, strconv.Itoa(c.classID(cls)))
		}
	}
	c.w("func rbHas%s(recv any) bool {\n", marker)
	if methods {
		c.w("\tif _, ok := recv.(interface{ %s() }); ok {\n\t\treturn true\n\t}\n", marker)
	}
	if len(ids) > 0 {
		c.w("\tif id, ok := recv.(interface{ _ClassID() int }); ok {\n")
		c.w("\t\tswitch id._ClassID() {\n\t\tcase %s:\n\t\t\treturn true\n\t\tdefault:\n\t\t}\n\t}\n", strings.Join(ids, ", "))
	}
	c.w("\treturn false\n}\n\n")
}

// emitRespond emits rbRespondsName(recv, priv), backed by a
// marker method on every class that has the public method (with or without
// a block) and, when private methods count, the _PrivateName markers.
func (c *Compiler) emitRespond(name string) {
	gn := c.dynGoName(name)
	var public []*Class
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal || rubyPrivate[name] {
			continue
		}
		if e := cls.lookup(name); e != nil && !e.M.Private {
			public = append(public, cls)
		}
	}
	c.emitMarkers("_Responds"+gn, public)
	c.w("func rbResponds%s(recv any, priv bool) Boolean {\n", gn)
	c.w("\tif rbHas_Responds%s(recv) {\n\t\treturn true\n\t}\n", gn)
	if procCallNames[name] || name == "arity" || name == "to_proc" || name == "lambda?" { // a Proc is a Go func: no class table (liquid's `operation.respond_to?(:call)`, decision 331)
		c.w("\tif rbIsProc(recv) {\n\t\treturn true\n\t}\n")
	}
	if len(c.privateIn(name)) > 0 {
		c.w("\tif priv && rbHas_Private%s(recv) {\n\t\treturn true\n\t}\n", gn)
	}
	delete(c.dynLazy, "respond_to_missing?") // the fallback below needs its wrappers
	c.w("\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(%q), Boolean(priv))))\n\t}\n", name)
	c.w("\treturn false\n}\n\n")
}

// allMethodNames lists every method name a computed send may name.
func (c *Compiler) allMethodNames(gemOnly bool) []string {
	seen := map[string]bool{}
	lits := c.symbolLiterals()
	for _, cls := range c.classList {
		if cls.IsModule || cls.universal {
			continue
		}
		preludeOnly := gemOnly && cls.File != nil && cls.File.isPrelude() // its names count only as symbol literals in gem or user code (decision 235)
		for _, e := range cls.methodSet() {
			helper := strings.HasPrefix(e.M.Name, "_") && e.M.File != nil && e.M.File.isPrelude() // the views' accessors, _to_any ...
			if e.M.Name != "initialize" && !strings.HasPrefix(e.M.Name, "__") && !helper && (!preludeOnly || lits[e.M.Name]) {
				seen[e.M.Name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

var symbolLiteral = regexp.MustCompile(`(?:^|[^:\w]):([A-Za-z_]\w*[?!=]?|[<>=!~+\-*/%^&|\[\]]+@?)|%i\[([^\]\n]*)\]`)

// symbolLiterals is every symbol written in a gem or user file (`:name`, `&:name`, `%i[a b]`): what a computed send
// there can name without building a string. Over-approximate by regex; a wrong extra name only widens the set.
func (c *Compiler) symbolLiterals() map[string]bool {
	if c.symLits != nil {
		return c.symLits
	}
	c.symLits = map[string]bool{}
	word := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*[?!=]?|[<>=!~+\-*/%^&|\[\]]+@?`)
	for _, f := range c.files {
		if f.isPrelude() {
			continue
		}
		for _, m := range symbolLiteral.FindAllSubmatch(f.Src, -1) {
			for _, w := range word.FindAll(append(m[1], m[2]...), -1) { // `:name`, `&:name` or `%i[a b]` (one group is empty)
				c.symLits[string(w)] = true
			}
		}
	}
	return c.symLits
}

// emitNameSwitches backs send and respond_to? with computed names.
func (c *Compiler) emitNameSwitches() {
	c.w("func rbSendByName(recv any, name string, how int, args ...any) any {\n\tswitch name {\n")
	for _, name := range slices.Sorted(slices.Values(c.dynNames)) { // name order: noting order follows what discarded emissions noted (decision 432)
		c.w("\tcase %q:\n\t\treturn rbDyn%s(how, recv, args...)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
	c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(name)}, args...)...)\n\t}\n")
	c.w("\tpanic(rbNoMethod(name, recv, false))\n}\n\n")
	c.w("func rbRespondsByName(recv any, name string, priv bool) Boolean {\n\tswitch name {\n")
	for _, name := range slices.Sorted(slices.Values(c.respondNames)) {
		c.w("\tcase %q:\n\t\treturn rbResponds%s(recv, priv)\n", name, goMethodName(name))
	}
	c.w("\t}\n\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
	c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(name), Boolean(priv))))\n\t}\n")
	c.w("\treturn false\n}\n\n")
}

// noteSendOn asks for rbSendOn_K and rbRespondsOn_K, K the Go names of the struct classes a computed send or
// respond_to?'s receiver may hold (decision 316), and returns K. The switch covers what those classes and their
// subclasses answer, so only those names need dispatchers, not every name in the program (rbSendByName).
func (c *Compiler) noteSendOn(classes []*Class) string {
	slices.SortFunc(classes, func(a, b *Class) int { return strings.Compare(a.Name, b.Name) })
	classes = slices.CompactFunc(classes, func(a, b *Class) bool { return a == b })
	names := make([]string, 0, len(classes))
	for _, cls := range classes {
		names = append(names, cls.Name)
	}
	key := strings.Join(names, "_")
	if _, ok := c.sendOn[key]; ok {
		return key
	}
	if c.sendOn == nil {
		c.sendOn = map[string][]*Class{}
	}
	c.sendOn[key] = classes
	for _, name := range c.sendOnNames(classes) {
		c.noteDyn(name)
		c.noteRespond(name)
	}
	return key
}

// sendOnNames lists what an instance of one of classes or a subclass answers, filtered as allMethodNames does.
func (c *Compiler) sendOnNames(classes []*Class) []string {
	seen := map[string]bool{}
	var walk func(k *Class)
	walk = func(k *Class) {
		for _, e := range k.methodSet() {
			helper := strings.HasPrefix(e.M.Name, "_") && e.M.File != nil && e.M.File.isPrelude()
			if e.M.Name != "initialize" && !strings.HasPrefix(e.M.Name, "__") && !helper {
				seen[e.M.Name] = true
			}
		}
		for _, sub := range k.Subclasses {
			walk(sub)
		}
	}
	for _, cls := range classes {
		walk(cls)
	}
	return slices.Sorted(maps.Keys(seen))
}

// emitSendOn emits each noted class's rbSendOn_C and rbRespondsOn_C (decision 316): rbSendByName and rbRespondsByName
// over that class's names only, with the same method_missing and respond_to_missing? fallbacks.
func (c *Compiler) emitSendOn() {
	for _, key := range slices.Sorted(maps.Keys(c.sendOn)) {
		names := c.sendOnNames(c.sendOn[key])
		c.w("func rbSendOn_%s(recv any, name string, how int, args ...any) any {\n", key)
		c.emitNameCases(names, func(n string) string { return "rbDyn" + goMethodName(n) + "(how, recv, args...)" })
		c.w("\tif r, ok := recv.(interface{ DynMethodMissing(...any) any }); ok {\n")
		c.w("\t\treturn r.DynMethodMissing(append([]any{Symbol(name)}, args...)...)\n\t}\n")
		c.w("\tpanic(rbNoMethod(name, recv, false))\n}\n\n")
		c.w("func rbRespondsOn_%s(recv any, name string, priv bool) Boolean {\n", key)
		c.emitNameCases(names, func(n string) string { return "rbResponds" + goMethodName(n) + "(recv, priv)" })
		c.w("\tif r, ok := recv.(interface{ DynRespondToMissingQ(...any) any }); ok {\n")
		c.w("\t\treturn Boolean(rbTruthy(r.DynRespondToMissingQ(Symbol(name), Boolean(priv))))\n\t}\n")
		c.w("\treturn false\n}\n\n")
	}
}

// emitNameCases writes `switch name` returning ret(name) per case; an `if` for one name, nothing for none (gocritic).
func (c *Compiler) emitNameCases(names []string, ret func(string) string) {
	switch len(names) {
	case 0:
	case 1:
		c.w("\tif name == %q {\n\t\treturn %s\n\t}\n", names[0], ret(names[0]))
	default:
		c.w("\tswitch name {\n")
		for _, n := range names {
			c.w("\tcase %q:\n\t\treturn %s\n", n, ret(n))
		}
		c.w("\t}\n")
	}
}
