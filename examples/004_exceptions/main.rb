# rbs_inline: enabled

class AppError < StandardError; end
class NotFound < AppError; end

#: (Hash[String, Integer], String) -> Integer
def lookup(h, k)
  h.fetch(k)
rescue KeyError
  raise NotFound, "missing #{k}"
end

begin
  lookup({ "a" => 1 }, "b")
rescue AppError => e
  puts "handled: #{e.message}"
ensure
  puts "done"
end
