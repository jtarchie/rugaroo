module Demo
  # One file under two autoloads, as prism's visitor.rb (BasicVisitor, Visitor): naming either keeps it (decision 427).
  class BothA
    def tag = :a
  end

  class BothB < BothA
    def tag = :b
  end
end
