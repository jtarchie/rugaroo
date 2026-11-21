# A block forwarded to a method that yields another type than its callers gave it (decision 484).
module Fwdblk
  Cn = Struct.new(:parent, :kind)

  def self.of(n)
    return nil unless n.parent

    k = { a: Ia, b: Ib }[n.kind]
    k ? k.new(n) : nil
  end

  class Base
    attr_reader :child

    def initialize(child)
      @child = child
    end

    def ctl
      child.parent
    end

    def parent
      return @parent if instance_variable_defined?(:@parent)

      @parent = Fwdblk.of(ctl)
    end

    def each_anc(include_self: false, &block)
      return to_enum(__method__, include_self: include_self) unless block

      yield self if include_self
      scan_anc(&block)
      self
    end

    def excl(other)
      other.each_anc(include_self: true) do |o|
        return !child.equal?(o.child) if ctl.equal?(o.ctl)
      end
      parent ? parent.excl(other) : false
    end

    private

    def scan_anc
      b = self
      while (b = b.parent)
        yield b
      end
    end
  end

  class Ia < Base; end
  class Ib < Base; end
end
