# prelude/symbol.rb
# rbs_inline: enabled

#
# Symbol as a named Go string, distinct from String.

# @go_type string
class Symbol < Object
  include Comparable

  #: (Symbol) -> Integer
  def <=>(other) = %x{ Integer(strings.Compare(string(self), string(other))) }

  # == on two Symbols (decision 12's class overload): a plain Go compare,
  # inlined, with no boxing of the argument into untyped.
  #: (Symbol) -> bool
  def __eq_symbol(other) = %x{ Boolean(self == other) }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, ok := other.(Symbol)
    return Boolean(ok && self == o)
  }

  # A new String each call; name is the one frozen String, as in MRI.
  #: () -> String
  def to_s = %x{ rbStrClone(String(self)) }

  #: () -> String
  def name = %x{ String(self) }

  #: () -> Symbol
  def to_sym = self

  # Immediates are always frozen.
  #: () -> bool
  def frozen? = true

  #: () -> String
  def inspect = %x{ rbSymbolInspect(string(self)) }

  #: () -> Integer
  def size = name.size

  #: () -> Integer
  def hash = name.hash

  #: () -> Integer
  def length = name.size

  #: () -> bool
  def empty? = name.empty?

  #: () -> Symbol
  def upcase = name.upcase.to_sym

  #: () -> Symbol
  def downcase = name.downcase.to_sym

  #: () -> Symbol
  def capitalize = name.capitalize.to_sym

  #: () -> Symbol
  def swapcase = name.swapcase.to_sym

  #: () -> Symbol
  def succ = name.succ.to_sym

  #: () -> Symbol
  def next = succ

  #: (Symbol) -> Integer
  def casecmp(other) = name.casecmp(other.name)

  #: (Symbol) -> bool
  def casecmp?(other) = name.casecmp?(other.name)

  # The same Symbol (parser's `pair.type.eql?(:pair)` on an untyped node type; decision 444).
  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := rbUnbox(other).(Symbol)
    return Boolean(ok && o == self)
  }

  #: (String) -> bool
  def start_with?(prefix) = name.start_with?(prefix)

  #: (String) -> bool
  def end_with?(suffix) = name.end_with?(suffix)

  #: (String, String) -> bool
  def __start_with_q_2(a, b) = name.start_with?(a) || name.start_with?(b)

  #: (String, String, String) -> bool
  def __start_with_q_3(a, b, c) = name.start_with?(a) || name.start_with?(b) || name.start_with?(c)

  #: (String, String) -> bool
  def __end_with_q_2(a, b) = name.end_with?(a) || name.end_with?(b)

  #: () -> String
  def id2name = to_s

  #: (Integer) -> String?
  def [](i) = name[i]

  #: (Integer, Integer) -> String?
  def __idx_2(start, count) = name[start, count]

  #: (Range[Integer]) -> String?
  def __idx_range(r) = name[r]
end

class String
  #: () -> Symbol
  def to_sym = %x{ Symbol(self) }
end
