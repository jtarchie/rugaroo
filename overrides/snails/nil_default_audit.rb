# Finds nil-defaulted params whose sidecar .rbs type rejects nil: Spinel accepts that, rugaroo does not. Args: dirs.
require "prism"
require "rbs"

def nilable?(type)
  case type
  when RBS::Types::Optional, RBS::Types::Bases::Nil, RBS::Types::Bases::Any, RBS::Types::Bases::Void, RBS::Types::Bases::Top then true
  when RBS::Types::Union then type.types.any? { nilable?(_1) }
  else false
  end
end

# [owner, singleton?, name] => MethodType, from every .rbs under the dirs.
def sigs(dirs)
  out = {}
  walk = lambda do |decl, ns|
    case decl
    when RBS::AST::Declarations::Module, RBS::AST::Declarations::Class
      owner = (ns + [decl.name.to_s]).join("::")
      decl.members.each do |m|
        case m
        when RBS::AST::Members::MethodDefinition
          out[[owner, m.singleton?, m.name.to_s]] = m.overloads.map(&:method_type)
        when RBS::AST::Declarations::Module, RBS::AST::Declarations::Class
          walk.call(m, ns + [decl.name.to_s])
        end
      end
    end
  end
  dirs.flat_map { Dir["#{_1}/**/*.rbs"] }.each do |path|
    _, _, decls = RBS::Parser.parse_signature(File.read(path))
    decls.each { walk.call(_1, []) }
  rescue RBS::ParsingError => e
    warn "skip #{path}: #{e.message}"
  end
  out
end

class Finder < Prism::Visitor
  attr_reader :hits

  def initialize(path)
    super()
    @path = path
    @ns = []
    @hits = []
  end

  def visit_module_node(node) = scoped(node) { super }
  def visit_class_node(node) = scoped(node) { super }

  def visit_def_node(node)
    params = node.parameters
    if params
      opt = params.optionals.select { _1.value.is_a?(Prism::NilNode) }.map { [:pos, _1.name.to_s, params.requireds.size + params.optionals.index(_1)] }
      kw = params.keywords.select { _1.is_a?(Prism::OptionalKeywordParameterNode) && _1.value.is_a?(Prism::NilNode) }.map { [:kw, _1.name.to_s] }
      (opt + kw).each { @hits << [@ns.join("::"), node.receiver.is_a?(Prism::SelfNode), node.name.to_s, _1, "#{@path}:#{node.location.start_line}"] }
    end
    super
  end

  private

  def scoped(node)
    @ns.push(node.constant_path.slice)
    yield
    @ns.pop
  end
end

dirs = ARGV
table = sigs(dirs)
dirs.flat_map { Dir["#{_1}/**/*.rb"] }.each do |path|
  f = Finder.new(path)
  Prism.parse_file(path).value.accept(f)
  f.hits.each do |owner, single, name, (kind, pname, idx), at|
    types = table[[owner, single, name]] or next
    types.each do |mt|
      fn = mt.type
      next unless fn.respond_to?(:optional_keywords)

      t = kind == :kw ? (fn.optional_keywords[pname.to_sym] || fn.required_keywords[pname.to_sym])&.type : (fn.required_positionals + fn.optional_positionals)[idx]&.type
      next if t.nil? || nilable?(t)

      puts "#{at}: #{owner}#{single ? "." : "#"}#{name} #{kind == :kw ? "#{pname}:" : pname} defaults to nil, sig says #{t}"
    end
  end
end
