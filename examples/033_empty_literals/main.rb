# rbs_inline: enabled

# Blocks on untyped receivers are unsupported, so these only compile once each `[]`/`{}` is typed.

words = %w[pear fig apple fig]

shout = []
words.each { |w| shout << w.upcase }
puts shout.sort.inspect

# Keys type only after `shout` does: `w` is untyped until then.
lengths = {}
shout.each { |w| lengths[w] = w.size }
puts lengths.inspect
puts lengths.map { |w, n| "#{w}=#{n}" }.join(",")

ranked = []
words.each_with_index { |w, i| ranked << [w, i] }
puts ranked.sort_by { |w, i| [-i, w] }.map { |w, _| w }.inspect

found = []
words.each { |w| found.push(w.index("p")) }
puts found.compact.inspect

# A gap before the index reads nil, so the elements are Integer?.
slots = []
slots[2] = 7
puts slots.inspect

#: (Array[String]) -> Hash[String, Integer]
def tally_by_first(ws)
  out = {}
  ws.each { |w| out[w[0] || ""] = (out[w[0] || ""] || 0) + 1 }
  out
end
puts tally_by_first(words).inspect

bag = []
bag << 1
bag << "two"
puts bag.inspect

#: (Array[untyped]) -> void
def add_marker(xs)
  xs << "marker"
end

# Typed, it would be copied into Array[untyped] and lose the mutation: stays untyped.
shared = []
shared << 1
add_marker(shared)
puts shared.inspect
