# rbs_inline: enabled

# Built at the call site by the compiler (decision 141); typed call/to_proc/curry/bind are inline, these are the untyped forms.

# @rbs generic F
# @go_type struct { fn F; dyn func(any, ...any) any; recv any; info *rbMethodInfo }
class Method < Object
  # A block goes on to the method, as `m.call(x) { }` yields to it (parser's `@enforcer.call(event) do ... end`).
  #: (*untyped) ?{ (*untyped) -> untyped } -> untyped
  def call(*args) = %x{
    if blk != nil {
      rest_ = append(slices.Clip(rest_), rbBlockArg{fn: blk})
    }
    return rbMethodCallDyn(self.dyn, self.recv, self.info, rest_)
  }

  #: (*untyped) -> untyped
  def [](*args) = %x{ return rbMethodCallDyn(self.dyn, self.recv, self.info, rest_) }

  #: (untyped) -> untyped
  def ===(arg) = %x{ return rbMethodCallDyn(self.dyn, self.recv, self.info, []any{arg}) }

  #: () -> Integer
  def arity = %x{ return Integer(rbMethodKnown(self.info, "arity").arity) }

  #: () -> Array[Array[Symbol]]
  def parameters = %x{ return rbMethodParams(rbMethodKnown(self.info, "parameters")) }

  #: () -> Symbol
  def name = %x{ return Symbol(self.info.name) }

  #: () -> Module
  def owner = %x{ return rbMethodKnown(self.info, "owner").owner }

  #: () -> untyped
  def receiver = %x{ return self.recv }

  #: () -> UnboundMethod[untyped]
  def unbind = %x{ return rbMethodUnbind(self) }

  #: () -> String
  def inspect = %x{ return rbMethodInspect("Method", self.recv, self.info) }

  #: () -> String
  def to_s = inspect

  # Same receiver (identity) and the same method definition, as MRI's.
  #: (untyped) -> bool
  def ==(other) = %x{ return Boolean(rbMethodEq(true, self.recv, self.info, other)) }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ return rbMethodHash(self.recv, self.info) }

  #: () -> Method[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*Method[any]); ok {
      return same
    }
    return &Method[any]{fn: any(self.fn), dyn: self.dyn, recv: self.recv, info: self.info}
  }
end

# @rbs generic F
# @go_type struct { fn F; dyn func(any, ...any) any; info *rbMethodInfo }
class UnboundMethod < Object
  #: (untyped) -> Method[untyped]
  def bind(obj) = %x{ return rbUnboundBind(self.dyn, self.info, obj) }

  #: (untyped, *untyped) -> untyped
  def bind_call(obj, *args) = %x{
    m := rbUnboundBind(self.dyn, self.info, obj)
    return rbMethodCallDyn(m.dyn, m.recv, m.info, rest_)
  }

  #: () -> Integer
  def arity = %x{ return Integer(rbMethodKnown(self.info, "arity").arity) }

  #: () -> Array[Array[Symbol]]
  def parameters = %x{ return rbMethodParams(rbMethodKnown(self.info, "parameters")) }

  #: () -> Symbol
  def name = %x{ return Symbol(self.info.name) }

  #: () -> Module
  def owner = %x{ return rbMethodKnown(self.info, "owner").owner }

  #: () -> String
  def inspect = %x{ return rbMethodInspect("UnboundMethod", nil, self.info) }

  #: () -> String
  def to_s = inspect

  #: (untyped) -> bool
  def ==(other) = %x{ return Boolean(rbMethodEq(false, nil, self.info, other)) }

  #: (untyped) -> bool
  def eql?(other) = self == other

  #: () -> Integer
  def hash = %x{ return rbMethodHash(nil, self.info) }

  #: () -> UnboundMethod[untyped]
  def _to_any = %x{
    if same, ok := any(self).(*UnboundMethod[any]); ok {
      return same
    }
    return &UnboundMethod[any]{fn: any(self.fn), dyn: self.dyn, info: self.info}
  }
end
