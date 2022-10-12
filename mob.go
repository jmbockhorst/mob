package main

import (
	"bytes"
	"crypto/tls"
	x509 "crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/remotemobprogramming/mob/v4/branches"
	"github.com/remotemobprogramming/mob/v4/command"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/git"
	"github.com/remotemobprogramming/mob/v4/help"
	"github.com/remotemobprogramming/mob/v4/reset"
	"github.com/remotemobprogramming/mob/v4/say"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	versionNumber = "4.0.0"
)

var (
	workingDir = ""
)

func openCommandFor(c config.Configuration, filepath string) (string, []string) {
	if !c.IsOpenCommandGiven() {
		return "", []string{}
	}
	split := strings.Split(injectCommandWithMessage(c.OpenCommand, filepath), " ")
	return split[0], split[1:]
}

func main() {
	say.TurnOnDebuggingByArgs(os.Args)
	say.Debug(runtime.Version())

	if !git.IsGitInstalled() {
		say.Error("'git' command was not found in PATH. It may be not installed. " +
			"To learn how to install 'git' refer to https://git-scm.com/book/en/v2/Getting-Started-Installing-Git.")
		exit(1)
	}

	projectRootDir := ""
	if git.IsGitRepository() {
		projectRootDir = git.GitRootDir()
	}
	currentUser, _ := user.Current()
	homeDir := currentUser.HomeDir
	configuration := config.ReadConfiguration(homeDir, projectRootDir)
	say.Debug("Args '" + strings.Join(os.Args, " ") + "'")
	currentCliName := currentCliName(os.Args[0])
	if currentCliName != configuration.CliName {
		say.Debug("Updating cli name to " + currentCliName)
		configuration.CliName = currentCliName
	}

	command, parameters, configuration := config.ParseArgs(os.Args, configuration)
	say.Debug("command '" + command + "'")
	say.Debug("parameters '" + strings.Join(parameters, " ") + "'")
	say.Debug("version " + versionNumber)
	say.Debug("workingDir '" + git.WorkingDir + "'")

	// workaround until we have a better design
	if configuration.GitHooksEnabled {
		git.GitPassthroughStderrStdout = true
	}

	execute(command, parameters, configuration)
}

func currentCliName(argZero string) string {
	argZero = strings.TrimSuffix(argZero, ".exe")
	if strings.Contains(argZero, "/") {
		argZero = argZero[strings.LastIndex(argZero, "/")+1:]
	}
	return argZero
}

func execute(command string, parameter []string, configuration config.Configuration) {
	if helpRequested(parameter) {
		help.Help(configuration)
		return
	}

	switch command {
	case "s", "start":
		err := start(configuration)
		if !isMobProgramming(configuration) || err != nil {
			return
		}
		if len(parameter) > 0 {
			timer := parameter[0]
			startTimer(timer, configuration)
		} else if configuration.Timer != "" {
			startTimer(configuration.Timer, configuration)
		} else {
			say.Info("It's now " + currentTime() + ". Happy collaborating! :)")
		}
	case "b", "branch":
		branch(configuration)
	case "n", "next":
		next(configuration)
	case "d", "done":
		done(configuration)
	case "fetch":
		git.Fetch(configuration)
	case "reset":
		reset.Reset(configuration)
	case "clean":
		clean(configuration)
	case "config":
		config.Config(configuration)
	case "status":
		status(configuration)
	case "t", "timer":
		if len(parameter) > 0 {
			timer := parameter[0]
			startTimer(timer, configuration)
		} else if configuration.Timer != "" {
			startTimer(configuration.Timer, configuration)
		} else {
			help.Help(configuration)
		}
	case "break":
		if len(parameter) > 0 {
			startBreakTimer(parameter[0], configuration)
		} else {
			help.Help(configuration)
		}
	case "moo":
		moo(configuration)
	case "sw", "squash-wip":
		if len(parameter) > 1 && parameter[0] == "--git-editor" {
			squashWipGitEditor(parameter[1], configuration)
		} else if len(parameter) > 1 && parameter[0] == "--git-sequence-editor" {
			squashWipGitSequenceEditor(parameter[1], configuration)
		}
	case "version", "--version", "-v":
		version()
	case "help", "--help", "-h":
		help.Help(configuration)
	default:
		help.Help(configuration)
	}
}

func helpRequested(parameter []string) bool {
	for i := 0; i < len(parameter); i++ {
		element := parameter[i]
		if element == "help" || element == "--help" || element == "-h" {
			return true
		}
	}
	return false
}

func clean(configuration config.Configuration) {
	git.Git("fetch", configuration.RemoteName)

	currentBranch := branches.CurrentBranch()
	localBranches := git.GitBranches()

	if currentBranch.IsOrphanWipBranch(configuration) {
		currentBaseBranch, _ := branches.DetermineBranches(currentBranch, localBranches, configuration)

		say.Info("Current branch " + currentBranch.Name + " is an orphan")
		if currentBaseBranch.Exists(localBranches) {
			git.Git("checkout", currentBaseBranch.Name)
		} else if branches.NewBranch("main").Exists(localBranches) {
			git.Git("checkout", "main")
		} else {
			git.Git("checkout", "master")
		}
	}

	for _, branch := range localBranches {
		b := branches.NewBranch(branch)
		if b.IsOrphanWipBranch(configuration) {
			say.Info("Removing orphan wip branch " + b.Name)
			git.Git("branch", "-D", b.Name)
		}
	}

}

func branch(configuration config.Configuration) {
	say.Say(git.Silentgit("branch", "--list", "--remote", branches.NewBranch("*").AddWipPrefix(configuration).Remote(configuration).Name))

	// DEPRECATED
	say.Say(git.Silentgit("branch", "--list", "--remote", branches.NewBranch("mob-session").Remote(configuration).Name))
}

func getSleepCommand(timeoutInSeconds int) string {
	return fmt.Sprintf("sleep %d", timeoutInSeconds)
}

func injectCommandWithMessage(command string, message string) string {
	placeHolders := strings.Count(command, "%s")
	if placeHolders > 1 {
		say.Error(fmt.Sprintf("Too many placeholders (%d) in format command string: %s", placeHolders, command))
		exit(1)
	}
	if placeHolders == 0 {
		return fmt.Sprintf("%s %s", command, message)
	}
	return fmt.Sprintf(command, message)
}

func getVoiceCommand(message string, voiceCommand string) string {
	if len(voiceCommand) == 0 {
		return ""
	}
	return injectCommandWithMessage(voiceCommand, message)
}

func getNotifyCommand(message string, notifyCommand string) string {
	if len(notifyCommand) == 0 {
		return ""
	}
	return injectCommandWithMessage(notifyCommand, message)
}

func executeCommandsInBackgroundProcess(commands ...string) (err error) {
	cmds := make([]string, 0)
	for _, c := range commands {
		if len(c) > 0 {
			cmds = append(cmds, c)
		}
	}
	say.Debug(fmt.Sprintf("Operating System %s", runtime.GOOS))
	switch runtime.GOOS {
	case "windows":
		_, err = startCommand("powershell", "-command", fmt.Sprintf("start-process powershell -NoNewWindow -ArgumentList '-command \"%s\"'", strings.Join(cmds, ";")))
	case "darwin", "linux":
		_, err = startCommand("sh", "-c", fmt.Sprintf("(%s) &", strings.Join(cmds, ";")))
	default:
		say.Warning(fmt.Sprintf("Cannot execute background commands on your os: %s", runtime.GOOS))
	}
	return err
}

func startTimer(timerInMinutes string, configuration config.Configuration) {
	timeoutInMinutes := toMinutes(timerInMinutes)

	timeoutInSeconds := timeoutInMinutes * 60
	timeOfTimeout := time.Now().Add(time.Minute * time.Duration(timeoutInMinutes)).Format("15:04")
	say.Debug(fmt.Sprintf("Starting timer at %s for %d minutes = %d seconds (parsed from user input %s)", timeOfTimeout, timeoutInMinutes, timeoutInSeconds, timerInMinutes))

	room := getMobTimerRoom(configuration)
	startRemoteTimer := room != ""
	startLocalTimer := configuration.TimerLocal

	if !startRemoteTimer && !startLocalTimer {
		say.Error("No timer configured, not starting timer")
		exit(1)
	}

	if startRemoteTimer {
		timerUser := getUserForMobTimer(configuration.TimerUser)
		err := httpPutTimer(timeoutInMinutes, room, timerUser, configuration.TimerUrl, configuration.TimerInsecure)
		if err != nil {
			say.Error("remote timer couldn't be started")
			say.Error(err.Error())
			exit(1)
		}
	}

	if startLocalTimer {
		abortRunningTimers()
		err := executeCommandsInBackgroundProcess(getSleepCommand(timeoutInSeconds), getVoiceCommand(configuration.VoiceMessage, configuration.VoiceCommand), getNotifyCommand(configuration.NotifyMessage, configuration.NotifyCommand), "echo \"mobTimer\"")

		if err != nil {
			say.Error(fmt.Sprintf("timer couldn't be started on your system (%s)", runtime.GOOS))
			say.Error(err.Error())
			exit(1)
		}
	}

	say.Info("It's now " + currentTime() + ". " + fmt.Sprintf("%d min timer ends at approx. %s", timeoutInMinutes, timeOfTimeout) + ". Happy collaborating! :)")
}

func abortRunningTimers() {
	processIds := findMobTimerProcessIds()
	for _, processId := range processIds {
		killRunningProcess(processId)
	}
}

func killRunningProcess(processId string) {
	var err error
	switch runtime.GOOS {
	case "darwin", "linux":
		err = killRunningProcessLinuxAndDarwin(processId)
		break
	case "windows":
		err = killRunningProcessWindows(processId)
	}
	if err != nil {
		say.Error(fmt.Sprintf("old timer couldn't be aborted on your system (%s)", runtime.GOOS))
		say.Error(err.Error())
	}
	say.Debug("Killed mob timer with PID " + processId)
}

func killRunningProcessLinuxAndDarwin(processId string) error {
	_, _, err := command.RunCommandSilent("kill", processId)
	return err
}

func killRunningProcessWindows(processId string) error {
	_, _, err := command.RunCommandSilent("powershell", "-command", "Stop-Process", "-Id", processId)
	return err
}

func findMobTimerProcessIds() []string {
	say.Debug("find all mob timer processes")
	switch runtime.GOOS {
	case "darwin", "linux":
		return findMobTimerProcessIdsLinuxAndDarwin()
	case "windows":
		return findMobTimerProcessIdsWindows()
	}
	return []string{}
}

func findMobTimerProcessIdsWindows() []string {
	_, output, err := command.RunCommandSilent("powershell", "-command", "Get-WmiObject", "Win32_Process", "-Filter", "\"commandLine LIKE '%mobTimer%'\"", "|", "Select-Object", "-Property", "ProcessId,CommandLine", "|", "Out-String", "-Width", "10000")
	if err != nil {
		say.Error(fmt.Sprintf("could not find processes on your system (%s)", runtime.GOOS))
		say.Error(err.Error())
	}
	lines := strings.Split(output, "\r\n")
	var filteredLines []string
	for _, line := range lines {
		if line != "" {
			filteredLines = append(filteredLines, line)
		}
	}
	processInfos := filteredLines[2:]
	var timerProcessIds []string
	for _, processInfo := range processInfos {
		if strings.Contains(processInfo, "echo \"mobTimer\"") {
			timerProcessIds = append(timerProcessIds, strings.Split(strings.TrimSpace(processInfo), " ")[0])
		}
	}
	return timerProcessIds
}

func findMobTimerProcessIdsLinuxAndDarwin() []string {
	_, output, err := command.RunCommandSilent("ps", "-axo", "pid,command")
	lines := strings.Split(output, "\n")
	if err != nil {
		say.Error(fmt.Sprintf("could not find processes on your system (%s)", runtime.GOOS))
		say.Error(err.Error())
	}
	var processIds []string
	for _, line := range lines {
		if strings.Contains(line, "echo \"mobTimer\"") {
			line = strings.TrimSpace(line)
			processId := strings.Split(line, " ")[0]
			processIds = append(processIds, processId)
			say.Debug("Found mob timer with PID " + processId)
		}
	}
	return processIds
}

func getMobTimerRoom(configuration config.Configuration) string {
	if !git.IsGitRepository() {
		say.Debug("timer not in git repository, using MOB_TIMER_ROOM for room name")
		return configuration.TimerRoom
	}

	currentWipBranchQualifier := branches.GetCurrentWipBranchQualifier(configuration)

	if configuration.TimerRoomUseWipBranchQualifier && currentWipBranchQualifier != "" {
		say.Info("Using wip branch qualifier for room name")
		return currentWipBranchQualifier
	}

	return configuration.TimerRoom
}

func startBreakTimer(timerInMinutes string, configuration config.Configuration) {
	timeoutInMinutes := toMinutes(timerInMinutes)

	timeoutInSeconds := timeoutInMinutes * 60
	timeOfTimeout := time.Now().Add(time.Minute * time.Duration(timeoutInMinutes)).Format("15:04")
	say.Debug(fmt.Sprintf("Starting break timer at %s for %d minutes = %d seconds (parsed from user input %s)", timeOfTimeout, timeoutInMinutes, timeoutInSeconds, timerInMinutes))

	room := getMobTimerRoom(configuration)
	startRemoteTimer := room != ""
	startLocalTimer := configuration.TimerLocal

	if !startRemoteTimer && !startLocalTimer {
		say.Error("No break timer configured, not starting break timer")
		exit(1)
	}

	if startRemoteTimer {
		timerUser := getUserForMobTimer(configuration.TimerUser)
		err := httpPutBreakTimer(timeoutInMinutes, room, timerUser, configuration.TimerUrl, configuration.TimerInsecure)

		if err != nil {
			say.Error("remote break timer couldn't be started")
			say.Error(err.Error())
			exit(1)
		}
	}

	if startLocalTimer {
		abortRunningTimers()
		err := executeCommandsInBackgroundProcess(getSleepCommand(timeoutInSeconds), getVoiceCommand("mob start", configuration.VoiceCommand), getNotifyCommand("mob start", configuration.NotifyCommand), "echo \"mobTimer\"")

		if err != nil {
			say.Error(fmt.Sprintf("break timer couldn't be started on your system (%s)", runtime.GOOS))
			say.Error(err.Error())
			exit(1)
		}
	}

	say.Info("It's now " + currentTime() + ". " + fmt.Sprintf("%d min break timer ends at approx. %s", timeoutInMinutes, timeOfTimeout) + ". Happy collaborating! :)")
}

func getUserForMobTimer(userOverride string) string {
	if userOverride == "" {
		return git.GitUserName()
	}
	return userOverride
}

func toMinutes(timerInMinutes string) int {
	timeoutInMinutes, _ := strconv.Atoi(timerInMinutes)
	if timeoutInMinutes < 0 {
		timeoutInMinutes = 0
	}
	return timeoutInMinutes
}

func httpPutTimer(timeoutInMinutes int, room string, user string, timerService string, disableSSLVerification bool) error {
	putBody, _ := json.Marshal(map[string]interface{}{
		"timer": timeoutInMinutes,
		"user":  user,
	})
	return sendRequest(putBody, "PUT", timerService+room, disableSSLVerification)
}

func httpPutBreakTimer(timeoutInMinutes int, room string, user string, timerService string, disableSSLVerification bool) error {
	putBody, _ := json.Marshal(map[string]interface{}{
		"breaktimer": timeoutInMinutes,
		"user":       user,
	})
	return sendRequest(putBody, "PUT", timerService+room, disableSSLVerification)
}

func sendRequest(requestBody []byte, requestMethod string, requestUrl string, disableSSLVerification bool) error {
	say.Info(requestMethod + " " + requestUrl + " " + string(requestBody))

	responseBody := bytes.NewBuffer(requestBody)
	request, requestCreationError := http.NewRequest(requestMethod, requestUrl, responseBody)

	httpClient := http.DefaultClient
	if disableSSLVerification {
		transCfg := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient = &http.Client{Transport: transCfg}
	}

	if requestCreationError != nil {
		return fmt.Errorf("failed to create the http request object: %w", requestCreationError)
	}

	request.Header.Set("Content-Type", "application/json")
	response, responseErr := httpClient.Do(request)
	if e, ok := responseErr.(*url.Error); ok {
		switch e.Err.(type) {
		case x509.UnknownAuthorityError:
			say.Error("The timer.mob.sh SSL certificate is signed by an unknown authority!")
			say.Fix("HINT: You can ignore that by adding MOB_TIMER_INSECURE=true to your configuration or environment.",
				"echo MOB_TIMER_INSECURE=true >> ~/.mob")
			return fmt.Errorf("failed, to amke the http request: %w", responseErr)

		default:
			return fmt.Errorf("failed to make the http request: %w", responseErr)

		}
	}

	if responseErr != nil {
		return fmt.Errorf("failed to make the http request: %w", responseErr)
	}
	defer response.Body.Close()
	body, responseReadingErr := ioutil.ReadAll(response.Body)
	if responseReadingErr != nil {
		return fmt.Errorf("failed to read the http response: %w", responseReadingErr)
	}
	if string(body) != "" {
		say.Info(string(body))
	}
	return nil
}

func currentTime() string {
	return time.Now().Format("15:04")
}

func moo(configuration config.Configuration) {
	voiceMessage := "moo"
	err := executeCommandsInBackgroundProcess(getVoiceCommand(voiceMessage, configuration.VoiceCommand))

	if err != nil {
		say.Warning(fmt.Sprintf("can't run voice command on your system (%s)", runtime.GOOS))
		say.Warning(err.Error())
		return
	}

	say.Info(voiceMessage)
}

func start(configuration config.Configuration) error {
	uncommittedChanges := hasUncommittedChanges()
	if uncommittedChanges && !configuration.StartIncludeUncommittedChanges {
		say.Info("cannot start; clean working tree required")
		sayUnstagedChangesInfo()
		sayUntrackedFilesInfo()
		if configuration.StartCreate {
			say.Fix("To start, including uncommitted changes and create the remote branch, use", configuration.Mob("start --create --include-uncommitted-changes"))
		} else {
			say.Fix("To start, including uncommitted changes, use", configuration.Mob("start --include-uncommitted-changes"))
		}
		return errors.New("cannot start; clean working tree required")
	}

	git.Fetch(configuration)
	currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	if !currentBaseBranch.HasRemoteBranch(configuration) && !configuration.StartCreate {
		say.Error("Remote branch " + currentBaseBranch.Remote(configuration).String() + " is missing")
		say.Fix("To start and and create the remote branch", "mob start --create")
		return errors.New("remote branch is missing")
	}

	createRemoteBranch(configuration, currentBaseBranch)

	if currentBaseBranch.HasUnpushedCommits(configuration) {
		say.Error("cannot start; unpushed changes on base branch must be pushed upstream")
		say.Fix("to fix this, push those commits and try again", "git push "+configuration.RemoteName+" "+currentBaseBranch.String())
		return errors.New("cannot start; unpushed changes on base branch must be pushed upstream")
	}

	if uncommittedChanges && git.Silentgit("ls-tree", "-r", "HEAD", "--full-name", "--name-only", ".") == "" {
		say.Error("cannot start; current working dir is an uncommitted subdir")
		say.Fix("to fix this, go to the parent directory and try again", "cd ..")
		return errors.New("cannot start; current working dir is an uncommitted subdir")
	}

	if uncommittedChanges {
		git.Git("stash", "push", "--include-untracked", "--message", configuration.StashName)
		say.Info("uncommitted changes were stashed. If an error occurs later on, you can recover them with 'git stash pop'.")
	}

	if !isMobProgramming(configuration) {
		git.Git("merge", "FETCH_HEAD", "--ff-only")
	}

	if currentWipBranch.HasRemoteBranch(configuration) {
		startJoinMobSession(configuration)
	} else {
		warnForActiveWipBranches(configuration, currentBaseBranch)

		startNewMobSession(configuration)
	}

	if uncommittedChanges && configuration.StartIncludeUncommittedChanges {
		stashes := git.Silentgit("stash", "list")
		stash := findStashByName(stashes, configuration.StashName)
		git.Git("stash", "pop", stash)
	}

	say.Info("you are on wip branch '" + currentWipBranch.String() + "' (base branch '" + currentBaseBranch.String() + "')")
	sayLastCommitsList(currentBaseBranch.String(), currentWipBranch.String())

	openLastModifiedFileIfPresent(configuration)

	return nil // no error
}

func createRemoteBranch(configuration config.Configuration, currentBaseBranch branches.Branch) {
	if !currentBaseBranch.HasRemoteBranch(configuration) && configuration.StartCreate {
		git.Git("push", configuration.RemoteName, currentBaseBranch.String(), "--set-upstream")
	} else if currentBaseBranch.HasRemoteBranch(configuration) && configuration.StartCreate {
		say.Info("Remote branch " + currentBaseBranch.Remote(configuration).String() + " already exists")
	}
}

func openLastModifiedFileIfPresent(configuration config.Configuration) {
	if !configuration.IsOpenCommandGiven() {
		say.Debug("No open command given")
		return
	}

	say.Debug("Try to open last modified file")
	if !lastCommitIsWipCommit(configuration) {
		say.Debug("Last commit isn't a WIP commit.")
		return
	}
	lastCommitMessage := lastCommitMessage()
	split := strings.Split(lastCommitMessage, "lastFile:")
	if len(split) == 1 {
		say.Warning("Couldn't find last modified file in commit message!")
		return
	}
	if len(split) > 2 {
		say.Warning("Could not determine last modified file from commit message, separator was used multiple times!")
		return
	}
	lastModifiedFile := split[1]
	if lastModifiedFile == "" {
		say.Debug("Could not find last modified file in commit message")
		return
	}
	lastModifiedFilePath := git.GitRootDir() + "/" + lastModifiedFile
	commandname, args := openCommandFor(configuration, lastModifiedFilePath)
	_, err := startCommand(commandname, args...)
	if err != nil {
		say.Warning(fmt.Sprintf("Couldn't open last modified file on your system (%s)", runtime.GOOS))
		say.Warning(err.Error())
		return
	}
	say.Debug("Open last modified file: " + lastModifiedFilePath)
}

func warnForActiveWipBranches(configuration config.Configuration, currentBaseBranch branches.Branch) {
	if isMobProgramming(configuration) {
		return
	}

	// TODO show all active wip branches, even non-qualified ones
	existingWipBranches := getWipBranchesForBaseBranch(currentBaseBranch, configuration)
	if len(existingWipBranches) > 0 && configuration.WipBranchQualifier == "" {
		say.Warning("Creating a new wip branch even though preexisting wip branches have been detected.")
		for _, wipBranch := range existingWipBranches {
			say.WithPrefix(wipBranch, "  - ")
		}
	}
}

func showActiveMobSessions(configuration config.Configuration, currentBaseBranch branches.Branch) {
	existingWipBranches := getWipBranchesForBaseBranch(currentBaseBranch, configuration)
	if len(existingWipBranches) > 0 {
		say.Info("remote wip branches detected:")
		for _, wipBranch := range existingWipBranches {
			say.WithPrefix(wipBranch, "  - ")
		}
	}
}

func sayUntrackedFilesInfo() {
	untrackedFiles := getUntrackedFiles()
	hasUntrackedFiles := len(untrackedFiles) > 0
	if hasUntrackedFiles {
		say.Info("untracked files present:")
		say.InfoIndented(untrackedFiles)
	}
}

func sayUnstagedChangesInfo() {
	unstagedChanges := getUnstagedChanges()
	hasUnstagedChanges := len(unstagedChanges) > 0
	if hasUnstagedChanges {
		say.Info("unstaged changes present:")
		say.InfoIndented(unstagedChanges)
	}
}

func getWipBranchesForBaseBranch(currentBaseBranch branches.Branch, configuration config.Configuration) []string {
	remoteBranches := git.GitRemoteBranches()
	say.Debug("check on current base branch " + currentBaseBranch.String() + " with remote branches " + strings.Join(remoteBranches, ","))

	remoteBranchWithQualifier := currentBaseBranch.AddWipPrefix(configuration).AddWipQualifier(configuration).Remote(configuration).Name
	remoteBranchNoQualifier := currentBaseBranch.AddWipPrefix(configuration).Remote(configuration).Name
	if currentBaseBranch.Is("master") {
		// LEGACY
		remoteBranchNoQualifier = "mob-session"
	}

	var result []string
	for _, remoteBranch := range remoteBranches {
		if strings.Contains(remoteBranch, remoteBranchWithQualifier) || strings.Contains(remoteBranch, remoteBranchNoQualifier) {
			result = append(result, remoteBranch)
		}
	}

	return result
}

func startJoinMobSession(configuration config.Configuration) {
	baseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	say.Info("joining existing session from " + currentWipBranch.Remote(configuration).String())
	if git.DoBranchesDiverge(baseBranch.Remote(configuration).Name, currentWipBranch.Name) {
		say.Warning("Careful, your wip branch (" + currentWipBranch.Name + ") diverges from your main branch (" + baseBranch.Remote(configuration).Name + ") !")
	}

	git.Git("checkout", "-B", currentWipBranch.Name, currentWipBranch.Remote(configuration).Name)
	git.Git("branch", "--set-upstream-to="+currentWipBranch.Remote(configuration).Name, currentWipBranch.Name)
}

func startNewMobSession(configuration config.Configuration) {
	currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	say.Info("starting new session from " + currentBaseBranch.Remote(configuration).String())
	git.Git("checkout", "-B", currentWipBranch.Name, currentBaseBranch.Remote(configuration).Name)
	git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), "--set-upstream", configuration.RemoteName, currentWipBranch.Name)
}

func getUntrackedFiles() string {
	return git.Silentgit("ls-files", "--others", "--exclude-standard", "--full-name")
}

func getUnstagedChanges() string {
	return git.Silentgit("diff", "--stat")
}

func findStashByName(stashes string, stash string) string {
	lines := strings.Split(stashes, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, stash) {
			return line[:strings.Index(line, ":")]
		}
	}
	return "unknown"
}

func next(configuration config.Configuration) {
	if !isMobProgramming(configuration) {
		say.Fix("to start working together, use", configuration.Mob("start"))
		return
	}

	if !configuration.HasCustomCommitMessage() && configuration.RequireCommitMessage && hasUncommittedChanges() {
		say.Error("commit message required")
		return
	}

	currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	if isNothingToCommit() {
		if currentWipBranch.HasLocalCommits(configuration) {
			git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), configuration.RemoteName, currentWipBranch.Name)
		} else {
			say.Info("nothing was done, so nothing to commit")
		}
	} else {
		makeWipCommit(configuration)
		git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), configuration.RemoteName, currentWipBranch.Name)
	}
	showNext(configuration)
	abortRunningTimers()

	if !configuration.NextStay {
		git.Git("checkout", currentBaseBranch.Name)
	}
}

func getChangesOfLastCommit() string {
	return git.Silentgit("diff", "HEAD^1", "--stat")
}

func getCachedChanges() string {
	return git.Silentgit("diff", "--cached", "--stat")
}

func makeWipCommit(configuration config.Configuration) {
	git.Git("add", "--all")
	commitMessage := createWipCommitMessage(configuration)
	git.GitWithoutEmptyStrings("commit", "--message", commitMessage, git.GitHooksOption(configuration))
	say.InfoIndented(getChangesOfLastCommit())
	say.InfoIndented(git.GitCommitHash())
}

func createWipCommitMessage(configuration config.Configuration) string {
	commitMessage := configuration.WipCommitMessage

	lastModifiedFilePath := getPathOfLastModifiedFile()
	if lastModifiedFilePath != "" {
		commitMessage += "\n\nlastFile:" + lastModifiedFilePath
	}

	return commitMessage
}

// uses git status --short. To work properly files have to be staged.
func getPathOfLastModifiedFile() string {
	rootDir := git.GitRootDir()
	files := getModifiedFiles(rootDir)
	lastModifiedFilePath := ""
	lastModifiedTime := time.Time{}

	say.Debug("Find last modified file")
	if len(files) == 1 {
		lastModifiedFilePath = files[0]
		say.Debug("Just one modified file: " + lastModifiedFilePath)
		return lastModifiedFilePath
	}

	for _, file := range files {
		absoluteFilepath := rootDir + "/" + file
		say.Debug(absoluteFilepath)
		info, err := os.Stat(absoluteFilepath)
		if err != nil {
			say.Warning("Could not get statistics of file: " + absoluteFilepath)
			say.Warning(err.Error())
			continue
		}
		modTime := info.ModTime()
		if modTime.After(lastModifiedTime) {
			lastModifiedTime = modTime
			lastModifiedFilePath = file
		}
		say.Debug(modTime.String())
	}
	return lastModifiedFilePath
}

// uses git status --short. To work properly files have to be staged.
func getModifiedFiles(rootDir string) []string {
	say.Debug("Find modified files")
	oldWorkingDir := git.WorkingDir
	git.WorkingDir = rootDir
	gitstatus := git.Silentgit("status", "--short")
	git.WorkingDir = oldWorkingDir
	lines := strings.Split(gitstatus, "\n")
	files := []string{}
	for _, line := range lines {
		relativeFilepath := ""
		if strings.HasPrefix(line, "M") {
			relativeFilepath = strings.TrimPrefix(line, "M")
		} else if strings.HasPrefix(line, "A") {
			relativeFilepath = strings.TrimPrefix(line, "A")
		} else {
			continue
		}
		relativeFilepath = strings.TrimSpace(relativeFilepath)
		say.Debug(relativeFilepath)
		files = append(files, relativeFilepath)
	}
	return files
}

func done(configuration config.Configuration) {
	if !isMobProgramming(configuration) {
		say.Fix("to start working together, use", configuration.Mob("start"))
		return
	}

	git.Fetch(configuration)

	baseBranch, wipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	if wipBranch.HasRemoteBranch(configuration) {
		if configuration.DoneSquash == config.SquashWip {
			squashWip(configuration)
		}
		uncommittedChanges := hasUncommittedChanges()
		if uncommittedChanges {
			makeWipCommit(configuration)
		}
		git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), configuration.RemoteName, wipBranch.Name)

		git.Git("checkout", baseBranch.Name)
		git.Git("merge", baseBranch.Remote(configuration).Name, "--ff-only")
		mergeFailed := git.Gitignorefailure("merge", squashOrCommit(configuration), "--ff", wipBranch.Name)

		if mergeFailed != nil {
			// TODO should this be an error and a fix for that error?
			say.Warning("Skipped deleting " + wipBranch.Name + " because of merge conflicts.")
			say.Warning("To fix this, solve the merge conflict manually, commit, push, and afterwards delete " + wipBranch.Name)
			return
		}

		git.Git("branch", "-D", wipBranch.Name)

		if uncommittedChanges && configuration.DoneSquash != config.Squash { // give the user the chance to name their final commit
			git.Git("reset", "--soft", "HEAD^")
		}

		git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), configuration.RemoteName, "--delete", wipBranch.Name)

		cachedChanges := getCachedChanges()
		hasCachedChanges := len(cachedChanges) > 0
		if hasCachedChanges {
			say.InfoIndented(cachedChanges)
		}
		err := appendCoauthorsToSquashMsg(git.GitDir())
		if err != nil {
			say.Warning(err.Error())
		}

		if hasUncommittedChanges() {
			say.Next("To finish, use", "git commit")
		} else if configuration.DoneSquash == config.Squash {
			say.Info("nothing was done, so nothing to commit")
		}

	} else {
		git.Git("checkout", baseBranch.Name)
		git.Git("branch", "-D", wipBranch.Name)
		say.Info("someone else already ended your session")
	}
	abortRunningTimers()
}

func squashOrCommit(configuration config.Configuration) string {
	if configuration.DoneSquash == config.Squash {
		return "--squash"
	} else {
		return "--commit"
	}
}

func status(configuration config.Configuration) {
	if isMobProgramming(configuration) {
		currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)
		say.Info("you are on wip branch " + currentWipBranch.String() + " (base branch " + currentBaseBranch.String() + ")")

		sayLastCommitsList(currentBaseBranch.String(), currentWipBranch.String())
	} else {
		currentBaseBranch, _ := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)
		say.Info("you are on base branch '" + currentBaseBranch.String() + "'")
		showActiveMobSessions(configuration, currentBaseBranch)
	}
}

func sayLastCommitsList(currentBaseBranch string, currentWipBranch string) {
	commitsBaseWipBranch := currentBaseBranch + ".." + currentWipBranch
	log := git.Silentgit("--no-pager", "log", commitsBaseWipBranch, "--pretty=format:%h %cr <%an>", "--abbrev-commit")
	lines := strings.Split(log, "\n")
	if len(lines) > 5 {
		say.Info("wip branch '" + currentWipBranch + "' contains " + strconv.Itoa(len(lines)) + " commits. The last 5 were:")
		lines = lines[:5]
	}
	ReverseSlice(lines)
	output := strings.Join(lines, "\n")
	say.Say(output)
}

func ReverseSlice(s interface{}) {
	size := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, size-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func isNothingToCommit() bool {
	output := git.Silentgit("status", "--short")
	return len(output) == 0
}

func hasUncommittedChanges() bool {
	return !isNothingToCommit()
}

func isMobProgramming(configuration config.Configuration) bool {
	currentBranch := branches.CurrentBranch()
	_, currentWipBranch := branches.DetermineBranches(currentBranch, git.GitBranches(), configuration)
	say.Debug("current branch " + currentBranch.String() + " and currentWipBranch " + currentWipBranch.String())
	return currentWipBranch == currentBranch
}

func showNext(configuration config.Configuration) {
	say.Debug("determining next person based on previous changes")
	gitUserName := git.GitUserName()
	if gitUserName == "" {
		say.Warning("failed to detect who's next because you haven't set your git user name")
		say.Fix("To fix, use", "git config --global user.name \"Your Name Here\"")
		return
	}

	currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)
	commitsBaseWipBranch := currentBaseBranch.String() + ".." + currentWipBranch.String()

	changes := git.Silentgit("--no-pager", "log", commitsBaseWipBranch, "--pretty=format:%an", "--abbrev-commit")
	lines := strings.Split(strings.Replace(changes, "\r\n", "\n", -1), "\n")
	numberOfLines := len(lines)
	say.Debug("there have been " + strconv.Itoa(numberOfLines) + " changes")
	say.Debug("current git user.name is '" + gitUserName + "'")
	if numberOfLines < 1 {
		return
	}
	nextTypist, previousCommitters := findNextTypist(lines, gitUserName)
	if nextTypist != "" {
		if len(previousCommitters) != 0 {
			say.Info("Committers after your last commit: " + strings.Join(previousCommitters, ", "))
		}
		say.Info("***" + nextTypist + "*** is (probably) next.")
	}
}

func version() {
	say.Say("v" + versionNumber)
}

func startCommand(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	if len(workingDir) > 0 {
		command.Dir = workingDir
	}
	commandString := strings.Join(command.Args, " ")
	say.Debug("Starting command " + commandString)
	err := command.Start()
	return commandString, err
}

var exit = func(code int) {
	os.Exit(code)
}
