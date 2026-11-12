module Bufish
  class Buf
    def initialize(source) = @source = source

    def source_lines
      @lines ||= begin
        lines = @source.lines.to_a
        lines << ''.dup if @source.end_with?("\n".freeze)

        lines.each do |line|
          line.chomp!("\n".freeze)
          line.freeze
        end

        lines.freeze
      end
    end

    def source_line(lineno) = source_lines.fetch(lineno - 1).dup
  end

  def self.demo
    b = Buf.new("class A\n  x\nend\n")
    [b.source_lines, b.source_line(1), b.source_lines.frozen?, b.source_line(1).frozen?]
  end
end
