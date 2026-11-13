# rbs_inline: enabled

# Shared by IO and File, as MRI's IO::generic_writable/readable; includers supply write and gets.
# rugaroo's own, so ancestors leaves it out.
# @hidden
module IOWritable
  #: (untyped) -> Integer
  def write(x) = raise(NotImplementedError)

  # The stream itself: an IO or File is its own IO object (a Tempfile delegates it: Homebrew's Readall `stdout_file.to_io`)
  #: () -> self
  def to_io = self

  # write(a, b, ...): each in turn, the byte count summed (MRI 2.5+).
  #: (untyped, untyped) -> Integer
  def __write_2(a, b) = write(a) + write(b)

  #: (untyped, untyped, untyped) -> Integer
  def __write_3(a, b, c) = write(a) + write(b) + write(c)

  #: (untyped, untyped, untyped, untyped) -> Integer
  def __write_4(a, b, c, d) = write(a) + write(b) + write(c) + write(d)

  #: (*untyped) -> nil
  def print(*args)
    ofs = $, # joins the arguments, and $\ follows them, when the program set them (decision 414)
    first = true
    args.each do |a|
      write(ofs) if ofs && !first
      first = false
      write(a)
    end
    ors = $\
    write(ors) if ors
    nil
  end

  #: (Integer) -> Integer
  def putc(c)
    write((c & 0xff).chr)
    c
  end

  #: (String) -> String
  def __putc_string(s)
    write(s[0] || "")
    s
  end

  #: (*untyped) -> nil
  def puts(*args)
    if args.empty?
      write("\n")
      return nil
    end
    args.each do |a|
      case a
      when nil then write("\n")
      when Array then a.each { |e| puts(e) }
      else
        s = a.to_s
        write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  #: (String, *untyped) -> nil
  def printf(fmt, *args)
    write(format(fmt, *args))
    nil
  end
end

# @hidden
module IOReadable
  #: () -> String?
  def gets = raise(NotImplementedError)

  #: (?chomp: bool) -> String
  def readline(chomp: false)
    line = gets
    raise EOFError, "end of file reached" unless line

    chomp && line.end_with?("\n") ? line.chomp : line # a final "x\r" keeps its \r, as MRI
  end

  #: () { (String) -> void } -> void
  def each_line
    while (line = gets)
      yield line
    end
  end

  #: () -> Array[String]
  def readlines
    out = [] #: Array[String]
    while (line = gets)
      out << line
    end
    out
  end
end

# A standard stream (fd 0-2), or a pipe or popen end when own (decision 138).
# @go_type struct { fd int; via rbWriter; lineno int; own bool; noclose bool; borrowed bool; closed bool; sync bool; rf *os.File; wf *os.File; r *bufio.Reader; w *bufio.Writer; cmd *exec.Cmd; enc rbIOEnc; conv func(string) string }
class IO < Object
  include IOWritable
  include IOReadable

  SEEK_SET = 0 #: Integer
  SEEK_CUR = 1 #: Integer
  SEEK_END = 2 #: Integer

  #: (Integer) -> IO
  def self.__new(fd) = %x{ return &IO{fd: int(fd)} }

  # A raw descriptor, as MRI's (decision 62's Errno classes on failure).
  #: (String, ?String, ?Integer) -> Integer
  def self.sysopen(path, mode = "r", perm = 0o666) = %x{ return Integer(rbSysopen(string(path), string(mode), int(perm))) }

  # An IO over descriptor fd; closing it closes fd unless `autoclose: false` (then fd stays the caller's).
  # mode: a String, or the `mode:` keyword when it is nil; other options (binmode:, encodings) are accepted and ignored (ponytail: honor them when a program needs one).
  #: (Integer, ?String?, ?untyped, **untyped) -> IO
  def self.new(fd, mode = nil, perm = :__none, **opts)
    raise ArgumentError, "wrong number of arguments (given 3, expected 1..2)" unless perm == :__none

    raise ArgumentError, "invalid access mode " if mode == ""

    __check_mode_opts(mode, opts)
    m = mode || opts[:mode]&.to_s || "" # none: the descriptor's own access mode
    io = if opts[:autoclose] == false
           # a dup: closing it, by close or by Go's finalizer on the os.File, leaves the caller's descriptor alone
           b = __for_fd(__dup_fd(fd), m)
           b.__borrowed = true
           b
         else
           __for_fd(fd, m)
         end
    io.binmode if m.split(":").first.to_s.include?("b") || opts[:binmode]
    io
  end

  #: (String?, Hash[Symbol, untyped]) -> void
  def self.__check_mode_opts(mode, opts)
    return unless mode # MRI's rb_io_extract_modeenc: what the mode string says no option may say again

    raise ArgumentError, "mode specified twice" unless opts[:mode].nil?

    access, enc = mode.split(":", 2)
    if enc && (opts.key?(:encoding) || opts.key?(:external_encoding) || opts.key?(:internal_encoding))
      raise ArgumentError, "encoding specified twice"
    end

    a = access.to_s
    raise ArgumentError, "binmode specified twice" if a.include?("b") && opts.key?(:binmode)
    raise ArgumentError, "textmode specified twice" if a.include?("t") && opts.key?(:textmode)
    raise ArgumentError, "both textmode and binmode specified" if (a.include?("b") && opts.key?(:textmode)) || (a.include?("t") && opts.key?(:binmode))
  end

  #: (Integer, String) -> IO
  def self.__for_fd(fd, mode) = %x{ return rbIOForFd(int(fd), string(mode)) }

  #: (Integer) -> Integer
  def self.__dup_fd(fd) = %x{ return Integer(rbDupFd(int(fd))) }

  #: (bool) -> void
  def __borrowed=(v)
    %x{ self.borrowed = bool(v) }
  end

  # Whether close closes the descriptor (MRI's IO#autoclose=): false for a borrowed fd another IO still owns.
  #: (bool) -> bool
  def autoclose=(v)
    %x{
      self.noclose = !bool(v)
      if self.noclose {
        rbPinFiles(self.rf, self.wf) // Go's os.File finalizer would close the borrowed fd when this IO is collected
      }
      return v
    }
  end

  #: () -> bool
  def autoclose? = %x{ Boolean(!self.noclose && !self.borrowed) }

  # IO.open with a block yields the new IO and closes it after, answering the block's value; without one it is IO.new.
  #: [T] (Integer, ?String?, ?untyped, **untyped) { (IO) -> T } -> T
  def self.open(fd, mode = nil, perm = :__none, **opts)
    io = new(fd, mode, perm, **opts)
    begin
      yield io
    ensure
      io.close unless io.closed?
    end
  end

  #: (Integer, ?String?, ?untyped, **untyped) -> IO
  def self.__open_enum(fd, mode = nil, perm = :__none, **opts) = new(fd, mode, perm, **opts)

  #: (Integer, ?String?, ?untyped, **untyped) -> IO
  def self.for_fd(fd, mode = nil, perm = :__none, **opts) = new(fd, mode, perm, **opts)

  #: (String, ?Integer?, ?Integer?, **untyped) -> String
  def self.read(path, length = nil, offset = nil, **opts) = File.read(path, length, offset, **opts)

  # @binary
  #: (String) -> String
  def self.binread(path) = File.read(path)

  #: (String, untyped) -> Integer
  def self.write(path, data) = File.write(path, data)

  #: (String, untyped) -> Integer
  def self.binwrite(path, data) = File.write(path, data)

  #: (String, ?chomp: bool) -> Array[String]
  def self.readlines(path, chomp: false) = File.readlines(path, chomp: chomp)

  #: (String) { (String) -> void } -> void
  def self.foreach(path)
    File.foreach(path) { |line| yield line }
  end

  # [rows, columns] of the terminal self is (TIOCGWINSZ); Errno::ENOTTY when it is none.
  #: () -> [Integer, Integer]
  def winsize = %x{
    ws, err := rbWinsize(self.rbOSFile())
    if err != nil {
      panic(rbSysErr(err, "rb_io_winsize", ""))
    }
    return Tuple2[Integer, Integer]{Integer(ws[0]), Integer(ws[1])}
  }

  # What is there to read now, up to maxlen bytes; IO::EAGAINWaitReadable when nothing is, EOFError at the end.
  # @binary
  #: (Integer) -> String
  def read_nonblock(maxlen) = %x{ return rbReadNonblock(self, int(maxlen), true) }

  # Up to maxlen bytes, blocking only until some are there; EOFError at the end.
  # @binary
  #: (Integer) -> String
  def readpartial(maxlen) = %x{ return rbReadNonblock(self, int(maxlen), false) }

  # The readers with something to read (or at their end) once one has, as MRI's [readers, writers, errors];
  # nil when timeout seconds pass first. Writers count as always writable.
  #: (Array[IO], ?Array[IO]?, ?Array[IO]?, ?untyped) -> [Array[IO], Array[IO], Array[IO]]?
  def self.select(reads, writes = nil, errs = nil, timeout = nil) = %x{
    var limit *time.Duration
    if t := rbUnbox(timeout); t != nil {
      d := time.Duration(rbNumFloat(t) * float64(time.Second))
      limit = &d
    }
    ready := rbIOSelect(reads.s, limit)
    if ready == nil && (writes == nil || len((*writes).s) == 0) {
      return nil
    }
    out := Tuple3[*Array[*IO], *Array[*IO], *Array[*IO]]{F0: &Array[*IO]{s: ready}, F1: &Array[*IO]{}, F2: &Array[*IO]{}}
    if writes != nil {
      out.F1.s = append(out.F1.s, (*writes).s...)
    }
    return &out
  }

  #: () -> [IO, IO]
  def self.pipe = %x{
    r, w, err := os.Pipe()
    if err != nil {
      panic(rbSysErr(err, "rb_io_s_pipe", ""))
    }
    return Tuple2[*IO, *IO]{rbIONew(r, nil, nil), rbIONew(nil, w, nil)}
  }

  # A String follows system's shell rule, an Array of Strings runs directly; closing the IO reaps the child into $?.
  # An env Hash may lead an Array command; opts are spawn's (decision 241).
  # @rbs [T] (untyped, ?String, ?Hash[Symbol, untyped]) { (IO) -> T } -> T
  def self.popen(cmd, mode = "r", opts = {})
    io = __popen_enum(cmd, mode, opts)
    begin
      yield io
    ensure
      io.close
    end
  end

  # Blockless IO.popen (decision 12's `__<name>_enum` stands for "no block").
  #: (untyped, ?String, ?Hash[Symbol, untyped]) -> IO
  def self.__popen_enum(cmd, mode = "r", opts = {}) = %x{ return rbPopen(cmd, string(mode), opts) }

  # Path to path, as MRI's with two filenames; the count of bytes copied.
  #: (String, String) -> Integer
  def self.copy_stream(src, dst) = File.write(dst, File.read(src))

  #: () -> Array[String]
  def self.__argv = %x{
    a := &Array[String]{s: make([]String, 0, len(os.Args)-1)}
    for _, s := range os.Args[1:] {
      a.s = append(a.s, String(s))
    }
    return a
  }

  #: (untyped) -> Integer
  def write(x) = %x{
    s := string(rbToS(x))
    if self.conv != nil {
      s = self.conv(s)
    }
    if self.via != nil { // $stdout/$stderr read while assigned (decision 109)
      self.via.Write(String(s))
      return Integer(len(s))
    }
    if self.closed {
      panic(NewIOError(Ref[String]("closed stream")))
    }
    switch {
    case self.own:
      self.rbWritePipe(s)
    case self.fd == 1:
      rbWrite(s)
    case self.fd == 2:
      rbFlushIfTTY()
      _, _ = os.Stderr.WriteString(s)
    default:
      panic(NewIOError(Ref[String]("not opened for writing")))
    }
    return Integer(len(s))
  }

  #: (untyped) -> IO
  def <<(x)
    write(x)
    self
  end

  #: () -> IO
  def flush = %x{
    self.rbOpen()
    switch {
    case self.own && self.w != nil:
      if err := self.w.Flush(); err != nil {
        panic(rbIOWriteErr(err))
      }
    case self.fd == 1:
      rbFlush()
    }
    return self
  }

  #: () -> Integer
  def fileno = %x{
    self.rbOpen()
    return Integer(self.fd)
  }

  #: () -> Integer
  def to_i = fileno

  # STDERR is always sync, as in MRI; STDOUT until `sync = true` only flushes per write on a terminal.
  #: () -> bool
  def sync = %x{
    self.rbOpen()
    if self.own {
      return Boolean(self.sync)
    }
    return Boolean(self.fd == 2 || self.fd == 1 && stdoutSync.Load())
  }

  #: (bool) -> bool
  def sync=(on)
    %x{
    self.rbOpen()
    switch {
    case self.own:
      self.sync = bool(on)
      if on && self.w != nil {
        _ = self.w.Flush()
      }
    case self.fd == 1:
      stdoutSync.Store(bool(on))
      if on {
        rbFlush()
      }
    }
    return on
    }
  end

  #: () -> bool
  def tty? = %x{
    self.rbOpen()
    return Boolean(rbIsTTY(self.rbOSFile()))
  }

  #: () -> Integer?
  def pid = %x{
    self.rbOpen()
    if self.cmd == nil {
      return rbOpt[Integer]{}
    }
    return rbSome(Integer(self.cmd.Process.Pid))
  }

  # ponytail: a standard stream is only marked closed, so Kernel#puts still writes; close the real fd and route Kernel output through this state (decision 138).
  #: () -> nil
  def close = %x{ self.rbClose() }

  #: () -> bool
  def closed? = %x{ Boolean(self.closed) }

  #: () -> self
  def dup = %x{
    self.rbOpen()
    return rbDup(self)
  }

  #: () -> nil
  def close_read = %x{ self.rbCloseHalf(true) }

  #: () -> nil
  def close_write = %x{ self.rbCloseHalf(false) }

  #: () -> String?
  def gets = %x{
    line, err := (*self.rbReader()).ReadString('\\n')
    if line == "" && err != nil {
      return nil
    }
    self.lineno++
    rbNoteLineNo(self.lineno)
    s := String(line)
    return &s
  }

  #: () -> Integer
  def lineno = %x{
    _ = self.rbReader()
    return Integer(self.lineno)
  }

  #: (Integer) -> Integer
  def lineno=(n)
    %x{
    _ = self.rbReader()
    self.lineno = rbCInt(n)
    return n
    }
  end

  #: () -> String
  def read = %x{
    b, _ := io.ReadAll(*self.rbReader())
    return String(b)
  }

  # read(length): at most length bytes, nil at the end ("" for 0); nil reads the rest (as File#read's).
  # @binary
  #: (Integer?) -> String?
  def __read_1(length) = %x{
    r := *self.rbReader()
    if !length.ok {
      b, _ := io.ReadAll(r)
      s := String(b)
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
    n, _ := io.ReadFull(r, buf)
    if n == 0 {
      return nil
    }
    s := String(buf[:n])
    return &s
  }

  #: () -> bool
  def eof? = %x{
    _, err := (*self.rbReader()).Peek(1)
    return Boolean(err != nil)
  }

  #: () -> String?
  def getc = %x{ return rbGetc(*self.rbReader()) }

  # io/console: one character, without raw mode (line-buffered on a terminal).
  #: () -> String?
  def getch = getc

  #: () -> Integer?
  def getbyte = %x{ return rbOptFromPtr(rbGetbyte(*self.rbReader())) }

  #: () -> String
  def readchar = %x{ return *rbEOF(rbGetc(*self.rbReader())) }

  #: () -> Integer
  def readbyte = %x{ return *rbEOF(rbGetbyte(*self.rbReader())) }

  #: (String) -> nil
  def ungetc(s) = %x{ rbUngetc(self.rbReader(), string(s)) }

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

  #: () -> String
  def inspect = %x{
    switch {
    case self.own && self.closed:
      return "#<IO:(closed)>"
    case self.own:
      return String("#<IO:fd " + strconv.Itoa(self.fd) + ">")
    case self.closed:
      return String([]string{"#<IO:<STDIN> (closed)>", "#<IO:<STDOUT> (closed)>", "#<IO:<STDERR> (closed)>"}[self.fd])
    }
    return String([]string{"#<IO:<STDIN>>", "#<IO:<STDOUT>>", "#<IO:<STDERR>>"}[self.fd])
  }

  # A reading IO ($stdin, a pipe's reader, popen "r") reads with default_external; a writing one has none until set_encoding, as in MRI.
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

  # Writes convert to the new external encoding; a pipe's reads convert as a File's, $stdin's do not (decision 136). The conversion is a func field so programs that never call this carry no transcoder.
  #: (untyped, ?untyped) -> IO
  def set_encoding(ext, intern = nil) = %x{
    e := rbSetEncoding(ext, intern)
    e.bin = self.enc.bin
    if self.own {
      e.raw, e.unread, e.restart = self.enc.raw, self.enc.unread, self.enc.restart
      same := e.ext == self.enc.ext && e.intern == self.enc.intern // keep the transcoder and what it has read ahead
      self.enc = e
      if self.r != nil && !same {
        self.r = self.enc.readerFor(self.r)
      }
    } else {
      self.enc = e
    }
    self.conv = self.enc.writeConv
    if !self.own && self.fd == 1 && self.via == nil { // a $stdout bound to a StringIO keeps its encoding to itself
      c := self.conv
      rbStdoutConv.Store(&c)
    }
    return self
  }

  #: () -> IO
  def binmode = %x{
    self.rbOpen()
    self.enc = rbIOEnc{ext: "ASCII-8BIT", bin: true, raw: self.enc.raw, unread: self.enc.unread}
    if self.own && self.r != nil {
      self.r = self.enc.readerFor(self.r)
    }
    self.conv = nil
    if !self.own && self.fd == 1 && self.via == nil {
      rbStdoutConv.Store(nil)
    }
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
  def __readable? = %x{ Boolean(self.own && self.r != nil || !self.own && self.fd == 0) }
end

STDIN = IO.__new(0) #: IO
STDOUT = IO.__new(1) #: IO
STDERR = IO.__new(2) #: IO

ARGV = IO.__argv #: Array[String]

class IO
  # A read that would block: rescued as IO::WaitReadable or Errno::EAGAIN (read_nonblock, accept_nonblock).
  module WaitReadable; end
  module WaitWritable; end

  class TimeoutError < IOError; end

  class EAGAINWaitReadable < Errno::EAGAIN
    include IO::WaitReadable
  end
end

# @go_type struct{}
class ENVClass < Object
  #: () -> ENVClass
  def self.new = %x{ return &ENVClass{} }

  #: () -> String
  def self.__platform = %x{ return String(rbRubyPlatform()) }

  #: (String) -> String?
  def [](name) = %x{
    v, ok := os.LookupEnv(string(name))
    if !ok {
      return nil
    }
    s := String(v)
    return &s
  }

  #: (String, String?) -> String?
  def []=(name, value)
    %x{
    if value == nil {
      _ = os.Unsetenv(string(name))
    } else {
      if name == "" || strings.Contains(string(name), "=") {
        panic(NewErrno_EINVAL(Ref(String("Invalid argument - setenv(" + string(name) + ")"))))
      }
      _ = os.Setenv(string(name), string(*value))
    }
    if name == "TZ" {
      rbResetLocalZone(value)
    }
    return value
    }
  end

  #: (String, ?String?) -> String
  def fetch(name, default = nil)
    v = self[name]
    return v if v
    return default if default

    raise KeyError.__for("key not found: #{name.inspect}", self, name)
  end

  #: (String) -> bool
  def key?(name) = !self[name].nil?

  #: (String) -> bool
  def include?(name) = key?(name)

  #: (String) -> String?
  def delete(name)
    v = self[name]
    self[name] = nil
    v
  end

  #: () -> Hash[String, String]
  def to_h = %x{
    h := NewHash[String, String]()
    for _, kv := range os.Environ() {
      k, v, _ := strings.Cut(kv, "=")
      Hash_Op_idxSet(h, String(k), String(v))
    }
    return h
  }

  #: () -> Array[String]
  def keys = to_h.keys

  #: () { (String, String) -> void } -> void
  def each
    to_h.each { |k, v| yield k, v }
  end

  #: () -> String
  def inspect = to_h.inspect

  #: () -> String
  def to_s = "ENV"

  #: () -> self
  def dup = raise(TypeError, "Cannot dup ENV, use ENV.to_h to get a copy of ENV as a hash")

  #: () { (String, String) -> void } -> void
  def each_pair
    to_h.each { |k, v| yield k, v }
  end

  #: () { (String) -> void } -> void
  def each_key
    to_h.each { |k, _v| yield k }
  end

  #: () { (String) -> void } -> void
  def each_value
    to_h.each { |_k, v| yield v }
  end

  #: () -> Hash[String, String]
  def to_hash = to_h

  #: () -> Array[[String, String]]
  def to_a = to_h.to_a

  #: () -> Array[String]
  def values = to_h.values

  #: (*String) -> Array[String?]
  def values_at(*names) = names.map { |n| self[n] }

  #: () -> Integer
  def size = to_h.size

  #: () -> Integer
  def length = size

  #: () -> bool
  def empty? = size == 0

  #: (String) -> bool
  def has_key?(name) = key?(name)

  #: (String) -> bool
  def member?(name) = key?(name)

  #: (String) -> bool
  def value?(value) = to_h.value?(value)

  #: (String) -> bool
  def has_value?(value) = value?(value)

  #: (String) -> String?
  def key(value) = to_h.key(value)

  #: () -> Hash[String, String]
  def invert = to_h.invert

  #: (String) -> [String, String]?
  def assoc(name) = to_h.assoc(name)

  #: (*String) -> Hash[String, String]
  def slice(*names) = to_h.slice(*names)

  #: (*String) -> Hash[String, String]
  def except(*names) = to_h.except(*names)

  #: [U] () { (String, String) -> U } -> Array[U]
  def map
    out = [] #: Array[U]
    to_h.each { |k, v| out << yield(k, v) }
    out
  end

  #: [U] () { (String, String) -> U? } -> Array[U]
  def filter_map
    out = [] #: Array[U]
    to_h.each do |k, v|
      x = yield(k, v)
      out << x if x
    end
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def select
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def filter
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v if yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> Hash[String, String]
  def reject
    out = {} #: Hash[String, String]
    to_h.each { |k, v| out[k] = v unless yield(k, v) }
    out
  end

  #: () { (String, String) -> bool } -> bool
  def any?
    to_h.each { |k, v| return true if yield(k, v) }
    false
  end

  #: (String, String) -> String
  def store(name, value)
    self[name] = value
    value
  end

  #: (Hash[String, String]) -> self
  def update(other)
    other.each { |k, v| self[k] = v }
    self
  end

  #: (Hash[String, String]) -> self
  def merge!(other) = update(other)

  #: () { (String, String) -> bool } -> self
  def delete_if
    to_h.each { |k, v| self[k] = nil if yield(k, v) }
    self
  end

  #: () { (String, String) -> bool } -> self
  def keep_if
    to_h.each { |k, v| self[k] = nil unless yield(k, v) }
    self
  end

  #: (Hash[String, String]) -> self
  def replace(other)
    update(other)
    to_h.each { |k, _v| self[k] = nil unless other.key?(k) }
    self
  end

  #: () -> self
  def clear
    to_h.each { |k, _v| self[k] = nil }
    self
  end
end

ENV = ENVClass.new #: ENVClass

# What class-body RUBY_VERSION guards fold against (decision 167); the platform is MRI's spelling of Go's GOARCH-GOOS,
# without the OS version MRI appends (arm64-darwin, x86_64-linux).
RUBY_VERSION = "4.0.0" #: String
RUBY_PLATFORM = ENVClass.__platform #: String
RUBY_ENGINE = "ruby" #: String
RUBY_ENGINE_VERSION = RUBY_VERSION #: String

# ARGF (decision 95): the files named in ARGV, read in turn and shifted out
# of it as each opens, or stdin when ARGV is empty at the first read.
# @go_type struct { r *bufio.Reader; f *os.File; name string; started, stdin, done bool; lineno int }
class ARGFClass < Object
  #: () -> ARGFClass
  def self.new = %x{ return &ARGFClass{} }

  #: () -> String?
  def gets = __gets(ARGV)

  #: () -> String
  def read = __read(ARGV)

  #: () -> Array[String]
  def readlines
    out = [] #: Array[String]
    while (line = gets)
      out << line
    end
    out
  end

  #: () { (String) -> void } -> void
  def each_line
    while (line = gets)
      yield line
    end
  end

  #: () -> String
  def filename = __filename(ARGV)

  #: () -> bool
  def eof? = __eof(ARGV)

  #: () -> String
  def inspect = "ARGF"

  #: () -> String
  def to_s = "ARGF"

  #: (Array[String]) -> String?
  def __gets(argv) = %x{
    for r := self.next(argv); r != nil; r = self.next(argv) {
      line, err := r.ReadString('\\n')
      if err != nil {
        self.spent()
      }
      if line != "" {
        self.lineno++ // counted across ARGV's files, as MRI's ARGF.lineno and $. (decision 425)
        rbNoteLineNo(self.lineno)
        s := String(line)
        return &s
      }
    }
    return nil
  }

  #: (Array[String]) -> String
  def __read(argv) = %x{
    var b strings.Builder
    for r := self.next(argv); r != nil; r = self.next(argv) {
      _, _ = io.Copy(&b, r)
      self.spent()
    }
    return String(b.String())
  }

  #: (Array[String]) -> String
  def __filename(argv) = %x{
    self.next(argv)
    return String(self.name)
  }

  #: (Array[String]) -> bool
  def __eof(argv) = %x{
    r := self.next(argv)
    if r == nil && !self.stdin {
      panic(NewIOError(Ref[String]("closed stream"))) // the last file is closed, as in MRI
    }
    if r == nil {
      return true
    }
    _, err := r.Peek(1)
    return Boolean(err != nil)
  }
end

ARGF = ARGFClass.new #: ARGFClass

# Kernel#warn writes through Warning.warn; the category switches are MRI 4.0's defaults (decision 129).
module Warning
  CATEGORIES__ = { deprecated: false, experimental: true, performance: false, strict_unused_block: false } #: Hash[Symbol, bool]

  #: (Symbol) -> bool
  def self.[](category)
    on = CATEGORIES__[category]
    raise ArgumentError, "unknown category: #{category}" if on.nil?

    on
  end

  #: (Symbol, bool) -> bool
  def self.[]=(category, flag)
    self[category]
    CATEGORIES__[category] = flag
  end

  #: () -> Array[Symbol]
  def self.categories = CATEGORIES__.keys

  #: (String, ?category: Symbol?) -> nil
  def self.warn(msg, category: nil)
    self[category] if category
    $stderr.write(msg)
    nil
  end
end

module Kernel
  private

  # Built as MRI's rb_warn_m: arrays flatten, each message gets a newline unless it has one, and the whole goes to Warning.warn.
  #: (*untyped, ?uplevel: Integer?, ?category: Symbol?) -> nil
  def warn(*msgs, uplevel: nil, category: nil)
    msgs = msgs.flatten
    return nil if msgs.empty? || $VERBOSE.nil? || (category && !Warning[category])

    s = ""
    if uplevel
      raise ArgumentError, "negative level (#{uplevel})" if uplevel < 0

      loc = __caller_loc(uplevel)
      s += "#{loc}: " unless loc.empty?
      s += "warning: "
    end
    msgs.each do |m|
      t = m.to_s
      s += (t.end_with?("\n") ? t : t + "\n")
    end
    Warning.warn(s, category: category)
  end

  # $_ before a frame's first gets: nil, typed as gets' line (decision 446).
  #: () -> String?
  def __no_line = nil

  # `recv.send(m, ...)` in a $_ frame: gets and readline sent by name set $_ as a literal call does (decision 459).
  #: (untyped, untyped, String?) -> String?
  def __line_or(name, value, old) = %w[gets readline].include?(name.to_s) ? value : old

  # The user-code frame uplevel frames up from warn's caller (decision 129).
  #: (Integer) -> String
  def __caller_loc(n) = %x{ return String(rbCallerLocN(int(n))) }

  #: (?String?) -> void
  def abort(msg = nil)
    STDERR.puts(msg) if msg
    raise SystemExit.new(1)
  end

  #: () -> String?
  def gets = ARGF.gets

  # The calling stack as backtrace lines (decision 189): caller(0) starts at the method that calls it.
  #: (?Integer, ?Integer?) -> Array[String]?
  def caller(start = 1, length = nil) = %x{
    if start < 0 {
      panic(NewArgumentError(Ref(String(fmt.Sprintf("negative level (%d)", start)))))
    }
    l := -1
    if length.ok {
      if length.v < 0 {
        panic(NewArgumentError(Ref(String(fmt.Sprintf("negative size (%d)", length.v)))))
      }
      l = int(length.v)
    }
    out := rbCaller(int(start), l)
    if out == nil {
      return nil
    }
    return &out
  }

  # caller's frames as Thread::Backtrace::Location objects (decision 190).
  #: (?Integer, ?Integer?) -> Array[Thread::Backtrace::Location]?
  def caller_locations(start = 1, length = nil)
    lines = caller(start + 1, length)
    return nil unless lines

    lines.map { |l| Thread::Backtrace::Location.__parse(l) }
  end

  # Kernel#open on a path is File.open (a "|cmd" pipe is not supported: decision 97's spawn is the way).
  #: [T] (String, ?(String | Integer | nil), ?Integer?, **untyped) { (File) -> T } -> T
  def open(path, mode = nil, perm = nil, **opts, &blk) = File.open(path, mode, perm, **opts, &blk)

  #: (String, ?(String | Integer | nil), ?Integer?, **untyped) -> File
  def __open_enum(path, mode = nil, perm = nil, **opts) = File.new(path, mode, perm, **opts)

  # Kernel#test's file tests: ?e ?f ?d ?r ?w ?x ?s ?z ?l (a String's first character also names one).
  #: (String | Integer, String) -> untyped
  def test(cmd, path)
    c = cmd.is_a?(Integer) ? cmd.chr : cmd[0].to_s
    case c
    when "e" then File.exist?(path)
    when "f" then File.file?(path)
    when "d" then File.directory?(path)
    when "r" then File.readable?(path)
    when "w" then File.writable?(path)
    when "x" then File.executable?(path)
    when "l" then File.symlink?(path)
    when "z" then File.zero?(path)
    when "s" then File.size?(path)
    else raise ArgumentError, "unknown command '#{c}'"
    end
  end
end

class Thread
  module Backtrace
    # One frame of caller_locations: path, lineno and label parsed from its backtrace line.
    class Location
      attr_reader :path #: String
      attr_reader :lineno #: Integer
      attr_reader :label #: String

      #: (String, Integer, String) -> void
      def initialize(path, lineno, label)
        @path = path
        @lineno = lineno
        @label = label
      end

      #: (String) -> Thread::Backtrace::Location
      def self.__parse(line)
        m = line.match(/\A(.*?):(\d+):in '(.*)'\z/)
        return new(line, 0, "") unless m

        new(m[1] || "", (m[2] || "0").to_i, m[3] || "")
      end

      #: () -> String
      def absolute_path = File.expand_path(path)

      #: () -> String
      def base_label = label.sub(/\A.*[#.]/, "")

      #: () -> String
      def to_s = "#{path}:#{lineno}:in '#{label}'"

      #: () -> String
      def inspect = to_s.inspect
    end
  end
end
