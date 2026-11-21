# kramdown's update_tree (decision 482): `children.map! { ... }.flatten!` leaves each child, its children or nothing,
# and the Array keeps its typed elements.
class Treeish
  attr_accessor :kind
  attr_accessor :children #: Array[Treeish]

  def initialize(kind)
    @kind = kind
    @children = [] #: Array[Treeish]
  end

  def flatten_groups
    spliced = @children.map! do |c|
      case c.kind
      when :group then c.children
      when :drop then []
      else c
      end
    end.flatten!
    [spliced.nil?, @children.map(&:kind)]
  end
end
