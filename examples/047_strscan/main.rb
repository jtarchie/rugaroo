# rbs_inline: enabled

require "strscan"
require "shellwords"

# A tokenizer for a tiny arithmetic language, then an evaluator over its tokens.
class Lexer
  #: (String) -> void
  def initialize(src)
    @ss = StringScanner.new(src)
  end

  #: () -> Array[[Symbol, String]]
  def tokens
    out = [] #: Array[[Symbol, String]]
    until @ss.eos?
      if @ss.skip(/\s+/)
        next
      elsif (t = @ss.scan(/\d+(\.\d+)?/))
        out << [:num, t]
      elsif (t = @ss.scan(/[a-z_]\w*/))
        out << [:ident, t]
      elsif (t = @ss.scan(/\*\*|[-+*\/()=]/))
        out << [:op, t]
      else
        raise ArgumentError, "unexpected #{@ss.peek(1).inspect} at #{@ss.pos}"
      end
    end
    out
  end
end

p Lexer.new("x = 3 + 4.5 * (y ** 2)").tokens
begin
  Lexer.new("1 + $").tokens
rescue ArgumentError => e
  puts e.message
end

ss = StringScanner.new("key: value; other: thing")
p ss.scan(/(\w+): (\w+)/), ss[1], ss[2], ss[0], ss.matched, ss.matched?, ss.matched_size
p ss.pos, ss.charpos, ss.rest, ss.rest_size, ss.pre_match, ss.post_match
p ss.captures
p ss.scan(/nope/), ss.matched?, ss[0]
p ss.check(/;/), ss.pos
p ss.scan_until(/other/), ss.pre_match, ss.pos
p ss.skip_until(/i/), ss.exist?(/g/), ss.match?(/n/), ss.pos
p ss.getch, ss.get_byte, ss.peek(10), ss.eos?
ss.unscan
p ss.pos
ss.pos = 5
p ss.rest, ss.bol?, ss.check_until(/;/)
ss.terminate
p ss.eos?, ss.rest, ss.scan(/x/)
ss.reset
p ss.pos, ss.string
ss.string = "héllo wörld"
p ss.scan(/h./), ss.pos, ss.charpos, ss.scan_until(/ö/), ss.charpos
p ss.inspect
ss.reset
p ss.inspect
ss.terminate
p ss.inspect
p StringScanner.new("a\nb").tap { |s| s.scan(/a\n/) }.beginning_of_line?
p StringScanner.new("abc").scan(/^b/), StringScanner.new("abc").tap { |s| s.getch }.scan(/^b/)

puts "== shellwords"
p Shellwords.split(%q{cp "my file.txt" 'other dir/' plain\ word x"y z"w})
p "a  b\tc\n".shellsplit, Shellwords.shellwords(%q{"esc \" \$ \\ \a"})
p Shellwords.escape("it's a $test"), "".shellescape, "multi\nline".shellescape, "héllo".shellescape
p ["ls", "-la", "a b"].shelljoin, Shellwords.join(["x", "y;z"])
begin
  Shellwords.split(%q{echo 'oops})
rescue ArgumentError => e
  puts e.message
end
begin
  Shellwords.split(%q{a "b})
rescue ArgumentError => e
  puts e.message
end
