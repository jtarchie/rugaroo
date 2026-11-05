# rbs_inline: enabled

require "forwardable"

# Forwardable: a class hands methods to an object it holds.
class Playlist
  extend Forwardable
  include Enumerable #[String]

  def_delegators :@songs, :size, :each, :<<, :first, :empty?
  def_delegator :@songs, :join, :to_text
  delegate %i[keys fetch] => :@ratings

  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
    @songs = [] #: Array[String]
    @ratings = {} #: Hash[String, Integer]
  end

  #: (String, Integer) -> void
  def rate(song, stars)
    @ratings[song] = stars
  end
end

mix = Playlist.new("road trip")
mix << "Holiday" << "Roadrunner"
mix << "Drive"
puts "#{mix.name}: #{mix.size} songs, first #{mix.first.inspect}"
puts mix.to_text(" / ")
puts "sorted by Enumerable over the delegated each: #{mix.sort.inspect}"
puts "long titles: #{mix.select { |s| s.size > 5 }.inspect}"

mix.rate("Drive", 5)
puts "rated: #{mix.keys.inspect}"
puts "Drive: #{mix.fetch("Drive")}, Holiday: #{mix.fetch("Holiday", 0)}"
