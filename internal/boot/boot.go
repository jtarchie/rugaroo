// Package boot runs a Ruby program's load phase under MRI at compile time and
// records what the source alone does not show: methods built by define_method
// or a string eval, the files autoload actually loaded, the classes and their
// ancestors, and the objects held in class state (docs/design.md decision
// 161). The compiled binary still runs no eval or define_method; only boot is
// evaluated, and only for a program that requires a gem.
package boot

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed boot_hook.rb find_gems.rb trace_hook.rb
var hookFS embed.FS

// Manifest is what one boot recorded.
type Manifest struct {
	LoadedFeatures []string                `json:"loaded_features"` // files loaded during boot
	LoadPath       []string                `json:"load_path"`       // $LOAD_PATH as boot left it: what `$LOAD_PATH` reads in the compiled program (decision 301)
	StdDirs        []string                `json:"std_dirs"`        // rubylibdir and archdir: MRI's own library, never a source (decision 404)
	Classes        []ClassInfo             `json:"classes"`
	DefineMethods  []MethodInfo            `json:"define_methods"`
	Aliases        []AliasInfo             `json:"aliases"`  // each alias_method boot ran (decision 330)
	Includes       []IncludeInfo           `json:"includes"` // each module an include call added during boot (decision 331)
	Evals          []EvalInfo              `json:"evals"`
	Autoloads      []AutoloadInfo          `json:"autoloads"`
	ClassState     []ClassStateInfo        `json:"class_state"`
	Branches       []BranchInfo            `json:"branches"`
	ConstClasses   map[string]string       `json:"const_classes"` // a constant naming a class under another name (decision 203)
	Delegators     map[string][]string     `json:"delegators"`    // def_delegators' method names by "path:line" (decision 204)
	Structs        map[string][]StructProp `json:"structs"`       // each T::Struct's props after load (decision 241)
}

// Loc is a file and 1-based line; records embed it so "file"/"line" stay flat JSON keys, and bare "path:line" strings go through String/ParseLoc.
type Loc struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// String is "file:line".
func (l Loc) String() string { return l.File + ":" + strconv.Itoa(l.Line) }

// ParseLoc reads "path:line", splitting at the last colon (the path may hold one). ok is false without a numeric line.
func ParseLoc(s string) (Loc, bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return Loc{}, false
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return Loc{}, false
	}
	return Loc{File: s[:i], Line: n}, true
}

// AliasInfo is one alias_method boot ran: owner's new name for its method old, called at file:line.
type AliasInfo struct {
	Owner     string `json:"owner"`
	Singleton bool   `json:"singleton"`
	Name      string `json:"name"`
	Old       string `json:"old"`
	Loc              // the alias_method call
}

// IncludeInfo is one module an include call added during boot: to the class Owner names, or (anonymous) the one the
// Class.new call at Site ("path:line") made; File and Line are the include call's.
type IncludeInfo struct {
	Owner  string `json:"owner"`
	Site   string `json:"site"`
	Module string `json:"module"`
	Loc           // the include call
}

// StructProp is one T::Struct prop as MRI holds it: its Sorbet type's text, and its default's literal source if it has a
// literal one.
type StructProp struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
}

// BranchInfo is one if/unless boot reached and how often each arm ran (decision 201).
type BranchInfo struct {
	Loc
	Col  int       `json:"col"`
	Kind string    `json:"kind"`
	Then int       `json:"then"`
	Else int       `json:"else"`
	Arms []ArmInfo `json:"arms"` // a case's when/else arms, by where each arm's body starts
}

// ArmInfo is one arm of a branch: where its body starts and how often it ran.
type ArmInfo struct {
	Kind  string `json:"kind"`
	Line  int    `json:"line"`
	Col   int    `json:"col"`
	Count int    `json:"count"`
}

// ClassInfo is a class or module defined during boot.
type ClassInfo struct {
	Name      string   `json:"name"`
	Super     string   `json:"super"`
	Ancestors []string `json:"ancestors,omitempty"`
	Path      string   `json:"path"`
	Line      int      `json:"line"`
}

// MethodInfo is a method made by define_method (or from an existing
// UnboundMethod): Source is the block's text, nil for the latter.
type MethodInfo struct {
	Owner          string            `json:"owner"`
	Singleton      bool              `json:"singleton"` // defined on Owner's singleton class
	Name           string            `json:"name"`
	Loc                              // the define_method call's site
	Source         string            `json:"source"`
	From           string            `json:"from"`
	Private        bool              `json:"private"`
	ModuleFunction bool              `json:"module_function"`    // also a singleton method (module_function scope)
	Captures       map[string]string `json:"captures,omitempty"` // the caller's plain-data locals, as Ruby literals (decision 198)
}

// EvalInfo is a string eval: Source is the evaluated text.
type EvalInfo struct {
	Loc
	Source string `json:"source"`
	Caller string `json:"caller"` // "path:line" of the eval call's caller (ParseLoc)
	Kind   string `json:"kind"`   // eval, class_eval (module_eval) or instance_eval
	Self   string `json:"self"`   // the named module it ran on, "" for eval or an object
}

// AutoloadInfo is an autoload registration and whether boot resolved it.
type AutoloadInfo struct {
	Owner    string `json:"owner"`
	Const    string `json:"const"`
	Path     string `json:"path"`
	Resolved bool   `json:"resolved"`
	Loaded   string `json:"loaded"`
}

// ClassStateInfo is one instance variable held on a class/module object.
type ClassStateInfo struct {
	Owner    string `json:"owner"`
	Ivar     string `json:"ivar"`
	Kind     string `json:"kind"`
	Desc     string `json:"desc"`
	Recreate bool   `json:"recreate"` // a Mutex/Queue: recreated empty, not copied
	Frozen   bool   `json:"frozen"`
}

// Limits on one ruby run. Variables so tests can shrink them; compile-time boot executes the program (decision 262).
var (
	bootTimeout       = 10 * time.Minute // a boot that blocks (a server started at load, sleep) must not hang the compile
	waitDelay         = 5 * time.Second  // after the kill, a pipe held by a grandchild cannot block Wait
	maxBytes    int64 = 256 << 20        // the manifest and FindGems' output
	maxStderr         = 64 << 10         // the first bytes of stderr kept for an error
)

// headWriter keeps the first maxStderr bytes written and counts the rest, so a chatty boot cannot grow memory.
type headWriter struct {
	buf     bytes.Buffer
	dropped int
}

func (w *headWriter) Write(p []byte) (int, error) {
	room := min(maxStderr-w.buf.Len(), len(p))
	if room > 0 {
		w.buf.Write(p[:room])
	}
	w.dropped += len(p) - max(room, 0)
	return len(p), nil
}

func (w *headWriter) String() string {
	if w.dropped == 0 {
		return w.buf.String()
	}
	return fmt.Sprintf("%s\n... stderr truncated, %d more bytes", w.buf.String(), w.dropped)
}

// limitWriter fails, and cancels the run so ruby is not left blocked on a full pipe, once more than maxBytes are written.
type limitWriter struct {
	buf    bytes.Buffer
	over   bool
	cancel context.CancelFunc
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(w.buf.Len()+len(p)) > maxBytes {
		w.over = true
		w.cancel()
		return 0, fmt.Errorf("output exceeds the %d byte limit", maxBytes)
	}
	return w.buf.Write(p) //nolint:wrapcheck // bytes.Buffer never fails
}

// Ruby is the MRI that runs boot and traces: RUGAROO_RUBY, else ruby on PATH. A launcher that puts rugaroo in ruby's place
// (overrides/brew's) names the real one there (decision 296).
func Ruby() string {
	if r := os.Getenv("RUGAROO_RUBY"); r != "" {
		return r
	}
	return "ruby"
}

// runRuby runs ruby with args under bootTimeout (the earlier of it and ctx's deadline wins) and returns its capped stderr.
func runRuby(ctx context.Context, args, env []string, stdout func(context.CancelFunc) io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, bootTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, Ruby(), args...) //nolint:gosec // ruby runs the program's own boot
	cmd.Env = env
	cmd.WaitDelay = waitDelay
	// ponytail: the kill reaches ruby only; a grandchild (system("sleep 99")) outlives it, WaitDelay just stops Wait blocking on it. Setpgid + kill(-pid) needs build-tagged files (this package also builds into the wasm playground).
	var stderr headWriter
	cmd.Stderr = &stderr
	if stdout != nil {
		cmd.Stdout = stdout(cancel)
	}
	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) { // a run that finished in time is not a timeout
		return stderr.String(), fmt.Errorf("timed out (boot timeout %s, or the caller's deadline): %w", bootTimeout, context.DeadlineExceeded)
	}
	return stderr.String(), err
}

// Capture runs entry under MRI with the tracing hook installed, with
// loadPaths as `-I` directories, and returns the boot manifest. This executes
// the program's load phase (decision 262).
func Capture(ctx context.Context, entry string, loadPaths ...string) (*Manifest, error) {
	dir, err := os.MkdirTemp("", "rugaroo-boot-")
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	hook, err := hookFS.ReadFile("boot_hook.rb")
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	hookPath := filepath.Join(dir, "boot_hook.rb")
	err = os.WriteFile(hookPath, hook, 0o600)
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	out := filepath.Join(dir, "manifest.json")
	args := []string{"-r", hookPath} // by path, not `-I dir`: the temp dir would enter $LOAD_PATH and the output (decision 88)
	for _, lp := range loadPaths {
		args = append(args, "-I", lp)
	}
	args = append(args, "--", entry) // "--": an entry named -x.rb is a file, not an option
	stderr, err := runRuby(ctx, args, append(os.Environ(), "RUGAROO_BOOT_OUT="+out), nil)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("boot %s: running the program's load phase needs ruby on PATH", entry)
	}
	if err != nil {
		return nil, fmt.Errorf("boot %s: %w\n%s", entry, err, stderr)
	}
	info, err := os.Stat(out)
	if err != nil {
		return nil, fmt.Errorf("boot %s: the run wrote no manifest (the program called exit!, or the hook's dump raised): %w\n%s", entry, err, stderr)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("boot %s: manifest is %d bytes, over the %d byte limit", entry, info.Size(), maxBytes)
	}
	data, err := os.ReadFile(out) //nolint:gosec // under os.MkdirTemp
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	var m Manifest
	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("boot: %w", err)
	}
	return &m, nil
}

// FindGems asks MRI, once, for the lib dirs of the installed gems providing names and their runtime dependencies, through Bundler when BUNDLE_GEMFILE is set or dir holds a Gemfile. Bundler's setup runs the Gemfile (decision 262).
func FindGems(ctx context.Context, dir string, names []string) ([]string, error) {
	script, err := hookFS.ReadFile("find_gems.rb")
	if err != nil {
		return nil, fmt.Errorf("find gems: %w", err)
	}
	args := []string{"-e", string(script), "--"}
	env := os.Environ()
	gemfile := os.Getenv("BUNDLE_GEMFILE")
	if gemfile == "" {
		_, serr := os.Stat(filepath.Join(dir, "Gemfile"))
		if serr == nil {
			gemfile = filepath.Join(dir, "Gemfile")
			env = append(env, "BUNDLE_GEMFILE="+gemfile)
		}
	}
	if gemfile != "" {
		args = append([]string{"-rbundler/setup"}, args...)
	}
	var out *limitWriter
	stderr, err := runRuby(ctx, append(args, names...), env, func(cancel context.CancelFunc) io.Writer {
		out = &limitWriter{cancel: cancel}
		return out
	})
	if out != nil && out.over { // the kill after the cap is what Run reports
		return nil, fmt.Errorf("find gems: ruby output exceeds the %d byte limit", maxBytes)
	}
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("require %q: finding an installed gem needs ruby on PATH", names[0])
	}
	if err != nil {
		return nil, fmt.Errorf("%s (%w)", strings.TrimSpace(stderr), err)
	}
	return strings.FieldsFunc(out.buf.String(), func(r rune) bool { return r == '\n' }), nil
}

// Class returns the named class, or nil.
func (m *Manifest) Class(name string) *ClassInfo {
	for i := range m.Classes {
		if m.Classes[i].Name == name {
			return &m.Classes[i]
		}
	}
	return nil
}

// DefinedMethods returns the methods define_method made on owner.
func (m *Manifest) DefinedMethods(owner string) []MethodInfo {
	var out []MethodInfo
	for _, d := range m.DefineMethods {
		if d.Owner == owner {
			out = append(out, d)
		}
	}
	return out
}
