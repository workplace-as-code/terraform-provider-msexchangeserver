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
	// The message Exchange returns for an account without any role.
	noRole := `[FailureCategory=AuthZ-CmdletAccessDeniedException] The user "contoso.local/Service Accounts/svc-terraform" isn't assigned to any management roles.`
	if got := withHint(noRole); !strings.Contains(got, "Add the account to an Exchange role group") || !strings.Contains(got, "integrated authentication") {
		t.Errorf("no-role advice missing: %q", got)
	}
	// A 503 from Exchange, as the WinRM client reports it: the error
	// page read as UTF-16 text. The integrated authentication advice that
	// auth.ps1 used to add does not apply.
	page := "Failed to connect to Exchange Server (mail.contoso.local): Running the Get-Command command in a remote session reported " +
		"the following error: Starting a command on the remote server failed with the following error message : \u0BC0\u2C8C\u01EB " +
		"For more information, see the about_Remote_Troubleshooting Help topic.."
	got = withHint(page)
	if !strings.Contains(got, "error message : HTTP error page. For more information") || !strings.Contains(got, "503 Service Unavailable") {
		t.Errorf("HTTP error page not explained: %q", got)
	}
	if strings.ContainsAny(got, "\u0BC0\u2C8C\u01EB") {
		t.Errorf("unreadable characters kept: %q", got)
	}
	// An empty error page leaves nothing at all.
	empty := "Failed to connect to Exchange Server (mail.contoso.local) as CONTOSO\\svc: [mail.contoso.local] Connecting to remote server mail.contoso.local " +
		"failed with the following error message :  For more information, see the about_Remote_Troubleshooting Help topic.."
	if got := withHint(empty); !strings.Contains(got, "error message : HTTP error page. For more information") || !strings.Contains(got, "503") {
		t.Errorf("empty HTTP error page not explained: %q", got)
	}
	// A readable WinRM message is left alone.
	readable := "Connecting to remote server exch01 failed with the following error message : The WinRM client cannot complete the operation " +
		"within the time specified. For more information, see the about_Remote_Troubleshooting Help topic."
	if got := withHint(readable); strings.Contains(got, "HTTP error page") {
		t.Errorf("readable message taken for an HTTP error page: %q", got)
	}
	if got := withHint("Some other error"); got != "Some other error" {
		t.Errorf("unknown error changed: %q", got)
	}
}
