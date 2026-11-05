# rbs_inline: enabled

class ValidationError < StandardError
  attr_reader :field #: String

  #: (String, String) -> void
  def initialize(field, msg)
    super(msg)
    @field = field
  end
end

#: (Integer) -> Integer
def checked_div(n)
  100 / n
rescue ZeroDivisionError => e
  puts "rescued: #{e.message}"
  -1
ensure
  puts "checked #{n}"
end

#: (String) -> String
def validate(s)
  raise ValidationError.new("name", "too short") if s.size < 3

  s
end

puts checked_div(5), checked_div(0)

begin
  validate("ab")
rescue ValidationError => e
  puts "#{e.field}: #{e.message}"
end

#: (Integer) -> String
def classify(n)
  begin
    raise ArgumentError, "negative" if n < 0
    return "zero" if n == 0

    "positive"
  rescue ArgumentError => e
    return "error(#{e.message})"
  ensure
    puts "classified #{n}"
  end
end

puts classify(-1), classify(0), classify(1)

begin
  [1, 2].fetch(5)
rescue IndexError => e
  puts "index: #{e.message}"
end

begin
  begin
    raise "inner"
  rescue RuntimeError => e
    raise ArgumentError, "outer from #{e.message}"
  end
rescue => e
  puts e.message, e.inspect
end

#: () { () -> void } -> void
def risky
  yield
rescue StandardError => e
  puts "caught #{e.message}"
end

risky { raise "in block" }
risky { puts "no error" }
