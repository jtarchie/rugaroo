# rbs_inline: enabled

require "minitest/autorun"

# Helpers for the checks that were control_and_or.rb.
#: (String?) -> bool
def blank?(s) = s.nil? || s.empty?

#: (Integer?) -> bool
def big?(n) = !n.nil? && n > 10

#: (Integer?, Integer) -> Integer
def control_pick(a, b)
  return a || b
end

#: (Integer) -> String
def control_show(n) = "n=#{n}"

#: () -> bot
def control_die = raise(ArgumentError, "died")

# Helpers for the checks that were control_assign.rb.
#: (Array[String], String, Integer?) -> Integer?
def control_trace(log, name, v)
  log << name
  v
end

#: (Integer, Integer) -> [Integer, Integer]
def control_divmod2(x, y) = [x / y, x % y]

#: () -> [String, Integer, bool]
def control_triple = ["t", 3, true]

#: (Array[String], String, bool) -> bool
def tb(log, name, v)
  log << name
  v
end

#: (Array[Integer], Integer) -> Integer
def control_note(c, v)
  c << v
  v
end

# Helpers for the checks that were control_begin_jumps.rb.
#: (Array[String], Integer) -> Integer
def nested(out, n)
  i = 0
  while i < 10
    i += 1
    begin
      case i
      when 3
        begin
          next
        ensure
          out << "inner ensure #{i}"
        end
      when 6
        begin
          break
        rescue
          out << "no"
        end
      end
      begin
        return i * 100 if i == n
      ensure
        out << "ens #{i}"
      end
    rescue => e
      out << e.message
    end
  end
  -i
end

#: (Array[String]) -> String
def ens_ret(out)
  begin
    out << "body"
  ensure
    return "from ensure"
  end
  "after"
end

#: () -> Integer
def ens_ret2
  x = 0
  [1, 2, 3].each do |v|
    begin
      x += v
    ensure
      return x if v == 2
    end
  end
  x
end

#: (Array[String], Integer) -> Integer
def tail_ret(out, n)
  begin
    return 5 if n == 1
    raise "x" if n == 2

    n * 10
  rescue
    return -1
  ensure
    out << "ens #{n}"
  end
end

#: (Array[String], Integer) -> void
def tail_void(out, n)
  out << "start"
  begin
    return if n == 1

    out << "mid #{n}"
  ensure
    out << "ens #{n}"
  end
end

# Helpers for the checks that were control_loops_nested.rb.
#: (Hash[String, Integer], Integer) -> String?
def key_for(h, v)
  h.each do |k, val|
    return k if val == v
  end
  nil
end

#: (Array[Integer]) -> Integer
def count_until_neg(xs)
  count = 0
  xs.each do |x|
    break if x < 0

    count += 1
  end
  count
end

# Helpers for the checks that were control_bugs.rb.
# a rescue binding is nil after a begin that did not raise
#: (Array[String], bool) -> void
def check(out, fail)
  begin
    raise ArgumentError, "bad" if fail
  rescue ArgumentError => err
    out << "rescued"
  end
  out << err.inspect
end

#: (Integer) -> Integer
def bugs_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

# Helpers for the checks that were control_conditionals.rb.
#: (Integer) -> String
def sign(n)
  if n > 0
    "positive"
  elsif n < 0
    "negative"
  else
    "zero"
  end
end

#: (Integer) -> String?
def only_positive(n)
  if n > 0
    "yes #{n}"
  end
end

#: (Integer) -> String?
def unless_zero(n)
  unless n.zero?
    "nonzero"
  end
end

#: (Integer) -> String
def unless_else(n)
  unless n.even?
    "odd"
  else
    "even"
  end
end

#: (Integer) -> String
def bucket(n)
  if n < 0 then "neg"
  elsif n < 10 then "digit"
  elsif n < 100 then "tens"
  elsif n < 100 then "never"
  else "big"
  end
end

#: (Integer) -> String
def grade(n)
  case n
  when 90, 100 then "A"
  when 80 then "B"
  when -1 then "negative one"
  else "F"
  end
end

#: (String) -> Symbol?
def control_color(s)
  case s
  when "red" then :warm
  when "blue", "green"
    :cool
  when ""
    :empty
  when "ünï"
    :unicode
  end
end

#: (Symbol) -> Integer
def sym_case(s)
  case s
  when :a then 1
  when :b, :c then 2
  else 0
  end
end

#: (Array[String], Integer) -> Integer
def subject(log, v)
  log << "subject"
  v
end

#: (Integer?) -> String
def nil_case(x)
  case x
  when nil then "nil"
  when 0 then "zero"
  else "other #{x.inspect}"
  end
end

#: (String) -> String
def control_kind(s)
  case s
  when /\A\d+\z/ then "digits"
  when /\A[a-z]+\z/ then "lower"
  when /é/ then "has é"
  else "other"
  end
end

#: (bool) -> String
def bool_case(b)
  case b
  when true then "T"
  when false then "F"
  else "?"
  end
end

#: (untyped) -> String
def control_what(x)
  case x
  when nil then "nil"
  when Integer then "int #{x + 1}"
  when String then "str #{x.upcase}"
  when ControlTests::Dog then "dog"
  when ControlTests::Animal then "animal"
  when Array then "array of #{x.size}"
  when Hash then "hash of #{x.size}"
  else "other"
  end
end

#: (ControlTests::Animal) -> String
def typed(a)
  case a
  when ControlTests::Dog then "D"
  when ControlTests::Cat then "C"
  else "A"
  end
end

#: (ControlTests::Animal) -> String
def shadowed(a)
  case a
  when ControlTests::Animal then "animal first"
  when ControlTests::Dog then "never"
  else "else"
  end
end

#: (bool) -> String
def yn(flag) = flag ? "yes" : "no"

#: (String?) -> String
def guarded(s)
  return "none" unless s

  "got #{s.upcase}"
end

#: (String?) -> Integer
def guarded_nil(s)
  return -1 if s.nil?

  s.size
end

# `return s unless s.nil?` narrows s for its own return (Homebrew's BottleManifest#manifest_annotations)
#: (String?) -> String
def guarded_unless_nil(s)
  return s unless s.nil?

  "none"
end

#: (Integer) -> String
def vs_const(n)
  case n
  when ControlTests::LIMIT then "at limit"
  when ControlTests::LIMIT * 2 then "double"
  else "no"
  end
end

#: (untyped) -> String
def multi(v)
  case v
  when String, Symbol then "text #{v}"
  when Integer, Float then "num #{v}"
  when nil then "nil"
  else "?"
  end
end

#: (Integer?) -> String
def nil_or_zero(v)
  case v
  when nil, 0 then "empty"
  else "val #{v}"
  end
end

# a value `when` listed before `when nil` must not call == on nil

#: (Integer?) -> String
def zero_first(v)
  case v
  when 0 then "zero"
  when nil then "nil"
  else "other"
  end
end

#: (String?) -> String
def str_first(s)
  case s
  when "a" then "A"
  else "other #{s.inspect}"
  end
end

#: (Integer) -> String
def early(n)
  return "neg" if n < 0
  return "zero" if n.zero?
  unless n > 10
    return "small"
  end

  "big"
end

#: (Integer) -> Integer
def abs_val(n) = n < 0 ? -n : n

# Helpers for the checks that were control_bug_empty_body.rb.
#: (Integer) -> Integer
def empty_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

#: (Array[Integer], Integer) -> void
def quiet(out, n)
  out << empty_risky(n)
rescue ArgumentError => e
end

# Helpers for the checks that were control_bug_iterator_return_in_ensure_begin.rb.
#: (Array[String], Integer) { (Integer) -> void } -> void
def upto_stop(out, n)
  i = 0
  while i < n
    begin
      return if i == 2
    ensure
      out << "ensure #{i}"
    end
    yield i
    i += 1
  end
  out << "loop done"
end

#: (Array[String]) { (Integer) -> void } -> void
def each_logged_ensure(out)
  i = 0
  while i < 3
    i += 1
    begin
      yield i
    ensure
      out << "after #{i}"
    end
  end
  out << "each_logged done"
end

# Helpers for the checks that were control_bug_local_name_collision.rb.
#: (Integer?) -> Integer
def collide_pick(x)
  t1 = 100
  y = x || t1
  y + t1
end

#: (Integer) -> Integer
def collide_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

#: (Integer) -> String
def doubled(n)
  ret_ = n * 2
  raise "x" if n < 0

  "v#{ret_}"
rescue
  "rescued"
end

# Helpers for the checks that were control_bug_unassigned_branch_local.rb.
#: (bool) -> Integer?
def branch_local(flag)
  if flag
    a = 1
  end
  a
end

#: (Integer) -> String?
def loop_local(n)
  i = 0
  while i < n
    last = "v#{i}"
    i += 1
  end
  last
end

#: (Array[String], Integer) -> void
def control_half(out, n)
  begin
    raise ArgumentError, "odd" if n.odd?

    h = n / 2
  rescue ArgumentError
    out << "rescue sees #{h.inspect}"
  end
  out << h.inspect
end

# Helpers for the checks that were control_definite_assign.rb.
# twice/shout reject nil: a local assigned on every path must stay non-optional.

#: (Integer) -> Integer
def twice(n)
  n * 2
end

#: (String) -> String
def shout(s)
  s.upcase
end

#: (bool) -> Integer
def both_branches(flag)
  if flag
    a = 1
  else
    a = 2
  end
  twice(a)
end

#: (Integer) -> String
def case_else(n)
  case n
  when 1 then s = "one"
  else s = "many"
  end
  shout(s)
end

#: (Array[Integer], Integer) -> void
def definite_guarded(out, n)
  if n > 0
    v = n
  else
    return
  end
  out << twice(v)
end

#: (Integer) -> Integer
def raised(n)
  if n > 0
    v = n
  else
    raise ArgumentError, "neg"
  end
  twice(v)
end

#: () -> Integer
def definite_countdown
  i = 3
  while true
    last = i
    i -= 1
    break if i == 0
  end
  twice(last)
end

#: (Array[Integer], Integer) -> void
def rescued(out, n)
  begin
    got = n
    raise ArgumentError, "big" if n > 1

    out << twice(got)
  rescue ArgumentError
    out << twice(got)
  end
end

#: (Array[String]) -> void
def ensured(out)
  begin
    out << "body"
  ensure
    e = 5
  end
  out << twice(e).to_s
end

#: (Array[String], bool) -> void
def narrowed(out, flag)
  if flag
    a = 3
    out << twice(a).to_s
    a += 1
    out << twice(a).to_s
  end
  out << a.inspect
end

# Helpers for the checks that were control_edges.rb.
#: (Integer) -> Integer
def edges_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

# method-level rescue that returns nil into a T? result, and `return` from it

#: (Integer) -> Integer?
def safe(n)
  edges_risky(n)
rescue ArgumentError
  nil
end

#: (Integer) -> Integer
def rescue_return(n)
  edges_risky(n)
rescue ArgumentError
  return -1
end

#: (Integer) -> String
def cased(n)
  begin
    case n
    when 1 then return "one"
    when 2 then raise ArgumentError, "two"
    end
    "other"
  rescue ArgumentError => e
    "err #{e.message}"
  end
end

# while as the last statement of a value method, a void method and a block

#: (Integer) -> String
def tail_while(n)
  i = 0
  while i < n
    i += 1
  end
  "done #{i}"
end

#: (Array[String]) -> void
def void_while(out)
  k = 0
  while k < 2
    out << "k#{k}"
    k += 1
  end
end

#: (Hash[String, Integer], String) -> String
def control_probe(h, k)
  if k == "x"
    "x"
  elsif (m = h[k])
    "found #{m}"
  else
    "missing"
  end
end

#: (Array[String], Integer) -> String
def control_label(out, n)
  case n
  when 1
    out << "matched one"
    "one"
  when 2, 3
    t = n * 10
    "few #{t}"
  else
    "many"
  end
end

#: () -> Array[String]
def parts = ["x", "y", "z"]

#: (Array[Integer?]) -> Integer
def total_until_nil(xs)
  sum = 0
  xs.each do |x|
    break unless x

    sum += x
  end
  sum
end

# an iterator that yields inside begin/ensure runs the ensure after each block call

#: (Array[String]) { (Integer) -> void } -> void
def edges_each_logged(out)
  i = 0
  while i < 3
    i += 1
    begin
      yield i
    ensure
      out << "after #{i}"
    end
  end
  out << "each_logged done"
end

# value `when`s on untyped and optional subjects, mixed with `when nil`

#: (untyped) -> String
def control_classify(v)
  case v
  when 1 then "one"
  when "a", :b then "a or b"
  when nil then "nil"
  when 2.5 then "float"
  else "other"
  end
end

#: (String?) -> String
def opt_str(s)
  case s
  when nil then "nil"
  when "x", "y" then "xy"
  else "other"
  end
end

# rescue that raises, with ensure in the same method

#: (Array[String]) -> Integer
def rescue_raises(out)
  raise ArgumentError, "a"
rescue ArgumentError
  raise KeyError, "b"
ensure
  out << "ens"
end

# the same name bound in two rescue clauses with different classes

#: (Integer) -> String
def two_bindings(k)
  raise ArgumentError, "arg" if k == 0
  raise KeyError, "key" if k == 1

  "none"
rescue ArgumentError => e
  "A #{e.message}"
rescue KeyError => e
  "K #{e.message}"
end

# raise in a deeply nested call inside a loop inside a method with rescue

#: (Array[Integer]) -> String
def sum_positive(xs)
  total = 0
  xs.each do |x|
    raise ArgumentError, "negative #{x}" if x < 0

    total += x
  end
  "sum #{total}"
rescue ArgumentError => e
  "failed: #{e.message} after #{total}"
end

#: (Integer) -> Integer
def depth(n)
  return depth(n - 1) + 1 if n > 0

  raise ArgumentError, "bottom"
rescue ArgumentError
  n * 1000
end

#: (Array[String], bool) -> void
def void_ensure_return(out, flag)
  out << "body"
  raise "x" if flag
ensure
  out << "ensure"
  return
end

#: (Array[String], Integer) -> String
def rescue_then_ensure_return(out, n)
  raise ArgumentError, "a" if n > 0

  "body"
rescue ArgumentError
  "rescue"
ensure
  out << "ensure #{n}"
end

# rescue clauses see locals assigned before the raise, even in a loop

#: (Array[Integer]) -> String
def progress(xs)
  done = 0
  xs.each do |x|
    raise ArgumentError, "stop at #{x}" if x < 0

    done += 1
  end
  "all #{done}"
rescue ArgumentError => e
  "#{e.message} after #{done}"
end

#: (Array[String], Array[Integer]) -> Integer
def first_ok(out, xs)
  xs.each do |x|
    begin
      raise "bad #{x}" if x < 0

      return x * 10
    rescue => e
      out << "skip #{e.message}"
    end
  end
  -1
end

#: (Integer) -> String
def case_tail(n)
  raise ArgumentError, "neg" if n < 0

  case n
  when 0 then "zero"
  else "pos"
  end
rescue ArgumentError
  "rescued"
end

#: (Integer) -> Integer
def flow_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

#: (Array[Integer]) -> Array[String]
def labels(xs)
  xs.map do |x|
    x > 0 ? "p#{x}" : raise(ArgumentError, "bad #{x}")
  end
end

#: (Array[Integer]) -> Array[String]
def names(xs)
  xs.map do |x|
    case x
    when 1 then "one"
    else raise ArgumentError, "unknown #{x}"
    end
  end
end

#: (Array[Integer]) -> Array[Integer]
def divs(xs) = xs.map { |d| 10 / d }

#: () -> Integer
def multi_in_begin
  begin
    a, b = 3, 4
  rescue
    a = 0
    b = 0
  end
  a + b
end

#: (Array[String], Integer) -> void
def boom(out, kind)
  case kind
  when 0 then raise ArgumentError, "bad arg"
  when 1 then raise "plain"
  when 2 then raise ControlTests::NotFound
  when 3 then raise ControlTests::NotFound.new("built")
  when 4 then raise KeyError, "k"
  when 5 then raise ControlTests::AppError.new
  when 6 then raise ControlTests::ValidationError.new("name", "too short")
  when 7 then raise ControlTests::DefaultMsg
  when 8 then raise IndexError, ""
  when 9 then raise TypeError, "ünïcode ✓"
  end
  out << "no raise #{kind}"
end

#: (Array[String], Integer) -> String
def which(out, k)
  boom(out, k)
  "none"
rescue ControlTests::NotFound => e
  "NotFound #{e.message}"
rescue ControlTests::AppError, KeyError => e
  "AppError|KeyError #{e.class} #{e.message}"
rescue ControlTests::ValidationError => e
  "#{e.field}: #{e.message}"
rescue StandardError => e
  "StandardError #{e.class}"
end

#: () -> String
def outer
  begin
    begin
      raise ControlTests::Fatal, "deep"
    rescue => e
      "inner caught #{e.class}"
    end
  rescue Exception => e
    "outer caught #{e.class} #{e.message}"
  end
end

#: (Array[String], bool) -> Integer
def ordered(out, fail)
  out << "body"
  raise "f" if fail

  1
rescue
  out << "rescue"
  2
ensure
  out << "ensure"
end

#: () -> String
def ensure_value
  "body value"
ensure
  "ensure value"
end

#: (Integer) -> String
def value_if(n)
  if n > 0 then "pos" else raise ArgumentError, "nonpositive" end
rescue ArgumentError => e
  "rescued #{e.message}"
end

#: () -> Integer
def ensure_overrides
  return 1
ensure
  return 2
end

#: () -> String
def swallow
  raise ArgumentError, "lost"
ensure
  return "ensure wins"
end

#: (Integer) -> String
def ensure_after_rescue(n)
  raise ArgumentError, "a" if n > 0

  "body"
rescue ArgumentError
  raise "from rescue"
ensure
  return "ensure #{n}" if n > 1
end

#: (Array[String], Array[Integer]) -> String
def scan(out, list)
  i = 0
  while i < list.size
    begin
      begin
        raise ArgumentError, "neg #{list[i]}" if (list[i] || 0) < 0
        return "found #{list[i]}" if list[i] == 7
      ensure
        out << "inner #{i}"
      end
    rescue ArgumentError => e
      return "error #{e.message}"
    end
    i += 1
  end
  "none"
end

#: (Array[String], bool) -> void
def void_ensure(out, flag)
  begin
    return if flag

    out << "not returned"
  ensure
    out << "ensure #{flag}"
  end
  out << "after"
end

#: (Array[String], Array[Integer]) -> Integer
def first_big(out, list)
  list.each do |x|
    begin
      return x if x > 10
    ensure
      out << "checked #{x}"
    end
  end
  -1
end

#: (Array[String]) { () -> void } -> void
def exc_risky(out)
  yield
rescue StandardError => e
  out << "caught #{e.message}"
end

#: (Integer) -> String
def name_of(n)
  case n
  when 1 then "one"
  else raise ArgumentError, "unknown #{n}"
  end
end

#: (Integer?) -> Integer
def must(v)
  x = v || raise(KeyError, "missing")
  x + 1
end

#: (bool) -> Integer
def tern(c)
  c ? 10 : raise("no")
end

#: (Integer) -> Integer
def risky_int(n)
  raise ArgumentError, "neg" if n < 0
  raise NotImplementedError, "nie" if n == 99

  n * 2
end

#: (String) -> Integer
def control_safe_fetch(k)
  { "a" => 1 }.fetch(k) rescue -1
end

# Helpers for the checks that were control_locals.rb.
#: (Integer) -> String
def size_word(n)
  if n > 100
    word = "large"
  elsif n > 10
    word = "medium"
  else
    word = "small"
  end
  word + "!"
end

#: (Symbol) -> Integer
def control_weight(s)
  case s
  when :a
    w = 1
  when :b
    w = 2
  else
    w = 0
  end
  w * 10
end

#: (Array[String], Integer) -> String
def parse(out, d)
  begin
    step = "start"
    q = 100 / d
    step = "divided"
  rescue ZeroDivisionError
    out << "rescue sees #{step}"
    q = -1
  ensure
    out << "ensure sees #{step}"
  end
  "#{step} #{q}"
end

#: (Integer) -> String
def nil_first(n)
  if n > 0
    label = nil
  else
    label = "non-positive"
  end
  label.inspect
end

#: (Array[Integer]) -> Array[Integer]
def running(xs)
  total = 0
  xs.map { |x| total += x }
end

#: (Integer) -> Integer
def control_bump(n)
  n += 1
  n *= 2
  n -= 3
  n
end

#: (Integer) -> String
def shadow(v)
  out = [] #: Array[String]
  [v, v + 1].each do |w|
    tmp = "#{w}!"
    out << tmp
  end
  tmp = "outer"
  out << tmp
  out.join(",")
end

# Helpers for the checks that were control_locals_scope.rb.
#: () -> String
def ens_local
  begin
    a = 1
  ensure
    cleaned = "cleaned #{a}"
  end
  cleaned
end

#: (Integer) -> String
def rescue_local(d)
  begin
    q = 10 / d
  rescue ZeroDivisionError
    note = "div by zero"
    q = 0
  end
  "#{q} #{note.inspect}"
end

#: (bool) -> Integer
def writes_only(flag)
  unused = 1
  if flag
    unused2 = "x"
  end
  3.times { |t| inner_unused = t }
  7
end

#: (String?) -> String
def default_name(name)
  name ||= "anon"
  name.upcase
end

#: (Integer?) -> Integer
def or_value(v) = (v ||= 42)

#: (Integer) -> [Integer, Integer]
def step(n) = [n / 2, n % 2]

#: (String, Integer) -> String
def kw(type, range) = "#{type}#{range}"

# Helpers for the checks that were control_loops.rb.
#: (Integer) -> Integer
def collatz(n)
  steps = 0
  while n != 1
    n = n.even? ? n / 2 : 3 * n + 1
    steps += 1
  end
  steps
end

#: (Array[String]) -> Integer?
def index_of_empty(list)
  list.each_with_index do |s, idx|
    return idx if s.empty?
  end
  nil
end

#: (Array[Array[Integer]], Integer) -> String
def find_pair(rows, target)
  rows.each_with_index do |row, r|
    row.each_with_index do |v, c|
      return "#{r},#{c}" if v == target
    end
  end
  "missing"
end

#: (Integer) -> Integer?
def control_first_square_over(limit)
  i = 1
  while true
    return i * i if i * i > limit
    return nil if i > 100

    i += 1
  end
end

#: (Array[String], Integer) -> void
def void_early(out, n)
  return if n > 1

  out << "small #{n}"
end

#: (Array[String]) { (Integer) -> void } -> void
def with_cleanup(out)
  yield 1
  yield 2
ensure
  out << "cleanup"
end

#: (Array[String], Array[Integer]) -> Integer?
def find_neg(out, list)
  with_cleanup(out) do |v|
    return v * 100 if list.include?(-v)
  end
  nil
end

#: (Integer) { (Integer) -> void } -> void
def loops_countdown(from)
  while from > 0
    yield from
    from -= 1
  end
end

#: (Array[String]) { (Integer) -> void } -> void
def loops_guarded(out)
  [1, 2, 3].each { |g| yield g }
rescue => e
  out << "guarded caught #{e.message}"
end

#: (Array[untyped]) -> String
def first_string(items)
  items.each do |item|
    case item
    when String then return "string #{item}"
    when nil then next
    end
  end
  "no string"
end

# Helpers for the checks that were control_mid.rb.
#: (Array[String]) -> Array[String]
def control_tags(words)
  out = [] #: Array[String]
  words.each do |w|
    tag = nil if w.empty?
    tag = "long" if w.size > 3
    out << "#{w}: #{tag.inspect}"
  end
  out
end

#: (Array[String], Array[String]) -> String
def last_seen(out, items)
  item = "none"
  items.each do |item|
    out << "item #{item}"
  end
  item
end

#: (Integer) -> Integer
def dbl(n) = n * 2

#: (ControlTests::MidAnimal) -> String
def control_greet(a) = "hi #{a.name}"

#: (Array[String]) { (Integer) -> void } -> void
def mid_guarded(out)
  [1, 2, 3].each { |g| yield g }
rescue => e
  out << "guarded caught #{e.message}"
end

# nil assigned after a typed assignment joins the local to T?
#: (Array[String], Integer) -> void
def joined(out, n)
  y = "a"
  y = nil if n > 5
  out << y.inspect
  if n > 0
    label = "pos"
  else
    label = nil
  end
  out << label.inspect
end

#: (Integer) -> Integer
def mid_risky(n)
  raise ArgumentError, "neg" if n < 0

  n
end

#: (Array[String]) -> Array[String]
def later_outer(out)
  labels = [1, 2].map do |z|
    item = z.to_s
    item
  end
  out << labels.inspect
  item = 5
  out << (item + 1).to_s
end

#: (Array[String], Array[Integer]) -> Array[Integer]
def halves(out, list)
  list.map { |d| d / 2 }
ensure
  out << "done"
end

# Helpers for catch/throw (issue #4): throw from a typed method, in each tail form.
#: (Integer) -> Integer
def control_pos(x)
  return x if x > 0

  throw :neg, x
end

#: (Integer) -> Integer
def control_pos_ternary(x) = x > 0 ? x : throw(:neg, x)

#: (Integer) -> Integer
def control_pos_if(x)
  if x > 0
    x
  else
    throw :neg, x
  end
end

module Kernel
  #: () -> String
  def control_tag = "<#{self.class}>"
end

# Narrowings decision 371 added: each body compiles without a NoMethodError guard (or warning) on a known non-nil local.
#: (bool) -> Integer
def control_branch_then_or(c)
  name = nil #: String?
  name = "x" if c
  return 0 if name.nil? || name.empty?

  name.size
end

#: (String?) -> bool
def control_strip_or(s)
  return false if s.nil?

  s.strip.empty? || s.strip == ":"
end

#: () -> Integer
def control_leaf_assign
  item = nil #: ControlTests::Counter?
  item = ControlTests::Counter.new
  item.n += 1
  item.n
end

# A block's `item` before the method's own is block-local, so it does not stop narrowing the later one.
#: () -> Integer
def control_block_local_first
  [1].each { |i| item = ControlTests::Counter.new; item.n += i }
  item = nil #: ControlTests::Counter?
  item = ControlTests::Counter.new
  item.n += 2 if item.n.positive?
  item.n
end

module ControlTests
  class Memo
    #: () -> void
    def initialize
      @hits = 0
    end

    #: () -> Integer
    def hits = @hits

    #: () -> String
    def value
      @value ||= compute
    end

    #: () -> Array[Integer]
    def list
      @list ||= [@hits]
    end

    #: () -> bool
    def ready
      @ready ||= @hits > 0
    end

    private

    #: () -> String
    def compute
      @hits += 1
      "v#{@hits}"
    end
  end

  class Point
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (Integer, Integer) -> void
    def initialize(x, y)
      @x, @y = x, y
    end

    #: () -> void
    def flip
      @x, @y = @y, @x
    end

    #: () -> String
    def to_s = "(#{@x}, #{@y})"
  end

  # case on classes is a type test, most specific first
  class Animal; end

  class Dog < Animal; end

  class Cat < Animal; end

  # Helpers for the checks that were control_conditions_more.rb.
  LIMIT = 3

  class EdgesAppError < StandardError; end

  module EdgesApp
    class Missing < StandardError; end
  end

  # rescue by a module the exception class includes
  module Tag; end

  class TaggedError < StandardError
    include Tag
  end

  # decision 20: attribute reads on self narrow like locals
  class Box
    attr_reader :v #: Integer?

    #: (Integer?) -> void
    def initialize(v)
      @v = v
    end

    #: () -> String
    def show
      return "empty" unless v

      "v=#{v + 1}"
    end

    #: () -> String
    def show2 = v ? "v=#{v * 2}" : "none"

    #: () -> String
    def show3
      if v && v > 3
        "big #{v}"
      else
        "small"
      end
    end
  end

  # rescue/ensure in initialize, in a class method and in a module function
  class Parser
    attr_reader :ok #: bool

    #: (String) -> void
    def initialize(s)
      @ok = true
      raise ArgumentError, "empty" if s.empty?
    rescue ArgumentError
      @ok = false
    end

    #: (Array[String], String) -> Integer
    def self.parse(out, s)
      raise ArgumentError, "bad" unless s == "1"

      1
    rescue ArgumentError => e
      out << "class method rescued #{e.message}"
      0
    ensure
      out << "class method ensure"
    end
  end

  module Util
    #: (Integer) -> String
    def self.check(n)
      return "neg" if n < 0
      raise KeyError, "zero" if n.zero?

      "pos"
    rescue KeyError => e
      "rescued #{e.message}"
    end
  end

  # rescue/ensure in inherited, overriding (super inside rescue) and mixed-in methods
  class Base
    #: () -> String
    def run = raise(NotImplementedError, "abstract run")

    #: (Array[String]) -> String
    def safe_run(out)
      run
    rescue NotImplementedError => e
      "base caught #{e.message}"
    ensure
      out << "base ensure #{self.class}"
    end
  end

  class Impl < Base
    #: () -> String
    def run
      super
    rescue NotImplementedError => e
      "impl fallback: #{e.message}"
    end
  end

  class EdgesPlain < Base; end

  module Retrying
    #: (Integer) -> String
    def attempt(n)
      raise ArgumentError, "attempt #{n} failed" if n < 2

      "attempt #{n} ok"
    rescue ArgumentError => e
      "#{worker_name}: #{e.message}"
    end
  end

  class Worker
    include Retrying

    #: () -> String
    def worker_name = "worker"
  end

  # Helpers for the checks that were control_exception_flow.rb.
  class Fixed < StandardError
    #: () -> void
    def initialize
      super("fixed message")
    end
  end

  class Blank < StandardError
    #: () -> void
    def initialize
      super()
    end
  end

  class WithCode < StandardError
    attr_reader :code #: Integer

    #: (Integer) -> void
    def initialize(code)
      @code = code
      super("code #{code}")
    end
  end

  # raise message built from interpolation and a to_s
  class Thing
    #: () -> String
    def to_s = "thing"
  end

  module FlowApp
    class Error < StandardError; end
    class Missing < Error; end

    #: (String) -> String
    def self.find(k)
      raise Missing, "no #{k}" if k.empty?

      k
    end
  end

  # an exception raised in initialize escapes new
  class Strict
    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      raise ArgumentError, "n must be positive, got #{n}" unless n > 0

      @n = n
    end
  end

  class Cache
    #: () -> void
    def initialize
      @calls = 0
    end

    #: () -> Integer
    def value
      @v ||= compute
    rescue ArgumentError
      -1
    end

    #: () -> Integer
    def compute
      @calls += 1
      raise ArgumentError, "first call fails" if @calls == 1

      @calls * 100
    end
  end

  class Holder
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(n)
      @v = begin
        flow_risky(n)
      rescue ArgumentError
        0
      end
    end
  end

  # Helpers for the checks that were control_exceptions.rb.
  class AppError < StandardError; end

  class NotFound < AppError; end

  class Fatal < Exception; end

  class ValidationError < StandardError
    attr_reader :field #: String

    #: (String, String) -> void
    def initialize(field, msg)
      super(msg)
      @field = field
    end
  end

  class DefaultMsg < StandardError
    #: (?String) -> void
    def initialize(msg = "default message")
      super(msg)
    end
  end

  # swap through ivars and locals mixed
  class Pair
    attr_reader :l #: Integer
    attr_reader :r #: Integer

    #: (Integer, Integer) -> void
    def initialize(l, r)
      @l = l
      @r = r
    end

    #: () -> [Integer, Integer]
    def swapped
      lo, hi = @r, @l
      [lo, hi]
    end
  end

  # an overridden message or to_s drives what inspect and rescue see
  class CustomMessage < StandardError
    #: () -> String
    def message = "custom"
  end

  class CustomToS < StandardError
    #: () -> String
    def to_s = "tos"
  end

  class MidAnimal
    #: () -> String
    def name = "animal"
  end

  # zsuper in an exception initialize forwards the (defaulted) message
  class MidPlain < StandardError
    #: (String) -> void
    def initialize(msg)
      super
    end
  end

  class MidDefaultMsg < StandardError
    #: (?String) -> void
    def initialize(msg = "default message")
      super
    end
  end

  class Counter
    attr_accessor :n #: Integer

    def initialize = @n = 1
  end

  class ControlAndOrTest < Minitest::Test
    def test_narrowing_between_operands
      assert_equal [1, 0], [control_branch_then_or(true), control_branch_then_or(false)]
      assert_equal [true, false, false], [control_strip_or(" : "), control_strip_or("ab"), control_strip_or(nil)]
      assert_equal [2, 3], [control_leaf_assign, control_block_local_first]
    end

    def test_optional_and_untyped_operands
      assert_equal true, blank?(nil)
      assert_equal true, blank?("")
      assert_equal false, blank?("x")
      assert_equal false, big?(nil)
      assert_equal false, big?(5)
      assert_equal true, big?(50)
      assert_equal 2, (control_pick(nil, 2))
      assert_equal 1, (control_pick(1, 2))
      flag = true
      opt = nil #: Integer?
      mixed = flag && opt
      assert_nil mixed
      mixed2 = !flag || opt
      assert_nil mixed2
      values = [opt || 1, (flag && 2) || 3] #: Array[Integer]
      assert_equal [1, 2], values
      assert_equal "n=5", (control_show(opt || 5))
      u = nil #: untyped
      w = 3 #: untyped
      assert_equal "dflt", (u || "dflt")
      assert_equal 3, (w || "dflt")
      assert_nil (u && "x")
      assert_equal "x", (w && "x")
      name = nil #: String?
      greeting = name && "hi #{name}"
      assert_nil greeting
      name = "zed"
      greeting = name && "hi #{name}"
      assert_equal "hi zed", greeting

      # assignment inside && in a condition
      store = { "k" => 5 } #: Hash[String, Integer]
      seen = "" #: String
      if flag && (got = store["k"])
        seen = "got #{got}"
      end
      assert_equal "got 5", seen

      # not / ! on optional and untyped values
      assert_equal true, (not opt)
      assert_equal true, (!u)
      assert_equal false, (!w)
    end

    # && binding tighter than ||, and `and`/`or` equal precedence left to right
    def test_binding_tighter_than_and_and
      t = true
      f = false
      assert_equal true, (t || f && f)
      r = (f or t and f)
      assert_equal false, r
    end

    # || on an optional with a side-effecting right side, only when nil
    def test_or_on_optional_side_effect_only_when_nil
      hits = 0
      vals = [] #: Array[Integer]
      3.times do |i|
        cached = (i == 1 ? 7 : nil) #: Integer?
        val = cached || (hits += 1)
        vals << val
      end
      assert_equal [1, 7, 2], vals
      assert_equal 2, hits
    end

    # multiple assignment from Arrays, untyped values and optionals
    def test_multiple_assignment_from_arrays_untyped
      kname, kval = "a=1".split("=")
      assert_equal "a", kname
      assert_equal "1", kval
      k2, v2 = "novalue".split("=")
      assert_equal "novalue", k2
      assert_nil v2
      un = [1, "two"] #: Array[untyped]
      u1, u2, u3 = un
      assert_equal 1, u1
      assert_equal "two", u2
      assert_nil u3
      o1 = 1 #: Integer?
      o2 = nil #: Integer?
      o1, o2 = o2, o1
      assert_nil o1
      assert_equal 1, o2
      h = { "x" => 1, "y" => 2 } #: Hash[String, Integer]
      got = [] #: Array[String]
      h.each do |pair|
        kk, vv = pair
        got << "#{kk}:#{vv}"
      end
      h.to_a.each do |kk, vv|
        got << kk + vv.to_s
      end
      assert_equal ["x:1", "y:2", "x1", "y2"], got
      ix = 0
      while ix < 3
        ix, dummy = ix + 1, ix
      end
      assert_equal 3, ix
    end

    # multiple assignment inside a closure block writes outer locals
    def test_multiple_assignment_inside_a_closure
      lo = 0
      hi = 0
      pairs = [[1, 9], [2, 8]] #: Array[[Integer, Integer]]
      pairs.each { |pair| lo, hi = pair[1], pair[0] }
      assert_equal 8, lo
      assert_equal 2, hi
      ww = 0
      res = [5].map { |x| ww, z = x, x * 2; z }
      assert_equal 5, ww
      assert_equal [10], res
    end
  end

  class ControlAssignTest < Minitest::Test
    # An untyped value destructures as MRI's implicit to_ary: an Array
    # splits, anything else is the first target (#79's Dir fixture).
    def test_destructure_untyped
      pair = [1, "a"] #: untyped
      a, b = pair
      c, *d = pair
      one = 7 #: untyped
      e, f = one
      assert_equal [1, "a", 1, ["a"], 7, nil], [a, b, c, d, e, f]
      rows = [] #: Array[untyped]
      rows << ["k", "v"] << 3 << nil
      seen = rows.map { |k, v| [k, v] }
      assert_equal [["k", "v"], [3, nil], [nil, nil]], seen
      assert_equal [["k", ["v"]], [3, []]], rows.first(2).map { |k, *v| [k, v] }
      nested = [] #: Array[untyped]
      nested << [[1, 2], 3]
      assert_equal [[1, 2, 3]], nested.map { |(x, y), z| [x, y, z] }
      assert_equal ["k", 3, nil], rows.map { |k,| k }
    end

    # `x ||= v` after `x = ...` on some paths only: x is nil on the others, so ||= tests it (decision 248)
    def test_or_assign_after_branch_assignment
      assert_equal [["a"], ["z"], [["l"], 1], [[], 0]], [control_maybe_set(true), control_maybe_set(false), control_maybe_pair(true), control_maybe_pair(false)]
    end

    #: (bool) -> Array[String]
    def control_maybe_set(c)
      x = ["a"] if c
      x ||= ["z"]
      x
    end

    #: (bool) -> [Array[String], Integer]
    def control_maybe_pair(c)
      libs, n = [["l"], 1] if c
      libs ||= []
      n ||= 0
      [libs, n]
    end

    # `return if (data = build).nil?` narrows data after it, as `return if data.nil?` does (decision 249)
    def test_guard_on_parenthesized_assignment
      assert_equal [{ "v" => 2, "a" => 1 }, nil], [control_guard_write(true), control_guard_write(false)]
    end

    #: (bool) -> Hash[String, Integer]?
    def control_guard_build(ok) = ok ? { "a" => 1 } : nil

    #: (bool) -> Hash[String, Integer]?
    def control_guard_write(ok)
      return if (data = control_guard_build(ok)).nil?

      { "v" => 2, **data }
    end

    # `|a,|` splats a yielded Array (its first element), unlike `|a|`.
    def test_trailing_comma_block_param
      pairs = [["a", true], ["b", false]] #: Array[[String, bool]]
      pairs.reject! { |name,| name == "a" }
      assert_equal [["b", false]], pairs
      assert_equal [1, 3], [[1, 2], [3, 4]].map { |x,| x }
      assert_equal ["k", "j"], { "k" => 1, "j" => 2 }.map { |k,| k }
      assert_equal [10, 20], [1, 2].map { |x,| x * 10 }
    end

    def test_or_assign_multiple_assign_and_short_circuit
      calls = [] #: Array[String]
      a = nil #: Integer?
      a ||= control_trace(calls, "first", 1)
      a ||= control_trace(calls, "second", 2)
      assert_equal 1, a
      assert_equal ["first"], calls
      fresh ||= "made"
      assert_equal "made", fresh
      n = 0
      n ||= control_trace(calls, "never", 9) || 0
      assert_equal 0, n
      assert_equal ["first"], calls
      flag = false
      flag ||= true
      assert_equal true, flag
      t = true
      t ||= false
      assert_equal true, t
      untyped_vals = [false, nil, 0, "", [], :s] #: Array[untyped]
      filled = [] #: Array[String]
      untyped_vals.each do |u|
        u ||= "was falsy"
        filled << u.inspect
      end
      assert_equal ["\"was falsy\"", "\"was falsy\"", "0", "\"\"", "[]", ":s"], filled
      m = nil #: String?
      m ||= nil
      assert_nil m
      m ||= "later"
      assert_equal "later", m
      m ||= "ignored"
      assert_equal "later", m
      memo = Memo.new
      assert_equal false, memo.ready
      assert_equal "v1", memo.value
      assert_equal "v1", memo.value
      assert_equal 1, memo.hits
      assert_equal [1], memo.list
      assert_equal true, memo.ready
      cache = nil #: Array[Integer]?
      3.times do |i|
        cache ||= []
        cache << i
      end
      assert_equal [0, 1, 2], cache
      q, r = control_divmod2(17, 5)
      assert_equal 3, q
      assert_equal 2, r
      q, r = control_divmod2(-7, 2)
      assert_equal(-4, q)
      assert_equal 1, r
      s, i, b = control_triple
      assert_equal "t", s
      assert_equal 3, i
      assert_equal true, b
      a1, b1 = 1, 2
      a1, b1 = b1, a1
      assert_equal 2, a1
      assert_equal 1, b1
      x, y, z = "x", "y", "z"
      x, y, z = y, z, x
      assert_equal ["y", "z", "x"], ([x, y, z])
      f0, f1 = 0, 1
      10.times { f0, f1 = f1, f0 + f1 }
      assert_equal 55, f0
      assert_equal 89, f1
      arr = [10, 20] #: Array[Integer]
      p1, p2, p3 = arr
      assert_equal 10, p1
      assert_equal 20, p2
      assert_nil p3
      e1, e2 = [] #: Array[String]
      assert_nil e1
      assert_nil e2
      one, two = [5] #: Array[Integer]
      assert_equal 5, one
      assert_nil two
      h1, h2 = 1, 2, 3
      assert_equal 1, h1
      assert_equal 2, h2
      mm = 0
      nn = 0
      mm, nn = 3, 4
      assert_equal 7, (mm + nn)
      pt = Point.new(1, 2)
      pt.flip
      assert_equal "(2, 1)", pt.to_s
      pairs = [[1, "one"], [2, "two"]] #: Array[[Integer, String]]
      ranges = [] #: Array[String]
      pairs.each do |num, word|
        lo, hi = num, num * 100
        ranges << "#{word}: #{lo}..#{hi}"
      end
      assert_equal ["one: 1..100", "two: 2..200"], ranges
      maybe = "set" #: String?
      other = 1
      maybe, other = nil, 2
      assert_nil maybe
      assert_equal 2, other
      log = [] #: Array[String]
      r1 = tb(log, "a", false) && tb(log, "b", true)
      r2 = tb(log, "c", true) || tb(log, "d", true)
      r3 = tb(log, "e", true) && tb(log, "f", false)
      r4 = tb(log, "g", false) || tb(log, "h", false)
      assert_equal [false, true, false, false], ([r1, r2, r3, r4])
      assert_equal ["a", "c", "e", "f", "g", "h"], log
      log.clear
      r5 = (tb(log, "i", true) and tb(log, "j", true))
      r6 = (tb(log, "k", false) or tb(log, "l", true))
      r7 = (not tb(log, "m", true))
      assert_equal [true, true, false], ([r5, r6, r7])
      assert_equal ["i", "j", "k", "l", "m"], log
      none = nil #: Integer?
      some = 3 #: Integer?
      assert_equal 10, (none || 10)
      assert_equal 3, (some || 10)
      assert_nil (none && none + 1)
      assert_equal 4, (some && some + 1)
      str = nil #: String?
      assert_equal "", (str || "")
      assert_nil (str && str.size)
      str = "abc"
      assert_equal 3, (str && str.size)
      other_nil = nil #: Integer?
      assert_nil (none || other_nil)
      yes = true
      no = false
      assert_equal "fallback", (no || "fallback")
      assert_equal "yes", (yes && "yes")
      assert_equal false, (no && "never")
      assert_equal true, (yes || "never")
      assert_equal false, (none || false)
      assert_equal "three", (some && "three")
      assert_nil (none && "none")
      zero_and = 0 && "zero is truthy"
      assert_equal "zero is truthy", zero_and
      empty_and = "" && :empty_is_truthy
      assert_equal :empty_is_truthy, empty_and
      xn = nil #: Integer?
      yn = nil #: Integer?
      zn = 7 #: Integer?
      assert_equal 7, (xn || yn || zn)
      assert_equal "big", (zn && zn > 5 && "big")
      assert_equal 0, (xn || yn || 0)
      counter = [] #: Array[Integer]
      one_opt = 1 #: Integer?
      got = one_opt || control_note(counter, 2)
      got2 = none || control_note(counter, 3)
      got3 = none && control_note(counter, 4)
      got4 = one_opt && control_note(counter, 5)
      assert_equal 1, got
      assert_equal 3, got2
      assert_nil got3
      assert_equal 5, got4
      assert_equal [3, 5], counter
      vals = [nil, false, 0, "s"] #: Array[untyped]
      rows = [] #: Array[String]
      vals.each do |v|
        rows << (v || "right").inspect + " " + (v && "right").inspect
      end
      assert_equal ["\"right\" nil", "\"right\" false", "0 \"right\"", "\"s\" \"right\""], rows
      assert_equal true, (!none)
      assert_equal false, (!some)
      assert_equal true, (!!some)
      assert_equal false, (!yes)
      assert_equal false, (!!no)
      h = { "a" => 1 } #: Hash[String, Integer]
      v = h["a"] or raise "missing"
      assert_equal 1, v
      e = assert_raises(KeyError) do
        w = h["b"] or raise KeyError, "missing b"
        flunk "got #{w.inspect}"
      end
      assert_equal "missing b", e.message
      ok = h.key?("a")
      got_unless = "" #: String
      got_unless = "unless-and" unless ok && h.empty?
      assert_equal "unless-and", got_unless
    end
  end

  class ControlBeginJumpsTest < Minitest::Test
    def test_next_break_return_through_nested_begin
      out = [] #: Array[String]
      assert_equal 400, nested(out, 4)
      assert_equal ["ens 1", "ens 2", "inner ensure 3", "ens 4"], out
      out = [] #: Array[String]
      assert_equal(-6, nested(out, 8))
      assert_equal ["ens 1", "ens 2", "inner ensure 3", "ens 4", "ens 5"], out
    end

    def test_return_from_ensure
      out = [] #: Array[String]
      assert_equal "from ensure", ens_ret(out)
      assert_equal ["body"], out
      assert_equal 3, ens_ret2
    end

    def test_next_and_break_in_nested_blocks_with_ensure
      out = [] #: Array[String]
      [1, 2, 3].each do |v|
        begin
          [10, 20].each do |w|
            begin
              next if w == 10
              break if v == 2

              out << "#{v} #{w}"
            ensure
              out << "e #{v} #{w}"
            end
          end
        ensure
          out << "outer e #{v}"
        end
      end
      assert_equal ["e 1 10", "1 20", "e 1 20", "outer e 1", "e 2 10", "e 2 20", "outer e 2", "e 3 10", "3 20", "e 3 20", "outer e 3"], out
    end

    def test_next_and_break_in_ensure_swallow_the_raise
      out = [] #: Array[String]
      i = 0
      while i < 4
        i += 1
        begin
          raise "boom #{i}" if i.even?

          out << "body #{i}"
        ensure
          next if i == 2
          break if i == 4
        end
        out << "after #{i}"
      end
      assert_equal ["body 1", "after 1", "body 3", "after 3"], out
      assert_equal 4, i

      xs = [] #: Array[String]
      [1, 2, 3].each do |x|
        begin
          raise "a" if x == 2
        ensure
          next if x == 2
        end
        xs << "x #{x}"
      end
      assert_equal ["x 1", "x 3"], xs
    end

    def test_tail_begin_with_return
      out = [] #: Array[String]
      assert_equal [5, -1, 30], [tail_ret(out, 1), tail_ret(out, 2), tail_ret(out, 3)]
      assert_equal ["ens 1", "ens 2", "ens 3"], out
      out = [] #: Array[String]
      tail_void(out, 1)
      tail_void(out, 2)
      assert_equal ["start", "ens 1", "start", "mid 2", "ens 2"], out
    end
  end

  class ControlLoopsNestedTest < Minitest::Test
    def test_while_and_until_conditions
      arr = [3, 8, 0, 5] #: Array[Integer]
      i = 0
      while i < arr.size && arr[i] != 0
        i += 1
      end
      assert_equal 2, i

      opt = [1, 2] #: Array[Integer]
      j = 0
      until opt[j].nil?
        j += 1
      end
      assert_equal 2, j
    end

    # break from an if/else arm leaves only the inner iterator
    def test_break_from_if_else_arm_leaves_inner_iterator
      found = [] #: Array[String]
      [1, 2, 3].each do |a|
        [10, 20, 30].each do |b|
          if b > 10 * a
            break
          else
            found << "#{a}-#{b}"
          end
        end
      end
      assert_equal ["1-10", "2-10", "2-20", "3-10", "3-20", "3-30"], found
    end

    def test_next_and_break_in_while
      k = 0
      odds = [] #: Array[Integer]
      while k < 6
        k += 1
        if k.even?
          if k == 4
            next
          end

          odds << -k
          next
        end
        odds << k
      end
      assert_equal [1, -2, 3, 5, -6], odds

      w = 0
      while true
        break
      end
      assert_equal 0, w

      n = 0
      total = 0
      (total += n; n += 1) while n < 4
      assert_equal 6, total
    end

    def test_next_break_return_in_blocks
      res = [] #: Array[String]
      %w[a b c].each_with_index do |s, idx|
        2.times do |t|
          next if t == idx

          res << "#{s}#{t}"
        end
      end
      assert_equal ["a1", "b0", "c0", "c1"], res

      hh = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      assert_equal "b", key_for(hh, 2)
      assert_nil key_for(hh, 3)
      assert_equal 2, count_until_neg([1, 2, -1, 4])
      assert_equal 0, count_until_neg([])

      acc = [] #: Array[Integer]
      10.downto(1) do |d|
        next if d.odd?
        break if d < 4

        acc << d
      end
      assert_equal [10, 8, 6, 4], acc

      evens = [] #: Array[String]
      [1, 2, 3, 4].select(&:even?).each do |e|
        evens << "even #{e}"
        break
      end
      assert_equal ["even 2"], evens

      chars = [] #: Array[String]
      "abcd".each_char do |ch|
        next if ch == "b"

        chars << ch
      end
      assert_equal "acd", chars.join
    end

    # break and next inside a while stay in the while when it sits in a closure block
    def test_while_inside_closure_block
      sums = [3, 5].map do |lim|
        sum = 0
        m = 0
        while true
          m += 1
          next if m == 2
          break if m > lim

          sum += m
        end
        sum
      end
      assert_equal [4, 13], sums
    end
  end

  class ControlBugsTest < Minitest::Test
    # block-local params after `;` shadow the outer local instead of assigning it
    def test_block_local_params_shadow_outer
      x_semi = 5
      [1, 2].each { |v; x_semi| x_semi = v * 100 }
      assert_equal 5, x_semi
      total_semi = 0
      sums_semi = [3, 4].map do |v; total_semi|
        total_semi = v + 1
        total_semi
      end
      assert_equal [4, 5], sums_semi
      assert_equal 0, total_semi
    end

    # literal true/false/nil on the left of &&/|| still yields the right value
    def test_literal_left_of_and_or
      assert_equal false, (false && "never")
      assert_equal true, (true || "never")
      assert_equal "yes", (true && "yes")
      assert_equal "fallback", (false || "fallback")
      x_nil = nil && 5
      assert_nil x_nil
      s_nil = "a"
      y_nil = nil && s_nil
      assert_nil y_nil
      assert_equal false, (nil || false)
      assert_equal "right", (nil || "right")
    end

    # case on a literal subject, with and without an else
    def test_case_on_literal_subject
      v_case = case 3
               when 1 then "one"
               when 3 then "three"
               end
      assert_equal "three", v_case
      w_case = case "s"
               when "s" then :matched
               else :no
               end
      assert_equal :matched, w_case
    end

    # a condition's right side lifted out of &&/|| keeps short-circuiting
    def test_lifted_condition_keeps_short_circuit
      out = [] #: Array[String]
      h_cond = { "a" => 1, "b" => 5 } #: Hash[String, Integer]
      if h_cond["c"].nil? && (h_cond["b"] || 0) > 1
        out << "lifted and"
      end
      if h_cond["a"].nil? || (h_cond["b"] || 0) > 1
        out << "lifted or"
      end
      ok_cond = true
      unless ok_cond && (h_cond["zz"] || 0) > 1
        out << "unless lifted and"
      end
      assert_equal ["lifted and", "lifted or", "unless lifted and"], out
    end

    # Exception#inspect with an empty message shows the class name; control characters are escaped
    def test_exception_inspect
      assert_equal "ArgumentError", ArgumentError.new("").inspect
      e_empty = assert_raises(KeyError) { raise KeyError, "" }
      assert_equal "KeyError", e_empty.inspect
      assert_equal "", e_empty.message
      assert_equal "#<RuntimeError:\"a\\nb\">", RuntimeError.new("a\nb").inspect
      assert_equal "#<ArgumentError: tab\there>", ArgumentError.new("tab\there").inspect
    end

    # a bare `next` in a value block yields nil for that element
    def test_bare_next_in_value_block
      xs_next = [1, 2, 3] #: Array[Integer]
      ys_next = xs_next.map do |x|
        next if x == 2

        x * 10
      end
      assert_equal "[10, nil, 30]", ys_next.inspect
      picked_next = xs_next.select do |x|
        next if x.odd?

        true
      end
      assert_equal [2], picked_next
    end

    # ||= with a raising right side only raises when the left is nil
    def test_or_assign_with_raising_right_side
      y_raise = 5 #: Integer?
      y_raise ||= raise(ArgumentError, "never")
      assert_equal 5, y_raise
      x_raise = nil #: Integer?
      e = assert_raises(KeyError) do
        x_raise ||= raise(KeyError, "x missing")
        flunk "unreachable #{x_raise}"
      end
      assert_equal "x missing", e.message
    end

    def test_rescue_binding_nil_without_raise
      out = [] #: Array[String]
      check(out, true)
      check(out, false)
      assert_equal ["rescued", "#<ArgumentError: bad>", "nil"], out
    end

    # an untyped local takes new literal types and ||= replaces false
    def test_untyped_local_retyped_and_or_assign
      v_untyped = 1 #: untyped
      assert_equal 1, v_untyped
      v_untyped = "s"
      assert_equal "s", v_untyped
      u_untyped = false #: untyped
      u_untyped ||= "was false"
      assert_equal "was false", u_untyped
      # ||= on an untyped hash read fills false and missing alike
      h_fill = { "f" => false, "s" => "set" } #: Hash[String, untyped]
      filled = [] #: Array[String]
      %w[f s missing].each do |k|
        v = h_fill[k]
        v ||= "filled"
        filled << v.inspect
      end
      assert_equal ["\"filled\"", "\"set\"", "\"filled\""], filled
    end

    # truthiness of untyped values read from a hash or array
    def test_truthiness_of_untyped_reads
      out = [] #: Array[String]
      h_truthy = { "t" => true, "f" => false, "n" => nil, "z" => 0 } #: Hash[String, untyped]
      %w[t f n z missing].each do |k|
        out << "#{k}: #{h_truthy[k] ? "truthy" : "falsy"}"
        out << "  if" if h_truthy[k]
        v = h_truthy[k]
        out << "  local if" if v
        out << "  nil? #{h_truthy[k].nil?}"
        out << "  && #{(h_truthy[k] && "rhs").inspect}"
      end
      arr_truthy = [false, nil, 1] #: Array[untyped]
      out << "arr[0] truthy" if arr_truthy[0]
      out << "arr[1] truthy" if arr_truthy[1]
      out << "arr[2] truthy" if arr_truthy[2]
      assert_equal [
        "t: truthy", "  if", "  local if", "  nil? false", "  && \"rhs\"",
        "f: falsy", "  nil? false", "  && false",
        "n: falsy", "  nil? true", "  && nil",
        "z: truthy", "  if", "  local if", "  nil? false", "  && \"rhs\"",
        "missing: falsy", "  nil? true", "  && nil",
        "arr[2] truthy",
      ], out
    end

    # value types (String, "", Symbol) are always truthy
    def test_value_types_always_truthy
      out = [] #: Array[String]
      s_truthy = "x"
      out << "string is truthy" if s_truthy
      e_truthy = ""
      out << (e_truthy ? "empty string is truthy" : "falsy")
      sym_truthy = :a
      out << "symbol is truthy" if sym_truthy
      assert_equal ["string is truthy", "empty string is truthy", "symbol is truthy"], out
    end

    # next with a value gives the block's value
    def test_next_with_value
      nv_h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      assert_equal "{\"a\" => 1}", nv_h.select { |k, v| next false if k == "b"; v > 0 }.inspect
      nv_xs = [1, 2] #: Array[Integer]
      assert_equal [2, 4], nv_xs.map { |x| next x * 2 }
      assert_equal [1], nv_xs.select { |n| next false if n > 1; true }
    end
  end

  class ControlConditionalsTest < Minitest::Test
    def test_if_unless_ternary_and_case
      assert_equal "positive", sign(5)
      assert_equal "negative", sign(-3)
      assert_equal "zero", sign(0)
      assert_equal "yes 1", only_positive(1)
      assert_nil only_positive(0)
      assert_nil only_positive(-1)
      assert_nil unless_zero(0)
      assert_equal "nonzero", unless_zero(-2)
      assert_equal "odd", unless_else(3)
      assert_equal "even", unless_else(0)
      assert_equal "even", unless_else(-4)
      assert_equal ["neg", "digit", "digit", "tens", "tens", "big", "big"], ([-1, 0, 9, 10, 99, 100, 1_000_000].map { |n| bucket(n) })
      x = 3
      assert_equal "big", (x > 2 ? "big" : "small")
      assert_equal 2, (x.even? ? 1 : 2)
      z = if x == 3 then 30 else 0 end
      assert_equal 30, z
      w = unless x == 3 then 1 else 2 end
      assert_equal 2, w
      v = x > 100 ? nil : x * 2
      assert_equal 6, v
      v2 = x < 100 ? nil : x * 2
      assert_nil v2
      nested = x > 1 ? (x > 2 ? "gt2" : "eq2") : "le1"
      assert_equal "gt2", nested
      assert_equal "arg if", (if x > 1 then "arg if" else "arg else" end)
      s = "a" + (x.odd? ? "odd" : "even") + "z"
      assert_equal "aoddz", s
      mods = [] #: Array[String]
      mods << "mod-if" if x == 3
      mods << "mod-if never" if x == 4
      mods << "mod-unless" unless x == 4
      mods << "mod-unless never" unless x == 3
      assert_equal ["mod-if", "mod-unless"], mods
      # Ruby 4: a line starting with && / || continues the condition
      teen = x >= 13
        || x == 3
      assert_equal true, teen
      assert_equal ["A", "A", "B", "negative one", "F", "F"], ([90, 100, 80, -1, 0, 85].map { |n| grade(n) })
      assert_equal :warm, control_color("red")
      assert_equal :cool, control_color("green")
      assert_equal :empty, control_color("")
      assert_equal :unicode, control_color("ünï")
      assert_nil control_color("RED")
      assert_equal 1, sym_case(:a)
      assert_equal 2, sym_case(:c)
      assert_equal 0, sym_case(:zz)
      log = [] #: Array[String]
      lim = 3
      r = case subject(log, 4)
          when lim then "lim"
          when lim + 1 then "lim+1"
          else "other"
          end
      assert_equal "lim+1", r
      assert_equal ["subject"], log
      assert_equal "nil", nil_case(nil)
      assert_equal "zero", nil_case(0)
      assert_equal "other 5", nil_case(5)
      assert_equal "digits", control_kind("123")
      assert_equal "lower", control_kind("abc")
      assert_equal "has é", control_kind("café")
      assert_equal "other", control_kind("A1")
      assert_equal "other", control_kind("")
      assert_equal "T", bool_case(true)
      assert_equal "F", bool_case(false)
      assert_equal "nil", control_what(nil)
      assert_equal "int 42", control_what(41)
      assert_equal "str HI", control_what("hi")
      assert_equal "dog", control_what(Dog.new)
      assert_equal "animal", control_what(Cat.new)
      assert_equal "array of 2", (control_what([1, 2]))
      assert_equal "hash of 1", (control_what({ "a" => 1 }))
      assert_equal "other", control_what(2.5)
      assert_equal "other", control_what(:sym)
      assert_equal "other", control_what(false)
      assert_equal "D", typed(Dog.new)
      assert_equal "C", typed(Cat.new)
      assert_equal "A", typed(Animal.new)
      assert_equal "animal first", shadowed(Dog.new)
    end

    # case without a match and without else is nil; untyped truthiness
    def test_case_without_match_and_truthiness
      three = 3
      none = case three
             when 1 then "x"
             end
      assert_nil none
      assert_equal "three", (case three when 3 then "three" else "other" end)
      assert_equal "n=III", ("n=#{case three when 3 then "III" else "?" end}")
      vals = [nil, false, 0, "", [], true, 0.0] #: Array[untyped]
      rows = vals.map do |val|
        word = val ? "T" : "F"
        word2 = "F"
        word2 = "T" if val
        word3 = "T"
        word3 = "F" unless val
        "#{val.inspect} #{word} #{word2} #{word3} #{yn(val)} #{(!val).inspect}"
      end
      assert_equal ["nil F F F no true", "false F F F no true", "0 T T T yes false", "\"\" T T T yes false", "[] T T T yes false", "true T T T yes false", "0.0 T T T yes false"], rows
    end

    # narrowing: `if x`, `x && ...`, ternary, and early-exit guards
    def test_narrowing_guards
      maybe = 5 #: Integer?
      nothing = nil #: Integer?
      got = [] #: Array[Integer]
      got << maybe + 1 if maybe
      got << nothing + 1 if nothing
      assert_equal [6], got
      assert_equal 10, (maybe ? maybe * 2 : 0)
      assert_equal 0, (nothing ? nothing * 2 : 0)
      msg = "" #: String
      if maybe && maybe > 4
        msg = "maybe #{maybe} > 4"
      end
      assert_equal "maybe 5 > 4", msg
      assert_equal "none", guarded(nil)
      assert_equal "got OK", guarded("ok")
      assert_equal(-1, guarded_nil(nil))
      assert_equal 4, guarded_nil("four")
      assert_equal %w[four none], [guarded_unless_nil("four"), guarded_unless_nil(nil)]
    end
  end

  class ControlConditionsMoreTest < Minitest::Test
    # an assignment as the condition: the value decides, the local stays set
    def test_assignment_as_condition
      h = { "a" => 1, "b" => 2 } #: Hash[String, Integer]
      got = [] #: Array[String]
      if (m = h["a"])
        got << "found #{m + 1}"
      end
      if (m2 = h["zz"])
        got << "never #{m2}"
      else
        got << "missing #{m2.inspect}"
      end
      assert_equal ["found 2", "missing nil"], got
    end

    # nested ternaries without parentheses associate to the right
    def test_nested_ternaries_associate_right
      x = 4
      assert_equal "b", (x > 5 ? "a" : x > 3 ? "b" : "c")
      assert_equal "c", (x > 5 ? "a" : x > 4 ? "b" : "c")
      assert_equal "even big", (x.even? && x > 2 ? "even big" : "other")
    end

    # `when` with constant values and expressions on constants is ==, not a type test
    def test_when_with_constants_and_several_classes
      assert_equal "at limit", vs_const(3)
      assert_equal "double", vs_const(6)
      assert_equal "no", vs_const(1)
      assert_equal ["text s", "text y", "num 2", "num 2.5", "nil", "?"], [multi("s"), multi(:y), multi(2), multi(2.5), multi(nil), multi([1])]
      assert_equal ["empty", "empty", "val 3"], [nil_or_zero(nil), nil_or_zero(0), nil_or_zero(3)]
      assert_equal ["zero", "other", "nil"], [zero_first(0), zero_first(5), zero_first(nil)]
      assert_equal "A", str_first("a")
      assert_equal "other nil", str_first(nil)
    end

    # case and if as the value of a block
    def test_case_and_if_as_block_value
      words = [1, 5, 12].map do |n|
        case n
        when 1 then "one"
        when 5 then "five"
        else "many"
        end
      end
      assert_equal ["one", "five", "many"], words
      labels = [1, 5].map { |n| if n > 2 then "big" end }
      assert_equal [nil, "big"], labels
    end

    # and/or/not keywords and their precedence
    def test_and_or_not_keywords
      a = true
      b = false
      got = [] #: Array[String]
      got << "and-not" if a and not b
      got << "unless-or" unless a || b
      got << "not-or" if !(a || b)
      got << "neither" unless a or b
      assert_equal ["and-not"], got
      assert_equal true, (a || b && b)
      r = (b or a and b)
      assert_equal false, r
      y = z = 5
      assert_equal 10, y + z
    end

    def test_early_returns_and_modifiers
      assert_equal ["neg", "zero", "small", "big"], [early(-1), early(0), early(5), early(50)]
      assert_equal 4, abs_val(-4)
      assert_equal 4, abs_val(4)
      c = 0
      c += 1 if true
      c += 10 unless false
      c += 100 if false
      assert_equal 11, c
    end
  end

  class ControlBugEmptyBodyTest < Minitest::Test
    def test_empty_bodies
      begin
        empty_risky(-1)
      rescue ArgumentError
      end
      out = [] #: Array[Integer]
      quiet(out, 1)
      quiet(out, -1)
      assert_equal [1], out

      seen = [] #: Array[String]
      i = 0
      if i > 2
      end
      unless i.zero?
      end
      if i > 5
        seen << "big"
      elsif i > 3
      else
      end
      while i > 5
      end
      i += 1 until i >= 3
      case i
      when 1, 2
      when 3 then seen << "three"
      else
      end
      begin
        seen << "body"
      ensure
      end
      assert_equal ["three", "body"], seen
      x = if i > 5 then end #: Integer?
      assert_nil x
      assert_equal 3, i
    end
  end

  class ControlBugIteratorReturnInEnsureBeginTest < Minitest::Test
    def test_return_in_ensure_begin_inside_iterator
      out = [] #: Array[String]
      upto_stop(out, 4) { |x| out << "got #{x}" }
      assert_equal ["ensure 0", "got 0", "ensure 1", "got 1", "ensure 2"], out
    end

    def test_break_through_ensure_inside_iterator
      out = [] #: Array[String]
      each_logged_ensure(out) do |v|
        break if v == 2

        out << "b#{v}"
      end
      assert_equal ["b1", "after 1", "after 2"], out
    end
  end

  class ControlBugLocalNameCollisionTest < Minitest::Test
    def test_locals_named_like_generated_temps
      r = 10
      assert_equal 10, r
      seen = 0
      begin
        raise "boom"
      rescue
        seen = r
      end
      assert_equal 10, seen
      assert_equal 200, collide_pick(nil)
      assert_equal 101, collide_pick(1)
      p = 7
      v = collide_risky(-1) rescue p
      assert_equal 7, v
      assert_equal "v2", doubled(1)
      assert_equal "rescued", doubled(-1)
    end
  end

  class ControlBugUnassignedBranchLocalTest < Minitest::Test
    def test_unassigned_branch_locals_are_nil
      assert_equal 1, branch_local(true)
      assert_nil branch_local(false)
      assert_equal "v1", loop_local(2)
      assert_nil loop_local(0)
      b = 5 if [].size > 0
      assert_nil b
      out = [] #: Array[String]
      control_half(out, 4)
      control_half(out, 3)
      assert_equal ["2", "rescue sees nil", "nil"], out
    end
  end

  class ControlDefiniteAssignTest < Minitest::Test
    def test_definite_assignment_keeps_locals_non_optional
      assert_equal 2, both_branches(true)
      assert_equal 4, both_branches(false)
      assert_equal "ONE", case_else(1)
      assert_equal "MANY", case_else(2)
      out = [] #: Array[Integer]
      definite_guarded(out, 4)
      definite_guarded(out, -1)
      assert_equal [8], out
      assert_equal 10, raised(5)
      assert_equal 2, definite_countdown
      out2 = [] #: Array[Integer]
      rescued(out2, 1)
      rescued(out2, 2)
      assert_equal [2, 4], out2
      out3 = [] #: Array[String]
      ensured(out3)
      assert_equal ["body", "10"], out3
      out4 = [] #: Array[String]
      narrowed(out4, true)
      narrowed(out4, false)
      assert_equal ["6", "8", "4", "nil"], out4
    end

    def test_branch_locals_in_blocks_and_loops
      got = [] #: Array[String?]
      [1, 2].each do |k|
        if k == 1
          m = "set"
        end
        got << m
      end
      assert_equal ["set", nil], got
      zs = [] #: Array[Integer?]
      i = 0
      while i < 2
        if i == 0
          z = 1
        end
        zs << z
        i += 1
      end
      assert_equal [1, 1], zs
      w = 1 unless [1].empty?
      assert_equal 1, w
    end
  end

  class ControlEdgesTest < Minitest::Test
    # a local assigned on every pass of a block is fresh each iteration
    def test_block_local_fresh_each_iteration
      seen = [] #: Array[String]
      [1, 2, 3].each do |v|
        acc = [] #: Array[Integer]
        acc << v
        seen << acc.inspect
      end
      assert_equal ["[1]", "[2]", "[3]"], seen
      doubled = [1, 2].map do |v|
        tmp = v * 2
        tmp
      end
      assert_equal [2, 4], doubled
    end

    # an if modifier / ternary with a nil branch inside interpolation is ""
    def test_nil_branch_in_interpolation
      x = 3
      assert_equal "ac", "a#{"b" if x > 5}c"
      assert_equal "abc", "a#{"b" if x < 5}c"
      assert_equal "n=.", "n=#{x > 5 ? "big" : nil}."
    end

    def test_method_level_rescue
      assert_equal 1, safe(1)
      assert_nil safe(-1)
      assert_equal 2, rescue_return(2)
      assert_equal(-1, rescue_return(-2))
      assert_equal ["one", "err two", "other"], [cased(1), cased(2), cased(3)]
    end

    # nested rescue modifiers: the outer one catches the inner fallback's raise
    def test_nested_rescue_modifiers
      v = (edges_risky(-1) rescue edges_risky(-2)) rescue 3
      assert_equal 3, v
      v2 = (edges_risky(-1) rescue edges_risky(4)) rescue 3
      assert_equal 4, v2
    end

    # begin/ensure as a value keeps the body's value
    def test_begin_ensure_value
      out = [] #: Array[String]
      y = begin
        5
      ensure
        out << "y ensure"
      end
      assert_equal ["y ensure"], out
      assert_equal 5, y
    end

    # a rescued exception can be the subject of a type case
    def test_rescued_exception_in_type_case
      kinds = [] #: Array[String]
      [KeyError.new("k"), ArgumentError.new("a"), IOError.new("io")].each do |ex|
        begin
          raise ex
        rescue => e
          kind = case e
                 when KeyError then "key"
                 when ArgumentError then "arg"
                 else "other #{e.class}"
                 end
          kinds << kind
        end
      end
      assert_equal ["key", "arg", "other IOError"], kinds
    end

    # exceptions show their message through to_s, interpolation and untyped
    # (puts/print of an exception stays in testdata/run/control_output.rb)
    def test_exception_message_through_to_s_and_interpolation
      e = assert_raises(EdgesAppError) { raise EdgesAppError, "boom" }
      assert_equal "boom", e.to_s
      assert_equal "Error: boom", "Error: #{e}"
      err = KeyError.new("k") #: untyped
      assert_equal "k", err.to_s
      assert_equal "u: k", "u: #{err}"
      assert_equal "ArgumentError", ArgumentError.new.to_s
      assert_equal "RuntimeError", "#{RuntimeError.new}"
      list = [ArgumentError.new("a1"), TypeError.new("t1")] #: Array[StandardError]
      assert_equal ["a1", "t1"], list.map(&:message)
      assert_equal ["ArgumentError", "TypeError"], list.map { |l| l.class.name }
      assert_equal "[#<ArgumentError: a1>, #<TypeError: t1>]", list.inspect
      assert_equal "#<ControlTests::EdgesApp::Missing: x>", EdgesApp::Missing.new("x").inspect
      assert_equal "ControlTests::EdgesApp::Missing", EdgesApp::Missing.new.message
    end

    # messages built from nil optionals, and used as hash keys
    def test_messages_from_nil_optionals_and_as_keys
      none = nil #: Integer?
      e = assert_raises(ArgumentError) { raise ArgumentError, "value was #{none.inspect}" }
      assert_equal "value was nil", e.message
      e2 = assert_raises(RuntimeError) { raise "count: #{none || 0}" }
      assert_equal "count: 0", e2.message
      assert_equal RuntimeError, e2.class
      msgs = {} #: Hash[String, Integer]
      [ArgumentError.new("a"), KeyError.new("a"), TypeError.new("b")].each do |ex|
        msgs[ex.message] = (msgs[ex.message] || 0) + 1
      end
      assert_equal "{\"a\" => 2, \"b\" => 1}", msgs.inspect
    end

    # rescue by a module the exception class includes
    def test_rescue_by_included_module
      got = "" #: String
      begin
        raise TaggedError, "t"
      rescue Tag
        got = "rescued by module"
      end
      assert_equal "rescued by module", got
    end

    # while as the last statement of a value method, a void method and a block
    def test_while_as_last_statement
      assert_equal "done 3", tail_while(3)
      out = [] #: Array[String]
      void_while(out)
      assert_equal ["k0", "k1"], out
      ws = [] #: Array[Integer]
      [1].each do |z|
        w = 0
        w += 1 while w < z
        ws << w
      end
      assert_equal [1], ws
      a = 0
      while a < 2 do a += 1 end
      assert_equal 2, a
      b = 10
      until b < 5 || b.even? && b < 8
        b -= 1
      end
      assert_equal 6, b
    end

    # break from a while inside a begin body, and a loop inside ensure
    def test_loops_inside_begin_and_ensure
      out = [] #: Array[String]
      begin
        n = 0
        while true
          n += 1
          break if n == 3
        end
        out << n.to_s
      rescue => e
        out << e.message
      end
      begin
        out << "body"
      ensure
        c = 0
        while c < 2
          c += 1
          next if c == 1

          out << "ensure loop #{c}"
        end
      end
      # break inside an iterator in a rescue clause
      begin
        raise "x"
      rescue => e
        [1, 2, 3].each do |q|
          break if q == 2

          out << "#{e.message}#{q}"
        end
      end
      assert_equal ["3", "body", "ensure loop 2", "x1"], out
    end

    # retry by hand: an until loop around begin/rescue
    def test_retry_by_hand
      out = [] #: Array[String]
      attempts = 0
      done = false
      until done
        begin
          attempts += 1
          raise "flaky" if attempts < 3

          done = true
        rescue
          out << "retrying #{attempts}"
        end
      end
      assert_equal ["retrying 1", "retrying 2"], out
      assert_equal 3, attempts
    end

    # conditions that need lifted statements outside && / ||
    def test_lifted_conditions
      x = 3
      h = { "a" => 1, "b" => 5 } #: Hash[String, Integer]
      assert_equal ["x", "found 1", "missing"], [control_probe(h, "x"), control_probe(h, "a"), control_probe(h, "zz")]
      assert_equal "zero", ((h["q"] || 0) > 0 ? "pos" : "zero")
      out = [] #: Array[String]
      unless (h["b"] || 0) > 100
        out << "unless lifted"
      end
      if (h["b"] || 0) > 1 && x > 2
        out << "lifted left"
      end
      assert_equal ["unless lifted", "lifted left"], out
      # ! and not on values that are never nil
      s = "s"
      assert_equal false, (!x)
      assert_equal false, (not x)
      assert_equal false, (!s)
      assert_equal true, (!!s)
    end

    def test_case_values
      out = [] #: Array[String]
      assert_equal ["one", "few 30", "many"], [control_label(out, 1), control_label(out, 3), control_label(out, 9)]
      assert_equal ["matched one"], out
      sym = :b
      res = case sym
            when :a then 1
            when :b then 2
            end
      assert_equal 2, res
      res2 = case sym
             when :zz then 1
             end
      assert_nil res2
    end

    # ||= in a while loop, and ||= from another optional
    def test_or_assign_in_loop_and_from_optional
      got = nil #: String?
      i = 0
      while i < 3
        got ||= "set at #{i}"
        i += 1
      end
      assert_equal "set at 0", got
      o1 = nil #: Integer?
      o2 = nil #: Integer?
      o1 ||= o2
      assert_nil o1
      o2 = 4
      o1 ||= o2
      assert_equal 4, o1
    end

    # && / || chains mixing optionals, Booleans and interpolation
    def test_and_or_chains
      none = nil #: Integer?
      some = 3 #: Integer?
      assert_equal "x", ((some && "x") || "y")
      assert_equal "y", ((none && "x") || "y")
      name = nil #: String?
      assert_equal "hi anon", "hi #{name || "anon"}"
      big = some && some > 2
      big2 = none && none > 2
      assert_equal true, big
      assert_nil big2
      flag = false
      assert_equal 3, (flag || some)
      assert_nil (flag || none)
    end

    # multiple assignment from an Array-returning method and from a pair
    def test_multiple_assignment_from_method_and_pair
      p1, p2 = parts
      assert_equal "x", p1
      assert_equal "y", p2
      h = { "a" => 1, "b" => 5 } #: Hash[String, Integer]
      pair = h.to_a[0] || ["none", 0]
      kk, vv = pair
      assert_equal "a", kk
      assert_equal 1, vv
      arr = [1, 2] #: Array[Integer]
      f0, f1 = arr[1], arr[0]
      assert_equal [2, 1], [f0, f1]
    end

    # next/break guards narrow the block parameter for the rest of the block
    def test_guards_narrow_block_param
      opts = [1, nil, 3] #: Array[Integer?]
      got = [] #: Array[Integer]
      opts.each do |o|
        next unless o

        got << o + 1
      end
      opts.each do |o|
        next if o.nil?

        got << o * 2
      end
      assert_equal [2, 4, 2, 6], got
      assert_equal 3, total_until_nil([1, 2, nil, 4])
      # break in an iterator nested in a while leaves only the iterator
      laps = 0
      while true
        [1, 2].each { |q| break if q == 1 }
        laps += 1
        break if laps > 1
      end
      assert_equal 2, laps
    end

    # decision 20: attribute reads on self narrow like locals
    def test_attribute_reads_narrow
      assert_equal ["v=2", "empty", "v=4", "none", "big 5", "small"],
                   [Box.new(1).show, Box.new(nil).show, Box.new(2).show2, Box.new(nil).show2, Box.new(5).show3, Box.new(nil).show3]
    end

    # rescue/ensure in initialize, in a class method and in a module function
    def test_rescue_in_initialize_class_method_and_module_function
      assert_equal true, Parser.new("x").ok
      assert_equal false, Parser.new("").ok
      out = [] #: Array[String]
      assert_equal 1, Parser.parse(out, "1")
      assert_equal 0, Parser.parse(out, "2")
      assert_equal ["class method ensure", "class method rescued bad", "class method ensure"], out
      assert_equal ["neg", "rescued zero", "pos"], [Util.check(-1), Util.check(0), Util.check(1)]
    end

    # rescue modifiers as conditions and as the left of ||
    def test_rescue_modifiers_as_conditions
      out = [] #: Array[String]
      out << "cond ok" if (edges_risky(1) rescue nil)
      out << "cond never" if (edges_risky(-1) rescue nil)
      out << ((edges_risky(-1) rescue false) ? "t" : "f")
      assert_equal ["cond ok", "f"], out
      rx = (edges_risky(2) rescue nil) || 9
      ry = (edges_risky(-2) rescue nil) || 9
      assert_equal 2, rx
      assert_equal 9, ry
    end

    # an iterator that yields inside begin/ensure runs the ensure after each block call
    def test_iterator_yield_inside_ensure
      out = [] #: Array[String]
      edges_each_logged(out) { |lv| out << "v#{lv}" }
      assert_equal ["v1", "after 1", "v2", "after 2", "v3", "after 3", "each_logged done"], out
    end

    # value `when`s on untyped and optional subjects, mixed with `when nil`
    def test_value_whens_on_untyped_and_optional
      assert_equal ["one", "a or b", "a or b", "nil", "float", "other", "other"],
                   [control_classify(1), control_classify("a"), control_classify(:b), control_classify(nil), control_classify(2.5), control_classify([1]), control_classify(false)]
      assert_equal ["nil", "xy", "other"], [opt_str(nil), opt_str("y"), opt_str("z")]
    end

    # rescue/ensure in inherited, overriding (super inside rescue) and mixed-in methods
    def test_rescue_in_inherited_overriding_and_mixed_in
      out = [] #: Array[String]
      assert_equal "impl fallback: abstract run", Impl.new.run
      assert_equal "impl fallback: abstract run", Impl.new.safe_run(out)
      assert_equal "base caught abstract run", EdgesPlain.new.safe_run(out)
      assert_equal ["base ensure ControlTests::Impl", "base ensure ControlTests::EdgesPlain"], out
      wk = Worker.new
      assert_equal "worker: attempt 1 failed", wk.attempt(1)
      assert_equal "attempt 2 ok", wk.attempt(2)
    end
  end

  class ControlExceptionFlowTest < Minitest::Test
    def test_custom_exception_initialize
      e = assert_raises(Fixed) { raise Fixed }
      assert_equal "fixed message", e.message
      e2 = assert_raises(Blank) { raise Blank }
      assert_equal "ControlTests::Blank", e2.message
      assert_equal "#<ControlTests::Blank: ControlTests::Blank>", e2.inspect
      e3 = assert_raises(WithCode) { raise WithCode.new(42) }
      assert_equal 42, e3.code
      assert_equal "code 42", e3.message
    end

    # an exception raised in ensure replaces the pending one
    def test_raise_in_ensure_replaces_pending
      out = [] #: Array[String]
      e = assert_raises(KeyError) do
        begin
          raise ArgumentError, "first"
        ensure
          out << "ensure raising"
          raise KeyError, "from ensure"
        end
      end
      assert_equal ["ensure raising"], out
      assert_equal "KeyError: from ensure", "#{e.class}: #{e.message}"
    end

    def test_rescue_that_raises_with_ensure
      out = [] #: Array[String]
      e = assert_raises(KeyError) { rescue_raises(out) }
      assert_equal ["ens"], out
      assert_equal "b", e.message
    end

    # nested rescue inside a rescue body, outer binding still visible
    def test_nested_rescue_sees_outer_binding
      got = "" #: String
      begin
        raise "x"
      rescue => e
        begin
          raise ArgumentError, "inner #{e.message}"
        rescue ArgumentError => e2
          got = "nested #{e2.message} outer #{e.message}"
        end
      end
      assert_equal "nested inner x outer x", got
      assert_equal ["A arg", "K key", "none"], [two_bindings(0), two_bindings(1), two_bindings(2)]
    end

    # exception objects stored in collections and passed through untyped
    def test_exceptions_in_collections_and_untyped
      errors = [] #: Array[StandardError]
      [ArgumentError.new("a"), KeyError.new("k")].each { |x| errors << x }
      assert_equal ["ArgumentError: a", "KeyError: k"], errors.map { |x| "#{x.class}: #{x.message}" }
      anything = RuntimeError.new("untyped") #: untyped
      assert_equal "untyped", anything.message
      e = assert_raises(KeyError) { raise errors[1] || StandardError.new("fallback") }
      assert_equal "stored k", "stored #{e.message}"
    end

    def test_rescue_clause_matching
      got = [] #: Array[String]
      # rescue matches a superclass in a list with a non-match first
      begin
        raise KeyError, "kk"
      rescue TypeError, IndexError => e
        got << "list: #{e.class} #{e.is_a?(IndexError)}"
      end
      # rescue Exception catches StandardError too
      begin
        raise "std"
      rescue Exception => e
        got << "Exception caught #{e.class}"
      end
      # rescue with no binding and a class list
      begin
        raise TypeError, "t"
      rescue ArgumentError, TypeError
        got << "no binding"
      end
      assert_equal ["list: KeyError true", "Exception caught RuntimeError", "no binding"], got
    end

    def test_raise_inside_loop_inside_method
      assert_equal "sum 6", sum_positive([1, 2, 3])
      assert_equal "failed: negative -2 after 1", sum_positive([1, -2, 3])
    end

    # ensure runs on normal completion inside a loop, every iteration
    def test_ensure_every_iteration
      out = [] #: Array[String]
      3.times do |t|
        begin
          out << "try #{t}"
        ensure
          out << "done #{t}"
        end
      end
      assert_equal ["try 0", "done 0", "try 1", "done 1", "try 2", "done 2"], out
    end

    def test_message_is_a_string
      e = assert_raises(ArgumentError) { raise ArgumentError, "bad #{Thing.new}" }
      assert_equal "bad thing", e.message
      e2 = assert_raises(RuntimeError) { raise "Mixed Case" }
      assert_equal "MIXED CASE", e2.message.upcase
      assert_equal 10, e2.message.size
      assert_equal true, ArgumentError.new("x").class == ArgumentError
      assert_equal "KeyError", KeyError.name
    end

    def test_rescue_by_namespaced_superclass
      e = assert_raises(FlowApp::Error) { FlowApp.find("") }
      assert_equal "ControlTests::FlowApp::Missing: no  true", "#{e.class}: #{e.message} #{e.is_a?(FlowApp::Missing)}"
    end

    # re-raise the same object from a rescue
    def test_reraise_same_object
      out = [] #: Array[String]
      outer = assert_raises(KeyError) do
        begin
          raise KeyError, "orig"
        rescue KeyError => e
          out << "logging #{e.message}"
          raise e
        end
      end
      assert_equal ["logging orig"], out
      assert_equal "outer KeyError orig", "outer #{outer.class} #{outer.message}"
    end

    # an exception raised in initialize escapes new
    def test_raise_in_initialize
      e = assert_raises(ArgumentError) { Strict.new(-1) }
      assert_equal "n must be positive, got -1", e.message
      assert_equal 2, Strict.new(2).n
      assert_equal 3, depth(3)
    end

    def test_ensure_return_and_rescue_values
      out = [] #: Array[String]
      void_ensure_return(out, false)
      void_ensure_return(out, true)
      assert_equal ["body", "ensure", "body", "ensure"], out
      out = [] #: Array[String]
      assert_equal "body", rescue_then_ensure_return(out, 0)
      assert_equal "rescue", rescue_then_ensure_return(out, 1)
      assert_equal ["ensure 0", "ensure 1"], out
      assert_equal "all 2", progress([1, 2])
      assert_equal "stop at -1 after 1", progress([1, -1, 2])
      out = [] #: Array[String]
      assert_equal 30, first_ok(out, [-1, -2, 3, 4])
      assert_equal(-1, first_ok(out, [-5]))
      assert_equal ["skip bad -1", "skip bad -2", "skip bad -5"], out
      assert_equal ["zero", "pos", "rescued"], [case_tail(0), case_tail(3), case_tail(-3)]
    end

    def test_rescue_values_in_ivars_and_blocks
      cc = Cache.new
      assert_equal [-1, 200, 200], [cc.value, cc.value, cc.value]
      assert_equal 5, Holder.new(5).v
      assert_equal 0, Holder.new(-5).v
      got = begin
        flow_risky(-2)
      rescue ArgumentError => err
        err.message.size
      end
      assert_equal 3, got
      assert_equal ["p1", "p2"], labels([1, 2])
      e = assert_raises(ArgumentError) { labels([1, -2]) }
      assert_equal "bad -2", e.message
      assert_equal ["one", "one"], names([1, 1])
      e = assert_raises(ArgumentError) { names([1, 2]) }
      assert_equal "unknown 2", e.message
      e2 = assert_raises(ZeroDivisionError) { divs([1, 0]) }
      assert_equal "from map: divided by 0", "from map: #{e2.message}"
      assert_equal 7, multi_in_begin
    end

    # rescue modifier on the right of ||=
    def test_rescue_modifier_right_of_or_assign
      opt = nil #: Integer?
      opt ||= (flow_risky(-1) rescue 5)
      assert_equal 5, opt
    end

    # ensure sees the loop variable of an enclosing iterator
    def test_ensure_sees_iterator_variable
      out = [] #: Array[String]
      [1, 2].each do |i|
        begin
          raise "odd" if i.odd?
        rescue
          out << "rescued #{i}"
        ensure
          out << "ensure #{i}"
        end
      end
      assert_equal ["rescued 1", "ensure 1", "ensure 2"], out
    end

    # exceptions from an untyped call
    def test_exception_from_untyped_call
      u = [1, 2] #: untyped
      e = assert_raises(IndexError) { u.fetch(10) }
      assert_equal "untyped fetch: IndexError", "untyped fetch: #{e.class}"
    end
  end

  class ControlExceptionsTest < Minitest::Test
    # Exception#cause: a raise inside a rescue clause records the exception being handled (decision 103).
    def test_cause
      outer = assert_raises(TypeError) do
        begin
          raise "inner"
        rescue RuntimeError
          raise TypeError, "outer"
        end
      end
      cause = outer.cause
      refute_nil cause
      assert_equal "inner", cause.message if cause
      assert_equal RuntimeError, cause.class
      plain = assert_raises(ArgumentError) { raise ArgumentError, "alone" }
      assert_nil plain.cause
      # re-raising the handled exception itself does not make it its own cause
      same = assert_raises(ArgumentError) do
        begin
          raise ArgumentError, "once"
        rescue ArgumentError => e
          raise e
        end
      end
      assert_nil same.cause
      # an exception that already has a cause keeps it
      kept = assert_raises(ArgumentError) do
        begin
          raise "first"
        rescue RuntimeError
          begin
            raise ArgumentError, "second"
          rescue ArgumentError => e
            raise e
          end
        end
      end
      kept_cause = kept.cause
      assert_equal "first", kept_cause.message if kept_cause
      refute_nil kept_cause
    end

    def test_raise_forms_and_messages
      out = [] #: Array[String]
      rows = [] #: Array[String]
      11.times do |k|
        begin
          boom(out, k)
        rescue => e
          rows << "#{e.class}: #{e.message.inspect} #{e.to_s.inspect}"
        end
      end
      assert_equal [
        "ArgumentError: \"bad arg\" \"bad arg\"",
        "RuntimeError: \"plain\" \"plain\"",
        "ControlTests::NotFound: \"ControlTests::NotFound\" \"ControlTests::NotFound\"",
        "ControlTests::NotFound: \"built\" \"built\"",
        "KeyError: \"k\" \"k\"",
        "ControlTests::AppError: \"ControlTests::AppError\" \"ControlTests::AppError\"",
        "ControlTests::ValidationError: \"too short\" \"too short\"",
        "ControlTests::DefaultMsg: \"default message\" \"default message\"",
        "IndexError: \"\" \"\"",
        "TypeError: \"ünïcode ✓\" \"ünïcode ✓\"",
      ], rows
      assert_equal ["no raise 10"], out
    end

    def test_rescue_clause_order_and_hierarchy
      out = [] #: Array[String]
      assert_equal [
        "NotFound ControlTests::NotFound", "AppError|KeyError ControlTests::AppError ControlTests::AppError", "AppError|KeyError KeyError k",
        "name: too short", "StandardError ArgumentError", "StandardError RuntimeError", "none",
      ], [which(out, 2), which(out, 5), which(out, 4), which(out, 6), which(out, 0), which(out, 1), which(out, 10)]
      assert_equal ["no raise 10"], out
      got = [] #: Array[String]
      begin
        raise NotFound, "nf"
      rescue AppError => e
        got << "superclass clause first: #{e.class}"
      rescue NotFound
        got << "never"
      end
      begin
        raise StopIteration, "stop"
      rescue IndexError => e
        got << "index family: #{e.class} #{e.message}"
      end
      assert_equal ["superclass clause first: ControlTests::NotFound", "index family: StopIteration stop"], got
      assert_equal "outer caught ControlTests::Fatal deep", outer
      se = assert_raises(ScriptError) do
        begin
          raise NotImplementedError, "nie"
        rescue StandardError
          got << "never"
        end
      end
      assert_equal "script: NotImplementedError nie false", "script: #{se.class} #{se.message} #{se.is_a?(StandardError)}"
    end

    # #43: the rest of MRI's exception tree, with its parents and default messages.
    def test_exception_tree_rest
      assert_equal [ScriptError, ScriptError, StandardError, Exception, Exception, StandardError],
                   [LoadError.superclass, SyntaxError.superclass, LocalJumpError.superclass,
                    SystemStackError.superclass, SecurityError.superclass, EncodingError.superclass]
      assert_equal ["LoadError", "compile error", "LocalJumpError", "SystemStackError", "bad"],
                   [LoadError.new.message, SyntaxError.new.message, LocalJumpError.new.message,
                    SystemStackError.new.message, SyntaxError.new("bad").message]
      got = [] #: Array[String]
      begin
        raise LoadError, "cannot load such file -- nope"
      rescue StandardError
        got << "never"
      rescue ScriptError => e
        got << "#{e.class}: #{e.message}"
      end
      begin
        raise SecurityError, "unsafe"
      rescue StandardError
        got << "never"
      rescue Exception => e
        got << "#{e.class}: #{e.message}"
      end
      assert_equal ["LoadError: cannot load such file -- nope", "SecurityError: unsafe"], got
    end

    def test_raise_stored_exception_and_constructors
      err = ArgumentError.new("stored")
      e = assert_raises(ArgumentError) { raise err }
      assert_equal true, e.equal?(err)
      assert_equal "stored", e.message
      assert_equal "ArgumentError", e.class.name
      assert_equal true, e.is_a?(StandardError)
      assert_equal false, e.is_a?(IndexError)
      assert_equal "ArgumentError", ArgumentError.new.message
      assert_equal "#<ArgumentError: ArgumentError>", ArgumentError.new.inspect
      assert_equal "ArgumentError", ArgumentError.new.to_s
      assert_equal "#<RuntimeError: ö ü>", RuntimeError.new("ö ü").inspect
      assert_equal "x", StandardError.new("x").message
      assert_equal "#<Exception: e>", Exception.new("e").inspect
      assert_equal "RuntimeError", RuntimeError.new(nil).message
      assert_nil Exception.new.backtrace
    end

    def test_rescue_ensure_ordering_and_values
      out = [] #: Array[String]
      assert_equal [1, 2], [ordered(out, false), ordered(out, true)]
      assert_equal ["body", "ensure", "body", "rescue", "ensure"], out
      assert_equal "body value", ensure_value
      assert_equal "pos", value_if(1)
      assert_equal "rescued nonpositive", value_if(0)
    end

    def test_nested_ensures_and_raise_from_rescue
      out = [] #: Array[String]
      begin
        begin
          begin
            raise ArgumentError, "x"
          ensure
            out << "e1"
          end
        ensure
          out << "e2"
        end
      rescue ArgumentError => e
        out << "caught #{e.message}"
      ensure
        out << "e3"
      end
      ae = assert_raises(AppError) do
        begin
          raise "first"
        rescue => e1
          raise AppError, "second from #{e1.message}"
        ensure
          out << "inner ensure"
        end
      end
      assert_equal "second from first", ae.message
      assert_equal ["e1", "e2", "caught x", "e3", "inner ensure"], out
    end

    def test_return_in_ensure
      assert_equal 2, ensure_overrides
      assert_equal "ensure wins", swallow
      assert_equal "body", ensure_after_rescue(0)
      assert_equal "ensure 2", ensure_after_rescue(2)
      e = assert_raises(RuntimeError) { ensure_after_rescue(1) }
      assert_equal "escaped RuntimeError: from rescue", "escaped #{e.class}: #{e.message}"
    end

    def test_return_through_ensure_in_loops
      out = [] #: Array[String]
      assert_equal ["found 7", "error neg -1", "none"], [scan(out, [1, 7, 3]), scan(out, [2, -1]), scan(out, [])]
      assert_equal ["inner 0", "inner 1", "inner 0", "inner 1"], out
      out = [] #: Array[String]
      void_ensure(out, true)
      void_ensure(out, false)
      assert_equal ["ensure true", "not returned", "ensure false", "after"], out
      out = [] #: Array[String]
      assert_equal 20, first_big(out, [1, 20, 30])
      assert_equal(-1, first_big(out, [1]))
      assert_equal ["checked 1", "checked 20", "checked 1"], out
    end

    def test_rescue_and_ensure_in_blocks
      out = [] #: Array[String]
      [1, 0, 2].each do |d|
        out << (10 / d).to_s
      rescue ZeroDivisionError => e
        out << e.message
      ensure
        out << "e#{d}"
      end
      assert_equal ["10", "e1", "divided by 0", "e0", "5", "e2"], out
      out = [] #: Array[String]
      exc_risky(out) { raise "in block" }
      exc_risky(out) { out << "no error" }
      assert_equal ["caught in block", "no error"], out
    end

    def test_runtime_errors_from_go
      out = [] #: Array[String]
      [0, 3, -3].each do |d|
        begin
          out << (10 / d).to_s << (10 % d).to_s << (-7 / d).to_s
        rescue ZeroDivisionError => e
          out << "zde #{e.message} #{e.class}"
        end
      end
      assert_equal ["zde divided by 0 ZeroDivisionError", "3", "1", "-3", "-4", "-2", "2"], out
      arr = [1, 2] #: Array[Integer]
      e = assert_raises(IndexError) { arr[-10] = 5 }
      assert_equal IndexError, e.class
      caught = "" #: String
      begin
        arr[-10] = 5
      rescue => e2
        caught = "catch-all sees #{e2.class} #{e2.is_a?(StandardError)}"
      end
      assert_equal "catch-all sees IndexError true", caught
    end

    def test_method_calls_on_nil
      none = nil #: Integer?
      e = assert_raises(NoMethodError) { none + 1 }
      assert_equal "undefined method '+' for nil", e.message
      e2 = assert_raises(NameError) { none.abs }
      assert_equal "NoMethodError: undefined method 'abs' for nil", "#{e2.class}: #{e2.message}"
    end

    def test_raise_as_an_expression
      assert_equal "one", name_of(1)
      e = assert_raises(ArgumentError) { name_of(7) }
      assert_equal "unknown 7", e.message
      assert_equal 2, must(1)
      e2 = assert_raises(KeyError) { must(nil) }
      assert_equal "missing", e2.message
      assert_equal 10, tern(true)
    end

    def test_rescue_modifiers
      a = risky_int(2) rescue 0
      b = risky_int(-1) rescue 0
      assert_equal 4, a
      assert_equal 0, b
      c = (risky_int(-5) rescue nil)
      assert_nil c
      d = risky_int(-1) rescue "failed"
      assert_equal "failed", d
      e2 = risky_int(3) rescue "failed"
      assert_equal 6, e2
      assert_equal(-10, (tern(false) rescue -10))
      hits = [] #: Array[String]
      f = risky_int(1) rescue hits.push("f").size
      f2 = risky_int(-1) rescue hits.push("f2").size
      assert_equal 2, f
      assert_equal 1, f2
      assert_equal ["f2"], hits
      ex = assert_raises(NotImplementedError) do
        g = risky_int(99) rescue -1
        flunk "got #{g}"
      end
      assert_equal "nie", ex.message
      list = [1, 2, 3] #: Array[Integer]
      assert_equal [100, 0, 2], list.map { |v| risky_int(v - 2) rescue 100 }
      assert_equal 1, control_safe_fetch("a")
      assert_equal(-1, control_safe_fetch("b"))
    end

    def test_begin_as_value_and_std_hierarchy
      out = [] #: Array[String]
      y = begin
        1 / 0
      rescue ZeroDivisionError
        -5
      ensure
        out << "y ensure"
      end
      assert_equal ["y ensure"], out
      assert_equal(-5, y)
      z = begin 5 end
      assert_equal 5, z
      e = assert_raises(IOError) { raise IOError, "closed stream" }
      assert_equal "IOError: closed stream true", "#{e.class}: #{e.message} #{e.is_a?(StandardError)}"
      e2 = assert_raises(RangeError) { raise RangeError, "out of range" }
      assert_equal "#<RangeError: out of range>", e2.inspect
      assert_equal false, e2.is_a?(IndexError)
    end
  end

  class ControlLocalsTest < Minitest::Test
    def test_locals_in_branches_loops_blocks_and_op_assign
      assert_equal "large!", size_word(1000)
      assert_equal "medium!", size_word(50)
      assert_equal "small!", size_word(0)
      assert_equal 10, control_weight(:a)
      assert_equal 20, control_weight(:b)
      assert_equal 0, control_weight(:c)
      out = [] #: Array[String]
      assert_equal "divided 20", parse(out, 5)
      assert_equal "start -1", parse(out, 0)
      assert_equal ["ensure sees divided", "rescue sees start", "ensure sees start"], out
      assert_equal "nil", nil_first(1)
      assert_equal "\"non-positive\"", nil_first(0)
      i = 0
      while i < 3
        last = i * 10
        i += 1
      end
      assert_equal 20, last
      unless i.zero?
        msg = "i=#{i}"
      else
        msg = "zero"
      end
      assert_equal "i=3", msg
      maybe = nil
      maybe = "now" if i == 3
      assert_equal "now", maybe
      unset = nil
      unset = "never" if i == 99
      assert_nil unset
      count = nil #: Integer?
      assert_nil count
      count = 5
      assert_equal 5, count
      count = nil
      assert_nil count
      count = -1
      assert_equal(-1, count)
      sum = 0
      doubled = [1, 2, 3].map { |x| sum += x; x * 2 }
      assert_equal 6, sum
      assert_equal [2, 4, 6], doubled
      seen = 0
      [4, 5].each { |v| seen = v }
      assert_equal 5, seen
      grid = [] #: Array[String]
      [1, 2].each do |r|
        [3, 4].each do |c|
          cell = "#{r}x#{c}"
          grid << cell
        end
      end
      assert_equal ["1x3", "1x4", "2x3", "2x4"], grid
      assert_equal [1, 3, 6, 0], (running([1, 2, 3, -6]))
      assert_equal [], running([])
      assert_equal 7, control_bump(4)
      assert_equal(-9, control_bump(-4))
      assert_equal(-1, control_bump(0))
      s = "a"
      s += "b"
      s *= 2
      assert_equal "abab", s
      f = 1.5
      f /= 2
      f -= 0.25
      assert_equal "0.5", (f).inspect
      b = 7
      b %= 4
      b **= 3
      b /= 2
      assert_equal 13, b
      nb = -7
      nb /= 2
      assert_equal(-4, nb)
      nm = -7
      nm %= 3
      assert_equal 2, nm
      list = [1] #: Array[Integer]
      list += [2, 3]
      assert_equal [1, 2, 3], list
      x1 = 1
      x1 = x1 + 1 while x1 < 100
      assert_equal 100, x1
      assert_equal "1!,2!,outer", shadow(1)
      nums = [1, 2, 3] #: Array[Integer]
      evens = [] #: Array[Integer]
      odds = 0
      nums.each do |n|
        if n.even?
          evens << n
        else
          odds += 1
        end
      end
      assert_equal [2], evens
      assert_equal 2, odds
      flag = false
      nums.each { |n| flag = true if n > 2 }
      assert_equal true, flag
      it_sum = 0
      nums.each { it_sum += it }
      assert_equal 6, it_sum
    end
  end

  class ControlLocalsScopeTest < Minitest::Test
    # locals assigned inside ensure or first assigned inside rescue are visible after
    def test_locals_from_ensure_and_rescue
      assert_equal "cleaned 1", ens_local
      assert_equal "0 \"div by zero\"", rescue_local(0)
    end

    # locals assigned in while loop bodies accumulate across iterations
    def test_locals_assigned_in_while_loop
      i = 0
      parts = [] #: Array[String]
      while i < 3
        piece = "p#{i}"
        parts << piece
        i += 1
      end
      assert_equal "p0,p1,p2", parts.join(",")
      assert_equal "p2", piece
      assert_equal 7, writes_only(true)
    end

    # chained assignment and assignment as an expression
    def test_chained_assignment_and_assignment_as
      a = b = c = 0
      a += 1
      assert_equal 1, a
      assert_equal 0, b
      assert_equal 0, c
      d = (e = 5) + 1
      assert_equal 6, d
      assert_equal 5, e
      arr = [] #: Array[Integer]
      arr << (f = 9)
      assert_equal [9], arr
      assert_equal 9, f
    end

    # op-assign on a narrowed optional
    def test_op_assign_on_a_narrowed
      maybe = 3 #: Integer?
      inside = "" #: String
      if maybe
        maybe += 1
        inside = maybe.to_s
      end
      assert_equal "4", inside
      assert_equal 4, maybe
      assert_equal "ANON", default_name(nil)
      assert_equal "BO", default_name("bo")
      assert_equal 42, or_value(nil)
      assert_equal 1, or_value(1)
      n = 13
      bits = [] #: Array[Integer]
      while n > 0
        n, bit = step(n)
        bits << bit
      end
      assert_equal [1, 0, 1, 1], bits
      lo, hi = Pair.new(1, 2).swapped
      assert_equal 2, lo
      assert_equal 1, hi
    end

    # multiple assignment of a mixed-type literal
    def test_multiple_assignment_of_a_mixed
      name, age, admin = "ann", 30, false
      assert_equal "ann", name
      assert_equal 30, age
      assert_equal false, admin
    end

    # destructure with an optional element used after a check
    def test_destructure_with_an_optional_element
      first, second = [7] #: Array[Integer]
      got = 0
      got = first + 1 if first
      assert_equal 8, got
      assert_equal(-1, (second || -1))
    end

    # `it` and numbered params
    def test_it_and_numbered_params
      assert_equal [2, 4], ([1, 2].map { it * 2 })
      assert_equal [4, 5], ([3, 4].map { _1 + 1 })
    end

    # Ruby locals, params and rescue bindings may use Go keywords and predeclared names
    def test_ruby_locals_params_and_rescue
      type = "t"
      range = 1
      default = :d
      select = [1, 2] #: Array[Integer]
      len = select.size
      string = "s"
      error = ArgumentError.new("e")
      new = 5
      copy = new + 1
      max = [3, 9].max
      min = 2
      func = true
      go = "go"
      var = 1
      map = { "a" => 1 } #: Hash[String, Integer]
      interface = nil #: Integer?
      chan = 3
      struct = "st"
      package = "pkg"
      import = "imp"
      fallthrough = 1
      defer = 2
      goto = 3
      const = 4
      main = "main"
      init = "init"
      stdout = "out"
      any = select.empty?
      bool = false
      int = 7
      append = [1] #: Array[Integer]
      append << 2
      panic = "p"
      recover = "r"
      real = 1.5
      iota = 0
      byte = "b"
      rune = "r"
      float64 = 2.5
      assert_equal "[\"t\", 1, :d, 2, \"s\", \"e\", 5, 6, 9, 2, true, \"go\", 1, 1, \"nil\", 3, \"st\", \"pkg\", \"imp\", 1, 2, 3, 4, \"main\", \"init\", \"out\", false, false, 7, \"[1, 2]\", \"p\", \"r\", 1.5, 0, \"b\", \"r\", 2.5]", (([type, range, default, len, string, error.message, new, copy, max, min, func, go, var,
                                                                                                                                                                                                                                              map.size, interface.inspect, chan, struct, package, import, fallthrough, defer, goto, const,
                                                                                                                                                                                                                                              main, init, stdout, any, bool, int, append.inspect, panic, recover, real, iota, byte, rune, float64])).inspect
      assert_equal "a1", (kw("a", 1))
      msg = "" #: String
      begin
        raise "x"
      rescue => error
        msg = error.message
      end
      assert_equal "x", msg
      types = [] #: Array[Integer]
      [1].each { |type| types << type }
      assert_equal [1], types
    end
  end

  class ControlLoopsTest < Minitest::Test
    def test_while_and_until
      i = 0
      while i < 3
        i += 1
      end
      assert_equal 3, i
      j = 10
      until j <= 7
        j -= 1
      end
      assert_equal 7, j
      never = 0
      while false
        never += 1
      end
      until true
        never += 1
      end
      assert_equal 0, never
      k = 0
      k += 2 while k < 9
      assert_equal 10, k
      m = 5
      m -= 1 until m.zero?
      assert_equal 0, m
      neg = -3
      neg += 1 while neg.negative?
      assert_equal 0, neg
      acc = [] #: Array[Integer]
      n = 0
      while true
        n += 1
        next if n.odd?
        break if n > 8

        acc << n
      end
      assert_equal [2, 4, 6, 8], acc
      assert_equal 10, n
    end

    # break and next only affect the innermost loop
    def test_break_and_next_only_affect
      out = [] #: Array[String]
      a = 0
      while a < 3
        b = 0
        while true
          b += 1
          break if b > a
          next if b == 2

          out << "#{a}#{b}"
        end
        a += 1
      end
      assert_equal ["11", "21"], out
      xs = [3, 1, 2] #: Array[Integer]
      popped = [] #: Array[Integer]
      until xs.empty?
        popped << xs.size
        xs.pop
      end
      assert_equal [3, 2, 1], popped
      assert_equal [], xs
      assert_equal 0, collatz(1)
      assert_equal 8, collatz(6)
      assert_equal 111, collatz(27)
      total = 0
      10.times do |t|
        next if t.even?
        break if t > 7

        total += t
      end
      assert_equal 16, total
      never = 0
      0.times { never += 1 }
      neg = 0
      neg.pred.pred.times { never += 1 }
      assert_equal 0, never
      steps = [] #: Array[Integer]
      5.downto(1) do |d|
        next if d == 3

        steps << d
      end
      1.upto(0) { |u| steps << 100 + u }
      -1.upto(1) { |u| steps << u }
      assert_equal [5, 4, 2, 1, -1, 0, 1], steps
      words = ["a", "b", "c", "d"] #: Array[String]
      got = [] #: Array[String]
      words.each_with_index do |s, idx|
        next if idx.zero?
        break if s == "d"

        got << "#{idx}:#{s}"
      end
      words.reverse_each do |s|
        next if s == "c"

        got << s
        break if s == "b"
      end
      h = { "x" => 1, "y" => 2, "z" => 3 } #: Hash[String, Integer]
      h.each do |key, val|
        next if val == 1

        got << "#{key}=#{val}"
        break if key == "y"
      end
      assert_equal ["1:b", "2:c", "d", "b", "y=2"], got
      assert_equal 1, (index_of_empty(["a", "", "b"]))
      assert_nil index_of_empty(["a"])
      assert_nil index_of_empty([])
      grid = [[1, 2], [3, 4]] #: Array[Array[Integer]]
      assert_equal "1,1", (find_pair(grid, 4))
      assert_equal "0,0", (find_pair(grid, 1))
      assert_equal "missing", (find_pair(grid, 9))
      assert_equal 64, control_first_square_over(50)
      assert_equal 1, control_first_square_over(0)
      assert_nil control_first_square_over(100_000)
    end

    # break, next, return and raise out of blocks given to methods with ensure/rescue
    def test_jumps_out_of_yielding_methods
      out = [] #: Array[String]
      void_early(out, 1)
      void_early(out, 5)
      with_cleanup(out) do |v|
        out << "got #{v}"
        break if v == 1
      end
      with_cleanup(out) do |v|
        next if v == 1

        out << "next skipped to #{v}"
      end
      e = assert_raises(ArgumentError) do
        with_cleanup(out) { |v| raise ArgumentError, "from block #{v}" }
      end
      assert_equal "from block 1", e.message
      assert_equal ["small 1", "got 1", "cleanup", "next skipped to 2", "cleanup", "cleanup"], out
      out = [] #: Array[String]
      assert_equal 200, find_neg(out, [-2])
      assert_nil find_neg(out, [])
      assert_equal ["cleanup", "cleanup"], out
      out = [] #: Array[String]
      loops_countdown(5) do |c|
        next if c == 4
        break if c == 2

        out << "countdown #{c}"
      end
      loops_guarded(out) do |g|
        next if g == 2

        out << "guarded #{g}"
        raise "stop at #{g}" if g == 3
      end
      assert_equal ["countdown 5", "countdown 3", "guarded 1", "guarded 3", "guarded caught stop at 3"], out
    end

    def test_next_in_case_inside_loop
      kinds = [] #: Array[String]
      v = 0
      while v < 5
        v += 1
        case v
        when 2, 4 then next
        when 5 then kinds << "last"
        else kinds << "v#{v}"
        end
      end
      assert_equal ["v1", "v3", "last"], kinds
      assert_equal "string s", (first_string([1, nil, "s", "t"]))
      assert_equal "no string", first_string([nil])
      assert_equal "no string", first_string([])
    end
  end

  class ControlMidTest < Minitest::Test
    # a block local first assigned nil joins to T?; only hoisting scope is under test, not a never-assigned zero value
    def test_block_local_first_assigned_nil
      marks = [] #: Array[String]
      [1, 2].each do |v|
        if v > 5
          mark = nil
        elsif v == 1
          mark = "first"
        end
        marks << mark.inspect
      end
      assert_equal ["\"first\"", "nil"], marks
      out = [1, 2, 3].map do |v|
        seen = nil if v > 100
        seen = v * 10 if v.odd?
        seen.inspect
      end
      assert_equal ["10", "nil", "30"], out
      assert_equal ["ruby: \"long\"", "go: nil", "rust: \"long\"", "c: nil"], control_tags(["ruby", "go", "rust", "c"])
    end

    # a block param shadows an outer local of the same name without assigning it
    def test_block_param_shadows_outer_local
      x = 10
      got = [] #: Array[Integer]
      [1, 2].each { |x| got << x }
      assert_equal [1, 2], got
      assert_equal 10, x
      ys = [1, 2].map { |x| x * 3 }
      assert_equal [3, 6], ys
      assert_equal 10, x
      out = [] #: Array[String]
      assert_equal "none", last_seen(out, ["a", "b"])
      assert_equal ["item a", "item b"], out
    end

    # break inside a case arm leaves the enclosing loop, not the case
    def test_break_inside_case_arm
      i_break = 0
      while i_break < 10
        i_break += 1
        case i_break
        when 3 then break
        end
      end
      assert_equal 3, i_break
      out_break = [] #: Array[String]
      [1, 2, 3].each do |x|
        case x
        when 2 then break
        else out_break << x.to_s
        end
      end
      assert_equal ["1"], out_break
      vals = [1, "s", nil] #: Array[untyped]
      n_break = 0
      vals.each do |v|
        n_break += 1
        case v
        when String then break
        end
      end
      assert_equal 2, n_break
    end

    # an overridden message or to_s drives what inspect and rescue see
    def test_overridden_message_and_to_s
      assert_equal "x", CustomMessage.new("x").to_s
      assert_equal "#<ControlTests::CustomMessage: x>", CustomMessage.new("x").inspect
      assert_equal "custom", CustomMessage.new("x").message
      assert_equal "tos", CustomToS.new("x").message
      assert_equal "#<ControlTests::CustomToS: tos>", CustomToS.new("x").inspect
      e = assert_raises(CustomToS) { raise CustomToS, "zz" }
      assert_equal "tos", e.message
    end

    # is_a? narrows a T? local so the typed call accepts it
    def test_is_a_narrows_optional
      got = [] #: Array[Integer]
      x_narrow = 5 #: Integer?
      if x_narrow.is_a?(Integer)
        got << dbl(x_narrow)
      end
      y_narrow = nil #: Integer?
      got << (y_narrow.is_a?(Integer) ? dbl(y_narrow) : -1)
      assert_equal [10, -1], got
      pet = MidAnimal.new #: MidAnimal?
      hi = "" #: String
      hi = control_greet(pet) if pet.is_a?(MidAnimal)
      assert_equal "hi animal", hi
      s_narrow = "a" #: String?
      src = s_narrow.is_a?(Regexp) ? s_narrow.source : "not a regexp"
      assert_equal "not a regexp", src
      assert_equal 0, s_narrow.is_a?(Array) ? s_narrow.size : 0
    end

    # loop jumps from inside begin/rescue/ensure must still reach the enclosing loop
    def test_loop_jumps_from_begin_rescue_ensure
      i_jump = 0
      seen_jump = [] #: Array[Integer]
      while i_jump < 6
        i_jump += 1
        begin
          next if i_jump == 2
          break if i_jump == 5
          raise "odd" if i_jump.odd?

          seen_jump << i_jump
        rescue
          seen_jump << -i_jump
        end
      end
      assert_equal [-1, -3, 4], seen_jump
      assert_equal 5, i_jump
      out_jump = [] #: Array[Integer]
      [1, 2, 3].each do |x|
        begin
          raise "two" if x == 2

          out_jump << x
        rescue
          next
        ensure
          out_jump << 0
        end
        out_jump << 10
      end
      assert_equal [1, 0, 10, 0, 3, 0, 10], out_jump
    end

    # a closure-style block: `next` inside begin still runs ensure and skips the rest
    def test_next_inside_begin_in_closure_block
      out = [] #: Array[String]
      mid_guarded(out) do |g|
        begin
          next if g == 2

          out << "in begin #{g}"
        ensure
          out << "ensure #{g}"
        end
        out << "after begin #{g}"
      end
      mid_guarded(out) do |g|
        begin
          raise "odd" if g.odd?
        rescue
          next
        end
        out << "even #{g}"
      end
      assert_equal ["in begin 1", "ensure 1", "after begin 1", "ensure 2", "in begin 3", "ensure 3", "after begin 3", "even 2"], out
    end

    def test_nil_joins_local_to_optional
      out = [] #: Array[String]
      joined(out, 1)
      joined(out, 9)
      joined(out, -1)
      assert_equal ["\"a\"", "\"pos\"", "nil", "\"pos\"", "\"a\"", "nil"], out
    end

    # rescue and ensure bodies on a value-returning block
    def test_rescue_and_ensure_on_value_block
      ds_rescue = [2, 0, 5] #: Array[Integer]
      q_rescue = ds_rescue.map do |d|
        10 / d
      rescue ZeroDivisionError
        -1
      end
      assert_equal [5, -1, 2], q_rescue
      r_rescue = ds_rescue.map { |d| begin; 10 / d; rescue ZeroDivisionError; -2; end }
      assert_equal [5, -2, 2], r_rescue
      ens = [] #: Array[String]
      s_ensure = [2, 5].map do |d|
        10 / d
      ensure
        ens << "ensure #{d}"
      end
      assert_equal ["ensure 2", "ensure 5"], ens
      assert_equal [5, 2], s_ensure
    end

    # a rescue modifier whose fallback itself raises
    def test_rescue_modifier_fallback_raises
      e = assert_raises(KeyError) do
        v_wrap = mid_risky(-1) rescue raise(KeyError, "wrapped")
        flunk "got #{v_wrap}"
      end
      assert_equal "wrapped", e.message
      w_risky = mid_risky(2) rescue raise("no")
      assert_equal 2, w_risky
    end

    # same-named locals in sibling blocks are separate, even with different types
    def test_same_named_locals_in_sibling_blocks
      got = [] #: Array[String]
      [1, 2].each do |z|
        w = z * 2
        got << w.to_s
      end
      [3].each do |z|
        w = "s#{z}"
        got << w
      end
      names = ["ann", "bo"] #: Array[String]
      names.each { |n| s = n.upcase; got << s }
      [4, 5].each { |c| s = c * 2; got << s.to_s }
      assert_equal ["2", "4", "s3", "ANN", "BO", "8", "10"], got
      out = [] #: Array[String]
      later_outer(out)
      assert_equal ["[\"1\", \"2\"]", "6"], out
    end

    # a value block inside begin, and a method-level ensure around one
    def test_value_block_inside_begin_and_method_ensure
      ds_begin = [2, 5] #: Array[Integer]
      got = [] #: Array[Integer]
      begin
        q_begin = ds_begin.map { |d| 10 / d }
        got = q_begin
      rescue ZeroDivisionError => e_div
        flunk e_div.message
      end
      assert_equal [5, 2], got
      out = [] #: Array[String]
      assert_equal [2, 3], halves(out, [4, 6])
      assert_equal ["done"], out
    end

    # assignments and calls inside while/until conditions
    def test_assignments_in_loop_conditions
      queue = [1, 2, 3] #: Array[Integer]
      got = [] #: Array[String]
      guard = 0
      while (item = queue.shift)
        guard += 1
        break if guard > 5

        got << item.to_s
      end
      assert_equal ["1", "2", "3"], got
      assert_equal [], queue
      assert_equal 3, guard

      q_while = [1, 2, 5, 1] #: Array[Integer]
      shifted = [] #: Array[String]
      guard = 0
      while (q_while[0] || 0) < 3
        guard += 1
        break if guard > 5

        shifted << q_while.shift.inspect
      end
      assert_equal ["1", "2"], shifted
      assert_equal 2, guard

      stack = [4, 0] #: Array[Integer]
      popped = [] #: Array[String]
      guard = 0
      until (top = stack.pop).nil?
        guard += 1
        break if guard > 5

        popped << top.to_s
      end
      assert_equal ["0", "4"], popped
      assert_equal 2, guard
    end

    # zsuper in an exception initialize forwards the (defaulted) message
    def test_zsuper_in_exception_initialize
      assert_equal "x", MidPlain.new("x").message
      e = assert_raises(MidDefaultMsg) { raise MidDefaultMsg }
      assert_equal "default message", e.message
    end

    # next in value blocks yields nil for that element
    def test_next_in_value_blocks
      xs = [1, 2, 3] #: Array[Integer]
      ys_next = xs.map do |x|
        next if x == 2

        x * 10
      end
      assert_equal "10,nil,30", ys_next.map { |y| y ? y.to_s : "nil" }.join(",")
      picked = xs.select do |x|
        next if x.odd?

        true
      end
      assert_equal [2], picked
      assert_equal "[nil, nil, nil]", xs.map { next }.inspect
      printed = [] #: Array[Integer]
      ws = xs.map do |x|
        next if x == 1

        printed << x
        nil
      end
      assert_equal [2, 3], printed
      assert_equal "[nil, nil, nil]", ws.inspect
    end
  end

  # Kernel#catch / #throw (issue #4, decision 91).
  class ControlCatchThrowTest < Minitest::Test
    def test_grid_search
      grid = [[1, 2, 3], [4, 5, 6], [7, 8, 9]]
      found = catch(:found) do
        grid.each_with_index do |row, r|
          row.each_with_index do |v, c|
            throw :found, [r, c] if v == 5
          end
        end
        nil
      end
      assert_equal [1, 1], found
    end

    def test_results
      assert_equal 42, catch(:x) { 42 }
      assert_nil catch(:x) { throw :x }
      r = catch(:done) do
        10.times { |i| throw :done, i * 10 if i == 3 }
        :never
      end
      assert_equal 30, r
      assert_equal 7, catch { |tag| throw tag, 7 }
    end

    def test_typed_methods_throw
      assert_equal 3, catch(:neg) { control_pos(3) }
      assert_equal(-3, catch(:neg) { control_pos(-3) })
      assert_equal(-4, catch(:neg) { control_pos_ternary(-4) })
      assert_equal(-5, catch(:neg) { control_pos_if(-5) + 1 })
      assert_equal(-6, catch(:neg) { [1, 2, -6].map { |v| control_pos_if(v) } })
    end

    def test_innermost_matching_tag
      log = [] #: Array[String]
      v = catch(:a) do
        catch(:b) do
          catch(:a) { throw :a, 1 }
          log << "after inner a"
          throw :b, 2
        end
        log << "after b"
        3
      end
      assert_equal 3, v
      assert_equal ["after inner a", "after b"], log
    end

    def test_ensure_runs_rescue_passes
      log = [] #: Array[String]
      v = catch(:outer) do
        catch(:inner) do
          begin
            throw :outer, "deep"
          rescue Exception
            log << "rescued"
          ensure
            log << "ensure"
          end
        end
        "not reached"
      end
      assert_equal "deep", v
      assert_equal ["ensure"], log
    end

    def test_uncaught
      e = assert_raises(UncaughtThrowError) { throw :nope, 1 }
      assert_equal "uncaught throw :nope", e.message
      assert_equal :nope, e.tag
      assert_equal 1, e.value
      a = assert_raises(ArgumentError) { catch(:other) { throw :nope } }
      assert_equal "uncaught throw :nope", a.message
    end

    # catch tags are per thread and per fiber, with no lock shared between them
    def test_per_thread_and_fiber
      r = catch(:x) do
        t = Thread.new do
          throw :x, 9
        rescue UncaughtThrowError => e
          "thread: #{e.message}"
        end
        t.value
      end
      f = catch(:y) do
        Fiber.new do
          throw :y
        rescue UncaughtThrowError => e
          "fiber: #{e.message}"
        end.resume
      end
      threads = 4.times.map { |i| Thread.new { catch(:k) { throw :k, i * 10 } } }
      assert_equal ["thread: uncaught throw :x", "fiber: uncaught throw :y", [0, 10, 20, 30]], [r, f, threads.map(&:value)]
    end
  end

  # Operands run left to right even when a later one hoists a temporary (`x&.y`) (decision 98).
  class ControlEvalOrderTest < Minitest::Test
    def test_args_and_array_elements
      log = [] #: Array[Integer]
      h = { a: 1 } #: Hash[Symbol, Integer]
      assert_equal [2, 3], [log.push(2).size, log.last&.succ].then { |a| [a[0] + 1, a[1] || 0] }
      log.clear
      assert_equal [1, 2], [log.push(1).size, log.last&.succ]
      assert_equal [1, 2, 2], [log.size, h[:a]&.succ, log.push(9).size]
      assert_equal "1 8", format("%d %d", log.shift, log.first&.pred || 0)
    end
  end

  # ruby/spec core/kernel gaps (#49)
  class ControlRubySpecKernelTest < Minitest::Test
    def test_loop
      i = 0
      loop do
        i += 1
        next if i == 1
        break if i > 3
      end
      assert_equal 4, i
    end

    def test_loop_returns_from_method
      assert_equal 3, control_loop_find([1, 3, 5])
    end

    def test_itself_and_yield_self
      assert_equal 1, 1.itself
      assert_equal({ 1 => [1, 1], 2 => [2] }, [1, 1, 2].group_by(&:itself))
      assert_equal 3, 2.yield_self { |x| x + 1 }
    end

    def test_fail
      e = assert_raises(RuntimeError) { fail "boom" }
      assert_equal "boom", e.message
      e2 = assert_raises(ArgumentError) { fail ArgumentError, "bad" }
      assert_equal "bad", e2.message
    end

    #: (Array[Integer]) -> Integer
    def control_loop_find(xs)
      i = 0
      loop do
        x = xs[i] || 0
        return x if x > 2

        i += 1
      end
      -1
    end
  end

  # ruby/spec language gaps (#49): subject-less case, begin/end while, begin/else
  class ControlRubySpecLanguageTest < Minitest::Test
    #: (Integer) -> String
    def control_size_of(x)
      case
      when x > 100, x < -100 then "huge"
      when x > 10 then "big"
      else "small"
      end
    end

    def test_case_without_subject
      assert_equal %w[huge huge big small], [control_size_of(500), control_size_of(-500), control_size_of(50), control_size_of(1)]
      y = nil #: Integer?
      r = case
          when y.nil? then "none"
          end
      assert_equal "none", r
    end

    def test_begin_end_while
      i = 0
      begin
        i += 1
      end while i < 0
      j = 10
      begin
        j -= 1
      end until j < 3
      seen = [] #: Array[Integer?]
      q = [3, 2, 1]
      begin
        x = q.shift
        seen << x
      end while x && x > 1
      assert_equal [1, 2, [3, 2, 1]], [i, j, seen]
    end

    # a skipped condition would loop forever here
    def test_next_in_begin_end_while
      i = 0
      odd = [] #: Array[Integer]
      begin
        i += 1
        next if i.even?

        odd << i
      end while i < 5
      j = 0
      begin
        j += 1
        next
      end until j >= 3
      log = [] #: Array[String]
      q = [1, 2, 3]
      begin
        x = q.shift
        case x
        when 2 then next
        end
        log << "x#{x}"
      end while x && x < 3
      assert_equal [[1, 3, 5], 5, 3, ["x1", "x3"]], [odd, i, j, log]
    end

    # #52: redo in a block compiled as a closure reruns it with the same arguments
    def test_redo_in_closure_block
      tries = 0
      r = [1, 2, 3].map do |x|
        tries += 1
        redo if x == 2 && tries < 4
        x * 10
      end
      assert_equal [[10, 20, 30], 5], [r, tries]
    end

    def test_next_in_begin_end_while_nested
      out = [] #: Array[String]
      k = 0
      tries = 0
      redone = false
      begin
        k += 1
        if k == 2 && !redone
          redone = true
          redo
        end
        m = 0
        while m < 3
          m += 1
          next if m == 2

          out << "#{k}w#{m}"
        end
        [1, 2].each do |v|
          next if v == 1

          out << "#{k}e#{v}"
        end
        begin
          tries += 1
          raise "again" if k == 3 && tries < 5
          next if k == 1
        rescue
          retry
        end
        break if k == 4

        out << "#{k}end"
      end until k > 9
      assert_equal ["1w1", "1w3", "1e2", "3w1", "3w3", "3e2", "3end", "4w1", "4w3", "4e2"], out
      assert_equal [4, 6, true], [k, tries, redone]
    end

    #: (String) -> Integer?
    def control_parse(s)
      x = Integer(s)
    rescue ArgumentError
      nil
    else
      x * 2
    end

    #: (Integer) -> String
    def control_flow(n)
      log = [] #: Array[String]
      begin
        log << "body"
        raise "boom" if n == 1
        return "early" if n == 2
      rescue
        log << "rescue"
      else
        log << "else"
      ensure
        log << "ensure"
      end
      log.join(",")
    end

    def test_begin_else
      assert_equal [42, nil], [control_parse("21"), control_parse("zz")]
      assert_equal ["body,else,ensure", "body,rescue,ensure", "early"], [control_flow(0), control_flow(1), control_flow(2)]
      e = assert_raises(ArgumentError) do
        begin
          1
        rescue
          2
        else
          raise ArgumentError, "from else"
        end
      end
      assert_equal "from else", e.message
      v = begin
        10
      rescue
        0
          else
            20
      end
      assert_equal 20, v
    end

    def test_source_position
      line = __LINE__
      assert_equal line + 1, __LINE__
      assert_equal :test_source_position, __method__
      assert_equal [:test_source_position], [1].map { |_| __method__ }
      assert_equal File.dirname(File.expand_path(__FILE__)), __dir__
    end

    def test_interpolated_symbol
      x = "dyn"
      assert_equal [:dyn_sym, :plain2], [:"#{x}_sym", :"plain#{1 + 1}"]
    end
  end

  # ruby/spec language gaps (#49): retry, for, defined?
  class ControlRubySpecLoopsTest < Minitest::Test
    #: () -> Integer
    def control_fetch_with_retry
      tries = 0
      begin
        tries += 1
        raise "x" if tries < 4

        tries * 10
      rescue
        retry if tries < 5
        -1
      end
    end

    #: (Integer) -> String
    def control_give_up(limit)
      n = 0
      begin
        n += 1
        raise IOError, "nope"
      rescue IOError
        retry if n < limit
        "gave up after #{n}"
      ensure
        n += 100
      end
    end

    def test_retry
      attempts = 0
      seen = [] #: Array[String]
      begin
        attempts += 1
        raise ArgumentError, "flaky #{attempts}" if attempts < 3
      rescue ArgumentError => e
        seen << e.message
        retry
      end
      assert_equal [3, ["flaky 1", "flaky 2"]], [attempts, seen]
      assert_equal [40, "gave up after 3"], [control_fetch_with_retry, control_give_up(3)]
    end

    #: (Array[Integer]) -> Integer
    def control_first_big(xs)
      for x in xs
        return x if x > 10
      end
      0
    end

    def test_for
      sum = 0
      for i in 1..4
        next if i == 2

        sum += i
        last = i
      end
      assert_equal [8, 4, 4], [sum, i, last]
      pairs = [[1, 2], [3, 4]] #: Array[[Integer, Integer]]
      totals = [] #: Array[Integer]
      for a, b in pairs
        totals << a + b
      end
      seen = [] #: Array[untyped]
      for k, v in { x: 1, y: 2 }
        seen << [k, v]
      end
      for w in %w[a b c]
        break if w == "b"
      end
      assert_equal [[3, 7], [[:x, 1], [:y, 2]], "b"], [totals, seen, w]
      assert_equal [20, 0], [control_first_big([1, 20, 30]), control_first_big([1])]
    end

    #: () -> Integer
    def control_helper = 1

    def test_defined
      x = 1
      assert_equal ["local-variable", nil, "constant", nil, "method", nil], [defined?(x), defined?(y), defined?(String), defined?(Nope), defined?(puts), defined?(nope_method)]
      assert_equal ["method", nil, "method", "method", "self", "nil", "true", "assignment", "expression"], [defined?(x.succ), defined?(x.nope), defined?("a".upcase), defined?(1 + 1), defined?(self), defined?(nil), defined?(true), defined?(z = 2), defined?(3)]
      assert_equal ["method", "method", nil, "method", nil, "method"], [defined?(control_helper), defined?(String.new), defined?(Nope.x), defined?(Integer.sqrt), defined?(Integer.nope), defined?(File.join)]
      r = defined?(x)
      assert_equal true, r.frozen?
      assert_equal ["global-variable", "global-variable", "global-variable", "global-variable", nil], [defined?($stdout), defined?($0), defined?($;), defined?($VERBOSE), defined?($control_never_set)]
    end

    # A guard on a constant no file defines always returns, so what follows (naming it) is never compiled
    #: () -> String?
    def control_absent_guard
      return unless defined?(ControlNoSuchGem)

      ControlNoSuchGem.paths.home
    end

    def test_defined_guard_return
      assert_nil control_absent_guard
    end

    #: (Hash[String, Integer] blocks, Array[String] keys) -> Integer?
    def control_or_assign(blocks, keys)
      keys.each do |key|
        next if key.empty? || (blk = blocks[key]).nil?

        return blk * 10
      end
      nil
    end

    # A local assigned on the right of `||` in a condition, read after it (Homebrew's Cask::DSL#language_eval)
    def test_condition_or_assigns
      assert_equal [20, nil], [control_or_assign({ "b" => 2 }, ["", "z", "b"]), control_or_assign({}, ["a"])]
    end
  end

  # ruby/spec language gaps (#49): when *LIST and rescue *ERRORS
  CONTROL_VOWELS = %w[a e i o u]
  CONTROL_RANGES = [1..3, 10..12]
  CONTROL_NET_ERRORS = [IOError, ArgumentError]

  class ControlRubySpecSplatTest < Minitest::Test
    #: (String) -> Symbol
    def control_kind(c)
      case c
      when *CONTROL_VOWELS then :vowel
      when "y", *%w[w h] then :semi
      else :consonant
      end
    end

    #: (Integer) -> bool
    def control_in_ranges?(n)
      case n
      when *CONTROL_RANGES then true
      else false
      end
    end

    def test_when_splat
      assert_equal %i[vowel semi semi consonant], [control_kind("e"), control_kind("y"), control_kind("h"), control_kind("z")]
      assert_equal [true, true, false], [control_in_ranges?(2), control_in_ranges?(11), control_in_ranges?(5)]
    end

    def test_rescue_splat
      seen = [] #: Array[untyped]
      [1, 2, 3].each do |i|
        begin
          raise IOError, "io" if i == 1
          raise ArgumentError, "arg" if i == 2

          raise TypeError, "type"
        rescue *CONTROL_NET_ERRORS => e
          seen << [:net, e.message]
        rescue TypeError, *[KeyError] => e
          seen << [:other, e.message]
        end
      end
      assert_equal [[:net, "io"], [:net, "arg"], [:other, "type"]], seen
    end
  end

  # ruby/spec core/exception gaps (#49): detailed_message, errno, NameError#name and #receiver
  class ControlCustomDetail < StandardError
    #: (?highlight: bool) -> String
    def detailed_message(highlight: false) = "custom"
  end

  class ControlRubySpecExceptionTest < Minitest::Test
    def test_detailed_message
      assert_equal ["boom (RuntimeError)", "unhandled exception", "a (RuntimeError)\nb", "boom (RuntimeError)"],
                   [RuntimeError.new("boom").detailed_message, RuntimeError.new("").detailed_message, RuntimeError.new("a\nb").detailed_message, RuntimeError.new("boom").detailed_message(highlight: false)]
      assert_equal ["abc (StandardError)", "a (StandardError)\n\nb", " (StandardError)\nfoo", "a (StandardError)\nb\n", " (StandardError)", "StandardError", "abc\r (StandardError)"],
                   ["abc\n", "a\n\nb", "\nfoo", "a\nb\n", "\n", "", "abc\r\n"].map { |m| StandardError.new(m).detailed_message }
      assert_equal ["\e[1ma (\e[1;4mStandardError\e[m\e[1m)\e[m\n\e[1mb\e[m\n\e[1mc\e[m\n", "\e[1;4mStandardError\e[m", "\e[1;4munhandled exception\e[m"],
                   [StandardError.new("a\nb\nc\n").detailed_message(highlight: true), StandardError.new("").detailed_message(highlight: true), RuntimeError.new("").detailed_message(highlight: true)]
    end

    # Exception#full_message (#55, decision 139): MRI's report, no error_highlight snippet
    def test_full_message_without_backtrace
      line = __LINE__ + 1
      got = [RuntimeError.new("made").full_message(highlight: false), StandardError.new("").full_message(highlight: false, order: :bottom), RuntimeError.new("").full_message]
      assert_equal ["#{__FILE__}:#{line}:in 'full_message': made (RuntimeError)\n", "Traceback (most recent call last):\n#{__FILE__}:#{line}:in 'full_message': StandardError\n", "#{__FILE__}:#{line}:in 'full_message': unhandled exception\n"], got
    end

    def test_full_message_with_backtrace
      e = StandardError.new("x")
      e.set_backtrace(["a.rb:1:in 'f'", "a.rb:2:in 'g'"])
      assert_equal "a.rb:1:in 'f': x (StandardError)\n\tfrom a.rb:2:in 'g'\n", e.full_message(highlight: false)
      assert_equal "Traceback (most recent call last):\n\t1: from a.rb:2:in 'g'\na.rb:1:in 'f': x (StandardError)\n", e.full_message(highlight: false, order: :bottom)
      assert_equal "a.rb:1:in 'f': \e[1mx (\e[1;4mStandardError\e[m\e[1m)\e[m\n\tfrom a.rb:2:in 'g'\n", e.full_message(highlight: true, order: :top)
      assert_equal "\e[1mTraceback\e[m (most recent call last):\n\t1: from a.rb:2:in 'g'\na.rb:1:in 'f': \e[1mx (\e[1;4mStandardError\e[m\e[1m)\e[m\n", e.full_message(highlight: true, order: :bottom)
      long = StandardError.new("y\nz")
      long.set_backtrace((1..11).map { |i| "b.rb:#{i}" })
      want = "Traceback (most recent call last):\n" + (2..11).reverse_each.map { |i| "\t#{(i - 1).to_s.rjust(2)}: from b.rb:#{i}\n" }.join + "b.rb:1: y (StandardError)\nz\n"
      assert_equal want, long.full_message(highlight: false, order: :bottom)
      assert_equal "c.rb:1: custom\n", ControlCustomDetail.new("q").tap { |c| c.set_backtrace(["c.rb:1"]) }.full_message(highlight: false)
    end

    def test_full_message_cause
      e = assert_raises(IOError) do
        begin
          raise "inner"
        rescue => inner
          inner.set_backtrace(["i.rb:1:in 'x'"])
          raise IOError, "outer"
        end
      end
      e.set_backtrace(["o.rb:1:in 'y'", "o.rb:2:in 'z'"])
      assert_equal "o.rb:1:in 'y': outer (IOError)\n\tfrom o.rb:2:in 'z'\ni.rb:1:in 'x': inner (RuntimeError)\n", e.full_message(highlight: false)
      assert_equal "Traceback (most recent call last):\ni.rb:1:in 'x': inner (RuntimeError)\n\t1: from o.rb:2:in 'z'\no.rb:1:in 'y': outer (IOError)\n", e.full_message(highlight: false, order: :bottom)
    end

    #: () -> void
    def control_fm_raise = raise(IOError, "boom")

    def test_full_message_raised
      e = assert_raises(IOError) { control_fm_raise }
      first = e.full_message(highlight: false).lines.first || ""
      assert_equal "#{e.backtrace&.first}: boom (IOError)\n", first
      assert_match(/control_test\.rb:\d+:in 'ControlTests::ControlRubySpecExceptionTest#control_fm_raise': boom/, first)
      err = assert_raises(ArgumentError) { e.full_message(order: :foo) }
      assert_equal "expected :top or :bottom as order: :foo", err.message
    end

    def test_errno
      no = nil #: Integer?
      begin
        File.read("/nope/nope")
      rescue SystemCallError => e
        no = e.errno
      end
      assert_equal 2, no
      assert_nil SystemCallError.new("x").errno
    end

    def test_name_error_call
      x = nil #: untyped
      e = assert_raises(NoMethodError) { x.upcase }
      assert_equal [:upcase, nil], [e.name, e.receiver]
      y = 5 #: untyped
      e2 = assert_raises(NoMethodError) { y.nope }
      assert_equal [:nope, 5], [e2.name, e2.receiver]
      e3 = assert_raises(ArgumentError) { NameError.new("plain").receiver }
      assert_equal "no receiver is available", e3.message
      assert_nil NameError.new("plain").name
    end
  end

  # a nil-returning call as a block's value: method expressions like (*File).Close(f) kept their parens
  class ControlBugVoidCallValueTest < Minitest::Test
    def test_void_call_as_block_value
      v = [1].map { |_| File.open("/dev/null").close }
      assert_equal [nil], v
      r = Dir.mktmpdir do |d|
        f = File.open(File.join(d, "x"), "w")
        f.close
      end
      assert_nil r
    end
  end

  # $? inside another class's method compiled to a dynamic call on main that could not see Kernel's private helper
  class ControlBugLastStatusInMethodTest < Minitest::Test
    def test_last_status_in_method
      system("true")
      assert_equal [true, 0], [$?&.success?, $?&.exitstatus]
    end
  end

  # ruby/spec language gaps (#49): redo
  class ControlRubySpecRedoTest < Minitest::Test
    def test_redo
      seen = [] #: Array[[Integer, Integer]]
      tries = 0
      [1, 2].each do |x|
        tries += 1
        redo if x == 2 && tries < 4
        seen << [x, tries]
      end
      i = 0
      n = 0
      while i < 3
        i += 1
        n += 1
        redo if n == 2
        seen << [i, n]
      end
      k = 0
      for v in [10, 20]
        k += 1
        redo if k == 1
        seen << [v, k]
      end
      assert_equal [[1, 1], [2, 4], [1, 1], [3, 3], [10, 2], [20, 3]], seen
    end
  end

  # an optional block (?{ }) is a Proc?: blk&.call, block_given? and `if blk` check it; it used to be called unchecked
  class ControlOptionalBlockTest < Minitest::Test
    #: (Integer) ?{ (Integer) -> Integer } -> Integer?
    def control_safe(x, &blk) = blk&.call(x)

    #: (Integer) ?{ (Integer) -> Integer } -> Integer
    def control_guarded(x)
      return x unless block_given?

      yield x
    end

    #: () ?{ () -> String } -> String
    def control_either(&b)
      if b
        b.call
      else
        "none"
      end
    end

    #: () ?{ () -> String } -> bool
    def control_given = block_given?

    #: () ?{ () -> void } -> String?
    def control_def_yield = defined?(yield)

    #: () { () -> void } -> String?
    def control_def_yield_required = defined?(yield)

    #: () -> String?
    def control_def_yield_none = defined?(yield)

    #: () ?{ () -> void } -> Array[String?]
    def control_def_yield_in_block = [1].map { defined?(yield) }

    def test_defined_yield
      assert_equal [nil, "yield", "yield", nil], [control_def_yield, control_def_yield {}, control_def_yield_required {}, control_def_yield_none]
      assert_equal [[nil], ["yield"]], [control_def_yield_in_block, control_def_yield_in_block {}]
    end

    #: (Integer) ?{ (Integer) -> Integer } -> Integer
    def control_unchecked(x) = yield(x + 1)

    #: (Array[Integer]) ?{ (Integer, Integer) -> void } -> void
    def control_unchecked_pairs(xs)
      xs.each { |a| yield a, a * 2 }
    end

    #: (bool) ?{ (Integer) -> Integer } -> Integer
    def control_unchecked_and(c) = c && yield(1) > 0 ? 2 : 3

    # MRI evaluates yield's arguments, then raises LocalJumpError (decision 132)
    def test_yield_without_block
      assert_equal 30, control_unchecked(2) { |v| v * 10 }
      seen = [] #: Array[Integer]
      e = assert_raises(LocalJumpError) { control_unchecked(seen.push(1).size) }
      assert_equal ["no block given (yield)", [1]], [e.message, seen]
      out = [] #: Array[Array[Integer]]
      control_unchecked_pairs([1, 2]) { |a, b| out << [a, b] }
      assert_equal [[1, 2], [2, 4]], out
      assert_raises(LocalJumpError) { control_unchecked_pairs([1]) }
      control_unchecked_pairs([])
      assert_equal [3, 2], [control_unchecked_and(false), control_unchecked_and(true) { |v| v }]
      assert_equal StandardError, LocalJumpError.superclass
    end

    def test_optional_block
      assert_equal [nil, 20], [control_safe(1), control_safe(2) { |v| v * 10 }]
      assert_equal [3, 4], [control_guarded(3), control_guarded(3) { |v| v + 1 }]
      assert_equal ["none", "some"], [control_either, control_either { "some" }]
      assert_equal [false, true], [control_given, control_given { "x" }]
    end
  end

  # a local read only as a statement (`{ |x| x }`, `z` alone) was a Go "declared and not used" error
  class ControlBugLoneLocalReadTest < Minitest::Test
    def test_lone_local_read
      n = 0
      [1, 2].each { |x| x }
      (1..2).each do |y|
        y
        n += 1
      end
      z = 5
      z
      assert_equal 2, n
    end
  end

  # a Kernel method called on a lambda was a dynamic call: NoMethodError at run time
  class ControlBugProcKernelTest < Minitest::Test
    def test_kernel_method_on_proc
      f = -> { 1 }
      assert_equal "<Proc>", f.control_tag
      assert_equal false, f.frozen?
      x = f #: untyped
      assert_equal Proc, x.class
    end
  end

  # #53: defined?(X) is nil until X's assignment has run
  def self.later_defined = defined?(CONTROL_LATER_K)
  CONTROL_EARLY_DEFINED = later_defined
  CONTROL_LATER_K = 1

  class ControlDefinedGuardTest < Minitest::Test
    def test_defined_before_assignment
      assert_equal [nil, "constant"], [CONTROL_EARLY_DEFINED, ControlTests.later_defined]
    end
  end

  # A multiple assignment writes through []= and attribute setters (#82).
  class ControlSetterPoint
    attr_accessor :x #: Integer
    attr_accessor :y #: String

    #: () -> void
    def initialize
      @x = 0
      @y = ""
    end
  end

  class ControlMultiSetterTest < Minitest::Test
    def test_index_targets
      arr = [1, 2, 3]
      arr[0], arr[2] = 9, 8
      assert_equal [9, 2, 8], arr
      h = { "a" => 1 }
      h["a"], h["b"] = h.fetch("a") + 1, 5
      assert_equal({ "a" => 2, "b" => 5 }, h)
    end

    def test_attribute_targets
      pt = ControlSetterPoint.new
      pt.x, pt.y = 4, "four"
      assert_equal [4, "four"], [pt.x, pt.y]
    end

    def test_mixed_with_locals
      h = {} #: Hash[String, Integer]
      k, h["z"] = [7, 3]
      assert_equal [7, { "z" => 3 }], [k, h]
    end
  end

  # A block declared `{ (*untyped) -> void }` takes what yield passes, splats spliced in (#82).
  class ControlRestRouter
    #: (Array[String]) -> void
    def initialize(caps)
      @caps = caps
    end

    #: () { (*untyped) -> void } -> void
    def each_capture
      yield(*@caps)
      yield(1, *@caps, "z")
    end
  end

  # A branch guarded by defined? of a constant no file defines is dropped (decision 167).
  CONTROL_PARSER = defined?(::ControlTests::NoSuchParser) ? ::ControlTests::NoSuchParser : :fallback

  class ControlRestBlockTest < Minitest::Test
    def test_named_params_bind_from_rest
      got = [] #: Array[untyped]
      ControlRestRouter.new(["7", "x"]).each_capture { |id, rest| got << [id, rest] }
      assert_equal [["7", "x"], [1, "7"]], got
    end

    def test_rest_param_collects
      got = [] #: Array[untyped]
      ControlRestRouter.new(["a"]).each_capture { |*all| got << all }
      assert_equal [["a"], [1, "a", "z"]], got
    end

    def test_absent_constant_guard
      assert_equal :fallback, CONTROL_PARSER
    end
  end

  class ControlLastMatchTest < Minitest::Test
    #: (String) -> Array[String?]
    def split_url(location)
      if location =~ %r{\Ahttps?://(.*?)(/.*)}
        [$1, $2, $&]
      else
        [nil, location, $&]
      end
    end

    def test_numbered_groups
      assert_equal ["example.com", "/a", "http://example.com/a"], split_url("http://example.com/a")
      assert_equal [nil, "/plain", nil], split_url("/plain")
    end

    def test_last_match_in_block
      assert_equal ["1", nil], %w[x1 y].map { |w| w =~ /(\d)/ ? $1 : nil }
    end

    def test_regexp_on_the_left
      s = "abc"
      assert_equal [1, "b"], [/b/ =~ s, $~&.[](0)]
    end

    #: (String?) -> Array[untyped]
    def pair(s)
      return nil, nil if s.nil?

      a, b = s.split(":")
      return a, b&.to_i
    end

    def test_return_many
      assert_equal [["h", 80], [nil, nil]], [pair("h:80"), pair(nil)]
    end

    #: (Array[String]) -> String?
    def control_last(xs) = xs.last

    def test_assignments_in_and_narrow
      got = if (v = control_last(["r"])) && (w = control_last([v]))
              w.upcase
            end
      assert_equal "R", got
    end
  end

  class ControlLastMatchSetTest < Minitest::Test
    #: (String) -> String?
    def control_quoted(s)
      i = s.index(/(["\\])/)
      i ? $1 : nil
    end

    def test_index_sets_last_match
      assert_equal ["\"", nil], [control_quoted("a\"b"), control_quoted("ab")]
    end

    def test_match_sets_last_match
      m = "k=v".match(/(\w)=(\w)/)
      assert_equal ["k", "v", "k"], [$1, $2, m&.[](1)]
    end
  end

  class ControlVerboseTest < Minitest::Test
    def test_verbose_global
      was = $VERBOSE
      verbose, $VERBOSE = $VERBOSE, nil
      silenced = $VERBOSE
      $VERBOSE = verbose
      assert_equal [false, nil, false], [was, silenced, $VERBOSE]
    end

    def test_kernel_array
      h = nil #: untyped
      assert_equal [[], [1, 2], ["a"]], [Array(h), Array([1, 2]), Array("a")]
    end

    # a parameter narrowed by `||=` and then reassigned stays readable and non-nil
    #: (?String?) -> String
    def control_refill(buffer = nil)
      buffer ||= ""
      buffer = buffer + "x"
      buffer
    end

    def test_narrowed_parameter_reassigned
      assert_equal ["x", "ax"], [control_refill, control_refill("a")]
    end

    def test_include_untyped_nil
      v = nil #: untyped
      assert_equal [false, true], [%w[yes on].include?(v), %w[yes on].include?("on")]
    end
  end

  ControlRaiseAlias = KeyError

  # What rack's request and query parser raise (decision 175).
  class ControlReraiseTest < Minitest::Test
    def control_reraise
      Integer("x")
    rescue ArgumentError
      raise
    end

    def control_reraise_as(e)
      raise e.class, "again: #{e.message}", cause: nil
    end

    def test_bare_raise_reraises
      e = assert_raises(ArgumentError) { control_reraise }
      assert_equal "invalid value for Integer(): \"x\"", e.message
    end

    def test_raise_class_message_backtrace
      e = assert_raises(KeyError) { raise KeyError, "k", ["here:1"] }
      assert_equal ["k", ["here:1"]], [e.message, e.backtrace]
    end

    def test_raise_computed_class
      e = assert_raises(IndexError) { control_reraise_as(IndexError.new("boom")) }
      assert_equal "again: boom", e.message
      x = RangeError.new("r")
      assert_equal [true, "r", "s"], [x.exception.equal?(x), x.message, x.exception("s").message]
    end

    def test_raise_constant_alias
      assert_raises(KeyError) { raise ControlRaiseAlias }
    end

    def test_and_assign_ivar
      @control_n = 1 #: Integer?
      @control_n &&= @control_n.to_i + 1
      @control_m = nil #: Integer?
      @control_m &&= 5
      assert_equal [2, nil], [@control_n, @control_m]
    end

    def test_errno_emfile
      assert_equal [true, 24], [Errno::EMFILE.new.is_a?(SystemCallError), Errno::EMFILE.new.errno]
    end

    def test_noreturn_argument
      a = assert_raises(ArgumentError) { control_show(control_die) }
      b = assert_raises(KeyError) { control_show(raise(KeyError, "k")) }
      c = assert_raises(ArgumentError) do
        out = control_die
        control_show(out)
      end
      assert_equal ["died", "k", "died"], [a.message, b.message, c.message]
    end
  end

  # a local assigned in a condition of a constant's initializer (`X = if (p = a).empty? || (p = b).empty?`)
  CONTROL_PICKED = if (control_pick = "").empty? && (control_pick = "/tmp").empty?
                     control_pick.upcase
                   else
                     control_pick
                   end.freeze

  class ControlBeginValue
    #: () -> void
    def initialize
      @memo = nil #: Integer?
      @calls = 0
    end

    #: () -> Integer
    def memo
      @memo ||= begin
        v = 20
        v + 1
      end
    end

    #: () -> Integer
    def counted
      @calls += 1
      self
      @calls
    end

    #: () -> ControlBeginValue
    def bump
      @calls += 1
      self
    end

    #: (*Integer) -> Integer
    def ignore_rest(*_) = 7
  end

  class ControlBeginValueTest < Minitest::Test
    def test_begin_locals_as_value
      x = begin
        v = 1
        v + 1
      end
      assert_equal 2, x
      assert_equal 21, ControlBeginValue.new.memo
    end

    def test_const_init_condition_local
      assert_equal "/tmp", CONTROL_PICKED
    end

    def test_class_keeps_receiver_effects
      b = ControlBeginValue.new
      assert_equal ControlBeginValue, b.bump.class
      assert_equal 2, b.counted
    end

    def test_anonymous_rest
      assert_equal 7, ControlBeginValue.new.ignore_rest(1, 2)
    end
  end

  # `a, b = pair_or_nil`: nil destructures into nils (decision 222).
  class ControlOptionalTupleTest < Minitest::Test
    #: (bool) -> [String, Integer?]?
    def control_pair(ok) = ok ? ["x", nil] : nil

    def test_optional_tuple
      h = { a: 1, b: 2 }
      k, v = h.find { |_, x| x > 1 }
      k2, v2 = h.find { |_, x| x > 5 }
      a, b = control_pair(true)
      c, d = control_pair(false)
      assert_equal [:b, 2, nil, nil, "x", nil, nil, nil], [k, v, k2, v2, a, b, c, d]
    end
  end

  # A yield inside a block the method passes on: a call of the closure block, or (in an iterator) Go's yield, a
  # consumer's break unwinding through it (decision 226).
  class ControlNestedYieldTest < Minitest::Test
    #: () { (Integer) -> void } -> void
    def control_each_locked
      m = Thread::Mutex.new
      [1, 2, 3].each { |x| m.synchronize { yield x } }
    end

    #: (Array[Integer]) { (Integer) -> Integer } -> Array[Integer]
    def control_via(nums) = nums.map { |n| yield(n) }

    def test_nested_yield
      out = [] #: Array[Integer]
      control_each_locked { |x| out << x }
      control_each_locked do |x|
        break if x == 2

        out << (x * 10)
      end
      assert_equal [1, 2, 3, 10], out
      assert_equal [2, 3], control_via([1, 2]) { |n| n + 1 }
    end
  end

  # `[*x]` of a non-Array: x.to_a when it answers to_a, nothing for nil, else x itself; `x &&= v` on a local
  # (decision 241).
  class ControlSplatValueTest < Minitest::Test
    #: (String, Symbol?, [Integer, Integer]) -> [Array[String], Array[Symbol], Array[Integer]]
    def control_splats(s, sym, pair) = [[*s, "z"], [:x, *sym], [0, *pair]]

    def test_splat_non_array
      assert_equal [["u", "z"], [:x], [0, 1, 2]], control_splats("u", nil, [1, 2])
      assert_equal [:x, :y], control_splats("u", :y, [1, 2])[1]
      m = "ab".match(/(a)(c)?/)
      assert_equal [["a", "a", nil], [], [["k", 1]], [1, 2]], [[*m], [*"zz".match(/q/)], [*{ "k" => 1 }], [*1..2]]
    end

    def test_local_and_write
      ok = true
      [1, -2, 3].each { |x| ok &&= x > 0 }
      s = "a" #: String?
      s &&= s + "b"
      t = nil #: String?
      t &&= t + "b"
      assert_equal [false, "ab", nil], [ok, s, t]
    end
  end

  # `break` in a closure block leaves the call statement that passed it; ensure runs (decision 241).
  class ControlClosureBreak
    def initialize
      @m = Thread::Mutex.new
      @running = true
      @log = [] #: Array[String]
    end

    attr_reader :log

    #: [T] () { () -> T } -> T
    def synchronize = @m.synchronize { yield }

    #: () -> bool
    def shutdown
      synchronize do
        break unless @running

        @running = false
        @log << "stopped"
      end
      @log << "after"
      true
    end
  end

  class ControlClosureBreakTest < Minitest::Test
    def test_closure_break
      s = ControlClosureBreak.new
      s.shutdown
      s.shutdown
      got = [] #: Array[String]
      begin
        [1].map do |x|
          break if x == 1

          x
        end
        got << "after map"
      ensure
        got << "ensure"
      end
      assert_equal [["stopped", "after", "after"], ["after map", "ensure"]], [s.log, got]
      control_guarded do |i|
        break if i > 1

        got << i.to_s
      end
      assert_equal ["after map", "ensure", "1"], got
    end

    #: () { (Integer) -> void } -> void
    def control_guarded
      [1, 2].each { |i| yield i }
    rescue => e
      raise e
    end

    def test_symbol_block_of_two
      hs = [{ a: 1 }, { b: 2 }, { a: 3 }] #: Array[Hash[Symbol, Integer]]
      assert_equal [{ a: 3, b: 2 }, 6, [1, 2, 3]], [hs.reduce(&:merge), [1, 2, 3].reduce(&:+), [[1, 2], [3]].inject(&:concat)]
    end

    def test_mixed_array_plus
      m = { "a" => :a }.freeze #: Hash[String, Symbol]
      h = { a: 1 } #: Hash[Symbol, Integer]
      assert_equal [[:a, "gcc-8"], [1, "x"], { a: 1, b: "x" }], [m.values + %w[8].map { |n| "gcc-#{n}" }, [1] | ["x"], h.merge({ b: "x" })]
    end

    def test_tuple_queries
      pair = [1, "a"]
      assert_equal [2, "1,a", true], [pair.size, pair.join(","), pair.include?("a")]
    end
  end

  # Decision 241: `raise err, msg, bt` raises err.exception(msg); `e = nil` widened by a later write raises;
  # destructuring a union or an optional tuple.
  class ControlTapErr < StandardError
    #: (String) -> void
    def initialize(name)
      @name = name
      super("tap #{name}")
    end

    attr_reader :name
  end

  class ControlRaiseDestructureTest < Minitest::Test
    #: () -> void
    def control_reraise
      raise ArgumentError, "inner"
    rescue ArgumentError => e
      raise ControlTapErr.new("x"), "re", e.backtrace
    end

    #: (Integer) -> String
    def control_pending(n)
      err = nil
      begin
        raise ControlTapErr, "p" if n == 1
      rescue ControlTapErr => e
        err ||= e
      end
      raise err if err

      "ok"
    end

    #: (Integer) -> (String | [String, String?] | nil)
    def control_pick(n)
      case n
      when 0 then "s"
      when 1 then ["a", "b"]
      when 2 then ["c", nil]
      end
    end

    #: (Integer) -> [String, Integer, Symbol?]?
    def control_trip(n) = n.zero? ? nil : ["t", n, :k]

    def test_raise_object_with_message
      got = [] #: Array[untyped]
      begin
        control_reraise
      rescue ControlTapErr => e
        got = [e.message, e.class]
      end
      assert_equal ["re", ControlTapErr], got
      assert_equal "ok", control_pending(0)
      assert_raises(ControlTapErr) { control_pending(1) }
    end

    def test_destructure_union_and_optional_tuple
      got = [0, 1, 2, 3].map do |i|
        path, base = control_pick(i)
        [path, base]
      end
      assert_equal [["s", nil], ["a", "b"], ["c", nil], [nil, nil]], got
      tok, num, = control_trip(1)
      first, = control_trip(0)
      assert_equal ["t", 1, nil], [tok, num, first]
    end

    # `|_, (token, _)|` over a Hash of pairs (Homebrew's Keg#link): the blank params bind nothing
    def test_blank_params_in_nested_tuple
      h = { "a" => ["x", 1], "b" => ["y", 2], "c" => ["x", 3] } #: Hash[String, [String, Integer]]
      assert_equal [["x", 2], ["y", 1]], h.group_by { |_, (tok, _)| tok }.map { |tok, pairs| [tok, pairs.size] }
    end

    # `(a, = pair_or_nil)` as a condition (cleanup.rb's autoremove): its value is the right-hand side
    def test_destructure_as_condition
      got = [0, 1].map do |i|
        if (n = i + 1) && (first, = control_trip(i)) && (up = first.to_s.upcase)
          [n, up]
        else
          :none
        end
      end
      assert_equal [:none, [2, "T"]], got
    end
  end

  # Decision 244: `class X < self`, constants assigned inside a constant's value, $! in a rescue, $1 after a `when /re/`,
  # `Array === x` narrowing, try_convert and collect.
  class ControlSwitch
    class NoArg < self
      #: () -> String
      def kind = "none"
    end

    #: () -> String
    def kind = "any"
  end

  module ControlOpt
    NoArgument = [NO_ARGUMENT = :NONE, nil].freeze
  end

  class ControlOptParseTest < Minitest::Test
    #: (String | Integer) -> String
    def control_opt(o)
      case o
      when /\A--\[no-\]([^=]*)/ then "neg #{$1}"
      when /\A--([^=]*)(=)?/ then "long #{$1} #{$2.inspect}"
      when Integer then "int"
      else "other"
      end
    end

    #: (untyped) -> Integer
    def control_size(o)
      return o.size if Array === o

      -1
    end

    def test_optparse_shapes
      assert_equal ["none", "any", ControlSwitch], [ControlSwitch::NoArg.new.kind, ControlSwitch.new.kind, ControlSwitch::NoArg.superclass]
      assert_equal [[:NONE, nil], :NONE], [ControlOpt::NoArgument, ControlOpt::NO_ARGUMENT]
      msg = begin
        raise ArgumentError, "boom"
      rescue
        $!.message
      end
      assert_equal "boom", msg
      assert_equal ["neg color", "long days \"=\"", "long json nil", "int", "other"],
                   [control_opt("--[no-]color"), control_opt("--days="), control_opt("--json"), control_opt(3), control_opt("x")]
      assert_equal [2, -1], [control_size([1, 2]), control_size("ab")]
      assert_equal ["a", nil, [1], nil, [2, 4]], [String.try_convert("a"), String.try_convert(1), Array.try_convert([1]), Array.try_convert("x"), [1, 2].collect { |x| x * 2 }]
    end

    # #130: $! is the handled exception, not nil, in every rescue form
    def test_bang_every_rescue_form
      bare = begin
        raise "x"
      rescue
        $!
      end
      assert_equal [RuntimeError, "x"], [bare.class, bare.message]
      named = begin
        raise ArgumentError, "z"
      rescue ArgumentError
        $!.message
      end
      assert_equal "z", named
      v = (raise "y" rescue $!.message)
      assert_equal "y", v
      enc = ("é".encode("Shift_JIS") rescue $!)
      assert_equal Encoding::UndefinedConversionError, enc.class
    end

    def test_thread_raise_on_current_thread
      e = begin
        Thread.current.raise(ArgumentError, "bad")
        nil
      rescue ArgumentError => ex
        ex.message
      end
      assert_equal "bad", e
    end

    def test_thread_join_nil_timeout_waits
      lim = nil #: Float?
      t = Thread.new { 2 }
      assert_equal 2, t.join(lim)&.value
    end
  end

  # #102: Exception#== compares class, message and backtrace
  class ControlExceptionEqualTest < Minitest::Test
    def test_exception_equal
      e = ArgumentError.new
      o = RuntimeError.new("m")
      o.set_backtrace ["x"]
      t = RuntimeError.new("m")
      t.set_backtrace nil
      assert_equal [true, true, true, false, false, false, false],
                   [e == e.dup, RuntimeError.new == RuntimeError.new, TypeError.new("m") == TypeError.new("m"), TypeError.new("m") == TypeError.new("n"),
                    RuntimeError.new("m") == TypeError.new("m"), ArgumentError.new == String.new, o == t]
    end
  end
end

module ControlTests
  class ControlYieldValueTest < Minitest::Test
    def control_yield_found(flag)
      found = nil
      found = (block_given? ? yield : true) if flag
      found
    end

    def test_yield_value_in_ternary
      assert_equal true, control_yield_found(true) { 1 > 0 }
      assert_equal true, control_yield_found(true)
      assert_nil control_yield_found(false) { false }
      assert_equal false, control_yield_found(true) { 1 > 2 }
    end
  end
end

module ControlTests
  CONTROL_SQ = /\A'(.*)'\z/m
  CONTROL_DQ = /\A"(.*)"\z/m
  CONTROL_INT = /\A(-?\d+)\z/

  class ControlCaseRegexpConstTest < Minitest::Test
    #: (String) -> (String | Integer | Symbol | nil)
    def control_parse(markup)
      case markup
      when CONTROL_SQ, CONTROL_DQ
        $1
      when CONTROL_INT
        $1.to_i
      else
        :var
      end
    end

    def test_when_regexp_constant_sets_last_match
      assert_equal "hi", control_parse(%("hi"))
      assert_equal "yo", control_parse("'yo'")
      assert_equal 42, control_parse("42")
      assert_equal :var, control_parse("x")
    end
  end
end

module ControlTests
  # `x and return v` / `x or next` as statements: the jump when x is truthy (falsy) (decision 334).
  class ControlLogicJumpTest < Minitest::Test
    #: (Integer) -> Symbol
    def control_sign(x)
      x.positive? and return :pos
      x.zero? or return :neg
      :zero
    end

    #: (Array[Integer]) -> nil
    def control_nil_log(calls)
      calls << 1
      nil
    end

    def test_nil_condition_still_runs
      calls = [] #: Array[Integer]
      hit = control_nil_log(calls) ? 1 : 0
      control_nil_log(calls) and calls << 99
      assert_equal [[1, 1], 0], [calls, hit]
    end

    def test_and_or_jumps
      assert_equal [:pos, :zero, :neg], [control_sign(3), control_sign(0), control_sign(-2)]
      assert_equal [1, 20, 3, 40], [1, 2, 3, 4].map { |i|
        i.even? and next i * 10
        i
      }
      seen = [] #: Array[Integer]
      [1, 2, 3].each do |v|
        v.odd? and next
        seen << v
      end
      assert_equal [2], seen
    end
  end
end

module ControlTests
  # A block with fewer names than a `(Integer, *String)` signature's leading params, or naming the rest's elements positionally.
  class ControlLeadRestYielder
    #: () { (Integer, *String) -> void } -> void
    def each_pair_rest
      yield 1, "a", "b"
    end
  end

  class ControlLeadRestTest < Minitest::Test
    def test_block_names_fewer_than_leading_params
      y = ControlLeadRestYielder.new
      n = 0
      y.each_pair_rest { n += 1 }
      got = [] #: Array[Integer]
      y.each_pair_rest { |a| got << a }
      assert_equal [1, [1]], [n, got]
    end

    def test_extra_names_are_rest_elements
      got = [] #: Array[untyped]
      y = ControlLeadRestYielder.new
      y.each_pair_rest { |a, b| got << [a, b] }
      y.each_pair_rest { |a, b, c| got << [a, b, c] }
      y.each_pair_rest { |a, b, *r| got << [a, b, r] }
      y.each_pair_rest { |a, *r| got << [a, r] }
      assert_equal [[1, "a"], [1, "a", "b"], [1, "a", ["b"]], [1, ["a", "b"]]], got
    end
  end
end

module ControlTests
  # A `when` condition that needs statements (a ternary, a modifier if) runs only when the earlier arms did not match.
  class ControlLazyWhenTest < Minitest::Test
    #: (Integer, Array[Integer]) -> Symbol
    def lazy_case(x, log)
      case x
      when 1 then :one
      when (x > 5 ? control_note(log, 3) : control_note(log, 4)) then :ternary
      when (control_note(log, 6) if x > 0) then :modifier
      when (x > 7 || control_note(log, 7) > 0 ? 8 : 9) then :logic
      else :other
      end
    end

    #: (Integer) -> [Symbol, Integer?]
    def assigned_in_when(x)
      r = case x
          when 1 then :a
          when (w = (x > 5 ? 3 : 2)) then :b
          else :c
          end
      [r, w]
    end

    def test_local_assigned_in_a_later_condition
      assert_equal [[:a, nil], [:b, 2], [:c, 3]], [assigned_in_when(1), assigned_in_when(2), assigned_in_when(7)]
    end

    def test_conditions_after_a_match_do_not_run
      log = [] #: Array[Integer]
      assert_equal [:one, []], [lazy_case(1, log), log]
    end

    def test_conditions_run_in_order_until_a_match
      log = [] #: Array[Integer]
      assert_equal [:ternary, [4]], [lazy_case(4, log), log]
      log.clear
      assert_equal [:modifier, [3, 6]], [lazy_case(6, log), log]
      log.clear
      assert_equal [:other, [4, 6, 7]], [lazy_case(2, log), log]
    end
  end
end

module ControlTests
  # decision 364: a bare raise outside a rescue re-raises the $! a caller handles, else an empty RuntimeError
  class BareRaise
    def again = raise
  end

  class BareRaiseTest < Minitest::Test
    def test_bare_raise_outside_rescue
      got = begin
        begin
          raise ArgumentError, "x"
        rescue StandardError
          BareRaise.new.again
        end
      rescue StandardError => e
        e
      end
      assert_equal [ArgumentError, "x"], [got.class, got.message]
      f = -> { raise }
      e2 = begin
        f.call
      rescue StandardError => e
        e
      end
      assert_equal [RuntimeError, ""], [e2.class, e2.message]
    end
  end
end

module ControlTests
  # decision 365: NameError.new(msg, name)
  class ExceptionArityTest < Minitest::Test
    def test_two_argument_constructors
      e = NameError.new("msg", "name")
      assert_equal ["name", "msg"], [e.name, e.message]
    end
  end
end

module ControlTests
  # decision 367: `(a, b) = x` of a T?, and defined? of an assignment or an Array literal
  class NestedOptDestructureTest < Minitest::Test
    def test_nested_destructure_and_defined
      x = [1, 2].first #: Integer?
      (a, b), c = x, 3
      assert_equal [1, nil, 3], [a, b, c]
      assert_equal "assignment", defined?(a += 1)
      assert_nil defined?([ControlNoSuchConst, Array])
      assert_equal "expression", defined?([1, Array])
    end
  end
end

module ControlTests
  # decision 369: an Array? destructures as the Array, nil as an empty one
  class OptArrayDestructureTest < Minitest::Test
    def test_opt_array_destructure
      y = [].first #: Array[Integer]?
      d, e = y
      assert_equal [nil, nil], [d, e]
      z = [[1, [2, 3]]].first
      (a, (b, c)) = z
      assert_equal [1, 2, 3], [a, b, c]
    end
  end
end

module ControlTests
  # decision 370: SystemCallError messages and arities, SignalException.new(signo, name), FrozenError#receiver,
  # NoMethodError#args
  class ExceptionConstructorsTest < Minitest::Test
    def test_system_call_error_forms
      assert_equal ["Invalid argument - x", "Invalid argument"], [Errno::EINVAL.new("x").message, Errno::EINVAL.new.message]
      assert_equal [42, -1, "Operation not permitted - message"],
                   [SystemCallError.new("message", 42).errno, SystemCallError.new("message", -1).errno, SystemCallError.new("message", 1).message]
      assert_equal 42, SystemCallError.new("message", 42).dup.errno
      exc = Errno::EINVAL.new("custom message", "location")
      assert_equal [Errno::EINVAL, "Invalid argument @ location - custom message"], [exc.class, exc.message]
      msg = begin
        File.read("/nonexistent/control_zz")
      rescue Errno::ENOENT => e
        e.message
      end
      assert_equal "No such file or directory @ rb_sysopen - /nonexistent/control_zz", msg
    end

    def test_signal_frozen_no_method
      s = SignalException.new(2, "name")
      assert_equal [2, "name"], [s.signo, s.message]
      k = begin
        SignalException.new("INT", "name")
        nil
      rescue ArgumentError => x
        x.class
      end
      assert_equal ArgumentError, k
      o = Object.new
      assert_same o, FrozenError.new("msg", receiver: o).receiver
      assert_equal ["args"], NoMethodError.new("msg", "name", ["args"]).args
    end
  end

  # decision 382: each thread appends to its own block-local buffer
  class ThreadAppendTest < Minitest::Test
    def test_threads_build_their_own_strings
      ts = (0...4).map do |k|
        Thread.new(k) do |id|
          b = +"t#{id}:"
          200.times { |j| b << j.to_s << "," }
          b
        end
      end
      sizes = ts.map { |t| t.value.size }
      assert_equal [693, 693, 693, 693], sizes
    end
  end

  # times, upto, downto and an Integer Range literal's each run as plain Go loops (decision 387).
  class ControlCountedLoopTest < Minitest::Test
    def test_times_and_ranges
      seen = [] #: Array[Integer]
      3.times { |i| seen << i }
      0.times { seen << 99 }
      -2.times { seen << 98 }
      (5..7).each { |i| seen << i }
      (5...7).each { |i| seen << i * 10 }
      (3..1).each { seen << 97 }
      2.upto(4) { |i| seen << i }
      4.downto(2) { |i| seen << -i }
      3.upto(1) { seen << 96 }
      assert_equal [0, 1, 2, 5, 6, 7, 50, 60, 2, 3, 4, -4, -3, -2], seen
      n = 0
      5.times { n += 1 }
      (1..3).each { n += 10 }
      assert_equal 35, n
    end

    def test_jumps_and_param_writes
      got = [] #: Array[Integer]
      10.times do |i|
        next if i.odd?
        break if i > 6

        got << i
      end
      (1..10).each do |i|
        i += 100
        got << i if i < 103
      end
      1.upto(3) do |i|
        i *= 2
        got << i
      end
      assert_equal [0, 2, 4, 6, 101, 102, 2, 4, 6], got
      hits = [] #: Array[Integer]
      3.downto(1) do |k|
        case k
        when 3 then hits << k
        else break
        end
      end
      (1..5).each do |k|
        case k
        when 1, 2 then hits << k
        else break
        end
      end
      assert_equal [3, 1, 2], hits
    end

    def test_closures_see_their_own_element
      procs = [] #: Array[^() -> Integer]
      3.times { |i| procs << -> { i } }
      (4..5).each { |i| procs << -> { i * 2 } }
      assert_equal [0, 1, 2, 8, 10], procs.map(&:call)
    end

    def test_bounds_at_the_edges
      big = 2**62 + (2**62 - 1)
      got = [] #: Array[Integer]
      (big - 1).upto(big) { |i| got << i - big }
      (-big - 1).downto(-big - 1) { |i| got << i + big }
      ((big - 1)..big).each { |i| got << i - big }
      assert_equal [-1, 0, -1, -1, 0], got
      calls = 0
      lim = -> { calls += 1; 3 }
      1.upto(lim.call) { |i| calls += i * 0 }
      lim.call.times { calls += 0 }
      assert_equal 2, calls
    end
  end

  # Proc literals with keyword or &block parameters, held in a local that is only called (decision 456)
  class ProcParamsTest < Minitest::Test
    def test_keywords
      scale = ->(x, by: 2) { x * by }
      assert_equal [2, 15], [scale.call(1), scale.(3, by: 5)]
      req = ->(a, name:) { "#{a}:#{name}" }
      assert_equal "1:n", req.call(1, name: "n")
      bag = ->(*xs, **opts) { [xs, opts] }
      assert_equal [[1, 2], { k: 3 }], bag.call(1, 2, k: 3)
      assert_equal [[], {}], bag.call
    end

    def test_block_parameter
      twice = proc { |a, &b| b.call(b.call(a)) }
      assert_equal 12, twice.call(3) { |x| x * 2 }
      maybe = ->(a, &b) { b ? b.call(a) : :none }
      assert_equal :none, maybe.call(1)
      assert_equal 2, maybe.call(1) { |x| x + 1 }
    end
  end
end
