# frozen_string_literal: true

# rubocop-ast's NodePattern::Compiler::Subcompiler: a visitor registry filled by `method_added` and copied to subclasses
# by `inherited`'s `base.class_eval { @registry = us.registry.dup }`, read through `self.class.registry`.
class Registryish
  def initialize
    @node = nil
  end

  def compile(node)
    prev = @node
    @node = node
    do_compile
  ensure
    @node = prev
  end

  private

  attr_reader :node

  def do_compile
    send(self.class.registry.fetch(node.first, :visit_other_type))
  end

  @registry = {}
  class << self
    attr_reader :registry

    def method_added(method)
      @registry[Regexp.last_match(1).to_sym] = method if method =~ /^visit_(.*)/
      super
    end

    def inherited(base)
      us = self
      base.class_eval { @registry = us.registry.dup }
      super
    end
  end
end
