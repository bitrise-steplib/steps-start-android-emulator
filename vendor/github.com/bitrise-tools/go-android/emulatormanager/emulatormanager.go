package emulatormanager

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-android/sdk"
)

// Model ...
type Model struct {
	binPth string
	envs   []string
}

func emulatorBinPth(androidHome string) (string, error) {
	binPth := filepath.Join(androidHome, "emulator", "emulator64-arm")
	if exist, err := pathutil.IsPathExists(binPth); err != nil {
		return "", err
	} else if !exist {
		binPth = filepath.Join(androidHome, "emulator", "emulator")
		if exist, err := pathutil.IsPathExists(binPth); err != nil {
			return "", err
		} else if !exist {
			return "", fmt.Errorf("no emulator binary found in $ANDROID_HOME/emulator")
		}
	}
	return binPth, nil
}

func lib64QTLibEnv(androidHome, hostOSName string) (string, error) {
	envKey := ""
	libPth := filepath.Join(androidHome, "emulator", "lib64", "qt", "lib")

	if hostOSName == "linux" {
		envKey = "LD_LIBRARY_PATH"
	} else if hostOSName == "darwin" {
		envKey = "DYLD_LIBRARY_PATH"
	} else {
		return "", fmt.Errorf("unsupported os %s", hostOSName)
	}

	if exist, err := pathutil.IsPathExists(libPth); err != nil {
		return "", err
	} else if !exist {
		return "", fmt.Errorf("qt lib does not exist at: %s", libPth)
	}

	return envKey + "=" + libPth, nil
}

// New ...
func New(sdk sdk.AndroidSdkInterface) (*Model, error) {
	binPth, err := emulatorBinPth(sdk.GetAndroidHome())
	if err != nil {
		return nil, err
	}

	envs := []string{}
	if strings.HasSuffix(binPth, "emulator64-arm") {
		env, err := lib64QTLibEnv(sdk.GetAndroidHome(), runtime.GOOS)
		if err != nil {
			log.Warnf("failed to get lib64 qt lib path, error: %s", err)
		} else {
			envs = append(envs, env)
		}
	}

	return &Model{
		binPth: binPth,
		envs:   envs,
	}, nil
}

// StartEmulatorCommand ...
func (model Model) StartEmulatorCommand(name, skin string, options ...string) *command.Model {
	args := []string{model.binPth, "-avd", name}
	if len(skin) == 0 {
		args = append(args, "-noskin")
	} else {
		args = append(args, "-skin", skin)
	}
	args = append(args, options...)

	commandModel := command.New(args[0], args[1:]...).AppendEnvs(model.envs...)

	return commandModel
}
