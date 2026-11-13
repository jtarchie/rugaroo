# exe/rubocop's non-server path (its server CLI is a no-op without --server flags; exe/rubocop itself stops at an
# autoload named only from the main file). RUBOCOP_LIB is the patched copy's lib (`setup` sets it).
$LOAD_PATH.unshift(ENV.fetch("RUBOCOP_LIB"))
require 'rubocop'
exit RuboCop::CLI.new.run
