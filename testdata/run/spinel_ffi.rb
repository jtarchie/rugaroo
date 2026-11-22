# Spinel ffi_* syntax (decision 375), driving sqlite3 as snails does; MRI reads it through spinel_ffi_mri/ on RUBYLIB.
# env: RUBYLIB=spinel_ffi_mri
require "spinel/ffi"

module SpinelSQL
  ffi_lib "sqlite3"
  ffi_cflags "-I/usr/local/include"

  ffi_const :OK, 0
  ffi_const :ROW, 100
  ffi_const :DONE, 101

  ffi_func :sqlite3_open, [:str, :ptr], :int
  ffi_func :sqlite3_close, [:ptr], :int
  ffi_func :sqlite3_prepare_v2, [:ptr, :str, :int, :ptr, :ptr], :int
  ffi_func :sqlite3_bind_text, [:ptr, :int, :str, :int, :ptr], :int
  ffi_func :sqlite3_step, [:ptr], :int, blocking: true
  ffi_func :sqlite3_column_int, [:ptr, :int], :int
  ffi_func :sqlite3_column_text, [:ptr, :int], :str
  ffi_func :sqlite3_finalize, [:ptr], :int
  ffi_func :sqlite3_errmsg, [:ptr], :str

  ffi_buffer :db_out, 8
  ffi_buffer :stmt_out, 8
  ffi_read_ptr :read_ptr, 0
  ffi_read_i32 :read_i32, 0
  ffi_write_i32 :write_i32, 0
end

module SpinelLibC
  ffi_func :strlen, [:str], :size_t # no ffi_lib: libc, as Spinel links it
end

puts SpinelLibC.strlen("hello, world")
puts SpinelSQL.sqlite3_open(":memory:", SpinelSQL.db_out) == SpinelSQL::OK
db = SpinelSQL.read_ptr(SpinelSQL.db_out)
puts db.nil?

rc = SpinelSQL.sqlite3_prepare_v2(db, "SELECT 40 + 2, upper(?)", -1, SpinelSQL.stmt_out, nil)
stmt = SpinelSQL.read_ptr(SpinelSQL.stmt_out)
puts rc
puts SpinelSQL.sqlite3_bind_text(stmt, 1, "snails", -1, -1) # -1 is SQLITE_TRANSIENT: an Integer where the spec says :ptr
puts SpinelSQL.sqlite3_step(stmt) == SpinelSQL::ROW
puts SpinelSQL.sqlite3_column_int(stmt, 0)
puts SpinelSQL.sqlite3_column_text(stmt, 1)
puts SpinelSQL.sqlite3_step(stmt) == SpinelSQL::DONE
puts SpinelSQL.sqlite3_finalize(stmt)
puts SpinelSQL.sqlite3_prepare_v2(db, "SELEKT", -1, SpinelSQL.stmt_out, nil)
puts SpinelSQL.sqlite3_errmsg(db)
puts SpinelSQL.sqlite3_close(db)

SpinelSQL.write_i32(SpinelSQL.db_out, 123_456)
puts SpinelSQL.read_i32(SpinelSQL.db_out)
