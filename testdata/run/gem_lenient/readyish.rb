module Readyish
  class Base
    attr_reader :config, :options

    def initialize(config = nil, options = nil)
      @config = config
      @options = options
      @used = false
    end

    def use! = @used = true

    def ready
      return self unless @used

      self.class.new(@config, @options)
    end

    def name = self.class.name
  end

  class Plain < Base
  end

  class Extra < Base
    attr_reader :offenses

    def initialize(config = nil, options = nil, offenses = nil)
      @offenses = offenses
      super(config, options)
    end
  end

  class Match
    def each_match(text, regex)
      out = []
      text.scan(regex) { out << Regexp.last_match.begin(0) }
      out
    end
  end

  def self.demo
    cops = [Plain.new(1, 2), Extra.new(3, 4, 5)] #: Array[untyped]
    out = []
    cops.each do |c|
      c.use!
      n = c.ready
      out << [n.name, n.config, n.options, n.equal?(c)]
    end
    h = { t: "a bb ccc" } #: Hash[Symbol, untyped]
    out << Match.new.each_match(h[:t], /\w+/)
    out
  end
end
