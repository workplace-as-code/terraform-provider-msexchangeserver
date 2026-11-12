package bridge

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// objectAuth replaces auth.ps1 for object.ps1: Set-AdminAuditLogConfig fails
// as Exchange does in the non-interactive bridge when it would ask whether to
// delete the entries older than a lower age limit, unless -Force is given.
const objectAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = "Mailbox"
function Get-AdminAuditLogConfig { [pscustomobject]@{ Name = "Admin Audit Log Settings"; AdminAuditLogAgeLimit = "90.00:00:00" } }
function Set-AdminAuditLogConfig {
    param($AdminAuditLogAgeLimit, [switch]$Force)
    if (-not $Force) { throw "Windows PowerShell is in NonInteractive mode. Read and Prompt functionality is not available." }
}
`

// Lowering the age limit of the admin audit log asks for a confirmation that
// -Confirm:$false does not answer: object.ps1 sends -Force.
func TestAdminAuditLogConfigUpdateSendsForce(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"object.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(objectAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "object.ps1"),
		"-Action", "Update", "-Noun", "AdminAuditLogConfig",
		"-Values", enc(`{"AdminAuditLogAgeLimit":"30.00:00:00"}`), "-Properties", enc(`["AdminAuditLogAgeLimit"]`))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil || !strings.Contains(stdout.String(), "AdminAuditLogAgeLimit") {
		t.Errorf("err %v, stdout %q, stderr %q: want the update to succeed with -Force", err, stdout.String(), stderr.String())
	}
}
