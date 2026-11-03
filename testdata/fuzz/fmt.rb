# frozen_string_literal: true

# Differential fuzz driver (decision 436, fuzzdiff_test.go): reads `op<TAB>hexarg<TAB>...` lines on stdin, prints `OK <hex of result>` or `EXC <class>` per line.
# Strings arrive hex-encoded so any bytes pass; floats as the 16 hex digits of their big-endian IEEE bits.

#: (String h) -> String
def unhex(h)
  [h].pack("H*")
end

#: (String h) -> Float
def unfloat(h)
  unhex(h).unpack("G").fetch(0).to_f
end

#: (String op, Array[String] a) -> String
def run_case(op, a)
  case op
  when "fmt_float" then format(unhex(a.fetch(0)), unfloat(a.fetch(1)))
  when "fmt_int" then format(unhex(a.fetch(0)), Integer(a.fetch(1)))
  when "fmt_str" then format(unhex(a.fetch(0)), unhex(a.fetch(1)))
  when "float_to_s" then unfloat(a.fetch(0)).to_s
  when "float_round" then unfloat(a.fetch(0)).round(Integer(a.fetch(1))).to_s
  when "float_floor" then unfloat(a.fetch(0)).floor(Integer(a.fetch(1))).to_s
  when "float_ceil" then unfloat(a.fetch(0)).ceil(Integer(a.fetch(1))).to_s
  when "pack_int" then a.fetch(1).split(",").map { |x| Integer(x) }.pack(unhex(a.fetch(0)))
  when "unpack" then unhex(a.fetch(1)).unpack(unhex(a.fetch(0))).inspect
  when "str_tr" then unhex(a.fetch(0)).tr(unhex(a.fetch(1)), unhex(a.fetch(2)))
  when "str_delete" then unhex(a.fetch(0)).delete(unhex(a.fetch(1)))
  when "str_squeeze" then unhex(a.fetch(0)).squeeze(unhex(a.fetch(1)))
  when "str_count" then unhex(a.fetch(0)).count(unhex(a.fetch(1))).to_s
  when "str_succ" then unhex(a.fetch(0)).succ
  when "str_to_i" then unhex(a.fetch(0)).to_i(Integer(a.fetch(1))).to_s
  when "str_integer" then Integer(unhex(a.fetch(0)), Integer(a.fetch(1))).to_s
  when "str_float" then Float(unhex(a.fetch(0))).to_s
  when "str_split" then unhex(a.fetch(0)).split(unhex(a.fetch(1)), Integer(a.fetch(2))).inspect
  when "str_slice" then unhex(a.fetch(0))[Integer(a.fetch(1)), Integer(a.fetch(2))].inspect
  else raise ArgumentError, "unknown op #{op}"
  end
end

#: (String op, Array[String] a) -> String
def safe_case(op, a)
  "OK " + run_case(op, a).unpack1("H*").to_s
rescue Exception => e # rubocop:disable Lint/RescueException
  "EXC " + e.class.name.to_s
end

while (line = $stdin.gets)
  parts = line.chomp.split("\t", -1)
  puts safe_case(parts.fetch(0), parts.drop(1))
end
