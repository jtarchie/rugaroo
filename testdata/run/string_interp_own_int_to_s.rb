# A program's own Integer#to_s: interpolation calls it, not the digit writer (decision 488). Standalone: it changes every Integer.
class Integer
  #: () -> String
  def to_s = "int"
end

i = 42
puts "a#{i}b#{-0 + 7}"
puts "#{i}"
puts "x#{"y#{i}"}"
