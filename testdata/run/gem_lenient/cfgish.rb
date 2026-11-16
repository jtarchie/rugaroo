# frozen_string_literal: true

require "forwardable"

# rubocop's Config: Hash methods forwarded to an untyped ivar, called with and without a block (decision 444).
module Cfgish
  class Config
    extend Forwardable

    def initialize(hash)
      @hash = hash
    end

    def_delegators :@hash, :[], :each_key, :keys, :key?, :to_hash

    def under(other) = @hash.merge(other)
  end

  def self.disabled(config)
    config.each_key.with_object([]) do |key, out|
      out << key unless config[key]
    end
  end

  def self.demo
    config = Config.new({ "Lint" => true, "Style" => false, "Layout" => nil })
    seen = []
    config.each_key { |k| seen << k.downcase }
    other = Config.new({ a: 1, b: 2 }) # a second kind of Hash: @hash is untyped, as rubocop's is
    [disabled(config), seen, config.each_key.to_a, config.key?("Lint"), other.each_key.map(&:to_s), other.keys,
     other.under(Config.new({ b: 20, c: 30 }))]
  end
end
