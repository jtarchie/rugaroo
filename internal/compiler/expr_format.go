package compiler

import (
	"strconv"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// fmtDirective is one `%[flags][width][.prec]verb` of a literal format string; text is the literal text before it.
type fmtDirective struct {
	text        string
	flags       string
	width, prec int // prec -1: none
	verb        byte
}

// spec is rbFormatSpec's rendering, which rbFormatOne reads.
func (d fmtDirective) spec() string {
	s := "%" + d.flags
	if d.width > 0 {
		s += strconv.Itoa(d.width)
	}
	if d.prec >= 0 {
		s += "." + strconv.Itoa(d.prec)
	}
	return s
}

// parseLiteralFormat is false for whatever rbFormat must see at run time: `*`, `N$`, `%<name>`, a malformed directive's ArgumentError.
func parseLiteralFormat(s string) (dirs []fmtDirective, tail string, ok bool) {
	var text strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			text.WriteByte(s[i])
			continue
		}
		i++
		if i < len(s) && s[i] == '%' {
			text.WriteByte('%')
			continue
		}
		d := fmtDirective{prec: -1}
		for ; i < len(s) && strings.IndexByte("-+ 0#", s[i]) >= 0; i++ {
			d.flags += string(s[i])
		}
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			d.width = d.width*10 + int(s[i]-'0')
			if d.width > 4096 { // before the digit run can wrap
				return nil, "", false
			}
		}
		if i < len(s) && s[i] == '.' {
			d.prec = 0
			for i++; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
				d.prec = d.prec*10 + int(s[i]-'0')
				if d.prec > 4096 {
					return nil, "", false
				}
			}
		}
		if i >= len(s) || strings.IndexByte("sdiuxXfeEgGoboBcp", s[i]) < 0 || d.width > 4096 || d.prec > 4096 {
			return nil, "", false
		}
		d.verb, d.text = s[i], text.String()
		text.Reset()
		dirs = append(dirs, d)
	}
	return dirs, text.String(), true
}

// genLiteralFormat lowers a literal format string to appends (decision 377): no boxed arguments, no parse at run time.
func (f *fctx) genLiteralFormat(n *parser.CallNode) (expr, bool) {
	lit, args := f.literalFormatCall(n)
	if lit == nil {
		return expr{}, false
	}
	dirs, tail, ok := parseLiteralFormat(lit.Unescaped.Value)
	if !ok || len(dirs) != len(args) || len(dirs) == 0 {
		return expr{}, false
	}
	for _, a := range args {
		if _, splat := a.(*parser.SplatNode); splat {
			return expr{}, false
		}
		if _, kw := a.(*parser.KeywordHashNode); kw {
			return expr{}, false
		}
	}
	vals, types := make([]string, len(args)), make([]Type, len(args))
	for i, a := range args { // every argument first, left to right, as Ruby evaluates them before formatting
		t := f.probeType(a, nil)
		types[i] = t
		v := f.genExpr(a, nil)
		tmp := f.newTmp()
		if f.fmtScalar(t) { // typed: an untyped Go constant would default to string or int
			f.emit("var %s %s = %s", tmp, f.c.goType(t), f.coerce(a, v, t))
		} else {
			f.emit("var %s any = %s", tmp, f.coerce(a, v, TAny{}))
		}
		vals[i] = tmp
	}
	size := len(tail)
	for _, d := range dirs {
		size += len(d.text) + max(d.width, 8)
	}
	buf := f.newTmp()
	f.emit("%s := make([]byte, 0, %d)", buf, size)
	for i, d := range dirs {
		if d.text != "" {
			f.emit("%s = append(%s, %s...)", buf, buf, strconv.Quote(d.text))
		}
		f.emit("%s = %s", buf, f.fmtCall(buf, d, vals[i], types[i]))
	}
	if tail != "" {
		f.emit("%s = append(%s, %s...)", buf, buf, strconv.Quote(tail))
	}
	return expr{code: "String(" + buf + ")", typ: f.cls("String")}, true
}

// literalFormatCall is the format string and arguments of `format("lit", ...)`, `sprintf` or `"lit" % x`, when they
// are the prelude's methods; lit is nil for any other call.
func (f *fctx) literalFormatCall(n *parser.CallNode) (*parser.StringNode, []parser.Node) {
	if n.Block != nil {
		return nil, nil
	}
	switch {
	case (n.Name == "format" || n.Name == "sprintf") && n.Receiver == nil && len(callArgs(n)) > 0:
		e := f.resolve(f.selfType, n.Name)
		if e == nil || e.M.Owner == nil || e.M.Owner.Name != "Kernel" || e.M.File == nil || !e.M.File.isPrelude() {
			return nil, nil
		}
		lit, _ := callArgs(n)[0].(*parser.StringNode)
		return lit, callArgs(n)[1:]
	case n.Name == "%" && len(callArgs(n)) == 1:
		lit, _ := n.Receiver.(*parser.StringNode)
		e := f.resolve(f.cls("String"), "%")
		if lit == nil || e == nil || e.M.File == nil || !e.M.File.isPrelude() {
			return nil, nil
		}
		args := callArgs(n)
		if a, ok := args[0].(*parser.ArrayNode); ok {
			return lit, a.Elements
		}
		if !f.fmtScalar(f.probeType(args[0], nil)) { // an Array value spreads at run time
			return nil, nil
		}
		return lit, args
	}
	return nil, nil
}

func (f *fctx) fmtScalar(t Type) bool {
	return isClass(t, "Integer") || isClass(t, "Float") || isClass(t, "String")
}

// fmtCall is the helper call appending one directive's text for v (of type t) to buf.
func (f *fctx) fmtCall(buf string, d fmtDirective, v string, t Type) string {
	left, zero := strings.Contains(d.flags, "-"), strings.Contains(d.flags, "0")
	plain := strings.Trim(d.flags, "-0") == "" // + space # take rbFormat's path
	verb := strconv.QuoteRune(rune(d.verb))
	b := strconv.FormatBool
	switch {
	case plain && d.prec < 0 && isClass(t, "Integer") && strings.IndexByte("diuxX", d.verb) >= 0:
		return "rbFmtInt(" + buf + ", " + v + ", " + verb + ", " + strconv.Itoa(d.width) + ", " + b(left) + ", " + b(zero) + ", " + strconv.Quote(d.spec()) + ")"
	case plain && isClass(t, "Float") && d.verb == 'f':
		prec := d.prec
		if prec < 0 {
			prec = 6
		}
		return "rbFmtFloat(" + buf + ", " + v + ", " + strconv.Itoa(prec) + ", " + strconv.Itoa(d.width) + ", " + b(left) + ", " + b(zero) + ", " + strconv.Quote(d.spec()) + ")"
	case d.flags == "" || d.flags == "-":
		if d.prec < 0 && d.verb == 's' && isClass(t, "String") {
			return "rbFmtStr(" + buf + ", " + v + ", " + strconv.Itoa(d.width) + ", " + b(left) + ")"
		}
	}
	return "rbFmtAny(" + buf + ", " + v + ", " + verb + ", " + strconv.Quote(d.spec()) + ")"
}

// genStrChain compiles `a + b + c ...` of Strings to one rbConcat (decision 377): one allocation, not one per `+`.
func (f *fctx) genStrChain(n *parser.CallNode) (expr, bool) {
	var ops []parser.Node
	for c := n; ; {
		if c.Name != "+" || c.Block != nil || len(callArgs(c)) != 1 || c.Receiver == nil {
			return expr{}, false
		}
		ops = append(ops, callArgs(c)[0])
		r, ok := c.Receiver.(*parser.CallNode)
		if !ok || r.Name != "+" || r.Block != nil || len(callArgs(r)) != 1 || r.Receiver == nil {
			ops = append(ops, c.Receiver)
			break
		}
		c = r
	}
	if len(ops) < 3 {
		return expr{}, false
	}
	str := f.cls("String")
	if e := f.resolve(str, "+"); e == nil || e.M.File == nil || !e.M.File.isPrelude() {
		return expr{}, false
	}
	for _, o := range ops {
		if t := f.probeType(o, nil); !isClass(t, "String") {
			return expr{}, false
		}
	}
	codes := make([]string, len(ops))
	for i := range ops { // ops run right to left; Ruby evaluates the leftmost first
		o := ops[len(ops)-1-i]
		codes[i] = f.coerce(o, f.genExpr(o, str), str)
	}
	return expr{code: "rbConcat(" + strings.Join(codes, ", ") + ")", typ: str}, true
}
