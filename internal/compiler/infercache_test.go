package compiler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestInferCache: a hit must equal a cold compile byte for byte, and an edited input must compile cold (decision 430).
func TestInferCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // os.UserCacheDir: the cache lands here, not in the developer's
	t.Setenv("XDG_CACHE_HOME", "")
	dir := t.TempDir()
	main := filepath.Join(dir, "main.rb")
	src := "def twice(x) = x * 2\n\ndef shout(word, times)\n  word.upcase * times\nend\n\nputs twice(21)\nputs shout(\"hi\", 3)\n"
	run := func(on bool) ([]byte, int) {
		t.Helper()
		SetInferCache(on)
		defer SetInferCache(false)
		data, err := os.ReadFile(main) //nolint:gosec // under t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
		n := 0 // compile passes: one is a hit, a cold one adds the rounds
		out, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: main, Src: data, Path: main}}, &options{testHook: func() { n++ }})
		if err != nil {
			t.Fatal(err)
		}
		return out, n
	}
	write := func(s string) {
		t.Helper()
		err := os.WriteFile(main, []byte(s), 0o600) //nolint:gosec // under t.TempDir()
		if err != nil {
			t.Fatal(err)
		}
	}
	write(src)
	ref, cold := run(false)
	miss, _ := run(true)
	hit, warm := run(true)
	if !bytes.Equal(ref, miss) || !bytes.Equal(ref, hit) {
		t.Fatal("a cached compile differs from a cold one")
	}
	if warm != 1 || cold < 3 {
		t.Errorf("a hit ran %d compile passes, cold %d: want 1 and the rounds", warm, cold)
	}
	write("# edited\n" + src)
	ref2, _ := run(false)
	edited, n := run(true)
	if !bytes.Equal(ref2, edited) {
		t.Fatal("a compile after an edit differs from a cold one")
	}
	if n != cold {
		t.Errorf("after an edit %d compile passes, cold %d: the stale cache was used", n, cold)
	}
}

// TestRefineReplay: a hit applies the cold compile's refineIvars record instead of the dry-runs, output byte for byte the cold one (decision 433).
func TestRefineReplay(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")
	dir := t.TempDir()
	main := filepath.Join(dir, "main.rb")
	src := "class Bag\n  def initialize\n    @items = []\n  end\n\n  def add(x)\n    @items << x\n    self\n  end\n\n  def total = @items.sum\nend\n\ndef twice(x) = x * 2\n\nputs Bag.new.add(twice(1)).add(3).total\n"
	err := os.WriteFile(main, []byte(src), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	run := func(on bool) ([]byte, *options) {
		t.Helper()
		SetInferCache(on)
		defer SetInferCache(false)
		opts := &options{}
		out, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: main, Src: []byte(src), Path: main}}, opts)
		if err != nil {
			t.Fatal(err)
		}
		return out, opts
	}
	ref, _ := run(false)
	miss, mopts := run(true)
	if mopts.refineOut["lazy"] == nil {
		t.Fatal("the cold compile recorded no refineIvars outcome: the program has no candidate")
	}
	hit, hopts := run(true)
	if !bytes.Equal(ref, miss) || !bytes.Equal(ref, hit) {
		t.Fatal("a cached compile differs from a cold one")
	}
	if hopts.refineIn["lazy"] == nil || hopts.refineOut["lazy"] != hopts.refineIn["lazy"] {
		t.Error("the hit did not replay the record")
	}
}
