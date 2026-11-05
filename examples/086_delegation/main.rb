# rbs_inline: enabled

require "ostruct"
require "delegate"

# OpenStruct holds fields made up on the fly; SimpleDelegator and
# DelegateClass wrap an object and pass through what they don't define.

config = OpenStruct.new(host: "localhost", port: 8080)
config.debug = true
config[:port] = 9090
puts "#{config.host}:#{config.port} debug=#{config.debug} timeout=#{config.timeout.inspect}"
puts config.inspect
puts "fields: #{config.to_h.keys.inspect}"
puts "responds to host? #{config.respond_to?(:host)}, to nope? #{config.respond_to?(:nope)}"

class Audited < SimpleDelegator
  #: (untyped) -> void
  def initialize(obj)
    super
    @calls = 0
  end

  #: () -> Integer
  def calls = @calls

  #: () -> String
  def describe
    @calls += 1
    "#{__getobj__.class.name} of size #{__getobj__.size}"
  end
end

list = Audited.new([3, 1, 2])
puts list.describe, list.sort.inspect, list.max.inspect, list.calls
puts "wraps an Array: #{list == [3, 1, 2]}, class #{list.class}"

class Account
  attr_reader :owner #: String
  attr_reader :balance #: Integer

  #: (String, Integer) -> void
  def initialize(owner, balance)
    @owner = owner
    @balance = balance
  end

  #: (Integer) -> Integer
  def deposit(amount)
    @balance += amount
  end
end

class LoggedAccount < DelegateClass(Account)
  #: (Account) -> void
  def initialize(account)
    super(account)
    @log = [] #: Array[String]
  end

  #: (Integer) -> Integer
  def deposit(amount)
    @log << "deposit #{amount}"
    __getobj__.deposit(amount)
  end

  #: () -> Array[String]
  def log = @log
end

acct = LoggedAccount.new(Account.new("Ada", 100))
acct.deposit(50)
acct.deposit(25)
puts "#{acct.owner} has #{acct.balance}; log: #{acct.log.join(", ")}"
