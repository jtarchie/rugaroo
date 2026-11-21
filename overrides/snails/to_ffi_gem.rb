# Rewrites Spinel's ffi_* syntax in a file to the ffi gem's spelling in place (the shape of a possible upstream PR).
TYPES = { "str" => "string", "binstr" => "string", "ptr" => "pointer" }.freeze

def types(list) = list.gsub(/:(\w+)/) { ":#{TYPES.fetch(Regexp.last_match(1), Regexp.last_match(1))}" }

ARGV.each do |path|
  src = File.read(path)
  next unless src.match?(/^\s*ffi_(func|lib|const|callback|buffer|read_)/)

  out = src.gsub(/^(\s*)ffi_lib (.+)$/) { "#{Regexp.last_match(1)}extend FFI::Library\n#{Regexp.last_match(1)}ffi_lib #{Regexp.last_match(2)}" }
           .gsub(/^(\s*)ffi_func (:\w+),\s*(\[[^\]]*\]),\s*:(\w+)/) do
             "#{Regexp.last_match(1)}attach_function #{Regexp.last_match(2)}, #{types(Regexp.last_match(3))}, :#{TYPES.fetch(Regexp.last_match(4), Regexp.last_match(4))}"
           end
           .gsub(/^(\s*)ffi_callback (:\w+),\s*(\[[^\]]*\]),\s*:(\w+)/) do
             "#{Regexp.last_match(1)}callback #{Regexp.last_match(2)}, #{types(Regexp.last_match(3))}, :#{TYPES.fetch(Regexp.last_match(4), Regexp.last_match(4))}"
           end
           .gsub(/^(\s*)ffi_const :(\w+),\s*(.+)$/) { "#{Regexp.last_match(1)}#{Regexp.last_match(2)} = #{Regexp.last_match(3)}" }
           .gsub(/^(\s*)ffi_buffer :(\w+),\s*(\d+)$/) do
             "#{Regexp.last_match(1)}#{Regexp.last_match(2).upcase}_BUF = FFI::MemoryPointer.new(#{Regexp.last_match(3)})\n" \
               "#{Regexp.last_match(1)}def self.#{Regexp.last_match(2)} = #{Regexp.last_match(2).upcase}_BUF"
           end
           .gsub(/^(\s*)ffi_read_ptr :(\w+),\s*(\d+)$/) { "#{Regexp.last_match(1)}def self.#{Regexp.last_match(2)}(buf) = buf.get_pointer(#{Regexp.last_match(3)})" }
           # Spinel passes an Integer to :ptr (SQLITE_TRANSIENT is -1); the gem refuses one, so the destructor slot is a pointer-sized :long.
           .sub("attach_function :sqlite3_bind_text, [:pointer, :int, :string, :int, :pointer]", "attach_function :sqlite3_bind_text, [:pointer, :int, :string, :int, :long]")
  File.write(path, "require \"ffi\"\n#{out}")
end
