# rbs_inline: enabled

# Encoding without a tag on String (decision 136): every String is UTF-8 bytes, so `encoding` is derived and only encode, Integer#chr and IO convert.

# @go_type struct { name string; names []string; ascii, dummy bool }
class Encoding < Object
  #: (String, String, bool, bool) -> Encoding
  def self.__new(name, names, ascii, dummy) = %x{ return &Encoding{name: string(name), names: strings.Split(string(names), ","), ascii: bool(ascii), dummy: bool(dummy)} }

  #: () -> String
  def name = %x{ String(self.name) }

  #: () -> String
  def to_s = name

  #: () -> Array[String]
  def names = %x{
    a := &Array[String]{s: make([]String, len(self.names))}
    for i, n := range self.names {
      a.s[i] = String(n)
    }
    return a
  }

  #: () -> String
  def inspect = %x{
    switch {
    case self.name == "ASCII-8BIT":
      return "#<Encoding:BINARY (ASCII-8BIT)>"
    case self.dummy:
      return String("#<Encoding:" + self.name + " (dummy)>")
    }
    return String("#<Encoding:" + self.name + ">")
  }

  #: () -> bool
  def ascii_compatible? = %x{ Boolean(self.ascii) }

  #: () -> bool
  def dummy? = %x{ Boolean(self.dummy) }

  ASCII_8BIT = __new("ASCII-8BIT", "ASCII-8BIT,BINARY", true, false) #: Encoding
  BINARY = ASCII_8BIT #: Encoding
  UTF_8 = __new("UTF-8", "UTF-8,CP65001,locale,external,filesystem", true, false) #: Encoding
  CP65001 = UTF_8 #: Encoding
  US_ASCII = __new("US-ASCII", "US-ASCII,ASCII,ANSI_X3.4-1968,646", true, false) #: Encoding
  ASCII = US_ASCII #: Encoding
  ANSI_X3_4_1968 = US_ASCII #: Encoding
  UTF_16BE = __new("UTF-16BE", "UTF-16BE,UCS-2BE", false, false) #: Encoding
  UCS_2BE = UTF_16BE #: Encoding
  UTF_16LE = __new("UTF-16LE", "UTF-16LE", false, false) #: Encoding
  UTF_32BE = __new("UTF-32BE", "UTF-32BE,UCS-4BE", false, false) #: Encoding
  UCS_4BE = UTF_32BE #: Encoding
  UTF_32LE = __new("UTF-32LE", "UTF-32LE,UCS-4LE", false, false) #: Encoding
  UCS_4LE = UTF_32LE #: Encoding
  UTF_16 = __new("UTF-16", "UTF-16", false, true) #: Encoding
  UTF_32 = __new("UTF-32", "UTF-32", false, true) #: Encoding
  ISO_8859_1 = __new("ISO-8859-1", "ISO-8859-1,ISO8859-1", true, false) #: Encoding
  ISO8859_1 = ISO_8859_1 #: Encoding
  ISO_2022_JP = __new("ISO-2022-JP", "ISO-2022-JP,ISO2022-JP", false, true) #: Encoding
  ISO2022_JP = ISO_2022_JP #: Encoding

  # BEGIN scripts/gen-encoding-fixes: the x/text encodings (decision 380), MRI's list and names
  EUC_JP = __new("EUC-JP", "EUC-JP,eucJP", true, false) #: Encoding
  EucJP = EUC_JP #: Encoding
  EUCJP = EUC_JP #: Encoding
  WINDOWS_31J = __new("Windows-31J", "Windows-31J,CP932,csWindows31J,SJIS,PCK", true, false) #: Encoding
  CP932 = WINDOWS_31J #: Encoding
  SJIS = WINDOWS_31J #: Encoding
  CsWindows31J = WINDOWS_31J #: Encoding
  CSWINDOWS31J = WINDOWS_31J #: Encoding
  PCK = WINDOWS_31J #: Encoding
  Windows_31J = WINDOWS_31J #: Encoding
  BIG5 = __new("Big5", "Big5", true, false) #: Encoding
  Big5 = BIG5 #: Encoding
  CP949 = __new("CP949", "CP949", true, false) #: Encoding
  EUC_KR = __new("EUC-KR", "EUC-KR,eucKR", true, false) #: Encoding
  EucKR = EUC_KR #: Encoding
  EUCKR = EUC_KR #: Encoding
  GB18030 = __new("GB18030", "GB18030", true, false) #: Encoding
  GBK = __new("GBK", "GBK,CP936", true, false) #: Encoding
  CP936 = GBK #: Encoding
  ISO_8859_2 = __new("ISO-8859-2", "ISO-8859-2,ISO8859-2", true, false) #: Encoding
  ISO8859_2 = ISO_8859_2 #: Encoding
  ISO_8859_3 = __new("ISO-8859-3", "ISO-8859-3,ISO8859-3", true, false) #: Encoding
  ISO8859_3 = ISO_8859_3 #: Encoding
  ISO_8859_4 = __new("ISO-8859-4", "ISO-8859-4,ISO8859-4", true, false) #: Encoding
  ISO8859_4 = ISO_8859_4 #: Encoding
  ISO_8859_5 = __new("ISO-8859-5", "ISO-8859-5,ISO8859-5", true, false) #: Encoding
  ISO8859_5 = ISO_8859_5 #: Encoding
  ISO_8859_6 = __new("ISO-8859-6", "ISO-8859-6,ISO8859-6", true, false) #: Encoding
  ISO8859_6 = ISO_8859_6 #: Encoding
  ISO_8859_7 = __new("ISO-8859-7", "ISO-8859-7,ISO8859-7", true, false) #: Encoding
  ISO8859_7 = ISO_8859_7 #: Encoding
  ISO_8859_8 = __new("ISO-8859-8", "ISO-8859-8,ISO8859-8", true, false) #: Encoding
  ISO8859_8 = ISO_8859_8 #: Encoding
  ISO_8859_9 = __new("ISO-8859-9", "ISO-8859-9,ISO8859-9", true, false) #: Encoding
  ISO8859_9 = ISO_8859_9 #: Encoding
  ISO_8859_10 = __new("ISO-8859-10", "ISO-8859-10,ISO8859-10", true, false) #: Encoding
  ISO8859_10 = ISO_8859_10 #: Encoding
  ISO_8859_11 = __new("ISO-8859-11", "ISO-8859-11,ISO8859-11", true, false) #: Encoding
  ISO8859_11 = ISO_8859_11 #: Encoding
  ISO_8859_13 = __new("ISO-8859-13", "ISO-8859-13,ISO8859-13", true, false) #: Encoding
  ISO8859_13 = ISO_8859_13 #: Encoding
  ISO_8859_14 = __new("ISO-8859-14", "ISO-8859-14,ISO8859-14", true, false) #: Encoding
  ISO8859_14 = ISO_8859_14 #: Encoding
  ISO_8859_15 = __new("ISO-8859-15", "ISO-8859-15,ISO8859-15", true, false) #: Encoding
  ISO8859_15 = ISO_8859_15 #: Encoding
  ISO_8859_16 = __new("ISO-8859-16", "ISO-8859-16,ISO8859-16", true, false) #: Encoding
  ISO8859_16 = ISO_8859_16 #: Encoding
  KOI8_R = __new("KOI8-R", "KOI8-R,CP878", true, false) #: Encoding
  CP878 = KOI8_R #: Encoding
  KOI8_U = __new("KOI8-U", "KOI8-U", true, false) #: Encoding
  SHIFT_JIS = __new("Shift_JIS", "Shift_JIS", true, false) #: Encoding
  Shift_JIS = SHIFT_JIS #: Encoding
  WINDOWS_1250 = __new("Windows-1250", "Windows-1250,CP1250", true, false) #: Encoding
  CP1250 = WINDOWS_1250 #: Encoding
  Windows_1250 = WINDOWS_1250 #: Encoding
  WINDOWS_1251 = __new("Windows-1251", "Windows-1251,CP1251", true, false) #: Encoding
  CP1251 = WINDOWS_1251 #: Encoding
  Windows_1251 = WINDOWS_1251 #: Encoding
  WINDOWS_1252 = __new("Windows-1252", "Windows-1252,CP1252", true, false) #: Encoding
  CP1252 = WINDOWS_1252 #: Encoding
  Windows_1252 = WINDOWS_1252 #: Encoding
  WINDOWS_1253 = __new("Windows-1253", "Windows-1253,CP1253", true, false) #: Encoding
  CP1253 = WINDOWS_1253 #: Encoding
  Windows_1253 = WINDOWS_1253 #: Encoding
  WINDOWS_1254 = __new("Windows-1254", "Windows-1254,CP1254", true, false) #: Encoding
  CP1254 = WINDOWS_1254 #: Encoding
  Windows_1254 = WINDOWS_1254 #: Encoding
  WINDOWS_1257 = __new("Windows-1257", "Windows-1257,CP1257", true, false) #: Encoding
  CP1257 = WINDOWS_1257 #: Encoding
  Windows_1257 = WINDOWS_1257 #: Encoding
  IBM437 = __new("IBM437", "IBM437,CP437", true, false) #: Encoding
  CP437 = IBM437 #: Encoding
  CP850 = __new("CP850", "CP850,IBM850", true, false) #: Encoding
  IBM850 = CP850 #: Encoding
  IBM852 = __new("IBM852", "IBM852", true, false) #: Encoding
  CP852 = __new("CP852", "CP852", true, false) #: Encoding
  IBM855 = __new("IBM855", "IBM855", true, false) #: Encoding
  CP855 = __new("CP855", "CP855", true, false) #: Encoding
  IBM860 = __new("IBM860", "IBM860,CP860", true, false) #: Encoding
  CP860 = IBM860 #: Encoding
  IBM862 = __new("IBM862", "IBM862,CP862", true, false) #: Encoding
  CP862 = IBM862 #: Encoding
  IBM863 = __new("IBM863", "IBM863,CP863", true, false) #: Encoding
  CP863 = IBM863 #: Encoding
  IBM865 = __new("IBM865", "IBM865,CP865", true, false) #: Encoding
  CP865 = IBM865 #: Encoding
  IBM866 = __new("IBM866", "IBM866,CP866", true, false) #: Encoding
  CP866 = IBM866 #: Encoding
  MACCYRILLIC = __new("macCyrillic", "macCyrillic", true, false) #: Encoding
  MacCyrillic = MACCYRILLIC #: Encoding
  MACROMAN = __new("macRoman", "macRoman", true, false) #: Encoding
  MacRoman = MACROMAN #: Encoding
  CP950 = __new("CP950", "CP950", true, false) #: Encoding
  CP51932 = __new("CP51932", "CP51932", true, false) #: Encoding
  GB2312 = __new("GB2312", "GB2312,EUC-CN,eucCN", true, false) #: Encoding
  EUC_CN = GB2312 #: Encoding
  EucCN = GB2312 #: Encoding
  EUCCN = GB2312 #: Encoding
  WINDOWS_1256 = __new("Windows-1256", "Windows-1256,CP1256", true, false) #: Encoding
  Windows_1256 = WINDOWS_1256 #: Encoding
  CP1256 = WINDOWS_1256 #: Encoding
  WINDOWS_1255 = __new("Windows-1255", "Windows-1255,CP1255", true, false) #: Encoding
  Windows_1255 = WINDOWS_1255 #: Encoding
  CP1255 = WINDOWS_1255 #: Encoding
  TIS_620 = __new("TIS-620", "TIS-620", true, false) #: Encoding
  WINDOWS_874 = __new("Windows-874", "Windows-874,CP874", true, false) #: Encoding
  Windows_874 = WINDOWS_874 #: Encoding
  CP874 = WINDOWS_874 #: Encoding
  LIST__ = [ASCII_8BIT, UTF_8, US_ASCII, UTF_16BE, UTF_16LE, UTF_32BE, UTF_32LE, UTF_16, UTF_32, EUC_JP, WINDOWS_31J, BIG5, CP949, EUC_KR, GB18030, GBK, ISO_8859_1, ISO_8859_2, ISO_8859_3, ISO_8859_4, ISO_8859_5, ISO_8859_6, ISO_8859_7, ISO_8859_8, ISO_8859_9, ISO_8859_10, ISO_8859_11, ISO_8859_13, ISO_8859_14, ISO_8859_15, ISO_8859_16, KOI8_R, KOI8_U, SHIFT_JIS, WINDOWS_1250, WINDOWS_1251, WINDOWS_1252, WINDOWS_1253, WINDOWS_1254, WINDOWS_1257, IBM437, CP850, IBM852, CP852, IBM855, CP855, IBM860, IBM862, IBM863, IBM865, IBM866, MACCYRILLIC, MACROMAN, CP950, CP51932, GB2312, WINDOWS_1256, WINDOWS_1255, TIS_620, WINDOWS_874] #: Array[Encoding]
  NAME_LIST__ = %w[ASCII-8BIT UTF-8 US-ASCII UTF-16BE UTF-16LE UTF-32BE UTF-32LE UTF-16 UTF-32 EUC-JP Windows-31J Big5 CP949 EUC-KR GB18030 GBK ISO-8859-1 ISO-8859-2 ISO-8859-3 ISO-8859-4 ISO-8859-5 ISO-8859-6 ISO-8859-7 ISO-8859-8 ISO-8859-9 ISO-8859-10 ISO-8859-11 ISO-8859-13 ISO-8859-14 ISO-8859-15 ISO-8859-16 KOI8-R KOI8-U Shift_JIS Windows-1250 Windows-1251 Windows-1252 Windows-1253 Windows-1254 Windows-1257 BINARY IBM437 CP437 CP850 IBM850 IBM852 CP852 IBM855 CP855 IBM860 CP860 IBM862 CP862 IBM863 CP863 IBM865 CP865 IBM866 CP866 macCyrillic macRoman CP950 eucJP CP51932 eucKR GB2312 EUC-CN eucCN CP936 ISO8859-1 ISO8859-2 ISO8859-3 ISO8859-4 ISO8859-5 ISO8859-6 Windows-1256 CP1256 ISO8859-7 ISO8859-8 Windows-1255 CP1255 ISO8859-9 ISO8859-10 ISO8859-11 TIS-620 Windows-874 CP874 ISO8859-13 ISO8859-14 ISO8859-15 ISO8859-16 CP878 ASCII ANSI_X3.4-1968 646 CP65001 UCS-2BE UCS-4BE UCS-4LE CP932 csWindows31J SJIS PCK CP1250 CP1251 CP1252 CP1253 CP1254 CP1257 locale external filesystem] #: Array[String]
  # END scripts/gen-encoding-fixes

  #: () -> Array[Encoding]
  def self.list = LIST__.dup

  #: () -> Array[String]
  def self.name_list = NAME_LIST__.dup

  #: () -> Hash[String, String]
  def self.aliases
    h = {} #: Hash[String, String]
    LIST__.each { |e| e.names.drop(1).each { |a| h[a] = e.name } }
    h
  end

  # "internal" with no default_internal is nil in MRI; Encoding? would make every find's result optional, so it raises here.
  #: (untyped) -> Encoding
  def self.find(name)
    return name if name.is_a?(Encoding)

    n = __name_arg(name)
    d = n.casecmp?("internal") ? default_internal : nil
    return d if d
    return default_external if n.casecmp?("external")

    found = LIST__.find { |e| e.names.any? { |x| x.casecmp?(n) } }
    raise ArgumentError, "unknown encoding name - #{n}" unless found

    found
  end

  #: (untyped) -> String
  def self.__name_arg(x) = %x{ return String(rbEncNameArg(x)) }

  #: () -> Encoding
  def self.default_external = %x{
    if e := rbEncDefaultExternal.Load(); e != nil {
      return e
    }
    return Encoding_UTF_8
  }

  # Stored for default_external to answer; IO keeps reading and writing UTF-8 (decision 136).
  #: (untyped) -> untyped
  def self.default_external=(enc)
    e = find(enc)
    __set_defaults(e, default_internal)
    enc
  end

  #: () -> Encoding?
  def self.default_internal = %x{
    if e := rbEncDefaultInternal.Load(); e != nil {
      return Ref(e)
    }
    return nil
  }

  # Stored, and encode with no target encodes to it, as MRI's does; IO does not convert to it (decision 136).
  #: (untyped) -> untyped
  def self.default_internal=(enc)
    e = enc.nil? ? nil : find(enc)
    __set_defaults(default_external, e)
    enc
  end

  #: (Encoding, Encoding?) -> void
  def self.__set_defaults(ext, intern) = %x{
    rbEncDefaultExternal.Store(ext)
    rbEncDefaultInternal.Store(nil)
    if intern != nil {
      rbEncDefaultInternal.Store(*intern)
    }
  }

  # MRI's rb_enc_compatible over derived encodings: the encoding a concatenation of a and b would have, or nil.
  #: (untyped, untyped) -> Encoding?
  def self.compatible?(a, b)
    ea = __enc_of(a)
    eb = __enc_of(b)
    return nil unless ea && eb
    return ea if ea.equal?(eb)

    sa = a.is_a?(String)
    sb = b.is_a?(String)
    return ea if sb && b.to_s.empty?
    return (ea.ascii_compatible? && b.to_s.ascii_only? ? ea : eb) if sa && sb && a.to_s.empty?
    return nil unless ea.ascii_compatible? && eb.ascii_compatible?
    return ea if !sb && eb.equal?(US_ASCII)
    return eb if !sa && ea.equal?(US_ASCII)
    return nil unless sa || sb

    __compatible_str(sa ? a.to_s : b.to_s, sa ? ea : eb, sa ? eb : ea, sa && sb ? b.to_s : nil)
  end

  #: (String, Encoding, Encoding, String?) -> Encoding?
  def self.__compatible_str(s1, e1, e2, s2)
    a1 = s1.ascii_only?
    if s2
      a2 = s2.ascii_only?
      return e2 if a1 && !a2
      return e1 if a2
    end
    a1 ? e2 : nil
  end

  #: (untyped) -> Encoding?
  def self.__enc_of(x) = %x{
    switch v := rbUnbox(x).(type) {
    case *Encoding:
      return Ref(v)
    case String:
      if utf8.ValidString(string(v)) {
        return Ref(Encoding_UTF_8)
      }
      return Ref(Encoding_ASCII_8BIT)
    }
    return nil
  }

  # Raised only for the encodings and conversions decision 136 leaves out.
  class CompatibilityError < EncodingError; end

  class ConverterNotFoundError < EncodingError; end

  # Encoding::Converter on String#encode's transcoder (decision 452): each convert transcodes what it is given, an
  # incomplete multibyte tail waits for the next call; errors raise as encode's do and are kept in last_error.
  class Converter
    # @rbs @source: Encoding
    # @rbs @destination: Encoding
    # @rbs @invalid: Symbol?
    # @rbs @undef: Symbol?
    # @rbs @replace: String?
    # @rbs @xml: Symbol?
    # @rbs @universal: bool
    # @rbs @crlf: bool
    # @rbs @cr: bool
    # @rbs @pending: String
    # @rbs @out_prefix: String
    # @rbs @last_error: Exception?
    # @rbs @errinfo: Array[untyped]

    INVALID_MASK = 0x0f
    INVALID_REPLACE = 0x02
    UNDEF_MASK = 0xf0
    UNDEF_REPLACE = 0x20
    UNDEF_HEX_CHARREF = 0x30
    PARTIAL_INPUT = 0x10000
    AFTER_OUTPUT = 0x20000
    UNIVERSAL_NEWLINE_DECORATOR = 0x100
    CRLF_NEWLINE_DECORATOR = 0x1000
    CR_NEWLINE_DECORATOR = 0x2000
    XML_TEXT_DECORATOR = 0x4000
    XML_ATTR_CONTENT_DECORATOR = 0x8000
    XML_ATTR_QUOTE_DECORATOR = 0x100000

    # `undef` is a keyword Ruby code cannot read as a local, so the keywords pass through Go to __setup.
    #: (untyped, untyped, ?invalid: Symbol?, ?undef: Symbol?, ?replace: String?, ?xml: Symbol?, ?universal_newline: bool?, ?crlf_newline: bool?, ?cr_newline: bool?, ?newline: Symbol?) -> void
    def initialize(src, dst, invalid: nil, undef: nil, replace: nil, xml: nil, universal_newline: nil, crlf_newline: nil, cr_newline: nil, newline: nil) = %x{
      self.__Setup(src, dst, invalid, undef, replace, xml, universal_newline, crlf_newline, cr_newline, newline)
    }

    #: (untyped, untyped, Symbol?, Symbol?, String?, Symbol?, bool?, bool?, bool?, Symbol?) -> void
    def __setup(src, dst, invalid, undf, replace, xml, universal_newline, crlf_newline, cr_newline, newline)
      @source = Converter.__enc(src, src, dst)
      @destination = Converter.__enc(dst, src, dst)
      @invalid = invalid
      @undef = undf
      @replace = replace
      @xml = xml
      @universal = universal_newline == true || newline == :universal
      @crlf = crlf_newline == true || newline == :crlf
      @cr = cr_newline == true || newline == :cr
      @pending = +""
      @out_prefix = +""
      @last_error = nil
      @errinfo = [:source_buffer_empty, nil, nil, nil, nil]
    end

    # An encoding by name or object, or ConverterNotFoundError naming both ends, as MRI's.
    #: (untyped, untyped, untyped) -> Encoding
    def self.__enc(x, src, dst)
      Encoding.find(x)
    rescue ArgumentError
      raise ConverterNotFoundError, "code converter not found (#{src} to #{dst})"
    end

    #: () -> Encoding
    def source_encoding = @source

    #: () -> Encoding
    def destination_encoding = @destination

    #: () -> Exception?
    def last_error = @last_error

    #: () -> String
    def replacement = @replace || (@destination.name.start_with?("UTF-") ? "\uFFFD" : "?")

    #: (String) -> String
    def replacement=(s)
      @replace = s
    end

    #: () -> Array[untyped]
    def convpath = Converter.search_convpath(@source, @destination, xml: @xml, universal_newline: @universal, crlf_newline: @crlf, cr_newline: @cr)

    #: () -> String
    def inspect = "#<Encoding::Converter: #{@source.name} to #{@destination.name}>"

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Converter) && other.inspect == inspect

    # The conversion of s; a multibyte character cut at s's end waits for the next call (or finish).
    #: (String) -> String
    def convert(s)
      src = @pending + s
      @pending = +""
      keep = __incomplete_tail(src)
      if keep > 0
        @pending = src.byteslice(src.bytesize - keep, keep) || ""
        src = src.byteslice(0, src.bytesize - keep) || ""
      end
      __run(src)
    end

    # The rest: an incomplete character left by convert raises (or is replaced) here, as MRI's finish.
    #: () -> String
    def finish
      src = @pending
      @pending = +""
      return "" if src.empty?

      __run(src)
    end

    #: (String) -> String
    def __run(src)
      out = @out_prefix + src.encode(@destination, @source, invalid: @invalid, undef: @undef, replace: @replace, xml: @xml,
                                                           universal_newline: @universal, crlf_newline: @crlf, cr_newline: @cr)
      @out_prefix = +""
      @last_error = nil
      @errinfo = [:finished, nil, nil, nil, nil]
      out
    rescue InvalidByteSequenceError => ie
      @last_error = ie
      @errinfo = [:invalid_byte_sequence, ie.source_encoding_name, ie.destination_encoding_name, ie.error_bytes, ie.readagain_bytes || ""]
      raise ie
    rescue UndefinedConversionError => ue
      @last_error = ue
      @errinfo = [:undefined_conversion, ue.source_encoding_name, ue.destination_encoding_name, ue.error_char, ""]
      raise ue
    end

    # Bytes at s's end that start a character the encoding needs more bytes for (UTF-8 only; other sources
    # convert whole).
    #: (String) -> Integer
    def __incomplete_tail(s)
      return 0 unless @source == Encoding::UTF_8

      n = s.bytesize
      k = 1
      while k <= 3 && k <= n
        c = s.getbyte(n - k) || 0
        if c & 0xC0 != 0x80
          need = if c >= 0xF0 then 4 elsif c >= 0xE0 then 3 elsif c >= 0xC0 then 2 else 1 end
          return need > k ? k : 0
        end
        k += 1
      end
      0
    end

    #: () -> Array[untyped]
    def primitive_errinfo = @errinfo

    #: (?Integer?) -> String
    def putback(_n = nil) = +""

    #: (String) -> nil
    def insert_output(s)
      @out_prefix += s.encode(@destination)
      nil
    end

    # The ASCII-compatible encoding a non-ASCII-compatible one converts through: UTF-8 for the UTF-16/32 family.
    #: (untyped) -> Encoding?
    def self.asciicompat_encoding(enc)
      e = Encoding.find(enc)
      return nil if e.ascii_compatible? || e.dummy?

      e.name.start_with?("UTF-16", "UTF-32") ? Encoding::UTF_8 : nil
    rescue ArgumentError
      nil
    end

    # The steps a conversion takes, as pairs of Encodings (through UTF-8 unless an end is UTF-8), then any decorators.
    #: (untyped, untyped, ?xml: Symbol?, ?universal_newline: bool?, ?crlf_newline: bool?, ?cr_newline: bool?, ?newline: Symbol?) -> Array[untyped]
    def self.search_convpath(src, dst, xml: nil, universal_newline: nil, crlf_newline: nil, cr_newline: nil, newline: nil)
      a = __enc(src, src, dst)
      b = __enc(dst, src, dst)
      path = [] #: Array[untyped]
      if a != b
        if a == Encoding::UTF_8 || b == Encoding::UTF_8
          path << [a, b]
        else
          path << [a, Encoding::UTF_8] << [Encoding::UTF_8, b]
        end
      end
      path << "universal_newline" if universal_newline || newline == :universal
      path << "xml_text_escape" if xml == :text
      path << "xml_attr_content_escape" << "xml_attr_quote" if xml == :attr
      path << "crlf_newline" if crlf_newline || newline == :crlf
      path << "cr_newline" if cr_newline || newline == :cr
      path
    end
  end

  class UndefinedConversionError < EncodingError
    # @rbs @source_encoding_name: String?
    # @rbs @destination_encoding_name: String?
    # @rbs @error_char: String?

    #: (?String?, ?String?, ?String?, ?String?) -> void
    def initialize(message = nil, src = nil, dst = nil, char = nil)
      super(message)
      @source_encoding_name = src
      @destination_encoding_name = dst
      @error_char = char
    end

    #: () -> String?
    def source_encoding_name = @source_encoding_name

    #: () -> String?
    def destination_encoding_name = @destination_encoding_name

    #: () -> Encoding?
    def source_encoding = (n = @source_encoding_name) ? Encoding.find(n) : nil

    #: () -> Encoding?
    def destination_encoding = (n = @destination_encoding_name) ? Encoding.find(n) : nil

    #: () -> String?
    def error_char = @error_char
  end

  class InvalidByteSequenceError < EncodingError
    # @rbs @source_encoding_name: String?
    # @rbs @destination_encoding_name: String?
    # @rbs @error_bytes: String?
    # @rbs @readagain_bytes: String?
    # @rbs @incomplete: bool?

    #: (?String?, ?String?, ?String?, ?String?, ?String?, ?bool?) -> void
    def initialize(message = nil, src = nil, dst = nil, bytes = nil, again = nil, incomplete = nil)
      super(message)
      @source_encoding_name = src
      @destination_encoding_name = dst
      @error_bytes = bytes
      @readagain_bytes = again
      @incomplete = incomplete
    end

    #: () -> String?
    def source_encoding_name = @source_encoding_name

    #: () -> String?
    def destination_encoding_name = @destination_encoding_name

    #: () -> Encoding?
    def source_encoding = (n = @source_encoding_name) ? Encoding.find(n) : nil

    #: () -> Encoding?
    def destination_encoding = (n = @destination_encoding_name) ? Encoding.find(n) : nil

    #: () -> String?
    def error_bytes = @error_bytes

    #: () -> String?
    def readagain_bytes = @readagain_bytes

    #: () -> bool
    def incomplete_input? = @incomplete == true
  end
end

class String
  # Derived, not stored (decision 136): valid UTF-8 is UTF-8, anything else ASCII-8BIT.
  #: () -> Encoding
  def encoding = valid_encoding? ? Encoding::UTF_8 : Encoding::ASCII_8BIT

  # The bytes stay as they are; the name is still checked, as MRI's is.
  #: (untyped) -> String
  def force_encoding(enc)
    Encoding.find(enc)
    self
  end

  # force_encoding with a literal BINARY/ASCII-8BIT (decision 352).
  # @binary
  #: (untyped) -> String
  def __force_encoding_binary(enc) = self

  # The compiler calls these on a String it knows is binary (decision 352): each byte is a character.
  #: () -> Integer
  def __bin_size = bytesize

  #: () -> Integer
  def __bin_length = bytesize

  # @binary
  #: (Integer) -> String?
  def __bin_idx(i) = %x{
    if v, ok := rbBinAt(self, i); ok {
      return &v
    }
    return nil
  }

  # @binary
  #: (Integer, Integer) -> String?
  def __bin_idx_2(start, count) = %x{
    if v, ok := rbBinSub(self, start, count); ok {
      return &v
    }
    return nil
  }

  # @binary
  #: (Range[Integer]) -> String?
  def __bin_idx_range(r) = %x{
    if v, ok := rbBinRange(self, r); ok {
      return &v
    }
    return nil
  }

  #: () -> Array[String]
  def __bin_chars = %x{
    out := &Array[String]{s: make([]String, len(self))}
    for i := range len(self) {
      out.s[i] = self[i : i+1]
    }
    return out
  }

  # A separator other than "" splits the same bytes either way.
  #: (?String?) -> Array[String]
  def __bin_split(sep = nil) = sep == "" ? __bin_chars : split(sep)

  #: () { (String) -> void } -> void
  def __bin_each_char = %x{
    return func(yield func(String) bool) {
      for i := range len(self) {
        if !yield(self[i : i+1]) {
          return
        }
      }
    }
  }

  #: () -> Enumerator[String]
  def __bin_each_char_enum = %x{ return rbEnumOf(self.__BinEachChar(), any(self), "each_char", rbSizeOf(Integer(len(self))), nil) }

  #: () -> Integer
  def __bin_ord = %x{
    if self == "" {
      panic(NewArgumentError(Ref[String]("empty string")))
    }
    return Integer(self[0])
  }

  # @binary
  #: () -> String
  def __bin_reverse = %x{
    b := []byte(string(self))
    slices.Reverse(b)
    return String(b)
  }

  #: (String) -> Integer?
  def __bin_index(s) = byteindex(s)

  #: (String) -> Integer?
  def __bin_rindex(s) = byterindex(s)

  # @binary
  #: (String, String) -> String
  def __bin_tr(from, to) = %x{ rbTr(self, string(from), string(to), true, true, false) }

  # @binary
  #: (String) -> String
  def __bin_delete(chars) = __bin_tr(chars, "")

  #: (String) -> Integer
  def __bin_count(chars) = %x{
    set, _ := rbLatin1(string(chars)) // its bytes are characters too
    in := rbCharSet(set)
    n := 0
    for i := range len(self) {
      if in(rune(self[i])) {
        n++
      }
    }
    return Integer(n)
  }

  #: () -> Encoding
  def __bin_encoding = Encoding::ASCII_8BIT

  #: () -> bool
  def __bin_valid_encoding_q = true

  #: () -> String
  def __bin_inspect = %x{ String(rbDump(string(self), true)) }

  #: () -> bool
  def valid_encoding? = %x{ Boolean(utf8.ValidString(string(self))) }

  #: (?String?) -> String
  def scrub(repl = nil) = %x{
    r := "\\uFFFD"
    if repl != nil {
      r = string(*repl)
    }
    return rbNewStr(self, String(rbScrub(string(self), r)))
  }

  #: () { (String) -> String } -> String
  def __scrub_block
    parts = __scrub_parts
    out = ""
    i = 0
    while i < parts.size
      p = parts[i] || ""
      out += i.odd? ? yield(p) : p
      i += 1
    end
    out
  end

  #: () -> Array[String]
  def __scrub_parts = %x{ return rbStrs(rbScrubParts(string(self))) }

  #: (?untyped, ?untyped, ?invalid: Symbol?, ?undef: Symbol?, ?replace: String?, ?xml: Symbol?, ?universal_newline: bool?, ?crlf_newline: bool?, ?cr_newline: bool?) -> String
  def encode(to = nil, from = nil, invalid: nil, undef: nil, replace: nil, xml: nil, universal_newline: nil, crlf_newline: nil, cr_newline: nil) = %x{
    flag := func(b *Boolean) bool { return b != nil && bool(*b) }
    o := rbEncOptions(invalid, undef, replace, xml, flag(universal_newline), flag(crlf_newline), flag(cr_newline))
    dflt := ""
    if e := rbEncDefaultInternal.Load(); e != nil {
      dflt = e.name
    }
    return rbStrEncode(self, to, from, dflt, o)
  }

  #: (?Symbol) -> String
  def unicode_normalize(form = :nfc) = %x{ return String(rbUnicodeNormalize(string(self), string(form))) }

  #: (?Symbol) -> bool
  def unicode_normalized?(form = :nfc) = %x{ Boolean(rbUnicodeNormalize(string(self), string(form)) == string(self)) }
end

class Integer
  #: (untyped) -> String
  def __chr_1(enc) = %x{ return rbIntChr(int(self), enc) }
end
