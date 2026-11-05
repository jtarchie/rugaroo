# rbs_inline: enabled

require "erb"

# ERB templates: compiled with the program, run in the caller's scope.

class Item
  attr_reader :name #: String
  attr_reader :price #: Float

  #: (String, Float) -> void
  def initialize(name, price)
    @name = name
    @price = price
  end
end

RECEIPT = ERB.new(<<~ERB, trim_mode: "-")
  Receipt for <%= customer %>
  <%- items.each do |item| -%>
    <%= item.name.ljust(8) %> <%= format("%6.2f", item.price) %>
  <%- end -%>
  Total: <%= format("%.2f", items.sum(&:price)) %>
ERB

#: (String, Array[Item]) -> String
def receipt(customer, items) = RECEIPT.result(binding)

items = [Item.new("tea", 3.5), Item.new("scone", 2.25)]
puts receipt("Ada", items)

greeting = ERB.new('Hello <%= name %>, you have <%= count %> message<%= count == 1 ? "" : "s" %>.')
puts greeting.result_with_hash(name: "Grace", count: 3)
puts greeting.result_with_hash(name: "Linus", count: 1)

html = ERB.new(<<~HTML, trim_mode: "<>")
  <ul>
  <% items.each do |item| %>
    <li><%= ERB::Util.h(item.name + " & more") %></li>
  <% end %>
  </ul>
HTML
puts html.result(binding)

ERB.new("<%# a comment %>literal <%% not code %%> and <%= 6 * 7 %>\n").run
