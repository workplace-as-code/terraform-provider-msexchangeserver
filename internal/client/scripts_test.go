package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTransportRuleClearsOnlyWhatTheRuleUses runs the parameter builder of
// transport_rule.ps1 against simulated Edge and Mailbox rules: clearing a
// condition the server does not have fails the whole update.
func TestTransportRuleClearsOnlyWhatTheRuleUses(t *testing.T) {
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		if shell, err = exec.LookPath("pwsh"); err != nil {
			t.Skip("neither powershell.exe nor pwsh is installed")
		}
	}
	out, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", "testdata/rule_params.ps1", "-Script", "../../bridge/scripts/transport_rule.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(string(out), "PASS") || strings.Contains(string(out), "FAIL") {
		t.Errorf("transport_rule.ps1 parameters:\n%s", out)
	}
}

// rawIdentity matches an Identity property copied as is into a script's JSON
// output, such as `Identity = $Domain.Identity`.
var rawIdentity = regexp.MustCompile(`(?m)^\s*Identity\s*=\s*\$\w+\.Identity\s*$`)

// TestScriptsReturnIdentityAsString guards the local Edge mode: the snap-in
// returns live Exchange objects, whose Identity (an ADObjectId) ConvertTo-Json
// writes as an object. Remote sessions return it as a string, which hid it.
func TestScriptsReturnIdentityAsString(t *testing.T) {
	scripts, err := filepath.Glob("../../bridge/scripts/*.ps1")
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no bridge script found: %v", err)
	}
	for _, path := range scripts {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range rawIdentity.FindAllString(string(data), -1) {
			t.Errorf("%s: %q must be converted with [string]", filepath.Base(path), strings.TrimSpace(line))
		}
	}
}

// TestAuthLeaksNoVariable guards the scope of auth.ps1, which every bridge
// script dot-sources: a variable it assigns there overwrites the script's
// parameter of the same name. $name once replaced the -Name of
// transport_rule.ps1, which created a rule named after the Exchange snap-in.
func TestAuthLeaksNoVariable(t *testing.T) {
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		if shell, err = exec.LookPath("pwsh"); err != nil {
			t.Skip("neither powershell.exe nor pwsh is installed")
		}
	}
	out, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", "testdata/auth_scope.ps1", "-Script", "../../bridge/scripts/auth.ps1").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	allowed := map[string]bool{"BridgeNoConfirm": true, "BridgeRole": true, "ErrorActionPreference": true}
	for _, name := range strings.Fields(string(out)) {
		if !allowed[name] {
			t.Errorf("auth.ps1 assigns $%s in the scope of the bridge scripts: move it into the connection block", name)
		}
	}
}
