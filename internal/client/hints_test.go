package client

import (
	"strings"
	"testing"
)

func TestWithHint(t *testing.T) {
	got := withHint("||The specified rule priority is invalid.")
	if strings.HasPrefix(got, "|") {
		t.Errorf("leading pipes not trimmed: %q", got)
	}
	kerberos := "Connecting to remote server exch01 failed with the following error message : WinRM cannot process the request. " +
		"The following error occurred while using Kerberos authentication: Cannot find the computer exch01."
	if got := withHint(kerberos); !strings.Contains(got, "joined to the Active Directory domain") {
		t.Errorf("Kerberos advice missing: %q", got)
	}
	if got := withHint("The term 'New-AcceptedDomain' is not recognized as a name of a cmdlet"); !strings.Contains(got, "management roles") {
		t.Errorf("RBAC advice missing: %q", got)
	}
	// With integrated authentication there is no password to check: the
	// advice must also name the account Terraform runs as.
	if got := withHint("Access is denied"); !strings.Contains(got, "with integrated authentication") {
		t.Errorf("access advice ignores integrated authentication: %q", got)
	}
	// The message Exchange returned on the lab for an account without any role.
	noRole := `[FailureCategory=AuthZ-CmdletAccessDeniedException] The user "ad.local/Service Accounts/svc-gitlab-runner" isn't assigned to any management roles.`
	if got := withHint(noRole); !strings.Contains(got, "Add the account to an Exchange role group") || !strings.Contains(got, "integrated authentication") {
		t.Errorf("no-role advice missing: %q", got)
	}
	if got := withHint("Some other error"); got != "Some other error" {
		t.Errorf("unknown error changed: %q", got)
	}
}
