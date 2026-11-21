# Serves a Snails app with Tep::APP on WEBrick; without PORT it only loads, which is what rugaroo's build-time boot run sees.
require_relative "shim/tep_net"
$LOADED_FEATURES << File.expand_path("snails-core/tep/net.rb", __dir__)
require_relative "shim/tep_webrick"
require_relative "snails-core/boot"

port = ENV.fetch("PORT", "")
if port.empty?
  puts "loaded; set PORT to serve"
else
  Tep::WEBrickServer.start_app(port.to_i)
end
