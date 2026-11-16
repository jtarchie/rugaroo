# frozen_string_literal: true

class BufGem
  def self.recognize_encoding(string)
    return if string.empty?

    string.start_with?("#") ? Encoding::UTF_8 : nil
  end

  def self.reencode_string(input)
    original_encoding = input.encoding
    detected_encoding = recognize_encoding(input.force_encoding(Encoding::BINARY))

    if detected_encoding.nil?
      input.force_encoding(original_encoding)
    elsif detected_encoding == Encoding::BINARY
      input
    else
      input.force_encoding(detected_encoding).encode(Encoding::UTF_8)
    end
  end

  def initialize(name, first_line = 1, source: nil)
    @name = name.to_s
    @source = nil
    @first_line = first_line
    self.source = source if source
  end

  def source
    raise "uninitialized" if @source.nil?

    @source
  end

  def source=(input)
    input = input.dup if input.frozen?
    input = self.class.reencode_string(input)

    raise EncodingError, "invalid byte sequence in #{input.encoding.name}" unless input.valid_encoding?

    self.raw_source = input
  end

  def raw_source=(input)
    raise ArgumentError, "immutable" if @source

    @source = input.gsub("\r\n", "\n").freeze
  end
end
