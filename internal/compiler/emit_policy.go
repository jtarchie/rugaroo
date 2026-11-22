package compiler

import (
	"slices"
)

// wantsForwarder decides whether class cls gets a Go method forwarding to
// the free func of entry e. Struct classes forward everything (their
// interface is the full method set). Primitive classes only forward what a
// module's constraint needs: a forwarder on a generic container can
// instantiate ever-growing types (Hash[K,V].Tally → Hash[[K,V],Integer]...),
// which Go rejects as an instantiation cycle.
func (c *Compiler) wantsForwarder(cls *Class, e entry) bool {
	if growsOwnClass(cls, e) {
		return false // a Go method on Lazy[E] returning Lazy[[E, Integer]] is an instantiation cycle; calls reach the free func (decision 207)
	}
	if c.dispatchableAll(cls, e.M.Name) {
		return false // calls go to the free func or an rbSend_ type switch; no interface declares the name (dispatch.go)
	}
	if e.Owner == cls {
		if len(cls.TypeParams) == 0 || cls.GoType == "" {
			return true
		}
		// a generic primitive's own method is a Go method only where something reaches it through a Go method set:
		// the std library, an included module's self calls, or a universal name (inspect, ==, hash) an ObjectI or
		// rbEq assertion selects; typed calls already reach the free func (decision 86), so Array[E] keeps a fixed,
		// small method set per instantiation instead of one per name (#96 phase 3b)
		return stdMethodNames[e.M.GoName] || genericProtocol[e.M.Name] || c.includerCalls(cls, e.M.Name) || c.universalHas(e.M.Name)
	}
	if cls.isStruct() {
		return true
	}
	if e.Owner.universal {
		return c.includerCalls(cls, e.M.Name)
	}
	return e.Owner.IsModule && c.selfCalls(e.Owner)[e.M.Name] && !c.dispatchable(e.Owner, e.M)
}

// genericProtocol are the names Go code asserts or calls as methods on a generic primitive of unknown instantiation.
var genericProtocol = map[string]bool{
	"_to_any": true, "_from_any": true, "_eq_rec": true, "_eql_rec": true, "_hash_rec": true, "_inspect_rec": true,
	"==": true, "eql?": true, "hash": true, "<=>": true, "each": true, "succ": true, "to_s": true, "inspect": true,
	"to_json": true, "pretty_print": true,
	"_len": true, "_at": true, "_set": true, "_make": true, "_append": true, "_resize": true, "_drop": true, // the untyped views (decision 236)
	"_keys_any": true, "_pairs_any": true, "_has": true, "_get": true, "_store": true, "_delete": true, "_dup": true,
}

// universalHas reports whether a universal owner (Kernel, Object, BasicObject) defines name, so a class's own method of
// that name overrides a protocol every object answers.
func (c *Compiler) universalHas(name string) bool {
	if c.universalNames == nil {
		c.universalNames = map[string]bool{}
		for _, k := range c.classes {
			if !k.universal {
				continue
			}
			for _, e := range k.methodSet() {
				c.universalNames[e.M.Name] = true
			}
		}
	}
	return c.universalNames[name]
}

// growsOwnClass reports a generic class's method whose signature names the class at a type built from its params (Lazy[[E, Integer]] on Lazy[E]).
func growsOwnClass(cls *Class, e entry) bool {
	if len(cls.TypeParams) == 0 {
		return false
	}
	var grows func(t Type) bool
	grows = func(t Type) bool {
		switch t := t.(type) {
		case TClass:
			if t.C == cls && mentionsVar(t) {
				for i, a := range t.Args {
					if v, ok := a.(TVar); !ok || i >= len(cls.TypeParams) || v.Name != cls.TypeParams[i] {
						return true
					}
				}
			}
			return slices.ContainsFunc(t.Args, grows)
		case TOpt:
			return grows(t.Elem)
		case TTuple:
			return slices.ContainsFunc(t.Elems, grows)
		case TFunc:
			return slices.ContainsFunc(t.Params, grows) || t.Ret != nil && grows(t.Ret)
		case TUnion:
			return slices.ContainsFunc(t.Members, grows)
		case TAny, TNil, TVar, TVoid:
		}
		return false
	}
	ts := []Type{e.M.Ret}
	for _, p := range e.M.Params {
		ts = append(ts, p.Type)
	}
	if b := e.M.Block; b != nil {
		ts = append(ts, b.Params...)
		ts = append(ts, b.Ret)
	}
	return slices.ContainsFunc(ts, func(t Type) bool { return t != nil && grows(subst(t, e.Env)) })
}

// includerCalls: a module method calling name on self needs cls to answer it.
func (c *Compiler) includerCalls(cls *Class, name string) bool {
	return c.includerCaller(cls, name) != nil
}

// includerCaller is the module included by cls (or an ancestor) whose constraint declares name, or nil.
func (c *Compiler) includerCaller(cls *Class, name string) *Class {
	if c.universalDispatch(name) {
		return nil // the module body dispatches it (dispatch.go), so its constraint does not declare it
	}
	for k := cls; k != nil && !k.universal; k = k.Super {
		for _, inc := range k.Includes {
			if !c.selfCalls(inc.Mod)[name] {
				continue
			}
			if e := inc.Mod.lookup(name); e != nil && e.Owner.IsModule && c.dispatchable(e.Owner, e.M) {
				continue // the module body dispatches it by type switch (dispatch.go); its constraint does not declare it
			}
			if c.notModules(inc.Mod, name) {
				continue // the module does not answer it: its self call is the includers' owner's (includerRoot) or dynamic, never through its constraint (decision 486)
			}
			return inc.Mod
		}
	}
	return nil
}
