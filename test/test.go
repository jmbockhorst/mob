package test

import (
	"fmt"
	"github.com/remotemobprogramming/mob/v4/branches"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/git"
	"github.com/remotemobprogramming/mob/v4/say"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

var (
	workingDir string
	TempDir    string
)

const (
	AWAIT_DEFAULT_POLL_INTERVAL = 100 * time.Millisecond
	AWAIT_DEFAULT_AT_MOST       = 2 * time.Second
)

func Equals(t *testing.T, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		t.Log(string(debug.Stack()))
		failWithFailure(t, exp, act)
	}
}

func failWithFailure(t *testing.T, exp interface{}, act interface{}) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
	t.FailNow()
}

func failWithFailureMessage(t *testing.T, message string) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Printf("\033[31m%s:%d:\n\n\t%s", filepath.Base(file), line, message)
	t.FailNow()
}

func CreateFile(t *testing.T, filename string, content string) (pathToFile string) {
	contentAsBytes := []byte(content)
	pathToFile = workingDir + "/" + filename
	err := ioutil.WriteFile(pathToFile, contentAsBytes, 0644)
	if err != nil {
		failWithFailure(t, "creating file "+filename+" with content "+content, "error")
	}
	return
}

func SetWorkingDir(dir string) {
	workingDir = dir
	git.WorkingDir = dir
	say.Say("\n===== cd " + dir)
}

func CaptureOutput(t *testing.T) *string {
	messages := ""
	say.PrintToConsole = func(text string) {
		t.Log(strings.TrimRight(text, "\n"))
		messages += text
	}
	return &messages
}

func AssertOutputContains(t *testing.T, output *string, contains string) {
	currentOutput := *output
	if !strings.Contains(currentOutput, contains) {
		failWithFailure(t, "output contains '"+contains+"'", currentOutput)
	}
}

func AssertOutputNotContains(t *testing.T, output *string, notContains string) {
	if strings.Contains(*output, notContains) {
		failWithFailure(t, "output not contains "+notContains, output)
	}
}

func Await(t *testing.T, until func() bool, awaitedState string) {
	AwaitBlocking(t, AWAIT_DEFAULT_POLL_INTERVAL, AWAIT_DEFAULT_AT_MOST, until, awaitedState)
}

func AwaitBlocking(t *testing.T, pollInterval time.Duration, atMost time.Duration, until func() bool, awaitedState string) {
	if pollInterval <= 0 {
		failWithFailureMessage(t, fmt.Sprintf("PollInterval cannot be 0 or below, got: %d", pollInterval))
		return
	}
	if atMost <= 0 {
		failWithFailureMessage(t, fmt.Sprintf("AtMost timeout cannot be 0 or below, got: %d", atMost))
		return
	}
	if pollInterval > atMost {
		failWithFailureMessage(t, fmt.Sprintf("PollInterval must be smaller than atMost timeout, got: pollInterval=%d, atMost=%d", pollInterval, atMost))
		return
	}
	startTime := time.Now()
	timeLeft := atMost

	for {
		if until() {
			return
		} else {
			timeLeft = atMost - time.Now().Sub(startTime)
			if timeLeft <= 0 {
				stackTrace := string(debug.Stack())
				failWithFailureMessage(t, fmt.Sprintf("expected '%s' to occur within %s but did not: %s", awaitedState, atMost, stackTrace))
				return
			}
		}
		time.Sleep(pollInterval)
	}
}

type GitStatus = map[string]string

func GetGitStatus() GitStatus {
	shortStatus := git.Silentgit("status", "--short")
	statusLines := strings.Split(shortStatus, "\n")
	var statusMap = make(GitStatus)
	for _, line := range statusLines {
		if len(line) == 0 {
			continue
		}
		file := strings.Fields(line)
		statusMap[file[1]] = file[0]
	}
	return statusMap
}

func AssertCleanGitStatus(t *testing.T) {
	status := GetGitStatus()
	if len(status) != 0 {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texpected a clean git status, but contained %s\"\n", filepath.Base(file), line, status)
		t.FailNow()
	}
}

func Setup(t *testing.T) (output *string, configuration config.Configuration) {
	configuration = config.GetDefaultConfiguration()
	configuration.NextStay = false
	output = CaptureOutput(t)
	createTestbed(t, configuration)
	AssertOnBranch(t, "master")
	Equals(t, []string{"master"}, git.GitBranches())
	Equals(t, []string{"origin/master"}, git.GitRemoteBranches())
	AssertNoMobSessionBranches(t, configuration, "mob-session")
	return output, configuration
}

func createTestbed(t *testing.T, configuration config.Configuration) {
	SetWorkingDir("")
	TempDir = t.TempDir()
	say.Say("Creating testbed in temporary directory " + TempDir)
	createTestbedIn(t, TempDir)
	SetWorkingDir(TempDir + "/local")
	AssertOnBranch(t, "master")
	AssertNoMobSessionBranches(t, configuration, "mob-session")
}

func AssertOnBranch(t *testing.T, branch string) {
	currentBranch := branches.CurrentBranch()
	if currentBranch.Name != branch {
		failWithFailure(t, "on branch "+branch, "on branch "+currentBranch.String())
	}
}

func AssertNoMobSessionBranches(t *testing.T, configuration config.Configuration, branch string) {
	if branches.NewBranch(branch).HasRemoteBranch(configuration) {
		failWithFailure(t, "none", branches.NewBranch(branch).Remote(configuration).Name)
	}
	if git.HasLocalBranch(branch) {
		failWithFailure(t, "none", branch)
	}
}

func createTestbedIn(t *testing.T, temporaryDirectory string) {
	say.Debug("Creating temporary test assets in " + temporaryDirectory)
	err := os.MkdirAll(temporaryDirectory, 0755)
	if err != nil {
		say.Error("Could not create temporary dir " + temporaryDirectory)
		say.Error(err.Error())
		return
	}
	say.Debug("Create remote repository")
	remoteDirectory := getRemoteDirectory(temporaryDirectory)
	cleanRepository(remoteDirectory)
	createRemoteRepository(remoteDirectory)

	say.Debug("Create first local repository")
	localDirectory := getLocalDirectory(temporaryDirectory)
	cleanRepository(localDirectory)
	cloneRepository(localDirectory, remoteDirectory)

	say.Debug("Populate, initial import and push")
	SetWorkingDir(localDirectory)
	createFile(t, "test.txt", "test")
	createDirectory(t, "subdir")
	createFileInPath(t, localDirectory+"/subdir", "subdir.txt", "subdir")
	git.Git("checkout", "-b", "master")
	git.Git("add", ".")
	git.Git("commit", "-m", "\"initial import\"")
	git.Git("push", "--set-upstream", "--all", "origin")

	for _, name := range [3]string{"localother", "alice", "bob"} {
		cleanRepository(temporaryDirectory + "/" + name)
		cloneRepository(temporaryDirectory+"/"+name, remoteDirectory)
		say.Debug("Created local repository " + name)
	}

	notGitDirectory := getNotGitDirectory(temporaryDirectory)
	err = os.MkdirAll(notGitDirectory, 0755)
	if err != nil {
		say.Error("Count not create directory " + notGitDirectory)
		say.Error(err.Error())
		return
	}

	say.Debug("Creating local repository with .git symlink")
	symlinkDirectory := getSymlinkDirectory(temporaryDirectory)
	symlinkGitDirectory := getSymlinkGitDirectory(temporaryDirectory)
	cleanRepositoryWithSymlink(symlinkDirectory, symlinkGitDirectory)
	cloneRepositoryWithSymlink(symlinkDirectory, symlinkGitDirectory, remoteDirectory)
	say.Debug("Done.")
}

func cleanRepository(path string) {
	say.Debug("cleanrepository: Delete " + path)
	err := os.RemoveAll(path)
	if err != nil {
		say.Error("Could not remove directory " + path)
		say.Error(err.Error())
		return
	}
}

func createRemoteRepository(path string) {
	branch := "master" // fixed to master for now
	say.Debug("createremoterepository: Creating remote repository " + path)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		say.Error("Could not create directory " + path)
		say.Error(err.Error())
		return
	}
	SetWorkingDir(path)
	say.Debug("before git init")
	git.Git("--bare", "init")
	say.Debug("before symbolic-ref")
	git.Git("symbolic-ref", "HEAD", "refs/heads/"+branch)
	say.Debug("finished")
}

func cloneRepository(path, remoteDirectory string) {
	say.Debug("clonerepository: Cloning remote " + remoteDirectory + " to " + path)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		say.Error("Could not create directory " + path)
		say.Error(err.Error())
		return
	}
	SetWorkingDir(path)
	name := basename(path)
	git.Git("clone", "--origin", "origin", "file://"+remoteDirectory, ".")
	git.Git("config", "--local", "user.name", name)
	git.Git("config", "--local", "user.email", name+"@example.com")
}

func cloneRepositoryWithSymlink(path, gitDirectory, remoteDirectory string) {
	cloneRepository(path, remoteDirectory)
	say.Debug(fmt.Sprintf("clonerepositorywithsymlink: move .git to %s and create symlink to it", gitDirectory))
	err := os.Rename(filepath.FromSlash(path+"/.git"), gitDirectory)
	if err != nil {
		say.Error("Could not move directory " + path + " to " + gitDirectory)
		say.Error(err.Error())
		return
	}
	err = os.Symlink(gitDirectory, filepath.FromSlash(path+"/.git"))
	if err != nil {
		say.Error("Could not create smylink from " + gitDirectory + " to " + path + "/.git")
		say.Error(err.Error())
		return
	}
}

func cleanRepositoryWithSymlink(path, gitDirectory string) {
	cleanRepository(path)
	say.Debug("cleanrepositorywithsymlink: Delete " + gitDirectory)
	err := os.RemoveAll(gitDirectory)
	if err != nil {
		say.Error("Could not remove directory " + gitDirectory)
		say.Error(err.Error())
		return
	}
}

func basename(path string) string {
	split := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	return split[len(split)-1]
}

func getRemoteDirectory(path string) string {
	return path + "/remote"
}

func getLocalDirectory(path string) string {
	return path + "/local"
}

func getNotGitDirectory(path string) string {
	return path + "/notgit"
}

func getSymlinkGitDirectory(path string) string {
	return path + "/local-symlink.git"
}

func getSymlinkDirectory(path string) string {
	return path + "/local-symlink"
}

func createFile(t *testing.T, filename string, content string) (pathToFile string) {
	return createFileInPath(t, workingDir, filename, content)
}

func createFileInPath(t *testing.T, path, filename, content string) (pathToFile string) {
	contentAsBytes := []byte(content)
	pathToFile = path + "/" + filename
	err := ioutil.WriteFile(pathToFile, contentAsBytes, 0644)
	if err != nil {
		failWithFailure(t, "creating file "+filename+" with content "+content, "error")
	}
	return
}

func createDirectory(t *testing.T, directory string) (pathToFile string) {
	return createDirectoryInPath(t, workingDir, directory)
}

func createDirectoryInPath(t *testing.T, path, directory string) (pathToFile string) {
	pathToFile = path + "/" + directory
	err := os.Mkdir(pathToFile, 0755)
	if err != nil {
		failWithFailure(t, "creating directory "+pathToFile, "error")
	}
	return
}
