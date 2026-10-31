# frozen_string_literal: true

# A class body with locals runs as one unit (decision 170); a constant assigned later in the body without reading a
# local is still assigned in order, before the unit's statements that read it (racc's generated parsers:
# `Racc_token_to_s_table = [...]; Ractor.make_shareable(Racc_token_to_s_table)`).
class Unitish
  table = [1, 2]
  ARG = [table, 3]
  NAMES = %w[a b c]
  Ractor.make_shareable(NAMES) if defined?(Ractor)
  COUNT = NAMES.size
end
