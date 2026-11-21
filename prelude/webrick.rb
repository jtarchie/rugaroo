# prelude/webrick.rb
# rbs_inline: enabled

require_relative "date"
require_relative "time_parse"
require_relative "fileutils"
require_relative "tempfile"
require_relative "digest"
require_relative "cgi"
require_relative "strscan"
require_relative "singleton"
require_relative "etc"
require_relative "timeout"
require_relative "uri"
require_relative "socket"
require_relative "erb"
require_relative "delegate"
#
# WEBrick's server on Go's net/http: one goroutine per request instead of
# one thread. Behaviour kept from WEBrick because programs observe it: no
# Content-Type unless set (Go would sniff one), form bodies parsed into
# `query` only for form content types, mount_proc answering only GET, HEAD,
# POST and PUT, relative Location headers made absolute, and HTTPStatus
# exceptions becoming their status code.

module WEBrick
  # @go_type struct { r *http.Request; query *Hash[String, String]; body *String }
  class HTTPRequest < Object
    #: () -> String
    def request_method = %x{ String(self.r.Method) }

    #: () -> String
    def path = %x{ String(self.r.URL.Path) }

    #: () -> String?
    def query_string = %x{
      if self.r.URL.RawQuery == "" {
        return nil
      }
      return Ref(String(self.r.URL.RawQuery))
    }

    #: () -> Hash[String, String]
    def query = %x{ self.query }

    #: () ?{ (String) -> void } -> String?
    def body = %x{
      if blk != nil && self.body != nil { // one chunk: net/http has read it all
        blk(*self.body)
      }
      return self.body
    }

    #: () -> String
    def unparsed_uri = %x{ String(self.r.RequestURI) }

    #: () -> URI::Generic
    def request_uri = URI.parse(__uri)

    #: () -> String
    def __uri = %x{ String(rbWEBrickURI(self.r)) }

    #: () -> Hash[String, untyped]
    def meta_vars = %x{ rbWEBrickMetaVars(self.r) }

    #: (String) -> String?
    def [](name) = %x{
      values := self.r.Header.Values(string(name))
      if len(values) == 0 {
        return nil
      }
      return Ref(String(strings.Join(values, ", ")))
    }

    #: () -> Hash[String, Array[String]]
    def header = %x{ rbHTTPHeaderFields(self.r.Header) }

    #: () { (String, String) -> void } -> void
    def each
      header.each { |k, v| yield k, v.join(", ") }
    end

    #: () -> Array[Cookie]
    def cookies = %x{ rbWEBrickParseCookies(self.r.Header.Get("Cookie")) }
  end

  class Cookie < Object
    #: (String, String) -> void
    def initialize(name, value)
      @name = name
      @value = value
    end

    attr_accessor :name #: String
    attr_accessor :value #: String

    #: () -> String
    def to_s = "#{name}=#{value}"
  end

  # @go_type struct { status Integer; body String; header http.Header; cookies *Array[any] }
  class HTTPResponse < Object
    #: () -> Integer
    def status = %x{ self.status }

    #: (Integer) -> void
    def status=(status)
      %x{ self.status = status }
    end

    #: () -> String
    def body = %x{ self.body }

    #: (untyped) -> void
    def body=(body)
      %x{
        s, ok := rbUnbox(body).(String)
        if !ok { // WEBrick's IO and streaming (callable) bodies
          panic(NewNotImplementedError(Ref(String("rugaroo's WEBrick takes a String body, not " + rbClassName(body)))))
        }
        self.body = s
      }
    end

    #: (String) -> String?
    def [](name) = %x{
      values := self.header.Values(string(name))
      if len(values) == 0 {
        return nil
      }
      return Ref(String(strings.Join(values, ", ")))
    }

    #: (String, String) -> void
    def []=(name, value)
      %x{ self.header.Set(string(name), string(value)) }
    end

    #: (String) -> void
    def content_type=(value)
      self["Content-Type"] = value
    end

    #: () -> String?
    def content_type = self["Content-Type"]

    #: () -> Array[untyped]
    def cookies = %x{ self.cookies }

    #: (String) -> void
    def upgrade!(protocol)
      self["Connection"] = "upgrade"
      self["Upgrade"] = protocol
    end

    #: (singleton(HTTPStatus::Redirect), String) -> void
    def set_redirect(status, url) = %x{
      loc := string(url)
      self.body = String("<HTML><A HREF=\\"" + loc + "\\">" + loc + "</A>.</HTML>\\n")
      self.header.Set("Location", loc)
      inst := any(status).(interface{ New(*String) ExceptionI }).New(nil)
      panic(inst)
    }
  end

  # A struct class over a Go handle, so it can be subclassed as rackup's Server is.
  class HTTPServer < Object
    # @go_type struct { srv *http.Server; ln net.Listener; mux *http.ServeMux }
    class Handle__ < Object
      #: (Hash[Symbol, untyped]) -> HTTPServer::Handle__
      def self.listen(config) = %x{ return rbWEBrickListen(config) }

      #: (String) { (HTTPRequest, HTTPResponse) -> void } -> void
      def mount(dir) = %x{ rbWEBrickMount(self.mux, string(dir), blk) }

      #: (String) ?{ (HTTPRequest, HTTPResponse) -> void } -> void
      def mount_proc(dir) = %x{
        rbWEBrickMount(self.mux, string(dir), func(req *WEBrick_HTTPRequest, res *WEBrick_HTTPResponse) {
          switch req.r.Method {
          case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut:
            blk(req, res)
          default:
            panic(NewWEBrick_HTTPStatus_MethodNotAllowed(Ref(String("unsupported method '" + req.r.Method + "'."))))
          }
        })
      }

      #: (HTTPRequest, HTTPResponse) -> void
      def route(req, res) = %x{ rbWEBrickRoute(self.mux, req, res) }

      #: () { (HTTPRequest, HTTPResponse) -> void } -> void
      def serve = %x{
        rbSpawned.Store(true)
        self.srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          rbTagGoroutine() // net/http's connection goroutine: its own identity, not the server thread's
          rbWEBrickServe(w, r, blk)
        })
        if err := self.srv.Serve(self.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
          panic(NewIOError(Ref(String(err.Error()))))
        }
      }

      #: () -> void
      def close = %x{ _ = self.srv.Close() }
    end

    attr_reader :config #: Hash[Symbol, untyped]

    # Listens at once, as WEBrick does; Port: 0's chosen port is written back into config[:Port].
    #: (?Hash[Symbol, untyped]) -> void
    def initialize(config = {})
      @config = config
      @h = Handle__.listen(config)
    end

    #: (Symbol) -> untyped
    def [](key) = @config[key]

    #: (String) ?{ (HTTPRequest, HTTPResponse) -> void } -> void
    def mount_proc(dir, &) = @h.mount_proc(dir, &)

    #: (String, singleton(HTTPServlet::AbstractServlet)) -> void
    def mount(dir, servlet)
      @h.mount(dir) { |req, res| servlet.get_instance(self).service(req, res) }
    end

    # What every request runs; a subclass overrides it to bypass the mount table.
    #: (HTTPRequest, HTTPResponse) -> void
    def service(req, res) = @h.route(req, res)

    #: () -> void
    def start = @h.serve { |req, res| service(req, res) }

    #: () -> void
    def shutdown = @h.close
  end

  module HTTPStatus
    class Status < StandardError
      #: () -> Integer
      def code = 500
    end

    class BadRequest < Status
      def code = 400
    end

    class Unauthorized < Status
      def code = 401
    end

    class Forbidden < Status
      def code = 403
    end

    class NotFound < Status
      def code = 404
    end

    class MethodNotAllowed < Status
      def code = 405
    end

    class InternalServerError < Status
      def code = 500
    end

    class Redirect < Status; end

    class MovedPermanently < Redirect
      def code = 301
    end

    class Found < Redirect
      def code = 302
    end
  end

  module HTTPServlet
    class AbstractServlet
      attr_reader :server #: HTTPServer

      # A new instance per request, as WEBrick's.
      #: (HTTPServer) -> AbstractServlet
      def self.get_instance(server) = new(server)

      #: (HTTPServer) -> void
      def initialize(server)
        @server = server
      end

      #: (HTTPRequest, HTTPResponse) -> void
      def service(req, res)
        case req.request_method
        when "GET" then do_GET(req, res)
        when "HEAD" then do_HEAD(req, res)
        when "POST" then do_POST(req, res)
        when "PUT" then do_PUT(req, res)
        when "DELETE" then do_DELETE(req, res)
        when "PATCH" then do_PATCH(req, res)
        when "OPTIONS" then do_OPTIONS(req, res)
        else unsupported(req)
        end
      end

      #: (HTTPRequest, HTTPResponse) -> void
      def do_GET(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_HEAD(req, res) = do_GET(req, res)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_POST(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_PUT(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_DELETE(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_PATCH(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_OPTIONS(req, res) = unsupported(req)

      private

      #: (HTTPRequest) -> void
      def unsupported(req) = raise(HTTPStatus::MethodNotAllowed, "unsupported method '#{req.request_method}'.")
    end
  end
end
