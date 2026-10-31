module Demo
  autoload :Lazy, "demo/lazy"
  autoload :Config, "demo/config"
  autoload :Unnamed, "demo/unnamed"
  autoload :BothA, "demo/both"
  autoload :BothB, "demo/both"

  class Base
    def base = :base
  end

  class Thing < Base
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def greet
      "hello #{@name}"
    end

    # Nothing calls it, so it is never typed (decision 169): an &block with no signature would not compile.
    def never_called(&blk) = blk
  end

  # Nothing names it, so none of its methods is compiled (decision 169).
  class Unused
    def initialize(x) = @x = x.frobnicate(&nil)

    def broken(&blk) = blk
  end
end

# Nothing reads it, and a lambda has no effect to keep, so it is never compiled (decision 169): `binding` would not.
Demo::TOPLEVEL_BINDING_MAKER = ->(x) { x.instance_eval { binding } }

# MRI's defined? is true for an autoload it has not loaded yet, so this never runs (decision 167).
require_relative "demo/never" unless defined?(Demo::Lazy)
