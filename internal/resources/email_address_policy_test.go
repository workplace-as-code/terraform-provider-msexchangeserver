package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEmailAddressPolicyAppliesOnlyWhenASettingChanges(t *testing.T) {
	state := emailAddressPolicyModel{
		EnabledEmailAddressTemplates: stringsToList([]string{"SMTP:%m@contoso.com"}),
		Priority:                     types.Int64Value(1),
		IncludedRecipients:           types.StringValue("MailboxUsers"),
		ConditionalDepartment:        types.StringValue(""),
		ConditionalCompany:           types.StringValue(""),
		ApplyToRecipients:            types.BoolNull(),
	}

	onlyFlag := state
	onlyFlag.ApplyToRecipients = types.BoolValue(true)
	if onlyFlag.settingsChanged(state) {
		t.Error("setting apply_to_recipients alone must not apply the policy")
	}

	priority := onlyFlag
	priority.Priority = types.Int64Value(2)
	if !priority.settingsChanged(state) {
		t.Error("a priority change must apply the policy")
	}

	templates := onlyFlag
	templates.EnabledEmailAddressTemplates = stringsToList([]string{"SMTP:%g.%s@contoso.com"})
	if !templates.settingsChanged(state) {
		t.Error("a template change must apply the policy")
	}
}
