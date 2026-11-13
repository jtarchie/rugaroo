# matrix's Matrix.build: a callee-side default, a to_enum guard after argument checks, a block whose value is used (decision 460).
class Buildish
  def self.build(rows, cols = rows)
    raise ArgumentError if rows < 0
    return to_enum :build, rows, cols unless block_given?

    new(Array.new(rows) { |i| Array.new(cols) { |j| yield i, j } })
  end

  def initialize(rows) = @rows = rows

  def lower? = @rows.each_with_index.all? { |r, i| r.each_with_index.all? { |v, j| j <= i || v.zero? } }

  def to_s = @rows.inspect
end
