package compiler

import (
	"maps"
	"slices"
)

// refineRecord: one final pass's dry-run outcome and catalog side effects, so a matching cache hit skips the dry-runs (decision 433).
type refineRecord struct {
	Cands         []string              `json:"cands"`
	Accepted      []bool                `json:"accepted"`
	Boxes         map[string]bool       `json:"boxes,omitempty"`
	ArgBoxes      map[string]bool       `json:"arg_boxes,omitempty"`
	LeafBoxes     []string              `json:"leaf_boxes,omitempty"`
	OptConv       []string              `json:"opt_conv,omitempty"`
	Marshal       map[string]cachedType `json:"marshal,omitempty"`
	DynNames      []string              `json:"dyn_names,omitempty"`
	RespondNames  []string              `json:"respond_names,omitempty"`
	DynBlockNames []string              `json:"dyn_block_names,omitempty"`
	Flags         []bool                `json:"flags,omitempty"`
	NilSites      map[string][]int      `json:"nil_sites,omitempty"` // decision 440's deferred sites: offsets by file name
}

// refineMaps is the cache's records and the map this compile's go into.
func (o *options) refineMaps() (in, out map[string]*refineRecord) {
	if o.refineOut == nil {
		o.refineOut = map[string]*refineRecord{}
	}
	return o.refineIn, o.refineOut
}

// refineKind names a final pass: the eager recompile (decision 49) runs refineIvars again from its own state.
func (c *Compiler) refineKind() string {
	if c.dynEvery {
		return "eager"
	}
	return "lazy"
}

// refineCands is the candidate list as the record keys it: owner, ivar, the tried type and the original.
func refineCands(cands []*Ivar, typed, orig map[*Ivar]Type) []string {
	out := make([]string, len(cands))
	for i, iv := range cands {
		o := "<nil>"
		if orig[iv] != nil {
			o = orig[iv].String()
		}
		out[i] = iv.Owner.Name + " " + iv.Name + " " + typed[iv].String() + " " + o
	}
	return out
}

// refineSnap is the catalogs before the dry-runs, to record what they add.
type refineSnap struct {
	boxes, argBoxes, leafBoxes, optConv, dynSeen, respondSeen, dynBlockNames map[string]bool
	marshal                                                                  map[string]Type
	flags                                                                    []bool
}

func (c *Compiler) refineFlags() []bool {
	return []bool{c.dynAll, c.dynAllStrict, c.dynBlockAll, c.classOf, c.dynEach, c.dynEachV, c.usesLineNo, c.needRequireTable}
}

func (c *Compiler) takeRefineSnap() refineSnap {
	return refineSnap{
		boxes: maps.Clone(c.boxes), argBoxes: maps.Clone(c.argBoxes), leafBoxes: maps.Clone(c.leafBoxes), optConv: maps.Clone(c.optConv),
		dynSeen: maps.Clone(c.dynSeen), respondSeen: maps.Clone(c.respondSeen), dynBlockNames: maps.Clone(c.dynBlockNames),
		marshal: maps.Clone(c.marshalSeen), flags: c.refineFlags(),
	}
}

func added(before, after map[string]bool) []string {
	var out []string
	for k := range after {
		if !before[k] {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

// recordRefine is the record of a pass that ran the dry-runs; nil when a marshal type cannot be stored.
func (c *Compiler) recordRefine(s refineSnap, cands []string, accepted []bool) *refineRecord {
	r := &refineRecord{Cands: cands, Accepted: accepted, Boxes: map[string]bool{}, ArgBoxes: map[string]bool{}, Marshal: map[string]cachedType{}}
	for k, v := range c.boxes {
		if _, ok := s.boxes[k]; !ok {
			r.Boxes[k] = v
		}
	}
	for k, v := range c.argBoxes {
		if _, ok := s.argBoxes[k]; !ok {
			r.ArgBoxes[k] = v
		}
	}
	r.LeafBoxes, r.OptConv = added(s.leafBoxes, c.leafBoxes), added(s.optConv, c.optConv)
	r.DynNames, r.RespondNames, r.DynBlockNames = added(s.dynSeen, c.dynSeen), added(s.respondSeen, c.respondSeen), added(s.dynBlockNames, c.dynBlockNames)
	for k, t := range c.marshalSeen {
		if s.marshal[k] != nil {
			continue
		}
		ct, err := encodeType(t)
		if err != nil {
			return nil
		}
		r.Marshal[k] = ct
	}
	if c.nilDefer != nil {
		r.NilSites = map[string][]int{}
		for k := range c.nilDefer.sites {
			r.NilSites[k.file.Name] = append(r.NilSites[k.file.Name], k.off)
		}
		for _, offs := range r.NilSites {
			slices.Sort(offs)
		}
	}
	r.Flags = c.refineFlags()
	for i, f := range s.flags { // only what the dry-runs turned on
		r.Flags[i] = r.Flags[i] && !f
	}
	return r
}

// replayRefine applies a record whose candidates match: the kept typings, and the catalog entries the dry-runs left.
func (c *Compiler) replayRefine(r *refineRecord, cands []*Ivar, typed, orig map[*Ivar]Type) {
	for i, iv := range cands {
		iv.Type = orig[iv]
		if r.Accepted[i] {
			iv.Type = typed[iv]
		}
	}
	maps.Copy(c.boxes, r.Boxes)
	maps.Copy(c.argBoxes, r.ArgBoxes)
	for _, k := range r.LeafBoxes {
		if c.leafBoxes == nil {
			c.leafBoxes = map[string]bool{}
		}
		c.leafBoxes[k] = true
	}
	for _, k := range r.OptConv {
		if c.optConv == nil {
			c.optConv = map[string]bool{}
		}
		c.optConv[k] = true
	}
	for _, k := range slices.Sorted(maps.Keys(r.Marshal)) {
		if c.marshalSeen[k] == nil {
			ct := r.Marshal[k]
			c.marshalSeen[k] = c.rehome(decodeType(&ct)) // placeholder classes to this compile's
		}
	}
	for _, n := range r.DynNames {
		c.noteDyn(n)
	}
	for _, n := range r.RespondNames {
		c.noteRespond(n)
	}
	for _, n := range r.DynBlockNames {
		if c.dynBlockNames == nil {
			c.dynBlockNames = map[string]bool{}
		}
		c.dynBlockNames[n] = true
	}
	flags := []*bool{&c.dynAll, &c.dynAllStrict, &c.dynBlockAll, &c.classOf, &c.dynEach, &c.dynEachV, &c.usesLineNo, &c.needRequireTable}
	for i, f := range r.Flags {
		if f && i < len(flags) {
			*flags[i] = true
		}
	}
	if r.NilSites != nil {
		c.noteNilDefer()
		c.nilDefer = &nilDeferral{sites: map[nilSite]bool{}}
		for _, file := range c.files {
			for _, off := range r.NilSites[file.Name] {
				c.nilDefer.sites[nilSite{file, off}] = true
			}
		}
	}
}
