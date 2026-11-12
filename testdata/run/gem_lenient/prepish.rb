module Prepish
  class Nd
    attr_reader :type, :children

    def initialize(type, children)
      @type = type
      @children = children
    end
  end

  module Sx
    def s(type, *children)
      Nd.new(type, children)
    end
  end

  module Discount
    include Sx

    def initialize(node, discount: false)
      super(node)
      return unless discount

      shared = {}
      @known = { s(:self) => shared, nil => shared }
    end

    def discount?
      defined?(@known)
    end

    def calc(x)
      discount? ? super + @known.size : super
    end
  end

  class Calc
    prepend Discount

    def self.calculate(node, discount: false)
      new(node, discount: discount).calc(1)
    end

    def initialize(node)
      @node = node
    end

    def calc(x)
      x + @node.size
    end
  end
end
