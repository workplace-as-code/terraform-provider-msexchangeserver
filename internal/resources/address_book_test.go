package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func fieldsByName(s objectSpec) map[string]field {
	out := map[string]field{}
	for _, f := range s.fields {
		out[f.name] = f
	}
	return out
}

// The four address book resources run on address_book.ps1, on Mailbox servers only.
func TestAddressBookSpecs(t *testing.T) {
	want := map[string][]string{
		"address_list":         {"display_name", "container", "recipient_filter", "recipient_container"},
		"global_address_list":  {"recipient_filter", "recipient_container"},
		"offline_address_book": {"address_lists", "generating_mailbox", "virtual_directories", "global_web_distribution_enabled", "is_default"},
		"address_book_policy":  {"global_address_list", "offline_address_book", "room_list", "address_lists"},
	}
	for typeName, names := range want {
		s := specByType(t, typeName)
		if s.scriptName() != "address_book.ps1" || s.singleton || s.adopted || s.edge {
			t.Errorf("%s: script %s, singleton %v, adopted %v, edge %v", typeName, s.scriptName(), s.singleton, s.adopted, s.edge)
		}
		fields := fieldsByName(s)
		if len(fields) != len(names) {
			t.Errorf("%s: %d fields, want %v", typeName, len(fields), names)
		}
		for _, n := range names {
			if _, ok := fields[n]; !ok {
				t.Errorf("%s: no field %s", typeName, n)
			}
		}
	}
	al := fieldsByName(specByType(t, "address_list"))
	if al["recipient_filter"].kind != filterField || !al["container"].forceNew || !al["recipient_container"].caseless {
		t.Error("address_list: recipient_filter must be a filter, container force new, recipient_container caseless")
	}
	if gal := fieldsByName(specByType(t, "global_address_list")); gal["recipient_filter"].kind != filterField {
		t.Error("global_address_list: recipient_filter must be a filter")
	}
	oab := fieldsByName(specByType(t, "offline_address_book"))
	if oab["is_default"].kind != boolField || oab["address_lists"].kind != setField || oab["virtual_directories"].kind != setField {
		t.Error("offline_address_book: is_default bool, address_lists and virtual_directories sets")
	}
	if oab["virtual_directories"].coupled == "" || oab["virtual_directories"].coupled != oab["global_web_distribution_enabled"].coupled {
		t.Error("offline_address_book: virtual_directories and global_web_distribution_enabled must be coupled")
	}
	if abp := fieldsByName(specByType(t, "address_book_policy")); abp["address_lists"].kind != setField {
		t.Error("address_book_policy: address_lists must be a set")
	}
}

func galAttrs(name, filter string) map[string]attr.Value {
	return map[string]attr.Value{"id": types.StringValue(name), "name": types.StringValue(name),
		"recipient_filter": types.StringValue(filter), "recipient_container": types.StringValue("")}
}

// The default GAL cannot be changed: imported and planned without change it
// passes, created or changed it is refused at plan time.
func TestDefaultGALRefusedOnChange(t *testing.T) {
	check := specByType(t, "global_address_list").planCheck
	if check == nil {
		t.Fatal("global_address_list has no planCheck")
	}
	def := galAttrs("default global address list", "((Alias -ne $null))")
	if d := check(def, def, def); d.HasError() {
		t.Errorf("unchanged default GAL refused: %v", d)
	}
	changed := galAttrs("default global address list", "Company -eq 'Contoso'")
	if d := check(changed, changed, def); !d.HasError() {
		t.Error("change to the default GAL passes")
	}
	if d := check(def, def, nil); !d.HasError() {
		t.Error("creation of the default GAL passes")
	}
	other := galAttrs("TF-GAL", "Company -eq 'Contoso'")
	if d := check(other, other, galAttrs("TF-GAL", "Company -eq 'Other'")); d.HasError() {
		t.Errorf("change to another GAL refused: %v", d)
	}
}

// Exchange refuses virtual directories while web distribution is on.
func TestOabWebAndVirtualDirectoriesRefused(t *testing.T) {
	check := specByType(t, "offline_address_book").planCheck
	if check == nil {
		t.Fatal("offline_address_book has no planCheck")
	}
	vdirs := types.SetValueMust(types.StringType, []attr.Value{types.StringValue(`EX01\OAB (Default Web Site)`)})
	empty := types.SetValueMust(types.StringType, []attr.Value{})
	cases := []struct {
		vdirs   attr.Value
		web     attr.Value
		refused bool
	}{
		{vdirs, types.BoolValue(true), true},
		{vdirs, types.BoolValue(false), false},
		{empty, types.BoolValue(true), false},
		{types.SetNull(types.StringType), types.BoolValue(true), false},
		{vdirs, types.BoolNull(), false},
	}
	for i, c := range cases {
		config := map[string]attr.Value{"virtual_directories": c.vdirs, "global_web_distribution_enabled": c.web}
		if d := check(config, config, nil); d.HasError() != c.refused {
			t.Errorf("case %d: refused %v, want %v", i, d.HasError(), c.refused)
		}
	}
}

// Exchange refuses to unset the default OAB: false is refused when the OAB is
// the default, and passes when it is not (a generated configuration writes
// is_default = false for every other OAB).
func TestOabIsDefaultFalseRefusedOnDefault(t *testing.T) {
	check := specByType(t, "offline_address_book").planCheck
	isDefault := func(v attr.Value) map[string]attr.Value { return map[string]attr.Value{"is_default": v} }
	cases := []struct {
		config, state map[string]attr.Value
		refused       bool
	}{
		{isDefault(types.BoolValue(false)), isDefault(types.BoolValue(true)), true},
		{isDefault(types.BoolValue(false)), isDefault(types.BoolValue(false)), false},
		{isDefault(types.BoolValue(false)), nil, false},
		{isDefault(types.BoolValue(true)), isDefault(types.BoolValue(false)), false},
		{isDefault(types.BoolNull()), isDefault(types.BoolValue(true)), false},
	}
	for i, c := range cases {
		if d := check(c.config, c.config, c.state); d.HasError() != c.refused {
			t.Errorf("case %d: refused %v, want %v", i, d.HasError(), c.refused)
		}
	}
}
