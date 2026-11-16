# frozen_string_literal: true

# Records, for every Ruby method called under gems/, the classes each parameter held: what a run under MRI observes,
# as evidence for typing gem code (docs/research/runtime-speed-2026-11.md). A dev aid, not part of the build.
# usage: OBS_OUT=observed.tsv ruby -r ./observe.rb -I ... main.rb args...   (OBS_MATCH: a path substring, default /gems/)
# Columns: file, line, class, method, calls, name=Class:count,...;..., return classes (keyed by the return's line:
# not joined to the call's row yet).
OBS = Hash.new { |h, k| h[k] = { calls: 0, params: Hash.new { |p, n| p[n] = Hash.new(0) }, ret: Hash.new(0) } }
OBS_MATCH = ENV.fetch("OBS_MATCH", "/gems/")
OBS_CLASS = Kernel.instance_method(:class)

obs_call = TracePoint.new(:call) do |tp|
  next unless tp.path.include?(OBS_MATCH)

  rec = OBS[[tp.path, tp.lineno, tp.defined_class.to_s, tp.method_id]]
  rec[:calls] += 1
  b = tp.binding
  tp.parameters.each do |kind, name|
    next unless name && %i[req opt key keyreq].include?(kind)

    v = b.local_variable_get(name)
    rec[:params][name][OBS_CLASS.bind_call(v)] += 1
  end
end
obs_ret = TracePoint.new(:return) do |tp|
  next unless tp.path.include?(OBS_MATCH)

  OBS[[tp.path, tp.lineno, tp.defined_class.to_s, tp.method_id]][:ret][OBS_CLASS.bind_call(tp.return_value)] += 1
end

at_exit do
  obs_call.disable
  obs_ret.disable
  File.open(ENV.fetch("OBS_OUT", "observed.tsv"), "w") do |f|
    OBS.each do |(path, line, klass, name), rec|
      params = rec[:params].map { |n, cs| "#{n}=#{cs.sort_by { |_, c| -c }.map { |k, c| "#{k}:#{c}" }.join(",")}" }.join(";")
      ret = rec[:ret].sort_by { |_, c| -c }.map { |k, c| "#{k}:#{c}" }.join(",")
      f.puts [path.sub(%r{.*/gems/}, ""), line, klass, name, rec[:calls], params, ret].join("\t")
    end
  end
end
obs_call.enable
obs_ret.enable
