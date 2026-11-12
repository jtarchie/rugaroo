#!/bin/bash
# usage: small.sh <rugaroo> <tag> [file]  -- one-cop build, then compare on the file (default b.rb) with that one cop
S=/private/tmp/claude-501/-Users-jtarchie-workspace-transpiler/96888534-69b3-4796-9e52-95c04efaffc4/scratchpad
f=${3:-b.rb}; A="--cache false --no-parallel --only Style/FrozenStringLiteralComment"
(ARGS="$A a.rb" GCF="-N -l" $S/round.sh $1 $2 > $S/r$2.out 2>&1)
grep "^real\|TYPECHECK\|GEN FAILED" $S/r$2.out | head -3 | tr '\n' ' '; echo
[ -x $S/r-$2/rubocop ] || { grep -A3 "^vet: " $S/r$2.out | head -4 | cut -c1-300; exit 1; }
cd $S/work; export RUBOCOP_LIB=$S/work/rubocop-1.92.1/lib RUGAROO_BACKTRACE=1
ruby -I regexp_parser-2.13.1/lib -I rubocop-ast-1.50.0/lib -I rubocop-1.92.1/lib main.rb $A $f > /tmp/sm.out 2>/dev/null; me=$?
$S/r-$2/rubocop -d $A $f > /tmp/sr.out 2> /tmp/sr.err; re=$?
echo "mri[$me]: $(tail -1 /tmp/sm.out) | rg[$re]: $(grep -v '^For /\|^Scanning\|^Finished' /tmp/sr.out | tail -1 | cut -c1-200)"
grep -v "^For /\|^Inspecting\|^Scanning\|^$\|^Finished\|inspected" /tmp/sr.out | sed "s|$S/work/||g; s|/opt/homebrew/lib/ruby/gems/4.0.0/gems/||g; s|/opt/homebrew/Cellar/ruby/4.0.9/lib/ruby/gems/4.0.0/gems/||g" | head -${N:-5} | cut -c1-300
