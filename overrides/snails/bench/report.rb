# Median RPS and p95 per contender, route and VU level across rounds, from k6 --summary-export files.
require "json"

dir = ARGV.fetch(0)
rows = Hash.new { |h, k| h[k] = [] }
Dir[File.join(dir, "*.json")].each do |path|
  c, route, vus, = File.basename(path, ".json").split("-")
  m = JSON.parse(File.read(path)).fetch("metrics")
  failed = m.dig("http_req_failed", "value") || m.dig("http_req_failed", "rate") || 0
  rows[[route, vus.to_i, c]] << [m.dig("http_reqs", "rate"), m.dig("http_req_duration", "p(95)"), failed]
end
median = ->(xs) { xs.sort[xs.size / 2] }
rss = File.exist?(File.join(dir, "rss-kb.txt")) ? File.readlines(File.join(dir, "rss-kb.txt")).map(&:split).group_by(&:first).transform_values { |v| v.map { _1[1].to_i }.max } : {}
puts "| route | VUs | contender | RPS (median) | p95 ms (median) | failed |"
puts "|---|---|---|---|---|---|"
rows.keys.sort_by { |r, v, c| [r, v, c] }.each do |key|
  route, vus, c = key
  runs = rows[key]
  puts format("| %s | %d | %s | %d | %.2f | %.4f |", route, vus, c, median.call(runs.map(&:first)), median.call(runs.map { _1[1] }), runs.map(&:last).max)
end
puts
rss.each { |c, kb| puts "peak RSS #{c}: #{kb / 1024} MiB" }
