package boot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Loaded is what one traced run of a program loaded (decision 296).
type Loaded struct {
	Features []string   // loaded files, in require start order
	LoadPath []string   // $LOAD_PATH at exit
	StdDirs  []string   // MRI's rubylibdir and archdir
	Evals    []EvalInfo // each run-time string eval's source and caller (Kind "run"), compiled at its call site (decision 374)
}

// Trace runs the whole program under MRI, as `ruby program args` would, for its load list only (decision 296).
func Trace(ctx context.Context, program string, args, rubyOpts []string) (*Loaded, error) {
	dir, err := os.MkdirTemp("", "rugaroo-trace-")
	if err != nil {
		return nil, fmt.Errorf("trace: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	hook, err := hookFS.ReadFile("trace_hook.rb")
	if err != nil {
		return nil, fmt.Errorf("trace: %w", err)
	}
	hookPath := filepath.Join(dir, "trace_hook.rb")
	err = os.WriteFile(hookPath, hook, 0o600)
	if err != nil {
		return nil, fmt.Errorf("trace: %w", err)
	}
	out := filepath.Join(dir, "loaded.txt")
	rargs := append(append([]string{}, rubyOpts...), "-r", hookPath, "--", program) // by path: a temp dir on $LOAD_PATH would reach the output (decision 88)
	stderr, err := runRuby(ctx, append(rargs, args...), append(os.Environ(), "RUGAROO_TRACE_OUT="+out), func(context.CancelFunc) io.Writer { return io.Discard })
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("trace %s: needs ruby on PATH (or RUGAROO_RUBY)", program)
	}
	f, rerr := os.Open(out) //nolint:gosec // under os.MkdirTemp
	if rerr != nil {
		return nil, fmt.Errorf("trace %s: the run wrote no load list: %w\n%s", program, errors.Join(rerr, err), stderr)
	}
	defer func() { _ = f.Close() }()
	data, rerr := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if rerr != nil {
		return nil, fmt.Errorf("trace %s: reading the load list: %w", program, rerr)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("trace %s: the load list is over the %d byte limit", program, maxBytes)
	}
	l := &Loaded{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, len(data)+1) // an eval record holds its whole source on one line; data is already capped at maxBytes
	for sc.Scan() {
		kind, val, _ := strings.Cut(sc.Text(), "\t")
		switch kind {
		case "F":
			if filepath.Base(val) != "trace_hook.rb" || !strings.Contains(val, "rugaroo-trace-") { // the temp dir, symlinks resolved
				l.Features = append(l.Features, val)
			}
		case "L":
			l.LoadPath = append(l.LoadPath, val)
		case "S":
			l.StdDirs = append(l.StdDirs, val)
		case "E":
			caller, rest, _ := strings.Cut(val, "\t")
			file, enc, _ := strings.Cut(rest, "\t")
			src, derr := base64.StdEncoding.DecodeString(enc)
			if derr != nil {
				return nil, fmt.Errorf("trace %s: an eval record: %w", program, derr)
			}
			l.Evals = append(l.Evals, EvalInfo{Loc: Loc{File: file}, Caller: caller, Source: string(src), Kind: "run"})
		}
	}
	err = sc.Err() // a line over the buffer would otherwise end the list early
	if err != nil {
		return nil, fmt.Errorf("trace %s: reading the load list: %w", program, err)
	}
	return l, nil
}
