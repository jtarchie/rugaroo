# rbs_inline: enabled

# Parse Ruby with the prism gem: rugaroo compiles the gem's own Ruby and runs prism.wasm under it (decision 427).
require "prism"

SOURCE = <<~RUBY
  # Greets someone.
  def greet(name) = "Hello, \#{name}!"
  puts greet("world")
RUBY

# Collects each method call's name, in source order.
class CallNames < Prism::Visitor
  attr_reader :names #: Array[Symbol]

  def initialize
    super
    @names = [] #: Array[Symbol]
  end

  #: (Prism::CallNode) -> void
  def visit_call_node(node)
    @names << node.name
    super
  end
end

result = Prism.parse(SOURCE)
puts result.value.inspect
puts "comments: #{result.comments.map { |c| c.location.slice }.inspect}"

calls = CallNames.new
result.value.accept(calls)
puts "calls: #{calls.names.inspect}"

broken = Prism.parse("def oops(")
puts "errors: #{broken.errors.map { |e| "#{e.location.start_line}: #{e.message}" }.inspect}"
