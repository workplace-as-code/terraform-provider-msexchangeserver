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
	if got := withHint("Some other error"); got != "Some other error" {
		t.Errorf("unknown error changed: %q", got)
	}
}
