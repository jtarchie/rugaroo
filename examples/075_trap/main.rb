# rbs_inline: enabled

# Signal handlers: trap runs a block when a signal arrives.

events = Queue.new #: Queue[String]

trap("USR1") { |_| events << "reload requested" }
trap("USR2") { |_| events << "stats requested" }
trap("TERM") { |_| events << "shutdown requested" }
trap("EXIT") { |_| puts "cleaning up on exit" }

%w[USR1 USR2 USR1 TERM].each do |sig|
  Process.kill(sig, Process.pid)
  puts "handled: #{events.pop}"
end

trap("USR1", "IGNORE")
puts "USR1 ignored; restoring the default: #{trap("USR1", "DEFAULT").inspect}"
puts "done"
