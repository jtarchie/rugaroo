# frozen_string_literal: true

# A constant assigned inside a class-body block (unicode-display_width's INDEX, Marshal.load of a data file inside
# File.open's block): declared untyped in gem code, assigned where the body's statement runs.
class BlkConstWidth
  DATA_FILE = __FILE__
  File.open(DATA_FILE, "rb") do |file|
    data = file.read
    INDEX = data.bytesize
  end
  FIRST = INDEX + 1

  def self.idx = INDEX
end
