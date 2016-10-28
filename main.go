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

	"github.com/bitrise-io/go-utils/cmdex"
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
	log.Info("Configs:")
	log.Detail("- EmulatorName: %s", configs.EmulatorName)
	log.Detail("- Skin: %s", configs.Skin)
	log.Detail("- EmulatorOptions: %s", configs.EmulatorOptions)
	log.Detail("- AndroidHome: %s", configs.AndroidHome)
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

	log.Detail("$ %s", cmdex.PrintableCommandArgs(false, cmd.GetCmd().Args))

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return map[string]string{}, fmt.Errorf("command failed, error: %s", err)
	}

	log.Detail(out)

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
	cmd := cmdex.NewCommand("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		log.Error("Issue with input: %s", err)
		os.Exit(1)
	}

	//
	// Validate AVD image
	fmt.Println()
	log.Info("Validate AVD image")

	avdImages, err := listAVDImages()
	if err != nil {
		log.Error("Failed to list AVD images, error: %s", err)
		os.Exit(1)
	}

	if !sliceutil.IsStringInSlice(configs.EmulatorName, avdImages) {
		log.Error("AVD image not exists with name: %s", configs.EmulatorName)

		if len(avdImages) > 0 {
			log.Detail("Available avd images:")
			for _, avdImage := range avdImages {
				log.Detail("* %s", avdImage)
			}
		}

		os.Exit(1)
	}

	log.Done("AVD image (%s) exist", configs.EmulatorName)
	// ---

	adb, err := tools.NewADB(configs.AndroidHome)
	if err != nil {
		log.Error("Failed to create adb model, error: %s", err)
		os.Exit(1)
	}

	//
	// Print running devices Info
	deviceStateMap, err := runningDeviceInfos(*adb)
	if err != nil {
		log.Error("Failed to list running device infos, error: %s", err)
		os.Exit(1)
	}

	if len(deviceStateMap) > 0 {
		fmt.Println()
		log.Info("Running devices:")

		for serial, state := range deviceStateMap {
			log.Detail("* %s (%s)", serial, state)
		}
	}
	// ---

	emulator, err := tools.NewEmulator(configs.AndroidHome)
	if err != nil {
		log.Error("Failed to create emulator model, error: %s", err)
		os.Exit(1)
	}

	//
	// Start AVD image
	fmt.Println()
	log.Info("Start AVD image")

	options := []string{}
	if len(configs.EmulatorOptions) > 0 {
		split, err := shellquote.Split(configs.EmulatorOptions)
		if err != nil {
			log.Error("Failed to split emulatoro ptions (%s), error: %s", configs.EmulatorOptions, err)
			os.Exit(1)
		}
		options = split
	}

	startEmulatorCommand := emulator.StartEmulatorCmd(configs.EmulatorName, configs.Skin, options...)
	startEmulatorCmd := startEmulatorCommand.GetCmd()

	e := make(chan error)

	// Redirect output
	stdoutReader, err := startEmulatorCmd.StdoutPipe()
	if err != nil {
		log.Error("Failed to redirect output, error: %s", err)
		os.Exit(1)
	}

	outScanner := bufio.NewScanner(stdoutReader)
	go func() {
		for outScanner.Scan() {
			line := outScanner.Text()
			fmt.Println(line)
		}
	}()
	if err := outScanner.Err(); err != nil {
		log.Error("Scanner failed, error: %s", err)
		os.Exit(1)
	}

	// Redirect error
	stderrReader, err := startEmulatorCmd.StderrPipe()
	if err != nil {
		log.Error("Failed to redirect error, error: %s", err)
		os.Exit(1)
	}

	errScanner := bufio.NewScanner(stderrReader)
	go func() {
		for errScanner.Scan() {
			line := errScanner.Text()
			log.Warn(line)

			// e <- errors.New(line)
		}
	}()
	if err := errScanner.Err(); err != nil {
		log.Error("Scanner failed, error: %s", err)
		os.Exit(1)
	}
	// ---

	serial := ""

	go func() {
		// Start emulator
		log.Detail("$ %s", cmdex.PrintableCommandArgs(false, startEmulatorCmd.Args))
		fmt.Println()

		if err := startEmulatorCommand.Run(); err != nil {
			log.Error("Start failed: %s", err.Error())
			e <- err
			return
		}
	}()

	go func() {
		// Wait until device appears in device list
		for len(serial) == 0 {
			time.Sleep(5 * time.Second)

			log.Detail("> Checking for started device serial...")

			currentDeviceStateMap, err := runningDeviceInfos(*adb)
			if err != nil {
				e <- err
				return
			}

			serial = currentlyStartedDeviceSerial(deviceStateMap, currentDeviceStateMap)
		}

		log.Done("> Started device serial: %s", serial)

		// Wait until device is booted
		bootInProgress := true
		for bootInProgress {
			time.Sleep(5 * time.Second)

			log.Detail("> Checking if device booted...")

			booted, err := adb.IsDeviceBooted(serial)
			if err != nil {
				e <- err
				return
			}

			bootInProgress = !booted
		}

		log.Done("> Device booted")

		e <- nil
	}()

	select {
	case <-time.After(800 * time.Second):
		if err := startEmulatorCmd.Process.Kill(); err != nil {
			log.Error("Failed to kill emulator command, error: %s", err)
			os.Exit(1)
		}

		log.Error("Start emulator timed out")
		os.Exit(1)
	case err := <-e:
		if err != nil {
			log.Error("Failed to start emultor, error: %s", err)
			os.Exit(1)
		}

	}
	// ---

	if err := exportEnvironmentWithEnvman("BITRISE_EMULATOR_SERIAL", serial); err != nil {
		log.Warn("Failed to export environment (BITRISE_EMULATOR_SERIAL), error: %s", err)
	}

	fmt.Println()
	log.Done("Emulator (%s) booted", serial)
}
