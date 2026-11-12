module Slotish
  module Align
    def check_alignment(items)
      items.map { |i| register_offense(i, i * 2) }
    end

    def width = 2

    def register_offense(node, message_node)
      "#{node}/#{message_node}/#{width}"
    end
  end

  module Help
    include Align

    def tab = width * 2
  end

  class Line
    include Help

    def run(line)
      register_offense(1, line, 0)
    end

    def register_offense(loc, line, index, length: line.size)
      "#{loc}:#{line}:#{index}:#{length}:#{tab}"
    end
  end

  class Other
    include Help

    def run = check_alignment([1, 2])
  end
end
