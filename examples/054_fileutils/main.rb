# rbs_inline: enabled

require "fileutils"
require "tempfile"
require "tmpdir"

# FileUtils and Tempfile, built and torn down entirely under Dir.mktmpdir so nothing here touches real paths or prints anything nondeterministic.

Dir.mktmpdir do |dir|
  FileUtils.mkdir_p(File.join(dir, "a/b/c"))
  puts "mkdir_p nested: #{Dir.exist?(File.join(dir, "a/b/c"))}"

  FileUtils.mkdir_p(dir) # already exists: no error
  puts "mkdir_p idempotent ok"

  FileUtils.mkdir_p([File.join(dir, "x"), File.join(dir, "y")])
  puts "mkdir_p array: #{Dir.exist?(File.join(dir, "x"))} #{Dir.exist?(File.join(dir, "y"))}"

  FileUtils.touch(File.join(dir, "a/b/c/f.txt"))
  puts "touch: #{File.exist?(File.join(dir, "a/b/c/f.txt"))}"

  FileUtils.touch([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])
  puts "touch array: #{Dir.children(File.join(dir, "x")).sort}"

  # cp into a file, then into a directory (MRI: dest/basename(src))
  FileUtils.cp(File.join(dir, "a/b/c/f.txt"), File.join(dir, "a/b/c/f2.txt"))
  puts "cp file: #{File.exist?(File.join(dir, "a/b/c/f2.txt"))}"

  FileUtils.cp(File.join(dir, "a/b/c/f.txt"), File.join(dir, "y"))
  puts "cp into dir: #{File.exist?(File.join(dir, "y/f.txt"))}"

  FileUtils.cp([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")], File.join(dir, "y"))
  puts "cp array into dir: #{Dir.children(File.join(dir, "y")).sort}"

  # cp_r: recursive copy of a directory tree
  FileUtils.cp_r(File.join(dir, "a"), File.join(dir, "acopy"))
  puts "cp_r tree: #{Dir.exist?(File.join(dir, "acopy/b/c"))} #{File.exist?(File.join(dir, "acopy/b/c/f.txt"))}"

  # mv: rename in place, then into an existing directory
  FileUtils.mv(File.join(dir, "acopy"), File.join(dir, "amoved"))
  puts "mv rename: #{Dir.exist?(File.join(dir, "acopy"))} #{Dir.exist?(File.join(dir, "amoved"))}"

  FileUtils.mkdir_p(File.join(dir, "movedest"))
  FileUtils.mv(File.join(dir, "amoved"), File.join(dir, "movedest"))
  puts "mv into dir: #{Dir.exist?(File.join(dir, "movedest/amoved"))}"

  # ln_s: symlink to a file, and into a directory
  FileUtils.ln_s(File.join(dir, "a/b/c/f.txt"), File.join(dir, "link1"))
  puts "ln_s: #{File.symlink?(File.join(dir, "link1"))}"

  FileUtils.mkdir_p(File.join(dir, "linkdest"))
  FileUtils.ln_s(File.join(dir, "a/b/c/f.txt"), File.join(dir, "linkdest"))
  puts "ln_s into dir: #{File.symlink?(File.join(dir, "linkdest/f.txt"))}"

  # rm/rm_f/rm_rf
  FileUtils.rm_f(File.join(dir, "does_not_exist.txt"))
  puts "rm_f missing ok"

  begin
    FileUtils.rm(File.join(dir, "does_not_exist.txt"))
  rescue Errno::ENOENT => e
    puts "rm missing: #{e.class}"
  end

  FileUtils.rm(File.join(dir, "a/b/c/f2.txt"))
  puts "rm file: #{File.exist?(File.join(dir, "a/b/c/f2.txt"))}"

  FileUtils.rm([File.join(dir, "x/g1.txt"), File.join(dir, "x/g2.txt")])
  puts "rm array: #{Dir.children(File.join(dir, "x"))}"

  FileUtils.rm_rf(File.join(dir, "a"))
  puts "rm_rf: #{Dir.exist?(File.join(dir, "a"))}"

  FileUtils.rm_rf(File.join(dir, "a")) # already gone: no error
  puts "rm_rf idempotent ok"

  # Tempfile.new: object-style, explicit close/unlink
  t = Tempfile.new("scratch-")
  tpath = t.path
  puts "tempfile basename: #{tpath ? File.basename(tpath).start_with?("scratch-") : false}"
  t.write("hello tempfile\n")
  t.close
  puts "tempfile roundtrip: #{tpath ? File.read(tpath) : nil}"
  t.unlink
  puts "tempfile unlinked: #{t.path.nil?} #{tpath ? !File.exist?(tpath) : false}"

  # Tempfile.create: block form, auto-unlinked at block exit, yields a File
  created_path = nil #: String?
  Tempfile.create("scratch-block-") do |f|
    created_path = f.path
    f.write("block body\n")
    f.close
    puts "tempfile create body: #{File.read(f.path)}"
  end
  puts "tempfile create cleaned up: #{created_path ? !File.exist?(created_path) : false}"
end
