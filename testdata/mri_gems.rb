# frozen_string_literal: true

# MRI oracle on Gemfile.lock's gems, not the newest installed (decision 481); ENV restored so programs see no Bundler vars.
# Bundler's own stderr notes (an installed gem "is missing extensions", under a test's `# env:`) are not the program's.
env = ENV.to_h
err = $stderr.dup
ENV["BUNDLE_GEMFILE"] = File.expand_path("../Gemfile", __dir__)
$stderr.reopen(File::NULL)
begin
  require "bundler/setup"
ensure
  $stderr.reopen(err)
  err.close
end
ENV.replace(env)
