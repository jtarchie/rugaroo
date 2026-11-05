# rbs_inline: enabled

require "tsort"

class BuildGraph
  include TSort #[String]

  #: () -> void
  def initialize
    @deps = {} #: Hash[String, Array[String]]
  end

  #: (String, *String) -> void
  def task(name, *deps)
    @deps[name] = deps
  end

  #: () { (String) -> void } -> void
  def tsort_each_node(&block) = @deps.each_key(&block)

  #: (String) { (String) -> void } -> void
  def tsort_each_child(node, &block) = @deps.fetch(node, []).each(&block)
end

graph = BuildGraph.new
graph.task("parse")
graph.task("compile", "parse")
graph.task("link", "compile")
graph.task("test", "link")
graph.task("package", "compile", "link")
graph.task("deploy", "package", "test")

puts graph.tsort.inspect
puts graph.strongly_connected_components.inspect
graph.tsort_each { |t| print t, " " }
puts

cycle = BuildGraph.new
cycle.task("a", "b")
cycle.task("b", "c")
cycle.task("c", "a")

begin
  cycle.tsort
rescue TSort::Cyclic => e
  puts "cycle: #{e.message}"
end
puts cycle.strongly_connected_components.inspect
