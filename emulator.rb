require 'timeout'

# -----------------------
# --- Constants
# -----------------------

@adb = File.join(ENV['android_home'], 'platform-tools/adb')
puts "(i) adb: #{@adb}"

# -----------------------
# --- Functions
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

def emulator_list
  command = "#{@adb} devices 2>&1"
  output = %x(#{command}).strip

  return [] unless output

  output_split = output.split("\n")
  return [] unless output_split

  devices = []
  output_split.each do |device|
    match = device.match(/^(?<emulator>emulator-\d*)/)
    next if !match || !match.captures || match.captures.length != 1

    devices << match.captures[0]
  end
  devices
end

def start_emulator(avd_name, skin, timeout)
  running_devices = emulator_list

  if running_devices.length > 0
    puts 'Running emulators:'
    running_devices.each do |device|
      puts " * #{device}"
    end
  else
    puts
    puts '(i) No running emulator found'
  end


  os = %x(uname -s 2>&1)
  puts
  puts "os: #{os}"

  emulator = File.join(ENV['android_home'], 'tools/emulator')
  emulator = File.join(ENV['android_home'], 'tools/emulator64-arm') if os.include? 'Linux'

  params = [emulator, '-avd', avd_name]
  params << '-no-boot-anim' # Disable the boot animation during emulator startup.
  params << '-noaudio' # Disable audio support in the current emulator instance.
  params << '-no-window' # Disable the emulator's graphical window display.

  params << "-skin #{skin}" unless skin.to_s == ''
  params << '-noskin' if skin.to_s == ''

  command = params.join(' ')

  puts "#{command}"
  execute_with_timeout!(command, timeout)

  devices = emulator_list

  puts
  if devices.length > 0
    puts 'Running emulators:'
    devices.each do |device|
      puts " * #{device}"
    end
  else
    puts
    puts '(i) No running emulator found'
  end

  started_emulator = ''
  if running_devices.length == 0
    started_emulator = devices[0] if devices.length == 1
  else
    if devices.length - running_devices.length != 1
      raise "Running devices: #{running_devices.length} - after start #{devices.length}"
    end

    devices.each do |device|
      next if running_devices.include? device

      started_emulator = device
      break
    end
  end

  started_emulator
end

def execute_with_timeout!(command, timeout)
  begin
    pipe = IO.popen(command, 'r')
  rescue Exception => e
    raise "Execution of command #{command} unsuccessful, error: #{e}"
  end

  begin
    Timeout::timeout(timeout) {
      loop do
        output = pipe.gets
        if output.to_s != ''
          puts "#{output}"

          raise Timeout::Error.new if output.include? 'emulator: UpdateChecker'
        end

        sleep 5
      end
    }
  rescue Timeout::Error
    Process.detach(pipe.pid)
    return
  end
end

def ensure_emulator_booted!(serial, timeout)
  device_started = false

  Timeout.timeout(timeout) do
    loop do
      sleep 10

      unless device_started
        devices = %x(#{@adb} devices 2>&1).strip
        next unless devices

        devices = devices.split("\n")
        next unless devices

        devices.each do |device|
          match = device.match("^#{serial}\\s(?<state>.*)")
          next if !match || !match.captures || match.captures.length != 1

          state = match.captures[0].strip

          if state != 'device'
            puts "#{serial} state: #{state}"
            break
          end

          device_started = true
          break
        end

        unless device_started
          next
        end
      end

      dev_boot = "#{@adb} -s #{serial} shell \"getprop dev.bootcomplete\""
      dev_boot_complete_out = `#{dev_boot}`.strip

      sys_boot = "#{@adb} -s #{serial} shell \"getprop sys.boot_completed\""
      sys_boot_complete_out = `#{sys_boot}`.strip

      boot_anim = "#{@adb} -s #{serial} shell \"getprop init.svc.bootanim\""
      boot_anim_out = `#{boot_anim}`.strip

      puts "booted: #{dev_boot_complete_out} | booted: #{sys_boot_complete_out} | boot_anim: #{boot_anim_out}"

      return if dev_boot_complete_out.eql?('1') && sys_boot_complete_out.eql?('1') && boot_anim_out.eql?('stopped')
    end
  end
  puts 'Emulator timed out while booting'
end

# -----------------------
# --- Main
# -----------------------

emulator_name = ENV['emulator_name']
emulator_skin = ENV['emulator_skin']

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
puts "=> Starting emulator (#{emulator_name}) ..."
emulator_serial = start_emulator(emulator_name, emulator_skin, 120)
raise 'no serial' if emulator_serial.to_s == ''
puts
puts "(i) emulator started with serial: #{emulator_serial}"

puts
puts '=> Ensure device is booted'
ensure_emulator_booted!(emulator_serial, 600)

puts
puts "(i) Emulator running wit serial: #{emulator_serial}"
`#{@adb} -s #{emulator_serial} shell input keyevent 82 &`
`envman add --key BITRISE_EMULATOR_SERIAL --value #{emulator_serial}`

puts
puts "\e[32mEmulator is ready to use 🚀\e[0m"
