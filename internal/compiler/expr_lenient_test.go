package compiler

import "testing"

func TestLookupMissMode(t *testing.T) {
	for _, tc := range []struct {
		recv, name string
		nargs      int
		block      bool
		want       string
	}{
		{"Hash", "[]", 1, false, "default"},
		{"Hash", "key?", 1, false, "false"},
		{"Hash", "delete", 1, false, "nil"},
		{"Hash", "dig", 2, false, "nil"},
		{"Hash", "dig", 1, false, "nil"},
		{"Hash", "fetch", 2, false, "arg"},
		{"Hash", "fetch", 1, true, "block"},
		{"Hash", "fetch", 1, false, "keyerror"},
		{"Hash", "fetch", 3, false, ""},
		{"Hash", "fetch", 2, true, ""},
		{"Hash", "[]", 2, false, ""},
		{"Hash", "delete", 1, true, ""},
		{"Array", "include?", 1, false, "false"},
		{"Array", "index", 1, true, ""},
		{"Array", "push", 1, false, ""},
		{"Set", "include?", 1, false, "false"}, // a wrong-class key is a miss, as Array's (decision 444)
	} {
		got := lookupMissMode(lookupMiss[tc.recv][tc.name], tc.name, tc.nargs, tc.block)
		if string(got) != tc.want {
			t.Errorf("%s#%s nargs=%d block=%v: got %q, want %q", tc.recv, tc.name, tc.nargs, tc.block, got, tc.want)
		}
	}
}
