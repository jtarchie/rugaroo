# rbs_inline: enabled

# Stands in for snails-core/tep/net.rb (sp_net lives in Spinel's runtime): tep_webrick.rb serves, so only the clock and crypto are real.
require "openssl"
require "digest"
require "base64"
require "securerandom"

module Tep
  module Sock
    #: () -> Integer
    def self.sphttp_now_us = (Process.clock_gettime(Process::CLOCK_MONOTONIC) * 1_000_000.0).to_i

    #: (String) -> Integer
    def self.sphttp_filesize(path) = File.exist?(path) ? File.size(path) : -1

    #: (Integer, String) -> Integer
    def self.sphttp_write_str(_fd, _s)
      -1 # Spinel's type seeding writes to fd 0 at load; with no fds here every write fails, as write(2) on a bad fd does
    end

    #: (Integer, String, Integer) -> Integer
    def self.sphttp_write_bytes(_fd, _data, _n) = -1

    #: (Integer, String) -> void
    def self.sphttp_write_chunk(_fd, _s) = nil

    #: (Integer) -> void
    def self.sphttp_write_chunk_end(_fd) = nil

    #: (Integer, String) -> void
    def self.sphttp_sendfile(_fd, _path) = nil

    #: () -> Integer
    def self.sphttp_getpid = Process.pid

    #: () -> Integer
    def self.sphttp_shutdown_requested = 0

    #: () -> Integer
    def self.unused
      raise NotImplementedError, "tep's fd-level server is replaced by Tep::WEBrickServer" # its servers stay loaded, never run
    end

    #: (Integer, Integer) -> Integer
    def self.sphttp_listen(_a1, _a2) = unused

    #: (Integer) -> Integer
    def self.sphttp_accept(_a1) = unused

    #: (Integer) -> Integer
    def self.sphttp_accept_nb(_a1) = unused

    #: (Integer) -> Integer
    def self.sphttp_set_nonblock(_a1) = unused

    #: (Integer) -> Integer
    def self.sphttp_close(_a1) = unused

    #: (Integer, Integer) -> String
    def self.sphttp_recv_some(_a1, _a2) = unused.to_s

    #: (Integer, Integer) -> String
    def self.sphttp_recv_all(_a1, _a2) = unused.to_s

    #: (String, Integer) -> Integer
    def self.sphttp_connect(_a1, _a2) = unused

    #: () -> Integer
    def self.sphttp_fork = unused

    #: (Integer) -> Integer
    def self.sphttp_exit(_a1) = unused

    #: () -> Integer
    def self.sphttp_wait_any = unused

    #: () -> Integer
    def self.sphttp_install_term_handlers = unused

    #: () -> Integer
    def self.sphttp_poll_reset = unused

    #: (Integer, Integer) -> Integer
    def self.sphttp_poll_add(_a1, _a2) = unused

    #: (Integer) -> Integer
    def self.sphttp_poll_run(_a1) = unused

    #: (Integer) -> Integer
    def self.sphttp_poll_ready(_a1) = unused

    #: (String, Integer) -> String
    def self.sphttp_shell_capture(_a1, _a2) = unused.to_s
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
