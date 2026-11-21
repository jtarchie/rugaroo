# rbs_inline: enabled

# Serves Snails' Tep::APP on WEBrick (Go's net/http under rugaroo): no fds, no prefork, one goroutine per request.
require "webrick"

module Tep
  class WEBrickServer < WEBrick::HTTPServer
    #: (Integer) -> void
    def self.start_app(port)
      # ponytail: Snails::Main.start's setup, copied; an upstream `serve` hook would leave only the server choice here
      Snails::Main.configure_database!
      Sqlx::Database.require_leases!
      ActiveJob.register_executor
      Tep::APP.name = Snails.app.app_name
      server = new({ Port: port, BindAddress: "127.0.0.1" })
      Tep.log_process_started(port, 1)
      ActiveJob::Launcher.start
      server.start
    end

    #: (WEBrick::HTTPRequest) -> Tep::Request?
    def self.tep_request(req)
      # tep's own parser reads it, so query, cookie and body handling stay Snails' code
      body = req.body.to_s
      blob = +"#{req.request_method} #{req.unparsed_uri} HTTP/1.1\r\n"
      req.header.each do |name, values|
        next if %w[content-length transfer-encoding].include?(name) # net/http has already de-chunked the body

        values.each { |v| blob << "#{name}: #{v}\r\n" }
      end
      blob << "content-length: #{body.bytesize}\r\n\r\n" << body
      treq = Tep::Parser.parse(blob)
      treq&.parse_body_params
      treq
    end

    #: (WEBrick::HTTPRequest, WEBrick::HTTPResponse) -> void
    def service(req, res)
      treq = WEBrickServer.tep_request(req)
      if treq.nil?
        simple(res, 400, "bad request")
        return
      end
      tres = Tep::Response.new
      begin
        Tep::APP.dispatch(treq, tres)
      rescue Exception => e # rubocop:disable Lint/RescueException -- tep's server boundary does the same
        raise if Snails::ErrorBoundary.fatal?(e)

        simple(res, 500, "internal server error")
        return
      end
      write(tres, res)
    end

    #: (Tep::Response, WEBrick::HTTPResponse) -> void
    def write(tres, res)
      # ponytail: streaming, WebSocket upgrades and Set-Cookie lines answer 501; the scaffold uses none of them
      if tres.streaming || tres.upgrading_ws || !tres.set_cookies.empty?
        simple(res, 501, "not implemented by Tep::WEBrickServer")
        return
      end
      path = tres.file_path.to_s
      body = path.empty? ? tres.body.to_s : File.binread(path)
      tres.headers["Content-Type"] = "text/html; charset=utf-8" if path.empty? && !body.empty? && !tres.headers.key?("Content-Type")
      res.status = tres.status || 500
      tres.headers.each { |k, v| res[k.to_s] = v.to_s }
      res.body = body
    end

    #: (WEBrick::HTTPResponse, Integer, String) -> void
    def simple(res, status, msg)
      res.status = status
      res.body = msg
    end
  end
end
