# rbs_inline: enabled

text = "the cat and the hat and the bat"
counts = text.split.tally
counts.sort_by { |w, n| [-n, w] }.first(3).each { |w, n| puts "#{w}: #{n}" }
