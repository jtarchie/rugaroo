#!/bin/bash
# usage: run.sh /path/to/rugaroo [out.go]   (Homebrew Ruby 4.0.9 gem paths; adjust G/RACC)
set -e
here=$(cd "$(dirname "$0")" && pwd); G=/opt/homebrew/lib/ruby/gems/4.0.0/gems
RACC=/opt/homebrew/Cellar/ruby/4.0.9/lib/ruby/gems/4.0.0/gems/racc-1.8.1
w=$(mktemp -d); cp -R $G/rubocop-ast-1.48.0 $w/ && patch -s -p1 -d $w/rubocop-ast-1.48.0 < $here/rubocop-ast.patch
cd $here
time RUGAROO_TIMING=1 "$1" gen -I $w/rubocop-ast-1.48.0/lib -I $G/parser-3.3.10.0/lib -I $G/ast-2.4.3/lib -I $RACC/lib -trace main.rb > "${2:-/dev/null}"
