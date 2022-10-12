package git_test

import (
	"github.com/remotemobprogramming/mob/v4/git"
	"github.com/remotemobprogramming/mob/v4/test"
	"testing"
)

func TestIsGitIdentifiesGitRepo(t *testing.T) {
	test.Setup(t)
	test.Equals(t, true, git.IsGitRepository())
}

func TestIsGitIdentifiesOutsideOfGitRepo(t *testing.T) {
	test.Setup(t)
	test.SetWorkingDir(test.TempDir + "/notgit")
	test.Equals(t, false, git.IsGitRepository())
}

func TestEmptyGitStatus(t *testing.T) {
	test.Setup(t)

	status := test.GetGitStatus()

	test.Equals(t, 0, len(status))
	test.AssertCleanGitStatus(t)
}

func TestGitStatusWithOneFile(t *testing.T) {
	test.Setup(t)
	test.CreateFile(t, "hello.txt", "contentIrrelevant")

	status := test.GetGitStatus()

	test.Equals(t, test.GitStatus{
		"hello.txt": "??",
	}, status)
}

func TestGitStatusWithManyFiles(t *testing.T) {
	test.Setup(t)
	test.CreateFile(t, "hello.txt", "contentIrrelevant")
	test.CreateFile(t, "added.txt", "contentIrrelevant")
	git.Git("add", "added.txt")

	status := test.GetGitStatus()

	test.Equals(t, test.GitStatus{
		"added.txt": "A",
		"hello.txt": "??",
	}, status)
}
