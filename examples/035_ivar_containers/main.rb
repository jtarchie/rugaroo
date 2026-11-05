# rbs_inline: enabled

# Destructuring `|path, n|` needs typed elements, so `busiest` only compiles once @visits is typed.
class Router
  #: () -> void
  def initialize
    @routes = []
    @hits = {}
    @log = []
    @visits = []
  end

  # @rbs path: String
  def add(path)
    @routes << path
    @hits[path] = 0
    self
  end

  # @rbs path: String
  def hit(path)
    @hits[path] = (@hits[path] || 0) + 1 # untyped until @hits is: `add` decides its values
    @log.push("hit #{path}")
    @visits << [path, @hits[path] || 0]
  end

  def busiest = @visits.max_by { |_, n| n }&.first

  def report = @routes.sort.map { |r| "#{r}=#{@hits[r]}" }.join(" ")

  def last = @log.last
end

class TracingRouter < Router
  def trace = @log.map { |l| l.upcase }
end

#: (Array[untyped]) -> void
def tag(xs)
  xs << "tag"
end

class Shelf
  #: () -> void
  def initialize
    @books = []
    @notes = []
  end

  def fill
    @books << "Dune"
    @notes << "n"
    tag(@notes) # typed, @notes would be copied into Array[untyped]: it stays untyped
    puts @books.map { |b| b.upcase }.inspect, @notes.inspect
  end
end

r = TracingRouter.new
r.add("/b").add("/a")
r.hit("/a")
r.hit("/a")
puts r.report, r.last.inspect, r.trace.inspect, r.busiest.inspect
Shelf.new.fill
