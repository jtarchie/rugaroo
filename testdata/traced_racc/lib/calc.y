class Calc
  prechigh
    left '*'
    left '+'
  preclow
rule
  exp: exp '+' exp { result = val[0] + val[2] }
     | exp '*' exp { result = val[0] * val[2] }
     | NUM
end

---- inner
  def parse(tokens)
    @tokens = tokens
    do_parse
  end

  def next_token = @tokens.shift
