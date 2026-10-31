# frozen_string_literal: true

require "strscan"

# rubocop-ast's rex-generated NodePattern lexer: `action { emit :tSYMBOL, &:to_sym }` (emit lives in the subclass, so the
# base's self-call is dynamic and the symbol is its block), and `emit ss.matched, &:to_sym` passing a String where the
# other calls pass Symbols (the dynamic block value is not a parameter's use, so `type` widens).
class RexishBase
  attr_accessor :ss, :state

  def action
    yield
  end

  def scanner_class
    StringScanner
  end unless instance_methods(false).map(&:to_s).include?("scanner_class")

  def parse(str)
    self.ss = scanner_class.new(str)
    self.state = nil
    do_parse
  end

  def do_parse
    while (token = next_token)
      process(token)
    end
  end

  def next_token
    token = nil
    until ss.eos? || token
      token =
        case state
        when nil
          case
          when ss.skip(/\s+/) then
            # nothing
          when ss.skip(/:(\w+)/) then
            action { emit :tSYMBOL, &:to_sym }
          when ss.skip(/\d+/) then
            action { emit :tNUMBER, &:to_i }
          when ss.skip(/[(){}]/) then
            action { emit ss.matched, &:to_sym }
          when ss.skip(/\w+/) then
            action { emit :tWORD }
          else
            raise "bad char"
          end
        else
          raise "bad state"
        end
    end
    token
  end
end

class Rexish < RexishBase
  attr_reader :tokens

  def initialize(source)
    @tokens = []
    super()
    parse(source)
  end

  def emit(type)
    value = ss[1] || ss.matched
    value = yield value if block_given?
    token = token(type, value)
    @tokens << token
    token
  end

  def token(type, value)
    [type, value]
  end

  def process(token)
    token
  end
end
