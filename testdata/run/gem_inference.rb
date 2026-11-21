# frozen_string_literal: true

# gem: gem_inference/lib
# load_path: gem_inference/lib
# Gem code whose types only settle across inference rounds (docs/research/inference-2026-11.md, gate 1): a scan
# position that flows through helper methods and through lambdas held in ivars (cyc_gem), a parameter reassigned from
# a method that takes it (buf_gem), a Ragel-shaped scanner's class-level tables read through `send` (ragel_gem), its
# state constants read as `self.class.lex_en_x` by two classes (lexen_gem), and arguments that reach a constructor
# through a `*args` forwarder (fwd_gem).
require "cyc_gem"
require "buf_gem"
require "ragel_gem"
require "lexen_gem"
require "fwd_gem"

puts CycLexer.new.advance(30_000)

buffer = BufGem.new("x.rb", source: "puts 1\r\nputs 2\n")
puts buffer.source.size

text = "hello world 12345 Ruby to Go\n" * 200
total = 0
5.times do
  scanner = RagelScanner.new
  scanner.source = text
  total += scanner.advance
end
puts total

puts LexEn.new.step(3)

stack = FwdStack.new
stack.push("<<", 3)
stack.push("<<~", 5, 40, true)
puts stack.jump(7)
puts stack.top.kind
