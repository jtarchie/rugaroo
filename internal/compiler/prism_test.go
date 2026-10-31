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

// TestPrismWasm: one wasm (go-ruby-prism's), one version across it, prismVersion and prelude/prism.rb (decision 427).
func TestPrismWasm(t *testing.T) {
	src, err := os.ReadFile("../../prelude/go/prism_wasm.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`const rbPrismWasm = "([^"]+)"`).FindSubmatch(src)
	if m == nil {
		t.Fatal("prism_wasm.go has no rbPrismWasm")
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
		t.Fatal("prelude/go/prism_wasm.go is not go-ruby-prism's prism.wasm: run scripts/prism-wasm")
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
	rb, err := os.ReadFile("../../prelude/prism.rb")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(rb, []byte(`VERSION = "`+prismVersion+`"`)) {
		t.Errorf("prelude/prism.rb's VERSION is not %s", prismVersion)
	}
}

func TestCheckPrismGem(t *testing.T) {
	ok := "/r/lib/ruby/gems/4.0.0/gems/prism-" + prismVersion + "/lib"
	for _, c := range []struct {
		names, dirs []string
		want        string
	}{
		{[]string{"prism"}, []string{ok}, ""},
		{nil, []string{"/r/gems/rack-3.2.7/lib"}, ""},
		{[]string{"prism"}, []string{"/r/gems/prism-1.7.0/lib"}, "the prism gem found is 1.7.0"},
		{nil, []string{"/r/gems/prism-1.10.0/lib"}, "gem install prism -v " + prismVersion},
		{[]string{"prism"}, nil, "needs the prism " + prismVersion + " gem installed"},
	} {
		err := checkPrismGem(c.names, c.dirs)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("checkPrismGem(%q, %q) = %v, want %q", c.names, c.dirs, err, c.want)
		}
	}
}
