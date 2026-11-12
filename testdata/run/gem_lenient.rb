# gem: gem_lenient
# load_path: gem_lenient
# Gem code compiled leniently (decision 410): constants a constant's value assigns, a container ivar reset to nil,
# and a nil in a multiple assignment; matrix's shapes (decision 414); attr_accessor(*CONST) (decision 427).
require "foo"
require "ng"
require "mx"
require "ftpish"
require "primeish"
require "ctx"
require "seqish"
require "blkconst"
require "autoish_user"
require "autoish/lazy_width"
require "autoish/lazy_name"
require "outish"
require "unitish"
require "metaish"
require "rexish"
require "registryish_sub"
require "registryish_sub2"
require "arityish"
require "strapish"
require "prepish"
require "shapish"
require "redish"
require "nodish"
require "srcish"
require "slotish"
require "rngish"
require "dynish"
require "fwdish"
require "findish"
require "bufish"
require "cmpish"
require "sexpish"
require "capish"
p Foo::ORDERINGS
p Foo::FLAGS
p Foo::RETURN_IN_ORDER
p Foo.ordered?
g = Ng.new
p g.multi
p g.names, g.args
g.reset
p g.names, g.args
p g.multi
p g.sieve(10)
p g.upto_inf(3)
x = Mx.new([[1, 2], [3, 4]])
p x.solve(5..6)
p x.fill
p x.each_with_index.to_a
p x.columns([0, 1])
p Mx.build(2) { |i, j| i * 10 + j }
p x.map.to_a, x.collect(:diagonal).to_a
p x.find_index(4), x.index { |e| e > 2 }, x.find_index(:all, 9)
p x.strict_upper_even?
p Mx.diagonal(1, 2)
p Mx.ramp(4)
p x.twin_solve
p x.dyn_col
f = Ftpish.new
p f.parse("t", "2024.5"), f.parse("t", "2024"), f.parse("z", "q")
p Ftpish::KIND, f.connect, f.get, f.get("x"), f.drain("ab")
w = Ftpish::Wire.new
p f.push(w, "STAT"), w.sent, f.by_name("abc", "upcase")
p f.soft_require, f.forward(1, 2), f.pump, Ftpish::Proto.open("u", {}, false)
pr = Primeish.new
p pr.witness?(3, 2, 16), pr.witness?(3, 2, 5), pr.witness?(0, 2, 16)
p pr.scan([1, 2, 3], 2), pr.scan([1, 2, 3], 9)
gen = PrimeishFront.new.each
p gen.class, gen.next, gen.next
gen.rewind
p gen.next
seen = []
PrimeishFront.new.each { |v| seen << v }
p seen
g2 = PrimeishGen.new
e2 = g2.each
p e2.class, e2.next, g2.next
p PrimeishFront.new.each(PrimeishOddGen.new(9)).to_a
pw = PrimeishFront.new.each(PrimeishGen.new)
p pw.next, pw.next
pw.rewind
p pw.next
ua = PrimeishFront.new.each_any(PrimeishGen.new)
p ua.next, ua.next
ua.rewind
p ua.next
ub = []
PrimeishFront.new.each_any(PrimeishGen.new) { |v| ub << v }
p ub, PrimeishFront.new.each_any([4, 5]).next
p ua.method(:succ) == ua.method(:next), ua.method(:succ) == ua.method(:rewind), g2.method(:succ) == g2.method(:next)
p PrimeishBound.new.each(3) { |v| v * 10 }, PrimeishBound.new.each(0) { :value }
p PrimeishFront.new.each_any(PrimeishLast.new(3)) { |v| v * 10 }, PrimeishFront.new.each_any(PrimeishLast.new(0)) { :value }
p MxEigen.new(2, false).scale(0.25), MxEigen.new(1, true).scale(0.5)
c = Ctx.new
c.in_def = true
p c.in_def, c.in_class
sq = SubSeqish.new([3, 1, 2])
p sq.map { |e| e * 2 }, sq.sort, sq.to_h, sq.first_two, sq.each_with_index.to_a, sq.select(&:odd?)
p BlkConstWidth::INDEX > 0, BlkConstWidth::FIRST == BlkConstWidth::INDEX + 1, BlkConstWidth.idx == BlkConstWidth::INDEX
p AutoishUser.show
p Outish.new.build(%w[a bc]), Outish.new.build(%w[a bc]).bytesize
p Unitish::NAMES.frozen?, Unitish::COUNT, Unitish::ARG.size
p Metaish::TYPE_COUNT, Metaish::TYPE_SET.include?(:sym)
p Rexish.new(":abc (12) word").tokens
rc = RegistryishSub.new
p rc.compile([:int, 1]), rc.compile([:str, "x"]), rc.compile([:sym, :y]), RegistryishSub.registry.keys.sort, Registryish.registry, RegistryishSub2.registry.keys, RegistryishSub2.new.compile([:float, 15])
p [Arityish.new, Arityish::Rest.new, Arityish::Pair.new].map { |n| n.arity_range.minmax }
st = Strapish.new
buf = +"s"
st.into(buf, "x")
p buf, st.build(%w[a b])
# an append through an element block reaches the element, as in MRI, or raises: never a silent copy
rows = [+"a", +"b"]
p(begin
  st.newlines(rows) == ["a\n", "b\n"] && rows == ["a\n", "b\n"]
rescue NotImplementedError
  true
end)
mixed = [["x"], +"b"] #: Array[untyped]
p(begin
  st.tails(mixed) == [["x", "!"], "b!"] && mixed[1] == "b!"
rescue NotImplementedError
  true
end)
p st.tails([["y"]])
# a module only ever prepended, with its own include (decision 444): its methods and its module's are the class's
p Prepish::Calc.calculate([1, 2])
p Prepish::Calc.calculate([1, 2], discount: true)
p Prepish::Calc.ancestors.first(3)
# nested destructuring over pairs of pairs; `&order`, a Symbol or a lambda when it runs; reduce(&:join) with a forwarded block (decision 444)
sh_parts = [Shapish::Rng.new(5), Shapish::Nd.new("x", 1), Shapish::Nd.new("y", 9), Shapish::Rng.new(12), Shapish::Nd.new("z", 14), Shapish::Rng.new(20)]
p Shapish::Safe.new.on_and(sh_parts)
sh_f = Shapish::Fmt.new
p sh_f.offense?(1), sh_f.offense?(2), sh_f.offense?(3)
p Shapish::Finder.new(true).targets(%w[bbb a cc dddd]), Shapish::Finder.new(false).targets(%w[bbb a cc])
sh_a = Shapish::Al.new
p sh_a.for_args(Shapish::Nd.new("n", 0)), sh_a.for_args(Shapish::Nd.new("n", 0)) { |r| "!#{r.pos}" }
sh_st = { files: %w[bbb a cc dddd], n: 1 } #: Hash[Symbol, untyped]
p Shapish::Finder.new(true).targets_any(sh_st), Shapish::Finder.new(false).targets_any(sh_st)
# reduce(&:join) over untyped elements: the Hash arm, where a String is no pair, raises instead of failing in Go
rd_a = Redish::Al.new
rd_nodes = [Redish::Nd.new([Redish::Leaf.new("a"), Redish::Nd.new([])]), Redish::Nd.new([Redish::Leaf.new("q"), Redish::Leaf.new("r")])]
rd_nodes.each { |n| p rd_a.for_args(n), rd_a.for_args(n) { |r| "!#{r.src}" } }
# rubocop-ast's shapes (decision 444): a module's override of a method of the module it includes, match(re, pos) with
# \G, \R, an Enumerator's to_enum, yield(*node), flat_map of a value or an Array, a Set splatted into arguments
Nodish.demo.each { |v| p v }
# a Range through `lines[*args]`, Set.new of an untyped list, to_enum(:scan, re).map { Regexp.last_match }, Module#<=>
Srcish.demo.each { |v| p v }
# a class's own method of another arity under one of a module its module includes (decision 444)
p Slotish::Line.new.run("abc"), Slotish::Other.new.run
# a Range subclass whose initialize calls super(first, last), with infinite bounds (decision 444)
Rngish.demo.each { |v| p v }
# untyped receivers (decision 444): Set.new of an untyped list, a sentinel keyword default with its own ==, an each
# shared by classes through one dispatcher arm, blockless none?/any? beside block forms, find with next, and an each
# whose block edits its String elements in place
Dynish.demo.each { |v| p v }
Fwdish.demo.each { |v| p v }
Findish.demo.each { |v| p v }
Bufish.demo.each { |v| p v }
# enum_for(__method__, named: named), a Range through an untyped `[]`, Array#<=> answering nil for classes that do not
# compare, a break in a block whose call is assigned (decision 444)
Cmpish.demo.each { |v| p v }
# a to_s (an alias) and an inspect that take an optional argument are the object's string forms (decision 444)
Sexpish.demo.each { |v| p v }
# enum_for(__method__, named: named) in a mixin's method whose keyword default is a sentinel constant (decision 444)
Capish.demo.each { |v| p v }
