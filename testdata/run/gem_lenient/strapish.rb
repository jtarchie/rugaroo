# A gem's public instance method appending to its String parameter, and appends through an element block (#128).
class Strapish
  def into(buf, x)
    buf << x << ";"
    nil
  end

  def build(parts)
    out = +""
    parts.each { |part| into(out, part) }
    out
  end

  def newlines(rows)
    rows.each { |r| r << "\n" }
    rows
  end

  def tails(rows)
    rows.each { |r| r << "!" }
    rows
  end
end
