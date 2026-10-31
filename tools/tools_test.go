// Package tools holds helper scripts for administrators. The tests keep them
// consistent with the bridge scripts embedded in the provider.
package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const roleGroupScript = "new_role_group.ps1"

var (
	cmdletRe    = regexp.MustCompile(`\b(?:Get|Set|New|Remove|Enable|Disable)-[A-Z][A-Za-z]+`)
	functionRe  = regexp.MustCompile(`(?m)^\s*function\s+([A-Za-z]+-[A-Za-z]+)`)
	listRe      = regexp.MustCompile(`(?s)\$ProviderCmdlets = @\((.*?)\n\)`)
	rolesRe     = regexp.MustCompile(`(?s)\$ProviderRoles = @\((.*?)\n\)`)
	retentionRe = regexp.MustCompile(`(?s)\$RetentionCmdlets = @\((.*?)\n\)`)
	// guideRoleRe matches the role list of the authentication guide: one
	// "- **Role**" item per role, alone on its line.
	guideRoleRe = regexp.MustCompile(`(?m)^- \*\*([^*]+)\*\*$`)
	quotedRe    = regexp.MustCompile(`"([^"]+)"`)
)

var (
	specNounRe      = regexp.MustCompile(`noun:\s*"([A-Za-z]+)"`)
	specSingletonRe = regexp.MustCompile(`singleton:\s*true`)
)

// specCmdlets returns the cmdlets the generic resources (internal/resources/specs.go)
// call through object.ps1, which builds the names from the noun.
func specCmdlets(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../internal/resources/specs.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, block := range strings.Split(string(data), "\n\t{\n")[1:] {
		m := specNounRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		verbs := []string{"Get", "New", "Set", "Remove"}
		if specSingletonRe.MatchString(block) {
			verbs = []string{"Get", "Set"}
		}
		for _, v := range verbs {
			out = append(out, v+"-"+m[1])
		}
	}
	if len(out) == 0 {
		t.Fatal("no noun found in specs.go")
	}
	return out
}

// builtins are PowerShell helpers used by the bridge scripts, not Exchange
// cmdlets, so they need no management role. Set-CASMailbox and
// Set-ExternalInOutlook only appear in comments.
var builtins = map[string]bool{
	"Get-Command": true, "Get-Item": true, "Get-PSSession": true, "Get-PSSnapin": true, "Get-TypeData": true,
	"New-Object": true, "New-PSSession": true, "New-PSSessionOption": true, "Remove-PSSession": true,
	"Remove-TypeData": true, "Set-StrictMode": true, "Set-CASMailbox": true, "Set-ExternalInOutlook": true,
}

func bridgeCmdlets(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../bridge/scripts/*.ps1")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bridge scripts found: %v", err)
	}
	found := map[string]bool{}
	helpers := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range functionRe.FindAllStringSubmatch(string(data), -1) {
			helpers[m[1]] = true
		}
		for _, c := range cmdletRe.FindAllString(string(data), -1) {
			found[c] = true
		}
	}
	for _, c := range specCmdlets(t) {
		found[c] = true
	}
	var out []string
	for c := range found {
		if !builtins[c] && !helpers[c] {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// quotedList returns the quoted items of the PowerShell list that re matches
// in the role group script.
func quotedList(t *testing.T, re *regexp.Regexp, name string) []string {
	t.Helper()
	data, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	block := re.FindStringSubmatch(string(data))
	if block == nil {
		t.Fatalf("%s list not found", name)
	}
	var out []string
	for _, m := range quotedRe.FindAllStringSubmatch(block[1], -1) {
		out = append(out, m[1])
	}
	return out
}

func TestRoleGroupScriptListsEveryBridgeCmdlet(t *testing.T) {
	listed := quotedList(t, listRe, "$ProviderCmdlets")
	sort.Strings(listed)
	want := bridgeCmdlets(t)
	if strings.Join(listed, ",") != strings.Join(want, ",") {
		t.Errorf("$ProviderCmdlets in %s is out of date.\nlisted: %v\nbridge: %v", roleGroupScript, listed, want)
	}
}

func TestFindUncoveredCmdlets(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	script := `
$env:MSEXCHANGESERVER_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + `
$byCmdlet = @{
  "Get-TransportRule" = @("Transport Rules", "Organization Management")
  "Get-Mailbox"       = @("Mail Recipient Creation", "Retention Management")
  "New-Mailbox"       = @("Mail Recipient Creation")
}
$missing = Find-UncoveredCmdlets -RolesByCmdlet $byCmdlet -Roles @("Transport Rules", "Retention Management")
$none = Find-UncoveredCmdlets -RolesByCmdlet $byCmdlet -Roles @("Transport Rules", "Mail Recipient Creation")
@{ missing = @($missing); none = @($none) } | ConvertTo-Json -Compress
`
	out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("pwsh: %v\n%s", err, out)
	}
	var res struct {
		Missing []string `json:"missing"`
		None    []string `json:"none"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if strings.Join(res.Missing, ",") != "New-Mailbox" || len(res.None) != 0 {
		t.Errorf("got %+v, want only New-Mailbox uncovered, then nothing", res)
	}
}

// TestGuideListsTheProviderRoles keeps the role list of the authentication
// guide in line with $ProviderRoles, so that the docs say exactly what the
// role group grants.
func TestGuideListsTheProviderRoles(t *testing.T) {
	roles := quotedList(t, rolesRe, "$ProviderRoles")
	guide, err := os.ReadFile(filepath.Join("..", "templates", "guides", "authentication.md"))
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, m := range guideRoleRe.FindAllStringSubmatch(string(guide), -1) {
		documented = append(documented, m[1])
	}
	if strings.Join(documented, ",") != strings.Join(roles, ",") {
		t.Errorf("the authentication guide lists %v, the script grants %v", documented, roles)
	}
}

// TestRetentionRoleKeepsTheRetentionCmdlets checks the custom retention role:
// it keeps exactly the retention cmdlets of the bridge, Get-Mailbox and
// Set-ADServerSettings, so a new retention cmdlet in a script cannot be
// forgotten.
func TestRetentionRoleKeepsTheRetentionCmdlets(t *testing.T) {
	kept := quotedList(t, retentionRe, "$RetentionCmdlets")
	want := []string{"Get-Mailbox", "Set-ADServerSettings"}
	for _, c := range bridgeCmdlets(t) {
		if strings.Contains(c, "-Retention") {
			want = append(want, c)
		}
	}
	sort.Strings(kept)
	sort.Strings(want)
	if strings.Join(kept, ",") != strings.Join(want, ",") {
		t.Errorf("$RetentionCmdlets is %v, the bridge runs %v", kept, want)
	}
}
