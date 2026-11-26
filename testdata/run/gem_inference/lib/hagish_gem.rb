# frozen_string_literal: true

# An attr writer only ever passed `{}` (rubocop's HashAlignment `self.offenses_by = {}`), keyed by class later: no
# record, so not Hash[Symbol, untyped] (decision 499).
module Hagish
  class KeyAlign
    def delta = 1
  end

  class TableAlign
    def delta = 2
  end

  class Cop
    attr_accessor :offenses_by, :column_deltas, :unalignable_styles

    def reset!
      self.offenses_by = {}
      self.column_deltas = Hash.new { |hash, key| hash[key] = {} }
      self.unalignable_styles = []
    end

    def check(node, alignment)
      offenses_by[alignment.class] ||= []
      if node == :skip
        unalignable_styles << alignment.class
        return
      end
      column_deltas[alignment.class][node] = alignment.delta
      offenses_by[alignment.class].push(node)
    end

    def report
      [offenses_by.keys.map(&:name), column_deltas.map { |k, v| [k.name, v.size] }, unalignable_styles.size]
    end
  end
end
