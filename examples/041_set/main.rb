# rbs_inline: enabled

# Tag index: which posts share tags, via Set algebra.
class Post
  attr_reader :title #: String
  attr_reader :tags #: Set[String]

  #: (String, Array[String]) -> void
  def initialize(title, tags)
    @title = title
    @tags = Set.new(tags)
  end
end

#: (Post, Post) -> Float
def jaccard(a, b)
  union = a.tags | b.tags
  return 0.0 if union.empty?

  (a.tags & b.tags).size.to_f / union.size
end

posts = [
  Post.new("Go generics", %w[go generics types]),
  Post.new("Ruby sets", %w[ruby types collections ruby]),
  Post.new("Typed Ruby to Go", %w[ruby go types])
]

all_tags = Set.new #: Set[String]
posts.each { |p| all_tags.merge(p.tags) }
puts all_tags.inspect, all_tags.size
puts posts[1].tags.inspect

posts.each_with_index do |a, i|
  posts.each_with_index do |b, j|
    next unless i < j

    puts "#{a.title} ~ #{b.title}: #{jaccard(a, b).round(2)} shared=#{(a.tags & b.tags).to_a.sort.inspect}"
  end
end

go = posts[0].tags
ruby = posts[1].tags
puts (go ^ ruby).inspect, (go - ruby).inspect, go.disjoint?(Set["java"]), go.intersect?(ruby)
puts Set[1, 2] <= Set[1, 2, 3], Set[1, 2] < Set[1, 2], Set[3, 1] == Set[1, 3]

seen = Set.new #: Set[Integer]
dups = [3, 1, 4, 1, 5, 9, 2, 6, 5, 3].reject { |n| seen.add?(n) }
puts dups.inspect, seen.to_a.inspect, seen.sort.inspect, seen.select(&:even?).inspect

by_set = { Set[1, 2] => "pair" }
puts by_set[Set[2, 1]].inspect

case 9
when seen then puts "9 seen"
end
