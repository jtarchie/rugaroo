a = ["x"]
s = a[-1]
a[-1] = "y"
p s

m = %w[p q]
x1, x2 = m
m[0] = "z"
p x1, x2
