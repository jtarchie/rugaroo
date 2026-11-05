# rbs_inline: enabled

require "open3"

# Open3 runs commands without a shell: capture2/capture2e/capture3 collect output, popen3 streams it through a block.

out, status = Open3.capture2("echo", "hello world")
puts out
puts status.success?
puts status.exitstatus

merged, status2 = Open3.capture2e("sh", "-c", "echo out; echo err 1>&2")
puts merged
puts status2.success?

out3, err3, status3 = Open3.capture3("sh", "-c", "echo out; echo err 1>&2; exit 3")
puts out3
puts err3
puts status3.exitstatus
puts status3.success?

data, err_data, code = Open3.popen3("cat") do |stdin, stdout, stderr, wait_thr|
  stdin.puts "piped"
  stdin.close
  puts stdin.closed?
  out4 = stdout.read
  puts stdout.eof?
  errd = stderr.read
  puts wait_thr.pid > 0
  [out4, errd, wait_thr.value.exitstatus]
end
puts data
puts err_data.empty?
puts code

begin
  Open3.capture2("rugaroo_missing_command_xyz")
rescue Errno::ENOENT => e
  puts e.message
end
