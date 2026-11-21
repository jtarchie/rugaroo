# Runs the Snails scaffold app on MRI and rugaroo: Tep::APP served by WEBrick (Go's net/http under rugaroo), requests over real HTTP.
require_relative "shim/tep_net"
$LOADED_FEATURES << File.expand_path("snails-core/tep/net.rb", __dir__)
require "net/http"
require "json"
require "tmpdir"
require_relative "shim/tep_webrick"
require_relative "snails-core/boot"

port = 3917
ENV["PORT"] = port.to_s
ENV["DATABASE_PATH"] = File.join(Dir.mktmpdir, "demo.sqlite3")
IO.popen(["sqlite3", "-bail", ENV.fetch("DATABASE_PATH")], "w") { |io| io.write(File.read(File.join(__dir__, "db/migrate/0001_baseline.sqlite.sql"))) }

Thread.new do
  Tep::WEBrickServer.start_app(port)
rescue StandardError => e
  warn "server: #{e.class}: #{e.message}\n  #{e.backtrace&.first(12)&.join("\n  ")}"
end

http = Net::HTTP.new("127.0.0.1", port)
50.times do
  http.get("/up")
  break
rescue SystemCallError
  sleep 0.1
end

json = { "Content-Type" => "application/json" }
show = lambda do |label, res|
  body = res.body.to_s.gsub(/"\d{4}-\d\d-\d\dT[^"]*"/, '"T"')
  puts "#{label} #{res.code} #{body}"
end
show.call("GET /up", http.get("/up"))
show.call("POST /posts", http.post("/posts", JSON.generate({ "title" => "hello", "body" => "world" }), json))
show.call("POST /posts blank", http.post("/posts", JSON.generate({ "title" => "", "body" => "x" }), json))
show.call("GET /posts", http.get("/posts"))
show.call("GET /posts/1", http.get("/posts/1"))
show.call("PUT /posts/1", http.put("/posts/1", JSON.generate({ "title" => "renamed", "body" => "again" }), json))
show.call("GET /posts/1", http.get("/posts/1"))
show.call("PATCH /posts/1", http.patch("/posts/1", JSON.generate({ "title" => "renamed" }), json))
show.call("POST comment", http.post("/posts/1/comments", JSON.generate({ "body" => "nice" }), json))
show.call("GET comments", http.get("/posts/1/comments"))
show.call("DELETE /posts/1", http.delete("/posts/1"))
show.call("GET /posts/1", http.get("/posts/1"))
show.call("GET /nope", http.get("/nope"))
$stdout.flush
exit(0)
