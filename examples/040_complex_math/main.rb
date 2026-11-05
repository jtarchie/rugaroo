# rbs_inline: enabled

# Roots of a quadratic, real or complex: Complex parts keep their class.
#: (Float, Float, Float) -> Array[Complex]
def roots(a, b, c)
  disc = b * b - 4 * a * c
  if disc >= 0
    s = Math.sqrt(disc)
    return [Complex((-b + s) / (2 * a)), Complex((-b - s) / (2 * a))]
  end
  s = Math.sqrt(-disc)
  [Complex(-b / (2 * a), s / (2 * a)), Complex(-b / (2 * a), -s / (2 * a))]
end

puts roots(1.0, -3.0, 2.0).inspect
puts roots(1.0, 2.0, 5.0).inspect

z = 3 + 4i
w = Complex(1, -2)
puts z, w.inspect, z.abs, z.abs2, z.conjugate
puts z + w, z - w, z * w, (z / w).inspect, (z**2).inspect, (w**-1).inspect
puts (z * 2).inspect, (2.5 * w).inspect, (z / 2).inspect, (z + Rational(1, 2)).inspect
puts z.real, z.imaginary, z.rectangular.inspect, z == Complex(3, 4), Complex(5, 0) == 5
puts Complex.polar(2, Math::PI).inspect, Complex(0, 1).arg == Math::PI / 2
puts (1i**2).inspect, Complex(1.5, 0).to_f

begin
  z.to_i
rescue RangeError => e
  puts "error: #{e.message}"
end

puts Math.sqrt(2), Math.cbrt(27), Math.hypot(5, 12), Math.log(Math::E), Math.log(1024, 2), Math.log10(0.001)
puts Math.sin(Math::PI).round(10), Math.cos(Math::PI / 2).round(10), Math.atan2(1, 1) * 4 == Math::PI, Math.exp(1) == Math::E
puts Float::INFINITY > 1e308, (Float::NAN == Float::NAN), Float::EPSILON

begin
  Math.sqrt(-1)
rescue Math::DomainError => e
  puts "error: #{e.message}"
end
