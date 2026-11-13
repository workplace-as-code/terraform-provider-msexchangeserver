package bridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// addressBookAuth replaces auth.ps1: the address book cmdlets write each call
// and its bound parameters to $env:STUB_LOG, and return objects shaped as
// Exchange returns them (references as identities, canonical names).
const addressBookAuth = `param([string]$Server = "", [string]$Username = "", [string]$Password = "")
$BridgeRole = "Mailbox"
$BridgeNoConfirm = @{ Confirm = $false }
function Write-StubCall([string]$Verb, $Bound) {
    $parts = foreach ($k in ($Bound.Keys | Sort-Object)) {
        if ($k -eq "Identity" -or $k -eq "Name") { continue }
        $v = $Bound[$k]
        if ($null -eq $v) { "$k=<null>" } else { "$k=" + (@($v) -join ",") }
    }
    Add-Content -LiteralPath $env:STUB_LOG -Value ("$Verb " + ($parts -join " "))
}
function Get-AddressBookPolicy { param([string]$Identity)
    [pscustomobject]@{ Name = $Identity; GlobalAddressList = "\TF-GAL"; OfflineAddressBook = "\TF-OAB";
        RoomList = "\Rooms"; AddressLists = @("\A", "\Sites\B") }
}
function Get-AddressList { param([string]$Identity)
    $id = if ($Identity -eq "Paris") { "\Sites\Paris" } else { "\$Identity" }
    [pscustomobject]@{ Name = $Identity; Identity = $id; Container = "\"; RecipientContainer = $null;
        RecipientFilter = "((Company -eq 'Contoso'))"; DisplayName = $Identity }
}
function Set-AddressList { param([string]$Identity, $RecipientContainer, $RecipientFilter, $DisplayName)
    Write-StubCall "Set-AddressList" $PSBoundParameters }
function New-AddressList { param([string]$Name, $Container, $RecipientContainer, $RecipientFilter, $DisplayName)
    Write-StubCall "New-AddressList" $PSBoundParameters }
function Get-OfflineAddressBook { param([string]$Identity)
    [pscustomobject]@{ Name = $Identity; IsDefault = ($env:STUB_OAB_DEFAULT -eq "1");
        GeneratingMailbox = "contoso.com/Users/SystemMailbox{1f05a927-0000}"; GlobalWebDistributionEnabled = $false;
        VirtualDirectories = @("EX01\OAB (Default Web Site)"); AddressLists = @("\Default Global Address List") }
}
function Set-OfflineAddressBook { param([string]$Identity, $VirtualDirectories, $GlobalWebDistributionEnabled,
    $GeneratingMailbox, $AddressLists, $IsDefault)
    Write-StubCall "Set-OfflineAddressBook" $PSBoundParameters }
function Remove-OfflineAddressBook { param([string]$Identity, $Confirm)
    Write-StubCall "Remove-OfflineAddressBook" $PSBoundParameters }
`

// runAddressBook runs address_book.ps1 with the stub and returns the calls it
// made, its output and its error output; ok is false when it exited non-zero.
func runAddressBook(t *testing.T, action, noun, identity, values string, props []string, env ...string) (calls, stdout, stderr string, ok bool) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	dir := t.TempDir()
	for _, name := range []string{"address_book.ps1", "common.ps1"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.ps1"), []byte(addressBookAuth), 0o600); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls.log")
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	propsJSON, _ := json.Marshal(props)
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-File", filepath.Join(dir, "address_book.ps1"),
		"-Action", action, "-Noun", noun, "-Identity", identity, "-Values", enc(values), "-Properties", enc(string(propsJSON)))
	cmd.Env = append(append(os.Environ(), "STUB_LOG="+log), env...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	runErr := cmd.Run()
	data, _ := os.ReadFile(log)
	return string(data), out.String(), errOut.String(), runErr == nil
}

func decodeOutput(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("output %q: %v", stdout, err)
	}
	return got
}

// References to GALs, OABs and address lists go back as the names the
// configuration writes, not as identities.
func TestAddressBookReturnsLeafNames(t *testing.T) {
	_, out, errOut, ok := runAddressBook(t, "Get", "AddressBookPolicy", "TF-ABP", "",
		[]string{"GlobalAddressList", "OfflineAddressBook", "RoomList", "AddressLists"})
	if !ok {
		t.Fatalf("Get failed: %s", errOut)
	}
	got := decodeOutput(t, out)
	if got["GlobalAddressList"] != "TF-GAL" || got["OfflineAddressBook"] != "TF-OAB" || got["RoomList"] != "Rooms" {
		t.Errorf("references = %v, want leaf names", got)
	}
	lists, _ := got["AddressLists"].([]any)
	if len(lists) != 2 || lists[0] != "A" || lists[1] != "B" {
		t.Errorf("AddressLists = %v, want [A B]", got["AddressLists"])
	}
}

// The container of an address list is its parent list, from its identity.
func TestAddressListContainerFromIdentity(t *testing.T) {
	for identity, want := range map[string]string{"Paris": "Sites", "Top": ""} {
		_, out, errOut, ok := runAddressBook(t, "Get", "AddressList", identity, "", []string{"Container", "RecipientContainer"})
		if !ok {
			t.Fatalf("Get failed: %s", errOut)
		}
		if got := decodeOutput(t, out)["Container"]; got != want {
			t.Errorf("%s: Container = %v, want %q", identity, got, want)
		}
	}
}

// The generating mailbox goes back as the mailbox name, without the canonical
// path Exchange returns.
func TestOabGeneratingMailboxLeaf(t *testing.T) {
	_, out, errOut, ok := runAddressBook(t, "Get", "OfflineAddressBook", "TF-OAB", "",
		[]string{"GeneratingMailbox", "VirtualDirectories", "AddressLists"})
	if !ok {
		t.Fatalf("Get failed: %s", errOut)
	}
	got := decodeOutput(t, out)
	if got["GeneratingMailbox"] != "SystemMailbox{1f05a927-0000}" {
		t.Errorf("GeneratingMailbox = %v", got["GeneratingMailbox"])
	}
	// One value comes back as a string, as from object.ps1: the provider reads both.
	if vdirs := got["VirtualDirectories"]; vdirs != `EX01\OAB (Default Web Site)` {
		t.Errorf("VirtualDirectories = %v", got["VirtualDirectories"])
	}
}

// Remove-OfflineAddressBook on the default OAB crashes Exchange (Watson dump):
// the script refuses before calling it.
func TestOabDeleteRefusesDefault(t *testing.T) {
	calls, _, errOut, ok := runAddressBook(t, "Delete", "OfflineAddressBook", "TF-OAB", "", nil, "STUB_OAB_DEFAULT=1")
	if ok {
		t.Fatal("deleting the default OAB succeeded")
	}
	if !strings.Contains(errOut, "default offline address book") || strings.Contains(calls, "Remove-OfflineAddressBook") {
		t.Errorf("stderr %q, calls %q: want a refusal without Remove-OfflineAddressBook", errOut, calls)
	}
	calls, _, errOut, ok = runAddressBook(t, "Delete", "OfflineAddressBook", "TF-OAB", "", nil)
	if !ok || !strings.Contains(calls, "Remove-OfflineAddressBook Confirm=False") {
		t.Errorf("delete of another OAB: ok %v, calls %q, stderr %q", ok, calls, errOut)
	}
}

// Exchange refuses web distribution while virtual directories are set: they
// are cleared first, in a call of their own.
func TestOabWebOnClearsVirtualDirectoriesFirst(t *testing.T) {
	calls, _, errOut, ok := runAddressBook(t, "Update", "OfflineAddressBook", "TF-OAB",
		`{"GlobalWebDistributionEnabled":true}`, []string{"GlobalWebDistributionEnabled"})
	if !ok {
		t.Fatalf("Update failed: %s", errOut)
	}
	want := "Set-OfflineAddressBook VirtualDirectories=<null>\nSet-OfflineAddressBook GlobalWebDistributionEnabled=True\n"
	if calls != want {
		t.Errorf("calls:\n%s\nwant:\n%s", calls, want)
	}
}

// Exchange refuses "" for a container, a room list or a mailbox: an empty
// value is sent as $null, which clears it.
func TestAddressBookEmptyClears(t *testing.T) {
	calls, _, errOut, ok := runAddressBook(t, "Update", "AddressList", "Top", `{"RecipientContainer":""}`,
		[]string{"RecipientContainer"})
	if !ok || strings.TrimSpace(calls) != "Set-AddressList RecipientContainer=<null>" {
		t.Errorf("ok %v, calls %q, stderr %q", ok, calls, errOut)
	}
}

// A nested list gets its container on creation only.
func TestAddressListCreateWithContainer(t *testing.T) {
	calls, _, errOut, ok := runAddressBook(t, "Create", "AddressList", "Paris",
		`{"Container":"Sites","RecipientFilter":"Company -eq 'Contoso'"}`, []string{"Container"})
	if !ok || strings.TrimSpace(calls) != "New-AddressList Container=Sites RecipientFilter=Company -eq 'Contoso'" {
		t.Errorf("ok %v, calls %q, stderr %q", ok, calls, errOut)
	}
}
