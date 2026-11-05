# rbs_inline: enabled

# `extend Enumerable` on a module whose `each` walks its own constants,
# `&block` parameters forwarded into iterators and closures, and user code
# reopening a core class.

class String
  #: () -> String
  def camelize = split("_").map(&:capitalize).join
end

module Handlers
  extend Enumerable #[singleton(Handlers::Base)]

  class Base
    #: (String) -> bool
    def self.matches?(path) = false
  end

  class Posts < Base
    def self.matches?(path) = path.start_with?("/posts")
  end

  class Users < Base
    def self.matches?(path) = path.start_with?("/users")
  end

  # MRI orders constants by its symbol table; sort for a stable order.
  #: () { (singleton(Base)) -> void } -> void
  def self.each(&block)
    constants.sort.map { |c| const_get(c) }.each(&block)
  end
end

class Registry
  #: () -> void
  def initialize
    @names = [] #: Array[String]
  end

  #: (String) -> self
  def add(name)
    @names << name
    self
  end

  # forwarded into an iterator
  #: () { (String) -> void } -> void
  def each(&block) = @names.each(&block)

  # forwarded into a closure
  #: () { (String) -> String } -> Array[String]
  def transform(&block) = @names.map(&block)

  # called directly
  #: () { (String) -> String } -> String
  def first_transformed(&block) = block.call(@names[0].to_s)
end

puts Handlers.detect { |h| h.matches?("/users/1") }.inspect
puts Handlers.map { |h| h.name }.inspect, Handlers.count
puts Handlers.select { |h| h.matches?("/posts/9") }.inspect
puts "user_sessions".camelize, "posts".camelize

registry = Registry.new.add("a").add("b")
registry.each { |name| puts name }
puts registry.transform { |name| name.upcase }.inspect
puts registry.first_transformed { |name| name * 3 }
