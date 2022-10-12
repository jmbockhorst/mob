package reset_test

import (
	"github.com/remotemobprogramming/mob/v4/reset"
	"github.com/remotemobprogramming/mob/v4/test"
	"testing"
)

func TestReset(t *testing.T) {
	output, configuration := test.Setup(t)

	reset.Reset(configuration)

	test.AssertOutputContains(t, output, "mob reset --delete-remote-wip-branch")
}

func TestResetDeleteRemoteWipBranch(t *testing.T) {
	_, configuration := test.Setup(t)
	configuration.ResetDeleteRemoteWipBranch = true

	reset.Reset(configuration)

	test.AssertOnBranch(t, "master")
	test.AssertNoMobSessionBranches(t, configuration, "mob-session")
}
