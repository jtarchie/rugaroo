# nil_check: runtime
# A shim stands in for a file the program requires; the required files' .rbs sidecars type them, an untyped parameter from its callers (decision 404).
$LOADED_FEATURES << File.expand_path("require_shims/real.rb", __dir__)
require_relative "require_shims/boot"

puts Lib.first_byte("A")
puts Lib.sum_to(4)
puts Lib.show(3)
puts Lib.show("x")

# A sidecar's `untyped` return is a hint: the finders a class extends answer that class's instances (decision 494).
puts Article.find(1).title
puts Article.all.map { |article| article.title }.join(", ")
puts Article.find(2).notes.map { |note| note.text }.join(", ")
p Article.find_by(0)
puts Note.find_all(1).first&.text
