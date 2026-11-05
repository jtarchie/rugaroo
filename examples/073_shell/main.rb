# rbs_inline: enabled

# Shelling out: Kernel#system, backticks and $?.

files = `printf 'b.txt\na.rb\nc.rb\n'`.lines.map(&:chomp)
puts "listed #{files.size} names, status #{$?&.exitstatus}"

rubies = files.select { |f| f.end_with?(".rb") }.sort
puts "ruby files: #{rubies.join(", ")}"

count = `printf '%s\n' #{rubies.join(" ")} | wc -l`.strip
puts "wc counts #{count}"

puts "system says:"
ok = system("echo", "  args pass through untouched: $HOME *")
puts "succeeded: #{ok.inspect}"

ok = system("exit 2")
puts "exit 2 -> #{ok.inspect}, exitstatus #{$?&.exitstatus}"

missing = system("no_such_command_rugaroo")
puts "missing command -> #{missing.inspect}"
