require 'securerandom'
require 'timeout'

emulator_uuid=SecureRandom.uuid
emulator_name = nil
emulator_booted = false

# Start the emulator
puts "start emulator with uuid #{emulator_uuid}"
pid = spawn("emulator -avd #{ENV['emulator_name']} -no-skin -noaudio -no-window -prop emu.uuid=#{emulator_uuid}", [:out, :err]=>["emulator.log", "w"])
Process.detach(pid)

puts ""
puts "Looking for emulator"
# Get emulator name
begin
	Timeout::timeout(120) do
		while emulator_name.nil?
			sleep 5
			devices = `adb devices -l`.split("\n")

			devices.each do |device|
				if match = device.match(/^(?<emulator>emulator-\d*)/)
					emulator_name = match[0] if `adb -s #{match[0]} shell getprop emu.uuid`.strip.eql? emulator_uuid
				end
			end
		end
	end
rescue => ex
	puts "Getting emulator's name timed out"
	exit 1
end
puts "Emulator found with name: #{emulator_name}"

puts ""
puts "Waiting for emulator to boot"
# Wait till device is booted
begin
	Timeout::timeout(120) do
		while emulator_booted == false
			sleep 1
			if `adb -s #{emulator_name} shell "getprop dev.bootcomplete"`.strip.eql?("1") &&
				`adb -s #{emulator_name} shell "getprop sys.boot_completed"`.strip.eql?("1") &&
				`adb -s #{emulator_name} shell "getprop init.svc.bootanim"`.strip.eql?("stopped")
				emulator_booted = true
			end
		end
	end
rescue
	puts "Emulator timed out while booting"
	exit 1
end

`adb -s #{emulator_name} shell input keyevent 82 &`
puts ""
puts "\e[32mEmulator is ready to use 🚀\e[0m"