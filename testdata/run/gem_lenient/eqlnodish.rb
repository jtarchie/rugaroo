# frozen_string_literal: true

# The ast gem's Node: eql? (an untyped && chain) and a stored hash, so Array#& and uniq match equal nodes, as
# Style/ComparableBetween's `(min_and_value & max_and_value).first` needs (decision 497).
class EqlNodish
  attr_reader :type, :children

  def initialize(type, children)
    @type = type
    @children = children.freeze
    @hash = [@type, @children, self.class].hash
  end

  def eql?(other)
    self.class.eql?(other.class) && @type.eql?(other.type) && @children.eql?(other.children)
  end

  def ==(other)
    if equal?(other)
      true
    elsif other.respond_to? :to_ast
      other = other.to_ast
      other.type == type && other.children == children
    else
      false
    end
  end

  def to_ast = self

  attr_reader :hash
end

class EqlNodishInter
  def first_common(a, b) = (a & b).first
end
