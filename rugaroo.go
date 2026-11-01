// Package rugaroo compiles a typed subset of Ruby to Go.
package rugaroo

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtarchie/rugaroo/internal/compiler"
)

// Prelude holds prelude.rb, its require_relatives, prelude/go/*.go (pure-Go helpers embedded directly; see loadPreludeGo), and
// the vendored gems' lib files (vendored/<name>/, mirrored from the gems/<name> submodules by scripts/vendor-gems so the
// module proxy has them; pure Ruby compiled as user code when required: vendoredLibs, decision 328).
// The character class leaves out prelude/go/0_stubs.go, the generated type-checking stubs (decision 154): helper files
// must start with a letter.
//
//go:embed prelude.rb prelude/*.rb prelude/go/[a-z]*.go prelude/go/[a-z]*.h vendored/*/*.rb vendored/*/*/*.rb vendored/*/*/*/*.rb vendored/prism
var Prelude embed.FS

// GemSigs holds vendored `.rbs` signatures for the gems rugaroo compiles
// (decision 160), under sig/gems/<gem>/. A gem program (one that requires a
// gem, so its load phase runs under MRI at compile time, decision 161) reads
// them to type the gem's otherwise-unannotated classes.
//
//go:embed sig/gems/*/*.rbs
var GemSigs embed.FS

// GoVersion is the go directive for the module generated code builds in.
const GoVersion = "1.25.0"

//go:embed go.sum
var goSum string

// GoMod is the go.mod generated code builds in: the runtime modules a program may import (regexp2 for patterns RE2
// cannot match, decision 242; purego for Fiddle, decision 326; go-yaml for YAML, decision 376; x/text for case mapping,
// normalization and legacy encodings, decision 380; wazero for prism.wasm, decision 427); Go loads one only when main.go imports it.
const GoMod = "module gen\n\ngo " + GoVersion + "\n\nrequire (\n\tgithub.com/dlclark/regexp2 " + regexp2Version + "\n\tgithub.com/ebitengine/purego " + puregoVersion + "\n\tgithub.com/goccy/go-yaml " + goYAMLVersion + "\n\tgithub.com/tetratelabs/wazero " + wazeroVersion + "\n\tgolang.org/x/text " + xTextVersion + "\n)\n"

// regexp2Version, puregoVersion, goYAMLVersion and xTextVersion are the pins GoMod and GoSum derive from;
// TestGoModMatchesRepo holds them to go.mod. x/text v0.41.0 is the last that builds with go 1.25 (Unicode 17, as MRI 4.0).
const (
	regexp2Version = "v1.12.0"
	puregoVersion  = "v0.11.2"
	goYAMLVersion  = "v1.19.2"
	xTextVersion   = "v0.41.0"
	wazeroVersion  = "v1.9.0"
)

// GoSum is go.sum for GoMod: its modules' lines from rugaroo's own.
func GoSum() string {
	var b strings.Builder
	for line := range strings.Lines(goSum) {
		for _, mod := range []string{"github.com/dlclark/regexp2 " + regexp2Version, "github.com/ebitengine/purego " + puregoVersion, "github.com/goccy/go-yaml " + goYAMLVersion, "github.com/tetratelabs/wazero " + wazeroVersion, "golang.org/x/text " + xTextVersion} {
			if strings.HasPrefix(line, mod+" ") || strings.HasPrefix(line, mod+"/go.mod ") {
				b.WriteString(line)
			}
		}
	}
	return b.String()
}

// WriteModule writes GoMod and GoSum into dir.
func WriteModule(dir string) error {
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(GoMod), 0o600)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "go.sum"), []byte(GoSum()), 0o600)
	}
	if err != nil {
		return fmt.Errorf("generated module: %w", err)
	}
	return nil
}

// Compile transpiles the Ruby source of mainName (with the embedded prelude)
// into a single Go file. loadPath is `ruby -I`'s: directories a `require`
// searches for the program's own files.
func Compile(ctx context.Context, mainName string, src []byte, loadPath ...string) ([]byte, []string, error) {
	out, warnings, err := compiler.CompileWithWarnings(ctx, Prelude, mainName, src, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, nil
}

// CompileTraced compiles an entry file listing the requires MRI loaded (decision 197), then the program's main file
// (the CLI's -main, decision 288). The -I dirs are gem dirs.
func CompileTraced(ctx context.Context, entryName string, src []byte, main string, loadPath ...string) ([]byte, []string, error) {
	srcs := []compiler.Source{{Name: filepath.Base(entryName), Src: src, Path: entryName}}
	out, warnings, err := compiler.CompileTraced(ctx, Prelude, GemSigs, srcs, main, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, nil
}

// CompileTrace runs main under MRI once per args in runs (rubyOpts first) and compiles what those runs loaded
// (decision 296).
func CompileTrace(ctx context.Context, main string, runs [][]string, rubyOpts []string, loadPath ...string) ([]byte, []string, error) {
	out, warnings, err := compiler.CompileTrace(ctx, Prelude, GemSigs, main, runs, rubyOpts, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, nil
}

// CompileWithGems is Compile for the CLI: a require only an installed gem answers is looked up under MRI and the program compiled with the gem (decision 173); a program without one never starts Ruby.
func CompileWithGems(ctx context.Context, mainName string, src []byte, loadPath ...string) ([]byte, []string, error) {
	return CompileWithGemDirs(ctx, mainName, src, nil, loadPath...)
}

// CompileWithGemDirs is CompileWithGems with directories compiled as gems the program carries (`rugaroo build -gem dir`, decision 406).
func CompileWithGemDirs(ctx context.Context, mainName string, src []byte, gemDirs []string, loadPath ...string) ([]byte, []string, error) {
	srcs := []compiler.Source{{Name: filepath.Base(mainName), Src: src, Path: mainName}}
	out, warnings, err := compiler.CompileWithGemDirs(ctx, Prelude, GemSigs, srcs, gemDirs, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, nil
}

// File is one Ruby source file of a multi-file program.
type File struct {
	Name string // as given, relative to where the program runs: it is what messages and failure locations show
	Src  []byte
}

// CompileFiles transpiles several Ruby files into one Go program, as Ruby
// loads them into one process: in order, each file's top level running
// before the next's (decision 84).
func CompileFiles(ctx context.Context, files []File, loadPath ...string) ([]byte, []string, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, err := compiler.CompileFilesWithWarnings(ctx, Prelude, srcs, loadPath...)
	if err != nil {
		return nil, warnings, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, nil
}

// CompileTestsSkippingWithGems is CompileTestsSkipping for a suite that requires installed gems: a require only a gem
// answers is looked up under MRI from dir (its Gemfile, when it has one), as CompileWithGems does for a program.
func CompileTestsSkippingWithGems(ctx context.Context, files []File, seed *Inference, dir string) ([]byte, []string, []SkippedTest, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, skipped, err := compiler.CompileTestsSkippingWithGems(ctx, Prelude, GemSigs, srcs, seed, dir)
	if err != nil {
		return nil, warnings, skipped, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, skipped, nil
}

// SkippedTest is a test method CompileTestsSkipping could not compile and turned into a skip.
type SkippedTest = compiler.SkippedTest

// Inference carries parameter types inferred from use (docs/design.md
// decision 146) between compiles of nearly the same sources.
type Inference = compiler.Inference

// NewInference is an empty Inference for CompileTestsSkipping.
func NewInference() *Inference { return compiler.NewInference() }

// SetInferCache turns on the on-disk inference cache (decision 430) for this process.
func SetInferCache(on bool) { compiler.SetInferCache(on) }

// CompileTestsSkipping is CompileFiles for running what a test suite can: each test_ method (an `it` included) that fails to compile becomes a minitest skip carrying the error instead of failing the build.
// seed, when not nil, carries inferred parameter types to the next call for the same sources, which then starts from them.
func CompileTestsSkipping(ctx context.Context, files []File, seed *Inference) ([]byte, []string, []SkippedTest, error) {
	srcs := make([]compiler.Source, len(files))
	for i, f := range files {
		srcs[i] = compiler.Source{Name: f.Name, Src: f.Src}
	}
	out, warnings, skipped, err := compiler.CompileTestsSkipping(ctx, Prelude, srcs, seed)
	if err != nil {
		return nil, warnings, skipped, fmt.Errorf("rugaroo: %w", err)
	}
	return out, warnings, skipped, nil
}
