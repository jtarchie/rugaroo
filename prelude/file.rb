# rbs_inline: enabled

# @go_type struct { f *os.File; r *bufio.Reader; w *bufio.Writer; path string; sync bool; lineno int; enc rbIOEnc; conv func(string) string }
class File < Object
  include IOWritable
  include IOReadable

  # Modes "r", "w", "a", their "+" forms, "b", and ":ext[:int]" encodings (decision 136); failures raise MRI's Errno classes (decision 62).
  # MRI's forms: mode a String, Integer flags or the `mode:` keyword; perm for a created file; `flags:` or'd in.
  # ponytail: other options (encoding:, binmode:, newline:) are accepted and ignored; honor them when a program needs one.
  #: (String, ?(String | Integer | nil), ?Integer?, **untyped) -> File
  def self.new(path, mode = nil, perm = nil, **opts)
    m = mode.nil? ? opts.fetch(:mode, "r") : mode
    raise ArgumentError, "newline decorator with binary mode" if opts[:newline] && (opts[:binmode] || (m.is_a?(String) && m.split(":").first.to_s.include?("b")))

    extra = __int(opts.fetch(:flags, 0))
    return __new_flags(path, m | extra, perm || 0o666) if m.is_a?(Integer)

    __new_mode(path, m.to_s, extra, perm || 0o666)
  end

  #: (untyped) -> Integer
  def self.__int(x) = %x{ return rbAs[Integer](x, "Integer") }

  #: (Integer) -> Integer
  def self.__o(bit) = %x{
    v := []int{syscall.O_RDONLY, syscall.O_WRONLY, syscall.O_RDWR, syscall.O_CREAT, syscall.O_EXCL, syscall.O_TRUNC, syscall.O_APPEND, syscall.O_NONBLOCK, syscall.O_NOCTTY, syscall.O_SYNC, syscall.O_NOFOLLOW}
    return Integer(v[bit])
  }

  RDONLY = __o(0) #: Integer
  WRONLY = __o(1) #: Integer
  RDWR = __o(2) #: Integer
  CREAT = __o(3) #: Integer
  EXCL = __o(4) #: Integer
  TRUNC = __o(5) #: Integer
  APPEND = __o(6) #: Integer
  NONBLOCK = __o(7) #: Integer
  NOCTTY = __o(8) #: Integer
  SYNC = __o(9) #: Integer
  NOFOLLOW = __o(10) #: Integer
  BINARY = 0 #: Integer
  SHARE_DELETE = 0 #: Integer

  # Integer open flags, as MRI's rb_sysopen; reading and writing follow the access bits.
  #: (String, Integer, Integer) -> File
  def self.__new_flags(path, flag, perm) = %x{
    f, err := os.OpenFile(string(path), int(flag), os.FileMode(perm)) //nolint:gosec // the program's own flags and perm
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    out := &File{f: f, path: string(path)}
    acc := int(flag) & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR)
    if acc != syscall.O_WRONLY {
      out.r = bufio.NewReader(f)
    }
    if acc != syscall.O_RDONLY {
      out.w = bufio.NewWriter(f)
    }
    return out
  }

  #: (String, String, Integer, Integer) -> File
  def self.__new_mode(path, mode, extra, perm) = %x{
    flags := map[string]int{
      "r": os.O_RDONLY, "r+": os.O_RDWR,
      "w": os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w+": os.O_RDWR | os.O_CREATE | os.O_TRUNC,
      "a": os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a+": os.O_RDWR | os.O_CREATE | os.O_APPEND,
    }
    access, spec, _ := strings.Cut(string(mode), ":")
    bin := strings.ContainsRune(access, 'b')
    access = strings.NewReplacer("b", "", "t", "").Replace(access)
    excl := strings.HasPrefix(access, "w") && strings.HasSuffix(access, "x") // "wx": O_EXCL, only with w
    if excl {
      access = strings.TrimSuffix(access, "x")
    }
    flag, ok := flags[access]
    if !ok {
      panic(NewArgumentError(Ref("invalid access mode " + mode)))
    }
    if excl {
      flag |= os.O_EXCL
    }
    var withEnc func(*File)
    if spec != "" {
      hook := rbFileEncHook.Load()
      if hook == nil {
        panic(NewNotImplementedError(Ref(String("rugaroo: a File mode naming encodings must be passed to File.open, File.new or CSV.open directly (decision 136)"))))
      }
      withEnc = (*hook)(access, spec, bin)
    }
    f, err := os.OpenFile(string(path), flag|int(extra), os.FileMode(perm)) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    out := &File{f: f, path: string(path)}
    if bin {
      out.enc = rbIOEnc{ext: "ASCII-8BIT", bin: true}
    }
    if access != "w" && access != "a" {
      out.r = bufio.NewReader(f)
    }
    if access != "r" {
      out.w = bufio.NewWriter(f)
    }
    if withEnc != nil {
      withEnc(out)
    }
    return out
  }

  SEPARATOR = "/" #: String
  FNM_NOESCAPE = 1 #: Integer
  FNM_PATHNAME = 2 #: Integer
  FNM_DOTMATCH = 4 #: Integer
  FNM_CASEFOLD = 8 #: Integer
  FNM_EXTGLOB = 16 #: Integer
  FNM_SYSCASE = 0 #: Integer
  ALT_SEPARATOR = nil #: String?
  PATH_SEPARATOR = ":" #: String

  # Blockless File.open is File.new (decision 12's `__<name>_enum` stands for "no block").
  #: (String, ?(String | Integer | nil), ?Integer?, **untyped) -> File
  def self.__open_enum(path, mode = nil, perm = nil, **opts) = File.new(path, mode, perm, **opts)

  #: (String) -> Time
  def self.mtime(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_mtime", string(path)))
    }
    return &Time{t: fi.ModTime()}
  }

  #: (String) -> Time
  def self.atime(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_atime", string(path)))
    }
    return &Time{t: rbAtime(fi)}
  }

  # The permission bits when others may write, else nil.
  #: (String) -> Integer?
  def self.world_writable?(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil || fi.Mode().Perm()&0o002 == 0 {
      return rbOpt[Integer]{}
    }
    v := Integer(fi.Mode().Perm())
    return rbSome(v)
  }

  #: (String) -> Time
  def self.ctime(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_ctime", string(path)))
    }
    return &Time{t: rbCtime(fi)}
  }

  LOCK_SH = 1 #: Integer
  LOCK_EX = 2 #: Integer
  LOCK_NB = 4 #: Integer
  LOCK_UN = 8 #: Integer

  # Sets access and modification times; the count of paths, as MRI's.
  #: (Time, Time, *String) -> Integer
  def self.utime(atime, mtime, *paths) = %x{
    for _, p := range rest_ {
      if err := os.Chtimes(string(p), atime.t, mtime.t); err != nil {
        panic(rbSysErr(err, "rb_file_s_utime", string(p)))
      }
    }
    return Integer(len(rest_))
  }

  #: (String) -> File::Stat
  def self.stat(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_stat", string(path)))
    }
    return &File_Stat{fi: fi}
  }

  #: [T] (String, ?(String | Integer | nil), ?Integer?, **untyped) { (File) -> T } -> T
  def self.open(path, mode = nil, perm = nil, **opts)
    f = File.new(path, mode, perm, **opts)
    begin
      yield f
    ensure
      f.close
    end
  end

  # read(path, length, offset): at most length bytes from offset ("" past the end, where MRI answers nil: ponytail, a String? result would make every File.read optional);
  # Strings are bytes without a tag (decision 136), so encoding: "ext:int" transcodes and an external one alone keeps the bytes; mode: is checked only.
  #: (String, ?Integer?, ?Integer?, **untyped) -> String
  def self.read(path, length = nil, offset = nil, **opts)
    raise ArgumentError, "negative offset #{offset} given" if offset && offset < 0
    raise ArgumentError, "negative length #{length} given" if length && length < 0

    mode = opts[:mode]
    raise IOError, "not opened for reading" if !opts.key?(:open_args) && mode.is_a?(String) && mode.start_with?("w", "a") && !mode.include?("+")

    data = length.nil? && offset.nil? ? __read_all(path) : __read_at(path, length || -1, offset || 0)
    enc = opts[:encoding]
    if enc.is_a?(String) && enc.include?(":")
      ext, int = enc.split(":", 2)
      return data.encode(int.to_s, ext.to_s.delete_prefix("BOM|").delete_prefix("bom|"))
    end
    data
  end

  #: (String, Integer, Integer) -> String
  def self.__read_at(path, length, offset) = %x{
    if length < -1 {
      panic(NewArgumentError(Ref(String(fmt.Sprintf("negative length %d given", length)))))
    }
    f, err := os.Open(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    defer f.Close()
    if offset < 0 {
      panic(rbSysErr(syscall.EINVAL, "rb_io_seek", string(path)))
    }
    if _, err = f.Seek(int64(offset), io.SeekStart); err != nil {
      panic(rbSysErr(err, "rb_io_seek", string(path)))
    }
    var b []byte
    if length < 0 {
      b, _ = io.ReadAll(f)
    } else {
      b = make([]byte, length)
      n, _ := io.ReadFull(f, b)
      b = b[:n]
    }
    return String(b)
  }

  #: (String) -> String
  def self.__read_all(path) = %x{
    b, err := os.ReadFile(string(path))
    if err != nil {
      fn := "rb_sysopen"
      if errors.Is(err, syscall.EISDIR) {
        fn = "io_fread"
      }
      panic(rbSysErr(err, fn, string(path)))
    }
    return String(b)
  }

  #: (String, untyped) -> Integer
  def self.write(path, data) = %x{
    s := string(rbToS(data))
    err := os.WriteFile(string(path), []byte(s), 0o666) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_sysopen", string(path)))
    }
    return Integer(len(s))
  }

  # @binary
  #: (String) -> String
  def self.binread(path) = read(path)

  #: (String, untyped) -> Integer
  def self.binwrite(path, data) = write(path, data)

  #: (String, ?chomp: bool) -> Array[String]
  def self.readlines(path, chomp: false)
    lines = File.open(path) { |f| f.readlines }
    chomp ? lines.map(&:chomp) : lines
  end

  #: (String) { (String) -> void } -> void
  def self.foreach(path)
    f = File.new(path)
    begin
      while (line = f.gets)
        yield line
      end
    ensure
      f.close
    end
  end

  #: (String) -> bool
  def self.exist?(path) = %x{
    _, err := os.Stat(string(path))
    return Boolean(err == nil)
  }

  #: (String) -> bool
  def self.file?(path) = %x{
    fi, err := os.Stat(string(path))
    return Boolean(err == nil && fi.Mode().IsRegular())
  }

  # The same file (device and inode, symlinks followed), as MRI's; false when either is missing.
  #: (String | Pathname | File, String | Pathname | File) -> bool
  def self.identical?(a, b)
    sa = __identical_stat(a)
    sb = __identical_stat(b)
    return false unless sa && sb

    sa.dev == sb.dev && sa.ino == sb.ino
  end

  #: (String | Pathname | File) -> File::Stat?
  def self.__identical_stat(x)
    return x.stat if x.is_a?(File)

    path = x.to_s
    File.exist?(path) ? File.stat(path) : nil
  end

  #: (String) -> bool
  def self.symlink?(path) = %x{
    fi, err := os.Lstat(string(path))
    return Boolean(err == nil && fi.Mode()&os.ModeSymlink != 0)
  }

  #: (String) -> bool
  def self.directory?(path) = %x{
    fi, err := os.Stat(string(path))
    return Boolean(err == nil && fi.IsDir())
  }

  NULL = "/dev/null" #: String

  #: (String) -> bool
  def self.zero?(path) = exist?(path) && size(path) == 0

  #: (String) -> bool
  def self.empty?(path) = zero?(path)

  #: (String) -> bool
  def self.readable?(path) = %x{ Boolean(syscall.Access(string(path), 4) == nil) }

  #: (String) -> bool
  def self.writable?(path) = %x{ Boolean(syscall.Access(string(path), 2) == nil) }

  #: (String) -> bool
  def self.executable?(path) = %x{ Boolean(syscall.Access(string(path), 1) == nil) }

  #: (String) -> bool
  def self.grpowned?(path) = %x{
    fi, err := os.Stat(string(path))
    return Boolean(err == nil && rbStatGroupOwned(fi))
  }

  #: (String) -> bool
  def self.owned?(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      return false
    }
    st, ok := fi.Sys().(*syscall.Stat_t)
    return Boolean(ok && int(st.Uid) == os.Geteuid())
  }

  #: (String) -> String
  def self.realpath(path) = %x{
    p, err := filepath.Abs(string(path))
    if err == nil {
      p, err = filepath.EvalSymlinks(p)
    }
    if err != nil {
      panic(rbSysErr(err, "rb_check_realpath_internal", string(path)))
    }
    return String(p)
  }

  #: (String, String?) -> String
  def self.__realpath_2(path, dir)
    return realpath(path) if dir.nil? || path.start_with?("/")

    realpath(File.join(dir, path))
  end

  #: (String) -> [String, String]
  def self.split(path) = [dirname(path), basename(path)]

  #: (String) -> String
  def self.ftype(path) = lstat(path).ftype

  #: (String) -> File::Stat
  def self.lstat(path) = %x{
    fi, err := os.Lstat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_lstat", string(path)))
    }
    return &File_Stat{fi: fi}
  }

  #: (String) -> String
  def self.readlink(path) = %x{
    s, err := os.Readlink(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_readlink", string(path)))
    }
    return String(s)
  }

  #: (Integer, *String) -> Integer
  def self.chmod(mode, *paths) = %x{
    fm := fs.FileMode(mode & 0o777)
    if mode&0o4000 != 0 {
      fm |= fs.ModeSetuid
    }
    if mode&0o2000 != 0 {
      fm |= fs.ModeSetgid
    }
    if mode&0o1000 != 0 {
      fm |= fs.ModeSticky
    }
    for _, p := range rest_ {
      if err := os.Chmod(string(p), fm); err != nil {
        panic(rbSysErr(err, "apply2files", string(p)))
      }
    }
    return Integer(len(rest_))
  }

  # nil leaves the owner or the group as it is (chown(2)'s -1).
  #: (Integer?, Integer?, *String) -> Integer
  def self.chown(owner, group, *paths) = %x{
    uid, gid := -1, -1
    if owner.ok {
      uid = int(owner.v)
    }
    if group.ok {
      gid = int(group.v)
    }
    for _, p := range rest_ {
      if err := os.Chown(string(p), uid, gid); err != nil {
        panic(rbSysErr(err, "apply2files", string(p)))
      }
    }
    return Integer(len(rest_))
  }

  #: (String, String) -> Integer
  def self.symlink(old, dst) = %x{
    if err := os.Symlink(string(old), string(dst)); err != nil {
      panic(rbSysErr(err, "rb_file_s_symlink", "("+string(old)+", "+string(dst)+")"))
    }
    return 0
  }

  #: (String, String) -> Integer
  def self.link(old, dst) = %x{
    if err := os.Link(string(old), string(dst)); err != nil {
      panic(rbSysErr(err, "rb_file_s_link", "("+string(old)+", "+string(dst)+")"))
    }
    return 0
  }

  #: (String, Integer) -> Integer
  def self.truncate(path, n) = %x{
    if err := os.Truncate(string(path), int64(n)); err != nil {
      panic(rbSysErr(err, "rb_file_s_truncate", string(path)))
    }
    return 0
  }

  #: (String) -> Integer
  def self.size(path) = %x{
    fi, err := os.Stat(string(path))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_size", string(path)))
    }
    return Integer(fi.Size())
  }

  #: (String) -> Integer?
  def self.size?(path) = zero?(path) || !exist?(path) ? nil : size(path)

  #: (String) -> Integer
  def self.delete(path) = %x{
    if err := syscall.Unlink(string(path)); err != nil { // not os.Remove, which falls back to rmdir
      panic(rbSysErr(err, "apply2files", string(path)))
    }
    return 1
  }

  #: (String) -> Integer
  def self.unlink(path) = File.delete(path)

  #: (String, String) -> Integer
  def self.rename(from, to) = %x{
    err := os.Rename(string(from), string(to))
    if err != nil {
      panic(rbSysErr(err, "rb_file_s_rename", "("+string(from)+", "+string(to)+")"))
    }
    return 0
  }

  #: (String, ?String) -> String
  def self.basename(path, suffix = "") = %x{ String(rbBasename(string(path), string(suffix))) }

  #: (String) -> String
  def self.dirname(path) = %x{ String(rbDirname(string(path))) }

  #: (String) -> String
  def self.extname(path) = %x{ String(rbExtname(string(path))) }

  # Arrays (nested too) are flattened into the parts, as MRI does.
  #: (*(String | Array[untyped])) -> String
  def self.join(*parts) = %x{
    if len(rest_) == 2 { // two Strings, the common call: no flattening, one allocation
      a, aok := rest_[0].(String)
      b, bok := rest_[1].(String)
      if aok && bok && a != "" && b != "" && a[len(a)-1] != '/' && b[0] != '/' && strings.IndexByte(string(a), 0) < 0 && strings.IndexByte(string(b), 0) < 0 {
        return a + "/" + b
      }
    }
    flat := &Array[any]{}
    rbFlattenInto(flat, rest_, -1)
    out := ""
    for i, p := range flat.s {
      str, ok := p.(String)
      if !ok {
        panic(NewTypeError(Ref(String("no implicit conversion of " + rbClassName(p) + " into String"))))
      }
      s := string(str)
      if strings.IndexByte(s, 0) >= 0 {
        panic(NewArgumentError(Ref(String("string contains null byte"))))
      }
      switch { // MRI keeps the right part's separators and drops the left's
      case i == 0:
        out = s
      case strings.HasPrefix(s, "/"):
        out = strings.TrimRight(out, "/") + s
      case strings.HasSuffix(out, "/"):
        out += s
      default:
        out += "/" + s
      }
    }
    return String(out)
  }

  #: (String) -> String
  def self.expand_path(path) = %x{ String(rbExpandPath(string(path))) }

  # A relative path against dir (itself expanded, so it may start with ~); nil dir is the working directory.
  #: (String, String?) -> String
  def self.__expand_path_2(path, dir)
    return expand_path(path) unless dir
    return expand_path(path) if path.start_with?("/", "~")

    expand_path(File.join(expand_path(dir), path))
  end

  #: (String | Pathname, String | Pathname, ?Integer) -> bool
  def self.fnmatch(pattern, path, flags = 0) = %x{ Boolean(rbFnmatch(string(rbToS(pattern)), string(rbToS(path)), int(flags))) }

  #: (String | Pathname, String | Pathname, ?Integer) -> bool
  def self.fnmatch?(pattern, path, flags = 0) = fnmatch(pattern, path, flags)

  # expand_path without `~`: a leading ~ is a directory name like any other.
  #: (String, ?String?) -> String
  def self.absolute_path(path, dir = nil)
    return expand_path(path) if path.start_with?("/")

    expand_path(File.join(dir ? absolute_path(dir) : Dir.pwd, path))
  end

  #: (String) -> bool
  def self.absolute_path?(path) = %x{ Boolean(strings.HasPrefix(string(path), "/")) }

  #: () -> String
  def path = %x{ String(self.path) }

  # A reading File without one is default_external's, a writing one has none, as in MRI.
  #: () -> Encoding?
  def external_encoding
    e = __ext
    return Encoding.find(e) unless e.empty?

    __readable? ? Encoding.default_external : nil
  end

  #: () -> Encoding?
  def internal_encoding
    e = __int
    e.empty? ? nil : Encoding.find(e)
  end

  #: (untyped, ?untyped) -> File
  def set_encoding(ext, intern = nil) = %x{
    e := rbSetEncoding(ext, intern)
    e.bin, e.raw, e.unread, e.restart = self.enc.bin, self.enc.raw, self.enc.unread, self.enc.restart
    same := e.ext == self.enc.ext && e.intern == self.enc.intern // keep the transcoder and what it has read ahead
    self.enc = e
    if self.r != nil && !same {
      self.r = self.enc.readerFor(self.r)
    }
    self.conv = self.enc.writeConv
    return self
  }

  # Binary from here on: no conversion either way, external encoding ASCII-8BIT.
  #: () -> File
  def binmode = %x{
    self.rbOpen()
    self.enc = rbIOEnc{ext: "ASCII-8BIT", bin: true, raw: self.enc.raw, unread: self.enc.unread}
    if self.r != nil {
      self.r = self.enc.readerFor(self.r)
    }
    self.conv = nil
    return self
  }

  #: () -> bool
  def binmode? = %x{
    self.rbOpen()
    return Boolean(self.enc.bin)
  }

  #: () -> String
  def __ext = %x{ String(self.enc.ext) }

  #: () -> String
  def __int = %x{ String(self.enc.intern) }

  #: () -> bool
  def __readable? = %x{ Boolean(self.r != nil) }

  #: (untyped) -> Integer
  def write(x) = %x{
    self.rbOpen()
    s := string(rbToS(x))
    if s == "" { // MRI checks writability only when there is something to write
      return 0
    }
    if self.w == nil {
      panic(NewIOError(Ref[String]("not opened for writing")))
    }
    if self.conv != nil {
      s = self.conv(s)
    }
    _, _ = self.w.WriteString(s)
    if self.sync {
      _ = self.w.Flush()
    }
    return Integer(len(s))
  }

  #: () -> bool
  def sync = %x{
    self.rbOpen()
    return Boolean(self.sync)
  }

  #: (bool) -> bool
  def sync=(on)
    %x{
    self.rbOpen()
    self.sync = bool(on)
    if self.sync && self.w != nil {
      _ = self.w.Flush()
    }
    return on
    }
  end

  #: (untyped) -> File
  def <<(x)
    write(x)
    self
  end

  #: () -> String?
  def gets = %x{
    self.rbReadable()
    line, err := self.r.ReadString('\\n')
    if line == "" && err != nil {
      return nil
    }
    self.lineno++
    rbNoteLineNo(self.lineno)
    s := String(line)
    return &s
  }

  # The count of lines gets has read (each_line and readline too); rewind sets it back to 0.
  #: () -> Integer
  def lineno = %x{
    self.rbReadable()
    return Integer(self.lineno)
  }

  #: (Integer) -> Integer
  def lineno=(n)
    %x{
    self.rbReadable()
    self.lineno = rbCInt(n)
    return n
    }
  end

  #: () -> String
  def read = %x{
    self.rbReadable()
    return rbReadRest(self.r, self.f)
  }

  # read(length): at most length bytes, nil at the end (but "" for 0); nil length reads the rest
  # @binary
  #: (Integer?) -> String?
  def __read_1(length) = %x{
    self.rbReadable()
    if !length.ok {
      s := rbReadRest(self.r, self.f)
      return &s
    }
    if length.v < 0 {
      panic(NewArgumentError(Ref(String(fmt.Sprintf("negative length %d given", length.v)))))
    }
    if length.v == 0 {
      s := String("")
      return &s
    }
    buf := make([]byte, length.v)
    n, _ := io.ReadFull(self.r, buf)
    if n == 0 {
      return nil
    }
    s := String(buf[:n])
    return &s
  }

  # gets(sep), gets(limit), gets(sep, limit) (decision 181): "" is paragraph mode, nil the rest of the file
  #: (String | Integer | nil) -> String?
  def __gets_1(arg) = %x{
    switch v := arg.(type) {
    case Integer:
      return rbGetsFrom(self.r, Ref(String("\\n")), int(v), &self.lineno)
    case String:
      return rbGetsFrom(self.r, &v, -1, &self.lineno)
    }
    return rbGetsFrom(self.r, nil, -1, &self.lineno)
  }

  #: (String?, Integer) -> String?
  def __gets_2(sep, limit) = %x{ return rbGetsFrom(self.r, sep, int(limit), &self.lineno) }

  #: () -> Integer
  def fileno = %x{
    if self.f == nil {
      panic(NewIOError(Ref[String]("closed stream")))
    }
    return Integer(self.f.Fd())
  }

  #: () -> File::Stat
  def stat = %x{
    if self.f == nil {
      panic(NewIOError(Ref[String]("closed stream")))
    }
    fi, err := self.f.Stat()
    if err != nil {
      panic(rbSysErr(err, "rb_io_stat", self.path))
    }
    return &File_Stat{fi: fi}
  }

  #: () -> bool
  def eof? = %x{
    self.rbReadable()
    _, err := self.r.Peek(1)
    return Boolean(err != nil)
  }

  #: () -> String?
  def getc = %x{
    self.rbReadable()
    return rbGetc(self.r)
  }

  #: () -> Integer?
  def getbyte = %x{
    self.rbReadable()
    return rbOptFromPtr(rbGetbyte(self.r))
  }

  #: () -> String
  def readchar = %x{
    self.rbReadable()
    return *rbEOF(rbGetc(self.r))
  }

  #: () -> Integer
  def readbyte = %x{
    self.rbReadable()
    return *rbEOF(rbGetbyte(self.r))
  }

  #: (String) -> nil
  def ungetc(s) = %x{
    self.rbReadable()
    rbUngetc(&self.r, string(s))
  }

  #: () { (String) -> void } -> void
  def each_char
    while (c = getc)
      yield c
    end
  end

  #: () { (Integer) -> void } -> void
  def each_byte
    while (b = getbyte)
      yield b
    end
  end

  #: () -> Integer
  def pos = %x{
    off, err := self.f.Seek(0, io.SeekCurrent)
    if err != nil {
      panic(rbSysErr(err, "rb_io_tell", self.path))
    }
    if self.r != nil {
      off -= int64(self.r.Buffered())
    }
    if self.w != nil {
      off += int64(self.w.Buffered())
    }
    return Integer(off)
  }

  #: () -> Integer
  def tell = pos

  # fcntl(2) with an integer argument (Fcntl::F_SETFD, Fcntl::FD_CLOEXEC); its result, as MRI's.
  #: (Integer, ?Integer) -> Integer
  def fcntl(cmd, arg = 0) = %x{
    if self.f == nil {
      panic(NewIOError(Ref[String]("closed stream")))
    }
    r, _, errno := syscall.Syscall(syscall.SYS_FCNTL, self.f.Fd(), uintptr(cmd), uintptr(arg))
    if errno != 0 {
      panic(rbSysErr(errno, "rb_io_fcntl", self.path))
    }
    return Integer(r)
  }

  # LOCK_SH, LOCK_EX or LOCK_UN, with LOCK_NB not to wait: 0, or false when LOCK_NB finds it held.
  #: (Integer) -> untyped
  def flock(op) = %x{
    if self.w != nil {
      _ = self.w.Flush()
    }
    if err := syscall.Flock(int(self.f.Fd()), int(op)); err != nil {
      if errors.Is(err, syscall.EWOULDBLOCK) {
        return Boolean(false)
      }
      panic(rbSysErr(err, "rb_file_flock", self.path))
    }
    return Integer(0)
  }

  #: (Integer, ?Integer) -> Integer
  def seek(offset, whence = 0) = %x{
    if self.w != nil {
      _ = self.w.Flush()
    }
    if whence == 1 && self.r != nil {
      offset -= Integer(self.r.Buffered())
    }
    if _, err := self.f.Seek(int64(offset), int(whence)); err != nil {
      panic(rbSysErr(err, "rb_io_seek", self.path))
    }
    if self.enc.raw != nil { // the file's own reader under any transcoder
      self.enc.raw.Reset(self.f)
    }
    switch {
    case self.enc.restart != nil: // what the transcoder read ahead is stale
      self.r = self.enc.restart()
    case self.enc.raw != nil:
      self.r = self.enc.raw
    case self.r != nil:
      self.r.Reset(self.f)
    }
    return 0
  }

  #: () -> Integer
  def rewind
    self.lineno = 0
    seek(0)
  end

  #: (Integer) -> Integer
  def pos=(offset)
    seek(offset)
    offset
  end

  #: () -> File
  def flush = %x{
    self.rbOpen()
    if self.w != nil {
      _ = self.w.Flush()
    }
    return self
  }

  #: () -> self
  def dup = %x{
    self.rbOpen()
    return rbDup(self)
  }

  #: () -> nil
  def close = %x{
    if self.f == nil {
      return
    }
    if self.w != nil {
      _ = self.w.Flush()
    }
    _ = self.f.Close()
    self.f = nil
  }

  #: () -> bool
  def closed? = %x{ Boolean(self.f == nil) }

  #: () -> String
  def inspect = %x{
    s := "#<File:" + self.path
    if self.f == nil {
      s += " (closed)"
    }
    return String(s + ">")
  }
end

# File.stat's result: a snapshot of os.Stat.
# @go_type struct { fi os.FileInfo }
class File::Stat < Object
  #: () -> Integer
  def size = %x{ Integer(self.fi.Size()) }

  #: () -> Time
  def mtime = %x{ &Time{t: self.fi.ModTime()} }

  #: () -> Time
  def atime = %x{ &Time{t: rbAtime(self.fi)} }

  #: () -> Time
  def ctime = %x{ &Time{t: rbCtime(self.fi)} }

  # The mode bits as the effective user sees them (root: any), as MRI's Stat#writable? and friends.
  #: () -> bool
  def writable? = %x{ Boolean(rbStatAccess(self.fi, 2)) }

  #: () -> bool
  def readable? = %x{ Boolean(rbStatAccess(self.fi, 4)) }

  #: () -> bool
  def executable? = %x{ Boolean(rbStatAccess(self.fi, 1)) }

  #: () -> Integer
  def uid = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Uid)
    }
    return 0
  }

  #: () -> Integer
  def gid = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Gid)
    }
    return 0
  }

  #: () -> bool
  def grpowned? = %x{ Boolean(rbStatGroupOwned(self.fi)) }

  #: () -> bool
  def file? = %x{ Boolean(self.fi.Mode().IsRegular()) }

  #: () -> bool
  def directory? = %x{ Boolean(self.fi.IsDir()) }

  #: () -> bool
  def zero? = size == 0

  #: () -> bool
  def symlink? = %x{ Boolean(self.fi.Mode()&fs.ModeSymlink != 0) }

  #: () -> String
  def ftype = %x{
    m := self.fi.Mode()
    switch {
    case m.IsRegular():
      return "file"
    case m.IsDir():
      return "directory"
    case m&fs.ModeSymlink != 0:
      return "link"
    case m&fs.ModeCharDevice != 0:
      return "characterSpecial"
    case m&fs.ModeDevice != 0:
      return "blockSpecial"
    case m&fs.ModeNamedPipe != 0:
      return "fifo"
    case m&fs.ModeSocket != 0:
      return "socket"
    }
    return "unknown"
  }

  # Homebrew's LockFile compares these to see that the file it locked is still the one on disk.
  #: () -> Integer
  def ino = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Ino)
    }
    return 0
  }

  #: () -> Integer
  def dev = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Dev)
    }
    return 0
  }

  # st_mode, file type bits included (0100644 for a plain file), as MRI's.
  #: () -> Integer
  def mode = %x{
    if st, ok := self.fi.Sys().(*syscall.Stat_t); ok {
      return Integer(st.Mode)
    }
    return Integer(self.fi.Mode().Perm())
  }
end

# FileTest: File's predicates as module functions.
module FileTest
  #: (String) -> bool
  def self.exist?(path) = File.exist?(path)

  #: (String) -> bool
  def self.file?(path) = File.file?(path)

  #: (String) -> bool
  def self.directory?(path) = File.directory?(path)

  #: (String) -> bool
  def self.symlink?(path) = File.symlink?(path)

  #: (String) -> bool
  def self.zero?(path) = File.zero?(path)

  #: (String) -> bool
  def self.empty?(path) = File.empty?(path)

  #: (String) -> Integer
  def self.size(path) = File.size(path)

  #: (String) -> Integer?
  def self.size?(path) = File.size?(path)

  #: (String) -> bool
  def self.readable?(path) = File.readable?(path)

  #: (String) -> bool
  def self.writable?(path) = File.writable?(path)

  #: (String) -> bool
  def self.executable?(path) = File.executable?(path)

  #: (String) -> bool
  def self.owned?(path) = File.owned?(path)
end

class Dir < Object
  attr_reader :path #: String

  #: (String) -> void
  def initialize(path)
    raise Errno::ENOTDIR, "Not a directory @ dir_initialize - #{path}" if File.exist?(path) && !File.directory?(path)

    Dir.children(path) # ENOENT, as MRI's dir_initialize
    @path = path
  end

  #: [T] (String) { (Dir) -> T } -> T
  def self.open(path)
    d = Dir.new(path)
    yield d
  end

  #: (String) -> Dir
  def self.__open_enum(path) = Dir.new(path)

  # ponytail: sorted, as Dir.children; MRI walks readdir order.
  #: () { (String) -> void } -> void
  def each
    Dir.entries(path).each { |e| yield e }
  end

  #: () -> Array[String]
  def children = Dir.children(path)

  #: () -> Array[String]
  def entries = Dir.entries(path)

  #: () { (String) -> void } -> void
  def each_child
    Dir.children(path).each { |e| yield e }
  end

  #: () -> String
  def to_path = path

  #: () -> nil
  def close = nil

  #: () -> String
  def inspect = "#<Dir:#{path}>"

  # Dir.home(user): that user's home, from the user database; MRI's ArgumentError for one it does not know.
  #: (String?) -> String
  def self.__home_1(name) = %x{
    if name == nil {
      if h, ok := os.LookupEnv("HOME"); ok {
        return String(h)
      }
      return String(rbHome(""))
    }
    u, err := user.Lookup(string(*name))
    if err != nil {
      panic(NewArgumentError(Ref(String("user " + string(*name) + " doesn't exist"))))
    }
    return String(u.HomeDir)
  }

  #: () -> String
  def self.home = %x{
    if h, ok := os.LookupEnv("HOME"); ok {
      return String(h)
    }
    return String(rbHome(""))
  }

  #: () -> String
  def self.getwd = pwd

  #: (String) -> bool
  def self.empty?(path) = %x{
    es, err := os.ReadDir(string(path))
    if errors.Is(err, fs.ErrNotExist) {
      panic(rbSysErr(err, "rb_dir_s_empty_p", string(path)))
    }
    return Boolean(err == nil && len(es) == 0)
  }

  #: (String) -> Integer
  def self.delete(path) = rmdir(path)

  #: (String) -> Integer
  def self.unlink(path) = rmdir(path)

  #: () -> String
  def self.pwd = %x{
    d, err := os.Getwd()
    if err != nil {
      panic(rbSysErr(err, "rb_dir_getwd", "."))
    }
    return String(d)
  }

  # ponytail: sorted, where MRI's children/entries come in readdir order; sort both sides to compare.
  #: (String) -> Array[String]
  def self.children(path) = %x{
    es, err := os.ReadDir(string(path))
    if err != nil {
      panic(rbSysErr(err, "dir_initialize", string(path)))
    }
    names := make([]string, len(es))
    for i, e := range es {
      names[i] = e.Name()
    }
    return rbStrs(names)
  }

  #: (String) -> Array[String]
  def self.entries(path) = [".", ".."] + children(path)

  # `*`, `?`, `[set]`, `**/`, `{a,b}` and a trailing `/`, in MRI's sorted order (rbGlob, decision 94); flags may be
  # File::FNM_DOTMATCH.
  #: (String, ?Integer) -> Array[String]
  def self.glob(pattern, flags = 0) = %x{ return rbStrs(rbGlob(string(pattern), int(flags))) }

  # Several patterns: each one's matches, in order (rubocop's TargetFinder#find_files; decision 444).
  #: (Array[String], ?Integer) -> Array[String]
  def self.__glob_array(patterns, flags = 0)
    out = [] #: Array[String]
    patterns.each { |pattern| out.concat(glob(pattern, flags)) }
    out
  end

  #: (*String) -> Array[String]
  def self.[](*patterns)
    out = [] #: Array[String]
    patterns.each { |pat| out.concat(glob(pat)) }
    out
  end

  # ponytail: sorted, like children.
  #: (String) { (String) -> void } -> void
  def self.each_child(path)
    children(path).each { |c| yield c }
  end

  # The working directory is the process's: other threads see the change too, as in MRI.
  #: [T] (String) { (String) -> T } -> T
  def self.chdir(path)
    old = pwd
    __chdir(path)
    begin
      yield path
    ensure
      __chdir(old)
    end
  end

  # Blockless Dir.chdir (decision 12's `__<name>_enum` stands for "no block").
  #: (String) -> Integer
  def self.__chdir_enum(path)
    __chdir(path)
    0
  end

  #: (String) -> void
  def self.__chdir(path) = %x{
    if err := os.Chdir(string(path)); err != nil {
      panic(rbSysErr(err, "dir_s_chdir", string(path)))
    }
  }

  #: (String) -> bool
  def self.exist?(path) = File.directory?(path)

  # mode nil is 0777; the umask applies, as MRI's.
  #: (String, ?Integer?) -> Integer
  def self.mkdir(path, mode = nil) = %x{
    fm := fs.FileMode(0o777)
    if mode.ok {
      fm = fs.FileMode(mode.v & 0o777)
      if mode.v&0o4000 != 0 {
        fm |= fs.ModeSetuid
      }
      if mode.v&0o2000 != 0 {
        fm |= fs.ModeSetgid
      }
      if mode.v&0o1000 != 0 {
        fm |= fs.ModeSticky
      }
    }
    err := os.Mkdir(string(path), fm) //nolint:gosec // MRI's mode; the umask applies
    if err != nil {
      panic(rbSysErr(err, "rb_dir_s_mkdir", string(path)))
    }
    return 0
  }

  #: (String) -> Integer
  def self.rmdir(path) = %x{
    err := syscall.Rmdir(string(path)) // os.Remove would unlink a file too
    if err != nil {
      panic(rbSysErr(err, "dir_s_rmdir", string(path)))
    }
    return 0
  }
end
