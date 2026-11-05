# rbs_inline: enabled

# to_json on core types and user classes; JSON.generate.
require "json"

class Post
  attr_reader :id #: Integer?
  attr_reader :title #: String

  #: (Integer?, String) -> void
  def initialize(id, title)
    @id = id
    @title = title
  end

  # Array#to_json passes a generator state to each element.
  #: (*untyped) -> String
  def to_json(*_state) = { "post" => { "id" => id, "title" => title } }.to_json
end

puts "plain \"quoted\" / \\ \n\t".to_json, 42.to_json, 1.5.to_json, true.to_json, nil.to_json
puts [1, "two", nil, 3.0].to_json, [[1, 2], []].to_json
puts({ "a" => 1, "b" => [true, false], "c" => { "d" => nil } }.to_json)
puts({ name: "sym", "é" => "ünï" }.to_json)
puts JSON.generate({ "k" => "v" }), JSON.generate([1, 2])
puts Post.new(1, "Hi").to_json, Post.new(nil, "New").to_json
posts = [Post.new(1, "a"), Post.new(2, "b")]
puts posts.to_json
puts [1e20, 1.23e-5, 1e-10, 100.0, -0.0, 12.5, 1e15, 1e7, 0.1, 5e-324].to_json
puts "\u0001\u001f\u007f\e".to_json
