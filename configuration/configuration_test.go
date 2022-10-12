package configuration_test

import (
	"fmt"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/say"
	"github.com/remotemobprogramming/mob/v4/test"
	"os"
	"strings"
	"testing"
)

var (
	tempDir string
)

func TestQuote(t *testing.T) {
	output, _ := test.Setup(t)
	os.Setenv("MOB_TIMER_ROOM", "mobRoom1234")
	configuration := config.ReadConfiguration("", "")

	config.Config(configuration)

	test.AssertOutputContains(t, output, "\"mobRoom1234\"")
}

func TestQuoteWithQuote(t *testing.T) {
	output, _ := test.Setup(t)
	os.Setenv("MOB_TIMER_ROOM", "mob\"Room1234")
	configuration := config.ReadConfiguration("", "")

	config.Config(configuration)

	test.AssertOutputContains(t, output, "\"mob\\\"Room1234\"")
}

func TestParseArgs(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	test.Equals(t, configuration.WipBranchQualifier, "")

	command, parameters, configuration := config.ParseArgs([]string{"mob", "start", "--branch", "green"}, configuration)

	test.Equals(t, "start", command)
	test.Equals(t, "", strings.Join(parameters, ""))
	test.Equals(t, "green", configuration.WipBranchQualifier)
}

func TestParseArgsStartCreate(t *testing.T) {
	configuration := config.GetDefaultConfiguration()

	command, parameters, configuration := config.ParseArgs([]string{"mob", "start", "--create"}, configuration)

	test.Equals(t, "start", command)
	test.Equals(t, "", strings.Join(parameters, ""))
	test.Equals(t, true, configuration.StartCreate)
}

func TestParseArgsDoneNoSquash(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	test.Equals(t, config.Squash, configuration.DoneSquash)

	command, parameters, configuration := config.ParseArgs([]string{"mob", "done", "--no-squash"}, configuration)

	test.Equals(t, "done", command)
	test.Equals(t, "", strings.Join(parameters, ""))
	test.Equals(t, config.NoSquash, configuration.DoneSquash)
}

func TestParseArgsDoneSquash(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	configuration.DoneSquash = config.NoSquash

	command, parameters, configuration := config.ParseArgs([]string{"mob", "done", "--squash"}, configuration)

	test.Equals(t, "done", command)
	test.Equals(t, "", strings.Join(parameters, ""))
	test.Equals(t, config.Squash, configuration.DoneSquash)
}

func TestParseArgsMessage(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	test.Equals(t, configuration.WipBranchQualifier, "")

	command, parameters, configuration := config.ParseArgs([]string{"mob", "next", "--message", "ci-skip"}, configuration)

	test.Equals(t, "next", command)
	test.Equals(t, "", strings.Join(parameters, ""))
	test.Equals(t, "ci-skip", configuration.WipCommitMessage)
}

func TestMobRemoteNameEnvironmentVariable(t *testing.T) {
	configuration := setEnvVarAndParse("MOB_REMOTE_NAME", "GITHUB")

	test.Equals(t, "GITHUB", configuration.RemoteName)
}

func TestMobRemoteNameEnvironmentVariableEmptyString(t *testing.T) {
	configuration := setEnvVarAndParse("MOB_REMOTE_NAME", "")

	test.Equals(t, "origin", configuration.RemoteName)
}

func TestMobDoneSquashEnvironmentVariable(t *testing.T) {
	assertMobDoneSquashValue(t, "", config.Squash)
	assertMobDoneSquashValue(t, "garbage", config.Squash)
	assertMobDoneSquashValue(t, "squash", config.Squash)
	assertMobDoneSquashValue(t, "no-squash", config.NoSquash)
	assertMobDoneSquashValue(t, "squash-wip", config.SquashWip)
}

func assertMobDoneSquashValue(t *testing.T, value string, expected string) {
	configuration := setEnvVarAndParse("MOB_DONE_SQUASH", value)
	test.Equals(t, expected, configuration.DoneSquash)
}

func TestBooleanEnvironmentVariables(t *testing.T) {
	assertBoolEnvVarParsed(t, "MOB_START_CREATE", false, getMobStartCreateRemoteBranch)
	assertBoolEnvVarParsed(t, "MOB_NEXT_STAY", true, getMobNextStay)
	assertBoolEnvVarParsed(t, "MOB_REQUIRE_COMMIT_MESSAGE", false, getRequireCommitMessage)
}

func assertBoolEnvVarParsed(t *testing.T, envVar string, defaultValue bool, actual func(config.Configuration) bool) {
	t.Run(envVar, func(t *testing.T) {
		assertEnvVarParsed(t, envVar, "", defaultValue, boolToInterface(actual))
		assertEnvVarParsed(t, envVar, "true", true, boolToInterface(actual))
		assertEnvVarParsed(t, envVar, "false", false, boolToInterface(actual))
		assertEnvVarParsed(t, envVar, "garbage", defaultValue, boolToInterface(actual))
	})
}

func assertEnvVarParsed(t *testing.T, variable string, value string, expected interface{}, actual func(config.Configuration) interface{}) {
	t.Run(fmt.Sprintf("%s=\"%s\"->(expects:%t)", variable, value, expected), func(t *testing.T) {
		configuration := setEnvVarAndParse(variable, value)
		test.Equals(t, expected, actual(configuration))
	})
}

func setEnvVarAndParse(variable string, value string) config.Configuration {
	os.Setenv(variable, value)
	defer os.Unsetenv(variable)

	return config.ReadConfiguration("", "")
}

func boolToInterface(actual func(config.Configuration) bool) func(c config.Configuration) interface{} {
	return func(c config.Configuration) interface{} {
		return actual(c)
	}
}

var getMobStartCreateRemoteBranch = func(c config.Configuration) bool {
	return c.StartCreate
}

var getMobNextStay = func(c config.Configuration) bool {
	return c.NextStay
}

var getRequireCommitMessage = func(c config.Configuration) bool {
	return c.RequireCommitMessage
}

func TestParseRequireCommitMessageEnvVariables(t *testing.T) {
	os.Unsetenv("MOB_REQUIRE_COMMIT_MESSAGE")
	defer os.Unsetenv("MOB_REQUIRE_COMMIT_MESSAGE")

	configuration := config.ReadConfiguration("", "")
	test.Equals(t, false, configuration.RequireCommitMessage)

	os.Setenv("MOB_REQUIRE_COMMIT_MESSAGE", "false")
	configuration = config.ReadConfiguration("", "")
	test.Equals(t, false, configuration.RequireCommitMessage)

	os.Setenv("MOB_REQUIRE_COMMIT_MESSAGE", "true")
	configuration = config.ReadConfiguration("", "")
	test.Equals(t, true, configuration.RequireCommitMessage)
}

func TestReadConfigurationFromFileOverrideEverything(t *testing.T) {
	say.TurnOnDebugging()
	tempDir = t.TempDir()
	test.SetWorkingDir(tempDir)

	test.CreateFile(t, ".mob", `
		MOB_CLI_NAME="team"
		MOB_REMOTE_NAME="gitlab"
		MOB_WIP_COMMIT_MESSAGE="team next"
		MOB_REQUIRE_COMMIT_MESSAGE=true
		MOB_VOICE_COMMAND="whisper \"%s\""
		MOB_VOICE_MESSAGE="team next"
		MOB_NOTIFY_COMMAND="/usr/bin/osascript -e 'display notification \"%s!!!\"'"
		MOB_NOTIFY_MESSAGE="team next"
		MOB_NEXT_STAY=false
		MOB_START_CREATE=true
		MOB_WIP_BRANCH_QUALIFIER="green"
		MOB_WIP_BRANCH_QUALIFIER_SEPARATOR="---"
		MOB_WIP_BRANCH_PREFIX="ensemble/"
		MOB_DONE_SQUASH=no-squash
		MOB_OPEN_COMMAND="idea %s"
		MOB_TIMER="123"
		MOB_TIMER_ROOM="Room_42"
		MOB_TIMER_ROOM_USE_WIP_BRANCH_QUALIFIER=true
		MOB_TIMER_LOCAL=false
		MOB_TIMER_USER="Mona"
		MOB_TIMER_URL="https://timer.innoq.io/"
		MOB_STASH_NAME="team-stash-name"
	`)
	actualConfiguration := config.ReadConfiguration(tempDir, "")
	test.Equals(t, "team", actualConfiguration.CliName)
	test.Equals(t, "gitlab", actualConfiguration.RemoteName)
	test.Equals(t, "team next", actualConfiguration.WipCommitMessage)
	test.Equals(t, true, actualConfiguration.RequireCommitMessage)
	test.Equals(t, "whisper \"%s\"", actualConfiguration.VoiceCommand)
	test.Equals(t, "team next", actualConfiguration.VoiceMessage)
	test.Equals(t, "/usr/bin/osascript -e 'display notification \"%s!!!\"'", actualConfiguration.NotifyCommand)
	test.Equals(t, "team next", actualConfiguration.NotifyMessage)
	test.Equals(t, false, actualConfiguration.NextStay)
	test.Equals(t, true, actualConfiguration.StartCreate)
	test.Equals(t, "green", actualConfiguration.WipBranchQualifier)
	test.Equals(t, "---", actualConfiguration.WipBranchQualifierSeparator)
	test.Equals(t, "ensemble/", actualConfiguration.WipBranchPrefix)
	test.Equals(t, config.NoSquash, actualConfiguration.DoneSquash)
	test.Equals(t, "idea %s", actualConfiguration.OpenCommand)
	test.Equals(t, "123", actualConfiguration.Timer)
	test.Equals(t, "Room_42", actualConfiguration.TimerRoom)
	test.Equals(t, true, actualConfiguration.TimerRoomUseWipBranchQualifier)
	test.Equals(t, false, actualConfiguration.TimerLocal)
	test.Equals(t, "Mona", actualConfiguration.TimerUser)
	test.Equals(t, "https://timer.innoq.io/", actualConfiguration.TimerUrl)
	test.Equals(t, "team-stash-name", actualConfiguration.StashName)

	test.CreateFile(t, ".mob", "\nMOB_TIMER_ROOM=\"Room\\\"\\\"_42\"\n")
	actualConfiguration1 := config.ReadConfiguration(tempDir, "")
	test.Equals(t, "Room\"\"_42", actualConfiguration1.TimerRoom)
}

func TestReadConfigurationFromFileWithNonBooleanQuotedDoneSquashValue(t *testing.T) {
	say.TurnOnDebugging()
	tempDir = t.TempDir()
	test.SetWorkingDir(tempDir)

	test.CreateFile(t, ".mob", "\nMOB_DONE_SQUASH=\"squash-wip\"")
	actualConfiguration := config.ReadConfiguration(tempDir, "")
	test.Equals(t, config.SquashWip, actualConfiguration.DoneSquash)
}

func TestReadConfigurationFromFileAndSkipBrokenLines(t *testing.T) {
	say.TurnOnDebugging()
	tempDir = t.TempDir()
	test.SetWorkingDir(tempDir)

	test.CreateFile(t, ".mob", "\nMOB_TIMER_ROOM=\"Broken\" \"String\"")
	actualConfiguration := config.ReadConfiguration(tempDir, "")
	test.Equals(t, config.GetDefaultConfiguration().TimerRoom, actualConfiguration.TimerRoom)
}

func TestSkipIfConfigurationDoesNotExist(t *testing.T) {
	say.TurnOnDebugging()
	tempDir = t.TempDir()
	test.SetWorkingDir(tempDir)

	actualConfiguration := config.ReadConfiguration(tempDir, "")
	test.Equals(t, config.GetDefaultConfiguration(), actualConfiguration)
}

func TestSetMobDoneSquash(t *testing.T) {
	os.Setenv("MOB_DONE_SQUASH", "no-squash")
	configuration := config.ReadConfiguration("", "")
	test.Equals(t, config.NoSquash, configuration.DoneSquash)

	os.Setenv("MOB_DONE_SQUASH", "squash")
	configuration = config.ReadConfiguration("", "")
	test.Equals(t, config.Squash, configuration.DoneSquash)

	os.Setenv("MOB_DONE_SQUASH", "squash-wip")
	configuration = config.ReadConfiguration("", "")
	test.Equals(t, config.SquashWip, configuration.DoneSquash)
}

func TestSetMobDoneSquashGarbageValue(t *testing.T) {
	os.Setenv("MOB_DONE_SQUASH", "garbage")
	configuration := config.ReadConfiguration("", "")

	test.Equals(t, config.Squash, configuration.DoneSquash)
}

func TestSetMobDoneSquashEmptyStringValue(t *testing.T) {
	os.Setenv("MOB_DONE_SQUASH", "garbage")
	configuration := config.ReadConfiguration("", "")

	test.Equals(t, config.Squash, configuration.DoneSquash)
}
