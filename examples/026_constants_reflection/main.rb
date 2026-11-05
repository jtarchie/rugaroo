# rbs_inline: enabled

# Class objects for every class and module; constants, const_get,
# const_defined?, Object.const_get with paths. MRI orders `constants` by its
# symbol table, so this example sorts before printing.

module Formats
  VERSION = "2.0" #: String

  class Base
    #: (String) -> bool
    def self.matches?(path) = false
  end

  class HTML < Base
    def self.matches?(path) = path.end_with?(".html")
  end

  class JSON < Base
    def self.matches?(path) = path.end_with?(".json")
  end
end

module Handlers
  class Base
    #: () -> String
    def self.kind = "base"
  end

  class Index < Base
    def self.kind = "index"
  end

  class Show < Base
    def self.kind = "show"
  end
end

# Every constant of Handlers is a class under Base, so const_get is typed
# singleton(Handlers::Base) and class methods can be called on the result.
#: (String) -> String
def kind_for(name) = Handlers.const_get(name).kind

puts Formats.name, Formats::HTML.name, Formats.class, Formats::HTML.class
puts String, String.class, Integer.name, 42.class, "s".class == String
puts Formats.constants.sort.inspect, Handlers.constants.sort.inspect
puts Formats.const_defined?(:HTML), Formats.const_defined?("XML")
puts kind_for("Index"), kind_for("Show"), Handlers.const_get(:Base).kind
puts Formats.const_get(:VERSION), Object.const_get("Formats::JSON"), Object.const_get(:Handlers)
puts Object.const_get("Formats::JSON") == Formats::JSON

matching = Handlers.constants.sort.map { |c| Handlers.const_get(c) }.select { |k| k.kind.size > 4 }
puts matching.inspect

begin
  Formats.const_get(:XML)
rescue NameError => e
  puts "NameError: #{e.message}"
end

begin
  Object.const_get("Formats::Nope::Deeper")
rescue NameError => e
  puts "NameError: #{e.message}"
end
