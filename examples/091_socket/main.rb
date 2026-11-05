# rbs_inline: enabled

# A TCP echo server and its client in one process, then a UDP round trip.
require "socket"

server = TCPServer.new("127.0.0.1", 0)
port = server.addr[1]
puts "server family: #{server.addr[0]}"

thread = Thread.new do
  client = server.accept
  while (line = client.gets)
    client.puts "echo: #{line.chomp}"
  end
  client.close
end

sock = TCPSocket.new("127.0.0.1", port)
["hello", "socket world"].each do |msg|
  sock.puts msg
  puts sock.gets
end
puts "peer: #{sock.peeraddr[0]} #{sock.remote_address.ip_address}"
sock.close_write
puts "eof after close_write: #{sock.read.inspect}"
sock.close
thread.join
server.close
puts "server closed: #{server.closed?}"

u = UDPSocket.new
u.bind("127.0.0.1", 0)
sent = u.send("ping", 0, "127.0.0.1", u.addr[1])
msg, from = u.recvfrom(16)
puts "udp sent #{sent} bytes, got #{msg.inspect} from #{from[0]} #{from[3]}"
u.close

ai = Addrinfo.tcp("127.0.0.1", 80)
puts ai.inspect
puts "#{ai.ip_address} #{ai.ip_port} ipv4=#{ai.ipv4?} ipv6=#{ai.ipv6?}"

begin
  TCPSocket.new("127.0.0.1", port)
rescue Errno::ECONNREFUSED => e
  puts "refused: #{e.class}"
end
