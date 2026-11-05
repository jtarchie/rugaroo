# rbs_inline: enabled

require "base64"
require "digest"
require "zlib"
require "securerandom"
require "cgi/escape"

# A content-addressed store: blobs keyed by SHA-256, checked by CRC32 on read.
class BlobStore
  #: () -> void
  def initialize
    @blobs = {} #: Hash[String, String]
    @crcs = {} #: Hash[String, Integer]
  end

  #: (String) -> String
  def put(data)
    key = Digest::SHA256.hexdigest(data)
    @blobs[key] = Base64.strict_encode64(data)
    @crcs[key] = Zlib.crc32(data)
    key
  end

  #: (String) -> String?
  def get(key)
    enc = @blobs[key]
    return nil if enc.nil?

    data = Base64.strict_decode64(enc)
    raise "corrupt #{key}" unless Zlib.crc32(data) == @crcs[key]

    data
  end
end

store = BlobStore.new
k = store.put("hello, world")
puts k, store.get(k).inspect, store.get("nope").inspect

text = "The quick brown fox jumps over the lazy dog"
puts Digest::MD5.hexdigest(text)
puts Digest::SHA1.hexdigest(text)
puts Digest::SHA384.hexdigest(text)
puts Digest::SHA512.hexdigest("")
puts Digest::SHA256.digest(text).bytesize
puts Digest::MD5.base64digest(text)
puts Digest::SHA256.hexdigest(text) == Digest::SHA256.new.update("The quick ").update("brown fox jumps over the lazy dog").hexdigest
d = Digest::SHA1.new
d << "abc"
puts d.hexdigest, d.to_s == d.hexdigest

puts Base64.encode64("A" * 60)
puts Base64.encode64("hi").inspect
puts Base64.urlsafe_encode64("\xff\xfe?>"), Base64.urlsafe_encode64("ab", padding: false)
puts Base64.urlsafe_decode64("__4_Pg"), Base64.decode64("aGVsbG8=\n").inspect
begin
  Base64.strict_decode64("not base64!")
rescue ArgumentError => e
  puts "ArgumentError: #{e.message}"
end

puts Zlib.crc32(""), Zlib.crc32("abc"), Zlib.adler32("Wikipedia"), Zlib.crc32("def", Zlib.crc32("abc")) == Zlib.crc32("abcdef")

puts SecureRandom.hex.size, SecureRandom.hex(4).match?(/\A[0-9a-f]{8}\z/)
puts SecureRandom.uuid.match?(/\A\h{8}-\h{4}-4\h{3}-[89ab]\h{3}-\h{12}\z/)
puts SecureRandom.random_bytes(10).bytesize, SecureRandom.base64(9).size, SecureRandom.urlsafe_base64.match?(/\A[\w-]+\z/)
puts SecureRandom.alphanumeric(12).match?(/\A[A-Za-z0-9]{12}\z/)
n = SecureRandom.random_number(10)
puts n >= 0 && n < 10
f = SecureRandom.random_number
puts f >= 0.0 && f < 1.0

puts CGI.escape("a b&c=d/é~"), CGI.unescape("a+b%2F%C3%A9")
puts CGI.escapeHTML(%q(<a href="x">'&</a>)), CGI.unescapeHTML("&lt;&amp;&#39;&quot;&gt;&#x41;&#65;")
puts CGI.escapeURIComponent("a b+c"), CGI.unescapeURIComponent("a%20b+c")
