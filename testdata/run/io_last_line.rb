# stdin: "a\nb\nc\nd\nerror one\nwarn two\nok\n"

# $_ is the last line gets read in this frame: each method has its own, its blocks share it (decision 446)
def ll_read_one
  gets
  $_
end

gets
p ll_read_one
p $_

def ll_in_block
  r = []
  2.times do
    gets
    r << $_
  end
  [r, $_]
end
p ll_in_block

# bare print prints $_; ~ /re/ matches it
while gets
  print if $_ =~ /error/
  p(~ /warn/)
end
p $_
$_ = "set\n"
print

require "English"
p $LAST_READ_LINE

def ll_untouched = $_
p ll_untouched
p(ll_untouched.nil? ? ~ /x/ : 1)

# gets sent by name sets $_; another sent method leaves it (decision 459)
require "stringio"
def ll_sent
  io = StringIO.new("x\ny\n")
  m = :gets
  io.send(m)
  r = [$_]
  io.send(:eof?)
  r << $_
  io.public_send("readline")
  r << $_
end
p ll_sent

# StringIO#print with no argument prints $_; an object's own print gets no argument (decision 459)
class LlPrinter
  def initialize = (@out = [])
  attr_reader :out

  def print(*a) = @out << a.size
end
ll_io = StringIO.new(+"example")
$_ = "blah"
ll_io.print
ll_pr = LlPrinter.new
ll_pr.print
p ll_io.string, ll_pr.out

# a thread's block has its own $_ (decision 459)
def ll_thread
  $_ = "mine\n"
  t = Thread.new { $_ = "theirs\n" }
  t.join
  $_
end
p ll_thread
