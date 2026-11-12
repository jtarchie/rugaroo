module Scanish
  class Seq
    attr_reader :source, :begin_pos, :flags, :name

    def initialize(match)
      @source = match[0]
      @begin_pos = match.begin(0)
      @flags = match[:flags].to_s + match[:more].to_s
      @name = match[:name]
    end
  end

  class Fmt
    SEQUENCE = /%(?<flags>[-+0]*)(?:<(?<name>\w+)>)?(?<more>[#]*)(?<type>[sd])/

    def initialize(source) = @source = source

    def sequences
      @sequences ||= parse
    end

    private

    def parse
      matches = []
      @source.scan(SEQUENCE) { matches << Seq.new(Regexp.last_match) }
      matches
    end
  end

  class Cop
    def pad(block_args, yield_args)
      yield_args = yield_args.dup.fill(nil, yield_args.length, block_args.length - yield_args.length)
      yield_args.zip(block_args).map { |y, b| [y, b] }
    end

    def files(base, flags)
      patterns = [File.join(base, "*.rb"), File.join(base, "gem_lenient", "foo.*")]
      Dir.glob(patterns, flags).select { |path| FileTest.file?(path) }.sort
    end

    def one(base) = Dir.glob(File.join(base, "**/*.rb")).sort
  end

  def self.demo(dir)
    f = Fmt.new("a %-5s b %<n>d c %+#d")
    h = { b: [1, 2, 3], y: [:a] } #: Hash[Symbol, untyped]
    c = Cop.new
    [f.sequences.map { |s| [s.source, s.begin_pos, s.flags, s.name] }, c.pad(h[:b], h[:y]), c.pad([1], [:q]),
     c.files(dir, 0).map { |p| File.basename(p) }.include?("gem_lenient.rb")]
  end
end
