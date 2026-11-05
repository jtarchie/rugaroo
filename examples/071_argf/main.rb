# rbs_inline: enabled

require "tmpdir"

# ARGF: a `cat -n` style filter over the files named in ARGV (stdin when
# there are none). Here the program names two files itself.

Dir.mktmpdir do |dir|
  %w[one two].each_with_index do |name, i|
    File.write(File.join(dir, "#{name}.txt"), "#{name} line 1\n#{name} line #{i + 2}\n")
    ARGV << File.join(dir, "#{name}.txt")
  end

  $stdout.sync = true
  n = 0
  current = "" #: String
  while (line = gets)
    name = File.basename(ARGF.filename)
    if name != current
      puts "== #{name}"
      current = name
    end
    n += 1
    puts format("%3d  %s", n, line)
  end
  puts "#{n} lines, ARGV now #{ARGV.inspect}"
end
