# rbs_inline: enabled

require "rexml/document"

# REXML: parse an XML feed, query it with XPath, change it and write it back.

FEED = <<~XML
  <?xml version="1.0" encoding="UTF-8"?>
  <feed updated="2024-05-01">
    <entry id="1" status="published">
      <title>Compilers &amp; You</title>
      <author>Ada</author>
      <tags><tag>go</tag><tag>ruby</tag></tags>
    </entry>
    <entry id="2" status="draft">
      <title>Notes on &lt;XML&gt;</title>
      <author>Grace</author>
      <tags><tag>xml</tag></tags>
    </entry>
  </feed>
XML

doc = REXML::Document.new(FEED)
feed = doc.root || raise("empty feed")

puts "updated #{feed.attributes["updated"]}, #{feed.elements.size} entries"
feed.each_element("entry") do |entry|
  tags = entry.get_elements("tags/tag").map(&:text).join(", ")
  puts "##{entry.attributes["id"]} #{entry.text("title")} by #{entry.text("author")} [#{tags}]"
end

draft = feed.elements["entry[@status='draft']"] || raise("no draft")
puts "draft: #{draft.text("title")}"
puts "all tags: #{REXML::XPath.match(doc, "//tag").map(&:text).inspect}"
puts "last id: #{REXML::XPath.first(doc, "//entry[last()]/@id").value}"

draft.attributes["status"] = "published"
entry = feed.add_element("entry", { "id" => "3", "status" => "draft" })
entry.add_element("title").text = "Escapes: <, > & \"quotes\""
entry.add_element("author").text = "Linus"
puts "published: #{feed.get_elements("entry[@status='published']").size}"

puts "--- compact"
puts entry
puts "--- pretty"
doc.write($stdout, 2)
puts
