# rbs_inline: enabled

# Keyword parameters (required, optional, **rest), plus retry, for,
# defined?, splat assignment and class << self, from ruby/spec (#49).

class Request
  attr_reader :path #: String
  attr_reader :method #: Symbol
  attr_reader :retries #: Integer

  #: (String, ?method: Symbol, ?retries: Integer) -> void
  def initialize(path, method: :get, retries: 2)
    @path = path
    @method = method
    @retries = retries
  end

  #: (**String headers) -> String
  def describe(**headers)
    "#{method.upcase} #{path} #{headers.map { |k, v| "#{k}=#{v}" }.join(" ")}".strip
  end

  class << self
    #: (String) -> Request
    def post(path) = new(path, method: :post, retries: 0)
  end
end

puts Request.new("/a").describe
puts Request.new("/b", retries: 5).describe(accept: "json", auth: "token")
puts Request.post("/c").describe

#: (Request) -> String
def fetch(req)
  attempts = 0
  begin
    attempts += 1
    raise IOError, "timeout" if attempts <= req.retries

    "#{req.path} ok after #{attempts}"
  rescue IOError
    retry
  end
end
puts fetch(Request.new("/flaky"))

head, *tail = %w[alpha beta gamma]
puts "#{head} then #{tail.join(", ")}"

for word in tail
  puts "#{word}: #{word.size}"
end
puts "last word was #{word}"

puts [defined?(word), defined?(Request), defined?(missing_thing).inspect].inspect
