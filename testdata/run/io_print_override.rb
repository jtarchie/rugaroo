# A program's own print method, called bare: decision 446's $_ is Kernel#print's only (#147 follow-up)
def print = puts("top print")

class PrintOverrideBoard
  def print = puts("board print")
  def show = print
end

print
PrintOverrideBoard.new.show
