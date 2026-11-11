package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// testCmd is Ruby's `minitest` command: one program, one run, one report, because Ruby loads every test file into one process (decision 80).
func testCmd(ctx context.Context, argv []string) int {
	opts, rest, err := testBuildFlags(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	files, flags, err := expandTestArgs(rest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "rugaroo test: no test files (test_*.rb, *_test.rb, spec_*.rb, *_spec.rb)")
		return 1
	}
	dir, err := compileFiles(ctx, files, "", opts, os.Stderr)
	defer cleanup(dir, opts)
	if err != nil {
		return failed(ctx, err)
	}
	return runProg(ctx, filepath.Join(dir, "prog"), flags)
}

// testBuildFlags finds rugaroo's flags anywhere: minitest's own (-v, -s N, -n /re/) never share their names.
func testBuildFlags(argv []string) (buildOpts, []string, error) {
	var opts buildOpts
	var rest []string
	want := "" // the flag whose value is the next argument
	for _, a := range argv {
		switch {
		case want == "-gcflags":
			opts.gcflags, want = a, ""
		case want == "-I":
			opts.loadPath, want = append(opts.loadPath, a), ""
		case a == "-race":
			opts.race = true
		case a == "-work":
			opts.work = true
		case a == "-debug":
			opts.debug = true
		case a == "-cgo", a == "--cgo":
			err := os.Setenv("RUGAROO_CGO", "1")
			if err != nil {
				return opts, nil, fmt.Errorf("rugaroo test: %w", err)
			}
		case a == "-gcflags", a == "-I":
			want = a
		case strings.HasPrefix(a, "-int-overflow="), strings.HasPrefix(a, "--int-overflow="):
			_, m, _ := strings.Cut(a, "=")
			err := os.Setenv("RUGAROO_INT_OVERFLOW", m)
			if err != nil {
				return opts, nil, fmt.Errorf("rugaroo test: %w", err)
			}
		case strings.HasPrefix(a, "-nil-check="), strings.HasPrefix(a, "--nil-check="):
			_, m, _ := strings.Cut(a, "=")
			err := os.Setenv("RUGAROO_NIL_CHECK", m)
			if err != nil {
				return opts, nil, fmt.Errorf("rugaroo test: %w", err)
			}
		case strings.HasPrefix(a, "-gcflags="):
			opts.gcflags = strings.TrimPrefix(a, "-gcflags=")
		case strings.HasPrefix(a, "-I"): // -Idir, as ruby's, or -I=dir, as the flag package's
			opts.loadPath = append(opts.loadPath, strings.TrimPrefix(strings.TrimPrefix(a, "-I"), "="))
		default:
			rest = append(rest, a)
		}
	}
	if want != "" {
		return opts, nil, fmt.Errorf("rugaroo test: %s needs a value", want)
	}
	return opts, rest, nil
}

// expandTestArgs splits args into test files and program flags, as
// minitest's PathExpander#process_args does. With no path, it is "test".
func expandTestArgs(args []string) ([]string, []string, error) {
	var files, excluded, flags []string
	sawPath := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-") && a != "-" && exists(a[1:]):
			sawPath = true
			ex, err := expandTestPath(a[1:])
			if err != nil {
				return nil, nil, err
			}
			excluded = append(excluded, ex...)
		case !strings.HasPrefix(a, "-") && exists(a):
			sawPath = true
			in, err := expandTestPath(a)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, in...)
		default:
			flags = append(flags, a)
		}
	}
	if !sawPath && exists("test") {
		in, err := expandTestPath("test")
		if err != nil {
			return nil, nil, err
		}
		files = in
	}
	var out []string
	for _, f := range files {
		if !slices.Contains(excluded, f) && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out, flags, nil
}

// expandTestPath is a file as given, or a directory's test files, recursively and sorted (Ruby's Dir[] sorts).
func expandTestPath(p string) ([]string, error) {
	info, err := os.Stat(p) //nolint:gosec // the user's own paths
	if err != nil {
		return nil, err //nolint:wrapcheck // *PathError names the path
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	var out []string
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		for _, pat := range []string{"test_*.rb", "*_test.rb", "spec_*.rb", "*_spec.rb"} {
			if ok, _ := filepath.Match(pat, d.Name()); ok {
				out = append(out, path)
				break
			}
		}
		return nil
	}
	err = filepath.WalkDir(p, walk) //nolint:gosec // the user's own paths
	if err != nil {
		return nil, fmt.Errorf("rugaroo test: %w", err)
	}
	slices.Sort(out)
	return out, nil
}

func exists(p string) bool {
	_, err := os.Stat(p) //nolint:gosec // the user's own paths
	return err == nil
}
