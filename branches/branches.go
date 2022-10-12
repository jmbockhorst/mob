package branches

import (
	"fmt"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/git"
	"github.com/remotemobprogramming/mob/v4/say"
	"strconv"
	"strings"
)

type Branch struct {
	Name string
}

func NewBranch(name string) Branch {
	return Branch{
		Name: strings.TrimSpace(name),
	}
}

func (branch Branch) String() string {
	return branch.Name
}

func (branch Branch) Is(branchName string) bool {
	return branch.Name == branchName
}

func (branch Branch) Remote(configuration config.Configuration) Branch {
	return NewBranch(configuration.RemoteName + "/" + branch.Name)
}

func (branch Branch) HasRemoteBranch(configuration config.Configuration) bool {
	remoteBranches := git.GitRemoteBranches()
	remoteBranch := branch.Remote(configuration).Name
	say.Debug("Remote Branches: " + strings.Join(remoteBranches, "\n"))
	say.Debug("Remote Branch: " + remoteBranch)

	for i := 0; i < len(remoteBranches); i++ {
		if remoteBranches[i] == remoteBranch {
			return true
		}
	}

	return false
}

func (branch Branch) IsWipBranch(configuration config.Configuration) bool {
	if branch.Name == "mob-session" {
		return true
	}

	return strings.Index(branch.Name, configuration.WipBranchPrefix) == 0
}

func (branch Branch) AddWipPrefix(configuration config.Configuration) Branch {
	return NewBranch(configuration.WipBranchPrefix + branch.Name)
}

func (branch Branch) AddWipQualifier(configuration config.Configuration) Branch {
	if configuration.CustomWipBranchQualifierConfigured() {
		return NewBranch(addSuffix(branch.Name, configuration.WipBranchQualifierSuffix()))
	}
	return branch
}

func addSuffix(branch string, suffix string) string {
	return branch + suffix
}

func (branch Branch) RemoveWipPrefix(configuration config.Configuration) Branch {
	return NewBranch(removePrefix(branch.Name, configuration.WipBranchPrefix))
}

func removePrefix(branch string, prefix string) string {
	if !strings.HasPrefix(branch, prefix) {
		return branch
	}
	return branch[len(prefix):]
}

func (branch Branch) RemoveWipQualifier(localBranches []string, configuration config.Configuration) Branch {
	for !branch.Exists(localBranches) && branch.hasWipBranchQualifierSeparator(configuration) {
		afterRemoval := branch.removeWipQualifierSuffixOrSeparator(configuration)

		if branch == afterRemoval { // avoids infinite loop
			break
		}

		branch = afterRemoval
	}
	return branch
}

func (branch Branch) removeWipQualifierSuffixOrSeparator(configuration config.Configuration) Branch {
	if !configuration.CustomWipBranchQualifierConfigured() { // WipBranchQualifier not configured
		return branch.removeFromSeparator(configuration.WipBranchQualifierSeparator)
	} else { // WipBranchQualifier not configured
		return branch.removeWipQualifierSuffix(configuration)
	}
}

func (branch Branch) removeFromSeparator(separator string) Branch {
	return NewBranch(branch.Name[:strings.LastIndex(branch.Name, separator)])
}

func (branch Branch) removeWipQualifierSuffix(configuration config.Configuration) Branch {
	if strings.HasSuffix(branch.Name, configuration.WipBranchQualifierSuffix()) {
		return NewBranch(branch.Name[:strings.LastIndex(branch.Name, configuration.WipBranchQualifierSuffix())])
	}
	return branch
}

func (branch Branch) Exists(existingBranches []string) bool {
	return stringContains(existingBranches, branch.Name)
}

func (branch Branch) hasWipBranchQualifierSeparator(configuration config.Configuration) bool { //TODO improve (dont use strings.Contains, add tests)
	return strings.Contains(branch.Name, configuration.WipBranchQualifierSeparator)
}

func (branch Branch) HasLocalCommits(configuration config.Configuration) bool {
	local := git.Silentgit("for-each-ref", "--format=%(objectname)", "refs/heads/"+branch.Name)
	remote := git.Silentgit("for-each-ref", "--format=%(objectname)", "refs/remotes/"+branch.Remote(configuration).Name)
	return local != remote
}

func (branch Branch) HasUnpushedCommits(configuration config.Configuration) bool {
	countOutput := git.Silentgit(
		"rev-list", "--count", "--left-only",
		"refs/heads/"+branch.Name+"..."+"refs/remotes/"+branch.Remote(configuration).Name,
	)
	unpushedCount, err := strconv.Atoi(countOutput)
	if err != nil {
		panic(err)
	}
	unpushedCommits := unpushedCount != 0
	if unpushedCommits {
		say.Info(fmt.Sprintf("there are %d unpushed commits on local base branch <%s>", unpushedCount, branch.Name))
	}
	return unpushedCommits
}

func (branch Branch) IsOrphanWipBranch(configuration config.Configuration) bool {
	return branch.IsWipBranch(configuration) && !branch.HasRemoteBranch(configuration)
}

func stringContains(list []string, element string) bool {
	found := false
	for i := 0; i < len(list); i++ {
		if list[i] == element {
			found = true
		}
	}
	return found
}

func CurrentBranch() Branch {
	return NewBranch(git.CurrentBranch())
}

func DetermineBranches(currentBranch Branch, localBranches []string, configuration config.Configuration) (baseBranch Branch, wipBranch Branch) {
	if currentBranch.Is("mob-session") || (currentBranch.Is("master") && !configuration.CustomWipBranchQualifierConfigured()) {
		// DEPRECATED
		baseBranch = NewBranch("master")
		wipBranch = NewBranch("mob-session")
	} else if currentBranch.IsWipBranch(configuration) {
		baseBranch = currentBranch.RemoveWipPrefix(configuration).RemoveWipQualifier(localBranches, configuration)
		wipBranch = currentBranch
	} else {
		baseBranch = currentBranch
		wipBranch = currentBranch.AddWipPrefix(configuration).AddWipQualifier(configuration)
	}

	say.Debug("on currentBranch " + currentBranch.String() + " => BASE " + baseBranch.String() + " WIP " + wipBranch.String() + " with allLocalBranches " + strings.Join(localBranches, ","))
	if currentBranch != baseBranch && currentBranch != wipBranch {
		// this is unreachable code, but we keep it as a backup
		panic("assertion failed! neither on base nor on wip branch")
	}
	return
}

func GetCurrentWipBranchQualifier(configuration config.Configuration) string {
	currentWipBranchQualifier := configuration.WipBranchQualifier
	if currentWipBranchQualifier == "" {
		currentBranch := CurrentBranch()
		currentBaseBranch, _ := DetermineBranches(currentBranch, git.GitBranches(), configuration)

		if currentBranch.IsWipBranch(configuration) {
			wipBranchWithoutWipPrefix := currentBranch.RemoveWipPrefix(configuration).Name
			currentWipBranchQualifier = removePrefix(removePrefix(wipBranchWithoutWipPrefix, currentBaseBranch.Name), configuration.WipBranchQualifierSeparator)
		}
	}
	return currentWipBranchQualifier
}
