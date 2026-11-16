//go:build rugaroo_prelude

package prelude

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// rbHome is `~` and `~name`: HOME (which must be absolute) or the passwd entry when HOME is unset, as MRI's rb_default_home_dir.
func rbHome(name string) string {
	if name != "" {
		u, err := user.Lookup(name)
		if err != nil {
			panic(NewArgumentError(Ref(String("user " + name + " doesn't exist"))))
		}
		return u.HomeDir
	}
	h, ok := os.LookupEnv("HOME")
	if !ok {
		u, err := user.Current()
		if err != nil {
			panic(NewArgumentError(Ref(String("couldn't find login name -- expanding '~'"))))
		}
		return u.HomeDir
	}
	if !strings.HasPrefix(h, "/") {
		panic(NewArgumentError(Ref(String("non-absolute home"))))
	}
	return h
}

// rbExpandPath is File.expand_path without a base: `~` expanded, `.`/`..` and repeated `/` folded. A leading run of `/` stays (POSIX leaves `//` implementation-defined) until a `..` climbs past it, as in MRI.
func rbExpandPath(p string) string {
	if strings.IndexByte(p, 0) >= 0 {
		panic(NewArgumentError(Ref(String("string contains null byte"))))
	}
	switch {
	case strings.HasPrefix(p, "~"):
		name, rest, _ := strings.Cut(p[1:], "/")
		p = rbHome(name) + "/" + rest
	case !strings.HasPrefix(p, "/"):
		wd, err := os.Getwd()
		if err != nil {
			panic(rbSysErr(err, "rb_file_expand_path", p))
		}
		p = wd + "/" + p
	}
	rest := strings.TrimLeft(p, "/")
	lead := p[:len(p)-len(rest)]
	var parts []string
	for _, s := range strings.Split(rest, "/") {
		switch s {
		case "", ".":
		case "..":
			if len(parts) == 0 {
				lead = "/"
			} else {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, s)
		}
	}
	return lead + strings.Join(parts, "/")
}

// rbSysErr is MRI's Errno exception for a failed call: "<strerror> @ <MRI function> - <path>".
func rbSysErr(err error, fn, path string) any {
	loc := " @ " + fn + " - " + path
	msg := func(s string) *String { return Ref(String(s + loc)) }
	kloc := "" // Process.kill's EPERM and ESRCH messages are the strerror alone; File.delete's carry the location
	if fn == "apply2files" {
		kloc = loc
	}
	switch { // errnos before fs.Err*: Go files ENOTEMPTY under ErrExist, EPERM under ErrPermission
	case rbKillErrnoHook != nil && errors.Is(err, syscall.EPERM):
		return rbKillErrnoHook(err, kloc)
	case errors.Is(err, syscall.ENOTEMPTY):
		return NewErrno_ENOTEMPTY(msg("Directory not empty"))
	case errors.Is(err, fs.ErrNotExist):
		return NewErrno_ENOENT(msg("No such file or directory"))
	case errors.Is(err, fs.ErrExist):
		return NewErrno_EEXIST(msg("File exists"))
	case errors.Is(err, fs.ErrPermission):
		return NewErrno_EACCES(msg("Permission denied"))
	case errors.Is(err, syscall.EISDIR):
		return NewErrno_EISDIR(msg("Is a directory"))
	case errors.Is(err, syscall.ENOTDIR):
		return NewErrno_ENOTDIR(msg("Not a directory"))
	case errors.Is(err, syscall.EBUSY):
		return NewErrno_EBUSY(msg(strings.ToUpper(syscall.EBUSY.Error()[:1]) + syscall.EBUSY.Error()[1:])) // strerror: "Device or resource busy" (Linux), "Resource busy" (macOS)
	case errors.Is(err, syscall.ESPIPE):
		return NewErrno_ESPIPE(msg("Illegal seek"))
	case errors.Is(err, syscall.ELOOP):
		return NewErrno_ELOOP(msg("Too many levels of symbolic links"))
	case errors.Is(err, syscall.EROFS):
		return NewErrno_EROFS(msg("Read-only file system"))
	case errors.Is(err, syscall.EINVAL):
		return NewErrno_EINVAL(msg("Invalid argument"))
	case errors.Is(err, syscall.EBADF):
		return NewErrno_EBADF(Ref(String("Bad file descriptor")))
	case errors.Is(err, syscall.ENOTTY):
		return NewErrno_ENOTTY(Ref(String("Inappropriate ioctl for device")))
	case errors.Is(err, syscall.ENODEV):
		return NewErrno_ENODEV(Ref(String("Operation not supported by device")))
	case errors.Is(err, syscall.ESRCH), errors.Is(err, syscall.EPERM):
		if rbKillErrnoHook != nil {
			return rbKillErrnoHook(err, kloc)
		}
	}
	return NewIOError(Ref(String(err.Error())))
}

// rbKillErrnoHook builds Errno::ESRCH or Errno::EPERM with loc (" @ fn - path", or "") after the strerror; prelude/errno_process.rb sets it when a program names either.
var rbKillErrnoHook func(err error, loc string) any

// rbStrs converts Go strings to an Array[String].
func rbStrs(ss []string) *Array[String] {
	a := &Array[String]{s: make([]String, 0, len(ss))}
	for _, s := range ss {
		a.s = append(a.s, String(s))
	}
	return a
}

// rbTrimSlash drops trailing slashes but keeps a lone "/", as MRI's basename/dirname see paths.
func rbTrimSlash(p string) string {
	t := strings.TrimRight(p, "/")
	if t == "" && p != "" {
		return "/"
	}
	return t
}

// rbBasename is File.basename; suffix ".*" drops any extension.
func rbBasename(path, suffix string) string {
	p := rbTrimSlash(path)
	if p == "/" || p == "" {
		return p
	}
	b := p[strings.LastIndexByte(p, '/')+1:]
	switch {
	case suffix == ".*":
		if e := rbExtname(b); e != "" {
			b = b[:len(b)-len(e)]
		}
	case suffix != "" && suffix != b && strings.HasSuffix(b, suffix):
		b = b[:len(b)-len(suffix)]
	}
	return b
}

func rbDirname(path string) string {
	p := rbTrimSlash(path)
	i := strings.LastIndexByte(p, '/')
	switch {
	case p == "/":
		return "/"
	case i < 0:
		return "."
	}
	if d := strings.TrimRight(p[:i], "/"); d != "" {
		if strings.HasPrefix(d, "//") { // MRI folds a leading run of / to one
			d = "/" + strings.TrimLeft(d, "/")
		}
		return d
	}
	return "/"
}

// rbExtname is File.extname: a leading dot is a hidden file, not an extension.
func rbExtname(path string) string {
	b := rbTrimSlash(path)
	b = b[strings.LastIndexByte(b, '/')+1:]
	i := strings.LastIndexByte(b, '.')
	if i <= 0 || strings.TrimLeft(b, ".") == "" {
		return ""
	}
	return b[i:]
}

// rbGlob is Dir.glob: `*`, `?`, `[set]` per path segment (a leading dot
// only matched by a dot), `**/` for any depth of non-hidden directories,
// `{a,b}` alternatives expanded in order, and a trailing `/` for
// directories only. Each directory's entries are visited in sorted order,
// a match before its subtree, as MRI's sort: true default gives. flags may
// hold File::FNM_DOTMATCH: wildcards and `**` take dot entries too.
func rbGlob(pattern string, flags int) []string {
	if strings.IndexByte(pattern, 0) >= 0 {
		panic(NewArgumentError(Ref(String("nul-separated glob pattern is deprecated"))))
	}
	var out []string
	for _, p := range rbBraces(pattern) {
		if p == "" {
			continue
		}
		dir, prefix := ".", ""
		if strings.HasPrefix(p, "/") {
			dir, prefix, p = "/", "/", strings.TrimLeft(p, "/")
		}
		segs := strings.Split(p, "/")
		for i, s := range segs {
			if s == "**" && i == len(segs)-1 {
				segs[i] = "*" // `a/**` is `a/*`
			}
		}
		rbGlobWalk(dir, prefix, segs, flags&rbFnmDotmatch, &out)
	}
	return out
}

// rbGlobDeep marks a walk that `**` recursed into, where MRI's DOTMATCH `*` no longer matches `.`.
const rbGlobDeep = 1 << 16

// rbBraces expands the first top-level {a,b} of p, recursively.
func rbBraces(p string) []string {
	depth, start := 0, -1
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '\\':
			i++
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth > 0 {
				continue
			}
			var alts []string
			d, last := 0, start+1
			for j := start + 1; j < i; j++ {
				switch p[j] {
				case '\\':
					j++
				case '{':
					d++
				case '}':
					d--
				case ',':
					if d == 0 {
						alts = append(alts, p[last:j])
						last = j + 1
					}
				}
			}
			alts = append(alts, p[last:i])
			out := make([]string, 0, len(alts))
			for _, a := range alts {
				out = append(out, rbBraces(p[:start]+a+p[i+1:])...)
			}
			return out
		}
	}
	return []string{p}
}

func rbGlobMatch(seg, name string, flags int) bool {
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(seg, ".") && flags&rbFnmDotmatch == 0 {
		return false
	}
	ok, _ := filepath.Match(seg, name)
	return ok
}

func rbGlobWalk(dir, prefix string, segs []string, flags int, out *[]string) {
	seg, rest := segs[0], segs[1:]
	if seg == "" {
		if len(rest) > 0 { // `a//b`: MRI keeps the doubled slash
			rbGlobWalk(dir, prefix+"/", rest, flags, out)
			return
		}
		*out = append(*out, prefix) // a trailing slash: dir itself, already known to be one
		return
	}
	if seg != "**" && !strings.ContainsAny(seg, "*?[\\") {
		p := filepath.Join(dir, seg)
		fi, err := os.Stat(p)
		if err != nil {
			if _, lerr := os.Lstat(p); lerr != nil {
				return
			}
			rbGlobStep(p, prefix+seg, false, rest, flags, out) // a symlink to nothing: it is there, and not a directory
			return
		}
		rbGlobStep(p, prefix+seg, fi.IsDir(), rest, flags, out)
		return
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	dotSelf := flags&rbFnmDotmatch != 0 && flags&rbGlobDeep == 0 && len(rest) == 0
	if seg != "**" && (strings.HasPrefix(seg, ".") || dotSelf) && rbGlobMatch(seg, ".", flags) { // a dot pattern matches `.` (never `..`), as MRI 3.1+; DOTMATCH's last `*` too, short of `**`'s depth
		rbGlobStep(dir, prefix+".", true, rest, flags, out)
	}
	if seg == "**" && flags&rbFnmDotmatch != 0 && flags&rbGlobDeep == 0 && len(rest) == 1 && rbGlobMatch(rest[0], ".", flags) {
		*out = append(*out, prefix+".") // `a/**/*` with DOTMATCH: a/. itself, at zero depth only
	}
	for _, e := range es { // ReadDir sorts by name
		name := e.Name()
		p := filepath.Join(dir, name)
		// The entry's type is the directory's own record of it (decision 473): only a symlink needs a stat, to learn
		// whether it leads to a directory. One that leads nowhere is still an entry, as in MRI.
		plainDir := e.IsDir() // `**` neither lists nor enters a symlink to a directory, as MRI
		isDir := plainDir
		if e.Type()&fs.ModeSymlink != 0 {
			fi, err := os.Stat(p)
			isDir = err == nil && fi.IsDir()
		}
		if seg == "**" {
			hidden := strings.HasPrefix(name, ".") && flags&rbFnmDotmatch == 0
			if rbGlobMatch(rest[0], name, flags) || rest[0] == "" && !hidden {
				if rest[0] == "" {
					if plainDir {
						*out = append(*out, prefix+name+"/")
					}
				} else {
					rbGlobStep(p, prefix+name, isDir, rest[1:], flags&^rbGlobDeep, out)
				}
			}
			if plainDir && !hidden {
				rbGlobWalk(p, prefix+name+"/", segs, flags|rbGlobDeep, out)
			}
			continue
		}
		if rbGlobMatch(seg, name, flags) {
			rbGlobStep(p, prefix+name, isDir, rest, flags&^rbGlobDeep, out)
		}
	}
}

// rbGlobStep continues past a matched entry: emit it, or descend.
func rbGlobStep(p, shown string, isDir bool, rest []string, flags int, out *[]string) {
	switch {
	case len(rest) == 0:
		*out = append(*out, shown)
	case isDir:
		if len(rest) == 2 && rest[0] == "**" && rest[1] == "" { // `a/**/` matches a/ itself, zero directories deep
			*out = append(*out, shown+"/")
		}
		rbGlobWalk(p, shown+"/", rest, flags, out)
	}
}

// rbSourceDir is Kernel#__dir__: the source file's absolute directory, resolved against the working directory as MRI
// does at load. Each source file calls it once, at its first `__dir__`, and keeps the answer (decision 472).
func rbSourceDir(name string) String {
	p, err := filepath.Abs(name)
	if err != nil {
		return String(filepath.Dir(name))
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return String(filepath.Dir(p))
}

// Stat_t names atime Atim (Linux, OpenBSD) or Atimespec (macOS, BSDs); zero stand-ins one level deeper let Go's shallowest-field rule pick the real one at build time (decision 138).
type rbAtimeNames struct{ Atim, Atimespec syscall.Timespec }

type rbAtimeDeeper struct{ rbAtimeNames }

type rbAtimeStat struct {
	*syscall.Stat_t
	rbAtimeDeeper
}

// Stat_t names ctime Ctim or Ctimespec, the same trick as atime's.
type rbCtimeNames struct{ Ctim, Ctimespec syscall.Timespec }

type rbCtimeDeeper struct{ rbCtimeNames }

type rbCtimeStat struct {
	*syscall.Stat_t
	rbCtimeDeeper
}

// rbCtime is fi's status-change time, or the modification time without a Stat_t.
func rbCtime(fi os.FileInfo) time.Time {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.ModTime()
	}
	x := rbCtimeStat{Stat_t: st}
	s1, n1 := x.Ctim.Unix()
	s2, n2 := x.Ctimespec.Unix()
	return time.Unix(s1+s2, n1+n2)
}

// rbAtime is fi's access time, or the modification time without a Stat_t; ponytail: js/wasip1's Stat_t has neither name (Atime instead), so it reads as the epoch there.
func rbAtime(fi os.FileInfo) time.Time {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.ModTime()
	}
	x := rbAtimeStat{Stat_t: st}
	s1, n1 := x.Atim.Unix()
	s2, n2 := x.Atimespec.Unix()
	return time.Unix(s1+s2, n1+n2)
}

// rbGetsFrom is IO#gets with a separator (nil: the rest; "": a paragraph, its trailing newlines skipped) and a byte
// limit (-1: none) that never splits a UTF-8 character; lineno counts a line the limit did not cut (decision 181).
func rbGetsFrom(r *bufio.Reader, sep *String, limit int, lineno *int) *String {
	if limit == 0 {
		return Ref(String(""))
	}
	para := sep != nil && *sep == ""
	if para {
		for {
			b, err := r.ReadByte()
			if err != nil {
				return nil
			}
			if b != '\n' {
				_ = r.UnreadByte()
				break
			}
		}
	}
	var want []byte
	switch {
	case para:
		want = []byte("\n\n")
	case sep != nil:
		want = []byte(*sep)
	}
	var out []byte
	cut := false
	if limit < 0 {
		out = rbReadUntil(r, want)
	}
	for limit > 0 {
		if limit > 0 && len(out) >= limit {
			for { // finish the character the limit fell inside
				b, err := r.Peek(1)
				if err != nil || b[0]&0xC0 != 0x80 {
					break
				}
				c, _ := r.ReadByte()
				out = append(out, c)
			}
			cut = len(want) == 0 || !bytes.HasSuffix(out, want)
			break
		}
		b, err := r.ReadByte()
		if err != nil {
			break
		}
		out = append(out, b)
		if len(want) > 0 && bytes.HasSuffix(out, want) {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	if para {
		for {
			b, err := r.ReadByte()
			if err != nil {
				break
			}
			if b != '\n' {
				_ = r.UnreadByte()
				break
			}
		}
	}
	if !cut {
		*lineno++
		rbNoteLineNo(*lineno)
	}
	return Ref(String(out))
}

// rbLineNo is $.: the line number of the last line an IO's gets read (decision 425). It is stored only when the program
// reads $. (rbTrackLineNo, set by an init the compiler emits then), so a program that never does pays one bool test;
// atomic, as threads may read lines while another reads $..
var (
	rbLineNo      atomic.Int64
	rbTrackLineNo bool
)

func rbNoteLineNo(n int) {
	if rbTrackLineNo {
		rbLineNo.Store(int64(n))
	}
}

// rbStatAccess is whether the effective user may do mode bit want (4 read, 2 write, 1 execute) on fi: root may read and
// write anything, and execute what any execute bit allows.
func rbStatAccess(fi os.FileInfo, want uint32) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	mode := uint32(st.Mode)
	if os.Geteuid() == 0 {
		return want != 1 || mode&0o111 != 0 || fi.IsDir()
	}
	switch {
	case int(st.Uid) == os.Geteuid():
		mode >>= 6
	case rbInGroups(int(st.Gid)):
		mode >>= 3
	}
	return mode&want != 0
}

// rbStatGroupOwned is whether fi's group is the effective group or one of the process's groups.
func rbStatGroupOwned(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && rbInGroups(int(st.Gid))
}

func rbInGroups(gid int) bool {
	if gid == os.Getegid() {
		return true
	}
	gs, err := os.Getgroups()
	return err == nil && slices.Contains(gs, gid)
}
