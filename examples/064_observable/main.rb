# rbs_inline: enabled

require "observer"

class WeatherStation
  include Observable

  #: (Integer) -> void
  def temperature=(value)
    changed
    notify_observers(value)
  end
end

class Display
  #: (String) -> void
  def initialize(name)
    @name = name
  end

  #: (Integer) -> void
  def update(temp)
    puts "#{@name}: #{temp}F"
  end
end

class Recorder
  #: () -> void
  def initialize
    @count = 0
  end

  #: (Integer) -> void
  def record(temp)
    @count += 1
    puts "recorder: reading ##{@count} is #{temp}F"
  end
end

station = WeatherStation.new
porch = Display.new("porch")
kitchen = Display.new("kitchen")
recorder = Recorder.new

station.add_observer(porch)
station.add_observer(kitchen)
station.add_observer(recorder, :record)
puts "observers: #{station.count_observers}"

station.temperature = 72

station.delete_observer(kitchen)
puts "observers: #{station.count_observers}"

station.temperature = 68

station.changed
puts "changed?: #{station.changed?}"
station.changed(false)
puts "changed?: #{station.changed?}"
station.notify_observers(999) # no-op: changed? is false

station.delete_observers
puts "observers: #{station.count_observers}"
station.notify_observers(1) # no-op: no observers, changed? still false
