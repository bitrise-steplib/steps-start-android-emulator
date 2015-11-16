require 'securerandom'
require 'timeout'
require 'net/telnet'

@adb = File.join(ENV['HOME'], 'Library/Developer/Xamarin/android-sdk-macosx/platform-tools/adb')

# -----------------------
# --- functions
# -----------------------

def avd_name_match?(avd_name, port)
  telnet = Net::Telnet.new('Host' => 'localhost',
                           'Port' => port,
                           'Timeout' => 15,
                           'Binmode' => true)

  match = false

  telnet.puts('avd name')
  telnet.waitfor('Match' => /OK/) { |c| match = true if c.include? avd_name }
  if match
    telnet.close
    return true
  end

  telnet.puts('avd name')
  telnet.waitfor('Match' => /OK/) { |c| match = true if c.include? avd_name }
  if match
    telnet.close
    return true
  end

  telnet.puts('avd name')
  telnet.waitfor('Match' => /OK/) { |c| match = true if c.include? avd_name }
  telnet.close

  return match
end

def avd_image_serial(avd_name)
  devices = `#{@adb} devices -l`.split("\n")

  return nil unless devices

  devices.each do |device|
    serial = device.match(/^emulator-(?<port>\d*)/)
    next unless serial

    port = serial.captures[0]
    match = avd_name_match?(avd_name, port)
    return serial if match
  end

  return nil
end

def start_emulator(avd_name, uuid)
  emulator = File.join(ENV['HOME'], 'Library/Developer/Xamarin/android-sdk-macosx/tools/emulator')
  pid = spawn("#{emulator} -avd #{avd_name} -no-skin -noaudio -no-window -prop emu.uuid=#{uuid}", [:out, :err] => ['emulator.log', 'w'])
  Process.detach(pid)
end

def emulator_serial!(uuid)
  Timeout.timeout(120) do
    loop do
      sleep 5
      devices = `#{@adb} devices -l`.split("\n")

      devices.each do |device|
        match = device.match(/^(?<emulator>emulator-\d*)/)
        next unless match

        emu_udid_out = `#{@adb} -s #{match[0]} shell getprop emu.uuid`
        return match[0] if emu_udid_out.strip.eql? uuid
      end
    end
  end
  puts "Getting emulator's name timed out"
  exit 1
end

def ensure_emulator_booted!(serial)
  Timeout.timeout(120) do
    loop do
      sleep 5

      dev_boot_complete_out = `#{@adb} -s #{serial} shell "getprop dev.bootcomplete"`.strip
      sys_boot_complete_out = `#{@adb} -s #{serial} shell "getprop sys.boot_completed"`.strip
      boot_anim_out = `#{@adb} -s #{serial} shell "getprop init.svc.bootanim"`.strip
      puts "booted: #{dev_boot_complete_out} | booted: #{sys_boot_complete_out} | boot_anim: #{boot_anim_out}"

      return if dev_boot_complete_out.eql?('1') && sys_boot_complete_out.eql?('1') && boot_anim_out.eql?('stopped')
    end
  end
  puts 'Emulator timed out while booting'
  exit 1
end

# -----------------------
# --- main
# -----------------------

emulator_uuid = SecureRandom.uuid
emulator_name = ENV['emulator_name']

puts
puts '=> Check if emulator already running'
emulator_serial = avd_image_serial(emulator_name)
unless emulator_serial
  puts
  puts '=> Emulator not running, starting it...'
  start_emulator(emulator_name, emulator_uuid)

  puts
  puts '=> Get emulator serial'
  emulator_serial = emulator_serial!(emulator_uuid)
end

puts
puts '=> Ensure device is booted'
ensure_emulator_booted!(emulator_serial)

puts
puts "(i) Emulator running wit serial: #{emulator_serial}"

`#{@adb} -s #{emulator_serial} shell input keyevent 82 &`

`envman add --key BITRISE_EMULATOR_SERIAL --value #{emulator_serial}`

puts
puts "\e[32mEmulator is ready to use 🚀\e[0m"
