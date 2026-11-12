#!/bin/bash
# usage: dbg.sh <tag> [args...]: run r-<tag>/rubocop with -d and show the first error
S=/private/tmp/claude-501/-Users-jtarchie-workspace-transpiler/96888534-69b3-4796-9e52-95c04efaffc4/scratchpad
t=$1; shift; cd $S/work; export RUBOCOP_LIB=$S/work/rubocop-1.92.1/lib RUGAROO_BACKTRACE=1
$S/r-$t/rubocop -d "$@" > $S/r-$t/d.out 2> $S/r-$t/d.err; echo "exit=$?"
sed "s|$S/work/||g; s|/opt/homebrew/lib/ruby/gems/4.0.0/gems/||g; s|/opt/homebrew/Cellar/ruby/4.0.9/lib/ruby/gems/4.0.0/gems/||g" $S/r-$t/d.out | grep -v "^For /\|^Scanning\|^Use parallel\|^Skipping" | head -${LINES_OUT:-14} | cut -c1-330
echo "--- stderr"; grep -v "^Ignoring\|^  Enabled\|# new in\|^$\|NewCops\|AllCops\|^Please\|^The following\|^For more" $S/r-$t/d.err | sed "s|$S/work/||g" | head -6 | cut -c1-300
