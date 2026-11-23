# rbs_inline: enabled

require "minitest/autorun"

module ClassMethodTests
  # ActiveRecord's shape: Base's class methods build through hooks each model overrides.
  class Model
    #: (Integer) -> instance
    def self.find(id) = _find(id)

    #: (Integer) -> Model
    def self._find(id) = Model.new

    #: () -> Array[Model]
    def self._all = []

    #: () -> String
    def kind = "model"
  end

  class Post < Model
    #: (Integer) -> Post
    def self._find(id) = Post.new

    #: () -> Array[Post]
    def self._all = [Post.new, Post.new]

    #: () -> String
    def kind = "post"

    #: () -> String
    def title = "hello"
  end

  #: (Hash[String, untyped]) -> Hash[String, untyped]
  def self.copy(h)
    out = {} #: Hash[String, untyped]
    h.each do |k, v|
      if v.is_a?(Hash)
        out[k] = ClassMethodTests.copy(v)
      else
        out[k] = v
      end
    end
    out
  end

  #: (Hash[String, String]) -> Hash[String, String]
  def self.copy_strings(h)
    out = {} #: Hash[String, String]
    h.each do |k, v|
      if v.is_a?(Integer)
        out[k] = (v + 1).to_s
      else
        out[k] = v
      end
    end
    out
  end

  # Sqlx's shape: the finders arrive by `extend`, so `instance` is the extending class at each call.
  module Finder
    #: (Integer id) -> instance
    def find(id) = new(id)

    #: (Integer id) -> instance?
    def find_by(id) = id.positive? ? new(id) : nil
  end

  class Row
    extend Finder

    attr_reader :id #: Integer

    #: (Integer id) -> void
    def initialize(id)
      @id = id
    end
  end

  class Article < Row
    #: () -> String
    def title = "article #{id}"
  end

  #: (Article) -> String
  def self.article_title(article) = article.title

  class InstanceTest < Minitest::Test
    def test_instance_is_the_receivers_class
      assert_equal "hello", Post.find(1).title
      assert_equal "model", Model.find(1).kind
    end

    def test_instance_from_an_extended_module
      assert_equal "article 1", ClassMethodTests.article_title(Article.find(1))
      assert_equal "article 2", Article.find_by(2)&.title
      assert_nil Article.find_by(0)
      assert_equal 3, Row.find(3).id
    end

    def test_a_wider_array_result_override
      assert_equal 2, Post._all.length
      assert_equal %w[post post], Post._all.map(&:kind)
    end

    def test_an_impossible_is_a_branch
      assert_equal({ "a" => "1" }, ClassMethodTests.copy_strings({ "a" => "1" }))
      assert_equal({ "a" => { "b" => 1 } }, ClassMethodTests.copy({ "a" => { "b" => 1 } }))
    end
  end
end
