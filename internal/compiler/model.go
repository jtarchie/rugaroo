package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/jtarchie/rugaroo/internal/rbs"
)

// Class is a Ruby class or module.
type Class struct {
	Name       string // Go identifier: "Resty_Actions_Show"
	RubyName   string // constant path: "Resty::Actions::Show"
	IsModule   bool
	GoType     string // `@go_type` underlying Go type; "" for struct classes
	hashBase   bool   // a user class extending Hash or Array: embeds a Hash[any, V] or an Array[E] (decisions 165, 202)
	selfDefs   bool   // some method has `# @self` (selfOverload)
	TypeParams []string
	superRef   *constRef   // superclass expression, resolved in link
	reSupers   []*constRef // superclasses named when reopening; must resolve to Super
	Super      *Class
	Includes   []Include
	Methods    map[string]*Method
	MethodList []*Method
	Ivars      map[string]*Ivar
	IvarList   []*Ivar
	srcPos
	Subclasses []*Class
	// universal classes (BasicObject, Object, Kernel) are ancestors of the
	// primitive types too, so their free funcs take `Self any`.
	universal bool
	// ivar type annotations `# @rbs @x: T`, resolved in resolveSigs
	ivarDecls         []ivarDecl
	singletonDefs     []singletonDef
	instanceDelegator *instanceDelegator // `class << self; def method_added(m) = singleton_class.def_delegator(:instance, m)`: prime's (decision 410)
	perRun            []string           // a Class.new literal whose block runs statements: a fresh class object per evaluation, its body `self.rugaroo_class_body` taking these captured locals (decision 259)
	isPerRun          bool
	singletonMisc     []singletonMisc   // attr_*/alias inside `class << self`, applied to the metaclass once it exists
	privateNew        bool              // `private_class_method :new`
	undefs            map[string]bool   // `undef x`: calls through this class find nothing, even inherited
	cvars             map[string]*Const // @@x first assigned in this body, emitted as package variables
	extends           []Include         // `extend M`: included into the class object
	mixesIn           []Include         // Sorbet's `mixes_in_class_methods(M)` in a module: its includers extend M (decision 241)
	includers         []*Class          // a module's: the classes and modules that include it (linkInclude)
	delegations       []delegation
	singleFwd         bool       // `extend SingleForwardable` seen: def_single_delegator(s) and single_delegate work
	fwdSingle         bool       // the later of Forwardable/SingleForwardable extended was SingleForwardable: def_delegator(s) and delegate define class methods
	meta              *Class     // the class object's class (holds `def self.` methods)
	metaOf            *Class     // for a metaclass: the class it describes
	constNames        []string   // constants (classes included) declared directly inside, in order
	value             *valueSpec // set on a Struct.new / Data.define class
	msetCache         []entry
	msetIndex         map[string]int // name → index in msetCache; the dynamic pass asks every class about every name
	selfCallCache     map[string]bool
	gemSelfCalls      map[string]bool   // what gem methods added to a core module call on self: dynamic, not in the constraint (selfCalls)
	scratchIvars      map[string]*Ivar  // a gem module a value class (Pathname) includes: its ivars are per-call locals
	prepended         []*Class          // modules prepended to it, in prepend order: ancestors lists them before it, last first
	slotsLinked       bool              // linkOverrides ran
	Display           string            // the name Ruby shows, when the declaration has none of its own (a describe's class)
	specChild         bool              // a nested describe's class: MRI undefines the test methods it inherits
	specTests         int               // its it/specify count, for MRI's test_0001_ names
	hidden            bool              // `# @hidden` on a prelude module MRI lacks: left out of ancestors (decision 139)
	ivarBits          bool              // its objects record which ivars were assigned (decision 153)
	template          *Class            // an instance's generic class (decision 308)
	instArgs          []Type            // an instance's type arguments
	instances         []*Class          // a template's instances
	superArgs         []rbs.Type        // `class Min < Base #[E]` / a sig's `< Base[E]`: the superclass's type arguments (decision 309)
	superInst         *Class            // the superclass instance superArgs name, made with the instances
	tmpl              bool              // a template: markTemplates decided before link
	bounds            map[string]string // a template's `generic P < Comparable` bounds, by parameter (decision 312)
	helperOf          *Class            // a helper template: the generic class it is nested in, whose arguments it takes (decision 311)
}

// displayName is the class's name as Ruby shows it.
func (c *Class) displayName() string {
	if c.Display != "" {
		return c.Display
	}
	return c.RubyName
}

type ivarDecl struct {
	name  string
	rbs   rbs.Type
	line  int
	scope []*Class
}

// classBodyLit is a user class body's unannotated `@x = <scalar literal>`: the class object's ivar, typed by the
// literal when no method's write typed it (decision 383).
type classBodyLit struct {
	ivarDecl
	cls *Class
}

// constRef is a constant expression to resolve once every class is known.
type constRef struct {
	node  parser.Node
	scope []*Class
	file  *File
}

// Include is `include Mod #[Args]`.
type Include struct {
	Mod      *Class
	Args     []Type
	fromEach bool // no type args written: they are the includer's each's block parameters (decision 146)
	latent   bool // a run-time include boot saw (decision 331): its methods are there, include? and ancestors wait for the call
	ref      constRef
	args     []rbs.Type
	srcPos
	scope []*Class
}

// Const is a non-class constant (`VERSION = "1.0"`), emitted as a Go
// package-level variable.
type Const struct {
	RubyName string
	GoName   string
	Value    parser.Node
	srcPos
	Scope     []*Class // lexical scope of the assignment
	ann       string   // trailing `#: T`
	typeAlias string   // `X = T.type_alias { T }`: the RBS type X names in a sig (decision 196)
	poison    string   // a gem constant whose value does not compile: why; it stays nil (decision 205)
	Type      Type
	resolving bool
	guarded   bool     // may be read before its assignment runs (guardConsts)
	inMethod  bool     // a class variable first assigned in a method: no init in main, reads check its set flag
	inBody    bool     // reads a local of its class body: assigned in the body's grouped statements (decision 170)
	inMain    bool     // a top-level constant reading a top-level local: assigned in place in its file's top level (decision 296)
	initAt    *int     // a constant lifted out of another's value: the offset it initializes at (decision 244)
	first     *Const   // a later `X = v` of the constant again: assigns first's variable, MRI's warning first (decision 363)
	rewrites  []*Const // this constant's later assignments
}

// Ivar is an instance variable of a struct class.
type Ivar struct {
	Name    string // with @
	Type    Type
	Owner   *Class
	open    bool      // first assigned an unannotated `[]`/`{}` (refineIvars)
	elems   [2][]Type // what the class's methods put in it
	scratch bool      // a value-included gem module's: a local of each method (declareGemModuleIvars)
	joins   bool      // a user ivar declared by its first write, unannotated: later writes join in (joinIvar)
}

type methodKind int

const (
	kindDef methodKind = iota
	kindPrimitive
	kindAttrReader
	kindAttrWriter
	kindSynth // generated per metaclass: new, name, to_s, inspect
)

// Method is a Ruby method definition.
type Method struct {
	ivarUse      *bool                // mentionsIvar's answer, once asked
	attrWriter   bool                 // an unannotated attr_writer/attr_accessor's `def x=(v) = @x = v` (addModuleAttr)
	blocklessOf  *Method              // a synthesized `__name_enum`: the gem method whose `return x unless block_given?` it answers (decision 418)
	pendingIdx   map[int]pendingParam // sig param index → a parameter typed from use (decision 146)
	pendingBlock []pendingParam       // the block's parameters, typed from the yields
	// yields that disagree on their count, reported at the yield (yieldShape)
	yieldDisagree   string
	yieldDisagreeAt parser.Node
	Name            string
	GoName          string
	Owner           *Class // nil for top-level defs
	Kind            methodKind
	Attr            string // ivar for attr kinds
	Private         bool
	Protected       bool // callable with an explicit receiver only from inside the owner's family
	Node            *parser.DefNode
	srcPos
	Scope       []*Class // lexical scope (Module.nesting), innermost last
	sigText     string
	declared    bool   // a `#:` or Sorbet sig typed it: its param types are the author's, not guessed (decision 241)
	retWhy      string // RUGAROO_UNTYPED_WHY: why inferRet left the return untyped
	roundRet    bool   // untyped only this inference round, its parameters typed from use not known yet: no local widens to it (decision 358)
	retUnion    Type   // a gem sig's `U | String` return, untyped as declared (decision 172): a call that leaves U unbound gets String (decision 321)
	sig         *rbs.MethodType
	arms        []*rbs.MethodType // a `.rbs` sig's overloads; sig is their merge, the call site picks one (decision 12)
	TypeParams  []string
	Params      []Param
	Block       *BlockSig
	Ret         Type
	Iterator    bool   // block returns void → iter.Seq
	seqAdapter  bool   // a closure overriding an iterator: GoName gains _blk, an iter.Seq adapter keeps the name
	seqRest     bool   // an iterator `each(*args)` under Enumerable: GoName gains _rest, a no-argument iter.Seq adapter keeps the name
	shadowed    []slot // ancestors' interface slots this override's signature differs from, nearest first; adapters answer them
	BlockParam  string // name of an explicit &block parameter
	forwardAll  bool   // `def f(...)`: its signature is its forwarding target's (resolveForwarding)
	resolved    bool
	inherited   *Method // signature source for unannotated overrides
	inferRet    bool    // no return annotation: Ret comes from the body (inferRet)
	inferring   bool
	recursed    bool    // inferRet met a call to itself while inferring: its return is a fixpoint (decision 355)
	structDef   *Method // the generated Struct/Data method a block def overrides; super reaches it
	superTarget *Method // a prepended module's copy: the class's own method it wraps, which super calls (decision 199)
	fwdTarget   *Class  // a delegation's `(...)` forwarder: the class it forwards to
	poison      string  // a gem method whose body does not compile: why; its body raises NotImplementedError (decision 205)
	freeName    string  // its free func's name when Owner_Name is taken by a class (separateFreeFuncs)
	valueGen    bool    // generated for Struct/Data: a pattern reads the members directly (decision 143)

	calleeDefaults bool // Ruby runs defaults in the callee: Go takes rbArgc first, callers pass zero values for the rest
	superBridge    bool // a module method whose `super` target depends on the includer (superBridges)
	quietDynamic   bool // prelude `# @dynamic`: dynamic by design, so no dynamic-call warnings (decision 78)
	binary         int8 // prelude `# @binary`: 1 its String result is binary, 2 when its receiver is (`@binary self`, decision 352)
	binBlock       bool // `# @binary block`: its result and its block's String params are binary
	specForm       bool // declared by a Minitest::Spec DSL call, not a def (decision 83)
	noReturn       bool // RBS `bot`: a call ends its statement list, like raise (decision 91)
	selfText       string
	SelfType       Type // prelude `# @self T`: the receiver must unify with T, binding the method's type params (decision 92)
}

// root is the topmost struct class of c's hierarchy (below Object).
func (c *Class) root() *Class {
	k := c
	for k.Super != nil && !k.Super.universal {
		k = k.Super
	}
	return k
}

// Param is a positional parameter.
type Param struct {
	Name    string
	Type    Type
	Default parser.Node // literal default for optional params
	Rest    bool
	Keyword bool // `name:`: passed by name, a positional Go param after the positional ones
	Post    bool // after *rest or the optional params (`def f(a, *r, z)`): bound from the last arguments
	KwRest  bool // `**opts`: the call's other keywords, a Hash[Symbol, T]
	// Want is T for a `T | untyped` parameter (Type is untyped): typed
	// arguments are checked against T, untyped ones pass as they are.
	Want Type
	// Pending keys a parameter typed from use (decision 146): its
	// arguments are recorded under it during inference rounds.
	Pending pendingKey
	// Out: a String parameter the method appends to, passed as *String: copied in, and back out on return (decision 405).
	Out bool
}

// BlockSig is the block a method takes.
type BlockSig struct {
	Params      []Type
	Ret         Type
	Optional    bool         // `?{ ... }`: a call may leave the block out, and the method sees a nil func
	Pending     []pendingKey // per param, the key of one typed from the yields (decision 146), or ""
	Rest        bool         // a lambda's: the last param is a `*rest`'s element type, a Go variadic (decision 152)
	Self        Type         // non-nil: the block escaped (stored, instance_eval'd) and takes `self` as a leading Go param (decision 164)
	SelfPending pendingKey   // the key Self's type is inferred from (decision 164): instance_eval records the receiver, Proc#call the lexical self
	Untyped     bool         // an unannotated def's &block (decision 183): calling it when missing is MRI's NoMethodError at run time
	Src         lambdaSrc    // an unannotated def's &block: the key its Proc's calls record their arguments under (decision 307)
}

func (m *Method) generic() bool { return len(m.TypeParams) > 0 }

// slot is method entry e as its Go signature appears in class in's interface.
type slot struct {
	e     entry
	in    *Class
	dyn   bool // a gem override no typed adapter fits: the slot boxes its arguments into a dynamic call (decision 249)
	raise bool // a gem override no adapter fits at all (another positional count, or keywords): the slot raises MRI's ArgumentError
}

// entry is a method as seen from a concrete class's method set.
type entry struct {
	M     *Method
	Owner *Class
	Env   map[string]Type // owner's type params → types in terms of the class's params
	Entry *Class          // class where the method entered the hierarchy (for Self)
}

func (c *Class) isSubclassOf(other *Class) bool {
	for k := c; k != nil; k = k.Super {
		if k == other || k.template != nil && k.template == other {
			return true
		}
		for _, inc := range k.Includes {
			if inc.Mod == other || inc.Mod.isSubclassOf(other) {
				return true
			}
		}
	}
	return false
}

// includedBelow reports whether a subclass of c includes mod.
func (c *Class) includedBelow(mod *Class) bool {
	for _, sub := range c.Subclasses {
		if sub.isSubclassOf(mod) || sub.includedBelow(mod) {
			return true
		}
	}
	return false
}

func (c *Class) isStruct() bool { return !c.IsModule && c.GoType == "" }

// mutable reports whether the @go_type is a reference-like Go type, in which
// case instances are handled as pointers.
func (c *Class) mutable() bool {
	g := c.GoType
	return strings.HasPrefix(g, "[]") || strings.HasPrefix(g, "map[") ||
		strings.HasPrefix(g, "struct") || strings.HasPrefix(g, "*")
}

func (c *Class) paramTypes() []Type {
	out := make([]Type, len(c.TypeParams))
	for i, p := range c.TypeParams {
		out[i] = TVar{Name: p}
	}
	return out
}

// instance is the type of an instance with the class's own params as vars.
func (c *Class) instance() TClass { return TClass{C: c, Args: c.paramTypes()} }

// methodSet returns the full method set in lookup order: own methods,
// included modules (last include first), then the superclass chain.
func (c *Class) methodSet() []entry {
	if c.msetCache != nil {
		return c.msetCache
	}
	seen := map[string]bool{}
	var out []entry
	add := func(e entry) {
		if seen[e.M.Name] {
			return
		}
		seen[e.M.Name] = true
		out = append(out, e)
	}
	identity := map[string]Type{}
	for _, p := range c.TypeParams {
		identity[p] = TVar{Name: p}
	}
	for _, m := range c.MethodList {
		add(entry{M: m, Owner: c, Env: identity, Entry: c})
	}
	var inSuper map[*Class]bool // a module the superclass already has stays where it is in the chain, as MRI's include skips it
	if c.Super != nil {
		inSuper = map[*Class]bool{}
		for _, a := range c.Super.ancestors() {
			inSuper[a] = true
		}
	}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		inc := c.Includes[i]
		env := map[string]Type{}
		for j, p := range inc.Mod.TypeParams {
			if j < len(inc.Args) {
				env[p] = inc.Args[j]
			}
		}
		for _, e := range inc.Mod.methodSet() {
			if inSuper[e.Owner] {
				continue // AbstractDownloadStrategy's own ohai, not Utils::Output::Mixin's again through Utils::Curl
			}
			e2 := entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: c}
			if c.IsModule {
				e2.Entry = e.Entry
			}
			add(e2)
		}
	}
	if c.Super != nil {
		for _, e := range c.Super.methodSet() {
			add(e)
		}
	}
	c.msetCache = out
	return out
}

func (c *Class) lookup(name string) *entry {
	set := c.methodSet()
	if c.msetIndex == nil {
		c.msetIndex = make(map[string]int, len(set))
		for i, e := range set {
			c.msetIndex[e.M.Name] = i
		}
	}
	if i, ok := c.msetIndex[name]; ok {
		e := set[i]
		for k := c; k != nil && k != e.Owner; k = k.Super {
			if k.undefs[name] {
				return nil
			}
		}
		return &e
	}
	return nil
}

// ancestors is c then what MRI's Module#ancestors lists after it, short of
// Object (whose constants are the top-level ones): included modules last
// first, each followed by its own, then the superclass's ancestors.
// Includes not yet resolved (during link) are skipped.
func (c *Class) ancestors() []*Class {
	out := []*Class{c}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		if m := c.Includes[i].Mod; m != nil {
			out = append(out, m.ancestors()...)
		}
	}
	if c.Super != nil && !c.Super.universal {
		out = append(out, c.Super.ancestors()...)
	}
	return out
}

// structChain is c and its superclasses up to (excluding) Object, nearest first; mixins are not included.
func (c *Class) structChain() []*Class {
	var out []*Class
	for k := c; k != nil && !k.universal; k = k.Super {
		out = append(out, k)
	}
	return out
}

func callArgs(n *parser.CallNode) []parser.Node {
	if n.Arguments == nil {
		return nil
	}
	return n.Arguments.Arguments
}

// instanceDelegator is a class-body hook that makes each instance method defined after it a class method forwarding
// to the class's `instance` (prime's Prime.each, Prime.prime?): at offset in file.
type instanceDelegator struct {
	target string
	file   *File
	offset int
	at     parser.Node
}

type singletonDef struct {
	node    *parser.DefNode
	private bool
	scope   []*Class
	file    *File
}

// singletonMisc is an attr_* call or an alias inside `class << self`.
type singletonMisc struct {
	node    parser.Node
	private bool
	scope   []*Class
	file    *File
}

// Decision 3's naming still merges capitals (`foo_bar`/`fooBar`) and digits after `_` (`utf_8`/`utf8`), so reject those here, not at go build.
func (c *Compiler) checkGoNames(where string, ms []*Method) {
	seen := map[string]*Method{}
	for _, m := range ms {
		if p := seen[m.GoName]; p != nil && p.Name != m.Name {
			c.errorAt(m.srcPos, "`%s` and `%s` (%s) both become Go %s in %s; rename one", m.Name, p.Name, p.srcPos, m.GoName, where)
		}
		seen[m.GoName] = m
	}
}

// defsOf lists every definition of name in c's lookup order (methodSet
// without the shadowing): a module method's `super` target follows it.
func (c *Class) defsOf(name string) []entry {
	var out []entry
	if m := c.Methods[name]; m != nil {
		identity := map[string]Type{}
		for _, p := range c.TypeParams {
			identity[p] = TVar{Name: p}
		}
		out = append(out, entry{M: m, Owner: c, Env: identity, Entry: c})
	}
	for i := len(c.Includes) - 1; i >= 0; i-- {
		inc := c.Includes[i]
		env := map[string]Type{}
		for j, p := range inc.Mod.TypeParams {
			if j < len(inc.Args) {
				env[p] = inc.Args[j]
			}
		}
		for _, e := range inc.Mod.defsOf(name) {
			e2 := entry{M: e.M, Owner: e.Owner, Env: composeEnv(e.Env, env), Entry: c}
			if c.IsModule {
				e2.Entry = e.Entry
			}
			out = append(out, e2)
		}
	}
	if c.Super != nil {
		out = append(out, c.Super.defsOf(name)...)
	}
	return out
}

// allAncestors: superclasses and included modules above c, transitively,
// with repeats, universal ones included (unlike ancestors).
func (c *Class) allAncestors() []*Class {
	var out []*Class
	for _, inc := range c.Includes {
		if inc.Mod != nil {
			out = append(append(out, inc.Mod), inc.Mod.allAncestors()...)
		}
	}
	if c.Super != nil {
		out = append(append(out, c.Super), c.Super.allAncestors()...)
	}
	return out
}

// hasKeywords reports keyword parameters (`a:` or `**opts`).
func (m *Method) hasKeywords() bool {
	return slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword })
}

// optionalBlockLocal names the local that holds an optional block (`?{ }`)
// in a def: its &block name, or a hidden one for yield and block_given?.
// The local is a Proc?, so calling it unchecked is a compile error and
// `if block_given?` / `blk&.call` narrow it like any optional value.
func (m *Method) optionalBlockLocal() string {
	if m == nil || m.Block == nil || !m.Block.Optional || m.Kind != kindDef || m.Iterator {
		return ""
	}
	if m.BlockParam != "" {
		return m.BlockParam
	}
	return "__block"
}

// postCount is how many post params m has.
func (m *Method) postCount() int {
	n := 0
	for _, p := range m.Params {
		if p.Post {
			n++
		}
	}
	return n
}

// positionalCount is how many positional arguments m takes, *rest aside.
func (m *Method) positionalCount() int {
	n := 0
	for _, p := range m.Params {
		if !p.Keyword && !p.Rest {
			n++
		}
	}
	return n
}

func requiredArgs(m *Method) int {
	n := 0
	for _, p := range m.Params {
		if !p.Rest && !p.Keyword && p.Default == nil {
			n++
		}
	}
	return n
}

// kwMask reports a Go `rbKw int` parameter: which optional keywords the
// caller passed, for defaults run in the callee (calleeDefaults).
func (m *Method) kwMask() bool {
	return m.calleeDefaults && slices.ContainsFunc(m.Params, func(p Param) bool { return p.Keyword && !p.KwRest && p.Default != nil })
}

func (m *Method) String() string {
	owner := "main"
	switch {
	case m.Owner != nil && m.Owner.metaOf != nil:
		return m.Owner.RubyName + "." + m.Name
	case m.Owner != nil:
		owner = m.Owner.Name
	}
	return fmt.Sprintf("%s#%s", owner, m.Name)
}

// containerEnv adds the embedded container's fixed type args to env: a Hash subclass's keys are untyped (decision 165).
func (cls *Class) containerEnv(env map[string]Type) {
	if !cls.hashBase || cls.Super == nil {
		return
	}
	base := cls.Super
	for base.hashBase && base.Super != nil { // Concurrent::Set < CRubySet < ::Set: the container itself
		base = base.Super
	}
	if base.RubyName == "Hash" {
		env["K"] = TAny{}
		return
	}
	for _, p := range base.TypeParams { // an Array (Set, Queue) subclass's elements are untyped
		env[p] = TAny{}
	}
}

// containerTargs are the Go type args of the embedded container's free funcs for a subclass instantiated with args.
func (c *Compiler) containerTargs(cls *Class, args []Type) string {
	switch {
	case cls.Super.RubyName != "Hash":
		return anyArgs(len(cls.Super.TypeParams))
	case len(args) == 0:
		return ""
	}
	return "[any, " + c.goTypes(args) + "]"
}

// anyArgs is `[any, ...]` for n type params, or "".
func anyArgs(n int) string {
	if n == 0 {
		return ""
	}
	return "[" + strings.Repeat("any, ", n-1) + "any]"
}

// embeddedSuper is how code reaches a @go_type subclass's embedded base: its address for a mutable base (Hash,
// Array), the value itself for a value type (Pathname).
func embeddedSuper(cls *Class, recv string) string {
	if cls.Super.mutable() {
		return "(&" + recv + "." + superField(cls.Super) + ")"
	}
	return recv + "." + superField(cls.Super)
}

// leaf is a user or gem class no class inherits from: its objects are exactly *C, so values typed as it are *C rather
// than its interface, and its methods take self as *C, direct field access the Go compiler inlines (decision 359). A
// module's singleton class is one too: nothing can subclass it (decision 398). A class's is not: its subclasses'
// metaclasses inherit its class methods, and that hierarchy lives on the class (metaOf.Subclasses), not the metaclass.
func (c *Class) leaf() bool {
	return c.isStruct() && !c.universal && len(c.Subclasses) == 0 && len(c.TypeParams) == 0 && (c.metaOf == nil || c.metaOf.IsModule) && !c.hashBase &&
		!c.isPerRun && !c.isTemplate() && c.template == nil && c.File != nil && !c.File.isPrelude() && !mixesUserModule(c)
}

// mixesUserModule reports a user or gem module among c's ancestors: its constraint may want `m() Self` of its
// includers, which a leaf's *C methods do not declare (ponytail: prelude modules only; check the constraint's
// signatures to let these in).
func mixesUserModule(c *Class) bool {
	for k := c; k != nil && !k.universal; k = k.Super {
		for _, inc := range k.Includes {
			if inc.Mod != nil && (inc.Mod.File == nil || !inc.Mod.File.isPrelude() || mixesUserModule(inc.Mod)) {
				return true
			}
		}
	}
	return false
}

// objType is the Go type of a value of struct class c: *C for a leaf, else its interface.
func (c *Class) objType() string {
	if c.leaf() {
		return "*" + c.Name
	}
	return c.ifaceName()
}
