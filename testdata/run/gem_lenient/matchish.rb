module Matchish
  class Nd
    attr_reader :type, :children

    def initialize(type, children = []) = (@type = type; @children = children)
    def send_type? = type == :send
    def splat_type? = type == :splat
    def hash_type? = type == :hash
    def kwsplat_type? = type == :kwsplat
  end

  class M
    NAMES = Set[:format, :sprintf].freeze

    def splatted?(node = self)
      return unless node.is_a?(Nd) && node.children.size  + 1 >= 4 &&
node.send_type? &&
true &&
NAMES === node.children[1] &&
(cur_child = node.children[cur_index = 2]; cur_child || true) && (matched2 = {}; true) &&
(node.children.size  - cur_index - 0).times do
  break node.children.size  - cur_index <= 0 && cur_index - 2 >= 1 unless case
when !matched2[0] && (cur_child.is_a?(Nd) && cur_child.splat_type? || cur_child.is_a?(Nd) && cur_child.children.size  + 1 >= 2 &&
cur_child.hash_type? &&
(cur_child3 = cur_child.children[cur_index3 = 0]; cur_child3 || true) && (matched4 = {}; true) &&
(cur_child.children.size  - cur_index3 - 0).times do
  break cur_child.children.size  - cur_index3 <= 0 && cur_index3 - 0 >= 1 unless case
when !matched4[0] && cur_child3.is_a?(Nd) && cur_child3.kwsplat_type? then matched4[0] = true
else true
end \
 && (cur_child3 = cur_child.children[cur_index3 +=1]; cur_child3 || true)
end \
 &&
matched4.size == 1 \
) then matched2[0] = true
else true
end \
 && (cur_child = node.children[cur_index +=1]; cur_child || true)
end \
 &&
matched2.size == 1
      block_given? ? yield() : (return true)
    end
  end

  def self.demo
    m = M.new
    s = ->(t, *c) { Nd.new(t, c) }
    h = { a: s.(:send, nil, :format, s.(:str, "x"), s.(:splat, s.(:lvar, :a))),
          b: s.(:send, nil, :format, s.(:str, "x"), s.(:hash, s.(:pair), s.(:kwsplat, s.(:lvar, :o)))),
          c: s.(:send, nil, :format, s.(:str, "x"), s.(:hash, s.(:pair))),
          d: s.(:send, nil, :puts, s.(:splat)), e: s.(:send, nil, :format, s.(:str, "x")), f: 5 } #: Hash[Symbol, untyped]
    h.values.map { |v| m.splatted?(v) }
  end
end
