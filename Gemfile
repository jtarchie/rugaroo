# frozen_string_literal: true

source "https://rubygems.org"

# Type annotations: `rbs-inline` extracts `#:` comments, `rbs` validates them.
gem "rbs", "~> 4.0"
gem "rbs-inline", "~> 0.14", require: false

# The HTTP server examples run on under MRI (stdlib until Ruby 3.0). The
# transpiler maps it, and net/http's client, onto Go's net/http.
gem "webrick", "~> 1.9"

# Gems rugaroo compiles unmodified (#82): rack 3.2.7 is what sig/gems/rack was written against.
gem "cuba", "~> 4.0"
gem "rack", "~> 3.2", ">= 3.2.7"

# Serves a Rack app on WEBrick under MRI; the transpiler maps it onto Go's net/http too.
gem "rackup", "~> 2.3"

# Sorbet-typed programs (#95): MRI runs them with the real runtime; rugaroo reads `sig {}` as types.
gem "sorbet-runtime", "~> 0.6"

# A pure-Ruby priority queue rugaroo compiles unmodified (example 120).
gem "pairing_heap", "~> 3.1"

# Type inference without annotations (dev tool, not a test dependency): `bundle exec tyda file.rb`
# prints inferred RBS. See CLAUDE.md for when to reach for it.
gem "tyda", require: false, group: :development

# Markdown to HTML (example 123): rugaroo compiles the unmodified gem from a traced run (decision 330).
gem "kramdown", "~> 2.5"

# Templates (example 124): Jekyll 4's liquid, compiled unmodified from a traced run.
gem "bigdecimal", "~> 3.1" # liquid 4 requires it; a bundled gem since Ruby 3.4, so Bundler must list it
gem "liquid", "~> 4.0"

# SQLite under MRI for example 122; rugaroo maps `require "sqlite3"` onto its own Fiddle port of this gem (decision 327).
gem "sqlite3", "~> 2.10"

# C calls under MRI for example 127; rugaroo maps `require "ffi"` onto its own port over Fiddle (decision 375).
gem "ffi", "~> 1.17"

# MRI's side of the prism oracle (prism_test, example 136) at the release rugaroo bundles in vendored/prism (decision 427).
gem "prism", "1.9.0"

# Bundled gems ruby/spec's library/ directories exercise (decision 409): pinned to what Ruby 4.0.9 (.mri-version, decision 413) bundles, so the spec
# numbers in docs/support.md name one version. rugaroo compiles them unmodified, as installed gems.
gem "drb", "~> 2.2.3"
gem "getoptlong", "~> 0.2.1"
gem "matrix", "~> 0.4.3"
gem "mutex_m", "~> 0.3.0"
gem "net-ftp", "~> 0.3.9"
gem "net-imap", "~> 0.6.4"
gem "net-pop", "~> 0.1.2"
gem "net-smtp", "~> 0.5.1"
gem "prime", "~> 0.1.4"
gem "pstore", "~> 0.2.1"
gem "resolv-replace", "~> 0.1.1"
gem "rinda", "~> 0.2.0"
gem "rss", "~> 0.3.2"
