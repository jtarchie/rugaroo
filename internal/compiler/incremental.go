package compiler

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Crystal re-types only what a changed type reaches; this measures how much of a round that would be at method granularity.

// walkRec keeps positions, not keys, because a changed parameter changes every key under its def.
type walkRec struct {
	reads   map[string]bool
	callees map[string]bool
	ret     string
	retType Type
	retWhy  string
	uses    map[pendingKey][]paramUse
	counted bool
	took    time.Duration
	start   time.Time
	child   time.Duration
}

// walkLog is carried to the next round in inference.walks, so that round can tell which walks it repeats.
type walkLog struct {
	recs  map[string]*walkRec
	stack []*walkRec
	bufs  map[*Method]map[pendingKey][]paramUse
}

var keyPos = regexp.MustCompile(`^(.*?:\d+:\d+:)`)

// pendingPos groups a def's, lambda's or block's keys, which all start with its position.
func pendingPos(k pendingKey) string {
	if m := keyPos.FindStringSubmatch(string(k)); m != nil {
		return m[1]
	}
	return string(k)
}

// walkKey survives rounds, where *Method does not: each round is a fresh compile.
func walkKey(m *Method) string {
	if m.Node == nil || m.File == nil {
		return ""
	}
	owner := ""
	if m.Owner != nil {
		owner = m.Owner.Name
	}
	return m.File.posKey(m.Node.Location.StartOffset) + owner + "#" + m.Name
}

// methodPos is what a caller depends on when it reads m's signature.
func methodPos(m *Method) string {
	if m.Node == nil || m.File == nil {
		return ""
	}
	return m.File.posKey(m.Node.Location.StartOffset)
}

// noteRead is called from pendingType, the one place a pending type is read (decision 279).
func (c *Compiler) noteRead(k pendingKey) {
	if c.walks == nil || len(c.walks.stack) == 0 {
		return
	}
	c.walks.stack[len(c.walks.stack)-1].reads[pendingPos(k)] = true
}

// noteCallee: a caller reads both m's signature and its inferred return.
func (c *Compiler) noteCallee(m *Method) {
	if c.walks == nil || len(c.walks.stack) == 0 || m == nil {
		return
	}
	top := c.walks.stack[len(c.walks.stack)-1]
	if k := walkKey(m); k != "" {
		top.callees[k] = true
	}
	if p := methodPos(m); p != "" {
		top.reads[p] = true
	}
	if m.Owner != nil {
		top.reads[classPos(m.Owner)] = true
	}
	for _, k := range sigKeys(m) { // a stored block's keys sit at the &block parameter, not the def
		top.reads[pendingPos(k)] = true
	}
}

// sigKeys are the pending keys m's signature is typed from, wherever their positions are.
func sigKeys(m *Method) []pendingKey {
	var out []pendingKey
	for _, p := range m.Params {
		if p.Pending != "" {
			out = append(out, p.Pending)
		}
	}
	if b := m.Block; b != nil {
		if b.Src != "" {
			out = append(out, pendingKey(b.Src))
		}
		if b.SelfPending != "" {
			out = append(out, b.SelfPending)
		}
	}
	return out
}

// beginWalk times a walk without its callees', which run nested inside it.
func (c *Compiler) beginWalk(m *Method) func() {
	if c.walks == nil {
		return func() {}
	}
	c.noteCallee(m)
	r := &walkRec{reads: map[string]bool{}, callees: map[string]bool{}, start: time.Now()}
	if p := methodPos(m); p != "" {
		r.reads[p] = true
	}
	if m.Owner != nil {
		r.reads[classPos(m.Owner)] = true
	}
	for _, k := range sigKeys(m) {
		r.reads[pendingPos(k)] = true
	}
	c.walks.stack = append(c.walks.stack, r)
	return func() {
		c.walks.stack = c.walks.stack[:len(c.walks.stack)-1]
		elapsed := time.Since(r.start)
		r.took = elapsed - r.child
		if n := len(c.walks.stack); n > 0 {
			c.walks.stack[n-1].child += elapsed
		}
		if m.Ret != nil {
			r.ret, r.retType, r.retWhy = fmt.Sprint(m.Ret), m.Ret, m.retWhy
		}
		r.counted, r.uses = c.retWalked[m], c.walks.bufs[m]
		if k := walkKey(m); k != "" {
			c.walks.recs[k] = r
		}
	}
}

// reportIncremental compares with the full round actually run, so its counts are exact for this round's changes.
func (c *Compiler) reportIncremental(inf *inference) {
	if c.walks == nil {
		return
	}
	recs := c.walks.recs
	c.walks = nil
	defer func() { inf.walks = recs; c.snapshot(inf) }()
	if inf.walks == nil || !c.env.Timing {
		return
	}
	changed := c.changedNow
	if changed == nil {
		changed = c.changedPositions(inf)
	}
	retChanged := map[string]bool{}
	for k, r := range recs {
		if prev := inf.walks[k]; prev == nil || prev.ret != r.ret {
			retChanged[k] = true
		}
	}
	var all, dirty, initial int
	var allT, dirtyT time.Duration
	for _, k := range slices.Sorted(mapKeys(recs)) {
		r := recs[k]
		all++
		allT += r.took
		prev := inf.walks[k]
		need := prev == nil || !prev.counted
		for p := range prev.reads {
			if need {
				break
			}
			need = changed[p]
		}
		if need {
			initial++
		}
		for ck := range prev.callees {
			if need {
				break
			}
			need = retChanged[ck]
		}
		if need {
			dirty++
			dirtyT += r.took
		}
	}
	ivarClasses := 0
	for p := range changed {
		if strings.HasPrefix(p, "class:") {
			ivarClasses++
		}
	}
	fmt.Fprintf(os.Stderr, "rugaroo: incremental: %d of %d return walks needed (%d by their reads), %v of %v walk time, %d positions changed (%d classes by their ivars), %d reused\n",
		dirty, all, initial, dirtyT.Round(time.Millisecond), allT.Round(time.Millisecond), len(changed), ivarClasses, c.reused)
}

func typeStrings(m map[pendingKey]Type) map[pendingKey]string {
	out := make(map[pendingKey]string, len(m))
	for k, t := range m {
		out[k] = fmt.Sprint(t)
	}
	return out
}

func mapKeys[K comparable, V any](m map[K]V) func(func(K) bool) {
	return func(yield func(K) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// stateSig is only what no position stands for; per-parameter sets and per-class ivars are positions (changedPositions).
func (c *Compiler) stateSig(inf *inference) string {
	return fmt.Sprint(inf.settling, inf.resolved, len(c.classList), inf.procAlias)
}

// keySets are the per-parameter sets a walk may read for its own or a callee's parameter.
func keySets(inf *inference) map[string]map[pendingKey]bool {
	return map[string]map[pendingKey]bool{"none": inf.none, "noDrop": inf.noDrop, "hardAny": inf.hardAny, "anyUse": inf.anyUse, "noUnion": inf.noUnion}
}

// ivarPrints is each class's ivar types, so a change dirties only the walks of that class's family.
func (c *Compiler) ivarPrints() map[string]string {
	out := map[string]string{}
	for _, cls := range c.classList {
		if len(cls.Ivars) == 0 {
			continue
		}
		var b strings.Builder
		for _, name := range slices.Sorted(mapKeys(cls.Ivars)) {
			iv := cls.Ivars[name]
			fmt.Fprintf(&b, "%s %v %v %v %v %v\n", name, iv.Type, iv.open, iv.elems, iv.scratch, iv.joins) // an open container's elements type its reads too
		}
		out[cls.Name] = b.String() // not RubyName: a class and its metaclass share that
	}
	return out
}

// classPos is the position a walk depends on by being, or calling, a method of cls.
func classPos(cls *Class) string { return "class:" + cls.Name }

// snapshot is what the next round compares with: the state this round's walks read.
func (c *Compiler) snapshot(inf *inference) {
	inf.walkTypes, inf.walkSig, inf.walkIvars = typeStrings(inf.types), c.stateSig(inf), c.ivarPrints()
	inf.walkSets = map[string]map[pendingKey]bool{}
	for name, set := range keySets(inf) {
		inf.walkSets[name] = maps.Clone(set)
	}
}

// planReuse is the last round's walks whose inputs this round leaves as they were; their callers' returns are checked after the round's walks (propagateReuse).
func (c *Compiler) planReuse(inf *inference) map[*Method]*walkRec {
	if c.env.NoIncremental || c.env.NoFileReuse || inf.walks == nil {
		return nil
	}
	if inf.walkSig != c.stateSig(inf) {
		return nil // a phase changed, or what a lambda shares
	}
	changed := c.changedPositions(inf)
	c.changedNow = changed
	out := map[*Method]*walkRec{}
	for _, m := range c.inferredMethods() {
		r := inf.walks[walkKey(m)]
		if r == nil || !r.counted || r.retType == nil {
			continue
		}
		clean := true
		for p := range r.reads {
			if changed[p] {
				clean = false
				break
			}
		}
		if clean {
			out[m] = r
		}
	}
	return out
}

func (c *Compiler) changedPositions(inf *inference) map[string]bool {
	changed := map[string]bool{}
	for k, t := range inf.walkTypes {
		if now, ok := inf.types[k]; !ok || fmt.Sprint(now) != t {
			changed[pendingPos(k)] = true
		}
	}
	for k := range inf.types {
		if _, ok := inf.walkTypes[k]; !ok {
			changed[pendingPos(k)] = true
		}
	}
	for name, set := range keySets(inf) {
		was := inf.walkSets[name]
		for k := range set {
			if !was[k] {
				changed[pendingPos(k)] = true
			}
		}
		for k := range was {
			if !set[k] {
				changed[pendingPos(k)] = true
			}
		}
	}
	now := c.ivarPrints()
	for _, cls := range c.classList {
		if now[cls.Name] != inf.walkIvars[cls.Name] {
			for _, k := range c.ivarFamily(cls) {
				changed[classPos(k)] = true
			}
		}
	}
	return changed
}

// ivarFamily is every class or module whose methods may read cls's ivars: its ancestors, descendants and their modules.
func (c *Compiler) ivarFamily(cls *Class) []*Class {
	out := append([]*Class{cls}, cls.allAncestors()...)
	for _, k := range c.classList {
		if k != cls && k.isSubclassOf(cls) {
			out = append(out, k)
			out = append(out, k.allAncestors()...)
		}
	}
	return out
}

// presetWalk stands in for m's walk with the last round's: inferRet then returns at once, as for a walk already done.
func (c *Compiler) presetWalk(m *Method, r *walkRec) {
	m.Ret, m.retWhy = c.rehome(r.retType), r.retWhy
	m.roundRet = c.awaitsParams(m, m.Ret)
	c.noteHardAny(m, m.Ret)
	c.retWalked[m] = true
}

// propagateReuse re-walks a reused method whose callee this round returned otherwise, until none does; then adds the reused walks' uses.
func (c *Compiler) propagateReuse(inf *inference, reuse map[*Method]*walkRec) {
	for len(reuse) > 0 {
		changed := map[string]bool{}
		for k, r := range c.walks.recs {
			if prev := inf.walks[k]; prev == nil || prev.ret != r.ret {
				changed[k] = true
			}
		}
		var redo []*Method
		for m, r := range reuse {
			for ck := range r.callees {
				if changed[ck] {
					redo = append(redo, m)
					break
				}
			}
		}
		if len(redo) == 0 {
			break
		}
		slices.SortFunc(redo, func(a, b *Method) int { return cmp.Compare(walkKey(a), walkKey(b)) })
		for _, m := range redo {
			delete(reuse, m)
			m.Ret, m.retWhy, c.retWalked[m] = nil, "", false
		}
		for _, m := range redo {
			_ = catchCompileError(func() { c.inferRet(m) })
		}
	}
	for _, m := range slices.SortedFunc(mapKeys(reuse), func(a, b *Method) int { return cmp.Compare(walkKey(a), walkKey(b)) }) {
		r := reuse[m]
		for k, us := range r.uses {
			for _, u := range us {
				u.typ = c.rehome(u.typ)
				c.retUses[k] = append(c.retUses[k], u)
			}
		}
		c.walks.recs[walkKey(m)] = r
		c.reused++
	}
}
