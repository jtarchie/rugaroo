# frozen_string_literal: true

# A token literal whose first element is typed a round after the literal is first seen (prism's
# `builder.integer([node.value, srange(node.location)])`): the element not known yet must not drop out of the
# literal's type, or the token is an Array of its other element's class and the value is converted to it at run time.
class LitNode
  def initialize(fields)
    @fields = fields
  end

  # untyped once the rounds resolve: the fields are a mixed Array
  def value
    @fields[0]
  end
end

class LitBuilder
  def integer(token)
    numeric(:int, token)
  end

  def numeric(kind, token)
    "#{kind}:#{token[0]}:#{token[1].size}"
  end
end

class LitVisitor
  def initialize
    @builder = LitBuilder.new
  end

  def visit(node)
    @builder.integer([node.value, (1..3)])
  end

  def visit_hash(node)
    opts = { value: node.value, span: (1..4) }
    "#{opts[:value]}:#{opts[:span].size}"
  end
end
