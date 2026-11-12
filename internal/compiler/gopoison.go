package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A gem method whose generated Go does not type-check is poisoned as one rugaroo cannot compile is (decision 205): the
// traced build type-checks its output and replaces each body Go rejected with the NotImplementedError raise, in the
// Go text, keeping the signature so its callers still type-check (decision 303).

// methodKey names a method across compiles of the same sources.
func methodKey(m *Method) string {
	return fmt.Sprintf("%s:%d:%s", m.File.path, m.Line, m.Name)
}

// goFunc is a gem method's Go func: its methodKey and the name its NotImplementedError gives.
type goFunc struct{ key, name string }

// stubRejected type-checks code once and replaces the body of each func Go rejects that stub answers for (with its
// NotImplementedError message, true) by that raise. An import only stubbed bodies used is deleted: the signatures stay,
// so nothing else can fail, and no second check is needed (decision 303). Code that does not parse is returned as is:
// go build reports that, with the generated file kept.
func stubRejected(ctx context.Context, code []byte, stub func(fn, msg string) (string, bool)) []byte {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", code, parser.SkipObjectResolution)
	if err != nil {
		return code
	}
	var funcs []*ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			funcs = append(funcs, fd)
		}
	}
	at := func(pos token.Pos) *ast.FuncDecl { // the top-level func enclosing pos, or nil
		i := sort.Search(len(funcs), func(i int) bool { return funcs[i].End() > pos })
		if i < len(funcs) && funcs[i].Pos() <= pos {
			return funcs[i]
		}
		return nil
	}
	bad := map[*ast.FuncDecl]string{} // the first error in each
	var impossible []token.Pos        // a dispatcher's arm for a class that cannot be the value (see dropArms)
	conf := types.Config{Importer: stdImporter(ctx, fset, f), Error: func(err error) {
		var te types.Error
		if errors.As(err, &te) {
			if fd := at(te.Pos); impossibleArm(fd, te.Msg) {
				impossible = append(impossible, te.Pos)
				return
			}
			if fd := at(te.Pos); fd != nil && bad[fd] == "" {
				bad[fd] = te.Msg
			}
		}
	}}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	_, _ = conf.Check("main", fset, []*ast.File{f}, info)
	type cut struct {
		from, to int
		with     string
	}
	var cuts []cut
	stubbed := map[*ast.FuncDecl]bool{}
	for _, fd := range funcs {
		if msg, ok := bad[fd]; ok && fd.Body != nil {
			if text, ok := stub(funcDeclName(fd), msg); ok {
				stubbed[fd] = true
				cuts = append(cuts, cut{fset.Position(fd.Body.Lbrace).Offset + 1, fset.Position(fd.Body.Rbrace).Offset,
					fmt.Sprintf("\n\tpanic(NewNotImplementedError(Ref(String(%q))))\n", text)})
			}
		}
	}
	// dropArms: an rbSend_ type switch names each class under the owner that overrides the method. A class whose own
	// method of another name has a different shape than a mixin's cannot be the mixin's Self at all (rubocop's LineLength
	// #register_offense under Alignment's), so Go calls its arm impossible: the arm goes, the dispatcher stays (decision 444)
	for _, pos := range impossible {
		if from, to, ok := caseAt(at(pos), pos); ok {
			cuts = append(cuts, cut{fset.Position(from).Offset, fset.Position(to).Offset, ""})
		}
	}
	if len(cuts) == 0 {
		return code
	}
	kept := map[string]bool{} // import paths a use outside the stubbed bodies needs
	for id, obj := range info.Uses {
		if pn, ok := obj.(*types.PkgName); ok {
			if fd := at(id.Pos()); fd == nil || !stubbed[fd] || id.Pos() < fd.Body.Pos() {
				kept[pn.Imported().Path()] = true
			}
		}
	}
	for _, spec := range f.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); !kept[path] && (spec.Name == nil || spec.Name.Name != "_") {
			from := fset.Position(spec.Pos()).Offset
			from = bytes.LastIndexByte(code[:from], '\n') + 1
			to := from + bytes.IndexByte(code[from:], '\n') + 1
			cuts = append(cuts, cut{from, to, ""})
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].from > cuts[j].from })
	out := slices.Clone(code)
	for _, c := range cuts {
		out = slices.Replace(out, c.from, c.to, []byte(c.with)...)
	}
	return out
}

// impossibleArm reports Go's rejection of a dispatcher's type switch arm: a class that cannot be the switch's value.
func impossibleArm(fd *ast.FuncDecl, msg string) bool {
	return fd != nil && strings.HasPrefix(fd.Name.Name, "rbSend_") && strings.HasPrefix(msg, "impossible type switch case")
}

// caseAt is the extent of fd's single-type case clause holding pos.
func caseAt(fd *ast.FuncDecl, pos token.Pos) (from, to token.Pos, ok bool) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if cc, isCase := n.(*ast.CaseClause); isCase && cc.Pos() <= pos && pos < cc.End() && len(cc.List) == 1 {
			from, to, ok = cc.Pos(), cc.End(), true
			return false
		}
		return true
	})
	return from, to, ok
}

// funcDeclName is a func's name, or Recv.Name for a method (`func (self *X[V]) M()` is X.M).
func funcDeclName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// stdImporter reads the file's standard-library imports' export data from one `go list -export -deps`: importer.Default runs one
// per package (3.5 s of a rubocop gen for 58). Same export files, so the same types; a module import gets no data, as from Default.
func stdImporter(ctx context.Context, fset *token.FileSet, f *ast.File) types.Importer {
	var pkgs []string
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil && !strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
			pkgs = append(pkgs, path)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}"}, pkgs...)...) //nolint:gosec // the go command over the program's own import paths
	cmd.Dir = os.TempDir()                                                                                                               // the standard library resolves outside any module; a user's go.mod has no say
	out, err := cmd.Output()
	if err != nil {
		return importer.Default()
	}
	exports := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if path, file, ok := strings.Cut(line, "="); ok && file != "" {
			exports[path] = file
		}
	}
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errNoExport, path)
		}
		return os.Open(file) //nolint:gosec // the go command's export file
	})
}

var errNoExport = errors.New("no export data")
