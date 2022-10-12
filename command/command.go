package command

import (
	"github.com/remotemobprogramming/mob/v4/say"
	"os/exec"
	"strings"
)

func RunCommandSilent(name string, args ...string) (string, string, error) {
	return RunCommandSilentIn("", name, args...)
}

func RunCommandSilentIn(workingDir, name string, args ...string) (string, string, error) {
	command := exec.Command(name, args...)
	if len(workingDir) > 0 {
		command.Dir = workingDir
	}
	commandString := strings.Join(command.Args, " ")
	say.Debug("Running command <" + commandString + "> in silent mode, capturing combined output")
	outputBytes, err := command.CombinedOutput()
	output := string(outputBytes)
	say.Debug(output)
	return commandString, output, err
}
