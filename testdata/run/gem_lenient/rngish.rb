module Rngish
  MIN = -Float::INFINITY

  class DRange < Range
    attr_reader :directive

    def initialize(first_line, last_line, directive)
      super(first_line, last_line)
      @directive = directive
    end
  end

  Analysis = Struct.new(:line_ranges, :start_line_number, :start_directive) do
    def close(line)
      return line_ranges if !start_line_number || line < start_line_number

      line_ranges + [DRange.new(start_line_number, line, start_directive)]
    end
  end

  class Config
    def initialize(starts)
      @ranges = {}
      starts.each do |name, (from, to)|
        a = Analysis.new([], from, "d:#{name}")
        @ranges[name] = a.close(to || Float::INFINITY)
      end
      @ranges["Off"] = Analysis.new([], MIN, "cfg").close(Float::INFINITY)
    end

    def enabled?(cop, first_line, last_line)
      ranges = @ranges[cop]
      return true unless ranges

      ranges.none? do |range|
        range.end >= first_line && range.begin <= last_line
      end
    end

    def describe(cop)
      @ranges[cop].map do |r|
        [r.min == MIN, r.max == Float::INFINITY, r.max - r.min < 5, r.cover?(3), r.directive, r.begin, r.end, r == (r.begin..r.end)]
      end
    end
  end

  def self.demo
    c = Config.new({ "A" => [2, 4], "B" => [3, nil] })
    [c.enabled?("A", 1, 1), c.enabled?("A", 3, 3), c.enabled?("B", 1, 2), c.enabled?("B", 9, 9), c.enabled?("Z", 1, 1), c.enabled?("Off", 1, 1),
     c.describe("A"), c.describe("B"), c.describe("Off")]
  end
end
