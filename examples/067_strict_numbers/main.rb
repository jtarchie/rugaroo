# rbs_inline: enabled

# Kernel#Integer and Kernel#Float: strict conversions that reject what
# String#to_i and #to_f would quietly read as 0.

CONFIG = <<~TEXT
  port = 8080
  workers = 0x10
  mode = 0o755
  ratio = 0.75
  timeout = 1_500
  retries = three
  scale = 2.5x
TEXT

#: (String, String) -> String
def parse_value(key, raw)
  key == "ratio" || key == "scale" ? Float(raw).inspect : Integer(raw).inspect
end

CONFIG.each_line do |line|
  key, raw = line.split("=").map(&:strip)
  next unless key && raw

  begin
    value = parse_value(key, raw)
    puts "#{key}: #{value}"
  rescue ArgumentError => e
    puts "#{key}: rejected (#{e.message}); to_i would give #{raw.to_i}"
  end
end

puts Integer("ff", 16)
puts Integer(9.99)
puts Float(3)
begin
  Integer(nil)
rescue TypeError => e
  puts e.message
end
