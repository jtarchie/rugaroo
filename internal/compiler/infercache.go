package compiler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
)

// Decision 430: the rounds' result is kept on disk so an unchanged program compiles in one pass; any changed input compiles cold, as rounds started from a stale seed can end elsewhere.

var inferCacheOn atomic.Bool

// SetInferCache turns the inference cache on for this process; the CLI does, tests do not, so the oracle stays cold.
func SetInferCache(on bool) { inferCacheOn.Store(on) }

const inferCacheVersion = 3 // 3: refine records carry decision 440's nil sites

// cachedType is a Type with its classes by Ruby name; decode builds placeholder classes that rehome resolves.
type cachedType struct {
	K    string       `json:"k"`
	N    string       `json:"n,omitempty"`
	T    string       `json:"t,omitempty"`
	A    []cachedType `json:"a,omitempty"`
	I    []cachedType `json:"i,omitempty"`
	R    *cachedType  `json:"r,omitempty"`
	S    *cachedType  `json:"s,omitempty"`
	P    bool         `json:"p,omitempty"`
	Rest bool         `json:"rest,omitempty"`
	Src  string       `json:"src,omitempty"`
	SP   string       `json:"sp,omitempty"`
}

var errUncachedType = errors.New("type kind the inference cache does not store")

func encodeTypes(ts []Type) ([]cachedType, error) {
	if ts == nil {
		return nil, nil
	}
	out := make([]cachedType, len(ts))
	for i, t := range ts {
		ct, err := encodeType(t)
		if err != nil {
			return nil, err
		}
		out[i] = ct
	}
	return out, nil
}

// encodeRef is a field that may hold no type (a TFunc's Self, a void block's Ret).
func encodeRef(t Type) (*cachedType, error) {
	ct, err := encodeType(t)
	if err != nil || ct.K == "" {
		return nil, err
	}
	return &ct, nil
}

func encodeType(t Type) (cachedType, error) {
	switch t := t.(type) {
	case nil:
		return cachedType{}, nil
	case TClass:
		args, err := encodeTypes(t.Args)
		if err != nil {
			return cachedType{}, err
		}
		switch {
		case t.C.metaOf != nil:
			return cachedType{K: "meta", N: t.C.metaOf.RubyName, A: args}, nil
		case t.C.template != nil:
			inst, ierr := encodeTypes(t.C.instArgs)
			if ierr != nil {
				return cachedType{}, ierr
			}
			return cachedType{K: "inst", N: t.C.RubyName, T: t.C.template.RubyName, A: args, I: inst}, nil
		}
		return cachedType{K: "class", N: t.C.RubyName, A: args}, nil
	case TOpt:
		e, err := encodeRef(t.Elem)
		if err != nil {
			return cachedType{}, err
		}
		return cachedType{K: "opt", R: e}, nil
	case TTuple:
		es, err := encodeTypes(t.Elems)
		if err != nil {
			return cachedType{}, err
		}
		return cachedType{K: "tuple", A: es}, nil
	case TUnion:
		ms, err := encodeTypes(t.Members)
		if err != nil {
			return cachedType{}, err
		}
		return cachedType{K: "union", A: ms}, nil
	case TFunc:
		ps, err := encodeTypes(t.Params)
		if err != nil {
			return cachedType{}, err
		}
		r, err := encodeRef(t.Ret)
		if err != nil {
			return cachedType{}, err
		}
		s, err := encodeRef(t.Self)
		if err != nil {
			return cachedType{}, err
		}
		return cachedType{K: "func", A: ps, R: r, S: s, P: t.Proc, Rest: t.Rest, Src: string(t.Src), SP: string(t.SelfPending)}, nil
	case TVar:
		return cachedType{K: "var", N: t.Name}, nil
	case TAny:
		return cachedType{K: "any"}, nil
	case TNil:
		return cachedType{K: "nil"}, nil
	case TVoid:
		return cachedType{K: "void"}, nil
	}
	return cachedType{}, fmt.Errorf("%w: %T", errUncachedType, t)
}

func decodeTypes(cs []cachedType) []Type {
	if cs == nil {
		return nil
	}
	out := make([]Type, len(cs))
	for i := range cs {
		out[i] = decodeType(&cs[i])
	}
	return out
}

func decodeType(c *cachedType) Type {
	if c == nil {
		return nil
	}
	switch c.K {
	case "":
		return nil
	case "class":
		return TClass{C: &Class{RubyName: c.N}, Args: decodeTypes(c.A)}
	case "meta":
		return TClass{C: &Class{metaOf: &Class{RubyName: c.N}}, Args: decodeTypes(c.A)}
	case "inst":
		return TClass{C: &Class{RubyName: c.N, template: &Class{RubyName: c.T}, instArgs: decodeTypes(c.I)}, Args: decodeTypes(c.A)}
	case "opt":
		return TOpt{Elem: decodeType(c.R)}
	case "tuple":
		return TTuple{Elems: decodeTypes(c.A)}
	case "union":
		return TUnion{Members: decodeTypes(c.A)}
	case "func":
		return TFunc{Params: decodeTypes(c.A), Ret: decodeType(c.R), Self: decodeType(c.S), Proc: c.P, Rest: c.Rest, Src: lambdaSrc(c.Src), SelfPending: pendingKey(c.SP)}
	case "var":
		return TVar{Name: c.N}
	case "nil":
		return TNil{}
	case "void":
		return TVoid{}
	}
	return TAny{}
}

// inferCacheFile: the instance list and input hashes ride along, since a hit without them restarts or goes stale.
type inferCacheFile struct {
	Version   int                       `json:"version"`
	Boot      string                    `json:"boot"`
	Inputs    map[string]string         `json:"inputs"`
	Types     map[pendingKey]cachedType `json:"types"`
	NoUnion   []pendingKey              `json:"no_union,omitempty"`
	None      []pendingKey              `json:"none,omitempty"`
	NoDrop    []pendingKey              `json:"no_drop,omitempty"`
	From      map[pendingKey]string     `json:"from,omitempty"`
	Instances []string                  `json:"instances,omitempty"`
	Refine    map[string]*refineRecord  `json:"refine,omitempty"` // refineIvars' outcome per final-pass kind (decision 433)
}

// binaryID is the hash of the running compiler: a cache from another build (another prelude, other rules) never loads.
var binaryID = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe) //nolint:gosec // our own executable
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
})

// inferCachePath names a program's cache by compiler, sources and options; "" when there is no cache dir.
func inferCachePath(sources []Source, opts *options) string {
	id := binaryID()
	base, err := os.UserCacheDir()
	if id == "" || err != nil {
		return ""
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\n%d\n", id, inferCacheVersion)
	for _, s := range sources {
		_, _ = fmt.Fprintf(h, "src %s\n", s.Name) // by name: a traced entry's path is its content hash, which a changed file list moves
	}
	env := opts.env // what changes the output, not the diagnostics
	env.NoWasmCache, env.InferDebug, env.Timing, env.ErrStack, env.GemWarnings, env.PruneDebug, env.PruneWhy, env.UntypedWhy = false, false, false, false, false, false, false, false
	env.DispatchDebug, env.GopoisonDump = "", ""
	_, _ = fmt.Fprintf(h, "lp %q\ngems %q\n%v %v %v %v %v\n%#v\n", opts.loadPath, opts.gemDirs, opts.mainLast, opts.allLibs, opts.skipTests, opts.noPrune, opts.purego, env)
	return filepath.Join(base, "rugaroo", "infer", hex.EncodeToString(h.Sum(nil))[:32]+".json")
}

func bootHash(opts *options) string {
	data, err := json.Marshal(struct {
		B any `json:"b"`
		A any `json:"a"`
	}{opts.boot, opts.autoloaded})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(bootNoise.ReplaceAll(data, nil)) // MRI's object addresses and the hook's temp dir differ every run
	return hex.EncodeToString(sum[:])
}

var bootNoise = regexp.MustCompile(`0x[0-9a-f]{8,}|rugaroo-(?:boot|trace)-[0-9]+`)

func hashFile(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // a file the compile read
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// noteInput records a file the compile tried to read, for the cache's manifest.
func (c *Compiler) noteInput(path string, data []byte, err error) {
	if c.inputs == nil {
		c.inputs = map[string]string{}
	}
	if err != nil {
		c.inputs[path] = ""
		return
	}
	sum := sha256.Sum256(data)
	c.inputs[path] = hex.EncodeToString(sum[:])
}

// cacheInputs is every on-disk file the compile read, by its posKey path, hashed as it is on disk now.
func (c *Compiler) cacheInputs() map[string]string {
	if !inferCacheOn.Load() {
		return nil
	}
	out := map[string]string{}
	for p, h := range c.inputs {
		out[p] = h
	}
	for _, f := range c.files {
		if f.isPrelude() {
			continue
		}
		p := f.path
		if p == "" {
			p = f.Name
		}
		if filepath.IsAbs(p) {
			out[p] = hashFile(p)
		}
	}
	return out
}

func readInferCache(path string) *inferCacheFile {
	data, err := os.ReadFile(path) //nolint:gosec // under the user cache dir
	if err != nil {
		return nil
	}
	var cf inferCacheFile
	if json.Unmarshal(data, &cf) != nil || cf.Version != inferCacheVersion {
		return nil
	}
	return &cf
}

// changed lists the inputs whose content differs now; a different boot manifest changes everything ("").
func (cf *inferCacheFile) changed(opts *options) []string {
	if cf.Boot != bootHash(opts) {
		return []string{""}
	}
	var out []string
	for _, p := range slices.Sorted(func(yield func(string) bool) {
		for p := range cf.Inputs {
			if !yield(p) {
				return
			}
		}
	}) {
		if hashFile(p) != cf.Inputs[p] {
			out = append(out, p)
		}
	}
	return out
}

func (cf *inferCacheFile) inference() *inference {
	inf := &inference{types: map[pendingKey]Type{}, from: cf.From}
	for k, ct := range cf.Types {
		inf.types[k] = decodeType(&ct)
	}
	set := func(ks []pendingKey) map[pendingKey]bool {
		m := map[pendingKey]bool{}
		for _, k := range ks {
			m[k] = true
		}
		return m
	}
	inf.none, inf.noUnion, inf.noDrop = set(cf.None), set(cf.NoUnion), set(cf.NoDrop)
	return inf
}

func saveInferCache(path string, opts *options) {
	inf := opts.infer
	if inf == nil || path == "" {
		return // nothing was inferred: a program with no pending keys has nothing to keep
	}
	cf := inferCacheFile{Version: inferCacheVersion, Boot: bootHash(opts), Inputs: opts.inputs, Types: map[pendingKey]cachedType{}, From: inf.from, Instances: opts.instances, Refine: opts.refineOut}
	for k, t := range inf.types {
		ct, err := encodeType(t)
		if err != nil {
			return // a kind the cache cannot store: no cache, since one missing key reruns every round
		}
		cf.Types[k] = ct
	}
	keys := func(m map[pendingKey]bool) []pendingKey {
		var out []pendingKey
		for k, v := range m {
			if v {
				out = append(out, k)
			}
		}
		slices.Sort(out)
		return out
	}
	cf.None, cf.NoUnion, cf.NoDrop = keys(inf.none), keys(inf.noUnion), keys(inf.noDrop)
	data, err := json.Marshal(cf)
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "infer-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil { // atomic: a concurrent compile reads the old cache or the new
		_ = os.Remove(tmp.Name())
	}
}

// compile retries any error from a cached start cold, so the cache can only save time.
func compile(ctx context.Context, preludeFS fs.FS, sources []Source, opts *options) ([]byte, error) {
	if !inferCacheOn.Load() || opts.seed != nil || opts.infer != nil {
		return compileFresh(ctx, preludeFS, sources, opts)
	}
	opts.env = loadEnv()
	path := inferCachePath(sources, opts)
	cf := readInferCache(path)
	if cf != nil && path != "" {
		if changed := cf.changed(opts); len(changed) > 0 { // liquid: a seed-started round typed ResourceLimits#render_length untyped, cold Integer?
			if opts.env.Timing {
				fmt.Fprintf(os.Stderr, "rugaroo: infer cache: %d of %d inputs changed (%s ...): compiling cold\n", len(changed), len(cf.Inputs), changed[0])
			}
			cf = nil
		}
	}
	if cf != nil {
		opts.infer, opts.instances, opts.refineIn = cf.inference(), slices.Clone(cf.Instances), cf.Refine
		if opts.env.Timing {
			fmt.Fprintf(os.Stderr, "rugaroo: infer cache: hit, %d keys\n", len(cf.Types))
		}
		out, err := compileFresh(ctx, preludeFS, sources, opts)
		if err == nil {
			saveInferCache(path, opts)
			return out, nil
		}
		if opts.env.Timing {
			fmt.Fprintf(os.Stderr, "rugaroo: infer cache: cached start failed (%v); compiling cold\n", err)
		}
		opts.infer, opts.instances, opts.instAt, opts.refineIn, opts.refineOut = nil, nil, nil, nil, nil
		opts.warnings, opts.skipped, opts.goFuncs, opts.instRestarts = nil, nil, nil, 0
	}
	out, err := compileFresh(ctx, preludeFS, sources, opts)
	if err == nil {
		saveInferCache(path, opts)
	}
	return out, err
}
