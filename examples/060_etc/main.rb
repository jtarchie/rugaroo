# rbs_inline: enabled

require "etc"

# Etc via os/user and runtime; machine-specific values are checked structurally, not against literal expectations.

login = Etc.getlogin
puts "getlogin ok: #{login.nil? || login.is_a?(String)}"

puts "nprocessors ok: #{Etc.nprocessors.is_a?(Integer) && Etc.nprocessors > 0}"

tmpdir = Etc.systmpdir
puts "systmpdir ok: #{tmpdir.is_a?(String) && !tmpdir.empty?}"

pw = Etc.getpwuid
puts "getpwuid ok: #{pw.is_a?(Etc::Passwd)}"
puts "name ok: #{pw.name.is_a?(String) && !pw.name.empty?}"
puts "uid ok: #{pw.uid.is_a?(Integer) && pw.uid >= 0}"
puts "gid ok: #{pw.gid.is_a?(Integer) && pw.gid >= 0}"
puts "dir ok: #{pw.dir.is_a?(String) && !pw.dir.empty?}"
puts "gecos ok: #{pw.gecos.is_a?(String)}"

by_uid = Etc.getpwuid(pw.uid)
puts "getpwuid(uid) round-trips: #{by_uid.name == pw.name && by_uid.uid == pw.uid}"

by_name = Etc.getpwnam(pw.name)
puts "getpwnam round-trips: #{by_name.uid == pw.uid && by_name.dir == pw.dir}"

puts pw.members.inspect
puts pw.to_h.keys.inspect

begin
  Etc.getpwuid(999_999_999)
rescue ArgumentError => e
  puts "#{e.class}: #{e.message}"
end

begin
  Etc.getpwnam("no_such_rugaroo_user")
rescue ArgumentError => e
  puts "#{e.class}: #{e.message}"
end

p Etc::Passwd.new("alice", "x", 1000, 1000, "Alice Example", "/home/alice", "/bin/bash", 0, "", 0)
p Etc::Passwd.new("bob", "x", 1001, 1001, "Bob Example", "/home/bob")
p Etc::Passwd.members
