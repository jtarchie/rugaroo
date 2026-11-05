# rbs_inline: enabled

require "pp"

# pp: nested data laid out to the terminal's width (79 columns on a pipe),
# with Ruby 3.4's `key: value` hashes, and a class's own pretty_print.

Server = Struct.new(:host, :port, :tags) #: [String, Integer, Array[String]]

class Grid
  #: (Array[Array[Integer]]) -> void
  def initialize(rows)
    @rows = rows
  end

  #: (PP) -> void
  def pretty_print(q)
    q.group(1, "Grid[", "]") do
      q.seplist(@rows) { |row| q.pp row }
    end
  end
end

config = {
  name: "deploy",
  servers: [
    Server.new("app-1.example.com", 8080, %w[web primary]),
    Server.new("app-2.example.com", 8080, %w[web replica]),
    Server.new("db.example.com", 5432, ["database"])
  ],
  retries: { attempts: 3, backoff: [0.5, 1.0, 2.0, 4.0, 8.0, 16.0, 32.0, 64.0, 128.0, 256.0] },
  "notes" => "line one\nline two\n"
} #: Hash[untyped, untyped]

pp config
pp Grid.new([[1, 2, 3], [4, 5, 6], [7, 8, 9]])
pp Grid.new((1..8).map { |i| (1..10).map { |j| i * j } })
nested = [1, [2, [3, [4]]]] #: Array[untyped]
s = nested.pretty_inspect
puts "pretty_inspect: #{s.inspect}"
value = pp({ answer: 42 })
puts "pp returns its argument: #{value.inspect}"
