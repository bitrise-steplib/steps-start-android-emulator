package emulatormanager

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-android/sdk"
)

// Model ...
type Model struct {
	binPth string
	env    string
}

// New ...
func New(sdk sdk.AndroidSdkInterface) (*Model, error) {
	binPth := filepath.Join(sdk.GetAndroidHome(), "emulator", "emulator")
	env := ""
	if runtime.GOOS == "linux" {
		binPth = filepath.Join(sdk.GetAndroidHome(), "emulator", "emulator64-arm")
		env = "LD_LIBRARY_PATH=" + filepath.Join(sdk.GetAndroidHome(), "emulator", "lib64", "qt", "lib")
	}

	exist, err := pathutil.IsPathExists(binPth)
	if err != nil {
		return nil, err
	} else if !exist {
		binPth = filepath.Join(sdk.GetAndroidHome(), "tools", "emulator")
		if runtime.GOOS == "linux" {
			binPth = filepath.Join(sdk.GetAndroidHome(), "tools", "emulator64-arm")
		}
	}

	if exist, err := pathutil.IsPathExists(binPth); err != nil {
		return nil, fmt.Errorf("failed to check if emulator exist, error: %s", err)
	} else if !exist {
		return nil, fmt.Errorf("emulator not exist at: %s", binPth)
	}

	return &Model{
		binPth: binPth,
		env: env,
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

	return command.New(args[0], args[1:]...).AppendEnvs(model.env)
}
