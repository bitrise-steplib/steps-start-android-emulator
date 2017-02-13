package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/sliceutil"
	"github.com/bitrise-steplib/steps-start-android-emulator/tools"
	"github.com/kballard/go-shellquote"
)

// ConfigsModel ...
type ConfigsModel struct {
	EmulatorName    string
	Skin            string
	EmulatorOptions string
	AndroidHome     string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		EmulatorName:    os.Getenv("emulator_name"),
		Skin:            os.Getenv("skin"),
		EmulatorOptions: os.Getenv("emulator_options"),
		AndroidHome:     os.Getenv("android_home"),
	}
}

func (configs ConfigsModel) print() {
	log.Infof("Configs:")
	log.Printf("- EmulatorName: %s", configs.EmulatorName)
	log.Printf("- Skin: %s", configs.Skin)
	log.Printf("- EmulatorOptions: %s", configs.EmulatorOptions)
	log.Printf("- AndroidHome: %s", configs.AndroidHome)
}

func (configs ConfigsModel) validate() error {
	if configs.EmulatorName == "" {
		return errors.New("no EmulatorName parameter specified")
	}
	if configs.AndroidHome == "" {
		return errors.New("no AndroidHome parameter specified")
	}
	if exist, err := pathutil.IsPathExists(configs.AndroidHome); err != nil {
		return fmt.Errorf("failed to check if android home exist, error: %s", err)
	} else if !exist {
		return fmt.Errorf("android home not exist at: %s", configs.AndroidHome)
	}

	return nil
}

func listAVDImages() ([]string, error) {
	homeDir := pathutil.UserHomeDir()
	avdDir := filepath.Join(homeDir, ".android", "avd")

	avdImagePattern := filepath.Join(avdDir, "*.ini")
	avdImages, err := filepath.Glob(avdImagePattern)
	if err != nil {
		return []string{}, fmt.Errorf("glob failed with pattern (%s), error: %s", avdImagePattern, err)
	}

	avdImageNames := []string{}

	for _, avdImage := range avdImages {
		imageName := filepath.Base(avdImage)
		imageName = strings.TrimSuffix(imageName, filepath.Ext(avdImage))
		avdImageNames = append(avdImageNames, imageName)
	}

	return avdImageNames, nil
}

func avdImageDir(name string) string {
	homeDir := pathutil.UserHomeDir()
	return filepath.Join(homeDir, ".android", "avd", name+".avd")
}

func currentlyStartedDeviceSerial(alreadyRunningDeviceInfos, currentlyRunningDeviceInfos map[string]string) string {
	startedSerial := ""

	for serial := range currentlyRunningDeviceInfos {
		_, found := alreadyRunningDeviceInfos[serial]
		if !found {
			startedSerial = serial
			break
		}
	}

	if len(startedSerial) > 0 {
		state := currentlyRunningDeviceInfos[startedSerial]
		if state == "device" {
			return startedSerial
		}
	}

	return ""
}

func runningDeviceInfos(adb tools.ADBModel) (map[string]string, error) {
	cmd := adb.DevicesCmd()
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return map[string]string{}, fmt.Errorf("command failed, error: %s", err)
	}

	// List of devices attached
	// emulator-5554	device
	deviceListItemPattern := `^(?P<emulator>emulator-\d*)[\s+](?P<state>.*)`
	deviceListItemRegexp := regexp.MustCompile(deviceListItemPattern)

	deviceStateMap := map[string]string{}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		matches := deviceListItemRegexp.FindStringSubmatch(line)
		if len(matches) == 3 {
			serial := matches[1]
			state := matches[2]

			deviceStateMap[serial] = state
		}

	}
	if scanner.Err() != nil {
		return map[string]string{}, fmt.Errorf("scanner failed, error: %s", err)
	}

	return deviceStateMap, nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := command.New("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		failf("Issue with input: %s", err)
	}

	//
	// Validate AVD image
	fmt.Println()
	log.Infof("Validate AVD image")

	avdImages, err := listAVDImages()
	if err != nil {
		failf("Failed to list AVD images, error: %s", err)
	}

	if !sliceutil.IsStringInSlice(configs.EmulatorName, avdImages) {
		log.Errorf("AVD image not exists with name: %s", configs.EmulatorName)

		if len(avdImages) > 0 {
			log.Printf("Available avd images:")
			for _, avdImage := range avdImages {
				log.Printf("* %s", avdImage)
			}
		}

		os.Exit(1)
	}

	log.Donef("AVD image (%s) exist", configs.EmulatorName)
	// ---

	adb, err := tools.NewADB(configs.AndroidHome)
	if err != nil {
		failf("Failed to create adb model, error: %s", err)
	}

	//
	// Print running devices Info
	deviceStateMap, err := runningDeviceInfos(*adb)
	if err != nil {
		failf("Failed to list running device infos, error: %s", err)
	}

	if len(deviceStateMap) > 0 {
		fmt.Println()
		log.Infof("Running devices:")

		for serial, state := range deviceStateMap {
			log.Printf("* %s (%s)", serial, state)
		}
	}
	// ---

	emulator, err := tools.NewEmulator(configs.AndroidHome)
	if err != nil {
		failf("Failed to create emulator model, error: %s", err)
	}

	//
	// Start AVD image
	fmt.Println()
	log.Infof("Start AVD image")

	options := []string{}
	if len(configs.EmulatorOptions) > 0 {
		split, err := shellquote.Split(configs.EmulatorOptions)
		if err != nil {
			failf("Failed to split emulatoro ptions (%s), error: %s", configs.EmulatorOptions, err)
		}
		options = split
	}

	startEmulatorCommand := emulator.StartEmulatorCmd(configs.EmulatorName, configs.Skin, options...)
	startEmulatorCmd := startEmulatorCommand.GetCmd()

	e := make(chan error)

	// Redirect output
	stdoutReader, err := startEmulatorCmd.StdoutPipe()
	if err != nil {
		failf("Failed to redirect output, error: %s", err)
	}

	outScanner := bufio.NewScanner(stdoutReader)
	go func() {
		for outScanner.Scan() {
			line := outScanner.Text()
			fmt.Println(line)
		}
	}()
	if err := outScanner.Err(); err != nil {
		failf("Scanner failed, error: %s", err)
	}

	// Redirect error
	stderrReader, err := startEmulatorCmd.StderrPipe()
	if err != nil {
		failf("Failed to redirect error, error: %s", err)
	}

	errScanner := bufio.NewScanner(stderrReader)
	go func() {
		for errScanner.Scan() {
			line := errScanner.Text()
			log.Warnf(line)
		}
	}()
	if err := errScanner.Err(); err != nil {
		failf("Scanner failed, error: %s", err)
	}
	// ---

	serial := ""

	go func() {
		// Start emulator
		log.Printf("$ %s", command.PrintableCommandArgs(false, startEmulatorCmd.Args))
		fmt.Println()

		if err := startEmulatorCommand.Run(); err != nil {
			e <- err
			return
		}
	}()

	go func() {
		// Wait until device appears in device list
		for len(serial) == 0 {
			time.Sleep(5 * time.Second)

			log.Printf("> Checking for started device serial...")

			currentDeviceStateMap, err := runningDeviceInfos(*adb)
			if err != nil {
				e <- err
				return
			}

			serial = currentlyStartedDeviceSerial(deviceStateMap, currentDeviceStateMap)
		}

		log.Donef("> Started device serial: %s", serial)

		// Wait until device is booted
		bootInProgress := true
		for bootInProgress {
			time.Sleep(5 * time.Second)

			log.Printf("> Checking if device booted...")

			booted, err := adb.IsDeviceBooted(serial)
			if err != nil {
				e <- err
				return
			}

			bootInProgress = !booted
		}

		log.Donef("> Device booted")

		e <- nil
	}()

	select {
	case <-time.After(1600 * time.Second):
		if err := startEmulatorCmd.Process.Kill(); err != nil {
			failf("Failed to kill emulator command, error: %s", err)
		}

		failf("Start emulator timed out")
	case err := <-e:
		if err != nil {
			failf("Failed to start emultor, error: %s", err)
		}

	}
	// ---

	if err := exportEnvironmentWithEnvman("BITRISE_EMULATOR_SERIAL", serial); err != nil {
		log.Warnf("Failed to export environment (BITRISE_EMULATOR_SERIAL), error: %s", err)
	}

	fmt.Println()
	log.Donef("Emulator (%s) booted", serial)
}
