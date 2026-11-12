# prelude/module.rb
# rbs_inline: enabled

#
# Class objects. Every class and module has one: an instance of its
# metaclass, which the transpiler generates and which inherits from Class
# (or Module, for modules). Each metaclass also gets a generated constant
# table, in definition order; MRI orders `constants` by its symbol table,
# so programs should not depend on the order.

class Module < Object
  # Overridden by every class object's generated name.
  #: () -> String
  def name = ""

  #: () -> String
  def to_s = name

  #: () -> String
  def inspect = name

  # Case equality: obj is an instance of this class (or includes this
  # module). The class object's generated _IsInstance answers (decision 76).
  #: (untyped) -> bool
  def ===(obj) = %x{ Boolean(rbIsInstanceOf(self, obj)) }

  #: (?bool) -> Array[Symbol]
  def constants(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, c := range any(self).(rbConstTable)._Consts("") {
      if p, ok := c.value.(rbEvalConst); ok && !*p.defined {
        continue
      }
      if bool(inherit) || !c.inherited {
        out.s = append(out.s, Symbol(c.name))
      }
    }
    return out
  }

  # Public instance methods from a generated table, without Object's and
  # Kernel's (decision 77). instance_methods is the same: there is no
  # protected.
  #: (?bool) -> Array[Symbol]
  def public_instance_methods(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, m := range any(self).(interface{ _Methods() []rbConst })._Methods() {
      if bool(inherit) || !m.inherited {
        out.s = append(out.s, Symbol(m.name))
      }
    }
    return out
  }

  #: (?bool) -> Array[Symbol]
  def instance_methods(inherit = true) = public_instance_methods(inherit)

  # Protected instance methods, from their own generated table (decision 331).
  #: (?bool) -> Array[Symbol]
  def protected_instance_methods(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, m := range any(self).(interface{ _ProtectedMethods() []rbConst })._ProtectedMethods() {
      if bool(inherit) || !m.inherited {
        out.s = append(out.s, Symbol(m.name))
      }
    }
    return out
  }

  # Private instance methods from a generated table (decision 181).
  #: (?bool) -> Array[Symbol]
  def private_instance_methods(inherit = true) = %x{
    out := &Array[Symbol]{}
    for _, m := range any(self).(interface{ _PrivateMethods() []rbConst })._PrivateMethods() {
      if bool(inherit) || !m.inherited {
        out.s = append(out.s, Symbol(m.name))
      }
    }
    return out
  }

  # Its own singleton methods (Kernel's module functions too), from a generated table (decision 181).
  # ponytail: an extended module's methods and a superclass's singletons are left out.
  #: (?bool) -> Array[Symbol]
  def singleton_methods(all = true) = %x{
    out := &Array[Symbol]{}
    for _, m := range any(self).(interface{ _SingletonMethods() []rbConst })._SingletonMethods() {
      out.s = append(out.s, Symbol(m.name))
    }
    return out
  }

  # public_methods(false) is the singleton methods; with true MRI adds Module's and Object's, which this leaves out (ponytail).
  #: (?bool) -> Array[Symbol]
  def public_methods(all = true) = singleton_methods(all)

  #: (untyped, ?bool) -> bool
  def private_method_defined?(name, inherit = true) = private_instance_methods(inherit).include?(__method_sym(name))

  # A method name argument: a Symbol or String, else MRI's TypeError.
  #: (untyped) -> Symbol
  def __method_sym(name)
    return name if name.is_a?(Symbol)
    return name.to_sym if name.is_a?(String)

    raise TypeError, "#{name.inspect} is not a symbol nor a string"
  end

  # Itself, its modules and its superclasses' (with Object, Kernel and BasicObject), from a generated table.
  #: () -> Array[Module]
  def ancestors = %x{
    out := &Array[ModuleI]{}
    if t, ok := any(self).(interface{ _Ancestors() []any }); ok {
      for _, k := range t._Ancestors() {
        out.s = append(out.s, k.(ModuleI))
      }
    }
    return out
  }

  #: () -> Array[Module]
  def included_modules = %x{
    out := &Array[ModuleI]{}
    if t, ok := any(self).(interface{ _Ancestors() []any }); ok {
      for _, k := range t._Ancestors() {
        if m, ok := k.(interface{ _Kind() string }); ok && m._Kind() == "module" {
          out.s = append(out.s, k.(ModuleI))
        }
      }
    }
    return out
  }

  # Module#include called at run time (liquid's `send(:include, filter)`): only a module the traced run saw this
  # class include, compiled in as a latent include (decision 331). A class body's `include` is collected statically.
  #: (*untyped) -> self
  def include(*mods)
    mods.each { |m| __include_at_run(m) }
    self
  end

  #: (untyped) -> void
  def __include_at_run(mod) = %x{ rbIncludeAtRun(self, mod) }

  # Whether mod is among the modules self includes, its ancestors' too (Homebrew's `cmd_class&.include?(ShellCommand)`).
  #: (untyped) -> bool
  def include?(mod) = %x{
    if k, ok := mod.(interface{ _Kind() string }); !ok || k._Kind() != "module" {
      panic(NewTypeError(Ref(String("wrong argument type " + rbClassName(mod) + " (expected Module)"))))
    }
    if t, ok := any(self).(interface{ _Ancestors() []any }); ok {
      for _, k := range t._Ancestors() {
        if m, ok := k.(interface{ _Kind() string }); ok && m._Kind() == "module" && k == mod {
          return true
        }
      }
    }
    return false
  }

  #: (untyped, ?bool) -> bool
  def public_method_defined?(name, inherit = true) = public_instance_methods(inherit).include?(__method_sym(name))

  #: (untyped, ?bool) -> bool
  def method_defined?(name, inherit = true) = public_method_defined?(name, inherit)

  # Every constant of the closed world is defined already: none waits on an autoload (decision 324).
  #: (untyped, ?bool) -> String?
  def autoload?(name, inherit = true) = nil

  # An autoload with run-time arguments (rubocop's `register_cop` calls `autoload constant_name, path`): the files the
  # program loads are compiled in already, so there is nothing to register (decisions 324, 444).
  #: (untyped, untyped) -> nil
  def autoload(name, path) = nil

  # The transpiler narrows the result type: a literal name gets that
  # constant's type, any other name the join of the module's constants.
  #: (untyped, ?bool) -> untyped
  def const_get(name, inherit = true) = %x{
    val, missing := rbConstResolve(self, name, bool(inherit))
    if missing != "" {
      panic(NewNameError(Ref(String(missing))))
    }
    return val
  }

  #: (untyped, ?bool) -> bool
  def const_defined?(name, inherit = true) = %x{
    _, missing := rbConstLookup(self, name, bool(inherit), false)
    return Boolean(missing == "")
  }

  # The value, as MRI's const_set returns (decision 231). ponytail: the constant table is fixed at compile time, so
  # const_get/constants do not see a name set here; boot (decision 161) could declare what load-time const_sets make.
  #: [T] (untyped, T) -> T
  def const_set(name, value)
    n = __method_sym(name).to_s
    raise NameError, "wrong constant name #{n}" unless n.match?(/\A[A-Z]\w*\z/)

    value
  end
end

class Class < Module
  # The parent's class object; nil above Object, whose BasicObject has none in rugaroo.
  #: () -> Class?
  def superclass = %x{
    t, ok := any(self).(interface{ _Superclass() any })
    if !ok {
      return nil
    }
    k, ok := t._Superclass().(ClassI)
    if !ok {
      return nil
    }
    return &k
  }

  #: () -> Array[Class]
  def subclasses = %x{
    out := &Array[ClassI]{}
    if t, ok := any(self).(interface{ _Subclasses() []any }); ok {
      for _, k := range t._Subclasses() {
        out.s = append(out.s, k.(ClassI))
      }
    }
    return out
  }
end
