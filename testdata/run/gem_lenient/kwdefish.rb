# frozen_string_literal: true

# A keyword whose default is another parameter of another class than the calls that name it pass (rubocop's
# `report_offense(node, ..., offense_range: node)`, a Node by default and a Range from two callers).
module Kwdefish
  Span = Struct.new(:from, :to)
  Node = Struct.new(:name)

  def self.report(node, *extra, span: node)
    out = "#{node.name}:#{extra.size}:#{label(span)}"
    out += yield(out.size).to_s if block_given?
    out
  end

  def self.label(where)
    where.is_a?(Span) ? "#{where.from}-#{where.to}" : "node #{where.name}"
  end
end
