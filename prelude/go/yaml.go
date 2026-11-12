//go:build rugaroo_prelude

package prelude

import (
	"encoding/base64"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// YAML for yaml.rb (decision 376): go-yaml parses to an AST, these helpers revive it by Psych's YAML 1.1 rules, and rbYAMLDump is a small emitter of its own.

// rbYAMLLoader is one load's options: Psych's ToRuby visitor plus its Restricted class loader.
type rbYAMLLoader struct {
	file                               *String
	classes                            map[string]bool // permitted class names; nil: unrestricted (unsafe_load, load_stream)
	symbols                            map[string]bool // permitted_symbols; empty: any symbol, once Symbol is permitted
	aliases, symbolizeNames, strictInt bool
	parseSymbols, freeze               bool
	anchors                            map[string]any
	fallback                           any
}

// rbYAMLNewLoader: permitted nil is an unrestricted load; yaml.rb has turned permitted_classes and permitted_symbols into names.
func rbYAMLNewLoader(opts *Hash[Symbol, any], permitted, symbols *Array[String]) *rbYAMLLoader {
	l := &rbYAMLLoader{parseSymbols: true, anchors: map[string]any{}, symbols: map[string]bool{}}
	if permitted != nil {
		l.classes = map[string]bool{}
		for _, c := range permitted.s {
			l.classes[string(c)] = true
		}
	} else {
		l.aliases, l.fallback = true, Boolean(false) // unsafe_load's fallback is false
	}
	if symbols != nil {
		for _, c := range symbols.s {
			l.symbols[string(c)] = true
		}
	}
	if opts == nil {
		return l
	}
	for k, v := range rbHashPairs(opts) {
		switch k {
		case "aliases":
			l.aliases = rbTruthy(v)
		case "filename":
			if v != nil {
				s := rbToS(v)
				l.file = &s
			}
		case "fallback":
			l.fallback = v
		case "symbolize_names":
			l.symbolizeNames = rbTruthy(v)
		case "freeze":
			l.freeze = rbTruthy(v)
		case "strict_integer":
			l.strictInt = rbTruthy(v)
		case "parse_symbols":
			l.parseSymbols = rbTruthy(v)
		}
	}
	return l
}

// rbYAMLLoad is Psych.safe_load (permitted non-nil) or unsafe_load (nil): the first document, or fallback when there is none.
func rbYAMLLoad(src string, opts *Hash[Symbol, any], unsafe bool, permitted, symbols *Array[String]) any {
	if unsafe {
		permitted = nil
	}
	l := rbYAMLNewLoader(opts, permitted, symbols)
	docs := l.parse(src)
	if len(docs) == 0 || rbYAMLNoDoc(docs[0]) {
		return l.fallback
	}
	return l.document(docs[0])
}

// rbYAMLLoadStream is Psych.load_stream: every document, unrestricted.
func rbYAMLLoadStream(src string, opts *Hash[Symbol, any]) *Array[any] {
	l := rbYAMLNewLoader(opts, nil, nil)
	out := &Array[any]{}
	for _, d := range l.parse(src) {
		if !rbYAMLNoDoc(d) {
			out.s = append(out.s, l.document(d))
		}
	}
	return out
}

// rbYAMLNoDoc: go-yaml's stand-in for an empty or comment-only stream; an explicit `---` with no node is a document whose value is nil.
func rbYAMLNoDoc(d *ast.DocumentNode) bool { return d.Start == nil && d.Body == nil }

func (l *rbYAMLLoader) document(d *ast.DocumentNode) any {
	clear(l.anchors)
	v := l.node(d.Body)
	if l.freeze {
		rbYAMLFreeze(v)
	}
	return v
}

func (l *rbYAMLLoader) parse(src string) []*ast.DocumentNode {
	f, err := parser.ParseBytes([]byte(src), 0, parser.AllowDuplicateMapKey()) // later keys win, as Psych
	if err != nil {
		var se interface {
			GetMessage() string
			GetToken() *token.Token
		}
		line, col, offset, problem := 0, 0, 0, err.Error()
		if errors.As(err, &se) {
			problem = se.GetMessage()
			if tk := se.GetToken(); tk != nil {
				line, col, offset = tk.Position.Line, tk.Position.Column, tk.Position.Offset
			}
		}
		panic(NewPsych_SyntaxError(l.file, Integer(line), Integer(col), Integer(offset), String(problem), String("while parsing")))
	}
	return f.Docs
}

func (l *rbYAMLLoader) node(n ast.Node) any {
	switch n := n.(type) {
	case nil:
		return nil
	case *ast.AnchorNode:
		v := l.node(n.Value)
		l.anchors[n.Name.GetToken().Value] = v
		return v
	case *ast.AliasNode:
		name := n.Value.GetToken().Value
		if !l.aliases {
			panic(NewPsych_AliasesNotEnabled())
		}
		v, ok := l.anchors[name]
		if !ok {
			panic(NewPsych_AnchorNotDefined(String(name)))
		}
		return v
	case *ast.TagNode:
		return l.tagged(n)
	case *ast.MappingNode:
		return l.mapping(n.Values)
	case *ast.MappingValueNode:
		return l.mapping([]*ast.MappingValueNode{n})
	case *ast.SequenceNode:
		arr := &Array[any]{s: make([]any, 0, len(n.Values))}
		for _, e := range n.Values {
			arr.s = append(arr.s, l.node(e))
		}
		return arr
	case *ast.LiteralNode:
		return String(n.Value.Value)
	case *ast.StringNode:
		if rbYAMLQuoted(n.GetToken()) {
			return String(n.Value)
		}
		return l.scalar(n.Value)
	case *ast.MappingKeyNode:
		return l.node(n.Value)
	case *ast.CommentGroupNode, *ast.CommentNode:
		return nil
	case ast.ScalarNode:
		if rbYAMLQuoted(n.GetToken()) {
			return String(n.GetToken().Value)
		}
		return l.scalar(n.GetToken().Value)
	}
	panic(NewNotImplementedError(Ref(String("YAML node " + n.Type().String() + " is not supported"))))
}

func rbYAMLQuoted(tk *token.Token) bool {
	return tk != nil && (tk.Type == token.SingleQuoteType || tk.Type == token.DoubleQuoteType)
}

// rbYAMLText is a scalar node's text, unresolved.
func rbYAMLText(n ast.Node) string {
	switch n := n.(type) {
	case *ast.StringNode:
		return n.Value
	case *ast.LiteralNode:
		return n.Value.Value
	case nil:
		return ""
	}
	return n.GetToken().Value
}

// tagged follows ToRuby's deserialize for the tags core values carry; any other ruby/ tag names a class, which a restricted load refuses and unsafe_load cannot build.
func (l *rbYAMLLoader) tagged(n *ast.TagNode) any {
	tag := n.Start.Value
	switch tag {
	case "!!str", "!str", "!ruby/string", "tag:yaml.org,2002:str":
		return String(rbYAMLText(n.Value))
	case "!!float", "tag:yaml.org,2002:float":
		s := rbYAMLText(n.Value)
		if v, ok := l.scalar(s).(Float); ok {
			return v
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
		if err != nil {
			panic(NewArgumentError(Ref(String("invalid value for Float(): " + strconv.Quote(s)))))
		}
		return Float(f)
	case "!!binary", "!binary", "tag:yaml.org,2002:binary":
		b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(rbYAMLText(n.Value)), ""))
		if err != nil {
			panic(NewArgumentError(Ref(String("invalid base64"))))
		}
		return String(b)
	case "!ruby/symbol", "!ruby/sym":
		return l.symbol(rbYAMLText(n.Value))
	case "!!map", "!!seq", "!ruby/hash", "!ruby/array", "tag:yaml.org,2002:map", "tag:yaml.org,2002:seq":
		return l.node(n.Value)
	case "!ruby/regexp": // `!ruby/regexp /\Afoo/i` (rubocop's default.yml), as Psych's ToRuby: the text between the slashes, then the option letters
		l.permit("Regexp")
		text := rbYAMLText(n.Value)
		end := strings.LastIndexByte(text, '/')
		if !strings.HasPrefix(text, "/") || end < 1 {
			return rbRegexpFromValue(String(text), nil)
		}
		return rbRegexpFromValue(String(text[1:end]), String(strings.Map(func(r rune) rune {
			if strings.ContainsRune("imx", r) {
				return r
			}
			return -1
		}, text[end+1:])))
	case "!!set":
		l.permit("Psych::Set")
		return l.node(n.Value)
	}
	if name, ok := strings.CutPrefix(tag, "!ruby/"); ok {
		if _, cls, ok := strings.Cut(name, ":"); ok {
			name = cls
		} else if name != "" {
			name = strings.ToUpper(name[:1]) + name[1:] // !ruby/regexp, !ruby/range: the class Psych's loader asks for
		}
		l.permit(name)
		panic(NewNotImplementedError(Ref(String("YAML tag " + tag + " is not supported (no runtime reflection)"))))
	}
	switch n.Value.(type) {
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode, *ast.AnchorNode, *ast.AliasNode, *ast.TagNode:
		return l.node(n.Value)
	}
	return l.scalar(rbYAMLText(n.Value)) // !!int, !!null, !!bool, a local tag: Psych tokenizes the text, quoted or not
}

// permit is ClassLoader::Restricted#find.
func (l *rbYAMLLoader) permit(class string) {
	if l.classes != nil && !l.classes[class] {
		panic(NewPsych_DisallowedClass(String("load"), String(class)))
	}
}

func (l *rbYAMLLoader) symbol(name string) Symbol {
	l.permit("Symbol")
	if len(l.symbols) > 0 && !l.symbols[name] {
		panic(NewPsych_DisallowedClass(String("load"), String("Symbol")))
	}
	return Symbol(name)
}

// mapping is ToRuby#revive_hash: later keys win, and a `<<` key merges a Hash (or each Hash of a list, last first) in place;
// anything else under `<<` (a scalar, a list holding a non-Hash) is kept as a plain "<<" pair, as Psych's rescue of TypeError.
func (l *rbYAMLLoader) mapping(pairs []*ast.MappingValueNode) any {
	keys := make([]any, 0, len(pairs))
	vals := make([]any, 0, len(pairs))
	for _, p := range pairs {
		if rbYAMLMergeKey(p.Key) {
			v := l.node(p.Value)
			if merged, ok := rbYAMLMerges(v); ok {
				for _, m := range merged {
					m.rbYAMLEach(func(k, v any) { keys, vals = append(keys, k), append(vals, v) })
				}
			} else {
				keys, vals = append(keys, String("<<")), append(vals, v)
			}
			continue
		}
		k := l.node(p.Key)
		if s, ok := k.(String); ok && l.symbolizeNames {
			k = Symbol(s)
		}
		keys, vals = append(keys, k), append(vals, l.node(p.Value))
	}
	return rbYAMLHash(keys, vals)
}

// rbYAMLHash picks the key type JSON.parse would (decision 25): String or Symbol keys when every key is one, else any.
func rbYAMLHash(keys, vals []any) any {
	allStr, allSym := true, true
	for _, k := range keys {
		_, s := k.(String)
		_, y := k.(Symbol)
		allStr, allSym = allStr && s, allSym && y
	}
	switch {
	case allStr:
		h := NewHash[String, any]()
		for i, k := range keys {
			Hash_Op_idxSet(h, k.(String), vals[i])
		}
		return h
	case allSym:
		h := NewHash[Symbol, any]()
		for i, k := range keys {
			Hash_Op_idxSet(h, k.(Symbol), vals[i])
		}
		return h
	}
	h := NewHash[any, any]()
	for i, k := range keys {
		Hash_Op_idxSet(h, k, vals[i])
	}
	return h
}

// rbYAMLMergeKey: Psych merges on a `<<` key unless it is tagged !!str, so a quoted '<<' merges too.
func rbYAMLMergeKey(k ast.Node) bool {
	switch k := k.(type) {
	case *ast.MergeKeyNode:
		return true
	case *ast.StringNode:
		return k.Value == "<<"
	}
	return false
}

// rbYAMLMerges is what a `<<` value merges, last first: one Hash, or a list of only Hashes.
func rbYAMLMerges(v any) ([]rbYAMLCore, bool) {
	if c, ok := v.(rbYAMLCore); ok && c.rbYAMLIsHash() {
		return []rbYAMLCore{c}, true
	}
	a, ok := v.(*Array[any])
	if !ok {
		return nil, false
	}
	out := make([]rbYAMLCore, 0, len(a.s))
	for i := len(a.s) - 1; i >= 0; i-- {
		c, ok := a.s[i].(rbYAMLCore)
		if !ok || !c.rbYAMLIsHash() {
			return nil, false
		}
		out = append(out, c)
	}
	return out, true
}

func rbYAMLFreeze(v any) {
	if c, ok := v.(rbYAMLCore); ok {
		c.rbYAMLEach(func(_, e any) { rbYAMLFreeze(e) })
	}
	if f, ok := v.(interface{ rbSetFrozen() }); ok {
		f.rbSetFrozen()
	}
}

// Psych::ScalarScanner's patterns; Ruby's ^/$ are line anchors, but a scalar with a newline never reaches them.
var (
	rbYAMLStrRx    = regexp.MustCompile(`^[^\d.:-]?[\p{L}_\s!@#$%^&*(){}<>|/\\~;=]+`)
	rbYAMLTimeRx   = regexp.MustCompile(`^-?\d{4}-\d{1,2}-\d{1,2}(?:[Tt]|\s+)\d{1,2}:\d\d:\d\d(?:\.\d*)?(?:\s*(?:Z|[-+]\d{1,2}:?(?:\d\d)?))?$`)
	rbYAMLDateRx   = regexp.MustCompile(`^\d{4}-(?:1[012]|0\d|\d)-(?:[12]\d|3[01]|0\d|\d)$`)
	rbYAMLSexInt   = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}$`)
	rbYAMLSexFloat = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9]){1,2}\.[0-9_]*$`)
	rbYAMLFloatRx  = regexp.MustCompile(`^[-+]?([0-9][0-9_,]*)?\.[0-9]*([eE][-+][0-9]+)?$`)
	rbYAMLIntRx    = regexp.MustCompile(`^(?:[-+]?0b[_,]*[0-1][0-1_,]*|[-+]?0[_,]*[0-7][0-7_,]*|[-+]?(?:0|[1-9](?:[0-9]|,[0-9]|_[0-9])*)|[-+]?0x[_,]*[0-9a-fA-F][0-9a-fA-F_,]*)$`)
	rbYAMLIntStrRx = regexp.MustCompile(`^(?:[-+]?0b[_]*[0-1][0-1_]*|[-+]?0[_]*[0-7][0-7_]*|[-+]?(0|[1-9][0-9_]*)|[-+]?0x[_]*[0-9a-fA-F][0-9a-fA-F_]*)$`)
	rbYAMLTimeTail = regexp.MustCompile(`(\d+):(\d+):(\d+)(?:\.(\d*))?\s*(Z|[-+]\d+(:\d\d)?)?`)
)

// scalar is ScalarScanner#tokenize: a plain scalar's Ruby value.
func (l *rbYAMLLoader) scalar(s string) any {
	switch {
	case s == "":
		return nil
	case rbYAMLStrRx.MatchString(s) || strings.Contains(s, "\n"):
		if utf8.RuneCountInString(s) > 5 {
			return String(s)
		}
		switch strings.ToLower(s) {
		case "~", "null":
			return nil
		case "yes", "true", "on":
			return Boolean(true)
		case "no", "false", "off":
			return Boolean(false)
		}
		return String(s)
	case rbYAMLTimeRx.MatchString(s):
		l.permit("Time") // before the value is checked, as Psych asks its class loader first
		if t, ok := l.time(s); ok {
			return t
		}
		return String(s)
	case rbYAMLDateRx.MatchString(s):
		l.permit("Date")
		y, _ := strconv.Atoi(s[:4])
		md := strings.SplitN(s[5:], "-", 2)
		m, _ := strconv.Atoi(md[0])
		d, _ := strconv.Atoi(md[1])
		jd, ok := rbCivilJD(y, m, d)
		if !ok {
			return String(s)
		}
		return rbNewDate(rbDateParts{jd: jd})
	case strings.EqualFold(s, ".inf") || strings.EqualFold(s, "+.inf"):
		return Float(math.Inf(1))
	case strings.EqualFold(s, "-.inf"):
		return Float(math.Inf(-1))
	case strings.EqualFold(s, ".nan"):
		return Float(math.NaN())
	case l.parseSymbols && len(s) > 1 && s[0] == ':':
		name := s[1:]
		if len(name) >= 2 && (name[0] == '"' || name[0] == '\'') {
			if end := strings.LastIndexByte(name, name[0]); end > 0 {
				name = strings.TrimPrefix(name[1:end], ":")
			}
		}
		return l.symbol(name)
	case rbYAMLSexInt.MatchString(s):
		n := 0
		for i, part := range strings.Split(s, ":") {
			v, _ := strconv.Atoi(strings.ReplaceAll(part, "_", ""))
			n += v * rbYAMLPow60(i)
		}
		return Integer(n)
	case rbYAMLSexFloat.MatchString(s):
		f := 0.0
		for i, part := range strings.Split(s, ":") {
			v, _ := strconv.ParseFloat(strings.ReplaceAll(part, "_", ""), 64)
			f += v * float64(rbYAMLPow60(i))
		}
		return Float(f)
	case rbYAMLFloatRx.MatchString(s):
		if s == "." || s == "+." || s == "-." {
			return String(s)
		}
		t := strings.NewReplacer(",", "", "_", "").Replace(s)
		t = strings.Replace(strings.Replace(t, ".e", "e", 1), ".E", "E", 1)
		t = strings.TrimSuffix(t, ".")
		f, _ := strconv.ParseFloat(t, 64) // ErrRange saturates to +-Inf, as MRI
		return Float(f)
	case !l.strictInt && rbYAMLIntRx.MatchString(s), l.strictInt && rbYAMLIntStrRx.MatchString(s):
		return l.scalarInt(s)
	}
	return String(s)
}

// rbYAMLPow60 is ScalarScanner's 60 ** (e - 2).abs, which weighs a two-part value as hours:minutes.
func rbYAMLPow60(e int) int {
	p := e - 2
	if p < 0 {
		p = -p
	}
	n := 1
	for range p {
		n *= 60
	}
	return n
}

// scalarInt is ScalarScanner#parse_int: Integer(s.delete(",_")), 0-prefixed octal included.
func (l *rbYAMLLoader) scalarInt(s string) Integer {
	t := strings.NewReplacer(",", "", "_", "").Replace(s)
	digits := strings.TrimLeft(t, "+-")
	if len(digits) > 1 && digits[0] == '0' && digits[1] >= '0' && digits[1] <= '9' {
		t = t[:len(t)-len(digits)] + "0o" + digits[1:]
	}
	n, err := strconv.ParseInt(t, 0, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			panic(NewRangeError(Ref(String(s + " overflows Integer (64-bit; no Bignum)"))))
		}
		panic(NewArgumentError(Ref(String("invalid value for Integer(): " + strconv.Quote(s)))))
	}
	return Integer(n)
}

// time is ScalarScanner#parse_time: UTC for Z, local time without a zone, else the given offset.
func (l *rbYAMLLoader) time(s string) (*Time, bool) {
	date, rest, _ := strings.Cut(strings.Replace(strings.ToLower(s), "t", " ", 1), " ")
	neg := strings.HasPrefix(date, "-")
	ymd := strings.Split(strings.TrimPrefix(date, "-"), "-")
	y, _ := strconv.Atoi(ymd[0])
	if neg {
		y = -y
	}
	mo, _ := strconv.Atoi(ymd[1])
	d, _ := strconv.Atoi(ymd[2])
	m := rbYAMLTimeTail.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(rest)))
	if m == nil {
		return nil, false
	}
	hh, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	ss, _ := strconv.Atoi(m[3])
	ns := 0
	if m[4] != "" {
		frac := (m[4] + "000000000")[:9]
		ns, _ = strconv.Atoi(frac)
	}
	if _, ok := rbCivilJD(y, mo, d); !ok || hh > 24 || mi > 59 || ss > 60 {
		return nil, false
	}
	switch tz := m[5]; tz {
	case "Z":
		return &Time{t: time.Date(y, time.Month(mo), d, hh, mi, ss, ns, time.UTC), utc: true}, true
	case "":
		return &Time{t: time.Date(y, time.Month(mo), d, hh, mi, ss, ns, time.UTC).In(time.Local)}, true
	default:
		h, mm, _ := strings.Cut(strings.TrimLeft(tz, "+-"), ":")
		if mm == "" && len(h) > 2 {
			h, mm = h[:len(h)-2], h[len(h)-2:]
		}
		oh, _ := strconv.Atoi(h)
		om, _ := strconv.Atoi(mm)
		off := oh*3600 + om*60
		if tz[0] == '-' {
			off = -off
		}
		return &Time{t: time.Date(y, time.Month(mo), d, hh, mi, ss, ns, time.FixedZone("", off))}, true
	}
}

// rbYAMLDump is Psych.dump. go-yaml's encoder takes only string map keys and quotes by YAML 1.2, so this small emitter writes block style
// and quotes whatever ScalarScanner would read back as another value (decision 376).
func rbYAMLDump(o any) String {
	var b strings.Builder
	v := rbYAMLUnbox(o)
	if rbYAMLBlock(v) {
		b.WriteString("---\n")
		rbYAMLEmit(&b, v, 0, false)
	} else {
		b.WriteString("---")
		t := rbYAMLScalar(v, 2, false) // a literal block's lines sit at 2, as libyaml's, so a "---" line in the text stays text
		if strings.HasPrefix(t, "|2") {
			t = strconv.Quote(string(v.(String))) // go-yaml reads a top-level |2 as indent 1, libyaml as 2: both read a quoted one alike
		}
		if t != "" {
			b.WriteString(" " + t)
		}
		b.WriteString("\n")
	}
	return String(b.String())
}

// rbYAMLBlock: a non-empty Array or Hash, written one entry per line.
func rbYAMLBlock(v any) bool {
	n := -1
	switch x := v.(type) {
	case rbYAMLCore:
		n = x.rbYAMLLen()
	}
	return n > 0
}

// rbYAMLCore walks a core collection whatever its element types.
type rbYAMLCore interface {
	rbYAMLLen() int
	rbYAMLEach(f func(k, v any)) // k is nil for an Array
	rbYAMLIsHash() bool
}

func (self *Array[E]) rbYAMLLen() int     { return len(self.s) }
func (self *Array[E]) rbYAMLIsHash() bool { return false }
func (self *Array[E]) rbYAMLEach(f func(k, v any)) {
	for _, e := range self.s {
		f(nil, e)
	}
}

func (self *Hash[K, V]) rbYAMLLen() int     { return int(self.live) }
func (self *Hash[K, V]) rbYAMLIsHash() bool { return true }
func (self *Hash[K, V]) rbYAMLEach(f func(k, v any)) {
	for k, v := range rbHashPairs(self) {
		f(k, v)
	}
}

// rbYAMLEmit writes block collection c at indent; inline: the first line continues a "- " already written.
func rbYAMLEmit(b *strings.Builder, c any, indent int, inline bool) {
	col := c.(rbYAMLCore)
	pad := strings.Repeat(" ", indent)
	first := true
	col.rbYAMLEach(func(k, v any) {
		if !first || !inline {
			b.WriteString(pad)
		}
		first = false
		v = rbYAMLUnbox(v)
		if col.rbYAMLIsHash() {
			b.WriteString(rbYAMLKey(rbYAMLUnbox(k)) + ":")
			switch {
			case rbYAMLBlock(v) && v.(rbYAMLCore).rbYAMLIsHash():
				b.WriteString("\n")
				rbYAMLEmit(b, v, indent+2, false)
			case rbYAMLBlock(v):
				b.WriteString("\n")
				rbYAMLEmit(b, v, indent, false) // Psych's sequences under a key are not indented
			default:
				if t := rbYAMLScalar(v, indent+2, false); t != "" {
					b.WriteString(" " + t)
				}
				b.WriteString("\n")
			}
			return
		}
		b.WriteString("-")
		if rbYAMLBlock(v) {
			b.WriteString(" ")
			rbYAMLEmit(b, v, indent+2, true)
			return
		}
		if t := rbYAMLScalar(v, indent+2, false); t != "" {
			b.WriteString(" " + t)
		}
		b.WriteString("\n")
	})
}

// rbYAMLKey is a mapping key: a scalar, or a collection in flow style.
func rbYAMLKey(k any) string {
	if k == nil {
		return "~"
	}
	if c, ok := k.(rbYAMLCore); ok {
		return rbYAMLFlow(c)
	}
	return rbYAMLScalar(k, 0, true)
}

func rbYAMLFlow(c rbYAMLCore) string {
	var parts []string
	c.rbYAMLEach(func(k, v any) {
		v = rbYAMLUnbox(v)
		t := rbYAMLScalar(v, 0, true) // a collection is its flow form
		if v == nil {
			t = "~"
		}
		if c.rbYAMLIsHash() {
			t = rbYAMLKey(rbYAMLUnbox(k)) + ": " + t
		}
		parts = append(parts, t)
	})
	if c.rbYAMLIsHash() {
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// rbYAMLScalar is v's text on one line ("" for nil), or a literal block whose lines sit at indent.
func rbYAMLScalar(v any, indent int, flow bool) string {
	switch x := v.(type) {
	case nil:
		return ""
	case rbYAMLCore:
		return rbYAMLFlow(x) // empty: [] or {}
	case String:
		return rbYAMLString(string(x), indent, flow)
	case Symbol:
		if t := ":" + string(x); rbYAMLPlain(t, flow) {
			if sym, ok := (&rbYAMLLoader{parseSymbols: true}).scalar(t).(Symbol); ok && sym == x {
				return t
			}
		}
		return "!ruby/symbol " + rbYAMLQuote(string(x))
	case Integer:
		return strconv.FormatInt(int64(x), 10)
	case Float:
		f := float64(x)
		switch {
		case math.IsInf(f, 1):
			return ".inf"
		case math.IsInf(f, -1):
			return "-.inf"
		case math.IsNaN(f):
			return ".nan"
		}
		return string(rbFloatToS(f))
	case Boolean:
		return strconv.FormatBool(bool(x))
	case *Date:
		return string(x.ToS())
	case *Time:
		if x.utc {
			return x.t.Format("2006-01-02 15:04:05.000000000 Z")
		}
		return x.t.Format("2006-01-02 15:04:05.000000000 -07:00")
	}
	panic(NewNotImplementedError(Ref(String("YAML.dump of " + rbClassName(v) + " is not supported (no runtime reflection)"))))
}

func rbYAMLString(s string, indent int, flow bool) string {
	if !utf8.ValidString(s) { // bytes, not text: a "\xff" escape would read back as U+00FF
		return "!binary " + base64.StdEncoding.EncodeToString([]byte(s))
	}
	if strings.ContainsFunc(s, func(r rune) bool { return r != '\n' && r != '\t' && (r < ' ' || r == 0x7f || r == utf8.RuneError) }) {
		return strconv.Quote(s)
	}
	if strings.Contains(s, "\n") {
		if flow || strings.Trim(s, "\n") == "" { // a literal block of only line breaks reads back as ""
			return strconv.Quote(s)
		}
		head := "|"
		if strings.HasPrefix(s, " ") {
			head += "2"
		}
		body := strings.TrimSuffix(s, "\n")
		switch {
		case !strings.HasSuffix(s, "\n"):
			head += "-"
		case strings.HasSuffix(body, "\n"):
			head += "+"
		}
		pad := strings.Repeat(" ", indent)
		var b strings.Builder
		b.WriteString(head)
		for line := range strings.SplitSeq(body, "\n") {
			b.WriteString("\n")
			if line != "" {
				b.WriteString(pad + line)
			}
		}
		return b.String()
	}
	if rbYAMLPlain(s, flow) {
		if r, ok := (&rbYAMLLoader{parseSymbols: true}).scalar(s).(String); ok && string(r) == s {
			return s
		}
	}
	return rbYAMLQuote(s)
}

func rbYAMLQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// rbYAMLPlain: s parses back as one plain scalar of the same text.
func rbYAMLPlain(s string, flow bool) bool {
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s[:1], "-?,[]{}#&*!|>'\"%@`") && !(len(s) > 1 && s[0] == '-' && s[1] != ' ') {
		return false
	}
	if strings.Contains(s, ": ") || strings.Contains(s, " #") || strings.HasSuffix(s, ":") || strings.Contains(s, "\t") {
		return false
	}
	return !flow || !strings.ContainsAny(s, ",[]{}")
}

// rbYAMLUnbox is rbUnbox, with a tuple (a literal Array of mixed types) as the Array it stands for.
func rbYAMLUnbox(v any) any {
	v = rbUnbox(v)
	if t, ok := v.(interface{ _Tuple() *Array[any] }); ok {
		return t._Tuple()
	}
	return v
}
