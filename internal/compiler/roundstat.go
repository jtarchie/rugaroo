package compiler

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"
)

// roundStats is what one inference round did, for RUGAROO_TIMING's `round work` line and RUGAROO_ROUND_DUMP: the
// measurements docs/research/round-cost-2026-11.md's stages are sized by, and the state a stage must leave as it was.
type roundStats struct {
	dry, dryPass int             // refineIvars' tries (its baseline is not one), and those that passed
	seen         map[*Class]bool // the classes a dry run emitted: those that can see a candidate ivar
	rets         map[*Method]retSnap
	retSame      int // inferred returns the accepted try left that the round's own pass inferred again the same
	retDiff      int // and those it inferred differently
	usesAll      time.Duration
	usesSeen     time.Duration // collectUses' time in methods of seen classes
	refine       string        // the candidates and which were kept
}

type retSnap struct {
	ret      Type
	roundRet bool
}

func (c *Compiler) newRoundStats() *roundStats {
	if !c.round || !c.env.Timing && c.env.RoundDump == "" {
		return nil
	}
	return &roundStats{seen: map[*Class]bool{}}
}

func (rs *roundStats) try(ok bool) {
	if rs == nil {
		return
	}
	rs.dry++
	if ok {
		rs.dryPass++
	}
}

func (rs *roundStats) sees(cls *Class) {
	if rs != nil {
		rs.seen[cls] = true
	}
}

// accepted is the end of refineIvars: the returns its last try left, and what it kept.
func (rs *roundStats) accepted(c *Compiler, sig []string, kept []bool) {
	if rs == nil {
		return
	}
	rs.rets = map[*Method]retSnap{}
	for _, m := range c.inferredMethods() {
		if m.Ret != nil {
			rs.rets[m] = retSnap{m.Ret, m.roundRet}
		}
	}
	var b strings.Builder
	for i, s := range sig {
		fmt.Fprintf(&b, "refine %s kept=%v\n", s, kept[i])
	}
	rs.refine = b.String()
}

// compareRets counts the returns the round's own pass inferred against what the accepted try left.
func (rs *roundStats) compareRets() {
	if rs == nil {
		return
	}
	for m, was := range rs.rets {
		if m.Ret != nil && typeEq(m.Ret, was.ret) && m.roundRet == was.roundRet {
			rs.retSame++
		} else {
			rs.retDiff++
		}
	}
}

func (rs *roundStats) used(m *Method, d time.Duration) {
	if rs == nil {
		return
	}
	rs.usesAll += d
	if m.Owner != nil && rs.seen[m.Owner] {
		rs.usesSeen += d
	}
}

// report ends the round: the counters under RUGAROO_TIMING, the state appended to RUGAROO_ROUND_DUMP's file.
func (rs *roundStats) report(c *Compiler, inf *inference) {
	if rs == nil {
		return
	}
	if c.env.Timing {
		fmt.Fprintf(os.Stderr, "rugaroo: round work: %d tries (%d passed), %d classes seen, returns %d same %d different, uses %dms of %dms in seen classes\n",
			rs.dry, rs.dryPass, len(rs.seen), rs.retSame, rs.retDiff, rs.usesSeen.Milliseconds(), rs.usesAll.Milliseconds())
	}
	if c.env.RoundDump == "" {
		return
	}
	var b strings.Builder
	b.WriteString("== round\n")
	b.WriteString(rs.refine)
	for _, k := range slices.Sorted(maps.Keys(inf.types)) {
		fmt.Fprintf(&b, "param %s: %s\n", k, typeString(inf.types[k]))
	}
	var rets []string
	for _, m := range c.inferredMethods() {
		rets = append(rets, fmt.Sprintf("ret %s: %s roundRet=%v\n", m, typeString(m.Ret), m.roundRet))
	}
	slices.Sort(rets)
	b.WriteString(strings.Join(rets, ""))
	f, err := os.OpenFile(c.env.RoundDump, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rugaroo: RUGAROO_ROUND_DUMP:", err)
		return
	}
	defer f.Close() //nolint:errcheck // a debugging aid's file
	_, _ = f.WriteString(b.String())
}
