# rbs_inline: enabled

require "csv"

# Reads an inventory export, totals it per category, and writes a report back out as CSV.
class Inventory
  #: (String) -> void
  def initialize(data)
    rows = CSV.parse(data)
    @header = rows.shift || [] #: Array[String?]
    @rows = rows.reject(&:empty?)
  end

  #: (String) -> Integer
  def column(name) = @header.index(name) || raise(ArgumentError, "no column #{name}")

  #: () -> Hash[String, Integer]
  def totals
    cat = column("category")
    qty = column("qty")
    out = Hash.new #: Hash[String, Integer]
    @rows.each do |r|
      key = r[cat] || "(none)"
      out[key] = (out[key] || 0) + (r[qty] || "0").to_i
    end
    out
  end
end

data = <<~CSV
  sku,name,category,qty
  A1,"Widget, large",tools,4
  A2,"The ""best"" gadget",gadgets,7

  A3,Plain,tools,
  A4,"multi
  line",,2
CSV
inv = Inventory.new(data)
report = CSV.generate do |csv|
  csv << %w[category total]
  inv.totals.each { |k, v| csv << [k, v] }
end
puts report
p CSV.parse(data).last

p CSV.parse_line("1;2;;\"x;y\"", col_sep: ";"), CSV.parse_line(""), "a,\"\",b".parse_csv
p CSV.parse("a,b\r\nc,d\r\n")
p CSV.generate_line(["a", nil, "", "b,c", "q\"t", 1, 2.5, " sp", "nl\n"])
p %w[x y].to_csv, [1, nil, :sym].to_csv, CSV.generate_line(%W[a b\tc], col_sep: "\t")
puts CSV.generate(col_sep: "|") { |c| c << ["a|b", "c"] }
begin
  CSV.parse("ok\n\"abc")
rescue CSV::MalformedCSVError => e
  puts "#{e.class}: #{e.message}"
end
begin
  CSV.parse("a\"b\"c")
rescue CSV::MalformedCSVError => e
  puts e.message
end
