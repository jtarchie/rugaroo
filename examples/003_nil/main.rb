# rbs_inline: enabled

#: (Hash[String, String], String) -> String
def greeting(h, k)
  h[k]&.upcase || "DEFAULT"
end

h = { "a" => "hi" }
puts greeting(h, "a")
puts greeting(h, "b")

name = h["a"]
puts name.size if name
puts name.inspect, h["zz"].inspect
