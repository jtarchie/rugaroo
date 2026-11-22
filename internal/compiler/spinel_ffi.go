package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// Spinel's ffi_* forms become the ffi gem's spelling (decision 375 compiles it) on their own line, so positions hold (decision 375).

// spinelFFITypes are Spinel's type names the ffi gem spells otherwise.
var spinelFFITypes = map[string]string{"str": "string", "binstr": "string", "ptr": "pointer"}

// spinelFFIWidths are ffi_read_<w>/ffi_write_<w>'s widths as the gem's get_/put_ type names.
var spinelFFIWidths = map[string]string{
	"u8": "uint8", "i8": "int8", "u16": "uint16", "i16": "int16", "u32": "uint32", "i32": "int32", "u64": "uint64", "i64": "int64", "ptr": "pointer",
}

type spinelEdit struct {
	at, n int
	text  string
}

// spinelFFI rewrites the Spinel FFI forms in root's class and module bodies; ok is false when there are none.
func spinelFFI(name string, root parser.Node, src []byte) (out []byte, ok bool, err error) {
	var edits []spinelEdit
	walkNode(root, func(n parser.Node) (bool, bool) {
		var body parser.Node
		switch n := n.(type) {
		case *parser.ModuleNode:
			body = n.Body
		case *parser.ClassNode:
			body = n.Body
		default:
			return false, true
		}
		st, isStmts := body.(*parser.StatementsNode)
		if !isStmts || err != nil {
			return false, true
		}
		var es []spinelEdit
		es, err = spinelBody(name, st.Body, src)
		edits = append(edits, es...)
		return false, true
	})
	if err != nil || len(edits) == 0 {
		return nil, false, err
	}
	slices.SortFunc(edits, func(a, b spinelEdit) int { return b.at - a.at })
	out = slices.Clone(src)
	for _, e := range edits {
		out = slices.Concat(out[:e.at], []byte(e.text), out[e.at+e.n:])
	}
	return out, true, nil
}

// spinelBody is one body's edits; a body with ffi_func but no ffi_lib binds libc, as Spinel links it by default.
func spinelBody(name string, stmts []parser.Node, src []byte) ([]spinelEdit, error) {
	spinel := slices.ContainsFunc(stmts, func(s parser.Node) bool { // ffi_lib alone is the gem's own spelling too
		call, isCall := s.(*parser.CallNode)
		return isCall && call.Receiver == nil && strings.HasPrefix(call.Name, "ffi_") && call.Name != "ffi_lib"
	})
	if !spinel {
		return nil, nil
	}
	var edits []spinelEdit
	hasLib := false
	first := -1
	for _, s := range stmts {
		call, isCall := s.(*parser.CallNode)
		if !isCall || call.Receiver != nil || !strings.HasPrefix(call.Name, "ffi_") || call.Block != nil {
			continue
		}
		text, err := spinelForm(name, call, src)
		if err != nil {
			return nil, err
		}
		if text == "" {
			continue // not a Spinel form: a method the program defines itself
		}
		hasLib = hasLib || call.Name == "ffi_lib"
		if first < 0 {
			first = len(edits)
		}
		edits = append(edits, spinelEdit{at: call.Location.StartOffset, n: call.Location.Length, text: text})
	}
	if first >= 0 && !hasLib {
		edits[first].text = `extend FFI::Library; ffi_lib "c"; ` + edits[first].text
	}
	return edits, nil
}

// spinelForm is a form's replacement, "" for a name that is no Spinel form.
func spinelForm(file string, call *parser.CallNode, src []byte) (string, error) {
	var args []parser.Node
	if call.Arguments != nil {
		args = call.Arguments.Arguments
	}
	text := func(n parser.Node) string {
		loc := n.GetLocation()
		return string(src[loc.StartOffset : loc.StartOffset+loc.Length])
	}
	sym := func(i int) (string, error) {
		if i < len(args) {
			if s, ok := args[i].(*parser.SymbolNode); ok {
				return s.Unescaped.Value, nil
			}
		}
		return "", fmt.Errorf("%s: %s needs a literal Symbol as argument %d (Spinel FFI, decision 375)", file, call.Name, i+1)
	}
	typ := func(t string) string {
		if g, ok := spinelFFITypes[t]; ok {
			return g
		}
		return t
	}
	signature := func() (string, error) { // `:name, [types], :ret` plus any keywords (blocking: is the gem's own)
		name, err := sym(0)
		if err != nil {
			return "", err
		}
		ret, err := sym(2)
		if err != nil {
			return "", err
		}
		arr, ok := args[1].(*parser.ArrayNode)
		if !ok {
			return "", fmt.Errorf("%s: %s needs a literal Array of argument types (Spinel FFI, decision 375)", file, call.Name)
		}
		types := make([]string, 0, len(arr.Elements))
		for _, el := range arr.Elements {
			s, isSym := el.(*parser.SymbolNode)
			if !isSym {
				return "", fmt.Errorf("%s: %s needs literal Symbol argument types (Spinel FFI, decision 375)", file, call.Name)
			}
			types = append(types, ":"+typ(s.Unescaped.Value))
		}
		parts := []string{":" + name, "[" + strings.Join(types, ", ") + "]", ":" + typ(ret)}
		for _, a := range args[3:] {
			parts = append(parts, text(a))
		}
		return strings.Join(parts, ", "), nil
	}
	read := strings.HasPrefix(call.Name, "ffi_read_")
	switch {
	case call.Name == "ffi_lib":
		return "extend FFI::Library; " + text(call), nil
	case call.Name == "ffi_cflags":
		return "nil", nil // a C compiler flag: nothing to link here
	case call.Name == "ffi_func" && len(args) >= 3:
		sig, err := signature()
		return "attach_function " + sig, err
	case call.Name == "ffi_callback" && len(args) >= 3:
		sig, err := signature()
		return "callback " + sig, err
	case call.Name == "ffi_const" && len(args) == 2:
		name, err := sym(0)
		return name + " = " + text(args[1]), err
	case call.Name == "ffi_buffer" && len(args) == 2:
		name, err := sym(0)
		c := "FFI_BUFFER_" + strings.ToUpper(name)
		return fmt.Sprintf("%s = ::FFI::MemoryPointer.new(%s); def self.%s = %s", c, text(args[1]), name, c), err
	case (read || strings.HasPrefix(call.Name, "ffi_write_")) && len(args) == 2:
		w := strings.TrimPrefix(strings.TrimPrefix(call.Name, "ffi_read_"), "ffi_write_")
		t, ok := spinelFFIWidths[w]
		if !ok {
			return "", fmt.Errorf("%s: %s: no such width (Spinel FFI's are u8/i8/u16/i16/u32/i32/u64/i64/ptr)", file, call.Name)
		}
		name, err := sym(0)
		if read {
			return fmt.Sprintf("def self.%s(buf) = buf.get_%s(%s)", name, t, text(args[1])), err
		}
		return fmt.Sprintf("def self.%s(buf, v) = (buf.put_%s(%s, v); v)", name, t, text(args[1])), err
	case call.Name == "ffi_struct" || call.Name == "ffi_source":
		return "", fmt.Errorf("%s: Spinel's %s is not supported (decision 375)", file, call.Name)
	}
	return "", nil
}
