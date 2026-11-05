# rbs_inline: enabled

require "pathname"
require "tmpdir"

# Pathname: a fluent OO wrapper over File/Dir (decision 62), delegating path
# math and filesystem checks instead of reimplementing them (decision 65).

#: (Pathname) -> void
def describe(p)
  puts "#{p}: basename=#{p.basename} dirname=#{p.dirname} extname=#{p.extname.inspect} parent=#{p.parent}"
end

describe(Pathname.new("a/b/report.tar.gz"))
describe(Pathname.new("/usr/lib/"))
describe(Pathname.new(".profile"))
puts Pathname.new("report.tar.gz").basename(".gz")
puts Pathname.new("report.tar.gz").basename(".*")

root = Pathname("logs")
puts (root / "2026" / "app.log")
puts (root + "2026" + "app.log")
puts root.join("2026", "app.log")
puts root.join("/abs")

p Pathname.new("/a") == Pathname.new("/a")
p Pathname.new("/a") == "/a"
p Pathname.new("/a").eql?(Pathname.new("/a"))
p(Pathname.new("/a") <=> Pathname.new("/b"))
p(Pathname.new("/b") <=> Pathname.new("/a"))
p(Pathname.new("/a") <=> Pathname.new("/a"))
p [Pathname.new("/b"), Pathname.new("/a")].sort.map(&:to_s)

Pathname.new("a/b/c.txt").each_filename { |part| print part, "|" }
puts

deep = Pathname.new("/tmp/project/src/lib")
other = Pathname.new("/tmp/project/build")
puts deep.relative_path_from(other)
puts other.relative_path_from(deep)
puts deep.relative_path_from(deep)
begin
  Pathname.new("rel/path").relative_path_from("/abs")
rescue ArgumentError => e
  puts "ArgumentError: #{e.class}"
end

Dir.mktmpdir do |dir|
  root = Pathname.new(dir)
  (root / "a.txt").write("alpha\n")
  (root / "b.txt").write("beta\n")
  Dir.mkdir((root / "sub").to_s)
  (root / "sub" / "c.txt").write("gamma\n")

  puts "exist? #{(root / "a.txt").exist?} file? #{(root / "a.txt").file?} dir? #{(root / "sub").directory?}"
  puts (root / "a.txt").read
  print (root / "sub" / "c.txt").read

  names = root.children.map { |c| c.relative_path_from(root).to_s }.sort
  p names
  p root.children(false).map(&:to_s).sort

  sub = root / "sub"
  p sub.relative_path_from(root).to_s
  p root.relative_path_from(sub).to_s

  missing = root / "nope.txt"
  puts "missing exist? #{missing.exist?}"
end
