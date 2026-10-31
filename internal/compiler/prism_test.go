package compiler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// TestPrismWasm: one wasm (go-ruby-prism's), one version across it, vendored/prism, prismVersion and prelude/prism.rb (decision 427).
func TestPrismWasm(t *testing.T) {
	src, err := os.ReadFile("../../prelude/go/prismwasm.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const rbPrismWasm = "([^"]+)"`).FindSubmatch(src)
	if m == nil {
		t.Fatal("prismwasm.go has no rbPrismWasm")
	}
	gz, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(m[1])))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", "github.com/danielgatis/go-ruby-prism").Output()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), "wasm", "prism.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bin, want) {
		t.Fatal("prelude/go/prismwasm.go is not go-ruby-prism's prism.wasm: run scripts/prism-wasm")
	}
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer func() { _ = rt.Close(ctx) }()
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	mod, err := rt.Instantiate(ctx, bin)
	if err != nil {
		t.Fatal(err)
	}
	out, err := mod.ExportedFunction("pm_version").Call(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ver, _ := mod.Memory().Read(uint32(out[0]), uint32(len(prismVersion)+1)) //nolint:gosec // a wasm32 pointer
	if string(ver) != prismVersion+"\x00" {
		t.Errorf("prism.wasm is %q, prismVersion %q", ver, prismVersion)
	}
	serialize, err := os.ReadFile("../../vendored/prism/lib/prism/serialize.rb")
	if err != nil {
		t.Fatal(err)
	}
	var parts []string
	for _, k := range []string{"MAJOR", "MINOR", "PATCH"} {
		if m := regexp.MustCompile(k + `_VERSION = (\d+)`).FindSubmatch(serialize); m != nil {
			parts = append(parts, string(m[1]))
		}
	}
	if got := strings.Join(parts, "."); got != prismVersion {
		t.Errorf("vendored/prism is %s, prismVersion %s: run scripts/vendor-prism %s", got, prismVersion, prismVersion)
	}
	rb, err := os.ReadFile("../../prelude/prism.rb")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(rb, []byte(`VERSION = "`+prismVersion+`"`)) {
		t.Errorf("prelude/prism.rb's VERSION is not %s", prismVersion)
	}
}
