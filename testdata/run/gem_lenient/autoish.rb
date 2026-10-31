# frozen_string_literal: true

# A gem's autoload with a computed path (unicode-emoji's `File.join(dir, name.downcase)`): the file that defines the
# constant, when another require loads it, runs where the autoload registers (decision 175).
module Autoish
  lazy_dir = File.expand_path("autoish", __dir__)
  %w[
    LAZY_WIDTH
    LAZY_NAME
  ].each do |const_name|
    autoload const_name, File.join(lazy_dir, const_name.downcase)
  end
end
