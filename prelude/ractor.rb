# rbs_inline: enabled

#
# Ractor (decision 103): an isolated goroutine with a port. Ractor.new's
# block is checked for isolation at compile time (internal/compiler/ractor.go),
# messages and constructor arguments are deep-copied (rbRactorCopy), and
# Ractor.current is the ractor registered for the running goroutine
# (rbCurrentRactor, decision 104), main when none.

# @go_type struct { port *Ractor_Port; done chan struct{}; val any; err any; name *String; id int; loc rbCallerFrames; catches []any }
class Ractor < Object
  class Error < RuntimeError; end
  class IsolationError < Error; end
  class ClosedError < StopIteration; end
  class MovedError < Error; end
  class UnsafeError < Error; end

  # value/join raise it around the exception that ended the ractor, its cause.
  class RemoteError < Error
    #: (Ractor, Exception) -> void
    def initialize(ractor, cause)
      @message = "thrown by remote Ractor."
      @ractor = ractor
      @cause = cause
    end

    #: () -> Ractor
    def ractor = @ractor
  end

  # An unbounded message queue, owned by the ractor that created it: only it may receive.
  # @go_type struct { q rbPort; owner *Ractor; id int }
  class Port < Object
    #: () -> Port
    def self.new = %x{ return &Ractor_Port{owner: rbCurrentRactor(), id: int(rbPortIDs.Add(1))} }

    # The message is deep-copied first; a frozen Array/Hash is shared, as MRI shares shareable objects.
    #: (untyped) -> self
    def <<(obj) = %x{
      self.q.send(rbRactorCopy(obj))
      return self
    }

    #: (untyped) -> self
    def send(obj) = self << obj

    # `move: true` is a plain send: the sender keeps a usable object, where MRI leaves a MovedObject.
    #: (untyped, Hash[Symbol, bool]) -> self
    def __send_2(obj, opts) = self << obj

    # Ractor::Error unless called by the creating ractor, as MRI.
    #: () -> untyped
    def receive = %x{ return self.receiveOwned() }

    #: () -> void
    def close = %x{ self.q.close() }

    #: () -> bool
    def closed? = %x{ Boolean(self.q.isClosed()) }

    #: () -> String
    def inspect = %x{ return String(fmt.Sprintf("#<Ractor::Port to:#%d id:%d>", self.owner.id, self.id)) }
  end

  # Arguments are deep-copied into the block (Thread.new's arity overloads, decision 45).
  #: () { () -> untyped } -> Ractor
  def self.new = %x{ return rbNewRactor().start(func() any { return blk() }) }

  # @rbs [A] (A) { (A) -> untyped } -> Ractor
  def self.__new_1(a) = %x{
    a = rbCopyAs(a, map[any]any{})
    return rbNewRactor().start(func() any { return blk(a) })
  }

  # @rbs [A, B] (A, B) { (A, B) -> untyped } -> Ractor
  def self.__new_2(a, b) = %x{
    seen := map[any]any{}
    a, b = rbCopyAs(a, seen), rbCopyAs(b, seen)
    return rbNewRactor().start(func() any { return blk(a, b) })
  }

  # @rbs [A, B, C] (A, B, C) { (A, B, C) -> untyped } -> Ractor
  def self.__new_3(a, b, c) = %x{
    seen := map[any]any{}
    a, b, c = rbCopyAs(a, seen), rbCopyAs(b, seen), rbCopyAs(c, seen)
    return rbNewRactor().start(func() any { return blk(a, b, c) })
  }

  # `Ractor.new(name: "w") { }`, routed by the compiler from a keyword literal (a Hash argument is a message, `__new_1`).
  #: (Hash[Symbol, String]) { () -> untyped } -> Ractor
  def self.__new_named(opts) = %x{
    r := rbNewRactor()
    r.name = Hash_Op_idx(opts, Symbol("name"))
    return r.start(func() any { return blk() })
  }

  # The ractor running this goroutine (or the one that started this thread); main otherwise.
  #: () -> Ractor
  def self.current = %x{ return rbCurrentRactor() }

  #: () -> bool
  def self.main? = %x{ Boolean(rbCurrentRactor() == rbMainRactor) }

  # Blocks for the next message on the current ractor's port.
  #: () -> untyped
  def self.receive = %x{ return rbCurrentRactor().port.q.receive() }

  #: () -> untyped
  def self.recv = receive

  #: () -> Ractor
  def self.main = %x{ return rbMainRactor }

  # Deep-freezes obj and answers it, as MRI does for the Arrays, Hashes and Strings a parser table holds (racc's
  # generated parsers: `Ractor.make_shareable(Racc_arg) if defined?(Ractor)`). ponytail: MRI also freezes an object's
  # ivars and raises for an unshareable Proc; a frozen value is already shared as is (see Ractor::Port).
  #: (untyped, ?copy: bool) -> untyped
  # @dynamic
  def self.make_shareable(obj, copy: false)
    case obj
    when Array then obj.each { |e| make_shareable(e) }
    when Hash then obj.each { |k, v| make_shareable(k); make_shareable(v) }
    end
    obj.freeze
  end

  # Main plus the ractors still running.
  #: () -> Integer
  def self.count = %x{ Integer(1 + rbRactorLive.Load()) }

  # The first port with a message, with it: `port, msg = Ractor.select(a, b)`.
  #: (*Port) -> [Port, untyped]
  def self.select(*ports) = %x{
    p, v := rbPortSelect(rest_)
    return Tuple2[*Ractor_Port, any]{F0: p, F1: v}
  }

  # The message is deep-copied first.
  #: (untyped) -> self
  def <<(obj) = %x{
    self.port.q.send(rbRactorCopy(obj))
    return self
  }

  #: (untyped) -> self
  def send(obj) = self << obj

  # `move: true` is a plain send (see Port#__send_2).
  #: (untyped, Hash[Symbol, bool]) -> self
  def __send_2(obj, opts) = self << obj

  # Blocks until the block finishes; an exception there is re-raised as RemoteError with it as cause.
  #: () -> untyped
  def value = %x{
    <-self.done
    self.reraise()
    return self.val
  }

  #: () -> self
  def join = %x{
    <-self.done
    self.reraise()
    return self
  }

  #: () -> Port
  def default_port = %x{ return self.port }

  #: () -> String?
  def name = %x{ return self.name }

  #: () -> String
  def inspect = %x{
    status := "running"
    select {
    case <-self.done:
      status = "terminated"
    default:
    }
    loc := self.loc.loc(0)
    if loc == "" {
      return String(fmt.Sprintf("#<Ractor:#%d %s>", self.id, status))
    }
    return String(fmt.Sprintf("#<Ractor:#%d %s %s>", self.id, loc, status))
  }

  #: () -> String
  def to_s = inspect
end
