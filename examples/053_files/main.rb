# rbs_inline: enabled

require "tmpdir"

# File and Dir: read a fixture next to the program, write and reread files
# in a temp dir, and rescue the Errno classes MRI raises.

#: (String) -> Hash[String, Integer]
def load_scores(path)
  scores = {} #: Hash[String, Integer]
  File.foreach(path) do |line|
    name, score = line.chomp.split(",")
    next if name.nil?
    next if name == "name"

    scores[name] = (score || "0").to_i
  end
  scores
end

scores = load_scores("scores.csv")
scores.sort_by { |_, s| -s }.each { |n, s| puts "#{n}: #{s}" }
puts "lines: #{File.readlines("scores.csv").size}"
puts "exist? #{File.exist?("scores.csv")} file? #{File.file?("scores.csv")} dir? #{File.directory?("scores.csv")}"
puts "size: #{File.size("scores.csv")}"

%w[a/b.rb /usr/lib/ report.tar.gz .profile x.].each do |p|
  puts "#{p}: #{File.basename(p)} #{File.basename(p, ".*")} #{File.dirname(p)} #{File.extname(p).inspect}"
end
puts File.join("logs", "2026", "app.log")
puts File.join("root/", "/leaf")
puts File.absolute_path?(File.expand_path("scores.csv"))
puts File.expand_path("scores.csv") == File.join(Dir.pwd, "scores.csv")

Dir.mktmpdir do |dir|
  report = File.join(dir, "report.txt")
  n = File.write(report, "total #{scores.values.sum}\n")
  puts "wrote #{n} bytes"

  File.open(report, "a") do |f|
    f.puts "best #{scores.max_by { |_, s| s }&.first}"
    f.print "count ", scores.size, "\n"
    f << "done" << "\n"
    f.printf("%05.1f\n", 72.25)
  end
  print File.read(report)

  first = File.open(report) do |f|
    f.gets
  end
  p first

  File.open(report) do |f|
    f.each_line { |l| puts "> #{l}" }
    puts "eof? #{f.eof?}"
    p f.gets
  end

  Dir.mkdir(File.join(dir, "sub"))
  File.write(File.join(dir, "sub", "x.txt"), "x")
  File.rename(report, File.join(dir, "final.txt"))
  p Dir.children(dir).sort
  p Dir.entries(dir).sort
  p Dir.glob(File.join(dir, "*.txt")).map { |f| File.basename(f) }
  puts "dir exist? #{Dir.exist?(File.join(dir, "sub"))}"

  begin
    Dir.rmdir(File.join(dir, "sub"))
  rescue SystemCallError => e
    puts "#{e.class}: not empty" if e.is_a?(Errno::ENOTEMPTY)
  end
  File.delete(File.join(dir, "sub", "x.txt"))
  Dir.rmdir(File.join(dir, "sub"))
  p Dir.children(dir)

  begin
    Dir.mkdir(dir)
  rescue Errno::EEXIST
    puts "EEXIST"
  end
end

begin
  File.read("missing.txt")
rescue Errno::ENOENT => e
  puts "#{e.class}: #{e.message}"
end

begin
  File.read(".")
rescue Errno::EISDIR => e
  puts "#{e.class}: #{e.message}"
end

begin
  File.open("scores.csv") { |f| f.write("nope") }
rescue IOError => e
  puts "IOError: #{e.message}"
end

begin
  Dir.children("no_such_dir")
rescue SystemCallError => e
  puts e.message
end
