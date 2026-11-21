# Spinel ffi_* syntax on MRI through the ffi gem; rugaroo reads it itself (decision 375). snails-rugaroo ships this file too.
require "ffi"

module SpinelFFI
  TYPES = { str: :string, binstr: :string, ptr: :pointer }.freeze
  WIDTHS = { u8: :uint8, i8: :int8, u16: :uint16, i16: :int16, u32: :uint32, i32: :int32, u64: :uint64, i64: :int64, ptr: :pointer }.freeze

  def self.type(t) = TYPES.fetch(t, t)

  # Spinel passes an Integer where the spec says :ptr (sqlite's SQLITE_TRANSIENT is -1); the gem wants a Pointer.
  def self.arg(t, v) = t == :ptr && v.is_a?(Integer) ? FFI::Pointer.new(v) : v

  def self.library(mod)
    mod.extend(FFI::Library) unless mod.is_a?(FFI::Library)
    mod.ffi_lib(FFI::Library::LIBC) unless mod.instance_variable_defined?(:@ffi_libs) # the gem raises on an empty list
  end
end

class Module
  def ffi_lib(name) # FFI::Library#ffi_lib shadows this once the module extends it
    extend(FFI::Library)
    ffi_lib(name)
  end

  def ffi_cflags(*) = nil

  def ffi_const(name, value) = const_set(name, value)

  def ffi_callback(name, args, ret)
    SpinelFFI.library(self)
    callback(name, args.map { SpinelFFI.type(_1) }, SpinelFFI.type(ret))
  end

  def ffi_func(name, args, ret, blocking: false)
    SpinelFFI.library(self)
    raw = :"__spinel_#{name}"
    attach_function(raw, name, args.map { SpinelFFI.type(_1) }, SpinelFFI.type(ret), blocking: blocking)
    define_singleton_method(name) { |*vs| send(raw, *vs.each_with_index.map { |v, i| SpinelFFI.arg(args[i], v) }) }
  end

  def ffi_buffer(name, size)
    buf = FFI::MemoryPointer.new(size)
    define_singleton_method(name) { buf }
  end

  SpinelFFI::WIDTHS.each do |w, t|
    define_method(:"ffi_read_#{w}") { |name, off| define_singleton_method(name) { |buf| buf.get(t, off) } }
    define_method(:"ffi_write_#{w}") do |name, off|
      define_singleton_method(name) do |buf, v|
        buf.put(t, off, v)
        v
      end
    end
  end
end
