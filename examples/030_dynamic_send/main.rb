# rbs_inline: enabled

# Calls on untyped values dispatch at run time: to the method if the
# receiver's class has one, else to method_missing, else NoMethodError.
# send/public_send take literal or computed names; respond_to? works too.

class Greeter
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (?String) -> String
  def greet(greeting = "hello") = "#{greeting}, #{name}"

  #: (*String) -> String
  def shout(*words) = words.map(&:upcase).join(" ")

  #: () -> Integer?
  def lucky = nil
end

class Ghost
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = "ghost #{name}(#{args.size})"

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = name.to_s.start_with?("g")
end

things = [Greeter.new("ada"), Ghost.new] #: Array[untyped]
things.each do |thing|
  puts thing.greet, thing.greet("hi"), thing.shout("a", "b")
  puts thing.respond_to?(:greet), thing.respond_to?(:zap)
end

greeter = things.fetch(0)
puts greeter.send(:greet, "yo"), greeter.public_send(:name), greeter.lucky.inspect
["greet", "shout", "name"].each { |m| puts greeter.send(m) }
puts ["name", "fly"].map { |m| greeter.respond_to?(m) }.inspect

class_name = "Greeter"
klass = Object.const_get(class_name)
puts klass.new("bob").greet("hey"), klass.name

begin
  greeter.fly
rescue NoMethodError => e
  puts e.message
end

begin
  greeter.greet("a", "b", "c")
rescue ArgumentError => e
  puts e.message
end

nothing = nil #: untyped
begin
  nothing.greet
rescue NoMethodError => e
  puts e.message
end
