package compiler

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// loadSigs parses .rbs signature files beside each user source (a `sig/`
// directory next to the file) and the vendored gem signatures (decision 160:
// `sig/gems/<gem>/`), keyed by qualified class name. Inline `#:` annotations
// win; a sig is consulted only where one is missing.
func (c *Compiler) loadSigs(sources []Source) {
	seen := map[string]bool{}
	for _, src := range sources {
		dir := filepath.Join(filepath.Dir(sourceName(src)), "sig")
		if seen[dir] {
			continue
		}
		seen[dir] = true
		c.loadSigDir(dir, false)
	}
	c.loadGemSigs()
	c.loadRBSCollections(sources, seen)
	for _, lib := range c.gemDirs { // a gem's own sig/ beside its lib/ (decision 306), after the vendored ones it may need patching by
		if dir := filepath.Join(filepath.Dir(lib), "sig"); filepath.Base(lib) == "lib" && !seen[dir] {
			seen[dir] = true
			c.loadSigDir(dir, true)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(c.parsed)) { // a foo.rbs beside foo.rb, as roundhouse and spinel write them: hints, so last, any other sig wins (decision 404)
		if side := strings.TrimSuffix(path, ".rb") + ".rbs"; side != path {
			_ = c.loadSigFile(side, true, true) // a hint it cannot read is no hint (a source named with a NUL byte has no sidecar)
		}
	}
}

// loadGemSigs parses the vendored gem signatures embedded in the compiler's
// gemSigs FS (decision 160). They are keyed by class name like a user's, so a
// program that carries a class of the same qualified name gets its types.
func (c *Compiler) loadGemSigs() {
	if c.gemSigs == nil {
		return
	}
	paths, _ := fs.Glob(c.gemSigs, "sig/gems/*/*.rbs") // the pattern is constant
	for _, p := range paths {
		data, err := fs.ReadFile(c.gemSigs, p)
		if err != nil {
			c.errorf(nil, nil, "%s: %v", p, err)
		}
		f, perr := rbs.ParseFile(string(data))
		if perr != nil {
			c.errorf(nil, nil, "%s: %v", p, perr)
		}
		for _, decl := range f.Decls {
			c.addSig(decl)
		}
	}
}

// addSig merges decl into the sigs of its class. A reopened class (several
// files, or the same name twice) keeps every member; on a conflicting member
// the declaration seen first wins, and a user's sig dir loads before the
// vendored gem sigs.
func (c *Compiler) addSig(decl *rbs.ClassDecl) {
	d := c.sigs[decl.Name]
	if d == nil {
		c.sigs[decl.Name] = decl
		return
	}
	merged := *d
	if len(merged.TypeParams) == 0 {
		merged.TypeParams = decl.TypeParams
	}
	if merged.Super == nil {
		merged.Super = decl.Super
	}
	merged.Includes = appendNew(d.Includes, decl.Includes, func(i *rbs.IncludeDecl) string { return string(i.Kind) + " " + i.Module })
	merged.Methods = appendNew(d.Methods, decl.Methods, func(m *rbs.MethodDecl) string { return fmt.Sprint(m.Self, m.Name) })
	merged.Attrs = appendNew(d.Attrs, decl.Attrs, func(a *rbs.AttrDecl) string { return fmt.Sprint(a.Self, a.Name) })
	merged.Ivars = appendNew(d.Ivars, decl.Ivars, func(k *rbs.ConstDecl) string { return k.Name })
	merged.Consts = appendNew(d.Consts, decl.Consts, func(k *rbs.ConstDecl) string { return k.Name })
	merged.Aliases = appendNew(d.Aliases, decl.Aliases, func(a *rbs.TypeAliasDecl) string { return a.Name })
	merged.MethodAliases = appendNew(d.MethodAliases, decl.MethodAliases, func(a *rbs.MethodAliasDecl) string { return fmt.Sprint(a.Self, a.New) })
	c.sigs[decl.Name] = &merged
}

// appendNew is old plus the members of add whose key old lacks.
func appendNew[T any](old, add []T, key func(T) string) []T {
	have := map[string]bool{}
	for _, o := range old {
		have[key(o)] = true
	}
	out := slices.Clone(old)
	for _, a := range add {
		if k := key(a); !have[k] {
			have[k] = true
			out = append(out, a)
		}
	}
	return out
}

// loadSigFile parses one .rbs file into the sigs; a missing one is no error.
func (c *Compiler) loadSigFile(path string, shipped, advisory bool) error {
	data, rerr := os.ReadFile(path) //nolint:gosec // the program's own sig file
	c.noteInput(path, data, rerr)
	if errors.Is(rerr, fs.ErrNotExist) {
		return nil
	}
	if rerr != nil {
		return fmt.Errorf("read %s: %w", path, rerr)
	}
	f, perr := rbs.ParseFile(string(data))
	if perr != nil && shipped {
		if c.env.GemWarnings {
			c.addWarning(nil, fmt.Sprintf("%s: skipped, rugaroo cannot read it: %v (decision 306)", path, perr))
		}
		return nil
	}
	if perr != nil {
		c.errorf(nil, nil, "%s: %v", path, perr)
	}
	for _, decl := range f.Decls {
		if shipped {
			c.eraseGemGenerics(path, decl)
		}
		for _, md := range decl.Methods {
			md.Advisory = advisory
		}
		c.addSig(decl)
	}
	return nil
}

// loadSigDir reads a gem's shipped sig dir leniently, as its code is (decision 306); `_` dirs are skipped as RBS does.
func (c *Compiler) loadSigDir(dir string, shipped bool) {
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && shipped && path != dir && strings.HasPrefix(d.Name(), "_") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".rbs") {
			return nil
		}
		return c.loadSigFile(path, shipped, false)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, errors.ErrUnsupported) { // ErrUnsupported: a browser's js/wasm has no filesystem, so no sig dir
		c.errorf(nil, nil, "%v", err)
	}
}

// sigMethod is the sig file's method `name` on cls (a `def self.` when cls is
// a metaclass), or nil.
func (c *Compiler) sigMethod(cls *Class, name string) *rbs.MethodDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	wantSelf := cls.metaOf != nil
	for range len(d.MethodAliases) + 1 { // each hop follows one `alias new old` (decision 306)
		for _, m := range d.Methods {
			if m.Name == name && (m.Self == wantSelf || m.Both) {
				return m
			}
		}
		i := slices.IndexFunc(d.MethodAliases, func(a *rbs.MethodAliasDecl) bool { return a.New == name && a.Self == wantSelf })
		if i < 0 {
			return nil
		}
		name = d.MethodAliases[i].Old
	}
	return nil
}

// eraseGemGenerics drops a shipped sig's type params where rugaroo cannot keep them: a class whose superclass is a core
// @go_type (only decision 165's Hash subclass is generic, over V alone). A struct class keeps them: decisions 308-312.
func (c *Compiler) eraseGemGenerics(path string, d *rbs.ClassDecl) {
	if d.IsModule || len(d.TypeParams) == 0 {
		return
	}
	if sn, ok := d.Super.(rbs.Name); !ok || c.classes[strings.TrimPrefix(sn.Name, "::")] == nil || c.classes[strings.TrimPrefix(sn.Name, "::")].GoType == "" {
		return // a generic struct class (decision 312)
	}
	if old := c.sigs[d.Name]; old != nil && len(old.TypeParams) > 0 {
		return
	}
	if cls := c.classes[d.Name]; cls != nil && cls.File.isPrelude() {
		return
	}
	if c.env.GemWarnings {
		c.addWarning(nil, fmt.Sprintf("%s:%d: %s[%s]: members typed by its type params are left to inference: its superclass is a core @go_type (decisions 306, 312)", path, d.Line, d.Name, strings.Join(d.TypeParams, ", ")))
	}
	vars := map[string]bool{}
	for _, tp := range d.TypeParams {
		vars[tp] = true
	}
	d.TypeParams = nil
	// a member naming a type var has no sig at all, so inference types it as without one (minimize untyped)
	mentions := func(t rbs.Type) bool { return t != nil && eraseVars(t, vars).String() != t.String() }
	params := func(ps []rbs.Param) bool {
		return slices.ContainsFunc(ps, func(p rbs.Param) bool { return mentions(p.Type) })
	}
	if d.Super != nil {
		d.Super = eraseVars(d.Super, vars)
	}
	d.Methods = slices.DeleteFunc(d.Methods, func(m *rbs.MethodDecl) bool {
		return slices.ContainsFunc(m.Overloads, func(mt *rbs.MethodType) bool {
			b := mt.Block
			return params(mt.Params) || mentions(mt.Return) || b != nil && (params(b.Params) || mentions(b.Return) || mentions(b.Self))
		})
	})
	d.Attrs = slices.DeleteFunc(d.Attrs, func(a *rbs.AttrDecl) bool { return mentions(a.Type) })
	d.Ivars = slices.DeleteFunc(d.Ivars, func(k *rbs.ConstDecl) bool { return mentions(k.Type) })
	d.Consts = slices.DeleteFunc(d.Consts, func(k *rbs.ConstDecl) bool { return mentions(k.Type) })
	for _, inc := range d.Includes {
		for i, a := range inc.Args {
			inc.Args[i] = eraseVars(a, vars)
		}
	}
}

// eraseVars is t with each type variable in vars read as untyped.
func eraseVars(t rbs.Type, vars map[string]bool) rbs.Type {
	all := func(ts []rbs.Type) []rbs.Type {
		out := make([]rbs.Type, len(ts))
		for i, x := range ts {
			out[i] = eraseVars(x, vars)
		}
		return out
	}
	switch t := t.(type) {
	case rbs.Name:
		if vars[t.Name] && len(t.Args) == 0 && !t.Absolute {
			return rbs.Untyped{}
		}
		t.Args = all(t.Args)
		return t
	case rbs.Optional:
		return rbs.Optional{Elem: eraseVars(t.Elem, vars)}
	case rbs.Tuple:
		return rbs.Tuple{Elems: all(t.Elems)}
	case rbs.Union:
		return rbs.Union{Elems: all(t.Elems)}
	case rbs.Intersection:
		return rbs.Intersection{Elems: all(t.Elems)}
	case rbs.Proc:
		p := rbs.Proc{Params: all(t.Params), Ret: eraseVars(t.Ret, vars)}
		if t.Self != nil {
			p.Self = eraseVars(t.Self, vars)
		}
		return p
	case rbs.Record:
		fs := slices.Clone(t.Fields)
		for i := range fs {
			fs[i].Value = eraseVars(fs[i].Value, vars)
		}
		return rbs.Record{Fields: fs}
	case rbs.Bool, rbs.Bot, rbs.Instance, rbs.Literal, rbs.Nil, rbs.Self, rbs.Singleton, rbs.Untyped, rbs.Void: // no type variable inside
	}
	return t
}

// sigAttr is the sig file's attribute `name` on cls, of any kind (an `attr_* self.` one when cls is a metaclass).
func (c *Compiler) sigAttr(cls *Class, name string) *rbs.AttrDecl {
	if cls == nil {
		return nil
	}
	d := c.sigs[cls.RubyName]
	if d == nil {
		return nil
	}
	wantSelf := cls.metaOf != nil
	for _, a := range d.Attrs {
		if a.Name == name && a.Self == wantSelf {
			return a
		}
	}
	return nil
}

// sigTypeParams is the sig file's type parameters for cls, or nil.
func (c *Compiler) sigTypeParams(cls *Class) []string {
	if cls == nil {
		return nil
	}
	if d := c.sigs[cls.RubyName]; d != nil {
		return d.TypeParams
	}
	return nil
}

// sigConst is the sig file's type for constant k (`NAME: T` in its class), or nil.
func (c *Compiler) sigConst(k *Const) rbs.Type {
	if k.File == nil || k.File.isPrelude() {
		return nil
	}
	owner, name := "", k.RubyName
	if i := strings.LastIndex(name, "::"); i >= 0 {
		owner, name = name[:i], name[i+2:]
	}
	if d := c.sigs[owner]; d != nil {
		for _, cd := range d.Consts {
			if cd.Name == name {
				return cd.Type
			}
		}
	}
	return nil
}

// addSigIvars declares each user class's `.rbs` instance variables (`@x: T`, `self.@x: T`) that no inline
// `# @rbs` annotation already declares (decision 160).
func (c *Compiler) addSigIvars() {
	for _, cls := range c.classList {
		d := c.sigs[cls.RubyName]
		if d == nil || cls.metaOf != nil || cls.File == nil || cls.File.isPrelude() {
			continue
		}
		for _, iv := range d.Ivars {
			if !slices.ContainsFunc(cls.ivarDecls, func(x ivarDecl) bool { return x.name == iv.Name }) {
				cls.ivarDecls = append(cls.ivarDecls, ivarDecl{name: iv.Name, rbs: iv.Type, line: cls.Line, scope: c.nestingOf(cls)})
			}
		}
	}
}

// mergeArms is the one signature a `.rbs` method's overloads compile under
// (decision 12): each parameter is the union of the arms' types at its
// position, optional where an arm omits it, and the return the union of
// theirs. Nil when the arms cannot share a Go signature.
func mergeArms(sd *rbs.MethodDecl) *rbs.MethodType {
	if sd == nil || len(sd.Overloads) < 2 {
		return nil
	}
	first := sd.Overloads[0]
	out := &rbs.MethodType{TypeParams: first.TypeParams, Block: first.Block}
	var rets []rbs.Type
	for _, arm := range sd.Overloads {
		// ponytail: positional-only arms sharing one block; keyword/rest arms and differing blocks fall back to inference
		if blockText(arm.Block) != blockText(first.Block) || !slices.Equal(arm.TypeParams, first.TypeParams) {
			return nil
		}
		for i, p := range arm.Params {
			if p.Kind != rbs.ParamPositional {
				return nil
			}
			if i == len(out.Params) {
				out.Params = append(out.Params, rbs.Param{Name: p.Name, Optional: arm != first})
			}
			out.Params[i].Type = unionAdd(out.Params[i].Type, p.Type)
			out.Params[i].Optional = out.Params[i].Optional || p.Optional
		}
		for i := len(arm.Params); i < len(out.Params); i++ {
			out.Params[i].Optional = true
		}
		rets = append(rets, arm.Return)
	}
	out.Return = rets[0]
	for _, r := range rets[1:] {
		out.Return = unionAdd(out.Return, r)
	}
	return out
}

func blockText(b *rbs.Block) string {
	if b == nil {
		return ""
	}
	return (&rbs.MethodType{Block: b, Return: rbs.Void{}}).String()
}

// unionAdd is `a | b`, dropping b when a already spells it.
func unionAdd(a, b rbs.Type) rbs.Type {
	if a == nil {
		return b
	}
	elems := []rbs.Type{a}
	if u, ok := a.(rbs.Union); ok {
		elems = u.Elems
	}
	for _, e := range elems {
		if e.String() == b.String() {
			return a
		}
	}
	return rbs.Union{Elems: append(slices.Clone(elems), b)}
}

// inferAdvisoryParams types a sidecar's `untyped` positional parameters from use, as an unannotated def's (decisions 146, 404).
func (c *Compiler) inferAdvisoryParams(m *Method, n *parser.DefNode) {
	names, _, _ := c.defParams(m, n.Parameters)
	for i, p := range m.sig.Params {
		if _, ok := p.Type.(rbs.Untyped); !ok || p.Kind != rbs.ParamPositional || i >= len(names) {
			continue
		}
		if m.pendingIdx == nil {
			m.pendingIdx = map[int]pendingParam{}
		}
		m.pendingIdx[i] = pendingParam{key: paramKey(m, names[i]), name: names[i]}
	}
}

// loadRBSCollections reads `rbs collection install`'s directory: `.gem_rbs_collection` beside the program, or the one
// RUGAROO_RBS_COLLECTION names, one version per gem (the highest present; the install pins it to Gemfile.lock). Its
// sigs come after a program's own sig/ and the vendored sig/gems, before a gem's shipped sig/ (decision 457).
func (c *Compiler) loadRBSCollections(sources []Source, seen map[string]bool) {
	var roots []string
	if c.env.RBSCollection != "" {
		roots = append(roots, c.env.RBSCollection)
	}
	for _, src := range sources {
		roots = append(roots, filepath.Join(filepath.Dir(sourceName(src)), ".gem_rbs_collection"))
	}
	for _, root := range roots {
		if seen[root] {
			continue
		}
		seen[root] = true
		gems, err := os.ReadDir(root)
		if err != nil {
			continue // no collection there
		}
		for _, g := range gems {
			if !g.IsDir() || strings.HasPrefix(g.Name(), ".") || strings.HasPrefix(g.Name(), "_") {
				continue
			}
			versions, verr := os.ReadDir(filepath.Join(root, g.Name()))
			if verr != nil {
				continue
			}
			var names []string
			for _, v := range versions {
				if v.IsDir() {
					names = append(names, v.Name())
				}
			}
			if len(names) == 0 {
				continue
			}
			slices.SortFunc(names, compareVersions)
			c.loadSigDir(filepath.Join(root, g.Name(), names[len(names)-1]), true)
		}
	}
}

// compareVersions orders "2.2" before "2.10" by numeric segments.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(as), len(bs)) {
		var x, y int
		_, xe := fmt.Sscanf(as[i], "%d", &x)
		_, ye := fmt.Sscanf(bs[i], "%d", &y)
		if xe != nil || ye != nil {
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
			continue
		}
		if x != y {
			return x - y
		}
	}
	return len(as) - len(bs)
}
