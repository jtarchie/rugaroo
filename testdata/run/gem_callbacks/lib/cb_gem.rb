# frozen_string_literal: true

# Callbacks reached only by public_send with a variable name, as rubocop's Commissioner calls its cops.
module CbGem
  class Node
    attr_reader :type, :children

    def initialize(type, children)
      @type = type
      @children = children
    end

    def send_type? = @type == :send
  end

  class Base
    def name = self.class.name
  end

  class CountCop < Base
    attr_reader :total

    def initialize = @total = 0

    def on_send(node)
      @total += node.children.size + node.type.size
      @total += 1 if node.send_type?
    end
  end

  class IntCop < Base
    attr_reader :total

    def initialize = @total = 0

    def on_int(node)
      @total += node.children.first
    end
  end

  class Commissioner
    def initialize(cops) = @cops = cops

    def trigger(callback, node)
      @cops.each { |cop| cop.public_send(callback, node) if cop.respond_to?(callback) }
    end

    def walk(node)
      trigger(:"on_#{node.type}", node)
      node.children.each { |child| walk(child) if child.is_a?(Node) }
    end
  end

  def self.tree(depth)
    return Node.new(:int, [depth + 1]) if depth.zero?

    Node.new(:send, [tree(depth - 1), tree(depth - 1), :name])
  end
end
