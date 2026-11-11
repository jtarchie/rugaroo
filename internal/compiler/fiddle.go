package compiler

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// MRI's extern/bind module_eval a method at run time; it is synthesized from the literal signature instead (decision 326).

// fiddleCTypes are cparser's parse_ctype patterns in its order; the first match wins.
var fiddleCTypes = []struct {
	re   *regexp.Regexp
	code int
}{
	{regexp.MustCompile(`\Avoid\z`), 0},
	{regexp.MustCompile(`\A(?:(?:signed\s+)?long\s+long(?:\s+int\s+)?|int64_t)(?:\s+\w+)?\z`), 6},
	{regexp.MustCompile(`\A(?:unsigned\s+long\s+long(?:\s+int\s+)?|uint64_t)(?:\s+\w+)?\z`), -6},
	{regexp.MustCompile(`\A(?:unsigned\s+long(?:\s+int\s+)?(?:\s+\w+)?|unsigned\s+int\s+long(?:\s+\w+)?|long(?:\s+int)?\s+unsigned(?:\s+\w+)?|int\s+unsigned\s+long(?:\s+\w+)?|(?:int\s+)?long\s+unsigned(?:\s+\w+)?)\z`), -5},
	{regexp.MustCompile(`\A(?:(?:signed\s+)?long(?:\s+int\s+)?(?:\s+\w+)?|(?:signed\s+)?int\s+long(?:\s+\w+)?|long(?:\s+int)?\s+signed(?:\s+\w+)?)\z`), 5},
	{regexp.MustCompile(`\A(?:unsigned\s+short(?:\s+int\s+)?(?:\s+\w+)?|unsigned\s+int\s+short(?:\s+\w+)?|short(?:\s+int)?\s+unsigned(?:\s+\w+)?|int\s+unsigned\s+short(?:\s+\w+)?|(?:int\s+)?short\s+unsigned(?:\s+\w+)?)\z`), -3},
	{regexp.MustCompile(`\A(?:(?:signed\s+)?short(?:\s+int\s+)?(?:\s+\w+)?|(?:signed\s+)?int\s+short(?:\s+\w+)?|int\s+(?:signed\s+)?short(?:\s+\w+)?)\z`), 3},
	{regexp.MustCompile(`\A(?:signed\s+)?int(?:\s+\w+)?\z`), 4},
	{regexp.MustCompile(`\A(?:unsigned\s+int|uint)(?:\s+\w+)?\z`), -4},
	{regexp.MustCompile(`\A(?:signed\s+)?char(?:\s+\w+)?\z`), 2},
	{regexp.MustCompile(`\Aunsigned\s+char(?:\s+\w+)?\z`), -2},
	{regexp.MustCompile(`\Aint8_t(?:\s+\w+)?\z`), 2},
	{regexp.MustCompile(`\Auint8_t(?:\s+\w+)?\z`), -2},
	{regexp.MustCompile(`\Aint16_t(?:\s+\w+)?\z`), 3},
	{regexp.MustCompile(`\Auint16_t(?:\s+\w+)?\z`), -3},
	{regexp.MustCompile(`\Aint32_t(?:\s+\w+)?\z`), 4},
	{regexp.MustCompile(`\Auint32_t(?:\s+\w+)?\z`), -4},
	{regexp.MustCompile(`\Afloat(?:\s+\w+)?\z`), 7},
	{regexp.MustCompile(`\Adouble(?:\s+\w+)?\z`), 8},
	{regexp.MustCompile(`\Asize_t(?:\s+\w+)?\z`), -5},
	{regexp.MustCompile(`\A(?:ssize_t|ptrdiff_t|intptr_t)(?:\s+\w+)?\z`), 5},
	{regexp.MustCompile(`\Auintptr_t(?:\s+\w+)?\z`), -5},
	{regexp.MustCompile(`\Abool(?:\s+\w+)?\z`), 11},
	{regexp.MustCompile(`\*|\[[\s\d]*\]`), 1},
	{regexp.MustCompile(`\A\.\.\.\z`), 9},
}

var (
	fiddleFuncPtrSig = regexp.MustCompile(`^(?:[\w*\s]+)\(\*(\w+)\((.*?)\)\)(?:\[\w*\]|\(.*?\));?$`)
	fiddleFuncSig    = regexp.MustCompile(`^([\w*\s]+[*\s])(\w+)\((.*?)\);?$`)
	fiddleSpaces     = regexp.MustCompile(`\s+`)
	fiddlePunct      = regexp.MustCompile(`\s*([()\[\]*,;])\s*`)
	fiddleArg        = regexp.MustCompile(`([\w*\s]+\(\*\w*\)\(.*?\)|[\w*\s\[\]]+|\.\.\.)(?:,\s*|\z)`)
)

// fiddleCType is parse_ctype: a C type's TYPE_* code; ok is false for a name neither C nor aliased.
func fiddleCType(ty string, alias map[string]string) (int, bool) {
	ty = strings.TrimPrefix(ty, "const ")
	for _, t := range fiddleCTypes {
		if t.re.MatchString(ty) {
			return t.code, true
		}
	}
	name, _, _ := strings.Cut(ty, " ")
	if orig, ok := alias[name]; ok && orig != ty {
		return fiddleCType(orig, alias)
	}
	return 0, false
}

// fiddleSignature is parse_signature's function name, return type and argument types.
func fiddleSignature(sig string, alias map[string]string) (name string, ret int, args []int, err error) {
	sig = strings.TrimSpace(fiddlePunct.ReplaceAllString(fiddleSpaces.ReplaceAllString(sig, " "), "$1"))
	var retType, argList string
	if m := fiddleFuncPtrSig.FindStringSubmatch(sig); m != nil {
		name, ret, argList = m[1], 1, m[2]
	} else if m := fiddleFuncSig.FindStringSubmatch(sig); m != nil {
		retType, name, argList = strings.TrimSpace(m[1]), m[2], m[3]
		var ok bool
		if ret, ok = fiddleCType(retType, alias); !ok {
			return "", 0, nil, fmt.Errorf("unknown type: %s", retType)
		}
	} else {
		return "", 0, nil, fmt.Errorf("can't parse the function prototype: %s", sig)
	}
	if strings.TrimSpace(argList) != "void" {
		for _, m := range fiddleArg.FindAllStringSubmatch(argList, -1) {
			t, ok := fiddleCType(m[1], alias)
			if !ok {
				return "", 0, nil, fmt.Errorf("unknown type: %s", m[1])
			}
			args = append(args, t)
		}
	}
	return name, ret, args, nil
}

// fiddleRubyType is the Ruby type Function#call returns for a C return type.
func fiddleRubyType(code int) string {
	switch code {
	case 0:
		return "nil"
	case 1:
		return "Fiddle::Pointer"
	case 7, 8:
		return "Float"
	case 11:
		return "bool"
	}
	return "Integer"
}

// fiddleImporter reports a module that extends Fiddle::Importer.
func fiddleImporter(f *File, cls *Class) bool {
	for _, e := range cls.extends {
		if strings.TrimPrefix(constText(f, e.ref.node), "Fiddle::") == "Importer" {
			return true
		}
	}
	return false
}

// collectFiddleCall reads a literal typealias/extern/bind in an Importer module; the call itself still runs at load.
func (c *Compiler) collectFiddleCall(ctx context.Context, f *File, cls *Class, n *parser.CallNode, scope []*Class) {
	if n.Name != "extern" && n.Name != "bind" && n.Name != "typealias" || f.isPrelude() || !fiddleImporter(f, cls) {
		return
	}
	args := callArgs(n)
	lit := func(i int) string {
		s, ok := args[i].(*parser.StringNode)
		if !ok {
			c.errorf(f, n, "Fiddle::Importer#%s needs a literal C declaration string, read at compile time (decision 326)", n.Name)
		}
		return s.Unescaped.Value
	}
	if len(args) == 0 {
		return
	}
	if c.fiddleAliases == nil {
		c.fiddleAliases = map[*Class]map[string]string{}
	}
	alias := c.fiddleAliases[cls]
	if alias == nil {
		alias = map[string]string{}
		c.fiddleAliases[cls] = alias
	}
	if n.Name == "typealias" {
		if len(args) == 2 {
			alias[lit(0)] = lit(1)
		}
		return
	}
	name, ret, argTypes, err := fiddleSignature(lit(0), alias)
	if err != nil {
		c.errorf(f, n, "Fiddle::Importer#%s: %v", n.Name, err)
	}
	params := make([]string, 0, len(argTypes))
	types := make([]string, 0, len(argTypes))
	variadic := false
	for i, t := range argTypes {
		if t == 9 {
			if !c.env.CGO {
				c.errorf(f, n, "Fiddle::Importer#%s: a variadic C function needs --cgo (purego has no C varargs; decisions 326, 426)", n.Name)
			}
			variadic = true
			break
		}
		params = append(params, fmt.Sprintf("a%d", i))
		types = append(types, "untyped")
	}
	if c.fiddleSlots == nil {
		c.fiddleSlots = map[*Class]int{}
	}
	slot := c.fiddleSlots[cls]
	fn := fmt.Sprintf("__fiddle_func_at(%d, %q)", slot, name) // an Array slot, not a hash of the name per call (decision 329)
	c.fiddleSlots[cls]++
	call := fmt.Sprintf("%s.call(%s)", fn, strings.Join(params, ", "))
	switch n := len(params); { // Fiddle.__call's fixed arity: no *args Array per call (decision 329)
	case variadic: // type and value pairs after the fixed arguments, as MRI's Importer passes them (decision 426)
		params, types = append(params, "*rest"), append(types, "*untyped")
		call = fmt.Sprintf("%s.call(%s)", fn, strings.Join(params, ", "))
	case n >= 1 && n <= 3:
		call = fmt.Sprintf("::Fiddle.__call%d(%s)", n, strings.Join(append([]string{fn}, params...), ", "))
	case n <= 8:
		call = fmt.Sprintf("::Fiddle.__call(%s)", strings.Join(append([]string{fn, strconv.Itoa(n)}, params...), ", "))
	}
	src := fmt.Sprintf("#: (%s) -> %s\ndef self.%s(%s) = %s\n",
		strings.Join(types, ", "), fiddleRubyType(ret), name, strings.Join(params, ", "), call)
	sf, def := c.synthDef(ctx, f, n, "Fiddle function", src, synthOpt{Origin: f.synthOrigin(), NoPath: true})
	c.addMethod(sf, cls, def, false, scope)
	if !variadic {
		c.addFiddleTwin(ctx, f, cls, n, def, slot, name, ret, argTypes, scope)
	}
}

// fiddleTwin is an extern's typed twin: its method's name and the C argument types a call's arguments must match.
type fiddleTwin struct {
	name string
	args []int
}

// addFiddleTwin: `__fiddle_t_<name>` takes typed args so a call skips boxing; `__fiddle_c_<name>` falls back to the extern's path unless the slot is a plain Function of this signature (decision 442).
func (c *Compiler) addFiddleTwin(ctx context.Context, f *File, cls *Class, n *parser.CallNode, ext *parser.DefNode, slot int, name string, ret int, args []int, scope []*Class) {
	params := make([]string, 0, len(args))
	types := make([]string, 0, len(args))
	codes := []string{strconv.Itoa(ret)}
	var place strings.Builder
	ni, nf := 0, 0
	for i, t := range args {
		p := fmt.Sprintf("a%d", i)
		switch t {
		case 11: // bool: any object, by Ruby truthiness
			return
		case 7, 8:
			types = append(types, "Float")
			fmt.Fprintf(&place, "\t\tfl[%d] = rbFiddleFloatArg(%d, %s)\n", nf, t, p)
			nf++
		default:
			types = append(types, "Integer")
			fmt.Fprintf(&place, "\t\tin[%d] = rbFiddleIntArg(%s)\n", ni, p)
			ni++
		}
		params = append(params, p)
		codes = append(codes, strconv.Itoa(t))
	}
	if ni > 8 || nf > 8 {
		return // stack arguments: the spread call (decision 426)
	}
	// The extern's own path in Go, as its synthesized body calls it.
	slow := fmt.Sprintf("Fiddle_Meta___Call(Fiddle_class, f, %d, %s)", len(args), strings.Join(append(slices.Clone(params), slices.Repeat([]string{"nil"}, 8-len(args))...), ", "))
	if len(args) >= 1 && len(args) <= 3 {
		slow = fmt.Sprintf("Fiddle_Meta___Call%d(Fiddle_class, f, %s)", len(args), strings.Join(params, ", "))
	}
	var fast string
	switch ret {
	case 0:
		fast = "rbFiddleCallTyped(fn, &in, &fl)\n\t\treturn"
	case 1:
		fast = "r, _ := rbFiddleCallTyped(fn, &in, &fl)\n\t\treturn NewFiddle_Pointer(rbFiddleIntValue(1, r), 0, nil)"
		slow = fmt.Sprintf("return rbAs[Fiddle_PointerI](%s, %q)", slow, "Fiddle::Pointer")
	case 7, 8:
		fast = fmt.Sprintf("_, x := rbFiddleCallTyped(fn, &in, &fl)\n\t\treturn rbFiddleFloatValue(%d, x)", ret)
		slow = fmt.Sprintf("return rbAs[Float](%s, %q)", slow, "Float")
	case 11:
		fast = "r, _ := rbFiddleCallTyped(fn, &in, &fl)\n\t\treturn rbFiddleBoolValue(r)"
		slow = fmt.Sprintf("return Boolean(rbTruthy(%s))", slow)
	default:
		fast = fmt.Sprintf("r, _ := rbFiddleCallTyped(fn, &in, &fl)\n\t\treturn rbFiddleIntValue(%d, r)", ret)
		slow = fmt.Sprintf("return rbAs[Integer](%s, %q)", slow, "Integer")
	}
	rt, tname, cname := fiddleRubyType(ret), "__fiddle_t_"+name, "__fiddle_c_"+name
	lookup := fmt.Sprintf("__fiddle_func_at(%d, %q)", slot, name)
	src := fmt.Sprintf("#: (%s) -> %s\ndef self.%s(%s) = %s(%s)\n", strings.Join(types, ", "), rt, tname, strings.Join(params, ", "),
		cname, strings.Join(append([]string{lookup}, params...), ", "))
	src += fmt.Sprintf("#: (%s) -> %s\ndef self.%s(%s) = %%x{\n\tif fn := rbFiddleTyped(f, %s); fn != nil {\n\t\tvar in [8]uintptr\n\t\tvar fl [8]uint64\n%s\t\t%s\n\t}\n\t%s\n}\n",
		strings.Join(append([]string{"::Fiddle::Function"}, types...), ", "), rt, cname, strings.Join(append([]string{"f"}, params...), ", "),
		strings.Join(codes, ", "), place.String(), fast, slow)
	sf := c.synthFile(ctx, f, n, "Fiddle function", src, synthOpt{Origin: f.synthOrigin(), NoPath: true})
	sf.goBody = true
	for _, d := range c.synthDefs(f, n, "Fiddle function", sf) {
		c.addMethod(sf, cls, d, false, scope)
	}
	if c.fiddleTwins == nil {
		c.fiddleTwins = map[*parser.DefNode]fiddleTwin{}
	}
	c.fiddleTwins[ext] = fiddleTwin{name: tname, args: args}
}

// fiddleTwinCall picks the twin only when every arg is statically the class its C type holds, so no call changes meaning (decision 442).
func (f *fctx) fiddleTwinCall(e *entry, recv expr, args []parser.Node, block parser.Node) *entry {
	t, ok := f.c.fiddleTwins[e.M.Node]
	rc, isCls := recv.typ.(TClass)
	if !ok || !isCls || block != nil || len(args) != len(t.args) {
		return nil
	}
	for i, a := range args {
		if isSplat(a) {
			return nil
		}
		var x expr
		f.probe(func() { x = f.genExpr(a, nil) })
		want := "Integer"
		if t.args[i] == 7 || t.args[i] == 8 {
			want = "Float"
		}
		if c, ok := x.typ.(TClass); !ok || c.C.Name != want {
			return nil
		}
	}
	return rc.C.lookup(t.name)
}

// fiddleMember is one struct member: name, TYPE_* code and array length (0: a scalar).
type fiddleMember struct {
	name  string
	code  int
	count int
}

var (
	fiddleScalarMember = regexp.MustCompile(`^[\w*\s]+[*\s](\w+)$`)
	fiddleFuncMember   = regexp.MustCompile(`^[\w*\s]+\(\*(\w+)\)\(.*?\)$`)
	fiddleArrayMember  = regexp.MustCompile(`^([\w*\s]+[*\s])(\w+)\[(\d+)\]$`)
)

// fiddleSizes are SIZE_MAP's byte sizes, which are also ALIGN_MAP's alignments on LP64.
var fiddleSizes = map[int]int{1: 8, 2: 1, -2: 1, 3: 2, -3: 2, 4: 4, -4: 4, 5: 8, -5: 8, 6: 8, -6: 8, 7: 4, 8: 8, 11: 1}

// fiddleStructMember is parse_struct_signature for one member string.
func fiddleStructMember(sig string, alias map[string]string) (fiddleMember, error) {
	sig = strings.TrimSpace(fiddlePunct.ReplaceAllString(fiddleSpaces.ReplaceAllString(sig, " "), "$1"))
	var m fiddleMember
	var ty string
	switch {
	case fiddleScalarMember.MatchString(sig):
		m.name, ty = fiddleScalarMember.FindStringSubmatch(sig)[1], sig
	case fiddleFuncMember.MatchString(sig):
		m.name, ty = fiddleFuncMember.FindStringSubmatch(sig)[1], sig
	case fiddleArrayMember.MatchString(sig):
		g := fiddleArrayMember.FindStringSubmatch(sig)
		m.name, ty = g[2], strings.TrimSpace(g[1])
		_, _ = fmt.Sscan(g[3], &m.count)
	default:
		return m, fmt.Errorf("can't parse the struct member: %s", sig)
	}
	code, ok := fiddleCType(ty, alias)
	if !ok || fiddleSizes[code] == 0 {
		return m, fmt.Errorf("unknown type: %s", ty)
	}
	m.code = code
	return m, nil
}

// collectFiddleStruct declares `Name = struct [...]`/`union` in an Importer module as CStructBuilder would, typed.
func (c *Compiler) collectFiddleStruct(ctx context.Context, f *File, n *parser.ConstantWriteNode, scope []*Class) bool {
	call, ok := n.Value.(*parser.CallNode)
	if !ok || call.Receiver != nil || call.Name != "struct" && call.Name != "union" || f.isPrelude() || len(scope) == 0 || !fiddleImporter(f, scope[len(scope)-1]) {
		return false
	}
	cls := scope[len(scope)-1]
	args := callArgs(call)
	arr, ok := firstArg(args).(*parser.ArrayNode)
	if !ok || len(args) != 1 {
		c.errorf(f, call, "Fiddle::Importer#%s needs a literal Array of member declarations (decision 326)", call.Name)
	}
	alias := c.fiddleAliases[cls]
	members := make([]fiddleMember, 0, len(arr.Elements))
	for _, el := range arr.Elements {
		s, ok := el.(*parser.StringNode)
		if !ok {
			c.errorf(f, el, "Fiddle::Importer#%s: a member must be a literal String (nested structs are not supported; decision 326)", call.Name)
		}
		m, err := fiddleStructMember(s.Unescaped.Value, alias)
		if err != nil {
			c.errorf(f, el, "Fiddle::Importer#%s: %v", call.Name, err)
		}
		members = append(members, m)
	}
	c.addFiddleStruct(ctx, f, n, call, members, call.Name == "union", scope)
	return true
}

// fiddleReserved are methods a struct member must not replace (MRI skips a member whose name is already defined).
var fiddleReserved = map[string]bool{"to_ptr": true, "to_i": true, "to_h": true, "each": true, "each_pair": true, "replace": true,
	"class": true, "hash": true, "inspect": true, "to_s": true, "send": true, "freeze": true, "dup": true, "clone": true, "method": true}

// addFiddleStruct lays the members out as CStructBuilder does (LP64) and collects the class.
func (c *Compiler) addFiddleStruct(ctx context.Context, f *File, n *parser.ConstantWriteNode, call *parser.CallNode, members []fiddleMember, union bool, scope []*Class) {
	var b strings.Builder
	super := "Fiddle::CStruct"
	if union {
		super = "Fiddle::CUnion"
	}
	offset, size, maxAlign := 0, 0, 1
	offsets := make([]int, len(members))
	for i, m := range members {
		align, n := fiddleSizes[m.code], fiddleSizes[m.code]*max(m.count, 1)
		maxAlign = max(maxAlign, align)
		if union {
			size = max(size, n)
			continue
		}
		offset = (offset + align - 1) / align * align
		offsets[i] = offset
		offset += n
	}
	if !union {
		size = (offset + maxAlign - 1) / maxAlign * maxAlign
	}
	names, types, offs := make([]string, 0, len(members)), make([]string, 0, len(members)), make([]string, 0, len(members))
	gets, sets := make([]string, 0, len(members)), make([]string, 0, len(members))
	for i, m := range members {
		names = append(names, fmt.Sprintf("%q", m.name))
		t := strconv.Itoa(m.code)
		if m.count > 0 {
			t = fmt.Sprintf("[%d, %d]", m.code, m.count)
		}
		types = append(types, t)
		offs = append(offs, fmt.Sprintf("%q => %d", m.name, offsets[i]))
		gets = append(gets, fmt.Sprintf("    when %q then %s", m.name, fiddleGetter(m, offsets[i])))
		sets = append(sets, fmt.Sprintf("    when %q then %s", m.name, fiddleSetter(m, offsets[i], "v")))
	}
	fmt.Fprintf(&b, "class %s < %s\n", n.Name, super)
	fmt.Fprintf(&b, "  #: () -> Integer\n  def self.size = %d\n", size)
	fmt.Fprintf(&b, "  #: () -> Integer\n  def self.alignment = %d\n", maxAlign)
	fmt.Fprintf(&b, "  #: () -> Array[String]\n  def self.members = [%s]\n", strings.Join(names, ", "))
	fmt.Fprintf(&b, "  #: () -> Array[untyped]\n  def self.types = [%s]\n", strings.Join(types, ", "))
	fmt.Fprintf(&b, "  #: (String) -> Integer?\n  def self.offsetof(name)\n    t = {%s} #: Hash[String, Integer]\n    t[name]\n  end\n", strings.Join(offs, ", "))
	fmt.Fprintf(&b, "  #: (?untyped) -> %s\n  def self.malloc(free = nil) = new(Fiddle::Pointer.malloc(%d), free)\n", n.Name, size)
	fmt.Fprintf(&b, "  #: (untyped, ?untyped) -> void\n  def initialize(addr, free = nil)\n    @entity = Fiddle::CStructEntity.new(addr, %d, free)\n  end\n", size)
	fmt.Fprintf(&b, "  #: () -> Array[String]\n  def __members = %s.members\n", n.Name)
	b.WriteString("  #: () -> Fiddle::CStructEntity\n  def to_ptr = @entity\n")
	fmt.Fprintf(&b, "  #: (String) -> untyped\n  def [](name)\n    case name\n%s\n    else raise ArgumentError, \"no such member: #{name}\"\n    end\n  end\n", strings.Join(gets, "\n"))
	fmt.Fprintf(&b, "  #: (String, untyped) -> untyped\n  def []=(name, v)\n    case name\n%s\n    else raise ArgumentError, \"no such member: #{name}\"\n    end\n    v\n  end\n", strings.Join(sets, "\n"))
	for i, m := range members {
		if fiddleReserved[m.name] {
			continue
		}
		rt, at := fiddleMemberType(m)
		fmt.Fprintf(&b, "  #: () -> %s\n  def %s = %s\n", rt, m.name, fiddleGetter(m, offsets[i]))
		fmt.Fprintf(&b, "  #: (%s) -> %s\n  def %s=(v)\n    %s\n    v\n  end\n", at, at, m.name, fiddleSetter(m, offsets[i], "v"))
	}
	b.WriteString("end\n")
	sf := c.synthFile(ctx, f, call, "Fiddle struct", b.String(), synthOpt{Origin: f.synthOrigin(), NoPath: true})
	cn, ok := sf.Root.Statements.Body[0].(*parser.ClassNode)
	if !ok {
		c.errorf(f, call, "Fiddle struct: synthesized class did not parse")
	}
	c.collectClass(ctx, sf, cn, scope)
}

// fiddleMemberType is a member's reader and writer types.
func fiddleMemberType(m fiddleMember) (string, string) {
	el := "Integer"
	switch m.code {
	case 7, 8:
		el = "Float"
	case 1:
		el = "Fiddle::Pointer"
	}
	if m.count > 0 {
		if m.code == 1 {
			return "Array[Fiddle::Pointer]", "Array[untyped]"
		}
		return "Array[" + el + "]", "Array[" + el + "]"
	}
	if m.code == 1 {
		return el, "untyped"
	}
	return el, el
}

// fiddleGetter reads a member: a scalar, or each element of an array.
func fiddleGetter(m fiddleMember, off int) string {
	one := func(at string) string {
		switch m.code {
		case 7, 8:
			return fmt.Sprintf("@entity.__get_f(%s, %d)", at, m.code)
		case 1:
			return fmt.Sprintf("Fiddle::Pointer.new(@entity.__get_i(%s, 1))", at)
		}
		return fmt.Sprintf("@entity.__get_i(%s, %d)", at, m.code)
	}
	if m.count == 0 {
		return one(strconv.Itoa(off))
	}
	return fmt.Sprintf("Array.new(%d) { |i| %s }", m.count, one(fmt.Sprintf("%d + i * %d", off, fiddleSizes[m.code])))
}

// fiddleSetter writes a member from v.
func fiddleSetter(m fiddleMember, off int, v string) string {
	one := func(at, x string) string {
		switch m.code {
		case 7, 8:
			return fmt.Sprintf("@entity.__set_f(%s, %d, %s)", at, m.code, x)
		case 1:
			return fmt.Sprintf("@entity.__set_i(%s, 1, Fiddle.__addr(%s))", at, x)
		}
		return fmt.Sprintf("@entity.__set_i(%s, %d, %s)", at, m.code, x)
	}
	if m.count == 0 {
		return one(strconv.Itoa(off), v)
	}
	return fmt.Sprintf("Array(%s).each_with_index { |x, i| %s if i < %d }", v, one(fmt.Sprintf("%d + i * %d", off, fiddleSizes[m.code]), "x"), m.count)
}
