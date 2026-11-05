# rbs_inline: enabled

require "ipaddr"

net = IPAddr.new("192.168.1.0/24")
puts net
puts net.inspect
puts net.include?("192.168.1.42")
puts net.include?("192.168.2.1")
puts net.include?(IPAddr.new("192.168.1.128/25"))
puts net.include?(IPAddr.new("192.168.0.0/16"))
puts net.mask(16)
puts net.mask("255.255.0.0")

range = net.to_range
puts range.first
puts range.last
puts range.first.succ

other = IPAddr.new("192.168.1.5/24")
puts(net == other)
puts(net <=> other)
puts net.eql?(other)
puts net.ipv4?
puts net.ipv6?

v6 = IPAddr.new("2001:db8::/32")
puts v6
puts v6.inspect
puts v6.ipv4?
puts v6.ipv6?
puts v6.include?("2001:db8:1::1")
puts v6.include?("2001:db9::1")
puts v6.succ

mapped = IPAddr.new("::ffff:192.168.1.1")
puts mapped
puts mapped.ipv4?
puts mapped.ipv6?
puts(mapped <=> IPAddr.new("192.168.1.1"))

last = IPAddr.new("255.255.255.255")
begin
  last.succ
rescue IPAddr::InvalidAddressError => e
  puts e.message
end

begin
  IPAddr.new("999.1.1.1")
rescue IPAddr::InvalidAddressError => e
  puts e.message
end

begin
  IPAddr.new("1.2.3.4/33")
rescue IPAddr::InvalidPrefixError => e
  puts e.message
end
