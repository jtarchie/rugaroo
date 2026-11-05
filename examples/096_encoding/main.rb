# rbs_inline: enabled

# Encodings (#48): cleaning text that arrives with bad bytes, downgrading to ASCII, UTF-16/32 sizes, and Unicode normalization.

["plain", "Zoë", "naïve café", "😀!"].each do |s|
  puts "#{s.ljust(10)} #{s.encoding}  #{s.length} chars, #{s.bytesize} bytes, ascii_only #{s.ascii_only?}"
end

# Bytes from a Latin-1 file read as if they were UTF-8.
legacy = "caf\xE9 cr\xE8me br\xFBl\xE9e"
puts "valid UTF-8? #{legacy.valid_encoding?}"
puts "scrubbed:    #{legacy.scrub("?")}"
puts "marked:      #{legacy.scrub { |bad| "<#{bad.bytes.map { |b| format("%02X", b) }.join}>" }}"
fixed = legacy.encode("UTF-8", "ISO-8859-1")
puts "transcoded:  #{fixed} (valid now: #{fixed.valid_encoding?})"

# Down to 7-bit ASCII: by default an unmappable character raises; undef: :replace substitutes.
title = "Crème brûlée — €4"
puts title.encode("US-ASCII", undef: :replace)
puts title.encode("US-ASCII", undef: :replace, replace: "_")
puts title.encode("ISO-8859-1", undef: :replace).bytesize
begin
  title.encode("US-ASCII")
rescue Encoding::UndefinedConversionError => e
  puts "#{e.class}: #{e.message}"
  puts "  #{e.error_char.inspect} from #{e.source_encoding_name} to #{e.destination_encoding_name}"
end

# The same text in wider encodings: sizes differ, the round trip is lossless.
text = "héllo 😀"
%w[UTF-8 UTF-16LE UTF-16BE UTF-16 UTF-32LE].each do |name|
  wide = text.encode(name)
  back = wide.encode("UTF-8", name)
  puts "#{name.ljust(8)} #{wide.bytesize.to_s.rjust(2)} bytes, starts #{wide.bytes.first(4).inspect}, round trip #{back == text}"
end
puts "UTF-16LE is ASCII-compatible? #{Encoding::UTF_16LE.ascii_compatible?}; UTF-16 is a dummy? #{Encoding::UTF_16.dummy?}"

# Precomposed é (U+00E9) and e + combining acute (U+0301) look alike but differ until normalized.
composed = "café"
decomposed = "café"
puts "equal as written: #{composed == decomposed} (#{composed.length} vs #{decomposed.length} chars)"
puts "NFC equal:  #{composed.unicode_normalize(:nfc) == decomposed.unicode_normalize(:nfc)}"
puts "NFD codepoints: #{composed.unicode_normalize(:nfd).codepoints.map { |c| format("U+%04X", c) }.join(" ")}"
puts "already NFC? #{composed.unicode_normalized?} / #{decomposed.unicode_normalized?}"
puts "NFKC folds compatibility forms: #{"ﬁle №2 ①".unicode_normalize(:nfkc)}"
