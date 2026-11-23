# frozen_string_literal: true

# rubocop's Lint/ConstantReassignment: `each_ancestor(...).reverse_each` with no block on the Enumerator a blockless
# each_ancestor returns, then with_object (decision 497).
class AncestorWalkish
  attr_reader :parent, :type
  def initialize(type, parent) = (@type, @parent = type, parent)

  def each_ancestor(*types, &block)
    return to_enum(__method__, *types) unless block

    visit_ancestors(types, &block)
    self
  end

  def visit_ancestors(types)
    last_node = self
    while (current_node = last_node.parent)
      yield current_node if types.include?(current_node.type)
      last_node = current_node
    end
  end
end

class AncestorWalkishCop
  def run(node) = ancestor_namespaces(node)

  def ancestor_namespaces(node)
    ancestors = node.each_ancestor(:class, :module).reverse_each
    ancestors.with_object([]) { |x, acc| acc << x.type }
  end

  def others(node)
    node.each_ancestor(:class).reverse_each { |x| x }
    node.each_ancestor(:module).each_with_index.map { |x, i| [x.type, i] }
  end
end
