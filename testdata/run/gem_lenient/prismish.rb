# frozen_string_literal: true

# Prism's translation to the parser gem's AST, in miniature (decision 496): a byteslice by a range whose end may be
# nil, a blockless filter_map's with_index, start_with? of four prefixes, and an Integer compared with an untyped value
# that is a Float when it runs.
class Prismish
  def initialize(source, flags)
    @source = source
    @flags = flags
  end

  def source = [@source, 1].first

  # compiler.rb's srange_semicolon: `end_offset` may be nil, searching to the end
  def semicolon(start_offset, end_offset)
    if (match = source.byteslice(start_offset...end_offset)[/\A\s*;/])
      start_offset + match.bytesize
    end
  end

  def semicolon_typed(start_offset, end_offset)
    if (match = @source.byteslice(start_offset...end_offset)[/\A\s*;/])
      start_offset + match.bytesize
    end
  end

  # string_nodes_from_line_continuations: the lines whose flag is set
  def kept_lines
    lines = [@source.lines, 1].first
    lines.filter_map.with_index do |line, index|
      @flags[index] ? line.chomp : nil
    end
  end

  def percent?(opening) = opening&.start_with?("%w", "%W", "%i", "%I")

  # lexer.rb's calculate_heredoc_whitespace: Float::MAX until a line is shorter
  def least_indent(lines)
    result = Float::MAX
    lines.each do |line|
      n = line[/\A */].size
      result = n if n < result
    end
    result
  end
end
