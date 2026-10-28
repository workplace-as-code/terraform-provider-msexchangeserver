package client

import (
	"regexp"
	"strings"
)

// httpErrorPage matches what the WinRM client reports when Exchange answers
// with an HTTP error page instead of a remote PowerShell response (WinRM error
// 0x80338043, "the XML is invalid"): it reads the page as UTF-16 text, which
// leaves no readable text after "error message :", only a few unreadable
// characters or nothing at all.
var httpErrorPage = regexp.MustCompile(`(following error message ?:)[^\x21-\x7E]*(For more information)`)

// httpErrorPageAdvice explains the HTTP error page, a 503 in practice.
const httpErrorPageAdvice = "Exchange answered with an HTTP error page (usually 503 Service Unavailable) instead of remote PowerShell: " +
	"the Exchange server that runs the commands for this account is unavailable. Check MSExchangePowerShellAppPool and the " +
	"Exchange services on the Exchange servers."

// hints maps fragments of a known Exchange error message (all must appear) to
// advice that tells the user how to fix it.
var hints = []struct {
	fragments []string
	advice    string
}{
	{
		[]string{"WinRM", "Kerberos"},
		"Kerberos authentication to the Exchange server failed. Run Terraform on a machine joined to the Active Directory domain, " +
			"and set server to the FQDN of the Exchange server as registered in Active Directory: an IP address or a short name " +
			"does not work with Kerberos, and a DNS alias or load balancer name needs an alternate service account (ASA).",
	},
	{
		[]string{"Access is denied"},
		"The account cannot open an Exchange remote PowerShell session. Check its password (with integrated authentication, " +
			"check which account Terraform runs as: the CI runner service or the scheduled task), that it is a member of the " +
			"Organization Management role group (or equivalent), and that remote PowerShell is enabled for it " +
			"(Get-User <account> | Format-List RemotePowerShellEnabled).",
	},
	{
		[]string{"isn't assigned to any management roles"},
		"Exchange opened the session but the account has no Exchange permission. Add the account to an Exchange role group " +
			"(Organization Management, or one with the roles listed in the Permissions section of each resource). With integrated " +
			"authentication, the account is the one Terraform runs as: the CI runner service or the scheduled task.",
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
	if httpErrorPage.MatchString(msg) {
		return httpErrorPage.ReplaceAllString(msg, "$1 HTTP error page. $2") + "\n\n" + httpErrorPageAdvice
	}
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
