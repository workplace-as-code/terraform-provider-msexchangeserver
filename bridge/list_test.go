package bridge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// listAuth replaces auth.ps1 for list.ps1: two Mailbox servers and an Edge
// server. Get-TransportService returns the three, as on a Mailbox server
// (EdgeSync keeps a copy of the Edge server in Active Directory);
// Get-PopSettings returns the local server only, as Exchange does.
const listAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = if ($env:STUB_ROLE) { $env:STUB_ROLE } else { "Mailbox" }
$stubServers = @(
    [pscustomobject]@{ Name = "EX01"; IsEdgeServer = $false; IsClientAccessServer = $true },
    [pscustomobject]@{ Name = "EDGE01"; IsEdgeServer = $true; IsClientAccessServer = $false },
    [pscustomobject]@{ Name = "EX02"; IsEdgeServer = $false; IsClientAccessServer = $true }
)
function Get-ExchangeServer { $stubServers }
function Get-TransportService {
    if ($BridgeRole -eq "Edge") { return [pscustomobject]@{ Name = "EDGE01"; Identity = "EDGE01" } }
    $stubServers | ForEach-Object { [pscustomobject]@{ Name = $_.Name; Identity = $_.Name } }
}
function Get-PopSettings { [pscustomobject]@{ Name = "1"; Identity = "EX01\1" } }
function Get-ImapSettings { [pscustomobject]@{ Name = "1"; Identity = "EX01\1" } }
`

// runList runs list.ps1 for noun and property with the stub and returns its
// exit code and output.
func runList(t *testing.T, noun, property, role string) (int, string, string) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"list.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(listAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "list.ps1"),
		"-Noun", noun, "-Property", property)
	cmd.Env = append(os.Environ(), "STUB_ROLE="+role)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, strings.TrimSpace(stdout.String()), stderr.String()
}

// On a Mailbox connection, the Edge server is left out: server_settings.ps1
// refuses it there.
func TestListTransportServiceSkipsEdgeOnMailbox(t *testing.T) {
	code, stdout, stderr := runList(t, "TransportService", "Name", "Mailbox")
	if code != 0 || stdout != `["EX01","EX02"]` {
		t.Errorf("exit %d, stdout %s, stderr %q: want [\"EX01\",\"EX02\"]", code, stdout, stderr)
	}
}

// On an Edge connection, the Edge server lists itself.
func TestListTransportServiceOnEdge(t *testing.T) {
	code, stdout, stderr := runList(t, "TransportService", "Name", "Edge")
	if code != 0 || stdout != `["EDGE01"]` {
		t.Errorf("exit %d, stdout %s, stderr %q: want [\"EDGE01\"]", code, stdout, stderr)
	}
}

// Get-PopSettings reads the local server only: the list names every Client
// Access server instead, without the Edge server.
func TestListPopSettingsNamesTheServers(t *testing.T) {
	for _, noun := range []string{"PopSettings", "ImapSettings"} {
		code, stdout, stderr := runList(t, noun, "Name", "Mailbox")
		if code != 0 || stdout != `["EX01","EX02"]` {
			t.Errorf("%s: exit %d, stdout %s, stderr %q: want [\"EX01\",\"EX02\"]", noun, code, stdout, stderr)
		}
	}
}
