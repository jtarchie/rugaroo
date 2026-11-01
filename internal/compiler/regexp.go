package compiler

import (
	_ "embed"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// rubyRegexFlags carries a literal's options.
type rubyRegexFlags struct {
	ignoreCase, multiline, extended, binary bool
	fixed                                   string // /u, /e or /s: the encoding it is fixed to (decision 368), not shown by inspect
}

type regexpLiteral interface {
	IsIGNORE_CASE() bool
	IsMULTI_LINE() bool
	IsEXTENDED() bool
	IsASCII_8BIT() bool
	IsUTF_8() bool
	IsEUC_JP() bool
	IsWINDOWS_31J() bool
}

// regexFlagsOf reads the flags of a regexp literal, interpolated or not.
func regexFlagsOf(n regexpLiteral) rubyRegexFlags {
	fixed := ""
	switch {
	case n.IsUTF_8():
		fixed = "u"
	case n.IsEUC_JP():
		fixed = "e"
	case n.IsWINDOWS_31J():
		fixed = "s"
	}
	return rubyRegexFlags{n.IsIGNORE_CASE(), n.IsMULTI_LINE(), n.IsEXTENDED(), n.IsASCII_8BIT(), fixed}
}

func (r rubyRegexFlags) goPrefix() string {
	// Ruby's ^ and $ always match at line boundaries: Go's (?m). Ruby's /m
	// (dot matches newline) is Go's (?s).
	p := "(?m"
	if r.ignoreCase {
		p += "i"
	}
	if r.multiline {
		p += "s"
	}
	return p + ")"
}

func (r rubyRegexFlags) opts() string {
	o := ""
	if r.multiline {
		o += "m"
	}
	if r.ignoreCase {
		o += "i"
	}
	if r.extended {
		o += "x"
	}
	if r.binary { // /n: matches a binary String byte by byte (decision 324)
		o += "n"
	}
	return o + r.fixed
}

//go:embed rxtranslate.go
var rxTranslateSrc string

//go:embed rxalt.go
var rxAltSrc string

// The compiler validates patterns with the rxAlt engine too: rxAltReady's initializer registers it.
var _ = rxAltReady

// rxAltGo is rxalt.go minus its package clause and imports, for programs that need the rxAlt engine (decision 242).
func rxAltGo() string {
	return rxAltSrc[strings.Index(rxAltSrc, "\n)\n")+3:]
}

// rxTranslateGo is rxtranslate.go minus its package clause and imports
// (goimports restores them), for emitProgram. The generated lint config
// excludes G115, so the file's nolint for it would be flagged as unused.
func rxTranslateGo() string {
	code := rxTranslateSrc[strings.Index(rxTranslateSrc, "\n)\n")+3:]
	return regexp.MustCompile(` //nolint:gosec //.*`).ReplaceAllString(code, "")
}

// rxUnsupported are the RE2 parse errors an interpolated regexp's static parts can cause on their own: syntax that
// neither engine takes (possessive quantifiers, \g). Others (unbalanced groups or classes) an interpolated value may
// cause or cure, so they wait for run time.
var rxUnsupported = []syntax.ErrorCode{syntax.ErrInvalidPerlOp, syntax.ErrInvalidNamedCapture, syntax.ErrInvalidEscape, syntax.ErrInvalidRepeatOp}

// regexpSource is a literal's Regexp#source: as in MRI's lexer, an escaped
// terminator loses its backslash unless it is a regexp metacharacter
// (/a\/b/ is "a/b"; %r{a\}b} keeps "a\\}b").
func regexpSource(raw string, term byte) string {
	if strings.IndexByte("$*+.?^|)]}>", term) >= 0 {
		return raw
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			i++
			if raw[i] != term {
				b.WriteByte('\\')
			}
		}
		b.WriteByte(raw[i])
	}
	return b.String()
}

// literalGroups is the number of capture groups in a static regexp
// literal; ok is false for anything else.
func (f *fctx) literalGroups(n parser.Node) (int, bool) {
	file := f.f
	if _, isConst := n.(*parser.ConstantReadNode); isConst || isConstPath(n) { // `s.scan(CVE_PATTERN)`: the constant's literal
		var k *Const
		if catchCompileError(func() { _, k = f.c.lookupConst(f.f, n, f.lex) }) != nil || k == nil || k.File == nil {
			return 0, false
		}
		file, n = k.File, k.Value
		if c, ok := n.(*parser.CallNode); ok && c.Name == "freeze" && len(callArgs(c)) == 0 && c.Receiver != nil {
			n = c.Receiver
		}
	}
	var src string
	var closing byte
	var ext bool
	switch r := n.(type) {
	case *parser.RegularExpressionNode:
		src, closing, ext = file.text(r.ContentLoc), file.text(r.ClosingLoc)[0], r.IsEXTENDED()
	case *parser.InterpolatedRegularExpressionNode: // the literal parts' groups, and an interpolated Regexp constant's; other interpolations count as plain text
		var b strings.Builder
		for _, p := range r.Parts {
			if s, ok := p.(*parser.StringNode); ok {
				b.WriteString(file.text(s.ContentLoc))
			} else if g, ok := f.embeddedConstGroups(file, p); ok {
				b.WriteString(strings.Repeat("()", g)) // `/(?:\A|\s)(?:#{PAIR}|#{REF})/`: the groups are PAIR's and REF's
			} else {
				b.WriteString("x")
			}
		}
		src, closing, ext = b.String(), file.text(r.ClosingLoc)[0], r.IsEXTENDED()
	default:
		return 0, false
	}
	goPat, err := rxPrepare(regexpSource(src, closing), ext)
	if err != nil {
		return 0, false
	}
	re, err := rxCompile(goPat)
	if err != nil {
		return 0, false
	}
	return re.NumSubexp(), true
}

// embeddedConstGroups is the group count of a `#{CONST}` part whose constant is a Regexp literal, read in file's scope.
func (f *fctx) embeddedConstGroups(file *File, p parser.Node) (int, bool) {
	e, ok := p.(*parser.EmbeddedStatementsNode)
	if !ok || e.Statements == nil || len(e.Statements.Body) != 1 {
		return 0, false
	}
	n := e.Statements.Body[0]
	if _, isConst := n.(*parser.ConstantReadNode); !isConst && !isConstPath(n) {
		return 0, false
	}
	if file != f.f { // a constant's own literal, in another file: its parts resolve there
		return 0, false
	}
	return f.literalGroups(n)
}

// genStaticRegexp is a literal without interpolation: validated now, compiled once into a package variable.
func (f *fctx) genStaticRegexp(n *parser.RegularExpressionNode) expr {
	flags := regexFlagsOf(n)
	src := regexpSource(f.f.text(n.ContentLoc), f.f.text(n.ClosingLoc)[0])
	pat := src
	if !n.IsASCII_8BIT() {
		var bad bool
		if pat, bad = multibyteEscapes(src); bad {
			f.errorf(n, "invalid multibyte escape: /%s/ (MRI's SyntaxError; /n matches bytes, decision 352)", src)
		}
	}
	goPat, err := rxPrepare(pat, flags.extended) // source and inspect keep the spacing; only the matcher drops it
	alt := false
	if err == nil {
		var rx *rxRegexp
		rx, err = rxCompile(flags.goPrefix() + goPat)
		alt = rx != nil && rx.alt != nil
	}
	if err != nil && f.lenient() {
		return expr{code: fmt.Sprintf("rbRegexpLenient(%s, %s, %q)", strconv.Quote(flags.goPrefix()), strconv.Quote(src), flags.opts()), typ: f.cls("Regexp")}
	}
	if err != nil {
		f.errorf(n, "regexp /%s/ is not supported: %v", src, err)
	}
	args := fmt.Sprintf("%s, %s, %q", strconv.Quote(flags.goPrefix()+goPat), strconv.Quote(src), flags.opts())
	name, ok := f.c.regexpVars[args]
	if !ok {
		h := fnv.New64a()
		_, _ = h.Write([]byte(args))                // a hash.Hash never fails
		name = fmt.Sprintf("rbRe_%016x", h.Sum64()) // by pattern, not by when it was first rendered (decision 432)
		f.c.regexpVars[args] = name
		init := "rbRegexpNew(" + args + ")"
		if alt { // after the regexp2 engine registers, which naming it keeps (decision 242)
			init = "rbRxAlt(rxAltReady, " + init + ")"
		}
		f.c.addPkgVar("var " + name + " = " + init)
	}
	return expr{code: name, typ: f.cls("Regexp")}
}

// multibyteEscapes is src with each run of \xHH escapes past 0x7F that is valid UTF-8 written as its characters, as
// MRI reads `/\xc3\xa9/` (Go's \xC3 is U+00C3); bad reports a run that is not valid UTF-8 (`/\xff/`), which MRI
// rejects without /n.
func multibyteEscapes(src string) (string, bool) {
	var b strings.Builder
	var run []byte
	bad := false
	flush := func() {
		switch {
		case len(run) == 0:
		case utf8.Valid(run):
			b.WriteString(regexp.QuoteMeta(string(run)))
		default:
			bad = true
		}
		run = run[:0]
	}
	for i := 0; i < len(src); i++ {
		if src[i] != '\\' || i+1 >= len(src) {
			flush()
			b.WriteByte(src[i])
			continue
		}
		if src[i+1] == 'x' && i+3 < len(src) {
			v, err := strconv.ParseUint(src[i+2:i+4], 16, 8)
			if err == nil && v >= 0x80 {
				run = append(run, byte(v))
				i += 3
				continue
			}
		}
		flush()
		b.WriteString(src[i : i+2]) // the escaped character
		i++
	}
	flush()
	return b.String(), bad
}

// genRegexp renders a regexp literal. Static ones are validated now and
// compiled once into a package variable.
func (f *fctx) genRegexp(n parser.Node) expr {
	var flags rubyRegexFlags
	var parts []parser.Node
	var term byte
	var once bool
	switch n := n.(type) {
	case *parser.RegularExpressionNode:
		return f.genStaticRegexp(n)
	case *parser.InterpolatedRegularExpressionNode:
		flags = regexFlagsOf(n)
		parts = n.Parts
		term = f.f.text(n.ClosingLoc)[0]
		once = n.IsONCE()
	}
	// Interpolated values are inserted raw, as Ruby does, so the joined
	// source is translated at run time (rbRegexpDyn); each value is
	// evaluated once. The static parts are checked now, with a letter
	// standing in for each value: valid wherever one plausibly sits (an
	// atom, a flag, a group name).
	var srcParts []string
	var probe strings.Builder
	alt := false
	for _, p := range parts {
		switch p := p.(type) {
		case *parser.StringNode:
			raw := regexpSource(f.f.text(p.Location), term)
			probe.WriteString(raw)
			srcParts = append(srcParts, strconv.Quote(raw))
		case *parser.EmbeddedStatementsNode:
			if p.Statements == nil || len(p.Statements.Body) != 1 {
				f.errorf(p, "interpolation must contain a single expression")
			}
			e := f.genExpr(p.Statements.Body[0], nil)
			if c, ok := p.Statements.Body[0].(*parser.CallNode); !f.f.isPrelude() && (ok && c.Name == "source" || isAny(e.typ) || isClass(e.typ, "Regexp")) {
				alt = true // another pattern's text may hold what RE2 rejects (decision 242)
			}
			probe.WriteString("i") // under /x too: stripping a letter leaves it
			srcParts = append(srcParts, "string("+f.toS(p.Statements.Body[0], e)+")")
		default:
			f.c.unsupported(f.f, p)
		}
	}
	goPat, err := rxPrepare(probe.String(), flags.extended)
	if err == nil {
		var rx *rxRegexp
		rx, err = rxCompile(flags.goPrefix() + goPat)
		alt = alt || rx != nil && rx.alt != nil || f.lenient() && !f.f.isPrelude()
		var se *syntax.Error
		if errors.As(err, &se) && !slices.Contains(rxUnsupported, se.Code) {
			err = nil
		}
	}
	if err != nil && !f.lenient() {
		f.errorf(n, "regexp is not supported: %v", err)
	}
	if len(srcParts) == 0 {
		srcParts = []string{`""`}
	}
	ctor := "rbRegexpDyn"
	if f.lenient() {
		ctor = "rbRegexpLenient"
	}
	code := fmt.Sprintf("%s(%s, %s, %q)", ctor, strconv.Quote(flags.goPrefix()), strings.Join(srcParts, " + "), flags.opts())
	if alt { // names the regexp2 engine, so the pruner keeps it (decision 242)
		code = "rbRxAlt(rxAltReady, " + code + ")"
	}
	if once {
		// /o interpolates on first evaluation and keeps that Regexp. Not
		// sync.Once: a raise (RegexpError) must leave it unset to retry, as
		// MRI does; the lock is because threads are goroutines.
		name := fmt.Sprintf("rbReO%d_%d", slices.Index(f.c.files, f.f), n.GetLocation().StartOffset) // one per literal, MRI's rule
		f.c.addPkgVar("var " + name + " struct {\n\tsync.Mutex\n\tre *Regexp\n}")
		code = fmt.Sprintf("func() *Regexp {\n%[1]s.Lock()\ndefer %[1]s.Unlock()\nif %[1]s.re == nil {\n%[1]s.re = %[2]s\n}\nreturn %[1]s.re\n}()", name, code)
	}
	return expr{code: code, typ: f.cls("Regexp")}
}

func isConstPath(n parser.Node) bool { _, ok := n.(*parser.ConstantPathNode); return ok }
