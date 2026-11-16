//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"strings"
)

// do matches re at the pointer (anchored) or anywhere after it. On a match
// it records the groups, moves the pointer when advance is set, and returns
// the text from the old pointer to the match end when str is set (else a
// non-nil marker); nil on no match.
func (s *StringScanner) do(re *Regexp, anchored, advance, str bool) *String {
	rx := re.rx()
	if anchored {
		// each Regexp's \A-anchored twin, so a failed scan costs one attempt at the pointer instead of a search of the rest
		rx = nil
		anch := re.anchSlot()
		if anch != nil {
			rx = anch.Load()
		}
		if rx == nil {
			twin, err := rxCompile(`\A(?:` + re.rx().String() + `)`)
			if err != nil {
				panic(NewRegexpError(Ref(String(err.Error()))))
			}
			if anch != nil {
				anch.CompareAndSwap(nil, twin)
			}
			rx = twin
		}
	}
	rest := s.str[s.pos:]
	var loc []int
	if anchored {
		loc = rx.FindPrefixSubmatchIndex(rest)
	} else {
		loc = rx.FindStringSubmatchIndex(rest)
	}
	if loc == nil {
		s.groups = nil
		return nil
	}
	groups := make([]*String, len(loc)/2)
	for i := range groups {
		if loc[2*i] >= 0 {
			groups[i] = Ref(String(rest[loc[2*i]:loc[2*i+1]]))
		}
	}
	from := s.pos
	s.last, s.mbeg, s.mend, s.groups, s.names = s.pos, s.pos+loc[0], s.pos+loc[1], groups, rx.SubexpNames()
	if advance {
		s.pos = s.mend
	}
	out := String(s.str[from:s.mend])
	if !str {
		out = ""
	}
	return &out
}

// advance records [beg, end) as a match with no groups past 0 and moves the pointer.
func (s *StringScanner) advance(beg, end int) *String {
	m := Ref(String(s.str[beg:end]))
	s.last, s.mbeg, s.mend, s.groups, s.names, s.pos = s.pos, beg, end, []*String{m}, nil, end
	return m
}

// rbScanLen is the byte length skip/match?/exist? return: from the pointer
// before the call to the match end.
func rbScanLen(m *String, s *StringScanner) *Integer {
	if m == nil {
		return nil
	}
	return Ref(Integer(s.mend - s.last))
}

// rbByteInspect is inspect of a binary string: bytes past ASCII as \xHH.
func rbByteInspect(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		if c := s[i]; c >= 0x80 {
			fmt.Fprintf(&b, "\\x%02X", c)
		} else {
			q := string(rbStringInspect(string(c)))
			b.WriteString(q[1 : len(q)-1])
		}
	}
	b.WriteByte('"')
	return b.String()
}
