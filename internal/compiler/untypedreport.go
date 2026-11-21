package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// RUGAROO_UNTYPED_REPORT (debug only) ranks untypedWhy's roots by dynamic-call sites; each reason is recorded where the compiler decides it, never rebuilt afterwards, and a kind with nothing recorded says so.

// untypedRoot is where an untyped value starts. It is comparable: the report groups call sites by it.
type untypedRoot struct {
	kind string     // param, sig param, param reassigned, return, sig return, core return, block param, ivar, local, constant, self, expr, typed receiver, missing method
	name string     // Class#method(param), Class#method, Class@ivar, ...
	elem string     // the root is typed, as this container: the untyped value is one of its elements
	pos  srcPos     // the def, the declaring write, the expression
	key  pendingKey // a parameter typed from use: its reason is inference.why's
	m    *Method    // a return's or a yield's method: its reason is retWhy, or its yields' keys'
	iv   *Ivar      // an ivar: its reason is Compiler.ivarNotes'
	lam  lambdaSrc  // a lambda's parameters: their reasons are inference.why's, by position
	note string     // a reason known where the root was named (a sig, a local's two types)
}

// passes reports a root worth blaming in place of the return or ivar that only carries its value on.
func (r untypedRoot) passes() bool {
	switch r.kind {
	case "", "expr", "self", "constant", "missing method":
		return false
	}
	return true
}

// why is untypedWhy's answer: the text RUGAROO_UNTYPED_WHY prints, the root, and the return or ivar the value came through.
type why struct {
	text string
	root untypedRoot
	via  string
}

func (w why) pre(prefix string) why {
	w.text = prefix + w.text
	return w
}

// ivarNote is why an ivar is untyped, noted by the write that declared or widened it.
type ivarNote struct {
	pos  srcPos
	text string
	up   untypedRoot // the root of the untyped value the write stored
}

func methodLabel(m *Method) string {
	if m.Owner == nil {
		return m.Name
	}
	return m.Owner.RubyName + "#" + m.Name
}

// sigNote is the signature that typed m, for a root that is one.
func sigNote(m *Method) string {
	switch {
	case m.File != nil && m.File.isPrelude():
		return "the prelude's signature at " + m.srcPos.String()
	case m.sigText != "":
		return "its signature at " + m.srcPos.String() + ": `" + m.sigText + "`"
	}
	return "a signature declared elsewhere (a sig file, an ancestor's or a Sorbet sig) for the def at " + m.srcPos.String()
}

// scopeName is the method, or class body, f generates.
func (f *fctx) scopeName() string {
	switch {
	case f.m != nil:
		return methodLabel(f.m)
	case f.owner != nil:
		return f.owner.RubyName
	}
	return "main"
}

func (f *fctx) posOf(n parser.Node) srcPos {
	if n == nil {
		return srcPos{}
	}
	return srcPos{f.f, f.f.shownLine(n.GetLocation().StartOffset)}
}

// exprWhy is a root that is only an expression: nothing says more about it.
func (f *fctx) exprWhy(n parser.Node, text string) why {
	return why{text: text, root: untypedRoot{kind: "expr", name: text, pos: f.posOf(n)}}
}

// localWhy is a local that is its own root: its writes made it untyped.
func (f *fctx) localWhy(n parser.Node, name, text string) why {
	return why{text: text, root: untypedRoot{kind: "local", name: name + " in " + f.scopeName(), pos: f.posOf(n), note: text}}
}

// elemWhy is an untyped value read out of recv, a container of type rt: the root is the container's.
func (f *fctx) elemWhy(recv parser.Node, rt Type, text string, depth int) why {
	if f.c.env.UntypedReport == "" {
		return why{text: text}
	}
	w := f.untypedWhy(recv, depth+1)
	w.text, w.root.elem = text, typeString(rt)
	return w
}

// lambdaWhy is an untyped parameter of the lambda literal n, typed from its calls.
func (f *fctx) lambdaWhy(n parser.Node, text string) why {
	return why{text: text, root: untypedRoot{kind: "lambda param", name: "of the lambda in " + f.scopeName(), pos: f.posOf(n), lam: lambdaSrcAt(f.f, n.GetLocation().StartOffset)}}
}

// yieldWhy is an untyped block parameter of call on a typed receiver: the method that yields it.
func (f *fctx) yieldWhy(call *parser.CallNode, rt Type, text string) why {
	w := why{text: text, root: untypedRoot{kind: "block param", name: strings.TrimPrefix(text, "block param: "), pos: f.posOf(call)}}
	if f.c.env.UntypedReport == "" {
		return w
	}
	if call.Receiver == nil && (call.Name == "lambda" || call.Name == "proc") {
		return f.lambdaWhy(call, text)
	}
	if isSelfVar(rt) && f.owner != nil {
		rt = TClass{C: f.owner}
	}
	c, ok := stripOpt(rt).(TClass)
	if !ok {
		return w
	}
	e := c.C.lookup(call.Name)
	switch {
	case e == nil:
	case e.M.File != nil && e.M.File.isPrelude():
		if holdsAny(rt) && call.Receiver != nil { // `xs.each { |x| ... }` over an Array[untyped]: the container's elements
			return f.elemWhy(call.Receiver, rt, text, 1)
		}
		w.root.pos, w.root.note = e.M.srcPos, sigNote(e.M)
		if each := c.C.lookup("each"); each != nil && each.M.File != nil && !each.M.File.isPrelude() { // an Enumerable method yields what the class's own each does
			w.root = untypedRoot{kind: "block param", name: "yield of " + c.C.RubyName + "#each", pos: each.M.srcPos, m: each.M}
		}
	default:
		w.root.pos, w.root.m = e.M.srcPos, e.M
	}
	return w
}

// ivarWhy is the root of an untyped ivar read: the root of the value its declaring write stored, else the ivar.
func (f *fctx) ivarWhy(name string) why {
	w := why{text: "ivar", root: untypedRoot{kind: "ivar", name: f.scopeName() + " " + name}}
	if f.owner == nil || f.c.env.UntypedReport == "" {
		return w
	}
	w.root, w.via = f.c.ivarRoot(f.owner, name)
	return w
}

// ivarRoot is the root of cls's ivar name, and the ivar's label when the root is upstream of it.
func (c *Compiler) ivarRoot(cls *Class, name string) (untypedRoot, string) {
	iv := c.findIvar(cls, name)
	if iv == nil { // a gem module's or superclass's read of an ivar only an includer or subclass has (decisions 324, 330)
		return untypedRoot{kind: "ivar read by name", name: cls.RubyName + name, note: cls.RubyName + " has no such ivar (a class that includes or inherits it does), so it is read by name: untyped whatever its type there (decisions 324, 330)"}, ""
	}
	label := iv.Owner.RubyName + name
	note := c.ivarNotes[iv]
	if note.up.passes() {
		return note.up, label
	}
	return untypedRoot{kind: "ivar", name: label, pos: note.pos, iv: iv}, ""
}

// noteTargetWhy keeps the root of the untyped value a multiple assignment is about to store, for the ivar it may declare.
func (f *fctx) noteTargetWhy(value parser.Node, v expr) {
	f.targetWhy = why{}
	if f.c.env.UntypedReport != "" && isAny(v.typ) {
		_ = catchCompileError(func() { f.targetWhy = f.untypedWhy(value, 1) })
	}
}

// noteIvar records why iv is untyped, at the write (or declaration) that made it so.
func (c *Compiler) noteIvar(iv *Ivar, pos srcPos, text string, up untypedRoot) {
	if c.env.UntypedReport == "" || iv == nil {
		return
	}
	if c.ivarNotes == nil {
		c.ivarNotes = map[*Ivar]ivarNote{}
	}
	c.ivarNotes[iv] = ivarNote{pos: pos, text: text, up: up}
}

// noteRefineDrops records the typings refineIvars tried and did not keep.
func (c *Compiler) noteRefineDrops(cands []*Ivar, typed map[*Ivar]Type, accepted []bool) {
	if c.env.UntypedReport == "" {
		return
	}
	c.refineDrops = map[*Ivar]Type{}
	for i, iv := range cands {
		if !accepted[i] {
			c.refineDrops[iv] = typed[iv]
		}
	}
}

// noteIvarWrite is noteIvar for the write n that declares iv with what it stores.
func (f *fctx) noteIvarWrite(iv *Ivar, n parser.Node, assigned Type) {
	if f.c.env.UntypedReport == "" || !holdsAny(assigned) {
		return
	}
	text := "its first write stores " + typeString(assigned)
	var up untypedRoot
	if w, ok := n.(*parser.InstanceVariableWriteNode); ok && isAny(assigned) {
		var o why
		if catchCompileError(func() { o = f.untypedWhy(w.Value, 1) }) == nil {
			text += ": " + o.text
			up = o.root
		}
	} else if isAny(assigned) && f.targetWhy.text != "" {
		text += ": multi-assign from: " + f.targetWhy.text
		up = f.targetWhy.root
	}
	f.c.noteIvar(iv, f.posOf(n), text, up)
}

// dynSiteMark prefixes a call site's entry in Compiler.Warnings, so the warnings' dedup and a discarded pass's dropWarnings apply to it; shownWarnings leaves it out.
const dynSiteMark = "\x00dyn:"

// noteDynSite records a dynamic call for the report: msg is its warning's text, recvNode the call when its receiver may be untyped.
func (f *fctx) noteDynSite(n, recvNode parser.Node, recvT Type, msg string) {
	c := f.c
	if c.env.UntypedReport == "" || f.pass != 2 {
		return
	}
	key := dynSiteMark + f.posOf(n).String() + ": " + msg
	if c.warned[key] {
		return
	}
	w := f.exprWhy(n, strings.TrimPrefix(msg, "dynamic call: "))
	recv, ok := opReceiver(recvNode)
	switch {
	case recvT != nil && !isAny(recvT):
		w = why{root: untypedRoot{kind: "typed receiver", name: typeString(recvT), note: "the receiver's type is known, and is one only a dynamic call can call (a union, a module, a class object or Object)"}}
	case ok:
		_ = catchCompileError(func() { w = f.untypedWhy(recv, 0) })
	}
	if c.dynSites == nil {
		c.dynSites = map[string]why{}
	}
	c.dynSites[key] = w
	c.warned[key] = true
	c.Warnings = append(c.Warnings, key)
}

// shownWarnings is the warnings without the report's call-site entries.
func (c *Compiler) shownWarnings() []string {
	if c.env.UntypedReport == "" {
		return c.Warnings
	}
	return slices.DeleteFunc(slices.Clone(c.Warnings), func(w string) bool { return strings.HasPrefix(w, dynSiteMark) })
}

// noteWhy records the rule that decided key's type this round, with the uses it read (RUGAROO_UNTYPED_REPORT).
func (c *Compiler) noteWhy(key pendingKey, uses []paramUse, rule string) {
	if c.infer == nil || c.infer.why == nil {
		return
	}
	c.infer.why[key] = rule + "; " + usesText(uses)
}

// useWhy is the root of an untyped argument n, for the report's list of a parameter's uses.
func (f *fctx) useWhy(n parser.Node, t Type) string {
	if f.c.env.UntypedReport == "" || f.pass != 2 || t == nil || !isAny(stripOpt(t)) {
		return ""
	}
	if en, ok := n.(*exprNode); ok {
		n = en.Node
	}
	text := ""
	_ = catchCompileError(func() {
		w := f.untypedWhy(n, 1)
		if text = w.text; w.root.passes() {
			text = w.root.kind + " " + w.root.name
		}
	})
	return text
}

// usesText lists a parameter's uses by type: how many calls passed each, and where the first is.
func usesText(uses []paramUse) string {
	if len(uses) == 0 {
		return "no uses recorded"
	}
	type group struct {
		label string
		at    string
		n     int
	}
	var gs []*group
	byLabel := map[string]*group{}
	for _, u := range uses {
		var lb strings.Builder
		lb.WriteString(typeString(u.typ))
		for _, flag := range []struct {
			on   bool
			text string
		}{{u.deflt, " (its default)"}, {u.dyn, " (dynamic call by name)"}, {u.widen, " (widening)"}, {u.anyRest, " (untyped rest)"}, {u.why != "", " <- " + u.why}} {
			if flag.on {
				lb.WriteString(flag.text)
			}
		}
		label := lb.String()
		g := byLabel[label]
		if g == nil {
			g = &group{label: label, at: u.at().String()}
			byLabel[label] = g
			gs = append(gs, g)
		}
		g.n++
	}
	const shown = 4
	parts := make([]string, 0, shown+1)
	for i, g := range gs {
		if i == shown {
			parts = append(parts, fmt.Sprintf("and %d more types", len(gs)-shown))
			break
		}
		times := ""
		if g.n > 1 {
			times = fmt.Sprintf(" x%d", g.n)
		}
		parts = append(parts, g.label+times+" at "+g.at)
	}
	return "uses: " + strings.Join(parts, ", ")
}

// amendWhy puts what growState or the final pass did to key in front of the round's own reason.
func (inf *inference) amendWhy(key pendingKey, text string) {
	if inf.why == nil {
		return
	}
	if old := inf.why[key]; old != "" {
		text += "; this round: " + old
	}
	inf.why[key] = text
}

// rootReason is what was recorded about r when the compiler decided it.
func (c *Compiler) rootReason(r untypedRoot) string {
	const none = "reason not recorded"
	switch {
	case r.key != "":
		switch {
		case c.infer == nil:
			return none + " (no inference rounds ran)"
		case c.infer.why[r.key] != "":
			return c.infer.why[r.key]
		case c.infer.none[r.key]:
			return "no call typed it in the last round, and it recorded no uses"
		}
		return none + " (the last round did not decide this key)"
	case r.kind == "block param" && r.m != nil:
		return c.yieldReason(r.m)
	case r.lam != "":
		var parts []string
		for i := range maxStoredBlockArgs {
			if k := r.lam.arg(i); c.infer != nil && (c.infer.why[k] != "" || c.infer.none[k]) {
				parts = append(parts, fmt.Sprintf("parameter %d: %s", i, c.rootReason(untypedRoot{key: k})))
			}
		}
		if len(parts) == 0 {
			return none + " (no round decided this lambda's parameters)"
		}
		return strings.Join(parts, " | ")
	case r.m != nil:
		ret := "returns " + typeString(r.m.Ret)
		if r.m.retWhy != "" {
			ret += ": " + r.m.retWhy
		}
		return ret + " (the return expressions' places are not recorded)"
	case r.iv != nil:
		open := ""
		if r.iv.open {
			tried := "no element type found (nothing typed is written to it, or no one type holds what is)"
			if t, ok := c.refineDrops[r.iv]; ok {
				tried = "typed " + typeString(t) + " from what is written to it, then dropped: the classes that see it do not compile with that type, or need a conversion (refineIvars)"
			}
			open = fmt.Sprintf("; an open container (first assigned `[]` or `{}`): %s; the last discovery walk saw written, per type argument (an Array's elements; a Hash's keys, then values): %v, %v", tried, r.iv.elems[0], r.iv.elems[1])
		}
		if n, ok := c.ivarNotes[r.iv]; ok {
			return n.text + " at " + n.pos.String() + open
		}
		if open != "" {
			return open[2:]
		}
		if !holdsAny(r.iv.Type) {
			return "the ivar is typed " + typeString(r.iv.Type) + "; why this read of it is untyped is not recorded (a nilable ivar read in gem code, or a narrowing)"
		}
		return none + " (declared " + typeString(r.iv.Type) + " by an annotation, a sig or an attr declaration)"
	case r.note != "":
		return r.note
	}
	return none
}

// yieldReason is why the values m yields are untyped: each yielded position's key, as a parameter's.
func (c *Compiler) yieldReason(m *Method) string {
	if m.Block == nil {
		return "reason not recorded (the method has no block signature)"
	}
	var parts []string
	for i, k := range m.Block.Pending {
		if k == "" || i >= len(m.Block.Params) || !holdsAny(m.Block.Params[i]) {
			continue
		}
		parts = append(parts, fmt.Sprintf("yield value %d: %s", i, c.rootReason(untypedRoot{key: k})))
	}
	if len(parts) == 0 {
		return "reason not recorded (its block's types are declared, not inferred from its yields)"
	}
	return strings.Join(parts, " | ")
}

// untypedReport ranks the roots of this compile's dynamic calls.
func (c *Compiler) untypedReport() string {
	type entry struct {
		root  untypedRoot
		sites []string
		via   map[string]int
	}
	byRoot := map[untypedRoot]*entry{}
	total := 0
	for _, w := range c.Warnings {
		site, ok := strings.CutPrefix(w, dynSiteMark)
		if !ok {
			continue
		}
		total++
		o := c.dynSites[w]
		e := byRoot[o.root]
		if e == nil {
			e = &entry{root: o.root, via: map[string]int{}}
			byRoot[o.root] = e
		}
		e.sites = append(e.sites, strings.Replace(site, ": dynamic call: ", " ", 1))
		if o.via != "" {
			e.via[o.via]++
		}
	}
	text := func(e *entry) string {
		r := e.root
		s := r.kind + " " + r.name
		if r.elem != "" {
			s += " [typed " + r.elem + ": its elements are untyped]"
		}
		if p := r.pos.String(); p != "" {
			s += " at " + p
		}
		return s
	}
	entries := slices.SortedFunc(maps.Values(byRoot), func(a, b *entry) int {
		return cmp.Or(cmp.Compare(len(b.sites), len(a.sites)), cmp.Compare(text(a), text(b)))
	})
	top := 0
	for _, e := range entries[:min(10, len(entries))] {
		top += len(e.sites)
	}
	var b strings.Builder
	lane := "default rounds"
	if c.env.InferUnknown {
		lane = "RUGAROO_INFER_UNKNOWN rounds"
	}
	fmt.Fprintf(&b, "rugaroo: untyped report (%s): %d dynamic call sites, %d roots; the top 10 roots cover %d sites\n", lane, total, len(entries), top)
	for i, e := range entries {
		fmt.Fprintf(&b, "%4d. %d sites: %s\n", i+1, len(e.sites), text(e))
		fmt.Fprintf(&b, "      why: %s\n", c.rootReason(e.root))
		if len(e.via) > 0 {
			vias := slices.SortedFunc(maps.Keys(e.via), func(x, y string) int {
				return cmp.Or(cmp.Compare(e.via[y], e.via[x]), cmp.Compare(x, y))
			})
			for j, v := range vias {
				vias[j] = fmt.Sprintf("%s (%d)", v, e.via[v])
			}
			fmt.Fprintf(&b, "      through: %s\n", strings.Join(vias[:min(3, len(vias))], ", "))
		}
		fmt.Fprintf(&b, "      sites: %s\n", strings.Join(e.sites[:min(3, len(e.sites))], "; "))
	}
	return b.String()
}

var gemsDirRE = regexp.MustCompile(`/[^\s:]*/gems/`)

// opReceiver is the receiver of the call n makes: a call's own (nil: self), or what an operator write reads (`p += 1`, `x.depth += 1`, `h[k] ||= v`).
func opReceiver(n parser.Node) (parser.Node, bool) {
	switch x := n.(type) {
	case *parser.CallNode:
		return x.Receiver, true
	case *parser.LocalVariableOperatorWriteNode:
		return &parser.LocalVariableReadNode{Name: x.Name, Depth: x.Depth}, true
	case *parser.InstanceVariableOperatorWriteNode:
		return &parser.InstanceVariableReadNode{Name: x.Name}, true
	case *parser.CallOperatorWriteNode:
		return x.Receiver, true
	case *parser.CallOrWriteNode:
		return x.Receiver, true
	case *parser.CallAndWriteNode:
		return x.Receiver, true
	case *parser.IndexOperatorWriteNode:
		return x.Receiver, true
	case *parser.IndexOrWriteNode:
		return x.Receiver, true
	case *parser.IndexAndWriteNode:
		return x.Receiver, true
	}
	return nil, false
}

// emitUntypedReport prints the last compile's report to stderr, or to the file RUGAROO_UNTYPED_REPORT names.
func emitUntypedReport(opts *options) {
	path, text := opts.env.UntypedReport, opts.whyReport
	wd, err := os.Getwd()
	if err == nil { // a gem's files are named by their full paths
		text = strings.ReplaceAll(text, wd+string(os.PathSeparator), "")
	}
	text = gemsDirRE.ReplaceAllString(text, "")
	switch path {
	case "":
	case "1":
		fmt.Fprint(os.Stderr, text)
	default:
		err = os.WriteFile(path, []byte(text), 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rugaroo: untyped report:", err)
		}
	}
}
