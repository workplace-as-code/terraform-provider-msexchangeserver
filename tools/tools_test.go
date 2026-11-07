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
	cmdletRe   = regexp.MustCompile(`\b(?:Get|Set|New|Remove|Enable|Disable|Import)-[A-Z][A-Za-z]+`)
	functionRe = regexp.MustCompile(`(?m)^\s*function\s+([A-Za-z]+-[A-Za-z]+)`)
	listRe     = regexp.MustCompile(`(?s)\$ProviderCmdlets = @\((.*?)\n\)`)
	rolesRe    = regexp.MustCompile(`(?s)\$ProviderRoles = @\((.*?)\n\)`)
	// customRoleRe matches one entry of $CustomRoles: name, parent, cmdlets.
	customRoleRe = regexp.MustCompile(`(?s)@\{ Name = "([^"]+)"; Parent = "([^"]+)"; Cmdlets = @\((.*?)\) \}`)
	// guideRoleRe matches the role list of the authentication guide: one
	// "- **Role**" item per role, alone on its line.
	guideRoleRe = regexp.MustCompile(`(?m)^- \*\*([^*]+)\*\*$`)
	quotedRe    = regexp.MustCompile(`"([^"]+)"`)
)

var (
	specNounRe      = regexp.MustCompile(`noun:\s*"([A-Za-z]+)"`)
	specSingletonRe = regexp.MustCompile(`singleton:\s*true`)
	specAdoptedRe   = regexp.MustCompile(`adopted:\s*true`)
	// Edge-only specs: Edge has no RBAC, so their cmdlets are not in the role group.
	specEdgeOnlyRe = regexp.MustCompile(`edgeOnly:\s*true`)
	// ipListEntrySpec(typeName, noun, ...) builds an Edge-only spec.
	specIPListEntryRe = regexp.MustCompile(`ipListEntrySpec\("[a-z_]+", "([A-Za-z]+)"`)
	// virtualDirectorySpec(typeName, noun, ...) builds an adopted spec.
	specVirtualDirRe = regexp.MustCompile(`virtualDirectorySpec\("[a-z_]+", "([A-Za-z]+)"`)
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
	for _, block := range specBlocks(data) {
		m := specNounRe.FindStringSubmatch(block)
		if m == nil || specEdgeOnlyRe.MatchString(block) {
			continue
		}
		verbs := []string{"Get", "New", "Set", "Remove"}
		if specSingletonRe.MatchString(block) || specAdoptedRe.MatchString(block) {
			verbs = []string{"Get", "Set"}
		}
		for _, v := range verbs {
			out = append(out, v+"-"+m[1])
		}
	}
	for _, m := range specVirtualDirRe.FindAllStringSubmatch(string(data), -1) {
		out = append(out, "Get-"+m[1], "Set-"+m[1])
	}
	if len(out) == 0 {
		t.Fatal("no noun found in specs.go")
	}
	return out
}

// specBlocks splits specs.go into the text of each spec literal of
// objectSpecs, from its opening brace to its closing one.
func specBlocks(data []byte) []string {
	var out []string
	for _, block := range strings.Split(string(data), "\n\t{\n")[1:] {
		if end := strings.Index(block, "\n\t},\n"); end >= 0 {
			block = block[:end]
		}
		out = append(out, block)
	}
	return out
}

// edgeOnlyNouns returns the nouns of the Edge-only specs, whose cmdlets the
// bridge scripts run without a management role.
func edgeOnlyNouns(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile("../internal/resources/specs.go")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, block := range specBlocks(data) {
		if m := specNounRe.FindStringSubmatch(block); m != nil && specEdgeOnlyRe.MatchString(block) {
			out[m[1]] = true
		}
	}
	for _, m := range specIPListEntryRe.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	return out
}

// builtins are PowerShell helpers used by the bridge scripts, not Exchange
// cmdlets, so they need no management role. Set-CASMailbox and
// Set-ExternalInOutlook only appear in comments.
var builtins = map[string]bool{
	"Get-Command": true, "Get-Item": true, "Import-Module": true, "Import-PSSession": true, "Get-PSSession": true, "Get-PSSnapin": true, "Get-TypeData": true,
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
	edgeOnly := edgeOnlyNouns(t)
	var out []string
	for c := range found {
		if !builtins[c] && !helpers[c] && !edgeOnly[c[strings.Index(c, "-")+1:]] {
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

func TestCustomRoleNames(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	run := func(args string) (string, error) {
		script := `
$env:MSEXCHANGESERVER_ROLEGROUP_NO_RUN = "1"
. ./` + roleGroupScript + ` ` + args + `
($CustomRoles | ForEach-Object { "$($_.Name)=$($_.Role)" }) -join "|"
`
		out, err := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	out, err := run(`-RoleGroupName RG-TF`)
	if err != nil || !strings.Contains(out, "Certificates=RG-TF Certificates") {
		t.Errorf("default names: %v %s", err, out)
	}
	out, err = run(`-CustomRolePrefix CR-TF- -CustomRoleNames @{ Certificates = "CR-CERT" }`)
	if err != nil || !strings.Contains(out, "Certificates=CR-CERT") || !strings.Contains(out, "Databases=CR-TF-Databases") {
		t.Errorf("prefix and names: %v %s", err, out)
	}
	if out, err = run(`-CustomRoleNames @{ Certificate = "CR-CERT" }`); err == nil {
		t.Errorf("an unknown custom role name must fail: %s", out)
	}
	if out, err = run(`-CustomRoleNames @{ Certificates = "CR-X"; Databases = "CR-X" }`); err == nil {
		t.Errorf("two custom roles with the same name must fail: %s", out)
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

// TestCustomRolesKeepTheBridgeCmdlets checks each custom role of $CustomRoles:
// for every noun of its list that the bridge runs, the role keeps exactly the
// cmdlets the bridge runs on that noun. A new cmdlet in a script cannot be
// forgotten, and the role never keeps more than needed. Nouns the bridge does
// not run yet are skipped: the roles of a release are granted before its
// resources are written.
func TestCustomRolesKeepTheBridgeCmdlets(t *testing.T) {
	script, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	roles := customRoleRe.FindAllStringSubmatch(string(script), -1)
	if len(roles) == 0 {
		t.Fatal("$CustomRoles entries not found")
	}
	bridgeByNoun := map[string][]string{}
	for _, c := range bridgeCmdlets(t) {
		noun := c[strings.Index(c, "-")+1:]
		bridgeByNoun[noun] = append(bridgeByNoun[noun], c)
	}
	// Together, the custom roles keep every cmdlet the bridge runs on a noun
	// they hold: a role may keep part of them when its parent built-in role
	// has only those (Exchange Servers has Set-MapiVirtualDirectory, for
	// -IISAuthenticationMethods, but not Get-MapiVirtualDirectory).
	keptByNoun := map[string]map[string]bool{}
	for _, role := range roles {
		for _, m := range quotedRe.FindAllStringSubmatch(role[3], -1) {
			noun := m[1][strings.Index(m[1], "-")+1:]
			if keptByNoun[noun] == nil {
				keptByNoun[noun] = map[string]bool{}
			}
			keptByNoun[noun][m[1]] = true
		}
	}
	for noun, keptSet := range keptByNoun {
		want, ok := bridgeByNoun[noun]
		if !ok {
			continue
		}
		var kept []string
		for c := range keptSet {
			kept = append(kept, c)
		}
		sort.Strings(kept)
		if strings.Join(kept, ",") != strings.Join(want, ",") {
			t.Errorf("the custom roles keep %v, the bridge runs %v", kept, want)
		}
	}
}

// Set-MapiVirtualDirectory -IISAuthenticationMethods comes with the built-in
// Exchange Servers role, not Exchange Virtual Directories: the custom role
// derived from Exchange Servers must hold the MAPI cmdlets too (RBAC adds up
// the parameters a cmdlet gets from each role).
func TestMapiAuthenticationFromExchangeServers(t *testing.T) {
	script, err := os.ReadFile(roleGroupScript)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range customRoleRe.FindAllStringSubmatch(string(script), -1) {
		if role[1] != "Client Access" {
			continue
		}
		if !strings.Contains(role[3], `"Set-MapiVirtualDirectory"`) {
			t.Errorf("custom role Client Access lacks Set-MapiVirtualDirectory: %s", role[3])
		}
		return
	}
	t.Fatal("custom role Client Access not found")
}
