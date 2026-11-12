module Hookish
  class Base
    def begin_investigation
      on_new_investigation
      self
    end

    private

    def on_new_investigation
      # hook
    end
  end

  module ReqLib
    def on_send(name)
      return if name.nil?

      @required_libs.add(name)
    end

    def libs = @required_libs.to_a

    private

    def on_new_investigation
      # Holds the required files at top-level
      @required_libs = Set.new
      super
    end
  end

  class Cop < Base
    include ReqLib

    def english? = @english

    def on_new_investigation
      super
      @english = true
    end
  end

  class Plain < Base
    include ReqLib
  end

  def self.demo
    h = { c: Cop.new, p: Plain.new } #: Hash[Symbol, untyped]
    out = []
    h.each_value do |cop|
      cop.begin_investigation
      cop.on_send("English")
      out << cop.libs
    end
    out << h[:c].english?
    out
  end
end
