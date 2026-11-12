# Boot-snapshot tracing hook (docs/design.md decision 161).
#
# Loaded with `ruby -I <hookdir> -r boot_hook <entry>` before the program's
# own requires. It records what a boot does that the compiler cannot read from
# source: methods built by define_method or a string eval, the files autoload
# actually loaded, every class/ancestor defined, and the objects held in class
# state. The manifest is written as JSON to the file named by RUGAROO_BOOT_OUT.

module RugarooBoot
  @classes = {} # name => { name:, super:, path:, line: }
  @define_methods = [] # { owner:, name:, file:, line:, source: }
  @aliases = []        # { owner:, singleton:, name:, old:, file:, line: }
  @includes = []       # { owner:, site:, module:, file:, line: }
  @evals = []          # { file:, line:, source:, caller: }
  @autoloads = []      # { owner:, const:, path:, registered: }
  @features = nil

  class << self
    attr_reader :classes, :define_methods, :aliases, :evals, :autoloads, :pid

    # Valid UTF-8, since JSON raises on a binary or invalid String.
    def text(s)
      s.is_a?(String) ? s.dup.force_encoding(Encoding::UTF_8).scrub : s
    end

    def install
      @pid = $$
      require "coverage"
      Coverage.start(branches: true) # which arm of each load-time if boot took (decision 201)
      @features = $LOADED_FEATURES.dup
      install_class_trace
      install_define_method
      install_alias_method
      install_include
      install_evals
      install_const_set
      install_autoload
      install_private_constant
      @delegators = {}
    end

    attr_reader :delegators

    # Forwardable's delegated names by call site, for a list built at load time (`*(DSL_METHODS - [:x])`, decision 204).
    def install_forwardable
      return if @forwardable_hooked || !defined?(::Forwardable) || !::Forwardable.method_defined?(:def_delegators) # not while forwardable.rb itself loads

      @forwardable_hooked = true
      ::Forwardable.prepend(Module.new do
        (%i[def_delegators def_instance_delegators] & ::Forwardable.instance_methods(false)).each do |m|
          define_method(m) do |accessor, *methods|
            loc = caller_locations(1, 1).first
            ::RugarooBoot.delegators["#{loc.path}:#{loc.lineno}"] ||= methods.map(&:to_s) if loc
            super(accessor, *methods)
          end
        end
      end)
    end

    # Module#constants omits private ones; const_classes needs them too.
    def install_private_constant
      @private_constants = Hash.new { |h, k| h[k] = [] }
      Module.prepend(Module.new do
        def private_constant(*names)
          ::RugarooBoot.private_constants[self].concat(names)
          super
        end
      end)
    end

    def private_constants = @private_constants

    def install_class_trace
      @class_tp = TracePoint.new(:class) do |tp|
        install_forwardable
        klass = tp.self
        next unless klass.is_a?(Class) || klass.is_a?(Module)

        name = safe_name(klass)
        next if name.nil? || name.empty? || @classes.key?(name)

        @classes[name] = { "name" => name, "super" => safe_name(klass.is_a?(Class) ? klass.superclass : nil),
                           "path" => tp.path, "line" => tp.lineno }
      end
      @class_tp.enable
    end

    # TracePoints, not a prepended wrapper: MRI's define_method reads the caller's scope (`module_function` makes
    # it a module function too), which a wrapper's `super` would hide. The caller's locals come from the last :line
    # event when it is the define_method call's own line: what the block captures, recorded when it is a literal
    # (decision 198).
    def install_define_method
      @line_bindings = {} # the latest binding of each executed line: a define_method call's own, though its args ran other lines
      @line_tp = TracePoint.new(:line) { |tp| @line_bindings["#{tp.path}:#{tp.lineno}"] = tp.binding }
      @line_tp.enable
      calls = []
      @define_method_tp = TracePoint.new(:c_call, :c_return) do |tp|
        next unless tp.method_id == :define_method || tp.method_id == :define_singleton_method

        if tp.event == :c_call
          b = @line_bindings["#{tp.path}:#{tp.lineno}"]
          calls << [tp.path, tp.lineno, b]
          next
        end
        path, line, b = calls.pop
        name = tp.return_value
        owner = tp.method_id == :define_singleton_method ? tp.self.singleton_class : tp.self
        next unless owner.is_a?(Module) && name.is_a?(Symbol)

        singleton = owner.singleton_class?
        named = singleton ? owner.attached_object : owner
        m = (owner.instance_method(name) rescue nil)
        from = m && m.original_name != name ? m.owner : nil
        @define_methods << {
          "owner" => safe_name(named), "singleton" => singleton, "name" => name.to_s, "file" => path, "line" => line,
          "source" => nil, "from" => safe_name(from), "private" => owner.private_method_defined?(name),
          "module_function" => !singleton && named.singleton_class.method_defined?(name) &&
                               named.singleton_class.instance_method(name).source_location == m&.source_location,
          "captures" => captures(b)
        }
      end
      @define_method_tp.enable
    end

    # Each alias_method MRI ran, by its caller's line: a computed name (`%w[b i].each { |t| alias_method("convert_#{t}", :convert_em) }`)
    # becomes the alias it made (decision 330). The old name is the alias's original_name, read after the call.
    def install_alias_method
      @alias_tp = TracePoint.new(:c_return) do |tp|
        next unless tp.method_id == :alias_method && Module === tp.self && tp.return_value.is_a?(Symbol)

        owner = tp.self
        m = (owner.instance_method(tp.return_value) rescue next)
        singleton = owner.singleton_class?
        @aliases << { "owner" => safe_name(singleton ? owner.attached_object : owner), "singleton" => singleton,
                      "name" => tp.return_value.to_s, "old" => m.original_name.to_s, "file" => tp.path, "line" => tp.lineno }
      end
      @alias_tp.enable
    end

    # Each module an `include` call added during boot (decision 331): a method's `send(:include, filter)` (liquid's
    # Strainer.add_filter) names its module only at run time. An anonymous class is named by its Class.new call site.
    def install_include
      @anon_sites = {}.compare_by_identity
      stack = []
      @include_tp = TracePoint.new(:c_call, :c_return) do |tp|
        case tp.method_id
        when :new
          next unless tp.event == :c_return && tp.self.equal?(Class) && tp.return_value.is_a?(Class)

          @anon_sites[tp.return_value] ||= "#{tp.path}:#{tp.lineno}"
        when :include
          next unless Module === tp.self

          if tp.event == :c_call
            stack << MOD_ANCESTORS.bind_call(tp.self)
            next
          end
          before = stack.pop || []
          owner = tp.self
          (MOD_ANCESTORS.bind_call(owner) - before).each do |m|
            next if m.is_a?(Class)

            @includes << { "owner" => (MOD_NAME.bind_call(owner) unless @anon_sites.key?(owner)), "site" => @anon_sites[owner],
                           "module" => MOD_NAME.bind_call(m), "file" => tp.path, "line" => tp.lineno }
          end
        end
      end
      @include_tp.enable
    end

    # The caller's locals as Ruby expressions: a literal for plain data, else the constant that holds the same object.
    def captures(b)
      return {} unless b

      nesting = (b.eval("Module.nesting") rescue [])
      b.local_variables.each_with_object({}) do |v, out|
        value = b.local_variable_get(v)
        lit = literal(value) || held_by_constant(value, nesting)
        out[v.to_s] = lit if lit
      end
    end

    # `M::C`, `M::C.fetch(key)` or `M::C[i]` when a constant of the lexical scope holds value itself.
    def held_by_constant(value, nesting)
      nesting.each do |mod|
        next unless (name = safe_name(mod))

        mod.constants(false).each do |c|
          next if mod.autoload?(c, false) # resolving it would load a file the program never did (decision 404)

          obj = (mod.const_get(c, false) rescue next)
          return "#{name}::#{c}" if obj.equal?(value)

          case obj
          when Hash
            obj.each { |k, e| return "#{name}::#{c}.fetch(#{literal(k)})" if e.equal?(value) && literal(k) }
          when Array
            obj.each_with_index { |e, i| return "#{name}::#{c}[#{i}]" if e.equal?(value) }
          end
        end
      end
      nil
    end

    def literal(x, depth = 0)
      return nil if depth > 6

      case x
      when Float then x.nan? ? "Float::NAN" : x.infinite? ? "#{"-" if x < 0}Float::INFINITY" : x.inspect # NaN/Infinity inspect as bare words
      when nil, true, false, Integer, Symbol then x.inspect
      when Module then (n = MOD_NAME.bind_call(x)) && (Object.const_get(n) rescue nil).equal?(x) ? "::#{n}" : nil
      when Proc then (loc = x.source_location) && !x.lambda? ? "proc@#{loc[0]}:#{loc[1]}" : nil # rebuilt from its literal block
      when String then x.size > 100_000 ? nil : x.inspect
      when Array
        parts = x.map { |e| literal(e, depth + 1) }
        parts.all? ? "[#{parts.join(", ")}]" : nil
      when Hash
        parts = x.map { |k, e| (kl = literal(k, depth + 1)) && (el = literal(e, depth + 1)) && "#{kl} => #{el}" }
        parts.all? ? "{#{parts.join(", ")}}" : nil
      when ::Set
        parts = x.map { |e| literal(e, depth + 1) }
        parts.all? ? "::Set[#{parts.join(", ")}]" : nil
      end
    end

    # A string eval reaches the compiler only through the manifest. Kernel#eval,
    # Module#class_eval/module_eval and BasicObject#instance_eval are each
    # hooked for their String form; the block form is the source already.
    def install_evals
      Kernel.prepend(Module.new do
        def eval(*args, **kw, &blk)
          ::RugarooBoot.record_eval(args[0], args[2], args[3], "eval", nil) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end
      end)
      Module.prepend(Module.new do
        def class_eval(*args, **kw, &blk)
          ::RugarooBoot.record_eval(args[0], args[1], args[2], "class_eval", self) if args[0].is_a?(String)
          ::RugarooBoot.record_block_eval(blk, self) if blk && args.empty?
          super(*args, **kw, &blk)
        end

        def module_eval(*args, **kw, &blk)
          ::RugarooBoot.record_eval(args[0], args[1], args[2], "class_eval", self) if args[0].is_a?(String)
          ::RugarooBoot.record_block_eval(blk, self) if blk && args.empty?
          super(*args, **kw, &blk)
        end
      end)
      BasicObject.prepend(Module.new do
        def instance_eval(*args, **kw, &blk)
          ::RugarooBoot.record_eval(args[0], args[1], args[2], "instance_eval", self) if args[0].is_a?(String)
          super(*args, **kw, &blk)
        end
      end)
    end

    # `Sets.const_set(name, set)` at load time (rubocop-ast names each NodePattern set as it compiles a pattern): a
    # constant no source line writes. One whose value is a plain literal is recorded as the `NAME = value` its module
    # would have evaluated, so the compiler declares it there (decisions 204, 444). A name the module already has is
    # left alone: its source assigns it.
    def install_const_set
      Module.prepend(Module.new do
        def const_set(name, value)
          fresh = begin
            !const_defined?(name, false)
          rescue ::StandardError
            false
          end
          ::RugarooBoot.record_const_set(self, name, value) if fresh
          super
        end
      end)
    end

    def record_const_set(mod, name, value)
      named = safe_name(mod)
      lit = literal(value)
      return unless named && lit && name.to_s.match?(/\A[A-Z][A-Za-z0-9_]*\z/)

      lit = "#{lit}.freeze" if value.frozen? && (value.is_a?(::Array) || value.is_a?(::Hash) || value.is_a?(::Set) || value.is_a?(::String))
      loc = caller_locations(2, 1).first
      return unless loc

      @evals << { "file" => loc.path, "line" => loc.lineno, "source" => "#{name} = #{lit}\n", "kind" => "const_set", "self" => named,
                  "caller" => "#{loc.path}:#{loc.lineno}" }
    end

    def install_autoload
      hook = Module.new do
        def autoload(name, path)
          ::RugarooBoot.record_autoload(self, name, path)
          super(name, path)
        end
      end
      Module.prepend(hook)
    end

    # Where it was called, what it evaluated, and on what: a named module's class_eval or instance_eval (decision 204).
    def record_eval(src, file, line, kind, recv)
      loc = caller_locations(2, 1).first
      named = recv.is_a?(Module) ? safe_name(recv) : nil
      @evals << { "file" => text(file), "line" => line, "source" => text(src), "kind" => kind, "self" => named,
                  "caller" => loc ? "#{loc.path}:#{loc.lineno}" : nil }
    end

    # `klass.class_eval do def x ... end end` on a module the caller computed: the block's location, compiled into
    # that module as its body (decision 204).
    def record_block_eval(blk, recv)
      loc = blk.source_location
      named = safe_name(recv)
      return unless loc && named

      @evals << { "file" => loc[0], "line" => loc[1], "source" => nil, "kind" => "class_eval_block", "self" => named,
                  "caller" => "#{loc[0]}:#{loc[1]}" }
    end

    def record_autoload(owner, name, path)
      @autoloads << { "owner" => safe_name(owner), "const" => name.to_s,
                      "path" => text(path), "registered" => true }
    end

    def safe_name(obj)
      return nil if obj.nil?

      Module === obj ? MOD_NAME.bind_call(obj) : obj.to_s # not obj.name: a module may define its own (REXML::Functions.name)
    rescue StandardError
      nil
    end

    def resolve_autoloads
      @autoloads.map do |a|
        owner = RugarooBoot.const_at(a["owner"])
        pending = owner.respond_to?(:autoload?) ? owner.autoload?(a["const"]) : nil
        resolved = owner && pending.nil?
        loaded = nil
        if resolved
          base = a["path"].to_s
          loaded = $LOADED_FEATURES.find { |f| f.end_with?("/#{base}.rb") || f == "#{base}.rb" }
        end
        a.merge("resolved" => !!resolved, "loaded" => loaded)
      end
    end

    def class_state
      out = []
      @classes.each_key do |name|
        owner = RugarooBoot.const_at(name)
        next unless owner.is_a?(Module)

        owner.instance_variables.each do |ivar|
          value = owner.instance_variable_get(ivar)
          out << describe_state(name, ivar.to_s, value)
        end
      end
      out
    end

    def describe_state(owner, ivar, value)
      kind = safe_name(value.class)
      recreate = value.is_a?(Mutex) || value.is_a?(Queue)
      desc = case value
             when Mutex, Queue then "<#{kind}>"
             when Hash then "Hash(size=#{value.size}, default=#{!value.default.nil? || !value.default_proc.nil?})"
             when Array then "Array(size=#{value.size})"
             when String then text(value)
             else kind.to_s
             end
      { "owner" => owner, "ivar" => ivar, "kind" => kind, "desc" => desc,
        "recreate" => recreate, "frozen" => value.frozen? }
    end

    # Every if/unless boot reached: its arms' counts, by file and the keyword's line and column.
    def branches
      out = []
      Coverage.result.each do |file, cov|
        (cov[:branches] || {}).each do |(kind, _, line, col), arms|
          next unless %i[if unless case].include?(kind)

          counts = arms.to_h { |(arm, *), n| [arm.to_s, n] }
          out << { "file" => file, "line" => line, "col" => col, "kind" => kind.to_s,
                   "then" => counts["then"].to_i, "else" => counts["else"].to_i,
                   "arms" => arms.map { |(arm, _, l, c), n| { "kind" => arm.to_s, "line" => l, "col" => c, "count" => n } } }
        end
      end
      out
    end

    # Constants whose value is a class named otherwise: `Impl = case ... end` picked at load time.
    MOD_CONSTANTS = Module.instance_method(:constants)
    MOD_AUTOLOAD = Module.instance_method(:autoload?)
    # The constant a qualified name names, or nil: never through a pending autoload, which would load a file the
    # program never did, with this hook watching (decision 404).
    def const_at(name)
      name.to_s.split("::").reject(&:empty?).reduce(Object) do |mod, seg|
        return nil unless mod.is_a?(Module) && MOD_CONST_DEFINED.bind_call(mod, seg, false) && !MOD_AUTOLOAD.bind_call(mod, seg)

        MOD_CONST_GET.bind_call(mod, seg, false)
      end
    rescue StandardError, NameError
      nil
    end

    MOD_NAME = Module.instance_method(:name)
    MOD_ANCESTORS = Module.instance_method(:ancestors)
    MOD_CONST_GET = Module.instance_method(:const_get)
    MOD_CONST_DEFINED = Module.instance_method(:const_defined?)

    def const_classes
      out = {}
      @classes.each_key do |name|
        owner = RugarooBoot.const_at(name)
        next unless Module === owner

        # Module's own methods: a module can make them private for itself (brew's Cask::Quarantine, `extend self`).
        (MOD_CONSTANTS.bind_call(owner, false) | @private_constants[owner]).each do |c|
          next if MOD_AUTOLOAD.bind_call(owner, c)

          v = (MOD_CONST_GET.bind_call(owner, c, false) rescue next)
          next unless Module === v # not v.is_a?: a BasicObject constant (brew's LazyObject) has none

          vn = MOD_NAME.bind_call(v)
          out["#{name}::#{c}"] = vn if vn && vn != "#{name}::#{c}"
        end
      end
      out
    end

    # Each T::Struct's props as MRI ended up with them, for those a load-time loop declared under computed names
    # (Homebrew's FormulaStruct predicates, decision 241): name, Sorbet type text, and a literal default.
    def sorbet_structs
      return {} unless defined?(::T::Struct)

      literal = [NilClass, TrueClass, FalseClass, Integer, Float, String, Symbol]
      out = {}
      ObjectSpace.each_object(Class) do |k|
        next unless k < ::T::Struct && MOD_NAME.bind_call(k)

        props = (k.props rescue next)
        out[MOD_NAME.bind_call(k)] = props.map do |name, rules|
          h = { "name" => name.to_s, "type" => rules[:type_object].to_s }
          if rules.key?(:default)
            d = rules[:default]
            h["default"] = d.inspect if literal.any? { |c| d.is_a?(c) } || ((d.is_a?(Array) || d.is_a?(Hash)) && d.empty?)
          end
          h
        end
      end
      out
    end

    # After the program ran: load each autoload it registered and never touched, so the class bodies in those files run
    # under the hooks (their `class_eval` strings, `const_set`s, aliases). The compiler offers such a file only when
    # compiled code names its constant (decision 444), and it needs what the body evaluated to compile it at all
    # (rubocop's helper mixins define their NodePattern matchers at load). What the program itself loaded is taken
    # first, so the manifest still says which autoloads it resolved.
    def load_unresolved_autoloads(resolved)
      std = [RbConfig::CONFIG["rubylibdir"], RbConfig::CONFIG["archdir"]]
      before = @evals.size
      resolved.each do |a|
        next if a["resolved"] || a["owner"].nil?

        path = a["path"].to_s
        path += ".rb" unless path.end_with?(".rb")
        file = File.absolute_path?(path) ? path : $LOAD_PATH.map { |d| File.join(d.to_s, path) }.find { |f| File.file?(f) }
        next unless file && File.file?(file) && std.none? { |d| file.start_with?("#{d}/") }

        owner = RugarooBoot.const_at(a["owner"])
        next unless owner.is_a?(Module)

        begin
          owner.const_get(a["const"])
        rescue ::Exception # rubocop:disable Lint/RescueException -- a file that cannot load here is simply not offered
          nil
        end
      end
      @evals[before..].each { |e| e["lazy"] = true }
      @lazy_features = $LOADED_FEATURES - @features
    end

    def dump(path)
      autoloads = resolve_autoloads
      loaded_features = $LOADED_FEATURES - @features
      state = class_state
      load_unresolved_autoloads(autoloads)
      @class_tp&.disable
      @line_tp&.disable
      @alias_tp&.disable
      @include_tp&.disable
      classes = @classes.map do |name, info|
        owner = RugarooBoot.const_at(name)
        anc = owner.is_a?(Module) ? owner.ancestors.map { |a| safe_name(a) } : []
        info.merge("ancestors" => anc)
      end
      manifest = {
        "loaded_features" => loaded_features,
        "load_order" => (@lazy_features || loaded_features), # every file in the order its load finished, the lazy phase's last
        "load_path" => $LOAD_PATH.map(&:to_s),
        "std_dirs" => [RbConfig::CONFIG["rubylibdir"], RbConfig::CONFIG["archdir"]], # MRI's own library: a file there is never a source (decision 404)
        "classes" => classes,
        "define_methods" => @define_methods,
        "aliases" => @aliases,
        "includes" => @includes,
        "evals" => @evals.uniq { |e| [e["source"], e["kind"], e["self"], e["caller"]] },
        "autoloads" => autoloads,
        "class_state" => state,
        "branches" => branches,
        "const_classes" => const_classes,
        "delegators" => @delegators,
        "structs" => sorbet_structs,
      }
      File.write(path, generate(manifest))
    end

    # json loads at dump, from the stdlib dirs first: the program's own `require "json"` resolves as without the hook, and a local json.rb cannot shadow ours.
    def generate(manifest)
      require "rbconfig"
      saved = $LOAD_PATH.dup
      begin
        $LOAD_PATH.unshift(RbConfig::CONFIG["rubylibdir"], RbConfig::CONFIG["archdir"])
        require "json"
      ensure
        $LOAD_PATH.replace(saved)
      end
      JSON.pretty_generate(manifest)
    end
  end
end

# racc's C extension is not compiled (pureFallback, decision 427): its own switch makes boot take the Ruby routines the
# program will, so the class_eval'd `do_parse` names `_racc_do_parse_rb`, not `_racc_do_parse_c`.
Racc_No_Extensions = true
RugarooBoot.install
at_exit { RugarooBoot.dump(ENV["RUGAROO_BOOT_OUT"]) if ENV["RUGAROO_BOOT_OUT"] && $$ == RugarooBoot.pid } # a forked child exits through here too
