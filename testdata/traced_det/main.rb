# a run-time require and a $LOAD_PATH edit: both tables reach the output
$LOAD_PATH.unshift(__dir__)
name = ARGV.fetch(0, "set")
puts require(name)
puts $LOAD_PATH.include?(__dir__)
