package resources

import (
	"sort"
	"strings"
	"testing"
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
