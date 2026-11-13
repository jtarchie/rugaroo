# rbs_inline: enabled

# @go_type struct { mu sync.Mutex; buf []byte; pos int; lineno int; readOpen bool; writeOpen bool; canWrite bool; appendMode bool; oum *rbOpenURIMeta }
class StringIO < Object
  #: (?String, ?String) -> StringIO
  def self.new(s = "", mode = "r+") = %x{
    canRead, canWrite, appendMode, truncate := true, true, false, false
    switch string(mode) {
    case "r":
      canWrite = false
    case "r+":
    case "w":
      canRead, truncate = false, true
    case "w+":
      truncate = true
    case "a":
      canRead, appendMode = false, true
    case "a+":
      appendMode = true
    default:
      panic(NewArgumentError(Ref(String("invalid access mode " + string(mode)))))
    }
    buf := []byte(string(s))
    if truncate {
      buf = []byte{}
    }
    return &StringIO{
      buf: buf, readOpen: canRead, writeOpen: canWrite,
      canWrite: canWrite, appendMode: appendMode,
    }
  }

  #: () -> String
  def string = %x{
    self.mu.Lock(); defer self.mu.Unlock() // MRI's GVL makes each call atomic; a goroutine per Thread needs a lock per object
    return String(self.buf)
  }

  #: (String) -> String
  def string=(s)
    %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.buf, self.pos, self.lineno = []byte(string(s)), 0, 0
    return s}
  end

  # write(a, b, ...): each in turn, the byte count summed (MRI 2.5+), as IOWritable's.
  #: (untyped, untyped) -> Integer
  def __write_2(a, b) = write(a) + write(b)

  #: (untyped, untyped, untyped) -> Integer
  def __write_3(a, b, c) = write(a) + write(b) + write(c)

  #: (untyped) -> Integer
  def write(x) = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbWritable()
    s := string(rbToS(x))
    if self.appendMode {
      self.pos = len(self.buf)
    }
    if gap := self.pos - len(self.buf); gap > 0 {
      self.buf = append(self.buf, make([]byte, gap)...)
    }
    n := copy(self.buf[self.pos:], s)
    self.buf = append(self.buf, s[n:]...)
    self.pos += len(s)
    return Integer(len(s))
  }

  #: (untyped) -> StringIO
  def <<(x)
    write(x)
    self
  end

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

  #: () -> String
  def read = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    s := String(self.buf[min(self.pos, len(self.buf)):])
    self.pos = max(self.pos, len(self.buf))
    return s
  }

  # @binary
  #: (Integer) -> String?
  def __read_1(n) = %x{ // nil at EOF, as MRI's read(n)
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    if self.pos >= len(self.buf) && n > 0 {
      return nil
    }
    end := min(self.pos+int(n), len(self.buf))
    s := String(self.buf[self.pos:end])
    self.pos = end
    return &s
  }

  #: () -> String?
  def gets = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    if self.pos >= len(self.buf) {
      return nil
    }
    rest := self.buf[self.pos:]
    n := len(rest)
    if i := bytes.IndexByte(rest, '\\n'); i >= 0 {
      n = i + 1
    }
    s := String(rest[:n])
    self.pos += n
    self.lineno++
    return &s
  }

  #: () -> String?
  def getc = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    if self.pos >= len(self.buf) {
      return nil
    }
    _, n := utf8.DecodeRune(self.buf[self.pos:])
    s := String(self.buf[self.pos : self.pos+n])
    self.pos += n
    return &s
  }

  #: () -> Integer?
  def getbyte = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    if self.pos >= len(self.buf) {
      return rbOpt[Integer]{}
    }
    b := Integer(self.buf[self.pos])
    self.pos++
    return rbSome(b)
  }

  #: (?chomp: bool) -> String
  def readline(chomp: false)
    line = gets
    raise EOFError, "end of file reached" unless line

    chomp && line.end_with?("\n") ? line.chomp : line # a final "x\r" keeps its \r, as MRI
  end

  #: () -> String
  def readchar
    c = getc
    raise EOFError, "end of file reached" unless c

    c
  end

  #: () -> Integer
  def readbyte
    b = getbyte
    raise EOFError, "end of file reached" unless b

    b
  end

  #: () { (Integer) -> void } -> void
  def each_byte
    while (b = getbyte)
      yield b
    end
  end

  #: () { (String) -> void } -> void
  def each_char
    while (c = getc)
      yield c
    end
  end

  #: (untyped) -> nil
  def ungetc(c) = %x{ // replaces the last-read char's bytes at pos with c, or prepends c at pos 0
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    var s string
    switch v := c.(type) {
    case nil:
      return
    case Integer:
      s = string(rune(v))
    default:
      s = string(rbToS(c))
    }
    if s == "" {
      return
    }
    start := self.pos
    if start > 0 {
      _, n := utf8.DecodeLastRune(self.buf[:start])
      start -= n
    }
    tail := append([]byte{}, self.buf[self.pos:]...)
    self.buf = append(self.buf[:start:start], append([]byte(s), tail...)...)
    self.pos = start
  }

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

  #: () -> bool
  def eof? = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbReadable()
    return Boolean(self.pos >= len(self.buf))
  }

  #: () -> Integer
  def rewind = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.pos, self.lineno = 0, 0
    return 0
  }

  #: () -> Integer
  def pos = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    return Integer(self.pos)
  }

  #: (Integer) -> Integer
  def pos=(n)
    %x{
    self.mu.Lock(); defer self.mu.Unlock()
    if n < 0 {
      panic(NewErrno_EINVAL(Ref(String("Invalid argument"))))
    }
    self.pos = int(n)
    return n}
  end

  #: () -> Integer
  def lineno = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    return Integer(self.lineno)
  }

  #: () -> Integer
  def size = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    return Integer(len(self.buf))
  }

  #: () -> Integer
  def length = size

  #: (Integer, ?Integer) -> Integer
  def seek(offset, whence = IO::SEEK_SET) = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    if !self.readOpen && !self.writeOpen {
      panic(NewIOError(Ref[String]("closed stream")))
    }
    var base int
    switch whence {
    case IO_SEEK_CUR:
      base = self.pos
    case IO_SEEK_END:
      base = len(self.buf)
    default:
      base = 0
    }
    n := base + int(offset)
    if n < 0 {
      panic(NewErrno_EINVAL(Ref(String("Invalid argument"))))
    }
    self.pos = n
    return Integer(0)
  }

  #: () -> Integer
  def tell = pos

  # Bytes in, bytes out already: no transcoding to turn off.
  #: () -> StringIO
  def binmode = self

  #: () -> nil
  def close = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.readOpen, self.writeOpen = false, false
  }

  #: () -> bool
  def closed? = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    return Boolean(!self.readOpen && !self.writeOpen)
  }

  #: () -> nil
  def close_write = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    if !self.canWrite {
      panic(NewIOError(Ref[String]("closing non-duplex IO for writing")))
    }
    self.writeOpen = false
  }

  #: (Integer) -> Integer
  def truncate(newlen) = %x{
    self.mu.Lock(); defer self.mu.Unlock()
    self.rbWritable()
    if newlen < 0 {
      panic(NewErrno_EINVAL(Ref(String("Invalid argument - negative length"))))
    }
    n := int(newlen)
    switch {
    case n < len(self.buf):
      self.buf = self.buf[:n]
    case n > len(self.buf):
      self.buf = append(self.buf, make([]byte, n-len(self.buf))...)
    }
    return Integer(0)
  }
end
