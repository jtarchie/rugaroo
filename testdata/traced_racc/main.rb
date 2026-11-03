# A racc-generated parser (lib/calc.y, `racc -l -o lib/calc.rb lib/calc.y`) under -trace: MRI loads racc/cparse, the
# program has none, so boot must record racc's Ruby `do_parse`.
# load_path: lib
require "calc"

puts Calc.new.parse([[:NUM, 2], ["+", "+"], [:NUM, 3], ["*", "*"], [:NUM, 4], [false, "$"]])
