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

// journalAuth replaces auth.ps1: the journal rule cmdlets write each call and
// its parameters to the file $env:STUB_LOG.
const journalAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = "Mailbox"
$BridgeNoConfirm = @{ Confirm = $false }
function Write-StubCall([string]$Line) { Add-Content -LiteralPath $env:STUB_LOG -Value $Line }
function New-JournalRule {
    param([string]$Name, $JournalEmailAddress, $Recipient, $Scope, $Enabled)
    Write-StubCall "New-JournalRule Enabled=$Enabled Scope=$Scope"
}
function Set-JournalRule {
    param([string]$Identity, $JournalEmailAddress, $Recipient, $Scope)
    Write-StubCall "Set-JournalRule Scope=$Scope"
}
function Enable-JournalRule { param([string]$Identity, $Confirm) Write-StubCall "Enable-JournalRule Confirm=$Confirm" }
function Disable-JournalRule { param([string]$Identity, $Confirm) Write-StubCall "Disable-JournalRule Confirm=$Confirm" }
function Get-JournalRule {
    param([string]$Identity)
    [pscustomobject]@{ Name = $Identity; Scope = "Global"; Enabled = $false; JournalEmailAddress = "journal@contoso.com" }
}
`

// runJournalRule runs journal_rule.ps1 with the stub and returns the calls it made.
func runJournalRule(t *testing.T, action, values string) (string, string) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"journal_rule.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(journalAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls.log")
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "journal_rule.ps1"),
		"-Action", action, "-Noun", "JournalRule", "-Identity", "TF-CI-Journal",
		"-Values", enc(values), "-Properties", enc(`["Scope","Enabled"]`))
	cmd.Env = append(os.Environ(), "STUB_LOG="+log)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s failed: %v, stderr %q", action, err, stderr.String())
	}
	calls, _ := os.ReadFile(log)
	return string(calls), stdout.String()
}

// A rule created disabled never journals a message: New-JournalRule gets
// -Enabled $false, the rule is not created enabled then disabled.
func TestJournalRuleCreatedDisabledSendsEnabledFalse(t *testing.T) {
	calls, _ := runJournalRule(t, "Create", `{"JournalEmailAddress":"journal@contoso.com","Scope":"Global","Enabled":false}`)
	if !strings.Contains(calls, "New-JournalRule Enabled=False") || strings.Contains(calls, "able-JournalRule") {
		t.Errorf("calls:\n%s\nwant New-JournalRule with Enabled False and no Enable-/Disable-JournalRule", calls)
	}
}

// Enabled is changed with Enable-/Disable-JournalRule, without a confirmation
// prompt, and never passed to Set-JournalRule.
func TestJournalRuleUpdateTogglesWithEnableDisable(t *testing.T) {
	calls, _ := runJournalRule(t, "Update", `{"Scope":"External","Enabled":true}`)
	if !strings.Contains(calls, "Set-JournalRule Scope=External") || !strings.Contains(calls, "Enable-JournalRule Confirm=False") {
		t.Errorf("calls:\n%s\nwant Set-JournalRule for the scope, then Enable-JournalRule -Confirm:$false", calls)
	}
}
