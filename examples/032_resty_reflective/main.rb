# rbs_inline: enabled

#
# jtarchie/resty's library code as written (lib/resty/*.rb, in load order),
# compiled with its reflection intact: constantize, const_get, constants,
# method_missing, Struct.new, extend Enumerable. Only type annotations are
# added; lines rugaroo needed changed are marked `# rugaroo:`. Rack::Request,
# ActiveSupport's inflector and ActiveRecord are replaced by small shims.
# Same transcript as example 25, which ports resty by hand instead.
require "json"
require "net/http"
require "webrick"

# ---- shims: ActiveSupport's inflector, Rack::Request ----

class String
  #: () -> String
  def camelize = split("_").map(&:capitalize).join

  #: () -> untyped
  def constantize = Object.const_get(self)
end

module Rack
  class Request
    attr_reader :env #: Hash[String, untyped]

    #: (Hash[String, untyped]) -> void
    def initialize(env)
      @env = env
    end

    #: () -> String
    def path = env["PATH_INFO"].to_s

    #: () -> String
    def request_method = env["REQUEST_METHOD"].to_s

    #: () -> bool
    def get? = request_method == "GET"

    #: () -> bool
    def post? = request_method == "POST"

    #: () -> bool
    def put? = request_method == "PUT"

    #: () -> bool
    def delete? = request_method == "DELETE"

    #: () -> Hash[String, untyped]
    def params = env["rack.params"]
  end
end

# ---- lib/resty/version.rb ----

module Resty
  VERSION = "0.0.1"
end

# ---- lib/resty/controller.rb ----

module Resty
  module NullController
    NullAction = Struct.new(:params) do
      #: (*untyped) -> untyped
      def method_missing(*args); nil; end
    end #: [Hash[String, untyped]]
    Index   = NullAction
    Show    = NullAction
    Create  = NullAction
    Update  = NullAction
    Edit    = NullAction
  end

  Controller = Struct.new(:namespace, :controller_path) do
    #: (Module, Request) -> Controller
    def self.find_by_namespace_and_request(namespace, request)
      controller_path = request.path.match(%r{^/(\w+)})[1].to_s # rugaroo: .to_s (String? -> String)
      new(namespace, controller_path)
    end

    #: (Request) -> String?
    def self.exists_on_request?(request)
      request.path.match(%r{^/(\w+)(\.\w+)?/?$})[1] rescue nil
    end

    #: () -> untyped
    def constant
      @class ||= begin
        "#{namespace}::#{controller_path.camelize}Controller".constantize
      rescue NameError => e
        NullController
      end
    end

    #: () -> String
    def name
      controller_path
    end
  end #: [Module, String]
end

# ---- lib/resty/resource.rb ----

module Resty
  Resource = Struct.new(:id) do
    #: (Request) -> Resource
    def self.find_by_request(request)
      id = request.path.dup.match(%r{^/\w+/([\w-]+)/?})[1] rescue nil
      id = nil if id == "edit" || id == "new" # rugaroo: was ["edit", "new"].include?(id)
      new(id)
    end
  end #: [String?]
end

# ---- lib/resty/formats/base.rb, unknown_content_type.rb, formats.rb ----

module Resty
  module Formats
    Base = Struct.new(:resource) do
      #: (Request) -> untyped
      def self.matches?(request)
        false
      end

      #: () -> Hash[String, String]
      def headers
        {}
      end

      #: () -> String
      def body
        ""
      end
    end #: [untyped]
  end
end

module Resty
  module Formats
    class UnknownContentType < Base
      def self.matches?(request)
        false
      end

      def body
        ''
      end

      def headers
        {}
      end
    end
  end
end

module Resty
  module Formats
    extend Enumerable #[singleton(Resty::Formats::Base)]

    #: () { (singleton(Base)) -> void } -> void
    def self.each(&block)
      constants.map { |c| self.const_get(c) }.each(&block)
    end
  end
end

# ---- lib/resty/formats/html.rb, json.rb ----

module Resty
  module Formats
    class HTML < Base
      def self.matches?(request)
        request.path =~ /\.html$/ || request.path !~ /\.\w+$/
      end

      def body
        return "" unless resource

        if resource.is_a?(Array)
          resource.map { |r| self.class.new(r).body }.join
        else
          "<h1>#{resource.to_s}</h1>"
        end
      end
    end
  end
end

module Resty
  module Formats
    class JSON < Base
      def self.matches?(request)
        request.path =~ /\.json$/
      end

      def body
        (resource || {}).to_json
      end

      def headers
        { "Content-type" => "application/json" }
      end
    end
  end
end

# ---- lib/resty/actions/base.rb, service_unavailable.rb, actions.rb ----

module Resty
  module Actions
    Base = Struct.new(:controller, :request) do
      #: (Request) -> untyped
      def self.matches?(request)
        false
      end

      #: () -> Hash[String, String]
      def headers
        {}
      end

      private

      #: () -> untyped
      def action
        action_class = self.class.name.split('::').last
        controller.constant.const_get(action_class)
      end
    end #: [Controller, Request]
  end
end

module Resty
  module Actions
    class ServiceUnavailable < Base
      def self.matches?(request)
        false
      end

      #: () -> Integer
      def status
        501
      end

      #: () -> untyped
      def resource
        nil
      end
    end
  end
end

module Resty
  module Actions
    extend Enumerable #[singleton(Resty::Actions::Base)]

    #: () { (singleton(Base)) -> void } -> void
    def self.each(&block)
      constants.map { |c| self.const_get(c) }.each(&block)
    end
  end
end

# ---- lib/resty/actions/*.rb (all.rb order) ----

module Resty
  module Actions
    class Create < Base
      def self.matches?(request)
        request.post? &&
          Controller.exists_on_request?(request)
      end

      #: () -> Integer
      def status
        return 404 if controller.constant == NullController

        resource ? 201 : 422
      end

      def headers
        return {} unless resource

        {
          'Location' => "/#{controller_name}/#{resource.id}#{format}"
        }
      end

      #: () -> untyped
      def resource
        @resource ||= action.new(params).resource
      end

      private

      #: () -> String?
      def format
        request.path.dup.match(%r{(\.\w+)/?$})[1]
      end

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params
      end

      #: () -> String
      def controller_name
        controller.name
      end
    end
  end
end

module Resty
  module Actions
    class Destroy < Base
      def self.matches?(request)
        request.delete?
      end

      #: () -> Integer
      def status
        200
      end

      #: () -> untyped
      def resource
        @resource ||= action.new(params).resource
      end

      private

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params.merge(
          'id' => resource_id
        )
      end

      #: () -> String?
      def resource_id
        @resource_id ||= Resource.find_by_request(request).id
      end
    end
  end
end

module Resty
  module Actions
    class Edit < Base
      def self.matches?(request)
        request.get? &&
          request.path =~ %r{\/edit(\.\w+)?/?$} &&
          !Resource.find_by_request(request).id.nil?
      end

      #: () -> Integer
      def status
        resource ? 200 : 404
      end

      #: () -> untyped
      def resource
        @resources ||= action.new(params).resource
      end

      private

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params.merge(
          'id' => resource_id
        )
      end

      #: () -> String?
      def resource_id
        @resource_id ||= Resource.find_by_request(request).id
      end
    end
  end
end

module Resty
  module Actions
    class Index < Base
      def self.matches?(request)
        request.get? &&
          request.path !~ %r{/(edit|new)(\.\w+)?/?$} &&
          Resource.find_by_request(request).id.nil?
      end

      #: () -> Integer
      def status
        resource ? 200 : 404
      end

      #: () -> untyped
      def resource
        @resources ||= action.new(params).resource
      end

      private

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params
      end
    end
  end
end

module Resty
  module Actions
    class New < Base
      def self.matches?(request)
        request.get? && request.path =~ %r{/new(\.\w+)?/?$}
      end

      #: () -> Integer
      def status
        resource ? 200 : 404
      end

      #: () -> untyped
      def resource
        @resources ||= action.new(params).resource
      end

      private

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params
      end
    end
  end
end

module Resty
  module Actions
    class Show < Base
      def self.matches?(request)
        resource = Resource.find_by_request(request)
        request.get? &&
          !resource.id.nil? &&
          request.path =~ %r{/#{resource.id}(\.\w+)?/?$}
      end

      #: () -> Integer
      def status
        resource ? 200 : 404
      end

      #: () -> untyped
      def resource
        @resource ||= action.new(params).resource
      end

      private

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params.merge(
          'id' => resource_id
        )
      end

      #: () -> String?
      def resource_id
        @resource_id ||= Resource.find_by_request(request).id
      end
    end
  end
end

module Resty
  module Actions
    class Update < Base
      def self.matches?(request)
        request.put?
      end

      #: () -> Integer
      def status
        resource ? 201 : 404
      end

      #: () -> untyped
      def resource
        @resource ||= action.new(params).resource
      end

      def headers
        return {} unless resource

        {
          'Location' => "/#{controller_name}/#{resource.id}#{format}"
        }
      end

      private

      #: () -> String?
      def format
        request.path.dup.match(%r{(\.\w+)/?$})[1]
      end

      #: () -> Hash[String, untyped]
      def params
        @params ||= request.params.merge(
          'id' => resource_id
        )
      end

      #: () -> String?
      def resource_id
        @resource_id ||= Resource.find_by_request(request).id
      end

      #: () -> String
      def controller_name
        controller.name
      end
    end
  end
end

# ---- lib/resty/app.rb ----

module Resty
  class App
    attr_reader :namespace #: Module

    #: (Module) -> void
    def initialize(namespace)
      @namespace = namespace
    end

    #: (Hash[String, untyped]) -> [Integer, Hash[String, String], String]
    def call(env)
      request = Request.new(env)

      controller = Controller.find_by_namespace_and_request(namespace, request)
      action = request.invoker.new(controller, request)
      output = request.formatter.new(action.resource)
    rescue NameError
      action = Actions::ServiceUnavailable.new(controller, request)
      output = request.formatter.new(action.resource)
    ensure
      return [
        action.status,
        action.headers.merge(output.headers),
        output.body
      ]
    end
  end

  class Request < Rack::Request
    #: () -> singleton(Actions::Base)
    def invoker
      Actions.detect do |action|
        action.matches?(self)
      end || Actions::ServiceUnavailable
    end

    #: () -> singleton(Formats::Base)
    def formatter
      Formats.detect do |format|
        format.matches?(self)
      end || Formats::UnknownContentType
    end
  end
end

# ---- spec/example_app.rb, with an in-memory Post for ActiveRecord ----

module ExampleApp
  class Post
    attr_accessor :id #: Integer?
    attr_accessor :title #: String?
    attr_accessor :body #: String?

    RECORDS = {} #: Hash[Integer, Post]

    #: (?Integer?, ?String?, ?String?) -> void
    def initialize(id = nil, title = nil, body = nil)
      @id = id
      @title = title
      @body = body
    end

    #: () -> Array[Post]
    def self.all = RECORDS.values

    #: (untyped) -> Post?
    def self.find(id) = RECORDS[id.to_s.to_i]

    #: (untyped) -> Post
    def self.create(attrs)
      post = new
      post.attributes = attrs
      post.save
      post
    end

    #: () -> void
    def self.delete_all = RECORDS.clear

    #: (untyped) -> void
    def attributes=(attrs)
      return unless attrs.is_a?(Hash)

      id = attrs["id"]
      @id = id.to_s.to_i if id
      title = attrs["title"]
      @title = title.to_s if title
      body = attrs["body"]
      @body = body.to_s if body
    end

    #: () -> bool
    def persisted?
      key = id
      key ? RECORDS.key?(key) : false
    end

    # validates :title, presence: true
    #: () -> bool
    def save
      return false unless title

      key = (@id ||= RECORDS.size + 1)
      RECORDS[key] = self
      true
    end

    #: () -> Post
    def destroy
      key = id
      RECORDS.delete(key) if key
      self
    end

    #: () -> String
    def to_s
      "Post #{id || "not persisted"}"
    end

    #: (*untyped) -> String
    def to_json(*_state) = { "post" => { "id" => id, "title" => title, "body" => body } }.to_json
  end

  module UsersController
  end

  module PostsController
    Action = Struct.new(:params) #: [Hash[String, untyped]]

    class Show < Action
      #: () -> untyped
      def resource
        Post.find(params['id'])
      end
    end

    class Index < Action
      #: () -> untyped
      def resource
        Post.all
      end
    end

    class New < Action
      #: () -> untyped
      def resource
        Post.new
      end
    end

    class Edit < Action
      #: () -> untyped
      def resource
        Post.find(params['id'])
      end
    end

    class Create < Action
      #: () -> untyped
      def resource
        post = Post.create(params['post'])
        post.persisted? ? post : nil
      end
    end

    class Update < Action
      #: () -> untyped
      def resource
        post = Post.find(params['id'])
        return nil unless post

        post.attributes = params['post']
        post.save
        post
      end
    end

    class Destroy < Action
      #: () -> untyped
      def resource
        Post.find(params['id'])&.destroy
      end
    end
  end
end

# ---- serving it: WEBrick builds the Rack env, Net::HTTP drives the specs ----

APP = Resty::App.new(ExampleApp) #: Resty::App

class RestyServlet < WEBrick::HTTPServlet::AbstractServlet
  def service(req, res)
    # Rack would nest post[title]; build that from WEBrick's flat query.
    params = {} #: Hash[String, untyped]
    post = {}
    req.query.each do |key, value|
      match = key.match(/\Apost\[(\w+)\]\z/)
      if match
        post[match[1].to_s] = value
      else
        params[key] = value
      end
    end
    params["post"] = post unless post.empty?
    env = { "REQUEST_METHOD" => req.request_method, "PATH_INFO" => req.path, "rack.params" => params }
    status, headers, body = APP.call(env)
    res.status = status
    headers.each { |name, value| res[name] = value }
    res.body = body
  end
end

#: () -> void
def seed
  ExampleApp::Post.delete_all
  ExampleApp::Post.create({ "id" => "1", "title" => "Title" })
  ExampleApp::Post.create({ "id" => "2", "title" => "Title" })
  ExampleApp::Post.create({ "id" => "3", "title" => "Title" })
end

FORM = { "Content-Type" => "application/x-www-form-urlencoded" } #: Hash[String, String]

#: (Net::HTTP, String, String, ?Hash[String, String]) -> void
def call(http, verb, path, form = {})
  data = URI.encode_www_form(form)
  res = case verb
        when "POST" then http.post(path, data, FORM)
        when "PUT" then http.put(path, data, FORM)
        when "DELETE" then http.delete(path)
        else http.get(path)
        end
  line = "#{verb} #{path} -> #{res.code}"
  # WEBrick makes Location absolute; the port is random, so keep the path.
  location = res["Location"]&.match(%r{^(?:https?://[^/]+)?(/.*)$})
  line += " Location: #{location[1]}" if location
  content_type = res["Content-Type"]
  line += " Content-Type: #{content_type}" if content_type
  puts line
  puts "  #{res.body}" unless res.body.empty?
end

server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
server.mount("/", RestyServlet)
thread = Thread.new { server.start }
http = Net::HTTP.new("127.0.0.1", server.config[:Port])

puts "resty #{Resty::VERSION}", "# errors"
seed
call(http, "GET", "/entries")
call(http, "GET", "/entries/1.json")
call(http, "GET", "/entries/1/edit")
call(http, "POST", "/entries.json")
call(http, "PUT", "/entries/1")
call(http, "POST", "/entries/1/asdfasdfasdfsadfasdft")
call(http, "GET", "/entries/1.format")
call(http, "GET", "/users")
call(http, "POST", "/posts")

puts "# show, new, edit, index"
seed
call(http, "GET", "/posts/123")
call(http, "GET", "/posts/f0429391-ee0c-4c74-b9e1-3aa102bed145")
call(http, "GET", "/posts/1")
call(http, "GET", "/posts/1.json")
call(http, "GET", "/posts/1.html")
call(http, "GET", "/posts/new")
call(http, "GET", "/posts/1/edit")
call(http, "GET", "/posts")
call(http, "GET", "/posts.json")

puts "# full api"
seed
post = { "post[id]" => "100", "post[title]" => "Title", "post[body]" => "Body" }
call(http, "GET", "/posts/100.json")
call(http, "GET", "/posts/new.json")
call(http, "POST", "/posts.json", post)
call(http, "GET", "/posts/100.json")
call(http, "GET", "/posts/100/edit.json")
call(http, "GET", "/posts.json")
call(http, "PUT", "/posts/100.json", { "post[title]" => "Title 123", "post[body]" => "Body 456" })
call(http, "DELETE", "/posts/100.json")
call(http, "GET", "/posts/100.json")

server.shutdown
thread.join
