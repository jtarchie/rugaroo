# rbs_inline: enabled

# Stands in for snails-core/tep/net.rb: its sp_net calls live in Spinel's runtime, so here they are Socket/Process.
require "socket"
require "openssl"
require "digest"
require "base64"
require "securerandom"

module Tep
  module Sock
    @servers = {} #: Hash[Integer, TCPServer]
    @ios = {} #: Hash[Integer, BasicSocket]
    @stop = false #: bool

    #: (BasicSocket) -> Integer
    def self.track(io)
      @ios[io.fileno] = io
      io.fileno
    end

    #: (Integer, Integer) -> Integer
    def self.sphttp_listen(port, _reuse)
      s = TCPServer.new("0.0.0.0", port)
      @servers[s.fileno] = s
      s.fileno
    rescue SystemCallError
      -1
    end

    #: (Integer) -> Integer
    def self.sphttp_accept(sfd) = track(@servers.fetch(sfd).accept)

    #: (Integer) -> Integer
    def self.sphttp_accept_nb(sfd)
      track(@servers.fetch(sfd).accept_nonblock)
    rescue IO::WaitReadable
      -1
    end

    #: (Integer) -> Integer
    def self.sphttp_set_nonblock(_fd) = 0

    #: (Integer) -> Integer
    def self.sphttp_close(fd)
      @ios.delete(fd)&.close
      @servers.delete(fd)&.close
      0
    end

    #: (Integer, String) -> Integer
    def self.sphttp_write_str(fd, s)
      @ios[fd]&.write(s) || -1 # Spinel type seeding writes to fd 0 at load
    end

    #: (Integer, String, Integer) -> Integer
    def self.sphttp_write_bytes(fd, data, n) = @ios[fd]&.write(data.byteslice(0, n).to_s) || -1

    #: (Integer, Integer) -> String
    def self.sphttp_recv_some(fd, n)
      @ios[fd]&.read_nonblock(n) || "" # "" is EOF to every caller
    rescue IO::WaitReadable, EOFError
      ""
    end

    #: (Integer, Integer) -> String
    def self.sphttp_recv_all(fd, n) = @ios[fd]&.read(n) || ""

    #: (String, Integer) -> Integer
    def self.sphttp_connect(host, port) = track(TCPSocket.new(host, port))

    # ponytail: Snails 1.0 clamps workers to 1, so no prefork; Go cannot fork.
    #: () -> Integer
    def self.sphttp_fork = -1

    #: (Integer) -> void
    def self.sphttp_exit(status)
      $stdout.flush
      exit!(status)
    end

    #: () -> Integer
    def self.sphttp_getpid = Process.pid

    #: () -> Integer
    def self.sphttp_wait_any = -1

    #: () -> Integer
    def self.sphttp_install_term_handlers
      trap("TERM") { @stop = true }
      trap("INT") { @stop = true }
      0
    end

    #: () -> Integer
    def self.sphttp_shutdown_requested = @stop ? 1 : 0

    # ponytail: no poll_* (only the fiber server uses them, unreachable from Snails::Main).

    #: (String, Integer) -> String
    def self.sphttp_shell_capture(cmd, n) = IO.popen(cmd, &:read).to_s.byteslice(0, n).to_s

    #: () -> Integer
    def self.sphttp_now_us = (Process.clock_gettime(Process::CLOCK_MONOTONIC) * 1_000_000.0).to_i

    #: (String) -> Integer
    def self.sphttp_filesize(path) = File.exist?(path) ? File.size(path) : -1

    #: (Integer, String) -> void
    def self.sphttp_sendfile(fd, path)
      return unless File.exist?(path)

      data = File.binread(path)
      sphttp_write_bytes(fd, data, data.bytesize)
    end

    #: (Integer, String) -> void
    def self.sphttp_write_chunk(fd, s)
      sphttp_write_str(fd, "#{s.bytesize.to_s(16)}\r\n")
      sphttp_write_bytes(fd, s, s.bytesize)
      sphttp_write_str(fd, "\r\n")
    end

    #: (Integer) -> void
    def self.sphttp_write_chunk_end(fd)
      sphttp_write_str(fd, "0\r\n\r\n")
    end
  end

  module Crypto
    WS_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11".freeze #: String

    #: (String, String) -> String
    def self.sp_crypto_hmac_sha256_hex(key, msg) = OpenSSL::HMAC.hexdigest("SHA256", key, msg)

    #: (String, String) -> String
    def self.sp_crypto_hmac_sha256_b64url(key, msg) = sp_crypto_b64url_encode(OpenSSL::HMAC.digest("SHA256", key, msg))

    #: (String) -> String
    def self.sp_crypto_b64url_encode(s) = Base64.urlsafe_encode64(s, padding: false)

    #: (String) -> String
    def self.sp_crypto_b64url_decode(s) = Base64.urlsafe_decode64(s)

    #: (String, String, Integer) -> String
    def self.sp_crypto_pbkdf2_sha256_b64url(secret, salt, iters)
      sp_crypto_b64url_encode(OpenSSL::PKCS5.pbkdf2_hmac(secret, salt, iters, 32, "SHA256"))
    end

    #: (Integer) -> String
    def self.sp_crypto_random_b64url(n) = SecureRandom.urlsafe_base64(n, false)

    #: (String) -> String
    def self.sp_crypto_sha1_hex(s) = Digest::SHA1.hexdigest(s)

    #: (String) -> String
    def self.sp_crypto_websocket_accept(key) = Base64.strict_encode64(Digest::SHA1.digest(key + WS_GUID))
  end
end
