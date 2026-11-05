# rbs_inline: enabled

require "tmpdir"

# Dir.glob with ** and {a,b}, Dir.chdir with a block, File.stat.

FILES = {
  "lib/app.rb" => "puts 1\n",
  "lib/app/models/user.rb" => "class User; end\n",
  "lib/app/views/index.erb" => "<p>hi</p>\n",
  "test/app_test.rb" => "# test\n",
  "README.md" => "# App\n",
  ".git/config" => "[core]\n"
} #: Hash[String, String]

Dir.mktmpdir do |root|
  Dir.chdir(root) do
    FILES.each do |path, body|
      dir = File.dirname(path)
      parts = dir.split("/")
      parts.each_index do |i|
        d = parts[0..i].join("/")
        Dir.mkdir(d) unless d == "." || Dir.exist?(d)
      end
      File.write(path, body)
    end

    puts "Ruby files:"
    Dir.glob("**/*.rb").each { |f| puts "  #{f} (#{File.stat(f).size} bytes)" }

    puts "Templates and docs:"
    Dir.glob("{lib/**/*.erb,*.md}").each { |f| puts "  #{f}" }

    puts "Directories:"
    Dir.glob("**/").each { |d| puts "  #{d}" }

    Dir.chdir("lib") { puts "inside lib: #{Dir["*"].join(", ")}" }
    puts "back at root: #{Dir.glob("*").join(", ")}"
  end
end
