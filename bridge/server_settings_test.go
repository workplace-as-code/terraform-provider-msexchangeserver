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

// serverSettingsAuth replaces auth.ps1: no Exchange session, two Mailbox
// servers (EX01, EX02) and an Edge server (EDGE01) known to Get-ExchangeServer.
// Each stub declares only the parameters of the real cmdlet: Get-PopSettings
// has -Server and no -Identity, so a call with -Identity fails.
const serverSettingsAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = if ($env:STUB_ROLE) { $env:STUB_ROLE } else { "Mailbox" }
$servers = @{
    "EX01" = [pscustomobject]@{ Name = "EX01"; Fqdn = "ex01.contoso.com"; IsEdgeServer = $false; IsClientAccessServer = $true }
    "EX02" = [pscustomobject]@{ Name = "EX02"; Fqdn = "ex02.contoso.com"; IsEdgeServer = $false; IsClientAccessServer = $true }
    "EDGE01" = [pscustomobject]@{ Name = "EDGE01"; Fqdn = "edge01.contoso.com"; IsEdgeServer = $true; IsClientAccessServer = $false }
}
function Find-StubServer([string]$Name) {
    $short = ($Name -split '\.')[0].ToUpper()
    if ($servers.ContainsKey($short)) { return $servers[$short] }
    throw "The operation couldn't be performed because object '$Name' couldn't be found on 'DC01.contoso.com'."
}
function Get-ExchangeServer {
    param([string]$Identity)
    if ($Identity) { return Find-StubServer $Identity }
    $servers.Values | Sort-Object Name
}
function Get-TransportService {
    param([string]$Identity)
    $s = Find-StubServer $Identity
    [pscustomobject]@{ Name = $s.Name; Identity = $s.Name; ConnectivityLogMaxAge = "30.00:00:00" }
}
function Get-PopSettings {
    param([string]$Server)
    $s = Find-StubServer $Server
    if ($s.IsEdgeServer) { throw "Exchange server ""$Server"" was not found. Please make sure you typed the name correctly." }
    [pscustomobject]@{ Name = "1"; Identity = "$($s.Name)\1"; X509CertificateName = "mail.contoso.com" }
}
`

// runServerSettings runs server_settings.ps1 -Action Get with the stub and
// returns its exit code, stdout and stderr.
func runServerSettings(t *testing.T, noun, identity, role, props string) (int, string, string) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"server_settings.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(serverSettingsAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "server_settings.ps1"),
		"-Action", "Get", "-Noun", noun, "-Identity", identity,
		"-Properties", base64.StdEncoding.EncodeToString([]byte(props)))
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
	return code, stdout.String(), stderr.String()
}

// The name comes back as it was given: a server given by FQDN must not show a
// change to its short name in every plan.
func TestServerSettingsKeepsTheGivenName(t *testing.T) {
	code, stdout, stderr := runServerSettings(t, "TransportService", "ex01.contoso.com", "Mailbox", `["ConnectivityLogMaxAge"]`)
	if code != 0 || !strings.Contains(stdout, `"Name":"ex01.contoso.com"`) || !strings.Contains(stdout, `"ConnectivityLogMaxAge":"30.00:00:00"`) {
		t.Errorf("exit %d, stdout %q, stderr %q: want the name as given and the max age", code, stdout, stderr)
	}
}

// An unknown server is an error, never a missing object: the resource would
// silently leave the state.
func TestServerSettingsUnknownServerIsAnError(t *testing.T) {
	for _, noun := range []string{"TransportService", "PopSettings"} {
		code, stdout, stderr := runServerSettings(t, noun, "EX09", "Mailbox", `[]`)
		if code == 0 || !strings.Contains(stderr, "EX09") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q: want an error naming EX09", noun, code, stdout, stderr)
		}
	}
}

// The same unknown server on an Edge connection, where Get-ExchangeServer is
// not used: still an error.
func TestServerSettingsUnknownEdgeServerIsAnError(t *testing.T) {
	code, stdout, stderr := runServerSettings(t, "TransportService", "EDGE09", "Edge", `[]`)
	if code == 0 || !strings.Contains(stderr, "EDGE09") {
		t.Errorf("exit %d, stdout %q, stderr %q: want an error naming EDGE09", code, stdout, stderr)
	}
}

// Get-PopSettings takes -Server: the read works and returns the settings.
func TestServerSettingsPopUsesServer(t *testing.T) {
	code, stdout, stderr := runServerSettings(t, "PopSettings", "EX02", "Mailbox", `["X509CertificateName"]`)
	if code != 0 || !strings.Contains(stdout, `"X509CertificateName":"mail.contoso.com"`) || !strings.Contains(stdout, `"Name":"EX02"`) {
		t.Errorf("exit %d, stdout %q, stderr %q: want the POP settings of EX02", code, stdout, stderr)
	}
}

// On a Mailbox connection, an Edge server's settings are the EdgeSync copy in
// Active Directory, not the Edge's own: refused, with the way to manage them.
func TestServerSettingsRefusesEdgeOnMailbox(t *testing.T) {
	code, stdout, stderr := runServerSettings(t, "TransportService", "EDGE01", "Mailbox", `[]`)
	if code == 0 || !strings.Contains(stderr, "Edge Transport server") {
		t.Errorf("exit %d, stdout %q, stderr %q: want a refusal naming the Edge Transport server", code, stdout, stderr)
	}
}

// On an Edge connection, the Edge server itself is read by -Identity.
func TestServerSettingsReadsEdgeOnEdge(t *testing.T) {
	code, stdout, stderr := runServerSettings(t, "TransportService", "EDGE01", "Edge", `["ConnectivityLogMaxAge"]`)
	if code != 0 || !strings.Contains(stdout, `"Name":"EDGE01"`) {
		t.Errorf("exit %d, stdout %q, stderr %q: want the transport service of EDGE01", code, stdout, stderr)
	}
}
