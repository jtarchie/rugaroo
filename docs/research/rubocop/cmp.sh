#!/bin/bash
# usage: cmp.sh <tag> file...   -- compiled r-<tag>/rubocop vs MRI on each file (stdout, exit, failing cops)
S=/private/tmp/claude-501/-Users-jtarchie-workspace-transpiler/96888534-69b3-4796-9e52-95c04efaffc4/scratchpad
t=$1; shift; cd $S/work; export RUBOCOP_LIB=$S/work/rubocop-1.92.1/lib RUGAROO_BACKTRACE=1
I="-I regexp_parser-2.13.1/lib -I rubocop-ast-1.50.0/lib -I rubocop-1.92.1/lib"
for f in "$@"; do
  b=$(basename $f)
  ruby $I main.rb --cache false --no-parallel $f > /tmp/mri_$b.out 2>/dev/null; me=$?
  $S/r-$t/rubocop --cache false --no-parallel $f > /tmp/rg_$b.out 2> /tmp/rg_$b.err; re=$?
  if diff -q /tmp/mri_$b.out /tmp/rg_$b.out > /dev/null && [ $me = $re ]; then echo "$f: SAME (exit $me) $(tail -1 /tmp/mri_$b.out)"; else echo "$f: DIFF mri[$me]: $(tail -1 /tmp/mri_$b.out) | rg[$re]: $(tail -1 /tmp/rg_$b.out)"; fi
  grep "An error occurred while" /tmp/rg_$b.err | sed 's/ cop was inspecting.*//; s/An error occurred while //' | sort -u | tr '\n' ' ' | sed 's/^/  cops: /; s/$/\n/'
  grep -v "^Ignoring\|^  Enabled\|# new in\|^$\|NewCops\|AllCops\|^Please\|^The following\|^For more\|An error occurred\|^To see\|errors\? occurred\|^[A-Z][a-z]*/\|^Errors are\|^Mention\|^1.92\|^https" /tmp/rg_$b.err | sed "s|$S/work/||g; s|/opt/homebrew/lib/ruby/gems/4.0.0/gems/||g" | head -3 | cut -c1-260
done
