module Sigdemo
  # Unannotated: the gem's own sig/sigdemo.rbs types it; sig/broken.rbs only warns (decision 306).
  class Meter
    def initialize
      @readings = []
    end

    def add(value)
      @readings << value
      self
    end

    def push(value) = add(value)

    def total
      @readings.sum
    end

    def absorb(other)
      add(other.total)
    end

    def label(src)
      "#{src["name"]}:#{src["kind"]}"
    end

    # sig/sigdemo.rbs is stale for these two (prism 1.9.0's Source.for and lex_compat): the gem's def wins (decision 427)
    def scaled(factor, offset = 0) = (total * factor) + offset

    def summary(**opts) = "#{opts.size} #{total}"

    def each_reading(sink)
      @readings.each { |r| sink.call(r) }
    end
  end
end
