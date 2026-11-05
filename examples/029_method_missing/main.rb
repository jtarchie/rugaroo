# rbs_inline: enabled

# method_missing and respond_to? on statically typed receivers: an unknown
# method compiles to a method_missing call, typed by its signature.

class NullObject
  #: (Symbol, *untyped) -> untyped
  def method_missing(name, *args) = nil

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = true
end

class Settings
  #: () -> void
  def initialize
    @values = { "color" => "blue", "size" => "L" } #: Hash[String, String]
  end

  #: () -> String
  def to_s = "settings"

  #: (Symbol, *untyped) -> String
  def method_missing(name, *args)
    value = @values[name.to_s]
    return value if value

    "(no #{name}, #{args.size} args)"
  end

  #: (Symbol, ?bool) -> bool
  def respond_to_missing?(name, include_private = false) = @values.key?(name.to_s)
end

class Plain
  #: () -> String
  def hello = "hi"
end

settings = Settings.new
puts settings.color, settings.size, settings.weight, settings.weight(1, "two")
puts settings.respond_to?(:color), settings.respond_to?(:weight), settings.respond_to?(:to_s)
puts settings.respond_to?(:initialize)

null = NullObject.new
puts null.anything.inspect, null.deeply(1, "two").inspect, null.respond_to?(:x)
puts Plain.new.respond_to?(:hello), Plain.new.respond_to?(:bye), 5.respond_to?(:even?)
