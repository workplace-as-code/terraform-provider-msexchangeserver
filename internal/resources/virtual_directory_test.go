package resources

import (
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// specByType returns the generic spec of a resource type.
func specByType(t *testing.T, typeName string) objectSpec {
	t.Helper()
	for _, s := range objectSpecs {
		if s.typeName == typeName {
			return s
		}
	}
	t.Fatalf("no spec %s", typeName)
	return objectSpec{}
}

// TestVirtualDirectoryAuthFields pins the authentication attributes of each
// virtual directory to the parameters its Set- cmdlet accepts on Exchange
// Server (checked on Exchange SE), and the read-only ones.
func TestVirtualDirectoryAuthFields(t *testing.T) {
	urls := []string{"internal_url", "external_url"}
	readOnly := []string{"extended_protection_token_checking", "internal_authentication_methods",
		"external_authentication_methods"}
	formsDirs := []string{"basic_authentication", "windows_authentication", "digest_authentication",
		"forms_authentication", "oauth_authentication", "adfs_authentication"}
	want := map[string]struct{ writable, readOnly []string }{
		"owa_virtual_directory": {formsDirs, readOnly},
		"ecp_virtual_directory": {formsDirs, readOnly},
		"web_services_virtual_directory": {[]string{"basic_authentication", "windows_authentication",
			"digest_authentication", "oauth_authentication", "ws_security_authentication"}, readOnly},
		"activesync_virtual_directory": {[]string{"basic_auth_enabled", "windows_auth_enabled", "client_cert_auth"},
			[]string{"extended_protection_token_checking"}},
		"oab_virtual_directory": {[]string{"basic_authentication", "windows_authentication", "oauth_authentication",
			"require_ssl"}, readOnly},
		"mapi_virtual_directory": {[]string{"iis_authentication_methods"}, readOnly},
	}
	for typeName, w := range want {
		var gotW, gotR []string
		for _, f := range specByType(t, typeName).fields {
			if f.readOnly {
				gotR = append(gotR, f.name)
			} else {
				gotW = append(gotW, f.name)
			}
		}
		if g, e := sorted(gotW), sorted(append(append([]string{}, urls...), w.writable...)); g != e {
			t.Errorf("%s writable = %s, want %s", typeName, g, e)
		}
		if g, e := sorted(gotR), sorted(w.readOnly); g != e {
			t.Errorf("%s read-only = %s, want %s", typeName, g, e)
		}
	}
}

// The spec notes say where the authentication settings come from, not that
// they are unmanaged (v0.4.0).
func TestVirtualDirectoryNotesMentionIIS(t *testing.T) {
	s := specByType(t, "owa_virtual_directory")
	if strings.Contains(s.notes, "not managed") || !strings.Contains(s.notes, "IIS") {
		t.Errorf("notes = %q", s.notes)
	}
}

func sorted(s []string) string {
	c := append([]string{}, s...)
	sort.Strings(c)
	return strings.Join(c, ",")
}

// Outlook Anywhere (the /Rpc directory) is adopted per server like the
// virtual directories, through the same script.
func TestOutlookAnywhereSpec(t *testing.T) {
	s := specByType(t, "outlook_anywhere")
	if s.noun != "OutlookAnywhere" || !s.adopted || s.script != "virtual_directory.ps1" {
		t.Errorf("noun=%s adopted=%v script=%s", s.noun, s.adopted, s.script)
	}
	kinds := map[string]fieldKind{}
	for _, f := range s.fields {
		if f.readOnly {
			t.Errorf("%s: unexpected read-only attribute", f.name)
		}
		kinds[f.name] = f.kind
	}
	want := map[string]fieldKind{
		"internal_hostname": stringField, "external_hostname": stringField,
		"internal_client_authentication_method": enumField, "external_client_authentication_method": enumField,
		"iis_authentication_methods":   setField,
		"internal_clients_require_ssl": boolField, "external_clients_require_ssl": boolField, "ssl_offloading": boolField,
	}
	if len(kinds) != len(want) {
		t.Errorf("attributes = %v, want %v", kinds, want)
	}
	for name, k := range want {
		if got, ok := kinds[name]; !ok || got != k {
			t.Errorf("%s: kind %v (present %v), want %v", name, got, ok, k)
		}
	}
}

// Turning forms authentication on also sets Basic and turns Digest and Windows
// off (Set-OwaVirtualDirectory): the unconfigured members of the group are
// planned unknown, or apply would end in "inconsistent result".
func TestCoupledFieldsUnknown(t *testing.T) {
	spec := specByType(t, "owa_virtual_directory")
	state := map[string]attr.Value{
		"basic_authentication": types.BoolValue(true), "windows_authentication": types.BoolValue(true),
		"digest_authentication": types.BoolValue(false), "forms_authentication": types.BoolValue(false),
		"oauth_authentication": types.BoolValue(false),
	}
	plan := map[string]attr.Value{}
	for k, v := range state {
		plan[k] = v
	}
	plan["forms_authentication"] = types.BoolValue(true)
	config := map[string]attr.Value{"forms_authentication": types.BoolValue(true)}

	got := coupledUnknown(spec.fields, plan, state, config)
	for _, name := range []string{"basic_authentication", "windows_authentication", "digest_authentication"} {
		if !got[name].IsUnknown() {
			t.Errorf("%s = %s, want unknown", name, got[name])
		}
	}
	if got["forms_authentication"].IsUnknown() || got["oauth_authentication"].IsUnknown() {
		t.Errorf("configured or uncoupled attribute became unknown: %v", got)
	}

	// Nothing configured in the group changes: the plan stays as it is.
	plan["forms_authentication"] = types.BoolValue(false)
	config["forms_authentication"] = types.BoolValue(false)
	for name, v := range coupledUnknown(spec.fields, plan, state, config) {
		if v.IsUnknown() {
			t.Errorf("%s became unknown without a change", name)
		}
	}
}

// Exchange accepts NTLM and reads it back as Ntlm: the state keeps the
// configured case, and unknown methods are refused at plan time.
func TestIISAuthenticationMethodsCase(t *testing.T) {
	for _, typeName := range []string{"mapi_virtual_directory", "outlook_anywhere"} {
		r := &objectResource{spec: specByType(t, typeName)}
		attrs := map[string]attr.Value{}
		for name, typ := range r.spec.attrTypes() {
			attrs[name] = nullOf(typ)
		}
		attrs["name"] = types.StringValue("EX01")
		attrs["iis_authentication_methods"] = types.SetValueMust(types.StringType,
			[]attr.Value{types.StringValue("NTLM"), types.StringValue("Negotiate")})
		prior := types.ObjectValueMust(r.spec.attrTypes(), attrs)
		got, diags := r.fromAPI(prior, []byte(`{"Name":"EX01","IISAuthenticationMethods":["Ntlm","Negotiate"]}`), nil)
		if diags.HasError() {
			t.Fatal(diags)
		}
		if g := got.Attributes()["iis_authentication_methods"]; !g.Equal(attrs["iis_authentication_methods"]) {
			t.Errorf("%s: iis_authentication_methods = %s, want the configured case", typeName, g)
		}
		for _, f := range r.spec.fields {
			if f.name == "iis_authentication_methods" && len(f.values) == 0 {
				t.Errorf("%s: iis_authentication_methods has no valid values", typeName)
			}
		}
	}
}

func nullOf(typ attr.Type) attr.Value {
	switch typ {
	case types.BoolType:
		return types.BoolNull()
	case types.Int64Type:
		return types.Int64Null()
	case types.StringType:
		return types.StringNull()
	}
	if _, ok := typ.(types.SetType); ok {
		return types.SetNull(types.StringType)
	}
	return types.ListNull(types.StringType)
}
