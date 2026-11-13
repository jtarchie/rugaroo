# rbs_inline: enabled

# A class or module statement inside a block. Ruby runs it when the block
# runs; rugaroo declares it at compile time, where the block's constant
# scope is, so the program below works the same on both (decision 451).

PLUGINS = %w[upcase reverse].freeze

PLUGINS.each do |name|
  module Plugins
    class Upcase
      def call(s) = s.upcase
    end

    class Reverse
      def call(s) = s.reverse
    end
  end
  puts "registered #{name}"
end

word = "rugaroo"
puts Plugins::Upcase.new.call(word)
puts Plugins::Reverse.new.call(word)
