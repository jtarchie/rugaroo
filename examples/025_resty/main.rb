# rbs_inline: enabled

#
# A typed port of https://github.com/jtarchie/resty, a web framework that
# forces RESTful conventions, served by WEBrick and driven over HTTP with
# Net::HTTP by replaying resty's own integration specs.
#
# The structure is resty's: a request picks its action class and its format
# class by asking each candidate class `matches?`, then instantiates the
# winner. What changed is what resty did reflectively, which rugaroo does not
# support:
#
#   - `"#{ns}::#{path.camelize}Controller".constantize` and
#     `controller.const_get(action_name)` become a registry of action classes
#     by name (`Hash[String, singleton(Action)]`);
#   - `Actions.constants.map { const_get }` becomes an explicit `ALL` list;
#   - NullController's `method_missing` becomes a NullAction class;
#   - Rack::Request becomes Resty::Request, built from WEBrick's request;
#   - ActiveRecord becomes an in-memory Resty::Model.
require "json"
require "net/http"
require "webrick"

module Resty
  VERSION = "0.0.1" #: String

  # What resty needs from a resource: an id (ActiveRecord::Base's role).
  class Model
    attr_accessor :id #: Integer?
  end

  # An app's controller action (the app-defined `Action = Struct.new(:params)`).
  class Action
    attr_reader :params #: Hash[String, String]

    #: (Hash[String, String]) -> void
    def initialize(params)
      @params = params
    end

    #: () -> untyped
    def resource = nil
  end

  # Stands in for controllers that don't exist: every action finds nothing.
  module NullController
    class NullAction < Action; end

    ACTIONS = {
      "Index" => NullAction, "Show" => NullAction, "Create" => NullAction,
      "Update" => NullAction, "Edit" => NullAction
    } #: Hash[String, singleton(Action)]
  end

  class Request
    attr_reader :request_method #: String
    attr_reader :path #: String
    attr_reader :params #: Hash[String, String]

    #: (String, String, Hash[String, String]) -> void
    def initialize(request_method, path, params)
      @request_method = request_method
      @path = path
      @params = params
    end

    #: () -> bool
    def get? = request_method == "GET"

    #: () -> bool
    def post? = request_method == "POST"

    #: () -> bool
    def put? = request_method == "PUT"

    #: () -> bool
    def delete? = request_method == "DELETE"

    #: () -> singleton(Actions::Base)
    def invoker = Actions::ALL.detect { |action| action.matches?(self) } || Actions::ServiceUnavailable

    #: () -> singleton(Formats::Base)
    def formatter = Formats::ALL.detect { |format| format.matches?(self) } || Formats::UnknownContentType
  end

  class Controller
    attr_reader :actions #: Hash[String, singleton(Action)]
    attr_reader :controller_path #: String

    #: (Hash[String, singleton(Action)], String) -> void
    def initialize(actions, controller_path)
      @actions = actions
      @controller_path = controller_path
    end

    #: (Hash[String, Hash[String, singleton(Action)]], Request) -> Controller
    def self.find_by_namespace_and_request(namespace, request)
      match = request.path.match(%r{^/(\w+)})
      controller_path = match ? match[1].to_s : ""
      new(namespace[controller_path] || NullController::ACTIONS, controller_path)
    end

    #: (Request) -> bool
    def self.exists_on_request?(request) = request.path.match?(%r{^/(\w+)(\.\w+)?/?$})

    #: () -> bool
    def null? = actions.equal?(NullController::ACTIONS)

    #: (String) -> singleton(Action)
    def constant(name) = actions[name] || raise(NameError, "uninitialized constant #{controller_path}::#{name}")

    #: () -> String
    def name = controller_path
  end

  class Resource
    attr_reader :id #: String?

    #: (String?) -> void
    def initialize(id)
      @id = id
    end

    #: (Request) -> Resource
    def self.find_by_request(request)
      match = request.path.match(%r{^/\w+/([\w-]+)/?})
      id = match ? match[1] : nil
      id = nil if id == "edit" || id == "new"
      new(id)
    end
  end

  module Formats
    class Base
      attr_reader :resource #: untyped

      #: (untyped) -> void
      def initialize(resource)
        @resource = resource
      end

      #: (Request) -> bool
      def self.matches?(request) = false

      #: () -> Hash[String, String]
      def headers = {}

      #: () -> String
      def body = ""
    end

    class UnknownContentType < Base; end

    class HTML < Base
      def self.matches?(request) = request.path.match?(/\.html$/) || !request.path.match?(/\.\w+$/)

      def body
        res = resource
        return "" unless res

        if res.is_a?(Array)
          res.map { |r| self.class.new(r).body }.join
        else
          "<h1>#{res}</h1>"
        end
      end
    end

    class JSON < Base
      def self.matches?(request) = request.path.match?(/\.json$/)

      def body = (resource || {}).to_json

      def headers = { "Content-type" => "application/json" }
    end

    ALL = [HTML, JSON] #: Array[singleton(Base)]
  end

  module Actions
    class Base
      attr_reader :controller #: Controller
      attr_reader :request #: Request

      #: (Controller, Request) -> void
      def initialize(controller, request)
        @controller = controller
        @request = request
      end

      #: (Request) -> bool
      def self.matches?(request) = false

      #: () -> Integer
      def status = 501

      #: () -> untyped
      def resource = nil

      #: () -> Hash[String, String]
      def headers = {}

      private

      # The app's action class named like this one ("Show" for Actions::Show).
      #: () -> singleton(Action)
      def action = controller.constant(self.class.name.split("::").last.to_s)

      #: () -> Hash[String, String]
      def params = request.params

      #: () -> String?
      def resource_id = Resource.find_by_request(request).id

      #: () -> Hash[String, String]
      def params_with_id = params.merge("id" => resource_id.to_s)

      #: () -> String
      def format
        match = request.path.match(%r{(\.\w+)/?$})
        match ? match[1].to_s : ""
      end

      #: () -> Hash[String, String]
      def location
        res = resource
        return {} unless res.is_a?(Model)

        { "Location" => "/#{controller.name}/#{res.id}#{format}" }
      end
    end

    class ServiceUnavailable < Base; end

    class Create < Base
      def self.matches?(request) = request.post? && Controller.exists_on_request?(request)

      def status
        return 404 if controller.null?

        resource ? 201 : 422
      end

      def headers = location

      def resource = @resource ||= action.new(params).resource
    end

    class Destroy < Base
      def self.matches?(request) = request.delete?

      def status = 200

      def resource = @resource ||= action.new(params_with_id).resource
    end

    class Edit < Base
      def self.matches?(request)
        request.get? && request.path.match?(%r{/edit(\.\w+)?/?$}) && !Resource.find_by_request(request).id.nil?
      end

      def status = resource ? 200 : 404

      def resource = @resource ||= action.new(params_with_id).resource
    end

    class Index < Base
      def self.matches?(request)
        request.get? && !request.path.match?(%r{/(edit|new)(\.\w+)?/?$}) && Resource.find_by_request(request).id.nil?
      end

      def status = resource ? 200 : 404

      def resource = @resource ||= action.new(params).resource
    end

    class New < Base
      def self.matches?(request) = request.get? && request.path.match?(%r{/new(\.\w+)?/?$})

      def status = resource ? 200 : 404

      def resource = @resource ||= action.new(params).resource
    end

    class Show < Base
      def self.matches?(request)
        id = Resource.find_by_request(request).id
        request.get? && !id.nil? && request.path.match?(%r{/#{id}(\.\w+)?/?$})
      end

      def status = resource ? 200 : 404

      def resource = @resource ||= action.new(params_with_id).resource
    end

    class Update < Base
      def self.matches?(request) = request.put?

      def status = resource ? 201 : 404

      def headers = location

      def resource = @resource ||= action.new(params_with_id).resource
    end

    ALL = [Create, Destroy, Edit, Index, New, Show, Update] #: Array[singleton(Base)]
  end

  class App
    attr_reader :namespace #: Hash[String, Hash[String, singleton(Action)]]

    #: (Hash[String, Hash[String, singleton(Action)]]) -> void
    def initialize(namespace)
      @namespace = namespace
    end

    # Rack's contract: [status, headers, body].
    #: (Request) -> [Integer, Hash[String, String], String]
    def call(request)
      controller = Controller.find_by_namespace_and_request(namespace, request)
      action = begin
        invoked = request.invoker.new(controller, request)
        invoked.resource
        invoked
      rescue NameError
        Actions::ServiceUnavailable.new(controller, request)
      end
      output = request.formatter.new(action.resource)
      [action.status, action.headers.merge(output.headers), output.body]
    end
  end
end

# The app under test: resty's spec/example_app.rb, minus ActiveRecord.
module ExampleApp
  class Post < Resty::Model
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

    #: (String?) -> Post?
    def self.find(id) = id ? RECORDS[id.to_i] : nil

    #: (Hash[String, String]) -> Post
    def self.create(attrs)
      post = new(attrs["post[id]"]&.to_i, attrs["post[title]"], attrs["post[body]"])
      post.save
      post
    end

    #: () -> void
    def self.delete_all = RECORDS.clear

    #: () -> bool
    def persisted?
      key = id
      key ? RECORDS.key?(key) : false
    end

    # Validates presence of title, like the original's ActiveRecord model.
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

    def to_s
      key = id
      key ? "Post #{key}" : "Post not persisted"
    end

    #: (*untyped) -> String
    def to_json(*_state) = { "post" => { "id" => id, "title" => title, "body" => body } }.to_json
  end

  module PostsController
    class Show < Resty::Action
      def resource = Post.find(params["id"])
    end

    class Index < Resty::Action
      def resource = Post.all
    end

    class New < Resty::Action
      def resource = Post.new
    end

    class Edit < Resty::Action
      def resource = Post.find(params["id"])
    end

    class Create < Resty::Action
      def resource
        post = Post.create(params)
        post.persisted? ? post : nil
      end
    end

    class Update < Resty::Action
      def resource
        post = Post.find(params["id"])
        return nil unless post

        post.title = params["post[title]"] || post.title
        post.body = params["post[body]"] || post.body
        post.save
        post
      end
    end

    class Destroy < Resty::Action
      def resource = Post.find(params["id"])&.destroy
    end

    ACTIONS = {
      "Show" => Show, "Index" => Index, "New" => New, "Edit" => Edit,
      "Create" => Create, "Update" => Update, "Destroy" => Destroy
    } #: Hash[String, singleton(Resty::Action)]
  end

  CONTROLLERS = {
    "posts" => PostsController::ACTIONS,
    "users" => {}
  } #: Hash[String, Hash[String, singleton(Resty::Action)]]
end

APP = Resty::App.new(ExampleApp::CONTROLLERS) #: Resty::App

# Mounts the Rack-style app on WEBrick; `service` takes every HTTP method.
class RestyServlet < WEBrick::HTTPServlet::AbstractServlet
  def service(req, res)
    status, headers, body = APP.call(Resty::Request.new(req.request_method, req.path, req.query))
    res.status = status
    headers.each { |name, value| res[name] = value }
    res.body = body
  end
end

#: () -> void
def seed
  ExampleApp::Post.delete_all
  ExampleApp::Post.create("post[id]" => "1", "post[title]" => "Title")
  ExampleApp::Post.create("post[id]" => "2", "post[title]" => "Title")
  ExampleApp::Post.create("post[id]" => "3", "post[title]" => "Title")
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
