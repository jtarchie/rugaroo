# frozen_string_literal: true

require "registryish"

class RegistryishSub < Registryish
  def visit_int
    "int:#{node.last}"
  end

  def visit_str
    "str:#{node.last}"
  end

  def visit_other_type
    "other:#{node.first}"
  end
end
