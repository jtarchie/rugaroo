# load_path: lib
# load_path: sigdemo/lib
require "demo"
require "demo/text"
require "demo/gemish"
require "demo/objdyn"
require "demo/cli"
require "sigdemo"

# A computed require in a method is a compile-time no-op (decision 163); under MRI it loads demo/unnamed.
def load_extra(name) = require(name)
load_extra("demo/unnamed")

puts Demo::Thing.new("hi").greet
puts Demo::Lazy.new.go
p Demo::Text.forwarded_values("for=192.0.2.60;proto=http;by=203.0.113.43")
p Demo::Text.forwarded_values('for="_gazonk", For="[2001:db8:cafe::17]:4711"')
p Demo::Text.forwarded_values(nil)
p Demo::Text.first_param(+"for=x")
p Demo::Config::DEFAULTS
g = Demo::Gemish.new
p g.state
p Demo::Gemish::Pair.new("k", 1).to_a
p Demo::Gemish::Part.new("b", "n").label
h = {} #: Hash[String, String]
p g.kind?(h)
h["kind"] = "html"
p g.kind?(h)
p g.joined
p g.splat_args, Demo::Gemish.factory_name
require "tmpdir"
Dir.mktmpdir { |d| p g.observed_mkpath(d) }
p g.spawn_opts("/tmp"), g.spawn_opts(nil)
p Demo::Gemish::Registry.names, Demo::Gemish::Guarded.seen, Demo::Gemish::Guard1.fields, Demo::Gemish::Guard2.fields
p g.rebind_block(1) { 1 }.class, g.rebind_block(-3) { 1 }
p $LOAD_PATH.any? { |d| d.end_with?("/lib") }, $:.size == $LOAD_PATH.size
p g.rebind("a b")
p g.safe_responds?(:joined), g.safe_responds?(:nope)
p g.first_big([1]) if ENV["RUGAROO_NEVER"]
p g.never_defined(1) if ENV["RUGAROO_NEVER"]
begin
  g.recheck
rescue ArgumentError => e
  p e.message
end
p g.setup_optional(false)
begin
  g.setup_optional(true)
rescue NameError => e
  p e.class, e.message
end
p g.never_set_or(7), Demo::Gemish.cached
p g.dyn_send(g, :state), g.dyn_send(Demo::GemTab.empty, :kind), g.dyn_send(Demo::Lenient, :mixed, false), g.dyn_send(g, :frozen?)
p g.dyn_responds(g, :joined), g.dyn_responds(g, :nope), g.dyn_responds(Demo::GemTab.empty, :kind)
p g.json_kids('{"kids":[{"name":"x","age":5},{"name":"y","age":2}]}')
p g.json_find('{"sigs":[{"h":{"kid":"a"}},{"h":{"kid":"b"},"v":1}]}'), g.json_find("{}")
p g.json_big('{"a":1,"b":2}')
p g.tab_for("x").kind, g.tab_for("")
p g.synced
p g.remaining(3), g.remaining(0.5)
$stdout.ohai("extended")
cli = Demo::CliParser.new.parse(["json"])
p cli.json?, cli.sizes?

p DemoCli::InfoCmd.new.run
ENV["DEMO_X_TOKEN"] = "s"
p ENV.sensitive?("A_TOKEN"), ENV.sensitive_environment.keys.include?("DEMO_X_TOKEN")
ctr = Demo::Counter.new
3.times { ctr.inc }
p ctr.value
p Demo::Lenient.three(3), Demo::Lenient.pairs(4), Demo::Lenient.mixed(true)
p Demo::Lenient.raised(ArgumentError, "boom"), Demo::Lenient.lines("one\ntwo")
p Demo::Lenient.uri_path("http://h/a/b"), Demo::Lenient.uri_path("x")
p Demo::Lenient.merged({ x: 1 })
fs = DemoFormulaStruct.new(desc: "d", bottle_present: true)
p fs.desc, fs.bottle?, fs.head?
fh = DemoFormulaStruct.from_hash({ "desc" => "h", "head_present" => true, "junk" => 1 })
p fh.desc, fh.head?, fh.bottle?
p DemoSystemCommand.new("a").input, DemoSystemCommand.new(%w[b c]).input
p DemoPackagesIndex.write!
p DemoVersion.new(1) < DemoVersion.new(2), [DemoVersion.new(3), DemoVersion.new(1)].max.v, DemoVersion.new(1) <=> "x"
bes = [DemoBundle::Entry.new("a"), DemoBundle::Entry.new("b")]
p DemoBundle::Ext.run(bes), DemoBundle::Vscode.run(bes), DemoBundle::Vscode.cleanup_items(bes)
p DemoBottle.new("wget").name, DemoBottle.new(nil, name: "curl").name
begin
  DemoBottle.strict(nil)
rescue TypeError => e
  p e.class
end
lg = Demo::Lenient.logger
p lg.call(1, "x", nil), lg.call(3, "p", "m")
p Demo::Lenient.level_of, Demo::Lenient.level_of(:SEV_DEBUG)
p Demo::Lenient.sent("aBc")
p Demo::Lenient.missing(true), Demo::Lenient.missing(false)
tagb = [{ "a" => :x }, { "b" => %i[y z] }, { "c" => nil }] #: Array[Hash[String, untyped]]
p(tagb.map { |h| Demo::Lenient.tags_of(h) })
p DemoPool.new(name: "q", size: 4) { |x| x * 2 }.report, DemoPool.new.report
p DemoPlainPool.new(size: 3).size
p DemoTimerService.new.schedule(4)
p Pathname("/x/api.json").demo_stem, Pathname("/x/lib-1.2").demo_extname
dev2 = DemoEvaluate.new #: untyped
p DemoEvaluateRun.go(dev2)
p DemoChain2.blocked_by(nil, :io, [3, 4]) { |a, b| a + b }.run, DemoBlocked2.blocked_by(nil, 5).run
p DemoCurl.curl(print_stdout: :err), DemoCurl.curl(secrets: ["x"], debug: true)
p DemoExec3.exec3(7, chdir: "/"), DemoExec3.exec3(nil)
ta = [:x, 1, true] #: Array[untyped]
tb = [] #: Array[untyped]
p DemoThenTask.new(ta) { |v, sym, n, flag| [v, sym, n, flag] }.run(6), DemoTaskBase.new(tb) { |v| v }.run(1)
p DemoPoster.go(DemoRubyExec.new), DemoPoster.go(DemoImmediateExec.new)
p DemoCaskDepsUser.deps([DemoCaskDeps.new, Object.new].first)
p DemoChdir.new(Pathname.new("/")).run
p DemoAPIRun.run_api
p DemoDeep.go
dsc = DemoSandboxCmd.new
dsa = dsc.options
dsc.sandbox_inheritance = $stdout
p dsa, dsc.options
dn = DemoNode[1, DemoNode[2, nil]]
p dn.value, dn.nxt.value
dcp2 = DemoPromises.chain(:io)
p dcp2.run, dcp2.chained?, DemoPromises.make(DemoPromEvent.new)
rc = DemoEnvConfig.curlrc
p rc&.start_with?("/"), (rc ? 1 : 2)
dcp = DemoCachedPool.new
dcp.ns_initialize({ size: 5 })
p dcp.size
p DemoThen.new([3]) { |x| x * 10 }.run, DemoExec.new.post(1, 2) { |a, b| a + b }
ENV["DEMO_TOKEN"] = "s"
p ENV.demo_clear_sensitive! { ENV["DEMO_TOKEN"] }, ENV["DEMO_TOKEN"], ENV.demo_sensitive?("a_password")
p([DemoPostExec.new, DemoTimerSet.new].map { |x| x.run_all([1, 2]) })
p DemoTimerSet.new.post(7, 1) { |v| v * 3 }
dev = DemoEvent.new
dev.set
dev.set
p dev.count
dfe = DemoFallbackExec.new
dfe.go
p dfe.log
div = DemoIVar.new
div.add_observer { |x| x * 2 }
div.with_observer { |x| x + 1 }
p div.demo_notify(10)
lines = [] #: Array[untyped]
DemoLineSource.new.each_line_from(%w[a b]) { |t, l| lines << [t, l] }
p lines
nlog = [] #: Array[String]
p Demo::Lenient.first_over([1, 5, 20, 30], 10, nlog), Demo::Lenient.first_over([1], 10, nlog), nlog
dlog = [] #: Array[String]
p Demo::Lenient.depth_find(3, dlog), dlog, Demo::Lenient.void_early(dlog), dlog
dd = DemoDelayish.new
p dd.dupv, dd.value_with(:v)
dv = DemoVisitor.new
p dv.collect_yields, dv.as_proc("ab").call, dv.pair
p DemoTailCli.new.run(%w[-q x --help])
DemoBoundFormula.bottle { root_url "x" }
dbr1 = DemoBoundFormula.stable.bs.root
DemoBoundFormula.via { bottle { root_url "y" } }
p dbr1, DemoBoundFormula.via.bs.root
p DemoNeedsBlockSub.new.ok?
dus = DemoUrlStruct.deserialize('{"desc":"d","url_args":["https://x",{"tag":"v1"}]}')
p dus.url_args, DemoUrlStruct.deserialize('{"desc":"e"}').url_args
p DemoRebind.version_rebuild("1.0", 2, "arm"), DemoRebind.version_rebuild("1.0", 0), DemoRebind.version_rebuild("1.0", 3)
p DemoClassS.class_s("wget"), DemoClassS.class_s("foo-bar@2"), DemoClassS.class_s("a+b")
dcv = DemoCvCollector.new
dcv1 = dcv.building?
dcv.building!
p dcv1, dcv.building?, DemoCvCollector.new.building?
p DemoDlg.go
p DemoCmp.go
p DemoFreshBang.go
p DemoBoundBlk.go
p DemoTokNew.go
p DemoDupSuper.go
p DemoSharedIvar.go
p DemoComputedSendBlock.go
p DemoTruthySelect.go
p DemoSymbolBlank.go
p DemoLazySplat.go
p DemoStanza.go
p DemoDelegBlock.go
p [DemoHoistUnderscore.go(true), DemoHoistUnderscore.go(false)]
p DemoSetDlg.go
p DemoBoolDyn.go

require "demo/views"
ints = [3, 1, 2] #: untyped
strs = ["2", nil, "1"] #: untyped
pairs = [[1, "a"], [2, "b"]] #: untyped
p Demo::Views.blocks(ints), Demo::Views.blocks(strs)
p Demo::Views.blocks(pairs).size
p Demo::Views.iterate(ints), Demo::Views.iterate(pairs)
m = [1, 2, nil, 3] #: untyped
p Demo::Views.mutate(m), m
p Demo::Views.search(ints.sort), Demo::Views.search([1, 5, 9, 12])
p Demo::Views.errors(ints.dup)
names = %i[select filter find_all reject find detect take_while drop_while all? any? none? one? select! reject! keep_if delete_if]
p Demo::Views.sent(ints.dup, names), Demo::Views.sent(strs.dup, names), Demo::Views.sent(pairs.dup, names)
p Demo::Views.sent_more(ints), Demo::Views.sent_more(strs.dup)
ha = { "a" => 1, "b" => 2, "c" => nil } #: untyped
p Demo::Views.hash_blocks(ha)
hm = { "a" => "1", "b" => nil, "c" => "3" } #: untyped
p Demo::Views.hash_mutate(hm)
p Demo::Views.core(ints.dup), Demo::Views.core(strs.dup)
p Demo::Views.core_errors(ints), Demo::Views.core_errors(strs)
hdt = Hash.new(0) #: Hash[String, Integer]
hdt["a"] = 1
hd = hdt #: untyped
hpt = Hash.new { |_h, key| key.size } #: Hash[String, Integer]
hpt["b"] = 2
hp = hpt #: untyped
hs = { "x" => "1", "y" => nil } #: untyped
p Demo::Views.hash_core(ha, "a"), Demo::Views.hash_core(hs, "x"), Demo::Views.hash_core(hd, "a"), Demo::Views.hash_core(hp, "b")
p Demo::Views.hash_core_errors(ha, "a"), Demo::Views.hash_core_errors(hs, "x")
p Sigdemo::Meter.new.add(2).push(3).total
sm = Sigdemo::Meter.new.add(1).absorb(Sigdemo::Meter.new.add(4).push(5))
p sm.total, sm.label({ "name" => "m", "kind" => "meter" })
sm.each_reading(->(r) { p r })
p DemoKeyed.new.add("ab").add("cde").probe_both
p DemoKeyed.new.add("ab").add("cde").probe_fetches
p DemoOwnerSpec.go
p DemoLateIvar.go
p DemoRead.go
p DemoCached.go
p DemoRefs.go
p DemoObjDyn.run
p DemoVerify.go(DemoSum.new("aa"))
DemoUrlCache.seen = { "a" => "b", "c" => "d" }
p DemoUrlCache.seen_count
DemoUrlCache.seen = nil
p DemoUrlCache.seen_nil?
p DemoStash.new.shape&.kind
p DemoDepList.new.deps
p DemoFlight.go
p Demo::BothB.new.tag
m = Sigdemo::Meter.new.add(2)
p m.scaled(3), m.scaled(3, 1), m.summary(a: 1)
