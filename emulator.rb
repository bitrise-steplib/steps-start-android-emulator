require 'securerandom'
require 'timeout'
require 'net/telnet'

@adb = File.join(ENV['android_home'], 'platform-tools/adb')
puts "(i) adb: #{@adb}"

# -----------------------
# --- functions
# -----------------------

def list_of_avd_images
  user_home_dir = ENV['HOME']
  return nil unless user_home_dir

  avd_path = File.join(user_home_dir, '.android', 'avd')
  return nil unless File.exist? avd_path

  images_paths = Dir[File.join(avd_path, '*.ini')]

  images_names = []
  images_paths.each do |image_path|
    ext = File.extname(image_path)
    file_name = File.basename(image_path, ext)
    images_names << file_name
  end
  return nil unless images_names

  images_names
end

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
  devices = %x(#{@adb} devices 2>&1)
  puts "devices: #{devices}"
  return nil unless devices

  devices = devices.split("\n")
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
  os = %x(uname -s 2>&1)
  puts "os: #{os}"

  emulator = File.join(ENV['android_home'], 'tools/emulator')

  cmd = "#{emulator} -avd #{avd_name} -no-boot-anim -no-skin -noaudio -no-window -prop emu.uuid=#{uuid}"
  cmd += ' -force-32bit' if os.include? 'Linux'

  puts "#{cmd}"
  pid = spawn(cmd, [:out, :err] => ['emulator.log', 'w'])
  Process.detach(pid)
end

def emulator_serial!(uuid)
  Timeout.timeout(600) do
    loop do
      sleep 5
      devices = %x(#{@adb} devices 2>&1).strip
      puts
      puts "devices_out: #{devices}"
      next unless devices

      devices = devices.split("\n")
      next unless devices

      devices.each do |device|
        match = device.match(/^(?<emulator>emulator-\d*)/)
        next unless match

        emu_udid_out = %x(#{@adb} -s #{match[0]} shell getprop emu.uuid 2>&1)
        puts "emu_udid_out: #{emu_udid_out}"
        return match[0] if emu_udid_out.strip.eql? uuid
      end
    end
  end
  puts "Getting emulator's name timed out"
  exit 1
end

def ensure_emulator_booted!(serial)
  Timeout.timeout(600) do
    loop do
      sleep 10

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

avd_images = list_of_avd_images
if avd_images
  unless avd_images.include? emulator_name
    puts
    puts "(!) AVD image with name (#{emulator_name}) not found!"
    puts "Available AVD images: #{avd_images}"
    exit 1
  end
end

puts
puts '=> Restart adb'
puts "#{@adb} kill-server"
system("#{@adb} kill-server", out: $stdout, err: $stderr)

puts "#{@adb} start-server"
system("#{@adb} start-server", out: $stdout, err: $stderr)

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
