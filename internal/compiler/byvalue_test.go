package compiler

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Which classes are Go value structs (decision 441): the MRI tests prove the behaviour, this pins the representation.
func TestValueClasses(t *testing.T) {
	const data = "P = Data.define(:x, :y) #: [Integer, Float]\na = P.new(x: 1, y: 2.0)\np a.with(x: 3)\n"
	const plain = "# @value\nclass V\n  attr_reader :x #: Integer\n\n  #: (Integer x) -> void\n  def initialize(x)\n    @x = x\n  end\nend\np V.new(1).x\n"
	cases := map[string]struct{ src, want string }{
		"data":         {data, "func NewP(__v0 Integer, __v1 Float) P {"},
		"data_equal":   {data + "p a.equal?(a)\n", "func NewP(__v0 Integer, __v1 Float) *P {"},
		"data_big":     {"P = Data.define(:a, :b, :c) #: [String, String, Integer]\np P.new(\"a\", \"b\", 1)\n", "func NewP(__v0 String, __v1 String, __v2 Integer) *P {"},
		"data_untyped": {"P = Data.define(:a)\np P.new(1)\n", "func NewP(__v0 any) *P {"},
		"annotated":    {plain, "func NewV(x Integer) V {"},
		"unannotated":  {strings.TrimPrefix(plain, "# @value\n"), "func NewV(x Integer) *V {"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out, _, err := CompileWithWarnings(context.Background(), os.DirFS("../.."), "main.rb", []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("want %q in the generated Go", tc.want)
			}
		})
	}
}
