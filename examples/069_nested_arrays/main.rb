# rbs_inline: enabled

# Array#flatten, #to_h and #transpose: methods that only exist when the
# elements are themselves Arrays or [key, value] pairs.

names = %w[ann bob cy]
marks = [[90, 85, 77], [60, 72, 88], [95, 91, 99]]

# Rows become columns: one Array per exam.
by_exam = marks.transpose
by_exam.each_with_index do |col, i|
  puts "exam #{i + 1}: best #{col.max}, mean #{col.sum / col.size}"
end

# Every mark in one list.
all = marks.flatten
puts "all marks: #{all.sort.inspect}"
puts "overall mean: #{all.sum / all.size}"

# Pairs become a Hash.
totals = names.zip(marks.map(&:sum)).to_h
p totals
best = totals.max_by { |_, v| v }
puts "top: #{best.inspect}" if best

# to_h with a block builds the pairs itself.
initials = names.to_h { |n| [n.chr.upcase, n.length] }
p initials

# Deeper nesting flattens all the way, or one level at a time.
groups = [[[1, 2], [3]], [[4, 5]]]
p groups.flatten
p groups.flatten(1)

begin
  [[1, 2], [3]].transpose
rescue IndexError => e
  puts "transpose: #{e.message}"
end
