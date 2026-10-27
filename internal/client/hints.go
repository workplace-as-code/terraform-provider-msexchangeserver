package client

import "strings"

// hints maps fragments of a known Exchange error message (all must appear) to
// advice that tells the user how to fix it.
var hints = []struct {
	fragments []string
	advice    string
}{
	{
		[]string{"WinRM", "Kerberos"},
		"Kerberos authentication to the Exchange server failed. Run Terraform on a machine joined to the Active Directory domain, " +
			"and set server to the FQDN of the Exchange server as registered in Active Directory: an IP address, a short name " +
			"or a load balancer name does not work with Kerberos.",
	},
	{
		[]string{"Access is denied"},
		"The account cannot open an Exchange remote PowerShell session. Check its password (with integrated authentication, " +
			"check which account Terraform runs as: the CI runner service or the scheduled task), that it is a member of the " +
			"Organization Management role group (or equivalent), and that remote PowerShell is enabled for it " +
			"(Get-User <account> | Format-List RemotePowerShellEnabled).",
	},
	{
		[]string{"is not recognized as"},
		"Exchange remote PowerShell only exposes the cmdlets that the account's management roles allow: give the account " +
			"the role that contains this cmdlet, or the Organization Management role group.",
	},
}

// withHint trims the noise PowerShell adds to some messages and appends the
// advice for known errors.
func withHint(msg string) string {
	msg = strings.TrimLeft(msg, "| ")
	for _, h := range hints {
		if containsAll(msg, h.fragments) {
			return msg + "\n\n" + h.advice
		}
	}
	return msg
}

func containsAll(s string, fragments []string) bool {
	for _, f := range fragments {
		if !strings.Contains(s, f) {
			return false
		}
	}
	return true
}
