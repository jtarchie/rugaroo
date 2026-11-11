package compiler

import (
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// rugarooEncodings mirrors prelude/go/encoding.go's rbEncAliases: the names rugaroo can transcode (decision 136).
var rugarooEncodings = map[string]bool{
	"ASCII-8BIT": true, "BINARY": true, "UTF-8": true, "CP65001": true, "LOCALE": true, "EXTERNAL": true, "FILESYSTEM": true,
	"US-ASCII": true, "ASCII": true, "ANSI_X3.4-1968": true, "646": true, "UTF-16BE": true, "UCS-2BE": true, "UTF-16LE": true,
	"UTF-32BE": true, "UCS-4BE": true, "UTF-32LE": true, "UCS-4LE": true, "UTF-16": true, "UTF-32": true,
	"ISO-8859-1": true, "ISO8859-1": true, "INTERNAL": true,
}

// mriEncodings is MRI 4.0's Encoding.name_list, upper-cased: a literal naming one rugaroo lacks is a compile error, while a name MRI lacks too stays MRI's run-time error.
var mriEncodings = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`646 ANSI_X3.4-1968 ASCII ASCII-8BIT BIG5 BIG5-HKSCS BIG5-HKSCS:2008 BIG5-UAO
		BINARY CESU-8 CP1250 CP1251 CP1252 CP1253 CP1254 CP1255 CP1256 CP1257 CP1258 CP437 CP50220 CP50221 CP51932 CP65000
		CP65001 CP720 CP737 CP775 CP850 CP852 CP855 CP857 CP860 CP861 CP862 CP863 CP864 CP865 CP866 CP869
		CP874 CP878 CP932 CP936 CP949 CP950 CP951 CSWINDOWS31J EBCDIC-CP-US EMACS-MULE EUC-CN EUC-JIS-2004 EUC-JISX0213
		EUC-JP EUC-JP-MS EUC-KR EUC-TW EUCCN EUCJP EUCJP-MS EUCKR EUCTW EXTERNAL FILESYSTEM GB12345 GB18030 GB1988 GB2312
		GBK IBM037 IBM437 IBM720 IBM737 IBM775 IBM850 IBM852 IBM855 IBM857 IBM860 IBM861 IBM862 IBM863 IBM864 IBM865
		IBM866 IBM869 INTERNAL ISO-2022-JP ISO-2022-JP-2 ISO-2022-JP-KDDI ISO-8859-1 ISO-8859-10 ISO-8859-11 ISO-8859-13
		ISO-8859-14 ISO-8859-15 ISO-8859-16 ISO-8859-2 ISO-8859-3 ISO-8859-4 ISO-8859-5 ISO-8859-6 ISO-8859-7 ISO-8859-8
		ISO-8859-9 ISO2022-JP ISO2022-JP2 ISO8859-1 ISO8859-10 ISO8859-11 ISO8859-13 ISO8859-14 ISO8859-15 ISO8859-16
		ISO8859-2 ISO8859-3 ISO8859-4 ISO8859-5 ISO8859-6 ISO8859-7 ISO8859-8 ISO8859-9 KOI8-R KOI8-U LOCALE MACCENTEURO
		MACCROATIAN MACCYRILLIC MACGREEK MACICELAND MACJAPAN MACJAPANESE MACROMAN MACROMANIA MACTHAI MACTURKISH MACUKRAINE
		PCK SHIFT_JIS SJIS SJIS-DOCOMO SJIS-KDDI SJIS-SOFTBANK STATELESS-ISO-2022-JP STATELESS-ISO-2022-JP-KDDI TIS-620
		UCS-2BE UCS-4BE UCS-4LE US-ASCII UTF-16 UTF-16BE UTF-16LE UTF-32 UTF-32BE UTF-32LE UTF-7 UTF-8 UTF-8-HFS UTF-8-MAC
		UTF8-DOCOMO UTF8-KDDI UTF8-MAC UTF8-SOFTBANK WINDOWS-1250 WINDOWS-1251 WINDOWS-1252 WINDOWS-1253 WINDOWS-1254
		WINDOWS-1255 WINDOWS-1256 WINDOWS-1257 WINDOWS-1258 WINDOWS-31J WINDOWS-874`) {
		m[n] = true
	}
	return m
}()

// encodingArgs are the arguments of a user call that name encodings, by receiver class and method; -1 is every positional argument.
var encodingArgs = map[string]map[string]int{
	"String":         {"encode": -1, "force_encoding": 0},
	"Integer":        {"chr": 0},
	"File":           {"set_encoding": -1},
	"IO":             {"set_encoding": -1},
	"Encoding.class": {"find": 0, "default_external=": 0, "default_internal=": 0},
	"File.class":     {"new": 1, "open": 1},
	"CSV.class":      {"open": 1},
}

// checkEncodingNames rejects at compile time a string literal naming an encoding MRI has and rugaroo does not (decision 136).
func (f *fctx) checkEncodingNames(n parser.Node, recv expr, name string, args []parser.Node) {
	if f.f.isPrelude() {
		return
	}
	f.linkEncodingKeywords(args)
	t, ok := recv.typ.(TClass)
	if !ok {
		return
	}
	key := t.C.RubyName
	if t.C.metaOf != nil {
		key = t.C.metaOf.RubyName + ".class"
	}
	idx, ok := encodingArgs[key][name]
	if !ok {
		return
	}
	if strings.HasSuffix(key, ".class") && idx == 1 && len(args) > 1 {
		if s, lit := args[1].(*parser.StringNode); !lit || strings.Contains(s.Unescaped.Value, ":") {
			f.emit("rbFileEncModes()") // the mode may name encodings: link the transcoder in (rbFileEncHook)
			if !lit {
				f.emit("rbUseEncAll()")
			}
		}
	}
	for i, a := range args {
		if idx >= 0 && i != idx {
			continue
		}
		if name == "force_encoding" {
			f.warnTextEncoding(a)
		}
		s, isStr := a.(*parser.StringNode)
		if !isStr {
			f.linkEncodingValue(a)
			continue
		}
		lit := s.Unescaped.Value
		names := []string{lit}
		switch {
		case key == "File.class" || key == "CSV.class":
			names = strings.Split(lit, ":")[1:] // the access mode comes first
		case name == "set_encoding":
			names = strings.Split(lit, ":")
		}
		for _, e := range names {
			u := strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(e, "BOM|"), "bom|"))
			fam, xtext := xtextEncodings[u]
			if fam != "" {
				f.emit("rbUseEnc%s()", fam) // link that family's x/text tables (decision 380)
			}
			if mriEncodings[u] && !rugarooEncodings[u] && !xtext {
				f.errorf(n, "encoding %s is not supported: rugaroo transcodes UTF-8, ASCII-8BIT, US-ASCII, UTF-16LE/BE, UTF-32LE/BE, UTF-16, UTF-32 and x/text's tables (ISO-8859-*, Windows-125*, KOI8-*, IBM/CP 437-866, macRoman, macCyrillic, Shift_JIS, Windows-31J, EUC-JP, CP51932, EUC-KR, CP949, GBK, GB2312, GB18030, Big5, CP950; decisions 136, 380)", e)
			}
		}
	}
}

// textTag are the encodings force_encoding can name without a warning, upper-cased without `-`/`_`: UTF-8 and binary,
// which a String is, and US-ASCII and the default-encoding names, which read as UTF-8 (decision 439).
var textTag = map[string]bool{
	"UTF8": true, "CP65001": true, "UTF8MAC": true, "UTF8HFS": true, "ASCII8BIT": true, "BINARY": true, "USASCII": true,
	"ASCII": true, "ANSIX3.41968": true, "646": true, "LOCALE": true, "EXTERNAL": true, "FILESYSTEM": true, "INTERNAL": true,
}

// mriEncodingKeys is mriEncodings spelled as textTag is, so an Encoding constant (ISO_8859_1, Shift_JIS) matches its name.
var mriEncodingKeys = func() map[string]bool {
	m := map[string]bool{}
	for n := range mriEncodings {
		m[encodingKey(n)] = true
	}
	return m
}()

func encodingKey(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", "_", "").Replace(name))
}

// warnTextEncoding warns at `s.force_encoding(lit)` whose literal names a text encoding but UTF-8: the String keeps its
// bytes and reads them as UTF-8, where MRI retags them (decision 439).
func (f *fctx) warnTextEncoding(a parser.Node) {
	var enc string
	switch a := a.(type) {
	case *parser.StringNode:
		enc = a.Unescaped.Value
		if _, xtext := xtextEncodings[strings.ToUpper(enc)]; !xtext && !rugarooEncodings[strings.ToUpper(enc)] {
			return // unsupported: checkEncodingNames' compile error says so
		}
	case *parser.ConstantPathNode:
		t := strings.TrimPrefix(f.f.text(a.Location), "::")
		if !strings.HasPrefix(t, "Encoding::") {
			return
		}
		enc = strings.TrimPrefix(t, "Encoding::")
	default:
		return
	}
	if k := encodingKey(enc); mriEncodingKeys[k] && !textTag[k] {
		src := f.f.text(a.GetLocation())
		f.warn(a, "force_encoding(%s) keeps the bytes and reads them as UTF-8: a String carries no encoding tag (decision 439); convert the text with encode(\"UTF-8\", %s)", src, src)
	}
}

// linkEncodingValue links the x/text tables (decision 380) an encoding argument that is not a string literal may need:
// an Encoding constant its own family's, anything else (a variable, a call) every family's.
func (f *fctx) linkEncodingValue(a parser.Node) {
	switch a := a.(type) {
	case *parser.KeywordHashNode, *parser.NilNode, *parser.SymbolNode:
		return
	case *parser.ConstantPathNode:
		if p, ok := a.Parent.(*parser.ConstantReadNode); ok && p.Name == "Encoding" && a.Name != nil {
			if fam := xtextEncodingConsts[*a.Name]; fam != "" {
				f.emit("rbUseEnc%s()", fam)
			}
			return
		}
	}
	f.emit("rbUseEncAll()")
}

// linkEncodingKeywords links the tables an encoding:, external_encoding: or internal_encoding: keyword names
// (`File.read(path, encoding: "Shift_JIS:UTF-8")`).
func (f *fctx) linkEncodingKeywords(args []parser.Node) {
	for _, a := range args {
		kw, ok := a.(*parser.KeywordHashNode)
		if !ok {
			continue
		}
		for _, el := range kw.Elements {
			assoc, ok := el.(*parser.AssocNode)
			if !ok {
				continue
			}
			key, ok := assoc.Key.(*parser.SymbolNode)
			if !ok || key.Unescaped.Value != "encoding" && key.Unescaped.Value != "external_encoding" && key.Unescaped.Value != "internal_encoding" {
				continue
			}
			s, lit := assoc.Value.(*parser.StringNode)
			if !lit {
				f.linkEncodingValue(assoc.Value)
				continue
			}
			for _, e := range strings.Split(s.Unescaped.Value, ":") {
				if fam := xtextEncodings[strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(e, "BOM|"), "bom|"))]; fam != "" {
					f.emit("rbUseEnc%s()", fam)
				}
			}
		}
	}
}
