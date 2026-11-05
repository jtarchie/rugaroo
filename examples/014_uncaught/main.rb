# rbs_inline: enabled

class Boom < StandardError; end

#: (Integer) -> Integer
def go(n)
  puts "step #{n}"
  raise Boom, "boom at #{n}" if n == 2

  go(n + 1)
end

begin
  go(0)
ensure
  puts "cleanup"
end
puts "unreachable"
