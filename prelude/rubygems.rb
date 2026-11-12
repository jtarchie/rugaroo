# rbs_inline: enabled

# RubyGems' surface for a program requiring it itself (Homebrew's gem setup, decision 296); no gem loading, as --disable=gems.

module Gem
  VERSION = "4.0.20"

  # @rbs self.@discover_gems_on_require: bool?
  # @rbs self.@paths: Hash[String, String]?

  #: () -> bool
  def self.discover_gems_on_require = @discover_gems_on_require != false

  # A compiled program activates no gems: what it loaded is compiled in (decision 444).
  #: () -> Hash[String, untyped]
  def self.loaded_specs = {}

  #: () -> Array[String]
  def self.path = []

  #: () -> bool
  def self.win_platform? = false

  #: (bool) -> void
  def self.discover_gems_on_require=(on)
    @discover_gems_on_require = on
  end

  #: (Hash[String, String]) -> void
  def self.paths=(env)
    @paths = env
  end

  #: () -> PathSupport
  def self.paths = PathSupport.new(@paths || {})

  # Where gems live, from GEM_HOME and GEM_PATH as `paths=` set them.
  class PathSupport
    #: (Hash[String, String]) -> void
    def initialize(env)
      @env = env
    end

    #: () -> String
    def home = @env.fetch("GEM_HOME", "")

    #: () -> Array[String]
    def path = @env.fetch("GEM_PATH", home).split(File::PATH_SEPARATOR)
  end

  #: () -> String
  def self.dir = paths.home

  #: () -> String
  def self.bindir = File.join(dir, "bin")

  #: () -> String
  def self.ruby_api_version = RbConfig::CONFIG.fetch("ruby_version")

  # A gem version: numeric segments compare as numbers, a letter segment marks a prerelease, which sorts first.
  class Version
    include Comparable

    #: (String) -> void
    def initialize(version)
      @version = version.strip #: String
    end

    #: () -> String
    def to_s = @version

    #: () -> String
    def version = @version

    #: () -> Array[Integer | String]
    def segments = @version.scan(/[0-9]+|[a-z]+/i).map { |s| s.match?(/\A\d/) ? s.to_i : s } #: Array[Integer | String]

    #: () -> bool
    def prerelease? = @version.match?(/[a-zA-Z]/)

    #: () -> Array[Integer | String]
    def canonical_segments
      segs = segments
      release = segs.take_while { |s| s.is_a?(Integer) }
      pre = segs.drop(release.size)
      release.pop while release.size > 1 && release.last == 0 # 1.2.0 is 1.2
      release + pre
    end

    #: (Version) -> Integer
    def <=>(other)
      a = canonical_segments
      b = other.canonical_segments
      (a.size > b.size ? a.size : b.size).times do |i|
        c = Version.__cmp(a[i] || 0, b[i] || 0)
        return c unless c.zero?
      end
      0
    end

    #: (Integer | String, Integer | String) -> Integer
    def self.__cmp(x, y)
      if x.is_a?(Integer) && y.is_a?(Integer)
        x <=> y
      elsif x.is_a?(String) && y.is_a?(String)
        x <=> y
      elsif x.is_a?(String)
        -1
      else
        1
      end
    end
  end
end
