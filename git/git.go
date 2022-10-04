package git

import (
	"bufio"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/say"
	"os"
	"os/exec"
	"strings"
)

var (
	WorkingDir                 = ""
	GitPassthroughStderrStdout = false // hack to get git hooks to print to stdout/stderr
)

func HasLocalBranch(localBranch string) bool {
	localBranches := GitBranches()
	say.Debug("Local Branches: " + strings.Join(localBranches, "\n"))
	say.Debug("Local Branch: " + localBranch)

	for _, branch := range localBranches {
		if branch == localBranch {
			return true
		}
	}
	return false
}

func CurrentBranch() string {
	// upgrade to branch --show-current when git v2.21 is more widely spread
	return Silentgit("rev-parse", "--abbrev-ref", "HEAD")
}

func Fetch(configuration config.Configuration) {
	Git("fetch", configuration.RemoteName, "--prune")
}

func GitBranches() []string {
	return strings.Split(Silentgit("branch", "--format=%(refname:short)"), "\n")
}

func GitRemoteBranches() []string {
	return strings.Split(Silentgit("branch", "--remotes", "--format=%(refname:short)"), "\n")
}

func GitDir() string {
	return Silentgit("rev-parse", "--absolute-git-dir")
}

func GitRootDir() string {
	return Silentgit("rev-parse", "--show-toplevel")
}

func GitUserName() string {
	return Silentgitignorefailure("config", "--get", "user.name")
}

func GitUserEmail() string {
	return Silentgit("config", "--get", "user.email")
}

func IsGitRepository() bool {
	_, _, err := RunCommandSilent("git", "rev-parse")
	return err == nil
}

func RunCommandSilent(name string, args ...string) (string, string, error) {
	command := exec.Command(name, args...)
	if len(WorkingDir) > 0 {
		command.Dir = WorkingDir
	}
	commandString := strings.Join(command.Args, " ")
	say.Debug("Running command <" + commandString + "> in silent mode, capturing combined output")
	outputBytes, err := command.CombinedOutput()
	output := string(outputBytes)
	say.Debug(output)
	return commandString, output, err
}

func runCommand(name string, args ...string) (string, string, error) {
	command := exec.Command(name, args...)
	if len(WorkingDir) > 0 {
		command.Dir = WorkingDir
	}
	commandString := strings.Join(command.Args, " ")
	say.Debug("Running command <" + commandString + "> passing output through")

	stdout, _ := command.StdoutPipe()
	command.Stderr = command.Stdout
	errStart := command.Start()
	if errStart != nil {
		return commandString, "", errStart
	}

	output := ""

	stdoutscanner := bufio.NewScanner(stdout)
	lineEnded := true
	stdoutscanner.Split(bufio.ScanBytes)
	for stdoutscanner.Scan() {
		character := stdoutscanner.Text()
		if character == "\n" {
			lineEnded = true
		} else {
			if lineEnded {
				say.PrintToConsole("  ")
				lineEnded = false
			}
		}
		say.PrintToConsole(character)
		output += character
	}

	errWait := command.Wait()
	if errWait != nil {
		say.Debug(output)
		return commandString, output, errWait
	}

	say.Debug(output)
	return commandString, output, nil
}

func Silentgit(args ...string) string {
	commandString, output, err := RunCommandSilent("git", args...)

	if err != nil {
		if !IsGitRepository() {
			say.Error("expecting the current working directory to be a git repository.")
		} else {
			say.Error(commandString)
			say.Error(output)
			say.Error(err.Error())
		}
		os.Exit(1)
	}
	return strings.TrimSpace(output)
}

func Silentgitignorefailure(args ...string) string {
	_, output, err := RunCommandSilent("git", args...)

	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

func deleteEmptyStrings(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func GitWithoutEmptyStrings(args ...string) {
	argsWithoutEmptyStrings := deleteEmptyStrings(args)
	Git(argsWithoutEmptyStrings...)
}

func Git(args ...string) {
	commandString, output, err := "", "", error(nil)
	if GitPassthroughStderrStdout {
		commandString, output, err = runCommand("git", args...)
	} else {
		commandString, output, err = RunCommandSilent("git", args...)
	}

	if err != nil {
		if !IsGitRepository() {
			say.Error("expecting the current working directory to be a git repository.")
		} else {
			say.Error(commandString)
			say.Error(output)
			say.Error(err.Error())
		}
		os.Exit(1)
	} else {
		say.Indented(commandString)
	}
}

func Gitignorefailure(args ...string) error {
	commandString, output, err := "", "", error(nil)
	if GitPassthroughStderrStdout {
		commandString, output, err = runCommand("git", args...)
	} else {
		commandString, output, err = RunCommandSilent("git", args...)
	}

	say.Indented(commandString)

	if err != nil {
		if !IsGitRepository() {
			say.Error("expecting the current working directory to be a git repository.")
			os.Exit(1)
		} else {
			say.Warning(commandString)
			say.Warning(output)
			say.Warning(err.Error())
			return err
		}
	}

	say.Indented(commandString)
	return nil
}

func DoBranchesDiverge(ancestor string, successor string) bool {
	_, _, err := RunCommandSilent("git", "merge-base", "--is-ancestor", ancestor, successor)
	if err == nil {
		return false
	}
	return true
}

func GitCommitHash() string {
	return Silentgitignorefailure("rev-parse", "HEAD")
}

func gitCurrentBranch() string {
	// upgrade to branch --show-current when git v2.21 is more widely spread
	return Silentgit("rev-parse", "--abbrev-ref", "HEAD")
}

func IsGitInstalled() bool {
	_, _, err := RunCommandSilent("git", "--version")
	if err != nil {
		say.Debug("isGitInstalled encountered an error: " + err.Error())
	}
	return err == nil
}
