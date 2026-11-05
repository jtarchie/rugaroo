# rbs_inline: enabled

require "abbrev"

p Abbrev.abbrev(%w[ruby rules])
p Abbrev.abbrev(%w[car cars care])
p %w[summer winter].abbrev
p Abbrev.abbrev(%w[car box cone crab], "ca")
p Abbrev.abbrev(%w[car box cone crab], /b/)
p Abbrev.abbrev([])
p Abbrev.abbrev(%w[x])
