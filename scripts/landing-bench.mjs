// Prints the landing page benchmark chart (site/index.html, from "About N× faster" up to "Checked against ruby/spec"): node scripts/landing-bench.mjs
// ponytail: rows copied by hand from BENCHMARKS.md (2026-11-22, after decision 489); parse that table if it gets rerun often
const rows = [
  ['fib(32)', .26, .093, .016], ['Hash counting', .11, .097, .012], ['Array map/select/sum', .15, .13, .015],
  ['string build/split', .11, .11, .015], ['Math.sin/cos', .086, .073, .009], ['classes, polymorphic calls', .26, .17, .023],
  ['binary trees', .082, .069, .008], ['splay tree', .21, .12, .014], ['blocks, procs, enumerators', .36, .34, .033],
  ['raise/rescue, catch/throw', .28, .25, .11], ['regexp, StringScanner', .75, .73, .18], ['sort_by/group_by/tally, Set', .55, .52, .1],
  ['format, StringIO', .48, .45, .12], ['JSON generate + parse', .14, .15, .11], ['Bignum', .21, .21, .057],
  ['pairing_heap gem', .66, .35, .091], ['liquid gem', .38, .3, .17], ['kramdown gem', .46, .58, .4],
].map(([n, mri, yjit, rg]) => ({ n, mri, yjit, rg, sr: mri / rg, sy: mri / yjit })).sort((a, b) => b.sr - a.sr);
const max = 18, pct = v => (v / max * 100).toFixed(2), x = v => v >= 10 ? v.toFixed(0) : v.toFixed(1);
const t = s => `${s < 1 ? (s * 1000).toFixed(0) + ' ms' : s + ' s'}`;
let h = rows.map(r => `      <div class="brow"><span class="bname">${r.n}</span><span class="btrack" title="${r.n}: MRI ${t(r.mri)}, YJIT ${t(r.yjit)}, rugaroo ${t(r.rg)}"><span class="l1"><span class="bar rg" style="width:${pct(r.sr)}%"></span><span class="bval">${x(r.sr)}×</span></span><span class="bar yj" style="width:${pct(r.sy)}%"></span></span></div>`).join('\n');
let tbl = rows.map(r => `<tr><td>${r.n}</td><td>${t(r.mri)}</td><td>${t(r.yjit)}</td><td>${t(r.rg)}</td><td>${x(r.sr)}×</td></tr>`).join('');
console.log(`  <h2>About 3.7× faster than MRI</h2>
  <p class="sub">Speedup over plain MRI (1×) on the <a href="https://github.com/jtarchie/rugaroo/tree/main/benchmarks">benchmark workloads</a>, against MRI with YJIT. All 18 together take 1.53 s compiled and 5.5 s on MRI. Median of 9 runs on one Apple-silicon machine.</p>
  <figure class="bench" aria-labelledby="bench-cap">
    <figcaption id="bench-cap" class="legend"><span><i class="sw rg"></i>rugaroo</span><span><i class="sw yj"></i>MRI + YJIT</span><span><i class="sw base"></i>MRI = 1×</span></figcaption>
    <div class="bchart" style="--one:${pct(1)}%">
${h}
      <div class="brow axis" aria-hidden="true"><span class="bname"></span><span class="btrack">${[1, 5, 10, 15].map(v => `<span style="left:${pct(v)}%">${v}×</span>`).join('')}</span></div>
    </div>
    <details><summary>Table</summary><table><thead><tr><th>workload</th><th>MRI</th><th>MRI+YJIT</th><th>rugaroo</th><th>speedup</th></tr></thead><tbody>${tbl}</tbody></table></details>
  </figure>
  <p class="sub out">Where it trails, and how it compares with spinel, Crystal and hand-written Go: <a href="https://github.com/jtarchie/rugaroo/blob/main/BENCHMARKS.md">BENCHMARKS.md</a>.</p>
`);
