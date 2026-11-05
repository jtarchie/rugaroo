# rbs_inline: enabled

require "bigdecimal"
require "bigdecimal/util"

# BigDecimal: exact decimal arithmetic for money, where Float drifts.

puts "Float:      #{0.1 + 0.2}"
puts "BigDecimal: #{(BigDecimal("0.1") + BigDecimal("0.2")).to_s("F")}"

class Invoice
  #: (Array[[String, String, Integer]]) -> void
  def initialize(lines)
    @lines = lines
  end

  #: () -> BigDecimal
  def subtotal
    total = BigDecimal("0")
    @lines.each { |(_, price, qty)| total += BigDecimal(price) * qty }
    total
  end

  #: (String) -> BigDecimal
  def tax(rate) = (subtotal * BigDecimal(rate)).round(2, :half_even)

  #: (String) -> BigDecimal
  def total(rate) = subtotal + tax(rate)
end

inv = Invoice.new([["widget", "19.99", 3], ["gadget", "4.05", 7], ["gizmo", "0.333", 9]])
puts "subtotal #{inv.subtotal.to_s("F")}"
puts "tax      #{inv.tax("0.0825").to_s("F")}"
puts "total    #{inv.total("0.0825").to_s("F")}"

third = BigDecimal("1") / 3
puts "1/3 to the default precision: #{third}"
puts "1/3 to 10 digits: #{BigDecimal("1").div(3, 10)}"
puts "split three ways: #{(third * 3).round(20)} (rounded back)"
puts "sqrt(2) to 30 digits: #{BigDecimal("2").sqrt(30).to_s("F")}"
puts "1.07 ** 30 = #{(BigDecimal("1.07")**30).round(6).to_s("F")}"
puts "grouped: #{BigDecimal("1234567.891").to_s("3F")}"
puts "from strings: #{"12.50 USD".to_d}, #{"abc".to_d}"
puts "rounding 2.5: half_up #{BigDecimal("2.5").round(0, :half_up)}, half_even #{BigDecimal("2.5").round(0, :half_even)}, floor #{BigDecimal("-2.5").floor}"
q, r = BigDecimal("17.5").divmod(4)
puts "17.5 divmod 4 = #{q}, #{r}"
