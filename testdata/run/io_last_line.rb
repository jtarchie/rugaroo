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
