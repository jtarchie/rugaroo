# frozen_string_literal: true

# rubocop-ast's deferred prism tokens: a lambda returned untyped, tested with is_a?(Proc), stored and called later
# (decision 496). A Proc is a Go func, so the test narrows nothing.
module Lazyish
  def tokenize_deferred(n)
    ast = n * 2
    [ast, [n], deferred(n)]
  end

  private

  def deferred(n)
    lambda do
      build(n)
    end
  end

  def build(n) = Array.new(n) { |i| i }
end

class LazyishBase
  def tokenize(n) = [n, [n], [1]]
end

class LazyishLazy < LazyishBase
  include Lazyish
end

class LazyishSrc
  def initialize(n, parser)
    @n = n
    @ast, @comments, tokens = tokenize(parser)
    store(tokens)
  end

  def tokenize(parser)
    ast, comments, tokens = parse_and_lex(parser)
    [ast, comments, tokens]
  end

  def parse_and_lex(parser)
    if parser.respond_to?(:tokenize_deferred)
      parser.tokenize_deferred(@n)
    else
      parser.tokenize(@n)
    end
  end

  def store(tokens)
    if tokens.is_a?(Proc)
      @deferred = tokens
    else
      @toks = tokens
    end
  end

  def toks
    if @deferred
      @toks = @deferred.call
      @deferred = nil
    end
    @toks
  end

  attr_reader :ast
end
