package bridge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// orgConfigAuth replaces auth.ps1: Set-OrganizationConfig writes the default
// authentication policy it gets, or <null>, to $env:STUB_LOG.
const orgConfigAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = "Mailbox"
function Get-OrganizationConfig { [pscustomobject]@{ Identity = "Contoso"; DisplayName = "Contoso"; DefaultAuthenticationPolicy = "Block Basic" } }
function Set-OrganizationConfig {
    param($DefaultAuthenticationPolicy, $ErrorAction)
    $v = if ($null -eq $DefaultAuthenticationPolicy) { "<null>" } else { [string]$DefaultAuthenticationPolicy }
    Add-Content -LiteralPath $env:STUB_LOG -Value "DefaultAuthenticationPolicy=$v"
}
`

// "" in the configuration clears the default policy: Set-OrganizationConfig
// gets $null, the only value that removes it.
func TestOrganizationConfigClearsDefaultAuthenticationPolicy(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("scripts", "organization_config.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "organization_config.ps1"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(orgConfigAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls.log")
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "organization_config.ps1"),
		"-Action", "Set", "-ClearDefaultAuthenticationPolicy", "true")
	cmd.Env = append(os.Environ(), "STUB_LOG="+log)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("err %v, stderr %q", err, stderr.String())
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "DefaultAuthenticationPolicy=<null>") || !strings.Contains(stdout.String(), `"DefaultAuthenticationPolicy":"Block Basic"`) {
		t.Errorf("calls %q, stdout %q: want Set-OrganizationConfig -DefaultAuthenticationPolicy $null and the policy read back", calls, stdout.String())
	}
}
