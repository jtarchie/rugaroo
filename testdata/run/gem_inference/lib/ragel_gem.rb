# frozen_string_literal: true

# The shape of a Ragel-generated scanner (parser's lexer, regexp_parser's scanner): class-level Integer tables read
# through `send`, locals introduced by one multiple assignment of nil, state kept in ivars.
class RagelScanner
  class << self
    attr_accessor :_keys_table, :_targs_table, :_offsets_table
    private :_keys_table, :_keys_table=, :_targs_table, :_targs_table=, :_offsets_table, :_offsets_table=
  end
  self._keys_table = [0, 127, 48, 57, 65, 122, 0, 0]
  self._targs_table = [1, 2, 3, 1, 2, 3, 0, 1]
  self._offsets_table = [0, 2, 4, 6]

  def initialize
    @cs = 1
    @p = 0
    @source_pts = nil
    @count = 0
  end

  attr_reader :count

  def source=(source)
    @source_pts = source.unpack("U*")
  end

  def advance
    klass = self.class
    _keys_table = klass.send :_keys_table
    _targs_table = klass.send :_targs_table
    _offsets_table = klass.send :_offsets_table
    pe = @source_pts.size
    p, eof = @p, pe
    _slen, _trans, _keys, _inds = nil
    while p != eof
      _keys = @cs << 1
      _inds = _offsets_table[@cs]
      _wide = @source_pts[p] || 0
      _trans = if _keys_table[_keys] <= _wide && _wide <= _keys_table[_keys + 1]
                 _inds + 1
               else
                 _inds
               end
      @cs = _targs_table[_trans]
      case _targs_table[_trans]
      when 1 then @count += 1
      when 2 then @count += 2
      when 3 then @count += 3
      end
      p += 1
    end
    @p = p
    @count
  end
end
