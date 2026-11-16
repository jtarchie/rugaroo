# gem: gem_callbacks/lib
# load_path: gem_callbacks/lib
# Callbacks reached only by public_send with a variable name (rubocop's Commissioner and its cops): the gem's sig/
# declares them on the base class, which defines none, and each subclass's method takes that signature (decision 470).
# A node's children are a mixed literal in one place and an Array of Integers in another: a tuple held in a union
# loops as the Array it is (decision 444).
require "cb_gem"

cops = [CbGem::CountCop.new, CbGem::IntCop.new]
commissioner = CbGem::Commissioner.new(cops)
tree = CbGem.tree(8)
3.times { commissioner.walk(tree) }
puts cops.map(&:total).join(" ")
puts cops.map(&:name).join(" ")
