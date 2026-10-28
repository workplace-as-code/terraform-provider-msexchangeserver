package client

import (
	"encoding/json"
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

// TestAuthConnects checks how auth.ps1 opens the Exchange session with each
// authentication method: integrated authentication passes no credential, so
// New-PSSession uses the identity of the process.
func TestAuthConnects(t *testing.T) {
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		if shell, err = exec.LookPath("pwsh"); err != nil {
			t.Skip("neither powershell.exe nor pwsh is installed")
		}
	}
	type result struct {
		Connected *struct {
			ConnectionUri   string
			Authentication  string
			Credential      bool
			PreferredServer string
		} `json:"connected"`
		Error string `json:"error"`
		User  string `json:"user"`
	}
	connect := func(t *testing.T, name string) result {
		t.Helper()
		out, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-File", "testdata/auth_connect.ps1", "-Script", "../../bridge/scripts/auth.ps1", "-Case", name).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return r
	}

	t.Run("password", func(t *testing.T) {
		r := connect(t, "password")
		if r.Error != "" || r.Connected == nil || !r.Connected.Credential || r.Connected.Authentication != "Kerberos" ||
			r.Connected.ConnectionUri != "http://exch01.contoso.local/PowerShell/" {
			t.Errorf("got %+v, error %q", r.Connected, r.Error)
		}
	})
	t.Run("password-nouser", func(t *testing.T) {
		if r := connect(t, "password-nouser"); !strings.Contains(r.Error, "username is required") {
			t.Errorf("error = %q", r.Error)
		}
	})
	t.Run("integrated", func(t *testing.T) {
		r := connect(t, "integrated")
		if r.Error != "" || r.Connected == nil || r.Connected.Credential || r.Connected.Authentication != "Kerberos" {
			t.Errorf("got %+v, error %q: want Kerberos without a credential", r.Connected, r.Error)
		}
		if r.Connected != nil && r.Connected.PreferredServer != "" {
			t.Errorf("session pinned to %q without a domain controller", r.Connected.PreferredServer)
		}
	})
	t.Run("integrated-dc", func(t *testing.T) {
		r := connect(t, "integrated-dc")
		if r.Error != "" || r.Connected == nil || r.Connected.PreferredServer != "dc01.contoso.local" {
			t.Errorf("got %+v, error %q: want the session pinned to dc01.contoso.local", r.Connected, r.Error)
		}
	})
	t.Run("integrated-dc-fails", func(t *testing.T) {
		r := connect(t, "integrated-dc-fails")
		if !strings.Contains(r.Error, "Failed to use the domain controller dc01.contoso.local") || !strings.Contains(r.Error, "suitability") {
			t.Errorf("error = %q", r.Error)
		}
	})
	t.Run("integrated-fails", func(t *testing.T) {
		r := connect(t, "integrated-fails")
		if r.User == "" || !strings.Contains(r.Error, `\`+r.User+":") {
			t.Errorf("error does not name the account %q: %s", r.User, r.Error)
		}
		for _, want := range []string{" as ", "FQDN", "role group", "Kerberos error"} {
			if !strings.Contains(r.Error, want) {
				t.Errorf("error lacks %q: %s", want, r.Error)
			}
		}
	})
	// An Exchange server that is down is not an account problem: no advice
	// about domain membership or role groups.
	t.Run("integrated-503", func(t *testing.T) {
		r := connect(t, "integrated-503")
		if !strings.Contains(r.Error, `\`+r.User+":") || strings.Contains(r.Error, "role group") {
			t.Errorf("error = %q", r.Error)
		}
	})
}
