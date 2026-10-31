# prelude/object.rb
# rbs_inline: enabled

#
# The root of the hierarchy: BasicObject, Kernel, Object, Comparable.

class BasicObject
  #: (untyped) -> bool
  def equal?(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def ==(other) = %x{ Boolean(rbIdentical(self, other)) }

  #: (untyped) -> bool
  def !=(other) = %x{ !rbEq[any](self, other) }

  #: () -> bool
  def ! = false
end

module Kernel
  # @rbs [X] () { (self) -> X } -> X
  def then = yield(self)

  # @rbs [X] () { (self) -> X } -> X
  def yield_self = yield(self)

  #: () -> self
  def itself = self

  #: () -> nil
  def display = %x{ rbWriteOut(string(rbToS(self))) }

  #: () -> Array[Symbol]
  def instance_variables = %x{ return rbIvarNames(self) }

  #: (untyped) -> untyped
  def instance_variable_get(name) = %x{ return rbIvarGet(self, name) }

  #: (untyped, untyped) -> untyped
  def instance_variable_set(name, value) = %x{ return rbIvarSet(self, name, value) }

  #: (untyped) -> untyped
  def remove_instance_variable(name) = %x{ return rbIvarRemove(self, name) }

  #: (untyped) -> bool
  def instance_variable_defined?(name) = %x{ Boolean(rbIvarDefined(self, name)) }

  #: () { () -> void } -> void
  def loop
    while true
      yield
    end
  end

  # @rbs [X] () { (self) -> X } -> self
  def tap
    yield(self)
    self
  end

  # A shallow copy (Object, struct classes; Array, String, Hash and Set have their own); immediates are themselves.
  #: () -> self
  def dup = %x{ return rbDup(self) }

  # dup keeping the frozen flag (freeze: false drops it). ponytail: no singleton methods to copy (none are made at run time).
  #: (?freeze: bool?) -> self
  def clone(freeze: nil)
    c = dup
    c.freeze if freeze || (freeze.nil? && frozen?)
    c
  end

  #: () -> String
  def to_s = %x{ rbObjToS(self) }

  # MRI's default inspect lists the ivars and never calls to_s.
  #: () -> String
  def inspect = %x{ rbObjInspect(self) }

  #: () -> bool
  def nil? = false

  # Heap objects by address, other values by hash: unique while alive, but not MRI's numbers.
  #: () -> Integer
  def object_id = %x{ return rbObjectID(self) }

  # Exact class, unlike is_a?: the generated class IDs are equal (decision 82).
  #: (Module) -> bool
  def instance_of?(klass) = %x{
    d, ok := any(klass).(interface{ _DescID() int })
    return Boolean(ok && rbRubyClassID(rbClassID(self)) == d._DescID())
  }

  # Objects, Array, Hash and Struct values are mutable; frozen classes override. nil reaches here held untyped (NilClass has no Go type).
  # A struct object's flag is on its root struct (decision 221).
  #: () -> bool
  def frozen? = %x{
    if any(self) == nil {
      return true
    }
    if p := rbObjFrozenFlag(any(self)); p != nil {
      return Boolean(*p)
    }
    return Boolean(rbIsFrozen(any(self)))
  }

  # ponytail: an ivar write does not check the flag, so a frozen object's setters still run (MRI raises FrozenError);
  # check it in the struct's ivar assignment once a program needs the error.
  #: () -> self
  def freeze = %x{
    if p := rbObjFrozenFlag(any(self)); p != nil {
      *p = true
    } else if f, ok := any(self).(rbFreezable); ok {
      f.rbSetFrozen()
    }
    return self
  }

  # Seconds slept, rounded, as MRI returns them. The argument's class picks the overload (decision 12).
  #: (Float) -> Integer
  def sleep(secs) = %x{ return rbSleep(float64(secs)) }

  #: (Integer) -> Integer
  def __sleep_integer(secs) = %x{ return rbSleep(float64(secs)) }

  # No argument: forever, as MRI (nothing wakes it; Go reports a deadlock if no other goroutine runs).
  #: () -> Integer
  def __sleep_0 = %x{ rbSleepForever() }

  # A labelled break that carries a value (decision 91): throw unwinds to
  # the innermost catch with an identical tag, running ensures, passing
  # rescues. The thrown value's type is only known at the throw, so the
  # result is untyped.
  #: (?untyped) { (untyped) -> untyped } -> untyped
  def catch(tag = __fresh_object) = %x{ return rbCatch(tag, blk) }

  #: () -> untyped
  def __fresh_object = %x{ return &Object{} }

  #: (untyped, ?untyped) -> bot
  def throw(tag, value = nil) = %x{ rbThrowTag(tag, value) }

  # A Go program cannot fork (decision 233): reaching fork raises, where MRI forks. Homebrew's SystemCommand forks
  # only when Process.spawn failed. MRI's parent gets the child's pid.
  #: () { () -> untyped } -> Integer?
  def fork = raise(NotImplementedError, "fork() function is unimplemented on this machine")

  private

  #: (?Integer) -> void
  def exit(status = 0) = raise(SystemExit.new(status))

  # Kernel#exit! skips at_exit handlers and doesn't flush stdout.
  #: (?Integer) -> void
  def __exit_bang(status = 0) = %x{ os.Exit(int(status)) }

  # For at_exit handlers, in place of `$!`: true unless an uncaught exception or a failing exit is pending.
  #: () -> bool
  def __exit_status_ok? = %x{ return Boolean(rbExitStatusNow.Load() == 0) }

  # Handlers run LIFO after main, exit or an uncaught exception (rbTopRecover).
  #: () { () -> void } -> void
  def at_exit = %x{ rbAtExitPush(blk) }

  #: (*untyped) -> nil
  def puts(*args)
    return __write("\n") if args.empty?

    args.each do |a|
      case a
      when nil then __write("\n")
      when Array then a.each { |e| puts(e) }
      else
        s = a.to_s
        __write(s.end_with?("\n") ? s : s + "\n")
      end
    end
    nil
  end

  # MRI returns its argument(s); rugaroo's p is a statement-only nil.
  #: (*untyped) -> nil
  def p(*args)
    args.each { |a| __write(a.inspect + "\n") }
    nil
  end

  # `p bin`: a binary String's inspect (decision 352).
  #: (String?) -> nil
  def __bin_p(s)
    __write((s ? s.__bin_inspect : "nil") + "\n")
    nil
  end

  #: (String, *untyped) -> String
  def format(fmt, *args) = %x{ String(rbFormat(string(fmt), rest_)) }

  #: (String, *untyped) -> String
  def sprintf(fmt, *args) = %x{ String(rbFormat(string(fmt), rest_)) }

  # printf(io, fmt, *args) writes to io, as MRI's does when the first
  # argument is not a String.
  # @dynamic
  #: (untyped, *untyped) -> nil
  def printf(fmt, *args)
    if fmt.is_a?(String)
      __write(format(fmt, *args))
    else
      fmt.write(format(args.shift.to_s, *args))
    end
    nil
  end

  #: (*untyped) -> nil
  def print(*args)
    ofs = $, # joins the arguments, and $\ follows them, when the program set them (decision 414)
    first = true
    args.each do |a|
      __write(ofs) if ofs && !first
      first = false
      __write(a.to_s)
    end
    ors = $\
    __write(ors) if ors
    nil
  end

  #: (String) -> nil
  def __write(s) = %x{ rbWriteOut(string(s)) }

  # One byte of an Integer (its low 8 bits), or a String's first character.
  #: (Integer) -> Integer
  def putc(c)
    __write((c & 0xff).chr)
    c
  end

  #: (String) -> String
  def __putc_string(s)
    __write(s[0] || "")
    s
  end

  #: () -> String
  def __class_name = %x{ String(rbClassName(self)) }
end

class Object < BasicObject
  include Kernel
end

module Comparable
  #: (self) -> Integer
  def <=>(other) = raise(NotImplementedError)

  #: (self) -> bool
  def <(other) = (self <=> other) < 0

  #: (self) -> bool
  def <=(other) = (self <=> other) <= 0

  #: (self) -> bool
  def >(other) = (self <=> other) > 0

  #: (self) -> bool
  def >=(other) = (self <=> other) >= 0

  #: (self, self) -> bool
  def between?(lo, hi) = (self <=> lo) >= 0 && (self <=> hi) <= 0

  #: (self, self) -> self
  def clamp(lo, hi)
    raise ArgumentError, "min argument must be less than or equal to max argument" if (lo <=> hi) > 0
    return lo if (self <=> lo) < 0
    return hi if (self <=> hi) > 0

    self
  end
end
