package reset

import (
	"github.com/remotemobprogramming/mob/v4/branches"
	config "github.com/remotemobprogramming/mob/v4/configuration"
	"github.com/remotemobprogramming/mob/v4/git"
	"github.com/remotemobprogramming/mob/v4/say"
)

func Reset(configuration config.Configuration) {
	if configuration.ResetDeleteRemoteWipBranch {
		deleteRemoteWipBranch(configuration)
	} else {
		say.Fix("Executing this command deletes the mob branch for everyone. If you're sure you want that, use", configuration.Mob("reset --delete-remote-wip-branch"))
	}
}

func deleteRemoteWipBranch(configuration config.Configuration) {
	git.Git("fetch", configuration.RemoteName)

	currentBaseBranch, currentWipBranch := branches.DetermineBranches(branches.CurrentBranch(), git.GitBranches(), configuration)

	git.Git("checkout", currentBaseBranch.String())
	if git.HasLocalBranch(currentWipBranch.String()) {
		git.Git("branch", "--delete", "--force", currentWipBranch.String())
	}
	if currentWipBranch.HasRemoteBranch(configuration) {
		git.GitWithoutEmptyStrings("push", git.GitHooksOption(configuration), configuration.RemoteName, "--delete", currentWipBranch.String())
	}
	say.Info("Branches " + currentWipBranch.String() + " and " + currentWipBranch.Remote(configuration).String() + " deleted")
}
