# rbs_inline: enabled

# Ruby 4.0: a continuation line may start with `&&` / `||`.

#: (Integer) -> bool
def teen?(n)
  n >= 13
    && n <= 19
end

#: (Integer) -> String
def describe(n)
  small = n < 10
    || n == 10
  kind = small ? "small" : "big"
  "#{n} is #{kind}"
end

puts teen?(15), teen?(20)
puts describe(3), describe(10), describe(42)
words = ["it", "works"]
puts words.map { it.upcase }.join(" ")
