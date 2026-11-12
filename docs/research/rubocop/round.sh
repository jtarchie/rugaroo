#!/bin/bash
# usage: round.sh <rugaroo> <tag>  -- gen (work dir reused), fast typecheck, then go build; logs under $S/r-<tag>
S=/private/tmp/claude-501/-Users-jtarchie-workspace-transpiler/96888534-69b3-4796-9e52-95c04efaffc4/scratchpad
w=$S/work; R=$S/r-$2; mkdir -p $R/chk $R/mod
export RUGAROO_BACKTRACE=1; cd $w; export RUBOCOP_LIB=$w/rubocop-1.92.1/lib
I="-I $w/regexp_parser-2.13.1/lib -I $w/rubocop-ast-1.50.0/lib -I $w/rubocop-1.92.1/lib"
( time RUGAROO_TIMING=1 $1 gen $I -trace main.rb ${ARGS:---cache false --no-parallel a.rb} > $R/mod/main.go ) > $R/gen.log 2>&1 || { echo GEN FAILED; grep -v "^warning\|^	\|^Ignoring" $R/gen.log | tail -20; exit 1; }
grep "^real\|infer cache\|compile " $R/gen.log
cp $S/dbg/go.mod $S/dbg/go.sum $R/chk/; cp $S/dbg/go.mod $S/dbg/go.sum $R/mod/
grep -v '^//line ' $R/mod/main.go > $R/chk/main.go
cd $R/chk && go vet ./... > $R/chk.log 2>&1
if grep -q "^vet: " $R/chk.log; then echo TYPECHECK ERRORS; grep -A30 "^vet: " $R/chk.log | head -40; exit 1; fi
cd $R/mod && ( time go build -trimpath -gcflags="-dwarf=false $GCF" -ldflags=-w -o $R/rubocop . ) > $R/build.log 2>&1; tail -5 $R/build.log
ls -la $R/rubocop
cd $w; ( time $R/rubocop ${ARGS:---cache false --no-parallel a.rb} ) > $R/run.out 2> $R/run.err; echo "exit=$?"
tail -3 $R/run.out | cut -c1-300; grep -v "^Ignoring" $R/run.err | head -12 | cut -c1-600
