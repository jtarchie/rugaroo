# rbs_inline: enabled

# open-uri (#46) against an in-process WEBrick server, so the example needs no network.
require "open-uri"
require "logger"
require "webrick"

PAGES = {
  "/books.csv" => "title,year\nDune,1965\nNeuromancer,1984\nHyperion,1989\n",
  "/about" => "A tiny catalog."
} #: Hash[String, String]

quiet = Logger.new(File::NULL)
server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1", Logger: quiet, AccessLog: [])
server.mount_proc("/") do |req, res|
  body = PAGES[req.path]
  if body
    res["Content-Type"] = req.path.end_with?(".csv") ? "text/csv; charset=UTF-8" : "text/plain"
    res.body = body
  elsif req.path == "/catalog"
    res.set_redirect(WEBrick::HTTPStatus::MovedPermanently, "/books.csv")
  elsif req.path == "/admin"
    res.status = 403
    res.body = "members only"
  else
    res.status = 404
    res.body = "no page at #{req.path}"
  end
end
thread = Thread.new { server.start }
base = "http://127.0.0.1:#{server.config[:Port]}"

# Block form: the response is an IO, closed when the block ends; the block's value is returned.
books = URI.open("#{base}/books.csv") do |f|
  puts "status #{f.status.join(" ")}, #{f.content_type}, charset #{f.charset}"
  f.gets # header row
  f.each_line.map { |line| line.chomp.split(",") }
end
books.each { |title, year| puts "  #{title} (#{year})" }
puts "oldest: #{books.min_by { |_, year| year.to_i }&.first}"

# Blockless: the caller reads and closes it.
io = URI.open("#{base}/about")
puts "about: #{io.read} [#{io.content_type}, charset #{io.charset}]"
io.close

# A redirect is followed; base_uri is where the body came from.
URI.open("#{base}/catalog") do |f|
  puts "catalog redirected to #{f.base_uri&.path}, #{f.readlines.size - 1} books"
end

# Non-2xx responses raise OpenURI::HTTPError carrying the response as io.
%w[/admin /missing].each do |path|
  URI.open("#{base}#{path}")
rescue OpenURI::HTTPError => e
  puts "#{path}: #{e.message} -> #{e.io.read.inspect}"
end

# redirect: false turns the redirect itself into an error.
begin
  URI.open("#{base}/catalog", redirect: false)
rescue OpenURI::HTTPRedirect => e
  puts "not following: #{e.message} to #{e.uri.path}"
end

server.shutdown
thread.join
puts "done"
