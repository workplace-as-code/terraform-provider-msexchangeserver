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

// stubAuth replaces auth.ps1: no Exchange session, and a Get-OabVirtualDirectory
// that answers from Active Directory (-ADPropertiesOnly) and from IIS as
// $env:STUB_MODE says.
const stubAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
function Get-OabVirtualDirectory {
    param([string]$Identity, [string]$Server, [switch]$ADPropertiesOnly)
    if ($env:STUB_MODE -eq "absent") {
        throw "The operation couldn't be performed because object 'EX01\OAB (Default Web Site)' couldn't be found."
    }
    $dir = [pscustomobject]@{ Identity = "EX01\OAB (Default Web Site)"; Name = "OAB (Default Web Site)"; BasicAuthentication = $false }
    if ($ADPropertiesOnly) { return $dir }
    if ($env:STUB_MODE -eq "iisdown") { throw "The network path was not found." }
    $dir.BasicAuthentication = $true
    $dir
}
`

// runVirtualDirectory runs virtual_directory.ps1 -Action Get for the OAB of
// EX01 with the stub, and returns its exit code, stdout and stderr.
func runVirtualDirectory(t *testing.T, mode string) (int, string, string) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"virtual_directory.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(stubAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	props := base64.StdEncoding.EncodeToString([]byte(`["BasicAuthentication"]`))
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "virtual_directory.ps1"),
		"-Action", "Get", "-Noun", "OabVirtualDirectory", "-Identity", "EX01", "-Properties", props)
	cmd.Env = append(os.Environ(), "STUB_MODE="+mode)
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

// A server whose IIS does not answer is an error naming the directory and the
// server, even when the message says "not found": it must never pass for a
// directory that does not exist, which would silently drop it from the state.
func TestVirtualDirectoryIISDownIsAnError(t *testing.T) {
	code, stdout, stderr := runVirtualDirectory(t, "iisdown")
	if code == 0 || !strings.Contains(stderr, "from IIS") || !strings.Contains(stderr, "EX01") {
		t.Errorf("exit %d, stdout %q, stderr %q: want an IIS read error naming EX01", code, stdout, stderr)
	}
}

// A directory that does not exist in Active Directory is "gone": empty output, exit 0.
func TestVirtualDirectoryAbsentIsNotFound(t *testing.T) {
	code, stdout, stderr := runVirtualDirectory(t, "absent")
	if code != 0 || strings.TrimSpace(stdout) != "" {
		t.Errorf("exit %d, stdout %q, stderr %q: want empty output and exit 0", code, stdout, stderr)
	}
}

// The read returns the IIS values, not the False that Active Directory alone gives.
func TestVirtualDirectoryReadsIIS(t *testing.T) {
	code, stdout, stderr := runVirtualDirectory(t, "ok")
	if code != 0 || !strings.Contains(stdout, `"BasicAuthentication":true`) {
		t.Errorf("exit %d, stdout %q, stderr %q: want BasicAuthentication true from IIS", code, stdout, stderr)
	}
}
