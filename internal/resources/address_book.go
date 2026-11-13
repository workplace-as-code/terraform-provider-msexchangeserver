package resources

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Address lists, global address lists, offline address books and address book
// policies (address_book.ps1). The provider writes their configuration only:
// recalculating members (Update-AddressList, Update-GlobalAddressList) and
// generating offline address books (Update-OfflineAddressBook) stay
// administrator tasks.

// defaultGAL is the GAL Exchange creates, which cannot be changed.
const defaultGAL = "Default Global Address List"

// membersNote is the note of a list whose members Exchange recalculates with cmdlet.
func membersNote(cmdlet string) string {
	return "The provider never recalculates members: a recipient enters or leaves the list when its own " +
		"object next changes, or when an administrator runs `" + cmdlet + "`."
}

func recipientContainerAttr(what string) field {
	return caselessAttr(stringAttr("recipient_container", "RecipientContainer", "Organizational unit the filter "+
		"applies to, as a canonical name (`contoso.com/Paris`). `\"\"` applies it to the whole organization. "+
		"Only recipients in this unit can be in the "+what+"."))
}

// checkDefaultGAL refuses to create or change the default GAL: Exchange refuses
// every change to it. An imported default GAL planned without change passes.
func checkDefaultGAL(_, plan, state map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	name, _ := plan["name"].(types.String)
	if !strings.EqualFold(name.ValueString(), defaultGAL) {
		return diags
	}
	changed := state == nil
	for k, v := range plan {
		if s, ok := state[k]; state != nil && known(v) && (!ok || !v.Equal(s)) {
			changed = true
		}
	}
	if changed {
		diags.AddAttributeError(path.Root("name"), "The default global address list cannot be changed",
			"Exchange refuses every change to \""+defaultGAL+"\". Import it to read it, without changing its "+
				"attributes, or use the msexchangeserver_global_address_list data source.")
	}
	return diags
}

// checkOabDistribution refuses virtual directories with web distribution on:
// Exchange refuses the combination.
func checkOabDistribution(config, _, _ map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	web, _ := config["global_web_distribution_enabled"].(types.Bool)
	vdirs, _ := config["virtual_directories"].(types.Set)
	if known(web) && web.ValueBool() && known(vdirs) && len(vdirs.Elements()) > 0 {
		diags.AddAttributeError(path.Root("virtual_directories"), "Conflicting distribution settings",
			"virtual_directories cannot be set when global_web_distribution_enabled is true: Exchange refuses it.")
	}
	return diags
}
