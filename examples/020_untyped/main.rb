# rbs_inline: enabled

# Working with `untyped`: is_a? checks narrow a local, `||` and truthiness.

#: (untyped) -> String
def render(resource)
  return "none" unless resource

  if resource.is_a?(Array)
    "[" + resource.map { |r| render(r) }.join(",") + "]"
  elsif resource.is_a?(Integer)
    "int:#{resource + 1}"
  elsif resource.is_a?(String)
    "str:#{resource.upcase}"
  else
    "<#{resource}>"
  end
end

puts render(nil), render(false), render(5), render("s"), render([1, "x", [2]]), render(2.5)
value = nil #: untyped
puts value || "fallback", (value || 3).inspect
puts 5.is_a?(Integer), 5.is_a?(Comparable), "s".is_a?(Integer), 5.kind_of?(Object)
