# rbs_inline: enabled

# A WEBrick server and a Net::HTTP client in one process.
require "net/http"
require "webrick"

server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
server.mount_proc("/") do |req, res|
  if req.path == "/missing"
    res.status = 404
    res.body = "no such page"
  else
    res["Content-Type"] = "text/plain"
    res["X-Method"] = req.request_method
    res.body = "#{req.request_method} #{req.path} #{req.query.inspect}"
  end
end
thread = Thread.new { server.start }

http = Net::HTTP.new("127.0.0.1", server.config[:Port])
form = { "Content-Type" => "application/x-www-form-urlencoded" }
[
  http.get("/hello?name=world&x=1"),
  http.get("/missing"),
  http.post("/items", URI.encode_www_form("item[name]" => "a b", "qty" => "2"), form),
  http.put("/items/1", "qty=3", form)
].each do |res|
  puts "#{res.code} #{res["Content-Type"].inspect} #{res["x-method"].inspect}"
  puts "  #{res.body}"
end

server.shutdown
thread.join
puts "stopped"
