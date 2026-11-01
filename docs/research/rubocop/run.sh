#!/bin/bash
# usage: run.sh /path/to/rugaroo [out.go]
# rugaroo gen of the rubocop CLI (rubocop 1.92.1, rubocop-ast 1.50.0, regexp_parser 2.13.1, Homebrew Ruby 4.0.9 gem
# paths) on a 2-line a.rb, with the research patches applied to fresh copies first on -I. Also runs MRI on the same
# copies, whose last line must be "1 file inspected, 1 offense detected, 1 offense autocorrectable".
set -e
here=$(cd "$(dirname "$0")" && pwd); G=/opt/homebrew/lib/ruby/gems/4.0.0/gems
w=$(mktemp -d); I=""
for g in regexp_parser-2.13.1 rubocop-ast-1.50.0 rubocop-1.92.1; do
  cp -R $G/$g $w/ && patch -s -p1 -d $w/$g < $here/$g.patch
  I="$I -I $w/$g/lib"
done
cd $w && printf 'x = 1\nputs x\n' > a.rb && cp $here/main.rb .
export RUBOCOP_LIB=$w/rubocop-1.92.1/lib
ruby $I main.rb --cache false --no-parallel a.rb 2>/dev/null | tail -1 || true
time RUGAROO_TIMING=1 "$1" gen $I -trace main.rb --cache false --no-parallel a.rb > "${2:-/dev/null}"
