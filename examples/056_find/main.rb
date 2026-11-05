# rbs_inline: enabled

require "find"
require "tmpdir"

# Find.find over a scratch tree: plain traversal, Find.prune, break, multiple roots, and the missing-root error.

#: (String, String) -> String
def rel(dir, path) = path.sub(dir, ".")

Dir.mktmpdir do |dir|
  Dir.mkdir(File.join(dir, "a"))
  Dir.mkdir(File.join(dir, "a", "b"))
  Dir.mkdir(File.join(dir, "a", "c"))
  Dir.mkdir(File.join(dir, "skip_me"))
  Dir.mkdir(File.join(dir, "z"))
  File.write(File.join(dir, "a", "f1.txt"), "x")
  File.write(File.join(dir, "a", "b", "f2.txt"), "x")
  File.write(File.join(dir, "skip_me", "ignored.txt"), "x")
  File.write(File.join(dir, "z", "f3.txt"), "x")
  File.write(File.join(dir, "top.txt"), "x")

  puts "== plain =="
  Find.find(dir) { |path| puts rel(dir, path) }

  puts "== prune skip_me =="
  Find.find(dir) do |path|
    if File.basename(path) == "skip_me"
      Find.prune
    else
      puts rel(dir, path)
    end
  end

  puts "== break at top.txt =="
  Find.find(dir) do |path|
    break if File.basename(path) == "top.txt"

    puts rel(dir, path)
  end

  puts "== multiple roots =="
  Find.find(File.join(dir, "a"), File.join(dir, "z")) { |path| puts rel(dir, path) }
  nil
end

puts "== missing root =="
begin
  Find.find("/no/such/rugaroo/find/target") { |path| puts path }
rescue Errno::ENOENT => e
  puts "#{e.class}: #{e.message}"
end
