# rbs_inline: enabled

# Regexp literals (Ruby syntax on Go's RE2), =~, !~, match, MatchData.

path = "/posts/42.json"
puts path =~ /\d+/, (path =~ /zzz/).inspect
puts path !~ /\.html$/, path !~ /json/
m = path.match(%r{^/(\w+)/(\d+)(\.\w+)?$})
puts m[0], m[1], m[2], m[3].inspect, m[4].inspect if m
puts path.match(/xml/).inspect
puts path.match?(/json/), "abc".match?(/^b/), "Hello".match?(/hello/i)

id = "42"
puts path.match?(%r{/#{id}(\.\w+)?/?$}), "/posts/43".match?(%r{/#{id}$})
puts "héllo wörld" =~ /w/

case path
when /\.html$/ then puts "html"
when /\.json$/ then puts "json"
else puts "other"
end

puts "line one\nline two".match?(/^line two$/)
