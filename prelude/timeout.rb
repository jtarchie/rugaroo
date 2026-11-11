# prelude/timeout.rb
# rbs_inline: enabled

#
# Timeout.timeout races the block against a timer goroutine (decision 71).
# Go cannot preempt a running goroutine the way MRI can interrupt a stuck
# thread with an injected exception: on timeout, this can only detect that
# time passed and raise from the *caller's* side. The block's own goroutine
# is abandoned running in the background (leaked) if it never returns on
# its own — a real semantic gap from MRI, not just an implementation detail.

module Timeout
  class Error < RuntimeError; end

  #: [X] (Float?, singleton(StandardError)?, String?) { () -> X } -> X
  def self.timeout(sec = nil, klass = nil, message = nil, &block)
    return block.call if sec.nil?

    k = klass || Error
    msg = message || "execution expired"
    __race(sec, -> { k.new(msg) }, &block)
  end

  # @rbs [X] (Float, ^() -> Exception) { () -> X } -> X
  def self.__race(sec, err) = %x{
    type rbTimeoutResult struct {
      val X
      err any
    }
    ch := make(chan rbTimeoutResult, 1)
    rbSpawned.Store(true)
    go func() {
      rbTagGoroutine()
      defer rbUntagGoroutine()
      defer func() {
        if r := recover(); r != nil {
          ch <- rbTimeoutResult{err: rbWrapPanic(r)}
        }
      }()
      ch <- rbTimeoutResult{val: blk()}
    }()
    select {
    case res := <-ch:
      if res.err != nil {
        panic(res.err)
      }
      return res.val
    case <-time.After(time.Duration(float64(sec) * float64(time.Second))):
      panic((*err)())
    }
  }
end
