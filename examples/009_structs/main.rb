# rbs_inline: enabled

class Animal
  attr_reader :name #: String
  attr_accessor :legs #: Integer

  #: (String, Integer) -> void
  def initialize(name, legs)
    @name = name
    @legs = legs
    @sound = "..." #: String
  end

  #: () -> String
  def speak = "#{name} says #{@sound}"

  #: () -> String
  def to_s = "#{name} (#{legs} legs)"
end

class Dog < Animal
  #: (String) -> void
  def initialize(name)
    super(name, 4)
    @sound = "woof"
    @tricks = [] #: Array[String]
  end

  #: (String) -> self
  def learn(trick)
    @tricks << trick
    self
  end

  #: () -> Integer
  def trick_count = @tricks.size

  def speak = super + "!"
end

class Bird < Animal
  #: (String) -> void
  def initialize(name) = super(name, 2)

  def speak = "#{name} tweets"
end

zoo = [Dog.new("Rex"), Bird.new("Tweety"), Animal.new("Blob", 0)]
zoo.each { |a| puts a.speak }
zoo.each { |a| puts a }

rex = Dog.new("Rex")
rex.learn("sit").learn("roll")
puts rex.trick_count
rex.legs = 3
puts rex
puts rex == rex, rex == Dog.new("Rex"), rex.equal?(rex)
puts zoo.map { |a| a.legs }.inspect
