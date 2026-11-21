# frozen_string_literal: true

# A parser's environment as kramdown keeps it: options read from a Symbol-keyed Hash of mixed values (a record), a
# stack born as `stack: []` in that Hash that holds entries of two shapes, and the state saved to and restored from an
# Array.
class EnvEl
  def initialize(type)
    @type = type
    @children = []
  end
  attr_reader :type, :children
end

class EnvScan
  def initialize(str)
    @str = str
    @pos = 0
  end
  attr_accessor :pos

  def rest_size
    @str.size - @pos
  end
end

class EnvParser
  def initialize(source)
    @source = source
    @root = EnvEl.new(:root)
    @src = nil
    @tree = nil
    @block_ial = nil
    @text_type = :raw_text
    reset_env
  end

  def reset_env(opts = {})
    opts = { text_type: :raw_text, stack: [] }.merge(opts)
    @src = opts[:src]
    @tree = opts[:tree]
    @block_ial = opts[:block_ial]
    @stack = opts[:stack]
    @text_type = opts[:text_type]
  end

  def save_env
    [@src, @tree, @block_ial, @stack, @text_type]
  end

  def restore_env(env)
    @src, @tree, @block_ial, @stack, @text_type = *env
  end

  def parse_blocks(el, text)
    @stack.push([@tree, @src, @block_ial])
    @tree, @block_ial = el, nil
    @block_ial = { "id" => text } if text.size > 100
    @src = EnvScan.new(text)
    @tree.children << EnvEl.new(:p)
    n = @src.rest_size + @tree.children.size
    @tree, @src, @block_ial = *@stack.pop
    n
  end

  def parse_spans(el)
    @stack.push([@tree, @text_type]) unless @tree.nil?
    @tree, @text_type = el, :text
    n = @tree.children.size
    @tree, @text_type = @stack.pop
    n
  end

  def nested(text)
    env = save_env
    reset_env(src: EnvScan.new(text), text_type: :header_text)
    n = @src.rest_size
    restore_env(env)
    n
  end

  def run
    a = parse_blocks(@root, @source)
    @tree = @root
    b = parse_spans(EnvEl.new(:em))
    c = nested("abc")
    a + b + c + (@text_type == :raw_text ? 1 : 0)
  end
end
