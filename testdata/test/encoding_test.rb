# rbs_inline: enabled

require "minitest/autorun"
require "tmpdir"

# Encoding without a tag on String (decision 136). MRI loads some encodings lazily and inspects them as "(autoload)" until then, so those are compared by name.
module EncodingTests
  class EncodingClassTest < Minitest::Test
    def test_constants
      assert_equal "UTF-8", Encoding::UTF_8.name
      assert_equal "UTF-8", Encoding::UTF_8.to_s
      assert_equal ["UTF-8", "CP65001", "locale", "external", "filesystem"], Encoding::UTF_8.names
      assert_equal ["ASCII-8BIT", "BINARY"], Encoding::BINARY.names
      assert_equal ["US-ASCII", "ASCII", "ANSI_X3.4-1968", "646"], Encoding::US_ASCII.names
      assert_equal ["ISO-8859-1", "ISO8859-1"], Encoding::ISO_8859_1.names
      assert_equal ["UTF-16BE", "UCS-2BE"], Encoding::UTF_16BE.names
      assert_equal ["UTF-32LE", "UCS-4LE"], Encoding::UTF_32LE.names
      assert_same Encoding::ASCII_8BIT, Encoding::BINARY
      assert_same Encoding::US_ASCII, Encoding::ASCII
      assert_equal "ISO-8859-1", Encoding::ISO8859_1.name
    end

    def test_inspect
      assert_equal "#<Encoding:UTF-8>", Encoding::UTF_8.inspect
      assert_equal "#<Encoding:BINARY (ASCII-8BIT)>", Encoding::BINARY.inspect
      assert_equal "#<Encoding:US-ASCII>", Encoding::US_ASCII.inspect
      assert_equal "#<Encoding:UTF-16 (dummy)>", Encoding::UTF_16.inspect
      assert_equal "#<Encoding:UTF-16LE>", Encoding::UTF_16LE.inspect
    end

    def test_flags
      encs = [Encoding::UTF_8, Encoding::BINARY, Encoding::US_ASCII, Encoding::ISO_8859_1, Encoding::UTF_16LE, Encoding::UTF_32BE, Encoding::UTF_16, Encoding::UTF_32]
      assert_equal [true, true, true, true, false, false, false, false], encs.map(&:ascii_compatible?)
      assert_equal [false, false, false, false, false, false, true, true], encs.map(&:dummy?)
    end

    def test_find
      assert_same Encoding::UTF_8, Encoding.find("utf-8")
      assert_same Encoding::UTF_8, Encoding.find(Encoding::UTF_8)
      assert_same Encoding::BINARY, Encoding.find("binary")
      assert_same Encoding::US_ASCII, Encoding.find("ASCII")
      assert_same Encoding::UTF_8, Encoding.find("locale")
      assert_same Encoding::UTF_8, Encoding.find("CP65001")
      assert_equal "ISO-8859-1", Encoding.find("iso8859-1").name
      e = assert_raises(ArgumentError) { Encoding.find("bogus") }
      assert_equal "unknown encoding name - bogus", e.message
    end

    def test_lists
      assert_equal ["ASCII-8BIT", "UTF-8", "US-ASCII", "UTF-16BE", "UTF-16LE"], Encoding.list.first(5).map(&:name)
      assert_equal true, Encoding.list.include?(Encoding::ISO_8859_1)
      assert_equal ["ASCII-8BIT", "UTF-8", "US-ASCII"], Encoding.name_list.first(3)
      assert_equal true, Encoding.name_list.include?("BINARY")
      assert_equal "ASCII-8BIT", Encoding.aliases["BINARY"]
      assert_equal "US-ASCII", Encoding.aliases["646"]
    end

    def test_defaults
      assert_same Encoding::UTF_8, Encoding.default_external
      assert_nil Encoding.default_internal
      begin
        Encoding.default_external = "ISO-8859-1"
        assert_equal "ISO-8859-1", Encoding.default_external.name
        Encoding.default_internal = Encoding::UTF_16LE
        assert_same Encoding::UTF_16LE, Encoding.default_internal
        assert_equal [104, 0, 105, 0], "hi".encode.bytes
        assert_equal [233], "é".encode("external").bytes
        assert_equal [233, 0], "é".encode("internal").bytes
      ensure
        Encoding.default_external = Encoding::UTF_8
        Encoding.default_internal = nil
      end
      assert_same Encoding::UTF_8, Encoding.default_external
      assert_nil Encoding.default_internal
    end

    def test_compatible
      assert_same Encoding::BINARY, Encoding.compatible?("a", "\xff".b)
      assert_nil Encoding.compatible?("é", "\xff".b)
      assert_same Encoding::UTF_8, Encoding.compatible?("é", "a".b)
      assert_same Encoding::UTF_8, Encoding.compatible?(Encoding::UTF_8, Encoding::US_ASCII)
      assert_same Encoding::UTF_8, Encoding.compatible?(Encoding::US_ASCII, Encoding::UTF_8)
      assert_nil Encoding.compatible?(Encoding::UTF_8, Encoding::UTF_16LE)
      assert_same Encoding::UTF_8, Encoding.compatible?("é", Encoding::US_ASCII)
      assert_same Encoding::UTF_8, Encoding.compatible?("é", "é")
      assert_same Encoding::BINARY, Encoding.compatible?("", "\xff".b)
      assert_same Encoding::UTF_8, Encoding.compatible?("é", "")
      assert_nil Encoding.compatible?(1, 2)
    end

    def test_errors_tree
      assert_equal EncodingError, Encoding::UndefinedConversionError.superclass
      assert_equal EncodingError, Encoding::InvalidByteSequenceError.superclass
      assert_equal EncodingError, Encoding::ConverterNotFoundError.superclass
      assert_equal EncodingError, Encoding::CompatibilityError.superclass
      assert_equal StandardError, EncodingError.superclass
      err = assert_raises(Encoding::CompatibilityError) { raise Encoding::CompatibilityError, "mixed" }
      assert_equal "mixed", err.message
    end
  end

  class StringEncodingTest < Minitest::Test
    def test_encoding
      assert_same Encoding::UTF_8, "abc".encoding
      assert_same Encoding::UTF_8, "héllo".encoding
      assert_same Encoding::BINARY, "\xff".b.encoding
      assert_same Encoding::BINARY, [255].pack("C").encoding
      assert_same Encoding::UTF_8, [233].pack("U").encoding
      assert_equal 6, "héllo".b.bytesize
    end

    def test_force_encoding
      s = "\xc3\xa9".b.force_encoding("UTF-8")
      assert_equal "é", s
      assert_equal true, s.valid_encoding?
      assert_equal "é", "é".dup.force_encoding(Encoding::UTF_8)
      e = assert_raises(ArgumentError) { "x".dup.force_encoding("bogus") }
      assert_equal "unknown encoding name - bogus", e.message
    end

    def test_valid_and_ascii_only
      assert_equal [false, true, true], ["\xff".valid_encoding?, "é".valid_encoding?, "abc".valid_encoding?]
      assert_equal [true, false, false, true], ["abc".ascii_only?, "é".ascii_only?, "\xff".ascii_only?, "".ascii_only?]
    end

    def test_scrub
      assert_equal "a\uFFFDb\uFFFDc", "a\xffb\xe3\x81c".scrub
      assert_equal "a?b?c", "a\xffb\xe3\x81c".scrub("?")
      assert_equal "abc", "abc".scrub
      assert_equal "***", "\xff\xfe\xf0\x9f\x98".scrub("*")
      assert_equal "*****", "\xc0\x80\xed\xa0\x80".scrub("*")
      assert_equal "a<ff>b<e381>c", "a\xffb\xe3\x81c".scrub { |bad| "<#{bad.unpack1('H*')}>" }
    end

    def test_encode_targets
      assert_equal [99, 97, 102, 233], "café".encode("ISO-8859-1").bytes
      assert_equal [233], "é".encode(Encoding::ISO_8859_1).bytes
      assert_equal "café", "caf\xe9".encode("UTF-8", "ISO-8859-1")
      assert_equal [254, 255, 0, 104, 0, 105], "hi".encode("UTF-16").bytes
      assert_equal [0, 0, 254, 255, 0, 0, 0, 104, 0, 0, 0, 105], "hi".encode("UTF-32").bytes
      assert_equal [], "".encode("UTF-16").bytes
      assert_equal [0, 104, 32, 172, 216, 61, 222, 0], "h€😀".encode("UTF-16BE").bytes
      assert_equal [104, 0, 0, 0, 172, 32, 0, 0, 0, 246, 1, 0], "h€😀".encode("UTF-32LE").bytes
      assert_equal "h€😀", "h€😀".encode("UTF-16LE").encode("UTF-8", "UTF-16LE")
      assert_equal "h", "\xFF\xFEh\x00".encode("UTF-8", "UTF-16")
      assert_equal "h", "\xFE\xFF\x00h".encode("UTF-8", "UTF-16")
      assert_equal "h", "\xFF\xFE\x00\x00h\x00\x00\x00".encode("UTF-8", "UTF-32")
      assert_equal [0, 233, 255, 253], "é\xff".encode("UTF-16LE", invalid: :replace).encode("UTF-16BE", "UTF-16LE").bytes
      assert_equal [233, 0], "\xe9".encode("UTF-16LE", "ISO-8859-1").bytes
      assert_equal "a", "a".b.encode("US-ASCII")
      assert_equal "a\xFF", "a\xff".encode
      assert_equal "\xFF", "\xff".encode("UTF-8")
    end

    def test_encode_replace
      assert_equal "caf?", "café".encode("US-ASCII", undef: :replace)
      assert_equal "caf?", "café".encode("US-ASCII", undef: :replace, replace: "?")
      assert_equal "cafe", "café".encode("US-ASCII", undef: :replace, replace: "e")
      assert_equal "a?", "a€".encode("ISO-8859-1", undef: :replace)
      assert_equal [233], "€".encode("ISO-8859-1", undef: :replace, replace: "é").bytes
      assert_equal "a\uFFFDb\uFFFDc", "a\xffb\xe3\x81c".encode("UTF-8", invalid: :replace)
      assert_equal "a?b?c", "a\xffb\xe3\x81c".encode("UTF-8", invalid: :replace, replace: "?")
      assert_equal "a\uFFFDb", "a\xffb".encode(invalid: :replace)
      assert_equal [97, 0, 253, 255, 98, 0], "a\xffb".encode("UTF-16LE", invalid: :replace).bytes
      assert_equal [253, 255], "\xff".b.encode("UTF-16LE", "BINARY", undef: :replace).bytes
      assert_equal "a\uFFFDb", "a\xffb".encode("UTF-8", "US-ASCII", invalid: :replace)
    end

    def test_encode_decorators
      assert_equal "&lt;a&amp;b&gt;\"'", "<a&b>\"'".encode(xml: :text)
      assert_equal "\"&lt;a&amp;b&gt;&quot;&apos;\"", "<a&b>\"'".encode(xml: :attr)
      assert_equal "&lt;&#xE9;&gt;", "<é>".encode("US-ASCII", xml: :text)
      assert_equal [38, 0, 108, 0, 116, 0, 59, 0, 233, 0], "<é".encode("UTF-16LE", xml: :text).bytes
      assert_equal "a\nb\nc\n", "a\r\nb\rc\n".encode(universal_newline: true)
      assert_equal "a\n", "a\r".encode(universal_newline: true)
      assert_equal "a\nb", "a\r\nb".encode("UTF-16LE").encode("UTF-8", "UTF-16LE", universal_newline: true)
      assert_equal "a\r\nb", "a\nb".encode(crlf_newline: true)
      assert_equal "a\rb", "a\nb".encode(cr_newline: true)
      assert_equal "&lt;\r\n", "<\n".encode(xml: :text, crlf_newline: true)
    end

    def test_undefined_conversion
      e = assert_raises(Encoding::UndefinedConversionError) { "café".encode("US-ASCII") }
      assert_equal "U+00E9 from UTF-8 to US-ASCII", e.message
      assert_equal ["UTF-8", "US-ASCII", "é"], [e.source_encoding_name, e.destination_encoding_name, e.error_char]
      assert_same Encoding::UTF_8, e.source_encoding
      assert_same Encoding::US_ASCII, e.destination_encoding
      assert_equal "U+20AC from UTF-8 to ISO-8859-1", assert_raises(Encoding::UndefinedConversionError) { "a€".encode("ISO-8859-1") }.message
      assert_equal "U+1F600 from UTF-8 to US-ASCII", assert_raises(Encoding::UndefinedConversionError) { "😀".encode("US-ASCII") }.message
      assert_equal "U+00E9 from UTF-8 to ASCII-8BIT", assert_raises(Encoding::UndefinedConversionError) { "é".encode("BINARY") }.message
      assert_equal "\"\\xFF\" from ASCII-8BIT to UTF-8", assert_raises(Encoding::UndefinedConversionError) { "\xff".encode("UTF-8", "BINARY") }.message
      assert_equal "\"\\xFF\" to UTF-8 in conversion from ASCII-8BIT to UTF-8 to UTF-16LE", assert_raises(Encoding::UndefinedConversionError) { "\xffa".encode("UTF-16LE", "BINARY") }.message
      assert_equal "U+20AC to ISO-8859-1 in conversion from UTF-16BE to UTF-8 to ISO-8859-1", assert_raises(Encoding::UndefinedConversionError) { "€".encode("UTF-16BE").encode("ISO-8859-1", "UTF-16BE") }.message
      assert_equal "U+00E9 to US-ASCII in conversion from ISO-8859-1 to UTF-8 to US-ASCII", assert_raises(Encoding::UndefinedConversionError) { "\xe9".encode("US-ASCII", "ISO-8859-1") }.message
    end

    def test_invalid_byte_sequence
      e = assert_raises(Encoding::InvalidByteSequenceError) { "a\xffb".encode("UTF-16LE") }
      assert_equal "\"\\xFF\" on UTF-8", e.message
      assert_equal ["UTF-8", "UTF-16LE", [255], nil, false], [e.source_encoding_name, e.destination_encoding_name, e.error_bytes&.bytes, e.readagain_bytes, e.incomplete_input?]
      e2 = assert_raises(Encoding::InvalidByteSequenceError) { "a\xe3\x81".encode("UTF-16LE") }
      assert_equal ["incomplete \"\\xE3\\x81\" on UTF-8", [227, 129], true], [e2.message, e2.error_bytes&.bytes, e2.incomplete_input?]
      e3 = assert_raises(Encoding::InvalidByteSequenceError) { "a\xe3\x81b".encode("UTF-16LE") }
      assert_equal ["\"\\xE3\\x81\" followed by \"b\" on UTF-8", "b"], [e3.message, e3.readagain_bytes]
      assert_equal "\"\\x00h\" on UTF-16", assert_raises(Encoding::InvalidByteSequenceError) { "\x00h".encode("UTF-8", "UTF-16") }.message
      assert_equal "incomplete \"i\" on UTF-16LE", assert_raises(Encoding::InvalidByteSequenceError) { "h\x00i".encode("UTF-8", "UTF-16LE") }.message
      assert_equal "\"\\x00\\xD8\" followed by \"h\\x00\" on UTF-16LE", assert_raises(Encoding::InvalidByteSequenceError) { "\x00\xd8h\x00".encode("UTF-8", "UTF-16LE") }.message
      assert_equal "incomplete \"\\x00\\xD8\" on UTF-16LE", assert_raises(Encoding::InvalidByteSequenceError) { "\x00\xd8".encode("ISO-8859-1", "UTF-16LE") }.message
      assert_equal "\"\\x00\\x00\\x11\\x00\" on UTF-32LE", assert_raises(Encoding::InvalidByteSequenceError) { "\x00\x00\x11\x00".encode("UTF-8", "UTF-32LE") }.message
      assert_equal "incomplete \"a\\x00\" on UTF-32LE", assert_raises(Encoding::InvalidByteSequenceError) { "a\x00".encode("UTF-8", "UTF-32LE") }.message
      assert_equal "\"\\xFF\" on US-ASCII", assert_raises(Encoding::InvalidByteSequenceError) { "a\xffb".encode("UTF-8", "US-ASCII") }.message
    end

    # MRI decodes UTF-16/32 byte by byte: the first byte that cannot continue a character ends it, but a fault inside the first unit still takes the whole unit.
    #: (String, String) -> Encoding::InvalidByteSequenceError
    def bad(s, from) = assert_raises(Encoding::InvalidByteSequenceError) { s.encode("UTF-8", from) }

    def test_invalid_units
      assert_equal "\"\\xDC\" on UTF-16BE", bad("\xDC", "UTF-16BE").message
      assert_equal "\"\\xD8\\x00\" followed by \"\\x00\" on UTF-16BE", bad("\xD8\x00\x00h", "UTF-16BE").message
      assert_equal "incomplete \"\\xD8\\x00\\xDC\" on UTF-16BE", bad("\xD8\x00\xDC", "UTF-16BE").message
      assert_equal "\"\\x00\\xD8\" followed by \"\\x00\\x00\" on UTF-16LE", bad("\x00\xD8\x00\x00", "UTF-16LE").message
      assert_equal "\"\\x00\\x11\" on UTF-32BE", bad("\x00\x11", "UTF-32BE").message
      assert_equal "\"\\x00\\x00\\xD8\" on UTF-32BE", bad("\x00\x00\xD8", "UTF-32BE").message
      assert_equal "\"\\x00\\xD8\\x00\\x00\" on UTF-32LE", bad("\x00\xD8\x00\x00", "UTF-32LE").message
      assert_equal "incomplete \"\\x00\\x00\\x00\" on UTF-32LE", bad("\x00\x00\x00", "UTF-32LE").message
      assert_equal "incomplete \"A\" on UTF-16", bad("A", "UTF-16").message
      assert_equal "\"\\xFE\\x00\" on UTF-16", bad("\xFE\x00", "UTF-16").message
      assert_equal "\"\\x00\\x01\\xFE\\xFF\" on UTF-32", bad("\x00\x01\xFE\xFF", "UTF-32").message
      assert_equal "\uFFFDh", "\x00h\xFE\xFF\x00h".encode("UTF-8", "UTF-16", invalid: :replace)
      assert_equal "h\uFFFEh", "\xFE\xFF\x00h\xFF\xFE\x00h".encode("UTF-8", "UTF-16")
      assert_equal "h\uFEFF", "h\x00\xFF\xFE".encode("UTF-8", "UTF-16LE")
    end

    def test_encode_bad_arguments
      assert_equal "code converter not found (UTF-8 to bogus)", assert_raises(Encoding::ConverterNotFoundError) { "a".encode("bogus") }.message
      assert_equal "code converter not found (bogus to UTF-8)", assert_raises(Encoding::ConverterNotFoundError) { "a".encode("UTF-8", "bogus") }.message
      assert_equal "unknown value for invalid character option", assert_raises(ArgumentError) { "a".encode("UTF-8", invalid: :foo) }.message
      assert_equal "unknown value for undefined character option", assert_raises(ArgumentError) { "a".encode("UTF-8", undef: :foo) }.message
      assert_equal "unexpected value for xml option: foo", assert_raises(ArgumentError) { "a".encode(xml: :foo) }.message
    end

    def test_integer_chr
      assert_equal "😀", 0x1F600.chr(Encoding::UTF_8)
      assert_equal "é", 233.chr("UTF-8")
      assert_equal [233], 233.chr(Encoding::ISO_8859_1).bytes
      assert_equal [233], 233.chr(Encoding::BINARY).bytes
      assert_equal "A", 65.chr(Encoding::US_ASCII)
      assert_equal "300 out of char range", assert_raises(RangeError) { 300.chr(Encoding::ISO_8859_1) }.message
      assert_equal "invalid codepoint 0x80 in US-ASCII", assert_raises(RangeError) { 128.chr(Encoding::US_ASCII) }.message
      assert_equal "300 out of char range", assert_raises(RangeError) { 300.chr(Encoding::US_ASCII) }.message
      assert_equal "-1 out of char range", assert_raises(RangeError) { -1.chr(Encoding::UTF_8) }.message
      assert_equal "invalid codepoint 0xD800 in UTF-16LE", assert_raises(RangeError) { 0xD800.chr("UTF-16LE") }.message
      assert_equal [65, 0], 0x41.chr("UTF-16LE").bytes
      assert_equal "invalid codepoint 0xD800 in UTF-8", assert_raises(RangeError) { 0xD800.chr(Encoding::UTF_8) }.message
      assert_equal "1114112 out of char range", assert_raises(RangeError) { 0x110000.chr(Encoding::UTF_8) }.message
      assert_equal "invalid codepoint 0x110000 in UTF-32LE", assert_raises(RangeError) { 0x110000.chr("UTF-32LE") }.message
    end

    def test_pack_u
      assert_equal [237, 160, 128], [0xD800].pack("U").bytes
      assert_equal [244, 144, 128, 128], [0x110000].pack("U").bytes
      assert_equal [253, 191, 191, 191, 191, 191], [0x7FFFFFFF].pack("U").bytes
      assert_equal "pack(U): value out of range", assert_raises(RangeError) { [0x80000000].pack("U") }.message
    end

    def test_pack_nul_directive
      assert_equal "unknown pack directive '\\x00' in 'C\\0C'", assert_raises(ArgumentError) { [1, 2].pack("C\0C") }.message
      assert_equal "unknown unpack directive '\\x00' in 'C\\0C'", assert_raises(ArgumentError) { "ab".unpack("C\0C") }.message
      assert_equal "unknown pack directive '\\x01' in 'C\\u0001Z\\0\\u00E9'", assert_raises(ArgumentError) { [1].pack("C\x01Z\0é") }.message
      assert_equal [97, 98], "ab".unpack("C C")
    end

    def test_unicode_normalize
      assert_equal 6, "café".unicode_normalize(:nfd).bytesize
      assert_equal [65, 778], "Å".unicode_normalize(:nfd).codepoints
      assert_equal [197], "\u212b".unicode_normalize.codepoints
      assert_equal [7835, 803], "\u1e9b\u0323".unicode_normalize(:nfc).codepoints
      assert_equal [383, 803, 775], "\u1e9b\u0323".unicode_normalize(:nfd).codepoints
      assert_equal [7785], "\u1e9b\u0323".unicode_normalize(:nfkc).codepoints
      assert_equal [115, 803, 775], "\u1e9b\u0323".unicode_normalize(:nfkd).codepoints
      assert_equal [44033], "\u1100\u1161\u11a8".unicode_normalize.codepoints
      assert_equal [4352, 4449, 4520], "각".unicode_normalize(:nfd).codepoints
      assert_equal "1", "①".unicode_normalize(:nfkc)
      assert_equal "fi", "ﬁ".unicode_normalize(:nfkd)
      assert_equal [97, 808, 769], "a\u0328\u0301".unicode_normalize(:nfd).codepoints
      assert_equal [261, 769], "a\u0301\u0328".unicode_normalize(:nfc).codepoints
      assert_equal [776, 97], "\u0308a".unicode_normalize.codepoints
      assert_equal "plain", "plain".unicode_normalize(:nfkc)
      assert_equal [false, true, false, true, false], ["e\u0301".unicode_normalized?, "é".unicode_normalized?, "é".unicode_normalized?(:nfd), "ﬁ".unicode_normalized?, "ﬁ".unicode_normalized?(:nfkc)]
      assert_equal "Invalid normalization form x.", assert_raises(ArgumentError) { "a".unicode_normalize(:x) }.message
    end
  end

  class IOEncodingTest < Minitest::Test
    def test_file_modes
      Dir.mktmpdir do |dir|
        path = File.join(dir, "enc.txt")
        File.open(path, "w:ISO-8859-1") do |f|
          f.write("café\n")
          f.puts "ñ"
          assert_equal "ISO-8859-1", f.external_encoding&.name
          assert_nil f.internal_encoding
          assert_equal false, f.binmode?
        end
        assert_equal [99, 97, 102, 233, 10, 241, 10], File.binread(path).bytes
        File.open(path, "r:ISO-8859-1:UTF-8") do |f|
          assert_equal "café\n", f.gets
          assert_equal "ISO-8859-1", f.external_encoding&.name
          assert_same Encoding::UTF_8, f.internal_encoding
          assert_equal "ñ\n", f.read
        end
        File.open(path, "r:ISO-8859-1") { |f| assert_equal [99, 97, 102, 233, 10], f.gets&.bytes }
        mode = ["r", "ISO-8859-1", "UTF-8"].join(":")
        f2 = File.new(path, mode)
        assert_equal "café\n", f2.gets
        f2.close
        File.open(path, "rb") do |f|
          assert_same Encoding::BINARY, f.external_encoding
          assert_equal true, f.binmode?
          assert_equal [99, 97, 102, 233, 10], f.gets&.bytes
        end
        File.open(path, "r") do |f|
          assert_same Encoding::UTF_8, f.external_encoding
          assert_equal false, f.binmode?
        end
        File.open(path, "r:UTF-8") { |f| assert_same Encoding::UTF_8, f.external_encoding }
        File.open(path, "w") { |f| assert_nil f.external_encoding }
        File.open(path, "wb") { |f| assert_same Encoding::BINARY, f.external_encoding }
        File.open(path, "r+b") { |f| assert_same Encoding::BINARY, f.external_encoding }
      end
    end

    def test_utf16_files
      Dir.mktmpdir do |dir|
        path = File.join(dir, "u16.txt")
        File.open(path, "w:UTF-16LE") { |f| f.write("hé\n") }
        assert_equal [104, 0, 233, 0, 10, 0], File.binread(path).bytes
        File.open(path, "r:UTF-16LE:UTF-8") { |f| assert_equal ["hé\n", nil], [f.gets, f.gets] }
        File.open(path, "w:UTF-16") { |f| f.write("é"); f.write("a") }
        assert_equal [254, 255, 0, 233, 0, 97], File.binread(path).bytes
        e = assert_raises(ArgumentError) { File.open(path, "r:UTF-16LE") { |f| f.gets } }
        assert_equal "ASCII incompatible encoding needs binmode", e.message
        File.binwrite(path, "\xEF\xBB\xBFhi\n")
        File.open(path, "r:BOM|UTF-8") { |f| assert_equal ["hi\n", "UTF-8"], [f.gets, f.external_encoding&.name] }
        File.binwrite(path, "\xFF\xFEh\x00\n\x00")
        File.open(path, "r:BOM|UTF-16LE:UTF-8") { |f| assert_equal ["h\n", "UTF-16LE"], [f.gets, f.external_encoding&.name] }
      end
    end

    def test_write_errors_and_binmode
      Dir.mktmpdir do |dir|
        path = File.join(dir, "w.txt")
        e = assert_raises(Encoding::UndefinedConversionError) { File.open(path, "w:ISO-8859-1") { |f| f.write("€") } }
        assert_equal "U+20AC from UTF-8 to ISO-8859-1", e.message
        File.open(path, "w:ISO-8859-1") do |f|
          assert_equal true, f.binmode.equal?(f)
          assert_equal true, f.binmode?
          assert_same Encoding::BINARY, f.external_encoding
          f.write("é")
        end
        assert_equal [195, 169], File.binread(path).bytes
        File.open(path, "wb:ISO-8859-1") { |f| f.write("é") }
        assert_equal [233], File.binread(path).bytes
      end
    end

    def test_set_encoding
      Dir.mktmpdir do |dir|
        path = File.join(dir, "s.txt")
        File.binwrite(path, "caf\xe9\nna\xefve\n")
        File.open(path) do |f|
          assert_equal true, f.set_encoding("ISO-8859-1:UTF-8").equal?(f)
          assert_equal ["ISO-8859-1", "UTF-8"], [f.external_encoding&.name, f.internal_encoding&.name]
          assert_equal "café\n", f.gets
        end
        File.open(path) do |f|
          f.set_encoding(Encoding::ISO_8859_1, Encoding::UTF_8)
          assert_equal "café\n", f.gets
          f.set_encoding(Encoding::ISO_8859_1, Encoding::UTF_8)
          assert_equal "naïve\n", f.gets
        end
        File.open(path, "r:ISO-8859-1:UTF-8") do |f|
          assert_equal "café\n", f.gets
          f.set_encoding("ISO-8859-1")
          assert_equal "na\xefve\n".b, f.gets.b
          f.set_encoding("ISO-8859-1", "UTF-8")
          f.rewind
          assert_equal "café\n", f.gets
        end
        File.open(path) do |f|
          f.set_encoding(nil)
          assert_equal ["UTF-8", nil], [f.external_encoding&.name, f.internal_encoding]
        end
        File.open(path) do |f|
          f.set_encoding("BINARY")
          assert_equal [Encoding::BINARY, false], [f.external_encoding, f.binmode?]
        end
      end
    end

    def test_std_streams
      assert_nil $stdout.external_encoding
      assert_same Encoding::UTF_8, $stdin.external_encoding
      assert_nil $stderr.internal_encoding
      assert_equal false, $stdout.binmode?
    end
  end

  class EncodingDummyTest < Minitest::Test
    def test_iso_2022_jp_is_dummy
      e = Encoding::ISO_2022_JP
      assert_equal ["ISO-2022-JP", true, false, true], [e.name, e.dummy?, e.ascii_compatible?, Encoding::ISO2022_JP == e]
    end
  end

  # The single-byte encodings x/text's tables carry, with MRI's corrections (decision 380).
  class SingleByteEncodingTest < Minitest::Test
    def test_round_trips
      s = "Grüße, €5 - ok"
      w = s.encode("Windows-1252")
      assert_equal [71, 114, 252, 223, 101, 44, 32, 128, 53, 32, 45, 32, 111, 107], w.bytes
      assert_equal s, w.encode("UTF-8", "Windows-1252")
      assert_equal s, s.encode("ISO-8859-15").encode("UTF-8", "ISO-8859-15")
      assert_equal [240, 210, 201, 215, 197, 212], "Привет".encode("KOI8-R").bytes
      assert_equal [143, 224, 168, 162, 165, 226], "Привет".encode("IBM866").bytes
      assert_equal "Привет", "Привет".encode("Windows-1251").encode("UTF-8", "CP1251")
    end

    def test_mri_tables_where_x_text_differs
      assert_equal [0x80], "\u0080".encode("ISO-8859-2").bytes # C1 controls map to themselves
      assert_equal "\u2126", "\xBD".b.encode("UTF-8", "macRoman") # Ohm sign, not omega
      assert_equal "\u255D", "\xAE".b.encode("UTF-8", "KOI8-U")
      assert_equal [0xE9, 0xC4], "\uFB1D".encode("Windows-1255").bytes # a presentation form as letter and point
    end

    def test_errors_as_mri
      e = assert_raises(Encoding::UndefinedConversionError) { "\x81".b.encode("UTF-8", "Windows-1252") }
      assert_equal "\"\\x81\" from WINDOWS-1252 to UTF-8", e.message
      assert_equal "WINDOWS-1252", e.source_encoding_name
      e = assert_raises(Encoding::UndefinedConversionError) { "\u0100".encode("Windows-1252") }
      assert_equal "U+0100 from UTF-8 to WINDOWS-1252", e.message
      e = assert_raises(Encoding::UndefinedConversionError) { "€".encode("Windows-1252").encode("ISO-8859-2", "Windows-1252") }
      assert_equal "U+20AC to ISO-8859-2 in conversion from WINDOWS-1252 to UTF-8 to ISO-8859-2", e.message
      assert_equal [42, 42, 120], "Ā€x".encode("KOI8-R", undef: :replace, replace: "*").bytes
    end

    def test_encoding_objects
      assert_equal Encoding::Windows_1252, Encoding.find("cp1252")
      assert_equal ["ISO-8859-15", "ISO8859-15"], Encoding.find("ISO8859-15").names
      assert Encoding::MacRoman.ascii_compatible?
      assert Encoding.list.include?(Encoding::KOI8_U)
      assert Encoding.name_list.include?("CP1252")
      assert_equal "Windows-1252", Encoding.aliases.fetch("CP1252")
      assert_equal "\xE9".b.bytes, 0xE9.chr("Windows-1252").bytes
    end

    def test_file_read_transcodes
      path = "encoding_test_sb.txt"
      File.write(path, "Grüße\n".encode("Windows-1252"))
      read = File.read(path, encoding: "Windows-1252:UTF-8")
      line = File.open(path, "r:CP1252:UTF-8") { |f| f.gets }
      File.delete(path)
      assert_equal "Grüße\n", read
      assert_equal "Grüße\n", line
    end
  end

  # Normalization and case maps through x/text, with MRI's choices where x/text's differ (decision 380).
  class XTextUnicodeTest < Minitest::Test
    def test_normalize
      assert_equal [65, 778], "Å".unicode_normalize(:nfd).codepoints
      assert_equal "fi", "ﬁ".unicode_normalize(:nfkc)
      assert_equal "\u1e0b\u0323".unicode_normalize(:nfc), "\u1e0d\u0307"
      assert "café".unicode_normalized?
      refute "cafe\u0301".unicode_normalized?
    end

    def test_case_maps
      assert_equal "σασ οδοσ", "ΣΑΣ ΟΔΟΣ".downcase # MRI applies no final-sigma rule
      assert_equal "ı̇", "I\u0307".downcase(:turkic)
      assert_equal "İSTANBUL", "istanbul".upcase(:turkic)
      assert_equal "ss", "ß".downcase(:fold)
      assert_equal "Ꭰ", "ꭰ".downcase(:fold) # Cherokee folds to its capitals
      assert_equal "აბ", "ᲐᲑ".capitalize # a Georgian Mtavruli first letter is lowered
      assert "ß".casecmp?("SS")
      assert "Straße".casecmp?("STRASSE")
      assert "ꭰ".casecmp?("Ꭰ")
      refute "É".capitalize.equal?("É")
    end
  end

  # The multibyte encodings x/text's tables carry, with MRI's byte structure and corrections (decision 380).
  class MultiByteEncodingTest < Minitest::Test
    def test_japanese
      s = "日本語のテキスト、ｶﾅ"
      assert_equal s, s.encode("Shift_JIS").encode("UTF-8", "Shift_JIS")
      assert_equal s, s.encode("EUC-JP").encode("UTF-8", "EUC-JP")
      assert_equal [0x93, 0xFA, 0x96, 0x7B], "日本".encode("Windows-31J").bytes
      assert_equal "\u301C", "\x81\x60".b.encode("UTF-8", "Shift_JIS") # wave dash, as MRI's JIS table has it
      assert_equal "\uE000", "\xF0\x40".b.encode("UTF-8", "Windows-31J") # the user-defined area
      assert_equal "\u4E02", "\x8F\xB0\xA1".b.encode("UTF-8", "EUC-JP") # JIS X 0212's three bytes
    end

    def test_chinese_and_korean
      assert_equal "中文字符", "中文字符".encode("GBK").encode("UTF-8", "GBK")
      assert_equal "\u{1F600}", "\u{1F600}".encode("GB18030").encode("UTF-8", "GB18030")
      assert_equal [0x90, 0x30, 0x81, 0x30], "\u{10000}".encode("GB18030").bytes
      assert_equal "繁體", "繁體".encode("Big5").encode("UTF-8", "CP950")
      assert_equal "한국어", "한국어".encode("EUC-KR").encode("UTF-8", "CP949")
    end

    def test_errors_as_mri
      e = assert_raises(Encoding::InvalidByteSequenceError) { "\x81 ".b.encode("UTF-8", "Shift_JIS") }
      assert_equal "\"\\x81\" followed by \" \" on Shift_JIS", e.message
      e = assert_raises(Encoding::InvalidByteSequenceError) { "\x81".b.encode("UTF-8", "Shift_JIS") }
      assert_equal "incomplete \"\\x81\" on Shift_JIS", e.message
      e = assert_raises(Encoding::InvalidByteSequenceError) { "\x81\x30\x81\x2f".b.encode("UTF-8", "GB18030") }
      assert_equal "\"\\x810\\x81\" followed by \"/\" on GB18030", e.message
      e = assert_raises(Encoding::UndefinedConversionError) { "\u0100".encode("EUC-KR") }
      assert_equal "U+0100 from UTF-8 to EUC-KR", e.message
      assert_equal "a?b", "a\u0100b".encode("Shift_JIS", undef: :replace).encode("UTF-8", "Shift_JIS")
    end

    def test_names_constants_and_files
      assert_equal Encoding::Shift_JIS, Encoding.find("shift_jis")
      assert_equal Encoding::Windows_31J, Encoding.find("SJIS")
      assert_equal [0x93, 0xFA], "日".encode(Encoding::Windows_31J).bytes
      name = ["EUC", "KR"].join("-")
      assert_equal [0xC7, 0xD1], "한".encode(name).bytes
      path = "encoding_test_mb.txt"
      File.write(path, "日本\n".encode("Shift_JIS"))
      read = File.read(path, encoding: "Shift_JIS:UTF-8")
      line = File.open(path, "r:Shift_JIS:UTF-8") { |f| f.gets }
      File.delete(path)
      assert_equal "日本\n", read
      assert_equal "日本\n", line
    end
  end

  # Encoding::Converter on encode's transcoder (decision 452)
  class ConverterTest < Minitest::Test
    def test_convert_and_inspect
      ec = Encoding::Converter.new("UTF-8", "ISO-8859-1")
      assert_equal "caf\xE9".b, ec.convert("café").b
      assert_equal "#<Encoding::Converter: UTF-8 to ISO-8859-1>", ec.inspect
      assert_equal Encoding::UTF_8, ec.source_encoding
      assert_equal Encoding::ISO_8859_1, ec.destination_encoding
      assert_equal "", ec.finish
    end

    def test_incomplete_tail_waits
      ec = Encoding::Converter.new("UTF-8", "UTF-16LE")
      assert_equal [97, 0], ec.convert("a\xE3".b).bytes
      assert_equal [66, 48], ec.convert("\x81\x82".b).bytes
      assert_equal "", ec.finish
    end

    def test_errors_and_last_error
      ec = Encoding::Converter.new("UTF-8", "US-ASCII")
      begin
        ec.convert("aé")
        flunk "no error"
      rescue Encoding::UndefinedConversionError => err
        assert_equal "é", err.error_char
        assert_equal "UTF-8", err.source_encoding_name
      end
      assert_equal Encoding::UndefinedConversionError, ec.last_error.class
      assert_equal :undefined_conversion, ec.primitive_errinfo[0]
      assert_equal "?", ec.replacement
      assert_equal "\uFFFD", Encoding::Converter.new("ISO-8859-1", "UTF-8").replacement
      r = Encoding::Converter.new("UTF-8", "US-ASCII", undef: :replace, replace: "*")
      assert_equal "a*", r.convert("aé")
    end

    def test_convpath
      assert_equal [[Encoding::UTF_8, Encoding::EUC_JP]], Encoding::Converter.new("UTF-8", "EUC-JP").convpath
      assert_equal [[Encoding::ISO_8859_1, Encoding::UTF_8], [Encoding::UTF_8, Encoding::EUC_JP], "universal_newline"],
                   Encoding::Converter.search_convpath("ISO-8859-1", "EUC-JP", newline: :universal)
      assert_equal Encoding::UTF_8, Encoding::Converter.asciicompat_encoding("UTF-16BE")
      assert_nil Encoding::Converter.asciicompat_encoding("UTF-8")
      begin
        Encoding::Converter.new("UTF-8", "nope")
        flunk "no error"
      rescue Encoding::ConverterNotFoundError => nf
        assert_equal "code converter not found (UTF-8 to nope)", nf.message
      end
    end
  end
end
