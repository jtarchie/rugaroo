# frozen_string_literal: true

# A gem's private instance method appending to a String parameter (prism's Lexer#escape_read(result, ...)), with `<<` and
# Ruby 3.4's append_as_bytes (a String's bytes, an Integer's low byte).
class Outish
  def build(parts)
    result = "".b
    parts.each { |part| push_part(result, part) }
    result << "!"
    result.append_as_bytes(0x21, "?")
    result
  end

  private

  def push_part(result, part)
    result << part.upcase
    result.append_as_bytes("-", 0x2E)
    nest(result, part) if part.size > 1
  end

  def nest(result, part)
    result.append_as_bytes(part.size + 0x30)
  end
end
