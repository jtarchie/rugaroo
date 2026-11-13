# net-ftp's set_socket: an untyped parameter written over a placeholder ivar (decision 458).
class Sockish
  class Null
    def read = "none"
  end

  class Buffered
    def initialize(io) = (@io = io)
    def read = "buffered #{@io}"
  end

  def initialize
    @sock = Null.new
    @spare = Buffered.new("spare")
  end

  def connect(host) = attach(Buffered.new(host))
  def reuse = attach(instance_variable_get(:@spare))

  def attach(sock)
    @sock = sock
    @sock.read
  end
end
