# frozen_string_literal: true

# MRI oracle on Gemfile.lock's gems, not the newest installed (decision 481); ENV restored so programs see no Bundler vars.
env = ENV.to_h
ENV["BUNDLE_GEMFILE"] = File.expand_path("../Gemfile", __dir__)
require "bundler/setup"
ENV.replace(env)
