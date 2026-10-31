# frozen_string_literal: true

require "registryish"

class RegistryishSub2 < Registryish
  def visit_float
    "float:#{node.last}"
  end
end
