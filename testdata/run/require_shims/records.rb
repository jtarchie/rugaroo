module Finders
  def find_all(n)
    (1..n).map { |i| from_row(i) }
  end

  def find_by(id)
    id.positive? ? from_row(id) : nil
  end

  def find(id)
    record = find_by(id)
    raise ArgumentError, "no record #{id}" if record.nil?

    record
  end
end

class Record
  extend Finders

  attr_reader :id

  def initialize(id)
    @id = id
  end

  def self.from_row(id)
    new(id)
  end

  def self.all
    raise NotImplementedError, "all: subclasses must override"
  end
end

class Article < Record
  def title
    "article #{id}"
  end

  def self.all
    find_all(2)
  end

  def notes
    Note.find_all(id)
  end
end

class Note < Record
  def text
    "note #{id}"
  end
end
