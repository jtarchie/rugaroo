import subprocess, time, statistics, sys, os
S=sys.argv[1]; files=sys.argv[2:]
I=["-I","regexp_parser-2.13.1/lib","-I","rubocop-ast-1.50.0/lib","-I","rubocop-1.92.1/lib"]
def run(cmd):
    t=time.perf_counter(); subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); return time.perf_counter()-t
for f in files:
    mri=["ruby",*I,"main.rb","--cache","false","--no-parallel",f]
    opt=[S+"/r-opt/rubocop","--cache","false","--no-parallel",f]
    dbg=[S+"/r-f36/rubocop","--cache","false","--no-parallel",f]
    m,o,d=[],[],[]
    for _ in range(5):
        m.append(run(mri)); o.append(run(opt)); d.append(run(dbg))
    md,od,dd=statistics.median(m),statistics.median(o),statistics.median(d)
    print(f"{f:6} mri {md:5.2f}s  compiled-opt {od:5.2f}s ({md/od:4.1f}x)  compiled-debug {dd:5.2f}s", flush=True)
