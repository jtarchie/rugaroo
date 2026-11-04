# frozen_string_literal: true

require "json"

# Differential fuzz driver for JSON (decision 436, fuzzdiff_test.go): `op<TAB>hexarg...` lines in, `OK <hex>` or `EXC <class>` out.

#: (String h) -> String
def unhex(h)
  [h].pack("H*").force_encoding("UTF-8")
end

#: (String h) -> Float
def unfloat(h)
  unhex(h).unpack("G").fetch(0).to_f
end

#: (String op, Array[String] a) -> String
def run_case(op, a)
  case op
  when "gen_float" then JSON.generate([unfloat(a.fetch(0))])
  when "float_roundtrip" then (JSON.parse(JSON.generate([unfloat(a.fetch(0))])).first == unfloat(a.fetch(0))).to_s
  when "gen_str" then JSON.generate([unhex(a.fetch(0))])
  when "gen_int" then JSON.generate([Integer(a.fetch(0))])
  when "parse" then JSON.parse(unhex(a.fetch(0))).inspect
  when "regen" then JSON.generate(JSON.parse(unhex(a.fetch(0))))
  when "pretty" then JSON.pretty_generate(JSON.parse(unhex(a.fetch(0))))
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
