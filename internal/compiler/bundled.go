package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// bundledGems are gems carried whole in vendored/<name>/{lib,sig} and compiled as installed gems, so no `ruby` is needed (decision 427).
var bundledGems = map[string]bool{"prism": true}

// bundledDirs memoizes each bundled gem's lib dir on disk, extracted once per process (the compiler reads gem files by path).
var bundledDirs sync.Map // name -> func() (string, error)

// bundledLib extracts vendored/<name> to the user cache dir, keyed by content hash so a rugaroo upgrade gets a fresh copy.
// ponytail: the compiler reads gem files with os.ReadFile; an fs.FS-backed gem dir would skip the extraction.
func bundledLib(preludeFS fs.FS, name string) (string, error) {
	f, _ := bundledDirs.LoadOrStore(name, sync.OnceValues(func() (string, error) { return extractBundled(preludeFS, name) }))
	return f.(func() (string, error))() //nolint:forcetypeassert // only this function stores
}

func extractBundled(preludeFS fs.FS, name string) (string, error) {
	root := path.Join("vendored", name)
	files := map[string][]byte{}
	h := sha256.New()
	err := fs.WalkDir(preludeFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(preludeFS, p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		files[p[len(root)+1:]] = data
		h.Write([]byte(p + "\x00"))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("bundled %s: %w", name, err)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "rugaroo", "bundled", name+"-"+hex.EncodeToString(h.Sum(nil))[:16])
	_, err = os.Stat(filepath.Join(dir, "lib"))
	if err == nil {
		return filepath.Join(dir, "lib"), nil
	}
	err = os.MkdirAll(filepath.Dir(dir), 0o750)
	if err != nil {
		return "", fmt.Errorf("bundled %s: %w", name, err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), name+"-tmp-")
	if err != nil {
		return "", fmt.Errorf("bundled %s: %w", name, err)
	}
	for rel, data := range files {
		target := filepath.Join(tmp, filepath.FromSlash(rel))
		err = os.MkdirAll(filepath.Dir(target), 0o750)
		if err == nil {
			err = os.WriteFile(target, data, 0o600)
		}
		if err != nil {
			_ = os.RemoveAll(tmp)
			return "", fmt.Errorf("bundled %s: %w", name, err)
		}
	}
	err = os.Rename(tmp, dir) // atomic: a concurrent compile either sees the whole tree or extracts its own
	if err != nil {
		_ = os.RemoveAll(tmp)
		_, serr := os.Stat(filepath.Join(dir, "lib"))
		if serr != nil {
			return "", fmt.Errorf("bundled %s: %w", name, errors.Join(err, serr))
		}
	}
	return filepath.Join(dir, "lib"), nil
}

// useBundled adds a bundled gem's lib to the gem dirs, so its files are gem code (decision 169) and its sig/ is read (decision 306).
func (c *Compiler) useBundled(name string) string {
	if !bundledGems[name] {
		return ""
	}
	dir, err := bundledLib(c.preludeFS, name)
	if err != nil {
		panic(compileError{msg: err.Error()})
	}
	for _, d := range c.gemDirs {
		if d == dir {
			return dir
		}
	}
	c.gemDirs = append(c.gemDirs, dir)
	c.gemDirsReal = nil
	c.usedBundled, c.pruneGems = true, true
	return dir
}

// bundledFile reports a file under a bundled gem's lib: nothing booted it, so its autoloads follow bundledAutoloads.
func (c *Compiler) bundledFile(f *File) bool {
	if !c.usedBundled || f == nil || f.path == "" {
		return false
	}
	for name := range bundledGems {
		dir, ok := bundledDirs.Load(name)
		if !ok {
			continue
		}
		d, err := dir.(func() (string, error))() //nolint:forcetypeassert // only bundledLib stores
		if err == nil && within(realPath(d), f.path) {
			return true
		}
	}
	return false
}

// bundledAutoloads adds a bundled gem's autoloads to boot's; nothing booted them, so neededAutoloads keeps the named ones (decision 427).
func (c *Compiler) bundledAutoloads(ctx context.Context, booted map[string][]string) map[string][]string {
	if !c.usedBundled {
		return booted
	}
	out := maps.Clone(booted)
	if out == nil {
		out = map[string][]string{}
	}
	var queue []*File
	for _, f := range c.parsed {
		if c.bundledFile(f) {
			queue = append(queue, f)
		}
	}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		anyNode(f.Root, func(n parser.Node) bool {
			call, ok := n.(*parser.CallNode)
			if !ok || call.Receiver != nil || call.Name != "autoload" {
				return false
			}
			args := callArgs(call)
			sym, ok := symbolName(firstArg(args))
			if !ok || len(args) != 2 {
				return false
			}
			target := c.findRequire(f, &parser.CallNode{Location: call.Location, Name: "require", Arguments: &parser.ArgumentsNode{Arguments: args[1:]}})
			if target == "" {
				return false
			}
			path := c.real(target)
			if !slices.Contains(out[path], sym) {
				out[path] = append(out[path], sym)
			}
			if c.parsed[path] == nil {
				src, err := c.readUser(target)
				if err != nil {
					return false
				}
				uf, err := parseFile(ctx, c.parser, path, src, originSource)
				if err != nil {
					panic(compileError{msg: err.Error()})
				}
				uf.path = path
				c.parsed[path] = uf
				queue = append(queue, uf)
			}
			return false
		})
	}
	return out
}

// bundledOptional: an autoload needing a name from outside the gem's core; prism's translation/ and lex_compat need parser or Ripper (decision 427).
func (c *Compiler) bundledOptional(path string) bool {
	p := filepath.ToSlash(path)
	return c.usedBundled && (strings.HasSuffix(p, "/lib/prism/translation.rb") || strings.HasSuffix(p, "/lib/prism/lex_compat.rb") || strings.Contains(p, "/lib/prism/translation/"))
}

// bundledRequireRelative loads a bundled gem's class-body require_relative at compile time, as a top-level one: prism's
// Translation::Parser loads its builder so, which a traced compile would leave to a run-time require (decision 427).
func (c *Compiler) bundledRequireRelative(ctx context.Context, f *File, n *parser.CallNode) bool {
	if n.Receiver != nil || n.Name != "require_relative" || !c.bundledFile(f) {
		return false
	}
	if uf := c.userRequireRelative(ctx, f, n); uf != nil {
		c.addMainStmt(f, &loadFile{call: n, file: uf})
	}
	return true
}

// collectRefineOrRequire pairs two class-body forms so collectClassCall stays under the complexity limit.
func (c *Compiler) collectRefineOrRequire(ctx context.Context, f *File, n *parser.CallNode, scope []*Class) bool {
	return c.collectRefine(ctx, f, n, scope) || c.bundledRequireRelative(ctx, f, n)
}
