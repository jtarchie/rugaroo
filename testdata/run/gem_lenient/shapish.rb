module Shapish
  class Nd
    attr_reader :src, :pos

    def initialize(src, pos)
      @src = src
      @pos = pos
    end

    def source_range = self
    def begin_pos = @pos

    def children = [Nd.new("a", 1), Nd.new("b", 2)]

    def join(other) = Nd.new(src + other.src, pos)
  end

  class Rng
    attr_reader :begin_pos

    def initialize(pos) = @begin_pos = pos
  end

  # 1: nested destructuring over an untyped each_slice/each_cons chain
  class Safe
    def clauses(parts)
      parts.sort_by { |a| a.is_a?(Nd) ? a.source_range.begin_pos : a.begin_pos }
           .each_slice(2)
           .each_cons(2)
    end

    def on_and(parts)
      out = []
      clauses(parts).each do |(lhs, lhs_op), (rhs, _rhs_op)|
        out << "#{lhs.source_range.begin_pos}-#{lhs_op.begin_pos}-#{rhs.source_range.begin_pos}"
      end
      out
    end
  end

  # 2: Integer or Symbol in one variable
  class Fmt
    def offense?(kind)
      passed, expected = count(kind)
      return false if passed == :unknown

      matched?(expected, passed)
    end

    def count(kind)
      if kind == 1
        [3 - 1, 2]
      elsif kind == 2
        [-2, 3]
      else
        [:unknown] * 2
      end
    end

    def matched?(expected, passed)
      if passed.negative?
        expected < passed.abs
      else
        expected != passed
      end
    end
  end

  # 4: sort_by!(&order) where order is a Symbol or a lambda
  class Finder
    def initialize(fast) = @fast = fast

    def targets(files)
      list = files.select { |f| f.size > 1 }
      list.sort_by!(&order)
    end

    def targets_any(store)
      list = store[:files]
      list.sort_by!(&order)
    end

    def order
      if @fast
        ->(path) { -path.size }
      else
        :itself
      end
    end
  end

  # 5: reduce(&:join) on mapped untyped children, forwarded block
  class Al
    def add_offense(range, message:)
      r = "#{range.src}:#{message}"
      r += yield(range) if block_given?
      r
    end

    def for_args(node, &block)
      existing = node.children.map(&:src).join(' ')
      ranges = node.children.map(&:source_range)
      msg = format('use %<prefer>s not %<current>s', prefer: existing.upcase, current: existing)
      add_offense(ranges.reduce(&:join), message: msg, &block)
    end
  end
end
