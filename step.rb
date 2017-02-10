require 'timeout'

# -----------------------
# --- Constants
# -----------------------

@adb = File.join(ENV['android_home'], 'platform-tools/adb')

# -----------------------
# --- Functions
# -----------------------

def log_fail(message)
  puts
  puts "\e[31m#{message}\e[0m"
  exit(1)
end

def log_warn(message)
  puts "\e[33m#{message}\e[0m"
end

def log_info(message)
  puts
  puts "\e[34m#{message}\e[0m"
end

def log_details(message)
  puts "  #{message}"
end

def log_done(message)
  puts "  \e[32m#{message}\e[0m"
end

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
  devices = {}

  output = `#{@adb} devices 2>&1`.strip
  return {} unless output

  output_split = output.split("\n")
  return {} unless output_split

  output_split.each do |device|
    regex = /^(?<emulator>emulator-\d*)\s(?<state>.*)/
    match = device.match(regex)
    next unless match

    serial = match.captures[0]
    state = match.captures[1]

    devices[serial] = state
  end

  devices
end

def find_started_serial(running_devices)
  started_emulator = nil
  devices = emulator_list
  serials = devices.keys - running_devices.keys

  if serials.length == 1
    started_serial = serials[0]
    started_state = devices[serials[0]]

    if started_serial.to_s != '' && started_state.to_s != ''
      started_emulator = { started_serial => started_state }
    end
  end

  unless started_emulator.nil?
    started_emulator.each do |serial, state|
      return serial if state == 'device'
    end
  end

  nil
end

# -----------------------
# --- Main
# -----------------------

#
# Input validation
emulator_name = ENV['emulator_name']
emulator_skin = ENV['skin']
emulator_options = ENV['emulator_options']
other_options = ENV['other_options']
wait_for_boot = ENV['wait_for_boot']

log_info('Configs:')
log_details("emulator_name: #{emulator_name}")
log_details("emulator_skin: #{emulator_skin}")
log_details("emulator_options: #{emulator_options}")
log_details("wait_for_boot: #{wait_for_boot}")
log_details("[deprecated!] other_options: #{other_options}")

log_fail('Missing required input: emulator_name') if emulator_name.to_s == ''

unless other_options.to_s.empty?
  puts
  log_warn('other_options input is deprecated!')
  log_warn('Use emulator_options input to control all of emulator command\'s flags')

  options = []
  options << emulator_options unless emulator_options.to_s.empty?
  options << other_options unless other_options.to_s.empty?

  emulator_options = options.join(' ')
end

avd_images = list_of_avd_images
if avd_images
  unless avd_images.include? emulator_name
    log_info "Available AVD images: #{avd_images}"
    log_fail "AVD image with name (#{emulator_name}) not found!"
  end
end

#
# Print running devices
running_devices = emulator_list
unless running_devices.empty?
  log_info('Running emulators:')
  running_devices.each do |device, _|
    log_details("* #{device}")
  end
end

#
# Start adb-server
`#{@adb} start-server`

begin
  Timeout.timeout(800) do
    #
    # Start AVD image
    os = `uname -s 2>&1`

    emulator = File.join(ENV['android_home'], 'tools/emulator')
    emulator = File.join(ENV['android_home'], 'tools/emulator64-arm') if os.include? 'Linux'

    params = [emulator, '-avd', emulator_name]
    params << "-skin #{emulator_skin}" unless emulator_skin.to_s.empty?
    params << '-noskin' if emulator_skin.to_s.empty?

    params << emulator_options unless emulator_options.to_s.empty?

    command = params.join(' ')

    log_info('Starting emulator')
    log_details(command)

    Thread.new do
      system(command)
    end

    #
    # Check for started emulator serial
    serial = nil
    looking_for_serial = true

    while looking_for_serial
      sleep 5

      serial = find_started_serial(running_devices)
      looking_for_serial = false if serial.to_s != ''
    end

    log_done("Emulator started: (#{serial})")

    #
    # Wait for boot finish
    if wait_for_boot != "false"

      log_info('Waiting for emulator boot')

      boot_in_progress = true

      while boot_in_progress
        sleep 5

        dev_boot = "#{@adb} -s #{serial} shell \"getprop dev.bootcomplete\""
        dev_boot_complete_out = `#{dev_boot}`.strip

        sys_boot = "#{@adb} -s #{serial} shell \"getprop sys.boot_completed\""
        sys_boot_complete_out = `#{sys_boot}`.strip

        boot_anim = "#{@adb} -s #{serial} shell \"getprop init.svc.bootanim\""
        boot_anim_out = `#{boot_anim}`.strip

        boot_in_progress = false if dev_boot_complete_out.eql?('1') && sys_boot_complete_out.eql?('1') && boot_anim_out.eql?('stopped')
      end

      `#{@adb} -s #{serial} shell input keyevent 82 &`
      `#{@adb} -s #{serial} shell input keyevent 1 &`

      log_done('Emulator is ready to use 🚀')
    end

    `envman add --key BITRISE_EMULATOR_SERIAL --value #{serial}`

    exit(0)
  end
rescue Timeout::Error
  log_fail('Starting emulator timed out')
end
