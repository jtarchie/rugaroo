require "prism"
require "parser"
require "rubocop-ast"
src = "x = 1 + 2\nputs x\n"
ps = RuboCop::AST::ProcessedSource.new(src, 3.4, parser_engine: :parser_prism)
puts ps.tokens.size
