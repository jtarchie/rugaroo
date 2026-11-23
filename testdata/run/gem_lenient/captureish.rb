# frozen_string_literal: true

# rubocop-ast's generated node matchers: `yield(*captures)` into a `|*args|` block gets every capture (decision 497;
# Style/ComparableBetween's `(min_and_value & max_and_value).first`).
class Captureish
  def matcher(node)
    captures = Array.new(4)
    return unless node.is_a?(Array) &&
                  (captures[0] = node[0]; true) && (captures[1] = node[1]; true) &&
                  (captures[2] = node[2]; true) && (captures[3] = node[3]; true)
    block_given? ? yield(*captures) : (return captures)
  end

  def on_and(node)
    out = nil
    matcher(node) do |*args|
      min_and_value, max_and_value = args.each_slice(2).to_a
      out = [args, (min_and_value & max_and_value).first]
    end
    out
  end
end
