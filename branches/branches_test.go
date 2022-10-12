package branches_test

import (
	"github.com/remotemobprogramming/mob/v4/branches"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/test"
	"testing"
)

func TestDetermineBranches(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	configuration.WipBranchQualifierSeparator = "-"

	assertDetermineBranches(t, "master", "", []string{}, "master", "mob-session")
	assertDetermineBranches(t, "mob-session", "", []string{}, "master", "mob-session")
	assertDetermineBranches(t, "mob-session", "green", []string{}, "master", "mob-session")

	assertDetermineBranches(t, "master", "green", []string{}, "master", "mob/master-green")
	assertDetermineBranches(t, "mob/master-green", "", []string{}, "master", "mob/master-green")

	assertDetermineBranches(t, "master", "test-branch", []string{}, "master", "mob/master-test-branch")
	assertDetermineBranches(t, "mob/master-test-branch", "", []string{}, "master", "mob/master-test-branch")

	assertDetermineBranches(t, "feature1", "", []string{}, "feature1", "mob/feature1")
	assertDetermineBranches(t, "mob/feature1", "", []string{}, "feature1", "mob/feature1")
	assertDetermineBranches(t, "mob/feature1-green", "", []string{}, "feature1", "mob/feature1-green")
	assertDetermineBranches(t, "feature1", "green", []string{}, "feature1", "mob/feature1-green")

	assertDetermineBranches(t, "feature/test", "", []string{"feature/test"}, "feature/test", "mob/feature/test")
	assertDetermineBranches(t, "mob/feature/test", "", []string{"feature/test", "mob/feature/test"}, "feature/test", "mob/feature/test")

	assertDetermineBranches(t, "feature/test-ch", "", []string{"DPL-2638-update-apis", "DPL-2814-create-project", "feature/test-ch", "fix/smallChanges", "master", "pipeship/pipelineupdate-pipeship-pipeline.yaml"}, "feature/test-ch", "mob/feature/test-ch")
}

func assertDetermineBranches(t *testing.T, branch string, qualifier string, branchList []string, expectedBase string, expectedWip string) {
	configuration := config.GetDefaultConfiguration()
	configuration.WipBranchQualifier = qualifier
	baseBranch, wipBranch := branches.DetermineBranches(branches.NewBranch(branch), branchList, configuration)
	test.Equals(t, branches.NewBranch(expectedBase), baseBranch)
	test.Equals(t, branches.NewBranch(expectedWip), wipBranch)
}

func TestRemoveWipPrefix(t *testing.T) {
	configuration := config.GetDefaultConfiguration()
	configuration.WipBranchPrefix = "mob/"
	test.Equals(t, "master-green", branches.NewBranch("mob/master-green").RemoveWipPrefix(configuration).Name)
	test.Equals(t, "master-green-blue", branches.NewBranch("mob/master-green-blue").RemoveWipPrefix(configuration).Name)
	test.Equals(t, "main-branch", branches.NewBranch("mob/main-branch").RemoveWipPrefix(configuration).Name)
}

func TestRemoveWipBranchQualifier(t *testing.T) {
	var configuration config.Configuration

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = "green"
	test.Equals(t, "master", branches.NewBranch("master-green").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = "test-branch"
	test.Equals(t, "master", branches.NewBranch("master-test-branch").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = "branch"
	test.Equals(t, "master-test", branches.NewBranch("master-test-branch").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = "branch"
	test.Equals(t, "master-test", branches.NewBranch("master-test-branch").RemoveWipQualifier([]string{"master-test"}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "/-/"
	configuration.WipBranchQualifier = "branch-qualifier"
	test.Equals(t, "main", branches.NewBranch("main/-/branch-qualifier").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = "branchqualifier"
	test.Equals(t, "main/branchqualifier", branches.NewBranch("main/branchqualifier").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = ""
	configuration.WipBranchQualifier = "branchqualifier"
	test.Equals(t, "main", branches.NewBranch("mainbranchqualifier").RemoveWipQualifier([]string{}, configuration).Name)
}

func TestRemoveWipBranchQualifierWithoutBranchQualifierSet(t *testing.T) {
	var configuration config.Configuration

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = ""
	test.Equals(t, "main", branches.NewBranch("main").RemoveWipQualifier([]string{}, configuration).Name)

	configuration.WipBranchQualifierSeparator = "-"
	configuration.WipBranchQualifier = ""
	test.Equals(t, "master", branches.NewBranch("master-test-branch").RemoveWipQualifier([]string{}, configuration).Name)
}
