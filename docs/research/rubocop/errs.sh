#!/bin/bash
# usage: errs.sh <tag> -- union of cop errors (first two frames) over the real files
S=/private/tmp/claude-501/-Users-jtarchie-workspace-transpiler/96888534-69b3-4796-9e52-95c04efaffc4/scratchpad
cd $S/work
for f in ${FILES:-b d e f g h i j k l m n}; do RUGAROO_BACKTRACE=1 RUBOCOP_LIB=$S/work/rubocop-1.92.1/lib $S/r-$1/rubocop -d --cache false --no-parallel $f.rb 2>&1 | sed "s|$S/work/||g; s|/opt/homebrew/lib/ruby/gems/4.0.0/gems/||g" | grep -A2 "An error occurred while" | grep -v "An error occurred\|^--" | paste - - ; done | sed -E 's/[a-z_0-9.-]*\/lib\///g' | cut -c1-420 | sort | uniq -c | sort -rn > $S/r-$1/errs.txt
wc -l < $S/r-$1/errs.txt
