# Stands in for prism's C extension and prism/ffi.rb (decision 427): ffi.rb's API over prism.wasm; the gem's Serialize builds the tree.
require_relative "stringio" # serialize.rb's: it loads through an autoload boot never resolved, which the require scan does not walk

# ponytail: Kernel#gem lives here, not in core, since a core Kernel name adds a dynamic wrapper per class to programs that
# dispatch dynamically (+37 funcs on example 64); move it to core if another gem needs it.
module Kernel
  # RubyGems' activation: the closed world fixed each gem's version when it compiled (prism's translation/parser.rb)
  #: (String, *String) -> bool
  private def gem(name, *requirements) = true
end

module Prism
  VERSION = "1.9.0"

  class << self
    #: (String, **untyped) -> String
    def dump(source, **options)
      dumped = __prism_serialize(0, source, dump_options(options))
      dumped.freeze if options.fetch(:freeze, false)
      dumped
    end

    #: (String, **untyped) -> String
    def dump_file(filepath, **options)
      options[:filepath] = filepath
      dump(File.read(filepath), **options)
    end

    #: (String, **untyped) -> LexResult
    def lex(code, **options)
      Serialize.load_lex(code, __prism_serialize(1, code, dump_options(options)), options.fetch(:freeze, false))
    end

    #: (String, **untyped) -> LexResult
    def lex_file(filepath, **options)
      options[:filepath] = filepath
      lex(File.read(filepath), **options)
    end

    #: (String, **untyped) -> ParseResult
    def parse(code, **options)
      Serialize.load_parse(code, __prism_serialize(0, code, dump_options(options)), options.fetch(:freeze, false))
    end

    #: (String, **untyped) -> ParseResult
    def parse_file(filepath, **options)
      options[:filepath] = filepath
      parse(File.read(filepath), **options)
    end

    # ponytail: reads the whole stream, where MRI stops at __END__; pm_serialize_parse_stream needs a host fgets callback.
    #: (IO, **untyped) -> ParseResult
    def parse_stream(stream, **options)
      parse(stream.read, **options)
    end

    #: (String, **untyped) -> Array[Comment]
    def parse_comments(code, **options)
      Serialize.load_parse_comments(code, __prism_serialize(2, code, dump_options(options)), options.fetch(:freeze, false))
    end

    #: (String, **untyped) -> Array[Comment]
    def parse_file_comments(filepath, **options)
      options[:filepath] = filepath
      parse_comments(File.read(filepath), **options)
    end

    #: (String, **untyped) -> ParseLexResult
    def parse_lex(code, **options)
      Serialize.load_parse_lex(code, __prism_serialize(3, code, dump_options(options)), options.fetch(:freeze, false))
    end

    #: (String, **untyped) -> ParseLexResult
    def parse_lex_file(filepath, **options)
      options[:filepath] = filepath
      parse_lex(File.read(filepath), **options)
    end

    #: (String, **untyped) -> bool
    def parse_success?(code, **options)
      __prism_call(4, code, dump_options(options)) != 0
    end

    #: (String, **untyped) -> bool
    def parse_failure?(code, **options)
      !parse_success?(code, **options)
    end

    #: (String, **untyped) -> bool
    def parse_file_success?(filepath, **options)
      options[:filepath] = filepath
      parse_success?(File.read(filepath), **options)
    end

    #: (String, **untyped) -> bool
    def parse_file_failure?(filepath, **options)
      !parse_file_success?(filepath, **options)
    end

    #: (String, **untyped) -> nil
    def profile(source, **options)
      __prism_serialize(0, source, dump_options(options))
      nil
    end

    #: (String, **untyped) -> nil
    def profile_file(filepath, **options)
      options[:filepath] = filepath
      profile(File.read(filepath), **options)
    end

    #: (Integer, String, String) -> String
    def __prism_serialize(fn, source, options) = __prism_bytes(fn, source, options).b

    #: (Integer, String, String) -> String
    def __prism_bytes(fn, source, options) = %x{ return rbPrismSerialize(int(fn), string(source), string(options)) }

    #: (Integer, String, String) -> Integer
    def __prism_call(fn, source, arg) = %x{ return rbPrismCall(int(fn), string(source), string(arg)) }

    private

    #: (Hash[Symbol, untyped]) -> Integer
    def dump_options_command_line(options)
      command_line = options.fetch(:command_line, "")
      raise ArgumentError, "command_line must be a string" unless command_line.is_a?(String)

      value = 0
      command_line.each_char do |char|
        case char
        when "a" then value |= 0b000001
        when "e" then value |= 0b000010
        when "l" then value |= 0b000100
        when "n" then value |= 0b001000
        when "p" then value |= 0b010000
        when "x" then value |= 0b100000
        else raise ArgumentError, "invalid command_line option: #{char}"
        end
      end
      value
    end

    #: (untyped) -> Integer
    def dump_options_version(version)
      current = version == "current"
      v = current ? RUBY_VERSION : version
      case v
      when nil, "latest"
        0
      when /\A3\.3(\.\d+)?\z/
        1
      when /\A3\.4(\.\d+)?\z/
        2
      when /\A3\.5(\.\d+)?\z/, /\A4\.0(\.\d+)?\z/
        3
      when /\A4\.1(\.\d+)?\z/
        4
      else
        raise CurrentVersionError, RUBY_VERSION if current

        raise ArgumentError, "invalid version: #{version}"
      end
    end

    #: (Hash[Symbol, untyped]) -> String
    def dump_options(options)
      out = ""
      if options[:filepath]
        filepath = String(options[:filepath])
        out += [filepath.bytesize].pack("L") + filepath.b
      else
        out += [0].pack("L")
      end
      out += [options.fetch(:line, 1)].pack("l")
      encoding = options[:encoding]
      if encoding
        name = encoding.is_a?(Encoding) ? encoding.name : String(encoding)
        out += [name.bytesize].pack("L") + name.b
      else
        out += [0].pack("L")
      end
      out += [
        options.fetch(:frozen_string_literal, false) ? 1 : 0,
        dump_options_command_line(options),
        dump_options_version(options[:version]),
        options[:encoding] == false ? 1 : 0,
        options.fetch(:main_script, false) ? 1 : 0,
        options.fetch(:partial_script, false) ? 1 : 0,
        options.fetch(:freeze, false) ? 1 : 0
      ].pack("C*")
      scopes = options[:scopes] #: Array[untyped]?
      if scopes
        out += [scopes.length].pack("L")
        scopes.each do |scope|
          forwarding = 0
          case scope
          when Array
            locals = scope #: Array[untyped]
          when Scope
            locals = scope.locals
            scope.forwarding.each do |forward|
              case forward
              when :* then forwarding |= 0x1
              when :** then forwarding |= 0x2
              when :& then forwarding |= 0x4
              when :"..." then forwarding |= 0x8
              else raise ArgumentError, "invalid forwarding value: #{forward}"
              end
            end
          else
            raise TypeError, "wrong argument type #{scope.class.inspect} (expected Array or Prism::Scope)"
          end
          out += [locals.length, forwarding].pack("LC")
          locals.each do |local|
            name = String(local)
            out += [name.bytesize].pack("L") + name.b
          end
        end
      else
        out += [0].pack("L")
      end
      out
    end
  end

  module Serialize
    class Loader
      # serialize.rb's, less `raise unless serialized.encoding == Encoding::BINARY`: a String here has no encoding flag,
      # so bytes that happen to be valid UTF-8 report UTF_8 (decision 427)
      def initialize(source, serialized)
        @input = source.source.dup
        @io = FastStringIO.new(serialized)
        @source = source
      end
    end
  end

  class StringQuery
    class << self
      #: (String) -> bool
      def local?(string) = __query(Prism.__prism_call(5, string, string.encoding.name))

      #: (String) -> bool
      def constant?(string) = __query(Prism.__prism_call(6, string, string.encoding.name))

      #: (String) -> bool
      def method_name?(string) = __query(Prism.__prism_call(7, string, string.encoding.name))

      private

      #: (Integer) -> bool
      def __query(result)
        raise ArgumentError, "Invalid or non ascii-compatible encoding" if result < 0

        result == 1
      end
    end
  end
end
