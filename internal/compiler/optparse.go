package compiler

import (
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// optKind types OptionParser#on at compile time (decision 101): the
// literal switch strings say whether the switch takes an argument
// (`--name NAME`, `-n [N]`), the coercion class what it becomes. The
// result names the prelude variant whose block has that parameter type;
// "" (a non-literal switch string) keeps the untyped `on`.
func (f *fctx) optKind(args []parser.Node) string {
	style, conv := 0, "string"
	for _, a := range args {
		switch a := a.(type) {
		case *parser.StringNode:
			s := a.Unescaped.Value
			if !strings.HasPrefix(s, "-") || len(s) < 2 {
				continue
			}
			var rest string
			if body, ok := strings.CutPrefix(s, "--"); ok {
				body = strings.TrimPrefix(body, "[no-]")
				if i := strings.IndexAny(body, "= ["); i >= 0 {
					rest = body[i:]
				}
			} else {
				rest = s[2:]
			}
			switch t := strings.TrimSpace(rest); {
			case t == "":
			case strings.Contains(t, "["):
				style = max(style, 2)
			default:
				style = max(style, 1)
			}
		case *parser.ConstantPathNode: // OptionParser::REQUIRED_ARGUMENT / OPTIONAL_ARGUMENT / NO_ARGUMENT (decision 245)
			if a.Name != nil {
				switch *a.Name {
				case "REQUIRED_ARGUMENT":
					style = max(style, 1)
				case "OPTIONAL_ARGUMENT":
					style = max(style, 2)
				}
			}
		case *parser.ConstantReadNode:
			switch a.Name {
			case "Integer", "Float", "Array", "String":
				conv = strings.ToLower(a.Name)
			default:
				f.errorf(a, "OptionParser#on: coercion to %s is not supported (Integer, Float, Array or String)", a.Name)
			}
		case *parser.ArrayNode, *parser.HashNode, *parser.KeywordHashNode, *parser.RegularExpressionNode:
			f.errorf(a, "OptionParser#on: completion lists and patterns are not supported")
		case *parser.InterpolatedStringNode, *parser.LocalVariableReadNode, *parser.CallNode, *parser.InstanceVariableReadNode, *parser.SplatNode: // `on(*args)` (rubocop's option helper): the switch strings are not here to read
			return "" // maybe a switch: decided at run time
		}
	}
	switch style {
	case 0:
		return "flag"
	case 1:
		return conv
	}
	if conv == "array" {
		f.errorf(args[0], "OptionParser#on: an optional Array argument is not supported")
	}
	return conv + "_opt"
}
